package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// feishuMockServer 模拟飞书开放平台的各端点
func feishuMockServer(t *testing.T, failAt string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/open-apis/auth/v3/app_access_token/internal", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["app_id"] != "cli_test" || body["app_secret"] != "secret_test" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if failAt == "app_access_token" {
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 99991663, "msg": "app access token invalid"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "app_access_token": "t-app-token", "expire": 7200})
	})
	mux.HandleFunc("/open-apis/authen/v1/oidc/access_token", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer t-app-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["grant_type"] != "authorization_code" || body["code"] == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": map[string]any{"access_token": "u-user-token", "token_type": "Bearer"},
		})
	})
	mux.HandleFunc("/open-apis/authen/v1/user_info", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer u-user-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": map[string]any{
				"name": "张伟", "email": "zhangwei@example.com",
				"avatar_url": "https://avatar.example.com/zw.png",
				"open_id":    "ou_abc123", "union_id": "on_xyz",
			},
		})
	})
	return httptest.NewServer(mux)
}

func TestFeishuProviderExchange(t *testing.T) {
	srv := feishuMockServer(t, "")
	defer srv.Close()
	p := NewFeishuProvider("cli_test", "secret_test")
	p.BaseURL = srv.URL

	profile, err := p.Exchange(context.Background(), "code-123")
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if profile.OpenID != "ou_abc123" || profile.UnionID != "on_xyz" {
		t.Errorf("profile = %+v", profile)
	}
	if profile.Name != "张伟" || profile.Email != "zhangwei@example.com" {
		t.Errorf("profile = %+v", profile)
	}
	if profile.AvatarURL == "" {
		t.Error("avatar_url must be mapped")
	}
}

func TestFeishuProviderAuthURL(t *testing.T) {
	p := NewFeishuProvider("cli_test", "secret_test")
	u := p.AuthURL("https://alerts.example.com/faf/api/auth/oauth/feishu/callback", "state-1")
	if !strings.HasPrefix(u, "https://open.feishu.cn/open-apis/authen/v1/authorize?") {
		t.Fatalf("auth url = %q", u)
	}
	for _, want := range []string{"app_id=cli_test", "state=state-1", "redirect_uri="} {
		if !strings.Contains(u, want) {
			t.Errorf("auth url %q missing %q", u, want)
		}
	}
	// 子路径回调地址必须经得起 URL 编码往返
	if !strings.Contains(u, "redirect_uri=https%3A%2F%2Falerts.example.com%2Ffaf%2Fapi%2Fauth%2Foauth%2Ffeishu%2Fcallback") {
		t.Errorf("redirect_uri not encoded correctly: %q", u)
	}
}

func TestFeishuProviderError(t *testing.T) {
	srv := feishuMockServer(t, "app_access_token")
	defer srv.Close()
	p := NewFeishuProvider("cli_test", "secret_test")
	p.BaseURL = srv.URL
	_, err := p.Exchange(context.Background(), "code-123")
	if err == nil || !strings.Contains(err.Error(), "app_access_token") {
		t.Fatalf("err = %v, want app_access_token failure", err)
	}
}
