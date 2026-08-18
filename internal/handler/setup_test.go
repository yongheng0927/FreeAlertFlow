package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yongheng0927/fenghuo/internal/model"
	fafjwt "github.com/yongheng0927/fenghuo/internal/pkg/jwt"
	"github.com/yongheng0927/fenghuo/internal/pkg/password"
	"github.com/yongheng0927/fenghuo/internal/service"
)

// --- setup 端点测试用的内存 fake（service 包的 fake 是测试内部实现，不可复用） ---

type setupUserStore struct {
	byID   map[int64]*model.User
	nextID int64
}

func newSetupUserStore() *setupUserStore {
	return &setupUserStore{byID: map[int64]*model.User{}, nextID: 1}
}

func (f *setupUserStore) FindByUsername(_ context.Context, username string) (*model.User, error) {
	for _, u := range f.byID {
		if u.Username == username {
			return u, nil
		}
	}
	return nil, nil
}

func (f *setupUserStore) FindByID(_ context.Context, id int64) (*model.User, error) {
	return f.byID[id], nil
}

func (f *setupUserStore) Count(context.Context) (int64, error) { return int64(len(f.byID)), nil }

func (f *setupUserStore) CountBootstrapAdmins(context.Context) (int64, error) {
	var n int64
	for _, u := range f.byID {
		if u.IsBootstrap {
			n++
		}
	}
	return n, nil
}

func (f *setupUserStore) FindBootstrap(context.Context) (*model.User, error) {
	for _, u := range f.byID {
		if u.IsBootstrap {
			return u, nil
		}
	}
	return nil, nil
}

func (f *setupUserStore) List(context.Context, int, int) ([]model.User, int64, error) {
	return nil, 0, nil
}

func (f *setupUserStore) Create(_ context.Context, u *model.User) error {
	u.ID = f.nextID
	f.nextID++
	f.byID[u.ID] = u
	return nil
}

func (f *setupUserStore) UpdateLastLogin(context.Context, int64, time.Time) error { return nil }
func (f *setupUserStore) UpdatePassword(context.Context, int64, string) error     { return nil }
func (f *setupUserStore) UpdateRoleAndStatus(context.Context, int64, string, bool) error {
	return nil
}
func (f *setupUserStore) UpdateProfile(context.Context, int64, string, string, string) error {
	return nil
}
func (f *setupUserStore) CountEnabledAdmins(context.Context) (int64, error) { return 0, nil }
func (f *setupUserStore) Delete(context.Context, int64) error               { return nil }

type setupTokenStore struct{}

func (setupTokenStore) Create(context.Context, *model.RefreshToken) error { return nil }
func (setupTokenStore) FindByHash(context.Context, string) (*model.RefreshToken, error) {
	return nil, nil
}
func (setupTokenStore) Save(context.Context, *model.RefreshToken) error  { return nil }
func (setupTokenStore) RevokeByHash(context.Context, string) error       { return nil }
func (setupTokenStore) RevokeAllForUser(context.Context, int64) error    { return nil }
func (setupTokenStore) DeleteAllForUser(context.Context, int64) error    { return nil }

// fakeSetupLimiter 记录 Fail/Reset 调用，可预置锁定状态
type fakeSetupLimiter struct {
	locked bool
	fails  int
	resets int
}

func (l *fakeSetupLimiter) Locked(context.Context, string) (bool, error) { return l.locked, nil }
func (l *fakeSetupLimiter) Fail(context.Context, string) error           { l.fails++; return nil }
func (l *fakeSetupLimiter) Reset(context.Context, string) error          { l.resets++; return nil }

func newSetupTestRouter() (*gin.Engine, *setupUserStore, *fakeSetupLimiter) {
	gin.SetMode(gin.TestMode)
	users := newSetupUserStore()
	jwtMgr := fafjwt.NewManager([]byte("test-secret-0123456789abcdef0123"), 2*time.Hour, 7*24*time.Hour)
	auth := service.NewAuthService(users, setupTokenStore{}, jwtMgr)
	limiter := &fakeSetupLimiter{}
	h := NewSetupHandler(auth, limiter)
	r := gin.New()
	r.GET("/api/v1/setup/status", h.Status)
	r.POST("/api/v1/setup", h.Setup)
	return r, users, limiter
}

func doSetupRequest(t *testing.T, r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	} else {
		reader = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestSetupStatusEndpoint(t *testing.T) {
	r, users, _ := newSetupTestRouter()

	w := doSetupRequest(t, r, http.MethodGet, "/api/v1/setup/status", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"initialized":false`) {
		t.Fatalf("status before setup = %d %s", w.Code, w.Body.String())
	}

	hash, _ := password.Hash("admin-pw-123")
	_ = users.Create(context.Background(), &model.User{
		Username: "admin", PasswordHash: &hash, Role: model.RoleAdmin, Enabled: true, IsBootstrap: true,
	})
	w = doSetupRequest(t, r, http.MethodGet, "/api/v1/setup/status", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"initialized":true`) {
		t.Fatalf("status after setup = %d %s", w.Code, w.Body.String())
	}
}

func TestSetupEndpointCreatesAdminAndIssuesTokens(t *testing.T) {
	r, users, limiter := newSetupTestRouter()

	w := doSetupRequest(t, r, http.MethodPost, "/api/v1/setup",
		`{"username":"admin","password":"admin-pw-123"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	for _, key := range []string{`"access_token"`, `"refresh_token"`, `"token_type":"Bearer"`, `"expires_in"`} {
		if !strings.Contains(w.Body.String(), key) {
			t.Errorf("response must contain %s: %s", key, w.Body.String())
		}
	}
	admin, _ := users.FindByUsername(context.Background(), "admin")
	if admin == nil || admin.Role != model.RoleAdmin || !admin.IsBootstrap {
		t.Fatalf("bootstrap admin = %+v", admin)
	}
	if limiter.resets != 1 || limiter.fails != 0 {
		t.Errorf("limiter resets=%d fails=%d, want 1/0", limiter.resets, limiter.fails)
	}
}

func TestSetupEndpointAlreadyCompleted(t *testing.T) {
	r, _, limiter := newSetupTestRouter()

	w := doSetupRequest(t, r, http.MethodPost, "/api/v1/setup",
		`{"username":"admin","password":"admin-pw-123"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("first setup = %d", w.Code)
	}
	w = doSetupRequest(t, r, http.MethodPost, "/api/v1/setup",
		`{"username":"other","password":"other-pw-123"}`)
	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "setup already completed") {
		t.Fatalf("second setup = %d %s, want 403 setup already completed", w.Code, w.Body.String())
	}
	if limiter.fails != 1 {
		t.Errorf("limiter fails = %d, want 1 (failed attempt must count)", limiter.fails)
	}
}

func TestSetupEndpointValidation(t *testing.T) {
	r, users, limiter := newSetupTestRouter()

	bad := []string{
		`{"username":"admin","password":"short"}`, // 密码不足 8 位
		`{"username":"","password":"admin-pw-123"}`, // 空用户名（bind 即拒绝）
		`{}`, // 缺字段
	}
	want := []int{http.StatusBadRequest, http.StatusBadRequest, http.StatusBadRequest}
	for i, body := range bad {
		w := doSetupRequest(t, r, http.MethodPost, "/api/v1/setup", body)
		if w.Code != want[i] {
			t.Errorf("case %d: status = %d, want %d (%s)", i, w.Code, want[i], w.Body.String())
		}
	}
	if n, _ := users.Count(context.Background()); n != 0 {
		t.Errorf("users = %d, want 0", n)
	}
	// 服务层校验失败计入限流；bind 失败不计（镜像 Login 只计认证失败）
	if limiter.fails != 1 {
		t.Errorf("limiter fails = %d, want 1", limiter.fails)
	}
}

func TestSetupEndpointLocked(t *testing.T) {
	r, _, limiter := newSetupTestRouter()
	limiter.locked = true

	w := doSetupRequest(t, r, http.MethodPost, "/api/v1/setup",
		`{"username":"admin","password":"admin-pw-123"}`)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429 (%s)", w.Code, w.Body.String())
	}
}
