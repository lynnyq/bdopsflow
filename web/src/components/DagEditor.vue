<template>
  <div class="dag-editor">
    <div class="editor-toolbar">
      <div class="toolbar-left">
        <div class="toolbar-group">
          <el-button-group>
            <el-button :icon="Plus" @click="addNode('http')" size="small">
              <span class="btn-label">HTTP</span>
            </el-button>
            <el-button :icon="Monitor" @click="addNode('shell')" size="small">
              <span class="btn-label">脚本</span>
            </el-button>
            <el-button :icon="Clock" @click="addNode('delay')" size="small">
              <span class="btn-label">延迟</span>
            </el-button>
          </el-button-group>
        </div>
        <div class="toolbar-divider"></div>
        <el-button
          :icon="Delete"
          type="danger"
          text
          size="small"
          @click="deleteSelected"
          :disabled="!selectedNode"
        >
          删除节点
        </el-button>
      </div>

      <div class="toolbar-center">
        <div class="zoom-controls">
          <el-button :icon="ZoomOut" size="small" circle @click="zoomOut" />
          <span class="zoom-level">{{ Math.round(scale * 100) }}%</span>
          <el-button :icon="ZoomIn" size="small" circle @click="zoomIn" />
          <el-button :icon="FullScreen" size="small" circle @click="resetView" />
        </div>
      </div>

      <div class="toolbar-right">
        <el-button :icon="Refresh" size="small" circle @click="clearAll" />
        <el-button type="primary" :icon="VideoPlay" size="small" @click="runWorkflow">
          执行
        </el-button>
      </div>
    </div>

    <div class="editor-container">
      <div
        ref="canvasRef"
        class="dag-canvas"
        @mousedown="handleCanvasMouseDown"
        @mousemove="handleMouseMove"
        @mouseup="handleMouseUp"
        @wheel="handleWheel"
      >
        <svg
          ref="svgRef"
          class="connection-layer"
          :viewBox="`0 0 ${canvasWidth} ${canvasHeight}`"
          preserveAspectRatio="xMidYMid meet"
        >
          <defs>
            <linearGradient id="connectionGradient" x1="0%" y1="0%" x2="100%" y2="0%">
              <stop offset="0%" stop-color="#06b6d4" />
              <stop offset="100%" stop-color="#8b5cf6" />
            </linearGradient>
            <filter id="glow">
              <feGaussianBlur stdDeviation="3" result="coloredBlur" />
              <feMerge>
                <feMergeNode in="coloredBlur" />
                <feMergeNode in="SourceGraphic" />
              </feMerge>
            </filter>
            <marker
              id="arrowhead"
              markerWidth="10"
              markerHeight="7"
              refX="9"
              refY="3.5"
              orient="auto"
            >
              <polygon points="0 0, 10 3.5, 0 7" fill="#06b6d4" />
            </marker>
            <marker
              id="arrowhead-hover"
              markerWidth="10"
              markerHeight="7"
              refX="9"
              refY="3.5"
              orient="auto"
            >
              <polygon points="0 0, 10 3.5, 0 7" fill="#06b6d4" filter="url(#glow)" />
            </marker>
          </defs>

          <g :transform="`translate(${panX}, ${panY}) scale(${scale})`">
            <path
              v-for="conn in connections"
              :key="conn.id"
              :d="getConnectionPath(conn)"
              :class="['dag-connection', { active: hoveredConnection === conn.id }]"
              :marker-end="hoveredConnection === conn.id ? 'url(#arrowhead-hover)' : 'url(#arrowhead)'"
              @mouseenter="hoveredConnection = conn.id"
              @mouseleave="hoveredConnection = null"
              @click="deleteConnection(conn.id)"
            />

            <path
              v-if="isDrawing && drawingFrom"
              :d="getTempConnectionPath()"
              class="dag-connection temp"
              stroke-dasharray="8,4"
            />
          </g>
        </svg>

        <div class="nodes-layer" :style="transformStyle">
          <div
            v-for="node in nodes"
            :key="node.id"
            :class="['dag-node', node.type, node.status, { selected: selectedNode?.id === node.id }]"
            :style="{ left: `${node.x}px`, top: `${node.y}px`, transform: 'translate(-50%, -50%)' }"
            @mousedown.stop="startNodeDrag($event, node)"
            @click.stop="selectNode(node)"
          >
            <div class="node-content">
              <div class="node-header">
                <div class="node-icon" :class="node.type">
                  <el-icon><component :is="getNodeIcon(node.type)" /></el-icon>
                </div>
                <div class="node-info">
                  <div class="node-name">{{ node.name }}</div>
                  <div class="node-type">{{ node.type.toUpperCase() }}</div>
                </div>
                <div class="node-actions">
                  <el-button :icon="Plus" circle size="small" @click.stop="addChildNode(node)" />
                </div>
              </div>
              <div class="node-footer">
                <div class="status-badge" :class="node.status">
                  <span class="status-dot"></span>
                  <span>{{ getStatusText(node.status) }}</span>
                </div>
              </div>
            </div>

            <div
              class="node-port port-input"
              @mousedown.stop="startConnection($event, node, 'input')"
            >
              <div class="port-inner"></div>
              <div class="port-label">IN</div>
            </div>
            <div
              class="node-port port-output"
              @mousedown.stop="startConnection($event, node, 'output')"
            >
              <div class="port-inner"></div>
              <div class="port-label">OUT</div>
            </div>
          </div>
        </div>
      </div>

      <Transition name="slide">
        <div v-if="selectedNode" class="properties-panel">
          <div class="panel-header">
            <div class="panel-title">
              <el-icon><Setting /></el-icon>
              <span>节点属性</span>
            </div>
            <el-button :icon="Close" text circle size="small" @click="selectedNode = null" />
          </div>

          <div class="panel-content">
            <div class="property-section">
              <span class="property-label">节点名称</span>
              <el-input
                v-model="selectedNode.name"
                size="small"
                placeholder="请输入节点名称"
                @change="notifyUpdate"
              />
            </div>

            <div class="property-section">
              <span class="property-label">类型</span>
              <div class="property-value">
                <el-tag :type="getTypeTagType(selectedNode.type)" size="small">
                  {{ selectedNode.type.toUpperCase() }}
                </el-tag>
              </div>
            </div>

            <div class="property-section">
              <span class="property-label">状态</span>
              <el-select v-model="selectedNode.status" size="small" @change="notifyUpdate">
                <el-option label="待执行" value="pending" />
                <el-option label="运行中" value="running" />
                <el-option label="成功" value="success" />
                <el-option label="失败" value="failed" />
              </el-select>
            </div>

            <el-divider />

            <div class="config-section">
              <div class="config-header">
                <el-icon><Document /></el-icon>
                <span>配置</span>
              </div>

              <template v-if="selectedNode.type === 'http'">
                <div class="property-section">
                  <span class="property-label">URL</span>
                  <el-input
                    v-model="selectedNode.config!.url"
                    size="small"
                    placeholder="https://api.example.com"
                    @change="notifyUpdate"
                  />
                </div>
                <div class="property-section">
                  <span class="property-label">方法</span>
                  <el-select v-model="selectedNode.config!.method" size="small" @change="notifyUpdate">
                    <el-option label="GET" value="GET" />
                    <el-option label="POST" value="POST" />
                    <el-option label="PUT" value="PUT" />
                    <el-option label="DELETE" value="DELETE" />
                  </el-select>
                </div>
              </template>

              <template v-else-if="selectedNode.type === 'shell'">
                <div class="property-section">
                  <span class="property-label">脚本</span>
                  <el-input
                    v-model="selectedNode.config!.script"
                    type="textarea"
                    :rows="4"
                    size="small"
                    placeholder="./script.sh"
                    class="code-input"
                    @change="notifyUpdate"
                  />
                </div>
              </template>

              <template v-else-if="selectedNode.type === 'delay'">
                <div class="property-section">
                  <span class="property-label">延迟(毫秒)</span>
                  <el-input-number
                    v-model="selectedNode.config!.delay"
                    :min="100"
                    :step="500"
                    size="small"
                    controls-position="right"
                    @change="notifyUpdate"
                  />
                </div>
              </template>
            </div>
          </div>
        </div>
      </Transition>
    </div>

    <div class="editor-statusbar">
      <div class="statusbar-left">
        <span class="status-item">
          <el-icon><Document /></el-icon>
          {{ nodes.length }} 个节点
        </span>
        <span class="status-item">
          <el-icon><Connection /></el-icon>
          {{ connections.length }} 条连接
        </span>
      </div>
      <div class="statusbar-right">
        <span class="status-item">
          <el-icon><View /></el-icon>
          {{ Math.round(scale * 100) }}% 缩放
        </span>
        <span class="status-item mode">
          <el-icon><EditPen /></el-icon>
          编辑模式
        </span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch, onMounted } from 'vue'
import {
  Plus,
  Delete,
  ZoomIn,
  ZoomOut,
  FullScreen,
  Setting,
  VideoPlay,
  Close,
  Monitor,
  Clock,
  Connection,
  Document,
  View,
  EditPen,
  Link,
  Box,
  Refresh
} from '@element-plus/icons-vue'
import type { WorkflowNode, WorkflowConnection, WorkflowDAG } from '@/types'

const props = defineProps<{
  workflowId?: number
  initialDag?: WorkflowDAG
}>()

const emit = defineEmits<{
  update: [dag: WorkflowDAG]
  execute: [workflowId: number]
}>()

const canvasWidth = 4000
const canvasHeight = 4000

const nodes = ref<WorkflowNode[]>([])
const connections = ref<WorkflowConnection[]>([])
const selectedNode = ref<WorkflowNode | null>(null)
const hoveredConnection = ref<string | null>(null)

const scale = ref(1)
const panX = ref(0)
const panY = ref(0)

const isDrawing = ref(false)
const drawingFrom = ref<{ node: WorkflowNode; port: 'input' | 'output' } | null>(null)
const tempMousePos = reactive({ x: 0, y: 0 })

const isPanning = ref(false)
const isDragging = ref(false)
const dragNode = ref<WorkflowNode | null>(null)
const lastMousePos = reactive({ x: 0, y: 0 })

const canvasRef = ref<HTMLElement>()
const svgRef = ref<SVGElement>()

const transformStyle = computed(() => ({
  transform: `translate(${panX.value}px, ${panY.value}px) scale(${scale.value})`,
  transformOrigin: '0 0'
}))

const getNodeIcon = (type: string) => {
  const icons: Record<string, typeof Link> = {
    http: Link,
    shell: Monitor,
    delay: Clock
  }
  return icons[type] || Box
}

const getTypeTagType = (type: string) => {
  const types: Record<string, string> = {
    http: 'primary',
    shell: 'warning',
    delay: 'info'
  }
  return types[type] || 'info'
}

const getStatusText = (status: string) => {
  const texts: Record<string, string> = {
    pending: '待执行',
    running: '运行中',
    success: '成功',
    failed: '失败'
  }
  return texts[status] || status
}

const notifyUpdate = () => {
  emit('update', { nodes: nodes.value, connections: connections.value })
}

const addNode = (type: string) => {
  const typeNames: Record<string, string> = {
    http: 'HTTP',
    shell: '脚本',
    delay: '延迟'
  }
  const newNode: WorkflowNode = {
    id: `node_${Date.now()}`,
    name: `新${typeNames[type]}任务`,
    type: type as 'http' | 'shell' | 'delay',
    x: 400 + Math.random() * 200,
    y: 300 + Math.random() * 100,
    status: 'pending',
    config:
      type === 'http'
        ? { method: 'GET', url: '' }
        : type === 'delay'
          ? { delay: 1000 }
          : { script: '' }
  }
  nodes.value.push(newNode)
  selectedNode.value = newNode
  notifyUpdate()
}

const addChildNode = (parentNode: WorkflowNode) => {
  const newNode: WorkflowNode = {
    id: `node_${Date.now()}`,
    name: `新HTTP任务`,
    type: 'http',
    x: parentNode.x,
    y: parentNode.y + 160,
    status: 'pending',
    config: { method: 'GET', url: '' }
  }
  nodes.value.push(newNode)
  connections.value.push({
    id: `conn_${Date.now()}`,
    from: parentNode.id,
    to: newNode.id
  })
  selectedNode.value = newNode
  notifyUpdate()
}

const selectNode = (node: WorkflowNode) => {
  selectedNode.value = node
}

const deleteSelected = () => {
  if (!selectedNode.value) return
  nodes.value = nodes.value.filter(n => n.id !== selectedNode.value!.id)
  connections.value = connections.value.filter(
    c => c.from !== selectedNode.value!.id && c.to !== selectedNode.value!.id
  )
  selectedNode.value = null
  notifyUpdate()
}

const deleteConnection = (connId: string) => {
  connections.value = connections.value.filter(c => c.id !== connId)
  notifyUpdate()
}

const startNodeDrag = (e: MouseEvent, node: WorkflowNode) => {
  isDragging.value = true
  dragNode.value = node
  lastMousePos.x = e.clientX
  lastMousePos.y = e.clientY
}

const startConnection = (e: MouseEvent, node: WorkflowNode, port: 'input' | 'output') => {
  isDrawing.value = true
  drawingFrom.value = { node, port }
  updateTempMousePos(e)
}

const updateTempMousePos = (e: MouseEvent) => {
  if (!canvasRef.value) return
  const rect = canvasRef.value.getBoundingClientRect()
  tempMousePos.x = e.clientX - rect.left
  tempMousePos.y = e.clientY - rect.top
}

const handleCanvasMouseDown = (e: MouseEvent) => {
  if (e.button === 1 || (e.button === 0 && e.altKey)) {
    isPanning.value = true
    lastMousePos.x = e.clientX
    lastMousePos.y = e.clientY
  } else if (e.button === 0 && !isDrawing.value) {
    selectedNode.value = null
  }
}

const handleMouseMove = (e: MouseEvent) => {
  if (isPanning.value) {
    const dx = e.clientX - lastMousePos.x
    const dy = e.clientY - lastMousePos.y
    panX.value += dx
    panY.value += dy
    lastMousePos.x = e.clientX
    lastMousePos.y = e.clientY
  }

  if (isDragging.value && dragNode.value) {
    const dx = (e.clientX - lastMousePos.x) / scale.value
    const dy = (e.clientY - lastMousePos.y) / scale.value
    dragNode.value.x += dx
    dragNode.value.y += dy
    lastMousePos.x = e.clientX
    lastMousePos.y = e.clientY
    notifyUpdate()
  }

  if (isDrawing.value) {
    updateTempMousePos(e)
  }
}

const handleMouseUp = (e: MouseEvent) => {
  if (isDrawing.value && drawingFrom.value) {
    const targetNode = findNodeAtPosition(e.clientX, e.clientY)

    if (targetNode && targetNode.id !== drawingFrom.value.node.id) {
      const fromNode =
        drawingFrom.value.port === 'output' ? drawingFrom.value.node : targetNode
      const toNode = drawingFrom.value.port === 'output' ? targetNode : drawingFrom.value.node

      const exists = connections.value.some(c => c.from === fromNode.id && c.to === toNode.id)

      if (!exists) {
        connections.value.push({
          id: `conn_${Date.now()}`,
          from: fromNode.id,
          to: toNode.id
        })
        notifyUpdate()
      }
    }
  }

  isPanning.value = false
  isDragging.value = false
  dragNode.value = null
  isDrawing.value = false
  drawingFrom.value = null
}

const handleWheel = (e: WheelEvent) => {
  e.preventDefault()
  const delta = e.deltaY > 0 ? 0.9 : 1.1
  const newScale = Math.max(0.3, Math.min(2.5, scale.value * delta))

  if (canvasRef.value) {
    const rect = canvasRef.value.getBoundingClientRect()
    const mouseX = e.clientX - rect.left
    const mouseY = e.clientY - rect.top

    panX.value = mouseX - ((mouseX - panX.value) / scale.value) * newScale
    panY.value = mouseY - ((mouseY - panY.value) / scale.value) * newScale
  }

  scale.value = newScale
}

const findNodeAtPosition = (clientX: number, clientY: number): WorkflowNode | null => {
  if (!canvasRef.value) return null

  const rect = canvasRef.value.getBoundingClientRect()
  const x = (clientX - rect.left - panX.value) / scale.value
  const y = (clientY - rect.top - panY.value) / scale.value

  return (
    nodes.value.find(n => {
      const nodeWidth = 180
      const nodeHeight = 90
      return x >= n.x - nodeWidth / 2 && x <= n.x + nodeWidth / 2 && y >= n.y - nodeHeight / 2 && y <= n.y + nodeHeight / 2
    }) || null
  )
}

const getNodeCenter = (node: WorkflowNode) => ({
  x: node.x,
  y: node.y
})

const getConnectionPath = (conn: WorkflowConnection) => {
  const fromNode = nodes.value.find(n => n.id === conn.from)
  const toNode = nodes.value.find(n => n.id === conn.to)

  if (!fromNode || !toNode) return ''

  const from = getNodeCenter(fromNode)
  const to = getNodeCenter(toNode)

  const nodeWidth = 180

  const fromX = fromNode.x < toNode.x ? from.x + nodeWidth / 2 - 20 : from.x - nodeWidth / 2 + 20
  const toX = fromNode.x < toNode.x ? to.x - nodeWidth / 2 + 20 : to.x + nodeWidth / 2 - 20

  const fromY = from.y
  const toY = to.y

  const controlOffset = Math.abs(toX - fromX) * 0.5

  return `M ${fromX} ${fromY} C ${fromX + controlOffset} ${fromY}, ${toX - controlOffset} ${toY}, ${toX} ${toY}`
}

const getTempConnectionPath = () => {
  if (!drawingFrom.value) return ''

  const from = getNodeCenter(drawingFrom.value.node)
  const nodeWidth = 180

  const fromX =
    drawingFrom.value.port === 'output'
      ? from.x + nodeWidth / 2 - 20
      : from.x - nodeWidth / 2 + 20

  const controlOffset = Math.abs(tempMousePos.x / scale.value - fromX) * 0.5

  return `M ${fromX} ${from.y} C ${fromX + controlOffset} ${from.y}, ${tempMousePos.x / scale.value - controlOffset} ${tempMousePos.y / scale.value}, ${tempMousePos.x / scale.value} ${tempMousePos.y / scale.value}`
}

const zoomIn = () => {
  scale.value = Math.min(scale.value * 1.2, 2.5)
}

const zoomOut = () => {
  scale.value = Math.max(scale.value / 1.2, 0.3)
}

const resetView = () => {
  scale.value = 1
  panX.value = 0
  panY.value = 0
}

const clearAll = () => {
  nodes.value = []
  connections.value = []
  selectedNode.value = null
  notifyUpdate()
}

const runWorkflow = () => {
  if (props.workflowId !== undefined) {
    emit('execute', props.workflowId)
  }
}

watch(() => props.initialDag, (newDag) => {
  if (newDag && newDag.nodes) {
    nodes.value = JSON.parse(JSON.stringify(newDag.nodes))
    connections.value = JSON.parse(JSON.stringify(newDag.connections || []))
  }
}, { immediate: true })

onMounted(() => {
  if (props.initialDag && props.initialDag.nodes) {
    nodes.value = JSON.parse(JSON.stringify(props.initialDag.nodes))
    connections.value = JSON.parse(JSON.stringify(props.initialDag.connections || []))
  }
})
</script>

<style scoped>
.dag-editor {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: #f9fafb;
  border-radius: 12px;
  overflow: hidden;
  border: 1px solid #e5e7eb;
}

.editor-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background: white;
  border-bottom: 1px solid #e5e7eb;
  gap: 16px;
}

.toolbar-left,
.toolbar-center,
.toolbar-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.toolbar-group {
  display: flex;
  gap: 8px;
}

.toolbar-divider {
  width: 1px;
  height: 28px;
  background: #e5e7eb;
}

.btn-label {
  margin-left: 4px;
}

.zoom-controls {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 8px;
  background: #f3f4f6;
  border-radius: 8px;
}

.zoom-level {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.75rem;
  color: #6b7280;
  min-width: 48px;
  text-align: center;
}

.editor-container {
  flex: 1;
  display: flex;
  position: relative;
  overflow: hidden;
}

.dag-canvas {
  flex: 1;
  position: relative;
  overflow: hidden;
  cursor: grab;
  background:
    radial-gradient(circle at 0 0, rgba(59, 130, 246, 0.03) 0, transparent 50%),
    radial-gradient(circle at 100% 0, rgba(139, 92, 246, 0.03) 0, transparent 50%);
}

.dag-canvas:active {
  cursor: grabbing;
}

.connection-layer {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  pointer-events: none;
}

.connection-layer path {
  pointer-events: stroke;
}

.dag-connection {
  stroke: #cbd5e1;
  stroke-width: 2;
  fill: none;
  transition: stroke 0.2s ease, stroke-width 0.2s ease;
  cursor: pointer;
}

.dag-connection:hover,
.dag-connection.active {
  stroke: url(#connectionGradient);
  stroke-width: 3;
  filter: url(#glow);
}

.dag-connection.temp {
  stroke: #06b6d4;
  opacity: 0.6;
  stroke-dasharray: 8, 4;
}

.nodes-layer {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
}

.dag-node {
  position: absolute;
  width: 180px;
  background: white;
  border: 2px solid #e5e7eb;
  border-radius: 12px;
  cursor: grab;
  transition: all 0.2s ease-out;
  user-select: none;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05);
}

.dag-node:hover {
  border-color: #06b6d4;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.1), 0 0 0 1px rgba(6, 182, 212, 0.1);
  transform: translateY(-2px) translate(-50%, -50%);
}

.dag-node.selected {
  border-color: #06b6d4;
  box-shadow: 0 0 0 4px rgba(6, 182, 212, 0.2), 0 12px 32px rgba(0, 0, 0, 0.12);
}

.dag-node.running {
  border-color: #f59e0b;
}

.dag-node.success {
  border-color: #22c55e;
}

.dag-node.failed {
  border-color: #ef4444;
}

.dag-node.pending {
  border-color: #9ca3af;
}

.node-content {
  padding: 12px;
}

.node-header {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 8px;
}

.node-icon {
  width: 36px;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  font-size: 1.1rem;
  transition: all 0.2s ease;
}

.node-icon.http {
  background: linear-gradient(135deg, #3b82f6, #2563eb);
  color: white;
}

.node-icon.shell {
  background: linear-gradient(135deg, #f59e0b, #d97706);
  color: white;
}

.node-icon.delay {
  background: linear-gradient(135deg, #8b5cf6, #7c3aed);
  color: white;
}

.node-info {
  flex: 1;
  min-width: 0;
}

.node-name {
  font-weight: 600;
  font-size: 0.85rem;
  color: #1f293b;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.node-type {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.65rem;
  color: #9ca3af;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.node-actions {
  display: flex;
  gap: 4px;
  opacity: 0;
  transition: opacity 0.2s ease;
}

.dag-node:hover .node-actions {
  opacity: 1;
}

.node-footer {
  display: flex;
  justify-content: flex-start;
}

.status-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: 9999px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.65rem;
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.03em;
}

.status-dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: currentColor;
}

.status-badge.pending {
  background: #f1f5f9;
  color: #6b7280;
}

.status-badge.running {
  background: #fef3c7;
  color: #d97706;
}

.status-badge.running .status-dot {
  animation: pulse 1.5s ease-in-out infinite;
}

.status-badge.success {
  background: #dcfce7;
  color: #16a34a;
}

.status-badge.failed {
  background: #fee2e2;
  color: #dc2626;
}

@keyframes pulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.5; transform: scale(1.2); }
}

.node-port {
  position: absolute;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
  cursor: crosshair;
  z-index: 10;
}

.port-input {
  left: -10px;
  top: 50%;
  transform: translateY(-50%);
}

.port-output {
  right: -10px;
  top: 50%;
  transform: translateY(-50%);
}

.port-inner {
  width: 12px;
  height: 12px;
  background: white;
  border: 2px solid #06b6d4;
  border-radius: 50%;
  transition: all 0.2s ease;
}

.node-port:hover .port-inner {
  background: #06b6d4;
  transform: scale(1.3);
  box-shadow: 0 0 12px rgba(6, 182, 212, 0.5);
}

.port-label {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.55rem;
  color: #9ca3af;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.properties-panel {
  width: 320px;
  background: white;
  border-left: 1px solid #e5e7eb;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  box-shadow: -4px 0 24px rgba(0, 0, 0, 0.08);
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px;
  border-bottom: 1px solid #e5e7eb;
}

.panel-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
  font-size: 0.95rem;
  color: #1f2937;
}

.panel-content {
  flex: 1;
  padding: 16px;
  overflow-y: auto;
}

.property-section {
  margin-bottom: 16px;
}

.property-label {
  display: block;
  font-size: 0.75rem;
  font-weight: 500;
  color: #6b7280;
  margin-bottom: 8px;
  text-transform: uppercase;
  letter-spacing: 0.03em;
}

.property-value {
  display: flex;
  align-items: center;
}

.config-section {
  margin-top: 16px;
}

.config-header {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
  font-size: 0.85rem;
  color: #6b7280;
  margin-bottom: 12px;
  text-transform: uppercase;
  letter-spacing: 0.03em;
}

.code-input :deep(.el-textarea__inner) {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.8rem;
  background: #f9fafb;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  resize: none;
}

.slide-enter-active,
.slide-leave-active {
  transition: all 0.3s ease-out;
}

.slide-enter-from,
.slide-leave-to {
  transform: translateX(100%);
  opacity: 0;
}

.editor-statusbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 16px;
  background: white;
  border-top: 1px solid #e5e7eb;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.75rem;
}

.statusbar-left,
.statusbar-right {
  display: flex;
  gap: 24px;
}

.status-item {
  display: flex;
  align-items: center;
  gap: 6px;
  color: #6b7280;
}

.status-item.mode {
  color: #06b6d4;
  background: rgba(6, 182, 212, 0.1);
  padding: 4px 10px;
  border-radius: 6px;
}
</style>