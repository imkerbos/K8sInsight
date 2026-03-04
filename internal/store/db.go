package store

import (
	"fmt"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/kerbos/k8sinsight/internal/config"
	"github.com/kerbos/k8sinsight/internal/store/model"
)

// NewDB 创建数据库连接并自动迁移
func NewDB(cfg config.DBConfig, logger *zap.Logger) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取底层 SQL DB 失败: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.MaxConns)
	sqlDB.SetMaxIdleConns(cfg.MinConns)

	// 自动迁移表结构
	if err := db.AutoMigrate(
		&model.Incident{},
		&model.Evidence{},
		&model.TimelineEntry{},
		&model.Cluster{},
		&model.MonitorRule{},
		&model.User{},
		&model.RefreshToken{},
		&model.SystemSetting{},
		&model.Role{},
		&model.SSOState{},
	); err != nil {
		return nil, fmt.Errorf("数据库迁移失败: %w", err)
	}

	logger.Info("数据库连接成功",
		zap.String("host", cfg.Host),
		zap.Int("port", cfg.Port),
		zap.String("dbname", cfg.DBName),
	)

	return db, nil
}
