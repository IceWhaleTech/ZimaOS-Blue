<template>
  <div v-if="showDialog" class="privacy-consent-overlay">
    <div class="privacy-consent-dialog">
      <div class="dialog-header">
        <h2>{{ t('speech.privacyDialog.title', { provider: t('speech.edgeTTSName') }) }}</h2>
        <button class="close-btn" @click="decline">✕</button>
      </div>

      <div class="dialog-content">
        <p class="intro">
          {{ t('speech.privacyDialog.intro', { provider: t('speech.edgeTTSName') }) }}
        </p>

        <div class="warning-box">
          <span class="warning-icon">⚠️</span>
          <div class="warning-content">
            <h3>{{ t('speech.privacyDialog.infoTitle') }}</h3>
            <ul>
              <li>{{ t('speech.privacyDialog.items.textSent') }}</li>
              <li>{{ t('speech.privacyDialog.items.audioCloud') }}</li>
              <li>{{ t('speech.privacyDialog.items.requestLogging') }}</li>
              <li>{{ t('speech.privacyDialog.items.noPersonalData') }}</li>
            </ul>
          </div>
        </div>

        <div class="alternatives-box">
          <h3>{{ t('speech.privacyDialog.alternativesTitle') }}</h3>
          <ul>
            <li><strong>Sherpa-ONNX:</strong> {{ t('speech.sherpaNote') }}</li>
            <li><strong>eSpeak-NG:</strong> {{ t('speech.espeakNote') }}</li>
          </ul>
        </div>

        <div class="checkbox-group">
          <label>
            <input v-model="dontShowAgain" type="checkbox" />
            {{ t('speech.privacyDialog.dontShowAgain') }}
          </label>
        </div>
      </div>

      <div class="dialog-footer">
        <button class="btn-secondary" @click="decline">{{ t('decline') }}</button>
        <a href="#" class="learn-more">{{ t('speech.privacyDialog.learnMore') }}</a>
        <button class="btn-primary" @click="accept">
          {{ t('speech.privacyDialog.acceptContinue') }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ttsApi } from '@/api/tts'

const { t } = useI18n()

const props = defineProps<{
  visible: boolean
}>()

const emit = defineEmits<{
  accept: []
  decline: []
}>()

const showDialog = ref(false)
const dontShowAgain = ref(false)

onMounted(() => {
  showDialog.value = props.visible
})

const accept = async () => {
  try {
    await ttsApi.setConsent('edge-tts', true)
    if (dontShowAgain.value) {
      localStorage.setItem('edge-tts-consent-dismissed', 'true')
    }
    showDialog.value = false
    emit('accept')
  } catch (error) {
    console.error('Failed to save consent:', error)
  }
}

const decline = () => {
  showDialog.value = false
  emit('decline')
}
</script>

<style scoped>
.privacy-consent-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.privacy-consent-dialog {
  background: white;
  border-radius: 8px;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.15);
  max-width: 500px;
  width: 90%;
  max-height: 80vh;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
}

.dialog-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 20px;
  border-bottom: 1px solid #e0e0e0;
}

.dialog-header h2 {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
}

.close-btn {
  background: none;
  border: none;
  font-size: 24px;
  cursor: pointer;
  color: #666;
}

.dialog-content {
  padding: 20px;
  flex: 1;
}

.intro {
  margin: 0 0 16px 0;
  color: #333;
  line-height: 1.5;
}

.warning-box {
  background: #fff3cd;
  border: 1px solid #ffc107;
  border-radius: 6px;
  padding: 12px;
  margin-bottom: 16px;
  display: flex;
  gap: 12px;
}

.warning-icon {
  font-size: 20px;
  flex-shrink: 0;
}

.warning-content h3 {
  margin: 0 0 8px 0;
  font-size: 14px;
  font-weight: 600;
}

.warning-content ul {
  margin: 0;
  padding-left: 20px;
  font-size: 13px;
  color: #333;
}

.warning-content li {
  margin: 4px 0;
}

.alternatives-box {
  background: #f5f5f5;
  border-radius: 6px;
  padding: 12px;
  margin-bottom: 16px;
}

.alternatives-box h3 {
  margin: 0 0 8px 0;
  font-size: 14px;
  font-weight: 600;
}

.alternatives-box ul {
  margin: 0;
  padding-left: 20px;
  font-size: 13px;
  color: #333;
}

.alternatives-box li {
  margin: 4px 0;
}

.checkbox-group {
  margin-top: 16px;
}

.checkbox-group label {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  color: #666;
  cursor: pointer;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding: 20px;
  border-top: 1px solid #e0e0e0;
}

.btn-primary,
.btn-secondary {
  padding: 8px 16px;
  border: none;
  border-radius: 6px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: opacity 0.2s;
}

.btn-primary:hover,
.btn-secondary:hover {
  opacity: 0.9;
}

.btn-primary {
  background: #007bff;
  color: white;
}

.btn-secondary {
  background: #f5f5f5;
  color: #333;
}

.learn-more {
  padding: 8px 16px;
  color: #007bff;
  text-decoration: none;
  font-size: 14px;
  line-height: 20px;
}

.learn-more:hover {
  text-decoration: underline;
}
</style>
