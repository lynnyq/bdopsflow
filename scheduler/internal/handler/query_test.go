package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/datasource/driver"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
)

func TestIsSelectOnly_AllowedReadOnlySQL(t *testing.T) {
	h := &QueryHandler{}
	allowed := []struct {
		name string
		sql  string
	}{
		{"SELECT", "SELECT * FROM users"},
		{"SELECT lowercase", "select * from users"},
		{"SELECT with WHERE", "SELECT id, name FROM users WHERE id = 1"},
		{"WITH CTE", "WITH cte AS (SELECT 1) SELECT * FROM cte"},
		{"EXPLAIN", "EXPLAIN SELECT * FROM users"},
		{"SHOW TABLES", "SHOW TABLES"},
		{"SHOW COLUMNS", "SHOW COLUMNS FROM users"},
		{"DESCRIBE", "DESCRIBE users"},
		{"DESCRIBE lowercase", "describe users"},
		{"DESC with space", "DESC users"},
		{"DESC lowercase", "desc users"},
		{"PRAGMA", "PRAGMA table_info(users)"},
		{"DESC with newline", "desc\n  bdopsflow_domains\n"},
		{"DESC with tab", "desc\tbdopsflow_domains"},
		{"DESC with CRLF", "desc\r\nbdopsflow_domains\r\n"},
		{"SELECT with newline", "SELECT\n*\nFROM users"},
		{"SHOW with newline", "SHOW\nTABLES"},
		{"DESCRIBE with newline", "DESCRIBE\n  users"},
	}

	for _, tc := range allowed {
		t.Run(tc.name, func(t *testing.T) {
			if !h.isSelectOnly(tc.sql, false) {
				t.Errorf("isSelectOnly(%q, false) = false, want true", tc.sql)
			}
		})
	}
}

func TestIsSelectOnly_DeniedWriteSQL(t *testing.T) {
	h := &QueryHandler{}
	denied := []struct {
		name string
		sql  string
	}{
		{"INSERT", "INSERT INTO users (name) VALUES ('test')"},
		{"UPDATE", "UPDATE users SET name = 'test' WHERE id = 1"},
		{"DELETE", "DELETE FROM users WHERE id = 1"},
		{"DROP TABLE", "DROP TABLE users"},
		{"CREATE TABLE", "CREATE TABLE test (id INT)"},
		{"ALTER TABLE", "ALTER TABLE users ADD COLUMN age INT"},
		{"TRUNCATE", "TRUNCATE TABLE users"},
		{"REPLACE", "REPLACE INTO users (id, name) VALUES (1, 'test')"},
	}

	for _, tc := range denied {
		t.Run(tc.name, func(t *testing.T) {
			if h.isSelectOnly(tc.sql, false) {
				t.Errorf("isSelectOnly(%q, false) = true, want false", tc.sql)
			}
		})
	}
}

func TestIsSelectOnly_AllowWriteEnabled(t *testing.T) {
	h := &QueryHandler{}
	writeSQL := []string{
		"INSERT INTO users (name) VALUES ('test')",
		"UPDATE users SET name = 'test'",
		"DELETE FROM users WHERE id = 1",
		"DROP TABLE users",
	}

	for _, sql := range writeSQL {
		if !h.isSelectOnly(sql, true) {
			t.Errorf("isSelectOnly(%q, true) = false, want true", sql)
		}
	}
}

func TestIsSelectOnly_DescNotConfusedWithOrder(t *testing.T) {
	h := &QueryHandler{}

	if !h.isSelectOnly("SELECT * FROM users ORDER BY id DESC", false) {
		t.Error("SELECT with ORDER BY DESC should be allowed")
	}

	if !h.isSelectOnly("DESC users", false) {
		t.Error("DESC users should be allowed as read-only")
	}

	if !h.isSelectOnly("DESCRIBE users", false) {
		t.Error("DESCRIBE users should be allowed as read-only")
	}
}

func TestIsSelectOnly_EdgeCases(t *testing.T) {
	h := &QueryHandler{}

	if h.isSelectOnly("", false) {
		t.Error("empty SQL should not be allowed")
	}

	if h.isSelectOnly("   ", false) {
		t.Error("whitespace-only SQL should not be allowed")
	}

	if !h.isSelectOnly("  SELECT  * FROM users  ", false) {
		t.Error("SELECT with leading/trailing whitespace should be allowed")
	}

	if !h.isSelectOnly("desc users", false) {
		t.Error("lowercase desc should be allowed")
	}

	if !h.isSelectOnly("describe users", false) {
		t.Error("lowercase describe should be allowed")
	}
}

func TestJoinSpaces(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"hello world", "hello world"},
		{"hello  world", "hello world"},
		{"hello\nworld", "hello world"},
		{"hello\r\nworld", "hello world"},
		{"hello\tworld", "hello world"},
		{"hello  \n  world", "hello world"},
		{"desc\n  bdopsflow_domains", "desc bdopsflow_domains"},
	}

	for _, tc := range tests {
		got := joinSpaces(tc.input)
		if got != tc.expect {
			t.Errorf("joinSpaces(%q) = %q, want %q", tc.input, got, tc.expect)
		}
	}
}

func TestQueryRegistry_UpdateResult_NilResult(t *testing.T) {
	registry := NewQueryRegistry()
	queryID := "q_test_nil_result"
	registry.Register(&RunningQuery{
		QueryID:    queryID,
		Status:     QueryStatusPending,
		CancelFunc: func() {},
	})

	registry.UpdateResult(queryID, nil, 1.0)

	q, ok := registry.Get(queryID)
	if !ok {
		t.Fatal("query should exist in registry")
	}
	if q.Status != QueryStatusCompleted {
		t.Errorf("status = %v, want %v", q.Status, QueryStatusCompleted)
	}
	if q.Result != nil {
		t.Errorf("result should be nil, got %v", q.Result)
	}
}

func TestGetResult_NilResultNoPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registry := NewQueryRegistry()
	queryID := "q_test_nil_get"
	registry.Register(&RunningQuery{
		QueryID:    queryID,
		Status:     QueryStatusCompleted,
		Result:     nil,
		CancelFunc: func() {},
	})

	h := &QueryHandler{registry: registry}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "query_id", Value: queryID}}

	h.GetResult(c)

	if w.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("response data should be a map")
	}

	if data["status"] != "completed" {
		t.Errorf("status = %v, want completed", data["status"])
	}

	columns, ok := data["columns"].([]interface{})
	if !ok {
		t.Fatal("columns should be present in response")
	}
	if len(columns) != 0 {
		t.Errorf("columns length = %d, want 0", len(columns))
	}
}

func TestGetResult_NonNilResult(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registry := NewQueryRegistry()
	queryID := "q_test_nonnil"
	registry.Register(&RunningQuery{
		QueryID: queryID,
		Status:  QueryStatusCompleted,
		Result: &driver.QueryResult{
			Columns:  []string{"id", "name"},
			Rows:     [][]interface{}{{int64(1), "test"}},
			RowCount: 1,
		},
		CancelFunc: func() {},
	})

	h := &QueryHandler{registry: registry}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "query_id", Value: queryID}}

	h.GetResult(c)

	if w.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("response data should be a map")
	}

	if data["status"] != "completed" {
		t.Errorf("status = %v, want completed", data["status"])
	}

	rowCount, ok := data["row_count"].(float64)
	if !ok {
		t.Fatal("row_count should be a number")
	}
	if rowCount != 1 {
		t.Errorf("row_count = %v, want 1", rowCount)
	}
}

func TestExecuteQuerySafe_PanicRecovery(t *testing.T) {
	registry := NewQueryRegistry()
	queryID := "q_test_panic"
	registry.Register(&RunningQuery{
		QueryID:    queryID,
		Status:     QueryStatusRunning,
		CancelFunc: func() {},
	})

	h := &QueryHandler{registry: registry}

	ds := &model.Datasource{ID: 1, Name: "test", Type: "hive"}
	req := struct {
		DatasourceID int64  `json:"datasource_id" binding:"required"`
		SQL          string `json:"sql" binding:"required"`
		Database     string `json:"database"`
	}{
		DatasourceID: 1,
		SQL:          "SELECT 1",
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer func() { done <- struct{}{} }()
		h.executeQuerySafe(ctx, cancel, queryID, ds, req, 1, 1, 1000)
	}()

	<-done

	q, ok := registry.Get(queryID)
	if !ok {
		t.Fatal("query should exist in registry")
	}

	if q.Status != QueryStatusFailed {
		t.Errorf("status = %v, want %v", q.Status, QueryStatusFailed)
	}
}

func TestGetResult_FailedStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registry := NewQueryRegistry()
	queryID := "q_test_failed"
	registry.Register(&RunningQuery{
		QueryID:    queryID,
		Status:     QueryStatusFailed,
		Error:      "hive query error",
		CancelFunc: func() {},
	})

	h := &QueryHandler{registry: registry}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "query_id", Value: queryID}}

	h.GetResult(c)

	if w.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("response data should be a map")
	}

	if data["status"] != "failed" {
		t.Errorf("status = %v, want failed", data["status"])
	}
	if data["error"] != "hive query error" {
		t.Errorf("error = %v, want 'hive query error'", data["error"])
	}
}

func TestQueryRegistry_UpdateError_SetsFailedStatus(t *testing.T) {
	registry := NewQueryRegistry()
	queryID := "q_test_update_error"
	registry.Register(&RunningQuery{
		QueryID:    queryID,
		Status:     QueryStatusRunning,
		CancelFunc: func() {},
	})

	errMsg := "hive query error: SemanticException Table not found"
	registry.UpdateError(queryID, errMsg, 2.5)

	q, ok := registry.Get(queryID)
	if !ok {
		t.Fatal("query should exist in registry")
	}
	if q.Status != QueryStatusFailed {
		t.Errorf("status = %v, want %v", q.Status, QueryStatusFailed)
	}
	if q.Error != errMsg {
		t.Errorf("error = %v, want %v", q.Error, errMsg)
	}
	if q.ExecutionTime != 2.5 {
		t.Errorf("execution_time = %v, want 2.5", q.ExecutionTime)
	}
}

func TestGetResult_HiveErrorPropagation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testCases := []struct {
		name       string
		errorMsg   string
		wantStatus string
	}{
		{
			name:       "semantic error",
			errorMsg:   "hive query error: SemanticException Table not found",
			wantStatus: "failed",
		},
		{
			name:       "permission error",
			errorMsg:   "hive query error: org.apache.hadoop.hive.ql.metadata.AuthorizationException",
			wantStatus: "failed",
		},
		{
			name:       "syntax error",
			errorMsg:   "hive query error: org.apache.hadoop.hive.ql.parse.ParseException",
			wantStatus: "failed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			registry := NewQueryRegistry()
			queryID := "q_test_hive_err_" + tc.name
			registry.Register(&RunningQuery{
				QueryID:    queryID,
				Status:     QueryStatusFailed,
				Error:      tc.errorMsg,
				CancelFunc: func() {},
			})

			h := &QueryHandler{registry: registry}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = gin.Params{{Key: "query_id", Value: queryID}}

			h.GetResult(c)

			var resp map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}

			data := resp["data"].(map[string]interface{})
			if data["status"] != tc.wantStatus {
				t.Errorf("status = %v, want %v", data["status"], tc.wantStatus)
			}
			if data["error"] != tc.errorMsg {
				t.Errorf("error = %v, want %v", data["error"], tc.errorMsg)
			}
		})
	}
}

func TestUpdateSavedSQL_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := &QueryHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "abc"}}

	h.UpdateSavedSQL(c)

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["code"] != float64(CodeBadRequest) {
		t.Errorf("code = %v, want %d", resp["code"], CodeBadRequest)
	}
}

func TestUpdateSavedSQL_MissingRequiredFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := &QueryHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	c.Request = httptest.NewRequest(http.MethodPut, "/", nil)

	h.UpdateSavedSQL(c)

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["code"] != float64(CodeBadRequest) {
		t.Errorf("code = %v, want %d", resp["code"], CodeBadRequest)
	}
}

func TestQueryRegistry_UpdateError_OverwritesCompleted(t *testing.T) {
	registry := NewQueryRegistry()
	queryID := "q_test_overwrite"
	registry.Register(&RunningQuery{
		QueryID:    queryID,
		Status:     QueryStatusRunning,
		CancelFunc: func() {},
	})

	registry.UpdateResult(queryID, &driver.QueryResult{
		Columns:  []string{"id"},
		Rows:     [][]interface{}{{1}},
		RowCount: 1,
	}, 1.0)

	q, _ := registry.Get(queryID)
	if q.Status != QueryStatusCompleted {
		t.Errorf("status should be completed after UpdateResult, got %v", q.Status)
	}

	registry.UpdateError(queryID, "hive query error: late error", 1.0)

	q, _ = registry.Get(queryID)
	if q.Status != QueryStatusFailed {
		t.Errorf("status should be failed after UpdateError, got %v", q.Status)
	}
	if q.Error != "hive query error: late error" {
		t.Errorf("error = %v, want 'hive query error: late error'", q.Error)
	}
}
