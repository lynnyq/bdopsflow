package service

import (
	"errors"

	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
	rqlite "github.com/rqlite/gorqlite"
)

// MockDB 可配置的数据库 mock，用于单元测试。
// 通过设置 QueryResult/QueryError/WriteResult/WriteError 字段来控制返回值。
//
// 用法：
//
//	db := &MockDB{
//	    QueryResult: database.NewQueryResultWithRows([][]interface{}{
//	        {int64(1), "alice"},
//	    }),
//	}
type MockDB struct {
	// 查询返回的结果
	QueryResult rqlite.QueryResult
	// QueryResults 按调用顺序依次返回，非空时优先于 QueryResult
	// 用于多次查询需要返回不同结果的场景
	QueryResults []rqlite.QueryResult
	// 查询返回的错误（非 nil 时优先返回）
	QueryError error
	// 写入返回的结果
	WriteResult rqlite.WriteResult
	// 写入返回的错误（非 nil 时优先返回）
	WriteError error
	// 批量写入返回的结果
	BatchWriteResult []rqlite.WriteResult
	// 批量写入返回的错误
	BatchWriteError error
	// 记录最后一次查询的语句（用于断言）
	LastQueryStmt rqlite.ParameterizedStatement
	// 记录最后一次写入的语句
	LastWriteStmt rqlite.ParameterizedStatement
	// 记录所有查询调用
	QueryStmts []rqlite.ParameterizedStatement
	// 记录所有写入调用
	WriteStmts []rqlite.ParameterizedStatement
}

// nextQueryResult 按 FIFO 顺序取出 QueryResults 中的下一个结果
// 当 QueryResults 耗尽或未设置时，退回到 QueryResult
func (m *MockDB) nextQueryResult() rqlite.QueryResult {
	if len(m.QueryResults) > 0 {
		qr := m.QueryResults[0]
		m.QueryResults = m.QueryResults[1:]
		return qr
	}
	return m.QueryResult
}

func (m *MockDB) QueryOne(query string) (rqlite.QueryResult, error) {
	m.QueryStmts = append(m.QueryStmts, rqlite.ParameterizedStatement{Query: query})
	if m.QueryError != nil {
		return rqlite.QueryResult{}, m.QueryError
	}
	return m.nextQueryResult(), nil
}

func (m *MockDB) QueryOneParameterized(stmt rqlite.ParameterizedStatement) (rqlite.QueryResult, error) {
	m.LastQueryStmt = stmt
	m.QueryStmts = append(m.QueryStmts, stmt)
	if m.QueryError != nil {
		return rqlite.QueryResult{}, m.QueryError
	}
	return m.nextQueryResult(), nil
}

func (m *MockDB) WriteOneParameterized(stmt rqlite.ParameterizedStatement) (rqlite.WriteResult, error) {
	m.LastWriteStmt = stmt
	m.WriteStmts = append(m.WriteStmts, stmt)
	if m.WriteError != nil {
		return rqlite.WriteResult{}, m.WriteError
	}
	return m.WriteResult, nil
}

func (m *MockDB) WriteParameterized(stmts []rqlite.ParameterizedStatement) ([]rqlite.WriteResult, error) {
	for _, s := range stmts {
		m.WriteStmts = append(m.WriteStmts, s)
	}
	if m.BatchWriteError != nil {
		return nil, m.BatchWriteError
	}
	if m.BatchWriteResult != nil {
		return m.BatchWriteResult, nil
	}
	results := make([]rqlite.WriteResult, len(stmts))
	return results, nil
}

var _ database.DB = (*MockDB)(nil)

// ErrMockDB 测试用错误
var ErrMockDB = errors.New("mock db error")
