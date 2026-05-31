<template>
  <div class="workflow-canvas">
    <div class="canvas-toolbar">
      <div class="toolbar-section">
        <span class="section-title">添加节点</span>
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

      <div class="toolbar-divider"></div>

      <div class="toolbar-section">
        <el-button :icon="MagicStick" @click="autoLayout" size="small">
          自动布局
        </el-button>
        <el-button :icon="Download" @click="exportWorkflow" size="small">
          导出
        </el-button>
        <el-button :icon="RefreshRight" @click="clearCanvas" size="small">
          清空
        </el-button>
      </div>

      <div class="toolbar-section zoom-controls">
        <el-button :icon="ZoomOut" circle size="small" @click="zoomOut" />
        <span class="zoom-level">{{ Math.round(scale * 100) }}%</span>
        <el-button :icon="ZoomIn" circle size="small" @click="zoomIn" />
        <el-button :icon="FullScreen" circle size="small" @click="resetView" />
      </div>
    </div>

    <div class="canvas-container" ref="canvasRef">
      <div
        class="canvas-content"
        :style="{ transform: `translate(${panX}px, ${panY}px) scale(${scale})` }"
        @mousedown="handleCanvasMouseDown"
        @mousemove="handleCanvasMouseMove"
        @mouseup="handleCanvasMouseUp"
        @drop="handleDrop"
        @dragover.prevent
      >
        <svg class="connections-layer">
          <defs>
            <marker id="arrow" markerWidth="10" markerHeight="10" refX="9" refY="3" orient="auto">
              <polygon points="0 0, 10 3, 0 6" fill="#3b82f6" />
            </marker>
            <marker id="arrow-hover" markerWidth="10" markerHeight="10" refX="9" refY="3" orient="auto">
              <polygon points="0 0, 10 3, 0 6" fill="#1d4ed8" />
            </marker>
          </defs>

          <path
            v-for="conn in connections"
            :key="conn.id"
            :d="getConnectionPath(conn)"
            :class="['connection-line', { active: hoveredConnection === conn.id }]"
            :marker-end="hoveredConnection === conn.id ? 'url(#arrow-hover)' : 'url(#arrow)'"
            @mouseenter="hoveredConnection = conn.id"
            @mouseleave="hoveredConnection = null"
            @click="deleteConnection(conn.id)"
          />

          <path
            v-if="isDrawingConnection"
            :d="getTempConnectionPath()"
            class="connection-line temp"
          />
        </svg>

        <div
          v-for="node in nodes"
          :key="node.id"
          :class="['workflow-node', node.type, { selected: selectedNode?.id === node.id }]"
          :style="{ left: `${node.x}px`, top: `${node.y}px` }"
          @mousedown.stop="startNodeDrag($event, node)"
          @click.stop="selectNode(node)"
        >
          <div class="node-header">
            <el-icon class="node-icon"><component :is="getNodeIcon(node.type)" /></el-icon>
            <div class="node-title">
              <div class="node-name">{{ node.name }}</div>
              <div class="node-type-label">{{ getNodeTypeName(node.type) }}</div>
            </div>
            <div class="node-actions">
              <el-button :icon="Plus" circle size="small" @click.stop="addChildNode(node)" />
              <el-button :icon="Delete" circle size="small" type="danger" @click.stop="deleteNode(node.id)" />
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

          <div class="port port-input" @mousedown.stop="startConnection($event, node, 'input')">
            <div class="port-dot"></div>
          </div>
          <div class="port port-output" @mousedown.stop="startConnection($event, node, 'output')">
            <div class="port-dot"></div>
          </div>
        </div>
      </div>
    </div>

    <div v-if="selectedNode" class="properties-sidebar">
      <div class="sidebar-header">
        <h3>节点属性</h3>
        <el-button :icon="Close" circle size="small" @click="selectedNode = null" />
      </div>
      <div class="sidebar-content">
        <el-form label-position="top" size="small">
          <el-form-item label="节点名称">
            <el-input v-model="selectedNode.name" @change="notifyUpdate" />
          </el-form-item>
          <el-form-item label="节点类型">
            <el-tag :type="getNodeTagType(selectedNode.type)">
              {{ getNodeTypeName(selectedNode.type) }}
            </el-tag>
          </el-form-item>
          <el-form-item label="状态">
            <el-select v-model="selectedNode.status" @change="notifyUpdate">
              <el-option label="待执行" value="pending" />
              <el-option label="运行中" value="running" />
              <el-option label="成功" value="success" />
              <el-option label="失败" value="failed" />
            </el-select>
          </el-form-item>

          <el-divider />

          <template v-if="selectedNode.type === 'http'">
            <el-form-item label="URL">
              <el-input v-model="selectedNode.config!.url" placeholder="https://api.example.com" @change="notifyUpdate" />
            </el-form-item>
            <el-form-item label="方法">
              <el-select v-model="selectedNode.config!.method" @change="notifyUpdate">
                <el-option label="GET" value="GET" />
                <el-option label="POST" value="POST" />
                <el-option label="PUT" value="PUT" />
                <el-option label="DELETE" value="DELETE" />
              </el-select>
            </el-form-item>
            <el-form-item label="超时(秒)">
              <el-input-number v-model="selectedNode.config!.timeout" :min="1" :max="300" @change="notifyUpdate" />
            </el-form-item>
          </template>

          <template v-else-if="selectedNode.type === 'shell'">
            <el-form-item label="脚本内容">
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
            <el-form-item label="延迟时间(秒)">
              <el-input-number v-model="selectedNode.config!.delay" :min="1" :max="3600" @change="notifyUpdate" />
            </el-form-item>
          </template>

          <template v-else-if="selectedNode.type === 'condition'">
            <el-form-item label="条件表达式">
              <el-input
                v-model="selectedNode.config!.condition"
                type="textarea"
                placeholder="例如: status == 'success'"
                @change="notifyUpdate"
              />
            </el-form-item>
          </template>

          <template v-else-if="selectedNode.type === 'webhook'">
            <el-form-item label="Webhook URL">
              <el-input v-model="selectedNode.config!.url" placeholder="https://webhook.example.com" @change="notifyUpdate" />
            </el-form-item>
            <el-form-item label="触发条件">
              <el-select v-model="selectedNode.config!.trigger" @change="notifyUpdate">
                <el-option label="任务完成" value="completed" />
                <el-option label="任务失败" value="failed" />
                <el-option label="任何状态" value="all" />
              </el-select>
            </el-form-item>
          </template>
        </el-form>
      </div>
    </div>

    <div class="canvas-statusbar">
      <div class="status-left">
        <span><el-icon><Document /></el-icon> {{ nodes.length }} 个节点</span>
        <span><el-icon><Connection /></el-icon> {{ connections.length }} 条连接</span>
      </div>
      <div class="status-right">
        <span class="status-mode"><el-icon><EditPen /></el-icon> 编辑模式</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, watch, onMounted } from 'vue'
import {
  Plus,
  Delete,
  Close,
  EditPen,
  Document,
  Connection,
  ZoomIn,
  ZoomOut,
  FullScreen,
  MagicStick,
  Download,
  RefreshRight,
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
const selectedNode = ref<WorkflowNode | null>(null)
const hoveredConnection = ref<string | null>(null)

const scale = ref(1)
const panX = ref(0)
const panY = ref(0)

const isDraggingNode = ref(false)
const isDraggingCanvas = ref(false)
const isDrawingConnection = ref(false)
const drawingFrom = ref<{ node: WorkflowNode; port: 'input' | 'output' } | null>(null)
const tempMousePos = reactive({ x: 0, y: 0 })
const lastMousePos = reactive({ x: 0, y: 0 })

const nodeTypes = [
  { type: 'http', label: 'HTTP请求', icon: Link },
  { type: 'shell', label: 'Shell脚本', icon: Monitor },
  { type: 'delay', label: '延迟等待', icon: Clock },
  { type: 'condition', label: '条件判断', icon: Position },
  { type: 'webhook', label: 'Webhook', icon: Bell }
]

const getNodeIcon = (type: string) => {
  const typeIcon: Record<string, any> = {
    http: Link,
    shell: Monitor,
    delay: Clock,
    condition: Position,
    webhook: Bell
  }
  return typeIcon[type] || Link
}

const getNodeTypeName = (type: string) => {
  const names: Record<string, string> = {
    http: 'HTTP请求',
    shell: 'Shell脚本',
    delay: '延迟等待',
    condition: '条件判断',
    webhook: 'Webhook'
  }
  return names[type] || type
}

const getNodeTagType = (type: string) => {
  const types: Record<string, string> = {
    http: 'primary',
    shell: 'warning',
    delay: 'info',
    condition: 'success',
    webhook: 'danger'
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
      return '配置'
  }
}

const notifyUpdate = () => {
  emit('update', { nodes: nodes.value, connections: connections.value })
}

const handleDragStart = (e: DragEvent, nodeType: any) => {
  e.dataTransfer?.setData('nodeType', JSON.stringify(nodeType))
}

const handleDrop = (e: DragEvent) => {
  const nodeTypeData = e.dataTransfer?.getData('nodeType')
  if (!nodeTypeData) return

  const nodeType = JSON.parse(nodeTypeData)
  const rect = canvasRef.value?.getBoundingClientRect()
  if (!rect) return

  const x = (e.clientX - rect.left - panX.value) / scale.value
  const y = (e.clientY - rect.top - panY.value) / scale.value

  addNodeAtPosition(nodeType.type, x, y)
}

const addNodeAtPosition = (type: string, x: number, y: number) => {
  const newNode: WorkflowNode = {
    id: `node_${Date.now()}`,
    name: `新${getNodeTypeName(type)}`,
    type: type as any,
    x: x - 100,
    y: y - 50,
    status: 'pending',
    config: type === 'http' ? { method: 'GET', url: '', timeout: 30 } :
            type === 'shell' ? { script: '' } :
            type === 'delay' ? { delay: 5 } :
            type === 'condition' ? { condition: '' } :
            { url: '', trigger: 'completed' }
  }

  nodes.value.push(newNode)
  selectedNode.value = newNode
  notifyUpdate()
}

const addChildNode = (parentNode: WorkflowNode) => {
  const childNode: WorkflowNode = {
    id: `node_${Date.now()}`,
    name: `新${getNodeTypeName('http')}`,
    type: 'http',
    x: parentNode.x,
    y: parentNode.y + 160,
    status: 'pending',
    config: { method: 'GET', url: '', timeout: 30 }
  }

  nodes.value.push(childNode)
  connections.value.push({
    id: `conn_${Date.now()}`,
    from: parentNode.id,
    to: childNode.id
  })
  selectedNode.value = childNode
  notifyUpdate()
}

const deleteNode = (nodeId: string) => {
  nodes.value = nodes.value.filter(n => n.id !== nodeId)
  connections.value = connections.value.filter(c => c.from !== nodeId && c.to !== nodeId)
  if (selectedNode.value?.id === nodeId) {
    selectedNode.value = null
  }
  notifyUpdate()
}

const selectNode = (node: WorkflowNode) => {
  selectedNode.value = node
}

const startNodeDrag = (e: MouseEvent, _node: WorkflowNode) => {
  isDraggingNode.value = true
  lastMousePos.x = e.clientX
  lastMousePos.y = e.clientY
}

const startConnection = (e: MouseEvent, node: WorkflowNode, port: 'input' | 'output') => {
  isDrawingConnection.value = true
  drawingFrom.value = { node, port }
  lastMousePos.x = e.clientX
  lastMousePos.y = e.clientY
}

const handleCanvasMouseDown = (e: MouseEvent) => {
  if (e.button === 0) {
    isDraggingCanvas.value = true
    lastMousePos.x = e.clientX
    lastMousePos.y = e.clientY
    selectedNode.value = null
  }
}

const handleCanvasMouseMove = (e: MouseEvent) => {
  if (isDraggingCanvas.value) {
    const dx = e.clientX - lastMousePos.x
    const dy = e.clientY - lastMousePos.y
    panX.value += dx
    panY.value += dy
    lastMousePos.x = e.clientX
    lastMousePos.y = e.clientY
  }

  if (isDraggingNode.value && selectedNode.value) {
    const dx = (e.clientX - lastMousePos.x) / scale.value
    const dy = (e.clientY - lastMousePos.y) / scale.value
    selectedNode.value.x += dx
    selectedNode.value.y += dy
    lastMousePos.x = e.clientX
    lastMousePos.y = e.clientY
  }

  if (isDrawingConnection.value) {
    const rect = canvasRef.value?.getBoundingClientRect()
    if (rect) {
      tempMousePos.x = (e.clientX - rect.left - panX.value) / scale.value
      tempMousePos.y = (e.clientY - rect.top - panY.value) / scale.value
    }
  }
}

const handleCanvasMouseUp = (e: MouseEvent) => {
  if (isDrawingConnection.value && drawingFrom.value) {
    const targetNode = findNodeAtPosition(e.clientX, e.clientY)
    if (targetNode && targetNode.id !== drawingFrom.value.node.id) {
      const fromNode = drawingFrom.value.port === 'output' ? drawingFrom.value.node : targetNode
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

  if (isDraggingNode.value) {
    notifyUpdate()
  }

  isDraggingNode.value = false
  isDraggingCanvas.value = false
  isDrawingConnection.value = false
  drawingFrom.value = null
}

const findNodeAtPosition = (clientX: number, clientY: number): WorkflowNode | null => {
  const rect = canvasRef.value?.getBoundingClientRect()
  if (!rect) return null

  const x = (clientX - rect.left - panX.value) / scale.value
  const y = (clientY - rect.top - panY.value) / scale.value

  return nodes.value.find(n => {
    const nodeWidth = 200
    const nodeHeight = 100
    return x >= n.x && x <= n.x + nodeWidth && y >= n.y && y <= n.y + nodeHeight
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

const getTempConnectionPath = () => {
  if (!drawingFrom.value) return ''

  const fromNode = drawingFrom.value.node
  const fromX = drawingFrom.value.port === 'output' ? fromNode.x + 200 : fromNode.x
  const fromY = fromNode.y + 50

  const controlOffset = Math.abs(tempMousePos.x - fromX) * 0.5
  return `M ${fromX} ${fromY} C ${fromX + controlOffset} ${fromY}, ${tempMousePos.x - controlOffset} ${tempMousePos.y}, ${tempMousePos.x} ${tempMousePos.y}`
}

const deleteConnection = (connId: string) => {
  connections.value = connections.value.filter(c => c.id !== connId)
  notifyUpdate()
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
  const visited = new Set<string>()
  const inDegree = new Map<string, number>()

  nodes.value.forEach(n => inDegree.set(n.id, 0))
  connections.value.forEach(c => {
    inDegree.set(c.to, (inDegree.get(c.to) || 0) + 1)
  })

  const layers: WorkflowNode[][] = []
  let currentLayer = nodes.value.filter(n => (inDegree.get(n.id) || 0) === 0)

  let layerIndex = 0
  while (currentLayer.length > 0) {
    layers.push(currentLayer)
    const nextLayer: WorkflowNode[] = []

    currentLayer.forEach(node => {
      visited.add(node.id)
      const outgoing = connections.value.filter(c => c.from === node.id)

      outgoing.forEach(c => {
        const newInDegree = (inDegree.get(c.to) || 0) - 1
        inDegree.set(c.to, newInDegree)
        if (newInDegree === 0) {
          const nextNode = nodes.value.find(n => n.id === c.to)
          if (nextNode && !visited.has(nextNode.id)) {
            nextLayer.push(nextNode)
          }
        }
      })
    })

    currentLayer = nextLayer
    layerIndex++
  }

  const remaining = nodes.value.filter(n => !visited.has(n.id))
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

  panX.value = 0
  panY.value = 0
  scale.value = 1

  notifyUpdate()
}

const clearCanvas = () => {
  nodes.value = []
  connections.value = []
  selectedNode.value = null
  notifyUpdate()
}

const exportWorkflow = () => {
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
  if (props.initialDag && props.initialDag.nodes) {
    nodes.value = JSON.parse(JSON.stringify(props.initialDag.nodes))
    connections.value = JSON.parse(JSON.stringify(props.initialDag.connections || []))
  }
})
</script>

<style scoped>
.workflow-canvas {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: #f8fafc;
  border-radius: 12px;
  overflow: hidden;
}

.canvas-toolbar {
  display: flex;
  align-items: center;
  gap: 24px;
  padding: 12px 16px;
  background: white;
  border-bottom: 1px solid #e2e8f0;
  flex-wrap: wrap;
}

.toolbar-section {
  display: flex;
  align-items: center;
  gap: 12px;
}

.section-title {
  font-size: 0.8rem;
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
  transition: all 0.2s ease;
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

.toolbar-divider {
  width: 1px;
  height: 28px;
  background: #e2e8f0;
}

.zoom-controls {
  gap: 8px;
}

.zoom-level {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.75rem;
  color: #64748b;
  min-width: 48px;
  text-align: center;
}

.canvas-container {
  flex: 1;
  overflow: hidden;
  position: relative;
  background: radial-gradient(circle at 0 0, rgba(59, 130, 246, 0.03) 0, transparent 50%),
              radial-gradient(circle at 100% 0, rgba(139, 92, 246, 0.03) 0, transparent 50%);
}

.canvas-content {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  transform-origin: 0 0;
  cursor: grab;
}

.connections-layer {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  pointer-events: none;
}

.connections-layer path {
  pointer-events: stroke;
}

.connection-line {
  stroke: #cbd5e1;
  stroke-width: 2;
  fill: none;
  transition: all 0.2s ease;
  cursor: pointer;
}

.connection-line:hover,
.connection-line.active {
  stroke: #3b82f6;
  stroke-width: 3;
}

.connection-line.temp {
  stroke: #3b82f6;
  stroke-dasharray: 8, 4;
  opacity: 0.6;
}

.workflow-node {
  position: absolute;
  width: 200px;
  background: white;
  border: 2px solid #e2e8f0;
  border-radius: 12px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05);
  cursor: grab;
  transition: all 0.2s ease;
  user-select: none;
}

.workflow-node:hover {
  border-color: #3b82f6;
  box-shadow: 0 8px 24px rgba(59, 130, 246, 0.15);
  transform: translateY(-2px);
}

.workflow-node.selected {
  border-color: #3b82f6;
  box-shadow: 0 0 0 4px rgba(59, 130, 246, 0.2), 0 12px 32px rgba(0, 0, 0, 0.12);
}

.workflow-node.success {
  border-color: #22c55e;
}

.workflow-node.failed {
  border-color: #ef4444;
}

.workflow-node.running {
  border-color: #f59e0b;
}

.node-header {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px;
  border-bottom: 1px solid #f1f5f9;
  background: linear-gradient(135deg, #f8fafc, white);
  border-radius: 10px 10px 0 0;
}

.node-icon {
  width: 36px;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #3b82f6, #2563eb);
  color: white;
  border-radius: 8px;
  font-size: 1.1rem;
}

.workflow-node.shell .node-icon {
  background: linear-gradient(135deg, #f59e0b, #d97706);
}

.workflow-node.delay .node-icon {
  background: linear-gradient(135deg, #8b5cf6, #7c3aed);
}

.workflow-node.condition .node-icon {
  background: linear-gradient(135deg, #22c55e, #16a34a);
}

.workflow-node.webhook .node-icon {
  background: linear-gradient(135deg, #ef4444, #dc2626);
}

.node-title {
  flex: 1;
  min-width: 0;
}

.node-name {
  font-weight: 600;
  font-size: 0.9rem;
  color: #1e293b;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.node-type-label {
  font-size: 0.7rem;
  color: #94a3b8;
  text-transform: uppercase;
  letter-spacing: 0.03em;
}

.node-actions {
  display: flex;
  gap: 4px;
  opacity: 0;
  transition: opacity 0.2s ease;
}

.workflow-node:hover .node-actions {
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
  background: #f8fafc;
  border-radius: 0 0 10px 10px;
}

.status-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: 9999px;
  font-size: 0.7rem;
  font-weight: 500;
}

.status-badge.pending {
  background: #f1f5f9;
  color: #64748b;
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

.port-dot {
  width: 12px;
  height: 12px;
  background: white;
  border: 2px solid #3b82f6;
  border-radius: 50%;
  transition: all 0.2s ease;
}

.port:hover .port-dot {
  background: #3b82f6;
  transform: scale(1.3);
  box-shadow: 0 0 12px rgba(59, 130, 246, 0.6);
}

.properties-sidebar {
  position: absolute;
  right: 0;
  top: 72px;
  bottom: 40px;
  width: 320px;
  background: white;
  border-left: 1px solid #e2e8f0;
  display: flex;
  flex-direction: column;
  z-index: 100;
  box-shadow: -4px 0 24px rgba(0, 0, 0, 0.08);
}

.sidebar-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px;
  border-bottom: 1px solid #f1f5f9;
}

.sidebar-header h3 {
  margin: 0;
  font-size: 1rem;
  font-weight: 600;
  color: #1e293b;
}

.sidebar-content {
  flex: 1;
  padding: 16px;
  overflow-y: auto;
}

.canvas-statusbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 16px;
  background: white;
  border-top: 1px solid #e2e8f0;
  font-size: 0.75rem;
  color: #64748b;
}

.status-left {
  display: flex;
  gap: 24px;
}

.status-left span {
  display: flex;
  align-items: center;
  gap: 6px;
}

.status-mode {
  display: flex;
  align-items: center;
  gap: 6px;
  color: #3b82f6;
  font-weight: 500;
}
</style>