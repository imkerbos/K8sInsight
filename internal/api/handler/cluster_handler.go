package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/kerbos/k8sinsight/internal/api/response"
	"github.com/kerbos/k8sinsight/internal/service"
)

// ClusterHandler 集群管理 API 处理器
type ClusterHandler struct {
	svc    *service.ClusterService
	logger *zap.Logger
}

// NewClusterHandler 创建集群处理器
func NewClusterHandler(svc *service.ClusterService, logger *zap.Logger) *ClusterHandler {
	return &ClusterHandler{
		svc:    svc,
		logger: logger.Named("api.cluster"),
	}
}

type createClusterRequest struct {
	Name             string `json:"name" binding:"required"`
	KubeconfigData   string `json:"kubeconfigData" binding:"required"`
	PrometheusURL    string `json:"prometheusUrl"`
	PrometheusLabels string `json:"prometheusLabels"`
}

type updateClusterRequest struct {
	Name             string  `json:"name"`
	KubeconfigData   string  `json:"kubeconfigData"`
	PrometheusURL    *string `json:"prometheusUrl"`
	PrometheusLabels *string `json:"prometheusLabels"`
}

// Create 创建集群
func (h *ClusterHandler) Create(c *gin.Context) {
	var req createClusterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数无效: "+err.Error())
		return
	}

	cl, err := h.svc.Create(c.Request.Context(), req.Name, req.KubeconfigData, req.PrometheusURL, req.PrometheusLabels)
	if err != nil {
		h.logger.Error("创建集群失败", zap.Error(err))
		response.ServerError(c, "创建集群失败")
		return
	}

	response.Created(c, cl)
}

// List 获取集群列表
func (h *ClusterHandler) List(c *gin.Context) {
	clusters, err := h.svc.List(c.Request.Context())
	if err != nil {
		h.logger.Error("查询集群列表失败", zap.Error(err))
		response.ServerError(c, "查询失败")
		return
	}

	response.OK(c, gin.H{"items": clusters})
}

// Get 获取集群详情
func (h *ClusterHandler) Get(c *gin.Context) {
	cl, err := h.svc.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.NotFound(c, "集群未找到")
		return
	}
	response.OK(c, cl)
}

// Update 更新集群配置
func (h *ClusterHandler) Update(c *gin.Context) {
	var req updateClusterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数无效: "+err.Error())
		return
	}

	cl, err := h.svc.Update(c.Request.Context(), c.Param("id"), req.Name, req.KubeconfigData, req.PrometheusURL, req.PrometheusLabels)
	if err != nil {
		h.logger.Error("更新集群失败", zap.Error(err))
		response.ServerError(c, "更新集群失败")
		return
	}

	response.OK(c, cl)
}

// Delete 删除集群
func (h *ClusterHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		h.logger.Error("删除集群失败", zap.Error(err))
		response.ServerError(c, "删除集群失败")
		return
	}
	response.OK(c, gin.H{"message": "集群已删除"})
}

// TestConnection 测试集群连接
func (h *ClusterHandler) TestConnection(c *gin.Context) {
	result, err := h.svc.TestConnection(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.OK(c, result)
}

// TestPrometheus 测试集群的 Prometheus/VictoriaMetrics 连接
func (h *ClusterHandler) TestPrometheus(c *gin.Context) {
	result, err := h.svc.TestPrometheus(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.OK(c, result)
}

// Activate 启用集群
func (h *ClusterHandler) Activate(c *gin.Context) {
	_, err := h.svc.Activate(c.Request.Context(), c.Param("id"))
	if err != nil {
		// 区分"未找到"和"管道启动失败"
		response.OK(c, gin.H{"message": err.Error(), "status": "active"})
		return
	}
	response.OK(c, gin.H{"message": "集群已启用", "status": "active"})
}

// Deactivate 禁用集群
func (h *ClusterHandler) Deactivate(c *gin.Context) {
	_, err := h.svc.Deactivate(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"message": "集群已禁用", "status": "inactive"})
}

// Metrics 查询集群级别 Prometheus 指标
func (h *ClusterHandler) Metrics(c *gin.Context) {
	rangeStr := c.DefaultQuery("range", "1h")
	rangeDur, err := time.ParseDuration(rangeStr)
	if err != nil || rangeDur <= 0 || rangeDur > 7*24*time.Hour {
		response.BadRequest(c, "无效时间范围，支持：1h, 6h, 24h, 72h")
		return
	}

	metrics, err := h.svc.GetMetrics(c.Request.Context(), c.Param("id"), rangeDur)
	if err != nil {
		h.logger.Warn("查询集群指标失败", zap.String("clusterID", c.Param("id")), zap.Error(err))
		response.ServerError(c, err.Error())
		return
	}

	response.OK(c, metrics)
}
