package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/kerbos/k8sinsight/internal/api"
	"github.com/kerbos/k8sinsight/internal/auth"
	"github.com/kerbos/k8sinsight/internal/cluster"
	"github.com/kerbos/k8sinsight/internal/config"
	"github.com/kerbos/k8sinsight/internal/core"
	"github.com/kerbos/k8sinsight/internal/store"
	"github.com/kerbos/k8sinsight/internal/store/repository"
)

func main() {
	configPath := flag.String("config", "", "配置文件路径")
	flag.Parse()

	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 应用默认时区（默认北京时间 Asia/Shanghai）
	if cfg.Server.Timezone != "" {
		loc, err := time.LoadLocation(cfg.Server.Timezone)
		if err != nil {
			fmt.Fprintf(os.Stderr, "加载时区失败(%s): %v\n", cfg.Server.Timezone, err)
			os.Exit(1)
		}
		time.Local = loc
	}

	// 初始化日志
	logger, err := core.NewLogger(cfg.Log.Level)
	if err != nil {
		fmt.Fprintf(os.Stderr, "初始化日志失败: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = logger.Sync() }()

	logger.Info("K8sInsight 启动中...",
		zap.Int("port", cfg.Server.Port),
	)

	// 初始化数据库
	db, err := store.NewDB(cfg.DB, logger)
	if err != nil {
		logger.Fatal("数据库初始化失败", zap.Error(err))
	}

	// 全局 context，用于优雅关停
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 初始化 Repository
	incidentRepo := repository.NewIncidentRepository(db)
	evidenceRepo := repository.NewEvidenceRepository(db)
	clusterRepo := repository.NewClusterRepository(db)
	monitorRuleRepo := repository.NewMonitorRuleRepository(db)
	userRepo := repository.NewUserRepository(db)
	settingRepo := repository.NewSettingRepository(db)
	roleRepo := repository.NewRoleRepository(db)

	// 初始化 TokenService
	jwtSecret := cfg.Server.JWTSecret
	if jwtSecret == "" {
		// 未配置 JWT Secret 时自动生成（仅适用于开发环境）
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			logger.Fatal("生成 JWT Secret 失败", zap.Error(err))
		}
		jwtSecret = hex.EncodeToString(b)
		logger.Warn("未配置 JWT Secret，已自动生成随机密钥（重启后所有令牌将失效）")
	}
	tokenService := auth.NewTokenService(jwtSecret, cfg.Server.AccessTokenTTL, cfg.Server.RefreshTokenTTL)

	// 播种默认管理员
	adminPassword := cfg.Server.DefaultAdminPassword
	if adminPassword == "" {
		adminPassword = "k8sinsight"
	}
	if err := auth.SeedDefaultAdmin(ctx, userRepo, adminPassword, logger); err != nil {
		logger.Error("创建默认管理员失败", zap.Error(err))
	}

	// 播种内置角色
	if err := auth.SeedBuiltInRoles(ctx, roleRepo, logger); err != nil {
		logger.Error("创建内置角色失败", zap.Error(err))
	}

	// 初始化 SSO 服务
	ssoService := auth.NewSSOService(settingRepo, userRepo, roleRepo, tokenService, logger)

	// 初始化集群管理器
	clusterMgr := cluster.NewManager(clusterRepo, monitorRuleRepo, incidentRepo, evidenceRepo, settingRepo, cfg, logger)

	// 从数据库重载活跃集群（重启恢复）
	if err := clusterMgr.ReloadFromDB(ctx); err != nil {
		logger.Error("重载活跃集群失败", zap.Error(err))
	}

	// 启动 HTTP 服务
	router := api.NewRouter(incidentRepo, evidenceRepo, clusterRepo, monitorRuleRepo, userRepo, settingRepo, roleRepo, clusterMgr, tokenService, ssoService, logger)
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	go func() {
		logger.Info("HTTP 服务启动", zap.String("addr", server.Addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP 服务启动失败", zap.Error(err))
		}
	}()

	// 等待退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	logger.Info("收到退出信号", zap.String("signal", sig.String()))

	// 优雅关停
	cancel()
	clusterMgr.StopAll()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP 服务关停失败", zap.Error(err))
	}

	logger.Info("K8sInsight 已关停")
}
