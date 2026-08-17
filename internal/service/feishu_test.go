package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/yongheng0927/fenghuo/internal/model"
)

// TestSignKnownAnswer 用一个独立的 openssl 计算向量验证 FR-2.2 的签名算法：
//
//	printf '' | openssl dgst -sha256 -hmac $'1599360473\ntest-secret' -binary | base64
func TestSignKnownAnswer(t *testing.T) {
	got := Sign("test-secret", 1599360473)
	want := "wSds2BzzFIIGf/WrhUO+NI1q/9j+FRJd3JNHKAq0NZY="
	if got != want {
		t.Fatalf("Sign = %q, want %q", got, want)
	}
}

func TestRetryableClassification(t *testing.T) {
	cases := []struct {
		name string
		res  SendResult
		want bool
	}{
		{"success", SendResult{HTTPStatus: 200, Code: 0, Msg: "success"}, false},
		{"network error", SendResult{Err: errors.New("connection refused")}, true},
		{"timeout", SendResult{Err: context.DeadlineExceeded}, true},
		{"http 500", SendResult{HTTPStatus: 500, Code: -1}, true},
		{"http 502", SendResult{HTTPStatus: 502, Code: -1}, true},
		{"http 429", SendResult{HTTPStatus: 429, Code: -1}, true},
		{"feishu rate limit", SendResult{HTTPStatus: 200, Code: feishuCodeRateLimit}, true},
		{"sign error", SendResult{HTTPStatus: 200, Code: feishuCodeSignFailed}, false},
		{"keyword missing", SendResult{HTTPStatus: 200, Code: feishuCodeKeywordMiss}, false},
		{"invalid token", SendResult{HTTPStatus: 200, Code: feishuCodeTokenInvalid}, false},
		{"ip not allowed", SendResult{HTTPStatus: 200, Code: feishuCodeIPNotAllowed}, false},
		{"local error", SendResult{Code: -1, Msg: "render error"}, false},
	}
	for _, tc := range cases {
		if got := tc.res.Retryable(); got != tc.want {
			t.Errorf("%s: Retryable = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestFeishuSendSuccessWithSign(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"msg":"success"}`))
	}))
	defer srv.Close()

	ch := &model.Channel{WebhookURL: srv.URL, Secret: "my-secret"}
	sender := NewFeishuSender(5 * time.Second)
	res := sender.Send(context.Background(), ch, []byte(`{"msg_type":"text","content":{"text":"hi"}}`))
	if !res.Success() {
		t.Fatalf("result = %+v, want success", res)
	}
	if res.Code != 0 || res.HTTPStatus != 200 {
		t.Fatalf("result = %+v", res)
	}
	// 请求体中必须注入了签名字段
	ts, _ := gotBody["timestamp"].(string)
	sign, _ := gotBody["sign"].(string)
	if ts == "" || sign == "" {
		t.Fatalf("signed body must carry timestamp and sign: %v", gotBody)
	}
	tsInt, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		t.Fatalf("timestamp must be numeric string: %q", ts)
	}
	if Sign("my-secret", tsInt) != sign {
		t.Fatal("sign must match the official algorithm for the sent timestamp")
	}
	if gotBody["msg_type"] != "text" {
		t.Fatalf("payload fields must be preserved: %v", gotBody)
	}
}

func TestFeishuSendLegacyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"Extra":null,"StatusCode":0,"StatusMessage":"success"}`))
	}))
	defer srv.Close()
	ch := &model.Channel{WebhookURL: srv.URL}
	res := NewFeishuSender(5 * time.Second).
		Send(context.Background(), ch, []byte(`{"msg_type":"text","content":{"text":"hi"}}`))
	if !res.Success() {
		t.Fatalf("legacy response must parse as success: %+v", res)
	}
}

func TestFeishuSendBusinessError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":19021,"msg":"sign match fail or timestamp is not within one hour from current time"}`))
	}))
	defer srv.Close()
	ch := &model.Channel{WebhookURL: srv.URL}
	res := NewFeishuSender(5 * time.Second).
		Send(context.Background(), ch, []byte(`{"msg_type":"text","content":{"text":"hi"}}`))
	if res.Success() {
		t.Fatal("business error must not be success")
	}
	if res.Retryable() {
		t.Fatal("sign error must not be retryable")
	}
	if res.Code != 19021 {
		t.Fatalf("code = %d", res.Code)
	}
	msg := res.Message()
	if !strings.Contains(msg, "signature") {
		t.Fatalf("message should contain the human-readable hint: %q", msg)
	}
}

func TestFeishuSendNetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // 强制连接被拒
	ch := &model.Channel{WebhookURL: url}
	res := NewFeishuSender(time.Second).
		Send(context.Background(), ch, []byte(`{}`))
	if res.Err == nil {
		t.Fatal("expected transport error")
	}
	if !res.Retryable() {
		t.Fatal("transport error must be retryable")
	}
}
