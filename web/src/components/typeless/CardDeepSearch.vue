<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { TypelessCardDeepSearch, DeepSearchCitationItem } from '@/types/typeless'

const { t } = useI18n()

const props = defineProps<{
  card: TypelessCardDeepSearch
}>()

const query = computed(() => props.card.query || '')
const mode = computed(() => props.card.mode || 'standard')
const answer = computed(() => props.card.answer || '')
const evidenceCount = computed(() => props.card.evidence_count || 0)
const citations = computed(() => (props.card.citations || []).filter(validCitation).slice(0, 8))
const openQuestions = computed(() => props.card.open_questions || [])
const confidenceText = computed(() => {
  const v = props.card.confidence
  if (typeof v !== 'number') return '--'
  const pct = Math.max(0, Math.min(100, Math.round(v * 100)))
  return `${pct}%`
})

function validCitation(item: unknown): item is DeepSearchCitationItem {
  if (!item || typeof item !== 'object') return false
  const c = item as Record<string, unknown>
  return typeof c.url === 'string' && c.url.length > 0
}

function domainOf(url: string): string {
  try {
    return new URL(url).hostname.replace(/^www\./, '')
  } catch {
    return url
  }
}
</script>

<template>
  <div class="deep-research-card rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 overflow-hidden shadow-sm">
    <div class="px-4 py-3 border-b border-gray-100 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/60">
      <div class="flex items-center justify-between gap-3">
        <div class="text-sm font-semibold text-gray-800 dark:text-gray-100">
          {{ t('chat.deepSearchTitle', 'Deep Research') }}
        </div>
        <div class="flex items-center gap-2 text-xs">
          <span class="px-2 py-0.5 rounded bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300">
            {{ mode }}
          </span>
          <span class="px-2 py-0.5 rounded bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300">
            {{ confidenceText }}
          </span>
        </div>
      </div>
      <div v-if="query" class="mt-1 text-xs text-gray-500 dark:text-gray-400 truncate">
        {{ query }}
      </div>
    </div>

    <div class="px-4 py-3 space-y-3">
      <p v-if="answer" class="text-sm leading-6 text-gray-700 dark:text-gray-200 whitespace-pre-wrap">
        {{ answer }}
      </p>

      <div class="text-xs text-gray-500 dark:text-gray-400">
        {{ t('chat.deepSearchEvidence', 'Evidence') }}: {{ evidenceCount }}
      </div>

      <div v-if="citations.length > 0" class="space-y-2">
        <div class="text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('chat.deepSearchCitations', 'Citations') }}</div>
        <a
          v-for="(c, idx) in citations"
          :key="idx"
          :href="c.url"
          target="_blank"
          rel="noopener noreferrer"
          class="block px-2 py-1.5 rounded border border-gray-100 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/40 transition-colors"
        >
          <div class="text-sm text-blue-600 dark:text-blue-400 line-clamp-1">
            {{ c.title || c.url }}
          </div>
          <div class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
            {{ domainOf(c.url) }}
          </div>
        </a>
      </div>

      <div v-if="openQuestions.length > 0" class="space-y-1">
        <div class="text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('chat.deepSearchOpenQuestions', 'Open questions') }}</div>
        <ul class="text-xs text-gray-500 dark:text-gray-400 list-disc pl-4">
          <li v-for="(q, idx) in openQuestions" :key="idx">{{ q }}</li>
        </ul>
      </div>
    </div>
  </div>
</template>
