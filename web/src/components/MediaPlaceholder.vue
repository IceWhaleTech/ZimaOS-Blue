<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import type { MediaTask } from '@/api/media'
import { getTask, retryTask } from '@/api/media'
import { useNotificationStore } from '@/stores/notification'
import { onSSEEvent, offSSEEvent } from '@/composables/useEventStream'

const props = defineProps<{
  taskId: string
}>()

const { t } = useI18n()
const router = useRouter()
const notificationStore = useNotificationStore()

// activeTaskId tracks the current task — may differ from props.taskId after a retry
const activeTaskId = ref(props.taskId)

const task = ref<MediaTask | null>(null)
const pollTimer = ref<ReturnType<typeof setTimeout> | null>(null)
const loading = ref(true)
const retrying = ref(false)
const elapsedSeconds = ref(0)
let elapsedInterval: ReturnType<typeof setInterval> | null = null
let notified = false
let sseActive = false
let initialFetch = true // true until first poll completes — suppresses toast for already-terminal tasks

// Lightbox viewer state
const viewerUrl = ref('')
const viewerOpen = ref(false)

function openViewer(url: string) {
  viewerUrl.value = url
  viewerOpen.value = true
}

function closeViewer() {
  viewerOpen.value = false
}

function downloadImage() {
  if (!viewerUrl.value) return
  const a = document.createElement('a')
  a.href = viewerUrl.value
  a.download = viewerUrl.value.split('/').pop() || 'generated-image'
  a.click()
}

function openInNewTab() {
  if (viewerUrl.value) window.open(viewerUrl.value, '_blank')
}

const isTerminal = computed(() => {
  const s = task.value?.status
  return s === 'succeeded' || s === 'failed' || s === 'cancelled'
})

const statusLabel = computed(() => {
  if (!task.value) return t('media.pending')
  return t(`media.${task.value.status}`)
})

const progressPercent = computed(() => {
  return Math.round((task.value?.progress || 0) * 100)
})

const resultUrls = computed(() => {
  return task.value?.response?.data?.map((d: any) => d.url) || []
})

const isVideo = computed(() => {
  return task.value?.type === 'video'
})

// Detect provider/config errors to show setup guidance
const isProviderError = computed(() => {
  if (!task.value?.error) return false
  const e = task.value.error.toLowerCase()
  return e.includes('404') || e.includes('no provider') || e.includes('not configured') || e.includes('api error') || e.includes('auth') || e.includes('api key')
})

const cancelMessage = computed(() => {
  const raw = task.value?.error?.trim()
  if (!raw) return t('chat.taskCancelled')
  const normalized = raw.toLowerCase()
  if (normalized === 'cancelled by user' || normalized === 'task cancelled') {
    return t('chat.taskCancelled')
  }
  return raw
})

function calcElapsed(): number {
  if (task.value?.created_at) {
    return Math.max(0, (Date.now() - new Date(task.value.created_at).getTime()) / 1000)
  }
  return 0
}

function startElapsedTimer() {
  elapsedSeconds.value = calcElapsed()
  elapsedInterval = setInterval(() => { elapsedSeconds.value = calcElapsed() }, 100)
}

function stopElapsedTimer() {
  if (elapsedInterval) {
    clearInterval(elapsedInterval)
    elapsedInterval = null
  }
}

// --- Notification on terminal state ---
function handleTerminal() {
  stopElapsedTimer()
  stopPolling()
  if (!notified) {
    notified = true
    if (task.value?.status === 'succeeded') {
      notificationStore.success(t('media.succeeded'), t('media.completedToast'), { titleKey: 'media.succeeded', messageKey: 'media.completedToast' })
    } else if (task.value?.status === 'failed') {
      notificationStore.error(t('media.failed'), task.value?.error || t('media.toast.failed'), { titleKey: 'media.failed', messageKey: task.value?.error ? undefined : 'media.toast.failed' })
    } else if (task.value?.status === 'cancelled') {
      notificationStore.info(t('media.cancelled'), cancelMessage.value, { titleKey: 'media.cancelled' })
    }
  }
}

// --- SSE listener for real-time updates ---
function onTaskUpdate(data: any) {
  if (data.id !== activeTaskId.value) return
  sseActive = true
  // Stop polling — SSE is delivering updates
  stopPolling()

  // Merge SSE data into task
  if (!task.value) {
    task.value = { id: data.id, status: data.status, progress: data.progress || 0 } as MediaTask
  } else {
    task.value = { ...task.value, status: data.status, progress: data.progress || task.value.progress }
  }
  if (data.error) task.value.error = data.error
  if (data.response) task.value.response = data.response
  loading.value = false

  if (isTerminal.value) handleTerminal()
}

// --- Polling fallback (2-4s jitter) ---
function nextPollDelay(): number {
  return 2000 + Math.random() * 2000
}

async function poll() {
  if (!activeTaskId.value || sseActive) return
  const isFirst = initialFetch
  initialFetch = false
  try {
    const result = await getTask(activeTaskId.value)
    task.value = result
    loading.value = false
  } catch {
    loading.value = false
  }

  if (isTerminal.value) {
    // If the task was already terminal on first fetch, suppress the toast —
    // the user is just re-entering the conversation, not witnessing a live completion.
    if (isFirst) notified = true
    handleTerminal()
  } else if (!sseActive) {
    pollTimer.value = setTimeout(poll, nextPollDelay())
  }
}

function stopPolling() {
  if (pollTimer.value) {
    clearTimeout(pollTimer.value)
    pollTimer.value = null
  }
}

async function handleRetry() {
  if (retrying.value) return
  retrying.value = true
  try {
    const result = await retryTask(activeTaskId.value)
    // Switch to tracking the new task
    activeTaskId.value = result.task_id
    stopTracking()
    startTracking()
  } catch {
    notificationStore.error(t('media.failed'), t('media.retryFailed'), { titleKey: 'media.failed', messageKey: 'media.retryFailed' })
  } finally {
    retrying.value = false
  }
}

function startTracking() {
  sseActive = false
  notified = false
  initialFetch = true
  loading.value = true
  task.value = null
  startElapsedTimer()
  onSSEEvent('media_task_update', onTaskUpdate)
  // Initial fetch + start polling as fallback until SSE delivers
  poll()
}

function stopTracking() {
  stopPolling()
  stopElapsedTimer()
  offSSEEvent('media_task_update', onTaskUpdate)
}

onMounted(startTracking)
onUnmounted(stopTracking)

// If taskId prop changes externally, sync activeTaskId and restart tracking
watch(() => props.taskId, (newId) => {
  if (newId && newId !== activeTaskId.value) {
    activeTaskId.value = newId
    stopTracking()
    startTracking()
  }
})
</script>

<template>
  <div class="media-placeholder" :class="task ? `status-${task.status}` : 'status-pending'" role="region" :aria-label="statusLabel">
    <!-- Loading / Pending / Processing — AI waiting card style -->
    <div v-if="!task || task.status === 'pending' || task.status === 'processing'" class="mp-card" role="status" aria-live="polite">
      <div class="mp-card-inner">
        <div class="mp-card-header">
          <svg class="mp-spinner" width="20" height="20" viewBox="0 0 24 24">
            <circle cx="12" cy="12" r="10" fill="none" stroke="currentColor" stroke-width="2" opacity="0.15" />
            <circle cx="12" cy="12" r="10" fill="none" stroke="url(#mpGrad)" stroke-width="2.5" stroke-linecap="round" stroke-dasharray="40 23" />
            <defs>
              <linearGradient id="mpGrad" x1="0" y1="0" x2="1" y2="1">
                <stop offset="0%" stop-color="#818cf8" />
                <stop offset="100%" stop-color="#c084fc" />
              </linearGradient>
            </defs>
          </svg>
          <span class="mp-card-title">{{ statusLabel }}</span>
          <span v-if="elapsedSeconds >= 0.5" class="mp-card-timer">{{ elapsedSeconds.toFixed(1) }}s</span>
        </div>
        <div v-if="task?.model" class="mp-card-model">{{ task.model }}</div>
        <div v-if="elapsedSeconds > 10" class="mp-card-hint">{{ t('media.processingHint') }}</div>
        <div class="mp-card-bar">
          <div v-if="progressPercent > 0" class="mp-card-bar-real" :style="{ width: progressPercent + '%' }" role="progressbar" :aria-valuenow="progressPercent" aria-valuemin="0" aria-valuemax="100" />
          <div v-else class="mp-card-bar-shimmer" />
        </div>
      </div>
    </div>

    <!-- Succeeded -->
    <div v-else-if="task.status === 'succeeded' && resultUrls.length > 0" class="mp-result">
      <template v-for="(url, i) in resultUrls" :key="i">
        <video v-if="isVideo" :src="url" controls class="mp-media" :aria-label="`Generated video ${i + 1}`" />
        <img v-else :src="url" class="mp-media mp-media-clickable" loading="lazy" :alt="`Generated image ${i + 1}`" @click="openViewer(url)" />
      </template>
    </div>

    <!-- Failed -->
    <div v-else-if="task.status === 'failed'" class="mp-error" role="alert">
      <div class="mp-error-body">
        <span class="mp-error-text">{{ task.error || statusLabel }}</span>
        <div v-if="isProviderError" class="mp-error-hint">
          {{ t('media.errorHint') }}
          <a class="mp-error-link" @click.prevent="router.push({ path: '/settings', query: { tab: 'llm', section: 'media' } })">{{ t('media.goSettings') }}</a>
        </div>
      </div>
      <button class="mp-retry" :disabled="retrying" @click="handleRetry" :aria-label="t('media.retry')">{{ retrying ? '...' : t('media.retry') }}</button>
    </div>

    <!-- Cancelled -->
    <div v-else class="mp-cancelled" role="status">
      <div class="mp-error-body">
        <span class="mp-cancelled-text">{{ cancelMessage }}</span>
      </div>
      <button class="mp-retry mp-retry--cancelled" :disabled="retrying" @click="handleRetry" :aria-label="t('media.retry')">{{ retrying ? '...' : t('media.retry') }}</button>
    </div>

    <!-- Image Viewer Overlay -->
    <Teleport to="body">
      <div v-if="viewerOpen" class="mp-viewer-overlay" @click.self="closeViewer" role="dialog" aria-modal="true" :aria-label="t('media.fullscreen')">
        <div class="mp-viewer-toolbar">
          <button class="mp-viewer-btn" @click="downloadImage" :title="t('media.download')">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
          </button>
          <button class="mp-viewer-btn" @click="openInNewTab" :title="t('media.openInNewTab')">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/><polyline points="15 3 21 3 21 9"/><line x1="10" y1="14" x2="21" y2="3"/></svg>
          </button>
          <button class="mp-viewer-btn" @click="closeViewer" :title="t('media.exitFullscreen')">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
          </button>
        </div>
        <img :src="viewerUrl" class="mp-viewer-img" :alt="t('media.fullscreen')" />
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
.media-placeholder {
  border-radius: 8px;
  overflow: hidden;
  margin: 4px 0;
}

/* AI waiting card — matches chat thinking card */
.mp-card {
  animation: mp-fade-in 0.3s ease;
}

.mp-card-inner {
  border-radius: 0.75rem;
  border: 1px solid transparent;
  background:
    linear-gradient(#fff, #fff) padding-box,
    linear-gradient(135deg, #818cf8, #c084fc, #f472b6) border-box;
  padding: 0.75rem 1rem;
  box-shadow: 0 1px 3px rgba(129, 140, 248, 0.12);
}

:root.dark .mp-card-inner,
[data-theme="dark"] .mp-card-inner {
  background:
    linear-gradient(#1e293b, #1e293b) padding-box,
    linear-gradient(135deg, #818cf8, #c084fc, #f472b6) border-box;
  box-shadow: 0 1px 6px rgba(129, 140, 248, 0.15);
}

.mp-card-header {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.mp-spinner {
  animation: mp-spin 1.2s linear infinite;
  color: #818cf8;
  flex-shrink: 0;
}

.mp-card-title {
  font-size: 0.8125rem;
  font-weight: 500;
  color: #6366f1;
  flex: 1;
}

:root.dark .mp-card-title,
[data-theme="dark"] .mp-card-title {
  color: #a5b4fc;
}

.mp-card-timer {
  font-size: 0.75rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  color: #a78bfa;
  min-width: 2.5rem;
  text-align: right;
}

.mp-card-model {
  font-size: 0.6875rem;
  color: #94a3b8;
  margin-top: 2px;
  padding-left: 1.75rem;
}

:root.dark .mp-card-model,
[data-theme="dark"] .mp-card-model {
  color: #64748b;
}

.mp-card-hint {
  font-size: 0.6875rem;
  color: #94a3b8;
  padding-left: 1.75rem;
  margin-top: 2px;
  animation: mp-fade-in 0.3s ease;
}

:root.dark .mp-card-hint,
[data-theme="dark"] .mp-card-hint {
  color: #64748b;
}

.mp-card-bar {
  margin-top: 0.5rem;
  height: 3px;
  border-radius: 2px;
  background: #e2e8f0;
  overflow: hidden;
}

:root.dark .mp-card-bar,
[data-theme="dark"] .mp-card-bar {
  background: #334155;
}

.mp-card-bar-shimmer {
  height: 100%;
  border-radius: 2px;
  background: linear-gradient(90deg, #818cf8, #c084fc, #f472b6);
  animation: mp-bar-slide 2s ease-in-out infinite;
  width: 40%;
}

.mp-card-bar-real {
  height: 100%;
  border-radius: 2px;
  background: linear-gradient(90deg, #818cf8, #c084fc);
  transition: width 0.5s ease;
}

@keyframes mp-spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

@keyframes mp-bar-slide {
  0% { transform: translateX(-100%); }
  50% { transform: translateX(150%); }
  100% { transform: translateX(-100%); }
}

@keyframes mp-fade-in {
  from { opacity: 0; transform: translateY(4px); }
  to { opacity: 1; transform: translateY(0); }
}

.mp-result {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.mp-media {
  max-width: 100%;
  max-height: 400px;
  border-radius: 8px;
  object-fit: contain;
}

.mp-media-clickable {
  cursor: pointer;
  transition: opacity 0.15s;
}

.mp-media-clickable:hover {
  opacity: 0.85;
}

/* Image Viewer Overlay */
.mp-viewer-overlay {
  position: fixed;
  inset: 0;
  z-index: 9999;
  background: rgba(0, 0, 0, 0.9);
  display: flex;
  align-items: center;
  justify-content: center;
  animation: mp-fade-in 0.2s ease;
}

.mp-viewer-toolbar {
  position: absolute;
  top: 16px;
  right: 16px;
  display: flex;
  gap: 8px;
  z-index: 10000;
}

.mp-viewer-btn {
  background: rgba(255, 255, 255, 0.15);
  border: none;
  border-radius: 8px;
  padding: 8px;
  color: #fff;
  cursor: pointer;
  transition: background 0.15s;
  display: flex;
  align-items: center;
  justify-content: center;
}

.mp-viewer-btn:hover {
  background: rgba(255, 255, 255, 0.3);
}

.mp-viewer-img {
  max-width: 90vw;
  max-height: 90vh;
  object-fit: contain;
  border-radius: 4px;
}

.mp-error {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 10px 12px;
  background: var(--color-error-bg, #fef2f2);
  border: 1px solid var(--color-error-border, #fecaca);
  border-radius: 8px;
}

.mp-error-body {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.mp-error-text {
  font-size: 13px;
  color: var(--color-error, #dc2626);
}

.mp-error-hint {
  font-size: 12px;
  color: var(--color-text-secondary, #666);
}

.mp-error-link {
  color: var(--color-primary, #6366f1);
  text-decoration: none;
  font-weight: 500;
  cursor: pointer;
}

.mp-error-link:hover {
  text-decoration: underline;
}

.mp-retry {
  background: none;
  border: 1px solid var(--color-error, #dc2626);
  color: var(--color-error, #dc2626);
  border-radius: 4px;
  padding: 2px 10px;
  font-size: 12px;
  cursor: pointer;
}

.mp-cancelled {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 10px 12px;
  background: var(--color-warning-bg, #fffbeb);
  border: 1px solid var(--color-warning-border, #fde68a);
  border-radius: 8px;
}

.mp-cancelled-text {
  font-size: 13px;
  color: var(--color-warning, #d97706);
}

.mp-retry--cancelled {
  border-color: var(--color-warning, #d97706);
  color: var(--color-warning, #d97706);
}
</style>
