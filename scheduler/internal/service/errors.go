package service

import "errors"

type AppError struct {
	err     error
	Code    int
	Message string
}

func (e *AppError) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.err
}

func NewAppError(code int, message string) *AppError {
	return &AppError{Code: code, Message: message}
}

func WrapError(code int, err error) *AppError {
	return &AppError{err: err, Code: code, Message: err.Error()}
}

// 权限系统相关错误
var (
	// 用户相关错误
	ErrUserNotFound        = errors.New("user not found")
	ErrUserAlreadyExists   = errors.New("user already exists")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrUserInactive        = errors.New("user is inactive")
	ErrWrongPassword       = errors.New("wrong old password")
	ErrPasswordTooShort    = errors.New("password must be at least 6 characters")
	ErrPasswordTooLong     = errors.New("password must be at most 30 characters")
	ErrPasswordWeak        = errors.New("password must contain both letters and numbers")
	ErrCannotModifyOwnRole = errors.New("cannot modify your own role")

	ErrRoleNotFound           = errors.New("role not found")
	ErrRoleAlreadyExists      = errors.New("role already exists")
	ErrSystemRoleCannotDelete = errors.New("system role cannot be deleted")
	ErrSystemRoleCannotModify = errors.New("system role cannot be modified")

	// 领域相关错误
	ErrDomainNotFound     = errors.New("domain not found")
	ErrDomainHasResources = errors.New("domain has associated resources")

	// 执行器相关错误
	ErrExecutorNotFound    = errors.New("executor not found")
	ErrExecutorNotInDomain = errors.New("executor not in the specified domain")

	// 权限相关错误
	ErrPermissionDenied = errors.New("permission denied")
	ErrUnauthorized     = errors.New("unauthorized")

	// 任务相关错误
	ErrTaskNotFound      = errors.New("task not found")
	ErrTaskAlreadyExists = errors.New("task already exists")
	ErrTaskRunning       = errors.New("task is running")
	ErrTaskLocked        = errors.New("task is locked by another execution")

	ErrWorkflowNotFound = errors.New("workflow not found")

	ErrNoAvailableExecutor = errors.New("no available executor")
	ErrDispatchFailed      = errors.New("dispatch failed")
	ErrDispatcherNotConfigured = errors.New("dispatcher not configured")
)

var errorCodeMap = map[error]int{
	ErrUserNotFound:           11001,
	ErrUserAlreadyExists:      11002,
	ErrInvalidCredentials:     11003,
	ErrUserInactive:           11004,
	ErrWrongPassword:          11005,
	ErrPasswordWeak:           11006,
	ErrRoleNotFound:           12001,
	ErrRoleAlreadyExists:      12002,
	ErrSystemRoleCannotDelete: 12003,
	ErrSystemRoleCannotModify: 12003,
	ErrDomainNotFound:         13001,
	ErrDomainHasResources:     13002,
	ErrPermissionDenied:       14001,
	ErrUnauthorized:           14001,
	ErrTaskNotFound:           10003,
	ErrTaskAlreadyExists:      10001,
	ErrTaskRunning:            10001,
	ErrTaskLocked:             10002,
	ErrWorkflowNotFound:       15001,
	ErrExecutorNotFound:       10004,
	ErrNoAvailableExecutor:    10007,
	ErrDispatchFailed:         10008,
	ErrDispatcherNotConfigured: 10008,
}

func GetErrorCode(err error) int {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Code
	}
	if code, ok := errorCodeMap[err]; ok {
		return code
	}
	return 500
}
