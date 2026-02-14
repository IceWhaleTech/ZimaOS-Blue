<template>
  <div class="space-y-4">
    <div v-if="loading" class="text-center py-10 text-gray-500 dark:text-gray-400">
      {{ t('personality.loading') }}
    </div>

    <div v-else-if="personalities.length === 0" class="text-center py-10 text-gray-500 dark:text-gray-400">
      {{ t('personality.noPersonalitiesDesc') }}
    </div>

    <div v-else class="grid gap-4">
      <div
        v-for="p in personalities"
        :key="p.id"
        class="border rounded-lg p-5 transition-all"
        :class="activeId === p.id
          ? 'border-green-300 dark:border-green-700 bg-green-50/50 dark:bg-green-900/10'
          : 'border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800'"
      >
        <!-- Header -->
        <div class="flex justify-between items-start mb-2">
          <div class="flex items-center gap-2">
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">
              {{ p.id === 'default' ? t('personality.defaultName') : p.name }}
            </h3>
            <span
              v-if="activeId === p.id"
              class="px-2 py-0.5 text-xs font-medium bg-green-100 dark:bg-green-900/40 text-green-700 dark:text-green-300 rounded-full"
            >
              {{ t('personality.active') }}
            </span>
            <span
              v-if="p.id === 'default'"
              class="px-2 py-0.5 text-xs font-medium bg-gray-100 dark:bg-gray-700 text-gray-500 dark:text-gray-400 rounded-full"
            >
              {{ t('personality.default') }}
            </span>
          </div>
          <!-- Actions -->
          <div class="flex items-center gap-1.5">
            <button
              v-if="activeId !== p.id"
              @click="$emit('activate', p.id)"
              class="px-3 py-1.5 text-xs font-medium text-green-600 dark:text-green-400 hover:bg-green-50 dark:hover:bg-green-900/20 rounded-md transition-colors"
            >
              {{ t('personality.activate') }}
            </button>
            <button
              @click="$emit('edit', p)"
              class="px-3 py-1.5 text-xs font-medium text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-md transition-colors"
            >
              {{ t('personality.edit') }}
            </button>
            <button
              v-if="p.id !== 'default'"
              @click="$emit('delete', p.id)"
              class="px-3 py-1.5 text-xs font-medium text-red-500 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-md transition-colors"
            >
              {{ t('personality.delete') }}
            </button>
          </div>
        </div>

        <!-- Description -->
        <p class="text-sm text-gray-500 dark:text-gray-400 mb-3">
          {{ p.id === 'default' ? t('personality.defaultDescription') : p.description }}
        </p>

        <!-- Traits -->
        <div v-if="p.traits && p.traits.length > 0" class="flex flex-wrap gap-2 mb-3">
          <span
            v-for="trait in p.traits"
            :key="trait.key"
            class="inline-flex items-center gap-1 px-2.5 py-1 text-xs bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 rounded-full"
          >
            <span class="font-medium">{{ trait.key }}:</span> {{ getTraitValue(p, trait) }}
          </span>
        </div>

        <!-- System Prompt Summary -->
        <div
          class="text-sm text-gray-500 dark:text-gray-400 bg-gray-50 dark:bg-gray-700/30 rounded-md px-3 py-2 cursor-pointer select-none"
          @click="toggleExpand(p.id)"
        >
          <div class="flex items-center justify-between">
            <span class="text-xs font-medium text-gray-400 dark:text-gray-500">
              {{ t('personality.systemPrompt') }}
            </span>
            <svg
              class="w-4 h-4 text-gray-400 transition-transform"
              :class="{ 'rotate-180': expandedId === p.id }"
              fill="none" stroke="currentColor" viewBox="0 0 24 24"
            >
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
            </svg>
          </div>
          <p v-if="expandedId !== p.id" class="mt-1 line-clamp-2">
            {{ extractSummary(p.system_prompt) }}
          </p>
          <div
            v-else
            class="mt-2 prose-content max-h-80 overflow-y-auto text-xs text-gray-600 dark:text-gray-300"
            v-html="renderMarkdown(p.system_prompt)"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Personality } from '@/api/personality'
import { renderMarkdown } from '@/utils/markdown'

const { t } = useI18n()
const expandedId = ref<string | null>(null)

defineProps<{
  personalities: Personality[]
  activeId?: string
  loading?: boolean
}>()

defineEmits<{
  edit: [personality: Personality]
  activate: [id: string]
  delete: [id: string]
}>()

function toggleExpand(id: string) {
  expandedId.value = expandedId.value === id ? null : id
}

function getTraitValue(p: Personality, trait: { key: string; value: string }): string {
  if (p.id === 'default') {
    const key = `personality.defaultTraits.${trait.key}`
    const translated = t(key)
    if (translated !== key) return translated
  }
  return trait.value
}

function extractSummary(systemPrompt: string): string {
  if (!systemPrompt) return ''
  // Strip markdown headers, blank lines, and extract meaningful text
  const lines = systemPrompt.split('\n')
    .map(l => l.replace(/^#{1,6}\s+/, '').trim())
    .filter(l => l && !l.startsWith('---') && !l.startsWith('```'))
  return lines.slice(0, 3).join(' ').slice(0, 200) || systemPrompt.slice(0, 200)
}
</script>
