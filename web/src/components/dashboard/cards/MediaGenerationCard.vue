<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import api from '@/api/client'

interface MediaStats {
  total_tasks: number
  succeeded: number
  failed: number
  total_cost_usd: number
  cost_by_model?: Record<string, number>
  tasks_by_type?: Record<string, number>
  tasks_by_category?: Record<string, number>
  tasks_by_provider?: Record<string, number>
  cost_by_provider?: Record<string, number>
}

const { t } = useI18n()
const stats = ref<MediaStats | null>(null)
const loading = ref(false)

const hasData = computed(() => (stats.value?.total_tasks ?? 0) > 0)
const totalTasks = computed(() => stats.value?.total_tasks ?? 0)
const imageCount = computed(() => stats.value?.tasks_by_type?.image ?? 0)
const videoCount = computed(() => stats.value?.tasks_by_type?.video ?? 0)
const failedCount = computed(() => stats.value?.failed ?? 0)
const successRate = computed(() => {
  if (!stats.value || stats.value.total_tasks === 0) return 0
  return Math.round((stats.value.succeeded / stats.value.total_tasks) * 100)
})

// Top models by cost (sorted desc, max 5)
const topModels = computed(() => {
  if (!stats.value?.cost_by_model) return []
  return Object.entries(stats.value.cost_by_model)
    .sort((a, b) => b[1] - a[1])
    .slice(0, 5)
})

const topProviderRows = computed(() => {
  if (!stats.value?.tasks_by_provider) return []
  const byProvider = stats.value.tasks_by_provider
  const costByProvider = stats.value.cost_by_provider ?? {}
  return Object.entries(byProvider)
    .map(([provider, tasks]) => ({
      provider,
      tasks,
      cost: costByProvider[provider] ?? 0,
    }))
    .sort((a, b) => b.tasks - a.tasks)
    .slice(0, 5)
})

const topCategories = computed(() => {
  if (!stats.value?.tasks_by_category) return []
  return Object.entries(stats.value.tasks_by_category)
    .sort((a, b) => b[1] - a[1])
    .slice(0, 4)
})

function formatCost(n: number): string {
  if (n >= 100) return '$' + Math.round(n)
  if (n >= 0.01) return '$' + n.toFixed(2)
  return '$0.00'
}

async function fetchStats() {
  loading.value = true
  try {
    const res = await api.get<MediaStats>('/media/stats')
    if (res?.data) stats.value = res.data
  } catch {
    // ignore
  } finally {
    loading.value = false
  }
}

onMounted(fetchStats)
defineExpose({ refresh: fetchStats })
</script>

<template>
  <div class="space-y-4 rounded-xl border border-gray-200 dark:border-gray-600 bg-white dark:bg-gray-700 p-4">
    <!-- Loading -->
    <div v-if="loading" class="flex items-center justify-center py-8">
      <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-pink-500"></div>
    </div>

    <template v-else>
      <div v-if="!hasData" class="py-8 text-center text-sm text-gray-500 dark:text-gray-400">
        {{ t('mediaStats.noData') }}
      </div>

      <div v-else class="space-y-4">
        <div class="grid grid-cols-2 lg:grid-cols-5 gap-3">
          <div class="rounded-lg p-3 border border-gray-200 dark:border-gray-600 bg-gray-50 dark:bg-gray-800/40">
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('common.total') }}</p>
            <p class="text-xl font-bold text-gray-900 dark:text-white mt-1">
              {{ totalTasks }}
            </p>
          </div>
          <div class="rounded-lg p-3 border border-gray-200 dark:border-gray-600 bg-green-50 dark:bg-green-900/20">
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('mediaStats.successRate') }}</p>
            <p class="text-xl font-bold text-green-600 dark:text-green-400 mt-1">
              {{ successRate }}%
            </p>
          </div>
          <div class="rounded-lg p-3 border border-gray-200 dark:border-gray-600 bg-gray-50 dark:bg-gray-800/40">
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('mediaStats.totalCost') }}</p>
            <p class="text-xl font-bold text-gray-900 dark:text-white mt-1">
              {{ formatCost(stats!.total_cost_usd) }}
            </p>
          </div>
          <div class="rounded-lg p-3 border border-gray-200 dark:border-gray-600 bg-blue-50 dark:bg-blue-900/20">
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('mediaStats.images') }}</p>
            <p class="text-xl font-bold text-blue-600 dark:text-blue-400 mt-1">
              {{ imageCount }}
            </p>
          </div>
          <div class="rounded-lg p-3 border border-gray-200 dark:border-gray-600 bg-violet-50 dark:bg-violet-900/20">
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('mediaStats.videos') }}</p>
            <p class="text-xl font-bold text-violet-600 dark:text-violet-400 mt-1">
              {{ videoCount }}
            </p>
          </div>
        </div>

        <div class="rounded-lg p-3 border border-gray-200 dark:border-gray-600">
          <div class="flex items-center justify-between text-sm mb-2">
            <span class="text-gray-700 dark:text-gray-300">{{ t('mediaStats.successRate') }}</span>
            <span class="font-medium text-gray-900 dark:text-white">{{ successRate }}%</span>
          </div>
          <div class="h-2 rounded-full bg-gray-100 dark:bg-gray-600 overflow-hidden">
            <div class="h-full bg-green-500 dark:bg-green-400 transition-all" :style="{ width: `${successRate}%` }"></div>
          </div>
          <div class="mt-2 flex items-center justify-between text-xs">
            <span class="text-green-600 dark:text-green-400">{{ stats?.succeeded ?? 0 }} {{ t('mediaStats.success', 'success') }}</span>
            <span v-if="failedCount > 0" class="text-red-500">{{ failedCount }} {{ t('mediaStats.failed') }}</span>
          </div>
        </div>

        <div class="grid grid-cols-1 lg:grid-cols-2 gap-3">
          <div v-if="topModels.length > 0" class="rounded-lg p-3 border border-gray-200 dark:border-gray-600">
            <p class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              {{ t('mediaStats.costByModel') }}
            </p>
            <div class="space-y-2">
              <div v-for="[model, cost] in topModels" :key="model" class="space-y-1">
                <div class="flex items-center justify-between text-sm gap-3">
                  <span class="text-gray-600 dark:text-gray-400 truncate">{{ model }}</span>
                  <span class="font-mono text-gray-900 dark:text-white whitespace-nowrap">
                    {{ formatCost(cost) }}
                  </span>
                </div>
                <div class="h-1.5 rounded-full bg-gray-100 dark:bg-gray-600 overflow-hidden">
                  <div
                    class="h-full bg-pink-500 dark:bg-pink-400"
                    :style="{
                      width: `${stats?.total_cost_usd ? Math.max((cost / stats.total_cost_usd) * 100, 4) : 0}%`,
                    }"
                  ></div>
                </div>
              </div>
            </div>
          </div>

          <div v-if="topProviderRows.length > 0" class="rounded-lg p-3 border border-gray-200 dark:border-gray-600">
            <p class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              {{ t('mediaStats.byProvider') }}
            </p>
            <div class="space-y-2">
              <div
                v-for="row in topProviderRows"
                :key="row.provider"
                class="flex items-center justify-between text-sm"
              >
                <span class="text-gray-600 dark:text-gray-400 truncate mr-2">{{ row.provider }}</span>
                <span class="text-gray-900 dark:text-white">
                  {{ row.tasks }} · {{ formatCost(row.cost) }}
                </span>
              </div>
            </div>
          </div>
        </div>

        <div v-if="topCategories.length > 0" class="flex flex-wrap gap-2">
          <span
            v-for="[category, count] in topCategories"
            :key="category"
            class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 dark:bg-gray-600 text-gray-700 dark:text-gray-200"
          >
            {{ category }}: {{ count }}
          </span>
        </div>
      </div>
    </template>
  </div>
</template>
