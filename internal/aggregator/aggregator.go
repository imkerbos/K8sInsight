package aggregator

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/kerbos/k8sinsight/internal/collector"
	"github.com/kerbos/k8sinsight/internal/config"
	"github.com/kerbos/k8sinsight/internal/detector"
	"github.com/kerbos/k8sinsight/internal/store/model"
	"github.com/kerbos/k8sinsight/internal/store/repository"
)

// Aggregator 去重聚合引擎
type Aggregator struct {
	index         *dedupIndex
	cfg           config.AggregationConfig
	incidentRepo  repository.IncidentRepository
	evidenceRepo  repository.EvidenceRepository
	notifier      detector.EventSink
	logger        *zap.Logger
	evidenceInput <-chan collector.EvidenceBundle
}

// NewAggregator 创建聚合引擎
func NewAggregator(
	cfg config.AggregationConfig,
	incidentRepo repository.IncidentRepository,
	evidenceRepo repository.EvidenceRepository,
	notifier detector.EventSink,
	evidenceInput <-chan collector.EvidenceBundle,
	logger *zap.Logger,
) *Aggregator {
	return &Aggregator{
		index:         newDedupIndex(),
		cfg:           cfg,
		incidentRepo:  incidentRepo,
		evidenceRepo:  evidenceRepo,
		notifier:      notifier,
		logger:        logger.Named("aggregator"),
		evidenceInput: evidenceInput,
	}
}

// HandleAnomaly 实现 detector.EventSink 接口
func (a *Aggregator) HandleAnomaly(ctx context.Context, event detector.AnomalyEvent) error {
	dedupKey := event.DedupKey()

	// 查找活跃事件
	if inc, found := a.index.findActive(dedupKey); found {
		inc.LastSeen = time.Now()
		inc.Count++
		inc.Message = event.Message
		if !contains(inc.PodNames, event.PodName) {
			inc.PodNames = append(inc.PodNames, event.PodName)
		}
		a.logger.Info("异常合并到活跃事件",
			zap.String("incidentId", inc.ID),
			zap.String("dedupKey", dedupKey),
			zap.Int("count", inc.Count),
		)
		return a.incidentRepo.Update(ctx, toModel(inc))
	}

	// 创建新事件
	inc := &Incident{
		ID:          uuid.New().String(),
		DedupKey:    dedupKey,
		State:       StateDetecting,
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

	a.index.upsert(inc)
	a.logger.Info("创建新事件",
		zap.String("incidentId", inc.ID),
		zap.String("dedupKey", dedupKey),
	)

	if err := a.incidentRepo.Create(ctx, toModel(inc)); err != nil {
		return err
	}

	go a.groupWaitTimer(context.Background(), inc)

	return nil
}

// Start 启动证据消费和活跃窗口检查协程
func (a *Aggregator) Start(ctx context.Context) {
	go a.consumeEvidence(ctx)
	go a.checkActiveWindow(ctx)
}

func (a *Aggregator) consumeEvidence(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case bundle, ok := <-a.evidenceInput:
			if !ok {
				return
			}
			dedupKey := bundle.AnomalyEvent.DedupKey()
			inc, found := a.index.findActive(dedupKey)
			if !found {
				a.logger.Warn("证据对应的事件未找到", zap.String("dedupKey", dedupKey))
				continue
			}
			if err := a.evidenceRepo.SaveBundle(ctx, inc.ID, &bundle); err != nil {
				a.logger.Error("保存证据失败", zap.Error(err))
			}

			// 证据驱动通知：基于已采集证据生成数据化结论后再发送
			if a.notifier != nil {
				enriched := buildEvidenceDrivenEvent(inc.ID, bundle.AnomalyEvent, bundle.Evidences)
				if err := a.notifier.HandleAnomaly(ctx, enriched); err != nil {
					a.logger.Error("发送证据驱动通知失败", zap.Error(err))
				}
			}
		}
	}
}

func (a *Aggregator) groupWaitTimer(ctx context.Context, inc *Incident) {
	wait := a.cfg.GroupWait
	if wait == 0 {
		wait = 30 * time.Second
	}

	select {
	case <-time.After(wait):
		if inc.State == StateDetecting {
			inc.State = StateActive
			a.index.upsert(inc)
			_ = a.incidentRepo.Update(ctx, toModel(inc))
			a.logger.Info("事件转为 Active",
				zap.String("incidentId", inc.ID),
			)
		}
	case <-ctx.Done():
	}
}

func (a *Aggregator) checkActiveWindow(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	window := a.cfg.ActiveWindow
	if window == 0 {
		window = 6 * time.Hour
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, inc := range a.index.allActive() {
				if time.Since(inc.LastSeen) > window {
					inc.State = StateResolved
					a.index.remove(inc.DedupKey)
					_ = a.incidentRepo.Update(ctx, toModel(inc))
					a.logger.Info("事件自动 Resolved",
						zap.String("incidentId", inc.ID),
					)
				}
			}
		}
	}
}

// toModel 将内存 Incident 转换为数据库模型
func toModel(inc *Incident) *model.Incident {
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
