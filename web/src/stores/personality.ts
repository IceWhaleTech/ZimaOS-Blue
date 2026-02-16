import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { personalityApi, type Personality } from '@/api/personality'

export const usePersonalityStore = defineStore('personality', () => {
  const personalities = ref<Personality[]>([])
  const activePersonality = ref<Personality | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)

  const fetchPersonalities = async () => {
    loading.value = true
    try {
      const response = await personalityApi.list()
      personalities.value = response.data
      error.value = null
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch personalities'
    } finally {
      loading.value = false
    }
  }

  const fetchActive = async () => {
    try {
      const response = await personalityApi.getActive()
      activePersonality.value = response.data
    } catch (e) {
      activePersonality.value = null
    }
  }

  const createPersonality = async (name: string, description: string, systemPrompt: string) => {
    try {
      const response = await personalityApi.create({ name, description, system_prompt: systemPrompt })
      personalities.value.push(response.data)
      return response.data
    } catch (e) {
      throw e
    }
  }

  const updatePersonality = async (id: string, name: string, description: string, systemPrompt: string) => {
    try {
      const response = await personalityApi.update(id, { name, description, system_prompt: systemPrompt })
      const index = personalities.value.findIndex(p => p.id === id)
      if (index !== -1) {
        personalities.value[index] = response.data
      }
      return response.data
    } catch (e) {
      throw e
    }
  }

  const deletePersonality = async (id: string) => {
    try {
      await personalityApi.delete(id)
      personalities.value = personalities.value.filter(p => p.id !== id)
      if (activePersonality.value?.id === id) {
        activePersonality.value = null
      }
    } catch (e) {
      throw e
    }
  }

  const activatePersonality = async (id: string) => {
    try {
      await personalityApi.activate(id)
      personalities.value = personalities.value.map(p => ({
        ...p,
        is_active: p.id === id,
      }))
      await fetchActive()
    } catch (e) {
      throw e
    }
  }

  return {
    personalities,
    activePersonality,
    loading,
    error,
    fetchPersonalities,
    fetchActive,
    createPersonality,
    updatePersonality,
    deletePersonality,
    activatePersonality,
  }
})
