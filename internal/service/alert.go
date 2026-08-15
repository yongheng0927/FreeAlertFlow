package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/yongheng0927/fenghuo/internal/model"
	"github.com/yongheng0927/fenghuo/internal/pkg/metrics"
)

// webhook 接入的哨兵错误，映射为 HTTP 状态码
var (
	ErrSourceNotFound = errors.New("source not found")
	ErrSourceDisabled = errors.New("source disabled")
)

// AlertService 接入 Alertmanager webhook（同步部分：解析 + 入库），投递
// 分发异步执行（NFR-2：约 200ms 内响应，路由和发送在后台 goroutine 中进行）
type AlertService struct {
	sources     SourceStore
	alerts      AlertStore
	rules       RuleStore
	channels    ChannelStore
	deliverer   *Deliverer
	dedupWindow time.Duration
	// async 控制分发是否在 goroutine 中执行，测试置为 false 以同步驱动
	// Dispatch
	async bool
	now   func() time.Time
}

// NewAlertService 创建 AlertService dedupWindow 为 0 时关闭去重
func NewAlertService(sources SourceStore, alerts AlertStore, rules RuleStore,
	channels ChannelStore, deliverer *Deliverer, dedupWindow time.Duration) *AlertService {
	return &AlertService{
		sources:     sources,
		alerts:      alerts,
		rules:       rules,
		channels:    channels,
		deliverer:   deliverer,
		dedupWindow: dedupWindow,
		async:       true,
		now:         time.Now,
	}
}

// Ingest 校验接入源 token，解析 v4 负载并为每条告警存一行
// （FR-1.1/1.2），然后启动异步分发 返回接入源和入库告警条数
func (s *AlertService) Ingest(ctx context.Context, token string, body []byte) (*model.Source, int, error) {
	src, err := s.sources.FindByToken(ctx, token)
	if err != nil {
		return nil, 0, err
	}
	if src == nil {
		return nil, 0, ErrSourceNotFound
	}
	if !src.Enabled {
		return nil, 0, ErrSourceDisabled
	}

	msg, err := ParseAMWebhook(body)
	if err != nil {
		return nil, 0, err
	}

	now := s.now()
	raw := json.RawMessage(body)
	ids := make([]int64, 0, len(msg.Alerts))
	for _, a := range msg.Alerts {
		fp := a.Fingerprint
		if fp == "" {
			fp = FingerprintFromLabels(a.Labels)
		}
		var endsAt *time.Time
		if !a.EndsAt.IsZero() {
			t := a.EndsAt
			endsAt = &t
		}
		labels, _ := json.Marshal(a.Labels)
		annotations, _ := json.Marshal(a.Annotations)
		row := &model.Alert{
			SourceID:    src.ID,
			Fingerprint: fp,
			ContentHash: ContentHash(a.Status, a.Labels, a.Annotations),
			Status:      a.Status,
			Alertname:   a.Labels["alertname"],
			Severity:    a.Labels["severity"],
			Labels:      labels,
			Annotations: annotations,
			StartsAt:    a.StartsAt,
			EndsAt:      endsAt,
			RawPayload:  raw,
			Disposition: "delivered",
			ReceivedAt:  now,
		}
		if err := s.alerts.Create(ctx, row); err != nil {
			return nil, 0, fmt.Errorf("store alert: %w", err)
		}
		ids = append(ids, row.ID)
	}
	if err := s.sources.UpdateLastAlertAt(ctx, src.ID, now); err != nil {
		slog.Warn("update last_alert_at failed", "source_id", src.ID, "error", err)
	}
	metrics.ObserveAlertsReceived(src.Name, len(ids))

	if s.async {
		go s.dispatchAll(ids, src.Name)
	} else {
		s.dispatchAll(ids, src.Name)
	}
	return src, len(ids), nil
}

// dispatchAll 在后台逐条处理已入库的告警
func (s *AlertService) dispatchAll(ids []int64, sourceName string) {
	for _, id := range ids {
		if err := s.Dispatch(context.Background(), id, sourceName); err != nil {
			slog.Error("dispatch alert failed", "alert_id", id, "error", err)
		}
	}
}

// Dispatch 执行单条告警的处理流水线：去重检查（FR-1.3）、路由匹配
// （FR-3.2）、多渠道并行投递（NFR-2）、更新 disposition
func (s *AlertService) Dispatch(ctx context.Context, alertID int64, sourceName string) error {
	alert, err := s.alerts.FindByID(ctx, alertID)
	if err != nil {
		return err
	}
	if alert == nil {
		return fmt.Errorf("alert %d not found", alertID)
	}

	// 去重：窗口内 (fingerprint, status) 相同且 content_hash 未变的告警
	// 只入库不发送（FR-1.3）
	if s.dedupWindow > 0 {
		prev, err := s.alerts.FindLatestInWindow(ctx, alert.Fingerprint, alert.Status,
			s.now().Add(-s.dedupWindow), alert.ID)
		if err != nil {
			return err
		}
		if prev != nil && prev.ContentHash == alert.ContentHash {
			return s.setDisposition(ctx, alert.ID, "deduped")
		}
	}

	msg, err := ParseAMWebhook(alert.RawPayload)
	if err != nil {
		return err
	}

	rules, err := s.rules.ListEnabledBySource(ctx, alert.SourceID)
	if err != nil {
		return err
	}
	var labels map[string]string
	if err := json.Unmarshal(alert.Labels, &labels); err != nil {
		return fmt.Errorf("decode alert labels: %w", err)
	}
	matched := MatchRules(rules, labels)
	if len(matched) == 0 {
		return s.setDisposition(ctx, alert.ID, "unmatched")
	}

	// 解析目标渠道，跳过已禁用或已删除的
	type target struct {
		rule model.RoutingRule
		ch   *model.Channel
	}
	var targets []target
	for _, r := range matched {
		ch, err := s.channels.FindByID(ctx, r.ChannelID)
		if err != nil {
			return err
		}
		if ch == nil || !ch.Enabled {
			slog.Warn("route target channel unavailable, skipped",
				"rule_id", r.ID, "channel_id", r.ChannelID)
			continue
		}
		targets = append(targets, target{rule: r, ch: ch})
	}
	if len(targets) == 0 {
		// 规则命中了但没有可投递的渠道（渠道已禁用/删除）
		return s.setDisposition(ctx, alert.ID, "unmatched")
	}

	// 并行投递到所有绑定渠道（NFR-2）
	var wg sync.WaitGroup
	for _, t := range targets {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.deliverer.Deliver(ctx, alert, msg, t.ch, t.rule.ID, sourceName)
		}()
	}
	wg.Wait()
	return s.setDisposition(ctx, alert.ID, "delivered")
}

// setDisposition 更新告警 disposition 并记录指标（NFR-3）
func (s *AlertService) setDisposition(ctx context.Context, id int64, disposition string) error {
	metrics.IncDisposition(disposition)
	return s.alerts.UpdateDisposition(ctx, id, disposition)
}
