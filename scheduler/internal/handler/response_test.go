package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
)

func setupTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	return c, w
}

func TestSuccess(t *testing.T) {
	c, w := setupTestContext()

	Success(c, map[string]string{"key": "value"})

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Code != CodeSuccess {
		t.Errorf("expected code %d, got %d", CodeSuccess, resp.Code)
	}
	if resp.Status != "success" {
		t.Errorf("expected status 'success', got %q", resp.Status)
	}
	if resp.Message != "success" {
		t.Errorf("expected message 'success', got %q", resp.Message)
	}
	if resp.Data == nil {
		t.Error("expected data to be non-nil")
	}
}

func TestSuccess_NilData(t *testing.T) {
	c, w := setupTestContext()

	Success(c, nil)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != CodeSuccess {
		t.Errorf("expected code %d, got %d", CodeSuccess, resp.Code)
	}
}

func TestFail(t *testing.T) {
	c, w := setupTestContext()

	Fail(c, CodeBadRequest, "invalid request")

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d, got %d", CodeBadRequest, resp.Code)
	}
	if resp.Status != "error" {
		t.Errorf("expected status 'error', got %q", resp.Status)
	}
	if resp.Message != "invalid request" {
		t.Errorf("expected message 'invalid request', got %q", resp.Message)
	}
	if resp.Data != nil {
		t.Errorf("expected data to be nil, got %v", resp.Data)
	}
}

func TestFailFromError(t *testing.T) {
	c, w := setupTestContext()

	err := service.ErrUserNotFound
	FailFromError(c, err)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != 11001 {
		t.Errorf("expected code 11001, got %d", resp.Code)
	}
	if resp.Status != "error" {
		t.Errorf("expected status 'error', got %q", resp.Status)
	}
	if resp.Message != "user not found" {
		t.Errorf("expected message 'user not found', got %q", resp.Message)
	}
}

func TestFailFromError_AppError(t *testing.T) {
	c, w := setupTestContext()

	appErr := service.NewAppError(99999, "custom error")
	FailFromError(c, appErr)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != 99999 {
		t.Errorf("expected code 99999, got %d", resp.Code)
	}
	if resp.Message != "custom error" {
		t.Errorf("expected message 'custom error', got %q", resp.Message)
	}
}

func TestFailFromError_UnknownError(t *testing.T) {
	c, w := setupTestContext()

	err := errors.New("something unexpected")
	FailFromError(c, err)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != 500 {
		t.Errorf("expected code 500 for unknown error, got %d", resp.Code)
	}
	if resp.Message != "something unexpected" {
		t.Errorf("expected message 'something unexpected', got %q", resp.Message)
	}
}

func TestSuccessPaginated(t *testing.T) {
	c, w := setupTestContext()

	items := []map[string]string{{"name": "item1"}, {"name": "item2"}}
	SuccessPaginated(c, items, 100, 2, 10)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp PaginatedResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != CodeSuccess {
		t.Errorf("expected code %d, got %d", CodeSuccess, resp.Code)
	}
	if resp.Status != "success" {
		t.Errorf("expected status 'success', got %q", resp.Status)
	}
	if resp.Total != 100 {
		t.Errorf("expected total 100, got %d", resp.Total)
	}
	if resp.Page != 2 {
		t.Errorf("expected page 2, got %d", resp.Page)
	}
	if resp.PageSize != 10 {
		t.Errorf("expected page_size 10, got %d", resp.PageSize)
	}
	if resp.Data == nil {
		t.Error("expected data to be non-nil")
	}
}

func TestBadRequest(t *testing.T) {
	c, w := setupTestContext()

	BadRequest(c, "bad input")

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d, got %d", CodeBadRequest, resp.Code)
	}
	if resp.Status != "error" {
		t.Errorf("expected status 'error', got %q", resp.Status)
	}
	if resp.Message != "bad input" {
		t.Errorf("expected message 'bad input', got %q", resp.Message)
	}
}

func TestUnauthorized(t *testing.T) {
	c, w := setupTestContext()

	Unauthorized(c, "not authorized")

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != CodeUnauthorized {
		t.Errorf("expected code %d, got %d", CodeUnauthorized, resp.Code)
	}
	if resp.Status != "error" {
		t.Errorf("expected status 'error', got %q", resp.Status)
	}
	if resp.Message != "not authorized" {
		t.Errorf("expected message 'not authorized', got %q", resp.Message)
	}
}

func TestForbidden(t *testing.T) {
	c, w := setupTestContext()

	Forbidden(c, "access denied")

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != CodeForbidden {
		t.Errorf("expected code %d, got %d", CodeForbidden, resp.Code)
	}
	if resp.Status != "error" {
		t.Errorf("expected status 'error', got %q", resp.Status)
	}
	if resp.Message != "access denied" {
		t.Errorf("expected message 'access denied', got %q", resp.Message)
	}
}

func TestNotFound(t *testing.T) {
	c, w := setupTestContext()

	NotFound(c, "resource not found")

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != CodeNotFound {
		t.Errorf("expected code %d, got %d", CodeNotFound, resp.Code)
	}
	if resp.Status != "error" {
		t.Errorf("expected status 'error', got %q", resp.Status)
	}
	if resp.Message != "resource not found" {
		t.Errorf("expected message 'resource not found', got %q", resp.Message)
	}
}

func TestInternalServerError(t *testing.T) {
	c, w := setupTestContext()

	InternalServerError(c, "internal failure")

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != CodeInternalError {
		t.Errorf("expected code %d, got %d", CodeInternalError, resp.Code)
	}
	if resp.Status != "error" {
		t.Errorf("expected status 'error', got %q", resp.Status)
	}
	if resp.Message != "internal failure" {
		t.Errorf("expected message 'internal failure', got %q", resp.Message)
	}
}

func TestCreated(t *testing.T) {
	c, w := setupTestContext()

	Created(c, map[string]int64{"id": 42})

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != CodeSuccess {
		t.Errorf("expected code %d, got %d", CodeSuccess, resp.Code)
	}
	if resp.Status != "success" {
		t.Errorf("expected status 'success', got %q", resp.Status)
	}
	if resp.Message != "created" {
		t.Errorf("expected message 'created', got %q", resp.Message)
	}
	if resp.Data == nil {
		t.Error("expected data to be non-nil")
	}
}

func TestError(t *testing.T) {
	c, w := setupTestContext()

	Error(c, http.StatusServiceUnavailable, "service unavailable")

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", w.Code)
	}

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != http.StatusServiceUnavailable {
		t.Errorf("expected code 503, got %d", resp.Code)
	}
	if resp.Status != "error" {
		t.Errorf("expected status 'error', got %q", resp.Status)
	}
	if resp.Message != "service unavailable" {
		t.Errorf("expected message 'service unavailable', got %q", resp.Message)
	}
}

func TestSuccessWithMessage(t *testing.T) {
	c, w := setupTestContext()

	SuccessWithMessage(c, "operation completed", map[string]string{"result": "ok"})

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != CodeSuccess {
		t.Errorf("expected code %d, got %d", CodeSuccess, resp.Code)
	}
	if resp.Status != "success" {
		t.Errorf("expected status 'success', got %q", resp.Status)
	}
	if resp.Message != "operation completed" {
		t.Errorf("expected message 'operation completed', got %q", resp.Message)
	}
}

func TestFailWithData(t *testing.T) {
	c, w := setupTestContext()

	FailWithData(c, CodeBadRequest, "validation failed", map[string]string{"field": "name"})

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d, got %d", CodeBadRequest, resp.Code)
	}
	if resp.Status != "error" {
		t.Errorf("expected status 'error', got %q", resp.Status)
	}
	if resp.Message != "validation failed" {
		t.Errorf("expected message 'validation failed', got %q", resp.Message)
	}
	if resp.Data == nil {
		t.Error("expected data to be non-nil for FailWithData")
	}
}

func TestErrorWithData(t *testing.T) {
	c, w := setupTestContext()

	ErrorWithData(c, http.StatusBadGateway, "gateway error", map[string]string{"detail": "upstream timeout"})

	if w.Code != http.StatusBadGateway {
		t.Errorf("expected status 502, got %d", w.Code)
	}

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != http.StatusBadGateway {
		t.Errorf("expected code 502, got %d", resp.Code)
	}
	if resp.Status != "error" {
		t.Errorf("expected status 'error', got %q", resp.Status)
	}
	if resp.Message != "gateway error" {
		t.Errorf("expected message 'gateway error', got %q", resp.Message)
	}
	if resp.Data == nil {
		t.Error("expected data to be non-nil for ErrorWithData")
	}
}

func TestFailFromError_TaskNotFound(t *testing.T) {
	c, w := setupTestContext()

	FailFromError(c, service.ErrTaskNotFound)

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != 10003 {
		t.Errorf("expected code 10003, got %d", resp.Code)
	}
	if resp.Message != "task not found" {
		t.Errorf("expected message 'task not found', got %q", resp.Message)
	}
}

func TestFailFromError_RoleNotFound(t *testing.T) {
	c, w := setupTestContext()

	FailFromError(c, service.ErrRoleNotFound)

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != 12001 {
		t.Errorf("expected code 12001, got %d", resp.Code)
	}
	if resp.Message != "role not found" {
		t.Errorf("expected message 'role not found', got %q", resp.Message)
	}
}

func TestFailFromError_DomainNotFound(t *testing.T) {
	c, w := setupTestContext()

	FailFromError(c, service.ErrDomainNotFound)

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != 13001 {
		t.Errorf("expected code 13001, got %d", resp.Code)
	}
	if resp.Message != "domain not found" {
		t.Errorf("expected message 'domain not found', got %q", resp.Message)
	}
}

func TestFailFromError_PermissionDenied(t *testing.T) {
	c, w := setupTestContext()

	FailFromError(c, service.ErrPermissionDenied)

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != 14001 {
		t.Errorf("expected code 14001, got %d", resp.Code)
	}
	if resp.Message != "permission denied" {
		t.Errorf("expected message 'permission denied', got %q", resp.Message)
	}
}

func TestFailFromError_WorkflowNotFound(t *testing.T) {
	c, w := setupTestContext()

	FailFromError(c, service.ErrWorkflowNotFound)

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != 15001 {
		t.Errorf("expected code 15001, got %d", resp.Code)
	}
	if resp.Message != "workflow not found" {
		t.Errorf("expected message 'workflow not found', got %q", resp.Message)
	}
}

func TestFailFromError_WrapError(t *testing.T) {
	c, w := setupTestContext()

	innerErr := errors.New("database connection lost")
	appErr := service.WrapError(5001, innerErr)
	FailFromError(c, appErr)

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != 5001 {
		t.Errorf("expected code 5001, got %d", resp.Code)
	}
	if resp.Message != "database connection lost" {
		t.Errorf("expected message 'database connection lost', got %q", resp.Message)
	}
}

func TestSuccessPaginated_ZeroValues(t *testing.T) {
	c, w := setupTestContext()

	SuccessPaginated(c, []string{}, 0, 1, 10)

	var resp PaginatedResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Total != 0 {
		t.Errorf("expected total 0, got %d", resp.Total)
	}
	if resp.Page != 1 {
		t.Errorf("expected page 1, got %d", resp.Page)
	}
	if resp.PageSize != 10 {
		t.Errorf("expected page_size 10, got %d", resp.PageSize)
	}
}

func TestSuccessPaginated_NilData(t *testing.T) {
	c, w := setupTestContext()

	SuccessPaginated(c, nil, 0, 0, 0)

	var resp PaginatedResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != CodeSuccess {
		t.Errorf("expected code %d, got %d", CodeSuccess, resp.Code)
	}
	if resp.Status != "success" {
		t.Errorf("expected status 'success', got %q", resp.Status)
	}
	if resp.Total != 0 {
		t.Errorf("expected total 0, got %d", resp.Total)
	}
}

func TestSuccessPaginated_LargeValues(t *testing.T) {
	c, w := setupTestContext()

	SuccessPaginated(c, []int{1, 2, 3}, 99999, 500, 20)

	var resp PaginatedResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Total != 99999 {
		t.Errorf("expected total 99999, got %d", resp.Total)
	}
	if resp.Page != 500 {
		t.Errorf("expected page 500, got %d", resp.Page)
	}
	if resp.PageSize != 20 {
		t.Errorf("expected page_size 20, got %d", resp.PageSize)
	}
}

func TestResponse_JSONStructure(t *testing.T) {
	c, w := setupTestContext()

	Success(c, map[string]interface{}{"id": float64(1), "name": "test"})

	var raw map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if _, ok := raw["code"]; !ok {
		t.Error("response JSON missing 'code' field")
	}
	if _, ok := raw["status"]; !ok {
		t.Error("response JSON missing 'status' field")
	}
	if _, ok := raw["message"]; !ok {
		t.Error("response JSON missing 'message' field")
	}
	if _, ok := raw["data"]; !ok {
		t.Error("response JSON missing 'data' field")
	}
}

func TestPaginatedResponse_JSONStructure(t *testing.T) {
	c, w := setupTestContext()

	SuccessPaginated(c, []string{}, 50, 3, 10)

	var raw map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	for _, field := range []string{"code", "status", "message", "data", "total", "page", "page_size"} {
		if _, ok := raw[field]; !ok {
			t.Errorf("paginated response JSON missing '%s' field", field)
		}
	}
}
