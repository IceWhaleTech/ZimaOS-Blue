<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { myApi, type UserSkillConfig } from '@/api/my'

const { t } = useI18n()
const skills = ref<UserSkillConfig[]>([])
const loading = ref(true)

onMounted(async () => {
  loading.value = true
  try {
    const { data } = await myApi.listSkills()
    skills.value = data
  } finally {
    loading.value = false
  }
})

async function toggleSkill(skill: UserSkillConfig) {
  const prev = skill.enabled
  skill.enabled = !prev
  try {
    await myApi.toggleSkill(skill.id, skill.enabled)
  } catch {
    skill.enabled = prev // rollback on failure
  }
}
</script>

<template>
  <div class="max-w-4xl mx-auto">
    <h1 class="text-2xl font-bold text-gray-900 dark:text-white mb-2">{{ t('mySkills.title') }}</h1>
    <p class="text-sm text-gray-500 dark:text-gray-400 mb-6">{{ t('mySkills.description') }}</p>

    <div v-if="loading" class="text-gray-500 dark:text-gray-400">{{ t('common.loading') }}...</div>
    <div v-else-if="skills.length === 0" class="text-gray-500 dark:text-gray-400">{{ t('mySkills.empty') }}</div>

    <div v-else class="space-y-3">
      <div
        v-for="skill in skills"
        :key="skill.id"
        class="bg-white dark:bg-gray-800 rounded-xl p-4 border border-gray-200 dark:border-gray-700 flex items-center justify-between"
      >
        <div class="flex-1 min-w-0">
          <div class="font-medium text-gray-900 dark:text-white">{{ skill.name }}</div>
          <div class="text-sm text-gray-500 dark:text-gray-400 truncate">{{ skill.summary }}</div>
          <div class="text-xs text-gray-400 dark:text-gray-500 mt-1">
            {{ skill.category }} · {{ skill.author }}
          </div>
        </div>
        <button
          class="ml-4 relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none"
          :class="skill.enabled ? 'bg-blue-600' : 'bg-gray-200 dark:bg-gray-600'"
          @click="toggleSkill(skill)"
        >
          <span
            class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
            :class="skill.enabled ? 'translate-x-5' : 'translate-x-0'"
          />
        </button>
      </div>
    </div>
  </div>
</template>
