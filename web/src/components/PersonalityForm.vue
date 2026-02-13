<template>
  <div class="fixed inset-0 bg-black/50 flex items-center justify-center z-50" @click="$emit('close')">
    <div class="bg-white dark:bg-gray-800 rounded-xl p-6 w-full max-w-3xl mx-4 max-h-[90vh] overflow-y-auto" @click.stop>
      <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">{{ personality ? 'Edit Personality' : 'New Personality' }}</h3>

      <form @submit.prevent="handleSubmit" class="space-y-4">
        <div class="grid grid-cols-2 gap-4">
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Name</label>
            <input v-model="form.name" type="text" required class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 border border-gray-200 dark:border-gray-600 focus:outline-none focus:ring-2 focus:ring-gray-400" />
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Description</label>
            <input v-model="form.description" type="text" class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 border border-gray-200 dark:border-gray-600 focus:outline-none focus:ring-2 focus:ring-gray-400" />
          </div>
        </div>

        <!-- System Prompt Markdown Editor -->
        <div>
          <div class="flex items-center justify-between mb-1">
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">System Prompt (Markdown)</label>
            <div class="flex gap-1">
              <button
                v-for="m in (['edit', 'split', 'preview'] as const)"
                :key="m"
                type="button"
                :class="[
                  'px-2 py-0.5 text-xs rounded transition-colors',
                  editorMode === m
                    ? 'bg-gray-700 dark:bg-gray-500 text-white'
                    : 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400 hover:bg-gray-200 dark:hover:bg-gray-600'
                ]"
                @click="editorMode = m"
              >
                {{ m === 'edit' ? 'Edit' : m === 'split' ? 'Split' : 'Preview' }}
              </button>
            </div>
          </div>

          <div :class="['border border-gray-200 dark:border-gray-600 rounded-lg overflow-hidden', editorMode === 'split' ? 'grid grid-cols-2' : '']">
            <textarea
              v-if="editorMode !== 'preview'"
              v-model="form.systemPrompt"
              rows="16"
              required
              class="w-full bg-gray-50 dark:bg-gray-900 text-gray-900 dark:text-gray-100 px-4 py-3 font-mono text-sm focus:outline-none resize-none"
              :class="{ 'border-r border-gray-200 dark:border-gray-600': editorMode === 'split' }"
              placeholder="# Personality&#10;&#10;Write your system prompt in **Markdown** format..."
            ></textarea>

            <div
              v-if="editorMode !== 'edit'"
              class="bg-white dark:bg-gray-800 px-4 py-3 overflow-y-auto prose-content text-sm text-gray-900 dark:text-gray-100"
              :class="editorMode === 'split' ? 'max-h-[384px]' : 'min-h-[384px]'"
              v-html="renderedPrompt"
            />
          </div>
        </div>

        <div class="flex gap-2 pt-2">
          <button type="button" @click="$emit('close')" class="flex-1 px-4 py-2 bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors">Cancel</button>
          <button type="submit" class="flex-1 px-4 py-2 bg-gray-800 dark:bg-gray-500 text-white rounded-lg hover:bg-gray-900 dark:hover:bg-gray-400 transition-colors">Save</button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { usePersonalityStore } from '@/stores/personality'
import { renderMarkdown } from '@/utils/markdown'
import type { Personality } from '@/api/personality'

const props = defineProps<{
  personality?: Personality | null
}>()

const emit = defineEmits<{
  save: []
  close: []
}>()

const store = usePersonalityStore()
const editorMode = ref<'edit' | 'split' | 'preview'>('split')
const form = ref({
  name: '',
  description: '',
  systemPrompt: '',
})

const renderedPrompt = computed(() => {
  if (!form.value.systemPrompt) return '<p class="text-gray-400 italic">No content yet</p>'
  return renderMarkdown(form.value.systemPrompt)
})

watch(
  () => props.personality,
  (p) => {
    if (p) {
      form.value = {
        name: p.name,
        description: p.description,
        systemPrompt: p.system_prompt,
      }
    } else {
      form.value = { name: '', description: '', systemPrompt: '' }
    }
  },
  { immediate: true }
)

const handleSubmit = async () => {
  try {
    if (props.personality) {
      await store.updatePersonality(
        props.personality.id,
        form.value.name,
        form.value.description,
        form.value.systemPrompt
      )
    } else {
      await store.createPersonality(
        form.value.name,
        form.value.description,
        form.value.systemPrompt
      )
    }
    emit('save')
  } catch (e) {
    alert('Error: ' + (e instanceof Error ? e.message : 'Unknown error'))
  }
}
</script>
