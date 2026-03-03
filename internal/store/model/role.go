package model

import "time"

// Role 角色数据库模型
type Role struct {
	ID          string    `gorm:"primaryKey;type:uuid" json:"id"`
	Name        string    `gorm:"uniqueIndex;not null;type:varchar(64)" json:"name"`
	Description string    `gorm:"type:varchar(255)" json:"description"`
	Permissions string    `gorm:"type:text;not null" json:"permissions"` // JSON 数组 ["cluster:write","user:manage"]
	BuiltIn     bool      `gorm:"not null;default:false" json:"builtIn"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (Role) TableName() string {
	return "roles"
}
