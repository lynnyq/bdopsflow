package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
)

type Response struct {
	Code    int         `json:"code"`
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

type PaginatedResponse struct {
	Code    int         `json:"code"`
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
	Total   int         `json:"total"`
	Page    int         `json:"page"`
	PageSize int        `json:"page_size"`
}

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    CodeSuccess,
		Status:  "success",
		Message: "success",
		Data:    data,
	})
}

func SuccessWithMessage(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    CodeSuccess,
		Status:  "success",
		Message: message,
		Data:    data,
	})
}

func SuccessPaginated(c *gin.Context, data interface{}, total, page, pageSize int) {
	c.JSON(http.StatusOK, PaginatedResponse{
		Code:     CodeSuccess,
		Status:   "success",
		Message:  "success",
		Data:     data,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
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

func FailFromError(c *gin.Context, err error) {
	code := service.GetErrorCode(err)
	Fail(c, code, err.Error())
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
	Fail(c, CodeBadRequest, message)
}

func Unauthorized(c *gin.Context, message string) {
	Fail(c, CodeUnauthorized, message)
}

func Forbidden(c *gin.Context, message string) {
	Fail(c, CodeForbidden, message)
}

func NotFound(c *gin.Context, message string) {
	Fail(c, CodeNotFound, message)
}

func InternalServerError(c *gin.Context, message string) {
	Fail(c, CodeInternalError, message)
}

func ServiceUnavailable(c *gin.Context, message string) {
	Fail(c, CodeServiceUnavailable, message)
}

func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    CodeSuccess,
		Status:  "success",
		Message: "created",
		Data:    data,
	})
}
