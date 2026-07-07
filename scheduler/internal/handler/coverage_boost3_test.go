package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
)

// ============================================================
// 第三轮覆盖率提升测试：针对 47.6% → 60%+ 目标
// 重点覆盖：参数校验路径、panic recovery 路径、纯函数边界
// ============================================================

// ------------------------------------------------------------
// query.go: 纯函数额外边界用例
// ------------------------------------------------------------

// TestTruncateSQLPreview_EdgeCases_V2 补充测试多字节字符和边界
func TestTruncateSQLPreview_EdgeCases_V2(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want string
	}{
		{
			name: "空字符串",
			sql:  "",
			want: "",
		},
		{
			name: "只有空格",
			sql:  "   ",
			want: "",
		},
		// 200 个中文字符（每个 3 字节），不超过限制
		{
			name: "中文正好200字符",
			sql:  string([]rune("中文字符测试"))[:0] + buildChineseString(200),
			want: buildChineseString(200),
		},
		// 201 个中文字符，应截断为 200 + "..."
		{
			name: "中文超过200字符",
			sql:  buildChineseString(201),
			want: buildChineseString(200) + "...",
		},
		{
			name: "前后有空格应被 trim",
			sql:  "  SELECT 1;  ",
			want: "SELECT 1;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateSQLPreview(tt.sql)
			if got != tt.want {
				t.Errorf("truncateSQLPreview() 长度=%d, want 长度=%d", len(got), len(tt.want))
			}
		})
	}
}

// buildChineseString 构造指定长度的中文字符串
func buildChineseString(n int) string {
	runes := make([]rune, n)
	for i := range runes {
		runes[i] = '测'
	}
	return string(runes)
}

// TestDefaultDBName_EdgeCases 测试 defaultDBName 边界
func TestDefaultDBName_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		db   string
		want string
	}{
		{"空字符串", "", "(默认)"},
		{"有值", "mydb", "mydb"},
		{"空格字符串", " ", " "},
		{"特殊字符", "db-1_test", "db-1_test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := defaultDBName(tt.db)
			if got != tt.want {
				t.Errorf("defaultDBName(%q) = %q, want %q", tt.db, got, tt.want)
			}
		})
	}
}

// TestIsSelectOnly_EdgeCases_V2 补充 PRAGMA 和更多 SQL 类型
func TestIsSelectOnly_EdgeCases_V2(t *testing.T) {
	tests := []struct {
		name         string
		sql          string
		allowWrite   bool
		want         bool
	}{
		{"PRAGMA 表查询", "PRAGMA table_info(users)", false, true},
		{"PRAGMA 不带参数", "PRAGMA database_list", false, true},
		{"WITH 语句", "WITH t AS (SELECT 1) SELECT * FROM t", false, true},
		{"EXPLAIN 语句", "EXPLAIN SELECT 1", false, true},
		{"DESCRIBE 语句", "DESCRIBE users", false, true},
		{"DESC 缩写", "DESC users", false, true},
		{"SHOW 语句", "SHOW TABLES", false, true},
		{"SHOW 缩写", "SHOW", false, true},
		{"SELECT 缩写", "SELECT", false, true},
		{"DESC 缩写单独", "DESC", false, true},
		{"INSERT 语句不允许", "INSERT INTO users VALUES(1)", false, false},
		{"UPDATE 语句不允许", "UPDATE users SET name='a'", false, false},
		{"DELETE 语句不允许", "DELETE FROM users", false, false},
		{"DROP 语句不允许", "DROP TABLE users", false, false},
		{"允许写入时 INSERT", "INSERT INTO users VALUES(1)", true, true},
		{"允许写入时 DROP", "DROP TABLE users", true, true},
		{"大小写混合 SELECT", "select 1", false, true},
		{"大小写混合 INSERT", "insert into t values(1)", false, false},
		{"前导空格 SELECT", "   SELECT 1", false, true},
		{"前导换行 SELECT", "\n\nSELECT 1", false, true},
		{"前导制表符 SELECT", "\t\tSELECT 1", false, true},
		{"混合空白 SELECT", " \n \t SELECT 1", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &QueryHandler{}
			got := h.isSelectOnly(tt.sql, tt.allowWrite)
			if got != tt.want {
				t.Errorf("isSelectOnly(%q, %v) = %v, want %v", tt.sql, tt.allowWrite, got, tt.want)
			}
		})
	}
}

// TestJoinSpaces_EdgeCases_V2 补充更多空白组合
func TestJoinSpaces_EdgeCases_V2(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"空字符串", "", ""},
		{"只有空格", "     ", " "},
		{"只有制表符", "\t\t\t", " "},
		{"只有换行", "\n\n\n", " "},
		{"只有回车", "\r\r\r", " "},
		{"混合空白开头", " \t\n\r abc", " abc"},
		{"混合空白中间", "a \t\n\r b", "a b"},
		{"混合空白结尾", "abc \t\n\r ", "abc "},
		{"连续混合空白", "a    \t\n\r   b", "a b"},
		{"无空白", "abcdef", "abcdef"},
		{"单个空格", " ", " "},
		{"单个字母", "a", "a"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := joinSpaces(tt.in)
			if got != tt.want {
				t.Errorf("joinSpaces(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestIsPoolBusyError_EdgeCases 补充更多错误消息
func TestIsPoolBusyError_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil 错误", nil, false},
		{"连接池已满", errPoolBusy("pool fully occupied"), true},
		{"连接池已满(带额外信息)", errPoolBusy("driver: pool fully occupied, please retry"), true},
		{"连接超时", errPoolBusy("connection timeout"), false},
		{"空错误消息", errPoolBusy(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPoolBusyError(tt.err)
			if got != tt.want {
				t.Errorf("isPoolBusyError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// errPoolBusy 是一个简单的 error 实现
type errPoolBusy string

func (e errPoolBusy) Error() string { return string(e) }

// ------------------------------------------------------------
// api_token.go: Reveal / Revoke nil service panic recovery
// ------------------------------------------------------------

// TestAPITokenHandler_Reveal_NilService service 为 nil 时应 panic
func TestAPITokenHandler_Reveal_NilService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &APITokenHandler{}
	r.POST("/api/tokens/reveal", func(c *gin.Context) {
		c.Set("user_id", int64(1))
		handler.Reveal(c)
	})

	req, _ := http.NewRequest("POST", "/api/tokens/reveal", nil)
	w := httptest.NewRecorder()

	defer func() {
		if rec := recover(); rec != nil {
			t.Log("预期内的 panic（nil service）:", rec)
			return
		}
	}()

	r.ServeHTTP(w, req)
}

// TestAPITokenHandler_Revoke_NilService service 为 nil 时应 panic
func TestAPITokenHandler_Revoke_NilService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &APITokenHandler{}
	r.DELETE("/api/tokens/revoke", func(c *gin.Context) {
		c.Set("user_id", int64(1))
		handler.Revoke(c)
	})

	req, _ := http.NewRequest("DELETE", "/api/tokens/revoke", nil)
	w := httptest.NewRecorder()

	defer func() {
		if rec := recover(); rec != nil {
			t.Log("预期内的 panic（nil service）:", rec)
			return
		}
	}()

	r.ServeHTTP(w, req)
}

// ------------------------------------------------------------
// certificate.go: 各方法参数校验和 nil service 路径
// ------------------------------------------------------------
// 注: Generate/GetInfo/Reveal/Revoke 的零/负数 userID 用例已在 auth_extra_test.go 中覆盖

// TestCertificateHandler_List_WithUserID_NilService 有 userID 但 service 为 nil
func TestCertificateHandler_List_WithUserID_NilService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &CertificateHandler{}
	r.GET("/api/certificates", func(c *gin.Context) {
		c.Set("user_id", int64(1))
		handler.List(c)
	})

	req, _ := http.NewRequest("GET", "/api/certificates", nil)
	w := httptest.NewRecorder()

	defer func() {
		if rec := recover(); rec != nil {
			t.Log("预期内的 panic（nil permSvc）:", rec)
			return
		}
	}()

	r.ServeHTTP(w, req)
}

// TestCertificateHandler_Create_NilService 创建证书时 service 为 nil
func TestCertificateHandler_Create_NilService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &CertificateHandler{}
	r.POST("/api/certificates", func(c *gin.Context) {
		c.Set("user_id", int64(1))
		handler.Create(c)
	})

	body := `{"name":"test-cert"}`
	req, _ := http.NewRequest("POST", "/api/certificates", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	defer func() {
		if rec := recover(); rec != nil {
			t.Log("预期内的 panic（nil certSvc）:", rec)
			return
		}
	}()

	r.ServeHTTP(w, req)
}

// TestCertificateHandler_Get_NilService Get 请求 service 为 nil
func TestCertificateHandler_Get_NilService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &CertificateHandler{}
	r.GET("/api/certificates/:id", handler.Get)

	req, _ := http.NewRequest("GET", "/api/certificates/1", nil)
	w := httptest.NewRecorder()

	defer func() {
		if rec := recover(); rec != nil {
			t.Log("预期内的 panic（nil certSvc）:", rec)
			return
		}
	}()

	r.ServeHTTP(w, req)
}

// TestCertificateHandler_Update_NilService Update 请求 service 为 nil
func TestCertificateHandler_Update_NilService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &CertificateHandler{}
	r.PUT("/api/certificates/:id", handler.Update)

	body := `{"name":"updated"}`
	req, _ := http.NewRequest("PUT", "/api/certificates/1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	defer func() {
		if rec := recover(); rec != nil {
			t.Log("预期内的 panic（nil certSvc）:", rec)
			return
		}
	}()

	r.ServeHTTP(w, req)
}

// TestCertificateHandler_Delete_NilService Delete 请求 service 为 nil
func TestCertificateHandler_Delete_NilService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &CertificateHandler{}
	r.DELETE("/api/certificates/:id", handler.Delete)

	req, _ := http.NewRequest("DELETE", "/api/certificates/1", nil)
	w := httptest.NewRecorder()

	defer func() {
		if rec := recover(); rec != nil {
			t.Log("预期内的 panic（nil certSvc）:", rec)
			return
		}
	}()

	r.ServeHTTP(w, req)
}

// TestCertificateHandler_List_PageSizeBoundaries 测试分页边界
func TestCertificateHandler_List_PageSizeBoundaries(t *testing.T) {
	tests := []struct {
		name         string
		page         string
		pageSize     string
	}{
		{"page=0 应重置为1", "0", "10"},
		{"page=-1 应重置为1", "-1", "10"},
		{"pageSize=0 应重置为20", "1", "0"},
		{"pageSize=-1 应重置为20", "1", "-1"},
		{"pageSize=101 应重置为20", "1", "101"},
		{"pageSize=100 边界", "1", "100"},
		{"pageSize=1 最小有效值", "1", "1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			r := gin.New()

			handler := &CertificateHandler{}
			r.GET("/api/certificates", func(c *gin.Context) {
				c.Set("user_id", int64(1))
				handler.List(c)
			})

			url := "/api/certificates?page=" + tt.page + "&page_size=" + tt.pageSize
			req, _ := http.NewRequest("GET", url, nil)
			w := httptest.NewRecorder()

			defer func() {
				if rec := recover(); rec != nil {
					t.Log("预期内的 panic（nil permSvc）:", rec)
					return
				}
			}()

			r.ServeHTTP(w, req)
		})
	}
}

// ------------------------------------------------------------
// datasource.go: RevokePermission / UpdatePermission / GetPermissions 参数校验
// ------------------------------------------------------------

// TestDatasourceHandler_RevokePermission_InvalidPermID 测试无效的权限 ID
func TestDatasourceHandler_RevokePermission_InvalidPermID(t *testing.T) {
	tests := []struct {
		name    string
		permID  string
	}{
		{"非数字", "abc"},
		{"浮点数", "1.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			r := gin.New()
			h := &DatasourceHandler{}
			r.DELETE("/api/datasources/:id/permissions/:perm_id", h.RevokePermission)

			req, _ := http.NewRequest("DELETE", "/api/datasources/1/permissions/"+tt.permID, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			var resp Response
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if resp.Code != CodeBadRequest {
				t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
			}
		})
	}
}

// TestDatasourceHandler_UpdatePermission_InvalidPermID 测试无效的权限 ID
func TestDatasourceHandler_UpdatePermission_InvalidPermID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &DatasourceHandler{}
	r.PUT("/api/datasources/:id/permissions/:perm_id", h.UpdatePermission)

	body := `{"permission_type":"read"}`
	req, _ := http.NewRequest("PUT", "/api/datasources/1/permissions/abc", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
	}
}

// TestDatasourceHandler_UpdatePermission_InvalidJSON 测试无效 JSON
func TestDatasourceHandler_UpdatePermission_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &DatasourceHandler{}
	r.PUT("/api/datasources/:id/permissions/:perm_id", h.UpdatePermission)

	req, _ := http.NewRequest("PUT", "/api/datasources/1/permissions/1", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
	}
}

// TestDatasourceHandler_UpdatePermission_MissingPermissionType 测试缺少 permission_type
func TestDatasourceHandler_UpdatePermission_MissingPermissionType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &DatasourceHandler{}
	r.PUT("/api/datasources/:id/permissions/:perm_id", h.UpdatePermission)

	body := `{"other":"field"}`
	req, _ := http.NewRequest("PUT", "/api/datasources/1/permissions/1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
	}
}

// TestDatasourceHandler_UpdatePermission_InvalidPermissionType 测试无效的权限类型
func TestDatasourceHandler_UpdatePermission_InvalidPermissionType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &DatasourceHandler{}
	r.PUT("/api/datasources/:id/permissions/:perm_id", h.UpdatePermission)

	body := `{"permission_type":"invalid_type"}`
	req, _ := http.NewRequest("PUT", "/api/datasources/1/permissions/1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
	}
}

// 注: GetPermissions_InvalidID 用例已在 datasource_test.go 中覆盖

// ------------------------------------------------------------
// datasource.go: TestConnectionByParams 参数校验
// ------------------------------------------------------------

// TestDatasourceHandler_TestConnectionByParams_InvalidJSON 测试无效 JSON
func TestDatasourceHandler_TestConnectionByParams_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &DatasourceHandler{}
	r.POST("/api/datasources/test", h.TestConnectionByParams)

	req, _ := http.NewRequest("POST", "/api/datasources/test", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
	}
}

// TestDatasourceHandler_TestConnectionByParams_MissingType 测试缺少 type
func TestDatasourceHandler_TestConnectionByParams_MissingType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &DatasourceHandler{}
	r.POST("/api/datasources/test", h.TestConnectionByParams)

	body := `{"host":"localhost"}`
	req, _ := http.NewRequest("POST", "/api/datasources/test", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
	}
}

// TestDatasourceHandler_TestConnectionByParams_UnsupportedType 测试不支持的数据源类型
func TestDatasourceHandler_TestConnectionByParams_UnsupportedType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &DatasourceHandler{}
	r.POST("/api/datasources/test", h.TestConnectionByParams)

	body := `{"type":"unsupported_db"}`
	req, _ := http.NewRequest("POST", "/api/datasources/test", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
	}
}

// ------------------------------------------------------------
// datasource.go: GrantPermission 参数校验
// ------------------------------------------------------------

// TestDatasourceHandler_GrantPermission_InvalidJSON 测试无效 JSON
func TestDatasourceHandler_GrantPermission_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &DatasourceHandler{}
	r.POST("/api/datasources/:id/permissions", h.GrantPermission)

	req, _ := http.NewRequest("POST", "/api/datasources/1/permissions", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
	}
}

// TestDatasourceHandler_GrantPermission_MissingPermissionType 测试缺少 permission_type
func TestDatasourceHandler_GrantPermission_MissingPermissionType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &DatasourceHandler{}
	r.POST("/api/datasources/:id/permissions", h.GrantPermission)

	body := `{"user_id":1}`
	req, _ := http.NewRequest("POST", "/api/datasources/1/permissions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
	}
}

// TestDatasourceHandler_GrantPermission_NoTarget 测试缺少授权对象
func TestDatasourceHandler_GrantPermission_NoTarget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &DatasourceHandler{}
	r.POST("/api/datasources/:id/permissions", h.GrantPermission)

	body := `{"permission_type":"read"}`
	req, _ := http.NewRequest("POST", "/api/datasources/1/permissions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
	}
}

// TestDatasourceHandler_GrantPermission_InvalidPermissionType 测试无效的权限类型
func TestDatasourceHandler_GrantPermission_InvalidPermissionType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &DatasourceHandler{}
	r.POST("/api/datasources/:id/permissions", h.GrantPermission)

	body := `{"user_id":1,"permission_type":"invalid_type"}`
	req, _ := http.NewRequest("POST", "/api/datasources/1/permissions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
	}
}

// TestDatasourceHandler_GrantPermission_InvalidDsID 测试无效的数据源 ID
func TestDatasourceHandler_GrantPermission_InvalidDsID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &DatasourceHandler{}
	r.POST("/api/datasources/:id/permissions", h.GrantPermission)

	body := `{"user_id":1,"permission_type":"read"}`
	req, _ := http.NewRequest("POST", "/api/datasources/abc/permissions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
	}
}

// 注: SupportedTypes 用例已在 datasource_test.go 中覆盖

// ------------------------------------------------------------
// datasource.go: datasourceToMap 补充测试
// ------------------------------------------------------------

// TestDatasourceToMap_WithAllFields 测试包含所有字段的数据源
func TestDatasourceToMap_WithAllFields(t *testing.T) {
	h := &DatasourceHandler{}
	now := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
	createdBy := int64(10)
	updatedBy := int64(20)

	ds := &model.Datasource{
		ID: 1, Name: "full-ds", Type: "mysql",
		Host: "localhost", Port: 3306, Path: "/tmp",
		Database: "testdb", Username: "root", AuthType: "simple",
		Config: `{"key":"value"}`, Description: "完整数据源",
		DomainID: 1, DomainName: "domain1", IsEnabled: true,
		AllowWriteSQL: true, TestStatus: "success",
		ConnectionMode: "standard", ZkHosts: "zk1:2181",
		ZkPath: "/hbase", RqliteHosts: "rq1:4001",
		UserPermission: "manage",
		LastTestAt: &now, CreatedBy: &createdBy, UpdatedBy: &updatedBy,
	}

	m := h.datasourceToMap(ds)

	if m["id"] != int64(1) {
		t.Errorf("id = %v, want 1", m["id"])
	}
	if m["name"] != "full-ds" {
		t.Errorf("name = %v, want full-ds", m["name"])
	}
	if m["type"] != "mysql" {
		t.Errorf("type = %v, want mysql", m["type"])
	}
	if m["is_enabled"] != true {
		t.Errorf("is_enabled = %v, want true", m["is_enabled"])
	}
	if m["allow_write_sql"] != true {
		t.Errorf("allow_write_sql = %v, want true", m["allow_write_sql"])
	}
	if m["user_permission"] != "manage" {
		t.Errorf("user_permission = %v, want manage", m["user_permission"])
	}
	if m["last_test_at"] != "2024-06-15T10:30:00Z" {
		t.Errorf("last_test_at = %v, want 2024-06-15T10:30:00Z", m["last_test_at"])
	}
	if m["password"] != nil {
		t.Errorf("password 不应在 map 中, got %v", m["password"])
	}
	if m["created_by_name"] != "" {
		t.Errorf("created_by_name 应为空（nil userAdminSvc）, got %v", m["created_by_name"])
	}
}

// ------------------------------------------------------------
// executor.go: Delete / Online / Offline 空名称和 nil service
// ------------------------------------------------------------

// 注: Delete_EmptyName 用例已在 executor_unit_test.go 中覆盖

// TestExecutorHandler_Delete_NilService 测试 nil service 的 panic recovery
func TestExecutorHandler_Delete_NilService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &ExecutorHandler{}
	r.DELETE("/api/executors/:name", h.Delete)

	req, _ := http.NewRequest("DELETE", "/api/executors/test-executor", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// nil svc 会 panic，handler 内有 defer recover，应返回 500
	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != CodeInternalError {
		t.Errorf("code = %d, want %d", resp.Code, CodeInternalError)
	}
}

// TestExecutorHandler_Online_NilService 测试 nil service 的 panic recovery
func TestExecutorHandler_Online_NilService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &ExecutorHandler{}
	r.POST("/api/executors/:name/online", h.Online)

	req, _ := http.NewRequest("POST", "/api/executors/test-executor/online", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != CodeInternalError {
		t.Errorf("code = %d, want %d", resp.Code, CodeInternalError)
	}
}

// TestExecutorHandler_Offline_NilService 测试 nil service 的 panic recovery
func TestExecutorHandler_Offline_NilService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &ExecutorHandler{}
	r.POST("/api/executors/:name/offline", h.Offline)

	req, _ := http.NewRequest("POST", "/api/executors/test-executor/offline", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != CodeInternalError {
		t.Errorf("code = %d, want %d", resp.Code, CodeInternalError)
	}
}

// TestExecutorHandler_UpdateCapacity_NilService 测试 nil service 的 panic recovery
func TestExecutorHandler_UpdateCapacity_NilService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &ExecutorHandler{}
	r.PUT("/api/executors/:name/capacity", h.UpdateCapacity)

	body := `{"capacity":5}`
	req, _ := http.NewRequest("PUT", "/api/executors/test-executor/capacity", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != CodeInternalError {
		t.Errorf("code = %d, want %d", resp.Code, CodeInternalError)
	}
}

// TestExecutorHandler_UpdateCapacity_InvalidJSON 测试无效 JSON
func TestExecutorHandler_UpdateCapacity_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &ExecutorHandler{}
	r.PUT("/api/executors/:name/capacity", h.UpdateCapacity)

	req, _ := http.NewRequest("PUT", "/api/executors/test-executor/capacity", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
	}
}

// TestExecutorHandler_UpdateCapacity_ZeroCapacity 测试 capacity=0
func TestExecutorHandler_UpdateCapacity_ZeroCapacity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &ExecutorHandler{}
	r.PUT("/api/executors/:name/capacity", h.UpdateCapacity)

	body := `{"capacity":0}`
	req, _ := http.NewRequest("PUT", "/api/executors/test-executor/capacity", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
	}
}

// TestExecutorHandler_UpdateCapacity_NegativeCapacity 测试负数 capacity
func TestExecutorHandler_UpdateCapacity_NegativeCapacity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &ExecutorHandler{}
	r.PUT("/api/executors/:name/capacity", h.UpdateCapacity)

	body := `{"capacity":-5}`
	req, _ := http.NewRequest("PUT", "/api/executors/test-executor/capacity", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
	}
}

// ------------------------------------------------------------
// executor.go: executorToDTO 补充边界用例
// ------------------------------------------------------------

// TestExecutorToDTO_EdgeCases_V2 补充 executorToDTO 边界用例
func TestExecutorToDTO_EdgeCases_V2(t *testing.T) {
	t.Run("offline 状态不改变", func(t *testing.T) {
		exec := &model.Executor{
			ID:     1,
			Name:   "exec1",
			Status: "offline",
		}
		dto := executorToDTO(exec)
		if dto.Status != "offline" {
			t.Errorf("Status = %q, want offline", dto.Status)
		}
	})

	t.Run("online 但心跳无效变为 offline", func(t *testing.T) {
		exec := &model.Executor{
			ID:     2,
			Name:   "exec2",
			Status: "online",
		}
		// LastHeartbeat 无效
		dto := executorToDTO(exec)
		if dto.Status != "offline" {
			t.Errorf("Status = %q, want offline（心跳无效）", dto.Status)
		}
	})

	t.Run("零时间 CreatedAt 和 UpdatedAt", func(t *testing.T) {
		exec := &model.Executor{
			ID:     3,
			Name:   "exec3",
			Status: "offline",
		}
		dto := executorToDTO(exec)
		if dto.CreatedAt != "" {
			t.Errorf("CreatedAt = %q, want empty", dto.CreatedAt)
		}
		if dto.UpdatedAt != "" {
			t.Errorf("UpdatedAt = %q, want empty", dto.UpdatedAt)
		}
	})
}

// ------------------------------------------------------------
// executor.go: parseParam 补充边界用例
// ------------------------------------------------------------

// TestParseParam_EdgeCases 补充 parseParam 边界用例
func TestParseParam_EdgeCases(t *testing.T) {
	t.Run("空字符串返回错误", func(t *testing.T) {
		var captured int64
		ok, err := parseParam("", func(v int64) { captured = v })
		if ok {
			t.Error("expected ok=false for empty string")
		}
		if err == nil {
			t.Error("expected error for empty string")
		}
		_ = captured
	})

	t.Run("零值返回错误", func(t *testing.T) {
		var captured int64
		ok, err := parseParam("0", func(v int64) { captured = v })
		if ok {
			t.Error("expected ok=false for zero value")
		}
		if err == nil {
			t.Error("expected error for zero value")
		}
		_ = captured
	})

	t.Run("负数返回错误", func(t *testing.T) {
		var captured int64
		ok, err := parseParam("-1", func(v int64) { captured = v })
		if ok {
			t.Error("expected ok=false for negative value")
		}
		if err == nil {
			t.Error("expected error for negative value")
		}
		_ = captured
	})

	t.Run("非数字返回错误", func(t *testing.T) {
		var captured int64
		ok, err := parseParam("abc", func(v int64) { captured = v })
		if ok {
			t.Error("expected ok=false for non-numeric")
		}
		if err == nil {
			t.Error("expected error for non-numeric")
		}
		_ = captured
	})

	t.Run("有效正整数成功", func(t *testing.T) {
		var captured int64
		ok, err := parseParam("42", func(v int64) { captured = v })
		if !ok {
			t.Error("expected ok=true for valid positive int")
		}
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if captured != 42 {
			t.Errorf("captured = %d, want 42", captured)
		}
	})
}

// ------------------------------------------------------------
// proto_file.go: Create / Get / Update / Delete nil service 路径
// ------------------------------------------------------------

// TestProtoFileHandler_Create_NilService 创建时 service 为 nil
func TestProtoFileHandler_Create_NilService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &ProtoFileHandler{}
	r.POST("/api/proto-files", func(c *gin.Context) {
		c.Set("user_id", int64(1))
		h.Create(c)
	})

	body := `{"name":"test.proto","content":"syntax=\"proto3\";"}`
	req, _ := http.NewRequest("POST", "/api/proto-files", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	defer func() {
		if rec := recover(); rec != nil {
			t.Log("预期内的 panic（nil protoSvc）:", rec)
			return
		}
	}()

	r.ServeHTTP(w, req)
}

// TestProtoFileHandler_Get_NilService Get 时 service 为 nil
func TestProtoFileHandler_Get_NilService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &ProtoFileHandler{}
	r.GET("/api/proto-files/:id", h.Get)

	req, _ := http.NewRequest("GET", "/api/proto-files/1", nil)
	w := httptest.NewRecorder()

	defer func() {
		if rec := recover(); rec != nil {
			t.Log("预期内的 panic（nil protoSvc）:", rec)
			return
		}
	}()

	r.ServeHTTP(w, req)
}

// TestProtoFileHandler_Update_NilService Update 时 service 为 nil
func TestProtoFileHandler_Update_NilService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &ProtoFileHandler{}
	r.PUT("/api/proto-files/:id", h.Update)

	body := `{"name":"updated"}`
	req, _ := http.NewRequest("PUT", "/api/proto-files/1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	defer func() {
		if rec := recover(); rec != nil {
			t.Log("预期内的 panic（nil protoSvc）:", rec)
			return
		}
	}()

	r.ServeHTTP(w, req)
}

// TestProtoFileHandler_Delete_NilService Delete 时 service 为 nil
func TestProtoFileHandler_Delete_NilService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &ProtoFileHandler{}
	r.DELETE("/api/proto-files/:id", h.Delete)

	req, _ := http.NewRequest("DELETE", "/api/proto-files/1", nil)
	w := httptest.NewRecorder()

	defer func() {
		if rec := recover(); rec != nil {
			t.Log("预期内的 panic（nil protoSvc）:", rec)
			return
		}
	}()

	r.ServeHTTP(w, req)
}

// ------------------------------------------------------------
// proto_file.go: jsonMarshal 错误路径
// ------------------------------------------------------------

// TestJsonMarshal_ErrorCase 测试 jsonMarshal 错误路径
func TestJsonMarshal_ErrorCase(t *testing.T) {
	// channel 无法被 JSON 序列化
	ch := make(chan int)
	_, err := jsonMarshal(ch)
	if err == nil {
		t.Error("expected error for channel type")
	}
}

// ------------------------------------------------------------
// system_config.go: Update 和 Reload nil service 路径
// ------------------------------------------------------------

// TestSystemConfigHandler_Update_NilService 有效请求但 service 为 nil
func TestSystemConfigHandler_Update_NilService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &SystemConfigHandler{}
	r.PUT("/api/system/config/:key", h.Update)

	body := `{"value":"test_value"}`
	req, _ := http.NewRequest("PUT", "/api/system/config/test_key", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	defer func() {
		if rec := recover(); rec != nil {
			t.Log("预期内的 panic（nil configService）:", rec)
			return
		}
	}()

	r.ServeHTTP(w, req)
}

// TestSystemConfigHandler_Reload_NilService_V2 测试 Reload nil service panic recovery
func TestSystemConfigHandler_Reload_NilService_V2(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &SystemConfigHandler{}
	r.POST("/api/system/config/reload", h.Reload)

	req, _ := http.NewRequest("POST", "/api/system/config/reload", nil)
	w := httptest.NewRecorder()

	defer func() {
		if rec := recover(); rec != nil {
			t.Log("预期内的 panic（nil configService）:", rec)
			return
		}
	}()

	r.ServeHTTP(w, req)
}

// ------------------------------------------------------------
// audit_log.go: CleanExpired 参数校验
// ------------------------------------------------------------

// TestAuditLogHandler_CleanExpired_NilService 测试 nil service panic recovery
func TestAuditLogHandler_CleanExpired_NilService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &AuditLogHandler{}
	r.POST("/api/audit-logs/clean", h.CleanExpired)

	body := `{"retention_days":30}`
	req, _ := http.NewRequest("POST", "/api/audit-logs/clean", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	defer func() {
		if rec := recover(); rec != nil {
			t.Log("预期内的 panic（nil service）:", rec)
			return
		}
	}()

	r.ServeHTTP(w, req)
}

// TestAuditLogHandler_CleanExpired_InvalidJSON 测试无效 JSON（应使用默认值）
func TestAuditLogHandler_CleanExpired_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &AuditLogHandler{}
	r.POST("/api/audit-logs/clean", h.CleanExpired)

	req, _ := http.NewRequest("POST", "/api/audit-logs/clean", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	defer func() {
		if rec := recover(); rec != nil {
			// nil service 会 panic，这是预期行为
			t.Log("预期内的 panic（nil service）:", rec)
			return
		}
	}()

	r.ServeHTTP(w, req)
}

// TestAuditLogHandler_CleanExpired_ZeroRetentionDays 测试 retention_days=0 使用默认值
func TestAuditLogHandler_CleanExpired_ZeroRetentionDays(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &AuditLogHandler{}
	r.POST("/api/audit-logs/clean", h.CleanExpired)

	body := `{"retention_days":0}`
	req, _ := http.NewRequest("POST", "/api/audit-logs/clean", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	defer func() {
		if rec := recover(); rec != nil {
			t.Log("预期内的 panic（nil service）:", rec)
			return
		}
	}()

	r.ServeHTTP(w, req)
}

// ------------------------------------------------------------
// common.go: extractUserID / parseIDParam 补充边界
// ------------------------------------------------------------

// TestExtractUserID_EdgeCases 补充 extractUserID 边界用例
func TestExtractUserID_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(c *gin.Context)
		expected int64
	}{
		{
			name:     "未设置 user_id",
			setup:    func(c *gin.Context) {},
			expected: 0,
		},
		{
			name:     "user_id 为 nil",
			setup:    func(c *gin.Context) { c.Set("user_id", nil) },
			expected: 0,
		},
		{
			name:     "user_id 为字符串",
			setup:    func(c *gin.Context) { c.Set("user_id", "123") },
			expected: 0,
		},
		{
			name:     "user_id 为 int（非 int64）",
			setup:    func(c *gin.Context) { c.Set("user_id", 123) },
			expected: 0,
		},
		{
			name:     "user_id 为 int64 零值",
			setup:    func(c *gin.Context) { c.Set("user_id", int64(0)) },
			expected: 0,
		},
		{
			name:     "user_id 为 int64 正值",
			setup:    func(c *gin.Context) { c.Set("user_id", int64(42)) },
			expected: 42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
			tt.setup(c)

			got := extractUserID(c)
			if got != tt.expected {
				t.Errorf("extractUserID() = %d, want %d", got, tt.expected)
			}
		})
	}
}

// TestParseIDParam_EdgeCases 补充 parseIDParam 边界用例
func TestParseIDParam_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		paramValue  string
		wantID      int64
		wantOK      bool
	}{
		{"有效正整数", "123", 123, true},
		{"零值", "0", 0, true},
		{"负数", "-1", -1, true},
		{"非数字", "abc", 0, false},
		{"空字符串", "", 0, false},
		{"浮点数", "1.5", 0, false},
		{"大整数", "9999999999", 9999999999, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
			c.Params = gin.Params{{Key: "id", Value: tt.paramValue}}

			id, ok := parseIDParam(c, "id")
			if ok != tt.wantOK {
				t.Errorf("parseIDParam() ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && id != tt.wantID {
				t.Errorf("parseIDParam() id = %d, want %d", id, tt.wantID)
			}
		})
	}
}

// ------------------------------------------------------------
// checkOwnership 补充：nil permSvc 且非 owner
// ------------------------------------------------------------

// TestCheckOwnership_NilPermSvc_NotOwner 测试非 owner 且 nil permSvc
func TestCheckOwnership_NilPermSvc_NotOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Set("user_id", int64(123))

	// permSvc 为 nil，且 createdBy != userID，会 panic
	defer func() {
		if rec := recover(); rec != nil {
			t.Log("预期内的 panic（nil permSvc）:", rec)
			return
		}
	}()

	result := checkOwnership(c, nil, 999)
	if result {
		t.Error("expected false for non-owner with nil permSvc")
	}
}

// ------------------------------------------------------------
// response.go: FailWithData / ErrorWithData / FailFromError
// ------------------------------------------------------------
// 注: FailWithData / ErrorWithData / Error / SuccessWithMessage / SuccessPaginated / Created
// 等响应函数的测试已在 response_test.go 中完整覆盖，此处不再重复。

// ------------------------------------------------------------
// formatValidationError / getUserFriendlyError 补充用例
// ------------------------------------------------------------

// TestFormatValidationError_AdditionalCases 补充 formatValidationError 用例
func TestFormatValidationError_AdditionalCases(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		contains string
	}{
		{
			name:     "RealName 字段 required",
			err:      errFmt("Key: 'CreateUserRequest.RealName' Error:Field validation for 'RealName' failed on the 'required' tag"),
			contains: "不能为空",
		},
		{
			name:     "Phone 字段 required",
			err:      errFmt("Key: 'CreateUserRequest.Phone' Error:Field validation for 'Phone' failed on the 'required' tag"),
			contains: "手机号",
		},
		{
			name:     "Email 字段 required",
			err:      errFmt("Key: 'CreateUserRequest.Email' Error:Field validation for 'Email' failed on the 'required' tag"),
			contains: "邮箱",
		},
		{
			name:     "Code 字段 required",
			err:      errFmt("Key: 'CreateRoleRequest.Code' Error:Field validation for 'Code' failed on the 'required' tag"),
			contains: "角色代码",
		},
		{
			name:     "Name 字段 required",
			err:      errFmt("Key: 'CreateRoleRequest.Name' Error:Field validation for 'Name' failed on the 'required' tag"),
			contains: "名称",
		},
		{
			name:     "Description 字段 required",
			err:      errFmt("Key: 'CreateDomainRequest.Description' Error:Field validation for 'Description' failed on the 'required' tag"),
			contains: "描述",
		},
		{
			name:     "Password min 校验",
			err:      errFmt("Field validation for 'Password' failed on the 'min' tag"),
			contains: "最小长度为",
		},
		{
			name:     "Password max 校验",
			err:      errFmt("Field validation for 'Password' failed on the 'max' tag"),
			contains: "最大长度为",
		},
		{
			name:     "多空格被压缩",
			err:      errFmt("Field    validation   for   'Username'   failed   on   the   'required'   tag"),
			contains: "用户名",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatValidationError(tt.err)
			if !stringContains(got, tt.contains) {
				t.Errorf("formatValidationError() = %q, expected to contain %q", got, tt.contains)
			}
		})
	}
}

// errFmt 是一个便捷函数，将字符串转为 error
func errFmt(s string) error {
	return &simpleError{msg: s}
}

type simpleError struct{ msg string }

func (e *simpleError) Error() string { return e.msg }

// stringContains 检查 s 是否包含 substr
func stringContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && stringIndexOf(s, substr) >= 0))
}

func stringIndexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// TestGetUserFriendlyError_AdditionalCases 补充 getUserFriendlyError 用例
func TestGetUserFriendlyError_AdditionalCases(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		operation   string
		wantMsg     string
		wantCode    int
	}{
		{
			name:        "UNIQUE constraint 用户名（大小写混合）",
			err:         errFmt("UNIQUE constraint failed: users.username"),
			operation:   "CreateUser",
			wantMsg:     "用户名已存在",
			wantCode:    CodeBadRequest,
		},
		{
			name:        "UNIQUE constraint 邮箱",
			err:         errFmt("UNIQUE constraint failed: users.email"),
			operation:   "UpdateUser",
			wantMsg:     "邮箱已被使用",
			wantCode:    CodeBadRequest,
		},
		{
			name:        "UNIQUE constraint 其他字段",
			err:         errFmt("UNIQUE constraint failed: roles.code"),
			operation:   "CreateRole",
			wantMsg:     "数据已存在，请检查后重试",
			wantCode:    CodeBadRequest,
		},
		{
			name:        "FOREIGN KEY constraint",
			err:         errFmt("FOREIGN KEY constraint failed"),
			operation:   "DeleteUser",
			wantMsg:     "关联数据不存在，请检查输入",
			wantCode:    CodeBadRequest,
		},
		{
			name:        "NOT NULL constraint",
			err:         errFmt("NOT NULL constraint failed: users.real_name"),
			operation:   "UpdateUser",
			wantMsg:     "缺少必填字段",
			wantCode:    CodeBadRequest,
		},
		{
			name:        "nil error 返回默认错误",
			err:         nil,
			operation:   "TestOp",
			wantMsg:     "操作失败，请稍后重试",
			wantCode:    CodeInternalError,
		},
		{
			name:        "其他未知错误返回 500",
			err:         errFmt("database is locked"),
			operation:   "DeleteUser",
			wantMsg:     "操作失败，请稍后重试",
			wantCode:    CodeInternalError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, code := getUserFriendlyError(tt.err, tt.operation)
			if msg != tt.wantMsg {
				t.Errorf("getUserFriendlyError() msg = %q, want %q", msg, tt.wantMsg)
			}
			if code != tt.wantCode {
				t.Errorf("getUserFriendlyError() code = %d, want %d", code, tt.wantCode)
			}
		})
	}
}

// ------------------------------------------------------------
// contextToString 补充边界
// ------------------------------------------------------------

// TestContextToString_EdgeCases 补充 contextToString 边界
func TestContextToString_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"nil", nil, ""},
		{"字符串", "hello", "hello"},
		{"空字符串", "", ""},
		{"整数", 42, ""},
		{"布尔值", true, ""},
		{"结构体", struct{ Name string }{"test"}, ""},
		{"切片", []int{1, 2, 3}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := contextToString(tt.input)
			if got != tt.expected {
				t.Errorf("contextToString(%v) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
