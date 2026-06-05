package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type WebhookConfig struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Events  []string          `json:"events"`
}

type WebhookPayload struct {
	Event       string      `json:"event"`
	Timestamp   int64       `json:"timestamp"`
	TaskID      int64       `json:"task_id"`
	ExecutionID string      `json:"execution_id"`
	Status      string      `json:"status"`
	Output      string      `json:"output"`
	Error       string      `json:"error"`
	Duration    int64       `json:"duration_ms"`
	Metadata    interface{} `json:"metadata,omitempty"`
}

type Service struct {
	client *http.Client
}

func NewService() *Service {
	return &Service{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (s *Service) Send(ctx context.Context, config WebhookConfig, payload WebhookPayload) error {
	if !shouldSendForEvent(config.Events, payload.Event) {
		return nil
	}

	method := config.Method
	if method == "" {
		method = "POST"
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, config.URL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for key, value := range config.Headers {
		req.Header.Set(key, value)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned non-2xx status: %d", resp.StatusCode)
	}

	slog.Info("[Webhook] Sent event", "event", payload.Event, "url", config.URL, "status", resp.StatusCode)
	return nil
}

func (s *Service) SendFromMap(ctx context.Context, configMap map[string]interface{}, payloadMap map[string]interface{}) error {
	config := WebhookConfig{}
	if url, ok := configMap["url"].(string); ok {
		config.URL = url
	} else {
		return fmt.Errorf("webhook config missing url")
	}

	if method, ok := configMap["method"].(string); ok {
		config.Method = method
	}

	if headers, ok := configMap["headers"].(map[string]interface{}); ok {
		config.Headers = make(map[string]string)
		for k, v := range headers {
			if str, ok := v.(string); ok {
				config.Headers[k] = str
			}
		}
	}

	if events, ok := configMap["events"].([]interface{}); ok {
		for _, e := range events {
			if str, ok := e.(string); ok {
				config.Events = append(config.Events, str)
			}
		}
	}

	payload := WebhookPayload{}
	if event, ok := payloadMap["event"].(string); ok {
		payload.Event = event
	}

	if timestamp, ok := payloadMap["timestamp"].(int64); ok {
		payload.Timestamp = timestamp
	} else if timestamp, ok := payloadMap["timestamp"].(float64); ok {
		payload.Timestamp = int64(timestamp)
	}

	if taskID, ok := payloadMap["task_id"].(int64); ok {
		payload.TaskID = taskID
	} else if taskID, ok := payloadMap["task_id"].(float64); ok {
		payload.TaskID = int64(taskID)
	}

	if executionID, ok := payloadMap["execution_id"].(string); ok {
		payload.ExecutionID = executionID
	}
	if status, ok := payloadMap["status"].(string); ok {
		payload.Status = status
	}
	if output, ok := payloadMap["output"].(string); ok {
		payload.Output = output
	}
	if errMsg, ok := payloadMap["error"].(string); ok {
		payload.Error = errMsg
	} else {
		payload.Error = ""
	}

	if durationMs, ok := payloadMap["duration_ms"].(int64); ok {
		payload.Duration = durationMs
	} else if durationMs, ok := payloadMap["duration_ms"].(float64); ok {
		payload.Duration = int64(durationMs)
	}

	if metadata, ok := payloadMap["metadata"].(map[string]interface{}); ok {
		payload.Metadata = metadata
	}

	return s.Send(ctx, config, payload)
}

func (s *Service) SendWithRetry(ctx context.Context, config WebhookConfig, payload WebhookPayload, maxRetries int) error {
	var lastErr error

	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			backoff := time.Duration(i*i) * time.Second
			slog.Info("[Webhook] Retrying", "backoff", backoff, "attempt", i, "max_retries", maxRetries)
			time.Sleep(backoff)
		}

		err := s.Send(ctx, config, payload)
		if err == nil {
			return nil
		}

		lastErr = err
		slog.Warn("[Webhook] Attempt failed", "attempt", i+1, "error", err)
	}

	return fmt.Errorf("webhook failed after %d retries: %w", maxRetries, lastErr)
}

func shouldSendForEvent(configuredEvents []string, event string) bool {
	slog.Debug("[Webhook] shouldSendForEvent", "configured_events", configuredEvents, "event", event)
	if len(configuredEvents) == 0 {
		return true
	}

	for _, e := range configuredEvents {
		if e == event || e == "*" {
			slog.Debug("[Webhook] event matched", "configured_event", e, "event", event)
			return true
		}
	}

	slog.Debug("[Webhook] no matching event found")
	return false
}

func BuildPayload(event string, taskID int64, executionID, status, output, error string, duration int64) WebhookPayload {
	return WebhookPayload{
		Event:       event,
		Timestamp:   time.Now().Unix(),
		TaskID:      taskID,
		ExecutionID: executionID,
		Status:      status,
		Output:      output,
		Error:       error,
		Duration:    duration,
	}
}
