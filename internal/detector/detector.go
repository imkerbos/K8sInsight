package detector

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// EventSink 异常事件输出接口
// Collector、Aggregator 等下游模块实现此接口
type EventSink interface {
	HandleAnomaly(ctx context.Context, event AnomalyEvent) error
}

// Detector 异常检测引擎
type Detector struct {
	rules     []Rule
	sinks     []EventSink
	clientset kubernetes.Interface
	clusterID string
	logger    *zap.Logger
}

// NewDetector 创建异常检测引擎
func NewDetector(clientset kubernetes.Interface, logger *zap.Logger) *Detector {
	d := &Detector{
		clientset: clientset,
		logger:    logger.Named("detector"),
	}
	// 注册默认规则
	d.rules = []Rule{
		NewCrashLoopRule(),
		NewOOMKilledRule(),
		NewErrorExitRule(),
		NewImagePullBackOffRule(),
		NewCreateContainerConfigErrorRule(),
		NewEvictedRule(),
	}
	return d
}

// AddSink 注册异常事件消费方
func (d *Detector) AddSink(sink EventSink) {
	d.sinks = append(d.sinks, sink)
}

// AddRule 注册自定义检测规则
func (d *Detector) AddRule(rule Rule) {
	d.rules = append(d.rules, rule)
}

// SetClusterID 设置集群标识，注入到所有产出的 AnomalyEvent
func (d *Detector) SetClusterID(id string) {
	d.clusterID = id
}

// EvaluatePodChange 评估 Pod 状态变化
// 由 Watcher 的 Pod Handler 调用
func (d *Detector) EvaluatePodChange(ctx context.Context, oldPod, newPod *corev1.Pod) {
	for _, rule := range d.rules {
		oldP := oldPod
		newP := newPod
		anomalies := rule.Evaluate(oldP, newP)
		for i := range anomalies {
			// 尝试解析 ReplicaSet → Deployment
			d.resolveDeploymentOwner(ctx, &anomalies[i])
			d.dispatch(ctx, anomalies[i])
		}
	}
}

// EvaluateEvent 评估 K8s Warning Event
// 由 Watcher 的 Event Handler 调用
func (d *Detector) EvaluateEvent(ctx context.Context, event *corev1.Event) {
	if event == nil || event.Type != "Warning" {
		return
	}

	var anomalyType AnomalyType
	switch event.Reason {
	case "FailedScheduling":
		anomalyType = AnomalyFailedScheduling
	default:
		return
	}

	anomaly := AnomalyEvent{
		Type:      anomalyType,
		Source:    SourceK8sEvent,
		Message:   fmt.Sprintf("[%s] %s", event.Reason, event.Message),
		PodName:   event.InvolvedObject.Name,
		Namespace: event.InvolvedObject.Namespace,
		PodUID:    string(event.InvolvedObject.UID),
		Reason:    event.Reason,
	}
	d.dispatch(ctx, anomaly)
}

// resolveDeploymentOwner 如果 Owner 是 ReplicaSet，追溯到 Deployment
func (d *Detector) resolveDeploymentOwner(ctx context.Context, event *AnomalyEvent) {
	if event.OwnerKind != "ReplicaSet" || event.OwnerName == "" {
		return
	}

	rs, err := d.clientset.AppsV1().ReplicaSets(event.Namespace).Get(
		ctx, event.OwnerName, metav1.GetOptions{},
	)
	if err != nil {
		d.logger.Debug("无法查询 ReplicaSet Owner", zap.String("rs", event.OwnerName), zap.Error(err))
		return
	}

	for _, ref := range rs.OwnerReferences {
		if ref.Kind == "Deployment" {
			event.OwnerKind = "Deployment"
			event.OwnerName = ref.Name
			return
		}
	}
}

func (d *Detector) dispatch(ctx context.Context, event AnomalyEvent) {
	event.ClusterID = d.clusterID

	d.logger.Info("检测到异常",
		zap.String("type", string(event.Type)),
		zap.String("pod", event.PodName),
		zap.String("namespace", event.Namespace),
		zap.String("dedupKey", event.DedupKey()),
		zap.String("message", event.Message),
	)

	for _, sink := range d.sinks {
		if err := sink.HandleAnomaly(ctx, event); err != nil {
			d.logger.Error("分发异常事件失败", zap.Error(err))
		}
	}
}
