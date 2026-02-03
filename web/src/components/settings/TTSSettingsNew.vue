<template>
  <div class="tts-settings">
    <h3>{{ t('speech.ttsSettings') }}</h3>

    <!-- Provider Selector -->
    <div class="setting-group">
      <label>{{ t('speech.provider') }}</label>
      <select v-model="selectedProvider" @change="handleProviderChange" class="provider-select">
        <option value="espeak-ng">eSpeak-NG ({{ t('speech.offline') }})</option>
        <option value="sherpa-onnx">Sherpa-ONNX ({{ t('speech.offline') }})</option>
      </select>
    </div>

    <!-- Voice Customizer -->
    <div class="setting-group">
      <label>{{ t('speech.voiceSettings') }}</label>
      <div class="voice-controls">
        <div class="control">
          <label>{{ t('speech.rate') }}: {{ rate.toFixed(1) }}x</label>
          <input v-model.number="rate" type="range" min="0.5" max="2" step="0.1" />
        </div>
        <div class="control">
          <label>{{ t('speech.pitch') }}: {{ pitch }}</label>
          <input v-model.number="pitch" type="range" min="-50" max="50" step="1" />
        </div>
        <div class="control">
          <label>{{ t('speech.volume') }}: {{ volume }}%</label>
          <input v-model.number="volume" type="range" min="0" max="100" step="1" />
        </div>
      </div>
    </div>

    <!-- Language Packs (eSpeak-NG) - Simplified -->
    <div v-if="selectedProvider === 'espeak-ng'" class="setting-group">
      <label>{{ t('speech.languagePacks') }}</label>
      <div class="language-pack-download">
        <p class="pack-info">{{ t('speech.allPacksInfo', { count: 27, size: '8.5 MB' }) }}</p>
        <button
          @click="downloadAllLanguagePacks"
          :disabled="downloading || allPacksDownloaded"
          class="btn-download-all"
        >
          {{ allPacksDownloaded ? t('speech.allDownloaded') : t('speech.downloadAll') }}
        </button>
      </div>
    </div>

    <!-- Error Message -->
    <div v-if="error" class="error-message">
      {{ error }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useTTSStore } from '@/stores/tts'

const { t } = useI18n()
const ttsStore = useTTSStore()

const selectedProvider = ref('espeak-ng')
const rate = ref(1.0)
const pitch = ref(0)
const volume = ref(100)
const languagePacks = ref<any[]>([])
const downloading = ref(false)
const error = ref('')

const allPacksDownloaded = computed(() => {
  return languagePacks.value.length > 0 && languagePacks.value.every(pack => pack.downloaded)
})

onMounted(async () => {
  await ttsStore.loadProviders()
  await ttsStore.loadLanguagePacks()
  selectedProvider.value = ttsStore.preferredProvider
  languagePacks.value = ttsStore.languagePacks
})

const handleProviderChange = async () => {
  await ttsStore.setPreferredProvider(selectedProvider.value)
}

const downloadAllLanguagePacks = async () => {
  downloading.value = true
  error.value = ''
  try {
    await ttsStore.downloadAllLanguagePacks()
    await ttsStore.loadLanguagePacks()
    languagePacks.value = ttsStore.languagePacks
  } catch (err) {
    error.value = err instanceof Error ? err.message : t('speech.downloadFailed')
  } finally {
    downloading.value = false
  }
}
</script>

<style scoped>
.tts-settings {
  padding: 16px;
  background: #f9f9f9;
  border-radius: 8px;
}

h3 {
  margin: 0 0 16px 0;
  font-size: 16px;
}

.setting-group {
  margin-bottom: 16px;
}

label {
  display: block;
  font-weight: 500;
  margin-bottom: 8px;
  font-size: 14px;
}

.provider-select {
  width: 100%;
  padding: 8px;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 14px;
}

.voice-controls {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.control {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.control input[type="range"] {
  cursor: pointer;
}

.language-pack-download {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.pack-info {
  margin: 0;
  padding: 12px;
  background: white;
  border: 1px solid #e0e0e0;
  border-radius: 4px;
  font-size: 14px;
  color: #333;
}

.btn-download-all {
  padding: 10px 16px;
  border: none;
  border-radius: 4px;
  font-size: 14px;
  cursor: pointer;
  background: #007bff;
  color: white;
  font-weight: 500;
}

.btn-download-all:disabled {
  background: #6c757d;
  cursor: not-allowed;
  opacity: 0.6;
}

.error-message {
  padding: 8px;
  background: #f8d7da;
  color: #721c24;
  border: 1px solid #f5c6cb;
  border-radius: 4px;
  font-size: 12px;
}
</style>
