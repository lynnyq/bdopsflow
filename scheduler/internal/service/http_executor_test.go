package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	sysconfig "github.com/lynnyq/bdopsflow/scheduler/internal/system_config"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
)

// newConfigServiceWithPrivateNetwork 构造一个允许访问内网地址的 system_config.Service，
// 用于 HTTP executor 测试中使用 httptest 服务器（监听 127.0.0.1）
func newConfigServiceWithPrivateNetwork(t *testing.T) *sysconfig.Service {
	t.Helper()
	qr := database.NewQueryResultWithRows([][]interface{}{
		{"api_test.allow_private_network", "true"},
	})
	db := &MockDB{QueryResult: qr}
	svc := sysconfig.NewService(db)
	t.Cleanup(func() { svc.Close() })
	return svc
}

// ============ NewHTTPExecutor ============

func TestNewHTTPExecutor(t *testing.T) {
	t.Run("nil configService", func(t *testing.T) {
		e := NewHTTPExecutor(nil)
		if e == nil {
			t.Fatal("期望返回非 nil 实例")
		}
		if e.client == nil {
			t.Error("期望 client 被初始化")
		}
	})

	t.Run("with configService", func(t *testing.T) {
		cs := newConfigServiceWithPrivateNetwork(t)
		e := NewHTTPExecutor(cs)
		if e == nil {
			t.Fatal("期望返回非 nil 实例")
		}
		if e.configService == nil {
			t.Error("期望 configService 被赋值")
		}
	})
}

// ============ Execute ============

func TestHTTPExecutor_Execute_NilConfig(t *testing.T) {
	e := NewHTTPExecutor(nil)
	_, err := e.Execute(context.Background(), nil)
	if err == nil {
		t.Fatal("期望返回错误（config is nil）")
	}
}

func TestHTTPExecutor_Execute_EmptyURL(t *testing.T) {
	e := NewHTTPExecutor(nil)
	config := &model.HTTPRequestConfig{URL: ""}
	_, err := e.Execute(context.Background(), config)
	if err == nil {
		t.Fatal("期望返回错误（URL 为空）")
	}
}

func TestHTTPExecutor_Execute_GET(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("期望 GET，实际 %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"hello"}`))
	}))
	defer srv.Close()

	e := NewHTTPExecutor(newConfigServiceWithPrivateNetwork(t))
	config := &model.HTTPRequestConfig{
		Method: "GET",
		URL:    srv.URL,
	}

	result, err := e.Execute(context.Background(), config)
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if result.StatusCode != http.StatusOK {
		t.Errorf("期望 StatusCode=200，实际=%d", result.StatusCode)
	}
	if !strings.Contains(result.Body, "hello") {
		t.Errorf("期望 Body 包含 hello，实际=%s", result.Body)
	}
	if result.LatencyMs < 0 {
		t.Error("期望 LatencyMs >= 0")
	}
}

func TestHTTPExecutor_Execute_EmptyMethodDefaultsToGET(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("期望默认 GET，实际 %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	e := NewHTTPExecutor(newConfigServiceWithPrivateNetwork(t))
	config := &model.HTTPRequestConfig{
		Method: "",
		URL:    srv.URL,
	}

	_, err := e.Execute(context.Background(), config)
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
}

func TestHTTPExecutor_Execute_POST_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("期望 POST，实际 %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("期望 Content-Type=application/json，实际=%s", r.Header.Get("Content-Type"))
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "test") {
			t.Errorf("期望 body 包含 test，实际=%s", string(body))
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":1}`))
	}))
	defer srv.Close()

	e := NewHTTPExecutor(newConfigServiceWithPrivateNetwork(t))
	config := &model.HTTPRequestConfig{
		Method: "POST",
		URL:    srv.URL,
		Body: &model.HTTPBodyConfig{
			Type:    "json",
			Content: `{"name":"test"}`,
		},
	}

	result, err := e.Execute(context.Background(), config)
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if result.StatusCode != http.StatusCreated {
		t.Errorf("期望 StatusCode=201，实际=%d", result.StatusCode)
	}
}

func TestHTTPExecutor_Execute_WithHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Custom") != "custom-value" {
			t.Errorf("期望 X-Custom=custom-value，实际=%s", r.Header.Get("X-Custom"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	e := NewHTTPExecutor(newConfigServiceWithPrivateNetwork(t))
	config := &model.HTTPRequestConfig{
		URL: srv.URL,
		Headers: []model.KeyValue{
			{Key: "X-Custom", Value: "custom-value"},
		},
	}

	_, err := e.Execute(context.Background(), config)
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
}

func TestHTTPExecutor_Execute_BearerAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			t.Errorf("期望 Bearer token，实际=%s", auth)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	e := NewHTTPExecutor(newConfigServiceWithPrivateNetwork(t))
	config := &model.HTTPRequestConfig{
		URL: srv.URL,
		Auth: &model.HTTPAuthConfig{
			Type:  "bearer",
			Token: "my-token",
		},
	}

	_, err := e.Execute(context.Background(), config)
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
}

func TestHTTPExecutor_Execute_BasicAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "admin" || pass != "secret" {
			t.Errorf("期望 admin:secret，实际 user=%s pass=%s", user, pass)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	e := NewHTTPExecutor(newConfigServiceWithPrivateNetwork(t))
	config := &model.HTTPRequestConfig{
		URL: srv.URL,
		Auth: &model.HTTPAuthConfig{
			Type: "basic",
			User: "admin",
			Pass: "secret",
		},
	}

	_, err := e.Execute(context.Background(), config)
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
}

func TestHTTPExecutor_Execute_APIKeyHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "key-value" {
			t.Errorf("期望 X-API-Key=key-value，实际=%s", r.Header.Get("X-API-Key"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	e := NewHTTPExecutor(newConfigServiceWithPrivateNetwork(t))
	config := &model.HTTPRequestConfig{
		URL: srv.URL,
		Auth: &model.HTTPAuthConfig{
			Type:  "apikey",
			In:    "header",
			Key:   "X-API-Key",
			Value: "key-value",
		},
	}

	_, err := e.Execute(context.Background(), config)
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
}

func TestHTTPExecutor_Execute_APIKeyQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("api_key") != "query-value" {
			t.Errorf("期望 api_key=query-value，实际=%s", r.URL.Query().Get("api_key"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	e := NewHTTPExecutor(newConfigServiceWithPrivateNetwork(t))
	config := &model.HTTPRequestConfig{
		URL: srv.URL,
		Auth: &model.HTTPAuthConfig{
			Type:  "apikey",
			In:    "query",
			Key:   "api_key",
			Value: "query-value",
		},
	}

	_, err := e.Execute(context.Background(), config)
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
}

func TestHTTPExecutor_Execute_WithParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("foo") != "bar" {
			t.Errorf("期望 foo=bar，实际=%s", r.URL.Query().Get("foo"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	e := NewHTTPExecutor(newConfigServiceWithPrivateNetwork(t))
	config := &model.HTTPRequestConfig{
		URL: srv.URL,
		Params: []model.KeyValue{
			{Key: "foo", Value: "bar"},
		},
	}

	_, err := e.Execute(context.Background(), config)
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
}

func TestHTTPExecutor_Execute_PreScript(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	e := NewHTTPExecutor(newConfigServiceWithPrivateNetwork(t))
	config := &model.HTTPRequestConfig{
		URL:       srv.URL,
		PreScript: "",
	}

	_, err := e.Execute(context.Background(), config)
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
}

func TestHTTPExecutor_Execute_FormUrlEncoded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("期望 form-urlencoded Content-Type，实际=%s", r.Header.Get("Content-Type"))
		}
		_ = r.ParseForm()
		if r.Form.Get("field1") != "value1" {
			t.Errorf("期望 field1=value1，实际=%s", r.Form.Get("field1"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	e := NewHTTPExecutor(newConfigServiceWithPrivateNetwork(t))
	bodyContent, _ := json.Marshal([]model.KeyValue{
		{Key: "field1", Value: "value1"},
	})
	config := &model.HTTPRequestConfig{
		Method: "POST",
		URL:    srv.URL,
		Body: &model.HTTPBodyConfig{
			Type:    "form-urlencoded",
			Content: string(bodyContent),
		},
	}

	_, err := e.Execute(context.Background(), config)
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
}

func TestHTTPExecutor_Execute_RawBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if string(body) != "raw text content" {
			t.Errorf("期望 raw text content，实际=%s", string(body))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	e := NewHTTPExecutor(newConfigServiceWithPrivateNetwork(t))
	config := &model.HTTPRequestConfig{
		Method: "POST",
		URL:    srv.URL,
		Body: &model.HTTPBodyConfig{
			Type:    "raw",
			Content: "raw text content",
		},
	}

	_, err := e.Execute(context.Background(), config)
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
}

func TestHTTPExecutor_Execute_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal"}`))
	}))
	defer srv.Close()

	e := NewHTTPExecutor(newConfigServiceWithPrivateNetwork(t))
	config := &model.HTTPRequestConfig{URL: srv.URL}

	result, err := e.Execute(context.Background(), config)
	if err != nil {
		t.Fatalf("期望无错误（HTTP 500 不是请求错误），实际: %v", err)
	}
	if result.StatusCode != http.StatusInternalServerError {
		t.Errorf("期望 StatusCode=500，实际=%d", result.StatusCode)
	}
}

func TestHTTPExecutor_Execute_InvalidURL(t *testing.T) {
	e := NewHTTPExecutor(nil)
	config := &model.HTTPRequestConfig{
		URL: "http://192.168.1.1/test", // 私有 IP，SSRF 应阻止
	}

	_, err := e.Execute(context.Background(), config)
	if err == nil {
		t.Fatal("期望返回错误（SSRF 阻止）")
	}
}

// ============ GenerateCurl ============

func TestGenerateCurl_NilConfig(t *testing.T) {
	e := NewHTTPExecutor(nil)
	_, err := e.GenerateCurl(nil)
	if err == nil {
		t.Fatal("期望返回错误（config is nil）")
	}
}

func TestGenerateCurl_SimpleGET(t *testing.T) {
	e := NewHTTPExecutor(nil)
	config := &model.HTTPRequestConfig{
		Method: "GET",
		URL:    "http://example.com/api",
	}

	result, err := e.GenerateCurl(config)
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if !strings.Contains(result, "curl") {
		t.Error("期望包含 curl")
	}
	if !strings.Contains(result, "-X GET") {
		t.Error("期望包含 -X GET")
	}
	if !strings.Contains(result, "example.com") {
		t.Error("期望包含 URL")
	}
}

func TestGenerateCurl_EmptyMethodDefaultsToGET(t *testing.T) {
	e := NewHTTPExecutor(nil)
	config := &model.HTTPRequestConfig{
		Method: "",
		URL:    "http://example.com",
	}

	result, err := e.GenerateCurl(config)
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if !strings.Contains(result, "-X GET") {
		t.Error("空 method 应默认为 GET")
	}
}

func TestGenerateCurl_WithJSONBody(t *testing.T) {
	e := NewHTTPExecutor(nil)
	config := &model.HTTPRequestConfig{
		Method: "POST",
		URL:    "http://example.com/api",
		Body: &model.HTTPBodyConfig{
			Type:    "json",
			Content: `{"key":"value"}`,
		},
	}

	result, err := e.GenerateCurl(config)
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if !strings.Contains(result, "-d") {
		t.Error("期望包含 -d（数据）")
	}
}

func TestGenerateCurl_WithBearerAuth(t *testing.T) {
	e := NewHTTPExecutor(nil)
	config := &model.HTTPRequestConfig{
		URL: "http://example.com",
		Auth: &model.HTTPAuthConfig{
			Type:  "bearer",
			Token: "tok",
		},
	}

	result, err := e.GenerateCurl(config)
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if !strings.Contains(result, "Authorization") {
		t.Error("期望包含 Authorization header")
	}
}

func TestGenerateCurl_WithBasicAuth(t *testing.T) {
	e := NewHTTPExecutor(nil)
	config := &model.HTTPRequestConfig{
		URL: "http://example.com",
		Auth: &model.HTTPAuthConfig{
			Type: "basic",
			User: "u",
			Pass: "p",
		},
	}

	result, err := e.GenerateCurl(config)
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if !strings.Contains(result, "--user") {
		t.Error("期望包含 --user")
	}
}

func TestGenerateCurl_WithAPIKeyInQuery(t *testing.T) {
	e := NewHTTPExecutor(nil)
	config := &model.HTTPRequestConfig{
		URL: "http://example.com",
		Auth: &model.HTTPAuthConfig{
			Type:  "apikey",
			In:    "query",
			Key:   "k",
			Value: "v",
		},
	}

	result, err := e.GenerateCurl(config)
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if !strings.Contains(result, "k=v") {
		t.Error("期望包含 apikey query 参数")
	}
}

// ============ ExecuteAssertions ============

func TestExecuteAssertions(t *testing.T) {
	e := NewHTTPExecutor(nil)
	result := &model.ApiTestResult{
		StatusCode: 200,
		Body:       `{"name":"test","count":42}`,
		Headers:    `{"Content-Type":["application/json"]}`,
	}

	tests := []struct {
		name      string
		assertion model.AssertionConfig
		expectPassed bool
	}{
		{
			name: "status_code equals",
			assertion: model.AssertionConfig{
				Type: "status_code", Operator: "equals", Expected: "200",
			},
			expectPassed: true,
		},
		{
			name: "status_code not_equals",
			assertion: model.AssertionConfig{
				Type: "status_code", Operator: "not_equals", Expected: "404",
			},
			expectPassed: true,
		},
		{
			name: "json_path equals",
			assertion: model.AssertionConfig{
				Type: "json_path", Target: "name", Operator: "equals", Expected: "test",
			},
			expectPassed: true,
		},
		{
			name: "json_path contains",
			assertion: model.AssertionConfig{
				Type: "json_path", Target: "name", Operator: "contains", Expected: "es",
			},
			expectPassed: true,
		},
		{
			name: "json_path gt",
			assertion: model.AssertionConfig{
				Type: "json_path", Target: "count", Operator: "gt", Expected: "10",
			},
			expectPassed: true,
		},
		{
			name: "json_path lt fail",
			assertion: model.AssertionConfig{
				Type: "json_path", Target: "count", Operator: "lt", Expected: "10",
			},
			expectPassed: false,
		},
		{
			name: "json_path exists",
			assertion: model.AssertionConfig{
				Type: "json_path", Target: "name", Operator: "exists",
			},
			expectPassed: true,
		},
		{
			name: "header equals",
			assertion: model.AssertionConfig{
				Type: "header", Target: "Content-Type", Operator: "contains", Expected: "json",
			},
			expectPassed: true,
		},
		{
			name: "unsupported type",
			assertion: model.AssertionConfig{
				Type: "unsupported", Operator: "equals",
			},
			expectPassed: false,
		},
		{
			name: "unsupported operator",
			assertion: model.AssertionConfig{
				Type: "status_code", Operator: "unsupported",
			},
			expectPassed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := e.ExecuteAssertions(result, []model.AssertionConfig{tt.assertion})
			if len(results) != 1 {
				t.Fatalf("期望 1 个结果，实际 %d", len(results))
			}
			if results[0].Passed != tt.expectPassed {
				t.Errorf("期望 Passed=%v，实际=%v，消息=%s", tt.expectPassed, results[0].Passed, results[0].Message)
			}
		})
	}
}

func TestExecuteAssertions_Empty(t *testing.T) {
	e := NewHTTPExecutor(nil)
	result := &model.ApiTestResult{StatusCode: 200}

	results := e.ExecuteAssertions(result, nil)
	if len(results) != 0 {
		t.Errorf("期望 0 个结果，实际 %d", len(results))
	}
}

func TestExecuteAssertions_JSONPathEmptyBody(t *testing.T) {
	e := NewHTTPExecutor(nil)
	result := &model.ApiTestResult{StatusCode: 200, Body: ""}

	results := e.ExecuteAssertions(result, []model.AssertionConfig{
		{Type: "json_path", Target: "name", Operator: "equals", Expected: "test"},
	})
	if len(results) != 1 {
		t.Fatalf("期望 1 个结果，实际 %d", len(results))
	}
	if results[0].Passed {
		t.Error("空 body 时 json_path 断言应失败")
	}
}

func TestExecuteAssertions_JSONPathNotFound(t *testing.T) {
	e := NewHTTPExecutor(nil)
	result := &model.ApiTestResult{
		StatusCode: 200,
		Body:       `{"name":"test"}`,
	}

	results := e.ExecuteAssertions(result, []model.AssertionConfig{
		{Type: "json_path", Target: "nonexistent", Operator: "equals", Expected: "x"},
	})
	if len(results) != 1 {
		t.Fatalf("期望 1 个结果，实际 %d", len(results))
	}
	if results[0].Passed {
		t.Error("不存在的路径应失败")
	}
}

func TestExecuteAssertions_HeaderNotFound(t *testing.T) {
	e := NewHTTPExecutor(nil)
	result := &model.ApiTestResult{
		StatusCode: 200,
		Headers:    `{"Content-Type":["application/json"]}`,
	}

	results := e.ExecuteAssertions(result, []model.AssertionConfig{
		{Type: "header", Target: "X-Nonexistent", Operator: "exists"},
	})
	if len(results) != 1 {
		t.Fatalf("期望 1 个结果，实际 %d", len(results))
	}
	if results[0].Passed {
		t.Error("不存在的 header 应失败")
	}
}

// ============ buildURL ============

func TestBuildURL(t *testing.T) {
	tests := []struct {
		name        string
		rawURL      string
		params      []model.KeyValue
		expectErr   bool
		expectContain string
	}{
		{
			name:        "empty URL",
			rawURL:      "",
			expectErr:   true,
		},
		{
			name:        "simple URL no params",
			rawURL:      "http://example.com/api",
			expectContain: "example.com",
		},
		{
			name:   "URL with params",
			rawURL: "http://example.com/api",
			params: []model.KeyValue{
				{Key: "foo", Value: "bar"},
				{Key: "baz", Value: "qux"},
			},
			expectContain: "foo=bar",
		},
		{
			name:   "params with empty key skipped",
			rawURL: "http://example.com/api",
			params: []model.KeyValue{
				{Key: "", Value: "skip"},
				{Key: "keep", Value: "me"},
			},
			expectContain: "keep=me",
		},
		{
			name:        "invalid URL",
			rawURL:      "://invalid",
			expectErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := buildURL(tt.rawURL, tt.params)
			if tt.expectErr && err == nil {
				t.Fatal("期望返回错误")
			}
			if !tt.expectErr && err != nil {
				t.Fatalf("期望无错误，实际: %v", err)
			}
			if !tt.expectErr && tt.expectContain != "" {
				if !strings.Contains(result, tt.expectContain) {
					t.Errorf("期望结果包含 %q，实际=%s", tt.expectContain, result)
				}
			}
		})
	}
}

// ============ buildBody ============

func TestBuildBody(t *testing.T) {
	tests := []struct {
		name             string
		body             *model.HTTPBodyConfig
		expectErr        bool
		expectContentType string
	}{
		{
			name: "nil body",
			body: nil,
		},
		{
			name: "none type",
			body: &model.HTTPBodyConfig{Type: "none"},
		},
		{
			name: "empty type",
			body: &model.HTTPBodyConfig{Type: ""},
		},
		{
			name: "json type",
			body: &model.HTTPBodyConfig{
				Type:    "json",
				Content: `{"key":"value"}`,
			},
			expectContentType: "application/json",
		},
		{
			name: "raw type",
			body: &model.HTTPBodyConfig{
				Type:    "raw",
				Content: "plain text",
			},
		},
		{
			name: "form-urlencoded invalid json",
			body: &model.HTTPBodyConfig{
				Type:    "form-urlencoded",
				Content: "invalid json",
			},
			expectErr: true,
		},
		{
			name: "form-multipart invalid json",
			body: &model.HTTPBodyConfig{
				Type:    "form-multipart",
				Content: "invalid json",
			},
			expectErr: true,
		},
		{
			name: "binary invalid base64",
			body: &model.HTTPBodyConfig{
				Type:    "binary",
				Content: "not-base64!!!",
			},
			expectErr: true,
		},
		{
			name: "unsupported type",
			body: &model.HTTPBodyConfig{
				Type:    "unsupported",
				Content: "data",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, contentType, err := buildBody(tt.body)
			if tt.expectErr {
				if err == nil {
					t.Fatal("期望返回错误")
				}
				return
			}
			if err != nil {
				t.Fatalf("期望无错误，实际: %v", err)
			}
			if tt.expectContentType != "" && contentType != tt.expectContentType {
				t.Errorf("期望 Content-Type=%s，实际=%s", tt.expectContentType, contentType)
			}
			// 验证 reader 可读（非 nil 时）
			if reader != nil && tt.body != nil && tt.body.Content != "" {
				data, readErr := io.ReadAll(reader)
				if readErr != nil {
					t.Errorf("读取 body 失败: %v", readErr)
				}
				if len(data) == 0 {
					t.Error("期望非空 body")
				}
			}
		})
	}
}

func TestBuildBody_FormUrlEncoded(t *testing.T) {
	bodyContent, _ := json.Marshal([]model.KeyValue{
		{Key: "k1", Value: "v1"},
	})
	reader, contentType, err := buildBody(&model.HTTPBodyConfig{
		Type:    "form-urlencoded",
		Content: string(bodyContent),
	})
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if contentType != "application/x-www-form-urlencoded" {
		t.Errorf("期望 Content-Type=application/x-www-form-urlencoded，实际=%s", contentType)
	}
	data, _ := io.ReadAll(reader)
	if !strings.Contains(string(data), "k1=v1") {
		t.Errorf("期望包含 k1=v1，实际=%s", string(data))
	}
}

func TestBuildBody_FormMultipart(t *testing.T) {
	bodyContent, _ := json.Marshal([]model.KeyValue{
		{Key: "field1", Value: "value1"},
	})
	reader, contentType, err := buildBody(&model.HTTPBodyConfig{
		Type:    "form-multipart",
		Content: string(bodyContent),
	})
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		t.Errorf("期望 multipart/form-data Content-Type，实际=%s", contentType)
	}
	data, _ := io.ReadAll(reader)
	if !strings.Contains(string(data), "field1") {
		t.Errorf("期望包含 field1，实际=%s", string(data))
	}
}

func TestBuildBody_Binary(t *testing.T) {
	reader, _, err := buildBody(&model.HTTPBodyConfig{
		Type:    "binary",
		Content: "aGVsbG8=", // "hello" in base64
	})
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	data, _ := io.ReadAll(reader)
	if string(data) != "hello" {
		t.Errorf("期望 hello，实际=%s", string(data))
	}
}

// ============ applyAuth ============

func TestApplyAuth(t *testing.T) {
	tests := []struct {
		name        string
		auth        *model.HTTPAuthConfig
		expectErr   bool
		expectHeader string
		expectValue  string
	}{
		{
			name: "nil auth",
			auth: nil,
		},
		{
			name: "none type",
			auth: &model.HTTPAuthConfig{Type: "none"},
		},
		{
			name: "empty type",
			auth: &model.HTTPAuthConfig{Type: ""},
		},
		{
			name:         "bearer",
			auth:         &model.HTTPAuthConfig{Type: "bearer", Token: "tok"},
			expectHeader: "Authorization",
			expectValue:  "Bearer tok",
		},
		{
			name:         "basic",
			auth:         &model.HTTPAuthConfig{Type: "basic", User: "u", Pass: "p"},
			expectHeader: "Authorization",
		},
		{
			name:         "apikey header default",
			auth:         &model.HTTPAuthConfig{Type: "apikey", Key: "X-Key", Value: "v"},
			expectHeader: "X-Key",
			expectValue:  "v",
		},
		{
			name:      "apikey unsupported location",
			auth:      &model.HTTPAuthConfig{Type: "apikey", In: "cookie", Key: "k", Value: "v"},
			expectErr: true,
		},
		{
			name:      "unsupported auth type",
			auth:      &model.HTTPAuthConfig{Type: "oauth"},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "http://example.com", nil)
			err := applyAuth(req, tt.auth, "http://example.com")
			if tt.expectErr {
				if err == nil {
					t.Fatal("期望返回错误")
				}
				return
			}
			if err != nil {
				t.Fatalf("期望无错误，实际: %v", err)
			}
			if tt.expectHeader != "" && tt.expectValue != "" {
				if req.Header.Get(tt.expectHeader) != tt.expectValue {
					t.Errorf("期望 %s=%s，实际=%s", tt.expectHeader, tt.expectValue, req.Header.Get(tt.expectHeader))
				}
			}
		})
	}
}

func TestApplyAuth_APIKeyQuery(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://example.com/path", nil)
	err := applyAuth(req, &model.HTTPAuthConfig{
		Type:  "apikey",
		In:    "query",
		Key:   "api_key",
		Value: "secret",
	}, "http://example.com/path")
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if req.URL.Query().Get("api_key") != "secret" {
		t.Errorf("期望 api_key=secret，实际=%s", req.URL.Query().Get("api_key"))
	}
}

// ============ compareValues ============

func TestCompareValues(t *testing.T) {
	tests := []struct {
		name     string
		operator string
		actual   string
		expected string
		passed   bool
	}{
		{"equals pass", "equals", "200", "200", true},
		{"equals fail", "equals", "200", "404", false},
		{"not_equals pass", "not_equals", "200", "404", true},
		{"not_equals fail", "not_equals", "200", "200", false},
		{"contains pass", "contains", "hello world", "world", true},
		{"contains fail", "contains", "hello", "world", false},
		{"gt pass", "gt", "100", "50", true},
		{"gt fail", "gt", "50", "100", false},
		{"lt pass", "lt", "50", "100", true},
		{"lt fail", "lt", "100", "50", false},
		{"exists pass", "exists", "value", "", true},
		{"exists fail", "exists", "", "", false},
		{"unsupported operator", "regex", "value", "val", false},
		{"gt non-numeric", "gt", "abc", "50", false},
		{"lt non-numeric", "lt", "abc", "50", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			passed, msg := compareValues(tt.operator, tt.actual, tt.expected)
			if passed != tt.passed {
				t.Errorf("期望 passed=%v，实际=%v，msg=%s", tt.passed, passed, msg)
			}
		})
	}
}

// ============ valueToString ============

func TestValueToString(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"string", "hello", "hello"},
		{"int float64", float64(42), "42"},
		{"float64", float64(3.14), "3.14"},
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"nil", nil, "null"},
		{"slice", []int{1, 2}, "[1,2]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := valueToString(tt.input)
			if result != tt.expected {
				t.Errorf("期望 %q，实际 %q", tt.expected, result)
			}
		})
	}
}

// ============ shellEscape ============

func TestShellEscape(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple", "hello", "'hello'"},
		{"with space", "hello world", "'hello world'"},
		{"with single quote", "it's", "'it'\\''s'"},
		{"empty", "", "''"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shellEscape(tt.input)
			if result != tt.expected {
				t.Errorf("期望 %q，实际 %q", tt.expected, result)
			}
		})
	}
}

// ============ extractJSONPath ============

func TestExtractJSONPath(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		path      string
		expectErr bool
		expected  string
	}{
		{
			name:     "simple path",
			body:     `{"name":"test"}`,
			path:     "name",
			expected: "test",
		},
		{
			name:     "nested path",
			body:     `{"user":{"name":"alice"}}`,
			path:     "user.name",
			expected: "alice",
		},
		{
			name:      "empty body",
			body:      "",
			path:      "name",
			expectErr: true,
		},
		{
			name:      "empty path",
			body:      `{"name":"test"}`,
			path:      "",
			expectErr: true,
		},
		{
			name:      "invalid JSON",
			body:      "not json",
			path:      "name",
			expectErr: true,
		},
		{
			name:      "path not found",
			body:      `{"name":"test"}`,
			path:      "nonexistent",
			expectErr: true,
		},
		{
			name:      "path on non-object",
			body:      `{"name":"test"}`,
			path:      "name.foo",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractJSONPath(tt.body, tt.path)
			if tt.expectErr {
				if err == nil {
					t.Fatal("期望返回错误")
				}
				return
			}
			if err != nil {
				t.Fatalf("期望无错误，实际: %v", err)
			}
			if result != tt.expected {
				t.Errorf("期望 %q，实际 %q", tt.expected, result)
			}
		})
	}
}

// ============ extractHeader ============

func TestExtractHeader(t *testing.T) {
	tests := []struct {
		name       string
		headersJSON string
		target     string
		expectErr  bool
		expected   string
	}{
		{
			name:        "exact match",
			headersJSON: `{"Content-Type":["application/json"]}`,
			target:      "Content-Type",
			// valueToString 对切片值会进行 JSON 序列化，返回 ["application/json"]
			expected: `["application/json"]`,
		},
		{
			name:        "case insensitive",
			headersJSON: `{"Content-Type":["application/json"]}`,
			target:      "content-type",
			// valueToString 对切片值会进行 JSON 序列化，返回 ["application/json"]
			expected: `["application/json"]`,
		},
		{
			name:        "not found",
			headersJSON: `{"Content-Type":["application/json"]}`,
			target:      "X-Nonexistent",
			expectErr:   true,
		},
		{
			name:        "empty headers",
			headersJSON: "",
			target:      "Content-Type",
			expectErr:   true,
		},
		{
			name:        "invalid JSON",
			headersJSON: "not json",
			target:      "Content-Type",
			expectErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractHeader(tt.headersJSON, tt.target)
			if tt.expectErr {
				if err == nil {
					t.Fatal("期望返回错误")
				}
				return
			}
			if err != nil {
				t.Fatalf("期望无错误，实际: %v", err)
			}
			if result != tt.expected {
				t.Errorf("期望 %q，实际 %q", tt.expected, result)
			}
		})
	}
}

// ============ applyPreScript ============

func TestApplyPreScript(t *testing.T) {
	t.Run("empty script", func(t *testing.T) {
		config := &model.HTTPRequestConfig{URL: "http://example.com"}
		err := applyPreScript("", config)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
	})

	t.Run("nil config", func(t *testing.T) {
		err := applyPreScript("script", nil)
		if err != nil {
			t.Fatalf("期望无错误（nil config 时直接返回 nil），实际: %v", err)
		}
	})

	t.Run("valid template on URL", func(t *testing.T) {
		config := &model.HTTPRequestConfig{
			URL: "http://example.com/{{timestamp}}",
		}
		// 注意：applyPreScript 会将整个 URL 替换为模板执行结果
		// 模板 "http://example.com/{{timestamp}}" 会被解析执行
		err := applyPreScript("http://example.com/{{timestamp}}", config)
		// 模板可能执行成功也可能失败，取决于模板语法
		// 主要验证不 panic
		_ = err
	})

	t.Run("template with uuid function", func(t *testing.T) {
		config := &model.HTTPRequestConfig{
			Headers: []model.KeyValue{
				{Key: "X-Request-ID", Value: "{{uuid}}"},
			},
		}
		err := applyPreScript("{{uuid}}", config)
		_ = err
	})
}

// ============ validateURL ============

func TestValidateURL(t *testing.T) {
	t.Run("nil configService - private IP blocked", func(t *testing.T) {
		e := NewHTTPExecutor(nil)
		err := e.validateURL("http://192.168.1.1/test")
		if err == nil {
			t.Fatal("期望私有 IP 被阻止")
		}
	})

	t.Run("nil configService - empty URL", func(t *testing.T) {
		e := NewHTTPExecutor(nil)
		err := e.validateURL("")
		if err == nil {
			t.Fatal("期望空 URL 返回错误")
		}
	})

	t.Run("allow private network", func(t *testing.T) {
		e := NewHTTPExecutor(newConfigServiceWithPrivateNetwork(t))
		err := e.validateURL("http://192.168.1.1/test")
		if err != nil {
			t.Fatalf("期望允许私有网络时无错误，实际: %v", err)
		}
	})

	t.Run("nil configService - invalid URL", func(t *testing.T) {
		e := NewHTTPExecutor(nil)
		err := e.validateURL("://invalid")
		if err == nil {
			t.Fatal("期望无效 URL 返回错误")
		}
	})

	t.Run("nil configService - no host", func(t *testing.T) {
		e := NewHTTPExecutor(nil)
		err := e.validateURL("http:///path")
		if err == nil {
			t.Fatal("期望无 host 的 URL 返回错误")
		}
	})
}
