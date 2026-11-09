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
	"github.com/lynnyq/bdopsflow/scheduler/pkg/iconfetch"
	rqlite "github.com/rqlite/gorqlite"
)

// NavigationLinkService 导航快捷链接业务逻辑
type NavigationLinkService struct {
	db database.DB
}

// NewNavigationLinkService 创建导航链接服务
func NewNavigationLinkService(db database.DB) *NavigationLinkService {
	return &NavigationLinkService{db: db}
}

// validateNavigation 校验导航链接配置字段合法性
func validateNavigation(d *model.NavigationLink) error {
	if strings.TrimSpace(d.Name) == "" {
		return fmt.Errorf("name cannot be empty")
	}
	if len(d.Name) > 100 {
		return fmt.Errorf("name must be at most 100 characters")
	}
	if d.URL == "" {
		return ErrNavigationInvalidURL
	}
	parsed, err := url.Parse(d.URL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ErrNavigationInvalidURL
	}
	if len(d.URL) > 2048 {
		return fmt.Errorf("url must be at most 2048 characters")
	}
	if !model.ValidWidths[d.Width] {
		return ErrNavigationInvalidWidth
	}
	if !model.ValidDisplayModes[d.DisplayMode] {
		return ErrNavigationInvalidDisplayMode
	}
	// HeightPx 为 0 表示使用基准值，40-200 表示覆盖
	if d.HeightPx != 0 && (d.HeightPx < 40 || d.HeightPx > 200) {
		return ErrNavigationInvalidHeight
	}
	if d.SortOrder < 0 {
		return ErrNavigationInvalidSortOrder
	}
	return nil
}

// Create 新建导航链接。创建成功后异步触发 favicon 抓取（不阻塞主请求）。
func (s *NavigationLinkService) Create(ctx context.Context, d *model.NavigationLink) (*model.NavigationLink, error) {
	if err := validateNavigation(d); err != nil {
		return nil, err
	}

	now := time.Now().Format(DateTimeFormat)
	query := `
		INSERT INTO bdopsflow_navigation_links
			(name, description, url, width, display_mode, height_px, sort_order, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	stmt := rqlite.ParameterizedStatement{
		Query: query,
		Arguments: []interface{}{
			d.Name, d.Description, d.URL, d.Width, d.DisplayMode,
			nullableInt(d.HeightPx), d.SortOrder, d.Enabled, now, now,
		},
	}

	result, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		slog.Error("failed to create navigation link", "error", err, "name", d.Name)
		return nil, err
	}
	if result.Err != nil {
		slog.Error("navigation link create result error", "error", result.Err, "name", d.Name)
		return nil, result.Err
	}

	d.ID = result.LastInsertID
	parsedTime, parseErr := time.Parse(DateTimeFormat, now)
	if parseErr != nil {
		slog.Warn("failed to parse navigation link created_at", "error", parseErr)
		parsedTime = time.Now()
	}
	d.CreatedAt = parsedTime
	d.UpdatedAt = parsedTime

	// 异步抓 favicon，失败不影响主流程
	sgoFetchIcon(s.db, d.ID, d.URL)

	return d, nil
}

// Update 更新导航链接。当 URL 变更时自动重新触发 favicon 抓取。
func (s *NavigationLinkService) Update(ctx context.Context, id int64, d *model.NavigationLink) error {
	if err := validateNavigation(d); err != nil {
		return err
	}

	// 读取旧记录以判断 URL 是否变更
	old, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}

	now := time.Now().Format(DateTimeFormat)
	query := `
		UPDATE bdopsflow_navigation_links
		SET name = ?, description = ?, url = ?, width = ?, display_mode = ?,
		    height_px = ?, sort_order = ?, enabled = ?, updated_at = ?
		WHERE id = ?
	`
	stmt := rqlite.ParameterizedStatement{
		Query: query,
		Arguments: []interface{}{
			d.Name, d.Description, d.URL, d.Width, d.DisplayMode,
			nullableInt(d.HeightPx), d.SortOrder, d.Enabled, now, id,
		},
	}

	result, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		slog.Error("failed to update navigation link", "error", err, "id", id)
		return err
	}
	if result.Err != nil {
		slog.Error("navigation link update result error", "error", result.Err, "id", id)
		return result.Err
	}
	if result.RowsAffected == 0 {
		return ErrNavigationLinkNotFound
	}

	// URL 变更时异步重抓 icon
	if old.URL != d.URL {
		sgoFetchIcon(s.db, id, d.URL)
	}

	return nil
}

// Delete 删除导航链接
func (s *NavigationLinkService) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM bdopsflow_navigation_links WHERE id = ?`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{id},
	}

	result, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		slog.Error("failed to delete navigation link", "error", err, "id", id)
		return err
	}
	if result.Err != nil {
		slog.Error("navigation link delete result error", "error", result.Err, "id", id)
		return result.Err
	}
	if result.RowsAffected == 0 {
		return ErrNavigationLinkNotFound
	}
	slog.Info("navigation link deleted", "id", id)
	return nil
}

// ListEnabled 列出启用的导航链接（Dashboard 首页用）
func (s *NavigationLinkService) ListEnabled(ctx context.Context) ([]model.NavigationLink, error) {
	return s.list(ctx, true)
}

// ListAll 列出全部导航链接（管理页用）
func (s *NavigationLinkService) ListAll(ctx context.Context) ([]model.NavigationLink, error) {
	return s.list(ctx, false)
}

func (s *NavigationLinkService) list(ctx context.Context, onlyEnabled bool) ([]model.NavigationLink, error) {
	var stmt rqlite.ParameterizedStatement
	if onlyEnabled {
		stmt = rqlite.ParameterizedStatement{
			Query: `SELECT id, name, description, url, width, display_mode, height_px, icon_data, icon_fetched_at, sort_order, enabled, created_at, updated_at
					FROM bdopsflow_navigation_links WHERE enabled = 1 ORDER BY sort_order ASC, id ASC`,
		}
	} else {
		stmt = rqlite.ParameterizedStatement{
			Query: `SELECT id, name, description, url, width, display_mode, height_px, icon_data, icon_fetched_at, sort_order, enabled, created_at, updated_at
					FROM bdopsflow_navigation_links ORDER BY sort_order ASC, id ASC`,
		}
	}

	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	return scanNavLinks(qr), nil
}

// GetByID 获取单个导航链接
func (s *NavigationLinkService) GetByID(ctx context.Context, id int64) (*model.NavigationLink, error) {
	query := `SELECT id, name, description, url, width, display_mode, height_px, icon_data, icon_fetched_at, sort_order, enabled, created_at, updated_at
			  FROM bdopsflow_navigation_links WHERE id = ?`
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
		return nil, ErrNavigationLinkNotFound
	}

	item, err := scanOneNavLink(qr)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// Reorder 批量更新排序（rqlite 单次 Raft 提交）
func (s *NavigationLinkService) Reorder(ctx context.Context, items []model.NavigationReorderItem) error {
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
			return ErrNavigationInvalidSortOrder
		}
		stmts = append(stmts, rqlite.ParameterizedStatement{
			Query:     `UPDATE bdopsflow_navigation_links SET sort_order = ?, updated_at = ? WHERE id = ?`,
			Arguments: []interface{}{item.SortOrder, now, item.ID},
		})
	}

	results, err := s.db.WriteParameterized(stmts)
	if err != nil {
		slog.Error("failed to reorder navigation links", "error", err)
		return err
	}
	for i, result := range results {
		if result.Err != nil {
			slog.Error("navigation link reorder result error", "error", result.Err, "index", i)
			return result.Err
		}
	}
	return nil
}

// UpdateIcon 更新指定导航的图标数据（被异步 iconfetch goroutine 或 RefreshIcon 调用）
func (s *NavigationLinkService) UpdateIcon(ctx context.Context, id int64, iconData string, fetchedAt time.Time) error {
	query := `UPDATE bdopsflow_navigation_links SET icon_data = ?, icon_fetched_at = ?, updated_at = ? WHERE id = ?`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{iconData, fetchedAt.Format(DateTimeFormat), time.Now().Format(DateTimeFormat), id},
	}

	_, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		slog.Error("failed to update navigation link icon", "error", err, "id", id)
		return err
	}
	return nil
}

// RefreshIcon 手动触发重新抓取指定导航的 favicon
func (s *NavigationLinkService) RefreshIcon(ctx context.Context, id int64) (*model.NavigationLink, error) {
	d, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	result, fetchErr := iconfetch.Fetch(ctx, d.URL)
	if fetchErr != nil {
		slog.Warn("refresh navigation link icon failed", "id", id, "url", d.URL, "error", fetchErr)
		// 抓失败不清空旧 icon，保持最后一次成功的结果
		return d, nil
	}

	// 成功则更新
	if updateErr := s.UpdateIcon(ctx, id, result.IconData, result.FetchedAt); updateErr != nil {
		return nil, updateErr
	}
	d.IconData = result.IconData
	d.IconFetchedAt = result.FetchedAt
	return d, nil
}

// sgoFetchIcon 在独立 goroutine 中抓取 favicon 并回写 DB
// 失败只记日志，不影响已创建/更新的记录
func sgoFetchIcon(db database.DB, linkID int64, targetURL string) {
	go func() {
		// 用独立 context.Background() 避免请求结束 context 被取消
		ctx := context.Background()
		result, err := iconfetch.Fetch(ctx, targetURL)
		if err != nil {
			slog.Warn("navigation link icon fetch failed",
				"id", linkID, "url", targetURL, "error", err)
			return
		}
		navSvc := NewNavigationLinkService(db)
		if err := navSvc.UpdateIcon(ctx, linkID, result.IconData, result.FetchedAt); err != nil {
			slog.Warn("navigation link icon update failed", "id", linkID, "error", err)
		} else {
			slog.Info("navigation link icon fetched", "id", linkID, "source", result.Source)
		}
	}()
}

// nullableInt 将 0 转为 nil，方便 height_px 存入 DB NULL
func nullableInt(v int) interface{} {
	if v == 0 {
		return nil
	}
	return v
}

// scanNavLinks 遍历 QueryResult 扫描完整导航链接列表
func scanNavLinks(qr rqlite.QueryResult) []model.NavigationLink {
	var items []model.NavigationLink
	for qr.Next() {
		item, err := scanOneNavLink(qr)
		if err != nil {
			slog.Warn("navigation link list: failed to scan row, skipping", "error", err)
			continue
		}
		items = append(items, item)
	}
	return items
}

// scanOneNavLink 从 QueryResult 当前行扫描一个导航链接
func scanOneNavLink(qr rqlite.QueryResult) (model.NavigationLink, error) {
	row, err := qr.Slice()
	if err != nil {
		return model.NavigationLink{}, err
	}

	d := model.NavigationLink{
		ID:          rowInt64(row[0]),
		Name:        rowString(row[1]),
		Description: rowString(row[2]),
		URL:         rowString(row[3]),
		Width:       rowString(row[4]),
		DisplayMode: rowString(row[5]),
		HeightPx:    rowInt(row[6]),
		IconData:    rowString(row[7]),
		SortOrder:   rowInt(row[9]),
		Enabled:     rowInt(row[10]),
	}
	if !isEmpty(row[8]) {
		d.IconFetchedAt = parseDateTime(row[8])
	}
	if !isEmpty(row[11]) {
		d.CreatedAt = parseDateTime(row[11])
	}
	if !isEmpty(row[12]) {
		d.UpdatedAt = parseDateTime(row[12])
	}
	return d, nil
}
