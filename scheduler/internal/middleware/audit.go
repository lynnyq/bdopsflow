package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
)

type auditRouteRule struct {
	Resource string
	Action   string
}

var writeMethodActions = map[string]string{
	"POST":   "create",
	"PUT":    "update",
	"DELETE": "delete",
}

var routeAuditRules = map[string]auditRouteRule{
	"/api/auth/login":           {Resource: "auth", Action: "login"},
	"/api/auth/register":        {Resource: "auth", Action: "register"},
	"/api/auth/change-password": {Resource: "auth", Action: "change_password"},
	"/api/auth/profile":         {Resource: "auth", Action: "update_profile"},
	"/api/admin/users":          {Resource: "user", Action: "create"},
	"/api/admin/roles":          {Resource: "role", Action: "create"},
	"/api/admin/domains":        {Resource: "domain", Action: "create"},
	"/api/admin/system-config":  {Resource: "config", Action: "config_change"},
	"/api/datasources":          {Resource: "datasource", Action: "create"},
	"/api/datasources/test":     {Resource: "datasource", Action: "test_connection"},
	"/api/tasks":                {Resource: "task", Action: "create"},
	"/api/query/execute":        {Resource: "query", Action: "execute"},
	"/api/query/export":         {Resource: "query", Action: "export"},
	"/api/query/saved-sql":      {Resource: "saved_sql", Action: "create"},
	"/api/interfaces":           {Resource: "api_test", Action: "create"},
	"/api/interfaces/execute":   {Resource: "api_test", Action: "execute"},
	"/api/certificates":         {Resource: "certificate", Action: "create"},
	"/api/proto-files":          {Resource: "proto_file", Action: "create"},
	"/api/proto-files/parse":    {Resource: "proto_file", Action: "parse"},
	"/api/proto-files/reflect":  {Resource: "proto_file", Action: "reflect"},
	"/api/proto-files/template": {Resource: "proto_file", Action: "generate_template"},
	"/api/proto-files/fields":   {Resource: "proto_file", Action: "generate_fields"},
	"/api/auth/api-token":       {Resource: "api_token", Action: "generate"},
	"/api/auth/api-token/reveal": {Resource: "api_token", Action: "reveal"},
}

var routePrefixRules = []struct {
	Prefix   string
	Resource string
}{
	{"/api/admin/users/", "user"},
	{"/api/admin/roles/", "role"},
	{"/api/admin/domains/", "domain"},
	{"/api/admin/system-config/", "config"},
	{"/api/datasources/", "datasource"},
	{"/api/tasks/", "task"},
	{"/api/executors/", "executor"},
	{"/api/query/saved-sql/", "saved_sql"},
	{"/api/query/history/", "query_history"},
	{"/api/query/clear-cache/", "datasource"},
	{"/api/query/cancel/", "query"},
	{"/api/logs/", "log"},
	{"/api/interfaces/", "api_test"},
	{"/api/certificates/", "certificate"},
	{"/api/proto-files/", "proto_file"},
	{"/api/auth/api-token/", "api_token"},
}

func AuditMiddleware(auditService *service.AuditLogService) gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		if method != "POST" && method != "PUT" && method != "DELETE" {
			c.Next()
			return
		}

		path := c.Request.URL.Path

		// 用 defer recover 包裹 c.Next()，确保 handler panic 时仍能记录审计日志。
		// panic 时 Gin 的 Recovery 中间件会捕获并返回 500，但若不在此处 recover，
		// 后续审计日志构造和异步写入代码不会执行，导致最重要的故障场景无审计记录。
		defer func() {
			if r := recover(); r != nil {
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()
		c.Next()

		resource, action := resolveAuditInfo(method, path)

		if overrideAction, exists := c.Get("audit_action"); exists {
			if a, ok := overrideAction.(string); ok && a != "" {
				action = a
			}
		}
		if overrideResource, exists := c.Get("audit_resource"); exists {
			if r, ok := overrideResource.(string); ok && r != "" {
				resource = r
			}
		}

		if resource == "" || action == "" {
			return
		}

		userID, _ := c.Get("user_id")
		username, _ := c.Get("username")
		realName, _ := c.Get("real_name")
		role, _ := c.Get("role")
		domainID, _ := c.Get("current_domain_id")

		responseCode := c.Writer.Status()
		status := "success"
		if responseCode >= 400 {
			status = "failure"
		}

		var auditUserID *int64
		if id, ok := userID.(int64); ok && id > 0 {
			auditUserID = &id
		}

		var auditDomainID *int64
		if id, ok := domainID.(int64); ok && id > 0 {
			auditDomainID = &id
		}

		resourceID, _ := c.Get("audit_resource_id")
		resourceName, _ := c.Get("audit_resource_name")
		detail, _ := c.Get("audit_detail")

		auditLog := &model.AuditLog{
			UserID:        auditUserID,
			Username:      toString(username),
			RealName:      toString(realName),
			Role:          toString(role),
			DomainID:      auditDomainID,
			Action:        action,
			Resource:      resource,
			ResourceID:    toString(resourceID),
			ResourceName:  toString(resourceName),
			Status:        status,
			ResponseCode:  responseCode,
			IPAddress:     c.ClientIP(),
			UserAgent:     truncateString(c.Request.UserAgent(), 500),
			RequestMethod: method,
			RequestPath:   path,
			Detail:        truncateString(toString(detail), 2000),
			CreatedAt:     time.Now(),
		}

		slog.Debug("audit event captured",
			"module", "middleware_audit",
			"method", method,
			"path", path,
			"resource", resource,
			"action", action,
			"status", status,
			"response_code", responseCode,
			"user_id", auditUserID,
		)

		// 异步写入审计日志，不阻塞主请求。
		// goroutine 内加 recover，防止 Create panic 导致 goroutine 崩溃且无错误日志。
		go func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("audit log goroutine panicked",
						"module", "middleware_audit",
						"recover", r,
						"action", action,
						"resource", resource,
						"path", path,
					)
				}
			}()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := auditService.Create(ctx, auditLog); err != nil {
				slog.Error("failed to write audit log", "module", "middleware_audit", "error", err, "action", action, "resource", resource)
			}
		}()
	}
}

func resolveAuditInfo(method, path string) (resource, action string) {
	if rule, ok := routeAuditRules[path]; ok {
		return rule.Resource, rule.Action
	}

	for _, prefix := range routePrefixRules {
		if strings.HasPrefix(path, prefix.Prefix) {
			resource = prefix.Resource
			break
		}
	}

	if action == "" {
		if a, ok := writeMethodActions[method]; ok {
			action = a
		}
	}

	if strings.Contains(path, "/trigger") {
		action = "trigger"
	} else if (strings.HasSuffix(path, "/test") || strings.Contains(path, "/test")) && !strings.Contains(path, "/executors/") {
		action = "test_connection"
	} else if strings.Contains(path, "/permissions") && method == "POST" {
		action = "assign"
	} else if strings.Contains(path, "/permissions") && method == "DELETE" {
		action = "revoke"
	} else if strings.Contains(path, "/roles") && method == "POST" {
		action = "assign"
	} else if strings.Contains(path, "/domains") && method == "POST" {
		action = "assign"
	} else if strings.Contains(path, "/reset-password") {
		action = "reset_password"
	} else if strings.HasSuffix(path, "/online") {
		action = "online"
	} else if strings.HasSuffix(path, "/offline") {
		action = "offline"
	} else if strings.Contains(path, "/capacity") {
		action = "update"
	} else if strings.Contains(path, "/batch-delete") {
		action = "delete"
	} else if strings.Contains(path, "/clean") {
		action = "clean"
	} else if strings.Contains(path, "/clear-cache") {
		action = "clear_cache"
	} else if strings.Contains(path, "/cancel") {
		action = "cancel"
	} else if strings.Contains(path, "/pause") {
		action = "pause"
	} else if strings.Contains(path, "/resume") {
		action = "resume"
	} else if strings.Contains(path, "/system-config") {
		action = "config_change"
	} else if strings.Contains(path, "/interfaces/") && strings.Contains(path, "/execute") {
		action = "execute"
	} else if strings.Contains(path, "/generate-curl") {
		action = "generate_curl"
	} else if strings.Contains(path, "/interfaces/") && strings.Contains(path, "/results") && method == "DELETE" {
		action = "delete_result"
	} else if strings.Contains(path, "/api-token") && method == "DELETE" {
		action = "revoke"
	}

	return resource, action
}

// toString 将 gin.Context 中的任意值转为字符串。
// 支持非 string 类型（如 int、float），避免 handler 设置 int 类型的 resource_id 时丢失。
func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// truncateString 按 rune 截断字符串，避免截断 UTF-8 多字节字符产生乱码。
func truncateString(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes])
}
