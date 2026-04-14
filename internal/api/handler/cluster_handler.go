package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/kerbos/k8sinsight/internal/cluster"
	"github.com/kerbos/k8sinsight/internal/core"
	"github.com/kerbos/k8sinsight/internal/store/model"
	"github.com/kerbos/k8sinsight/internal/store/repository"
)

// ClusterHandler 集群管理 API 处理器
type ClusterHandler struct {
	clusterRepo repository.ClusterRepository
	clusterMgr  *cluster.Manager
	logger      *zap.Logger
}

// NewClusterHandler 创建集群处理器
func NewClusterHandler(
	clusterRepo repository.ClusterRepository,
	clusterMgr *cluster.Manager,
	logger *zap.Logger,
) *ClusterHandler {
	return &ClusterHandler{
		clusterRepo: clusterRepo,
		clusterMgr:  clusterMgr,
		logger:      logger.Named("api.cluster"),
	}
}

type createClusterRequest struct {
	Name           string `json:"name" binding:"required"`
	KubeconfigData string `json:"kubeconfigData" binding:"required"`
}

type updateClusterRequest struct {
	Name           string `json:"name"`
	KubeconfigData string `json:"kubeconfigData"`
}

// Create 创建集群（仅保存，默认启用，连接状态 unknown）
func (h *ClusterHandler) Create(c *gin.Context) {
	var req createClusterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	cl := &model.Cluster{
		ID:               uuid.New().String(),
		Name:             req.Name,
		KubeconfigData:   req.KubeconfigData,
		Status:           "active",
		ConnectionStatus: "unknown",
	}

	if err := h.clusterRepo.Create(c.Request.Context(), cl); err != nil {
		h.logger.Error("创建集群失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建集群失败"})
		return
	}

	// 自动启动集群管道
	if err := h.clusterMgr.StartCluster(c.Request.Context(), cl); err != nil {
		h.logger.Error("集群已创建但管道启动失败", zap.String("clusterID", cl.ID), zap.Error(err))
	}

	h.logger.Info("集群已创建", zap.String("id", cl.ID), zap.String("name", cl.Name))
	c.JSON(http.StatusCreated, cl)
}

// List 获取集群列表
func (h *ClusterHandler) List(c *gin.Context) {
	clusters, err := h.clusterRepo.List(c.Request.Context())
	if err != nil {
		h.logger.Error("查询集群列表失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": clusters})
}

// Get 获取集群详情
func (h *ClusterHandler) Get(c *gin.Context) {
	id := c.Param("id")
	cl, err := h.clusterRepo.FindByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "集群未找到"})
		return
	}
	c.JSON(http.StatusOK, cl)
}

// Update 更新集群配置
func (h *ClusterHandler) Update(c *gin.Context) {
	id := c.Param("id")
	cl, err := h.clusterRepo.FindByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "集群未找到"})
		return
	}

	var req updateClusterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if req.Name != "" {
		cl.Name = req.Name
	}
	if req.KubeconfigData != "" {
		cl.KubeconfigData = req.KubeconfigData
		// kubeconfig 变了，重置连接状态
		cl.ConnectionStatus = "unknown"
		cl.Version = ""
		cl.NodeCount = 0
		cl.StatusMessage = ""
	}

	if err := h.clusterRepo.Update(c.Request.Context(), cl); err != nil {
		h.logger.Error("更新集群失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新集群失败"})
		return
	}

	c.JSON(http.StatusOK, cl)
}

// Delete 删除集群（先停管道）
func (h *ClusterHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	if h.clusterMgr.IsRunning(id) {
		if err := h.clusterMgr.StopCluster(id); err != nil {
			h.logger.Error("停止集群管道失败", zap.Error(err))
		}
	}

	if err := h.clusterRepo.Delete(c.Request.Context(), id); err != nil {
		h.logger.Error("删除集群失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除集群失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "集群已删除"})
}

// TestConnection 测试已有集群的连接（通过集群 ID，5s 超时重试 3 次）
// 成功后将 version、nodeCount、connectionStatus 写入数据库
func (h *ClusterHandler) TestConnection(c *gin.Context) {
	id := c.Param("id")
	cl, err := h.clusterRepo.FindByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "集群未找到"})
		return
	}

	info, err := core.TestKubeConnection(cl.KubeconfigData)
	if err != nil {
		cl.ConnectionStatus = "failed"
		cl.StatusMessage = err.Error()
		cl.Version = ""
		cl.NodeCount = 0
		_ = h.clusterRepo.Update(c.Request.Context(), cl)

		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	cl.ConnectionStatus = "connected"
	cl.StatusMessage = ""
	cl.Version = info.Version
	cl.NodeCount = info.NodeCount
	_ = h.clusterRepo.Update(c.Request.Context(), cl)

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "连接成功",
		"version":   info.Version,
		"nodeCount": info.NodeCount,
	})
}

// Activate 启用集群（仅更新数据库状态）
func (h *ClusterHandler) Activate(c *gin.Context) {
	id := c.Param("id")
	cl, err := h.clusterRepo.FindByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "集群未找到"})
		return
	}

	cl.Status = "active"
	if err := h.clusterRepo.Update(c.Request.Context(), cl); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新状态失败"})
		return
	}

	// 启动集群管道（如果尚未运行）
	if !h.clusterMgr.IsRunning(cl.ID) {
		if err := h.clusterMgr.StartCluster(c.Request.Context(), cl); err != nil {
			h.logger.Error("启动集群管道失败", zap.String("clusterID", cl.ID), zap.Error(err))
			c.JSON(http.StatusOK, gin.H{"message": "集群已启用，但管道启动失败: " + err.Error(), "status": "active"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "集群已启用", "status": "active"})
}

// Deactivate 禁用集群（停管道 + 更新状态）
func (h *ClusterHandler) Deactivate(c *gin.Context) {
	id := c.Param("id")
	cl, err := h.clusterRepo.FindByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "集群未找到"})
		return
	}

	if h.clusterMgr.IsRunning(cl.ID) {
		if err := h.clusterMgr.StopCluster(cl.ID); err != nil {
			h.logger.Error("停止集群管道失败", zap.Error(err))
		}
	}

	cl.Status = "inactive"
	if err := h.clusterRepo.Update(c.Request.Context(), cl); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新状态失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "集群已禁用", "status": "inactive"})
}
