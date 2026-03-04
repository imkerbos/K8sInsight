package watcher

import (
	"context"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/kerbos/k8sinsight/internal/detector"
)

// PodHandler Pod Informer 事件处理器（主通道）
type PodHandler struct {
	filter   *Filter
	detector *detector.Detector
	logger   *zap.Logger
	ctx      context.Context
}

// NewPodHandler 创建 Pod 事件处理器
func NewPodHandler(ctx context.Context, filter *Filter, det *detector.Detector, logger *zap.Logger) *PodHandler {
	return &PodHandler{
		filter:   filter,
		detector: det,
		logger:   logger.Named("pod-handler"),
		ctx:      ctx,
	}
}

// EventHandler 返回 cache.ResourceEventHandlerFuncs
func (h *PodHandler) EventHandler() cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    h.onAdd,
		UpdateFunc: h.onUpdate,
		DeleteFunc: h.onDelete,
	}
}

// onAdd 处理新发现的 Pod（启动时已存在的异常 Pod）
func (h *PodHandler) onAdd(obj interface{}) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return
	}

	if !h.filter.ShouldProcess(pod) {
		return
	}

	// 启动时检查已存在的异常 Pod
	if hasAnomalyStatus(pod) {
		h.logger.Debug("发现已存在的异常 Pod",
			zap.String("pod", pod.Name),
			zap.String("namespace", pod.Namespace),
		)
		h.detector.EvaluatePodChange(h.ctx, nil, pod)
	}
}

// onUpdate 处理 Pod 状态变化（核心检测路径）
func (h *PodHandler) onUpdate(oldObj, newObj interface{}) {
	oldPod, ok1 := oldObj.(*corev1.Pod)
	newPod, ok2 := newObj.(*corev1.Pod)
	if !ok1 || !ok2 {
		return
	}

	if !h.filter.ShouldProcess(newPod) {
		return
	}

	// 快速判断：ResourceVersion 相同则无变化
	if oldPod.ResourceVersion == newPod.ResourceVersion {
		return
	}

	// 快速判断：只关注状态变化
	if !containerStatusChanged(oldPod, newPod) {
		return
	}

	h.detector.EvaluatePodChange(h.ctx, oldPod, newPod)
}

// onDelete Pod 被删除（记录日志，不产生异常事件）
func (h *PodHandler) onDelete(obj interface{}) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		// 处理 DeletedFinalStateUnknown
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			return
		}
		pod, ok = tombstone.Obj.(*corev1.Pod)
		if !ok {
			return
		}
	}

	if !h.filter.ShouldProcess(pod) {
		return
	}

	h.logger.Debug("Pod 被删除",
		zap.String("pod", pod.Name),
		zap.String("namespace", pod.Namespace),
	)
}

// hasAnomalyStatus 快速检查 Pod 是否处于异常状态
func hasAnomalyStatus(pod *corev1.Pod) bool {
	// 检查是否被驱逐
	if pod.Status.Phase == corev1.PodFailed && pod.Status.Reason == "Evicted" {
		return true
	}

	for _, cs := range pod.Status.ContainerStatuses {
		// CrashLoopBackOff 或 ImagePullBackOff
		if cs.State.Waiting != nil {
			reason := cs.State.Waiting.Reason
			if reason == "CrashLoopBackOff" || reason == "ImagePullBackOff" || reason == "ErrImagePull" {
				return true
			}
		}
		// OOMKilled 或异常退出（exitCode=143 视为优雅停止）
		if isAbnormalTermination(cs.LastTerminationState.Terminated) {
			return true
		}
	}
	return false
}

func isAbnormalTermination(t *corev1.ContainerStateTerminated) bool {
	if t == nil {
		return false
	}
	if t.ExitCode == 0 || t.ExitCode == 143 {
		return false
	}
	return true
}

// containerStatusChanged 判断容器状态是否发生了有意义的变化
func containerStatusChanged(oldPod, newPod *corev1.Pod) bool {
	// Phase 变化
	if oldPod.Status.Phase != newPod.Status.Phase {
		return true
	}

	// Reason 变化（如 Evicted）
	if oldPod.Status.Reason != newPod.Status.Reason {
		return true
	}

	oldStatuses := containerStatusMap(oldPod)
	for _, cs := range newPod.Status.ContainerStatuses {
		old, exists := oldStatuses[cs.Name]
		if !exists {
			return true
		}
		// 重启次数变化
		if cs.RestartCount != old.RestartCount {
			return true
		}
		// 状态类型变化（Running→Waiting, Running→Terminated 等）
		if stateType(cs) != stateType(old) {
			return true
		}
		// Waiting reason 变化
		if cs.State.Waiting != nil && old.State.Waiting != nil &&
			cs.State.Waiting.Reason != old.State.Waiting.Reason {
			return true
		}
	}
	return false
}

func containerStatusMap(pod *corev1.Pod) map[string]corev1.ContainerStatus {
	m := make(map[string]corev1.ContainerStatus, len(pod.Status.ContainerStatuses))
	for _, cs := range pod.Status.ContainerStatuses {
		m[cs.Name] = cs
	}
	return m
}

func stateType(cs corev1.ContainerStatus) string {
	if cs.State.Running != nil {
		return "Running"
	}
	if cs.State.Waiting != nil {
		return "Waiting"
	}
	if cs.State.Terminated != nil {
		return "Terminated"
	}
	return "Unknown"
}
