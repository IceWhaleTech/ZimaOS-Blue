<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { usePersonalityStore } from '@/stores/personality'
import PersonalityList from '@/components/PersonalityList.vue'
import PersonalityForm from '@/components/PersonalityForm.vue'

const { t } = useI18n()
const store = usePersonalityStore()

const showForm = ref(false)
const selectedPersonality = ref(null)

onMounted(async () => {
  await store.fetchPersonalities()
})

function openNewForm() {
  selectedPersonality.value = null
  showForm.value = true
}

function openEditForm(personality: any) {
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

<template>
  <div class="personality-view p-4 sm:p-6 max-w-6xl mx-auto">
    <div class="flex items-center justify-between mb-6">
      <h1 class="text-xl sm:text-2xl font-bold text-gray-900 dark:text-white">
        {{ t('personality.title') || 'Personalities' }}
      </h1>
      <button
        class="px-4 py-2 bg-gray-800 dark:bg-gray-500 hover:bg-gray-900 dark:hover:bg-gray-400 text-white rounded-lg font-medium transition-colors"
        @click="openNewForm"
      >
        {{ t('personality.new') || 'New Personality' }}
      </button>
    </div>

    <PersonalityList
      :personalities="store.personalities"
      :active-id="store.activePersonality?.id"
      :loading="store.loading"
      @edit="openEditForm"
      @activate="store.activatePersonality"
      @delete="store.deletePersonality"
    />

    <PersonalityForm
      v-if="showForm"
      :personality="selectedPersonality"
      @save="handleSave"
      @close="closeForm"
    />
  </div>
</template>

<style scoped>
.personality-view {
  min-height: 100vh;
}
</style>
