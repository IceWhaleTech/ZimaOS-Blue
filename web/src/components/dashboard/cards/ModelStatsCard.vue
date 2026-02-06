<script setup lang="ts">
import { useMetricsStore } from '@/stores/metrics'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const metricsStore = useMetricsStore()
</script>

<template>
  <div class="bg-white dark:bg-gray-700 rounded-lg p-4 shadow border border-gray-200 dark:border-gray-700">
    <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">{{ t('metrics.modelStats') }}</h3>
    <div v-if="metricsStore.modelStats?.models?.length" class="overflow-x-auto">
      <table class="w-full text-sm">
        <thead>
          <tr class="border-b border-gray-200 dark:border-gray-700">
            <th class="text-left py-3 px-2 text-gray-500 dark:text-gray-400">{{ t('metrics.model') }}</th>
            <th class="text-right py-3 px-2 text-gray-500 dark:text-gray-400">{{ t('metrics.calls') }}</th>
            <th class="text-right py-3 px-2 text-gray-500 dark:text-gray-400">{{ t('metrics.successRate') }}</th>
            <th class="text-right py-3 px-2 text-gray-500 dark:text-gray-400">{{ t('metrics.tokens') }}</th>
            <th class="text-right py-3 px-2 text-gray-500 dark:text-gray-400">{{ t('metrics.cost') }}</th>
            <th class="text-right py-3 px-2 text-gray-500 dark:text-gray-400">{{ t('metrics.avgLatency') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="model in metricsStore.modelStats.models"
            :key="model.model"
            class="border-b border-gray-100 dark:border-gray-800 hover:bg-gray-50 dark:hover:bg-gray-700/50"
          >
            <td class="py-3 px-2 font-medium text-gray-900 dark:text-white">{{ model.model }}</td>
            <td class="py-3 px-2 text-right text-gray-700 dark:text-gray-300">{{ model.calls ?? 0 }}</td>
            <td class="py-3 px-2 text-right">
              <span :class="(model.success_rate ?? 0) >= 95 ? 'text-green-600 dark:text-green-400' : 'text-orange-600 dark:text-orange-400'">
                {{ (model.success_rate ?? 0).toFixed(1) }}%
              </span>
            </td>
            <td class="py-3 px-2 text-right text-gray-700 dark:text-gray-300">{{ (model.total_tokens ?? 0).toLocaleString() }}</td>
            <td class="py-3 px-2 text-right text-green-600 dark:text-green-400">${{ (model.estimated_cost ?? 0).toFixed(4) }}</td>
            <td class="py-3 px-2 text-right text-gray-700 dark:text-gray-300">{{ (model.avg_latency_ms ?? 0).toFixed(0) }}ms</td>
          </tr>
        </tbody>
      </table>
    </div>
    <div v-else class="text-center py-8 text-gray-500 dark:text-gray-400">
      {{ t('metrics.noData') }}
    </div>
  </div>
</template>
