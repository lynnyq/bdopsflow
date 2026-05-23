package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code    int         `json:"code"`
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Status:  "success",
		Message: "success",
		Data:    data,
	})
}

func SuccessWithMessage(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Status:  "success",
		Message: message,
		Data:    data,
	})
}

func Fail(c *gin.Context, code int, message string) {
	c.JSON(http.StatusOK, Response{
		Code:    code,
		Status:  "error",
		Message: message,
		Data:    nil,
	})
}

func FailWithData(c *gin.Context, code int, message string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    code,
		Status:  "error",
		Message: message,
		Data:    data,
	})
}

func Error(c *gin.Context, httpStatus int, message string) {
	c.JSON(httpStatus, Response{
		Code:    httpStatus,
		Status:  "error",
		Message: message,
		Data:    nil,
	})
}

func ErrorWithData(c *gin.Context, httpStatus int, message string, data interface{}) {
	c.JSON(httpStatus, Response{
		Code:    httpStatus,
		Status:  "error",
		Message: message,
		Data:    data,
	})
}

func BadRequest(c *gin.Context, message string) {
	Fail(c, 400, message)
}

func Unauthorized(c *gin.Context, message string) {
	Fail(c, 401, message)
}

func Forbidden(c *gin.Context, message string) {
	Fail(c, 403, message)
}

func NotFound(c *gin.Context, message string) {
	Fail(c, 404, message)
}

func InternalServerError(c *gin.Context, message string) {
	Fail(c, 500, message)
}

func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Status:  "success",
		Message: "created",
		Data:    data,
	})
}
