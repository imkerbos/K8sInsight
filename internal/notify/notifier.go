package notify

import (
	"context"

	"github.com/kerbos/k8sinsight/internal/detector"
)

// Notifier 通知发送接口
type Notifier interface {
	// Name 通知渠道名称
	Name() string
	// Send 发送异常通知
	Send(ctx context.Context, event detector.AnomalyEvent) error
}
