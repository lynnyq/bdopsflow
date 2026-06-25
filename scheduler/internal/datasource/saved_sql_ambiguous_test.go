package datasource

import (
	"context"
	"strings"
	"testing"

	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
	rqlite "github.com/rqlite/gorqlite"
)

// sqlCapturingDB 捕获 SQL 查询,用于验证 whereClause 不会产生列名二义性
type sqlCapturingDB struct {
	queries []string
}

func (m *sqlCapturingDB) QueryOne(query string) (rqlite.QueryResult, error) {
	m.queries = append(m.queries, query)
	return rqlite.QueryResult{}, nil
}

func (m *sqlCapturingDB) QueryOneParameterized(stmt rqlite.ParameterizedStatement) (rqlite.QueryResult, error) {
	m.queries = append(m.queries, stmt.Query)
	return rqlite.QueryResult{}, nil
}

func (m *sqlCapturingDB) WriteOneParameterized(stmt rqlite.ParameterizedStatement) (rqlite.WriteResult, error) {
	return rqlite.WriteResult{}, nil
}

func (m *sqlCapturingDB) WriteParameterized(sqlStatements []rqlite.ParameterizedStatement) ([]rqlite.WriteResult, error) {
	results := make([]rqlite.WriteResult, len(sqlStatements))
	return results, nil
}

var _ database.DB = (*sqlCapturingDB)(nil)

// TestGetSavedSQL_NoAmbiguousColumns 回归测试:
// bdopsflow_users 表本身有 created_by/updated_by 字段,JOIN 后 WHERE 子句必须使用表别名(s.)
// 避免列名二义性,否则 SQLite 会返回 "ambiguous column name" 错误
func TestGetSavedSQL_NoAmbiguousColumns(t *testing.T) {
	captured := &sqlCapturingDB{}
	svc := &DatasourceService{
		db:      captured,
		manager: nil,
	}

	// 调用 GetSavedSQL,虽然会因 manager=nil 在后续失败,
	// 但我们只需要确认 SQL 已经构建且已发出
	_, _, _ = svc.GetSavedSQL(context.Background(), 1, 1, 1, 20)

	if len(captured.queries) < 2 {
		t.Fatalf("expected at least 2 queries (count + data), got %d", len(captured.queries))
	}

	// 验证每条查询:WHERE 子句中如果出现 bdopsflow_users 也有的列(created_by/updated_by),必须带表前缀
	for i, q := range captured.queries {
		upper := strings.ToUpper(q)

		if !strings.Contains(upper, "JOIN BDOPSFLOW_USERS") {
			continue
		}
		whereIdx := strings.Index(upper, "WHERE")
		if whereIdx == -1 {
			continue
		}
		wherePart := upper[whereIdx:]

		// 检查未限定的 created_by/updated_by(无 s./u1./u2. 前缀)
		ambiguousPatterns := []string{
			" CREATED_BY ", " (CREATED_BY", "(CREATED_BY ",
			" UPDATED_BY ", " (UPDATED_BY", "(UPDATED_BY ",
		}
		for _, pat := range ambiguousPatterns {
			if strings.Contains(wherePart, pat) {
				t.Errorf("query[%d] has ambiguous column in WHERE clause: %s\nfull SQL: %s", i, pat, q)
			}
		}
	}
}

// TestGetQueryHistory_NoAmbiguousColumns 同上回归测试,确保 query_history JOIN users 后无列名二义
func TestGetQueryHistory_NoAmbiguousColumns(t *testing.T) {
	captured := &sqlCapturingDB{}
	svc := &DatasourceService{
		db:      captured,
		manager: nil,
	}

	_, _, _ = svc.GetQueryHistory(context.Background(), 1, 1, 20, 0, "", "", "", 0)

	if len(captured.queries) < 2 {
		t.Fatalf("expected at least 2 queries (count + data), got %d", len(captured.queries))
	}

	for i, q := range captured.queries {
		upper := strings.ToUpper(q)
		if !strings.Contains(upper, "JOIN BDOPSFLOW_USERS") {
			continue
		}
		whereIdx := strings.Index(upper, "WHERE")
		if whereIdx == -1 {
			continue
		}
		wherePart := upper[whereIdx:]

		// 防御: 任何未来添加 bdopsflow_users 也有的列时
		ambiguousPatterns := []string{
			" CREATED_BY ", " (CREATED_BY",
			" UPDATED_BY ", " (UPDATED_BY",
		}
		for _, pat := range ambiguousPatterns {
			if strings.Contains(wherePart, pat) {
				t.Errorf("query[%d] has ambiguous column in WHERE clause: %s\nfull SQL: %s", i, pat, q)
			}
		}
	}
}
