package model

import (
	"encoding/json"
	"testing"
)

func TestQueryHistory_ExecutedByNameField(t *testing.T) {
	// 验证新字段 ExecutedByName 可正确序列化
	h := &QueryHistory{
		ID:             1,
		SQLText:        "SELECT 1",
		Status:         "success",
		ExecutedByName: "张三",
	}

	data, err := json.Marshal(h)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	// 验证 JSON 包含 executed_by_name 字段
	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if got["executed_by_name"] != "张三" {
		t.Errorf("executed_by_name = %v, want 张三", got["executed_by_name"])
	}
}

func TestQueryHistory_ExecutedByName_OmitEmpty(t *testing.T) {
	// 验证 executed_by 为空时 omitted,executed_by_name 也不输出
	h := &QueryHistory{
		ID:      1,
		SQLText: "SELECT 1",
		Status:  "success",
	}

	data, err := json.Marshal(h)
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

func TestQueryHistory_ExecutedByPtr(t *testing.T) {
	// 验证 ExecutedBy 指针字段行为
	uid := int64(42)
	h := &QueryHistory{
		ID:         1,
		SQLText:    "SELECT 1",
		Status:     "success",
		ExecutedBy: &uid,
	}

	data, err := json.Marshal(h)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	// executed_by 序列化为数字
	if v, ok := got["executed_by"].(float64); !ok || int64(v) != 42 {
		t.Errorf("executed_by = %v, want 42", got["executed_by"])
	}
}
