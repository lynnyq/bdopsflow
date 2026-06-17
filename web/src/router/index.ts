import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { menuPermissionMap } from '@/config/menuPermissionMap'
import type { MenuPermissionDef } from '@/types'

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/Login.vue'),
    meta: { requiresAuth: false },
  },
  {
    path: '/',
    component: () => import('@/views/Layout.vue'),
    children: [
      {
        path: '',
        name: 'Dashboard',
        component: () => import('@/views/Dashboard.vue'),
        meta: { menuPermission: 'dashboard' },
      },
      {
        path: 'tasks',
        name: 'Tasks',
        component: () => import('@/views/Tasks.vue'),
        meta: { menuPermission: 'task' },
      },
      {
        path: 'logs',
        name: 'Logs',
        component: () => import('@/views/Logs.vue'),
        meta: { menuPermission: 'log' },
      },
      {
        path: 'executors',
        name: 'Executors',
        component: () => import('@/views/Executors.vue'),
        meta: { menuPermission: 'executor' },
      },
      {
        path: 'profile',
        name: 'Profile',
        component: () => import('@/views/Profile.vue'),
      },
      {
        path: 'datasources',
        name: 'Datasources',
        component: () => import('@/views/Datasource/DatasourceList.vue'),
        meta: { menuPermission: 'datasource' },
      },
      {
        path: 'datasources/create',
        name: 'CreateDatasource',
        component: () => import('@/views/Datasource/DatasourceForm.vue'),
        meta: { menuPermission: 'datasource', permission: { resource: 'datasource', action: 'create' } },
      },
      {
        path: 'datasources/:id/edit',
        name: 'EditDatasource',
        component: () => import('@/views/Datasource/DatasourceForm.vue'),
        meta: { menuPermission: 'datasource', permission: { resource: 'datasource', action: 'update' } },
      },
      {
        path: 'datasources/:id/permissions',
        name: 'DatasourcePermission',
        component: () => import('@/views/Datasource/DatasourcePermission.vue'),
        meta: { menuPermission: 'datasource', permission: { resource: 'datasource', action: 'manage' } },
      },
      {
        path: 'sql-query',
        name: 'SQLQuery',
        component: () => import('@/views/SQLQuery/SQLQuery.vue'),
        meta: { menuPermission: 'sql-query' },
      },
      {
        path: 'saved-sql',
        name: 'SavedSQL',
        component: () => import('@/views/SQLQuery/SavedSQLList.vue'),
        meta: { menuPermission: 'saved-sql' },
      },
      {
        path: 'query-history',
        name: 'QueryHistory',
        component: () => import('@/views/SQLQuery/QueryHistory.vue'),
        meta: { menuPermission: 'query-history' },
      },
      {
        path: 'interface/http',
        name: 'HttpTest',
        component: () => import('@/views/ApiTest/HttpTest.vue'),
        meta: { menuPermission: 'interface-http' },
      },
      {
        path: 'interface/grpc',
        name: 'GrpcTest',
        component: () => import('@/views/ApiTest/GrpcTest.vue'),
        meta: { menuPermission: 'interface-grpc' },
      },
      {
        path: 'interface/proto-files',
        name: 'ProtoFiles',
        component: () => import('@/views/ApiTest/ProtoFiles.vue'),
        meta: { menuPermission: 'interface-proto' },
      },
      {
        path: 'interface/certificates',
        name: 'Certificates',
        component: () => import('@/views/ApiTest/Certificates.vue'),
        meta: { menuPermission: 'interface-cert' },
      },
      {
        path: 'interface/history',
        name: 'TestHistory',
        component: () => import('@/views/ApiTest/TestHistory.vue'),
        meta: { menuPermission: 'interface-history' },
      },
      {
        path: 'admin/users',
        name: 'AdminUsers',
        component: () => import('@/views/admin/Users.vue'),
        meta: { menuPermission: 'user-management' },
      },
      {
        path: 'admin/roles',
        name: 'AdminRoles',
        component: () => import('@/views/admin/Roles.vue'),
        meta: { menuPermission: 'role-management' },
      },
      {
        path: 'admin/domains',
        name: 'AdminDomains',
        component: () => import('@/views/admin/Domains.vue'),
        meta: { menuPermission: 'domain-management' },
      },
      {
        path: 'admin/webhooks',
        name: 'WebhookManagement',
        component: () => import('@/views/admin/WebhookManagement.vue'),
        meta: { menuPermission: 'webhook-management' },
      },
      {
        path: 'admin/system-config',
        name: 'SystemConfig',
        component: () => import('@/views/admin/SystemConfig.vue'),
        meta: { menuPermission: 'system-config' },
      },
      {
        path: 'admin/audit-logs',
        name: 'AuditLogs',
        component: () => import('@/views/admin/AuditLogs.vue'),
        meta: { menuPermission: 'audit-log' },
      },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

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

function deriveMenuVisibility(menu: MenuPermissionDef, hasAnyPermission: (resource: string) => boolean): boolean {
  if (menu.children && menu.children.length > 0) {
    return menu.children.some(child => deriveMenuVisibility(child, hasAnyPermission))
  }
  return menu.resources.some(r => hasAnyPermission(r))
}

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
