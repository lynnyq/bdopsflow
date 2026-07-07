package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/metrics"
	"github.com/lynnyq/bdopsflow/scheduler/internal/middleware"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/rsautil"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	authSvc    *service.AuthService
	permSvc    *service.PermissionService
	rsaUtil    *rsautil.RSAUtil
	ssoEnabled bool
	ssoUrl     string
	ssoRsaUtil *rsautil.RSAUtil
	ssoTimeout time.Duration
}

func NewAuthHandler(authSvc *service.AuthService, permSvc *service.PermissionService, rsaUtil *rsautil.RSAUtil, ssoEnabled bool, ssoUrl string, ssoRsaUtil *rsautil.RSAUtil, ssoTimeout int) *AuthHandler {
	timeout := time.Duration(ssoTimeout) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &AuthHandler{
		authSvc:    authSvc,
		permSvc:    permSvc,
		rsaUtil:    rsaUtil,
		ssoEnabled: ssoEnabled,
		ssoUrl:     ssoUrl,
		ssoRsaUtil: ssoRsaUtil,
		ssoTimeout: timeout,
	}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "用户名和密码不能为空")
		return
	}

	if req.Username == "" || req.Password == "" {
		BadRequest(c, "用户名和密码不能为空")
		return
	}

	c.Set("audit_resource_name", req.Username)
	c.Set("username", req.Username)

	slog.Debug("Login: request entry", "module", "handler_auth", "username", req.Username)

	loginUser, found, err := h.authSvc.GetUserByUsername(c.Request.Context(), req.Username)
	if err != nil {
		slog.Error("Login: query user failed", "error", err, "username", req.Username)
		InternalServerError(c, "服务器错误，请稍后重试")
		return
	}

	if !found {
		slog.Warn("Login: user not found", "module", "handler_auth", "username", req.Username)
		metrics.AuthAttempts.WithLabelValues("local", "failed").Inc()
		Fail(c, CodeInvalidCredentials, "用户名或密码错误")
		return
	}

	userID := loginUser.ID
	username := loginUser.Username
	realName := loginUser.RealName
	phone := loginUser.Phone
	hashedPassword := loginUser.HashedPassword
	email := loginUser.Email
	isActive := loginUser.IsActive

	decryptedPassword, err := h.rsaUtil.Decrypt(req.Password)
	if err != nil {
		slog.Warn("Login: password decryption failed", "module", "handler_auth", "username", req.Username, "error", err)
		Fail(c, CodeInvalidCredentials, "用户名或密码错误")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(decryptedPassword)); err != nil {
		slog.Warn("Login: password comparison failed", "module", "handler_auth", "username", req.Username, "error", err)
		metrics.AuthAttempts.WithLabelValues("local", "failed").Inc()
		Fail(c, CodeInvalidCredentials, "用户名或密码错误")
		return
	}

	if !isActive {
		slog.Warn("Login: user is inactive", "module", "handler_auth", "user_id", userID, "username", username)
		metrics.AuthAttempts.WithLabelValues("local", "failed").Inc()
		Fail(c, CodeInvalidCredentials, "该账号已被禁用，请联系管理员")
		return
	}

	domains, domainErr := h.permSvc.GetUserDomainInfos(c.Request.Context(), userID)
	if domainErr != nil {
		slog.Error("Login: get user domain infos failed", "error", domainErr, "user_id", userID)
	}
	var currentDomainID int64
	defaultDomainID, defaultErr := h.permSvc.GetUserDefaultDomain(c.Request.Context(), userID)
	if defaultErr != nil {
		slog.Error("Login: get user default domain failed", "error", defaultErr, "user_id", userID)
	}
	if defaultDomainID > 0 {
		currentDomainID = defaultDomainID
	} else if len(domains) > 0 {
		currentDomainID = domains[0].DomainID
	}

	tokenString, err := middleware.GenerateToken(userID, username, realName, currentDomainID)
	if err != nil {
		slog.Error("Login: generate token failed", "error", err, "user_id", userID)
		InternalServerError(c, "服务器错误，请稍后重试")
		return
	}

	refreshToken, refreshErr := middleware.GenerateRefreshToken(userID, username, realName, currentDomainID)
	if refreshErr != nil {
		slog.Error("Login: generate refresh token failed", "error", refreshErr, "user_id", userID)
		InternalServerError(c, "服务器错误，请稍后重试")
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if updateErr := h.authSvc.UpdateLastLogin(ctx, userID); updateErr != nil {
			slog.Error("Login: update last login failed", "error", updateErr, "user_id", userID)
		}
	}()

	permissions, permErr := h.permSvc.GetUserPermissions(c.Request.Context(), userID)
	if permErr != nil {
		slog.Error("Login: get user permissions failed", "error", permErr, "user_id", userID)
	}
	if permissions == nil {
		permissions = []*model.Permission{}
	}
	if domains == nil {
		domains = []*model.UserDomainInfo{}
	}

	roleCodes, roleErr := h.permSvc.GetUserRoleCodes(c.Request.Context(), userID)
	if roleErr != nil {
		slog.Error("Login: get user role codes failed", "error", roleErr, "user_id", userID)
	}
	if roleCodes == nil {
		roleCodes = []string{}
	}

	slog.Info("Login: success", "user_id", userID, "username", username, "domain_id", currentDomainID, "permissions_count", len(permissions), "domains_count", len(domains))
	metrics.AuthAttempts.WithLabelValues("local", "success").Inc()

	Success(c, gin.H{
		"token":         tokenString,
		"refresh_token": refreshToken,
		"user": map[string]interface{}{
			"id":        userID,
			"username":  username,
			"real_name": realName,
			"phone":     phone,
			"email":     email,
			"is_active": isActive,
		},
		"permissions":       permissions,
		"domains":           domains,
		"current_domain_id": currentDomainID,
		"role_codes":        roleCodes,
	})
}

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "refresh_token 不能为空")
		return
	}

	claims, err := middleware.ParseRefreshToken(req.RefreshToken)
	if err != nil {
		slog.Warn("RefreshToken: invalid or expired refresh token", "error", err)
		Fail(c, CodeInvalidToken, "refresh token 无效或已过期")
		return
	}

	if claims.Issuer != "bdopsflow-refresh" {
		slog.Warn("RefreshToken: invalid token issuer", "issuer", claims.Issuer)
		Fail(c, CodeInvalidToken, "无效的 refresh token")
		return
	}

	// 检查用户是否被禁用
	isActive, userFound, err := h.authSvc.GetUserActiveStatus(c.Request.Context(), claims.UserID)
	if err != nil {
		slog.Error("RefreshToken: query user failed", "error", err, "user_id", claims.UserID)
		InternalServerError(c, "服务器错误，请稍后重试")
		return
	}
	if !userFound {
		slog.Warn("RefreshToken: user not found", "user_id", claims.UserID)
		Fail(c, CodeInvalidToken, "用户不存在")
		return
	}
	if !isActive {
		slog.Warn("RefreshToken: user is inactive", "user_id", claims.UserID)
		Fail(c, CodeInvalidToken, "该账号已被禁用，请联系管理员")
		return
	}

	newToken, tokenErr := middleware.GenerateToken(claims.UserID, claims.Username, claims.RealName, claims.CurrentDomainID)
	if tokenErr != nil {
		slog.Error("RefreshToken: generate token failed", "error", tokenErr, "user_id", claims.UserID)
		InternalServerError(c, "服务器错误，请稍后重试")
		return
	}

	newRefreshToken, refreshErr := middleware.GenerateRefreshToken(claims.UserID, claims.Username, claims.RealName, claims.CurrentDomainID)
	if refreshErr != nil {
		slog.Error("RefreshToken: generate refresh token failed", "error", refreshErr, "user_id", claims.UserID)
		InternalServerError(c, "服务器错误，请稍后重试")
		return
	}

	slog.Info("RefreshToken: success", "user_id", claims.UserID, "username", claims.Username)

	Success(c, gin.H{
		"token":         newToken,
		"refresh_token": newRefreshToken,
	})
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required,min=1,max=512"`
		Email    string `json:"email" binding:"omitempty,email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "请求参数错误：用户名和密码为必填项，密码至少6位")
		return
	}

	if req.Username == "" || req.Password == "" {
		BadRequest(c, "用户名和密码不能为空")
		return
	}

	decryptedPassword, err := h.rsaUtil.Decrypt(req.Password)
	if err != nil {
		BadRequest(c, "密码解密失败")
		return
	}

	if len(decryptedPassword) < 6 {
		BadRequest(c, "密码长度至少为6位")
		return
	}
	if len(decryptedPassword) > 30 {
		BadRequest(c, "密码长度不能超过30位")
		return
	}
	hasLetter := false
	hasDigit := false
	for _, ch := range decryptedPassword {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') {
			hasLetter = true
		}
		if ch >= '0' && ch <= '9' {
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		BadRequest(c, "密码必须包含字母和数字")
		return
	}

	c.Set("audit_resource_name", req.Username)
	c.Set("username", req.Username)

	slog.Debug("Register: request entry", "module", "handler_auth", "username", req.Username)

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(decryptedPassword), bcrypt.DefaultCost)
	if err != nil {
		slog.Error("Register: bcrypt generate failed", "module", "handler_auth", "username", req.Username, "error", err)
		InternalServerError(c, "服务器错误，请稍后重试")
		return
	}

	userID, err := h.authSvc.CreateUser(c.Request.Context(), req.Username, "", "", string(hashedPassword), req.Email)
	if err != nil {
		slog.Error("Register: create user failed", "module", "handler_auth", "username", req.Username, "error", err)
		InternalServerError(c, "服务器错误，请稍后重试")
		return
	}

	slog.Info("Register: success", "module", "handler_auth", "user_id", userID, "username", req.Username)

	// 分配默认角色 "user"
	roleID, roleErr := h.authSvc.GetRoleIDByCode(c.Request.Context(), "user")
	if roleErr != nil {
		slog.Warn("Register: query default role failed", "module", "handler_auth", "error", roleErr)
	} else if roleID > 0 {
		if assignErr := h.authSvc.AssignUserRole(c.Request.Context(), userID, roleID); assignErr != nil {
			slog.Warn("Register: assign user role failed", "module", "handler_auth", "user_id", userID, "error", assignErr)
		}
	}

	Created(c, gin.H{
		"id":       userID,
		"username": req.Username,
		"email":    req.Email,
	})
}

func (h *AuthHandler) GetCurrentUser(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		Unauthorized(c, "未授权，请重新登录")
		return
	}

	slog.Debug("GetCurrentUser: request entry", "module", "handler_auth", "user_id", userID)

	uid, _ := userID.(int64)
	userInfo, found, err := h.authSvc.GetUserByID(c.Request.Context(), uid)
	if err != nil {
		slog.Error("GetCurrentUser: query failed", "error", err, "user_id", userID)
		InternalServerError(c, "服务器错误，请稍后重试")
		return
	}

	if !found {
		NotFound(c, "用户不存在")
		return
	}

	if !userInfo.IsActive {
		slog.Warn("GetCurrentUser: user is inactive", "module", "handler_auth", "user_id", userID)
		Unauthorized(c, "该账号已被禁用，请联系管理员")
		return
	}

	permissions, permErr := h.permSvc.GetUserPermissions(c.Request.Context(), uid)
	if permErr != nil {
		slog.Error("GetCurrentUser: get permissions failed", "error", permErr, "user_id", uid)
	}
	if permissions == nil {
		permissions = []*model.Permission{}
	}

	domains, domainErr := h.permSvc.GetUserDomainInfos(c.Request.Context(), uid)
	if domainErr != nil {
		slog.Error("GetCurrentUser: get domains failed", "error", domainErr, "user_id", uid)
	}
	if domains == nil {
		domains = []*model.UserDomainInfo{}
	}
	var currentDomainID int64
	domainIDVal, _ := c.Get("current_domain_id")
	if v, ok := domainIDVal.(int64); ok {
		currentDomainID = v
	}

	var lastLoginAtStr *string
	if userInfo.LastLoginAt != nil && !userInfo.LastLoginAt.IsZero() {
		s := userInfo.LastLoginAt.Format(TimeResponseFormat)
		lastLoginAtStr = &s
	}

	roleCodes, roleErr := h.permSvc.GetUserRoleCodes(c.Request.Context(), uid)
	if roleErr != nil {
		slog.Error("GetCurrentUser: get role codes failed", "error", roleErr, "user_id", uid)
	}
	if roleCodes == nil {
		roleCodes = []string{}
	}

	Success(c, gin.H{
		"user": map[string]interface{}{
			"id":            userID,
			"username":      userInfo.Username,
			"real_name":     userInfo.RealName,
			"phone":         userInfo.Phone,
			"email":         userInfo.Email,
			"is_active":     userInfo.IsActive,
			"last_login_at": lastLoginAtStr,
		},
		"permissions":       permissions,
		"domains":           domains,
		"current_domain_id": currentDomainID,
		"role_codes":        roleCodes,
	})
}

func (h *AuthHandler) SwitchDomain(c *gin.Context) {
	var req struct {
		DomainID int64 `json:"domain_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "领域ID不能为空")
		return
	}

	userID, _ := c.Get("user_id")
	var uid int64
	if v, ok := userID.(int64); ok {
		uid = v
	}

	slog.Debug("SwitchDomain: request entry", "module", "handler_auth", "user_id", uid, "domain_id", req.DomainID)

	permissions, err := h.permSvc.SwitchDomain(c.Request.Context(), uid, req.DomainID)
	if err != nil {
		if errors.Is(err, service.ErrDomainAccessDenied) {
			slog.Warn("SwitchDomain: domain access denied", "module", "handler_auth", "user_id", uid, "domain_id", req.DomainID)
			Forbidden(c, "无权访问该领域")
			return
		}
		InternalServerError(c, "切换领域失败")
		return
	}

	username, _ := c.Get("username")
	realName, _ := c.Get("real_name")
	var uname, rname string
	if v, ok := username.(string); ok {
		uname = v
	}
	if v, ok := realName.(string); ok {
		rname = v
	}
	tokenString, err := middleware.GenerateToken(uid, uname, rname, req.DomainID)
	if err != nil {
		slog.Error("SwitchDomain: generate token failed", "module", "handler_auth", "user_id", uid, "domain_id", req.DomainID, "error", err)
		InternalServerError(c, "生成Token失败")
		return
	}

	refreshToken, refreshErr := middleware.GenerateRefreshToken(uid, uname, rname, req.DomainID)
	if refreshErr != nil {
		slog.Error("SwitchDomain: generate refresh token failed", "module", "handler_auth", "user_id", uid, "domain_id", req.DomainID, "error", refreshErr)
		InternalServerError(c, "生成Token失败")
		return
	}

	roleCodes, roleErr := h.permSvc.GetUserRoleCodes(c.Request.Context(), uid)
	if roleErr != nil {
		slog.Error("SwitchDomain: get role codes failed", "error", roleErr, "user_id", uid)
	}
	if roleCodes == nil {
		roleCodes = []string{}
	}

	Success(c, gin.H{
		"token":             tokenString,
		"refresh_token":     refreshToken,
		"permissions":       permissions,
		"current_domain_id": req.DomainID,
		"role_codes":        roleCodes,
	})
}

func (h *AuthHandler) GetPublicKey(c *gin.Context) {
	result := gin.H{
		"public_key":  h.rsaUtil.PublicKeyB64(),
		"sso_enabled": h.ssoEnabled,
	}
	if h.ssoEnabled && h.ssoRsaUtil != nil {
		result["sso_public_key"] = h.ssoRsaUtil.PublicKeyB64()
	}
	Success(c, result)
}

type ssoRequest struct {
	LoginName string `json:"loginName"`
	Password  string `json:"password"`
}

type ssoContent struct {
	ID          int64  `json:"id"`
	LoginName   string `json:"loginName"`
	IDCardName  string `json:"idCardName"`
	MobileNo    string `json:"mobileNo"`
	Email       string `json:"email"`
	DeptNo      string `json:"deptNo"`
	WorkID      string `json:"workId"`
	Gender      string `json:"gender"`
	OfficePhone string `json:"officePhone"`
}

type ssoResponse struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Content *ssoContent `json:"content"`
}

func (h *AuthHandler) SSOLogin(c *gin.Context) {
	if !h.ssoEnabled {
		BadRequest(c, "SSO登录未启用，请使用本地登录")
		return
	}

	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "用户名和密码不能为空")
		return
	}

	c.Set("audit_resource_name", req.Username)
	c.Set("username", req.Username)

	slog.Debug("SSOLogin: request entry", "module", "handler_auth", "username", req.Username)

	ssoReq := ssoRequest{
		LoginName: req.Username,
		Password:  req.Password,
	}
	ssoBody, err := json.Marshal(ssoReq)
	if err != nil {
		slog.Error("SSOLogin: failed to marshal SSO request", "error", err)
		InternalServerError(c, "SSO登录失败，请稍后再试")
		return
	}

	httpReq, err := http.NewRequestWithContext(c.Request.Context(), "POST", h.ssoUrl, bytes.NewReader(ssoBody))
	if err != nil {
		slog.Error("SSOLogin: failed to create SSO request", "error", err)
		InternalServerError(c, "SSO登录失败，请稍后再试")
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{Timeout: h.ssoTimeout}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		slog.Error("SSOLogin: failed to call SSO service", "error", err, "url", h.ssoUrl)
		Fail(c, CodeInternalError, "SSO登录失败，请稍后再试")
		return
	}
	defer resp.Body.Close()

	var ssoResp ssoResponse
	if err := json.NewDecoder(resp.Body).Decode(&ssoResp); err != nil {
		slog.Error("SSOLogin: failed to decode SSO response", "error", err)
		InternalServerError(c, "SSO登录失败，请稍后再试")
		return
	}

	if ssoResp.Code != "3000" || ssoResp.Content == nil {
		errMsg := ssoResp.Message
		if errMsg == "" {
			errMsg = "SSO登录失败"
		}
		slog.Warn("SSOLogin: SSO authentication failed", "code", ssoResp.Code, "message", errMsg)
		metrics.AuthAttempts.WithLabelValues("sso", "failed").Inc()
		Fail(c, CodeInvalidCredentials, errMsg)
		return
	}

	ssoUser := ssoResp.Content
	loginName := ssoUser.LoginName
	if loginName == "" {
		loginName = req.Username
	}

	ssoUserInfo, ssoFound, err := h.authSvc.GetSSOUserByUsername(c.Request.Context(), loginName)
	if err != nil {
		slog.Error("SSOLogin: failed to query user", "error", err)
		InternalServerError(c, "服务器错误，请稍后重试")
		return
	}

	var userID int64
	var username, realName, phone, email string
	var isActive bool

	if ssoFound {
		userID = ssoUserInfo.ID
		username = ssoUserInfo.Username
		realName = ssoUserInfo.RealName
		phone = ssoUserInfo.Phone
		email = ssoUserInfo.Email
		isActive = ssoUserInfo.IsActive

		if !isActive {
			slog.Warn("SSOLogin: user is inactive", "module", "handler_auth", "user_id", userID, "username", username)
			metrics.AuthAttempts.WithLabelValues("sso", "failed").Inc()
			Fail(c, CodeInvalidCredentials, "该账号已被禁用，请联系管理员")
			return
		}

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if updateErr := h.authSvc.UpdateLastLogin(ctx, userID); updateErr != nil {
				slog.Error("SSOLogin: update last login failed", "error", updateErr, "user_id", userID)
			}
		}()

		slog.Info("SSOLogin: existing user login success", "module", "handler_auth", "user_id", userID, "username", username)
	} else {
		realName = ssoUser.IDCardName
		phone = ssoUser.MobileNo
		email = ssoUser.Email
		isActive = true

		userID, err = h.authSvc.CreateUser(c.Request.Context(), loginName, realName, phone, "", email)
		if err != nil {
			slog.Error("SSOLogin: failed to create user", "error", err)
			InternalServerError(c, "服务器错误，请稍后重试")
			return
		}
		username = loginName

		// 分配默认角色 "user"
		roleID, roleErr := h.authSvc.GetRoleIDByCode(c.Request.Context(), "user")
		if roleErr != nil {
			slog.Warn("SSOLogin: query default role failed", "error", roleErr)
		} else if roleID > 0 {
			if assignErr := h.authSvc.AssignUserRole(c.Request.Context(), userID, roleID); assignErr != nil {
				slog.Warn("SSOLogin: assign user role failed", "user_id", userID, "error", assignErr)
			}
		}

		slog.Info("SSOLogin: auto created user from SSO", "username", loginName, "user_id", userID)
	}

	domains, domainErr := h.permSvc.GetUserDomainInfos(c.Request.Context(), userID)
	if domainErr != nil {
		slog.Error("SSOLogin: get user domain infos failed", "error", domainErr, "user_id", userID)
	}
	if domains == nil {
		domains = []*model.UserDomainInfo{}
	}
	var currentDomainID int64
	defaultDomainID, defaultErr := h.permSvc.GetUserDefaultDomain(c.Request.Context(), userID)
	if defaultErr != nil {
		slog.Error("SSOLogin: get user default domain failed", "error", defaultErr, "user_id", userID)
	}
	if defaultDomainID > 0 {
		currentDomainID = defaultDomainID
	} else if len(domains) > 0 {
		currentDomainID = domains[0].DomainID
	}

	tokenString, err := middleware.GenerateToken(userID, username, realName, currentDomainID)
	if err != nil {
		slog.Error("SSOLogin: failed to generate token", "error", err)
		InternalServerError(c, "服务器错误，请稍后重试")
		return
	}

	refreshToken, refreshErr := middleware.GenerateRefreshToken(userID, username, realName, currentDomainID)
	if refreshErr != nil {
		slog.Error("SSOLogin: generate refresh token failed", "error", refreshErr, "user_id", userID)
		InternalServerError(c, "服务器错误，请稍后重试")
		return
	}

	permissions, permErr := h.permSvc.GetUserPermissions(c.Request.Context(), userID)
	if permErr != nil {
		slog.Error("SSOLogin: get user permissions failed", "error", permErr, "user_id", userID)
	}
	if permissions == nil {
		permissions = []*model.Permission{}
	}

	roleCodes, roleErr := h.permSvc.GetUserRoleCodes(c.Request.Context(), userID)
	if roleErr != nil {
		slog.Error("SSOLogin: get user role codes failed", "error", roleErr, "user_id", userID)
	}
	if roleCodes == nil {
		roleCodes = []string{}
	}

	metrics.AuthAttempts.WithLabelValues("sso", "success").Inc()

	Success(c, gin.H{
		"token":         tokenString,
		"refresh_token": refreshToken,
		"user": map[string]interface{}{
			"id":        userID,
			"username":  username,
			"real_name": realName,
			"phone":     phone,
			"email":     email,
			"is_active": isActive,
		},
		"permissions":       permissions,
		"domains":           domains,
		"current_domain_id": currentDomainID,
		"role_codes":        roleCodes,
	})
}
