package middleware

import (
	"testing"
)

func TestResolveAuditInfo(t *testing.T) {
	tests := []struct {
		method           string
		path             string
		expectedResource string
		expectedAction   string
	}{
		{"POST", "/api/auth/login", "auth", "login"},
		{"POST", "/api/auth/register", "auth", "register"},
		{"POST", "/api/admin/users", "user", "create"},
		{"PUT", "/api/admin/users/1", "user", "update"},
		{"DELETE", "/api/admin/users/1", "user", "delete"},
		{"POST", "/api/admin/roles", "role", "create"},
		{"PUT", "/api/admin/roles/1", "role", "update"},
		{"DELETE", "/api/admin/roles/1", "role", "delete"},
		{"POST", "/api/admin/domains", "domain", "create"},
		{"PUT", "/api/admin/domains/1", "domain", "update"},
		{"DELETE", "/api/admin/domains/1", "domain", "delete"},
		{"PUT", "/api/admin/system-config/datasource.default_limit", "config", "config_change"},
		{"POST", "/api/datasources", "datasource", "create"},
		{"PUT", "/api/datasources/1", "datasource", "update"},
		{"DELETE", "/api/datasources/1", "datasource", "delete"},
		{"POST", "/api/datasources/test", "datasource", "test_connection"},
		{"POST", "/api/tasks", "task", "create"},
		{"PUT", "/api/tasks/1", "task", "update"},
		{"DELETE", "/api/tasks/1", "task", "delete"},
		{"POST", "/api/tasks/1/trigger", "task", "trigger"},
		{"POST", "/api/workflows", "workflow", "create"},
		{"PUT", "/api/workflows/1", "workflow", "update"},
		{"DELETE", "/api/workflows/1", "workflow", "delete"},
		{"POST", "/api/workflows/1/trigger", "workflow", "trigger"},
		{"POST", "/api/query/execute", "query", "execute"},
		{"POST", "/api/query/export", "query", "export"},
		{"POST", "/api/datasources/1/permissions", "datasource", "assign"},
		{"DELETE", "/api/datasources/1/permissions/1", "datasource", "revoke"},
		{"POST", "/api/admin/users/1/reset-password", "user", "reset_password"},
		{"POST", "/api/executors/test/online", "executor", "online"},
		{"POST", "/api/executors/test/offline", "executor", "offline"},
		{"PUT", "/api/executors/test/capacity", "executor", "update"},
		{"DELETE", "/api/executors/test", "executor", "delete"},
		{"POST", "/api/query/saved-sql", "saved_sql", "create"},
		{"DELETE", "/api/query/saved-sql/1", "saved_sql", "delete"},
		{"POST", "/api/logs/batch-delete", "log", "delete"},
		{"POST", "/api/admin/audit-logs/clean", "", "clean"},
		{"POST", "/api/dashboard/scheduler/pause", "", "pause"},
		{"POST", "/api/dashboard/scheduler/resume", "", "resume"},
	}

	for _, tt := range tests {
		t.Run(tt.method+"_"+tt.path, func(t *testing.T) {
			resource, action := resolveAuditInfo(tt.method, tt.path)
			if resource != tt.expectedResource {
				t.Errorf("resolveAuditInfo(%s, %s) resource = %q, want %q", tt.method, tt.path, resource, tt.expectedResource)
			}
			if action != tt.expectedAction {
				t.Errorf("resolveAuditInfo(%s, %s) action = %q, want %q", tt.method, tt.path, action, tt.expectedAction)
			}
		})
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input   string
		maxLen  int
		want    string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello"},
		{"", 10, ""},
		{"abc", 3, "abc"},
	}

	for _, tt := range tests {
		result := truncateString(tt.input, tt.maxLen)
		if result != tt.want {
			t.Errorf("truncateString(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.want)
		}
	}
}

func TestToString(t *testing.T) {
	tests := []struct {
		input interface{}
		want  string
	}{
		{nil, ""},
		{"hello", "hello"},
		{123, ""},
		{"", ""},
	}

	for _, tt := range tests {
		result := toString(tt.input)
		if result != tt.want {
			t.Errorf("toString(%v) = %q, want %q", tt.input, result, tt.want)
		}
	}
}
