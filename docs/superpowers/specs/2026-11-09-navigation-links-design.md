# 导航快捷链接 (Navigation Links) 功能设计

> 日期：2026-11-09
> 作者：brainstorming session
> 关联：Grafana Dashboard 嵌入模式（复用其 CRUD + Dashboard 内嵌的完整模式）

---

## 1. 需求概述

在 BDopsFlow 系统中新增"导航快捷链接"功能，管理员可在系统配置中维护一组外部网站导航，所有登录用户在 Dashboard 首页看到一个"快捷导航"卡片区域，点击任意卡片在**新标签页**打开对应网站。

核心特性：
- 完全复用 Grafana Dashboard 的管理 + Dashboard 内嵌模式（后端 CRUD + 前端 Dashboard 展示）
- 每条导航支持可配置的宽度（占 Dashboard 网格列数）、密度/高度（三档基准 + 像素微调）、排序、启用禁用
- 创建/更新时后端**自动抓取目标网站的 favicon** 存储为 base64，前端零跨域依赖展示
- favicon 获取失败时前端渲染 fallback：域名首字母 + 自动取色方块
- Dashboard 导航缓存 5 分钟刷新一次，避免频繁请求

---

## 2. 架构总览

```
                    ┌────────────────────────────┐
                    │     Dashboard.vue (首页)     │
                    │  ┌──────────────────────┐  │
    GET /navigation- │  │  快捷导航 section    │  │
    links (公开)    ▶ │  │  3列网格 高度可选   │  │
                    │  └──────────────────────┘  │
                    └────────────────────────────┘
                             ▲
                             │ 并行加载
                    ┌────────┴────────┐
                    │ NavigationLink  │
                    │   Service + DB   │
                    └─────────────────┘
                             ▲
                    ┌────────┴─────────┐
                    │ NavigationLinks  │
                    │ .vue (管理页)    │
                    └──────────────────┘
                             ▲
                             │ CRUD + reorder
                    /admin/navigation-links/*
                             ▲
                    ┌────────┴────────┐
                    │  Permission:    │
                    │  config (admin) │
                    └─────────────────┘

                    ┌────────────────────┐
                    │ icon_fetcher.go    │
                    │ 创建/更新时异步触发│
                    │ 抓取 favicon       │
                    │ 存 base64 data URL │
                    └────────────────────┘
```

---

## 3. 后端设计

### 3.1 数据库 Schema

**新表 `navigation_links`**，对齐 `grafana_dashboards` 字段风格：

```sql
CREATE TABLE navigation_links (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT    NOT NULL,
    description     TEXT,
    url             TEXT    NOT NULL,
    width           TEXT    NOT NULL DEFAULT 'third',    -- 'third' | 'half' | 'full'
    display_mode    TEXT    NOT NULL DEFAULT 'normal',   -- 'compact' | 'normal' | 'banner'
    height_px       INTEGER,                              -- NULL 使用 display_mode 基准值
    icon_data       TEXT,                                 -- favicon base64 data: URL (可空)
    icon_fetched_at TEXT,                                 -- ISO 时间，最近抓取时间 (可空)
    sort_order      INTEGER NOT NULL DEFAULT 0,
    enabled         INTEGER NOT NULL DEFAULT 1,          -- 0=禁用 1=启用
    created_at      TEXT    NOT NULL,
    updated_at      TEXT    NOT NULL
);

CREATE INDEX idx_nav_links_enabled_sort ON navigation_links(enabled, sort_order);
```

同时更新 `deploy/schema.sql` 主 schema 文件，并创建增量迁移脚本 `deploy/migrations/v{version}_navigation_links.sql`（存量升级从 migration 走，新部署从 schema.sql 走）。

### 3.2 display_mode 基准高度定义

| display_mode | 基准高度 | 适用场景 |
|---|---|---|
| compact | 52px | 一屏想塞多项目 |
| normal | 68px | 默认，信息密度均衡 |
| banner | 100px | 突出展示，会显示 description |

`height_px` 有值时直接覆盖基准值，前端 CSS `height: height_px ?? 基准值`。范围限制 40–200px。

### 3.3 API 路由

**公开接口**（登录即可访问，Dashboard 首页使用）：

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/navigation-links` | 登录即可 | 返回 enabled=1 列表，按 sort_order 升序 |

**管理接口**（需 admin / system_admin 权限，复用 `config` resource）：

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/admin/navigation-links` | config | 返回全部含启用禁用状态 |
| GET | `/admin/navigation-links/:id` | config | 单条详情 |
| POST | `/admin/navigation-links` | config | 创建，异步触发 favicon 抓取 |
| PUT | `/admin/navigation-links/:id` | config | 更新，URL 变更时自动重新抓 icon |
| DELETE | `/admin/navigation-links/:id` | config | 删除 |
| PUT | `/admin/navigation-links/reorder` | config | 批量更新 sort_order |
| POST | `/admin/navigation-links/:id/refresh-icon` | config | 手动重新抓取 favicon |

### 3.4 Handler 层 (`internal/handler/navigation_link.go`)

完全复用 `GrafanaDashboardHandler` 的结构：

- `NavigationLinkHandler` struct，持有 `*service.NavigationLinkService`
- `ListEnabled` — 公开接口，返回 enabled=1 的 items 数组
- `ListAll` — 管理接口，返回全部
- `GetByID` / `Create` / `Update` / `Delete` / `Reorder`
- `RefreshIcon` — 手动重新抓取，同步调用 icon_fetcher，返回更新后的记录

**请求校验**：
- URL 必须匹配 `^https?://`
- width 必须在 `{third, half, full}`
- display_mode 必须在 `{compact, normal, banner}`
- height_px 如提供，必须在 40–200 范围
- sort_order ≥ 0

### 3.5 Service 层 (`internal/service/navigation_link.go`)

核心职责：
- CRUD 操作（使用项目现有 DB 访问封装）
- 创建/更新 URL 变更时，异步启动 goroutine 调用 icon_fetcher
- 返回统一的 service error（not found 等），handler 层统一处理

### 3.6 Icon Fetcher 模块（`pkg/iconfetch/fetcher.go`）

**独立模块**，可单元测试，不依赖其他业务模块。

#### 抓取策略

按以下顺序尝试，取第一个成功的结果：

1. **并发**请求目标 origin 的 `/favicon.ico` 和 `/` HTML 页面
2. HTML 页面正则匹配（优先级从高到低）：
   - `<link rel="icon" sizes="32x32" href="...">`
   - `<link rel="icon" href="...">`
   - `<link rel="shortcut icon" href="...">`
   - `<link rel="apple-touch-icon" href="...">`
3. 回退到 `/favicon.ico` 直取

#### 容错与限制

- 单次 HTTP 请求超时 **5 秒**
- HTTPS 证书校验：**跳过**（内网环境自签证书常见）
- 响应体大小限制 **20KB**
- 允许格式：svg / png / jpeg / ico（WebP 跳过，浏览器兼容性好但数据量不稳定）
- 遇到 403/反爬 → 放弃，记 `slog.Warn`，icon_data 留 NULL

#### 输出

```go
type FetchResult struct {
    IconData string    // data URL: "data:image/svg+xml;base64,PHN2Zy..."
    FetchedAt time.Time
    Source   string    // "link-tag" | "favicon-ico" | "manual"
    Error    error     // nil 表示成功
}
```

#### 异步触发

```go
// service/navigation_link.go 中
func (s *NavigationLinkService) Create(ctx context.Context, d *model.NavigationLink) (*model.NavigationLink, error) {
    // ... DB insert ...
    
    // 异步抓 favicon，不阻塞主流程
    go func() {
        result := iconfetch.Fetch(context.Background(), d.URL)
        if result.Error == nil {
            s.UpdateIcon(context.Background(), d.ID, result.IconData, result.FetchedAt)
        }
    }()
    
    return d, nil
}
```

#### 单测覆盖

- 正常 HTML 页面含 `<link rel="icon">` → 提取成功
- 只有 `/favicon.ico` 可用 → 回退成功
- 404 / 超时 / 证书错误 → 返回 Error
- 大图标（>20KB）→ 截断或跳过
- SVG → 正确识别并转 data URL
- 相对路径 href → 正确拼完整 URL

### 3.7 路由注册

在路由注册处新增两组：

```go
// 公开（登录即可）
public.GET("/navigation-links", navHandler.ListEnabled)

// 管理（需 admin）
admin := r.Group("/admin", middleware.RequireRole("admin", "system_admin"))
{
    admin.GET("/navigation-links", navHandler.ListAll)
    admin.GET("/navigation-links/:id", navHandler.GetByID)
    admin.POST("/navigation-links", navHandler.Create)
    admin.PUT("/navigation-links/:id", navHandler.Update)
    admin.DELETE("/navigation-links/:id", navHandler.Delete)
    admin.PUT("/navigation-links/reorder", navHandler.Reorder)
    admin.POST("/navigation-links/:id/refresh-icon", navHandler.RefreshIcon)
}
```

---

## 4. 前端设计

### 4.1 类型定义（types/index.ts）

```typescript
export interface NavigationLink {
  id: number
  name: string
  description: string
  url: string
  width: 'third' | 'half' | 'full'
  display_mode: 'compact' | 'normal' | 'banner'
  height_px: number | null
  icon_data: string | null
  icon_fetched_at: string | null
  sort_order: number
  enabled: number
  created_at: string
  updated_at: string
}

export interface NavigationLinkInput {
  name: string
  description?: string
  url: string
  width: 'third' | 'half' | 'full'
  display_mode: 'compact' | 'normal' | 'banner'
  height_px?: number | null
  sort_order: number
  enabled: number
}

export interface NavigationReorderItem {
  id: number
  sort_order: number
}
```

### 4.2 API 封装（api/navigation.ts）

和 `grafana.ts` 完全对齐的 7 个方法：

```typescript
export const navigationAPI = {
  listEnabled: () => api.get<{ items: NavigationLink[] }>('/navigation-links'),
  listAll: () => api.get<{ items: NavigationLink[] }>('/admin/navigation-links'),
  get: (id: number) => api.get<NavigationLink>(`/admin/navigation-links/${id}`),
  create: (data: NavigationLinkInput) => api.post<NavigationLink>('/admin/navigation-links', data),
  update: (id: number, data: NavigationLinkInput) => api.put(`/admin/navigation-links/${id}`, data),
  delete: (id: number) => api.delete(`/admin/navigation-links/${id}`),
  reorder: (items: NavigationReorderItem[]) =>
    api.put('/admin/navigation-links/reorder', { items }),
  refreshIcon: (id: number) => api.post<NavigationLink>(`/admin/navigation-links/${id}/refresh-icon`),
}
```

### 4.3 菜单配置（config/menuPermissionMap.ts）

在 `system-admin` 父菜单 children 中新增：

```typescript
{
  key: 'navigation-link-management',
  label: '导航配置',
  icon: 'Link',
  path: '/admin/navigation-links',
  resources: ['config'],  // 复用 Grafana 相同的 config 权限
}
```

### 4.4 路由配置（router/index.ts）

新增子路由：

```typescript
{
  path: 'admin/navigation-links',
  name: 'NavigationLinks',
  component: () => import('@/views/admin/NavigationLinks.vue'),
  meta: { menuPermission: 'navigation-link-management' },
},
```

### 4.5 管理页（views/admin/NavigationLinks.vue）

**完全复用 GrafanaDashboards.vue 结构**：

- 顶部工具栏：搜索框（名称/URL）+ 刷新 + 批量保存排序 + 新建导航
- el-table 列：
  - 排序 input-number
  - 图标列（缩略 img，空时显示 fallback 首字母方块 + 🔄 重新抓取按钮）
  - 名称
  - URL（截断显示 origin + pathname）
  - 描述（show-overflow-tooltip）
  - 宽度 el-tag（蓝色 third / 黄色 half / 绿色 full）
  - 密度 el-tag（灰 compact / 蓝 normal / 紫 banner）
  - 高度（有覆盖值时显示数字+"px"，无显示 "—"）
  - 状态 Switch
  - 操作（编辑 / 删除）
- 弹窗表单（destroy-on-close）：
  - 顶部图标预览卡：48×48 favicon + 名称 + 域名 + 🔄 重新抓取按钮
  - 名称 input（maxlength 100）
  - URL input（maxlength 2048，输入时触发图标获取状态提示）
  - 描述 textarea（banner 模式会展示）
  - 宽度下拉（third/half/full）
  - 密度下拉（compact/normal/banner）
  - 高度 input-number（范围 40–200，placeholder "留空使用默认值"）
  - 排序号
  - 启用 Switch
- 搜索 300ms debounce（和项目约定一致）
- 所有表单校验规则与 GrafanaDashboards 对齐
- 保存时 full NavigationLinkInput 发送（保证字段完整性）

### 4.6 Dashboard 内嵌（views/Dashboard.vue）

**位置**：现有 Grafana section **之前**（上方）。

**加载**：挂载时并行 `loadNavigationLinks()`，独立 5 分钟刷新定时器。

**模板**：

```vue
<section v-if="navigationLinks.length > 0" class="navigation-section">
  <div class="section-header">
    <h2 class="section-title">
      <el-icon><Link /></el-icon>
      快捷导航
    </h2>
  </div>
  <div class="nav-grid">
    <a
      v-for="item in navigationLinks"
      :key="item.id"
      :href="item.url"
      target="_blank"
      rel="noopener"
      class="nav-card"
      :class="[`nav-width-${item.width}`, `nav-mode-${item.display_mode}`]"
      :style="cardHeightStyle(item)"
    >
      <div class="nav-icon">
        <img
          v-if="item.icon_data"
          :src="item.icon_data"
          :alt="item.name"
          @error="onIconError($event)"
        />
        <span v-else>{{ fallbackInitial(item.url) }}</span>
      </div>
      <div class="nav-info">
        <div class="nav-name">{{ item.name }}</div>
        <div v-if="item.display_mode === 'banner'" class="nav-desc">{{ item.description }}</div>
        <div v-if="item.display_mode !== 'compact'" class="nav-domain">{{ extractDomain(item.url) }}</div>
      </div>
      <el-icon class="nav-open-icon"><TopRight /></el-icon>
    </a>
  </div>
</section>
```

**关键函数**：

```typescript
// 高度计算：有微调值用微调值，否则用 display_mode 基准
const DISPLAY_MODE_BASE_HEIGHT: Record<string, number> = {
  compact: 52,
  normal: 68,
  banner: 100,
}

const cardHeightStyle = (item: NavigationLink) => {
  const h = item.height_px ?? DISPLAY_MODE_BASE_HEIGHT[item.display_mode] ?? 68
  return { height: `${h}px` }
}

// favicon 获取失败时，用域名首字母 + 自动取色
const FALLBACK_COLORS = ['#f46800', '#e6522c', '#00bfb3', '#24292f', '#d33833', '#0052cc', '#9333ea']
const fallbackInitial = (url: string) => {
  try {
    const host = new URL(url).hostname.replace(/^www\./, '')
    return host.charAt(0).toUpperCase()
  } catch {
    return '?'
  }
}
const fallbackColor = (url: string) => {
  // 域名 hash 选色，同一域名稳定同一颜色
  try {
    const host = new URL(url).hostname
    let h = 0
    for (const c of host) h = (h * 31 + c.charCodeAt(0)) >>> 0
    return FALLBACK_COLORS[h % FALLBACK_COLORS.length]
  } catch {
    return '#6b7280'
  }
}

// 提取域名展示
const extractDomain = (url: string) => {
  try {
    return new URL(url).hostname
  } catch {
    return url
  }
}
```

**CSS（scoped）**：与 Grafana dashboard grid 完全对齐的 3 列布局 + 响应式。

### 4.7 颜色基准（Dashboard fallback 图标色板）

| 颜色 | hex | 示例网站 |
|---|---|---|
| 橙色 | #f46800 | Grafana |
| 红色 | #e6522c | Prometheus |
| 青绿 | #00bfb3 | Kibana |
| 深灰 | #24292f | GitHub |
| 红橙 | #d33833 | Jenkins |
| 蓝色 | #0052cc | Jira |
| 紫色 | #9333ea | GitLab |

域名 hash 稳定映射，同一域名永远同色。

---

## 5. 安全与边界

| 风险 | 处理方式 |
|---|---|
| URL 钓鱼/恶意跳转 | 前端 `target="_blank" rel="noopener"`；后端白名单 http/https |
| favicon 抓取 DoS | 超时 5s，大小限制 20KB，证书跳过（内网环境） |
| 用户配置 XSS | name/description 字段前端由 Element Plus 渲染，自动转义；后端 SQL 参数化 |
| 跨域 Dashboard 内嵌 | 导航是新标签页打开，不是 iframe 嵌入（和 Grafana 不同），无此风险 |
| 大 favicon 数据膨胀 | 前端加载 base64 即弃；后端限制 20KB；列表 API 正常返回完整数据（Dashboard 卡片需要） |
| icon_fetcher goroutine 泄漏 | 创建/更新时启动 goroutine，用独立 context.Background（短生命周期）；不阻塞主请求 |
| 权限绕过 | 管理接口统一走 admin 角色中间件；公开接口只返回 enabled=1 |

---

## 6. 完整性检查清单

| 层 | 文件/位置 | 改动 |
|---|---|---|
| DB schema | `deploy/schema.sql` | 新增 `navigation_links` 表 + 索引 |
| DB migration | `deploy/migrations/v{ver}_navigation_links.sql` | 同上，增量脚本 |
| Backend model | `internal/model/navigation_link.go` | NavigationLink 结构体 + DisplayMode 常量 |
| Backend handler | `internal/handler/navigation_link.go` | CRUD + reorder + refresh-icon |
| Backend handler test | `internal/handler/navigation_link_test.go` | 覆盖所有 handler 路径 |
| Backend service | `internal/service/navigation_link.go` | 业务逻辑 + DB 访问 + 异步 icon 触发 |
| Backend service test | `internal/service/navigation_link_test.go` | CRUD 边界 + 排序 + 异步 icon 触发验证 |
| Icon fetcher | `pkg/iconfetch/fetcher.go` | 独立模块，零业务依赖 |
| Icon fetcher test | `pkg/iconfetch/fetcher_test.go` | 核心覆盖点见 3.6 节 |
| Router | `cmd/app.go` 或路由注册 | 注册 8 条新路由 |
| 前端 API | `web/src/api/navigation.ts` | 8 个方法 |
| 前端类型 | `web/src/types/index.ts` | 3 个新 interface |
| 前端管理页 | `web/src/views/admin/NavigationLinks.vue` | 完整 CRUD 页面 |
| 前端 Dashboard | `web/src/views/Dashboard.vue` | 新增 section + loadNavigationLinks |
| 前端路由 | `web/src/router/index.ts` | 新增 admin/navigation-links 路由 |
| 前端菜单 | `web/src/config/menuPermissionMap.ts` | 新增导航配置菜单项 |

---

## 7. 与 Grafana Dashboard 的模式对照

| 维度 | Grafana Dashboard | Navigation Link |
|---|---|---|
| 展示方式 | Dashboard 内 iframe 嵌入 | Dashboard 内卡片 + 新标签页打开 |
| 宽度 | width: third/half/full | width: third/half/full ✅ 相同 |
| 高度 | height_px 像素 | display_mode 基准 + height_px 覆盖（两者结合） |
| 图标 | iframe 自带 | 后端自动抓取 favicon 存 base64 |
| 管理菜单 | system-admin > Grafana 图表 | system-admin > 导航配置 ✅ 同级 |
| 权限 | config | config ✅ 相同 |
| 管理页模式 | CRUD + 排序 + 启用 | 完全相同 ✅ |
| 公开接口 | 有（listEnabled） | 有（listEnabled）✅ |
| 异步副作用 | 无 | 创建/更新 URL 时异步抓 favicon |

---

## 8. 后续可扩展（不在本次 scope）

- 导航图标可手动上传（PNG/SVG 文件）
- 导航 URL 支持 SSO Token 注入（某些内网系统需要）
- 按领域（domain）隔离不同导航集
- 导航访问审计（谁点了哪个链接）

本次只做：DB 新表 + 后端 CRUD + icon_fetcher + 管理页 + Dashboard 内嵌。
