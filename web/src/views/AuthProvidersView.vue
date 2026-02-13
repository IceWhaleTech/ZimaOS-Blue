<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { extauthAdminApi, getProviderDisplayName } from '@/api/extauth'
import type { ProviderConfig, ProviderType, CreateProviderRequest, UpdateProviderRequest } from '@/api/extauth'

const { t } = useI18n()

const providers = ref<ProviderConfig[]>([])
const loading = ref(false)
const error = ref<string | null>(null)
const successMessage = ref<string | null>(null)

// Modal state
const showModal = ref(false)
const editingProvider = ref<ProviderConfig | null>(null)
const isCreating = computed(() => !editingProvider.value)

// Form state
const form = ref<CreateProviderRequest>({
  id: '',
  name: '',
  type: 'generic',
  client_id: '',
  client_secret: '',
  issuer_url: '',
  redirect_url: '',
  auto_create_user: true,
  default_role: 'user',
  scopes: ['openid', 'profile', 'email'],
  allowed_domains: [],
  order: 0,
})

const providerTypes: { value: ProviderType; label: string }[] = [
  { value: 'generic', label: 'Generic OIDC' },
  { value: 'google', label: 'Google' },
  { value: 'github', label: 'GitHub' },
  { value: 'microsoft', label: 'Microsoft / Azure AD' },
  { value: 'keycloak', label: 'Keycloak' },
  { value: 'authentik', label: 'Authentik' },
  { value: 'auth0', label: 'Auth0' },
]

const defaultScopes: Record<ProviderType, string[]> = {
  generic: ['openid', 'profile', 'email'],
  google: ['openid', 'profile', 'email'],
  github: ['read:user', 'user:email'],
  microsoft: ['openid', 'profile', 'email'],
  keycloak: ['openid', 'profile', 'email'],
  authentik: ['openid', 'profile', 'email'],
  auth0: ['openid', 'profile', 'email'],
}

onMounted(async () => {
  await loadProviders()
})

async function loadProviders() {
  try {
    loading.value = true
    error.value = null
    const response = await extauthAdminApi.listAllProviders()
    providers.value = response.data.sort((a, b) => a.order - b.order)
  } catch (e) {
    error.value = e instanceof Error ? e.message : t('authProviders.failedToLoadProviders')
  } finally {
    loading.value = false
  }
}

function openCreateModal() {
  editingProvider.value = null
  form.value = {
    id: '',
    name: '',
    type: 'generic',
    client_id: '',
    client_secret: '',
    issuer_url: '',
    redirect_url: `${window.location.origin}/auth/callback/`,
    auto_create_user: true,
    default_role: 'user',
    scopes: ['openid', 'profile', 'email'],
    allowed_domains: [],
    order: providers.value.length,
  }
  showModal.value = true
}

function openEditModal(provider: ProviderConfig) {
  editingProvider.value = provider
  form.value = {
    id: provider.id,
    name: provider.name,
    type: provider.type,
    client_id: provider.client_id,
    client_secret: '', // Don't show existing secret
    issuer_url: provider.issuer_url || '',
    auth_url: provider.auth_url,
    token_url: provider.token_url,
    userinfo_url: provider.userinfo_url,
    redirect_url: provider.redirect_url,
    auto_create_user: provider.auto_create_user,
    default_role: provider.default_role,
    scopes: provider.scopes || [],
    allowed_domains: provider.allowed_domains || [],
    icon_url: provider.icon_url,
    order: provider.order,
  }
  showModal.value = true
}

function closeModal() {
  showModal.value = false
  editingProvider.value = null
}

function onTypeChange() {
  form.value.scopes = [...defaultScopes[form.value.type]]
  // Update redirect URL with provider ID
  if (form.value.id) {
    form.value.redirect_url = `${window.location.origin}/auth/callback/${form.value.id}`
  }
}

function onIdChange() {
  if (isCreating.value && form.value.id) {
    form.value.redirect_url = `${window.location.origin}/auth/callback/${form.value.id}`
  }
}

async function saveProvider() {
  try {
    loading.value = true
    error.value = null

    if (isCreating.value) {
      const response = await extauthAdminApi.createProvider(form.value)
      providers.value = [...providers.value, response.data].sort((a, b) => a.order - b.order)
      showSuccess(t('authProviders.providerCreated'))
    } else {
      const updateData: UpdateProviderRequest = {
        name: form.value.name,
        client_id: form.value.client_id,
        issuer_url: form.value.issuer_url,
        auth_url: form.value.auth_url,
        token_url: form.value.token_url,
        userinfo_url: form.value.userinfo_url,
        redirect_url: form.value.redirect_url,
        auto_create_user: form.value.auto_create_user,
        default_role: form.value.default_role,
        scopes: form.value.scopes,
        allowed_domains: form.value.allowed_domains,
        icon_url: form.value.icon_url,
        order: form.value.order,
      }
      // Only include secret if provided
      if (form.value.client_secret) {
        updateData.client_secret = form.value.client_secret
      }
      const response = await extauthAdminApi.updateProvider(editingProvider.value!.id, updateData)
      const index = providers.value.findIndex(p => p.id === editingProvider.value!.id)
      if (index !== -1) {
        providers.value[index] = response.data
      }
      showSuccess(t('authProviders.providerUpdated'))
    }

    closeModal()
  } catch (e) {
    error.value = e instanceof Error ? e.message : t('authProviders.failedToSaveProvider')
  } finally {
    loading.value = false
  }
}

async function toggleProvider(provider: ProviderConfig) {
  try {
    const response = await extauthAdminApi.toggleProvider(provider.id, !provider.enabled)
    const index = providers.value.findIndex(p => p.id === provider.id)
    if (index !== -1) {
      providers.value[index] = response.data
    }
    showSuccess(response.data.enabled ? t('authProviders.providerEnabled') : t('authProviders.providerDisabled'))
  } catch (e) {
    error.value = e instanceof Error ? e.message : t('authProviders.failedToToggleProvider')
  }
}

async function deleteProvider(provider: ProviderConfig) {
  if (!confirm(t('authProviders.confirmDeleteProvider', { name: provider.name }))) {
    return
  }

  try {
    await extauthAdminApi.deleteProvider(provider.id)
    providers.value = providers.value.filter(p => p.id !== provider.id)
    showSuccess(t('authProviders.providerDeleted'))
  } catch (e) {
    error.value = e instanceof Error ? e.message : t('authProviders.failedToDeleteProvider')
  }
}

function showSuccess(message: string) {
  successMessage.value = message
  setTimeout(() => {
    successMessage.value = null
  }, 3000)
}

function addAllowedDomain() {
  form.value.allowed_domains = [...(form.value.allowed_domains || []), '']
}

function removeAllowedDomain(index: number) {
  form.value.allowed_domains = form.value.allowed_domains?.filter((_, i) => i !== index)
}

function updateAllowedDomain(index: number, value: string) {
  if (form.value.allowed_domains) {
    form.value.allowed_domains[index] = value
  }
}

function addScope() {
  form.value.scopes = [...(form.value.scopes || []), '']
}

function removeScope(index: number) {
  form.value.scopes = form.value.scopes?.filter((_, i) => i !== index)
}

function updateScope(index: number, value: string) {
  if (form.value.scopes) {
    form.value.scopes[index] = value
  }
}
</script>

<template>
  <div class="auth-providers-view p-6 max-w-6xl mx-auto">
    <div class="flex items-center justify-between mb-6">
      <h1 class="text-2xl font-bold text-white">{{ t('authProviders.title') }}</h1>
      <button
        class="px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg text-sm transition-colors flex items-center gap-2"
        @click="openCreateModal"
      >
        <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
        </svg>
        {{ t('authProviders.addProvider') }}
      </button>
    </div>

    <!-- Success Message -->
    <div
      v-if="successMessage"
      class="fixed top-20 right-4 bg-green-600 text-white px-4 py-2 rounded-lg shadow-lg z-50"
    >
      {{ successMessage }}
    </div>

    <!-- Error Message -->
    <div
      v-if="error"
      class="mb-4 bg-red-900/50 border border-red-500 text-red-200 px-4 py-3 rounded-lg"
    >
      {{ error }}
    </div>

    <!-- Loading -->
    <div v-if="loading && providers.length === 0" class="text-gray-400 text-center py-8">
      {{ t('authProviders.loadingProviders') }}
    </div>

    <!-- Empty State -->
    <div v-else-if="providers.length === 0" class="bg-gray-700 rounded-lg p-8 text-center">
      <svg xmlns="http://www.w3.org/2000/svg" class="h-12 w-12 mx-auto text-gray-500 mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
      </svg>
      <h3 class="text-lg font-medium text-white mb-2">{{ t('authProviders.noProviders') }}</h3>
      <p class="text-gray-400 mb-4">{{ t('authProviders.noProvidersDesc') }}</p>
      <button
        class="px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg text-sm transition-colors"
        @click="openCreateModal"
      >
        {{ t('authProviders.addYourFirstProvider') }}
      </button>
    </div>

    <!-- Providers List -->
    <div v-else class="space-y-4">
      <div
        v-for="provider in providers"
        :key="provider.id"
        class="bg-gray-700 rounded-lg p-4"
      >
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-4">
            <!-- Status Indicator -->
            <div
              class="w-3 h-3 rounded-full"
              :class="provider.enabled ? 'bg-green-500' : 'bg-gray-500'"
            ></div>

            <!-- Provider Info -->
            <div>
              <div class="flex items-center gap-2">
                <h3 class="text-white font-medium">{{ provider.name }}</h3>
                <span class="text-xs bg-gray-700 px-2 py-0.5 rounded text-gray-300">
                  {{ getProviderDisplayName(provider.type) }}
                </span>
              </div>
              <div class="text-sm text-gray-400 mt-1">
                <span>ID: {{ provider.id }}</span>
                <span class="mx-2">|</span>
                <span>Client ID: {{ provider.client_id.substring(0, 20) }}...</span>
              </div>
              <div v-if="provider.issuer_url" class="text-sm text-gray-500 mt-1">
                {{ t('authProviders.issuer') }}: {{ provider.issuer_url }}
              </div>
            </div>
          </div>

          <!-- Actions -->
          <div class="flex items-center gap-2">
            <button
              class="px-3 py-1.5 text-sm rounded-lg transition-colors"
              :class="provider.enabled
                ? 'text-yellow-400 hover:bg-yellow-900/20'
                : 'text-green-400 hover:bg-green-900/20'"
              @click="toggleProvider(provider)"
            >
              {{ provider.enabled ? t('authProviders.disable') : t('common.enable') }}
            </button>
            <button
              class="px-3 py-1.5 text-sm text-gray-900 dark:text-white hover:bg-gray-200 dark:bg-gray-600/20 rounded-lg transition-colors"
              @click="openEditModal(provider)"
            >
              {{ t('common.edit') }}
            </button>
            <button
              class="px-3 py-1.5 text-sm text-red-400 hover:bg-red-900/20 rounded-lg transition-colors"
              @click="deleteProvider(provider)"
            >
              {{ t('common.delete') }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Create/Edit Modal -->
    <div
      v-if="showModal"
      class="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4"
      @click.self="closeModal"
    >
      <div class="bg-gray-700 rounded-lg max-w-2xl w-full max-h-[90vh] overflow-y-auto">
        <div class="p-6">
          <h3 class="text-lg font-semibold text-white mb-6">
            {{ isCreating ? 'Add Authentication Provider' : 'Edit Provider' }}
          </h3>

          <form class="space-y-6" @submit.prevent="saveProvider">
            <!-- Basic Info -->
            <div class="grid grid-cols-2 gap-4">
              <div>
                <label class="block text-sm text-gray-400 mb-2">Provider ID</label>
                <input
                  v-model="form.id"
                  type="text"
                  required
                  :disabled="!isCreating"
                  class="w-full bg-gray-700 text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400 disabled:opacity-50"
                  placeholder="e.g., google, github"
                  @input="onIdChange"
                />
              </div>
              <div>
                <label class="block text-sm text-gray-400 mb-2">Display Name</label>
                <input
                  v-model="form.name"
                  type="text"
                  required
                  class="w-full bg-gray-700 text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400"
                  placeholder="e.g., Google, GitHub"
                />
              </div>
            </div>

            <!-- Provider Type -->
            <div>
              <label class="block text-sm text-gray-400 mb-2">Provider Type</label>
              <select
                v-model="form.type"
                class="w-full bg-gray-700 text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400"
                @change="onTypeChange"
              >
                <option v-for="type in providerTypes" :key="type.value" :value="type.value">
                  {{ type.label }}
                </option>
              </select>
            </div>

            <!-- OAuth Credentials -->
            <div class="grid grid-cols-2 gap-4">
              <div>
                <label class="block text-sm text-gray-400 mb-2">Client ID</label>
                <input
                  v-model="form.client_id"
                  type="text"
                  required
                  class="w-full bg-gray-700 text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400"
                  placeholder="OAuth Client ID"
                />
              </div>
              <div>
                <label class="block text-sm text-gray-400 mb-2">
                  Client Secret
                  <span v-if="!isCreating" class="text-gray-500">(leave blank to keep current)</span>
                </label>
                <input
                  v-model="form.client_secret"
                  type="password"
                  :required="isCreating"
                  class="w-full bg-gray-700 text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400"
                  placeholder="OAuth Client Secret"
                />
              </div>
            </div>

            <!-- OIDC URLs -->
            <div>
              <label class="block text-sm text-gray-400 mb-2">
                Issuer URL
                <span class="text-gray-500">(for OIDC discovery)</span>
              </label>
              <input
                v-model="form.issuer_url"
                type="url"
                class="w-full bg-gray-700 text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400"
                placeholder="https://accounts.google.com"
              />
            </div>

            <div>
              <label class="block text-sm text-gray-400 mb-2">Redirect URL</label>
              <input
                v-model="form.redirect_url"
                type="url"
                required
                class="w-full bg-gray-700 text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400"
                placeholder="https://your-domain.com/auth/callback/provider-id"
              />
              <p class="text-xs text-gray-500 mt-1">Configure this URL in your OAuth provider's settings</p>
            </div>

            <!-- Scopes -->
            <div>
              <div class="flex items-center justify-between mb-2">
                <label class="text-sm text-gray-400">Scopes</label>
                <button
                  type="button"
                  class="text-xs text-gray-900 dark:text-white hover:text-gray-900 dark:text-white"
                  @click="addScope"
                >
                  + Add Scope
                </button>
              </div>
              <div class="space-y-2">
                <div v-for="(scope, index) in form.scopes" :key="index" class="flex gap-2">
                  <input
                    :value="scope"
                    type="text"
                    class="flex-1 bg-gray-700 text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400"
                    :placeholder="t('authProviders.placeholderScopes')"
                    @input="updateScope(index, ($event.target as HTMLInputElement).value)"
                  />
                  <button
                    type="button"
                    class="p-2 text-red-400 hover:bg-red-900/20 rounded-lg"
                    @click="removeScope(index)"
                  >
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                    </svg>
                  </button>
                </div>
              </div>
            </div>

            <!-- User Settings -->
            <div class="grid grid-cols-2 gap-4">
              <div>
                <label class="flex items-center gap-2 cursor-pointer">
                  <input
                    v-model="form.auto_create_user"
                    type="checkbox"
                    class="w-4 h-4 rounded border-gray-600 bg-gray-700 text-gray-900 dark:text-white focus:ring-gray-900 dark:focus:ring-gray-400"
                  />
                  <span class="text-sm text-gray-300">{{ t('authProviders.autoCreateUser') }}</span>
                </label>
              </div>
              <div>
                <label class="block text-sm text-gray-400 mb-2">{{ t('authProviders.defaultRole') }}</label>
                <input
                  v-model="form.default_role"
                  type="text"
                  class="w-full bg-gray-700 text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400"
                  placeholder="user"
                />
              </div>
            </div>

            <!-- Allowed Domains -->
            <div>
              <div class="flex items-center justify-between mb-2">
                <label class="text-sm text-gray-400">
                  {{ t('authProviders.allowedEmailDomains') }}
                  <span class="text-gray-500">{{ t('authProviders.leaveEmptyToAllowAll') }}</span>
                </label>
                <button
                  type="button"
                  class="text-xs text-gray-900 dark:text-white hover:text-gray-900 dark:text-white"
                  @click="addAllowedDomain"
                >
                  {{ t('authProviders.addDomain') }}
                </button>
              </div>
              <div class="space-y-2">
                <div v-for="(domain, index) in form.allowed_domains" :key="index" class="flex gap-2">
                  <input
                    :value="domain"
                    type="text"
                    class="flex-1 bg-gray-700 text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400"
                    :placeholder="t('authProviders.placeholderDomain')"
                    @input="updateAllowedDomain(index, ($event.target as HTMLInputElement).value)"
                  />
                  <button
                    type="button"
                    class="p-2 text-red-400 hover:bg-red-900/20 rounded-lg"
                    @click="removeAllowedDomain(index)"
                  >
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                    </svg>
                  </button>
                </div>
              </div>
            </div>

            <!-- Display Order -->
            <div>
              <label class="block text-sm text-gray-400 mb-2">{{ t('authProviders.displayOrder') }}</label>
              <input
                v-model.number="form.order"
                type="number"
                min="0"
                class="w-full bg-gray-700 text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400"
              />
            </div>

            <!-- Actions -->
            <div class="flex gap-3 pt-4">
              <button
                type="submit"
                :disabled="loading"
                class="flex-1 px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg transition-colors disabled:opacity-50"
              >
                {{ loading ? t('common.saving') : (isCreating ? t('authProviders.createProvider') : t('authProviders.saveChanges')) }}
              </button>
              <button
                type="button"
                class="px-4 py-2 bg-gray-600 hover:bg-gray-500 text-white rounded-lg transition-colors"
                @click="closeModal"
              >
                {{ t('common.cancel') }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  </div>
</template>
