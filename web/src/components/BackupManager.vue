<script setup lang="ts">
import { ref, computed } from 'vue'

export interface Backup {
  id: string
  name: string
  createdAt: Date
  size: number
  type: 'full' | 'config' | 'data'
  status: 'completed' | 'in_progress' | 'failed'
  description?: string
}

const props = withDefaults(
  defineProps<{
    backups?: Backup[]
    loading?: boolean
    restoring?: boolean
  }>(),
  {
    backups: () => [],
    loading: false,
    restoring: false,
  }
)

const emit = defineEmits<{
  create: [type: Backup['type'], name: string]
  restore: [id: string]
  delete: [id: string]
  download: [id: string]
}>()

const showCreateModal = ref(false)
const newBackupType = ref<Backup['type']>('full')
const newBackupName = ref('')

const sortedBackups = computed(() => {
  return [...props.backups].sort((a, b) =>
    new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime()
  )
})

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`
  return `${(bytes / 1024 / 1024 / 1024).toFixed(1)} GB`
}

function formatDate(date: Date): string {
  return new Date(date).toLocaleString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function getTypeColor(type: Backup['type']): string {
  switch (type) {
    case 'full':
      return 'bg-purple-100 text-purple-800 dark:bg-purple-900/30 dark:text-purple-400'
    case 'config':
      return 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400'
    case 'data':
      return 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400'
  }
}

function getStatusColor(status: Backup['status']): string {
  switch (status) {
    case 'completed':
      return 'text-green-600 dark:text-green-400'
    case 'in_progress':
      return 'text-yellow-600 dark:text-yellow-400'
    case 'failed':
      return 'text-red-600 dark:text-red-400'
  }
}

function getStatusIcon(status: Backup['status']): string {
  switch (status) {
    case 'completed':
      return 'M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z'
    case 'in_progress':
      return 'M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z'
    case 'failed':
      return 'M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z'
  }
}

function handleCreate() {
  const name = newBackupName.value.trim() || `Backup ${new Date().toISOString().split('T')[0]}`
  emit('create', newBackupType.value, name)
  closeModal()
}

function closeModal() {
  showCreateModal.value = false
  newBackupType.value = 'full'
  newBackupName.value = ''
}

function confirmRestore(backup: Backup) {
  if (confirm(`Are you sure you want to restore from "${backup.name}"? This will overwrite current data.`)) {
    emit('restore', backup.id)
  }
}

function confirmDelete(backup: Backup) {
  if (confirm(`Are you sure you want to delete "${backup.name}"? This action cannot be undone.`)) {
    emit('delete', backup.id)
  }
}
</script>

<template>
  <div class="bg-white dark:bg-gray-800 rounded-lg shadow">
    <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white">Backup & Restore</h2>
      <button
        class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-lg transition-colors"
        @click="showCreateModal = true"
      >
        Create Backup
      </button>
    </div>

    <!-- Restore in progress banner -->
    <div v-if="restoring" class="px-6 py-3 bg-yellow-50 dark:bg-yellow-900/20 border-b border-yellow-200 dark:border-yellow-800">
      <div class="flex items-center gap-3">
        <div class="animate-spin h-5 w-5 border-2 border-yellow-600 border-t-transparent rounded-full"></div>
        <span class="text-yellow-800 dark:text-yellow-200">Restore in progress... Please do not close this page.</span>
      </div>
    </div>

    <div v-if="loading" class="p-6 text-center">
      <div class="animate-spin h-8 w-8 border-4 border-blue-500 border-t-transparent rounded-full mx-auto"></div>
      <p class="mt-2 text-gray-500 dark:text-gray-400">Loading backups...</p>
    </div>

    <div v-else-if="backups.length === 0" class="p-6 text-center">
      <svg xmlns="http://www.w3.org/2000/svg" class="h-12 w-12 mx-auto text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4" />
      </svg>
      <p class="mt-2 text-gray-500 dark:text-gray-400">No backups yet</p>
      <p class="text-sm text-gray-400 dark:text-gray-500">Create a backup to protect your data</p>
    </div>

    <div v-else class="divide-y divide-gray-200 dark:divide-gray-700">
      <div
        v-for="backup in sortedBackups"
        :key="backup.id"
        class="px-6 py-4"
      >
        <div class="flex items-start justify-between">
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-2">
              <span class="font-medium text-gray-900 dark:text-white">{{ backup.name }}</span>
              <span
                class="px-2 py-0.5 text-xs font-medium rounded-full"
                :class="getTypeColor(backup.type)"
              >
                {{ backup.type }}
              </span>
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="h-5 w-5"
                :class="getStatusColor(backup.status)"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getStatusIcon(backup.status)" />
              </svg>
            </div>
            <div class="mt-1 flex items-center gap-4 text-sm text-gray-500 dark:text-gray-400">
              <span>{{ formatDate(backup.createdAt) }}</span>
              <span>{{ formatSize(backup.size) }}</span>
            </div>
            <p v-if="backup.description" class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ backup.description }}
            </p>
          </div>

          <div class="flex items-center gap-2 ml-4">
            <button
              class="p-2 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
              title="Download"
              :disabled="backup.status !== 'completed'"
              @click="emit('download', backup.id)"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
              </svg>
            </button>
            <button
              class="p-2 text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300 hover:bg-blue-50 dark:hover:bg-blue-900/20 rounded-lg transition-colors"
              title="Restore"
              :disabled="backup.status !== 'completed' || restoring"
              @click="confirmRestore(backup)"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
              </svg>
            </button>
            <button
              class="p-2 text-red-600 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg transition-colors"
              title="Delete"
              @click="confirmDelete(backup)"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
              </svg>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Create Modal -->
    <Teleport to="body">
      <div
        v-if="showCreateModal"
        class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
        @click.self="closeModal"
      >
        <div class="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-md w-full mx-4">
          <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">Create Backup</h3>
          </div>

          <div class="p-6 space-y-4">
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Backup Name (optional)
              </label>
              <input
                v-model="newBackupName"
                type="text"
                :placeholder="`Backup ${new Date().toISOString().split('T')[0]}`"
                class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent"
              />
            </div>

            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                Backup Type
              </label>
              <div class="space-y-2">
                <label class="flex items-start gap-3 p-3 border border-gray-200 dark:border-gray-600 rounded-lg cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-700">
                  <input
                    v-model="newBackupType"
                    type="radio"
                    value="full"
                    class="mt-0.5"
                  />
                  <div>
                    <span class="font-medium text-gray-900 dark:text-white">Full Backup</span>
                    <p class="text-sm text-gray-500 dark:text-gray-400">Includes all data, configuration, and conversation history</p>
                  </div>
                </label>
                <label class="flex items-start gap-3 p-3 border border-gray-200 dark:border-gray-600 rounded-lg cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-700">
                  <input
                    v-model="newBackupType"
                    type="radio"
                    value="config"
                    class="mt-0.5"
                  />
                  <div>
                    <span class="font-medium text-gray-900 dark:text-white">Configuration Only</span>
                    <p class="text-sm text-gray-500 dark:text-gray-400">Includes settings and provider configuration</p>
                  </div>
                </label>
                <label class="flex items-start gap-3 p-3 border border-gray-200 dark:border-gray-600 rounded-lg cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-700">
                  <input
                    v-model="newBackupType"
                    type="radio"
                    value="data"
                    class="mt-0.5"
                  />
                  <div>
                    <span class="font-medium text-gray-900 dark:text-white">Data Only</span>
                    <p class="text-sm text-gray-500 dark:text-gray-400">Includes conversation history and user data</p>
                  </div>
                </label>
              </div>
            </div>
          </div>

          <div class="px-6 py-4 border-t border-gray-200 dark:border-gray-700 flex justify-end gap-3">
            <button
              class="px-4 py-2 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
              @click="closeModal"
            >
              Cancel
            </button>
            <button
              class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors"
              @click="handleCreate"
            >
              Create Backup
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
