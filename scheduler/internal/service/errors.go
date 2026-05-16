package service

import "errors"

// 权限系统相关错误
var (
	// 用户相关错误
	ErrUserNotFound        = errors.New("user not found")
	ErrUserAlreadyExists   = errors.New("user already exists")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrUserInactive        = errors.New("user is inactive")
	ErrWrongPassword       = errors.New("wrong old password")
	ErrPasswordTooShort    = errors.New("password must be at least 6 characters")
	ErrCannotModifyOwnRole = errors.New("cannot modify your own role")

	// 角色相关错误
	ErrRoleNotFound            = errors.New("role not found")
	ErrRoleAlreadyExists       = errors.New("role already exists")
	ErrSystemRoleCannotDelete  = errors.New("system role cannot be deleted")
	ErrSystemRoleCannotModify  = errors.New("system role cannot be modified")

	// 领域相关错误
	ErrDomainNotFound     = errors.New("domain not found")
	ErrDomainHasResources = errors.New("domain has associated resources")

	// 执行器相关错误
	ErrExecutorNotFound     = errors.New("executor not found")
	ErrExecutorNotInDomain  = errors.New("executor not in the specified domain")

	// 权限相关错误
	ErrPermissionDenied = errors.New("permission denied")
	ErrUnauthorized    = errors.New("unauthorized")

	// 任务相关错误
	ErrTaskNotFound      = errors.New("task not found")
	ErrTaskAlreadyExists = errors.New("task already exists")
	ErrTaskRunning       = errors.New("task is running")
	ErrTaskLocked        = errors.New("task is locked by another execution")
)
