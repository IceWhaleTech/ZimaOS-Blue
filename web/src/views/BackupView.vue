<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { backupApi } from '@/api'
import type { BackupInfo } from '@/api'

const { t } = useI18n()

const loading = ref(false)
const backups = ref<BackupInfo[]>([])
const creating = ref(false)
const restoring = ref<string | null>(null)

onMounted(async () => {
  await loadBackups()
})

async function loadBackups() {
  try {
    loading.value = true
    const response = await backupApi.list()
    backups.value = response.data
  } catch {
    backups.value = []
  } finally {
    loading.value = false
  }
}

async function createBackup() {
  try {
    creating.value = true
    await backupApi.create()
    await loadBackups()
  } catch {
    // Handle error
  } finally {
    creating.value = false
  }
}

async function restoreBackup(backup: BackupInfo) {
  if (!confirm(t('backup.confirmRestore', { date: formatDate(backup.created_at) }))) return

  try {
    restoring.value = backup.id
    await backupApi.restore(backup.id)
    alert(t('backup.restoreSuccess'))
  } catch {
    alert(t('backup.restoreFailed'))
  } finally {
    restoring.value = null
  }
}

async function deleteBackup(backup: BackupInfo) {
  if (!confirm(t('backup.confirmDelete'))) return

  try {
    await backupApi.delete(backup.id)
    await loadBackups()
  } catch {
    // Handle error
  }
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleString()
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  if (bytes < 1024 * 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
  return (bytes / (1024 * 1024 * 1024)).toFixed(1) + ' GB'
}
</script>

<template>
  <div class="backup-view p-4 sm:p-6 max-w-4xl mx-auto">
    <div class="flex items-center justify-between mb-6">
      <h1 class="text-xl sm:text-2xl font-bold text-gray-900 dark:text-white">{{ t('backup.title') }}</h1>
      <button
        :disabled="creating"
        class="px-4 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg text-sm transition-colors disabled:opacity-50 flex items-center gap-2"
        @click="createBackup"
      >
        <svg v-if="creating" class="animate-spin h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
        </svg>
        <svg v-else xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
        </svg>
        {{ creating ? t('backup.creating') : t('backup.create') }}
      </button>
    </div>

    <div class="glass-card p-4 mb-6">
      <p class="text-sm text-gray-500 dark:text-slate-400">
        {{ t('backup.description') }}
      </p>
    </div>

    <div v-if="loading" class="text-center py-8 text-gray-500 dark:text-slate-400">
      {{ t('common.loading') }}
    </div>

    <div v-else-if="backups.length === 0" class="text-center py-8">
      <svg xmlns="http://www.w3.org/2000/svg" class="h-12 w-12 mx-auto text-gray-400 dark:text-slate-500 mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 8h14M5 8a2 2 0 110-4h14a2 2 0 110 4M5 8v10a2 2 0 002 2h10a2 2 0 002-2V8m-9 4h4" />
      </svg>
      <p class="text-gray-500 dark:text-slate-400">{{ t('backup.noBackups') }}</p>
    </div>

    <div v-else class="space-y-3">
      <div v-for="backup in backups" :key="backup.id" class="glass-card p-4">
        <div class="flex items-center justify-between">
          <div>
            <div class="text-gray-900 dark:text-white font-medium mb-1">
              {{ formatDate(backup.created_at) }}
            </div>
            <div class="flex items-center gap-4 text-sm text-gray-500 dark:text-slate-400">
              <span>{{ formatSize(backup.size_bytes) }}</span>
              <span class="px-2 py-0.5 bg-gray-100 dark:bg-slate-700 rounded text-xs">{{ backup.type }}</span>
            </div>
          </div>
          <div class="flex items-center gap-2">
            <button
              :disabled="restoring === backup.id"
              :title="t('backup.restore')"
              class="p-2 text-blue-600 dark:text-blue-400 hover:bg-blue-50 dark:hover:bg-blue-900/20 rounded-lg transition-colors disabled:opacity-50"
              @click="restoreBackup(backup)"
            >
              <svg v-if="restoring === backup.id" class="animate-spin h-5 w-5" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
              </svg>
              <svg v-else xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
              </svg>
            </button>
            <button
              :title="t('common.delete')"
              class="p-2 text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg transition-colors"
              @click="deleteBackup(backup)"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
              </svg>
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
