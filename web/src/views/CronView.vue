<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { cronApi } from '@/api/cron'
import type { CronJob, JobExecution } from '@/api/cron'
import { useNotificationStore } from '@/stores/notification'
import { getErrorMessage } from '@/utils/error'

const { t } = useI18n()
const notification = useNotificationStore()

const loading = ref(false)
const jobs = ref<CronJob[]>([])
const selectedJob = ref<CronJob | null>(null)
const executions = ref<JobExecution[]>([])
const showCreateModal = ref(false)
const showEditModal = ref(false)
const showExecutionsModal = ref(false)
const advancedOptionsOpen = ref(false)

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
  {
    value: 'command',
    label: t('cron.handlers.command'),
    description: t('cron.handlers.commandDesc'),
  },
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

const enabledJobsCount = computed(() => jobs.value.filter((job) => job.enabled).length)
const disabledJobsCount = computed(() => jobs.value.length - enabledJobsCount.value)
const sortedJobs = computed(() =>
  [...jobs.value].sort((left, right) => {
    const enabledDiff = Number(right.enabled) - Number(left.enabled)
    if (enabledDiff !== 0) return enabledDiff

    const nextRunDiff = parseDateValue(left.next_run_at) - parseDateValue(right.next_run_at)
    if (nextRunDiff !== 0) return nextRunDiff

    return left.name.localeCompare(right.name)
  })
)

const nextUpcomingJob = computed(() => {
  const upcomingJobs = jobs.value.filter((job) => job.enabled && job.next_run_at)
  if (upcomingJobs.length === 0) return null

  return [...upcomingJobs].sort(
    (left, right) => parseDateValue(left.next_run_at) - parseDateValue(right.next_run_at)
  )[0]
})

const nextUpcomingValue = computed(() => {
  if (nextUpcomingJob.value?.next_run_at) {
    return formatDate(nextUpcomingJob.value.next_run_at)
  }

  return jobs.value.length === 0 ? t('cron.noJobs') : t('cron.calculating')
})

onMounted(async () => {
  await loadJobs()
})

function getDefaultTimeout(handler: string): number {
  return handler === 'http' ? 30 : 60
}

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
    timeout: getDefaultTimeout('command'),
    url: '',
    method: 'GET',
  }
  advancedOptionsOpen.value = false
  showCreateModal.value = true
}

function openEditModal(job: CronJob) {
  selectedJob.value = job
  const payload = job.payload || {}
  const timeout =
    typeof payload.timeout === 'number' ? payload.timeout : getDefaultTimeout(job.handler)
  jobForm.value = {
    name: job.name,
    description: job.description || '',
    schedule: job.schedule,
    handler: job.handler,
    payload: job.payload ? JSON.stringify(job.payload, null, 2) : '{}',
    // Extract command handler fields
    command: (payload.command as string) || '',
    workdir: (payload.workdir as string) || '',
    timeout,
    // Extract HTTP handler fields
    url: (payload.url as string) || '',
    method: (payload.method as string) || 'GET',
  }
  advancedOptionsOpen.value =
    Boolean(job.description) ||
    (job.handler === 'command' &&
      (Boolean(payload.workdir) || timeout !== getDefaultTimeout('command'))) ||
    (job.handler === 'http' &&
      (jobForm.value.method !== 'GET' || timeout !== getDefaultTimeout('http')))
  showEditModal.value = true
}

function closeJobModal() {
  showCreateModal.value = false
  showEditModal.value = false
  advancedOptionsOpen.value = false
}

function setHandler(handler: string) {
  if (jobForm.value.handler === handler) return

  jobForm.value.handler = handler
  jobForm.value.timeout = getDefaultTimeout(handler)
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
    if (jobForm.value.timeout && jobForm.value.timeout !== getDefaultTimeout('command')) {
      payload.timeout = jobForm.value.timeout
    }
    return payload
  } else if (jobForm.value.handler === 'http') {
    const payload: Record<string, unknown> = {
      url: jobForm.value.url,
      method: jobForm.value.method,
    }
    if (jobForm.value.timeout && jobForm.value.timeout !== getDefaultTimeout('http')) {
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
  } catch (error) {
    notification.error(t('common.error'), getErrorMessage(error), { titleKey: 'common.error' })
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
  } catch (error) {
    notification.error(t('common.error'), getErrorMessage(error), { titleKey: 'common.error' })
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
  } catch (error) {
    notification.error(t('common.error'), getErrorMessage(error), { titleKey: 'common.error' })
  }
}

async function triggerJob(job: CronJob) {
  try {
    await cronApi.trigger(job.id)
    await loadExecutions(job)
  } catch (error) {
    notification.error(t('common.error'), getErrorMessage(error), { titleKey: 'common.error' })
  }
}

async function deleteJob(job: CronJob) {
  if (!confirm(t('cron.confirmDelete', { name: job.name }))) return

  try {
    await cronApi.delete(job.id)
    await loadJobs()
  } catch (error) {
    notification.error(t('common.error'), getErrorMessage(error), { titleKey: 'common.error' })
  }
}

async function loadExecutions(job: CronJob) {
  selectedJob.value = job
  try {
    const response = await cronApi.getExecutions(job.id, 20)
    executions.value = response.data
    showExecutionsModal.value = true
  } catch (error) {
    executions.value = []
    notification.error(t('common.error'), getErrorMessage(error), { titleKey: 'common.error' })
  }
}

function applyPreset(preset: string) {
  jobForm.value.schedule = preset
}

function parseDateValue(dateStr: string | undefined): number {
  if (!dateStr) return Number.MAX_SAFE_INTEGER
  const parsed = Date.parse(dateStr)
  return Number.isNaN(parsed) ? Number.MAX_SAFE_INTEGER : parsed
}

function formatDate(dateStr: string | undefined): string {
  if (!dateStr) return '-'
  return new Date(dateStr).toLocaleString()
}

function getNextRunText(job: CronJob): string {
  if (!job.enabled) return t('cron.disabled')
  if (!job.next_run_at) return t('cron.calculating')
  return formatDate(job.next_run_at)
}

function getExecutionDurationMs(execution: JobExecution): number {
  if (!execution.duration) return 0
  return Math.round(execution.duration / 1_000_000)
}

function getJobHandlerLabel(handler: string): string {
  if (handler === 'command') return t('cron.handlers.command')
  if (handler === 'http') return t('cron.handlers.http')
  return handler
}

function getJobPreview(job: CronJob): string {
  const payload = job.payload || {}

  if (job.handler === 'command' && typeof payload.command === 'string') {
    return payload.command
  }

  if (job.handler === 'http') {
    const method = typeof payload.method === 'string' ? payload.method : 'GET'
    const url = typeof payload.url === 'string' ? payload.url : ''
    return [method, url].filter(Boolean).join(' ')
  }

  return ''
}
</script>

<template>
  <div class="cron-page dashboard-page-frame">
    <section class="automation-stage dashboard-page-stage configuration-page-stage">
      <section class="automation-hero dashboard-page-hero configuration-page-hero">
        <div class="automation-copy dashboard-page-copy configuration-page-copy">
          <p class="automation-kicker dashboard-page-eyebrow">{{ t('nav.configuration') }}</p>
          <h1 class="automation-title dashboard-page-title configuration-page-title">
            {{ t('cron.title') }}
          </h1>
          <p class="automation-description dashboard-page-description configuration-page-description">
            {{ t('automation.tabs.cronDesc') }}
          </p>
        </div>
      </section>

      <section class="automation-shell">
        <section class="automation-surface-panel automation-panel-card automation-library-card">
          <div class="automation-list-header">
            <div>
              <p class="dashboard-card-label">{{ t('automation.title') }}</p>
              <h2 class="automation-section-title">{{ t('cron.title') }}</h2>
              <p class="automation-section-description">{{ t('automation.tabs.cronDesc') }}</p>
            </div>
            <div class="automation-library-card__aside">
              <div class="automation-library-card__topline">
                <span class="automation-count-chip">{{ sortedJobs.length }}</span>
                <div class="automation-library-card__controls">
                  <button
                    class="automation-refresh-button"
                    type="button"
                    :disabled="loading"
                    :title="t('common.refresh')"
                    :aria-label="t('common.refresh')"
                    @click="loadJobs"
                  >
                    <svg
                      class="h-4 w-4"
                      :class="{ 'animate-spin': loading }"
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                    >
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
                      />
                    </svg>
                  </button>
                  <button class="automation-create-button" type="button" @click="openCreateModal">
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      class="h-4 w-4"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M12 4v16m8-8H4"
                      />
                    </svg>
                    {{ t('cron.create') }}
                  </button>
                </div>
              </div>
              <p class="automation-library-card__note">
                {{ enabledJobsCount }} {{ t('automation.stats.activeJobs') }}
                <span v-if="disabledJobsCount > 0">
                  · {{ disabledJobsCount }} {{ t('cron.disabled') }}
                </span>
              </p>
              <p class="automation-library-card__note">
                {{ t('cron.nextRun') }}:
                {{ nextUpcomingValue }}
              </p>
            </div>
          </div>
        </section>

        <section
          v-if="loading"
          class="automation-surface-panel automation-state-card automation-state"
        >
          {{ t('common.loading') }}
        </section>

        <section
          v-else-if="sortedJobs.length === 0"
          class="automation-surface-panel automation-state-card automation-empty"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="automation-empty-icon"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="1.75"
              d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
            />
          </svg>
          <h3 class="automation-empty-title">{{ t('cron.noJobs') }}</h3>
          <p class="automation-empty-description">{{ t('automation.tabs.cronDesc') }}</p>
          <button class="automation-create-inline" @click="openCreateModal">
            {{ t('cron.createFirst') }}
          </button>
        </section>

        <div v-else class="automation-job-grid">
          <article
            v-for="job in sortedJobs"
            :key="job.id"
            class="automation-surface-card automation-panel-card automation-job-card"
          >
            <div class="automation-job-main">
              <div class="automation-job-badges">
                <span class="automation-badge automation-badge--handler">
                  {{ getJobHandlerLabel(job.handler) }}
                </span>
                <span
                  class="automation-badge"
                  :class="job.enabled ? 'automation-badge--enabled' : 'automation-badge--disabled'"
                >
                  {{ job.enabled ? t('cron.enabled') : t('cron.disabled') }}
                </span>
                <span v-if="job.fail_count > 0" class="automation-badge automation-badge--danger">
                  {{ job.fail_count }} fail
                </span>
              </div>

              <h3 class="automation-job-name">{{ job.name }}</h3>

              <p v-if="job.description" class="automation-job-description">
                {{ job.description }}
              </p>
              <p
                v-else-if="getJobPreview(job)"
                class="automation-job-description automation-job-description--mono"
              >
                {{ getJobPreview(job) }}
              </p>
            </div>

            <div class="automation-job-meta-grid">
              <div class="automation-subsurface-card automation-meta-card">
                <span class="automation-meta-label">{{ t('cron.schedule') }}</span>
                <span class="automation-meta-value automation-meta-value--mono">
                  {{ job.schedule }}
                </span>
              </div>
              <div class="automation-subsurface-card automation-meta-card">
                <span class="automation-meta-label">{{ t('cron.nextRun') }}</span>
                <span class="automation-meta-value automation-meta-value--strong">
                  {{ getNextRunText(job) }}
                </span>
              </div>
              <div class="automation-subsurface-card automation-meta-card">
                <span class="automation-meta-label">{{ t('cron.lastRun') }}</span>
                <span class="automation-meta-value">
                  {{ job.last_run_at ? formatDate(job.last_run_at) : '-' }}
                </span>
              </div>
            </div>

            <div class="automation-job-footer">
              <p class="automation-job-footnote">{{ job.run_count ?? 0 }} runs</p>

              <div class="automation-job-actions">
                <button
                  :title="t('cron.viewExecutions')"
                  class="automation-action-button"
                  @click="loadExecutions(job)"
                >
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    class="h-5 w-5"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"
                    />
                  </svg>
                </button>
                <button
                  v-if="job.enabled"
                  :title="t('cron.triggerNow')"
                  class="automation-action-button automation-action-button--primary"
                  @click="triggerJob(job)"
                >
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    class="h-5 w-5"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z"
                    />
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                    />
                  </svg>
                </button>
                <button
                  :title="t('common.edit')"
                  class="automation-action-button"
                  @click="openEditModal(job)"
                >
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    class="h-5 w-5"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
                    />
                  </svg>
                </button>
                <button
                  :title="job.enabled ? t('cron.disable') : t('cron.enable')"
                  class="automation-action-button"
                  :class="
                    job.enabled
                      ? 'automation-action-button--warning'
                      : 'automation-action-button--success'
                  "
                  @click="toggleJob(job)"
                >
                  <svg
                    v-if="job.enabled"
                    xmlns="http://www.w3.org/2000/svg"
                    class="h-5 w-5"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M10 9v6m4-6v6m7-3a9 9 0 11-18 0 9 9 0 0118 0z"
                    />
                  </svg>
                  <svg
                    v-else
                    xmlns="http://www.w3.org/2000/svg"
                    class="h-5 w-5"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z"
                    />
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                    />
                  </svg>
                </button>
                <button
                  :title="t('common.delete')"
                  class="automation-action-button automation-action-button--danger"
                  @click="deleteJob(job)"
                >
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    class="h-5 w-5"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                    />
                  </svg>
                </button>
              </div>
            </div>
          </article>
        </div>
      </section>
    </section>

    <!-- Create/Edit Modal -->
    <div
      v-if="showCreateModal || showEditModal"
      class="automation-modal-backdrop"
      @click.self="closeJobModal"
    >
      <div class="automation-surface-panel automation-modal-panel automation-modal-panel--form">
        <div class="automation-modal-header">
          <h3 class="automation-modal-title">
            {{ showCreateModal ? t('cron.createNew') : t('cron.edit') }}
          </h3>
        </div>

        <form
          class="automation-form automation-form--modal"
          @submit.prevent="showCreateModal ? createJob() : updateJob()"
        >
          <div class="automation-modal-scroll automation-modal-scroll--form">
            <div class="automation-form-field">
              <label class="automation-label">{{ t('cron.name') }}</label>
              <input
                v-model="jobForm.name"
                type="text"
                required
                :placeholder="t('cron.namePlaceholder')"
                class="automation-input"
              />
            </div>

            <div class="automation-form-field">
              <label class="automation-label">{{ t('cron.schedule') }}</label>
              <input
                v-model="jobForm.schedule"
                type="text"
                required
                placeholder="* * * * *"
                class="automation-input automation-input--mono"
              />
              <div class="automation-chip-group">
                <button
                  v-for="preset in cronPresets"
                  :key="preset.value"
                  type="button"
                  class="automation-chip-button"
                  @click="applyPreset(preset.value)"
                >
                  {{ preset.label }}
                </button>
              </div>
            </div>

            <div v-if="showCreateModal" class="automation-form-field">
              <label class="automation-label">{{ t('cron.handler') }}</label>
              <div class="automation-handler-grid">
                <button
                  v-for="ht in handlerTypes"
                  :key="ht.value"
                  type="button"
                  :class="[
                    'automation-handler-card',
                    { 'automation-handler-card--active': jobForm.handler === ht.value },
                  ]"
                  @click="setHandler(ht.value)"
                >
                  <div class="automation-handler-card-title">{{ ht.label }}</div>
                  <div class="automation-handler-card-description">{{ ht.description }}</div>
                </button>
              </div>
            </div>

            <!-- Command Handler Fields -->
            <div v-if="jobForm.handler === 'command'" class="automation-handler-panel">
              <div class="automation-form-field">
                <label class="automation-label">{{ t('cron.commandInput') }} *</label>
                <input
                  v-model="jobForm.command"
                  type="text"
                  required
                  :placeholder="t('cron.commandPlaceholder')"
                  class="automation-input automation-input--mono"
                />
                <p class="automation-hint">{{ t('cron.commandHint') }}</p>
              </div>
            </div>

            <!-- HTTP Handler Fields -->
            <div v-if="jobForm.handler === 'http'" class="automation-handler-panel">
              <div class="automation-form-field">
                <label class="automation-label">URL *</label>
                <input
                  v-model="jobForm.url"
                  type="url"
                  required
                  placeholder="https://example.com/api/webhook"
                  class="automation-input automation-input--mono"
                />
              </div>
            </div>

            <div class="automation-disclosure">
              <button
                type="button"
                class="automation-disclosure-button"
                :aria-expanded="advancedOptionsOpen"
                aria-controls="cron-optional-settings"
                @click="advancedOptionsOpen = !advancedOptionsOpen"
              >
                <span class="automation-disclosure-copy">
                  <span class="automation-disclosure-title">
                    {{ t('cron.optionalSettings') }}
                  </span>
                  <span class="automation-disclosure-caption">
                    {{ t('cron.optionalSettingsHint') }}
                  </span>
                </span>
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  class="automation-disclosure-icon"
                  :class="{ 'automation-disclosure-icon--open': advancedOptionsOpen }"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="1.8"
                    d="M19 9l-7 7-7-7"
                  />
                </svg>
              </button>

              <div
                v-if="advancedOptionsOpen"
                id="cron-optional-settings"
                class="automation-disclosure-panel"
              >
                <div class="automation-form-grid automation-form-grid--compact">
                  <div class="automation-form-field automation-form-field--full">
                    <label class="automation-label">{{ t('cron.description') }}</label>
                    <input
                      v-model="jobForm.description"
                      type="text"
                      :placeholder="t('cron.descriptionPlaceholder')"
                      class="automation-input"
                    />
                  </div>

                  <template v-if="jobForm.handler === 'command'">
                    <div class="automation-form-field automation-form-field--full">
                      <label class="automation-label">{{ t('cron.workdir') }}</label>
                      <input
                        v-model="jobForm.workdir"
                        type="text"
                        :placeholder="t('cron.workdirPlaceholder')"
                        class="automation-input automation-input--mono"
                      />
                    </div>
                    <div class="automation-form-field">
                      <label class="automation-label"
                        >{{ t('cron.timeout') }} ({{ t('cron.seconds') }})</label
                      >
                      <input
                        v-model.number="jobForm.timeout"
                        type="number"
                        min="1"
                        max="3600"
                        class="automation-input"
                      />
                    </div>
                  </template>

                  <template v-if="jobForm.handler === 'http'">
                    <div class="automation-form-field">
                      <label class="automation-label">{{ t('cron.httpMethod') }}</label>
                      <select v-model="jobForm.method" class="automation-input">
                        <option value="GET">GET</option>
                        <option value="POST">POST</option>
                        <option value="PUT">PUT</option>
                        <option value="DELETE">DELETE</option>
                      </select>
                    </div>
                    <div class="automation-form-field">
                      <label class="automation-label"
                        >{{ t('cron.timeout') }} ({{ t('cron.seconds') }})</label
                      >
                      <input
                        v-model.number="jobForm.timeout"
                        type="number"
                        min="1"
                        max="300"
                        class="automation-input"
                      />
                    </div>
                  </template>
                </div>
              </div>
            </div>
          </div>
          <div class="automation-modal-footer">
            <div class="automation-form-actions">
              <button
                type="submit"
                :disabled="
                  loading ||
                  !jobForm.name ||
                  !jobForm.schedule ||
                  (showCreateModal && !jobForm.handler) ||
                  (jobForm.handler === 'command' && !jobForm.command) ||
                  (jobForm.handler === 'http' && !jobForm.url)
                "
                class="automation-submit-button"
              >
                {{
                  loading
                    ? t('common.saving')
                    : showCreateModal
                      ? t('cron.create')
                      : t('common.save')
                }}
              </button>
              <button type="button" class="automation-secondary-button" @click="closeJobModal">
                {{ t('common.cancel') }}
              </button>
            </div>
          </div>
        </form>
      </div>
    </div>

    <!-- Executions Modal -->
    <div
      v-if="showExecutionsModal && selectedJob"
      class="automation-modal-backdrop"
      @click.self="showExecutionsModal = false"
    >
      <div class="automation-surface-panel automation-modal-panel automation-modal-panel--wide">
        <div class="automation-modal-header">
          <h3 class="automation-modal-title">
            {{ t('cron.executionsFor', { name: selectedJob.name }) }}
          </h3>
        </div>

        <div class="automation-modal-scroll">
          <div v-if="executions.length === 0" class="automation-state">
            {{ t('cron.noExecutions') }}
          </div>

          <div v-else class="automation-execution-list">
            <div
              v-for="execution in executions"
              :key="execution.id"
              class="automation-surface-card automation-execution-card"
            >
              <div class="automation-execution-head">
                <span
                  class="automation-badge"
                  :class="[
                    execution.status === 'completed'
                      ? 'automation-badge--enabled'
                      : execution.status === 'failed'
                        ? 'automation-badge--danger'
                        : 'automation-badge--disabled',
                  ]"
                >
                  {{ execution.status }}
                </span>
                <span class="automation-execution-date">
                  {{ formatDate(execution.started_at) }}
                </span>
              </div>

              <div class="automation-execution-meta">
                <span>{{ t('cron.duration') }}: {{ getExecutionDurationMs(execution) }}ms</span>
              </div>

              <div v-if="execution.error" class="automation-error-surface">
                {{ execution.error }}
              </div>
            </div>
          </div>
        </div>

        <div class="automation-modal-footer">
          <button
            class="automation-secondary-button automation-secondary-button--full"
            @click="showExecutionsModal = false"
          >
            {{ t('common.close') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.cron-page {
  --dashboard-page-accent: 37, 99, 235;
  max-width: 1480px;
  margin: 0 auto;
  padding: 0 0.75rem 1.8rem;
}

.automation-stage {
  position: relative;
  padding: 1.15rem 0 0.35rem;
}

.automation-stage::before,
.automation-stage::after {
  content: none;
  position: absolute;
  width: 18rem;
  height: 18rem;
  pointer-events: none;
  opacity: 0.82;
  background-image: none;
  background-size: 14px 14px;
  z-index: 0;
}

.automation-stage::before {
  right: 12%;
  top: 7.6rem;
}

.automation-stage::after {
  left: 16%;
  bottom: -1.25rem;
}

.automation-hero {
  position: relative;
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  padding: 0 0 0.2rem;
  margin-bottom: 0;
  border: 0;
  border-radius: 0;
  background: transparent;
  box-shadow: none;
}

.automation-copy {
  position: relative;
  z-index: 1;
  flex: 1 1 0%;
  min-width: 0;
  max-width: 42rem;
  padding-top: 0.1rem;
}

.automation-kicker {
  margin: 0;
}

.automation-title {
  margin: 0;
  font-size: clamp(1.34rem, 0.7vw + 0.95rem, 1.9rem);
  line-height: 1.06;
  letter-spacing: -0.04em;
  font-weight: 700;
  color: #111827;
}

.automation-description {
  margin: 0.42rem 0 0;
  max-width: 34rem;
  font-size: 0.92rem;
  line-height: 1.55;
  color: #9ca3af;
}

.automation-refresh-button,
.automation-create-button,
.automation-create-inline,
.automation-action-button,
.automation-disclosure-button,
.automation-chip-button,
.automation-submit-button,
.automation-secondary-button {
  transition:
    transform 0.18s ease,
    box-shadow 0.18s ease,
    border-color 0.18s ease,
    background-color 0.18s ease,
    color 0.18s ease,
    opacity 0.18s ease;
}

.automation-refresh-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2.5rem;
  height: 2.5rem;
  border: 1px solid rgba(226, 232, 240, 0.96);
  border-radius: 999px;
  color: #475569;
  background: #ffffff;
  box-shadow: none;
}

.automation-refresh-button:hover:not(:disabled),
.automation-refresh-button:focus-visible {
  color: #1d4ed8;
  border-color: rgba(37, 99, 235, 0.24);
  background: #f8fafc;
  transform: translateY(-1px);
}

.automation-refresh-button:disabled {
  cursor: not-allowed;
  opacity: 0.72;
}

.automation-create-button,
.automation-create-inline,
.automation-submit-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  min-height: 2.65rem;
  padding: 0.7rem 1.05rem;
  border: 0;
  border-radius: 999px;
  background: #0f172a;
  color: #f8fafc;
  font-size: 0.88rem;
  font-weight: 600;
  box-shadow: none;
}

.automation-create-button:hover,
.automation-create-inline:hover,
.automation-submit-button:hover:not(:disabled) {
  transform: translateY(-1px);
  background: #1e293b;
}

.automation-submit-button {
  flex: 1;
  border-radius: 0.95rem;
}

.automation-submit-button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
  transform: none;
  background: rgba(15, 23, 42, 0.55);
}

.automation-secondary-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 2.65rem;
  padding: 0.7rem 1rem;
  border: 1px solid rgba(148, 163, 184, 0.24);
  border-radius: 0.95rem;
  background: rgba(255, 255, 255, 0.72);
  color: #475569;
  font-size: 0.88rem;
  font-weight: 600;
}

.automation-secondary-button:hover,
.automation-chip-button:hover,
.automation-action-button:hover {
  transform: translateY(-1px);
}

.automation-secondary-button--full {
  width: 100%;
}

.automation-surface-panel,
.automation-surface-card,
.automation-subsurface-card {
  position: relative;
  overflow: hidden;
  transition:
    border-color 0.22s ease,
    background-color 0.22s ease,
    color 0.22s ease;
  box-shadow: none;
}

.automation-surface-panel {
  border: 1px solid rgba(203, 213, 225, 0.96);
  border-radius: 1.5rem;
  background: #f8fafc;
}

.automation-surface-card {
  border: 1px solid rgba(226, 232, 240, 0.96);
  border-radius: 1.25rem;
  background: #ffffff;
}

.automation-subsurface-card {
  border: 1px solid rgba(226, 232, 240, 0.96);
  border-radius: 0.95rem;
  background: rgba(248, 250, 252, 0.96);
}

.automation-shell {
  position: relative;
  z-index: 1;
  display: flex;
  flex-direction: column;
  gap: 1rem;
  max-width: 82rem;
  margin: 0 auto;
}

.automation-panel-card,
.automation-state-card {
  padding: 1.1rem;
}

.automation-library-card__aside {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 0.38rem;
}

.automation-library-card__topline {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  width: 100%;
}

.automation-library-card__controls {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.6rem;
  flex-wrap: wrap;
}

.automation-library-card__note {
  margin: 0;
  color: #64748b;
  font-size: 0.8rem;
  line-height: 1.45;
  text-align: right;
}

.automation-list-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.8rem;
}

.automation-section-title {
  margin: 0.35rem 0 0;
  color: #0f172a;
  font-size: 1.15rem;
  font-weight: 700;
}

.automation-section-description {
  max-width: 44rem;
  margin: 0.45rem 0 0;
  color: #64748b;
  font-size: 0.9rem;
  line-height: 1.6;
}

.automation-count-chip {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 2rem;
  min-height: 2rem;
  padding: 0.35rem 0.68rem;
  border-radius: 999px;
  border: 1px solid rgba(147, 197, 253, 0.55);
  background: rgba(37, 99, 235, 0.08);
  color: #1d4ed8;
  font-size: 0.82rem;
  font-weight: 700;
}

.automation-state,
.automation-empty {
  padding: 2.2rem 1rem;
  text-align: center;
}

.automation-state {
  color: #64748b;
  font-size: 0.92rem;
}

.automation-empty-icon {
  width: 3.3rem;
  height: 3.3rem;
  margin: 0 auto;
  color: #94a3b8;
}

.automation-empty-title {
  margin: 1rem 0 0;
  color: #111827;
  font-size: 1rem;
  font-weight: 700;
}

.automation-empty-description {
  max-width: 24rem;
  margin: 0.45rem auto 0;
  color: #64748b;
  font-size: 0.9rem;
  line-height: 1.55;
}

.automation-create-inline {
  margin-top: 1.05rem;
}

.automation-job-grid {
  display: grid;
  gap: 1rem;
}

.automation-job-card {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.automation-job-main {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
}

.automation-job-badges {
  display: flex;
  flex-wrap: wrap;
  gap: 0.45rem;
  margin-bottom: 0.72rem;
}

.automation-badge {
  display: inline-flex;
  align-items: center;
  min-height: 1.45rem;
  padding: 0.12rem 0.56rem;
  border-radius: 999px;
  font-size: 0.72rem;
  font-weight: 600;
}

.automation-badge--enabled {
  background: rgba(16, 185, 129, 0.12);
  color: #047857;
}

.automation-badge--disabled {
  background: rgba(148, 163, 184, 0.14);
  color: #475569;
}

.automation-badge--handler {
  background: rgba(59, 130, 246, 0.1);
  color: #1d4ed8;
}

.automation-badge--danger {
  background: rgba(239, 68, 68, 0.12);
  color: #b91c1c;
}

.automation-job-name {
  margin: 0;
  color: #111827;
  font-size: 1rem;
  font-weight: 650;
  line-height: 1.3;
}

.automation-job-footnote {
  margin: 0;
  color: #64748b;
  font-size: 0.8rem;
  line-height: 1.45;
}

.automation-job-description {
  margin: 0.42rem 0 0;
  color: #64748b;
  font-size: 0.88rem;
  line-height: 1.55;
  word-break: break-word;
}

.automation-job-description--mono {
  font-family:
    ui-monospace,
    SFMono-Regular,
    SFMono-Regular,
    Menlo,
    Monaco,
    Consolas,
    Liberation Mono,
    Courier New,
    monospace;
  font-size: 0.8rem;
}

.automation-job-meta-grid {
  display: grid;
  gap: 0.78rem;
  grid-template-columns: repeat(1, minmax(0, 1fr));
}

.automation-meta-card {
  display: flex;
  flex-direction: column;
  gap: 0.38rem;
  min-width: 0;
  padding: 0.9rem 0.95rem;
}

.automation-meta-label {
  color: #64748b;
  font-size: 0.7rem;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.automation-meta-value {
  color: #334155;
  font-size: 0.83rem;
  line-height: 1.5;
  word-break: break-word;
}

.automation-meta-value--strong {
  color: #0f172a;
  font-weight: 650;
}

.automation-meta-value--mono {
  font-family:
    ui-monospace,
    SFMono-Regular,
    Menlo,
    Monaco,
    Consolas,
    Liberation Mono,
    Courier New,
    monospace;
  font-size: 0.78rem;
}

.automation-job-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.9rem;
  margin-top: auto;
}

.automation-job-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  gap: 0.55rem;
}

.automation-action-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2.45rem;
  height: 2.45rem;
  border: 1px solid rgba(226, 232, 240, 0.96);
  border-radius: 0.95rem;
  background: #ffffff;
  color: #475569;
  box-shadow: none;
}

.automation-action-button--primary {
  border-color: #0f172a;
  background: #0f172a;
  color: #f8fafc;
}

.automation-action-button--warning {
  border-color: rgba(245, 158, 11, 0.18);
  background: rgba(245, 158, 11, 0.08);
  color: #b45309;
}

.automation-action-button--success {
  border-color: rgba(16, 185, 129, 0.18);
  background: rgba(16, 185, 129, 0.08);
  color: #047857;
}

.automation-action-button--danger {
  border-color: rgba(239, 68, 68, 0.18);
  background: rgba(239, 68, 68, 0.08);
  color: #b91c1c;
}

.automation-modal-backdrop {
  position: fixed;
  inset: 0;
  z-index: 50;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 1rem;
  background: rgba(15, 23, 42, 0.48);
  backdrop-filter: blur(6px);
  -webkit-backdrop-filter: blur(6px);
}

.automation-modal-panel {
  width: min(100%, 34rem);
  max-height: 90vh;
  overflow: hidden;
}

.automation-modal-panel--form {
  width: min(100%, 38rem);
  display: flex;
  flex-direction: column;
}

.automation-modal-panel--wide {
  width: min(100%, 42rem);
  max-height: 80vh;
}

.automation-modal-header,
.automation-modal-body,
.automation-modal-footer {
  padding: 1.05rem 1.1rem;
}

.automation-modal-header,
.automation-modal-footer {
  border-color: rgba(226, 232, 240, 0.92);
}

.automation-modal-header {
  border-bottom: 1px solid rgba(226, 232, 240, 0.92);
}

.automation-modal-footer {
  border-top: 1px solid rgba(226, 232, 240, 0.92);
}

.automation-modal-body {
  overflow-y: auto;
}

.automation-modal-scroll {
  max-height: 60vh;
  overflow-y: auto;
  padding: 1.05rem 1.1rem;
}

.automation-modal-scroll--form {
  flex: 1;
  min-height: 0;
  max-height: none;
}

.automation-modal-title {
  margin: 0;
  color: #111827;
  font-size: 1.06rem;
  font-weight: 700;
}

.automation-form {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.automation-form--modal {
  flex: 1;
  min-height: 0;
  gap: 0;
}

.automation-form-grid {
  display: grid;
  gap: 1rem;
}

.automation-form-field {
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
}

.automation-form-field--full {
  grid-column: 1 / -1;
}

.automation-label {
  color: #334155;
  font-size: 0.84rem;
  font-weight: 600;
}

.automation-input {
  width: 100%;
  min-height: 2.8rem;
  padding: 0.72rem 0.95rem;
  border: 1px solid rgba(148, 163, 184, 0.34);
  border-radius: 0.95rem;
  background: rgba(255, 255, 255, 0.9);
  color: #0f172a;
  outline: none;
  transition:
    border-color 160ms ease,
    box-shadow 160ms ease,
    background-color 160ms ease;
}

.automation-input:focus {
  border-color: rgba(37, 99, 235, 0.45);
  box-shadow: 0 0 0 4px rgba(37, 99, 235, 0.12);
}

.automation-input--mono {
  font-family:
    ui-monospace,
    SFMono-Regular,
    SFMono-Regular,
    Menlo,
    Monaco,
    Consolas,
    Liberation Mono,
    Courier New,
    monospace;
  font-size: 0.85rem;
}

.automation-chip-group {
  display: flex;
  flex-wrap: wrap;
  gap: 0.45rem;
}

.automation-chip-button {
  min-height: 2.2rem;
  padding: 0.4rem 0.72rem;
  border: 1px solid rgba(148, 163, 184, 0.24);
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.72);
  color: #475569;
  font-size: 0.76rem;
  font-weight: 700;
}

.automation-handler-grid {
  display: grid;
  gap: 0.65rem;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.automation-handler-card {
  padding: 0.95rem 1rem;
  border: 1px solid rgba(226, 232, 240, 0.96);
  border-radius: 1.25rem;
  background: #ffffff;
  text-align: left;
  transition:
    border-color 0.22s ease,
    background-color 0.22s ease,
    color 0.22s ease;
}

.automation-handler-card--active {
  border-color: rgba(37, 99, 235, 0.36);
  background: rgba(37, 99, 235, 0.08);
}

.automation-handler-card-title {
  color: #111827;
  font-size: 0.86rem;
  font-weight: 650;
}

.automation-handler-card-description {
  margin-top: 0.3rem;
  color: #6b7280;
  font-size: 0.76rem;
  line-height: 1.45;
}

.automation-handler-panel {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  padding: 1rem;
  border: 1px solid rgba(203, 213, 225, 0.96);
  border-radius: 1.25rem;
  background: #f8fafc;
}

.automation-hint {
  margin: 0;
  color: #94a3b8;
  font-size: 0.75rem;
  line-height: 1.5;
}

.automation-disclosure {
  margin-top: 1rem;
}

.automation-disclosure-button {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.85rem;
  width: 100%;
  padding: 0.92rem 1rem;
  border: 1px solid rgba(203, 213, 225, 0.92);
  border-radius: 1.15rem;
  background: rgba(248, 250, 252, 0.92);
  color: #0f172a;
  text-align: left;
}

.automation-disclosure-button:hover {
  border-color: rgba(37, 99, 235, 0.22);
  background: rgba(239, 246, 255, 0.92);
}

.automation-disclosure-copy {
  display: flex;
  flex-direction: column;
  gap: 0.18rem;
}

.automation-disclosure-title {
  color: #111827;
  font-size: 0.88rem;
  font-weight: 650;
}

.automation-disclosure-caption {
  color: #64748b;
  font-size: 0.76rem;
  line-height: 1.45;
}

.automation-disclosure-icon {
  width: 1rem;
  height: 1rem;
  margin-top: 0.1rem;
  color: #64748b;
  flex-shrink: 0;
  transition: transform 0.18s ease;
}

.automation-disclosure-icon--open {
  transform: rotate(180deg);
}

.automation-disclosure-panel {
  margin-top: 0.75rem;
  padding: 1rem;
  border: 1px solid rgba(203, 213, 225, 0.92);
  border-radius: 1.25rem;
  background: #f8fafc;
}

.automation-form-actions {
  display: flex;
  gap: 0.75rem;
  padding-top: 0.25rem;
}

.automation-execution-list {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.automation-execution-card {
  padding: 0.9rem;
}

.automation-execution-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.automation-execution-date {
  color: #94a3b8;
  font-size: 0.74rem;
}

.automation-execution-meta {
  margin-top: 0.55rem;
  color: #64748b;
  font-size: 0.8rem;
}

.automation-error-surface {
  margin-top: 0.62rem;
  padding: 0.62rem 0.72rem;
  border-radius: 0.9rem;
  background: rgba(254, 226, 226, 0.72);
  color: #b91c1c;
  font-size: 0.76rem;
  line-height: 1.5;
}

:root.dark .automation-title,
[data-theme='dark'] .automation-title,
html.dark .automation-title,
:root.dark .automation-section-title,
[data-theme='dark'] .automation-section-title,
html.dark .automation-section-title,
:root.dark .automation-empty-title,
[data-theme='dark'] .automation-empty-title,
html.dark .automation-empty-title,
:root.dark .automation-job-name,
[data-theme='dark'] .automation-job-name,
html.dark .automation-job-name,
:root.dark .automation-modal-title,
[data-theme='dark'] .automation-modal-title,
html.dark .automation-modal-title,
:root.dark .automation-handler-card-title,
[data-theme='dark'] .automation-handler-card-title,
html.dark .automation-handler-card-title,
:root.dark .automation-disclosure-title,
[data-theme='dark'] .automation-disclosure-title,
html.dark .automation-disclosure-title {
  color: rgb(241 245 249);
}

:root.dark .automation-description,
[data-theme='dark'] .automation-description,
html.dark .automation-description,
:root.dark .automation-section-description,
[data-theme='dark'] .automation-section-description,
html.dark .automation-section-description,
:root.dark .automation-state,
[data-theme='dark'] .automation-state,
html.dark .automation-state,
:root.dark .automation-empty-description,
[data-theme='dark'] .automation-empty-description,
html.dark .automation-empty-description,
:root.dark .automation-job-description,
[data-theme='dark'] .automation-job-description,
html.dark .automation-job-description,
:root.dark .automation-handler-card-description,
[data-theme='dark'] .automation-handler-card-description,
html.dark .automation-handler-card-description,
:root.dark .automation-library-card__note,
[data-theme='dark'] .automation-library-card__note,
html.dark .automation-library-card__note,
:root.dark .automation-hint,
[data-theme='dark'] .automation-hint,
html.dark .automation-hint,
:root.dark .automation-disclosure-caption,
[data-theme='dark'] .automation-disclosure-caption,
html.dark .automation-disclosure-caption,
:root.dark .automation-job-footnote,
[data-theme='dark'] .automation-job-footnote,
html.dark .automation-job-footnote {
  color: rgb(148 163 184);
}

:root.dark .automation-stage::before,
[data-theme='dark'] .automation-stage::before,
html.dark .automation-stage::before,
:root.dark .automation-stage::after,
[data-theme='dark'] .automation-stage::after,
html.dark .automation-stage::after {
  opacity: 0.46;
  background-image: none;
}

:root.dark .automation-surface-panel,
[data-theme='dark'] .automation-surface-panel,
html.dark .automation-surface-panel {
  border-color: rgba(71, 85, 105, 0.58);
  background: #1e293b;
}

:root.dark .automation-surface-card,
[data-theme='dark'] .automation-surface-card,
html.dark .automation-surface-card,
:root.dark .automation-handler-card,
[data-theme='dark'] .automation-handler-card,
html.dark .automation-handler-card {
  border-color: rgba(71, 85, 105, 0.46);
  background: #111827;
}

:root.dark .automation-subsurface-card,
[data-theme='dark'] .automation-subsurface-card,
html.dark .automation-subsurface-card,
:root.dark .automation-refresh-button,
[data-theme='dark'] .automation-refresh-button,
html.dark .automation-refresh-button,
:root.dark .automation-action-button,
[data-theme='dark'] .automation-action-button,
html.dark .automation-action-button,
:root.dark .automation-secondary-button,
[data-theme='dark'] .automation-secondary-button,
html.dark .automation-secondary-button,
:root.dark .automation-chip-button,
[data-theme='dark'] .automation-chip-button,
html.dark .automation-chip-button,
:root.dark .automation-disclosure-button,
[data-theme='dark'] .automation-disclosure-button,
html.dark .automation-disclosure-button,
:root.dark .automation-input,
[data-theme='dark'] .automation-input,
html.dark .automation-input,
:root.dark .automation-handler-panel,
[data-theme='dark'] .automation-handler-panel,
html.dark .automation-handler-panel,
:root.dark .automation-disclosure-panel,
[data-theme='dark'] .automation-disclosure-panel,
html.dark .automation-disclosure-panel {
  border-color: rgba(71, 85, 105, 0.5);
  background: rgba(15, 23, 42, 0.72);
  box-shadow: none;
}

:root.dark .automation-meta-label,
[data-theme='dark'] .automation-meta-label,
html.dark .automation-meta-label,
:root.dark .automation-execution-date,
[data-theme='dark'] .automation-execution-date,
html.dark .automation-execution-date,
:root.dark .automation-label,
[data-theme='dark'] .automation-label,
html.dark .automation-label {
  color: rgb(148 163 184);
}

:root.dark .automation-meta-value,
[data-theme='dark'] .automation-meta-value,
html.dark .automation-meta-value,
:root.dark .automation-count-chip,
[data-theme='dark'] .automation-count-chip,
html.dark .automation-count-chip,
:root.dark .automation-execution-meta,
[data-theme='dark'] .automation-execution-meta,
html.dark .automation-execution-meta,
:root.dark .automation-secondary-button,
[data-theme='dark'] .automation-secondary-button,
html.dark .automation-secondary-button,
:root.dark .automation-input,
[data-theme='dark'] .automation-input,
html.dark .automation-input {
  color: rgb(226 232 240);
}

:root.dark .automation-refresh-button,
[data-theme='dark'] .automation-refresh-button,
html.dark .automation-refresh-button,
:root.dark .automation-action-button,
[data-theme='dark'] .automation-action-button,
html.dark .automation-action-button,
:root.dark .automation-secondary-button:hover,
[data-theme='dark'] .automation-secondary-button:hover,
html.dark .automation-secondary-button:hover,
:root.dark .automation-chip-button:hover,
[data-theme='dark'] .automation-chip-button:hover,
html.dark .automation-chip-button:hover {
  background: #1f2937;
  color: rgb(226 232 240);
}

:root.dark .automation-refresh-button:hover:not(:disabled),
[data-theme='dark'] .automation-refresh-button:hover:not(:disabled),
html.dark .automation-refresh-button:hover:not(:disabled),
:root.dark .automation-action-button:hover,
[data-theme='dark'] .automation-action-button:hover,
html.dark .automation-action-button:hover,
:root.dark .automation-disclosure-button:hover,
[data-theme='dark'] .automation-disclosure-button:hover,
html.dark .automation-disclosure-button:hover {
  border-color: rgba(96, 165, 250, 0.28);
}

:root.dark .automation-action-button--primary,
[data-theme='dark'] .automation-action-button--primary,
html.dark .automation-action-button--primary {
  border-color: #e2e8f0;
  background: #e2e8f0;
  color: #0f172a;
}

:root.dark .automation-action-button--warning,
[data-theme='dark'] .automation-action-button--warning,
html.dark .automation-action-button--warning {
  border-color: rgba(245, 158, 11, 0.28);
  background: rgba(245, 158, 11, 0.16);
  color: rgb(253 186 116);
}

:root.dark .automation-action-button--success,
[data-theme='dark'] .automation-action-button--success,
html.dark .automation-action-button--success {
  border-color: rgba(16, 185, 129, 0.28);
  background: rgba(16, 185, 129, 0.16);
  color: rgb(110 231 183);
}

:root.dark .automation-action-button--danger,
[data-theme='dark'] .automation-action-button--danger,
html.dark .automation-action-button--danger {
  border-color: rgba(239, 68, 68, 0.28);
  background: rgba(239, 68, 68, 0.16);
  color: rgb(252 165 165);
}

:root.dark .automation-handler-card--active,
[data-theme='dark'] .automation-handler-card--active,
html.dark .automation-handler-card--active {
  border-color: rgba(147, 197, 253, 0.45);
  background: rgba(37, 99, 235, 0.18);
}

:root.dark .automation-count-chip,
[data-theme='dark'] .automation-count-chip,
html.dark .automation-count-chip {
  border-color: rgba(147, 197, 253, 0.45);
  background: rgba(30, 64, 175, 0.25);
  color: rgb(191 219 254);
}

:root.dark .automation-error-surface,
[data-theme='dark'] .automation-error-surface,
html.dark .automation-error-surface {
  background: rgba(127, 29, 29, 0.32);
  color: rgb(252 165 165);
}

:root.dark .automation-modal-backdrop,
[data-theme='dark'] .automation-modal-backdrop,
html.dark .automation-modal-backdrop {
  background: rgba(2, 6, 23, 0.64);
}

:root.dark .automation-modal-header,
[data-theme='dark'] .automation-modal-header,
html.dark .automation-modal-header,
:root.dark .automation-modal-footer,
[data-theme='dark'] .automation-modal-footer,
html.dark .automation-modal-footer {
  border-color: rgba(255, 255, 255, 0.08);
}

@media (min-width: 760px) {
  .automation-job-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .automation-job-meta-grid {
    grid-template-columns: minmax(0, 1.25fr) repeat(2, minmax(0, 1fr));
  }

  .automation-form-grid--compact {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 820px) {
  .automation-hero,
  .automation-job-footer,
  .automation-list-header,
  .automation-form-actions {
    flex-direction: column;
  }

  .automation-job-actions {
    width: 100%;
    justify-content: flex-start;
  }

  .automation-library-card__aside {
    align-items: flex-start;
  }

  .automation-library-card__topline {
    justify-content: flex-start;
  }

  .automation-library-card__controls {
    justify-content: flex-start;
  }

  .automation-handler-grid {
    grid-template-columns: repeat(1, minmax(0, 1fr));
  }
}

@media (max-width: 768px) {
  .cron-page {
    padding-inline: 0.75rem;
    padding-bottom: 1rem;
  }

  .automation-stage {
    padding-top: 0.7rem;
  }

  .automation-library-card,
  .automation-state-card,
  .automation-panel-card,
  .automation-modal-header,
  .automation-modal-body,
  .automation-modal-footer,
  .automation-modal-scroll {
    padding-inline: 0.92rem;
  }
}
</style>
