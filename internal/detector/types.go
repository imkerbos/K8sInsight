package detector

import (
	"time"

	corev1 "k8s.io/api/core/v1"
)

// AnomalyType 异常类型枚举
type AnomalyType string

const (
	AnomalyCrashLoopBackOff AnomalyType = "CrashLoopBackOff"
	AnomalyOOMKilled        AnomalyType = "OOMKilled"
	AnomalyErrorExit        AnomalyType = "ErrorExit"
	AnomalyRestartIncrement AnomalyType = "RestartIncrement"
	AnomalyImagePullBackOff AnomalyType = "ImagePullBackOff"
	AnomalyCreateContainerConfigError AnomalyType = "CreateContainerConfigError"
	AnomalyFailedScheduling AnomalyType = "FailedScheduling"
	AnomalyEvicted          AnomalyType = "Evicted"
	AnomalyStateOscillation AnomalyType = "StateOscillation"
)

// DetectionSource 检测来源
type DetectionSource string

const (
	SourcePodState   DetectionSource = "PodState"    // 主通道：Pod 状态变化
	SourceK8sEvent   DetectionSource = "K8sEvent"    // 补充通道：K8s Warning Event
)

// AnomalyEvent 异常事件 —— 核心领域对象
// 从 Watcher/Detector 产出，流向 Collector、Aggregator
type AnomalyEvent struct {
	// 标识
	ID        string      `json:"id"`
	Timestamp time.Time   `json:"timestamp"`

	// 集群标识
	ClusterID string `json:"clusterId,omitempty"`

	// 异常信息
	Type      AnomalyType     `json:"type"`
	Source    DetectionSource  `json:"source"`
	Message   string          `json:"message"`

	// Pod 信息
	PodUID       string `json:"podUid"`
	PodName      string `json:"podName"`
	Namespace    string `json:"namespace"`
	NodeName     string `json:"nodeName"`
	ContainerName string `json:"containerName"`

	// Owner 信息（用于 dedup key）
	OwnerKind string `json:"ownerKind"`
	OwnerName string `json:"ownerName"`

	// 容器状态详情
	ExitCode      int32  `json:"exitCode,omitempty"`
	Reason        string `json:"reason,omitempty"`
	RestartCount  int32  `json:"restartCount,omitempty"`

	// 原始 Pod 快照（用于证据采集）
	PodSnapshot *corev1.Pod `json:"-"`
}

// DedupKey 生成去重聚合键（Pod 级别）
// 格式: {clusterID}/{namespace}/Pod/{pod-identity}/{anomaly-type}
// 说明:
// - pod-identity 优先使用 PodUID，避免同名 Pod 重建后的误聚合
// - 当 PodUID 为空时回退到 PodName
func (e *AnomalyEvent) DedupKey() string {
	podIdentity := e.PodUID
	if podIdentity == "" {
		podIdentity = e.PodName
	}
	if podIdentity == "" {
		podIdentity = "_"
	}

	clusterPrefix := e.ClusterID
	if clusterPrefix == "" {
		clusterPrefix = "_"
	}

	return clusterPrefix + "/" + e.Namespace + "/Pod/" + podIdentity + "/" + string(e.Type)
}
