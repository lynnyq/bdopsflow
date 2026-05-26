package service

import (
	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
	rqlite "github.com/rqlite/gorqlite"
)

type MockDB struct{}

func (m *MockDB) QueryOne(query string) (rqlite.QueryResult, error) {
	return rqlite.QueryResult{}, nil
}

func (m *MockDB) QueryOneParameterized(stmt rqlite.ParameterizedStatement) (rqlite.QueryResult, error) {
	return rqlite.QueryResult{}, nil
}

func (m *MockDB) WriteOneParameterized(stmt rqlite.ParameterizedStatement) (rqlite.WriteResult, error) {
	return rqlite.WriteResult{}, nil
}

func (m *MockDB) WriteParameterized(sqlStatements []rqlite.ParameterizedStatement) ([]rqlite.WriteResult, error) {
	results := make([]rqlite.WriteResult, len(sqlStatements))
	return results, nil
}

var _ database.DB = (*MockDB)(nil)
