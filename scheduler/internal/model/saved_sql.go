package model

import "time"

type SavedSQL struct {
	ID           int64     `db:"id" json:"id"`
	Name         string    `db:"name" json:"name"`
	DatasourceID int64     `db:"datasource_id" json:"datasource_id"`
	SQLText      string    `db:"sql_text" json:"sql_text"`
	Description  string    `db:"description" json:"description,omitempty"`
	CreatedBy    *int64    `db:"created_by" json:"created_by,omitempty"`
	UpdatedBy    *int64    `db:"updated_by" json:"updated_by,omitempty"`
	DomainID     int64     `db:"domain_id" json:"domain_id"`
	IsPublic     bool      `db:"is_public" json:"is_public"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
}
