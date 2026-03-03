package middleware

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/kerbos/k8sinsight/internal/auth"
	"github.com/kerbos/k8sinsight/internal/store/repository"
)

const (
	contextKeyUserID   = "userID"
	contextKeyUsername = "username"
	contextKeyRole     = "role"
)

// JWTAuth JWT 认证中间件
func JWTAuth(tokenService *auth.TokenService) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "缺少认证信息"})
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "认证格式错误"})
			return
		}

		claims, err := tokenService.ValidateAccessToken(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "令牌无效或已过期"})
			return
		}

		c.Set(contextKeyUserID, claims.UserID)
		c.Set(contextKeyUsername, claims.Username)
		c.Set(contextKeyRole, claims.Role)
		c.Next()
	}
}

// GetUserID 从 gin.Context 获取当前用户 ID
func GetUserID(c *gin.Context) string {
	v, _ := c.Get(contextKeyUserID)
	s, _ := v.(string)
	return s
}

// GetUsername 从 gin.Context 获取当前用户名
func GetUsername(c *gin.Context) string {
	v, _ := c.Get(contextKeyUsername)
	s, _ := v.(string)
	return s
}

// GetRole 从 gin.Context 获取当前用户角色
func GetRole(c *gin.Context) string {
	v, _ := c.Get(contextKeyRole)
	s, _ := v.(string)
	return s
}

// PermissionChecker 权限检查器，带内存缓存
type PermissionChecker struct {
	roleRepo repository.RoleRepository
	cache    sync.Map // map[roleName][]string
}

// NewPermissionChecker 创建权限检查器
func NewPermissionChecker(roleRepo repository.RoleRepository) *PermissionChecker {
	return &PermissionChecker{roleRepo: roleRepo}
}

// ClearCache 清除缓存（角色更新时调用）
func (pc *PermissionChecker) ClearCache() {
	pc.cache.Range(func(key, _ any) bool {
		pc.cache.Delete(key)
		return true
	})
}

// getPermissions 获取角色权限（优先从缓存）
func (pc *PermissionChecker) getPermissions(c *gin.Context, roleName string) ([]string, error) {
	if cached, ok := pc.cache.Load(roleName); ok {
		return cached.([]string), nil
	}

	role, err := pc.roleRepo.FindByName(c.Request.Context(), roleName)
	if err != nil {
		return nil, err
	}

	var perms []string
	if err := json.Unmarshal([]byte(role.Permissions), &perms); err != nil {
		return nil, err
	}

	pc.cache.Store(roleName, perms)
	return perms, nil
}

// RequirePermission 返回一个检查指定权限的中间件
func (pc *PermissionChecker) RequirePermission(perm string) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleName := GetRole(c)
		if roleName == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "无权限"})
			return
		}

		perms, err := pc.getPermissions(c, roleName)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "无权限"})
			return
		}

		for _, p := range perms {
			if p == perm {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "无权限执行此操作"})
	}
}
