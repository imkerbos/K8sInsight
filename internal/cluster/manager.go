package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"k8s.io/client-go/kubernetes"

	"github.com/kerbos/k8sinsight/internal/aggregator"
	"github.com/kerbos/k8sinsight/internal/collector"
	"github.com/kerbos/k8sinsight/internal/config"
	"github.com/kerbos/k8sinsight/internal/core"
	"github.com/kerbos/k8sinsight/internal/detector"
	"github.com/kerbos/k8sinsight/internal/notify"
	"github.com/kerbos/k8sinsight/internal/notify/sink"
	"github.com/kerbos/k8sinsight/internal/store/model"
	"github.com/kerbos/k8sinsight/internal/store/repository"
	"github.com/kerbos/k8sinsight/internal/watcher"
)

// Pipeline 表示一个集群的完整 watcher 管道
type Pipeline struct {
	ClusterID  string
	clientset  kubernetes.Interface
	watcher    *watcher.Watcher
	aggregator *aggregator.Aggregator
	cancelFunc context.CancelFunc
}

// Manager 管理所有集群的 watcher 管道生命周期
type Manager struct {
	mu              sync.RWMutex
	pipelines       map[string]*Pipeline
	clusterRepo     repository.ClusterRepository
	monitorRuleRepo repository.MonitorRuleRepository
	incidentRepo    repository.IncidentRepository
	evidenceRepo    repository.EvidenceRepository
	settingRepo     repository.SettingRepository
	cfg             *config.Config
	logger          *zap.Logger
}

// NewManager 创建集群管理器
func NewManager(
	clusterRepo repository.ClusterRepository,
	monitorRuleRepo repository.MonitorRuleRepository,
	incidentRepo repository.IncidentRepository,
	evidenceRepo repository.EvidenceRepository,
	settingRepo repository.SettingRepository,
	cfg *config.Config,
	logger *zap.Logger,
) *Manager {
	return &Manager{
		pipelines:       make(map[string]*Pipeline),
		clusterRepo:     clusterRepo,
		monitorRuleRepo: monitorRuleRepo,
		incidentRepo:    incidentRepo,
		evidenceRepo:    evidenceRepo,
		settingRepo:     settingRepo,
		cfg:             cfg,
		logger:          logger.Named("cluster-mgr"),
	}
}

// StartCluster 为指定集群创建并启动完整的 watcher 管道
func (m *Manager) StartCluster(ctx context.Context, cluster *model.Cluster) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.pipelines[cluster.ID]; exists {
		return fmt.Errorf("集群 %s 的管道已在运行", cluster.Name)
	}

	// 从 kubeconfig 内容创建客户端
	clientset, err := core.NewKubeClientFromContent(cluster.KubeconfigData)
	if err != nil {
		m.updateClusterConnectionStatus(ctx, cluster.ID, "failed", err.Error())
		return fmt.Errorf("创建 K8s 客户端失败: %w", err)
	}

	// 构建 WatchConfig
	watchCfg := m.buildWatchConfig(cluster)

	// 创建管道 context
	pipelineCtx, cancel := context.WithCancel(ctx)

	// 证据传输 channel
	evidenceCh := make(chan collector.EvidenceBundle, 100)

	// 初始化组件
	det := detector.NewDetector(clientset, m.logger)
	det.SetClusterID(cluster.ID)

	col := collector.NewCollector(clientset, m.cfg.Collect, evidenceCh, m.logger)
	col.SetConfigLoader(m.loadCollectConfig)

	dispatcher := notify.NewDynamicDispatcher(m.loadNotifyNotifiers, m.logger)
	agg := aggregator.NewAggregator(m.cfg.Watch.Aggregation, m.incidentRepo, m.evidenceRepo, dispatcher, evidenceCh, m.logger)

	// 注册 EventSink
	det.AddSink(col)
	det.AddSink(agg)

	// 启动聚合引擎
	agg.Start(pipelineCtx)

	// 创建并启动 Watcher
	w := watcher.New(clientset, watchCfg, det, m.logger)
	if err := w.Start(pipelineCtx); err != nil {
		cancel()
		m.updateClusterConnectionStatus(ctx, cluster.ID, "failed", err.Error())
		return fmt.Errorf("Watcher 启动失败: %w", err)
	}

	m.pipelines[cluster.ID] = &Pipeline{
		ClusterID:  cluster.ID,
		clientset:  clientset,
		watcher:    w,
		aggregator: agg,
		cancelFunc: cancel,
	}

	m.updateClusterConnectionStatus(ctx, cluster.ID, "connected", "")
	m.logger.Info("集群管道已启动",
		zap.String("clusterID", cluster.ID),
		zap.String("clusterName", cluster.Name),
	)

	return nil
}

func (m *Manager) loadCollectConfig(ctx context.Context) config.CollectConfig {
	fallback := m.cfg.Collect
	if m.settingRepo == nil {
		return fallback
	}

	cfg := fallback
	if v, ok := m.readSetting(ctx, "collect_enable_metrics"); ok && strings.TrimSpace(v) != "" {
		parsed, parseErr := strconv.ParseBool(v)
		if parseErr != nil {
			m.logger.Warn("解析 collect_enable_metrics 失败，使用回退值", zap.String("value", v), zap.Error(parseErr))
		} else {
			cfg.EnableMetrics = parsed
		}
	}
	// 显式覆盖（允许设置为空字符串以清空地址）
	if v, ok := m.readSetting(ctx, "collect_prometheus_url"); ok {
		cfg.PrometheusURL = strings.TrimSpace(v)
	}
	if v, ok := m.readSetting(ctx, "collect_prom_query_range"); ok && strings.TrimSpace(v) != "" {
		parsed, parseErr := time.ParseDuration(v)
		if parseErr != nil {
			m.logger.Warn("解析 collect_prom_query_range 失败，使用回退值", zap.String("value", v), zap.Error(parseErr))
		} else {
			cfg.PromQueryRange = parsed
		}
	}

	return cfg
}

func (m *Manager) readSetting(ctx context.Context, key string) (string, bool) {
	v, err := m.settingRepo.Get(ctx, key)
	if err == nil {
		return v, true
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false
	}
	m.logger.Warn("读取系统设置失败", zap.String("key", key), zap.Error(err))
	return "", false
}

func (m *Manager) loadNotifyNotifiers(ctx context.Context) (bool, []notify.Notifier, error) {
	// 默认回退到配置文件
	fallbackEnabled := m.cfg.Notify.Enabled
	fallbackNotifiers := m.buildNotifiers(
		"webhook",
		m.cfg.Notify.Webhooks,
		m.cfg.Notify.Larks,
		m.cfg.Notify.Telegrams,
	)

	if m.settingRepo == nil {
		return fallbackEnabled, fallbackNotifiers, nil
	}

	settings, err := m.settingRepo.BatchGet(ctx, []string{
		"notify_enabled",
		"notify_channel",
		"notify_webhooks",
		"notify_larks",
		"notify_telegrams",
	})
	if err != nil {
		m.logger.Warn("读取通知设置失败，回退配置文件", zap.Error(err))
		return fallbackEnabled, fallbackNotifiers, nil
	}

	enabled := fallbackEnabled
	channel := "webhook"
	if v, ok := settings["notify_enabled"]; ok && v != "" {
		enabled = v == "true"
	}
	if v, ok := settings["notify_channel"]; ok && v != "" {
		channel = v
	}

	webhooks := m.cfg.Notify.Webhooks
	larks := m.cfg.Notify.Larks
	telegrams := m.cfg.Notify.Telegrams

	if raw := settings["notify_webhooks"]; raw != "" {
		var parsed []config.WebhookSink
		if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
			m.logger.Warn("解析 notify_webhooks 失败，回退配置文件", zap.Error(err))
		} else {
			webhooks = parsed
		}
	}
	if raw := settings["notify_larks"]; raw != "" {
		var parsed []config.LarkSink
		if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
			m.logger.Warn("解析 notify_larks 失败，回退配置文件", zap.Error(err))
		} else {
			larks = parsed
		}
	}
	if raw := settings["notify_telegrams"]; raw != "" {
		var parsed []config.TelegramSink
		if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
			m.logger.Warn("解析 notify_telegrams 失败，回退配置文件", zap.Error(err))
		} else {
			telegrams = parsed
		}
	}

	return enabled, m.buildNotifiers(channel, webhooks, larks, telegrams), nil
}

func (m *Manager) buildNotifiers(
	channel string,
	webhooks []config.WebhookSink,
	larks []config.LarkSink,
	telegrams []config.TelegramSink,
) []notify.Notifier {
	notifiers := make([]notify.Notifier, 0, 1)
	switch channel {
	case "lark":
		for _, lk := range larks {
			if lk.URL == "" {
				m.logger.Warn("跳过无效 Lark 通知配置", zap.String("name", lk.Name))
				continue
			}
			notifiers = append(notifiers, sink.NewLark(lk.Name, lk.URL, lk.Secret))
		}
	case "telegram":
		for _, tg := range telegrams {
			if tg.BotToken == "" || tg.ChatID == "" {
				m.logger.Warn("跳过无效 Telegram 通知配置", zap.String("name", tg.Name))
				continue
			}
			notifiers = append(notifiers, sink.NewTelegram(tg.Name, tg.BotToken, tg.ChatID, tg.ParseMode))
		}
	default:
		for _, wh := range webhooks {
			if wh.URL == "" {
				m.logger.Warn("跳过无效 Webhook 通知配置", zap.String("name", wh.Name))
				continue
			}
			notifiers = append(notifiers, sink.NewWebhook(wh.Name, wh.URL, wh.Headers))
		}
	}
	return notifiers
}

// StopCluster 停止指定集群的 watcher 管道
func (m *Manager) StopCluster(clusterID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, exists := m.pipelines[clusterID]
	if !exists {
		return nil
	}

	p.cancelFunc()
	p.watcher.Stop()
	delete(m.pipelines, clusterID)

	m.logger.Info("集群管道已停止", zap.String("clusterID", clusterID))
	return nil
}

// StopAll 停止所有集群管道（优雅关停）
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, p := range m.pipelines {
		p.cancelFunc()
		p.watcher.Stop()
		m.logger.Info("集群管道已停止", zap.String("clusterID", id))
	}
	m.pipelines = make(map[string]*Pipeline)
}

// ReloadFromDB 从数据库重载所有 active 状态的集群并启动管道
func (m *Manager) ReloadFromDB(ctx context.Context) error {
	clusters, err := m.clusterRepo.FindActive(ctx)
	if err != nil {
		return fmt.Errorf("查询活跃集群失败: %w", err)
	}

	m.logger.Info("从数据库重载集群", zap.Int("count", len(clusters)))

	for i := range clusters {
		c := &clusters[i]
		if err := m.StartCluster(ctx, c); err != nil {
			m.logger.Error("启动集群管道失败",
				zap.String("clusterID", c.ID),
				zap.String("clusterName", c.Name),
				zap.Error(err),
			)
		}
	}

	return nil
}

// IsRunning 检查指定集群的管道是否正在运行
func (m *Manager) IsRunning(clusterID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.pipelines[clusterID]
	return exists
}

// buildWatchConfig 根据监控规则构建 WatchConfig
func (m *Manager) buildWatchConfig(cluster *model.Cluster) config.WatchConfig {
	watchCfg := m.cfg.Watch

	rule, err := m.monitorRuleRepo.FindByClusterID(context.Background(), cluster.ID)
	if err != nil || !rule.Enabled {
		return watchCfg
	}

	if rule.WatchScope != "" {
		watchCfg.Scope = rule.WatchScope
	}

	if rule.WatchScope == "namespaces" && rule.WatchNamespaces != "" {
		var nsList []string
		for _, ns := range strings.Split(rule.WatchNamespaces, ",") {
			ns = strings.TrimSpace(ns)
			if ns != "" {
				nsList = append(nsList, ns)
			}
		}
		if len(nsList) > 0 {
			watchCfg.Namespaces.Include = nsList
		}
	}

	if rule.LabelSelector != "" {
		watchCfg.LabelSelector = rule.LabelSelector
	}

	return watchCfg
}

// updateClusterConnectionStatus 更新集群连接状态（内部方法，不持锁）
func (m *Manager) updateClusterConnectionStatus(ctx context.Context, clusterID, connStatus, message string) {
	cluster, err := m.clusterRepo.FindByID(ctx, clusterID)
	if err != nil {
		m.logger.Error("更新集群状态失败：找不到集群", zap.String("clusterID", clusterID), zap.Error(err))
		return
	}
	cluster.ConnectionStatus = connStatus
	cluster.StatusMessage = message
	if err := m.clusterRepo.Update(ctx, cluster); err != nil {
		m.logger.Error("更新集群状态失败", zap.String("clusterID", clusterID), zap.Error(err))
	}
}
