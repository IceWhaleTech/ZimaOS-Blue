<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { speechApi, type ASRModel, type TTSModel } from '@/api/speech'

const { t } = useI18n()

const props = defineProps<{
  modelVisible: boolean
  type: 'asr' | 'tts'
}>()

const emit = defineEmits<{
  'update:modelVisible': [value: boolean]
  'downloaded': []
}>()

const models = ref<(ASRModel | TTSModel)[]>([])
const selectedModel = ref('')
const downloading = ref(false)
const downloadProgress = ref(0)
const error = ref<string | null>(null)

const title = computed(() =>
  props.type === 'asr' ? t('speech.prompt.asrTitle') : t('speech.prompt.ttsTitle')
)

const description = computed(() =>
  props.type === 'asr' ? t('speech.prompt.asrDescription') : t('speech.prompt.ttsDescription')
)

async function loadModels() {
  try {
    if (props.type === 'asr') {
      const res = await speechApi.listASRModels()
      models.value = res.data?.models || []
    } else {
      const res = await speechApi.listTTSModels()
      models.value = res.data?.models || []
    }
    // Select first model by default
    if (models.value.length > 0 && !selectedModel.value) {
      selectedModel.value = models.value[0].id
    }
  } catch (e) {
    console.error('Failed to load models:', e)
  }
}

async function handleDownload() {
  if (!selectedModel.value) return

  downloading.value = true
  downloadProgress.value = 0
  error.value = null

  try {
    if (props.type === 'asr') {
      await speechApi.downloadASRModel(selectedModel.value)
    } else {
      await speechApi.downloadTTSModel(selectedModel.value)
    }

    // Poll for progress
    const pollInterval = setInterval(async () => {
      const statusRes = props.type === 'asr'
        ? await speechApi.getASRStatus()
        : await speechApi.getTTSStatus()

      if (statusRes.data?.progress) {
        downloadProgress.value = statusRes.data.progress.percentage
      }

      if (!statusRes.data?.downloading) {
        clearInterval(pollInterval)
        downloading.value = false
        emit('update:modelVisible', false)
        emit('downloaded')
      }
    }, 1000)
  } catch (e) {
    console.error('Download failed:', e)
    error.value = t('speech.downloadError')
    downloading.value = false
  }
}

function handleClose() {
  if (!downloading.value) {
    emit('update:modelVisible', false)
  }
}

// Load models when dialog opens
import { watch } from 'vue'
watch(() => props.modelVisible, (visible) => {
  if (visible) {
    loadModels()
  }
})
</script>

<template>
  <Teleport to="body">
    <Transition name="modal">
      <div
        v-if="modelVisible"
        class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50"
        @click.self="handleClose"
      >
        <div class="bg-white dark:bg-gray-800 rounded-xl shadow-xl max-w-md w-full p-6">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-2">
            {{ title }}
          </h3>
          <p class="text-sm text-gray-600 dark:text-gray-400 mb-4">
            {{ description }}
          </p>

          <!-- Model Selection -->
          <div v-if="!downloading" class="space-y-2 mb-4">
            <div
              v-for="model in models"
              :key="model.id"
              class="flex items-center gap-3 p-3 rounded-lg cursor-pointer transition-colors"
              :class="selectedModel === model.id
                ? 'bg-accent/10 border border-accent'
                : 'bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600'"
              @click="selectedModel = model.id"
            >
              <input
                type="radio"
                :checked="selectedModel === model.id"
                class="w-4 h-4 text-accent"
              />
              <div class="flex-1">
                <div class="font-medium text-gray-900 dark:text-white">{{ model.name }}</div>
                <div class="text-xs text-gray-500 dark:text-gray-400">
                  {{ model.description }} · {{ model.size }}
                </div>
              </div>
            </div>
          </div>

          <!-- Download Progress -->
          <div v-if="downloading" class="mb-4">
            <div class="flex justify-between text-sm mb-2">
              <span class="text-gray-600 dark:text-gray-400">{{ t('speech.downloading') }}</span>
              <span class="font-medium text-gray-900 dark:text-white">{{ downloadProgress.toFixed(0) }}%</span>
            </div>
            <div class="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-2">
              <div
                class="bg-accent h-2 rounded-full transition-all duration-300"
                :style="{ width: `${downloadProgress}%` }"
              ></div>
            </div>
            <p class="text-xs text-gray-500 dark:text-gray-400 mt-2">
              {{ t('speech.resumeSupported') }}
            </p>
          </div>

          <!-- Error -->
          <div v-if="error" class="mb-4 p-3 bg-red-100 dark:bg-red-900/30 text-red-600 dark:text-red-400 rounded-lg text-sm">
            {{ error }}
          </div>

          <!-- Actions -->
          <div class="flex gap-3">
            <button
              v-if="!downloading"
              class="flex-1 px-4 py-2 text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors"
              @click="handleClose"
            >
              {{ t('common.cancel') }}
            </button>
            <button
              class="flex-1 px-4 py-2 text-white bg-accent rounded-lg hover:bg-accent/90 transition-colors disabled:opacity-50"
              :disabled="!selectedModel || downloading"
              @click="handleDownload"
            >
              {{ downloading ? t('speech.downloading') : t('speech.downloadAndUse') }}
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.modal-enter-active,
.modal-leave-active {
  transition: opacity 0.2s ease;
}

.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}
</style>
