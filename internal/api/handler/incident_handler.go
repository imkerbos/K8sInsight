package handler

import (
	"context"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/kerbos/k8sinsight/internal/api/response"
	"github.com/kerbos/k8sinsight/internal/service"
	"github.com/kerbos/k8sinsight/internal/store/repository"
)

// IncidentHandler 异常事件 API 处理器
type IncidentHandler struct {
	svc           *service.IncidentService
	logger        *zap.Logger
	listCache     *localTTLCache
	detailCache   *localTTLCache
	evidenceCache *localTTLCache
}

// NewIncidentHandler 创建事件处理器
func NewIncidentHandler(svc *service.IncidentService, logger *zap.Logger) *IncidentHandler {
	return &IncidentHandler{
		svc:           svc,
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
		response.OK(c, cached)
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
			response.BadRequest(c, "cursorLastSeen 格式无效")
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
	result, err := h.svc.List(queryCtx, opts)
	if err != nil {
		h.logger.Error("查询事件列表失败", zap.Error(err))
		response.ServerError(c, "查询失败")
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
	response.OK(c, resp)
}

// Get 获取事件详情
func (h *IncidentHandler) Get(c *gin.Context) {
	id := c.Param("id")
	if cached, ok := h.detailCache.get(id); ok {
		response.OK(c, cached)
		return
	}

	incident, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "事件未找到")
		return
	}

	h.detailCache.set(id, incident)
	response.OK(c, incident)
}

// GetEvidences 获取事件关联的证据列表
func (h *IncidentHandler) GetEvidences(c *gin.Context) {
	id := c.Param("id")
	if cached, ok := h.evidenceCache.get(id); ok {
		response.OK(c, cached)
		return
	}

	evidences, err := h.svc.GetEvidences(c.Request.Context(), id)
	if err != nil {
		h.logger.Error("查询证据失败", zap.Error(err))
		response.ServerError(c, "查询失败")
		return
	}

	resp := gin.H{"items": evidences}
	h.evidenceCache.set(id, resp)
	response.OK(c, resp)
}

// GetTimeline 获取事件时间线（预留）
func (h *IncidentHandler) GetTimeline(c *gin.Context) {
	response.OK(c, gin.H{"items": []interface{}{}})
}

// RecollectMetrics 手动补采 Prometheus 指标
func (h *IncidentHandler) RecollectMetrics(c *gin.Context) {
	incidentID := c.Param("id")
	if incidentID == "" {
		response.BadRequest(c, "事件ID不能为空")
		return
	}

	result, err := h.svc.RecollectMetrics(c.Request.Context(), incidentID)
	if err != nil {
		h.logger.Warn("补采事件指标失败", zap.String("incidentID", incidentID), zap.Error(err))
		response.ServerError(c, err.Error())
		return
	}

	h.evidenceCache.del(incidentID)
	response.OK(c, gin.H{
		"message":       "指标补采成功",
		"incidentId":    result.IncidentID,
		"podName":       result.PodName,
		"collectedAt":   result.CollectedAt,
		"prometheusURL": result.PrometheusURL,
	})
}
