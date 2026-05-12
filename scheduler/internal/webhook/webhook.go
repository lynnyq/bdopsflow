package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
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
	Error       string      `json:"error,omitempty"`
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

	log.Printf("[Webhook] Sent %s event to %s, status: %d", payload.Event, config.URL, resp.StatusCode)
	return nil
}

func (s *Service) SendWithRetry(ctx context.Context, config WebhookConfig, payload WebhookPayload, maxRetries int) error {
	var lastErr error

	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			backoff := time.Duration(i*i) * time.Second
			log.Printf("[Webhook] Retrying after %v (attempt %d/%d)", backoff, i, maxRetries)
			time.Sleep(backoff)
		}

		err := s.Send(ctx, config, payload)
		if err == nil {
			return nil
		}

		lastErr = err
		log.Printf("[Webhook] Attempt %d failed: %v", i+1, err)
	}

	return fmt.Errorf("webhook failed after %d retries: %w", maxRetries, lastErr)
}

func shouldSendForEvent(configuredEvents []string, event string) bool {
	if len(configuredEvents) == 0 {
		return true
	}

	for _, e := range configuredEvents {
		if e == event || e == "*" {
			return true
		}
	}

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
