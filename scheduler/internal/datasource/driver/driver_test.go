package driver

import (
	"context"
	"testing"
)

func TestRegisterDriver(t *testing.T) {
	expectedTypes := []string{
		"mysql", "sqlite", "hive", "kyuubi", "spark",
		"trino", "starrocks", "doris", "rqlite",
	}

	for _, dsType := range expectedTypes {
		if !IsSupported(dsType) {
			t.Errorf("driver type %s should be supported", dsType)
		}
	}
}

func TestGetDriver(t *testing.T) {
	tests := []struct {
		name    string
		dsType  string
		wantErr bool
	}{
		{"hive driver", "hive", false},
		{"kyuubi driver", "kyuubi", false},
		{"spark driver", "spark", false},
		{"trino driver", "trino", false},
		{"starrocks driver", "starrocks", false},
		{"doris driver", "doris", false},
		{"rqlite driver", "rqlite", false},
		{"mysql driver", "mysql", false},
		{"sqlite driver", "sqlite", false},
		{"unsupported driver", "unsupported", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := GetDriver(tt.dsType)
			if tt.wantErr {
				if err == nil {
					t.Errorf("GetDriver(%s) expected error, got nil", tt.dsType)
				}
				return
			}
			if err != nil {
				t.Errorf("GetDriver(%s) unexpected error: %v", tt.dsType, err)
				return
			}
			if d == nil {
				t.Errorf("GetDriver(%s) returned nil driver", tt.dsType)
			}
		})
	}
}

func TestHiveDriverImplementsInterface(t *testing.T) {
	var _ Driver = &HiveDriver{}
}

func TestKyuubiDriverImplementsInterface(t *testing.T) {
	var _ Driver = &KyuubiDriver{}
}

func TestSparkDriverImplementsInterface(t *testing.T) {
	var _ Driver = &SparkDriver{}
}

func TestTrinoDriverImplementsInterface(t *testing.T) {
	var _ Driver = &TrinoDriver{}
}

func TestStarRocksDriverImplementsInterface(t *testing.T) {
	var _ Driver = &StarRocksDriver{}
}

func TestDorisDriverImplementsInterface(t *testing.T) {
	var _ Driver = &DorisDriver{}
}

func TestRqliteDriverImplementsInterface(t *testing.T) {
	var _ Driver = &RqliteDriver{}
}

func TestHiveDriverNew(t *testing.T) {
	d := NewHiveDriver()
	if d == nil {
		t.Fatal("NewHiveDriver returned nil")
	}
}

func TestKyuubiDriverNew(t *testing.T) {
	d := NewKyuubiDriver()
	if d == nil {
		t.Fatal("NewKyuubiDriver returned nil")
	}
}

func TestSparkDriverNew(t *testing.T) {
	d := NewSparkDriver()
	if d == nil {
		t.Fatal("NewSparkDriver returned nil")
	}
}

func TestTrinoDriverNew(t *testing.T) {
	d := NewTrinoDriver()
	if d == nil {
		t.Fatal("NewTrinoDriver returned nil")
	}
}

func TestStarRocksDriverNew(t *testing.T) {
	d := NewStarRocksDriver()
	if d == nil {
		t.Fatal("NewStarRocksDriver returned nil")
	}
}

func TestDorisDriverNew(t *testing.T) {
	d := NewDorisDriver()
	if d == nil {
		t.Fatal("NewDorisDriver returned nil")
	}
}

func TestRqliteDriverNew(t *testing.T) {
	d := NewRqliteDriver()
	if d == nil {
		t.Fatal("NewRqliteDriver returned nil")
	}
}

func TestHiveDriverCloseWithoutConnect(t *testing.T) {
	d := NewHiveDriver()
	if err := d.Close(); err != nil {
		t.Errorf("HiveDriver.Close() on unconnected driver should not error, got: %v", err)
	}
}

func TestKyuubiDriverCloseWithoutConnect(t *testing.T) {
	d := NewKyuubiDriver()
	if err := d.Close(); err != nil {
		t.Errorf("KyuubiDriver.Close() on unconnected driver should not error, got: %v", err)
	}
}

func TestSparkDriverCloseWithoutConnect(t *testing.T) {
	d := NewSparkDriver()
	if err := d.Close(); err != nil {
		t.Errorf("SparkDriver.Close() on unconnected driver should not error, got: %v", err)
	}
}

func TestTrinoDriverCloseWithoutConnect(t *testing.T) {
	d := NewTrinoDriver()
	if err := d.Close(); err != nil {
		t.Errorf("TrinoDriver.Close() on unconnected driver should not error, got: %v", err)
	}
}

func TestStarRocksDriverCloseWithoutConnect(t *testing.T) {
	d := NewStarRocksDriver()
	if err := d.Close(); err != nil {
		t.Errorf("StarRocksDriver.Close() on unconnected driver should not error, got: %v", err)
	}
}

func TestDorisDriverCloseWithoutConnect(t *testing.T) {
	d := NewDorisDriver()
	if err := d.Close(); err != nil {
		t.Errorf("DorisDriver.Close() on unconnected driver should not error, got: %v", err)
	}
}

func TestRqliteDriverCloseWithoutConnect(t *testing.T) {
	d := NewRqliteDriver()
	if err := d.Close(); err != nil {
		t.Errorf("RqliteDriver.Close() on unconnected driver should not error, got: %v", err)
	}
}

func TestHiveDriverTestConnectionWithoutConnect(t *testing.T) {
	d := NewHiveDriver()
	err := d.TestConnection(context.Background())
	if err == nil {
		t.Error("HiveDriver.TestConnection() on unconnected driver should return error")
	}
}

func TestKyuubiDriverTestConnectionWithoutConnect(t *testing.T) {
	d := NewKyuubiDriver()
	err := d.TestConnection(context.Background())
	if err == nil {
		t.Error("KyuubiDriver.TestConnection() on unconnected driver should return error")
	}
}

func TestSparkDriverTestConnectionWithoutConnect(t *testing.T) {
	d := NewSparkDriver()
	err := d.TestConnection(context.Background())
	if err == nil {
		t.Error("SparkDriver.TestConnection() on unconnected driver should return error")
	}
}

func TestTrinoDriverTestConnectionWithoutConnect(t *testing.T) {
	d := NewTrinoDriver()
	err := d.TestConnection(context.Background())
	if err == nil {
		t.Error("TrinoDriver.TestConnection() on unconnected driver should return error")
	}
}

func TestStarRocksDriverTestConnectionWithoutConnect(t *testing.T) {
	d := NewStarRocksDriver()
	err := d.TestConnection(context.Background())
	if err == nil {
		t.Error("StarRocksDriver.TestConnection() on unconnected driver should return error")
	}
}

func TestDorisDriverTestConnectionWithoutConnect(t *testing.T) {
	d := NewDorisDriver()
	err := d.TestConnection(context.Background())
	if err == nil {
		t.Error("DorisDriver.TestConnection() on unconnected driver should return error")
	}
}

func TestRqliteDriverTestConnectionWithoutConnect(t *testing.T) {
	d := NewRqliteDriver()
	err := d.TestConnection(context.Background())
	if err == nil {
		t.Error("RqliteDriver.TestConnection() on unconnected driver should return error")
	}
}

func TestHiveDriverQueryWithoutConnect(t *testing.T) {
	d := NewHiveDriver()
	_, err := d.Query(context.Background(), "SELECT 1")
	if err == nil {
		t.Error("HiveDriver.Query() on unconnected driver should return error")
	}
}

func TestKyuubiDriverQueryWithoutConnect(t *testing.T) {
	d := NewKyuubiDriver()
	_, err := d.Query(context.Background(), "SELECT 1")
	if err == nil {
		t.Error("KyuubiDriver.Query() on unconnected driver should return error")
	}
}

func TestSparkDriverQueryWithoutConnect(t *testing.T) {
	d := NewSparkDriver()
	_, err := d.Query(context.Background(), "SELECT 1")
	if err == nil {
		t.Error("SparkDriver.Query() on unconnected driver should return error")
	}
}

func TestTrinoDriverQueryWithoutConnect(t *testing.T) {
	d := NewTrinoDriver()
	_, err := d.Query(context.Background(), "SELECT 1")
	if err == nil {
		t.Error("TrinoDriver.Query() on unconnected driver should return error")
	}
}

func TestStarRocksDriverQueryWithoutConnect(t *testing.T) {
	d := NewStarRocksDriver()
	_, err := d.Query(context.Background(), "SELECT 1")
	if err == nil {
		t.Error("StarRocksDriver.Query() on unconnected driver should return error")
	}
}

func TestDorisDriverQueryWithoutConnect(t *testing.T) {
	d := NewDorisDriver()
	_, err := d.Query(context.Background(), "SELECT 1")
	if err == nil {
		t.Error("DorisDriver.Query() on unconnected driver should return error")
	}
}

func TestRqliteDriverQueryWithoutConnect(t *testing.T) {
	d := NewRqliteDriver()
	_, err := d.Query(context.Background(), "SELECT 1")
	if err == nil {
		t.Error("RqliteDriver.Query() on unconnected driver should return error")
	}
}

func TestHiveDriverSupportsCancel(t *testing.T) {
	d := NewHiveDriver()
	if !d.SupportsCancel() {
		t.Error("HiveDriver.SupportsCancel() should return true")
	}
}

func TestKyuubiDriverSupportsCancel(t *testing.T) {
	d := NewKyuubiDriver()
	if !d.SupportsCancel() {
		t.Error("KyuubiDriver.SupportsCancel() should return true")
	}
}

func TestSparkDriverSupportsCancel(t *testing.T) {
	d := NewSparkDriver()
	if !d.SupportsCancel() {
		t.Error("SparkDriver.SupportsCancel() should return true")
	}
}

func TestTrinoDriverSupportsCancel(t *testing.T) {
	d := NewTrinoDriver()
	if !d.SupportsCancel() {
		t.Error("TrinoDriver.SupportsCancel() should return true")
	}
}

func TestStarRocksDriverSupportsCancel(t *testing.T) {
	d := NewStarRocksDriver()
	if !d.SupportsCancel() {
		t.Error("StarRocksDriver.SupportsCancel() should return true")
	}
}

func TestDorisDriverSupportsCancel(t *testing.T) {
	d := NewDorisDriver()
	if !d.SupportsCancel() {
		t.Error("DorisDriver.SupportsCancel() should return true")
	}
}

func TestRqliteDriverSupportsCancel(t *testing.T) {
	d := NewRqliteDriver()
	if d.SupportsCancel() {
		t.Error("RqliteDriver.SupportsCancel() should return false")
	}
}

func TestHiveDriverGetDatabasesWithoutConnect(t *testing.T) {
	d := NewHiveDriver()
	_, err := d.GetDatabases(context.Background())
	if err == nil {
		t.Error("HiveDriver.GetDatabases() on unconnected driver should return error")
	}
}

func TestHiveDriverGetTablesWithoutConnect(t *testing.T) {
	d := NewHiveDriver()
	_, err := d.GetTables(context.Background(), "test")
	if err == nil {
		t.Error("HiveDriver.GetTables() on unconnected driver should return error")
	}
}

func TestHiveDriverGetColumnsWithoutConnect(t *testing.T) {
	d := NewHiveDriver()
	_, err := d.GetColumns(context.Background(), "test", "table")
	if err == nil {
		t.Error("HiveDriver.GetColumns() on unconnected driver should return error")
	}
}

func TestTrinoDriverBuildDSN(t *testing.T) {
	d := &TrinoDriver{config: DatasourceConfig{
		Host:     "localhost",
		Port:     8080,
		Username: "test",
		Database: "hive.default",
	}}

	dsn := d.buildDSN(8080)
	if dsn == "" {
		t.Error("TrinoDriver.buildDSN() returned empty string")
	}
}

func TestStarRocksDriverBuildDSN(t *testing.T) {
	d := &StarRocksDriver{config: DatasourceConfig{
		Host:     "localhost",
		Port:     9030,
		Username: "root",
		Password: "",
		Database: "test_db",
	}}

	dsn := d.buildDSN()
	if dsn == "" {
		t.Error("StarRocksDriver.buildDSN() returned empty string")
	}
}

func TestDorisDriverBuildDSN(t *testing.T) {
	d := &DorisDriver{config: DatasourceConfig{
		Host:     "localhost",
		Port:     9030,
		Username: "root",
		Password: "",
		Database: "test_db",
	}}

	dsn := d.buildDSN()
	if dsn == "" {
		t.Error("DorisDriver.buildDSN() returned empty string")
	}
}

func TestStarRocksDriverDefaultPort(t *testing.T) {
	d := &StarRocksDriver{config: DatasourceConfig{
		Host:     "localhost",
		Username: "root",
		Password: "",
		Database: "test_db",
	}}

	dsn := d.buildDSN()
	if dsn == "" {
		t.Error("StarRocksDriver.buildDSN() returned empty string")
	}
}

func TestDorisDriverDefaultPort(t *testing.T) {
	d := &DorisDriver{config: DatasourceConfig{
		Host:     "localhost",
		Username: "root",
		Password: "",
		Database: "test_db",
	}}

	dsn := d.buildDSN()
	if dsn == "" {
		t.Error("DorisDriver.buildDSN() returned empty string")
	}
}

func TestEscapeHiveIdentifier(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal", "normal"},
		{"with`backtick", "with``backtick"},
	}

	for _, tt := range tests {
		result := escapeHiveIdentifier(tt.input)
		if result != tt.expected {
			t.Errorf("escapeHiveIdentifier(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestEscapeTrinoIdentifier(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal", "normal"},
		{`with"quote`, `with""quote`},
	}

	for _, tt := range tests {
		result := escapeTrinoIdentifier(tt.input)
		if result != tt.expected {
			t.Errorf("escapeTrinoIdentifier(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestEscapeRqliteIdentifier(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal", "normal"},
		{`with"quote`, `with""quote`},
	}

	for _, tt := range tests {
		result := escapeRqliteIdentifier(tt.input)
		if result != tt.expected {
			t.Errorf("escapeRqliteIdentifier(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestConvertTrinoValue(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected interface{}
	}{
		{nil, nil},
		{[]byte("hello"), "hello"},
		{"string", "string"},
		{42, 42},
	}

	for _, tt := range tests {
		result := convertTrinoValue(tt.input)
		if result != tt.expected {
			t.Errorf("convertTrinoValue(%v) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestSupportedTypes(t *testing.T) {
	types := SupportedTypes()
	expectedCount := 9
	if len(types) != expectedCount {
		t.Errorf("SupportedTypes() returned %d types, want %d", len(types), expectedCount)
	}
}

func TestNormalizeSQL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"SELECT 1", "SELECT 1"},
		{"SELECT 1;", "SELECT 1"},
		{"SELECT 1 ; ", "SELECT 1"},
		{"  SELECT 1;  ", "SELECT 1"},
		{"SELECT 1;;", "SELECT 1;"},
		{"", ""},
		{";", ""},
		{"  ;  ", ""},
	}

	for _, tt := range tests {
		result := normalizeSQL(tt.input)
		if result != tt.expected {
			t.Errorf("normalizeSQL(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestTruncateSQL(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly10", 10, "exactly10"},
		{"this is a long sql statement", 10, "this is a ..."},
		{"", 5, ""},
	}

	for _, tt := range tests {
		result := truncateSQL(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncateSQL(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

func TestHiveDriverUseDatabaseWithoutConnect(t *testing.T) {
	d := NewHiveDriver()
	err := d.UseDatabase(context.Background(), "test_db")
	if err == nil {
		t.Error("HiveDriver.UseDatabase() on unconnected driver should return error")
	}
}

func TestMySQLDriverUseDatabaseWithoutConnect(t *testing.T) {
	d := NewMySQLDriver()
	err := d.UseDatabase(context.Background(), "test_db")
	if err == nil {
		t.Error("MySQLDriver.UseDatabase() on unconnected driver should return error")
	}
}

func TestHiveDriverUseDatabaseEmpty(t *testing.T) {
	d := &HiveDriver{}
	err := d.UseDatabase(context.Background(), "")
	if err != nil {
		t.Errorf("HiveDriver.UseDatabase() with empty database should return nil, got: %v", err)
	}
}

func TestMySQLDriverUseDatabaseEmpty(t *testing.T) {
	d := &MySQLDriver{}
	err := d.UseDatabase(context.Background(), "")
	if err != nil {
		t.Errorf("MySQLDriver.UseDatabase() with empty database should return nil, got: %v", err)
	}
}

func TestSQLiteDriverUseDatabase(t *testing.T) {
	d := NewSQLiteDriver()
	err := d.UseDatabase(context.Background(), "test_db")
	if err != nil {
		t.Errorf("SQLiteDriver.UseDatabase() should always return nil, got: %v", err)
	}
}

func TestEscapeMySQLIdentifier(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal", "normal"},
		{"with`backtick", "with``backtick"},
	}

	for _, tt := range tests {
		result := escapeMySQLIdentifier(tt.input)
		if result != tt.expected {
			t.Errorf("escapeMySQLIdentifier(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestTrinoDriverBuildDSNWithSSL(t *testing.T) {
	d := &TrinoDriver{config: DatasourceConfig{
		Host:     "secure-host",
		Port:     443,
		Username: "admin",
		Password: "secret",
		Database: "catalog1",
		Config:   map[string]interface{}{"ssl": true},
	}}

	dsn := d.buildDSN(443)
	if dsn == "" {
		t.Error("TrinoDriver.buildDSN() with SSL returned empty string")
	}
}

func TestTrinoDriverBuildDSNWithLDAP(t *testing.T) {
	d := &TrinoDriver{config: DatasourceConfig{
		Host:     "ldap-host",
		Port:     8080,
		Username: "user",
		Password: "pass",
		AuthType: "ldap",
		Database: "catalog1.schema1",
	}}

	dsn := d.buildDSN(8080)
	if dsn == "" {
		t.Error("TrinoDriver.buildDSN() with LDAP returned empty string")
	}
}

func TestStarRocksDriverBuildDSNWithSSL(t *testing.T) {
	d := &StarRocksDriver{config: DatasourceConfig{
		Host:     "secure-host",
		Port:     9030,
		Username: "root",
		Password: "pass",
		Database: "test_db",
		Config:   map[string]interface{}{"ssl": true},
	}}

	dsn := d.buildDSN()
	if dsn == "" {
		t.Error("StarRocksDriver.buildDSN() with SSL returned empty string")
	}
}

func TestExtractLastStatement(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"SELECT 1", "SELECT 1"},
		{"SELECT 1; SELECT 2", "SELECT 2"},
		{"SELECT 1; SELECT 2;", "SELECT 2"},
		{"SELECT 1; SELECT 2; ", "SELECT 2"},
		{"  SELECT 1;  SELECT 2;  ", "SELECT 2"},
		{"SELECT 1;; SELECT 2;;", "SELECT 2"},
		{";SELECT 1;", "SELECT 1"},
		{";;", ""},
		{"", ""},
		{"SELECT * FROM t WHERE id = 1; SELECT name FROM t2", "SELECT name FROM t2"},
	}

	for _, tt := range tests {
		result := ExtractLastStatement(tt.input)
		if result != tt.expected {
			t.Errorf("ExtractLastStatement(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestNormalizeSQLForType(t *testing.T) {
	tests := []struct {
		sql      string
		dsType   string
		expected string
	}{
		{"SELECT 1;", "mysql", "SELECT 1"},
		{"SELECT 1; SELECT 2;", "hive", "SELECT 2"},
		{"SELECT 1; SELECT 2;", "kyuubi", "SELECT 2"},
		{"SELECT 1; SELECT 2;", "spark", "SELECT 2"},
		{"SELECT 1; SELECT 2;", "trino", "SELECT 1; SELECT 2"},
		{"SELECT 1;", "hive", "SELECT 1"},
		{"SELECT 1", "hive", "SELECT 1"},
		{"  SELECT 1;  ", "mysql", "SELECT 1"},
	}

	for _, tt := range tests {
		result := NormalizeSQLForType(tt.sql, tt.dsType)
		if result != tt.expected {
			t.Errorf("NormalizeSQLForType(%q, %q) = %q, want %q", tt.sql, tt.dsType, result, tt.expected)
		}
	}
}
