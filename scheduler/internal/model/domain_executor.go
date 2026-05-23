package model

import "time"

// DomainExecutor 执行器领域分配模型
type DomainExecutor struct {
	ID         int64     `json:"id"`
	DomainID   int64     `json:"domain_id"`   // 领域ID
	ExecutorID int64     `json:"executor_id"` // 执行器ID
	AssignedBy *int64    `json:"assigned_by"`  // 分配者ID
	CreatedAt  time.Time `json:"created_at"`

	// 关联数据
	Domain   *Domain   `json:"domain,omitempty"`
	Executor *Executor `json:"executor,omitempty"`
}

// ExecutorDomainRequest 分配执行器到领域的请求
type ExecutorDomainRequest struct {
	DomainIDs []int64 `json:"domain_ids" binding:"required"`
}

// ExecutorWithDomains 带有领域信息的执行器
type ExecutorWithDomains struct {
	Executor
	Domains  []*Domain `json:"domains"`
	IsGlobal bool      `json:"is_global"` // 是否全局执行器
}

// DomainWithStats 带有统计信息的领域
type DomainWithStats struct {
	Domain
	UserCount     int64 `json:"user_count"`     // 用户数
	ExecutorCount int64 `json:"executor_count"` // 执行器数
	TaskCount     int64 `json:"task_count"`     // 任务数
}
