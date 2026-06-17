package service

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"github.com/jhump/protoreflect/grpcreflect"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	sysconfig "github.com/lynnyq/bdopsflow/scheduler/internal/system_config"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// GRPCExecutor handles executing gRPC requests using dynamic invocation via protoreflect.
type GRPCExecutor struct {
	configService *sysconfig.Service
}

// NewGRPCExecutor creates a new GRPCExecutor instance.
func NewGRPCExecutor(configService *sysconfig.Service) *GRPCExecutor {
	return &GRPCExecutor{configService: configService}
}

// defaultGRPCTimeout is the default timeout for gRPC requests.
const defaultGRPCTimeout = 30 * time.Second

// maxGRPCTimeout is the maximum allowed timeout for gRPC requests.
const maxGRPCTimeout = 300 * time.Second

// Execute executes a gRPC request using dynamic invocation and returns the result.
func (e *GRPCExecutor) Execute(ctx context.Context, config *model.GRPCRequestConfig, protoContent string, depContents map[string]string, cert *model.Certificate) (*model.ApiTestResult, error) {
	// 1. Validate address (SSRF protection)
	if err := e.validateAddress(config.Address); err != nil {
		return nil, fmt.Errorf("address validation failed: %w", err)
	}

	// 2. Build dial options and connect
	dialOpts, err := e.buildDialOptions(config.TLSMode, cert)
	if err != nil {
		return nil, fmt.Errorf("failed to build dial options: %w", err)
	}

	timeout := defaultGRPCTimeout
	if config.Timeout > 0 {
		timeout = time.Duration(config.Timeout) * time.Second
		if timeout > maxGRPCTimeout {
			timeout = maxGRPCTimeout
		}
	}

	connCtx, connCancel := context.WithTimeout(ctx, timeout)
	defer connCancel()

	conn, err := grpc.DialContext(connCtx, config.Address, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to dial gRPC server %s: %w", config.Address, err)
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			slog.Warn("failed to close gRPC connection", "address", config.Address, "error", closeErr)
		}
	}()

	// 3. Build metadata from config
	mdPairs := make([]string, 0, len(config.Metadata)*2)
	for _, kv := range config.Metadata {
		mdPairs = append(mdPairs, kv.Key, kv.Value)
	}
	md := metadata.Pairs(mdPairs...)

	// Create context with metadata and timeout
	reqCtx, reqCancel := context.WithTimeout(metadata.NewOutgoingContext(ctx, md), timeout)
	defer reqCancel()

	// 4. Get method descriptor
	var methodDesc *desc.MethodDescriptor

	if config.UseReflection {
		// Use server reflection to resolve the method
		refClient := grpcreflect.NewClientAuto(reqCtx, conn)
		svcDesc, resolveErr := refClient.ResolveService(config.Service)
		if resolveErr != nil {
			return nil, fmt.Errorf("failed to resolve service %s via reflection: %w", config.Service, resolveErr)
		}
		methodDesc = svcDesc.FindMethodByName(config.Method)
		if methodDesc == nil {
			return nil, fmt.Errorf("method %s not found in service %s", config.Method, config.Service)
		}
	} else if protoContent != "" {
		// Parse proto file to get method descriptor
		methodDesc, err = e.parseMethodFromProto(protoContent, depContents, config.Service, config.Method)
		if err != nil {
			return nil, fmt.Errorf("failed to parse method from proto file: %w", err)
		}
	} else {
		return nil, fmt.Errorf("either UseReflection must be true or a proto file must be provided")
	}

	// 5. Create dynamic stub and invoke
	stub := grpcdynamic.NewStub(conn)

	// Create dynamic message from input type
	reqMsg := dynamic.NewMessage(methodDesc.GetInputType())

	requestBody := config.RequestBody
	if requestBody == "" {
		requestBody = "{}"
	}

	if unmarshalErr := reqMsg.UnmarshalJSON([]byte(requestBody)); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to unmarshal request body into dynamic message: %w", unmarshalErr)
	}

	// Capture headers and trailers
	var header, trailer metadata.MD

	// Measure latency
	start := time.Now()

	respMsg, invokeErr := stub.InvokeRpc(reqCtx, methodDesc, reqMsg,
		grpc.Header(&header), grpc.Trailer(&trailer))

	latencyMs := time.Since(start).Milliseconds()

	result := &model.ApiTestResult{
		Type:      "grpc",
		LatencyMs: latencyMs,
	}

	// Build headers JSON from header + trailer
	headersMap := make(map[string][]string)
	for k, v := range header {
		headersMap[k] = v
	}
	for k, v := range trailer {
		headersMap[k] = append(headersMap[k], v...)
	}
	headersJSON, marshalErr := json.Marshal(headersMap)
	if marshalErr != nil {
		slog.Warn("failed to marshal gRPC headers to JSON", "error", marshalErr)
		headersJSON = []byte("{}")
	}
	result.Headers = string(headersJSON)

	if invokeErr != nil {
		result.StatusCode = e.extractGRPCStatusCode(invokeErr)
		result.Error = invokeErr.Error()
		slog.Warn("gRPC invocation failed",
			"address", config.Address,
			"service", config.Service,
			"method", config.Method,
			"error", invokeErr,
			"latency_ms", latencyMs,
		)
		return result, nil
	}

	result.StatusCode = 0 // gRPC OK

	// Marshal response dynamic message to JSON
	if respMsg != nil {
		if dynMsg, ok := respMsg.(*dynamic.Message); ok {
			respJSON, marshalErr := dynMsg.MarshalJSON()
			if marshalErr != nil {
				slog.Warn("failed to marshal gRPC response to JSON", "error", marshalErr)
				result.Body = fmt.Sprintf(`{"error": "failed to marshal response: %s"}`, marshalErr.Error())
			} else {
				result.Body = string(respJSON)
			}
		} else {
			result.Body = fmt.Sprintf(`{"error": "unexpected response type: %T"}`, respMsg)
		}
	}

	slog.Info("gRPC invocation succeeded",
		"address", config.Address,
		"service", config.Service,
		"method", config.Method,
		"latency_ms", latencyMs,
	)

	return result, nil
}

// ConnectTest tests connectivity to a gRPC server by dialing and verifying the connection.
func (e *GRPCExecutor) ConnectTest(ctx context.Context, config *model.GRPCRequestConfig, cert *model.Certificate) (*model.ApiTestResult, error) {
	// 1. Validate address (SSRF protection)
	if err := e.validateAddress(config.Address); err != nil {
		return nil, fmt.Errorf("address validation failed: %w", err)
	}

	// 2. For TLS/mtls mode, validate certificate requirement
	if config.TLSMode == "mtls" && cert == nil {
		return &model.ApiTestResult{
			Type:  "grpc",
			Error: "mTLS 模式需要选择客户端证书",
		}, nil
	}

	// 3. Build dial options and connect
	dialOpts, err := e.buildDialOptions(config.TLSMode, cert)
	if err != nil {
		return nil, fmt.Errorf("failed to build dial options: %w", err)
	}

	timeout := defaultGRPCTimeout
	if config.Timeout > 0 {
		timeout = time.Duration(config.Timeout) * time.Second
		if timeout > maxGRPCTimeout {
			timeout = maxGRPCTimeout
		}
	}

	connCtx, connCancel := context.WithTimeout(ctx, timeout)
	defer connCancel()

	start := time.Now()
	conn, err := grpc.DialContext(connCtx, config.Address, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to dial gRPC server %s: %w", config.Address, err)
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			slog.Warn("failed to close gRPC connection", "address", config.Address, "error", closeErr)
		}
	}()

	// Wait for connection to reach a terminal state (READY or TRANSIENT_FAILURE).
	// Loop on state changes until we reach a conclusive state or timeout.
	for {
		state := conn.GetState()
		switch state {
		case connectivity.Ready:
			latencyMs := time.Since(start).Milliseconds()
			slog.Info("gRPC connect test succeeded", "address", config.Address, "latency_ms", latencyMs)
			return &model.ApiTestResult{
				Type:      "grpc",
				LatencyMs: latencyMs,
				Body:      fmt.Sprintf(`{"status":"connected","address":"%s"}`, config.Address),
			}, nil
		case connectivity.TransientFailure:
			latencyMs := time.Since(start).Milliseconds()
			errMsg := fmt.Sprintf("连接失败: %s", config.Address)
			if config.TLSMode == "tls" {
				errMsg = fmt.Sprintf("TLS连接失败: %s，请检查TLS配置和证书是否正确", config.Address)
			} else if config.TLSMode == "mtls" {
				errMsg = fmt.Sprintf("mTLS连接失败: %s，请检查客户端证书是否正确", config.Address)
			}
			return &model.ApiTestResult{
				Type:      "grpc",
				LatencyMs: latencyMs,
				Error:     errMsg,
			}, nil
		}

		// Wait for state change (IDLE, CONNECTING are non-terminal, keep waiting)
		if !conn.WaitForStateChange(connCtx, state) {
			// Context timeout — connection didn't reach a terminal state in time
			latencyMs := time.Since(start).Milliseconds()
			return &model.ApiTestResult{
				Type:      "grpc",
				LatencyMs: latencyMs,
				Error:     fmt.Sprintf("连接超时: %s", config.Address),
			}, nil
		}
	}
}

// GenerateFields parses proto content and returns detailed message field definitions for all messages.
func (e *GRPCExecutor) GenerateFields(protoContent string, depContents map[string]string) ([]model.ProtoMessageDef, error) {
	filesMap := make(map[string]string)
	for name, content := range depContents {
		filesMap[name] = content
	}
	filesMap["input.proto"] = protoContent

	parser := protoparse.Parser{
		Accessor: protoparse.FileContentsFromMap(filesMap),
	}

	files, err := parser.ParseFiles("input.proto")
	if err != nil {
		return nil, fmt.Errorf("failed to parse proto file: %w", err)
	}

	var messages []model.ProtoMessageDef
	for _, fd := range files {
		for _, md := range fd.GetMessageTypes() {
			messages = append(messages, e.buildMessageDef(md))
		}
	}

	return messages, nil
}

// buildMessageDef recursively builds a ProtoMessageDef from a desc.MessageDescriptor.
func (e *GRPCExecutor) buildMessageDef(md *desc.MessageDescriptor) model.ProtoMessageDef {
	msgDef := model.ProtoMessageDef{
		Name:     md.GetName(),
		FullName: md.GetFullyQualifiedName(),
		Fields:   e.buildFields(md.GetFields()),
	}
	return msgDef
}

// buildFields converts field descriptors to ProtoMessageField list.
func (e *GRPCExecutor) buildFields(fields []*desc.FieldDescriptor) []model.ProtoMessageField {
	result := make([]model.ProtoMessageField, 0, len(fields))
	for _, f := range fields {
		field := model.ProtoMessageField{
			Name:   f.GetName(),
			Number: int(f.GetNumber()),
		}

		// Label
		if f.IsRepeated() {
			if f.IsMap() {
				field.Label = "map"
			} else {
				field.Label = "repeated"
			}
		} else {
			field.Label = "optional"
		}

		// Type
		if f.GetMessageType() != nil {
			msgType := f.GetMessageType()
			field.Type = "message:" + msgType.GetFullyQualifiedName()
			// Inline nested message fields for form generation
			field.Fields = e.buildFields(msgType.GetFields())
		} else if f.GetEnumType() != nil {
			field.Type = "enum:" + f.GetEnumType().GetFullyQualifiedName()
		} else {
			field.Type = protoTypeToShortName(f.GetType())
		}

		// Map key/value
		if f.IsMap() {
			mapFields := f.GetMessageType().GetFields()
			if len(mapFields) >= 2 {
				field.MapKey = protoTypeToShortName(mapFields[0].GetType())
				if mapFields[1].GetMessageType() != nil {
					field.MapValue = "message:" + mapFields[1].GetMessageType().GetFullyQualifiedName()
				} else if mapFields[1].GetEnumType() != nil {
					field.MapValue = "enum:" + mapFields[1].GetEnumType().GetFullyQualifiedName()
				} else {
					field.MapValue = protoTypeToShortName(mapFields[1].GetType())
				}
			}
		}

		result = append(result, field)
	}
	return result
}

// protoTypeToShortName converts a FieldDescriptorProto_Type (e.g. TYPE_STRING) to short name (e.g. string).
func protoTypeToShortName(t descriptorpb.FieldDescriptorProto_Type) string {
	switch t {
	case descriptorpb.FieldDescriptorProto_TYPE_DOUBLE:
		return "double"
	case descriptorpb.FieldDescriptorProto_TYPE_FLOAT:
		return "float"
	case descriptorpb.FieldDescriptorProto_TYPE_INT64:
		return "int64"
	case descriptorpb.FieldDescriptorProto_TYPE_UINT64:
		return "uint64"
	case descriptorpb.FieldDescriptorProto_TYPE_INT32:
		return "int32"
	case descriptorpb.FieldDescriptorProto_TYPE_FIXED64:
		return "fixed64"
	case descriptorpb.FieldDescriptorProto_TYPE_FIXED32:
		return "fixed32"
	case descriptorpb.FieldDescriptorProto_TYPE_BOOL:
		return "bool"
	case descriptorpb.FieldDescriptorProto_TYPE_STRING:
		return "string"
	case descriptorpb.FieldDescriptorProto_TYPE_BYTES:
		return "bytes"
	case descriptorpb.FieldDescriptorProto_TYPE_UINT32:
		return "uint32"
	case descriptorpb.FieldDescriptorProto_TYPE_SFIXED32:
		return "sfixed32"
	case descriptorpb.FieldDescriptorProto_TYPE_SFIXED64:
		return "sfixed64"
	case descriptorpb.FieldDescriptorProto_TYPE_SINT32:
		return "sint32"
	case descriptorpb.FieldDescriptorProto_TYPE_SINT64:
		return "sint64"
	default:
		return t.String()
	}
}

// GenerateTemplate generates a JSON request body template for the given service and method.
func (e *GRPCExecutor) GenerateTemplate(protoContent string, depContents map[string]string, serviceName string, methodName string) (string, error) {
	methodDesc, err := e.parseMethodFromProto(protoContent, depContents, serviceName, methodName)
	if err != nil {
		return "", err
	}

	inputMsg := dynamic.NewMessage(methodDesc.GetInputType())
	templateJSON, err := inputMsg.MarshalJSON()
	if err != nil {
		return "", fmt.Errorf("failed to marshal template: %w", err)
	}

	return string(templateJSON), nil
}

// ReflectServices uses gRPC Server Reflection to discover services on a target server.
func (e *GRPCExecutor) ReflectServices(ctx context.Context, address string, tlsMode string, cert *model.Certificate) ([]model.ProtoService, error) {
	// Validate address (SSRF protection)
	if err := e.validateAddress(address); err != nil {
		return nil, fmt.Errorf("address validation failed: %w", err)
	}

	dialOpts, err := e.buildDialOptions(tlsMode, cert)
	if err != nil {
		return nil, fmt.Errorf("failed to build dial options: %w", err)
	}

	connCtx, connCancel := context.WithTimeout(ctx, defaultGRPCTimeout)
	defer connCancel()

	conn, err := grpc.DialContext(connCtx, address, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to dial gRPC server %s: %w", address, err)
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			slog.Warn("failed to close gRPC connection", "address", address, "error", closeErr)
		}
	}()

	// Use grpcreflect client to list services
	refClient := grpcreflect.NewClientAuto(ctx, conn)

	serviceNames, err := refClient.ListServices()
	if err != nil {
		return nil, fmt.Errorf("failed to list services via reflection for %s: %w", address, err)
	}

	var services []model.ProtoService
	for _, svcName := range serviceNames {
		svcDesc, resolveErr := refClient.ResolveService(svcName)
		if resolveErr != nil {
			slog.Warn("failed to resolve service via reflection",
				"service", svcName,
				"address", address,
				"error", resolveErr,
			)
			services = append(services, model.ProtoService{Name: svcName})
			continue
		}

		methods := make([]model.ProtoMethod, 0, len(svcDesc.GetMethods()))
		for _, m := range svcDesc.GetMethods() {
			methods = append(methods, model.ProtoMethod{
				Name:         m.GetName(),
				InputType:    m.GetInputType().GetFullyQualifiedName(),
				OutputType:   m.GetOutputType().GetFullyQualifiedName(),
				ClientStream: m.IsClientStreaming(),
				ServerStream: m.IsServerStreaming(),
			})
		}

		services = append(services, model.ProtoService{
			Name:    svcName,
			Methods: methods,
		})
	}

	return services, nil
}

// parseMethodFromProto parses proto content and returns the method descriptor for the given service and method.
func (e *GRPCExecutor) parseMethodFromProto(protoContent string, depContents map[string]string, serviceName string, methodName string) (*desc.MethodDescriptor, error) {
	// Build a file map for the parser: main file + dependencies
	filesMap := make(map[string]string)
	for name, content := range depContents {
		filesMap[name] = content
	}
	filesMap["input.proto"] = protoContent

	parser := protoparse.Parser{
		Accessor: protoparse.FileContentsFromMap(filesMap),
	}

	// Parse the main proto file
	files, err := parser.ParseFiles("input.proto")
	if err != nil {
		return nil, fmt.Errorf("failed to parse proto file: %w", err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no file descriptors returned from proto parsing")
	}

	// Find the service and method
	// Match by fully qualified name (e.g. "bdopsflow.ExecutorService") or short name (e.g. "ExecutorService")
	for _, fd := range files {
		for _, svc := range fd.GetServices() {
			fqn := svc.GetFullyQualifiedName()
			if fqn != serviceName && svc.GetName() != serviceName {
				continue
			}
			method := svc.FindMethodByName(methodName)
			if method == nil {
				return nil, fmt.Errorf("method %s not found in service %s", methodName, fqn)
			}
			return method, nil
		}
	}

	return nil, fmt.Errorf("service %s not found in proto file", serviceName)
}

// validateAddress validates that the gRPC server address does not point to a private/reserved IP.
// If api_test.allow_private_network is enabled in system config, private IPs are allowed.
func (e *GRPCExecutor) validateAddress(address string) error {
	// Check if private network access is allowed via system config
	if e.configService != nil && e.configService.GetBool("api_test.allow_private_network") {
		return nil
	}

	// Extract host from address (may include port)
	host := address
	if idx := strings.LastIndex(address, ":"); idx > 0 {
		host = address[:idx]
	}

	// Remove brackets from IPv6 addresses
	host = strings.TrimPrefix(host, "[")
	host = strings.TrimSuffix(host, "]")

	// Resolve hostname to IP addresses
	ips, err := net.LookupIP(host)
	if err != nil {
		// If resolution fails, try parsing as IP directly
		ip := net.ParseIP(host)
		if ip == nil {
			return fmt.Errorf("cannot resolve address: %s", address)
		}
		ips = []net.IP{ip}
	}

	for _, ip := range ips {
		if isPrivateIP(ip) {
			return fmt.Errorf("address %s resolves to private/reserved IP %s, which is not allowed", address, ip)
		}
	}

	return nil
}

// isPrivateIP checks if an IP address is private or reserved.
func isPrivateIP(ip net.IP) bool {
	privateRanges := []struct {
		network *net.IPNet
	}{
		{mustParseCIDR("10.0.0.0/8")},
		{mustParseCIDR("172.16.0.0/12")},
		{mustParseCIDR("192.168.0.0/16")},
		{mustParseCIDR("127.0.0.0/8")},
		{mustParseCIDR("169.254.0.0/16")},
		{mustParseCIDR("::1/128")},
		{mustParseCIDR("fc00::/7")},
		{mustParseCIDR("fe80::/10")},
	}

	for _, r := range privateRanges {
		if r.network.Contains(ip) {
			return true
		}
	}

	return false
}

// mustParseCIDR parses a CIDR string or panics.
func mustParseCIDR(s string) *net.IPNet {
	_, network, err := net.ParseCIDR(s)
	if err != nil {
		panic(fmt.Sprintf("invalid CIDR %s: %v", s, err))
	}
	return network
}

// buildDialOptions constructs gRPC dial options based on TLS mode.
func (e *GRPCExecutor) buildDialOptions(tlsMode string, cert *model.Certificate) ([]grpc.DialOption, error) {
	switch tlsMode {
	case "insecure", "":
		return []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		}, nil
	case "tls":
		tlsCfg := &tls.Config{}
		// If a certificate is provided, use its CA cert for server verification
		if cert != nil && cert.CaCert != "" {
			caCertPool := x509.NewCertPool()
			if !caCertPool.AppendCertsFromPEM([]byte(cert.CaCert)) {
				return nil, fmt.Errorf("failed to append CA certificate to pool")
			}
			tlsCfg.RootCAs = caCertPool
		}
		return []grpc.DialOption{
			grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)),
		}, nil
	case "mtls":
		tlsCfg, err := e.buildTLSConfig(tlsMode, cert)
		if err != nil {
			return nil, fmt.Errorf("failed to build mTLS config: %w", err)
		}
		return []grpc.DialOption{
			grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported TLS mode: %s", tlsMode)
	}
}

// buildTLSConfig builds a TLS configuration based on the TLS mode and certificate.
func (e *GRPCExecutor) buildTLSConfig(tlsMode string, cert *model.Certificate) (*tls.Config, error) {
	switch tlsMode {
	case "insecure", "":
		return nil, nil
	case "tls":
		return &tls.Config{}, nil
	case "mtls":
		if cert == nil {
			return nil, fmt.Errorf("certificate is required for mTLS mode")
		}

		// Create CA cert pool
		caCertPool := x509.NewCertPool()
		if cert.CaCert != "" {
			if !caCertPool.AppendCertsFromPEM([]byte(cert.CaCert)) {
				return nil, fmt.Errorf("failed to append CA certificate to pool")
			}
		}

		// Load client certificate and key
		if cert.ClientCert == "" || cert.ClientKey == "" {
			return nil, fmt.Errorf("client certificate and key are required for mTLS mode")
		}

		certPair, err := tls.X509KeyPair([]byte(cert.ClientCert), []byte(cert.ClientKey))
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate key pair: %w", err)
		}

		return &tls.Config{
			RootCAs:      caCertPool,
			Certificates: []tls.Certificate{certPair},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported TLS mode: %s", tlsMode)
	}
}

// extractGRPCStatusCode extracts the gRPC status code from an error.
func (e *GRPCExecutor) extractGRPCStatusCode(err error) int {
	if err == nil {
		return 0
	}
	st, ok := status.FromError(err)
	if !ok {
		slog.Warn("failed to extract gRPC status code from error", "error", err)
		return 2 // Unknown as fallback
	}
	return int(st.Code())
}
