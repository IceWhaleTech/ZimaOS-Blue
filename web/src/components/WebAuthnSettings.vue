<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  webauthnApi,
  prepareRegistrationOptions,
  type WebAuthnCredential,
  type WebAuthnStatus,
} from '@/api/webauthn'

const { t } = useI18n()

const emit = defineEmits<{
  (e: 'status-change', message: string): void
}>()

const loading = ref(false)
const status = ref<WebAuthnStatus | null>(null)
const showRegisterModal = ref(false)
const newCredentialName = ref('')
const registering = ref(false)
const isSupported = ref(false)

onMounted(async () => {
  // Check if WebAuthn is supported
  isSupported.value = !!window.PublicKeyCredential
  if (isSupported.value) {
    await loadStatus()
  }
})

async function loadStatus() {
  try {
    loading.value = true
    const response = await webauthnApi.getStatus()
    status.value = response.data
  } catch {
    status.value = { enabled: false, credentials: [] }
  } finally {
    loading.value = false
  }
}

function openRegisterModal() {
  showRegisterModal.value = true
  newCredentialName.value = ''
}

function closeRegisterModal() {
  showRegisterModal.value = false
  newCredentialName.value = ''
}

async function registerCredential() {
  if (!newCredentialName.value) return

  try {
    registering.value = true

    // Begin registration
    const beginResponse = await webauthnApi.beginRegistration(newCredentialName.value)
    const options = prepareRegistrationOptions(beginResponse.data)

    // Create credential using browser API
    const credential = (await navigator.credentials.create({
      publicKey: options,
    })) as PublicKeyCredential

    if (!credential) {
      emit('status-change', t('webauthn.userCancelled'))
      return
    }

    // Finish registration
    await webauthnApi.finishRegistration(credential, newCredentialName.value)

    emit('status-change', t('webauthn.registerSuccess'))
    closeRegisterModal()
    await loadStatus()
  } catch (e) {
    if (e instanceof Error && e.name === 'NotAllowedError') {
      emit('status-change', t('webauthn.userCancelled'))
    } else {
      emit('status-change', t('webauthn.registerFailed'))
    }
  } finally {
    registering.value = false
  }
}

async function deleteCredential(credentialId: string) {
  if (!confirm(t('webauthn.confirmDelete'))) return

  try {
    loading.value = true
    await webauthnApi.deleteCredential(credentialId)
    emit('status-change', t('webauthn.deleteSuccess'))
    await loadStatus()
  } catch {
    emit('status-change', t('webauthn.deleteFailed'))
  } finally {
    loading.value = false
  }
}

function formatDate(dateStr: string): string {
  if (!dateStr) return t('webauthn.never')
  return new Date(dateStr).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

const hasCredentials = computed(() => status.value && status.value.credentials.length > 0)
</script>

<template>
  <section class="mb-6 sm:mb-8">
    <h2 class="text-base sm:text-lg font-semibold text-gray-900 dark:text-white mb-3 sm:mb-4 flex items-center gap-2">
      <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z" />
      </svg>
      <span class="truncate">{{ t('webauthn.title') }}</span>
    </h2>

    <div class="glass-card p-4 sm:p-6">
      <!-- Not supported message -->
      <div v-if="!isSupported" class="text-center py-4">
        <div class="text-yellow-600 dark:text-yellow-400 mb-2">
          <svg xmlns="http://www.w3.org/2000/svg" class="h-12 w-12 mx-auto" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
          </svg>
        </div>
        <p class="text-gray-500 dark:text-slate-400">{{ t('webauthn.notSupported') }}</p>
      </div>

      <!-- Loading state -->
      <div v-else-if="loading && !status" class="text-gray-500 dark:text-slate-400 text-center py-4">
        {{ t('common.loading') }}
      </div>

      <!-- Content -->
      <div v-else-if="status">
        <p class="text-sm text-gray-500 dark:text-slate-400 mb-4">
          {{ t('webauthn.subtitle') }}
        </p>

        <!-- Credentials list -->
        <div v-if="hasCredentials" class="space-y-3 mb-4">
          <div
            v-for="credential in status.credentials"
            :key="credential.id"
            class="flex items-center justify-between p-3 bg-gray-50 dark:bg-slate-700/50 rounded-lg"
          >
            <div class="flex items-center gap-3 min-w-0">
              <div class="w-10 h-10 rounded-lg bg-accent/10 flex items-center justify-center flex-shrink-0">
                <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-accent" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z" />
                </svg>
              </div>
              <div class="min-w-0">
                <h4 class="text-gray-900 dark:text-white font-medium truncate">{{ credential.name }}</h4>
                <p class="text-xs text-gray-500 dark:text-slate-400">
                  {{ t('webauthn.lastUsed') }}: {{ formatDate(credential.last_used_at) }}
                </p>
              </div>
            </div>
            <button
              :disabled="loading"
              class="p-2 text-red-600 dark:text-red-400 hover:text-red-700 dark:hover:text-red-300 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg transition-colors disabled:opacity-50"
              :title="t('webauthn.delete')"
              @click="deleteCredential(credential.id)"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
              </svg>
            </button>
          </div>
        </div>

        <!-- Empty state -->
        <div v-else class="text-center py-6 mb-4">
          <div class="text-gray-400 dark:text-slate-500 mb-2">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-12 w-12 mx-auto" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z" />
            </svg>
          </div>
          <h3 class="text-gray-900 dark:text-white font-medium mb-1">{{ t('webauthn.noCredentials') }}</h3>
          <p class="text-sm text-gray-500 dark:text-slate-400">{{ t('webauthn.noCredentialsDesc') }}</p>
        </div>

        <!-- Add button -->
        <button
          :disabled="loading"
          class="px-4 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg text-sm transition-colors disabled:opacity-50 flex items-center gap-2"
          @click="openRegisterModal"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
          </svg>
          {{ t('webauthn.addCredential') }}
        </button>
      </div>
    </div>

    <!-- Register Modal -->
    <div
      v-if="showRegisterModal"
      class="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4"
      @click.self="closeRegisterModal"
    >
      <div class="bg-white dark:bg-slate-800 rounded-lg max-w-md w-full shadow-xl">
        <div class="p-4 sm:p-6">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">
            {{ t('webauthn.addCredential') }}
          </h3>

          <form @submit.prevent="registerCredential" class="space-y-4">
            <div>
              <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">
                {{ t('webauthn.credentialName') }}
              </label>
              <input
                v-model="newCredentialName"
                type="text"
                required
                class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-accent border border-gray-200 dark:border-slate-600"
                :placeholder="t('webauthn.credentialNamePlaceholder')"
              />
            </div>

            <div class="flex gap-3 pt-4">
              <button
                type="submit"
                :disabled="registering || !newCredentialName"
                class="flex-1 px-4 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg text-sm transition-colors disabled:opacity-50"
              >
                {{ registering ? t('webauthn.registering') : t('webauthn.register') }}
              </button>
              <button
                type="button"
                :disabled="registering"
                class="px-4 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-900 dark:text-white rounded-lg text-sm transition-colors"
                @click="closeRegisterModal"
              >
                {{ t('common.cancel') }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  </section>
</template>
