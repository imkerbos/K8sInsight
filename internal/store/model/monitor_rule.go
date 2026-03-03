package model

import "time"

// MonitorRule 监控规则数据库模型（一对一关联集群）
type MonitorRule struct {
	ID              string    `gorm:"primaryKey;type:uuid" json:"id"`
	ClusterID       string    `gorm:"uniqueIndex;type:uuid;not null" json:"clusterId"`
	Name            string    `gorm:"not null" json:"name"`
	Description     string    `gorm:"type:text" json:"description,omitempty"`
	Enabled         bool      `gorm:"not null;default:true" json:"enabled"`
	WatchScope      string    `gorm:"default:'cluster'" json:"watchScope"`
	WatchNamespaces string    `gorm:"type:text" json:"watchNamespaces,omitempty"`
	LabelSelector   string    `gorm:"type:varchar(512)" json:"labelSelector,omitempty"`
	AnomalyTypes    string    `gorm:"type:text" json:"anomalyTypes,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func (MonitorRule) TableName() string {
	return "monitor_rules"
}
