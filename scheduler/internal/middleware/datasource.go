package middleware

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type datasourceResponse struct {
	Code    int         `json:"code"`
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

type DatasourceGetter interface {
	GetDatasourceDomainID(dsID int64) (int64, error)
	CheckDatasourcePermission(userID int64, dsID int64, action string) (bool, error)
}

func DatasourcePermissionMiddleware(dsSvc DatasourceGetter, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, _ := c.Get("role")
		userID, _ := c.Get("user_id")
		domainID, _ := c.Get("domain_id")

		role := userRole.(string)
		uID := userID.(int64)
		dID, _ := domainID.(int64)

		if role == "system_admin" || role == "admin" {
			c.Next()
			return
		}

		dsID := getDatasourceID(c)
		if dsID == 0 {
			c.JSON(http.StatusForbidden, datasourceResponse{
				Code:    http.StatusForbidden,
				Status:  "error",
				Message: "缺少数据源标识，无法进行权限校验",
				Data:    nil,
			})
			c.Abort()
			return
		}

		dsDomainID, err := dsSvc.GetDatasourceDomainID(dsID)
		if err != nil {
			c.JSON(http.StatusNotFound, datasourceResponse{
				Code:    http.StatusNotFound,
				Status:  "error",
				Message: "数据源不存在",
				Data:    nil,
			})
			c.Abort()
			return
		}

		if role == "domain_admin" && dsDomainID == dID {
			c.Set("datasource_id", dsID)
			c.Next()
			return
		}

		hasPerm, err := dsSvc.CheckDatasourcePermission(uID, dsID, action)
		if err != nil {
			c.JSON(http.StatusInternalServerError, datasourceResponse{
				Code:    http.StatusInternalServerError,
				Status:  "error",
				Message: "权限校验失败，请稍后重试",
				Data:    nil,
			})
			c.Abort()
			return
		}
		if !hasPerm {
			actionLabel := map[string]string{
				"read":     "查看",
				"query":    "查询",
				"download": "下载",
				"update":   "编辑",
				"delete":   "删除",
				"manage":   "管理",
				"write":    "写入",
			}
			label := actionLabel[action]
			if label == "" {
				label = action
			}
			c.JSON(http.StatusForbidden, datasourceResponse{
				Code:    http.StatusForbidden,
				Status:  "error",
				Message: fmt.Sprintf("您没有该数据源的%s权限，请联系管理员开通", label),
				Data:    nil,
			})
			c.Abort()
			return
		}

		c.Set("datasource_id", dsID)
		c.Next()
	}
}

func getDatasourceID(c *gin.Context) int64 {
	dsIDStr := c.Param("id")
	if dsIDStr != "" {
		if id, err := strconv.ParseInt(dsIDStr, 10, 64); err == nil {
			return id
		}
	}

	dsIDStr = c.Query("datasource_id")
	if dsIDStr != "" {
		if id, err := strconv.ParseInt(dsIDStr, 10, 64); err == nil {
			return id
		}
	}

	if c.Request.Method == http.MethodPost || c.Request.Method == http.MethodPut {
		if c.ContentType() == "application/json" {
			bodyBytes, err := io.ReadAll(c.Request.Body)
			if err != nil {
				return 0
			}

			c.Request.Body = io.NopCloser(bytesReader{bodyBytes})

			var body struct {
				DatasourceID int64 `json:"datasource_id"`
			}
			if json.Unmarshal(bodyBytes, &body) == nil && body.DatasourceID > 0 {
				return body.DatasourceID
			}
		}
	}

	return 0
}

type bytesReader struct {
	data []byte
}

func (r bytesReader) Read(p []byte) (n int, err error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	n = copy(p, r.data)
	r.data = r.data[n:]
	return
}
