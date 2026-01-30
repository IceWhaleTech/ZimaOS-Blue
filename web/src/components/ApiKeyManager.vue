<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

export interface ApiKey {
  id: string
  name: string
  prefix: string
  createdAt: Date
  lastUsedAt?: Date
  expiresAt?: Date
  scopes: string[]
}

const props = withDefaults(
  defineProps<{
    apiKeys?: ApiKey[]
    loading?: boolean
  }>(),
  {
    apiKeys: () => [],
    loading: false,
  }
)

const emit = defineEmits<{
  create: [name: string, scopes: string[]]
  revoke: [id: string]
  copy: [key: string]
}>()

const showCreateModal = ref(false)
const newKeyName = ref('')
const newKeyScopes = ref<string[]>(['read'])
const createdKey = ref<string | null>(null)

const availableScopes = [
  { id: 'read', label: 'Read', description: 'Read access to resources' },
  { id: 'write', label: 'Write', description: 'Write access to resources' },
  { id: 'admin', label: 'Admin', description: 'Administrative access' },
]

const sortedKeys = computed(() => {
  return [...props.apiKeys].sort((a, b) =>
    new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime()
  )
})

function formatDate(date: Date | undefined): string {
  if (!date) return 'Never'
  return new Date(date).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  })
}

function isExpired(key: ApiKey): boolean {
  if (!key.expiresAt) return false
  return new Date(key.expiresAt) < new Date()
}

function handleCreate() {
  if (newKeyName.value.trim()) {
    emit('create', newKeyName.value.trim(), newKeyScopes.value)
    // In real implementation, the created key would be returned from the API
    createdKey.value = 'sk_live_' + Math.random().toString(36).substring(2, 15)
  }
}

function handleCopyKey() {
  if (createdKey.value) {
    emit('copy', createdKey.value)
    navigator.clipboard.writeText(createdKey.value)
  }
}

function closeModal() {
  showCreateModal.value = false
  newKeyName.value = ''
  newKeyScopes.value = ['read']
  createdKey.value = null
}

function confirmRevoke(key: ApiKey) {
  if (confirm(`Are you sure you want to revoke the API key "${key.name}"? This action cannot be undone.`)) {
    emit('revoke', key.id)
  }
}
</script>

<template>
  <div class="bg-white dark:bg-gray-800 rounded-lg shadow">
    <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white">API Keys</h2>
      <button
        class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-lg transition-colors"
        @click="showCreateModal = true"
      >
        Create API Key
      </button>
    </div>

    <div v-if="loading" class="p-6 text-center">
      <div class="animate-spin h-8 w-8 border-4 border-blue-500 border-t-transparent rounded-full mx-auto"></div>
      <p class="mt-2 text-gray-500 dark:text-gray-400">Loading API keys...</p>
    </div>

    <div v-else-if="apiKeys.length === 0" class="p-6 text-center">
      <svg xmlns="http://www.w3.org/2000/svg" class="h-12 w-12 mx-auto text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z" />
      </svg>
      <p class="mt-2 text-gray-500 dark:text-gray-400">No API keys yet</p>
      <p class="text-sm text-gray-400 dark:text-gray-500">Create an API key to get started</p>
    </div>

    <div v-else class="divide-y divide-gray-200 dark:divide-gray-700">
      <div
        v-for="key in sortedKeys"
        :key="key.id"
        class="px-6 py-4 flex items-center justify-between"
        :class="{ 'opacity-50': isExpired(key) }"
      >
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2">
            <span class="font-medium text-gray-900 dark:text-white">{{ key.name }}</span>
            <span v-if="isExpired(key)" class="px-2 py-0.5 text-xs bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400 rounded-full">
              Expired
            </span>
          </div>
          <div class="mt-1 flex items-center gap-4 text-sm text-gray-500 dark:text-gray-400">
            <span class="font-mono">{{ key.prefix }}...</span>
            <span>Created {{ formatDate(key.createdAt) }}</span>
            <span>Last used {{ formatDate(key.lastUsedAt) }}</span>
          </div>
          <div class="mt-1 flex gap-1">
            <span
              v-for="scope in key.scopes"
              :key="scope"
              class="px-2 py-0.5 text-xs bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 rounded"
            >
              {{ scope }}
            </span>
          </div>
        </div>
        <button
          class="ml-4 px-3 py-1.5 text-sm text-red-600 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg transition-colors"
          @click="confirmRevoke(key)"
        >
          Revoke
        </button>
      </div>
    </div>

    <!-- Create Modal -->
    <Teleport to="body">
      <div
        v-if="showCreateModal"
        class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
        @click.self="closeModal"
      >
        <div class="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-md w-full mx-4">
          <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ createdKey ? 'API Key Created' : 'Create API Key' }}
            </h3>
          </div>

          <div class="p-6">
            <template v-if="createdKey">
              <div class="bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg p-4 mb-4">
                <p class="text-sm text-yellow-800 dark:text-yellow-200">
                  Make sure to copy your API key now. You won't be able to see it again!
                </p>
              </div>
              <div class="flex items-center gap-2">
                <input
                  type="text"
                  :value="createdKey"
                  readonly
                  class="flex-1 px-3 py-2 bg-gray-100 dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-lg font-mono text-sm"
                />
                <button
                  class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors"
                  @click="handleCopyKey"
                >
                  Copy
                </button>
              </div>
            </template>

            <template v-else>
              <div class="space-y-4">
                <div>
                  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                    Key Name
                  </label>
                  <input
                    v-model="newKeyName"
                    type="text"
                    placeholder="e.g., Production API Key"
                    class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                  />
                </div>

                <div>
                  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                    Scopes
                  </label>
                  <div class="space-y-2">
                    <label
                      v-for="scope in availableScopes"
                      :key="scope.id"
                      class="flex items-start gap-3 p-3 border border-gray-200 dark:border-gray-600 rounded-lg cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-700"
                    >
                      <input
                        v-model="newKeyScopes"
                        type="checkbox"
                        :value="scope.id"
                        class="mt-0.5 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                      />
                      <div>
                        <span class="font-medium text-gray-900 dark:text-white">{{ scope.label }}</span>
                        <p class="text-sm text-gray-500 dark:text-gray-400">{{ scope.description }}</p>
                      </div>
                    </label>
                  </div>
                </div>
              </div>
            </template>
          </div>

          <div class="px-6 py-4 border-t border-gray-200 dark:border-gray-700 flex justify-end gap-3">
            <button
              class="px-4 py-2 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
              @click="closeModal"
            >
              {{ createdKey ? t('common.done') : t('common.cancel') }}
            </button>
            <button
              v-if="!createdKey"
              class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              :disabled="!newKeyName.trim() || newKeyScopes.length === 0"
              @click="handleCreate"
            >
              Create Key
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
