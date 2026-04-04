<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { SearchResultItem, TypelessCardSearch } from '@/types/typeless'
import api from '@/api/index'
import { usePersistentDisclosureState } from '@/utils/chatCardUiState'

type SearchCardStatus = 'success' | 'partial' | 'empty'

interface LinkPreview {
  title?: string
  description?: string
  image?: string
  favicon?: string
  siteName?: string
  loading: boolean
  error: boolean
  fetched: boolean
}

const { t, te } = useI18n()

const props = defineProps<{
  card: TypelessCardSearch
  uiStateKey?: string
}>()

const query = computed(() => props.card.query || '')
const results = computed(() => (props.card.results ?? []).filter(validResult).slice(0, 8))
const isStreaming = computed(() => props.card._streaming === true)
const summaryKey = computed(() => props.uiStateKey || props.card.id || `search:${query.value}`)
const { expanded, toggleExpanded } = usePersistentDisclosureState(summaryKey, false)
const summaryTitle = computed(() =>
  te('search.summaryTitle') ? String(t('search.summaryTitle')) : 'Web search'
)
const searchingLabel = computed(() =>
  te('common.searching') ? String(t('common.searching')) : 'Searching...'
)
const summaryLabel = computed(() => query.value || searchingLabel.value)
const summaryCount = computed(() => props.card.totalCount ?? results.value.length)
const collapsedPreviewResults = computed(() => results.value.slice(0, 2))
const collapsedRemainingCount = computed(() =>
  Math.max(summaryCount.value - collapsedPreviewResults.value.length, 0)
)
const collapsedRemainingCountLabel = computed(() => {
  if (collapsedRemainingCount.value <= 0) return ''
  if (te('search.moreResults')) {
    return String(t('search.moreResults', { count: collapsedRemainingCount.value }))
  }
  return `+${collapsedRemainingCount.value} more`
})
const resultCountLabel = computed(() => {
  if (summaryCount.value <= 0) return ''
  if (te('search.resultCount')) {
    return String(t('search.resultCount', { count: summaryCount.value }))
  }
  return summaryCount.value === 1 ? '1 result' : `${summaryCount.value} results`
})
const providerLabel = computed(() => {
  const provider = firstNonEmptyString(props.card.provider)
  if (!provider) return ''
  return provider.replace(/^browser:/i, 'browser: ')
})
const status = computed<SearchCardStatus>(() => {
  const raw = firstNonEmptyString(props.card.status)
  if (raw === 'empty' || raw === 'partial' || raw === 'success') {
    return raw
  }
  if (!isStreaming.value && results.value.length === 0) {
    return 'empty'
  }
  return 'success'
})
const statusMessage = computed(() => {
  const explicit = firstNonEmptyString(props.card.message)
  if (explicit) return explicit
  if (status.value === 'empty') {
    return te('search.emptyState') ? String(t('search.emptyState')) : 'No matching results'
  }
  if (status.value === 'partial') {
    return te('search.partialState')
      ? String(t('search.partialState'))
      : 'Showing source summaries because readable page content was unavailable.'
  }
  return ''
})
const selectedUrl = computed(() => firstNonEmptyString(props.card.selectedUrl))
const showCollapsedPreview = computed(
  () => !expanded.value && collapsedPreviewResults.value.length > 0
)

function validResult(value: unknown): value is SearchResultItem {
  if (!value || typeof value !== 'object') return false
  const item = value as Record<string, unknown>
  return typeof item.url === 'string' && item.url.length > 0
}

function firstNonEmptyString(...values: unknown[]): string {
  for (const value of values) {
    if (typeof value !== 'string') continue
    const trimmed = value.trim()
    if (trimmed) return trimmed
  }
  return ''
}

function getDomain(url: string): string {
  try {
    return new URL(url).hostname.replace(/^www\./, '')
  } catch {
    return url
  }
}

function getFaviconUrl(url: string): string {
  try {
    const origin = new URL(url).origin
    return `${origin}/favicon.ico`
  } catch {
    return ''
  }
}

function getCollapsedPreviewTitle(result: SearchResultItem): string {
  return firstNonEmptyString(result.title, getDomain(result.url))
}

function getCollapsedPreviewDomain(result: SearchResultItem): string {
  const domain = getDomain(result.url)
  return domain === getCollapsedPreviewTitle(result) ? '' : domain
}

function getCollapsedPreviewDescription(result: SearchResultItem): string {
  return firstNonEmptyString(result.description, getCollapsedPreviewDomain(result))
}

function isSelectedResult(result: SearchResultItem): boolean {
  return selectedUrl.value !== '' && result.url === selectedUrl.value
}

const previewSupported = ref(false)
const previewCache = reactive<Record<string, LinkPreview>>({})
const activePreview = ref<string | null>(null)
const previewPosition = ref({ x: 16, y: 16 })
let hoverTimer: ReturnType<typeof setTimeout> | null = null
let previewMediaQuery: MediaQueryList | null = null
let previewMediaQueryListener: ((event: MediaQueryListEvent) => void) | (() => void) | null = null

function buildFallbackPreview(result: SearchResultItem): LinkPreview {
  return {
    title: firstNonEmptyString(result.title, getDomain(result.url)),
    description: firstNonEmptyString(result.description),
    favicon: firstNonEmptyString(getFaviconUrl(result.url)) || undefined,
    siteName: getDomain(result.url),
    loading: false,
    error: false,
    fetched: false,
  }
}

function mergePreview(url: string, fallback: LinkPreview, override: Partial<LinkPreview> = {}) {
  const existing = previewCache[url]
  previewCache[url] = {
    title: firstNonEmptyString(override.title, existing?.title, fallback.title) || undefined,
    description:
      firstNonEmptyString(override.description, existing?.description, fallback.description) ||
      undefined,
    image: firstNonEmptyString(override.image, existing?.image) || undefined,
    favicon:
      firstNonEmptyString(override.favicon, existing?.favicon, fallback.favicon) || undefined,
    siteName:
      firstNonEmptyString(override.siteName, existing?.siteName, fallback.siteName) || undefined,
    loading: override.loading ?? existing?.loading ?? fallback.loading,
    error: override.error ?? existing?.error ?? fallback.error,
    fetched: override.fetched ?? existing?.fetched ?? fallback.fetched,
  }
  return previewCache[url]
}

function hidePreview() {
  if (hoverTimer) {
    clearTimeout(hoverTimer)
    hoverTimer = null
  }
  activePreview.value = null
}

function updatePreviewSupport(matches: boolean) {
  previewSupported.value = matches
  if (!matches) {
    hidePreview()
  }
}

function onResultMouseEnter(result: SearchResultItem, event: MouseEvent) {
  if (!previewSupported.value) return
  if (hoverTimer) clearTimeout(hoverTimer)
  hoverTimer = setTimeout(() => {
    hoverTimer = null
    activePreview.value = result.url
    updatePreviewPosition(event)
    mergePreview(result.url, buildFallbackPreview(result))
    void fetchPreview(result)
  }, 400)
}

function onResultMouseMove(event: MouseEvent) {
  if (!previewSupported.value || !activePreview.value) return
  updatePreviewPosition(event)
}

function onResultMouseLeave() {
  hidePreview()
}

function updatePreviewPosition(event: MouseEvent) {
  if (typeof window === 'undefined') return
  const row = event.currentTarget as HTMLElement | null
  const rect = row?.getBoundingClientRect()
  const previewWidth = 320
  const previewHeight = 260
  const fallbackX = Math.max(window.innerWidth - previewWidth - 16, 16)
  const x = rect ? Math.min(rect.right + 12, fallbackX) : 16
  const y = Math.max(
    16,
    Math.min(event.clientY - 72, Math.max(window.innerHeight - previewHeight - 16, 16))
  )
  previewPosition.value = { x, y }
}

async function fetchPreview(result: SearchResultItem) {
  const fallback = buildFallbackPreview(result)
  const current = mergePreview(result.url, fallback)
  if (current.loading || current.fetched) return

  mergePreview(result.url, fallback, { loading: true, error: false })
  try {
    const response = await api.get('/link-preview', {
      params: { url: result.url },
      timeout: 8000,
    })
    const data = (response as { data?: Record<string, unknown> })?.data ?? {}
    mergePreview(result.url, fallback, {
      title: firstNonEmptyString(String(data.title ?? '')),
      description: firstNonEmptyString(String(data.description ?? '')),
      image: firstNonEmptyString(String(data.image ?? '')),
      favicon: firstNonEmptyString(String(data.favicon ?? '')),
      siteName: firstNonEmptyString(String(data.siteName ?? '')),
      loading: false,
      error: false,
      fetched: true,
    })
  } catch {
    mergePreview(result.url, fallback, {
      loading: false,
      error: true,
      fetched: true,
    })
  }
}

const currentPreview = computed(() => {
  if (!activePreview.value) return null
  return previewCache[activePreview.value] ?? null
})

onMounted(() => {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return
  previewMediaQuery = window.matchMedia('(hover: hover) and (pointer: fine)')
  updatePreviewSupport(previewMediaQuery.matches)
  previewMediaQueryListener = (event: MediaQueryListEvent) => updatePreviewSupport(event.matches)
  if (typeof previewMediaQuery.addEventListener === 'function') {
    previewMediaQuery.addEventListener('change', previewMediaQueryListener)
  } else if (typeof previewMediaQuery.addListener === 'function') {
    previewMediaQuery.addListener(previewMediaQueryListener)
  }
})

onBeforeUnmount(() => {
  hidePreview()
  if (!previewMediaQuery || !previewMediaQueryListener) return
  if (typeof previewMediaQuery.removeEventListener === 'function') {
    previewMediaQuery.removeEventListener('change', previewMediaQueryListener)
  } else if (typeof previewMediaQuery.removeListener === 'function') {
    previewMediaQuery.removeListener(previewMediaQueryListener)
  }
})
</script>

<template>
  <div
    class="search-card relative overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-[0_18px_40px_-32px_rgba(15,23,42,0.55)] dark:border-slate-700 dark:bg-slate-900"
  >
    <button
      class="flex w-full items-start gap-3 px-4 py-3 text-start transition-colors hover:bg-slate-50 dark:hover:bg-slate-800/60"
      :aria-expanded="expanded ? 'true' : 'false'"
      @click="toggleExpanded"
    >
      <span
        class="mt-0.5 flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-xl border border-slate-200 bg-slate-50 text-slate-600 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
      >
        <svg
          aria-hidden="true"
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
            d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
          />
        </svg>
      </span>

      <span class="min-w-0 flex-1">
        <span
          class="block text-[11px] font-semibold uppercase tracking-[0.12em] text-slate-500 dark:text-slate-400"
        >
          {{ summaryTitle }}
        </span>

        <span class="mt-1 flex flex-wrap items-center gap-2">
          <span class="min-w-0 truncate text-sm font-semibold text-slate-900 dark:text-slate-100">
            {{ summaryLabel }}
          </span>
          <span
            v-if="summaryCount > 0"
            class="inline-flex rounded-full border border-slate-200 bg-slate-50 px-2 py-0.5 text-[11px] font-medium text-slate-600 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300"
          >
            {{ resultCountLabel }}
          </span>
          <span
            v-if="providerLabel"
            class="inline-flex rounded-full border border-slate-200 px-2 py-0.5 text-[11px] font-medium text-slate-500 dark:border-slate-700 dark:text-slate-400"
          >
            {{ providerLabel }}
          </span>
          <span
            v-if="isStreaming"
            class="inline-flex rounded-full border border-sky-200 bg-sky-50 px-2 py-0.5 text-[11px] font-medium text-sky-700 dark:border-sky-900/60 dark:bg-sky-950/40 dark:text-sky-300"
          >
            {{ searchingLabel }}
          </span>
        </span>

        <p
          v-if="statusMessage"
          class="search-card__status-note mt-2 text-xs leading-relaxed text-slate-500 dark:text-slate-400"
        >
          {{ statusMessage }}
        </p>

        <div v-if="showCollapsedPreview" class="mt-3 space-y-2">
          <div
            v-for="(result, index) in collapsedPreviewResults"
            :key="`${result.url}-${index}`"
            class="search-card__compact-row flex items-start gap-3 rounded-xl border border-slate-200/80 bg-slate-50/80 px-3 py-2.5 dark:border-slate-700/80 dark:bg-slate-800/70"
            :class="{
              'border-sky-200 bg-sky-50/70 dark:border-sky-900/60 dark:bg-sky-950/30':
                isSelectedResult(result),
            }"
          >
            <span
              class="search-card__compact-favicon relative mt-0.5 flex h-6 w-6 flex-shrink-0 items-center justify-center overflow-hidden rounded-lg border border-slate-200 bg-white text-slate-500 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-400"
            >
              <svg
                aria-hidden="true"
                xmlns="http://www.w3.org/2000/svg"
                class="h-3.5 w-3.5"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9"
                />
              </svg>
              <img
                :src="getFaviconUrl(result.url)"
                :alt="getCollapsedPreviewDomain(result) || getCollapsedPreviewTitle(result)"
                class="absolute inset-0 m-auto h-4 w-4 rounded-sm"
                loading="lazy"
                @error="($event.target as HTMLImageElement).style.display = 'none'"
              />
            </span>

            <span class="min-w-0 flex-1">
              <span
                class="block line-clamp-1 text-sm font-medium leading-snug text-slate-900 dark:text-slate-100"
              >
                {{ getCollapsedPreviewTitle(result) }}
              </span>
              <span
                v-if="getCollapsedPreviewDomain(result)"
                class="mt-0.5 block truncate text-[11px] text-slate-500 dark:text-slate-400"
              >
                {{ getCollapsedPreviewDomain(result) }}
              </span>
              <span
                v-if="getCollapsedPreviewDescription(result)"
                class="mt-1 block line-clamp-2 text-xs leading-relaxed text-slate-600 dark:text-slate-300"
              >
                {{ getCollapsedPreviewDescription(result) }}
              </span>
            </span>
          </div>

          <span
            v-if="collapsedRemainingCount > 0"
            class="inline-flex rounded-full border border-slate-200 px-2 py-0.5 text-[11px] font-medium text-slate-500 dark:border-slate-700 dark:text-slate-400"
          >
            {{ collapsedRemainingCountLabel }}
          </span>
        </div>
      </span>

      <span class="flex flex-shrink-0 items-center pt-1 text-slate-400 dark:text-slate-500">
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-4 w-4 transition-transform duration-200"
          :class="{ 'rotate-180': expanded }"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M19 9l-7 7-7-7"
          />
        </svg>
      </span>
    </button>

    <transition name="search-card-content">
      <div v-if="expanded" class="border-t border-slate-200 dark:border-slate-700">
        <div v-if="statusMessage" class="border-b border-slate-100 px-4 py-3 dark:border-slate-800">
          <div
            class="search-card__status-note rounded-xl border px-3 py-2 text-xs leading-relaxed"
            :class="{
              'border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-200':
                status === 'partial',
              'border-slate-200 bg-slate-50 text-slate-700 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200':
                status !== 'partial',
            }"
          >
            {{ statusMessage }}
          </div>
        </div>

        <div v-if="results.length > 0" class="py-1.5">
          <a
            v-for="(result, index) in results"
            :key="`${result.url}-${index}`"
            :href="result.url"
            target="_blank"
            rel="noopener noreferrer"
            class="search-card__result-row group flex items-start gap-3 px-4 py-3 transition-colors"
            :class="{
              'bg-sky-50/80 dark:bg-sky-950/20': isSelectedResult(result),
              'hover:bg-slate-50 dark:hover:bg-slate-800/70': !isSelectedResult(result),
            }"
            @mouseenter="onResultMouseEnter(result, $event)"
            @mousemove="onResultMouseMove"
            @mouseleave="onResultMouseLeave"
          >
            <span
              class="relative mt-0.5 flex h-7 w-7 flex-shrink-0 items-center justify-center overflow-hidden rounded-lg border border-slate-200 bg-slate-50 text-slate-500 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-400"
            >
              <svg
                aria-hidden="true"
                xmlns="http://www.w3.org/2000/svg"
                class="h-3.5 w-3.5"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9"
                />
              </svg>
              <img
                :src="getFaviconUrl(result.url)"
                :alt="getDomain(result.url)"
                class="absolute inset-0 m-auto h-4 w-4 rounded-sm"
                loading="lazy"
                @error="($event.target as HTMLImageElement).style.display = 'none'"
              />
            </span>

            <span class="min-w-0 flex-1">
              <span
                class="block line-clamp-1 text-sm font-semibold leading-snug text-sky-700 transition-colors group-hover:text-sky-800 dark:text-sky-300 dark:group-hover:text-sky-200"
              >
                {{ result.title || getDomain(result.url) }}
              </span>
              <span class="mt-0.5 block truncate text-[11px] text-slate-500 dark:text-slate-400">
                {{ getDomain(result.url) }}
              </span>
              <p
                v-if="result.description"
                class="mt-1 line-clamp-2 text-xs leading-relaxed text-slate-600 dark:text-slate-300"
              >
                {{ result.description }}
              </p>
            </span>

            <span class="flex-shrink-0 pt-0.5 text-slate-300 dark:text-slate-600">
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
                  d="M14 5h5m0 0v5m0-5L10 14M5 9v10h10"
                />
              </svg>
            </span>
          </a>
        </div>

        <div v-else-if="isStreaming" class="px-4 py-6 text-center">
          <svg
            class="mx-auto h-4 w-4 animate-spin text-slate-400 dark:text-slate-500"
            fill="none"
            viewBox="0 0 24 24"
          >
            <circle
              class="opacity-25"
              cx="12"
              cy="12"
              r="10"
              stroke="currentColor"
              stroke-width="4"
            />
            <path
              class="opacity-75"
              fill="currentColor"
              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
            />
          </svg>
          <div class="mt-2 text-sm text-slate-500 dark:text-slate-400">
            {{ searchingLabel }}
          </div>
        </div>

        <div v-else class="px-4 py-5 text-sm text-slate-500 dark:text-slate-400">
          {{ statusMessage || t('common.noResponses', 'No responses') }}
        </div>
      </div>
    </transition>

    <Teleport to="body">
      <Transition name="preview-fade">
        <div
          v-if="previewSupported && activePreview && currentPreview"
          class="search-card__preview-popover fixed z-[9999] w-80 overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-2xl pointer-events-none dark:border-slate-700 dark:bg-slate-900"
          :style="{
            left: `${previewPosition.x}px`,
            top: `${previewPosition.y}px`,
          }"
        >
          <img
            v-if="currentPreview.image"
            :src="currentPreview.image"
            class="h-32 w-full object-cover"
            @error="($event.target as HTMLImageElement).style.display = 'none'"
          />

          <div class="p-3.5">
            <div class="mb-2 flex items-center gap-2">
              <img
                v-if="currentPreview.favicon"
                :src="currentPreview.favicon"
                class="h-4 w-4 rounded-sm"
                @error="($event.target as HTMLImageElement).style.display = 'none'"
              />
              <span class="truncate text-[11px] text-slate-500 dark:text-slate-400">
                {{ currentPreview.siteName || getDomain(activePreview) }}
              </span>
              <span
                v-if="currentPreview.loading"
                class="inline-flex rounded-full border border-slate-200 px-1.5 py-0.5 text-[10px] font-medium text-slate-400 dark:border-slate-700 dark:text-slate-500"
              >
                {{ searchingLabel }}
              </span>
            </div>

            <div class="text-sm font-semibold leading-snug text-slate-900 dark:text-slate-100">
              {{ currentPreview.title || getDomain(activePreview) }}
            </div>

            <p
              v-if="currentPreview.description"
              class="mt-1.5 line-clamp-3 text-xs leading-relaxed text-slate-600 dark:text-slate-300"
            >
              {{ currentPreview.description }}
            </p>
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<style scoped>
.search-card-content-enter-active,
.search-card-content-leave-active {
  overflow: hidden;
  transition:
    max-height 220ms ease,
    opacity 180ms ease,
    transform 180ms ease;
}

.search-card-content-enter-from,
.search-card-content-leave-to {
  max-height: 0;
  opacity: 0;
  transform: translateY(-4px);
}

.search-card-content-enter-to,
.search-card-content-leave-from {
  max-height: 960px;
  opacity: 1;
  transform: translateY(0);
}

.preview-fade-enter-active {
  transition:
    opacity 0.15s ease,
    transform 0.15s ease;
}

.preview-fade-leave-active {
  transition: opacity 0.1s ease;
}

.preview-fade-enter-from {
  opacity: 0;
  transform: translateY(4px);
}

.preview-fade-leave-to {
  opacity: 0;
}
</style>
