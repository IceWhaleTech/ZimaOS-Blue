<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { myApi, type UserTokenUsage } from '@/api/my'

const { t } = useI18n()
const usage = ref<UserTokenUsage | null>(null)
const loading = ref(true)
const error = ref(false)

onMounted(async () => {
  try {
    const { data } = await myApi.getUsage()
    usage.value = data
  } catch {
    error.value = true
  } finally {
    loading.value = false
  }
})

const formatNumber = (n: number) => n?.toLocaleString() ?? '0'
const formatCost = (n: number) => `$${(n ?? 0).toFixed(4)}`

const cards = computed(() => [
  { label: t('myUsage.totalTokens'), value: formatNumber(usage.value?.total_tokens ?? 0), icon: '📊' },
  { label: t('myUsage.requests'), value: formatNumber(usage.value?.request_count ?? 0), icon: '📨' },
  { label: t('myUsage.estimatedCost'), value: formatCost(usage.value?.estimated_cost ?? 0), icon: '💰' },
  { label: t('myUsage.inputTokens'), value: formatNumber(usage.value?.input_tokens ?? 0), icon: '📥' },
  { label: t('myUsage.outputTokens'), value: formatNumber(usage.value?.output_tokens ?? 0), icon: '📤' },
  { label: t('myUsage.cacheTokens'), value: formatNumber((usage.value?.cache_read_tokens ?? 0) + (usage.value?.cache_write_tokens ?? 0)), icon: '⚡' },
])
</script>

<template>
  <div class="max-w-4xl mx-auto">
    <h1 class="text-2xl font-bold text-gray-900 dark:text-white mb-6">{{ t('myUsage.title') }}</h1>

    <div v-if="loading" class="text-gray-500 dark:text-gray-400">{{ t('common.loading') }}...</div>
    <div v-else-if="error" class="text-red-500 dark:text-red-400">{{ t('myUsage.loadError') }}</div>

    <div v-else class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
      <div
        v-for="card in cards"
        :key="card.label"
        class="bg-white dark:bg-gray-800 rounded-xl p-5 border border-gray-200 dark:border-gray-700"
      >
        <div class="text-sm text-gray-500 dark:text-gray-400 mb-1">{{ card.label }}</div>
        <div class="text-2xl font-semibold text-gray-900 dark:text-white">{{ card.value }}</div>
      </div>
    </div>
  </div>
</template>
