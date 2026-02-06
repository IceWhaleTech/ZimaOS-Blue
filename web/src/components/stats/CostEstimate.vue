<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const props = defineProps<{
  cost: number
}>()

const formattedCost = computed(() => {
  return new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits: 2,
    maximumFractionDigits: 4,
  }).format(props.cost ?? 0)
})

const costLevel = computed(() => {
  const cost = props.cost ?? 0
  if (cost < 1) return 'low'
  if (cost < 10) return 'medium'
  return 'high'
})
</script>

<template>
  <div class="cost-estimate bg-white dark:bg-gray-700 rounded-lg p-4 border border-gray-200 dark:border-gray-700">
    <div class="flex items-center justify-between">
      <div>
        <h3 class="font-medium text-gray-900 dark:text-white">
          {{ t('stats.estimatedCost') }}
        </h3>
        <p class="text-sm text-gray-500 dark:text-gray-400">
          {{ t('stats.costDisclaimer') }}
        </p>
      </div>
      <div class="text-right">
        <p
          :class="[
            'text-2xl font-semibold',
            costLevel === 'low' ? 'text-green-600 dark:text-green-400' :
            costLevel === 'medium' ? 'text-yellow-600 dark:text-yellow-400' :
            'text-red-600 dark:text-red-400'
          ]"
        >
          {{ formattedCost }}
        </p>
        <p class="text-xs text-gray-400 dark:text-gray-500">
          {{ t('stats.thisPeriod') }}
        </p>
      </div>
    </div>
  </div>
</template>
