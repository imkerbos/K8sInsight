package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/kerbos/k8sinsight/internal/store/repository"
)

// IncidentHandler 异常事件 API 处理器
type IncidentHandler struct {
	incidentRepo  repository.IncidentRepository
	evidenceRepo  repository.EvidenceRepository
	logger        *zap.Logger
	listCache     *localTTLCache
	detailCache   *localTTLCache
	evidenceCache *localTTLCache
}

// NewIncidentHandler 创建事件处理器
func NewIncidentHandler(
	incidentRepo repository.IncidentRepository,
	evidenceRepo repository.EvidenceRepository,
	logger *zap.Logger,
) *IncidentHandler {
	return &IncidentHandler{
		incidentRepo:  incidentRepo,
		evidenceRepo:  evidenceRepo,
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
