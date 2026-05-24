package handler

import "testing"

func TestCodeSuccessIsZero(t *testing.T) {
	if CodeSuccess != 0 {
		t.Errorf("CodeSuccess should be 0, got %d", CodeSuccess)
	}
}

func TestCodeConstantsNonZero(t *testing.T) {
	codes := map[string]int{
		"CodeBadRequest":          CodeBadRequest,
		"CodeUnauthorized":        CodeUnauthorized,
		"CodeForbidden":           CodeForbidden,
		"CodeNotFound":            CodeNotFound,
		"CodeConflict":            CodeConflict,
		"CodeInternalError":       CodeInternalError,
		"CodeDatabaseError":       CodeDatabaseError,
		"CodeRedisError":          CodeRedisError,
		"CodeTaskRunning":         CodeTaskRunning,
		"CodeTaskLocked":          CodeTaskLocked,
		"CodeTaskNotFound":        CodeTaskNotFound,
		"CodeExecutorNotFound":    CodeExecutorNotFound,
		"CodeExecutorOffline":     CodeExecutorOffline,
		"CodeExecutorNoCapacity":  CodeExecutorNoCapacity,
		"CodeNoAvailableExecutor": CodeNoAvailableExecutor,
		"CodeDispatchFailed":      CodeDispatchFailed,
		"CodeUserNotFound":        CodeUserNotFound,
		"CodeUserExists":          CodeUserExists,
		"CodeInvalidCredentials":  CodeInvalidCredentials,
		"CodeUserInactive":        CodeUserInactive,
		"CodeWrongPassword":       CodeWrongPassword,
		"CodePasswordWeak":        CodePasswordWeak,
		"CodeRoleNotFound":        CodeRoleNotFound,
		"CodeRoleExists":          CodeRoleExists,
		"CodeRoleSystemProtected": CodeRoleSystemProtected,
		"CodeDomainNotFound":      CodeDomainNotFound,
		"CodeDomainHasResources":  CodeDomainHasResources,
		"CodePermissionDenied":    CodePermissionDenied,
		"CodeWorkflowNotFound":    CodeWorkflowNotFound,
		"CodeDatasourceNotFound":  CodeDatasourceNotFound,
		"CodeDatasourceExists":    CodeDatasourceExists,
		"CodeDatasourceConnectFailed": CodeDatasourceConnectFailed,
		"CodeDatasourceNameExists":    CodeDatasourceNameExists,
		"CodeQueryError":          CodeQueryError,
		"CodeConcurrentLimit":     CodeConcurrentLimit,
		"CodeQueryNoDatasource":   CodeQueryNoDatasource,
		"CodeQueryDisabled":       CodeQueryDisabled,
		"CodeQueryConnectFailed":  CodeQueryConnectFailed,
		"CodeQuerySelectOnly":     CodeQuerySelectOnly,
		"CodeQueryTimeout":        CodeQueryTimeout,
		"CodeQueryHistoryNotFound": CodeQueryHistoryNotFound,
		"CodeSavedSQLNotFound":    CodeSavedSQLNotFound,
		"CodePermissionExists":    CodePermissionExists,
	}

	for name, code := range codes {
		if code == 0 {
			t.Errorf("%s should be non-zero, got 0", name)
		}
	}
}

func TestCodeRangesNoOverlap(t *testing.T) {
	allCodes := []int{
		CodeSuccess,
		CodeBadRequest,
		CodeUnauthorized,
		CodeForbidden,
		CodeNotFound,
		CodeConflict,
		CodeInternalError,
		CodeDatabaseError,
		CodeRedisError,
		CodeTaskRunning,
		CodeTaskLocked,
		CodeTaskNotFound,
		CodeExecutorNotFound,
		CodeExecutorOffline,
		CodeExecutorNoCapacity,
		CodeNoAvailableExecutor,
		CodeDispatchFailed,
		CodeUserNotFound,
		CodeUserExists,
		CodeInvalidCredentials,
		CodeUserInactive,
		CodeWrongPassword,
		CodePasswordWeak,
		CodeRoleNotFound,
		CodeRoleExists,
		CodeRoleSystemProtected,
		CodeDomainNotFound,
		CodeDomainHasResources,
		CodePermissionDenied,
		CodeWorkflowNotFound,
		CodeDatasourceNotFound,
		CodeDatasourceExists,
		CodeDatasourceConnectFailed,
		CodeDatasourceNameExists,
		CodeQueryError,
		CodeConcurrentLimit,
		CodeQueryNoDatasource,
		CodeQueryDisabled,
		CodeQueryConnectFailed,
		CodeQuerySelectOnly,
		CodeQueryTimeout,
		CodeQueryHistoryNotFound,
		CodeSavedSQLNotFound,
		CodePermissionExists,
	}

	seen := make(map[int]string)
	names := []string{
		"CodeSuccess",
		"CodeBadRequest",
		"CodeUnauthorized",
		"CodeForbidden",
		"CodeNotFound",
		"CodeConflict",
		"CodeInternalError",
		"CodeDatabaseError",
		"CodeRedisError",
		"CodeTaskRunning",
		"CodeTaskLocked",
		"CodeTaskNotFound",
		"CodeExecutorNotFound",
		"CodeExecutorOffline",
		"CodeExecutorNoCapacity",
		"CodeNoAvailableExecutor",
		"CodeDispatchFailed",
		"CodeUserNotFound",
		"CodeUserExists",
		"CodeInvalidCredentials",
		"CodeUserInactive",
		"CodeWrongPassword",
		"CodePasswordWeak",
		"CodeRoleNotFound",
		"CodeRoleExists",
		"CodeRoleSystemProtected",
		"CodeDomainNotFound",
		"CodeDomainHasResources",
		"CodePermissionDenied",
		"CodeWorkflowNotFound",
		"CodeDatasourceNotFound",
		"CodeDatasourceExists",
		"CodeDatasourceConnectFailed",
		"CodeDatasourceNameExists",
		"CodeQueryError",
		"CodeConcurrentLimit",
		"CodeQueryNoDatasource",
		"CodeQueryDisabled",
		"CodeQueryConnectFailed",
		"CodeQuerySelectOnly",
		"CodeQueryTimeout",
		"CodeQueryHistoryNotFound",
		"CodeSavedSQLNotFound",
		"CodePermissionExists",
	}

	for i, code := range allCodes {
		name := names[i]
		if existing, ok := seen[code]; ok {
			t.Errorf("code %d used by both %s and %s", code, existing, name)
		}
		seen[code] = name
	}
}

func TestHTTPCodesMatchStandard(t *testing.T) {
	if CodeBadRequest != 400 {
		t.Errorf("CodeBadRequest should be 400, got %d", CodeBadRequest)
	}
	if CodeUnauthorized != 401 {
		t.Errorf("CodeUnauthorized should be 401, got %d", CodeUnauthorized)
	}
	if CodeForbidden != 403 {
		t.Errorf("CodeForbidden should be 403, got %d", CodeForbidden)
	}
	if CodeNotFound != 404 {
		t.Errorf("CodeNotFound should be 404, got %d", CodeNotFound)
	}
	if CodeConflict != 409 {
		t.Errorf("CodeConflict should be 409, got %d", CodeConflict)
	}
	if CodeInternalError != 500 {
		t.Errorf("CodeInternalError should be 500, got %d", CodeInternalError)
	}
}

func TestTaskCodesInRange(t *testing.T) {
	taskCodes := map[string]int{
		"CodeTaskRunning":         CodeTaskRunning,
		"CodeTaskLocked":          CodeTaskLocked,
		"CodeTaskNotFound":        CodeTaskNotFound,
		"CodeExecutorNotFound":    CodeExecutorNotFound,
		"CodeExecutorOffline":     CodeExecutorOffline,
		"CodeExecutorNoCapacity":  CodeExecutorNoCapacity,
		"CodeNoAvailableExecutor": CodeNoAvailableExecutor,
		"CodeDispatchFailed":      CodeDispatchFailed,
	}

	for name, code := range taskCodes {
		if code < 10001 || code > 10099 {
			t.Errorf("%s = %d, expected to be in range 10001-10099", name, code)
		}
	}
}

func TestUserCodesInRange(t *testing.T) {
	userCodes := map[string]int{
		"CodeUserNotFound":       CodeUserNotFound,
		"CodeUserExists":         CodeUserExists,
		"CodeInvalidCredentials": CodeInvalidCredentials,
		"CodeUserInactive":       CodeUserInactive,
		"CodeWrongPassword":      CodeWrongPassword,
		"CodePasswordWeak":       CodePasswordWeak,
	}

	for name, code := range userCodes {
		if code < 11001 || code > 11099 {
			t.Errorf("%s = %d, expected to be in range 11001-11099", name, code)
		}
	}
}

func TestRoleCodesInRange(t *testing.T) {
	roleCodes := map[string]int{
		"CodeRoleNotFound":        CodeRoleNotFound,
		"CodeRoleExists":          CodeRoleExists,
		"CodeRoleSystemProtected": CodeRoleSystemProtected,
	}

	for name, code := range roleCodes {
		if code < 12001 || code > 12099 {
			t.Errorf("%s = %d, expected to be in range 12001-12099", name, code)
		}
	}
}

func TestDomainCodesInRange(t *testing.T) {
	domainCodes := map[string]int{
		"CodeDomainNotFound":     CodeDomainNotFound,
		"CodeDomainHasResources": CodeDomainHasResources,
	}

	for name, code := range domainCodes {
		if code < 13001 || code > 13099 {
			t.Errorf("%s = %d, expected to be in range 13001-13099", name, code)
		}
	}
}

func TestPermissionCodesInRange(t *testing.T) {
	permCodes := map[string]int{
		"CodePermissionDenied": CodePermissionDenied,
		"CodePermissionExists": CodePermissionExists,
	}

	for name, code := range permCodes {
		if code < 14001 || code > 14099 {
			t.Errorf("%s = %d, expected to be in range 14001-14099", name, code)
		}
	}
}

func TestWorkflowCodesInRange(t *testing.T) {
	if CodeWorkflowNotFound < 15001 || CodeWorkflowNotFound > 15099 {
		t.Errorf("CodeWorkflowNotFound = %d, expected to be in range 15001-15099", CodeWorkflowNotFound)
	}
}

func TestDatasourceCodesInRange(t *testing.T) {
	dsCodes := map[string]int{
		"CodeDatasourceNotFound":     CodeDatasourceNotFound,
		"CodeDatasourceExists":       CodeDatasourceExists,
		"CodeDatasourceConnectFailed": CodeDatasourceConnectFailed,
		"CodeDatasourceNameExists":   CodeDatasourceNameExists,
	}

	for name, code := range dsCodes {
		if code < 16001 || code > 16099 {
			t.Errorf("%s = %d, expected to be in range 16001-16099", name, code)
		}
	}
}

func TestQueryCodesInRange(t *testing.T) {
	queryCodes := map[string]int{
		"CodeQueryError":          CodeQueryError,
		"CodeConcurrentLimit":     CodeConcurrentLimit,
		"CodeQueryNoDatasource":   CodeQueryNoDatasource,
		"CodeQueryDisabled":       CodeQueryDisabled,
		"CodeQueryConnectFailed":  CodeQueryConnectFailed,
		"CodeQuerySelectOnly":     CodeQuerySelectOnly,
		"CodeQueryTimeout":        CodeQueryTimeout,
		"CodeQueryHistoryNotFound": CodeQueryHistoryNotFound,
		"CodeSavedSQLNotFound":    CodeSavedSQLNotFound,
	}

	for name, code := range queryCodes {
		if code < 17001 || code > 17099 {
			t.Errorf("%s = %d, expected to be in range 17001-17099", name, code)
		}
	}
}

func TestInfrastructureCodesInRange(t *testing.T) {
	infraCodes := map[string]int{
		"CodeDatabaseError": CodeDatabaseError,
		"CodeRedisError":    CodeRedisError,
	}

	for name, code := range infraCodes {
		if code < 5001 || code > 5099 {
			t.Errorf("%s = %d, expected to be in range 5001-5099", name, code)
		}
	}
}

func TestCodeExactValues(t *testing.T) {
	tests := []struct {
		name  string
		code  int
		value int
	}{
		{"CodeSuccess", CodeSuccess, 0},
		{"CodeBadRequest", CodeBadRequest, 400},
		{"CodeUnauthorized", CodeUnauthorized, 401},
		{"CodeForbidden", CodeForbidden, 403},
		{"CodeNotFound", CodeNotFound, 404},
		{"CodeConflict", CodeConflict, 409},
		{"CodeInternalError", CodeInternalError, 500},
		{"CodeDatabaseError", CodeDatabaseError, 5001},
		{"CodeRedisError", CodeRedisError, 5002},
		{"CodeTaskRunning", CodeTaskRunning, 10001},
		{"CodeTaskLocked", CodeTaskLocked, 10002},
		{"CodeTaskNotFound", CodeTaskNotFound, 10003},
		{"CodeExecutorNotFound", CodeExecutorNotFound, 10004},
		{"CodeExecutorOffline", CodeExecutorOffline, 10005},
		{"CodeExecutorNoCapacity", CodeExecutorNoCapacity, 10006},
		{"CodeNoAvailableExecutor", CodeNoAvailableExecutor, 10007},
		{"CodeDispatchFailed", CodeDispatchFailed, 10008},
		{"CodeUserNotFound", CodeUserNotFound, 11001},
		{"CodeUserExists", CodeUserExists, 11002},
		{"CodeInvalidCredentials", CodeInvalidCredentials, 11003},
		{"CodeUserInactive", CodeUserInactive, 11004},
		{"CodeWrongPassword", CodeWrongPassword, 11005},
		{"CodePasswordWeak", CodePasswordWeak, 11006},
		{"CodeRoleNotFound", CodeRoleNotFound, 12001},
		{"CodeRoleExists", CodeRoleExists, 12002},
		{"CodeRoleSystemProtected", CodeRoleSystemProtected, 12003},
		{"CodeDomainNotFound", CodeDomainNotFound, 13001},
		{"CodeDomainHasResources", CodeDomainHasResources, 13002},
		{"CodePermissionDenied", CodePermissionDenied, 14001},
		{"CodeWorkflowNotFound", CodeWorkflowNotFound, 15001},
		{"CodeDatasourceNotFound", CodeDatasourceNotFound, 16001},
		{"CodeDatasourceExists", CodeDatasourceExists, 16002},
		{"CodeDatasourceConnectFailed", CodeDatasourceConnectFailed, 16003},
		{"CodeDatasourceNameExists", CodeDatasourceNameExists, 16004},
		{"CodeQueryError", CodeQueryError, 17001},
		{"CodeConcurrentLimit", CodeConcurrentLimit, 17002},
		{"CodeQueryNoDatasource", CodeQueryNoDatasource, 17003},
		{"CodeQueryDisabled", CodeQueryDisabled, 17004},
		{"CodeQueryConnectFailed", CodeQueryConnectFailed, 17005},
		{"CodeQuerySelectOnly", CodeQuerySelectOnly, 17006},
		{"CodeQueryTimeout", CodeQueryTimeout, 17007},
		{"CodeQueryHistoryNotFound", CodeQueryHistoryNotFound, 17008},
		{"CodeSavedSQLNotFound", CodeSavedSQLNotFound, 17009},
		{"CodePermissionExists", CodePermissionExists, 14002},
	}

	for _, tt := range tests {
		if tt.code != tt.value {
			t.Errorf("%s = %d, want %d", tt.name, tt.code, tt.value)
		}
	}
}

func TestCodeCategories(t *testing.T) {
	categories := map[string]struct {
		codes map[string]int
		min   int
		max   int
	}{
		"task": {
			codes: map[string]int{
				"CodeTaskRunning":         CodeTaskRunning,
				"CodeTaskLocked":          CodeTaskLocked,
				"CodeTaskNotFound":        CodeTaskNotFound,
				"CodeExecutorNotFound":    CodeExecutorNotFound,
				"CodeExecutorOffline":     CodeExecutorOffline,
				"CodeExecutorNoCapacity":  CodeExecutorNoCapacity,
				"CodeNoAvailableExecutor": CodeNoAvailableExecutor,
				"CodeDispatchFailed":      CodeDispatchFailed,
			},
			min: 10000,
			max: 10999,
		},
		"user": {
			codes: map[string]int{
				"CodeUserNotFound":       CodeUserNotFound,
				"CodeUserExists":         CodeUserExists,
				"CodeInvalidCredentials": CodeInvalidCredentials,
				"CodeUserInactive":       CodeUserInactive,
				"CodeWrongPassword":      CodeWrongPassword,
				"CodePasswordWeak":       CodePasswordWeak,
			},
			min: 11000,
			max: 11999,
		},
		"role": {
			codes: map[string]int{
				"CodeRoleNotFound":        CodeRoleNotFound,
				"CodeRoleExists":          CodeRoleExists,
				"CodeRoleSystemProtected": CodeRoleSystemProtected,
			},
			min: 12000,
			max: 12999,
		},
		"domain": {
			codes: map[string]int{
				"CodeDomainNotFound":     CodeDomainNotFound,
				"CodeDomainHasResources": CodeDomainHasResources,
			},
			min: 13000,
			max: 13999,
		},
		"permission": {
			codes: map[string]int{
				"CodePermissionDenied": CodePermissionDenied,
				"CodePermissionExists": CodePermissionExists,
			},
			min: 14000,
			max: 14999,
		},
		"workflow": {
			codes: map[string]int{
				"CodeWorkflowNotFound": CodeWorkflowNotFound,
			},
			min: 15000,
			max: 15999,
		},
		"datasource": {
			codes: map[string]int{
				"CodeDatasourceNotFound":     CodeDatasourceNotFound,
				"CodeDatasourceExists":       CodeDatasourceExists,
				"CodeDatasourceConnectFailed": CodeDatasourceConnectFailed,
				"CodeDatasourceNameExists":   CodeDatasourceNameExists,
			},
			min: 16000,
			max: 16999,
		},
		"query": {
			codes: map[string]int{
				"CodeQueryError":          CodeQueryError,
				"CodeConcurrentLimit":     CodeConcurrentLimit,
				"CodeQueryNoDatasource":   CodeQueryNoDatasource,
				"CodeQueryDisabled":       CodeQueryDisabled,
				"CodeQueryConnectFailed":  CodeQueryConnectFailed,
				"CodeQuerySelectOnly":     CodeQuerySelectOnly,
				"CodeQueryTimeout":        CodeQueryTimeout,
				"CodeQueryHistoryNotFound": CodeQueryHistoryNotFound,
				"CodeSavedSQLNotFound":    CodeSavedSQLNotFound,
			},
			min: 17000,
			max: 17999,
		},
	}

	for catName, cat := range categories {
		for name, code := range cat.codes {
			if code < cat.min || code > cat.max {
				t.Errorf("%s (%s) = %d, expected to be in range %d-%d", name, catName, code, cat.min, cat.max)
			}
		}
	}
}
