package service

import (
	"context"
	"log/slog"
	"time"
)

// retentionInterval 是保留期清理的默认执行间隔
const retentionInterval = time.Hour

// RetentionCleaner 周期清理超过保留期的告警及其投递记录
// （alert.retention_days 配置的实际执行者）
type RetentionCleaner struct {
	alerts    AlertStore
	retention time.Duration
	interval  time.Duration
	now       func() time.Time
}

// NewRetentionCleaner 创建 RetentionCleaner retention <= 0 表示关闭清理
func NewRetentionCleaner(alerts AlertStore, retention time.Duration) *RetentionCleaner {
	return &RetentionCleaner{
		alerts:    alerts,
		retention: retention,
		interval:  retentionInterval,
		now:       time.Now,
	}
}

// Run 阻塞运行直到 ctx 取消：启动后先等一个周期再首次清理，避免与
// 启动迁移/预热争抢资源
func (c *RetentionCleaner) Run(ctx context.Context) {
	if c.retention <= 0 {
		return
	}
	t := time.NewTimer(c.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := c.cleanOnce(ctx); err != nil {
				slog.Error("retention cleanup failed", "error", err)
			}
			t.Reset(c.interval)
		}
	}
}

// cleanOnce 删除 cutoff 之前的告警（deliveries 随 alerts 级联删除）
func (c *RetentionCleaner) cleanOnce(ctx context.Context) error {
	cutoff := c.now().Add(-c.retention)
	n, err := c.alerts.DeleteOlderThan(ctx, cutoff)
	if err != nil {
		return err
	}
	if n > 0 {
		slog.Info("retention cleanup done", "cutoff", cutoff, "alerts_deleted", n)
	}
	return nil
}
