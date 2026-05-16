package service

import (
	"context"
	"time"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	rqlite "github.com/rqlite/gorqlite"
)

// DomainAdminService 领域管理服务
type DomainAdminService struct {
	db rqlite.Connection
}

// NewDomainAdminService 创建领域管理服务
func NewDomainAdminService(db rqlite.Connection) *DomainAdminService {
	return &DomainAdminService{db: db}
}

// ListDomains 获取领域列表
func (s *DomainAdminService) ListDomains(ctx context.Context) ([]*model.DomainWithStats, error) {
	query := `
		SELECT d.id, d.name, d.description, d.created_at,
			(SELECT COUNT(*) FROM users WHERE domain_id = d.id) as user_count,
			(SELECT COUNT(*) FROM domain_executors WHERE domain_id = d.id) as executor_count,
			(SELECT COUNT(*) FROM tasks WHERE domain_id = d.id) as task_count
		FROM domains d
		ORDER BY d.id ASC
	`

	qr, err := s.db.QueryOne(query)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	var domains []*model.DomainWithStats
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}

		domain := &model.DomainWithStats{
			Domain: model.Domain{
				ID:          rowInt64(row[0]),
				Name:        rowString(row[1]),
				Description: rowString(row[2]),
			},
			UserCount:     rowInt64(row[4]),
			ExecutorCount: rowInt64(row[5]),
			TaskCount:     rowInt64(row[6]),
		}

		domains = append(domains, domain)
	}

	return domains, nil
}

// GetDomain 获取领域详情
func (s *DomainAdminService) GetDomain(ctx context.Context, domainID int64) (*model.DomainWithStats, error) {
	query := `
		SELECT d.id, d.name, d.description, d.created_at,
			(SELECT COUNT(*) FROM users WHERE domain_id = d.id) as user_count,
			(SELECT COUNT(*) FROM domain_executors WHERE domain_id = d.id) as executor_count,
			(SELECT COUNT(*) FROM tasks WHERE domain_id = d.id) as task_count
		FROM domains d
		WHERE d.id = ?
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{domainID},
	}
	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	if !qr.Next() {
		return nil, nil
	}

	row, err := qr.Slice()
	if err != nil {
		return nil, err
	}

	domain := &model.DomainWithStats{
		Domain: model.Domain{
			ID:          rowInt64(row[0]),
			Name:        rowString(row[1]),
			Description: rowString(row[2]),
		},
		UserCount:     rowInt64(row[4]),
		ExecutorCount: rowInt64(row[5]),
		TaskCount:     rowInt64(row[6]),
	}

	return domain, nil
}

// CreateDomain 创建领域
func (s *DomainAdminService) CreateDomain(ctx context.Context, name, description string) (*model.Domain, error) {
	query := `
		INSERT INTO domains (name, description, created_at)
		VALUES (?, ?, ?)
	`

	now := time.Now()
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{name, description, now},
	}
	result, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if result.Err != nil {
		return nil, result.Err
	}

	domainID := result.LastInsertID
	return s.GetDomainByID(ctx, domainID)
}

// UpdateDomain 更新领域
func (s *DomainAdminService) UpdateDomain(ctx context.Context, domainID int64, name, description string) (*model.Domain, error) {
	query := `
		UPDATE domains
		SET name = ?, description = ?
		WHERE id = ?
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{name, description, domainID},
	}
	_, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		return nil, err
	}

	return s.GetDomainByID(ctx, domainID)
}

// DeleteDomain 删除领域
func (s *DomainAdminService) DeleteDomain(ctx context.Context, domainID int64) error {
	// 检查是否有资源
	domain, err := s.GetDomain(ctx, domainID)
	if err != nil {
		return err
	}
	if domain == nil {
		return ErrDomainNotFound
	}

	if domain.UserCount > 0 || domain.ExecutorCount > 0 || domain.TaskCount > 0 {
		return ErrDomainHasResources
	}

	// 删除领域
	query := `DELETE FROM domains WHERE id = ?`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{domainID},
	}
	_, err = s.db.WriteOneParameterized(stmt)
	return err
}

// GetDomainByID 根据ID获取领域
func (s *DomainAdminService) GetDomainByID(ctx context.Context, domainID int64) (*model.Domain, error) {
	query := `SELECT id, name, description, created_at FROM domains WHERE id = ?`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{domainID},
	}
	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	if !qr.Next() {
		return nil, nil
	}

	row, err := qr.Slice()
	if err != nil {
		return nil, err
	}

	domain := &model.Domain{
		ID:          rowInt64(row[0]),
		Name:        rowString(row[1]),
		Description: rowString(row[2]),
	}

	return domain, nil
}
