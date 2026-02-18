<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { myApi, type UserProviderConfig } from '@/api/my'

const { t } = useI18n()
const providers = ref<UserProviderConfig[]>([])
const loading = ref(true)
const editingProvider = ref<string | null>(null)
const editForm = ref({ api_key: '', base_url: '' })
const testResult = ref<{ name: string; success: boolean; message: string } | null>(null)
const saving = ref(false)

onMounted(async () => {
  await loadProviders()
})

async function loadProviders() {
  loading.value = true
  try {
    const { data } = await myApi.listProviders()
    providers.value = data
  } finally {
    loading.value = false
  }
}

function startEdit(p: UserProviderConfig) {
  editingProvider.value = p.provider_name
  editForm.value = { api_key: '', base_url: p.base_url || '' }
  testResult.value = null
}

function cancelEdit() {
  editingProvider.value = null
  testResult.value = null
}

async function saveProvider(name: string) {
  saving.value = true
  try {
    const data: Record<string, string | boolean> = {}
    if (editForm.value.api_key) data.api_key = editForm.value.api_key
    if (editForm.value.base_url !== undefined) data.base_url = editForm.value.base_url
    await myApi.updateProvider(name, data)
    await loadProviders()
    editingProvider.value = null
  } finally {
    saving.value = false
  }
}

async function removeProvider(name: string) {
  if (!confirm(t('myProviders.confirmDelete'))) return
  try {
    await myApi.deleteProvider(name)
    await loadProviders()
  } catch {
    // silent — user can retry
  }
}

async function testProvider(name: string) {
  testResult.value = null
  try {
    const { data } = await myApi.testProvider(name)
    testResult.value = { name, success: data.success, message: t(`provider.${data.messageKey}`, data.messageKey) }
  } catch {
    testResult.value = { name, success: false, message: t('provider.testFailed') }
  }
}
</script>

<template>
  <div class="max-w-4xl mx-auto">
    <h1 class="text-2xl font-bold text-gray-900 dark:text-white mb-2">{{ t('myProviders.title') }}</h1>
    <p class="text-sm text-gray-500 dark:text-gray-400 mb-6">{{ t('myProviders.description') }}</p>

    <div v-if="loading" class="text-gray-500 dark:text-gray-400">{{ t('common.loading') }}...</div>

    <div v-else class="space-y-3">
      <div
        v-for="p in providers"
        :key="p.provider_name"
        class="bg-white dark:bg-gray-800 rounded-xl p-4 border border-gray-200 dark:border-gray-700"
      >
        <div class="flex items-center justify-between">
          <div>
            <span class="font-medium text-gray-900 dark:text-white capitalize">{{ p.provider_name }}</span>
            <span v-if="p.configured" class="ml-2 text-xs px-2 py-0.5 rounded-full bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400">
              {{ t('myProviders.configured') }}
            </span>
          </div>
          <div class="flex gap-2">
            <button
              v-if="editingProvider !== p.provider_name"
              class="text-sm px-3 py-1 rounded-lg bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600"
              @click="startEdit(p)"
            >
              {{ p.configured ? t('common.edit') : t('myProviders.configure') }}
            </button>
            <button
              v-if="p.configured && editingProvider !== p.provider_name"
              class="text-sm px-3 py-1 rounded-lg text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20"
              @click="removeProvider(p.provider_name)"
            >
              {{ t('common.delete') }}
            </button>
          </div>
        </div>

        <!-- Edit form -->
        <div v-if="editingProvider === p.provider_name" class="mt-4 space-y-3">
          <div>
            <label class="block text-sm text-gray-600 dark:text-gray-400 mb-1">API Key</label>
            <input
              v-model="editForm.api_key"
              type="password"
              :placeholder="p.has_api_key ? t('myProviders.keyUnchanged') : 'sk-...'"
              class="w-full px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm"
            />
          </div>
          <div>
            <label class="block text-sm text-gray-600 dark:text-gray-400 mb-1">Base URL ({{ t('common.optional') }})</label>
            <input
              v-model="editForm.base_url"
              type="text"
              placeholder="https://api.example.com"
              class="w-full px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm"
            />
          </div>
          <div class="flex gap-2">
            <button
              class="text-sm px-4 py-1.5 rounded-lg bg-blue-600 text-white hover:bg-blue-700"
              :disabled="saving"
              @click="saveProvider(p.provider_name)"
            >
              {{ t('common.save') }}
            </button>
            <button
              v-if="p.configured"
              class="text-sm px-4 py-1.5 rounded-lg bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600"
              @click="testProvider(p.provider_name)"
            >
              {{ t('myProviders.test') }}
            </button>
            <button
              class="text-sm px-4 py-1.5 rounded-lg text-gray-500 hover:text-gray-700 dark:hover:text-gray-300"
              @click="cancelEdit"
            >
              {{ t('common.cancel') }}
            </button>
          </div>
          <div v-if="testResult && testResult.name === p.provider_name" class="text-sm" :class="testResult.success ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'">
            {{ testResult.message }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
