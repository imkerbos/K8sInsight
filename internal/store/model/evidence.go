package model

import "time"

// Evidence 证据数据库模型
type Evidence struct {
	ID          string    `gorm:"primaryKey;type:uuid" json:"id"`
	IncidentID  string    `gorm:"index;not null;type:uuid" json:"incidentId"`
	Type        string    `gorm:"not null" json:"type"` // PreviousLogs, CurrentLogs, PodEvents, PodSnapshot, PodDescribe, WorkloadSpec, NodeContext, Metrics
	Content     string    `gorm:"type:text" json:"content"`
	Error       string    `gorm:"type:text" json:"error,omitempty"`
	CollectedAt time.Time `gorm:"not null" json:"collectedAt"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (Evidence) TableName() string {
	return "evidences"
}

// TimelineEntry 时间线条目数据库模型
type TimelineEntry struct {
	ID         string    `gorm:"primaryKey;type:uuid" json:"id"`
	IncidentID string    `gorm:"index;not null;type:uuid" json:"incidentId"`
	Timestamp  time.Time `gorm:"not null;index" json:"timestamp"`
	Type       string    `gorm:"not null" json:"type"` // StateChange, EventDetected, EvidenceCollected
	Summary    string    `gorm:"type:text" json:"summary"`
	Detail     string    `gorm:"type:text" json:"detail,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}

func (TimelineEntry) TableName() string {
	return "timeline_entries"
}
