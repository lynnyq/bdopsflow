package model

import (
	"encoding/json"
	"testing"
)

func TestApiTestResult_ExecutedByNameField(t *testing.T) {
	// 验证 ExecutedByName 字段可正确序列化
	r := &ApiTestResult{
		ID:             1,
		TestID:         100,
		Type:           "http",
		StatusCode:     200,
		LatencyMs:      50,
		ExecutedBy:     42,
		ExecutedByName: "张三",
	}

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if got["executed_by_name"] != "张三" {
		t.Errorf("executed_by_name = %v, want 张三", got["executed_by_name"])
	}
	// 确认 executed_by (id) 仍然保留
	if v, ok := got["executed_by"].(float64); !ok || int64(v) != 42 {
		t.Errorf("executed_by = %v, want 42", got["executed_by"])
	}
}

func TestApiTestResult_ExecutedByName_OmitEmpty(t *testing.T) {
	// 验证 executed_by_name 为空时 omitted
	r := &ApiTestResult{
		ID:     1,
		TestID: 100,
		Type:   "http",
	}

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if _, ok := got["executed_by_name"]; ok {
		t.Errorf("executed_by_name should be omitted when empty, got %v", got["executed_by_name"])
	}
}

func TestApiTestResult_TestName_OmitEmpty(t *testing.T) {
	// 验证 TestName 字段(已有)同样 omitted
	r := &ApiTestResult{
		ID:     1,
		TestID: 100,
		Type:   "http",
	}

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if _, ok := got["test_name"]; ok {
		t.Errorf("test_name should be omitted when empty, got %v", got["test_name"])
	}
}
