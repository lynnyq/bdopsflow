# BDopsFlow 权限管理系统分阶段实施计划

## 文档信息

- **项目名称**：BDopsFlow 用户权限与多租户管理系统
- **关联设计文档**：[DESIGN.md](./DESIGN.md)
- **版本**：v1.0
- **创建日期**：2026-05-14
- **作者**：BDopsFlow Team
- **状态**：待实施

---

## 总体实施策略

本实施计划分为 **4 个阶段**，预计总工期为 **8-10 周**。

每个阶段包含：
- **具体任务清单**
- **实现产物**
- **单元测试用例**
- **功能测试用例**
- **验收标准**

---

## 阶段一：数据库与后端基础设施

**预计工期**：2 周

**目标**：完成数据库表结构设计和基础权限验证框架

### 1.1 数据库迁移

#### 任务清单

- [ ] 1.1.1 创建 roles 表
- [ ] 1.1.2 创建 permissions 表
- [ ] 1.1.3 创建 role_permissions 表
- [ ] 1.1.4 创建 user_roles 表
- [ ] 1.1.5 创建 domain_executors 表
- [ ] 1.1.6 修改 users 表（添加字段）
- [ ] 1.1.7 修改 executors 表（添加字段）
- [ ] 1.1.8 编写数据迁移脚本
- [ ] 1.1.9 创建初始化数据脚本

#### 实现产物

1. **数据库文件**：
   - `deploy/schema_permissions.sql` - 新增权限相关表结构
   - `deploy/migration_v2.sql` - 数据迁移脚本

2. **Model 文件**：
   - `scheduler/internal/model/role.go` - 角色模型
   - `scheduler/internal/model/permission.go` - 权限模型
   - `scheduler/internal/model/user_role.go` - 用户角色映射模型
   - `scheduler/internal/model/domain_executor.go` - 执行器领域映射模型

3. **初始化数据**：
   - 预设 3 个系统角色
   - 预设所有权限定义
   - 角色权限关联

#### 单元测试用例

```go
// 角色模型测试
func TestRoleModel(t *testing.T) {
    // Test 1: 创建角色对象
    role := &model.Role{
        Name:        "测试角色",
        Code:        "test_role",
        Description: "测试用角色",
        IsSystem:    false,
    }
    
    // Test 2: 验证角色字段
    assert.Equal(t, "测试角色", role.Name)
    assert.Equal(t, "test_role", role.Code)
    assert.False(t, role.IsSystem)
}

// 权限模型测试
func TestPermissionModel(t *testing.T) {
    // Test 1: 创建权限对象
    perm := &model.Permission{
        Resource: "task",
        Action:   "create",
    }
    
    // Test 2: 验证权限格式
    assert.Equal(t, "task", perm.Resource)
    assert.Equal(t, "create", perm.Action)
    assert.Equal(t, "task:create", perm.GetCode())
}

// 用户角色映射测试
func TestUserRoleModel(t *testing.T) {
    // Test 1: 创建用户角色映射
    ur := &model.UserRole{
        UserID:  1,
        RoleID:  1,
        DomainID: nil,
    }
    
    // Test 2: 验证映射关系
    assert.Equal(t, int64(1), ur.UserID)
    assert.Equal(t, int64(1), ur.RoleID)
    assert.Nil(t, ur.DomainID)
}
```

#### 功能测试用例

```bash
# Test 1: 验证角色表创建
curl -XPOST 'http://localhost:4001/db/query?pretty' \
  -d '["SELECT COUNT(*) as count FROM roles"]'
# 预期：返回 {"results":[{"columns":["count"],"types":["integer"],"values":[[3]]}]}

# Test 2: 验证权限表创建
curl -XPOST 'http://localhost:4001/db/query?pretty' \
  -d '["SELECT COUNT(*) as count FROM permissions"]'
# 预期：返回包含所有权限定义的记录

# Test 3: 验证角色权限关联
curl -XPOST 'http://localhost:4001/db/query?pretty' \
  -d '["SELECT COUNT(*) as count FROM role_permissions"]'
# 预期：返回系统角色关联的权限记录

# Test 4: 验证 users 表字段
curl -XPOST 'http://localhost:4001/db/query?pretty' \
  -d '["PRAGMA table_info(users)"]'
# 预期：包含 is_active, last_login_at, created_by 字段
```

#### 验收标准

- ✅ 所有权限相关表创建成功
- ✅ 预设角色和权限数据插入成功
- ✅ users 表新增字段正确
- ✅ executors 表新增字段正确
- ✅ 数据迁移脚本可正常执行
- ✅ 所有单元测试通过
- ✅ 所有功能测试通过

---

### 1.2 权限验证框架

#### 任务清单

- [ ] 1.2.1 创建权限检查 Service
- [ ] 1.2.2 实现角色权限验证逻辑
- [ ] 1.2.3 实现领域访问验证逻辑
- [ ] 1.2.4 创建权限中间件
- [ ] 1.2.5 实现 JWT Token 权限信息扩展
- [ ] 1.2.6 添加权限缓存机制

#### 实现产物

1. **Service 文件**：
   - `scheduler/internal/service/permission_service.go` - 权限检查服务

2. **Middleware 文件**：
   - `scheduler/internal/middleware/permission.go` - 权限中间件

3. **工具文件**：
   - `scheduler/pkg/auth/permission.go` - 权限工具函数

#### 核心代码示例

```go
// permission_service.go
type PermissionService struct {
    db *gorm.DB
    cache *redis.Client
}

func (s *PermissionService) HasPermission(ctx context.Context, userID int64, resource, action string, domainID int64) (bool, error) {
    // 1. 检查缓存
    cacheKey := fmt.Sprintf("perm:%d:%s:%s:%d", userID, resource, action, domainID)
    if cached, err := s.cache.Get(ctx, cacheKey).Result(); err == nil {
        return cached == "1", nil
    }
    
    // 2. 获取用户角色
    userRoles, err := s.GetUserRoles(ctx, userID)
    if err != nil {
        return false, err
    }
    
    // 3. 检查是否有系统管理员权限
    for _, role := range userRoles {
        if role.Code == "system_admin" {
            s.cache.Set(ctx, cacheKey, "1", 5*time.Minute)
            return true, nil
        }
    }
    
    // 4. 检查权限
    hasPermission := false
    for _, role := range userRoles {
        if s.checkRolePermission(ctx, role.ID, resource, action) {
            hasPermission = true
            break
        }
    }
    
    // 5. 缓存结果
    if hasPermission {
        s.cache.Set(ctx, cacheKey, "1", 5*time.Minute)
    }
    
    return hasPermission, nil
}

func (s *PermissionService) CanAccessDomain(ctx context.Context, userID, domainID int64) (bool, error) {
    // 1. 全局管理员可访问所有领域
    if s.IsSystemAdmin(ctx, userID) {
        return true, nil
    }
    
    // 2. 检查用户是否有该领域访问权限
    userDomains, err := s.GetUserDomains(ctx, userID)
    if err != nil {
        return false, err
    }
    
    for _, domain := range userDomains {
        if domain.ID == domainID {
            return true, nil
        }
    }
    
    return false, nil
}
```

```go
// permission.go (middleware)
func RequirePermission(resource, action string) gin.HandlerFunc {
    return func(c *gin.Context) {
        userID := c.GetInt64("user_id")
        domainID := c.GetInt64("domain_id")
        
        hasPermission, err := permissionService.HasPermission(
            c.Request.Context(),
            userID,
            resource,
            action,
            domainID,
        )
        
        if err != nil {
            c.JSON(500, gin.H{"error": "权限检查失败"})
            c.Abort()
            return
        }
        
        if !hasPermission {
            c.JSON(403, gin.H{"error": "权限不足"})
            c.Abort()
            return
        }
        
        c.Next()
    }
}
```

#### 单元测试用例

```go
// 权限服务测试
func TestPermissionService_HasPermission(t *testing.T) {
    // Setup
    service := setupPermissionService()
    ctx := context.Background()
    
    // Test 1: 系统管理员应拥有所有权限
    t.Run("系统管理员权限", func(t *testing.T) {
        hasPerm, err := service.HasPermission(ctx, 1, "task", "create", 1)
        assert.NoError(t, err)
        assert.True(t, hasPerm)
    })
    
    // Test 2: 普通用户应有查看权限
    t.Run("普通用户查看权限", func(t *testing.T) {
        hasPerm, err := service.HasPermission(ctx, 2, "task", "read", 1)
        assert.NoError(t, err)
        assert.True(t, hasPerm)
    })
    
    // Test 3: 普通用户无创建权限
    t.Run("普通用户无创建权限", func(t *testing.T) {
        hasPerm, err := service.HasPermission(ctx, 2, "task", "create", 1)
        assert.NoError(t, err)
        assert.False(t, hasPerm)
    })
    
    // Test 4: 跨领域访问应被拒绝
    t.Run("跨领域访问拒绝", func(t *testing.T) {
        hasPerm, err := service.HasPermission(ctx, 2, "task", "read", 2)
        assert.NoError(t, err)
        assert.False(t, hasPerm)
    })
}

// 权限缓存测试
func TestPermissionService_Cache(t *testing.T) {
    // Test 1: 首次查询应缓存
    t.Run("首次查询缓存", func(t *testing.T) {
        service := setupPermissionService()
        ctx := context.Background()
        
        // 第一次查询
        _, err := service.HasPermission(ctx, 1, "task", "read", 1)
        assert.NoError(t, err)
        
        // 检查缓存
        cacheKey := "perm:1:task:read:1"
        cached, _ := service.cache.Get(ctx, cacheKey).Result()
        assert.Equal(t, "1", cached)
    })
    
    // Test 2: 权限变更后应刷新缓存
    t.Run("权限变更刷新缓存", func(t *testing.T) {
        service := setupPermissionService()
        ctx := context.Background()
        
        // 变更权限
        err := service.UpdateRolePermissions(ctx, 1, []int64{1, 2, 3})
        assert.NoError(t, err)
        
        // 检查缓存是否被清除
        cacheKey := "perm:1:task:read:1"
        _, err = service.cache.Get(ctx, cacheKey).Result()
        assert.Error(t, err) // 缓存应该被清除
    })
}

// 领域访问测试
func TestPermissionService_CanAccessDomain(t *testing.T) {
    // Test 1: 全局管理员可访问所有领域
    t.Run("全局管理员访问所有领域", func(t *testing.T) {
        service := setupPermissionService()
        ctx := context.Background()
        
        canAccess, err := service.CanAccessDomain(ctx, 1, 1)
        assert.NoError(t, err)
        assert.True(t, canAccess)
        
        canAccess, err = service.CanAccessDomain(ctx, 1, 2)
        assert.NoError(t, err)
        assert.True(t, canAccess)
    })
    
    // Test 2: 领域用户只能访问所属领域
    t.Run("领域用户访问限制", func(t *testing.T) {
        service := setupPermissionService()
        ctx := context.Background()
        
        // 可访问所属领域
        canAccess, err := service.CanAccessDomain(ctx, 2, 1)
        assert.NoError(t, err)
        assert.True(t, canAccess)
        
        // 不可访问其他领域
        canAccess, err = service.CanAccessDomain(ctx, 2, 2)
        assert.NoError(t, err)
        assert.False(t, canAccess)
    })
}
```

#### 功能测试用例

```bash
# Test 1: 系统管理员访问所有资源
curl -X GET http://localhost:8080/api/tasks \
  -H "Authorization: Bearer <system_admin_token>"
# 预期：返回所有任务（包含所有领域）

# Test 2: 领域用户访问本领域资源
curl -X GET http://localhost:8080/api/tasks \
  -H "Authorization: Bearer <domain_user_token>"
# 预期：只返回本领域任务

# Test 3: 普通用户无创建权限
curl -X POST http://localhost:8080/api/tasks \
  -H "Authorization: Bearer <user_token>" \
  -H "Content-Type: application/json" \
  -d '{"name":"test"}'
# 预期：返回 403 Forbidden

# Test 4: 跨领域访问应被拒绝
curl -X GET http://localhost:8080/api/tasks?domain_id=2 \
  -H "Authorization: Bearer <domain1_user_token>"
# 预期：返回 403 Forbidden

# Test 5: 权限变更后立即生效
# 5.1 使用管理员给用户添加创建权限
curl -X PUT http://localhost:8080/api/admin/users/2/permissions \
  -H "Authorization: Bearer <admin_token>" \
  -d '{"permissions":["task:create"]}'
# 预期：成功

# 5.2 验证用户立即获得权限
curl -X POST http://localhost:8080/api/tasks \
  -H "Authorization: Bearer <user_token>" \
  -H "Content-Type: application/json" \
  -d '{"name":"test"}'
# 预期：成功创建（权限变更后缓存应被刷新）
```

#### 验收标准

- ✅ 权限检查 Service 正确实现
- ✅ 中间件正确拦截无权限请求
- ✅ 系统管理员拥有所有权限
- ✅ 领域用户只能访问本领域资源
- ✅ 权限变更立即生效（缓存正确刷新）
- ✅ 所有单元测试通过
- ✅ 所有功能测试通过

---

## 阶段二：后端 API 实现

**预计工期**：3 周

**目标**：完成所有权限管理相关的后端 API 实现

### 2.1 用户管理 API

#### 任务清单

- [ ] 2.1.1 实现用户 CRUD API
- [ ] 2.1.2 实现用户角色分配 API
- [ ] 2.1.3 实现用户领域分配 API
- [ ] 2.1.4 实现用户列表查询（支持分页和过滤）
- [ ] 2.1.5 实现用户详情查询

#### 实现产物

- `scheduler/internal/handler/user_admin.go` - 用户管理 Handler
- `scheduler/internal/service/user_admin_service.go` - 用户管理 Service

#### API 接口

```go
// 用户管理路由
users := adminGroup.Group("/users")
{
    users.GET("", RequirePermission("user", "read"), ListUsers)
    users.POST("", RequirePermission("user", "create"), CreateUser)
    users.GET("/:id", RequirePermission("user", "read"), GetUser)
    users.PUT("/:id", RequirePermission("user", "update"), UpdateUser)
    users.DELETE("/:id", RequirePermission("user", "delete"), DeleteUser)
    users.POST("/:id/roles", RequirePermission("user", "update"), AssignRoles)
    users.DELETE("/:id/roles/:roleId", RequirePermission("user", "update"), RemoveRole)
    users.POST("/:id/domains", RequirePermission("user", "update"), AssignDomains)
    users.DELETE("/:id/domains/:domainId", RequirePermission("user", "update"), RemoveDomain)
}
```

#### 单元测试用例

```go
func TestUserAdminService_CreateUser(t *testing.T) {
    // Test 1: 创建用户成功
    t.Run("创建用户成功", func(t *testing.T) {
        service := setupUserAdminService()
        ctx := context.Background()
        
        user := &model.User{
            Username: "newuser",
            Password: "password123",
            Email:    "newuser@example.com",
        }
        
        createdUser, err := service.CreateUser(ctx, user)
        assert.NoError(t, err)
        assert.NotNil(t, createdUser)
        assert.Equal(t, "newuser", createdUser.Username)
        assert.True(t, createdUser.IsActive)
    })
    
    // Test 2: 创建重复用户名应失败
    t.Run("重复用户名失败", func(t *testing.T) {
        service := setupUserAdminService()
        ctx := context.Background()
        
        user := &model.User{
            Username: "existinguser",
            Password: "password123",
        }
        
        _, err := service.CreateUser(ctx, user)
        assert.Error(t, err)
        assert.Equal(t, ErrUsernameExists, err)
    })
    
    // Test 3: 分配角色成功
    t.Run("分配角色成功", func(t *testing.T) {
        service := setupUserAdminService()
        ctx := context.Background()
        
        err := service.AssignRoles(ctx, 1, []int64{1, 2}, []int64{1})
        assert.NoError(t, err)
        
        // 验证角色分配
        roles, err := service.GetUserRoles(ctx, 1)
        assert.NoError(t, err)
        assert.Len(t, roles, 2)
    })
}

func TestUserAdminService_ListUsers(t *testing.T) {
    // Test 1: 列出所有用户（系统管理员）
    t.Run("系统管理员列出所有用户", func(t *testing.T) {
        service := setupUserAdminService()
        ctx := context.Background()
        ctx = context.WithValue(ctx, "user_id", int64(1))
        
        users, err := service.ListUsers(ctx, nil)
        assert.NoError(t, err)
        assert.True(t, len(users) > 0)
    })
    
    // Test 2: 分页查询
    t.Run("分页查询", func(t *testing.T) {
        service := setupUserAdminService()
        ctx := context.Background()
        
        users, err := service.ListUsers(ctx, &Pagination{Page: 1, PageSize: 10})
        assert.NoError(t, err)
        assert.LessOrEqual(t, len(users), 10)
    })
    
    // Test 3: 按角色过滤
    t.Run("按角色过滤", func(t *testing.T) {
        service := setupUserAdminService()
        ctx := context.Background()
        
        users, err := service.ListUsers(ctx, &UserFilter{RoleID: 1})
        assert.NoError(t, err)
        
        for _, user := range users {
            hasRole := false
            for _, role := range user.Roles {
                if role.ID == 1 {
                    hasRole = true
                    break
                }
            }
            assert.True(t, hasRole)
        }
    })
}
```

#### 功能测试用例

```bash
# Test 1: 创建用户
curl -X POST http://localhost:8080/api/admin/users \
  -H "Authorization: Bearer <admin_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "newuser",
    "password": "password123",
    "email": "newuser@example.com"
  }'
# 预期：返回新创建的用户信息

# Test 2: 列出所有用户
curl -X GET http://localhost:8080/api/admin/users \
  -H "Authorization: Bearer <admin_token>"
# 预期：返回用户列表

# Test 3: 分配角色
curl -X POST http://localhost:8080/api/admin/users/2/roles \
  -H "Authorization: Bearer <admin_token>" \
  -H "Content-Type: application/json" \
  -d '{"role_ids":[2],"domain_ids":[1]}'
# 预期：成功分配角色

# Test 4: 分配领域
curl -X POST http://localhost:8080/api/admin/users/2/domains \
  -H "Authorization: Bearer <admin_token>" \
  -H "Content-Type: application/json" \
  -d '{"domain_ids":[1,2]}'
# 预期：成功分配领域

# Test 5: 普通用户无权访问管理接口
curl -X GET http://localhost:8080/api/admin/users \
  -H "Authorization: Bearer <user_token>"
# 预期：返回 403 Forbidden
```

#### 验收标准

- ✅ 创建用户 API 正常工作
- ✅ 用户列表查询支持分页
- ✅ 角色分配 API 正常工作
- ✅ 领域分配 API 正常工作
- ✅ 普通用户无法访问管理接口
- ✅ 所有单元测试通过
- ✅ 所有功能测试通过

---

### 2.2 角色管理 API

#### 任务清单

- [ ] 2.2.1 实现角色 CRUD API
- [ ] 2.2.2 实现角色权限配置 API
- [ ] 2.2.3 实现角色权限查询 API
- [ ] 2.2.4 实现系统角色保护（不可删除）

#### 实现产物

- `scheduler/internal/handler/role_admin.go` - 角色管理 Handler
- `scheduler/internal/service/role_admin_service.go` - 角色管理 Service

#### 单元测试用例

```go
func TestRoleAdminService_CRUD(t *testing.T) {
    // Test 1: 创建自定义角色
    t.Run("创建自定义角色", func(t *testing.T) {
        service := setupRoleAdminService()
        ctx := context.Background()
        
        role := &model.Role{
            Name:        "自定义角色",
            Code:        "custom_role",
            Description: "测试用自定义角色",
            IsSystem:    false,
        }
        
        created, err := service.CreateRole(ctx, role)
        assert.NoError(t, err)
        assert.NotNil(t, created)
        assert.Equal(t, "custom_role", created.Code)
    })
    
    // Test 2: 系统角色不可删除
    t.Run("系统角色不可删除", func(t *testing.T) {
        service := setupRoleAdminService()
        ctx := context.Background()
        
        err := service.DeleteRole(ctx, 1) // system_admin
        assert.Error(t, err)
        assert.Equal(t, ErrSystemRoleCannotDelete, err)
    })
    
    // Test 3: 自定义角色可删除
    t.Run("自定义角色可删除", func(t *testing.T) {
        service := setupRoleAdminService()
        ctx := context.Background()
        
        err := service.DeleteRole(ctx, 10) // 自定义角色
        assert.NoError(t, err)
    })
    
    // Test 4: 配置角色权限
    t.Run("配置角色权限", func(t *testing.T) {
        service := setupRoleAdminService()
        ctx := context.Background()
        
        permIDs := []int64{1, 2, 3} // task:create, task:read, task:update
        err := service.AssignPermissions(ctx, 10, permIDs)
        assert.NoError(t, err)
        
        // 验证权限分配
        perms, err := service.GetRolePermissions(ctx, 10)
        assert.NoError(t, err)
        assert.Len(t, perms, 3)
    })
}
```

#### 功能测试用例

```bash
# Test 1: 创建自定义角色
curl -X POST http://localhost:8080/api/admin/roles \
  -H "Authorization: Bearer <admin_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "自定义角色",
    "code": "custom_role",
    "description": "测试用自定义角色"
  }'
# 预期：返回新创建的角色信息

# Test 2: 查询角色权限
curl -X GET http://localhost:8080/api/admin/roles/1/permissions \
  -H "Authorization: Bearer <admin_token>"
# 预期：返回角色的所有权限

# Test 3: 配置角色权限
curl -X PUT http://localhost:8080/api/admin/roles/1/permissions \
  -H "Authorization: Bearer <admin_token>" \
  -H "Content-Type: application/json" \
  -d '{"permission_ids":[1,2,3,4,5]}'
# 预期：成功更新权限

# Test 4: 删除系统角色（应失败）
curl -X DELETE http://localhost:8080/api/admin/roles/1 \
  -H "Authorization: Bearer <admin_token>"
# 预期：返回 400 Bad Request
```

#### 验收标准

- ✅ 角色 CRUD API 正常工作
- ✅ 权限配置 API 正常工作
- ✅ 系统角色不可删除
- ✅ 自定义角色可正常删除
- ✅ 角色权限查询正确返回
- ✅ 所有单元测试通过
- ✅ 所有功能测试通过

---

### 2.3 领域管理 API

#### 任务清单

- [ ] 2.3.1 实现领域 CRUD API
- [ ] 2.3.2 实现领域用户查询 API
- [ ] 2.3.3 实现领域执行器查询 API
- [ ] 2.3.4 实现领域统计 API

#### 实现产物

- `scheduler/internal/handler/domain_admin.go` - 领域管理 Handler
- `scheduler/internal/service/domain_admin_service.go` - 领域管理 Service

#### 单元测试用例

```go
func TestDomainAdminService_CRUD(t *testing.T) {
    // Test 1: 创建领域
    t.Run("创建领域", func(t *testing.T) {
        service := setupDomainAdminService()
        ctx := context.Background()
        
        domain := &model.Domain{
            Name:        "测试领域",
            Description: "测试用领域",
        }
        
        created, err := service.CreateDomain(ctx, domain)
        assert.NoError(t, err)
        assert.NotNil(t, created)
        assert.Equal(t, "测试领域", created.Name)
    })
    
    // Test 2: 删除有资源的领域应失败
    t.Run("删除有资源的领域失败", func(t *testing.T) {
        service := setupDomainAdminService()
        ctx := context.Background()
        
        err := service.DeleteDomain(ctx, 1)
        assert.Error(t, err)
        assert.Equal(t, ErrDomainHasResources, err)
    })
    
    // Test 3: 查询领域用户
    t.Run("查询领域用户", func(t *testing.T) {
        service := setupDomainAdminService()
        ctx := context.Background()
        
        users, err := service.GetDomainUsers(ctx, 1)
        assert.NoError(t, err)
        assert.NotNil(t, users)
    })
}
```

#### 功能测试用例

```bash
# Test 1: 创建领域
curl -X POST http://localhost:8080/api/admin/domains \
  -H "Authorization: Bearer <admin_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "新领域",
    "description": "测试领域"
  }'
# 预期：返回新创建的领域信息

# Test 2: 查询领域用户
curl -X GET http://localhost:8080/api/admin/domains/1/users \
  -H "Authorization: Bearer <admin_token>"
# 预期：返回该领域的所有用户

# Test 3: 查询领域执行器
curl -X GET http://localhost:8080/api/admin/domains/1/executors \
  -H "Authorization: Bearer <admin_token>"
# 预期：返回该领域的所有执行器
```

#### 验收标准

- ✅ 领域 CRUD API 正常工作
- ✅ 领域资源统计正确
- ✅ 有资源的领域不可删除
- ✅ 领域用户查询正确
- ✅ 所有单元测试通过
- ✅ 所有功能测试通过

---

### 2.4 执行器领域分配 API

#### 任务清单

- [ ] 2.4.1 实现执行器领域查询 API
- [ ] 2.4.2 实现执行器分配到领域 API
- [ ] 2.4.3 实现从领域移除执行器 API
- [ ] 2.4.4 实现执行器跨领域分发验证

#### 实现产物

- `scheduler/internal/handler/executor_admin.go` - 执行器管理扩展 Handler
- `scheduler/internal/service/executor_domain_service.go` - 执行器领域分配 Service

#### 单元测试用例

```go
func TestExecutorDomainService(t *testing.T) {
    // Test 1: 分配执行器到领域
    t.Run("分配执行器到领域", func(t *testing.T) {
        service := setupExecutorDomainService()
        ctx := context.Background()
        
        err := service.AssignExecutorToDomain(ctx, 1, 1, 1) // executor_id=1, domain_id=1, assigned_by=1
        assert.NoError(t, err)
        
        // 验证分配
        domains, err := service.GetExecutorDomains(ctx, 1)
        assert.NoError(t, err)
        assert.Contains(t, domains, int64(1))
    })
    
    // Test 2: 执行器可分配到多个领域
    t.Run("执行器分配到多个领域", func(t *testing.T) {
        service := setupExecutorDomainService()
        ctx := context.Background()
        
        // 分配到领域1
        err := service.AssignExecutorToDomain(ctx, 1, 1, 1)
        assert.NoError(t, err)
        
        // 分配到领域2
        err = service.AssignExecutorToDomain(ctx, 1, 2, 1)
        assert.NoError(t, err)
        
        // 验证
        domains, err := service.GetExecutorDomains(ctx, 1)
        assert.NoError(t, err)
        assert.Len(t, domains, 2)
    })
    
    // Test 3: 从领域移除执行器
    t.Run("从领域移除执行器", func(t *testing.T) {
        service := setupExecutorDomainService()
        ctx := context.Background()
        
        err := service.RemoveExecutorFromDomain(ctx, 1, 1)
        assert.NoError(t, err)
        
        // 验证
        domains, err := service.GetExecutorDomains(ctx, 1)
        assert.NoError(t, err)
        assert.NotContains(t, domains, int64(1))
    })
}
```

#### 功能测试用例

```bash
# Test 1: 查询执行器所属领域
curl -X GET http://localhost:8080/api/admin/executors/1/domains \
  -H "Authorization: Bearer <admin_token>"
# 预期：返回执行器所属的所有领域

# Test 2: 分配执行器到领域
curl -X POST http://localhost:8080/api/admin/executors/1/domains \
  -H "Authorization: Bearer <admin_token>" \
  -H "Content-Type: application/json" \
  -d '{"domain_ids":[1,2]}'
# 预期：成功分配到多个领域

# Test 3: 从领域移除执行器
curl -X DELETE http://localhost:8080/api/admin/executors/1/domains/1 \
  -H "Authorization: Bearer <admin_token>"
# 预期：成功从领域移除

# Test 4: 任务分发验证执行器领域归属
# 4.1 创建领域1的任务
curl -X POST http://localhost:8080/api/tasks \
  -H "Authorization: Bearer <admin_token>" \
  -d '{"name":"test","domain_id":1}'
# 预期：成功

# 4.2 指定领域2的执行器（应失败）
curl -X PUT http://localhost:8080/api/tasks/1 \
  -H "Authorization: Bearer <admin_token>" \
  -d '{"assigned_executor_id":"executor-domain2"}'
# 预期：返回错误，执行器不属于该领域
```

#### 验收标准

- ✅ 执行器领域查询 API 正常工作
- ✅ 执行器分配到领域 API 正常工作
- ✅ 执行器可分配到多个领域
- ✅ 从领域移除执行器 API 正常工作
- ✅ 任务分发验证执行器领域归属
- ✅ 所有单元测试通过
- ✅ 所有功能测试通过

---

## 阶段三：前端权限管理界面

**预计工期**：2 周

**目标**：完成前端权限管理界面的开发

### 3.1 用户管理界面

#### 任务清单

- [ ] 3.1.1 创建用户列表页面
- [ ] 3.1.2 创建用户编辑对话框
- [ ] 3.1.3 实现角色分配组件
- [ ] 3.1.4 实现领域分配组件
- [ ] 3.1.5 添加权限指令

#### 实现产物

- `web/src/views/admin/Users.vue` - 用户管理页面
- `web/src/components/admin/UserForm.vue` - 用户表单组件
- `web/src/components/admin/RoleSelector.vue` - 角色选择组件
- `web/src/components/admin/DomainSelector.vue` - 领域选择组件
- `web/src/directives/permission.ts` - 权限指令

#### 功能测试用例

```bash
# Test 1: 用户列表显示
# 预期：显示所有用户列表，包含用户名、邮箱、角色、领域等信息

# Test 2: 创建用户
# 2.1 点击"创建用户"按钮
# 2.2 填写用户信息
# 2.3 选择角色和领域
# 2.4 点击保存
# 预期：用户创建成功，列表刷新显示新用户

# Test 3: 编辑用户
# 3.1 点击用户行的"编辑"按钮
# 3.2 修改用户信息
# 3.3 点击保存
# 预期：用户信息更新成功

# Test 4: 分配角色
# 4.1 点击用户行的"编辑"按钮
# 4.2 在角色选择器中选择角色
# 4.3 点击保存
# 预期：角色分配成功，用户权限立即生效

# Test 5: 权限不足时按钮隐藏
# 使用普通用户登录
# 预期："创建用户"按钮被隐藏

# Test 6: 权限不足时操作被拒绝
# 普通用户尝试访问用户管理页面
# 预期：显示"无权限访问"或重定向到首页
```

#### 验收标准

- ✅ 用户列表页面正确显示所有用户
- ✅ 用户创建、编辑、删除功能正常
- ✅ 角色分配组件正常工作
- ✅ 领域分配组件正常工作
- ✅ 权限不足时按钮自动隐藏
- ✅ 所有功能测试通过

---

### 3.2 角色管理界面

#### 任务清单

- [ ] 3.2.1 创建角色列表页面
- [ ] 3.2.2 创建角色编辑对话框
- [ ] 3.2.3 实现权限配置组件
- [ ] 3.2.4 实现系统角色保护提示

#### 实现产物

- `web/src/views/admin/Roles.vue` - 角色管理页面
- `web/src/components/admin/RoleForm.vue` - 角色表单组件
- `web/src/components/admin/PermissionSelector.vue` - 权限选择组件

#### 功能测试用例

```bash
# Test 1: 角色列表显示
# 预期：显示所有角色，包含系统角色和自定义角色

# Test 2: 创建自定义角色
# 2.1 点击"创建角色"按钮
# 2.2 填写角色信息
# 2.3 选择权限
# 2.4 点击保存
# 预期：角色创建成功

# Test 3: 配置角色权限
# 3.1 点击角色行的"编辑"按钮
# 3.2 在权限列表中勾选权限
# 3.3 点击保存
# 预期：权限配置成功

# Test 4: 系统角色保护
# 4.1 点击系统角色的"删除"按钮
# 预期：显示提示"系统角色不可删除"

# Test 5: 自定义角色删除
# 5.1 点击自定义角色的"删除"按钮
# 5.2 确认删除
# 预期：角色删除成功
```

#### 验收标准

- ✅ 角色列表页面正确显示所有角色
- ✅ 角色创建、编辑功能正常
- ✅ 权限配置组件正常工作
- ✅ 系统角色不可删除
- ✅ 自定义角色可正常删除
- ✅ 所有功能测试通过

---

### 3.3 领域管理界面

#### 任务清单

- [ ] 3.3.1 创建领域列表页面
- [ ] 3.3.2 创建领域编辑对话框
- [ ] 3.3.3 显示领域统计信息

#### 实现产物

- `web/src/views/admin/Domains.vue` - 领域管理页面
- `web/src/components/admin/DomainForm.vue` - 领域表单组件

#### 功能测试用例

```bash
# Test 1: 领域列表显示
# 预期：显示所有领域，包含领域名称、描述、用户数、执行器数

# Test 2: 创建领域
# 2.1 点击"创建领域"按钮
# 2.2 填写领域信息
# 2.3 点击保存
# 预期：领域创建成功

# Test 3: 删除有资源的领域
# 3.1 点击有资源的领域的"删除"按钮
# 预期：显示提示"该领域存在资源，无法删除"
```

#### 验收标准

- ✅ 领域列表页面正确显示所有领域
- ✅ 领域创建、编辑功能正常
- ✅ 领域统计信息正确显示
- ✅ 有资源的领域不可删除
- ✅ 所有功能测试通过

---

### 3.4 执行器领域分配界面

#### 任务清单

- [ ] 3.4.1 在执行器详情页面添加领域分配功能
- [ ] 3.4.2 实现领域多选组件

#### 实现产物

- `web/src/components/admin/ExecutorDomainSelector.vue` - 执行器领域选择组件

#### 功能测试用例

```bash
# Test 1: 查看执行器所属领域
# 1.1 进入执行器管理页面
# 1.2 点击执行器的"详情"按钮
# 预期：显示执行器的领域分配信息

# Test 2: 分配执行器到领域
# 2.1 在执行器详情页面
# 2.2 选择要分配的领域
# 2.3 点击"分配"按钮
# 预期：执行器成功分配到所选领域

# Test 3: 从领域移除执行器
# 3.1 在执行器详情页面
# 3.2 点击已分配领域的"移除"按钮
# 预期：执行器成功从领域移除
```

#### 验收标准

- ✅ 执行器所属领域正确显示
- ✅ 执行器可分配到多个领域
- ✅ 执行器可从领域移除
- ✅ 所有功能测试通过

---

## 阶段四：系统集成与测试

**预计工期**：1-2 周

**目标**：完成系统集成、功能测试和性能测试

### 4.1 系统集成测试

#### 任务清单

- [ ] 4.1.1 用户认证与权限集成测试
- [ ] 4.1.2 任务分发与领域隔离集成测试
- [ ] 4.1.3 执行器跨领域协作集成测试
- [ ] 4.1.4 前后端权限控制集成测试
- [ ] 4.1.5 JWT Token 权限信息验证

#### 集成测试用例

```go
// 完整权限流程测试
func TestPermissionFlow_Integration(t *testing.T) {
    // Test 1: 用户登录并获取权限信息
    t.Run("用户登录获取权限", func(t *testing.T) {
        // 1.1 管理员创建用户
        adminToken := loginAsAdmin()
        createUserResp := createUser(adminToken, "testuser", "password123")
        assert.NotEmpty(t, createUserResp.UserID)
        
        // 1.2 分配角色和领域
        assignRoles(adminToken, createUserResp.UserID, []int64{2}, []int64{1})
        
        // 1.3 用户登录
        userToken := login("testuser", "password123")
        
        // 1.4 验证 Token 包含权限信息
        claims := parseToken(userToken)
        assert.Contains(t, claims.Roles, "domain_admin")
        assert.Contains(t, claims.Permissions, "task:create")
    })
    
    // Test 2: 领域隔离验证
    t.Run("领域隔离验证", func(t *testing.T) {
        // 2.1 领域1的管理员创建任务
        domain1Admin := loginAsDomain1Admin()
        task := createTask(domain1Admin, "domain1-task", 1)
        assert.NotNil(t, task)
        
        // 2.2 领域2的用户无法访问领域1的任务
        domain2User := loginAsDomain2User()
        resp := getTask(domain2User, task.ID)
        assert.Equal(t, 403, resp.StatusCode)
        
        // 2.3 领域1的用户可以访问
        domain1User := loginAsDomain1User()
        resp = getTask(domain1User, task.ID)
        assert.Equal(t, 200, resp.StatusCode)
    })
    
    // Test 3: 执行器领域验证
    t.Run("执行器领域验证", func(t *testing.T) {
        // 3.1 管理员分配执行器到领域1
        adminToken := loginAsAdmin()
        assignExecutorToDomain(adminToken, executorID, []int64{1, 2})
        
        // 3.2 领域1的任务只能分发到领域1或跨领域的执行器
        domain1Task := createTask(adminToken, "domain1-task", 1)
        assignExecutor(domain1Task.ID, "executor-domain2-only") // 只属于领域2
        resp := triggerTask(adminToken, domain1Task.ID)
        assert.Equal(t, 400, resp.StatusCode) // 应该失败
        
        // 3.3 指定跨领域执行器应该成功
        assignExecutor(domain1Task.ID, "executor-both-domains")
        resp = triggerTask(adminToken, domain1Task.ID)
        assert.Equal(t, 200, resp.StatusCode)
    })
}
```

#### 验收标准

- ✅ 所有集成测试通过
- ✅ 权限系统与业务系统正常协作
- ✅ 前后端权限控制一致性验证通过
- ✅ JWT Token 权限信息正确

---

### 4.2 性能测试

#### 任务清单

- [ ] 4.2.1 权限检查性能测试
- [ ] 4.2.2 权限缓存性能测试
- [ ] 4.2.3 并发权限验证测试

#### 性能测试用例

```bash
# Test 1: 权限检查性能
ab -n 1000 -c 100 -H "Authorization: Bearer <token>" \
  http://localhost:8080/api/tasks
# 预期：平均响应时间 < 50ms

# Test 2: 权限缓存命中率
# 重复访问相同资源
for i in {1..100}; do
  curl -s -o /dev/null -w "%{time_total}\n" \
    -H "Authorization: Bearer <token>" \
    http://localhost:8080/api/tasks/1
done | awk '{sum+=$1; count++} END {print "平均响应时间:", sum/count "秒"}'
# 预期：缓存命中后响应时间显著降低

# Test 3: 并发权限验证
ab -n 5000 -c 200 -H "Authorization: Bearer <token>" \
  http://localhost:8080/api/admin/users
# 预期：系统稳定，无错误
```

#### 验收标准

- ✅ 权限检查平均响应时间 < 50ms
- ✅ 权限缓存命中率 > 80%（重复访问）
- ✅ 并发 200 用户时系统稳定
- ✅ 无内存泄漏和性能退化

---

### 4.3 安全测试

#### 任务清单

- [ ] 4.3.1 SQL 注入测试
- [ ] 4.3.2 权限绕过测试
- [ ] 4.3.3 跨领域访问测试
- [ ] 4.3.4 Token 安全测试

#### 安全测试用例

```bash
# Test 1: SQL 注入测试
curl -X GET "http://localhost:8080/api/tasks?id=1' OR '1'='1" \
  -H "Authorization: Bearer <token>"
# 预期：返回错误或空结果，不会执行注入

# Test 2: 权限绕过测试
curl -X POST http://localhost:8080/api/admin/users \
  -H "Authorization: Bearer <user_token>" \  # 普通用户 Token
  -d '{"username":"hacker"}'
# 预期：返回 403 Forbidden

# Test 3: 跨领域访问测试
curl -X GET http://localhost:8080/api/domains/2/tasks \
  -H "Authorization: Bearer <domain1_user_token>"
# 预期：返回 403 Forbidden

# Test 4: Token 伪造测试
curl -X GET http://localhost:8080/api/admin/users \
  -H "Authorization: Bearer fake_token_12345"
# 预期：返回 401 Unauthorized
```

#### 验收标准

- ✅ 无 SQL 注入漏洞
- ✅ 权限绕过攻击失败
- ✅ 跨领域访问被正确阻止
- ✅ 无效 Token 被正确拒绝

---

## 实施进度追踪

### 里程碑

| 里程碑 | 预计完成时间 | 状态 |
|--------|-------------|------|
| 阶段一完成 | 第 2 周末 | ⏳ 待开始 |
| 阶段二完成 | 第 5 周末 | ⏳ 待开始 |
| 阶段三完成 | 第 7 周末 | ⏳ 待开始 |
| 阶段四完成 | 第 9-10 周末 | ⏳ 待开始 |

### 周报模板

```markdown
## 第 X 周进度报告

### 完成内容
- [ ] 任务1
- [ ] 任务2

### 遇到的问题
- 问题1：描述
- 解决方案：...

### 下周计划
- [ ] 任务1
- [ ] 任务2

### 指标
- 完成的任务数：X
- 发现的 Bug 数：X
- 修复的 Bug 数：X
```

---

## 风险评估

| 风险 | 影响 | 可能性 | 缓解措施 |
|------|------|--------|----------|
| 数据库迁移失败 | 高 | 低 | 提前备份，准备回滚脚本 |
| 权限逻辑复杂度过高 | 中 | 中 | 充分单元测试，代码审查 |
| 前后端集成问题 | 中 | 中 | 提前定义接口，持续集成测试 |
| 性能不达标 | 中 | 低 | 提前进行性能测试，优化缓存 |
| 用户接受度低 | 低 | 中 | 充分用户培训，详细文档 |

---

## 资源需求

### 人力资源

| 角色 | 人数 | 负责阶段 |
|------|------|----------|
| 后端开发工程师 | 2 | 阶段一、阶段二 |
| 前端开发工程师 | 1 | 阶段三 |
| 测试工程师 | 1 | 阶段四 |
| 技术负责人 | 1 | 全程 |

### 环境需求

- 开发环境：3 套
- 测试环境：2 套
- 预生产环境：1 套

---

## 附录

### A. API 完整列表

详见 [DESIGN.md](./DESIGN.md#7-api-接口设计)

### B. 数据库变更脚本

详见 [DESIGN.md](./DESIGN.md#5-数据库设计)

### C. 术语表

| 术语 | 说明 |
|------|------|
| RBAC | Role-Based Access Control，基于角色的访问控制 |
| 多租户 | Multi-tenancy，多个租户共享系统资源但数据隔离 |
| 领域 | Domain，业务隔离边界 |

### D. 参考资料

1. RBAC 标准模型
2. NIST RBAC 模型
3. Vue 3 权限控制最佳实践
4. Go Gin 权限中间件实践

---

**文档版本历史**：

| 版本 | 日期 | 作者 | 变更说明 |
|------|------|------|----------|
| v1.0 | 2026-05-14 | BDopsFlow Team | 初始版本 |
