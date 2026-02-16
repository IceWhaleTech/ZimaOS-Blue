<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { usePersonalityStore } from '@/stores/personality'
import PersonalityDialog from '@/components/PersonalityDialog.vue'
import type { Personality } from '@/api/personality'

const { t } = useI18n()
const store = usePersonalityStore()
const showDialog = ref(false)

const emit = defineEmits<{
  (e: 'status-change', message: string): void
}>()

onMounted(async () => {
  await Promise.all([store.fetchPersonalities(), store.fetchActive()])
})

function openDialog() {
  showDialog.value = true
}

async function handleActivate(id: string) {
  try {
    await store.activatePersonality(id)
    emit('status-change', t('personality.activated'))
  } catch (e) {
    console.error('Failed to activate personality:', e)
  }
}

async function handleDelete(id: string) {
  if (!confirm(t('personality.confirmDelete'))) return
  try {
    await store.deletePersonality(id)
    emit('status-change', t('personality.deleted'))
  } catch (e) {
    console.error('Failed to delete personality:', e)
  }
}

function getPersonalityIcon(personality: Personality): string {
  if (personality.name.toLowerCase().includes('assistant')) return '🤖'
  if (personality.name.toLowerCase().includes('creative')) return '🎨'
  if (personality.name.toLowerCase().includes('professional')) return '💼'
  if (personality.name.toLowerCase().includes('friendly')) return '😊'
  return '✨'
}

function getTraitValue(p: Personality, trait: { key: string; value: string }): string {
  if (p.id === 'default') {
    const key = `personality.defaultTraits.${trait.key}`
    const translated = t(key)
    if (translated !== key) return translated
  }
  return trait.value
}
</script>

<template>
  <div class="personality-manager">
    <div class="flex items-center justify-between mb-4">
      <div>
        <h3 class="text-base font-semibold text-gray-900 dark:text-white">
          {{ t('personality.title') }}
        </h3>
        <p class="text-sm text-gray-500 dark:text-gray-400 mt-1">
          {{ t('personality.description') }}
        </p>
      </div>
      <button
        class="px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-600 text-white rounded-lg text-sm font-medium transition-colors flex items-center gap-2"
        @click="openDialog"
      >
        <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
        </svg>
        {{ t('personality.manage') }}
      </button>
    </div>

    <!-- Loading State -->
    <div v-if="store.loading" class="flex items-center justify-center py-8">
      <svg class="animate-spin h-8 w-8 text-gray-400" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
      </svg>
    </div>

    <!-- Empty State -->
    <div v-else-if="store.personalities.length === 0" class="text-center py-8">
      <div class="text-4xl mb-3">🎭</div>
      <p class="text-gray-500 dark:text-gray-400 text-sm mb-4">
        {{ t('personality.empty') }}
      </p>
      <button
        class="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded-lg text-sm font-medium transition-colors"
        @click="openDialog"
      >
        {{ t('personality.createFirst') }}
      </button>
    </div>

    <!-- Personality List -->
    <div v-else class="space-y-3">
      <div
        v-for="personality in store.personalities"
        :key="personality.id"
        class="bg-gray-50 dark:bg-slate-700/50 rounded-lg p-4 hover:bg-gray-100 dark:hover:bg-slate-700 transition-colors"
      >
        <div class="flex items-start gap-3">
          <!-- Icon -->
          <div class="text-2xl flex-shrink-0">
            {{ getPersonalityIcon(personality) }}
          </div>

          <!-- Content -->
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-2 mb-1">
              <h4 class="text-sm font-medium text-gray-900 dark:text-white truncate">
                {{ personality.id === 'default' ? t('personality.defaultName') : personality.name }}
              </h4>
              <span
                v-if="store.activePersonality?.id === personality.id"
                class="px-2 py-0.5 bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400 text-xs font-medium rounded"
              >
                {{ t('personality.active') }}
              </span>
            </div>
            <p class="text-xs text-gray-500 dark:text-gray-400 line-clamp-2">
              {{ personality.id === 'default' ? t('personality.defaultDescription') : (personality.description || personality.system_prompt) }}
            </p>

            <!-- Traits -->
            <div v-if="personality.traits && personality.traits.length > 0" class="flex flex-wrap gap-1 mt-2">
              <span
                v-for="trait in personality.traits.slice(0, 3)"
                :key="trait.key"
                class="px-2 py-0.5 bg-gray-200 dark:bg-slate-600 text-gray-700 dark:text-gray-300 text-xs rounded"
                :title="`${trait.key}: ${trait.value} (${trait.weight})`"
              >
                {{ trait.key }}: {{ getTraitValue(personality, trait) }}
              </span>
              <span
                v-if="personality.traits.length > 3"
                class="px-2 py-0.5 text-gray-500 dark:text-gray-400 text-xs"
              >
                +{{ personality.traits.length - 3 }}
              </span>
            </div>
          </div>

          <!-- Actions -->
          <div class="flex flex-col gap-1 flex-shrink-0">
            <button
              v-if="store.activePersonality?.id !== personality.id"
              class="px-3 py-1 bg-gray-700 hover:bg-gray-600 text-white rounded text-xs font-medium transition-colors"
              @click="handleActivate(personality.id)"
            >
              {{ t('personality.activate') }}
            </button>
            <button
              v-if="personality.id !== 'default'"
              class="px-3 py-1 bg-red-100 dark:bg-red-900/30 hover:bg-red-200 dark:hover:bg-red-900/50 text-red-600 dark:text-red-400 rounded text-xs font-medium transition-colors"
              @click="handleDelete(personality.id)"
            >
              {{ t('personality.delete') }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Quick Stats -->
    <div v-if="store.personalities.length > 0" class="mt-4 pt-4 border-t border-gray-200 dark:border-slate-600">
      <div class="flex items-center justify-between text-sm">
        <span class="text-gray-500 dark:text-gray-400">
          {{ t('personality.total') }}: <span class="font-medium text-gray-900 dark:text-white">{{ store.personalities.length }}</span>
        </span>
        <button
          class="text-blue-600 dark:text-blue-400 hover:underline text-sm"
          @click="openDialog"
        >
          {{ t('personality.viewAll') }} →
        </button>
      </div>
    </div>

    <!-- Personality Management Dialog -->
    <PersonalityDialog v-if="showDialog" @close="showDialog = false" />
  </div>
</template>

<style scoped>
.line-clamp-2 {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
</style>
