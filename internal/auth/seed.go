package auth

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/kerbos/k8sinsight/internal/store/model"
	"github.com/kerbos/k8sinsight/internal/store/repository"
)

// AllPermissions 所有可用权限点
var AllPermissions = []string{
	"dashboard:view",
	"incident:read",
	"cluster:read",
	"cluster:write",
	"rule:read",
	"rule:write",
	"user:manage",
	"role:manage",
	"settings:manage",
}

// builtInRoles 内置角色定义
var builtInRoles = []struct {
	Name        string
	Description string
	Permissions []string
}{
	{
		Name:        "admin",
		Description: "系统管理员，拥有全部权限",
		Permissions: AllPermissions,
	},
	{
		Name:        "operator",
		Description: "运维人员，可管理集群和监控规则",
		Permissions: []string{
			"dashboard:view",
			"incident:read",
			"cluster:read",
			"cluster:write",
			"rule:read",
			"rule:write",
		},
	},
	{
		Name:        "viewer",
		Description: "只读用户，仅可查看信息",
		Permissions: []string{
			"dashboard:view",
			"incident:read",
			"cluster:read",
			"rule:read",
		},
	},
}

// SeedDefaultAdmin 当数据库中无用户时，创建默认 admin 用户
func SeedDefaultAdmin(ctx context.Context, userRepo repository.UserRepository, password string, logger *zap.Logger) error {
	count, err := userRepo.Count(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		logger.Debug("已存在用户，跳过默认管理员创建")
		return nil
	}

	hash, err := HashPassword(password)
	if err != nil {
		return err
	}

	user := &model.User{
		ID:           uuid.New().String(),
		Username:     "admin",
		PasswordHash: hash,
		Role:         "admin",
		IsActive:     true,
	}

	if err := userRepo.Create(ctx, user); err != nil {
		return err
	}

	logger.Info("默认管理员用户已创建", zap.String("username", "admin"))
	return nil
}

// SeedBuiltInRoles 创建内置角色（如果不存在则创建，已存在则更新权限）
func SeedBuiltInRoles(ctx context.Context, roleRepo repository.RoleRepository, logger *zap.Logger) error {
	for _, def := range builtInRoles {
		permsJSON, err := json.Marshal(def.Permissions)
		if err != nil {
			return err
		}

		existing, err := roleRepo.FindByName(ctx, def.Name)
		if err != nil {
			// 角色不存在，创建
			role := &model.Role{
				ID:          uuid.New().String(),
				Name:        def.Name,
				Description: def.Description,
				Permissions: string(permsJSON),
				BuiltIn:     true,
			}
			if err := roleRepo.Create(ctx, role); err != nil {
				return err
			}
			logger.Info("内置角色已创建", zap.String("name", def.Name))
		} else {
			// 角色已存在，更新权限
			existing.Permissions = string(permsJSON)
			existing.Description = def.Description
			existing.BuiltIn = true
			if err := roleRepo.Update(ctx, existing); err != nil {
				return err
			}
			logger.Debug("内置角色已更新", zap.String("name", def.Name))
		}
	}
	return nil
}
