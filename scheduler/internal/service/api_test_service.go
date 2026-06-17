package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
	rqlite "github.com/rqlite/gorqlite"
)

type ApiTestService struct {
	db database.DB
}

func NewApiTestService(db database.DB) *ApiTestService {
	return &ApiTestService{db: db}
}

func (s *ApiTestService) Create(ctx context.Context, test *model.ApiTest) (*model.ApiTest, error) {
	query := `
		INSERT INTO bdopsflow_api_tests (name, type, config, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	now := time.Now().Format(DateTimeFormat)
	stmt := rqlite.ParameterizedStatement{
		Query: query,
		Arguments: []interface{}{
			test.Name,
			test.Type,
			test.Config,
			test.CreatedBy,
			now,
			now,
		},
	}

	result, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		slog.Error("failed to create api test", "error", err, "name", test.Name)
		return nil, err
	}
	if result.Err != nil {
		slog.Error("api test create result error", "error", result.Err, "name", test.Name)
		return nil, result.Err
	}

	test.ID = result.LastInsertID
	parsedTime, parseErr := time.Parse(DateTimeFormat, now)
	if parseErr != nil {
		slog.Warn("failed to parse api test created_at time, using zero value", "error", parseErr)
		parsedTime = time.Now()
	}
	test.CreatedAt = parsedTime
	test.UpdatedAt = parsedTime
	return test, nil
}

func (s *ApiTestService) Update(ctx context.Context, id int64, test *model.ApiTest) error {
	query := `
		UPDATE bdopsflow_api_tests SET name = ?, type = ?, config = ?, updated_at = ?
		WHERE id = ?
	`
	now := time.Now().Format(DateTimeFormat)
	stmt := rqlite.ParameterizedStatement{
		Query: query,
		Arguments: []interface{}{
			test.Name,
			test.Type,
			test.Config,
			now,
			id,
		},
	}

	result, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		slog.Error("failed to update api test", "error", err, "id", id)
		return err
	}
	if result.Err != nil {
		slog.Error("api test update result error", "error", result.Err, "id", id)
		return result.Err
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("api test not found")
	}

	return nil
}

func (s *ApiTestService) Delete(ctx context.Context, id int64) error {
	stmts := []rqlite.ParameterizedStatement{
		{Query: "DELETE FROM bdopsflow_api_test_results WHERE test_id = ?", Arguments: []interface{}{id}},
		{Query: "DELETE FROM bdopsflow_api_tests WHERE id = ?", Arguments: []interface{}{id}},
	}

	results, err := s.db.WriteParameterized(stmts)
	if err != nil {
		slog.Error("failed to delete api test", "error", err, "id", id)
		return err
	}

	// Check the second statement (DELETE from api_tests) for result
	if len(results) < 2 {
		slog.Error("unexpected number of results from batch delete", "expected", 2, "got", len(results), "id", id)
		return fmt.Errorf("api test delete failed: unexpected result count")
	}
	if results[1].Err != nil {
		slog.Error("api test delete result error", "error", results[1].Err, "id", id)
		return results[1].Err
	}
	if results[1].RowsAffected == 0 {
		return fmt.Errorf("api test not found")
	}

	return nil
}

func (s *ApiTestService) GetByID(ctx context.Context, id int64) (*model.ApiTest, error) {
	qr, err := s.db.QueryOneParameterized(rqlite.ParameterizedStatement{
		Query:     "SELECT id, name, type, config, created_by, created_at, updated_at FROM bdopsflow_api_tests WHERE id = ?",
		Arguments: []interface{}{id},
	})
	if err != nil {
		slog.Error("failed to get api test", "error", err, "id", id)
		return nil, err
	}
	if qr.Err != nil {
		slog.Error("api test query result error", "error", qr.Err, "id", id)
		return nil, qr.Err
	}

	if !qr.Next() {
		return nil, fmt.Errorf("api test not found")
	}

	row, err := qr.Slice()
	if err != nil {
		return nil, err
	}

	test := &model.ApiTest{
		ID:        rowInt64(row[0]),
		Name:      rowString(row[1]),
		Type:      rowString(row[2]),
		Config:    rowString(row[3]),
		CreatedBy: rowInt64(row[4]),
		CreatedAt: parseDateTime(row[5]),
		UpdatedAt: parseDateTime(row[6]),
	}

	return test, nil
}

func (s *ApiTestService) ListByUser(ctx context.Context, userID int64, isAdmin bool, testType string, page, pageSize int) ([]*model.ApiTest, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	var conditions []string
	var args []interface{}

	// 管理员可查看所有记录，普通用户只能查看自己创建的
	if !isAdmin {
		conditions = append(conditions, "created_by = ?")
		args = append(args, userID)
	}

	if testType != "" {
		conditions = append(conditions, "type = ?")
		args = append(args, testType)
	}

	var whereClause string
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count query
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM bdopsflow_api_tests %s", whereClause)
	countStmt := rqlite.ParameterizedStatement{
		Query:     countQuery,
		Arguments: args,
	}
	countQR, err := s.db.QueryOneParameterized(countStmt)
	if err != nil {
		slog.Error("failed to count api tests", "error", err, "user_id", userID)
		return nil, 0, err
	}
	if countQR.Err != nil {
		slog.Error("api test count query result error", "error", countQR.Err, "user_id", userID)
		return nil, 0, countQR.Err
	}

	var total int64
	if countQR.Next() {
		row, sliceErr := countQR.Slice()
		if sliceErr != nil {
			slog.Warn("failed to slice count row", "error", sliceErr)
		} else {
			total = rowInt64(row[0])
		}
	}

	// Data query
	offset := (page - 1) * pageSize
	dataQuery := fmt.Sprintf(
		"SELECT id, name, type, config, created_by, created_at, updated_at FROM bdopsflow_api_tests %s ORDER BY created_at DESC LIMIT %d OFFSET %d",
		whereClause, pageSize, offset,
	)
	dataStmt := rqlite.ParameterizedStatement{
		Query:     dataQuery,
		Arguments: args,
	}

	qr, err := s.db.QueryOneParameterized(dataStmt)
	if err != nil {
		slog.Error("failed to list api tests", "error", err, "user_id", userID)
		return nil, 0, err
	}
	if qr.Err != nil {
		slog.Error("api test list query result error", "error", qr.Err, "user_id", userID)
		return nil, 0, qr.Err
	}

	var tests []*model.ApiTest
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			slog.Warn("failed to slice api test row", "error", err)
			continue
		}

		test := &model.ApiTest{
			ID:        rowInt64(row[0]),
			Name:      rowString(row[1]),
			Type:      rowString(row[2]),
			Config:    rowString(row[3]),
			CreatedBy: rowInt64(row[4]),
			CreatedAt: parseDateTime(row[5]),
			UpdatedAt: parseDateTime(row[6]),
		}
		tests = append(tests, test)
	}

	return tests, total, nil
}

func (s *ApiTestService) SaveResult(ctx context.Context, result *model.ApiTestResult) (*model.ApiTestResult, error) {
	// Truncate body to 100KB before storing in the database
	const maxStoredBodySize = 100 * 1024
	if len(result.Body) > maxStoredBodySize {
		result.Body = result.Body[:maxStoredBodySize] + "\n... [truncated]"
	}

	query := `
		INSERT INTO bdopsflow_api_test_results (test_id, type, status_code, latency_ms, headers, body, error, assertions_result, executed_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	now := time.Now().Format(DateTimeFormat)
	stmt := rqlite.ParameterizedStatement{
		Query: query,
		Arguments: []interface{}{
			result.TestID,
			result.Type,
			result.StatusCode,
			result.LatencyMs,
			result.Headers,
			result.Body,
			result.Error,
			result.AssertionsResult,
			result.ExecutedBy,
			now,
		},
	}

	wr, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		slog.Error("failed to save api test result", "error", err, "test_id", result.TestID)
		return nil, err
	}
	if wr.Err != nil {
		slog.Error("api test result save result error", "error", wr.Err, "test_id", result.TestID)
		return nil, wr.Err
	}

	result.ID = wr.LastInsertID
	parsedTime, parseErr := time.Parse(DateTimeFormat, now)
	if parseErr != nil {
		slog.Warn("failed to parse api test result created_at time, using zero value", "error", parseErr)
		parsedTime = time.Now()
	}
	result.CreatedAt = parsedTime
	return result, nil
}

func (s *ApiTestService) GetResults(ctx context.Context, testID int64, page, pageSize int) ([]*model.ApiTestResult, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	// Count query
	countStmt := rqlite.ParameterizedStatement{
		Query:     "SELECT COUNT(*) FROM bdopsflow_api_test_results WHERE test_id = ?",
		Arguments: []interface{}{testID},
	}
	countQR, err := s.db.QueryOneParameterized(countStmt)
	if err != nil {
		slog.Error("failed to count api test results", "error", err, "test_id", testID)
		return nil, 0, err
	}
	if countQR.Err != nil {
		slog.Error("api test result count query result error", "error", countQR.Err, "test_id", testID)
		return nil, 0, countQR.Err
	}

	var total int64
	if countQR.Next() {
		row, sliceErr := countQR.Slice()
		if sliceErr != nil {
			slog.Warn("failed to slice count row", "error", sliceErr)
		} else {
			total = rowInt64(row[0])
		}
	}

	// Data query
	offset := (page - 1) * pageSize
	dataQuery := fmt.Sprintf(
		"SELECT id, test_id, COALESCE(NULLIF(type, ''), 'http') as type, status_code, latency_ms, headers, body, error, assertions_result, executed_by, created_at FROM bdopsflow_api_test_results WHERE test_id = ? ORDER BY created_at DESC LIMIT %d OFFSET %d",
		pageSize, offset,
	)
	dataStmt := rqlite.ParameterizedStatement{
		Query:     dataQuery,
		Arguments: []interface{}{testID},
	}

	qr, err := s.db.QueryOneParameterized(dataStmt)
	if err != nil {
		slog.Error("failed to get api test results", "error", err, "test_id", testID)
		return nil, 0, err
	}
	if qr.Err != nil {
		slog.Error("api test results query result error", "error", qr.Err, "test_id", testID)
		return nil, 0, qr.Err
	}

	var results []*model.ApiTestResult
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			slog.Warn("failed to slice api test result row", "error", err)
			continue
		}

		r := &model.ApiTestResult{
			ID:               rowInt64(row[0]),
			TestID:           rowInt64(row[1]),
			Type:             rowString(row[2]),
			StatusCode:       rowInt(row[3]),
			LatencyMs:        rowInt64(row[4]),
			Headers:          rowString(row[5]),
			Body:             rowString(row[6]),
			Error:            rowString(row[7]),
			AssertionsResult: rowString(row[8]),
			ExecutedBy:       rowInt64(row[9]),
			CreatedAt:        parseDateTime(row[10]),
		}
		results = append(results, r)
	}

	return results, total, nil
}

// ListResultsByUser returns all test results for a given user with optional type filter.
// System admin can see all results; other users can only see their own.
func (s *ApiTestService) ListResultsByUser(ctx context.Context, userID int64, isAdmin bool, resultType string, page, pageSize int) ([]*model.ApiTestResult, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	// Build WHERE clause
	var conditions []string
	var args []interface{}

	// 管理员可查看所有记录，普通用户只能查看自己执行的
	if !isAdmin {
		conditions = append(conditions, "r.executed_by = ?")
		args = append(args, userID)
	}

	if resultType != "" {
		conditions = append(conditions, "r.type = ?")
		args = append(args, resultType)
	}

	var whereClause string
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count query
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM bdopsflow_api_test_results r %s", whereClause)
	countStmt := rqlite.ParameterizedStatement{
		Query:     countQuery,
		Arguments: args,
	}
	countQR, err := s.db.QueryOneParameterized(countStmt)
	if err != nil {
		slog.Error("failed to count api test results by user", "error", err, "user_id", userID)
		return nil, 0, err
	}
	if countQR.Err != nil {
		slog.Error("api test result count by user query result error", "error", countQR.Err, "user_id", userID)
		return nil, 0, countQR.Err
	}

	var total int64
	if countQR.Next() {
		row, sliceErr := countQR.Slice()
		if sliceErr != nil {
			slog.Warn("failed to slice count row", "error", sliceErr)
		} else {
			total = rowInt64(row[0])
		}
	}

	// Data query with LEFT JOIN to get test name and fallback type
	offset := (page - 1) * pageSize
	dataQuery := fmt.Sprintf(
		"SELECT r.id, r.test_id, COALESCE(NULLIF(r.type, ''), t.type, '') as type, r.status_code, r.latency_ms, r.headers, r.body, r.error, r.assertions_result, r.executed_by, r.created_at, COALESCE(t.name, '') as test_name FROM bdopsflow_api_test_results r LEFT JOIN bdopsflow_api_tests t ON r.test_id = t.id %s ORDER BY r.created_at DESC LIMIT %d OFFSET %d",
		whereClause, pageSize, offset,
	)
	dataStmt := rqlite.ParameterizedStatement{
		Query:     dataQuery,
		Arguments: args,
	}

	qr, err := s.db.QueryOneParameterized(dataStmt)
	if err != nil {
		slog.Error("failed to list api test results by user", "error", err, "user_id", userID)
		return nil, 0, err
	}
	if qr.Err != nil {
		slog.Error("api test results by user query result error", "error", qr.Err, "user_id", userID)
		return nil, 0, qr.Err
	}

	var results []*model.ApiTestResult
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			slog.Warn("failed to slice api test result row", "error", err)
			continue
		}

		r := &model.ApiTestResult{
			ID:               rowInt64(row[0]),
			TestID:           rowInt64(row[1]),
			Type:             rowString(row[2]),
			StatusCode:       rowInt(row[3]),
			LatencyMs:        rowInt64(row[4]),
			Headers:          rowString(row[5]),
			Body:             rowString(row[6]),
			Error:            rowString(row[7]),
			AssertionsResult: rowString(row[8]),
			ExecutedBy:       rowInt64(row[9]),
			CreatedAt:        parseDateTime(row[10]),
		}
		// Attach test name if available (index 11)
		if len(row) > 11 {
			r.TestName = rowString(row[11])
		}
		results = append(results, r)
	}

	return results, total, nil
}

func (s *ApiTestService) DeleteResult(ctx context.Context, id int64) error {
	stmt := rqlite.ParameterizedStatement{
		Query:     "DELETE FROM bdopsflow_api_test_results WHERE id = ?",
		Arguments: []interface{}{id},
	}

	result, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		slog.Error("failed to delete api test result", "error", err, "id", id)
		return err
	}
	if result.Err != nil {
		slog.Error("api test result delete result error", "error", result.Err, "id", id)
		return result.Err
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("api test not found")
	}

	return nil
}

func (s *ApiTestService) GetResultByID(ctx context.Context, id int64) (*model.ApiTestResult, error) {
	qr, err := s.db.QueryOneParameterized(rqlite.ParameterizedStatement{
		Query:     "SELECT id, test_id, type, status_code, latency_ms, headers, body, error, assertions_result, executed_by, created_at FROM bdopsflow_api_test_results WHERE id = ?",
		Arguments: []interface{}{id},
	})
	if err != nil {
		slog.Error("failed to get api test result", "error", err, "id", id)
		return nil, err
	}
	if qr.Err != nil {
		slog.Error("api test result query result error", "error", qr.Err, "id", id)
		return nil, qr.Err
	}

	if !qr.Next() {
		return nil, fmt.Errorf("api test not found")
	}

	row, err := qr.Slice()
	if err != nil {
		return nil, err
	}

	result := &model.ApiTestResult{
		ID:               rowInt64(row[0]),
		TestID:           rowInt64(row[1]),
		Type:             rowString(row[2]),
		StatusCode:       rowInt(row[3]),
		LatencyMs:        rowInt64(row[4]),
		Headers:          rowString(row[5]),
		Body:             rowString(row[6]),
		Error:            rowString(row[7]),
		AssertionsResult: rowString(row[8]),
		ExecutedBy:       rowInt64(row[9]),
		CreatedAt:        parseDateTime(row[10]),
	}

	return result, nil
}
