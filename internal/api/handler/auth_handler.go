package handler

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/kerbos/k8sinsight/internal/api/middleware"
	"github.com/kerbos/k8sinsight/internal/auth"
	"github.com/kerbos/k8sinsight/internal/store/repository"
)

// AuthHandler 认证相关 API 处理器
type AuthHandler struct {
	userRepo     repository.UserRepository
	roleRepo     repository.RoleRepository
	tokenService *auth.TokenService
	logger       *zap.Logger
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler(
	userRepo repository.UserRepository,
	roleRepo repository.RoleRepository,
	tokenService *auth.TokenService,
	logger *zap.Logger,
) *AuthHandler {
	return &AuthHandler{
		userRepo:     userRepo,
		roleRepo:     roleRepo,
		tokenService: tokenService,
		logger:       logger.Named("api.auth"),
	}
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type tokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

// Login 用户登录
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	ctx := c.Request.Context()

	user, err := h.userRepo.FindByUsername(ctx, req.Username)
	if err != nil || !auth.CheckPassword(req.Password, user.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	if !user.IsActive {
		c.JSON(http.StatusForbidden, gin.H{"error": "账户已被禁用"})
		return
	}

	accessToken, err := h.tokenService.GenerateAccessToken(user)
	if err != nil {
		h.logger.Error("生成 access token 失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "内部错误"})
		return
	}

	rawRefresh, hashRefresh, err := h.tokenService.GenerateRefreshToken()
	if err != nil {
		h.logger.Error("生成 refresh token 失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "内部错误"})
		return
	}

	rt := h.tokenService.NewRefreshTokenModel(user.ID, hashRefresh)
	if err := h.userRepo.CreateRefreshToken(ctx, rt); err != nil {
		h.logger.Error("保存 refresh token 失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "内部错误"})
		return
	}

	c.JSON(http.StatusOK, tokenResponse{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
	})
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

// Refresh 刷新令牌（Rotation：撤销旧 token，发新 token）
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	ctx := c.Request.Context()
	hash := auth.HashToken(req.RefreshToken)

	rt, err := h.userRepo.FindRefreshTokenByHash(ctx, hash)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "刷新令牌无效或已过期"})
		return
	}

	// Rotation：撤销旧 token
	if err := h.userRepo.RevokeRefreshToken(ctx, rt.ID); err != nil {
		h.logger.Error("撤销旧 refresh token 失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "内部错误"})
		return
	}

	user, err := h.userRepo.FindByID(ctx, rt.UserID)
	if err != nil || !user.IsActive {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在或已禁用"})
		return
	}

	accessToken, err := h.tokenService.GenerateAccessToken(user)
	if err != nil {
		h.logger.Error("生成 access token 失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "内部错误"})
		return
	}

	rawRefresh, hashRefresh, err := h.tokenService.GenerateRefreshToken()
	if err != nil {
		h.logger.Error("生成 refresh token 失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "内部错误"})
		return
	}

	newRT := h.tokenService.NewRefreshTokenModel(user.ID, hashRefresh)
	if err := h.userRepo.CreateRefreshToken(ctx, newRT); err != nil {
		h.logger.Error("保存新 refresh token 失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "内部错误"})
		return
	}

	c.JSON(http.StatusOK, tokenResponse{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
	})
}

type logoutRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

// Logout 登出（撤销指定的 refresh token）
func (h *AuthHandler) Logout(c *gin.Context) {
	var req logoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	ctx := c.Request.Context()
	hash := auth.HashToken(req.RefreshToken)

	rt, err := h.userRepo.FindRefreshTokenByHash(ctx, hash)
	if err != nil {
		// token 已失效或不存在，仍返回成功
		c.JSON(http.StatusOK, gin.H{"message": "已登出"})
		return
	}

	_ = h.userRepo.RevokeRefreshToken(ctx, rt.ID)
	c.JSON(http.StatusOK, gin.H{"message": "已登出"})
}

type changePasswordRequest struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required"`
}

// ChangePassword 修改密码（修改后撤销所有 refresh token）
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	if len(req.NewPassword) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "新密码至少 6 位"})
		return
	}

	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)

	user, err := h.userRepo.FindByID(ctx, userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
		return
	}

	if !auth.CheckPassword(req.OldPassword, user.PasswordHash) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "原密码错误"})
		return
	}

	newHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		h.logger.Error("哈希新密码失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "内部错误"})
		return
	}

	user.PasswordHash = newHash
	if err := h.userRepo.Update(ctx, user); err != nil {
		h.logger.Error("更新密码失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "内部错误"})
		return
	}

	// 改密后撤销所有 refresh token
	if err := h.userRepo.RevokeAllUserRefreshTokens(ctx, userID); err != nil {
		h.logger.Error("撤销 refresh token 失败", zap.Error(err))
	}

	c.JSON(http.StatusOK, gin.H{"message": "密码已修改"})
}

// Me 获取当前用户信息（含权限列表）
func (h *AuthHandler) Me(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)

	user, err := h.userRepo.FindByID(ctx, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	// 查询角色权限
	var permissions []string
	role, err := h.roleRepo.FindByName(ctx, user.Role)
	if err == nil {
		_ = json.Unmarshal([]byte(role.Permissions), &permissions)
	}
	if permissions == nil {
		permissions = []string{}
	}

	c.JSON(http.StatusOK, gin.H{
		"id":          user.ID,
		"username":    user.Username,
		"role":        user.Role,
		"isActive":    user.IsActive,
		"permissions": permissions,
	})
}
