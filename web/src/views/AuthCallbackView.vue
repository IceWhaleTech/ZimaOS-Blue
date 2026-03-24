<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { extauthApi } from '@/api/extauth'
import { setStoredAccessToken, setStoredRefreshToken } from '@/utils/authStorage'

const { t } = useI18n()
const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()

const status = ref<'loading' | 'success' | 'error'>('loading')
const errorMessage = ref('')
const providerName = ref('')

onMounted(async () => {
  const provider = route.params.provider as string
  const code = route.query.code as string
  const state = route.query.state as string
  const error = route.query.error as string
  const errorDescription = route.query.error_description as string

  providerName.value = provider

  // Check for OAuth error
  if (error) {
    status.value = 'error'
    errorMessage.value = errorDescription || error || 'Authentication failed'
    return
  }

  // Validate required params
  if (!code || !state) {
    status.value = 'error'
    errorMessage.value = 'Missing authorization code or state'
    return
  }

  try {
    // Exchange code for tokens
    const response = await extauthApi.exchangeToken(provider, code, state)
    const data = response.data

    // Store tokens
    setStoredAccessToken(data.access_token)
    if (data.refresh_token) {
      setStoredRefreshToken(data.refresh_token)
    }

    // Update auth store
    authStore.token = data.access_token
    await authStore.fetchUser()

    status.value = 'success'

    // Redirect after short delay
    const redirectTo = sessionStorage.getItem('oauth_redirect') || '/home'
    sessionStorage.removeItem('oauth_redirect')

    setTimeout(() => {
      router.push(redirectTo)
    }, 1500)
  } catch (e) {
    status.value = 'error'
    if ((e as { response?: { data?: { error?: string } } }).response?.data?.error) {
      errorMessage.value = (e as { response: { data: { error: string } } }).response.data.error
    } else {
      errorMessage.value = e instanceof Error ? e.message : 'Authentication failed'
    }
  }
})

function goToLogin() {
  router.push('/login')
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-gray-100 dark:bg-gray-700 px-4">
    <div class="max-w-md w-full text-center">
      <!-- Loading State -->
      <div v-if="status === 'loading'" class="space-y-6">
        <div
          class="inline-flex items-center justify-center w-16 h-16 rounded-full bg-gray-700 dark:bg-gray-500"
        >
          <svg
            class="animate-spin h-8 w-8 text-white"
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
        <div>
          <h2 class="text-xl font-semibold text-gray-900 dark:text-white">
            {{ t('auth.completingSignIn') }}
          </h2>
          <p class="text-gray-500 dark:text-gray-400 mt-2">{{ t('auth.verifyingCredentials') }}</p>
        </div>
      </div>

      <!-- Success State -->
      <div v-else-if="status === 'success'" class="space-y-6">
        <div class="inline-flex items-center justify-center w-16 h-16 rounded-full bg-green-600">
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-8 w-8 text-white"
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
        </div>
        <div>
          <h2 class="text-xl font-semibold text-gray-900 dark:text-white">
            {{ t('auth.signInSuccessful') }}
          </h2>
          <p class="text-gray-500 dark:text-gray-400 mt-2">{{ t('auth.redirecting') }}</p>
        </div>
      </div>

      <!-- Error State -->
      <div v-else-if="status === 'error'" class="space-y-6">
        <div class="inline-flex items-center justify-center w-16 h-16 rounded-full bg-red-600">
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-8 w-8 text-white"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M6 18L18 6M6 6l12 12"
            />
          </svg>
        </div>
        <div>
          <h2 class="text-xl font-semibold text-gray-900 dark:text-white">
            {{ t('auth.signInFailed') }}
          </h2>
          <p class="text-gray-500 dark:text-gray-400 mt-2">{{ errorMessage }}</p>
        </div>
        <button
          type="button"
          class="inline-flex items-center px-4 py-2 border border-transparent rounded-lg shadow-sm text-sm font-medium text-white bg-gray-700 dark:bg-gray-500 hover:bg-gray-700 dark:bg-gray-500 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-gray-900 dark:focus:ring-gray-400 transition-colors"
          @click="goToLogin"
        >
          {{ t('auth.backToLogin') }}
        </button>
      </div>
    </div>
  </div>
</template>
