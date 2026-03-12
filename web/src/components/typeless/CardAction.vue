<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { ActionButton, TypelessCardAction } from '@/types/typeless'
import { translateCardActionLabel } from '@/utils/cardActionLabels'

const { t, te } = useI18n()

const props = defineProps<{
  card: TypelessCardAction
  actionLoading?: boolean
  activeActionId?: string
}>()

const emit = defineEmits<{
  action: [actionId: string, cardId?: string]
}>()

const buttonClasses = {
  primary: 'bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white',
  secondary: 'bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 text-gray-700 dark:text-gray-300',
  danger: 'bg-red-500 hover:bg-red-600 text-white',
}

function isActionActive(actionId: string): boolean {
  return props.actionLoading === true && props.activeActionId === actionId
}

function isActionDisabled(action: ActionButton): boolean {
  return props.actionLoading === true || action.disabled === true
}

function actionButtonLabel(action: ActionButton): string {
  if (isActionActive(action.id)) return t('common.processing', 'Processing...')
  return translateCardActionLabel({ id: action.id, fallback: action.label, t, te })
}

function handleClick(actionId: string, disabled = false) {
  if (props.actionLoading || disabled) return
  emit('action', actionId, props.card.id)
}
</script>

<template>
  <div class="rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-700 p-4">
    <h4 class="font-medium text-gray-900 dark:text-white mb-1">
      {{ card.title }}
    </h4>
    <p
      v-if="card.description"
      class="text-sm text-gray-500 dark:text-gray-400 mb-4"
    >
      {{ card.description }}
    </p>

    <div class="flex flex-wrap gap-2">
      <button
        v-for="action in card.actions"
        :key="action.id"
        class="px-4 py-2 rounded-lg text-sm font-medium transition-colors disabled:opacity-60 disabled:cursor-wait"
        :class="[buttonClasses[action.variant || 'secondary'], { 'opacity-60 cursor-wait': actionLoading }]"
        :disabled="isActionDisabled(action)"
        :aria-busy="isActionActive(action.id) ? 'true' : undefined"
        @click="handleClick(action.id, !!action.disabled)"
      >
        <span
          v-if="isActionActive(action.id)"
          class="mr-1.5 inline-block h-3 w-3 animate-spin rounded-full border border-current border-r-transparent align-[-2px]"
        />
        <span v-else-if="action.icon" class="mr-1.5">{{ action.icon }}</span>
        {{ actionButtonLabel(action) }}
      </button>
    </div>
  </div>
</template>
