<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { heartbeatApi } from '@/api/heartbeat'
import type { HeartbeatStatus } from '@/api/heartbeat'

const { t } = useI18n()
const emit = defineEmits<{ 'status-change': [msg: string] }>()

const status = ref<HeartbeatStatus | null>(null)
const loading = ref(false)
const fetchError = ref(false)
const triggering = ref(false)
const toggling = ref(false)

// HEARTBEAT.md editor state
const contentText = ref('')
const contentLoading = ref(false)
const contentSaving = ref(false)
const contentEditing = ref(false)
const contentDraft = ref('')

const indicatorClass = computed(() => {
  const type = status.value?.last_event?.indicator_type
  switch (type) {
    case 'ok': return 'bg-green-500'
    case 'alert': return 'bg-yellow-500'
    case 'error': return 'bg-red-500'
    default: return 'bg-gray-400'
  }
})

const statusLabel = computed(() => {
  const evt = status.value?.last_event
  if (!evt) return t('heartbeat.neverRun')
  const key = `heartbeat.status.${evt.status}`
  const translated = t(key)
  // Fallback to raw status if no translation exists
  return translated === key ? evt.status : translated
})

async function fetchStatus() {
  loading.value = true
  fetchError.value = false
  try {
    const res = await heartbeatApi.getStatus()
    status.value = res.data
  } catch (e) {
    console.error('Failed to fetch heartbeat status:', e)
    fetchError.value = true
  } finally {
    loading.value = false
  }
}

async function fetchContent() {
  contentLoading.value = true
  try {
    const res = await heartbeatApi.getContent()
    contentText.value = res.data.content
  } catch (e) {
    console.error('Failed to fetch heartbeat content:', e)
  } finally {
    contentLoading.value = false
  }
}

function startEditContent() {
  contentDraft.value = contentText.value
  contentEditing.value = true
}

function cancelEditContent() {
  contentEditing.value = false
}

async function saveContent() {
  contentSaving.value = true
  try {
    await heartbeatApi.putContent(contentDraft.value)
    contentText.value = contentDraft.value
    contentEditing.value = false
    emit('status-change', t('heartbeat.contentSaved'))
  } catch (e) {
    console.error('Failed to save heartbeat content:', e)
  } finally {
    contentSaving.value = false
  }
}

async function triggerNow() {
  triggering.value = true
  try {
    await heartbeatApi.trigger()
    emit('status-change', t('heartbeat.triggered'))
    // Refresh after a short delay to get the result
    setTimeout(fetchStatus, 3000)
  } catch (e) {
    console.error('Failed to trigger heartbeat:', e)
  } finally {
    triggering.value = false
  }
}

async function toggleEnabled() {
  if (!status.value || toggling.value) return
  toggling.value = true
  try {
    const newEnabled = !status.value.enabled
    await heartbeatApi.updateConfig({ enabled: newEnabled })
    status.value.enabled = newEnabled
    emit('status-change', newEnabled ? t('heartbeat.enabled') : t('heartbeat.disabled'))
    if (newEnabled) setTimeout(fetchStatus, 1000)
  } catch (e) {
    console.error('Failed to toggle heartbeat:', e)
  } finally {
    toggling.value = false
  }
}

function formatTime(ts?: string) {
  if (!ts) return '-'
  return new Date(ts).toLocaleString()
}

onMounted(() => {
  fetchStatus()
  fetchContent()
})
</script>

<template>
  <div class="space-y-4">
    <div class="flex items-center justify-between">
      <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('heartbeat.title') }}</h3>
      <button
        class="px-3 py-1.5 text-sm rounded-lg transition-colors"
        :class="loading ? 'bg-gray-200 dark:bg-gray-700 text-gray-400' : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600'"
        :disabled="loading"
        @click="fetchStatus"
      >
        {{ loading ? t('common.refreshing') : t('common.refresh') }}
      </button>
    </div>

    <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('heartbeat.description') }}</p>

    <!-- Enable/Disable Toggle -->
    <div v-if="status" class="flex items-center justify-between py-2">
      <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('heartbeat.enableToggle') }}</span>
      <button
        role="switch"
        :aria-checked="status.enabled"
        :disabled="toggling"
        class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500"
        :class="status.enabled ? 'bg-green-500' : 'bg-gray-300 dark:bg-gray-600'"
        @click="toggleEnabled"
      >
        <span
          class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform"
          :class="status.enabled ? 'translate-x-6' : 'translate-x-1'"
        />
      </button>
    </div>

    <!-- Status Card (only when enabled) -->
    <div v-if="status && status.enabled" class="rounded-lg border border-gray-200 dark:border-gray-700 p-4 space-y-3">
      <!-- Indicator -->
      <div class="flex items-center gap-3">
        <span class="w-2.5 h-2.5 rounded-full" :class="indicatorClass" />
        <span class="text-sm font-medium text-gray-900 dark:text-white">{{ statusLabel }}</span>
        <span v-if="status.interval" class="ml-auto text-xs text-gray-500 dark:text-gray-400">
          {{ t('heartbeat.interval') }}: {{ status.interval }}
        </span>
      </div>

      <!-- Last Event -->
      <div v-if="status.last_event" class="text-sm text-gray-600 dark:text-gray-400 space-y-1">
        <div class="flex justify-between">
          <span>{{ t('heartbeat.lastRun') }}</span>
          <span>{{ formatTime(status.last_event.timestamp) }}</span>
        </div>
        <div v-if="status.last_event.reason" class="flex justify-between">
          <span>{{ t('heartbeat.reasonLabel') }}</span>
          <span class="text-gray-500">{{ t(`heartbeat.reason.${status.last_event.reason}`, status.last_event.reason) }}</span>
        </div>
        <div v-if="status.last_event.duration_ms" class="flex justify-between">
          <span>{{ t('heartbeat.duration') }}</span>
          <span>{{ status.last_event.duration_ms }}ms</span>
        </div>
      </div>

      <!-- Next Due -->
      <div v-if="status.next_due" class="text-sm text-gray-600 dark:text-gray-400 flex justify-between">
        <span>{{ t('heartbeat.nextDue') }}</span>
        <span>{{ formatTime(status.next_due) }}</span>
      </div>

      <!-- Trigger Button -->
      <button
        class="w-full mt-2 px-4 py-2 text-sm font-medium rounded-lg transition-colors"
        :class="triggering ? 'bg-gray-300 dark:bg-gray-600 text-gray-500' : 'bg-gray-800 dark:bg-gray-200 text-white dark:text-gray-900 hover:bg-gray-700 dark:hover:bg-gray-300'"
        :disabled="triggering"
        @click="triggerNow"
      >
        {{ triggering ? t('heartbeat.triggering') : t('heartbeat.triggerNow') }}
      </button>
    </div>

    <!-- HEARTBEAT.md Editor -->
    <div v-if="status" class="rounded-lg border border-gray-200 dark:border-gray-700 p-4 space-y-3">
      <div class="flex items-center justify-between">
        <div>
          <span class="text-sm font-medium text-gray-900 dark:text-white">HEARTBEAT.md</span>
          <p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">{{ t('heartbeat.contentDescription') }}</p>
        </div>
        <button
          v-if="!contentEditing"
          class="px-3 py-1.5 text-sm rounded-lg transition-colors bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600"
          @click="startEditContent"
        >
          {{ t('common.edit') }}
        </button>
        <div v-else class="flex items-center gap-2">
          <button
            class="px-3 py-1.5 text-sm rounded-lg transition-colors bg-gray-800 dark:bg-gray-200 text-white dark:text-gray-900 hover:bg-gray-700 dark:hover:bg-gray-300"
            :disabled="contentSaving"
            @click="saveContent"
          >
            {{ contentSaving ? t('common.saving') : t('common.save') }}
          </button>
          <button
            class="px-3 py-1.5 text-sm rounded-lg transition-colors bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600"
            :disabled="contentSaving"
            @click="cancelEditContent"
          >
            {{ t('common.cancel') }}
          </button>
        </div>
      </div>

      <div v-if="contentLoading" class="text-sm text-gray-500 dark:text-gray-400 py-2 text-center">
        {{ t('common.loading') }}
      </div>
      <template v-else>
        <textarea
          v-if="contentEditing"
          v-model="contentDraft"
          rows="8"
          class="w-full px-3 py-2 text-sm font-mono bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-600 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 resize-y"
          :placeholder="t('heartbeat.contentPlaceholder')"
        />
        <pre
          v-else
          class="text-sm font-mono text-gray-700 dark:text-gray-300 bg-gray-50 dark:bg-gray-800 rounded-lg p-3 whitespace-pre-wrap min-h-[4rem] max-h-48 overflow-y-auto"
        >{{ contentText || t('heartbeat.contentEmpty') }}</pre>
      </template>
    </div>

    <!-- Loading -->
    <div v-else-if="loading" class="text-sm text-gray-500 dark:text-gray-400 py-4 text-center">
      {{ t('common.loading') }}
    </div>

    <!-- Error fallback -->
    <div v-else-if="fetchError" class="text-sm text-gray-500 dark:text-gray-400 py-4 text-center space-y-2">
      <p>{{ t('heartbeat.fetchError') }}</p>
      <button
        class="px-3 py-1.5 text-sm rounded-lg bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors"
        @click="fetchStatus"
      >
        {{ t('common.retry') }}
      </button>
    </div>
  </div>
</template>
