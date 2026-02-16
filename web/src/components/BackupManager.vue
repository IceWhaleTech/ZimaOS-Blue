<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { BackupInfo, BackupProgress } from '@/api'

const { t } = useI18n()

type BackupType = 'full' | 'config' | 'data'
type BackupStatus = 'completed' | 'in_progress' | 'failed'

interface BackupDisplay {
  id: string
  name: string
  createdAt: Date
  size: number
  type: BackupType
  status: BackupStatus
  description?: string
}

const props = withDefaults(
  defineProps<{
    backups?: BackupInfo[]
    loading?: boolean
    restoring?: boolean
    creating?: boolean
    progress?: BackupProgress | null
  }>(),
  {
    backups: () => [],
    loading: false,
    restoring: false,
    creating: false,
    progress: null,
  }
)

const emit = defineEmits<{
  create: [type: BackupType, name: string]
  restore: [id: string]
  delete: [id: string]
  download: [id: string]
}>()

const showCreateModal = ref(false)
const newBackupType = ref<BackupType>('full')
const newBackupName = ref('')

function mapToDisplay(b: BackupInfo): BackupDisplay {
  const type = (b.type === 'full' || b.type === 'config' || b.type === 'data' ? b.type : 'full') as BackupType
  return {
    id: b.id,
    name: b.id,
    createdAt: new Date(b.created_at),
    size: b.size_bytes ?? 0,
    type,
    status: 'completed' as BackupStatus,
    description: undefined,
  }
}

const displayBackups = computed(() => (props.backups ?? []).map(mapToDisplay))

const sortedBackups = computed(() =>
  [...displayBackups.value].sort((a, b) => b.createdAt.getTime() - a.createdAt.getTime())
)

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`
  return `${(bytes / 1024 / 1024 / 1024).toFixed(1)} GB`
}

function formatBackupName(backup: BackupDisplay): string {
  // Format as date string instead of showing UUID
  return formatDate(backup.createdAt)
}

function formatDate(date: Date): string {
  const d = new Date(date)
  const year = d.getFullYear()
  const month = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  const hours = String(d.getHours()).padStart(2, '0')
  const minutes = String(d.getMinutes()).padStart(2, '0')
  return `${year}-${month}-${day} ${hours}:${minutes}`
}

function getTypeColor(type: BackupType): string {
  switch (type) {
    case 'full':
      return 'bg-purple-100 text-purple-800 dark:bg-purple-900/30 dark:text-purple-400'
    case 'config':
      return 'bg-gray-700 dark:bg-gray-500/30 text-gray-900 dark:text-white'
    case 'data':
      return 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400'
  }
}

function getStatusColor(status: BackupStatus): string {
  switch (status) {
    case 'completed':
      return 'text-green-600 dark:text-green-400'
    case 'in_progress':
      return 'text-yellow-600 dark:text-yellow-400'
    case 'failed':
      return 'text-red-600 dark:text-red-400'
  }
}

function getStatusIcon(status: BackupStatus): string {
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

function confirmRestore(backup: BackupDisplay) {
  if (confirm(t('backup.confirmRestore', { date: formatDate(backup.createdAt) }))) {
    emit('restore', backup.id)
  }
}

function confirmDelete(backup: BackupDisplay) {
  if (confirm(t('backup.confirmDelete'))) {
    emit('delete', backup.id)
  }
}
</script>

<template>
  <div class="bg-white dark:bg-gray-700/30 rounded-lg shadow">
    <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('backup.title') }}</h2>
      <button
        class="px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white text-sm font-medium rounded-lg transition-colors"
        @click="showCreateModal = true"
      >
        {{ t('backup.create') }}
      </button>
    </div>

    <!-- Restore in progress banner -->
    <div v-if="restoring" class="px-6 py-3 bg-yellow-50 dark:bg-yellow-900/20 border-b border-yellow-200 dark:border-yellow-800">
      <div class="flex items-center gap-3 mb-2">
        <div class="animate-spin h-5 w-5 border-2 border-yellow-600 border-t-transparent rounded-full"></div>
        <span class="text-yellow-800 dark:text-yellow-200">{{ t('backup.restoringWarning') }}</span>
      </div>
      <div v-if="progress && progress.operation === 'restore'">
        <div class="flex items-center justify-between text-sm text-yellow-700 dark:text-yellow-300 mb-1">
          <span>{{ progress.current_file || t('backup.processing') }}</span>
          <span>{{ progress.progress }}%</span>
        </div>
        <div class="h-2 bg-yellow-200 dark:bg-yellow-800 rounded-full overflow-hidden">
          <div class="h-full bg-yellow-500 transition-all duration-300" :style="{ width: `${progress.progress}%` }"></div>
        </div>
        <div class="flex items-center justify-between text-xs text-yellow-600 dark:text-yellow-400 mt-1">
          <span>{{ progress.files_processed }} / {{ progress.total_files }} {{ t('backup.files') }}</span>
          <span>{{ formatSize(progress.bytes_processed) }} / {{ formatSize(progress.total_bytes) }}</span>
        </div>
      </div>
    </div>

    <!-- Creating backup progress banner -->
    <div v-if="creating && progress" class="px-6 py-3 bg-gray-700 dark:bg-gray-500/20 border-b border-gray-900 dark:border-white dark:border-gray-900 dark:border-white">
      <div class="flex items-center gap-3 mb-2">
        <div class="animate-spin h-5 w-5 border-2 border-gray-900 dark:border-white border-t-transparent rounded-full"></div>
        <span class="text-gray-900 dark:text-white dark:text-white">{{ t('backup.creatingBackup') }}</span>
      </div>
      <div class="flex items-center justify-between text-sm text-gray-900 dark:text-white dark:text-white mb-1">
        <span>{{ progress.current_file || t('backup.processing') }}</span>
        <span>{{ progress.progress }}%</span>
      </div>
      <div class="h-2 bg-gray-700 dark:bg-gray-500 dark:bg-gray-500 rounded-full overflow-hidden">
        <div class="h-full bg-gray-700 dark:bg-gray-500 transition-all duration-300" :style="{ width: `${progress.progress}%` }"></div>
      </div>
      <div class="flex items-center justify-between text-xs text-gray-900 dark:text-white dark:text-white mt-1">
        <span>{{ progress.files_processed }} / {{ progress.total_files }} {{ t('backup.files') }}</span>
        <span>{{ formatSize(progress.bytes_processed) }} / {{ formatSize(progress.total_bytes) }}</span>
      </div>
    </div>

    <div v-if="loading" class="p-6 text-center">
      <div class="animate-spin h-8 w-8 border-4 border-gray-900 dark:border-white border-t-transparent rounded-full mx-auto"></div>
      <p class="mt-2 text-gray-500 dark:text-gray-400">{{ t('backup.loading') }}</p>
    </div>

    <div v-else-if="displayBackups.length === 0" class="p-6 text-center">
      <svg xmlns="http://www.w3.org/2000/svg" class="h-12 w-12 mx-auto text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4" />
      </svg>
      <p class="mt-2 text-gray-500 dark:text-gray-400">{{ t('backup.noBackups') }}</p>
      <p class="text-sm text-gray-400 dark:text-gray-500">{{ t('backup.noBackupsHint') }}</p>
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
              <span class="font-medium text-gray-900 dark:text-white">{{ formatBackupName(backup) }}</span>
              <span
                class="px-2 py-0.5 text-xs font-medium rounded-full"
                :class="getTypeColor(backup.type)"
              >
                {{ t('backup.types.' + backup.type) }}
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
            <div class="mt-1 text-sm text-gray-500 dark:text-gray-400">
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
              class="p-2 text-gray-900 dark:text-white hover:text-gray-900 dark:text-white dark:text-white dark:hover:text-gray-900 dark:text-white hover:bg-gray-700 dark:bg-gray-500 dark:hover:bg-gray-600 rounded-lg transition-colors"
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
        <div class="bg-white dark:bg-gray-700/30 rounded-lg shadow-xl max-w-md w-full mx-4">
          <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('backup.create') }}</h3>
          </div>

          <div class="p-6 space-y-4">
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                {{ t('backup.nameOptional') }}
              </label>
              <input
                v-model="newBackupName"
                type="text"
                :placeholder="`Backup ${new Date().toISOString().split('T')[0]}`"
                class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400 focus:border-transparent"
              />
            </div>

            <div class="p-3 border border-gray-200 dark:border-gray-600 rounded-lg bg-gray-50 dark:bg-gray-700/50">
              <div class="font-medium text-gray-900 dark:text-white">{{ t('backup.fullBackup') }}</div>
              <p class="text-sm text-gray-500 dark:text-gray-400 mt-0.5">{{ t('backup.fullBackupDesc') }}</p>
            </div>
          </div>

          <div class="px-6 py-4 border-t border-gray-200 dark:border-gray-700 flex justify-end gap-3">
            <button
              class="px-4 py-2 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
              @click="closeModal"
            >
              {{ t('common.cancel') }}
            </button>
            <button
              class="px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg transition-colors"
              @click="handleCreate"
            >
              {{ t('backup.create') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
