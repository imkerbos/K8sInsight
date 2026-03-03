package collector

import (
	"encoding/json"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/kerbos/k8sinsight/internal/detector"
)

// podDigest 精简的 Pod 诊断摘要
type podDigest struct {
	Name       string            `json:"name"`
	Namespace  string            `json:"namespace"`
	Node       string            `json:"node"`
	Phase      string            `json:"phase"`
	Reason     string            `json:"reason,omitempty"`
	Message    string            `json:"message,omitempty"`
	StartTime  string            `json:"startTime,omitempty"`
	Conditions []conditionDigest `json:"conditions,omitempty"`
	Containers []containerDigest `json:"containers"`
}

type conditionDigest struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

type containerDigest struct {
	Name         string            `json:"name"`
	Image        string            `json:"image"`
	Ready        bool              `json:"ready"`
	RestartCount int32             `json:"restartCount"`
	State        string            `json:"state"`
	StateDetail  string            `json:"stateDetail,omitempty"`
	LastState    string            `json:"lastState,omitempty"`
	ExitCode     *int32            `json:"exitCode,omitempty"`
	Reason       string            `json:"reason,omitempty"`
	Resources    *resourcesDigest  `json:"resources,omitempty"`
}

type resourcesDigest struct {
	RequestsCPU    string `json:"requestsCPU,omitempty"`
	RequestsMemory string `json:"requestsMemory,omitempty"`
	LimitsCPU      string `json:"limitsCPU,omitempty"`
	LimitsMemory   string `json:"limitsMemory,omitempty"`
}

// collectPodSnapshot 提取 Pod 关键诊断信息
func collectPodSnapshot(event detector.AnomalyEvent) Evidence {
	if event.PodSnapshot == nil {
		return Evidence{
			Type:      EvidencePodSnapshot,
			Timestamp: time.Now(),
			Error:     "Pod 快照不可用",
		}
	}

	pod := event.PodSnapshot
	digest := podDigest{
		Name:      pod.Name,
		Namespace: pod.Namespace,
		Node:      pod.Spec.NodeName,
		Phase:     string(pod.Status.Phase),
		Reason:    pod.Status.Reason,
		Message:   pod.Status.Message,
	}

	if pod.Status.StartTime != nil {
		digest.StartTime = pod.Status.StartTime.Format(time.RFC3339)
	}

	for _, cond := range pod.Status.Conditions {
		if cond.Status != corev1.ConditionTrue || cond.Reason != "" {
			digest.Conditions = append(digest.Conditions, conditionDigest{
				Type:    string(cond.Type),
				Status:  string(cond.Status),
				Reason:  cond.Reason,
				Message: cond.Message,
			})
		}
	}

	// 建立 spec 容器 -> resources 的映射
	specMap := make(map[string]corev1.Container, len(pod.Spec.Containers))
	for _, c := range pod.Spec.Containers {
		specMap[c.Name] = c
	}

	for _, cs := range pod.Status.ContainerStatuses {
		cd := containerDigest{
			Name:         cs.Name,
			Image:        cs.Image,
			Ready:        cs.Ready,
			RestartCount: cs.RestartCount,
		}

		// 当前状态
		switch {
		case cs.State.Running != nil:
			cd.State = "Running"
			cd.StateDetail = "since " + cs.State.Running.StartedAt.Format(time.RFC3339)
		case cs.State.Waiting != nil:
			cd.State = "Waiting"
			cd.StateDetail = cs.State.Waiting.Reason
			if cs.State.Waiting.Message != "" {
				cd.StateDetail += ": " + cs.State.Waiting.Message
			}
		case cs.State.Terminated != nil:
			cd.State = "Terminated"
			cd.Reason = cs.State.Terminated.Reason
			cd.ExitCode = &cs.State.Terminated.ExitCode
			cd.StateDetail = cs.State.Terminated.Message
		}

		// 上次终止状态（对 OOM/CrashLoop 关键）
		if cs.LastTerminationState.Terminated != nil {
			t := cs.LastTerminationState.Terminated
			cd.LastState = t.Reason
			cd.ExitCode = &t.ExitCode
		}

		// 资源配置
		if spec, ok := specMap[cs.Name]; ok {
			res := &resourcesDigest{}
			if v, ok := spec.Resources.Requests[corev1.ResourceCPU]; ok {
				res.RequestsCPU = v.String()
			}
			if v, ok := spec.Resources.Requests[corev1.ResourceMemory]; ok {
				res.RequestsMemory = v.String()
			}
			if v, ok := spec.Resources.Limits[corev1.ResourceCPU]; ok {
				res.LimitsCPU = v.String()
			}
			if v, ok := spec.Resources.Limits[corev1.ResourceMemory]; ok {
				res.LimitsMemory = v.String()
			}
			cd.Resources = res
		}

		digest.Containers = append(digest.Containers, cd)
	}

	data, err := json.Marshal(digest)
	if err != nil {
		return Evidence{
			Type:      EvidencePodSnapshot,
			Timestamp: time.Now(),
			Error:     err.Error(),
		}
	}

	return Evidence{
		Type:      EvidencePodSnapshot,
		Content:   string(data),
		Timestamp: time.Now(),
	}
}
