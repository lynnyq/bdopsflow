package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// === truncateString 测试 ===

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"empty string", "", 10, ""},
		{"within limit", "hello", 10, "hello"},
		{"exactly at limit", "hello", 5, "hello"},
		{"exceeds limit", "hello world", 5, "hello..."},
		{"limit is zero", "hello", 0, "..."},
		{"single char exceeds", "ab", 1, "a..."},
		// 注意：truncateString 使用字节切片（len/s[:]），不是 rune 切片，
		// 因此多字节字符（如中文）会按字节截断。这里测试 ASCII 行为即可。
		{"ascii long string", "abcdefghij", 4, "abcd..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateString(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateString(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

// === buildMarkdownMessage 测试 ===

func TestBuildMarkdownMessage_Success(t *testing.T) {
	h := &WeComHandler{}
	eventData := BdopsFlowEvent{
		DeliveryID: "delivery-1",
		Event:      "task.executed",
		Execution: BdopsFlowExecution{
			DurationMs: 1500,
			Error:      "",
			ID:         "exec-1",
			Output:     "task completed",
			Status:     "success",
		},
		Task: BdopsFlowTask{
			ID:   1,
			Name: "test-task",
			Type: "shell",
		},
		Timestamp: 1234567890,
	}

	msg := h.buildMarkdownMessage(eventData)

	if msg == "" {
		t.Error("expected non-empty markdown message")
	}
	if !contains(msg, "test-task") {
		t.Errorf("expected message to contain task name 'test-task', got: %s", msg)
	}
	if !contains(msg, "✅") {
		t.Errorf("expected message to contain success icon ✅, got: %s", msg)
	}
	if !contains(msg, "1.50") {
		t.Errorf("expected message to contain duration 1.50, got: %s", msg)
	}
}

func TestBuildMarkdownMessage_Failed(t *testing.T) {
	h := &WeComHandler{}
	eventData := BdopsFlowEvent{
		Execution: BdopsFlowExecution{
			DurationMs: 2000,
			Error:      "command not found",
			ID:         "exec-2",
			Output:     "",
			Status:     "failed",
		},
		Task: BdopsFlowTask{
			ID:   2,
			Name: "failed-task",
		},
	}

	msg := h.buildMarkdownMessage(eventData)

	if !contains(msg, "failed-task") {
		t.Errorf("expected message to contain task name 'failed-task', got: %s", msg)
	}
	if !contains(msg, "❌") {
		t.Errorf("expected message to contain failure icon ❌, got: %s", msg)
	}
	if !contains(msg, "command not found") {
		t.Errorf("expected message to contain error message, got: %s", msg)
	}
}

func TestBuildMarkdownMessage_DefaultStatus(t *testing.T) {
	h := &WeComHandler{}
	eventData := BdopsFlowEvent{
		Execution: BdopsFlowExecution{
			DurationMs: 500,
			Status:     "running",
			Output:     "in progress",
		},
		Task: BdopsFlowTask{
			ID:   3,
			Name: "running-task",
		},
	}

	msg := h.buildMarkdownMessage(eventData)

	if !contains(msg, "running-task") {
		t.Errorf("expected message to contain task name, got: %s", msg)
	}
	if !contains(msg, "⚙️") {
		t.Errorf("expected message to contain default icon ⚙️, got: %s", msg)
	}
}

func TestBuildMarkdownMessage_FailedNoError(t *testing.T) {
	h := &WeComHandler{}
	eventData := BdopsFlowEvent{
		Execution: BdopsFlowExecution{
			DurationMs: 100,
			Error:      "",
			Output:     "some output",
			Status:     "failed",
		},
		Task: BdopsFlowTask{
			ID:   4,
			Name: "failed-no-err",
		},
	}

	msg := h.buildMarkdownMessage(eventData)

	// failed status with no error → errorDisplay should be "无"
	if !contains(msg, "无") {
		t.Errorf("expected message to contain '无' for no error, got: %s", msg)
	}
}

func TestBuildMarkdownMessage_LongOutputTruncated(t *testing.T) {
	h := &WeComHandler{}
	longOutput := ""
	for i := 0; i < 2000; i++ {
		longOutput += "a"
	}

	eventData := BdopsFlowEvent{
		Execution: BdopsFlowExecution{
			DurationMs: 100,
			Output:     longOutput,
			Status:     "success",
		},
		Task: BdopsFlowTask{
			ID:   5,
			Name: "long-output-task",
		},
	}

	msg := h.buildMarkdownMessage(eventData)

	// 输出被截断为 1000 字符 + "..."
	if !contains(msg, "...") {
		t.Errorf("expected message to contain '...' for truncated output, got message of length %d", len(msg))
	}
}

// === WeComHandler.SendWeComMessage 测试 ===

func TestWeComHandler_SendWeComMessage_EmptyGroupID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	h := &WeComHandler{}
	r.POST("/api/wecom/:wx_group_id", h.SendWeComMessage)

	body, _ := json.Marshal(BdopsFlowEvent{})
	req, _ := http.NewRequest("POST", "/api/wecom/", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// 空 wx_group_id → 路由匹配失败返回 404
	if w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest {
		t.Errorf("expected 404 or 400, got %d", w.Code)
	}
}

func TestWeComHandler_SendWeComMessage_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	h := &WeComHandler{}
	r.POST("/api/wecom/:wx_group_id", h.SendWeComMessage)

	req, _ := http.NewRequest("POST", "/api/wecom/group1", bytes.NewBuffer([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", w.Code)
	}
}

// === WeComHandler.SendAppMsg 测试 ===

func TestWeComHandler_SendAppMsg_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	h := &WeComHandler{}
	r.POST("/api/wecom/app/msg", h.SendAppMsg)

	req, _ := http.NewRequest("POST", "/api/wecom/app/msg", bytes.NewBuffer([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", w.Code)
	}
}

// === WeComHandler.SendRobotImageMsg 测试 ===

func TestWeComHandler_SendRobotImageMsg_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	h := &WeComHandler{}
	r.POST("/api/wecom/robot/image", h.SendRobotImageMsg)

	req, _ := http.NewRequest("POST", "/api/wecom/robot/image", bytes.NewBuffer([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", w.Code)
	}
}

func TestWeComHandler_SendRobotImageMsg_InvalidBase64(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	h := &WeComHandler{}
	r.POST("/api/wecom/robot/image", h.SendRobotImageMsg)

	body, _ := json.Marshal(map[string]string{
		"group_id":     "group1",
		"image_base64": "!!!invalid-base64!!!",
	})
	req, _ := http.NewRequest("POST", "/api/wecom/robot/image", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid base64, got %d", w.Code)
	}
}

// === WeComHandler.SendRobotTextPeopleMsg 测试 ===

func TestWeComHandler_SendRobotTextPeopleMsg_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	h := &WeComHandler{}
	r.POST("/api/wecom/robot/text-people", h.SendRobotTextPeopleMsg)

	req, _ := http.NewRequest("POST", "/api/wecom/robot/text-people", bytes.NewBuffer([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", w.Code)
	}
}

// === WeComHandler.SendRobotMarkdownMsg 测试 ===

func TestWeComHandler_SendRobotMarkdownMsg_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	h := &WeComHandler{}
	r.POST("/api/wecom/robot/markdown", h.SendRobotMarkdownMsg)

	req, _ := http.NewRequest("POST", "/api/wecom/robot/markdown", bytes.NewBuffer([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", w.Code)
	}
}

// === WeComHandler.SendChatMarkdownMsg 测试 ===

func TestWeComHandler_SendChatMarkdownMsg_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	h := &WeComHandler{}
	r.POST("/api/wecom/chat/markdown", h.SendChatMarkdownMsg)

	req, _ := http.NewRequest("POST", "/api/wecom/chat/markdown", bytes.NewBuffer([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", w.Code)
	}
}

// === WeComHandler.CreateChatGroup 测试 ===

func TestWeComHandler_CreateChatGroup_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	h := &WeComHandler{}
	r.POST("/api/wecom/chat/group", h.CreateChatGroup)

	req, _ := http.NewRequest("POST", "/api/wecom/chat/group", bytes.NewBuffer([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", w.Code)
	}
}

// === WeComHandler.GetChatGroupInfo 测试 ===

func TestWeComHandler_GetChatGroupInfo_EmptyChatID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	h := &WeComHandler{}
	r.GET("/api/wecom/chat/:chat_id", h.GetChatGroupInfo)

	req, _ := http.NewRequest("GET", "/api/wecom/chat/", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// 空 chat_id → 路由不匹配返回 404
	if w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest {
		t.Errorf("expected 404 or 400, got %d", w.Code)
	}
}

// === WeComHandler.UpdateChatGroup 测试 ===

func TestWeComHandler_UpdateChatGroup_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	h := &WeComHandler{}
	r.PUT("/api/wecom/chat/group", h.UpdateChatGroup)

	req, _ := http.NewRequest("PUT", "/api/wecom/chat/group", bytes.NewBuffer([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", w.Code)
	}
}

func TestWeComHandler_UpdateChatGroup_EmptyChatID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	h := &WeComHandler{}
	r.PUT("/api/wecom/chat/group", h.UpdateChatGroup)

	body, _ := json.Marshal(map[string]interface{}{
		"chat_id": "",
	})
	req, _ := http.NewRequest("PUT", "/api/wecom/chat/group", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty chat_id, got %d", w.Code)
	}
}
