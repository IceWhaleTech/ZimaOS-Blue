<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { usersApi, permissionsApi, PagePermissions, type PermissionInfo } from '@/api/users'
import { authApi, type PasswordPolicy } from '@/api/auth'
import { getUserErrorMessage } from '@/utils/userErrors'
import { buildPasswordChecks } from '@/utils/passwordPolicy'

const emit = defineEmits<{
  close: []
  created: []
}>()

const { t } = useI18n()

// Form state
const username = ref('')
const email = ref('')
const password = ref('')
const confirmPassword = ref('')
const role = ref<'user' | 'guest'>('user')
const selectedPermissions = ref<string[]>([])
const showPassword = ref(false)
const loading = ref(false)
const error = ref<string | null>(null)
const availablePermissions = ref<PermissionInfo[]>([])
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

const requiresPasswordConfirmation = computed(() => !showPassword.value)

const passwordMismatch = computed(() => {
  return (
    requiresPasswordConfirmation.value &&
    confirmPassword.value.length > 0 &&
    password.value !== confirmPassword.value
  )
})

const isValid = computed(() => {
  const baseValid =
    !policyLoading.value && username.value.length >= 3 && allPasswordChecksPassed.value
  return (
    baseValid && (!requiresPasswordConfirmation.value || password.value === confirmPassword.value)
  )
})

// Default permissions based on role
const defaultPermissions: Record<string, string[]> = {
  user: [PagePermissions.CHAT, PagePermissions.PROFILE, PagePermissions.HOME],
  guest: [PagePermissions.CHAT],
}

function onRoleChange() {
  selectedPermissions.value = [...(defaultPermissions[role.value] || [])]
}

async function loadPermissions() {
  try {
    const response = await permissionsApi.getAvailablePermissions()
    availablePermissions.value = response.data.permissions
  } catch {
    // Use default list if API fails
    availablePermissions.value = [
      { key: PagePermissions.CHAT, name: 'Chat', description: 'Access to chat', category: 'core' },
      {
        key: PagePermissions.HOME,
        name: 'Dashboard',
        description: 'Access to dashboard',
        category: 'core',
      },
      {
        key: PagePermissions.PROFILE,
        name: 'Profile',
        description: 'Access to profile',
        category: 'core',
      },
      {
        key: PagePermissions.CHANNELS,
        name: 'Channels',
        description: 'Access to channels',
        category: 'communication',
      },
      {
        key: PagePermissions.SETTINGS,
        name: 'Settings',
        description: 'Access to settings',
        category: 'admin',
      },
      {
        key: PagePermissions.SECURITY,
        name: 'Security',
        description: 'Access to security',
        category: 'admin',
      },
      {
        key: PagePermissions.AUTOMATION,
        name: 'Operations',
        description: 'Access to operations',
        category: 'advanced',
      },
      {
        key: PagePermissions.PLUGINS,
        name: 'Plugins',
        description: 'Access to plugins',
        category: 'advanced',
      },
      {
        key: PagePermissions.TOOLS,
        name: 'Tool Store',
        description: 'Access to tools',
        category: 'advanced',
      },
      {
        key: PagePermissions.SKILLS,
        name: 'Skill Store',
        description: 'Access to skills',
        category: 'advanced',
      },
    ]
  }
}

async function handleSubmit() {
  if (!isValid.value) return

  try {
    loading.value = true
    error.value = null

    await usersApi.create({
      username: username.value,
      email: email.value || undefined,
      password: password.value,
      role: role.value,
      permissions: selectedPermissions.value,
    })

    emit('created')
  } catch (e) {
    const axiosError = e as { response?: { data?: { message?: string } } }
    error.value = getUserErrorMessage(
      axiosError.response?.data?.message,
      'users.error.createFailed'
    )
  } finally {
    loading.value = false
  }
}

// Initialize
onRoleChange()
loadPermissions()
</script>

<template>
  <Teleport to="body">
    <div class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50">
      <div
        class="w-full max-w-lg bg-white dark:bg-gray-700 rounded-xl shadow-xl max-h-[90vh] overflow-y-auto"
      >
        <!-- Header -->
        <div
          class="flex items-center justify-between px-6 py-4 border-b border-gray-200 dark:border-gray-700 sticky top-0 bg-white dark:bg-gray-700"
        >
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('users.addUser') }}
          </h2>
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
          <!-- Error message -->
          <div
            v-if="error"
            class="mb-4 p-3 bg-red-50 dark:bg-red-900/20 rounded-lg"
          >
            <p class="text-sm text-red-600 dark:text-red-400">
              {{ error }}
            </p>
          </div>

          <!-- Form -->
          <form
            class="space-y-4"
            @submit.prevent="handleSubmit"
          >
            <!-- Username -->
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                {{ t('users.username') }} *
              </label>
              <input
                v-model="username"
                type="text"
                class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-gray-400 focus:border-transparent"
                :placeholder="t('users.usernamePlaceholder')"
                required
                minlength="3"
              >
            </div>

            <!-- Email -->
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                {{ t('users.email') }}
              </label>
              <input
                v-model="email"
                type="email"
                class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-gray-400 focus:border-transparent"
                :placeholder="t('users.emailPlaceholder')"
              >
            </div>

            <!-- Role -->
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                {{ t('users.role') }}
              </label>
              <select
                v-model="role"
                class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-gray-400 focus:border-transparent"
                @change="onRoleChange"
              >
                <option value="user">
                  {{ t('users.roleUser') }}
                </option>
                <option value="guest">
                  {{ t('users.roleGuest') }}
                </option>
              </select>
            </div>

            <!-- Password -->
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                {{ t('auth.password') }} *
              </label>
              <div class="relative">
                <input
                  v-model="password"
                  :type="showPassword ? 'text' : 'password'"
                  class="w-full px-3 py-2 pe-10 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-gray-400 focus:border-transparent"
                  :minlength="policy.min_length"
                  required
                >
                <button
                  type="button"
                  class="absolute end-2 top-1/2 -translate-y-1/2 p-1 text-gray-500 hover:text-gray-700 dark:hover:text-gray-300"
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
              <!-- Password Strength Indicator -->
              <div
                v-if="!policyLoading && password.length > 0"
                class="mt-2 space-y-2"
              >
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
                    <span :class="check.passed ? 'text-green-500' : 'text-gray-400'">
                      {{ check.passed ? '✓' : '○' }}
                    </span>
                    <span
                      :class="check.passed ? 'text-green-600 dark:text-green-400' : 'text-gray-500'"
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
                {{ t('auth.confirmPassword') }} *
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
              >
              <p
                v-if="passwordMismatch"
                class="mt-1 text-sm text-red-500"
              >
                {{ t('auth.passwordMismatch') }}
              </p>
            </div>

            <!-- Permissions -->
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                {{ t('users.permissions') }}
              </label>
              <div
                class="space-y-2 max-h-48 overflow-y-auto p-3 border border-gray-200 dark:border-gray-600 rounded-lg"
              >
                <label
                  v-for="perm in availablePermissions"
                  :key="perm.key"
                  class="flex items-center gap-3 cursor-pointer"
                >
                  <input
                    v-model="selectedPermissions"
                    type="checkbox"
                    :value="perm.key"
                    class="w-4 h-4 text-gray-900 dark:text-gray-300 border-gray-300 rounded focus:ring-gray-400"
                  >
                  <div>
                    <span class="text-sm text-gray-900 dark:text-white">{{
                      t(`users.pagePermissions.${perm.key}`, perm.name)
                    }}</span>
                    <span class="text-xs text-gray-500 dark:text-gray-400 ms-2">{{
                      t(`users.pagePermissionDesc.${perm.key}`, perm.description)
                    }}</span>
                  </div>
                </label>
              </div>
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
                <span
                  v-if="loading"
                  class="flex items-center justify-center space-x-2"
                >
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
                    />
                    <path
                      class="opacity-75"
                      fill="currentColor"
                      d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                    />
                  </svg>
                  <span>{{ t('common.creating') }}</span>
                </span>
                <span v-else>{{ t('users.createUser') }}</span>
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  </Teleport>
</template>
