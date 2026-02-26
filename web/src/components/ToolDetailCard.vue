<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ToolResultItem } from '@/stores/chat'

const { t } = useI18n()

const props = defineProps<{
  item: ToolResultItem
  defaultExpanded?: boolean
}>()

const expanded = ref(props.defaultExpanded ?? false)

function toggle() {
  expanded.value = !expanded.value
}

// Tool display name via i18n
const displayName = computed(() => {
  const key = `tools.names.${props.item.name}`
  const translated = t(key)
  if (translated !== key) return translated
  return props.item.name.replace(/_/g, ' ').replace(/\b\w/g, c => c.toUpperCase())
})

// Tool icon path
const iconSrc = computed(() => {
  const iconMap: Record<string, string> = {
    exec: '/icons/tools/terminal.svg',
    execute_command: '/icons/tools/terminal.svg',
    browser: '/icons/tools/browser.svg',
    browser_navigate: '/icons/tools/browser.svg',
    browser_click: '/icons/tools/browser.svg',
    browser_read: '/icons/tools/browser.svg',
    browser_screenshot: '/icons/tools/browser.svg',
    memory: '/icons/tools/memory.svg',
    memory_search: '/icons/tools/memory.svg',
    web_search: '/icons/tools/web-search.svg',
    scheduler: '/icons/tools/schedule.svg',
    ui_reviewer: '/icons/tools/eye.svg',
    analyze: '/icons/tools/analyze.svg',
    sandbox: '/icons/tools/sandbox.svg',
    workflows: '/icons/tools/workflow.svg',
    notifications: '/icons/tools/notifications.svg',
    ask: '/icons/tools/question.svg',
  }
  return iconMap[props.item.name] || '/icons/tools/process.svg'
})

// Status color
const statusClass = computed(() => {
  if (props.item.icon === '✓') return 'text-green-600 dark:text-green-400'
  if (props.item.icon === '✗') return 'text-red-500 dark:text-red-400'
  return 'text-yellow-500 dark:text-yellow-400'
})

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
</script>

<template>
  <div
    class="tool-detail-card"
    :class="{ 'tool-detail-card--expandable': hasOutput && !isShortOutput }"
    @click="hasOutput && !isShortOutput ? toggle() : undefined"
  >
    <!-- Header -->
    <div class="tool-detail-card__header">
      <div class="tool-detail-card__title">
        <img :src="iconSrc" :alt="item.name" class="tool-detail-card__icon" />
        <span class="tool-detail-card__name">{{ displayName }}</span>
        <span :class="statusClass" class="tool-detail-card__status-icon">{{ item.icon }}</span>
      </div>
      <div class="tool-detail-card__meta">
        <span v-if="item.status" class="tool-detail-card__duration">{{ item.status }}</span>
        <span v-if="item.host === 'sandbox'" class="tool-detail-card__badge">
          <svg class="h-3 w-3" viewBox="0 0 20 20" fill="currentColor">
            <path fill-rule="evenodd" d="M2.166 4.999A11.954 11.954 0 0010 1.944 11.954 11.954 0 0017.834 5c.11.65.166 1.32.166 2.001 0 5.225-3.34 9.67-8 11.317C5.34 16.67 2 12.225 2 7c0-.682.057-1.35.166-2.001zm11.541 3.708a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
          </svg>
        </span>
        <button
          v-if="hasOutput && !isShortOutput"
          class="tool-detail-card__toggle"
          :title="expanded ? t('chat.toolDetailCollapse') : t('chat.toolDetailExpand')"
        >
          <svg class="w-4 h-4 transition-transform" :class="{ 'rotate-180': expanded }" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
          </svg>
        </button>
      </div>
    </div>

    <!-- Detail line (command/query) -->
    <div v-if="item.command" class="tool-detail-card__detail">
      {{ item.command }}
    </div>

    <!-- Short inline output -->
    <div v-if="hasOutput && isShortOutput" class="tool-detail-card__inline">
      {{ item.output }}
    </div>

    <!-- Preview (collapsed) -->
    <div v-else-if="hasOutput && !expanded" class="tool-detail-card__preview">
      {{ previewOutput }}
    </div>

    <!-- Full output (expanded) -->
    <div v-if="hasOutput && !isShortOutput && expanded" class="tool-detail-card__output">
      <pre>{{ item.output }}</pre>
    </div>

    <!-- No output -->
    <div v-if="!hasOutput && item.icon === '✓'" class="tool-detail-card__empty">
      {{ t('chat.toolDetailCompleted') }}
    </div>
  </div>
</template>

<style scoped>
.tool-detail-card {
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  padding: 8px 12px;
  margin: 4px 0;
  background: #f9fafb;
  transition: border-color 0.15s, background-color 0.15s;
}

:root.dark .tool-detail-card {
  border-color: rgba(255, 255, 255, 0.1);
  background: rgba(255, 255, 255, 0.03);
}

.tool-detail-card--expandable {
  cursor: pointer;
}

.tool-detail-card--expandable:hover {
  border-color: #d1d5db;
  background: #f3f4f6;
}

:root.dark .tool-detail-card--expandable:hover {
  border-color: rgba(255, 255, 255, 0.15);
  background: rgba(255, 255, 255, 0.05);
}

.tool-detail-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.tool-detail-card__title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-weight: 500;
  font-size: 13px;
  color: #1f2937;
  min-width: 0;
}

:root.dark .tool-detail-card__title {
  color: rgba(255, 255, 255, 0.85);
}

.tool-detail-card__icon {
  width: 16px;
  height: 16px;
  flex-shrink: 0;
  opacity: 0.7;
}

:root.dark .tool-detail-card__icon {
  filter: invert(0.85);
}

.tool-detail-card__name {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.tool-detail-card__status-icon {
  flex-shrink: 0;
  font-size: 12px;
}

.tool-detail-card__meta {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.tool-detail-card__duration {
  font-size: 11px;
  color: #6b7280;
  font-variant-numeric: tabular-nums;
}

:root.dark .tool-detail-card__duration {
  color: rgba(255, 255, 255, 0.45);
}

.tool-detail-card__badge {
  color: #9ca3af;
}

.tool-detail-card__toggle {
  padding: 2px;
  border-radius: 4px;
  color: #9ca3af;
  cursor: pointer;
  background: none;
  border: none;
}

.tool-detail-card__toggle:hover {
  color: #4b5563;
}

:root.dark .tool-detail-card__toggle:hover {
  color: rgba(255, 255, 255, 0.7);
}

.tool-detail-card__detail {
  font-size: 12px;
  color: #6b7280;
  margin-top: 4px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  font-family: ui-monospace, SFMono-Regular, 'SF Mono', Menlo, monospace;
}

:root.dark .tool-detail-card__detail {
  color: rgba(255, 255, 255, 0.45);
}

.tool-detail-card__inline {
  font-size: 12px;
  color: #4b5563;
  margin-top: 4px;
  padding: 4px 8px;
  background: #f3f4f6;
  border-radius: 4px;
  font-family: ui-monospace, SFMono-Regular, 'SF Mono', Menlo, monospace;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

:root.dark .tool-detail-card__inline {
  background: rgba(255, 255, 255, 0.05);
  color: rgba(255, 255, 255, 0.6);
}

.tool-detail-card__preview {
  font-size: 12px;
  color: #6b7280;
  margin-top: 4px;
  padding: 4px 8px;
  background: #f3f4f6;
  border-radius: 4px;
  font-family: ui-monospace, SFMono-Regular, 'SF Mono', Menlo, monospace;
  max-height: 44px;
  overflow: hidden;
  white-space: pre-wrap;
  word-break: break-all;
}

:root.dark .tool-detail-card__preview {
  background: rgba(255, 255, 255, 0.05);
  color: rgba(255, 255, 255, 0.5);
}

.tool-detail-card__output {
  margin-top: 4px;
  padding: 8px;
  background: #f3f4f6;
  border-radius: 4px;
  max-height: 300px;
  overflow: auto;
}

:root.dark .tool-detail-card__output {
  background: rgba(255, 255, 255, 0.05);
}

.tool-detail-card__output pre {
  font-size: 12px;
  font-family: ui-monospace, SFMono-Regular, 'SF Mono', Menlo, monospace;
  color: #374151;
  white-space: pre-wrap;
  word-break: break-all;
  margin: 0;
}

:root.dark .tool-detail-card__output pre {
  color: rgba(255, 255, 255, 0.7);
}

.tool-detail-card__empty {
  font-size: 11px;
  color: #9ca3af;
  margin-top: 2px;
}

:root.dark .tool-detail-card__empty {
  color: rgba(255, 255, 255, 0.35);
}
</style>
