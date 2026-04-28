package incident

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/kerbos/k8sinsight/internal/config"
	"github.com/kerbos/k8sinsight/internal/domain"
	"github.com/kerbos/k8sinsight/internal/pipeline/dedup"
	"github.com/kerbos/k8sinsight/internal/store/model"
	"github.com/kerbos/k8sinsight/internal/store/repository"
)

// Manager 管理 Incident 生命周期：创建、去重、状态转换、持久化
type Manager struct {
	index    *dedup.Index
	repo     repository.IncidentRepository
	cfg      config.AggregationConfig
	logger   *zap.Logger
}

// NewManager 创建 Incident 管理器
func NewManager(
	index *dedup.Index,
	repo repository.IncidentRepository,
	cfg config.AggregationConfig,
	logger *zap.Logger,
) *Manager {
	return &Manager{
		index:  index,
		repo:   repo,
		cfg:    cfg,
		logger: logger.Named("incident-mgr"),
	}
}

// Index 返回底层去重索引（供 EvidenceProcessor 使用）
func (m *Manager) Index() *dedup.Index {
	return m.index
}

// HandleAnomaly 接收异常事件，执行去重和状态管理
func (m *Manager) HandleAnomaly(ctx context.Context, event domain.AnomalyEvent) error {
	dedupKey := event.DedupKey()

	// 查找活跃事件
	if inc, found := m.index.FindActive(dedupKey); found {
		inc.LastSeen = time.Now()
		inc.Count++
		inc.Message = event.Message
		if !contains(inc.PodNames, event.PodName) {
			inc.PodNames = append(inc.PodNames, event.PodName)
		}
		m.logger.Info("异常合并到活跃事件",
			zap.String("incidentId", inc.ID),
			zap.String("dedupKey", dedupKey),
			zap.Int("count", inc.Count),
		)
		return m.repo.Update(ctx, toModel(inc))
	}

	// 创建新事件
	inc := &domain.Incident{
		ID:          uuid.New().String(),
		DedupKey:    dedupKey,
		State:       domain.StateDetecting,
		FirstSeen:   time.Now(),
		LastSeen:    time.Now(),
		Count:       1,
		Namespace:   event.Namespace,
		OwnerKind:   event.OwnerKind,
		OwnerName:   event.OwnerName,
		AnomalyType: string(event.Type),
		Message:     event.Message,
		PodNames:    []string{event.PodName},
		ClusterID:   event.ClusterID,
	}

	m.index.Upsert(inc)
	m.logger.Info("创建新事件",
		zap.String("incidentId", inc.ID),
		zap.String("dedupKey", dedupKey),
	)

	if err := m.repo.Create(ctx, toModel(inc)); err != nil {
		return err
	}

	go m.groupWaitTimer(ctx, inc)

	return nil
}

// Start 启动后台协程：活跃窗口检查
func (m *Manager) Start(ctx context.Context) {
	go m.checkActiveWindow(ctx)
}

func (m *Manager) groupWaitTimer(ctx context.Context, inc *domain.Incident) {
	wait := m.cfg.GroupWait
	if wait == 0 {
		wait = 30 * time.Second
	}

	select {
	case <-time.After(wait):
		if inc.State == domain.StateDetecting {
			inc.State = domain.StateActive
			m.index.Upsert(inc)
			_ = m.repo.Update(ctx, toModel(inc))
			m.logger.Info("事件转为 Active",
				zap.String("incidentId", inc.ID),
			)
		}
	case <-ctx.Done():
	}
}

func (m *Manager) checkActiveWindow(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	window := m.cfg.ActiveWindow
	if window == 0 {
		window = 6 * time.Hour
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			toResolve := m.index.Sweep(window)
			for _, inc := range toResolve {
				inc.State = domain.StateResolved
				m.index.Remove(inc.DedupKey)
				_ = m.repo.Update(ctx, toModel(inc))
				m.logger.Info("事件自动 Resolved",
					zap.String("incidentId", inc.ID),
				)
			}
		}
	}
}

// toModel 将内存 Incident 转换为数据库模型
func toModel(inc *domain.Incident) *model.Incident {
	podNamesJSON, _ := json.Marshal(inc.PodNames)
	m := &model.Incident{
		ID:          inc.ID,
		DedupKey:    inc.DedupKey,
		State:       string(inc.State),
		FirstSeen:   inc.FirstSeen,
		LastSeen:    inc.LastSeen,
		Count:       inc.Count,
		Namespace:   inc.Namespace,
		OwnerKind:   inc.OwnerKind,
		OwnerName:   inc.OwnerName,
		AnomalyType: inc.AnomalyType,
		Message:     inc.Message,
		PodNames:    string(podNamesJSON),
	}
	if inc.ClusterID != "" {
		m.ClusterID = &inc.ClusterID
	}
	return m
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
