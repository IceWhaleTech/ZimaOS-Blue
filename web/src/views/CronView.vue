<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { cronApi } from '@/api/cron'
import type { CronJob, JobExecution } from '@/api/cron'

const { t } = useI18n()

const loading = ref(false)
const jobs = ref<CronJob[]>([])
const selectedJob = ref<CronJob | null>(null)
const executions = ref<JobExecution[]>([])
const showCreateModal = ref(false)
const showEditModal = ref(false)
const showExecutionsModal = ref(false)

// Form
const jobForm = ref({
  name: '',
  description: '',
  schedule: '',
  handler: 'command',
  payload: '{}',
  // Command handler fields
  command: '',
  workdir: '',
  timeout: 60,
  // HTTP handler fields
  url: '',
  method: 'GET',
})

// Available handler types
const handlerTypes = computed(() => [
  { value: 'command', label: t('cron.handlers.command'), description: t('cron.handlers.commandDesc') },
  { value: 'http', label: t('cron.handlers.http'), description: t('cron.handlers.httpDesc') },
])

// Common cron presets
const cronPresets = computed(() => [
  { label: t('cron.presets.everyMinute'), value: '* * * * *' },
  { label: t('cron.presets.every5Minutes'), value: '*/5 * * * *' },
  { label: t('cron.presets.everyHour'), value: '0 * * * *' },
  { label: t('cron.presets.everyDayMidnight'), value: '0 0 * * *' },
  { label: t('cron.presets.everyMondayMorning'), value: '0 9 * * 1' },
  { label: t('cron.presets.everyMonth'), value: '0 0 1 * *' },
])

onMounted(async () => {
  await loadJobs()
})

async function loadJobs() {
  try {
    loading.value = true
    const response = await cronApi.list()
    jobs.value = response.data
  } catch {
    jobs.value = []
  } finally {
    loading.value = false
  }
}

function openCreateModal() {
  jobForm.value = {
    name: '',
    description: '',
    schedule: '',
    handler: 'command',
    payload: '{}',
    command: '',
    workdir: '',
    timeout: 60,
    url: '',
    method: 'GET',
  }
  showCreateModal.value = true
}

function openEditModal(job: CronJob) {
  selectedJob.value = job
  const payload = job.payload || {}
  jobForm.value = {
    name: job.name,
    description: job.description || '',
    schedule: job.schedule,
    handler: job.handler,
    payload: job.payload ? JSON.stringify(job.payload, null, 2) : '{}',
    // Extract command handler fields
    command: (payload.command as string) || '',
    workdir: (payload.workdir as string) || '',
    timeout: (payload.timeout as number) || 60,
    // Extract HTTP handler fields
    url: (payload.url as string) || '',
    method: (payload.method as string) || 'GET',
  }
  showEditModal.value = true
}

// Build payload based on handler type
function buildPayload(): Record<string, unknown> {
  if (jobForm.value.handler === 'command') {
    const payload: Record<string, unknown> = {
      command: jobForm.value.command,
    }
    if (jobForm.value.workdir) {
      payload.workdir = jobForm.value.workdir
    }
    if (jobForm.value.timeout && jobForm.value.timeout !== 60) {
      payload.timeout = jobForm.value.timeout
    }
    return payload
  } else if (jobForm.value.handler === 'http') {
    const payload: Record<string, unknown> = {
      url: jobForm.value.url,
      method: jobForm.value.method,
    }
    if (jobForm.value.timeout && jobForm.value.timeout !== 30) {
      payload.timeout = jobForm.value.timeout
    }
    return payload
  }
  // Fallback to raw JSON payload
  try {
    return JSON.parse(jobForm.value.payload)
  } catch {
    return {}
  }
}

async function createJob() {
  if (!jobForm.value.name || !jobForm.value.schedule || !jobForm.value.handler) return

  // Validate handler-specific fields
  if (jobForm.value.handler === 'command' && !jobForm.value.command) return
  if (jobForm.value.handler === 'http' && !jobForm.value.url) return

  try {
    loading.value = true
    const payload = buildPayload()

    await cronApi.create({
      name: jobForm.value.name,
      description: jobForm.value.description,
      schedule: jobForm.value.schedule,
      handler: jobForm.value.handler,
      payload,
    })
    showCreateModal.value = false
    await loadJobs()
  } catch {
    // Handle error
  } finally {
    loading.value = false
  }
}

async function updateJob() {
  if (!selectedJob.value || !jobForm.value.name || !jobForm.value.schedule) return

  // Validate handler-specific fields
  if (jobForm.value.handler === 'command' && !jobForm.value.command) return
  if (jobForm.value.handler === 'http' && !jobForm.value.url) return

  try {
    loading.value = true
    const payload = buildPayload()

    await cronApi.update(selectedJob.value.id, {
      name: jobForm.value.name,
      description: jobForm.value.description,
      schedule: jobForm.value.schedule,
      payload,
    })
    showEditModal.value = false
    await loadJobs()
  } catch {
    // Handle error
  } finally {
    loading.value = false
  }
}

async function toggleJob(job: CronJob) {
  try {
    if (job.enabled) {
      await cronApi.disable(job.id)
    } else {
      await cronApi.enable(job.id)
    }
    await loadJobs()
  } catch {
    // Handle error
  }
}

async function triggerJob(job: CronJob) {
  try {
    await cronApi.trigger(job.id)
    await loadExecutions(job)
  } catch {
    // Handle error
  }
}

async function deleteJob(job: CronJob) {
  if (!confirm(t('cron.confirmDelete', { name: job.name }))) return

  try {
    await cronApi.delete(job.id)
    await loadJobs()
  } catch {
    // Handle error
  }
}

async function loadExecutions(job: CronJob) {
  selectedJob.value = job
  try {
    const response = await cronApi.getExecutions(job.id, 20)
    executions.value = response.data
    showExecutionsModal.value = true
  } catch {
    executions.value = []
  }
}

function applyPreset(preset: string) {
  jobForm.value.schedule = preset
}

function formatDate(dateStr: string | undefined): string {
  if (!dateStr) return '-'
  return new Date(dateStr).toLocaleString()
}

function getNextRunText(job: CronJob): string {
  if (!job.enabled) return t('cron.disabled')
  if (!job.next_run) return t('cron.calculating')
  return formatDate(job.next_run)
}
</script>

<template>
  <div class="cron-view p-4 sm:p-6 max-w-6xl mx-auto">
    <div class="flex items-center justify-between mb-6">
      <h1 class="text-xl sm:text-2xl font-bold text-gray-900 dark:text-white">{{ t('cron.title') }}</h1>
      <button
        class="px-4 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg text-sm transition-colors flex items-center gap-2"
        @click="openCreateModal"
      >
        <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
        </svg>
        {{ t('cron.create') }}
      </button>
    </div>

    <!-- Jobs List -->
    <div v-if="loading" class="text-center py-8 text-gray-500 dark:text-slate-400">
      {{ t('common.loading') }}
    </div>

    <div v-else-if="jobs.length === 0" class="text-center py-8">
      <svg xmlns="http://www.w3.org/2000/svg" class="h-12 w-12 mx-auto text-gray-400 dark:text-slate-500 mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
      </svg>
      <p class="text-gray-500 dark:text-slate-400">{{ t('cron.noJobs') }}</p>
      <button
        class="mt-4 px-4 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg text-sm transition-colors"
        @click="openCreateModal"
      >
        {{ t('cron.createFirst') }}
      </button>
    </div>

    <div v-else class="space-y-4">
      <div
        v-for="job in jobs"
        :key="job.id"
        class="glass-card p-4"
      >
        <div class="flex items-start justify-between">
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-3 mb-2">
              <h3 class="text-gray-900 dark:text-white font-medium truncate">{{ job.name }}</h3>
              <span
                :class="[
                  'px-2 py-0.5 rounded-full text-xs font-medium',
                  job.enabled
                    ? 'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300'
                    : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
                ]"
              >
                {{ job.enabled ? t('cron.enabled') : t('cron.disabled') }}
              </span>
            </div>
            <p v-if="job.description" class="text-sm text-gray-500 dark:text-slate-400 mb-2">
              {{ job.description }}
            </p>
            <div class="flex flex-wrap items-center gap-4 text-xs text-gray-500 dark:text-slate-400">
              <div class="flex items-center gap-1">
                <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                <code class="bg-gray-100 dark:bg-slate-700 px-2 py-0.5 rounded">{{ job.schedule }}</code>
              </div>
              <div>
                {{ t('cron.nextRun') }}: {{ getNextRunText(job) }}
              </div>
              <div v-if="job.last_run">
                {{ t('cron.lastRun') }}: {{ formatDate(job.last_run) }}
              </div>
            </div>
          </div>
          <div class="flex items-center gap-2 ml-4">
            <button
              :title="t('cron.viewExecutions')"
              class="p-2 text-gray-500 dark:text-slate-400 hover:text-gray-900 dark:hover:text-white hover:bg-gray-100 dark:hover:bg-slate-700 rounded-lg transition-colors"
              @click="loadExecutions(job)"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
              </svg>
            </button>
            <button
              v-if="job.enabled"
              :title="t('cron.triggerNow')"
              class="p-2 text-blue-600 dark:text-blue-400 hover:bg-blue-50 dark:hover:bg-blue-900/20 rounded-lg transition-colors"
              @click="triggerJob(job)"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </button>
            <button
              :title="t('common.edit')"
              class="p-2 text-gray-500 dark:text-slate-400 hover:text-gray-900 dark:hover:text-white hover:bg-gray-100 dark:hover:bg-slate-700 rounded-lg transition-colors"
              @click="openEditModal(job)"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
              </svg>
            </button>
            <button
              :title="job.enabled ? t('cron.disable') : t('cron.enable')"
              :class="[
                'p-2 rounded-lg transition-colors',
                job.enabled
                  ? 'text-yellow-600 dark:text-yellow-400 hover:bg-yellow-50 dark:hover:bg-yellow-900/20'
                  : 'text-green-600 dark:text-green-400 hover:bg-green-50 dark:hover:bg-green-900/20'
              ]"
              @click="toggleJob(job)"
            >
              <svg v-if="job.enabled" xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 9v6m4-6v6m7-3a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              <svg v-else xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </button>
            <button
              :title="t('common.delete')"
              class="p-2 text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg transition-colors"
              @click="deleteJob(job)"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
              </svg>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Create/Edit Modal -->
    <div
      v-if="showCreateModal || showEditModal"
      class="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4"
      @click.self="showCreateModal = false; showEditModal = false"
    >
      <div class="bg-white dark:bg-slate-800 rounded-lg max-w-lg w-full max-h-[90vh] overflow-y-auto shadow-xl">
        <div class="p-4 sm:p-6">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">
            {{ showCreateModal ? t('cron.createNew') : t('cron.edit') }}
          </h3>

          <form class="space-y-4" @submit.prevent="showCreateModal ? createJob() : updateJob()">
            <div>
              <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">{{ t('cron.name') }}</label>
              <input
                v-model="jobForm.name"
                type="text"
                required
                :placeholder="t('cron.namePlaceholder')"
                class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-accent border border-gray-200 dark:border-slate-600"
              />
            </div>

            <div>
              <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">{{ t('cron.description') }}</label>
              <input
                v-model="jobForm.description"
                type="text"
                :placeholder="t('cron.descriptionPlaceholder')"
                class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-accent border border-gray-200 dark:border-slate-600"
              />
            </div>

            <div>
              <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">{{ t('cron.schedule') }}</label>
              <input
                v-model="jobForm.schedule"
                type="text"
                required
                placeholder="* * * * *"
                class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-accent border border-gray-200 dark:border-slate-600 font-mono"
              />
              <div class="flex flex-wrap gap-2 mt-2">
                <button
                  v-for="preset in cronPresets"
                  :key="preset.value"
                  type="button"
                  class="px-2 py-1 text-xs bg-gray-100 dark:bg-slate-700 hover:bg-gray-200 dark:hover:bg-slate-600 text-gray-700 dark:text-gray-300 rounded transition-colors"
                  @click="applyPreset(preset.value)"
                >
                  {{ preset.label }}
                </button>
              </div>
            </div>

            <div v-if="showCreateModal">
              <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">{{ t('cron.handler') }}</label>
              <div class="grid grid-cols-2 gap-2">
                <button
                  v-for="ht in handlerTypes"
                  :key="ht.value"
                  type="button"
                  :class="[
                    'p-3 rounded-lg border-2 text-left transition-colors',
                    jobForm.handler === ht.value
                      ? 'border-accent bg-accent/10 dark:bg-accent/20'
                      : 'border-gray-200 dark:border-slate-600 hover:border-gray-300 dark:hover:border-slate-500'
                  ]"
                  @click="jobForm.handler = ht.value"
                >
                  <div class="font-medium text-gray-900 dark:text-white text-sm">{{ ht.label }}</div>
                  <div class="text-xs text-gray-500 dark:text-slate-400 mt-1">{{ ht.description }}</div>
                </button>
              </div>
            </div>

            <!-- Command Handler Fields -->
            <div v-if="jobForm.handler === 'command'" class="space-y-4 p-4 bg-gray-50 dark:bg-slate-700/50 rounded-lg">
              <div>
                <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">{{ t('cron.commandInput') }} *</label>
                <input
                  v-model="jobForm.command"
                  type="text"
                  required
                  :placeholder="t('cron.commandPlaceholder')"
                  class="w-full bg-white dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-accent border border-gray-200 dark:border-slate-600 font-mono text-sm"
                />
                <p class="text-xs text-gray-400 dark:text-slate-500 mt-1">{{ t('cron.commandHint') }}</p>
              </div>
              <div>
                <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">{{ t('cron.workdir') }}</label>
                <input
                  v-model="jobForm.workdir"
                  type="text"
                  :placeholder="t('cron.workdirPlaceholder')"
                  class="w-full bg-white dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-accent border border-gray-200 dark:border-slate-600 font-mono text-sm"
                />
              </div>
              <div>
                <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">{{ t('cron.timeout') }} ({{ t('cron.seconds') }})</label>
                <input
                  v-model.number="jobForm.timeout"
                  type="number"
                  min="1"
                  max="3600"
                  class="w-full bg-white dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-accent border border-gray-200 dark:border-slate-600"
                />
              </div>
            </div>

            <!-- HTTP Handler Fields -->
            <div v-if="jobForm.handler === 'http'" class="space-y-4 p-4 bg-gray-50 dark:bg-slate-700/50 rounded-lg">
              <div>
                <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">URL *</label>
                <input
                  v-model="jobForm.url"
                  type="url"
                  required
                  placeholder="https://example.com/api/webhook"
                  class="w-full bg-white dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-accent border border-gray-200 dark:border-slate-600 font-mono text-sm"
                />
              </div>
              <div>
                <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">{{ t('cron.httpMethod') }}</label>
                <select
                  v-model="jobForm.method"
                  class="w-full bg-white dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-accent border border-gray-200 dark:border-slate-600"
                >
                  <option value="GET">GET</option>
                  <option value="POST">POST</option>
                  <option value="PUT">PUT</option>
                  <option value="DELETE">DELETE</option>
                </select>
              </div>
              <div>
                <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">{{ t('cron.timeout') }} ({{ t('cron.seconds') }})</label>
                <input
                  v-model.number="jobForm.timeout"
                  type="number"
                  min="1"
                  max="300"
                  class="w-full bg-white dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-accent border border-gray-200 dark:border-slate-600"
                />
              </div>
            </div>

            <div class="flex gap-3 pt-4">
              <button
                type="submit"
                :disabled="loading || !jobForm.name || !jobForm.schedule || (showCreateModal && !jobForm.handler) || (jobForm.handler === 'command' && !jobForm.command) || (jobForm.handler === 'http' && !jobForm.url)"
                class="flex-1 px-4 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg transition-colors disabled:opacity-50"
              >
                {{ loading ? t('common.saving') : (showCreateModal ? t('cron.create') : t('common.save')) }}
              </button>
              <button
                type="button"
                class="px-4 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-900 dark:text-white rounded-lg transition-colors"
                @click="showCreateModal = false; showEditModal = false"
              >
                {{ t('common.cancel') }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>

    <!-- Executions Modal -->
    <div
      v-if="showExecutionsModal && selectedJob"
      class="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4"
      @click.self="showExecutionsModal = false"
    >
      <div class="bg-white dark:bg-slate-800 rounded-lg max-w-2xl w-full max-h-[80vh] overflow-hidden shadow-xl">
        <div class="p-4 sm:p-6 border-b border-gray-200 dark:border-slate-700">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('cron.executionsFor', { name: selectedJob.name }) }}
          </h3>
        </div>

        <div class="p-4 sm:p-6 overflow-y-auto max-h-[60vh]">
          <div v-if="executions.length === 0" class="text-center py-8 text-gray-500 dark:text-slate-400">
            {{ t('cron.noExecutions') }}
          </div>

          <div v-else class="space-y-3">
            <div
              v-for="execution in executions"
              :key="execution.id"
              class="glass-card p-3"
            >
              <div class="flex items-center justify-between mb-2">
                <span
                  :class="[
                    'px-2 py-0.5 rounded-full text-xs font-medium',
                    execution.status === 'success'
                      ? 'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300'
                      : 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300'
                  ]"
                >
                  {{ execution.status }}
                </span>
                <span class="text-xs text-gray-400 dark:text-slate-500">
                  {{ formatDate(execution.started_at) }}
                </span>
              </div>
              <div class="flex items-center gap-4 text-xs text-gray-500 dark:text-slate-400">
                <span>{{ t('cron.duration') }}: {{ execution.duration_ms }}ms</span>
              </div>
              <div v-if="execution.error" class="mt-2 text-xs text-red-500 dark:text-red-400 bg-red-50 dark:bg-red-900/20 p-2 rounded">
                {{ execution.error }}
              </div>
            </div>
          </div>
        </div>

        <div class="p-4 sm:p-6 border-t border-gray-200 dark:border-slate-700">
          <button
            class="w-full px-4 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-900 dark:text-white rounded-lg transition-colors"
            @click="showExecutionsModal = false"
          >
            {{ t('common.close') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
