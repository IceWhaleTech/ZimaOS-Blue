<template>
  <div class="voice-customizer">
    <h3>Voice Settings</h3>

    <div class="setting-group">
      <label>Voice</label>
      <select
        v-model="selectedVoice"
        class="voice-select"
      >
        <option
          v-for="voice in voices"
          :key="voice.id"
          :value="voice.id"
        >
          {{ voice.name }} ({{ voice.language }})
        </option>
      </select>
    </div>

    <div class="setting-group">
      <label>Speech Rate: {{ rate.toFixed(1) }}x</label>
      <input
        v-model.number="rate"
        type="range"
        min="0.5"
        max="2"
        step="0.1"
        class="slider"
      >
    </div>

    <div class="setting-group">
      <label>Pitch: {{ pitch }}</label>
      <input
        v-model.number="pitch"
        type="range"
        min="-50"
        max="50"
        step="1"
        class="slider"
      >
    </div>

    <div class="setting-group">
      <label>Volume: {{ volume }}%</label>
      <input
        v-model.number="volume"
        type="range"
        min="0"
        max="100"
        step="1"
        class="slider"
      >
    </div>

    <div class="button-group">
      <button
        class="btn-preview"
        @click="playPreview"
      >
        ▶ Preview
      </button>
      <button
        class="btn-reset"
        @click="resetDefaults"
      >
        Reset
      </button>
      <button
        class="btn-save"
        @click="saveSettings"
      >
        Save
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useTTSStore } from '@/stores/tts'
import { ttsApi } from '@/api/tts'
import { isTtsSpeechMuted } from '@/utils/ttsPreferences'

useTTSStore()
const voices = ref<{ id: string; name: string; language?: string }[]>([])
const selectedVoice = ref('')
const rate = ref(1.0)
const pitch = ref(0)
const volume = ref(100)

onMounted(async () => {
  try {
    const response = await ttsApi.listVoices()
    voices.value = response.data.voices
    const first = voices.value[0]
    if (first) {
      selectedVoice.value = first.id
    }
  } catch (err) {
    console.error('Failed to load voices:', err)
  }
})

const playPreview = async () => {
  if (isTtsSpeechMuted()) {
    return
  }
  try {
    const text = 'Hello, this is a voice preview.'
    await ttsApi.synthesize(text)
  } catch (err) {
    console.error('Failed to play preview:', err)
  }
}

const resetDefaults = () => {
  rate.value = 1.0
  pitch.value = 0
  volume.value = 100
}

const saveSettings = () => {
  localStorage.setItem(
    'tts-voice-settings',
    JSON.stringify({
      voice: selectedVoice.value,
      rate: rate.value,
      pitch: pitch.value,
      volume: volume.value,
    })
  )
}
</script>

<style scoped>
.voice-customizer {
  padding: 16px;
  background: #f9f9f9;
  border-radius: 8px;
}

h3 {
  margin: 0 0 16px 0;
  font-size: 16px;
}

.setting-group {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 16px;
}

label {
  font-weight: 500;
  font-size: 14px;
}

.voice-select,
.slider {
  padding: 8px;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 14px;
}

.slider {
  cursor: pointer;
}

.button-group {
  display: flex;
  gap: 8px;
  margin-top: 16px;
}

button {
  flex: 1;
  padding: 8px 12px;
  border: none;
  border-radius: 4px;
  font-size: 14px;
  cursor: pointer;
  transition: all 0.2s;
}

.btn-preview {
  background: #28a745;
  color: white;
}

.btn-preview:hover {
  background: #218838;
}

.btn-reset {
  background: #6c757d;
  color: white;
}

.btn-reset:hover {
  background: #5a6268;
}

.btn-save {
  background: #007bff;
  color: white;
}

.btn-save:hover {
  background: #0056b3;
}
</style>
