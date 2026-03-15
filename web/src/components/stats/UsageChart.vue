<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  title: string
  data: Record<string, number>
}>()

const chartData = computed(() => {
  if (!props.data) return []
  const entries = Object.entries(props.data)
  const total = entries.reduce((sum, [, value]) => sum + value, 0)

  return entries.map(([label, value]) => ({
    label,
    value,
    percentage: total > 0 ? (value / total) * 100 : 0,
  }))
})

const colors = [
  'bg-gray-700 dark:bg-gray-500',
  'bg-green-500',
  'bg-yellow-500',
  'bg-purple-500',
  'bg-pink-500',
  'bg-indigo-500',
  'bg-red-500',
  'bg-orange-500',
]

function getColor(index: number): string {
  return colors[index % colors.length] ?? 'bg-gray-700 dark:bg-gray-500'
}
</script>

<template>
  <div
    class="usage-chart bg-white dark:bg-gray-700 rounded-lg p-4 border border-gray-200 dark:border-gray-700"
  >
    <h3 class="font-medium text-gray-900 dark:text-white mb-4">{{ props.title }}</h3>

    <!-- Empty State -->
    <div v-if="chartData.length === 0" class="text-center py-8 text-gray-500 dark:text-gray-400">
      No data
    </div>

    <!-- Bar Chart -->
    <div v-else class="space-y-3">
      <div v-for="(item, index) in chartData" :key="item.label" class="space-y-1">
        <div class="flex items-center justify-between text-sm">
          <span class="text-gray-700 dark:text-gray-300 truncate">{{ item.label }}</span>
          <span class="text-gray-500 dark:text-gray-400 ml-2">
            {{ item.value.toLocaleString() }} ({{ item.percentage.toFixed(1) }}%)
          </span>
        </div>
        <div class="h-2 bg-gray-700 dark:bg-gray-500 rounded-full overflow-hidden">
          <div
            :class="['h-full rounded-full transition-all duration-300', getColor(index)]"
            :style="{ width: `${item.percentage}%` }"
          />
        </div>
      </div>
    </div>
  </div>
</template>
