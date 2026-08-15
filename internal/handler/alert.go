package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yongheng0927/fenghuo/internal/model"
	"github.com/yongheng0927/fenghuo/internal/service"
)

// AlertHandler 提供 /api/v1/alerts 告警记录查询接口（FR-4）
type AlertHandler struct {
	alerts     service.AlertStore
	deliveries service.DeliveryStore
}

// NewAlertHandler 创建 AlertHandler
func NewAlertHandler(alerts service.AlertStore, deliveries service.DeliveryStore) *AlertHandler {
	return &AlertHandler{alerts: alerts, deliveries: deliveries}
}

type alertView struct {
	ID          int64           `json:"id"`
	SourceID    int64           `json:"source_id"`
	Fingerprint string          `json:"fingerprint"`
	Status      string          `json:"status"`
	Alertname   string          `json:"alertname"`
	Severity    string          `json:"severity"`
	Labels      json.RawMessage `json:"labels"`
	Annotations json.RawMessage `json:"annotations"`
	StartsAt    time.Time       `json:"starts_at"`
	EndsAt      *time.Time      `json:"ends_at"`
	Disposition string          `json:"disposition"`
	ReceivedAt  time.Time       `json:"received_at"`
}

func toAlertView(a *model.Alert) alertView {
	return alertView{
		ID: a.ID, SourceID: a.SourceID, Fingerprint: a.Fingerprint,
		Status: a.Status, Alertname: a.Alertname, Severity: a.Severity,
		Labels: a.Labels, Annotations: a.Annotations,
		StartsAt: a.StartsAt, EndsAt: a.EndsAt,
		Disposition: a.Disposition, ReceivedAt: a.ReceivedAt,
	}
}

// List 处理 GET /api/v1/alerts，支持筛选和分页（FR-4.1）
func (h *AlertHandler) List(c *gin.Context) {
	channelID, ok := parseInt64Query(c, "channel_id")
	if !ok {
		return
	}
	start, ok := parseTimeParam(c, "start")
	if !ok {
		return
	}
	end, ok := parseTimeParam(c, "end")
	if !ok {
		return
	}
	offset, limit, page, size := pageParams(c)
	alerts, total, err := h.alerts.List(c.Request.Context(), service.AlertFilter{
		Status:    c.Query("status"),
		Severity:  c.Query("severity"),
		Alertname: c.Query("alertname"),
		ChannelID: channelID,
		Start:     start,
		End:       end,
		Offset:    offset,
		Limit:     limit,
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, "internal error")
		return
	}
	views := make([]alertView, 0, len(alerts))
	for i := range alerts {
		views = append(views, toAlertView(&alerts[i]))
	}
	listJSON(c, views, total, page, size)
}

// Get 处理 GET /api/v1/alerts/:id，响应中包含投递记录（FR-4.2）
func (h *AlertHandler) Get(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	alert, err := h.alerts.FindByID(c.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusInternalServerError, "internal error")
		return
	}
	if alert == nil {
		fail(c, http.StatusNotFound, "alert not found")
		return
	}
	deliveries, err := h.deliveries.ListByAlertID(c.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusInternalServerError, "internal error")
		return
	}
	views := make([]deliveryView, 0, len(deliveries))
	for i := range deliveries {
		views = append(views, toDeliveryView(&deliveries[i]))
	}
	c.JSON(http.StatusOK, gin.H{
		"alert": gin.H{
			"id":           alert.ID,
			"source_id":    alert.SourceID,
			"fingerprint":  alert.Fingerprint,
			"content_hash": alert.ContentHash,
			"status":       alert.Status,
			"alertname":    alert.Alertname,
			"severity":     alert.Severity,
			"labels":       alert.Labels,
			"annotations":  alert.Annotations,
			"starts_at":    alert.StartsAt,
			"ends_at":      alert.EndsAt,
			"disposition":  alert.Disposition,
			"received_at":  alert.ReceivedAt,
			"raw_payload":  alert.RawPayload,
		},
		"deliveries": views,
	})
}
