package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lynnyq/bdopsflow/scheduler/internal/datasource"
	"github.com/lynnyq/bdopsflow/scheduler/internal/handler"
	"github.com/lynnyq/bdopsflow/scheduler/internal/middleware"
	"github.com/lynnyq/bdopsflow/scheduler/web"
)

func setupRoutes(router *gin.Engine, app *App) {
	router.GET("/health", func(c *gin.Context) {
		result := app.schedulerService.HealthCheck(c.Request.Context())
		healthData := map[string]interface{}{
			"status":    result.Status,
			"timestamp": result.Timestamp,
			"node_id":   app.nodeID,
			"is_leader": app.leaderElection.IsLeader(),
		}
		if result.Components != nil {
			healthData["components"] = result.Components
		}
		if result.Status == "healthy" {
			c.JSON(http.StatusOK, healthData)
		} else {
			c.JSON(http.StatusServiceUnavailable, healthData)
		}
	})

	authHandler := handler.NewAuthHandler(app.logDB, app.permissionService, app.rsaUtil, app.cfg.SSOEnabled, app.cfg.SSOUrl, app.ssoRsaUtil, app.cfg.SSOTimeout)
	userAdminHandler := handler.NewUserAdminHandler(app.userAdminService)
	router.POST("/api/auth/login", middleware.AuditMiddleware(app.auditLogService), authHandler.Login)
	router.POST("/api/auth/sso-login", middleware.AuditMiddleware(app.auditLogService), authHandler.SSOLogin)
	if app.cfg.AllowRegister {
		router.POST("/api/auth/register", middleware.AuditMiddleware(app.auditLogService), authHandler.Register)
	}
	router.GET("/api/auth/public-key", authHandler.GetPublicKey)
	router.POST("/api/auth/refresh", authHandler.RefreshToken)

	wecomHandler := handler.NewWeComHandler(app.dsConfigService)
	router.POST("/api/wecom/:wx_group_id", wecomHandler.SendWeComMessage)

	protected := router.Group("/api")
	protected.Use(middleware.JWTAuthMiddleware())
	protected.Use(middleware.InjectUserRole(app.permissionService))
	protected.Use(middleware.AuditMiddleware(app.auditLogService))
	{
		protected.GET("/auth/current", authHandler.GetCurrentUser)
		protected.PUT("/auth/profile", userAdminHandler.UpdateCurrentUser)
		protected.POST("/auth/change-password", userAdminHandler.ChangePassword)
		protected.POST("/auth/switch-domain", authHandler.SwitchDomain)

		taskHandler := handler.NewTaskHandler(app.schedulerService)
		tasks := protected.Group("/tasks")
		{
			tasks.GET("", taskHandler.List)
			tasks.POST("", middleware.RequirePermission(app.permissionService, "task", "create"), taskHandler.Create)
			tasks.GET("/:id", taskHandler.Get)
			tasks.PUT("/:id", middleware.RequirePermission(app.permissionService, "task", "update"), taskHandler.Update)
			tasks.DELETE("/:id", middleware.RequirePermission(app.permissionService, "task", "delete"), taskHandler.Delete)
			tasks.POST("/:id/trigger", middleware.RequirePermission(app.permissionService, "task", "trigger"), taskHandler.Trigger)
			tasks.GET("/:id/executions", taskHandler.Executions)
			tasks.GET("/executions/:execution_id/logs", taskHandler.ExecutionLogs)
		}

		executorHandler := handler.NewExecutorHandler(app.schedulerService)
		executorDomainHandler := handler.NewExecutorDomainHandler(app.executorDomainService, app.permissionService, app.userAdminService)
		executors := protected.Group("/executors")
		{
			executors.GET("", executorDomainHandler.GetExecutorsWithDomains)
			executors.GET("/:name", executorHandler.Get)
			executors.GET("/:name/domains", middleware.RequireSystemAdmin(app.permissionService), executorDomainHandler.GetExecutorDomains)
			executors.POST("/:name/domains", middleware.RequireSystemAdmin(app.permissionService), executorDomainHandler.AssignDomains)
			executors.DELETE("/:name/domains/:domain_id", middleware.RequireSystemAdmin(app.permissionService), executorDomainHandler.RemoveDomain)
			executors.GET("/:name/tasks", middleware.RequirePermission(app.permissionService, "executor", "read"), executorDomainHandler.GetAssignedTasks)
			executors.GET("/:name/can-delete", executorDomainHandler.CanDeleteExecutor)
			executors.POST("/:name/online", middleware.RequirePermission(app.permissionService, "executor", "online"), executorHandler.Online)
			executors.POST("/:name/offline", middleware.RequirePermission(app.permissionService, "executor", "offline"), executorHandler.Offline)
			executors.PUT("/:name/capacity", middleware.RequirePermission(app.permissionService, "executor", "manage"), executorHandler.UpdateCapacity)
			executors.DELETE("/:name", middleware.RequirePermission(app.permissionService, "executor", "delete"), executorHandler.Delete)
		}

		logHandler := handler.NewLogHandler(app.schedulerService)
		logs := protected.Group("/logs")
		{
			logs.GET("", logHandler.List)
			logs.GET("/stats", logHandler.GetStats)
			logs.DELETE("/:id", middleware.RequirePermission(app.permissionService, "log", "delete"), logHandler.Delete)
			logs.POST("/batch-delete", middleware.RequirePermission(app.permissionService, "log", "delete"), logHandler.BatchDelete)
		}

		protected.GET("/logs/stream", taskHandler.StreamLogs)

		dashboardHandler := handler.NewDashboardHandler(app.schedulerService)
		dashboard := protected.Group("/dashboard")
		{
			dashboard.GET("/stats", dashboardHandler.GetStats)
			dashboard.GET("/trends", dashboardHandler.GetTrends)
			dashboard.GET("/scheduler/status", dashboardHandler.GetSchedulerStatus)
			dashboard.POST("/scheduler/pause", middleware.RequireSystemAdmin(app.permissionService), dashboardHandler.PauseScheduler)
			dashboard.POST("/scheduler/resume", middleware.RequireSystemAdmin(app.permissionService), dashboardHandler.ResumeScheduler)
			dashboard.GET("/health", dashboardHandler.HealthCheck)
		}

		admin := protected.Group("/admin")
		{
			permissionHandler := handler.NewPermissionHandler(app.permissionService)
			admin.GET("/permissions", middleware.RequirePermission(app.permissionService, "permission", "read"), permissionHandler.GetAllPermissions)

			admin.GET("/users", middleware.RequirePermission(app.permissionService, "user", "read"), userAdminHandler.ListUsers)
			admin.GET("/users/by-domain", middleware.RequirePermission(app.permissionService, "user", "read"), userAdminHandler.ListUsersByDomain)
			admin.GET("/users/:id", middleware.RequirePermission(app.permissionService, "user", "read"), userAdminHandler.GetUser)
			admin.POST("/users", middleware.RequirePermission(app.permissionService, "user", "manage"), userAdminHandler.CreateUser)
			admin.PUT("/users/:id", middleware.RequirePermission(app.permissionService, "user", "update"), userAdminHandler.UpdateUser)
			admin.DELETE("/users/:id", middleware.RequirePermission(app.permissionService, "user", "manage"), userAdminHandler.DeleteUser)
			admin.POST("/users/:id/roles", middleware.RequirePermission(app.permissionService, "user", "manage"), userAdminHandler.AssignUserRoles)
			admin.GET("/users/:id/roles", middleware.RequirePermission(app.permissionService, "user", "read"), userAdminHandler.GetUserRoles)
			admin.POST("/users/:id/domains", middleware.RequirePermission(app.permissionService, "user", "manage"), userAdminHandler.AssignUserDomains)
			admin.POST("/users/:id/reset-password", middleware.RequirePermission(app.permissionService, "user", "update"), userAdminHandler.ResetUserPassword)

			roleAdminHandler := handler.NewRoleAdminHandler(app.roleAdminService)
			admin.GET("/roles", middleware.RequirePermission(app.permissionService, "role", "read"), roleAdminHandler.ListRoles)
			admin.GET("/roles/:id", middleware.RequirePermission(app.permissionService, "role", "read"), roleAdminHandler.GetRole)
			admin.POST("/roles", middleware.RequirePermission(app.permissionService, "role", "manage"), roleAdminHandler.CreateRole)
			admin.PUT("/roles/:id", middleware.RequirePermission(app.permissionService, "role", "manage"), roleAdminHandler.UpdateRole)
			admin.DELETE("/roles/:id", middleware.RequirePermission(app.permissionService, "role", "manage"), roleAdminHandler.DeleteRole)
			admin.GET("/roles/:id/permissions", middleware.RequirePermission(app.permissionService, "role", "read"), roleAdminHandler.GetRolePermissions)
			admin.POST("/roles/:id/permissions", middleware.RequirePermission(app.permissionService, "role", "manage"), roleAdminHandler.AssignPermissions)

			domainAdminHandler := handler.NewDomainAdminHandler(app.domainAdminService)
			admin.GET("/domains", middleware.RequirePermission(app.permissionService, "domain", "read"), domainAdminHandler.ListDomains)
			admin.GET("/domains/:id", middleware.RequirePermission(app.permissionService, "domain", "read"), domainAdminHandler.GetDomain)
			admin.POST("/domains", middleware.RequireSystemAdmin(app.permissionService), domainAdminHandler.CreateDomain)
			admin.PUT("/domains/:id", middleware.RequirePermission(app.permissionService, "domain", "update"), domainAdminHandler.UpdateDomain)
			admin.DELETE("/domains/:id", middleware.RequireSystemAdmin(app.permissionService), domainAdminHandler.DeleteDomain)

			systemConfigHandler := handler.NewSystemConfigHandler(app.dsConfigService)
			admin.GET("/system-config", middleware.RequireSystemAdmin(app.permissionService), systemConfigHandler.List)
			admin.PUT("/system-config/:key", middleware.RequireSystemAdmin(app.permissionService), systemConfigHandler.Update)

			auditLogHandler := handler.NewAuditLogHandler(app.auditLogService)
			admin.GET("/audit-logs", middleware.RequireSystemAdmin(app.permissionService), auditLogHandler.List)
			admin.GET("/audit-logs/stats", middleware.RequireSystemAdmin(app.permissionService), auditLogHandler.GetStats)
			admin.POST("/audit-logs/clean", middleware.RequireSystemAdmin(app.permissionService), auditLogHandler.CleanExpired)
			admin.GET("/audit-logs/retention", middleware.RequireSystemAdmin(app.permissionService), auditLogHandler.GetRetentionDays)
			admin.PUT("/audit-logs/retention", middleware.RequireSystemAdmin(app.permissionService), auditLogHandler.UpdateRetentionDays)
		}

		webhookHandler := handler.NewWebhookHandler(app.webhookSvc)
		webhooks := protected.Group("/webhooks")
		{
			webhooks.GET("", middleware.RequirePermission(app.permissionService, "webhook", "read"), webhookHandler.List)
			webhooks.POST("", middleware.RequirePermission(app.permissionService, "webhook", "create"), webhookHandler.Create)
			webhooks.PUT("/:id", middleware.RequirePermission(app.permissionService, "webhook", "update"), webhookHandler.Update)
			webhooks.DELETE("/:id", middleware.RequirePermission(app.permissionService, "webhook", "delete"), webhookHandler.Delete)
			webhooks.POST("/:id/test", middleware.RequirePermission(app.permissionService, "webhook", "create"), webhookHandler.Test)
		}

		dsHandler := handler.NewDatasourceHandler(app.dsService, app.dsManager, app.dsConfigService, app.instancePermSvc, app.permissionService, app.domainAdminService)
		queryHandler := handler.NewQueryHandler(app.dsService, app.dsManager, app.dsConfigService, app.dsCacheService, app.dsConcurrentService)

		datasources := protected.Group("/datasources")
		{
			datasources.GET("", middleware.RequirePermission(app.permissionService, "datasource", "read"), dsHandler.List)
			datasources.GET("/types", dsHandler.SupportedTypes)
			datasources.POST("/test", middleware.RequirePermission(app.permissionService, "datasource", "create"), dsHandler.TestConnectionByParams)
			datasources.POST("", middleware.RequirePermission(app.permissionService, "datasource", "create"), dsHandler.Create)
			datasources.GET("/:id", middleware.DatasourcePermissionMiddleware(app.instancePermSvc, "read"), dsHandler.Get)
			datasources.PUT("/:id", middleware.DatasourcePermissionMiddleware(app.instancePermSvc, "update"), dsHandler.Update)
			datasources.DELETE("/:id", middleware.DatasourcePermissionMiddleware(app.instancePermSvc, "delete"), dsHandler.Delete)
			datasources.POST("/:id/test", middleware.DatasourcePermissionMiddleware(app.instancePermSvc, "read"), dsHandler.TestConnection)
			datasources.POST("/:id/permissions", middleware.DatasourcePermissionMiddleware(app.instancePermSvc, "manage"), dsHandler.GrantPermission)
			datasources.PUT("/:id/permissions/:perm_id", middleware.DatasourcePermissionMiddleware(app.instancePermSvc, "manage"), dsHandler.UpdatePermission)
			datasources.DELETE("/:id/permissions/:perm_id", middleware.DatasourcePermissionMiddleware(app.instancePermSvc, "manage"), dsHandler.RevokePermission)
			datasources.GET("/:id/permissions", middleware.DatasourcePermissionMiddleware(app.instancePermSvc, "manage"), dsHandler.GetPermissions)
			datasources.GET("/:id/metadata", middleware.DatasourcePermissionMiddleware(app.instancePermSvc, "query"), queryHandler.GetMetadata)
		}

		query := protected.Group("/query")
		{
			query.POST("/execute", middleware.DatasourcePermissionMiddleware(app.instancePermSvc, "query"), queryHandler.Execute)
			query.GET("/result/:query_id", queryHandler.GetResult)
			query.POST("/cancel/:query_id", queryHandler.Cancel)
			query.POST("/export", middleware.DatasourcePermissionMiddleware(app.instancePermSvc, "download"), queryHandler.ExportCSV)
			query.GET("/history", queryHandler.GetHistory)
			query.DELETE("/history/:id", queryHandler.DeleteQueryHistory)
			query.POST("/history/batch-delete", queryHandler.BatchDeleteQueryHistory)
			query.GET("/saved-sql", queryHandler.ListSavedSQL)
			query.POST("/saved-sql", queryHandler.SaveSQL)
			query.DELETE("/saved-sql/:id", queryHandler.DeleteSavedSQL)
		}
	}

	setupStaticRoutes(router, app.dsConfigService)
}

func setupStaticRoutes(router *gin.Engine, dsConfigService *datasource.ConfigService) {
	webEnabled := func() bool {
		return dsConfigService.GetBool("web.enabled")
	}

	hasWebAssets := func() bool {
		staticFS, _ := web.GetStaticFS()
		if staticFS == nil {
			return false
		}
		_, err := staticFS.Open("index.html")
		return err == nil
	}

	router.GET("/", func(c *gin.Context) {
		if !webEnabled() {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"status":  "error",
				"message": "Not found. Built-in web service is disabled.",
			})
			return
		}
		if !hasWebAssets() {
			c.HTML(http.StatusOK, "", `
<!DOCTYPE html>
<html>
<head>
    <title>BDopsFlow</title>
    <style>
        body { font-family: system-ui, -apple-system, sans-serif; max-width: 600px; margin: 100px auto; padding: 0 20px; }
        h1 { color: #333; }
        .info { background: #f5f5f5; padding: 20px; border-radius: 8px; margin-top: 20px; }
    </style>
</head>
<body>
    <h1>BDopsFlow</h1>
    <div class="info">
        <p>Built-in web UI is not available.</p>
        <p>Please run <code>make build-frontend</code> to build the frontend first.</p>
        <p>Or use <code>make run-dev</code> for development mode.</p>
    </div>
</body>
</html>`)
			return
		}
		staticFS, _ := web.GetStaticFS()
		http.FileServer(staticFS).ServeHTTP(c.Writer, c.Request)
	})

	router.GET("/assets/*filepath", func(c *gin.Context) {
		if !webEnabled() {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"status":  "error",
				"message": "Not found. Built-in web service is disabled.",
			})
			return
		}
		if !hasWebAssets() {
			c.String(http.StatusNotFound, "Not found")
			return
		}
		staticFS, _ := web.GetStaticFS()
		http.FileServer(staticFS).ServeHTTP(c.Writer, c.Request)
	})

	router.GET("/favicon.ico", func(c *gin.Context) {
		if !webEnabled() {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"status":  "error",
				"message": "Not found. Built-in web service is disabled.",
			})
			return
		}
		if !hasWebAssets() {
			c.String(http.StatusNotFound, "Not found")
			return
		}
		staticFS, _ := web.GetStaticFS()
		http.FileServer(staticFS).ServeHTTP(c.Writer, c.Request)
	})

	router.NoRoute(func(c *gin.Context) {
		if len(c.Request.URL.Path) >= 4 && c.Request.URL.Path[:4] == "/api" {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"status":  "error",
				"message": "API endpoint not found",
			})
			return
		}
		if c.Request.URL.Path == "/health" {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"status":  "error",
				"message": "Not found",
			})
			return
		}

		if !webEnabled() {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"status":  "error",
				"message": "Not found. Built-in web service is disabled.",
			})
			return
		}

		if !hasWebAssets() {
			c.HTML(http.StatusOK, "", `
<!DOCTYPE html>
<html>
<head>
    <title>BDopsFlow</title>
    <style>
        body { font-family: system-ui, -apple-system, sans-serif; max-width: 600px; margin: 100px auto; padding: 0 20px; }
        h1 { color: #333; }
        .info { background: #f5f5f5; padding: 20px; border-radius: 8px; margin-top: 20px; }
    </style>
</head>
<body>
    <h1>BDopsFlow</h1>
    <div class="info">
        <p>Built-in web UI is not available.</p>
        <p>Please run <code>make build-frontend</code> to build the frontend first.</p>
        <p>Or use <code>make run-dev</code> for development mode.</p>
    </div>
</body>
</html>`)
			return
		}

		staticFS, _ := web.GetStaticFS()
		file, err := staticFS.Open("index.html")
		if err != nil {
			c.String(http.StatusNotFound, "Page not found")
			return
		}
		defer file.Close()

		http.ServeContent(c.Writer, c.Request, "index.html", time.Now(), file)
	})
}
