package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/kerbos/k8sinsight/internal/store/model"
)

// SettingRepository 系统设置数据访问接口
type SettingRepository interface {
	Get(ctx context.Context, key string) (string, error)
	BatchGet(ctx context.Context, keys []string) (map[string]string, error)
	Set(ctx context.Context, key, value string) error
	BatchSet(ctx context.Context, settings map[string]string) error
}

type settingRepo struct {
	db *gorm.DB
}

func NewSettingRepository(db *gorm.DB) SettingRepository {
	return &settingRepo{db: db}
}

func (r *settingRepo) Get(ctx context.Context, key string) (string, error) {
	var s model.SystemSetting
	err := r.db.WithContext(ctx).Where("key = ?", key).First(&s).Error
	if err != nil {
		return "", err
	}
	return s.Value, nil
}

func (r *settingRepo) BatchGet(ctx context.Context, keys []string) (map[string]string, error) {
	var settings []model.SystemSetting
	err := r.db.WithContext(ctx).Where("key IN ?", keys).Find(&settings).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(keys))
	for _, k := range keys {
		result[k] = ""
	}
	for _, s := range settings {
		result[s.Key] = s.Value
	}
	return result, nil
}

func (r *settingRepo) Set(ctx context.Context, key, value string) error {
	s := model.SystemSetting{Key: key, Value: value}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&s).Error
}

func (r *settingRepo) BatchSet(ctx context.Context, settings map[string]string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for k, v := range settings {
			s := model.SystemSetting{Key: k, Value: v}
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "key"}},
				DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
			}).Create(&s).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
