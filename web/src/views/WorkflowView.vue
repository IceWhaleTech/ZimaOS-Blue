<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { workflowApi } from '@/api/workflow'
import type { Workflow, WorkflowExecution, WorkflowStats, WorkflowTemplate } from '@/api/workflow'

const { t } = useI18n()

const loading = ref(false)
const workflows = ref<Workflow[]>([])
const stats = ref<WorkflowStats | null>(null)
const templates = ref<WorkflowTemplate[]>([])
const selectedWorkflow = ref<Workflow | null>(null)
const executions = ref<WorkflowExecution[]>([])
const showCreateModal = ref(false)
const showExecutionsModal = ref(false)
const showTemplatesModal = ref(false)

// Create form
const newWorkflow = ref({
  name: '',
  description: '',
})

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
    workflows.value = response.data.workflows
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
    executions.value = response.data.executions
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
          class="px-4 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg text-sm transition-colors flex items-center gap-2"
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
        <div class="text-2xl font-bold text-blue-600 dark:text-blue-400">{{ stats.total_executions }}</div>
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
          class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-accent border border-gray-200 dark:border-slate-600"
        />
      </div>
      <select
        v-model="statusFilter"
        class="bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-accent border border-gray-200 dark:border-slate-600"
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
        class="mt-4 px-4 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg text-sm transition-colors"
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

          <form @submit.prevent="createWorkflow" class="space-y-4">
            <div>
              <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">{{ t('workflow.name') }}</label>
              <input
                v-model="newWorkflow.name"
                type="text"
                required
                :placeholder="t('workflow.namePlaceholder')"
                class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-accent border border-gray-200 dark:border-slate-600"
              />
            </div>

            <div>
              <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">{{ t('workflow.description') }}</label>
              <textarea
                v-model="newWorkflow.description"
                rows="3"
                :placeholder="t('workflow.descriptionPlaceholder')"
                class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-accent border border-gray-200 dark:border-slate-600 resize-none"
              />
            </div>

            <div class="flex gap-3 pt-4">
              <button
                type="submit"
                :disabled="loading || !newWorkflow.name"
                class="flex-1 px-4 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg transition-colors disabled:opacity-50"
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
  </div>
</template>
