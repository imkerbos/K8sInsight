package repository

import (
	"context"
	"time"

	"github.com/kerbos/k8sinsight/internal/store/model"
)

// ClusterRepository 集群配置数据访问接口
type ClusterRepository interface {
	Create(ctx context.Context, cluster *model.Cluster) error
	Update(ctx context.Context, cluster *model.Cluster) error
	UpdateLastEventTime(ctx context.Context, clusterID string, t time.Time) error
	FindByID(ctx context.Context, id string) (*model.Cluster, error)
	FindActive(ctx context.Context) ([]model.Cluster, error)
	List(ctx context.Context) ([]model.Cluster, error)
	Delete(ctx context.Context, id string) error
}
