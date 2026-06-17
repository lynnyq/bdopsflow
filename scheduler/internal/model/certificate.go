package model

import "time"

// Certificate 证书文件
type Certificate struct {
	ID         int64     `db:"id" json:"id"`
	Name       string    `db:"name" json:"name"`
	CaCert     string    `db:"ca_cert" json:"ca_cert,omitempty"`
	ClientCert string    `db:"client_cert" json:"client_cert,omitempty"`
	ClientKey  string    `db:"client_key" json:"-"` // 私钥不返回前端
	CreatedBy  int64     `db:"created_by" json:"created_by"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time `db:"updated_at" json:"updated_at"`
}

// CertificateSummary 证书摘要（列表展示用，不含敏感内容）
type CertificateSummary struct {
	ID            int64     `db:"id" json:"id"`
	Name          string    `db:"name" json:"name"`
	HasCACert     bool      `db:"-" json:"has_ca_cert"`
	HasClientCert bool      `db:"-" json:"has_client_cert"`
	HasClientKey  bool      `db:"-" json:"has_client_key"`
	CreatedBy     int64     `db:"created_by" json:"created_by"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time `db:"updated_at" json:"updated_at"`
}
