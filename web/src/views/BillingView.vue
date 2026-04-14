<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

import {
  billingApi,
  type BillingGroupBy,
  type BillingSummaryBreakdown,
  type BillingSummaryResponse,
  type BillingLinesResponse,
} from '@/api/billing'
import ResourceChart from '@/components/ResourceChart.vue'
import { providerPoolApi, type Provider } from '@/api/providerPool'
import { filterProvidersVisibleInUI } from '@/utils/providerVisibility'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
let querySyncing = false

const loading = ref(false)
const exporting = ref(false)
const featureDisabled = ref(false)
const error = ref<string | null>(null)

const providers = ref<Provider[]>([])
const summary = ref<BillingSummaryResponse | null>(null)
const daySummary = ref<BillingSummaryResponse | null>(null)
const lines = ref<BillingLinesResponse | null>(null)

const fromDate = ref(defaultDateOffset(-7))
const toDate = ref(defaultDateOffset(0))
const providerId = ref('')
const modelId = ref('')
const groupBy = ref<BillingGroupBy>('day')
const page = ref(1)
const pageSize = ref(20)

const totalPages = computed(() => {
  const total = lines.value?.total ?? 0
  return Math.max(1, Math.ceil(total / pageSize.value))
})

const successRate = computed(() => {
  const totalRequests = summary.value?.totals.request_count ?? 0
  if (totalRequests <= 0) return 0
  const successCount = summary.value?.totals.success_count ?? 0
  return (successCount / totalRequests) * 100
})

const hasRows = computed(() => (lines.value?.items?.length ?? 0) > 0)

const tokenTrendData = computed(() => {
  const buckets = daySummary.value?.breakdown ?? []
  return buckets
    .slice()
    .filter((item) => !!item.day)
    .sort((a, b) => String(a.day).localeCompare(String(b.day)))
    .map((item) => ({
      timestamp: item.day || item.key,
      value: item.total_tokens || 0,
    }))
})

const costTrendData = computed(() => {
  const buckets = daySummary.value?.breakdown ?? []
  return buckets
    .slice()
    .filter((item) => !!item.day)
    .sort((a, b) => String(a.day).localeCompare(String(b.day)))
    .map((item) => ({
      timestamp: item.day || item.key,
      value: item.estimated_cost || 0,
    }))
})

interface BillingAnomaly {
  day: string
  totalTokens: number
  estimatedCost: number
  requestCount: number
  baselineTokens: number
  spikeRatio: number
}

interface AnomalyDrilldownData {
  providers: BillingSummaryBreakdown[]
  models: BillingSummaryBreakdown[]
}

const anomalyLookbackDays = ref(7)
const anomalyMinTokens = ref(50000)
const anomalyMinRatio = ref(2.5)
const anomalyLimit = ref(8)

const selectedAnomalyDay = ref('')
const anomalyDrilldownLoading = ref(false)
const anomalyDrilldownError = ref<string | null>(null)
const anomalyProviderBreakdown = ref<BillingSummaryBreakdown[]>([])
const anomalyModelBreakdown = ref<BillingSummaryBreakdown[]>([])
const anomalyDrilldownCache = ref<Record<string, AnomalyDrilldownData>>({})
const pendingAnomalyDay = ref('')

const anomalies = computed<BillingAnomaly[]>(() => {
  const buckets = (daySummary.value?.breakdown ?? [])
    .slice()
    .filter((item) => !!item.day)
    .sort((a, b) => String(a.day).localeCompare(String(b.day)))

  const lookbackDays = Math.max(3, Math.round(anomalyLookbackDays.value || 7))
  const minTokens = Math.max(0, Math.round(anomalyMinTokens.value || 0))
  const minRatio = Math.max(1, anomalyMinRatio.value || 1)
  const minLookbackPoints = Math.min(3, lookbackDays)
  if (buckets.length < minLookbackPoints + 1) return []

  const out: BillingAnomaly[] = []
  for (let i = 0; i < buckets.length; i++) {
    const current = buckets[i]
    if (!current) continue
    const lookback = buckets.slice(Math.max(0, i - lookbackDays), i)
    if (lookback.length < minLookbackPoints) continue

    const baseline = lookback.reduce((sum, b) => sum + (b.total_tokens || 0), 0) / lookback.length
    const currentTokens = current.total_tokens || 0
    if (baseline <= 0 || currentTokens <= 0) continue

    const ratio = currentTokens / baseline
    // Significant usage spike: enough absolute volume + relative jump.
    if (currentTokens >= minTokens && ratio >= minRatio) {
      out.push({
        day: current.day || current.key,
        totalTokens: currentTokens,
        estimatedCost: current.estimated_cost || 0,
        requestCount: current.request_count || 0,
        baselineTokens: baseline,
        spikeRatio: ratio,
      })
    }
  }

  const limit = Math.max(1, Math.round(anomalyLimit.value || 8))
  return out.sort((a, b) => b.spikeRatio - a.spikeRatio).slice(0, limit)
})

onMounted(async () => {
  hydrateStateFromQuery()
  await Promise.all([loadProviders(), loadData()])
  if (
    pendingAnomalyDay.value &&
    anomalies.value.some((item) => item.day === pendingAnomalyDay.value)
  ) {
    await analyzeAnomaly(pendingAnomalyDay.value)
  }
  await syncStateToQuery()
})

watch(anomalies, (items) => {
  if (!selectedAnomalyDay.value) return
  if (!items.some((item) => item.day === selectedAnomalyDay.value)) {
    selectedAnomalyDay.value = ''
    anomalyDrilldownError.value = null
    anomalyProviderBreakdown.value = []
    anomalyModelBreakdown.value = []
    void syncStateToQuery()
  }
})

watch([anomalyLookbackDays, anomalyMinTokens, anomalyMinRatio, anomalyLimit], () => {
  void syncStateToQuery()
})

function defaultDateOffset(days: number): string {
  const d = new Date()
  d.setDate(d.getDate() + days)
  const year = d.getFullYear()
  const month = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

function buildQuery() {
  return {
    from: fromDate.value || undefined,
    to: toDate.value || undefined,
    provider_id: providerId.value || undefined,
    model_id: modelId.value || undefined,
  }
}

function queryString(name: string): string {
  const value = route.query[name]
  if (Array.isArray(value)) return value[0] || ''
  return typeof value === 'string' ? value : ''
}

function parsePositiveInt(raw: string, fallback: number): number {
  const n = Number.parseInt(raw, 10)
  if (!Number.isFinite(n) || n <= 0) return fallback
  return n
}

function parsePositiveNumber(raw: string, fallback: number): number {
  const n = Number.parseFloat(raw)
  if (!Number.isFinite(n) || n <= 0) return fallback
  return n
}

function hydrateStateFromQuery() {
  const from = queryString('from')
  const to = queryString('to')
  const provider = queryString('provider_id')
  const model = queryString('model_id')
  const group = queryString('group_by')
  const pageRaw = queryString('page')
  const pageSizeRaw = queryString('page_size')

  const anLookback = queryString('an_lb')
  const anMinTokens = queryString('an_min_tokens')
  const anMinRatio = queryString('an_min_ratio')
  const anLimit = queryString('an_limit')
  const anDay = queryString('an_day')

  if (/^\d{4}-\d{2}-\d{2}$/.test(from)) fromDate.value = from
  if (/^\d{4}-\d{2}-\d{2}$/.test(to)) toDate.value = to
  if (provider) providerId.value = provider
  if (model) modelId.value = model
  if (group === 'day' || group === 'provider' || group === 'model') groupBy.value = group

  page.value = parsePositiveInt(pageRaw, page.value)
  pageSize.value = parsePositiveInt(pageSizeRaw, pageSize.value)
  anomalyLookbackDays.value = parsePositiveInt(anLookback, anomalyLookbackDays.value)
  anomalyMinTokens.value = parsePositiveInt(anMinTokens, anomalyMinTokens.value)
  anomalyMinRatio.value = parsePositiveNumber(anMinRatio, anomalyMinRatio.value)
  anomalyLimit.value = parsePositiveInt(anLimit, anomalyLimit.value)
  if (/^\d{4}-\d{2}-\d{2}$/.test(anDay)) pendingAnomalyDay.value = anDay
}

async function syncStateToQuery() {
  if (querySyncing) return

  const query: Record<string, string> = {
    from: fromDate.value,
    to: toDate.value,
  }
  if (providerId.value) query.provider_id = providerId.value
  if (modelId.value) query.model_id = modelId.value
  if (groupBy.value !== 'day') query.group_by = groupBy.value
  if (page.value > 1) query.page = String(page.value)
  if (pageSize.value !== 20) query.page_size = String(pageSize.value)
  if (anomalyLookbackDays.value !== 7) query.an_lb = String(anomalyLookbackDays.value)
  if (anomalyMinTokens.value !== 50000) query.an_min_tokens = String(anomalyMinTokens.value)
  if (Math.abs(anomalyMinRatio.value - 2.5) > 0.00001)
    query.an_min_ratio = String(anomalyMinRatio.value)
  if (anomalyLimit.value !== 8) query.an_limit = String(anomalyLimit.value)
  if (selectedAnomalyDay.value) query.an_day = selectedAnomalyDay.value

  querySyncing = true
  try {
    await router.replace({ query })
  } finally {
    querySyncing = false
  }
}

function resetAnomalyState(clearCache: boolean) {
  selectedAnomalyDay.value = ''
  anomalyDrilldownError.value = null
  anomalyProviderBreakdown.value = []
  anomalyModelBreakdown.value = []
  if (clearCache) {
    anomalyDrilldownCache.value = {}
  }
}

async function loadProviders() {
  try {
    const response = await providerPoolApi.listProviders()
    providers.value = filterProvidersVisibleInUI(response.data.providers ?? [])
  } catch {
    providers.value = []
  }
}

async function loadData() {
  try {
    loading.value = true
    error.value = null

    const baseQuery = buildQuery()
    const summaryReq = billingApi.getSummary({
      ...baseQuery,
      group_by: groupBy.value,
    })
    const daySummaryReq =
      groupBy.value === 'day'
        ? summaryReq
        : billingApi.getSummary({
            ...baseQuery,
            group_by: 'day',
          })
    const linesReq = billingApi.getLines({
      ...baseQuery,
      page: page.value,
      page_size: pageSize.value,
    })

    const [summaryResp, dayResp, linesResp] = await Promise.all([
      summaryReq,
      daySummaryReq,
      linesReq,
    ])

    const summaryData = summaryResp.data as BillingSummaryResponse & { enabled?: boolean }
    if (summaryData.enabled === false) {
      featureDisabled.value = true
      summary.value = null
      daySummary.value = null
      lines.value = null
      return
    }

    featureDisabled.value = false
    summary.value = summaryData
    daySummary.value = dayResp.data as BillingSummaryResponse
    lines.value = linesResp.data
    await syncStateToQuery()
  } catch (e) {
    featureDisabled.value = false
    summary.value = null
    daySummary.value = null
    lines.value = null
    resetAnomalyState(true)
    error.value = e instanceof Error ? e.message : t('billing.loadFailed')
  } finally {
    loading.value = false
  }
}

const selectedAnomaly = computed(
  () => anomalies.value.find((item) => item.day === selectedAnomalyDay.value) || null
)

function toDayStartRFC3339(day: string): string {
  return `${day}T00:00:00Z`
}

function toNextDayStartRFC3339(day: string): string {
  const d = new Date(`${day}T00:00:00Z`)
  d.setUTCDate(d.getUTCDate() + 1)
  const year = d.getUTCFullYear()
  const month = String(d.getUTCMonth() + 1).padStart(2, '0')
  const nextDay = String(d.getUTCDate()).padStart(2, '0')
  return `${year}-${month}-${nextDay}T00:00:00Z`
}

function topBreakdownByCost(
  items: BillingSummaryBreakdown[],
  limit = 5
): BillingSummaryBreakdown[] {
  return items
    .slice()
    .sort((a, b) => (b.estimated_cost || 0) - (a.estimated_cost || 0))
    .slice(0, limit)
}

async function analyzeAnomaly(day: string) {
  selectedAnomalyDay.value = day
  anomalyDrilldownError.value = null
  await syncStateToQuery()

  const cacheKey = `${day}|${fromDate.value}|${toDate.value}|${providerId.value || '*'}|${modelId.value || '*'}`
  const cache = anomalyDrilldownCache.value[cacheKey]
  if (cache) {
    anomalyProviderBreakdown.value = cache.providers
    anomalyModelBreakdown.value = cache.models
    return
  }

  try {
    anomalyDrilldownLoading.value = true
    const baseQuery = buildQuery()
    const from = toDayStartRFC3339(day)
    const to = toNextDayStartRFC3339(day)

    const providerReq = billingApi.getSummary({
      ...baseQuery,
      from,
      to,
      group_by: 'provider',
    })
    const modelReq = billingApi.getSummary({
      ...baseQuery,
      from,
      to,
      group_by: 'model',
    })

    const [providerResp, modelResp] = await Promise.all([providerReq, modelReq])
    const providersTop = topBreakdownByCost(providerResp.data.breakdown ?? [])
    const modelsTop = topBreakdownByCost(modelResp.data.breakdown ?? [])

    anomalyProviderBreakdown.value = providersTop
    anomalyModelBreakdown.value = modelsTop
    anomalyDrilldownCache.value[cacheKey] = {
      providers: providersTop,
      models: modelsTop,
    }
  } catch (e) {
    anomalyProviderBreakdown.value = []
    anomalyModelBreakdown.value = []
    anomalyDrilldownError.value = e instanceof Error ? e.message : t('billing.loadFailed')
  } finally {
    anomalyDrilldownLoading.value = false
  }
}

function percent(part: number, total: number): string {
  if (!total || total <= 0) return '0.0%'
  return `${((part / total) * 100).toFixed(1)}%`
}

async function applyDrilldownFilter(kind: 'provider' | 'model', value: string) {
  if (!value) return
  if (kind === 'provider') {
    providerId.value = value
    modelId.value = ''
  } else {
    modelId.value = value
  }
  page.value = 1
  resetAnomalyState(true)
  await loadData()
}

async function applyFilters() {
  page.value = 1
  resetAnomalyState(true)
  await loadData()
}

async function resetFilters() {
  fromDate.value = defaultDateOffset(-7)
  toDate.value = defaultDateOffset(0)
  providerId.value = ''
  modelId.value = ''
  groupBy.value = 'day'
  pageSize.value = 20
  page.value = 1
  anomalyLookbackDays.value = 7
  anomalyMinTokens.value = 50000
  anomalyMinRatio.value = 2.5
  anomalyLimit.value = 8
  resetAnomalyState(true)
  await loadData()
}

async function setPage(nextPage: number) {
  if (nextPage < 1 || nextPage > totalPages.value || nextPage === page.value) return
  page.value = nextPage
  await loadData()
}

async function onPageSizeChange() {
  page.value = 1
  await loadData()
}

async function exportCSV() {
  try {
    exporting.value = true
    const response = await billingApi.exportCSV(buildQuery())
    const blob =
      response.data instanceof Blob
        ? response.data
        : new Blob([response.data], { type: 'text/csv; charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `billing_${fromDate.value || 'from'}_${toDate.value || 'to'}.csv`
    a.click()
    URL.revokeObjectURL(url)
  } catch (e) {
    error.value = e instanceof Error ? e.message : t('billing.loadFailed')
  } finally {
    exporting.value = false
  }
}

function formatInteger(v: number): string {
  return new Intl.NumberFormat().format(v || 0)
}

function formatCurrency(v: number): string {
  return `$${(v || 0).toFixed(4)}`
}

function formatDateTime(v: string): string {
  if (!v) return '-'
  return new Date(v).toLocaleString()
}

function formatGroupLabel(item: BillingSummaryBreakdown): string {
  if (groupBy.value === 'provider') return item.provider_id || item.key
  if (groupBy.value === 'model') return item.model_id || item.key
  return item.day || item.key
}
</script>

<template>
  <div class="p-4 sm:p-6 max-w-7xl mx-auto">
    <div class="flex items-start justify-between gap-3 mb-6">
      <div>
        <h1 class="text-2xl font-bold text-gray-900 dark:text-white">
          {{ t('billing.title') }}
        </h1>
        <p class="text-sm text-gray-500 dark:text-gray-400 mt-1">
          {{ t('billing.description') }}
        </p>
      </div>
      <button
        class="px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg text-sm transition-colors disabled:opacity-50"
        :disabled="exporting || featureDisabled"
        @click="exportCSV"
      >
        {{ exporting ? t('common.loading') : t('billing.exportCSV') }}
      </button>
    </div>

    <div class="glass-card p-4 mb-6">
      <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-6 gap-3">
        <div>
          <label class="block text-xs text-gray-500 dark:text-gray-400 mb-1">{{
            t('billing.filters.from')
          }}</label>
          <input
            v-model="fromDate"
            type="date"
            class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-3 py-2 border border-gray-200 dark:border-slate-600"
          >
        </div>
        <div>
          <label class="block text-xs text-gray-500 dark:text-gray-400 mb-1">{{
            t('billing.filters.to')
          }}</label>
          <input
            v-model="toDate"
            type="date"
            class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-3 py-2 border border-gray-200 dark:border-slate-600"
          >
        </div>
        <div>
          <label class="block text-xs text-gray-500 dark:text-gray-400 mb-1">{{
            t('billing.filters.provider')
          }}</label>
          <select
            v-model="providerId"
            class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-3 py-2 border border-gray-200 dark:border-slate-600"
          >
            <option value="">
              {{ t('common.allSources') }}
            </option>
            <option
              v-for="p in providers"
              :key="p.id"
              :value="p.id"
            >
              {{ p.name }} ({{ p.id }})
            </option>
          </select>
        </div>
        <div>
          <label class="block text-xs text-gray-500 dark:text-gray-400 mb-1">{{
            t('billing.filters.model')
          }}</label>
          <input
            v-model.trim="modelId"
            type="text"
            placeholder="gpt-5"
            class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-3 py-2 border border-gray-200 dark:border-slate-600"
          >
        </div>
        <div>
          <label class="block text-xs text-gray-500 dark:text-gray-400 mb-1">{{
            t('billing.filters.groupBy')
          }}</label>
          <select
            v-model="groupBy"
            class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-3 py-2 border border-gray-200 dark:border-slate-600"
          >
            <option value="day">
              {{ t('billing.filters.groupDay') }}
            </option>
            <option value="provider">
              {{ t('billing.filters.groupProvider') }}
            </option>
            <option value="model">
              {{ t('billing.filters.groupModel') }}
            </option>
          </select>
        </div>
        <div>
          <label class="block text-xs text-gray-500 dark:text-gray-400 mb-1">{{
            t('billing.filters.pageSize')
          }}</label>
          <select
            v-model.number="pageSize"
            class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-3 py-2 border border-gray-200 dark:border-slate-600"
            @change="onPageSizeChange"
          >
            <option :value="20">
              20
            </option>
            <option :value="50">
              50
            </option>
            <option :value="100">
              100
            </option>
          </select>
        </div>
      </div>
      <div class="flex gap-2 mt-4">
        <button
          class="px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg text-sm transition-colors"
          @click="applyFilters"
        >
          {{ t('billing.apply') }}
        </button>
        <button
          class="px-4 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-900 dark:text-white rounded-lg text-sm transition-colors"
          @click="resetFilters"
        >
          {{ t('billing.reset') }}
        </button>
      </div>
    </div>

    <div
      v-if="error"
      class="mb-4 p-3 rounded-lg bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-300 text-sm"
    >
      {{ error }}
    </div>

    <div
      v-if="featureDisabled"
      class="glass-card p-5 text-sm text-gray-600 dark:text-gray-300"
    >
      {{ t('billing.featureDisabled') }}
    </div>

    <template v-else>
      <div
        v-if="summary"
        class="grid grid-cols-2 lg:grid-cols-3 xl:grid-cols-6 gap-4 mb-6"
      >
        <div class="glass-card p-4">
          <div class="text-sm text-gray-500 dark:text-gray-400">
            {{ t('billing.totals.estimatedCost') }}
          </div>
          <div class="text-2xl font-semibold text-gray-900 dark:text-white mt-1">
            {{ formatCurrency(summary.totals.estimated_cost) }}
          </div>
        </div>
        <div class="glass-card p-4">
          <div class="text-sm text-gray-500 dark:text-gray-400">
            {{ t('billing.totals.requests') }}
          </div>
          <div class="text-2xl font-semibold text-gray-900 dark:text-white mt-1">
            {{ formatInteger(summary.totals.request_count) }}
          </div>
        </div>
        <div class="glass-card p-4">
          <div class="text-sm text-gray-500 dark:text-gray-400">
            {{ t('billing.totals.totalTokens') }}
          </div>
          <div class="text-2xl font-semibold text-gray-900 dark:text-white mt-1">
            {{ formatInteger(summary.totals.total_tokens) }}
          </div>
        </div>
        <div class="glass-card p-4">
          <div class="text-sm text-gray-500 dark:text-gray-400">
            {{ t('billing.totals.successRate') }}
          </div>
          <div class="text-2xl font-semibold text-gray-900 dark:text-white mt-1">
            {{ successRate.toFixed(2) }}%
          </div>
        </div>
        <div class="glass-card p-4">
          <div class="text-sm text-gray-500 dark:text-gray-400">
            {{ t('billing.totals.cacheReadTokens') }}
          </div>
          <div class="text-2xl font-semibold text-gray-900 dark:text-white mt-1">
            {{ formatInteger(summary.totals.cache_read_tokens) }}
          </div>
        </div>
        <div class="glass-card p-4">
          <div class="text-sm text-gray-500 dark:text-gray-400">
            {{ t('billing.totals.cacheWriteTokens') }}
          </div>
          <div class="text-2xl font-semibold text-gray-900 dark:text-white mt-1">
            {{ formatInteger(summary.totals.cache_write_tokens) }}
          </div>
        </div>
      </div>

      <div class="grid grid-cols-1 xl:grid-cols-2 gap-4 mb-6">
        <ResourceChart
          :title="t('billing.trends.tokens')"
          :data="tokenTrendData"
          color="blue"
          :format-value="(v: number) => formatInteger(Math.round(v))"
        />
        <ResourceChart
          :title="t('billing.trends.cost')"
          :data="costTrendData"
          color="green"
          :format-value="(v: number) => formatCurrency(v)"
        />
      </div>

      <div class="glass-card p-4 mb-6">
        <h2 class="text-base font-semibold text-gray-900 dark:text-white mb-1">
          {{ t('billing.anomalyTitle') }}
        </h2>
        <p class="text-xs text-gray-500 dark:text-gray-400 mb-3">
          {{ t('billing.anomalyHint') }}
        </p>

        <div class="grid grid-cols-1 md:grid-cols-4 gap-3 mb-4">
          <div>
            <label class="block text-xs text-gray-500 dark:text-gray-400 mb-1">{{
              t('billing.anomalyControls.lookbackDays')
            }}</label>
            <select
              v-model.number="anomalyLookbackDays"
              class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-3 py-2 border border-gray-200 dark:border-slate-600"
            >
              <option :value="3">
                3
              </option>
              <option :value="7">
                7
              </option>
              <option :value="14">
                14
              </option>
              <option :value="30">
                30
              </option>
            </select>
          </div>
          <div>
            <label class="block text-xs text-gray-500 dark:text-gray-400 mb-1">{{
              t('billing.anomalyControls.minTokens')
            }}</label>
            <input
              v-model.number="anomalyMinTokens"
              type="number"
              min="0"
              step="1000"
              class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-3 py-2 border border-gray-200 dark:border-slate-600"
            >
          </div>
          <div>
            <label class="block text-xs text-gray-500 dark:text-gray-400 mb-1">{{
              t('billing.anomalyControls.minRatio')
            }}</label>
            <input
              v-model.number="anomalyMinRatio"
              type="number"
              min="1"
              step="0.1"
              class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-3 py-2 border border-gray-200 dark:border-slate-600"
            >
          </div>
          <div>
            <label class="block text-xs text-gray-500 dark:text-gray-400 mb-1">{{
              t('billing.anomalyControls.maxAlerts')
            }}</label>
            <select
              v-model.number="anomalyLimit"
              class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-3 py-2 border border-gray-200 dark:border-slate-600"
            >
              <option :value="5">
                5
              </option>
              <option :value="8">
                8
              </option>
              <option :value="12">
                12
              </option>
              <option :value="20">
                20
              </option>
            </select>
          </div>
        </div>

        <div
          v-if="anomalies.length === 0"
          class="text-sm text-gray-500 dark:text-gray-400 py-2"
        >
          {{ t('billing.anomalyNone') }}
        </div>

        <div
          v-else
          class="overflow-x-auto"
        >
          <table class="w-full text-sm">
            <thead>
              <tr
                class="billing-table-head text-gray-500 dark:text-gray-400 border-b border-gray-200 dark:border-gray-700"
              >
                <th class="billing-cell-end py-2">
                  {{ t('billing.anomalyTable.day') }}
                </th>
                <th class="billing-cell-end py-2">
                  {{ t('billing.anomalyTable.tokens') }}
                </th>
                <th class="billing-cell-end py-2">
                  {{ t('billing.anomalyTable.baseline') }}
                </th>
                <th class="billing-cell-end py-2">
                  {{ t('billing.anomalyTable.spike') }}
                </th>
                <th class="billing-cell-end py-2">
                  {{ t('billing.anomalyTable.requests') }}
                </th>
                <th class="billing-cell-end py-2">
                  {{ t('billing.anomalyTable.cost') }}
                </th>
                <th class="billing-cell-end py-2">
                  {{ t('billing.anomalyTable.action') }}
                </th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="item in anomalies"
                :key="item.day"
                class="border-b border-gray-100 dark:border-gray-800 text-gray-900 dark:text-gray-100"
                :class="
                  selectedAnomalyDay === item.day ? 'bg-orange-50/60 dark:bg-orange-900/10' : ''
                "
              >
                <td class="billing-cell-end py-2 whitespace-nowrap">
                  {{ item.day }}
                </td>
                <td class="billing-cell-end py-2">
                  {{ formatInteger(item.totalTokens) }}
                </td>
                <td class="billing-cell-end py-2">
                  {{ formatInteger(Math.round(item.baselineTokens)) }}
                </td>
                <td class="billing-cell-end py-2">
                  <span
                    class="px-2 py-0.5 rounded-full text-xs bg-orange-100 dark:bg-orange-900/30 text-orange-700 dark:text-orange-300"
                  >
                    {{ item.spikeRatio.toFixed(2) }}x
                  </span>
                </td>
                <td class="billing-cell-end py-2">
                  {{ formatInteger(item.requestCount) }}
                </td>
                <td class="billing-cell-end py-2">
                  {{ formatCurrency(item.estimatedCost) }}
                </td>
                <td class="billing-cell-end py-2">
                  <button
                    class="px-2.5 py-1 rounded text-xs bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-900 dark:text-white transition-colors"
                    @click="analyzeAnomaly(item.day)"
                  >
                    {{ t('billing.anomalyTable.actionAnalyze') }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div
          v-if="selectedAnomalyDay"
          class="mt-4 border-t border-gray-200 dark:border-gray-700 pt-4"
        >
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white mb-1">
            {{ t('billing.drilldownTitle') }}: {{ selectedAnomalyDay }}
          </h3>
          <p class="text-xs text-gray-500 dark:text-gray-400 mb-3">
            {{ t('billing.drilldownHint') }}
          </p>

          <div
            v-if="anomalyDrilldownLoading"
            class="text-sm text-gray-500 dark:text-gray-400 py-2"
          >
            {{ t('billing.drilldownLoading') }}
          </div>
          <div
            v-else-if="anomalyDrilldownError"
            class="text-sm text-red-600 dark:text-red-300 py-2"
          >
            {{ anomalyDrilldownError }}
          </div>
          <div
            v-else
            class="grid grid-cols-1 xl:grid-cols-2 gap-4"
          >
            <div class="rounded-lg border border-gray-200 dark:border-gray-700 p-3">
              <h4 class="text-sm font-medium text-gray-900 dark:text-white mb-2">
                {{ t('billing.drilldownProvider') }}
              </h4>
              <div
                v-if="anomalyProviderBreakdown.length === 0"
                class="text-xs text-gray-500 dark:text-gray-400"
              >
                {{ t('billing.anomalyNone') }}
              </div>
              <table
                v-else
                class="w-full text-xs"
              >
                <thead>
                  <tr
                    class="billing-table-head text-gray-500 dark:text-gray-400 border-b border-gray-200 dark:border-gray-700"
                  >
                    <th class="billing-cell-end-sm py-1">
                      {{ t('billing.table.provider') }}
                    </th>
                    <th class="billing-cell-end-sm py-1">
                      {{ t('billing.table.totalTokens') }}
                    </th>
                    <th class="billing-cell-end-sm py-1">
                      {{ t('billing.table.estimatedCost') }}
                    </th>
                    <th class="billing-cell-end-sm py-1">
                      {{ t('billing.drilldownTable.tokenShare') }}
                    </th>
                    <th class="billing-cell-end-sm py-1">
                      {{ t('billing.drilldownTable.costShare') }}
                    </th>
                    <th class="billing-cell-end-sm py-1">
                      {{ t('billing.anomalyTable.action') }}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="item in anomalyProviderBreakdown"
                    :key="item.key"
                    class="border-b border-gray-100 dark:border-gray-800"
                  >
                    <td class="billing-cell-end-sm py-1">
                      {{ item.provider_id || item.key }}
                    </td>
                    <td class="billing-cell-end-sm py-1">
                      {{ formatInteger(item.total_tokens || 0) }}
                    </td>
                    <td class="billing-cell-end-sm py-1">
                      {{ formatCurrency(item.estimated_cost || 0) }}
                    </td>
                    <td class="billing-cell-end-sm py-1">
                      {{ percent(item.total_tokens || 0, selectedAnomaly?.totalTokens || 0) }}
                    </td>
                    <td class="billing-cell-end-sm py-1">
                      {{ percent(item.estimated_cost || 0, selectedAnomaly?.estimatedCost || 0) }}
                    </td>
                    <td class="billing-cell-end-sm py-1">
                      <button
                        class="px-2 py-0.5 rounded text-[11px] bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-900 dark:text-white transition-colors"
                        @click="applyDrilldownFilter('provider', item.provider_id || item.key)"
                      >
                        {{ t('billing.drilldownActionFilter') }}
                      </button>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>

            <div class="rounded-lg border border-gray-200 dark:border-gray-700 p-3">
              <h4 class="text-sm font-medium text-gray-900 dark:text-white mb-2">
                {{ t('billing.drilldownModel') }}
              </h4>
              <div
                v-if="anomalyModelBreakdown.length === 0"
                class="text-xs text-gray-500 dark:text-gray-400"
              >
                {{ t('billing.anomalyNone') }}
              </div>
              <table
                v-else
                class="w-full text-xs"
              >
                <thead>
                  <tr
                    class="billing-table-head text-gray-500 dark:text-gray-400 border-b border-gray-200 dark:border-gray-700"
                  >
                    <th class="billing-cell-end-sm py-1">
                      {{ t('billing.table.model') }}
                    </th>
                    <th class="billing-cell-end-sm py-1">
                      {{ t('billing.table.totalTokens') }}
                    </th>
                    <th class="billing-cell-end-sm py-1">
                      {{ t('billing.table.estimatedCost') }}
                    </th>
                    <th class="billing-cell-end-sm py-1">
                      {{ t('billing.drilldownTable.tokenShare') }}
                    </th>
                    <th class="billing-cell-end-sm py-1">
                      {{ t('billing.drilldownTable.costShare') }}
                    </th>
                    <th class="billing-cell-end-sm py-1">
                      {{ t('billing.anomalyTable.action') }}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="item in anomalyModelBreakdown"
                    :key="item.key"
                    class="border-b border-gray-100 dark:border-gray-800"
                  >
                    <td class="billing-cell-end-sm py-1">
                      {{ item.model_id || item.key }}
                    </td>
                    <td class="billing-cell-end-sm py-1">
                      {{ formatInteger(item.total_tokens || 0) }}
                    </td>
                    <td class="billing-cell-end-sm py-1">
                      {{ formatCurrency(item.estimated_cost || 0) }}
                    </td>
                    <td class="billing-cell-end-sm py-1">
                      {{ percent(item.total_tokens || 0, selectedAnomaly?.totalTokens || 0) }}
                    </td>
                    <td class="billing-cell-end-sm py-1">
                      {{ percent(item.estimated_cost || 0, selectedAnomaly?.estimatedCost || 0) }}
                    </td>
                    <td class="billing-cell-end-sm py-1">
                      <button
                        class="px-2 py-0.5 rounded text-[11px] bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-900 dark:text-white transition-colors"
                        @click="applyDrilldownFilter('model', item.model_id || item.key)"
                      >
                        {{ t('billing.drilldownActionFilter') }}
                      </button>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </div>

      <div
        v-if="summary?.breakdown?.length"
        class="glass-card p-4 mb-6"
      >
        <h2 class="text-base font-semibold text-gray-900 dark:text-white mb-3">
          {{ t('billing.breakdownTitle') }}
        </h2>
        <div class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead>
              <tr
                class="billing-table-head text-gray-500 dark:text-gray-400 border-b border-gray-200 dark:border-gray-700"
              >
                <th class="billing-cell-end py-2">
                  {{ t('billing.filters.groupBy') }}
                </th>
                <th class="billing-cell-end py-2">
                  {{ t('billing.totals.requests') }}
                </th>
                <th class="billing-cell-end py-2">
                  {{ t('billing.totals.totalTokens') }}
                </th>
                <th class="billing-cell-end py-2">
                  {{ t('billing.totals.estimatedCost') }}
                </th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="item in summary.breakdown"
                :key="item.key"
                class="border-b border-gray-100 dark:border-gray-800 text-gray-900 dark:text-gray-100"
              >
                <td class="billing-cell-end py-2">
                  {{ formatGroupLabel(item) }}
                </td>
                <td class="billing-cell-end py-2">
                  {{ formatInteger(item.request_count) }}
                </td>
                <td class="billing-cell-end py-2">
                  {{ formatInteger(item.total_tokens) }}
                </td>
                <td class="billing-cell-end py-2">
                  {{ formatCurrency(item.estimated_cost) }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div class="glass-card p-4">
        <h2 class="text-base font-semibold text-gray-900 dark:text-white mb-3">
          {{ t('billing.linesTitle') }}
        </h2>

        <div
          v-if="loading"
          class="py-8 text-center text-gray-500 dark:text-gray-400"
        >
          {{ t('common.loading') }}
        </div>
        <div
          v-else-if="!hasRows"
          class="py-8 text-center text-gray-500 dark:text-gray-400"
        >
          {{ t('billing.noData') }}
        </div>
        <div
          v-else
          class="overflow-x-auto"
        >
          <table class="w-full text-sm">
            <thead>
              <tr
                class="billing-table-head text-gray-500 dark:text-gray-400 border-b border-gray-200 dark:border-gray-700"
              >
                <th class="billing-cell-end py-2">
                  {{ t('billing.table.timestamp') }}
                </th>
                <th class="billing-cell-end py-2">
                  {{ t('billing.table.provider') }}
                </th>
                <th class="billing-cell-end py-2">
                  {{ t('billing.table.model') }}
                </th>
                <th class="billing-cell-end py-2">
                  {{ t('billing.table.inputTokens') }}
                </th>
                <th class="billing-cell-end py-2">
                  {{ t('billing.table.outputTokens') }}
                </th>
                <th class="billing-cell-end py-2">
                  {{ t('billing.table.cacheReadTokens') }}
                </th>
                <th class="billing-cell-end py-2">
                  {{ t('billing.table.cacheWriteTokens') }}
                </th>
                <th class="billing-cell-end py-2">
                  {{ t('billing.table.totalTokens') }}
                </th>
                <th class="billing-cell-end py-2">
                  {{ t('billing.table.estimatedCost') }}
                </th>
                <th class="billing-cell-end py-2">
                  {{ t('billing.table.requests') }}
                </th>
                <th class="billing-cell-end py-2">
                  {{ t('billing.table.status') }}
                </th>
                <th class="billing-cell-end py-2">
                  {{ t('billing.table.latencyMs') }}
                </th>
                <th class="billing-cell-end py-2">
                  {{ t('billing.table.user') }}
                </th>
                <th class="billing-cell-end py-2">
                  {{ t('billing.table.session') }}
                </th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="item in lines?.items"
                :key="`${item.timestamp}-${item.provider_id}-${item.model_id}-${item.session_id || ''}`"
                class="border-b border-gray-100 dark:border-gray-800 text-gray-900 dark:text-gray-100 align-top"
              >
                <td class="billing-cell-end py-2 whitespace-nowrap">
                  {{ formatDateTime(item.timestamp) }}
                </td>
                <td class="billing-cell-end py-2">
                  {{ item.provider_id }}
                </td>
                <td class="billing-cell-end py-2">
                  {{ item.model_id }}
                </td>
                <td class="billing-cell-end py-2">
                  {{ formatInteger(item.input_tokens) }}
                </td>
                <td class="billing-cell-end py-2">
                  {{ formatInteger(item.output_tokens) }}
                </td>
                <td class="billing-cell-end py-2">
                  {{ formatInteger(item.cache_read_tokens) }}
                </td>
                <td class="billing-cell-end py-2">
                  {{ formatInteger(item.cache_write_tokens) }}
                </td>
                <td class="billing-cell-end py-2">
                  {{ formatInteger(item.total_tokens) }}
                </td>
                <td class="billing-cell-end py-2">
                  {{ formatCurrency(item.estimated_cost) }}
                </td>
                <td class="billing-cell-end py-2">
                  {{ formatInteger(item.request_count) }}
                </td>
                <td class="billing-cell-end py-2">
                  <span
                    class="px-2 py-0.5 rounded-full text-xs"
                    :class="
                      item.success
                        ? 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300'
                        : 'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300'
                    "
                  >
                    {{ item.success ? t('audit.success') : t('audit.failure') }}
                  </span>
                </td>
                <td class="billing-cell-end py-2">
                  {{ item.latency_ms > 0 ? `${formatInteger(item.latency_ms)}ms` : '-' }}
                </td>
                <td class="billing-cell-end py-2">
                  {{ item.user_id || '-' }}
                </td>
                <td class="billing-cell-end py-2">
                  {{ item.session_id || '-' }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div
          v-if="(lines?.total ?? 0) > 0"
          class="flex items-center justify-between mt-4 text-sm"
        >
          <div class="text-gray-500 dark:text-gray-400">
            {{
              t('common.showingEntries', {
                shown: lines?.items.length || 0,
                total: lines?.total || 0,
              })
            }}
          </div>
          <div class="flex items-center gap-2">
            <button
              class="px-3 py-1 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-900 dark:text-white rounded disabled:opacity-50"
              :disabled="page <= 1 || loading"
              @click="setPage(page - 1)"
            >
              {{ t('common.previous') }}
            </button>
            <span class="text-gray-500 dark:text-gray-400">{{ page }} / {{ totalPages }}</span>
            <button
              class="px-3 py-1 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-900 dark:text-white rounded disabled:opacity-50"
              :disabled="page >= totalPages || loading"
              @click="setPage(page + 1)"
            >
              {{ t('common.next') }}
            </button>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.billing-table-head {
  text-align: start;
}

.billing-cell-end {
  padding-inline-end: 0.75rem;
}

.billing-cell-end-sm {
  padding-inline-end: 0.5rem;
}
</style>
