<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { proxyApi, type MaskingRule, type MaskingStats } from '@/api/proxy'

const { t, te } = useI18n()

const loading = ref(true)
const maskingStats = ref<MaskingStats | null>(null)
const maskingRules = ref<MaskingRule[]>([])
const togglingMasking = ref(false)
const togglingMaskingRuleId = ref('')

function tr(key: string, fallback = ''): string {
  return te(key) ? t(key) : fallback
}

const maskingSummary = computed(() => {
  if (!maskingStats.value?.enabled) return ''
  return `${t('apiProxy.maskingRules')}: ${maskingStats.value.rule_count} · ${t('apiProxy.maskingMatches')}: ${maskingStats.value.total_masks}`
})

async function fetchMaskingData() {
  loading.value = true
  try {
    const [maskingRes, maskingRulesRes] = await Promise.all([
      proxyApi.getMaskingStats().catch(() => null),
      proxyApi.getMaskingRules().catch(() => null),
    ])

    if (maskingRes) maskingStats.value = maskingRes.data
    if (maskingRulesRes && Array.isArray(maskingRulesRes.data.rules)) {
      maskingRules.value = maskingRulesRes.data.rules
    }
  } finally {
    loading.value = false
  }
}

async function toggleMasking() {
  if (togglingMasking.value || !maskingStats.value) return
  togglingMasking.value = true
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
  try {
    const res = await proxyApi.updateMaskingRule(rule.id, { enabled: !rule.enabled })
    const idx = maskingRules.value.findIndex(r => r.id === rule.id)
    if (idx >= 0) maskingRules.value[idx] = res.data.rule
    if (maskingStats.value) maskingStats.value = res.data.stats
  } finally {
    togglingMaskingRuleId.value = ''
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

onMounted(fetchMaskingData)
</script>

<template>
  <div class="space-y-6">
    <div v-if="loading" class="text-center py-8">
      <div class="animate-spin w-6 h-6 border-2 border-gray-900 dark:border-gray-700 border-t-transparent rounded-full mx-auto"></div>
      <p class="text-gray-400 mt-2 text-sm">{{ t('common.loading') }}</p>
    </div>

    <div v-else-if="maskingStats" class="glass-card p-6">
      <div class="flex items-center justify-between">
        <div>
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('apiProxy.maskingTitle') }}</h3>
          <p class="text-sm text-gray-500 dark:text-gray-400 mt-0.5">{{ t('apiProxy.maskingDesc') }}</p>
        </div>
        <div class="flex items-center gap-4 text-xs text-gray-500 dark:text-gray-400">
          <span v-if="maskingSummary">{{ maskingSummary }}</span>
          <button
            type="button"
            :disabled="togglingMasking"
            :class="[
              'relative inline-flex h-6 w-11 items-center rounded-full transition-colors',
              maskingStats.enabled ? 'bg-green-600 dark:bg-green-500' : 'bg-gray-300 dark:bg-gray-600',
              togglingMasking ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'
            ]"
            @click="toggleMasking"
          >
            <span :class="['inline-block h-4 w-4 transform rounded-full bg-white transition-transform', maskingStats.enabled ? 'translate-x-6' : 'translate-x-1']" />
          </button>
        </div>
      </div>

      <div class="mt-4 border-t border-gray-100 dark:border-white/10 pt-3">
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('apiProxy.maskingRuleList') }}</p>
        <div v-if="maskingRules.length > 0" class="mt-2 space-y-1.5">
          <div
            v-for="rule in maskingRules"
            :key="rule.id"
            class="rounded-lg bg-gray-50 dark:bg-gray-700/30 px-2.5 py-2"
          >
            <div class="flex items-start justify-between gap-3">
              <p class="text-xs font-medium text-gray-900 dark:text-white">{{ getMaskingRuleName(rule) }}</p>
              <button
                type="button"
                :disabled="togglingMaskingRuleId === rule.id"
                @click="toggleMaskingRule(rule)"
                :class="[
                  'shrink-0 text-[10px] px-1.5 py-0.5 rounded-full transition-colors',
                  togglingMaskingRuleId === rule.id ? 'opacity-60 cursor-not-allowed' : 'cursor-pointer',
                  rule.enabled
                    ? 'bg-green-100 text-green-700 dark:bg-green-500/20 dark:text-green-300'
                    : 'bg-gray-200 text-gray-600 dark:bg-gray-600 dark:text-gray-300'
                ]"
              >
                {{ rule.enabled ? t('common.enabled') : t('common.disabled') }}
              </button>
            </div>
            <p class="mt-1 text-[10px] text-gray-500 dark:text-gray-400">
              {{ getMaskingRuleCategory(rule) }} · {{ getMaskingRuleDirection(rule) }}
            </p>
            <p class="mt-1 text-[10px] font-mono text-gray-500 dark:text-gray-400 break-all">{{ rule.pattern }}</p>
          </div>
        </div>
        <p v-else class="mt-2 text-xs text-gray-400 dark:text-gray-500">{{ t('common.noData') }}</p>
      </div>
    </div>
  </div>
</template>
