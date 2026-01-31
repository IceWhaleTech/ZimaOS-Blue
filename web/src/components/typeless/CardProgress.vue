<script setup lang="ts">
import { computed } from 'vue'
import type { TypelessCardProgress } from '@/types/typeless'

const props = defineProps<{
  card: TypelessCardProgress
}>()

const statusColors = {
  running: 'bg-blue-500',
  completed: 'bg-green-500',
  failed: 'bg-red-500',
  paused: 'bg-amber-500',
}

const statusIcons = {
  running: '⏳',
  completed: '✓',
  failed: '✗',
  paused: '⏸',
}

const progressWidth = computed(() => `${Math.min(100, Math.max(0, props.card.progress))}%`)
</script>

<template>
  <div class="rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 p-4">
    <div class="flex items-center justify-between mb-3">
      <h4 class="font-medium text-gray-900 dark:text-white">
        {{ card.title }}
      </h4>
      <span
        v-if="card.status"
        class="text-sm flex items-center gap-1"
        :class="{
          'text-blue-500': card.status === 'running',
          'text-green-500': card.status === 'completed',
          'text-red-500': card.status === 'failed',
          'text-amber-500': card.status === 'paused',
        }"
      >
        <span>{{ statusIcons[card.status] }}</span>
        <span class="capitalize">{{ card.status }}</span>
      </span>
    </div>

    <!-- Progress bar -->
    <div class="h-2 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden mb-2">
      <div
        class="h-full transition-all duration-300 rounded-full"
        :class="statusColors[card.status || 'running']"
        :style="{ width: progressWidth }"
      />
    </div>

    <div class="flex items-center justify-between text-sm text-gray-500 dark:text-gray-400">
      <span v-if="card.message">{{ card.message }}</span>
      <span v-else>&nbsp;</span>
      <span>{{ card.progress }}%</span>
    </div>

    <!-- Steps -->
    <div v-if="card.steps && card.steps.length > 0" class="mt-4 space-y-2">
      <div
        v-for="(step, index) in card.steps"
        :key="index"
        class="flex items-center gap-2 text-sm"
      >
        <span
          class="w-5 h-5 rounded-full flex items-center justify-center text-xs"
          :class="{
            'bg-gray-200 dark:bg-gray-700 text-gray-500': step.status === 'pending',
            'bg-blue-100 dark:bg-blue-900/30 text-blue-500': step.status === 'running',
            'bg-green-100 dark:bg-green-900/30 text-green-500': step.status === 'completed',
            'bg-red-100 dark:bg-red-900/30 text-red-500': step.status === 'failed',
          }"
        >
          <span v-if="step.status === 'pending'">{{ index + 1 }}</span>
          <span v-else-if="step.status === 'running'" class="animate-spin">⟳</span>
          <span v-else-if="step.status === 'completed'">✓</span>
          <span v-else-if="step.status === 'failed'">✗</span>
        </span>
        <span
          :class="{
            'text-gray-400': step.status === 'pending',
            'text-gray-700 dark:text-gray-300': step.status === 'running',
            'text-gray-600 dark:text-gray-400': step.status === 'completed',
            'text-red-500': step.status === 'failed',
          }"
        >
          {{ step.name }}
        </span>
        <span v-if="step.message" class="text-gray-400 text-xs">
          - {{ step.message }}
        </span>
      </div>
    </div>
  </div>
</template>
