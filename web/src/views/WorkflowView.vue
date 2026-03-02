<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { workflowApi } from '@/api/workflow'
import type { Workflow, WorkflowExecution, WorkflowStats, WorkflowTemplate, WorkflowNode, WorkflowConnection, NodeType } from '@/api/workflow'

const { t, te } = useI18n()

const loading = ref(false)
const workflows = ref<Workflow[]>([])
const stats = ref<WorkflowStats | null>(null)
const templates = ref<WorkflowTemplate[]>([])
const selectedWorkflow = ref<Workflow | null>(null)
const executions = ref<WorkflowExecution[]>([])
const showCreateModal = ref(false)
const showExecutionsModal = ref(false)
const showTemplatesModal = ref(false)
const showNodeEditorModal = ref(false)
const showAddNodeModal = ref(false)

function tr(key: string | undefined | null, fallback = ''): string {
  if (!key) return fallback
  return te(key) ? t(key) : fallback
}

// Create form
const newWorkflow = ref({
  name: '',
  description: '',
})

// Node editor state
const editingWorkflow = ref<Workflow | null>(null)
const editingNodes = ref<WorkflowNode[]>([])
const editingConnections = ref<WorkflowConnection[]>([])
const selectedNode = ref<WorkflowNode | null>(null)

// New node form
const newNode = ref<{
  type: NodeType
  name: string
  config: Record<string, unknown>
}>({
  type: 'trigger',
  name: '',
  config: {},
})

// Available node types
const nodeTypes: { value: NodeType; labelKey: string; icon: string }[] = [
  { value: 'trigger', labelKey: 'workflow.nodeTypes.trigger', icon: 'M13 10V3L4 14h7v7l9-11h-7z' },
  { value: 'action', labelKey: 'workflow.nodeTypes.action', icon: 'M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z' },
  { value: 'condition', labelKey: 'workflow.nodeTypes.condition', icon: 'M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z' },
  { value: 'loop', labelKey: 'workflow.nodeTypes.loop', icon: 'M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15' },
  { value: 'delay', labelKey: 'workflow.nodeTypes.delay', icon: 'M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z' },
  { value: 'switch', labelKey: 'workflow.nodeTypes.switch', icon: 'M8 9l4-4 4 4m0 6l-4 4-4-4' },
  { value: 'merge', labelKey: 'workflow.nodeTypes.merge', icon: 'M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1' },
  { value: 'subflow', labelKey: 'workflow.nodeTypes.subflow', icon: 'M4 5a1 1 0 011-1h14a1 1 0 011 1v2a1 1 0 01-1 1H5a1 1 0 01-1-1V5zM4 13a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H5a1 1 0 01-1-1v-6zM16 13a1 1 0 011-1h2a1 1 0 011 1v6a1 1 0 01-1 1h-2a1 1 0 01-1-1v-6z' },
]

// Filters
const statusFilter = ref<string>('')
const searchQuery = ref('')

const filteredWorkflows = computed(() => {
  let result = workflows.value
  if (statusFilter.value) {
    result = result.filter(w => w.status === statusFilter.value)
  }
  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    result = result.filter(w =>
      w.name.toLowerCase().includes(query) ||
      w.description?.toLowerCase().includes(query)
    )
  }
  return result
})

onMounted(async () => {
  await Promise.all([loadWorkflows(), loadStats()])
})

async function loadWorkflows() {
  try {
    loading.value = true
    const response = await workflowApi.list({ limit: 100 })
    workflows.value = response.data.workflows || []
  } catch {
    workflows.value = []
  } finally {
    loading.value = false
  }
}

async function loadStats() {
  try {
    const response = await workflowApi.getStats()
    stats.value = response.data
  } catch {
    stats.value = null
  }
}

async function loadTemplates() {
  try {
    const response = await workflowApi.getTemplates()
    templates.value = response.data
    showTemplatesModal.value = true
  } catch {
    templates.value = []
  }
}

async function createWorkflow() {
  if (!newWorkflow.value.name) return

  try {
    loading.value = true
    await workflowApi.create({
      name: newWorkflow.value.name,
      description: newWorkflow.value.description,
      nodes: [],
      connections: [],
    })
    showCreateModal.value = false
    newWorkflow.value = { name: '', description: '' }
    await loadWorkflows()
  } catch {
    // Handle error
  } finally {
    loading.value = false
  }
}

async function createFromTemplate(template: WorkflowTemplate) {
  try {
    loading.value = true
    await workflowApi.create({
      name: template.name,
      description: template.description,
      nodes: template.workflow.nodes || [],
      connections: template.workflow.connections || [],
      variables: template.workflow.variables,
      settings: template.workflow.settings,
      tags: template.tags,
    })
    showTemplatesModal.value = false
    await loadWorkflows()
  } catch {
    // Handle error
  } finally {
    loading.value = false
  }
}

async function toggleWorkflow(workflow: Workflow) {
  try {
    if (workflow.status === 'active') {
      await workflowApi.disable(workflow.id)
    } else {
      await workflowApi.enable(workflow.id)
    }
    await loadWorkflows()
  } catch {
    // Handle error
  }
}

async function deleteWorkflow(workflow: Workflow) {
  if (!confirm(t('workflow.confirmDelete', { name: workflow.name }))) return

  try {
    await workflowApi.delete(workflow.id)
    await loadWorkflows()
  } catch {
    // Handle error
  }
}

async function executeWorkflow(workflow: Workflow) {
  try {
    await workflowApi.execute(workflow.id)
    await loadExecutions(workflow)
  } catch {
    // Handle error
  }
}

async function loadExecutions(workflow: Workflow) {
  selectedWorkflow.value = workflow
  try {
    const response = await workflowApi.listExecutions(workflow.id, { limit: 20 })
    executions.value = response.data.executions || []
    showExecutionsModal.value = true
  } catch {
    executions.value = []
  }
}

function getStatusColor(status: string): string {
  switch (status) {
    case 'active':
      return 'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300'
    case 'draft':
      return 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
    case 'disabled':
      return 'bg-yellow-100 dark:bg-yellow-900/50 text-yellow-700 dark:text-yellow-300'
    case 'error':
      return 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300'
    default:
      return 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
  }
}

function getExecutionStatusColor(status: string): string {
  switch (status) {
    case 'completed':
      return 'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300'
    case 'running':
      return 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300'
    case 'pending':
      return 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
    case 'failed':
      return 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300'
    case 'cancelled':
      return 'bg-yellow-100 dark:bg-yellow-900/50 text-yellow-700 dark:text-yellow-300'
    default:
      return 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
  }
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleString()
}

// Node editor functions
function openNodeEditor(workflow: Workflow) {
  editingWorkflow.value = workflow
  editingNodes.value = [...workflow.nodes]
  editingConnections.value = [...workflow.connections]
  selectedNode.value = null
  showNodeEditorModal.value = true
}

function openAddNodeModal() {
  newNode.value = {
    type: 'trigger',
    name: '',
    config: {},
  }
  showAddNodeModal.value = true
}

function addNode() {
  if (!newNode.value.name) return

  // Generate a temporary ID for frontend use (backend will replace with UUID)
  const tempId = `temp_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`

  const node: WorkflowNode = {
    id: tempId,
    type: newNode.value.type,
    name: newNode.value.name,
    config: newNode.value.config,
    position: {
      x: 100 + editingNodes.value.length * 50,
      y: 100 + editingNodes.value.length * 50,
    },
  }

  editingNodes.value.push(node)
  showAddNodeModal.value = false
  newNode.value = { type: 'trigger', name: '', config: {} }
}

function selectNode(node: WorkflowNode) {
  selectedNode.value = node
}

function deleteNode(node: WorkflowNode) {
  const index = editingNodes.value.findIndex(n => n === node)
  if (index !== -1) {
    editingNodes.value.splice(index, 1)
    // Remove connections involving this node
    editingConnections.value = editingConnections.value.filter(
      c => c.source_node !== node.id && c.target_node !== node.id
    )
    if (selectedNode.value === node) {
      selectedNode.value = null
    }
  }
}

function addConnection(sourceNode: WorkflowNode, targetNode: WorkflowNode) {
  if (sourceNode.id === targetNode.id) return

  // Check if connection already exists
  const exists = editingConnections.value.some(
    c => c.source_node === sourceNode.id && c.target_node === targetNode.id
  )
  if (exists) return

  const connection: WorkflowConnection = {
    id: '', // Will be auto-generated by backend
    source_node: sourceNode.id,
    target_node: targetNode.id,
  }

  editingConnections.value.push(connection)
}

function deleteConnection(connection: WorkflowConnection) {
  const index = editingConnections.value.findIndex(c => c === connection)
  if (index !== -1) {
    editingConnections.value.splice(index, 1)
  }
}

async function saveNodes() {
  if (!editingWorkflow.value) return

  try {
    loading.value = true
    await workflowApi.update(editingWorkflow.value.id, {
      nodes: editingNodes.value,
      connections: editingConnections.value,
    })
    showNodeEditorModal.value = false
    await loadWorkflows()
  } catch {
    // Handle error
  } finally {
    loading.value = false
  }
}

function getNodeTypeInfo(type: NodeType) {
  return nodeTypes.find(nt => nt.value === type) || nodeTypes[0]
}

function getNodeTypeColor(type: NodeType): string {
  switch (type) {
    case 'trigger':
      return 'bg-purple-100 dark:bg-purple-900/50 text-purple-700 dark:text-purple-300 border-purple-300 dark:border-purple-700'
    case 'action':
      return 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 border-blue-300 dark:border-blue-700'
    case 'condition':
      return 'bg-yellow-100 dark:bg-yellow-900/50 text-yellow-700 dark:text-yellow-300 border-yellow-300 dark:border-yellow-700'
    case 'loop':
      return 'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300 border-green-300 dark:border-green-700'
    case 'delay':
      return 'bg-orange-100 dark:bg-orange-900/50 text-orange-700 dark:text-orange-300 border-orange-300 dark:border-orange-700'
    case 'switch':
      return 'bg-pink-100 dark:bg-pink-900/50 text-pink-700 dark:text-pink-300 border-pink-300 dark:border-pink-700'
    case 'merge':
      return 'bg-cyan-100 dark:bg-cyan-900/50 text-cyan-700 dark:text-cyan-300 border-cyan-300 dark:border-cyan-700'
    case 'subflow':
      return 'bg-indigo-100 dark:bg-indigo-900/50 text-indigo-700 dark:text-indigo-300 border-indigo-300 dark:border-indigo-700'
    default:
      return 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 border-gray-300 dark:border-gray-700'
  }
}
</script>

<template>
  <div class="workflow-view p-4 sm:p-6 max-w-6xl mx-auto">
    <div class="flex items-center justify-between mb-6">
      <h1 class="text-xl sm:text-2xl font-bold text-gray-900 dark:text-white">{{ t('workflow.title') }}</h1>
      <div class="flex gap-2">
        <button
          class="px-4 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-900 dark:text-white rounded-lg text-sm transition-colors"
          @click="loadTemplates"
        >
          {{ t('workflow.templates') }}
        </button>
        <button
          class="px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg text-sm transition-colors flex items-center gap-2"
          @click="showCreateModal = true"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
          </svg>
          {{ t('workflow.create') }}
        </button>
      </div>
    </div>

    <!-- Stats -->
    <div v-if="stats" class="grid grid-cols-2 sm:grid-cols-4 gap-4 mb-6">
      <div class="glass-card p-4">
        <div class="text-2xl font-bold text-gray-900 dark:text-white">{{ stats.total_workflows }}</div>
        <div class="text-sm text-gray-500 dark:text-slate-400">{{ t('workflow.totalWorkflows') }}</div>
      </div>
      <div class="glass-card p-4">
        <div class="text-2xl font-bold text-green-600 dark:text-green-400">{{ stats.active_workflows }}</div>
        <div class="text-sm text-gray-500 dark:text-slate-400">{{ t('workflow.activeWorkflows') }}</div>
      </div>
      <div class="glass-card p-4">
        <div class="text-2xl font-bold text-gray-900 dark:text-white dark:text-white">{{ stats.total_executions }}</div>
        <div class="text-sm text-gray-500 dark:text-slate-400">{{ t('workflow.totalExecutions') }}</div>
      </div>
      <div class="glass-card p-4">
        <div class="text-2xl font-bold text-gray-900 dark:text-white">
          {{ stats.total_executions > 0 ? Math.round((stats.successful_executions / stats.total_executions) * 100) : 0 }}%
        </div>
        <div class="text-sm text-gray-500 dark:text-slate-400">{{ t('workflow.successRate') }}</div>
      </div>
    </div>

    <!-- Filters -->
    <div class="flex flex-col sm:flex-row gap-4 mb-6">
      <div class="flex-1">
        <input
          v-model="searchQuery"
          type="text"
          :placeholder="t('workflow.searchPlaceholder')"
          class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-400 border border-gray-200 dark:border-slate-600"
        />
      </div>
      <select
        v-model="statusFilter"
        class="bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-400 border border-gray-200 dark:border-slate-600"
      >
        <option value="">{{ t('workflow.allStatuses') }}</option>
        <option value="active">{{ t('workflow.statusActive') }}</option>
        <option value="draft">{{ t('workflow.statusDraft') }}</option>
        <option value="disabled">{{ t('workflow.statusDisabled') }}</option>
      </select>
    </div>

    <!-- Workflows List -->
    <div v-if="loading" class="text-center py-8 text-gray-500 dark:text-slate-400">
      {{ t('common.loading') }}
    </div>

    <div v-else-if="filteredWorkflows.length === 0" class="text-center py-8">
      <svg xmlns="http://www.w3.org/2000/svg" class="h-12 w-12 mx-auto text-gray-400 dark:text-slate-500 mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
      </svg>
      <p class="text-gray-500 dark:text-slate-400">{{ t('workflow.noWorkflows') }}</p>
      <button
        class="mt-4 px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg text-sm transition-colors"
        @click="showCreateModal = true"
      >
        {{ t('workflow.createFirst') }}
      </button>
    </div>

    <div v-else class="space-y-4">
      <div
        v-for="workflow in filteredWorkflows"
        :key="workflow.id"
        class="glass-card p-4"
      >
        <div class="flex items-start justify-between">
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-3 mb-2">
              <h3 class="text-gray-900 dark:text-white font-medium truncate">{{ workflow.name }}</h3>
              <span
                :class="['px-2 py-0.5 rounded-full text-xs font-medium', getStatusColor(workflow.status)]"
              >
                {{ t(`workflow.status${workflow.status.charAt(0).toUpperCase() + workflow.status.slice(1)}`) }}
              </span>
            </div>
            <p v-if="workflow.description" class="text-sm text-gray-500 dark:text-slate-400 mb-2 line-clamp-2">
              {{ workflow.description }}
            </p>
            <div class="flex items-center gap-4 text-xs text-gray-400 dark:text-slate-500">
              <span>{{ t('workflow.nodes', { count: workflow.nodes.length }) }}</span>
              <span>{{ t('workflow.updated') }}: {{ formatDate(workflow.updated_at) }}</span>
            </div>
          </div>
          <div class="flex items-center gap-2 ml-4">
            <button
              :title="t('workflow.editNodes')"
              class="p-2 text-gray-900 dark:text-white dark:text-white hover:bg-gray-700 dark:bg-gray-500 dark:hover:bg-gray-600 rounded-lg transition-colors"
              @click="openNodeEditor(workflow)"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
              </svg>
            </button>
            <button
              :title="t('workflow.viewExecutions')"
              class="p-2 text-gray-500 dark:text-slate-400 hover:text-gray-900 dark:hover:text-white hover:bg-gray-100 dark:hover:bg-slate-700 rounded-lg transition-colors"
              @click="loadExecutions(workflow)"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
              </svg>
            </button>
            <button
              v-if="workflow.status === 'active'"
              :title="t('workflow.execute')"
              class="p-2 text-green-600 dark:text-green-400 hover:bg-green-50 dark:hover:bg-green-900/20 rounded-lg transition-colors"
              @click="executeWorkflow(workflow)"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </button>
            <button
              :title="workflow.status === 'active' ? t('workflow.disable') : t('workflow.enable')"
              :class="[
                'p-2 rounded-lg transition-colors',
                workflow.status === 'active'
                  ? 'text-yellow-600 dark:text-yellow-400 hover:bg-yellow-50 dark:hover:bg-yellow-900/20'
                  : 'text-green-600 dark:text-green-400 hover:bg-green-50 dark:hover:bg-green-900/20'
              ]"
              @click="toggleWorkflow(workflow)"
            >
              <svg v-if="workflow.status === 'active'" xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 9v6m4-6v6m7-3a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              <svg v-else xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </button>
            <button
              :title="t('common.delete')"
              class="p-2 text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg transition-colors"
              @click="deleteWorkflow(workflow)"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
              </svg>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Create Modal -->
    <div
      v-if="showCreateModal"
      class="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4"
      @click.self="showCreateModal = false"
    >
      <div class="bg-white dark:bg-slate-800 rounded-lg max-w-md w-full shadow-xl">
        <div class="p-4 sm:p-6">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">
            {{ t('workflow.createNew') }}
          </h3>

          <form class="space-y-4" @submit.prevent="createWorkflow">
            <div>
              <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">{{ t('workflow.name') }}</label>
              <input
                v-model="newWorkflow.name"
                type="text"
                required
                :placeholder="t('workflow.namePlaceholder')"
                class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-400 border border-gray-200 dark:border-slate-600"
              />
            </div>

            <div>
              <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">{{ t('workflow.description') }}</label>
              <textarea
                v-model="newWorkflow.description"
                rows="3"
                :placeholder="t('workflow.descriptionPlaceholder')"
                class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-400 border border-gray-200 dark:border-slate-600 resize-none"
              />
            </div>

            <div class="flex gap-3 pt-4">
              <button
                type="submit"
                :disabled="loading || !newWorkflow.name"
                class="flex-1 px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg transition-colors disabled:opacity-50"
              >
                {{ loading ? t('common.creating') : t('workflow.create') }}
              </button>
              <button
                type="button"
                class="px-4 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-900 dark:text-white rounded-lg transition-colors"
                @click="showCreateModal = false"
              >
                {{ t('common.cancel') }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>

    <!-- Templates Modal -->
    <div
      v-if="showTemplatesModal"
      class="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4"
      @click.self="showTemplatesModal = false"
    >
      <div class="bg-white dark:bg-slate-800 rounded-lg max-w-2xl w-full max-h-[80vh] overflow-hidden shadow-xl">
        <div class="p-4 sm:p-6 border-b border-gray-200 dark:border-slate-700">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('workflow.selectTemplate') }}
          </h3>
        </div>

        <div class="p-4 sm:p-6 overflow-y-auto max-h-[60vh]">
          <div v-if="templates.length === 0" class="text-center py-8 text-gray-500 dark:text-slate-400">
            {{ t('workflow.noTemplates') }}
          </div>

          <div v-else class="space-y-4">
            <div
              v-for="template in templates"
              :key="template.id"
              class="glass-card p-4 cursor-pointer hover:ring-2 hover:ring-accent transition-all"
              @click="createFromTemplate(template)"
            >
              <h4 class="text-gray-900 dark:text-white font-medium mb-1">{{ template.name }}</h4>
              <p class="text-sm text-gray-500 dark:text-slate-400 mb-2">{{ template.description }}</p>
              <div class="flex flex-wrap gap-2">
                <span
                  v-for="tag in template.tags"
                  :key="tag"
                  class="px-2 py-0.5 bg-gray-100 dark:bg-slate-700 text-gray-600 dark:text-gray-300 rounded text-xs"
                >
                  {{ tag }}
                </span>
              </div>
            </div>
          </div>
        </div>

        <div class="p-4 sm:p-6 border-t border-gray-200 dark:border-slate-700">
          <button
            class="w-full px-4 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-900 dark:text-white rounded-lg transition-colors"
            @click="showTemplatesModal = false"
          >
            {{ t('common.close') }}
          </button>
        </div>
      </div>
    </div>

    <!-- Executions Modal -->
    <div
      v-if="showExecutionsModal && selectedWorkflow"
      class="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4"
      @click.self="showExecutionsModal = false"
    >
      <div class="bg-white dark:bg-slate-800 rounded-lg max-w-2xl w-full max-h-[80vh] overflow-hidden shadow-xl">
        <div class="p-4 sm:p-6 border-b border-gray-200 dark:border-slate-700">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('workflow.executionsFor', { name: selectedWorkflow.name }) }}
          </h3>
        </div>

        <div class="p-4 sm:p-6 overflow-y-auto max-h-[60vh]">
          <div v-if="executions.length === 0" class="text-center py-8 text-gray-500 dark:text-slate-400">
            {{ t('workflow.noExecutions') }}
          </div>

          <div v-else class="space-y-3">
            <div
              v-for="execution in executions"
              :key="execution.id"
              class="glass-card p-3"
            >
              <div class="flex items-center justify-between mb-2">
                <span
                  :class="['px-2 py-0.5 rounded-full text-xs font-medium', getExecutionStatusColor(execution.status)]"
                >
                  {{ execution.status }}
                </span>
                <span class="text-xs text-gray-400 dark:text-slate-500">
                  {{ formatDate(execution.started_at) }}
                </span>
              </div>
              <div class="flex items-center gap-4 text-xs text-gray-500 dark:text-slate-400">
                <span v-if="execution.duration_ms">
                  {{ t('workflow.duration') }}: {{ execution.duration_ms }}ms
                </span>
                <span v-if="execution.error" class="text-red-500 dark:text-red-400 truncate">
                  {{ execution.error }}
                </span>
              </div>
            </div>
          </div>
        </div>

        <div class="p-4 sm:p-6 border-t border-gray-200 dark:border-slate-700">
          <button
            class="w-full px-4 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-900 dark:text-white rounded-lg transition-colors"
            @click="showExecutionsModal = false"
          >
            {{ t('common.close') }}
          </button>
        </div>
      </div>
    </div>

    <!-- Node Editor Modal -->
    <div
      v-if="showNodeEditorModal && editingWorkflow"
      class="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4"
      @click.self="showNodeEditorModal = false"
    >
      <div class="bg-white dark:bg-slate-800 rounded-lg max-w-4xl w-full max-h-[90vh] overflow-hidden shadow-xl">
        <div class="p-4 sm:p-6 border-b border-gray-200 dark:border-slate-700 flex items-center justify-between">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('workflow.editNodes') }}: {{ editingWorkflow.name }}
          </h3>
          <button
            class="px-3 py-1.5 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg text-sm transition-colors flex items-center gap-1"
            @click="openAddNodeModal"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
            </svg>
            {{ t('workflow.addNode') }}
          </button>
        </div>

        <div class="p-4 sm:p-6 overflow-y-auto max-h-[60vh]">
          <!-- Nodes List -->
          <div v-if="editingNodes.length === 0" class="text-center py-8 text-gray-500 dark:text-slate-400">
            {{ t('workflow.noNodes') }}
            <button
              class="mt-4 px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg text-sm transition-colors block mx-auto"
              @click="openAddNodeModal"
            >
              {{ t('workflow.addFirstNode') }}
            </button>
          </div>

          <div v-else class="space-y-4">
            <!-- Nodes -->
            <div class="mb-4">
              <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('workflow.nodesList') }}</h4>
              <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div
                  v-for="node in editingNodes"
                  :key="node.id || node.name"
                  :class="[
                    'p-3 rounded-lg border-2 cursor-pointer transition-all',
                    getNodeTypeColor(node.type),
                    selectedNode === node ? 'ring-2 ring-accent' : ''
                  ]"
                  @click="selectNode(node)"
                >
                  <div class="flex items-center justify-between">
                    <div class="flex items-center gap-2">
                      <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getNodeTypeInfo(node.type)?.icon ?? ''" />
                      </svg>
                      <span class="font-medium">{{ node.name }}</span>
                    </div>
                    <button
                      class="p-1 hover:bg-red-100 dark:hover:bg-red-900/30 rounded text-red-600 dark:text-red-400"
                      @click.stop="deleteNode(node)"
                    >
                      <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                      </svg>
                    </button>
                  </div>
                  <div class="text-xs mt-1 opacity-75">{{ tr(getNodeTypeInfo(node.type)?.labelKey, node.type) }}</div>
                </div>
              </div>
            </div>

            <!-- Connections -->
            <div v-if="editingNodes.length > 1">
              <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('workflow.connections') }}</h4>

              <!-- Existing connections -->
              <div v-if="editingConnections.length > 0" class="space-y-2 mb-3">
                <div
                  v-for="conn in editingConnections"
                  :key="conn.id || `${conn.source_node}-${conn.target_node}`"
                  class="flex items-center justify-between p-2 bg-gray-100 dark:bg-slate-700 rounded-lg text-sm"
                >
                  <span class="text-gray-700 dark:text-gray-300">
                    {{ editingNodes.find(n => n.id === conn.source_node)?.name || conn.source_node }}
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 inline mx-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 8l4 4m0 0l-4 4m4-4H3" />
                    </svg>
                    {{ editingNodes.find(n => n.id === conn.target_node)?.name || conn.target_node }}
                  </span>
                  <button
                    class="p-1 hover:bg-red-100 dark:hover:bg-red-900/30 rounded text-red-600 dark:text-red-400"
                    @click="deleteConnection(conn)"
                  >
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                    </svg>
                  </button>
                </div>
              </div>

              <!-- Add connection hint -->
              <p v-if="selectedNode" class="text-sm text-gray-500 dark:text-slate-400">
                {{ t('workflow.clickToConnect', { name: selectedNode.name }) }}
              </p>
              <div v-if="selectedNode" class="flex flex-wrap gap-2 mt-2">
                <button
                  v-for="node in editingNodes.filter(n => n !== selectedNode)"
                  :key="node.id || node.name"
                  class="px-3 py-1 text-sm bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-700 dark:text-gray-300 rounded-lg transition-colors"
                  @click="addConnection(selectedNode, node)"
                >
                  {{ node.name }}
                </button>
              </div>
            </div>
          </div>
        </div>

        <div class="p-4 sm:p-6 border-t border-gray-200 dark:border-slate-700 flex gap-3">
          <button
            :disabled="loading"
            class="flex-1 px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg transition-colors disabled:opacity-50"
            @click="saveNodes"
          >
            {{ loading ? t('common.saving') : t('common.save') }}
          </button>
          <button
            class="px-4 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-900 dark:text-white rounded-lg transition-colors"
            @click="showNodeEditorModal = false"
          >
            {{ t('common.cancel') }}
          </button>
        </div>
      </div>
    </div>

    <!-- Add Node Modal -->
    <div
      v-if="showAddNodeModal"
      class="fixed inset-0 bg-black/50 flex items-center justify-center z-[60] p-4"
      @click.self="showAddNodeModal = false"
    >
      <div class="bg-white dark:bg-slate-800 rounded-lg max-w-md w-full shadow-xl">
        <div class="p-4 sm:p-6">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">
            {{ t('workflow.addNode') }}
          </h3>

          <form class="space-y-4" @submit.prevent="addNode">
            <div>
              <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">{{ t('workflow.nodeType') }}</label>
              <div class="grid grid-cols-2 gap-2">
                <button
                  v-for="nt in nodeTypes"
                  :key="nt.value"
                  type="button"
                  :class="[
                    'p-2 rounded-lg border-2 text-left transition-all flex items-center gap-2',
                    newNode.type === nt.value
                      ? 'border-gray-900 dark:border-gray-700 bg-gray-100 dark:bg-gray-600/10'
                      : 'border-gray-200 dark:border-slate-600 hover:border-gray-300 dark:hover:border-slate-500'
                  ]"
                  @click="newNode.type = nt.value"
                >
                  <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="nt.icon" />
                  </svg>
                  <span class="text-sm text-gray-700 dark:text-gray-300">{{ tr(nt.labelKey, nt.value) }}</span>
                </button>
              </div>
            </div>

            <div>
              <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">{{ t('workflow.nodeName') }}</label>
              <input
                v-model="newNode.name"
                type="text"
                required
                :placeholder="t('workflow.nodeNamePlaceholder')"
                class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-400 border border-gray-200 dark:border-slate-600"
              />
            </div>

            <div class="flex gap-3 pt-4">
              <button
                type="submit"
                :disabled="!newNode.name"
                class="flex-1 px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg transition-colors disabled:opacity-50"
              >
                {{ t('workflow.addNode') }}
              </button>
              <button
                type="button"
                class="px-4 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-900 dark:text-white rounded-lg transition-colors"
                @click="showAddNodeModal = false"
              >
                {{ t('common.cancel') }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  </div>
</template>
