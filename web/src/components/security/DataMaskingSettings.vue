<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  proxyApi,
  type CreateMaskingRulePayload,
  type MaskingRule,
  type MaskingStats,
} from '@/api/proxy'
import { useI18n } from 'vue-i18n'

const { t, te } = useI18n()

const loading = ref(true)
const maskingStats = ref<MaskingStats | null>(null)
const maskingRules = ref<MaskingRule[]>([])
const defaultMaskingRules = ref<MaskingRule[]>([])
const togglingMasking = ref(false)
const togglingMaskingRuleId = ref('')
const addingMaskingRule = ref(false)
const deletingMaskingRuleId = ref('')
const feedbackError = ref('')
const feedbackSuccess = ref('')

const customRuleName = ref('')
const customRulePattern = ref('')
const customRuleReplacement = ref('【{MASKED}】[CUSTOM]')
const customRuleDirection = ref<MaskingRule['direction']>('both')

function tr(key: string, fallback = '', values?: Record<string, string | number>): string {
  return te(key) ? t(key, values ?? {}) : fallback
}

const maskingSummary = computed(() => {
  if (!maskingStats.value?.enabled) return ''
  return `${t('apiProxy.maskingRules')}: ${maskingStats.value.rule_count} · ${t('apiProxy.maskingMatches')}: ${maskingStats.value.total_masks}`
})

const builtinRuleIds = computed(() => new Set(defaultMaskingRules.value.map((rule) => rule.id)))

const builtinMaskingRules = computed(() =>
  maskingRules.value.filter((rule) => builtinRuleIds.value.has(rule.id))
)

const customMaskingRules = computed(() =>
  maskingRules.value.filter((rule) => !builtinRuleIds.value.has(rule.id))
)

const maskingDirectionOptions: MaskingRule['direction'][] = ['request', 'response', 'both']

const canAddCustomRule = computed(
  () =>
    customRuleName.value.trim().length > 0 &&
    customRulePattern.value.trim().length > 0 &&
    customRuleReplacement.value.trim().length > 0
)

function clearFeedback() {
  feedbackError.value = ''
  feedbackSuccess.value = ''
}

function resetCustomRuleForm() {
  customRuleName.value = ''
  customRulePattern.value = ''
  customRuleReplacement.value = '【{MASKED}】[CUSTOM]'
  customRuleDirection.value = 'both'
}

function buildCustomRuleId(name: string): string {
  const slug = name
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 28)

  const randomPart =
    typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function'
      ? crypto.randomUUID().slice(0, 8)
      : Math.random().toString(36).slice(2, 10)

  return `custom-${slug || 'rule'}-${randomPart}`
}

function getErrorMessage(error: unknown, fallback: string): string {
  if (typeof error === 'object' && error !== null) {
    const responseData = (
      error as {
        response?: {
          data?: {
            error?: string
            message?: string
          }
        }
      }
    ).response?.data

    if (typeof responseData?.error === 'string' && responseData.error.trim()) {
      return responseData.error
    }
    if (typeof responseData?.message === 'string' && responseData.message.trim()) {
      return responseData.message
    }
  }

  return error instanceof Error && error.message.trim() ? error.message : fallback
}

async function fetchMaskingData(showLoader = true) {
  if (showLoader) loading.value = true
  try {
    const [maskingRes, maskingRulesRes] = await Promise.all([
      proxyApi.getMaskingStats().catch(() => null),
      proxyApi.getMaskingRules().catch(() => null),
    ])

    if (maskingRes) maskingStats.value = maskingRes.data
    if (maskingRulesRes) {
      maskingRules.value = Array.isArray(maskingRulesRes.data.rules) ? maskingRulesRes.data.rules : []
      defaultMaskingRules.value = Array.isArray(maskingRulesRes.data.default_rules)
        ? maskingRulesRes.data.default_rules
        : []
    }
  } finally {
    if (showLoader) loading.value = false
  }
}

async function toggleMasking() {
  if (togglingMasking.value || !maskingStats.value) return
  togglingMasking.value = true
  clearFeedback()
  try {
    const res = await proxyApi.toggleMasking(!maskingStats.value.enabled)
    maskingStats.value = res.data
  } finally {
    togglingMasking.value = false
  }
}

async function toggleMaskingRule(rule: MaskingRule) {
  if (togglingMaskingRuleId.value) return
  togglingMaskingRuleId.value = rule.id
  clearFeedback()
  try {
    const res = await proxyApi.updateMaskingRule(rule.id, { enabled: !rule.enabled })
    const idx = maskingRules.value.findIndex((item) => item.id === rule.id)
    if (idx >= 0 && res.data.rule) maskingRules.value[idx] = res.data.rule
    if (maskingStats.value) maskingStats.value = res.data.stats
  } finally {
    togglingMaskingRuleId.value = ''
  }
}

async function addCustomMaskingRule() {
  if (addingMaskingRule.value || !canAddCustomRule.value) return

  const payload: CreateMaskingRulePayload = {
    id: buildCustomRuleId(customRuleName.value),
    name: customRuleName.value.trim(),
    category: 'custom',
    pattern: customRulePattern.value.trim(),
    replacement: customRuleReplacement.value.trim(),
    direction: customRuleDirection.value,
    enabled: true,
  }

  addingMaskingRule.value = true
  clearFeedback()
  try {
    await proxyApi.addMaskingRule(payload)
    await fetchMaskingData(false)
    feedbackSuccess.value = tr('apiProxy.customMaskingAddSuccess', 'Custom masking rule added.')
    resetCustomRuleForm()
  } catch (error: unknown) {
    feedbackError.value = getErrorMessage(
      error,
      tr('apiProxy.customMaskingAddError', 'Failed to add custom masking rule.')
    )
  } finally {
    addingMaskingRule.value = false
  }
}

async function deleteCustomMaskingRule(rule: MaskingRule) {
  if (deletingMaskingRuleId.value) return
  const ruleLabel = rule.name || rule.id
  const confirmed = window.confirm(
    tr('apiProxy.customMaskingDeleteConfirm', `Delete custom masking rule "${ruleLabel}"?`, {
      name: ruleLabel,
    })
  )
  if (!confirmed) return

  deletingMaskingRuleId.value = rule.id
  clearFeedback()
  try {
    await proxyApi.removeMaskingRule(rule.id)
    maskingRules.value = maskingRules.value.filter((item) => item.id !== rule.id)
    if (maskingStats.value) {
      maskingStats.value = {
        ...maskingStats.value,
        rule_count: Math.max(0, maskingStats.value.rule_count - 1),
      }
    }
    feedbackSuccess.value = tr('apiProxy.customMaskingDeleteSuccess', 'Custom masking rule deleted.')
  } catch (error: unknown) {
    feedbackError.value = getErrorMessage(
      error,
      tr('apiProxy.customMaskingDeleteError', 'Failed to delete custom masking rule.')
    )
  } finally {
    deletingMaskingRuleId.value = ''
  }
}

function getMaskingRuleName(rule: MaskingRule): string {
  return tr(`apiProxy.maskingRuleNames.${rule.id}`, rule.name || rule.id)
}

function getMaskingRuleCategory(rule: MaskingRule): string {
  return tr(`apiProxy.maskingCategories.${rule.category}`, rule.category)
}

function getMaskingRuleDirection(rule: MaskingRule): string {
  return tr(`apiProxy.maskingDirections.${rule.direction}`, rule.direction)
}

function getDirectionLabel(direction: MaskingRule['direction']): string {
  return tr(`apiProxy.maskingDirections.${direction}`, direction)
}

onMounted(() => {
  void fetchMaskingData(true)
})
</script>

<template>
  <div class="space-y-6">
    <div v-if="loading" class="text-center py-8">
      <div
        class="animate-spin w-6 h-6 border-2 border-gray-900 dark:border-gray-700 border-t-transparent rounded-full mx-auto"
      ></div>
      <p class="text-gray-400 mt-2 text-sm">{{ t('common.loading') }}</p>
    </div>

    <div v-else-if="maskingStats" class="glass-card security-outlined-card p-6 space-y-5">
      <div class="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('apiProxy.maskingTitle') }}
          </h3>
          <p class="text-sm text-gray-500 dark:text-gray-400 mt-0.5">
            {{ t('apiProxy.maskingDesc') }}
          </p>
        </div>
        <div class="flex items-center gap-4 text-xs text-gray-500 dark:text-gray-400">
          <span v-if="maskingSummary">{{ maskingSummary }}</span>
          <button
            type="button"
            :disabled="togglingMasking"
            :class="[
              'relative inline-flex h-6 w-11 items-center rounded-full transition-colors',
              maskingStats.enabled
                ? 'bg-green-600 dark:bg-green-500'
                : 'bg-gray-300 dark:bg-gray-600',
              togglingMasking ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer',
            ]"
            @click="toggleMasking"
          >
            <span
              :class="[
                'inline-block h-4 w-4 transform rounded-full bg-white transition-transform',
                maskingStats.enabled ? 'translate-x-6' : 'translate-x-1',
              ]"
            />
          </button>
        </div>
      </div>

      <div
        v-if="feedbackError"
        class="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900/40 dark:bg-red-950/40 dark:text-red-300"
      >
        {{ feedbackError }}
      </div>
      <div
        v-else-if="feedbackSuccess"
        class="rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700 dark:border-emerald-900/40 dark:bg-emerald-950/40 dark:text-emerald-300"
      >
        {{ feedbackSuccess }}
      </div>

      <div class="space-y-4 border-t border-gray-100 pt-4 dark:border-white/10">
        <div>
          <div class="mb-2 flex items-center justify-between gap-3">
            <p class="text-xs font-semibold uppercase tracking-[0.18em] text-gray-500 dark:text-gray-400">
              {{ tr('apiProxy.builtinMaskingRules', 'Built-in rules') }}
            </p>
            <span class="text-xs text-gray-500 dark:text-gray-400">{{ builtinMaskingRules.length }}</span>
          </div>
          <div v-if="builtinMaskingRules.length > 0" class="space-y-1.5">
            <div
              v-for="rule in builtinMaskingRules"
              :key="rule.id"
              class="rounded-lg border border-gray-200 bg-gray-50 px-3 py-3 dark:border-slate-700 dark:bg-slate-800/35"
            >
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0">
                  <p class="text-sm font-medium text-gray-900 dark:text-white">
                    {{ getMaskingRuleName(rule) }}
                  </p>
                  <p class="mt-1 text-[11px] text-gray-500 dark:text-gray-400">
                    {{ getMaskingRuleCategory(rule) }} · {{ getMaskingRuleDirection(rule) }}
                  </p>
                </div>
                <button
                  type="button"
                  :disabled="togglingMaskingRuleId === rule.id"
                  @click="toggleMaskingRule(rule)"
                  :class="[
                    'shrink-0 text-[10px] px-1.5 py-0.5 rounded-full transition-colors',
                    togglingMaskingRuleId === rule.id
                      ? 'opacity-60 cursor-not-allowed'
                      : 'cursor-pointer',
                    rule.enabled
                      ? 'bg-green-100 text-green-700 dark:bg-green-500/20 dark:text-green-300'
                      : 'bg-gray-200 text-gray-600 dark:bg-gray-600 dark:text-gray-300',
                  ]"
                >
                  {{ rule.enabled ? t('common.enabled') : t('common.disabled') }}
                </button>
              </div>
              <p class="mt-2 text-[11px] font-mono text-gray-500 dark:text-gray-400 break-all">
                {{ rule.pattern }}
              </p>
              <p class="mt-1 text-[11px] font-mono text-gray-500 dark:text-gray-400 break-all">
                {{ rule.replacement }}
              </p>
            </div>
          </div>
          <p v-else class="text-xs text-gray-400 dark:text-gray-500">
            {{ tr('apiProxy.builtinMaskingEmpty', 'No built-in rules loaded.') }}
          </p>
        </div>

        <div>
          <div class="mb-2 flex items-center justify-between gap-3">
            <p class="text-xs font-semibold uppercase tracking-[0.18em] text-gray-500 dark:text-gray-400">
              {{ tr('apiProxy.customMaskingListTitle', 'Custom rules') }}
            </p>
            <span class="text-xs text-gray-500 dark:text-gray-400">{{ customMaskingRules.length }}</span>
          </div>
          <div v-if="customMaskingRules.length > 0" class="space-y-1.5">
            <div
              v-for="rule in customMaskingRules"
              :key="rule.id"
              class="rounded-lg border border-gray-200 bg-gray-50 px-3 py-3 dark:border-slate-700 dark:bg-slate-800/35"
            >
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0">
                  <div class="flex flex-wrap items-center gap-2">
                    <p class="text-sm font-medium text-gray-900 dark:text-white">
                      {{ getMaskingRuleName(rule) }}
                    </p>
                    <span class="rounded-full bg-gray-200 px-2 py-0.5 text-[10px] font-medium text-gray-600 dark:bg-gray-600 dark:text-gray-200">
                      {{ tr('apiProxy.maskingCategories.custom', 'Custom') }}
                    </span>
                  </div>
                  <p class="mt-1 text-[11px] text-gray-500 dark:text-gray-400">
                    {{ getMaskingRuleDirection(rule) }}
                  </p>
                </div>
                <div class="flex items-center gap-2">
                  <button
                    type="button"
                    :disabled="togglingMaskingRuleId === rule.id"
                    @click="toggleMaskingRule(rule)"
                    :class="[
                      'shrink-0 text-[10px] px-1.5 py-0.5 rounded-full transition-colors',
                      togglingMaskingRuleId === rule.id
                        ? 'opacity-60 cursor-not-allowed'
                        : 'cursor-pointer',
                      rule.enabled
                        ? 'bg-green-100 text-green-700 dark:bg-green-500/20 dark:text-green-300'
                        : 'bg-gray-200 text-gray-600 dark:bg-gray-600 dark:text-gray-300',
                    ]"
                  >
                    {{ rule.enabled ? t('common.enabled') : t('common.disabled') }}
                  </button>
                  <button
                    type="button"
                    :disabled="deletingMaskingRuleId === rule.id"
                    class="shrink-0 rounded-full bg-red-100 px-2 py-1 text-[10px] font-medium text-red-700 transition hover:bg-red-200 disabled:cursor-not-allowed disabled:opacity-60 dark:bg-red-900/30 dark:text-red-300 dark:hover:bg-red-900/50"
                    @click="deleteCustomMaskingRule(rule)"
                  >
                    {{ deletingMaskingRuleId === rule.id ? tr('common.loading', 'Loading') : t('common.delete') }}
                  </button>
                </div>
              </div>
              <p class="mt-2 text-[11px] font-mono text-gray-500 dark:text-gray-400 break-all">
                {{ rule.pattern }}
              </p>
              <p class="mt-1 text-[11px] font-mono text-gray-500 dark:text-gray-400 break-all">
                {{ rule.replacement }}
              </p>
            </div>
          </div>
          <p v-else class="text-xs text-gray-400 dark:text-gray-500">
            {{
              tr(
                'apiProxy.customMaskingEmpty',
                'No custom rules yet. Use the form below to cover internal IDs or proprietary secrets.'
              )
            }}
          </p>
        </div>
      </div>

      <div
        class="rounded-xl border border-gray-200 bg-gray-50 p-4 dark:border-slate-700 dark:bg-slate-800/35"
      >
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h4 class="text-sm font-semibold text-gray-900 dark:text-white">
              {{ tr('apiProxy.customMaskingTitle', 'Custom Data Masking Rules') }}
            </h4>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{
                tr(
                  'apiProxy.customMaskingDesc',
                  'Add regex-based rules for internal IDs, secrets, or business-specific fields.'
                )
              }}
            </p>
          </div>
          <span class="rounded-full bg-white/80 dark:bg-slate-800/80 px-2.5 py-1 text-[11px] font-medium text-gray-600 dark:text-slate-300">
            {{ customMaskingRules.length }} {{ tr('apiProxy.maskingCategories.custom', 'Custom') }}
          </span>
        </div>

        <div class="mt-4 grid gap-3 md:grid-cols-2">
          <label class="space-y-1">
            <span class="text-xs font-medium text-gray-700 dark:text-gray-300">
              {{ tr('apiProxy.customMaskingNameLabel', 'Name') }}
            </span>
            <input
              v-model="customRuleName"
              type="text"
              data-testid="masking-custom-name"
              class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 outline-none transition focus:border-gray-900 focus:ring-2 focus:ring-gray-900/10 dark:border-gray-600 dark:bg-gray-800 dark:text-white dark:focus:border-gray-300 dark:focus:ring-gray-300/10"
              :placeholder="
                tr('apiProxy.customMaskingNamePlaceholder', 'Example: Internal ticket number')
              "
            />
          </label>

          <label class="space-y-1">
            <span class="text-xs font-medium text-gray-700 dark:text-gray-300">
              {{ tr('apiProxy.customMaskingDirectionLabel', 'Direction') }}
            </span>
            <select
              v-model="customRuleDirection"
              data-testid="masking-custom-direction"
              class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 outline-none transition focus:border-gray-900 focus:ring-2 focus:ring-gray-900/10 dark:border-gray-600 dark:bg-gray-800 dark:text-white dark:focus:border-gray-300 dark:focus:ring-gray-300/10"
            >
              <option
                v-for="direction in maskingDirectionOptions"
                :key="direction"
                :value="direction"
              >
                {{ getDirectionLabel(direction) }}
              </option>
            </select>
          </label>

          <label class="space-y-1 md:col-span-2">
            <span class="text-xs font-medium text-gray-700 dark:text-gray-300">
              {{ tr('apiProxy.customMaskingPatternLabel', 'Regex pattern') }}
            </span>
            <input
              v-model="customRulePattern"
              type="text"
              data-testid="masking-custom-pattern"
              class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm font-mono text-gray-900 outline-none transition focus:border-gray-900 focus:ring-2 focus:ring-gray-900/10 dark:border-gray-600 dark:bg-gray-800 dark:text-white dark:focus:border-gray-300 dark:focus:ring-gray-300/10"
              :placeholder="
                tr('apiProxy.customMaskingPatternPlaceholder', 'Example: TKT-\\d{6}')
              "
            />
          </label>

          <label class="space-y-1 md:col-span-2">
            <span class="text-xs font-medium text-gray-700 dark:text-gray-300">
              {{ tr('apiProxy.customMaskingReplacementLabel', 'Replacement text') }}
            </span>
            <input
              v-model="customRuleReplacement"
              type="text"
              data-testid="masking-custom-replacement"
              class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm font-mono text-gray-900 outline-none transition focus:border-gray-900 focus:ring-2 focus:ring-gray-900/10 dark:border-gray-600 dark:bg-gray-800 dark:text-white dark:focus:border-gray-300 dark:focus:ring-gray-300/10"
              :placeholder="
                tr('apiProxy.customMaskingReplacementPlaceholder', 'Example: 【{MASKED}】[TICKET]', {
                  maskLabel: '{MASKED}',
                })
              "
            />
          </label>
        </div>

        <div class="mt-4 flex flex-wrap items-center justify-between gap-3">
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{
              tr(
                'apiProxy.customMaskingHint',
                'Use a valid regular expression. {MASKED} in replacement text keeps the localized mask label.',
                { maskLabel: '{MASKED}' }
              )
            }}
          </p>
          <button
            type="button"
            data-testid="masking-add-rule"
            :disabled="addingMaskingRule || !canAddCustomRule"
            class="inline-flex items-center gap-2 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition hover:bg-gray-700 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-gray-200 dark:text-gray-900 dark:hover:bg-white"
            @click="addCustomMaskingRule"
          >
            <svg
              v-if="addingMaskingRule"
              class="h-4 w-4 animate-spin"
              xmlns="http://www.w3.org/2000/svg"
              fill="none"
              viewBox="0 0 24 24"
            >
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            {{ tr('apiProxy.customMaskingAdd', 'Add rule') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
