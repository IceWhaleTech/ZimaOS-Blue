<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { conversationApi, messageApi, type Conversation, type Message } from '@/api/chat'
import {
  harnessApi,
  type HarnessBaseline,
  type HarnessBaselineSpec,
  type HarnessCompareEvalRunRequest,
  type HarnessComparisonReport,
  type HarnessDataset,
  type HarnessDatasetVersion,
  type HarnessDatasetSpec,
  type HarnessDatasetVersionSpec,
  type HarnessEvalRun,
  type HarnessEvalRunReport,
  type HarnessEvalRunSpec,
  type HarnessEvalSpec,
  type HarnessEvalSpecSpec,
  type HarnessRunGroup,
  type HarnessRunGroupSpec,
  type HarnessRunGroupStatus,
  type HarnessRunKind,
  type HarnessScoringMode,
} from '@/api/harness'
import AutomationTabs from '@/components/automation/AutomationTabs.vue'
import { useNotificationStore } from '@/stores/notification'
import { getErrorMessage } from '@/utils/error'
import { harnessFailureLabelHint } from '@/utils/harnessFailureHints'

type GroupFilterMode = 'all' | 'active' | 'terminal'
type QuickEvalSourceMode = 'manifest' | 'dataset' | 'conversation'
type QuickEvalPreset = 'smoke' | 'regression' | 'research'

type DatasetFormState = {
  name: string
  description: string
  subject: string
  runKind: HarnessRunKind
  profile: string
}

type DatasetVersionFormState = {
  datasetID: string
  version: string
  sourceType: string
  sourceRef: string
  manifestText: string
}

type EvalSpecFormState = {
  name: string
  datasetID: string
  datasetVersionID: string
  subject: string
  runKind: HarnessRunKind
  profile: string
  scoringMode: HarnessScoringMode
  passThreshold: string
  judgeModel: string
  ruleProfile: string
}

type EvalRunFormState = {
  evalSpecID: string
  title: string
  baselineEvalRunID: string
  triggerKind: string
  triggerRef: string
}

type InlineBaselineFormState = {
  name: string
  isDefault: boolean
}

type InlineCompareFormState = {
  baselineID: string
  baseEvalRunID: string
}

type QuickEvalFormState = {
  sourceMode: QuickEvalSourceMode
  datasetID: string
  conversationID: string
  preset: QuickEvalPreset
  manifestText: string
}

const { t, te } = useI18n()
const router = useRouter()
const notification = useNotificationStore()

type NumberEntry = {
  key: string
  value: number
}

const loading = ref(false)
const refreshing = ref(false)
const error = ref('')
const groups = ref<HarnessRunGroup[]>([])
const datasets = ref<HarnessDataset[]>([])
const evalSpecs = ref<HarnessEvalSpec[]>([])
const evalRuns = ref<HarnessEvalRun[]>([])
const baselines = ref<HarnessBaseline[]>([])
const datasetVersionsByDataset = ref<Record<string, HarnessDatasetVersion[]>>({})
const selectedEvalRunID = ref('')
const selectedEvalRunReport = ref<HarnessEvalRunReport | null>(null)
const selectedComparisonReport = ref<HarnessComparisonReport | null>(null)
const reportLoadingRunID = ref('')
const compareLoadingRunID = ref('')
const createAction = ref<'dataset' | 'version' | 'spec' | 'run' | 'quick' | ''>('')
const cancelEvalRunID = ref('')
const baselineCreateRunID = ref('')
const advancedBuilderOpen = ref(false)
const quickEvalConversations = ref<Conversation[]>([])
const quickEvalConversationMessagesByID = ref<Record<string, Message[]>>({})
const quickEvalConversationLoaded = ref(false)
const quickEvalConversationLoading = ref(false)
const quickEvalConversationMessagesLoadingID = ref('')
const quickEvalConversationError = ref('')

const groupSearch = ref('')
const groupFilterMode = ref<GroupFilterMode>('all')

const defaultManifestExample = JSON.stringify(
  {
    dataset: {
      subject: 'agent_task',
    },
    defaults: {
      run_kind: 'agent_task',
      profile: 'smoke',
      scoring: {
        mode: 'rule',
        pass_threshold: 0.5,
      },
    },
    items: [],
  },
  null,
  2
)

const datasetForm = ref<DatasetFormState>({
  name: '',
  description: '',
  subject: 'agent_task',
  runKind: 'agent_task',
  profile: 'smoke',
})

const datasetVersionForm = ref<DatasetVersionFormState>({
  datasetID: '',
  version: 'v1',
  sourceType: 'manual',
  sourceRef: '',
  manifestText: defaultManifestExample,
})

const evalSpecForm = ref<EvalSpecFormState>({
  name: '',
  datasetID: '',
  datasetVersionID: '',
  subject: 'agent_task',
  runKind: 'agent_task',
  profile: 'smoke',
  scoringMode: 'rule',
  passThreshold: '0.5',
  judgeModel: '',
  ruleProfile: '',
})

const evalRunForm = ref<EvalRunFormState>({
  evalSpecID: '',
  title: '',
  baselineEvalRunID: '',
  triggerKind: 'manual',
  triggerRef: '',
})

const inlineBaselineForm = ref<InlineBaselineFormState>({
  name: '',
  isDefault: true,
})

const inlineCompareForm = ref<InlineCompareFormState>({
  baselineID: '',
  baseEvalRunID: '',
})

const quickEvalForm = ref<QuickEvalFormState>({
  sourceMode: 'manifest',
  datasetID: '',
  conversationID: '',
  preset: 'smoke',
  manifestText: defaultManifestExample,
})

const runKindOptions: HarnessRunKind[] = ['agent_task', 'research', 'subagent']
const scoringModeOptions: HarnessScoringMode[] = ['rule', 'judge', 'hybrid']
const quickEvalPresetOptions: QuickEvalPreset[] = ['smoke', 'regression', 'research']
const terminalStatuses = new Set<HarnessRunGroupStatus>([
  'completed',
  'partial',
  'failed',
  'cancelled',
])

let refreshTimer: ReturnType<typeof setInterval> | null = null

function tr(key: string, fallback: string): string {
  return te(key) ? t(key) : fallback
}

function trp(
  key: string,
  fallback: string,
  params: Record<string, string | number | boolean>
): string {
  if (te(key)) return t(key, params)
  let text = fallback
  for (const [name, value] of Object.entries(params)) {
    text = text.split(`{${name}}`).join(String(value))
  }
  return text
}

function humanizeEnum(value: string): string {
  return value.replace(/_/g, ' ').replace(/\b\w/g, (char) => char.toUpperCase())
}

function firstNonEmpty(...values: Array<string | null | undefined>): string {
  for (const value of values) {
    const normalized = String(value || '').trim()
    if (normalized) return normalized
  }
  return ''
}

function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  return value as Record<string, unknown>
}

function cloneRecord(value: unknown): Record<string, unknown> | null {
  const record = asRecord(value)
  if (!record) return null
  return JSON.parse(JSON.stringify(record)) as Record<string, unknown>
}

function mergeRecords(
  base?: Record<string, unknown> | null,
  override?: Record<string, unknown> | null
): Record<string, unknown> | null {
  if (!base && !override) return null
  return {
    ...(base || {}),
    ...(override || {}),
  }
}

function summaryValue(summary: unknown, key: string): number | null {
  const record = asRecord(summary)
  if (!record || !(key in record)) return null
  const value = Number(record[key])
  return Number.isFinite(value) ? value : null
}

function summaryNumber(summary: unknown, key: string): number {
  return summaryValue(summary, key) ?? 0
}

function manifestItemCount(manifest: Record<string, unknown>): number {
  return Array.isArray(manifest.items) ? manifest.items.length : 0
}

function numberMap(value: unknown): Record<string, number> {
  const record = asRecord(value)
  if (!record) return {}
  const out: Record<string, number> = {}
  for (const [key, raw] of Object.entries(record)) {
    const next = Number(raw)
    if (Number.isFinite(next)) out[key] = next
  }
  return out
}

function summaryNumberMap(summary: unknown, key: string): Record<string, number> {
  const record = asRecord(summary)
  if (!record) return {}
  return numberMap(record[key])
}

function sortedNumberEntries(
  values: Record<string, number>,
  options: { byAbsolute?: boolean } = {}
): NumberEntry[] {
  const list = Object.entries(values)
    .filter(([, value]) => Number.isFinite(value) && value !== 0)
    .map(([key, value]) => ({ key, value }))
  return list.sort((left, right) => {
    const leftValue = options.byAbsolute ? Math.abs(left.value) : left.value
    const rightValue = options.byAbsolute ? Math.abs(right.value) : right.value
    if (rightValue !== leftValue) return rightValue - leftValue
    return left.key.localeCompare(right.key)
  })
}

function hasFiniteNumber(value: unknown): boolean {
  return Number.isFinite(Number(value))
}

function optionalPercentLabel(summary: unknown, key: string): string {
  const value = summaryValue(summary, key)
  return value == null ? tr('common.notAvailable', 'Not available') : percentLabel(value)
}

function summaryCounts(summary: unknown): Record<string, number> {
  const record = asRecord(summary)
  const counts = asRecord(record?.counts)
  if (!counts) return {}
  const out: Record<string, number> = {}
  for (const [key, value] of Object.entries(counts)) {
    const next = Number(value)
    if (Number.isFinite(next)) out[key] = next
  }
  return out
}

function groupItemCount(group: HarnessRunGroup): number {
  const explicit = summaryNumber(group.summary, 'item_count')
  if (explicit > 0) return explicit
  return Object.entries(summaryCounts(group.summary))
    .filter(([key]) => !key.startsWith('verdict:'))
    .reduce((sum, [, value]) => sum + value, 0)
}

function passRate(summary: unknown): number {
  return summaryNumber(summary, 'pass_rate')
}

function overallScore(summary: unknown): number {
  return summaryNumber(summary, 'overall_score')
}

function runStatusIsActive(status: HarnessRunGroupStatus): boolean {
  return !terminalStatuses.has(status)
}

function statusTone(status: string): string {
  switch (status) {
    case 'completed':
    case 'passed':
    case 'pass':
      return 'is-success'
    case 'running':
    case 'scoring':
    case 'executing':
      return 'is-running'
    case 'partial':
    case 'queued':
    case 'pending':
    case 'planning':
    case 'waiting_input':
      return 'is-warning'
    case 'failed':
    case 'cancelled':
    case 'aborted':
    case 'error':
    case 'fail':
      return 'is-danger'
    default:
      return 'is-muted'
  }
}

function kindTone(kind: string): string {
  switch (kind) {
    case 'experiment':
      return 'is-experiment'
    case 'batch':
      return 'is-batch'
    default:
      return 'is-eval'
  }
}

function statusLabel(status: string): string {
  switch (status) {
    case 'running':
      return tr('harness.groups.running', 'Running')
    case 'failed':
      return tr('harness.groups.failed', 'Failed')
    case 'passed':
      return tr('harness.groups.passed', 'Passed')
    case 'pass':
      return tr('harness.group.passVerdict', 'Pass')
    case 'fail':
      return tr('harness.group.failVerdict', 'Fail')
    case 'partial':
      return tr('harness.group.partialVerdict', 'Partial')
    case 'error':
      return tr('harness.group.errorVerdict', 'Error')
    case 'queued':
      return tr('harness.group.queuedCount', 'Queued')
    default:
      return humanizeEnum(status)
  }
}

function formatDate(value?: string | null): string {
  if (!value) return tr('common.notAvailable', 'Not available')
  const parsed = Date.parse(value)
  if (Number.isNaN(parsed)) return value
  return new Date(parsed).toLocaleString()
}

function percentLabel(value: number): string {
  return `${Math.round(value * 100)}%`
}

function fixedScore(value: number): string {
  return Number(value || 0).toFixed(2)
}

function signedPercentLabel(value: number): string {
  const percent = Math.round(Number(value || 0) * 100)
  return `${percent > 0 ? '+' : ''}${percent}%`
}

function signedIntegerLabel(value: number): string {
  const normalized = Math.trunc(Number(value || 0))
  return normalized > 0 ? `+${normalized}` : String(normalized)
}

function formatQuickTimestamp(date = new Date()): string {
  const year = String(date.getFullYear())
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hour = String(date.getHours()).padStart(2, '0')
  const minute = String(date.getMinutes()).padStart(2, '0')
  return `${year}${month}${day}-${hour}${minute}`
}

function baselineNameByID(baselineID?: string | null): string {
  const normalizedID = String(baselineID || '').trim()
  if (!normalizedID) return tr('common.notAvailable', 'Not available')
  return baselines.value.find((baseline) => baseline.id === normalizedID)?.name || normalizedID
}

function verificationLabel(value?: string | null): string {
  const normalized = String(value || '').trim()
  return normalized ? statusLabel(normalized) : tr('common.notAvailable', 'Not available')
}

function failureLabelText(value?: string | null): string {
  const normalized = String(value || '').trim()
  return normalized || tr('common.notAvailable', 'Not available')
}

function remediationHint(label?: string | null): string {
  const normalized = String(label || '').trim()
  if (!normalized) return ''
  return harnessFailureLabelHint(normalized, tr)
}

function parseJSONObject(raw: string, label: string): Record<string, unknown> {
  const trimmed = raw.trim()
  if (!trimmed) {
    throw new Error(`${label} JSON is required`)
  }
  let parsed: unknown
  try {
    parsed = JSON.parse(trimmed)
  } catch {
    throw new Error(`${label} JSON is invalid`)
  }
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error(`${label} JSON must be an object`)
  }
  return parsed as Record<string, unknown>
}

function versionsForDataset(datasetID: string): HarnessDatasetVersion[] {
  return datasetVersionsByDataset.value[datasetID] || []
}

function activeVersionForDataset(dataset: HarnessDataset): HarnessDatasetVersion | null {
  return (
    versionsForDataset(dataset.id).find((version) => version.id === dataset.active_version_id) ||
    null
  )
}

function quickPresetLabel(preset: QuickEvalPreset): string {
  switch (preset) {
    case 'regression':
      return tr('harness.quickEval.regressionLabel', 'Regression')
    case 'research':
      return tr('harness.quickEval.researchLabel', 'Research')
    default:
      return tr('harness.quickEval.smokeLabel', 'Smoke')
  }
}

function quickPresetConfig(preset: QuickEvalPreset) {
  switch (preset) {
    case 'regression':
      return {
        label: quickPresetLabel(preset),
        runKind: 'agent_task' as HarnessRunKind,
        profile: 'regression',
        subject: 'agent_task',
        scoringMode: 'rule' as HarnessScoringMode,
        passThreshold: 0.75,
        ruleProfile: 'regression',
      }
    case 'research':
      return {
        label: quickPresetLabel(preset),
        runKind: 'research' as HarnessRunKind,
        profile: 'research',
        subject: 'research',
        scoringMode: 'rule' as HarnessScoringMode,
        passThreshold: 0.6,
        ruleProfile: 'research',
      }
    default:
      return {
        label: quickPresetLabel(preset),
        runKind: 'agent_task' as HarnessRunKind,
        profile: 'smoke',
        subject: 'agent_task',
        scoringMode: 'rule' as HarnessScoringMode,
        passThreshold: 0.5,
        ruleProfile: 'smoke',
      }
  }
}

function quickPresetDescription(preset: QuickEvalPreset): string {
  switch (preset) {
    case 'regression':
      return tr(
        'harness.quickEval.regressionHint',
        'Use a stricter pass threshold for repeatable regression checks.'
      )
    case 'research':
      return tr(
        'harness.quickEval.researchHint',
        'Bias the run kind and profile toward research-style evaluation cases.'
      )
    default:
      return tr(
        'harness.quickEval.smokeHint',
        'Fast default preset for smoke checks and first-pass validation.'
      )
  }
}

function quickEvalConversationTitle(conversation?: Conversation | null): string {
  return firstNonEmpty(
    conversation?.title,
    tr('harness.quickEval.untitledConversation', 'Untitled conversation')
  )
}

function buildConversationQuickEvalItems(
  conversation: Conversation,
  messages: Message[],
  preset: QuickEvalPreset
): Array<Record<string, unknown>> {
  const items: Array<Record<string, unknown>> = []
  let pendingUserMessage: Message | null = null
  let turnIndex = 0

  for (const message of messages) {
    if (message.role === 'system' || message.role === 'tool') continue
    const content = String(message.content || '').trim()
    if (message.role === 'user') {
      pendingUserMessage = content ? message : null
      continue
    }
    if (message.role !== 'assistant' || !pendingUserMessage || !content) continue

    const userGoal = String(pendingUserMessage.content || '').trim()
    if (!userGoal) {
      pendingUserMessage = null
      continue
    }

    turnIndex += 1
    const metadata: Record<string, unknown> = {
      conversation_id: conversation.id,
      conversation_title: quickEvalConversationTitle(conversation),
      user_message_id: pendingUserMessage.id,
      assistant_message_id: message.id,
      assistant_created_at: message.created_at,
    }
    const provider = firstNonEmpty(message.provider)
    const model = firstNonEmpty(message.model)
    if (provider) metadata.provider = provider
    if (model) metadata.model = model
    const expected: Record<string, unknown> = {
      status: 'completed',
    }
    if (preset === 'regression') {
      expected.contains = content
    } else if (preset === 'research') {
      expected.required_observations = ['evidence_tool_used']
    }

    items.push({
      id: `turn-${turnIndex}`,
      input: {
        goal: userGoal,
      },
      expected,
      metadata,
    })
    pendingUserMessage = null
  }

  return items
}

function conversationTurnsToQuickEvalManifest(
  conversation: Conversation,
  messages: Message[],
  preset: QuickEvalPreset
): Record<string, unknown> {
  const presetConfig = quickPresetConfig(preset)
  return normalizeQuickEvalManifest(
    {
      dataset: {
        name: `${quickEvalConversationTitle(conversation)} ${presetConfig.label} Dataset`,
        subject: presetConfig.subject,
      },
      defaults: {
        run_kind: presetConfig.runKind,
        profile: presetConfig.profile,
        scoring: {
          mode: presetConfig.scoringMode,
          pass_threshold: presetConfig.passThreshold,
          rule_profile: presetConfig.ruleProfile,
        },
      },
      items: buildConversationQuickEvalItems(conversation, messages, preset),
    },
    preset
  )
}

function normalizeQuickEvalManifest(
  manifest: Record<string, unknown>,
  preset: QuickEvalPreset
): Record<string, unknown> {
  const presetConfig = quickPresetConfig(preset)
  const datasetRecord = asRecord(manifest.dataset) || {}
  const defaultsRecord = asRecord(manifest.defaults) || {}
  const scoringRecord = asRecord(defaultsRecord.scoring) || {}

  return {
    ...manifest,
    dataset: {
      ...datasetRecord,
      name: firstNonEmpty(String(datasetRecord.name || ''), `${presetConfig.label} Dataset`),
      subject: firstNonEmpty(String(datasetRecord.subject || ''), presetConfig.subject),
    },
    defaults: {
      ...defaultsRecord,
      run_kind: firstNonEmpty(String(defaultsRecord.run_kind || ''), presetConfig.runKind),
      profile: firstNonEmpty(String(defaultsRecord.profile || ''), presetConfig.profile),
      scoring: {
        ...scoringRecord,
        mode: firstNonEmpty(String(scoringRecord.mode || ''), presetConfig.scoringMode),
        pass_threshold: Number.isFinite(Number(scoringRecord.pass_threshold))
          ? Number(scoringRecord.pass_threshold)
          : presetConfig.passThreshold,
        rule_profile: firstNonEmpty(
          String(scoringRecord.rule_profile || ''),
          presetConfig.ruleProfile
        ),
      },
    },
  }
}

function quickEvalDatasetName(manifest: Record<string, unknown>, preset: QuickEvalPreset): string {
  const presetConfig = quickPresetConfig(preset)
  const datasetRecord = asRecord(manifest.dataset)
  return firstNonEmpty(
    String(datasetRecord?.name || ''),
    `Quick ${presetConfig.label} ${formatQuickTimestamp()}`
  )
}

function quickEvalRunTitle(name: string, preset: QuickEvalPreset): string {
  return `${name} ${quickPresetConfig(preset).label} ${formatQuickTimestamp()}`
}

function quickEvalGroupSpec(
  manifest: Record<string, unknown>,
  preset: QuickEvalPreset,
  sourceMode: Exclude<QuickEvalSourceMode, 'dataset'>,
  sourceRef: string
): HarnessRunGroupSpec {
  const presetConfig = quickPresetConfig(preset)
  const defaultsRecord = asRecord(manifest.defaults) || {}
  const datasetRecord = asRecord(manifest.dataset) || {}
  const schedulerRecord = asRecord(defaultsRecord.scheduler) || {}
  const scoringRecord = asRecord(defaultsRecord.scoring) || {}
  const runtimePolicyRecord = cloneRecord(defaultsRecord.runtime_policy)
  const items = Array.isArray(manifest.items) ? manifest.items : []
  const datasetName = quickEvalDatasetName(manifest, preset)

  return {
    kind: 'eval',
    title: quickEvalRunTitle(datasetName, preset),
    subject: firstNonEmpty(String(datasetRecord.subject || ''), presetConfig.subject),
    metadata: {
      quick_eval: true,
      ephemeral: true,
      source_mode: sourceMode,
      source_ref: sourceRef,
      quick_eval_preset: preset,
      quick_eval_dataset_name: datasetName,
      quick_eval_eval_name: `${datasetName} ${presetConfig.label} Eval`,
    },
    scheduler: {
      max_concurrency: hasFiniteNumber(schedulerRecord.max_concurrency)
        ? Number(schedulerRecord.max_concurrency)
        : undefined,
      max_attempts: hasFiniteNumber(schedulerRecord.max_attempts)
        ? Number(schedulerRecord.max_attempts)
        : undefined,
      lease_ttl: hasFiniteNumber(schedulerRecord.lease_ttl)
        ? Number(schedulerRecord.lease_ttl)
        : undefined,
      retry_backoff: hasFiniteNumber(schedulerRecord.retry_backoff)
        ? Number(schedulerRecord.retry_backoff)
        : undefined,
    },
    scoring: {
      mode: firstNonEmpty(
        String(scoringRecord.mode || ''),
        presetConfig.scoringMode
      ) as HarnessScoringMode,
      rule_profile: firstNonEmpty(
        String(scoringRecord.rule_profile || ''),
        presetConfig.ruleProfile
      ),
      judge_model: firstNonEmpty(String(scoringRecord.judge_model || '')),
      pass_threshold: hasFiniteNumber(scoringRecord.pass_threshold)
        ? Number(scoringRecord.pass_threshold)
        : presetConfig.passThreshold,
    },
    items: items
      .map((rawItem) => {
        const item = asRecord(rawItem)
        if (!item) return null
        return {
          run_kind: firstNonEmpty(
            String(item.run_kind || ''),
            String(defaultsRecord.run_kind || ''),
            presetConfig.runKind
          ) as HarnessRunKind,
          profile: firstNonEmpty(
            String(item.profile || ''),
            String(defaultsRecord.profile || ''),
            presetConfig.profile
          ),
          input: cloneRecord(item.input),
          expected: cloneRecord(item.expected),
          metadata: mergeRecords(runtimePolicyRecord, cloneRecord(item.metadata)),
        }
      })
      .filter((item): item is NonNullable<typeof item> => Boolean(item)),
  }
}

const datasetByID = computed<Record<string, HarnessDataset>>(() => {
  const out: Record<string, HarnessDataset> = {}
  for (const dataset of datasets.value) out[dataset.id] = dataset
  return out
})

const datasetVersionByID = computed<Record<string, HarnessDatasetVersion>>(() => {
  const out: Record<string, HarnessDatasetVersion> = {}
  for (const versions of Object.values(datasetVersionsByDataset.value)) {
    for (const version of versions) out[version.id] = version
  }
  return out
})

const evalSpecByID = computed<Record<string, HarnessEvalSpec>>(() => {
  const out: Record<string, HarnessEvalSpec> = {}
  for (const spec of evalSpecs.value) out[spec.id] = spec
  return out
})

const selectedSpecDatasetVersions = computed(() => versionsForDataset(evalSpecForm.value.datasetID))

const selectedQuickDataset = computed(
  () => datasetByID.value[quickEvalForm.value.datasetID] || null
)

const selectedQuickConversation = computed(
  () =>
    quickEvalConversations.value.find(
      (conversation) => conversation.id === quickEvalForm.value.conversationID
    ) || null
)

const selectedQuickConversationMessages = computed(
  () => quickEvalConversationMessagesByID.value[quickEvalForm.value.conversationID] || []
)

const selectedQuickConversationManifest = computed(() => {
  const conversation = selectedQuickConversation.value
  if (!conversation) return null
  return conversationTurnsToQuickEvalManifest(
    conversation,
    selectedQuickConversationMessages.value,
    quickEvalForm.value.preset
  )
})

const selectedQuickConversationCaseCount = computed(() =>
  selectedQuickConversationManifest.value
    ? manifestItemCount(selectedQuickConversationManifest.value)
    : 0
)

const selectedQuickConversationManifestText = computed(() =>
  selectedQuickConversationManifest.value
    ? JSON.stringify(selectedQuickConversationManifest.value, null, 2)
    : defaultManifestExample
)

const selectedQuickDatasetVersion = computed(() => {
  const dataset = selectedQuickDataset.value
  if (!dataset) return null
  return activeVersionForDataset(dataset) || versionsForDataset(dataset.id)[0] || null
})

const selectedEvalRun = computed(
  () => evalRuns.value.find((run) => run.id === selectedEvalRunID.value) || null
)

const baselineOptions = computed(() =>
  evalRuns.value.filter((run) => run.eval_spec_id === evalRunForm.value.evalSpecID)
)

const selectedEvalRunBaselines = computed(() => {
  const evalSpecID =
    selectedEvalRunReport.value?.eval_run?.eval_spec_id || selectedEvalRun.value?.eval_spec_id || ''
  if (!evalSpecID) return []
  return baselines.value.filter((baseline) => baseline.eval_spec_id === evalSpecID)
})

const selectedEvalRunCompareOptions = computed(() => {
  const evalSpecID =
    selectedEvalRunReport.value?.eval_run?.eval_spec_id || selectedEvalRun.value?.eval_spec_id || ''
  const targetID = selectedEvalRunReport.value?.eval_run?.id || selectedEvalRun.value?.id || ''
  if (!evalSpecID) return []
  return evalRuns.value.filter((run) => run.eval_spec_id === evalSpecID && run.id !== targetID)
})

const selectedReportRun = computed(
  () => selectedEvalRunReport.value?.eval_run || selectedEvalRun.value || null
)

const selectedFailureLabelEntries = computed(() =>
  sortedNumberEntries(
    summaryNumberMap(
      selectedEvalRunReport.value?.group_report?.group?.summary,
      'failure_label_counts'
    )
  )
)

const selectedComparisonFailureLabelDeltaEntries = computed(() =>
  sortedNumberEntries(
    summaryNumberMap(selectedComparisonReport.value?.summary, 'failure_label_delta'),
    { byAbsolute: true }
  )
)

const filteredGroups = computed(() => {
  const query = groupSearch.value.trim().toLowerCase()
  return groups.value.filter((group) => {
    if (groupFilterMode.value === 'active' && terminalStatuses.has(group.status)) return false
    if (groupFilterMode.value === 'terminal' && !terminalStatuses.has(group.status)) return false
    if (!query) return true

    return [group.title, group.subject, group.kind, group.status, group.id]
      .map((value) => String(value || '').toLowerCase())
      .some((value) => value.includes(query))
  })
})

const activeGroups = computed(
  () => groups.value.filter((group) => runStatusIsActive(group.status)).length
)
const activeEvalRuns = computed(
  () => evalRuns.value.filter((run) => runStatusIsActive(run.status)).length
)
const defaultBaselines = computed(
  () => baselines.value.filter((baseline) => baseline.is_default).length
)
const averageEvalPassRate = computed(() => {
  const rated = evalRuns.value
    .map((run) => passRate(run.summary))
    .filter((value) => Number.isFinite(value) && value > 0)
  if (!rated.length) return 0
  return rated.reduce((sum, value) => sum + value, 0) / rated.length
})

async function loadDatasetVersions(datasetID: string, options: { force?: boolean } = {}) {
  const normalizedID = String(datasetID || '').trim()
  if (!normalizedID) return []
  if (!options.force && datasetVersionsByDataset.value[normalizedID]) {
    return datasetVersionsByDataset.value[normalizedID]
  }
  const response = await harnessApi.listDatasetVersions(normalizedID, { limit: 20 })
  const versions = response.data || []
  datasetVersionsByDataset.value = {
    ...datasetVersionsByDataset.value,
    [normalizedID]: versions,
  }
  return versions
}

async function hydrateDatasetVersions(datasetList: HarnessDataset[]) {
  const pending = datasetList
    .map((dataset) => dataset.id)
    .filter((datasetID) => !datasetVersionsByDataset.value[datasetID])
  if (!pending.length) return
  await Promise.allSettled(pending.map((datasetID) => loadDatasetVersions(datasetID)))
}

async function loadQuickEvalConversations(options: { force?: boolean } = {}) {
  if (quickEvalConversationLoaded.value && !options.force) {
    return quickEvalConversations.value
  }
  quickEvalConversationLoading.value = true
  quickEvalConversationError.value = ''
  try {
    const response = await conversationApi.list(50, 0)
    const conversations = response.data || []
    quickEvalConversations.value = conversations
    quickEvalConversationLoaded.value = true
    if (
      !quickEvalForm.value.conversationID ||
      !conversations.some((conversation) => conversation.id === quickEvalForm.value.conversationID)
    ) {
      quickEvalForm.value.conversationID = conversations[0]?.id || ''
    }
    return conversations
  } catch (err) {
    quickEvalConversationLoaded.value = false
    quickEvalConversationError.value = `${tr(
      'harness.quickEval.conversationLoadFailed',
      'Quick eval could not load conversation data.'
    )} ${getErrorMessage(err)}`.trim()
    throw new Error(quickEvalConversationError.value)
  } finally {
    quickEvalConversationLoading.value = false
  }
}

async function loadQuickEvalConversationMessages(
  conversationID: string,
  options: { force?: boolean } = {}
) {
  const normalizedID = String(conversationID || '').trim()
  if (!normalizedID) return []
  if (quickEvalConversationMessagesByID.value[normalizedID] && !options.force) {
    return quickEvalConversationMessagesByID.value[normalizedID]
  }
  quickEvalConversationMessagesLoadingID.value = normalizedID
  quickEvalConversationError.value = ''
  try {
    const response = await messageApi.list(normalizedID, 500, 0)
    const messages = response.data || []
    quickEvalConversationMessagesByID.value = {
      ...quickEvalConversationMessagesByID.value,
      [normalizedID]: messages,
    }
    return messages
  } catch (err) {
    quickEvalConversationError.value = `${tr(
      'harness.quickEval.conversationLoadFailed',
      'Quick eval could not load conversation data.'
    )} ${getErrorMessage(err)}`.trim()
    throw new Error(quickEvalConversationError.value)
  } finally {
    if (quickEvalConversationMessagesLoadingID.value === normalizedID) {
      quickEvalConversationMessagesLoadingID.value = ''
    }
  }
}

function copyConversationDraftToManifest() {
  quickEvalForm.value.manifestText = selectedQuickConversationManifestText.value
  quickEvalForm.value.sourceMode = 'manifest'
}

function ensureFormDefaults() {
  const firstDataset = datasets.value[0]
  if (firstDataset) {
    if (!quickEvalForm.value.datasetID || !datasetByID.value[quickEvalForm.value.datasetID]) {
      quickEvalForm.value.datasetID = firstDataset.id
    }
    if (
      !datasetVersionForm.value.datasetID ||
      !datasetByID.value[datasetVersionForm.value.datasetID]
    ) {
      datasetVersionForm.value.datasetID = firstDataset.id
    }
    if (!evalSpecForm.value.datasetID || !datasetByID.value[evalSpecForm.value.datasetID]) {
      evalSpecForm.value.datasetID = firstDataset.id
    }
  }

  const datasetVersions = selectedSpecDatasetVersions.value
  const firstDatasetVersion = datasetVersions[0]
  if (
    firstDatasetVersion &&
    !datasetVersions.some((version) => version.id === evalSpecForm.value.datasetVersionID)
  ) {
    evalSpecForm.value.datasetVersionID = firstDatasetVersion.id
  }

  const firstSpec = evalSpecs.value[0]
  if (firstSpec && !evalSpecByID.value[evalRunForm.value.evalSpecID]) {
    evalRunForm.value.evalSpecID = firstSpec.id
  }
}

function syncSelectedRunForms() {
  const selectedRun = selectedEvalRunReport.value?.eval_run || selectedEvalRun.value
  if (!selectedRun) {
    inlineBaselineForm.value.name = ''
    inlineBaselineForm.value.isDefault = true
    inlineCompareForm.value.baselineID = ''
    inlineCompareForm.value.baseEvalRunID = ''
    return
  }

  inlineBaselineForm.value.name = `${selectedRun.title || selectedRun.id} Baseline`

  const matchingBaselines = baselines.value.filter(
    (baseline) =>
      baseline.eval_spec_id === selectedRun.eval_spec_id && baseline.eval_run_id !== selectedRun.id
  )
  const preferredBaseline =
    matchingBaselines.find(
      (baseline) => baseline.eval_run_id === selectedRun.baseline_eval_run_id
    ) ||
    matchingBaselines.find((baseline) => baseline.is_default) ||
    matchingBaselines[0] ||
    null

  inlineCompareForm.value.baselineID = preferredBaseline?.id || ''
  inlineCompareForm.value.baseEvalRunID =
    preferredBaseline == null && selectedRun.baseline_eval_run_id
      ? selectedRun.baseline_eval_run_id
      : ''
}

async function loadEvalRunReport(runID: string, options: { silent?: boolean } = {}) {
  const normalizedID = String(runID || '').trim()
  if (!normalizedID) {
    selectedEvalRunID.value = ''
    selectedEvalRunReport.value = null
    selectedComparisonReport.value = null
    return
  }
  selectedEvalRunID.value = normalizedID
  if (!options.silent) {
    reportLoadingRunID.value = normalizedID
  }
  try {
    const response = await harnessApi.getEvalRunReport(normalizedID)
    selectedEvalRunReport.value = response.data || null
    selectedComparisonReport.value = null
    syncSelectedRunForms()
  } catch (err) {
    notification.error(tr('automation.tabs.harness', 'Harness'), getErrorMessage(err))
  } finally {
    reportLoadingRunID.value = ''
  }
}

async function loadConsole(options: { silent?: boolean } = {}) {
  if (options.silent) {
    refreshing.value = true
  } else {
    loading.value = true
  }
  error.value = ''
  try {
    const [groupResponse, datasetResponse, specResponse, runResponse, baselineResponse] =
      await Promise.all([
        harnessApi.listGroups({ limit: 100 }),
        harnessApi.listDatasets({ limit: 100 }),
        harnessApi.listEvalSpecs({ limit: 100 }),
        harnessApi.listEvalRuns({ limit: 100 }),
        harnessApi.listBaselines({ limit: 100 }),
      ])
    groups.value = groupResponse.data || []
    datasets.value = datasetResponse.data || []
    evalSpecs.value = specResponse.data || []
    evalRuns.value = runResponse.data || []
    baselines.value = baselineResponse.data || []
    await hydrateDatasetVersions(datasets.value)
    ensureFormDefaults()
    if (
      selectedEvalRunID.value &&
      evalRuns.value.some((run) => run.id === selectedEvalRunID.value)
    ) {
      await loadEvalRunReport(selectedEvalRunID.value, { silent: true })
    } else if (selectedEvalRunID.value) {
      selectedEvalRunID.value = ''
      selectedEvalRunReport.value = null
      selectedComparisonReport.value = null
    }
    syncSelectedRunForms()
  } catch (err) {
    error.value = getErrorMessage(err)
  } finally {
    loading.value = false
    refreshing.value = false
    syncRefreshTimer()
  }
}

function prefillVersionFromDataset(dataset: HarnessDataset) {
  datasetVersionForm.value.datasetID = dataset.id
  datasetVersionForm.value.version =
    activeVersionForDataset(dataset)?.version || datasetVersionForm.value.version || 'v1'
}

function prefillSpecFromDataset(dataset: HarnessDataset, version?: HarnessDatasetVersion | null) {
  evalSpecForm.value.datasetID = dataset.id
  evalSpecForm.value.datasetVersionID = version?.id || dataset.active_version_id || ''
  evalSpecForm.value.subject = dataset.subject || evalSpecForm.value.subject
  evalSpecForm.value.runKind = dataset.default_run_kind || evalSpecForm.value.runKind
  evalSpecForm.value.profile = dataset.default_profile || evalSpecForm.value.profile
  if (!evalSpecForm.value.name.trim()) {
    evalSpecForm.value.name = `${dataset.name} Eval`
  }
}

function prefillRunFromSpec(spec: HarnessEvalSpec) {
  evalRunForm.value.evalSpecID = spec.id
  if (!evalRunForm.value.title.trim()) {
    evalRunForm.value.title = `${spec.name} Run`
  }
}

async function submitDataset() {
  createAction.value = 'dataset'
  try {
    const payload: HarnessDatasetSpec = {
      name: datasetForm.value.name.trim(),
      description: datasetForm.value.description.trim(),
      subject: datasetForm.value.subject.trim(),
      default_run_kind: datasetForm.value.runKind,
      default_profile: datasetForm.value.profile.trim(),
    }
    const response = await harnessApi.createDataset(payload)
    datasetForm.value.name = ''
    datasetForm.value.description = ''
    datasetForm.value.subject = payload.subject || 'agent_task'
    datasetForm.value.runKind = payload.default_run_kind || 'agent_task'
    datasetForm.value.profile = payload.default_profile || 'smoke'
    await loadConsole({ silent: true })
    if (response.data?.id) {
      datasetVersionForm.value.datasetID = response.data.id
      evalSpecForm.value.datasetID = response.data.id
    }
    notification.success(
      tr('automation.tabs.harness', 'Harness'),
      tr('harness.dataset.created', 'Dataset created')
    )
  } catch (err) {
    notification.error(tr('automation.tabs.harness', 'Harness'), getErrorMessage(err))
  } finally {
    createAction.value = ''
  }
}

async function submitDatasetVersion() {
  createAction.value = 'version'
  try {
    const manifest = parseJSONObject(datasetVersionForm.value.manifestText, 'Manifest')
    const payload: HarnessDatasetVersionSpec = {
      version: datasetVersionForm.value.version.trim(),
      source_type: datasetVersionForm.value.sourceType.trim(),
      source_ref: datasetVersionForm.value.sourceRef.trim(),
      manifest,
    }
    await harnessApi.createDatasetVersion(datasetVersionForm.value.datasetID, payload)
    await loadDatasetVersions(datasetVersionForm.value.datasetID, { force: true })
    await loadConsole({ silent: true })
    const latestVersion = versionsForDataset(datasetVersionForm.value.datasetID)[0]
    if (latestVersion) {
      evalSpecForm.value.datasetID = latestVersion.dataset_id
      evalSpecForm.value.datasetVersionID = latestVersion.id
    }
    notification.success(
      tr('automation.tabs.harness', 'Harness'),
      tr('harness.dataset.versionCreated', 'Dataset version published')
    )
  } catch (err) {
    notification.error(tr('automation.tabs.harness', 'Harness'), getErrorMessage(err))
  } finally {
    createAction.value = ''
  }
}

async function submitEvalSpec() {
  createAction.value = 'spec'
  try {
    const passThreshold = Number(evalSpecForm.value.passThreshold)
    const payload: HarnessEvalSpecSpec = {
      name: evalSpecForm.value.name.trim(),
      dataset_id: evalSpecForm.value.datasetID,
      dataset_version_id: evalSpecForm.value.datasetVersionID,
      subject: evalSpecForm.value.subject.trim(),
      run_kind: evalSpecForm.value.runKind,
      profile: evalSpecForm.value.profile.trim(),
      scoring: {
        mode: evalSpecForm.value.scoringMode,
        rule_profile: evalSpecForm.value.ruleProfile.trim(),
        judge_model: evalSpecForm.value.judgeModel.trim(),
        pass_threshold: Number.isFinite(passThreshold) ? passThreshold : undefined,
      },
    }
    const response = await harnessApi.createEvalSpec(payload)
    evalSpecForm.value.name = ''
    await loadConsole({ silent: true })
    if (response.data?.id) {
      evalRunForm.value.evalSpecID = response.data.id
    }
    notification.success(
      tr('automation.tabs.harness', 'Harness'),
      tr('harness.evalSpec.created', 'Eval spec created')
    )
  } catch (err) {
    notification.error(tr('automation.tabs.harness', 'Harness'), getErrorMessage(err))
  } finally {
    createAction.value = ''
  }
}

async function submitEvalRun() {
  createAction.value = 'run'
  try {
    const payload: HarnessEvalRunSpec = {
      eval_spec_id: evalRunForm.value.evalSpecID,
      title: evalRunForm.value.title.trim(),
      baseline_eval_run_id: evalRunForm.value.baselineEvalRunID.trim(),
      trigger_kind: evalRunForm.value.triggerKind.trim(),
      trigger_ref: evalRunForm.value.triggerRef.trim(),
    }
    const response = await harnessApi.createEvalRun(payload)
    evalRunForm.value.title = ''
    evalRunForm.value.triggerRef = ''
    await loadConsole({ silent: true })
    if (response.data?.id) {
      await loadEvalRunReport(response.data.id)
    }
    notification.success(
      tr('automation.tabs.harness', 'Harness'),
      tr('harness.evalRun.created', 'Eval run launched')
    )
  } catch (err) {
    notification.error(tr('automation.tabs.harness', 'Harness'), getErrorMessage(err))
  } finally {
    createAction.value = ''
  }
}

function matchingQuickEvalSpec(
  datasetID: string,
  datasetVersionID: string,
  preset: QuickEvalPreset
): HarnessEvalSpec | null {
  const presetConfig = quickPresetConfig(preset)
  const desiredPassThreshold = presetConfig.passThreshold

  return (
    evalSpecs.value.find((spec) => {
      if (spec.dataset_id !== datasetID || spec.dataset_version_id !== datasetVersionID) {
        return false
      }
      if (spec.run_kind !== presetConfig.runKind) return false
      if (firstNonEmpty(spec.profile, presetConfig.profile) !== presetConfig.profile) return false
      if (
        firstNonEmpty(spec.scoring_config?.mode, presetConfig.scoringMode) !==
        presetConfig.scoringMode
      ) {
        return false
      }
      const passThreshold = Number(spec.scoring_config?.pass_threshold)
      if (
        Number.isFinite(passThreshold) &&
        Math.abs(passThreshold - desiredPassThreshold) > 0.0001
      ) {
        return false
      }
      return true
    }) || null
  )
}

function defaultBaselineRunID(evalSpecID: string): string {
  return (
    baselines.value.find((baseline) => baseline.eval_spec_id === evalSpecID && baseline.is_default)
      ?.eval_run_id || ''
  )
}

async function submitQuickEval() {
  createAction.value = 'quick'
  try {
    const presetConfig = quickPresetConfig(quickEvalForm.value.preset)
    let dataset: HarnessDataset | null = null
    let datasetVersion: HarnessDatasetVersion | null = null
    let manifest: Record<string, unknown> | null = null
    let quickSourceMode: Exclude<QuickEvalSourceMode, 'dataset'> | null = null
    let quickSourceRef = 'ui'

    if (quickEvalForm.value.sourceMode === 'dataset') {
      dataset = selectedQuickDataset.value
      if (!dataset) {
        throw new Error(tr('harness.quickEval.selectDataset', 'Select a dataset'))
      }
      datasetVersion = selectedQuickDatasetVersion.value
      if (!datasetVersion) {
        throw new Error(
          tr(
            'harness.quickEval.datasetVersionRequired',
            'Publish a dataset version before launching a quick eval.'
          )
        )
      }
    } else if (quickEvalForm.value.sourceMode === 'conversation') {
      await loadQuickEvalConversations()
      const conversationID = String(quickEvalForm.value.conversationID || '').trim()
      if (!conversationID) {
        throw new Error(
          tr(
            'harness.quickEval.conversationRequired',
            'Select a conversation before launching a quick eval.'
          )
        )
      }
      const conversation =
        selectedQuickConversation.value || (await conversationApi.get(conversationID)).data || null
      if (!conversation?.id) {
        throw new Error(
          tr(
            'harness.quickEval.conversationRequired',
            'Select a conversation before launching a quick eval.'
          )
        )
      }
      const messages = await loadQuickEvalConversationMessages(conversation.id)
      manifest = conversationTurnsToQuickEvalManifest(
        conversation,
        messages,
        quickEvalForm.value.preset
      )
      if (manifestItemCount(manifest) === 0) {
        throw new Error(
          tr(
            'harness.quickEval.conversationEmpty',
            'This conversation did not produce any draft cases yet.'
          )
        )
      }
      quickSourceMode = 'conversation'
      quickSourceRef = conversation.id
    } else {
      manifest = normalizeQuickEvalManifest(
        parseJSONObject(quickEvalForm.value.manifestText, 'Cases'),
        quickEvalForm.value.preset
      )
      if (manifestItemCount(manifest) === 0) {
        throw new Error(
          tr(
            'harness.quickEval.caseRequired',
            'Add at least one real case before launching a quick eval.'
          )
        )
      }
      quickSourceMode = 'manifest'
    }

    if (manifest && quickSourceMode) {
      const groupResponse = await harnessApi.createGroup(
        quickEvalGroupSpec(manifest, quickEvalForm.value.preset, quickSourceMode, quickSourceRef)
      )
      notification.success(
        tr('automation.tabs.harness', 'Harness'),
        tr('harness.quickEval.created', 'Quick eval launched')
      )
      if (groupResponse.data?.id) {
        await router.push({ name: 'HarnessGroupDetail', params: { id: groupResponse.data.id } })
      }
      return
    }

    const reusableSpec =
      dataset && datasetVersion
        ? matchingQuickEvalSpec(dataset.id, datasetVersion.id, quickEvalForm.value.preset)
        : null

    let evalSpecID = reusableSpec?.id || ''
    if (!evalSpecID && dataset && datasetVersion) {
      const specResponse = await harnessApi.createEvalSpec({
        name: `${dataset.name} ${presetConfig.label} Eval`,
        dataset_id: dataset.id,
        dataset_version_id: datasetVersion.id,
        subject: firstNonEmpty(dataset.subject, presetConfig.subject),
        run_kind: presetConfig.runKind,
        profile: presetConfig.profile,
        scoring: {
          mode: presetConfig.scoringMode,
          rule_profile: presetConfig.ruleProfile,
          pass_threshold: presetConfig.passThreshold,
        },
      })
      evalSpecID = String(specResponse.data?.id || '')
    }

    if (!evalSpecID || !dataset) {
      throw new Error(
        tr('harness.quickEval.specCreateFailed', 'Quick eval could not prepare an eval spec')
      )
    }

    const runResponse = await harnessApi.createEvalRun({
      eval_spec_id: evalSpecID,
      title: quickEvalRunTitle(dataset.name, quickEvalForm.value.preset),
      baseline_eval_run_id: defaultBaselineRunID(evalSpecID),
      trigger_kind: 'quick_eval',
      trigger_ref: 'ui',
    })

    await loadConsole({ silent: true })
    if (runResponse.data?.id) {
      await loadEvalRunReport(runResponse.data.id)
    }
    notification.success(
      tr('automation.tabs.harness', 'Harness'),
      tr('harness.quickEval.created', 'Quick eval launched')
    )
  } catch (err) {
    notification.error(tr('automation.tabs.harness', 'Harness'), getErrorMessage(err))
  } finally {
    createAction.value = ''
  }
}

async function cancelEvalRun(runID: string) {
  cancelEvalRunID.value = runID
  try {
    await harnessApi.cancelEvalRun(runID)
    await loadConsole({ silent: true })
    if (selectedEvalRunID.value === runID) {
      await loadEvalRunReport(runID, { silent: true })
    }
    notification.info(
      tr('automation.tabs.harness', 'Harness'),
      tr('harness.evalRun.cancelled', 'Eval run cancelled')
    )
  } catch (err) {
    notification.error(tr('automation.tabs.harness', 'Harness'), getErrorMessage(err))
  } finally {
    cancelEvalRunID.value = ''
  }
}

async function createBaselineForSelectedRun() {
  const selectedRun = selectedEvalRunReport.value?.eval_run || selectedEvalRun.value
  if (!selectedRun) return
  baselineCreateRunID.value = selectedRun.id
  try {
    const payload: HarnessBaselineSpec = {
      name:
        inlineBaselineForm.value.name.trim() || `${selectedRun.title || selectedRun.id} Baseline`,
      eval_run_id: selectedRun.id,
      eval_spec_id: selectedRun.eval_spec_id,
      is_default: inlineBaselineForm.value.isDefault,
    }
    const response = await harnessApi.createBaseline(payload)
    await loadConsole({ silent: true })
    inlineCompareForm.value.baselineID = response.data?.id || ''
    inlineCompareForm.value.baseEvalRunID = ''
    notification.success(
      tr('automation.tabs.harness', 'Harness'),
      tr('harness.baseline.created', 'Baseline pinned')
    )
  } catch (err) {
    notification.error(tr('automation.tabs.harness', 'Harness'), getErrorMessage(err))
  } finally {
    baselineCreateRunID.value = ''
  }
}

async function compareSelectedRun() {
  const selectedRun = selectedEvalRunReport.value?.eval_run || selectedEvalRun.value
  if (!selectedRun) return
  compareLoadingRunID.value = selectedRun.id
  try {
    const payload: HarnessCompareEvalRunRequest = {}
    if (inlineCompareForm.value.baselineID) {
      payload.baseline_id = inlineCompareForm.value.baselineID
    } else if (inlineCompareForm.value.baseEvalRunID) {
      payload.base_eval_run_id = inlineCompareForm.value.baseEvalRunID
    }
    const response = await harnessApi.compareEvalRun(selectedRun.id, payload)
    selectedComparisonReport.value = response.data || null
    notification.info(
      tr('automation.tabs.harness', 'Harness'),
      tr('harness.compare.created', 'Comparison report generated')
    )
  } catch (err) {
    notification.error(tr('automation.tabs.harness', 'Harness'), getErrorMessage(err))
  } finally {
    compareLoadingRunID.value = ''
  }
}

function syncRefreshTimer() {
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }
  if (activeGroups.value === 0 && activeEvalRuns.value === 0) return
  refreshTimer = setInterval(() => {
    void loadConsole({ silent: true })
  }, 15000)
}

watch(
  () => datasetVersionForm.value.datasetID,
  async (datasetID) => {
    if (!datasetID) return
    try {
      await loadDatasetVersions(datasetID)
    } catch {
      // Keep the rest of the console usable even if one dataset version query fails.
    }
  }
)

watch(
  () => quickEvalForm.value.sourceMode,
  async (sourceMode) => {
    if (sourceMode !== 'conversation') return
    try {
      await loadQuickEvalConversations()
    } catch {
      // Keep the rest of the quick-eval flow usable even if conversation loading fails.
    }
  }
)

watch(
  () => quickEvalForm.value.conversationID,
  async (conversationID) => {
    if (!conversationID || quickEvalForm.value.sourceMode !== 'conversation') return
    try {
      await loadQuickEvalConversationMessages(conversationID)
    } catch {
      // Surface inline state but do not break the rest of the page.
    }
  }
)

watch(
  () => evalSpecForm.value.datasetID,
  async (datasetID) => {
    if (!datasetID) return
    try {
      const versions = await loadDatasetVersions(datasetID)
      const firstVersion = versions[0]
      if (
        firstVersion &&
        !versions.some((version) => version.id === evalSpecForm.value.datasetVersionID)
      ) {
        evalSpecForm.value.datasetVersionID = firstVersion.id
      }
    } catch {
      // Ignore version hydration failures here; top-level load errors are surfaced elsewhere.
    }
  },
  { immediate: true }
)

watch(
  [datasets, evalSpecs, baselines, selectedEvalRunID],
  () => {
    ensureFormDefaults()
    syncSelectedRunForms()
  },
  { deep: true }
)

onMounted(() => {
  void loadConsole()
})

onUnmounted(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }
})
</script>

<template>
  <div class="harness-groups-page">
    <section class="hero-card">
      <div class="hero-copy">
        <p class="eyebrow">Harness V3</p>
        <h1>{{ tr('automation.tabs.harness', 'Harness') }}</h1>
        <p class="hero-description">
          {{
            tr(
              'harness.groups.subtitle',
              'Run the full eval loop from one place: define datasets, freeze versions, create reusable specs, launch runs, and still keep the raw group control plane in view.'
            )
          }}
        </p>
      </div>
      <div class="hero-actions">
        <button
          class="refresh-button"
          type="button"
          :disabled="loading || refreshing"
          @click="loadConsole({ silent: true })"
        >
          {{ refreshing ? tr('common.loading', 'Loading') : tr('common.refresh', 'Refresh') }}
        </button>
      </div>
    </section>

    <AutomationTabs class="automation-tab-strip" />

    <section class="stats-grid">
      <article class="stat-card">
        <span class="stat-label">{{ tr('harness.datasets.total', 'Datasets') }}</span>
        <strong class="stat-value">{{ datasets.length }}</strong>
      </article>
      <article class="stat-card">
        <span class="stat-label">{{ tr('harness.evalSpecs.total', 'Eval specs') }}</span>
        <strong class="stat-value">{{ evalSpecs.length }}</strong>
      </article>
      <article class="stat-card">
        <span class="stat-label">{{ tr('harness.evalRuns.total', 'Eval runs') }}</span>
        <strong class="stat-value">{{ evalRuns.length }}</strong>
      </article>
      <article class="stat-card">
        <span class="stat-label">{{ tr('harness.baseline.total', 'Baselines') }}</span>
        <strong class="stat-value">{{ baselines.length }}</strong>
      </article>
      <article class="stat-card">
        <span class="stat-label">{{ tr('harness.evalRuns.active', 'Active eval runs') }}</span>
        <strong class="stat-value">{{ activeEvalRuns }}</strong>
      </article>
      <article class="stat-card">
        <span class="stat-label">{{ tr('harness.groups.totalGroups', 'Groups') }}</span>
        <strong class="stat-value">{{ groups.length }}</strong>
      </article>
      <article class="stat-card">
        <span class="stat-label">{{ tr('harness.groups.avgPassRate', 'Average pass rate') }}</span>
        <strong class="stat-value">{{ percentLabel(averageEvalPassRate) }}</strong>
      </article>
      <article class="stat-card">
        <span class="stat-label">{{
          tr('harness.baseline.defaultCount', 'Default baselines')
        }}</span>
        <strong class="stat-value">{{ defaultBaselines }}</strong>
      </article>
    </section>

    <div v-if="error" class="state-card is-error">
      <h2>{{ tr('common.error', 'Error') }}</h2>
      <p>{{ error }}</p>
    </div>

    <div v-else-if="loading" class="state-card">
      <h2>{{ tr('common.loading', 'Loading') }}</h2>
      <p>{{ tr('harness.groups.loading', 'Fetching the latest harness group summaries.') }}</p>
    </div>

    <div v-else class="console-layout">
      <aside class="console-rail">
        <section class="panel rail-panel quick-eval-panel">
          <div class="section-header">
            <div>
              <p class="section-eyebrow">{{ tr('harness.quickEval.eyebrow', 'Default Path') }}</p>
              <h2>{{ tr('harness.quickEval.title', 'Quick Eval') }}</h2>
              <p class="section-description">
                {{
                  tr(
                    'harness.quickEval.description',
                    'Keep input minimal: choose cases, pick a preset, and Harness auto-creates the dataset version, eval spec, and run for you.'
                  )
                }}
              </p>
            </div>
            <button
              type="button"
              class="secondary-button"
              @click="advancedBuilderOpen = !advancedBuilderOpen"
            >
              {{
                advancedBuilderOpen
                  ? tr('harness.builder.hideAdvanced', 'Hide advanced')
                  : tr('harness.builder.showAdvanced', 'Open advanced')
              }}
            </button>
          </div>

          <div class="quick-eval-layout">
            <article class="detail-card quick-eval-copy-card">
              <h3>{{ tr('harness.quickEval.minimalInput', 'Only two decisions') }}</h3>
              <p class="card-copy">
                {{
                  tr(
                    'harness.quickEval.minimalDescription',
                    'Most runs only need the case source and a preset. Everything else becomes an implementation detail instead of a required form.'
                  )
                }}
              </p>
              <div class="version-list">
                <span class="version-chip active">{{
                  tr('harness.quickEval.sourceChip', '1. Cases')
                }}</span>
                <span class="version-chip active">{{
                  tr('harness.quickEval.presetChip', '2. Preset')
                }}</span>
                <span class="version-chip">{{
                  tr('harness.quickEval.autoChip', 'Auto: version + spec + run')
                }}</span>
              </div>
            </article>

            <form class="action-card quick-eval-card" @submit.prevent="submitQuickEval">
              <div class="action-card-header">
                <span class="step-chip">Q</span>
                <div>
                  <h3>{{ tr('harness.quickEval.launch', 'Launch quick eval') }}</h3>
                  <p>
                    {{
                      tr(
                        'harness.quickEval.launchHint',
                        'Start from pasted cases, a past conversation, or an existing dataset. Harness fills in the object model behind the scenes.'
                      )
                    }}
                  </p>
                </div>
              </div>

              <div
                class="filter-segment source-segment"
                role="tablist"
                :aria-label="tr('harness.quickEval.source', 'Case source')"
              >
                <button
                  type="button"
                  class="segment-button"
                  :class="{ active: quickEvalForm.sourceMode === 'manifest' }"
                  @click="quickEvalForm.sourceMode = 'manifest'"
                >
                  {{ tr('harness.quickEval.pasteCases', 'Paste cases') }}
                </button>
                <button
                  type="button"
                  class="segment-button"
                  :class="{ active: quickEvalForm.sourceMode === 'dataset' }"
                  @click="quickEvalForm.sourceMode = 'dataset'"
                >
                  {{ tr('harness.quickEval.reuseDataset', 'Reuse dataset') }}
                </button>
                <button
                  type="button"
                  class="segment-button"
                  :class="{ active: quickEvalForm.sourceMode === 'conversation' }"
                  @click="quickEvalForm.sourceMode = 'conversation'"
                >
                  {{ tr('harness.quickEval.useConversation', 'Use conversation') }}
                </button>
              </div>

              <div class="form-grid">
                <label v-if="quickEvalForm.sourceMode === 'manifest'" class="form-span-2">
                  <span>{{ tr('harness.quickEval.caseManifest', 'Cases JSON') }}</span>
                  <textarea
                    v-model="quickEvalForm.manifestText"
                    name="quick-eval-manifest"
                    rows="11"
                    spellcheck="false"
                    required
                  />
                  <small class="field-hint">
                    {{
                      tr(
                        'harness.quickEval.caseTemplateHint',
                        'Template only. Paste real cases here before launching.'
                      )
                    }}
                  </small>
                </label>

                <template v-else-if="quickEvalForm.sourceMode === 'dataset'">
                  <label class="form-span-2">
                    <span>{{ tr('harness.dataset.targetDataset', 'Dataset') }}</span>
                    <select v-model="quickEvalForm.datasetID" name="quick-eval-dataset" required>
                      <option disabled value="">
                        {{ tr('harness.dataset.selectDataset', 'Select a dataset') }}
                      </option>
                      <option v-for="dataset in datasets" :key="dataset.id" :value="dataset.id">
                        {{ dataset.name }}
                      </option>
                    </select>
                  </label>
                  <div class="quick-eval-summary form-span-2">
                    <strong>{{ tr('harness.quickEval.activeVersion', 'Active version') }}</strong>
                    <span>
                      {{
                        selectedQuickDatasetVersion
                          ? `${selectedQuickDatasetVersion.version} · ${selectedQuickDatasetVersion.item_count} ${tr('harness.dataset.items', 'items')}`
                          : tr(
                              'harness.quickEval.datasetVersionRequired',
                              'Publish a dataset version before launching a quick eval.'
                            )
                      }}
                    </span>
                  </div>
                </template>

                <template v-else>
                  <label class="form-span-2">
                    <span>{{
                      tr('harness.quickEval.selectConversation', 'Select a conversation')
                    }}</span>
                    <select
                      v-model="quickEvalForm.conversationID"
                      name="quick-eval-conversation"
                      required
                    >
                      <option disabled value="">
                        {{ tr('harness.quickEval.selectConversation', 'Select a conversation') }}
                      </option>
                      <option
                        v-for="conversation in quickEvalConversations"
                        :key="conversation.id"
                        :value="conversation.id"
                      >
                        {{ quickEvalConversationTitle(conversation) }}
                      </option>
                    </select>
                    <small class="field-hint">
                      {{
                        tr(
                          'harness.quickEval.conversationHint',
                          'Generate draft cases by pairing each user message with the next assistant reply.'
                        )
                      }}
                    </small>
                  </label>
                  <div class="quick-eval-summary form-span-2">
                    <strong>{{ tr('harness.quickEval.draftCases', 'Draft cases') }}</strong>
                    <span>
                      {{
                        quickEvalConversationLoading ||
                        quickEvalConversationMessagesLoadingID === quickEvalForm.conversationID
                          ? tr('common.loading', 'Loading')
                          : quickEvalConversationError
                            ? quickEvalConversationError
                            : !quickEvalForm.conversationID
                              ? tr(
                                  'harness.quickEval.conversationRequired',
                                  'Select a conversation before launching a quick eval.'
                                )
                              : selectedQuickConversationCaseCount > 0
                                ? trp(
                                    'harness.quickEval.conversationCaseCount',
                                    '{count} draft cases from this conversation',
                                    {
                                      count: selectedQuickConversationCaseCount,
                                    }
                                  )
                                : tr(
                                    'harness.quickEval.conversationEmpty',
                                    'This conversation did not produce any draft cases yet.'
                                  )
                      }}
                    </span>
                  </div>
                  <label class="form-span-2">
                    <span>{{ tr('harness.quickEval.previewManifest', 'Preview Cases JSON') }}</span>
                    <textarea
                      :value="selectedQuickConversationManifestText"
                      name="quick-eval-conversation-preview"
                      rows="11"
                      spellcheck="false"
                      readonly
                    />
                  </label>
                  <div class="form-span-2">
                    <button
                      type="button"
                      class="secondary-button"
                      @click="copyConversationDraftToManifest"
                    >
                      {{ tr('harness.quickEval.editManifest', 'Edit Cases JSON') }}
                    </button>
                  </div>
                </template>

                <label class="form-span-2">
                  <span>{{ tr('harness.quickEval.preset', 'Preset') }}</span>
                  <select v-model="quickEvalForm.preset" name="quick-eval-preset">
                    <option v-for="preset in quickEvalPresetOptions" :key="preset" :value="preset">
                      {{ quickPresetConfig(preset).label }}
                    </option>
                  </select>
                </label>
              </div>

              <div class="quick-eval-summary">
                <strong>{{ tr('harness.quickEval.systemWillDo', 'Harness will do') }}</strong>
                <span>
                  {{
                    tr(
                      'harness.quickEval.systemWillDoHint',
                      'Create or reuse the dataset snapshot, choose a matching spec, attach the default baseline, and launch the run.'
                    )
                  }}
                </span>
                <span>{{ quickPresetDescription(quickEvalForm.preset) }}</span>
              </div>

              <button
                class="primary-button"
                type="submit"
                :disabled="
                  createAction === 'quick' ||
                  (quickEvalForm.sourceMode === 'dataset' && !quickEvalForm.datasetID) ||
                  (quickEvalForm.sourceMode === 'conversation' && !quickEvalForm.conversationID)
                "
              >
                {{
                  createAction === 'quick'
                    ? tr('common.loading', 'Loading')
                    : tr('harness.quickEval.launch', 'Launch quick eval')
                }}
              </button>
            </form>
          </div>
        </section>

        <section class="panel rail-panel">
          <div class="section-header">
            <div>
              <p class="section-eyebrow">
                {{ tr('harness.builder.eyebrow', 'Advanced Controls') }}
              </p>
              <h2>{{ tr('harness.builder.title', 'Advanced V3 object model') }}</h2>
              <p class="section-description">
                {{
                  tr(
                    'harness.builder.description',
                    'Reach for the full dataset, version, spec, and run workflow when you need exact control over every eval object.'
                  )
                }}
              </p>
            </div>
            <button
              type="button"
              class="secondary-button"
              @click="advancedBuilderOpen = !advancedBuilderOpen"
            >
              {{
                advancedBuilderOpen
                  ? tr('harness.builder.hideAdvanced', 'Hide advanced')
                  : tr('harness.builder.showAdvanced', 'Open advanced')
              }}
            </button>
          </div>

          <div v-if="advancedBuilderOpen" class="quickstart-grid">
            <form class="action-card" @submit.prevent="submitDataset">
              <div class="action-card-header">
                <span class="step-chip">1</span>
                <div>
                  <h3>{{ tr('harness.dataset.create', 'Create dataset') }}</h3>
                  <p>
                    {{
                      tr(
                        'harness.dataset.createHint',
                        'Start a reusable case collection with default run settings.'
                      )
                    }}
                  </p>
                </div>
              </div>
              <div class="form-grid">
                <label>
                  <span>{{ tr('common.name', 'Name') }}</span>
                  <input v-model="datasetForm.name" name="dataset-name" required />
                </label>
                <label>
                  <span>{{ tr('harness.dataset.subject', 'Subject') }}</span>
                  <input v-model="datasetForm.subject" name="dataset-subject" />
                </label>
                <label>
                  <span>{{ tr('harness.dataset.runKind', 'Run kind') }}</span>
                  <select v-model="datasetForm.runKind" name="dataset-run-kind">
                    <option v-for="kind in runKindOptions" :key="kind" :value="kind">
                      {{ humanizeEnum(kind) }}
                    </option>
                  </select>
                </label>
                <label>
                  <span>{{ tr('harness.dataset.profile', 'Profile') }}</span>
                  <input v-model="datasetForm.profile" name="dataset-profile" />
                </label>
                <label class="form-span-2">
                  <span>{{ tr('common.description', 'Description') }}</span>
                  <textarea v-model="datasetForm.description" name="dataset-description" rows="3" />
                </label>
              </div>
              <button class="primary-button" type="submit" :disabled="createAction === 'dataset'">
                {{
                  createAction === 'dataset'
                    ? tr('common.loading', 'Loading')
                    : tr('harness.dataset.create', 'Create dataset')
                }}
              </button>
            </form>

            <form class="action-card" @submit.prevent="submitDatasetVersion">
              <div class="action-card-header">
                <span class="step-chip">2</span>
                <div>
                  <h3>{{ tr('harness.dataset.publishVersion', 'Publish version') }}</h3>
                  <p>
                    {{
                      tr(
                        'harness.dataset.publishHint',
                        'Freeze an immutable manifest so specs always bind to a concrete snapshot.'
                      )
                    }}
                  </p>
                </div>
              </div>
              <div class="form-grid">
                <label class="form-span-2">
                  <span>{{ tr('harness.dataset.targetDataset', 'Dataset') }}</span>
                  <select
                    v-model="datasetVersionForm.datasetID"
                    name="dataset-version-dataset"
                    required
                  >
                    <option disabled value="">
                      {{ tr('harness.dataset.selectDataset', 'Select a dataset') }}
                    </option>
                    <option v-for="dataset in datasets" :key="dataset.id" :value="dataset.id">
                      {{ dataset.name }}
                    </option>
                  </select>
                </label>
                <label>
                  <span>{{ tr('harness.dataset.versionLabel', 'Version') }}</span>
                  <input
                    v-model="datasetVersionForm.version"
                    name="dataset-version-label"
                    required
                  />
                </label>
                <label>
                  <span>{{ tr('harness.dataset.sourceType', 'Source type') }}</span>
                  <input
                    v-model="datasetVersionForm.sourceType"
                    name="dataset-version-source-type"
                  />
                </label>
                <label class="form-span-2">
                  <span>{{ tr('harness.dataset.sourceRef', 'Source ref') }}</span>
                  <input v-model="datasetVersionForm.sourceRef" name="dataset-version-source-ref" />
                </label>
                <label class="form-span-2">
                  <span>{{ tr('harness.dataset.manifest', 'Manifest JSON') }}</span>
                  <textarea
                    v-model="datasetVersionForm.manifestText"
                    name="dataset-version-manifest"
                    rows="11"
                    spellcheck="false"
                    required
                  />
                </label>
              </div>
              <button
                class="primary-button"
                type="submit"
                :disabled="createAction === 'version' || !datasetVersionForm.datasetID"
              >
                {{
                  createAction === 'version'
                    ? tr('common.loading', 'Loading')
                    : tr('harness.dataset.publishVersion', 'Publish version')
                }}
              </button>
            </form>

            <form class="action-card" @submit.prevent="submitEvalSpec">
              <div class="action-card-header">
                <span class="step-chip">3</span>
                <div>
                  <h3>{{ tr('harness.evalSpec.create', 'Create eval spec') }}</h3>
                  <p>
                    {{
                      tr(
                        'harness.evalSpec.createHint',
                        'Bind one dataset snapshot to a reusable scoring and runtime template.'
                      )
                    }}
                  </p>
                </div>
              </div>
              <div class="form-grid">
                <label class="form-span-2">
                  <span>{{ tr('common.name', 'Name') }}</span>
                  <input v-model="evalSpecForm.name" name="eval-spec-name" required />
                </label>
                <label>
                  <span>{{ tr('harness.dataset.targetDataset', 'Dataset') }}</span>
                  <select v-model="evalSpecForm.datasetID" name="eval-spec-dataset" required>
                    <option disabled value="">
                      {{ tr('harness.dataset.selectDataset', 'Select a dataset') }}
                    </option>
                    <option v-for="dataset in datasets" :key="dataset.id" :value="dataset.id">
                      {{ dataset.name }}
                    </option>
                  </select>
                </label>
                <label>
                  <span>{{ tr('harness.dataset.versionLabel', 'Version') }}</span>
                  <select v-model="evalSpecForm.datasetVersionID" name="eval-spec-version" required>
                    <option disabled value="">
                      {{ tr('harness.dataset.selectVersion', 'Select a version') }}
                    </option>
                    <option
                      v-for="version in selectedSpecDatasetVersions"
                      :key="version.id"
                      :value="version.id"
                    >
                      {{ version.version }} · {{ version.item_count }}
                      {{ tr('harness.dataset.items', 'items') }}
                    </option>
                  </select>
                </label>
                <label>
                  <span>{{ tr('harness.dataset.subject', 'Subject') }}</span>
                  <input v-model="evalSpecForm.subject" name="eval-spec-subject" />
                </label>
                <label>
                  <span>{{ tr('harness.dataset.runKind', 'Run kind') }}</span>
                  <select v-model="evalSpecForm.runKind" name="eval-spec-run-kind">
                    <option v-for="kind in runKindOptions" :key="kind" :value="kind">
                      {{ humanizeEnum(kind) }}
                    </option>
                  </select>
                </label>
                <label>
                  <span>{{ tr('harness.dataset.profile', 'Profile') }}</span>
                  <input v-model="evalSpecForm.profile" name="eval-spec-profile" />
                </label>
                <label>
                  <span>{{ tr('harness.evalSpec.scoringMode', 'Scoring mode') }}</span>
                  <select v-model="evalSpecForm.scoringMode" name="eval-spec-scoring-mode">
                    <option v-for="mode in scoringModeOptions" :key="mode" :value="mode">
                      {{ humanizeEnum(mode) }}
                    </option>
                  </select>
                </label>
                <label>
                  <span>{{ tr('harness.evalSpec.passThreshold', 'Pass threshold') }}</span>
                  <input v-model="evalSpecForm.passThreshold" name="eval-spec-pass-threshold" />
                </label>
                <label>
                  <span>{{ tr('harness.evalSpec.ruleProfile', 'Rule profile') }}</span>
                  <input v-model="evalSpecForm.ruleProfile" name="eval-spec-rule-profile" />
                </label>
                <label>
                  <span>{{ tr('harness.evalSpec.judgeModel', 'Judge model') }}</span>
                  <input v-model="evalSpecForm.judgeModel" name="eval-spec-judge-model" />
                </label>
              </div>
              <button
                class="primary-button"
                type="submit"
                :disabled="
                  createAction === 'spec' ||
                  !evalSpecForm.datasetID ||
                  !evalSpecForm.datasetVersionID
                "
              >
                {{
                  createAction === 'spec'
                    ? tr('common.loading', 'Loading')
                    : tr('harness.evalSpec.create', 'Create eval spec')
                }}
              </button>
            </form>

            <form class="action-card" @submit.prevent="submitEvalRun">
              <div class="action-card-header">
                <span class="step-chip">4</span>
                <div>
                  <h3>{{ tr('harness.evalRun.launch', 'Launch eval run') }}</h3>
                  <p>
                    {{
                      tr(
                        'harness.evalRun.launchHint',
                        'Materialize the spec into a tracked run group and keep the report linked here.'
                      )
                    }}
                  </p>
                </div>
              </div>
              <div class="form-grid">
                <label class="form-span-2">
                  <span>{{ tr('harness.evalRun.spec', 'Eval spec') }}</span>
                  <select v-model="evalRunForm.evalSpecID" name="eval-run-spec" required>
                    <option disabled value="">
                      {{ tr('harness.evalRun.selectSpec', 'Select an eval spec') }}
                    </option>
                    <option v-for="spec in evalSpecs" :key="spec.id" :value="spec.id">
                      {{ spec.name }}
                    </option>
                  </select>
                </label>
                <label class="form-span-2">
                  <span>{{ tr('common.title', 'Title') }}</span>
                  <input v-model="evalRunForm.title" name="eval-run-title" />
                </label>
                <label class="form-span-2">
                  <span>{{ tr('harness.evalRun.baseline', 'Baseline run') }}</span>
                  <select v-model="evalRunForm.baselineEvalRunID" name="eval-run-baseline">
                    <option value="">{{ tr('common.notAvailable', 'Not available') }}</option>
                    <option v-for="run in baselineOptions" :key="run.id" :value="run.id">
                      {{ run.title || run.id }} · {{ statusLabel(run.status) }}
                    </option>
                  </select>
                </label>
                <label>
                  <span>{{ tr('harness.evalRun.triggerKind', 'Trigger kind') }}</span>
                  <input v-model="evalRunForm.triggerKind" name="eval-run-trigger-kind" />
                </label>
                <label>
                  <span>{{ tr('harness.evalRun.triggerRef', 'Trigger ref') }}</span>
                  <input v-model="evalRunForm.triggerRef" name="eval-run-trigger-ref" />
                </label>
              </div>
              <button
                class="primary-button"
                type="submit"
                :disabled="createAction === 'run' || !evalRunForm.evalSpecID"
              >
                {{
                  createAction === 'run'
                    ? tr('common.loading', 'Loading')
                    : tr('harness.evalRun.launch', 'Launch eval run')
                }}
              </button>
            </form>
          </div>

          <div v-else class="empty-panel">
            {{
              tr(
                'harness.builder.collapsed',
                'Advanced mode is collapsed. Open it when you need to hand-author datasets, immutable versions, eval specs, or run metadata.'
              )
            }}
          </div>
        </section>
      </aside>

      <div class="console-main">
        <div class="content-split support-stack">
          <section class="panel support-panel">
            <div class="section-header">
              <div>
                <p class="section-eyebrow">
                  {{ tr('harness.datasets.eyebrow', 'Reusable Sources') }}
                </p>
                <h2>{{ tr('harness.datasets.title', 'Datasets & versions') }}</h2>
                <p class="section-description">
                  {{
                    tr(
                      'harness.datasets.description',
                      'Each dataset can carry multiple frozen versions, letting specs rerun against the same manifest later.'
                    )
                  }}
                </p>
              </div>
            </div>

            <div v-if="datasets.length === 0" class="empty-panel">
              {{
                tr(
                  'harness.datasets.empty',
                  'No datasets yet. Create one to begin the V3 eval flow.'
                )
              }}
            </div>

            <div v-else class="entity-grid">
              <article v-for="dataset in datasets" :key="dataset.id" class="entity-card">
                <div class="entity-header">
                  <div>
                    <h3>{{ dataset.name }}</h3>
                    <p>
                      {{
                        dataset.description ||
                        dataset.subject ||
                        tr('harness.groups.noSubject', 'No subject provided')
                      }}
                    </p>
                  </div>
                  <span class="kind-chip is-eval">{{
                    humanizeEnum(dataset.default_run_kind || 'agent_task')
                  }}</span>
                </div>

                <dl class="meta-grid">
                  <div>
                    <dt>{{ tr('harness.dataset.profile', 'Profile') }}</dt>
                    <dd>
                      {{ dataset.default_profile || tr('common.notAvailable', 'Not available') }}
                    </dd>
                  </div>
                  <div>
                    <dt>{{ tr('harness.dataset.activeVersion', 'Active version') }}</dt>
                    <dd>
                      {{
                        activeVersionForDataset(dataset)?.version ||
                        dataset.active_version_id ||
                        tr('common.notAvailable', 'Not available')
                      }}
                    </dd>
                  </div>
                  <div>
                    <dt>{{ tr('harness.dataset.caseCount', 'Cases') }}</dt>
                    <dd>{{ activeVersionForDataset(dataset)?.item_count || 0 }}</dd>
                  </div>
                  <div>
                    <dt>{{ tr('common.updatedAt', 'Updated') }}</dt>
                    <dd>{{ formatDate(dataset.updated_at) }}</dd>
                  </div>
                </dl>

                <div class="version-list">
                  <button
                    v-for="version in versionsForDataset(dataset.id)"
                    :key="version.id"
                    type="button"
                    class="version-chip"
                    :class="{ active: version.id === dataset.active_version_id }"
                    @click="prefillSpecFromDataset(dataset, version)"
                  >
                    {{ version.version }} · {{ version.item_count }}
                    {{ tr('harness.dataset.items', 'items') }}
                  </button>
                </div>

                <div class="action-row">
                  <button
                    type="button"
                    class="secondary-button"
                    @click="prefillVersionFromDataset(dataset)"
                  >
                    {{ tr('harness.dataset.publishVersion', 'Publish version') }}
                  </button>
                  <button
                    type="button"
                    class="secondary-button"
                    @click="prefillSpecFromDataset(dataset, activeVersionForDataset(dataset))"
                  >
                    {{ tr('harness.evalSpec.create', 'Create eval spec') }}
                  </button>
                </div>
              </article>
            </div>
          </section>

          <section class="panel support-panel">
            <div class="section-header">
              <div>
                <p class="section-eyebrow">
                  {{ tr('harness.evalSpecs.eyebrow', 'Reusable Templates') }}
                </p>
                <h2>{{ tr('harness.evalSpecs.title', 'Eval specs') }}</h2>
                <p class="section-description">
                  {{
                    tr(
                      'harness.evalSpecs.description',
                      'Specs capture one dataset snapshot plus the run kind, profile, and scoring setup needed to materialize repeatable runs.'
                    )
                  }}
                </p>
              </div>
            </div>

            <div v-if="evalSpecs.length === 0" class="empty-panel">
              {{
                tr(
                  'harness.evalSpecs.empty',
                  'No eval specs yet. Publish a version and bind it to a spec.'
                )
              }}
            </div>

            <div v-else class="entity-list">
              <article v-for="spec in evalSpecs" :key="spec.id" class="list-card">
                <div class="list-card-main">
                  <div class="entity-header compact">
                    <div>
                      <h3>{{ spec.name }}</h3>
                      <p>
                        {{ datasetByID[spec.dataset_id]?.name || spec.dataset_id }}
                        <span class="dot-separator">·</span>
                        {{
                          datasetVersionByID[spec.dataset_version_id || '']?.version ||
                          spec.dataset_version_id ||
                          tr('common.notAvailable', 'Not available')
                        }}
                      </p>
                    </div>
                    <span
                      class="status-chip"
                      :class="statusTone(spec.scoring_config?.mode || 'pending')"
                    >
                      {{ humanizeEnum(spec.scoring_config?.mode || 'rule') }}
                    </span>
                  </div>

                  <dl class="meta-grid compact">
                    <div>
                      <dt>{{ tr('harness.dataset.runKind', 'Run kind') }}</dt>
                      <dd>{{ humanizeEnum(spec.run_kind) }}</dd>
                    </div>
                    <div>
                      <dt>{{ tr('harness.dataset.profile', 'Profile') }}</dt>
                      <dd>
                        {{ spec.profile || tr('harness.group.unprofiled', 'Unprofiled item') }}
                      </dd>
                    </div>
                    <div>
                      <dt>{{ tr('harness.evalSpec.passThreshold', 'Pass threshold') }}</dt>
                      <dd>
                        {{
                          spec.scoring_config?.pass_threshold ??
                          tr('common.notAvailable', 'Not available')
                        }}
                      </dd>
                    </div>
                    <div>
                      <dt>{{ tr('harness.evalSpec.judgeModel', 'Judge model') }}</dt>
                      <dd>
                        {{
                          spec.scoring_config?.judge_model ||
                          tr('common.notAvailable', 'Not available')
                        }}
                      </dd>
                    </div>
                  </dl>
                </div>

                <div class="action-row">
                  <button type="button" class="secondary-button" @click="prefillRunFromSpec(spec)">
                    {{ tr('harness.evalRun.launch', 'Launch eval run') }}
                  </button>
                </div>
              </article>
            </div>
          </section>
        </div>

        <section class="panel eval-runs-panel">
          <div class="section-header">
            <div>
              <p class="section-eyebrow">
                {{ tr('harness.evalRuns.eyebrow', 'Tracked Executions') }}
              </p>
              <h2>{{ tr('harness.evalRuns.title', 'Eval runs') }}</h2>
              <p class="section-description">
                {{
                  tr(
                    'harness.evalRuns.description',
                    'Run rows keep the V3 entrypoint visible while still linking back to the underlying group report and raw runtime trail.'
                  )
                }}
              </p>
            </div>
          </div>

          <div v-if="evalRuns.length === 0" class="empty-panel">
            {{
              tr(
                'harness.evalRuns.empty',
                'No eval runs yet. Launch one from a spec to populate the tracked run ledger.'
              )
            }}
          </div>

          <div v-else class="eval-runs-console">
            <div class="eval-runs-list">
              <article
                v-for="run in evalRuns"
                :key="run.id"
                class="list-card eval-run-card"
                :class="[statusTone(run.status), { selected: run.id === selectedEvalRunID }]"
                role="button"
                tabindex="0"
                @click="loadEvalRunReport(run.id)"
                @keydown.enter.prevent="loadEvalRunReport(run.id)"
                @keydown.space.prevent="loadEvalRunReport(run.id)"
              >
                <div class="list-card-main eval-run-card-main">
                  <div class="entity-header compact">
                    <div>
                      <h3>{{ run.title || run.id }}</h3>
                      <p>
                        {{ evalSpecByID[run.eval_spec_id]?.name || run.eval_spec_id }}
                        <span class="dot-separator">·</span>
                        {{
                          datasetVersionByID[run.dataset_version_id || '']?.version ||
                          run.dataset_version_id ||
                          tr('common.notAvailable', 'Not available')
                        }}
                      </p>
                    </div>
                    <span class="status-chip" :class="statusTone(run.status)">{{
                      statusLabel(run.status)
                    }}</span>
                  </div>

                  <div class="run-compact-metrics">
                    <div class="run-metric-pill">
                      <span>{{ tr('harness.groups.passRate', 'Pass rate') }}</span>
                      <strong>{{ percentLabel(passRate(run.summary)) }}</strong>
                    </div>
                    <div class="run-metric-pill">
                      <span>{{ tr('harness.groups.score', 'Score') }}</span>
                      <strong>{{ fixedScore(overallScore(run.summary)) }}</strong>
                    </div>
                    <div class="run-metric-pill">
                      <span>{{ tr('harness.evalRun.triggerKind', 'Trigger kind') }}</span>
                      <strong>{{
                        run.trigger_kind || tr('common.notAvailable', 'Not available')
                      }}</strong>
                    </div>
                    <div class="run-metric-pill">
                      <span>{{ tr('harness.group.linkedRuns', 'Linked runs') }}</span>
                      <strong>{{ run.group_id ? 1 : 0 }}</strong>
                    </div>
                  </div>

                  <div class="run-submeta">
                    <span
                      >{{ tr('common.updatedAt', 'Updated') }}:
                      {{ formatDate(run.updated_at) }}</span
                    >
                    <span>
                      {{ tr('harness.evalRun.report', 'Linked report') }}:
                      {{
                        run.group_id
                          ? tr('common.available', 'Available')
                          : tr('common.notAvailable', 'Not available')
                      }}
                    </span>
                  </div>
                </div>

                <div class="action-row run-actions">
                  <button
                    type="button"
                    class="secondary-button"
                    :disabled="reportLoadingRunID === run.id"
                    @click.stop="loadEvalRunReport(run.id)"
                  >
                    {{
                      reportLoadingRunID === run.id
                        ? tr('common.loading', 'Loading')
                        : tr('harness.evalRun.inspect', 'Inspect report')
                    }}
                  </button>
                  <RouterLink
                    v-if="run.group_id"
                    class="secondary-button link-button"
                    :to="{ name: 'HarnessGroupDetail', params: { id: run.group_id } }"
                    @click.stop
                  >
                    {{ tr('harness.group.jumpToRun', 'Open group') }}
                  </RouterLink>
                  <button
                    v-if="runStatusIsActive(run.status)"
                    type="button"
                    class="secondary-button danger-button"
                    :disabled="cancelEvalRunID === run.id"
                    @click.stop="cancelEvalRun(run.id)"
                  >
                    {{
                      cancelEvalRunID === run.id
                        ? tr('common.loading', 'Loading')
                        : tr('common.cancel', 'Cancel')
                    }}
                  </button>
                </div>
              </article>
            </div>

            <section class="report-panel docked-report-panel">
              <template v-if="selectedEvalRunReport">
                <div class="report-hero">
                  <div class="report-hero-copy">
                    <p class="section-eyebrow">
                      {{ tr('harness.evalRun.report', 'Linked report') }}
                    </p>
                    <h2>
                      {{ selectedReportRun?.title || selectedReportRun?.id || selectedEvalRunID }}
                    </h2>
                    <p class="section-description">
                      {{
                        tr(
                          'harness.evalRun.reportDescription',
                          'This preview joins the V3 eval run record with the underlying group report so you can inspect outcomes without leaving the console.'
                        )
                      }}
                    </p>
                    <div class="report-hero-meta">
                      <span
                        class="status-chip"
                        :class="statusTone(selectedReportRun?.status || 'pending')"
                      >
                        {{ statusLabel(selectedReportRun?.status || 'pending') }}
                      </span>
                      <span
                        class="kind-chip"
                        :class="kindTone(selectedEvalRunReport.eval_spec?.run_kind || 'agent_task')"
                      >
                        {{
                          humanizeEnum(selectedEvalRunReport.eval_spec?.run_kind || 'agent_task')
                        }}
                      </span>
                      <span class="version-chip">
                        {{
                          selectedEvalRunReport.dataset_version?.version ||
                          tr('common.notAvailable', 'Not available')
                        }}
                      </span>
                    </div>
                  </div>

                  <div class="report-hero-actions">
                    <div class="action-row report-actions">
                      <button
                        v-if="selectedReportRun && runStatusIsActive(selectedReportRun.status)"
                        type="button"
                        class="secondary-button danger-button"
                        :disabled="cancelEvalRunID === selectedReportRun.id"
                        @click="cancelEvalRun(selectedReportRun.id)"
                      >
                        {{
                          cancelEvalRunID === selectedReportRun.id
                            ? tr('common.loading', 'Loading')
                            : tr('common.cancel', 'Cancel')
                        }}
                      </button>
                      <RouterLink
                        v-if="selectedEvalRunReport.group_report?.group?.id"
                        class="secondary-button link-button"
                        :to="{
                          name: 'HarnessGroupDetail',
                          params: { id: selectedEvalRunReport.group_report.group.id },
                        }"
                      >
                        {{ tr('harness.group.inspectRun', 'Inspect group detail') }}
                      </RouterLink>
                    </div>
                    <p class="card-copy muted report-hero-caption">
                      {{ tr('common.updatedAt', 'Updated') }}:
                      {{ formatDate(selectedReportRun?.updated_at || null) }}
                    </p>
                  </div>
                </div>

                <div class="report-highlight-grid">
                  <article class="highlight-card">
                    <span>{{ tr('harness.groups.passRate', 'Pass rate') }}</span>
                    <strong>{{
                      percentLabel(selectedEvalRunReport.group_report?.pass_rate || 0)
                    }}</strong>
                  </article>
                  <article class="highlight-card">
                    <span>{{ tr('harness.groups.score', 'Score') }}</span>
                    <strong>{{
                      fixedScore(selectedEvalRunReport.group_report?.overall_score || 0)
                    }}</strong>
                  </article>
                  <article class="highlight-card">
                    <span>{{
                      tr('harness.group.verificationPassRate', 'Verification pass rate')
                    }}</span>
                    <strong>
                      {{
                        optionalPercentLabel(
                          selectedEvalRunReport.group_report?.group?.summary,
                          'verification_pass_rate'
                        )
                      }}
                    </strong>
                  </article>
                  <article class="highlight-card">
                    <span>
                      {{ tr('harness.group.evidenceBackedPassRate', 'Evidence-backed pass rate') }}
                    </span>
                    <strong>
                      {{
                        optionalPercentLabel(
                          selectedEvalRunReport.group_report?.group?.summary,
                          'evidence_backed_pass_rate'
                        )
                      }}
                    </strong>
                  </article>
                  <article class="highlight-card">
                    <span>{{ tr('harness.groups.itemCount', 'Items') }}</span>
                    <strong>
                      {{
                        selectedEvalRunReport.group_report?.group
                          ? groupItemCount(selectedEvalRunReport.group_report.group)
                          : 0
                      }}
                    </strong>
                  </article>
                  <article class="highlight-card">
                    <span>{{ tr('harness.group.retryRecovered', 'Retry recovered') }}</span>
                    <strong>
                      {{
                        summaryNumber(
                          selectedEvalRunReport.group_report?.group?.summary,
                          'retry_recovered_count'
                        )
                      }}
                    </strong>
                  </article>
                </div>

                <div class="report-grid">
                  <article class="detail-card detail-card-feature">
                    <h3>{{ tr('harness.dataset.snapshot', 'Dataset snapshot') }}</h3>
                    <dl class="meta-grid compact">
                      <div>
                        <dt>{{ tr('harness.dataset.targetDataset', 'Dataset') }}</dt>
                        <dd>
                          {{
                            selectedEvalRunReport.dataset?.name ||
                            tr('common.notAvailable', 'Not available')
                          }}
                        </dd>
                      </div>
                      <div>
                        <dt>{{ tr('harness.dataset.versionLabel', 'Version') }}</dt>
                        <dd>
                          {{
                            selectedEvalRunReport.dataset_version?.version ||
                            tr('common.notAvailable', 'Not available')
                          }}
                        </dd>
                      </div>
                      <div>
                        <dt>{{ tr('harness.dataset.caseCount', 'Cases') }}</dt>
                        <dd>{{ selectedEvalRunReport.dataset_version?.item_count ?? 0 }}</dd>
                      </div>
                      <div>
                        <dt>{{ tr('harness.evalRun.baseline', 'Baseline run') }}</dt>
                        <dd>
                          {{
                            selectedReportRun?.baseline_eval_run_id ||
                            tr('common.notAvailable', 'Not available')
                          }}
                        </dd>
                      </div>
                      <div>
                        <dt>{{ tr('harness.evalRun.triggerKind', 'Trigger kind') }}</dt>
                        <dd>
                          {{
                            selectedReportRun?.trigger_kind ||
                            tr('common.notAvailable', 'Not available')
                          }}
                        </dd>
                      </div>
                      <div>
                        <dt>{{ tr('harness.evalRun.triggerRef', 'Trigger ref') }}</dt>
                        <dd>
                          {{
                            selectedReportRun?.trigger_ref ||
                            tr('common.notAvailable', 'Not available')
                          }}
                        </dd>
                      </div>
                    </dl>
                  </article>

                  <article class="detail-card detail-card-feature">
                    <h3>{{ tr('harness.evalSpec.title', 'Eval spec') }}</h3>
                    <dl class="meta-grid compact">
                      <div>
                        <dt>{{ tr('common.name', 'Name') }}</dt>
                        <dd>
                          {{
                            selectedEvalRunReport.eval_spec?.name ||
                            tr('common.notAvailable', 'Not available')
                          }}
                        </dd>
                      </div>
                      <div>
                        <dt>{{ tr('harness.dataset.runKind', 'Run kind') }}</dt>
                        <dd>
                          {{
                            humanizeEnum(selectedEvalRunReport.eval_spec?.run_kind || 'agent_task')
                          }}
                        </dd>
                      </div>
                      <div>
                        <dt>{{ tr('harness.evalSpec.scoringMode', 'Scoring mode') }}</dt>
                        <dd>
                          {{
                            humanizeEnum(
                              selectedEvalRunReport.eval_spec?.scoring_config?.mode || 'rule'
                            )
                          }}
                        </dd>
                      </div>
                      <div>
                        <dt>{{ tr('harness.evalSpec.judgeModel', 'Judge model') }}</dt>
                        <dd>
                          {{
                            selectedEvalRunReport.eval_spec?.scoring_config?.judge_model ||
                            tr('common.notAvailable', 'Not available')
                          }}
                        </dd>
                      </div>
                      <div>
                        <dt>{{ tr('harness.dataset.profile', 'Profile') }}</dt>
                        <dd>
                          {{
                            selectedEvalRunReport.eval_spec?.profile ||
                            tr('common.notAvailable', 'Not available')
                          }}
                        </dd>
                      </div>
                      <div>
                        <dt>{{ tr('harness.evalSpec.passThreshold', 'Pass threshold') }}</dt>
                        <dd>
                          {{
                            selectedEvalRunReport.eval_spec?.scoring_config?.pass_threshold ??
                            tr('common.notAvailable', 'Not available')
                          }}
                        </dd>
                      </div>
                    </dl>
                  </article>

                  <article class="detail-card detail-card-wide report-outcome-card">
                    <div class="detail-card-header">
                      <div>
                        <h3>{{ tr('harness.group.outcomeTitle', 'Group outcome') }}</h3>
                        <p class="card-copy">
                          {{
                            tr(
                              'harness.group.outcomeDescription',
                              'Keep the operational result, failure labels, and remediation hints together so this selected run reads like a single report instead of scattered mini-cards.'
                            )
                          }}
                        </p>
                      </div>
                    </div>
                    <dl class="meta-grid compact">
                      <div>
                        <dt>{{ tr('harness.group.linkedRuns', 'Linked runs') }}</dt>
                        <dd>
                          {{ selectedEvalRunReport.group_report?.linked_runs?.length || 0 }}
                        </dd>
                      </div>
                      <div>
                        <dt>{{ tr('common.updatedAt', 'Updated') }}</dt>
                        <dd>
                          {{
                            formatDate(
                              selectedEvalRunReport.group_report?.group?.updated_at || null
                            )
                          }}
                        </dd>
                      </div>
                      <div>
                        <dt>{{ tr('harness.evalRun.report', 'Linked report') }}</dt>
                        <dd>
                          {{
                            selectedEvalRunReport.group_report?.group?.id ||
                            tr('common.notAvailable', 'Not available')
                          }}
                        </dd>
                      </div>
                      <div>
                        <dt>{{ tr('harness.evalRun.baseline', 'Baseline run') }}</dt>
                        <dd>
                          {{
                            selectedReportRun?.baseline_eval_run_id ||
                            tr('common.notAvailable', 'Not available')
                          }}
                        </dd>
                      </div>
                    </dl>

                    <div class="report-outcome-section">
                      <p class="card-copy">
                        {{ tr('harness.group.failureLabels', 'Failure labels') }}
                      </p>
                      <div v-if="selectedFailureLabelEntries.length" class="version-list">
                        <span
                          class="version-chip"
                          v-for="entry in selectedFailureLabelEntries"
                          :key="`failure-label-${entry.key}`"
                        >
                          {{ entry.key }} · {{ entry.value }}
                        </span>
                      </div>
                      <p v-else class="card-copy muted">
                        {{ tr('harness.group.noFailureLabels', 'No failure labels recorded.') }}
                      </p>
                    </div>

                    <div
                      v-if="selectedFailureLabelEntries.length"
                      class="comparison-column failure-hints"
                    >
                      <span class="report-subtitle">
                        {{ tr('harness.group.remediation', 'Remediation') }}
                      </span>
                      <article
                        v-for="entry in selectedFailureLabelEntries"
                        :key="`failure-hint-${entry.key}`"
                        class="comparison-item is-regression"
                      >
                        <strong>{{ entry.key }}</strong>
                        <p>
                          {{ tr('harness.group.remediation', 'Remediation') }}:
                          {{ remediationHint(entry.key) }}
                        </p>
                      </article>
                    </div>
                  </article>
                </div>

                <div class="comparison-grid">
                  <article class="detail-card detail-card-feature">
                    <h3>{{ tr('harness.baseline.title', 'Baseline registry') }}</h3>
                    <p class="card-copy">
                      {{
                        tr(
                          'harness.baseline.description',
                          'Pin the selected eval run as a reusable baseline, then compare future candidates against it from the same console.'
                        )
                      }}
                    </p>
                    <div v-if="selectedEvalRunBaselines.length" class="version-list">
                      <span
                        v-for="baseline in selectedEvalRunBaselines"
                        :key="baseline.id"
                        class="version-chip"
                        :class="{ active: baseline.is_default }"
                      >
                        {{ baseline.name
                        }}<span v-if="baseline.is_default">
                          · {{ tr('harness.baseline.default', 'Default') }}</span
                        >
                      </span>
                    </div>
                    <p v-else class="card-copy muted">
                      {{
                        tr('harness.baseline.empty', 'No baselines pinned for this eval spec yet.')
                      }}
                    </p>

                    <div class="form-grid compact-grid">
                      <label class="form-span-2">
                        <span>{{ tr('harness.baseline.name', 'Baseline name') }}</span>
                        <input v-model="inlineBaselineForm.name" name="inline-baseline-name" />
                      </label>
                      <label class="checkbox-field form-span-2">
                        <input
                          v-model="inlineBaselineForm.isDefault"
                          type="checkbox"
                          name="inline-baseline-default"
                        />
                        <span>{{
                          tr('harness.baseline.makeDefault', 'Make this the default baseline')
                        }}</span>
                      </label>
                    </div>

                    <div class="action-row">
                      <button
                        type="button"
                        class="primary-button"
                        :disabled="baselineCreateRunID === (selectedReportRun?.id || '')"
                        @click="createBaselineForSelectedRun()"
                      >
                        {{
                          baselineCreateRunID === (selectedReportRun?.id || '')
                            ? tr('common.loading', 'Loading')
                            : tr('harness.baseline.pin', 'Pin as baseline')
                        }}
                      </button>
                    </div>
                  </article>

                  <article class="detail-card detail-card-feature">
                    <h3>{{ tr('harness.compare.title', 'Compare runs') }}</h3>
                    <p class="card-copy">
                      {{
                        tr(
                          'harness.compare.description',
                          'Generate a persisted comparison report against a named baseline or another eval run from the same spec.'
                        )
                      }}
                    </p>

                    <div class="form-grid compact-grid">
                      <label class="form-span-2">
                        <span>{{ tr('harness.compare.baseline', 'Baseline') }}</span>
                        <select
                          v-model="inlineCompareForm.baselineID"
                          name="inline-compare-baseline"
                        >
                          <option value="">
                            {{ tr('common.notAvailable', 'Not available') }}
                          </option>
                          <option
                            v-for="baseline in selectedEvalRunBaselines"
                            :key="baseline.id"
                            :value="baseline.id"
                          >
                            {{ baseline.name
                            }}{{
                              baseline.is_default
                                ? ` · ${tr('harness.baseline.default', 'Default')}`
                                : ''
                            }}
                          </option>
                        </select>
                      </label>
                      <label class="form-span-2">
                        <span>{{ tr('harness.compare.baseRun', 'Fallback run') }}</span>
                        <select v-model="inlineCompareForm.baseEvalRunID" name="inline-compare-run">
                          <option value="">
                            {{ tr('common.notAvailable', 'Not available') }}
                          </option>
                          <option
                            v-for="run in selectedEvalRunCompareOptions"
                            :key="run.id"
                            :value="run.id"
                          >
                            {{ run.title || run.id }} · {{ statusLabel(run.status) }}
                          </option>
                        </select>
                        <small class="field-hint">
                          {{
                            tr(
                              'harness.compare.hint',
                              'If a baseline is selected it wins. Otherwise Harness compares against the chosen eval run or the target run’s default baseline.'
                            )
                          }}
                        </small>
                      </label>
                    </div>

                    <div class="action-row">
                      <button
                        type="button"
                        class="primary-button"
                        :disabled="compareLoadingRunID === (selectedReportRun?.id || '')"
                        @click="compareSelectedRun()"
                      >
                        {{
                          compareLoadingRunID === (selectedReportRun?.id || '')
                            ? tr('common.loading', 'Loading')
                            : tr('harness.compare.generate', 'Generate comparison')
                        }}
                      </button>
                    </div>

                    <div v-if="selectedComparisonReport" class="comparison-report">
                      <div class="report-highlight-grid comparison-highlight-grid">
                        <article class="highlight-card">
                          <span>{{ tr('harness.compare.kind', 'Compare mode') }}</span>
                          <strong>
                            {{
                              humanizeEnum(
                                String(
                                  selectedComparisonReport.summary?.comparison_kind || 'eval_run'
                                )
                              )
                            }}
                          </strong>
                        </article>
                        <article class="highlight-card">
                          <span>{{ tr('harness.evalRun.baseline', 'Baseline run') }}</span>
                          <strong>
                            {{
                              String(
                                selectedComparisonReport.summary?.baseline_name ||
                                  baselineNameByID(selectedComparisonReport.baseline_id)
                              )
                            }}
                          </strong>
                        </article>
                        <article class="highlight-card">
                          <span>{{ tr('harness.compare.scoreDelta', 'Score delta') }}</span>
                          <strong>
                            {{
                              fixedScore(
                                summaryNumber(
                                  selectedComparisonReport.summary,
                                  'overall_score_delta'
                                )
                              )
                            }}
                          </strong>
                        </article>
                        <article class="highlight-card">
                          <span>{{ tr('harness.compare.passRateDelta', 'Pass rate delta') }}</span>
                          <strong>
                            {{
                              signedPercentLabel(
                                summaryNumber(selectedComparisonReport.summary, 'pass_rate_delta')
                              )
                            }}
                          </strong>
                        </article>
                        <article class="highlight-card">
                          <span>{{ tr('harness.compare.regressions', 'Regressions') }}</span>
                          <strong>
                            {{
                              summaryNumber(selectedComparisonReport.summary, 'regression_count')
                            }}
                          </strong>
                        </article>
                        <article class="highlight-card">
                          <span>{{ tr('harness.compare.improvements', 'Improvements') }}</span>
                          <strong>
                            {{
                              summaryNumber(selectedComparisonReport.summary, 'improvement_count')
                            }}
                          </strong>
                        </article>
                      </div>

                      <dl class="meta-grid compact">
                        <div>
                          <dt>
                            {{
                              tr(
                                'harness.compare.verificationPassRateDelta',
                                'Verification pass rate delta'
                              )
                            }}
                          </dt>
                          <dd>
                            {{
                              signedPercentLabel(
                                summaryNumber(
                                  selectedComparisonReport.summary,
                                  'verification_pass_rate_delta'
                                )
                              )
                            }}
                          </dd>
                        </div>
                        <div>
                          <dt>
                            {{
                              tr(
                                'harness.compare.evidenceBackedPassRateDelta',
                                'Evidence-backed pass rate delta'
                              )
                            }}
                          </dt>
                          <dd>
                            {{
                              signedPercentLabel(
                                summaryNumber(
                                  selectedComparisonReport.summary,
                                  'evidence_backed_pass_rate_delta'
                                )
                              )
                            }}
                          </dd>
                        </div>
                        <div>
                          <dt>
                            {{ tr('harness.compare.retryRecoveredDelta', 'Retry recovered delta') }}
                          </dt>
                          <dd>
                            {{
                              signedIntegerLabel(
                                summaryNumber(
                                  selectedComparisonReport.summary,
                                  'retry_recovered_delta'
                                )
                              )
                            }}
                          </dd>
                        </div>
                      </dl>

                      <div v-if="selectedComparisonFailureLabelDeltaEntries.length">
                        <p class="card-copy">
                          {{ tr('harness.compare.failureLabelDelta', 'Failure label delta') }}
                        </p>
                        <div class="version-list">
                          <span
                            v-for="entry in selectedComparisonFailureLabelDeltaEntries"
                            :key="`failure-label-delta-${entry.key}`"
                            class="version-chip"
                          >
                            {{ entry.key }} {{ signedIntegerLabel(entry.value) }}
                          </span>
                        </div>
                      </div>
                      <p v-else class="card-copy muted">
                        {{
                          tr(
                            'harness.compare.noFailureLabelDelta',
                            'No failure label changes recorded.'
                          )
                        }}
                      </p>

                      <div class="comparison-columns">
                        <div class="comparison-column">
                          <h4>{{ tr('harness.compare.regressions', 'Regressions') }}</h4>
                          <p
                            v-if="!selectedComparisonReport.regressions?.length"
                            class="card-copy muted"
                          >
                            {{
                              tr(
                                'harness.compare.noRegressions',
                                'No regressions recorded in this comparison.'
                              )
                            }}
                          </p>
                          <article
                            v-for="entry in selectedComparisonReport.regressions || []"
                            :key="`regression-${entry.key}`"
                            class="comparison-item is-regression"
                          >
                            <strong>{{ entry.label || entry.key }}</strong>
                            <span
                              >{{ statusLabel(entry.base_verdict || entry.base_status || '') }} ->
                              {{
                                statusLabel(entry.target_verdict || entry.target_status || '')
                              }}</span
                            >
                            <p v-if="entry.base_verification || entry.target_verification">
                              {{ tr('harness.group.verification', 'Verification') }}:
                              {{ verificationLabel(entry.base_verification) }} ->
                              {{ verificationLabel(entry.target_verification) }}
                            </p>
                            <p
                              v-if="
                                hasFiniteNumber(entry.base_evidence_score) ||
                                hasFiniteNumber(entry.target_evidence_score)
                              "
                            >
                              {{ tr('harness.group.evidenceScore', 'Evidence score') }}:
                              {{
                                hasFiniteNumber(entry.base_evidence_score)
                                  ? fixedScore(Number(entry.base_evidence_score))
                                  : tr('common.notAvailable', 'Not available')
                              }}
                              ->
                              {{
                                hasFiniteNumber(entry.target_evidence_score)
                                  ? fixedScore(Number(entry.target_evidence_score))
                                  : tr('common.notAvailable', 'Not available')
                              }}
                            </p>
                            <p v-if="entry.base_failure_label || entry.target_failure_label">
                              {{ tr('harness.group.failureLabel', 'Failure label') }}:
                              {{ failureLabelText(entry.base_failure_label) }} ->
                              {{ failureLabelText(entry.target_failure_label) }}
                            </p>
                            <p v-if="entry.target_failure_label">
                              {{ tr('harness.group.remediation', 'Remediation') }}:
                              {{ remediationHint(entry.target_failure_label) }}
                            </p>
                            <p>
                              {{
                                entry.target_reason ||
                                entry.base_reason ||
                                tr('harness.group.noFailureReason', 'No failure reason recorded.')
                              }}
                            </p>
                          </article>
                        </div>

                        <div class="comparison-column">
                          <h4>{{ tr('harness.compare.improvements', 'Improvements') }}</h4>
                          <p
                            v-if="!selectedComparisonReport.improvements?.length"
                            class="card-copy muted"
                          >
                            {{
                              tr(
                                'harness.compare.noImprovements',
                                'No improvements recorded in this comparison.'
                              )
                            }}
                          </p>
                          <article
                            v-for="entry in selectedComparisonReport.improvements || []"
                            :key="`improvement-${entry.key}`"
                            class="comparison-item is-improvement"
                          >
                            <strong>{{ entry.label || entry.key }}</strong>
                            <span
                              >{{ statusLabel(entry.base_verdict || entry.base_status || '') }} ->
                              {{
                                statusLabel(entry.target_verdict || entry.target_status || '')
                              }}</span
                            >
                            <p v-if="entry.base_verification || entry.target_verification">
                              {{ tr('harness.group.verification', 'Verification') }}:
                              {{ verificationLabel(entry.base_verification) }} ->
                              {{ verificationLabel(entry.target_verification) }}
                            </p>
                            <p
                              v-if="
                                hasFiniteNumber(entry.base_evidence_score) ||
                                hasFiniteNumber(entry.target_evidence_score)
                              "
                            >
                              {{ tr('harness.group.evidenceScore', 'Evidence score') }}:
                              {{
                                hasFiniteNumber(entry.base_evidence_score)
                                  ? fixedScore(Number(entry.base_evidence_score))
                                  : tr('common.notAvailable', 'Not available')
                              }}
                              ->
                              {{
                                hasFiniteNumber(entry.target_evidence_score)
                                  ? fixedScore(Number(entry.target_evidence_score))
                                  : tr('common.notAvailable', 'Not available')
                              }}
                            </p>
                            <p v-if="entry.base_failure_label || entry.target_failure_label">
                              {{ tr('harness.group.failureLabel', 'Failure label') }}:
                              {{ failureLabelText(entry.base_failure_label) }} ->
                              {{ failureLabelText(entry.target_failure_label) }}
                            </p>
                            <p v-if="entry.target_failure_label">
                              {{ tr('harness.group.remediation', 'Remediation') }}:
                              {{ remediationHint(entry.target_failure_label) }}
                            </p>
                            <p>
                              {{
                                entry.target_reason ||
                                entry.base_reason ||
                                tr('common.notAvailable', 'Not available')
                              }}
                            </p>
                          </article>
                        </div>
                      </div>
                    </div>
                  </article>
                </div>
              </template>

              <div v-else class="empty-panel report-empty-panel">
                {{
                  tr(
                    'harness.evalRun.selectReportHint',
                    'Select an eval run to open the linked report, baseline controls, and comparison view.'
                  )
                }}
              </div>
            </section>
          </div>
        </section>

        <section class="panel groups-panel">
          <div class="section-header">
            <div>
              <p class="section-eyebrow">
                {{ tr('harness.groups.eyebrow', 'Execution Substrate') }}
              </p>
              <h2>
                {{ tr('automation.tabs.harness', 'Harness') }}
                {{ tr('harness.groups.title', 'Groups') }}
              </h2>
              <p class="section-description">
                {{
                  tr(
                    'harness.groups.controlPlaneDescription',
                    'The raw group ledger still matters for retries, scorecards, linked runs, and runtime debugging, so it stays visible alongside the higher-level V3 objects.'
                  )
                }}
              </p>
            </div>
          </div>

          <section class="toolbar">
            <div
              class="filter-segment"
              role="tablist"
              :aria-label="tr('harness.groups.filters', 'Filters')"
            >
              <button
                type="button"
                class="segment-button"
                :class="{ active: groupFilterMode === 'all' }"
                @click="groupFilterMode = 'all'"
              >
                {{ tr('common.all', 'All') }}
              </button>
              <button
                type="button"
                class="segment-button"
                :class="{ active: groupFilterMode === 'active' }"
                @click="groupFilterMode = 'active'"
              >
                {{ tr('harness.groups.activeOnly', 'Active') }}
              </button>
              <button
                type="button"
                class="segment-button"
                :class="{ active: groupFilterMode === 'terminal' }"
                @click="groupFilterMode = 'terminal'"
              >
                {{ tr('harness.groups.terminalOnly', 'Terminal') }}
              </button>
            </div>
            <label class="search-field">
              <span class="search-label">{{ tr('common.search', 'Search') }}</span>
              <input
                v-model="groupSearch"
                type="search"
                :placeholder="
                  tr('harness.groups.searchPlaceholder', 'Search title, subject, kind, or status')
                "
              />
            </label>
          </section>

          <div v-if="filteredGroups.length === 0" class="empty-panel">
            {{
              tr(
                'harness.groups.emptyDescription',
                'Groups will appear here once eval batches, experiments, or projected research runs are recorded.'
              )
            }}
          </div>

          <section v-else class="groups-grid">
            <RouterLink
              v-for="group in filteredGroups"
              :key="group.id"
              :to="{ name: 'HarnessGroupDetail', params: { id: group.id } }"
              class="group-card"
            >
              <div class="group-card-header">
                <span class="kind-chip" :class="kindTone(group.kind)">{{
                  humanizeEnum(group.kind)
                }}</span>
                <span class="status-chip" :class="statusTone(group.status)">{{
                  statusLabel(group.status)
                }}</span>
              </div>

              <h2 class="group-title">{{ group.title || group.subject || group.id }}</h2>
              <p class="group-subject">
                {{ group.subject || tr('harness.groups.noSubject', 'No subject provided') }}
              </p>

              <div class="metrics-row">
                <div class="metric">
                  <span>{{ tr('harness.groups.itemCount', 'Items') }}</span>
                  <strong>{{ groupItemCount(group) }}</strong>
                </div>
                <div class="metric">
                  <span>{{ tr('harness.groups.passRate', 'Pass rate') }}</span>
                  <strong>{{ percentLabel(passRate(group.summary)) }}</strong>
                </div>
                <div class="metric">
                  <span>{{ tr('harness.groups.score', 'Score') }}</span>
                  <strong>{{ fixedScore(overallScore(group.summary)) }}</strong>
                </div>
              </div>

              <div class="count-row">
                <span
                  >{{ tr('harness.groups.running', 'Running') }}:
                  {{ summaryCounts(group.summary).running || 0 }}</span
                >
                <span
                  >{{ tr('harness.groups.failed', 'Failed') }}:
                  {{ summaryCounts(group.summary).failed || 0 }}</span
                >
                <span
                  >{{ tr('harness.groups.passed', 'Passed') }}:
                  {{ summaryCounts(group.summary).passed || 0 }}</span
                >
              </div>

              <div class="count-row">
                <span
                  >{{ tr('harness.group.verificationPassRate', 'Verification pass rate') }}:
                  {{ optionalPercentLabel(group.summary, 'verification_pass_rate') }}</span
                >
                <span
                  >{{ tr('harness.group.evidenceBackedPassRate', 'Evidence-backed pass rate') }}:
                  {{ optionalPercentLabel(group.summary, 'evidence_backed_pass_rate') }}</span
                >
                <span
                  >{{ tr('harness.group.retryRecovered', 'Retry recovered') }}:
                  {{ summaryNumber(group.summary, 'retry_recovered_count') }}</span
                >
              </div>

              <div class="footer-row">
                <span>{{ tr('common.updatedAt', 'Updated') }}</span>
                <strong>{{ formatDate(group.updated_at) }}</strong>
              </div>
            </RouterLink>
          </section>
        </section>
      </div>
    </div>
  </div>
</template>

<style scoped>
.harness-groups-page {
  --harness-surface-radius: 0.78rem;
  --harness-control-radius: 0.62rem;
  --harness-chip-radius: 999px;
  --harness-shadow: 0 8px 18px rgba(15, 23, 42, 0.045);
  --harness-shadow-strong: 0 10px 24px rgba(15, 23, 42, 0.07);
  --harness-panel-bg: #f8fafc;
  --harness-card-bg: #ffffff;
  --harness-subsurface-bg: rgba(248, 250, 252, 0.96);
  --harness-selected-card-bg: rgba(239, 246, 255, 0.92);
  display: flex;
  flex-direction: column;
  gap: 0.7rem;
  font-size: 0.82rem;
}

.console-layout {
  display: grid;
  grid-template-columns: minmax(18rem, 24rem) minmax(0, 1fr);
  gap: 0.7rem;
  align-items: start;
}

.console-rail {
  display: flex;
  flex-direction: column;
  gap: 0.7rem;
  position: sticky;
  top: 0.7rem;
  min-width: 0;
}

.console-main {
  display: grid;
  grid-template-columns: 1fr;
  grid-template-areas:
    'runs'
    'support'
    'groups';
  gap: 0.7rem;
  min-width: 0;
  align-items: start;
}

.content-split {
  display: grid;
  grid-area: support;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.7rem;
  align-items: start;
}

.rail-panel {
  min-width: 0;
}

.support-stack,
.support-panel,
.eval-runs-panel,
.groups-panel {
  min-width: 0;
}

.eval-runs-panel {
  grid-area: runs;
}

.groups-panel {
  grid-area: groups;
}

.support-stack .entity-grid {
  grid-template-columns: 1fr;
}

.hero-card {
  display: flex;
  justify-content: space-between;
  gap: 0.85rem;
  padding: 0.82rem 0.88rem;
  border-radius: 1.05rem;
  background: var(--harness-panel-bg);
  border: 1px solid rgba(203, 213, 225, 0.96);
  box-shadow:
    0 14px 24px -26px rgba(15, 23, 42, 0.38),
    inset 0 1px 0 rgba(255, 255, 255, 0.9);
}

.hero-copy h1 {
  margin: 0.12rem 0 0.32rem;
  font-size: 1.32rem;
  line-height: 1.08;
  color: #111827;
}

.eyebrow,
.section-eyebrow {
  margin: 0;
  text-transform: uppercase;
  letter-spacing: 0.14em;
  font-size: 0.62rem;
  font-weight: 700;
  color: #0f766e;
}

.hero-description,
.section-description {
  margin: 0;
  color: #475569;
  font-size: 0.8rem;
  line-height: 1.42;
}

.hero-actions {
  display: flex;
  align-items: flex-start;
}

.refresh-button,
.segment-button,
.primary-button,
.secondary-button,
.version-chip {
  appearance: none;
  border: 0;
  cursor: pointer;
}

.refresh-button,
.primary-button {
  padding: 0.46rem 0.7rem;
  border-radius: var(--harness-control-radius);
  background: #111827;
  color: #fff;
  font-weight: 600;
  font-size: 0.76rem;
  line-height: 1.15;
}

.refresh-button:disabled,
.primary-button:disabled,
.secondary-button:disabled {
  opacity: 0.65;
  cursor: wait;
}

.secondary-button {
  padding: 0.4rem 0.62rem;
  border-radius: var(--harness-control-radius);
  background: rgba(15, 23, 42, 0.06);
  color: #0f172a;
  font-weight: 600;
  font-size: 0.74rem;
  line-height: 1.15;
  text-decoration: none;
}

.secondary-button.link-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.secondary-button.danger-button {
  background: rgba(239, 68, 68, 0.12);
  color: #b91c1c;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(6, minmax(0, 1fr));
  gap: 0.6rem;
}

.stat-card,
.state-card,
.group-card,
.action-card,
.entity-card,
.list-card,
.detail-card {
  border-radius: var(--harness-surface-radius);
  border: 1px solid rgba(15, 23, 42, 0.08);
  background: var(--harness-card-bg);
  box-shadow: var(--harness-shadow);
}

.stat-card,
.state-card {
  border-radius: 1.05rem;
  border-color: rgba(203, 213, 225, 0.96);
  background: var(--harness-card-bg);
  box-shadow:
    0 14px 24px -26px rgba(15, 23, 42, 0.38),
    inset 0 1px 0 rgba(255, 255, 255, 0.9);
}

.panel {
  border-radius: 1.05rem;
  border: 1px solid rgba(203, 213, 225, 0.96);
  background: var(--harness-panel-bg);
  box-shadow:
    0 14px 24px -26px rgba(15, 23, 42, 0.38),
    inset 0 1px 0 rgba(255, 255, 255, 0.9);
}

.panel {
  padding: 0.72rem;
}

.section-header {
  display: flex;
  justify-content: space-between;
  gap: 0.65rem;
  align-items: flex-start;
  margin-bottom: 0.65rem;
}

.section-header h2 {
  margin: 0.12rem 0 0.24rem;
  color: #0f172a;
  font-size: 0.98rem;
  line-height: 1.2;
}

.stat-card {
  padding: 0.65rem 0.72rem;
}

.stat-label {
  display: block;
  color: #64748b;
  font-size: 0.68rem;
  line-height: 1.35;
}

.stat-value {
  display: block;
  margin-top: 0.24rem;
  font-size: 1.08rem;
  line-height: 1.1;
  color: #0f172a;
}

.state-card {
  padding: 0.74rem 0.8rem;
}

.state-card.is-error {
  border-color: rgba(239, 68, 68, 0.25);
  background: rgba(254, 242, 242, 0.9);
}

.state-card h2 {
  margin: 0 0 0.35rem;
  color: #0f172a;
}

.state-card p {
  margin: 0;
  color: #475569;
}

.quickstart-grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: 0.65rem;
}

.quick-eval-layout {
  display: grid;
  grid-template-columns: 1fr;
  gap: 0.65rem;
}

.quick-eval-copy-card,
.quick-eval-card {
  height: 100%;
}

.quick-eval-summary {
  display: flex;
  flex-direction: column;
  gap: 0.18rem;
  padding: 0.52rem 0.58rem;
  border-radius: 0.62rem;
  background: var(--harness-subsurface-bg);
  border: 1px solid rgba(226, 232, 240, 0.96);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.72);
}

.quick-eval-summary strong {
  color: #0f172a;
  font-size: 0.74rem;
}

.quick-eval-summary span {
  color: #64748b;
  font-size: 0.74rem;
  line-height: 1.38;
}

.source-segment {
  width: fit-content;
}

.comparison-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(19rem, 1fr));
  gap: 0.65rem;
  margin-top: 0.65rem;
}

.action-card {
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
  padding: 0.66rem;
}

.action-card-header {
  display: flex;
  gap: 0.55rem;
  align-items: flex-start;
}

.action-card-header h3 {
  margin: 0 0 0.14rem;
  color: #0f172a;
  font-size: 0.9rem;
  line-height: 1.2;
}

.action-card-header p {
  margin: 0;
  color: #64748b;
  font-size: 0.76rem;
  line-height: 1.35;
}

.step-chip {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 1.38rem;
  height: 1.38rem;
  border-radius: 999px;
  background: rgba(14, 165, 233, 0.12);
  color: #0369a1;
  font-weight: 700;
  font-size: 0.7rem;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.55rem;
}

.form-grid.compact-grid {
  margin-top: 0.6rem;
}

.form-span-2 {
  grid-column: span 2;
}

.form-grid label {
  display: flex;
  flex-direction: column;
  gap: 0.22rem;
}

.form-grid span {
  color: #64748b;
  font-size: 0.68rem;
}

.checkbox-field {
  flex-direction: row !important;
  align-items: center;
  gap: 0.38rem !important;
}

.checkbox-field input {
  width: auto;
  margin: 0;
}

.form-grid input,
.form-grid select,
.form-grid textarea,
.search-field input {
  width: 100%;
  padding: 0.48rem 0.58rem;
  border-radius: var(--harness-control-radius);
  border: 1px solid rgba(148, 163, 184, 0.35);
  background: #fff;
  color: #0f172a;
  font: inherit;
  font-size: 0.78rem;
  line-height: 1.25;
}

.form-grid textarea {
  resize: vertical;
  min-height: 6.2rem;
}

.field-hint,
.card-copy {
  margin: 0.35rem 0 0;
  color: #64748b;
  font-size: 0.72rem;
  line-height: 1.35;
}

.card-copy.muted {
  color: #94a3b8;
}

.entity-grid,
.entity-list {
  display: grid;
  gap: 0.65rem;
}

.entity-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.entity-card,
.list-card,
.detail-card {
  padding: 0.68rem;
}

.entity-header {
  display: flex;
  justify-content: space-between;
  gap: 0.55rem;
}

.entity-header.compact {
  align-items: flex-start;
}

.entity-header h3,
.detail-card h3 {
  margin: 0 0 0.25rem;
  color: #0f172a;
  font-size: 0.86rem;
  line-height: 1.18;
}

.entity-header p {
  margin: 0;
  color: #64748b;
  font-size: 0.76rem;
  line-height: 1.35;
}

.meta-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.55rem;
  margin-top: 0.6rem;
}

.meta-grid.compact {
  margin-top: 0.55rem;
}

.meta-grid dt {
  color: #64748b;
  font-size: 0.66rem;
  line-height: 1.35;
}

.meta-grid dd {
  margin: 0.15rem 0 0;
  color: #0f172a;
  font-size: 0.76rem;
  line-height: 1.28;
  word-break: break-word;
}

.version-list,
.action-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
  margin-top: 0.6rem;
}

.version-chip {
  padding: 0.28rem 0.46rem;
  border-radius: var(--harness-chip-radius);
  background: rgba(15, 23, 42, 0.06);
  color: #334155;
  font-weight: 600;
  font-size: 0.68rem;
  line-height: 1.2;
}

.version-chip.active {
  background: rgba(16, 185, 129, 0.14);
  color: #047857;
}

.list-card {
  display: flex;
  flex-direction: column;
  gap: 0.55rem;
}

.list-card.selected {
  border-color: rgba(14, 165, 233, 0.3);
  box-shadow: 0 14px 32px rgba(14, 165, 233, 0.09);
}

.eval-run-card {
  position: relative;
  overflow: hidden;
  gap: 0.48rem;
  cursor: pointer;
  transition:
    transform 0.16s ease,
    box-shadow 0.16s ease,
    border-color 0.16s ease,
    background 0.16s ease;
}

.eval-run-card-main {
  display: flex;
  flex-direction: column;
  gap: 0.52rem;
}

.eval-run-card::before {
  content: '';
  position: absolute;
  inset: 0 auto 0 0;
  width: 0.22rem;
  background: rgba(148, 163, 184, 0.45);
}

.eval-run-card.is-success::before {
  background: rgba(34, 197, 94, 0.82);
}

.eval-run-card.is-running::before {
  background: rgba(59, 130, 246, 0.82);
}

.eval-run-card.is-warning::before {
  background: rgba(245, 158, 11, 0.82);
}

.eval-run-card.is-danger::before {
  background: rgba(239, 68, 68, 0.82);
}

.eval-run-card.selected {
  border-color: rgba(14, 165, 233, 0.34);
  background: var(--harness-selected-card-bg);
  box-shadow: 0 16px 36px rgba(14, 165, 233, 0.12);
}

.eval-run-card:hover {
  transform: translateY(-1px);
  border-color: rgba(59, 130, 246, 0.22);
  box-shadow: 0 14px 30px rgba(15, 23, 42, 0.08);
}

.eval-run-card:focus-visible {
  outline: 2px solid rgba(59, 130, 246, 0.42);
  outline-offset: 2px;
}

.run-compact-metrics {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(8.4rem, 1fr));
  gap: 0.38rem;
  margin-top: 0.1rem;
}

.run-metric-pill {
  display: flex;
  flex-direction: column;
  gap: 0.14rem;
  padding: 0.42rem 0.48rem;
  border-radius: 0.56rem;
  background: var(--harness-subsurface-bg);
  border: 1px solid rgba(226, 232, 240, 0.96);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.72);
}

.run-metric-pill span {
  color: #64748b;
  font-size: 0.62rem;
  line-height: 1.2;
}

.run-metric-pill strong {
  color: #0f172a;
  font-size: 0.72rem;
  line-height: 1.2;
  word-break: break-word;
}

.run-submeta {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem 0.6rem;
  margin-top: 0.12rem;
  padding-top: 0.48rem;
  border-top: 1px solid rgba(148, 163, 184, 0.18);
  color: #64748b;
  font-size: 0.66rem;
  line-height: 1.28;
}

.run-actions {
  margin-top: 0.1rem;
}

.dot-separator {
  margin: 0 0.2rem;
  color: #94a3b8;
}

.empty-panel {
  padding: 0.62rem;
  border-radius: 0.62rem;
  background: var(--harness-subsurface-bg);
  border: 1px solid rgba(226, 232, 240, 0.96);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.72);
  color: #475569;
  font-size: 0.78rem;
  line-height: 1.35;
}

.eval-runs-console {
  display: grid;
  grid-template-columns: minmax(19rem, 23rem) minmax(0, 1fr);
  gap: 0.85rem;
  align-items: start;
}

.eval-runs-list {
  display: grid;
  gap: 0.65rem;
  min-width: 0;
  align-content: start;
}

.docked-report-panel {
  margin-top: 0;
  padding-top: 0;
  border-top: 0;
  min-width: 0;
  position: sticky;
  top: 0.7rem;
}

.report-empty-panel {
  min-height: 10rem;
  display: flex;
  align-items: center;
  justify-content: center;
}

.report-panel {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  margin-top: 0;
  padding-top: 0;
  border-top: 0;
}

.report-hero {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.92rem 1rem;
  border-radius: 0.9rem;
  border: 1px solid rgba(14, 165, 233, 0.15);
  background: var(--harness-panel-bg);
}

.report-hero-copy {
  min-width: 0;
}

.report-hero-copy h2 {
  margin: 0.12rem 0 0.3rem;
  color: #0f172a;
  font-size: 1.28rem;
  line-height: 1.08;
}

.report-hero-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
  margin-top: 0.7rem;
}

.report-hero-actions {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  justify-content: space-between;
  gap: 0.7rem;
  min-width: 15rem;
}

.report-actions {
  margin-top: 0;
  justify-content: flex-end;
}

.report-hero-caption {
  margin-top: 0;
  text-align: end;
}

.report-highlight-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(9.6rem, 1fr));
  gap: 0.65rem;
}

.highlight-card {
  padding: 0.72rem 0.76rem;
  border-radius: 0.74rem;
  border: 1px solid rgba(226, 232, 240, 0.96);
  background: var(--harness-subsurface-bg);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.72);
  display: flex;
  flex-direction: column;
  gap: 0.24rem;
}

.highlight-card span {
  color: #64748b;
  font-size: 0.68rem;
  line-height: 1.3;
}

.highlight-card strong {
  color: #0f172a;
  font-size: 1rem;
  line-height: 1.1;
  word-break: break-word;
}

.report-grid {
  display: grid;
  grid-template-columns: repeat(12, minmax(0, 1fr));
  gap: 0.7rem;
}

.detail-card-feature {
  grid-column: span 6;
}

.detail-card-wide {
  grid-column: 1 / -1;
}

.detail-card-header {
  display: flex;
  justify-content: space-between;
  gap: 0.65rem;
  align-items: flex-start;
  margin-bottom: 0.2rem;
}

.report-outcome-card {
  background: var(--harness-subsurface-bg);
}

.report-outcome-section {
  margin-top: 0.7rem;
}

.failure-hints {
  margin-top: 0.7rem;
  padding-top: 0.7rem;
  border-top: 1px solid rgba(148, 163, 184, 0.16);
}

.report-subtitle {
  color: #0f172a;
  font-size: 0.72rem;
  font-weight: 700;
  line-height: 1.2;
}

.comparison-report {
  margin-top: 0.75rem;
  padding-top: 0.75rem;
  border-top: 1px solid rgba(148, 163, 184, 0.2);
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.comparison-highlight-grid {
  margin-top: 0;
}

.comparison-columns {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(18rem, 1fr));
  gap: 0.65rem;
  margin-top: 0;
}

.comparison-column {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.comparison-column h4 {
  margin: 0;
  color: #0f172a;
  font-size: 0.84rem;
  line-height: 1.2;
}

.comparison-item {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
  padding: 0.56rem 0.6rem;
  border-radius: 0.62rem;
}

.comparison-item strong,
.comparison-item span {
  color: #0f172a;
}

.comparison-item p {
  margin: 0;
  color: #64748b;
  font-size: 0.74rem;
  line-height: 1.35;
}

.comparison-item.is-regression {
  background: rgba(254, 242, 242, 0.95);
  border: 1px solid rgba(239, 68, 68, 0.18);
}

.comparison-item.is-improvement {
  background: rgba(240, 253, 244, 0.95);
  border: 1px solid rgba(34, 197, 94, 0.18);
}

.toolbar {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 0.65rem;
  margin-bottom: 0.65rem;
}

.filter-segment {
  display: inline-flex;
  align-items: center;
  padding: 0.14rem;
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.14);
}

.segment-button {
  padding: 0.34rem 0.56rem;
  border-radius: 999px;
  background: transparent;
  color: #334155;
  font-weight: 600;
  font-size: 0.72rem;
  line-height: 1.15;
}

.segment-button.active {
  background: #fff;
  box-shadow: 0 3px 10px rgba(15, 23, 42, 0.07);
}

.search-field {
  display: flex;
  flex-direction: column;
  gap: 0.28rem;
  min-width: min(14rem, 100%);
}

.search-label {
  font-size: 0.68rem;
  color: #64748b;
}

.groups-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.65rem;
}

.group-card {
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
  padding: 0.72rem;
  color: inherit;
  text-decoration: none;
  transition:
    transform 0.18s ease,
    box-shadow 0.18s ease,
    border-color 0.18s ease;
}

.group-card:hover {
  transform: translateY(-1px);
  border-color: rgba(16, 185, 129, 0.28);
  box-shadow: var(--harness-shadow-strong);
}

.group-card-header,
.metrics-row,
.count-row,
.footer-row {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 0.4rem;
}

.group-title {
  margin: 0;
  font-size: 0.88rem;
  line-height: 1.18;
  color: #111827;
}

.group-subject {
  margin: 0;
  color: #64748b;
  font-size: 0.76rem;
  line-height: 1.35;
}

.kind-chip,
.status-chip {
  display: inline-flex;
  align-items: center;
  padding: 0.18rem 0.44rem;
  border-radius: var(--harness-chip-radius);
  font-size: 0.6rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.03em;
}

.kind-chip.is-eval {
  background: rgba(14, 165, 233, 0.14);
  color: #0369a1;
}

.kind-chip.is-experiment {
  background: rgba(245, 158, 11, 0.16);
  color: #b45309;
}

.kind-chip.is-batch {
  background: rgba(139, 92, 246, 0.16);
  color: #6d28d9;
}

.status-chip.is-success {
  background: rgba(34, 197, 94, 0.14);
  color: #166534;
}

.status-chip.is-running {
  background: rgba(59, 130, 246, 0.14);
  color: #1d4ed8;
}

.status-chip.is-warning {
  background: rgba(245, 158, 11, 0.15);
  color: #b45309;
}

.status-chip.is-danger {
  background: rgba(239, 68, 68, 0.13);
  color: #b91c1c;
}

.status-chip.is-muted {
  background: rgba(100, 116, 139, 0.15);
  color: #475569;
}

.metric {
  display: flex;
  flex-direction: column;
  gap: 0.16rem;
}

.metric span,
.count-row,
.footer-row span {
  color: #64748b;
  font-size: 0.68rem;
  line-height: 1.3;
}

.metric strong,
.footer-row strong {
  color: #0f172a;
  font-size: 0.78rem;
  line-height: 1.15;
}

@media (max-width: 1200px) {
  .console-layout {
    grid-template-columns: 1fr;
  }

  .console-rail {
    position: static;
  }

  .console-main {
    grid-template-columns: 1fr;
    grid-template-areas:
      'runs'
      'support'
      'groups';
  }

  .content-split {
    grid-template-columns: 1fr;
  }

  .stats-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .eval-runs-console {
    grid-template-columns: 1fr;
  }

  .docked-report-panel {
    position: static;
  }

  .run-compact-metrics {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .report-hero {
    flex-direction: column;
  }

  .report-hero-actions {
    align-items: flex-start;
    min-width: 0;
  }

  .report-actions {
    justify-content: flex-start;
  }

  .report-hero-caption {
    text-align: start;
  }

  .report-grid {
    grid-template-columns: 1fr;
  }

  .comparison-grid,
  .comparison-columns {
    grid-template-columns: 1fr;
  }

  .detail-card-feature,
  .detail-card-wide {
    grid-column: 1 / -1;
  }
}

@media (max-width: 960px) {
  .stats-grid,
  .eval-runs-console,
  .quick-eval-layout,
  .quickstart-grid,
  .comparison-grid,
  .entity-grid,
  .groups-grid,
  .form-grid,
  .meta-grid {
    grid-template-columns: 1fr;
  }

  .run-compact-metrics {
    grid-template-columns: 1fr;
  }

  .form-span-2 {
    grid-column: span 1;
  }

  .hero-card,
  .section-header,
  .entity-header {
    flex-direction: column;
  }

  .hero-actions {
    align-items: stretch;
  }

  .refresh-button,
  .primary-button {
    width: 100%;
  }
}
</style>
