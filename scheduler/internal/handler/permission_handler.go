package handler

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
)

type PermissionHandler struct {
	svc *service.PermissionService
}

func NewPermissionHandler(svc *service.PermissionService) *PermissionHandler {
	return &PermissionHandler{svc: svc}
}

func (h *PermissionHandler) GetAllPermissions(c *gin.Context) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("PermissionHandler.GetAllPermissions: panic recovered", "panic", r)
			InternalServerError(c, "internal server error")
		}
	}()

	slog.Debug("PermissionHandler.GetAllPermissions: handling request")

	bdopsflow_permissions := model.GetAllPermissionGroups()

	Success(c, gin.H{"items": bdopsflow_permissions})
}
