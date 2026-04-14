<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { TypelessCardAlert } from '@/types/typeless'
import { translateCardActionLabel } from '@/utils/cardActionLabels'

const props = defineProps<{
  card: TypelessCardAlert
}>()

const emit = defineEmits<{
  dismiss: []
  action: [actionId: string]
}>()

const { t, te } = useI18n()
const dismissed = ref(false)

const displayTitle = computed(() => {
  if (props.card.title_key && te(props.card.title_key)) {
    return t(props.card.title_key)
  }
  return props.card.title
})

const variantConfig = {
  info: {
    container: 'alert-info',
    iconBg: 'bg-blue-100 dark:bg-blue-500/20',
    iconColor: 'text-blue-600 dark:text-blue-400',
    title: 'text-blue-900 dark:text-blue-100',
    text: 'text-blue-800/80 dark:text-blue-200/80',
    svg: 'info',
  },
  success: {
    container: 'alert-success',
    iconBg: 'bg-emerald-100 dark:bg-emerald-500/20',
    iconColor: 'text-emerald-600 dark:text-emerald-400',
    title: 'text-emerald-900 dark:text-emerald-100',
    text: 'text-emerald-800/80 dark:text-emerald-200/80',
    svg: 'success',
  },
  warning: {
    container: 'alert-warning',
    iconBg: 'bg-amber-100 dark:bg-amber-500/20',
    iconColor: 'text-amber-600 dark:text-amber-400',
    title: 'text-amber-900 dark:text-amber-100',
    text: 'text-amber-800/80 dark:text-amber-200/80',
    svg: 'warning',
  },
  error: {
    container: 'alert-error',
    iconBg: 'bg-red-100 dark:bg-red-500/20',
    iconColor: 'text-red-600 dark:text-red-400',
    title: 'text-red-900 dark:text-red-100',
    text: 'text-red-800/80 dark:text-red-200/80',
    svg: 'error',
  },
}

const config = computed(() => variantConfig[props.card.variant])

function handleDismiss() {
  dismissed.value = true
  emit('dismiss')
}

function actionLabel(action: { id: string; label: string }): string {
  return translateCardActionLabel({ id: action.id, fallback: action.label, t, te })
}

function handleAction(actionId: string) {
  emit('action', actionId)
}
</script>

<template>
  <div
    v-if="!dismissed"
    class="alert-card"
    :class="config.container"
  >
    <!-- Icon -->
    <div
      class="alert-icon"
      :class="[config.iconBg, config.iconColor]"
    >
      <!-- Info -->
      <svg
        v-if="card.icon"
        class="w-5 h-5"
        viewBox="0 0 20 20"
        fill="currentColor"
      >
        <text
          x="50%"
          y="50%"
          dominant-baseline="central"
          text-anchor="middle"
          font-size="14"
        >
          {{ card.icon }}
        </text>
      </svg>
      <template v-else>
        <svg
          v-if="config.svg === 'info'"
          class="w-5 h-5"
          viewBox="0 0 20 20"
          fill="currentColor"
        >
          <path
            fill-rule="evenodd"
            d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a.75.75 0 000 1.5h.253a.25.25 0 01.244.304l-.459 2.066A1.75 1.75 0 0010.747 15H11a.75.75 0 000-1.5h-.253a.25.25 0 01-.244-.304l.459-2.066A1.75 1.75 0 009.253 9H9z"
            clip-rule="evenodd"
          />
        </svg>
        <svg
          v-else-if="config.svg === 'success'"
          class="w-5 h-5"
          viewBox="0 0 20 20"
          fill="currentColor"
        >
          <path
            fill-rule="evenodd"
            d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.857-9.809a.75.75 0 00-1.214-.882l-3.483 4.79-1.88-1.88a.75.75 0 10-1.06 1.061l2.5 2.5a.75.75 0 001.137-.089l4-5.5z"
            clip-rule="evenodd"
          />
        </svg>
        <svg
          v-else-if="config.svg === 'warning'"
          class="w-5 h-5"
          viewBox="0 0 20 20"
          fill="currentColor"
        >
          <path
            fill-rule="evenodd"
            d="M8.485 2.495c.673-1.167 2.357-1.167 3.03 0l6.28 10.875c.673 1.167-.17 2.625-1.516 2.625H3.72c-1.347 0-2.189-1.458-1.515-2.625L8.485 2.495zM10 5a.75.75 0 01.75.75v3.5a.75.75 0 01-1.5 0v-3.5A.75.75 0 0110 5zm0 9a1 1 0 100-2 1 1 0 000 2z"
            clip-rule="evenodd"
          />
        </svg>
        <svg
          v-else-if="config.svg === 'error'"
          class="w-5 h-5"
          viewBox="0 0 20 20"
          fill="currentColor"
        >
          <path
            fill-rule="evenodd"
            d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.28 7.22a.75.75 0 00-1.06 1.06L8.94 10l-1.72 1.72a.75.75 0 101.06 1.06L10 11.06l1.72 1.72a.75.75 0 101.06-1.06L11.06 10l1.72-1.72a.75.75 0 00-1.06-1.06L10 8.94 8.28 7.22z"
            clip-rule="evenodd"
          />
        </svg>
      </template>
    </div>

    <!-- Content -->
    <div class="alert-body">
      <p
        v-if="displayTitle"
        class="alert-title"
        :class="config.title"
      >
        {{ displayTitle }}
      </p>
      <p
        class="alert-message"
        :class="[config.text, { 'mt-0.5': displayTitle }]"
      >
        {{ card.message }}
      </p>

      <!-- Actions -->
      <div
        v-if="card.actions?.length"
        class="mt-3 flex gap-2"
      >
        <button
          v-for="action in card.actions"
          :key="action.id"
          class="alert-action"
          :class="{
            'alert-action-primary': action.variant === 'primary',
            'alert-action-secondary': action.variant !== 'primary' && action.variant !== 'danger',
            'alert-action-danger': action.variant === 'danger',
          }"
          :disabled="action.disabled"
          @click="handleAction(action.id)"
        >
          {{ actionLabel(action) }}
        </button>
      </div>
    </div>

    <!-- Dismiss button -->
    <button
      v-if="card.dismissible"
      class="alert-dismiss"
      :class="config.iconColor"
      @click="handleDismiss"
    >
      <svg
        class="w-4 h-4"
        viewBox="0 0 20 20"
        fill="currentColor"
      >
        <path
          d="M6.28 5.22a.75.75 0 00-1.06 1.06L8.94 10l-3.72 3.72a.75.75 0 101.06 1.06L10 11.06l3.72 3.72a.75.75 0 101.06-1.06L11.06 10l3.72-3.72a.75.75 0 00-1.06-1.06L10 8.94 6.28 5.22z"
        />
      </svg>
    </button>
  </div>
</template>

<style scoped>
.alert-card {
  display: flex;
  align-items: flex-start;
  gap: 0.75rem;
  padding: 0.875rem 1rem;
  border-radius: 0.75rem;
  border: 1px solid;
  transition: all 0.2s ease;
}

.alert-info {
  background: linear-gradient(135deg, rgba(239, 246, 255, 0.8), rgba(219, 234, 254, 0.4));
  border-color: rgba(147, 197, 253, 0.5);
}
:is(.dark) .alert-info {
  background: linear-gradient(135deg, rgba(30, 58, 138, 0.15), rgba(29, 78, 216, 0.08));
  border-color: rgba(59, 130, 246, 0.2);
}

.alert-success {
  background: linear-gradient(135deg, rgba(236, 253, 245, 0.8), rgba(209, 250, 229, 0.4));
  border-color: rgba(110, 231, 183, 0.5);
}
:is(.dark) .alert-success {
  background: linear-gradient(135deg, rgba(6, 78, 59, 0.15), rgba(5, 150, 105, 0.08));
  border-color: rgba(16, 185, 129, 0.2);
}

.alert-warning {
  background: linear-gradient(135deg, rgba(255, 251, 235, 0.8), rgba(254, 243, 199, 0.4));
  border-color: rgba(252, 211, 77, 0.5);
}
:is(.dark) .alert-warning {
  background: linear-gradient(135deg, rgba(120, 53, 15, 0.15), rgba(217, 119, 6, 0.08));
  border-color: rgba(245, 158, 11, 0.2);
}

.alert-error {
  background: linear-gradient(135deg, rgba(254, 242, 242, 0.8), rgba(254, 226, 226, 0.4));
  border-color: rgba(252, 165, 165, 0.5);
}
:is(.dark) .alert-error {
  background: linear-gradient(135deg, rgba(127, 29, 29, 0.15), rgba(220, 38, 38, 0.08));
  border-color: rgba(239, 68, 68, 0.2);
}

.alert-icon {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 2rem;
  height: 2rem;
  border-radius: 0.5rem;
}

.alert-body {
  flex: 1;
  min-width: 0;
  padding-top: 0.125rem;
}

.alert-title {
  font-size: 0.875rem;
  font-weight: 600;
  line-height: 1.25rem;
}

.alert-message {
  font-size: 0.8125rem;
  line-height: 1.25rem;
}

.alert-dismiss {
  flex-shrink: 0;
  padding: 0.25rem;
  border-radius: 0.375rem;
  opacity: 0.5;
  transition: opacity 0.15s ease;
  cursor: pointer;
  background: none;
  border: none;
}
.alert-dismiss:hover {
  opacity: 1;
}

.alert-action {
  padding: 0.375rem 0.75rem;
  font-size: 0.8125rem;
  font-weight: 500;
  border-radius: 0.5rem;
  transition: all 0.15s ease;
  cursor: pointer;
  border: 1px solid transparent;
}

.alert-action-primary {
  background: rgba(0, 0, 0, 0.8);
  color: white;
}
:is(.dark) .alert-action-primary {
  background: rgba(255, 255, 255, 0.15);
}
.alert-action-primary:hover {
  background: rgba(0, 0, 0, 0.9);
}
:is(.dark) .alert-action-primary:hover {
  background: rgba(255, 255, 255, 0.25);
}

.alert-action-secondary {
  background: rgba(255, 255, 255, 0.8);
  color: rgba(0, 0, 0, 0.7);
  border-color: rgba(0, 0, 0, 0.1);
}
:is(.dark) .alert-action-secondary {
  background: rgba(255, 255, 255, 0.08);
  color: rgba(255, 255, 255, 0.7);
  border-color: rgba(255, 255, 255, 0.1);
}

.alert-action-danger {
  background: rgba(220, 38, 38, 0.9);
  color: white;
}
.alert-action-danger:hover {
  background: rgba(185, 28, 28, 0.95);
}
</style>
