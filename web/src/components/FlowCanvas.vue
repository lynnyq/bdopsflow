<template>
  <div class="flow-canvas">
    <div class="canvas-header">
      <div class="node-palette">
        <div class="palette-title">节点组件</div>
        <div class="node-types">
          <div
            v-for="nodeType in nodeTypes"
            :key="nodeType.type"
            class="node-type-item"
            draggable="true"
            @dragstart="handleDragStart($event, nodeType)"
          >
            <el-icon><component :is="nodeType.icon" /></el-icon>
            <span>{{ nodeType.label }}</span>
          </div>
        </div>
      </div>

      <div class="canvas-controls">
        <el-button :icon="MagicStick" @click="autoLayout" size="small">
          自动布局
        </el-button>
        <el-button :icon="Download" @click="exportFlow" size="small">
          导出
        </el-button>
        <el-button :icon="RefreshRight" @click="clearCanvas" size="small">
          清空
        </el-button>

        <div class="zoom-controls">
          <el-button :icon="ZoomOut" circle size="small" @click="zoomOut" />
          <span class="zoom-level">{{ Math.round(scale * 100) }}%</span>
          <el-button :icon="ZoomIn" circle size="small" @click="zoomIn" />
          <el-button :icon="FullScreen" circle size="small" @click="resetView" />
        </div>
      </div>
    </div>

    <div class="canvas-wrapper" ref="canvasRef">
      <div
        class="canvas-main"
        :style="{ transform: `translate(${panX}px, ${panY}px) scale(${scale})` }"
        @mousedown="handleCanvasMouseDown"
        @mousemove="handleCanvasMouseMove"
        @mouseup="handleCanvasMouseUp"
        @mouseleave="handleCanvasMouseUp"
        @dragover.prevent
        @drop="handleDrop"
      >
        <svg class="connection-svg" width="3000" height="2000">
          <defs>
            <marker
              id="arrow"
              markerWidth="10"
              markerHeight="10"
              refX="9"
              refY="3"
              orient="auto"
            >
              <polygon points="0 0, 10 3, 0 6" fill="#3b82f6" />
            </marker>
            <marker
              id="arrow-hover"
              markerWidth="10"
              markerHeight="10"
              refX="9"
              refY="3"
              orient="auto"
            >
              <polygon points="0 0, 10 3, 0 6" fill="#1d4ed8" />
            </marker>
          </defs>

          <path
            v-for="conn in connections"
            :key="conn.id"
            :d="getConnectionPath(conn)"
            :class="['conn-line', { active: hoveredConnId === conn.id }]"
            :marker-end="hoveredConnId === conn.id ? 'url(#arrow-hover)' : 'url(#arrow)'"
            @mouseenter="hoveredConnId = conn.id"
            @mouseleave="hoveredConnId = null"
            @click="deleteConnection(conn.id)"
          />

          <path
            v-if="isDrawing && startNode"
            :d="getTempConnPath()"
            class="conn-line temp"
          />
        </svg>

        <div
          v-for="node in nodes"
          :key="node.id"
          :class="['flow-node', node.type, { selected: selectedNodeId === node.id }]"
          :style="{ left: `${node.x}px`, top: `${node.y}px` }"
          @mousedown.stop="startNodeDrag($event, node)"
          @click.stop="selectNode(node)"
        >
          <div class="node-header">
            <div class="node-icon">
              <el-icon><component :is="getNodeIcon(node.type)" /></el-icon>
            </div>
            <div class="node-info">
              <div class="node-name">{{ node.name }}</div>
              <div class="node-type-label">{{ getNodeTypeLabel(node.type) }}</div>
            </div>
            <div class="node-actions">
              <el-button
                :icon="Plus"
                circle
                size="small"
                @click.stop="addChild(node)"
              />
              <el-button
                :icon="Delete"
                circle
                size="small"
                type="danger"
                @click.stop="removeNode(node.id)"
              />
            </div>
          </div>

          <div class="node-body">
            <div class="node-preview">{{ getNodePreview(node) }}</div>
          </div>

          <div class="node-footer">
            <span class="status-badge" :class="node.status">
              {{ getStatusText(node.status) }}
            </span>
          </div>

          <div
            class="port port-in"
            @mousedown.stop="startConnection($event, node, 'in')"
          >
            <div class="port-dot"></div>
          </div>
          <div
            class="port port-out"
            @mousedown.stop="startConnection($event, node, 'out')"
          >
            <div class="port-dot"></div>
          </div>
        </div>
      </div>
    </div>

    <div v-if="selectedNode" class="properties-panel">
      <div class="panel-header">
        <h4>节点属性</h4>
        <el-button :icon="Close" circle size="small" @click="selectedNodeId = null" />
      </div>
      <div class="panel-body">
        <el-form size="small">
          <el-form-item label="节点名称">
            <el-input
              v-model="selectedNode.name"
              placeholder="输入节点名称"
              @change="notifyUpdate"
            />
          </el-form-item>
          <el-form-item label="节点类型">
            <el-tag :type="getNodeTagType(selectedNode.type)">
              {{ getNodeTypeLabel(selectedNode.type) }}
            </el-tag>
          </el-form-item>
          <el-form-item label="运行状态">
            <el-select v-model="selectedNode.status" @change="notifyUpdate">
              <el-option label="待执行" value="pending" />
              <el-option label="运行中" value="running" />
              <el-option label="成功" value="success" />
              <el-option label="失败" value="failed" />
            </el-select>
          </el-form-item>

          <el-divider />

          <template v-if="selectedNode.type === 'http'">
            <el-form-item label="请求URL">
              <el-input
                v-model="selectedNode.config!.url"
                placeholder="https://api.example.com"
                @change="notifyUpdate"
              />
            </el-form-item>
            <el-form-item label="请求方法">
              <el-select v-model="selectedNode.config!.method" @change="notifyUpdate">
                <el-option label="GET" value="GET" />
                <el-option label="POST" value="POST" />
                <el-option label="PUT" value="PUT" />
                <el-option label="DELETE" value="DELETE" />
              </el-select>
            </el-form-item>
            <el-form-item label="超时(秒)">
              <el-input-number
                v-model="selectedNode.config!.timeout"
                :min="1"
                :max="300"
                @change="notifyUpdate"
              />
            </el-form-item>
          </template>

          <template v-else-if="selectedNode.type === 'shell'">
            <el-form-item label="Shell脚本">
              <el-input
                v-model="selectedNode.config!.script"
                type="textarea"
                :rows="6"
                placeholder="输入shell脚本"
                @change="notifyUpdate"
              />
            </el-form-item>
          </template>

          <template v-else-if="selectedNode.type === 'delay'">
            <el-form-item label="延迟(秒)">
              <el-input-number
                v-model="selectedNode.config!.delay"
                :min="1"
                :max="3600"
                @change="notifyUpdate"
              />
            </el-form-item>
          </template>

          <template v-else-if="selectedNode.type === 'condition'">
            <el-form-item label="条件表达式">
              <el-input
                v-model="selectedNode.config!.condition"
                type="textarea"
                :rows="4"
                placeholder="输入条件表达式"
                @change="notifyUpdate"
              />
            </el-form-item>
          </template>

          <template v-else-if="selectedNode.type === 'webhook'">
            <el-form-item label="Webhook URL">
              <el-input
                v-model="selectedNode.config!.url"
                placeholder="https://webhook.example.com"
                @change="notifyUpdate"
              />
            </el-form-item>
            <el-form-item label="触发条件">
              <el-select v-model="selectedNode.config!.trigger" @change="notifyUpdate">
                <el-option label="任务完成" value="completed" />
                <el-option label="任务失败" value="failed" />
                <el-option label="全部状态" value="all" />
              </el-select>
            </el-form-item>
          </template>
        </el-form>
      </div>
    </div>

    <div class="canvas-footer">
      <div class="stats-left">
        <span><el-icon><Document /></el-icon> {{ nodes.length }} 节点</span>
        <span><el-icon><Connection /></el-icon> {{ connections.length }} 连接</span>
      </div>
      <div class="stats-right">
        <span class="mode-badge">
          <el-icon><EditPen /></el-icon>
          编辑模式
        </span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import {
  Plus,
  Delete,
  Close,
  ZoomIn,
  ZoomOut,
  FullScreen,
  MagicStick,
  Download,
  RefreshRight,
  Document,
  Connection,
  EditPen,
  Link,
  Monitor,
  Clock,
  Position,
  Bell
} from '@element-plus/icons-vue'
import type { WorkflowNode, WorkflowConnection, WorkflowDAG } from '@/types'

const props = defineProps<{
  initialDag?: WorkflowDAG
}>()

const emit = defineEmits<{
  update: [dag: WorkflowDAG]
}>()

const canvasRef = ref<HTMLElement>()

const nodes = ref<WorkflowNode[]>([])
const connections = ref<WorkflowConnection[]>([])
const selectedNodeId = ref<string | null>(null)
const hoveredConnId = ref<string | null>(null)

const scale = ref(1)
const panX = ref(0)
const panY = ref(0)

const isDraggingNode = ref(false)
const isDraggingCanvas = ref(false)
const isDrawing = ref(false)
const startNode = ref<WorkflowNode | null>(null)
const startPort = ref<'in' | 'out' | null>(null)
const lastPos = ref({ x: 0, y: 0 })
const tempEndPos = ref({ x: 0, y: 0 })

const nodeTypes = [
  { type: 'http', label: 'HTTP请求', icon: Link },
  { type: 'shell', label: 'Shell脚本', icon: Monitor },
  { type: 'delay', label: '延迟等待', icon: Clock },
  { type: 'condition', label: '条件判断', icon: Position },
  { type: 'webhook', label: 'Webhook通知', icon: Bell }
]

const selectedNode = computed(() =>
  nodes.value.find(n => n.id === selectedNodeId.value) || null
)

const getNodeIcon = (type: string) => {
  const typeMap: Record<string, any> = {
    http: Link,
    shell: Monitor,
    delay: Clock,
    condition: Position,
    webhook: Bell
  }
  return typeMap[type] || Link
}

const getNodeTypeLabel = (type: string) => {
  const labelMap: Record<string, string> = {
    http: 'HTTP请求',
    shell: 'Shell脚本',
    delay: '延迟等待',
    condition: '条件判断',
    webhook: 'Webhook通知'
  }
  return labelMap[type] || type
}

const getNodeTagType = (type: string) => {
  const tagMap: Record<string, any> = {
    http: 'primary',
    shell: 'warning',
    delay: 'info',
    condition: 'success',
    webhook: 'danger'
  }
  return tagMap[type] || 'info'
}

const getStatusText = (status: string) => {
  const statusMap: Record<string, string> = {
    pending: '待执行',
    running: '运行中',
    success: '成功',
    failed: '失败'
  }
  return statusMap[status] || status
}

const getNodePreview = (node: WorkflowNode) => {
  switch (node.type) {
    case 'http':
      return node.config?.method || 'GET'
    case 'shell':
      return (node.config?.script || '').substring(0, 30)
    case 'delay':
      return `延迟 ${node.config?.delay || 0}s`
    case 'condition':
      return (node.config?.condition || '').substring(0, 30)
    case 'webhook':
      return (node.config?.url || '').substring(0, 30)
    default:
      return ''
  }
}

const notifyUpdate = () => {
  emit('update', { nodes: nodes.value, connections: connections.value })
}

const handleDragStart = (e: DragEvent, nodeType: any) => {
  e.dataTransfer?.setData('nodeType', JSON.stringify(nodeType))
}

const handleDrop = (e: DragEvent) => {
  const typeData = e.dataTransfer?.getData('nodeType')
  if (!typeData) return

  const nodeType = JSON.parse(typeData)
  const rect = canvasRef.value?.getBoundingClientRect()
  if (!rect) return

  const x = (e.clientX - rect.left - panX.value) / scale.value - 100
  const y = (e.clientY - rect.top - panY.value) / scale.value - 50

  addNode(nodeType.type, x, y)
}

const addNode = (type: string, x: number, y: number) => {
  const newNode: WorkflowNode = {
    id: `node-${Date.now()}`,
    name: `新${getNodeTypeLabel(type)}`,
    type: type as any,
    x,
    y,
    status: 'pending',
    config:
      type === 'http'
        ? { method: 'GET', url: '', timeout: 30 }
        : type === 'shell'
          ? { script: '' }
          : type === 'delay'
            ? { delay: 5 }
            : type === 'condition'
              ? { condition: '' }
              : { url: '', trigger: 'completed' }
  }

  nodes.value.push(newNode)
  selectedNodeId.value = newNode.id
  notifyUpdate()
}

const addChild = (parentNode: WorkflowNode) => {
  const childNode: WorkflowNode = {
    id: `node-${Date.now()}`,
    name: '新HTTP请求',
    type: 'http',
    x: parentNode.x,
    y: parentNode.y + 150,
    status: 'pending',
    config: { method: 'GET', url: '', timeout: 30 }
  }

  nodes.value.push(childNode)
  connections.value.push({
    id: `conn-${Date.now()}`,
    from: parentNode.id,
    to: childNode.id
  })
  selectedNodeId.value = childNode.id
  notifyUpdate()
}

const removeNode = (nodeId: string) => {
  nodes.value = nodes.value.filter(n => n.id !== nodeId)
  connections.value = connections.value.filter(
    c => c.from !== nodeId && c.to !== nodeId
  )
  if (selectedNodeId.value === nodeId) {
    selectedNodeId.value = null
  }
  notifyUpdate()
}

const selectNode = (node: WorkflowNode) => {
  selectedNodeId.value = node.id
}

const deleteConnection = (connId: string) => {
  connections.value = connections.value.filter(c => c.id !== connId)
  notifyUpdate()
}

const startNodeDrag = (e: MouseEvent, _node: WorkflowNode) => {
  isDraggingNode.value = true
  lastPos.value = { x: e.clientX, y: e.clientY }
}

const startConnection = (e: MouseEvent, node: WorkflowNode, port: 'in' | 'out') => {
  isDrawing.value = true
  startNode.value = node
  startPort.value = port
  tempEndPos.value = { x: e.clientX, y: e.clientY }
}

const handleCanvasMouseDown = (e: MouseEvent) => {
  if (e.button === 0) {
    isDraggingCanvas.value = true
    lastPos.value = { x: e.clientX, y: e.clientY }
    if (!isDrawing.value) {
      selectedNodeId.value = null
    }
  }
}

const handleCanvasMouseMove = (e: MouseEvent) => {
  if (isDraggingCanvas.value) {
    panX.value += e.clientX - lastPos.value.x
    panY.value += e.clientY - lastPos.value.y
    lastPos.value = { x: e.clientX, y: e.clientY }
  }

  if (isDraggingNode.value && selectedNode.value) {
    const dx = (e.clientX - lastPos.value.x) / scale.value
    const dy = (e.clientY - lastPos.value.y) / scale.value
    selectedNode.value.x += dx
    selectedNode.value.y += dy
    lastPos.value = { x: e.clientX, y: e.clientY }
    notifyUpdate()
  }

  if (isDrawing.value) {
    const rect = canvasRef.value?.getBoundingClientRect()
    if (rect) {
      tempEndPos.value = {
        x: (e.clientX - rect.left - panX.value) / scale.value,
        y: (e.clientY - rect.top - panY.value) / scale.value
      }
    }
  }
}

const handleCanvasMouseUp = (e: MouseEvent) => {
  if (isDrawing.value && startNode.value) {
    const targetNode = findNodeAtPosition(e.clientX, e.clientY)
    if (targetNode && targetNode.id !== startNode.value.id) {
      const fromNode = startPort.value === 'out' ? startNode.value : targetNode
      const toNode = startPort.value === 'out' ? targetNode : startNode.value

      const exists = connections.value.some(
        c => c.from === fromNode.id && c.to === toNode.id
      )
      if (!exists) {
        connections.value.push({
          id: `conn-${Date.now()}`,
          from: fromNode.id,
          to: toNode.id
        })
        notifyUpdate()
      }
    }
  }

  isDraggingNode.value = false
  isDraggingCanvas.value = false
  isDrawing.value = false
  startNode.value = null
  startPort.value = null
}

const findNodeAtPosition = (clientX: number, clientY: number) => {
  const rect = canvasRef.value?.getBoundingClientRect()
  if (!rect) return null

  const x = (clientX - rect.left - panX.value) / scale.value
  const y = (clientY - rect.top - panY.value) / scale.value

  return nodes.value.find(node => {
    const nodeWidth = 200
    const nodeHeight = 100
    return (
      x >= node.x &&
      x <= node.x + nodeWidth &&
      y >= node.y &&
      y <= node.y + nodeHeight
    )
  }) || null
}

const getConnectionPath = (conn: WorkflowConnection) => {
  const fromNode = nodes.value.find(n => n.id === conn.from)
  const toNode = nodes.value.find(n => n.id === conn.to)

  if (!fromNode || !toNode) return ''

  const fromX = fromNode.x + 200
  const fromY = fromNode.y + 50
  const toX = toNode.x
  const toY = toNode.y + 50

  const controlOffset = Math.abs(toX - fromX) * 0.5

  return `M ${fromX} ${fromY} C ${fromX + controlOffset} ${fromY}, ${toX - controlOffset} ${toY}, ${toX} ${toY}`
}

const getTempConnPath = () => {
  if (!startNode.value) return ''

  const fromX = startPort.value === 'out' ? startNode.value.x + 200 : startNode.value.x
  const fromY = startNode.value.y + 50
  const toX = tempEndPos.value.x
  const toY = tempEndPos.value.y

  const controlOffset = Math.abs(toX - fromX) * 0.5

  return `M ${fromX} ${fromY} C ${fromX + controlOffset} ${fromY}, ${toX - controlOffset} ${toY}, ${toX} ${toY}`
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

const autoLayout = () => {
  if (nodes.value.length === 0) return

  const inDegree = new Map<string, number>()
  const outEdges = new Map<string, string[]>()

  nodes.value.forEach(node => inDegree.set(node.id, 0))
  connections.value.forEach(conn => {
    inDegree.set(conn.to, (inDegree.get(conn.to) || 0) + 1)
    const edges = outEdges.get(conn.from) || []
    edges.push(conn.to)
    outEdges.set(conn.from, edges)
  })

  const layers: WorkflowNode[][] = []
  let currentLayer = nodes.value.filter(n => inDegree.get(n.id) === 0)

  while (currentLayer.length > 0) {
    layers.push(currentLayer)
    const nextLayer: WorkflowNode[] = []

    currentLayer.forEach(node => {
      const outNodeIds = outEdges.get(node.id) || []
      outNodeIds.forEach(id => {
        const remaining = (inDegree.get(id) || 0) - 1
        inDegree.set(id, remaining)
        if (remaining === 0) {
          const nodeToAdd = nodes.value.find(n => n.id === id)
          if (nodeToAdd) nextLayer.push(nodeToAdd)
        }
      })
    })

    currentLayer = nextLayer
  }

  const remaining = nodes.value.filter(n =>
    !layers.some(layer => layer.some(l => l.id === n.id))
  )
  if (remaining.length > 0) {
    layers.push(remaining)
  }

  const layerWidth = 280
  const nodeHeight = 140
  const startX = 100
  const startY = 100

  layers.forEach((layer, i) => {
    layer.forEach((node, j) => {
      node.x = startX + i * layerWidth
      node.y = startY + j * nodeHeight
    })
  })

  notifyUpdate()
}

const clearCanvas = () => {
  nodes.value = []
  connections.value = []
  selectedNodeId.value = null
  notifyUpdate()
}

const exportFlow = () => {
  const data = JSON.stringify({ nodes: nodes.value, connections: connections.value }, null, 2)
  const blob = new Blob([data], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = 'workflow.json'
  a.click()
  URL.revokeObjectURL(url)
}

watch(() => props.initialDag, (newDag) => {
  if (newDag) {
    nodes.value = JSON.parse(JSON.stringify(newDag.nodes || []))
    connections.value = JSON.parse(JSON.stringify(newDag.connections || []))
  }
}, { immediate: true })

onMounted(() => {
  if (props.initialDag) {
    nodes.value = JSON.parse(JSON.stringify(props.initialDag.nodes || []))
    connections.value = JSON.parse(JSON.stringify(props.initialDag.connections || []))
  }
})
</script>

<style scoped>
.flow-canvas {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: #f1f5f9;
  border-radius: 12px;
  overflow: hidden;
}

.canvas-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background: white;
  border-bottom: 1px solid #e2e8f0;
  flex-wrap: wrap;
  gap: 12px;
}

.node-palette {
  display: flex;
  align-items: center;
  gap: 12px;
}

.palette-title {
  font-size: 0.75rem;
  font-weight: 600;
  color: #64748b;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.node-types {
  display: flex;
  gap: 8px;
}

.node-type-item {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  cursor: grab;
  transition: all 0.2s;
  font-size: 0.85rem;
  color: #475569;
}

.node-type-item:hover {
  background: #3b82f6;
  color: white;
  border-color: #3b82f6;
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(59, 130, 246, 0.2);
}

.canvas-controls {
  display: flex;
  align-items: center;
  gap: 12px;
}

.zoom-controls {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 8px;
  background: #f8fafc;
  border-radius: 8px;
}

.zoom-level {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.75rem;
  color: #64748b;
  min-width: 48px;
  text-align: center;
}

.canvas-wrapper {
  flex: 1;
  overflow: hidden;
  position: relative;
}

.canvas-main {
  position: absolute;
  top: 0;
  left: 0;
  width: 3000px;
  height: 2000px;
  cursor: grab;
  transform-origin: 0 0;
}

.canvas-main:active {
  cursor: grabbing;
}

.connection-svg {
  position: absolute;
  top: 0;
  left: 0;
  pointer-events: none;
}

.connection-svg path {
  pointer-events: stroke;
}

.conn-line {
  stroke: #cbd5e1;
  stroke-width: 2;
  fill: none;
  transition: stroke 0.2s, stroke-width 0.2s;
  cursor: pointer;
}

.conn-line:hover,
.conn-line.active {
  stroke: #3b82f6;
  stroke-width: 3;
}

.conn-line.temp {
  stroke: #3b82f6;
  stroke-dasharray: 8, 4;
  opacity: 0.6;
}

.flow-node {
  position: absolute;
  width: 200px;
  background: white;
  border: 2px solid #e2e8f0;
  border-radius: 12px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05);
  cursor: grab;
  transition: all 0.2s;
  user-select: none;
}

.flow-node:hover {
  border-color: #3b82f6;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.1), 0 0 0 1px rgba(59, 130, 246, 0.1);
  transform: translateY(-2px);
}

.flow-node.selected {
  border-color: #3b82f6;
  box-shadow: 0 0 0 4px rgba(59, 130, 246, 0.2), 0 12px 32px rgba(0, 0, 0, 0.12);
}

.flow-node.running {
  border-color: #f59e0b;
}

.flow-node.success {
  border-color: #22c55e;
}

.flow-node.failed {
  border-color: #ef4444;
}

.node-header {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px;
  border-bottom: 1px solid #f1f5f9;
  border-radius: 10px 10px 0 0;
  background: linear-gradient(135deg, #f8fafc, white);
}

.node-icon {
  width: 36px;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  color: white;
  font-size: 1.1rem;
}

.flow-node.http .node-icon {
  background: linear-gradient(135deg, #3b82f6, #2563eb);
}

.flow-node.shell .node-icon {
  background: linear-gradient(135deg, #f59e0b, #d97706);
}

.flow-node.delay .node-icon {
  background: linear-gradient(135deg, #8b5cf6, #7c3aed);
}

.flow-node.condition .node-icon {
  background: linear-gradient(135deg, #22c55e, #16a34a);
}

.flow-node.webhook .node-icon {
  background: linear-gradient(135deg, #ef4444, #dc2626);
}

.node-info {
  flex: 1;
  min-width: 0;
}

.node-name {
  font-weight: 600;
  font-size: 0.85rem;
  color: #1e293b;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.node-type-label {
  font-size: 0.65rem;
  color: #94a3b8;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.node-actions {
  display: flex;
  gap: 4px;
  opacity: 0;
  transition: opacity 0.2s;
}

.flow-node:hover .node-actions {
  opacity: 1;
}

.node-body {
  padding: 12px;
  min-height: 40px;
}

.node-preview {
  font-size: 0.75rem;
  color: #64748b;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.node-footer {
  padding: 8px 12px;
  border-top: 1px solid #f1f5f9;
  border-radius: 0 0 10px 10px;
  background: #f8fafc;
}

.status-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: 9999px;
  font-size: 0.65rem;
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.03em;
}

.status-badge.pending {
  background: #f1f5f9;
  color: #6b7280;
}

.status-badge.running {
  background: #fef3c7;
  color: #d97706;
}

.status-badge.success {
  background: #dcfce7;
  color: #16a34a;
}

.status-badge.failed {
  background: #fee2e2;
  color: #dc2626;
}

.port {
  position: absolute;
  width: 20px;
  height: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: crosshair;
  z-index: 10;
}

.port-in {
  left: -10px;
  top: 50%;
  transform: translateY(-50%);
}

.port-out {
  right: -10px;
  top: 50%;
  transform: translateY(-50%);
}

.port-dot {
  width: 12px;
  height: 12px;
  background: white;
  border: 2px solid #3b82f6;
  border-radius: 50%;
  transition: all 0.2s;
}

.port:hover .port-dot {
  background: #3b82f6;
  transform: scale(1.3);
  box-shadow: 0 0 12px rgba(59, 130, 246, 0.5);
}

.properties-panel {
  position: absolute;
  right: 0;
  top: 76px;
  bottom: 44px;
  width: 300px;
  background: white;
  border-left: 1px solid #e2e8f0;
  display: flex;
  flex-direction: column;
  box-shadow: -4px 0 24px rgba(0, 0, 0, 0.08);
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px;
  border-bottom: 1px solid #f1f5f9;
}

.panel-header h4 {
  margin: 0;
  font-size: 0.95rem;
  font-weight: 600;
  color: #1e293b;
}

.panel-body {
  flex: 1;
  padding: 16px;
  overflow-y: auto;
}

.canvas-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 16px;
  background: white;
  border-top: 1px solid #e2e8f0;
  font-size: 0.75rem;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.stats-left,
.stats-right {
  display: flex;
  gap: 24px;
  align-items: center;
}

.stats-left span {
  display: flex;
  align-items: center;
  gap: 6px;
  color: #64748b;
}

.mode-badge {
  display: flex;
  align-items: center;
  gap: 6px;
  color: #3b82f6;
  background: rgba(59, 130, 246, 0.1);
  padding: 4px 10px;
  border-radius: 6px;
  font-weight: 500;
}
</style>
