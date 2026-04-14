package detector

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
)

// Rule 异常检测规则接口
type Rule interface {
	// Name 规则名称
	Name() string
	// Evaluate 评估 Pod 状态变化，返回检测到的异常事件
	// oldPod 可能为 nil（首次发现时）
	Evaluate(oldPod, newPod *corev1.Pod) []AnomalyEvent
}

// ----- CrashLoopBackOff 检测 -----

type crashLoopRule struct{}

func NewCrashLoopRule() Rule { return &crashLoopRule{} }

func (r *crashLoopRule) Name() string { return "CrashLoopBackOff" }

func (r *crashLoopRule) Evaluate(oldPod, newPod *corev1.Pod) []AnomalyEvent {
	var events []AnomalyEvent
	for _, cs := range newPod.Status.ContainerStatuses {
		if cs.State.Waiting != nil && cs.State.Waiting.Reason == "CrashLoopBackOff" {
			// 如果旧状态已经是 CrashLoopBackOff 且 restartCount 未变，跳过
			if oldPod != nil && isAlreadyCrashLoop(oldPod, cs.Name, cs.RestartCount) {
				continue
			}
			image := getContainerImage(newPod, cs.Name)
			events = append(events, buildContainerAnomaly(
				AnomalyCrashLoopBackOff, SourcePodState, newPod, cs.Name,
				fmt.Sprintf("容器 %s (%s) 进入 CrashLoopBackOff，已重启 %d 次", cs.Name, image, cs.RestartCount),
				cs.RestartCount, 0, "CrashLoopBackOff",
			))
		}
	}
	return events
}

func isAlreadyCrashLoop(pod *corev1.Pod, containerName string, currentCount int32) bool {
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Name == containerName {
			return cs.State.Waiting != nil &&
				cs.State.Waiting.Reason == "CrashLoopBackOff" &&
				cs.RestartCount == currentCount
		}
	}
	return false
}

// ----- OOMKilled 检测 -----

type oomKilledRule struct{}

func NewOOMKilledRule() Rule { return &oomKilledRule{} }

func (r *oomKilledRule) Name() string { return "OOMKilled" }

func (r *oomKilledRule) Evaluate(oldPod, newPod *corev1.Pod) []AnomalyEvent {
	var events []AnomalyEvent
	for _, cs := range newPod.Status.ContainerStatuses {
		terminated := cs.LastTerminationState.Terminated
		// 检查 lastState（上一次终止原因）
		if terminated == nil || terminated.Reason != "OOMKilled" || isNormalExitCode(terminated.ExitCode) {
			continue
		}
		// 确认是新的 OOM（restartCount 增加了）
		if oldPod != nil && !restartCountIncreased(oldPod, cs.Name, cs.RestartCount) {
			continue
		}
		image := getContainerImage(newPod, cs.Name)
		events = append(events, buildContainerAnomaly(
			AnomalyOOMKilled, SourcePodState, newPod, cs.Name,
			fmt.Sprintf("容器 %s (%s) 因 OOMKilled 被终止，退出码 %d", cs.Name, image, terminated.ExitCode),
			cs.RestartCount, terminated.ExitCode, "OOMKilled",
		))
	}
	return events
}

// ----- 异常退出检测 -----

type errorExitRule struct{}

func NewErrorExitRule() Rule { return &errorExitRule{} }

func (r *errorExitRule) Name() string { return "ErrorExit" }

func (r *errorExitRule) Evaluate(oldPod, newPod *corev1.Pod) []AnomalyEvent {
	var events []AnomalyEvent
	for _, cs := range newPod.Status.ContainerStatuses {
		terminated := cs.LastTerminationState.Terminated
		if shouldSkipErrorExit(terminated) {
			continue
		}
		if oldPod != nil && !restartCountIncreased(oldPod, cs.Name, cs.RestartCount) {
			continue
		}
		image := getContainerImage(newPod, cs.Name)
		events = append(events, buildContainerAnomaly(
			AnomalyErrorExit, SourcePodState, newPod, cs.Name,
			fmt.Sprintf("容器 %s (%s) 异常退出，退出码 %d，原因: %s", cs.Name, image, terminated.ExitCode, terminated.Reason),
			cs.RestartCount, terminated.ExitCode, terminated.Reason,
		))
	}
	return events
}

func shouldSkipErrorExit(terminated *corev1.ContainerStateTerminated) bool {
	if terminated == nil {
		return true
	}
	if isNormalExitCode(terminated.ExitCode) {
		return true
	}
	if terminated.Reason == "OOMKilled" {
		return true
	}
	return false
}

func isNormalExitCode(exitCode int32) bool {
	// 0=正常退出；143=SIGTERM，常见于滚动更新/优雅停止。
	return exitCode == 0 || exitCode == 143
}

// ----- ImagePullBackOff 检测 -----

type imagePullBackOffRule struct{}

func NewImagePullBackOffRule() Rule { return &imagePullBackOffRule{} }

func (r *imagePullBackOffRule) Name() string { return "ImagePullBackOff" }

func (r *imagePullBackOffRule) Evaluate(oldPod, newPod *corev1.Pod) []AnomalyEvent {
	var events []AnomalyEvent
	for _, cs := range newPod.Status.ContainerStatuses {
		if cs.State.Waiting != nil &&
			(cs.State.Waiting.Reason == "ImagePullBackOff" || cs.State.Waiting.Reason == "ErrImagePull") {
			if oldPod != nil && isAlreadyImagePullBackOff(oldPod, cs.Name) {
				continue
			}
			image := getContainerImage(newPod, cs.Name)
			events = append(events, buildContainerAnomaly(
				AnomalyImagePullBackOff, SourcePodState, newPod, cs.Name,
				fmt.Sprintf("容器 %s 镜像 %s 拉取失败: %s", cs.Name, image, cs.State.Waiting.Message),
				0, 0, cs.State.Waiting.Reason,
			))
		}
	}
	return events
}

func isAlreadyImagePullBackOff(pod *corev1.Pod, containerName string) bool {
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Name == containerName && cs.State.Waiting != nil {
			return cs.State.Waiting.Reason == "ImagePullBackOff" || cs.State.Waiting.Reason == "ErrImagePull"
		}
	}
	return false
}

// ----- CreateContainerConfigError 检测 -----

type createContainerConfigErrorRule struct{}

func NewCreateContainerConfigErrorRule() Rule { return &createContainerConfigErrorRule{} }

func (r *createContainerConfigErrorRule) Name() string { return "CreateContainerConfigError" }

func (r *createContainerConfigErrorRule) Evaluate(oldPod, newPod *corev1.Pod) []AnomalyEvent {
	var events []AnomalyEvent
	for _, cs := range newPod.Status.ContainerStatuses {
		if cs.State.Waiting == nil || cs.State.Waiting.Reason != "CreateContainerConfigError" {
			continue
		}
		if oldPod != nil && isAlreadyCreateContainerConfigError(oldPod, cs.Name) {
			continue
		}
		image := getContainerImage(newPod, cs.Name)
		events = append(events, buildContainerAnomaly(
			AnomalyCreateContainerConfigError, SourcePodState, newPod, cs.Name,
			fmt.Sprintf("容器 %s (%s) 配置错误导致启动失败: %s", cs.Name, image, cs.State.Waiting.Message),
			cs.RestartCount, 0, cs.State.Waiting.Reason,
		))
	}
	return events
}

func isAlreadyCreateContainerConfigError(pod *corev1.Pod, containerName string) bool {
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Name == containerName && cs.State.Waiting != nil {
			return cs.State.Waiting.Reason == "CreateContainerConfigError"
		}
	}
	return false
}

// ----- Evicted 检测 -----

type evictedRule struct{}

func NewEvictedRule() Rule { return &evictedRule{} }

func (r *evictedRule) Name() string { return "Evicted" }

func (r *evictedRule) Evaluate(oldPod, newPod *corev1.Pod) []AnomalyEvent {
	if newPod.Status.Phase == corev1.PodFailed && newPod.Status.Reason == "Evicted" {
		if oldPod != nil && oldPod.Status.Reason == "Evicted" {
			return nil
		}
		return []AnomalyEvent{buildPodAnomaly(
			AnomalyEvicted, SourcePodState, newPod,
			fmt.Sprintf("Pod 被驱逐: %s", newPod.Status.Message),
			"Evicted",
		)}
	}
	return nil
}

// ----- 辅助函数 -----

// getContainerImage 从 Pod Spec 中按容器名查找镜像
func getContainerImage(pod *corev1.Pod, containerName string) string {
	for _, c := range pod.Spec.Containers {
		if c.Name == containerName {
			return c.Image
		}
	}
	return ""
}

func restartCountIncreased(oldPod *corev1.Pod, containerName string, newCount int32) bool {
	for _, cs := range oldPod.Status.ContainerStatuses {
		if cs.Name == containerName {
			return newCount > cs.RestartCount
		}
	}
	return true // 旧 Pod 中找不到该容器，视为新容器
}

func buildContainerAnomaly(
	anomalyType AnomalyType, source DetectionSource,
	pod *corev1.Pod, containerName, message string,
	restartCount, exitCode int32, reason string,
) AnomalyEvent {
	ownerKind, ownerName := resolveOwner(pod)
	return AnomalyEvent{
		Timestamp:     time.Now(),
		Type:          anomalyType,
		Source:        source,
		Message:       message,
		PodUID:        string(pod.UID),
		PodName:       pod.Name,
		Namespace:     pod.Namespace,
		NodeName:      pod.Spec.NodeName,
		ContainerName: containerName,
		OwnerKind:     ownerKind,
		OwnerName:     ownerName,
		ExitCode:      exitCode,
		Reason:        reason,
		RestartCount:  restartCount,
		PodSnapshot:   pod.DeepCopy(),
	}
}

func buildPodAnomaly(
	anomalyType AnomalyType, source DetectionSource,
	pod *corev1.Pod, message, reason string,
) AnomalyEvent {
	ownerKind, ownerName := resolveOwner(pod)
	return AnomalyEvent{
		Timestamp:   time.Now(),
		Type:        anomalyType,
		Source:      source,
		Message:     message,
		PodUID:      string(pod.UID),
		PodName:     pod.Name,
		Namespace:   pod.Namespace,
		NodeName:    pod.Spec.NodeName,
		OwnerKind:   ownerKind,
		OwnerName:   ownerName,
		Reason:      reason,
		PodSnapshot: pod.DeepCopy(),
	}
}

// resolveOwner 解析 Pod 的 Owner
// 优先级: Deployment > StatefulSet > DaemonSet > Job > ReplicaSet > 裸 Pod
func resolveOwner(pod *corev1.Pod) (kind, name string) {
	if len(pod.OwnerReferences) == 0 {
		return "", ""
	}

	ref := pod.OwnerReferences[0]
	// ReplicaSet 通常由 Deployment 管理，名称格式: {deployment-name}-{hash}
	// 这里先返回 ReplicaSet，后续由 Detector 通过 API 查询追溯到 Deployment
	return ref.Kind, ref.Name
}
