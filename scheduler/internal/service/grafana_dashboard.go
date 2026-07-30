package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
	rqlite "github.com/rqlite/gorqlite"
)

// GrafanaDashboardService Grafana 图表嵌入配置业务逻辑
type GrafanaDashboardService struct {
	db database.DB
}

// NewGrafanaDashboardService 创建 Grafana 图表服务
func NewGrafanaDashboardService(db database.DB) *GrafanaDashboardService {
	return &GrafanaDashboardService{db: db}
}

// validate 校验 Grafana 图表配置
func validate(d *model.GrafanaDashboard) error {
	if strings.TrimSpace(d.Name) == "" {
		return fmt.Errorf("name cannot be empty")
	}
	if len(d.Name) > 100 {
		return fmt.Errorf("name must be at most 100 characters")
	}
	if d.EmbedURL == "" {
		return ErrGrafanaInvalidURL
	}
	// 解析 URL 校验协议，避免正则库
	parsed, err := url.Parse(d.EmbedURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ErrGrafanaInvalidURL
	}
	if len(d.EmbedURL) > 2048 {
		return fmt.Errorf("embed_url must be at most 2048 characters")
	}
	switch d.Width {
	case "third", "half", "full":
	default:
		return ErrGrafanaInvalidWidth
	}
	if d.HeightPx < 120 || d.HeightPx > 1200 {
		return ErrGrafanaInvalidHeight
	}
	if d.SortOrder < 0 {
		return ErrGrafanaInvalidSortOrder
	}
	return nil
}

// Create 新建 Grafana 图表配置
func (s *GrafanaDashboardService) Create(ctx context.Context, d *model.GrafanaDashboard) (*model.GrafanaDashboard, error) {
	if err := validate(d); err != nil {
		return nil, err
	}

	now := time.Now().Format(DateTimeFormat)
	query := `
		INSERT INTO bdopsflow_grafana_dashboards (name, description, embed_url, width, height_px, sort_order, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	stmt := rqlite.ParameterizedStatement{
		Query: query,
		Arguments: []interface{}{
			d.Name,
			d.Description,
			d.EmbedURL,
			d.Width,
			d.HeightPx,
			d.SortOrder,
			d.Enabled,
			now,
			now,
		},
	}

	result, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		slog.Error("failed to create grafana dashboard", "error", err, "name", d.Name)
		return nil, err
	}
	if result.Err != nil {
		slog.Error("grafana dashboard create result error", "error", result.Err, "name", d.Name)
		return nil, result.Err
	}

	d.ID = result.LastInsertID
	parsedTime, parseErr := time.Parse(DateTimeFormat, now)
	if parseErr != nil {
		slog.Warn("failed to parse grafana dashboard created_at, using zero value", "error", parseErr)
		parsedTime = time.Now()
	}
	d.CreatedAt = parsedTime
	d.UpdatedAt = parsedTime
	return d, nil
}

// Update 更新 Grafana 图表配置
func (s *GrafanaDashboardService) Update(ctx context.Context, id int64, d *model.GrafanaDashboard) error {
	if err := validate(d); err != nil {
		return err
	}

	now := time.Now().Format(DateTimeFormat)
	query := `
		UPDATE bdopsflow_grafana_dashboards
		SET name = ?, description = ?, embed_url = ?, width = ?, height_px = ?, sort_order = ?, enabled = ?, updated_at = ?
		WHERE id = ?
	`
	stmt := rqlite.ParameterizedStatement{
		Query: query,
		Arguments: []interface{}{
			d.Name,
			d.Description,
			d.EmbedURL,
			d.Width,
			d.HeightPx,
			d.SortOrder,
			d.Enabled,
			now,
			id,
		},
	}

	result, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		slog.Error("failed to update grafana dashboard", "error", err, "id", id)
		return err
	}
	if result.Err != nil {
		slog.Error("grafana dashboard update result error", "error", result.Err, "id", id)
		return result.Err
	}
	if result.RowsAffected == 0 {
		return ErrGrafanaDashboardNotFound
	}
	return nil
}

// Delete 删除 Grafana 图表配置
func (s *GrafanaDashboardService) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM bdopsflow_grafana_dashboards WHERE id = ?`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{id},
	}

	result, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		slog.Error("failed to delete grafana dashboard", "error", err, "id", id)
		return err
	}
	if result.Err != nil {
		slog.Error("grafana dashboard delete result error", "error", result.Err, "id", id)
		return result.Err
	}
	if result.RowsAffected == 0 {
		return ErrGrafanaDashboardNotFound
	}
	slog.Info("grafana dashboard deleted", "id", id)
	return nil
}

// ListEnabled 列出启用的图表（按 sort_order 升序），供 Dashboard 嵌入用
func (s *GrafanaDashboardService) ListEnabled(ctx context.Context) ([]model.GrafanaDashboard, error) {
	return s.list(ctx, true)
}

// ListAll 列出全部图表（含禁用），管理页用
func (s *GrafanaDashboardService) ListAll(ctx context.Context) ([]model.GrafanaDashboard, error) {
	return s.list(ctx, false)
}

func (s *GrafanaDashboardService) list(ctx context.Context, onlyEnabled bool) ([]model.GrafanaDashboard, error) {
	var stmt rqlite.ParameterizedStatement
	if onlyEnabled {
		stmt = rqlite.ParameterizedStatement{
			Query: `SELECT id, name, description, embed_url, width, height_px, sort_order, enabled, created_at, updated_at FROM bdopsflow_grafana_dashboards WHERE enabled = 1 ORDER BY sort_order ASC, id ASC`,
		}
	} else {
		stmt = rqlite.ParameterizedStatement{
			Query: `SELECT id, name, description, embed_url, width, height_px, sort_order, enabled, created_at, updated_at FROM bdopsflow_grafana_dashboards ORDER BY sort_order ASC, id ASC`,
		}
	}

	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	var items []model.GrafanaDashboard
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			slog.Warn("grafana dashboard list: failed to scan row, skipping", "error", err)
			continue
		}
		d := model.GrafanaDashboard{
			ID:          rowInt64(row[0]),
			Name:        rowString(row[1]),
			Description: rowString(row[2]),
			EmbedURL:    rowString(row[3]),
			Width:       rowString(row[4]),
			HeightPx:    rowInt(row[5]),
			SortOrder:   rowInt(row[6]),
			Enabled:     rowInt(row[7]),
		}
		if !isEmpty(row[8]) {
			d.CreatedAt = parseDateTime(row[8])
		}
		if !isEmpty(row[9]) {
			d.UpdatedAt = parseDateTime(row[9])
		}
		items = append(items, d)
	}
	return items, nil
}

// GetByID 获取单个 Grafana 图表配置
func (s *GrafanaDashboardService) GetByID(ctx context.Context, id int64) (*model.GrafanaDashboard, error) {
	query := `SELECT id, name, description, embed_url, width, height_px, sort_order, enabled, created_at, updated_at FROM bdopsflow_grafana_dashboards WHERE id = ?`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{id},
	}

	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}
	if !qr.Next() {
		return nil, ErrGrafanaDashboardNotFound
	}

	row, err := qr.Slice()
	if err != nil {
		return nil, err
	}

	d := &model.GrafanaDashboard{
		ID:          rowInt64(row[0]),
		Name:        rowString(row[1]),
		Description: rowString(row[2]),
		EmbedURL:    rowString(row[3]),
		Width:       rowString(row[4]),
		HeightPx:    rowInt(row[5]),
		SortOrder:   rowInt(row[6]),
		Enabled:     rowInt(row[7]),
	}
	if !isEmpty(row[8]) {
		d.CreatedAt = parseDateTime(row[8])
	}
	if !isEmpty(row[9]) {
		d.UpdatedAt = parseDateTime(row[9])
	}
	return d, nil
}

// Reorder 批量更新排序（rqlite 单次 Raft 提交，原子执行）
func (s *GrafanaDashboardService) Reorder(ctx context.Context, items []model.GrafanaReorderItem) error {
	if len(items) == 0 {
		return nil
	}
	now := time.Now().Format(DateTimeFormat)
	stmts := make([]rqlite.ParameterizedStatement, 0, len(items))
	for _, item := range items {
		if item.ID <= 0 {
			return fmt.Errorf("invalid id in reorder items")
		}
		if item.SortOrder < 0 {
			return ErrGrafanaInvalidSortOrder
		}
		stmts = append(stmts, rqlite.ParameterizedStatement{
			Query:     `UPDATE bdopsflow_grafana_dashboards SET sort_order = ?, updated_at = ? WHERE id = ?`,
			Arguments: []interface{}{item.SortOrder, now, item.ID},
		})
	}

	results, err := s.db.WriteParameterized(stmts)
	if err != nil {
		slog.Error("failed to reorder grafana dashboards", "error", err)
		return err
	}
	for i, result := range results {
		if result.Err != nil {
			slog.Error("grafana dashboard reorder result error", "error", result.Err, "index", i)
			return result.Err
		}
	}
	return nil
}
