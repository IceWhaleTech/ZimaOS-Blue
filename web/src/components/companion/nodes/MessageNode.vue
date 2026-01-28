<script setup lang="ts">
import { computed } from 'vue'
import { Handle, Position } from '@vue-flow/core'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const props = defineProps<{
  data: {
    label: string
    direction: 'inbound' | 'outbound'
    contentType?: string
    length?: number
    status: string
  }
}>()

const isInbound = computed(() => props.data.direction === 'inbound')

const iconPath = computed(() => {
  return isInbound.value
    ? 'M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z' // inbox
    : 'M12 19l9 2-9-18-9 18 9-2zm0 0v-8' // send
})

const bgColor = computed(() => {
  return isInbound.value
    ? 'bg-blue-50 dark:bg-blue-900/30 border-blue-200 dark:border-blue-700'
    : 'bg-green-50 dark:bg-green-900/30 border-green-200 dark:border-green-700'
})

const iconColor = computed(() => {
  return isInbound.value
    ? 'text-blue-500 dark:text-blue-400'
    : 'text-green-500 dark:text-green-400'
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
      <div :class="['p-2 rounded-full bg-white dark:bg-gray-800', iconColor]">
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="iconPath" />
        </svg>
      </div>

      <div class="flex-1 min-w-0">
        <div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">
          {{ isInbound ? t('companion.nodes.received') : t('companion.nodes.sent') }}
        </div>
        <div class="text-sm text-gray-900 dark:text-white truncate">
          {{ data.label || t('companion.nodes.message') }}
        </div>
        <div v-if="data.length" class="text-xs text-gray-400 dark:text-gray-500 mt-1">
          {{ t('companion.nodes.chars', { count: data.length }) }}
        </div>
      </div>
    </div>

    <Handle type="source" :position="Position.Bottom" class="!bg-gray-400" />
  </div>
</template>
