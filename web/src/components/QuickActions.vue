<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

export interface QuickAction {
  id: string
  label: string
  description?: string
  icon: string
  color?: 'primary' | 'success' | 'warning' | 'danger' | 'secondary'
  disabled?: boolean
}

withDefaults(
  defineProps<{
    actions?: QuickAction[]
    loading?: boolean
  }>(),
  {
    actions: () => [],
    loading: false,
  }
)

const emit = defineEmits<{
  action: [actionId: string]
}>()

function getColorClasses(color: QuickAction['color'] = 'primary'): string {
  switch (color) {
    case 'primary':
      return 'bg-gray-700 dark:bg-gray-500/20 text-gray-900 dark:text-white dark:text-white hover:bg-gray-700 dark:bg-gray-500 dark:hover:bg-gray-200 dark:bg-gray-600/30'
    case 'success':
      return 'bg-green-50 dark:bg-green-900/20 text-green-600 dark:text-green-400 hover:bg-green-100 dark:hover:bg-green-900/30'
    case 'warning':
      return 'bg-yellow-50 dark:bg-yellow-900/20 text-yellow-600 dark:text-yellow-400 hover:bg-yellow-100 dark:hover:bg-yellow-900/30'
    case 'danger':
      return 'bg-red-50 dark:bg-red-900/20 text-red-600 dark:text-red-400 hover:bg-red-100 dark:hover:bg-red-900/30'
    case 'secondary':
      return 'bg-gray-50 dark:bg-gray-700 text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-600'
  }
}

function handleAction(action: QuickAction) {
  if (!action.disabled) {
    emit('action', action.id)
  }
}
</script>

<template>
  <div class="bg-white dark:bg-gray-700 rounded-lg shadow">
    <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('common.quickActionsTitle') }}</h2>
    </div>

    <div v-if="loading" class="p-6 text-center">
      <div class="animate-spin h-8 w-8 border-4 border-gray-900 dark:border-white border-t-transparent rounded-full mx-auto"></div>
      <p class="mt-2 text-gray-500 dark:text-gray-400">{{ t('common.loadingActions') }}</p>
    </div>

    <div v-else-if="actions.length === 0" class="p-6 text-center text-gray-500 dark:text-gray-400">
      {{ t('common.noActionsAvailable') }}
    </div>

    <div v-else class="p-4 grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-3">
      <button
        v-for="action in actions"
        :key="action.id"
        class="flex flex-col items-center p-4 rounded-lg transition-colors"
        :class="[
          getColorClasses(action.color),
          action.disabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'
        ]"
        :disabled="action.disabled"
        @click="handleAction(action)"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-8 w-8 mb-2"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="action.icon" />
        </svg>
        <span class="text-sm font-medium text-center">{{ action.label }}</span>
        <span v-if="action.description" class="text-xs opacity-75 text-center mt-1">
          {{ action.description }}
        </span>
      </button>
    </div>
  </div>
</template>
