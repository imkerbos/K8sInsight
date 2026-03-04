package api

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/kerbos/k8sinsight/internal/api/handler"
	"github.com/kerbos/k8sinsight/internal/api/middleware"
	"github.com/kerbos/k8sinsight/internal/auth"
	"github.com/kerbos/k8sinsight/internal/cluster"
	"github.com/kerbos/k8sinsight/internal/store/repository"
)

// NewRouter 创建并配置 Gin 路由
func NewRouter(
	incidentRepo repository.IncidentRepository,
	evidenceRepo repository.EvidenceRepository,
	clusterRepo repository.ClusterRepository,
	monitorRuleRepo repository.MonitorRuleRepository,
	userRepo repository.UserRepository,
	settingRepo repository.SettingRepository,
	roleRepo repository.RoleRepository,
	clusterMgr *cluster.Manager,
	tokenService *auth.TokenService,
	ssoService *auth.SSOService,
	logger *zap.Logger,
) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	// 权限检查器
	permChecker := middleware.NewPermissionChecker(roleRepo)

	// 健康检查（不需要认证）
	healthHandler := handler.NewHealthHandler()
	r.GET("/healthz", healthHandler.Healthz)
	r.GET("/readyz", healthHandler.Readyz)

	// 认证处理器
	authHandler := handler.NewAuthHandler(userRepo, roleRepo, tokenService, logger)

	// 公开认证端点（不需要认证）
	authPublic := r.Group("/api/v1/auth")
	{
		authPublic.POST("/login", authHandler.Login)
		authPublic.POST("/refresh", authHandler.Refresh)
	}

	// SSO 公开端点（无需认证）
	ssoHandler := handler.NewSSOHandler(ssoService, logger)
	authPublic.GET("/sso/config", ssoHandler.GetConfig)
	authPublic.GET("/sso/authorize", ssoHandler.Authorize)
	authPublic.POST("/sso/callback", ssoHandler.Callback)

	// 公开设置端点（登录页需要展示品牌信息）
	settingHandler := handler.NewSettingHandler(settingRepo, incidentRepo, evidenceRepo, logger)
	r.GET("/api/v1/settings/branding", settingHandler.GetBranding)

	// API v1（需要 JWT 认证）
	v1 := r.Group("/api/v1")
	v1.Use(middleware.JWTAuth(tokenService))
	{
		// 认证相关（需要登录）
		authGroup := v1.Group("/auth")
		{
			authGroup.POST("/logout", authHandler.Logout)
			authGroup.POST("/change-password", authHandler.ChangePassword)
			authGroup.GET("/me", authHandler.Me)
		}

		// 事件管理
		incidentHandler := handler.NewIncidentHandler(incidentRepo, evidenceRepo, settingRepo, logger)
		v1.GET("/incidents", permChecker.RequirePermission("incident:read"), incidentHandler.List)
		v1.GET("/incidents/:id", permChecker.RequirePermission("incident:read"), incidentHandler.Get)
		v1.GET("/incidents/:id/evidences", permChecker.RequirePermission("incident:read"), incidentHandler.GetEvidences)
		v1.GET("/incidents/:id/timeline", permChecker.RequirePermission("incident:read"), incidentHandler.GetTimeline)
		v1.POST("/incidents/:id/recollect-metrics", permChecker.RequirePermission("incident:read"), permChecker.RequirePermission("settings:manage"), incidentHandler.RecollectMetrics)

		// 集群管理
		clusterHandler := handler.NewClusterHandler(clusterRepo, clusterMgr, logger)
		v1.GET("/clusters", permChecker.RequirePermission("cluster:read"), clusterHandler.List)
		v1.GET("/clusters/:id", permChecker.RequirePermission("cluster:read"), clusterHandler.Get)
		v1.POST("/clusters", permChecker.RequirePermission("cluster:write"), clusterHandler.Create)
		v1.PUT("/clusters/:id", permChecker.RequirePermission("cluster:write"), clusterHandler.Update)
		v1.DELETE("/clusters/:id", permChecker.RequirePermission("cluster:write"), clusterHandler.Delete)
		v1.POST("/clusters/:id/test", permChecker.RequirePermission("cluster:write"), clusterHandler.TestConnection)
		v1.POST("/clusters/:id/activate", permChecker.RequirePermission("cluster:write"), clusterHandler.Activate)
		v1.POST("/clusters/:id/deactivate", permChecker.RequirePermission("cluster:write"), clusterHandler.Deactivate)

		// 监控规则
		monitorRuleHandler := handler.NewMonitorRuleHandler(monitorRuleRepo, clusterRepo, logger)
		v1.GET("/monitor-rules", permChecker.RequirePermission("rule:read"), monitorRuleHandler.List)
		v1.POST("/monitor-rules", permChecker.RequirePermission("rule:write"), monitorRuleHandler.Create)
		v1.PUT("/monitor-rules/:id", permChecker.RequirePermission("rule:write"), monitorRuleHandler.Update)
		v1.DELETE("/monitor-rules/:id", permChecker.RequirePermission("rule:write"), monitorRuleHandler.Delete)
		v1.POST("/monitor-rules/:id/toggle", permChecker.RequirePermission("rule:write"), monitorRuleHandler.Toggle)

		// 用户管理
		userHandler := handler.NewUserHandler(userRepo, settingRepo, logger)
		v1.GET("/users", permChecker.RequirePermission("user:manage"), userHandler.List)
		v1.POST("/users", permChecker.RequirePermission("user:manage"), userHandler.Create)
		v1.POST("/users/:id/toggle-active", permChecker.RequirePermission("user:manage"), userHandler.ToggleActive)
		v1.POST("/users/:id/reset-password", permChecker.RequirePermission("user:manage"), userHandler.ResetPassword)
		v1.PUT("/users/:id/role", permChecker.RequirePermission("user:manage"), userHandler.ChangeRole)

		// 角色管理
		roleHandler := handler.NewRoleHandler(roleRepo, permChecker, logger)
		v1.GET("/roles", permChecker.RequirePermission("role:manage"), roleHandler.List)
		v1.POST("/roles", permChecker.RequirePermission("role:manage"), roleHandler.Create)
		v1.PUT("/roles/:id", permChecker.RequirePermission("role:manage"), roleHandler.Update)
		v1.DELETE("/roles/:id", permChecker.RequirePermission("role:manage"), roleHandler.Delete)
		v1.GET("/permissions", permChecker.RequirePermission("role:manage"), roleHandler.ListPermissions)

		// 系统设置（写操作需要认证 + settings:manage 权限）
		v1.PUT("/settings/branding", permChecker.RequirePermission("settings:manage"), settingHandler.UpdateBranding)
		v1.GET("/settings/security", permChecker.RequirePermission("settings:manage"), settingHandler.GetSecurity)
		v1.PUT("/settings/security", permChecker.RequirePermission("settings:manage"), settingHandler.UpdateSecurity)
		v1.GET("/settings/collect", permChecker.RequirePermission("settings:manage"), settingHandler.GetCollect)
		v1.PUT("/settings/collect", permChecker.RequirePermission("settings:manage"), settingHandler.UpdateCollect)
		v1.POST("/settings/collect/test", permChecker.RequirePermission("settings:manage"), settingHandler.TestCollectConnection)
		v1.GET("/settings/notify", permChecker.RequirePermission("settings:manage"), settingHandler.GetNotify)
		v1.PUT("/settings/notify", permChecker.RequirePermission("settings:manage"), settingHandler.UpdateNotify)
		v1.POST("/settings/notify/test", permChecker.RequirePermission("settings:manage"), settingHandler.TestNotify)

		// SSO 设置管理
		v1.GET("/settings/sso", permChecker.RequirePermission("settings:manage"), settingHandler.GetSSO)
		v1.PUT("/settings/sso", permChecker.RequirePermission("settings:manage"), settingHandler.UpdateSSO)
	}

	return r
}
