# BDopsFlow SSO 登录完整文档

## 目录
1. [概述](#概述)
2. [架构设计](#架构设计)
3. [后端实现](#后端实现)
4. [前端实现](#前端实现)
5. [配置说明](#配置说明)
6. [接口定义](#接口定义)
7. [使用指南](#使用指南)

---

## 概述

BDopsFlow 支持两种登录方式：
- **本地登录**：使用系统内建的用户数据库进行认证
- **SSO 登录**：通过企业统一认证系统（SSO）进行身份验证

SSO 登录流程如下：
1. 用户输入 SSO 用户名和密码
2. 前端使用 SSO 公钥加密密码
3. 后端调用 SSO 服务验证凭证
4. SSO 验证成功后，后端自动创建或更新本地用户
5. 生成 JWT Token 返回给前端
6. 前端使用 Token 访问受保护的 API

---

## 架构设计

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   前端用户   │────▶│  BDopsFlow  │────▶│   SSO服务    │
│ (Login.vue) │     │   后端      │     │             │
└─────────────┘     └─────────────┘     └─────────────┘
      ▲                   │                   │
      │                   │                   │
      └───────────────────┴───────────────────┘
         JWT Token         用户信息
```

### 核心组件
| 组件 | 文件位置 | 职责 |
|------|---------|------|
| 认证处理器 | `scheduler/internal/handler/auth.go` | 处理登录请求，调用 SSO |
| JWT 中间件 | `scheduler/internal/middleware/auth.go` | 生成/验证 Token |
| 配置管理 | `scheduler/internal/config/config.go` | SSO 配置加载 |
| 登录页面 | `web/src/views/Login.vue` | 用户登录界面 |
| 认证 Store | `web/src/stores/auth.ts` | 前端认证状态管理 |

---

## 后端实现

### 1. 认证处理器 (`auth.go`)

#### 初始化 AuthHandler
```go
func NewAuthHandler(
    db database.DB,
    permSvc *service.PermissionService,
    rsaUtil *rsautil.RSAUtil,
    ssoEnabled bool,
    ssoUrl string,
    ssoRsaUtil *rsautil.RSAUtil,
    ssoTimeout int,
) *AuthHandler {
    timeout := time.Duration(ssoTimeout) * time.Second
    if timeout <= 0 {
        timeout = 10 * time.Second
    }
    return &AuthHandler{
        db:          db,
        permSvc:     permSvc,
        rsaUtil:     rsaUtil,
        ssoEnabled:  ssoEnabled,
        ssoUrl:      ssoUrl,
        ssoRsaUtil:  ssoRsaUtil,
        ssoTimeout:  timeout,
    }
}
```

#### SSO 请求与响应结构体
```go
// 发送给 SSO 服务的请求
type ssoRequest struct {
    LoginName string `json:"loginName"`
    Password  string `json:"password"`
}

// SSO 服务返回的用户信息
type ssoContent struct {
    ID          int64  `json:"id"`
    LoginName   string `json:"loginName"`
    IDCardName  string `json:"idCardName"`
    MobileNo    string `json:"mobileNo"`
    Email       string `json:"email"`
    DeptNo      string `json:"deptNo"`
    WorkID      string `json:"workId"`
    Gender      string `json:"gender"`
    OfficePhone string `json:"officePhone"`
}

// SSO 服务响应结构
type ssoResponse struct {
    Code    string      `json:"code"`     // "3000" 表示成功
    Message string      `json:"message"`
    Content *ssoContent `json:"content"`
}
```

#### SSO 登录处理函数
```go
func (h *AuthHandler) SSOLogin(c *gin.Context) {
    // 1. 检查 SSO 是否启用
    if !h.ssoEnabled {
        BadRequest(c, "SSO登录未启用，请使用本地登录")
        return
    }

    // 2. 解析请求参数
    var req struct {
        Username string `json:"username" binding:"required"`
        Password string `json:"password" binding:"required"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        BadRequest(c, "用户名和密码不能为空")
        return
    }

    // 3. 记录审计日志
    c.Set("audit_resource_name", req.Username)
    c.Set("username", req.Username)

    slog.Debug("SSOLogin: request entry", "module", "handler_auth", "username", req.Username)

    // 4. 调用 SSO 服务
    ssoReq := ssoRequest{
        LoginName: req.Username,
        Password:  req.Password,
    }
    ssoBody, err := json.Marshal(ssoReq)
    if err != nil {
        slog.Error("SSOLogin: failed to marshal SSO request", "error", err)
        InternalServerError(c, "SSO登录失败，请稍后再试")
        return
    }

    httpReq, err := http.NewRequestWithContext(
        c.Request.Context(), "POST", h.ssoUrl, bytes.NewReader(ssoBody))
    if err != nil {
        slog.Error("SSOLogin: failed to create SSO request", "error", err)
        InternalServerError(c, "SSO登录失败，请稍后再试")
        return
    }
    httpReq.Header.Set("Content-Type", "application/json")

    httpClient := &http.Client{Timeout: h.ssoTimeout}
    resp, err := httpClient.Do(httpReq)
    if err != nil {
        slog.Error("SSOLogin: failed to call SSO service", "error", err, "url", h.ssoUrl)
        Fail(c, CodeInternalError, "SSO登录失败，请稍后再试")
        return
    }
    defer resp.Body.Close()

    // 5. 解析 SSO 响应
    var ssoResp ssoResponse
    if err := json.NewDecoder(resp.Body).Decode(&ssoResp); err != nil {
        slog.Error("SSOLogin: failed to decode SSO response", "error", err)
        InternalServerError(c, "SSO登录失败，请稍后再试")
        return
    }

    // 6. 验证 SSO 响应状态码
    if ssoResp.Code != "3000" || ssoResp.Content == nil {
        errMsg := ssoResp.Message
        if errMsg == "" {
            errMsg = "SSO登录失败"
        }
        slog.Warn("SSOLogin: SSO authentication failed", "code", ssoResp.Code, "message", errMsg)
        metrics.AuthAttempts.WithLabelValues("sso", "failed").Inc()
        Fail(c, CodeInvalidCredentials, errMsg)
        return
    }

    ssoUser := ssoResp.Content
    loginName := ssoUser.LoginName
    if loginName == "" {
        loginName = req.Username
    }

    // 7. 查找或创建本地用户
    query := "SELECT id, username, real_name, phone, email, is_active FROM bdopsflow_users WHERE username = ?"
    stmt := rqlite.ParameterizedStatement{
        Query:     query,
        Arguments: []interface{}{loginName},
    }
    qr, err := h.db.QueryOneParameterized(stmt)
    if err != nil {
        slog.Error("SSOLogin: failed to query user", "error", err)
        InternalServerError(c, "服务器错误，请稍后重试")
        return
    }
    if qr.Err != nil {
        slog.Error("SSOLogin: query error", "error", qr.Err)
        InternalServerError(c, "服务器错误，请稍后重试")
        return
    }

    var userID int64
    var username, realName, phone, email string
    var isActive bool

    if qr.Next() {
        // 用户已存在，更新登录时间
        row, sliceErr := qr.Slice()
        if sliceErr != nil {
            slog.Error("SSOLogin: failed to slice user", "error", sliceErr)
            InternalServerError(c, "服务器错误，请稍后重试")
            return
        }
        userID = service.RowInt64(row[0])
        username = service.RowString(row[1])
        realName = service.RowString(row[2])
        phone = service.RowString(row[3])
        email = service.RowString(row[4])
        isActive = service.RowBool(row[5])

        go func() {
            updateQuery := "UPDATE bdopsflow_users SET last_login_at = ? WHERE id = ?"
            updateStmt := rqlite.ParameterizedStatement{
                Query:     updateQuery,
                Arguments: []interface{}{time.Now(), userID},
            }
            h.db.WriteOneParameterized(updateStmt)
        }()

        slog.Info("SSOLogin: existing user login success", "module", "handler_auth", "user_id", userID, "username", username)
    } else {
        // 用户不存在，自动创建
        realName = ssoUser.IDCardName
        phone = ssoUser.MobileNo
        email = ssoUser.Email
        isActive = true

        insertQuery := "INSERT INTO bdopsflow_users (username, real_name, phone, password, email, is_active, created_at) VALUES (?, ?, ?, '', ?, 1, ?)"
        insertStmt := rqlite.ParameterizedStatement{
            Query:     insertQuery,
            Arguments: []interface{}{loginName, realName, phone, email, time.Now()},
        }
        result, err := h.db.WriteOneParameterized(insertStmt)
        if err != nil {
            slog.Error("SSOLogin: failed to create user", "error", err)
            InternalServerError(c, "服务器错误，请稍后重试")
            return
        }
        if result.Err != nil {
            slog.Error("SSOLogin: create user error", "error", result.Err)
            InternalServerError(c, "服务器错误，请稍后重试")
            return
        }
        userID = result.LastInsertID
        username = loginName

        // 分配默认角色（user）
        roleQuery := "SELECT id FROM bdopsflow_roles WHERE code = 'user' LIMIT 1"
        roleStmt := rqlite.ParameterizedStatement{
            Query: roleQuery,
        }
        roleQr, roleErr := h.db.QueryOneParameterized(roleStmt)
        if roleErr == nil && roleQr.Err == nil && roleQr.Next() {
            roleRow, _ := roleQr.Slice()
            if len(roleRow) > 0 {
                roleID := service.RowInt64(roleRow[0])
                if roleID > 0 {
                    assignStmt := rqlite.ParameterizedStatement{
                        Query:     "INSERT INTO bdopsflow_user_roles (user_id, role_id, created_at) VALUES (?, ?, ?)",
                        Arguments: []interface{}{userID, roleID, time.Now()},
                    }
                    h.db.WriteOneParameterized(assignStmt)
                }
            }
        }

        slog.Info("SSOLogin: auto created user from SSO", "username", loginName, "user_id", userID)
    }

    // 8. 获取用户权限信息
    domains, domainErr := h.permSvc.GetUserDomainInfos(c.Request.Context(), userID)
    if domainErr != nil {
        slog.Error("SSOLogin: get user domain infos failed", "error", domainErr, "user_id", userID)
    }
    if domains == nil {
        domains = []*model.UserDomainInfo{}
    }
    var currentDomainID int64
    defaultDomainID, defaultErr := h.permSvc.GetUserDefaultDomain(c.Request.Context(), userID)
    if defaultErr != nil {
        slog.Error("SSOLogin: get user default domain failed", "error", defaultErr, "user_id", userID)
    }
    if defaultDomainID > 0 {
        currentDomainID = defaultDomainID
    } else if len(domains) > 0 {
        currentDomainID = domains[0].DomainID
    }

    // 9. 生成 JWT Token
    tokenString, err := middleware.GenerateToken(userID, username, realName, currentDomainID)
    if err != nil {
        slog.Error("SSOLogin: failed to generate token", "error", err)
        InternalServerError(c, "服务器错误，请稍后重试")
        return
    }

    refreshToken, refreshErr := middleware.GenerateRefreshToken(userID, username, realName, currentDomainID)
    if refreshErr != nil {
        slog.Error("SSOLogin: generate refresh token failed", "error", refreshErr, "user_id", userID)
        InternalServerError(c, "服务器错误，请稍后重试")
        return
    }

    permissions, permErr := h.permSvc.GetUserPermissions(c.Request.Context(), userID)
    if permErr != nil {
        slog.Error("SSOLogin: get user permissions failed", "error", permErr, "user_id", userID)
    }
    if permissions == nil {
        permissions = []*model.Permission{}
    }

    roleCodes, roleErr := h.permSvc.GetUserRoleCodes(c.Request.Context(), userID)
    if roleErr != nil {
        slog.Error("SSOLogin: get user role codes failed", "error", roleErr, "user_id", userID)
    }
    if roleCodes == nil {
        roleCodes = []string{}
    }

    metrics.AuthAttempts.WithLabelValues("sso", "success").Inc()

    // 10. 返回登录成功响应
    Success(c, gin.H{
        "token":               tokenString,
        "refresh_token":       refreshToken,
        "user": map[string]interface{}{
            "id":        userID,
            "username":  username,
            "real_name": realName,
            "phone":     phone,
            "email":     email,
            "is_active": isActive,
        },
        "permissions":         permissions,
        "domains":             domains,
        "current_domain_id":   currentDomainID,
        "role_codes":          roleCodes,
    })
}
```

### 2. JWT 中间件 (`auth.go`)

#### Claims 结构
```go
type Claims struct {
    UserID          int64  `json:"user_id"`
    Username        string `json:"username"`
    RealName        string `json:"real_name"`
    CurrentDomainID int64  `json:"current_domain_id"`
    jwt.RegisteredClaims
}
```

#### Token 生成函数
```go
func GenerateToken(userID int64, username, realName string, currentDomainID int64) (string, error) {
    claims := &Claims{
        UserID:          userID,
        Username:        username,
        RealName:        realName,
        CurrentDomainID: currentDomainID,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(jwtConfig.ExpiryHours) * time.Hour)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
            Issuer:    "bdopsflow",
        },
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString(jwtConfig.Secret)
}

func GenerateRefreshToken(userID int64, username, realName string, currentDomainID int64) (string, error) {
    claims := &Claims{
        UserID:          userID,
        Username:        username,
        RealName:        realName,
        CurrentDomainID: currentDomainID,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(jwtConfig.RefreshExpiryHours) * time.Hour)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
            Issuer:    "bdopsflow-refresh",
        },
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString(jwtConfig.RefreshSecret)
}
```

### 3. 路由配置 (`routes.go`)
```go
// SSO 登录路由
router.POST("/api/auth/sso-login", middleware.AuditMiddleware(app.auditLogService), authHandler.SSOLogin)
// 本地登录路由
router.POST("/api/auth/login", middleware.AuditMiddleware(app.auditLogService), authHandler.Login)
// 获取公钥
router.GET("/api/auth/public-key", authHandler.GetPublicKey)
// 刷新 Token
router.POST("/api/auth/refresh", authHandler.RefreshToken)
```

---

## 前端实现

### 1. 登录页面 (`Login.vue`)

#### 登录模式切换
```vue
<!-- 登录模式切换 -->
<div v-if="authStore.ssoEnabled" class="login-mode-switch">
  <span :class="{ active: isSso }" @click="isSso = true">SSO 登录</span>
  <span class="divider">|</span>
  <span :class="{ active: !isSso }" @click="isSso = false">本地登录</span>
</div>
```

#### 登录处理函数
```typescript
const handleLogin = async () => {
  errorMessage.value = ''
  await loginFormRef.value?.validate(async (valid: boolean) => {
    if (valid) {
      isLoading.value = true
      try {
        if (isSso.value && authStore.ssoEnabled) {
          await authStore.ssoLogin(loginForm.username, loginForm.password)
        } else {
          await authStore.login(loginForm.username, loginForm.password)
        }
        ElMessage.success('登录成功，欢迎回来！')
        router.push('/')
      } catch (error: any) {
        // 详细错误信息处理 - 确保全部是中文
        let errorMsg = '登录失败'
        
        // 从响应中获取错误信息
        if (error?.response?.data?.error) {
          errorMsg = error.response.data.error
        } else if (error?.response?.data?.message) {
          errorMsg = error.response.data.message
        } else if (error?.message) {
          errorMsg = error.message
        }
        
        // 使用统一的翻译函数转换为中文
        errorMsg = translateErrorMessage(errorMsg)
        
        // 如果翻译后还是英文或未知错误，设置为通用的中文提示
        if (!/[\u4e00-\u9fa5]/.test(errorMsg)) {
          if (error?.response?.status === 401) {
            errorMsg = '用户名或密码错误'
          } else if (error?.response?.status === 400) {
            errorMsg = '请求参数错误'
          } else if (error?.response?.status >= 500) {
            errorMsg = '服务器错误，请稍后重试'
          } else {
            errorMsg = '登录失败，请稍后重试'
          }
        }
        
        errorMessage.value = errorMsg
        ElMessage.error(errorMsg)
      } finally {
        isLoading.value = false
      }
    }
  })
}
```

### 2. 认证 Store (`auth.ts`)

#### SSO 登录方法
```typescript
const ssoLogin = async (username: string, password: string) => {
  await fetchPublicKey()
  if (!getSSOPublicKey()) {
    throw new Error("SSO公钥未加载，无法登录")
  }
  const encryptedPassword = encryptPasswordSSO(password)
  const response = await authAPI.ssoLogin({ username, password: encryptedPassword })
  const { 
    token: newToken, 
    refresh_token: newRefreshToken, 
    user: newUser, 
    permissions: newPermissions, 
    domains: newDomains, 
    current_domain_id, 
    role_codes 
  } = response.data
  setToken(newToken)
  if (newRefreshToken) {
    setRefreshToken(newRefreshToken)
  }
  setUser(newUser)
  setPermissions(newPermissions)
  setDomains(newDomains)
  setCurrentDomainId(current_domain_id)
  setRoleCodes(role_codes || [])
  return newUser
}
```

---

## 配置说明

### 1. 配置文件 (`config.yaml`)

```yaml
# SSO 配置
sso:
  enabled: true              # 是否启用 SSO 登录
  url: "https://sso.example.com/api/login"  # SSO 服务地址
  public_key: |              # SSO RSA 公钥（PEM 格式）
    -----BEGIN PUBLIC KEY-----
    ...
    -----END PUBLIC KEY-----
  timeout: 10                # SSO 服务调用超时时间（秒）

# JWT 配置
jwt:
  secret: "your-secret-key-change-in-production"
  expiry_hours: 2            # Token 过期时间（小时）
  refresh_expiry_hours: 168  # Refresh Token 过期时间（小时，默认 7 天）

# RSA 配置（用于密码加密）
rsa:
  public_key: |
    -----BEGIN PUBLIC KEY-----
    ...
    -----END PUBLIC KEY-----
  private_key: |
    -----BEGIN PRIVATE KEY-----
    ...
    -----END PRIVATE KEY-----
```

### 2. 配置结构体 (`config.go`)

```go
type Config struct {
    // ... 其他配置
    SSOEnabled   bool   // 是否启用 SSO
    SSOUrl       string // SSO 服务地址
    SSOPublicKey string // SSO RSA 公钥
    SSOTimeout   int    // SSO 超时时间（秒）
}
```

---

## 接口定义

### 1. SSO 登录接口

#### 请求
- **URL**: `POST /api/auth/sso-login`
- **Content-Type**: `application/json`

```json
{
  "username": "zhangsan",
  "password": "encrypted_password"
}
```

#### 响应
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "user": {
      "id": 1,
      "username": "zhangsan",
      "real_name": "张三",
      "phone": "13800138000",
      "email": "zhangsan@example.com",
      "is_active": true
    },
    "permissions": [
      {
        "resource": "task",
        "action": "read",
        "domain_id": 1
      }
    ],
    "domains": [
      {
        "domain_id": 1,
        "domain_name": "默认领域",
        "is_default": true
      }
    ],
    "current_domain_id": 1,
    "role_codes": ["user"]
  }
}
```

### 2. 获取公钥接口

#### 请求
- **URL**: `GET /api/auth/public-key`

#### 响应
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "public_key": "-----BEGIN PUBLIC KEY-----\n...",
    "sso_enabled": true,
    "sso_public_key": "-----BEGIN PUBLIC KEY-----\n..."
  }
}
```

### 3. 刷新 Token 接口

#### 请求
- **URL**: `POST /api/auth/refresh`
- **Content-Type**: `application/json`

```json
{
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

#### 响应
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  }
}
```

### 4. JWT 认证说明

后续请求需要在 Header 中携带 Token：
```
Authorization: Bearer {token}
```

或者通过 URL 参数传递（部分接口）：
```
?token={token}
```

---

## 使用指南

### 1. 配置 SSO

1. 在配置文件中启用 SSO
2. 配置 SSO 服务地址
3. 配置 SSO RSA 公钥用于加密密码
4. 配置本地 RSA 密钥对用于本地登录

### 2. 生成密钥对

可以使用项目提供的命令生成密钥：
```bash
# 生成 RSA 密钥对
./scheduler keygen
```

### 3. 用户自动创建

当用户首次通过 SSO 登录时：
1. 系统会自动创建本地用户账号
2. 分配默认角色 `user`
3. 用户可以使用相同的用户名继续登录

### 4. 管理员设置

对于自动创建的用户，管理员可以：
1. 修改用户信息
2. 分配额外角色
3. 分配领域权限
4. 启用/禁用用户账号

---

## 安全注意事项

1. **密码加密传输**：所有密码在传输前都使用 RSA 公钥加密
2. **Token 过期**：JWT Token 默认 2 小时过期，Refresh Token 默认 7 天过期
3. **HTTPS 建议**：生产环境建议使用 HTTPS 加密所有传输
4. **SSO 公钥保护**：SSO RSA 公钥应该安全存储和更新
5. **审计日志**：所有登录尝试（成功/失败）都会记录到审计日志

---

## 故障排查

### SSO 登录失败
1. 检查 SSO 服务地址是否正确
2. 检查 SSO 服务是否正常运行
3. 查看后端日志中的错误信息
4. 验证 SSO 响应格式是否符合预期

### Token 验证失败
1. 检查 JWT 密钥是否一致
2. 检查 Token 是否过期
3. 检查系统时间是否同步

### 用户权限问题
1. 确认用户角色分配正确
2. 确认权限配置正确
3. 检查领域权限设置

---

## 附录

### 完整的 SSO 服务响应示例
```json
{
  "code": "3000",
  "message": "认证成功",
  "content": {
    "id": 1001,
    "loginName": "zhangsan",
    "idCardName": "张三",
    "mobileNo": "13800138000",
    "email": "zhangsan@example.com",
    "deptNo": "DEV001",
    "workId": "E1001",
    "gender": "male",
    "officePhone": "010-88888888"
  }
}
```

### 状态码说明
| SSO Code | 说明 |
|----------|------|
| 3000 | 认证成功 |
| 其他 | 认证失败，具体错误看 message |
