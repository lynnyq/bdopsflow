package service

import (
	rqlite "github.com/rqlite/gorqlite"
)

type MockDB struct{}

func (m *MockDB) QueryOne(query string) (rqlite.QueryResult, error) {
	return rqlite.QueryResult{}, nil
}

func (m *MockDB) QueryOneParameterized(stmt rqlite.ParameterizedStatement) (rqlite.QueryResult, error) {
	return rqlite.QueryResult{}, nil
}

func (m *MockDB) WriteOne(query string) (rqlite.WriteResult, error) {
	return rqlite.WriteResult{}, nil
}

func (m *MockDB) WriteOneParameterized(stmt rqlite.ParameterizedStatement) (rqlite.WriteResult, error) {
	return rqlite.WriteResult{}, nil
}

func (m *MockDB) Execute(query string) error {
	return nil
}

func (m *MockDB) Close() error {
	return nil
}
