<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    title: string
    icon?: string
    iconColor?: 'blue' | 'green' | 'purple' | 'orange' | 'red' | 'gray'
    loading?: boolean
    error?: string | null
    collapsible?: boolean
    collapsed?: boolean
    removable?: boolean
  }>(),
  {
    icon: '',
    iconColor: 'blue',
    loading: false,
    error: null,
    collapsible: false,
    collapsed: false,
    removable: false,
  }
)

const emit = defineEmits<{
  (e: 'toggle-collapse'): void
  (e: 'remove'): void
}>()

const iconColorClass = computed(() => {
  const colors: Record<string, string> = {
    blue: 'text-blue-600 dark:text-blue-400',
    green: 'text-green-600 dark:text-green-400',
    purple: 'text-purple-600 dark:text-purple-400',
    orange: 'text-orange-600 dark:text-orange-400',
    red: 'text-red-600 dark:text-red-400',
    gray: 'text-gray-600 dark:text-gray-400',
  }
  return colors[props.iconColor] ?? colors.blue
})
</script>

<template>
  <div class="dashboard-card-surface overflow-hidden">
    <div
      class="flex items-center justify-between p-4 border-b border-slate-200/70 dark:border-slate-700/70"
    >
      <div class="flex items-center gap-3 min-w-0">
        <div v-if="icon" class="dashboard-card-chip">
          <component :is="icon" :class="['w-5 h-5', iconColorClass]" />
        </div>
        <div class="min-w-0">
          <p class="dashboard-card-label">Card</p>
          <h3 class="dashboard-card-subtitle mt-2 truncate" :class="iconColorClass">{{ title }}</h3>
        </div>
      </div>
      <div class="flex items-center gap-2">
        <button v-if="collapsible" class="dashboard-card-chip" @click="emit('toggle-collapse')">
          <svg
            class="w-4 h-4 text-gray-500 dark:text-gray-400 transition-transform"
            :class="{ 'rotate-180': collapsed }"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M19 9l-7 7-7-7"
            />
          </svg>
        </button>
        <button v-if="removable" class="dashboard-card-chip" @click="emit('remove')">
          <svg
            class="w-4 h-4 text-gray-400 hover:text-red-500"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M6 18L18 6M6 6l12 12"
            />
          </svg>
        </button>
      </div>
    </div>

    <div v-show="!collapsed" class="p-4">
      <div v-if="loading" class="flex items-center justify-center py-8">
        <div
          class="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900 dark:border-white"
        ></div>
      </div>

      <div v-else-if="error" class="text-center py-8 text-red-500">
        {{ error }}
      </div>

      <slot v-else />
    </div>
  </div>
</template>
