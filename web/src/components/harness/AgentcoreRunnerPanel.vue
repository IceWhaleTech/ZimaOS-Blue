<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { settingsApi, type AgentcoreRunnerTagList } from '@/api/settings'
import { useSettingsStore } from '@/stores/settings'

const props = withDefaults(
  defineProps<{
    showRefreshButton?: boolean
    embedded?: boolean
  }>(),
  {
    showRefreshButton: true,
    embedded: false,
  }
)

const emit = defineEmits<{
  (
    e: 'status-change',
    payload: {
      message: string
      tone: 'success' | 'error'
    }
  ): void
}>()

const { t } = useI18n()
const settingsStore = useSettingsStore()

const AGENTCORE_RUNNER_EVOLVABLE_PART_ORDER = [
  'constraints',
  'skill_definition',
  'prompt_template',
  'context_assembly',
  'coordinator_policy',
  'orchestrator_policy',
  'tool_exposure',
  'verification_policy',
  'runner_code',
  'build_recipe',
] as const

const AGENTCORE_RUNNER_EVOLVABLE_PART_FALLBACK_LABELS: Record<string, string> = {
  constraints: 'Constraints',
  skill_definition: 'Skill definition',
  prompt_template: 'Prompt template',
  context_assembly: 'Context assembly',
  coordinator_policy: 'Coordinator',
  orchestrator_policy: 'Orchestrator',
  tool_exposure: 'Tool exposure',
  verification_policy: 'Verification policy',
  runner_code: 'Runner code',
  build_recipe: 'Build recipe',
}

const AGENTCORE_RUNNER_EVOLVABLE_PART_FALLBACK_DESCRIPTIONS: Record<string, string> = {
  constraints: 'Defines the hard limits and guardrails the runner must follow.',
  skill_definition: 'Describes the skill contract, responsibilities, and expected capabilities.',
  prompt_template:
    'Shapes the reusable instructions and response structure sent to the model.',
  context_assembly:
    'Controls how evidence, state, and workspace context are gathered before each run.',
  coordinator_policy:
    'Decides how top-level tasks are broken down, sequenced, and handed off.',
  orchestrator_policy:
    'Governs multi-step flow control, retries, and cross-stage coordination.',
  tool_exposure:
    'Chooses which tools are available to the runner and how they are presented.',
  verification_policy:
    'Defines how outputs are checked before they are accepted or persisted.',
  runner_code: 'Implements the runtime logic that executes the agent loop and integrations.',
  build_recipe: 'Specifies how the runner is prepared, built, and packaged for execution.',
}

const AGENTCORE_RUNNER_EVOLVABLE_PART_ACTIVE_TOOLTIP_FALLBACK =
  'This part was optimized in the current candidate'
const AGENTCORE_RUNNER_EVOLVABLE_PART_INACTIVE_TOOLTIP_FALLBACK =
  'This part can participate in self-evolution, but the current version did not change it'

const agentcoreRunnerSaving = ref(false)
const agentcoreRunnerPreparing = ref(false)
const agentcoreRunnerRefreshing = ref(false)
const agentcoreRunnerSourceExpanded = ref(false)
const agentcoreRunnerStatusExpanded = ref(false)
const agentcoreRunnerRepoURL = ref('')
const agentcoreRunnerRef = ref('')
const agentcoreRunnerTags = ref<AgentcoreRunnerTagList | null>(null)
const agentcoreRunnerTagsLoading = ref(false)
const agentcoreRunnerTranscriptExpanded = ref(false)
const agentcoreRunnerTranscriptPreviewCount = 2
const agentcoreRunnerRootClass = computed(() =>
  props.embedded
    ? 'agentcore-runner-panel agentcore-runner-panel--embedded'
    : 'agentcore-runner-panel dashboard-card-surface'
)
const agentcoreRunnerBodyClass = computed(() =>
  props.embedded
    ? 'agentcore-runner-panel__body agentcore-runner-panel__body--embedded space-y-4'
    : 'dashboard-card-subsurface agentcore-runner-panel__body p-4 space-y-4'
)
const agentcoreRunnerSectionCardClass = computed(() =>
  props.embedded
    ? 'rounded-2xl bg-slate-50/85 p-4 ring-1 ring-slate-200/80 dark:bg-slate-900/55 dark:ring-slate-800/80'
    : 'rounded-xl border border-gray-200 bg-gray-50 p-4 dark:border-gray-700 dark:bg-slate-900/60'
)
const agentcoreRunnerLastRunCardClass = computed(() =>
  props.embedded
    ? 'mt-2 space-y-2 rounded-xl bg-white/90 p-3 ring-1 ring-slate-200/80 dark:bg-slate-950/60 dark:ring-slate-800/80'
    : 'mt-2 space-y-2 rounded-lg border border-gray-200 bg-white/80 p-3 dark:border-gray-700 dark:bg-slate-950/50'
)

const agentcoreRunnerStatus = computed(() => settingsStore.agentcoreRunnerStatus)
const agentcoreRunnerLastRun = computed(() => settingsStore.agentcoreRunnerLastRun)
const agentcoreRunnerEnabled = computed(() => settingsStore.experimentalAgentcoreRunnerEnabled)
const agentcoreRunnerLastError = computed(() =>
  normalizeAgentcoreRunnerStatusError(agentcoreRunnerStatus.value?.last_error)
)
const agentcoreRunnerHasLastError = computed(() => agentcoreRunnerLastError.value !== '')
const agentcoreRunnerRefOptions = computed(() => {
  const options: string[] = []
  const seen = new Set<string>()
  const push = (value: unknown) => {
    const normalized = normalizeAgentcoreRunnerRefValue(value)
    if (seen.has(normalized)) return
    seen.add(normalized)
    options.push(normalized)
  }
  push(agentcoreRunnerTags.value?.default_ref)
  push(agentcoreRunnerRef.value)
  for (const tag of agentcoreRunnerTags.value?.tags ?? []) {
    push(tag)
  }
  return options
})
const agentcoreRunnerBusy = computed(
  () =>
    agentcoreRunnerSaving.value ||
    agentcoreRunnerPreparing.value ||
    agentcoreRunnerRefreshing.value
)
const agentcoreRunnerSourceRepoSummary = computed(() =>
  formatAgentcoreRunnerRepoSummary(agentcoreRunnerRepoURL.value)
)
const agentcoreRunnerSourceRefSummary = computed(() =>
  normalizeAgentcoreRunnerRefValue(agentcoreRunnerRef.value)
)
const agentcoreRunnerLastRunMeta = computed(() => {
  const run = agentcoreRunnerLastRun.value
  if (run == null) return [] as string[]
  return [
    normalizeEvidenceText(run.reason),
    normalizeEvidenceText(run.candidate_id),
    normalizeEvidenceText(run.eval_run_id),
    typeof run.runner_protocol === 'string' ? run.runner_protocol.trim() : '',
    typeof run.runner_stop_reason === 'string' ? run.runner_stop_reason.trim() : '',
    typeof run.optimization_surface === 'string' ? run.optimization_surface.trim() : '',
    formatDurationMs(run.runner_duration_ms),
  ].filter(Boolean)
})
const agentcoreRunnerLastRunTranscriptEntries = computed(() => {
  const entries = agentcoreRunnerLastRun.value?.runner_transcript ?? []
  const suppressed = new Set(
    [
      normalizeEvidenceText(agentcoreRunnerLastRun.value?.runner_response_text),
      normalizeEvidenceText(agentcoreRunnerLastRun.value?.runner_error),
    ].filter(Boolean)
  )
  const seen = new Set<string>()
  return entries.filter((entry) => {
    const text = normalizeEvidenceText(entry.text)
    if (!text) return false
    if (suppressed.has(text)) return false
    const fingerprint = [
      normalizeEvidenceText(entry.direction),
      normalizeEvidenceText(entry.method),
      text,
    ].join('|')
    if (seen.has(fingerprint)) return false
    seen.add(fingerprint)
    return true
  })
})
const agentcoreRunnerVisibleTranscriptEntries = computed(() => {
  if (agentcoreRunnerTranscriptExpanded.value) {
    return agentcoreRunnerLastRunTranscriptEntries.value
  }
  return agentcoreRunnerLastRunTranscriptEntries.value.slice(0, agentcoreRunnerTranscriptPreviewCount)
})
const agentcoreRunnerHiddenTranscriptCount = computed(() =>
  Math.max(
    0,
    agentcoreRunnerLastRunTranscriptEntries.value.length - agentcoreRunnerTranscriptPreviewCount
  )
)
const agentcoreRunnerSupportedParts = computed(() => {
  const configured = new Set(
    (agentcoreRunnerStatus.value?.supported_parts ?? [])
      .map((part) => normalizeEvidenceText(part))
      .filter(Boolean)
  )
  if (configured.size === 0) {
    return [...AGENTCORE_RUNNER_EVOLVABLE_PART_ORDER]
  }
  return AGENTCORE_RUNNER_EVOLVABLE_PART_ORDER.filter((part) => configured.has(part))
})
const agentcoreRunnerOptimizedParts = computed(() => {
  const parts = (agentcoreRunnerStatus.value?.optimized_parts ?? [])
  return new Set(parts.map((part) => normalizeEvidenceText(part)).filter(Boolean))
})
const agentcoreRunnerOptimizedPartList = computed(() =>
  agentcoreRunnerSupportedParts.value.filter((part) => agentcoreRunnerOptimizedParts.value.has(part))
)
const agentcoreRunnerOptimizedSummary = computed(() =>
  agentcoreRunnerOptimizedPartList.value.map((part) => translateAgentcoreRunnerPart(part)).join(', ')
)
const agentcoreRunnerPrimaryPart = computed(() =>
  normalizeEvidenceText(agentcoreRunnerStatus.value?.primary_part)
)
const agentcoreRunnerPrimaryPartLabel = computed(() =>
  agentcoreRunnerPrimaryPart.value ? translateAgentcoreRunnerPart(agentcoreRunnerPrimaryPart.value) : ''
)
const agentcoreRunnerSourceOptimizationRunID = computed(() =>
  normalizeEvidenceText(agentcoreRunnerStatus.value?.source_optimization_run_id)
)
const agentcoreRunnerEvolvablePartBadges = computed(() =>
  agentcoreRunnerSupportedParts.value.map((part) => ({
    part,
    label: translateAgentcoreRunnerPart(part),
    active: agentcoreRunnerOptimizedParts.value.has(part),
    tooltip: buildAgentcoreRunnerPartTooltip(part, agentcoreRunnerOptimizedParts.value.has(part)),
  }))
)

watch(
  [
    () => settingsStore.experimentalAgentcoreRunnerRepoURL,
    () => settingsStore.experimentalAgentcoreRunnerRef,
  ],
  ([repoURL, refValue]) => {
    agentcoreRunnerRepoURL.value = repoURL
    agentcoreRunnerRef.value = normalizeAgentcoreRunnerRefValue(refValue)
  },
  { immediate: true }
)

watch(
  () => agentcoreRunnerLastRun.value?.id,
  () => {
    agentcoreRunnerTranscriptExpanded.value = false
  }
)

onMounted(() => {
  void ensureLoaded()
})

function emitStatus(message: string, tone: 'success' | 'error') {
  emit('status-change', { message, tone })
}

function formatStatusTime(value?: string) {
  if (!value) return t('settings.agentcoreRunner.empty', 'Not available')
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

function formatDurationMs(value?: number) {
  if (typeof value !== 'number' || !Number.isFinite(value) || value < 0) return ''
  if (value < 1000) return `${Math.round(value)}ms`
  return `${(value / 1000).toFixed(1)}s`
}

function normalizeEvidenceText(value: unknown) {
  if (typeof value !== 'string') return ''
  return value.trim()
}

function normalizeAgentcoreRunnerStatusError(value: unknown) {
  const text = normalizeEvidenceText(value)
  if (text.toLowerCase() === 'repo url is required') {
    return ''
  }
  return text
}

function translateAgentcoreRunnerPart(part: string) {
  return t(
    `settings.agentcoreRunner.parts.${part}`,
    AGENTCORE_RUNNER_EVOLVABLE_PART_FALLBACK_LABELS[part] ?? part
  )
}

function translateAgentcoreRunnerPartDescription(part: string) {
  return t(
    `settings.agentcoreRunner.partDescriptions.${part}`,
    AGENTCORE_RUNNER_EVOLVABLE_PART_FALLBACK_DESCRIPTIONS[part] ?? ''
  )
}

function buildAgentcoreRunnerPartTooltip(part: string, active: boolean) {
  const statusText = active
    ? t(
        'settings.agentcoreRunner.activePartTooltip',
        AGENTCORE_RUNNER_EVOLVABLE_PART_ACTIVE_TOOLTIP_FALLBACK
      )
    : t(
        'settings.agentcoreRunner.inactivePartTooltip',
        AGENTCORE_RUNNER_EVOLVABLE_PART_INACTIVE_TOOLTIP_FALLBACK
      )

  return [
    translateAgentcoreRunnerPart(part),
    translateAgentcoreRunnerPartDescription(part),
    statusText,
  ]
    .filter(Boolean)
    .join('\n')
}

function normalizeAgentcoreRunnerRepoURLValue(value: unknown) {
  const text = normalizeEvidenceText(value)
  if (text) return text
  return 'https://github.com/IceWhaleTech/ZimaOS-Blue'
}

function normalizeAgentcoreRunnerRefValue(value: unknown) {
  const text = normalizeEvidenceText(value)
  if (text) return text
  return 'main'
}

function formatAgentcoreRunnerRepoSummary(value: unknown) {
  const repoURL = normalizeAgentcoreRunnerRepoURLValue(value)
  return repoURL
    .replace(/^https?:\/\/github\.com\//i, '')
    .replace(/\.git$/i, '')
    .replace(/\/+$/, '')
}

async function fetchAgentcoreRunnerStatus() {
  try {
    await settingsStore.fetchAgentcoreRunnerStatus()
  } catch {
    // Ignore card bootstrap errors and keep the panel interactive.
  }
}

async function fetchAgentcoreRunnerLastRun() {
  try {
    await settingsStore.fetchAgentcoreRunnerLastRun()
  } catch {
    // Ignore card bootstrap errors and keep the panel interactive.
  }
}

async function fetchAgentcoreRunnerTags(repoURL = agentcoreRunnerRepoURL.value) {
  const resolvedRepoURL = normalizeAgentcoreRunnerRepoURLValue(repoURL)
  try {
    agentcoreRunnerTagsLoading.value = true
    const response = await settingsApi.getAgentcoreRunnerTags(resolvedRepoURL)
    agentcoreRunnerTags.value = response.data
  } catch {
    agentcoreRunnerTags.value = {
      repo_url: resolvedRepoURL,
      default_ref: 'main',
      tags: [],
    }
  } finally {
    agentcoreRunnerTagsLoading.value = false
  }
}

async function ensureLoaded() {
  const tasks: Promise<unknown>[] = []
  if (settingsStore.agentcoreRunnerStatus == null) {
    tasks.push(fetchAgentcoreRunnerStatus())
  } else if (
    settingsStore.agentcoreRunnerStatus.last_optimization_run_id &&
    settingsStore.agentcoreRunnerLastRun == null
  ) {
    tasks.push(fetchAgentcoreRunnerLastRun())
  }
  tasks.push(fetchAgentcoreRunnerTags())
  await Promise.allSettled(tasks)
}

async function refreshAgentcoreRunnerStatus() {
  if (agentcoreRunnerRefreshing.value) return
  try {
    agentcoreRunnerRefreshing.value = true
    await Promise.allSettled([fetchAgentcoreRunnerStatus(), fetchAgentcoreRunnerTags()])
  } finally {
    agentcoreRunnerRefreshing.value = false
  }
}

async function saveAgentcoreRunnerConfig() {
  if (agentcoreRunnerSaving.value) return
  const repoURL = normalizeAgentcoreRunnerRepoURLValue(agentcoreRunnerRepoURL.value)
  const refValue = normalizeAgentcoreRunnerRefValue(agentcoreRunnerRef.value)
  agentcoreRunnerRepoURL.value = repoURL
  agentcoreRunnerRef.value = refValue
  try {
    agentcoreRunnerSaving.value = true
    await settingsStore.updateBackendSettings({
      experimental_agentcore_runner_repo_url: repoURL,
      experimental_agentcore_runner_ref: refValue,
    })
    emitStatus(t('settings.saved', 'Saved'), 'success')
    await Promise.allSettled([fetchAgentcoreRunnerStatus(), fetchAgentcoreRunnerTags(repoURL)])
  } catch {
    emitStatus(t('settings.saveFailed', 'Failed to save configuration'), 'error')
  } finally {
    agentcoreRunnerSaving.value = false
  }
}

async function handleAgentcoreRunnerEnabledChange(next: boolean) {
  if (agentcoreRunnerSaving.value) return
  try {
    agentcoreRunnerSaving.value = true
    await settingsStore.setExperimentalAgentcoreRunnerEnabled(next)
    emitStatus(t('settings.saved', 'Saved'), 'success')
    await fetchAgentcoreRunnerStatus()
  } catch {
    emitStatus(t('settings.saveFailed', 'Failed to save configuration'), 'error')
  } finally {
    agentcoreRunnerSaving.value = false
  }
}

async function prepareAgentcoreRunner() {
  if (agentcoreRunnerPreparing.value) return
  const repoURL = normalizeAgentcoreRunnerRepoURLValue(agentcoreRunnerRepoURL.value)
  const refValue = normalizeAgentcoreRunnerRefValue(agentcoreRunnerRef.value)
  agentcoreRunnerRepoURL.value = repoURL
  agentcoreRunnerRef.value = refValue
  try {
    agentcoreRunnerPreparing.value = true
    await settingsStore.updateBackendSettings({
      experimental_agentcore_runner_repo_url: repoURL,
      experimental_agentcore_runner_ref: refValue,
    })
    await settingsStore.prepareAgentcoreRunner()
    await Promise.allSettled([fetchAgentcoreRunnerStatus(), fetchAgentcoreRunnerTags(repoURL)])
    emitStatus(
      t('settings.agentcoreRunner.prepareSuccess', 'Agentcore Runner prepared successfully'),
      'success'
    )
  } catch {
    emitStatus(
      t('settings.agentcoreRunner.prepareFailed', 'Failed to prepare Agentcore Runner'),
      'error'
    )
  } finally {
    agentcoreRunnerPreparing.value = false
  }
}
</script>

<template>
  <section :class="agentcoreRunnerRootClass" data-testid="agentcore-runner-card">
    <div v-if="!embedded" class="space-y-1.5">
      <span class="settings-module__eyebrow agentcore-runner-panel__eyebrow inline-flex w-fit">{{
        t('settings.agentcoreRunner.eyebrow', 'Harness · Beta')
      }}</span>
      <h2 class="settings-module__title agentcore-runner-panel__title">
        {{
          t(
            'settings.agentcoreRunner.title',
            'Harness Self-Iterating Agentcore Runner'
          )
        }}
      </h2>
      <p class="text-sm text-gray-500 dark:text-gray-400">
        {{
          t(
            'settings.agentcoreRunner.description',
            'Use Harness to iterate on an Agentcore runner by preparing a standalone runner from a public GitHub repo for local build, evaluation, and optimisation.'
          )
        }}
      </p>
    </div>

    <div :class="agentcoreRunnerBodyClass">
      <div class="settings-field-card__row flex items-start justify-between gap-3">
        <div class="settings-card-heading min-w-0">
          <label class="settings-field-label block font-medium text-gray-900 dark:text-gray-100">{{
            t('settings.agentcoreRunner.enabled', 'Enable Agentcore Runner')
          }}</label>
          <p class="settings-field-hint mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{
              t(
                'settings.agentcoreRunner.enabledHint',
                'Allow Harness beta flows to prepare and reuse a managed local runner for self-iteration.'
              )
            }}
          </p>
        </div>
        <button
          data-testid="agentcore-runner-enabled-switch"
          type="button"
          role="switch"
          :aria-checked="agentcoreRunnerEnabled"
          :disabled="agentcoreRunnerBusy"
          class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-gray-400 focus:ring-offset-2 disabled:opacity-50"
          :class="
            agentcoreRunnerEnabled
              ? 'bg-green-600 dark:bg-green-500'
              : 'bg-gray-300 dark:bg-gray-600'
          "
          @click="handleAgentcoreRunnerEnabledChange(!agentcoreRunnerEnabled)"
        >
          <span
            class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
            :class="agentcoreRunnerEnabled ? 'translate-x-5' : 'translate-x-0'"
          />
        </button>
      </div>

      <div
        :class="agentcoreRunnerSectionCardClass"
      >
        <button
          data-testid="agentcore-runner-source-toggle"
          type="button"
          :aria-expanded="agentcoreRunnerSourceExpanded ? 'true' : 'false'"
          class="flex w-full items-center justify-between gap-3 text-left"
          @click="agentcoreRunnerSourceExpanded = !agentcoreRunnerSourceExpanded"
        >
          <div class="min-w-0">
            <div class="text-sm font-medium text-gray-800 dark:text-gray-100">
              {{ t('settings.agentcoreRunner.source', 'GitHub Repo & Ref') }}
            </div>
            <div class="mt-2 flex flex-wrap gap-1.5">
              <span
                class="max-w-full break-all rounded-lg border border-blue-200 bg-blue-50 px-2.5 py-1 text-[11px] font-medium text-blue-700 dark:border-blue-800 dark:bg-blue-950/40 dark:text-blue-200"
              >
                {{ agentcoreRunnerSourceRepoSummary }}
              </span>
              <span
                class="rounded-full border border-gray-200 bg-white px-2.5 py-1 text-[11px] font-medium text-gray-700 dark:border-gray-700 dark:bg-slate-950/60 dark:text-gray-200"
              >
                {{ agentcoreRunnerSourceRefSummary }}
              </span>
            </div>
          </div>
          <span class="shrink-0 text-xs text-gray-500 dark:text-gray-400">{{
            agentcoreRunnerSourceExpanded
              ? t('settings.smallModel.collapse', 'Collapse')
              : t('settings.smallModel.expand', 'Expand')
          }}</span>
        </button>

        <div
          v-if="agentcoreRunnerSourceExpanded"
          data-testid="agentcore-runner-source-content"
          class="mt-3"
        >
          <div class="grid gap-3 md:grid-cols-[minmax(0,1fr),180px]">
            <label class="block">
              <span class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-200">
                {{ t('settings.agentcoreRunner.repoUrl', 'GitHub Repo URL') }}
              </span>
              <input
                data-testid="agentcore-runner-repo-input"
                v-model="agentcoreRunnerRepoURL"
                type="text"
                class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 outline-none transition focus:border-green-500 dark:border-gray-700 dark:bg-slate-900 dark:text-gray-100"
                :placeholder="
                  t(
                    'settings.agentcoreRunner.repoPlaceholder',
                    'https://github.com/owner/repo or owner/repo'
                  )
                "
                @blur="saveAgentcoreRunnerConfig"
              />
            </label>

            <label class="block">
              <span class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-200">
                {{ t('settings.agentcoreRunner.ref', 'Ref') }}
              </span>
              <select
                data-testid="agentcore-runner-ref-input"
                v-model="agentcoreRunnerRef"
                class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 outline-none transition focus:border-green-500 dark:border-gray-700 dark:bg-slate-900 dark:text-gray-100"
                :disabled="agentcoreRunnerSaving"
                @change="saveAgentcoreRunnerConfig"
              >
                <option v-for="option in agentcoreRunnerRefOptions" :key="option" :value="option">
                  {{ option }}
                </option>
              </select>
              <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                {{
                  agentcoreRunnerTagsLoading
                    ? t('settings.agentcoreRunner.refLoading', 'Loading tags...')
                    : t(
                        'settings.agentcoreRunner.refHint',
                        'Defaults to main and lists tags from the selected repo.'
                      )
                }}
              </p>
            </label>
          </div>
        </div>
      </div>

      <div class="agentcore-runner-panel__actions flex items-center justify-between gap-3">
        <p class="text-xs text-gray-500 dark:text-gray-400">
          {{
            t(
              'settings.agentcoreRunner.prepareHint',
              'Prepare downloads the repo, installs the required Go toolchain, and builds ./cmd/agentcore-runner in the managed cache.'
            )
          }}
        </p>
        <div class="flex items-center gap-2">
          <button
            v-if="showRefreshButton"
            data-testid="agentcore-runner-refresh"
            type="button"
            class="rounded-lg border border-gray-200 px-3 py-2 text-sm font-medium text-gray-700 transition hover:bg-gray-100 disabled:cursor-not-allowed disabled:opacity-60 dark:border-gray-700 dark:text-gray-200 dark:hover:bg-slate-800"
            :disabled="agentcoreRunnerBusy"
            @click="refreshAgentcoreRunnerStatus"
          >
            {{ t('common.refresh', 'Refresh') }}
          </button>
          <button
            data-testid="agentcore-runner-prepare"
            type="button"
            class="rounded-lg bg-green-600 px-3 py-2 text-sm font-medium text-white transition hover:bg-green-500 disabled:cursor-not-allowed disabled:opacity-60"
            :disabled="agentcoreRunnerBusy"
            @click="prepareAgentcoreRunner"
          >
            {{
              agentcoreRunnerPreparing
                ? t('settings.agentcoreRunner.preparing', 'Preparing...')
                : t('settings.agentcoreRunner.prepare', 'Prepare Runner')
            }}
          </button>
        </div>
      </div>

      <div :class="agentcoreRunnerSectionCardClass">
        <button
          data-testid="agentcore-runner-status-toggle"
          type="button"
          :aria-expanded="agentcoreRunnerStatusExpanded ? 'true' : 'false'"
          class="flex w-full items-center justify-between gap-3 text-left"
          @click="agentcoreRunnerStatusExpanded = !agentcoreRunnerStatusExpanded"
        >
          <div class="text-sm font-medium text-gray-800 dark:text-gray-100">
            {{ t('settings.agentcoreRunner.status', 'Status') }}
          </div>
          <span class="text-xs text-gray-500 dark:text-gray-400">{{
            agentcoreRunnerStatusExpanded
              ? t('settings.smallModel.collapse', 'Collapse')
              : t('settings.smallModel.expand', 'Expand')
          }}</span>
        </button>
        <div
          v-if="agentcoreRunnerStatusExpanded"
          data-testid="agentcore-runner-status-content"
          class="mt-3 grid gap-2 sm:grid-cols-2"
        >
          <div class="text-sm">
            <span class="text-gray-500 dark:text-gray-400">{{
              t('settings.agentcoreRunner.resolvedCommit', 'Resolved commit')
            }}</span>
            <div data-testid="agentcore-runner-resolved-commit" class="break-all text-gray-900 dark:text-gray-100">
              {{
                agentcoreRunnerStatus?.resolved_commit ||
                t('settings.agentcoreRunner.empty', 'Not available')
              }}
            </div>
          </div>
          <div class="text-sm">
            <span class="text-gray-500 dark:text-gray-400">{{
              t('settings.agentcoreRunner.requiredGoVersion', 'Required Go version')
            }}</span>
            <div data-testid="agentcore-runner-required-go" class="text-gray-900 dark:text-gray-100">
              {{
                agentcoreRunnerStatus?.required_go_version ||
                t('settings.agentcoreRunner.empty', 'Not available')
              }}
            </div>
          </div>
          <div class="text-sm">
            <span class="text-gray-500 dark:text-gray-400">{{
              t('settings.agentcoreRunner.installedGoVersion', 'Installed Go version')
            }}</span>
            <div data-testid="agentcore-runner-installed-go" class="text-gray-900 dark:text-gray-100">
              {{
                agentcoreRunnerStatus?.installed_go_version ||
                t('settings.agentcoreRunner.empty', 'Not available')
              }}
            </div>
          </div>
          <div class="text-sm">
            <span class="text-gray-500 dark:text-gray-400">{{
              t('settings.agentcoreRunner.toolchainReady', 'Toolchain ready')
            }}</span>
            <div data-testid="agentcore-runner-toolchain-ready" class="text-gray-900 dark:text-gray-100">
              {{
                agentcoreRunnerStatus?.toolchain_ready
                  ? t('common.yes', 'Yes')
                  : t('common.no', 'No')
              }}
            </div>
          </div>
          <div class="text-sm">
            <span class="text-gray-500 dark:text-gray-400">{{
              t('settings.agentcoreRunner.binaryReady', 'Binary ready')
            }}</span>
            <div data-testid="agentcore-runner-binary-ready" class="text-gray-900 dark:text-gray-100">
              {{
                agentcoreRunnerStatus?.binary_ready
                  ? t('common.yes', 'Yes')
                  : t('common.no', 'No')
              }}
            </div>
          </div>
          <div class="text-sm">
            <span class="text-gray-500 dark:text-gray-400">{{
              t('settings.agentcoreRunner.lastPrepareState', 'Last prepare state')
            }}</span>
            <div data-testid="agentcore-runner-last-prepare-state" class="text-gray-900 dark:text-gray-100">
              {{
                agentcoreRunnerStatus?.last_prepare_state ||
                t('settings.agentcoreRunner.empty', 'Not available')
              }}
            </div>
          </div>
          <div class="text-sm sm:col-span-2">
            <span class="text-gray-500 dark:text-gray-400">{{
              t('settings.agentcoreRunner.binaryPath', 'Binary path')
            }}</span>
            <div data-testid="agentcore-runner-binary-path" class="break-all text-gray-900 dark:text-gray-100">
              {{
                agentcoreRunnerStatus?.binary_path ||
                t('settings.agentcoreRunner.empty', 'Not available')
              }}
            </div>
          </div>
          <div class="text-sm sm:col-span-2">
            <span class="text-gray-500 dark:text-gray-400">{{
              t('settings.agentcoreRunner.binaryChecksum', 'Binary checksum')
            }}</span>
            <div
              data-testid="agentcore-runner-binary-checksum"
              class="break-all text-gray-900 dark:text-gray-100"
            >
              {{
                agentcoreRunnerStatus?.binary_sha256 ||
                t('settings.agentcoreRunner.empty', 'Not available')
              }}
            </div>
          </div>
          <div class="text-sm">
            <span class="text-gray-500 dark:text-gray-400">{{
              t('settings.agentcoreRunner.lastPrepareAt', 'Last prepare time')
            }}</span>
            <div class="text-gray-900 dark:text-gray-100">
              {{ formatStatusTime(agentcoreRunnerStatus?.last_prepare_at) }}
            </div>
          </div>
          <div class="text-sm sm:col-span-2">
            <span class="text-gray-500 dark:text-gray-400">{{
              t('settings.agentcoreRunner.evolvableParts', 'Evolvable parts')
            }}</span>
            <div class="mt-2 flex flex-wrap gap-2">
              <span
                v-for="badge in agentcoreRunnerEvolvablePartBadges"
                :key="badge.part"
                data-testid="agentcore-runner-evolvable-part"
                :data-part="badge.part"
                :data-active="badge.active ? 'true' : 'false'"
                :title="badge.tooltip"
                class="rounded-full border px-2.5 py-1 text-[11px] font-medium transition"
                :class="
                  badge.active
                    ? 'border-emerald-300 bg-emerald-50 text-emerald-700 dark:border-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-200'
                    : 'border-gray-200 bg-gray-100 text-gray-500 dark:border-gray-700 dark:bg-slate-900 dark:text-gray-400'
                "
              >
                {{ badge.label }}
              </span>
            </div>
            <div
              v-if="agentcoreRunnerOptimizedSummary"
              data-testid="agentcore-runner-optimized-summary"
              class="mt-2 text-xs text-gray-700 dark:text-gray-200"
            >
              {{ agentcoreRunnerOptimizedSummary }}
            </div>
          </div>
          <div class="text-sm">
            <span class="text-gray-500 dark:text-gray-400">{{
              t('settings.agentcoreRunner.lastOptimizationRunId', 'Last optimization run ID')
            }}</span>
            <div
              data-testid="agentcore-runner-last-optimization-run-id"
              class="break-all text-gray-900 dark:text-gray-100"
            >
              {{
                agentcoreRunnerStatus?.last_optimization_run_id ||
                t('settings.agentcoreRunner.empty', 'Not available')
              }}
            </div>
            <div
              data-testid="agentcore-runner-last-optimization-state"
              class="mt-1 text-xs text-gray-600 dark:text-gray-300"
            >
              {{
                agentcoreRunnerStatus?.last_optimization_state ||
                t('settings.agentcoreRunner.empty', 'Not available')
              }}
            </div>
            <div
              data-testid="agentcore-runner-last-optimization-time"
              class="text-xs text-gray-500 dark:text-gray-400"
            >
              {{ formatStatusTime(agentcoreRunnerStatus?.last_optimization_at) }}
            </div>
            <div
              data-testid="agentcore-runner-last-optimization-summary"
              class="mt-1 break-words text-xs text-gray-700 dark:text-gray-200"
            >
              {{
                agentcoreRunnerStatus?.last_optimization_summary ||
                t('settings.agentcoreRunner.empty', 'Not available')
              }}
            </div>
            <div
              v-if="agentcoreRunnerPrimaryPartLabel"
              data-testid="agentcore-runner-primary-part"
              class="mt-1 text-xs text-gray-600 dark:text-gray-300"
            >
              {{
                `${t('settings.agentcoreRunner.primaryPart', 'Primary')}: ${agentcoreRunnerPrimaryPartLabel}`
              }}
            </div>
            <div
              v-if="agentcoreRunnerSourceOptimizationRunID"
              data-testid="agentcore-runner-source-optimization-run-id"
              class="text-xs text-gray-500 dark:text-gray-400"
            >
              {{
                `${t('settings.agentcoreRunner.sourceOptimization', 'Source optimization')}: ${agentcoreRunnerSourceOptimizationRunID}`
              }}
            </div>
            <div
              v-if="agentcoreRunnerLastRun"
              data-testid="agentcore-runner-last-run-detail"
              :class="agentcoreRunnerLastRunCardClass"
            >
              <div
                v-if="agentcoreRunnerLastRunMeta.length > 0"
                data-testid="agentcore-runner-last-run-meta"
                class="flex flex-wrap gap-1.5"
              >
                <span
                  v-for="item in agentcoreRunnerLastRunMeta"
                  :key="item"
                  class="rounded-full bg-gray-100 px-2 py-0.5 text-[11px] font-medium text-gray-700 dark:bg-slate-800 dark:text-gray-200"
                >
                  {{ item }}
                </span>
              </div>
              <div
                v-if="agentcoreRunnerLastRun?.runner_error"
                data-testid="agentcore-runner-last-run-error"
                class="rounded-lg bg-red-50 px-3 py-2 text-[11px] leading-5 whitespace-pre-wrap text-red-700 dark:bg-red-950/30 dark:text-red-300"
              >
                {{ agentcoreRunnerLastRun.runner_error }}
              </div>
              <div
                v-if="agentcoreRunnerLastRun?.runner_response_text"
                data-testid="agentcore-runner-last-run-response"
                class="rounded-lg border border-gray-200 bg-white px-3 py-2 text-[11px] leading-5 whitespace-pre-wrap text-gray-700 dark:border-gray-700 dark:bg-slate-900 dark:text-gray-200"
              >
                {{ agentcoreRunnerLastRun.runner_response_text }}
              </div>
              <div
                v-if="agentcoreRunnerLastRunTranscriptEntries.length > 0"
                data-testid="agentcore-runner-last-run-transcript"
                class="max-h-48 space-y-2 overflow-auto"
              >
                <div
                  v-for="(entry, index) in agentcoreRunnerVisibleTranscriptEntries"
                  :key="`${entry.direction || 'run'}-${entry.method || 'message'}-${index}`"
                  class="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 dark:border-gray-700 dark:bg-slate-900/70"
                >
                  <div
                    class="flex flex-wrap gap-1.5 text-[10px] font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400"
                  >
                    <span>{{ entry.direction || 'run' }}</span>
                    <span v-if="entry.method">{{ entry.method }}</span>
                  </div>
                  <div class="mt-1 text-[11px] leading-5 whitespace-pre-wrap text-gray-700 dark:text-gray-200">
                    {{ entry.text }}
                  </div>
                </div>
              </div>
              <button
                v-if="agentcoreRunnerHiddenTranscriptCount > 0"
                data-testid="agentcore-runner-transcript-toggle"
                type="button"
                class="text-xs font-medium text-gray-500 transition hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
                :aria-expanded="agentcoreRunnerTranscriptExpanded ? 'true' : 'false'"
                @click="agentcoreRunnerTranscriptExpanded = !agentcoreRunnerTranscriptExpanded"
              >
                {{
                  agentcoreRunnerTranscriptExpanded
                    ? t('settings.smallModel.collapse', 'Collapse')
                    : t('settings.smallModel.expand', 'Expand')
                }}
              </button>
            </div>
          </div>
          <div class="text-sm sm:col-span-2">
            <span class="text-gray-500 dark:text-gray-400">{{
              t('settings.agentcoreRunner.lastError', 'Last error')
            }}</span>
            <div
              data-testid="agentcore-runner-last-error"
              class="break-all"
              :class="
                agentcoreRunnerHasLastError
                  ? 'text-red-600 dark:text-red-400'
                  : 'text-gray-500 dark:text-gray-400'
              "
            >
              {{
                agentcoreRunnerLastError || t('settings.agentcoreRunner.empty', 'Not available')
              }}
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.agentcore-runner-panel {
  display: flex;
  flex-direction: column;
  gap: 0.9rem;
  padding: 1.1rem;
  border-radius: 1.35rem;
  box-shadow: none;
}

.agentcore-runner-panel--embedded {
  padding: 0;
  border: 0;
  border-radius: 0;
  background: transparent;
}

.agentcore-runner-panel__title {
  margin: 0;
  font-size: 1.22rem;
  line-height: 1.15;
}

.agentcore-runner-panel__eyebrow {
  color: rgb(var(--dashboard-page-accent, 37, 99, 235));
}

.agentcore-runner-panel__body {
  border-radius: 1rem;
}

.agentcore-runner-panel__body--embedded {
  padding: 0;
  border-radius: 0;
  background: transparent;
}

.agentcore-runner-panel__actions {
  align-items: flex-start;
}

@media (max-width: 768px) {
  .agentcore-runner-panel {
    padding: 1rem;
  }

  .agentcore-runner-panel--embedded {
    padding: 0;
  }

  .agentcore-runner-panel__actions {
    flex-direction: column;
  }

  .agentcore-runner-panel__actions > div {
    width: 100%;
    justify-content: flex-end;
  }
}
</style>
