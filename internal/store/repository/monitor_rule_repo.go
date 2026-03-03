package repository

import (
	"context"

	"gorm.io/gorm"

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

type monitorRuleRepo struct {
	db *gorm.DB
}

func NewMonitorRuleRepository(db *gorm.DB) MonitorRuleRepository {
	return &monitorRuleRepo{db: db}
}

func (r *monitorRuleRepo) Create(ctx context.Context, rule *model.MonitorRule) error {
	return r.db.WithContext(ctx).Create(rule).Error
}

func (r *monitorRuleRepo) Update(ctx context.Context, rule *model.MonitorRule) error {
	return r.db.WithContext(ctx).Save(rule).Error
}

func (r *monitorRuleRepo) FindByID(ctx context.Context, id string) (*model.MonitorRule, error) {
	var m model.MonitorRule
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *monitorRuleRepo) FindByClusterID(ctx context.Context, clusterID string) (*model.MonitorRule, error) {
	var m model.MonitorRule
	err := r.db.WithContext(ctx).Where("cluster_id = ?", clusterID).First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *monitorRuleRepo) List(ctx context.Context) ([]model.MonitorRule, error) {
	var rules []model.MonitorRule
	err := r.db.WithContext(ctx).Order("created_at ASC").Find(&rules).Error
	return rules, err
}

func (r *monitorRuleRepo) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.MonitorRule{}).Error
}
