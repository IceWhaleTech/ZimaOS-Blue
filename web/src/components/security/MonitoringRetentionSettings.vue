<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { companionSettingsApi, type RetentionConfig, type StorageInfo } from '@/api/companion'

const { t } = useI18n()

const retentionLoading = ref(false)
const retentionConfig = ref<RetentionConfig>({
  events_days: 7,
  sessions_days: 30,
  alerts_days: 90,
})
const storageInfo = ref<StorageInfo>({
  session_count: 0,
  alert_count: 0,
  event_count: 0,
})
const totalMonitoringRecords = computed(
  () =>
    storageInfo.value.session_count + storageInfo.value.event_count + storageInfo.value.alert_count
)
const operationMessage = ref<{ type: 'success' | 'error'; text: string } | null>(null)
let messageTimer: number | null = null

function setOperationMessage(type: 'success' | 'error', text: string) {
  operationMessage.value = { type, text }
  if (messageTimer !== null) window.clearTimeout(messageTimer)
  messageTimer = window.setTimeout(() => {
    operationMessage.value = null
  }, 3200)
}

function normalizeRetentionDays(value: number): number {
  if (!Number.isFinite(value)) return 1
  return Math.max(1, Math.min(365, Math.round(value)))
}

function updateRetentionDays(field: keyof RetentionConfig, value: number) {
  retentionConfig.value[field] = normalizeRetentionDays(value)
}

async function fetchRetentionSettings() {
  try {
    retentionLoading.value = true
    const response = await companionSettingsApi.getSettings()
    retentionConfig.value = {
      sessions_days: normalizeRetentionDays(response.data.retention.sessions_days),
      events_days: normalizeRetentionDays(response.data.retention.events_days),
      alerts_days: normalizeRetentionDays(response.data.retention.alerts_days),
    }
    storageInfo.value = response.data.storage_info
  } catch (e) {
    console.error('Failed to fetch retention settings:', e)
    setOperationMessage('error', t('common.error'))
  } finally {
    retentionLoading.value = false
  }
}

async function saveRetentionSettings() {
  try {
    retentionLoading.value = true
    retentionConfig.value = {
      sessions_days: normalizeRetentionDays(retentionConfig.value.sessions_days),
      events_days: normalizeRetentionDays(retentionConfig.value.events_days),
      alerts_days: normalizeRetentionDays(retentionConfig.value.alerts_days),
    }
    await companionSettingsApi.updateSettings(retentionConfig.value)
    setOperationMessage('success', t('userdata.retentionSaved'))
    await fetchRetentionSettings()
  } catch (e) {
    console.error('Failed to save retention settings:', e)
    setOperationMessage('error', t('common.error'))
  } finally {
    retentionLoading.value = false
  }
}

onMounted(() => {
  void fetchRetentionSettings()
})

onBeforeUnmount(() => {
  if (messageTimer !== null) window.clearTimeout(messageTimer)
})
</script>

<template>
  <div class="space-y-4">
    <div
      v-if="operationMessage"
      class="rounded-lg px-3 py-2 text-sm border"
      :class="
        operationMessage.type === 'success'
          ? 'border-green-200 bg-green-50 text-green-700 dark:border-green-800 dark:bg-green-900/20 dark:text-green-300'
          : 'border-red-200 bg-red-50 text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-300'
      "
    >
      {{ operationMessage.text }}
    </div>

    <div class="glass-card p-4 sm:p-5 space-y-4">
      <div class="flex items-start gap-3">
        <div
          class="shrink-0 rounded-lg bg-sky-100 dark:bg-sky-900/30 p-2 text-sky-700 dark:text-sky-300"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-5 w-5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="1.8"
              d="M3 7h18M6 7v10a2 2 0 002 2h8a2 2 0 002-2V7M9 11h6m-6 4h4"
            />
          </svg>
        </div>
        <div>
          <h4 class="text-sm font-semibold text-gray-900 dark:text-white">
            {{ t('security.settings.title') }}
          </h4>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t('security.settings.description') }}
          </p>
        </div>
      </div>

      <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
        <div
          class="rounded-xl border border-sky-200/80 dark:border-sky-800/70 bg-sky-50/70 dark:bg-sky-900/10 px-3 py-3"
        >
          <div class="text-xs text-sky-700 dark:text-sky-300">
            {{ t('security.settings.sessions') }}
          </div>
          <div class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">
            {{ storageInfo.session_count }}
          </div>
        </div>
        <div
          class="rounded-xl border border-violet-200/80 dark:border-violet-800/70 bg-violet-50/70 dark:bg-violet-900/10 px-3 py-3"
        >
          <div class="text-xs text-violet-700 dark:text-violet-300">
            {{ t('security.settings.events') }}
          </div>
          <div class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">
            {{ storageInfo.event_count }}
          </div>
        </div>
        <div
          class="rounded-xl border border-amber-200/80 dark:border-amber-800/70 bg-amber-50/70 dark:bg-amber-900/10 px-3 py-3"
        >
          <div class="text-xs text-amber-700 dark:text-amber-300">
            {{ t('security.settings.alerts') }}
          </div>
          <div class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">
            {{ storageInfo.alert_count }}
          </div>
        </div>
      </div>

      <div
        class="rounded-lg border border-gray-200 dark:border-slate-700 bg-gray-50 dark:bg-slate-700/30 px-3 py-2 flex items-center justify-between"
      >
        <span class="text-xs text-gray-500 dark:text-slate-400">{{
          t('userdata.retention.totalRecords')
        }}</span>
        <span class="text-sm font-semibold text-gray-900 dark:text-white">{{
          totalMonitoringRecords
        }}</span>
      </div>
    </div>

    <div class="glass-card p-4 sm:p-5 space-y-4">
      <div>
        <h4 class="text-sm font-semibold text-gray-900 dark:text-white">
          {{ t('security.settings.retentionPolicy') }}
        </h4>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('userdata.retention.policyHint') }}
        </p>
        <p class="mt-1 text-xs text-gray-400 dark:text-slate-500">
          {{ t('userdata.retention.daysRange') }}
        </p>
      </div>

      <div class="space-y-3">
        <div
          class="rounded-xl border border-gray-200 dark:border-slate-700 bg-white dark:bg-slate-800/40 px-3 py-3"
        >
          <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
            <div class="min-w-0">
              <label class="text-sm font-medium text-gray-800 dark:text-slate-200">{{
                t('security.settings.sessionsRetention')
              }}</label>
              <p class="mt-0.5 text-xs text-gray-500 dark:text-slate-400">
                {{ t('userdata.retention.sessionsHint') }}
              </p>
            </div>
            <div class="flex items-center gap-2 sm:shrink-0">
              <input
                :value="retentionConfig.sessions_days"
                type="number"
                min="1"
                max="365"
                class="w-24 px-2.5 py-1.5 text-sm border border-gray-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-700 text-gray-900 dark:text-white"
                @change="
                  updateRetentionDays(
                    'sessions_days',
                    ($event.target as HTMLInputElement).valueAsNumber
                  )
                "
                @blur="
                  updateRetentionDays(
                    'sessions_days',
                    ($event.target as HTMLInputElement).valueAsNumber
                  )
                "
              />
              <span class="text-sm text-gray-500 dark:text-slate-400">{{
                t('security.settings.days')
              }}</span>
            </div>
          </div>
          <p class="mt-2 text-xs text-gray-400 dark:text-slate-500">
            {{ t('userdata.retention.sessionsSuggested') }}
          </p>
        </div>

        <div
          class="rounded-xl border border-gray-200 dark:border-slate-700 bg-white dark:bg-slate-800/40 px-3 py-3"
        >
          <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
            <div class="min-w-0">
              <label class="text-sm font-medium text-gray-800 dark:text-slate-200">{{
                t('security.settings.eventsRetention')
              }}</label>
              <p class="mt-0.5 text-xs text-gray-500 dark:text-slate-400">
                {{ t('userdata.retention.eventsHint') }}
              </p>
            </div>
            <div class="flex items-center gap-2 sm:shrink-0">
              <input
                :value="retentionConfig.events_days"
                type="number"
                min="1"
                max="365"
                class="w-24 px-2.5 py-1.5 text-sm border border-gray-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-700 text-gray-900 dark:text-white"
                @change="
                  updateRetentionDays(
                    'events_days',
                    ($event.target as HTMLInputElement).valueAsNumber
                  )
                "
                @blur="
                  updateRetentionDays(
                    'events_days',
                    ($event.target as HTMLInputElement).valueAsNumber
                  )
                "
              />
              <span class="text-sm text-gray-500 dark:text-slate-400">{{
                t('security.settings.days')
              }}</span>
            </div>
          </div>
          <p class="mt-2 text-xs text-gray-400 dark:text-slate-500">
            {{ t('userdata.retention.eventsSuggested') }}
          </p>
        </div>

        <div
          class="rounded-xl border border-gray-200 dark:border-slate-700 bg-white dark:bg-slate-800/40 px-3 py-3"
        >
          <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
            <div class="min-w-0">
              <label class="text-sm font-medium text-gray-800 dark:text-slate-200">{{
                t('security.settings.alertsRetention')
              }}</label>
              <p class="mt-0.5 text-xs text-gray-500 dark:text-slate-400">
                {{ t('userdata.retention.alertsHint') }}
              </p>
            </div>
            <div class="flex items-center gap-2 sm:shrink-0">
              <input
                :value="retentionConfig.alerts_days"
                type="number"
                min="1"
                max="365"
                class="w-24 px-2.5 py-1.5 text-sm border border-gray-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-700 text-gray-900 dark:text-white"
                @change="
                  updateRetentionDays(
                    'alerts_days',
                    ($event.target as HTMLInputElement).valueAsNumber
                  )
                "
                @blur="
                  updateRetentionDays(
                    'alerts_days',
                    ($event.target as HTMLInputElement).valueAsNumber
                  )
                "
              />
              <span class="text-sm text-gray-500 dark:text-slate-400">{{
                t('security.settings.days')
              }}</span>
            </div>
          </div>
          <p class="mt-2 text-xs text-gray-400 dark:text-slate-500">
            {{ t('userdata.retention.alertsSuggested') }}
          </p>
        </div>
      </div>

      <div
        class="rounded-lg bg-gray-50 dark:bg-slate-700/30 px-3 py-2 text-xs text-gray-500 dark:text-slate-400"
      >
        {{ t('userdata.retention.saveHint') }}
      </div>

      <button
        :disabled="retentionLoading"
        class="w-full px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg text-sm font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
        @click="saveRetentionSettings"
      >
        {{ retentionLoading ? t('common.loading') : t('common.save') }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.glass-card {
  background: rgba(255, 255, 255, 0.8);
  backdrop-filter: blur(8px);
  border-radius: 0.75rem;
  border: 1px solid rgb(229, 231, 235);
}

:root.dark .glass-card {
  background: rgba(30, 41, 59, 0.8);
  border-color: rgb(51, 65, 85);
}
</style>
