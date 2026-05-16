# BDopsFlow 用户权限与多租户管理系统设计方案

## 文档信息

- **项目名称**：BDopsFlow 用户权限与多租户管理系统
- **版本**：v1.0
- **创建日期**：2026-05-14
- **作者**：BDopsFlow Team
- **状态**：已批准

---

## 1. 项目背景与目标

### 1.1 项目背景

BDopsFlow 是一个分布式工作流调度平台，当前已实现基础的用户认证和简单角色体系（admin/operator/viewer）。随着系统规模的扩大和业务需求的发展，需要扩展完整的用户权限管理、多领域隔离、角色控制系统，以满足以下需求：

1. **多租户隔离**：不同业务领域的任务、执行器、日志需要完全隔离
2. **灵活的权限控制**：支持细粒度的权限配置和管理
3. **分权管理**：不同角色承担不同职责，相互制约
4. **执行器资源共享**：一个执行器可以服务于多个领域，提高资源利用率

### 1.2 项目目标

1. 实现基于 RBAC 的标准权限管理系统
2. 建立领域级多租户隔离机制
3. 支持执行器跨领域分配
4. 提供完整的前后端权限控制能力
5. 保证系统安全性和可审计性

---

## 2. 系统架构

### 2.1 技术栈

| 组件 | 技术选型 | 说明 |
|------|----------|------|
| 后端框架 | Go + Gin | HTTP API |
| RPC通信 | gRPC | 调度中心与执行器通信 |
| 数据库 | rqlite | 分布式数据库 |
| 缓存/锁 | Redis | 分布式锁、缓存 |
| 前端框架 | Vue 3 + TypeScript | Web界面 |
| UI组件库 | Element Plus | 前端组件 |
| 认证 | JWT | Token认证 |

### 2.2 核心设计原则

1. **多租户隔离**：以领域（Domain）为隔离边界，每个领域的资源完全独立
2. **最小权限原则**：用户只能访问被授权的资源
3. **分权制衡**：不同角色相互制约，防止权限滥用
4. **可审计性**：所有敏感操作记录日志

---

## 3. 角色体系

### 3.1 三层角色模型

#### 3.1.1 系统预设角色（不可删除）

| 角色代码 | 角色名称 | 描述 | 权限范围 |
|----------|----------|------|----------|
| `system_admin` | 全局管理员 | 系统最高权限 | 所有领域、所有功能 |
| `domain_admin` | 领域管理员 | 领域级管理权限 | 指定领域内的所有资源 |
| `user` | 普通用户 | 基础查看和操作权限 | 指定领域内的查看和手动触发 |

#### 3.1.2 自定义角色

- 支持创建领域级自定义角色
- 继承领域管理员的部分权限
- 可细化权限粒度

### 3.2 角色权限继承关系

```
全局管理员 (system_admin)
    │
    ├── 完整系统管理权限
    ├── 所有领域访问权限
    └── 所有资源管理权限
            │
            ├── 领域管理员 (domain_admin)
            │       │
            │       ├── 本领域所有资源管理
            │       ├── 本领域用户管理
            │       └── 本领域执行器管理
            │               │
            │               ├── 普通用户 (user)
            │               │       │
            │               │       ├── 本领域资源查看
            │               │       └── 本领域任务手动触发
            │               │
            │               └── 自定义角色
            │                       │
            │                       └── 基于领域管理员权限的自定义组合
```

---

## 4. 权限体系

### 4.1 Resource-Action 权限模型

**权限格式**：`{resource}:{action}`

### 4.2 资源分类

| 资源代码 | 资源名称 | 说明 |
|----------|----------|------|
| `user` | 用户管理 | 用户 CRUD |
| `role` | 角色管理 | 角色 CRUD |
| `permission` | 权限管理 | 权限查看 |
| `domain` | 领域管理 | 领域 CRUD |
| `executor` | 执行器管理 | 执行器管理 |
| `task` | 任务管理 | 任务 CRUD |
| `log` | 日志管理 | 日志查看 |
| `workflow` | 工作流管理 | 工作流 CRUD |

### 4.3 操作类型

| 操作代码 | 操作名称 | 说明 |
|----------|----------|------|
| `create` | 创建 | 新增资源 |
| `read` | 读取 | 查看资源 |
| `update` | 更新 | 修改资源 |
| `delete` | 删除 | 删除资源 |
| `manage` | 管理 | 完整管理权限（包含所有操作） |
| `trigger` | 触发 | 手动触发任务 |
| `assign` | 分配 | 分配执行器到领域 |

### 4.4 完整权限列表

```json
{
  "user:create": "创建用户",
  "user:read": "查看用户",
  "user:update": "更新用户",
  "user:delete": "删除用户",
  "user:manage": "完整管理用户",

  "role:create": "创建角色",
  "role:read": "查看角色",
  "role:update": "更新角色",
  "role:delete": "删除角色",
  "role:manage": "完整管理角色",

  "permission:read": "查看权限列表",

  "domain:create": "创建领域",
  "domain:read": "查看领域",
  "domain:update": "更新领域",
  "domain:delete": "删除领域",
  "domain:manage": "完整管理领域",

  "executor:read": "查看执行器",
  "executor:assign": "分配执行器到领域",
  "executor:manage": "完整管理执行器",

  "task:create": "创建任务",
  "task:read": "查看任务",
  "task:update": "更新任务",
  "task:delete": "删除任务",
  "task:trigger": "手动触发任务",
  "task:manage": "完整管理任务",

  "log:read": "查看日志",
  "log:delete": "删除日志",
  "log:manage": "完整管理日志",

  "workflow:create": "创建工作流",
  "workflow:read": "查看工作流",
  "workflow:update": "更新工作流",
  "workflow:delete": "删除工作流",
  "workflow:manage": "完整管理工作流"
}
```

---

## 5. 数据库设计

### 5.1 ER 图

```
┌──────────────┐     ┌──────────────────┐     ┌─────────────────┐
│    users     │────▶│   user_roles     │◀────│      roles      │
├──────────────┤     ├──────────────────┤     ├─────────────────┤
│ id           │     │ id               │     │ id              │
│ username     │     │ user_id          │     │ name            │
│ password     │     │ role_id          │     │ code            │
│ email        │     │ domain_id        │     │ description     │
│ domain_id    │     │ created_at       │     │ is_system       │
│ role         │     └──────────────────┘     │ domain_id       │
│ is_active    │            │                │ created_at      │
│ last_login   │            │                └─────────────────┘
│ created_by   │            │                        │
└──────────────┘            │                        │
      │                    │                        ▼
      │              ┌─────┴───────┐        ┌────────────────────┐
      │              │             │        │  role_permissions  │
      │              ▼             │        ├────────────────────┤
┌──────────────┐  ┌──────────────────┐       │ id                 │
│   domains    │  │ role_permissions │       │ role_id            │
├──────────────┤  ├──────────────────┤       │ permission_id      │
│ id           │  │ id               │       │ created_at         │
│ name         │◀─│ role_id          │       └────────────────────┘
│ description  │  │ permission_id    │               │
│ created_at   │  │ created_at       │               │
└──────────────┘  └──────────────────┘               ▼
      │              ┌──────────────────┐     ┌─────────────────┐
      │              │   permissions    │     │    executors    │
      │              ├──────────────────┤     ├─────────────────┤
      │              │ id               │◀────│ executor_id     │
      │              │ resource         │     │ is_global       │
      │              │ action           │     └─────────────────┘
      │              │ description      │             │
      │              └──────────────────┘             ▼
      │                                   ┌─────────────────────┐
      │                                   │  domain_executors  │
      │                                   ├─────────────────────┤
      │                                   │ id                  │
      └──────────────────────────────────▶│ domain_id          │
                                          │ executor_id         │
                                          │ assigned_by         │
                                          │ created_at          │
                                          └─────────────────────┘
```

### 5.2 新增表结构

#### 5.2.1 roles（角色表）

```sql
CREATE TABLE roles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    code TEXT NOT NULL UNIQUE,               -- 角色代码
    description TEXT,
    is_system BOOLEAN DEFAULT 0,             -- 系统预设角色
    domain_id INTEGER,                        -- 领域专属角色
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (domain_id) REFERENCES domains(id) ON DELETE SET NULL
);

CREATE UNIQUE INDEX idx_roles_name ON roles(name);
CREATE UNIQUE INDEX idx_roles_code ON roles(code);
CREATE INDEX idx_roles_domain_id ON roles(domain_id);
```

**字段说明**：

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER | 主键 |
| name | TEXT | 角色名称 |
| code | TEXT | 角色代码（唯一） |
| description | TEXT | 角色描述 |
| is_system | BOOLEAN | 是否系统预设（不可删除） |
| domain_id | INTEGER | 领域专属角色，NULL表示全局角色 |
| created_at | DATETIME | 创建时间 |
| updated_at | DATETIME | 更新时间 |

#### 5.2.2 permissions（权限表）

```sql
CREATE TABLE permissions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    resource TEXT NOT NULL,
    action TEXT NOT NULL,
    description TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(resource, action)
);

CREATE UNIQUE INDEX idx_permissions_resource_action ON permissions(resource, action);
```

**字段说明**：

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER | 主键 |
| resource | TEXT | 资源类型 |
| action | TEXT | 操作类型 |
| description | TEXT | 权限描述 |
| created_at | DATETIME | 创建时间 |

#### 5.2.3 role_permissions（角色权限映射表）

```sql
CREATE TABLE role_permissions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    role_id INTEGER NOT NULL,
    permission_id INTEGER NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE,
    FOREIGN KEY (permission_id) REFERENCES permissions(id) ON DELETE CASCADE,
    UNIQUE(role_id, permission_id)
);

CREATE INDEX idx_role_permissions_role_id ON role_permissions(role_id);
CREATE INDEX idx_role_permissions_permission_id ON role_permissions(permission_id);
```

**字段说明**：

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER | 主键 |
| role_id | INTEGER | 角色ID |
| permission_id | INTEGER | 权限ID |
| created_at | DATETIME | 创建时间 |

#### 5.2.4 user_roles（用户角色映射表）

```sql
CREATE TABLE user_roles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    role_id INTEGER NOT NULL,
    domain_id INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE,
    FOREIGN KEY (domain_id) REFERENCES domains(id) ON DELETE SET NULL,
    UNIQUE(user_id, role_id, domain_id)
);

CREATE INDEX idx_user_roles_user_id ON user_roles(user_id);
CREATE INDEX idx_user_roles_role_id ON user_roles(role_id);
CREATE INDEX idx_user_roles_domain_id ON user_roles(domain_id);
```

**字段说明**：

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER | 主键 |
| user_id | INTEGER | 用户ID |
| role_id | INTEGER | 角色ID |
| domain_id | INTEGER | 领域ID（NULL表示全局角色） |
| created_at | DATETIME | 创建时间 |

#### 5.2.5 domain_executors（执行器领域分配表）

```sql
CREATE TABLE domain_executors (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    domain_id INTEGER NOT NULL,
    executor_id INTEGER NOT NULL,
    assigned_by INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (domain_id) REFERENCES domains(id) ON DELETE CASCADE,
    FOREIGN KEY (executor_id) REFERENCES executors(id) ON DELETE CASCADE,
    FOREIGN KEY (assigned_by) REFERENCES users(id) ON DELETE SET NULL,
    UNIQUE(domain_id, executor_id)
);

CREATE INDEX idx_domain_executors_domain_id ON domain_executors(domain_id);
CREATE INDEX idx_domain_executors_executor_id ON domain_executors(executor_id);
```

**字段说明**：

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER | 主键 |
| domain_id | INTEGER | 领域ID |
| executor_id | INTEGER | 执行器ID（关联 executors 表） |
| assigned_by | INTEGER | 分配者ID |
| created_at | DATETIME | 创建时间 |

### 5.3 修改现有表

#### 5.3.1 users（用户表）

```sql
-- 新增字段
ALTER TABLE users ADD COLUMN is_active BOOLEAN DEFAULT 1;
ALTER TABLE users ADD COLUMN last_login_at DATETIME;
ALTER TABLE users ADD COLUMN created_by INTEGER;
```

#### 5.3.2 executors（执行器表）

```sql
-- 新增字段
ALTER TABLE executors ADD COLUMN is_global BOOLEAN DEFAULT 0;
```

---

## 6. 权限验证机制

### 6.1 分层验证架构

#### 6.1.1 第一层：中间件层（粗粒度）

**位置**：`scheduler/internal/middleware/auth.go`

**职责**：
1. JWT Token 验证
2. 用户身份认证
3. 基础角色检查
4. 请求速率限制

**流程**：

```
请求 → Token 解析 → 用户认证 → 角色提取 → 基础权限预检 → 路由匹配
```

#### 6.1.2 第二层：Service 层（细粒度）

**位置**：`scheduler/internal/service/{resource}_service.go`

**职责**：
1. 资源归属验证
2. 数据领域隔离
3. 跨领域访问控制
4. 业务规则验证

### 6.2 权限检查流程

```go
// 权限检查伪代码
func CheckPermission(ctx context.Context, resource, action string, domainID int64) error {
    // 1. 获取当前用户
    user := GetCurrentUser(ctx)
    if user == nil {
        return ErrUnauthorized
    }

    // 2. 检查是否是系统管理员
    if user.IsSystemAdmin {
        return nil
    }

    // 3. 检查用户是否有该资源权限
    if !user.HasPermission(resource, action) {
        return ErrForbidden
    }

    // 4. 检查用户是否有该领域访问权限
    if !user.CanAccessDomain(domainID) {
        return ErrDomainAccessDenied
    }

    return nil
}
```

### 6.3 领域隔离机制

```go
// 自动注入领域过滤
func (s *TaskService) ListTasks(ctx context.Context) ([]*Task, error) {
    user := GetCurrentUser(ctx)

    // 全局管理员可查看所有领域
    if user.IsSystemAdmin {
        return s.db.ListAllTasks()
    }

    // 领域管理员或普通用户只能查看本领域
    domainIDs := user.GetAccessibleDomains()
    return s.db.ListTasksByDomains(domainIDs)
}
```

### 6.4 任务分发权限验证

```go
func (s *SchedulerService) TriggerTask(ctx context.Context, taskID int64) error {
    task, err := s.GetTask(taskID)
    if err != nil {
        return err
    }

    // 1. 验证任务存在
    if task == nil {
        return ErrTaskNotFound
    }

    // 2. 验证用户有触发该任务的权限
    if !permission.HasResourcePermission(ctx, "task", "trigger", task.DomainID) {
        return ErrForbidden
    }

    // 3. 验证执行器属于该领域
    if task.AssignedExecutorID != "" {
        if !executor.BelongsToDomain(ctx, task.AssignedExecutorID, task.DomainID) {
            return ErrExecutorNotInDomain
        }
    }

    // 4. 执行分发逻辑
    return s.dispatchTask(ctx, task)
}
```

---

## 7. API 接口设计

### 7.1 用户管理接口

| 方法 | 接口 | 权限 | 说明 |
|------|------|------|------|
| GET | /api/admin/users | system_admin | 获取用户列表 |
| POST | /api/admin/users | system_admin | 创建用户 |
| GET | /api/admin/users/:id | system_admin | 获取用户详情 |
| PUT | /api/admin/users/:id | system_admin | 更新用户 |
| DELETE | /api/admin/users/:id | system_admin | 删除用户 |
| POST | /api/admin/users/:id/roles | system_admin | 分配角色 |
| DELETE | /api/admin/users/:id/roles/:roleId | system_admin | 移除角色 |
| POST | /api/admin/users/:id/domains | system_admin | 分配领域 |
| DELETE | /api/admin/users/:id/domains/:domainId | system_admin | 移除领域 |

### 7.2 角色管理接口

| 方法 | 接口 | 权限 | 说明 |
|------|------|------|------|
| GET | /api/admin/roles | system_admin | 获取角色列表 |
| POST | /api/admin/roles | system_admin | 创建角色 |
| GET | /api/admin/roles/:id | system_admin | 获取角色详情 |
| PUT | /api/admin/roles/:id | system_admin | 更新角色 |
| DELETE | /api/admin/roles/:id | system_admin | 删除角色（非系统角色） |
| GET | /api/admin/roles/:id/permissions | system_admin | 获取角色权限 |
| PUT | /api/admin/roles/:id/permissions | system_admin | 更新角色权限 |

### 7.3 权限管理接口

| 方法 | 接口 | 权限 | 说明 |
|------|------|------|------|
| GET | /api/admin/permissions | system_admin | 获取权限列表 |

### 7.4 领域管理接口

| 方法 | 接口 | 权限 | 说明 |
|------|------|------|------|
| GET | /api/admin/domains | system_admin | 获取领域列表 |
| POST | /api/admin/domains | system_admin | 创建领域 |
| GET | /api/admin/domains/:id | system_admin | 获取领域详情 |
| PUT | /api/admin/domains/:id | system_admin | 更新领域 |
| DELETE | /api/admin/domains/:id | system_admin | 删除领域 |

### 7.5 执行器管理接口（扩展）

| 方法 | 接口 | 权限 | 说明 |
|------|------|------|------|
| GET | /api/admin/executors | system_admin | 获取执行器列表（含领域信息） |
| GET | /api/admin/executors/:id/domains | system_admin | 获取执行器所属领域 |
| POST | /api/admin/executors/:id/domains | system_admin | 分配执行器到领域 |
| DELETE | /api/admin/executors/:id/domains/:domainId | system_admin | 从领域移除执行器 |

---

## 8. 前端权限控制

### 8.1 权限指令

```typescript
// 权限指令
export const permission: Directive = {
  mounted(el, binding) {
    const { value } = binding
    const permissions = JSON.parse(
      localStorage.getItem('permissions') || '[]'
    )

    if (value && !permissions.includes(value)) {
      el.parentNode?.removeChild(el)
    }
  }
}
```

### 8.2 权限 Hook

```typescript
// 权限 Hook
export function usePermission() {
  const userStore = useUserStore()

  function hasPermission(permission: string): boolean {
    if (userStore.isSystemAdmin) return true
    return userStore.permissions.includes(permission)
  }

  function hasDomainPermission(domainId: number, permission: string): boolean {
    if (userStore.isSystemAdmin) return true
    return userStore.domains.includes(domainId) && hasPermission(permission)
  }

  return {
    hasPermission,
    hasDomainPermission
  }
}
```

### 8.3 动态菜单

```typescript
// 菜单配置
export const menuConfig: MenuItem[] = [
  {
    path: '/',
    name: 'Dashboard',
    permission: 'dashboard:read'
  },
  {
    path: '/tasks',
    name: 'Tasks',
    permission: 'task:read'
  },
  {
    path: '/admin',
    name: 'System Admin',
    permission: 'user:manage',
    children: [
      { path: '/admin/users', permission: 'user:manage' },
      { path: '/admin/roles', permission: 'role:manage' },
      { path: '/admin/domains', permission: 'domain:manage' }
    ]
  }
]
```

---

## 9. 安全考虑

### 9.1 密码安全

- 使用 bcrypt 加密存储
- 密码强度验证
- 定期强制修改密码

### 9.2 Token 安全

- JWT 短期令牌（24小时）
- Refresh Token 机制
- Token 黑名单（退出登录）

### 9.3 数据隔离

- 每个查询自动添加领域过滤
- 跨领域访问严格控制
- 敏感操作审计日志

### 9.4 审计日志

- 记录所有权限敏感操作
- 用户登录/登出记录
- 资源创建/修改/删除记录

---

## 10. 性能优化

### 10.1 权限缓存

- 用户权限信息缓存到 Redis
- 缓存过期时间：5分钟
- 权限变更时主动刷新缓存

### 10.2 数据库优化

- 适当添加索引
- 查询优化
- 连接池管理

---

## 11. 兼容性考虑

### 11.1 向后兼容

- 保留原有的 role 字段用于兼容
- 新旧权限系统共存
- 提供数据迁移脚本

### 11.2 数据迁移

```sql
-- 将原有 admin 角色的用户迁移到 system_admin
UPDATE users
SET role = 'system_admin'
WHERE role = 'admin';

-- 将原有 operator 角色的用户迁移到 domain_admin
UPDATE users
SET role = 'domain_admin'
WHERE role = 'operator';

-- 将原有 viewer 角色的用户迁移到 user
UPDATE users
SET role = 'user'
WHERE role = 'viewer';
```

---

## 12. 文档更新

本文档需要同步更新以下现有文档：

1. **ARCHITECTURE.md** - 添加权限体系架构说明
2. **API.md** - 添加权限管理相关 API
3. **DEPLOYMENT.md** - 添加权限初始化说明
4. **数据库变更日志** - 记录 schema.sql 的修改

---

## 13. 附录

### 13.1 术语表

| 术语 | 说明 |
|------|------|
| RBAC | Role-Based Access Control，基于角色的访问控制 |
| 多租户 | Multi-tenancy，多个租户共享系统资源但数据隔离 |
| 领域 | Domain，业务隔离边界 |
| 权限 | Permission，资源操作的许可 |

### 13.2 参考资料

1. RBAC 标准模型
2. NIST RBAC 模型
3. Vue 3 权限控制最佳实践

---

**文档版本历史**：

| 版本 | 日期 | 作者 | 变更说明 |
|------|------|------|----------|
| v1.0 | 2026-05-14 | BDopsFlow Team | 初始版本 |
