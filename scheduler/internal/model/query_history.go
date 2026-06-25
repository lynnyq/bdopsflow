package model

import "time"

type QueryHistory struct {
	ID             int64     `db:"id" json:"id"`
	QueryID        string    `db:"query_id" json:"query_id,omitempty"`
	DatasourceID   *int64    `db:"datasource_id" json:"datasource_id,omitempty"`
	DatasourceName string    `db:"datasource_name" json:"datasource_name,omitempty"`
	SQLText        string    `db:"sql_text" json:"sql_text"`
	Database       string    `db:"database" json:"database,omitempty"`
	ExecutionTime  float64   `db:"execution_time" json:"execution_time"`
	RowCount       int       `db:"row_count" json:"row_count"`
	Status         string    `db:"status" json:"status"`
	ErrorMessage   string    `db:"error_message" json:"error_message,omitempty"`
	ExecutedBy     *int64    `db:"executed_by" json:"executed_by,omitempty"`
	ExecutedByName string    `db:"-" json:"executed_by_name,omitempty"`
	DomainID       int64     `db:"domain_id" json:"domain_id"`
	CreatedAt      time.Time `db:"created_at" json:"created_at"`
}
