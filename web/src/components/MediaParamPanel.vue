<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { MediaCategory, MediaIntent, MediaModelInfo } from '@/api/media'
import { getLocalizedMediaModelName } from '@/utils/mediaModelLocalization'

const props = defineProps<{
  intent: MediaIntent
  models: MediaModelInfo[]
  selectedModel: string
  generating: boolean
  ambiguous?: boolean
}>()

const emit = defineEmits<{
  'update:selectedModel': [value: string]
  generate: []
  dismiss: []
  close: []
  confirm: []
  switchCategory: [category: MediaCategory]
}>()

const { t, te } = useI18n()

function translateMediaCategory(category: MediaCategory): string {
  const key = `media.${category}`
  return te(key) ? t(key) : category
}

const categoryLabel = computed(() => {
  return translateMediaCategory(props.intent.category)
})

const categoryIcon = computed(() => {
  switch (props.intent.category) {
    case 't2i':
      return '🎨'
    case 't2v':
      return '🎬'
    case 'i2v':
      return '🎞️'
    case 'i2i':
      return '✏️'
    case 'kf2v':
      return '🎥'
    default:
      return '✨'
  }
})

function onModelChange(e: Event) {
  emit('update:selectedModel', (e.target as HTMLSelectElement).value)
}

function modelLabel(model: MediaModelInfo): string {
  return getLocalizedMediaModelName(model, t, te)
}
</script>

<template>
  <div
    class="mpp"
    role="region"
    :aria-label="ambiguous ? t('media.ambiguousPrompt') : t('media.mediaDetected')"
  >
    <!-- Close button -->
    <button class="mpp-close" @click="emit('close')" :aria-label="t('common.close')">
      <svg
        width="16"
        height="16"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
      >
        <line x1="18" y1="6" x2="6" y2="18" />
        <line x1="6" y1="6" x2="18" y2="18" />
      </svg>
    </button>

    <!-- Ambiguity resolution -->
    <template v-if="ambiguous">
      <div class="mpp-header">
        <span class="mpp-icon mpp-icon-warn">?</span>
        <div class="mpp-header-text">
          <span class="mpp-title">{{ t('media.ambiguousPrompt') }}</span>
          <span class="mpp-subtitle">{{ t('media.ambiguousDescription') }}</span>
        </div>
      </div>

      <div class="mpp-tag">
        <span class="mpp-tag-icon">{{ categoryIcon }}</span>
        <span class="mpp-tag-label">{{ categoryLabel }}</span>
      </div>

      <div v-if="intent.prompt" class="mpp-prompt">
        <div class="mpp-prompt-text">{{ intent.prompt }}</div>
      </div>

      <div class="mpp-actions">
        <button class="mpp-btn mpp-btn-secondary" @click="emit('dismiss')">
          {{ t('media.noChat') }}
        </button>
        <button class="mpp-btn mpp-btn-primary" @click="emit('confirm')">
          {{ t('media.yesGenerate') }}
        </button>
      </div>
    </template>

    <!-- Normal param panel -->
    <template v-else>
      <div class="mpp-header">
        <span class="mpp-icon">{{ categoryIcon }}</span>
        <div class="mpp-header-text">
          <span class="mpp-title">{{ categoryLabel }}</span>
          <span class="mpp-subtitle">{{ t('media.mediaDetected') }}</span>
        </div>
      </div>

      <!-- Category toggle when alternative exists (e.g. i2v ↔ kf2v) -->
      <div v-if="intent.alternative_category" class="mpp-category-toggle">
        <button class="mpp-cat-btn active" :disabled="generating">
          {{ translateMediaCategory(intent.category) }}
        </button>
        <button
          class="mpp-cat-btn"
          @click="emit('switchCategory', intent.alternative_category!)"
          :disabled="generating"
        >
          {{ translateMediaCategory(intent.alternative_category!) }}
          <span class="mpp-cat-hint">{{ t(`media.${intent.alternative_category}Desc`) }}</span>
        </button>
      </div>

      <div v-if="intent.prompt" class="mpp-prompt">
        <div class="mpp-prompt-text">{{ intent.prompt }}</div>
      </div>

      <div class="mpp-controls" v-if="models.length > 0">
        <label class="mpp-label" for="media-model-select">{{ t('media.model') }}</label>
        <select
          id="media-model-select"
          class="mpp-select"
          :value="selectedModel"
          @change="onModelChange"
          :disabled="generating"
        >
          <option v-for="m in models" :key="m.id" :value="m.id">
            {{ modelLabel(m) }}
          </option>
        </select>
      </div>
      <div class="mpp-no-models" v-else role="status">
        {{ t('media.noModels') }}
      </div>

      <div class="mpp-actions">
        <button class="mpp-btn mpp-btn-secondary" @click="emit('dismiss')">
          {{ t('media.noChat') }}
        </button>
        <button
          class="mpp-btn mpp-btn-primary"
          @click="emit('generate')"
          :disabled="generating || models.length === 0"
          :aria-busy="generating"
        >
          <svg v-if="generating" class="mpp-spinner" width="14" height="14" viewBox="0 0 24 24">
            <circle
              cx="12"
              cy="12"
              r="10"
              fill="none"
              stroke="currentColor"
              stroke-width="2.5"
              opacity="0.2"
            />
            <circle
              cx="12"
              cy="12"
              r="10"
              fill="none"
              stroke="currentColor"
              stroke-width="2.5"
              stroke-linecap="round"
              stroke-dasharray="40 23"
            />
          </svg>
          {{ generating ? t('media.generating') : t('media.generate') }}
        </button>
      </div>
    </template>
  </div>
</template>

<style scoped>
.mpp {
  position: relative;
  background: var(--color-bg-primary, #fff);
  border: 1px solid var(--color-border, #e5e7eb);
  border-radius: 12px;
  padding: 14px 16px;
  margin-bottom: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
  animation: mpp-slide-up 0.2s ease;
}

:root.dark .mpp,
[data-theme='dark'] .mpp {
  background: #1e293b;
  border-color: #334155;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.2);
}

@keyframes mpp-slide-up {
  from {
    opacity: 0;
    transform: translateY(8px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

/* Close button */
.mpp-close {
  position: absolute;
  top: 10px;
  right: 10px;
  background: none;
  border: none;
  color: var(--color-text-tertiary, #9ca3af);
  cursor: pointer;
  padding: 4px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.15s;
}
.mpp-close:hover {
  color: var(--color-text-primary, #333);
  background: var(--color-bg-hover, #f3f4f6);
}
:root.dark .mpp-close:hover,
[data-theme='dark'] .mpp-close:hover {
  background: #334155;
  color: #e2e8f0;
}

/* Header */
.mpp-header {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 10px;
  padding-right: 24px;
}

.mpp-icon {
  width: 32px;
  height: 32px;
  border-radius: 8px;
  background: linear-gradient(135deg, #ede9fe, #e0e7ff);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 16px;
  flex-shrink: 0;
}
:root.dark .mpp-icon,
[data-theme='dark'] .mpp-icon {
  background: linear-gradient(135deg, #312e81, #1e3a5f);
}

.mpp-icon-warn {
  background: linear-gradient(135deg, #fef3c7, #fee2e2);
  font-weight: 700;
  font-size: 14px;
  color: #d97706;
}
:root.dark .mpp-icon-warn,
[data-theme='dark'] .mpp-icon-warn {
  background: linear-gradient(135deg, #78350f, #7f1d1d);
  color: #fbbf24;
}

.mpp-header-text {
  display: flex;
  flex-direction: column;
  gap: 1px;
  min-width: 0;
}

.mpp-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text-primary, #1f2937);
}

.mpp-subtitle {
  font-size: 11px;
  color: var(--color-text-tertiary, #9ca3af);
}

/* Category tag (ambiguous mode) */
.mpp-tag {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  background: var(--color-bg-secondary, #f3f4f6);
  border-radius: 6px;
  padding: 4px 10px;
  margin-bottom: 8px;
  font-size: 12px;
  font-weight: 500;
  color: var(--color-text-primary, #374151);
}
:root.dark .mpp-tag,
[data-theme='dark'] .mpp-tag {
  background: #334155;
}

.mpp-tag-icon {
  font-size: 13px;
}

/* Prompt */
.mpp-prompt {
  margin-bottom: 10px;
}

.mpp-prompt-text {
  background: var(--color-bg-secondary, #f9fafb);
  border: 1px solid var(--color-border, #e5e7eb);
  border-radius: 8px;
  padding: 8px 10px;
  font-size: 12.5px;
  line-height: 1.5;
  color: var(--color-text-primary, #374151);
  max-height: 80px;
  overflow-y: auto;
  white-space: pre-wrap;
  word-break: break-word;
}
:root.dark .mpp-prompt-text,
[data-theme='dark'] .mpp-prompt-text {
  background: #0f172a;
  border-color: #334155;
}

/* Controls */
.mpp-controls {
  margin-bottom: 10px;
}

.mpp-label {
  display: block;
  font-size: 11px;
  font-weight: 500;
  color: var(--color-text-secondary, #6b7280);
  margin-bottom: 4px;
  text-transform: uppercase;
  letter-spacing: 0.03em;
}

.mpp-select {
  width: 100%;
  padding: 6px 10px;
  border: 1px solid var(--color-border, #e5e7eb);
  border-radius: 8px;
  font-size: 13px;
  background: var(--color-bg-primary, #fff);
  color: var(--color-text-primary, #374151);
  transition: border-color 0.15s;
}
.mpp-select:focus {
  outline: none;
  border-color: #818cf8;
  box-shadow: 0 0 0 2px rgba(129, 140, 248, 0.15);
}
:root.dark .mpp-select,
[data-theme='dark'] .mpp-select {
  background: #0f172a;
  border-color: #334155;
  color: #e2e8f0;
}

.mpp-no-models {
  font-size: 12px;
  color: var(--color-text-tertiary, #9ca3af);
  margin-bottom: 10px;
}

/* Actions */
.mpp-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.mpp-btn {
  border-radius: 8px;
  padding: 6px 16px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s;
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.mpp-btn-primary {
  background: linear-gradient(135deg, #6366f1, #8b5cf6);
  color: #fff;
  border: none;
  box-shadow: 0 1px 3px rgba(99, 102, 241, 0.3);
}
.mpp-btn-primary:hover:not(:disabled) {
  background: linear-gradient(135deg, #4f46e5, #7c3aed);
  box-shadow: 0 2px 6px rgba(99, 102, 241, 0.4);
}
.mpp-btn-primary:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.mpp-btn-secondary {
  background: transparent;
  color: var(--color-text-secondary, #6b7280);
  border: 1px solid var(--color-border, #e5e7eb);
}
.mpp-btn-secondary:hover {
  background: var(--color-bg-secondary, #f9fafb);
  color: var(--color-text-primary, #374151);
  border-color: var(--color-text-tertiary, #9ca3af);
}
:root.dark .mpp-btn-secondary,
[data-theme='dark'] .mpp-btn-secondary {
  border-color: #475569;
  color: #94a3b8;
}
:root.dark .mpp-btn-secondary:hover,
[data-theme='dark'] .mpp-btn-secondary:hover {
  background: #334155;
  color: #e2e8f0;
}

.mpp-spinner {
  animation: mpp-spin 1s linear infinite;
}

@keyframes mpp-spin {
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
}

/* Category toggle */
.mpp-category-toggle {
  display: flex;
  gap: 6px;
  margin-bottom: 10px;
}

.mpp-cat-btn {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
  padding: 6px 10px;
  border: 1px solid var(--color-border, #e5e7eb);
  border-radius: 8px;
  background: transparent;
  font-size: 12px;
  font-weight: 500;
  color: var(--color-text-secondary, #6b7280);
  cursor: pointer;
  transition: all 0.15s;
}
.mpp-cat-btn.active {
  border-color: #818cf8;
  background: rgba(129, 140, 248, 0.08);
  color: #6366f1;
  cursor: default;
}
.mpp-cat-btn:not(.active):hover:not(:disabled) {
  border-color: #a5b4fc;
  background: var(--color-bg-secondary, #f9fafb);
}
.mpp-cat-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
:root.dark .mpp-cat-btn,
[data-theme='dark'] .mpp-cat-btn {
  border-color: #475569;
  color: #94a3b8;
}
:root.dark .mpp-cat-btn.active,
[data-theme='dark'] .mpp-cat-btn.active {
  border-color: #818cf8;
  background: rgba(129, 140, 248, 0.12);
  color: #a5b4fc;
}

.mpp-cat-hint {
  font-size: 10px;
  font-weight: 400;
  opacity: 0.7;
}
</style>
