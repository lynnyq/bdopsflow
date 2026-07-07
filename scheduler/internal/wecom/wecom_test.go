package wecom

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/lynnyq/bdopsflow/scheduler/internal/system_config"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
	rqlite "github.com/rqlite/gorqlite"
)

// mockDB 实现 database.DB 接口，用于创建 sysconfig.Service。
// wocom 测试不需要真实的数据库行，只需 Service 能正常初始化即可。
type mockDB struct {
	mu sync.Mutex
}

func (m *mockDB) QueryOne(query string) (rqlite.QueryResult, error) {
	return rqlite.QueryResult{}, nil
}

func (m *mockDB) QueryOneParameterized(stmt rqlite.ParameterizedStatement) (rqlite.QueryResult, error) {
	return rqlite.QueryResult{}, nil
}

func (m *mockDB) WriteOneParameterized(stmt rqlite.ParameterizedStatement) (rqlite.WriteResult, error) {
	return rqlite.WriteResult{}, nil
}

func (m *mockDB) WriteParameterized(stmts []rqlite.ParameterizedStatement) ([]rqlite.WriteResult, error) {
	results := make([]rqlite.WriteResult, len(stmts))
	return results, nil
}

// 编译期接口检查
var _ database.DB = (*mockDB)(nil)

// newTestWeComService 创建用于测试的 WeComService，使用 mockDB 和指定的 URL。
// 返回的 cleanup 用于关闭后台 goroutine。
func newTestWeComService(t *testing.T, robotURL, appMsgURL, ewechatURL string) (*WeComService, *system_config.Service) {
	t.Helper()

	db := &mockDB{}
	svc := system_config.NewService(db)
	t.Cleanup(svc.Close)

	// 覆盖配置值
	if robotURL != "" {
		svc.Set(t.Context(), "wecom.robot_url", robotURL, 1)
	}
	if appMsgURL != "" {
		svc.Set(t.Context(), "wecom.app_msg_url", appMsgURL, 1)
	}
	if ewechatURL != "" {
		svc.Set(t.Context(), "wecom.ewechat_url", ewechatURL, 1)
	}

	wecomSvc := NewService(svc)
	return wecomSvc, svc
}

// ------------------------------------------------------------
// NewService / refreshRuntimeConfig
// ------------------------------------------------------------

func TestNewService_DefaultURLs(t *testing.T) {
	db := &mockDB{}
	svc := system_config.NewService(db)
	defer svc.Close()

	wecomSvc := NewService(svc)
	if wecomSvc == nil {
		t.Fatal("NewService should return non-nil service")
	}

	// 验证默认 URL 被正确加载（来自 defaultConfigValues）
	wecomSvc.mu.RLock()
	defer wecomSvc.mu.RUnlock()

	if wecomSvc.runtimeRobotURL != "https://qyapi.weixin.qq.com/cgi-bin/webhook/send" {
		t.Errorf("runtimeRobotURL = %v, want https://qyapi.weixin.qq.com/cgi-bin/webhook/send", wecomSvc.runtimeRobotURL)
	}
	if wecomSvc.runtimeAppMsgURL != "https://qyapi.weixin.qq.com/cgi-bin/app/send" {
		t.Errorf("runtimeAppMsgURL = %v, want https://qyapi.weixin.qq.com/cgi-bin/app/send", wecomSvc.runtimeAppMsgURL)
	}
	if wecomSvc.runtimeEwechatURL != "https://qyapi.weixin.qq.com/cgi-bin/app/send" {
		t.Errorf("runtimeEwechatURL = %v, want https://qyapi.weixin.qq.com/cgi-bin/app/send", wecomSvc.runtimeEwechatURL)
	}
}

func TestNewService_WithCustomURLs(t *testing.T) {
	db := &mockDB{}
	svc := system_config.NewService(db)
	defer svc.Close()

	// 设置自定义 URL
	svc.Set(t.Context(), "wecom.robot_url", "http://custom-robot.example.com", 1)
	svc.Set(t.Context(), "wecom.app_msg_url", "http://custom-app.example.com", 1)
	svc.Set(t.Context(), "wecom.ewechat_url", "http://custom-ewechat.example.com", 1)

	wecomSvc := NewService(svc)

	wecomSvc.mu.RLock()
	defer wecomSvc.mu.RUnlock()

	if wecomSvc.runtimeRobotURL != "http://custom-robot.example.com" {
		t.Errorf("runtimeRobotURL = %v, want http://custom-robot.example.com", wecomSvc.runtimeRobotURL)
	}
	if wecomSvc.runtimeAppMsgURL != "http://custom-app.example.com" {
		t.Errorf("runtimeAppMsgURL = %v, want http://custom-app.example.com", wecomSvc.runtimeAppMsgURL)
	}
	if wecomSvc.runtimeEwechatURL != "http://custom-ewechat.example.com" {
		t.Errorf("runtimeEwechatURL = %v, want http://custom-ewechat.example.com", wecomSvc.runtimeEwechatURL)
	}
}

// ------------------------------------------------------------
// OnConfigChanged
// ------------------------------------------------------------

func TestOnConfigChanged_RelevantKeys(t *testing.T) {
	db := &mockDB{}
	svc := system_config.NewService(db)
	defer svc.Close()

	wecomSvc := NewService(svc)

	// 设置新的 URL 到 config（但 wecomSvc 缓存还没更新）
	svc.Set(t.Context(), "wecom.robot_url", "http://updated-robot.example.com", 1)

	// 触发配置变更通知
	wecomSvc.OnConfigChanged("wecom.robot_url", "http://updated-robot.example.com")

	wecomSvc.mu.RLock()
	defer wecomSvc.mu.RUnlock()

	if wecomSvc.runtimeRobotURL != "http://updated-robot.example.com" {
		t.Errorf("runtimeRobotURL = %v, want http://updated-robot.example.com", wecomSvc.runtimeRobotURL)
	}
}

func TestOnConfigChanged_IrrelevantKey(t *testing.T) {
	db := &mockDB{}
	svc := system_config.NewService(db)
	defer svc.Close()

	wecomSvc := NewService(svc)

	// 记录原始 URL
	wecomSvc.mu.RLock()
	originalURL := wecomSvc.runtimeRobotURL
	wecomSvc.mu.RUnlock()

	// 触发不相关的配置变更通知
	wecomSvc.OnConfigChanged("datasource.cache_ttl", "600")

	// URL 不应变
	wecomSvc.mu.RLock()
	defer wecomSvc.mu.RUnlock()
	if wecomSvc.runtimeRobotURL != originalURL {
		t.Errorf("runtimeRobotURL should not change for irrelevant key, got %v", wecomSvc.runtimeRobotURL)
	}
}

func TestOnConfigChanged_AllThreeKeys(t *testing.T) {
	db := &mockDB{}
	svc := system_config.NewService(db)
	defer svc.Close()

	wecomSvc := NewService(svc)

	keys := []struct {
		key, url string
	}{
		{"wecom.robot_url", "http://new-robot.example.com"},
		{"wecom.app_msg_url", "http://new-app.example.com"},
		{"wecom.ewechat_url", "http://new-ewechat.example.com"},
	}

	for _, k := range keys {
		svc.Set(t.Context(), k.key, k.url, 1)
		wecomSvc.OnConfigChanged(k.key, k.url)
	}

	wecomSvc.mu.RLock()
	defer wecomSvc.mu.RUnlock()

	if wecomSvc.runtimeRobotURL != "http://new-robot.example.com" {
		t.Errorf("runtimeRobotURL = %v", wecomSvc.runtimeRobotURL)
	}
	if wecomSvc.runtimeAppMsgURL != "http://new-app.example.com" {
		t.Errorf("runtimeAppMsgURL = %v", wecomSvc.runtimeAppMsgURL)
	}
	if wecomSvc.runtimeEwechatURL != "http://new-ewechat.example.com" {
		t.Errorf("runtimeEwechatURL = %v", wecomSvc.runtimeEwechatURL)
	}
}

// ------------------------------------------------------------
// HTTP 请求测试（使用 httptest.Server）
// ------------------------------------------------------------

// newTestServer 创建一个测试 HTTP 服务器，返回指定的响应体和状态码。
// handler 可以自定义服务器的响应逻辑。
func newTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

// successHandler 返回成功响应的 handler
func successHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"retCode": "0000",
			"retMsg":  "success",
		})
	}
}

// failHandler 返回失败响应的 handler
func failHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"retCode": "1001",
			"retMsg":  "invalid token",
		})
	}
}

// captureHandler 捕获请求体并返回成功响应
func captureHandler(t *testing.T, captured *map[string]interface{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read request body: %v", err)
		}
		_ = json.Unmarshal(body, captured)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"retCode": "0000",
			"retMsg":  "success",
		})
	}
}

func TestSendAppMsg_Success(t *testing.T) {
	srv := newTestServer(t, successHandler())
	wecomSvc, _ := newTestWeComService(t, srv.URL, srv.URL, srv.URL)

	err := wecomSvc.SendAppMsg(1000027, "markdown", "test message", []string{"13800138000"})
	if err != nil {
		t.Errorf("SendAppMsg failed: %v", err)
	}
}

func TestSendAppMsg_FailResponse(t *testing.T) {
	srv := newTestServer(t, failHandler())
	wecomSvc, _ := newTestWeComService(t, srv.URL, srv.URL, srv.URL)

	err := wecomSvc.SendAppMsg(1000027, "markdown", "test message", []string{"13800138000"})
	if err == nil {
		t.Error("SendAppMsg should fail with non-0000 retCode")
	}
}

func TestSendAppMsg_ConnectionError(t *testing.T) {
	// 使用一个不存在的地址
	wecomSvc, _ := newTestWeComService(t, "http://127.0.0.1:1", "http://127.0.0.1:1", "http://127.0.0.1:1")

	err := wecomSvc.SendAppMsg(1000027, "markdown", "test message", []string{"13800138000"})
	if err == nil {
		t.Error("SendAppMsg should fail with connection error")
	}
}

func TestSendRobotMarkdownMsg_Success(t *testing.T) {
	var captured map[string]interface{}
	srv := newTestServer(t, captureHandler(t, &captured))
	wecomSvc, _ := newTestWeComService(t, srv.URL, srv.URL, srv.URL)

	err := wecomSvc.SendRobotMarkdownMsg("test-group", "# Hello")
	if err != nil {
		t.Errorf("SendRobotMarkdownMsg failed: %v", err)
	}

	// 验证请求体包含正确的数据
	if captured["groupId"] != "test-group" {
		t.Errorf("groupId = %v, want test-group", captured["groupId"])
	}
	if captured["fromChannel"] != "HDP" {
		t.Errorf("fromChannel = %v, want HDP", captured["fromChannel"])
	}
}

func TestSendRobotMarkdownMsg_FailResponse(t *testing.T) {
	srv := newTestServer(t, failHandler())
	wecomSvc, _ := newTestWeComService(t, srv.URL, srv.URL, srv.URL)

	err := wecomSvc.SendRobotMarkdownMsg("test-group", "# Hello")
	if err == nil {
		t.Error("SendRobotMarkdownMsg should fail with non-0000 retCode")
	}
}

func TestSendRobotImageMsg_Success(t *testing.T) {
	var captured map[string]interface{}
	srv := newTestServer(t, captureHandler(t, &captured))
	wecomSvc, _ := newTestWeComService(t, srv.URL, srv.URL, srv.URL)

	imageBytes := []byte("fake-image-data")
	err := wecomSvc.SendRobotImageMsg("test-group", imageBytes)
	if err != nil {
		t.Errorf("SendRobotImageMsg failed: %v", err)
	}

	// 验证请求体包含 base64 和 md5
	reqData, ok := captured["reqData"].(map[string]interface{})
	if !ok {
		t.Fatal("reqData should be a map")
	}
	ewechatMsg, ok := reqData["ewechatMsg"].(map[string]interface{})
	if !ok {
		t.Fatal("ewechatMsg should be a map")
	}
	if ewechatMsg["base64"] == "" {
		t.Error("base64 should not be empty")
	}
	if ewechatMsg["md5"] == "" {
		t.Error("md5 should not be empty")
	}
}

func TestSendRobotTextPeopleMsg_Success(t *testing.T) {
	var captured map[string]interface{}
	srv := newTestServer(t, captureHandler(t, &captured))
	wecomSvc, _ := newTestWeComService(t, srv.URL, srv.URL, srv.URL)

	err := wecomSvc.SendRobotTextPeopleMsg("test-group", "hello @all", "13800138000")
	if err != nil {
		t.Errorf("SendRobotTextPeopleMsg failed: %v", err)
	}

	// 验证请求体
	if captured["groupId"] != "test-group" {
		t.Errorf("groupId = %v, want test-group", captured["groupId"])
	}
}

func TestSendChatMarkdownMsg_Success(t *testing.T) {
	var captured map[string]interface{}
	srv := newTestServer(t, captureHandler(t, &captured))
	wecomSvc, _ := newTestWeComService(t, srv.URL, srv.URL, srv.URL)

	err := wecomSvc.SendChatMarkdownMsg("test-chat-id", "# Group Message")
	if err != nil {
		t.Errorf("SendChatMarkdownMsg failed: %v", err)
	}

	// 验证请求体
	if captured["agentId"] != "1000027" {
		t.Errorf("agentId = %v, want 1000027", captured["agentId"])
	}
	if captured["httpMethod"] != "POST" {
		t.Errorf("httpMethod = %v, want POST", captured["httpMethod"])
	}
}

func TestSendChatMarkdownMsg_FailResponse(t *testing.T) {
	srv := newTestServer(t, failHandler())
	wecomSvc, _ := newTestWeComService(t, srv.URL, srv.URL, srv.URL)

	err := wecomSvc.SendChatMarkdownMsg("test-chat-id", "# Group Message")
	if err == nil {
		t.Error("SendChatMarkdownMsg should fail with non-0000 retCode")
	}
}

// ------------------------------------------------------------
// 带返回值的请求测试
// ------------------------------------------------------------

func TestCreateChatGroup_Success(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"retCode": "0000",
			"retMsg":  "success",
			"chatid":  "new-chat-123",
		})
	})
	wecomSvc, _ := newTestWeComService(t, srv.URL, srv.URL, srv.URL)

	result, err := wecomSvc.CreateChatGroup("test-group", []string{"user1", "user2"})
	if err != nil {
		t.Errorf("CreateChatGroup failed: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if result["chatid"] != "new-chat-123" {
		t.Errorf("chatid = %v, want new-chat-123", result["chatid"])
	}
}

func TestCreateChatGroup_FailResponse(t *testing.T) {
	srv := newTestServer(t, failHandler())
	wecomSvc, _ := newTestWeComService(t, srv.URL, srv.URL, srv.URL)

	result, err := wecomSvc.CreateChatGroup("test-group", []string{"user1"})
	if err == nil {
		t.Error("CreateChatGroup should fail with non-0000 retCode")
	}
	// 即使失败，也应返回结果
	if result == nil {
		t.Error("result should not be nil even on failure")
	}
}

func TestGetChatGroupInfo_Success(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"retCode": "0000",
			"retMsg":  "success",
			"name":    "test-chat",
		})
	})
	wecomSvc, _ := newTestWeComService(t, srv.URL, srv.URL, srv.URL)

	result, err := wecomSvc.GetChatGroupInfo("chat-123")
	if err != nil {
		t.Errorf("GetChatGroupInfo failed: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if result["name"] != "test-chat" {
		t.Errorf("name = %v, want test-chat", result["name"])
	}
}

func TestUpdateChatGroup_Success(t *testing.T) {
	srv := newTestServer(t, successHandler())
	wecomSvc, _ := newTestWeComService(t, srv.URL, srv.URL, srv.URL)

	result, err := wecomSvc.UpdateChatGroup("chat-123", "owner1", []string{"user3"}, []string{"user1"}, "updated-name")
	if err != nil {
		t.Errorf("UpdateChatGroup failed: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
}

func TestUpdateChatGroup_FailResponse(t *testing.T) {
	srv := newTestServer(t, failHandler())
	wecomSvc, _ := newTestWeComService(t, srv.URL, srv.URL, srv.URL)

	_, err := wecomSvc.UpdateChatGroup("chat-123", "owner1", []string{"user3"}, []string{"user1"}, "updated-name")
	if err == nil {
		t.Error("UpdateChatGroup should fail with non-0000 retCode")
	}
}

// ------------------------------------------------------------
// 错误路径测试
// ------------------------------------------------------------

func TestSendRequestWithResult_InvalidURL(t *testing.T) {
	wecomSvc, _ := newTestWeComService(t, "http://127.0.0.1:1", "", "")

	_, err := wecomSvc.sendRequestWithResult("http://127.0.0.1:1", map[string]interface{}{"key": "value"})
	if err == nil {
		t.Error("sendRequestWithResult should fail with invalid URL")
	}
}

func TestSendRequestWithResult_DecodeError(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// 返回非法 JSON
		_, _ = w.Write([]byte("not-json"))
	})
	wecomSvc, _ := newTestWeComService(t, srv.URL, srv.URL, srv.URL)

	_, err := wecomSvc.sendRequestWithResult(srv.URL, map[string]interface{}{"key": "value"})
	if err == nil {
		t.Error("sendRequestWithResult should fail with invalid JSON response")
	}
}

func TestSendRequest_NoRetCode(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// 返回不含 retCode 的响应
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"someField": "someValue",
		})
	})
	wecomSvc, _ := newTestWeComService(t, srv.URL, srv.URL, srv.URL)

	// 不含 retCode 时应视为成功
	err := wecomSvc.sendRequest(srv.URL, map[string]interface{}{"key": "value"})
	if err != nil {
		t.Errorf("sendRequest should succeed when retCode is absent, got: %v", err)
	}
}

func TestSendRequest_SuccessRetCode(t *testing.T) {
	srv := newTestServer(t, successHandler())
	wecomSvc, _ := newTestWeComService(t, srv.URL, srv.URL, srv.URL)

	err := wecomSvc.sendRequest(srv.URL, map[string]interface{}{"key": "value"})
	if err != nil {
		t.Errorf("sendRequest should succeed with retCode=0000, got: %v", err)
	}
}

func TestSendRequest_FailRetCode(t *testing.T) {
	srv := newTestServer(t, failHandler())
	wecomSvc, _ := newTestWeComService(t, srv.URL, srv.URL, srv.URL)

	err := wecomSvc.sendRequest(srv.URL, map[string]interface{}{"key": "value"})
	if err == nil {
		t.Error("sendRequest should fail with retCode=1001")
	}
}

func TestSendRequest_WithMissingRetMsg(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// retCode 非 0000 但不含 retMsg
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"retCode": "9999",
		})
	})
	wecomSvc, _ := newTestWeComService(t, srv.URL, srv.URL, srv.URL)

	err := wecomSvc.sendRequest(srv.URL, map[string]interface{}{"key": "value"})
	if err == nil {
		t.Error("sendRequest should fail with retCode=9999")
	}
}

// ------------------------------------------------------------
// WeComResponse 结构体测试
// ------------------------------------------------------------

func TestWeComResponse_JSONUnmarshal(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantRet string
		wantMsg string
	}{
		{
			name:    "success response",
			json:    `{"retCode":"0000","retMsg":"success"}`,
			wantRet: "0000",
			wantMsg: "success",
		},
		{
			name:    "error response",
			json:    `{"retCode":"1001","retMsg":"invalid token"}`,
			wantRet: "1001",
			wantMsg: "invalid token",
		},
		{
			name:    "empty response",
			json:    `{}`,
			wantRet: "",
			wantMsg: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp WeComResponse
			if err := json.Unmarshal([]byte(tt.json), &resp); err != nil {
				t.Fatalf("unmarshal failed: %v", err)
			}
			if resp.RetCode != tt.wantRet {
				t.Errorf("RetCode = %v, want %v", resp.RetCode, tt.wantRet)
			}
			if resp.RetMsg != tt.wantMsg {
				t.Errorf("RetMsg = %v, want %v", resp.RetMsg, tt.wantMsg)
			}
		})
	}
}

// ------------------------------------------------------------
// 并发安全测试
// ------------------------------------------------------------

func TestWeComService_ConcurrentAccess(t *testing.T) {
	srv := newTestServer(t, successHandler())
	wecomSvc, _ := newTestWeComService(t, srv.URL, srv.URL, srv.URL)

	// 并发读取 URL
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			wecomSvc.mu.RLock()
			_ = wecomSvc.runtimeRobotURL
			_ = wecomSvc.runtimeAppMsgURL
			_ = wecomSvc.runtimeEwechatURL
			wecomSvc.mu.RUnlock()
		}()
	}

	// 并发写入 URL（通过 OnConfigChanged）
	for i := 0; i < 5; i++ {
		go func(i int) {
			defer func() { done <- struct{}{} }()
			wecomSvc.OnConfigChanged("wecom.robot_url", "http://updated.example.com")
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 15; i++ {
		<-done
	}
}

// ------------------------------------------------------------
// 综合场景测试
// ------------------------------------------------------------

func TestSendAppMsg_MultiplePhoneNumbers(t *testing.T) {
	var captured map[string]interface{}
	srv := newTestServer(t, captureHandler(t, &captured))
	wecomSvc, _ := newTestWeComService(t, srv.URL, srv.URL, srv.URL)

	phones := []string{"13800138000", "13900139000", "13700137000"}
	err := wecomSvc.SendAppMsg(1000027, "text", "meeting reminder", phones)
	if err != nil {
		t.Errorf("SendAppMsg failed: %v", err)
	}

	// 验证请求体中的手机号列表
	reqData, ok := captured["reqData"].(map[string]interface{})
	if !ok {
		t.Fatal("reqData should be a map")
	}
	toUniqueInnerUserId, ok := reqData["toUniqueInnerUserId"].(string)
	if !ok {
		t.Fatal("toUniqueInnerUserId should be a string")
	}
	expected := "13800138000|13900139000|13700137000"
	if toUniqueInnerUserId != expected {
		t.Errorf("toUniqueInnerUserId = %v, want %v", toUniqueInnerUserId, expected)
	}
}

func TestSendRobotImageMsg_EmptyBytes(t *testing.T) {
	srv := newTestServer(t, successHandler())
	wecomSvc, _ := newTestWeComService(t, srv.URL, srv.URL, srv.URL)

	// 空字节数组也应正常处理
	err := wecomSvc.SendRobotImageMsg("test-group", []byte{})
	if err != nil {
		t.Errorf("SendRobotImageMsg with empty bytes failed: %v", err)
	}
}

func TestSendAppMsg_EmptyPhoneList(t *testing.T) {
	srv := newTestServer(t, successHandler())
	wecomSvc, _ := newTestWeComService(t, srv.URL, srv.URL, srv.URL)

	err := wecomSvc.SendAppMsg(1000027, "text", "broadcast", []string{})
	if err != nil {
		t.Errorf("SendAppMsg with empty phone list failed: %v", err)
	}
}
