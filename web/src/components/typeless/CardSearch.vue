<script setup lang="ts">
import { computed, ref, reactive } from 'vue'
import { useI18n } from 'vue-i18n'
import type { TypelessCardSearch, SearchResultItem } from '@/types/typeless'
import api from '@/api/index'
import { usePersistentDisclosureState } from '@/utils/chatCardUiState'

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
const resultCountLabel = computed(() => {
  if (summaryCount.value <= 0) return ''
  if (te('search.resultCount')) {
    return String(t('search.resultCount', { count: summaryCount.value }))
  }
  return summaryCount.value === 1 ? '1 result' : `${summaryCount.value} results`
})

function validResult(r: unknown): r is SearchResultItem {
  if (!r || typeof r !== 'object') return false
  const item = r as Record<string, unknown>
  return typeof item.url === 'string' && item.url.length > 0
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

// Link preview popover state
interface LinkPreview {
  title?: string
  description?: string
  image?: string
  favicon?: string
  siteName?: string
  loading: boolean
  error: boolean
}

const previewCache = reactive<Map<string, LinkPreview>>(new Map())
const activePreview = ref<string | null>(null)
const previewPosition = ref({ x: 0, y: 0 })
let hoverTimer: ReturnType<typeof setTimeout> | null = null

function onResultMouseEnter(url: string, event: MouseEvent) {
  if (hoverTimer) clearTimeout(hoverTimer)
  hoverTimer = setTimeout(() => {
    activePreview.value = url
    updatePreviewPosition(event)
    if (!previewCache.has(url)) {
      fetchPreview(url)
    }
  }, 400)
}

function onResultMouseMove(event: MouseEvent) {
  if (activePreview.value) {
    updatePreviewPosition(event)
  }
}

function onResultMouseLeave() {
  if (hoverTimer) {
    clearTimeout(hoverTimer)
    hoverTimer = null
  }
  activePreview.value = null
}

function updatePreviewPosition(event: MouseEvent) {
  const rect = (event.currentTarget as HTMLElement)
    ?.closest('.search-card')
    ?.getBoundingClientRect()
  if (rect) {
    previewPosition.value = {
      x: rect.right + 8,
      y: Math.min(event.clientY - 60, window.innerHeight - 260),
    }
  }
}

async function fetchPreview(url: string) {
  previewCache.set(url, { loading: true, error: false })
  try {
    const response = await api.get('/link-preview', {
      params: { url },
      timeout: 8000,
    })
    if (response.data) {
      previewCache.set(url, {
        title: response.data.title || undefined,
        description: response.data.description || undefined,
        image: response.data.image || undefined,
        favicon: response.data.favicon || undefined,
        siteName: response.data.siteName || undefined,
        loading: false,
        error: false,
      })
    }
  } catch {
    previewCache.set(url, { loading: false, error: true })
  }
}

const currentPreview = computed(() => {
  if (!activePreview.value) return null
  return previewCache.get(activePreview.value) || null
})
</script>

<template>
  <div
    class="search-card rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 overflow-hidden shadow-sm relative"
  >
    <button
      class="flex w-full items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-gray-50 dark:hover:bg-gray-700/30"
      :aria-expanded="expanded ? 'true' : 'false'"
      @click="toggleExpanded"
    >
      <span
        class="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg bg-sky-100 text-sky-600 dark:bg-sky-900/40 dark:text-sky-300"
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
          class="block text-[11px] font-semibold uppercase tracking-[0.08em] text-gray-500 dark:text-gray-400"
          >{{ summaryTitle }}</span
        >
        <span class="block truncate text-sm font-medium text-gray-800 dark:text-gray-100">
          {{ summaryLabel }}
        </span>
      </span>
      <span class="flex flex-shrink-0 items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
        <span
          v-if="summaryCount > 0"
          class="rounded-full bg-black/5 px-2 py-0.5 dark:bg-white/10 tabular-nums"
        >
          {{ resultCountLabel }}
        </span>
        <span
          v-if="isStreaming"
          class="rounded-full bg-sky-100 px-2 py-0.5 text-sky-700 dark:bg-sky-900/30 dark:text-sky-300"
        >
          {{ searchingLabel }}
        </span>
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-4 w-4 text-gray-400 transition-transform duration-200"
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
      <div v-if="expanded" class="border-t border-gray-100 dark:border-gray-700/60">
        <div v-if="results.length > 0" class="divide-y divide-gray-100 dark:divide-gray-700/50">
          <a
            v-for="(result, i) in results"
            :key="i"
            :href="result.url"
            target="_blank"
            rel="noopener noreferrer"
            class="flex items-start gap-3 px-4 py-2.5 hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors group"
            @mouseenter="onResultMouseEnter(result.url, $event)"
            @mousemove="onResultMouseMove"
            @mouseleave="onResultMouseLeave"
          >
            <div
              class="w-4 h-4 mt-0.5 flex-shrink-0 rounded-sm bg-gray-100 dark:bg-gray-700 relative"
            >
              <svg
                aria-hidden="true"
                xmlns="http://www.w3.org/2000/svg"
                class="w-2.5 h-2.5 text-gray-400 dark:text-gray-500 absolute inset-0 m-auto"
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
                class="w-4 h-4 rounded-sm absolute inset-0"
                loading="lazy"
                @error="($event.target as HTMLImageElement).style.display = 'none'"
              />
            </div>
            <div class="flex-1 min-w-0">
              <div class="text-sm font-medium text-blue-600 dark:text-blue-400 line-clamp-1">
                {{ result.title || getDomain(result.url) }}
              </div>
              <div class="text-xs text-green-700 dark:text-green-500/80 truncate mt-0.5">
                {{ getDomain(result.url) }}
              </div>
              <p
                v-if="result.description"
                class="mt-0.5 text-xs text-gray-500 dark:text-gray-400 line-clamp-1"
              >
                {{ result.description }}
              </p>
            </div>
          </a>
        </div>

        <div v-else-if="isStreaming" class="px-4 py-6 text-center">
          <svg
            class="animate-spin h-4 w-4 mx-auto text-gray-400 dark:text-gray-500"
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
          <div class="mt-2 text-sm text-gray-400 dark:text-gray-500">
            {{ t('common.searching', 'Searching...') }}
          </div>
        </div>

        <div v-else class="px-4 py-5 text-center text-sm text-gray-500 dark:text-gray-400">
          {{ t('common.noResponses', 'No responses') }}
        </div>
      </div>
    </transition>

    <!-- Link Preview Popover (teleported to body to avoid overflow clipping) -->
    <Teleport to="body">
      <Transition name="preview-fade">
        <div
          v-if="activePreview && currentPreview && !currentPreview.error"
          class="fixed z-[9999] w-72 rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 shadow-xl overflow-hidden pointer-events-none"
          :style="{ left: previewPosition.x + 'px', top: previewPosition.y + 'px' }"
        >
          <!-- Loading state -->
          <template v-if="currentPreview.loading">
            <div class="p-3 space-y-2">
              <div class="h-3 bg-gray-200 dark:bg-gray-700 rounded animate-pulse w-3/4" />
              <div class="h-2.5 bg-gray-200 dark:bg-gray-700 rounded animate-pulse w-full" />
              <div class="h-2.5 bg-gray-200 dark:bg-gray-700 rounded animate-pulse w-2/3" />
            </div>
          </template>
          <!-- Preview content -->
          <template v-else>
            <img
              v-if="currentPreview.image"
              :src="currentPreview.image"
              class="w-full h-32 object-cover"
              @error="($event.target as HTMLImageElement).style.display = 'none'"
            />
            <div class="p-3">
              <div class="flex items-center gap-1.5 mb-1.5">
                <img
                  v-if="currentPreview.favicon"
                  :src="currentPreview.favicon"
                  class="w-3.5 h-3.5 rounded-sm"
                  @error="($event.target as HTMLImageElement).style.display = 'none'"
                />
                <span class="text-[11px] text-gray-400 dark:text-gray-500 truncate">
                  {{ currentPreview.siteName || getDomain(activePreview) }}
                </span>
              </div>
              <div
                v-if="currentPreview.title"
                class="text-sm font-medium text-gray-900 dark:text-gray-100 line-clamp-2 leading-snug"
              >
                {{ currentPreview.title }}
              </div>
              <p
                v-if="currentPreview.description"
                class="mt-1 text-xs text-gray-500 dark:text-gray-400 line-clamp-3 leading-relaxed"
              >
                {{ currentPreview.description }}
              </p>
            </div>
          </template>
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
</style>

<style scoped>
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
