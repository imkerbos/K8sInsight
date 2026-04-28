package domain

import "time"

// EvidenceType 证据类型
type EvidenceType string

const (
	EvidencePreviousLogs EvidenceType = "PreviousLogs"
	EvidenceCurrentLogs  EvidenceType = "CurrentLogs"
	EvidencePodEvents    EvidenceType = "PodEvents"
	EvidencePodSnapshot  EvidenceType = "PodSnapshot"
	EvidencePodDescribe  EvidenceType = "PodDescribe"
	EvidenceWorkloadSpec EvidenceType = "WorkloadSpec"
	EvidenceNodeContext  EvidenceType = "NodeContext"
	EvidenceMetrics      EvidenceType = "Metrics"
)

// Evidence 采集到的证据
type Evidence struct {
	Type      EvidenceType `json:"type"`
	Content   string       `json:"content"`
	Timestamp time.Time    `json:"timestamp"`
	Error     string       `json:"error,omitempty"`
}

// EvidenceBundle 一次异常采集到的全部证据
type EvidenceBundle struct {
	AnomalyEvent AnomalyEvent
	Evidences    []Evidence
	CollectedAt  time.Time
}
