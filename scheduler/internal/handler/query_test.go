package handler

import "testing"

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
