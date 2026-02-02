<template>
  <div class="provider-selector">
    <label>TTS Provider</label>
    <select v-model="selectedProvider" @change="handleChange" class="provider-select">
      <option v-for="provider in providers" :key="provider.type" :value="provider.type">
        {{ provider.name }}
      </option>
    </select>
    <p v-if="selectedProvider === 'edge-tts'" class="provider-note">
      ⚠️ Online service - requires privacy consent
    </p>
    <p v-else-if="selectedProvider === 'espeak-ng'" class="provider-note">
      ✓ Offline - lightweight (~5MB)
    </p>
    <p v-else-if="selectedProvider === 'sherpa-onnx'" class="provider-note">
      ✓ Offline - premium quality (requires download)
    </p>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useTTSStore } from '@/stores/tts'

const ttsStore = useTTSStore()
const selectedProvider = ref('')
const providers = ref<any[]>([])

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
