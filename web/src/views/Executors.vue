<template>
  <div class="executors-container">
    <h2>执行器管理</h2>

    <el-table :data="executors" style="width: 100%" v-loading="loading">
      <el-table-column prop="executor_id" label="ID" width="150" />
      <el-table-column prop="name" label="名称" width="150" />
      <el-table-column prop="address" label="地址" width="200" />
      <el-table-column prop="status" label="状态" width="120">
        <template #default="{ row }">
          <el-tag :type="row.status === 'online' ? 'success' : 'danger'">
            {{ row.status === 'online' ? '在线' : '离线' }}
          </el-tag>
          <el-button 
            type="success" 
            size="small" 
            :disabled="row.status === 'online'"
            @click="handleChangeStatus(row, 'online')"
            style="margin-left: 5px;">
            上线
          </el-button>
          <el-button 
            type="warning" 
            size="small" 
            :disabled="row.status === 'offline'"
            @click="handleChangeStatus(row, 'offline')"
            style="margin-left: 5px;">
            下线
          </el-button>
        </template>
      </el-table-column>
      <el-table-column prop="last_heartbeat" label="最后心跳" width="180">
        <template #default="{ row }">
          <span v-if="row.last_heartbeat">
            {{ formatLocalTime(row.last_heartbeat) }}
          </span>
          <span v-else class="text-muted">无</span>
        </template>
      </el-table-column>
      <el-table-column label="负载" width="150">
        <template #default="{ row }">
          <el-progress
            :percentage="getLoadPercentage(row)"
            :color="getLoadColor(getLoadPercentage(row))"
          />
          <span class="load-text">{{ row.current_load }}/{{ row.capacity }}</span>
        </template>
      </el-table-column>
      <el-table-column prop="created_at" label="注册时间" width="180">
        <template #default="{ row }">
          {{ formatLocalTime(row.created_at) }}
        </template>
      </el-table-column>
      <el-table-column label="操作" width="100" fixed="right">
        <template #default="{ row }">
          <el-button type="danger" size="small" @click="handleDelete(row)">
            删除
          </el-button>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { executorAPI } from '@/api'
import type { Executor } from '@/types'

const executors = ref<Executor[]>([])
const loading = ref(false)

const getLoadPercentage = (row: Executor) => {
  if (!row.capacity || row.capacity === 0) return 0
  return Math.min(100, Math.round((row.current_load / row.capacity) * 100))
}

const getLoadColor = (ratio: number) => {
  if (ratio < 50) return '#67c23a'
  if (ratio < 80) return '#e6a23c'
  return '#f56c6c'
}

const formatLocalTime = (timeStr: string) => {
  if (!timeStr) return '无'
  try {
    const date = new Date(timeStr)
    const year = date.getFullYear()
    const month = String(date.getMonth() + 1).padStart(2, '0')
    const day = String(date.getDate()).padStart(2, '0')
    const hours = String(date.getHours()).padStart(2, '0')
    const minutes = String(date.getMinutes()).padStart(2, '0')
    const seconds = String(date.getSeconds()).padStart(2, '0')
    return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`
  } catch {
    return timeStr
  }
}

const handleChangeStatus = async (row: Executor, status: string) => {
  try {
    await ElMessageBox.confirm(
      `确定要将执行器 "${row.executor_id}" 设置为${status === 'online' ? '上线' : '下线'}状态吗？`,
      '状态变更确认',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning',
      }
    )

    if (status === 'online') {
      await executorAPI.online(row.executor_id)
      ElMessage.success('上线成功')
    } else {
      await executorAPI.offline(row.executor_id)
      ElMessage.success('下线成功')
    }
    loadExecutors()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error('状态变更失败')
    }
  }
}

const handleDelete = async (row: Executor) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除执行器 "${row.executor_id}" 吗？`,
      '删除确认',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning',
      }
    )

    await executorAPI.delete(row.executor_id)
    ElMessage.success('删除成功')
    loadExecutors()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error('删除失败')
    }
  }
}

const loadExecutors = async () => {
  loading.value = true
  try {
    const response = await executorAPI.list()
    executors.value = response.data || []
  } catch (error) {
    ElMessage.error('加载执行器列表失败')
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadExecutors()
})
</script>

<style scoped>
.executors-container {
  padding: 20px;
}

.executors-container h2 {
  margin-bottom: 20px;
}

.load-text {
  font-size: 12px;
  color: #909399;
  margin-top: 5px;
}

.text-muted {
  color: #909399;
}
</style>
