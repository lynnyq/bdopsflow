/**
 * @file menuPermissionMap.ts - 菜单与后端资源的权限映射配置
 *
 * 驱动前端侧边栏菜单渲染和路由守卫权限判断的核心配置：
 *  - 每个菜单项（MenuPermissionDef）声明了自己需要哪些后端 resources，
 *    路由守卫会根据当前用户的 permissions 自动过滤不可见的菜单和页面；
 *  - 父菜单只要 children 中有任意一个对用户可见，整个父菜单就会显示；
 *  - key 字段与 router/index.ts 中 route.meta.menuPermission 一一对应，
 *    用于在 beforeEach 守卫里查找菜单定义、推导可见性。
 *
 * 修改此文件时需同步更新 router/index.ts 中的 meta.menuPermission 取值。
 */
import type { MenuPermissionDef } from '@/types'

/**
 * 全局菜单权限映射数组（顺序决定侧边栏展示顺序）
 * 每个 MenuPermissionDef 的字段含义：
 *   - key:       菜单唯一标识，用于路由 meta.menuPermission 关联
 *   - label:     侧边栏展示的中文名称
 *   - icon:      Element Plus 图标名
 *   - path:      对应路由路径（父菜单可为空字符串）
 *   - resources: 该菜单绑定的后端 resource 标识符数组；
 *                用户对其中任意一个 resource 拥有任何权限，该菜单对用户可见
 *   - children:  子菜单数组；父菜单的 resources 通常会被各子项覆盖
 */
export const menuPermissionMap: MenuPermissionDef[] = [
  {
    key: 'dashboard',
    label: '仪表盘',
    icon: 'DataAnalysis',
    path: '/',
    resources: ['dashboard'],  // 需要 dashboard 资源的任意权限
  },
  {
    key: 'task',
    label: '任务管理',
    icon: 'List',
    path: '/tasks',
    resources: ['task'],  // 需要 task 资源的任意权限
  },
  {
    key: 'log',
    label: '任务日志',
    icon: 'Document',
    path: '/logs',
    resources: ['log'],  // 需要 log 资源的任意权限
  },
  {
    key: 'executor',
    label: '执行器',
    icon: 'Cpu',
    path: '/executors',
    resources: ['executor'],  // 需要 executor 资源的任意权限
  },
  {
    // "数据查询" 父菜单：数据源管理 / SQL 查询 / 查询历史 / 已保存 SQL
    key: 'data-query',
    label: '数据查询',
    icon: 'Search',
    path: '',
    resources: ['datasource'],  // 父菜单本身不渲染，只用来聚合子项；实际权限由子项各自的 resources 决定
    children: [
      {
        key: 'datasource',
        label: '数据源管理',
        icon: 'Connection',
        path: '/datasources',
        resources: ['datasource'],
      },
      {
        key: 'sql-query',
        label: 'SQL 查询',
        icon: 'Search',
        path: '/sql-query',
        resources: ['datasource'],
      },
      {
        key: 'query-history',
        label: '查询历史',
        icon: 'List',
        path: '/query-history',
        resources: ['datasource'],
      },
      {
        key: 'saved-sql',
        label: '已保存 SQL',
        icon: 'Document',
        path: '/saved-sql',
        resources: ['datasource'],
      },
    ],
  },
  {
    // "接口测试" 父菜单：HTTP / gRPC / Proto / 证书 / 执行历史
    key: 'interface',
    label: '接口测试',
    icon: 'Monitor',
    path: '',
    resources: ['api_test'],
    children: [
      {
        key: 'interface-http',
        label: 'HTTP 测试',
        icon: 'Connection',
        path: '/interface/http',
        resources: ['api_test'],
      },
      {
        key: 'interface-grpc',
        label: 'gRPC 测试',
        icon: 'Connection',
        path: '/interface/grpc',
        resources: ['api_test'],
      },
      {
        key: 'interface-proto',
        label: 'Proto 文件',
        icon: 'Document',
        path: '/interface/proto-files',
        resources: ['api_test'],
      },
      {
        key: 'interface-cert',
        label: '证书管理',
        icon: 'Key',
        path: '/interface/certificates',
        resources: ['api_test'],
      },
      {
        key: 'interface-history',
        label: '执行历史',
        icon: 'Clock',
        path: '/interface/history',
        resources: ['api_test'],
      },
    ],
  },
  {
    // "系统管理" 父菜单：用户 / 角色 / 领域 / Webhook / Grafana / 系统配置 / 审计日志
    key: 'system-admin',
    label: '系统管理',
    icon: 'Setting',
    path: '',
    resources: ['user', 'role', 'domain', 'config', 'audit_log', 'webhook'],
    children: [
      {
        key: 'user-management',
        label: '用户管理',
        icon: 'User',
        path: '/admin/users',
        resources: ['user'],
      },
      {
        key: 'role-management',
        label: '角色管理',
        icon: 'Key',
        path: '/admin/roles',
        resources: ['role'],
      },
      {
        key: 'domain-management',
        label: '领域管理',
        icon: 'Grid',
        path: '/admin/domains',
        resources: ['domain'],
      },
      {
        key: 'webhook-management',
        label: 'Webhook管理',
        icon: 'Connection',
        path: '/admin/webhooks',
        resources: ['webhook'],
      },
      {
        key: 'grafana-dashboard-management',
        label: 'Grafana 图表',
        icon: 'DataLine',
        path: '/admin/grafana-dashboards',
        resources: ['config'],
      },
      {
        key: 'system-config',
        label: '系统配置',
        icon: 'Operation',
        path: '/admin/system-config',
        resources: ['config'],
      },
      {
        key: 'audit-log',
        label: '审计日志',
        icon: 'Notebook',
        path: '/admin/audit-logs',
        resources: ['audit_log'],
      },
    ],
  },
]
