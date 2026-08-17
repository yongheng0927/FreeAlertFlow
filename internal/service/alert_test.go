package service

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/yongheng0927/fenghuo/internal/model"
	"github.com/yongheng0927/fenghuo/internal/pkg/render"
)

// --- 接入/分发用的内存 fake ---

type fakeSourceStore struct {
	byToken map[string]*model.Source
}

func (f *fakeSourceStore) FindByToken(_ context.Context, token string) (*model.Source, error) {
	return f.byToken[token], nil
}

func (f *fakeSourceStore) UpdateLastAlertAt(_ context.Context, id int64, t time.Time) error {
	for _, s := range f.byToken {
		if s.ID == id {
			s.LastAlertAt = &t
		}
	}
	return nil
}

type fakeAlertStore struct {
	mu     sync.Mutex
	byID   map[int64]*model.Alert
	nextID int64
}

func newFakeAlertStore() *fakeAlertStore {
	return &fakeAlertStore{byID: map[int64]*model.Alert{}, nextID: 1}
}

func (f *fakeAlertStore) Create(_ context.Context, a *model.Alert) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	a.ID = f.nextID
	f.nextID++
	f.byID[a.ID] = a
	return nil
}

// CreateWithDedupCheck 用互斥锁模拟生产实现的咨询锁语义（FR-1.3）
func (f *fakeAlertStore) CreateWithDedupCheck(_ context.Context, a *model.Alert, window time.Duration) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	deduped := false
	if window > 0 {
		since := time.Now().Add(-window)
		var latest *model.Alert
		for _, x := range f.byID {
			if x.Fingerprint != a.Fingerprint || x.Status != a.Status || x.ReceivedAt.Before(since) {
				continue
			}
			if latest == nil || x.ReceivedAt.After(latest.ReceivedAt) {
				latest = x
			}
		}
		deduped = latest != nil && latest.ContentHash == a.ContentHash
	}
	if deduped {
		a.Disposition = "deduped"
	} else {
		a.Disposition = "pending"
	}
	a.ID = f.nextID
	f.nextID++
	f.byID[a.ID] = a
	return deduped, nil
}

func (f *fakeAlertStore) FindByID(_ context.Context, id int64) (*model.Alert, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.byID[id], nil
}

func (f *fakeAlertStore) UpdateDisposition(_ context.Context, id int64, disposition string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[id].Disposition = disposition
	return nil
}

// DeleteOlderThan 删除 cutoff 之前的告警（fake 不级联 fakeDeliveryStore，
// 清理器测试只断言告警侧）
func (f *fakeAlertStore) DeleteOlderThan(_ context.Context, cutoff time.Time) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var n int64
	for id, a := range f.byID {
		if a.ReceivedAt.Before(cutoff) {
			delete(f.byID, id)
			n++
		}
	}
	return n, nil
}

func (f *fakeAlertStore) FindLatestInWindow(_ context.Context, fingerprint, status string,
	since time.Time, excludeID int64) (*model.Alert, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var latest *model.Alert
	for _, a := range f.byID {
		if a.ID == excludeID || a.Fingerprint != fingerprint || a.Status != status {
			continue
		}
		if a.ReceivedAt.Before(since) {
			continue
		}
		if latest == nil || a.ReceivedAt.After(latest.ReceivedAt) {
			latest = a
		}
	}
	return latest, nil
}

type fakeRuleStore struct {
	rules []model.RoutingRule
	err   error
}

func (f *fakeRuleStore) ListEnabledBySource(_ context.Context, sourceID int64) ([]model.RoutingRule, error) {
	var out []model.RoutingRule
	for _, r := range f.rules {
		if r.SourceID == sourceID && r.Enabled {
			out = append(out, r)
		}
	}
	// store 契约要求按 priority 升序返回，fake 的数据由测试预先排好序
	return out, f.err
}

type fakeChannelStore struct {
	byID map[int64]*model.Channel
}

func (f *fakeChannelStore) FindByID(_ context.Context, id int64) (*model.Channel, error) {
	return f.byID[id], nil
}

// alertTestEnv 把 AlertService 和它的 fake 打包在一起
type alertTestEnv struct {
	svc        *AlertService
	sources    *fakeSourceStore
	alerts     *fakeAlertStore
	rules      *fakeRuleStore
	channels   *fakeChannelStore
	deliveries *fakeDeliveryStore
	sender     *fakeSender
}

func newAlertTestEnv(t *testing.T, dedupWindow time.Duration, sender *fakeSender) *alertTestEnv {
	t.Helper()
	deliveries := &fakeDeliveryStore{}
	sources := &fakeSourceStore{byToken: map[string]*model.Source{}}
	alerts := newFakeAlertStore()
	channels := &fakeChannelStore{byID: map[int64]*model.Channel{}}
	deliverer := NewDeliverer(newFakeTemplateStore(t), deliveries, alerts, channels, sources, sender,
		render.NewEngine(time.UTC), 2, "https://alerts.example.com/")
	deliverer.backoffs = []time.Duration{time.Millisecond, time.Millisecond}
	env := &alertTestEnv{
		sources:    sources,
		alerts:     alerts,
		rules:      &fakeRuleStore{},
		channels:   channels,
		deliveries: deliveries,
		sender:     sender,
	}
	env.svc = NewAlertService(env.sources, env.alerts, env.rules, env.channels, deliverer, dedupWindow)
	env.svc.async = false // 测试中同步驱动 dispatch
	return env
}

func (e *alertTestEnv) addSource(token string, enabled bool) *model.Source {
	src := &model.Source{ID: 1, Name: "生产环境 Prometheus", Token: token, Enabled: enabled}
	e.sources.byToken[token] = src
	return src
}

func TestIngestStoresAlertsAndUpdatesSource(t *testing.T) {
	env := newAlertTestEnv(t, 5*time.Minute, &fakeSender{results: []SendResult{okResult()}})
	src := env.addSource("tok123", true)

	got, n, err := env.svc.Ingest(context.Background(), "tok123", []byte(sampleWebhookJSON))
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if got.ID != src.ID || n != 1 {
		t.Fatalf("src=%v n=%d", got, n)
	}
	row := env.alerts.byID[1]
	if row == nil {
		t.Fatal("alert row must be stored")
	}
	if row.Fingerprint != "0123456789abcdef" {
		t.Errorf("fingerprint must come from Alertmanager: %q", row.Fingerprint)
	}
	if row.Alertname != "HighCPU" || row.Severity != "critical" {
		t.Errorf("redundant columns wrong: %+v", row)
	}
	if len(row.ContentHash) != 64 {
		t.Errorf("content_hash = %q", row.ContentHash)
	}
	if row.EndsAt != nil {
		t.Errorf("zero endsAt must be stored as NULL")
	}
	if len(row.RawPayload) == 0 {
		t.Error("raw_payload must store the whole group payload")
	}
	if src.LastAlertAt == nil {
		t.Error("last_alert_at must be updated")
	}
}

func TestIngestFingerprintFallback(t *testing.T) {
	env := newAlertTestEnv(t, 5*time.Minute, &fakeSender{results: []SendResult{okResult()}})
	env.addSource("tok123", true)

	// 从负载中去掉 fingerprint
	var msg map[string]any
	if err := json.Unmarshal([]byte(sampleWebhookJSON), &msg); err != nil {
		t.Fatal(err)
	}
	alerts := msg["alerts"].([]any)
	delete(alerts[0].(map[string]any), "fingerprint")
	body, _ := json.Marshal(msg)

	if _, _, err := env.svc.Ingest(context.Background(), "tok123", body); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	row := env.alerts.byID[1]
	labels := map[string]string{
		"alertname": "HighCPU", "severity": "critical",
		"instance": "10.0.0.1:9100", "namespace": "prod",
	}
	if row.Fingerprint != FingerprintFromLabels(labels) {
		t.Errorf("fingerprint fallback wrong: %q", row.Fingerprint)
	}
}

func TestIngestUnknownAndDisabledSource(t *testing.T) {
	env := newAlertTestEnv(t, 5*time.Minute, &fakeSender{results: []SendResult{okResult()}})
	env.addSource("disabled-tok", false)

	if _, _, err := env.svc.Ingest(context.Background(), "nope", []byte(sampleWebhookJSON)); err != ErrSourceNotFound {
		t.Fatalf("err = %v, want ErrSourceNotFound", err)
	}
	if _, _, err := env.svc.Ingest(context.Background(), "disabled-tok", []byte(sampleWebhookJSON)); err != ErrSourceDisabled {
		t.Fatalf("err = %v, want ErrSourceDisabled", err)
	}
	if _, _, err := env.svc.Ingest(context.Background(), "disabled-tok", []byte("junk")); err != ErrSourceDisabled {
		t.Fatalf("disabled source must be rejected before parsing, err = %v", err)
	}
}

func TestDispatchDeliversToMatchedChannels(t *testing.T) {
	env := newAlertTestEnv(t, 5*time.Minute, &fakeSender{results: []SendResult{okResult()}})
	env.addSource("tok123", true)
	env.rules.rules = []model.RoutingRule{
		{ID: 1, SourceID: 1, Priority: 10, MatchLabels: json.RawMessage(`{"severity":"critical"}`), ChannelID: 7, Enabled: true},
		{ID: 2, SourceID: 1, Priority: 20, MatchLabels: json.RawMessage(`{}`), ChannelID: 8, Enabled: true},
	}
	env.channels.byID[7] = &model.Channel{ID: 7, Name: "值班群", Type: "feishu", Enabled: true}
	env.channels.byID[8] = &model.Channel{ID: 8, Name: "默认群", Type: "feishu", Enabled: true}

	if _, _, err := env.svc.Ingest(context.Background(), "tok123", []byte(sampleWebhookJSON)); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	row := env.alerts.byID[1]
	if row.Disposition != "delivered" {
		t.Errorf("disposition = %q, want delivered", row.Disposition)
	}
	// rule 1 命中后停止（continue_matching=false）：只有渠道 7 收到
	rows := env.deliveries.all()
	if len(rows) != 1 || rows[0].ChannelID != 7 || rows[0].RuleID != 1 {
		t.Fatalf("deliveries = %+v, want exactly one to channel 7 via rule 1", rows)
	}
}

func TestDispatchParallelMultiChannel(t *testing.T) {
	env := newAlertTestEnv(t, 5*time.Minute, &fakeSender{results: []SendResult{okResult()}})
	env.addSource("tok123", true)
	env.rules.rules = []model.RoutingRule{
		{ID: 1, SourceID: 1, Priority: 10, MatchLabels: json.RawMessage(`{"severity":"critical"}`),
			ChannelID: 7, ContinueMatching: true, Enabled: true},
		{ID: 2, SourceID: 1, Priority: 20, MatchLabels: json.RawMessage(`{"namespace":"prod"}`), ChannelID: 8, Enabled: true},
	}
	env.channels.byID[7] = &model.Channel{ID: 7, Name: "值班群", Type: "feishu", Enabled: true}
	env.channels.byID[8] = &model.Channel{ID: 8, Name: "运维群", Type: "feishu", Enabled: true}

	if _, _, err := env.svc.Ingest(context.Background(), "tok123", []byte(sampleWebhookJSON)); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	rows := env.deliveries.all()
	if len(rows) != 2 {
		t.Fatalf("deliveries = %d, want 2 (continue_matching fans out)", len(rows))
	}
	seen := map[int64]bool{}
	for _, r := range rows {
		seen[r.ChannelID] = true
	}
	if !seen[7] || !seen[8] {
		t.Errorf("both channels must receive the alert: %+v", rows)
	}
}

func TestDispatchUnmatched(t *testing.T) {
	env := newAlertTestEnv(t, 5*time.Minute, &fakeSender{results: []SendResult{okResult()}})
	env.addSource("tok123", true)
	env.rules.rules = []model.RoutingRule{
		{ID: 1, SourceID: 1, Priority: 10, MatchLabels: json.RawMessage(`{"severity":"warning"}`), ChannelID: 7, Enabled: true},
	}

	if _, _, err := env.svc.Ingest(context.Background(), "tok123", []byte(sampleWebhookJSON)); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if got := env.alerts.byID[1].Disposition; got != "unmatched" {
		t.Errorf("disposition = %q, want unmatched", got)
	}
	if env.sender.calls != 0 {
		t.Errorf("sender calls = %d, want 0", env.sender.calls)
	}
}

func TestDispatchDedup(t *testing.T) {
	env := newAlertTestEnv(t, 5*time.Minute, &fakeSender{results: []SendResult{okResult()}})
	env.addSource("tok123", true)
	env.rules.rules = []model.RoutingRule{
		{ID: 1, SourceID: 1, Priority: 10, MatchLabels: json.RawMessage(`{}`), ChannelID: 7, Enabled: true},
	}
	env.channels.byID[7] = &model.Channel{ID: 7, Type: "feishu", Enabled: true}
	body := []byte(sampleWebhookJSON)

	// 第一次接入：delivered
	if _, _, err := env.svc.Ingest(context.Background(), "tok123", body); err != nil {
		t.Fatalf("Ingest 1: %v", err)
	}
	// 窗口内第二次接入完全相同的内容：deduped
	if _, _, err := env.svc.Ingest(context.Background(), "tok123", body); err != nil {
		t.Fatalf("Ingest 2: %v", err)
	}

	if got := env.alerts.byID[1].Disposition; got != "delivered" {
		t.Errorf("first alert disposition = %q, want delivered", got)
	}
	if got := env.alerts.byID[2].Disposition; got != "deduped" {
		t.Errorf("second alert disposition = %q, want deduped", got)
	}
	if env.sender.calls != 1 {
		t.Errorf("sender calls = %d, want 1 (deduped alert is not sent)", env.sender.calls)
	}
	if rows := env.deliveries.all(); len(rows) != 1 {
		t.Errorf("deliveries = %d, want 1 (dedup produces no delivery)", len(rows))
	}
}

func TestDispatchDedupDisabled(t *testing.T) {
	env := newAlertTestEnv(t, 0, &fakeSender{results: []SendResult{okResult()}})
	env.addSource("tok123", true)
	env.rules.rules = []model.RoutingRule{
		{ID: 1, SourceID: 1, Priority: 10, MatchLabels: json.RawMessage(`{}`), ChannelID: 7, Enabled: true},
	}
	env.channels.byID[7] = &model.Channel{ID: 7, Type: "feishu", Enabled: true}
	body := []byte(sampleWebhookJSON)

	for i := 0; i < 2; i++ {
		if _, _, err := env.svc.Ingest(context.Background(), "tok123", body); err != nil {
			t.Fatalf("Ingest %d: %v", i, err)
		}
	}
	if got := env.alerts.byID[2].Disposition; got != "delivered" {
		t.Errorf("disposition = %q, want delivered (dedup window 0 disables dedup)", got)
	}
	if env.sender.calls != 2 {
		t.Errorf("sender calls = %d, want 2", env.sender.calls)
	}
}

func TestDispatchDedupContentChanged(t *testing.T) {
	env := newAlertTestEnv(t, 5*time.Minute, &fakeSender{results: []SendResult{okResult()}})
	env.addSource("tok123", true)
	env.rules.rules = []model.RoutingRule{
		{ID: 1, SourceID: 1, Priority: 10, MatchLabels: json.RawMessage(`{}`), ChannelID: 7, Enabled: true},
	}
	env.channels.byID[7] = &model.Channel{ID: 7, Type: "feishu", Enabled: true}

	if _, _, err := env.svc.Ingest(context.Background(), "tok123", []byte(sampleWebhookJSON)); err != nil {
		t.Fatalf("Ingest 1: %v", err)
	}
	// fingerprint 相同但 annotations 变了 -> content hash 变 -> 会发送
	var msg map[string]any
	if err := json.Unmarshal([]byte(sampleWebhookJSON), &msg); err != nil {
		t.Fatal(err)
	}
	msg["alerts"].([]any)[0].(map[string]any)["annotations"] = map[string]string{"summary": "CPU above 95% now"}
	body, _ := json.Marshal(msg)
	if _, _, err := env.svc.Ingest(context.Background(), "tok123", body); err != nil {
		t.Fatalf("Ingest 2: %v", err)
	}
	if got := env.alerts.byID[2].Disposition; got != "delivered" {
		t.Errorf("disposition = %q, want delivered (content changed)", got)
	}
	if env.sender.calls != 2 {
		t.Errorf("sender calls = %d, want 2", env.sender.calls)
	}
}

func TestDispatchChannelDisabledTreatedAsUnmatched(t *testing.T) {
	env := newAlertTestEnv(t, 5*time.Minute, &fakeSender{results: []SendResult{okResult()}})
	env.addSource("tok123", true)
	env.rules.rules = []model.RoutingRule{
		{ID: 1, SourceID: 1, Priority: 10, MatchLabels: json.RawMessage(`{}`), ChannelID: 7, Enabled: true},
	}
	env.channels.byID[7] = &model.Channel{ID: 7, Type: "feishu", Enabled: false}

	if _, _, err := env.svc.Ingest(context.Background(), "tok123", []byte(sampleWebhookJSON)); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if got := env.alerts.byID[1].Disposition; got != "unmatched" {
		t.Errorf("disposition = %q, want unmatched (channel disabled)", got)
	}
	if env.sender.calls != 0 {
		t.Errorf("sender calls = %d, want 0", env.sender.calls)
	}
}

func TestDrainWaitsForAsyncDispatch(t *testing.T) {
	sender := &fakeSender{results: []SendResult{okResult()}, delay: 50 * time.Millisecond}
	env := newAlertTestEnv(t, 5*time.Minute, sender)
	env.svc.async = true // 走生产路径：分发在后台 goroutine 执行
	env.addSource("tok123", true)
	env.rules.rules = []model.RoutingRule{
		{ID: 1, SourceID: 1, Priority: 10, MatchLabels: json.RawMessage(`{}`), ChannelID: 7, Enabled: true},
	}
	env.channels.byID[7] = &model.Channel{ID: 7, Name: "默认群", Type: "feishu", Enabled: true}

	if _, _, err := env.svc.Ingest(context.Background(), "tok123", []byte(sampleWebhookJSON)); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := env.svc.Drain(ctx); err != nil {
		t.Fatalf("Drain: %v", err)
	}
	// Drain 返回时进行中的分发必须已完成
	if got := sender.callCount(); got != 1 {
		t.Errorf("sender calls = %d, want 1", got)
	}
	if got := env.alerts.byID[1].Disposition; got != "delivered" {
		t.Errorf("disposition = %q, want delivered", got)
	}
}

func TestDrainTimeout(t *testing.T) {
	sender := &fakeSender{results: []SendResult{okResult()}, delay: 500 * time.Millisecond}
	env := newAlertTestEnv(t, 5*time.Minute, sender)
	env.svc.async = true
	env.addSource("tok123", true)
	env.rules.rules = []model.RoutingRule{
		{ID: 1, SourceID: 1, Priority: 10, MatchLabels: json.RawMessage(`{}`), ChannelID: 7, Enabled: true},
	}
	env.channels.byID[7] = &model.Channel{ID: 7, Name: "默认群", Type: "feishu", Enabled: true}

	if _, _, err := env.svc.Ingest(context.Background(), "tok123", []byte(sampleWebhookJSON)); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := env.svc.Drain(ctx); err == nil {
		t.Fatal("Drain should time out while a slow dispatch is still in flight")
	}
	// 超时返回不取消分发本身，后台 goroutine 最终仍会完成
	if err := env.svc.Drain(context.Background()); err != nil {
		t.Fatalf("second Drain after dispatch finished: %v", err)
	}
}
