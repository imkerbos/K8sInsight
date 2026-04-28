package repository

import (
	"context"

	"github.com/kerbos/k8sinsight/internal/store/model"
)

// MonitorRuleRepository 监控规则数据访问接口
type MonitorRuleRepository interface {
	Create(ctx context.Context, rule *model.MonitorRule) error
	Update(ctx context.Context, rule *model.MonitorRule) error
	FindByID(ctx context.Context, id string) (*model.MonitorRule, error)
	FindByClusterID(ctx context.Context, clusterID string) (*model.MonitorRule, error)
	List(ctx context.Context) ([]model.MonitorRule, error)
	Delete(ctx context.Context, id string) error
}
