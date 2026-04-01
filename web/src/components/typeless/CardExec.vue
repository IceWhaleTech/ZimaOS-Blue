<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { TypelessCardExec } from '@/types/typeless'
import { formatToolWarningCodeLabel } from '@/utils/toolWarnings'

const { t } = useI18n()

const props = defineProps<{
  card: TypelessCardExec
}>()

interface SummaryItem {
  label: string
  value: string
  tone?: 'neutral' | 'success' | 'error' | 'warning' | 'info'
}

interface OutputSection {
  key: string
  label: string
  text: string
  tone: 'success' | 'error' | 'pending' | 'neutral'
  lineCount: number
}

function unescapeBackticks(s: string): string {
  return s.replace(/`​``/g, '```')
}

function lineCountOf(text: string): number {
  if (!text) return 0
  return text.split('\n').length
}

function formatDurationMs(value?: number): string {
  if (typeof value !== 'number' || !Number.isFinite(value) || value < 0) return ''
  if (value < 1000) return `${Math.round(value)} ms`
  if (value < 60_000) return `${(value / 1000).toFixed(value >= 10_000 ? 0 : 1)} s`

  const minutes = Math.floor(value / 60_000)
  const seconds = Math.round((value % 60_000) / 1000)
  if (seconds === 60) return `${minutes + 1}m`
  if (seconds === 0) return `${minutes}m`
  return `${minutes}m ${seconds}s`
}

function summaryChipClass(tone: SummaryItem['tone'] = 'neutral'): string {
  switch (tone) {
    case 'success':
      return 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-800/50 dark:bg-emerald-900/20 dark:text-emerald-200'
    case 'error':
      return 'border-red-200 bg-red-50 text-red-700 dark:border-red-800/50 dark:bg-red-900/20 dark:text-red-200'
    case 'warning':
      return 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-800/50 dark:bg-amber-900/20 dark:text-amber-200'
    case 'info':
      return 'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800/50 dark:bg-blue-900/20 dark:text-blue-200'
    default:
      return 'border-gray-200 bg-white text-gray-700 dark:border-gray-700 dark:bg-gray-800/80 dark:text-gray-200'
  }
}

function outputSectionClasses(tone: OutputSection['tone']): {
  container: string
  header: string
  badge: string
  text: string
} {
  switch (tone) {
    case 'success':
      return {
        container:
          'border-emerald-200 bg-emerald-50/70 dark:border-emerald-800/50 dark:bg-emerald-900/10',
        header:
          'border-emerald-200/80 bg-emerald-50 text-emerald-800 dark:border-emerald-800/40 dark:bg-emerald-900/20 dark:text-emerald-200',
        badge:
          'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200',
        text: 'text-emerald-950 dark:text-emerald-100',
      }
    case 'error':
      return {
        container: 'border-red-200 bg-red-50/70 dark:border-red-800/50 dark:bg-red-900/10',
        header:
          'border-red-200/80 bg-red-50 text-red-800 dark:border-red-800/40 dark:bg-red-900/20 dark:text-red-200',
        badge: 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-200',
        text: 'text-red-950 dark:text-red-100',
      }
    case 'pending':
      return {
        container:
          'border-amber-200 bg-amber-50/80 dark:border-amber-800/50 dark:bg-amber-900/10',
        header:
          'border-amber-200/80 bg-amber-50 text-amber-800 dark:border-amber-800/40 dark:bg-amber-900/20 dark:text-amber-200',
        badge:
          'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200',
        text: 'text-amber-950 dark:text-amber-100',
      }
    default:
      return {
        container:
          'border-gray-200 bg-gray-50/80 dark:border-gray-700/60 dark:bg-gray-900/40',
        header:
          'border-gray-200 bg-gray-50 text-gray-700 dark:border-gray-700/50 dark:bg-gray-900/50 dark:text-gray-200',
        badge: 'bg-gray-200 text-gray-700 dark:bg-gray-700/80 dark:text-gray-200',
        text: 'text-gray-800 dark:text-gray-100',
      }
  }
}

async function copyCommandText() {
  if (!command.value) return
  try {
    await navigator.clipboard.writeText(command.value)
    commandCopied.value = true
    setTimeout(() => {
      commandCopied.value = false
    }, 2000)
  } catch (err) {
    console.error('Failed to copy command:', err)
  }
}

const commandRedacted = computed(
  () => props.card.command_redacted === true || props.card.hide_command === true
)
const stdoutRedacted = computed(() => props.card.stdout_redacted === true)
const stderrRedacted = computed(() => props.card.stderr_redacted === true)

const command = computed(() =>
  commandRedacted.value ? '' : unescapeBackticks(props.card.command || '')
)
const stdout = computed(() => unescapeBackticks(props.card.stdout || ''))
const stderr = computed(() => unescapeBackticks(props.card.stderr || ''))
const outputRedacted = computed(
  () => (stdoutRedacted.value && !stdout.value) || (stderrRedacted.value && !stderr.value)
)
const isRunning = computed(() => props.card.status === 'running' || props.card._streaming)
const isError = computed(
  () =>
    props.card.status === 'error' || (props.card.exit_code != null && props.card.exit_code !== 0)
)
const fallbackText = computed(() => {
  if (props.card.message) return props.card.message
  if (outputRedacted.value) {
    return t('execCard.outputUnavailable', 'Output unavailable in this card')
  }
  if (isRunning.value) return t('execCard.running', 'Running...')
  return t('execCard.noOutput', 'No output')
})
const localizedTitle = computed(() => t('execCard.title', 'Command Execution'))
const warningCodeLabel = computed(() => formatToolWarningCodeLabel(props.card.warning_code, t))
const warningMessages = computed(() => {
  const items: string[] = []
  const seen = new Set<string>()
  const push = (value?: string) => {
    const trimmed = (value || '').trim()
    if (!trimmed) return
    const normalized = unescapeBackticks(trimmed)
    if (seen.has(normalized)) return
    seen.add(normalized)
    items.push(normalized)
  }
  push(props.card.warning)
  for (const warning of props.card.warnings || []) push(warning)
  return items
})
const hasWarnings = computed(() => !!(warningCodeLabel.value || warningMessages.value.length > 0))
const hostLabel = computed(() => {
  if (!props.card.host) return ''
  return t(`execCard.${props.card.host}`, props.card.host)
})
const riskLabel = computed(() => {
  if (!props.card.risk_level) return ''
  return t(`execCard.risk.${props.card.risk_level}`, props.card.risk_level)
})
const statusConfig = computed(() => {
  if (isRunning.value) {
    return {
      headerBg: 'bg-amber-50 dark:bg-amber-900/20',
      headerBorder: 'border-amber-100 dark:border-amber-800/50',
      border: 'border-amber-200 dark:border-amber-800/60',
      icon: '▶',
      iconBg: 'bg-amber-100 dark:bg-amber-900/40',
      iconColor: 'text-amber-600 dark:text-amber-300',
    }
  }
  if (isError.value) {
    return {
      headerBg: 'bg-red-50 dark:bg-red-900/20',
      headerBorder: 'border-red-100 dark:border-red-800/50',
      border: 'border-red-200 dark:border-red-800/60',
      icon: '✗',
      iconBg: 'bg-red-100 dark:bg-red-900/40',
      iconColor: 'text-red-600 dark:text-red-300',
    }
  }
  return {
    headerBg: 'bg-emerald-50 dark:bg-emerald-900/20',
    headerBorder: 'border-emerald-100 dark:border-emerald-800/50',
    border: 'border-emerald-200 dark:border-emerald-800/60',
    icon: '✓',
    iconBg: 'bg-emerald-100 dark:bg-emerald-900/40',
    iconColor: 'text-emerald-600 dark:text-emerald-300',
  }
})

const outputSections = computed<OutputSection[]>(() => {
  const sections: OutputSection[] = []

  if (stderr.value) {
    sections.push({
      key: 'stderr',
      label: t('execCard.stderr', 'Error Output'),
      text: stderr.value,
      tone: 'error',
      lineCount: lineCountOf(stderr.value),
    })
  }

  if (stdout.value) {
    sections.push({
      key: 'stdout',
      label: t('execCard.stdout', 'Output'),
      text: stdout.value,
      tone: isRunning.value ? 'pending' : 'success',
      lineCount: lineCountOf(stdout.value),
    })
  }

  if (sections.length > 0) return sections

  return [
    {
      key: 'fallback',
      label: t('execCard.output', 'Output'),
      text: fallbackText.value,
      tone: isError.value ? 'error' : isRunning.value ? 'pending' : 'neutral',
      lineCount: lineCountOf(fallbackText.value),
    },
  ]
})

const expanded = ref(false)
const commandCopied = ref(false)
const showCommand = ref(!commandRedacted.value && !!props.card.command)
const maxCollapsedLines = 14

const totalOutputLines = computed(() =>
  outputSections.value.reduce((sum, section) => sum + section.lineCount, 0)
)
const shouldCollapse = computed(() =>
  outputSections.value.some((section) => section.lineCount > maxCollapsedLines)
)
const hiddenLinesCount = computed(() =>
  outputSections.value.reduce(
    (sum, section) => sum + Math.max(0, section.lineCount - maxCollapsedLines),
    0
  )
)
const summaryItems = computed<SummaryItem[]>(() => {
  const items: SummaryItem[] = []

  if (props.card.duration_ms != null) {
    items.push({
      label: t('execCard.duration', 'Duration'),
      value: formatDurationMs(props.card.duration_ms),
    })
  }

  if (props.card.exit_code != null) {
    items.push({
      label: t('execCard.exitCode', 'exit'),
      value: String(props.card.exit_code),
      tone: isError.value ? 'error' : 'neutral',
    })
  }

  if (props.card.session_id) {
    items.push({
      label: t('execCard.session', 'Session'),
      value: props.card.session_id,
      tone: 'info',
    })
  }

  if (totalOutputLines.value > 0) {
    items.push({
      label: t('execCard.lines', 'lines'),
      value: String(totalOutputLines.value),
    })
  }

  if (props.card.truncated) {
    items.push({
      label: t('execCard.output', 'Output'),
      value: t('execCard.outputTruncated', 'truncated'),
      tone: 'warning',
    })
  }

  if (riskLabel.value) {
    items.push({
      label: t('execCard.riskLabel', 'Risk'),
      value: riskLabel.value,
      tone:
        props.card.risk_level === 'critical' || props.card.risk_level === 'high'
          ? 'error'
          : props.card.risk_level === 'medium'
            ? 'warning'
            : 'info',
    })
  }

  return items
})

function visibleSectionText(section: OutputSection): string {
  if (expanded.value || section.lineCount <= maxCollapsedLines) return section.text
  return section.text.split('\n').slice(0, maxCollapsedLines).join('\n')
}

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
  <div
    class="rounded-lg border overflow-hidden bg-white dark:bg-gray-800 shadow-sm"
    :class="statusConfig.border"
  >
    <div
      class="flex items-center gap-2.5 px-4 py-2.5 border-b"
      :class="[statusConfig.headerBg, statusConfig.headerBorder]"
    >
      <span
        class="w-5 h-5 rounded-full flex items-center justify-center text-xs flex-shrink-0"
        :class="[statusConfig.iconBg, statusConfig.iconColor]"
      >
        {{ statusConfig.icon }}
      </span>
      <div class="min-w-0 flex-1">
        <div class="truncate text-sm font-medium text-gray-800 dark:text-gray-100">
          {{ localizedTitle }}
        </div>
      </div>
      <span
        v-if="hostLabel"
        class="inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-semibold tracking-wide bg-gray-200 text-gray-700 dark:bg-gray-700/80 dark:text-gray-200"
      >
        {{ hostLabel }}
      </span>
      <span
        v-if="warningCodeLabel"
        class="inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide bg-amber-100 text-amber-800 dark:bg-amber-900/50 dark:text-amber-200"
      >
        {{ warningCodeLabel }}
      </span>
    </div>

    <div class="px-4 py-3">
      <div
        class="rounded-md border border-gray-200 bg-gray-50 dark:border-gray-700/60 dark:bg-gray-900/40"
      >
        <div class="flex items-start justify-between gap-3 px-3.5 py-3">
          <div class="min-w-0 flex-1">
            <div class="text-[11px] font-semibold uppercase tracking-wide text-gray-400 dark:text-gray-500">
              {{ t('execCard.command', 'Command') }}
            </div>
            <div
              class="mt-1 break-all font-mono text-xs leading-relaxed"
              :class="
                showCommand && command
                  ? 'text-gray-800 dark:text-gray-100'
                  : 'text-gray-400 dark:text-gray-500 italic'
              "
            >
              {{ command ? (showCommand ? `$ ${command}` : t('execCard.commandHidden', 'Command hidden')) : t('execCard.noCommand', 'No command') }}
            </div>
          </div>
          <div class="flex items-center gap-2 shrink-0">
            <button
              v-if="command && !commandRedacted"
              class="inline-flex items-center rounded-full border border-gray-200 bg-white px-2.5 py-1 text-[11px] font-medium text-gray-600 transition-colors hover:bg-gray-100 dark:border-gray-700 dark:bg-gray-800/80 dark:text-gray-200 dark:hover:bg-gray-700"
              @click="showCommand = !showCommand"
            >
              {{
                showCommand
                  ? t('execCard.hideCommand', 'Hide command')
                  : t('execCard.showCommand', 'Show command')
              }}
            </button>
            <button
              v-if="command && showCommand"
              class="inline-flex items-center rounded-full border border-gray-200 bg-white px-2.5 py-1 text-[11px] font-medium text-gray-600 transition-colors hover:bg-gray-100 dark:border-gray-700 dark:bg-gray-800/80 dark:text-gray-200 dark:hover:bg-gray-700"
              @click="copyCommandText"
            >
              {{ commandCopied ? t('execCard.copied', 'Copied!') : t('execCard.copyCommand', 'Copy command') }}
            </button>
          </div>
        </div>
      </div>

      <div v-if="summaryItems.length > 0" class="mt-3 flex flex-wrap gap-2">
        <span
          v-for="item in summaryItems"
          :key="`${item.label}-${item.value}`"
          class="inline-flex items-center gap-1 rounded-full border px-2.5 py-1 text-[11px]"
          :class="summaryChipClass(item.tone)"
        >
          <span class="opacity-70">{{ item.label }}</span>
          <span class="font-semibold">{{ item.value }}</span>
        </span>
      </div>

      <div
        v-if="hasWarnings"
        class="mt-3 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 dark:border-amber-800/60 dark:bg-amber-900/20"
      >
        <div
          class="flex items-center gap-2 text-xs font-semibold text-amber-800 dark:text-amber-200"
        >
          <span>{{ warningCodeLabel || t('toolWarnings.warning', 'Warning') }}</span>
        </div>
        <ul
          v-if="warningMessages.length > 0"
          class="mt-1 space-y-1 text-sm leading-relaxed text-amber-900 dark:text-amber-100"
        >
          <li v-for="(warning, index) in warningMessages" :key="index">{{ warning }}</li>
        </ul>
      </div>

      <div class="mt-3 space-y-2">
        <div
          v-for="section in outputSections"
          :key="section.key"
          class="rounded-md border overflow-hidden"
          :class="outputSectionClasses(section.tone).container"
        >
          <div
            class="flex items-center justify-between gap-3 border-b px-3 py-2"
            :class="outputSectionClasses(section.tone).header"
          >
            <span
              class="inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-semibold tracking-wide"
              :class="outputSectionClasses(section.tone).badge"
            >
              {{ section.label }}
            </span>
            <span class="text-[11px] opacity-70">
              {{ section.lineCount }} {{ t('execCard.lines', 'lines') }}
            </span>
          </div>
          <pre
            class="px-3 py-2 text-xs leading-relaxed font-mono whitespace-pre-wrap break-all overflow-x-auto"
            :class="outputSectionClasses(section.tone).text"
          >{{ visibleSectionText(section) }}</pre>
        </div>
      </div>

      <button
        v-if="shouldCollapse"
        class="mt-3 w-full rounded-md border border-gray-200 bg-gray-50 px-4 py-2 text-sm text-gray-600 transition-colors hover:bg-gray-100 hover:text-gray-800 dark:border-gray-700 dark:bg-gray-900/40 dark:text-gray-300 dark:hover:bg-gray-800 dark:hover:text-gray-100"
        @click="expanded = !expanded"
      >
        <span v-if="expanded">{{ t('execCard.collapse', 'Show less') }}</span>
        <span v-else
          >{{ t('execCard.expand', 'Show more') }} {{ hiddenLinesCount }}
          {{ t('execCard.lines', 'lines') }}</span
        >
      </button>
    </div>
  </div>
</template>
