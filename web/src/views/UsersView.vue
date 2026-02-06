<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { usersApi, type User, type ListUsersParams } from '@/api/users'
import CreateUserModal from '@/components/users/CreateUserModal.vue'
import EditUserModal from '@/components/users/EditUserModal.vue'

const { t } = useI18n()

// State
const users = ref<User[]>([])
const loading = ref(false)
const error = ref<string | null>(null)
const searchQuery = ref('')
const statusFilter = ref<string>('')
const roleFilter = ref<string>('')
const currentPage = ref(1)
const pageSize = ref(10)
const totalUsers = ref(0)
const totalPages = ref(0)

// Modals
const showCreateModal = ref(false)
const showEditModal = ref(false)
const selectedUser = ref<User | null>(null)

// Computed
const filteredParams = computed<ListUsersParams>(() => ({
  page: currentPage.value,
  page_size: pageSize.value,
  search: searchQuery.value || undefined,
  status: statusFilter.value || undefined,
  role: roleFilter.value || undefined,
}))

// Actions
async function fetchUsers() {
  try {
    loading.value = true
    error.value = null
    const response = await usersApi.list(filteredParams.value)
    users.value = response.data.users
    totalUsers.value = response.data.total
    totalPages.value = response.data.total_pages
  } catch (e) {
    error.value = t('users.error.loadFailed')
    console.error('Failed to fetch users:', e)
  } finally {
    loading.value = false
  }
}

async function handleLockUser(user: User) {
  if (!confirm(t('users.confirmLock', { username: user.username }))) return

  try {
    await usersApi.lock(user.id)
    await fetchUsers()
  } catch (_e) {
    error.value = t('users.error.lockFailed')
  }
}

async function handleUnlockUser(user: User) {
  try {
    await usersApi.unlock(user.id)
    await fetchUsers()
  } catch (_e) {
    error.value = t('users.error.unlockFailed')
  }
}

async function handleDeleteUser(user: User) {
  if (!confirm(t('users.confirmDelete', { username: user.username }))) return

  try {
    await usersApi.delete(user.id)
    await fetchUsers()
  } catch (_e) {
    error.value = t('users.error.deleteFailed')
  }
}

function openEditModal(user: User) {
  selectedUser.value = user
  showEditModal.value = true
}

function handleUserCreated() {
  showCreateModal.value = false
  fetchUsers()
}

function handleUserUpdated() {
  showEditModal.value = false
  selectedUser.value = null
  fetchUsers()
}

function formatDate(dateStr?: string) {
  if (!dateStr) return '-'
  return new Date(dateStr).toLocaleDateString()
}

function formatLastLogin(dateStr?: string) {
  if (!dateStr) return t('users.neverLoggedIn')
  const date = new Date(dateStr)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffHours = Math.floor(diffMs / (1000 * 60 * 60))
  const diffDays = Math.floor(diffHours / 24)

  if (diffHours < 1) return t('users.justNow')
  if (diffHours < 24) return t('users.hoursAgo', { hours: diffHours })
  if (diffDays < 7) return t('users.daysAgo', { days: diffDays })
  return date.toLocaleDateString()
}

function getStatusClass(status: string) {
  switch (status) {
    case 'active':
      return 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400'
    case 'locked':
      return 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400'
    case 'disabled':
      return 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-400'
    default:
      return 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-400'
  }
}

function getRoleClass(role: string) {
  switch (role) {
    case 'admin':
      return 'bg-purple-100 text-purple-800 dark:bg-purple-900/30 dark:text-purple-400'
    case 'user':
      return 'bg-gray-700 dark:bg-gray-700 text-gray-900 dark:text-white dark:bg-gray-700 dark:bg-gray-700/30 dark:text-gray-900 dark:text-white'
    case 'guest':
      return 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-400'
    default:
      return 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-400'
  }
}

onMounted(() => {
  fetchUsers()
})
</script>

<template>
  <div class="p-6 max-w-6xl mx-auto">
    <!-- Header -->
    <div class="flex items-center justify-between mb-6">
      <div>
        <h1 class="text-2xl font-bold text-gray-900 dark:text-white">
          {{ t('users.title') }}
        </h1>
        <p class="text-sm text-gray-500 dark:text-gray-400 mt-1">
          {{ t('users.description') }}
        </p>
      </div>
      <button
        class="px-4 py-2 bg-gray-700 dark:bg-gray-700 text-white rounded-lg hover:bg-gray-700 dark:bg-gray-700/90 transition-colors flex items-center gap-2"
        @click="showCreateModal = true"
      >
        <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
        </svg>
        {{ t('users.addUser') }}
      </button>
    </div>

    <!-- Filters -->
    <div class="flex flex-wrap gap-4 mb-6">
      <div class="flex-1 min-w-[200px]">
        <input
          v-model="searchQuery"
          type="text"
          :placeholder="t('users.searchPlaceholder')"
          class="w-full px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-gray-400 focus:border-transparent"
          @input="currentPage = 1; fetchUsers()"
        />
      </div>
      <select
        v-model="statusFilter"
        class="px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-gray-400 focus:border-transparent"
        @change="currentPage = 1; fetchUsers()"
      >
        <option value="">{{ t('users.allStatuses') }}</option>
        <option value="active">{{ t('users.statusActive') }}</option>
        <option value="locked">{{ t('users.statusLocked') }}</option>
        <option value="disabled">{{ t('users.statusDisabled') }}</option>
      </select>
      <select
        v-model="roleFilter"
        class="px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-gray-400 focus:border-transparent"
        @change="currentPage = 1; fetchUsers()"
      >
        <option value="">{{ t('users.allRoles') }}</option>
        <option value="admin">{{ t('users.roleAdmin') }}</option>
        <option value="user">{{ t('users.roleUser') }}</option>
        <option value="guest">{{ t('users.roleGuest') }}</option>
      </select>
    </div>

    <!-- Error -->
    <div v-if="error" class="mb-4 p-4 bg-red-50 dark:bg-red-900/20 rounded-lg">
      <p class="text-sm text-red-600 dark:text-red-400">{{ error }}</p>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="flex justify-center py-12">
      <svg class="animate-spin h-8 w-8 text-gray-900 dark:text-gray-300" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
      </svg>
    </div>

    <!-- Users Table -->
    <div v-else-if="users.length > 0" class="bg-white dark:bg-gray-700 rounded-xl shadow overflow-hidden">
      <table class="w-full">
        <thead class="bg-gray-50 dark:bg-gray-700">
          <tr>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
              {{ t('users.username') }}
            </th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
              {{ t('users.role') }}
            </th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
              {{ t('users.status') }}
            </th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
              {{ t('users.lastLogin') }}
            </th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
              {{ t('users.createdAt') }}
            </th>
            <th class="px-6 py-3 text-right text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
              {{ t('users.actions') }}
            </th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
          <tr v-for="user in users" :key="user.id" class="hover:bg-gray-50 dark:hover:bg-gray-700/50">
            <td class="px-6 py-4 whitespace-nowrap">
              <div class="flex items-center">
                <div class="h-10 w-10 rounded-full bg-gray-700 dark:bg-gray-700/20 flex items-center justify-center">
                  <span class="text-gray-900 dark:text-gray-300 font-medium">{{ user.username.charAt(0).toUpperCase() }}</span>
                </div>
                <div class="ml-4">
                  <div class="text-sm font-medium text-gray-900 dark:text-white">{{ user.username }}</div>
                  <div class="text-sm text-gray-500 dark:text-gray-400">{{ user.email || '-' }}</div>
                </div>
              </div>
            </td>
            <td class="px-6 py-4 whitespace-nowrap">
              <span
                class="px-2 py-1 text-xs font-medium rounded-full"
                :class="getRoleClass(user.role)"
              >
                {{ t(`users.role${user.role.charAt(0).toUpperCase() + user.role.slice(1)}`) }}
              </span>
            </td>
            <td class="px-6 py-4 whitespace-nowrap">
              <span
                class="px-2 py-1 text-xs font-medium rounded-full"
                :class="getStatusClass(user.status)"
              >
                {{ t(`users.status${user.status.charAt(0).toUpperCase() + user.status.slice(1)}`) }}
              </span>
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
              {{ formatLastLogin(user.last_login_at) }}
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
              {{ formatDate(user.created_at) }}
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
              <div class="flex items-center justify-end gap-2">
                <button
                  class="p-2 text-gray-500 hover:text-gray-900 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
                  :title="t('users.edit')"
                  @click="openEditModal(user)"
                >
                  <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                  </svg>
                </button>
                <button
                  v-if="user.status === 'active' && user.role !== 'admin'"
                  class="p-2 text-gray-500 hover:text-orange-500 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
                  :title="t('users.lock')"
                  @click="handleLockUser(user)"
                >
                  <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
                  </svg>
                </button>
                <button
                  v-if="user.status === 'locked'"
                  class="p-2 text-gray-500 hover:text-green-500 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
                  :title="t('users.unlock')"
                  @click="handleUnlockUser(user)"
                >
                  <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 11V7a4 4 0 118 0m-4 8v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2z" />
                  </svg>
                </button>
                <button
                  v-if="user.role !== 'admin'"
                  class="p-2 text-gray-500 hover:text-red-500 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
                  :title="t('users.delete')"
                  @click="handleDeleteUser(user)"
                >
                  <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                  </svg>
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>

      <!-- Pagination -->
      <div v-if="totalPages > 1" class="px-6 py-4 border-t border-gray-200 dark:border-gray-700 flex items-center justify-between">
        <div class="text-sm text-gray-500 dark:text-gray-400">
          {{ t('users.showing', { from: (currentPage - 1) * pageSize + 1, to: Math.min(currentPage * pageSize, totalUsers), total: totalUsers }) }}
        </div>
        <div class="flex gap-2">
          <button
            class="px-3 py-1 rounded border border-gray-300 dark:border-gray-600 text-sm disabled:opacity-50"
            :disabled="currentPage === 1"
            @click="currentPage--; fetchUsers()"
          >
            {{ t('common.previous') }}
          </button>
          <button
            class="px-3 py-1 rounded border border-gray-300 dark:border-gray-600 text-sm disabled:opacity-50"
            :disabled="currentPage === totalPages"
            @click="currentPage++; fetchUsers()"
          >
            {{ t('common.next') }}
          </button>
        </div>
      </div>
    </div>

    <!-- Empty State -->
    <div v-else class="text-center py-12 bg-white dark:bg-gray-700 rounded-xl">
      <svg xmlns="http://www.w3.org/2000/svg" class="h-12 w-12 mx-auto text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
      </svg>
      <h3 class="mt-4 text-lg font-medium text-gray-900 dark:text-white">{{ t('users.noUsers') }}</h3>
      <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">{{ t('users.noUsersDescription') }}</p>
    </div>

    <!-- Modals -->
    <CreateUserModal
      v-if="showCreateModal"
      @close="showCreateModal = false"
      @created="handleUserCreated"
    />

    <EditUserModal
      v-if="showEditModal && selectedUser"
      :user="selectedUser"
      @close="showEditModal = false; selectedUser = null"
      @updated="handleUserUpdated"
    />
  </div>
</template>
