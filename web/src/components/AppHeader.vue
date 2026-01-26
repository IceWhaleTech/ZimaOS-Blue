<script setup lang="ts">
import { useSystemStore } from '@/stores/system'
import { storeToRefs } from 'pinia'

const systemStore = useSystemStore()
const { health } = storeToRefs(systemStore)
</script>

<template>
  <header class="bg-white border-b border-gray-200 px-6 py-4">
    <div class="flex items-center justify-between">
      <div class="flex items-center space-x-4">
        <h1 class="text-xl font-bold text-gray-900">ZimaOS Echo</h1>
        <span
          v-if="health"
          class="px-2 py-1 text-xs font-medium rounded-full"
          :class="health.status === 'ok' ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'"
        >
          {{ health.status }}
        </span>
      </div>
      <div class="flex items-center space-x-4">
        <span v-if="health" class="text-sm text-gray-500">
          v{{ health.version }}
        </span>
      </div>
    </div>
  </header>
</template>
