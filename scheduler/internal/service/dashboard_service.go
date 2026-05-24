package service

import (
	"context"
	"fmt"
	"time"
)

type DashboardStats struct {
	Tasks struct {
		Total       int64 `json:"total"`
		Enabled     int64 `json:"enabled"`
		Cron        int64 `json:"cron"`
		Running     int64 `json:"running"`
		Success     int64 `json:"success"`
		Failed      int64 `json:"failed"`
		AvgDuration int64 `json:"avg_duration"`
	} `json:"tasks"`
	Workflows struct {
		Total   int64 `json:"total"`
		Enabled int64 `json:"enabled"`
	} `json:"workflows"`
	Executors struct {
		Total  int64 `json:"total"`
		Active int64 `json:"active"`
	} `json:"executors"`
	Scheduler struct {
		Paused bool  `json:"paused"`
		Uptime int64 `json:"uptime"`
	} `json:"scheduler"`
}

type TrendData struct {
	Date    string `json:"date"`
	Total   int64  `json:"total"`
	Success int64  `json:"success"`
	Failed  int64  `json:"failed"`
}

func (s *SchedulerService) GetDashboardStats(ctx context.Context, domainID int64, role string) (*DashboardStats, error) {
	stats := &DashboardStats{}
	isSystemAdmin := role == "system_admin" || role == "admin"

	var taskQuery string
	var args []interface{}
	if isSystemAdmin {
		taskQuery = `
			SELECT
				COUNT(*) as total,
				SUM(CASE WHEN is_enabled = 1 THEN 1 ELSE 0 END) as enabled,
				SUM(CASE WHEN cron_expression IS NOT NULL AND cron_expression != '' THEN 1 ELSE 0 END) as cron
			FROM bdopsflow_tasks
		`
	} else {
		taskQuery = `
			SELECT
				COUNT(*) as total,
				SUM(CASE WHEN is_enabled = 1 THEN 1 ELSE 0 END) as enabled,
				SUM(CASE WHEN cron_expression IS NOT NULL AND cron_expression != '' THEN 1 ELSE 0 END) as cron
			FROM bdopsflow_tasks WHERE domain_id = ?
		`
		args = append(args, domainID)
	}
	qr, err := s.executeQuery(taskQuery, args)
	if err != nil {
		return nil, err
	}
	if qr.Next() {
		row, _ := qr.Slice()
		stats.Tasks.Total = rowInt64(row[0])
		stats.Tasks.Enabled = rowInt64(row[1])
		stats.Tasks.Cron = rowInt64(row[2])
	}

	var runningQuery string
	args = []interface{}{}
	if isSystemAdmin {
		runningQuery = `SELECT COUNT(*) FROM bdopsflow_task_executions WHERE status = 'running'`
	} else {
		runningQuery = `
			SELECT COUNT(*)
			FROM bdopsflow_task_executions te
			JOIN bdopsflow_tasks t ON te.task_id = t.id
			WHERE te.status = 'running' AND t.domain_id = ?
		`
		args = append(args, domainID)
	}
	qr, err = s.executeQuery(runningQuery, args)
	if err == nil && qr.Next() {
		row, _ := qr.Slice()
		stats.Tasks.Running = rowInt64(row[0])
	}

	var recentExecQuery string
	args = []interface{}{}
	if isSystemAdmin {
		recentExecQuery = `
			SELECT
				SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success,
				SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed,
				AVG(CASE WHEN end_time IS NOT NULL AND start_time IS NOT NULL
					THEN julianday(end_time) - julianday(start_time) ELSE 0 END) * 86400 as avg_duration
			FROM bdopsflow_task_executions
			WHERE created_at > datetime('now', '-7 days')
		`
	} else {
		recentExecQuery = `
			SELECT
				SUM(CASE WHEN te.status = 'success' THEN 1 ELSE 0 END) as success,
				SUM(CASE WHEN te.status = 'failed' THEN 1 ELSE 0 END) as failed,
				AVG(CASE WHEN te.end_time IS NOT NULL AND te.start_time IS NOT NULL
					THEN julianday(te.end_time) - julianday(te.start_time) ELSE 0 END) * 86400 as avg_duration
			FROM bdopsflow_task_executions te
			JOIN bdopsflow_tasks t ON te.task_id = t.id
			WHERE te.created_at > datetime('now', '-7 days') AND t.domain_id = ?
		`
		args = append(args, domainID)
	}
	qr, err = s.executeQuery(recentExecQuery, args)
	if err == nil && qr.Next() {
		row, _ := qr.Slice()
		stats.Tasks.Success = rowInt64(row[0])
		stats.Tasks.Failed = rowInt64(row[1])
		stats.Tasks.AvgDuration = int64(rowFloat64(row[2]))
	}

	var wfQuery string
	args = []interface{}{}
	if isSystemAdmin {
		wfQuery = `
			SELECT
				COUNT(*) as total,
				SUM(CASE WHEN is_enabled = 1 THEN 1 ELSE 0 END) as enabled
			FROM bdopsflow_workflows
		`
	} else {
		wfQuery = `
			SELECT
				COUNT(*) as total,
				SUM(CASE WHEN is_enabled = 1 THEN 1 ELSE 0 END) as enabled
			FROM bdopsflow_workflows WHERE domain_id = ?
		`
		args = append(args, domainID)
	}
	qr, err = s.executeQuery(wfQuery, args)
	if err == nil && qr.Next() {
		row, _ := qr.Slice()
		stats.Workflows.Total = rowInt64(row[0])
		stats.Workflows.Enabled = rowInt64(row[1])
	}

	var execQuery string
	args = []interface{}{}
	if isSystemAdmin {
		execQuery = `
			SELECT
				COUNT(*) as total,
				SUM(CASE WHEN status = 'online' THEN 1 ELSE 0 END) as online
			FROM bdopsflow_executors
		`
	} else {
		execQuery = `
			SELECT
				COUNT(DISTINCT e.id) as total,
				SUM(CASE WHEN e.status = 'online' THEN 1 ELSE 0 END) as online
			FROM bdopsflow_executors e
			JOIN bdopsflow_domain_executors de ON e.id = de.executor_id
			WHERE de.domain_id = ?
		`
		args = append(args, domainID)
	}
	qr, err = s.executeQuery(execQuery, args)
	if err == nil && qr.Next() {
		row, _ := qr.Slice()
		stats.Executors.Total = rowInt64(row[0])
		stats.Executors.Active = rowInt64(row[1])
	}

	if s.cronScheduler != nil {
		stats.Scheduler.Paused = s.cronScheduler.IsPaused()
		stats.Scheduler.Uptime = int64(s.cronScheduler.GetUptime().Seconds())
	}

	return stats, nil
}

func (s *SchedulerService) GetTrendData(ctx context.Context, domainID int64, role string) ([]*TrendData, error) {
	var trends []*TrendData
	isSystemAdmin := role == "system_admin" || role == "admin"

	var query string
	var args []interface{}
	if isSystemAdmin {
		query = `
			SELECT
				date(created_at) as exec_date,
				COUNT(*) as total,
				SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success,
				SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed
			FROM bdopsflow_task_executions
			WHERE created_at > datetime('now', '-7 days')
			GROUP BY date(created_at)
			ORDER BY exec_date DESC
		`
	} else {
		query = `
			SELECT
				date(te.created_at) as exec_date,
				COUNT(*) as total,
				SUM(CASE WHEN te.status = 'success' THEN 1 ELSE 0 END) as success,
				SUM(CASE WHEN te.status = 'failed' THEN 1 ELSE 0 END) as failed
			FROM bdopsflow_task_executions te
			JOIN bdopsflow_tasks t ON te.task_id = t.id
			WHERE te.created_at > datetime('now', '-7 days') AND t.domain_id = ?
			GROUP BY date(te.created_at)
			ORDER BY exec_date DESC
		`
		args = append(args, domainID)
	}

	qr, err := s.executeQuery(query, args)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	for qr.Next() {
		row, _ := qr.Slice()
		trend := &TrendData{
			Date:    rowString(row[0]),
			Total:   rowInt64(row[1]),
			Success: rowInt64(row[2]),
			Failed:  rowInt64(row[3]),
		}
		trends = append(trends, trend)
	}

	return trends, nil
}

func (s *SchedulerService) PauseScheduler() {
	if s.cronScheduler != nil {
		s.cronScheduler.Pause()
	}
}

func (s *SchedulerService) ResumeScheduler() {
	if s.cronScheduler != nil {
		s.cronScheduler.Resume()
	}
}

func (s *SchedulerService) IsSchedulerPaused() bool {
	if s.cronScheduler != nil {
		return s.cronScheduler.IsPaused()
	}
	return false
}

type HealthCheckResult struct {
	Status     string                      `json:"status"`
	Timestamp  string                      `json:"timestamp"`
	Components map[string]ComponentCheck   `json:"components"`
}

type ComponentCheck struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Latency string `json:"latency,omitempty"`
}

var requiredTables = []string{
	"bdopsflow_domains",
	"bdopsflow_users",
	"bdopsflow_workflows",
	"bdopsflow_tasks",
	"bdopsflow_task_executions",
	"bdopsflow_executors",
	"bdopsflow_workflow_executions",
	"bdopsflow_task_logs",
	"bdopsflow_roles",
	"bdopsflow_permissions",
	"bdopsflow_role_permissions",
	"bdopsflow_user_roles",
	"bdopsflow_domain_executors",
}

func (s *SchedulerService) HealthCheck(ctx context.Context) *HealthCheckResult {
	result := &HealthCheckResult{
		Timestamp:  time.Now().Format("2006-01-02 15:04:05"),
		Components: make(map[string]ComponentCheck),
	}

	allHealthy := true

	rqliteCheck := s.checkRQLite()
	result.Components["rqlite"] = rqliteCheck
	if rqliteCheck.Status != "healthy" {
		allHealthy = false
	}

	tableCheck := s.checkTables()
	result.Components["rqlite_tables"] = tableCheck
	if tableCheck.Status != "healthy" {
		allHealthy = false
	}

	redisCheck := s.checkRedis()
	result.Components["redis"] = redisCheck
	if redisCheck.Status != "healthy" {
		allHealthy = false
	}

	schedulerCheck := s.checkScheduler()
	result.Components["scheduler"] = schedulerCheck
	if schedulerCheck.Status != "healthy" {
		allHealthy = false
	}

	if allHealthy {
		result.Status = "healthy"
	} else {
		result.Status = "unhealthy"
	}

	return result
}

func (s *SchedulerService) checkRQLite() ComponentCheck {
	start := time.Now()

	query := "SELECT 1"
	qr, err := s.DB.QueryOne(query)
	latency := time.Since(start)

	if err != nil {
		return ComponentCheck{
			Status:  "unhealthy",
			Message: fmt.Sprintf("连接失败: %v", err),
			Latency: latency.String(),
		}
	}
	if qr.Err != nil {
		return ComponentCheck{
			Status:  "unhealthy",
			Message: fmt.Sprintf("查询失败: %v", qr.Err),
			Latency: latency.String(),
		}
	}

	return ComponentCheck{
		Status:  "healthy",
		Message: "连接正常",
		Latency: latency.String(),
	}
}

func (s *SchedulerService) checkTables() ComponentCheck {
	missingTables := []string{}

	for _, tableName := range requiredTables {
		query := fmt.Sprintf("SELECT name FROM sqlite_master WHERE type='table' AND name='%s'", tableName)
		qr, err := s.DB.QueryOne(query)
		if err != nil {
			missingTables = append(missingTables, tableName)
			continue
		}
		if qr.Err != nil || !qr.Next() {
			missingTables = append(missingTables, tableName)
			continue
		}
	}

	if len(missingTables) > 0 {
		return ComponentCheck{
			Status:  "unhealthy",
			Message: fmt.Sprintf("缺少表: %v", missingTables),
		}
	}

	return ComponentCheck{
		Status:  "healthy",
		Message: fmt.Sprintf("所有 %d 个表正常", len(requiredTables)),
	}
}

func (s *SchedulerService) checkRedis() ComponentCheck {
	start := time.Now()

	err := s.redis.Ping(context.Background()).Err()
	latency := time.Since(start)

	if err != nil {
		return ComponentCheck{
			Status:  "unhealthy",
			Message: fmt.Sprintf("连接失败: %v", err),
			Latency: latency.String(),
		}
	}

	return ComponentCheck{
		Status:  "healthy",
		Message: "连接正常",
		Latency: latency.String(),
	}
}

func (s *SchedulerService) checkScheduler() ComponentCheck {
	paused := s.IsSchedulerPaused()

	if s.cronScheduler == nil {
		return ComponentCheck{
			Status:  "unhealthy",
			Message: "调度器未初始化",
		}
	}

	if paused {
		return ComponentCheck{
			Status:  "unhealthy",
			Message: "已暂停",
		}
	}

	return ComponentCheck{
		Status:  "healthy",
		Message: "运行中",
	}
}
