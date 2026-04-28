package incident

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"go.uber.org/zap"

	"github.com/kerbos/k8sinsight/internal/detector"
	"github.com/kerbos/k8sinsight/internal/domain"
	"github.com/kerbos/k8sinsight/internal/pipeline/dedup"
	"github.com/kerbos/k8sinsight/internal/store/repository"
)

// EvidenceProcessor 消费证据 channel，保存到 DB 并触发通知
type EvidenceProcessor struct {
	input        <-chan domain.EvidenceBundle
	index        *dedup.Index
	evidenceRepo repository.EvidenceRepository
	notifier     detector.EventSink
	workers      int
	logger       *zap.Logger
}

// NewEvidenceProcessor 创建证据处理器
func NewEvidenceProcessor(
	input <-chan domain.EvidenceBundle,
	index *dedup.Index,
	evidenceRepo repository.EvidenceRepository,
	notifier detector.EventSink,
	workers int,
	logger *zap.Logger,
) *EvidenceProcessor {
	if workers <= 0 {
		workers = 4
	}
	return &EvidenceProcessor{
		input:        input,
		index:        index,
		evidenceRepo: evidenceRepo,
		notifier:     notifier,
		workers:      workers,
		logger:       logger.Named("evidence-proc"),
	}
}

// Start 启动 worker pool 消费证据
func (p *EvidenceProcessor) Start(ctx context.Context) {
	for i := 0; i < p.workers; i++ {
		go p.worker(ctx)
	}
}

func (p *EvidenceProcessor) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case bundle, ok := <-p.input:
			if !ok {
				return
			}
			p.processBundle(ctx, bundle)
		}
	}
}

func (p *EvidenceProcessor) processBundle(ctx context.Context, bundle domain.EvidenceBundle) {
	dedupKey := bundle.AnomalyEvent.DedupKey()
	inc, found := p.index.FindActive(dedupKey)
	if !found {
		p.logger.Warn("证据对应的事件未找到", zap.String("dedupKey", dedupKey))
		return
	}

	if err := p.evidenceRepo.SaveBundle(ctx, inc.ID, &bundle); err != nil {
		p.logger.Error("保存证据失败", zap.Error(err))
	}

	// 证据驱动通知：基于已采集证据生成数据化结论后再发送
	if p.notifier != nil {
		enriched := buildEvidenceDrivenEvent(inc.ID, bundle.AnomalyEvent, bundle.Evidences)
		if err := p.notifier.HandleAnomaly(ctx, enriched); err != nil {
			p.logger.Error("发送证据驱动通知失败", zap.Error(err))
		}
	}
}

// === Evidence Analysis (migrated from aggregator/evidence_analysis.go) ===

func buildEvidenceDrivenEvent(incidentID string, event domain.AnomalyEvent, evidences []domain.Evidence) domain.AnomalyEvent {
	lines := []string{
		event.Message,
		"",
		"[证据分析]",
		fmt.Sprintf("incidentID=%s", incidentID),
	}
	lines = append(lines, analyzeByType(event, evidences)...)
	event.Message = strings.Join(lines, "\n")
	return event
}

func analyzeByType(event domain.AnomalyEvent, evidences []domain.Evidence) []string {
	pod := parsePodSnapshot(evidences, event.ContainerName)
	peakMem, latestMem := parsePromMemory(evidences)
	latestEvent := parseLatestPodEvent(evidences)

	out := make([]string, 0, 8)
	if pod != nil {
		if pod.LimitMemory != "" || pod.RequestMemory != "" {
			out = append(out, fmt.Sprintf("memory request/limit=%s/%s", safeVal(pod.RequestMemory), safeVal(pod.LimitMemory)))
		}
		if pod.LastState != "" || pod.ExitCode != 0 || pod.RestartCount > 0 {
			out = append(out, fmt.Sprintf("lastState=%s exitCode=%d restartCount=%d", safeVal(pod.LastState), pod.ExitCode, pod.RestartCount))
		}
	}
	if peakMem > 0 || latestMem > 0 {
		out = append(out, fmt.Sprintf("memory(oom前窗口) peak=%s latest=%s", humanBytes(peakMem), humanBytes(latestMem)))
	}
	if latestEvent != "" {
		out = append(out, "latestWarningEvent="+latestEvent)
	}

	switch event.Type {
	case domain.AnomalyOOMKilled:
		if pod != nil && pod.LimitMemory != "" {
			out = append(out, "结论=容器达到或逼近内存上限后被内核OOMKill")
		} else {
			out = append(out, "结论=检测到OOMKill，需补齐内存limit与用量数据进一步确认")
		}
	case domain.AnomalyCrashLoopBackOff:
		out = append(out, "结论=容器反复失败重启，需结合启动日志与退出码定位首因")
	case domain.AnomalyErrorExit:
		out = append(out, "结论=非OOM异常退出，优先根据退出码+日志定位")
	case domain.AnomalyImagePullBackOff:
		out = append(out, "结论=镜像拉取路径异常（仓库可达性/鉴权/tag）")
	case domain.AnomalyCreateContainerConfigError:
		out = append(out, "结论=容器配置依赖异常（ConfigMap/Secret/挂载）")
	case domain.AnomalyFailedScheduling:
		out = append(out, "结论=调度约束或资源不足导致未能落点")
	case domain.AnomalyEvicted:
		out = append(out, "结论=节点资源压力触发驱逐（内存/磁盘）")
	}

	if len(out) == 0 {
		out = append(out, "证据不足，等待更多采集数据")
	}
	return out
}

type podContainerInfo struct {
	LastState     string
	ExitCode      int32
	RestartCount  int32
	RequestMemory string
	LimitMemory   string
}

func parsePodSnapshot(evidences []domain.Evidence, container string) *podContainerInfo {
	type resources struct {
		RequestsMemory string `json:"requestsMemory"`
		LimitsMemory   string `json:"limitsMemory"`
	}
	type containerDigest struct {
		Name         string     `json:"name"`
		RestartCount int32      `json:"restartCount"`
		LastState    string     `json:"lastState"`
		ExitCode     *int32     `json:"exitCode"`
		Resources    *resources `json:"resources"`
	}
	type podDigest struct {
		Containers []containerDigest `json:"containers"`
	}
	for _, e := range evidences {
		if e.Type != domain.EvidencePodSnapshot || e.Content == "" {
			continue
		}
		var p podDigest
		if err := json.Unmarshal([]byte(e.Content), &p); err != nil {
			continue
		}
		for _, c := range p.Containers {
			if container != "" && c.Name != container {
				continue
			}
			info := &podContainerInfo{
				LastState:    c.LastState,
				RestartCount: c.RestartCount,
			}
			if c.ExitCode != nil {
				info.ExitCode = *c.ExitCode
			}
			if c.Resources != nil {
				info.RequestMemory = c.Resources.RequestsMemory
				info.LimitMemory = c.Resources.LimitsMemory
			}
			return info
		}
	}
	return nil
}

func parseLatestPodEvent(evidences []domain.Evidence) string {
	type eventSummary struct {
		Reason  string `json:"reason"`
		Message string `json:"message"`
	}
	for i := len(evidences) - 1; i >= 0; i-- {
		e := evidences[i]
		if e.Type != domain.EvidencePodEvents || e.Content == "" {
			continue
		}
		var list []eventSummary
		if err := json.Unmarshal([]byte(e.Content), &list); err != nil || len(list) == 0 {
			continue
		}
		last := list[len(list)-1]
		return strings.TrimSpace(last.Reason + " " + last.Message)
	}
	return ""
}

func parsePromMemory(evidences []domain.Evidence) (peakBytes, latestBytes float64) {
	type matrixSeries struct {
		Values [][]any `json:"values"`
	}
	type bundle struct {
		Source string `json:"source"`
		Series struct {
			Memory []matrixSeries `json:"memory"`
		} `json:"series"`
	}
	for _, e := range evidences {
		if e.Type != domain.EvidenceMetrics || e.Content == "" {
			continue
		}
		var b bundle
		if err := json.Unmarshal([]byte(e.Content), &b); err != nil || b.Source != "prometheus" {
			continue
		}
		for _, s := range b.Series.Memory {
			for _, point := range s.Values {
				if len(point) < 2 {
					continue
				}
				valStr, ok := point[1].(string)
				if !ok {
					continue
				}
				v, err := strconv.ParseFloat(valStr, 64)
				if err != nil {
					continue
				}
				if v > peakBytes {
					peakBytes = v
				}
				latestBytes = v
			}
		}
	}
	return
}

func humanBytes(v float64) string {
	if v <= 0 {
		return "-"
	}
	const mi = 1024 * 1024
	return fmt.Sprintf("%.1fMi", v/mi)
}

func safeVal(v string) string {
	if v == "" {
		return "-"
	}
	return v
}
