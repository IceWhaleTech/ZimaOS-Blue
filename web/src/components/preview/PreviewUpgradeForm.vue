<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { usePreviewStore } from '@/stores/preview'
import { useAuthStore } from '@/stores/auth'
import { authApi, type PasswordPolicy } from '@/api/auth'
import { getUserErrorMessage } from '@/utils/userErrors'
import { buildPasswordChecks } from '@/utils/passwordPolicy'

const emit = defineEmits<{
  close: []
  success: []
}>()

const { t } = useI18n()
const previewStore = usePreviewStore()
const authStore = useAuthStore()

// Form state
const username = ref('')
const password = ref('')
const confirmPassword = ref('')
const showPassword = ref(false)
const loading = ref(false)
const error = ref<string | null>(null)
const policyLoading = ref(true)

// Password policy from backend
const policy = ref<PasswordPolicy>({
  min_length: 8,
  require_uppercase: true,
  require_lowercase: true,
  require_letter: false,
  require_number: true,
  require_special: true,
})

const passwordPlaceholder = computed(() =>
  t('preview.passwordCheck.length', { n: policy.value.min_length })
)

onMounted(async () => {
  try {
    const res = await authApi.getPasswordPolicy()
    policy.value = res.data
  } catch {
    // fallback to defaults
  } finally {
    policyLoading.value = false
  }
})

// Validation
const requiresPasswordConfirmation = computed(() => !showPassword.value)

const isValid = computed(() => {
  const baseValid =
    !policyLoading.value && username.value.length >= 3 && allPasswordChecksPassed.value
  return (
    baseValid && (!requiresPasswordConfirmation.value || password.value === confirmPassword.value)
  )
})

const passwordMismatch = computed(() => {
  return (
    requiresPasswordConfirmation.value &&
    confirmPassword.value.length > 0 &&
    password.value !== confirmPassword.value
  )
})

// Password strength indicators
const passwordChecks = computed(() => {
  return buildPasswordChecks(password.value, policy.value, t)
})

const passwordStrength = computed(() => {
  return passwordChecks.value.filter((check) => check.passed).length
})

const allPasswordChecksPassed = computed(() => {
  return passwordChecks.value.every((check) => check.passed)
})

async function handleSubmit() {
  if (!isValid.value) return

  try {
    loading.value = true
    error.value = null

    const result = await previewStore.upgrade(username.value, password.value)

    if (result) {
      // Auto login with the new credentials
      const loginSuccess = await authStore.login(username.value, password.value)
      if (loginSuccess) {
        emit('success')
      } else {
        // If auto-login fails, still consider upgrade successful
        emit('success')
      }
    }
  } catch (e) {
    const axiosError = e as { response?: { data?: { message?: string } } }
    error.value = getUserErrorMessage(axiosError.response?.data?.message, 'preview.upgradeFailed')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <Teleport to="body">
    <div class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50">
      <div class="w-full max-w-md bg-white dark:bg-gray-700 rounded-xl shadow-xl">
        <!-- Header -->
        <div
          class="flex items-center justify-between px-6 py-4 border-b border-gray-200 dark:border-gray-700"
        >
          <div class="flex items-center space-x-3">
            <span class="text-2xl">🎉</span>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('preview.createAdminAccount') }}
            </h2>
          </div>
          <button
            class="p-1 rounded-lg text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-700"
            @click="emit('close')"
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
                d="M6 18L18 6M6 6l12 12"
              />
            </svg>
          </button>
        </div>

        <!-- Content -->
        <div class="px-6 py-4">
          <!-- Benefits -->
          <div class="mb-6 p-4 bg-green-50 dark:bg-green-900/20 rounded-lg">
            <p class="text-sm font-medium text-green-800 dark:text-green-300 mb-2">
              {{ t('preview.upgradeTitle') }}
            </p>
            <ul class="space-y-1 text-sm text-green-700 dark:text-green-400">
              <li class="flex items-center space-x-2">
                <svg class="h-4 w-4" fill="currentColor" viewBox="0 0 20 20">
                  <path
                    fill-rule="evenodd"
                    d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z"
                    clip-rule="evenodd"
                  />
                </svg>
                <span>{{ t('preview.benefit1') }}</span>
              </li>
              <li class="flex items-center space-x-2">
                <svg class="h-4 w-4" fill="currentColor" viewBox="0 0 20 20">
                  <path
                    fill-rule="evenodd"
                    d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z"
                    clip-rule="evenodd"
                  />
                </svg>
                <span>{{ t('preview.benefit2') }}</span>
              </li>
              <li class="flex items-center space-x-2">
                <svg class="h-4 w-4" fill="currentColor" viewBox="0 0 20 20">
                  <path
                    fill-rule="evenodd"
                    d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z"
                    clip-rule="evenodd"
                  />
                </svg>
                <span>{{ t('preview.benefit3') }}</span>
              </li>
            </ul>
          </div>

          <!-- Error message -->
          <div v-if="error" class="mb-4 p-3 bg-red-50 dark:bg-red-900/20 rounded-lg">
            <p class="text-sm text-red-600 dark:text-red-400">{{ error }}</p>
          </div>

          <!-- Form -->
          <form class="space-y-4" @submit.prevent="handleSubmit">
            <!-- Username -->
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                {{ t('auth.username') }}
              </label>
              <input
                v-model="username"
                type="text"
                class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-gray-400 focus:border-transparent"
                :placeholder="t('auth.usernamePlaceholder')"
                required
                minlength="3"
              />
            </div>

            <!-- Password -->
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                {{ t('auth.password') }}
              </label>
              <div class="relative">
                <input
                  v-model="password"
                  :type="showPassword ? 'text' : 'password'"
                  class="w-full px-3 py-2 pr-10 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-gray-400 focus:border-transparent"
                  :placeholder="passwordPlaceholder"
                  required
                  :minlength="policy.min_length"
                />
                <button
                  type="button"
                  class="absolute right-2 top-1/2 -translate-y-1/2 p-1 text-gray-500 hover:text-gray-700 dark:hover:text-gray-300"
                  @click="showPassword = !showPassword"
                >
                  <svg
                    v-if="showPassword"
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
                      d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21"
                    />
                  </svg>
                  <svg
                    v-else
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
              <div v-if="!policyLoading && password.length > 0" class="mt-2 space-y-2">
                <div class="flex gap-1">
                  <div
                    v-for="i in passwordChecks.length"
                    :key="i"
                    class="h-1 flex-1 rounded-full transition-colors"
                    :class="i <= passwordStrength ? 'bg-green-500' : 'bg-gray-300 dark:bg-gray-600'"
                  />
                </div>
                <div class="grid grid-cols-2 gap-x-4 gap-y-1 text-xs">
                  <div
                    v-for="check in passwordChecks"
                    :key="check.key"
                    class="flex items-center gap-1.5"
                  >
                    <span
                      :class="check.passed ? 'text-green-500' : 'text-gray-400 dark:text-gray-500'"
                    >
                      {{ check.passed ? '✓' : '○' }}
                    </span>
                    <span
                      :class="
                        check.passed
                          ? 'text-green-600 dark:text-green-400'
                          : 'text-gray-500 dark:text-gray-400'
                      "
                    >
                      {{ check.label }}
                    </span>
                  </div>
                </div>
              </div>
            </div>

            <!-- Confirm Password -->
            <div v-if="requiresPasswordConfirmation">
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                {{ t('auth.confirmPassword') }}
              </label>
              <input
                v-model="confirmPassword"
                :type="showPassword ? 'text' : 'password'"
                class="w-full px-3 py-2 border rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-gray-400 focus:border-transparent"
                :class="
                  passwordMismatch ? 'border-red-500' : 'border-gray-300 dark:border-gray-600'
                "
                :placeholder="t('auth.confirmPasswordPlaceholder')"
                required
              />
              <p v-if="passwordMismatch" class="mt-1 text-sm text-red-500">
                {{ t('auth.passwordMismatch') }}
              </p>
            </div>

            <!-- Actions -->
            <div class="flex space-x-3 pt-2">
              <button
                type="button"
                class="flex-1 px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors"
                @click="emit('close')"
              >
                {{ t('common.cancel') }}
              </button>
              <button
                type="submit"
                class="flex-1 px-4 py-2 text-sm font-medium text-white bg-gray-700 dark:bg-gray-500 rounded-lg hover:bg-gray-800 dark:hover:bg-gray-400 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                :disabled="!isValid || loading"
              >
                <span v-if="loading" class="flex items-center justify-center space-x-2">
                  <svg
                    class="animate-spin h-4 w-4"
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
                  <span>{{ t('common.creating') }}</span>
                </span>
                <span v-else>{{ t('preview.createAccount') }}</span>
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  </Teleport>
</template>
