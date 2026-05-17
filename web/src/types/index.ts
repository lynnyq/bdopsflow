export interface User {
  id: number
  username: string
  email?: string
  role: string
  domain_id: number
  is_active: boolean
  last_login_at: string | null
  created_by: number
  created_at: string
  updated_at: string
}

export interface Task {
  id: number
  workflow_id: number | null
  name: string
  type: string
  config: string | TaskConfig
  cron_expression: string
  timeout_seconds: number
  retry_count: number
  retry_interval: number
  is_enabled: boolean
  status: string
  domain_id: number
  webhook_config: string
  assigned_executor_id: number
  created_by: number
  created_at: string
  updated_at: string
  next_execution_time?: string
  last_execution_status?: string
}

export interface TaskConfig {
  url?: string
  method?: string
  timeout?: number
  headers?: string
  body?: string
  script?: string
}

export interface Workflow {
  id: number
  name: string
  description: string
  domain_id: number
  dag_config: string
  cron_expression: string
  is_enabled: boolean
  created_by: number
  created_at: string
  updated_at: string
}

export interface WorkflowNodeConfig {
  url?: string
  method?: string
  script?: string
  delay?: number
  timeout?: number
  condition?: string
  trigger?: 'completed' | 'failed' | 'all'
}

export interface WorkflowNode {
  id: string
  name: string
  type: 'http' | 'shell' | 'delay' | 'condition' | 'webhook'
  x: number
  y: number
  status: 'pending' | 'running' | 'success' | 'failed'
  config?: WorkflowNodeConfig
}

export interface WorkflowConnection {
  id: string
  from: string
  to: string
}

export interface WorkflowDAG {
  nodes: WorkflowNode[]
  connections: WorkflowConnection[]
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

export interface PaginatedResponse<T> {
  data: T[]
  total: number
  page: number
  page_size: number
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

export interface Domain {
  id: number
  name: string
  description?: string
  created_at: string
  updated_at: string
}

export interface ExecutorWithDomains extends Executor {
  domains?: Domain[]
}

export interface LoginRequest {
  username: string
  password: string
}

export interface LoginResponse {
  token: string
  user: User
}

export interface WorkflowExecution {
  id: number
  workflow_id: number
  execution_id: string
  status: string
  start_time: string | null
  end_time: string | null
  node_states: string
  created_at: string
}

export interface TaskLog {
  id: number
  execution_id: string
  task_id: number
  executor_id: number
  node_id: string
  log_level: string
  message: string
  log_time: string
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
  workflows: {
    total: number
    enabled: number
  }
  executors: {
    total: number
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
