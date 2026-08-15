package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const testSecret = "0123456789abcdef0123456789abcdef" // 恰好 32 字节

// setTestDatabaseEnv 设置最小可用的数据库环境变量
func setTestDatabaseEnv(t *testing.T) {
	t.Helper()
	t.Setenv("FENGHUO_DATABASE_USER", "faf")
	t.Setenv("FENGHUO_DATABASE_PASSWORD", "pass")
	t.Setenv("FENGHUO_DATABASE_DBNAME", "freealertflow")
}

// clearFenghuoEnv 清除所有可能从外部环境泄漏进来的 FENGHUO_* 变量
func clearFenghuoEnv(t *testing.T) {
	t.Helper()
	for _, kv := range os.Environ() {
		key := kv
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				key = kv[:i]
				break
			}
		}
		if len(key) >= 8 && key[:8] == "FENGHUO_" {
			t.Setenv(key, "") // viper 将空环境变量视为未设置
		}
	}
}

func TestDefaults(t *testing.T) {
	clearFenghuoEnv(t)
	t.Chdir(t.TempDir())
	t.Setenv("FENGHUO_SECRET_KEY", testSecret)
	setTestDatabaseEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.HTTPAddr != ":8080" {
		t.Errorf("http_addr = %q", cfg.Server.HTTPAddr)
	}
	if cfg.Server.RootURL != "http://localhost:8080/" {
		t.Errorf("root_url = %q", cfg.Server.RootURL)
	}
	if cfg.Database.Host != "localhost" || cfg.Database.Port != 5432 || cfg.Database.SSLMode != "disable" {
		t.Errorf("database defaults = %+v", cfg.Database)
	}
	if cfg.Log.Level != "info" {
		t.Errorf("log level = %q", cfg.Log.Level)
	}
	if cfg.Alert.DedupWindow != 5*time.Minute {
		t.Errorf("dedup_window = %v", cfg.Alert.DedupWindow)
	}
	if cfg.Alert.RetentionDays != 30 {
		t.Errorf("retention_days = %d", cfg.Alert.RetentionDays)
	}
	if cfg.Channel.HTTPTimeout != 10*time.Second {
		t.Errorf("http_timeout = %v", cfg.Channel.HTTPTimeout)
	}
	if cfg.Channel.RetryMax != 2 {
		t.Errorf("retry_max = %d", cfg.Channel.RetryMax)
	}
	if cfg.JWT.AccessTTL != 2*time.Hour {
		t.Errorf("access_ttl = %v", cfg.JWT.AccessTTL)
	}
	if cfg.JWT.RefreshTTL != 7*24*time.Hour {
		t.Errorf("refresh_ttl = %v", cfg.JWT.RefreshTTL)
	}
	// FENGHUO_JWT_SECRET 未设置：应生成随机密钥并置标志位
	if !cfg.JWTSecretGenerated || cfg.JWT.Secret == "" {
		t.Error("jwt secret should be randomly generated when unset")
	}
}

func TestEnvOverridesDefaults(t *testing.T) {
	clearFenghuoEnv(t)
	t.Chdir(t.TempDir())
	t.Setenv("FENGHUO_SECRET_KEY", testSecret)
	setTestDatabaseEnv(t)
	t.Setenv("FENGHUO_SERVER_HTTP_ADDR", "0.0.0.0:9090")
	t.Setenv("FENGHUO_JWT_ACCESS_TTL", "30m")
	t.Setenv("FENGHUO_JWT_SECRET", "fixed-secret")
	t.Setenv("FENGHUO_LOG_LEVEL", "debug")
	t.Setenv("FENGHUO_CHANNEL_RETRY_MAX", "0")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.HTTPAddr != "0.0.0.0:9090" {
		t.Errorf("http_addr = %q", cfg.Server.HTTPAddr)
	}
	if cfg.JWT.AccessTTL != 30*time.Minute {
		t.Errorf("access_ttl = %v", cfg.JWT.AccessTTL)
	}
	if cfg.JWT.Secret != "fixed-secret" || cfg.JWTSecretGenerated {
		t.Error("jwt secret should come from env")
	}
	if cfg.Log.Level != "debug" {
		t.Errorf("log level = %q", cfg.Log.Level)
	}
	if cfg.Channel.RetryMax != 0 {
		t.Errorf("retry_max = %d", cfg.Channel.RetryMax)
	}
}

func TestConfigFileAndPrecedence(t *testing.T) {
	clearFenghuoEnv(t)
	dir := t.TempDir()
	yaml := []byte(`
server:
  http_addr: ":1111"
  root_url: "https://alerts.example.com/"
database:
  user: "faf"
  password: "pass"
  dbname: "freealertflow"
secret_key: "` + testSecret + `"
`)
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), yaml, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	// 没有环境变量时：取值来自 config.yaml
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.HTTPAddr != ":1111" {
		t.Errorf("http_addr from file = %q", cfg.Server.HTTPAddr)
	}
	if cfg.Server.RootURL != "https://alerts.example.com/" {
		t.Errorf("root_url from file = %q", cfg.Server.RootURL)
	}

	// 环境变量优先于 config.yaml
	t.Setenv("FENGHUO_SERVER_HTTP_ADDR", ":2222")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.HTTPAddr != ":2222" {
		t.Errorf("env should override config file, got %q", cfg.Server.HTTPAddr)
	}
}

func TestSecretKeyRequired(t *testing.T) {
	clearFenghuoEnv(t)
	t.Chdir(t.TempDir())
	setTestDatabaseEnv(t)
	if _, err := Load(); err == nil {
		t.Fatal("Load must fail without FENGHUO_SECRET_KEY")
	}
}

func TestSecretKeyMustBe32Bytes(t *testing.T) {
	clearFenghuoEnv(t)
	t.Chdir(t.TempDir())
	t.Setenv("FENGHUO_SECRET_KEY", "too-short")
	setTestDatabaseEnv(t)
	if _, err := Load(); err == nil {
		t.Fatal("Load must fail when FENGHUO_SECRET_KEY is not 32 bytes")
	}
}

func TestDatabaseConfigRequired(t *testing.T) {
	clearFenghuoEnv(t)
	t.Chdir(t.TempDir())
	t.Setenv("FENGHUO_SECRET_KEY", testSecret)
	if _, err := Load(); err == nil {
		t.Fatal("Load must fail without database user/dbname")
	}
}

func TestDatabaseDSN(t *testing.T) {
	d := DatabaseConfig{
		Host:     "db.example.com",
		Port:     5433,
		User:     "faf",
		Password: "p@ss/word",
		DBName:   "freealertflow",
		SSLMode:  "require",
	}
	want := "postgres://faf:p%40ss%2Fword@db.example.com:5433/freealertflow?sslmode=require"
	if got := d.DSN(); got != want {
		t.Errorf("DSN() = %q, want %q", got, want)
	}
}

func TestBasePath(t *testing.T) {
	cases := []struct{ rootURL, want string }{
		{"http://localhost:8080/", ""},
		{"http://localhost:8080", ""},
		{"https://example.com/freealertflow/", "/freealertflow"},
		{"https://example.com/freealertflow", "/freealertflow"},
		{"https://example.com/a/b/", "/a/b"},
	}
	for _, tc := range cases {
		c := &Config{Server: ServerConfig{RootURL: tc.rootURL}}
		if got := c.BasePath(); got != tc.want {
			t.Errorf("BasePath(%q) = %q, want %q", tc.rootURL, got, tc.want)
		}
	}
}

func TestOAuthEnabledRequiresCredentials(t *testing.T) {
	clearFenghuoEnv(t)
	t.Chdir(t.TempDir())
	t.Setenv("FENGHUO_SECRET_KEY", testSecret)
	setTestDatabaseEnv(t)
	t.Setenv("FENGHUO_OAUTH_ENABLED", "true")
	if _, err := Load(); err == nil {
		t.Fatal("Load must fail when OAuth is enabled without app credentials")
	}
	t.Setenv("FENGHUO_OAUTH_FEISHU_APP_ID", "cli_x")
	t.Setenv("FENGHUO_OAUTH_FEISHU_APP_SECRET", "sec_x")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.OAuth.Enabled || cfg.OAuth.FeishuAppID != "cli_x" {
		t.Errorf("oauth config = %+v", cfg.OAuth)
	}
}
