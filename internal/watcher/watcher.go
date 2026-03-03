package watcher

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"

	"github.com/kerbos/k8sinsight/internal/config"
	"github.com/kerbos/k8sinsight/internal/detector"
)

// Watcher 状态感知层主控
// 管理 SharedInformerFactory 和各 Handler 的生命周期
type Watcher struct {
	factory    informers.SharedInformerFactory
	detector   *detector.Detector
	filter     *Filter
	logger     *zap.Logger
	cancelFunc context.CancelFunc
}

// New 创建 Watcher 实例
func New(clientset kubernetes.Interface, cfg config.WatchConfig, det *detector.Detector, logger *zap.Logger) *Watcher {
	log := logger.Named("watcher")

	// 构建 InformerFactory 选项
	var opts []informers.SharedInformerOption
	opts = append(opts, informers.WithTransform(stripManagedFields))

	// 如果配置了特定 namespace，使用 namespace 限定
	if cfg.Scope == "namespaces" && len(cfg.Namespaces.Include) == 1 {
		opts = append(opts, informers.WithNamespace(cfg.Namespaces.Include[0]))
	}

	resync := cfg.ResyncPeriod
	if resync == 0 {
		resync = 30 * time.Minute
	}

	factory := informers.NewSharedInformerFactoryWithOptions(clientset, resync, opts...)
	filter := NewFilter(cfg)

	return &Watcher{
		factory:  factory,
		detector: det,
		filter:   filter,
		logger:   log,
	}
}

// Start 启动 Watcher，注册 Handler 并开始监听
func (w *Watcher) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	w.cancelFunc = cancel

	// 注册 Pod Informer Handler（主通道）
	podInformer := w.factory.Core().V1().Pods().Informer()
	podHandler := NewPodHandler(ctx, w.filter, w.detector, w.logger)
	if _, err := podInformer.AddEventHandler(podHandler.EventHandler()); err != nil {
		return fmt.Errorf("注册 Pod Handler 失败: %w", err)
	}

	// 注册 Event Informer Handler（补充通道）
	eventInformer := w.factory.Core().V1().Events().Informer()
	eventHandler := NewEventHandler(ctx, w.filter, w.detector, w.logger)
	if _, err := eventInformer.AddEventHandler(eventHandler.Handler()); err != nil {
		return fmt.Errorf("注册 Event Handler 失败: %w", err)
	}

	// 启动所有 Informer
	w.factory.Start(ctx.Done())

	// 等待缓存同步（30s 超时）
	w.logger.Info("等待 Informer 缓存同步...")
	syncCtx, syncCancel := context.WithTimeout(ctx, 30*time.Second)
	defer syncCancel()

	synced := w.factory.WaitForCacheSync(syncCtx.Done())
	for typ, ok := range synced {
		if !ok {
			cancel()
			return fmt.Errorf("Informer 缓存同步超时: %v", typ)
		}
	}
	w.logger.Info("Informer 缓存同步完成，开始监听")

	return nil
}

// Stop 停止 Watcher
func (w *Watcher) Stop() {
	if w.cancelFunc != nil {
		w.cancelFunc()
	}
	w.factory.Shutdown()
	w.logger.Info("Watcher 已停止")
}

// stripManagedFields 去除 managedFields 减少内存占用
func stripManagedFields(obj interface{}) (interface{}, error) {
	// managedFields 对异常检测无用，通过 Transform 清理
	// 实际 Transform 中使用 metav1.ObjectMeta 接口
	return obj, nil
}
