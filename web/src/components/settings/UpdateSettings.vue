<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
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
const delayCooldown = ref(0)
let cooldownTimer: ReturnType<typeof setInterval> | null = null

const fetchInfo = async () => {
  try {
    const res = await updateApi.info()
    info.value = res.data
  } catch (e) { console.error(e) }
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
      setTimeout(() => { showUpToDate.value = false }, 3000)
    }
  } catch (e) {
    console.error(e)
    checkError.value = e instanceof Error ? e.message : 'Check failed'
    setTimeout(() => { checkError.value = null }, 5000)
  } finally { loading.value = false }
}

function startCooldown(seconds: number) {
  if (cooldownTimer) clearInterval(cooldownTimer)
  delayCooldown.value = seconds
  cooldownTimer = setInterval(() => {
    delayCooldown.value--
    if (delayCooldown.value <= 0) {
      clearInterval(cooldownTimer!)
      cooldownTimer = null
    }
  }, 1000)
}

function applyOTAResult(data: OTAStatus) {
  otaStatus.value = data
  if (data.delay && data.delay > 0) {
    startCooldown(data.delay)
  }
  if (data.update_available) {
    fetchReleaseNotes()
  }
}

function openDownload() {
  const urls = otaStatus.value?.download_urls
  if (urls && urls.length > 0) window.open(urls[0], '_blank')
}

const releaseNotesMd = ref('')
const loadingNotes = ref(false)
const releaseNotesHtml = computed(() => releaseNotesMd.value ? renderMarkdown(releaseNotesMd.value) : '')

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

onMounted(async () => {
  autoCheck.value = localStorage.getItem('update_auto_check') !== 'false'
  await fetchInfo()
  // Fetch cached OTA result
  try {
    const res = await updateApi.ota()
    applyOTAResult(res.data)
    if (res.data.update_available) showUpdateDialog.value = true
  } catch { /* ignore */ }
})
</script>

<template>
  <div class="space-y-4">
    <!-- Version & Check -->
    <div class="flex items-center justify-between">
      <div>
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('settings.update.title') }}</h3>
        <p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
          {{ t('settings.update.currentVersion') }}:
          <span class="font-mono font-medium text-gray-900 dark:text-white">{{ info?.current_version || '-' }}</span>
        </p>
      </div>
      <div class="flex items-center gap-3">
        <span v-if="showUpToDate" class="text-sm text-green-600 dark:text-green-400 flex items-center gap-1">
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/>
          </svg>
          {{ t('settings.update.upToDate') }}
        </span>
        <span v-if="checkError" class="text-sm text-red-600 dark:text-red-400">{{ checkError }}</span>
        <button
          @click="checkUpdate"
          :disabled="loading"
          class="px-3 py-1.5 text-sm rounded-lg transition-colors flex items-center gap-1.5"
          :class="loading ? 'bg-gray-200 dark:bg-gray-700 text-gray-400' : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600'"
        >
          <svg v-if="loading" class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
          </svg>
          {{ t('settings.update.checkNow') }}
        </button>
      </div>
    </div>

    <!-- Auto-check toggle -->
    <div class="flex items-center justify-between py-1">
      <span class="text-sm text-gray-700 dark:text-gray-300">{{ t('settings.update.autoCheck') }}</span>
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
      <div v-if="showUpdateDialog && otaStatus?.update_available" class="fixed inset-0 z-50 flex items-center justify-center p-4">
        <div class="absolute inset-0 bg-black/50" @click="showUpdateDialog = false"/>
        <div class="relative bg-white dark:bg-gray-700 rounded-xl shadow-xl max-w-lg w-full max-h-[80vh] flex flex-col">
          <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('settings.update.newVersionAvailable') }}</h3>
            <button class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300" @click="showUpdateDialog = false">
              <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/>
              </svg>
            </button>
          </div>
          <div class="p-6 overflow-y-auto flex-1">
            <div class="flex items-center justify-center gap-4 mb-4">
              <span class="text-lg font-mono text-gray-500 dark:text-gray-400">{{ otaStatus.current_version }}</span>
              <svg class="w-5 h-5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 7l5 5m0 0l-5 5m5-5H6"/>
              </svg>
              <span class="text-lg font-mono font-semibold text-green-600 dark:text-green-400">{{ otaStatus.latest_version }}</span>
            </div>
            <div class="rounded-lg bg-gray-50 dark:bg-gray-800 p-4 text-sm text-gray-700 dark:text-gray-300 max-h-[50vh] overflow-y-auto">
              <div v-if="loadingNotes" class="flex items-center gap-2 text-gray-400 py-4 justify-center">
                <svg class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
                  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
                  <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
                </svg>
                Loading...
              </div>
              <div v-else class="prose prose-sm dark:prose-invert max-w-none" v-html="releaseNotesHtml"/>
            </div>
          </div>
          <div class="px-6 py-4 border-t border-gray-200 dark:border-gray-700 flex justify-end gap-3">
            <button
              @click="showUpdateDialog = false"
              class="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
            >
              {{ t('common.cancel') }}
            </button>
            <button
              @click="openDownload"
              :disabled="delayCooldown > 0"
              class="px-4 py-2 text-sm font-medium rounded-lg transition-colors"
              :class="delayCooldown > 0 ? 'bg-gray-300 dark:bg-gray-600 text-gray-500 dark:text-gray-400 cursor-not-allowed' : 'bg-gray-800 hover:bg-gray-700 dark:bg-gray-200 dark:hover:bg-gray-300 text-white dark:text-gray-900'"
            >
              {{ delayCooldown > 0 ? `${t('settings.update.download')} (${delayCooldown}s)` : t('settings.update.download') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
