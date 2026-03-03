package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/kerbos/k8sinsight/internal/auth"
)

// SSOHandler SSO 认证 API 处理器
type SSOHandler struct {
	ssoService *auth.SSOService
	logger     *zap.Logger
}

// NewSSOHandler 创建 SSO 处理器
func NewSSOHandler(ssoService *auth.SSOService, logger *zap.Logger) *SSOHandler {
	return &SSOHandler{
		ssoService: ssoService,
		logger:     logger.Named("api.sso"),
	}
}

// GetConfig 获取 SSO 配置（公开端点，无需认证）
// GET /api/v1/auth/sso/config
func (h *SSOHandler) GetConfig(c *gin.Context) {
	resp, err := h.ssoService.GetSSOConfig(c.Request.Context())
	if err != nil {
		h.logger.Error("获取 SSO 配置失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取 SSO 配置失败"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Authorize 生成 OIDC 授权 URL
// GET /api/v1/auth/sso/authorize
func (h *SSOHandler) Authorize(c *gin.Context) {
	authURL, err := h.ssoService.GetAuthURL(c.Request.Context())
	if err != nil {
		h.logger.Error("生成 SSO 授权 URL 失败", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"authorizeUrl": authURL})
}

// CallbackRequest SSO 回调请求
type CallbackRequest struct {
	Code  string `json:"code" binding:"required"`
	State string `json:"state" binding:"required"`
}

// Callback 处理 OIDC 回调
// POST /api/v1/auth/sso/callback
func (h *SSOHandler) Callback(c *gin.Context) {
	var req CallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	resp, err := h.ssoService.HandleCallback(c.Request.Context(), req.Code, req.State)
	if err != nil {
		h.logger.Error("SSO 回调处理失败", zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}
