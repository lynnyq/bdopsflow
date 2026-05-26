package model

type WebhookPermission struct {
	ID             int64  `json:"id"`
	WebhookID      int64  `json:"webhook_id"`
	RoleID         *int64 `json:"role_id"`
	UserID         *int64 `json:"user_id"`
	PermissionType string `json:"permission_type"`
	GrantedBy      *int64 `json:"granted_by"`
	GrantedAt      string `json:"granted_at"`
}

type GrantInstancePermissionRequest struct {
	RoleID         *int64 `json:"role_id"`
	UserID         *int64 `json:"user_id"`
	PermissionType string `json:"permission_type" binding:"required"`
}
