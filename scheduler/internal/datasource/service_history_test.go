package datasource

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
	rqlite "github.com/rqlite/gorqlite"
)

// ==================== RecordQueryHistory ====================

func TestDatasourceService_RecordQueryHistory_Success(t *testing.T) {
	db := &dsMockDB{
		writeResult: database.NewWriteResult(1, 1),
	}
	svc := NewDatasourceService(db, nil, nil)

	dsID := int64(5)
	execBy := int64(100)
	history := &model.QueryHistory{
		QueryID:        "q-001",
		DatasourceID:   &dsID,
		DatasourceName: "test-ds",
		SQLText:        "SELECT 1",
		Database:       "testdb",
		ExecutionTime:  0.05,
		RowCount:       1,
		Status:         "success",
		ExecutedBy:     &execBy,
		DomainID:       1,
	}

	err := svc.RecordQueryHistory(context.Background(), history)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if history.ID != 1 {
		t.Errorf("expected ID=1, got %d", history.ID)
	}
}

func TestDatasourceService_RecordQueryHistory_NilOptionals(t *testing.T) {
	db := &dsMockDB{
		writeResult: database.NewWriteResult(2, 1),
	}
	svc := NewDatasourceService(db, nil, nil)

	history := &model.QueryHistory{
		QueryID: "q-002",
		// DatasourceID 和 ExecutedBy 都为 nil
		SQLText: "SELECT 2",
		Status:  "success",
	}

	err := svc.RecordQueryHistory(context.Background(), history)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestDatasourceService_RecordQueryHistory_WriteError(t *testing.T) {
	db := &dsMockDB{
		writeError: errors.New("write failed"),
	}
	svc := NewDatasourceService(db, nil, nil)

	history := &model.QueryHistory{QueryID: "q-err", SQLText: "SELECT 1", Status: "success"}
	err := svc.RecordQueryHistory(context.Background(), history)
	if err == nil {
		t.Fatal("expected error on write failure")
	}
	if !strings.Contains(err.Error(), "failed to record query history") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ==================== GetQueryHistory ====================

func TestDatasourceService_GetQueryHistory_Success(t *testing.T) {
	dsID := int64(5)
	execBy := int64(100)
	now := time.Now()

	db := &dsMockDB{
		queryResults: []rqlite.QueryResult{
			database.NewQueryResultWithRows([][]interface{}{{int64(1)}}), // count
			database.NewQueryResultWithRows([][]interface{}{
				{
					int64(1),           // id
					"q-001",            // query_id
					dsID,               // datasource_id
					"test-ds",          // datasource_name
					"SELECT 1",         // sql_text
					"testdb",           // database
					float64(0.05),      // execution_time
					int64(1),           // row_count
					"success",          // status
					"",                 // error_message
					execBy,             // executed_by
					"Alice",            // executed_by_name
					int64(1),           // domain_id
					now,                // created_at (time.Time)
				},
			}),
		},
	}
	svc := NewDatasourceService(db, nil, nil)

	histories, total, err := svc.GetQueryHistory(context.Background(), 1, 1, 10, 5, "success", "", "", 100)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total != 1 {
		t.Errorf("expected total=1, got %d", total)
	}
	if len(histories) != 1 {
		t.Fatalf("expected 1 history, got %d", len(histories))
	}
	h := histories[0]
	if h.ID != 1 {
		t.Errorf("expected ID=1, got %d", h.ID)
	}
	if h.QueryID != "q-001" {
		t.Errorf("expected QueryID='q-001', got %q", h.QueryID)
	}
	if h.DatasourceID == nil || *h.DatasourceID != 5 {
		t.Errorf("expected DatasourceID=5, got %v", h.DatasourceID)
	}
	if h.ExecutedBy == nil || *h.ExecutedBy != 100 {
		t.Errorf("expected ExecutedBy=100, got %v", h.ExecutedBy)
	}
	if h.ExecutedByName != "Alice" {
		t.Errorf("expected ExecutedByName='Alice', got %q", h.ExecutedByName)
	}
	if h.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestDatasourceService_GetQueryHistory_StringTime(t *testing.T) {
	db := &dsMockDB{
		queryResults: []rqlite.QueryResult{
			database.NewQueryResultWithRows([][]interface{}{{int64(1)}}),
			database.NewQueryResultWithRows([][]interface{}{
				{
					int64(1), "q-001", nil, "test-ds", "SELECT 1", "db",
					float64(0.1), int64(0), "success", "", nil, "",
					int64(1), "2025-01-01T12:00:00Z", // created_at as string
				},
			}),
		},
	}
	svc := NewDatasourceService(db, nil, nil)

	histories, _, err := svc.GetQueryHistory(context.Background(), 1, 1, 10, 0, "", "", "", 0)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(histories) != 1 {
		t.Fatalf("expected 1 history, got %d", len(histories))
	}
	if histories[0].CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt from string")
	}
	if histories[0].DatasourceID != nil {
		t.Errorf("expected nil DatasourceID, got %v", histories[0].DatasourceID)
	}
}

func TestDatasourceService_GetQueryHistory_WithSearch(t *testing.T) {
	db := &dsMockDB{
		queryResults: []rqlite.QueryResult{
			database.NewQueryResultWithRows([][]interface{}{{int64(0)}}),
			database.NewQueryResultWithRows([][]interface{}{}),
		},
	}
	svc := NewDatasourceService(db, nil, nil)

	histories, total, err := svc.GetQueryHistory(context.Background(), 1, 1, 10, 0, "", "2025-01-01", "2025-12-31", 0, "keyword")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total != 0 {
		t.Errorf("expected total=0, got %d", total)
	}
	if len(histories) != 0 {
		t.Errorf("expected 0 histories, got %d", len(histories))
	}
}

func TestDatasourceService_GetQueryHistory_NoArgs(t *testing.T) {
	// 没有过滤参数时走 QueryOne 路径
	db := &dsMockDB{
		queryResults: []rqlite.QueryResult{
			database.NewQueryResultWithRows([][]interface{}{{int64(0)}}),
			database.NewQueryResultWithRows([][]interface{}{}),
		},
	}
	svc := NewDatasourceService(db, nil, nil)

	_, _, err := svc.GetQueryHistory(context.Background(), 0, 1, 10, 0, "", "", "", 0)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestDatasourceService_GetQueryHistory_CountError(t *testing.T) {
	db := &dsMockDB{
		queryError: errors.New("count failed"),
	}
	svc := NewDatasourceService(db, nil, nil)

	_, _, err := svc.GetQueryHistory(context.Background(), 1, 1, 10, 5, "", "", "", 0)
	if err == nil {
		t.Fatal("expected error on count failure")
	}
}

// ==================== DeleteQueryHistory ====================

func TestDatasourceService_DeleteQueryHistory_Success(t *testing.T) {
	db := &dsMockDB{
		writeResult: database.NewWriteResult(0, 1),
	}
	svc := NewDatasourceService(db, nil, nil)

	err := svc.DeleteQueryHistory(context.Background(), 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestDatasourceService_DeleteQueryHistory_WriteError(t *testing.T) {
	db := &dsMockDB{
		writeError: errors.New("delete failed"),
	}
	svc := NewDatasourceService(db, nil, nil)

	err := svc.DeleteQueryHistory(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error on write failure")
	}
	if !strings.Contains(err.Error(), "failed to delete query history") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ==================== BatchDeleteQueryHistory ====================

func TestDatasourceService_BatchDeleteQueryHistory_Empty(t *testing.T) {
	svc := NewDatasourceService(&dsMockDB{}, nil, nil)

	err := svc.BatchDeleteQueryHistory(context.Background(), []int64{})
	if err != nil {
		t.Errorf("expected no error for empty list, got %v", err)
	}
}

func TestDatasourceService_BatchDeleteQueryHistory_Success(t *testing.T) {
	db := &dsMockDB{
		writeResult: database.NewWriteResult(0, 3),
	}
	svc := NewDatasourceService(db, nil, nil)

	err := svc.BatchDeleteQueryHistory(context.Background(), []int64{1, 2, 3})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(db.writeStmts) != 1 {
		t.Errorf("expected 1 write call, got %d", len(db.writeStmts))
	}
}

func TestDatasourceService_BatchDeleteQueryHistory_WriteError(t *testing.T) {
	db := &dsMockDB{
		writeError: errors.New("batch delete failed"),
	}
	svc := NewDatasourceService(db, nil, nil)

	err := svc.BatchDeleteQueryHistory(context.Background(), []int64{1, 2})
	if err == nil {
		t.Fatal("expected error on write failure")
	}
	if !strings.Contains(err.Error(), "failed to batch delete query history") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ==================== CreateSavedSQL ====================

func TestDatasourceService_CreateSavedSQL_Success(t *testing.T) {
	db := &dsMockDB{
		writeResult: database.NewWriteResult(1, 1),
	}
	svc := NewDatasourceService(db, nil, nil)

	createdBy := int64(10)
	updatedBy := int64(10)
	saved := &model.SavedSQL{
		Name:         "my-query",
		DatasourceID: 1,
		Database:     "testdb",
		SQLText:      "SELECT 1",
		Description:  "test query",
		CreatedBy:    &createdBy,
		UpdatedBy:    &updatedBy,
		DomainID:     1,
		IsPublic:     true,
	}

	err := svc.CreateSavedSQL(context.Background(), saved)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if saved.ID != 1 {
		t.Errorf("expected ID=1, got %d", saved.ID)
	}
}

func TestDatasourceService_CreateSavedSQL_NilOptionals(t *testing.T) {
	db := &dsMockDB{
		writeResult: database.NewWriteResult(2, 1),
	}
	svc := NewDatasourceService(db, nil, nil)

	saved := &model.SavedSQL{
		Name:         "minimal",
		DatasourceID: 1,
		SQLText:      "SELECT 1",
		DomainID:     1,
	}

	err := svc.CreateSavedSQL(context.Background(), saved)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestDatasourceService_CreateSavedSQL_WriteError(t *testing.T) {
	db := &dsMockDB{
		writeError: errors.New("insert failed"),
	}
	svc := NewDatasourceService(db, nil, nil)

	saved := &model.SavedSQL{Name: "test", SQLText: "SELECT 1"}
	err := svc.CreateSavedSQL(context.Background(), saved)
	if err == nil {
		t.Fatal("expected error on write failure")
	}
	if !strings.Contains(err.Error(), "failed to create saved SQL") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ==================== GetSavedSQL ====================

func TestDatasourceService_GetSavedSQL_Success(t *testing.T) {
	createdBy := int64(10)
	updatedBy := int64(20)
	now := time.Now()

	db := &dsMockDB{
		queryResults: []rqlite.QueryResult{
			database.NewQueryResultWithRows([][]interface{}{{int64(1)}}), // count
			database.NewQueryResultWithRows([][]interface{}{
				{
					int64(1),               // id
					"my-query",             // name
					int64(1),               // datasource_id
					"test-ds",              // datasource_name
					"testdb",               // database
					"SELECT 1",             // sql_text
					"test query",           // description
					createdBy,              // created_by
					"Alice",                // created_by_name
					updatedBy,              // updated_by
					"Bob",                  // updated_by_name
					int64(1),               // domain_id
					int64(1),               // is_public
					now,                    // created_at (time.Time)
					now,                    // updated_at (time.Time)
				},
			}),
		},
	}
	svc := NewDatasourceService(db, nil, nil)

	savedList, total, err := svc.GetSavedSQL(context.Background(), 1, 10, 1, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total != 1 {
		t.Errorf("expected total=1, got %d", total)
	}
	if len(savedList) != 1 {
		t.Fatalf("expected 1 saved SQL, got %d", len(savedList))
	}
	s := savedList[0]
	if s.ID != 1 {
		t.Errorf("expected ID=1, got %d", s.ID)
	}
	if s.Name != "my-query" {
		t.Errorf("expected name 'my-query', got %q", s.Name)
	}
	if s.DatasourceName != "test-ds" {
		t.Errorf("expected datasource name 'test-ds', got %q", s.DatasourceName)
	}
	if !s.IsPublic {
		t.Error("expected IsPublic=true")
	}
	if s.CreatedBy == nil || *s.CreatedBy != 10 {
		t.Errorf("expected CreatedBy=10, got %v", s.CreatedBy)
	}
	if s.UpdatedBy == nil || *s.UpdatedBy != 20 {
		t.Errorf("expected UpdatedBy=20, got %v", s.UpdatedBy)
	}
	if s.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestDatasourceService_GetSavedSQL_StringTime(t *testing.T) {
	db := &dsMockDB{
		queryResults: []rqlite.QueryResult{
			database.NewQueryResultWithRows([][]interface{}{{int64(1)}}),
			database.NewQueryResultWithRows([][]interface{}{
				{
					int64(1), "q", int64(1), "ds", "db", "SELECT 1", "desc",
					nil, "", nil, "", int64(1), int64(0),
					"2025-01-01T00:00:00Z", // created_at as string
					"2025-01-02T00:00:00Z", // updated_at as string
				},
			}),
		},
	}
	svc := NewDatasourceService(db, nil, nil)

	savedList, _, err := svc.GetSavedSQL(context.Background(), 1, 10, 1, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(savedList) != 1 {
		t.Fatalf("expected 1 saved SQL, got %d", len(savedList))
	}
	if savedList[0].CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt from string")
	}
	if savedList[0].UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt from string")
	}
}

func TestDatasourceService_GetSavedSQL_WithSearch(t *testing.T) {
	db := &dsMockDB{
		queryResults: []rqlite.QueryResult{
			database.NewQueryResultWithRows([][]interface{}{{int64(0)}}),
			database.NewQueryResultWithRows([][]interface{}{}),
		},
	}
	svc := NewDatasourceService(db, nil, nil)

	savedList, total, err := svc.GetSavedSQL(context.Background(), 1, 10, 1, 10, "keyword")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total != 0 {
		t.Errorf("expected total=0, got %d", total)
	}
	if len(savedList) != 0 {
		t.Errorf("expected 0 results, got %d", len(savedList))
	}
}

func TestDatasourceService_GetSavedSQL_NoArgs(t *testing.T) {
	db := &dsMockDB{
		queryResults: []rqlite.QueryResult{
			database.NewQueryResultWithRows([][]interface{}{{int64(0)}}),
			database.NewQueryResultWithRows([][]interface{}{}),
		},
	}
	svc := NewDatasourceService(db, nil, nil)

	_, _, err := svc.GetSavedSQL(context.Background(), 0, 0, 1, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestDatasourceService_GetSavedSQL_CountError(t *testing.T) {
	db := &dsMockDB{
		queryError: errors.New("count failed"),
	}
	svc := NewDatasourceService(db, nil, nil)

	_, _, err := svc.GetSavedSQL(context.Background(), 1, 10, 1, 10)
	if err == nil {
		t.Fatal("expected error on count failure")
	}
}

// ==================== UpdateSavedSQL ====================

func TestDatasourceService_UpdateSavedSQL_Success(t *testing.T) {
	db := &dsMockDB{
		writeResult: database.NewWriteResult(0, 1), // rows affected = 1
	}
	svc := NewDatasourceService(db, nil, nil)

	updatedBy := int64(20)
	saved := &model.SavedSQL{
		Name:         "updated-query",
		DatasourceID: 2,
		Database:     "newdb",
		SQLText:      "SELECT 2",
		Description:  "updated desc",
		UpdatedBy:    &updatedBy,
		IsPublic:     false,
	}

	err := svc.UpdateSavedSQL(context.Background(), 1, saved)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestDatasourceService_UpdateSavedSQL_NotFound(t *testing.T) {
	db := &dsMockDB{
		writeResult: database.NewWriteResult(0, 0), // rows affected = 0
	}
	svc := NewDatasourceService(db, nil, nil)

	saved := &model.SavedSQL{Name: "test", SQLText: "SELECT 1"}
	err := svc.UpdateSavedSQL(context.Background(), 999, saved)
	if err == nil {
		t.Fatal("expected error when rows affected = 0")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got %v", err)
	}
}

func TestDatasourceService_UpdateSavedSQL_WriteError(t *testing.T) {
	db := &dsMockDB{
		writeError: errors.New("update failed"),
	}
	svc := NewDatasourceService(db, nil, nil)

	saved := &model.SavedSQL{Name: "test", SQLText: "SELECT 1"}
	err := svc.UpdateSavedSQL(context.Background(), 1, saved)
	if err == nil {
		t.Fatal("expected error on write failure")
	}
}

// ==================== DeleteSavedSQL ====================

func TestDatasourceService_DeleteSavedSQL_Success(t *testing.T) {
	db := &dsMockDB{
		writeResult: database.NewWriteResult(0, 1),
	}
	svc := NewDatasourceService(db, nil, nil)

	err := svc.DeleteSavedSQL(context.Background(), 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestDatasourceService_DeleteSavedSQL_WriteError(t *testing.T) {
	db := &dsMockDB{
		writeError: errors.New("delete failed"),
	}
	svc := NewDatasourceService(db, nil, nil)

	err := svc.DeleteSavedSQL(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error on write failure")
	}
	if !strings.Contains(err.Error(), "failed to delete saved SQL") {
		t.Errorf("unexpected error: %v", err)
	}
}
