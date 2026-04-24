<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { TunnelStatus as TunnelStatusType } from '@/api/remote-access'
import {
  getRemoteAccessDiagnostics,
  getRemoteAccessLogs,
  getRemoteAccessQRCode,
  type RemoteAccessLog,
} from '@/api/remote-access'
import { getTunnelProviderIcon } from '@/utils/channelIcons'
import { useTauri } from '@/composables/useTauri'
import QRCodeDisplay from '@/components/common/QRCodeDisplay.vue'

const { t } = useI18n()
const { openInBrowser } = useTauri()

const props = defineProps<{
  status: TunnelStatusType
}>()

const emit = defineEmits<{
  (e: 'disconnect'): void
}>()

function handleDisconnect() {
  emit('disconnect')
}

// QR Code state - auto-show when URL available
const qrCodeLoading = ref(false)
const qrCodeValue = ref<string | null>(null)

// Diagnostics state
const showDiagnostics = ref(false)
const diagnosticsLoading = ref(false)
const diagnostics = ref<{
  ngrok_installed?: boolean
  ngrok_path?: string
  ngrok_version?: string
  firewall_exception: boolean
  tunnel_running: boolean
  ssh_available?: boolean
  cloudflared_installed?: boolean
  active_provider?: string
  recent_errors?: Array<{
    id: string
    event_type: string
    message: string
    created_at: string
  }>
  active_session?: {
    id: string
    status: string
    started_at: string
    error_message?: string
  }
  os: {
    platform: string
  }
  hints: string[]
} | null>(null)

// Logs state
const showLogs = ref(false)
const logsLoading = ref(false)
const logs = ref<RemoteAccessLog[]>([])

const statusColor = computed(() => {
  if (props.status.active || props.status.url) return 'text-green-600 dark:text-green-400'
  if (props.status.connecting) return 'text-amber-600 dark:text-amber-400'
  return 'text-gray-500 dark:text-slate-300'
})

const statusIcon = computed(() => {
  if (props.status.active || props.status.url) return '🟢'
  if (props.status.connecting) return '🟡'
  return '⚪'
})

const statusText = computed(() => {
  // URL available means tunnel is connected (even if active flag lags)
  if (props.status.active || props.status.url) return t('remoteAccess.connected')
  if (props.status.connecting) return t('remoteAccess.connecting')
  return t('remoteAccess.disconnected')
})

function copyUrl() {
  if (props.status.url) {
    navigator.clipboard.writeText(props.status.url)
  }
}

function openUrl() {
  if (props.status.url) {
    openInBrowser(props.status.url)
  }
}

async function loadQRCode() {
  if (qrCodeLoading.value) return
  qrCodeLoading.value = true
  try {
    const response = await getRemoteAccessQRCode()
    if (response.data.success) {
      qrCodeValue.value = response.data.qr_url || response.data.url
    }
  } catch (e) {
    console.error('Failed to load QR code:', e)
  } finally {
    qrCodeLoading.value = false
  }
}

async function loadDiagnostics() {
  if (diagnosticsLoading.value) return
  diagnosticsLoading.value = true
  try {
    const response = await getRemoteAccessDiagnostics()
    if (response.data.success) {
      diagnostics.value = response.data.diagnostics
    }
  } catch (e) {
    console.error('Failed to load diagnostics:', e)
  } finally {
    diagnosticsLoading.value = false
  }
}

async function loadLogs() {
  if (logsLoading.value) return
  logsLoading.value = true
  try {
    const response = await getRemoteAccessLogs(50, 0)
    if (response.data.success) {
      logs.value = response.data.logs || []
    }
  } catch (e) {
    console.error('Failed to load logs:', e)
  } finally {
    logsLoading.value = false
  }
}

function toggleDiagnostics() {
  showDiagnostics.value = !showDiagnostics.value
  if (showDiagnostics.value && !diagnostics.value) {
    loadDiagnostics()
  }
}

function toggleLogs() {
  showLogs.value = !showLogs.value
  if (showLogs.value && logs.value.length === 0) {
    loadLogs()
  }
}

function formatTime(dateStr: string) {
  return new Date(dateStr).toLocaleString()
}

function getEventTypeColor(eventType: string) {
  switch (eventType) {
    case 'error':
    case 'stderr':
      return 'text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-900/20'
    case 'started':
    case 'connected':
      return 'text-green-600 dark:text-green-400 bg-green-50 dark:bg-green-900/20'
    case 'stopped':
      return 'text-gray-600 dark:text-slate-300 bg-gray-100 dark:bg-slate-800/70'
    default:
      return 'text-gray-800 dark:text-slate-200 bg-gray-100 dark:bg-slate-800/60'
  }
}

onMounted(() => {
  // Auto-load QR code when URL available
  if (props.status.url && !qrCodeValue.value) {
    loadQRCode()
  }
  // Auto-load diagnostics if tunnel is not active (for troubleshooting)
  if (!props.status.active) {
    loadDiagnostics()
  }
})

// Auto-load QR when URL becomes available (e.g. from polling)
watch(
  () => props.status.url,
  (url) => {
    if (url && !qrCodeValue.value) {
      loadQRCode()
    }
  }
)
</script>

<template>
  <div class="tunnel-status text-gray-800 dark:text-slate-100">
    <!-- Status Header -->
    <div class="tunnel-status__header flex items-center justify-between mb-4">
      <div class="tunnel-status__status-row flex items-center gap-2">
        <span>{{ statusIcon }}</span>
        <span
          :class="statusColor"
          class="tunnel-status__status-text font-medium"
        >{{
          statusText
        }}</span>
        <span
          v-if="status.provider"
          class="tunnel-status__provider-badge inline-flex items-center gap-1.5 text-xs px-2 py-0.5 bg-gray-100 dark:bg-slate-800/80 text-gray-600 dark:text-slate-300 rounded-full"
        >
          <img
            v-if="getTunnelProviderIcon(status.provider)"
            :src="getTunnelProviderIcon(status.provider)"
            :alt="status.provider"
            class="tunnel-status__provider-icon h-3.5 w-3.5 shrink-0"
          >
          {{ status.provider }}
        </span>
      </div>
      <span
        v-if="status.remaining_time"
        class="tunnel-status__remaining text-sm text-gray-500 dark:text-slate-300"
      >
        {{ t('remoteAccess.remainingTime', { time: status.remaining_time }) }}
      </span>
    </div>

    <!-- URL Display (show when URL is available; active or connecting with url) -->
    <div
      v-if="status.url"
      class="tunnel-status__panel tunnel-status__panel--url bg-gray-50 dark:bg-slate-900/45 border border-gray-200 dark:border-slate-700 rounded-lg p-4 mb-4"
    >
      <div class="tunnel-status__caption text-sm text-gray-500 dark:text-slate-300 mb-2">
        {{ t('remoteAccess.accessUrl') }}
      </div>
      <div class="tunnel-status__url-row flex items-center gap-2">
        <code
          class="tunnel-status__url-code flex-1 text-sm bg-white dark:bg-slate-950/70 text-gray-900 dark:text-slate-100 px-3 py-2 rounded border border-gray-200 dark:border-slate-700 overflow-x-auto"
        >
          {{ status.url }}
        </code>
        <button
          class="tunnel-status__icon-button p-2 rounded-lg text-gray-500 hover:text-gray-900 hover:bg-gray-100 dark:text-slate-300 dark:hover:text-white dark:hover:bg-white/10 transition-colors"
          :title="t('common.copy')"
          @click="copyUrl"
        >
          <svg
            class="h-5 w-5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"
            />
          </svg>
        </button>
        <button
          class="tunnel-status__icon-button p-2 rounded-lg text-gray-500 hover:text-gray-900 hover:bg-gray-100 dark:text-slate-300 dark:hover:text-white dark:hover:bg-white/10 transition-colors"
          :title="t('common.openInNewTab')"
          @click="openUrl"
        >
          <svg
            class="h-5 w-5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"
            />
          </svg>
        </button>
        <button
          class="tunnel-status__icon-button p-2 rounded-lg text-gray-500 hover:text-gray-900 hover:bg-gray-100 dark:text-slate-300 dark:hover:text-white dark:hover:bg-white/10 transition-colors"
          :title="t('remoteAccess.showQRCode')"
          @click="loadQRCode"
        >
          <svg
            class="h-5 w-5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M12 4v1m6 11h2m-6 0h-2v4m0-11v3m0 0h.01M12 12h4.01M16 20h4M4 12h4m12 0h.01M5 8h2a1 1 0 001-1V5a1 1 0 00-1-1H5a1 1 0 00-1 1v2a1 1 0 001 1zm12 0h2a1 1 0 001-1V5a1 1 0 00-1-1h-2a1 1 0 00-1 1v2a1 1 0 001 1zM5 20h2a1 1 0 001-1v-2a1 1 0 00-1-1H5a1 1 0 00-1 1v2a1 1 0 001 1z"
            />
          </svg>
        </button>
      </div>

      <!-- QR Code Display (auto-show when URL available) -->
      <div
        v-if="status.url"
        class="tunnel-status__qr-wrap mt-3 flex justify-center"
      >
        <div
          v-if="qrCodeLoading"
          class="py-4"
        >
          <svg
            class="animate-spin h-8 w-8 text-gray-900 dark:text-white"
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
        </div>
        <QRCodeDisplay
          v-else-if="qrCodeValue"
          :value="qrCodeValue"
          alt="QR Code"
          :size="240"
          image-class="tunnel-status__qr rounded-lg border border-gray-200 dark:border-slate-700 bg-white dark:bg-slate-950/70"
          :error-text="t('remoteAccess.qrCodeError')"
        />
        <div
          v-else
          class="tunnel-status__empty text-sm text-gray-500 dark:text-slate-300 py-4"
        >
          {{ t('remoteAccess.qrCodeError') }}
        </div>
      </div>
    </div>

    <!-- Session Info -->
    <div
      v-if="status.active"
      class="tunnel-status__session-grid grid grid-cols-2 gap-4 text-sm mb-4"
    >
      <div>
        <div class="text-gray-500 dark:text-slate-400">
          {{ t('remoteAccess.startedAt') }}
        </div>
        <div class="text-gray-900 dark:text-gray-100">
          {{ status.started_at ? new Date(status.started_at).toLocaleString() : '-' }}
        </div>
      </div>
      <div>
        <div class="text-gray-500 dark:text-slate-400">
          {{ t('remoteAccess.expiresAt') }}
        </div>
        <div class="text-gray-900 dark:text-gray-100">
          {{ status.expires_at ? new Date(status.expires_at).toLocaleString() : '-' }}
        </div>
      </div>
      <div v-if="status.renewed_count !== undefined && status.renewed_count > 0">
        <div class="text-gray-500 dark:text-slate-400">
          {{ t('remoteAccess.renewedCount') }}
        </div>
        <div class="text-gray-900 dark:text-gray-100">
          {{ status.renewed_count }}
        </div>
      </div>
    </div>

    <!-- Disconnect Button (prominent, always visible when tunnel has URL or is active/connecting) -->
    <div
      v-if="status.active || status.connecting || status.url"
      class="mb-4"
    >
      <button
        type="button"
        class="tunnel-status__danger-action w-full px-4 py-3 text-sm font-medium rounded-lg border transition-colors flex items-center justify-center gap-2 bg-red-50 dark:bg-red-900/20 border-red-200 dark:border-red-800 text-red-700 dark:text-red-300 hover:bg-red-100 dark:hover:bg-red-900/40 cursor-pointer"
        @click.stop="handleDisconnect"
      >
        <svg
          class="h-5 w-5 pointer-events-none"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M18.364 18.364A9 9 0 005.636 5.636m12.728 12.728A9 9 0 015.636 5.636m12.728 12.728L5.636 5.636"
          />
        </svg>
        {{ status.connecting ? t('remoteAccess.cancel') : t('remoteAccess.disconnect') }}
      </button>
    </div>

    <!-- Action Buttons -->
    <div class="tunnel-status__action-row flex gap-2 mb-4">
      <button
        class="tunnel-status__toggle-button flex-1 px-3 py-2 text-sm rounded-lg border transition-colors flex items-center justify-center gap-2"
        :class="
          showDiagnostics
            ? 'bg-blue-100 dark:bg-blue-900/20 border-blue-300 dark:border-blue-700 text-blue-700 dark:text-blue-300'
            : 'bg-white dark:bg-slate-900/65 border-gray-200 dark:border-slate-700 text-gray-700 dark:text-slate-300 hover:bg-gray-50 dark:hover:bg-slate-800/80'
        "
        @click="toggleDiagnostics"
      >
        <svg
          class="h-4 w-4"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"
          />
        </svg>
        {{ t('remoteAccess.diagnostics') }}
      </button>
      <button
        class="tunnel-status__toggle-button flex-1 px-3 py-2 text-sm rounded-lg border transition-colors flex items-center justify-center gap-2"
        :class="
          showLogs
            ? 'bg-blue-100 dark:bg-blue-900/20 border-blue-300 dark:border-blue-700 text-blue-700 dark:text-blue-300'
            : 'bg-white dark:bg-slate-900/65 border-gray-200 dark:border-slate-700 text-gray-700 dark:text-slate-300 hover:bg-gray-50 dark:hover:bg-slate-800/80'
        "
        @click="toggleLogs"
      >
        <svg
          class="h-4 w-4"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
          />
        </svg>
        {{ t('remoteAccess.logs') }}
      </button>
    </div>

    <!-- Diagnostics Panel -->
    <div
      v-if="showDiagnostics"
      class="tunnel-status__panel bg-gray-50 dark:bg-slate-900/45 border border-gray-200 dark:border-slate-700 rounded-lg p-4 mb-4"
    >
      <div class="flex items-center justify-between mb-3">
        <h4 class="tunnel-status__section-title font-medium text-gray-900 dark:text-white">
          {{ t('remoteAccess.diagnosticsTitle') }}
        </h4>
        <button
          class="tunnel-status__refresh-button p-1.5 rounded-lg text-gray-500 hover:text-gray-900 hover:bg-gray-100 dark:text-slate-300 dark:hover:text-white dark:hover:bg-white/10 transition-colors"
          :title="t('common.refresh')"
          @click="loadDiagnostics"
        >
          <svg
            class="h-4 w-4"
            :class="{ 'animate-spin': diagnosticsLoading }"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
            />
          </svg>
        </button>
      </div>

      <div
        v-if="diagnosticsLoading && !diagnostics"
        class="flex items-center justify-center py-4"
      >
        <svg
          class="animate-spin h-6 w-6 text-gray-900 dark:text-white"
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
      </div>

      <div
        v-else-if="diagnostics"
        class="space-y-3"
      >
        <!-- Status Items -->
        <div class="grid grid-cols-2 gap-3 text-sm">
          <div class="flex items-center gap-2">
            <span :class="diagnostics.tunnel_running ? 'text-green-500' : 'text-gray-400'">
              {{ diagnostics.tunnel_running ? '✓' : '✗' }}
            </span>
            <span class="text-gray-700 dark:text-slate-200">{{
              t('remoteAccess.tunnelRunning')
            }}</span>
          </div>
          <div class="flex items-center gap-2">
            <span :class="diagnostics.firewall_exception ? 'text-green-500' : 'text-amber-500'">
              {{ diagnostics.firewall_exception ? '✓' : '!' }}
            </span>
            <span class="text-gray-700 dark:text-slate-200">{{
              t('remoteAccess.firewallException')
            }}</span>
          </div>
          <div
            v-if="diagnostics.ssh_available !== undefined"
            class="flex items-center gap-2"
          >
            <span :class="diagnostics.ssh_available ? 'text-green-500' : 'text-amber-500'">
              {{ diagnostics.ssh_available ? '✓' : '!' }}
            </span>
            <span class="text-gray-700 dark:text-slate-200">SSH</span>
          </div>
          <div
            v-if="diagnostics.cloudflared_installed !== undefined"
            class="flex items-center gap-2"
          >
            <span :class="diagnostics.cloudflared_installed ? 'text-green-500' : 'text-gray-400'">
              {{ diagnostics.cloudflared_installed ? '✓' : '✗' }}
            </span>
            <span class="text-gray-700 dark:text-slate-200">cloudflared</span>
          </div>
        </div>

        <!-- Active Provider -->
        <div
          v-if="diagnostics.active_provider"
          class="text-sm text-gray-600 dark:text-slate-300"
        >
          {{ t('remoteAccess.provider') }}:
          <span class="font-medium text-gray-900 dark:text-white">{{
            diagnostics.active_provider
          }}</span>
        </div>

        <!-- Platform Info -->
        <div class="text-sm text-gray-500 dark:text-slate-400">
          {{ t('remoteAccess.platform') }}: {{ diagnostics.os?.platform || 'unknown' }}
        </div>

        <!-- Hints -->
        <div
          v-if="diagnostics.hints && diagnostics.hints.length > 0"
          class="tunnel-status__hint-box bg-amber-50 dark:bg-amber-900/20 rounded-lg p-3 border border-amber-200 dark:border-amber-800"
        >
          <div class="flex items-start gap-2">
            <svg
              class="h-5 w-5 text-amber-600 dark:text-amber-400 mt-0.5 flex-shrink-0"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
              />
            </svg>
            <div class="tunnel-status__hint-body text-sm">
              <p
                class="tunnel-status__hint-title font-medium text-amber-800 dark:text-amber-200 mb-1"
              >
                {{ t('remoteAccess.troubleshootingHints') }}
              </p>
              <ul class="text-amber-700 dark:text-amber-300 list-disc list-inside space-y-1">
                <li
                  v-for="(hint, index) in diagnostics.hints"
                  :key="index"
                >
                  {{ hint }}
                </li>
              </ul>
            </div>
          </div>
        </div>

        <!-- Recent Errors -->
        <div v-if="diagnostics.recent_errors && diagnostics.recent_errors.length > 0">
          <p
            class="tunnel-status__subheading text-sm font-medium text-gray-700 dark:text-slate-200 mb-2"
          >
            {{ t('remoteAccess.recentErrors') }}
          </p>
          <div class="space-y-2 max-h-40 overflow-y-auto">
            <div
              v-for="error in diagnostics.recent_errors"
              :key="error.id"
              class="text-xs bg-red-50 dark:bg-red-900/20 rounded p-2 border border-red-200 dark:border-red-800"
            >
              <div class="flex items-center justify-between mb-1">
                <span class="font-medium text-red-700 dark:text-red-300">{{
                  error.event_type
                }}</span>
                <span class="text-red-500 dark:text-red-400">{{
                  formatTime(error.created_at)
                }}</span>
              </div>
              <p class="text-red-600 dark:text-red-300 break-all">
                {{ error.message }}
              </p>
            </div>
          </div>
        </div>

        <!-- Active Session -->
        <div v-if="diagnostics.active_session">
          <p
            class="tunnel-status__subheading text-sm font-medium text-gray-700 dark:text-slate-200 mb-2"
          >
            {{ t('remoteAccess.activeSession') }}
          </p>
          <div
            class="text-xs bg-white dark:bg-slate-950/70 rounded p-2 border border-gray-200 dark:border-slate-700"
          >
            <div class="grid grid-cols-2 gap-2">
              <div>
                <span class="text-gray-500 dark:text-slate-400">ID:</span>
                <span class="tunnel-status__inline-gap text-gray-700 dark:text-slate-200 font-mono">{{ diagnostics.active_session.id.slice(0, 8) }}...</span>
              </div>
              <div>
                <span class="text-gray-500 dark:text-slate-400">{{ t('remoteAccess.status') }}:</span>
                <span class="tunnel-status__inline-gap text-gray-700 dark:text-slate-200">{{
                  diagnostics.active_session.status
                }}</span>
              </div>
              <div class="col-span-2">
                <span class="text-gray-500 dark:text-slate-400">{{ t('remoteAccess.startedAt') }}:</span>
                <span class="tunnel-status__inline-gap text-gray-700 dark:text-slate-200">{{
                  formatTime(diagnostics.active_session.started_at)
                }}</span>
              </div>
              <div
                v-if="diagnostics.active_session.error_message"
                class="col-span-2"
              >
                <span class="text-red-500 dark:text-red-400">{{ t('remoteAccess.error') }}:</span>
                <span class="tunnel-status__inline-gap text-red-600 dark:text-red-300">{{
                  diagnostics.active_session.error_message
                }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Logs Panel -->
    <div
      v-if="showLogs"
      class="tunnel-status__panel bg-gray-50 dark:bg-slate-900/45 border border-gray-200 dark:border-slate-700 rounded-lg p-4"
    >
      <div class="flex items-center justify-between mb-3">
        <h4 class="tunnel-status__section-title font-medium text-gray-900 dark:text-white">
          {{ t('remoteAccess.logsTitle') }}
        </h4>
        <button
          class="tunnel-status__refresh-button p-1.5 rounded-lg text-gray-500 hover:text-gray-900 hover:bg-gray-100 dark:text-slate-300 dark:hover:text-white dark:hover:bg-white/10 transition-colors"
          :title="t('common.refresh')"
          @click="loadLogs"
        >
          <svg
            class="h-4 w-4"
            :class="{ 'animate-spin': logsLoading }"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
            />
          </svg>
        </button>
      </div>

      <div
        v-if="logsLoading && (!logs || logs.length === 0)"
        class="flex items-center justify-center py-4"
      >
        <svg
          class="animate-spin h-6 w-6 text-gray-900 dark:text-white"
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
      </div>

      <div
        v-else-if="!logs || logs.length === 0"
        class="tunnel-status__empty text-center py-4 text-sm text-gray-500 dark:text-slate-300"
      >
        {{ t('remoteAccess.noLogs') }}
      </div>

      <div
        v-else
        class="tunnel-status__logs space-y-2 max-h-60 overflow-y-auto"
      >
        <div
          v-for="log in logs"
          :key="log.id"
          class="tunnel-status__log-item text-xs rounded p-2 border"
          :class="getEventTypeColor(log.event_type)"
        >
          <div class="flex items-center justify-between mb-1">
            <span
              class="tunnel-status__log-badge font-medium px-1.5 py-0.5 rounded text-xs"
              :class="getEventTypeColor(log.event_type)"
            >
              {{ log.event_type }}
            </span>
            <span class="text-gray-500 dark:text-slate-400">{{ formatTime(log.created_at) }}</span>
          </div>
          <p class="break-all">
            {{ log.message }}
          </p>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.tunnel-status {
  font-size: 0.62rem;
}

.tunnel-status__header {
  margin-bottom: 0.5rem;
}

.tunnel-status__status-row {
  gap: 0.28rem;
  min-width: 0;
}

.tunnel-status__status-text {
  font-size: 0.58rem;
}

.tunnel-status__provider-badge {
  gap: 0.16rem;
  padding: 0.08rem 0.22rem;
  font-size: 0.44rem;
  line-height: 1.1;
}

.tunnel-status__provider-icon {
  width: 0.5rem;
  height: 0.5rem;
}

.tunnel-status__remaining {
  font-size: 0.52rem;
}

.tunnel-status__panel {
  margin-bottom: 0.5rem;
  padding: 0.5rem;
  border-radius: 0.54rem;
}

.tunnel-status__caption {
  margin-bottom: 0.2rem;
  font-size: 0.52rem;
}

.tunnel-status__url-row {
  gap: 0.24rem;
}

.tunnel-status__url-code {
  padding: 0.28rem 0.36rem;
  border-radius: 0.42rem;
  font-size: 0.52rem;
}

.tunnel-status__icon-button {
  padding: 0.22rem;
  border-radius: 0.42rem;
}

.tunnel-status__qr-wrap {
  margin-top: 0.38rem;
}

.tunnel-status__qr {
  width: min(13.6rem, 100%);
  height: auto;
  aspect-ratio: 1 / 1;
  padding: 0.22rem;
  border-radius: 0.5rem;
}

.tunnel-status__session-grid {
  gap: 0.46rem;
  margin-bottom: 0.5rem;
  font-size: 0.5rem;
}

.tunnel-status__danger-action {
  min-height: 1.8rem;
  padding: 0.34rem 0.46rem;
  border-radius: 0.48rem;
  font-size: 0.56rem;
}

.tunnel-status__action-row {
  gap: 0.28rem;
  margin-bottom: 0.5rem;
}

.tunnel-status__toggle-button {
  min-height: 1.58rem;
  padding: 0.24rem 0.38rem;
  border-radius: 0.46rem;
  font-size: 0.52rem;
}

.tunnel-status__section-title {
  font-size: 0.58rem;
}

.tunnel-status__refresh-button {
  padding: 0.16rem;
  border-radius: 0.36rem;
}

.tunnel-status__hint-box {
  padding: 0.4rem;
  border-radius: 0.48rem;
}

.tunnel-status__hint-body {
  font-size: 0.5rem;
}

.tunnel-status__hint-title {
  margin-bottom: 0.12rem;
  font-size: 0.52rem;
}

.tunnel-status__subheading {
  margin-bottom: 0.22rem;
  font-size: 0.5rem;
}

.tunnel-status__empty {
  padding-top: 0.46rem;
  padding-bottom: 0.46rem;
  font-size: 0.5rem;
}

.tunnel-status__logs {
  gap: 0.22rem;
  max-height: 9rem;
}

.tunnel-status__log-item {
  padding: 0.24rem;
  border-radius: 0.4rem;
  font-size: 0.46rem;
}

.tunnel-status__log-badge {
  padding: 0.06rem 0.16rem;
  font-size: 0.42rem;
}

.tunnel-status__inline-gap {
  margin-inline-start: 0.25rem;
}

@media (max-width: 639px) {
  .tunnel-status__header {
    align-items: flex-start;
    flex-direction: column;
  }

  .tunnel-status__url-row,
  .tunnel-status__action-row {
    flex-wrap: wrap;
  }
}
</style>
