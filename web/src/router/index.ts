import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

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
    meta: { requiresAuth: true },
    children: [
      {
        path: '',
        name: 'Dashboard',
        component: () => import('@/views/Dashboard.vue'),
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
        meta: { menuPermission: 'sql_query' },
      },
      {
        path: 'saved-sql',
        name: 'SavedSQL',
        component: () => import('@/views/SQLQuery/SavedSQLList.vue'),
        meta: { menuPermission: 'saved_sql' },
      },
      {
        path: 'query-history',
        name: 'QueryHistory',
        component: () => import('@/views/SQLQuery/QueryHistory.vue'),
        meta: { menuPermission: 'query_history' },
      },
      {
        path: 'admin/users',
        name: 'AdminUsers',
        component: () => import('@/views/admin/Users.vue'),
        meta: { menuPermission: 'user_management' },
      },
      {
        path: 'admin/roles',
        name: 'AdminRoles',
        component: () => import('@/views/admin/Roles.vue'),
        meta: { menuPermission: 'role_management' },
      },
      {
        path: 'admin/domains',
        name: 'AdminDomains',
        component: () => import('@/views/admin/Domains.vue'),
        meta: { menuPermission: 'domain_management' },
      },
      {
        path: 'admin/system-config',
        name: 'SystemConfig',
        component: () => import('@/views/admin/SystemConfig.vue'),
        meta: { menuPermission: 'system_config' },
      },
      {
        path: 'admin/audit-logs',
        name: 'AuditLogs',
        component: () => import('@/views/admin/AuditLogs.vue'),
        meta: { menuPermission: 'audit_log' },
      },
      {
        path: 'admin/webhooks',
        name: 'WebhookManagement',
        component: () => import('@/views/admin/WebhookManagement.vue'),
        meta: { requiresAdmin: true },
      },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach(async (to, from, next) => {
  const token = localStorage.getItem('token')
  if (to.meta.requiresAuth !== false && !token) {
    next('/login')
    return
  }

  if (to.meta.menuPermission || to.meta.permission) {
    const authStore = useAuthStore()
    if (!authStore.user && token) {
      await authStore.fetchCurrentUser()
    }

    if (to.meta.menuPermission) {
      const menuAction = to.meta.menuPermission as string
      if (!authStore.hasMenuPermission(menuAction)) {
        next('/')
        return
      }
    }

    if (to.meta.permission) {
      const { resource, action } = to.meta.permission as { resource: string; action: string }
      if (!authStore.hasPermission(resource, action)) {
        next('/')
        return
      }
    }
  }

  next()
})

export default router
