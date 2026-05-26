package database

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	rqlite "github.com/rqlite/gorqlite"
)

type DB interface {
	QueryOne(sqlStatement string) (rqlite.QueryResult, error)
	QueryOneParameterized(statement rqlite.ParameterizedStatement) (rqlite.QueryResult, error)
	WriteOneParameterized(statement rqlite.ParameterizedStatement) (rqlite.WriteResult, error)
	WriteParameterized(sqlStatements []rqlite.ParameterizedStatement) ([]rqlite.WriteResult, error)
}

type LogDB struct {
	conn *rqlite.Connection
}

func NewLogDB(conn *rqlite.Connection) *LogDB {
	return &LogDB{conn: conn}
}

func (d *LogDB) QueryOne(sqlStatement string) (rqlite.QueryResult, error) {
	start := time.Now()
	slog.Debug("[SQL QUERY]", "sql", strings.TrimSpace(sqlStatement))
	qr, err := d.conn.QueryOne(sqlStatement)
	elapsed := time.Since(start)
	if err != nil {
		slog.Debug("[SQL QUERY ERROR]", "sql", strings.TrimSpace(sqlStatement), "error", err, "elapsed", elapsed)
	} else {
		slog.Debug("[SQL QUERY DONE]", "sql", strings.TrimSpace(sqlStatement), "elapsed", elapsed)
	}
	return qr, err
}

func (d *LogDB) QueryOneParameterized(statement rqlite.ParameterizedStatement) (rqlite.QueryResult, error) {
	start := time.Now()
	slog.Debug("[SQL QUERY]", "sql", strings.TrimSpace(statement.Query), "args", fmt.Sprintf("%v", statement.Arguments))
	qr, err := d.conn.QueryOneParameterized(statement)
	elapsed := time.Since(start)
	if err != nil {
		slog.Debug("[SQL QUERY ERROR]", "sql", strings.TrimSpace(statement.Query), "args", fmt.Sprintf("%v", statement.Arguments), "error", err, "elapsed", elapsed)
	} else {
		slog.Debug("[SQL QUERY DONE]", "sql", strings.TrimSpace(statement.Query), "args", fmt.Sprintf("%v", statement.Arguments), "elapsed", elapsed)
	}
	return qr, err
}

func (d *LogDB) WriteOneParameterized(statement rqlite.ParameterizedStatement) (rqlite.WriteResult, error) {
	start := time.Now()
	slog.Debug("[SQL WRITE]", "sql", strings.TrimSpace(statement.Query), "args", fmt.Sprintf("%v", statement.Arguments))
	wr, err := d.conn.WriteOneParameterized(statement)
	elapsed := time.Since(start)
	if err != nil {
		slog.Debug("[SQL WRITE ERROR]", "sql", strings.TrimSpace(statement.Query), "args", fmt.Sprintf("%v", statement.Arguments), "error", err, "elapsed", elapsed)
	} else {
		slog.Debug("[SQL WRITE DONE]", "sql", strings.TrimSpace(statement.Query), "args", fmt.Sprintf("%v", statement.Arguments), "elapsed", elapsed)
	}
	return wr, err
}

func (d *LogDB) WriteParameterized(sqlStatements []rqlite.ParameterizedStatement) ([]rqlite.WriteResult, error) {
	start := time.Now()
	for i, stmt := range sqlStatements {
		slog.Debug("[SQL WRITE BATCH]", "index", i, "sql", strings.TrimSpace(stmt.Query), "args", fmt.Sprintf("%v", stmt.Arguments))
	}
	results, err := d.conn.WriteParameterized(sqlStatements)
	elapsed := time.Since(start)
	if err != nil {
		slog.Debug("[SQL WRITE BATCH ERROR]", "count", len(sqlStatements), "error", err, "elapsed", elapsed)
	} else {
		slog.Debug("[SQL WRITE BATCH DONE]", "count", len(sqlStatements), "elapsed", elapsed)
	}
	return results, err
}
