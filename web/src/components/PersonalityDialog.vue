<template>
  <Teleport to="body">
    <div class="fixed inset-0 bg-black/50 flex items-center justify-center z-50" @click="$emit('close')">
      <div
        class="bg-white dark:bg-gray-800 rounded-xl w-full max-w-4xl mx-4 max-h-[85vh] flex flex-col"
        @click.stop
      >
        <!-- Header -->
        <div class="flex items-center justify-between px-6 py-4 border-b border-gray-200 dark:border-gray-700">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('personality.title') || 'Personalities' }}
          </h2>
          <div class="flex items-center gap-3">
            <button
              class="px-4 py-2 bg-gray-800 dark:bg-gray-500 hover:bg-gray-900 dark:hover:bg-gray-400 text-white rounded-lg text-sm font-medium transition-colors"
              @click="openNewForm"
            >
              {{ t('personality.new') || 'New Personality' }}
            </button>
            <button
              class="p-1.5 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
              @click="$emit('close')"
            >
              <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
        </div>

        <!-- Content -->
        <div class="flex-1 overflow-y-auto p-6">
          <PersonalityList
            :personalities="store.personalities"
            :active-id="store.activePersonality?.id"
            :loading="store.loading"
            @edit="openEditForm"
            @activate="store.activatePersonality"
            @delete="store.deletePersonality"
          />
        </div>

        <!-- Edit/Create Form (overlay) -->
        <PersonalityForm
          v-if="showForm"
          :personality="selectedPersonality"
          @save="handleSave"
          @close="closeForm"
        />
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { usePersonalityStore } from '@/stores/personality'
import PersonalityList from '@/components/PersonalityList.vue'
import PersonalityForm from '@/components/PersonalityForm.vue'
import type { Personality } from '@/api/personality'

const { t } = useI18n()
const store = usePersonalityStore()

defineEmits<{ close: [] }>()

const showForm = ref(false)
const selectedPersonality = ref<Personality | null>(null)

onMounted(async () => {
  await store.fetchPersonalities()
})

function openNewForm() {
  selectedPersonality.value = null
  showForm.value = true
}

function openEditForm(personality: Personality) {
  selectedPersonality.value = personality
  showForm.value = true
}

function closeForm() {
  showForm.value = false
  selectedPersonality.value = null
}

async function handleSave() {
  await store.fetchPersonalities()
  closeForm()
}
</script>
