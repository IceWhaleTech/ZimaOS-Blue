<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { usersApi, permissionsApi, PagePermissions, type User, type PermissionInfo } from '@/api/users'

const props = defineProps<{
  user: User
}>()

const emit = defineEmits<{
  close: []
  updated: []
}>()

const { t } = useI18n()

// Form state
const email = ref(props.user.email || '')
const role = ref<'user' | 'guest'>(props.user.role === 'admin' ? 'user' : props.user.role as 'user' | 'guest')
const selectedPermissions = ref<string[]>([])
const loading = ref(false)
const error = ref<string | null>(null)
const availablePermissions = ref<PermissionInfo[]>([])

// Password reset
const showPasswordReset = ref(false)
const newPassword = ref('')
const confirmPassword = ref('')
const showPassword = ref(false)
const passwordResetLoading = ref(false)

// Password strength indicators
const passwordChecks = computed(() => {
  const pwd = newPassword.value
  return {
    length: pwd.length >= 8,
    uppercase: /[A-Z]/.test(pwd),
    lowercase: /[a-z]/.test(pwd),
    number: /[0-9]/.test(pwd),
    special: /[!@#$%^&*()_+\-=[\]{};':"\\|,.<>/?]/.test(pwd),
  }
})

const passwordStrength = computed(() => {
  const checks = passwordChecks.value
  return Object.values(checks).filter(Boolean).length
})

const allPasswordChecksPassed = computed(() => {
  const checks = passwordChecks.value
  return checks.length && checks.uppercase && checks.lowercase && checks.number && checks.special
})

const passwordMismatch = computed(() => {
  return confirmPassword.value.length > 0 && newPassword.value !== confirmPassword.value
})

const isPasswordValid = computed(() => {
  return allPasswordChecksPassed.value && newPassword.value === confirmPassword.value
})

const isAdmin = computed(() => props.user.role === 'admin')

async function loadPermissions() {
  try {
    const response = await permissionsApi.getAvailablePermissions()
    availablePermissions.value = response.data.permissions
  } catch {
    availablePermissions.value = [
      { key: PagePermissions.CHAT, name: 'Chat', description: 'Access to chat', category: 'core' },
      { key: PagePermissions.HOME, name: 'Dashboard', description: 'Access to dashboard', category: 'core' },
      { key: PagePermissions.PROFILE, name: 'Profile', description: 'Access to profile', category: 'core' },
      { key: PagePermissions.CHANNELS, name: 'Channels', description: 'Access to channels', category: 'communication' },
      { key: PagePermissions.SETTINGS, name: 'Settings', description: 'Access to settings', category: 'admin' },
      { key: PagePermissions.SECURITY, name: 'Security', description: 'Access to security', category: 'admin' },
      { key: PagePermissions.AUTOMATION, name: 'Automation', description: 'Access to automation', category: 'advanced' },
      { key: PagePermissions.PLUGINS, name: 'Plugins', description: 'Access to plugins', category: 'advanced' },
      { key: PagePermissions.TOOLS, name: 'Tool Store', description: 'Access to tools', category: 'advanced' },
      { key: PagePermissions.SKILLS, name: 'Skill Store', description: 'Access to skills', category: 'advanced' },
    ]
  }

  // Load user's current permissions
  if (!isAdmin.value) {
    try {
      const response = await permissionsApi.getUserPermissions(props.user.id)
      selectedPermissions.value = response.data.permissions
    } catch {
      selectedPermissions.value = []
    }
  }
}

async function handleSubmit() {
  try {
    loading.value = true
    error.value = null

    await usersApi.update(props.user.id, {
      email: email.value || undefined,
      role: isAdmin.value ? undefined : role.value,
      permissions: isAdmin.value ? undefined : selectedPermissions.value,
    })

    emit('updated')
  } catch (e) {
    const axiosError = e as { response?: { data?: { message?: string } } }
    error.value = axiosError.response?.data?.message || t('users.error.updateFailed')
  } finally {
    loading.value = false
  }
}

async function handlePasswordReset() {
  if (!isPasswordValid.value) return

  try {
    passwordResetLoading.value = true
    error.value = null

    await usersApi.resetPassword(props.user.id, { new_password: newPassword.value })

    showPasswordReset.value = false
    newPassword.value = ''
    confirmPassword.value = ''
  } catch (e) {
    const axiosError = e as { response?: { data?: { message?: string } } }
    error.value = axiosError.response?.data?.message || t('users.error.passwordResetFailed')
  } finally {
    passwordResetLoading.value = false
  }
}

onMounted(() => {
  loadPermissions()
})
</script>

<template>
  <Teleport to="body">
    <div class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50">
      <div class="w-full max-w-lg bg-white dark:bg-gray-700 rounded-xl shadow-xl max-h-[90vh] overflow-y-auto">
        <!-- Header -->
        <div class="flex items-center justify-between px-6 py-4 border-b border-gray-200 dark:border-gray-700 sticky top-0 bg-white dark:bg-gray-700">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('users.editUser') }}: {{ user.username }}
          </h2>
          <button
            class="p-1 rounded-lg text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-700"
            @click="emit('close')"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <!-- Content -->
        <div class="px-6 py-4">
          <!-- Error message -->
          <div v-if="error" class="mb-4 p-3 bg-red-50 dark:bg-red-900/20 rounded-lg">
            <p class="text-sm text-red-600 dark:text-red-400">{{ error }}</p>
          </div>

          <!-- Admin notice -->
          <div v-if="isAdmin" class="mb-4 p-3 bg-purple-50 dark:bg-purple-900/20 rounded-lg">
            <p class="text-sm text-purple-600 dark:text-purple-400">
              {{ t('users.adminNotice') }}
            </p>
          </div>

          <!-- Form -->
          <form class="space-y-4" @submit.prevent="handleSubmit">
            <!-- Username (read-only) -->
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                {{ t('users.username') }}
              </label>
              <input
                :value="user.username"
                type="text"
                class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-gray-100 dark:bg-gray-700 text-gray-500 dark:text-gray-400"
                disabled
              />
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
              />
            </div>

            <!-- Role (not for admin) -->
            <div v-if="!isAdmin">
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                {{ t('users.role') }}
              </label>
              <select
                v-model="role"
                class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-gray-400 focus:border-transparent"
              >
                <option value="user">{{ t('users.roleUser') }}</option>
                <option value="guest">{{ t('users.roleGuest') }}</option>
              </select>
            </div>

            <!-- Permissions (not for admin) -->
            <div v-if="!isAdmin">
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                {{ t('users.permissions') }}
              </label>
              <div class="space-y-2 max-h-48 overflow-y-auto p-3 border border-gray-200 dark:border-gray-600 rounded-lg">
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
                  />
                  <div>
                    <span class="text-sm text-gray-900 dark:text-white">{{ perm.name }}</span>
                    <span class="text-xs text-gray-500 dark:text-gray-400 ml-2">{{ perm.description }}</span>
                  </div>
                </label>
              </div>
            </div>

            <!-- Password Reset Section -->
            <div class="border-t border-gray-200 dark:border-gray-700 pt-4">
              <button
                type="button"
                class="text-sm text-gray-900 dark:text-gray-300 hover:underline"
                @click="showPasswordReset = !showPasswordReset"
              >
                {{ showPasswordReset ? t('users.hidePasswordReset') : t('users.resetPassword') }}
              </button>

              <div v-if="showPasswordReset" class="mt-4 space-y-4 p-4 bg-gray-50 dark:bg-gray-700/50 rounded-lg">
                <!-- New Password -->
                <div>
                  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                    {{ t('users.newPassword') }}
                  </label>
                  <div class="relative">
                    <input
                      v-model="newPassword"
                      :type="showPassword ? 'text' : 'password'"
                      class="w-full px-3 py-2 pr-10 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-gray-400 focus:border-transparent"
                    />
                    <button
                      type="button"
                      class="absolute right-2 top-1/2 -translate-y-1/2 p-1 text-gray-500"
                      @click="showPassword = !showPassword"
                    >
                      <svg v-if="showPassword" xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21" />
                      </svg>
                      <svg v-else xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
                      </svg>
                    </button>
                  </div>
                  <!-- Password Strength -->
                  <div v-if="newPassword.length > 0" class="mt-2 space-y-2">
                    <div class="flex gap-1">
                      <div
                        v-for="i in 5"
                        :key="i"
                        class="h-1 flex-1 rounded-full transition-colors"
                        :class="i <= passwordStrength ? 'bg-green-500' : 'bg-gray-300 dark:bg-gray-600'"
                      />
                    </div>
                    <div class="grid grid-cols-2 gap-1 text-xs">
                      <span :class="passwordChecks.length ? 'text-green-500' : 'text-gray-400'">{{ passwordChecks.length ? '✓' : '○' }} 8+ chars</span>
                      <span :class="passwordChecks.uppercase ? 'text-green-500' : 'text-gray-400'">{{ passwordChecks.uppercase ? '✓' : '○' }} Uppercase</span>
                      <span :class="passwordChecks.lowercase ? 'text-green-500' : 'text-gray-400'">{{ passwordChecks.lowercase ? '✓' : '○' }} Lowercase</span>
                      <span :class="passwordChecks.number ? 'text-green-500' : 'text-gray-400'">{{ passwordChecks.number ? '✓' : '○' }} Number</span>
                      <span :class="passwordChecks.special ? 'text-green-500' : 'text-gray-400'" class="col-span-2">{{ passwordChecks.special ? '✓' : '○' }} Special char</span>
                    </div>
                  </div>
                </div>

                <!-- Confirm Password -->
                <div>
                  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                    {{ t('auth.confirmPassword') }}
                  </label>
                  <input
                    v-model="confirmPassword"
                    :type="showPassword ? 'text' : 'password'"
                    class="w-full px-3 py-2 border rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                    :class="passwordMismatch ? 'border-red-500' : 'border-gray-300 dark:border-gray-600'"
                  />
                  <p v-if="passwordMismatch" class="mt-1 text-sm text-red-500">
                    {{ t('auth.passwordMismatch') }}
                  </p>
                </div>

                <!-- Reset Button -->
                <button
                  type="button"
                  class="w-full px-4 py-2 text-sm font-medium text-white bg-orange-500 rounded-lg hover:bg-orange-600 transition-colors disabled:opacity-50"
                  :disabled="!isPasswordValid || passwordResetLoading"
                  @click="handlePasswordReset"
                >
                  <span v-if="passwordResetLoading">{{ t('common.saving') }}</span>
                  <span v-else>{{ t('users.resetPassword') }}</span>
                </button>
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
                class="flex-1 px-4 py-2 text-sm font-medium text-white bg-gray-700 dark:bg-gray-500 rounded-lg hover:bg-gray-800 dark:hover:bg-gray-400 transition-colors disabled:opacity-50"
                :disabled="loading"
              >
                <span v-if="loading">{{ t('common.saving') }}</span>
                <span v-else>{{ t('common.save') }}</span>
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  </Teleport>
</template>
