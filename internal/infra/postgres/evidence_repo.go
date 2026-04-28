package postgres

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/kerbos/k8sinsight/internal/domain"
	"github.com/kerbos/k8sinsight/internal/store/model"
	"github.com/kerbos/k8sinsight/internal/store/repository"
)

type evidenceRepo struct {
	db *gorm.DB
}

func NewEvidenceRepository(db *gorm.DB) repository.EvidenceRepository {
	return &evidenceRepo{db: db}
}

func (r *evidenceRepo) SaveBundle(ctx context.Context, incidentID string, bundle *domain.EvidenceBundle) error {
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
