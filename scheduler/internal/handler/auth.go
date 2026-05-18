package handler

import (
	"encoding/base64"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/middleware"
	rqlite "github.com/rqlite/gorqlite"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	db *rqlite.Connection
}

func NewAuthHandler(db *rqlite.Connection) *AuthHandler {
	return &AuthHandler{
		db: db,
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

	query := "SELECT id, username, password, role, email, domain_id FROM bdopsflow_users WHERE username = ?"
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
	var username, role, email, hashedPassword string
	var domainID rqlite.NullInt64
	err = qr.Scan(&userID, &username, &hashedPassword, &role, &email, &domainID)
	if err != nil {
		InternalServerError(c, "服务器错误，请稍后重试")
		return
	}

	// 尝试解码密码，如果解码失败则使用原始密码（兼容旧数据）
	passwordToCheck := req.Password
	if decodedPassword, err := base64.StdEncoding.DecodeString(req.Password); err == nil {
		passwordToCheck = string(decodedPassword)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(passwordToCheck)); err != nil {
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

	tokenString, err := middleware.GenerateToken(userID, username, role, dID)
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

	Success(c, gin.H{
		"token": tokenString,
		"user": map[string]interface{}{
			"id":        userID,
			"username":  username,
			"role":      role,
			"email":     email,
			"domain_id": dID,
		},
	})
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required,min=6"`
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

	if len(req.Password) < 6 {
		BadRequest(c, "密码长度至少6位")
		return
	}

	role := req.Role
	if role == "" {
		role = "operator"
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		InternalServerError(c, "服务器错误，请稍后重试")
		return
	}

	query := "INSERT INTO bdopsflow_users (username, password, role, email, created_at) VALUES (?, ?, ?, ?, ?)"
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{req.Username, string(hashedPassword), role, req.Email, time.Now()},
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

	query := "SELECT username, role, email, domain_id FROM bdopsflow_users WHERE id = ?"
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

	var username, role, email string
	var domainID rqlite.NullInt64
	err = qr.Scan(&username, &role, &email, &domainID)
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

	Success(c, gin.H{
		"id":        userID,
		"username":  username,
		"role":      role,
		"email":     email,
		"domain_id": dID,
	})
}
