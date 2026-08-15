package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yongheng0927/fenghuo/internal/model"
	fafjwt "github.com/yongheng0927/fenghuo/internal/pkg/jwt"
	"github.com/yongheng0927/fenghuo/internal/pkg/password"
)

// --- 实现 store 接口的内存 fake ---

type fakeUserStore struct {
	byID   map[int64]*model.User
	byName map[string]int64
	nextID int64
}

func newFakeUserStore() *fakeUserStore {
	return &fakeUserStore{byID: map[int64]*model.User{}, byName: map[string]int64{}, nextID: 1}
}

func (f *fakeUserStore) FindByUsername(_ context.Context, username string) (*model.User, error) {
	id, ok := f.byName[username]
	if !ok {
		return nil, nil
	}
	return f.byID[id], nil
}

func (f *fakeUserStore) FindByID(_ context.Context, id int64) (*model.User, error) {
	return f.byID[id], nil
}

func (f *fakeUserStore) Count(_ context.Context) (int64, error) {
	return int64(len(f.byID)), nil
}

func (f *fakeUserStore) Create(_ context.Context, u *model.User) error {
	u.ID = f.nextID
	f.nextID++
	f.byID[u.ID] = u
	f.byName[u.Username] = u.ID
	return nil
}

func (f *fakeUserStore) UpdateLastLogin(_ context.Context, id int64, t time.Time) error {
	f.byID[id].LastLoginAt = &t
	return nil
}

func (f *fakeUserStore) UpdatePassword(_ context.Context, id int64, hash string) error {
	f.byID[id].PasswordHash = &hash
	return nil
}

func (f *fakeUserStore) addUser(username, pw string, role string, enabled bool) *model.User {
	var hash *string
	if pw != "" {
		h, _ := password.Hash(pw)
		hash = &h
	}
	u := &model.User{Username: username, PasswordHash: hash, Role: role, Enabled: enabled}
	_ = f.Create(context.Background(), u)
	return u
}

type fakeTokenStore struct {
	byHash map[string]*model.RefreshToken
	nextID int64
}

func newFakeTokenStore() *fakeTokenStore {
	return &fakeTokenStore{byHash: map[string]*model.RefreshToken{}, nextID: 1}
}

func (f *fakeTokenStore) Create(_ context.Context, t *model.RefreshToken) error {
	t.ID = f.nextID
	f.nextID++
	f.byHash[t.TokenHash] = t
	return nil
}

func (f *fakeTokenStore) FindByHash(_ context.Context, hash string) (*model.RefreshToken, error) {
	return f.byHash[hash], nil
}

func (f *fakeTokenStore) Save(_ context.Context, t *model.RefreshToken) error {
	f.byHash[t.TokenHash] = t
	return nil
}

func (f *fakeTokenStore) RevokeByHash(_ context.Context, hash string) error {
	if t := f.byHash[hash]; t != nil {
		t.Revoked = true
	}
	return nil
}

func (f *fakeTokenStore) RevokeAllForUser(_ context.Context, userID int64) error {
	for _, t := range f.byHash {
		if t.UserID == userID {
			t.Revoked = true
		}
	}
	return nil
}

func (f *fakeTokenStore) allRevoked(userID int64) bool {
	n := 0
	for _, t := range f.byHash {
		if t.UserID == userID {
			n++
			if !t.Revoked {
				return false
			}
		}
	}
	return n > 0
}

// --- 辅助函数 ---

func newTestAuth() (*AuthService, *fakeUserStore, *fakeTokenStore) {
	users := newFakeUserStore()
	tokens := newFakeTokenStore()
	jwtMgr := fafjwt.NewManager([]byte("test-secret-0123456789abcdef0123"), 2*time.Hour, 7*24*time.Hour)
	return NewAuthService(users, tokens, jwtMgr), users, tokens
}

// --- 登录 ---

func TestLoginSuccess(t *testing.T) {
	svc, users, _ := newTestAuth()
	u := users.addUser("alice", "s3cret-pw", model.RoleAdmin, true)

	got, pair, err := svc.Login(context.Background(), "alice", "s3cret-pw")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if got.ID != u.ID {
		t.Fatalf("user id = %d, want %d", got.ID, u.ID)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("token pair must be non-empty")
	}
	if pair.ExpiresIn != int64((2 * time.Hour).Seconds()) {
		t.Fatalf("expires_in = %d", pair.ExpiresIn)
	}
	if u.LastLoginAt == nil {
		t.Fatal("last_login_at must be updated on login")
	}
}

func TestLoginWrongPassword(t *testing.T) {
	svc, users, _ := newTestAuth()
	users.addUser("alice", "s3cret-pw", model.RoleViewer, true)
	if _, _, err := svc.Login(context.Background(), "alice", "wrong"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("err = %v, want ErrInvalidCredentials", err)
	}
}

func TestLoginUnknownUser(t *testing.T) {
	svc, _, _ := newTestAuth()
	if _, _, err := svc.Login(context.Background(), "ghost", "x"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("err = %v, want ErrInvalidCredentials", err)
	}
}

func TestLoginOAuthOnlyUserRejected(t *testing.T) {
	svc, users, _ := newTestAuth()
	users.addUser("oauth-user", "", model.RoleViewer, true) // password_hash 为 NULL
	if _, _, err := svc.Login(context.Background(), "oauth-user", "anything"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("err = %v, want ErrInvalidCredentials", err)
	}
}

func TestLoginDisabledUser(t *testing.T) {
	svc, users, _ := newTestAuth()
	users.addUser("bob", "pw-123456", model.RoleViewer, false)
	if _, _, err := svc.Login(context.Background(), "bob", "pw-123456"); !errors.Is(err, ErrAccountDisabled) {
		t.Fatalf("err = %v, want ErrAccountDisabled", err)
	}
}

// --- refresh token 轮换 ---

func TestRefreshRotation(t *testing.T) {
	svc, users, tokens := newTestAuth()
	users.addUser("alice", "s3cret-pw", model.RoleEditor, true)
	_, pair, err := svc.Login(context.Background(), "alice", "s3cret-pw")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	pair2, err := svc.Refresh(context.Background(), pair.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if pair2.RefreshToken == pair.RefreshToken || pair2.AccessToken == "" {
		t.Fatal("rotation must issue a new token pair")
	}

	old := tokens.byHash[fafjwt.HashRefreshToken(pair.RefreshToken)]
	if old == nil || !old.Revoked {
		t.Fatal("old token must be revoked after rotation")
	}
	if old.ReplacedBy != fafjwt.HashRefreshToken(pair2.RefreshToken) {
		t.Fatal("replaced_by must point to the new token hash")
	}
}

func TestRefreshReuseRevokesAllSessions(t *testing.T) {
	svc, users, tokens := newTestAuth()
	u := users.addUser("alice", "s3cret-pw", model.RoleEditor, true)
	_, pair1, _ := svc.Login(context.Background(), "alice", "s3cret-pw")
	pair2, err := svc.Refresh(context.Background(), pair1.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	// 重放已轮换的第一个 token：必须被判定为重用，并吊销该用户的全部会话
	if _, err := svc.Refresh(context.Background(), pair1.RefreshToken); !errors.Is(err, ErrTokenReuse) {
		t.Fatalf("err = %v, want ErrTokenReuse", err)
	}
	if !tokens.allRevoked(u.ID) {
		t.Fatal("all refresh tokens of the user must be revoked on reuse")
	}

	// 轮换得到的新 token 此时也已被吊销，不能再用来刷新
	if _, err := svc.Refresh(context.Background(), pair2.RefreshToken); err == nil {
		t.Fatal("revoked token must not refresh")
	}
}

func TestRefreshUnknownToken(t *testing.T) {
	svc, _, _ := newTestAuth()
	if _, err := svc.Refresh(context.Background(), "no-such-token"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("err = %v, want ErrInvalidToken", err)
	}
}

func TestRefreshExpiredToken(t *testing.T) {
	svc, users, tokens := newTestAuth()
	users.addUser("alice", "s3cret-pw", model.RoleViewer, true)
	_, pair, _ := svc.Login(context.Background(), "alice", "s3cret-pw")

	// 把存储的 token 强制改成已过期
	rt := tokens.byHash[fafjwt.HashRefreshToken(pair.RefreshToken)]
	rt.ExpiresAt = time.Now().Add(-time.Minute)

	if _, err := svc.Refresh(context.Background(), pair.RefreshToken); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("err = %v, want ErrInvalidToken", err)
	}
}

// --- 登出 ---

func TestLogoutRevokesToken(t *testing.T) {
	svc, users, tokens := newTestAuth()
	users.addUser("alice", "s3cret-pw", model.RoleViewer, true)
	_, pair, _ := svc.Login(context.Background(), "alice", "s3cret-pw")

	if err := svc.Logout(context.Background(), pair.RefreshToken); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	rt := tokens.byHash[fafjwt.HashRefreshToken(pair.RefreshToken)]
	if !rt.Revoked {
		t.Fatal("token must be revoked after logout")
	}
	// 登出（而非轮换）的 token 再刷新得到 ErrInvalidToken，而不是 ErrTokenReuse
	if _, err := svc.Refresh(context.Background(), pair.RefreshToken); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("err = %v, want ErrInvalidToken", err)
	}
	// 登出对未知 token 是幂等的
	if err := svc.Logout(context.Background(), "unknown"); err != nil {
		t.Fatalf("Logout unknown: %v", err)
	}
}

// --- 修改密码 ---

func TestChangePasswordRevokesAllSessions(t *testing.T) {
	svc, users, tokens := newTestAuth()
	u := users.addUser("alice", "old-pw-123", model.RoleViewer, true)
	_, pair1, _ := svc.Login(context.Background(), "alice", "old-pw-123")
	_, pair2, _ := svc.Login(context.Background(), "alice", "old-pw-123")

	if err := svc.ChangePassword(context.Background(), u.ID, "old-pw-123", "new-pw-456"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if !password.Verify(*u.PasswordHash, "new-pw-456") {
		t.Fatal("password must be updated")
	}
	if !tokens.allRevoked(u.ID) {
		t.Fatal("all sessions must be revoked after password change")
	}
	// 旧密码不再可用，新密码可用
	if _, _, err := svc.Login(context.Background(), "alice", "old-pw-123"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("old password must fail: %v", err)
	}
	if _, _, err := svc.Login(context.Background(), "alice", "new-pw-456"); err != nil {
		t.Fatalf("new password must work: %v", err)
	}
	_ = pair1
	_ = pair2
}

func TestChangePasswordWrongOld(t *testing.T) {
	svc, users, _ := newTestAuth()
	u := users.addUser("alice", "old-pw-123", model.RoleViewer, true)
	if err := svc.ChangePassword(context.Background(), u.ID, "wrong", "new-pw-456"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("err = %v, want ErrInvalidCredentials", err)
	}
}

func TestChangePasswordOAuthOnlyUser(t *testing.T) {
	svc, users, _ := newTestAuth()
	u := users.addUser("oauth-user", "", model.RoleViewer, true)
	if err := svc.ChangePassword(context.Background(), u.ID, "x", "new-pw-456"); !errors.Is(err, ErrNoLocalPassword) {
		t.Fatalf("err = %v, want ErrNoLocalPassword", err)
	}
}

// --- bootstrap ---

func TestBootstrapAdminCreatesUserWhenEmpty(t *testing.T) {
	svc, users, _ := newTestAuth()
	created, err := svc.BootstrapAdmin(context.Background(), "admin", "admin-pw-123")
	if err != nil || !created {
		t.Fatalf("BootstrapAdmin = (%v, %v)", created, err)
	}
	admin, _ := users.FindByUsername(context.Background(), "admin")
	if admin == nil || admin.Role != model.RoleAdmin || !admin.Enabled {
		t.Fatal("bootstrap admin must exist with admin role")
	}
	if _, _, err := svc.Login(context.Background(), "admin", "admin-pw-123"); err != nil {
		t.Fatalf("bootstrap admin must be able to log in: %v", err)
	}
}

func TestBootstrapAdminNoOpWhenUsersExist(t *testing.T) {
	svc, users, _ := newTestAuth()
	users.addUser("alice", "pw-123456", model.RoleViewer, true)
	created, err := svc.BootstrapAdmin(context.Background(), "admin", "admin-pw-123")
	if err != nil || created {
		t.Fatalf("BootstrapAdmin = (%v, %v), want no-op", created, err)
	}
	if n, _ := users.Count(context.Background()); n != 1 {
		t.Fatalf("user count = %d, want 1", n)
	}
}

func TestBootstrapAdminNoOpWithoutCredentials(t *testing.T) {
	svc, _, _ := newTestAuth()
	created, err := svc.BootstrapAdmin(context.Background(), "", "")
	if err != nil || created {
		t.Fatalf("BootstrapAdmin = (%v, %v), want no-op", created, err)
	}
}

// --- 登录限流器 ---

func TestLoginLimiterLocksAfterMaxFailures(t *testing.T) {
	l := NewLoginLimiter(5, time.Minute, 10*time.Minute)
	now := time.Now()
	l.now = func() time.Time { return now }

	for i := 0; i < 4; i++ {
		l.Fail("1.2.3.4")
		if l.Locked("1.2.3.4") {
			t.Fatalf("locked after %d failures, want lock at 5", i+1)
		}
	}
	l.Fail("1.2.3.4")
	if !l.Locked("1.2.3.4") {
		t.Fatal("must be locked after 5 failures")
	}

	// 其他 IP 不受影响
	if l.Locked("5.6.7.8") {
		t.Fatal("unrelated IP must not be locked")
	}

	// 锁定在 lockTime 后过期
	now = now.Add(10*time.Minute + time.Second)
	if l.Locked("1.2.3.4") {
		t.Fatal("lock must expire after lockTime")
	}
}

func TestLoginLimiterWindowSlides(t *testing.T) {
	l := NewLoginLimiter(5, time.Minute, 10*time.Minute)
	now := time.Now()
	l.now = func() time.Time { return now }

	for i := 0; i < 4; i++ {
		l.Fail("1.2.3.4")
	}
	// 移出窗口：过期的失败记录不再计数
	now = now.Add(2 * time.Minute)
	l.Fail("1.2.3.4")
	if l.Locked("1.2.3.4") {
		t.Fatal("failures older than the window must not count")
	}
}

func TestLoginLimiterReset(t *testing.T) {
	l := NewLoginLimiter(5, time.Minute, 10*time.Minute)
	for i := 0; i < 5; i++ {
		l.Fail("1.2.3.4")
	}
	if !l.Locked("1.2.3.4") {
		t.Fatal("must be locked")
	}
	l.Reset("1.2.3.4")
	if l.Locked("1.2.3.4") {
		t.Fatal("reset must clear the lock")
	}
}
