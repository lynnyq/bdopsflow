# Grafana 图表嵌入仪表盘 - 设计文档

- **日期**：2026-07-30
- **状态**：待实施
- **作者**：BDopsFlow Team

## 1. 背景与目标

当前仪表盘（`web/src/views/Dashboard.vue`）展示任务统计、健康状态、执行趋势等内置指标，使用纯 CSS 模拟的 7 天趋势条形图。运维团队已使用 Grafana 进行深度监控，希望在 BDopsFlow 仪表盘直接嵌入 Grafana 图表，无需跳转即可查看关键监控。

**目标**：
- 管理员可配置多个 Grafana 实例的多个图表
- 每个图表可配置宽度（1/3、1/2、整行）、高度、排序、启停
- 仪表盘自动渲染已启用的图表，响应式布局
- 支持匿名嵌入和 URL 带 cookie/token 参数嵌入两种认证方式

**非目标**：
- 不做 Grafana API 集成（不调 Grafana 查询接口、不解析 panel 数据）
- 不做图表数据代理（iframe 直接加载 Grafana URL）
- 不做多租户隔离（Grafana 图表配置为系统级资源）
- 不做拖拽排序（行内编辑数字 + 批量保存）

## 2. 数据模型

新建表 `bdopsflow_grafana_dashboards`，迁移脚本 `deploy/migrations/v8_grafana_dashboards.sql`（紧接现有 v7）：

```sql
CREATE TABLE IF NOT EXISTS bdopsflow_grafana_dashboards (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT    NOT NULL,
    description TEXT,
    embed_url   TEXT    NOT NULL,
    width       TEXT    NOT NULL DEFAULT 'full',
    height_px   INTEGER NOT NULL DEFAULT 300,
    sort_order  INTEGER NOT NULL DEFAULT 0,
    enabled     INTEGER NOT NULL DEFAULT 1,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_grafana_dashboards_enabled_sort
    ON bdopsflow_grafana_dashboards(enabled, sort_order);
```

同时更新 `deploy/schema.sql`，在系统表区域追加相同 DDL，保持 schema.sql 为全量权威源。

**字段说明**：
- `embed_url`：完整的 Grafana 分享链接。匿名模式直接是面板 URL（建议带 `?kiosk` 参数）；cookie 模式带 `auth_token` 等查询参数
- `width`：枚举 `third` / `half` / `full`，对应网格跨 1/2/3 列
- `height_px`：iframe 高度，范围 [120, 1200]
- `sort_order`：升序排列，越小越靠前
- `enabled`：0=禁用，1=启用

**Go model**（`scheduler/internal/model/grafana_dashboard.go`）字段与表一一对应，`Enabled` 用 `int` 类型保持与 schema 一致。

**设计决策**：
- 不存 Grafana 实例 base_url：匿名和 cookie 模式下 URL 均已含完整 origin 和参数，存 base_url 反而冗余
- 不做软删除：与项目其他业务表（如 webhook）一致
- 不加 `domain_id`：系统级展示资源，由 `RequireSystemAdmin` 权限控制管理

## 3. 后端 API

### 3.1 REST 路由

| 方法 | 路径 | 功能 | 权限 |
|------|------|------|------|
| GET | `/api/grafana-dashboards` | 列出启用图表（按 sort_order 升序） | 登录即可 |
| GET | `/api/admin/grafana-dashboards` | 列出全部图表（含禁用） | `RequireSystemAdmin` |
| GET | `/api/admin/grafana-dashboards/:id` | 获取单个 | `RequireSystemAdmin` |
| POST | `/api/admin/grafana-dashboards` | 新建 | `RequireSystemAdmin` |
| PUT | `/api/admin/grafana-dashboards/:id` | 更新 | `RequireSystemAdmin` |
| DELETE | `/api/admin/grafana-dashboards/:id` | 删除 | `RequireSystemAdmin` |
| PUT | `/api/admin/grafana-dashboards/reorder` | 批量更新 sort_order | `RequireSystemAdmin` |

### 3.2 文件分布

遵循项目分层：
- `scheduler/internal/model/grafana_dashboard.go` — model 定义
- `scheduler/internal/grafana_dashboard/service.go` — 业务逻辑、校验
- `scheduler/internal/grafana_dashboard/repository.go` — DB 访问
- `scheduler/internal/handler/grafana_dashboard.go` — HTTP handler
- `scheduler/cmd/routes.go` — 注册路由（参考现有 admin 路由组）

### 3.3 校验规则（service 层）

- `name` 非空，≤ 100 字符
- `embed_url` 非空，必须匹配 `^https?://`，≤ 2048 字符
- `width` ∈ {`third`, `half`, `full`}
- `height_px` ∈ [120, 1200]
- `sort_order` ≥ 0

### 3.4 reorder 接口

请求体：
```json
{ "items": [{ "id": 1, "sort_order": 0 }, { "id": 2, "sort_order": 1 }] }
```
在事务中批量 UPDATE，避免逐个请求的竞态。

### 3.5 审计日志

在 `scheduler/internal/middleware/audit.go` 的路由映射中追加：
```go
"/api/admin/grafana-dashboards": {Resource: "grafana_dashboard", Action: "manage"},
```

### 3.6 响应格式

遵循项目统一响应体（`{code, message, data, trace_id}`），data 为单个对象或数组。

## 4. 前端 API 封装与类型

### 4.1 类型定义

追加到 `web/src/types/`（按现有组织方式，建议新建 `grafana.ts` 或追加到 index）：

```typescript
export interface GrafanaDashboard {
  id: number
  name: string
  description: string
  embed_url: string
  width: 'third' | 'half' | 'full'
  height_px: number
  sort_order: number
  enabled: boolean
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
  enabled: boolean
}
```

### 4.2 API 封装

新建 `web/src/api/grafanaDashboard.ts`：

```typescript
export const grafanaDashboardAPI = {
  listEnabled: () => api.get<GrafanaDashboard[]>('/grafana-dashboards'),
  listAll: () => api.get<GrafanaDashboard[]>('/admin/grafana-dashboards'),
  get: (id: number) => api.get<GrafanaDashboard>(`/admin/grafana-dashboards/${id}`),
  create: (data: GrafanaDashboardInput) => api.post<GrafanaDashboard>('/admin/grafana-dashboards', data),
  update: (id: number, data: GrafanaDashboardInput) => api.put<GrafanaDashboard>(`/admin/grafana-dashboards/${id}`, data),
  delete: (id: number) => api.delete(`/admin/grafana-dashboards/${id}`),
  reorder: (items: { id: number; sort_order: number }[]) =>
    api.put('/admin/grafana-dashboards/reorder', { items }),
}
```

在 `web/src/api/index.ts` 中 re-export。

**决策**：`listEnabled` 与 `listAll` 分两个接口，避免禁用图表意外暴露给普通用户。

## 5. 前端管理页

### 5.1 路由与菜单

- 路由 `/admin/grafana-dashboards`，组件 `web/src/views/admin/GrafanaDashboards.vue`
- 在 `router/index.ts` 的 Layout children 中追加路由，**不设** `meta.menuPermission`（管理页访问由后端 `RequireSystemAdmin` 控制；前端菜单可见性通过下方 `menuPermissionMap` 的 `resources: ['config']` 推导，仅 system_admin 可见该菜单项）
- 在 `menuPermissionMap.ts` 的 `system-admin` children 中追加：
  ```typescript
  {
    key: 'grafana-dashboard-management',
    label: 'Grafana 图表',
    icon: 'DataLine',
    path: '/admin/grafana-dashboards',
    resources: ['config'],
  },
  ```
- 在 `Layout.vue` 的 `pageTitle` 映射追加 `'/admin/grafana-dashboards': 'Grafana 图表'`

### 5.2 页面结构

参考 `WebhookManagement.vue` 的列表+对话框模式：

1. **顶部工具栏**：标题 + "新建图表"按钮
2. **列表表格**（`el-table`）：
   - 排序号（`sort_order`，行内可编辑数字）
   - 名称
   - URL（截断显示，hover tooltip 显示 origin + path，隐藏 query string）
   - 宽度（`el-tag`：1/3、1/2、整行）
   - 高度（px）
   - 状态（`el-switch` 直接切换 `enabled`，即时 PUT 保存）
   - 操作（编辑、删除）
3. **编辑对话框**（`el-dialog`，新建/编辑复用）：
   - 顶部 `el-alert`（warning）：cookie/token 模式安全提示
   - 名称（必填）
   - 描述（可选，textarea）
   - 嵌入 URL（必填，textarea 方便粘贴长链接）
   - 宽度（`el-radio-group`：1/3、1/2、整行）
   - 高度（`el-input-number`，min=120 max=1200 step=10）
   - 排序号（`el-input-number`，min=0）
   - 启用（`el-switch`）
4. **排序操作**：行内编辑 `sort_order` 数字后，点击工具栏"保存排序"按钮调用 `reorder` 批量更新

### 5.3 前端校验

`el-form` rules：
- name 必填
- embed_url 必填 + 正则 `^https?://`
- height_px ∈ [120, 1200]

## 6. Dashboard 嵌入展示

### 6.1 位置

在 `Dashboard.vue` 的 `trend-section` 之后新增 `grafana-section`，仅当 `grafanaDashboards.length > 0` 时渲染。

### 6.2 模板结构

```vue
<section v-if="grafanaDashboards.length > 0" class="grafana-section">
  <div class="section-header">
    <h2 class="section-title">
      <el-icon><DataLine /></el-icon>
      Grafana 监控图表
    </h2>
    <el-tooltip content="刷新图表" placement="top">
      <el-button :icon="Refresh" circle size="small" @click="loadGrafanaDashboards" />
    </el-tooltip>
  </div>
  <div class="grafana-grid">
    <div
      v-for="item in grafanaDashboards"
      :key="item.id"
      class="grafana-panel"
      :class="`grafana-width-${item.width}`"
    >
      <div class="grafana-panel-header">
        <span class="grafana-panel-title" :title="item.name">{{ item.name }}</span>
        <el-tooltip content="在新窗口打开" placement="top">
          <el-icon class="grafana-open-icon" @click="openGrafanaPanel(item)">
            <Outer />
          </el-icon>
        </el-tooltip>
      </div>
      <iframe
        :src="item.embed_url"
        :style="{ height: item.height_px + 'px' }"
        class="grafana-iframe"
        sandbox="allow-scripts allow-same-origin allow-popups allow-forms"
        loading="lazy"
        referrerpolicy="no-referrer"
      />
    </div>
  </div>
</section>
```

### 6.3 布局 CSS

响应式 3 列网格：
- 默认 3 列，`third` 跨 1 列，`half` 跨 2 列（桌面端），`full` 跨 3 列
- ≤ 768px 单列堆叠
```css
.grafana-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: var(--space-4);
}
.grafana-width-third { grid-column: span 1; }
.grafana-width-half  { grid-column: span 1; }
.grafana-width-full  { grid-column: span 3; }
@media (min-width: 992px) {
  .grafana-width-half { grid-column: span 2; }
}
@media (max-width: 768px) {
  .grafana-grid { grid-template-columns: 1fr; }
  .grafana-width-third, .grafana-width-half { grid-column: span 1; }
}
```

### 6.4 数据加载

- `onMounted` 调用 `grafanaDashboardAPI.listEnabled()`，存入 `grafanaDashboards` ref
- **不**加入 30 秒定时刷新：iframe 自身按 Grafana 配置刷新，重载 URL 会闪烁
- 用户点击"刷新图表"按钮时重新拉取列表
- `loadGrafanaDashboards` 失败时静默处理（`console.warn`），不阻断其他统计展示

### 6.5 iframe 属性决策

- `sandbox="allow-scripts allow-same-origin allow-popups allow-forms"`：Grafana 面板需脚本渲染；cookie 模式需 same-origin 传递认证
- **不**放开 `allow-top-navigation`：防止 Grafana 页面劫持顶层窗口
- `loading="lazy"`：视口内才加载，避免首屏卡顿
- `referrerpolicy="no-referrer"`：不向 Grafana 暴露 BDopsFlow 站点信息

## 7. 安全与边界处理

### 7.1 URL 安全校验

- **协议白名单**：`embed_url` 必须匹配 `^https?://`（service 层正则 + 前端 `el-form` rule 双层），拒绝 `javascript:`、`data:`、`file:`
- **长度限制**：`embed_url` ≤ 2048 字符
- **XSS 防护**：`name`/`description` 存储不转义，前端 Vue 模板默认转义渲染，禁止 `v-html`

### 7.2 凭据泄露防护

- 管理页表格 URL 列**截断显示**（仅 origin + path，隐藏 query string），tooltip 同样截断
- 完整 URL 仅在编辑对话框可见可改
- `listEnabled` 返回给普通用户的 URL **原样返回**（iframe 加载需要完整 URL，无法脱敏）——这是 cookie 模式的固有风险
- 管理页对话框顶部 `el-alert` 提示：使用最小权限只读 Token 并定期轮换

### 7.3 审计

- 所有 Grafana 图表增删改操作记录审计日志（resource=`grafana_dashboard`，action=`manage`）
- 在 `middleware/audit.go` 路由映射中追加

### 7.4 性能

- `listEnabled` 返回数据量小（通常 < 20），无需分页
- 后端不代理 Grafana 流量，无额外性能负担
- 不做服务端缓存（QPS 极低，直接查 DB）

### 7.5 错误处理

- `listEnabled` 失败 → Dashboard 静默不渲染 grafana-section
- iframe 加载失败 → Grafana 自身显示错误页，BDopsFlow 不拦截
- 管理页 CRUD 失败 → `ElMessage.error`，复用 `isHandledError` 工具

## 8. 测试策略

### 8.1 后端单元测试

- `service_test.go`：校验规则（URL 协议、长度、width 枚举、height 范围、sort_order 非负）
- `repository_test.go`：CRUD + reorder 事务
- `handler_test.go`：路由权限（未登录/admin/普通用户）、参数校验、审计日志触发

### 8.2 前端

- 管理页：表单校验、列表渲染、启停切换、批量排序保存
- Dashboard：空数据不渲染 section、有数据渲染 iframe、加载失败静默

## 9. 文档更新

- `deploy/schema.sql`：追加 `bdopsflow_grafana_dashboards` 表 DDL
- `deploy/migrations/v8_grafana_dashboards.sql`：新建迁移脚本
- `docs/GUIDE.md`：新增"Grafana 图表嵌入"使用说明章节
- `docs/CONFIGURATION.md`：无需更新（未使用 system_config）
- `AGENTS.md`：无需更新

## 10. 向下兼容性

- 新增独立表和路由，不修改现有任何表结构或 API
- Dashboard.vue 新增 section 不影响原有 5 个 section 的渲染逻辑
- 管理页为独立路由，不影响现有菜单功能
- 未配置任何 Grafana 图表时，Dashboard 表现与当前完全一致（grafana-section 不渲染）
