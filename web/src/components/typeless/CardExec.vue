<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { TypelessCardExec } from '@/types/typeless'

const { t } = useI18n()

const props = defineProps<{
  card: TypelessCardExec
}>()

function unescapeBackticks(s: string): string {
  return s.replace(/`​``/g, '```')
}

const commandRedacted = computed(() => props.card.command_redacted === true || props.card.hide_command === true)
const stdoutRedacted = computed(() => props.card.stdout_redacted === true)
const stderrRedacted = computed(() => props.card.stderr_redacted === true)
const outputRedacted = computed(() => stdoutRedacted.value || stderrRedacted.value)

const command = computed(() => commandRedacted.value ? '' : unescapeBackticks(props.card.command || ''))
const stdout = computed(() => stdoutRedacted.value ? '' : unescapeBackticks(props.card.stdout || ''))
const stderr = computed(() => stderrRedacted.value ? '' : unescapeBackticks(props.card.stderr || ''))
const isRunning = computed(() => props.card.status === 'running' || props.card._streaming)
const isError = computed(() => props.card.status === 'error' || (props.card.exit_code != null && props.card.exit_code !== 0))
const hasOutput = computed(() => !!(stdout.value || stderr.value))
const outputText = computed(() => {
  if (stderr.value) return stderr.value
  if (stdout.value) return stdout.value
  return ''
})
const fallbackText = computed(() => {
  if (props.card.message) return props.card.message
  if (outputRedacted.value) return t('execCard.outputHidden', 'Output hidden for safety')
  if (isRunning.value) return '...'
  return t('execCard.noOutput', 'No output')
})
const outputToneClass = computed(() => {
  if (isRunning.value) return 'exec-output--pending'
  if (isError.value) return 'exec-output--error'
  return 'exec-output--success'
})

const showShield = computed(() => props.card.host === 'sandbox' || props.card.host === 'builtin')
const shieldLabel = computed(() => {
  if (props.card.host === 'builtin') return 'Built-in'
  return t('tools.sandboxProtected')
})

const expanded = ref(false)
const showCommand = ref(!commandRedacted.value)
const lineCount = computed(() => outputText.value ? outputText.value.split('\n').length : 0)
const shouldCollapse = computed(() => lineCount.value > 14)
const visibleOutput = computed(() => {
  if (!outputText.value) return ''
  if (expanded.value || !shouldCollapse.value) return outputText.value
  return outputText.value.split('\n').slice(0, 14).join('\n')
})

watch(
  () => [props.card.hide_command, props.card.command_redacted],
  ([hideCommand, commandIsRedacted]) => {
    if (hideCommand === true || commandIsRedacted === true) {
      showCommand.value = false
    }
  }
)
</script>

<template>
  <div class="exec-card">
    <div class="exec-card__header">
      <span class="exec-card__prompt">{{ showCommand ? '$' : '•' }}</span>
      <span v-if="showCommand" class="exec-card__command" :title="command">{{ command }}</span>
      <span v-else class="exec-card__command exec-card__command--muted">{{ t('execCard.commandHidden', 'Command hidden') }}</span>
      <span v-if="showShield" class="exec-card__shield" :title="shieldLabel">
        <svg xmlns="http://www.w3.org/2000/svg" class="exec-card__shield-icon" viewBox="0 0 20 20" fill="currentColor">
          <path fill-rule="evenodd" d="M2.166 4.999A11.954 11.954 0 0010 1.944 11.954 11.954 0 0017.834 5c.11.65.166 1.32.166 2.001 0 5.225-3.34 9.67-8 11.317C5.34 16.67 2 12.225 2 7c0-.682.057-1.35.166-2.001zm11.541 3.708a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
        </svg>
      </span>
      <button
        v-if="command && !commandRedacted"
        class="exec-card__command-toggle"
        @click="showCommand = !showCommand"
      >
        {{ showCommand ? t('execCard.hideCommand', 'Hide command') : t('execCard.showCommand', 'Show command') }}
      </button>
    </div>

    <div class="exec-card__body" :class="outputToneClass">
      <pre v-if="hasOutput" class="exec-card__output">{{ visibleOutput }}</pre>
      <pre v-else class="exec-card__output">{{ fallbackText }}</pre>
    </div>

    <button
      v-if="shouldCollapse"
      class="exec-card__toggle"
      @click="expanded = !expanded"
    >
      {{ expanded ? t('execCard.collapse', 'Show less') : t('execCard.expand', 'Show more') }}
    </button>
  </div>
</template>

<style scoped>
.exec-card {
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 8px;
  overflow: hidden;
  background: #0f172a;
}

.exec-card__header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  background: rgba(15, 23, 42, 0.95);
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}

.exec-card__prompt {
  color: #94a3b8;
  font-size: 12px;
  flex-shrink: 0;
}

.exec-card__command {
  flex: 1;
  min-width: 0;
  color: #e2e8f0;
  font-size: 12px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.exec-card__shield {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: #22c55e;
  background: rgba(34, 197, 94, 0.15);
  border: 1px solid rgba(34, 197, 94, 0.35);
  border-radius: 999px;
  width: 20px;
  height: 20px;
  flex-shrink: 0;
}

.exec-card__shield-icon {
  width: 13px;
  height: 13px;
}

.exec-card__command--muted {
  color: #94a3b8;
}

.exec-card__command-toggle {
  border: 0;
  background: transparent;
  color: #94a3b8;
  font-size: 11px;
  cursor: pointer;
  padding: 0;
  flex-shrink: 0;
}

.exec-card__command-toggle:hover {
  color: #e2e8f0;
}

.exec-card__body {
  padding: 10px;
  max-height: 340px;
  overflow: auto;
}

.exec-card__output {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 12px;
  line-height: 1.45;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
}

.exec-output--success .exec-card__output {
  color: #86efac;
}

.exec-output--error .exec-card__output {
  color: #fca5a5;
}

.exec-output--pending .exec-card__output {
  color: #fcd34d;
}

.exec-card__toggle {
  width: 100%;
  border: 0;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
  background: rgba(15, 23, 42, 0.9);
  color: #94a3b8;
  font-size: 11px;
  padding: 6px 8px;
  cursor: pointer;
}

.exec-card__toggle:hover {
  color: #e2e8f0;
}
</style>
