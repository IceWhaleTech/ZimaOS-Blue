<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useCompanionStore } from '@/stores/companion'
import AlertItem from '@/components/companion/AlertItem.vue'
import type { AlertSeverity } from '@/api/companion'

const { t } = useI18n()
const router = useRouter()
const companionStore = useCompanionStore()

// Filters
const severityFilter = ref<AlertSeverity | 'all'>('all')
const acknowledgedFilter = ref<'all' | 'unacked' | 'acked'>('all')
const searchQuery = ref('')
const dateFrom = ref('')
const dateTo = ref('')

// Selection for bulk actions
const selectedAlerts = ref<string[]>([])
const selectAll = ref(false)

// Computed
const alerts = computed(() => companionStore.alerts)
const loading = computed(() => companionStore.loadingAlerts)
const hasMore = computed(() => companionStore.hasMoreAlerts)
const totalAlerts = computed(() => companionStore.totalAlerts)

const filteredAlerts = computed(() => {
  let result = alerts.value

  // Search filter
  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    result = result.filter(
      (a) =>
        a.title.toLowerCase().includes(query) ||
        a.description?.toLowerCase().includes(query)
    )
  }

  return result
})

const severityOptions: { value: AlertSeverity | 'all'; label: string }[] = [
  { value: 'all', label: 'companion.alerts.allSeverities' },
  { value: 'critical', label: 'companion.alerts.critical' },
  { value: 'error', label: 'companion.alerts.error' },
  { value: 'warning', label: 'companion.alerts.warning' },
  { value: 'info', label: 'companion.alerts.info' },
]

const acknowledgedOptions = [
  { value: 'all', label: 'companion.alerts.allStatus' },
  { value: 'unacked', label: 'companion.alerts.unacknowledged' },
  { value: 'acked', label: 'companion.alerts.acknowledged' },
]

// Stats
const alertStats = computed(() => {
  const stats = {
    total: totalAlerts.value,
    critical: 0,
    error: 0,
    warning: 0,
    info: 0,
    unacked: 0,
  }

  alerts.value.forEach((a) => {
    stats[a.severity]++
    if (!a.acknowledged) stats.unacked++
  })

  return stats
})

// Actions
async function fetchAlerts() {
  const opts: Record<string, unknown> = {}

  if (severityFilter.value !== 'all') {
    opts.severity = severityFilter.value
  }

  if (acknowledgedFilter.value === 'unacked') {
    opts.acknowledged = false
  } else if (acknowledgedFilter.value === 'acked') {
    opts.acknowledged = true
  }

  if (dateFrom.value) {
    opts.from = new Date(dateFrom.value).toISOString()
  }

  if (dateTo.value) {
    opts.to = new Date(dateTo.value).toISOString()
  }

  await companionStore.fetchAlerts(opts as Parameters<typeof companionStore.fetchAlerts>[0])
}

async function loadMore() {
  if (loading.value || !hasMore.value) return
  await companionStore.loadMoreAlerts()
}

async function acknowledgeAlert(id: string) {
  await companionStore.acknowledgeAlert(id)
}

async function bulkAcknowledge() {
  if (selectedAlerts.value.length === 0) return

  for (const id of selectedAlerts.value) {
    await companionStore.acknowledgeAlert(id)
  }

  selectedAlerts.value = []
  selectAll.value = false
}

function toggleSelectAll() {
  if (selectAll.value) {
    selectedAlerts.value = filteredAlerts.value
      .filter((a) => !a.acknowledged)
      .map((a) => a.id)
  } else {
    selectedAlerts.value = []
  }
}

function toggleAlertSelection(id: string) {
  const index = selectedAlerts.value.indexOf(id)
  if (index === -1) {
    selectedAlerts.value.push(id)
  } else {
    selectedAlerts.value.splice(index, 1)
  }
}

function viewSession(sessionId: string) {
  router.push(`/companion/replay/${sessionId}`)
}

function goBack() {
  router.push('/companion')
}

// Lifecycle
onMounted(() => {
  fetchAlerts()
})

// Watch filters
watch([severityFilter, acknowledgedFilter, dateFrom, dateTo], () => {
  fetchAlerts()
})
</script>

<template>
  <div class="min-h-screen bg-gray-50 dark:bg-gray-900">
    <!-- Header -->
    <div class="bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 px-6 py-4">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-4">
          <button
            class="p-2 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
            @click="goBack"
          >
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
            </svg>
          </button>
          <div>
            <h1 class="text-xl font-semibold text-gray-900 dark:text-white">
              {{ t('companion.alerts.center') }}
            </h1>
            <p class="text-sm text-gray-500 dark:text-gray-400">
              {{ t('companion.alerts.centerDesc') }}
            </p>
          </div>
        </div>

        <!-- Bulk actions -->
        <div v-if="selectedAlerts.length > 0" class="flex items-center gap-3">
          <span class="text-sm text-gray-600 dark:text-gray-400">
            {{ t('companion.alerts.selected', { count: selectedAlerts.length }) }}
          </span>
          <button
            class="px-4 py-2 text-sm bg-accent text-white rounded-lg hover:bg-accent/90 transition-colors"
            @click="bulkAcknowledge"
          >
            {{ t('companion.alerts.bulkAcknowledge') }}
          </button>
        </div>
      </div>
    </div>

    <div class="p-6">
      <!-- Stats cards -->
      <div class="grid grid-cols-2 md:grid-cols-5 gap-4 mb-6">
        <div class="bg-white dark:bg-gray-800 rounded-lg p-4 shadow-sm">
          <div class="text-2xl font-bold text-gray-900 dark:text-white">{{ alertStats.total }}</div>
          <div class="text-sm text-gray-500 dark:text-gray-400">{{ t('companion.alerts.total') }}</div>
        </div>
        <div class="bg-red-50 dark:bg-red-900/30 rounded-lg p-4 shadow-sm">
          <div class="text-2xl font-bold text-red-600 dark:text-red-400">{{ alertStats.critical }}</div>
          <div class="text-sm text-red-600/70 dark:text-red-400/70">{{ t('companion.alerts.critical') }}</div>
        </div>
        <div class="bg-orange-50 dark:bg-orange-900/30 rounded-lg p-4 shadow-sm">
          <div class="text-2xl font-bold text-orange-600 dark:text-orange-400">{{ alertStats.error }}</div>
          <div class="text-sm text-orange-600/70 dark:text-orange-400/70">{{ t('companion.alerts.error') }}</div>
        </div>
        <div class="bg-yellow-50 dark:bg-yellow-900/30 rounded-lg p-4 shadow-sm">
          <div class="text-2xl font-bold text-yellow-600 dark:text-yellow-400">{{ alertStats.warning }}</div>
          <div class="text-sm text-yellow-600/70 dark:text-yellow-400/70">{{ t('companion.alerts.warning') }}</div>
        </div>
        <div class="bg-blue-50 dark:bg-blue-900/30 rounded-lg p-4 shadow-sm">
          <div class="text-2xl font-bold text-blue-600 dark:text-blue-400">{{ alertStats.unacked }}</div>
          <div class="text-sm text-blue-600/70 dark:text-blue-400/70">{{ t('companion.alerts.unacknowledged') }}</div>
        </div>
      </div>

      <!-- Filters -->
      <div class="bg-white dark:bg-gray-800 rounded-lg shadow-sm p-4 mb-6">
        <div class="flex flex-wrap items-center gap-4">
          <!-- Search -->
          <div class="flex-1 min-w-[200px]">
            <div class="relative">
              <svg class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
              </svg>
              <input
                v-model="searchQuery"
                type="text"
                :placeholder="t('common.search')"
                class="w-full pl-10 pr-4 py-2 bg-gray-50 dark:bg-gray-700 border border-gray-200 dark:border-gray-600 rounded-lg text-sm text-gray-900 dark:text-white placeholder-gray-500 focus:ring-2 focus:ring-accent focus:border-transparent"
              />
            </div>
          </div>

          <!-- Severity filter -->
          <select
            v-model="severityFilter"
            class="px-3 py-2 bg-gray-50 dark:bg-gray-700 border border-gray-200 dark:border-gray-600 rounded-lg text-sm text-gray-900 dark:text-white focus:ring-2 focus:ring-accent focus:border-transparent"
          >
            <option v-for="opt in severityOptions" :key="opt.value" :value="opt.value">
              {{ t(opt.label) }}
            </option>
          </select>

          <!-- Acknowledged filter -->
          <select
            v-model="acknowledgedFilter"
            class="px-3 py-2 bg-gray-50 dark:bg-gray-700 border border-gray-200 dark:border-gray-600 rounded-lg text-sm text-gray-900 dark:text-white focus:ring-2 focus:ring-accent focus:border-transparent"
          >
            <option v-for="opt in acknowledgedOptions" :key="opt.value" :value="opt.value">
              {{ t(opt.label) }}
            </option>
          </select>

          <!-- Date range -->
          <div class="flex items-center gap-2">
            <input
              v-model="dateFrom"
              type="date"
              class="px-3 py-2 bg-gray-50 dark:bg-gray-700 border border-gray-200 dark:border-gray-600 rounded-lg text-sm text-gray-900 dark:text-white focus:ring-2 focus:ring-accent focus:border-transparent"
            />
            <span class="text-gray-500">-</span>
            <input
              v-model="dateTo"
              type="date"
              class="px-3 py-2 bg-gray-50 dark:bg-gray-700 border border-gray-200 dark:border-gray-600 rounded-lg text-sm text-gray-900 dark:text-white focus:ring-2 focus:ring-accent focus:border-transparent"
            />
          </div>
        </div>
      </div>

      <!-- Select all checkbox -->
      <div class="flex items-center gap-3 mb-4">
        <label class="flex items-center gap-2 cursor-pointer">
          <input
            v-model="selectAll"
            type="checkbox"
            class="w-4 h-4 text-accent rounded border-gray-300 dark:border-gray-600 focus:ring-accent"
            @change="toggleSelectAll"
          />
          <span class="text-sm text-gray-600 dark:text-gray-400">
            {{ t('companion.alerts.selectAllUnacked') }}
          </span>
        </label>
      </div>

      <!-- Alert list -->
      <div class="space-y-4">
        <div v-if="loading && alerts.length === 0" class="flex items-center justify-center py-12">
          <svg class="animate-spin w-8 h-8 text-accent" fill="none" viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
          </svg>
        </div>

        <div v-else-if="filteredAlerts.length === 0" class="text-center py-12">
          <svg class="w-12 h-12 text-gray-400 mx-auto mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <p class="text-gray-500 dark:text-gray-400">{{ t('companion.alerts.noAlerts') }}</p>
        </div>

        <template v-else>
          <div
            v-for="alert in filteredAlerts"
            :key="alert.id"
            class="flex items-start gap-3"
          >
            <!-- Checkbox -->
            <div v-if="!alert.acknowledged" class="pt-4">
              <input
                type="checkbox"
                :checked="selectedAlerts.includes(alert.id)"
                class="w-4 h-4 text-accent rounded border-gray-300 dark:border-gray-600 focus:ring-accent"
                @change="toggleAlertSelection(alert.id)"
              />
            </div>
            <div v-else class="w-4" />

            <!-- Alert item -->
            <div class="flex-1">
              <AlertItem
                :alert="alert"
                @acknowledge="acknowledgeAlert"
                @view-session="viewSession"
              />
            </div>
          </div>

          <!-- Load more -->
          <div v-if="hasMore" class="flex justify-center pt-4">
            <button
              class="px-6 py-2 text-sm text-accent hover:bg-accent/10 rounded-lg transition-colors"
              :disabled="loading"
              @click="loadMore"
            >
              <span v-if="loading">{{ t('common.loading') }}</span>
              <span v-else>{{ t('companion.loadMore') }}</span>
            </button>
          </div>
        </template>
      </div>
    </div>
  </div>
</template>
