package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
)

type ApiTestHandler struct {
	apiTestSvc *service.ApiTestService
	httpExec   *service.HTTPExecutor
	grpcExec   *service.GRPCExecutor
	protoSvc   *service.ProtoService
	certSvc    *service.CertificateService
	permSvc    *service.PermissionService
}

func NewApiTestHandler(
	apiTestSvc *service.ApiTestService,
	httpExec *service.HTTPExecutor,
	grpcExec *service.GRPCExecutor,
	protoSvc *service.ProtoService,
	certSvc *service.CertificateService,
	permSvc *service.PermissionService,
) *ApiTestHandler {
	return &ApiTestHandler{
		apiTestSvc: apiTestSvc,
		httpExec:   httpExec,
		grpcExec:   grpcExec,
		protoSvc:   protoSvc,
		certSvc:    certSvc,
		permSvc:    permSvc,
	}
}

// List returns test cases for the current user with optional type filter and pagination.
func (h *ApiTestHandler) List(c *gin.Context) {
	userID := extractUserID(c)
	if userID == 0 {
		Unauthorized(c, "用户未登录")
		return
	}

	testType := c.Query("type")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	tests, total, err := h.apiTestSvc.ListByUser(c.Request.Context(), userID, testType, page, pageSize)
	if err != nil {
		slog.Error("failed to list api tests", "user_id", userID, "error", err)
		Fail(c, CodeQueryError, "获取接口测试列表失败")
		return
	}

	if tests == nil {
		tests = []*model.ApiTest{}
	}

	Success(c, gin.H{
		"items":     tests,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// Create creates a new test case.
func (h *ApiTestHandler) Create(c *gin.Context) {
	var req struct {
		Name   string `json:"name" binding:"required"`
		Type   string `json:"type" binding:"required"`
		Config string `json:"config"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	if req.Type != "http" && req.Type != "grpc" {
		BadRequest(c, "测试类型必须为 http 或 grpc")
		return
	}

	userID := extractUserID(c)
	if userID == 0 {
		Unauthorized(c, "用户未登录")
		return
	}

	test := &model.ApiTest{
		Name:      req.Name,
		Type:      req.Type,
		Config:    req.Config,
		CreatedBy: userID,
	}

	created, err := h.apiTestSvc.Create(c.Request.Context(), test)
	if err != nil {
		slog.Error("failed to create api test", "name", req.Name, "error", err)
		Fail(c, CodeQueryError, "创建接口测试失败")
		return
	}

	slog.Info("api test created", "id", created.ID, "name", created.Name)
	c.Set("audit_resource_id", strconv.FormatInt(created.ID, 10))
	c.Set("audit_resource_name", created.Name)
	Created(c, gin.H{"id": created.ID})
}

// Get returns a single test case by ID.
func (h *ApiTestHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的测试ID")
		return
	}

	test, err := h.apiTestSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		NotFound(c, "接口测试不存在")
		return
	}

	if !checkOwnership(c, h.permSvc, test.CreatedBy) {
		return
	}

	Success(c, test)
}

// Update modifies an existing test case.
func (h *ApiTestHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的测试ID")
		return
	}

	var req struct {
		Name   *string `json:"name"`
		Type   *string `json:"type"`
		Config *string `json:"config"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	test, err := h.apiTestSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		NotFound(c, "接口测试不存在")
		return
	}

	if !checkOwnership(c, h.permSvc, test.CreatedBy) {
		return
	}

	if req.Name != nil {
		test.Name = *req.Name
	}
	if req.Type != nil {
		if *req.Type != "http" && *req.Type != "grpc" {
			BadRequest(c, "测试类型必须为 http 或 grpc")
			return
		}
		test.Type = *req.Type
	}
	if req.Config != nil {
		test.Config = *req.Config
	}

	if err := h.apiTestSvc.Update(c.Request.Context(), id, test); err != nil {
		slog.Error("failed to update api test", "id", id, "error", err)
		Fail(c, CodeQueryError, "更新接口测试失败")
		return
	}

	slog.Info("api test updated", "id", id, "name", test.Name)
	c.Set("audit_resource_id", strconv.FormatInt(id, 10))
	c.Set("audit_resource_name", test.Name)
	Success(c, nil)
}

// Delete removes a test case by ID.
func (h *ApiTestHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的测试ID")
		return
	}

	test, err := h.apiTestSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		NotFound(c, "接口测试不存在")
		return
	}

	if !checkOwnership(c, h.permSvc, test.CreatedBy) {
		return
	}

	if err := h.apiTestSvc.Delete(c.Request.Context(), id); err != nil {
		slog.Error("failed to delete api test", "id", id, "error", err)
		Fail(c, CodeQueryError, "删除接口测试失败")
		return
	}

	slog.Info("api test deleted", "id", id)
	c.Set("audit_resource_id", strconv.FormatInt(id, 10))
	c.Set("audit_resource_name", test.Name)
	Success(c, nil)
}

// Execute runs a temporary (unsaved) test and returns the result.
func (h *ApiTestHandler) Execute(c *gin.Context) {
	userID := extractUserID(c)
	if userID == 0 {
		Unauthorized(c, "用户未登录")
		return
	}

	var req struct {
		Type       string                  `json:"type" binding:"required"`
		Config     string                  `json:"config" binding:"required"`
		SaveResult *bool                   `json:"save_result"`
		Assertions []model.AssertionConfig `json:"assertions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	var result *model.ApiTestResult

	switch req.Type {
	case "http":
		var httpConfig model.HTTPRequestConfig
		if err := json.Unmarshal([]byte(req.Config), &httpConfig); err != nil {
			BadRequest(c, "无效的HTTP请求配置")
			return
		}

		execResult, execErr := h.httpExec.Execute(c.Request.Context(), &httpConfig)
		if execErr != nil {
			slog.Error("failed to execute http test", "error", execErr)
			FailWithData(c, CodeApiTestExecuteFailed, "HTTP请求执行失败", gin.H{
				"error": execErr.Error(),
			})
			return
		}
		result = execResult
		result.Type = "http"

		// 执行断言：优先使用请求中的 assertions 字段，回退到 PostScript
		var assertions []model.AssertionConfig
		if len(req.Assertions) > 0 {
			assertions = req.Assertions
		} else if httpConfig.PostScript != "" {
			_ = json.Unmarshal([]byte(httpConfig.PostScript), &assertions)
		}
		if len(assertions) > 0 {
			assertionResults := h.httpExec.ExecuteAssertions(result, assertions)
			assertionsJSON, marshalErr := json.Marshal(assertionResults)
			if marshalErr != nil {
				slog.Warn("failed to marshal assertion results", "error", marshalErr)
			} else {
				result.AssertionsResult = string(assertionsJSON)
			}
		}

	case "grpc_connect_test":
		var grpcConfig model.GRPCRequestConfig
		if err := json.Unmarshal([]byte(req.Config), &grpcConfig); err != nil {
			BadRequest(c, "无效的gRPC请求配置")
			return
		}

		var cert *model.Certificate
		if grpcConfig.CertificateID != nil && *grpcConfig.CertificateID > 0 {
			loadedCert, certErr := h.certSvc.GetByID(c.Request.Context(), *grpcConfig.CertificateID)
			if certErr != nil {
				slog.Error("failed to load certificate for grpc connect test", "certificate_id", *grpcConfig.CertificateID, "error", certErr)
				Fail(c, CodeApiTestExecuteFailed, "加载证书失败")
				return
			}
			cert = loadedCert
		}

		connectResult, connectErr := h.grpcExec.ConnectTest(c.Request.Context(), &grpcConfig, cert)
		if connectErr != nil {
			slog.Error("failed to execute grpc connect test", "error", connectErr)
			FailWithData(c, CodeApiTestExecuteFailed, "gRPC连接测试失败", gin.H{
				"error": connectErr.Error(),
			})
			return
		}
		result = connectResult

	case "grpc":
		var grpcConfig model.GRPCRequestConfig
		if err := json.Unmarshal([]byte(req.Config), &grpcConfig); err != nil {
			BadRequest(c, "无效的gRPC请求配置")
			return
		}

		var cert *model.Certificate
		if grpcConfig.CertificateID != nil && *grpcConfig.CertificateID > 0 {
			loadedCert, certErr := h.certSvc.GetByID(c.Request.Context(), *grpcConfig.CertificateID)
			if certErr != nil {
				slog.Error("failed to load certificate for grpc test", "certificate_id", *grpcConfig.CertificateID, "error", certErr)
				Fail(c, CodeApiTestExecuteFailed, "加载证书失败")
				return
			}
			cert = loadedCert
		}

		// Fetch proto file content if ProtoFileID is set
		protoContent, depContents := h.loadProtoContent(c, grpcConfig.ProtoFileID)

		execResult, execErr := h.grpcExec.Execute(c.Request.Context(), &grpcConfig, protoContent, depContents, cert)
		if execErr != nil {
			slog.Error("failed to execute grpc test", "error", execErr)
			FailWithData(c, CodeApiTestExecuteFailed, "gRPC请求执行失败", gin.H{
				"error": execErr.Error(),
			})
			return
		}
		result = execResult
		result.Type = "grpc"

	default:
		BadRequest(c, "不支持的测试类型，必须为 http 或 grpc")
		return
	}

	result.ExecutedBy = userID

	// Set audit context for execute action
	c.Set("audit_detail", fmt.Sprintf("类型: %s", req.Type))

	// Optionally save result
	if req.SaveResult != nil && *req.SaveResult {
		result.TestID = 0 // temporary test, no test_id
		savedResult, saveErr := h.apiTestSvc.SaveResult(c.Request.Context(), result)
		if saveErr != nil {
			slog.Warn("failed to save api test result", "error", saveErr)
		} else {
			result = savedResult
		}
	}

	Success(c, result)
}

// ExecuteSaved runs a saved test case by ID and returns the result.
func (h *ApiTestHandler) ExecuteSaved(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的测试ID")
		return
	}

	userID := extractUserID(c)
	if userID == 0 {
		Unauthorized(c, "用户未登录")
		return
	}

	// Optionally accept assertions from request body
	var reqBody struct {
		Assertions []model.AssertionConfig `json:"assertions"`
	}
	_ = c.ShouldBindJSON(&reqBody)

	test, err := h.apiTestSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		NotFound(c, "接口测试不存在")
		return
	}

	if !checkOwnership(c, h.permSvc, test.CreatedBy) {
		return
	}

	var result *model.ApiTestResult

	switch test.Type {
	case "http":
		var httpConfig model.HTTPRequestConfig
		if err := json.Unmarshal([]byte(test.Config), &httpConfig); err != nil {
			slog.Error("failed to parse http config for saved test", "id", id, "error", err)
			Fail(c, CodeApiTestExecuteFailed, "解析HTTP请求配置失败")
			return
		}

		execResult, execErr := h.httpExec.Execute(c.Request.Context(), &httpConfig)
		if execErr != nil {
			slog.Error("failed to execute saved http test", "id", id, "error", execErr)
			FailWithData(c, CodeApiTestExecuteFailed, "HTTP请求执行失败", gin.H{
				"error": execErr.Error(),
			})
			return
		}
		result = execResult
		result.Type = "http"

		// Run assertions: prefer request body assertions, fallback to PostScript
		var assertions []model.AssertionConfig
		if len(reqBody.Assertions) > 0 {
			assertions = reqBody.Assertions
		} else if httpConfig.PostScript != "" {
			_ = json.Unmarshal([]byte(httpConfig.PostScript), &assertions)
		}
		if len(assertions) > 0 {
			assertionResults := h.httpExec.ExecuteAssertions(result, assertions)
			assertionsJSON, marshalErr := json.Marshal(assertionResults)
			if marshalErr != nil {
				slog.Warn("failed to marshal assertion results", "error", marshalErr)
			} else {
				result.AssertionsResult = string(assertionsJSON)
			}
		}

	case "grpc":
		var grpcConfig model.GRPCRequestConfig
		if err := json.Unmarshal([]byte(test.Config), &grpcConfig); err != nil {
			slog.Error("failed to parse grpc config for saved test", "id", id, "error", err)
			Fail(c, CodeApiTestExecuteFailed, "解析gRPC请求配置失败")
			return
		}

		var cert *model.Certificate
		if grpcConfig.CertificateID != nil && *grpcConfig.CertificateID > 0 {
			loadedCert, certErr := h.certSvc.GetByID(c.Request.Context(), *grpcConfig.CertificateID)
			if certErr != nil {
				slog.Error("failed to load certificate for saved grpc test", "certificate_id", *grpcConfig.CertificateID, "error", certErr)
				Fail(c, CodeApiTestExecuteFailed, "加载证书失败")
				return
			}
			cert = loadedCert
		}

		// Fetch proto file content if ProtoFileID is set
		protoContent, depContents := h.loadProtoContent(c, grpcConfig.ProtoFileID)

		execResult, execErr := h.grpcExec.Execute(c.Request.Context(), &grpcConfig, protoContent, depContents, cert)
		if execErr != nil {
			slog.Error("failed to execute saved grpc test", "id", id, "error", execErr)
			FailWithData(c, CodeApiTestExecuteFailed, "gRPC请求执行失败", gin.H{
				"error": execErr.Error(),
			})
			return
		}
		result = execResult
		result.Type = "grpc"

	default:
		BadRequest(c, "不支持的测试类型")
		return
	}

	result.TestID = id
	result.ExecutedBy = userID

	// Set audit context for execute saved action
	c.Set("audit_resource_id", strconv.FormatInt(id, 10))
	c.Set("audit_resource_name", test.Name)
	c.Set("audit_detail", fmt.Sprintf("类型: %s", test.Type))

	// Save result
	savedResult, saveErr := h.apiTestSvc.SaveResult(c.Request.Context(), result)
	if saveErr != nil {
		slog.Warn("failed to save api test result", "test_id", id, "error", saveErr)
	} else {
		result = savedResult
	}

	Success(c, result)
}

// ListResults returns all execution history for the current user.
func (h *ApiTestHandler) ListResults(c *gin.Context) {
	userID := extractUserID(c)
	if userID == 0 {
		Unauthorized(c, "用户未登录")
		return
	}

	resultType := c.Query("type")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	results, total, err := h.apiTestSvc.ListResultsByUser(c.Request.Context(), userID, resultType, page, pageSize)
	if err != nil {
		slog.Error("failed to list api test results", "user_id", userID, "error", err)
		Fail(c, CodeQueryError, "获取执行历史失败")
		return
	}

	if results == nil {
		results = []*model.ApiTestResult{}
	}

	Success(c, gin.H{
		"items":     results,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GetResults returns execution history for a specific test case.
func (h *ApiTestHandler) GetResults(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的测试ID")
		return
	}

	// 校验测试用例所有权
	test, err := h.apiTestSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		slog.Error("failed to get api test for ownership check", "test_id", id, "error", err)
		NotFound(c, "测试用例不存在")
		return
	}
	if !checkOwnership(c, h.permSvc, test.CreatedBy) {
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	results, total, err := h.apiTestSvc.GetResults(c.Request.Context(), id, page, pageSize)
	if err != nil {
		slog.Error("failed to get api test results", "test_id", id, "error", err)
		Fail(c, CodeQueryError, "获取执行历史失败")
		return
	}

	if results == nil {
		results = []*model.ApiTestResult{}
	}

	Success(c, gin.H{
		"items":     results,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// DeleteResult removes a test execution result by ID.
func (h *ApiTestHandler) DeleteResult(c *gin.Context) {
	resultID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的结果ID")
		return
	}

	// 校验执行结果所有权：查询结果获取 test_id，再校验测试用例所有权
	result, err := h.apiTestSvc.GetResultByID(c.Request.Context(), resultID)
	if err != nil {
		slog.Error("failed to get api test result for ownership check", "result_id", resultID, "error", err)
		NotFound(c, "执行结果不存在")
		return
	}
	if result.TestID > 0 {
		test, testErr := h.apiTestSvc.GetByID(c.Request.Context(), result.TestID)
		if testErr != nil {
			slog.Warn("failed to get test for result ownership check", "test_id", result.TestID, "error", testErr)
		} else if !checkOwnership(c, h.permSvc, test.CreatedBy) {
			return
		}
	} else {
		// 临时执行结果，校验执行者
		userID := extractUserID(c)
		isAdmin := false
		if userID > 0 {
			adminCheck, adminErr := h.permSvc.IsSystemAdmin(c.Request.Context(), userID)
			if adminErr == nil {
				isAdmin = adminCheck
			}
		}
		if !isAdmin && result.ExecutedBy != userID {
			Forbidden(c, "无权删除此执行结果")
			return
		}
	}

	if err := h.apiTestSvc.DeleteResult(c.Request.Context(), resultID); err != nil {
		slog.Error("failed to delete api test result", "result_id", resultID, "error", err)
		Fail(c, CodeApiTestNotFound, "删除执行结果失败")
		return
	}

	slog.Info("api test result deleted", "result_id", resultID)
	c.Set("audit_resource_id", strconv.FormatInt(resultID, 10))
	Success(c, nil)
}

// GenerateCurl generates a curl command from an HTTP request config.
func (h *ApiTestHandler) GenerateCurl(c *gin.Context) {
	var req struct {
		Method  string                `json:"method"`
		URL     string                `json:"url"`
		Headers []model.KeyValue      `json:"headers,omitempty"`
		Params  []model.KeyValue      `json:"params,omitempty"`
		Body    *model.HTTPBodyConfig `json:"body,omitempty"`
		Auth    *model.HTTPAuthConfig `json:"auth,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	config := &model.HTTPRequestConfig{
		Method:  req.Method,
		URL:     req.URL,
		Headers: req.Headers,
		Params:  req.Params,
		Body:    req.Body,
		Auth:    req.Auth,
	}

	curlCmd, err := h.httpExec.GenerateCurl(config)
	if err != nil {
		slog.Error("failed to generate curl command", "error", err)
		Fail(c, CodeApiTestExecuteFailed, "生成curl命令失败")
		return
	}

	Success(c, gin.H{"curl": curlCmd})
}

// loadProtoContent loads proto file content and its dependencies by proto file ID.
// Returns empty strings if the protoFileID is nil or loading fails.
func (h *ApiTestHandler) loadProtoContent(c *gin.Context, protoFileID *int64) (string, map[string]string) {
	if protoFileID == nil || *protoFileID == 0 {
		return "", nil
	}

	pf, err := h.protoSvc.GetByID(c.Request.Context(), *protoFileID)
	if err != nil {
		slog.Warn("failed to load proto file for gRPC test", "proto_file_id", *protoFileID, "error", err)
		return "", nil
	}

	// Parse dependencies
	depContents := make(map[string]string)
	if pf.Dependencies != "" {
		var depIDs []int64
		if jsonErr := json.Unmarshal([]byte(pf.Dependencies), &depIDs); jsonErr == nil {
			for _, depID := range depIDs {
				dep, depErr := h.protoSvc.GetByID(c.Request.Context(), depID)
				if depErr != nil {
					slog.Warn("failed to load dependency proto file", "dep_id", depID, "error", depErr)
					continue
				}
				depContents[dep.Name] = dep.Content
			}
		}
	}

	return pf.Content, depContents
}
