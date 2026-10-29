/**
 * @file index.ts - Vue Router 路由配置
 * 定义所有前端页面的路由表，并通过 beforeEach 守卫实现认证校验、菜单权限过滤和
 * 细粒度 resource/action 权限校验。
 */
import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { menuPermissionMap } from '@/config/menuPermissionMap'
import type { MenuPermissionDef } from '@/types'

/** 全局路由表：根布局包含所有受保护的子路由 */
const routes: RouteRecordRaw[] = [
  {
    // 登录页 - 无需认证
    path: '/login',
    name: 'Login',
    component: () => import('@/views/Login.vue'),
    meta: { requiresAuth: false },
  },
  {
    // 主布局容器 - 包含所有受保护的子页面
    path: '/',
    component: () => import('@/views/Layout.vue'),
    children: [
      {
        // 仪表盘 - 首页概览
        path: '',
        name: 'Dashboard',
        component: () => import('@/views/Dashboard.vue'),
        meta: { menuPermission: 'dashboard' },
      },
      {
        // 任务管理 - HTTP/Shell 定时任务的增删改查
        path: 'tasks',
        name: 'Tasks',
        component: () => import('@/views/Tasks.vue'),
        meta: { menuPermission: 'task' },
      },
      {
        // 任务日志 - 历史执行记录与详情
        path: 'logs',
        name: 'Logs',
        component: () => import('@/views/Logs.vue'),
        meta: { menuPermission: 'log' },
      },
      {
        // 执行器管理 - 注册的执行节点列表与状态
        path: 'executors',
        name: 'Executors',
        component: () => import('@/views/Executors.vue'),
        meta: { menuPermission: 'executor' },
      },
      {
        // 个人中心 - 用户查看/编辑自己的资料，无菜单权限限制
        path: 'profile',
        name: 'Profile',
        component: () => import('@/views/Profile.vue'),
      },
      {
        // 数据源列表 - 数据库连接管理
        path: 'datasources',
        name: 'Datasources',
        component: () => import('@/views/Datasource/DatasourceList.vue'),
        meta: { menuPermission: 'datasource' },
      },
      {
        // 新建数据源
        path: 'datasources/create',
        name: 'CreateDatasource',
        component: () => import('@/views/Datasource/DatasourceForm.vue'),
        meta: { menuPermission: 'datasource', permission: { resource: 'datasource', action: 'create' } },
      },
      {
        // 编辑数据源
        path: 'datasources/:id/edit',
        name: 'EditDatasource',
        component: () => import('@/views/Datasource/DatasourceForm.vue'),
        meta: { menuPermission: 'datasource', permission: { resource: 'datasource', action: 'update' } },
      },
      {
        // 数据源权限分配
        path: 'datasources/:id/permissions',
        name: 'DatasourcePermission',
        component: () => import('@/views/Datasource/DatasourcePermission.vue'),
        meta: { menuPermission: 'datasource', permission: { resource: 'datasource', action: 'manage' } },
      },
      {
        // SQL 查询 - 在线 SQL 编辑器
        path: 'sql-query',
        name: 'SQLQuery',
        component: () => import('@/views/SQLQuery/SQLQuery.vue'),
        meta: { menuPermission: 'sql-query' },
      },
      {
        // 已保存 SQL - 用户收藏的查询语句
        path: 'saved-sql',
        name: 'SavedSQL',
        component: () => import('@/views/SQLQuery/SavedSQLList.vue'),
        meta: { menuPermission: 'saved-sql' },
      },
      {
        // 查询历史 - 所有用户执行过的 SQL 记录
        path: 'query-history',
        name: 'QueryHistory',
        component: () => import('@/views/SQLQuery/QueryHistory.vue'),
        meta: { menuPermission: 'query-history' },
      },
      {
        // HTTP 接口测试
        path: 'interface/http',
        name: 'HttpTest',
        component: () => import('@/views/ApiTest/HttpTest.vue'),
        meta: { menuPermission: 'interface-http' },
      },
      {
        // gRPC 接口测试
        path: 'interface/grpc',
        name: 'GrpcTest',
        component: () => import('@/views/ApiTest/GrpcTest.vue'),
        meta: { menuPermission: 'interface-grpc' },
      },
      {
        // Proto 文件管理
        path: 'interface/proto-files',
        name: 'ProtoFiles',
        component: () => import('@/views/ApiTest/ProtoFiles.vue'),
        meta: { menuPermission: 'interface-proto' },
      },
      {
        // 接口测试证书管理
        path: 'interface/certificates',
        name: 'Certificates',
        component: () => import('@/views/ApiTest/Certificates.vue'),
        meta: { menuPermission: 'interface-cert' },
      },
      {
        // 接口测试执行历史
        path: 'interface/history',
        name: 'TestHistory',
        component: () => import('@/views/ApiTest/TestHistory.vue'),
        meta: { menuPermission: 'interface-history' },
      },
      {
        // 用户管理 - 系统管理员专属
        path: 'admin/users',
        name: 'AdminUsers',
        component: () => import('@/views/admin/Users.vue'),
        meta: { menuPermission: 'user-management' },
      },
      {
        // 角色管理 - RBAC 角色与权限分配
        path: 'admin/roles',
        name: 'AdminRoles',
        component: () => import('@/views/admin/Roles.vue'),
        meta: { menuPermission: 'role-management' },
      },
      {
        // 领域管理 - 多租户隔离单元
        path: 'admin/domains',
        name: 'AdminDomains',
        component: () => import('@/views/admin/Domains.vue'),
        meta: { menuPermission: 'domain-management' },
      },
      {
        // Webhook 管理 - 任务回调通知配置
        path: 'admin/webhooks',
        name: 'WebhookManagement',
        component: () => import('@/views/admin/WebhookManagement.vue'),
        meta: { menuPermission: 'webhook-management' },
      },
      {
        // Grafana 仪表盘嵌入管理
        path: 'admin/grafana-dashboards',
        name: 'GrafanaDashboards',
        component: () => import('@/views/admin/GrafanaDashboards.vue'),
        meta: { menuPermission: 'grafana-dashboard-management' },
      },
      {
        // 系统配置 - 运行时参数设置
        path: 'admin/system-config',
        name: 'SystemConfig',
        component: () => import('@/views/admin/SystemConfig.vue'),
        meta: { menuPermission: 'system-config' },
      },
      {
        // 审计日志 - 所有用户的敏感操作记录
        path: 'admin/audit-logs',
        name: 'AuditLogs',
        component: () => import('@/views/admin/AuditLogs.vue'),
        meta: { menuPermission: 'audit-log' },
      },
    ],
  },
]

/** 创建 router 实例，使用 HTML5 History 模式 */
const router = createRouter({
  history: createWebHistory(),
  routes,
})

/**
 * 在菜单权限映射中根据 key 查找对应的菜单项定义，支持父子两层嵌套。
 * @param key - 菜单唯一标识
 * @returns 匹配到的菜单定义，未找到返回 undefined
 */
function findMenuDefByKey(key: string): MenuPermissionDef | undefined {
  for (const menu of menuPermissionMap) {
    if (menu.key === key) return menu
    if (menu.children) {
      for (const child of menu.children) {
        if (child.key === key) return child
      }
    }
  }
  return undefined
}

/**
 * 递归判断某个菜单项（或其任意子菜单）是否对当前用户可见。
 * 父菜单只要有一个子菜单可见就视为可见；叶子菜单则检查其 resources 是否有匹配权限。
 * @param menu - 菜单定义
 * @param hasAnyPermission - 判断是否拥有某资源任意权限的函数
 * @returns 当前菜单是否可见
 */
function deriveMenuVisibility(menu: MenuPermissionDef, hasAnyPermission: (resource: string) => boolean): boolean {
  if (menu.children && menu.children.length > 0) {
    return menu.children.some(child => deriveMenuVisibility(child, hasAnyPermission))
  }
  return menu.resources.some(r => hasAnyPermission(r))
}

/**
 * 全局前置守卫：在每次路由跳转前执行以下检查：
 * 1. 未标记 requiresAuth=false 的页面必须携带有效 token，否则跳转登录页；
 * 2. 首次进入时若 store 中无用户信息，则调用 fetchCurrentUser 拉取；
 * 3. 校验目标页面对应的菜单是否对当前用户可见，不可见则跳转首页/个人中心；
 * 4. 校验 meta.permission 中声明的细粒度 resource/action 权限。
 */
router.beforeEach(async (to, _from, next) => {
  if (to.meta.requiresAuth === false) {
    next()
    return
  }

  const authStore = useAuthStore()
  const token = sessionStorage.getItem('token')

  if (!token) {
    next('/login')
    return
  }

  if (!authStore.user) {
    await authStore.fetchCurrentUser()
  }

  if (!authStore.token) {
    next('/login')
    return
  }

  const menuPermission = to.meta.menuPermission as string | undefined
  if (menuPermission) {
    const menuDef = findMenuDefByKey(menuPermission)
    if (menuDef && !deriveMenuVisibility(menuDef, authStore.hasAnyPermission.bind(authStore))) {
      if (to.path === '/') {
        next({ name: 'Profile' })
        return
      }
      next('/')
      return
    }
  }

  const permission = to.meta.permission as { resource: string; action: string } | undefined
  if (permission) {
    if (!authStore.hasPermission(permission.resource, permission.action)) {
      if (to.path === '/') {
        next({ name: 'Profile' })
        return
      }
      next('/')
      return
    }
  }

  next()
})

export default router
