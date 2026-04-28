package repository

import (
	"context"

	"github.com/kerbos/k8sinsight/internal/store/model"
)

// UserRepository 用户数据访问接口
type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	FindByID(ctx context.Context, id string) (*model.User, error)
	FindByUsername(ctx context.Context, username string) (*model.User, error)
	FindBySSOSubject(ctx context.Context, provider, subject string) (*model.User, error)
	Update(ctx context.Context, user *model.User) error
	Count(ctx context.Context) (int64, error)
	List(ctx context.Context) ([]model.User, error)

	// RefreshToken 操作
	CreateRefreshToken(ctx context.Context, token *model.RefreshToken) error
	FindRefreshTokenByHash(ctx context.Context, hash string) (*model.RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, id string) error
	RevokeAllUserRefreshTokens(ctx context.Context, userID string) error
	DeleteExpiredRefreshTokens(ctx context.Context) error
}
