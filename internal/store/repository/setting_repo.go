package repository

import "context"

// SettingRepository 系统设置数据访问接口
type SettingRepository interface {
	Get(ctx context.Context, key string) (string, error)
	BatchGet(ctx context.Context, keys []string) (map[string]string, error)
	Set(ctx context.Context, key, value string) error
	BatchSet(ctx context.Context, settings map[string]string) error
}
