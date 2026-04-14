<script setup lang="ts">
import { ref, computed } from 'vue'
import { useMetricsStore } from '@/stores/metrics'
import { useI18n } from 'vue-i18n'
import type { ModelStats } from '@/api/metrics'

const { t } = useI18n()
const metricsStore = useMetricsStore()

type SortKey =
  | 'model'
  | 'calls'
  | 'success_rate'
  | 'total_tokens'
  | 'estimated_cost'
  | 'avg_latency_ms'

const sortKey = ref<SortKey>('calls')
const sortAsc = ref(false)

function toggleSort(key: SortKey) {
  if (sortKey.value === key) {
    sortAsc.value = !sortAsc.value
  } else {
    sortKey.value = key
    sortAsc.value = key === 'model' // default asc for model name, desc for numbers
  }
}

const sortedModels = computed(() => {
  const models = metricsStore.modelStats?.models
  if (!models?.length) return []
  return [...models].sort((a: ModelStats, b: ModelStats) => {
    const k = sortKey.value
    let cmp: number
    if (k === 'model') {
      cmp = a.model.localeCompare(b.model)
    } else {
      cmp = (a[k] ?? 0) - (b[k] ?? 0)
    }
    return sortAsc.value ? cmp : -cmp
  })
})

function sortIcon(key: SortKey): string {
  if (sortKey.value !== key) return '↕'
  return sortAsc.value ? '↑' : '↓'
}
</script>

<template>
  <div class="dashboard-card-surface p-4">
    <div class="dashboard-card-stack">
      <div class="dashboard-card-footer">
        <div class="dashboard-card-copy">
          <p class="dashboard-card-label">
            {{ t('metrics.modelStats') }}
          </p>
          <p class="dashboard-card-subtitle mt-2">
            {{ t('metrics.cards.modelStats.subtitle', { count: sortedModels.length }) }}
          </p>
        </div>
      </div>

      <div
        v-if="metricsStore.modelStats?.models?.length"
        class="dashboard-card-table overflow-x-auto"
      >
        <table>
          <thead>
            <tr>
              <th
                class="text-start cursor-pointer select-none"
                @click="toggleSort('model')"
              >
                {{ t('metrics.model') }}
                <span class="text-xs opacity-60">{{ sortIcon('model') }}</span>
              </th>
              <th
                class="text-end cursor-pointer select-none"
                @click="toggleSort('calls')"
              >
                {{ t('metrics.calls') }}
                <span class="text-xs opacity-60">{{ sortIcon('calls') }}</span>
              </th>
              <th
                class="text-end cursor-pointer select-none"
                @click="toggleSort('success_rate')"
              >
                {{ t('metrics.successRate') }}
                <span class="text-xs opacity-60">{{ sortIcon('success_rate') }}</span>
              </th>
              <th
                class="text-end cursor-pointer select-none"
                @click="toggleSort('total_tokens')"
              >
                {{ t('metrics.tokens') }}
                <span class="text-xs opacity-60">{{ sortIcon('total_tokens') }}</span>
              </th>
              <th
                class="text-end cursor-pointer select-none"
                @click="toggleSort('estimated_cost')"
              >
                {{ t('metrics.cost') }}
                <span class="text-xs opacity-60">{{ sortIcon('estimated_cost') }}</span>
              </th>
              <th
                class="text-end cursor-pointer select-none"
                @click="toggleSort('avg_latency_ms')"
              >
                {{ t('metrics.avgLatency') }}
                <span class="text-xs opacity-60">{{ sortIcon('avg_latency_ms') }}</span>
              </th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="model in sortedModels"
              :key="model.model"
            >
              <td class="font-medium text-gray-900 dark:text-white">
                {{ model.model }}
              </td>
              <td class="text-end">
                {{ model.calls ?? 0 }}
              </td>
              <td class="text-end">
                <span
                  :class="
                    (model.success_rate ?? 0) >= 95
                      ? 'text-green-600 dark:text-green-400'
                      : 'text-orange-600 dark:text-orange-400'
                  "
                >
                  {{ (model.success_rate ?? 0).toFixed(1) }}%
                </span>
              </td>
              <td class="text-end">
                {{ (model.total_tokens ?? 0).toLocaleString() }}
              </td>
              <td class="text-end text-green-600 dark:text-green-400">
                ${{ (model.estimated_cost ?? 0).toFixed(4) }}
              </td>
              <td class="text-end">
                {{ (model.avg_latency_ms ?? 0).toFixed(0) }}ms
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div
        v-else
        class="dashboard-card-empty"
      >
        {{ t('metrics.noData') }}
      </div>
    </div>
  </div>
</template>
