package model

import "time"

// ProtoFile Proto文件
type ProtoFile struct {
	ID           int64     `db:"id" json:"id"`
	Name         string    `db:"name" json:"name"`
	Content      string    `db:"content" json:"content"`
	FileHash     string    `db:"file_hash" json:"file_hash"`
	ParsedResult string    `db:"parsed_result" json:"parsed_result,omitempty"`
	Dependencies string    `db:"dependencies" json:"dependencies"`
	CreatedBy    int64     `db:"created_by" json:"created_by"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
}

// ProtoService Proto服务定义
type ProtoService struct {
	Name    string          `json:"name"`
	Methods []ProtoMethod   `json:"methods"`
}

// ProtoMethod Proto方法定义
type ProtoMethod struct {
	Name         string `json:"name"`
	InputType    string `json:"input_type"`
	OutputType   string `json:"output_type"`
	ClientStream bool   `json:"client_stream"`
	ServerStream bool   `json:"server_stream"`
}

// ProtoMessageDef Proto消息定义（包含字段详情）
type ProtoMessageDef struct {
	Name     string              `json:"name"`
	FullName string              `json:"full_name"`
	Fields   []ProtoMessageField `json:"fields"`
}

// ProtoMessageField Proto消息字段定义
type ProtoMessageField struct {
	Name     string              `json:"name"`
	Number   int                 `json:"number"`
	Type     string              `json:"type"`               // "string", "int32", "bool", "message:FullTypeName", "enum:FullEnumName", etc.
	Label    string              `json:"label"`              // "optional", "repeated", "required"
	MapKey   string              `json:"map_key,omitempty"`  // map key type, e.g. "string"
	MapValue string              `json:"map_value,omitempty"` // map value type
	Fields   []ProtoMessageField `json:"fields,omitempty"`   // nested message fields (inline for convenience)
}

// ProtoParseResult Proto解析结果
type ProtoParseResult struct {
	Package  string            `json:"package,omitempty"`
	Services []ProtoService    `json:"services,omitempty"`
	Messages []string          `json:"messages,omitempty"`
}
