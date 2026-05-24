package handler

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/middleware"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/rsautil"
	rqlite "github.com/rqlite/gorqlite"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	db            *rqlite.Connection
	permSvc       *service.PermissionService
	rsaUtil       *rsautil.RSAUtil
	ssoEnabled    bool
	ssoUrl        string
	ssoRsaUtil    *rsautil.RSAUtil
	ssoTimeout    time.Duration
}

func NewAuthHandler(db *rqlite.Connection, permSvc *service.PermissionService, rsaUtil *rsautil.RSAUtil, ssoEnabled bool, ssoUrl string, ssoRsaUtil *rsautil.RSAUtil, ssoTimeout int) *AuthHandler {
	timeout := time.Duration(ssoTimeout) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &AuthHandler{
		db:         db,
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

	query := "SELECT id, username, real_name, phone, password, role, email, domain_id FROM bdopsflow_users WHERE username = ?"
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{req.Username},
	}
	qr, err := h.db.QueryOneParameterized(stmt)
	if err != nil {
		InternalServerError(c, "服务器错误，请稍后重试")
		return
	}

	if qr.Err != nil {
		InternalServerError(c, "服务器错误，请稍后重试")
		return
	}

	if !qr.Next() {
		Unauthorized(c, "用户名或密码错误")
		return
	}

	var userID int64
	var username, realName, phone, role, email, hashedPassword string
	var domainID rqlite.NullInt64
	err = qr.Scan(&userID, &username, &realName, &phone, &hashedPassword, &role, &email, &domainID)
	if err != nil {
		InternalServerError(c, "服务器错误，请稍后重试")
		return
	}

	decryptedPassword, err := h.rsaUtil.Decrypt(req.Password)
	if err != nil {
		Unauthorized(c, "用户名或密码错误")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(decryptedPassword)); err != nil {
		Unauthorized(c, "用户名或密码错误")
		return
	}

	var dID int64
	if domainID.Valid {
		dID = domainID.Int64
	}

	if role == "" {
		role = "admin"
	}

	tokenString, err := middleware.GenerateToken(userID, username, realName, role, dID)
	if err != nil {
		InternalServerError(c, "服务器错误，请稍后重试")
		return
	}

	go func() {
		updateQuery := "UPDATE bdopsflow_users SET last_login_at = ? WHERE id = ?"
		updateStmt := rqlite.ParameterizedStatement{
			Query:     updateQuery,
			Arguments: []interface{}{time.Now(), userID},
		}
		h.db.WriteOneParameterized(updateStmt)
	}()

	permissions, _ := h.permSvc.GetUserPermissions(c.Request.Context(), userID)
	permList := make([]map[string]string, 0, len(permissions))
	for _, p := range permissions {
		permList = append(permList, map[string]string{
			"resource": p.Resource,
			"action":   p.Action,
		})
	}

	Success(c, gin.H{
		"token": tokenString,
		"user": map[string]interface{}{
			"id":          userID,
			"username":    username,
			"real_name":   realName,
			"phone":       phone,
			"role":        role,
			"email":       email,
			"domain_id":   dID,
			"permissions": permList,
		},
	})
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required,min=1,max=512"`
		Role     string `json:"role"`
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

	role := req.Role
	if role == "" {
		role = "operator"
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(decryptedPassword), bcrypt.DefaultCost)
	if err != nil {
		InternalServerError(c, "服务器错误，请稍后重试")
		return
	}

	query := "INSERT INTO bdopsflow_users (username, real_name, phone, password, role, email, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)"
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{req.Username, "", "", string(hashedPassword), role, req.Email, time.Now()},
	}
	result, err := h.db.WriteOneParameterized(stmt)
	if err != nil {
		InternalServerError(c, "服务器错误，请稍后重试")
		return
	}

	if result.Err != nil {
		InternalServerError(c, "服务器错误，请稍后重试")
		return
	}

	userID := result.LastInsertID

	Created(c, gin.H{
		"id":       userID,
		"username": req.Username,
		"role":     role,
		"email":    req.Email,
	})
}

func (h *AuthHandler) GetCurrentUser(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		Unauthorized(c, "未授权，请重新登录")
		return
	}

	query := "SELECT username, real_name, phone, role, email, domain_id FROM bdopsflow_users WHERE id = ?"
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{userID},
	}
	qr, err := h.db.QueryOneParameterized(stmt)
	if err != nil {
		InternalServerError(c, "服务器错误，请稍后重试")
		return
	}

	if qr.Err != nil {
		InternalServerError(c, "服务器错误，请稍后重试")
		return
	}

	if !qr.Next() {
		NotFound(c, "用户不存在")
		return
	}

	var username, realName, phone, role, email string
	var domainID rqlite.NullInt64
	err = qr.Scan(&username, &realName, &phone, &role, &email, &domainID)
	if err != nil {
		InternalServerError(c, "服务器错误，请稍后重试")
		return
	}

	var dID int64
	if domainID.Valid {
		dID = domainID.Int64
	}

	if role == "" {
		role = "admin"
	}

	uid, _ := userID.(int64)
	permissions, _ := h.permSvc.GetUserPermissions(c.Request.Context(), uid)
	permList := make([]map[string]string, 0, len(permissions))
	for _, p := range permissions {
		permList = append(permList, map[string]string{
			"resource": p.Resource,
			"action":   p.Action,
		})
	}

	Success(c, gin.H{
		"id":          userID,
		"username":    username,
		"real_name":   realName,
		"phone":       phone,
		"role":        role,
		"email":       email,
		"domain_id":   dID,
		"permissions": permList,
	})
}

func (h *AuthHandler) GetPublicKey(c *gin.Context) {
	result := gin.H{
		"public_key": h.rsaUtil.PublicKeyB64(),
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
	ID            int64  `json:"id"`
	LoginName     string `json:"loginName"`
	IDCardName    string `json:"idCardName"`
	MobileNo      string `json:"mobileNo"`
	Email         string `json:"email"`
	DeptNo        string `json:"deptNo"`
	WorkID        string `json:"workId"`
	Gender        string `json:"gender"`
	OfficePhone   string `json:"officePhone"`
}

type ssoResponse struct {
	Code    string     `json:"code"`
	Message string     `json:"message"`
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
		Unauthorized(c, errMsg)
		return
	}

	ssoUser := ssoResp.Content
	loginName := ssoUser.LoginName
	if loginName == "" {
		loginName = req.Username
	}

	query := "SELECT id, username, real_name, phone, password, role, email, domain_id FROM bdopsflow_users WHERE username = ?"
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{loginName},
	}
	qr, err := h.db.QueryOneParameterized(stmt)
	if err != nil {
		slog.Error("SSOLogin: failed to query user", "error", err)
		InternalServerError(c, "服务器错误，请稍后重试")
		return
	}
	if qr.Err != nil {
		slog.Error("SSOLogin: query error", "error", qr.Err)
		InternalServerError(c, "服务器错误，请稍后重试")
		return
	}

	var userID int64
	var username, realName, phone, role, email string
	var domainID rqlite.NullInt64

	if qr.Next() {
		err = qr.Scan(&userID, &username, &realName, &phone, new(string), &role, &email, &domainID)
		if err != nil {
			slog.Error("SSOLogin: failed to scan user", "error", err)
			InternalServerError(c, "服务器错误，请稍后重试")
			return
		}

		go func() {
			updateQuery := "UPDATE bdopsflow_users SET last_login_at = ? WHERE id = ?"
			updateStmt := rqlite.ParameterizedStatement{
				Query:     updateQuery,
				Arguments: []interface{}{time.Now(), userID},
			}
			h.db.WriteOneParameterized(updateStmt)
		}()
	} else {
		realName = ssoUser.IDCardName
		phone = ssoUser.MobileNo
		email = ssoUser.Email
		role = "user"

		insertQuery := "INSERT INTO bdopsflow_users (username, real_name, phone, password, role, email, is_active, created_at) VALUES (?, ?, ?, '', ?, ?, 1, ?)"
		insertStmt := rqlite.ParameterizedStatement{
			Query:     insertQuery,
			Arguments: []interface{}{loginName, realName, phone, role, email, time.Now()},
		}
		result, err := h.db.WriteOneParameterized(insertStmt)
		if err != nil {
			slog.Error("SSOLogin: failed to create user", "error", err)
			InternalServerError(c, "服务器错误，请稍后重试")
			return
		}
		if result.Err != nil {
			slog.Error("SSOLogin: create user error", "error", result.Err)
			InternalServerError(c, "服务器错误，请稍后重试")
			return
		}
		userID = result.LastInsertID
		username = loginName

		slog.Info("SSOLogin: auto created user from SSO", "username", loginName, "user_id", userID)
	}

	var dID int64
	if domainID.Valid {
		dID = domainID.Int64
	}

	if role == "" {
		role = "user"
	}

	tokenString, err := middleware.GenerateToken(userID, username, realName, role, dID)
	if err != nil {
		slog.Error("SSOLogin: failed to generate token", "error", err)
		InternalServerError(c, "服务器错误，请稍后重试")
		return
	}

	permissions, _ := h.permSvc.GetUserPermissions(c.Request.Context(), userID)
	permList := make([]map[string]string, 0, len(permissions))
	for _, p := range permissions {
		permList = append(permList, map[string]string{
			"resource": p.Resource,
			"action":   p.Action,
		})
	}

	Success(c, gin.H{
		"token": tokenString,
		"user": map[string]interface{}{
			"id":          userID,
			"username":    username,
			"real_name":   realName,
			"phone":       phone,
			"role":        role,
			"email":       email,
			"domain_id":   dID,
			"permissions": permList,
		},
	})
}
