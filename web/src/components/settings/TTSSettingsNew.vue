<template>
  <div class="tts-settings">
    <h3>{{ t('speech.ttsSettings') }}</h3>

    <!-- Provider Selector -->
    <div class="setting-group">
      <label>{{ t('speech.provider') }}</label>
      <select v-model="selectedProvider" @change="handleProviderChange" class="provider-select">
        <option value="espeak-ng">eSpeak-NG ({{ t('speech.offline') }})</option>
        <option value="edge-tts">Edge-TTS ({{ t('speech.online') }})</option>
        <option value="sherpa-onnx">Sherpa-ONNX ({{ t('speech.offline') }})</option>
      </select>
      <p class="provider-note" v-if="selectedProvider === 'edge-tts'">
        ⚠️ {{ t('speech.privacyWarning') }}
      </p>
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

    <!-- Language Packs (eSpeak-NG) -->
    <div v-if="selectedProvider === 'espeak-ng'" class="setting-group">
      <label>{{ t('speech.languagePacks') }}</label>
      <div class="language-packs">
        <div v-for="pack in languagePacks" :key="pack.language" class="pack-item">
          <span>{{ pack.name }} ({{ pack.size_kb }} KB)</span>
          <button
            v-if="!pack.downloaded"
            @click="downloadLanguagePack(pack.language)"
            :disabled="downloading"
            class="btn-download"
          >
            {{ t('speech.download') }}
          </button>
          <button
            v-else
            @click="deleteLanguagePack(pack.language)"
            :disabled="downloading"
            class="btn-delete"
          >
            {{ t('speech.delete') }}
          </button>
        </div>
      </div>
    </div>

    <!-- Privacy Consent (Edge-TTS) -->
    <div v-if="selectedProvider === 'edge-tts'" class="setting-group">
      <label>
        <input v-model="edgeTTSConsent" type="checkbox" @change="handleConsentChange" />
        {{ t('speech.acceptPrivacy') }}
      </label>
    </div>

    <!-- Error Message -->
    <div v-if="error" class="error-message">
      {{ error }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useTTSStore } from '@/stores/tts'

const { t } = useI18n()
const ttsStore = useTTSStore()

const selectedProvider = ref('espeak-ng')
const rate = ref(1.0)
const pitch = ref(0)
const volume = ref(100)
const edgeTTSConsent = ref(false)
const languagePacks = ref<any[]>([])
const downloading = ref(false)
const error = ref('')

onMounted(async () => {
  await ttsStore.loadProviders()
  await ttsStore.loadLanguagePacks()
  selectedProvider.value = ttsStore.preferredProvider
  languagePacks.value = ttsStore.languagePacks
})

const handleProviderChange = async () => {
  await ttsStore.setPreferredProvider(selectedProvider.value)
}

const handleConsentChange = async () => {
  await ttsStore.setConsent('edge-tts', edgeTTSConsent.value)
}

const downloadLanguagePack = async (language: string) => {
  downloading.value = true
  error.value = ''
  try {
    await ttsStore.downloadLanguagePack(language)
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Download failed'
  } finally {
    downloading.value = false
  }
}

const deleteLanguagePack = async (language: string) => {
  downloading.value = true
  error.value = ''
  try {
    await ttsStore.deleteLanguagePack(language)
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Delete failed'
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

.provider-note {
  margin: 8px 0 0 0;
  font-size: 12px;
  color: #666;
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

.language-packs {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.pack-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px;
  background: white;
  border: 1px solid #e0e0e0;
  border-radius: 4px;
}

.btn-download,
.btn-delete {
  padding: 4px 8px;
  border: none;
  border-radius: 4px;
  font-size: 12px;
  cursor: pointer;
}

.btn-download {
  background: #007bff;
  color: white;
}

.btn-delete {
  background: #dc3545;
  color: white;
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
