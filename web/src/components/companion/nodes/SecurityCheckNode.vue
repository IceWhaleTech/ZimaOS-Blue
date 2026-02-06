<script setup lang="ts">
import { computed } from 'vue'
import { Handle, Position } from '@vue-flow/core'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const props = defineProps<{
  data: {
    label: string
    threatScore?: number
    threatTypes?: string[]
    action?: string
    status: string
  }
}>()

const threatLevel = computed(() => props.data.label || 'none')

const bgColor = computed(() => {
  switch (threatLevel.value) {
    case 'critical':
      return 'bg-red-50 dark:bg-red-900/30 border-red-300 dark:border-red-600'
    case 'high':
      return 'bg-orange-50 dark:bg-orange-900/30 border-orange-300 dark:border-orange-600'
    case 'medium':
      return 'bg-yellow-50 dark:bg-yellow-900/30 border-yellow-300 dark:border-yellow-600'
    case 'low':
      return 'bg-gray-700 dark:bg-gray-700 dark:bg-gray-700 dark:bg-gray-700/30 border-gray-900 dark:border-white dark:border-gray-900 dark:border-white'
    default:
      return 'bg-green-50 dark:bg-green-900/30 border-green-300 dark:border-green-600'
  }
})

const iconColor = computed(() => {
  switch (threatLevel.value) {
    case 'critical':
      return 'text-red-500 dark:text-red-400'
    case 'high':
      return 'text-orange-500 dark:text-orange-400'
    case 'medium':
      return 'text-yellow-500 dark:text-yellow-400'
    case 'low':
      return 'text-gray-900 dark:text-white dark:text-gray-900 dark:text-white'
    default:
      return 'text-green-500 dark:text-green-400'
  }
})

const iconPath = computed(() => {
  if (threatLevel.value === 'none') {
    return 'M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z'
  }
  return 'M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z'
})

const actionBadgeColor = computed(() => {
  switch (props.data.action) {
    case 'blocked':
      return 'bg-red-100 dark:bg-red-900/50 text-red-600 dark:text-red-400'
    case 'filtered':
      return 'bg-yellow-100 dark:bg-yellow-900/50 text-yellow-600 dark:text-yellow-400'
    default:
      return 'bg-green-100 dark:bg-green-900/50 text-green-600 dark:text-green-400'
  }
})
</script>

<template>
  <div
    :class="[
      'px-4 py-3 rounded-lg border-2 shadow-sm min-w-[200px] max-w-[300px]',
      bgColor
    ]"
  >
    <Handle type="target" :position="Position.Top" class="!bg-gray-400" />

    <div class="flex items-start gap-3">
      <div :class="['p-2 rounded-full bg-white dark:bg-gray-700', iconColor]">
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="iconPath" />
        </svg>
      </div>

      <div class="flex-1 min-w-0">
        <div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">
          {{ t('companion.nodes.securityCheck') }}
        </div>
        <div class="flex items-center gap-2">
          <span class="text-sm font-medium text-gray-900 dark:text-white">
            {{ t(`companion.threat.${threatLevel}`) }}
          </span>
          <span
            v-if="data.threatScore"
            class="text-xs text-gray-500 dark:text-gray-400"
          >
            ({{ data.threatScore }})
          </span>
        </div>

        <!-- Action badge -->
        <div v-if="data.action" class="mt-2">
          <span
            :class="[
              'px-1.5 py-0.5 text-[10px] rounded',
              actionBadgeColor
            ]"
          >
            {{ t(`companion.security.actions.${data.action}`) }}
          </span>
        </div>

        <!-- Threat types -->
        <div v-if="data.threatTypes?.length" class="mt-2 flex flex-wrap gap-1">
          <span
            v-for="type in data.threatTypes.slice(0, 3)"
            :key="type"
            class="px-1.5 py-0.5 text-[10px] bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400 rounded"
          >
            {{ type }}
          </span>
          <span
            v-if="data.threatTypes.length > 3"
            class="px-1.5 py-0.5 text-[10px] bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400 rounded"
          >
            +{{ data.threatTypes.length - 3 }}
          </span>
        </div>
      </div>
    </div>

    <Handle type="source" :position="Position.Bottom" class="!bg-gray-400" />
  </div>
</template>
