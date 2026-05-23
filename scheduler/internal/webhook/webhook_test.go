package webhook

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWebhookConfig_BuildPayload(t *testing.T) {
	payload := BuildPayload(
		"task.completed",
		123,
		"exec-456",
		"success",
		"output data",
		"",
		1000,
	)

	if payload.Event != "task.completed" {
		t.Errorf("Expected event 'task.completed', got %s", payload.Event)
	}

	if payload.TaskID != 123 {
		t.Errorf("Expected task_id 123, got %d", payload.TaskID)
	}

	if payload.ExecutionID != "exec-456" {
		t.Errorf("Expected execution_id 'exec-456', got %s", payload.ExecutionID)
	}

	if payload.Status != "success" {
		t.Errorf("Expected status 'success', got %s", payload.Status)
	}

	if payload.Duration != 1000 {
		t.Errorf("Expected duration 1000, got %d", payload.Duration)
	}
}

func TestShouldSendForEvent(t *testing.T) {
	tests := []struct {
		name            string
		configuredEvents []string
		event           string
		expected        bool
	}{
		{
			name:            "empty configured events",
			configuredEvents: []string{},
			event:           "task.completed",
			expected:        true,
		},
		{
			name:            "wildcard event",
			configuredEvents: []string{"*"},
			event:           "task.completed",
			expected:        true,
		},
		{
			name:            "matching event",
			configuredEvents: []string{"task.completed", "task.failed"},
			event:           "task.completed",
			expected:        true,
		},
		{
			name:            "non-matching event",
			configuredEvents: []string{"task.failed"},
			event:           "task.completed",
			expected:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldSendForEvent(tt.configuredEvents, tt.event)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestWebhookService_Send(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json")
		}

		var payload WebhookPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("Failed to decode payload: %v", err)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	svc := NewService()
	
	config := WebhookConfig{
		URL:     server.URL,
		Method:  "POST",
		Headers: map[string]string{"X-Custom": "value"},
		Events:  []string{"*"},
	}

	payload := BuildPayload(
		"task.completed",
		1,
		"exec-1",
		"success",
		"output",
		"",
		1000,
	)

	err := svc.Send(context.Background(), config, payload)
	if err != nil {
		t.Errorf("Send failed: %v", err)
	}
}

func TestWebhookService_SendWithRetry(t *testing.T) {
	attempt := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		if attempt < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	svc := NewService()

	config := WebhookConfig{
		URL:    server.URL,
		Method: "POST",
		Events: []string{"*"},
	}

	payload := WebhookPayload{
		Event:     "test",
		Timestamp: time.Now().Unix(),
	}

	err := svc.SendWithRetry(context.Background(), config, payload, 3)
	if err != nil {
		t.Errorf("SendWithRetry failed: %v", err)
	}

	if attempt != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempt)
	}
}

func TestWebhookService_SendWithRetry_AllFailed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	svc := NewService()

	config := WebhookConfig{
		URL:    server.URL,
		Method: "POST",
		Events: []string{"*"},
	}

	payload := WebhookPayload{
		Event:     "test",
		Timestamp: time.Now().Unix(),
	}

	err := svc.SendWithRetry(context.Background(), config, payload, 2)
	if err == nil {
		t.Errorf("Expected error, got nil")
	}
}

func TestWebhookService_Send_NonMatchingEvent(t *testing.T) {
	svc := NewService()

	config := WebhookConfig{
		URL:    "http://example.com/webhook",
		Method: "POST",
		Events: []string{"task.failed"},
	}

	payload := BuildPayload("task.completed", 1, "exec-1", "success", "", "", 0)

	err := svc.Send(context.Background(), config, payload)
	if err != nil {
		t.Errorf("Send should not fail for non-matching event, got: %v", err)
	}
}

func TestWebhookService_Send_DefaultMethod(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST method as default, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	svc := NewService()

	config := WebhookConfig{
		URL:    server.URL,
		Method: "",
		Events: []string{"*"},
	}

	payload := BuildPayload("test", 0, "", "", "", "", 0)

	err := svc.Send(context.Background(), config, payload)
	if err != nil {
		t.Errorf("Send failed: %v", err)
	}
}

// TestWebhookEventFiltering 测试 webhook 推送时机的灵活配置
func TestWebhookEventFiltering(t *testing.T) {
	tests := []struct {
		name            string
		configuredEvents []string
		testCases       []struct {
			event           string
			shouldSend      bool
			description     string
		}
	}{
		{
			name:            "仅任务成功时推送",
			configuredEvents: []string{"success"},
			testCases: []struct {
				event           string
				shouldSend      bool
				description     string
			}{
				{event: "success", shouldSend: true, description: "任务成功应该推送"},
				{event: "failed", shouldSend: false, description: "任务失败不应该推送"},
			},
		},
		{
			name:            "仅任务失败时推送",
			configuredEvents: []string{"failed"},
			testCases: []struct {
				event           string
				shouldSend      bool
				description     string
			}{
				{event: "success", shouldSend: false, description: "任务成功不应该推送"},
				{event: "failed", shouldSend: true, description: "任务失败应该推送"},
			},
		},
		{
			name:            "每次执行都推送（通配符）",
			configuredEvents: []string{"*"},
			testCases: []struct {
				event           string
				shouldSend      bool
				description     string
			}{
				{event: "success", shouldSend: true, description: "任务成功应该推送"},
				{event: "failed", shouldSend: true, description: "任务失败应该推送"},
			},
		},
		{
			name:            "任务成功和失败都推送",
			configuredEvents: []string{"success", "failed"},
			testCases: []struct {
				event           string
				shouldSend      bool
				description     string
			}{
				{event: "success", shouldSend: true, description: "任务成功应该推送"},
				{event: "failed", shouldSend: true, description: "任务失败应该推送"},
			},
		},
		{
			name:            "空配置（默认全部推送）",
			configuredEvents: []string{},
			testCases: []struct {
				event           string
				shouldSend      bool
				description     string
			}{
				{event: "success", shouldSend: true, description: "默认应该推送所有事件"},
				{event: "failed", shouldSend: true, description: "默认应该推送所有事件"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, tc := range tt.testCases {
				t.Run(tc.description, func(t *testing.T) {
					result := shouldSendForEvent(tt.configuredEvents, tc.event)
					if result != tc.shouldSend {
						t.Errorf("%s: 配置 %v, 事件 %s, 期望发送 %v, 实际 %v",
							tc.description, tt.configuredEvents, tc.event, tc.shouldSend, result)
					}
				})
			}
		})
	}
}

// TestWebhookPayloadWithDifferentEvents 测试不同事件类型的 payload 构建
func TestWebhookPayloadWithDifferentEvents(t *testing.T) {
	testCases := []struct {
		event      string
		status     string
		output     string
		errorMsg   string
		description string
	}{
		{
			event:      "success",
			status:     "success",
			output:     "Task completed successfully",
			errorMsg:   "",
			description: "任务成功场景",
		},
		{
			event:      "failed",
			status:     "failed",
			output:     "",
			errorMsg:   "Task execution failed: timeout",
			description: "任务失败场景",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			payload := BuildPayload(
				tc.event,
				123,
				"exec-789",
				tc.status,
				tc.output,
				tc.errorMsg,
				5000,
			)

			if payload.Event != tc.event {
				t.Errorf("Expected event %s, got %s", tc.event, payload.Event)
			}

			if payload.Status != tc.status {
				t.Errorf("Expected status %s, got %s", tc.status, payload.Status)
			}

			if payload.Output != tc.output {
				t.Errorf("Expected output %s, got %s", tc.output, payload.Output)
			}

			if payload.Error != tc.errorMsg {
				t.Errorf("Expected error %s, got %s", tc.errorMsg, payload.Error)
			}

			if payload.TaskID != 123 {
				t.Errorf("Expected task_id 123, got %d", payload.TaskID)
			}

			if payload.ExecutionID != "exec-789" {
				t.Errorf("Expected execution_id exec-789, got %s", payload.ExecutionID)
			}

			if payload.Duration != 5000 {
				t.Errorf("Expected duration 5000, got %d", payload.Duration)
			}
		})
	}
}

// TestWebhookServiceSendWithSpecificEvents 测试在特定事件配置下发送 webhook
func TestWebhookServiceSendWithSpecificEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload WebhookPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("Failed to decode payload: %v", err)
		}

		if payload.Event != "success" {
			t.Errorf("Expected event 'success', got %s", payload.Event)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	svc := NewService()

	config := WebhookConfig{
		URL:     server.URL,
		Method:  "POST",
		Headers: map[string]string{"Content-Type": "application/json"},
		Events:  []string{"success"},
	}

	successPayload := BuildPayload("success", 1, "exec-1", "success", "Output", "", 1000)
	err := svc.Send(context.Background(), config, successPayload)
	if err != nil {
		t.Errorf("Send failed for success event: %v", err)
	}

	failedPayload := BuildPayload("failed", 1, "exec-2", "failed", "", "Error", 500)
	err = svc.Send(context.Background(), config, failedPayload)
	if err != nil {
		t.Errorf("Send should not return error for non-matching event: %v", err)
	}
}

// TestWebhookPayloadSerialization 测试 webhook payload 的 JSON 序列化
func TestWebhookPayloadSerialization(t *testing.T) {
	payload := WebhookPayload{
		Event:       "success",
		Timestamp:   1234567890,
		TaskID:      42,
		ExecutionID: "exec-test-123",
		Status:      "success",
		Output:      "Test output",
		Error:       "",
		Duration:    1500,
		Metadata:    map[string]string{"key": "value"},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		t.Errorf("Failed to marshal payload: %v", err)
	}

	var decoded WebhookPayload
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Errorf("Failed to unmarshal payload: %v", err)
	}

	if decoded.Event != payload.Event {
		t.Errorf("Event mismatch: expected %s, got %s", payload.Event, decoded.Event)
	}
	if decoded.TaskID != payload.TaskID {
		t.Errorf("TaskID mismatch: expected %d, got %d", payload.TaskID, decoded.TaskID)
	}
	if decoded.ExecutionID != payload.ExecutionID {
		t.Errorf("ExecutionID mismatch: expected %s, got %s", payload.ExecutionID, decoded.ExecutionID)
	}
	if decoded.Status != payload.Status {
		t.Errorf("Status mismatch: expected %s, got %s", payload.Status, decoded.Status)
	}
	if decoded.Duration != payload.Duration {
		t.Errorf("Duration mismatch: expected %d, got %d", payload.Duration, decoded.Duration)
	}
}

// TestWebhookFlexiblePushScenarios 测试灵活的推送时机场景
func TestWebhookFlexiblePushScenarios(t *testing.T) {
	tests := []struct {
		name            string
		webhookConfig   WebhookConfig
		testEvents      []string
		expectedSends   []int
		description     string
	}{
		{
			name: "只订阅任务成功事件",
			webhookConfig: WebhookConfig{
				URL:    "http://example.com/webhook",
				Method: "POST",
				Events: []string{"success"},
			},
			testEvents:    []string{"success", "failed", "success"},
			expectedSends: []int{1, 1, 2},
			description:   "应该只发送2次成功的推送",
		},
		{
			name: "只订阅任务失败事件",
			webhookConfig: WebhookConfig{
				URL:    "http://example.com/webhook",
				Method: "POST",
				Events: []string{"failed"},
			},
			testEvents:    []string{"failed", "success", "failed"},
			expectedSends: []int{1, 1, 2},
			description:   "应该只发送2次失败的推送",
		},
		{
			name: "订阅所有事件（通配符）",
			webhookConfig: WebhookConfig{
				URL:    "http://example.com/webhook",
				Method: "POST",
				Events: []string{"*"},
			},
			testEvents:    []string{"success", "failed", "success", "failed"},
			expectedSends: []int{1, 2, 3, 4},
			description:   "应该发送所有4次推送",
		},
		{
			name: "同时订阅成功和失败事件",
			webhookConfig: WebhookConfig{
				URL:    "http://example.com/webhook",
				Method: "POST",
				Events: []string{"success", "failed"},
			},
			testEvents:    []string{"success", "failed", "success"},
			expectedSends: []int{1, 2, 3},
			description:   "应该发送所有3次推送",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receivedCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedCount++
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			tt.webhookConfig.URL = server.URL
			svc := NewService()

			for i, event := range tt.testEvents {
				payload := BuildPayload(event, 1, "exec-1", event, "", "", 0)
				svc.Send(context.Background(), tt.webhookConfig, payload)
				
				if receivedCount != tt.expectedSends[i] {
					t.Errorf("%s: 第 %d 次事件后，期望累计发送 %d 次，实际累计发送 %d 次",
						tt.description, i+1, tt.expectedSends[i], receivedCount)
				}
			}
		})
	}
}

func TestSendFromMap(t *testing.T) {
	var receivedPayload map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json")
		}

		if err := json.NewDecoder(r.Body).Decode(&receivedPayload); err != nil {
			t.Errorf("Failed to decode payload: %v", err)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	configMap := map[string]interface{}{
		"url":     server.URL,
		"method":  "POST",
		"headers": map[string]interface{}{"X-Custom": "test"},
		"events":  []interface{}{"success", "failed"},
	}

	payloadMap := map[string]interface{}{
		"event":         "success",
		"timestamp":     int64(time.Now().Unix()),
		"task_id":       int64(123),
		"execution_id":  "exec-456",
		"status":        "success",
		"output":        "test output",
		"error":         "",
		"duration_ms":   int64(1000),
		"metadata":      map[string]interface{}{"task_name": "test"},
	}

	svc := NewService()
	err := svc.SendFromMap(context.Background(), configMap, payloadMap)
	if err != nil {
		t.Errorf("SendFromMap failed: %v", err)
	}

	if receivedPayload == nil {
		t.Fatal("No payload received")
	}

	if receivedPayload["event"] != "success" {
		t.Errorf("Expected event 'success', got %v", receivedPayload["event"])
	}

	if receivedPayload["task_id"] != float64(123) {
		t.Errorf("Expected task_id 123, got %v", receivedPayload["task_id"])
	}

	if receivedPayload["execution_id"] != "exec-456" {
		t.Errorf("Expected execution_id 'exec-456', got %v", receivedPayload["execution_id"])
	}

	if receivedPayload["error"] != "" {
		t.Errorf("Expected error field to be empty for success message, got %v", receivedPayload["error"])
	}

	if receivedPayload["timestamp"] == nil || receivedPayload["timestamp"] == float64(0) {
		t.Errorf("Expected timestamp to be set, got %v", receivedPayload["timestamp"])
	}

	if receivedPayload["duration_ms"] == nil || receivedPayload["duration_ms"] == float64(0) {
		t.Errorf("Expected duration_ms to be set, got %v", receivedPayload["duration_ms"])
	}
}
