package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"k8s.io/client-go/kubernetes"

	"github.com/kerbos/k8sinsight/internal/config"
	"github.com/kerbos/k8sinsight/internal/detector"
)

// collectMetrics 采集 Pod 资源使用指标
// 通过 Metrics API (metrics.k8s.io) 获取 CPU/Memory 实时用量
func collectMetrics(
	ctx context.Context,
	client kubernetes.Interface,
	event detector.AnomalyEvent,
	cfg config.CollectConfig,
	timeout time.Duration,
) Evidence {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 通过 RESTClient 访问 metrics.k8s.io API
	data, err := client.CoreV1().RESTClient().
		Get().
		AbsPath("/apis/metrics.k8s.io/v1beta1").
		Namespace(event.Namespace).
		Resource("pods").
		Name(event.PodName).
		DoRaw(ctx)

	if err == nil {
		return Evidence{
			Type:      EvidenceMetrics,
			Content:   string(data),
			Timestamp: time.Now(),
		}
	}

	// metrics.k8s.io 不可用时，回退到 Prometheus query_range
	if strings.TrimSpace(cfg.PrometheusURL) != "" {
		content, pErr := collectPrometheusRange(ctx, cfg.PrometheusURL, cfg.PromQueryRange, event)
		if pErr == nil {
			return Evidence{
				Type:      EvidenceMetrics,
				Content:   content,
				Timestamp: time.Now(),
			}
		}
		return Evidence{
			Type:      EvidenceMetrics,
			Timestamp: time.Now(),
			Error:     fmt.Sprintf("metrics.k8s.io: %v; prometheus: %v", err, pErr),
		}
	}

	return Evidence{
		Type:      EvidenceMetrics,
		Timestamp: time.Now(),
		Error:     err.Error(),
	}
}

type promRangeResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string            `json:"resultType"`
		Result     []json.RawMessage `json:"result"`
	} `json:"data"`
	Error string `json:"error"`
}

type promMetricsBundle struct {
	Source string `json:"source"`
	Pod    struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"pod"`
	Window struct {
		Start string `json:"start"`
		End   string `json:"end"`
	} `json:"window"`
	Queries struct {
		Memory string `json:"memory"`
		CPU    string `json:"cpu"`
	} `json:"queries"`
	Series map[string][]json.RawMessage `json:"series"`
}

func collectPrometheusRange(
	ctx context.Context,
	baseURL string,
	rangeDur time.Duration,
	event detector.AnomalyEvent,
) (string, error) {
	if rangeDur <= 0 {
		rangeDur = 10 * time.Minute
	}

	end := event.Timestamp
	if end.IsZero() {
		end = time.Now()
	}
	start := end.Add(-rangeDur)
	step := "15s"

	memQuery := fmt.Sprintf(
		`sum(container_memory_working_set_bytes{namespace="%s",pod="%s"}) by (pod)`,
		event.Namespace, event.PodName,
	)
	cpuQuery := fmt.Sprintf(
		`sum(rate(container_cpu_usage_seconds_total{namespace="%s",pod="%s"}[1m])) by (pod)`,
		event.Namespace, event.PodName,
	)

	memSeries, err := queryPrometheusRange(ctx, baseURL, memQuery, start, end, step)
	if err != nil {
		return "", err
	}
	cpuSeries, err := queryPrometheusRange(ctx, baseURL, cpuQuery, start, end, step)
	if err != nil {
		return "", err
	}

	b := promMetricsBundle{
		Source: "prometheus",
		Series: map[string][]json.RawMessage{
			"memory": memSeries,
			"cpu":    cpuSeries,
		},
	}
	b.Pod.Name = event.PodName
	b.Pod.Namespace = event.Namespace
	b.Window.Start = start.Format(time.RFC3339)
	b.Window.End = end.Format(time.RFC3339)
	b.Queries.Memory = memQuery
	b.Queries.CPU = cpuQuery

	out, err := json.Marshal(b)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func queryPrometheusRange(
	ctx context.Context,
	baseURL, promQuery string,
	start, end time.Time,
	step string,
) ([]json.RawMessage, error) {
	u, err := url.Parse(strings.TrimRight(baseURL, "/") + "/api/v1/query_range")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("query", promQuery)
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

	var pr promRangeResponse
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 || pr.Status != "success" {
		if pr.Error == "" {
			pr.Error = resp.Status
		}
		return nil, fmt.Errorf("query_range failed: %s", pr.Error)
	}
	return pr.Data.Result, nil
}
