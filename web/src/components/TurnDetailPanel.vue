<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { devApi, type TurnMetrics, type ToolCallSummary } from '@/api/dev'
import JsonViewer from './JsonViewer.vue'
import MessageBubble from './MessageBubble.vue'

const props = defineProps<{
  visible: boolean
  turnId: string | null
  messageId: string | null
}>()

const emit = defineEmits<{
  close: []
  fork: [messageId: string]
  rewind: [messageId: string]
}>()

const loading = ref(false)
const metrics = ref<TurnMetrics | null>(null)
const error = ref<string | null>(null)
const llmRequestExpanded = ref(false)
const llmResponseExpanded = ref(false)
const messagesExpanded = ref(false)
const expandedToolSchemas = ref<Set<number>>(new Set())
const prevTurnMessages = ref<Array<{ role: string; content?: string; tool_calls?: any[]; tool_call_id?: string; name?: string }>>([])
const prevTurnTools = ref<any[]>([])

function toggleToolSchema(idx: number) {
  if (expandedToolSchemas.value.has(idx)) {
    expandedToolSchemas.value.delete(idx)
  } else {
    expandedToolSchemas.value.add(idx)
  }
}

// Compare two messages for equality (used for cache prefix detection)
function messagesEqual(a: any, b: any): boolean {
  if (a.role !== b.role) return false
  if (a.tool_call_id !== b.tool_call_id) return false
  if ((a.name || '') !== (b.name || '')) return false
  if ((a.content || '') !== (b.content || '')) return false
  // Compare tool_calls names and args (abbreviated)
  const aTc = a.tool_calls || []
  const bTc = b.tool_calls || []
  if (aTc.length !== bTc.length) return false
  for (let i = 0; i < aTc.length; i++) {
    if (aTc[i].name !== bTc[i].name) return false
    if ((aTc[i].args || '') !== (bTc[i].args || '')) return false
  }
  return true
}

// Number of messages from the start that are shared with the previous turn
const cachedPrefixLength = computed(() => {
  if (prevTurnMessages.value.length === 0) return 0
  const curr = parsedMessages.value
  const prev = prevTurnMessages.value
  let i = 0
  while (i < curr.length && i < prev.length && messagesEqual(curr[i], prev[i])) {
    i++
  }
  return i
})

// Whether the tools list is identical to the previous turn
const toolsCached = computed(() => {
  if (prevTurnTools.value.length === 0) return false
  const curr = parsedTools.value
  const prev = prevTurnTools.value
  if (curr.length !== prev.length) return false
  for (let i = 0; i < curr.length; i++) {
    if (curr[i].name !== prev[i].name) return false
  }
  return true
})

async function fetchTurnDetail() {
  if (!props.turnId) return
  loading.value = true
  error.value = null
  metrics.value = null
  prevTurnMessages.value = []
  prevTurnTools.value = []
  try {
    const res = await devApi.getTurnDetail(props.turnId)
    metrics.value = res.data

    // Fetch session turns to find the previous turn for KV cache comparison
    if (res.data.conversation_id) {
      try {
        const turnsRes = await devApi.getSessionTurns(res.data.conversation_id, 100)
        const turns = turnsRes.data
        // Find the turn immediately before the current one by created_at
        const currentIdx = turns.findIndex(t => t.id === props.turnId)
        if (currentIdx > 0) {
          const prevTurn = turns[currentIdx - 1] // API returns oldest first
          if (prevTurn?.llm_request) {
            const prevReq = parseJSON(prevTurn.llm_request)
            if (prevReq?.messages) {
              prevTurnMessages.value = prevReq.messages
            }
            if (prevReq?.tools) {
              prevTurnTools.value = prevReq.tools
            }
          }
        }
      } catch {
        // Ignore errors fetching session turns — just don't show cache info
      }
    }
  } catch (e: any) {
    error.value = e?.message || 'Failed to load turn details'
  } finally {
    loading.value = false
  }
}

watch(() => props.turnId, (id) => {
  if (id && props.visible) fetchTurnDetail()
})

watch(() => props.visible, (v) => {
  if (v && props.turnId && !metrics.value) fetchTurnDetail()
})

function formatCost(usd: number): string {
  if (usd === 0) return '-'
  if (usd < 0.001) return '<$0.001'
  return `$${usd.toFixed(4)}`
}

function formatLatency(ms: number): string {
  if (ms < 1000) return `${Math.round(ms)}ms`
  return `${(ms / 1000).toFixed(2)}s`
}

function formatTokens(n: number): string {
  if (n >= 1000000) return `${(n / 1000000).toFixed(1)}M`
  if (n >= 1000) return `${(n / 1000).toFixed(1)}K`
  return String(n)
}

function parseJSON(s: string): any {
  if (!s) return null
  try { return JSON.parse(s) } catch { return null }
}

function copyForReplay() {
  if (!metrics.value) return
  const req = parseJSON(metrics.value.llm_request)
  navigator.clipboard.writeText(JSON.stringify(req, null, 2))
}

function handleFork() {
  if (props.messageId) emit('fork', props.messageId)
}

function handleRewind() {
  if (props.messageId) emit('rewind', props.messageId)
}

function formatTimestamp(ts: string): string {
  if (!ts) return ''
  const d = new Date(ts)
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

// Parsed LLM request messages
const parsedMessages = computed(() => {
  if (!metrics.value?.llm_request) return []
  const req = parseJSON(metrics.value.llm_request)
  if (!req?.messages) return []
  return req.messages as Array<{ role: string; content?: string; tool_calls?: any[]; tool_call_id?: string; name?: string }>
})

// Parsed LLM request tools
const parsedTools = computed(() => {
  if (!metrics.value?.llm_request) return []
  const req = parseJSON(metrics.value.llm_request)
  if (!req?.tools) return []
  return req.tools as Array<{ name: string; description?: string; input_schema?: any }>
})

// Tool name lookup from tool definitions
const toolNameMap = computed(() => {
  const map = new Map<string, string>()
  for (const t of parsedTools.value) {
    map.set(t.name, t.description || t.name)
  }
  return map
})

// Parsed LLM response
const parsedResponse = computed(() => {
  if (!metrics.value?.llm_response) return null
  return parseJSON(metrics.value.llm_response)
})

// Token breakdown for visual bar
const totalTokens = computed(() => {
  if (!metrics.value) return 0
  return metrics.value.input_tokens + metrics.value.output_tokens
})

const cacheHitRate = computed(() => {
  if (!metrics.value || metrics.value.input_tokens === 0) return 0
  return (metrics.value.cache_read / metrics.value.input_tokens) * 100
})

// Cost breakdown (approximate per-Mtok rates)
const costBreakdown = computed(() => {
  if (!metrics.value) return null
  const m = metrics.value
  // Default Anthropic pricing (Sonnet-class)
  const inputPrice = 3.0
  const outputPrice = 15.0
  const cacheReadPrice = 0.30
  const cacheWritePrice = 3.75

  const inputCost = (m.input_tokens / 1_000_000) * inputPrice
  const outputCost = (m.output_tokens / 1_000_000) * outputPrice
  const cacheReadCost = (m.cache_read / 1_000_000) * cacheReadPrice
  const cacheWriteCost = (m.cache_write / 1_000_000) * cacheWritePrice
  const total = inputCost + outputCost + cacheReadCost + cacheWriteCost

  return { inputCost, outputCost, cacheReadCost, cacheWriteCost, total }
})

function handleCopyJson(text: string) {
  navigator.clipboard.writeText(text)
}

const copiedSessionId = ref(false)

function copySessionId() {
  if (!metrics.value) return
  navigator.clipboard.writeText(metrics.value.conversation_id)
  copiedSessionId.value = true
  setTimeout(() => { copiedSessionId.value = false }, 1500)
}
</script>

<template>
  <Transition name="turn-detail-panel">
    <div
      v-if="visible"
      class="turn-detail-panel-layer"
      @click.self="emit('close')"
    >
      <div
        class="turn-detail-panel-shell border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 shadow-2xl overflow-hidden flex flex-col"
      >
        <!-- Header -->
        <div class="px-4 py-3 border-b border-gray-200 dark:border-gray-700 flex items-start justify-between gap-3">
          <div class="min-w-0">
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">
              Turn Details
            </h3>
            <p v-if="metrics" class="text-xs text-gray-500 dark:text-slate-400 mt-0.5 truncate flex items-center gap-1.5">
              <span>{{ metrics.model }} &middot; {{ formatTimestamp(metrics.created_at) }}</span>
              <button
                class="inline-flex items-center gap-0.5 text-[10px] text-gray-400 dark:text-slate-500 hover:text-gray-600 dark:hover:text-slate-300 transition-colors font-mono"
                title="Copy session ID"
                @click="copySessionId"
              >
                {{ metrics.conversation_id.slice(0, 8) }}...
                <svg v-if="!copiedSessionId" xmlns="http://www.w3.org/2000/svg" class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                </svg>
                <svg v-else xmlns="http://www.w3.org/2000/svg" class="h-3 w-3 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                </svg>
              </button>
            </p>
          </div>
          <button
            class="p-1.5 rounded-lg text-gray-500 dark:text-slate-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
            title="Close"
            @click="emit('close')"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <!-- Content -->
        <div class="flex-1 overflow-y-auto px-4 py-3 space-y-4">
          <!-- Loading -->
          <div v-if="loading" class="flex items-center justify-center py-8">
            <div class="animate-spin h-6 w-6 border-2 border-gray-300 border-t-blue-500 rounded-full" />
          </div>

          <!-- Error -->
          <div v-else-if="error" class="text-sm text-red-500 dark:text-red-400 py-4 text-center">
            {{ error }}
          </div>

          <!-- No data -->
          <div v-else-if="!metrics" class="text-sm text-gray-400 dark:text-slate-500 py-8 text-center">
            Select a message to view turn details
          </div>

          <!-- Metrics content -->
          <template v-else>
            <!-- Status + Latency row -->
            <div class="flex items-center gap-2 flex-wrap">
              <span
                class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium"
                :class="metrics.status === 'success'
                  ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
                  : 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'"
              >
                {{ metrics.status }}
              </span>
              <span class="text-xs text-gray-500 dark:text-slate-400">
                {{ formatLatency(metrics.latency_ms) }}
              </span>
              <span v-if="metrics.ttft_ms > 0" class="text-xs text-gray-500 dark:text-slate-400">
                TTFT {{ formatLatency(metrics.ttft_ms) }}
              </span>
              <span class="text-xs text-gray-500 dark:text-slate-400">
                {{ formatCost(metrics.cost_usd) }}
              </span>
            </div>

            <!-- Cost Breakdown -->
            <div v-if="costBreakdown && metrics.cost_usd > 0" class="space-y-1.5">
              <div class="text-xs font-medium text-gray-700 dark:text-slate-300">Cost Breakdown</div>
              <div class="grid grid-cols-2 gap-1.5 text-[11px]">
                <div class="flex justify-between bg-gray-50 dark:bg-gray-700/50 rounded px-2 py-1">
                  <span class="text-gray-500 dark:text-slate-400">Input</span>
                  <span class="font-mono text-gray-900 dark:text-white">{{ formatCost(costBreakdown.inputCost) }}</span>
                </div>
                <div class="flex justify-between bg-gray-50 dark:bg-gray-700/50 rounded px-2 py-1">
                  <span class="text-gray-500 dark:text-slate-400">Output</span>
                  <span class="font-mono text-gray-900 dark:text-white">{{ formatCost(costBreakdown.outputCost) }}</span>
                </div>
                <div class="flex justify-between bg-gray-50 dark:bg-gray-700/50 rounded px-2 py-1">
                  <span class="text-gray-500 dark:text-slate-400">Cache Read</span>
                  <span class="font-mono text-gray-900 dark:text-white">{{ formatCost(costBreakdown.cacheReadCost) }}</span>
                </div>
                <div class="flex justify-between bg-gray-50 dark:bg-gray-700/50 rounded px-2 py-1">
                  <span class="text-gray-500 dark:text-slate-400">Cache Write</span>
                  <span class="font-mono text-gray-900 dark:text-white">{{ formatCost(costBreakdown.cacheWriteCost) }}</span>
                </div>
              </div>
            </div>

            <!-- Token breakdown -->
            <div class="space-y-1.5">
              <div class="flex items-center justify-between">
                <span class="text-xs font-medium text-gray-700 dark:text-slate-300">Tokens</span>
                <span v-if="cacheHitRate > 0" class="text-[10px] text-gray-400 dark:text-slate-500">
                  Cache hit {{ cacheHitRate.toFixed(0) }}%
                </span>
              </div>
              <div class="grid grid-cols-2 gap-1.5 text-xs">
                <div class="bg-gray-50 dark:bg-gray-700/50 rounded-lg px-2.5 py-1.5">
                  <div class="text-gray-500 dark:text-slate-400">Input</div>
                  <div class="font-mono font-medium text-gray-900 dark:text-white">{{ formatTokens(metrics.input_tokens) }}</div>
                </div>
                <div class="bg-gray-50 dark:bg-gray-700/50 rounded-lg px-2.5 py-1.5">
                  <div class="text-gray-500 dark:text-slate-400">Output</div>
                  <div class="font-mono font-medium text-gray-900 dark:text-white">{{ formatTokens(metrics.output_tokens) }}</div>
                </div>
                <div class="bg-gray-50 dark:bg-gray-700/50 rounded-lg px-2.5 py-1.5">
                  <div class="text-gray-500 dark:text-slate-400">Cache Read</div>
                  <div class="font-mono font-medium text-gray-900 dark:text-white">{{ formatTokens(metrics.cache_read) }}</div>
                </div>
                <div class="bg-gray-50 dark:bg-gray-700/50 rounded-lg px-2.5 py-1.5">
                  <div class="text-gray-500 dark:text-slate-400">Cache Write</div>
                  <div class="font-mono font-medium text-gray-900 dark:text-white">{{ formatTokens(metrics.cache_write) }}</div>
                </div>
              </div>
              <!-- Token bar with tooltips -->
              <div v-if="totalTokens > 0" class="space-y-1">
                <div class="h-1.5 rounded-full bg-gray-100 dark:bg-gray-700 overflow-hidden flex group">
                  <div
                    class="bg-blue-400 dark:bg-blue-500 h-full relative cursor-default"
                    :style="{ width: `${(metrics.input_tokens / totalTokens) * 100}%` }"
                  >
                    <div class="absolute bottom-full left-1/2 -translate-x-1/2 mb-1 px-2 py-0.5 rounded bg-gray-900 dark:bg-gray-100 text-[10px] text-white dark:text-gray-900 whitespace-nowrap opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none font-mono">
                      Input: {{ formatTokens(metrics.input_tokens) }}
                    </div>
                  </div>
                  <div
                    class="bg-emerald-400 dark:bg-emerald-500 h-full relative cursor-default"
                    :style="{ width: `${(metrics.output_tokens / totalTokens) * 100}%` }"
                  >
                    <div class="absolute bottom-full left-1/2 -translate-x-1/2 mb-1 px-2 py-0.5 rounded bg-gray-900 dark:bg-gray-100 text-[10px] text-white dark:text-gray-900 whitespace-nowrap opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none font-mono">
                      Output: {{ formatTokens(metrics.output_tokens) }}
                    </div>
                  </div>
                </div>
                <!-- Legend -->
                <div class="flex items-center gap-3 text-[10px] text-gray-400 dark:text-slate-500">
                  <span class="flex items-center gap-1"><span class="w-2 h-2 rounded-full bg-blue-400 dark:bg-blue-500" />Input</span>
                  <span class="flex items-center gap-1"><span class="w-2 h-2 rounded-full bg-emerald-400 dark:bg-emerald-500" />Output</span>
                </div>
              </div>
            </div>

            <!-- Latency breakdown -->
            <div class="space-y-1.5">
              <div class="text-xs font-medium text-gray-700 dark:text-slate-300">Latency</div>
              <div class="grid grid-cols-2 gap-1.5 text-xs">
                <div class="bg-gray-50 dark:bg-gray-700/50 rounded-lg px-2.5 py-1.5">
                  <div class="text-gray-500 dark:text-slate-400">Total</div>
                  <div class="font-mono font-medium text-gray-900 dark:text-white">{{ formatLatency(metrics.latency_ms) }}</div>
                </div>
                <div v-if="metrics.ttft_ms > 0" class="bg-gray-50 dark:bg-gray-700/50 rounded-lg px-2.5 py-1.5">
                  <div class="text-gray-500 dark:text-slate-400">TTFT</div>
                  <div class="font-mono font-medium text-gray-900 dark:text-white">{{ formatLatency(metrics.ttft_ms) }}</div>
                </div>
                <div v-if="metrics.tool_latency_ms > 0" class="bg-gray-50 dark:bg-gray-700/50 rounded-lg px-2.5 py-1.5">
                  <div class="text-gray-500 dark:text-slate-400">Tools</div>
                  <div class="font-mono font-medium text-gray-900 dark:text-white">{{ formatLatency(metrics.tool_latency_ms) }}</div>
                </div>
              </div>
            </div>

            <!-- Tool calls -->
            <div v-if="metrics.tool_calls && metrics.tool_calls.length > 0" class="space-y-1.5">
              <div class="text-xs font-medium text-gray-700 dark:text-slate-300">
                Tool Calls ({{ metrics.tool_count }})
              </div>
              <div class="space-y-1">
                <div
                  v-for="tc in metrics.tool_calls"
                  :key="tc.tool_call_id"
                  class="flex items-center gap-2 px-2.5 py-1.5 rounded-lg bg-gray-50 dark:bg-gray-700/50 text-xs"
                >
                  <span
                    class="inline-flex items-center justify-center w-4 h-4 rounded-full text-[10px] font-bold flex-shrink-0"
                    :class="tc.success
                      ? 'bg-green-100 text-green-600 dark:bg-green-900/30 dark:text-green-400'
                      : 'bg-red-100 text-red-600 dark:bg-red-900/30 dark:text-red-400'"
                  >
                    {{ tc.success ? '✓' : '✗' }}
                  </span>
                  <span class="font-mono text-gray-900 dark:text-white truncate">{{ tc.name }}</span>
                  <span class="ml-auto text-gray-400 dark:text-slate-500 flex-shrink-0">{{ formatLatency(tc.latency_ms) }}</span>
                </div>
              </div>
            </div>

            <!-- Messages (LLM Request) -->
            <div v-if="parsedMessages.length > 0 || parsedTools.length > 0" class="space-y-1.5">
              <button
                class="flex items-center gap-1.5 text-xs font-medium text-gray-700 dark:text-slate-300 hover:text-gray-900 dark:hover:text-white transition-colors"
                @click="messagesExpanded = !messagesExpanded"
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-3.5 w-3.5 transition-transform"
                  :class="{ 'rotate-90': messagesExpanded }"
                  fill="none" viewBox="0 0 24 24" stroke="currentColor"
                >
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
                </svg>
                Messages
                <span class="text-[10px] text-gray-400 dark:text-slate-500 font-normal">{{ parsedMessages.length }} messages</span>
                <span v-if="parsedTools.length > 0" class="text-[10px] text-gray-400 dark:text-slate-500 font-normal ml-1">
                  + {{ parsedTools.length }} tools
                </span>
                <span v-if="cachedPrefixLength > 0" class="ml-1 inline-flex items-center px-1.5 py-0.5 rounded-full text-[9px] font-bold bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300 ring-1 ring-amber-200 dark:ring-amber-800/50">
                  {{ cachedPrefixLength }} cached
                </span>
              </button>
              <div v-if="messagesExpanded" class="space-y-1.5">
                <!-- Tool definitions (shown first) -->
                <div v-if="parsedTools.length > 0" class="rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
                  <div class="px-2.5 py-1.5 bg-gray-50 dark:bg-gray-700/50 text-[10px] font-medium text-gray-600 dark:text-slate-300 flex items-center gap-1.5">
                    <span class="w-1.5 h-1.5 rounded-full bg-violet-400 flex-shrink-0" />
                    Tool Definitions ({{ parsedTools.length }})
                    <span v-if="toolsCached" class="ml-auto inline-flex items-center px-1.5 py-0.5 rounded-full text-[9px] font-bold bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300 ring-1 ring-amber-200 dark:ring-amber-800/50">
                      KV Cached
                    </span>
                  </div>
                  <div class="p-2 space-y-1">
                    <div
                      v-for="(tool, idx) in parsedTools"
                      :key="tool.name"
                      class="text-[10px] font-mono"
                    >
                      <div class="flex items-center gap-1.5">
                        <span class="text-violet-600 dark:text-violet-300 font-medium">{{ tool.name }}</span>
                        <button
                          v-if="tool.input_schema"
                          class="text-[9px] text-gray-400 dark:text-slate-500 hover:text-gray-600 dark:hover:text-slate-300 transition-colors"
                          @click="toggleToolSchema(idx)"
                        >
                          {{ expandedToolSchemas.has(idx) ? '[-]' : '[+]' }}
                        </button>
                      </div>
                      <span v-if="tool.description" class="text-gray-500 dark:text-slate-400 ml-1.5">{{ tool.description }}</span>
                      <pre v-if="tool.input_schema && expandedToolSchemas.has(idx)" class="mt-0.5 text-[9px] text-gray-400 dark:text-slate-500 whitespace-pre-wrap break-all max-h-60 overflow-auto">{{ JSON.stringify(tool.input_schema, null, 2) }}</pre>
                    </div>
                  </div>
                </div>

                <!-- Messages in order: system, user, assistant (with tool_calls), tool (tool_result) -->
                <MessageBubble
                  v-for="(msg, i) in parsedMessages"
                  :key="i"
                  :role="msg.role"
                  :content="msg.content"
                  :tool-calls="msg.tool_calls"
                  :tool-call-id="msg.tool_call_id"
                  :tool-name="msg.role === 'tool' ? (toolNameMap.get(msg.name || '') || msg.name || 'tool') : undefined"
                  :cached="i < cachedPrefixLength"
                />
              </div>
            </div>

            <!-- LLM Request (raw JSON) -->
            <div v-if="metrics.llm_request" class="space-y-1.5">
              <button
                class="flex items-center gap-1.5 text-xs font-medium text-gray-700 dark:text-slate-300 hover:text-gray-900 dark:hover:text-white transition-colors"
                @click="llmRequestExpanded = !llmRequestExpanded"
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-3.5 w-3.5 transition-transform"
                  :class="{ 'rotate-90': llmRequestExpanded }"
                  fill="none" viewBox="0 0 24 24" stroke="currentColor"
                >
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
                </svg>
                LLM Request
                <span class="text-[10px] text-gray-400 dark:text-slate-500 font-normal">JSON</span>
              </button>
              <JsonViewer v-if="llmRequestExpanded" :data="metrics.llm_request" />
            </div>

            <!-- LLM Response -->
            <div v-if="metrics.llm_response" class="space-y-1.5">
              <button
                class="flex items-center gap-1.5 text-xs font-medium text-gray-700 dark:text-slate-300 hover:text-gray-900 dark:hover:text-white transition-colors"
                @click="llmResponseExpanded = !llmResponseExpanded"
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-3.5 w-3.5 transition-transform"
                  :class="{ 'rotate-90': llmResponseExpanded }"
                  fill="none" viewBox="0 0 24 24" stroke="currentColor"
                >
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
                </svg>
                LLM Response
                <span class="text-[10px] text-gray-400 dark:text-slate-500 font-normal">JSON</span>
              </button>
              <JsonViewer v-if="llmResponseExpanded" :data="metrics.llm_response" />
            </div>

            <!-- Actions -->
            <div class="flex items-center gap-2 pt-2 border-t border-gray-100 dark:border-gray-700/50">
              <button
                class="px-3 py-1.5 text-xs rounded-lg border border-gray-200 dark:border-gray-600 text-gray-700 dark:text-slate-200 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
                @click="copyForReplay"
              >
                Copy for Replay
              </button>
              <button
                class="px-3 py-1.5 text-xs rounded-lg border border-gray-200 dark:border-gray-600 text-gray-700 dark:text-slate-200 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
                @click="handleFork"
              >
                Fork from here
              </button>
              <button
                class="px-3 py-1.5 text-xs rounded-lg border border-gray-200 dark:border-gray-600 text-amber-700 dark:text-amber-400 hover:bg-amber-50 dark:hover:bg-amber-900/20 transition-colors"
                @click="handleRewind"
              >
                Rewind to here
              </button>
            </div>
          </template>
        </div>
      </div>
    </div>
  </Transition>
</template>

<style scoped>
.turn-detail-panel-layer {
  position: fixed;
  inset: 0;
  z-index: 70;
  display: flex;
  align-items: stretch;
  justify-content: flex-end;
  padding: 0;
  background: transparent;
}

.turn-detail-panel-shell {
  width: min(32rem, calc(100vw - 0.75rem));
  max-width: 100%;
  height: 100%;
  margin-inline-start: auto;
  border-start-start-radius: 1.5rem;
  border-end-start-radius: 1.5rem;
  border-start-end-radius: 0;
  border-end-end-radius: 0;
}

.turn-detail-panel-enter-active,
.turn-detail-panel-leave-active {
  transition: opacity 0.22s ease;
}

.turn-detail-panel-enter-active .turn-detail-panel-shell,
.turn-detail-panel-leave-active .turn-detail-panel-shell {
  transition:
    transform 0.22s ease,
    opacity 0.22s ease;
}

.turn-detail-panel-enter-from,
.turn-detail-panel-leave-to {
  opacity: 0;
}

.turn-detail-panel-enter-from .turn-detail-panel-shell,
.turn-detail-panel-leave-to .turn-detail-panel-shell {
  opacity: 0;
  transform: translateX(24px);
}

html[dir='rtl'] .turn-detail-panel-enter-from .turn-detail-panel-shell,
html[dir='rtl'] .turn-detail-panel-leave-to .turn-detail-panel-shell {
  transform: translateX(-24px);
}

@media (min-width: 1024px) {
  .turn-detail-panel-layer {
    inset-block: 0.9rem;
    inset-inline-end: 0.9rem;
    inset-inline-start: auto;
    width: 32rem;
    padding: 0;
    align-items: stretch;
    justify-content: flex-end;
    background: transparent;
    backdrop-filter: none;
    -webkit-backdrop-filter: none;
  }

  .turn-detail-panel-shell {
    width: 100%;
    height: 100%;
    border-radius: 2rem;
    box-shadow: 0 30px 52px -30px rgba(15, 23, 42, 0.55);
  }
}
</style>
