<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { VueFlow, useVueFlow } from '@vue-flow/core'
import { Background } from '@vue-flow/background'
import { Controls } from '@vue-flow/controls'
import { MiniMap } from '@vue-flow/minimap'
import { useI18n } from 'vue-i18n'
import { useCompanionStore } from '@/stores/companion'
import type { FlowGraph, FlowNode as ApiFlowNode } from '@/api/companion'

// Import custom nodes
import MessageNode from './nodes/MessageNode.vue'
import ToolCallNode from './nodes/ToolCallNode.vue'
import LLMRequestNode from './nodes/LLMRequestNode.vue'
import SecurityCheckNode from './nodes/SecurityCheckNode.vue'

// Import VueFlow styles
import '@vue-flow/core/dist/style.css'
import '@vue-flow/core/dist/theme-default.css'
import '@vue-flow/controls/dist/style.css'
import '@vue-flow/minimap/dist/style.css'

const { t } = useI18n()
const companionStore = useCompanionStore()

const props = defineProps<{
  sessionId: string
}>()

const emit = defineEmits<{
  nodeClick: [node: ApiFlowNode]
}>()

// VueFlow instance
const { fitView, zoomIn, zoomOut } = useVueFlow()

// Local state
const loading = ref(false)
const error = ref<string | null>(null)
const selectedNode = ref<ApiFlowNode | null>(null)

// Flow data
const flowGraph = computed(() => companionStore.sessionFlow)

// Convert API flow data to VueFlow format
const nodes = computed(() => {
  if (!flowGraph.value?.nodes) return []

  return flowGraph.value.nodes.map((node) => ({
    id: node.id,
    type: getNodeType(node.type),
    position: node.position || { x: 100, y: 0 },
    data: {
      label: node.label,
      status: node.status,
      duration: node.duration,
      ...node.data,
    },
  }))
})

const edges = computed(() => {
  if (!flowGraph.value?.edges) return []

  return flowGraph.value.edges.map((edge) => ({
    id: edge.id,
    source: edge.source,
    target: edge.target,
    label: edge.label,
    animated: true,
    style: { stroke: '#6366f1' },
  }))
})

// Map API node types to custom component types
function getNodeType(type: string): string {
  switch (type) {
    case 'message':
      return 'message'
    case 'tool_call':
      return 'toolCall'
    case 'llm_request':
      return 'llmRequest'
    case 'security_check':
      return 'securityCheck'
    default:
      return 'default'
  }
}

// Load flow data
async function loadFlow() {
  if (!props.sessionId) return

  loading.value = true
  error.value = null

  try {
    await companionStore.fetchSessionFlow(props.sessionId)
    // Fit view after data loads
    setTimeout(() => {
      fitView({ padding: 0.2 })
    }, 100)
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to load flow'
  } finally {
    loading.value = false
  }
}

// Handle node click
function onNodeClick(_event: MouseEvent, node: { id: string; data: Record<string, unknown> }) {
  const apiNode = flowGraph.value?.nodes.find((n) => n.id === node.id)
  if (apiNode) {
    selectedNode.value = apiNode
    emit('nodeClick', apiNode)
  }
}

// Watch for session changes
watch(() => props.sessionId, loadFlow, { immediate: true })

onMounted(() => {
  if (props.sessionId) {
    loadFlow()
  }
})

// Custom node types
const nodeTypes = {
  message: MessageNode,
  toolCall: ToolCallNode,
  llmRequest: LLMRequestNode,
  securityCheck: SecurityCheckNode,
}
</script>

<template>
  <div class="flow-viewer h-full w-full relative">
    <!-- Loading state -->
    <div
      v-if="loading"
      class="absolute inset-0 flex items-center justify-center bg-white/80 dark:bg-gray-900/80 z-10"
    >
      <div class="flex items-center gap-3 text-gray-600 dark:text-gray-400">
        <svg class="animate-spin h-5 w-5" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
        </svg>
        <span>{{ t('companion.flow.loading') }}</span>
      </div>
    </div>

    <!-- Error state -->
    <div
      v-else-if="error"
      class="absolute inset-0 flex items-center justify-center"
    >
      <div class="text-center">
        <svg class="mx-auto h-12 w-12 text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
        </svg>
        <p class="mt-2 text-sm text-gray-600 dark:text-gray-400">{{ error }}</p>
        <button
          class="mt-4 px-4 py-2 text-sm bg-accent text-white rounded-lg hover:bg-accent/90"
          @click="loadFlow"
        >
          {{ t('companion.flow.retry') }}
        </button>
      </div>
    </div>

    <!-- Empty state -->
    <div
      v-else-if="!nodes.length"
      class="absolute inset-0 flex items-center justify-center"
    >
      <div class="text-center">
        <svg class="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 17V7m0 10a2 2 0 01-2 2H5a2 2 0 01-2-2V7a2 2 0 012-2h2a2 2 0 012 2m0 10a2 2 0 002 2h2a2 2 0 002-2M9 7a2 2 0 012-2h2a2 2 0 012 2m0 10V7m0 10a2 2 0 002 2h2a2 2 0 002-2V7a2 2 0 00-2-2h-2a2 2 0 00-2 2" />
        </svg>
        <p class="mt-2 text-sm text-gray-600 dark:text-gray-400">{{ t('companion.flow.empty') }}</p>
      </div>
    </div>

    <!-- Flow diagram -->
    <VueFlow
      v-else
      :nodes="nodes"
      :edges="edges"
      :node-types="nodeTypes"
      :default-viewport="{ x: 0, y: 0, zoom: 1 }"
      :min-zoom="0.1"
      :max-zoom="2"
      fit-view-on-init
      class="bg-gray-50 dark:bg-gray-900"
      @node-click="onNodeClick"
    >
      <Background pattern-color="#e5e7eb" :gap="20" />
      <Controls position="top-right" />
      <MiniMap
        position="bottom-right"
        :node-color="(node: { type?: string }) => {
          switch (node.type) {
            case 'message': return '#3b82f6'
            case 'toolCall': return '#8b5cf6'
            case 'llmRequest': return '#6366f1'
            case 'securityCheck': return '#ef4444'
            default: return '#9ca3af'
          }
        }"
      />
    </VueFlow>

    <!-- Zoom controls (custom) -->
    <div class="absolute bottom-4 left-4 flex gap-2 z-10">
      <button
        class="p-2 bg-white dark:bg-gray-800 rounded-lg shadow-md hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
        @click="zoomIn()"
        :title="t('companion.flow.zoomIn')"
      >
        <svg class="w-4 h-4 text-gray-600 dark:text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0zM10 7v3m0 0v3m0-3h3m-3 0H7" />
        </svg>
      </button>
      <button
        class="p-2 bg-white dark:bg-gray-800 rounded-lg shadow-md hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
        @click="zoomOut()"
        :title="t('companion.flow.zoomOut')"
      >
        <svg class="w-4 h-4 text-gray-600 dark:text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0zM13 10H7" />
        </svg>
      </button>
      <button
        class="p-2 bg-white dark:bg-gray-800 rounded-lg shadow-md hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
        @click="fitView({ padding: 0.2 })"
        :title="t('companion.flow.fitView')"
      >
        <svg class="w-4 h-4 text-gray-600 dark:text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4" />
        </svg>
      </button>
    </div>

    <!-- Legend -->
    <div class="absolute top-4 left-4 bg-white dark:bg-gray-800 rounded-lg shadow-md p-3 z-10">
      <div class="text-xs font-medium text-gray-600 dark:text-gray-400 mb-2">{{ t('companion.flow.legend') }}</div>
      <div class="space-y-1.5">
        <div class="flex items-center gap-2">
          <div class="w-3 h-3 rounded bg-blue-500" />
          <span class="text-xs text-gray-600 dark:text-gray-400">{{ t('companion.flow.nodeTypes.message') }}</span>
        </div>
        <div class="flex items-center gap-2">
          <div class="w-3 h-3 rounded bg-purple-500" />
          <span class="text-xs text-gray-600 dark:text-gray-400">{{ t('companion.flow.nodeTypes.toolCall') }}</span>
        </div>
        <div class="flex items-center gap-2">
          <div class="w-3 h-3 rounded bg-indigo-500" />
          <span class="text-xs text-gray-600 dark:text-gray-400">{{ t('companion.flow.nodeTypes.llmRequest') }}</span>
        </div>
        <div class="flex items-center gap-2">
          <div class="w-3 h-3 rounded bg-red-500" />
          <span class="text-xs text-gray-600 dark:text-gray-400">{{ t('companion.flow.nodeTypes.securityCheck') }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.flow-viewer {
  min-height: 400px;
}

:deep(.vue-flow__node) {
  cursor: pointer;
}

:deep(.vue-flow__edge-path) {
  stroke-width: 2;
}

:deep(.vue-flow__controls) {
  display: none; /* Using custom controls */
}
</style>
