<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { securityApi, type FixPreviewResponse, type SecurityScanItem } from '@/api/security'

const { t, te } = useI18n()

const props = defineProps<{
  visible: boolean
  item: SecurityScanItem | null
}>()

const emit = defineEmits<{
  close: []
  confirm: [fixAction: string]
}>()

const loading = ref(false)
const applying = ref(false)
const preview = ref<FixPreviewResponse | null>(null)
const error = ref('')

const FIX_PREVIEW_CHANGE_KEYS: Record<string, string[]> = {
  fix_permission: ['setPermissions', 'owner', 'group', 'others'],
}

function parseFixAction(fixAction: string): { actionType: string; actionPath?: string } {
  const idx = fixAction.indexOf(':')
  if (idx <= 0) return { actionType: fixAction }
  return {
    actionType: fixAction.slice(0, idx),
    actionPath: fixAction.slice(idx + 1),
  }
}

function getFixPreviewI18nParams(previewData: FixPreviewResponse): Record<string, string> {
  const { actionPath } = parseFixAction(previewData.fix_action)
  return actionPath ? { path: actionPath } : {}
}

function getPreviewDescription(previewData: FixPreviewResponse): string {
  const { actionType } = parseFixAction(previewData.fix_action)
  const key = `security.scan.fixPreviews.actions.${actionType}.description`
  return te(key) ? t(key, getFixPreviewI18nParams(previewData)) : previewData.description
}

function getPreviewChanges(previewData: FixPreviewResponse): string[] {
  const { actionType } = parseFixAction(previewData.fix_action)
  const changeKeys = FIX_PREVIEW_CHANGE_KEYS[actionType]
  if (!changeKeys) return previewData.changes

  const params = getFixPreviewI18nParams(previewData)
  const translated = changeKeys
    .map((changeKey) => {
      const key = `security.scan.fixPreviews.actions.${actionType}.changes.${changeKey}`
      return te(key) ? t(key, params) : ''
    })
    .filter((message): message is string => message.length > 0)

  return translated.length === changeKeys.length ? translated : previewData.changes
}

function getPreviewWarning(previewData: FixPreviewResponse): string | undefined {
  if (!previewData.warning) return undefined
  const { actionType } = parseFixAction(previewData.fix_action)
  const key = `security.scan.fixPreviews.actions.${actionType}.warning`
  return te(key) ? t(key, getFixPreviewI18nParams(previewData)) : previewData.warning
}

const translatedPreviewDescription = computed(() => {
  if (!preview.value) return ''
  return getPreviewDescription(preview.value)
})

const translatedPreviewChanges = computed(() => {
  if (!preview.value) return []
  return getPreviewChanges(preview.value)
})

const translatedPreviewWarning = computed(() => {
  if (!preview.value) return undefined
  return getPreviewWarning(preview.value)
})

watch(() => props.visible, async (visible) => {
  if (visible && props.item?.fix_action) {
    await loadPreview()
  } else {
    preview.value = null
    error.value = ''
  }
})

async function loadPreview() {
  if (!props.item?.fix_action) return

  loading.value = true
  error.value = ''

  try {
    const response = await securityApi.previewScanFix(props.item.fix_action)
    preview.value = response.data
  } catch (err) {
    error.value = t('security.scan.previewError')
    console.error('Failed to load fix preview:', err)
  } finally {
    loading.value = false
  }
}

async function handleApply() {
  if (!props.item?.fix_action) return

  applying.value = true

  try {
    emit('confirm', props.item.fix_action)
  } finally {
    applying.value = false
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
            {{ t('security.scan.fixPreview') }}
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
        <div class="p-6 space-y-4 overflow-y-auto flex-1">
          <!-- Loading state -->
          <div v-if="loading" class="flex items-center justify-center py-8">
            <svg class="animate-spin w-8 h-8 text-gray-900 dark:text-gray-300" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
            </svg>
          </div>

          <!-- Error state -->
          <div v-else-if="error" class="text-center py-8">
            <div class="text-red-500 dark:text-red-400 mb-2">
              <svg class="w-12 h-12 mx-auto" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              </svg>
            </div>
            <p class="text-gray-600 dark:text-gray-400">{{ error }}</p>
          </div>

          <!-- Preview content -->
          <template v-else-if="preview">
            <!-- Issue info -->
            <div v-if="item" class="bg-gray-50 dark:bg-gray-700/50 rounded-lg p-4">
              <div class="font-medium text-gray-900 dark:text-white mb-1">{{ item.name }}</div>
              <div class="text-sm text-gray-600 dark:text-gray-400">{{ item.description }}</div>
            </div>

            <!-- Fix description -->
            <div>
              <h3 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                {{ t('security.scan.fixDescription') }}
              </h3>
              <p class="text-sm text-gray-600 dark:text-gray-400">{{ translatedPreviewDescription }}</p>
            </div>

            <!-- Changes list -->
            <div v-if="translatedPreviewChanges.length > 0">
              <h3 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                {{ t('security.scan.changes') }}
              </h3>
              <ul class="space-y-2">
                <li
                  v-for="(change, index) in translatedPreviewChanges"
                  :key="index"
                  class="flex items-start gap-2 text-sm"
                >
                  <svg class="w-4 h-4 text-gray-900 dark:text-gray-300 mt-0.5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
                  </svg>
                  <span class="text-gray-600 dark:text-gray-400">{{ change }}</span>
                </li>
              </ul>
            </div>

            <!-- Reversible indicator -->
            <div class="flex items-center gap-2 text-sm">
              <span :class="preview.reversible ? 'text-green-600 dark:text-green-400' : 'text-yellow-600 dark:text-yellow-400'">
                <svg v-if="preview.reversible" class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                </svg>
                <svg v-else class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                </svg>
              </span>
              <span :class="preview.reversible ? 'text-green-600 dark:text-green-400' : 'text-yellow-600 dark:text-yellow-400'">
                {{ preview.reversible ? t('security.scan.reversible') : t('security.scan.irreversible') }}
              </span>
            </div>

            <!-- Warning -->
            <div v-if="translatedPreviewWarning" class="bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg p-3">
              <div class="flex items-start gap-2">
                <svg class="w-5 h-5 text-yellow-600 dark:text-yellow-400 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                </svg>
                <p class="text-sm text-yellow-700 dark:text-yellow-300">{{ translatedPreviewWarning }}</p>
              </div>
            </div>
          </template>
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
            :disabled="loading || applying || !!error"
            @click="handleApply"
          >
            <svg v-if="applying" class="animate-spin w-4 h-4" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
            </svg>
            <svg v-else class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
            </svg>
            {{ t('security.scan.applyFix') }}
          </button>
        </div>
      </div>
    </div>
  </Transition>
</template>
