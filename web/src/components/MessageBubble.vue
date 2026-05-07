<script setup lang="ts">
import { ref, computed } from 'vue'

const props = defineProps<{
  role: string
  content?: string
  toolCalls?: Array<{ id?: string; name?: string; args?: string }>
  toolCallId?: string
  toolName?: string
  toolArgs?: string
  cached?: boolean
}>()

const expanded = ref(false)
const expandedToolArgs = ref<Set<number>>(new Set())

function decodeJsonEscapes(s: string): string {
  try {
    return JSON.parse(`"${s.replace(/"/g, '\\"')}"`)
  } catch {
    return s.replace(/\\n/g, '\n').replace(/\\t/g, '\t').replace(/\\"/g, '"')
  }
}

function formatToolArgs(raw?: string): string {
  if (!raw) return ''
  const decoded = decodeJsonEscapes(raw)
  try {
    const obj = typeof decoded === 'string' ? JSON.parse(decoded) : decoded
    return JSON.stringify(obj, null, 2)
  } catch {
    return decoded
  }
}

function toolArgsLong(raw?: string): boolean {
  if (!raw) return false
  const formatted = formatToolArgs(raw)
  return formatted.length > 200 || formatted.split('\n').length > 6
}

function toggleToolArgs(idx: number) {
  if (expandedToolArgs.value.has(idx)) {
    expandedToolArgs.value.delete(idx)
  } else {
    expandedToolArgs.value.add(idx)
  }
}

const roleConfig = computed(() => {
  switch (props.role) {
    case 'system':
      return {
        label: 'system',
        bg: 'bg-purple-100 dark:bg-purple-900/20',
        text: 'text-purple-700 dark:text-purple-300',
        border: 'border-purple-200 dark:border-purple-800/30',
        dot: 'bg-purple-400',
      }
    case 'user':
      return {
        label: 'user',
        bg: 'bg-blue-50 dark:bg-blue-900/20',
        text: 'text-blue-700 dark:text-blue-300',
        border: 'border-blue-200 dark:border-blue-800/30',
        dot: 'bg-blue-400',
      }
    case 'assistant':
      return {
        label: 'assistant',
        bg: 'bg-emerald-50 dark:bg-emerald-900/20',
        text: 'text-emerald-700 dark:text-emerald-300',
        border: 'border-emerald-200 dark:border-emerald-800/30',
        dot: 'bg-emerald-400',
      }
    case 'tool':
      return {
        label: props.toolName ? `tool_result: ${props.toolName}` : 'tool_result',
        bg: 'bg-orange-50 dark:bg-orange-900/20',
        text: 'text-orange-700 dark:text-orange-300',
        border: 'border-orange-200 dark:border-orange-800/30',
        dot: 'bg-orange-400',
      }
    default:
      return {
        label: props.role,
        bg: 'bg-gray-50 dark:bg-gray-800/50',
        text: 'text-gray-700 dark:text-gray-300',
        border: 'border-gray-200 dark:border-gray-700',
        dot: 'bg-gray-400',
      }
  }
})

const decodedContent = computed(() => {
  if (!props.content) return ''
  return decodeJsonEscapes(props.content)
})

const needsCollapse = computed(() => {
  if (!decodedContent.value) return false
  return decodedContent.value.length > 300 || decodedContent.value.split('\n').length > 8
})
</script>

<template>
  <div
    class="rounded-lg border px-2.5 py-1.5 text-xs"
    :class="[roleConfig.bg, roleConfig.border]"
  >
    <div class="flex items-center gap-1.5 mb-1">
      <span class="w-1.5 h-1.5 rounded-full flex-shrink-0" :class="roleConfig.dot" />
      <span class="font-medium" :class="roleConfig.text">{{ roleConfig.label }}</span>
      <span
        v-if="cached"
        class="ml-auto inline-flex items-center px-1 py-0.5 rounded-full text-[8px] font-bold bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300 ring-1 ring-amber-200 dark:ring-amber-800/50"
      >
        KV Cached
      </span>
    </div>
    <div class="relative">
      <div
        v-if="decodedContent"
        class="text-gray-600 dark:text-slate-400 whitespace-pre-wrap break-all leading-relaxed font-mono text-[11px]"
        :class="needsCollapse && !expanded ? 'max-h-24 overflow-hidden' : ''"
      >{{ decodedContent }}</div>
      <button
        v-if="needsCollapse"
        class="absolute bottom-0 right-0 text-[10px] px-1.5 py-0.5 rounded bg-white/80 dark:bg-gray-800/80 text-gray-400 dark:text-slate-500 hover:text-gray-600 dark:hover:text-slate-300 transition-colors backdrop-blur-sm"
        @click="expanded = !expanded"
      >
        {{ expanded ? 'Collapse' : 'Expand' }}
      </button>
    </div>
    <div v-if="toolCalls && toolCalls.length > 0" class="space-y-1 mt-1">
      <div
        v-for="(tc, i) in toolCalls"
        :key="i"
        class="rounded bg-orange-50/50 dark:bg-orange-900/10 border border-orange-100 dark:border-orange-900/20 p-1.5"
      >
        <div class="flex items-center gap-1.5 text-[11px]">
          <span class="w-1.5 h-1.5 rounded-full bg-orange-400 flex-shrink-0" />
          <span class="font-mono font-medium text-orange-700 dark:text-orange-300">{{ tc.name || 'unknown' }}</span>
          <span v-if="tc.id" class="text-[9px] text-gray-400 dark:text-slate-500 font-mono truncate">{{ tc.id }}</span>
          <button
            v-if="tc.args && toolArgsLong(tc.args)"
            class="ml-auto text-[10px] text-gray-400 dark:text-slate-500 hover:text-gray-600 dark:hover:text-slate-300 transition-colors flex-shrink-0"
            @click="toggleToolArgs(i)"
          >
            {{ expandedToolArgs.has(i) ? 'Collapse' : 'Expand' }}
          </button>
        </div>
        <pre
          v-if="tc.args"
          class="mt-1 text-[10px] text-gray-500 dark:text-slate-400 font-mono whitespace-pre-wrap break-all leading-relaxed"
          :class="expandedToolArgs.has(i) ? '' : 'max-h-24 overflow-auto'"
        >{{ formatToolArgs(tc.args) }}</pre>
      </div>
    </div>
  </div>
</template>
