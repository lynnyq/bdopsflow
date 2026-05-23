package model

import "time"

type SystemConfig struct {
	ID          int64     `db:"id" json:"id"`
	ConfigKey   string    `db:"config_key" json:"config_key"`
	ConfigValue string    `db:"config_value" json:"config_value"`
	Description string    `db:"description" json:"description,omitempty"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

type SystemConfigHistory struct {
	ID        int64  `db:"id" json:"id"`
	ConfigKey string `db:"config_key" json:"config_key"`
	OldValue  string `db:"old_value" json:"old_value,omitempty"`
	NewValue  string `db:"new_value" json:"new_value"`
	ChangedBy *int64 `db:"changed_by" json:"changed_by,omitempty"`
	ChangedAt string `db:"changed_at" json:"changed_at"`
}
