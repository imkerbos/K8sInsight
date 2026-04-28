package retention

import (
	"context"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Config 数据保留配置
type Config struct {
	ArchiveAfter time.Duration // Resolved 多久后归档（默认 30 天）
	DeleteAfter  time.Duration // Archived 多久后删除证据（默认 90 天）
	Interval     time.Duration // 执行间隔（默认 1 小时）
}

// DefaultConfig 返回默认保留配置
func DefaultConfig() Config {
	return Config{
		ArchiveAfter: 30 * 24 * time.Hour,
		DeleteAfter:  90 * 24 * time.Hour,
		Interval:     1 * time.Hour,
	}
}

// Job 数据保留后台任务
type Job struct {
	db     *gorm.DB
	cfg    Config
	logger *zap.Logger
}

// NewJob 创建数据保留任务
func NewJob(db *gorm.DB, cfg Config, logger *zap.Logger) *Job {
	return &Job{
		db:     db,
		cfg:    cfg,
		logger: logger.Named("retention"),
	}
}

// Start 启动后台保留任务
func (j *Job) Start(ctx context.Context) {
	go j.run(ctx)
}

func (j *Job) run(ctx context.Context) {
	ticker := time.NewTicker(j.cfg.Interval)
	defer ticker.Stop()

	// 启动时立即执行一次
	j.execute(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			j.execute(ctx)
		}
	}
}

func (j *Job) execute(ctx context.Context) {
	j.logger.Info("开始执行数据保留清理")

	// Resolved 超过 ArchiveAfter → Archived
	archiveCutoff := time.Now().Add(-j.cfg.ArchiveAfter)
	result := j.db.WithContext(ctx).
		Table("incidents").
		Where("state = ? AND last_seen < ?", "Resolved", archiveCutoff).
		Update("state", "Archived")
	if result.Error != nil {
		j.logger.Error("归档事件失败", zap.Error(result.Error))
	} else if result.RowsAffected > 0 {
		j.logger.Info("事件已归档", zap.Int64("count", result.RowsAffected))
	}

	// Archived 超过 DeleteAfter → 删除证据
	deleteCutoff := time.Now().Add(-j.cfg.DeleteAfter)
	result = j.db.WithContext(ctx).
		Table("evidences").
		Where("incident_id IN (SELECT id FROM incidents WHERE state = ? AND last_seen < ?)", "Archived", deleteCutoff).
		Delete(nil)
	if result.Error != nil {
		j.logger.Error("清理过期证据失败", zap.Error(result.Error))
	} else if result.RowsAffected > 0 {
		j.logger.Info("过期证据已清理", zap.Int64("count", result.RowsAffected))
	}
}
