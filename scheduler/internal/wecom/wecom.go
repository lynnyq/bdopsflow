package wecom

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	sysconfig "github.com/lynnyq/bdopsflow/scheduler/internal/system_config"
)

type WeComService struct {
	config      *sysconfig.Service
	httpClient  *http.Client

	// 运行时配置缓存
	runtimeRobotURL  string
	runtimeAppMsgURL string
	runtimeEwechatURL string
	mu               sync.RWMutex
}

func NewService(config *sysconfig.Service) *WeComService {
	s := &WeComService{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// 初始化运行时配置
	s.refreshRuntimeConfig()

	// 注册为配置观察者
	config.RegisterObserver(s)

	return s
}

// OnConfigChanged 实现 sysconfig.ConfigObserver 接口
func (s *WeComService) OnConfigChanged(key, value string) {
	if key == "wecom.robot_url" || key == "wecom.app_msg_url" || key == "wecom.ewechat_url" {
		s.refreshRuntimeConfig()
		slog.Info("wecom service config updated", "key", key, "value", value)
	}
}

// refreshRuntimeConfig 刷新运行时配置缓存
func (s *WeComService) refreshRuntimeConfig() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runtimeRobotURL = s.config.Get("wecom.robot_url")
	s.runtimeAppMsgURL = s.config.Get("wecom.app_msg_url")
	s.runtimeEwechatURL = s.config.Get("wecom.ewechat_url")

	if s.runtimeRobotURL == "" {
		s.runtimeRobotURL = "https://qyapi.weixin.qq.com/cgi-bin/webhook/send"
	}
	if s.runtimeAppMsgURL == "" {
		s.runtimeAppMsgURL = "https://qyapi.weixin.qq.com/cgi-bin/webhook/send"
	}
	if s.runtimeEwechatURL == "" {
		s.runtimeEwechatURL = "https://qyapi.weixin.qq.com/cgi-bin/webhook/send"
	}
}

type WeComResponse struct {
	RetCode string `json:"retCode"`
	RetMsg  string `json:"retMsg"`
}

func (s *WeComService) SendAppMsg(agentID int, msgType, msgContent string, phoneNumList []string) error {
	s.mu.RLock()
	appMsgURL := s.runtimeAppMsgURL
	s.mu.RUnlock()

	data := map[string]interface{}{
		"agentId":      fmt.Sprintf("%d", agentID),
		"fromChannel": "HDP",
		"reqData": map[string]interface{}{
			"msgtype":     msgType,
			"agentid":     agentID,
			"ewechatMsg": map[string]string{
				"content": msgContent,
			},
			"userIdType":           "phoneNum",
			"toUniqueInnerUserId":  strings.Join(phoneNumList, "|"),
			"toInnerUserId":        strings.Join(phoneNumList, "|"),
			"safe":                 0,
			"toInnerPartyId":       "",
			"totag":                "",
		},
	}

	return s.sendRequest(appMsgURL, data)
}

func (s *WeComService) SendRobotMarkdownMsg(groupID string, msg string) error {
	s.mu.RLock()
	robotURL := s.runtimeRobotURL
	s.mu.RUnlock()

	data := map[string]interface{}{
		"groupId":     groupID,
		"fromChannel": "HDP",
		"reqData": map[string]interface{}{
			"msgtype": "markdown",
			"ewechatMsg": map[string]string{
				"content": msg,
			},
		},
	}

	return s.sendRequest(robotURL, data)
}

func (s *WeComService) SendRobotImageMsg(groupID string, pictureBytes []byte) error {
	s.mu.RLock()
	robotURL := s.runtimeRobotURL
	s.mu.RUnlock()

	b64Value := base64.StdEncoding.EncodeToString(pictureBytes)
	md := md5.New()
	md.Write(pictureBytes)
	md5Value := fmt.Sprintf("%x", md.Sum(nil))

	data := map[string]interface{}{
		"groupId":     groupID,
		"fromChannel": "HDP",
		"reqData": map[string]interface{}{
			"msgtype": "image",
			"ewechatMsg": map[string]string{
				"base64": b64Value,
				"md5":    md5Value,
			},
		},
	}

	return s.sendRequest(robotURL, data)
}

func (s *WeComService) SendRobotTextPeopleMsg(groupID, msg string, phoneNumber string) error {
	s.mu.RLock()
	robotURL := s.runtimeRobotURL
	s.mu.RUnlock()

	data := map[string]interface{}{
		"groupId":     groupID,
		"fromChannel": "HDP",
		"reqData": map[string]interface{}{
			"msgtype": "text",
			"ewechatMsg": map[string]interface{}{
				"content":               msg,
				"mentionedMobileList":   phoneNumber,
			},
		},
	}

	return s.sendRequest(robotURL, data)
}

func (s *WeComService) SendChatMarkdownMsg(chatID, msg string) error {
	s.mu.RLock()
	ewechatURL := s.runtimeEwechatURL
	s.mu.RUnlock()

	msgData := map[string]interface{}{
		"fromChannel": "HDP",
		"agentId":     "1000027",
		"httpMethod":  "POST",
		"url":         "/appchat/send",
		"apiName":     "应用推送消息",
		"reqData": map[string]interface{}{
			"chatid":   chatID,
			"msgtype":  "markdown",
			"markdown": map[string]string{
				"content": msg,
			},
			"safe": 0,
		},
	}

	return s.sendRequest(ewechatURL, msgData)
}

func (s *WeComService) CreateChatGroup(chatName string, userList []string) (map[string]interface{}, error) {
	s.mu.RLock()
	ewechatURL := s.runtimeEwechatURL
	s.mu.RUnlock()

	data := map[string]interface{}{
		"fromChannel": "HDP",
		"agentId":     "1000027",
		"httpMethod":  "POST",
		"url":         "/appchat/create",
		"apiName":     "创建群聊会话",
		"reqData": map[string]interface{}{
			"name":     chatName,
			"owner":    userList[0],
			"userlist": userList,
			"chatid":   "",
		},
	}

	return s.sendRequestWithResult(ewechatURL, data)
}

func (s *WeComService) GetChatGroupInfo(chatID string) (map[string]interface{}, error) {
	s.mu.RLock()
	ewechatURL := s.runtimeEwechatURL
	s.mu.RUnlock()

	data := map[string]interface{}{
		"fromChannel": "HDP",
		"agentId":     "1000027",
		"httpMethod":  "GET",
		"url":         fmt.Sprintf("/appchat/get?chatid=%s", chatID),
		"apiName":     "获取群聊会话",
	}

	return s.sendRequestWithResult(ewechatURL, data)
}

func (s *WeComService) UpdateChatGroup(chatID, ownerID string, addUserList, delUserList []string, chatName string) (map[string]interface{}, error) {
	s.mu.RLock()
	ewechatURL := s.runtimeEwechatURL
	s.mu.RUnlock()

	data := map[string]interface{}{
		"fromChannel": "HDP",
		"agentId":     "1000027",
		"httpMethod":  "POST",
		"url":         "/appchat/update",
		"apiName":     "修改群聊会话",
		"reqData": map[string]interface{}{
			"chatid":         chatID,
			"name":           chatName,
			"owner":          ownerID,
			"add_user_list":  addUserList,
			"del_user_list":  delUserList,
		},
	}

	return s.sendRequestWithResult(ewechatURL, data)
}

func (s *WeComService) sendRequest(url string, data map[string]interface{}) error {
	_, err := s.sendRequestWithResult(url, data)
	return err
}

func (s *WeComService) sendRequestWithResult(url string, data map[string]interface{}) (map[string]interface{}, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}

	slog.Debug("sending wecom request", "url", url, "data", string(jsonData))

	resp, err := s.httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	slog.Debug("wecom response received", "result", result)

	if retCode, ok := result["retCode"].(string); ok {
		if retCode != "0000" {
			retMsg := ""
			if msg, ok := result["retMsg"].(string); ok {
				retMsg = msg
			}
			return result, fmt.Errorf("message send failed: retCode=%s, retMsg=%s", retCode, retMsg)
		}
	}

	return result, nil
}
