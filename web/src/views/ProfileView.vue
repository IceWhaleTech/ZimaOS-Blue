<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { extauthApi, getProviderDisplayName } from '@/api/extauth'
import type { CreateApiKeyRequest } from '@/api/auth'
import type { ProviderInfo, LinkedAccount, ProviderType } from '@/api/extauth'
import MFASettings from '@/components/MFASettings.vue'
import WebAuthnSettings from '@/components/WebAuthnSettings.vue'

const { t } = useI18n()
const authStore = useAuthStore()

// Profile editing
const isEditingProfile = ref(false)
const editEmail = ref('')
const editPassword = ref('')
const editPasswordConfirm = ref('')
const profileSaveStatus = ref<string | null>(null)

// Linked accounts
const linkedAccounts = ref<LinkedAccount[]>([])
const availableProviders = ref<ProviderInfo[]>([])
const loadingLinkedAccounts = ref(false)
const linkingProvider = ref<string | null>(null)
const unlinkingProvider = ref<string | null>(null)

// API Key creation
const showCreateKeyModal = ref(false)
const newKeyName = ref('')
const newKeyScopes = ref<string[]>(['chat', 'skills.execute'])
const newKeyExpiry = ref('30d')
const createdKey = ref<string | null>(null)
const copiedKey = ref(false)

const availableScopes = computed(() => [
  { value: 'chat', label: t('profile.scopeChat'), description: t('profile.scopeChatDesc') },
  { value: 'chat.read', label: t('profile.scopeChatRead'), description: t('profile.scopeChatReadDesc') },
  { value: 'skills.execute', label: t('profile.scopeSkillsExecute'), description: t('profile.scopeSkillsExecuteDesc') },
  { value: 'skills.list', label: t('profile.scopeSkillsList'), description: t('profile.scopeSkillsListDesc') },
  { value: 'plugins.manage', label: t('profile.scopePluginsManage'), description: t('profile.scopePluginsManageDesc') },
  { value: 'system.read', label: t('profile.scopeSystemRead'), description: t('profile.scopeSystemReadDesc') },
])

const expiryOptions = computed(() => [
  { value: '7d', label: t('profile.expiry7Days') },
  { value: '30d', label: t('profile.expiry30Days') },
  { value: '90d', label: t('profile.expiry90Days') },
  { value: '365d', label: t('profile.expiry1Year') },
  { value: '', label: t('profile.expiryNever') },
])

const passwordsMatch = computed(() => {
  if (!editPassword.value && !editPasswordConfirm.value) return true
  return editPassword.value === editPasswordConfirm.value
})

onMounted(async () => {
  await authStore.fetchUser()
  await authStore.fetchApiKeys()
  await loadLinkedAccounts()
  await loadAvailableProviders()
  if (authStore.user) {
    editEmail.value = authStore.user.email || ''
  }
})

async function loadLinkedAccounts() {
  try {
    loadingLinkedAccounts.value = true
    const response = await extauthApi.getLinkedAccounts()
    linkedAccounts.value = response.data
  } catch {
    linkedAccounts.value = []
  } finally {
    loadingLinkedAccounts.value = false
  }
}

async function loadAvailableProviders() {
  try {
    const response = await extauthApi.listProviders()
    availableProviders.value = response.data.sort((a, b) => a.order - b.order)
  } catch {
    availableProviders.value = []
  }
}

async function linkProvider(provider: ProviderInfo) {
  try {
    linkingProvider.value = provider.id
    sessionStorage.setItem('oauth_redirect', '/profile')
    sessionStorage.setItem('oauth_action', 'link')
    const callbackUrl = `${window.location.origin}/auth/callback/${provider.id}`
    window.location.href = `/api/v1/auth/link/${provider.id}?redirect_uri=${encodeURIComponent(callbackUrl)}`
  } catch (e) {
    authStore.error = e instanceof Error ? e.message : t('profile.failedToStartLinking')
    linkingProvider.value = null
  }
}

async function unlinkProvider(providerId: string) {
  if (!confirm(t('profile.confirmUnlinkAccount'))) {
    return
  }
  try {
    unlinkingProvider.value = providerId
    await extauthApi.unlinkAccount(providerId)
    linkedAccounts.value = linkedAccounts.value.filter(a => a.provider_id !== providerId)
    showSaveStatus(t('profile.accountUnlinkedSuccessfully'))
  } catch (e) {
    authStore.error = e instanceof Error ? e.message : t('profile.failedToUnlinkAccount')
  } finally {
    unlinkingProvider.value = null
  }
}

function isProviderLinked(providerId: string): boolean {
  return linkedAccounts.value.some(a => a.provider_id === providerId)
}

function getLinkedAccount(providerId: string): LinkedAccount | undefined {
  return linkedAccounts.value.find(a => a.provider_id === providerId)
}

function getProviderName(provider: ProviderInfo): string {
  return provider.name || getProviderDisplayName(provider.type)
}

function getProviderIconSvg(type: ProviderType): string {
  const icons: Record<ProviderType, string> = {
    google: 'M12.545,10.239v3.821h5.445c-0.712,2.315-2.647,3.972-5.445,3.972c-3.332,0-6.033-2.701-6.033-6.032s2.701-6.032,6.033-6.032c1.498,0,2.866,0.549,3.921,1.453l2.814-2.814C17.503,2.988,15.139,2,12.545,2C7.021,2,2.543,6.477,2.543,12s4.478,10,10.002,10c8.396,0,10.249-7.85,9.426-11.748L12.545,10.239z',
    github: 'M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z',
    microsoft: 'M1 1h10v10H1V1zm12 0h10v10H13V1zM1 13h10v10H1V13zm12 0h10v10H13V13z',
    keycloak: 'M12 2L2 7v10l10 5 10-5V7L12 2zm0 2.18l6.9 3.45L12 11.08 5.1 7.63 12 4.18zM4 8.82l7 3.5v6.36l-7-3.5V8.82zm16 6.36l-7 3.5v-6.36l7-3.5v6.36z',
    authentik: 'M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z',
    auth0: 'M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 18c-4.41 0-8-3.59-8-8s3.59-8 8-8 8 3.59 8 8-3.59 8-8 8zm-1-13h2v6h-2zm0 8h2v2h-2z',
    generic: 'M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 17h-2v-2h2v2zm2.07-7.75l-.9.92C13.45 12.9 13 13.5 13 15h-2v-.5c0-1.1.45-2.1 1.17-2.83l1.24-1.26c.37-.36.59-.86.59-1.41 0-1.1-.9-2-2-2s-2 .9-2 2H8c0-2.21 1.79-4 4-4s4 1.79 4 4c0 .88-.36 1.68-.93 2.25z',
  }
  return icons[type] || icons.generic
}

function startEditProfile() {
  isEditingProfile.value = true
  editEmail.value = authStore.user?.email || ''
  editPassword.value = ''
  editPasswordConfirm.value = ''
}

function cancelEditProfile() {
  isEditingProfile.value = false
  editEmail.value = authStore.user?.email || ''
  editPassword.value = ''
  editPasswordConfirm.value = ''
}

async function saveProfile() {
  if (editPassword.value && !passwordsMatch.value) {
    return
  }

  const data: { email?: string; password?: string } = {}
  if (editEmail.value !== authStore.user?.email) {
    data.email = editEmail.value
  }
  if (editPassword.value) {
    data.password = editPassword.value
  }

  if (Object.keys(data).length === 0) {
    isEditingProfile.value = false
    return
  }

  const success = await authStore.updateProfile(data)
  if (success) {
    isEditingProfile.value = false
    showSaveStatus(t('profile.profileUpdatedSuccessfully'))
  }
}

function showSaveStatus(message: string) {
  profileSaveStatus.value = message
  setTimeout(() => {
    profileSaveStatus.value = null
  }, 3000)
}

function handleMFAStatusChange(message: string) {
  showSaveStatus(message)
}

function openCreateKeyModal() {
  showCreateKeyModal.value = true
  newKeyName.value = ''
  newKeyScopes.value = ['chat', 'skills.execute']
  newKeyExpiry.value = '30d'
  createdKey.value = null
}

function closeCreateKeyModal() {
  showCreateKeyModal.value = false
  createdKey.value = null
  copiedKey.value = false
}

async function createApiKey() {
  if (!newKeyName.value || newKeyScopes.value.length === 0) {
    return
  }

  const data: CreateApiKeyRequest = {
    name: newKeyName.value,
    scopes: newKeyScopes.value,
  }
  if (newKeyExpiry.value) {
    data.expires_in = newKeyExpiry.value
  }

  const result = await authStore.createApiKey(data)
  if (result) {
    createdKey.value = result.key
  }
}

async function copyKey() {
  if (createdKey.value) {
    await navigator.clipboard.writeText(createdKey.value)
    copiedKey.value = true
    setTimeout(() => {
      copiedKey.value = false
    }, 2000)
  }
}

async function deleteApiKey(id: string) {
  if (confirm(t('profile.confirmDeleteApiKey'))) {
    await authStore.deleteApiKey(id)
  }
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  })
}

function toggleScope(scope: string) {
  const index = newKeyScopes.value.indexOf(scope)
  if (index === -1) {
    newKeyScopes.value = [...newKeyScopes.value, scope]
  } else {
    newKeyScopes.value = newKeyScopes.value.filter((s) => s !== scope)
  }
}

// Get translated scope label
function getScopeLabel(scope: string): string {
  const scopeMap: Record<string, string> = {
    'chat': t('profile.scopeChat'),
    'chat.read': t('profile.scopeChatRead'),
    'skills.execute': t('profile.scopeSkillsExecute'),
    'skills.list': t('profile.scopeSkillsList'),
    'plugins.manage': t('profile.scopePluginsManage'),
    'system.read': t('profile.scopeSystemRead'),
  }
  return scopeMap[scope] || scope
}
</script>

<template>
  <div class="profile-view p-4 sm:p-6 max-w-4xl mx-auto">
    <h1 class="text-xl sm:text-2xl font-bold text-gray-900 dark:text-white mb-6">{{ $t('profile.title') }}</h1>

    <!-- Save status notification -->
    <div
      v-if="profileSaveStatus"
      class="fixed top-20 right-4 bg-green-600 text-white px-4 py-2 rounded-lg shadow-lg z-50"
    >
      {{ profileSaveStatus }}
    </div>

    <!-- User Profile Section -->
    <section class="mb-6 sm:mb-8">
      <h2 class="text-base sm:text-lg font-semibold text-gray-900 dark:text-white mb-3 sm:mb-4 flex items-center gap-2">
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
            d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"
          />
        </svg>
        <span class="truncate">{{ $t('profile.accountInformation') }}</span>
      </h2>

      <div class="glass-card p-4 sm:p-6">
        <div v-if="authStore.user" class="space-y-4">
          <!-- Username (read-only) -->
          <div>
            <label class="block text-sm text-gray-500 dark:text-slate-400 mb-1">{{ $t('profile.username') }}</label>
            <div class="text-gray-900 dark:text-white font-medium">{{ authStore.user.username }}</div>
          </div>

          <!-- Role -->
          <div>
            <label class="block text-sm text-gray-500 dark:text-slate-400 mb-1">{{ $t('profile.role') }}</label>
            <span
              class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium"
              :class="
                authStore.user.role === 'admin'
                  ? 'bg-purple-100 dark:bg-purple-900/50 text-purple-700 dark:text-purple-300'
                  : authStore.user.role === 'guest'
                    ? 'bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-400'
                    : 'bg-blue-100 dark:bg-blue-900/30 text-blue-800 dark:text-blue-400'
              "
            >
              {{ $t(`users.role${authStore.user.role.charAt(0).toUpperCase()}${authStore.user.role.slice(1)}`) }}
            </span>
          </div>

          <!-- Email -->
          <div>
            <label class="block text-sm text-gray-500 dark:text-slate-400 mb-1">{{ $t('profile.email') }}</label>
            <div v-if="!isEditingProfile" class="flex items-center gap-2">
              <span class="text-gray-900 dark:text-white">{{ authStore.user.email || $t('profile.notSet') }}</span>
            </div>
            <input
              v-else
              v-model="editEmail"
              type="email"
              class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-400 border border-gray-200 dark:border-slate-600"
              :placeholder="$t('profile.enterEmailAddress')"
            />
          </div>

          <!-- Password (edit mode only) -->
          <div v-if="isEditingProfile" class="space-y-4">
            <div>
              <label class="block text-sm text-gray-500 dark:text-slate-400 mb-1">{{ $t('profile.newPassword') }}</label>
              <input
                v-model="editPassword"
                type="password"
                class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-400 border border-gray-200 dark:border-slate-600"
                :placeholder="$t('profile.leaveBlankToKeepPassword')"
              />
            </div>
            <div>
              <label class="block text-sm text-gray-500 dark:text-slate-400 mb-1">{{ $t('profile.confirmPassword') }}</label>
              <input
                v-model="editPasswordConfirm"
                type="password"
                class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-400 border border-gray-200 dark:border-slate-600"
                :class="{ 'ring-2 ring-red-500': editPassword && !passwordsMatch }"
                :placeholder="$t('profile.confirmNewPasswordPlaceholder')"
              />
              <p v-if="editPassword && !passwordsMatch" class="text-red-500 dark:text-red-400 text-sm mt-1">
                {{ $t('profile.passwordsDoNotMatch') }}
              </p>
            </div>
          </div>

          <!-- Member since -->
          <div>
            <label class="block text-sm text-gray-500 dark:text-slate-400 mb-1">{{ $t('profile.memberSince') }}</label>
            <div class="text-gray-900 dark:text-white">{{ formatDate(authStore.user.created_at) }}</div>
          </div>

          <!-- Action buttons -->
          <div class="flex gap-3 pt-4">
            <button
              v-if="!isEditingProfile"
              class="px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg text-sm transition-colors"
              @click="startEditProfile"
            >
              {{ $t('profile.editProfile') }}
            </button>
            <template v-else>
              <button
                class="px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg text-sm transition-colors disabled:opacity-50"
                :disabled="authStore.loading || (editPassword && !passwordsMatch) || false"
                @click="saveProfile"
              >
                {{ authStore.loading ? $t('profile.saving') : $t('profile.saveChanges') }}
              </button>
              <button
                class="px-4 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-900 dark:text-white rounded-lg text-sm transition-colors"
                @click="cancelEditProfile"
              >
                {{ $t('common.cancel') }}
              </button>
            </template>
          </div>
        </div>

        <div v-else class="text-gray-500 dark:text-slate-400 text-center py-4">{{ $t('profile.loadingUserInformation') }}</div>
      </div>
    </section>

    <!-- User Management Link (Admin Only) -->
    <section v-if="authStore.isAdmin" class="mb-6 sm:mb-8">
      <RouterLink
        to="/users"
        class="glass-card p-4 sm:p-6 flex items-center justify-between hover:bg-gray-50 dark:hover:bg-white/5 transition-colors group"
      >
        <div class="flex items-center gap-3 sm:gap-4">
          <div class="w-10 h-10 rounded-lg bg-purple-100 dark:bg-purple-900/50 flex items-center justify-center flex-shrink-0">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-5 w-5 text-purple-600 dark:text-purple-400"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z"
              />
            </svg>
          </div>
          <div>
            <h3 class="text-gray-900 dark:text-white font-medium">{{ $t('profile.userManagement') }}</h3>
            <p class="text-sm text-gray-500 dark:text-slate-400">{{ $t('profile.userManagementDesc') }}</p>
          </div>
        </div>
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-5 w-5 text-gray-400 dark:text-slate-500 group-hover:text-gray-600 dark:group-hover:text-slate-300 transition-colors"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
        </svg>
      </RouterLink>
    </section>

    <!-- MFA Settings Section -->
    <MFASettings @status-change="handleMFAStatusChange" />

    <!-- WebAuthn Settings Section -->
    <WebAuthnSettings @status-change="handleMFAStatusChange" />

    <!-- Linked Accounts Section -->
    <section v-if="availableProviders.length > 0" class="mb-6 sm:mb-8">
      <h2 class="text-base sm:text-lg font-semibold text-gray-900 dark:text-white mb-3 sm:mb-4 flex items-center gap-2">
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
            d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1"
          />
        </svg>
        <span class="truncate">{{ $t('profile.linkedAccounts') }}</span>
      </h2>

      <div class="glass-card overflow-hidden">
        <div v-if="loadingLinkedAccounts" class="p-4 sm:p-6 text-gray-500 dark:text-slate-400 text-center">
          {{ $t('profile.loadingLinkedAccounts') }}
        </div>
        <div v-else class="divide-y divide-gray-200 dark:divide-slate-700">
          <div
            v-for="provider in availableProviders"
            :key="provider.id"
            class="p-3 sm:p-4 flex items-center justify-between"
          >
            <div class="flex items-center gap-3 sm:gap-4 min-w-0">
              <!-- Provider Icon -->
              <div class="w-10 h-10 rounded-lg bg-gray-100 dark:bg-slate-700 flex items-center justify-center flex-shrink-0">
                <img
                  v-if="provider.icon_url"
                  :src="provider.icon_url"
                  :alt="getProviderName(provider)"
                  class="h-6 w-6"
                />
                <svg
                  v-else
                  class="h-6 w-6 text-gray-600 dark:text-gray-300"
                  viewBox="0 0 24 24"
                  fill="currentColor"
                >
                  <path :d="getProviderIconSvg(provider.type)" />
                </svg>
              </div>

              <!-- Provider Info -->
              <div class="min-w-0">
                <h3 class="text-gray-900 dark:text-white font-medium truncate">{{ getProviderName(provider) }}</h3>
                <template v-if="isProviderLinked(provider.id)">
                  <p class="text-sm text-gray-500 dark:text-slate-400 truncate">
                    {{ getLinkedAccount(provider.id)?.email || getLinkedAccount(provider.id)?.name || $t('profile.connected') }}
                  </p>
                  <p class="text-xs text-gray-400 dark:text-slate-500">
                    {{ $t('profile.linked') }} {{ formatDate(getLinkedAccount(provider.id)!.created_at) }}
                  </p>
                </template>
                <p v-else class="text-sm text-gray-400 dark:text-slate-500">{{ $t('profile.notConnected') }}</p>
              </div>
            </div>

            <!-- Action Button -->
            <button
              v-if="isProviderLinked(provider.id)"
              :disabled="unlinkingProvider === provider.id"
              class="px-3 sm:px-4 py-2 text-sm text-red-600 dark:text-red-400 hover:text-red-700 dark:hover:text-red-300 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg transition-colors disabled:opacity-50 flex-shrink-0"
              @click="unlinkProvider(provider.id)"
            >
              {{ unlinkingProvider === provider.id ? $t('profile.unlinking') : $t('profile.unlink') }}
            </button>
            <button
              v-else
              :disabled="linkingProvider === provider.id"
              class="px-3 sm:px-4 py-2 text-sm bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg transition-colors disabled:opacity-50 flex-shrink-0"
              @click="linkProvider(provider)"
            >
              {{ linkingProvider === provider.id ? $t('profile.connecting') : $t('profile.connect') }}
            </button>
          </div>
        </div>
      </div>
    </section>

    <!-- API Keys Section -->
    <section class="mb-6 sm:mb-8">
      <div class="flex items-center justify-between mb-3 sm:mb-4">
        <h2 class="text-base sm:text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
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
              d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z"
            />
          </svg>
          <span class="truncate">{{ $t('profile.apiKeys') }}</span>
        </h2>
        <button
          class="px-3 sm:px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg text-sm transition-colors flex items-center gap-2 flex-shrink-0"
          @click="openCreateKeyModal"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-4 w-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
          </svg>
          <span class="hidden sm:inline">{{ $t('profile.createApiKey') }}</span>
          <span class="sm:hidden">{{ $t('profile.create') }}</span>
        </button>
      </div>

      <div class="glass-card overflow-hidden">
        <div v-if="authStore.apiKeys.length === 0" class="p-4 sm:p-6 text-gray-500 dark:text-slate-400 text-center">
          {{ $t('profile.noApiKeysCreated') }}
        </div>
        <div v-else class="divide-y divide-gray-200 dark:divide-slate-700">
          <div
            v-for="key in authStore.apiKeys"
            :key="key.id"
            class="p-3 sm:p-4 flex items-center justify-between"
          >
            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-2 sm:gap-3 flex-wrap">
                <h3 class="text-gray-900 dark:text-white font-medium">{{ key.name }}</h3>
                <code class="text-xs bg-gray-100 dark:bg-slate-700 px-2 py-1 rounded text-gray-600 dark:text-gray-300">
                  {{ key.key_prefix }}...
                </code>
              </div>
              <div class="flex items-center gap-2 sm:gap-4 mt-2 text-xs sm:text-sm text-gray-500 dark:text-slate-400 flex-wrap">
                <span>{{ $t('profile.created') }}: {{ formatDate(key.created_at) }}</span>
                <span v-if="key.expires_at">{{ $t('profile.expires') }}: {{ formatDate(key.expires_at) }}</span>
                <span v-else class="text-green-600 dark:text-green-400">{{ $t('profile.neverExpires') }}</span>
                <span v-if="key.last_used_at" class="hidden sm:inline">{{ $t('profile.lastUsed') }}: {{ formatDate(key.last_used_at) }}</span>
              </div>
              <div class="flex flex-wrap gap-1 mt-2">
                <span
                  v-for="scope in key.scopes"
                  :key="scope"
                  class="text-xs bg-gray-100 dark:bg-slate-700 px-2 py-0.5 rounded text-gray-600 dark:text-gray-300"
                >
                  {{ getScopeLabel(scope) }}
                </span>
              </div>
            </div>
            <button
              class="p-2 text-red-600 dark:text-red-400 hover:text-red-700 dark:hover:text-red-300 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg transition-colors flex-shrink-0"
              :title="$t('profile.deleteApiKey')"
              @click="deleteApiKey(key.id)"
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="h-5 w-5"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                />
              </svg>
            </button>
          </div>
        </div>
      </div>
    </section>

    <!-- Create API Key Modal -->
    <div
      v-if="showCreateKeyModal"
      class="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4"
      @click.self="closeCreateKeyModal"
    >
      <div class="bg-white dark:bg-slate-800 rounded-lg max-w-lg w-full max-h-[90vh] overflow-y-auto shadow-xl">
        <div class="p-4 sm:p-6">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">
            {{ createdKey ? $t('profile.apiKeyCreated') : $t('profile.createApiKey') }}
          </h3>

          <!-- Show created key -->
          <div v-if="createdKey" class="space-y-4">
            <div class="bg-yellow-50 dark:bg-yellow-900/30 border border-yellow-300 dark:border-yellow-600 rounded-lg p-4">
              <p class="text-yellow-800 dark:text-yellow-200 text-sm mb-2">
                {{ $t('profile.copyApiKeyWarning') }}
              </p>
              <div class="flex items-center gap-2">
                <code class="flex-1 bg-gray-100 dark:bg-gray-700 px-3 py-2 rounded text-green-600 dark:text-green-400 text-sm break-all">
                  {{ createdKey }}
                </code>
                <button
                  class="p-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-300 dark:hover:bg-gray-600 rounded-lg transition-colors"
                  :class="{ 'bg-green-500 dark:bg-green-600': copiedKey }"
                  @click="copyKey"
                >
                  <svg
                    v-if="!copiedKey"
                    xmlns="http://www.w3.org/2000/svg"
                    class="h-5 w-5 text-gray-700 dark:text-white"
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
                    class="h-5 w-5 text-white"
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
                </button>
              </div>
            </div>
            <button
              class="w-full px-4 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-900 dark:text-white rounded-lg transition-colors"
              @click="closeCreateKeyModal"
            >
              {{ $t('profile.done') }}
            </button>
          </div>

          <!-- Create key form -->
          <form v-else class="space-y-4" @submit.prevent="createApiKey">
            <!-- Name -->
            <div>
              <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">{{ $t('profile.name') }}</label>
              <input
                v-model="newKeyName"
                type="text"
                required
                class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-400 border border-gray-200 dark:border-slate-600"
                :placeholder="$t('profile.apiKeyNamePlaceholder')"
              />
            </div>

            <!-- Scopes -->
            <div>
              <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">{{ $t('profile.permissions') }}</label>
              <div class="space-y-2">
                <label
                  v-for="scope in availableScopes"
                  :key="scope.value"
                  class="flex items-start gap-3 p-3 bg-gray-50 dark:bg-slate-700/50 rounded-lg cursor-pointer hover:bg-gray-100 dark:hover:bg-slate-700 transition-colors"
                >
                  <input
                    type="checkbox"
                    :checked="newKeyScopes.includes(scope.value)"
                    class="mt-1 w-4 h-4 rounded border-gray-300 dark:border-slate-600 bg-white dark:bg-slate-700 text-gray-900 dark:text-gray-300 focus:ring-gray-400"
                    @change="toggleScope(scope.value)"
                  />
                  <div>
                    <div class="text-gray-900 dark:text-white text-sm font-medium">{{ scope.label }}</div>
                    <div class="text-gray-500 dark:text-slate-400 text-xs">{{ scope.description }}</div>
                  </div>
                </label>
              </div>
            </div>

            <!-- Expiry -->
            <div>
              <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">{{ $t('profile.expiration') }}</label>
              <select
                v-model="newKeyExpiry"
                class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-400 border border-gray-200 dark:border-slate-600"
              >
                <option v-for="opt in expiryOptions" :key="opt.value" :value="opt.value">
                  {{ opt.label }}
                </option>
              </select>
            </div>

            <!-- Actions -->
            <div class="flex gap-3 pt-4">
              <button
                type="submit"
                :disabled="authStore.loading || !newKeyName || newKeyScopes.length === 0"
                class="flex-1 px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg transition-colors disabled:opacity-50"
              >
                {{ authStore.loading ? $t('profile.creating') : $t('profile.createApiKey') }}
              </button>
              <button
                type="button"
                class="px-4 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-900 dark:text-white rounded-lg transition-colors"
                @click="closeCreateKeyModal"
              >
                {{ $t('common.cancel') }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  </div>
</template>
