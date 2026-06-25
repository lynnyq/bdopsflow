package model

import "time"

// ApiTest 接口测试用例
type ApiTest struct {
	ID        int64     `db:"id" json:"id"`
	Name      string    `db:"name" json:"name"`
	Type      string    `db:"type" json:"type"` // http 或 grpc
	Config    string    `db:"config" json:"config"`
	CreatedBy int64     `db:"created_by" json:"created_by"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// ApiTestResult 接口测试执行结果
type ApiTestResult struct {
	ID              int64     `db:"id" json:"id"`
	TestID          int64     `db:"test_id" json:"test_id"`
	Type            string    `db:"type" json:"type"`
	StatusCode      int       `db:"status_code" json:"status_code"`
	LatencyMs       int64     `db:"latency_ms" json:"latency_ms"`
	Headers         string    `db:"headers" json:"headers"`
	Body            string    `db:"body" json:"body"`
	Error           string    `db:"error" json:"error"`
	AssertionsResult string   `db:"assertions_result" json:"assertions_result"`
	ExecutedBy      int64     `db:"executed_by" json:"executed_by"`
	ExecutedByName  string    `db:"-" json:"executed_by_name,omitempty"`
	CreatedAt       time.Time `db:"created_at" json:"created_at"`
	TestName        string    `db:"-" json:"test_name,omitempty"`
}

// HTTPRequestConfig HTTP请求配置
type HTTPRequestConfig struct {
	Method     string              `json:"method"`
	URL        string              `json:"url"`
	Headers    []KeyValue          `json:"headers,omitempty"`
	Params     []KeyValue          `json:"params,omitempty"`
	Body       *HTTPBodyConfig     `json:"body,omitempty"`
	Auth       *HTTPAuthConfig     `json:"auth,omitempty"`
	PreScript  string              `json:"pre_script,omitempty"`
	PostScript string              `json:"post_script,omitempty"`
	Timeout    int                 `json:"timeout,omitempty"`
}

// HTTPBodyConfig HTTP请求体配置
type HTTPBodyConfig struct {
	Type    string `json:"type"`    // none/json/form-urlencoded/form-multipart/raw/binary
	Content string `json:"content"`
}

// HTTPAuthConfig HTTP认证配置
type HTTPAuthConfig struct {
	Type   string `json:"type"`   // none/bearer/basic/apikey
	Token  string `json:"token,omitempty"`
	User   string `json:"user,omitempty"`
	Pass   string `json:"pass,omitempty"`
	Key    string `json:"key,omitempty"`
	Value  string `json:"value,omitempty"`
	In     string `json:"in,omitempty"` // header/query
}

// GRPCRequestConfig gRPC请求配置
type GRPCRequestConfig struct {
	Address       string     `json:"address"`
	Service       string     `json:"service"`
	Method        string     `json:"method"`
	RequestBody   string     `json:"request_body"`
	Metadata      []KeyValue `json:"metadata,omitempty"`
	TLSMode       string     `json:"tls_mode"`        // insecure/tls/mtls
	CertificateID *int64     `json:"certificate_id,omitempty"`
	ProtoFileID   *int64     `json:"proto_file_id,omitempty"`
	UseReflection bool       `json:"use_reflection"`
	Timeout       int        `json:"timeout,omitempty"`
}

// KeyValue 通用键值对
type KeyValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// AssertionConfig 断言配置
type AssertionConfig struct {
	Type     string `json:"type"`     // status_code/json_path/header
	Target   string `json:"target"`   // 断言目标
	Operator string `json:"operator"` // equals/not_equals/contains/gt/lt/exists
	Expected string `json:"expected"` // 期望值
}

// AssertionResult 断言结果
type AssertionResult struct {
	Assertion AssertionConfig `json:"assertion"`
	Passed    bool           `json:"passed"`
	Actual    string         `json:"actual"`
	Message   string         `json:"message,omitempty"`
}
