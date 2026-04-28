package repository

import (
	"context"

	"github.com/kerbos/k8sinsight/internal/domain"
	"github.com/kerbos/k8sinsight/internal/store/model"
)

// EvidenceRepository 证据数据访问接口
type EvidenceRepository interface {
	SaveBundle(ctx context.Context, incidentID string, bundle *domain.EvidenceBundle) error
	FindByIncidentID(ctx context.Context, incidentID string) ([]model.Evidence, error)
}
