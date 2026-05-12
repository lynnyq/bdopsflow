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
