package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/kerbos/k8sinsight/internal/collector"
	"github.com/kerbos/k8sinsight/internal/domain"
	"github.com/kerbos/k8sinsight/internal/store/model"
	"github.com/kerbos/k8sinsight/internal/store/repository"
)

// IncidentService 异常事件业务逻辑
type IncidentService struct {
	repo         repository.IncidentRepository
	evidenceRepo repository.EvidenceRepository
	settingRepo  repository.SettingRepository
	clusterRepo  repository.ClusterRepository
	logger       *zap.Logger
}

// NewIncidentService 创建事件服务
func NewIncidentService(
	repo repository.IncidentRepository,
	evidenceRepo repository.EvidenceRepository,
	settingRepo repository.SettingRepository,
	clusterRepo repository.ClusterRepository,
	logger *zap.Logger,
) *IncidentService {
	return &IncidentService{
		repo:         repo,
		evidenceRepo: evidenceRepo,
		settingRepo:  settingRepo,
		clusterRepo:  clusterRepo,
		logger:       logger.Named("svc.incident"),
	}
}

// List 查询事件列表
func (s *IncidentService) List(ctx context.Context, opts repository.ListOptions) (*repository.ListResult, error) {
	return s.repo.List(ctx, opts)
}

// GetByID 查询事件详情
func (s *IncidentService) GetByID(ctx context.Context, id string) (*model.Incident, error) {
	return s.repo.FindByID(ctx, id)
}

// GetEvidences 查询事件关联的证据
func (s *IncidentService) GetEvidences(ctx context.Context, incidentID string) ([]model.Evidence, error) {
	return s.evidenceRepo.FindByIncidentID(ctx, incidentID)
}

// RecollectMetrics 手动补采单个事件的 Prometheus 指标
func (s *IncidentService) RecollectMetrics(ctx context.Context, incidentID string) (*RecollectResult, error) {
	incident, err := s.repo.FindByID(ctx, incidentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("事件未找到")
		}
		return nil, fmt.Errorf("查询事件失败: %w", err)
	}

	podName := firstPodName(incident.PodNames)
	if podName == "" {
		return nil, fmt.Errorf("事件缺少 Pod 信息，无法补采指标")
	}

	promURL, promQueryRange, err := s.loadPrometheusReplayConfig(ctx)
	if err != nil {
		return nil, err
	}

	// 如果事件关联了集群，优先使用集群级别的 Prometheus 配置
	var extraLabels string
	if incident.ClusterID != nil && *incident.ClusterID != "" && s.clusterRepo != nil {
		if cl, clErr := s.clusterRepo.FindByID(ctx, *incident.ClusterID); clErr == nil {
			if u := strings.TrimSpace(cl.PrometheusURL); u != "" {
				promURL = u
			}
			extraLabels = strings.TrimSpace(cl.PrometheusLabels)
		}
	}

	eventTs := incident.LastSeen
	if eventTs.IsZero() {
		eventTs = incident.FirstSeen
	}
	if eventTs.IsZero() {
		eventTs = time.Now()
	}

	event := domain.AnomalyEvent{
		ID:        "recollect-" + incident.ID,
		Timestamp: eventTs,
		Namespace: incident.Namespace,
		PodName:   podName,
	}

	queryCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	content, err := collector.CollectPrometheusRange(queryCtx, promURL, promQueryRange, event, extraLabels)
	if err != nil {
		return nil, fmt.Errorf("Prometheus 补采失败: %w", err)
	}

	now := time.Now()
	bundle := &domain.EvidenceBundle{
		AnomalyEvent: event,
		Evidences: []domain.Evidence{
			{
				Type:      domain.EvidenceMetrics,
				Content:   content,
				Timestamp: now,
			},
		},
		CollectedAt: now,
	}

	if err := s.evidenceRepo.SaveBundle(ctx, incident.ID, bundle); err != nil {
		return nil, fmt.Errorf("保存补采证据失败: %w", err)
	}

	return &RecollectResult{
		IncidentID:    incidentID,
		PodName:       podName,
		CollectedAt:   now,
		PrometheusURL: promURL,
	}, nil
}

// RecollectResult 补采结果
type RecollectResult struct {
	IncidentID    string    `json:"incidentId"`
	PodName       string    `json:"podName"`
	CollectedAt   time.Time `json:"collectedAt"`
	PrometheusURL string    `json:"prometheusURL"`
}

func (s *IncidentService) loadPrometheusReplayConfig(ctx context.Context) (string, time.Duration, error) {
	settings, err := s.settingRepo.BatchGet(ctx, []string{
		"collect_prometheus_url",
		"collect_prom_query_range",
	})
	if err != nil {
		return "", 0, fmt.Errorf("读取资源采集设置失败")
	}

	promURL := strings.TrimSpace(settings["collect_prometheus_url"])
	if promURL == "" {
		return "", 0, fmt.Errorf("未配置 Prometheus 地址，请先在资源采集配置中保存")
	}

	promRangeRaw := strings.TrimSpace(settings["collect_prom_query_range"])
	if promRangeRaw == "" {
		promRangeRaw = "10m"
	}
	promRange, err := time.ParseDuration(promRangeRaw)
	if err != nil {
		return "", 0, fmt.Errorf("Prometheus 查询时间窗配置无效")
	}

	return promURL, promRange, nil
}

func firstPodName(rawPodNames string) string {
	if strings.TrimSpace(rawPodNames) == "" {
		return ""
	}
	var pods []string
	if err := json.Unmarshal([]byte(rawPodNames), &pods); err != nil {
		return ""
	}
	if len(pods) == 0 {
		return ""
	}
	return strings.TrimSpace(pods[0])
}
