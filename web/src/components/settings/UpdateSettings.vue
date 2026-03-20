<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { updateApi, type UpdateInfoResponse, type OTAStatus } from '@/api/update'
import { renderMarkdown } from '@/utils/markdown'

const { t } = useI18n()

const info = ref<UpdateInfoResponse | null>(null)
const otaStatus = ref<OTAStatus | null>(null)
const loading = ref(false)
const showUpdateDialog = ref(false)
const autoCheck = ref(true)
const checkError = ref<string | null>(null)
const showUpToDate = ref(false)

// Update flow state
type UpdateState = 'idle' | 'downloading' | 'downloaded' | 'applying' | 'restarting' | 'polling'
const updateState = ref<UpdateState>('idle')
const downloadProgress = ref(0)
let progressTimer: ReturnType<typeof setInterval> | null = null
let healthTimer: ReturnType<typeof setInterval> | null = null

const fetchInfo = async () => {
  try {
    const res = await updateApi.info()
    info.value = res.data
  } catch (e) {
    console.error(e)
  }
}

const checkUpdate = async () => {
  loading.value = true
  checkError.value = null
  showUpToDate.value = false
  try {
    const res = await updateApi.ota()
    applyOTAResult(res.data)
    await fetchInfo()
    if (res.data.update_available) {
      showUpdateDialog.value = true
    } else {
      showUpToDate.value = true
      setTimeout(() => {
        showUpToDate.value = false
      }, 3000)
    }
  } catch (e) {
    console.error(e)
    checkError.value = e instanceof Error ? e.message : 'Check failed'
    setTimeout(() => {
      checkError.value = null
    }, 5000)
  } finally {
    loading.value = false
  }
}

function applyOTAResult(data: OTAStatus) {
  otaStatus.value = data
  if (data.update_available) {
    fetchReleaseNotes()
  }
}

// --- Server-side download + apply + restart flow ---

async function startDownload() {
  updateState.value = 'downloading'
  downloadProgress.value = 0
  checkError.value = null
  try {
    await updateApi.downloadOTA()
    startProgressPolling()
  } catch (e) {
    updateState.value = 'idle'
    checkError.value = t('settings.update.downloadFailed')
  }
}

function startProgressPolling() {
  progressTimer = setInterval(async () => {
    try {
      const res = await updateApi.info()
      info.value = res.data
      downloadProgress.value = res.data.status.progress || 0

      if (res.data.status.state === 'idle' && res.data.status.downloaded_path) {
        clearInterval(progressTimer!)
        progressTimer = null
        updateState.value = 'downloaded'
      } else if (res.data.status.state === 'failed') {
        clearInterval(progressTimer!)
        progressTimer = null
        updateState.value = 'idle'
        checkError.value = res.data.status.error || t('settings.update.downloadFailed')
      }
    } catch {
      // Network error during polling — keep trying
    }
  }, 500)
}

async function applyAndRestart() {
  updateState.value = 'applying'
  try {
    const res = await updateApi.apply()
    if (res.data.status === 'restarting') {
      updateState.value = 'restarting'
      startHealthPolling()
    }
  } catch {
    // Connection error is expected — server is restarting
    updateState.value = 'restarting'
    startHealthPolling()
  }
}

function startHealthPolling() {
  let attempts = 0
  const maxAttempts = 60 // 30 seconds at 500ms

  healthTimer = setInterval(async () => {
    attempts++
    try {
      const data = await updateApi.health()
      if (data.status === 'ok') {
        clearInterval(healthTimer!)
        healthTimer = null
        // Check if version changed — reload page to get new frontend
        if (data.version && data.version !== info.value?.current_version) {
          window.location.reload()
        } else {
          updateState.value = 'idle'
          showUpdateDialog.value = false
          await fetchInfo()
        }
        return
      }
    } catch {
      // Server still down, keep polling
    }
    if (attempts >= maxAttempts) {
      clearInterval(healthTimer!)
      healthTimer = null
      updateState.value = 'idle'
      checkError.value = t('settings.update.serverNotResponding')
    }
  }, 500)
}

const releaseNotesMd = ref('')
const loadingNotes = ref(false)
const releaseNotesHtml = computed(() =>
  releaseNotesMd.value ? renderMarkdown(releaseNotesMd.value) : ''
)

async function fetchReleaseNotes() {
  loadingNotes.value = true
  try {
    const res = await updateApi.releaseNotes(otaStatus.value?.release_note_url)
    releaseNotesMd.value = typeof res.data === 'string' ? res.data : String(res.data)
  } catch {
    releaseNotesMd.value = ''
  } finally {
    loadingNotes.value = false
  }
}

function toggleAutoCheck() {
  autoCheck.value = !autoCheck.value
  localStorage.setItem('update_auto_check', String(autoCheck.value))
}

const isUpdating = computed(() => updateState.value !== 'idle')

onMounted(async () => {
  autoCheck.value = localStorage.getItem('update_auto_check') !== 'false'
  await fetchInfo()
  try {
    const res = await updateApi.ota()
    applyOTAResult(res.data)
    if (res.data.update_available) showUpdateDialog.value = true
  } catch {
    /* ignore */
  }
})

onUnmounted(() => {
  if (progressTimer) clearInterval(progressTimer)
  if (healthTimer) clearInterval(healthTimer)
})
</script>

<template>
  <div class="space-y-4">
    <!-- Version & Check -->
    <div class="flex items-center justify-between">
      <div>
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
          {{ t('settings.update.title') }}
        </h3>
        <p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
          {{ t('settings.update.currentVersion') }}:
          <span class="font-mono font-medium text-gray-900 dark:text-white">{{
            info?.current_version || '-'
          }}</span>
        </p>
      </div>
      <div class="flex items-center gap-3">
        <span
          v-if="showUpToDate"
          class="text-sm text-green-600 dark:text-green-400 flex items-center gap-1"
        >
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M5 13l4 4L19 7"
            />
          </svg>
          {{ t('settings.update.upToDate') }}
        </span>
        <span v-if="checkError" class="text-sm text-red-600 dark:text-red-400">{{
          checkError
        }}</span>
        <button
          @click="checkUpdate"
          :disabled="loading || isUpdating"
          class="px-3 py-1.5 text-sm rounded-lg transition-colors flex items-center gap-1.5"
          :class="
            loading || isUpdating
              ? 'bg-gray-200 dark:bg-gray-700 text-gray-400'
              : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600'
          "
        >
          <svg v-if="loading" class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
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
              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
            />
          </svg>
          {{ t('settings.update.checkNow') }}
        </button>
      </div>
    </div>

    <!-- Auto-check toggle -->
    <div class="flex items-center justify-between py-1">
      <span class="text-sm text-gray-700 dark:text-gray-300">{{
        t('settings.update.autoCheck')
      }}</span>
      <button
        type="button"
        role="switch"
        :aria-checked="autoCheck"
        @click="toggleAutoCheck"
        class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-gray-400 focus:ring-offset-2"
        :class="autoCheck ? 'bg-green-600 dark:bg-green-500' : 'bg-gray-300 dark:bg-gray-600'"
      >
        <span
          class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
          :class="autoCheck ? 'translate-x-5' : 'translate-x-0'"
        />
      </button>
    </div>

    <!-- Update Dialog -->
    <Teleport to="body">
      <div
        v-if="showUpdateDialog && (otaStatus?.update_available || isUpdating)"
        class="fixed inset-0 z-50 flex items-center justify-center p-4"
      >
        <div
          class="absolute inset-0 bg-black/50"
          @click="!isUpdating && (showUpdateDialog = false)"
        />
        <div
          class="relative bg-white dark:bg-gray-700 rounded-xl shadow-xl max-w-lg w-full max-h-[80vh] flex flex-col"
        >
          <div
            class="px-6 py-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between"
          >
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{
                updateState === 'restarting' || updateState === 'polling'
                  ? t('settings.update.restarting')
                  : t('settings.update.newVersionAvailable')
              }}
            </h3>
            <button
              v-if="!isUpdating"
              class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
              @click="showUpdateDialog = false"
            >
              <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>
            </button>
          </div>

          <!-- Restarting / Polling state -->
          <div
            v-if="
              updateState === 'restarting' ||
              updateState === 'polling' ||
              updateState === 'applying'
            "
            class="p-8 flex flex-col items-center gap-4"
          >
            <svg class="w-10 h-10 animate-spin text-gray-400" fill="none" viewBox="0 0 24 24">
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
                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
              />
            </svg>
            <p class="text-sm text-gray-600 dark:text-gray-300">
              {{
                updateState === 'applying'
                  ? t('settings.update.applying')
                  : t('settings.update.waitingForServer')
              }}
            </p>
          </div>

          <!-- Normal update content -->
          <template v-else>
            <div class="p-6 overflow-y-auto flex-1">
              <div class="flex items-center justify-center gap-4 mb-4">
                <span class="text-lg font-mono text-gray-500 dark:text-gray-400">{{
                  otaStatus?.current_version
                }}</span>
                <svg
                  class="w-5 h-5 text-gray-400"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M13 7l5 5m0 0l-5 5m5-5H6"
                  />
                </svg>
                <span class="text-lg font-mono font-semibold text-green-600 dark:text-green-400">{{
                  otaStatus?.latest_version
                }}</span>
              </div>

              <!-- Download progress bar -->
              <div v-if="updateState === 'downloading'" class="mb-4">
                <div
                  class="flex items-center justify-between text-xs text-gray-500 dark:text-gray-400 mb-1"
                >
                  <span>{{ t('settings.update.downloading') }}</span>
                  <span>{{ Math.round(downloadProgress) }}%</span>
                </div>
                <div class="w-full bg-gray-200 dark:bg-gray-600 rounded-full h-2">
                  <div
                    class="bg-green-500 h-2 rounded-full transition-all duration-300"
                    :style="{ width: downloadProgress + '%' }"
                  />
                </div>
              </div>

              <!-- Downloaded badge -->
              <div
                v-if="updateState === 'downloaded'"
                class="mb-4 flex items-center gap-2 text-sm text-green-600 dark:text-green-400"
              >
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M5 13l4 4L19 7"
                  />
                </svg>
                {{ t('settings.update.downloadComplete') }}
              </div>

              <!-- Release notes -->
              <div
                class="rounded-lg bg-gray-50 dark:bg-gray-800 p-4 text-sm text-gray-700 dark:text-gray-300 max-h-[50vh] overflow-y-auto"
              >
                <div
                  v-if="loadingNotes"
                  class="flex items-center gap-2 text-gray-400 py-4 justify-center"
                >
                  <svg class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
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
                      d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
                    />
                  </svg>
                  Loading...
                </div>
                <div
                  v-else
                  class="prose prose-sm dark:prose-invert max-w-none"
                  v-html="releaseNotesHtml"
                />
              </div>
            </div>
            <div
              class="px-6 py-4 border-t border-gray-200 dark:border-gray-700 flex justify-end gap-3"
            >
              <button
                @click="showUpdateDialog = false"
                :disabled="isUpdating"
                class="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
              >
                {{ t('common.cancel') }}
              </button>

              <!-- Download button (idle state) -->
              <button
                v-if="updateState === 'idle'"
                @click="startDownload"
                class="px-4 py-2 text-sm font-medium rounded-lg transition-colors"
                :class="
                  'bg-gray-800 hover:bg-gray-700 dark:bg-gray-200 dark:hover:bg-gray-300 text-white dark:text-gray-900'
                "
              >
                {{ t('common.update') }}
              </button>

              <!-- Apply & Restart button (downloaded state) -->
              <button
                v-if="updateState === 'downloaded'"
                @click="applyAndRestart"
                class="px-4 py-2 text-sm font-medium rounded-lg transition-colors bg-green-600 hover:bg-green-500 text-white"
              >
                {{ t('settings.update.confirmRestart') }}
              </button>

              <!-- Downloading indicator -->
              <button
                v-if="updateState === 'downloading'"
                disabled
                class="px-4 py-2 text-sm font-medium rounded-lg bg-gray-300 dark:bg-gray-600 text-gray-500 dark:text-gray-400 cursor-not-allowed flex items-center gap-2"
              >
                <svg class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
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
                    d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
                  />
                </svg>
                {{ t('settings.update.downloading') }}
              </button>
            </div>
          </template>
        </div>
      </div>
    </Teleport>
  </div>
</template>
