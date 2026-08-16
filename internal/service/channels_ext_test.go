package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/yongheng0927/fenghuo/internal/model"
)

// TestDingTalkSignKnownAnswer 用独立的 openssl 计算向量验证钉钉加签算法：
//
//	printf '1600000000000\nSECtest-secret' | openssl dgst -sha256 -hmac 'SECtest-secret' -binary | base64
func TestDingTalkSignKnownAnswer(t *testing.T) {
	got := DingTalkSign("SECtest-secret", 1600000000000)
	want := "irKW0sQ9ABbA1olc/Xz1M0nZkBhRwGQN4/D2ERsZQD8="
	if got != want {
		t.Fatalf("DingTalkSign = %q, want %q", got, want)
	}
}

func TestDingTalkSendSuccessWithSign(t *testing.T) {
	cipher := testCipher(t)
	var gotBody map[string]any
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	defer srv.Close()

	secret := encrypt(t, cipher, "SECabc")
	ch := &model.Channel{
		WebhookURLEncrypted: encrypt(t, cipher, srv.URL+"/robot/send?access_token=token123"),
		SecretEncrypted:     &secret,
	}
	sender := NewDingTalkSender(cipher, 5*time.Second)
	res := sender.Send(context.Background(), ch,
		[]byte(`{"msgtype":"markdown","markdown":{"title":"[FIRING] HighCPU","text":"**告警** 内容"}}`))
	if !res.Success() {
		t.Fatalf("result = %+v, want success", res)
	}
	// 加签参数必须拼在 webhook URL 上且数值正确
	ts, sign := gotQuery.Get("timestamp"), gotQuery.Get("sign")
	if ts == "" || sign == "" {
		t.Fatalf("missing timestamp/sign: %v", gotQuery)
	}
	if gotQuery.Get("access_token") != "token123" {
		t.Fatalf("original query must be preserved: %v", gotQuery)
	}
	tsInt, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		t.Fatalf("timestamp must be milliseconds integer: %q", ts)
	}
	if DingTalkSign("SECabc", tsInt) != sign {
		t.Fatal("sign must match the official algorithm for the sent timestamp")
	}
	// 消息体原样透传
	md := gotBody["markdown"].(map[string]any)
	if md["title"] != "[FIRING] HighCPU" || md["text"] != "**告警** 内容" {
		t.Fatalf("markdown = %v", md)
	}
	if _, ok := gotBody["at"]; ok {
		t.Fatalf("at must not be injected when AtAll is off: %v", gotBody["at"])
	}
}

// TestDingTalkSendAtAll 渠道的 AtAll 打开时，Sender 往消息体合并
// at.isAtAll（覆盖模板自带的 at 字段）
func TestDingTalkSendAtAll(t *testing.T) {
	cipher := testCipher(t)
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	defer srv.Close()

	ch := &model.Channel{WebhookURLEncrypted: encrypt(t, cipher, srv.URL), AtAll: true}
	res := NewDingTalkSender(cipher, 5*time.Second).
		Send(context.Background(), ch, []byte(`{"msgtype":"text","text":{"content":"hi"}}`))
	if !res.Success() {
		t.Fatalf("result = %+v, want success", res)
	}
	if gotBody["at"].(map[string]any)["isAtAll"] != true {
		t.Fatalf("at = %v", gotBody["at"])
	}
}

func TestDingTalkSendBusinessError(t *testing.T) {
	cipher := testCipher(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"errcode":310000,"errmsg":"keywords not in content"}`))
	}))
	defer srv.Close()
	ch := &model.Channel{WebhookURLEncrypted: encrypt(t, cipher, srv.URL)}
	res := NewDingTalkSender(cipher, 5*time.Second).
		Send(context.Background(), ch, []byte(`{"msgtype":"text","text":{"content":"hi"}}`))
	if res.Success() || res.Retryable() {
		t.Fatalf("business error must be a non-retryable failure: %+v", res)
	}
	if res.Code != dingTalkCodeRejected {
		t.Fatalf("code = %d", res.Code)
	}
	if !strings.Contains(res.Message(), "dingtalk") {
		t.Fatalf("message should contain the human-readable hint: %q", res.Message())
	}
}

func TestDingTalkRateLimitRetryable(t *testing.T) {
	res := SendResult{HTTPStatus: 200, Code: dingTalkCodeRateLimit, Msg: "send too fast"}
	if !res.Retryable() {
		t.Fatal("dingtalk rate limit must be retryable")
	}
	wres := SendResult{HTTPStatus: 200, Code: weComCodeRateLimit, Msg: "freq limit"}
	if !wres.Retryable() {
		t.Fatal("wecom rate limit must be retryable")
	}
}

func TestWeComSendSuccess(t *testing.T) {
	cipher := testCipher(t)
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "" {
			t.Errorf("wecom send must not add sign params: %q", r.URL.RawQuery)
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	defer srv.Close()

	// 企微不支持 @人：AtAll 打开时消息体也必须原样透传
	ch := &model.Channel{WebhookURLEncrypted: encrypt(t, cipher, srv.URL), AtAll: true}
	res := NewWeComSender(cipher, 5*time.Second).
		Send(context.Background(), ch, []byte(`{"msgtype":"markdown","markdown":{"content":"**告警**"}}`))
	if !res.Success() {
		t.Fatalf("result = %+v, want success", res)
	}
	if gotBody["msgtype"] != "markdown" {
		t.Fatalf("msgtype = %v", gotBody["msgtype"])
	}
	if gotBody["markdown"].(map[string]any)["content"] != "**告警**" {
		t.Fatalf("markdown = %v", gotBody["markdown"])
	}
}

// TestFeishuSendAtAllText AtAll 打开时，text 消息的 content.text 末尾追加
// @所有人标记
func TestFeishuSendAtAllText(t *testing.T) {
	cipher := testCipher(t)
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		_, _ = w.Write([]byte(`{"code":0,"msg":"success"}`))
	}))
	defer srv.Close()

	ch := &model.Channel{WebhookURLEncrypted: encrypt(t, cipher, srv.URL), AtAll: true}
	res := NewFeishuSender(cipher, 5*time.Second).
		Send(context.Background(), ch, []byte(`{"msg_type":"text","content":{"text":"[FIRING] HighCPU"}}`))
	if !res.Success() {
		t.Fatalf("result = %+v, want success", res)
	}
	text := gotBody["content"].(map[string]any)["text"].(string)
	if !strings.HasSuffix(text, "\n"+feishuAtAllText) {
		t.Fatalf("text = %q", text)
	}
}

// TestFeishuSendAtAllCard AtAll 打开时，互动卡片 elements 末尾追加一个
// lark_md div；加签与注入在同一次改写中完成
func TestFeishuSendAtAllCardWithSign(t *testing.T) {
	cipher := testCipher(t)
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		_, _ = w.Write([]byte(`{"code":0,"msg":"success"}`))
	}))
	defer srv.Close()

	secret := encrypt(t, cipher, "sign-secret")
	ch := &model.Channel{
		WebhookURLEncrypted: encrypt(t, cipher, srv.URL),
		SecretEncrypted:     &secret,
		AtAll:               true,
	}
	payload := `{"msg_type":"interactive","card":{"header":{"template":"red","title":{"tag":"plain_text","content":"t"}},"elements":[{"tag":"div","text":{"tag":"lark_md","content":"body"}}]}}`
	res := NewFeishuSender(cipher, 5*time.Second).Send(context.Background(), ch, []byte(payload))
	if !res.Success() {
		t.Fatalf("result = %+v, want success", res)
	}
	elements := gotBody["card"].(map[string]any)["elements"].([]any)
	if len(elements) != 2 {
		t.Fatalf("elements = %v", elements)
	}
	last := elements[1].(map[string]any)["text"].(map[string]any)
	if last["tag"] != "lark_md" || last["content"] != feishuAtAllText {
		t.Fatalf("at-all element = %v", last)
	}
	// 签名也在同一消息体上
	ts, _ := gotBody["timestamp"].(string)
	sign, _ := gotBody["sign"].(string)
	tsInt, err := strconv.ParseInt(ts, 10, 64)
	if err != nil || Sign("sign-secret", tsInt) != sign {
		t.Fatalf("sign mismatch: timestamp=%q sign=%q", ts, sign)
	}
}

// TestFeishuSendAtAllOtherTypes post/image 等类型没有通用的注入位置，
// 消息体原样透传
func TestFeishuSendAtAllOtherTypes(t *testing.T) {
	cipher := testCipher(t)
	var gotRaw []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRaw, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"code":0,"msg":"success"}`))
	}))
	defer srv.Close()

	ch := &model.Channel{WebhookURLEncrypted: encrypt(t, cipher, srv.URL), AtAll: true}
	payload := `{"msg_type":"image","content":{"image_key":"img_x"}}`
	res := NewFeishuSender(cipher, 5*time.Second).Send(context.Background(), ch, []byte(payload))
	if !res.Success() {
		t.Fatalf("result = %+v, want success", res)
	}
	var m map[string]any
	if err := json.Unmarshal(gotRaw, &m); err != nil {
		t.Fatalf("sent body must be valid JSON: %v", err)
	}
	if m["msg_type"] != "image" || m["content"].(map[string]any)["image_key"] != "img_x" {
		t.Fatalf("image payload must pass through untouched: %s", gotRaw)
	}
}

func TestDispatcherDispatch(t *testing.T) {
	calls := map[string]int{}
	mk := func(name string) Sender {
		return SenderFunc(func(_ context.Context, _ *model.Channel, _ []byte) SendResult {
			calls[name]++
			return okResult()
		})
	}
	d := NewDispatcher(mk("feishu"), mk("dingtalk"), mk("wecom"))
	for _, typ := range []string{"feishu", "dingtalk", "wecom"} {
		res := d.Send(context.Background(), &model.Channel{Type: typ}, nil)
		if !res.Success() {
			t.Fatalf("%s: %+v", typ, res)
		}
		if calls[typ] != 1 {
			t.Fatalf("%s: calls = %v", typ, calls)
		}
	}
	res := d.Send(context.Background(), &model.Channel{Type: "slack"}, nil)
	if res.Success() || res.Retryable() {
		t.Fatalf("unknown type must be a non-retryable local failure: %+v", res)
	}
}

// SenderFunc 让函数适配 Sender 接口（测试用）
type SenderFunc func(ctx context.Context, ch *model.Channel, payload []byte) SendResult

func (f SenderFunc) Send(ctx context.Context, ch *model.Channel, payload []byte) SendResult {
	return f(ctx, ch, payload)
}
