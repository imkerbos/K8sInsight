package collector

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"

	"github.com/kerbos/k8sinsight/internal/config"
	"github.com/kerbos/k8sinsight/internal/detector"
)

// Evidence 采集到的证据
type Evidence struct {
	Type      EvidenceType `json:"type"`
	Content   string       `json:"content"`
	Timestamp time.Time    `json:"timestamp"`
	Error     string       `json:"error,omitempty"`
}

// EvidenceType 证据类型
type EvidenceType string

const (
	EvidencePreviousLogs EvidenceType = "PreviousLogs"
	EvidenceCurrentLogs  EvidenceType = "CurrentLogs"
	EvidencePodEvents    EvidenceType = "PodEvents"
	EvidencePodSnapshot  EvidenceType = "PodSnapshot"
	EvidenceMetrics      EvidenceType = "Metrics"
)

// EvidenceBundle 一次异常采集到的全部证据
type EvidenceBundle struct {
	AnomalyEvent detector.AnomalyEvent
	Evidences    []Evidence
	CollectedAt  time.Time
}

// Collector 证据采集编排器
type Collector struct {
	clientset kubernetes.Interface
	cfg       config.CollectConfig
	logger    *zap.Logger
	output    chan<- EvidenceBundle
}

// NewCollector 创建证据采集器
func NewCollector(clientset kubernetes.Interface, cfg config.CollectConfig, output chan<- EvidenceBundle, logger *zap.Logger) *Collector {
	return &Collector{
		clientset: clientset,
		cfg:       cfg,
		logger:    logger.Named("collector"),
		output:    output,
	}
}

// HandleAnomaly 实现 detector.EventSink 接口
// 检测到异常后立即触发证据采集
func (c *Collector) HandleAnomaly(ctx context.Context, event detector.AnomalyEvent) error {
	go c.collect(ctx, event)
	return nil
}

// collect 并行采集所有证据
func (c *Collector) collect(ctx context.Context, event detector.AnomalyEvent) {
	c.logger.Info("开始采集证据",
		zap.String("pod", event.PodName),
		zap.String("namespace", event.Namespace),
		zap.String("type", string(event.Type)),
	)

	var (
		mu        sync.Mutex
		evidences []Evidence
		wg        sync.WaitGroup
	)

	addEvidence := func(e Evidence) {
		mu.Lock()
		evidences = append(evidences, e)
		mu.Unlock()
	}

	timeout := c.cfg.TimeoutPerItem
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	// P0: 前一个容器日志 + 当前容器日志（并行）
	wg.Add(2)
	go func() {
		defer wg.Done()
		e := collectPreviousLogs(ctx, c.clientset, event, timeout)
		addEvidence(e)
	}()
	go func() {
		defer wg.Done()
		e := collectCurrentLogs(ctx, c.clientset, event, c.cfg.LogTailLines, timeout)
		addEvidence(e)
	}()

	// P1: Pod Events + Pod Snapshot（并行）
	wg.Add(2)
	go func() {
		defer wg.Done()
		e := collectPodEvents(ctx, c.clientset, event, timeout)
		addEvidence(e)
	}()
	go func() {
		defer wg.Done()
		e := collectPodSnapshot(event)
		addEvidence(e)
	}()

	// P2: 资源指标（可选）
	if c.cfg.EnableMetrics {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e := collectMetrics(ctx, c.clientset, event, c.cfg, timeout)
			addEvidence(e)
		}()
	}

	wg.Wait()

	bundle := EvidenceBundle{
		AnomalyEvent: event,
		Evidences:    evidences,
		CollectedAt:  time.Now(),
	}

	select {
	case c.output <- bundle:
	case <-ctx.Done():
		c.logger.Warn("证据采集完成但上下文已取消", zap.String("pod", event.PodName))
	}

	c.logger.Info("证据采集完成",
		zap.String("pod", event.PodName),
		zap.Int("evidenceCount", len(evidences)),
	)
}
