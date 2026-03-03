package aggregator

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/kerbos/k8sinsight/internal/collector"
	"github.com/kerbos/k8sinsight/internal/detector"
)

func buildEvidenceDrivenEvent(incidentID string, event detector.AnomalyEvent, evidences []collector.Evidence) detector.AnomalyEvent {
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

func analyzeByType(event detector.AnomalyEvent, evidences []collector.Evidence) []string {
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
	case detector.AnomalyOOMKilled:
		if pod != nil && pod.LimitMemory != "" {
			out = append(out, "结论=容器达到或逼近内存上限后被内核OOMKill")
		} else {
			out = append(out, "结论=检测到OOMKill，需补齐内存limit与用量数据进一步确认")
		}
	case detector.AnomalyCrashLoopBackOff:
		out = append(out, "结论=容器反复失败重启，需结合启动日志与退出码定位首因")
	case detector.AnomalyErrorExit:
		out = append(out, "结论=非OOM异常退出，优先根据退出码+日志定位")
	case detector.AnomalyImagePullBackOff:
		out = append(out, "结论=镜像拉取路径异常（仓库可达性/鉴权/tag）")
	case detector.AnomalyCreateContainerConfigError:
		out = append(out, "结论=容器配置依赖异常（ConfigMap/Secret/挂载）")
	case detector.AnomalyFailedScheduling:
		out = append(out, "结论=调度约束或资源不足导致未能落点")
	case detector.AnomalyEvicted:
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

func parsePodSnapshot(evidences []collector.Evidence, container string) *podContainerInfo {
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
		if e.Type != collector.EvidencePodSnapshot || e.Content == "" {
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
				LastState:     c.LastState,
				RestartCount:  c.RestartCount,
				RequestMemory: "",
				LimitMemory:   "",
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

func parseLatestPodEvent(evidences []collector.Evidence) string {
	type eventSummary struct {
		Reason  string `json:"reason"`
		Message string `json:"message"`
	}
	for i := len(evidences) - 1; i >= 0; i-- {
		e := evidences[i]
		if e.Type != collector.EvidencePodEvents || e.Content == "" {
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

func parsePromMemory(evidences []collector.Evidence) (peakBytes, latestBytes float64) {
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
		if e.Type != collector.EvidenceMetrics || e.Content == "" {
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
