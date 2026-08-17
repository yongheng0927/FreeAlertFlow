package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yongheng0927/fenghuo/internal/service"
)

// StatsHandler 提供 /api/v1/stats/* 聚合统计接口
type StatsHandler struct {
	stats service.StatsStore
}

// NewStatsHandler 创建 StatsHandler
func NewStatsHandler(stats service.StatsStore) *StatsHandler {
	return &StatsHandler{stats: stats}
}

// Dashboard 处理 GET /api/v1/stats/dashboard（viewer 及以上）：
// 告警总量/今日/本周 + 今日投递失败数 + 今日未匹配路由告警数。
// 「今日」「本周」按服务器本地时区取自然日、自然周（周一起算），与
// 前端 dayjs 本地时间的口径一致
func (h *StatsHandler) Dashboard(c *gin.Context) {
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	weekStart := todayStart.AddDate(0, 0, -(weekday - 1))
	st, err := h.stats.Dashboard(c.Request.Context(), todayStart, weekStart)
	if err != nil {
		fail(c, http.StatusInternalServerError, "internal error")
		return
	}
	c.JSON(http.StatusOK, st)
}
