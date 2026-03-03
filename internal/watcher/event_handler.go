package watcher

import (
	"context"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/kerbos/k8sinsight/internal/detector"
)

// EventHandler K8s Event Informer 事件处理器（补充通道）
type EventHandler struct {
	filter   *Filter
	detector *detector.Detector
	logger   *zap.Logger
	ctx      context.Context
}

// NewEventHandler 创建 Event 事件处理器
func NewEventHandler(ctx context.Context, filter *Filter, det *detector.Detector, logger *zap.Logger) *EventHandler {
	return &EventHandler{
		filter:   filter,
		detector: det,
		logger:   logger.Named("event-handler"),
		ctx:      ctx,
	}
}

// Handler 返回 cache.ResourceEventHandlerFuncs
func (h *EventHandler) Handler() cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: h.onAdd,
	}
}

// onAdd 处理新产生的 K8s Event
func (h *EventHandler) onAdd(obj interface{}) {
	event, ok := obj.(*corev1.Event)
	if !ok {
		return
	}

	// 只关注 Warning 类型事件
	if event.Type != "Warning" {
		return
	}

	// 只关注 Pod 相关事件
	if event.InvolvedObject.Kind != "Pod" {
		return
	}

	// Namespace 过滤
	if !h.filter.ShouldProcessNamespace(event.InvolvedObject.Namespace) {
		return
	}

	h.logger.Debug("捕获 Warning Event",
		zap.String("reason", event.Reason),
		zap.String("pod", event.InvolvedObject.Name),
		zap.String("namespace", event.InvolvedObject.Namespace),
		zap.String("message", event.Message),
	)

	h.detector.EvaluateEvent(h.ctx, event)
}
