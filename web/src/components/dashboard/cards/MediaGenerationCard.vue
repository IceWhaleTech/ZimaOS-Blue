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

const hasData = computed(() => stats.value && stats.value.total_tasks > 0)
const imageCount = computed(() => stats.value?.tasks_by_type?.image ?? 0)
const videoCount = computed(() => stats.value?.tasks_by_type?.video ?? 0)
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

// Provider breakdown
const providers = computed(() => {
  if (!stats.value?.tasks_by_provider) return []
  return Object.entries(stats.value.tasks_by_provider)
    .sort((a, b) => b[1] - a[1])
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
  <div class="space-y-4">
    <!-- Loading -->
    <div v-if="loading" class="flex items-center justify-center py-8">
      <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-pink-500"></div>
    </div>

    <!-- No data -->
    <div v-else-if="!hasData" class="text-center py-8 text-gray-400 dark:text-gray-500">
      {{ t('mediaStats.noData') }}
    </div>

    <template v-else>
      <!-- Summary row: 4 mini cards -->
      <div class="grid grid-cols-2 lg:grid-cols-4 gap-3">
        <!-- Total Cost -->
        <div class="bg-white dark:bg-gray-700 rounded-lg p-3 shadow">
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('mediaStats.totalCost') }}</p>
          <p class="text-xl font-bold text-gray-900 dark:text-white mt-1">
            {{ formatCost(stats!.total_cost_usd) }}
          </p>
        </div>
        <!-- Succeeded -->
        <div class="bg-white dark:bg-gray-700 rounded-lg p-3 shadow">
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('mediaStats.succeeded') }}</p>
          <p class="text-xl font-bold text-green-600 dark:text-green-400 mt-1">
            {{ stats!.succeeded }}
          </p>
        </div>
        <!-- Images -->
        <div class="bg-white dark:bg-gray-700 rounded-lg p-3 shadow">
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('mediaStats.images') }}</p>
          <p class="text-xl font-bold text-blue-600 dark:text-blue-400 mt-1">
            {{ imageCount }}
          </p>
        </div>
        <!-- Videos -->
        <div class="bg-white dark:bg-gray-700 rounded-lg p-3 shadow">
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('mediaStats.videos') }}</p>
          <p class="text-xl font-bold text-purple-600 dark:text-purple-400 mt-1">
            {{ videoCount }}
          </p>
        </div>
      </div>

      <!-- Success rate + failed -->
      <div class="flex items-center gap-4 text-sm text-gray-500 dark:text-gray-400">
        <span>{{ t('mediaStats.successRate') }}: {{ successRate }}%</span>
        <span v-if="stats!.failed > 0" class="text-red-500">
          {{ stats!.failed }} {{ t('mediaStats.failed') }}
        </span>
      </div>

      <!-- Cost by Model table -->
      <div v-if="topModels.length > 0">
        <p class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
          {{ t('mediaStats.costByModel') }}
        </p>
        <div class="space-y-1.5">
          <div
            v-for="[model, cost] in topModels"
            :key="model"
            class="flex items-center justify-between text-sm"
          >
            <span class="text-gray-600 dark:text-gray-400 truncate mr-2">{{ model }}</span>
            <span class="font-mono text-gray-900 dark:text-white whitespace-nowrap">
              {{ formatCost(cost) }}
            </span>
          </div>
        </div>
      </div>

      <!-- Provider breakdown -->
      <div v-if="providers.length > 0">
        <p class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
          {{ t('mediaStats.byProvider') }}
        </p>
        <div class="flex flex-wrap gap-2">
          <span
            v-for="[prov, count] in providers"
            :key="prov"
            class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-pink-100 dark:bg-pink-900/30 text-pink-700 dark:text-pink-300"
          >
            {{ prov }}: {{ count }}
          </span>
        </div>
      </div>
    </template>
  </div>
</template>