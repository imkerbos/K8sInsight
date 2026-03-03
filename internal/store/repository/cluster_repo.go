package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/kerbos/k8sinsight/internal/store/model"
)

// ClusterRepository 集群配置数据访问接口
type ClusterRepository interface {
	Create(ctx context.Context, cluster *model.Cluster) error
	Update(ctx context.Context, cluster *model.Cluster) error
	FindByID(ctx context.Context, id string) (*model.Cluster, error)
	FindActive(ctx context.Context) ([]model.Cluster, error)
	List(ctx context.Context) ([]model.Cluster, error)
	Delete(ctx context.Context, id string) error
}

// clusterRepo ClusterRepository 的 GORM 实现
type clusterRepo struct {
	db *gorm.DB
}

// NewClusterRepository 创建集群配置仓储
func NewClusterRepository(db *gorm.DB) ClusterRepository {
	return &clusterRepo{db: db}
}

func (r *clusterRepo) Create(ctx context.Context, cluster *model.Cluster) error {
	return r.db.WithContext(ctx).Create(cluster).Error
}

func (r *clusterRepo) Update(ctx context.Context, cluster *model.Cluster) error {
	return r.db.WithContext(ctx).Save(cluster).Error
}

func (r *clusterRepo) FindByID(ctx context.Context, id string) (*model.Cluster, error) {
	var m model.Cluster
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *clusterRepo) FindActive(ctx context.Context) ([]model.Cluster, error) {
	var clusters []model.Cluster
	err := r.db.WithContext(ctx).Where("status = ?", "active").Find(&clusters).Error
	return clusters, err
}

func (r *clusterRepo) List(ctx context.Context) ([]model.Cluster, error) {
	var clusters []model.Cluster
	err := r.db.WithContext(ctx).Order("created_at ASC").Find(&clusters).Error
	return clusters, err
}

func (r *clusterRepo) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.Cluster{}).Error
}
