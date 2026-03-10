<template>
  <div class="voice-pack-manager">
    <h3>{{ t('speech.languagePacks') }}</h3>
    <p class="description">{{ t('speech.voicePackManager.description') }}</p>

    <div class="pack-list">
      <div v-for="pack in packs" :key="pack.language" class="pack-item">
        <div class="pack-info">
          <span class="pack-name">{{ pack.name }}</span>
          <span class="pack-size">{{ pack.size_kb }} KB</span>
        </div>
        <button
          v-if="!pack.downloaded"
          :disabled="loading"
          class="btn-download"
          @click="downloadPack(pack.language)"
        >
          {{ loading ? t('common.downloading') : t('common.download') }}
        </button>
        <button
          v-else
          :disabled="loading"
          class="btn-delete"
          @click="deletePack(pack.language)"
        >
          {{ t('common.delete') }}
        </button>
      </div>
    </div>

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
const packs = ref<{ language: string; name: string; size_kb: number; downloaded: boolean; downloading?: boolean }[]>([])
const loading = ref(false)
const error = ref('')

onMounted(async () => {
  await ttsStore.loadLanguagePacks()
  packs.value = ttsStore.languagePacks
})

const downloadPack = async (language: string) => {
  loading.value = true
  error.value = ''
  try {
    await ttsStore.downloadLanguagePack(language)
  } catch (err) {
    error.value = err instanceof Error ? err.message : t('speech.downloadFailed')
  } finally {
    loading.value = false
  }
}

const deletePack = async (language: string) => {
  loading.value = true
  error.value = ''
  try {
    await ttsStore.deleteLanguagePack(language)
  } catch (err) {
    error.value = err instanceof Error ? err.message : t('speech.voicePackManager.deleteFailed')
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.voice-pack-manager {
  padding: 16px;
  background: #f9f9f9;
  border-radius: 8px;
}

h3 {
  margin: 0 0 8px 0;
  font-size: 16px;
}

.description {
  margin: 0 0 16px 0;
  font-size: 13px;
  color: #666;
}

.pack-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.pack-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px;
  background: white;
  border: 1px solid #e0e0e0;
  border-radius: 4px;
}

.pack-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.pack-name {
  font-weight: 500;
  font-size: 14px;
}

.pack-size {
  font-size: 12px;
  color: #999;
}

.btn-download,
.btn-delete {
  padding: 6px 12px;
  border: none;
  border-radius: 4px;
  font-size: 12px;
  cursor: pointer;
  transition: all 0.2s;
}

.btn-download {
  background: #007bff;
  color: white;
}

.btn-download:hover:not(:disabled) {
  background: #0056b3;
}

.btn-delete {
  background: #dc3545;
  color: white;
}

.btn-delete:hover:not(:disabled) {
  background: #c82333;
}

button:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.error-message {
  margin-top: 12px;
  padding: 8px 12px;
  background: #f8d7da;
  color: #721c24;
  border: 1px solid #f5c6cb;
  border-radius: 4px;
  font-size: 12px;
}
</style>
