package handler

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/kerbos/k8sinsight/internal/api/middleware"
	authpkg "github.com/kerbos/k8sinsight/internal/auth"
	"github.com/kerbos/k8sinsight/internal/store/model"
	"github.com/kerbos/k8sinsight/internal/store/repository"
)

// RoleHandler 角色管理 API 处理器
type RoleHandler struct {
	roleRepo    repository.RoleRepository
	permChecker *middleware.PermissionChecker
	logger      *zap.Logger
}

// NewRoleHandler 创建角色管理处理器
func NewRoleHandler(roleRepo repository.RoleRepository, permChecker *middleware.PermissionChecker, logger *zap.Logger) *RoleHandler {
	return &RoleHandler{
		roleRepo:    roleRepo,
		permChecker: permChecker,
		logger:      logger.Named("api.role"),
	}
}

type roleResponse struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
	BuiltIn     bool     `json:"builtIn"`
	CreatedAt   string   `json:"createdAt"`
	UpdatedAt   string   `json:"updatedAt"`
}

func toRoleResponse(r *model.Role) roleResponse {
	var perms []string
	_ = json.Unmarshal([]byte(r.Permissions), &perms)
	if perms == nil {
		perms = []string{}
	}
	return roleResponse{
		ID:          r.ID,
		Name:        r.Name,
		Description: r.Description,
		Permissions: perms,
		BuiltIn:     r.BuiltIn,
		CreatedAt:   r.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:   r.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// List 获取角色列表
func (h *RoleHandler) List(c *gin.Context) {
	roles, err := h.roleRepo.List(c.Request.Context())
	if err != nil {
		h.logger.Error("查询角色列表失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	items := make([]roleResponse, 0, len(roles))
	for i := range roles {
		items = append(items, toRoleResponse(&roles[i]))
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

type createRoleRequest struct {
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions" binding:"required"`
}

// Create 创建角色
func (h *RoleHandler) Create(c *gin.Context) {
	var req createRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	ctx := c.Request.Context()

	// 检查名称是否重复
	if existing, _ := h.roleRepo.FindByName(ctx, req.Name); existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "角色名称已存在"})
		return
	}

	// 校验权限点
	if !validatePermissions(req.Permissions) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "包含无效的权限点"})
		return
	}

	permsJSON, _ := json.Marshal(req.Permissions)
	role := &model.Role{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Permissions: string(permsJSON),
		BuiltIn:     false,
	}

	if err := h.roleRepo.Create(ctx, role); err != nil {
		h.logger.Error("创建角色失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建角色失败"})
		return
	}

	// 清除权限缓存
	h.permChecker.ClearCache()

	c.JSON(http.StatusCreated, toRoleResponse(role))
}

type updateRoleRequest struct {
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions" binding:"required"`
}

// Update 更新角色
func (h *RoleHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var req updateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	ctx := c.Request.Context()
	role, err := h.roleRepo.FindByID(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "角色未找到"})
		return
	}

	// 内置角色不允许修改名称
	if role.BuiltIn && req.Name != role.Name {
		c.JSON(http.StatusBadRequest, gin.H{"error": "内置角色不可修改名称"})
		return
	}

	// 检查名称是否和其他角色冲突
	if req.Name != role.Name {
		if existing, _ := h.roleRepo.FindByName(ctx, req.Name); existing != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "角色名称已存在"})
			return
		}
	}

	if !validatePermissions(req.Permissions) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "包含无效的权限点"})
		return
	}

	permsJSON, _ := json.Marshal(req.Permissions)
	role.Name = req.Name
	role.Description = req.Description
	role.Permissions = string(permsJSON)

	if err := h.roleRepo.Update(ctx, role); err != nil {
		h.logger.Error("更新角色失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新角色失败"})
		return
	}

	h.permChecker.ClearCache()
	c.JSON(http.StatusOK, toRoleResponse(role))
}

// Delete 删除角色
func (h *RoleHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	ctx := c.Request.Context()

	role, err := h.roleRepo.FindByID(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "角色未找到"})
		return
	}

	if role.BuiltIn {
		c.JSON(http.StatusBadRequest, gin.H{"error": "内置角色不可删除"})
		return
	}

	if err := h.roleRepo.Delete(ctx, id); err != nil {
		h.logger.Error("删除角色失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除角色失败"})
		return
	}

	h.permChecker.ClearCache()
	c.JSON(http.StatusOK, gin.H{"message": "角色已删除"})
}

// ListPermissions 返回所有可用权限点
func (h *RoleHandler) ListPermissions(c *gin.Context) {
	type permItem struct {
		Key         string `json:"key"`
		Description string `json:"description"`
	}

	perms := []permItem{
		{Key: "dashboard:view", Description: "查看仪表盘"},
		{Key: "incident:read", Description: "查看异常事件"},
		{Key: "cluster:read", Description: "查看集群"},
		{Key: "cluster:write", Description: "管理集群"},
		{Key: "rule:read", Description: "查看监控规则"},
		{Key: "rule:write", Description: "管理监控规则"},
		{Key: "user:manage", Description: "用户管理"},
		{Key: "role:manage", Description: "角色管理"},
		{Key: "settings:manage", Description: "系统设置管理"},
	}

	c.JSON(http.StatusOK, gin.H{"items": perms})
}

// validatePermissions 校验权限点是否都在合法列表中
func validatePermissions(perms []string) bool {
	allowed := make(map[string]bool, len(authpkg.AllPermissions))
	for _, p := range authpkg.AllPermissions {
		allowed[p] = true
	}
	for _, p := range perms {
		if !allowed[p] {
			return false
		}
	}
	return true
}
