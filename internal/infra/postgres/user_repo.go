package postgres

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/kerbos/k8sinsight/internal/store/model"
	"github.com/kerbos/k8sinsight/internal/store/repository"
)

type userRepo struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) repository.UserRepository {
	return &userRepo{db: db}
}

func (r *userRepo) Create(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *userRepo) FindByID(ctx context.Context, id string) (*model.User, error) {
	var u model.User
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&u).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userRepo) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	var u model.User
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&u).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userRepo) FindBySSOSubject(ctx context.Context, provider, subject string) (*model.User, error) {
	var u model.User
	err := r.db.WithContext(ctx).Where("sso_provider = ? AND sso_subject = ?", provider, subject).First(&u).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userRepo) Update(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

func (r *userRepo) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.User{}).Count(&count).Error
	return count, err
}

func (r *userRepo) List(ctx context.Context) ([]model.User, error) {
	var users []model.User
	err := r.db.WithContext(ctx).Order("created_at ASC").Find(&users).Error
	return users, err
}

func (r *userRepo) CreateRefreshToken(ctx context.Context, token *model.RefreshToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

func (r *userRepo) FindRefreshTokenByHash(ctx context.Context, hash string) (*model.RefreshToken, error) {
	var t model.RefreshToken
	err := r.db.WithContext(ctx).
		Where("token_hash = ? AND revoked = ? AND expires_at > ?", hash, false, time.Now()).
		First(&t).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *userRepo) RevokeRefreshToken(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).
		Model(&model.RefreshToken{}).
		Where("id = ?", id).
		Update("revoked", true).Error
}

func (r *userRepo) RevokeAllUserRefreshTokens(ctx context.Context, userID string) error {
	return r.db.WithContext(ctx).
		Model(&model.RefreshToken{}).
		Where("user_id = ? AND revoked = ?", userID, false).
		Update("revoked", true).Error
}

func (r *userRepo) DeleteExpiredRefreshTokens(ctx context.Context) error {
	return r.db.WithContext(ctx).
		Where("expires_at < ? OR revoked = ?", time.Now(), true).
		Delete(&model.RefreshToken{}).Error
}
