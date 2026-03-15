<script setup lang="ts">
import { computed } from 'vue'
import { useMetricsStore } from '@/stores/metrics'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const metricsStore = useMetricsStore()

const users = computed(() => metricsStore.userTokenUsage?.users ?? [])
const summary = computed(() => metricsStore.userTokenUsage?.summary)
const showCard = computed(() => metricsStore.hasMultipleUsers)

function formatNumber(num: number): string {
  if (num >= 1000000) return (num / 1000000).toFixed(1) + 'M'
  if (num >= 1000) return (num / 1000).toFixed(1) + 'K'
  return num.toFixed(0)
}

function formatCost(cost: number): string {
  return '$' + cost.toFixed(4)
}
</script>

<template>
  <div v-if="showCard" class="bg-white dark:bg-gray-700 rounded-lg p-4 shadow">
    <div class="flex items-center justify-between mb-4">
      <h3 class="text-lg font-medium text-gray-900 dark:text-white">
        {{ t('metrics.userUsage') }}
      </h3>
      <span class="text-sm text-gray-500 dark:text-gray-400">
        {{ summary?.total_users ?? 0 }} {{ t('metrics.users') }}
      </span>
    </div>

    <div class="space-y-3">
      <div
        v-for="user in users"
        :key="user.user_id"
        class="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-700/50 rounded-lg"
      >
        <div class="flex items-center space-x-3">
          <div
            class="w-8 h-8 bg-green-100 dark:bg-green-900/30 rounded-full flex items-center justify-center"
          >
            <svg
              class="w-4 h-4 text-green-600 dark:text-green-400"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"
              />
            </svg>
          </div>
          <div>
            <p class="text-sm font-medium text-gray-900 dark:text-white">
              {{ user.user_id || t('metrics.anonymousUser') }}
            </p>
            <p class="text-xs text-gray-500 dark:text-gray-400">
              {{ user.request_count }} {{ t('metrics.requests') }}
            </p>
          </div>
        </div>
        <div class="text-right">
          <p class="text-sm font-medium text-gray-900 dark:text-white">
            {{ formatNumber(user.total_tokens) }} {{ t('metrics.tokens') }}
          </p>
          <p class="text-xs text-green-600 dark:text-green-400">
            {{ formatCost(user.estimated_cost) }}
          </p>
        </div>
      </div>
    </div>
  </div>
</template>
