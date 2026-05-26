package model

import "time"

type Datasource struct {
	ID               int64      `db:"id" json:"id"`
	Name             string     `db:"name" json:"name"`
	Type             string     `db:"type" json:"type"`
	Host             string     `db:"host" json:"host,omitempty"`
	Port             int        `db:"port" json:"port,omitempty"`
	Path             string     `db:"path" json:"path,omitempty"`
	Database         string     `db:"database" json:"database,omitempty"`
	Username         string     `db:"username" json:"username,omitempty"`
	Password         string     `db:"password" json:"-"`
	AuthType         string     `db:"auth_type" json:"auth_type"`
	ConnectionMode   string     `db:"connection_mode" json:"connection_mode,omitempty"`
	ZkHosts          string     `db:"zk_hosts" json:"zk_hosts,omitempty"`
	ZkPath           string     `db:"zk_path" json:"zk_path,omitempty"`
	RqliteHosts      string     `db:"rqlite_hosts" json:"rqlite_hosts,omitempty"`
	Config           string     `db:"config" json:"config,omitempty"`
	Description      string     `db:"description" json:"description,omitempty"`
	DomainID         int64      `db:"domain_id" json:"domain_id"`
	DomainName       string     `db:"-" json:"domain_name,omitempty"`
	UserPermission   string     `db:"-" json:"user_permission,omitempty"`
	IsEnabled        bool       `db:"is_enabled" json:"is_enabled"`
	AllowWriteSQL    bool       `db:"allow_write_sql" json:"allow_write_sql"`
	TestStatus       string     `db:"test_status" json:"test_status"`
	LastTestAt       *time.Time `db:"last_test_at" json:"last_test_at,omitempty"`
	CreatedBy        *int64     `db:"created_by" json:"created_by,omitempty"`
	UpdatedBy        *int64     `db:"updated_by" json:"updated_by,omitempty"`
	CreatedAt        time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at" json:"updated_at"`
}

type DatasourcePermission struct {
	ID             int64  `db:"id" json:"id"`
	DatasourceID   int64  `db:"datasource_id" json:"datasource_id"`
	RoleID         *int64 `db:"role_id" json:"role_id,omitempty"`
	UserID         *int64 `db:"user_id" json:"user_id,omitempty"`
	PermissionType string `db:"permission_type" json:"permission_type"`
	GrantedBy      *int64 `db:"granted_by" json:"granted_by,omitempty"`
	GrantedAt      string `db:"granted_at" json:"granted_at"`
}
