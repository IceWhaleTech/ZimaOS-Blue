<template>
  <div class="provider-selector">
    <label>{{ t('speech.ttsProvider') }}</label>
    <select v-model="selectedProvider" class="provider-select" @change="handleChange">
      <option v-for="provider in providers" :key="provider.type" :value="provider.type">
        {{ provider.name }}
      </option>
    </select>
    <p v-if="selectedProvider === 'espeak-ng'" class="provider-note">
      ✓ {{ t('speech.espeakNote') }}
    </p>
    <p v-else-if="selectedProvider === 'sherpa-onnx'" class="provider-note">
      ✓ {{ t('speech.sherpaNote') }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useTTSStore } from '@/stores/tts'

const { t } = useI18n()

const ttsStore = useTTSStore()
const selectedProvider = ref('')
const providers = ref<{ type: string; name: string }[]>([])

onMounted(async () => {
  await ttsStore.loadProviders()
  providers.value = ttsStore.providers
  selectedProvider.value = ttsStore.preferredProvider
})

const handleChange = async () => {
  await ttsStore.setPreferredProvider(selectedProvider.value)
}
</script>

<style scoped>
.provider-selector {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

label {
  font-weight: 600;
  font-size: 14px;
}

.provider-select {
  padding: 8px 12px;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 14px;
  cursor: pointer;
}

.provider-note {
  font-size: 12px;
  color: #666;
  margin: 0;
}
</style>
