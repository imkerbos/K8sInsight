package notify

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/kerbos/k8sinsight/internal/detector"
)

// Loader 动态加载通知配置
// 返回值: enabled, notifiers, error
type Loader func(ctx context.Context) (bool, []Notifier, error)

// Dispatcher 通知分发器
type Dispatcher struct {
	notifiers []Notifier
	logger    *zap.Logger
	enabled   bool
	loader    Loader
}

// NewDispatcher 创建通知分发器
func NewDispatcher(enabled bool, logger *zap.Logger) *Dispatcher {
	return &Dispatcher{
		logger:  logger.Named("notify"),
		enabled: enabled,
	}
}

// NewDynamicDispatcher 创建动态通知分发器（每次事件按需加载通知配置）
func NewDynamicDispatcher(loader Loader, logger *zap.Logger) *Dispatcher {
	return &Dispatcher{
		logger: logger.Named("notify"),
		loader: loader,
	}
}

// AddNotifier 注册通知渠道
func (d *Dispatcher) AddNotifier(n Notifier) {
	d.notifiers = append(d.notifiers, n)
}

// HandleAnomaly 实现 detector.EventSink 接口
func (d *Dispatcher) HandleAnomaly(ctx context.Context, event detector.AnomalyEvent) error {
	enabled := d.enabled
	notifiers := d.notifiers

	if d.loader != nil {
		var err error
		enabled, notifiers, err = d.loader(ctx)
		if err != nil {
			d.logger.Error("动态加载通知配置失败", zap.Error(err))
			return fmt.Errorf("动态加载通知配置失败: %w", err)
		}
	}

	if !enabled {
		return nil
	}

	for _, n := range notifiers {
		if err := n.Send(ctx, event); err != nil {
			d.logger.Error("发送通知失败",
				zap.String("notifier", n.Name()),
				zap.Error(err),
			)
		}
	}
	return nil
}
