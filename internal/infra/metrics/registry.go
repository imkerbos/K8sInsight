package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTP 指标
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "k8sinsight_http_requests_total",
			Help: "Total HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "k8sinsight_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// Incident 指标
	IncidentsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "k8sinsight_incidents_total",
			Help: "Total incidents detected",
		},
		[]string{"type", "cluster"},
	)

	IncidentsActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "k8sinsight_incidents_active",
			Help: "Number of currently active incidents",
		},
	)

	// Pipeline 指标
	EvidenceChannelDepth = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "k8sinsight_evidence_channel_depth",
			Help: "Current depth of evidence channel",
		},
	)

	DedupIndexSize = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "k8sinsight_dedup_index_size",
			Help: "Number of entries in the dedup index",
		},
	)

	PipelinesActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "k8sinsight_pipelines_active",
			Help: "Number of active cluster pipelines",
		},
	)
)
