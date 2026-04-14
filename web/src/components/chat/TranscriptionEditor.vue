<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'

const props = defineProps<{
  text: string
  language?: string
  confidence?: number
  visible: boolean
}>()

const emit = defineEmits<{
  (e: 'confirm', text: string): void
  (e: 'cancel'): void
  (e: 'update:visible', value: boolean): void
}>()

const { t } = useI18n()

const editedText = ref(props.text)
const isEditing = ref(false)

// Watch for text changes from parent
watch(
  () => props.text,
  (newText) => {
    editedText.value = newText
  }
)

const confidencePercent = computed(() => {
  if (!props.confidence) return null
  return Math.round(props.confidence * 100)
})

const toggleEdit = () => {
  isEditing.value = !isEditing.value
}

const confirmAndSend = () => {
  if (editedText.value.trim()) {
    emit('confirm', editedText.value.trim())
    emit('update:visible', false)
    resetState()
  }
}

const cancel = () => {
  emit('cancel')
  emit('update:visible', false)
  resetState()
}

const resetState = () => {
  editedText.value = ''
  isEditing.value = false
}

// Handle keyboard shortcuts
const handleKeydown = (e: KeyboardEvent) => {
  if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
    e.preventDefault()
    confirmAndSend()
  } else if (e.key === 'Escape') {
    cancel()
  }
}
</script>

<template>
  <Transition name="slide-up">
    <div
      v-if="visible"
      class="transcription-editor"
    >
      <div class="editor-header">
        <span class="title">{{ t('chat.transcription.title') }}</span>
        <div class="meta">
          <span
            v-if="language"
            class="language"
          >{{ language.toUpperCase() }}</span>
          <span
            v-if="confidencePercent"
            class="confidence"
          >
            {{ t('chat.transcription.confidence', { percent: confidencePercent }) }}
          </span>
        </div>
      </div>

      <div class="editor-content">
        <textarea
          v-model="editedText"
          :readonly="!isEditing"
          :class="{ editing: isEditing }"
          :placeholder="t('chat.transcription.placeholder')"
          rows="3"
          @keydown="handleKeydown"
        />
      </div>

      <div class="editor-actions">
        <button
          v-if="!isEditing"
          class="btn-edit"
          @click="toggleEdit"
        >
          <span class="icon">✏️</span>
          {{ t('common.edit') }}
        </button>
        <button
          v-else
          class="btn-done"
          @click="toggleEdit"
        >
          <span class="icon">✓</span>
          {{ t('common.done') }}
        </button>

        <div class="spacer" />

        <button
          class="btn-cancel"
          @click="cancel"
        >
          {{ t('common.cancel') }}
        </button>
        <button
          class="btn-send"
          :disabled="!editedText.trim()"
          @click="confirmAndSend"
        >
          <span class="icon">📤</span>
          {{ t('chat.transcription.send') }}
        </button>
      </div>

      <div class="hint">
        {{ t('chat.transcription.hint') }}
      </div>
    </div>
  </Transition>
</template>

<style scoped>
.transcription-editor {
  background: var(--color-bg-secondary, #f5f5f5);
  border-radius: 12px;
  padding: 16px;
  margin: 8px 0;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.editor-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.title {
  font-weight: 600;
  font-size: 14px;
  color: var(--color-text-primary, #333);
}

.meta {
  display: flex;
  gap: 8px;
  font-size: 12px;
}

.language {
  background: var(--color-primary, #007bff);
  color: white;
  padding: 2px 6px;
  border-radius: 4px;
}

.confidence {
  color: var(--color-text-secondary, #666);
}

.editor-content textarea {
  width: 100%;
  padding: 12px;
  border: 1px solid var(--color-border, #ddd);
  border-radius: 8px;
  font-size: 14px;
  line-height: 1.5;
  resize: none;
  background: var(--color-bg-primary, #fff);
  color: var(--color-text-primary, #333);
  transition:
    border-color 0.2s,
    background-color 0.2s;
}

.editor-content textarea:read-only {
  background: var(--color-bg-tertiary, #f9f9f9);
  cursor: default;
}

.editor-content textarea.editing {
  border-color: var(--color-primary, #007bff);
  background: var(--color-bg-primary, #fff);
}

.editor-content textarea:focus {
  outline: none;
  border-color: var(--color-primary, #007bff);
}

.editor-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 12px;
}

.spacer {
  flex: 1;
}

.editor-actions button {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 8px 16px;
  border: none;
  border-radius: 6px;
  font-size: 14px;
  cursor: pointer;
  transition:
    background-color 0.2s,
    opacity 0.2s;
}

.btn-edit,
.btn-done {
  background: var(--color-bg-tertiary, #e9e9e9);
  color: var(--color-text-primary, #333);
}

.btn-edit:hover,
.btn-done:hover {
  background: var(--color-bg-hover, #ddd);
}

.btn-cancel {
  background: transparent;
  color: var(--color-text-secondary, #666);
}

.btn-cancel:hover {
  background: var(--color-bg-tertiary, #e9e9e9);
}

.btn-send {
  background: var(--color-primary, #007bff);
  color: white;
}

.btn-send:hover:not(:disabled) {
  background: var(--color-primary-dark, #0056b3);
}

.btn-send:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.icon {
  font-size: 14px;
}

.hint {
  margin-top: 8px;
  font-size: 12px;
  color: var(--color-text-tertiary, #999);
  text-align: center;
}

/* Transition */
.slide-up-enter-active,
.slide-up-leave-active {
  transition: all 0.3s ease;
}

.slide-up-enter-from,
.slide-up-leave-to {
  opacity: 0;
  transform: translateY(20px);
}

/* Dark mode */
:root.dark .transcription-editor {
  background: var(--color-bg-secondary, #2a2a2a);
}

:root.dark .editor-content textarea {
  background: var(--color-bg-primary, #1a1a1a);
  border-color: var(--color-border, #444);
  color: var(--color-text-primary, #eee);
}

:root.dark .editor-content textarea:read-only {
  background: var(--color-bg-tertiary, #222);
}
</style>
