package model

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token           string            `json:"token"`
	User            *UserWithRoles    `json:"user"`
	Permissions     []*Permission     `json:"permissions"`
	Domains         []*UserDomainInfo `json:"domains"`
	CurrentDomainID int64             `json:"current_domain_id"`
}

type CurrentUserResponse struct {
	User            *UserWithRoles    `json:"user"`
	Permissions     []*Permission     `json:"permissions"`
	Domains         []*UserDomainInfo `json:"domains"`
	CurrentDomainID int64             `json:"current_domain_id"`
}

type SwitchDomainResponse struct {
	Permissions     []*Permission `json:"permissions"`
	CurrentDomainID int64         `json:"current_domain_id"`
}
