package middleware

import (
	"context"
	"log/slog"
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

		status := "success"
		if c.Writer.Status() >= 400 {
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
			IPAddress:     c.ClientIP(),
			UserAgent:     truncateString(c.Request.UserAgent(), 500),
			RequestMethod: method,
			RequestPath:   path,
			Detail:        toString(detail),
			CreatedAt:     time.Now(),
		}

		slog.Debug("audit event captured",
			"module", "middleware_audit",
			"method", method,
			"path", path,
			"resource", resource,
			"action", action,
			"status", status,
			"user_id", auditUserID,
		)

		go func() {
			if err := auditService.Create(context.Background(), auditLog); err != nil {
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

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
