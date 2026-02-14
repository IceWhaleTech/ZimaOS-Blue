<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useCompanionStore } from '@/stores/companion'

const { t } = useI18n()
const companionStore = useCompanionStore()

defineProps<{
  visible: boolean
}>()

const emit = defineEmits<{
  close: []
}>()

// Form state
const format = ref<'json' | 'csv'>('json')
const dateRange = ref<'all' | '7d' | '30d' | 'custom'>('all')
const customFrom = ref('')
const customTo = ref('')
const selectedSessions = ref<string[]>([])
const selectAllSessions = ref(true)
const exporting = ref(false)

const sessions = computed(() => companionStore.sessions)

const dateRangeOptions = [
  { value: 'all', label: 'companion.export.allTime' },
  { value: '7d', label: 'companion.export.last7Days' },
  { value: '30d', label: 'companion.export.last30Days' },
  { value: 'custom', label: 'companion.export.customRange' },
]

const formatOptions = [
  { value: 'json', label: 'JSON', description: 'companion.export.jsonDesc' },
  { value: 'csv', label: 'CSV', description: 'companion.export.csvDesc' },
]

function getDateRange(): { from?: string; to?: string } {
  const now = new Date()

  switch (dateRange.value) {
    case '7d':
      return {
        from: new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000).toISOString(),
        to: now.toISOString(),
      }
    case '30d':
      return {
        from: new Date(now.getTime() - 30 * 24 * 60 * 60 * 1000).toISOString(),
        to: now.toISOString(),
      }
    case 'custom':
      return {
        from: customFrom.value ? new Date(customFrom.value).toISOString() : undefined,
        to: customTo.value ? new Date(customTo.value).toISOString() : undefined,
      }
    default:
      return {}
  }
}

async function handleExport() {
  exporting.value = true

  try {
    const { from, to } = getDateRange()
    const sessionIds = selectAllSessions.value ? undefined : selectedSessions.value

    await companionStore.exportData({
      format: format.value,
      sessionIds,
      from,
      to,
    })

    emit('close')
  } catch (error) {
    console.error('Export failed:', error)
  } finally {
    exporting.value = false
  }
}

function toggleSession(sessionId: string) {
  const index = selectedSessions.value.indexOf(sessionId)
  if (index === -1) {
    selectedSessions.value.push(sessionId)
  } else {
    selectedSessions.value.splice(index, 1)
  }
  selectAllSessions.value = false
}

function toggleSelectAll() {
  selectAllSessions.value = !selectAllSessions.value
  if (selectAllSessions.value) {
    selectedSessions.value = []
  }
}
</script>

<template>
  <Transition
    enter-active-class="transition-opacity duration-200"
    enter-from-class="opacity-0"
    enter-to-class="opacity-100"
    leave-active-class="transition-opacity duration-200"
    leave-from-class="opacity-100"
    leave-to-class="opacity-0"
  >
    <div
      v-if="visible"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
      @click.self="emit('close')"
    >
      <div class="bg-white dark:bg-gray-700 rounded-lg shadow-xl w-full max-w-lg mx-4 max-h-[90vh] overflow-hidden flex flex-col">
        <!-- Header -->
        <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('companion.export.title') }}
          </h2>
          <button
            class="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
            @click="emit('close')"
          >
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <!-- Content -->
        <div class="p-6 space-y-6 overflow-y-auto flex-1">
          <!-- Format selection -->
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              {{ t('companion.export.format') }}
            </label>
            <div class="grid grid-cols-2 gap-3">
              <button
                v-for="opt in formatOptions"
                :key="opt.value"
                :class="[
                  'p-3 rounded-lg border-2 text-left transition-colors',
                  format === opt.value
                    ? 'border-gray-900 dark:border-gray-700 bg-gray-100 dark:bg-gray-600/5'
                    : 'border-gray-200 dark:border-gray-600 hover:border-gray-300 dark:hover:border-gray-500'
                ]"
                @click="format = opt.value as 'json' | 'csv'"
              >
                <div class="font-medium text-gray-900 dark:text-white">{{ opt.label }}</div>
                <div class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                  {{ t(opt.description) }}
                </div>
              </button>
            </div>
          </div>

          <!-- Date range -->
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              {{ t('companion.export.dateRange') }}
            </label>
            <select
              v-model="dateRange"
              class="w-full px-3 py-2 bg-white dark:bg-gray-700 border border-gray-200 dark:border-gray-600 rounded-lg text-gray-900 dark:text-white focus:ring-2 focus:ring-gray-400 focus:border-transparent"
            >
              <option v-for="opt in dateRangeOptions" :key="opt.value" :value="opt.value">
                {{ t(opt.label) }}
              </option>
            </select>

            <!-- Custom date inputs -->
            <div v-if="dateRange === 'custom'" class="mt-3 grid grid-cols-2 gap-3">
              <div>
                <label class="block text-xs text-gray-500 dark:text-gray-400 mb-1">
                  {{ t('companion.export.from') }}
                </label>
                <input
                  v-model="customFrom"
                  type="date"
                  class="w-full px-3 py-2 bg-white dark:bg-gray-700 border border-gray-200 dark:border-gray-600 rounded-lg text-gray-900 dark:text-white text-sm"
                />
              </div>
              <div>
                <label class="block text-xs text-gray-500 dark:text-gray-400 mb-1">
                  {{ t('companion.export.to') }}
                </label>
                <input
                  v-model="customTo"
                  type="date"
                  class="w-full px-3 py-2 bg-white dark:bg-gray-700 border border-gray-200 dark:border-gray-600 rounded-lg text-gray-900 dark:text-white text-sm"
                />
              </div>
            </div>
          </div>

          <!-- Session selection -->
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              {{ t('companion.export.sessions') }}
            </label>

            <div class="border border-gray-200 dark:border-gray-600 rounded-lg overflow-hidden">
              <!-- Select all -->
              <label class="flex items-center gap-3 px-3 py-2 bg-gray-50 dark:bg-gray-700/50 border-b border-gray-200 dark:border-gray-600 cursor-pointer">
                <input
                  type="checkbox"
                  :checked="selectAllSessions"
                  class="w-4 h-4 text-gray-900 dark:text-gray-300 rounded border-gray-300 dark:border-gray-600 focus:ring-gray-400"
                  @change="toggleSelectAll"
                />
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('companion.export.selectAll') }}
                </span>
              </label>

              <!-- Session list -->
              <div class="max-h-40 overflow-y-auto">
                <label
                  v-for="session in sessions"
                  :key="session.id"
                  class="flex items-center gap-3 px-3 py-2 hover:bg-gray-50 dark:hover:bg-gray-700/50 cursor-pointer"
                >
                  <input
                    type="checkbox"
                    :checked="selectAllSessions || selectedSessions.includes(session.id)"
                    :disabled="selectAllSessions"
                    class="w-4 h-4 text-gray-900 dark:text-gray-300 rounded border-gray-300 dark:border-gray-600 focus:ring-gray-400 disabled:opacity-50"
                    @change="toggleSession(session.id)"
                  />
                  <div class="flex-1 min-w-0">
                    <div class="text-sm text-gray-900 dark:text-white truncate">
                      {{ session.user_id || t('companion.anonymous') }}
                    </div>
                    <div class="text-xs text-gray-500 dark:text-gray-400">
                      {{ session.platform }} · {{ session.event_count }} {{ t('companion.events') }}
                    </div>
                  </div>
                </label>
              </div>
            </div>
          </div>
        </div>

        <!-- Footer -->
        <div class="px-6 py-4 border-t border-gray-200 dark:border-gray-700 flex items-center justify-end gap-3">
          <button
            class="px-4 py-2 text-sm text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white transition-colors"
            @click="emit('close')"
          >
            {{ t('common.cancel') }}
          </button>
          <button
            class="px-4 py-2 text-sm bg-gray-700 dark:bg-gray-500 text-white rounded-lg hover:bg-gray-800 dark:hover:bg-gray-400 transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
            :disabled="exporting"
            @click="handleExport"
          >
            <svg v-if="exporting" class="animate-spin w-4 h-4" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
            </svg>
            <svg v-else class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
            </svg>
            {{ t('companion.export.download') }}
          </button>
        </div>
      </div>
    </div>
  </Transition>
</template>
