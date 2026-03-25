<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

export interface Connection {
  id: string
  type: 'websocket' | 'http' | 'sse'
  clientIp: string
  userAgent?: string
  connectedAt: Date
  lastActivity: Date
  requestCount: number
}

const props = withDefaults(
  defineProps<{
    connections?: Connection[]
    loading?: boolean
  }>(),
  {
    connections: () => [],
    loading: false,
  }
)

const sortBy = ref<'connectedAt' | 'lastActivity' | 'requestCount'>('connectedAt')
const sortOrder = ref<'asc' | 'desc'>('desc')
const selectedIp = ref<string>('all')

// Get unique IPs from connections
const uniqueIps = computed(() => {
  const ips = new Set(props.connections.map((c) => c.clientIp))
  return Array.from(ips).sort()
})

// Filter connections by selected IP
const filteredConnections = computed(() => {
  if (selectedIp.value === 'all') {
    return props.connections
  }
  return props.connections.filter((c) => c.clientIp === selectedIp.value)
})

const sortedConnections = computed(() => {
  return [...filteredConnections.value].sort((a, b) => {
    let comparison = 0
    switch (sortBy.value) {
      case 'connectedAt':
        comparison = new Date(a.connectedAt).getTime() - new Date(b.connectedAt).getTime()
        break
      case 'lastActivity':
        comparison = new Date(a.lastActivity).getTime() - new Date(b.lastActivity).getTime()
        break
      case 'requestCount':
        comparison = a.requestCount - b.requestCount
        break
    }
    return sortOrder.value === 'asc' ? comparison : -comparison
  })
})

function getTypeColor(type: Connection['type']): string {
  switch (type) {
    case 'websocket':
      return 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400'
    case 'sse':
      return 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400'
    case 'http':
      return 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400'
  }
}

function formatDuration(date: Date): string {
  const now = new Date()
  const diff = now.getTime() - new Date(date).getTime()
  const seconds = Math.floor(diff / 1000)
  const minutes = Math.floor(seconds / 60)
  const hours = Math.floor(minutes / 60)

  if (hours > 0) return `${hours}h ${minutes % 60}m`
  if (minutes > 0) return `${minutes}m ${seconds % 60}s`
  return `${seconds}s`
}

function toggleSort(field: typeof sortBy.value) {
  if (sortBy.value === field) {
    sortOrder.value = sortOrder.value === 'asc' ? 'desc' : 'asc'
  } else {
    sortBy.value = field
    sortOrder.value = 'desc'
  }
}
</script>

<template>
  <div class="bg-white dark:bg-gray-700 rounded-lg shadow">
    <div
      class="px-6 py-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between"
    >
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
        {{ t('common.activeConnectionsTitle') }}
      </h2>
      <div class="flex items-center gap-4">
        <!-- IP Filter -->
        <div class="flex items-center gap-2">
          <label class="text-sm text-gray-500 dark:text-gray-400"
            >{{ t('common.filterByIp') }}:</label
          >
          <select
            v-model="selectedIp"
            class="text-sm bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded px-2 py-1 border border-gray-200 dark:border-gray-600 focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400"
          >
            <option value="all">{{ t('common.allIps') }}</option>
            <option v-for="ip in uniqueIps" :key="ip" :value="ip">{{ ip }}</option>
          </select>
        </div>
        <span class="text-sm text-gray-500 dark:text-gray-400">
          {{ t('common.activeCount', { count: filteredConnections.length }) }}
        </span>
      </div>
    </div>

    <div v-if="loading" class="p-6 text-center">
      <div
        class="animate-spin h-8 w-8 border-4 border-gray-900 dark:border-white border-t-transparent rounded-full mx-auto"
      ></div>
      <p class="mt-2 text-gray-500 dark:text-gray-400">{{ t('common.loadingConnections') }}</p>
    </div>

    <div
      v-else-if="connections.length === 0"
      class="p-6 text-center text-gray-500 dark:text-gray-400"
    >
      {{ t('common.noActiveConnections') }}
    </div>

    <div v-else class="overflow-x-auto">
      <table class="w-full">
        <thead class="bg-gray-50 dark:bg-gray-700">
          <tr>
            <th
              class="active-connections-heading px-6 py-3 text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider"
            >
              {{ t('common.type') }}
            </th>
            <th
              class="active-connections-heading px-6 py-3 text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider"
            >
              {{ t('common.clientIp') }}
            </th>
            <th
              class="active-connections-heading px-6 py-3 text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider cursor-pointer hover:text-gray-700 dark:hover:text-gray-100"
              @click="toggleSort('connectedAt')"
            >
              {{ t('common.connected') }}
              <span v-if="sortBy === 'connectedAt'">{{ sortOrder === 'asc' ? '↑' : '↓' }}</span>
            </th>
            <th
              class="active-connections-heading px-6 py-3 text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider cursor-pointer hover:text-gray-700 dark:hover:text-gray-100"
              @click="toggleSort('lastActivity')"
            >
              {{ t('common.lastActivity') }}
              <span v-if="sortBy === 'lastActivity'">{{ sortOrder === 'asc' ? '↑' : '↓' }}</span>
            </th>
            <th
              class="active-connections-heading px-6 py-3 text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider cursor-pointer hover:text-gray-700 dark:hover:text-gray-100"
              @click="toggleSort('requestCount')"
            >
              {{ t('common.requests') }}
              <span v-if="sortBy === 'requestCount'">{{ sortOrder === 'asc' ? '↑' : '↓' }}</span>
            </th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
          <tr
            v-for="conn in sortedConnections"
            :key="conn.id"
            class="hover:bg-gray-50 dark:hover:bg-gray-700/50"
          >
            <td class="px-6 py-4 whitespace-nowrap">
              <span
                class="px-2 py-1 text-xs font-medium rounded-full"
                :class="getTypeColor(conn.type)"
              >
                {{ conn.type }}
              </span>
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900 dark:text-gray-100">
              {{ conn.clientIp }}
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
              {{ formatDuration(conn.connectedAt) }} {{ t('common.ago') }}
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
              {{ formatDuration(conn.lastActivity) }} {{ t('common.ago') }}
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900 dark:text-gray-100">
              {{ conn.requestCount }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<style scoped>
.active-connections-heading {
  text-align: start;
}
</style>
