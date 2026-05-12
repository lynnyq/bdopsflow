package handler

import (
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
	workflows, err := h.svc.ListWorkflows(ctx)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, workflows)
}

func (h *WorkflowHandler) Get(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid id"})
		return
	}

	ctx := c.Request.Context()
	wf, err := h.svc.GetWorkflow(ctx, id)
	if err != nil {
		c.JSON(404, gin.H{"error": "workflow not found"})
		return
	}
	c.JSON(200, wf)
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
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	query := `
		INSERT INTO workflows (name, description, domain_id, dag_config, cron_expression, is_enabled, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 1, 1, ?, ?)
	`

	now := time.Now()
	ctx := c.Request.Context()
	wf, err := h.svc.CreateWorkflow(ctx, query,
		req.Name, req.Description, req.DomainID, req.DAGConfig, req.CronExpression, now, now,
	)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(201, wf)
}

func (h *WorkflowHandler) Update(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid id"})
		return
	}

	var wf model.Workflow
	if err := c.ShouldBindJSON(&wf); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	err = h.svc.UpdateWorkflow(ctx, id, &wf)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	wf.ID = id
	c.JSON(200, wf)
}

func (h *WorkflowHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid id"})
		return
	}

	ctx := c.Request.Context()
	err = h.svc.DeleteWorkflow(ctx, id)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "deleted"})
}

// TriggerWorkflow 触发工作流执行
func (h *WorkflowHandler) TriggerWorkflow(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid id"})
		return
	}

	ctx := c.Request.Context()
	we, err := h.svc.TriggerWorkflow(ctx, id)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, we)
}

// GetWorkflowExecutions 获取工作流的所有执行记录
func (h *WorkflowHandler) GetWorkflowExecutions(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid id"})
		return
	}

	ctx := c.Request.Context()
	executions, err := h.svc.ListWorkflowExecutions(ctx, id)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, executions)
}

// GetWorkflowExecution 获取单个工作流执行记录
func (h *WorkflowHandler) GetWorkflowExecution(c *gin.Context) {
	executionID := c.Param("executionId")
	ctx := c.Request.Context()
	we, err := h.svc.GetWorkflowExecutionByExecutionID(ctx, executionID)
	if err != nil {
		c.JSON(404, gin.H{"error": "workflow execution not found"})
		return
	}

	c.JSON(200, we)
}

// GetExecutionLogs 获取执行的日志
func (h *WorkflowHandler) GetExecutionLogs(c *gin.Context) {
	executionID := c.Param("executionId")
	ctx := c.Request.Context()
	logs, err := h.svc.GetTaskLogsByExecutionID(ctx, executionID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, logs)
}
