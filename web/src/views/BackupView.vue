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

async function restoreBackup(id: string) {
  const backup = backups.value.find((b) => b.id === id)
  const dateStr = backup ? new Date(backup.created_at).toLocaleString() : id
  if (!confirm(t('backup.confirmRestore', { date: dateStr }))) return
  try {
    restoring.value = id
    await backupApi.restore(id)
    alert(t('backup.restoreSuccess'))
  } catch {
    alert(t('backup.restoreFailed'))
  } finally {
    restoring.value = null
  }
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
      @create="onBackupCreate"
      @restore="restoreBackup"
      @delete="deleteBackup"
      @download="onBackupDownload"
    />
  </div>
</template>
