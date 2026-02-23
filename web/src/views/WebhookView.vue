<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { webhookApi } from '@/api/webhook'
import type { Webhook, WebhookEvent } from '@/api/webhook'

const { t } = useI18n()

const loading = ref(false)
const webhooks = ref<Webhook[]>([])
const selectedWebhook = ref<Webhook | null>(null)
const events = ref<WebhookEvent[]>([])
const showCreateModal = ref(false)
const showEventsModal = ref(false)
const showSecretModal = ref(false)
const newSecret = ref('')
const copiedUrl = ref(false)
const copiedSecret = ref(false)

// Create form
const newWebhook = ref({
  name: '',
  type: 'incoming' as 'incoming' | 'outgoing',
  description: '',
})

const baseUrl = computed(() => window.location.origin)

onMounted(async () => {
  await loadWebhooks()
})

async function loadWebhooks() {
  try {
    loading.value = true
    const response = await webhookApi.list()
    webhooks.value = response.data
  } catch {
    webhooks.value = []
  } finally {
    loading.value = false
  }
}

async function createWebhook() {
  if (!newWebhook.value.name) return

  try {
    loading.value = true
    const response = await webhookApi.create({
      name: newWebhook.value.name,
      type: newWebhook.value.type,
      description: newWebhook.value.description,
    })
    // Show the secret for the new webhook
    if (response.data.secret) {
      newSecret.value = response.data.secret
      showSecretModal.value = true
    }
    showCreateModal.value = false
    newWebhook.value = { name: '', type: 'incoming', description: '' }
    await loadWebhooks()
  } catch {
    // Handle error
  } finally {
    loading.value = false
  }
}

async function toggleWebhook(webhook: Webhook) {
  try {
    await webhookApi.update(webhook.id, {
      name: webhook.name,
      description: webhook.description || '',
      enabled: !webhook.enabled,
    })
    await loadWebhooks()
  } catch {
    // Handle error
  }
}

async function deleteWebhook(webhook: Webhook) {
  if (!confirm(t('webhook.confirmDelete', { name: webhook.name }))) return

  try {
    await webhookApi.delete(webhook.id)
    await loadWebhooks()
  } catch {
    // Handle error
  }
}

async function regenerateSecret(webhook: Webhook) {
  if (!confirm(t('webhook.confirmRegenerate'))) return

  try {
    const response = await webhookApi.regenerateSecret(webhook.id)
    newSecret.value = response.data.secret
    showSecretModal.value = true
  } catch {
    // Handle error
  }
}

async function loadEvents(webhook: Webhook) {
  selectedWebhook.value = webhook
  try {
    const response = await webhookApi.getEvents(webhook.id, 20)
    events.value = response.data
    showEventsModal.value = true
  } catch {
    events.value = []
  }
}

async function copyWebhookUrl(webhook: Webhook) {
  const url = `${baseUrl.value}/hooks/${webhook.id}`
  await navigator.clipboard.writeText(url)
  copiedUrl.value = true
  setTimeout(() => {
    copiedUrl.value = false
  }, 2000)
}

async function copySecret() {
  await navigator.clipboard.writeText(newSecret.value)
  copiedSecret.value = true
  setTimeout(() => {
    copiedSecret.value = false
  }, 2000)
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleString()
}

function getStatusCodeColor(code: number): string {
  if (code >= 200 && code < 300) return 'text-green-600 dark:text-green-400'
  if (code >= 400 && code < 500) return 'text-yellow-600 dark:text-yellow-400'
  if (code >= 500) return 'text-red-600 dark:text-red-400'
  return 'text-gray-600 dark:text-gray-400'
}
</script>

<template>
  <div class="webhook-view p-4 sm:p-6 max-w-6xl mx-auto">
    <div class="flex items-center justify-between mb-6">
      <h1 class="text-xl sm:text-2xl font-bold text-gray-900 dark:text-white">{{ t('webhook.title') }}</h1>
      <button
        class="px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg text-sm transition-colors flex items-center gap-2"
        @click="showCreateModal = true"
      >
        <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
        </svg>
        {{ t('webhook.create') }}
      </button>
    </div>

    <!-- Webhooks List -->
    <div v-if="loading" class="text-center py-8 text-gray-500 dark:text-slate-400">
      {{ t('common.loading') }}
    </div>

    <div v-else-if="webhooks.length === 0" class="text-center py-8">
      <svg xmlns="http://www.w3.org/2000/svg" class="h-12 w-12 mx-auto text-gray-400 dark:text-slate-500 mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" />
      </svg>
      <p class="text-gray-500 dark:text-slate-400">{{ t('webhook.noWebhooks') }}</p>
      <button
        class="mt-4 px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg text-sm transition-colors"
        @click="showCreateModal = true"
      >
        {{ t('webhook.createFirst') }}
      </button>
    </div>

    <div v-else class="space-y-4">
      <div
        v-for="webhook in webhooks"
        :key="webhook.id"
        class="glass-card p-4"
      >
        <div class="flex items-start justify-between">
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-3 mb-2">
              <h3 class="text-gray-900 dark:text-white font-medium truncate">{{ webhook.name }}</h3>
              <span
                :class="[
                  'px-2 py-0.5 rounded-full text-xs font-medium',
                  webhook.enabled
                    ? 'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300'
                    : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
                ]"
              >
                {{ webhook.enabled ? t('webhook.enabled') : t('webhook.disabled') }}
              </span>
              <span class="px-2 py-0.5 bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 rounded-full text-xs font-medium">
                {{ webhook.type }}
              </span>
            </div>
            <p v-if="webhook.description" class="text-sm text-gray-500 dark:text-slate-400 mb-2">
              {{ webhook.description }}
            </p>
            <div class="flex items-center gap-2 text-xs">
              <code class="bg-gray-100 dark:bg-slate-700 px-2 py-1 rounded text-gray-600 dark:text-gray-300 truncate max-w-md">
                {{ baseUrl }}/hooks/{{ webhook.id }}
              </code>
              <button
                :title="t('common.copy')"
                class="p-1 text-gray-500 dark:text-slate-400 hover:text-gray-900 dark:hover:text-white"
                @click="copyWebhookUrl(webhook)"
              >
                <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                </svg>
              </button>
            </div>
          </div>
          <div class="flex items-center gap-2 ml-4">
            <button
              :title="t('webhook.viewEvents')"
              class="p-2 text-gray-500 dark:text-slate-400 hover:text-gray-900 dark:hover:text-white hover:bg-gray-100 dark:hover:bg-slate-700 rounded-lg transition-colors"
              @click="loadEvents(webhook)"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
              </svg>
            </button>
            <button
              :title="t('webhook.regenerateSecret')"
              class="p-2 text-yellow-600 dark:text-yellow-400 hover:bg-yellow-50 dark:hover:bg-yellow-900/20 rounded-lg transition-colors"
              @click="regenerateSecret(webhook)"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
              </svg>
            </button>
            <button
              :title="webhook.enabled ? t('webhook.disable') : t('webhook.enable')"
              :class="[
                'p-2 rounded-lg transition-colors',
                webhook.enabled
                  ? 'text-yellow-600 dark:text-yellow-400 hover:bg-yellow-50 dark:hover:bg-yellow-900/20'
                  : 'text-green-600 dark:text-green-400 hover:bg-green-50 dark:hover:bg-green-900/20'
              ]"
              @click="toggleWebhook(webhook)"
            >
              <svg v-if="webhook.enabled" xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 9v6m4-6v6m7-3a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              <svg v-else xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </button>
            <button
              :title="t('common.delete')"
              class="p-2 text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg transition-colors"
              @click="deleteWebhook(webhook)"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
              </svg>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Create Modal -->
    <div
      v-if="showCreateModal"
      class="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4"
      @click.self="showCreateModal = false"
    >
      <div class="bg-white dark:bg-slate-800 rounded-lg max-w-md w-full shadow-xl">
        <div class="p-4 sm:p-6">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">
            {{ t('webhook.createNew') }}
          </h3>

          <form class="space-y-4" @submit.prevent="createWebhook">
            <div>
              <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">{{ t('webhook.name') }}</label>
              <input
                v-model="newWebhook.name"
                type="text"
                required
                :placeholder="t('webhook.namePlaceholder')"
                class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-400 border border-gray-200 dark:border-slate-600"
              />
            </div>

            <div>
              <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">{{ t('webhook.type') }}</label>
              <select
                v-model="newWebhook.type"
                class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-400 border border-gray-200 dark:border-slate-600"
              >
                <option value="incoming">{{ t('webhook.typeIncoming') }}</option>
                <option value="outgoing">{{ t('webhook.typeOutgoing') }}</option>
              </select>
            </div>

            <div>
              <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">{{ t('webhook.description') }}</label>
              <textarea
                v-model="newWebhook.description"
                rows="3"
                :placeholder="t('webhook.descriptionPlaceholder')"
                class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-400 border border-gray-200 dark:border-slate-600 resize-none"
              />
            </div>

            <div class="flex gap-3 pt-4">
              <button
                type="submit"
                :disabled="loading || !newWebhook.name"
                class="flex-1 px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg transition-colors disabled:opacity-50"
              >
                {{ loading ? t('common.creating') : t('webhook.create') }}
              </button>
              <button
                type="button"
                class="px-4 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-900 dark:text-white rounded-lg transition-colors"
                @click="showCreateModal = false"
              >
                {{ t('common.cancel') }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>

    <!-- Secret Modal -->
    <div
      v-if="showSecretModal"
      class="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4"
      @click.self="showSecretModal = false"
    >
      <div class="bg-white dark:bg-slate-800 rounded-lg max-w-md w-full shadow-xl">
        <div class="p-4 sm:p-6">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">
            {{ t('webhook.secretGenerated') }}
          </h3>

          <div class="bg-yellow-50 dark:bg-yellow-900/30 border border-yellow-300 dark:border-yellow-600 rounded-lg p-4 mb-4">
            <p class="text-yellow-800 dark:text-yellow-200 text-sm">
              {{ t('webhook.secretWarning') }}
            </p>
          </div>

          <div class="flex items-center gap-2 mb-4">
            <code class="flex-1 bg-gray-100 dark:bg-slate-700 px-3 py-2 rounded text-sm text-gray-900 dark:text-white break-all">
              {{ newSecret }}
            </code>
            <button
              class="p-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 rounded-lg transition-colors"
              @click="copySecret"
            >
              <svg v-if="!copiedSecret" xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-gray-700 dark:text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
              </svg>
              <svg v-else xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-green-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
              </svg>
            </button>
          </div>

          <button
            class="w-full px-4 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-900 dark:text-white rounded-lg transition-colors"
            @click="showSecretModal = false; newSecret = ''"
          >
            {{ t('common.done') }}
          </button>
        </div>
      </div>
    </div>

    <!-- Events Modal -->
    <div
      v-if="showEventsModal && selectedWebhook"
      class="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4"
      @click.self="showEventsModal = false"
    >
      <div class="bg-white dark:bg-slate-800 rounded-lg max-w-2xl w-full max-h-[80vh] overflow-hidden shadow-xl">
        <div class="p-4 sm:p-6 border-b border-gray-200 dark:border-slate-700">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('webhook.eventsFor', { name: selectedWebhook.name }) }}
          </h3>
        </div>

        <div class="p-4 sm:p-6 overflow-y-auto max-h-[60vh]">
          <div v-if="events.length === 0" class="text-center py-8 text-gray-500 dark:text-slate-400">
            {{ t('webhook.noEvents') }}
          </div>

          <div v-else class="space-y-3">
            <div
              v-for="event in events"
              :key="event.id"
              class="glass-card p-3"
            >
              <div class="flex items-center justify-between mb-2">
                <div class="flex items-center gap-2">
                  <span class="px-2 py-0.5 bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 rounded text-xs font-medium">
                    {{ event.method }}
                  </span>
                  <span :class="['text-sm font-medium', getStatusCodeColor(event.status_code)]">
                    {{ event.status_code }}
                  </span>
                </div>
                <span class="text-xs text-gray-400 dark:text-slate-500">
                  {{ formatDate(event.processed_at) }}
                </span>
              </div>
              <div class="text-xs text-gray-500 dark:text-slate-400">
                {{ t('webhook.duration') }}: {{ event.duration_ms }}ms
              </div>
            </div>
          </div>
        </div>

        <div class="p-4 sm:p-6 border-t border-gray-200 dark:border-slate-700">
          <button
            class="w-full px-4 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-900 dark:text-white rounded-lg transition-colors"
            @click="showEventsModal = false"
          >
            {{ t('common.close') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
