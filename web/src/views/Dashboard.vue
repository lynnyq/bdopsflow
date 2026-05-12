<template>
  <div class="dashboard">
    <h1>仪表盘</h1>
    <el-row :gutter="20">
      <el-col :span="6">
        <el-card>
          <template #header>
            <span>总任务数</span>
          </template>
          <div class="stat-value">{{ stats.totalTasks }}</div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card>
          <template #header>
            <span>运行中</span>
          </template>
          <div class="stat-value running">{{ stats.runningTasks }}</div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card>
          <template #header>
            <span>成功</span>
          </template>
          <div class="stat-value success">{{ stats.successTasks }}</div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card>
          <template #header>
            <span>失败</span>
          </template>
          <div class="stat-value failed">{{ stats.failedTasks }}</div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="20" style="margin-top: 20px">
      <el-col :span="12">
        <el-card>
          <template #header>
            <span>执行器状态</span>
          </template>
          <div v-for="executor in executors" :key="executor.id" class="executor-item">
            <span>{{ executor.name }}</span>
            <el-tag :type="executor.status === 'online' ? 'success' : 'danger'">
              {{ executor.status === 'online' ? '在线' : '离线' }}
            </el-tag>
            <span class="load-info">负载: {{ executor.current_load }}/{{ executor.capacity }}</span>
          </div>
        </el-card>
      </el-col>
      <el-col :span="12">
        <el-card>
          <template #header>
            <span>最近执行记录</span>
          </template>
          <el-table :data="recentExecutions" style="width: 100%">
            <el-table-column prop="task_id" label="任务ID" width="80" />
            <el-table-column prop="status" label="状态" width="100">
              <template #default="{ row }">
                <el-tag :type="getStatusType(row.status)">{{ row.status }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="created_at" label="时间" />
          </el-table>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { taskAPI, executorAPI } from '@/api'
import type { Executor } from '@/types'

const stats = ref({
  totalTasks: 0,
  runningTasks: 0,
  successTasks: 0,
  failedTasks: 0,
})

const executors = ref<Executor[]>([])
const recentExecutions = ref<any[]>([])

const getStatusType = (status: string) => {
  switch (status) {
    case 'success':
      return 'success'
    case 'failed':
      return 'danger'
    case 'running':
      return 'warning'
    default:
      return 'info'
  }
}

onMounted(async () => {
  try {
    const tasksResponse = await taskAPI.list()
    const tasks = tasksResponse.data || []
    stats.value.totalTasks = tasks.length
    stats.value.runningTasks = tasks.filter((t) => t.status === 'running').length
    stats.value.successTasks = tasks.filter((t) => t.status === 'success').length
    stats.value.failedTasks = tasks.filter((t) => t.status === 'failed').length

    const executorsResponse = await executorAPI.list()
    executors.value = executorsResponse.data || []
  } catch (error) {
    console.error('Failed to load dashboard data:', error)
  }
})
</script>

<style scoped>
.dashboard h1 {
  margin-bottom: 20px;
}

.stat-value {
  font-size: 32px;
  font-weight: bold;
  text-align: center;
  padding: 20px;
}

.stat-value.running {
  color: #e6a23c;
}

.stat-value.success {
  color: #67c23a;
}

.stat-value.failed {
  color: #f56c6c;
}

.executor-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 0;
  border-bottom: 1px solid #eee;
}

.executor-item:last-child {
  border-bottom: none;
}

.load-info {
  color: #909399;
  font-size: 12px;
}
</style>
