// Package config 从环境变量（前缀 FENGHUO_）、可选的 config.yaml 以及内置默认值
// 加载应用配置
// 优先级：环境变量 > config.yaml > 默认值
package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config 是应用的根配置
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	JWT      JWTConfig
	Log      LogConfig
	Alert    AlertConfig
	Channel  ChannelConfig
	OAuth    OAuthConfig

	// JWTSecretGenerated 为 true 表示 jwt.secret 为空，启动时生成了随机
	// 密钥（会被持久化到 app_settings，重启不失效；多副本共享）
	JWTSecretGenerated bool
}

type ServerConfig struct {
	HTTPAddr string
	RootURL  string
	// TrustedProxies 是可信反向代理的 IP/CIDR 列表；为空表示不信任任何
	// 代理（忽略 X-Forwarded-For 等头，ClientIP 取直连对端），防止客户端
	// 伪造 XFF 绕过按 IP 的登录限流。部署在反向代理之后时应配置为代理的
	// IP 或网段
	TrustedProxies []string
}

type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// DSN 把分字段配置拼成 postgres:// 连接串（用户名/密码已做 URL 转义）
func (d DatabaseConfig) DSN() string {
	u := &url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(d.User, d.Password),
		Host:     fmt.Sprintf("%s:%d", d.Host, d.Port),
		Path:     d.DBName,
		RawQuery: "sslmode=" + d.SSLMode,
	}
	return u.String()
}

type JWTConfig struct {
	Secret     string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

type LogConfig struct {
	Level string
}

type AlertConfig struct {
	DedupWindow   time.Duration
	RetentionDays int
}

type ChannelConfig struct {
	HTTPTimeout time.Duration
	RetryMax    int
}

type OAuthConfig struct {
	Enabled         bool
	FeishuAppID     string
	FeishuAppSecret string
	AutoCreateUser  bool
	AllowedEmails   []string
}

// Load 按优先级读取配置：环境变量 > config.yaml > 默认值
func Load() (*Config, error) {
	v := viper.New()
	v.SetEnvPrefix("FENGHUO")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// 默认值依据 REQUIREMENTS §4.7
	v.SetDefault("server.http_addr", ":8080")
	v.SetDefault("server.root_url", "http://localhost:8080/")
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.sslmode", "disable")
	v.SetDefault("log.level", "info")
	v.SetDefault("alert.dedup_window", 5*time.Minute)
	v.SetDefault("alert.retention_days", 30)
	v.SetDefault("channel.http_timeout", 10*time.Second)
	v.SetDefault("channel.retry_max", 2)
	v.SetDefault("jwt.access_ttl", 2*time.Hour)
	v.SetDefault("jwt.refresh_ttl", 7*24*time.Hour)

	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")
	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return nil, fmt.Errorf("read config file: %w", err)
		}
	}

	cfg := &Config{
		Server: ServerConfig{
			HTTPAddr:       v.GetString("server.http_addr"),
			RootURL:        v.GetString("server.root_url"),
			TrustedProxies: v.GetStringSlice("server.trusted_proxies"),
		},
		Database: DatabaseConfig{
			Host:     v.GetString("database.host"),
			Port:     v.GetInt("database.port"),
			User:     v.GetString("database.user"),
			Password: v.GetString("database.password"),
			DBName:   v.GetString("database.dbname"),
			SSLMode:  v.GetString("database.sslmode"),
		},
		JWT: JWTConfig{
			Secret:     v.GetString("jwt.secret"),
			AccessTTL:  v.GetDuration("jwt.access_ttl"),
			RefreshTTL: v.GetDuration("jwt.refresh_ttl"),
		},
		Log: LogConfig{Level: v.GetString("log.level")},
		Alert: AlertConfig{
			DedupWindow:   v.GetDuration("alert.dedup_window"),
			RetentionDays: v.GetInt("alert.retention_days"),
		},
		Channel: ChannelConfig{
			HTTPTimeout: v.GetDuration("channel.http_timeout"),
			RetryMax:    v.GetInt("channel.retry_max"),
		},
		OAuth: OAuthConfig{
			Enabled:         v.GetBool("oauth.enabled"),
			FeishuAppID:     v.GetString("oauth.feishu_app_id"),
			FeishuAppSecret: v.GetString("oauth.feishu_app_secret"),
			AutoCreateUser:  v.GetBool("oauth.auto_create_user"),
			AllowedEmails:   v.GetStringSlice("oauth.allowed_emails"),
		},
	}

	if cfg.Database.User == "" || cfg.Database.DBName == "" {
		return nil, errors.New("database config incomplete: FENGHUO_DATABASE_USER and FENGHUO_DATABASE_DBNAME are required")
	}
	if cfg.JWT.Secret == "" {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return nil, fmt.Errorf("generate random jwt secret: %w", err)
		}
		cfg.JWT.Secret = hex.EncodeToString(b)
		cfg.JWTSecretGenerated = true
	}
	if cfg.OAuth.Enabled && (cfg.OAuth.FeishuAppID == "" || cfg.OAuth.FeishuAppSecret == "") {
		return nil, errors.New("FENGHUO_OAUTH_ENABLED=true requires FENGHUO_OAUTH_FEISHU_APP_ID and FENGHUO_OAUTH_FEISHU_APP_SECRET")
	}
	return cfg, nil
}

// BasePath 从 server.root_url 中提取 URL 路径前缀，用于子路径部署（FR-6）：
// "https://example.com/freealertflow/" -> "/freealertflow"，"http://localhost:8080/" -> ""
func (c *Config) BasePath() string {
	u, err := url.Parse(c.Server.RootURL)
	if err != nil {
		return ""
	}
	return strings.TrimRight(u.Path, "/")
}
