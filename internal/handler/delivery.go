package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yongheng0927/fenghuo/internal/model"
	"github.com/yongheng0927/fenghuo/internal/service"
)

// DeliveryHandler 提供 /api/v1/deliveries 投递记录接口（editor/admin，FR-2.6）
type DeliveryHandler struct {
	deliveries service.DeliveryStore
	deliverer  *service.Deliverer
}

// NewDeliveryHandler 创建 DeliveryHandler
func NewDeliveryHandler(deliveries service.DeliveryStore, deliverer *service.Deliverer) *DeliveryHandler {
	return &DeliveryHandler{deliveries: deliveries, deliverer: deliverer}
}

type deliveryView struct {
	ID              int64     `json:"id"`
	AlertID         int64     `json:"alert_id"`
	ChannelID       int64     `json:"channel_id"`
	ChannelName     string    `json:"channel_name"`
	RuleID          int64     `json:"rule_id"`
	TriggerType     string    `json:"trigger_type"`
	Attempts        int       `json:"attempts"`
	Status          string    `json:"status"`
	HTTPStatus      int       `json:"http_status"`
	ResponseCode    int       `json:"response_code"`
	ResponseMsg     string    `json:"response_msg"`
	DurationMS      int       `json:"duration_ms"`
	RenderedPayload *string   `json:"rendered_payload"`
	SentAt          time.Time `json:"sent_at"`
}

func toDeliveryView(d *model.Delivery) deliveryView {
	return deliveryView{
		ID: d.ID, AlertID: d.AlertID, ChannelID: d.ChannelID, ChannelName: d.ChannelName,
		RuleID: d.RuleID, TriggerType: d.TriggerType, Attempts: d.Attempts,
		Status: d.Status, HTTPStatus: d.HTTPStatus, ResponseCode: d.ResponseCode,
		ResponseMsg: d.ResponseMsg, DurationMS: d.DurationMS,
		RenderedPayload: d.RenderedPayload, SentAt: d.SentAt,
	}
}

// List 处理 GET /api/v1/deliveries，支持筛选和分页
func (h *DeliveryHandler) List(c *gin.Context) {
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
	deliveries, total, err := h.deliveries.List(c.Request.Context(), service.DeliveryFilter{
		Status:    c.Query("status"),
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
	views := make([]deliveryView, 0, len(deliveries))
	for i := range deliveries {
		views = append(views, toDeliveryView(&deliveries[i]))
	}
	listJSON(c, views, total, page, size)
}

// Resend 处理 POST /api/v1/deliveries/:id/resend（FR-2.6）：用当前的渠道
// 配置和模板重新渲染发送，并插入一条 trigger_type='manual' 的新记录，原
// 失败记录保持不动，保留完整排查痕迹
func (h *DeliveryHandler) Resend(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	row, err := h.deliverer.Resend(c.Request.Context(), id)
	if err != nil {
		serviceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success":  row.Status == "success",
		"delivery": toDeliveryView(row),
	})
}
