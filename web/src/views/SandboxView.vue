<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { sandboxApi, type SandboxInfo, type ExecutionResult, type ExecuteRequest } from '@/api/sandbox'

const { t } = useI18n()

// State
const sandboxInfo = ref<SandboxInfo | null>(null)
const executions = ref<ExecutionResult[]>([])
const loading = ref(true)
const executing = ref(false)
const error = ref<string | null>(null)

// Form state
const command = ref('')
const args = ref('')
const workDir = ref('')
const stdin = ref('')
const timeoutSecs = ref(30)
const memoryMb = ref(256)

// Current execution result
const currentResult = ref<ExecutionResult | null>(null)

// Computed
const isSupported = computed(() => sandboxInfo.value?.supported ?? false)

// Format bytes to human readable
function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

// Format duration
function formatDuration(ns: number): string {
  if (ns < 1000) return `${ns}ns`
  if (ns < 1000000) return `${(ns / 1000).toFixed(2)}µs`
  if (ns < 1000000000) return `${(ns / 1000000).toFixed(2)}ms`
  return `${(ns / 1000000000).toFixed(2)}s`
}

// Get status color
function getStatusColor(status: string): string {
  switch (status) {
    case 'completed':
      return 'text-green-500 bg-green-500/10'
    case 'running':
      return 'text-gray-900 dark:text-white bg-gray-700 dark:bg-gray-700/10'
    case 'pending':
      return 'text-yellow-500 bg-yellow-500/10'
    case 'failed':
    case 'timeout':
    case 'killed':
      return 'text-red-500 bg-red-500/10'
    default:
      return 'text-gray-500 bg-gray-500/10'
  }
}

// Fetch sandbox info
async function fetchSandboxInfo() {
  try {
    const response = await sandboxApi.getInfo()
    console.log('Sandbox info:', response.data)
    sandboxInfo.value = response.data
  } catch (e) {
    console.error('Failed to fetch sandbox info:', e)
    error.value = t('sandbox.errors.fetchInfo')
    // Set a default info object so the page can still render
    sandboxInfo.value = {
      supported: false,
      default_timeout: '-',
      max_timeout: '-',
      memory_limit: 0,
      cpu_limit: 0,
      process_limit: 0,
      network_enabled: false,
    }
  }
}

// Execute command
async function executeCommand() {
  if (!command.value.trim()) {
    error.value = t('sandbox.errors.commandRequired')
    return
  }

  executing.value = true
  error.value = null
  currentResult.value = null

  try {
    const request: ExecuteRequest = {
      command: command.value.trim(),
      timeout_secs: timeoutSecs.value,
      memory_mb: memoryMb.value,
    }

    if (args.value.trim()) {
      request.args = args.value.split(/\s+/).filter(Boolean)
    }
    if (workDir.value.trim()) {
      request.work_dir = workDir.value.trim()
    }
    if (stdin.value.trim()) {
      request.stdin = stdin.value
    }

    const response = await sandboxApi.execute(request)
    currentResult.value = response.data
    executions.value.unshift(response.data)
  } catch (e) {
    console.error('Failed to execute command:', e)
    error.value = e instanceof Error ? e.message : t('sandbox.errors.executeFailed')
  } finally {
    executing.value = false
  }
}

// Kill execution
async function killExecution(id: string) {
  try {
    await sandboxApi.kill(id)
    // Refresh status
    const response = await sandboxApi.getStatus(id)
    const index = executions.value.findIndex(e => e.id === id)
    if (index !== -1) {
      executions.value[index] = response.data
    }
    if (currentResult.value?.id === id) {
      currentResult.value = response.data
    }
  } catch (e) {
    console.error('Failed to kill execution:', e)
    error.value = t('sandbox.errors.killFailed')
  }
}

// Clear form
function clearForm() {
  command.value = ''
  args.value = ''
  workDir.value = ''
  stdin.value = ''
  timeoutSecs.value = 30
  memoryMb.value = 256
  currentResult.value = null
  error.value = null
}

// Initialize
onMounted(async () => {
  loading.value = true
  await fetchSandboxInfo()
  loading.value = false
})
</script>

<template>
  <div class="p-6 max-w-7xl mx-auto">
    <!-- Header -->
    <div class="mb-8">
      <h1 class="text-2xl font-bold text-gray-900 dark:text-white">
        {{ t('sandbox.title') }}
      </h1>
      <p class="mt-1 text-gray-500 dark:text-slate-400">
        {{ t('sandbox.subtitle') }}
      </p>
    </div>

    <!-- Loading State -->
    <div v-if="loading" class="flex items-center justify-center py-12">
      <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900 dark:border-gray-700"></div>
    </div>

    <!-- Not Supported Warning -->
    <div v-else-if="!isSupported" class="glass-card p-8 text-center">
      <div class="w-16 h-16 mx-auto mb-4 rounded-full bg-yellow-500/10 flex items-center justify-center">
        <svg xmlns="http://www.w3.org/2000/svg" class="h-8 w-8 text-yellow-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
        </svg>
      </div>
      <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-2">
        {{ t('sandbox.notSupported') }}
      </h3>
      <p class="text-gray-500 dark:text-slate-400">
        {{ t('sandbox.notSupportedDesc') }}
      </p>
    </div>

    <!-- Main Content -->
    <div v-else class="grid grid-cols-1 lg:grid-cols-3 gap-6">
      <!-- Left Column: Execution Form -->
      <div class="lg:col-span-2 space-y-6">
        <!-- Command Input Card -->
        <div class="glass-card p-6">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">
            {{ t('sandbox.executeCommand') }}
          </h2>

          <!-- Error Alert -->
          <div v-if="error" class="mb-4 p-4 rounded-lg bg-red-500/10 border border-red-500/20">
            <div class="flex items-center space-x-2">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-red-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              <span class="text-red-500">{{ error }}</span>
            </div>
          </div>

          <div class="space-y-4">
            <!-- Command -->
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1">
                {{ t('sandbox.form.command') }} *
              </label>
              <input
                v-model="command"
                type="text"
                class="w-full glass-input px-4 py-2 text-gray-900 dark:text-white"
                :placeholder="t('sandbox.form.commandPlaceholder')"
                :disabled="executing"
              />
            </div>

            <!-- Arguments -->
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1">
                {{ t('sandbox.form.arguments') }}
              </label>
              <input
                v-model="args"
                type="text"
                class="w-full glass-input px-4 py-2 text-gray-900 dark:text-white"
                :placeholder="t('sandbox.form.argumentsPlaceholder')"
                :disabled="executing"
              />
            </div>

            <!-- Working Directory -->
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1">
                {{ t('sandbox.form.workDir') }}
              </label>
              <input
                v-model="workDir"
                type="text"
                class="w-full glass-input px-4 py-2 text-gray-900 dark:text-white"
                :placeholder="t('sandbox.form.workDirPlaceholder')"
                :disabled="executing"
              />
            </div>

            <!-- Stdin -->
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1">
                {{ t('sandbox.form.stdin') }}
              </label>
              <textarea
                v-model="stdin"
                rows="3"
                class="w-full glass-input px-4 py-2 text-gray-900 dark:text-white resize-none"
                :placeholder="t('sandbox.form.stdinPlaceholder')"
                :disabled="executing"
              ></textarea>
            </div>

            <!-- Limits -->
            <div class="grid grid-cols-2 gap-4">
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1">
                  {{ t('sandbox.form.timeout') }}
                </label>
                <div class="flex items-center space-x-2">
                  <input
                    v-model.number="timeoutSecs"
                    type="number"
                    min="1"
                    max="300"
                    class="flex-1 glass-input px-4 py-2 text-gray-900 dark:text-white"
                    :disabled="executing"
                  />
                  <span class="text-gray-500 dark:text-slate-400">{{ t('sandbox.form.seconds') }}</span>
                </div>
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1">
                  {{ t('sandbox.form.memoryLimit') }}
                </label>
                <div class="flex items-center space-x-2">
                  <input
                    v-model.number="memoryMb"
                    type="number"
                    min="16"
                    max="1024"
                    class="flex-1 glass-input px-4 py-2 text-gray-900 dark:text-white"
                    :disabled="executing"
                  />
                  <span class="text-gray-500 dark:text-slate-400">MB</span>
                </div>
              </div>
            </div>

            <!-- Actions -->
            <div class="flex items-center space-x-3 pt-2">
              <button
                class="px-6 py-2 rounded-lg bg-gray-700 dark:bg-gray-700 text-white hover:bg-gray-700 dark:bg-gray-700-light transition-colors flex items-center space-x-2 disabled:opacity-50 disabled:cursor-not-allowed"
                :disabled="executing || !command.trim()"
                @click="executeCommand"
              >
                <svg v-if="executing" class="animate-spin h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                  <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
                <svg v-else xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                <span>{{ executing ? t('sandbox.executing') : t('sandbox.execute') }}</span>
              </button>
              <button
                class="px-4 py-2 rounded-lg border border-gray-300 dark:border-glass-border text-gray-700 dark:text-slate-300 hover:bg-gray-100 dark:hover:bg-white/5 transition-colors"
                :disabled="executing"
                @click="clearForm"
              >
                {{ t('common.clear') }}
              </button>
            </div>
          </div>
        </div>

        <!-- Execution Result Card -->
        <div v-if="currentResult" class="glass-card p-6">
          <div class="flex items-center justify-between mb-4">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('sandbox.result') }}
            </h2>
            <div class="flex items-center space-x-2">
              <span
                class="px-2 py-1 rounded-full text-xs font-medium"
                :class="getStatusColor(currentResult.status)"
              >
                {{ currentResult.status }}
              </span>
              <button
                v-if="currentResult.status === 'running'"
                class="p-1 rounded text-red-500 hover:bg-red-500/10 transition-colors"
                :title="t('sandbox.kill')"
                @click="killExecution(currentResult.id)"
              >
                <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
          </div>

          <!-- Exit Code & Duration -->
          <div class="grid grid-cols-2 gap-4 mb-4">
            <div class="p-3 rounded-lg bg-gray-100 dark:bg-white/5">
              <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('sandbox.exitCode') }}</div>
              <div class="text-lg font-semibold" :class="currentResult.exit_code === 0 ? 'text-green-500' : 'text-red-500'">
                {{ currentResult.exit_code }}
              </div>
            </div>
            <div class="p-3 rounded-lg bg-gray-100 dark:bg-white/5">
              <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('sandbox.duration') }}</div>
              <div class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ currentResult.duration ? `${currentResult.duration.toFixed(2)}s` : '-' }}
              </div>
            </div>
          </div>

          <!-- Resource Usage -->
          <div v-if="currentResult.resource_usage" class="grid grid-cols-2 gap-4 mb-4">
            <div class="p-3 rounded-lg bg-gray-100 dark:bg-white/5">
              <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('sandbox.cpuTime') }}</div>
              <div class="text-sm font-medium text-gray-900 dark:text-white">
                {{ formatDuration(currentResult.resource_usage.cpu_time_ns) }}
              </div>
            </div>
            <div class="p-3 rounded-lg bg-gray-100 dark:bg-white/5">
              <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('sandbox.memoryPeak') }}</div>
              <div class="text-sm font-medium text-gray-900 dark:text-white">
                {{ formatBytes(currentResult.resource_usage.memory_peak_bytes) }}
              </div>
            </div>
          </div>

          <!-- Stdout -->
          <div v-if="currentResult.stdout" class="mb-4">
            <div class="text-sm font-medium text-gray-700 dark:text-slate-300 mb-2">{{ t('sandbox.stdout') }}</div>
            <pre class="p-4 rounded-lg bg-gray-200 text-green-400 text-sm overflow-x-auto max-h-64 overflow-y-auto font-mono">{{ currentResult.stdout }}</pre>
          </div>

          <!-- Stderr -->
          <div v-if="currentResult.stderr" class="mb-4">
            <div class="text-sm font-medium text-gray-700 dark:text-slate-300 mb-2">{{ t('sandbox.stderr') }}</div>
            <pre class="p-4 rounded-lg bg-gray-200 text-red-400 text-sm overflow-x-auto max-h-64 overflow-y-auto font-mono">{{ currentResult.stderr }}</pre>
          </div>

          <!-- Error -->
          <div v-if="currentResult.error" class="p-4 rounded-lg bg-red-500/10 border border-red-500/20">
            <div class="text-sm font-medium text-red-500">{{ t('sandbox.error') }}: {{ currentResult.error }}</div>
          </div>
        </div>
      </div>

      <!-- Right Column: Info & History -->
      <div class="space-y-6">
        <!-- Sandbox Info Card -->
        <div class="glass-card p-6">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">
            {{ t('sandbox.configuration') }}
          </h2>
          <div class="space-y-3">
            <div class="flex justify-between items-center">
              <span class="text-gray-500 dark:text-slate-400">{{ t('sandbox.config.status') }}</span>
              <span class="px-2 py-1 rounded-full text-xs font-medium" :class="isSupported ? 'text-green-500 bg-green-500/10' : 'text-red-500 bg-red-500/10'">
                {{ isSupported ? t('common.enabled') : t('common.disabled') }}
              </span>
            </div>
            <div class="flex justify-between items-center">
              <span class="text-gray-500 dark:text-slate-400">{{ t('sandbox.config.defaultTimeout') }}</span>
              <span class="text-gray-900 dark:text-white">{{ sandboxInfo?.default_timeout || '-' }}</span>
            </div>
            <div class="flex justify-between items-center">
              <span class="text-gray-500 dark:text-slate-400">{{ t('sandbox.config.maxTimeout') }}</span>
              <span class="text-gray-900 dark:text-white">{{ sandboxInfo?.max_timeout || '-' }}</span>
            </div>
            <div class="flex justify-between items-center">
              <span class="text-gray-500 dark:text-slate-400">{{ t('sandbox.config.memoryLimit') }}</span>
              <span class="text-gray-900 dark:text-white">{{ sandboxInfo?.memory_limit ? formatBytes(sandboxInfo.memory_limit) : '-' }}</span>
            </div>
            <div class="flex justify-between items-center">
              <span class="text-gray-500 dark:text-slate-400">{{ t('sandbox.config.cpuLimit') }}</span>
              <span class="text-gray-900 dark:text-white">{{ sandboxInfo?.cpu_limit ? `${sandboxInfo.cpu_limit} core(s)` : '-' }}</span>
            </div>
            <div class="flex justify-between items-center">
              <span class="text-gray-500 dark:text-slate-400">{{ t('sandbox.config.processLimit') }}</span>
              <span class="text-gray-900 dark:text-white">{{ sandboxInfo?.process_limit || '-' }}</span>
            </div>
            <div class="flex justify-between items-center">
              <span class="text-gray-500 dark:text-slate-400">{{ t('sandbox.config.network') }}</span>
              <span class="px-2 py-1 rounded-full text-xs font-medium" :class="sandboxInfo?.network_enabled ? 'text-green-500 bg-green-500/10' : 'text-red-500 bg-red-500/10'">
                {{ sandboxInfo?.network_enabled ? t('common.enabled') : t('common.disabled') }}
              </span>
            </div>
          </div>
        </div>

        <!-- Execution History Card -->
        <div class="glass-card p-6">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">
            {{ t('sandbox.history') }}
          </h2>
          <div v-if="executions.length === 0" class="text-center py-8 text-gray-500 dark:text-slate-400">
            {{ t('sandbox.noHistory') }}
          </div>
          <div v-else class="space-y-2 max-h-96 overflow-y-auto">
            <div
              v-for="exec in executions"
              :key="exec.id"
              class="p-3 rounded-lg bg-gray-100 dark:bg-white/5 hover:bg-gray-200 dark:hover:bg-white/10 cursor-pointer transition-colors"
              @click="currentResult = exec"
            >
              <div class="flex items-center justify-between">
                <span class="text-sm font-mono text-gray-900 dark:text-white truncate max-w-[150px]">
                  {{ exec.id.slice(0, 8) }}...
                </span>
                <span
                  class="px-2 py-0.5 rounded-full text-xs font-medium"
                  :class="getStatusColor(exec.status)"
                >
                  {{ exec.status }}
                </span>
              </div>
              <div class="text-xs text-gray-500 dark:text-slate-400 mt-1">
                {{ exec.start_time ? new Date(exec.start_time).toLocaleString() : '-' }}
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
