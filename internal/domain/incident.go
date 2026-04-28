package domain

import "time"

// IncidentState 事件状态
type IncidentState string

const (
	StateDetecting IncidentState = "Detecting"
	StateActive    IncidentState = "Active"
	StateResolved  IncidentState = "Resolved"
	StateArchived  IncidentState = "Archived"
)

// Incident 聚合后的事件实体（一次完整异常的生命周期）
type Incident struct {
	ID          string        `json:"id"`
	DedupKey    string        `json:"dedupKey"`
	State       IncidentState `json:"state"`
	FirstSeen   time.Time     `json:"firstSeen"`
	LastSeen    time.Time     `json:"lastSeen"`
	Count       int           `json:"count"`
	Namespace   string        `json:"namespace"`
	OwnerKind   string        `json:"ownerKind"`
	OwnerName   string        `json:"ownerName"`
	AnomalyType string        `json:"anomalyType"`
	Message     string        `json:"message"`
	PodNames    []string      `json:"podNames"`
	ClusterID   string        `json:"clusterId,omitempty"`
}
