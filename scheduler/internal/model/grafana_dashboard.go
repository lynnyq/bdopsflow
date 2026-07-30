package model

import "time"

// GrafanaDashboard Grafana 图表嵌入配置
type GrafanaDashboard struct {
	ID          int64     `db:"id" json:"id"`
	Name        string    `db:"name" json:"name"`
	Description string    `db:"description" json:"description"`
	EmbedURL    string    `db:"embed_url" json:"embed_url"`
	Width       string    `db:"width" json:"width"`           // 'third' | 'half' | 'full'
	HeightPx    int       `db:"height_px" json:"height_px"`   // iframe 高度（px），范围 120-1200
	SortOrder   int       `db:"sort_order" json:"sort_order"` // 升序排列，越小越靠前
	Enabled     int       `db:"enabled" json:"enabled"`       // 0=禁用 1=启用
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

// GrafanaReorderItem 排序更新项
type GrafanaReorderItem struct {
	ID        int64 `json:"id"`
	SortOrder int   `json:"sort_order"`
}
