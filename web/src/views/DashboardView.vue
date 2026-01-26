<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useSystemStore } from '@/stores/system'
import { storeToRefs } from 'pinia'

const systemStore = useSystemStore()
const { health, workerStats } = storeToRefs(systemStore)

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
    <div class="flex items-center justify-between mb-6">
      <h1 class="text-2xl font-bold text-gray-900">Dashboard</h1>
      <label class="flex items-center space-x-2">
        <input
          v-model="autoRefresh"
          type="checkbox"
          class="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
        />
        <span class="text-sm text-gray-600">Auto refresh (5s)</span>
      </label>
    </div>

    <div class="grid gap-6 md:grid-cols-2">
      <!-- Health Card -->
      <div class="bg-white rounded-lg shadow">
        <div class="px-6 py-4 border-b border-gray-200">
          <h2 class="text-lg font-semibold text-gray-900">System Health</h2>
        </div>
        <div class="p-6" v-if="health">
          <dl class="space-y-4">
            <div class="flex justify-between">
              <dt class="text-gray-500">Status</dt>
              <dd class="font-medium" :class="health.status === 'ok' ? 'text-green-600' : 'text-red-600'">
                {{ health.status }}
              </dd>
            </div>
            <div class="flex justify-between">
              <dt class="text-gray-500">Uptime</dt>
              <dd class="font-medium text-gray-900">{{ health.uptime }}</dd>
            </div>
            <div class="flex justify-between">
              <dt class="text-gray-500">Memory</dt>
              <dd class="font-medium text-gray-900">
                {{ (health.mem_alloc_bytes / 1024 / 1024).toFixed(2) }} MB
              </dd>
            </div>
            <div class="flex justify-between">
              <dt class="text-gray-500">Goroutines</dt>
              <dd class="font-medium text-gray-900">{{ health.goroutines }}</dd>
            </div>
            <div class="flex justify-between">
              <dt class="text-gray-500">CPUs</dt>
              <dd class="font-medium text-gray-900">{{ health.num_cpu }}</dd>
            </div>
          </dl>
        </div>
      </div>

      <!-- Worker Stats Card -->
      <div class="bg-white rounded-lg shadow">
        <div class="px-6 py-4 border-b border-gray-200">
          <h2 class="text-lg font-semibold text-gray-900">Worker Pool</h2>
        </div>
        <div class="p-6" v-if="workerStats">
          <dl class="space-y-4">
            <div class="flex justify-between">
              <dt class="text-gray-500">Pool Size</dt>
              <dd class="font-medium text-gray-900">{{ workerStats.pool_size }}</dd>
            </div>
            <div class="flex justify-between">
              <dt class="text-gray-500">Running</dt>
              <dd class="font-medium text-gray-900">{{ workerStats.running }}</dd>
            </div>
            <div class="flex justify-between">
              <dt class="text-gray-500">Total Tasks</dt>
              <dd class="font-medium text-gray-900">{{ workerStats.total }}</dd>
            </div>
          </dl>
          <!-- Progress bar -->
          <div class="mt-4">
            <div class="flex justify-between text-sm text-gray-500 mb-1">
              <span>Pool Usage</span>
              <span>{{ workerStats.running }}/{{ workerStats.pool_size }}</span>
            </div>
            <div class="w-full bg-gray-200 rounded-full h-2">
              <div
                class="bg-blue-600 h-2 rounded-full transition-all"
                :style="{ width: `${(workerStats.running / workerStats.pool_size) * 100}%` }"
              ></div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
