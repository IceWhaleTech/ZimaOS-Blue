<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { mfaApi } from '@/api/mfa'
import type { MFAStatus, MFASetupResponse } from '@/api/mfa'

const { t } = useI18n()

const emit = defineEmits<{
  (e: 'status-change', message: string): void
}>()

const loading = ref(false)
const mfaStatus = ref<MFAStatus | null>(null)
const setupData = ref<MFASetupResponse | null>(null)
const verificationCode = ref('')
const recoveryCodes = ref<string[]>([])
const showRecoveryCodes = ref(false)
const disablePassword = ref('')
const showDisableModal = ref(false)
const copiedCodes = ref(false)

onMounted(async () => {
  await loadMFAStatus()
})

async function loadMFAStatus() {
  try {
    loading.value = true
    const response = await mfaApi.getStatus()
    mfaStatus.value = response.data
  } catch {
    mfaStatus.value = { enabled: false, recovery_codes_remaining: 0 }
  } finally {
    loading.value = false
  }
}

async function startSetup() {
  try {
    loading.value = true
    const response = await mfaApi.setup(true)
    setupData.value = response.data
  } catch (_e) {
    emit('status-change', t('mfa.setupFailed'))
  } finally {
    loading.value = false
  }
}

function cancelSetup() {
  setupData.value = null
  verificationCode.value = ''
}

async function verifySetup() {
  if (!setupData.value || !verificationCode.value) return

  try {
    loading.value = true
    const response = await mfaApi.verify(verificationCode.value, setupData.value.secret)
    if (response.data.enabled) {
      recoveryCodes.value = response.data.recovery_codes || []
      showRecoveryCodes.value = true
      setupData.value = null
      verificationCode.value = ''
      await loadMFAStatus()
      emit('status-change', t('mfa.enabledSuccessfully'))
    }
  } catch {
    emit('status-change', t('mfa.invalidCode'))
  } finally {
    loading.value = false
  }
}

async function disableMFA() {
  if (!disablePassword.value) return

  try {
    loading.value = true
    await mfaApi.disable(disablePassword.value)
    showDisableModal.value = false
    disablePassword.value = ''
    await loadMFAStatus()
    emit('status-change', t('mfa.disabledSuccessfully'))
  } catch {
    emit('status-change', t('mfa.disableFailed'))
  } finally {
    loading.value = false
  }
}

async function regenerateCodes() {
  try {
    loading.value = true
    const response = await mfaApi.regenerateRecoveryCodes()
    recoveryCodes.value = response.data.codes
    showRecoveryCodes.value = true
    emit('status-change', t('mfa.codesRegenerated'))
  } catch {
    emit('status-change', t('mfa.regenerateFailed'))
  } finally {
    loading.value = false
  }
}

async function copyRecoveryCodes() {
  const codesText = recoveryCodes.value.join('\n')
  await navigator.clipboard.writeText(codesText)
  copiedCodes.value = true
  setTimeout(() => {
    copiedCodes.value = false
  }, 2000)
}

function closeRecoveryCodes() {
  showRecoveryCodes.value = false
  recoveryCodes.value = []
}

function closeDisableModal() {
  showDisableModal.value = false
  disablePassword.value = ''
}
</script>

<template>
  <section class="mb-6 sm:mb-8">
    <h2
      class="text-base sm:text-lg font-semibold text-gray-900 dark:text-white mb-3 sm:mb-4 flex items-center gap-2"
    >
      <svg
        xmlns="http://www.w3.org/2000/svg"
        class="h-5 w-5 flex-shrink-0"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"
        />
      </svg>
      <span class="truncate">{{ t('mfa.title') }}</span>
    </h2>

    <div class="glass-card p-4 sm:p-6">
      <!-- Loading state -->
      <div v-if="loading && !setupData" class="text-gray-500 dark:text-slate-400 text-center py-4">
        {{ t('common.loading') }}
      </div>

      <!-- MFA Status -->
      <div v-else-if="mfaStatus && !setupData">
        <div class="flex items-center justify-between mb-4">
          <div class="flex items-center gap-3">
            <div
              :class="[
                'w-3 h-3 rounded-full',
                mfaStatus.enabled ? 'bg-green-500' : 'bg-yellow-500',
              ]"
            />
            <span class="text-gray-900 dark:text-white font-medium">
              {{ mfaStatus.enabled ? t('mfa.enabled') : t('mfa.disabled') }}
            </span>
          </div>
          <span v-if="mfaStatus.enabled" class="text-sm text-gray-500 dark:text-slate-400">
            {{ t('mfa.recoveryCodesRemaining', { count: mfaStatus.recovery_codes_remaining }) }}
          </span>
        </div>

        <p class="text-sm text-gray-500 dark:text-slate-400 mb-4">
          {{ mfaStatus.enabled ? t('mfa.enabledDescription') : t('mfa.disabledDescription') }}
        </p>

        <div class="flex flex-wrap gap-3">
          <button
            v-if="!mfaStatus.enabled"
            :disabled="loading"
            class="px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg text-sm transition-colors disabled:opacity-50"
            @click="startSetup"
          >
            {{ t('mfa.setup') }}
          </button>
          <template v-else>
            <button
              :disabled="loading"
              class="px-4 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-900 dark:text-white rounded-lg text-sm transition-colors disabled:opacity-50"
              @click="regenerateCodes"
            >
              {{ t('mfa.regenerateCodes') }}
            </button>
            <button
              :disabled="loading"
              class="px-4 py-2 bg-red-600 hover:bg-red-700 text-white rounded-lg text-sm transition-colors disabled:opacity-50"
              @click="showDisableModal = true"
            >
              {{ t('mfa.disable') }}
            </button>
          </template>
        </div>
      </div>

      <!-- MFA Setup Flow -->
      <div v-if="setupData" class="space-y-6">
        <div>
          <h3 class="text-gray-900 dark:text-white font-medium mb-2">{{ t('mfa.step1Title') }}</h3>
          <p class="text-sm text-gray-500 dark:text-slate-400 mb-4">
            {{ t('mfa.step1Description') }}
          </p>

          <!-- QR Code -->
          <div v-if="setupData.qr_code" class="flex justify-center mb-4">
            <img
              :src="setupData.qr_code"
              alt="MFA QR Code"
              class="w-48 h-48 bg-white p-2 rounded-lg"
            />
          </div>

          <!-- Manual entry -->
          <div class="bg-gray-100 dark:bg-slate-700 rounded-lg p-3">
            <p class="text-xs text-gray-500 dark:text-slate-400 mb-1">{{ t('mfa.manualEntry') }}</p>
            <code class="text-sm text-gray-900 dark:text-white break-all">{{
              setupData.secret
            }}</code>
          </div>
        </div>

        <div>
          <h3 class="text-gray-900 dark:text-white font-medium mb-2">{{ t('mfa.step2Title') }}</h3>
          <p class="text-sm text-gray-500 dark:text-slate-400 mb-4">
            {{ t('mfa.step2Description') }}
          </p>

          <div class="flex gap-3">
            <input
              v-model="verificationCode"
              type="text"
              inputmode="numeric"
              pattern="[0-9]*"
              maxlength="6"
              :placeholder="t('mfa.enterCode')"
              class="flex-1 bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-400 border border-gray-200 dark:border-slate-600 text-center text-lg tracking-widest"
              @keyup.enter="verifySetup"
            />
          </div>
        </div>

        <div class="flex gap-3">
          <button
            :disabled="loading || verificationCode.length !== 6"
            class="flex-1 px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg text-sm transition-colors disabled:opacity-50"
            @click="verifySetup"
          >
            {{ loading ? t('common.verifying') : t('mfa.verify') }}
          </button>
          <button
            :disabled="loading"
            class="px-4 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-900 dark:text-white rounded-lg text-sm transition-colors"
            @click="cancelSetup"
          >
            {{ t('common.cancel') }}
          </button>
        </div>
      </div>
    </div>

    <!-- Recovery Codes Modal -->
    <div
      v-if="showRecoveryCodes"
      class="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4"
      @click.self="closeRecoveryCodes"
    >
      <div class="bg-white dark:bg-slate-800 rounded-lg max-w-md w-full shadow-xl">
        <div class="p-4 sm:p-6">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">
            {{ t('mfa.recoveryCodes') }}
          </h3>

          <div
            class="bg-yellow-50 dark:bg-yellow-900/30 border border-yellow-300 dark:border-yellow-600 rounded-lg p-4 mb-4"
          >
            <p class="text-yellow-800 dark:text-yellow-200 text-sm">
              {{ t('mfa.recoveryCodesWarning') }}
            </p>
          </div>

          <div class="bg-gray-100 dark:bg-slate-700 rounded-lg p-4 mb-4">
            <div class="grid grid-cols-2 gap-2">
              <code
                v-for="code in recoveryCodes"
                :key="code"
                class="text-sm text-gray-900 dark:text-white font-mono"
              >
                {{ code }}
              </code>
            </div>
          </div>

          <div class="flex gap-3">
            <button
              class="flex-1 px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg text-sm transition-colors flex items-center justify-center gap-2"
              @click="copyRecoveryCodes"
            >
              <svg
                v-if="!copiedCodes"
                xmlns="http://www.w3.org/2000/svg"
                class="h-4 w-4"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"
                />
              </svg>
              <svg
                v-else
                xmlns="http://www.w3.org/2000/svg"
                class="h-4 w-4"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M5 13l4 4L19 7"
                />
              </svg>
              {{ copiedCodes ? t('common.copied') : t('common.copy') }}
            </button>
            <button
              class="px-4 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-900 dark:text-white rounded-lg text-sm transition-colors"
              @click="closeRecoveryCodes"
            >
              {{ t('common.done') }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Disable MFA Modal -->
    <div
      v-if="showDisableModal"
      class="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4"
      @click.self="showDisableModal = false"
    >
      <div class="bg-white dark:bg-slate-800 rounded-lg max-w-md w-full shadow-xl">
        <div class="p-4 sm:p-6">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">
            {{ t('mfa.disableTitle') }}
          </h3>

          <div
            class="bg-red-50 dark:bg-red-900/30 border border-red-300 dark:border-red-600 rounded-lg p-4 mb-4"
          >
            <p class="text-red-800 dark:text-red-200 text-sm">
              {{ t('mfa.disableWarning') }}
            </p>
          </div>

          <div class="mb-4">
            <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">
              {{ t('mfa.enterPassword') }}
            </label>
            <input
              v-model="disablePassword"
              type="password"
              :placeholder="t('mfa.passwordPlaceholder')"
              class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-400 border border-gray-200 dark:border-slate-600"
            />
          </div>

          <div class="flex gap-3">
            <button
              :disabled="loading || !disablePassword"
              class="flex-1 px-4 py-2 bg-red-600 hover:bg-red-700 text-white rounded-lg text-sm transition-colors disabled:opacity-50"
              @click="disableMFA"
            >
              {{ loading ? t('common.processing') : t('mfa.confirmDisable') }}
            </button>
            <button
              :disabled="loading"
              class="px-4 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-900 dark:text-white rounded-lg text-sm transition-colors"
              @click="closeDisableModal"
            >
              {{ t('common.cancel') }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>
