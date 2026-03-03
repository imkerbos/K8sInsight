package model

import "time"

// SystemSetting 系统设置 key-value 模型
type SystemSetting struct {
	Key       string    `gorm:"primaryKey;type:varchar(64)" json:"key"`
	Value     string    `gorm:"type:text;not null;default:''" json:"value"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (SystemSetting) TableName() string {
	return "system_settings"
}
