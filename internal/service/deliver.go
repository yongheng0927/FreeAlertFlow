package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/yongheng0927/FreeAlertFlow/internal/model"
	"github.com/yongheng0927/FreeAlertFlow/internal/pkg/metrics"
	"github.com/yongheng0927/FreeAlertFlow/internal/pkg/render"
)

// retryBackoffs 是各次重试前的等待时长（FR-2.4：1s、3s）
var retryBackoffs = []time.Duration{time.Second, 3 * time.Second}

// ErrChannelDeleted 表示对已删除渠道的手动重发（FR-2.6）
var ErrChannelDeleted = errors.New("channel has been deleted, resend refused")

// Deliverer 把一条告警投递到一个渠道：解析模板、渲染、有界原地重试发送，
// 并写入一条 deliveries 记录
type Deliverer struct {
	templates  TemplateStore
	deliveries DeliveryStore
	alerts     AlertStore
	channels   ChannelStore
	sources    SourceStore
	sender     Sender
	engine     *render.Engine
	retryMax   int
	backoffs   []time.Duration // 可注入，便于测试
	rootURL    string
	now        func() time.Time
}

// NewDeliverer 创建 Deliverer retryMax 为 0 时关闭重试（FR-2.4）
func NewDeliverer(templates TemplateStore, deliveries DeliveryStore,
	alerts AlertStore, channels ChannelStore, sources SourceStore, sender Sender,
	engine *render.Engine, retryMax int, rootURL string) *Deliverer {
	return &Deliverer{
		templates:  templates,
		deliveries: deliveries,
		alerts:     alerts,
		channels:   channels,
		sources:    sources,
		sender:     sender,
		engine:     engine,
		retryMax:   retryMax,
		backoffs:   retryBackoffs,
		rootURL:    rootURL,
		now:        time.Now,
	}
}

// Deliver 用渠道绑定的模板渲染告警并发送，瞬时错误原地重试 只写入一条
// trigger_type='auto' 的 deliveries 记录
func (d *Deliverer) Deliver(ctx context.Context, alert *model.Alert, msg *AMWebhook,
	ch *model.Channel, ruleID int64, sourceName string) {
	d.deliver(ctx, alert, msg, ch, ruleID, sourceName, "auto")
}

// Resend 用当前的渠道配置和模板重新投递一条失败记录（FR-2.6，原配置可能
// 正是失败原因，重发应使用修复后的配置）：原记录保持不动，新写一条
// trigger_type='manual' 的记录，沿用原 alert/channel/rule
func (d *Deliverer) Resend(ctx context.Context, deliveryID int64) (*model.Delivery, error) {
	orig, err := d.deliveries.FindByID(ctx, deliveryID)
	if err != nil {
		return nil, err
	}
	if orig == nil {
		return nil, fmt.Errorf("%w: delivery %d", ErrNotFound, deliveryID)
	}
	alert, err := d.alerts.FindByID(ctx, orig.AlertID)
	if err != nil {
		return nil, err
	}
	if alert == nil {
		return nil, fmt.Errorf("%w: alert %d no longer exists (retention cleanup)",
			ErrReferenced, orig.AlertID)
	}
	ch, err := d.channels.FindByID(ctx, orig.ChannelID)
	if err != nil {
		return nil, err
	}
	if ch == nil {
		return nil, ErrChannelDeleted
	}
	msg, err := ParseAMWebhook(alert.RawPayload)
	if err != nil {
		return nil, err
	}
	sourceName := ""
	if src, err := d.sources.FindByID(ctx, alert.SourceID); err != nil {
		return nil, err
	} else if src != nil {
		sourceName = src.Name
	}
	row := d.deliver(ctx, alert, msg, ch, orig.RuleID, sourceName, "manual")
	return row, nil
}

// deliver 是自动分发和手动重发共用的发送流水线
func (d *Deliverer) deliver(ctx context.Context, alert *model.Alert, msg *AMWebhook,
	ch *model.Channel, ruleID int64, sourceName, triggerType string) *model.Delivery {
	start := d.now()

	tmpl, err := d.resolveTemplate(ctx, msg, ch)
	if err != nil {
		return d.record(alert, ch, ruleID, triggerType, 1, start,
			SendResult{Code: -1, Msg: err.Error()}, nil)
	}

	rctx := buildRenderContext(msg, alert, sourceName, d.rootURL)
	payload, err := d.engine.Render(tmpl.Content, rctx)
	if err != nil {
		return d.record(alert, ch, ruleID, triggerType, 1, start,
			SendResult{Code: -1, Msg: "render error: " + err.Error()}, nil)
	}

	if ch.Keyword != "" && !strings.Contains(payload, ch.Keyword) {
		return d.record(alert, ch, ruleID, triggerType, 1, start, SendResult{
			Code: -1,
			Msg:  fmt.Sprintf("rendered message does not contain the required keyword %q", ch.Keyword),
		}, &payload)
	}

	// 有界原地重试（FR-2.4）：首次尝试 + 最多 retryMax 次重试，仅针对
	// 瞬时错误，明确的业务错误不重试
	var res SendResult
	attempts := 0
	for {
		attempts++
		res = d.sender.Send(ctx, ch, []byte(payload))
		if res.Success() || !res.Retryable() || attempts > d.retryMax {
			break
		}
		time.Sleep(d.backoff(attempts))
	}
	return d.record(alert, ch, ruleID, triggerType, attempts, start, res, &payload)
}

// backoff 返回第 n 次尝试之后、下一次尝试之前的等待时长
func (d *Deliverer) backoff(attempts int) time.Duration {
	i := attempts - 1
	if i >= len(d.backoffs) {
		i = len(d.backoffs) - 1
	}
	return d.backoffs[i]
}

// resolveTemplate 选取渠道绑定的模板，未绑定时用该渠道类型的内置默认
// 模板（FR-2.1/FR-2.3）
func (d *Deliverer) resolveTemplate(ctx context.Context, msg *AMWebhook, ch *model.Channel) (*model.Template, error) {
	if ch.TemplateID != nil {
		tmpl, err := d.templates.FindByID(ctx, *ch.TemplateID)
		if err != nil {
			return nil, err
		}
		if tmpl == nil {
			return nil, fmt.Errorf("bound template %d not found", *ch.TemplateID)
		}
		return tmpl, nil
	}
	name := render.DefaultBuiltinName(msg.Status, msg.CommonLabels["severity"])
	tmpl, err := d.templates.FindBuiltin(ctx, ch.Type, name)
	if err != nil {
		return nil, err
	}
	if tmpl == nil {
		return nil, fmt.Errorf("builtin template %s/%s not found", ch.Type, name)
	}
	return tmpl, nil
}

// record 写入一条带最终结果的 deliveries 记录
func (d *Deliverer) record(alert *model.Alert, ch *model.Channel, ruleID int64, triggerType string,
	attempts int, start time.Time, res SendResult, payload *string) *model.Delivery {
	status := "success"
	if !res.Success() {
		status = "failed"
	}
	row := &model.Delivery{
		AlertID:         alert.ID,
		ChannelID:       ch.ID,
		ChannelName:     ch.Name, // 渠道名快照，渠道被删后记录仍可读
		RuleID:          ruleID,
		TriggerType:     triggerType,
		Attempts:        attempts,
		Status:          status,
		HTTPStatus:      res.HTTPStatus,
		ResponseCode:    res.Code,
		ResponseMsg:     truncateRunes(res.Message(), 512),
		DurationMS:      int(d.now().Sub(start).Milliseconds()),
		RenderedPayload: payload,
		SentAt:          d.now(),
	}
	if err := d.deliveries.Create(context.Background(), row); err != nil {
		slog.Error("record delivery failed", "alert_id", alert.ID, "channel_id", ch.ID, "error", err)
	}
	metrics.ObserveDelivery(ch.Name, status, d.now().Sub(start).Seconds())
	if triggerType == "manual" {
		metrics.IncManualResend(status)
	}
	return row
}

// buildRenderContext 用入库的分组负载和当前告警行构建模板上下文（FR-2.3）
func buildRenderContext(msg *AMWebhook, alert *model.Alert, sourceName, rootURL string) *render.Context {
	rctx := &render.Context{
		Version:           msg.Version,
		Status:            msg.Status,
		Receiver:          msg.Receiver,
		GroupKey:          msg.GroupKey,
		ExternalURL:       msg.ExternalURL,
		GroupLabels:       msg.GroupLabels,
		CommonLabels:      msg.CommonLabels,
		CommonAnnotations: msg.CommonAnnotations,
		SourceName:        sourceName,
		RootURL:           rootURL,
		DetailURL:         strings.TrimRight(rootURL, "/") + "/alerts/" + fmt.Sprint(alert.ID),
	}
	for _, a := range msg.Alerts {
		rctx.Alerts = append(rctx.Alerts, render.Alert{
			Status:       a.Status,
			Labels:       a.Labels,
			Annotations:  a.Annotations,
			StartsAt:     a.StartsAt,
			EndsAt:       a.EndsAt,
			GeneratorURL: a.GeneratorURL,
			Fingerprint:  a.Fingerprint,
		})
	}
	return rctx
}
