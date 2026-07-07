export interface User {
  id: number
  username: string
  real_name: string
  phone: string
  email: string
  is_active: boolean
  last_login_at: string | null
  created_by?: number
  created_at: string
  updated_at: string
  role_codes?: string[]
  role_ids?: number[]
  domain_ids?: number[]
}

export interface Permission {
  id: number
  resource: string
  action: string
  description: string
}

export interface DomainInfo {
  domain_id: number
  domain_name: string
  is_default: boolean
}

export interface LoginResponse {
  token: string
  refresh_token: string
  user: User
  permissions: Permission[]
  domains: DomainInfo[]
  current_domain_id: number
  role_codes: string[]
}

export interface CurrentUserResponse {
  user: User
  permissions: Permission[]
  domains: DomainInfo[]
  current_domain_id: number
  role_codes: string[]
}

export interface SwitchDomainResponse {
  token: string
  refresh_token: string
  permissions: Permission[]
  current_domain_id: number
  role_codes: string[]
}

export interface Role {
  id: number
  name: string
  code: string
  description: string
  is_system: boolean
  parent_id: number | null
  domain_id: number | null
  created_at: string
  updated_at: string
  permissions?: Permission[]
}

export interface Domain {
  id: number
  name: string
  description: string
  created_at: string
  updated_at: string
}

export interface DomainWithStats extends Domain {
  user_count: number
  executor_count: number
  task_count: number
}

export interface Task {
  id: number
  name: string
  type: string
  config: string
  cron_expression: string
  timeout_seconds: number
  retry_count: number
  retry_interval: number
  is_enabled: boolean
  status: string
  domain_id: number
  webhook_id: number | null
  webhook_events: string
  assigned_executor_id: number | null
  has_available_executors: boolean
  created_by: number
  created_by_name: string
  created_at: string
  updated_at: string
  next_execution_time: string
  last_execution_status: string
}

export interface TaskConfig {
  url?: string
  method?: string
  timeout?: number
  headers?: string
  body?: string
  script?: string
}

export interface Executor {
  id: number
  name: string
  address: string
  status: string
  last_heartbeat: string | null
  capacity: number
  current_load: number
  is_global: boolean
  created_at: string
  updated_at: string
}

export interface TaskExecution {
  id: number
  task_id: number
  execution_id: string
  executor_id: number
  status: string
  start_time: string | null
  end_time: string | null
  output: string
  error: string
  retry_times: number
  created_at: string
  progress: number
  progress_msg: string
  updated_at: string
}

export interface TaskExecutionListResponse {
  id: number
  task_id: number
  execution_id: string
  executor_id: number
  executor_name: string | null
  task_name: string | null
  task_type: string | null
  status: string
  start_time: string | null
  end_time: string | null
  output: string
  error: string
  retry_times: number
  created_at: string
}

export interface TaskLog {
  id: number
  execution_id: string
  task_id: number
  executor_id: number
  node_id: string
  log_level: string
  level: string
  message: string
  log_content: string
  log_time: string
}

export interface Webhook {
  id: number
  name: string
  url: string
  method: string
  headers: string
  secret: string
  domain_id: number
  domain_name: string
  user_permission: string
  is_enabled: boolean
  description: string
  created_by: number | null
  created_at: string
  updated_at: string
}

export interface Datasource {
  id: number
  name: string
  type: string
  host: string
  port: number
  path: string
  database: string
  username: string
  password: string
  auth_type: string
  connection_mode: string
  zk_hosts: string
  zk_path: string
  rqlite_hosts: string
  config: string
  description: string
  domain_id: number
  domain_name: string
  user_permission: string
  is_enabled: boolean
  allow_write_sql: boolean
  test_status: string
  last_test_at: string | null
  created_by: number
  updated_by: number
  created_by_name: string
  updated_by_name: string
  created_at: string
  updated_at: string
}

export interface DatasourcePermission {
  id: number
  datasource_id: number
  role_id: number | null
  user_id: number | null
  permission_type: string
  granted_by: number | null
  granted_at: string
}

export interface WebhookPermission {
  id: number
  webhook_id: number
  role_id: number | null
  user_id: number | null
  permission_type: string
  granted_by: number | null
  granted_at: string
}

export interface PermissionGroup {
  resource: string
  resource_name: string
  permissions: Permission[]
}

export interface MenuPermissionDef {
  key: string
  label: string
  icon: string
  path: string
  resources: string[]
  children?: MenuPermissionDef[]
}

export interface PaginatedResponse<T> {
  items?: T[]
  data?: T[]
  total: number
  page?: number
  page_size?: number
}

export interface ExecutorWithDomains extends Executor {
  domains?: Domain[]
}

export interface LoginRequest {
  username: string
  password: string
}

export interface DashboardStats {
  tasks: {
    total: number
    enabled: number
    cron: number
    running: number
    success: number
    failed: number
    avg_duration: number
  }
  executors: {
    total: number
    active: number
    online: number
    offline: number
  }
  scheduler: {
    paused: boolean
    uptime: number
  }
}

export interface TrendData {
  date: string
  total: number
  success: number
  failed: number
}

export interface QueryResult {
  query_id?: string
  columns: string[]
  rows: any[][]
  row_count: number
  execution_time?: number
  from_cache?: boolean
}

export interface QueryHistory {
  id: number
  query_id?: string
  datasource_id?: number
  datasource_name?: string
  sql_text: string
  database?: string
  execution_time?: number
  row_count?: number
  status: string
  error_message?: string
  executed_by?: number
  executed_by_name?: string
  domain_id: number
  created_at: string
}

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
  is_public: boolean
  created_at: string
  updated_at: string
}

export interface TableInfo {
  name: string
  comment?: string
}

export interface ColumnInfo {
  name: string
  type: string
  comment?: string
  nullable: boolean
}

export interface SystemConfigItem {
  key: string
  label: string
  description: string
  type: string
  default_value: string
  value: string
  min_value?: number | null
  max_value?: number | null
  unit?: string
  group: string
}
