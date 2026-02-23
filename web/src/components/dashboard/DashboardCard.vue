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
    blue: 'bg-blue-100 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400',
    green: 'bg-green-100 dark:bg-green-900/30 text-green-600 dark:text-green-400',
    purple: 'bg-purple-100 dark:bg-purple-900/30 text-purple-600 dark:text-purple-400',
    orange: 'bg-orange-100 dark:bg-orange-900/30 text-orange-600 dark:text-orange-400',
    red: 'bg-red-100 dark:bg-red-900/30 text-red-600 dark:text-red-400',
    gray: 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400',
  }
  return colors[props.iconColor] ?? colors.blue
})
</script>

<template>
  <div class="bg-white dark:bg-gray-700 rounded-lg shadow overflow-hidden">
    <!-- Header -->
    <div class="flex items-center justify-between p-4 border-b border-gray-100 dark:border-gray-700">
      <div class="flex items-center gap-3">
        <div v-if="icon" :class="['p-2 rounded-lg', iconColorClass]">
          <component :is="icon" class="w-5 h-5" />
        </div>
        <h3 class="font-semibold text-gray-900 dark:text-white">{{ title }}</h3>
      </div>
      <div class="flex items-center gap-2">
        <button
          v-if="collapsible"
          class="p-1 hover:bg-gray-100 dark:hover:bg-gray-700 rounded transition-colors"
          @click="emit('toggle-collapse')"
        >
          <svg
            class="w-5 h-5 text-gray-500 dark:text-gray-400 transition-transform"
            :class="{ 'rotate-180': collapsed }"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
          </svg>
        </button>
        <button
          v-if="removable"
          class="p-1 hover:bg-red-100 dark:hover:bg-red-900/30 rounded transition-colors"
          @click="emit('remove')"
        >
          <svg class="w-5 h-5 text-gray-400 hover:text-red-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
    </div>

    <!-- Content -->
    <div v-show="!collapsed" class="p-4">
      <!-- Loading State -->
      <div v-if="loading" class="flex items-center justify-center py-8">
        <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900 dark:border-white"></div>
      </div>

      <!-- Error State -->
      <div v-else-if="error" class="text-center py-8 text-red-500">
        {{ error }}
      </div>

      <!-- Content Slot -->
      <slot v-else />
    </div>
  </div>
</template>
