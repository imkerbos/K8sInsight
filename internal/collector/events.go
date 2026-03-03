package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/kerbos/k8sinsight/internal/detector"
)

// eventSummary 精简的事件摘要，去除 managedFields 等噪音
type eventSummary struct {
	Type           string `json:"type"`
	Reason         string `json:"reason"`
	Message        string `json:"message"`
	Count          int32  `json:"count"`
	FirstTimestamp string `json:"firstTimestamp"`
	LastTimestamp   string `json:"lastTimestamp"`
	Source         string `json:"source"`
}

// collectPodEvents 采集 Pod 相关的 K8s Events（精简格式）
// 同时从容器终止状态中提取 OOM/异常退出信息（内核 OOM Kill 不会生成 K8s Event）
func collectPodEvents(ctx context.Context, client kubernetes.Interface, event detector.AnomalyEvent, timeout time.Duration) Evidence {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	fieldSelector := "involvedObject.name=" + event.PodName +
		",involvedObject.namespace=" + event.Namespace +
		",involvedObject.kind=Pod"

	events, err := client.CoreV1().Events(event.Namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return Evidence{
			Type:      EvidencePodEvents,
			Timestamp: time.Now(),
			Error:     err.Error(),
		}
	}

	// 按时间排序（最新的在后面）
	sort.Slice(events.Items, func(i, j int) bool {
		ti := events.Items[i].LastTimestamp.Time
		tj := events.Items[j].LastTimestamp.Time
		return ti.Before(tj)
	})

	summaries := make([]eventSummary, 0, len(events.Items)+2)

	// 从容器终止状态中补充 OOM/异常退出信息（内核 OOM Kill 不生成 K8s Event）
	if event.PodSnapshot != nil {
		for _, cs := range event.PodSnapshot.Status.ContainerStatuses {
			if t := cs.LastTerminationState.Terminated; t != nil {
				var msg string
				switch t.Reason {
				case "OOMKilled":
					// 查找容器的内存 limit
					memLimit := "unknown"
					for _, spec := range event.PodSnapshot.Spec.Containers {
						if spec.Name == cs.Name {
							if v, ok := spec.Resources.Limits["memory"]; ok {
								memLimit = v.String()
							}
							break
						}
					}
					msg = fmt.Sprintf("容器 %s 被 OOM Kill (退出码 %d)，内存限制 %s，已重启 %d 次",
						cs.Name, t.ExitCode, memLimit, cs.RestartCount)
				default:
					msg = fmt.Sprintf("容器 %s 异常终止，原因: %s，退出码 %d，已重启 %d 次",
						cs.Name, t.Reason, t.ExitCode, cs.RestartCount)
				}
				ts := t.FinishedAt.Format(time.RFC3339)
				summaries = append(summaries, eventSummary{
					Type:           "Warning",
					Reason:         t.Reason,
					Message:        msg,
					Count:          cs.RestartCount,
					FirstTimestamp: ts,
					LastTimestamp:   ts,
					Source:         "k8sinsight/container-status",
				})
			}
		}
	}

	for _, e := range events.Items {
		source := e.Source.Component
		if e.Source.Host != "" {
			source += "/" + e.Source.Host
		}
		summaries = append(summaries, eventSummary{
			Type:           e.Type,
			Reason:         e.Reason,
			Message:        e.Message,
			Count:          e.Count,
			FirstTimestamp: e.FirstTimestamp.Format(time.RFC3339),
			LastTimestamp:  e.LastTimestamp.Format(time.RFC3339),
			Source:         source,
		})
	}

	data, err := json.Marshal(summaries)
	if err != nil {
		return Evidence{
			Type:      EvidencePodEvents,
			Timestamp: time.Now(),
			Error:     err.Error(),
		}
	}

	return Evidence{
		Type:      EvidencePodEvents,
		Content:   string(data),
		Timestamp: time.Now(),
	}
}
