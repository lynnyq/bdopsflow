package service

import (
	"strings"
	"testing"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
)

// TestValidate_Name 校验名称
func TestValidate_Name(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty name", "", true},
		{"whitespace only name", "   ", true},
		{"valid name", "QPS 趋势", false},
		{"too long name", strings.Repeat("a", 101), true},
		{"max length name (100)", strings.Repeat("a", 100), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &model.GrafanaDashboard{
				Name:     tt.input,
				EmbedURL: "https://grafana.example.com/d/abc",
				Width:    "full",
				HeightPx: 300,
			}
			err := validate(d)
			if (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidate_URL 校验嵌入 URL
func TestValidate_URL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
		errType error
	}{
		{"empty url", "", true, ErrGrafanaInvalidURL},
		{"javascript protocol", "javascript:alert(1)", true, ErrGrafanaInvalidURL},
		{"data protocol", "data:text/html,<script>", true, ErrGrafanaInvalidURL},
		{"file protocol", "file:///etc/passwd", true, ErrGrafanaInvalidURL},
		{"ftp protocol", "ftp://example.com", true, ErrGrafanaInvalidURL},
		{"valid http", "http://grafana.example.com/d/abc?kiosk", false, nil},
		{"valid https", "https://grafana.example.com/d/abc?from=now-1h&to=now", false, nil},
		{"url too long", "https://example.com/" + strings.Repeat("a", 2040), true, nil},
		{"url max length (2048)", "https://example.com/" + strings.Repeat("a", 2024), false, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &model.GrafanaDashboard{
				Name:     "test",
				EmbedURL: tt.url,
				Width:    "full",
				HeightPx: 300,
			}
			err := validate(d)
			if (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.errType != nil && err != tt.errType {
				t.Errorf("validate() error = %v, want %v", err, tt.errType)
			}
		})
	}
}

// TestValidate_Width 校验宽度枚举
func TestValidate_Width(t *testing.T) {
	tests := []struct {
		name    string
		width   string
		wantErr bool
	}{
		{"third", "third", false},
		{"half", "half", false},
		{"full", "full", false},
		{"invalid value", "quarter", true},
		{"empty value", "", true},
		{"uppercase THIRD", "THIRD", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &model.GrafanaDashboard{
				Name:     "test",
				EmbedURL: "https://grafana.example.com/d/abc",
				Width:    tt.width,
				HeightPx: 300,
			}
			err := validate(d)
			if (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidate_Height 校验高度范围
func TestValidate_Height(t *testing.T) {
	tests := []struct {
		name    string
		height  int
		wantErr bool
	}{
		{"min-1 (119)", 119, true},
		{"min (120)", 120, false},
		{"normal (300)", 300, false},
		{"max (1200)", 1200, false},
		{"max+1 (1201)", 1201, true},
		{"zero", 0, true},
		{"negative", -100, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &model.GrafanaDashboard{
				Name:     "test",
				EmbedURL: "https://grafana.example.com/d/abc",
				Width:    "full",
				HeightPx: tt.height,
			}
			err := validate(d)
			if (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidate_SortOrder 校验排序号
func TestValidate_SortOrder(t *testing.T) {
	tests := []struct {
		name      string
		sortOrder int
		wantErr   bool
	}{
		{"zero", 0, false},
		{"positive", 10, false},
		{"large", 9999, false},
		{"negative", -1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &model.GrafanaDashboard{
				Name:      "test",
				EmbedURL:  "https://grafana.example.com/d/abc",
				Width:     "full",
				HeightPx:  300,
				SortOrder: tt.sortOrder,
			}
			err := validate(d)
			if (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidate_ValidConfig 完整合法配置
func TestValidate_ValidConfig(t *testing.T) {
	d := &model.GrafanaDashboard{
		Name:      "QPS 监控",
		EmbedURL:  "https://grafana.example.com/d/abc-xyz?from=now-1h&to=now&kiosk",
		Width:     "half",
		HeightPx:  400,
		SortOrder: 5,
		Enabled:   1,
	}
	if err := validate(d); err != nil {
		t.Errorf("validate() unexpected error = %v", err)
	}
}

// TestGetErrorCode_GrafanaErrors 错误码映射
func TestGetErrorCode_GrafanaErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
	}{
		{"not found", ErrGrafanaDashboardNotFound, 19001},
		{"invalid url", ErrGrafanaInvalidURL, 19002},
		{"invalid width", ErrGrafanaInvalidWidth, 19003},
		{"invalid height", ErrGrafanaInvalidHeight, 19004},
		{"invalid sort order", ErrGrafanaInvalidSortOrder, 19005},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if code := GetErrorCode(tt.err); code != tt.wantCode {
				t.Errorf("GetErrorCode() = %v, want %v", code, tt.wantCode)
			}
		})
	}
}
