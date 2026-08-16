package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yongheng0927/fenghuo/internal/model"
	"github.com/yongheng0927/fenghuo/internal/pkg/render"
)

func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }
func int64Ptr(i int64) *int64 { return &i }

// --- SourceService ---

func newSourceService() (*SourceService, *fakeSourceStore, *fakeRuleStore, *fakeAlertStore) {
	sources := &fakeSourceStore{byToken: map[string]*model.Source{}}
	rules := &fakeRuleStore{}
	alerts := newFakeAlertStore()
	return NewSourceService(sources, rules, alerts), sources, rules, alerts
}

func TestSourceCreateGeneratesToken(t *testing.T) {
	svc, _, _, _ := newSourceService()
	src, err := svc.Create(context.Background(), "生产 Prometheus", "desc")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(src.Token) != 32 {
		t.Fatalf("token length = %d, want 32 hex chars", len(src.Token))
	}
	if !src.Enabled {
		t.Error("new source must be enabled")
	}
	src2, _ := svc.Create(context.Background(), "测试 Prometheus", "")
	if src2.Token == src.Token {
		t.Fatal("tokens must be unique")
	}
}

func TestSourceCreateDuplicateName(t *testing.T) {
	svc, _, _, _ := newSourceService()
	_, _ = svc.Create(context.Background(), "prod", "")
	if _, err := svc.Create(context.Background(), "prod", ""); !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("err = %v, want ErrDuplicateName", err)
	}
	if _, err := svc.Create(context.Background(), "", ""); !errors.Is(err, ErrValidation) {
		t.Fatalf("err = %v, want ErrValidation", err)
	}
}

func TestSourceDeleteGuards(t *testing.T) {
	svc, sources, rules, alerts := newSourceService()
	src, _ := svc.Create(context.Background(), "prod", "")

	rules.rules = append(rules.rules, model.RoutingRule{ID: 1, SourceID: src.ID, ChannelID: 7})
	if err := svc.Delete(context.Background(), src.ID); !errors.Is(err, ErrReferenced) {
		t.Fatalf("err = %v, want ErrReferenced (rules exist)", err)
	}
	rules.rules = nil

	_ = alerts.Create(context.Background(), &model.Alert{SourceID: src.ID})
	if err := svc.Delete(context.Background(), src.ID); !errors.Is(err, ErrReferenced) {
		t.Fatalf("err = %v, want ErrReferenced (alerts exist)", err)
	}
	alerts.byID = map[int64]*model.Alert{}

	if err := svc.Delete(context.Background(), src.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if s, _ := sources.FindByID(context.Background(), src.ID); s != nil {
		t.Fatal("source must be deleted")
	}
	if err := svc.Delete(context.Background(), 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestSourceRotateToken(t *testing.T) {
	svc, _, _, _ := newSourceService()
	src, _ := svc.Create(context.Background(), "prod", "")
	old := src.Token
	rotated, err := svc.RotateToken(context.Background(), src.ID)
	if err != nil {
		t.Fatalf("RotateToken: %v", err)
	}
	if rotated.Token == old || len(rotated.Token) != 32 {
		t.Fatalf("token not rotated: %q", rotated.Token)
	}
	if _, err := svc.RotateToken(context.Background(), 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

// --- ChannelService ---

func TestMaskCredential(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"ab12", "****ab12"},
		{"my-secret-token", "****oken"},
		{"https://open.feishu.cn/open-apis/bot/v2/hook/abcdef123456",
			"https://open.feishu.cn/open-apis/bot/v2/hook/****3456"},
	}
	for _, tc := range cases {
		if got := MaskCredential(tc.in); got != tc.want {
			t.Errorf("MaskCredential(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func newChannelService(t *testing.T) (*ChannelService, *fakeChannelStore, *fakeRuleStore, *fakeTemplateStore, *fakeSender) {
	t.Helper()
	channels := &fakeChannelStore{byID: map[int64]*model.Channel{}}
	rules := &fakeRuleStore{}
	templates := newFakeTemplateStore(t)
	sender := &fakeSender{results: []SendResult{okResult()}}
	svc := NewChannelService(channels, rules, templates, testCipher(t), sender)
	return svc, channels, rules, templates, sender
}

func TestChannelCreateEncryptsCredentials(t *testing.T) {
	svc, channels, _, _, _ := newChannelService(t)
	ch, err := svc.Create(context.Background(), ChannelInput{
		Name:       "值班群",
		WebhookURL: "https://open.feishu.cn/open-apis/bot/v2/hook/abcdef123456",
		Enabled:    true,
	}, "my-secret")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if ch.Type != "feishu" {
		t.Errorf("type = %q, want feishu", ch.Type)
	}
	stored := channels.byID[ch.ID]
	if strings.Contains(string(stored.WebhookURLEncrypted), "abcdef123456") {
		t.Error("webhook url must be stored encrypted")
	}
	// 往返验证：解密得到原值，视图里是脱敏后的
	v, err := svc.View(stored)
	if err != nil {
		t.Fatalf("View: %v", err)
	}
	if v.WebhookURL != "https://open.feishu.cn/open-apis/bot/v2/hook/****3456" {
		t.Errorf("masked url = %q", v.WebhookURL)
	}
	if !v.HasSecret || v.Secret != "****cret" {
		t.Errorf("masked secret = %q has=%v", v.Secret, v.HasSecret)
	}
}

func TestChannelCreateValidation(t *testing.T) {
	svc, _, _, _, _ := newChannelService(t)
	if _, err := svc.Create(context.Background(), ChannelInput{Name: "", WebhookURL: "https://open.feishu.cn/hook/x"}, ""); !errors.Is(err, ErrValidation) {
		t.Fatalf("empty name: %v", err)
	}
	if _, err := svc.Create(context.Background(), ChannelInput{Name: "a"}, ""); !errors.Is(err, ErrValidation) {
		t.Fatalf("empty url: %v", err)
	}
	if _, err := svc.Create(context.Background(), ChannelInput{Name: "a", WebhookURL: "ftp://bad"}, ""); !errors.Is(err, ErrValidation) {
		t.Fatalf("bad scheme: %v", err)
	}
	if _, err := svc.Create(context.Background(), ChannelInput{Name: "a", Type: "slack", WebhookURL: "https://open.feishu.cn/hook/x"}, ""); !errors.Is(err, ErrValidation) {
		t.Fatalf("bad channel type: %v", err)
	}
	if _, err := svc.Create(context.Background(), ChannelInput{Name: "a", WebhookURL: "https://example.com/hook/x"}, ""); !errors.Is(err, ErrValidation) {
		t.Fatalf("wrong host for feishu: %v", err)
	}
	if _, err := svc.Create(context.Background(), ChannelInput{Name: "a", WebhookURL: "https://open.feishu.cn/hook/x", TemplateID: int64Ptr(999)}, ""); !errors.Is(err, ErrValidation) {
		t.Fatalf("missing template: %v", err)
	}
	_, _ = svc.Create(context.Background(), ChannelInput{Name: "dup", WebhookURL: "https://open.feishu.cn/hook/x"}, "")
	if _, err := svc.Create(context.Background(), ChannelInput{Name: "dup", WebhookURL: "https://open.feishu.cn/hook/x"}, ""); !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("dup name: %v", err)
	}
}

func TestChannelCreateDingTalkAndWeCom(t *testing.T) {
	svc, channels, _, _, _ := newChannelService(t)
	dt, err := svc.Create(context.Background(), ChannelInput{
		Name: "钉钉群", Type: "dingtalk",
		WebhookURL: "https://oapi.dingtalk.com/robot/send?access_token=abc",
		Enabled:    true,
	}, "SECdingsecret")
	if err != nil {
		t.Fatalf("Create dingtalk: %v", err)
	}
	if dt.Type != "dingtalk" {
		t.Errorf("type = %q", dt.Type)
	}
	if channels.byID[dt.ID].SecretEncrypted == nil {
		t.Error("dingtalk secret must be stored (sign supported)")
	}

	wc, err := svc.Create(context.Background(), ChannelInput{
		Name: "企微群", Type: "wecom",
		WebhookURL: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=abc",
		Enabled:    true,
	}, "ignored-secret")
	if err != nil {
		t.Fatalf("Create wecom: %v", err)
	}
	if channels.byID[wc.ID].SecretEncrypted != nil {
		t.Error("wecom has no sign mechanism; secret must be ignored")
	}
}

func TestChannelUpdateKeepsCredentials(t *testing.T) {
	svc, channels, _, _, _ := newChannelService(t)
	ch, _ := svc.Create(context.Background(), ChannelInput{
		Name: "值班群", WebhookURL: "https://open.feishu.cn/hook/aaaabbbb", Enabled: true,
	}, "secret-1")
	origURL := channels.byID[ch.ID].WebhookURLEncrypted

	// 只改名字：凭证不动
	if _, err := svc.Update(context.Background(), ch.ID, ChannelPatch{Name: strPtr("新名字")}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	stored := channels.byID[ch.ID]
	if string(stored.WebhookURLEncrypted) != string(origURL) {
		t.Error("webhook url must be kept when not resubmitted")
	}
	if stored.SecretEncrypted == nil {
		t.Error("secret must be kept when not resubmitted")
	}

	// 显式传空 secret 即清除，传新 url 则替换
	if _, err := svc.Update(context.Background(), ch.ID, ChannelPatch{
		Secret:     strPtr(""),
		WebhookURL: strPtr("https://open.feishu.cn/hook/ccccdddd"),
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	stored = channels.byID[ch.ID]
	if stored.SecretEncrypted != nil {
		t.Error("explicit empty secret must clear the secret")
	}
	v, _ := svc.View(stored)
	if !strings.HasSuffix(v.WebhookURL, "****dddd") {
		t.Errorf("url must be replaced: %q", v.WebhookURL)
	}
}

func TestChannelUpdateTemplateBinding(t *testing.T) {
	svc, _, _, templates, _ := newChannelService(t)
	ch, _ := svc.Create(context.Background(), ChannelInput{
		Name: "值班群", WebhookURL: "https://open.feishu.cn/hook/aaaabbbb", Enabled: true,
	}, "")
	templates.byID[9] = &model.Template{ID: 9, ChannelType: "feishu", Name: "custom",
		Content: `**x**`}
	templates.byID[10] = &model.Template{ID: 10, ChannelType: "dingtalk", Name: "dt",
		Content: `**x**`}

	if _, err := svc.Update(context.Background(), ch.ID, ChannelPatch{TemplateID: int64Ptr(9)}); err != nil {
		t.Fatalf("bind template: %v", err)
	}
	if _, err := svc.Update(context.Background(), ch.ID, ChannelPatch{TemplateID: int64Ptr(999)}); !errors.Is(err, ErrValidation) {
		t.Fatalf("bind missing template: %v", err)
	}
	// 模板渠道类型与渠道类型不一致时必须拒绝
	if _, err := svc.Update(context.Background(), ch.ID, ChannelPatch{TemplateID: int64Ptr(10)}); !errors.Is(err, ErrValidation) {
		t.Fatalf("bind cross-type template: %v", err)
	}
	updated, _ := svc.Update(context.Background(), ch.ID, ChannelPatch{ClearTemplate: true})
	if updated.TemplateID != nil {
		t.Error("ClearTemplate must unbind")
	}
}

func TestChannelDeleteGuard(t *testing.T) {
	svc, _, rules, _, _ := newChannelService(t)
	ch, _ := svc.Create(context.Background(), ChannelInput{
		Name: "值班群", WebhookURL: "https://open.feishu.cn/hook/aaaabbbb", Enabled: true,
	}, "")
	rules.rules = append(rules.rules, model.RoutingRule{ID: 1, SourceID: 1, ChannelID: ch.ID})
	if err := svc.Delete(context.Background(), ch.ID); !errors.Is(err, ErrReferenced) {
		t.Fatalf("err = %v, want ErrReferenced", err)
	}
	rules.rules = nil
	if err := svc.Delete(context.Background(), ch.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestChannelTestSend(t *testing.T) {
	svc, _, _, _, sender := newChannelService(t)
	ch, _ := svc.Create(context.Background(), ChannelInput{
		Name: "值班群", WebhookURL: "https://open.feishu.cn/hook/aaaabbbb", Enabled: true,
		Keyword: "告警",
	}, "")
	res, err := svc.TestSend(context.Background(), ch.ID)
	if err != nil {
		t.Fatalf("TestSend: %v", err)
	}
	if !res.Success {
		t.Errorf("res = %+v, want success", res)
	}
	if sender.calls != 1 {
		t.Fatalf("sender calls = %d", sender.calls)
	}
	if !strings.Contains(string(sender.payloads[0]), "告警") {
		t.Error("test message must contain the channel keyword")
	}
	if _, err := svc.TestSend(context.Background(), 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

// --- TemplateService ---

const validTemplateContent = `{"msg_type":"text","content":{"text":"**{{ label .CommonLabels "alertname" | jesc }}**"}}`

func newTemplateService(t *testing.T) (*TemplateService, *fakeTemplateStore, *fakeChannelStore, *fakeAlertStore) {
	t.Helper()
	templates := newFakeTemplateStore(t)
	channels := &fakeChannelStore{byID: map[int64]*model.Channel{}}
	alerts := newFakeAlertStore()
	sender := &fakeSender{results: []SendResult{okResult()}}
	svc := NewTemplateService(templates, channels, alerts, render.NewEngine(time.UTC), sender, "https://alerts.example.com/")
	return svc, templates, channels, alerts
}

func TestTemplateCreateValidation(t *testing.T) {
	svc, _, _, _ := newTemplateService(t)
	in := TemplateInput{Name: "custom", ChannelType: "feishu", Content: validTemplateContent}
	tmpl, err := svc.Create(context.Background(), in)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if tmpl.IsBuiltin {
		t.Error("custom template must not be builtin")
	}

	bad := []TemplateInput{
		{Name: "", ChannelType: "feishu", Content: validTemplateContent},
		{Name: "x", ChannelType: "common", Content: validTemplateContent}, // channel_type 只能 feishu/dingtalk/wecom
		{Name: "x", ChannelType: "feishu", Content: ""},
		{Name: "x", ChannelType: "feishu", Content: "{{ .Bogus"},
	}
	for i, in := range bad {
		if _, err := svc.Create(context.Background(), in); !errors.Is(err, ErrValidation) {
			t.Errorf("case %d: err = %v, want ErrValidation", i, err)
		}
	}
	if _, err := svc.Create(context.Background(), in); !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("dup: err = %v", err)
	}
}

func TestTemplateBuiltinReadOnly(t *testing.T) {
	svc, templates, _, _ := newTemplateService(t)
	builtin := templates.builtins["feishu/plain-text"]

	if _, err := svc.Update(context.Background(), builtin.ID, TemplateInput{
		Name: "plain-text", ChannelType: "feishu", Content: validTemplateContent,
	}); !errors.Is(err, ErrBuiltinReadOnly) {
		t.Fatalf("update builtin: err = %v", err)
	}
	if err := svc.Delete(context.Background(), builtin.ID); !errors.Is(err, ErrBuiltinReadOnly) {
		t.Fatalf("delete builtin: err = %v", err)
	}
}

func TestTemplateDeleteGuard(t *testing.T) {
	svc, templates, channels, _ := newTemplateService(t)
	tmpl, _ := svc.Create(context.Background(), TemplateInput{
		Name: "custom", ChannelType: "feishu", Content: validTemplateContent,
	})
	channels.byID[7] = &model.Channel{ID: 7, TemplateID: &tmpl.ID}
	if err := svc.Delete(context.Background(), tmpl.ID); !errors.Is(err, ErrReferenced) {
		t.Fatalf("err = %v, want ErrReferenced", err)
	}
	delete(channels.byID, 7)
	if err := svc.Delete(context.Background(), tmpl.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if got, _ := templates.FindByID(context.Background(), tmpl.ID); got != nil {
		t.Fatal("template must be deleted")
	}
}

func TestTemplatePreviewFallbacks(t *testing.T) {
	svc, _, _, alerts := newTemplateService(t)
	ctx := context.Background()
	tmpl := `{"msg_type":"text","content":{"text":"**{{ label .CommonLabels "alertname" | jesc }}**"}}`

	// 1. 显式给的告警 JSON 优先；rendered 是该渠道的消息体 JSON 字符串
	rendered, err := svc.Preview(ctx, tmpl, "feishu", []byte(sampleWebhookJSON))
	if err != nil || !strings.Contains(rendered, "HighCPU") {
		t.Fatalf("explicit payload: rendered=%q err=%v", rendered, err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(rendered), &m); err != nil || m["msg_type"] != "text" {
		t.Fatalf("rendered must be a feishu payload: %q err=%v", rendered, err)
	}

	// 按渠道类型校验：飞书 payload 用 dingtalk 类型预览必须失败
	if _, err := svc.Preview(ctx, tmpl, "dingtalk", []byte(sampleWebhookJSON)); err == nil {
		t.Fatal("feishu payload must not pass dingtalk validation")
	}

	// 2. 没带告警 JSON：用最近一条入库告警
	stored := strings.ReplaceAll(sampleWebhookJSON, "HighCPU", "StoredAlert")
	_ = alerts.Create(ctx, &model.Alert{
		SourceID: 1, ReceivedAt: time.Now(), RawPayload: json.RawMessage(stored),
	})
	rendered, err = svc.Preview(ctx, tmpl, "feishu", nil)
	if err != nil || !strings.Contains(rendered, "StoredAlert") {
		t.Fatalf("latest stored alert: rendered=%q err=%v", rendered, err)
	}

	// 3. store 为空：用内置样例
	alerts.byID = map[int64]*model.Alert{}
	rendered, err = svc.Preview(ctx, tmpl, "feishu", nil)
	if err != nil || !strings.Contains(rendered, "HighCPU") {
		t.Fatalf("builtin sample: rendered=%q err=%v", rendered, err)
	}

	// 4. 渲染错误、空内容、非法渠道类型都要报错
	if _, err := svc.Preview(ctx, "{{ .Bogus", "feishu", nil); err == nil {
		t.Fatal("syntax error must fail")
	}
	if _, err := svc.Preview(ctx, "", "feishu", nil); !errors.Is(err, ErrValidation) {
		t.Fatalf("empty content: err = %v", err)
	}
	if _, err := svc.Preview(ctx, tmpl, "slack", nil); !errors.Is(err, ErrValidation) {
		t.Fatalf("bad channel type: err = %v", err)
	}
}

func TestTemplateTestSend(t *testing.T) {
	templates := newFakeTemplateStore(t)
	channels := &fakeChannelStore{byID: map[int64]*model.Channel{}}
	alerts := newFakeAlertStore()
	sender := &fakeSender{results: []SendResult{okResult()}}
	svc := NewTemplateService(templates, channels, alerts, render.NewEngine(time.UTC), sender, "https://alerts.example.com/")
	ctx := context.Background()

	// 成功路径：渲染结果送达类型匹配的渠道
	channels.byID[1] = &model.Channel{ID: 1, Name: "fs", Type: "feishu"}
	res, err := svc.TestSend(ctx, validTemplateContent, "feishu", nil, 1)
	if err != nil || !res.Success {
		t.Fatalf("res = %+v, err = %v, want success", res, err)
	}
	if sender.calls != 1 {
		t.Fatalf("sender calls = %d", sender.calls)
	}
	var m map[string]any
	if err := json.Unmarshal(sender.payloads[0], &m); err != nil || m["msg_type"] != "text" {
		t.Fatalf("sent payload must be the rendered feishu message: %q", sender.payloads[0])
	}

	// 渠道类型不匹配
	if _, err := svc.TestSend(ctx, validTemplateContent, "dingtalk", nil, 1); !errors.Is(err, ErrValidation) {
		t.Fatalf("type mismatch: err = %v, want ErrValidation", err)
	}
	// 渠道不存在
	if _, err := svc.TestSend(ctx, validTemplateContent, "feishu", nil, 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing channel: err = %v, want ErrNotFound", err)
	}
	// 渲染结果缺少渠道关键词：本地拦截，不发送
	channels.byID[2] = &model.Channel{ID: 2, Name: "kw", Type: "feishu", Keyword: "必须出现的关键词"}
	if _, err := svc.TestSend(ctx, validTemplateContent, "feishu", nil, 2); !errors.Is(err, ErrValidation) {
		t.Fatalf("keyword miss: err = %v, want ErrValidation", err)
	}
	if sender.calls != 1 {
		t.Fatalf("keyword miss must not send, sender calls = %d", sender.calls)
	}
	// 渲染失败
	if _, err := svc.TestSend(ctx, "{{ .Bogus", "feishu", nil, 1); err == nil {
		t.Fatal("syntax error must fail")
	}
}

// --- RuleService ---

func newRuleService() (*RuleService, *fakeRuleStore, *fakeSourceStore, *fakeChannelStore) {
	rules := &fakeRuleStore{}
	sources := &fakeSourceStore{byToken: map[string]*model.Source{
		"tok": {ID: 1, Name: "prod", Token: "tok", Enabled: true},
	}}
	channels := &fakeChannelStore{byID: map[int64]*model.Channel{
		7: {ID: 7, Name: "值班群", Type: "feishu", Enabled: true},
	}}
	return NewRuleService(rules, sources, channels), rules, sources, channels
}

func TestRuleCreateValidation(t *testing.T) {
	svc, _, _, _ := newRuleService()
	base := RuleInput{
		SourceID: 1, ChannelID: 7, Priority: 10,
		MatchLabels: json.RawMessage(`{"severity":"critical"}`), Enabled: true,
	}
	if _, err := svc.Create(context.Background(), base); err != nil {
		t.Fatalf("Create: %v", err)
	}

	bad := []RuleInput{
		{SourceID: 0, ChannelID: 7, MatchLabels: json.RawMessage(`{}`)},
		{SourceID: 999, ChannelID: 7, MatchLabels: json.RawMessage(`{}`)},
		{SourceID: 1, ChannelID: 0, MatchLabels: json.RawMessage(`{}`)},
		{SourceID: 1, ChannelID: 999, MatchLabels: json.RawMessage(`{}`)},
		{SourceID: 1, ChannelID: 7, MatchLabels: nil},
		{SourceID: 1, ChannelID: 7, MatchLabels: json.RawMessage(`["not","object"]`)},
		{SourceID: 1, ChannelID: 7, MatchLabels: json.RawMessage(`{"sev":1}`)},
	}
	for i, in := range bad {
		if _, err := svc.Create(context.Background(), in); !errors.Is(err, ErrValidation) {
			t.Errorf("case %d: err = %v, want ErrValidation", i, err)
		}
	}
}

func TestRuleUpdateDelete(t *testing.T) {
	svc, rules, _, _ := newRuleService()
	r, _ := svc.Create(context.Background(), RuleInput{
		SourceID: 1, ChannelID: 7, Priority: 10,
		MatchLabels: json.RawMessage(`{}`), Enabled: true,
	})
	updated, err := svc.Update(context.Background(), r.ID, RuleInput{
		SourceID: 1, ChannelID: 7, Priority: 5, Name: "默认兜底",
		MatchLabels: json.RawMessage(`{}`), Enabled: false,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Priority != 5 || updated.Enabled {
		t.Errorf("updated = %+v", updated)
	}
	if _, err := svc.Update(context.Background(), 999, RuleInput{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if err := svc.Delete(context.Background(), r.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if got, _ := rules.FindByID(context.Background(), r.ID); got != nil {
		t.Fatal("rule must be deleted")
	}
}

// --- UserAdminService ---

func newUserAdminService() (*UserAdminService, *fakeUserStore, *fakeTokenStore, *fakeOAuthIdentityStore) {
	users := newFakeUserStore()
	tokens := newFakeTokenStore()
	oauth := &fakeOAuthIdentityStore{}
	return NewUserAdminService(users, tokens, oauth, "admin1"), users, tokens, oauth
}

func TestUserAdminCreate(t *testing.T) {
	svc, users, _, _ := newUserAdminService()
	ctx := context.Background()

	// 缺省角色为 viewer，密码以哈希存储
	u, err := svc.Create(ctx, "bob", "pw-123456", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if u.Role != model.RoleViewer || !u.Enabled {
		t.Errorf("default role/enabled: %+v", u)
	}
	if u.PasswordHash == nil || *u.PasswordHash == "pw-123456" {
		t.Error("password must be stored hashed")
	}

	// 显式角色
	admin, err := svc.Create(ctx, "ops", "pw-123456", "admin")
	if err != nil || admin.Role != model.RoleAdmin {
		t.Fatalf("create admin: %v, %+v", err, admin)
	}

	// 用户名去空白、查重
	if _, err := svc.Create(ctx, " bob ", "pw-123456", ""); !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("duplicate: err = %v, want ErrDuplicateName", err)
	}
	// 非法输入
	bad := [][3]string{
		{"", "pw-123456", "viewer"},     // 空用户名
		{"x", "short", "viewer"},        // 密码不足 8 位
		{"x", "pw-123456", "superroot"}, // 非法角色
	}
	for i, in := range bad {
		if _, err := svc.Create(ctx, in[0], in[1], in[2]); !errors.Is(err, ErrValidation) {
			t.Errorf("case %d: err = %v, want ErrValidation", i, err)
		}
	}
	if n, _ := users.Count(ctx); n != 2 {
		t.Errorf("users = %d, want 2 (failed creates must not persist)", n)
	}
}

func TestUserAdminInitialAdminGuards(t *testing.T) {
	svc, users, _, _ := newUserAdminService() // 测试夹具中 admin1 即初始管理员
	initial := users.addUser("admin1", "pw-123456", model.RoleAdmin, true)
	users.addUser("admin2", "pw-123456", model.RoleAdmin, true)
	ctx := context.Background()

	// 初始管理员：改角色、禁用、删除都被拒
	if _, err := svc.Update(ctx, initial.ID, strPtr("editor"), nil); !errors.Is(err, ErrInitialAdmin) {
		t.Fatalf("demote initial: err = %v, want ErrInitialAdmin", err)
	}
	if _, err := svc.Update(ctx, initial.ID, nil, boolPtr(false)); !errors.Is(err, ErrInitialAdmin) {
		t.Fatalf("disable initial: err = %v, want ErrInitialAdmin", err)
	}
	if err := svc.Delete(ctx, 999, initial.ID); !errors.Is(err, ErrInitialAdmin) {
		t.Fatalf("delete initial: err = %v, want ErrInitialAdmin", err)
	}
	// 相同角色/启用状态的原样更新是允许的（幂等操作不算降级）
	if _, err := svc.Update(ctx, initial.ID, strPtr("admin"), boolPtr(true)); err != nil {
		t.Fatalf("no-op update on initial admin: %v", err)
	}
	// 其他 admin 不受此守卫影响（仍有最后一个 admin 守卫兜底）
	other, _ := users.FindByUsername(ctx, "admin2")
	if _, err := svc.Update(ctx, other.ID, strPtr("editor"), nil); err != nil {
		t.Fatalf("demote non-initial admin: %v", err)
	}
}

func TestUserAdminUpdateRole(t *testing.T) {
	svc, users, _, _ := newUserAdminService()
	users.addUser("admin1", "pw-123456", model.RoleAdmin, true)
	u := users.addUser("alice", "", model.RoleViewer, true)

	updated, err := svc.Update(context.Background(), u.ID, strPtr("editor"), nil)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Role != model.RoleEditor {
		t.Errorf("role = %q", updated.Role)
	}
	if _, err := svc.Update(context.Background(), u.ID, strPtr("superroot"), nil); !errors.Is(err, ErrValidation) {
		t.Fatalf("invalid role: %v", err)
	}
	if _, err := svc.Update(context.Background(), 999, strPtr("viewer"), nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing user: %v", err)
	}
}

func TestUserAdminDisableRevokesTokens(t *testing.T) {
	svc, users, tokens, _ := newUserAdminService()
	users.addUser("admin1", "pw-123456", model.RoleAdmin, true)
	u := users.addUser("alice", "pw-123456", model.RoleEditor, true)
	// 给 alice 一个有效的 refresh token
	tokens.byHash["hash1"] = &model.RefreshToken{UserID: u.ID, TokenHash: "hash1"}

	if _, err := svc.Update(context.Background(), u.ID, nil, boolPtr(false)); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !tokens.byHash["hash1"].Revoked {
		t.Error("disabling a user must revoke all refresh tokens")
	}
}

func TestUserAdminLastAdminGuards(t *testing.T) {
	svc, users, _, _ := newUserAdminService() // admin1 是初始管理员，这里用非初始账号验证最后 admin 守卫
	admin := users.addUser("admin2", "pw-123456", model.RoleAdmin, true)

	if _, err := svc.Update(context.Background(), admin.ID, strPtr("editor"), nil); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("demote last admin: %v", err)
	}
	if _, err := svc.Update(context.Background(), admin.ID, nil, boolPtr(false)); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("disable last admin: %v", err)
	}
	if err := svc.Delete(context.Background(), 999, admin.ID); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("delete last admin: %v", err)
	}

	// 有了第二个 admin 后守卫解除
	users.addUser("admin1", "pw-123456", model.RoleAdmin, true)
	if _, err := svc.Update(context.Background(), admin.ID, strPtr("editor"), nil); err != nil {
		t.Fatalf("demote with second admin present: %v", err)
	}
}

func TestUserAdminDelete(t *testing.T) {
	svc, users, tokens, oauth := newUserAdminService()
	admin := users.addUser("admin1", "pw-123456", model.RoleAdmin, true)
	users.addUser("admin2", "pw-123456", model.RoleAdmin, true)
	u := users.addUser("alice", "", model.RoleViewer, true)
	tokens.byHash["h"] = &model.RefreshToken{UserID: u.ID, TokenHash: "h"}

	if err := svc.Delete(context.Background(), admin.ID, admin.ID); !errors.Is(err, ErrCannotDeleteSelf) {
		t.Fatalf("delete self: %v", err)
	}
	if err := svc.Delete(context.Background(), admin.ID, u.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if got, _ := users.FindByID(context.Background(), u.ID); got != nil {
		t.Fatal("user must be deleted")
	}
	if len(tokens.byHash) != 0 {
		t.Error("refresh tokens must be cleaned up")
	}
	if len(oauth.deleted) != 1 || oauth.deleted[0] != u.ID {
		t.Error("oauth identities must be cleaned up")
	}
	if err := svc.Delete(context.Background(), admin.ID, 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing user: %v", err)
	}
}

// --- 手动重发（FR-2.6） ---

func TestResendSemantics(t *testing.T) {
	sender := &fakeSender{results: []SendResult{okResult()}}
	deliverer, deliveries := newTestDeliverer(t, sender, 0)
	alerts := newFakeAlertStore()
	channels := &fakeChannelStore{byID: map[int64]*model.Channel{}}
	sources := &fakeSourceStore{byToken: map[string]*model.Source{
		"tok": {ID: 1, Name: "prod", Token: "tok", Enabled: true},
	}}
	deliverer.alerts = alerts
	deliverer.channels = channels
	deliverer.sources = sources

	// 原本失败的自动投递
	_ = alerts.Create(context.Background(), &model.Alert{
		SourceID: 1, Status: "firing", RawPayload: json.RawMessage(sampleWebhookJSON), ReceivedAt: time.Now(),
	})
	channels.byID[7] = &model.Channel{ID: 7, Name: "值班群", Type: "feishu", Enabled: true}
	orig := &model.Delivery{
		AlertID: 1, ChannelID: 7, ChannelName: "值班群", RuleID: 5,
		TriggerType: "auto", Attempts: 1, Status: "failed",
		ResponseCode: feishuCodeKeywordMiss, ResponseMsg: "Key Words Not Found", SentAt: time.Now(),
	}
	_ = deliveries.Create(context.Background(), orig)

	row, err := deliverer.Resend(context.Background(), orig.ID)
	if err != nil {
		t.Fatalf("Resend: %v", err)
	}
	if row.TriggerType != "manual" {
		t.Errorf("trigger_type = %q, want manual", row.TriggerType)
	}
	if row.AlertID != orig.AlertID || row.ChannelID != orig.ChannelID || row.RuleID != orig.RuleID {
		t.Errorf("resend must reuse alert/channel/rule ids: %+v", row)
	}
	if row.Status != "success" {
		t.Errorf("resend result = %q, want success (sender now OK)", row.Status)
	}
	// 原记录保持不动
	again, _ := deliveries.FindByID(context.Background(), orig.ID)
	if again.Status != "failed" || again.ResponseCode != feishuCodeKeywordMiss {
		t.Errorf("original delivery must stay untouched: %+v", again)
	}
	if len(deliveries.all()) != 2 {
		t.Errorf("deliveries = %d, want 2 (original + manual resend)", len(deliveries.all()))
	}
}

func TestResendRefusals(t *testing.T) {
	sender := &fakeSender{results: []SendResult{okResult()}}
	deliverer, deliveries := newTestDeliverer(t, sender, 0)
	alerts := newFakeAlertStore()
	channels := &fakeChannelStore{byID: map[int64]*model.Channel{}}
	deliverer.alerts = alerts
	deliverer.channels = channels
	deliverer.sources = &fakeSourceStore{byToken: map[string]*model.Source{}}

	if _, err := deliverer.Resend(context.Background(), 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing delivery: %v", err)
	}

	// 渠道已删除 -> 拒绝（FR-2.6）
	_ = alerts.Create(context.Background(), &model.Alert{
		SourceID: 1, Status: "firing", RawPayload: json.RawMessage(sampleWebhookJSON), ReceivedAt: time.Now(),
	})
	orig := &model.Delivery{AlertID: 1, ChannelID: 7, TriggerType: "auto", Status: "failed", SentAt: time.Now()}
	_ = deliveries.Create(context.Background(), orig)
	if _, err := deliverer.Resend(context.Background(), orig.ID); !errors.Is(err, ErrChannelDeleted) {
		t.Fatalf("deleted channel: %v", err)
	}
	if sender.calls != 0 {
		t.Error("refused resend must not call the sender")
	}
	if len(deliveries.all()) != 1 {
		t.Error("refused resend must not create a new row")
	}
}
