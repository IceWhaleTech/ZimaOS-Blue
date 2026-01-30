<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSystemStore } from '@/stores/system'
import { storeToRefs } from 'pinia'
import Skeleton from '@/components/Skeleton.vue'

const { t } = useI18n()
const systemStore = useSystemStore()
const { health, loading } = storeToRefs(systemStore)

let refreshInterval: ReturnType<typeof setInterval> | null = null
const autoRefresh = ref(true)

onMounted(() => {
  systemStore.fetchAll()
  refreshInterval = setInterval(() => {
    if (autoRefresh.value) {
      systemStore.fetchAll()
    }
  }, 5000)
})

onUnmounted(() => {
  if (refreshInterval) {
    clearInterval(refreshInterval)
  }
})
</script>

<template>
  <div>
    <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 mb-6">
      <h1 class="text-xl sm:text-2xl font-bold text-gray-900 dark:text-white">{{ t('dashboard.title') }}</h1>
      <label class="flex items-center space-x-2 cursor-pointer group">
        <input
          v-model="autoRefresh"
          type="checkbox"
          class="w-4 h-4 rounded border-gray-300 dark:border-slate-600 bg-gray-100 dark:bg-slate-700 text-accent focus:ring-accent focus:ring-offset-0 cursor-pointer"
        />
        <span class="text-sm text-gray-500 dark:text-slate-400 group-hover:text-gray-700 dark:group-hover:text-slate-300 transition-colors">{{ t('dashboard.autoRefresh') }}</span>
      </label>
    </div>

    <!-- Health Card -->
    <div class="glass-card p-4 sm:p-6 max-w-xl">
      <div class="flex items-center space-x-3 mb-6">
        <div class="w-10 h-10 rounded-lg bg-cta/20 flex items-center justify-center">
          <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-cta" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        </div>
        <h2 class="heading-sm text-gray-900 dark:text-white">{{ t('dashboard.systemHealth') }}</h2>
      </div>
      <dl class="space-y-4">
        <div class="flex justify-between items-center">
          <dt class="text-gray-500 dark:text-slate-400">{{ t('common.status') }}</dt>
          <dd v-if="loading && !health">
            <Skeleton height="1.5rem" width="4rem" rounded="full" />
          </dd>
          <dd
            v-else
            class="px-3 py-1 rounded-full text-sm font-medium"
            :class="health?.status === 'ok'
              ? 'bg-cta/20 text-cta'
              : 'bg-gray-200 dark:bg-slate-700 text-gray-500 dark:text-slate-400'"
          >
            {{ health?.status === 'ok' ? t('dashboard.healthy') : (health?.status || '-') }}
          </dd>
        </div>
        <div class="flex justify-between items-center">
          <dt class="text-gray-500 dark:text-slate-400">{{ t('dashboard.uptime') }}</dt>
          <dd v-if="loading && !health">
            <Skeleton height="1.25rem" width="6rem" rounded="md" />
          </dd>
          <dd v-else class="font-medium text-gray-900 dark:text-white">{{ health?.uptime || '-' }}</dd>
        </div>
        <div class="flex justify-between items-center">
          <dt class="text-gray-500 dark:text-slate-400">{{ t('dashboard.memory') }}</dt>
          <dd v-if="loading && !health">
            <Skeleton height="1.25rem" width="5rem" rounded="md" />
          </dd>
          <dd v-else class="font-medium text-gray-900 dark:text-white">
            {{ health ? (health.mem_alloc_bytes / 1024 / 1024).toFixed(2) + ' MB' : '-' }}
          </dd>
        </div>
        <div class="flex justify-between items-center">
          <dt class="text-gray-500 dark:text-slate-400">{{ t('dashboard.goroutines') }}</dt>
          <dd v-if="loading && !health">
            <Skeleton height="1.25rem" width="3rem" rounded="md" />
          </dd>
          <dd v-else class="font-medium text-gray-900 dark:text-white">{{ health?.goroutines || '-' }}</dd>
        </div>
        <div class="flex justify-between items-center">
          <dt class="text-gray-500 dark:text-slate-400">{{ t('dashboard.cpus') }}</dt>
          <dd v-if="loading && !health">
            <Skeleton height="1.25rem" width="2rem" rounded="md" />
          </dd>
          <dd v-else class="font-medium text-gray-900 dark:text-white">{{ health?.num_cpu || '-' }}</dd>
        </div>
      </dl>
    </div>
  </div>
</template>
