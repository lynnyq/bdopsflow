package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 统一响应结构
type Response struct {
	Code    int         `json:"code"`    // 业务状态码，0表示成功，非0表示失败
	Status  string      `json:"status"`  // 状态："success" 或 "error"
	Message string      `json:"message"` // 提示信息
	Data    interface{} `json:"data"`    // 数据
}

// HTTPStatus 获取HTTP状态码对应的业务码
func HTTPStatus(httpStatus int) int {
	// 将HTTP状态码映射为业务码
	// 2xx -> 0 (成功)
	// 4xx, 5xx -> 对应的HTTP状态码
	if httpStatus >= 200 && httpStatus < 300 {
		return 0
	}
	return httpStatus
}

// Success 成功响应
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Status:  "success",
		Message: "success",
		Data:    data,
	})
}

// SuccessWithMessage 成功响应（带自定义消息）
func SuccessWithMessage(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Status:  "success",
		Message: message,
		Data:    data,
	})
}

// Error 错误响应
func Error(c *gin.Context, httpStatus int, message string) {
	c.JSON(httpStatus, Response{
		Code:    HTTPStatus(httpStatus),
		Status:  "error",
		Message: message,
		Data:    nil,
	})
}

// ErrorWithData 错误响应（带数据）
func ErrorWithData(c *gin.Context, httpStatus int, message string, data interface{}) {
	c.JSON(httpStatus, Response{
		Code:    HTTPStatus(httpStatus),
		Status:  "error",
		Message: message,
		Data:    data,
	})
}

// BadRequest 400错误
func BadRequest(c *gin.Context, message string) {
	Error(c, http.StatusBadRequest, message)
}

// Unauthorized 401错误
func Unauthorized(c *gin.Context, message string) {
	Error(c, http.StatusUnauthorized, message)
}

// Forbidden 403错误
func Forbidden(c *gin.Context, message string) {
	Error(c, http.StatusForbidden, message)
}

// NotFound 404错误
func NotFound(c *gin.Context, message string) {
	Error(c, http.StatusNotFound, message)
}

// InternalServerError 500错误
func InternalServerError(c *gin.Context, message string) {
	Error(c, http.StatusInternalServerError, message)
}

// Created 201创建成功
func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, Response{
		Code:    0,
		Status:  "success",
		Message: "created",
		Data:    data,
	})
}
