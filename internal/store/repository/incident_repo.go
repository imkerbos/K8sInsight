package repository

import (
	"context"
	"time"

	"github.com/kerbos/k8sinsight/internal/store/model"
)

// IncidentRepository 事件数据访问接口
type IncidentRepository interface {
	Create(ctx context.Context, incident *model.Incident) error
	Update(ctx context.Context, incident *model.Incident) error
	FindByID(ctx context.Context, id string) (*model.Incident, error)
	FindActive(ctx context.Context, dedupKey string) (*model.Incident, error)
	List(ctx context.Context, opts ListOptions) (*ListResult, error)
}

// ListOptions 列表查询选项
type ListOptions struct {
	Namespace   string
	State       string
	AnomalyType string
	ClusterID   string
	OwnerName   string

	UseCursor      bool
	CursorLastSeen *time.Time
	CursorID       string
	IncludeTotal   bool

	Page     int
	PageSize int
}

type ListResult struct {
	Items              []model.Incident
	Total              *int64
	HasMore            bool
	NextCursorLastSeen *time.Time
	NextCursorID       string
}
