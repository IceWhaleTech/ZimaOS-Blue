<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { extauthApi, getProviderDisplayName } from '@/api/extauth'
import axios from 'axios'
import type { ProviderInfo, ProviderType } from '@/api/extauth'

const { t } = useI18n()
const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()

const username = ref('')
const password = ref('')
const showPassword = ref(false)
const rememberMe = ref(false)
const providers = ref<ProviderInfo[]>([])
const loadingProviders = ref(false)
const providerLoading = ref<string | null>(null)
const serverVersion = ref<string>('0.0.0')

const redirectTo = (route.query.redirect as string) || '/chat'

onMounted(async () => {
  // If already authenticated, redirect
  if (authStore.isAuthenticated) {
    router.push(redirectTo)
    return
  }

  // Load external auth providers and server version in parallel
  await Promise.all([loadProviders(), loadServerVersion()])
})

async function loadServerVersion() {
  try {
    // Use health API which is public and doesn't require auth
    const response = await axios.get<{ version: string }>('/api/v1/health')
    serverVersion.value = response.data.version
  } catch {
    // Silently fail - use default version
  }
}

async function loadProviders() {
  try {
    loadingProviders.value = true
    const response = await extauthApi.listProviders()
    providers.value = response.data.sort((a, b) => a.order - b.order)
  } catch {
    // Silently fail - providers are optional
    providers.value = []
  } finally {
    loadingProviders.value = false
  }
}

async function handleSubmit() {
  if (!username.value || !password.value) {
    return
  }

  const success = await authStore.login(username.value, password.value)
  if (success) {
    router.push(redirectTo)
  }
}

async function loginWithProvider(provider: ProviderInfo) {
  try {
    providerLoading.value = provider.id
    // Store redirect URL for after callback
    sessionStorage.setItem('oauth_redirect', redirectTo)
    // Redirect to provider's auth page
    const callbackUrl = `${window.location.origin}/auth/callback/${provider.id}`
    window.location.href = `/api/v1/auth/oidc/${provider.id}/authorize?redirect_uri=${encodeURIComponent(callbackUrl)}`
  } catch (e) {
    authStore.error = e instanceof Error ? e.message : 'Failed to start OAuth flow'
    providerLoading.value = null
  }
}

function toggleShowPassword() {
  showPassword.value = !showPassword.value
}

function getProviderName(provider: ProviderInfo): string {
  return provider.name || getProviderDisplayName(provider.type)
}

function getProviderIconSvg(type: ProviderType): string {
  // Return SVG path data for each provider
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
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-gray-100 dark:bg-gray-900 px-4">
    <div class="max-w-md w-full">
      <!-- Logo and Title -->
      <div class="text-center mb-8">
        <img src="/logo.png" alt="ZimaOS Echo" class="w-16 h-16 object-contain mx-auto mb-4 drop-shadow-lg" />
        <h1 class="text-2xl font-bold text-gray-900 dark:text-white">ZimaOS Echo</h1>
        <p class="text-gray-500 dark:text-gray-400 mt-2">{{ t('auth.signInToAccount') }}</p>
      </div>

      <!-- Login Form -->
      <div class="bg-white dark:bg-gray-800 rounded-lg p-6 shadow-xl">
        <form class="space-y-6" @submit.prevent="handleSubmit">
          <!-- Error Message -->
          <div
            v-if="authStore.error"
            class="bg-red-100 dark:bg-red-900/50 border border-red-300 dark:border-red-500 text-red-700 dark:text-red-200 px-4 py-3 rounded-lg text-sm"
          >
            {{ authStore.error }}
          </div>

          <!-- Username -->
          <div>
            <label for="username" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              {{ t('auth.username') }}
            </label>
            <div class="relative">
              <div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-5 w-5 text-gray-400 dark:text-gray-500"
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
              </div>
              <input
                id="username"
                v-model="username"
                type="text"
                autocomplete="username"
                required
                class="block w-full pl-10 pr-3 py-3 bg-gray-100 dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-lg text-gray-900 dark:text-white placeholder-gray-500 dark:placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                :placeholder="t('auth.enterUsername')"
              />
            </div>
          </div>

          <!-- Password -->
          <div>
            <label for="password" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              {{ t('auth.password') }}
            </label>
            <div class="relative">
              <div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-5 w-5 text-gray-400 dark:text-gray-500"
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
              </div>
              <input
                id="password"
                v-model="password"
                :type="showPassword ? 'text' : 'password'"
                autocomplete="current-password"
                required
                class="block w-full pl-10 pr-10 py-3 bg-gray-100 dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-lg text-gray-900 dark:text-white placeholder-gray-500 dark:placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                :placeholder="t('auth.enterPassword')"
              />
              <button
                type="button"
                class="absolute inset-y-0 right-0 pr-3 flex items-center"
                @click="toggleShowPassword"
              >
                <svg
                  v-if="showPassword"
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-5 w-5 text-gray-500 dark:text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21"
                  />
                </svg>
                <svg
                  v-else
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-5 w-5 text-gray-500 dark:text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
                  />
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"
                  />
                </svg>
              </button>
            </div>
          </div>

          <!-- Remember Me -->
          <div class="flex items-center justify-between">
            <label class="flex items-center">
              <input
                v-model="rememberMe"
                type="checkbox"
                class="w-4 h-4 rounded border-gray-300 dark:border-gray-600 bg-gray-100 dark:bg-gray-700 text-blue-600 focus:ring-blue-500 focus:ring-offset-white dark:focus:ring-offset-gray-800"
              />
              <span class="ml-2 text-sm text-gray-700 dark:text-gray-300">{{ t('auth.rememberMe') }}</span>
            </label>
          </div>

          <!-- Submit Button -->
          <button
            type="submit"
            :disabled="authStore.loading || !username || !password"
            class="w-full flex justify-center items-center py-3 px-4 border border-transparent rounded-lg shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            <svg
              v-if="authStore.loading"
              class="animate-spin -ml-1 mr-3 h-5 w-5 text-white"
              xmlns="http://www.w3.org/2000/svg"
              fill="none"
              viewBox="0 0 24 24"
            >
              <circle
                class="opacity-25"
                cx="12"
                cy="12"
                r="10"
                stroke="currentColor"
                stroke-width="4"
              ></circle>
              <path
                class="opacity-75"
                fill="currentColor"
                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
              ></path>
            </svg>
            {{ authStore.loading ? t('auth.signingIn') : t('auth.signIn') }}
          </button>
        </form>

        <!-- Divider -->
        <div v-if="providers.length > 0" class="relative my-6">
          <div class="absolute inset-0 flex items-center">
            <div class="w-full border-t border-gray-300 dark:border-gray-600"></div>
          </div>
          <div class="relative flex justify-center text-sm">
            <span class="px-2 bg-white dark:bg-gray-800 text-gray-500 dark:text-gray-400">{{ t('auth.orContinueWith') }}</span>
          </div>
        </div>

        <!-- External Auth Providers -->
        <div v-if="providers.length > 0" class="space-y-3">
          <button
            v-for="provider in providers"
            :key="provider.id"
            type="button"
            :disabled="providerLoading !== null"
            class="w-full flex items-center justify-center gap-3 py-3 px-4 border border-gray-300 dark:border-gray-600 rounded-lg text-sm font-medium text-gray-700 dark:text-gray-200 bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-gray-500 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            @click="loginWithProvider(provider)"
          >
            <svg
              v-if="providerLoading === provider.id"
              class="animate-spin h-5 w-5 text-gray-300"
              xmlns="http://www.w3.org/2000/svg"
              fill="none"
              viewBox="0 0 24 24"
            >
              <circle
                class="opacity-25"
                cx="12"
                cy="12"
                r="10"
                stroke="currentColor"
                stroke-width="4"
              ></circle>
              <path
                class="opacity-75"
                fill="currentColor"
                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
              ></path>
            </svg>
            <template v-else>
              <img
                v-if="provider.icon_url"
                :src="provider.icon_url"
                :alt="getProviderName(provider)"
                class="h-5 w-5"
              />
              <svg
                v-else
                class="h-5 w-5"
                viewBox="0 0 24 24"
                fill="currentColor"
              >
                <path :d="getProviderIconSvg(provider.type)" />
              </svg>
            </template>
            <span>{{ providerLoading === provider.id ? t('common.loading') : t('auth.signInWith', { provider: getProviderName(provider) }) }}</span>
          </button>
        </div>

        <!-- Loading providers -->
        <div v-if="loadingProviders" class="flex justify-center py-4">
          <svg
            class="animate-spin h-5 w-5 text-gray-500 dark:text-gray-400"
            xmlns="http://www.w3.org/2000/svg"
            fill="none"
            viewBox="0 0 24 24"
          >
            <circle
              class="opacity-25"
              cx="12"
              cy="12"
              r="10"
              stroke="currentColor"
              stroke-width="4"
            ></circle>
            <path
              class="opacity-75"
              fill="currentColor"
              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
            ></path>
          </svg>
        </div>
      </div>

      <!-- Footer -->
      <p class="mt-6 text-center text-sm text-gray-500 dark:text-gray-500">
        {{ t('footer.version', { version: serverVersion }) }}
      </p>
    </div>
  </div>
</template>
