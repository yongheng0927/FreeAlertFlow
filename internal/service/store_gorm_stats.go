package service

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/yongheng0927/fenghuo/internal/model"
)

// GormStatsStore 用 GORM/PostgreSQL 实现 StatsStore；几条 COUNT 聚合，
// 数据量下无需物化或缓存
type GormStatsStore struct{ db *gorm.DB }

// NewGormStatsStore 创建 GormStatsStore
func NewGormStatsStore(db *gorm.DB) *GormStatsStore { return &GormStatsStore{db: db} }

func (s *GormStatsStore) Dashboard(ctx context.Context, todayStart, weekStart time.Time) (*DashboardStats, error) {
	var st DashboardStats
	count := func(q *gorm.DB) (int64, error) {
		var n int64
		err := q.Count(&n).Error
		return n, err
	}
	var err error
	if st.AlertsTotal, err = count(s.db.WithContext(ctx).Model(&model.Alert{})); err != nil {
		return nil, err
	}
	if st.AlertsToday, err = count(s.db.WithContext(ctx).Model(&model.Alert{}).
		Where("received_at >= ?", todayStart)); err != nil {
		return nil, err
	}
	if st.AlertsWeek, err = count(s.db.WithContext(ctx).Model(&model.Alert{}).
		Where("received_at >= ?", weekStart)); err != nil {
		return nil, err
	}
	if st.FailedDeliveriesToday, err = count(s.db.WithContext(ctx).Model(&model.Delivery{}).
		Where("status = 'failed' AND sent_at >= ?", todayStart)); err != nil {
		return nil, err
	}
	if st.UnmatchedAlertsToday, err = count(s.db.WithContext(ctx).Model(&model.Alert{}).
		Where("disposition = 'unmatched' AND received_at >= ?", todayStart)); err != nil {
		return nil, err
	}
	return &st, nil
}
