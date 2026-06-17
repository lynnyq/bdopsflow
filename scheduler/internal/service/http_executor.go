package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/google/uuid"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	sysconfig "github.com/lynnyq/bdopsflow/scheduler/internal/system_config"
)

const maxResponseBodySize = 10 * 1024 * 1024 // 10MB

// HTTPExecutor handles executing HTTP requests as a proxy.
type HTTPExecutor struct {
	client       *http.Client
	configService *sysconfig.Service
}

// NewHTTPExecutor creates a new HTTPExecutor with a configured HTTP client.
func NewHTTPExecutor(configService *sysconfig.Service) *HTTPExecutor {
	return &HTTPExecutor{
		configService: configService,
		client: &http.Client{
			Timeout: 310 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				MaxIdleConns:    100,
				IdleConnTimeout: 90 * time.Second,
			},
		},
	}
}

// Execute performs an HTTP request based on the provided config and returns the result.
func (e *HTTPExecutor) Execute(ctx context.Context, config *model.HTTPRequestConfig) (*model.ApiTestResult, error) {
	if config == nil {
		return nil, fmt.Errorf("config is nil")
	}

	// SSRF protection: validate URL before making request
	if err := e.validateURL(config.URL); err != nil {
		return nil, fmt.Errorf("URL validation failed: %w", err)
	}

	// Apply pre-script variable substitution
	if err := applyPreScript(config.PreScript, config); err != nil {
		slog.Warn("failed to apply pre-script", "error", err)
	}

	// Build URL with query params
	reqURL, err := buildURL(config.URL, config.Params)
	if err != nil {
		return nil, fmt.Errorf("failed to build URL: %w", err)
	}

	// Build request body
	var bodyReader io.Reader
	var contentType string
	bodyReader, contentType, err = buildBody(config.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to build request body: %w", err)
	}

	// Create request with context
	method := config.Method
	if method == "" {
		method = http.MethodGet
	}
	req, err := http.NewRequestWithContext(ctx, method, reqURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set Content-Type if determined from body
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	// Apply custom headers
	for _, h := range config.Headers {
		if h.Key != "" {
			req.Header.Set(h.Key, h.Value)
		}
	}

	// Apply auth
	if err := applyAuth(req, config.Auth, reqURL); err != nil {
		return nil, fmt.Errorf("failed to apply auth: %w", err)
	}

	// Enforce max timeout of 300s, default to 30s if not set
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = 30
	}
	if timeout > 300 {
		timeout = 300
	}

	// Use shared client with context-based timeout for connection pooling
	ctxWithTimeout, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()
	req = req.WithContext(ctxWithTimeout)

	// Execute request and measure latency
	start := time.Now()
	resp, err := e.client.Do(req)
	latency := time.Since(start)
	if err != nil {
		return &model.ApiTestResult{
			LatencyMs: latency.Milliseconds(),
			Error:     err.Error(),
		}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body (limited to maxResponseBodySize+1 to detect truncation)
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodySize+1))
	if err != nil {
		return &model.ApiTestResult{
			StatusCode: resp.StatusCode,
			LatencyMs:  latency.Milliseconds(),
			Error:      fmt.Sprintf("failed to read response body: %v", err),
		}, fmt.Errorf("failed to read response body: %w", err)
	}

	// Serialize response headers
	headersMap := make(map[string][]string)
	for k, v := range resp.Header {
		headersMap[k] = v
	}
	headersJSON, err := json.Marshal(headersMap)
	if err != nil {
		slog.Warn("failed to marshal response headers", "error", err)
		headersJSON = []byte("{}")
	}

	result := &model.ApiTestResult{
		StatusCode: resp.StatusCode,
		LatencyMs:  latency.Milliseconds(),
		Headers:    string(headersJSON),
		Body:       string(bodyBytes),
	}

	// Check if response body was truncated
	if len(bodyBytes) > maxResponseBodySize {
		result.Body = string(bodyBytes[:maxResponseBodySize])
		result.Error = "response body truncated at 10MB"
	}

	return result, nil
}

// GenerateCurl generates a curl command string from the HTTP request config.
func (e *HTTPExecutor) GenerateCurl(config *model.HTTPRequestConfig) (string, error) {
	if config == nil {
		return "", fmt.Errorf("config is nil")
	}

	method := config.Method
	if method == "" {
		method = http.MethodGet
	}

	var parts []string
	parts = append(parts, "curl")
	parts = append(parts, fmt.Sprintf("-X %s", method))

	// Add auth flags
	if config.Auth != nil {
		switch config.Auth.Type {
		case "bearer":
			parts = append(parts, fmt.Sprintf("--header %s", shellEscape("Authorization: Bearer "+config.Auth.Token)))
		case "basic":
			parts = append(parts, fmt.Sprintf("--user %s:%s", shellEscape(config.Auth.User), shellEscape(config.Auth.Pass)))
		case "apikey":
			if config.Auth.In == "header" {
				parts = append(parts, fmt.Sprintf("--header %s", shellEscape(config.Auth.Key+": "+config.Auth.Value)))
			}
		}
	}

	// Add custom headers
	for _, h := range config.Headers {
		if h.Key != "" {
			parts = append(parts, fmt.Sprintf("-H %s", shellEscape(h.Key+": "+h.Value)))
		}
	}

	// Add body
	if config.Body != nil && config.Body.Content != "" {
		switch config.Body.Type {
		case "json", "raw":
			parts = append(parts, fmt.Sprintf("-d %s", shellEscape(config.Body.Content)))
		case "form-urlencoded":
			var kvs []model.KeyValue
			if err := json.Unmarshal([]byte(config.Body.Content), &kvs); err == nil {
				vals := url.Values{}
				for _, kv := range kvs {
					vals.Set(kv.Key, kv.Value)
				}
				parts = append(parts, fmt.Sprintf("-d %s", shellEscape(vals.Encode())))
			}
		case "binary":
			parts = append(parts, fmt.Sprintf("--data-binary %s", shellEscape(config.Body.Content)))
		}
	}

	// Build URL with params
	reqURL, err := buildURL(config.URL, config.Params)
	if err != nil {
		return "", fmt.Errorf("failed to build URL: %w", err)
	}

	// Add apikey in query param
	if config.Auth != nil && config.Auth.Type == "apikey" && config.Auth.In == "query" {
		u, parseErr := url.Parse(reqURL)
		if parseErr == nil {
			q := u.Query()
			q.Set(config.Auth.Key, config.Auth.Value)
			u.RawQuery = q.Encode()
			reqURL = u.String()
		}
	}

	parts = append(parts, shellEscape(reqURL))

	return strings.Join(parts, " "), nil
}

// ExecuteAssertions runs assertions against the API test result and returns the results.
func (e *HTTPExecutor) ExecuteAssertions(result *model.ApiTestResult, assertions []model.AssertionConfig) []model.AssertionResult {
	results := make([]model.AssertionResult, 0, len(assertions))

	for _, assertion := range assertions {
		ar := model.AssertionResult{
			Assertion: assertion,
			Passed:    false,
		}

		actual, err := extractActualValue(result, assertion)
		if err != nil {
			ar.Actual = ""
			ar.Message = err.Error()
			results = append(results, ar)
			continue
		}
		ar.Actual = actual

		passed, message := compareValues(assertion.Operator, actual, assertion.Expected)
		ar.Passed = passed
		ar.Message = message

		results = append(results, ar)
	}

	return results
}

// applyPreScript applies pre-script variable substitution using text/template.
func applyPreScript(script string, config *model.HTTPRequestConfig) error {
	if script == "" || config == nil {
		return nil
	}

	funcMap := template.FuncMap{
		"timestamp": func() string {
			return strconv.FormatInt(time.Now().Unix(), 10)
		},
		"uuid": func() string {
			return uuid.New().String()
		},
		"randomInt": func() string {
			return strconv.Itoa(int(time.Now().UnixNano() % 100000))
		},
	}

	tmpl, err := template.New("prescript").Funcs(funcMap).Parse(script)
	if err != nil {
		return fmt.Errorf("failed to parse pre-script template: %w", err)
	}

	// Apply template to URL
	if config.URL != "" {
		var buf bytes.Buffer
		if execErr := tmpl.Execute(&buf, nil); execErr != nil {
			return fmt.Errorf("failed to execute pre-script on URL: %w", execErr)
		}
		config.URL = buf.String()
	}

	// Re-parse template for each field since template execution consumes the output
	applyTemplate := func(input string) string {
		if input == "" {
			return input
		}
		t, parseErr := template.New("field").Funcs(funcMap).Parse(input)
		if parseErr != nil {
			slog.Warn("failed to parse template for field", "input", input, "error", parseErr)
			return input
		}
		var buf bytes.Buffer
		if execErr := t.Execute(&buf, nil); execErr != nil {
			slog.Warn("failed to execute template for field", "input", input, "error", execErr)
			return input
		}
		return buf.String()
	}

	// Apply to header values
	for i := range config.Headers {
		config.Headers[i].Value = applyTemplate(config.Headers[i].Value)
	}

	// Apply to body content
	if config.Body != nil {
		config.Body.Content = applyTemplate(config.Body.Content)
	}

	return nil
}

// buildURL constructs the full URL with query parameters.
func buildURL(rawURL string, params []model.KeyValue) (string, error) {
	if rawURL == "" {
		return "", fmt.Errorf("URL is empty")
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	if len(params) > 0 {
		q := u.Query()
		for _, p := range params {
			if p.Key != "" {
				q.Set(p.Key, p.Value)
			}
		}
		u.RawQuery = q.Encode()
	}

	return u.String(), nil
}

// buildBody constructs the request body and determines the Content-Type.
func buildBody(body *model.HTTPBodyConfig) (io.Reader, string, error) {
	if body == nil || body.Type == "none" || body.Type == "" {
		return nil, "", nil
	}

	switch body.Type {
	case "json":
		return strings.NewReader(body.Content), "application/json", nil

	case "form-urlencoded":
		var kvs []model.KeyValue
		if err := json.Unmarshal([]byte(body.Content), &kvs); err != nil {
			return nil, "", fmt.Errorf("failed to parse form-urlencoded body: %w", err)
		}
		vals := url.Values{}
		for _, kv := range kvs {
			vals.Set(kv.Key, kv.Value)
		}
		return strings.NewReader(vals.Encode()), "application/x-www-form-urlencoded", nil

	case "form-multipart":
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		var kvs []model.KeyValue
		if err := json.Unmarshal([]byte(body.Content), &kvs); err != nil {
			return nil, "", fmt.Errorf("failed to parse form-multipart body: %w", err)
		}
		for _, kv := range kvs {
			fieldErr := writer.WriteField(kv.Key, kv.Value)
			if fieldErr != nil {
				return nil, "", fmt.Errorf("failed to write multipart field %s: %w", kv.Key, fieldErr)
			}
		}
		if closeErr := writer.Close(); closeErr != nil {
			return nil, "", fmt.Errorf("failed to close multipart writer: %w", closeErr)
		}
		return &buf, writer.FormDataContentType(), nil

	case "raw":
		return strings.NewReader(body.Content), "", nil

	case "binary":
		decoded, err := base64.StdEncoding.DecodeString(body.Content)
		if err != nil {
			return nil, "", fmt.Errorf("failed to decode binary body: %w", err)
		}
		return bytes.NewReader(decoded), "", nil

	default:
		return nil, "", fmt.Errorf("unsupported body type: %s", body.Type)
	}
}

// applyAuth applies authentication configuration to the request.
func applyAuth(req *http.Request, auth *model.HTTPAuthConfig, reqURL string) error {
	if auth == nil || auth.Type == "" || auth.Type == "none" {
		return nil
	}

	switch auth.Type {
	case "bearer":
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", auth.Token))
	case "basic":
		req.SetBasicAuth(auth.User, auth.Pass)
	case "apikey":
		switch auth.In {
		case "header", "":
			req.Header.Set(auth.Key, auth.Value)
		case "query":
			u, err := url.Parse(reqURL)
			if err != nil {
				return fmt.Errorf("failed to parse URL for apikey query param: %w", err)
			}
			q := u.Query()
			q.Set(auth.Key, auth.Value)
			u.RawQuery = q.Encode()
			req.URL = u
		default:
			return fmt.Errorf("unsupported apikey location: %s", auth.In)
		}
	default:
		return fmt.Errorf("unsupported auth type: %s", auth.Type)
	}

	return nil
}

// extractActualValue extracts the actual value from the result based on the assertion type.
func extractActualValue(result *model.ApiTestResult, assertion model.AssertionConfig) (string, error) {
	switch assertion.Type {
	case "status_code":
		return strconv.Itoa(result.StatusCode), nil

	case "json_path":
		return extractJSONPath(result.Body, assertion.Target)

	case "header":
		return extractHeader(result.Headers, assertion.Target)

	default:
		return "", fmt.Errorf("unsupported assertion type: %s", assertion.Type)
	}
}

// extractJSONPath extracts a value from a JSON body using simple dot-notation path.
func extractJSONPath(body string, path string) (string, error) {
	if body == "" {
		return "", fmt.Errorf("response body is empty")
	}
	if path == "" {
		return "", fmt.Errorf("json_path target is empty")
	}

	var data interface{}
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return "", fmt.Errorf("failed to parse response body as JSON: %w", err)
	}

	parts := strings.Split(path, ".")
	current := data

	for _, part := range parts {
		if part == "" {
			continue
		}

		// Try as map key
		m, ok := current.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("path %q not found: expected object at segment %q", path, part)
		}

		current, ok = m[part]
		if !ok {
			return "", fmt.Errorf("path %q not found: key %q does not exist", path, part)
		}
	}

	return valueToString(current), nil
}

// extractHeader extracts a header value from the JSON-serialized headers.
func extractHeader(headersJSON string, target string) (string, error) {
	if headersJSON == "" {
		return "", fmt.Errorf("headers are empty")
	}

	var headers map[string]interface{}
	if err := json.Unmarshal([]byte(headersJSON), &headers); err != nil {
		return "", fmt.Errorf("failed to parse headers: %w", err)
	}

	// Header keys are case-insensitive; try exact match first, then case-insensitive
	if v, ok := headers[target]; ok {
		return valueToString(v), nil
	}

	lowerTarget := strings.ToLower(target)
	for k, v := range headers {
		if strings.ToLower(k) == lowerTarget {
			return valueToString(v), nil
		}
	}

	return "", fmt.Errorf("header %q not found", target)
}

// valueToString converts an interface{} value to its string representation.
func valueToString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		// Format without trailing zeros for integers
		if val == float64(int64(val)) {
			return strconv.FormatInt(int64(val), 10)
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(val)
	case nil:
		return "null"
	default:
		b, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprintf("%v", val)
		}
		return string(b)
	}
}

// compareValues compares actual and expected values using the specified operator.
func compareValues(operator, actual, expected string) (bool, string) {
	switch operator {
	case "equals":
		if actual == expected {
			return true, ""
		}
		return false, fmt.Sprintf("expected %q, got %q", expected, actual)

	case "not_equals":
		if actual != expected {
			return true, ""
		}
		return false, fmt.Sprintf("expected not %q, but got %q", expected, actual)

	case "contains":
		if strings.Contains(actual, expected) {
			return true, ""
		}
		return false, fmt.Sprintf("expected %q to contain %q", actual, expected)

	case "gt":
		actualNum, err1 := strconv.ParseFloat(actual, 64)
		expectedNum, err2 := strconv.ParseFloat(expected, 64)
		if err1 != nil || err2 != nil {
			return false, fmt.Sprintf("cannot compare non-numeric values: actual=%q, expected=%q", actual, expected)
		}
		if actualNum > expectedNum {
			return true, ""
		}
		return false, fmt.Sprintf("expected %s > %s, but %s <= %s", actual, expected, actual, expected)

	case "lt":
		actualNum, err1 := strconv.ParseFloat(actual, 64)
		expectedNum, err2 := strconv.ParseFloat(expected, 64)
		if err1 != nil || err2 != nil {
			return false, fmt.Sprintf("cannot compare non-numeric values: actual=%q, expected=%q", actual, expected)
		}
		if actualNum < expectedNum {
			return true, ""
		}
		return false, fmt.Sprintf("expected %s < %s, but %s >= %s", actual, expected, actual, expected)

	case "exists":
		if actual != "" {
			return true, ""
		}
		return false, "expected value to exist, but got empty"

	default:
		return false, fmt.Sprintf("unsupported operator: %s", operator)
	}
}

// shellEscape wraps a string in single quotes, escaping any embedded single quotes.
func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// validateURL checks that a URL does not resolve to a private/internal IP address (SSRF protection).
// If api_test.allow_private_network is enabled in system config, private IPs are allowed.
func (e *HTTPExecutor) validateURL(rawURL string) error {
	// Check if private network access is allowed via system config
	if e.configService != nil && e.configService.GetBool("api_test.allow_private_network") {
		return nil
	}

	if rawURL == "" {
		return fmt.Errorf("URL is empty")
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("URL has no host")
	}

	// Resolve hostname to IP addresses
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("failed to resolve host %q: %w", host, err)
	}

	for _, ip := range ips {
		if isPrivateIP(ip) {
			return fmt.Errorf("host %q resolves to private/reserved IP %s, requests to internal addresses are blocked", host, ip)
		}
	}

	return nil
}
