/**
 * @file index.ts - 前端全局 TypeScript 类型定义
 * 集中导出所有后端接口返回的数据模型、请求体、视图层使用的结构等类型，
 * 供 stores / api / views 等各模块统一引用。
 */

// ============================================================================
// 一、用户认证与登录相关类型
// ============================================================================

/** 用户基础信息 */
export interface User {
  id: number                    // 用户 ID
  username: string              // 登录用户名
  real_name: string             // 真实姓名
  phone: string                 // 手机号
  email: string                 // 邮箱
  is_active: boolean            // 是否启用
  last_login_at: string | null  // 最近一次登录时间
  created_by?: number           // 创建者用户 ID（仅管理员创建时有值）
  created_at: string            // 创建时间
  updated_at: string            // 更新时间
  role_codes?: string[]         // 当前角色编码列表（由后端在某些接口中返回）
  role_ids?: number[]           // 当前角色 ID 列表
  domain_ids?: number[]         // 可访问的领域 ID 列表
}

// ============================================================================
// 二、RBAC 权限模型相关类型（Permission / Role / PermissionGroup / Menu 映射）
// ============================================================================

/** RBAC 权限：声明某个 resource 上允许的 action */
export interface Permission {
  id: number          // 权限 ID
  resource: string    // 资源标识符，如 'task', 'datasource'
  action: string      // 操作标识符，如 'create', 'update', 'manage'
  description: string // 权限描述
}

// ============================================================================
// 三、领域 / 租户相关类型（Domain / DomainInfo / DomainWithStats）
// ============================================================================

/** 用户可访问的领域（租户）简化信息 */
export interface DomainInfo {
  domain_id: number       // 领域 ID
  domain_name: string     // 领域名称
  is_default: boolean     // 是否用户默认选中的领域
}

// ============================================================================
// 四、登录 / 会话相关接口返回体（LoginResponse / CurrentUserResponse / SwitchDomainResponse）
// ============================================================================

/** 登录接口返回体 */
export interface LoginResponse {
  token: string                 // 访问令牌
  refresh_token: string         // 刷新令牌
  user: User                    // 当前用户信息
  permissions: Permission[]     // 当前用户权限列表
  domains: DomainInfo[]         // 可访问领域列表
  current_domain_id: number     // 当前领域 ID
  role_codes: string[]          // 当前角色编码列表
}

/** 获取当前用户接口返回体 */
export interface CurrentUserResponse {
  user: User                    // 当前用户信息
  permissions: Permission[]     // 当前用户权限列表
  domains: DomainInfo[]         // 可访问领域列表
  current_domain_id: number     // 当前领域 ID
  role_codes: string[]          // 当前角色编码列表
}

/** 切换领域接口返回体（不含 user / domains，因为切换领域时用户身份不变） */
export interface SwitchDomainResponse {
  token: string                 // 新领域下的访问令牌
  refresh_token: string         // 新领域下的刷新令牌
  permissions: Permission[]     // 新领域下的权限列表
  current_domain_id: number     // 新领域 ID
  role_codes: string[]          // 新领域下的角色编码列表
}

// ============================================================================
// 五、角色定义（Role）与领域（Domain）详细类型
// ============================================================================

/** RBAC 角色定义 */
export interface Role {
  id: number                  // 角色 ID
  name: string                // 角色展示名
  code: string                // 角色唯一编码（如 system_admin）
  description: string         // 角色说明
  is_system: boolean          // 是否系统内置（不可删除）
  parent_id: number | null    // 父角色 ID（支持角色继承）
  domain_id: number | null    // 所属领域 ID，null 表示跨领域全局角色
  created_at: string          // 创建时间
  updated_at: string          // 更新时间
  permissions?: Permission[]  // 角色绑定的权限列表（某些接口展开返回）
}

/** 领域（租户）基础信息 */
export interface Domain {
  id: number          // 领域 ID
  name: string        // 领域名称
  description: string // 领域描述
  created_at: string  // 创建时间
  updated_at: string  // 更新时间
}

/** 带统计信息的领域（用于领域管理列表页） */
export interface DomainWithStats extends Domain {
  user_count: number      // 领域下用户数量
  executor_count: number  // 领域下执行器数量
  task_count: number      // 领域下任务数量
}

// ============================================================================
// 六、调度任务（Task / TaskConfig / TaskExecution / TaskLog）相关类型
// ============================================================================

/** 调度任务（HTTP / Shell 等） */
export interface Task {
  id: number                      // 任务 ID
  name: string                    // 任务名称
  type: string                    // 任务类型，如 'http', 'shell'
  config: string                  // 任务配置 JSON 字符串
  cron_expression: string         // Cron 调度表达式
  timeout_seconds: number         // 单次执行超时秒数
  retry_count: number             // 失败重试次数
  retry_interval: number          // 重试间隔秒数
  is_enabled: boolean             // 是否启用调度
  status: string                  // 调度状态，如 'running', 'paused'
  domain_id: number               // 所属领域 ID
  webhook_id: number | null       // 关联的 Webhook ID
  webhook_events: string          // 触发 Webhook 的事件列表 JSON
  assigned_executor_id: number | null // 绑定的执行器 ID，null 表示自动分配
  has_available_executors: boolean    // 是否有可用的执行器
  created_by: number              // 创建者用户 ID
  created_by_name: string         // 创建者用户名
  created_at: string              // 创建时间
  updated_at: string              // 更新时间
  next_execution_time: string     // 下次执行时间
  last_execution_status: string   // 上一次执行结果
}

/** 任务配置的结构化表示（解析自 Task.config JSON） */
export interface TaskConfig {
  url?: string        // HTTP 任务的目标 URL
  method?: string     // HTTP 方法
  timeout?: number    // HTTP 超时
  headers?: string    // HTTP 请求头 JSON
  body?: string       // HTTP 请求体
  script?: string     // Shell 任务脚本
}

// ============================================================================
// 七、执行器（Executor）与任务执行记录相关类型
// ============================================================================

/** 执行器：接收调度请求、实际执行任务的后端节点 */
export interface Executor {
  id: number              // 执行器 ID
  name: string            // 执行器注册名（唯一）
  address: string         // 执行器可达地址
  status: string          // 运行状态，如 'online', 'offline'
  last_heartbeat: string | null  // 最近一次心跳时间
  capacity: number        // 并发执行上限
  current_load: number    // 当前并发数
  is_global: boolean      // 是否全局执行器（可跨领域执行）
  created_at: string      // 创建时间
  updated_at: string      // 更新时间
}

/** 单次任务执行记录（详细版，含进度和输出） */
export interface TaskExecution {
  id: number          // 执行记录 ID
  task_id: number     // 所属任务 ID
  execution_id: string  // 执行唯一 ID（用于日志关联）
  executor_id: number // 实际执行的执行器 ID
  status: string      // 执行状态，如 'running', 'success', 'failed'
  start_time: string | null   // 开始时间
  end_time: string | null     // 结束时间
  output: string      // 执行输出
  error: string       // 错误信息
  retry_times: number // 已重试次数
  created_at: string  // 创建时间
  progress: number    // 进度百分比
  progress_msg: string // 进度说明
  updated_at: string  // 更新时间
}

/** 任务日志列表接口使用的精简执行记录 */
export interface TaskExecutionListResponse {
  id: number          // 执行记录 ID
  task_id: number     // 任务 ID
  execution_id: string  // 执行唯一 ID
  executor_id: number // 执行器 ID
  executor_name: string | null    // 执行器名称（冗余字段方便展示）
  task_name: string | null        // 任务名称（冗余字段）
  task_type: string | null        // 任务类型（冗余字段）
  status: string
  start_time: string | null
  end_time: string | null
  output: string
  error: string
  retry_times: number
  created_at: string
}

/** 实时任务日志条目 */
export interface TaskLog {
  id: number          // 日志条目 ID
  execution_id: string  // 关联执行 ID
  task_id: number
  executor_id: number
  node_id: string     // 执行节点 ID
  log_level: string   // 后端内部日志级别
  level: string       // 对外展示级别：INFO/WARN/ERROR 等
  message: string     // 简短消息
  log_content: string // 完整日志正文
  log_time: string    // 日志时间
}

// ============================================================================
// 八、Webhook 回调配置相关类型
// ============================================================================

/** Webhook 回调配置 */
export interface Webhook {
  id: number              // Webhook ID
  name: string            // 名称
  url: string             // 回调 URL
  method: string          // HTTP 方法
  headers: string         // 请求头 JSON
  secret: string          // 签名密钥
  domain_id: number       // 所属领域
  domain_name: string     // 所属领域名称（冗余）
  user_permission: string // 当前用户对此 Webhook 的权限级别
  is_enabled: boolean     // 是否启用
  description: string     // 说明
  created_by: number | null
  created_at: string
  updated_at: string
}

// ============================================================================
// 九、Grafana 仪表盘嵌入配置相关类型（Dashboard 页面使用）
// ============================================================================

/** Grafana 图表嵌入配置（用于仪表盘） */
export interface GrafanaDashboard {
  id: number              // 图表配置 ID
  name: string            // 图表名称
  description: string     // 描述
  embed_url: string       // Grafana iframe 嵌入地址
  width: 'third' | 'half' | 'full'  // 宽度：1/3、1/2、全宽
  height_px: number       // 高度（像素）
  sort_order: number      // 展示顺序（升序）
  enabled: number         // 是否启用，0=禁用 1=启用（与后端 int 对齐）
  created_at: string
  updated_at: string
}

/** 创建/更新 Grafana 图表时使用的输入类型（不含系统字段） */
export interface GrafanaDashboardInput {
  name: string
  description?: string
  embed_url: string
  width: 'third' | 'half' | 'full'
  height_px: number
  sort_order: number
  enabled: number
}

/** Grafana 图表排序调整项 */
export interface GrafanaReorderItem {
  id: number              // 图表配置 ID
  sort_order: number      // 新的排序值
}

// ============================================================================
// 十、数据源（Datasource）及其权限相关类型
// ============================================================================

/** 数据源：数据库连接配置 */
export interface Datasource {
  id: number              // 数据源 ID
  name: string            // 名称
  type: string            // 数据库类型：mysql/postgres/oracle/sqlserver/mongodb/clickhouse/hive/rqlite/sqlite
  host: string            // 主机
  port: number            // 端口
  path: string            // 连接路径（如 SQLite 文件、RQLite 集群路径）
  database: string        // 默认数据库名
  username: string        // 用户名
  password: string        // 密码（加密存储）
  auth_type: string       // 认证方式
  connection_mode: string // 连接模式（直连/代理等）
  zk_hosts: string        // ZooKeeper 地址（Hive 等需要）
  zk_path: string         // ZooKeeper 路径
  rqlite_hosts: string    // RQLite 集群地址
  config: string          // 扩展配置 JSON
  description: string     // 说明
  domain_id: number       // 所属领域
  domain_name: string     // 领域名称（冗余）
  user_permission: string // 当前用户对此数据源的权限级别
  is_enabled: boolean     // 是否启用
  allow_write_sql: boolean // 是否允许执行写 SQL
  test_status: string     // 连接测试结果
  last_test_at: string | null  // 最近一次测试时间
  created_by: number
  updated_by: number
  created_by_name: string
  updated_by_name: string
  created_at: string
  updated_at: string
}

// ============================================================================
// 十一、权限分配辅助类型（DatasourcePermission / WebhookPermission / PermissionGroup）
// ============================================================================

/** 数据源级权限分配记录 */
export interface DatasourcePermission {
  id: number              // 记录 ID
  datasource_id: number   // 数据源 ID
  role_id: number | null  // 被授权角色（与 user_id 二选一）
  user_id: number | null  // 被授权用户（与 role_id 二选一）
  permission_type: string // 权限类型，如 'read', 'write', 'admin'
  granted_by: number | null  // 授权操作者 ID
  granted_at: string      // 授权时间
}

/** Webhook 级权限分配记录 */
export interface WebhookPermission {
  id: number
  webhook_id: number
  role_id: number | null
  user_id: number | null
  permission_type: string
  granted_by: number | null
  granted_at: string
}

/** 按资源分组的权限集合（用于权限分配界面） */
export interface PermissionGroup {
  resource: string                // 资源标识符
  resource_name: string           // 资源展示名
  permissions: Permission[]       // 该资源下的所有权限
}

/** 菜单项与后端资源的映射定义 —— 驱动侧边栏渲染和路由守卫权限判断 */
export interface MenuPermissionDef {
  key: string                     // 菜单唯一标识，对应路由 meta.menuPermission
  label: string                   // 菜单展示名
  icon: string                    // Element Plus 图标名
  path: string                    // 对应路由路径（父菜单可为空）
  resources: string[]             // 该菜单需要的后端资源，任一拥有即可见
  children?: MenuPermissionDef[]  // 子菜单
}

// ============================================================================
// 十二、通用工具类型（PaginatedResponse / ExecutorWithDomains / LoginRequest）
// ============================================================================

/** 统一分页响应包装 */
export interface PaginatedResponse<T> {
  items?: T[]      // 数据项（后端两种字段名二选一）
  data?: T[]       // 数据项（旧字段名）
  total: number    // 总条数
  page?: number    // 当前页码
  page_size?: number  // 每页大小
}

/** 带所属领域信息的执行器 */
export interface ExecutorWithDomains extends Executor {
  domains?: Domain[]  // 该执行器被分配到的领域
}

/** 登录请求体 */
export interface LoginRequest {
  username: string
  password: string
}

// ============================================================================
// 十三、仪表盘统计相关类型（DashboardStats / TrendData）
// ============================================================================

/** 仪表盘统计数据 */
export interface DashboardStats {
  tasks: {
    total: number      // 任务总数
    enabled: number    // 启用中的任务数
    cron: number       // Cron 类型任务数
    running: number    // 正在运行的任务数
    success: number    // 成功的任务数
    failed: number     // 失败的任务数
    avg_duration: number  // 平均执行时长（秒）
  }
  executors: {
    total: number      // 执行器总数
    active: number     // 活跃执行器数
    online: number     // 在线执行器数
    offline: number    // 离线执行器数
  }
  scheduler: {
    paused: boolean    // 调度器是否暂停
    uptime: number     // 运行时长（秒）
  }
}

/** 趋势图数据点（按日期聚合的任务执行情况） */
export interface TrendData {
  date: string        // 日期 YYYY-MM-DD
  total: number       // 当日执行总数
  success: number     // 成功数
  failed: number      // 失败数
}

// ============================================================================
// 十四、SQL 查询相关类型（QueryResult / QueryHistory / SavedSQL / TableInfo / ColumnInfo）
// ============================================================================

/** SQL 查询结果 */
export interface QueryResult {
  query_id?: string     // 查询 ID（异步执行时用于后续拉取）
  columns: string[]     // 列名列表
  rows: any[][]         // 数据行（二维数组）
  row_count: number     // 总行数
  execution_time?: number  // 执行耗时（毫秒）
  from_cache?: boolean  // 是否从缓存命中
}

/** 查询历史记录 */
export interface QueryHistory {
  id: number
  query_id?: string
  datasource_id?: number
  datasource_name?: string
  sql_text: string
  database?: string
  execution_time?: number
  row_count?: number
  status: string          // 查询结果状态：success/failed
  error_message?: string
  executed_by?: number
  executed_by_name?: string
  domain_id: number
  created_at: string
}

/** 用户保存的 SQL 片段 */
export interface SavedSQL {
  id: number
  name: string
  datasource_id: number
  datasource_name?: string
  database?: string
  sql_text: string
  description?: string
  created_by?: number
  created_by_name?: string
  updated_by?: number
  updated_by_name?: string
  domain_id: number
  is_public: boolean      // 是否领域内公开
  created_at: string
  updated_at: string
}

/** 数据源中的表信息 */
export interface TableInfo {
  name: string           // 表名
  comment?: string       // 表注释
}

/** 表中的列信息 */
export interface ColumnInfo {
  name: string           // 列名
  type: string           // 数据类型
  comment?: string       // 列注释
  nullable: boolean      // 是否允许 NULL
}

// ============================================================================
// 十五、系统运行时配置项相关类型
// ============================================================================

/** 系统运行时配置项 */
export interface SystemConfigItem {
  key: string            // 配置键
  label: string          // 展示名
  description: string    // 说明
  type: string           // 值类型：string/number/boolean
  default_value: string  // 默认值
  value: string          // 当前值
  min_value?: number | null  // 数值下限
  max_value?: number | null  // 数值上限
  unit?: string          // 单位
  group: string          // 所属分组
}
