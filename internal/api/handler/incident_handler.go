package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/kerbos/k8sinsight/internal/collector"
	"github.com/kerbos/k8sinsight/internal/detector"
	"github.com/kerbos/k8sinsight/internal/store/repository"
)

// IncidentHandler 异常事件 API 处理器
type IncidentHandler struct {
	incidentRepo  repository.IncidentRepository
	evidenceRepo  repository.EvidenceRepository
	settingRepo   repository.SettingRepository
	logger        *zap.Logger
	listCache     *localTTLCache
	detailCache   *localTTLCache
	evidenceCache *localTTLCache
}

// NewIncidentHandler 创建事件处理器
func NewIncidentHandler(
	incidentRepo repository.IncidentRepository,
	evidenceRepo repository.EvidenceRepository,
	settingRepo repository.SettingRepository,
	logger *zap.Logger,
) *IncidentHandler {
	return &IncidentHandler{
		incidentRepo:  incidentRepo,
		evidenceRepo:  evidenceRepo,
		settingRepo:   settingRepo,
		logger:        logger.Named("api.incident"),
		listCache:     newLocalTTLCache(2 * time.Second),
		detailCache:   newLocalTTLCache(2 * time.Second),
		evidenceCache: newLocalTTLCache(2 * time.Second),
	}
}

// List 获取事件列表
func (h *IncidentHandler) List(c *gin.Context) {
	start := time.Now()
	cacheKey := c.Request.URL.RawQuery
	if cached, ok := h.listCache.get(cacheKey); ok {
		c.JSON(http.StatusOK, cached)
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	cursorLastSeenRaw := c.Query("cursorLastSeen")
	cursorID := c.Query("cursorId")
	includeTotal := c.Query("includeTotal") == "true"

	var cursorLastSeen *time.Time
	if cursorLastSeenRaw != "" {
		ts, err := time.Parse(time.RFC3339Nano, cursorLastSeenRaw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cursorLastSeen 格式无效"})
			return
		}
		cursorLastSeen = &ts
	}

	opts := repository.ListOptions{
		Namespace:      c.Query("namespace"),
		State:          c.Query("state"),
		AnomalyType:    c.Query("type"),
		ClusterID:      c.Query("clusterId"),
		OwnerName:      c.Query("ownerName"),
		UseCursor:      cursorLastSeen != nil && cursorID != "",
		CursorLastSeen: cursorLastSeen,
		CursorID:       cursorID,
		IncludeTotal:   includeTotal,
		Page:           page,
		PageSize:       pageSize,
	}

	queryCtx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	result, err := h.incidentRepo.List(queryCtx, opts)
	if err != nil {
		h.logger.Error("查询事件列表失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	resp := gin.H{
		"items":    result.Items,
		"page":     page,
		"pageSize": pageSize,
		"hasMore":  result.HasMore,
	}
	if result.Total != nil {
		resp["total"] = *result.Total
	}
	if result.NextCursorLastSeen != nil {
		resp["nextCursorLastSeen"] = result.NextCursorLastSeen.Format(time.RFC3339Nano)
		resp["nextCursorId"] = result.NextCursorID
	}

	dur := time.Since(start)
	if dur > 200*time.Millisecond {
		h.logger.Warn("事件列表慢查询",
			zap.Duration("duration", dur),
			zap.String("namespace", opts.Namespace),
			zap.String("state", opts.State),
			zap.String("type", opts.AnomalyType),
			zap.String("ownerName", opts.OwnerName),
			zap.Bool("useCursor", opts.UseCursor),
			zap.Int("pageSize", opts.PageSize),
		)
	}

	h.listCache.set(cacheKey, resp)
	c.JSON(http.StatusOK, resp)
}

// Get 获取事件详情
func (h *IncidentHandler) Get(c *gin.Context) {
	id := c.Param("id")
	if cached, ok := h.detailCache.get(id); ok {
		c.JSON(http.StatusOK, cached)
		return
	}

	incident, err := h.incidentRepo.FindByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "事件未找到"})
		return
	}

	h.detailCache.set(id, incident)
	c.JSON(http.StatusOK, incident)
}

// GetEvidences 获取事件关联的证据列表
func (h *IncidentHandler) GetEvidences(c *gin.Context) {
	id := c.Param("id")
	if cached, ok := h.evidenceCache.get(id); ok {
		c.JSON(http.StatusOK, cached)
		return
	}

	evidences, err := h.evidenceRepo.FindByIncidentID(c.Request.Context(), id)
	if err != nil {
		h.logger.Error("查询证据失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	resp := gin.H{"items": evidences}
	h.evidenceCache.set(id, resp)
	c.JSON(http.StatusOK, resp)
}

// GetTimeline 获取事件时间线（预留）
func (h *IncidentHandler) GetTimeline(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": []interface{}{}})
}

// RecollectMetrics 手动补采单个事件的 Prometheus 指标并落库存证据
func (h *IncidentHandler) RecollectMetrics(c *gin.Context) {
	incidentID := c.Param("id")
	if incidentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "事件ID不能为空"})
		return
	}

	if h.settingRepo == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "系统设置仓储未初始化"})
		return
	}

	incident, err := h.incidentRepo.FindByID(c.Request.Context(), incidentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "事件未找到"})
			return
		}
		h.logger.Error("查询事件失败", zap.String("incidentID", incidentID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询事件失败"})
		return
	}

	podName := firstPodName(incident.PodNames)
	if podName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "事件缺少 Pod 信息，无法补采指标"})
		return
	}

	promURL, promQueryRange, err := h.loadPrometheusReplayConfig(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	eventTs := incident.LastSeen
	if eventTs.IsZero() {
		eventTs = incident.FirstSeen
	}
	if eventTs.IsZero() {
		eventTs = time.Now()
	}

	event := detector.AnomalyEvent{
		ID:        "recollect-" + incident.ID,
		Timestamp: eventTs,
		Namespace: incident.Namespace,
		PodName:   podName,
	}

	queryCtx, cancel := context.WithTimeout(c.Request.Context(), 12*time.Second)
	defer cancel()

	content, err := collector.CollectPrometheusRange(queryCtx, promURL, promQueryRange, event)
	if err != nil {
		h.logger.Warn("补采事件指标失败", zap.String("incidentID", incidentID), zap.String("pod", podName), zap.Error(err))
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("Prometheus 补采失败: %v", err)})
		return
	}

	now := time.Now()
	bundle := &collector.EvidenceBundle{
		AnomalyEvent: event,
		Evidences: []collector.Evidence{
			{
				Type:      collector.EvidenceMetrics,
				Content:   content,
				Timestamp: now,
			},
		},
		CollectedAt: now,
	}

	if err := h.evidenceRepo.SaveBundle(c.Request.Context(), incident.ID, bundle); err != nil {
		h.logger.Error("保存补采指标证据失败", zap.String("incidentID", incidentID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存补采证据失败"})
		return
	}

	// 主动失效本地短缓存，确保前端立即拿到新证据。
	h.evidenceCache.del(incidentID)

	c.JSON(http.StatusOK, gin.H{
		"message":       "指标补采成功",
		"incidentId":    incidentID,
		"podName":       podName,
		"collectedAt":   now,
		"prometheusURL": promURL,
	})
}

func firstPodName(rawPodNames string) string {
	if strings.TrimSpace(rawPodNames) == "" {
		return ""
	}
	var pods []string
	if err := json.Unmarshal([]byte(rawPodNames), &pods); err != nil {
		return ""
	}
	if len(pods) == 0 {
		return ""
	}
	return strings.TrimSpace(pods[0])
}

func (h *IncidentHandler) loadPrometheusReplayConfig(ctx context.Context) (string, time.Duration, error) {
	settings, err := h.settingRepo.BatchGet(ctx, []string{
		"collect_prometheus_url",
		"collect_prom_query_range",
	})
	if err != nil {
		h.logger.Error("读取资源采集设置失败", zap.Error(err))
		return "", 0, fmt.Errorf("读取资源采集设置失败")
	}

	promURL := strings.TrimSpace(settings["collect_prometheus_url"])
	if promURL == "" {
		return "", 0, fmt.Errorf("未配置 Prometheus 地址，请先在资源采集配置中保存")
	}

	promRangeRaw := strings.TrimSpace(settings["collect_prom_query_range"])
	if promRangeRaw == "" {
		promRangeRaw = collectKeys["collect_prom_query_range"]
	}
	promRange, err := time.ParseDuration(promRangeRaw)
	if err != nil {
		return "", 0, fmt.Errorf("Prometheus 查询时间窗配置无效")
	}

	return promURL, promRange, nil
}
