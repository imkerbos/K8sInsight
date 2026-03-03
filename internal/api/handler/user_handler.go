package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/kerbos/k8sinsight/internal/auth"
	"github.com/kerbos/k8sinsight/internal/store/model"
	"github.com/kerbos/k8sinsight/internal/store/repository"
)

// UserHandler 用户管理 API 处理器
type UserHandler struct {
	userRepo    repository.UserRepository
	settingRepo repository.SettingRepository
	logger      *zap.Logger
}

// NewUserHandler 创建用户管理处理器
func NewUserHandler(userRepo repository.UserRepository, settingRepo repository.SettingRepository, logger *zap.Logger) *UserHandler {
	return &UserHandler{
		userRepo:    userRepo,
		settingRepo: settingRepo,
		logger:      logger.Named("api.user"),
	}
}

// List 获取用户列表
func (h *UserHandler) List(c *gin.Context) {
	users, err := h.userRepo.List(c.Request.Context())
	if err != nil {
		h.logger.Error("查询用户列表失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": users})
}

type createUserRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Role     string `json:"role"`
}

// Create 创建用户
func (h *UserHandler) Create(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	// 动态密码长度校验
	minLen := GetMinPasswordLength(h.settingRepo, c)
	if len(req.Password) < minLen {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("密码至少 %d 位", minLen)})
		return
	}

	ctx := c.Request.Context()

	// 检查用户名是否已存在
	if existing, _ := h.userRepo.FindByUsername(ctx, req.Username); existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "用户名已存在"})
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		h.logger.Error("哈希密码失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "内部错误"})
		return
	}

	role := req.Role
	if role == "" {
		role = "viewer"
	}

	user := &model.User{
		ID:           uuid.New().String(),
		Username:     req.Username,
		PasswordHash: hash,
		Role:         role,
		IsActive:     true,
	}

	if err := h.userRepo.Create(ctx, user); err != nil {
		h.logger.Error("创建用户失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建用户失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":       user.ID,
		"username": user.Username,
		"role":     user.Role,
		"isActive": user.IsActive,
	})
}

// ToggleActive 启用/禁用用户
func (h *UserHandler) ToggleActive(c *gin.Context) {
	id := c.Param("id")
	ctx := c.Request.Context()

	user, err := h.userRepo.FindByID(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户未找到"})
		return
	}

	user.IsActive = !user.IsActive
	if err := h.userRepo.Update(ctx, user); err != nil {
		h.logger.Error("更新用户状态失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
		return
	}

	// 禁用时撤销所有 refresh token
	if !user.IsActive {
		_ = h.userRepo.RevokeAllUserRefreshTokens(ctx, user.ID)
	}

	c.JSON(http.StatusOK, gin.H{
		"id":       user.ID,
		"isActive": user.IsActive,
	})
}

// ResetPassword 重置用户密码
func (h *UserHandler) ResetPassword(c *gin.Context) {
	id := c.Param("id")
	ctx := c.Request.Context()

	var req struct {
		NewPassword string `json:"newPassword" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	// 动态密码长度校验
	minLen := GetMinPasswordLength(h.settingRepo, c)
	if len(req.NewPassword) < minLen {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("密码至少 %d 位", minLen)})
		return
	}

	user, err := h.userRepo.FindByID(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户未找到"})
		return
	}

	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "内部错误"})
		return
	}

	user.PasswordHash = hash
	if err := h.userRepo.Update(ctx, user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "重置密码失败"})
		return
	}

	_ = h.userRepo.RevokeAllUserRefreshTokens(ctx, user.ID)
	c.JSON(http.StatusOK, gin.H{"message": "密码已重置"})
}

// ChangeRole 修改用户角色
func (h *UserHandler) ChangeRole(c *gin.Context) {
	id := c.Param("id")
	ctx := c.Request.Context()

	var req struct {
		Role string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	user, err := h.userRepo.FindByID(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户未找到"})
		return
	}

	user.Role = req.Role
	if err := h.userRepo.Update(ctx, user); err != nil {
		h.logger.Error("修改用户角色失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "修改角色失败"})
		return
	}

	// 修改角色后撤销所有 refresh token，使用户下次登录获取新权限
	_ = h.userRepo.RevokeAllUserRefreshTokens(ctx, user.ID)

	c.JSON(http.StatusOK, gin.H{
		"id":   user.ID,
		"role": user.Role,
	})
}
