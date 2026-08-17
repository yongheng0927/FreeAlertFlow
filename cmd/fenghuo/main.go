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
	fafjwt "github.com/yongheng0927/fenghuo/internal/pkg/jwt"
	"github.com/yongheng0927/fenghuo/internal/pkg/render"
	"github.com/yongheng0927/fenghuo/internal/service"
	"github.com/yongheng0927/fenghuo/web"
)

func main() {
	// 子命令：容器内重置初始管理员密码（docker exec / kubectl exec 使用）
	if len(os.Args) > 1 && os.Args[1] == "reset-admin-password" {
		if err := resetAdminPassword(os.Args[2:]); err != nil {
			slog.Error("reset-admin-password failed", "error", err)
			os.Exit(1)
		}
		return
	}
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

	db, err := database.Open(cfg.Database.DSN())
	if err != nil {
		return err
	}
	if err := database.Migrate(db); err != nil {
		return err
	}

	// 未显式配置 JWT 密钥时，随机生成值会持久化到 app_settings：重启和
	// 多副本共用同一个密钥，token 不再随重启失效
	jwtSecret := cfg.JWT.Secret
	if cfg.JWTSecretGenerated {
		jwtSecret, err = service.LoadOrStoreSetting(context.Background(),
			db, service.SettingKeyJWTSecret, cfg.JWT.Secret)
		if err != nil {
			return err
		}
		slog.Warn("FENGHUO_JWT_SECRET not set; using a generated secret persisted in app_settings " +
			"(set it explicitly for full control)")
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
	statsStore := service.NewGormStatsStore(db)

	jwtMgr := fafjwt.NewManager([]byte(jwtSecret), cfg.JWT.AccessTTL, cfg.JWT.RefreshTTL)
	authSvc := service.NewAuthService(userStore, tokenStore, jwtMgr)

	// M2 核心链路：webhook 接入 -> 去重 -> 路由 -> 渲染 -> 发送
	sender := service.NewDispatcher(
		service.NewFeishuSender(cfg.Channel.HTTPTimeout),
		service.NewDingTalkSender(cfg.Channel.HTTPTimeout),
		service.NewWeComSender(cfg.Channel.HTTPTimeout),
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
	channelSvc := service.NewChannelService(channelStore, ruleStore, templateStore, sender)
	templateSvc := service.NewTemplateService(templateStore, channelStore, alertStore, renderEngine, sender, cfg.Server.RootURL)
	ruleSvc := service.NewRuleService(ruleStore, sourceStore, channelStore)
	userAdminSvc := service.NewUserAdminService(userStore, tokenStore, oauthStore, cfg.Admin.User)

	// M4：飞书 OAuth（FR-5.3）以及带配置注入的内嵌 SPA
	var oauthSvc *service.OAuthService
	if cfg.OAuth.Enabled {
		provider := service.NewFeishuProvider(cfg.OAuth.FeishuAppID, cfg.OAuth.FeishuAppSecret)
		oauthSvc = service.NewOAuthService(provider, userStore, oauthStore, authSvc,
			cfg.OAuth.AutoCreateUser, cfg.OAuth.AllowedEmails)
	}
	oauthStates := service.NewOAuthStateStore(db, 10*time.Minute)
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

	// NFR-1：固定窗口 1 分钟内 5 次登录失败后，锁定该 IP 10 分钟；
	// 状态存于 login_attempts 表，多副本共享计数与锁定
	limiter := service.NewLoginLimiter(service.NewGormLoginAttemptStore(db), 5, time.Minute, 10*time.Minute)

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
		StatsStore:    statsStore,
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

	// 保留期清理后台任务（retention_days <= 0 时内部直接返回）
	go service.NewRetentionCleaner(alertStore,
		time.Duration(cfg.Alert.RetentionDays)*24*time.Hour).Run(ctx)

	<-ctx.Done()

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}

	// 等待进行中的异步投递完成（滚动更新时不打断正在重试的投递）；
	// 上限对齐单次投递最坏耗时（timeout × 尝试次数）+ 余量，防止卡死退出
	drainTimeout := cfg.Channel.HTTPTimeout*time.Duration(cfg.Channel.RetryMax+1) + 5*time.Second
	drainCtx, drainCancel := context.WithTimeout(context.Background(), drainTimeout)
	defer drainCancel()
	if err := alertSvc.Drain(drainCtx); err != nil {
		slog.Warn("drain in-flight dispatches timed out, forcing exit",
			"timeout", drainTimeout)
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
