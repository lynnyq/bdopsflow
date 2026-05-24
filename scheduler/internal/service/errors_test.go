package service

import (
	"errors"
	"testing"
)

func TestGetErrorCode_AllDefinedErrors(t *testing.T) {
	tests := []struct {
		err  error
		code int
	}{
		{ErrUserNotFound, 11001},
		{ErrUserAlreadyExists, 11002},
		{ErrInvalidCredentials, 11003},
		{ErrUserInactive, 11004},
		{ErrWrongPassword, 11005},
		{ErrPasswordWeak, 11006},
		{ErrRoleNotFound, 12001},
		{ErrRoleAlreadyExists, 12002},
		{ErrSystemRoleCannotDelete, 12003},
		{ErrSystemRoleCannotModify, 12003},
		{ErrDomainNotFound, 13001},
		{ErrDomainHasResources, 13002},
		{ErrPermissionDenied, 14001},
		{ErrUnauthorized, 14001},
		{ErrTaskNotFound, 10003},
		{ErrTaskAlreadyExists, 10001},
		{ErrTaskRunning, 10001},
		{ErrTaskLocked, 10002},
		{ErrWorkflowNotFound, 15001},
		{ErrExecutorNotFound, 10004},
		{ErrNoAvailableExecutor, 10007},
		{ErrDispatchFailed, 10008},
		{ErrDispatcherNotConfigured, 10008},
	}

	for _, tt := range tests {
		code := GetErrorCode(tt.err)
		if code != tt.code {
			t.Errorf("GetErrorCode(%v) = %d, want %d", tt.err, code, tt.code)
		}
	}
}

func TestGetErrorCode_UnknownError(t *testing.T) {
	unknownErr := errors.New("some unknown error")
	code := GetErrorCode(unknownErr)
	if code != 500 {
		t.Errorf("GetErrorCode(unknown) = %d, want 500", code)
	}
}

func TestGetErrorCode_AppError(t *testing.T) {
	appErr := NewAppError(12345, "custom app error")
	code := GetErrorCode(appErr)
	if code != 12345 {
		t.Errorf("GetErrorCode(AppError) = %d, want 12345", code)
	}
}

func TestGetErrorCode_NilError(t *testing.T) {
	code := GetErrorCode(nil)
	if code != 500 {
		t.Errorf("GetErrorCode(nil) = %d, want 500", code)
	}
}

func TestNewAppError(t *testing.T) {
	appErr := NewAppError(10001, "task running")

	if appErr.Code != 10001 {
		t.Errorf("expected code 10001, got %d", appErr.Code)
	}
	if appErr.Message != "task running" {
		t.Errorf("expected message 'task running', got %q", appErr.Message)
	}
	if appErr.err != nil {
		t.Errorf("expected inner err to be nil, got %v", appErr.err)
	}
}

func TestAppError_Error_WithInnerErr(t *testing.T) {
	innerErr := errors.New("inner error detail")
	appErr := WrapError(5001, innerErr)

	errMsg := appErr.Error()
	if errMsg != "inner error detail" {
		t.Errorf("expected Error() to return inner error message 'inner error detail', got %q", errMsg)
	}
}

func TestAppError_Error_WithoutInnerErr(t *testing.T) {
	appErr := NewAppError(10001, "task is running")

	errMsg := appErr.Error()
	if errMsg != "task is running" {
		t.Errorf("expected Error() to return 'task is running', got %q", errMsg)
	}
}

func TestAppError_Unwrap(t *testing.T) {
	innerErr := errors.New("inner error")
	appErr := WrapError(5001, innerErr)

	unwrapped := appErr.Unwrap()
	if unwrapped != innerErr {
		t.Errorf("expected unwrapped error to match inner error")
	}
}

func TestAppError_Unwrap_Nil(t *testing.T) {
	appErr := NewAppError(10001, "no inner error")

	unwrapped := appErr.Unwrap()
	if unwrapped != nil {
		t.Errorf("expected unwrapped to be nil, got %v", unwrapped)
	}
}

func TestAppError_ErrorsIs(t *testing.T) {
	innerErr := errors.New("base error")
	appErr := WrapError(5001, innerErr)

	if !errors.Is(appErr, innerErr) {
		t.Error("expected errors.Is to match inner error via Unwrap")
	}
}

func TestWrapError(t *testing.T) {
	innerErr := errors.New("database connection failed")
	appErr := WrapError(5001, innerErr)

	if appErr.Code != 5001 {
		t.Errorf("expected code 5001, got %d", appErr.Code)
	}
	if appErr.Message != "database connection failed" {
		t.Errorf("expected message 'database connection failed', got %q", appErr.Message)
	}
	if appErr.err != innerErr {
		t.Errorf("expected inner err to match")
	}
}

func TestWrapError_GetErrorCode(t *testing.T) {
	innerErr := errors.New("something broke")
	appErr := WrapError(99999, innerErr)

	code := GetErrorCode(appErr)
	if code != 99999 {
		t.Errorf("GetErrorCode(WrapError(...)) = %d, want 99999", code)
	}
}

func TestAllErrorVariablesDefined(t *testing.T) {
	errorVars := []error{
		ErrUserNotFound,
		ErrUserAlreadyExists,
		ErrInvalidCredentials,
		ErrUserInactive,
		ErrWrongPassword,
		ErrPasswordTooShort,
		ErrPasswordTooLong,
		ErrPasswordWeak,
		ErrCannotModifyOwnRole,
		ErrRoleNotFound,
		ErrRoleAlreadyExists,
		ErrSystemRoleCannotDelete,
		ErrSystemRoleCannotModify,
		ErrDomainNotFound,
		ErrDomainHasResources,
		ErrExecutorNotFound,
		ErrExecutorNotInDomain,
		ErrPermissionDenied,
		ErrUnauthorized,
		ErrTaskNotFound,
		ErrTaskAlreadyExists,
		ErrTaskRunning,
		ErrTaskLocked,
		ErrWorkflowNotFound,
		ErrNoAvailableExecutor,
		ErrDispatchFailed,
		ErrDispatcherNotConfigured,
	}

	for _, errVar := range errorVars {
		if errVar == nil {
			t.Error("expected error variable to be defined, got nil")
		}
		if errVar.Error() == "" {
			t.Error("expected error variable to have non-empty message")
		}
	}
}

func TestErrorCodeMapCoversAllMappedErrors(t *testing.T) {
	mappedErrors := []error{
		ErrUserNotFound,
		ErrUserAlreadyExists,
		ErrInvalidCredentials,
		ErrUserInactive,
		ErrWrongPassword,
		ErrPasswordWeak,
		ErrRoleNotFound,
		ErrRoleAlreadyExists,
		ErrSystemRoleCannotDelete,
		ErrSystemRoleCannotModify,
		ErrDomainNotFound,
		ErrDomainHasResources,
		ErrPermissionDenied,
		ErrUnauthorized,
		ErrTaskNotFound,
		ErrTaskAlreadyExists,
		ErrTaskRunning,
		ErrTaskLocked,
		ErrWorkflowNotFound,
		ErrExecutorNotFound,
		ErrNoAvailableExecutor,
		ErrDispatchFailed,
		ErrDispatcherNotConfigured,
	}

	for _, err := range mappedErrors {
		if _, ok := errorCodeMap[err]; !ok {
			t.Errorf("error %v is not in errorCodeMap", err)
		}
	}
}

func TestErrorCodeMapConsistentWithHandlerCodes(t *testing.T) {
	type codeMapping struct {
		err       error
		wantCode  int
	}
	mappings := []codeMapping{
		{ErrUserNotFound, 11001},
		{ErrUserAlreadyExists, 11002},
		{ErrInvalidCredentials, 11003},
		{ErrUserInactive, 11004},
		{ErrWrongPassword, 11005},
		{ErrPasswordWeak, 11006},
		{ErrRoleNotFound, 12001},
		{ErrRoleAlreadyExists, 12002},
		{ErrSystemRoleCannotDelete, 12003},
		{ErrDomainNotFound, 13001},
		{ErrDomainHasResources, 13002},
		{ErrPermissionDenied, 14001},
		{ErrTaskNotFound, 10003},
		{ErrTaskLocked, 10002},
		{ErrWorkflowNotFound, 15001},
		{ErrExecutorNotFound, 10004},
		{ErrNoAvailableExecutor, 10007},
		{ErrDispatchFailed, 10008},
	}

	for _, m := range mappings {
		code := GetErrorCode(m.err)
		if code != m.wantCode {
			t.Errorf("GetErrorCode(%v) = %d, want %d (handler code)", m.err, code, m.wantCode)
		}
	}
}

func TestGetErrorCode_UnmappedErrorsReturn500(t *testing.T) {
	unmappedErrors := []error{
		ErrPasswordTooShort,
		ErrPasswordTooLong,
		ErrCannotModifyOwnRole,
		ErrExecutorNotInDomain,
	}

	for _, err := range unmappedErrors {
		code := GetErrorCode(err)
		if code != 500 {
			t.Errorf("GetErrorCode(%v) = %d, want 500 for unmapped error", err, code)
		}
	}
}

func TestErrorMessages(t *testing.T) {
	tests := []struct {
		err     error
		message string
	}{
		{ErrUserNotFound, "user not found"},
		{ErrUserAlreadyExists, "user already exists"},
		{ErrInvalidCredentials, "invalid credentials"},
		{ErrUserInactive, "user is inactive"},
		{ErrWrongPassword, "wrong old password"},
		{ErrPasswordTooShort, "password must be at least 6 characters"},
		{ErrPasswordTooLong, "password must be at most 30 characters"},
		{ErrPasswordWeak, "password must contain both letters and numbers"},
		{ErrCannotModifyOwnRole, "cannot modify your own role"},
		{ErrRoleNotFound, "role not found"},
		{ErrRoleAlreadyExists, "role already exists"},
		{ErrSystemRoleCannotDelete, "system role cannot be deleted"},
		{ErrSystemRoleCannotModify, "system role cannot be modified"},
		{ErrDomainNotFound, "domain not found"},
		{ErrDomainHasResources, "domain has associated resources"},
		{ErrExecutorNotFound, "executor not found"},
		{ErrExecutorNotInDomain, "executor not in the specified domain"},
		{ErrPermissionDenied, "permission denied"},
		{ErrUnauthorized, "unauthorized"},
		{ErrTaskNotFound, "task not found"},
		{ErrTaskAlreadyExists, "task already exists"},
		{ErrTaskRunning, "task is running"},
		{ErrTaskLocked, "task is locked by another execution"},
		{ErrWorkflowNotFound, "workflow not found"},
		{ErrNoAvailableExecutor, "no available executor"},
		{ErrDispatchFailed, "dispatch failed"},
		{ErrDispatcherNotConfigured, "dispatcher not configured"},
	}

	for _, tt := range tests {
		if tt.err.Error() != tt.message {
			t.Errorf("error message = %q, want %q", tt.err.Error(), tt.message)
		}
	}
}

func TestAppError_ImplementsErrorInterface(t *testing.T) {
	var err error = NewAppError(10001, "test error")
	if err.Error() != "test error" {
		t.Errorf("AppError does not correctly implement error interface, got %q", err.Error())
	}
}

func TestGetErrorCode_AppErrorTakesPrecedenceOverMap(t *testing.T) {
	appErr := NewAppError(99999, "user not found")
	code := GetErrorCode(appErr)
	if code != 99999 {
		t.Errorf("GetErrorCode(AppError) = %d, want 99999 (AppError code should take precedence)", code)
	}
}

func TestErrorCodeMap_DuplicateCodeMappings(t *testing.T) {
	codeToErrors := make(map[int][]error)
	for err, code := range errorCodeMap {
		codeToErrors[code] = append(codeToErrors[code], err)
	}

	expectedDuplicates := map[int]int{
		12003: 2,
		14001: 2,
		10001: 2,
		10008: 2,
	}

	for code, errs := range codeToErrors {
		if len(errs) > 1 {
			expectedCount, isExpected := expectedDuplicates[code]
			if !isExpected {
				t.Errorf("unexpected duplicate code %d mapped by %d errors", code, len(errs))
			} else if len(errs) != expectedCount {
				t.Errorf("code %d has %d mappings, expected %d", code, len(errs), expectedCount)
			}
		}
	}
}
