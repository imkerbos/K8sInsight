package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/kerbos/k8sinsight/internal/collector"
	"github.com/kerbos/k8sinsight/internal/store/model"
)

// EvidenceRepository 证据数据访问接口
type EvidenceRepository interface {
	SaveBundle(ctx context.Context, incidentID string, bundle *collector.EvidenceBundle) error
	FindByIncidentID(ctx context.Context, incidentID string) ([]model.Evidence, error)
}

// evidenceRepo EvidenceRepository 的 GORM 实现
type evidenceRepo struct {
	db *gorm.DB
}

// NewEvidenceRepository 创建证据仓储
func NewEvidenceRepository(db *gorm.DB) EvidenceRepository {
	return &evidenceRepo{db: db}
}

func (r *evidenceRepo) SaveBundle(ctx context.Context, incidentID string, bundle *collector.EvidenceBundle) error {
	var models []model.Evidence
	for _, e := range bundle.Evidences {
		models = append(models, model.Evidence{
			ID:          uuid.New().String(),
			IncidentID:  incidentID,
			Type:        string(e.Type),
			Content:     e.Content,
			Error:       e.Error,
			CollectedAt: e.Timestamp,
		})
	}

	if len(models) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).Create(&models).Error
}

func (r *evidenceRepo) FindByIncidentID(ctx context.Context, incidentID string) ([]model.Evidence, error) {
	var evidences []model.Evidence
	err := r.db.WithContext(ctx).
		Where("incident_id = ?", incidentID).
		Order("collected_at ASC").
		Find(&evidences).Error
	return evidences, err
}
