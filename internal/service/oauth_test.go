package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yongheng0927/fenghuo/internal/model"
	fafjwt "github.com/yongheng0927/fenghuo/internal/pkg/jwt"
)

// fakeOAuthProvider 返回预设的用户信息
type fakeOAuthProvider struct {
	profile *OAuthProfile
	err     error
}

func (f *fakeOAuthProvider) Name() string { return "feishu" }
func (f *fakeOAuthProvider) AuthURL(redirectURI, state string) string {
	return "https://open.feishu.cn/authorize?redirect_uri=" + redirectURI + "&state=" + state
}
func (f *fakeOAuthProvider) Exchange(context.Context, string) (*OAuthProfile, error) {
	return f.profile, f.err
}

func newOAuthTestEnv(profile *OAuthProfile, autoCreate bool, allowed []string) (*OAuthService, *fakeUserStore, *fakeOAuthIdentityStore) {
	users := newFakeUserStore()
	identities := newFakeOAuthIdentityStore()
	tokens := newFakeTokenStore()
	jwtMgr := fafjwt.NewManager([]byte("test-secret-0123456789abcdef0123"), 2*time.Hour, 7*24*time.Hour)
	auth := NewAuthService(users, tokens, jwtMgr)
	svc := NewOAuthService(&fakeOAuthProvider{profile: profile}, users, identities, auth, autoCreate, allowed)
	return svc, users, identities
}

var testProfile = &OAuthProfile{
	OpenID: "ou_abc123", UnionID: "on_xyz", Name: "张伟",
	Email: "zhangwei@example.com", AvatarURL: "https://avatar.example.com/zw.png",
}

func TestOAuthLoginBoundIdentity(t *testing.T) {
	svc, users, identities := newOAuthTestEnv(testProfile, false, nil)
	u := users.addUser("zhangwei", "", model.RoleEditor, true)
	u.Name = "Old Name"
	_ = identities.Create(context.Background(), &model.OAuthIdentity{
		UserID: u.ID, Provider: "feishu", ProviderUserID: "ou_abc123",
	})

	user, pair, err := svc.LoginWithCode(context.Background(), "code-1")
	if err != nil {
		t.Fatalf("LoginWithCode: %v", err)
	}
	if user.ID != u.ID {
		t.Fatalf("user id = %d, want %d", user.ID, u.ID)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("token pair must be issued")
	}
	// 资料字段已从 provider 同步
	if u.Name != "张伟" || u.Email != "zhangwei@example.com" || u.AvatarURL == "" {
		t.Errorf("profile not synced: %+v", u)
	}
}

func TestOAuthAutoCreateUser(t *testing.T) {
	svc, users, identities := newOAuthTestEnv(testProfile, true, nil)

	user, pair, err := svc.LoginWithCode(context.Background(), "code-1")
	if err != nil {
		t.Fatalf("LoginWithCode: %v", err)
	}
	if user.Role != model.RoleViewer {
		t.Errorf("auto-created user must be viewer, got %q", user.Role)
	}
	if user.PasswordHash != nil {
		t.Error("auto-created user must have no local password")
	}
	if user.Username != "ou_abc123" {
		t.Errorf("username = %q, want open_id", user.Username)
	}
	if pair.AccessToken == "" {
		t.Error("token pair must be issued")
	}
	// OAuth 登录也要记录最近登录时间
	stored, _ := users.FindByID(context.Background(), user.ID)
	if stored.LastLoginAt == nil {
		t.Error("oauth login must update last_login_at")
	}
	// 身份已绑定，第二次登录解析到同一个用户
	iden, _ := identities.FindByProviderUserID(context.Background(), "feishu", "ou_abc123")
	if iden == nil || iden.UserID != user.ID {
		t.Fatal("identity must be bound")
	}
	user2, _, err := svc.LoginWithCode(context.Background(), "code-2")
	if err != nil || user2.ID != user.ID {
		t.Fatalf("second login must resolve the same user: %v", err)
	}
	if n, _ := users.Count(context.Background()); n != 1 {
		t.Fatalf("users = %d, want 1", n)
	}
}

func TestOAuthAutoCreateUsernameUniqueness(t *testing.T) {
	svc, users, _ := newOAuthTestEnv(testProfile, true, nil)
	users.addUser("ou_abc123", "pw-123456", model.RoleAdmin, true) // open_id 被历史数据占用

	user, err := func() (*model.User, error) {
		u, _, err := svc.LoginWithCode(context.Background(), "code-1")
		return u, err
	}()
	if err != nil {
		t.Fatalf("LoginWithCode: %v", err)
	}
	if user.Username != "ou_abc123-2" {
		t.Errorf("username = %q, want open_id + 递增后缀", user.Username)
	}
}

func TestOAuthAutoCreateDisabled(t *testing.T) {
	svc, _, _ := newOAuthTestEnv(testProfile, false, nil)
	if _, _, err := svc.LoginWithCode(context.Background(), "code-1"); !errors.Is(err, ErrOAuthNotBound) {
		t.Fatalf("err = %v, want ErrOAuthNotBound", err)
	}
}

func TestOAuthEmailWhitelist(t *testing.T) {
	svc, _, _ := newOAuthTestEnv(testProfile, true, []string{"other@example.com"})
	if _, _, err := svc.LoginWithCode(context.Background(), "c"); !errors.Is(err, ErrOAuthEmailNotAllowed) {
		t.Fatalf("err = %v, want ErrOAuthEmailNotAllowed", err)
	}

	svc2, _, _ := newOAuthTestEnv(testProfile, true, []string{"zhangwei@example.com"})
	if _, _, err := svc2.LoginWithCode(context.Background(), "c"); err != nil {
		t.Fatalf("whitelisted email must pass: %v", err)
	}
}

func TestOAuthDisabledAccountRejected(t *testing.T) {
	svc, users, identities := newOAuthTestEnv(testProfile, false, nil)
	u := users.addUser("zhangwei", "", model.RoleViewer, false)
	_ = identities.Create(context.Background(), &model.OAuthIdentity{
		UserID: u.ID, Provider: "feishu", ProviderUserID: "ou_abc123",
	})
	if _, _, err := svc.LoginWithCode(context.Background(), "c"); !errors.Is(err, ErrAccountDisabled) {
		t.Fatalf("err = %v, want ErrAccountDisabled", err)
	}
}

func TestOAuthStateStore(t *testing.T) {
	s := NewOAuthStateStore(10 * time.Minute)
	now := time.Now()
	s.now = func() time.Time { return now }

	state, err := s.Issue()
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if len(state) != 32 {
		t.Fatalf("state length = %d, want 32 hex chars", len(state))
	}
	if !s.Consume(state) {
		t.Fatal("fresh state must validate")
	}
	if s.Consume(state) {
		t.Fatal("state must be one-time use")
	}
	if s.Consume("never-issued") {
		t.Fatal("unknown state must fail")
	}

	state2, _ := s.Issue()
	now = now.Add(11 * time.Minute)
	if s.Consume(state2) {
		t.Fatal("expired state must fail")
	}
}
