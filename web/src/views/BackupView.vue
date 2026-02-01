<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { backupApi } from '@/api'
import type { BackupInfo, BackupProgress } from '@/api'

const { t } = useI18n()

const loading = ref(false)
const backups = ref<BackupInfo[]>([])
const creating = ref(false)
const restoring = ref<string | null>(null)
const progress = ref<BackupProgress | null>(null)
let progressInterval: ReturnType<typeof setInterval> | null = null

onMounted(async () => {
  await loadBackups()
  await checkProgress()
})

onUnmounted(() => {
  if (progressInterval) {
    clearInterval(progressInterval)
  }
})

async function checkProgress() {
  try {
    const res = await backupApi.getProgress()
    progress.value = res.data
    if (res.data.in_progress) {
      creating.value = res.data.operation === 'backup'
      restoring.value = res.data.operation === 'restore' ? 'in_progress' : null
      if (!progressInterval) {
        progressInterval = setInterval(checkProgress, 1000)
      }
    } else {
      creating.value = false
      restoring.value = null
      if (progressInterval) {
        clearInterval(progressInterval)
        progressInterval = null
      }
      progress.value = null
    }
  } catch {
    progress.value = null
  }
}

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
    progressInterval = setInterval(checkProgress, 1000)
    await backupApi.create()
    await loadBackups()
  } catch {
    // Handle error
  } finally {
    creating.value = false
    if (progressInterval) {
      clearInterval(progressInterval)
      progressInterval = null
    }
    progress.value = null
  }
}

async function restoreBackup(id: string) {
  const backup = backups.value.find((b) => b.id === id)
  const dateStr = backup ? formatBackupDate(backup.created_at) : id
  if (!confirm(t('backup.confirmRestore', { date: dateStr }))) return
  try {
    restoring.value = id
    // Start progress polling for restore
    progressInterval = setInterval(checkProgress, 1000)
    await backupApi.restore(id)
    // Restore completed - reload backups and show success
    await loadBackups()
    alert(t('backup.restoreSuccess'))
  } catch {
    alert(t('backup.restoreFailed'))
  } finally {
    restoring.value = null
    if (progressInterval) {
      clearInterval(progressInterval)
      progressInterval = null
    }
    progress.value = null
  }
}

function formatBackupDate(dateStr: string): string {
  const d = new Date(dateStr)
  const year = d.getFullYear()
  const month = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  const hours = String(d.getHours()).padStart(2, '0')
  const minutes = String(d.getMinutes()).padStart(2, '0')
  return `${year}-${month}-${day} ${hours}:${minutes}`
}

async function deleteBackup(id: string) {
  if (!confirm(t('backup.confirmDelete'))) return
  try {
    await backupApi.delete(id)
    await loadBackups()
  } catch {
    // Handle error
  }
}

function onBackupCreate(_type: string, _name: string) {
  createBackup()
}

function onBackupDownload(_id: string) {
  // Download not implemented in API yet
}
</script>

<template>
  <div class="backup-view p-4 sm:p-6 max-w-4xl mx-auto">
    <div class="glass-card p-4 mb-6">
      <p class="text-sm text-gray-500 dark:text-slate-400">
        {{ t('backup.description') }}
      </p>
    </div>
    <BackupManager
      :backups="backups"
      :loading="loading"
      :restoring="restoring"
      :creating="creating"
      :progress="progress"
      @create="onBackupCreate"
      @restore="restoreBackup"
      @delete="deleteBackup"
      @download="onBackupDownload"
    />
  </div>
</template>
