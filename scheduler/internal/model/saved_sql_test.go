package model

import (
	"encoding/json"
	"testing"
)

func TestSavedSQL_CreatedByNameField(t *testing.T) {
	// 验证新增字段 CreatedByName 可正确序列化
	uid := int64(42)
	s := &SavedSQL{
		ID:            1,
		Name:          "测试SQL",
		DatasourceID:  1,
		SQLText:       "SELECT 1",
		CreatedBy:     &uid,
		CreatedByName: "张三",
		DomainID:      1,
		IsPublic:      false,
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if got["created_by_name"] != "张三" {
		t.Errorf("created_by_name = %v, want 张三", got["created_by_name"])
	}
	// 同时确认 created_by (id) 仍然存在
	if v, ok := got["created_by"].(float64); !ok || int64(v) != 42 {
		t.Errorf("created_by = %v, want 42", got["created_by"])
	}
}

func TestSavedSQL_CreatedByName_OmitEmpty(t *testing.T) {
	// 验证 created_by_name 为空时 omitted
	s := &SavedSQL{
		ID:           1,
		Name:         "测试SQL",
		DatasourceID: 1,
		SQLText:      "SELECT 1",
		DomainID:     1,
		IsPublic:     false,
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if _, ok := got["created_by_name"]; ok {
		t.Errorf("created_by_name should be omitted when empty, got %v", got["created_by_name"])
	}
	if _, ok := got["updated_by_name"]; ok {
		t.Errorf("updated_by_name should be omitted when empty, got %v", got["updated_by_name"])
	}
}

func TestSavedSQL_UpdatedByNameField(t *testing.T) {
	// 验证 UpdatedByName 字段同样可正确序列化
	uid := int64(7)
	s := &SavedSQL{
		ID:            1,
		Name:          "测试SQL",
		DatasourceID:  1,
		SQLText:       "SELECT 1",
		UpdatedBy:     &uid,
		UpdatedByName: "李四",
		DomainID:      1,
		IsPublic:      false,
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if got["updated_by_name"] != "李四" {
		t.Errorf("updated_by_name = %v, want 李四", got["updated_by_name"])
	}
}
