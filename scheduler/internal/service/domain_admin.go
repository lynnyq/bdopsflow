package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
	rqlite "github.com/rqlite/gorqlite"
)

type DomainAdminService struct {
	db      database.DB
	permSvc *PermissionService
}

func NewDomainAdminService(db database.DB, permSvc *PermissionService) *DomainAdminService {
	return &DomainAdminService{db: db, permSvc: permSvc}
}

func (s *DomainAdminService) IsSystemAdmin(ctx context.Context, userID int64) (bool, error) {
	return s.permSvc.IsSystemAdmin(ctx, userID)
}

func (s *DomainAdminService) ListDomainsByUser(ctx context.Context, userID int64) ([]*model.DomainWithStats, error) {
	slog.Debug("ListDomainsByUser: fetching", "module", "domain_admin", "user_id", userID)
	query := `
		SELECT d.id, d.name, d.description, d.created_at,
			(SELECT COUNT(*) FROM bdopsflow_user_domains WHERE domain_id = d.id) as user_count,
			(SELECT COUNT(*) FROM bdopsflow_domain_executors WHERE domain_id = d.id) as executor_count,
			(SELECT COUNT(*) FROM bdopsflow_tasks WHERE domain_id = d.id) as task_count
		FROM bdopsflow_domains d
		JOIN bdopsflow_user_domains ud ON d.id = ud.domain_id
		WHERE ud.user_id = ?
		ORDER BY d.id ASC
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{userID},
	}
	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	var bdopsflow_domains []*model.DomainWithStats
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

		bdopsflow_domains = append(bdopsflow_domains, domain)
	}

	return bdopsflow_domains, nil
}

func (s *DomainAdminService) ListDomains(ctx context.Context) ([]*model.DomainWithStats, error) {
	query := `
		SELECT d.id, d.name, d.description, d.created_at,
			(SELECT COUNT(*) FROM bdopsflow_user_domains WHERE domain_id = d.id) as user_count,
			(SELECT COUNT(*) FROM bdopsflow_domain_executors WHERE domain_id = d.id) as executor_count,
			(SELECT COUNT(*) FROM bdopsflow_tasks WHERE domain_id = d.id) as task_count
		FROM bdopsflow_domains d
		ORDER BY d.id ASC
	`

	qr, err := s.db.QueryOne(query)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	var bdopsflow_domains []*model.DomainWithStats
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

		bdopsflow_domains = append(bdopsflow_domains, domain)
	}

	return bdopsflow_domains, nil
}

func (s *DomainAdminService) GetDomain(ctx context.Context, domainID int64) (*model.DomainWithStats, error) {
	query := `
		SELECT d.id, d.name, d.description, d.created_at,
			(SELECT COUNT(*) FROM bdopsflow_user_domains WHERE domain_id = d.id) as user_count,
			(SELECT COUNT(*) FROM bdopsflow_domain_executors WHERE domain_id = d.id) as executor_count,
			(SELECT COUNT(*) FROM bdopsflow_tasks WHERE domain_id = d.id) as task_count
		FROM bdopsflow_domains d
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

func (s *DomainAdminService) CreateDomain(ctx context.Context, name, description string) (*model.Domain, error) {
	slog.Info("CreateDomain: creating", "module", "domain_admin", "name", name)
	query := `
		INSERT INTO bdopsflow_domains (name, description, created_at)
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
	slog.Info("CreateDomain: success", "module", "domain_admin", "domain_id", domainID, "name", name)
	return s.GetDomainByID(ctx, domainID)
}

func (s *DomainAdminService) UpdateDomain(ctx context.Context, domainID int64, name, description string) (*model.Domain, error) {
	slog.Info("UpdateDomain: updating", "module", "domain_admin", "domain_id", domainID)
	query := `
		UPDATE bdopsflow_domains
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

func (s *DomainAdminService) DeleteDomain(ctx context.Context, domainID int64) error {
	slog.Info("DeleteDomain: deleting", "module", "domain_admin", "domain_id", domainID)
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

	query := `DELETE FROM bdopsflow_domains WHERE id = ?`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{domainID},
	}
	_, err = s.db.WriteOneParameterized(stmt)
	return err
}

func (s *DomainAdminService) GetDomainByID(ctx context.Context, domainID int64) (*model.Domain, error) {
	query := `SELECT id, name, description, created_at FROM bdopsflow_domains WHERE id = ?`

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
