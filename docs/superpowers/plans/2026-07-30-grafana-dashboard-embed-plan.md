# Implementation Plan: Grafana 图表嵌入仪表盘

- **Status**: Draft
- **Spec**: [2026-07-30-grafana-dashboard-embed-design.md](../specs/2026-07-30-grafana-dashboard-embed-design.md)
- **Date**: 2026-07-30
- **Author**: writing-plans

## Goals

- 管理员可在独立管理页 CRUD 多个 Grafana 图表配置（含启停、排序、宽度、高度）
- 仪表盘自动渲染已启用图表，响应式 3 列网格，iframe 沙箱安全加载
- 支持匿名嵌入和 URL 带 cookie/token 参数嵌入两种认证方式
- 所有写操作记录审计日志
- 后端单元测试覆盖校验规则和 CRUD

## Out of Scope

- Grafana API 集成（不调查询接口、不解析 panel 数据）
- 图表数据代理（iframe 直接加载 Grafana URL）
- 多租户隔离（系统级资源）
- 拖拽排序（行内编辑数字 + 批量保存）
- 自动迁移执行（迁移脚本手动执行，项目约定）

## Tech Stack (resolved)

spec 无待定选型。所有技术栈沿用项目现有：
- 后端：Go + Gin + rqlite（`database.DB` 接口）
- 前端：Vue 3 + TypeScript + Element Plus
- 数据访问：**直接在 service 包内**（项目无独立 repository 层，参考 `service/webhook.go` 模式）

**spec 偏差说明**：spec §3.2 写的是 `scheduler/internal/grafana_dashboard/service.go` + `repository.go`，但实际项目模式是 `scheduler/internal/service/grafana_dashboard.go` 单文件（与 `webhook.go`、`api_test_service.go` 一致）。本计划按实际模式执行。

## Phases

### Phase 1: 数据库与后端 Model

**Goal**: 建表并定义 Go model，为后续 service 提供数据结构。

**Tasks**:
- [ ] 创建迁移脚本 `deploy/migrations/v8_grafana_dashboards.sql`：
  - `CREATE TABLE IF NOT EXISTS bdopsflow_grafana_dashboards`（字段见 spec §2）
  - `CREATE INDEX idx_grafana_dashboards_enabled_sort`
  - 头部注释参考 `v4_api_test.sql` 格式（版本、日期、描述）
- [ ] 同步更新 `deploy/schema.sql`：在系统表区域（`bdopsflow_system_config` 附近）追加相同 DDL
- [ ] 创建 `scheduler/internal/model/grafana_dashboard.go`：
  ```go
  type GrafanaDashboard struct {
      ID          int64     `db:"id" json:"id"`
      Name        string    `db:"name" json:"name"`
      Description string    `db:"description" json:"description"`
      EmbedURL    string    `db:"embed_url" json:"embed_url"`
      Width       string    `db:"width" json:"width"`
      HeightPx    int       `db:"height_px" json:"height_px"`
      SortOrder   int       `db:"sort_order" json:"sort_order"`
      Enabled     int       `db:"enabled" json:"enabled"`
      CreatedAt   time.Time `db:"created_at" json:"created_at"`
      UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
  }
  ```
  - 参考 `model/webhook.go` 或 `model/system_config.go` 的字段 tag 风格

**Verification**:
- `cd scheduler && go build ./...` 通过
- 迁移脚本 SQL 语法正确（`sqlite3` 本地执行无报错，或人工审查）

**Done when**: model 可编译，迁移脚本和 schema.sql 已更新。

---

### Phase 2: 后端 Service 层（含校验和 CRUD）

**Goal**: 实现业务逻辑、校验规则、DB 访问，全部在 `service` 包内。

**Tasks**:
- [ ] 在 `scheduler/internal/service/errors.go` 追加 Grafana 错误定义：
  - `ErrGrafanaDashboardNotFound = NewAppError(19001, "图表不存在")`
  - `ErrGrafanaInvalidURL = NewAppError(19002, "嵌入 URL 必须以 http:// 或 https:// 开头")`
  - `ErrGrafanaInvalidWidth = NewAppError(19003, "宽度值无效")`
  - `ErrGrafanaInvalidHeight = NewAppError(19004, "高度范围 120-1200")`
  - 在 `errorCodeMap` 追加这四个错误到 19xxx 段（避开已用的 11-18 段）
- [ ] 创建 `scheduler/internal/service/grafana_dashboard.go`：
  - 结构体：`type GrafanaDashboardService struct { db database.DB }`
  - 构造函数：`func NewGrafanaDashboardService(db database.DB) *GrafanaDashboardService`
  - 方法（全部接收 `ctx context.Context` 首参）：
    - `ListEnabled(ctx) ([]model.GrafanaDashboard, error)` — `WHERE enabled=1 ORDER BY sort_order ASC`
    - `ListAll(ctx) ([]model.GrafanaDashboard, error)` — `ORDER BY sort_order ASC`
    - `GetByID(ctx, id int64) (*model.GrafanaDashboard, error)`
    - `Create(ctx, d *model.GrafanaDashboard) (*model.GrafanaDashboard, error)` — 调用 `validate` 后 INSERT
    - `Update(ctx, id int64, d *model.GrafanaDashboard) error` — 调用 `validate` 后 UPDATE，检查 `RowsAffected`
    - `Delete(ctx, id int64) error` — DELETE，检查 `RowsAffected`
    - `Reorder(ctx, items []ReorderItem) error` — 接收 `[{id, sort_order}]`，用 `s.db.WriteParameterized(stmts)` 批量 UPDATE（rqlite 单次 Raft 提交，原子）
  - 私有方法：`validate(d *model.GrafanaDashboard) error`：
    - name 非空且 ≤ 100 字符
    - embed_url 匹配 `^https?://` 且 ≤ 2048 字符（用 `net/url` 解析校验，不引入正则库）
    - width ∈ {third, half, full}
    - height_px ∈ [120, 1200]
    - sort_order ≥ 0
  - DB 操作参考 `service/webhook.go` 的 `WriteOneParameterized` / `QueryOneParameterized` 模式
  - 行扫描用 `service/scanners.go` 的 `rowInt64`/`rowString` 等辅助函数
  - 时间解析用 `service/helpers.go` 的 `parseDateTime`
- [ ] 创建 `scheduler/internal/service/grafana_dashboard_test.go`：
  - 测试 `validate` 的所有分支（空 name、非法 URL 协议、超长 URL、非法 width、height 越界、负数 sort_order、合法用例）
  - 如项目有 DB mock 工具，测试 CRUD 的 DB 调用；否则至少测试 validate 逻辑

**Verification**:
- `cd scheduler && go build ./...` 通过
- `cd scheduler && go test ./internal/service/ -run GrafanaDashboard -v` 通过
- `go vet ./internal/service/...` 无警告

**Done when**: service 层全部方法可编译，validate 测试通过。

---

### Phase 3: 后端 Handler 与路由注册

**Goal**: 暴露 REST API，接入审计中间件，完成 app 初始化注入。

**Tasks**:
- [ ] 创建 `scheduler/internal/handler/grafana_dashboard.go`：
  - 结构体：`type GrafanaDashboardHandler struct { svc *service.GrafanaDashboardService }`
  - 构造函数：`func NewGrafanaDashboardHandler(svc *service.GrafanaDashboardService) *GrafanaDashboardHandler`
  - 方法（参考 `handler/webhook.go` 模式）：
    - `ListEnabled(c *gin.Context)` — 调用 `svc.ListEnabled`，`response.Success(c, data)`
    - `ListAll(c *gin.Context)` — 调用 `svc.ListAll`，`response.Success(c, data)`
    - `GetByID(c *gin.Context)` — `c.Param("id")` + `strconv.ParseInt`，not found 时 `response.NotFound`
    - `Create(c *gin.Context)` — `c.ShouldBindJSON` 绑定 body（含 `binding:"required"` tag 校验 name/embed_url），`svc.Create`，`response.Success`
    - `Update(c *gin.Context)` — 同上，`svc.Update`
    - `Delete(c *gin.Context)` — `svc.Delete`，`response.Success`
    - `Reorder(c *gin.Context)` — 绑定 `{items: [{id, sort_order}]}`，`svc.Reorder`
  - 错误处理用 `response.FailFromError(c, err)` 统一，not found 显式判断后 `response.NotFound`
- [ ] 在 `scheduler/cmd/app.go` 注入 service：
  - App 结构体追加字段 `grafanaSvc *service.GrafanaDashboardService`（参考 `app.go:143` webhook 字段位置）
  - NewApp 函数中初始化（参考 `app.go:389-391`）：
    ```go
    grafanaSvc := service.NewGrafanaDashboardService(logDB)
    app.grafanaSvc = grafanaSvc
    ```
- [ ] 在 `scheduler/cmd/routes.go` 注册路由（参考 `routes.go:176-184` webhook 模式）：
  ```go
  grafanaHandler := handler.NewGrafanaDashboardHandler(app.grafanaSvc)
  // 公共查询（登录即可，所有用户都能看 dashboard）
  protected.GET("/grafana-dashboards", grafanaHandler.ListEnabled)
  // 管理接口（仅 system_admin）
  adminGroup := protected.Group("/admin/grafana-dashboards")
  adminGroup.Use(middleware.RequireSystemAdmin(app.permissionService))
  {
      adminGroup.GET("", grafanaHandler.ListAll)
      adminGroup.GET("/:id", grafanaHandler.GetByID)
      adminGroup.POST("", grafanaHandler.Create)
      adminGroup.PUT("/:id", grafanaHandler.Update)
      adminGroup.DELETE("/:id", grafanaHandler.Delete)
      adminGroup.PUT("/reorder", grafanaHandler.Reorder)
  }
  ```
  - 注意：`reorder` 路由需在 `:id` 之前注册或用明确路径避免冲突（rqlite/gin 路由优先级）
- [ ] 在 `scheduler/internal/middleware/audit.go` 路由映射追加：
  ```go
  "/api/admin/grafana-dashboards": {Resource: "grafana_dashboard", Action: "manage"},
  ```
- [ ] 创建 `scheduler/internal/handler/grafana_dashboard_test.go`：
  - 测试路由权限（未登录 401、普通用户 403、system_admin 200）
  - 测试参数校验（缺 name/embed_url 返回 400）
  - 测试正常 CRUD 流程

**Verification**:
- `cd scheduler && go build ./...` 通过
- `cd scheduler && go test ./internal/handler/ -run GrafanaDashboard -v` 通过
- 启动 scheduler，用 curl 验证：
  - 未登录访问 `/api/grafana-dashboards` 返回 401
  - 登录后访问 `/api/grafana-dashboards` 返回 200 + 空数组
  - system_admin 创建图表后列表可见

**Done when**: API 全部可访问，权限和审计正确，handler 测试通过。

---

### Phase 4: 前端类型与 API 封装

**Goal**: 定义前端类型，封装 API 调用。

**Tasks**:
- [ ] 在 `web/src/types/index.ts` 追加类型定义（参考 `Webhook` 接口风格，`types/index.ts:176-191`）：
  ```typescript
  export interface GrafanaDashboard {
    id: number
    name: string
    description: string
    embed_url: string
    width: 'third' | 'half' | 'full'
    height_px: number
    sort_order: number
    enabled: number  // 后端用 int，前端保持一致
    created_at: string
    updated_at: string
  }

  export interface GrafanaDashboardInput {
    name: string
    description?: string
    embed_url: string
    width: 'third' | 'half' | 'full'
    height_px: number
    sort_order: number
    enabled: number
  }

  export interface GrafanaReorderItem {
    id: number
    sort_order: number
  }
  ```
  - 注意：`enabled` 用 `number` 与后端 model `int` 对齐（spec §2 决策）
- [ ] 创建 `web/src/api/grafana.ts`（参考 `api/webhook.ts` 模式）：
  ```typescript
  import api from '@/utils/api'
  import type { GrafanaDashboard, GrafanaDashboardInput, GrafanaReorderItem } from '@/types'

  export const grafanaAPI = {
    listEnabled: () => api.get<GrafanaDashboard[]>('/grafana-dashboards'),
    listAll: () => api.get<GrafanaDashboard[]>('/admin/grafana-dashboards'),
    get: (id: number) => api.get<GrafanaDashboard>(`/admin/grafana-dashboards/${id}`),
    create: (data: GrafanaDashboardInput) => api.post<GrafanaDashboard>('/admin/grafana-dashboards', data),
    update: (id: number, data: GrafanaDashboardInput) => api.put<GrafanaDashboard>(`/admin/grafana-dashboards/${id}`, data),
    delete: (id: number) => api.delete(`/admin/grafana-dashboards/${id}`),
    reorder: (items: GrafanaReorderItem[]) =>
      api.put('/admin/grafana-dashboards/reorder', { items }),
  }
  ```
- [ ] 在 `web/src/api/index.ts` 追加 re-export：
  - `import { grafanaAPI } from './grafana'`（参考 `api/index.ts:7`）
  - `export { ..., grafanaAPI }`（参考 `api/index.ts:11`）
  - 类型 re-export：`export type { ..., GrafanaDashboard, GrafanaDashboardInput, GrafanaReorderItem }`

**Verification**:
- `cd web && npm run build` 通过（TypeScript 编译无错误）
- `cd web && npm run lint` 无新增错误

**Done when**: 类型可被其他文件导入，API 封装可编译。

---

### Phase 5: 前端管理页

**Goal**: 实现独立管理页，支持 CRUD、启停切换、批量排序保存。

**Tasks**:
- [ ] 创建 `web/src/views/admin/GrafanaDashboards.vue`（参考 `WebhookManagement.vue` 整体结构）：
  - **工具栏**：标题 + 搜索框 + "新建图表"按钮 + "保存排序"按钮
  - **表格**（`el-table`，列）：
    - 排序号（`el-input-number`，行内可编辑，绑定到 `row.sort_order`）
    - 名称
    - URL（截断显示 origin + path，隐藏 query string；`el-tooltip` 同样截断）
    - 宽度（`el-tag`：1/3、1/2、整行）
    - 高度（px）
    - 状态（`el-switch` 绑定 `row.enabled`，`@change` 调用 `handleToggleEnabled`）
    - 操作（编辑、删除按钮）
  - **编辑对话框**（`el-dialog`，新建/编辑复用）：
    - 顶部 `el-alert`（type=warning）：cookie/token 模式安全提示文案
    - `el-form` + `el-form-item`：
      - 名称（`el-input`，required）
      - 描述（`el-input` textarea，可选）
      - 嵌入 URL（`el-input` textarea，required，placeholder 提示粘贴 Grafana 分享链接）
      - 宽度（`el-radio-group`：1/3、1/2、整行）
      - 高度（`el-input-number` min=120 max=1200 step=10）
      - 排序号（`el-input-number` min=0）
      - 启用（`el-switch`）
  - **script setup**：
    - 导入 `grafanaAPI`、`isHandledError`、`ElMessage`、`ElMessageBox`
    - 状态：`list`、`loading`、`dialogVisible`、`isEditing`、`form`、`submitting`
    - `loadList`：调用 `grafanaAPI.listAll`，try/catch/finally 模式
    - `handleCreate`/`handleEdit`：打开对话框，初始化 form
    - `handleSubmit`：手动校验 name/embed_url（参考 WebhookManagement 简化模式），调用 create/update，成功关闭对话框 + 重新加载
    - `handleDelete`：`ElMessageBox.confirm` + `grafanaAPI.delete`
    - `handleToggleEnabled`：调用 `grafanaAPI.update(id, {...row, enabled: newVal})`，乐观更新
    - `handleSaveSortOrder`：收集 `list.map(item => ({id, sort_order}))`，调用 `grafanaAPI.reorder`
    - URL 截断工具函数：`truncateUrl(url)` 用 `new URL(url)` 解析，返回 `origin + pathname`（不返回 search）
- [ ] 在 `web/src/router/index.ts` 追加路由（参考 `router/index.ts:148-154`）：
  ```typescript
  {
    path: 'admin/grafana-dashboards',
    name: 'GrafanaDashboards',
    component: () => import('@/views/admin/GrafanaDashboards.vue'),
  },
  ```
  - 不设 `meta.menuPermission`（spec §5.1 决策，由后端 RequireSystemAdmin 控制）
- [ ] 在 `web/src/config/menuPermissionMap.ts` 的 `system-admin.children` 追加（参考 `menuPermissionMap.ts:141-147` webhook 项）：
  ```typescript
  {
    key: 'grafana-dashboard-management',
    label: 'Grafana 图表',
    icon: 'DataLine',
    path: '/admin/grafana-dashboards',
    resources: ['config'],
  },
  ```
- [ ] 在 `web/src/views/Layout.vue` 的 `pageTitle` 映射追加 `'/admin/grafana-dashboards': 'Grafana 图表'`

**Verification**:
- `cd web && npm run build` 通过
- `cd web && npm run lint` 无新增错误
- 浏览器访问 `/admin/grafana-dashboards`：
  - system_admin 可见页面和菜单项
  - 新建图表 → 列表显示 → 编辑 → 删除 全流程可用
  - el-switch 切换启停即时生效
  - 修改排序号 + 保存排序 → 顺序变化

**Done when**: 管理页全功能可用，菜单和路由正确。

---

### Phase 6: Dashboard 嵌入展示

**Goal**: 在仪表盘渲染已启用的 Grafana 图表。

**Tasks**:
- [ ] 修改 `web/src/views/Dashboard.vue`：
  - 导入 `grafanaAPI`、`DataLine`/`Refresh`/`Outer` 图标
  - 新增状态：`const grafanaDashboards = ref<GrafanaDashboard[]>([])`
  - 新增方法：
    - `loadGrafanaDashboards`：调用 `grafanaAPI.listEnabled`，失败 `console.warn` 静默处理
    - `openGrafanaPanel(item)`：`window.open(item.embed_url, '_blank', 'noopener')`
  - 在 `onMounted` 中调用 `loadGrafanaDashboards`（与现有 `refreshData` 并列，不加入 30 秒定时刷新）
  - 在模板 `trend-section` 之后（约第 219 行后）追加 `grafana-section`（模板结构见 spec §6.2）
  - 追加 CSS（见 spec §6.3，响应式 3 列网格）
- [ ] 确保未配置图表时 `grafanaDashboards.length === 0`，section 不渲染（与当前 Dashboard 表现一致）

**Verification**:
- `cd web && npm run build` 通过
- 浏览器访问 `/`（仪表盘）：
  - 未配置图表时：grafana-section 不显示，原有 5 个 section 正常
  - 配置 1 个图表：显示 1 个 iframe，加载 Grafana URL
  - 配置多个不同宽度图表：网格布局正确（third/half/full）
  - 点击"刷新图表"按钮：重新拉取列表
  - 点击"新窗口打开"图标：在新标签页打开 Grafana URL
  - iframe 加载失败（如 Grafana 不可达）：不阻断其他 section 展示

**Done when**: 仪表盘正确嵌入 Grafana 图表，响应式布局，无图表时向下兼容。

---

### Phase 7: 测试与文档收尾

**Goal**: 补充测试，更新文档。

**Tasks**:
- [ ] 后端测试补全：
  - `service/grafana_dashboard_test.go`：validate 全分支 + Reorder 批量逻辑
  - `handler/grafana_dashboard_test.go`：权限 + 参数校验 + CRUD
  - 运行 `cd scheduler && go test ./internal/... -v -run Grafana` 全部通过
- [ ] 前端手动测试清单：
  - 管理页 CRUD 全流程
  - 启停切换即时生效
  - 批量排序保存
  - Dashboard 嵌入渲染（含空数据、单图、多图、宽度差异）
  - URL 截断显示（cookie 模式 query string 不泄露到表格）
- [ ] 文档更新：
  - `docs/GUIDE.md`：新增"Grafana 图表嵌入"章节，含配置步骤、两种认证方式说明、安全提示
  - 检查 `AGENTS.md` 是否需更新（预计无需）

**Verification**:
- `cd scheduler && go test ./internal/... -v` 全部通过，覆盖率 ≥ 60%
- `cd web && npm run build && npm run lint` 通过
- 文档 review 完成

**Done when**: 测试全绿，文档更新完成。

## Risks & Open Questions

1. **Grafana iframe 跨域限制**：某些 Grafana 实例可能通过 `X-Frame-Options` 或 CSP 禁止被嵌入。需在文档中提示用户在 Grafana 侧配置 `allow_embedding = true`。无法在 BDopsFlow 侧解决。
2. **cookie 模式 URL 泄露风险**：`listEnabled` 返回完整 URL（含 token），任何登录用户可通过浏览器开发者工具看到。spec §7.2 已接受此风险，文档需明确提示使用最小权限只读 Token。
3. **rqlite 事务语义**：`WriteParameterized` 多语句在 rqlite 中是原子的（单次 Raft 提交），但如需更强保证可改用 `BEGIN TRANSACTION`/`COMMIT` 显式事务。当前项目其他批量操作（如 `api_test_service.go:95-121`）直接用 `WriteParameterized`，本计划沿用。
4. **路由优先级**：`/admin/grafana-dashboards/reorder` 与 `/admin/grafana-dashboards/:id` 可能冲突。gin 的路由树会优先匹配静态路径，但需测试验证。如有问题，将 reorder 改为 POST `/admin/grafana-dashboards/reorder` 或调整注册顺序。
5. **前端 `enabled` 字段类型**：后端用 `int`（0/1），前端 `el-switch` 默认绑定 boolean。需在 `el-switch` 上用 `:active-value="1" :inactive-value="0"` 适配，或在 API 层转换。

## Definition of Done

- 所有 7 个 Phase 完成
- `cd scheduler && go build ./... && go test ./internal/... -v` 通过
- `cd web && npm run build && npm run lint` 通过
- spec 验收标准全部满足：
  - 管理员可配置多个 Grafana 实例的多个图表
  - 每个图表可配置宽度、高度、排序、启停
  - 仪表盘自动渲染已启用图表，响应式布局
  - 支持匿名和 URL 带 cookie 两种认证
  - 审计日志记录所有写操作
- `docs/GUIDE.md` 更新
- 迁移脚本 `v8_grafana_dashboards.sql` 和 `schema.sql` 同步
