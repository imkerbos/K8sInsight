package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/kerbos/k8sinsight/internal/store/model"
	"github.com/kerbos/k8sinsight/internal/store/repository"
)

// MonitorRuleHandler 监控规则 API 处理器
type MonitorRuleHandler struct {
	ruleRepo    repository.MonitorRuleRepository
	clusterRepo repository.ClusterRepository
	logger      *zap.Logger
}

// NewMonitorRuleHandler 创建监控规则处理器
func NewMonitorRuleHandler(
	ruleRepo repository.MonitorRuleRepository,
	clusterRepo repository.ClusterRepository,
	logger *zap.Logger,
) *MonitorRuleHandler {
	return &MonitorRuleHandler{
		ruleRepo:    ruleRepo,
		clusterRepo: clusterRepo,
		logger:      logger.Named("api.monitor-rule"),
	}
}

type createMonitorRuleRequest struct {
	ClusterID       string `json:"clusterId" binding:"required"`
	Name            string `json:"name" binding:"required"`
	Description     string `json:"description"`
	WatchScope      string `json:"watchScope"`
	WatchNamespaces string `json:"watchNamespaces"`
	LabelSelector   string `json:"labelSelector"`
	AnomalyTypes    string `json:"anomalyTypes"`
}

type updateMonitorRuleRequest struct {
	Name            string `json:"name"`
	Description     string `json:"description"`
	WatchScope      string `json:"watchScope"`
	WatchNamespaces string `json:"watchNamespaces"`
	LabelSelector   string `json:"labelSelector"`
	AnomalyTypes    string `json:"anomalyTypes"`
}

// List 获取监控规则列表
func (h *MonitorRuleHandler) List(c *gin.Context) {
	rules, err := h.ruleRepo.List(c.Request.Context())
	if err != nil {
		h.logger.Error("查询监控规则列表失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rules})
}

// Create 创建监控规则
func (h *MonitorRuleHandler) Create(c *gin.Context) {
	var req createMonitorRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	// 验证集群存在
	if _, err := h.clusterRepo.FindByID(c.Request.Context(), req.ClusterID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "集群不存在"})
		return
	}

	// 检查集群是否已有规则
	if existing, _ := h.ruleRepo.FindByClusterID(c.Request.Context(), req.ClusterID); existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "该集群已存在监控规则"})
		return
	}

	rule := &model.MonitorRule{
		ID:              uuid.New().String(),
		ClusterID:       req.ClusterID,
		Name:            req.Name,
		Description:     req.Description,
		Enabled:         true,
		WatchScope:      req.WatchScope,
		WatchNamespaces: req.WatchNamespaces,
		LabelSelector:   req.LabelSelector,
		AnomalyTypes:    req.AnomalyTypes,
	}

	if rule.WatchScope == "" {
		rule.WatchScope = "cluster"
	}

	if err := h.ruleRepo.Create(c.Request.Context(), rule); err != nil {
		h.logger.Error("创建监控规则失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建监控规则失败"})
		return
	}

	h.logger.Info("监控规则已创建", zap.String("id", rule.ID), zap.String("name", rule.Name))
	c.JSON(http.StatusCreated, rule)
}

// Update 更新监控规则
func (h *MonitorRuleHandler) Update(c *gin.Context) {
	id := c.Param("id")
	rule, err := h.ruleRepo.FindByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "监控规则未找到"})
		return
	}

	var req updateMonitorRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if req.Name != "" {
		rule.Name = req.Name
	}
	if req.Description != "" {
		rule.Description = req.Description
	}
	if req.WatchScope != "" {
		rule.WatchScope = req.WatchScope
	}
	if req.WatchNamespaces != "" {
		rule.WatchNamespaces = req.WatchNamespaces
	}
	if req.LabelSelector != "" {
		rule.LabelSelector = req.LabelSelector
	}
	if req.AnomalyTypes != "" {
		rule.AnomalyTypes = req.AnomalyTypes
	}

	if err := h.ruleRepo.Update(c.Request.Context(), rule); err != nil {
		h.logger.Error("更新监控规则失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新监控规则失败"})
		return
	}

	c.JSON(http.StatusOK, rule)
}

// Delete 删除监控规则
func (h *MonitorRuleHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.ruleRepo.Delete(c.Request.Context(), id); err != nil {
		h.logger.Error("删除监控规则失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除监控规则失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "监控规则已删除"})
}

// Toggle 切换监控规则启用/禁用
func (h *MonitorRuleHandler) Toggle(c *gin.Context) {
	id := c.Param("id")
	rule, err := h.ruleRepo.FindByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "监控规则未找到"})
		return
	}

	rule.Enabled = !rule.Enabled
	if err := h.ruleRepo.Update(c.Request.Context(), rule); err != nil {
		h.logger.Error("切换监控规则状态失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "操作失败"})
		return
	}

	c.JSON(http.StatusOK, rule)
}
