<script setup lang="ts">
import { onMounted } from 'vue'
import { useSystemStore } from '@/stores/system'
import { storeToRefs } from 'pinia'

const systemStore = useSystemStore()
const { health, loading, error } = storeToRefs(systemStore)

onMounted(() => {
  systemStore.fetchHealth()
})
</script>

<template>
  <div class="max-w-4xl mx-auto">
    <h1 class="text-3xl font-bold text-gray-900 mb-8">Welcome to ZimaOS Echo</h1>

    <div v-if="loading" class="text-center py-8">
      <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600 mx-auto"></div>
      <p class="mt-4 text-gray-500">Loading...</p>
    </div>

    <div v-else-if="error" class="bg-red-50 border border-red-200 rounded-lg p-4">
      <p class="text-red-700">{{ error }}</p>
    </div>

    <div v-else-if="health" class="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
      <div class="bg-white rounded-lg shadow p-6">
        <h3 class="text-sm font-medium text-gray-500 mb-2">Status</h3>
        <p class="text-2xl font-bold" :class="health.status === 'ok' ? 'text-green-600' : 'text-red-600'">
          {{ health.status.toUpperCase() }}
        </p>
      </div>

      <div class="bg-white rounded-lg shadow p-6">
        <h3 class="text-sm font-medium text-gray-500 mb-2">Version</h3>
        <p class="text-2xl font-bold text-gray-900">{{ health.version }}</p>
      </div>

      <div class="bg-white rounded-lg shadow p-6">
        <h3 class="text-sm font-medium text-gray-500 mb-2">Uptime</h3>
        <p class="text-2xl font-bold text-gray-900">{{ health.uptime }}</p>
      </div>

      <div class="bg-white rounded-lg shadow p-6">
        <h3 class="text-sm font-medium text-gray-500 mb-2">Memory</h3>
        <p class="text-2xl font-bold text-gray-900">
          {{ (health.mem_alloc_bytes / 1024 / 1024).toFixed(2) }} MB
        </p>
      </div>

      <div class="bg-white rounded-lg shadow p-6">
        <h3 class="text-sm font-medium text-gray-500 mb-2">Goroutines</h3>
        <p class="text-2xl font-bold text-gray-900">{{ health.goroutines }}</p>
      </div>

      <div class="bg-white rounded-lg shadow p-6">
        <h3 class="text-sm font-medium text-gray-500 mb-2">Go Version</h3>
        <p class="text-2xl font-bold text-gray-900">{{ health.go_version }}</p>
      </div>
    </div>
  </div>
</template>
