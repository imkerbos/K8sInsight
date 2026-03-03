package model

import "time"

// User 用户数据库模型
type User struct {
	ID           string    `gorm:"primaryKey;type:uuid" json:"id"`
	Username     string    `gorm:"uniqueIndex;not null;type:varchar(64)" json:"username"`
	PasswordHash string    `gorm:"not null;type:varchar(255)" json:"-"`
	Role         string    `gorm:"not null;default:'admin';type:varchar(32)" json:"role"`
	IsActive     bool      `gorm:"not null;default:true" json:"isActive"`
	AuthSource   string    `gorm:"not null;default:'local';type:varchar(16)" json:"authSource"`
	SSOProvider  string    `gorm:"not null;default:'';type:varchar(64)" json:"ssoProvider,omitempty"`
	SSOSubject   string    `gorm:"not null;default:'';type:varchar(255)" json:"ssoSubject,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func (User) TableName() string {
	return "users"
}

// RefreshToken 刷新令牌数据库模型
type RefreshToken struct {
	ID        string    `gorm:"primaryKey;type:uuid" json:"id"`
	UserID    string    `gorm:"index;not null;type:uuid" json:"userId"`
	TokenHash string    `gorm:"uniqueIndex;not null;type:varchar(64)" json:"-"` // SHA-256 hex
	ExpiresAt time.Time `gorm:"not null;index" json:"expiresAt"`
	Revoked   bool      `gorm:"not null;default:false" json:"revoked"`
	CreatedAt time.Time `json:"createdAt"`
}

func (RefreshToken) TableName() string {
	return "refresh_tokens"
}
