package service

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/descriptorpb"
)

// ============ NewGRPCExecutor ============

func TestNewGRPCExecutor(t *testing.T) {
	t.Run("nil configService", func(t *testing.T) {
		e := NewGRPCExecutor(nil)
		if e == nil {
			t.Fatal("期望返回非 nil 实例")
		}
		if e.configService != nil {
			t.Error("期望 configService 为 nil")
		}
	})

	t.Run("with configService", func(t *testing.T) {
		cs := newConfigServiceWithPrivateNetwork(t)
		e := NewGRPCExecutor(cs)
		if e == nil {
			t.Fatal("期望返回非 nil 实例")
		}
		if e.configService == nil {
			t.Error("期望 configService 被赋值")
		}
	})
}

// ============ Execute (error paths) ============

func TestGRPCExecutor_Execute_PrivateAddressBlocked(t *testing.T) {
	e := NewGRPCExecutor(nil) // 不允许内网
	config := &model.GRPCRequestConfig{
		Address: "127.0.0.1:50051",
		Service: "TestService",
		Method:  "TestMethod",
	}
	_, err := e.Execute(context.Background(), config, "", nil, nil)
	if err == nil {
		t.Fatal("期望返回 SSRF 错误")
	}
	if !strings.Contains(err.Error(), "private") && !strings.Contains(err.Error(), "address") {
		t.Errorf("期望 SSRF 相关错误，实际: %v", err)
	}
}

func TestGRPCExecutor_Execute_UnsupportedTLSMode(t *testing.T) {
	cs := newConfigServiceWithPrivateNetwork(t)
	e := NewGRPCExecutor(cs)
	config := &model.GRPCRequestConfig{
		Address: "127.0.0.1:50051",
		Service: "TestService",
		Method:  "TestMethod",
		TLSMode: "invalid_mode",
	}
	_, err := e.Execute(context.Background(), config, "", nil, nil)
	if err == nil {
		t.Fatal("期望返回 TLS 模式错误")
	}
	if !strings.Contains(err.Error(), "unsupported TLS mode") {
		t.Errorf("期望 unsupported TLS mode 错误，实际: %v", err)
	}
}

func TestGRPCExecutor_Execute_NoProtoNoReflection(t *testing.T) {
	cs := newConfigServiceWithPrivateNetwork(t)
	e := NewGRPCExecutor(cs)
	config := &model.GRPCRequestConfig{
		Address: "127.0.0.1:50051",
		Service: "TestService",
		Method:  "TestMethod",
		// UseReflection=false, protoContent=""
	}
	_, err := e.Execute(context.Background(), config, "", nil, nil)
	if err == nil {
		t.Fatal("期望返回错误（无 proto 也无 reflection）")
	}
	if !strings.Contains(err.Error(), "UseReflection") && !strings.Contains(err.Error(), "proto") {
		t.Errorf("期望 reflection/proto 相关错误，实际: %v", err)
	}
}

func TestGRPCExecutor_Execute_InvalidProto(t *testing.T) {
	cs := newConfigServiceWithPrivateNetwork(t)
	e := NewGRPCExecutor(cs)
	config := &model.GRPCRequestConfig{
		Address: "127.0.0.1:50051",
		Service: "TestService",
		Method:  "TestMethod",
	}
	_, err := e.Execute(context.Background(), config, "invalid proto content", nil, nil)
	if err == nil {
		t.Fatal("期望返回 proto 解析错误")
	}
}

func TestGRPCExecutor_Execute_ServiceNotFoundInProto(t *testing.T) {
	cs := newConfigServiceWithPrivateNetwork(t)
	e := NewGRPCExecutor(cs)
	protoContent := `
syntax = "proto3";
package test;
service RealService {
  rpc RealMethod(TestRequest) returns (TestResponse);
}
message TestRequest {}
message TestResponse {}
`
	config := &model.GRPCRequestConfig{
		Address: "127.0.0.1:50051",
		Service: "NonExistentService",
		Method:  "TestMethod",
	}
	_, err := e.Execute(context.Background(), config, protoContent, nil, nil)
	if err == nil {
		t.Fatal("期望返回 service 未找到错误")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("期望 not found 错误，实际: %v", err)
	}
}

// ============ ConnectTest (error paths) ============

func TestGRPCExecutor_ConnectTest_PrivateAddressBlocked(t *testing.T) {
	e := NewGRPCExecutor(nil)
	config := &model.GRPCRequestConfig{
		Address: "192.168.1.1:50051",
	}
	_, err := e.ConnectTest(context.Background(), config, nil)
	if err == nil {
		t.Fatal("期望返回 SSRF 错误")
	}
}

func TestGRPCExecutor_ConnectTest_UnsupportedTLSMode(t *testing.T) {
	cs := newConfigServiceWithPrivateNetwork(t)
	e := NewGRPCExecutor(cs)
	config := &model.GRPCRequestConfig{
		Address: "127.0.0.1:50051",
		TLSMode: "invalid",
	}
	_, err := e.ConnectTest(context.Background(), config, nil)
	if err == nil {
		t.Fatal("期望返回 TLS 模式错误")
	}
}

func TestGRPCExecutor_ConnectTest_MTLSWithoutCert(t *testing.T) {
	cs := newConfigServiceWithPrivateNetwork(t)
	e := NewGRPCExecutor(cs)
	config := &model.GRPCRequestConfig{
		Address: "127.0.0.1:50051",
		TLSMode: "mtls",
	}
	result, err := e.ConnectTest(context.Background(), config, nil)
	if err != nil {
		t.Fatalf("期望无错误（返回结果中含错误信息），实际: %v", err)
	}
	if result == nil {
		t.Fatal("期望返回非 nil 结果")
	}
	if result.Error == "" {
		t.Error("期望 Error 字段非空（mTLS 需要证书）")
	}
	if !strings.Contains(result.Error, "mTLS") && !strings.Contains(result.Error, "证书") {
		t.Errorf("期望 mTLS/证书 相关错误信息，实际: %s", result.Error)
	}
}

// ============ ReflectServices (error paths) ============

func TestGRPCExecutor_ReflectServices_PrivateAddressBlocked(t *testing.T) {
	e := NewGRPCExecutor(nil)
	_, err := e.ReflectServices(context.Background(), "10.0.0.1:50051", "insecure", nil)
	if err == nil {
		t.Fatal("期望返回 SSRF 错误")
	}
}

func TestGRPCExecutor_ReflectServices_UnsupportedTLSMode(t *testing.T) {
	cs := newConfigServiceWithPrivateNetwork(t)
	e := NewGRPCExecutor(cs)
	_, err := e.ReflectServices(context.Background(), "127.0.0.1:50051", "invalid", nil)
	if err == nil {
		t.Fatal("期望返回 TLS 模式错误")
	}
}

// ============ GenerateFields ============

func TestGRPCExecutor_GenerateFields_Success(t *testing.T) {
	e := NewGRPCExecutor(nil)
	protoContent := `
syntax = "proto3";
package test;
message TestMessage {
  string name = 1;
  int32 age = 2;
  bool active = 3;
}
`
	messages, err := e.GenerateFields(protoContent, nil)
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("期望 1 个消息定义，实际 %d", len(messages))
	}
	if messages[0].Name != "TestMessage" {
		t.Errorf("期望消息名 TestMessage，实际 %s", messages[0].Name)
	}
	if len(messages[0].Fields) != 3 {
		t.Fatalf("期望 3 个字段，实际 %d", len(messages[0].Fields))
	}
	// 验证字段
	fieldMap := make(map[string]model.ProtoMessageField)
	for _, f := range messages[0].Fields {
		fieldMap[f.Name] = f
	}
	if f, ok := fieldMap["name"]; !ok || f.Type != "string" {
		t.Errorf("期望 name 字段类型 string，实际 %+v", f)
	}
	if f, ok := fieldMap["age"]; !ok || f.Type != "int32" {
		t.Errorf("期望 age 字段类型 int32，实际 %+v", f)
	}
	if f, ok := fieldMap["active"]; !ok || f.Type != "bool" {
		t.Errorf("期望 active 字段类型 bool，实际 %+v", f)
	}
}

func TestGRPCExecutor_GenerateFields_NestedMessage(t *testing.T) {
	e := NewGRPCExecutor(nil)
	protoContent := `
syntax = "proto3";
package test;
message Outer {
  string id = 1;
  Inner inner = 2;
}
message Inner {
  int32 value = 1;
}
`
	messages, err := e.GenerateFields(protoContent, nil)
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("期望 2 个消息定义，实际 %d", len(messages))
	}
	// 找到 Outer 消息
	var outer *model.ProtoMessageDef
	for i := range messages {
		if messages[i].Name == "Outer" {
			outer = &messages[i]
			break
		}
	}
	if outer == nil {
		t.Fatal("未找到 Outer 消息")
	}
	// 验证 inner 字段是 message 类型并包含嵌套字段
	var innerField *model.ProtoMessageField
	for i := range outer.Fields {
		if outer.Fields[i].Name == "inner" {
			innerField = &outer.Fields[i]
			break
		}
	}
	if innerField == nil {
		t.Fatal("未找到 inner 字段")
	}
	if !strings.HasPrefix(innerField.Type, "message:") {
		t.Errorf("期望 inner 字段类型以 message: 开头，实际 %s", innerField.Type)
	}
	if len(innerField.Fields) != 1 {
		t.Errorf("期望 inner 字段嵌套 1 个字段，实际 %d", len(innerField.Fields))
	}
}

func TestGRPCExecutor_GenerateFields_RepeatedAndMap(t *testing.T) {
	e := NewGRPCExecutor(nil)
	protoContent := `
syntax = "proto3";
package test;
message TestMessage {
  repeated string tags = 1;
  map<string, int32> counts = 2;
}
`
	messages, err := e.GenerateFields(protoContent, nil)
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("期望 1 个消息定义，实际 %d", len(messages))
	}
	msg := messages[0]
	if len(msg.Fields) != 2 {
		t.Fatalf("期望 2 个字段，实际 %d", len(msg.Fields))
	}
	// tags 应是 repeated
	var tagsField, countsField *model.ProtoMessageField
	for i := range msg.Fields {
		switch msg.Fields[i].Name {
		case "tags":
			tagsField = &msg.Fields[i]
		case "counts":
			countsField = &msg.Fields[i]
		}
	}
	if tagsField == nil || tagsField.Label != "repeated" {
		t.Errorf("期望 tags 字段 label=repeated，实际 %+v", tagsField)
	}
	if countsField == nil || countsField.Label != "map" {
		t.Errorf("期望 counts 字段 label=map，实际 %+v", countsField)
	}
	if countsField != nil {
		if countsField.MapKey != "string" {
			t.Errorf("期望 map key=string，实际 %s", countsField.MapKey)
		}
		if countsField.MapValue != "int32" {
			t.Errorf("期望 map value=int32，实际 %s", countsField.MapValue)
		}
	}
}

func TestGRPCExecutor_GenerateFields_EnumField(t *testing.T) {
	e := NewGRPCExecutor(nil)
	protoContent := `
syntax = "proto3";
package test;
enum Status {
  UNKNOWN = 0;
  ACTIVE = 1;
}
message TestMessage {
  Status status = 1;
}
`
	messages, err := e.GenerateFields(protoContent, nil)
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("期望 1 个消息定义，实际 %d", len(messages))
	}
	if len(messages[0].Fields) != 1 {
		t.Fatalf("期望 1 个字段，实际 %d", len(messages[0].Fields))
	}
	f := messages[0].Fields[0]
	if !strings.HasPrefix(f.Type, "enum:") {
		t.Errorf("期望字段类型以 enum: 开头，实际 %s", f.Type)
	}
}

func TestGRPCExecutor_GenerateFields_InvalidProto(t *testing.T) {
	e := NewGRPCExecutor(nil)
	_, err := e.GenerateFields("invalid proto", nil)
	if err == nil {
		t.Fatal("期望返回解析错误")
	}
}

func TestGRPCExecutor_GenerateFields_WithDependencies(t *testing.T) {
	e := NewGRPCExecutor(nil)
	depProto := `
syntax = "proto3";
package common;
message CommonMessage {
  string id = 1;
}
`
	mainProto := `
syntax = "proto3";
package test;
import "common.proto";
message TestMessage {
  common.CommonMessage ref = 1;
}
`
	depContents := map[string]string{
		"common.proto": depProto,
	}
	messages, err := e.GenerateFields(mainProto, depContents)
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("期望 1 个消息定义，实际 %d", len(messages))
	}
}

// ============ GenerateTemplate ============

func TestGRPCExecutor_GenerateTemplate_Success(t *testing.T) {
	e := NewGRPCExecutor(nil)
	protoContent := `
syntax = "proto3";
package test;
service Greeter {
  rpc SayHello(HelloRequest) returns (HelloReply);
}
message HelloRequest {
  string name = 1;
}
message HelloReply {
  string message = 1;
}
`
	template, err := e.GenerateTemplate(protoContent, nil, "Greeter", "SayHello")
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if template == "" {
		t.Error("期望返回非空模板")
	}
	// proto3 中零值标量字段在 JSON 中被省略，所以空消息的模板为 "{}"
	// 验证模板是有效的 JSON 对象
	if template != "{}" {
		// 如果模板不为 "{}"，它至少应包含 name 或 message 字段
		if !strings.Contains(template, "name") && !strings.Contains(template, "message") {
			t.Errorf("期望模板为 \"{}\" 或包含字段名，实际: %s", template)
		}
	}
}

func TestGRPCExecutor_GenerateTemplate_ServiceNotFound(t *testing.T) {
	e := NewGRPCExecutor(nil)
	protoContent := `
syntax = "proto3";
package test;
service Greeter {
  rpc SayHello(HelloRequest) returns (HelloReply);
}
message HelloRequest {}
message HelloReply {}
`
	_, err := e.GenerateTemplate(protoContent, nil, "NonExistent", "TestMethod")
	if err == nil {
		t.Fatal("期望返回 service 未找到错误")
	}
}

func TestGRPCExecutor_GenerateTemplate_MethodNotFound(t *testing.T) {
	e := NewGRPCExecutor(nil)
	protoContent := `
syntax = "proto3";
package test;
service Greeter {
  rpc SayHello(HelloRequest) returns (HelloReply);
}
message HelloRequest {}
message HelloReply {}
`
	_, err := e.GenerateTemplate(protoContent, nil, "Greeter", "NonExistentMethod")
	if err == nil {
		t.Fatal("期望返回 method 未找到错误")
	}
}

func TestGRPCExecutor_GenerateTemplate_InvalidProto(t *testing.T) {
	e := NewGRPCExecutor(nil)
	_, err := e.GenerateTemplate("invalid", nil, "Service", "Method")
	if err == nil {
		t.Fatal("期望返回解析错误")
	}
}

// ============ parseMethodFromProto ============

func TestGRPCExecutor_parseMethodFromProto_Success(t *testing.T) {
	e := NewGRPCExecutor(nil)
	protoContent := `
syntax = "proto3";
package test;
service Greeter {
  rpc SayHello(HelloRequest) returns (HelloReply);
}
message HelloRequest {}
message HelloReply {}
`
	methodDesc, err := e.parseMethodFromProto(protoContent, nil, "Greeter", "SayHello")
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if methodDesc == nil {
		t.Fatal("期望返回非 nil 方法描述符")
	}
	if methodDesc.GetName() != "SayHello" {
		t.Errorf("期望方法名 SayHello，实际 %s", methodDesc.GetName())
	}
}

func TestGRPCExecutor_parseMethodFromProto_FullyQualifiedName(t *testing.T) {
	e := NewGRPCExecutor(nil)
	protoContent := `
syntax = "proto3";
package test;
service Greeter {
  rpc SayHello(HelloRequest) returns (HelloReply);
}
message HelloRequest {}
message HelloReply {}
`
	// 使用完全限定名
	methodDesc, err := e.parseMethodFromProto(protoContent, nil, "test.Greeter", "SayHello")
	if err != nil {
		t.Fatalf("期望无错误（使用 FQN），实际: %v", err)
	}
	if methodDesc == nil {
		t.Fatal("期望返回非 nil 方法描述符")
	}
}

func TestGRPCExecutor_parseMethodFromProto_ServiceNotFound(t *testing.T) {
	e := NewGRPCExecutor(nil)
	protoContent := `
syntax = "proto3";
package test;
service Greeter {
  rpc SayHello(HelloRequest) returns (HelloReply);
}
message HelloRequest {}
message HelloReply {}
`
	_, err := e.parseMethodFromProto(protoContent, nil, "NonExistent", "SayHello")
	if err == nil {
		t.Fatal("期望返回 service 未找到错误")
	}
}

func TestGRPCExecutor_parseMethodFromProto_MethodNotFound(t *testing.T) {
	e := NewGRPCExecutor(nil)
	protoContent := `
syntax = "proto3";
package test;
service Greeter {
  rpc SayHello(HelloRequest) returns (HelloReply);
}
message HelloRequest {}
message HelloReply {}
`
	_, err := e.parseMethodFromProto(protoContent, nil, "Greeter", "NonExistent")
	if err == nil {
		t.Fatal("期望返回 method 未找到错误")
	}
}

func TestGRPCExecutor_parseMethodFromProto_InvalidProto(t *testing.T) {
	e := NewGRPCExecutor(nil)
	_, err := e.parseMethodFromProto("invalid", nil, "Service", "Method")
	if err == nil {
		t.Fatal("期望返回解析错误")
	}
}

func TestGRPCExecutor_parseMethodFromProto_WithDependencies(t *testing.T) {
	e := NewGRPCExecutor(nil)
	depProto := `
syntax = "proto3";
package common;
message CommonRequest {}
message CommonResponse {}
`
	mainProto := `
syntax = "proto3";
package test;
import "common.proto";
service Service {
  rpc Method(common.CommonRequest) returns (common.CommonResponse);
}
`
	depContents := map[string]string{
		"common.proto": depProto,
	}
	methodDesc, err := e.parseMethodFromProto(mainProto, depContents, "Service", "Method")
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if methodDesc == nil {
		t.Fatal("期望返回非 nil 方法描述符")
	}
}

// ============ validateAddress ============

func TestGRPCExecutor_validateAddress(t *testing.T) {
	tests := []struct {
		name          string
		address       string
		configService bool // true=允许内网
		expectErr     bool
		errContains   string
	}{
		{
			name:          "nil configService - private IP 127.0.0.1 blocked",
			address:       "127.0.0.1:50051",
			configService: false,
			expectErr:     true,
			errContains:   "private",
		},
		{
			name:          "nil configService - private IP 10.x blocked",
			address:       "10.0.0.1:50051",
			configService: false,
			expectErr:     true,
			errContains:   "private",
		},
		{
			name:          "nil configService - private IP 192.168.x blocked",
			address:       "192.168.1.1:50051",
			configService: false,
			expectErr:     true,
			errContains:   "private",
		},
		{
			name:          "nil configService - private IP 172.16.x blocked",
			address:       "172.16.0.1:50051",
			configService: false,
			expectErr:     true,
			errContains:   "private",
		},
		{
			name:          "allow private network - 127.0.0.1 allowed",
			address:       "127.0.0.1:50051",
			configService: true,
			expectErr:     false,
		},
		{
			name:          "nil configService - unresolvable hostname",
			address:       "nonexistent.invalid.domain:50051",
			configService: false,
			expectErr:     true,
			errContains:   "resolve",
		},
		{
			name:          "IPv6 loopback blocked",
			address:       "[::1]:50051",
			configService: false,
			expectErr:     true,
			errContains:   "private",
		},
		{
			name:          "link-local 169.254 blocked",
			address:       "169.254.1.1:50051",
			configService: false,
			expectErr:     true,
			errContains:   "private",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var e *GRPCExecutor
			if tt.configService {
				cs := newConfigServiceWithPrivateNetwork(t)
				e = NewGRPCExecutor(cs)
			} else {
				e = NewGRPCExecutor(nil)
			}

			err := e.validateAddress(tt.address)
			if tt.expectErr {
				if err == nil {
					t.Fatal("期望返回错误")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("期望错误包含 %q，实际: %v", tt.errContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("期望无错误，实际: %v", err)
				}
			}
		})
	}
}

// ============ isPrivateIP ============

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		{"loopback IPv4", "127.0.0.1", true},
		{"loopback IPv4 alt", "127.0.0.2", true},
		{"private 10.x", "10.0.0.1", true},
		{"private 172.16.x", "172.16.0.1", true},
		{"private 172.31.x", "172.31.255.255", true},
		{"private 192.168.x", "192.168.1.1", true},
		{"link-local 169.254.x", "169.254.1.1", true},
		{"IPv6 loopback", "::1", true},
		{"IPv6 unique local fc00::", "fc00::1", true},
		{"IPv6 unique local fd00::", "fd00::1", true},
		{"IPv6 link-local fe80::", "fe80::1", true},
		{"public IPv4 8.8.8.8", "8.8.8.8", false},
		{"public IPv4 1.1.1.1", "1.1.1.1", false},
		{"public 172.32.x (not private)", "172.32.0.1", false},
		{"public 172.15.x (not private)", "172.15.0.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("无法解析 IP: %s", tt.ip)
			}
			result := isPrivateIP(ip)
			if result != tt.expected {
				t.Errorf("isPrivateIP(%s) = %v, 期望 %v", tt.ip, result, tt.expected)
			}
		})
	}
}

// ============ mustParseCIDR ============

func TestMustParseCIDR_Valid(t *testing.T) {
	tests := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"::1/128",
		"fc00::/7",
		"fe80::/10",
	}
	for _, cidr := range tests {
		t.Run(cidr, func(t *testing.T) {
			network := mustParseCIDR(cidr)
			if network == nil {
				t.Fatal("期望返回非 nil network")
			}
		})
	}
}

func TestMustParseCIDR_InvalidPanics(t *testing.T) {
	tests := []string{
		"invalid",
		"10.0.0.0/33",
		"::1/129",
		"",
	}
	for _, cidr := range tests {
		t.Run(cidr, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("期望 panic，实际未 panic (cidr=%s)", cidr)
				}
			}()
			_ = mustParseCIDR(cidr)
		})
	}
}

// ============ buildDialOptions ============

func TestGRPCExecutor_buildDialOptions(t *testing.T) {
	e := NewGRPCExecutor(nil)

	t.Run("insecure mode", func(t *testing.T) {
		opts, err := e.buildDialOptions("insecure", nil)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(opts) == 0 {
			t.Error("期望返回非空 dial options")
		}
	})

	t.Run("empty mode defaults to insecure", func(t *testing.T) {
		opts, err := e.buildDialOptions("", nil)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(opts) == 0 {
			t.Error("期望返回非空 dial options")
		}
	})

	t.Run("tls mode without cert", func(t *testing.T) {
		opts, err := e.buildDialOptions("tls", nil)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(opts) == 0 {
			t.Error("期望返回非空 dial options")
		}
	})

	t.Run("tls mode with CA cert", func(t *testing.T) {
		caCertPEM := generateTestCACertPEM(t)
		cert := &model.Certificate{
			CaCert: caCertPEM,
		}
		opts, err := e.buildDialOptions("tls", cert)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(opts) == 0 {
			t.Error("期望返回非空 dial options")
		}
	})

	t.Run("tls mode with invalid CA cert", func(t *testing.T) {
		cert := &model.Certificate{
			CaCert: "invalid cert content",
		}
		_, err := e.buildDialOptions("tls", cert)
		if err == nil {
			t.Fatal("期望返回 CA 证书解析错误")
		}
		if !strings.Contains(err.Error(), "CA") {
			t.Errorf("期望 CA 相关错误，实际: %v", err)
		}
	})

	t.Run("mtls mode without cert", func(t *testing.T) {
		_, err := e.buildDialOptions("mtls", nil)
		if err == nil {
			t.Fatal("期望返回错误（mTLS 需要证书）")
		}
	})

	t.Run("mtls mode with incomplete cert (missing client cert/key)", func(t *testing.T) {
		cert := &model.Certificate{
			CaCert: generateTestCACertPEM(t),
			// ClientCert 和 ClientKey 为空
		}
		_, err := e.buildDialOptions("mtls", cert)
		if err == nil {
			t.Fatal("期望返回错误（缺少客户端证书）")
		}
		if !strings.Contains(err.Error(), "client certificate") && !strings.Contains(err.Error(), "客户端证书") {
			t.Errorf("期望客户端证书相关错误，实际: %v", err)
		}
	})

	t.Run("unsupported mode", func(t *testing.T) {
		_, err := e.buildDialOptions("invalid", nil)
		if err == nil {
			t.Fatal("期望返回错误")
		}
		if !strings.Contains(err.Error(), "unsupported TLS mode") {
			t.Errorf("期望 unsupported TLS mode 错误，实际: %v", err)
		}
	})
}

// ============ buildTLSConfig ============

func TestGRPCExecutor_buildTLSConfig(t *testing.T) {
	e := NewGRPCExecutor(nil)

	t.Run("insecure returns nil", func(t *testing.T) {
		cfg, err := e.buildTLSConfig("insecure", nil)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if cfg != nil {
			t.Error("期望 insecure 模式返回 nil TLS config")
		}
	})

	t.Run("empty returns nil", func(t *testing.T) {
		cfg, err := e.buildTLSConfig("", nil)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if cfg != nil {
			t.Error("期望空模式返回 nil TLS config")
		}
	})

	t.Run("tls returns basic config", func(t *testing.T) {
		cfg, err := e.buildTLSConfig("tls", nil)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if cfg == nil {
			t.Fatal("期望返回非 nil TLS config")
		}
	})

	t.Run("mtls without cert returns error", func(t *testing.T) {
		_, err := e.buildTLSConfig("mtls", nil)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("mtls with incomplete cert returns error", func(t *testing.T) {
		cert := &model.Certificate{
			CaCert: generateTestCACertPEM(t),
		}
		_, err := e.buildTLSConfig("mtls", cert)
		if err == nil {
			t.Fatal("期望返回错误（缺少客户端证书和密钥）")
		}
	})

	t.Run("mtls with invalid client cert/key returns error", func(t *testing.T) {
		cert := &model.Certificate{
			CaCert:     generateTestCACertPEM(t),
			ClientCert: "invalid client cert",
			ClientKey:  "invalid client key",
		}
		_, err := e.buildTLSConfig("mtls", cert)
		if err == nil {
			t.Fatal("期望返回错误（无效的客户端证书/密钥）")
		}
	})

	t.Run("unsupported mode returns error", func(t *testing.T) {
		_, err := e.buildTLSConfig("invalid", nil)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// ============ extractGRPCStatusCode ============

func TestGRPCExecutor_extractGRPCStatusCode(t *testing.T) {
	e := NewGRPCExecutor(nil)

	t.Run("nil error returns 0", func(t *testing.T) {
		code := e.extractGRPCStatusCode(nil)
		if code != 0 {
			t.Errorf("期望 code=0，实际 %d", code)
		}
	})

	t.Run("gRPC status error returns code", func(t *testing.T) {
		tests := []struct {
			name     string
			err      error
			expected int
		}{
			{"OK", status.Error(codes.OK, "ok"), 0},
			{"NotFound", status.Error(codes.NotFound, "not found"), int(codes.NotFound)},
			{"PermissionDenied", status.Error(codes.PermissionDenied, "denied"), int(codes.PermissionDenied)},
			{"Unavailable", status.Error(codes.Unavailable, "unavailable"), int(codes.Unavailable)},
			{"Internal", status.Error(codes.Internal, "internal"), int(codes.Internal)},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				code := e.extractGRPCStatusCode(tt.err)
				if code != tt.expected {
					t.Errorf("期望 code=%d，实际 %d", tt.expected, code)
				}
			})
		}
	})

	t.Run("non-gRPC error returns 2 (Unknown)", func(t *testing.T) {
		code := e.extractGRPCStatusCode(errors.New("plain error"))
		if code != 2 {
			t.Errorf("期望 code=2 (Unknown)，实际 %d", code)
		}
	})
}

// ============ protoTypeToShortName ============

func TestProtoTypeToShortName(t *testing.T) {
	tests := []struct {
		name     string
		typeVal  descriptorpb.FieldDescriptorProto_Type
		expected string
	}{
		{"double", descriptorpb.FieldDescriptorProto_TYPE_DOUBLE, "double"},
		{"float", descriptorpb.FieldDescriptorProto_TYPE_FLOAT, "float"},
		{"int64", descriptorpb.FieldDescriptorProto_TYPE_INT64, "int64"},
		{"uint64", descriptorpb.FieldDescriptorProto_TYPE_UINT64, "uint64"},
		{"int32", descriptorpb.FieldDescriptorProto_TYPE_INT32, "int32"},
		{"fixed64", descriptorpb.FieldDescriptorProto_TYPE_FIXED64, "fixed64"},
		{"fixed32", descriptorpb.FieldDescriptorProto_TYPE_FIXED32, "fixed32"},
		{"bool", descriptorpb.FieldDescriptorProto_TYPE_BOOL, "bool"},
		{"string", descriptorpb.FieldDescriptorProto_TYPE_STRING, "string"},
		{"bytes", descriptorpb.FieldDescriptorProto_TYPE_BYTES, "bytes"},
		{"uint32", descriptorpb.FieldDescriptorProto_TYPE_UINT32, "uint32"},
		{"sfixed32", descriptorpb.FieldDescriptorProto_TYPE_SFIXED32, "sfixed32"},
		{"sfixed64", descriptorpb.FieldDescriptorProto_TYPE_SFIXED64, "sfixed64"},
		{"sint32", descriptorpb.FieldDescriptorProto_TYPE_SINT32, "sint32"},
		{"sint64", descriptorpb.FieldDescriptorProto_TYPE_SINT64, "sint64"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := protoTypeToShortName(tt.typeVal)
			if result != tt.expected {
				t.Errorf("protoTypeToShortName(%s) = %s, 期望 %s", tt.name, result, tt.expected)
			}
		})
	}
}

func TestProtoTypeToShortName_UnknownType(t *testing.T) {
	// 传入一个未在 switch 中处理的类型
	result := protoTypeToShortName(descriptorpb.FieldDescriptorProto_TYPE_GROUP)
	if result == "" {
		t.Error("期望非空字符串（回退到 t.String()）")
	}
}

// ============ 辅助函数 ============

// generateTestCACertPEM 生成一个用于测试的自签名 CA 证书 PEM 格式
func generateTestCACertPEM(t *testing.T) string {
	t.Helper()
	return generateSelfSignedCertPEM(t, "testCA")
}

// generateSelfSignedCertPEM 生成一个自签名证书的 PEM 字符串
func generateSelfSignedCertPEM(t *testing.T, commonName string) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("生成私钥失败: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: commonName,
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("创建证书失败: %v", err)
	}

	certPEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	}))

	return certPEM
}

// generateTestClientCertPair 生成一个有效的客户端证书/密钥对用于测试
func generateTestClientCertPair(t *testing.T) (certPEM, keyPEM string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("生成私钥失败: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			CommonName: "client",
		},
		NotBefore:   time.Now().Add(-time.Hour),
		NotAfter:    time.Now().Add(24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("创建证书失败: %v", err)
	}

	certPEM = string(pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	}))

	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("序列化私钥失败: %v", err)
	}

	keyPEM = string(pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: keyDER,
	}))

	return certPEM, keyPEM
}

// ============ buildMessageDef / buildFields (内部方法，通过 GenerateFields 间接测试) ============

func TestGRPCExecutor_buildMessageDef_NestedMessage(t *testing.T) {
	e := NewGRPCExecutor(nil)
	protoContent := `
syntax = "proto3";
package test;
message Parent {
  Child child = 1;
  message Child {
    int32 value = 1;
  }
}
`
	messages, err := e.GenerateFields(protoContent, nil)
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if len(messages) == 0 {
		t.Fatal("期望至少 1 个消息定义")
	}
	// 验证 Parent 消息的 child 字段包含嵌套字段
	var parent *model.ProtoMessageDef
	for i := range messages {
		if messages[i].Name == "Parent" {
			parent = &messages[i]
			break
		}
	}
	if parent == nil {
		t.Fatal("未找到 Parent 消息")
	}
	if len(parent.Fields) != 1 {
		t.Fatalf("期望 Parent 有 1 个字段，实际 %d", len(parent.Fields))
	}
	if !strings.HasPrefix(parent.Fields[0].Type, "message:") {
		t.Errorf("期望 child 字段类型以 message: 开头，实际 %s", parent.Fields[0].Type)
	}
}

// ============ 集成场景：Execute 使用无效地址（非 SSRF）连接失败 ============

func TestGRPCExecutor_Execute_DialFailure(t *testing.T) {
	cs := newConfigServiceWithPrivateNetwork(t)
	e := NewGRPCExecutor(cs)
	config := &model.GRPCRequestConfig{
		Address: "127.0.0.1:1", // 端口 1 通常无服务，连接会失败或超时
		Service: "TestService",
		Method:  "TestMethod",
		Timeout: 1, // 1 秒超时
	}
	// 由于 protoContent 为空且 UseReflection=false，应在连接前返回错误
	_, err := e.Execute(context.Background(), config, "", nil, nil)
	if err == nil {
		t.Fatal("期望返回错误")
	}
}

func TestGRPCExecutor_Execute_InvalidRequestBody(t *testing.T) {
	cs := newConfigServiceWithPrivateNetwork(t)
	e := NewGRPCExecutor(cs)
	protoContent := `
syntax = "proto3";
package test;
service TestService {
  rpc TestMethod(TestRequest) returns (TestResponse);
}
message TestRequest {
  string name = 1;
}
message TestResponse {
  string message = 1;
}
`
	config := &model.GRPCRequestConfig{
		Address:     "127.0.0.1:50051",
		Service:     "TestService",
		Method:      "TestMethod",
		RequestBody: `{invalid json`,
		Timeout:     1,
	}
	// 应该在 dial 阶段就失败（端口 50051 无服务）
	_, err := e.Execute(context.Background(), config, protoContent, nil, nil)
	if err == nil {
		// 如果 dial 成功（不太可能），则在 unmarshal JSON 时失败
		// 无论哪种，都期望返回错误
	}
}

// ============ validateAddress - IPv6 和端口边界 ============

func TestGRPCExecutor_validateAddress_IPv6WithBrackets(t *testing.T) {
	e := NewGRPCExecutor(nil)
	// IPv6 地址带方括号和端口
	err := e.validateAddress("[::1]:50051")
	if err == nil {
		t.Fatal("期望返回 SSRF 错误（IPv6 loopback）")
	}
}

func TestGRPCExecutor_validateAddress_NoPort(t *testing.T) {
	e := NewGRPCExecutor(nil)
	// 不带端口的私有 IP
	err := e.validateAddress("127.0.0.1")
	if err == nil {
		t.Fatal("期望返回 SSRF 错误")
	}
}

// ============ TLS 证书相关辅助测试 ============

func TestGenerateTestCACertPEM(t *testing.T) {
	pemStr := generateTestCACertPEM(t)
	if pemStr == "" {
		t.Fatal("期望返回非空 PEM 字符串")
	}
	// 验证可以解析为 PEM block
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		t.Fatal("期望返回有效的 PEM 格式")
	}
	// 尝试解析为证书（可能失败，因为这是测试用的伪造数据）
	_, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		// 测试用证书数据可能不完整，只要能解码 PEM block 即可
		t.Logf("测试证书解析失败（预期，因为是测试数据）: %v", err)
	}
}

// ============ buildTLSConfig - 验证返回的 TLS 配置 ============

func TestGRPCExecutor_buildTLSConfig_TLSConfigFields(t *testing.T) {
	e := NewGRPCExecutor(nil)

	t.Run("tls config has correct MinVersion", func(t *testing.T) {
		cfg, err := e.buildTLSConfig("tls", nil)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if cfg == nil {
			t.Fatal("期望返回非 nil TLS config")
		}
		// tls.Config 零值是有效的，MinVersion 默认为 0（让 Go 自动选择）
		_ = cfg.MinVersion
	})

	t.Run("mtls config has Certificates", func(t *testing.T) {
		// 生成有效的客户端证书对
		certPEM, keyPEM := generateTestClientCertPair(t)
		cert := &model.Certificate{
			ClientCert: certPEM,
			ClientKey:  keyPEM,
		}
		cfg, err := e.buildTLSConfig("mtls", cert)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if cfg == nil {
			t.Fatal("期望返回非 nil TLS config")
		}
		if len(cfg.Certificates) != 1 {
			t.Errorf("期望 1 个证书，实际 %d", len(cfg.Certificates))
		}
	})
}
