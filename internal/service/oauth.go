package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/yongheng0927/FreeAlertFlow/internal/model"
)

// OAuth 用户开通（provisioning）相关错误（FR-5.3）
var (
	ErrOAuthEmailNotAllowed = errors.New("email is not in the allowed list")
	ErrOAuthNotBound        = errors.New("no local account is bound to this Feishu identity")
	ErrOAuthDisabled        = errors.New("oauth login is not enabled")
)

// OAuthProfile 是 provider 返回的归一化用户信息
type OAuthProfile struct {
	OpenID    string
	UnionID   string
	Name      string
	Email     string
	AvatarURL string
}

// OAuthProvider 抽象第三方登录 provider（V1 仅飞书，接口按 FR-5.3 预留
// 扩展）
type OAuthProvider interface {
	Name() string // 例如 "feishu"
	// AuthURL 构造 provider 授权页 URL
	AuthURL(redirectURI, state string) string
	// Exchange 用回调授权码换取用户信息
	Exchange(ctx context.Context, code string) (*OAuthProfile, error)
}

// OAuthStateStore 签发和校验带 TTL 的一次性 CSRF state
type OAuthStateStore struct {
	mu      sync.Mutex
	ttl     time.Duration
	entries map[string]time.Time
	now     func() time.Time // 可注入，便于测试
}

// NewOAuthStateStore 创建带指定 state TTL 的 store
func NewOAuthStateStore(ttl time.Duration) *OAuthStateStore {
	return &OAuthStateStore{ttl: ttl, entries: map[string]time.Time{}, now: time.Now}
}

// Issue 生成一个新的随机 state，TTL 内有效
func (s *OAuthStateStore) Issue() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	state := hex.EncodeToString(b)
	s.mu.Lock()
	defer s.mu.Unlock()
	// 顺带清理已过期的 state
	for k, exp := range s.entries {
		if s.now().After(exp) {
			delete(s.entries, k)
		}
	}
	s.entries[state] = s.now().Add(s.ttl)
	return state, nil
}

// Consume 校验 state 并删除（一次性使用）
func (s *OAuthStateStore) Consume(state string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, ok := s.entries[state]
	if !ok {
		return false
	}
	delete(s.entries, state)
	return s.now().Before(exp)
}

// OAuthService 把 provider 身份绑定到本地用户（FR-5.3）
type OAuthService struct {
	provider       OAuthProvider
	users          UserStore
	identities     OAuthIdentityStore
	auth           *AuthService
	autoCreateUser bool
	allowedEmails  map[string]struct{} // 空 = 不限制
}

// NewOAuthService 创建 OAuthService
func NewOAuthService(provider OAuthProvider, users UserStore, identities OAuthIdentityStore,
	auth *AuthService, autoCreateUser bool, allowedEmails []string) *OAuthService {
	s := &OAuthService{
		provider:       provider,
		users:          users,
		identities:     identities,
		auth:           auth,
		autoCreateUser: autoCreateUser,
		allowedEmails:  map[string]struct{}{},
	}
	for _, e := range allowedEmails {
		e = strings.ToLower(strings.TrimSpace(e))
		if e != "" {
			s.allowedEmails[e] = struct{}{}
		}
	}
	return s
}

// Provider 返回配置的 provider
func (s *OAuthService) Provider() OAuthProvider { return s.provider }

// LoginWithCode 完成一次 OAuth 登录：用 code 换用户信息、应用邮箱白名单、
// 解析或创建本地用户，并签发 token 对
func (s *OAuthService) LoginWithCode(ctx context.Context, code string) (*model.User, *TokenPair, error) {
	profile, err := s.provider.Exchange(ctx, code)
	if err != nil {
		return nil, nil, err
	}
	if profile.OpenID == "" {
		return nil, nil, errors.New("oauth provider returned no open_id")
	}
	if len(s.allowedEmails) > 0 {
		if _, ok := s.allowedEmails[strings.ToLower(profile.Email)]; !ok {
			return nil, nil, ErrOAuthEmailNotAllowed
		}
	}

	identity, err := s.identities.FindByProviderUserID(ctx, s.provider.Name(), profile.OpenID)
	if err != nil {
		return nil, nil, err
	}
	var user *model.User
	if identity != nil {
		user, err = s.users.FindByID(ctx, identity.UserID)
		if err != nil {
			return nil, nil, err
		}
		if user == nil {
			return nil, nil, ErrOAuthNotBound
		}
		// 同步 provider 侧可能变更的资料字段
		if user.Name != profile.Name || user.Email != profile.Email || user.AvatarURL != profile.AvatarURL {
			if err := s.users.UpdateProfile(ctx, user.ID, profile.Name, profile.Email, profile.AvatarURL); err != nil {
				return nil, nil, err
			}
			user.Name, user.Email, user.AvatarURL = profile.Name, profile.Email, profile.AvatarURL
		}
	} else {
		if !s.autoCreateUser {
			return nil, nil, ErrOAuthNotBound
		}
		user, err = s.provisionUser(ctx, profile)
		if err != nil {
			return nil, nil, err
		}
	}
	if !user.Enabled {
		return nil, nil, ErrAccountDisabled
	}
	pair, err := s.auth.IssueTokens(ctx, user)
	if err != nil {
		return nil, nil, err
	}
	return user, pair, nil
}

// provisionUser 自动创建一个绑定该身份的本地 viewer 账号
func (s *OAuthService) provisionUser(ctx context.Context, p *OAuthProfile) (*model.User, error) {
	username, err := s.uniqueUsername(ctx, p)
	if err != nil {
		return nil, err
	}
	user := &model.User{
		Username:  username, // password_hash 保持 NULL：纯 OAuth 账号
		Name:      p.Name,
		Email:     p.Email,
		AvatarURL: p.AvatarURL,
		Role:      model.RoleViewer, // 自动创建的用户默认 viewer（FR-5.4）
		Enabled:   true,
	}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, err
	}
	identity := &model.OAuthIdentity{
		UserID:          user.ID,
		Provider:        s.provider.Name(),
		ProviderUserID:  p.OpenID,
		ProviderUnionID: p.UnionID,
	}
	if err := s.identities.Create(ctx, identity); err != nil {
		return nil, err
	}
	return user, nil
}

// uniqueUsername 从用户资料推导出唯一登录名
func (s *OAuthService) uniqueUsername(ctx context.Context, p *OAuthProfile) (string, error) {
	base := strings.TrimSpace(p.Name)
	if base == "" {
		base = "feishu"
	}
	if len([]rune(base)) > 50 {
		base = string([]rune(base)[:50])
	}
	candidate := base
	for i := 0; ; i++ {
		existing, err := s.users.FindByUsername(ctx, candidate)
		if err != nil {
			return "", err
		}
		if existing == nil {
			return candidate, nil
		}
		suffix := p.OpenID
		if len(suffix) > 6 {
			suffix = suffix[len(suffix)-6:]
		}
		if i == 0 {
			candidate = fmt.Sprintf("%s-%s", base, suffix)
		} else {
			candidate = fmt.Sprintf("%s-%s-%d", base, suffix, i+1)
		}
	}
}
