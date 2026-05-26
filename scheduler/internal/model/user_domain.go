package model

import "time"

type UserDomain struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	DomainID  int64     `json:"domain_id"`
	IsDefault bool      `json:"is_default"`
	CreatedAt time.Time `json:"created_at"`
}

type UserDomainInfo struct {
	DomainID   int64  `json:"domain_id"`
	DomainName string `json:"domain_name"`
	IsDefault  bool   `json:"is_default"`
}

type SwitchDomainRequest struct {
	DomainID int64 `json:"domain_id" binding:"required"`
}
