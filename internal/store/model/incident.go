package model

import "time"

// Incident 异常事件数据库模型
type Incident struct {
	ID          string    `gorm:"primaryKey;type:uuid" json:"id"`
	DedupKey    string    `gorm:"index;not null" json:"dedupKey"`
	State       string    `gorm:"index;not null;default:'Detecting'" json:"state"`
	FirstSeen   time.Time `gorm:"not null" json:"firstSeen"`
	LastSeen    time.Time `gorm:"not null;index" json:"lastSeen"`
	Count       int       `gorm:"not null;default:1" json:"count"`
	Namespace   string    `gorm:"index;not null" json:"namespace"`
	OwnerKind   string    `gorm:"not null" json:"ownerKind"`
	OwnerName   string    `gorm:"index;not null" json:"ownerName"`
	AnomalyType string   `gorm:"index;not null" json:"anomalyType"`
	Message     string    `gorm:"type:text" json:"message"`
	PodNames    string    `gorm:"type:text" json:"podNames"` // JSON 数组序列化
	ClusterID   *string   `gorm:"index;type:varchar(36)" json:"clusterId,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (Incident) TableName() string {
	return "incidents"
}
