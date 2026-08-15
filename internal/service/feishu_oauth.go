package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// feishuDefaultBaseURL 是飞书开放平台的接口根地址
const feishuDefaultBaseURL = "https://open.feishu.cn"

// FeishuProvider 实现飞书 OAuth2 的 OAuthProvider（FR-5.3） BaseURL 可
// 覆盖，便于测试
type FeishuProvider struct {
	AppID     string
	AppSecret string
	BaseURL   string
	Client    *http.Client
}

// NewFeishuProvider 创建使用生产端点的 FeishuProvider
func NewFeishuProvider(appID, appSecret string) *FeishuProvider {
	return &FeishuProvider{
		AppID:     appID,
		AppSecret: appSecret,
		BaseURL:   feishuDefaultBaseURL,
		Client:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (p *FeishuProvider) Name() string { return "feishu" }

// AuthURL 构造飞书授权页 URL
func (p *FeishuProvider) AuthURL(redirectURI, state string) string {
	q := url.Values{
		"app_id":       {p.AppID},
		"redirect_uri": {redirectURI},
		"state":        {state},
	}
	return p.base() + "/open-apis/authen/v1/authorize?" + q.Encode()
}

func (p *FeishuProvider) base() string {
	if p.BaseURL != "" {
		return p.BaseURL
	}
	return feishuDefaultBaseURL
}

func (p *FeishuProvider) client() *http.Client {
	if p.Client != nil {
		return p.Client
	}
	return &http.Client{Timeout: 10 * time.Second}
}

// Exchange 实现 FR-5.3 的流程：code -> app_access_token ->
// user_access_token -> 用户信息
func (p *FeishuProvider) Exchange(ctx context.Context, code string) (*OAuthProfile, error) {
	appToken, err := p.appAccessToken(ctx)
	if err != nil {
		return nil, err
	}
	userToken, err := p.userAccessToken(ctx, appToken, code)
	if err != nil {
		return nil, err
	}
	return p.userInfo(ctx, userToken)
}

// oauthResponse 是 OAuth 相关接口的飞书通用响应包络
type oauthResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// feishuError 用非零的飞书响应 code 构造错误
func feishuError(step string, code int, msg string) error {
	return fmt.Errorf("feishu %s failed (code %d): %s", step, code, msg)
}

func (p *FeishuProvider) post(ctx context.Context, path string, body any, bearer string, out any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.base()+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	return p.do(req, out)
}

func (p *FeishuProvider) get(ctx context.Context, path, bearer string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.base()+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	return p.do(req, out)
}

func (p *FeishuProvider) do(req *http.Request, out any) error {
	resp, err := p.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode feishu response: %w", err)
	}
	return nil
}

func (p *FeishuProvider) appAccessToken(ctx context.Context) (string, error) {
	var out struct {
		oauthResponse
		AppAccessToken string `json:"app_access_token"`
	}
	err := p.post(ctx, "/open-apis/auth/v3/app_access_token/internal", map[string]string{
		"app_id":     p.AppID,
		"app_secret": p.AppSecret,
	}, "", &out)
	if err != nil {
		return "", err
	}
	if out.Code != 0 {
		return "", feishuError("app_access_token", out.Code, out.Msg)
	}
	return out.AppAccessToken, nil
}

func (p *FeishuProvider) userAccessToken(ctx context.Context, appToken, code string) (string, error) {
	var out struct {
		oauthResponse
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	err := p.post(ctx, "/open-apis/authen/v1/oidc/access_token", map[string]string{
		"grant_type": "authorization_code",
		"code":       code,
	}, appToken, &out)
	if err != nil {
		return "", err
	}
	if out.Code != 0 {
		return "", feishuError("user_access_token", out.Code, out.Msg)
	}
	if out.Data.AccessToken == "" {
		return "", feishuError("user_access_token", out.Code, "empty access_token")
	}
	return out.Data.AccessToken, nil
}

func (p *FeishuProvider) userInfo(ctx context.Context, userToken string) (*OAuthProfile, error) {
	var out struct {
		oauthResponse
		Data struct {
			Name      string `json:"name"`
			Email     string `json:"email"`
			AvatarURL string `json:"avatar_url"`
			OpenID    string `json:"open_id"`
			UnionID   string `json:"union_id"`
		} `json:"data"`
	}
	if err := p.get(ctx, "/open-apis/authen/v1/user_info", userToken, &out); err != nil {
		return nil, err
	}
	if out.Code != 0 {
		return nil, feishuError("user_info", out.Code, out.Msg)
	}
	return &OAuthProfile{
		OpenID:    out.Data.OpenID,
		UnionID:   out.Data.UnionID,
		Name:      out.Data.Name,
		Email:     out.Data.Email,
		AvatarURL: out.Data.AvatarURL,
	}, nil
}
