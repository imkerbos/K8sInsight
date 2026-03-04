package model

import "time"

// SSOState 数据库存储的 SSO state/nonce 条目
type SSOState struct {
	State     string    `gorm:"primaryKey;size:64"`
	Nonce     string    `gorm:"size:64;not null"`
	ExpiresAt time.Time `gorm:"not null;index"`
}
