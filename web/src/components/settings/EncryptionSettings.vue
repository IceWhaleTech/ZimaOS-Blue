<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { encryptionApi, type EncryptionStatus } from '@/api/encryption'

const emit = defineEmits<{ 'status-change': [msg: string] }>()
const { t } = useI18n()

const loading = ref(true)
const status = ref<EncryptionStatus | null>(null)
const toggling = ref(false)
const showPassphrase = ref(false)
const passphrase = ref('')
const pollTimer = ref<ReturnType<typeof setInterval> | null>(null)

const migrationPercent = computed(() => {
  if (!status.value || !status.value.migration_total) return 0
  return Math.round((status.value.migration_progress / status.value.migration_total) * 100)
})

async function fetchStatus() {
  try {
    const res = await encryptionApi.getStatus()
    status.value = res.data
    // Auto-poll during migration
    if (res.data.migrating && !pollTimer.value) {
      pollTimer.value = setInterval(fetchStatus, 2000)
    } else if (!res.data.migrating && pollTimer.value) {
      clearInterval(pollTimer.value)
      pollTimer.value = null
    }
  } catch {
    status.value = null
  }
}

async function toggleEncryption() {
  if (toggling.value || !status.value) return
  if (status.value.enabled) {
    // Disable
    toggling.value = true
    try {
      await encryptionApi.disable()
      emit('status-change', t('encryption.disabled'))
      await fetchStatus()
    } finally {
      toggling.value = false
    }
  } else {
    // Show passphrase input
    showPassphrase.value = true
  }
}

async function enableWithPassphrase() {
  if (!passphrase.value.trim()) return
  toggling.value = true
  try {
    await encryptionApi.enable(passphrase.value)
    passphrase.value = ''
    showPassphrase.value = false
    emit('status-change', t('encryption.enabled'))
    await fetchStatus()
  } finally {
    toggling.value = false
  }
}

function cancelPassphrase() {
  showPassphrase.value = false
  passphrase.value = ''
}

onMounted(async () => {
  await fetchStatus()
  loading.value = false
})
</script>

<template>
  <div class="glass-card p-4">
    <div class="flex items-center justify-between mb-3">
      <div>
        <h3 class="text-sm font-semibold text-gray-700 dark:text-white">
          {{ t('encryption.title') }}
        </h3>
        <p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
          {{ t('encryption.desc') }}
        </p>
      </div>
      <button
        v-if="status && !status.migrating"
        class="relative w-10 h-5 rounded-full transition-colors"
        :class="status.enabled ? 'bg-green-500' : 'bg-gray-300 dark:bg-gray-600'"
        :disabled="toggling"
        @click="toggleEncryption"
      >
        <span
          class="absolute top-0.5 left-0.5 w-4 h-4 bg-white rounded-full transition-transform"
          :class="status.enabled ? 'translate-x-5' : ''"
        />
      </button>
    </div>

    <!-- Passphrase input -->
    <div v-if="showPassphrase" class="mb-3 flex gap-2">
      <input
        v-model="passphrase"
        type="password"
        :placeholder="t('encryption.passphrasePlaceholder')"
        class="flex-1 px-3 py-1.5 text-sm bg-gray-50 dark:bg-gray-700/50 border border-gray-200 dark:border-gray-600 rounded-lg text-gray-700 dark:text-gray-200"
        @keyup.enter="enableWithPassphrase"
      />
      <button
        class="px-3 py-1.5 text-xs bg-green-600 hover:bg-green-700 text-white rounded-lg"
        :disabled="toggling || !passphrase.trim()"
        @click="enableWithPassphrase"
      >
        {{ t('encryption.confirm') }}
      </button>
      <button
        class="px-3 py-1.5 text-xs bg-gray-200 dark:bg-gray-600 text-gray-700 dark:text-gray-200 rounded-lg"
        @click="cancelPassphrase"
      >
        {{ t('common.cancel') }}
      </button>
    </div>

    <!-- Migration progress -->
    <div v-if="status?.migrating" class="mb-3">
      <div class="flex justify-between text-xs text-gray-500 dark:text-gray-400 mb-1">
        <span>{{ t('encryption.migrating') }}</span>
        <span>{{ migrationPercent }}%</span>
      </div>
      <div class="w-full h-1.5 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
        <div
          class="h-full bg-blue-500 rounded-full transition-all"
          :style="{ width: migrationPercent + '%' }"
        />
      </div>
    </div>

    <!-- Stats row -->
    <div v-if="status && !loading" class="grid grid-cols-3 gap-3 text-center">
      <div class="bg-gray-50 dark:bg-gray-700/30 rounded-lg p-2">
        <div class="text-lg font-bold text-gray-700 dark:text-white">
          {{ status.encrypted_count }}
        </div>
        <div class="text-xs text-gray-500 dark:text-gray-400">
          {{ t('encryption.encrypted') }}
        </div>
      </div>
      <div class="bg-gray-50 dark:bg-gray-700/30 rounded-lg p-2">
        <div class="text-lg font-bold text-gray-700 dark:text-white">
          {{ status.plaintext_count }}
        </div>
        <div class="text-xs text-gray-500 dark:text-gray-400">
          {{ t('encryption.plaintext') }}
        </div>
      </div>
      <div class="bg-gray-50 dark:bg-gray-700/30 rounded-lg p-2">
        <div class="text-lg font-bold text-gray-700 dark:text-white">
          {{ status.enabled ? 'AES-256' : 'OFF' }}
        </div>
        <div class="text-xs text-gray-500 dark:text-gray-400">
          {{ t('encryption.algorithm') }}
        </div>
      </div>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="text-center text-sm text-gray-400 py-4">
      {{ t('common.loading') }}
    </div>
  </div>
</template>
