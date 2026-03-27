package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/kerbos/k8sinsight/internal/detector"
)

const (
	nodeEventWindow = 5 * time.Minute
	maxNodeEvents   = 40
)

// collectPodDescribe 采集 Pod 的 describe 风格详情和完整快照，便于 AI 做更细粒度分析。
func collectPodDescribe(
	ctx context.Context,
	client kubernetes.Interface,
	event detector.AnomalyEvent,
	timeout time.Duration,
) Evidence {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	pod, source, err := getPodSnapshotOrLive(ctx, client, event)
	if err != nil {
		return Evidence{
			Type:      EvidencePodDescribe,
			Timestamp: time.Now(),
			Error:     err.Error(),
		}
	}

	payload := map[string]any{
		"capturedAt":    time.Now().Format(time.RFC3339),
		"captureSource": source,
		"eventTime":     formatRFC3339(event.Timestamp),
		"summary": map[string]any{
			"name":      pod.Name,
			"namespace": pod.Namespace,
			"node":      pod.Spec.NodeName,
			"phase":     string(pod.Status.Phase),
			"reason":    pod.Status.Reason,
			"message":   pod.Status.Message,
		},
		"describeText": buildPodDescribeText(pod),
		"pod":          pod,
	}

	data, mErr := json.Marshal(payload)
	if mErr != nil {
		return Evidence{
			Type:      EvidencePodDescribe,
			Timestamp: time.Now(),
			Error:     mErr.Error(),
		}
	}

	return Evidence{
		Type:      EvidencePodDescribe,
		Content:   string(data),
		Timestamp: time.Now(),
	}
}

// collectWorkloadSpec 拉取 Pod 所属工作负载规格（Deployment/StatefulSet/DaemonSet/Job/CronJob/ReplicaSet）。
func collectWorkloadSpec(
	ctx context.Context,
	client kubernetes.Interface,
	event detector.AnomalyEvent,
	timeout time.Duration,
) Evidence {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	pod, _, _ := getPodSnapshotOrLive(ctx, client, event)
	kind, name := resolveOwnerForCollection(event, pod)
	namespace := event.Namespace
	if namespace == "" && pod != nil {
		namespace = pod.Namespace
	}

	// 裸 Pod 场景：直接导出 Pod Spec，避免证据链断裂。
	if kind == "" || name == "" {
		if pod == nil {
			return Evidence{
				Type:      EvidenceWorkloadSpec,
				Timestamp: time.Now(),
				Error:     "无法解析工作负载 owner，且 Pod 快照不可用",
			}
		}
		manifest := map[string]any{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata":   compactObjectMeta(pod.ObjectMeta),
			"spec":       pod.Spec,
		}
		return marshalWorkloadEvidence(manifest)
	}

	manifest, err := fetchWorkloadManifest(ctx, client, namespace, kind, name)
	if err != nil {
		return Evidence{
			Type:      EvidenceWorkloadSpec,
			Timestamp: time.Now(),
			Error:     err.Error(),
		}
	}

	return marshalWorkloadEvidence(manifest)
}

// collectNodeContext 采集节点状态与事件窗口，补齐节点侧证据链。
func collectNodeContext(
	ctx context.Context,
	client kubernetes.Interface,
	event detector.AnomalyEvent,
	timeout time.Duration,
) Evidence {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	pod, _, _ := getPodSnapshotOrLive(ctx, client, event)
	nodeName := strings.TrimSpace(event.NodeName)
	if nodeName == "" && pod != nil {
		nodeName = strings.TrimSpace(pod.Spec.NodeName)
	}
	if nodeName == "" {
		return Evidence{
			Type:      EvidenceNodeContext,
			Timestamp: time.Now(),
			Error:     "节点名称为空，无法采集节点上下文",
		}
	}

	node, err := client.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return Evidence{
			Type:      EvidenceNodeContext,
			Timestamp: time.Now(),
			Error:     fmt.Sprintf("查询节点失败: %v", err),
		}
	}

	eventTs := event.Timestamp
	if eventTs.IsZero() {
		eventTs = time.Now()
	}
	windowStart := eventTs.Add(-nodeEventWindow)
	windowEnd := eventTs.Add(nodeEventWindow)

	var nodeEventList []eventSummary
	eventErr := ""
	nodeEvents, eErr := client.CoreV1().Events("").List(ctx, metav1.ListOptions{
		FieldSelector: "involvedObject.kind=Node,involvedObject.name=" + nodeName,
	})
	if eErr != nil {
		eventErr = eErr.Error()
	} else {
		sort.Slice(nodeEvents.Items, func(i, j int) bool {
			return k8sEventTimestamp(nodeEvents.Items[i]).After(k8sEventTimestamp(nodeEvents.Items[j]))
		})

		// 优先保留事件发生窗口内的节点事件。
		for _, item := range nodeEvents.Items {
			ts := k8sEventTimestamp(item)
			if ts.IsZero() || ts.Before(windowStart) || ts.After(windowEnd) {
				continue
			}
			nodeEventList = append(nodeEventList, toEventSummary(item))
			if len(nodeEventList) >= maxNodeEvents {
				break
			}
		}

		// 若窗口内无事件，回退到最近 Warning 事件，避免返回空证据。
		if len(nodeEventList) == 0 {
			for _, item := range nodeEvents.Items {
				if !strings.EqualFold(item.Type, "Warning") {
					continue
				}
				nodeEventList = append(nodeEventList, toEventSummary(item))
				if len(nodeEventList) >= 20 {
					break
				}
			}
		}
	}

	payload := map[string]any{
		"capturedAt": time.Now().Format(time.RFC3339),
		"eventTime":  eventTs.Format(time.RFC3339),
		"window": map[string]string{
			"start": windowStart.Format(time.RFC3339),
			"end":   windowEnd.Format(time.RFC3339),
		},
		"node": map[string]any{
			"name":          node.Name,
			"unschedulable": node.Spec.Unschedulable,
			"taints":        summarizeNodeTaints(node.Spec.Taints),
			"capacity":      resourceListToStrings(node.Status.Capacity),
			"allocatable":   resourceListToStrings(node.Status.Allocatable),
			"conditions":    summarizeNodeConditions(node.Status.Conditions),
			"nodeInfo": map[string]string{
				"kernelVersion":           node.Status.NodeInfo.KernelVersion,
				"osImage":                 node.Status.NodeInfo.OSImage,
				"kubeletVersion":          node.Status.NodeInfo.KubeletVersion,
				"containerRuntimeVersion": node.Status.NodeInfo.ContainerRuntimeVersion,
				"architecture":            node.Status.NodeInfo.Architecture,
			},
		},
		"nodeEvents": nodeEventList,
	}

	if pod != nil {
		payload["podRuntime"] = map[string]any{
			"name":              pod.Name,
			"namespace":         pod.Namespace,
			"qosClass":          string(pod.Status.QOSClass),
			"priorityClassName": pod.Spec.PriorityClassName,
			"hostNetwork":       pod.Spec.HostNetwork,
		}
	}
	if eventErr != "" {
		payload["nodeEventError"] = eventErr
	}

	data, mErr := json.Marshal(payload)
	if mErr != nil {
		return Evidence{
			Type:      EvidenceNodeContext,
			Timestamp: time.Now(),
			Error:     mErr.Error(),
		}
	}

	return Evidence{
		Type:      EvidenceNodeContext,
		Content:   string(data),
		Timestamp: time.Now(),
	}
}

func getPodSnapshotOrLive(
	ctx context.Context,
	client kubernetes.Interface,
	event detector.AnomalyEvent,
) (*corev1.Pod, string, error) {
	if event.PodSnapshot != nil {
		return event.PodSnapshot.DeepCopy(), "snapshot", nil
	}
	if event.Namespace == "" || event.PodName == "" {
		return nil, "", fmt.Errorf("Pod 快照不可用，且缺少 namespace/podName")
	}
	pod, err := client.CoreV1().Pods(event.Namespace).Get(ctx, event.PodName, metav1.GetOptions{})
	if err != nil {
		return nil, "", fmt.Errorf("Pod 快照不可用且实时查询失败: %w", err)
	}
	return pod, "live", nil
}

func resolveOwnerForCollection(event detector.AnomalyEvent, pod *corev1.Pod) (kind, name string) {
	if event.OwnerKind != "" && event.OwnerName != "" {
		return event.OwnerKind, event.OwnerName
	}
	if pod == nil || len(pod.OwnerReferences) == 0 {
		return "", ""
	}
	ref := pod.OwnerReferences[0]
	return ref.Kind, ref.Name
}

func fetchWorkloadManifest(
	ctx context.Context,
	client kubernetes.Interface,
	namespace, kind, name string,
) (map[string]any, error) {
	switch kind {
	case "Deployment":
		obj, err := client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("查询 Deployment 失败: %w", err)
		}
		return map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata":   compactObjectMeta(obj.ObjectMeta),
			"spec":       obj.Spec,
		}, nil
	case "StatefulSet":
		obj, err := client.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("查询 StatefulSet 失败: %w", err)
		}
		return map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "StatefulSet",
			"metadata":   compactObjectMeta(obj.ObjectMeta),
			"spec":       obj.Spec,
		}, nil
	case "DaemonSet":
		obj, err := client.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("查询 DaemonSet 失败: %w", err)
		}
		return map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "DaemonSet",
			"metadata":   compactObjectMeta(obj.ObjectMeta),
			"spec":       obj.Spec,
		}, nil
	case "ReplicaSet":
		obj, err := client.AppsV1().ReplicaSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("查询 ReplicaSet 失败: %w", err)
		}
		return map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "ReplicaSet",
			"metadata":   compactObjectMeta(obj.ObjectMeta),
			"spec":       obj.Spec,
		}, nil
	case "Job":
		obj, err := client.BatchV1().Jobs(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("查询 Job 失败: %w", err)
		}
		return map[string]any{
			"apiVersion": "batch/v1",
			"kind":       "Job",
			"metadata":   compactObjectMeta(obj.ObjectMeta),
			"spec":       obj.Spec,
		}, nil
	case "CronJob":
		obj, err := client.BatchV1().CronJobs(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("查询 CronJob 失败: %w", err)
		}
		return map[string]any{
			"apiVersion": "batch/v1",
			"kind":       "CronJob",
			"metadata":   compactObjectMeta(obj.ObjectMeta),
			"spec":       obj.Spec,
		}, nil
	default:
		return nil, fmt.Errorf("暂不支持的 owner kind: %s", kind)
	}
}

func marshalWorkloadEvidence(manifest map[string]any) Evidence {
	payload := map[string]any{
		"capturedAt": time.Now().Format(time.RFC3339),
		"workload":   manifest,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return Evidence{
			Type:      EvidenceWorkloadSpec,
			Timestamp: time.Now(),
			Error:     err.Error(),
		}
	}
	return Evidence{
		Type:      EvidenceWorkloadSpec,
		Content:   string(data),
		Timestamp: time.Now(),
	}
}

func compactObjectMeta(meta metav1.ObjectMeta) map[string]any {
	return map[string]any{
		"name":            meta.Name,
		"namespace":       meta.Namespace,
		"labels":          meta.Labels,
		"annotations":     meta.Annotations,
		"ownerReferences": meta.OwnerReferences,
	}
}

func resourceListToStrings(list corev1.ResourceList) map[string]string {
	if len(list) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(list))
	for k, v := range list {
		out[string(k)] = v.String()
	}
	return out
}

func summarizeNodeTaints(taints []corev1.Taint) []map[string]string {
	result := make([]map[string]string, 0, len(taints))
	for _, t := range taints {
		result = append(result, map[string]string{
			"key":    t.Key,
			"value":  t.Value,
			"effect": string(t.Effect),
		})
	}
	return result
}

func summarizeNodeConditions(conditions []corev1.NodeCondition) []map[string]string {
	result := make([]map[string]string, 0, len(conditions))
	for _, c := range conditions {
		result = append(result, map[string]string{
			"type":               string(c.Type),
			"status":             string(c.Status),
			"reason":             c.Reason,
			"message":            c.Message,
			"lastHeartbeatTime":  formatRFC3339(c.LastHeartbeatTime.Time),
			"lastTransitionTime": formatRFC3339(c.LastTransitionTime.Time),
		})
	}
	return result
}

func buildPodDescribeText(pod *corev1.Pod) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Name: %s\n", pod.Name)
	fmt.Fprintf(&b, "Namespace: %s\n", pod.Namespace)
	fmt.Fprintf(&b, "Node: %s\n", pod.Spec.NodeName)
	fmt.Fprintf(&b, "Status: %s\n", pod.Status.Phase)
	if pod.Status.Reason != "" {
		fmt.Fprintf(&b, "Reason: %s\n", pod.Status.Reason)
	}
	if pod.Status.Message != "" {
		fmt.Fprintf(&b, "Message: %s\n", pod.Status.Message)
	}
	if pod.Status.StartTime != nil {
		fmt.Fprintf(&b, "Start Time: %s\n", pod.Status.StartTime.Format(time.RFC3339))
	}
	if len(pod.OwnerReferences) > 0 {
		b.WriteString("Owner References:\n")
		for _, ref := range pod.OwnerReferences {
			fmt.Fprintf(&b, "- %s/%s (controller=%t)\n", ref.Kind, ref.Name, ref.Controller != nil && *ref.Controller)
		}
	}

	if len(pod.Status.Conditions) > 0 {
		b.WriteString("Conditions:\n")
		for _, c := range pod.Status.Conditions {
			fmt.Fprintf(&b, "- %s=%s reason=%s message=%s\n", c.Type, c.Status, c.Reason, c.Message)
		}
	}

	if len(pod.Spec.Containers) > 0 {
		b.WriteString("Containers:\n")
		for _, c := range pod.Spec.Containers {
			fmt.Fprintf(&b, "- %s image=%s\n", c.Name, c.Image)
			if len(c.Command) > 0 {
				fmt.Fprintf(&b, "  command: %s\n", strings.Join(c.Command, " "))
			}
			if len(c.Args) > 0 {
				fmt.Fprintf(&b, "  args: %s\n", strings.Join(c.Args, " "))
			}
			if len(c.Resources.Requests) > 0 || len(c.Resources.Limits) > 0 {
				fmt.Fprintf(&b, "  resources.requests=%v limits=%v\n", resourceListToStrings(c.Resources.Requests), resourceListToStrings(c.Resources.Limits))
			}
			if c.LivenessProbe != nil {
				fmt.Fprintf(&b, "  livenessProbe: initialDelay=%ds timeout=%ds period=%ds failureThreshold=%d\n",
					c.LivenessProbe.InitialDelaySeconds, c.LivenessProbe.TimeoutSeconds, c.LivenessProbe.PeriodSeconds, c.LivenessProbe.FailureThreshold)
			}
			if c.ReadinessProbe != nil {
				fmt.Fprintf(&b, "  readinessProbe: initialDelay=%ds timeout=%ds period=%ds failureThreshold=%d\n",
					c.ReadinessProbe.InitialDelaySeconds, c.ReadinessProbe.TimeoutSeconds, c.ReadinessProbe.PeriodSeconds, c.ReadinessProbe.FailureThreshold)
			}
		}
	}

	if len(pod.Status.ContainerStatuses) > 0 {
		b.WriteString("Container Statuses:\n")
		for _, cs := range pod.Status.ContainerStatuses {
			fmt.Fprintf(&b, "- %s ready=%t restartCount=%d\n", cs.Name, cs.Ready, cs.RestartCount)
			switch {
			case cs.State.Waiting != nil:
				fmt.Fprintf(&b, "  state=Waiting reason=%s message=%s\n", cs.State.Waiting.Reason, cs.State.Waiting.Message)
			case cs.State.Running != nil:
				fmt.Fprintf(&b, "  state=Running startedAt=%s\n", cs.State.Running.StartedAt.Format(time.RFC3339))
			case cs.State.Terminated != nil:
				fmt.Fprintf(&b, "  state=Terminated reason=%s exitCode=%d finishedAt=%s\n",
					cs.State.Terminated.Reason, cs.State.Terminated.ExitCode, cs.State.Terminated.FinishedAt.Format(time.RFC3339))
			}
			if cs.LastTerminationState.Terminated != nil {
				t := cs.LastTerminationState.Terminated
				fmt.Fprintf(&b, "  lastTermination reason=%s exitCode=%d finishedAt=%s\n", t.Reason, t.ExitCode, t.FinishedAt.Format(time.RFC3339))
			}
		}
	}

	if len(pod.Spec.NodeSelector) > 0 {
		fmt.Fprintf(&b, "Node Selector: %v\n", pod.Spec.NodeSelector)
	}
	if len(pod.Spec.Tolerations) > 0 {
		b.WriteString("Tolerations:\n")
		for _, t := range pod.Spec.Tolerations {
			fmt.Fprintf(&b, "- key=%s operator=%s value=%s effect=%s\n", t.Key, t.Operator, t.Value, t.Effect)
		}
	}

	return strings.TrimSpace(b.String())
}

func k8sEventTimestamp(e corev1.Event) time.Time {
	if !e.EventTime.IsZero() {
		return e.EventTime.Time
	}
	if !e.LastTimestamp.IsZero() {
		return e.LastTimestamp.Time
	}
	if !e.FirstTimestamp.IsZero() {
		return e.FirstTimestamp.Time
	}
	return e.CreationTimestamp.Time
}

func toEventSummary(e corev1.Event) eventSummary {
	ts := k8sEventTimestamp(e)
	first := e.FirstTimestamp.Time
	if first.IsZero() {
		first = ts
	}
	src := e.Source.Component
	if e.Source.Host != "" {
		if src == "" {
			src = e.Source.Host
		} else {
			src += "/" + e.Source.Host
		}
	}
	return eventSummary{
		Type:           e.Type,
		Reason:         e.Reason,
		Message:        e.Message,
		Count:          e.Count,
		FirstTimestamp: formatRFC3339(first),
		LastTimestamp:  formatRFC3339(ts),
		Source:         src,
	}
}

func formatRFC3339(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}
