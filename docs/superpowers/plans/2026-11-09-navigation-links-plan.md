# Implementation Plan: Navigation Links (导航快捷链接)

- **Status**: Draft
- **Spec**: [2026-11-09-navigation-links-design.md](../specs/2026-11-09-navigation-links-design.md)
- **Date**: 2026-11-09
- **Author**: writing-plans
- **Branch**: feature/navigation-links（新建）

## Goals

1. 管理员在系统管理 → 导航配置 中维护外部网站链接列表（CRUD + 排序 + 启用禁用 + 手动重抓图标）
2. 所有登录用户在 Dashboard 首页看到「快捷导航」卡片区域，点击在新标签页打开
3. 创建/更新 URL 时后端**自动抓取** favicon 存为 base64 data URL
4. 卡片支持 width（third/half/full）+ display_mode（compact/normal/banner）+ height_px 微调

## Out of Scope（本次不做）

- 手动上传图标文件（只用自动抓取 + fallback）
- 按领域隔离导航集
- SSO Token 注入到跳转 URL
- 导航访问审计
- 导航点击统计

## Tech Stack (resolved)

| 选择 | 理由 |
|---|---|
| DB 表名 `bdopsflow_navigation_links` | 与现有 `bdopsflow_grafana_dashboards` 前缀一致 |
| 表结构完全对齐 Grafana 字段风格 | 团队熟悉，减少认知负担 |
| favicon 存 base64 data URL（TEXT 列） | 前端零跨域依赖，避免 icon 服务器 |
| icon_fetcher 放 `scheduler/pkg/iconfetch/` | pkg 目录存可复用/可单测模块，现有 pkg/db、pkg/config 等已验证 |
| 迁移脚本编号 v10 | 现有最高 v9，顺序递增 |
| HTTP 客户端用 `net/http` + 5s 超时 | 标准库足够，避免引入新依赖 |
| HTML link 正则提取（不引入 html/tokenizer） | 目标简单（只提取 favicon URL），正则够用 |
| `height_px` 为 NULL 时前端用 display_mode 基准值 | 后端简化，前端语义清晰 |

---

## Phase 1: DB Schema + Model + Errors

**Goal**: 建表 + Go model 定义 + service errors（让后续 phases 有东西依赖）

### 1.1 迁移脚本

**文件**: `deploy/migrations/v10_navigation_links.sql`

```sql
PRAGMA foreign_keys = OFF;

CREATE TABLE IF NOT EXISTS bdopsflow_navigation_links (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT    NOT NULL,
    description     TEXT,
    url             TEXT    NOT NULL,
    width           TEXT    NOT NULL DEFAULT 'third',
    display_mode    TEXT    NOT NULL DEFAULT 'normal',
    height_px       INTEGER,
    icon_data       TEXT,
    icon_fetched_at TEXT,
    sort_order      INTEGER NOT NULL DEFAULT 0,
    enabled         INTEGER NOT NULL DEFAULT 1,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_nav_links_enabled_sort
    ON bdopsflow_navigation_links(enabled, sort_order);

PRAGMA foreign_keys = ON;
```

### 1.2 更新主 schema.sql

**文件**: `deploy/schema.sql`（在 grafana_dashboards 表之后追加上面同一段 CREATE）

### 1.3 Model 定义

**文件**: `scheduler/internal/model/navigation_link.go`

```go
// NavigationLink 导航快捷链接配置（Dashboard 首页展示）
type NavigationLink struct {
    ID           int64
    Name         string
    Description  string
    URL          string
    Width        string       // "third" | "half" | "full"
    DisplayMode  string       // "compact" | "normal" | "banner"
    HeightPx     int          // 0 表示使用 display_mode 基准值
    IconData     string       // base64 data: URL，可空
    IconFetchedAt time.Time  // 最近一次抓取时间，零值表示未抓过
    SortOrder    int
    Enabled      int          // 0 禁用 / 1 启用
    CreatedAt    time.Time
    UpdatedAt    time.Time
}

// NavigationReorderItem 批量排序项
type NavigationReorderItem struct {
    ID        int64
    SortOrder int
}

// DisplayMode 基准高度映射（单位 px）
var DisplayModeBaseHeight = map[string]int{
    "compact": 52,
    "normal":  68,
    "banner":  100,
}
```

### 1.4 Errors 定义

**文件**: `scheduler/internal/service/errors.go`（在现有 grafana errors 块后追加）

```go
// Navigation Link 错误
ErrNavigationLinkNotFound      = errors.New("navigation link not found")
ErrNavigationInvalidURL        = errors.New("navigation link url must start with http:// or https://")
ErrNavigationInvalidWidth      = errors.New("navigation link width must be one of: third, half, full")
ErrNavigationInvalidDisplayMode = errors.New("navigation link display_mode must be one of: compact, normal, banner")
ErrNavigationInvalidHeight      = errors.New("navigation link height_px must be between 40 and 200 or zero")
ErrNavigationInvalidSortOrder  = errors.New("navigation link sort_order must be >= 0")
```

错误码映射追加到 errors.go 的 `ErrorCodeMap`：
- 19101 → ErrNavigationLinkNotFound
- 19102 → ErrNavigationInvalidURL
- 19103 → ErrNavigationInvalidWidth
- 19104 → ErrNavigationInvalidDisplayMode
- 19105 → ErrNavigationInvalidHeight
- 19106 → ErrNavigationInvalidSortOrder

### 1.5 Verification

```bash
cd scheduler && go build ./internal/... && go vet ./internal/...
```

**Done when**: 编译通过，vet 无警告。

---

## Phase 2: Icon Fetcher 模块（pkg/iconfetch）

**Goal**: 独立的 favicon 抓取模块，纯函数风格，无业务依赖，完整单测

### 2.1 文件清单

| 文件 | 说明 |
|---|---|
| `scheduler/pkg/iconfetch/fetcher.go` | Fetch 主函数 + HTML link 提取 + SVG/PNG/ICO 处理 + base64 编码 |
| `scheduler/pkg/iconfetch/fetcher_test.go` | 完整单测（见 2.3 覆盖点） |

### 2.2 核心 API

```go
// FetchResult 抓取结果
type FetchResult struct {
    IconData   string    // data:image/svg+xml;base64,PHN2Zy... 格式
    FetchedAt  time.Time
    Source     string    // "link-tag" | "favicon-ico"
}

// Fetch 抓取指定 URL 域名下的 favicon
// 返回 (FetchResult, error)；error 非 nil 时 result 内字段可能为空
func Fetch(ctx context.Context, urlStr string) (FetchResult, error)
```

**内部逻辑（fetcher.go 约 150 行）**:

```
Fetch(url):
  1. net/url.Parse → 取 origin（scheme + host）
  2. context.WithTimeout(ctx, 5s)
  3. 并发启动两个 goroutine:
     a) GET origin + "/favicon.ico"
     b) GET origin + "/" 取 HTML
  4. 优先处理 HTML goroutine 结果:
     - 正则 <link[^>]+rel=["'](?:shortcut\s+)?icon["'][^>]+href=["']([^"']+)["']
     - 正则 <link[^>]+rel=["']apple-touch-icon["'][^>]+href=["']([^"']+)["']
     - 正则 <link[^>]+href=["']([^"']+)["'][^>]*rel=["'](?:shortcut\s+)?icon["']
     - 匹配到 → href 可能是相对路径，用 origin 拼完整 URL
     - 下载 href 内容
  5. HTML 无结果 → 用 favicon.ico goroutine 的结果
  6. 内容校验: 大小 ≤ 20KB, 检测 Content-Type 或文件头 magic number
  7. 转 base64 → "data:{mime};base64,{base64Bytes}"
  8. 返回 FetchResult{IconData, FetchedAt: now, Source}
```

**HTTP 请求细节**:
- `http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}`（内网自签证书）
- `http.Client.Timeout = 5 * time.Second`
- `resp.Body` 读入 `[]byte`，限制 `io.LimitReader(body, 20*1024)`
- 关闭 resp.Body

**格式识别**（magic number）:
- `0x3C 0x3F` → SVG（XML 声明开头）
- `0x89 0x50 0x4E 0x47` → PNG
- `0xFF 0xD8 0xFF` → JPEG
- `0x00 0x00` 开头且 header 含 "ico" → ICO
- `0x3C 0x73 0x76 0x67` 或含 `<svg` → SVG

### 2.3 单测覆盖点

**文件**: `fetcher_test.go`

使用 httptest.NewServer mock 响应：

| 测试 | 场景 | 预期 |
|---|---|---|
| TestFetch_HTMLLinkTag_SVG | HTML 含 `<link rel="icon" href="/favicon.svg">` | Source="link-tag", 返回 SVG data URL |
| TestFetch_HTMLLinkTag_Relative | href 是相对路径 `/assets/logo.png` | 正确拼 origin |
| TestFetch_FavIconIco | HTML 无 link tag 但 /favicon.ico 返回内容 | Source="favicon-ico" |
| TestFetch_EmptyHTML | HTML 空，favicon.ico 有 | 回退成功 |
| TestFetch_Both404 | 两个路径都 404 | 返回 error |
| TestFetch_Timeout | 目标服务器不响应 | 5s 超时返回 error |
| TestFetch_TooLarge | 返回 30KB 内容 | error: content too large |
| TestFetch_BadURL | url="not-a-url" | 早返回 error |
| TestFetch_NonHTTP | url="ftp://..." | 早返回 error |
| TestFetch_SSLCertFail | httptest 用自签 + InsecureSkipVerify | 仍然成功 |
| TestFetch_Base64Format | 返回的 IconData 前缀 | `data:image/...;base64,` |

### 2.4 Verification

```bash
cd scheduler && go test ./pkg/iconfetch/... -v -count=1
```

**Done when**: 11+ 测试全部 PASS，coverprofile ≥ 80%

---

## Phase 3: Service + Handler + 路由注册

**Goal**: 完整的后端 CRUD 链路，公开接口 + 管理接口 + 异步 icon 触发 + 手动重抓

### 3.1 Service 层

**文件**: `scheduler/internal/service/navigation_link.go`（~350 行，**直接复制 grafana_dashboard.go 结构改字段**）

核心方法：

```go
type NavigationLinkService struct {
    db database.DB
}

func NewNavigationLinkService(db database.DB) *NavigationLinkService

// CRUD
func (s *NavigationLinkService) Create(ctx context.Context, d *model.NavigationLink) (*model.NavigationLink, error)
func (s *NavigationLinkService) Update(ctx context.Context, id int64, d *model.NavigationLink) error
func (s *NavigationLinkService) Delete(ctx context.Context, id int64) error
func (s *NavigationLinkService) GetByID(ctx context.Context, id int64) (*model.NavigationLink, error)
func (s *NavigationLinkService) ListEnabled(ctx context.Context) ([]model.NavigationLink, error) // enabled=1
func (s *NavigationLinkService) ListAll(ctx context.Context) ([]model.NavigationLink, error)

// Reorder
func (s *NavigationLinkService) Reorder(ctx context.Context, items []model.NavigationReorderItem) error

// 图标相关
func (s *NavigationLinkService) UpdateIcon(ctx context.Context, id int64, iconData string, fetchedAt time.Time) error
func (s *NavigationLinkService) RefreshIcon(ctx context.Context, id int64) (*model.NavigationLink, error)
```

**validate 函数**（复制 grafana validate 结构）：
- Name: 非空、≤100 字符
- URL: http/https scheme、≤2048 字符
- Width: one of third/half/full
- DisplayMode: one of compact/normal/banner
- HeightPx: 0 或 40–200（0 表示用基准值）
- SortOrder: ≥0

**异步 icon 触发**（Create/Update 时）：

```go
func (s *NavigationLinkService) Create(ctx context.Context, d *model.NavigationLink) (*model.NavigationLink, error) {
    // ... DB insert ...
    // 异步触发 icon fetch
    go func() {
        result, err := iconfetch.Fetch(context.Background(), d.URL)
        if err != nil {
            slog.Warn("navigation link icon fetch failed", "id", d.ID, "error", err)
            return
        }
        _ = s.UpdateIcon(context.Background(), d.ID, result.IconData, result.FetchedAt)
    }()
    return d, nil
}

// Update: 只在 URL 变更时才自动重新抓 icon
func (s *NavigationLinkService) Update(ctx context.Context, id int64, d *model.NavigationLink) error {
    old, err := s.GetByID(ctx, id)
    // ... update ...
    if old != nil && old.URL != d.URL {
        go func() { /* 同上 fetch + UpdateIcon */ }()
    }
}
```

**RefreshIcon（手动触发）**:
```go
func (s *NavigationLinkService) RefreshIcon(ctx context.Context, id int64) (*model.NavigationLink, error) {
    d, err := s.GetByID(ctx, id)
    // ... fetch ... UpdateIcon ...
    return d, nil
}
```

**SQL 字段**（CREATE TABLE 后）:

```sql
INSERT INTO bdopsflow_navigation_links
  (name, description, url, width, display_mode, height_px, sort_order, enabled, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)

SELECT id, name, description, url, width, display_mode, height_px, icon_data, icon_fetched_at, sort_order, enabled, created_at, updated_at
FROM bdopsflow_navigation_links WHERE ... ORDER BY sort_order ASC, id ASC

UPDATE bdopsflow_navigation_links
  SET icon_data = ?, icon_fetched_at = ?, updated_at = ? WHERE id = ?
```

**Helper 函数**（从 grafana_dashboard.go 共用或新建）:
- `rowInt64(row[n])` → 新建到 `navigation_link.go` 内部
- `rowInt(row[n])` → 同上
- `rowString(row[n])` → 同上
- `parseDateTime(cell)` → 同上
- `isEmpty(cell)` → 同上

（如果这些已经抽成公共 helper 则 import 复用；否则在 service 包内各 grafana/navigation 各写一份，保持独立性）

### 3.2 Handler 层

**文件**: `scheduler/internal/handler/navigation_link.go`（~220 行，复制 grafana_dashboard.go 结构改字段）

```go
type NavigationLinkHandler struct {
    svc *service.NavigationLinkService
}

func NewNavigationLinkHandler(svc *service.NavigationLinkService) *NavigationLinkHandler

func (h *NavigationLinkHandler) ListEnabled(c *gin.Context)    // GET /navigation-links
func (h *NavigationLinkHandler) ListAll(c *gin.Context)       // GET /admin/navigation-links
func (h *NavigationLinkHandler) GetByID(c *gin.Context)       // GET /admin/navigation-links/:id
func (h *NavigationLinkHandler) Create(c *gin.Context)        // POST /admin/navigation-links
func (h *NavigationLinkHandler) Update(c *gin.Context)        // PUT /admin/navigation-links/:id
func (h *NavigationLinkHandler) Delete(c *gin.Context)        // DELETE /admin/navigation-links/:id
func (h *NavigationLinkHandler) Reorder(c *gin.Context)       // PUT /admin/navigation-links/reorder
func (h *NavigationLinkHandler) RefreshIcon(c *gin.Context)   // POST /admin/navigation-links/:id/refresh-icon
```

**请求结构体（Create/Update 共用）**:
```go
var req struct {
    Name        string `json:"name" binding:"required"`
    Description string `json:"description"`
    URL         string `json:"url" binding:"required"`
    Width       string `json:"width" binding:"required"`
    DisplayMode string `json:"display_mode" binding:"required"`
    HeightPx    int    `json:"height_px"`       // 0 表示 NULL
    SortOrder   int    `json:"sort_order"`
    Enabled     *int   `json:"enabled"`
}
```

**成功响应**: 统一 `handler.Success(c, item)` / `handler.Success(c, gin.H{"items": items})`

### 3.3 路由注册

**文件**: `scheduler/cmd/app.go`

- App struct 新增字段：`navSvc *service.NavigationLinkService`
- 初始化时新增：`navSvc := service.NewNavigationLinkService(logDB)`
- 位置：紧跟 `grafanaSvc := ...` 之后

**文件**: `scheduler/cmd/routes.go`

在 Grafana Dashboard 路由块之后新增：
```go
// 导航快捷链接
navHandler := handler.NewNavigationLinkHandler(app.navSvc)
// 公共查询（所有登录用户）
protected.GET("/navigation-links", navHandler.ListEnabled)
// 管理接口（仅系统管理员）
navAdmin := protected.Group("/admin/navigation-links")
{
    navAdmin.GET("", middleware.RequireSystemAdmin(app.permissionService), navHandler.ListAll)
    navAdmin.POST("", middleware.RequireSystemAdmin(app.permissionService), navHandler.Create)
    navAdmin.PUT("/reorder", middleware.RequireSystemAdmin(app.permissionService), navHandler.Reorder)
    navAdmin.GET("/:id", middleware.RequireSystemAdmin(app.permissionService), navHandler.GetByID)
    navAdmin.PUT("/:id", middleware.RequireSystemAdmin(app.permissionService), navHandler.Update)
    navAdmin.DELETE("/:id", middleware.RequireSystemAdmin(app.permissionService), navHandler.Delete)
    navAdmin.POST("/:id/refresh-icon", middleware.RequireSystemAdmin(app.permissionService), navHandler.RefreshIcon)
}
```

### 3.4 Service 单测

**文件**: `scheduler/internal/service/navigation_link_test.go`

**覆盖点**（复制 grafana_dashboard_test.go 结构）：
- TestValidate_Name（空、超长、正常）
- TestValidate_URL（协议、长度）
- TestValidate_Width（third/half/full）
- TestValidate_DisplayMode（compact/normal/banner）
- TestValidate_Height（0→允许、40→允许、200→允许、201→拒绝、39→拒绝）
- TestValidate_SortOrder（负→拒绝、正→允许）
- TestValidate_ValidConfig（完整合法对象）
- TestGetErrorCode_NavErrors（6 个错误码映射）

**DB 层 CRUD 集成测试**（如果项目现有 db testutil 支持）：
- TestNavigationLinkCRUD（Create → Get → Update → ListEnabled → ListAll → Reorder → Delete）

### 3.5 Verification

```bash
cd scheduler
go build ./... && go vet ./...
go test ./internal/service/... -run TestValidate -v -count=1
go test ./internal/handler/... -v -count=1   # 如有 handler 测试环境
go test ./pkg/iconfetch/... -v -count=1
```

**Done when**: 后端全部编译通过 + Service 校验测试 PASS + Icon fetcher 测试 PASS

---

## Phase 4: 前端 API + 类型 + 路由 + 菜单

**Goal**: 前端"管线"连通，但 Dashboard 展示和管理页 UI 还没写

### 4.1 类型定义

**文件**: `web/src/types/index.ts`（尾部追加）

```typescript
// ============================================================================
// 十六、导航快捷链接（NavigationLink）相关类型
// ============================================================================
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

### 4.2 API 封装

**新建文件**: `web/src/api/navigation.ts`

```typescript
import api from '@/utils/api'
import type { NavigationLink, NavigationLinkInput, NavigationReorderItem } from '@/types'

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

**更新文件**: `web/src/api/index.ts`（新增 export）

```typescript
export { navigationAPI } from './navigation'
```

### 4.3 路由配置

**文件**: `web/src/router/index.ts`（在 Grafana 路由之后追加）

```typescript
{
  path: 'admin/navigation-links',
  name: 'NavigationLinks',
  component: () => import('@/views/admin/NavigationLinks.vue'),
  meta: { menuPermission: 'navigation-link-management' },
},
```

### 4.4 菜单权限映射

**文件**: `web/src/config/menuPermissionMap.ts`（在 system-admin children 数组中 Grafana 之后追加）

```typescript
{
  key: 'navigation-link-management',
  label: '导航配置',
  icon: 'Link',
  path: '/admin/navigation-links',
  resources: ['config'],
},
```

### 4.5 Verification

```bash
cd web
npm run type-check  # 或 tsc --noEmit
npm run build       # 确保 build 不报错
```

**Done when**: tsc 无类型错误，build exit 0

---

## Phase 5: 管理页 UI（NavigationLinks.vue）

**Goal**: 完整的 CRUD 管理页 + 弹窗表单 + 搜索 + 排序 + 图标重新抓取按钮

### 5.1 文件

**新建文件**: `web/src/views/admin/NavigationLinks.vue`（约 500–600 行，**复制 GrafanaDashboards.vue 改字段**）

### 5.2 UI 结构

完全对齐 GrafanaDashboards.vue 的结构：
- `<template>` 顶部工具栏：搜索框 + 刷新 + 批量保存排序 + 新建导航按钮
- `el-table` 列：排序 / 图标（缩略 32×32 img + 🔄 重新抓取按钮）/ 名称 / URL / 描述 / 宽度 tag / 密度 tag / 高度 / 状态 Switch / 操作
- `<el-dialog>` 弹窗：图标预览区 / 名称 / URL / 描述 / 宽度下拉 / 密度下拉 / 高度 input-number（placeholder "留空使用默认值"）/ 排序号 / 启用 Switch

### 5.3 表单校验（validateForm 函数）

| 字段 | 规则 |
|---|---|
| name | 必填、≤100 字符 |
| url | 必填、http/https、≤2048 |
| width | one of third/half/full |
| display_mode | one of compact/normal/banner |
| height_px | 留空 or 40–200 |
| sort_order | ≥0 |

### 5.4 特殊点（相比 Grafana 的差异）

| Grafana | Navigation Link |
|---|---|
| height_px 必填 120–1200 | height_px 可选 NULL 或 40–200 |
| 宽度决定 iframe grid-col-span | 相同，复用 CSS 类 |
| 无图标列 | 图标列显示 favicon 缩略 + 🔄 按钮 |
| 无 | 手动刷新图标按钮 → 调 refreshIcon API |
| 管理 embed_url（iframe 链接） | 管理 url（新标签页跳转） |

### 5.5 Verification

```bash
cd web && npm run dev
# 浏览器访问 /admin/navigation-links
# 手动测试:
# 1. 新建导航 → 保存 → 列表出现
# 2. 刷新图标按钮 → 图标更新
# 3. 编辑 → 保存 → 字段更新
# 4. 禁用 → 列表仍在但 enabled 变 0
# 5. 删除 → 确认框 → 消失
# 6. 批量排序保存
```

**Done when**: 所有手动测试通过，无报错

---

## Phase 6: Dashboard 内嵌展示

**Goal**: Dashboard 首页「快捷导航」卡片区域，位置在 Grafana 图表 section 之前

### 6.1 修改文件

**文件**: `web/src/views/Dashboard.vue`

### 6.2 追加的 imports

```typescript
import { navigationAPI, type NavigationLink } from '@/api'
// 新增图标: Link（导航标题），保留已有 TopRight
```

### 6.3 追加的响应式状态

```typescript
const navigationLinks = ref<NavigationLink[]>([])
let navInterval: number | null = null

const DISPLAY_MODE_BASE_HEIGHT: Record<string, number> = {
  compact: 52,
  normal:  68,
  banner:  100,
}

const FALLBACK_COLORS = ['#f46800', '#e6522c', '#00bfb3', '#24292f', '#d33833', '#0052cc', '#9333ea']
```

### 6.4 追加的工具函数

```typescript
const loadNavigationLinks = async () => { /* 静默加载，失败不影响 Dashboard */ }
const cardHeightStyle = (item: NavigationLink) => item.height_px ?? DISPLAY_MODE_BASE_HEIGHT[item.display_mode] ?? 68
const fallbackInitial = (url: string) => { try { return new URL(url).hostname.replace(/^www\./, '').charAt(0).toUpperCase() } catch { return '?' } }
const fallbackColor = (url: string) => { /* 域名 hash → 稳定取色 */ }
const extractDomain = (url: string) => { try { return new URL(url).hostname } catch { return url } }
const onIconError = (e: Event) => { /* 隐藏 img，让 fallback span 显示 */ }
```

### 6.5 追加的模板 section

在 `<!-- Grafana 嵌入图表 -->` section **之前**：

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
      :style="{ height: cardHeightStyle(item) + 'px' }"
    >
      <div class="nav-icon" :style="{ backgroundColor: item.icon_data ? '' : fallbackColor(item.url) }">
        <img
          v-if="item.icon_data"
          :src="item.icon_data"
          :alt="item.name"
          @error="onIconError"
        />
        <span v-else>{{ fallbackInitial(item.url) }}</span>
      </div>
      <div class="nav-info">
        <div class="nav-name">{{ item.name }}</div>
        <div v-if="item.display_mode === 'banner' && item.description" class="nav-desc">{{ item.description }}</div>
        <div v-if="item.display_mode !== 'compact'" class="nav-domain">{{ extractDomain(item.url) }}</div>
      </div>
      <el-icon class="nav-open-icon"><TopRight /></el-icon>
    </a>
  </div>
</section>
```

### 6.6 onMounted 追加

```typescript
loadNavigationLinks()
navInterval = window.setInterval(loadNavigationLinks, 5 * 60 * 1000)  // 5 分钟
```

### 6.7 onUnmounted 追加

```typescript
if (navInterval) clearInterval(navInterval)
```

### 6.8 CSS（scoped，追加）

与现有 `.grafana-*` class 同文件，新增 `.navigation-section` / `.nav-grid` / `.nav-card` / `.nav-icon` / `.nav-info` / `.nav-name` / `.nav-desc` / `.nav-domain` / `.nav-open-icon` / `.nav-width-third` / `.nav-width-half` / `.nav-width-full` / `.nav-mode-compact` / `.nav-mode-normal` / `.nav-mode-banner`

**关键 CSS**：
- `.nav-grid`: `display:grid; grid-template-columns:repeat(3,1fr); gap:12px;`
- `.nav-width-third`: `grid-column:span 1;`
- `.nav-width-half`: 默认 1 列，`@media (min-width:992px) { grid-column:span 2; }`
- `.nav-width-full`: `grid-column:span 3;`
- `.nav-icon`: `width:40px; height:40px; border-radius:10px; flex-shrink:0; display:flex; align-items:center; justify-content:center; color:#fff; font-weight:700;`
- `.nav-icon img`: `width:100%; height:100%; object-fit:contain; border-radius:10px;`
- 响应式：`@media (max-width:768px)` nav-grid 变 1 列

### 6.9 Verification

```bash
cd web && npm run dev
# 浏览器访问 /（Dashboard 首页）
# 手动测试:
# 1. 先在管理页添加 6 条不同 width/display_mode 的导航
# 2. Dashboard 快捷导航区域出现，卡片高度正确
# 3. 点击卡片 → 新标签页打开目标 URL
# 4. 禁用一条 → Dashboard 自动移除（等刷新或 5 分钟缓存）
# 5. 无配置时 Dashboard 不显示该 section
# 6. 调整浏览器宽度 → 响应式正常
```

**Done when**: Dashboard 所有手动测试通过，卡片渲染正确

---

## Phase 7: 全链路验收 + 文档更新

### 7.1 端到端手动验收

| # | 步骤 | 预期 |
|---|---|---|
| 1 | 启动 scheduler，应用 migration v10 | 无报错，表建成功 |
| 2 | 管理页新建导航 https://grafana.example.com | 保存成功，icon_data 异步填充 |
| 3 | Dashboard 首页刷新 | 快捷导航卡片出现，含 favicon |
| 4 | 点击卡片 | 新标签页打开 |
| 5 | 禁用该导航 | Dashboard 刷新后消失 |
| 6 | 编辑 URL | Dashboard 刷新后新 URL 生效，favicon 自动更新 |
| 7 | 图标重新抓取按钮 | 返回更新后的图标 |
| 8 | 排序调整 | Dashboard 展示顺序同步变更 |

### 7.2 文档更新（如需）

- 更新 `docs/GUIDE.md` 新增"导航配置"章节（可选）
- 更新 `AGENTS.md` 项目结构中提及新文件（如果有）

### 7.3 提交 checklist

- [ ] Go 编译 `go build ./...` exit 0
- [ ] Go vet `go vet ./...` 无警告
- [ ] Go test `go test ./pkg/iconfetch/... ./internal/service/...` PASS
- [ ] 前端 `npm run build` exit 0
- [ ] TypeScript `tsc --noEmit` 无错误
- [ ] 端到端手动验收通过
- [ ] 提交 message: `feat(navigation): add navigation links with auto favicon fetching`

---

## Risks & Open Questions

| 风险 | 概率 | 缓解 |
|---|---|---|
| favicon 抓取被 CDN/WAF 拦截返回 403 | 中 | icon_data 留 NULL，前端 fallback 首字母不报错；后台记 warn 日志 |
| 内网站点需要认证页面才能取 HTML | 低 | fetch 只取公开 favicon.ico 即可，link tag 提取失败自然回退 |
| favicon 存储膨胀（20KB × 100 条 = 2MB） | 低 | 2MB 可忽略；如需更严格可改 10KB 限制 |
| Dashboard 5 分钟缓存导致配置变更感知滞后 | 可接受 | 用户点手动刷新即可；spec 已明确 |
| `Reorder` 用批量 WriteParameterized 会占 raft 提交 | 低 | 与 Grafana 完全一致，团队已验证 |
| 新增 pkg/iconfetch 目录未在 go.mod tidy | 低 | `go mod tidy` 会自动处理 |

## Definition of Done

- Phase 1–7 全部完成
- 所有 verification 命令 exit 0
- Go test 覆盖 icon_fetch ≥ 80%、service validate ≥ 90%
- 前端 TS 无类型错误、build 成功
- 端到端手动验收 8 项通过
- 无 security vulnerability（URL 协议白名单、CSP、XSS 防护）
