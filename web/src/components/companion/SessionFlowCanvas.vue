<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import type { SessionEvent } from '@/api/companion'

const { t } = useI18n()

// Props
const props = defineProps<{
  events: SessionEvent[]
  selectedEventId?: string
  autoScroll?: boolean
}>()

// Emits
const emit = defineEmits<{
  nodeClick: [event: SessionEvent]
  nodeHover: [event: SessionEvent | null]
}>()

// Canvas refs
const canvasRef = ref<HTMLCanvasElement | null>(null)
const containerRef = ref<HTMLDivElement | null>(null)

// State
const scale = ref(1)
const offsetX = ref(0)
const offsetY = ref(0)
const isDragging = ref(false)
const dragStart = ref({ x: 0, y: 0 })
const hoveredNode = ref<SessionEvent | null>(null)

// Node dimensions
const NODE_WIDTH = 360
const NODE_HEIGHT = 72
const NODE_MARGIN_X = 40
const NODE_MARGIN_Y = 24
const BORDER_RADIUS = 12

// Colors by event type
const nodeColors: Record<string, { bg: string; border: string; icon: string; text: string }> = {
  message_received: { bg: '#dbeafe', border: '#3b82f6', icon: '#2563eb', text: '#1e40af' },
  message_sent: { bg: '#dcfce7', border: '#22c55e', icon: '#16a34a', text: '#166534' },
  tool_call: { bg: '#f3e8ff', border: '#a855f7', icon: '#9333ea', text: '#6b21a8' },
  llm_request: { bg: '#e0e7ff', border: '#6366f1', icon: '#4f46e5', text: '#3730a3' },
  security_threat: { bg: '#fee2e2', border: '#ef4444', icon: '#dc2626', text: '#991b1b' },
  session_start: { bg: '#f0fdf4', border: '#22c55e', icon: '#16a34a', text: '#166534' },
  session_end: { bg: '#fef2f2', border: '#ef4444', icon: '#dc2626', text: '#991b1b' },
  sandbox_exec: { bg: '#fef3c7', border: '#f59e0b', icon: '#d97706', text: '#92400e' },
  error: { bg: '#fee2e2', border: '#ef4444', icon: '#dc2626', text: '#991b1b' },
  custom: { bg: '#f3f4f6', border: '#9ca3af', icon: '#6b7280', text: '#374151' },
}

// Status colors
const statusColors: Record<string, string> = {
  success: '#22c55e',
  completed: '#22c55e',
  running: '#f59e0b',
  pending: '#f59e0b',
  failed: '#ef4444',
  error: '#ef4444',
  blocked: '#ef4444',
}

// Computed nodes with positions
interface CanvasNode {
  event: SessionEvent
  x: number
  y: number
  width: number
  height: number
}

const nodes = computed<CanvasNode[]>(() => {
  return props.events.map((event, index) => ({
    event,
    x: NODE_MARGIN_X,
    y: NODE_MARGIN_Y + index * (NODE_HEIGHT + NODE_MARGIN_Y),
    width: NODE_WIDTH,
    height: NODE_HEIGHT,
  }))
})

// Canvas dimensions
const canvasWidth = computed(() => NODE_WIDTH + NODE_MARGIN_X * 2)
const canvasHeight = computed(() => {
  if (nodes.value.length === 0) return 400
  return nodes.value.length * (NODE_HEIGHT + NODE_MARGIN_Y) + NODE_MARGIN_Y
})

// Helper functions
function getNodeLabel(event: SessionEvent): string {
  switch (event.event_type) {
    case 'message_received':
      return event.message?.content?.substring(0, 40) || t('companion.eventType.message_received')
    case 'message_sent':
      return event.message?.content?.substring(0, 40) || t('companion.eventType.message_sent')
    case 'tool_call':
      return event.tool_call?.toolName || t('companion.eventType.tool_call')
    case 'llm_request':
      return `${event.llm_request?.provider || 'LLM'}: ${event.llm_request?.model || t('companion.eventType.llm_request')}`
    case 'security_threat':
      return `${t('companion.eventType.security_threat')}: ${event.security?.threatTypes?.join(', ') || ''}`
    case 'session_start':
      return t('companion.eventType.session_start')
    case 'session_end':
      return t('companion.eventType.session_end')
    case 'sandbox_exec':
      return t('companion.eventType.sandbox_exec')
    default:
      return event.event_type || t('companion.eventType.unknown')
  }
}

function getNodeSubtitle(event: SessionEvent): string {
  switch (event.event_type) {
    case 'message_received':
    case 'message_sent':
      return `${event.message?.content?.length || 0} chars`
    case 'tool_call':
      return event.tool_call?.status || ''
    case 'llm_request':
      return `${event.llm_request?.totalTokens || 0} tokens`
    case 'security_threat':
      return event.security?.action || ''
    default:
      return ''
  }
}

function formatTime(timestamp: string): string {
  return new Date(timestamp).toLocaleTimeString()
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`
  return `${(ms / 1000).toFixed(2)}s`
}

// Draw rounded rectangle
function drawRoundedRect(
  ctx: CanvasRenderingContext2D,
  x: number,
  y: number,
  width: number,
  height: number,
  radius: number
) {
  ctx.beginPath()
  ctx.moveTo(x + radius, y)
  ctx.lineTo(x + width - radius, y)
  ctx.quadraticCurveTo(x + width, y, x + width, y + radius)
  ctx.lineTo(x + width, y + height - radius)
  ctx.quadraticCurveTo(x + width, y + height, x + width - radius, y + height)
  ctx.lineTo(x + radius, y + height)
  ctx.quadraticCurveTo(x, y + height, x, y + height - radius)
  ctx.lineTo(x, y + radius)
  ctx.quadraticCurveTo(x, y, x + radius, y)
  ctx.closePath()
}

// Draw a single node
type NodeColorSet = { bg: string; border: string; icon: string; text: string }
function drawNode(ctx: CanvasRenderingContext2D, node: CanvasNode, isSelected: boolean, isHovered: boolean) {
  const { event, x, y, width, height } = node
  const colors: NodeColorSet = (nodeColors[event.event_type] ?? nodeColors.custom) as NodeColorSet

  // Shadow
  ctx.shadowColor = 'rgba(0, 0, 0, 0.1)'
  ctx.shadowBlur = isHovered ? 12 : 6
  ctx.shadowOffsetX = 0
  ctx.shadowOffsetY = isHovered ? 4 : 2

  // Background
  drawRoundedRect(ctx, x, y, width, height, BORDER_RADIUS)
  ctx.fillStyle = colors.bg
  ctx.fill()

  // Border
  ctx.shadowColor = 'transparent'
  ctx.strokeStyle = isSelected ? '#2563eb' : colors.border
  ctx.lineWidth = isSelected ? 3 : 2
  ctx.stroke()

  // Left accent bar
  ctx.fillStyle = colors.border
  drawRoundedRect(ctx, x, y, 6, height, BORDER_RADIUS)
  ctx.fill()
  ctx.fillRect(x + 3, y, 3, height)

  // Icon circle
  const iconX = x + 24
  const iconY = y + height / 2
  ctx.beginPath()
  ctx.arc(iconX, iconY, 14, 0, Math.PI * 2)
  ctx.fillStyle = colors.border
  ctx.fill()

  // Event type icon (simplified)
  ctx.fillStyle = '#ffffff'
  ctx.font = 'bold 12px system-ui'
  ctx.textAlign = 'center'
  ctx.textBaseline = 'middle'
  const iconChar = getIconChar(event.event_type)
  ctx.fillText(iconChar, iconX, iconY)

  // Title
  ctx.fillStyle = colors.text
  ctx.font = 'bold 13px system-ui'
  ctx.textAlign = 'left'
  ctx.textBaseline = 'top'
  const label = getNodeLabel(event)
  const truncatedLabel = label.length > 30 ? label.substring(0, 30) + '...' : label
  ctx.fillText(truncatedLabel, x + 48, y + 14)

  // Subtitle
  ctx.fillStyle = '#6b7280'
  ctx.font = '11px system-ui'
  const subtitle = getNodeSubtitle(event)
  ctx.fillText(subtitle, x + 48, y + 32)

  // Time
  ctx.fillStyle = '#9ca3af'
  ctx.font = '10px system-ui'
  ctx.fillText(formatTime(event.timestamp), x + 48, y + 48)

  // Duration badge
  if (event.duration) {
    const durationText = formatDuration(event.duration)
    ctx.fillStyle = '#f3f4f6'
    const badgeWidth = ctx.measureText(durationText).width + 12
    drawRoundedRect(ctx, x + width - badgeWidth - 12, y + 12, badgeWidth, 20, 4)
    ctx.fill()
    ctx.fillStyle = '#6b7280'
    ctx.font = '10px system-ui'
    ctx.textAlign = 'center'
    ctx.fillText(durationText, x + width - badgeWidth / 2 - 12, y + 22)
  }

  // Status indicator
  const statusColor = statusColors[event.status] || '#9ca3af'
  ctx.beginPath()
  ctx.arc(x + width - 16, y + height - 16, 5, 0, Math.PI * 2)
  ctx.fillStyle = statusColor
  ctx.fill()
}

function getIconChar(eventType: string): string {
  switch (eventType) {
    case 'message_received': return '↓'
    case 'message_sent': return '↑'
    case 'tool_call': return '⚡'
    case 'llm_request': return '🤖'
    case 'security_threat': return '⚠'
    case 'session_start': return '▶'
    case 'session_end': return '■'
    case 'sandbox_exec': return '📦'
    case 'error': return '✕'
    default: return '•'
  }
}

// Draw connector line between nodes
function drawConnector(ctx: CanvasRenderingContext2D, fromNode: CanvasNode, toNode: CanvasNode) {
  const startX = fromNode.x + fromNode.width / 2
  const startY = fromNode.y + fromNode.height
  const endX = toNode.x + toNode.width / 2
  const endY = toNode.y

  // Draw line
  ctx.beginPath()
  ctx.moveTo(startX, startY)
  ctx.lineTo(endX, endY)
  ctx.strokeStyle = '#d1d5db'
  ctx.lineWidth = 2
  ctx.stroke()

  // Draw arrow
  const arrowSize = 6
  const angle = Math.atan2(endY - startY, endX - startX)
  ctx.beginPath()
  ctx.moveTo(endX, endY)
  ctx.lineTo(
    endX - arrowSize * Math.cos(angle - Math.PI / 6),
    endY - arrowSize * Math.sin(angle - Math.PI / 6)
  )
  ctx.lineTo(
    endX - arrowSize * Math.cos(angle + Math.PI / 6),
    endY - arrowSize * Math.sin(angle + Math.PI / 6)
  )
  ctx.closePath()
  ctx.fillStyle = '#d1d5db'
  ctx.fill()
}

// Main draw function
function draw() {
  const canvas = canvasRef.value
  if (!canvas) return

  const ctx = canvas.getContext('2d')
  if (!ctx) return

  // Clear canvas
  ctx.clearRect(0, 0, canvas.width, canvas.height)

  // Apply transform for panning only (scaling is handled by CSS)
  ctx.save()
  ctx.translate(offsetX.value, offsetY.value)

  // Draw connectors first (behind nodes)
  for (let i = 0; i < nodes.value.length - 1; i++) {
    const fromNode = nodes.value[i]
    const toNode = nodes.value[i + 1]
    if (fromNode && toNode) {
      drawConnector(ctx, fromNode, toNode)
    }
  }

  // Draw nodes
  nodes.value.forEach((node) => {
    const isSelected = node.event.id === props.selectedEventId
    const isHovered = node.event.id === hoveredNode.value?.id
    drawNode(ctx, node, isSelected, isHovered)
  })

  ctx.restore()
}

// Find node at position
function findNodeAtPosition(x: number, y: number): CanvasNode | null {
  // Adjust for CSS scaling and panning
  const adjustedX = x / scale.value - offsetX.value
  const adjustedY = y / scale.value - offsetY.value

  for (const node of nodes.value) {
    if (
      adjustedX >= node.x &&
      adjustedX <= node.x + node.width &&
      adjustedY >= node.y &&
      adjustedY <= node.y + node.height
    ) {
      return node
    }
  }
  return null
}

// Mouse event handlers
function handleMouseDown(e: MouseEvent) {
  isDragging.value = true
  dragStart.value = { x: e.clientX - offsetX.value, y: e.clientY - offsetY.value }
}

function handleMouseMove(e: MouseEvent) {
  const canvas = canvasRef.value
  if (!canvas) return

  const rect = canvas.getBoundingClientRect()
  const x = e.clientX - rect.left
  const y = e.clientY - rect.top

  if (isDragging.value) {
    offsetX.value = e.clientX - dragStart.value.x
    offsetY.value = e.clientY - dragStart.value.y
    draw()
  } else {
    const node = findNodeAtPosition(x, y)
    if (node?.event.id !== hoveredNode.value?.id) {
      hoveredNode.value = node?.event || null
      emit('nodeHover', hoveredNode.value)
      draw()
    }
  }
}

function handleMouseUp() {
  isDragging.value = false
}

function handleClick(e: MouseEvent) {
  const canvas = canvasRef.value
  if (!canvas) return

  const rect = canvas.getBoundingClientRect()
  const x = e.clientX - rect.left
  const y = e.clientY - rect.top

  const node = findNodeAtPosition(x, y)
  if (node) {
    emit('nodeClick', node.event)
  }
}

function handleWheel(e: WheelEvent) {
  e.preventDefault()
  const delta = e.deltaY > 0 ? -0.1 : 0.1
  scale.value = Math.max(0.5, Math.min(2, scale.value + delta))
  draw()
}

// Lifecycle
onMounted(() => {
  const canvas = canvasRef.value
  if (canvas) {
    canvas.addEventListener('mousedown', handleMouseDown)
    canvas.addEventListener('mousemove', handleMouseMove)
    canvas.addEventListener('mouseup', handleMouseUp)
    canvas.addEventListener('mouseleave', handleMouseUp)
    canvas.addEventListener('click', handleClick)
    canvas.addEventListener('wheel', handleWheel, { passive: false })
  }
  draw()
})

onUnmounted(() => {
  const canvas = canvasRef.value
  if (canvas) {
    canvas.removeEventListener('mousedown', handleMouseDown)
    canvas.removeEventListener('mousemove', handleMouseMove)
    canvas.removeEventListener('mouseup', handleMouseUp)
    canvas.removeEventListener('mouseleave', handleMouseUp)
    canvas.removeEventListener('click', handleClick)
    canvas.removeEventListener('wheel', handleWheel)
  }
})

// Watch for changes
watch([() => props.events, () => props.selectedEventId, scale], () => {
  nextTick(draw)
}, { deep: true })

// Auto scroll to bottom when new events arrive
watch(() => props.events.length, (newLen, oldLen) => {
  if (props.autoScroll && newLen > oldLen) {
    nextTick(() => {
      const container = containerRef.value
      if (container) {
        container.scrollTop = container.scrollHeight
      }
    })
  }
})
</script>

<template>
  <div ref="containerRef" class="session-flow-canvas relative w-full h-full overflow-auto bg-gray-50 dark:bg-gray-900 rounded-lg">
    <!-- Canvas -->
    <canvas
      ref="canvasRef"
      class="cursor-grab active:cursor-grabbing"
      :width="canvasWidth"
      :height="canvasHeight"
      :style="{ width: canvasWidth * scale + 'px', height: canvasHeight * scale + 'px' }"
    />

    <!-- Empty state -->
    <div
      v-if="events.length === 0"
      class="absolute inset-0 flex items-center justify-center"
    >
      <div class="text-center">
        <svg class="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 17V7m0 10a2 2 0 01-2 2H5a2 2 0 01-2-2V7a2 2 0 012-2h2a2 2 0 012 2m0 10a2 2 0 002 2h2a2 2 0 002-2M9 7a2 2 0 012-2h2a2 2 0 012 2m0 10V7m0 10a2 2 0 002 2h2a2 2 0 002-2V7a2 2 0 00-2-2h-2a2 2 0 00-2 2" />
        </svg>
        <p class="mt-2 text-sm text-gray-600 dark:text-gray-400">{{ t('companion.flow.empty') }}</p>
      </div>
    </div>

    <!-- Controls -->
    <div class="absolute bottom-4 left-4 flex gap-2 z-10">
      <button
        class="p-2 bg-white dark:bg-gray-800 rounded-lg shadow-md hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
        :title="t('companion.flow.zoomIn')"
        @click="scale = Math.min(scale + 0.1, 2)"
      >
        <svg class="w-4 h-4 text-gray-600 dark:text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0zM10 7v3m0 0v3m0-3h3m-3 0H7" />
        </svg>
      </button>
      <button
        class="p-2 bg-white dark:bg-gray-800 rounded-lg shadow-md hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
        :title="t('companion.flow.zoomOut')"
        @click="scale = Math.max(scale - 0.1, 0.5)"
      >
        <svg class="w-4 h-4 text-gray-600 dark:text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0zM13 10H7" />
        </svg>
      </button>
      <button
        class="p-2 bg-white dark:bg-gray-800 rounded-lg shadow-md hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
        :title="t('companion.flow.reset')"
        @click="scale = 1; offsetX = 0; offsetY = 0"
      >
        <svg class="w-4 h-4 text-gray-600 dark:text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
        </svg>
      </button>
    </div>

    <!-- Legend - moved to bottom right to avoid blocking content -->
    <div class="absolute bottom-4 right-4 bg-white dark:bg-gray-800 rounded-lg shadow-md p-3 z-10">
      <div class="text-xs font-medium text-gray-600 dark:text-gray-400 mb-2">{{ t('companion.flow.legend') }}</div>
      <div class="grid grid-cols-2 gap-x-4 gap-y-1.5">
        <div class="flex items-center gap-2">
          <div class="w-3 h-3 rounded" style="background-color: #3b82f6" />
          <span class="text-xs text-gray-600 dark:text-gray-400">{{ t('companion.flow.nodeTypes.messageIn') }}</span>
        </div>
        <div class="flex items-center gap-2">
          <div class="w-3 h-3 rounded" style="background-color: #22c55e" />
          <span class="text-xs text-gray-600 dark:text-gray-400">{{ t('companion.flow.nodeTypes.messageOut') }}</span>
        </div>
        <div class="flex items-center gap-2">
          <div class="w-3 h-3 rounded" style="background-color: #a855f7" />
          <span class="text-xs text-gray-600 dark:text-gray-400">{{ t('companion.flow.nodeTypes.toolCall') }}</span>
        </div>
        <div class="flex items-center gap-2">
          <div class="w-3 h-3 rounded" style="background-color: #6366f1" />
          <span class="text-xs text-gray-600 dark:text-gray-400">{{ t('companion.flow.nodeTypes.llmRequest') }}</span>
        </div>
        <div class="flex items-center gap-2">
          <div class="w-3 h-3 rounded" style="background-color: #ef4444" />
          <span class="text-xs text-gray-600 dark:text-gray-400">{{ t('companion.flow.nodeTypes.security') }}</span>
        </div>
        <div class="flex items-center gap-2">
          <div class="w-3 h-3 rounded" style="background-color: #f59e0b" />
          <span class="text-xs text-gray-600 dark:text-gray-400">{{ t('companion.flow.nodeTypes.sandbox') }}</span>
        </div>
      </div>
    </div>

    <!-- Hovered node tooltip -->
    <div
      v-if="hoveredNode"
      class="absolute bg-white dark:bg-gray-800 rounded-lg shadow-lg p-3 z-20 pointer-events-none max-w-xs"
      :style="{ top: '50%', right: '16px', transform: 'translateY(-50%)' }"
    >
      <div class="text-sm font-medium text-gray-900 dark:text-white mb-1">
        {{ getNodeLabel(hoveredNode) }}
      </div>
      <div class="text-xs text-gray-500 dark:text-gray-400 space-y-1">
        <div>{{ t('companion.flow.type') }}: {{ hoveredNode.event_type }}</div>
        <div>{{ t('companion.flow.time') }}: {{ formatTime(hoveredNode.timestamp) }}</div>
        <div v-if="hoveredNode.duration">{{ t('companion.flow.duration') }}: {{ formatDuration(hoveredNode.duration) }}</div>
        <div>{{ t('companion.flow.status') }}: {{ hoveredNode.status }}</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.session-flow-canvas {
  min-height: 400px;
}

canvas {
  display: block;
}
</style>
