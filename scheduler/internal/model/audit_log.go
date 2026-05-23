package model

import "time"

type AuditLog struct {
	ID            int64     `db:"id" json:"id"`
	UserID        *int64    `db:"user_id" json:"user_id,omitempty"`
	Username      string    `db:"username" json:"username"`
	RealName      string    `db:"real_name" json:"real_name,omitempty"`
	Role          string    `db:"role" json:"role,omitempty"`
	DomainID      *int64    `db:"domain_id" json:"domain_id,omitempty"`
	Action        string    `db:"action" json:"action"`
	Resource      string    `db:"resource" json:"resource"`
	ResourceID    string    `db:"resource_id" json:"resource_id,omitempty"`
	ResourceName  string    `db:"resource_name" json:"resource_name,omitempty"`
	Status        string    `db:"status" json:"status"`
	IPAddress     string    `db:"ip_address" json:"ip_address,omitempty"`
	UserAgent     string    `db:"user_agent" json:"user_agent,omitempty"`
	RequestMethod string    `db:"request_method" json:"request_method,omitempty"`
	RequestPath   string    `db:"request_path" json:"request_path,omitempty"`
	Detail        string    `db:"detail" json:"detail,omitempty"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
}

type AuditLogFilter struct {
	Username  string `form:"username"`
	Action    string `form:"action"`
	Resource  string `form:"resource"`
	Status    string `form:"status"`
	StartTime string `form:"start_time"`
	EndTime   string `form:"end_time"`
	Page      int    `form:"page"`
	PageSize  int    `form:"page_size"`
}
