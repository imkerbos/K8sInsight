package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/kerbos/k8sinsight/internal/cluster"
	"github.com/kerbos/k8sinsight/internal/core"
	"github.com/kerbos/k8sinsight/internal/store/model"
	"github.com/kerbos/k8sinsight/internal/store/repository"
)

// ClusterService 集群业务逻辑
type ClusterService struct {
	repo       repository.ClusterRepository
	pipeline   *cluster.Manager
	logger     *zap.Logger
}

// NewClusterService 创建集群服务
func NewClusterService(
	repo repository.ClusterRepository,
	pipeline *cluster.Manager,
	logger *zap.Logger,
) *ClusterService {
	return &ClusterService{
		repo:     repo,
		pipeline: pipeline,
		logger:   logger.Named("svc.cluster"),
	}
}

// Create 创建集群并自动启动监控管道
func (s *ClusterService) Create(ctx context.Context, name, kubeconfigData, prometheusURL, prometheusLabels string) (*model.Cluster, error) {
	cl := &model.Cluster{
		ID:               uuid.New().String(),
		Name:             name,
		KubeconfigData:   kubeconfigData,
		PrometheusURL:    strings.TrimSpace(prometheusURL),
		PrometheusLabels: strings.TrimSpace(prometheusLabels),
		Status:           "active",
		ConnectionStatus: "unknown",
	}

	if err := s.repo.Create(ctx, cl); err != nil {
		return nil, fmt.Errorf("创建集群失败: %w", err)
	}

	if err := s.pipeline.StartCluster(ctx, cl); err != nil {
		s.logger.Error("集群已创建但管道启动失败", zap.String("clusterID", cl.ID), zap.Error(err))
	}

	s.logger.Info("集群已创建", zap.String("id", cl.ID), zap.String("name", cl.Name))
	return cl, nil
}

// Update 更新集群配置
func (s *ClusterService) Update(ctx context.Context, id string, name, kubeconfigData string, prometheusURL, prometheusLabels *string) (*model.Cluster, error) {
	cl, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("集群未找到: %w", err)
	}

	if name != "" {
		cl.Name = name
	}
	if kubeconfigData != "" {
		cl.KubeconfigData = kubeconfigData
		cl.ConnectionStatus = "unknown"
		cl.Version = ""
		cl.NodeCount = 0
		cl.StatusMessage = ""
	}
	if prometheusURL != nil {
		cl.PrometheusURL = strings.TrimSpace(*prometheusURL)
	}
	if prometheusLabels != nil {
		cl.PrometheusLabels = strings.TrimSpace(*prometheusLabels)
	}

	if err := s.repo.Update(ctx, cl); err != nil {
		return nil, fmt.Errorf("更新集群失败: %w", err)
	}

	return cl, nil
}

// Delete 删除集群（先停管道）
func (s *ClusterService) Delete(ctx context.Context, id string) error {
	if s.pipeline.IsRunning(id) {
		if err := s.pipeline.StopCluster(id); err != nil {
			s.logger.Error("停止集群管道失败", zap.Error(err))
		}
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("删除集群失败: %w", err)
	}

	return nil
}

// List 获取集群列表
func (s *ClusterService) List(ctx context.Context) ([]model.Cluster, error) {
	return s.repo.List(ctx)
}

// GetByID 获取集群详情
func (s *ClusterService) GetByID(ctx context.Context, id string) (*model.Cluster, error) {
	return s.repo.FindByID(ctx, id)
}

// TestConnectionResult 测试连接结果
type TestConnectionResult struct {
	Success   bool   `json:"success"`
	Version   string `json:"version,omitempty"`
	NodeCount int    `json:"nodeCount,omitempty"`
	Error     string `json:"error,omitempty"`
}

// TestConnection 测试集群连接
func (s *ClusterService) TestConnection(ctx context.Context, id string) (*TestConnectionResult, error) {
	cl, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("集群未找到: %w", err)
	}

	info, err := core.TestKubeConnection(cl.KubeconfigData)
	if err != nil {
		cl.ConnectionStatus = "failed"
		cl.StatusMessage = err.Error()
		cl.Version = ""
		cl.NodeCount = 0
		_ = s.repo.Update(ctx, cl)
		return &TestConnectionResult{Success: false, Error: err.Error()}, nil
	}

	cl.ConnectionStatus = "connected"
	cl.StatusMessage = ""
	cl.Version = info.Version
	cl.NodeCount = info.NodeCount
	_ = s.repo.Update(ctx, cl)

	return &TestConnectionResult{
		Success:   true,
		Version:   info.Version,
		NodeCount: info.NodeCount,
	}, nil
}

// Activate 启用集群并启动管道
func (s *ClusterService) Activate(ctx context.Context, id string) (*model.Cluster, error) {
	cl, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("集群未找到: %w", err)
	}

	cl.Status = "active"
	if err := s.repo.Update(ctx, cl); err != nil {
		return nil, fmt.Errorf("更新状态失败: %w", err)
	}

	if !s.pipeline.IsRunning(cl.ID) {
		if err := s.pipeline.StartCluster(ctx, cl); err != nil {
			s.logger.Error("启动集群管道失败", zap.String("clusterID", cl.ID), zap.Error(err))
			return cl, fmt.Errorf("集群已启用，但管道启动失败: %w", err)
		}
	}

	return cl, nil
}

// Deactivate 禁用集群并停止管道
func (s *ClusterService) Deactivate(ctx context.Context, id string) (*model.Cluster, error) {
	cl, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("集群未找到: %w", err)
	}

	if s.pipeline.IsRunning(cl.ID) {
		if err := s.pipeline.StopCluster(cl.ID); err != nil {
			s.logger.Error("停止集群管道失败", zap.Error(err))
		}
	}

	cl.Status = "inactive"
	if err := s.repo.Update(ctx, cl); err != nil {
		return nil, fmt.Errorf("更新状态失败: %w", err)
	}

	return cl, nil
}

// ClusterMetrics 集群指标查询结果
type ClusterMetrics struct {
	ClusterID string              `json:"clusterId"`
	Range     string              `json:"range"`
	Step      string              `json:"step"`
	Series    map[string][]TSPair `json:"series"`
}

// TSPair Prometheus 时序数据点 [timestamp, value]
type TSPair [2]json.Number

// GetMetrics 查询集群级别的 Prometheus 指标
func (s *ClusterService) GetMetrics(ctx context.Context, id string, rangeDur time.Duration) (*ClusterMetrics, error) {
	cl, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("集群未找到: %w", err)
	}

	// 优先使用集群自身配置的 Prometheus 地址，没有则回退到全局配置
	promURL := strings.TrimSpace(cl.PrometheusURL)
	if promURL == "" {
		promURL = s.pipeline.GetPrometheusURL(ctx)
	}
	if promURL == "" {
		return nil, fmt.Errorf("未配置 Prometheus 地址，请在集群设置或 系统管理 → 资源采集 中配置")
	}

	if !s.pipeline.IsRunning(cl.ID) {
		return nil, fmt.Errorf("集群管道未运行，请先启用集群")
	}

	end := time.Now()
	start := end.Add(-rangeDur)
	step := pickStep(rangeDur)

	// 构建额外的标签过滤条件（用于多集群场景，如 cluster="biz-1"）
	extraLabels := strings.TrimSpace(cl.PrometheusLabels)
	lf := func(base string) string {
		if extraLabels == "" {
			return base
		}
		return base + "," + extraLabels
	}

	queries := map[string]string{
		"cpu_usage":     fmt.Sprintf(`sum(rate(container_cpu_usage_seconds_total{%s}[2m]))`, lf(`image!=""`)),
		"memory_usage":  fmt.Sprintf(`sum(container_memory_working_set_bytes{%s})`, lf(`image!=""`)),
		"network_rx":    fmt.Sprintf(`sum(rate(container_network_receive_bytes_total{%s}[2m]))`, lf(`interface="eth0"`)),
		"network_tx":    fmt.Sprintf(`sum(rate(container_network_transmit_bytes_total{%s}[2m]))`, lf(`interface="eth0"`)),
		"pod_count":     fmt.Sprintf(`count(kube_pod_info{%s}) or vector(0)`, extraLabels),
		"cpu_requests":  fmt.Sprintf(`sum(kube_pod_container_resource_requests{%s}) or vector(0)`, lf(`resource="cpu"`)),
		"mem_requests":  fmt.Sprintf(`sum(kube_pod_container_resource_requests{%s}) or vector(0)`, lf(`resource="memory"`)),
	}

	result := &ClusterMetrics{
		ClusterID: cl.ID,
		Range:     rangeDur.String(),
		Step:      step,
		Series:    make(map[string][]TSPair),
	}

	queryCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	for name, query := range queries {
		values, err := queryPrometheus(queryCtx, promURL, query, start, end, step)
		if err != nil {
			s.logger.Warn("集群指标查询失败", zap.String("metric", name), zap.Error(err))
			result.Series[name] = []TSPair{}
			continue
		}
		result.Series[name] = values
	}

	return result, nil
}

func pickStep(rangeDur time.Duration) string {
	switch {
	case rangeDur <= time.Hour:
		return "15s"
	case rangeDur <= 6*time.Hour:
		return "60s"
	case rangeDur <= 24*time.Hour:
		return "120s"
	default:
		return "300s"
	}
}

type promResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Values []TSPair `json:"values"`
		} `json:"result"`
	} `json:"data"`
	Error string `json:"error"`
}

func queryPrometheus(ctx context.Context, baseURL, query string, start, end time.Time, step string) ([]TSPair, error) {
	u, err := url.Parse(strings.TrimRight(baseURL, "/") + "/api/v1/query_range")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("query", query)
	q.Set("start", fmt.Sprintf("%d", start.Unix()))
	q.Set("end", fmt.Sprintf("%d", end.Unix()))
	q.Set("step", step)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var pr promResponse
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 || pr.Status != "success" {
		if pr.Error == "" {
			pr.Error = resp.Status
		}
		return nil, fmt.Errorf("prometheus query failed: %s", pr.Error)
	}

	if len(pr.Data.Result) == 0 {
		return []TSPair{}, nil
	}
	return pr.Data.Result[0].Values, nil
}
