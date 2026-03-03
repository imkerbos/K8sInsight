package model

import "time"

// Cluster 集群配置数据库模型
type Cluster struct {
	ID               string    `gorm:"primaryKey;type:uuid" json:"id"`
	Name             string    `gorm:"uniqueIndex;not null" json:"name"`
	KubeconfigData   string    `gorm:"type:text;not null" json:"-"` // API 响应中隐藏
	Status           string    `gorm:"not null;default:'active'" json:"status"`              // active/inactive
	ConnectionStatus string    `gorm:"not null;default:'unknown'" json:"connectionStatus"`   // unknown/connected/failed
	StatusMessage    string    `gorm:"type:text" json:"statusMessage,omitempty"`
	Version          string    `gorm:"type:varchar(32)" json:"version,omitempty"`
	NodeCount        int       `gorm:"not null;default:0" json:"nodeCount"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

func (Cluster) TableName() string {
	return "clusters"
}
