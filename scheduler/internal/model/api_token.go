package model

import "time"

// APIToken API Token模型
type APIToken struct {
	ID             int64      `db:"id" json:"id"`
	UserID         int64      `db:"user_id" json:"user_id"`
	TokenEncrypted string     `db:"token_encrypted" json:"-"`
	TokenPrefix    string     `db:"token_prefix" json:"token_prefix"`
	LastUsedAt     *time.Time `db:"last_used_at" json:"last_used_at,omitempty"`
	CreatedAt      time.Time  `db:"created_at" json:"created_at"`
}
