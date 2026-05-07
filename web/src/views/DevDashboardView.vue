<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { devApi } from '@/api/dev'
import type { SessionInfo, TurnMetrics, AggregateStats, ModelBreakdown, ToolStats, ToolCallSummary } from '@/api/dev'

const loading = ref(false)
const stats = ref<AggregateStats | null>(null)
const sessions = ref<SessionInfo[]>([])
const models = ref<ModelBreakdown[]>([])
const tools = ref<ToolStats[]>([])
const selectedSession = ref<string | null>(null)
const turns = ref<TurnMetrics[]>([])
const selectedTurn = ref<TurnMetrics | null>(null)
const showReplay = ref(false)
const replayData = ref<any>(null)

onMounted(async () => {
  await loadDashboard()
})

async function loadDashboard() {
  loading.value = true
  try {
    const [statsRes, sessionsRes, modelsRes, toolsRes] = await Promise.all([
      devApi.getAggregateStats(),
      devApi.getSessions(50),
      devApi.getModelBreakdown(),
      devApi.getToolStats(),
    ])
    stats.value = statsRes.data
    sessions.value = sessionsRes.data
    models.value = modelsRes.data
    tools.value = toolsRes.data
  } catch (e) {
    console.error('Failed to load dev dashboard', e)
  } finally {
    loading.value = false
  }
}

async function selectSession(convId: string) {
  selectedSession.value = convId
  selectedTurn.value = null
  showReplay.value = false
  try {
    const res = await devApi.getSessionTurns(convId)
    turns.value = res.data
  } catch {
    turns.value = []
  }
}

async function selectTurn(turn: TurnMetrics) {
  selectedTurn.value = turn
  showReplay.value = false
}

async function loadReplay(turnId: string) {
  try {
    const res = await devApi.getTurnReplay(turnId)
    replayData.value = res.data
    showReplay.value = true
  } catch {
    replayData.value = null
  }
}

function formatCost(cost: number): string {
  return '$' + cost.toFixed(4)
}

function formatMs(ms: number): string {
  if (ms < 1000) return ms.toFixed(0) + 'ms'
  return (ms / 1000).toFixed(2) + 's'
}

function formatTokens(n: number): string {
  if (n >= 1000000) return (n / 1000000).toFixed(1) + 'M'
  if (n >= 1000) return (n / 1000).toFixed(1) + 'K'
  return String(n)
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleString()
}

function statusColor(status: string): string {
  if (status === 'success') return 'text-green-600 dark:text-green-400'
  if (status === 'error') return 'text-red-600 dark:text-red-400'
  return 'text-yellow-600 dark:text-yellow-400'
}

function truncate(s: string, len: number): string {
  if (!s) return ''
  return s.length > len ? s.slice(0, len) + '...' : s
}
</script>

<template>
  <div class="dev-dashboard p-4 sm:p-6 max-w-7xl mx-auto">
    <div class="flex items-center justify-between mb-6">
      <h1 class="text-xl sm:text-2xl font-bold text-gray-900 dark:text-white">
        Dev Dashboard
      </h1>
      <button
        class="px-3 py-2 bg-blue-500 hover:bg-blue-600 text-white rounded-lg text-sm transition-colors"
        @click="loadDashboard"
      >
        Refresh
      </button>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="text-center py-12 text-gray-500 dark:text-slate-400">
      Loading...
    </div>

    <!-- Aggregate Stats -->
    <div v-if="stats" class="grid grid-cols-2 sm:grid-cols-5 gap-4 mb-6">
      <div class="glass-card p-4">
        <div class="text-2xl font-bold text-blue-600 dark:text-blue-400">{{ stats.total_turns }}</div>
        <div class="text-sm text-gray-500 dark:text-slate-400">Turns</div>
      </div>
      <div class="glass-card p-4">
        <div class="text-2xl font-bold text-green-600 dark:text-green-400">{{ formatCost(stats.total_cost) }}</div>
        <div class="text-sm text-gray-500 dark:text-slate-400">Total Cost</div>
      </div>
      <div class="glass-card p-4">
        <div class="text-2xl font-bold text-purple-600 dark:text-purple-400">{{ formatTokens(stats.total_tokens) }}</div>
        <div class="text-sm text-gray-500 dark:text-slate-400">Tokens</div>
      </div>
      <div class="glass-card p-4">
        <div class="text-2xl font-bold text-orange-600 dark:text-orange-400">{{ formatMs(stats.avg_latency_ms) }}</div>
        <div class="text-sm text-gray-500 dark:text-slate-400">Avg Latency</div>
      </div>
      <div class="glass-card p-4">
        <div class="text-2xl font-bold" :class="stats.error_rate > 0.1 ? 'text-red-600 dark:text-red-400' : 'text-green-600 dark:text-green-400'">
          {{ (stats.error_rate * 100).toFixed(1) }}%
        </div>
        <div class="text-sm text-gray-500 dark:text-slate-400">Error Rate</div>
      </div>
    </div>

    <!-- Two-column layout: Sessions | Turns -->
    <div class="grid grid-cols-1 lg:grid-cols-3 gap-6 mb-6">
      <!-- Session List -->
      <div class="glass-card p-4 lg:col-span-1 max-h-[600px] overflow-y-auto">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white mb-3">Sessions</h2>
        <div
          v-for="s in sessions"
          :key="s.conversation_id"
          class="p-3 mb-2 rounded-lg cursor-pointer transition-colors"
          :class="selectedSession === s.conversation_id
            ? 'bg-blue-100 dark:bg-blue-900/50 border border-blue-300 dark:border-blue-700'
            : 'bg-gray-50 dark:bg-slate-800 hover:bg-gray-100 dark:hover:bg-slate-700'"
          @click="selectSession(s.conversation_id)"
        >
          <div class="flex justify-between items-center">
            <span class="font-mono text-xs text-gray-600 dark:text-slate-400 truncate">
              {{ s.conversation_id.slice(0, 8) }}...
            </span>
            <span class="text-sm font-medium text-green-600 dark:text-green-400">
              {{ formatCost(s.total_cost) }}
            </span>
          </div>
          <div class="flex justify-between items-center mt-1">
            <span class="text-xs text-gray-500 dark:text-slate-500">
              {{ s.turn_count }} turns &middot; {{ formatTokens(s.total_tokens) }} tok
            </span>
            <span class="text-xs text-gray-400 dark:text-slate-500">
              {{ s.model ? s.model.split('/').pop()?.split('-').slice(0, 2).join('-') : '' }}
            </span>
          </div>
        </div>
        <div v-if="sessions.length === 0" class="text-center py-8 text-gray-400 dark:text-slate-500 text-sm">
          No sessions recorded yet. Enable BLUE_DEV_MODE=true and send some chat messages.
        </div>
      </div>

      <!-- Turn Timeline -->
      <div class="glass-card p-4 lg:col-span-2 max-h-[600px] overflow-y-auto">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white mb-3">
          Turn Timeline
          <span v-if="selectedSession" class="text-sm font-normal text-gray-500 dark:text-slate-400">
            for {{ selectedSession.slice(0, 8) }}...
          </span>
        </h2>
        <div v-if="!selectedSession" class="text-center py-8 text-gray-400 dark:text-slate-500 text-sm">
          Select a session to view turns
        </div>
        <div v-for="t in turns" :key="t.turn_id" class="mb-3">
          <div
            class="p-3 rounded-lg cursor-pointer transition-colors border"
            :class="selectedTurn?.turn_id === t.turn_id
              ? 'bg-blue-50 dark:bg-blue-900/30 border-blue-300 dark:border-blue-700'
              : 'bg-gray-50 dark:bg-slate-800 border-transparent hover:border-gray-200 dark:hover:border-slate-600'"
            @click="selectTurn(t)"
          >
            <div class="flex justify-between items-center">
              <div class="flex items-center gap-2">
                <span class="text-xs font-mono text-gray-400 dark:text-slate-500">
                  {{ formatDate(t.created_at) }}
                </span>
                <span class="text-xs px-2 py-0.5 rounded-full" :class="statusColor(t.status)">
                  {{ t.status }}
                </span>
              </div>
              <div class="flex items-center gap-3 text-xs text-gray-500 dark:text-slate-400">
                <span>{{ t.model ? t.model.split('/').pop()?.split('-').slice(0, 2).join('-') : '' }}</span>
                <span>{{ formatMs(t.latency_ms) }}</span>
                <span>{{ formatCost(t.cost_usd) }}</span>
              </div>
            </div>
            <div class="flex items-center gap-4 mt-1 text-xs text-gray-400 dark:text-slate-500">
              <span>{{ formatTokens(t.input_tokens) }} in / {{ formatTokens(t.output_tokens) }} out</span>
              <span v-if="t.tool_count > 0">{{ t.tool_count }} tool{{ t.tool_count > 1 ? 's' : '' }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Turn Detail Panel -->
    <div v-if="selectedTurn" class="glass-card p-4 mb-6">
      <div class="flex justify-between items-center mb-4">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">Turn Detail</h2>
        <button
          class="px-3 py-1 bg-purple-500 hover:bg-purple-600 text-white rounded-lg text-sm transition-colors"
          @click="loadReplay(selectedTurn.turn_id)"
        >
          View LLM Request/Response
        </button>
      </div>

      <!-- Tool Calls -->
      <div v-if="selectedTurn.tool_calls && selectedTurn.tool_calls.length > 0" class="mb-4">
        <h3 class="text-sm font-medium text-gray-700 dark:text-slate-300 mb-2">Tool Calls</h3>
        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-2">
          <div
            v-for="tc in selectedTurn.tool_calls"
            :key="tc.tool_call_id"
            class="p-2 rounded-lg text-xs"
            :class="tc.success
              ? 'bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800'
              : 'bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800'"
          >
            <div class="font-medium text-gray-900 dark:text-white">{{ tc.name }}</div>
            <div class="text-gray-500 dark:text-slate-400">{{ formatMs(tc.latency_ms) }}</div>
          </div>
        </div>
      </div>

      <!-- Request/Response Replay -->
      <div v-if="showReplay && replayData" class="mt-4">
        <h3 class="text-sm font-medium text-gray-700 dark:text-slate-300 mb-2">LLM Request</h3>
        <pre class="p-3 bg-gray-50 dark:bg-slate-800 rounded-lg text-xs overflow-x-auto max-h-64 overflow-y-auto text-gray-800 dark:text-slate-200">{{ JSON.stringify(replayData.llm_request, null, 2) }}</pre>

        <h3 class="text-sm font-medium text-gray-700 dark:text-slate-300 mt-4 mb-2">LLM Response</h3>
        <pre class="p-3 bg-gray-50 dark:bg-slate-800 rounded-lg text-xs overflow-x-auto max-h-64 overflow-y-auto text-gray-800 dark:text-slate-200">{{ truncate(JSON.stringify(replayData.llm_response, null, 2), 2000) }}</pre>
      </div>
    </div>

    <!-- Model & Tool Stats -->
    <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
      <!-- Model Breakdown -->
      <div class="glass-card p-4">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white mb-3">Models</h2>
        <table class="w-full text-sm">
          <thead>
            <tr class="text-left text-gray-500 dark:text-slate-400 border-b dark:border-slate-700">
              <th class="pb-2">Model</th>
              <th class="pb-2 text-right">Turns</th>
              <th class="pb-2 text-right">Cost</th>
              <th class="pb-2 text-right">Avg Latency</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="m in models" :key="m.model" class="border-b dark:border-slate-800">
              <td class="py-2 text-gray-900 dark:text-white font-mono text-xs">{{ m.model }}</td>
              <td class="py-2 text-right text-gray-600 dark:text-slate-400">{{ m.turn_count }}</td>
              <td class="py-2 text-right text-green-600 dark:text-green-400">{{ formatCost(m.total_cost) }}</td>
              <td class="py-2 text-right text-gray-600 dark:text-slate-400">{{ formatMs(m.avg_latency_ms) }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Tool Stats -->
      <div class="glass-card p-4">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white mb-3">Tools</h2>
        <table class="w-full text-sm">
          <thead>
            <tr class="text-left text-gray-500 dark:text-slate-400 border-b dark:border-slate-700">
              <th class="pb-2">Tool</th>
              <th class="pb-2 text-right">Calls</th>
              <th class="pb-2 text-right">Avg Latency</th>
              <th class="pb-2 text-right">Success</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="t in tools" :key="t.tool_name" class="border-b dark:border-slate-800">
              <td class="py-2 text-gray-900 dark:text-white font-mono text-xs">{{ t.tool_name }}</td>
              <td class="py-2 text-right text-gray-600 dark:text-slate-400">{{ t.call_count }}</td>
              <td class="py-2 text-right text-gray-600 dark:text-slate-400">{{ formatMs(t.avg_latency_ms) }}</td>
              <td class="py-2 text-right">
                <span class="text-green-600 dark:text-green-400">{{ t.successes }}</span>
                <span class="text-gray-400 dark:text-slate-500"> / </span>
                <span class="text-red-600 dark:text-red-400">{{ t.failures }}</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>
