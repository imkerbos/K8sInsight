package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

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

// incidentRepo IncidentRepository 的 GORM 实现
type incidentRepo struct {
	db *gorm.DB
}

// NewIncidentRepository 创建事件仓储
func NewIncidentRepository(db *gorm.DB) IncidentRepository {
	return &incidentRepo{db: db}
}

func (r *incidentRepo) Create(ctx context.Context, incident *model.Incident) error {
	return r.db.WithContext(ctx).Create(incident).Error
}

func (r *incidentRepo) Update(ctx context.Context, incident *model.Incident) error {
	return r.db.WithContext(ctx).Save(incident).Error
}

func (r *incidentRepo) FindByID(ctx context.Context, id string) (*model.Incident, error) {
	var m model.Incident
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *incidentRepo) FindActive(ctx context.Context, dedupKey string) (*model.Incident, error) {
	var m model.Incident
	err := r.db.WithContext(ctx).
		Where("dedup_key = ? AND state IN ?", dedupKey, []string{"Detecting", "Active"}).
		First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *incidentRepo) List(ctx context.Context, opts ListOptions) (*ListResult, error) {
	query := r.db.WithContext(ctx).Model(&model.Incident{})

	if opts.Namespace != "" {
		query = query.Where("namespace = ?", opts.Namespace)
	}
	if opts.State != "" {
		query = query.Where("state = ?", opts.State)
	}
	if opts.AnomalyType != "" {
		query = query.Where("anomaly_type = ?", opts.AnomalyType)
	}
	if opts.ClusterID != "" {
		query = query.Where("cluster_id = ?", opts.ClusterID)
	}
	if opts.OwnerName != "" {
		query = query.Where("owner_name ILIKE ?", "%"+opts.OwnerName+"%")
	}

	var total *int64
	if opts.IncludeTotal {
		var c int64
		if err := query.Count(&c).Error; err != nil {
			return nil, err
		}
		total = &c
	}

	pageSize := opts.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	if opts.UseCursor && opts.CursorLastSeen != nil && opts.CursorID != "" {
		query = query.Where(
			"(last_seen < ?) OR (last_seen = ? AND id < ?)",
			*opts.CursorLastSeen, *opts.CursorLastSeen, opts.CursorID,
		)
	}

	var rows []model.Incident
	query = query.Order("last_seen DESC, id DESC").Limit(pageSize + 1)

	if !opts.UseCursor {
		page := opts.Page
		if page < 1 {
			page = 1
		}
		query = query.Offset((page - 1) * pageSize)
	}

	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}

	hasMore := len(rows) > pageSize
	items := rows
	if hasMore {
		items = rows[:pageSize]
	}

	var nextCursorLastSeen *time.Time
	var nextCursorID string
	if hasMore && len(items) > 0 {
		last := items[len(items)-1]
		nextCursorLastSeen = &last.LastSeen
		nextCursorID = last.ID
	}

	return &ListResult{
		Items:              items,
		Total:              total,
		HasMore:            hasMore,
		NextCursorLastSeen: nextCursorLastSeen,
		NextCursorID:       nextCursorID,
	}, nil
}
