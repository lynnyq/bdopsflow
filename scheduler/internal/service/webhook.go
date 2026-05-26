package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/internal/webhook"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
	rqlite "github.com/rqlite/gorqlite"
)

type WebhookService struct {
	db database.DB
}

func NewWebhookService(db database.DB) *WebhookService {
	return &WebhookService{db: db}
}

func (s *WebhookService) Create(ctx context.Context, webhook *model.Webhook) (*model.Webhook, error) {
	query := `
		INSERT INTO bdopsflow_webhooks (name, url, method, headers, secret, domain_id, is_enabled, description, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	now := time.Now().Format(DateTimeFormat)
	stmt := rqlite.ParameterizedStatement{
		Query: query,
		Arguments: []interface{}{
			webhook.Name,
			webhook.URL,
			webhook.Method,
			webhook.Headers,
			webhook.Secret,
			webhook.DomainID,
			webhook.IsEnabled,
			webhook.Description,
			webhook.CreatedBy,
			now,
			now,
		},
	}

	result, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		slog.Error("failed to create webhook", "error", err, "name", webhook.Name)
		return nil, err
	}
	if result.Err != nil {
		slog.Error("webhook create result error", "error", result.Err, "name", webhook.Name)
		return nil, result.Err
	}

	webhook.ID = result.LastInsertID
	webhook.CreatedAt, _ = time.Parse(DateTimeFormat, now)
	webhook.UpdatedAt = webhook.CreatedAt
	return webhook, nil
}

func (s *WebhookService) Update(ctx context.Context, id int64, webhook *model.Webhook) error {
	query := `
		UPDATE bdopsflow_webhooks SET name = ?, url = ?, method = ?, headers = ?, secret = ?, domain_id = ?, is_enabled = ?, description = ?, updated_at = ?
		WHERE id = ?
	`
	now := time.Now().Format(DateTimeFormat)
	stmt := rqlite.ParameterizedStatement{
		Query: query,
		Arguments: []interface{}{
			webhook.Name,
			webhook.URL,
			webhook.Method,
			webhook.Headers,
			webhook.Secret,
			webhook.DomainID,
			webhook.IsEnabled,
			webhook.Description,
			now,
			id,
		},
	}

	result, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		slog.Error("failed to update webhook", "error", err, "id", id)
		return err
	}
	if result.Err != nil {
		slog.Error("webhook update result error", "error", result.Err, "id", id)
		return result.Err
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("webhook not found")
	}

	return nil
}

func (s *WebhookService) Delete(ctx context.Context, id int64) error {
	clearQuery := `UPDATE bdopsflow_tasks SET webhook_id = NULL WHERE webhook_id = ?`
	clearStmt := rqlite.ParameterizedStatement{
		Query:     clearQuery,
		Arguments: []interface{}{id},
	}
	result, err := s.db.WriteOneParameterized(clearStmt)
	if err != nil {
		slog.Error("failed to clear webhook_id from tasks", "error", err, "webhook_id", id)
		return err
	}
	if result.Err != nil {
		slog.Error("clear webhook_id result error", "error", result.Err, "webhook_id", id)
		return result.Err
	}

	deleteQuery := `DELETE FROM bdopsflow_webhooks WHERE id = ?`
	deleteStmt := rqlite.ParameterizedStatement{
		Query:     deleteQuery,
		Arguments: []interface{}{id},
	}
	result, err = s.db.WriteOneParameterized(deleteStmt)
	if err != nil {
		slog.Error("failed to delete webhook", "error", err, "id", id)
		return err
	}
	if result.Err != nil {
		slog.Error("webhook delete result error", "error", result.Err, "id", id)
		return result.Err
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("webhook not found")
	}

	slog.Info("webhook deleted", "id", id)
	return nil
}

func (s *WebhookService) List(ctx context.Context, domainID int64) ([]model.Webhook, error) {
	var query string
	var stmt rqlite.ParameterizedStatement
	if domainID > 0 {
		query = `SELECT id, name, url, method, headers, secret, domain_id, is_enabled, description, created_by, created_at, updated_at FROM bdopsflow_webhooks WHERE domain_id = ? ORDER BY created_at DESC`
		stmt = rqlite.ParameterizedStatement{
			Query:     query,
			Arguments: []interface{}{domainID},
		}
	} else {
		query = `SELECT id, name, url, method, headers, secret, domain_id, is_enabled, description, created_by, created_at, updated_at FROM bdopsflow_webhooks ORDER BY created_at DESC`
		stmt = rqlite.ParameterizedStatement{
			Query: query,
		}
	}

	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	var webhooks []model.Webhook
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}

		wh := model.Webhook{
			ID:          rowInt64(row[0]),
			Name:        rowString(row[1]),
			URL:         rowString(row[2]),
			Method:      rowString(row[3]),
			Headers:     rowString(row[4]),
			Secret:      rowString(row[5]),
			DomainID:    rowInt64(row[6]),
			IsEnabled:   rowBool(row[7]),
			Description: rowString(row[8]),
		}

		if !isEmpty(row[9]) {
			createdBy := rowInt64(row[9])
			wh.CreatedBy = &createdBy
		}
		if !isEmpty(row[10]) {
			wh.CreatedAt = parseDateTime(row[10])
		}
		if !isEmpty(row[11]) {
			wh.UpdatedAt = parseDateTime(row[11])
		}

		webhooks = append(webhooks, wh)
	}

	return webhooks, nil
}

func (s *WebhookService) GetByID(ctx context.Context, id int64) (*model.Webhook, error) {
	query := `SELECT id, name, url, method, headers, secret, domain_id, is_enabled, description, created_by, created_at, updated_at FROM bdopsflow_webhooks WHERE id = ?`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{id},
	}

	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	if !qr.Next() {
		return nil, fmt.Errorf("webhook not found")
	}

	row, err := qr.Slice()
	if err != nil {
		return nil, err
	}

	wh := &model.Webhook{
		ID:          rowInt64(row[0]),
		Name:        rowString(row[1]),
		URL:         rowString(row[2]),
		Method:      rowString(row[3]),
		Headers:     rowString(row[4]),
		Secret:      rowString(row[5]),
		DomainID:    rowInt64(row[6]),
		IsEnabled:   rowBool(row[7]),
		Description: rowString(row[8]),
	}

	if !isEmpty(row[9]) {
		createdBy := rowInt64(row[9])
		wh.CreatedBy = &createdBy
	}
	if !isEmpty(row[10]) {
		wh.CreatedAt = parseDateTime(row[10])
	}
	if !isEmpty(row[11]) {
		wh.UpdatedAt = parseDateTime(row[11])
	}

	return wh, nil
}

type WebhookTestResult struct {
	StatusCode     int    `json:"status_code"`
	ResponseTimeMs int64  `json:"response_time_ms"`
	Error          string `json:"error,omitempty"`
}

func (s *WebhookService) Test(ctx context.Context, id int64) (*WebhookTestResult, error) {
	wh, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	payload := map[string]interface{}{
		"event":       "test",
		"timestamp":   time.Now().Unix(),
		"delivery_id": uuid.New().String(),
		"task": map[string]interface{}{
			"id":   0,
			"name": "test-task",
			"type": "test",
		},
		"execution": map[string]interface{}{
			"id":          "test-execution",
			"status":      "test",
			"output":      "This is a test webhook notification from BDopsFlow",
			"error":       "",
			"duration_ms": 0,
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal test payload: %w", err)
	}

	signature := ""
	if wh.Secret != "" {
		mac := hmac.New(sha256.New, []byte(wh.Secret))
		mac.Write(jsonData)
		signature = fmt.Sprintf("sha256=%s", hex.EncodeToString(mac.Sum(nil)))
	}

	return sendWebhookRequest(wh.Method, wh.URL, wh.Headers, signature, jsonData)
}

func ComputeWebhookSignature(secret string, body []byte) string {
	if secret == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return fmt.Sprintf("sha256=%s", hex.EncodeToString(mac.Sum(nil)))
}

func (s *WebhookService) SendWithSignature(ctx context.Context, config webhook.WebhookConfig, payload map[string]interface{}, secret string) error {
	method := config.Method
	if method == "" {
		method = "POST"
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	signature := ""
	if secret != "" {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(jsonData)
		signature = fmt.Sprintf("sha256=%s", hex.EncodeToString(mac.Sum(nil)))
	}

	headersJSON, _ := json.Marshal(config.Headers)
	_, err = sendWebhookRequest(method, config.URL, string(headersJSON), signature, jsonData)
	return err
}

func sendWebhookRequest(method, url, headersJSON, signature string, jsonData []byte) (*WebhookTestResult, error) {
	req, err := http.NewRequest(method, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if signature != "" {
		req.Header.Set("X-Webhook-Signature", signature)
	}

	if headersJSON != "" && headersJSON != "{}" {
		var headers map[string]string
		if json.Unmarshal([]byte(headersJSON), &headers) == nil {
			for key, value := range headers {
				req.Header.Set(key, value)
			}
		}
	}

	client := &http.Client{Timeout: 30 * time.Second}
	start := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		return &WebhookTestResult{Error: err.Error(), ResponseTimeMs: elapsed}, fmt.Errorf("webhook request failed: %w", err)
	}
	defer resp.Body.Close()

	result := &WebhookTestResult{
		StatusCode:     resp.StatusCode,
		ResponseTimeMs: elapsed,
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		result.Error = fmt.Sprintf("webhook returned non-2xx status: %d", resp.StatusCode)
		return result, fmt.Errorf("webhook returned non-2xx status: %d", resp.StatusCode)
	}

	return result, nil
}
