<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ToolResultItem } from '@/stores/chat'
import { formatToolWarningCodeLabel } from '@/utils/toolWarnings'

const { t } = useI18n()

const props = defineProps<{
  item: ToolResultItem
  defaultExpanded?: boolean
}>()

const expanded = ref(props.defaultExpanded ?? false)

function toggle() {
  expanded.value = !expanded.value
}

// Truncated output for preview (max 2 lines, 120 chars)
const previewOutput = computed(() => {
  if (!props.item.output) return ''
  const lines = props.item.output.split('\n').slice(0, 2)
  let text = lines.join('\n')
  if (text.length > 120) text = text.slice(0, 120) + '…'
  else if (props.item.output.split('\n').length > 2) text += '…'
  return text
})

const hasOutput = computed(() => !!props.item.output?.trim())
const isShortOutput = computed(() => {
  if (!props.item.output) return true
  return props.item.output.length <= 80 && !props.item.output.includes('\n')
})
const hasStatus = computed(() => !!props.item.status?.trim())
const hasWarningCode = computed(() => !!props.item.warningCode?.trim())
const statusText = computed(() => props.item.status || '')
const warningCodeText = computed(() => props.item.warningCode || '')
const warningLabel = computed(() => formatToolWarningCodeLabel(warningCodeText.value, t, 'label'))
const statusToneClass = computed(() => {
  if (props.item.icon === '✗') return 'tool-detail-card__status--error'
  if (props.item.icon === '⏳') return 'tool-detail-card__status--pending'
  if (hasWarningCode.value) return 'tool-detail-card__status--warning'
  return 'tool-detail-card__status--success'
})
</script>

<template>
  <div
    class="tool-detail-card"
    :class="{ 'tool-detail-card--expandable': hasOutput && !isShortOutput }"
    @click="hasOutput && !isShortOutput ? toggle() : undefined"
  >
    <!-- Command/Input (above) -->
    <div v-if="item.command" class="tool-detail-card__command">
      <span class="tool-detail-card__command-label">$</span>
      <span class="tool-detail-card__command-text">{{ item.command }}</span>
    </div>

    <div v-if="hasWarningCode" class="tool-detail-card__warning-row">
      <span class="tool-detail-card__warning-pill">warning_code={{ warningCodeText }}</span>
      <span v-if="warningLabel" class="tool-detail-card__warning-label">{{ warningLabel }}</span>
    </div>

    <!-- Status/Error -->
    <div v-if="hasStatus" class="tool-detail-card__status" :class="statusToneClass">
      {{ statusText }}
    </div>

    <!-- Output (below) -->
    <div v-if="hasOutput" class="tool-detail-card__output-wrapper">
      <!-- Short inline output -->
      <div v-if="isShortOutput" class="tool-detail-card__output-inline">
        {{ item.output }}
      </div>
      <!-- Preview (collapsed) -->
      <div v-else-if="!expanded" class="tool-detail-card__output-preview">
        {{ previewOutput }}
      </div>
      <!-- Full output (expanded) -->
      <div v-else class="tool-detail-card__output-full">
        <pre>{{ item.output }}</pre>
      </div>
    </div>

    <!-- Toggle button -->
    <button
      v-if="hasOutput && !isShortOutput"
      class="tool-detail-card__toggle"
      :title="expanded ? t('chat.toolDetailCollapse') : t('chat.toolDetailExpand')"
    >
      <svg
        class="w-3 h-3 transition-transform"
        :class="{ 'rotate-180': expanded }"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
      </svg>
    </button>
  </div>
</template>

<style scoped>
.tool-detail-card {
  border: 1px solid #e5e7eb;
  border-radius: 4px;
  padding: 8px 10px;
  margin: 4px 0;
  background: #fff;
}

:root.dark .tool-detail-card {
  border-color: rgba(255, 255, 255, 0.15);
  background: transparent;
}

.tool-detail-card--expandable {
  cursor: pointer;
}

/* Command/Input section */
.tool-detail-card__command {
  display: flex;
  align-items: flex-start;
  gap: 6px;
  font-family: ui-monospace, SFMono-Regular, 'SF Mono', Menlo, monospace;
  font-size: 12px;
  line-height: 1.5;
}

.tool-detail-card__command-label {
  color: #6b7280;
  flex-shrink: 0;
  user-select: none;
}

:root.dark .tool-detail-card__command-label {
  color: rgba(255, 255, 255, 0.5);
}

.tool-detail-card__command-text {
  color: #374151;
  white-space: pre-wrap;
  word-break: break-all;
}

:root.dark .tool-detail-card__command-text {
  color: rgba(255, 255, 255, 0.8);
}

.tool-detail-card__warning-row {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 6px;
  flex-wrap: wrap;
}

.tool-detail-card__warning-pill {
  display: inline-flex;
  align-items: center;
  border-radius: 999px;
  padding: 2px 8px;
  font-size: 11px;
  line-height: 1.4;
  font-family: ui-monospace, SFMono-Regular, 'SF Mono', Menlo, monospace;
  color: #9a3412;
  background: #fff7ed;
  border: 1px solid #fdba74;
}

.tool-detail-card__warning-label {
  font-size: 12px;
  line-height: 1.4;
  color: #9a3412;
}

:root.dark .tool-detail-card__warning-pill {
  color: #fdba74;
  background: rgba(249, 115, 22, 0.16);
  border-color: rgba(249, 115, 22, 0.34);
}

:root.dark .tool-detail-card__warning-label {
  color: #fdba74;
}

/* Output section */
.tool-detail-card__output-wrapper {
  margin-top: 6px;
}

.tool-detail-card__status {
  margin-top: 6px;
  font-size: 12px;
  line-height: 1.45;
  white-space: pre-wrap;
  word-break: break-word;
  padding: 4px 6px;
  border-radius: 4px;
  border: 1px solid transparent;
}

.tool-detail-card__status--success {
  color: #166534;
  background: #f0fdf4;
  border-color: #bbf7d0;
}

.tool-detail-card__status--error {
  color: #991b1b;
  background: #fef2f2;
  border-color: #fecaca;
}

.tool-detail-card__status--pending {
  color: #92400e;
  background: #fffbeb;
  border-color: #fde68a;
}

.tool-detail-card__status--warning {
  color: #9a3412;
  background: #fff7ed;
  border-color: #fdba74;
}

:root.dark .tool-detail-card__status--success {
  color: #86efac;
  background: rgba(34, 197, 94, 0.15);
  border-color: rgba(34, 197, 94, 0.35);
}

:root.dark .tool-detail-card__status--error {
  color: #fca5a5;
  background: rgba(239, 68, 68, 0.14);
  border-color: rgba(239, 68, 68, 0.32);
}

:root.dark .tool-detail-card__status--pending {
  color: #fcd34d;
  background: rgba(245, 158, 11, 0.14);
  border-color: rgba(245, 158, 11, 0.32);
}

:root.dark .tool-detail-card__status--warning {
  color: #fdba74;
  background: rgba(249, 115, 22, 0.16);
  border-color: rgba(249, 115, 22, 0.34);
}

.tool-detail-card__output-inline {
  font-family: ui-monospace, SFMono-Regular, 'SF Mono', Menlo, monospace;
  font-size: 12px;
  color: #6b7280;
  white-space: pre-wrap;
  word-break: break-all;
}

:root.dark .tool-detail-card__output-inline {
  color: rgba(255, 255, 255, 0.6);
}

.tool-detail-card__output-preview {
  font-family: ui-monospace, SFMono-Regular, 'SF Mono', Menlo, monospace;
  font-size: 12px;
  color: #6b7280;
  max-height: 44px;
  overflow: hidden;
  white-space: pre-wrap;
  word-break: break-all;
}

:root.dark .tool-detail-card__output-preview {
  color: rgba(255, 255, 255, 0.5);
}

.tool-detail-card__output-full {
  max-height: 300px;
  overflow: auto;
}

.tool-detail-card__output-full pre {
  font-family: ui-monospace, SFMono-Regular, 'SF Mono', Menlo, monospace;
  font-size: 12px;
  color: #374151;
  white-space: pre-wrap;
  word-break: break-all;
  margin: 0;
}

:root.dark .tool-detail-card__output-full pre {
  color: rgba(255, 255, 255, 0.7);
}

/* Toggle button */
.tool-detail-card__toggle {
  position: absolute;
  inset-inline-end: 8px;
  bottom: 8px;
  padding: 2px;
  border-radius: 2px;
  color: #9ca3af;
  cursor: pointer;
  background: #fff;
  border: 1px solid #e5e7eb;
}

.tool-detail-card__toggle:hover {
  color: #4b5563;
  border-color: #d1d5db;
}

:root.dark .tool-detail-card__toggle {
  background: transparent;
  border-color: rgba(255, 255, 255, 0.15);
}

:root.dark .tool-detail-card__toggle:hover {
  color: rgba(255, 255, 255, 0.7);
  border-color: rgba(255, 255, 255, 0.25);
}
</style>
