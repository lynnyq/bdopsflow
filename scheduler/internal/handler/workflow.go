package handler

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
)

type WorkflowHandler struct {
	svc *service.SchedulerService
}

func NewWorkflowHandler(svc *service.SchedulerService) *WorkflowHandler {
	return &WorkflowHandler{svc: svc}
}

func (h *WorkflowHandler) List(c *gin.Context) {
	ctx := c.Request.Context()

	defer func() {
		if r := recover(); r != nil {
			slog.Error("WorkflowHandler.List: panic recovered", "panic", r)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
	}()

	slog.Debug("WorkflowHandler.List: handling request")

	bdopsflow_workflows, err := h.svc.ListWorkflows(ctx)
	if err != nil {
		slog.Error("WorkflowHandler.List: failed to list bdopsflow_workflows", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, bdopsflow_workflows)
}

func (h *WorkflowHandler) Get(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("WorkflowHandler.Get: invalid id", "id_str", idStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if id <= 0 {
		slog.Warn("WorkflowHandler.Get: id must be positive", "id", id)
		c.JSON(http.StatusBadRequest, gin.H{"error": "id must be positive"})
		return
	}

	ctx := c.Request.Context()
	wf, err := h.svc.GetWorkflow(ctx, id)
	if err != nil {
		slog.Error("WorkflowHandler.Get: failed to get workflow", "id", id, "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow not found"})
		return
	}

	c.JSON(http.StatusOK, wf)
}

func (h *WorkflowHandler) Create(c *gin.Context) {
	var req struct {
		Name           string `json:"name"`
		Description    string `json:"description"`
		DomainID       int64  `json:"domain_id"`
		DAGConfig      string `json:"dag_config"`
		CronExpression string `json:"cron_expression"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("WorkflowHandler.Create: invalid request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if safeString(req.Name) == "" {
		slog.Warn("WorkflowHandler.Create: name is required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	if req.DomainID <= 0 {
		req.DomainID = 1
	}

	now := time.Now()
	ctx := c.Request.Context()
	wf, err := h.svc.CreateWorkflow(ctx,
		`INSERT INTO bdopsflow_workflows (name, description, domain_id, dag_config, cron_expression, is_enabled, created_by, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, 1, 1, ?, ?)`,
		safeString(req.Name), safeString(req.Description), req.DomainID,
		safeString(req.DAGConfig), safeString(req.CronExpression), now, now,
	)
	if err != nil {
		slog.Error("WorkflowHandler.Create: failed to create workflow", "name", req.Name, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	slog.Info("WorkflowHandler.Create: workflow created", "workflow_id", wf.ID, "name", wf.Name)
	c.JSON(http.StatusCreated, wf)
}

func (h *WorkflowHandler) Update(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("WorkflowHandler.Update: invalid id", "id_str", idStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if id <= 0 {
		slog.Warn("WorkflowHandler.Update: id must be positive", "id", id)
		c.JSON(http.StatusBadRequest, gin.H{"error": "id must be positive"})
		return
	}

	var wf model.Workflow
	if err := c.ShouldBindJSON(&wf); err != nil {
		slog.Warn("WorkflowHandler.Update: invalid request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	err = h.svc.UpdateWorkflow(ctx, id, &wf)
	if err != nil {
		slog.Error("WorkflowHandler.Update: failed to update workflow", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	wf.ID = id
	slog.Info("WorkflowHandler.Update: workflow updated", "id", id)
	c.JSON(http.StatusOK, wf)
}

func (h *WorkflowHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("WorkflowHandler.Delete: invalid id", "id_str", idStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if id <= 0 {
		slog.Warn("WorkflowHandler.Delete: id must be positive", "id", id)
		c.JSON(http.StatusBadRequest, gin.H{"error": "id must be positive"})
		return
	}

	ctx := c.Request.Context()
	err = h.svc.DeleteWorkflow(ctx, id)
	if err != nil {
		slog.Error("WorkflowHandler.Delete: failed to delete workflow", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	slog.Info("WorkflowHandler.Delete: workflow deleted", "id", id)
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *WorkflowHandler) TriggerWorkflow(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("WorkflowHandler.TriggerWorkflow: invalid id", "id_str", idStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if id <= 0 {
		slog.Warn("WorkflowHandler.TriggerWorkflow: id must be positive", "id", id)
		c.JSON(http.StatusBadRequest, gin.H{"error": "id must be positive"})
		return
	}

	ctx := c.Request.Context()
	we, err := h.svc.TriggerWorkflow(ctx, id)
	if err != nil {
		slog.Error("WorkflowHandler.TriggerWorkflow: failed to trigger workflow", "workflow_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	slog.Info("WorkflowHandler.TriggerWorkflow: workflow triggered", "workflow_id", id, "execution_id", we.ExecutionID)
	c.JSON(http.StatusOK, we)
}

func (h *WorkflowHandler) GetWorkflowExecutions(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("WorkflowHandler.GetWorkflowExecutions: invalid id", "id_str", idStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if id <= 0 {
		slog.Warn("WorkflowHandler.GetWorkflowExecutions: id must be positive", "id", id)
		c.JSON(http.StatusBadRequest, gin.H{"error": "id must be positive"})
		return
	}

	ctx := c.Request.Context()
	executions, err := h.svc.ListWorkflowExecutions(ctx, id)
	if err != nil {
		slog.Error("WorkflowHandler.GetWorkflowExecutions: failed to list executions", "workflow_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, executions)
}

func (h *WorkflowHandler) GetWorkflowExecution(c *gin.Context) {
	executionID := c.Param("executionId")
	if safeString(executionID) == "" {
		slog.Warn("WorkflowHandler.GetWorkflowExecution: executionId required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "executionId required"})
		return
	}

	ctx := c.Request.Context()
	we, err := h.svc.GetWorkflowExecutionByExecutionID(ctx, executionID)
	if err != nil {
		slog.Error("WorkflowHandler.GetWorkflowExecution: failed to get workflow execution", "execution_id", executionID, "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow execution not found"})
		return
	}

	c.JSON(http.StatusOK, we)
}

func (h *WorkflowHandler) GetExecutionLogs(c *gin.Context) {
	executionID := c.Param("executionId")
	if safeString(executionID) == "" {
		slog.Warn("WorkflowHandler.GetExecutionLogs: executionId required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "executionId required"})
		return
	}

	ctx := c.Request.Context()
	logs, err := h.svc.GetTaskLogsByExecutionID(ctx, executionID)
	if err != nil {
		slog.Error("WorkflowHandler.GetExecutionLogs: failed to get logs", "execution_id", executionID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var response []*TaskLogResponse
	for _, log := range logs {
		response = append(response, toTaskLogResponse(log))
	}

	c.JSON(http.StatusOK, response)
}
