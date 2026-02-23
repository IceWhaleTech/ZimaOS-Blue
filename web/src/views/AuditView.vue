<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { auditApi } from '@/api/audit'
import type { AuditEntry, AuditStats, AuditQueryParams } from '@/api/audit'

const { t } = useI18n()

const loading = ref(false)
const entries = ref<AuditEntry[]>([])
const stats = ref<AuditStats | null>(null)
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)

// Filters
const filters = ref<AuditQueryParams>({
  action: undefined,
  status: undefined,
  start_time: undefined,
  end_time: undefined,
})

const totalPages = computed(() => Math.ceil(total.value / pageSize.value))

onMounted(async () => {
  await Promise.all([loadEntries(), loadStats()])
})

async function loadEntries() {
  try {
    loading.value = true
    const params: AuditQueryParams = {
      ...filters.value,
      page: page.value,
      page_size: pageSize.value,
      sort_by: 'timestamp',
      sort_dir: 'desc',
    }
    const response = await auditApi.list(params)
    entries.value = response.data.entries
    total.value = response.data.total
  } catch {
    entries.value = []
  } finally {
    loading.value = false
  }
}

async function loadStats() {
  try {
    const response = await auditApi.getStats()
    stats.value = response.data
  } catch {
    stats.value = null
  }
}

function applyFilters() {
  page.value = 1
  loadEntries()
}

function clearFilters() {
  filters.value = { action: undefined, status: undefined, start_time: undefined, end_time: undefined }
  page.value = 1
  loadEntries()
}

function changePage(newPage: number) {
  if (newPage < 1 || newPage > totalPages.value) return
  page.value = newPage
  loadEntries()
}

function exportLogs(format: 'json' | 'csv') {
  auditApi.export({ ...filters.value, format })
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleString()
}

function getStatusColor(status: string): string {
  return status === 'success'
    ? 'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300'
    : 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300'
}

function getActionColor(action: string): string {
  if (action.includes('login')) return 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300'
  if (action.includes('create')) return 'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300'
  if (action.includes('delete')) return 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300'
  if (action.includes('update')) return 'bg-yellow-100 dark:bg-yellow-900/50 text-yellow-700 dark:text-yellow-300'
  return 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
}
</script>

<template>
  <div class="audit-view p-4 sm:p-6 max-w-6xl mx-auto">
    <div class="flex items-center justify-between mb-6">
      <h1 class="text-xl sm:text-2xl font-bold text-gray-900 dark:text-white">{{ t('audit.title') }}</h1>
      <div class="flex gap-2">
        <button
          class="px-3 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-900 dark:text-white rounded-lg text-sm transition-colors"
          @click="exportLogs('csv')"
        >
          {{ t('audit.exportCSV') }}
        </button>
        <button
          class="px-3 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-900 dark:text-white rounded-lg text-sm transition-colors"
          @click="exportLogs('json')"
        >
          {{ t('audit.exportJSON') }}
        </button>
      </div>
    </div>

    <!-- Stats -->
    <div v-if="stats" class="grid grid-cols-2 sm:grid-cols-4 gap-4 mb-6">
      <div class="glass-card p-4">
        <div class="text-2xl font-bold text-green-600 dark:text-green-400">{{ stats.by_status.success }}</div>
        <div class="text-sm text-gray-500 dark:text-slate-400">{{ t('audit.successfulActions') }}</div>
      </div>
      <div class="glass-card p-4">
        <div class="text-2xl font-bold text-red-600 dark:text-red-400">{{ stats.by_status.failure }}</div>
        <div class="text-sm text-gray-500 dark:text-slate-400">{{ t('audit.failedActions') }}</div>
      </div>
      <div class="glass-card p-4">
        <div class="text-2xl font-bold text-gray-900 dark:text-white dark:text-white">{{ stats.by_action.login || 0 }}</div>
        <div class="text-sm text-gray-500 dark:text-slate-400">{{ t('audit.logins') }}</div>
      </div>
      <div class="glass-card p-4">
        <div class="text-2xl font-bold text-gray-900 dark:text-white">{{ total }}</div>
        <div class="text-sm text-gray-500 dark:text-slate-400">{{ t('audit.totalEntries') }}</div>
      </div>
    </div>

    <!-- Filters -->
    <div class="glass-card p-4 mb-6">
      <div class="grid grid-cols-1 sm:grid-cols-4 gap-4">
        <select
          v-model="filters.action"
          class="bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 border border-gray-200 dark:border-slate-600"
        >
          <option :value="undefined">{{ t('audit.allActions') }}</option>
          <option value="login">Login</option>
          <option value="logout">Logout</option>
          <option value="user_create">User Create</option>
          <option value="user_update">User Update</option>
          <option value="user_delete">User Delete</option>
          <option value="mfa_setup">MFA Setup</option>
          <option value="mfa_disable">MFA Disable</option>
        </select>
        <select
          v-model="filters.status"
          class="bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 border border-gray-200 dark:border-slate-600"
        >
          <option :value="undefined">{{ t('audit.allStatuses') }}</option>
          <option value="success">{{ t('audit.success') }}</option>
          <option value="failure">{{ t('audit.failure') }}</option>
        </select>
        <div class="flex gap-2">
          <button
            class="flex-1 px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg text-sm transition-colors"
            @click="applyFilters"
          >
            {{ t('audit.filter') }}
          </button>
          <button
            class="px-4 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-900 dark:text-white rounded-lg text-sm transition-colors"
            @click="clearFilters"
          >
            {{ t('audit.clear') }}
          </button>
        </div>
      </div>
    </div>

    <!-- Entries List -->
    <div v-if="loading" class="text-center py-8 text-gray-500 dark:text-slate-400">
      {{ t('common.loading') }}
    </div>

    <div v-else-if="entries.length === 0" class="text-center py-8 text-gray-500 dark:text-slate-400">
      {{ t('audit.noEntries') }}
    </div>

    <div v-else class="space-y-3">
      <div v-for="entry in entries" :key="entry.id" class="glass-card p-4">
        <div class="flex items-start justify-between mb-2">
          <div class="flex items-center gap-2 flex-wrap">
            <span :class="['px-2 py-0.5 rounded-full text-xs font-medium', getActionColor(entry.action)]">
              {{ entry.action }}
            </span>
            <span :class="['px-2 py-0.5 rounded-full text-xs font-medium', getStatusColor(entry.status)]">
              {{ entry.status }}
            </span>
          </div>
          <span class="text-xs text-gray-400 dark:text-slate-500">{{ formatDate(entry.timestamp) }}</span>
        </div>
        <div class="text-sm text-gray-900 dark:text-white mb-1">
          {{ entry.username || t('audit.anonymous') }}
        </div>
        <div class="flex flex-wrap gap-4 text-xs text-gray-500 dark:text-slate-400">
          <span>IP: {{ entry.ip_address }}</span>
          <span v-if="entry.resource_type">{{ entry.resource_type }}: {{ entry.resource_id }}</span>
        </div>
      </div>
    </div>

    <!-- Pagination -->
    <div v-if="totalPages > 1" class="flex items-center justify-center gap-2 mt-6">
      <button
        :disabled="page === 1"
        class="px-3 py-1 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-900 dark:text-white rounded text-sm disabled:opacity-50"
        @click="changePage(page - 1)"
      >
        {{ t('common.previous') }}
      </button>
      <span class="text-sm text-gray-500 dark:text-slate-400">
        {{ page }} / {{ totalPages }}
      </span>
      <button
        :disabled="page === totalPages"
        class="px-3 py-1 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-900 dark:text-white rounded text-sm disabled:opacity-50"
        @click="changePage(page + 1)"
      >
        {{ t('common.next') }}
      </button>
    </div>
  </div>
</template>
