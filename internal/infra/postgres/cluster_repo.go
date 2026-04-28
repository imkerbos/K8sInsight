package postgres

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/kerbos/k8sinsight/internal/store/model"
	"github.com/kerbos/k8sinsight/internal/store/repository"
)

type clusterRepo struct {
	db *gorm.DB
}

func NewClusterRepository(db *gorm.DB) repository.ClusterRepository {
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

func (r *clusterRepo) UpdateLastEventTime(ctx context.Context, clusterID string, t time.Time) error {
	return r.db.WithContext(ctx).
		Model(&model.Cluster{}).
		Where("id = ?", clusterID).
		Update("last_event_time", t).Error
}

func (r *clusterRepo) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.Cluster{}).Error
}
