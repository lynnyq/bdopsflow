package handler

import (
	"net/http"
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
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	query := "SELECT id, username, password, role, email, domain_id FROM users WHERE username = ?"
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{req.Username},
	}
	qr, err := h.db.QueryOneParameterized(stmt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if qr.Err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": qr.Err.Error()})
		return
	}

	if !qr.Next() {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	var userID int64
	var username, role, email, hashedPassword string
	var domainID rqlite.NullInt64
	err = qr.Scan(&userID, &username, &hashedPassword, &role, &email, &domainID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
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
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
		Email    string `json:"email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	role := req.Role
	if role == "" {
		role = "operator"
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	query := "INSERT INTO users (username, password, role, email, created_at) VALUES (?, ?, ?, ?, ?)"
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{req.Username, string(hashedPassword), role, req.Email, time.Now()},
	}
	result, err := h.db.WriteOneParameterized(stmt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if result.Err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Err.Error()})
		return
	}

	userID := result.LastInsertID
	c.JSON(http.StatusCreated, gin.H{
		"id":       userID,
		"username": req.Username,
		"role":     role,
		"email":    req.Email,
	})
}

func (h *AuthHandler) GetCurrentUser(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	query := "SELECT username, role, email, domain_id FROM users WHERE id = ?"
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{userID},
	}
	qr, err := h.db.QueryOneParameterized(stmt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if qr.Err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": qr.Err.Error()})
		return
	}

	if !qr.Next() {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	var username, role, email string
	var domainID rqlite.NullInt64
	err = qr.Scan(&username, &role, &email, &domainID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var dID int64
	if domainID.Valid {
		dID = domainID.Int64
	}

	if role == "" {
		role = "admin"
	}

	c.JSON(http.StatusOK, gin.H{
		"id":        userID,
		"username":  username,
		"role":      role,
		"email":     email,
		"domain_id": dID,
	})
}
