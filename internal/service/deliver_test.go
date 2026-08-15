package service

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yongheng0927/fenghuo/internal/model"
	"github.com/yongheng0927/fenghuo/internal/pkg/render"
)

// --- 投递链路用的内存 fake ---

type fakeTemplateStore struct {
	byID     map[int64]*model.Template
	builtins map[string]*model.Template // "type/name"
}

func newFakeTemplateStore(t *testing.T) *fakeTemplateStore {
	t.Helper()
	f := &fakeTemplateStore{byID: map[int64]*model.Template{}, builtins: map[string]*model.Template{}}
	builtins, err := render.BuiltinTemplates()
	if err != nil {
		t.Fatalf("BuiltinTemplates: %v", err)
	}
	var id int64 = 100
	for _, b := range builtins {
		id++
		f.builtins[b.ChannelType+"/"+b.Name] = &model.Template{
			ID: id, Name: b.Name, ChannelType: b.ChannelType, Content: b.Content, IsBuiltin: true,
		}
	}
	return f
}

func (f *fakeTemplateStore) FindByID(_ context.Context, id int64) (*model.Template, error) {
	if t := f.byID[id]; t != nil {
		return t, nil
	}
	// 内置模板在生产环境是真实的数据库行，fake 里也照此镜像
	for _, t := range f.builtins {
		if t.ID == id {
			return t, nil
		}
	}
	return nil, nil
}

func (f *fakeTemplateStore) FindBuiltin(_ context.Context, channelType, name string) (*model.Template, error) {
	return f.builtins[channelType+"/"+name], nil
}

type fakeDeliveryStore struct {
	mu     sync.Mutex
	rows   []*model.Delivery
	nextID int64
}

func (f *fakeDeliveryStore) Create(_ context.Context, d *model.Delivery) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	d.ID = f.nextID
	f.rows = append(f.rows, d)
	return nil
}

func (f *fakeDeliveryStore) all() []*model.Delivery {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*model.Delivery(nil), f.rows...)
}

// fakeSender 按顺序返回预设结果，用完后重复最后一个
type fakeSender struct {
	mu       sync.Mutex
	results  []SendResult
	calls    int
	payloads [][]byte
}

func (f *fakeSender) Send(_ context.Context, _ *model.Channel, payload []byte) SendResult {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.payloads = append(f.payloads, append([]byte(nil), payload...))
	i := f.calls - 1
	if i >= len(f.results) {
		i = len(f.results) - 1
	}
	return f.results[i]
}

func okResult() SendResult        { return SendResult{HTTPStatus: 200, Code: 0, Msg: "success"} }
func transientResult() SendResult { return SendResult{HTTPStatus: 502, Code: -1, Msg: "bad gateway"} }
func businessResult() SendResult {
	return SendResult{HTTPStatus: 200, Code: feishuCodeKeywordMiss, Msg: "Key Words Not Found"}
}

func newTestDeliverer(t *testing.T, sender Sender, retryMax int) (*Deliverer, *fakeDeliveryStore) {
	t.Helper()
	deliveries := &fakeDeliveryStore{}
	d := NewDeliverer(newFakeTemplateStore(t), deliveries,
		newFakeAlertStore(),
		&fakeChannelStore{byID: map[int64]*model.Channel{}},
		&fakeSourceStore{byToken: map[string]*model.Source{}},
		sender, render.NewEngine(time.UTC), retryMax,
		"https://alerts.example.com/")
	d.backoffs = []time.Duration{time.Millisecond, time.Millisecond} // 让测试跑得快
	return d, deliveries
}

func testAlertAndMsg(t *testing.T) (*model.Alert, *AMWebhook) {
	t.Helper()
	msg, err := ParseAMWebhook([]byte(sampleWebhookJSON))
	if err != nil {
		t.Fatalf("ParseAMWebhook: %v", err)
	}
	return &model.Alert{ID: 42, SourceID: 1, Status: "firing"}, msg
}

func TestDeliverSuccess(t *testing.T) {
	sender := &fakeSender{results: []SendResult{okResult()}}
	d, deliveries := newTestDeliverer(t, sender, 2)
	alert, msg := testAlertAndMsg(t)
	ch := &model.Channel{ID: 7, Name: "值班群", Type: "feishu"}

	d.Deliver(context.Background(), alert, msg, ch, 5, "生产环境 Prometheus")

	rows := deliveries.all()
	if len(rows) != 1 {
		t.Fatalf("deliveries = %d, want 1", len(rows))
	}
	row := rows[0]
	if row.Status != "success" || row.Attempts != 1 {
		t.Errorf("row = %+v", row)
	}
	if row.ChannelID != 7 || row.ChannelName != "值班群" || row.RuleID != 5 {
		t.Errorf("snapshot fields wrong: %+v", row)
	}
	if row.TriggerType != "auto" || row.AlertID != 42 {
		t.Errorf("row = %+v", row)
	}
	if row.RenderedPayload == nil || *row.RenderedPayload == "" {
		t.Error("rendered payload must be stored")
	}
	if sender.calls != 1 {
		t.Errorf("sender calls = %d, want 1", sender.calls)
	}
}

func TestDeliverRetriesTransientThenSucceeds(t *testing.T) {
	sender := &fakeSender{results: []SendResult{transientResult(), transientResult(), okResult()}}
	d, deliveries := newTestDeliverer(t, sender, 2)
	alert, msg := testAlertAndMsg(t)

	d.Deliver(context.Background(), alert, msg, &model.Channel{ID: 7, Type: "feishu"}, 0, "src")

	if sender.calls != 3 {
		t.Fatalf("sender calls = %d, want 3 (1 + 2 retries)", sender.calls)
	}
	row := deliveries.all()[0]
	if row.Status != "success" || row.Attempts != 3 {
		t.Errorf("row = %+v, want success with 3 attempts", row)
	}
}

func TestDeliverRetriesExhausted(t *testing.T) {
	sender := &fakeSender{results: []SendResult{transientResult()}}
	d, deliveries := newTestDeliverer(t, sender, 2)
	alert, msg := testAlertAndMsg(t)

	d.Deliver(context.Background(), alert, msg, &model.Channel{ID: 7, Type: "feishu"}, 0, "src")

	if sender.calls != 3 {
		t.Fatalf("sender calls = %d, want 3", sender.calls)
	}
	row := deliveries.all()[0]
	if row.Status != "failed" || row.Attempts != 3 || row.HTTPStatus != 502 {
		t.Errorf("row = %+v", row)
	}
}

func TestDeliverRetryDisabled(t *testing.T) {
	sender := &fakeSender{results: []SendResult{transientResult()}}
	d, _ := newTestDeliverer(t, sender, 0)
	alert, msg := testAlertAndMsg(t)

	d.Deliver(context.Background(), alert, msg, &model.Channel{ID: 7, Type: "feishu"}, 0, "src")

	if sender.calls != 1 {
		t.Fatalf("sender calls = %d, want 1 when retry_max=0", sender.calls)
	}
}

func TestDeliverBusinessErrorNotRetried(t *testing.T) {
	sender := &fakeSender{results: []SendResult{businessResult()}}
	d, deliveries := newTestDeliverer(t, sender, 2)
	alert, msg := testAlertAndMsg(t)

	d.Deliver(context.Background(), alert, msg, &model.Channel{ID: 7, Type: "feishu"}, 0, "src")

	if sender.calls != 1 {
		t.Fatalf("sender calls = %d, want 1 (business errors are not retried)", sender.calls)
	}
	row := deliveries.all()[0]
	if row.Status != "failed" || row.ResponseCode != feishuCodeKeywordMiss {
		t.Errorf("row = %+v", row)
	}
	if row.ResponseMsg == "" {
		t.Error("response_msg must carry a human-readable hint")
	}
}

func TestDeliverKeywordGuard(t *testing.T) {
	sender := &fakeSender{results: []SendResult{okResult()}}
	d, deliveries := newTestDeliverer(t, sender, 2)
	alert, msg := testAlertAndMsg(t)
	ch := &model.Channel{ID: 7, Type: "feishu", Keyword: "必须出现的关键词"}

	d.Deliver(context.Background(), alert, msg, ch, 0, "src")

	if sender.calls != 0 {
		t.Fatalf("sender must not be called when keyword is missing, calls = %d", sender.calls)
	}
	row := deliveries.all()[0]
	if row.Status != "failed" || row.ResponseCode != -1 {
		t.Errorf("row = %+v", row)
	}
}

func TestDeliverRenderErrorRecorded(t *testing.T) {
	sender := &fakeSender{results: []SendResult{okResult()}}
	deliveries := &fakeDeliveryStore{}
	templates := newFakeTemplateStore(t)
	templates.byID[9] = &model.Template{ID: 9, ChannelType: "feishu", Content: "{{ .Bogus"}
	d := NewDeliverer(templates, deliveries, newFakeAlertStore(), &fakeChannelStore{byID: map[int64]*model.Channel{}}, &fakeSourceStore{byToken: map[string]*model.Source{}}, sender, render.NewEngine(time.UTC), 2, "https://x/")
	tplID := int64(9)
	alert, msg := testAlertAndMsg(t)
	ch := &model.Channel{ID: 7, Type: "feishu", TemplateID: &tplID}

	d.Deliver(context.Background(), alert, msg, ch, 0, "src")

	if sender.calls != 0 {
		t.Fatalf("render error must not call the sender, calls = %d", sender.calls)
	}
	row := deliveries.all()[0]
	if row.Status != "failed" || row.Attempts != 1 || row.RenderedPayload != nil {
		t.Errorf("row = %+v", row)
	}
}

func TestDeliverBoundTemplateUsed(t *testing.T) {
	sender := &fakeSender{results: []SendResult{okResult()}}
	deliveries := &fakeDeliveryStore{}
	templates := newFakeTemplateStore(t)
	templates.byID[9] = &model.Template{
		ID: 9, ChannelType: "feishu",
		Content: `{"msg_type":"text","content":{"text":"custom {{ label .CommonLabels "alertname" }}"}}`,
	}
	d := NewDeliverer(templates, deliveries, newFakeAlertStore(), &fakeChannelStore{byID: map[int64]*model.Channel{}}, &fakeSourceStore{byToken: map[string]*model.Source{}}, sender, render.NewEngine(time.UTC), 2, "https://x/")
	tplID := int64(9)
	alert, msg := testAlertAndMsg(t)
	ch := &model.Channel{ID: 7, Type: "feishu", TemplateID: &tplID}

	d.Deliver(context.Background(), alert, msg, ch, 0, "src")

	row := deliveries.all()[0]
	if row.RenderedPayload == nil || !strings.Contains(*row.RenderedPayload, "custom HighCPU") {
		t.Errorf("bound template must be used: %v", row.RenderedPayload)
	}
}
