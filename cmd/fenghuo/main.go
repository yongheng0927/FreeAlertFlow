// Fenghuo 入口：装配配置、数据库、各服务以及 HTTP 服务器
package main

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/yongheng0927/fenghuo/internal/config"
	"github.com/yongheng0927/fenghuo/internal/database"
	"github.com/yongheng0927/fenghuo/internal/handler"
	fafcrypto "github.com/yongheng0927/fenghuo/internal/pkg/crypto"
	fafjwt "github.com/yongheng0927/fenghuo/internal/pkg/jwt"
	"github.com/yongheng0927/fenghuo/internal/pkg/render"
	"github.com/yongheng0927/fenghuo/internal/service"
	"github.com/yongheng0927/fenghuo/web"
)

func main() {
	if err := run(); err != nil {
		slog.Error("startup failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	setupLogger(cfg.Log.Level)

	if cfg.JWTSecretGenerated {
		slog.Warn("FENGHUO_JWT_SECRET not set, generated a random secret; " +
			"all tokens become invalid after restart")
	}

	db, err := database.Open(cfg.Database.DSN())
	if err != nil {
		return err
	}
	if err := database.Migrate(db); err != nil {
		return err
	}

	userStore := service.NewGormUserStore(db)
	tokenStore := service.NewGormRefreshTokenStore(db)
	oauthStore := service.NewGormOAuthIdentityStore(db)
	sourceStore := service.NewGormSourceStore(db)
	alertStore := service.NewGormAlertStore(db)
	ruleStore := service.NewGormRuleStore(db)
	channelStore := service.NewGormChannelStore(db)
	templateStore := service.NewGormTemplateStore(db)
	deliveryStore := service.NewGormDeliveryStore(db)

	jwtMgr := fafjwt.NewManager([]byte(cfg.JWT.Secret), cfg.JWT.AccessTTL, cfg.JWT.RefreshTTL)
	authSvc := service.NewAuthService(userStore, tokenStore, jwtMgr)

	// M2 核心链路：webhook 接入 -> 去重 -> 路由 -> 渲染 -> 发送
	cipher, err := fafcrypto.New([]byte(cfg.SecretKey))
	if err != nil {
		return err
	}
	sender := service.NewDispatcher(
		service.NewFeishuSender(cipher, cfg.Channel.HTTPTimeout),
		service.NewDingTalkSender(cipher, cfg.Channel.HTTPTimeout),
		service.NewWeComSender(cipher, cfg.Channel.HTTPTimeout),
	)
	renderEngine := render.NewEngine(nil)
	deliverer := service.NewDeliverer(
		templateStore, deliveryStore, alertStore, channelStore, sourceStore,
		sender, renderEngine, cfg.Channel.RetryMax, cfg.Server.RootURL,
	)
	alertSvc := service.NewAlertService(sourceStore, alertStore, ruleStore, channelStore,
		deliverer, cfg.Alert.DedupWindow)

	// M3 管理服务
	sourceSvc := service.NewSourceService(sourceStore, ruleStore, alertStore)
	channelSvc := service.NewChannelService(channelStore, ruleStore, templateStore, cipher, sender)
	templateSvc := service.NewTemplateService(templateStore, channelStore, alertStore, renderEngine, sender, cfg.Server.RootURL)
	ruleSvc := service.NewRuleService(ruleStore, sourceStore, channelStore)
	userAdminSvc := service.NewUserAdminService(userStore, tokenStore, oauthStore)

	// M4：飞书 OAuth（FR-5.3）以及带配置注入的内嵌 SPA
	var oauthSvc *service.OAuthService
	if cfg.OAuth.Enabled {
		provider := service.NewFeishuProvider(cfg.OAuth.FeishuAppID, cfg.OAuth.FeishuAppSecret)
		oauthSvc = service.NewOAuthService(provider, userStore, oauthStore, authSvc,
			cfg.OAuth.AutoCreateUser, cfg.OAuth.AllowedEmails)
	}
	oauthStates := service.NewOAuthStateStore(10 * time.Minute)
	distFS, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		return err
	}
	staticH, err := handler.NewStaticHandler(cfg, distFS)
	if err != nil {
		return err
	}
	if base := cfg.BasePath(); base != "" {
		slog.Info("serving under root url sub-path", "base", base)
	}

	// 首次启动初始化时：当数据库为空且配置了 FENGHUO_ADMIN_USER/FENGHUO_ADMIN_PASSWORD  创建初始管理员（FR-5.1）
	created, err := authSvc.BootstrapAdmin(context.Background(), cfg.Admin.User, cfg.Admin.Password)
	if err != nil {
		return err
	}
	if created {
		slog.Info("initial admin user created", "username", cfg.Admin.User)
	}

	// NFR-1：每分钟 5 次登录失败后，锁定该 IP 10 分钟
	limiter := service.NewLoginLimiter(5, time.Minute, 10*time.Minute)

	router := handler.NewRouter(&handler.Deps{
		Config:        cfg,
		DB:            db,
		JWT:           jwtMgr,
		Auth:          authSvc,
		Limiter:       limiter,
		Users:         userStore,
		Alerts:        alertSvc,
		Sources:       sourceSvc,
		SourceStore:   sourceStore,
		ChannelStore:  channelStore,
		TemplateStore: templateStore,
		RuleStore:     ruleStore,
		AlertStore:    alertStore,
		DeliveryStore: deliveryStore,
		Channels:      channelSvc,
		Templates:     templateSvc,
		Rules:         ruleSvc,
		UserAdmin:     userAdminSvc,
		Deliverer:     deliverer,
		OAuth:         oauthSvc,
		States:        oauthStates,
		Static:        staticH,
	})

	srv := &http.Server{
		Addr:    cfg.Server.HTTPAddr,
		Handler: router,
	}
	go func() {
		slog.Info("http server listening", "addr", cfg.Server.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server failed", "error", err)
			os.Exit(1)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	slog.Info("shutdown complete")
	return nil
}

func setupLogger(level string) {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})))
}
