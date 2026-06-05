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
	"time"
)

type WeComService struct {
	robotMsgURL string
	appMsgURL   string
	ewechatURL  string
	httpClient  *http.Client
}

func NewService(robotMsgURL, appMsgURL, ewechatURL string) *WeComService {
	return &WeComService{
		robotMsgURL: robotMsgURL,
		appMsgURL:   appMsgURL,
		ewechatURL:  ewechatURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type WeComResponse struct {
	RetCode string `json:"retCode"`
	RetMsg  string `json:"retMsg"`
}

func (s *WeComService) SendAppMsg(agentID int, msgType, msgContent string, phoneNumList []string) error {
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

	return s.sendRequest(s.appMsgURL, data)
}

func (s *WeComService) SendRobotMarkdownMsg(groupID string, msg string) error {
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

	return s.sendRequest(s.robotMsgURL, data)
}

func (s *WeComService) SendRobotImageMsg(groupID string, pictureBytes []byte) error {
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

	return s.sendRequest(s.robotMsgURL, data)
}

func (s *WeComService) SendRobotTextPeopleMsg(groupID, msg string, phoneNumber string) error {
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

	return s.sendRequest(s.robotMsgURL, data)
}

func (s *WeComService) SendChatMarkdownMsg(chatID, msg string) error {
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

	return s.sendRequest(s.ewechatURL, msgData)
}

func (s *WeComService) CreateChatGroup(chatName string, userList []string) (map[string]interface{}, error) {
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

	return s.sendRequestWithResult(s.ewechatURL, data)
}

func (s *WeComService) GetChatGroupInfo(chatID string) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"fromChannel": "HDP",
		"agentId":     "1000027",
		"httpMethod":  "GET",
		"url":         fmt.Sprintf("/appchat/get?chatid=%s", chatID),
		"apiName":     "获取群聊会话",
	}

	return s.sendRequestWithResult(s.ewechatURL, data)
}

func (s *WeComService) UpdateChatGroup(chatID, ownerID string, addUserList, delUserList []string, chatName string) (map[string]interface{}, error) {
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

	return s.sendRequestWithResult(s.ewechatURL, data)
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
