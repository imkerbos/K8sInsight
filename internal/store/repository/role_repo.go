package repository

import (
	"context"

	"github.com/kerbos/k8sinsight/internal/store/model"
)

// RoleRepository 角色数据访问接口
type RoleRepository interface {
	Create(ctx context.Context, role *model.Role) error
	FindByID(ctx context.Context, id string) (*model.Role, error)
	FindByName(ctx context.Context, name string) (*model.Role, error)
	List(ctx context.Context) ([]model.Role, error)
	Update(ctx context.Context, role *model.Role) error
	Delete(ctx context.Context, id string) error
}
