<script setup lang="ts">
import { computed } from 'vue'
import type { PresetQuestion } from '@/api/preview'

const props = defineProps<{
  question: PresetQuestion
  tagLabels?: string[]
}>()

const emit = defineEmits<{
  click: [question: PresetQuestion]
}>()

const hasAttachments = computed(
  () => props.question.attachments && props.question.attachments.length > 0
)
const hasImageAttachment = computed(() =>
  props.question.attachments?.some((a) => a.type === 'image')
)
const displayTitle = computed(() =>
  String(props.question.title || props.question.text || props.question.prompt || '').trim()
)
const displayDescription = computed(() => String(props.question.description || '').trim())
const displayPrompt = computed(() =>
  String(props.question.prompt || props.question.text || '').trim()
)
const displayPromptPreview = computed(() => {
  if (!displayPrompt.value) return ''
  if (
    displayPrompt.value === displayTitle.value ||
    displayPrompt.value === displayDescription.value
  ) {
    return ''
  }
  return displayPrompt.value
})
const visibleTagLabels = computed(() => (props.tagLabels || []).slice(0, 3))
const summaryTagLabels = computed(() => visibleTagLabels.value.slice(0, 2))
const hasExpandedContent = computed(
  () => displayDescription.value.length > 0 || displayPromptPreview.value.length > 0
)
</script>

<template>
  <button
    class="preset-question-card group block w-full text-start"
    :data-testid="`preset-question-card-${question.id}`"
    :title="displayDescription || displayPromptPreview || displayTitle"
    :aria-label="displayDescription ? `${displayTitle}：${displayDescription}` : displayTitle"
    @click="emit('click', question)"
  >
    <div
      class="preset-question-card-base flex h-full min-h-[4.5rem] items-center gap-3 overflow-hidden rounded-[1.25rem] border border-slate-200/90 bg-white/90 px-4 py-3 transition-all duration-200 hover:border-slate-300 hover:bg-white dark:border-slate-700 dark:bg-slate-900/80 dark:hover:border-slate-500"
    >
      <span
        v-if="question.icon"
        class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-2xl bg-amber-50 text-base dark:bg-amber-500/10"
      >
        {{ question.icon }}
      </span>
      <div class="min-w-0 flex-1">
        <div v-if="summaryTagLabels.length > 0" class="mb-1 flex flex-wrap gap-1">
          <span
            v-for="tag in summaryTagLabels"
            :key="tag"
            class="inline-flex items-center rounded-full bg-slate-100 px-2 py-0.5 text-[10px] font-medium text-slate-500 dark:bg-slate-800 dark:text-slate-300"
          >
            {{ tag }}
          </span>
        </div>
        <span
          class="block line-clamp-2 text-[13px] font-semibold leading-5 text-slate-900 transition-colors group-hover:text-slate-950 dark:text-slate-100 dark:group-hover:text-white"
        >
          {{ displayTitle }}
        </span>
      </div>
      <span
        v-if="hasAttachments"
        class="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-xl bg-slate-100/80 text-slate-400 dark:bg-slate-800/80 dark:text-slate-500"
      >
        <svg
          v-if="hasImageAttachment"
          class="h-4 w-4"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z"
          />
        </svg>
        <svg v-else class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M15.172 7l-6.586 6.586a2 2 0 102.828 2.828l6.414-6.586a4 4 0 00-5.656-5.656l-6.415 6.585a6 6 0 108.486 8.486L20.5 13"
          />
        </svg>
      </span>
    </div>
    <div v-if="hasExpandedContent" class="preset-question-card-expanded">
      <div
        class="preset-question-card-expanded-surface rounded-[1rem] border border-slate-200/90 bg-white/96 px-3 py-3 text-slate-600 shadow-[0_14px_28px_rgba(15,23,42,0.1)] dark:border-slate-700 dark:bg-slate-900/96 dark:text-slate-300"
      >
        <span
          v-if="displayDescription"
          class="block text-[11px] font-medium leading-4 text-slate-600 dark:text-slate-300"
        >
          {{ displayDescription }}
        </span>
        <div
          v-if="displayPromptPreview"
          class="mt-2 rounded-xl border border-slate-200/80 bg-slate-50/90 px-2.5 py-2 text-[11px] leading-4 text-slate-700 dark:border-slate-700 dark:bg-slate-950/70 dark:text-slate-200"
        >
          <span class="line-clamp-3 block">
            {{ displayPromptPreview }}
          </span>
        </div>
      </div>
    </div>
  </button>
</template>

<style scoped>
.preset-question-card {
  overflow: hidden;
}

.preset-question-card-expanded {
  max-height: 0;
  margin-top: 0;
  overflow: hidden;
  opacity: 0;
  transform: translateY(-4px);
  transition:
    opacity 180ms ease,
    transform 220ms ease,
    max-height 220ms ease,
    margin-top 220ms ease;
}

.preset-question-card-expanded-surface {
  backdrop-filter: blur(14px);
}

.preset-question-card:focus-visible {
  outline: none;
}

.preset-question-card:focus-visible .preset-question-card-base {
  border-color: rgba(148, 163, 184, 0.9);
  background: rgba(255, 255, 255, 1);
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.08);
}

.preset-question-card:focus-visible .preset-question-card-expanded {
  opacity: 1;
  max-height: 10rem;
  margin-top: 0.35rem;
  transform: translateY(0);
}

@media (hover: hover) and (pointer: fine) {
  .preset-question-card:hover .preset-question-card-base {
    border-color: rgba(148, 163, 184, 0.9);
    background: rgba(255, 255, 255, 1);
    box-shadow: 0 10px 24px rgba(15, 23, 42, 0.08);
  }

  .preset-question-card:hover .preset-question-card-expanded {
    opacity: 1;
    max-height: 10rem;
    margin-top: 0.35rem;
    transform: translateY(0);
  }
}
</style>
