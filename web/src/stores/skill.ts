import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { skillApi } from '@/api/skill'
import type { Skill, SkillSource, RemoteSkill, BrowseParams, BrowseResponse } from '@/api/skill'

export const useSkillStore = defineStore('skill', () => {
  // State
  const skills = ref<Skill[]>([])
  const sources = ref<SkillSource[]>([])
  const remoteSkills = ref<RemoteSkill[]>([])
  const loading = ref(false)
  const refreshing = ref(false)
  const error = ref<string | null>(null)

  // Computed
  const enabledSkills = computed(() => skills.value.filter((s) => s.enabled))
  const disabledSkills = computed(() => skills.value.filter((s) => !s.enabled))
  const builtinSkills = computed(() => skills.value.filter((s) => s.builtin))
  const installedSkills = computed(() => skills.value.filter((s) => !s.builtin))

  const skillsByCategory = computed(() => {
    const grouped: Record<string, Skill[]> = {}
    skills.value.forEach((s) => {
      const category = s.category || 'other'
      if (!grouped[category]) {
        grouped[category] = []
      }
      grouped[category].push(s)
    })
    return grouped
  })

  const remoteSkillsBySource = computed(() => {
    const grouped: Record<string, RemoteSkill[]> = {}
    remoteSkills.value.forEach((s) => {
      if (!grouped[s.source_id]) {
        grouped[s.source_id] = []
      }
      grouped[s.source_id].push(s)
    })
    return grouped
  })

  const categories = computed(() => {
    const cats = new Set<string>()
    skills.value.forEach((s) => {
      if (s.category) cats.add(s.category)
    })
    remoteSkills.value.forEach((s) => {
      if (s.category) cats.add(s.category)
    })
    return Array.from(cats).sort()
  })

  // Actions
  async function fetchSkills() {
    try {
      loading.value = true
      error.value = null
      const response = await skillApi.list()
      skills.value = response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch skills'
    } finally {
      loading.value = false
    }
  }

  async function enableSkill(id: string) {
    try {
      loading.value = true
      error.value = null
      await skillApi.enable(id)
      const skill = skills.value.find((s) => s.id === id)
      if (skill) {
        skill.enabled = true
      }
      return true
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to enable skill'
      return false
    } finally {
      loading.value = false
    }
  }

  async function disableSkill(id: string) {
    try {
      loading.value = true
      error.value = null
      await skillApi.disable(id)
      const skill = skills.value.find((s) => s.id === id)
      if (skill) {
        skill.enabled = false
      }
      return true
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to disable skill'
      return false
    } finally {
      loading.value = false
    }
  }

  async function fetchSources() {
    try {
      const response = await skillApi.listSources()
      sources.value = response.data
    } catch (e) {
      console.error('Failed to fetch sources:', e)
    }
  }

  async function addSource(source: Omit<SkillSource, 'enabled'> & { enabled?: boolean }) {
    try {
      loading.value = true
      error.value = null
      await skillApi.addSource(source)
      await fetchSources()
      return true
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to add source'
      return false
    } finally {
      loading.value = false
    }
  }

  async function removeSource(id: string) {
    try {
      loading.value = true
      error.value = null
      await skillApi.removeSource(id)
      sources.value = sources.value.filter((s) => s.id !== id)
      return true
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to remove source'
      return false
    } finally {
      loading.value = false
    }
  }

  async function browseSkills(params?: BrowseParams) {
    try {
      loading.value = true
      error.value = null
      const response = await skillApi.browse(params)
      // Handle both paginated response (object with skills array) and legacy array response
      if (response.data && 'skills' in response.data) {
        remoteSkills.value = response.data.skills || []
      } else if (Array.isArray(response.data)) {
        remoteSkills.value = response.data
      } else {
        remoteSkills.value = []
      }
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to browse skills'
    } finally {
      loading.value = false
    }
  }

  async function installSkill(id: string) {
    try {
      loading.value = true
      error.value = null
      const response = await skillApi.install(id)
      if (response.data.success) {
        // Mark as installed in remote skills
        const skill = remoteSkills.value.find((s) => s.id === id)
        if (skill) {
          skill.installed = true
        }
        // Refresh local skills
        await fetchSkills()
      }
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to install skill'
      return { success: false, message: error.value }
    } finally {
      loading.value = false
    }
  }

  async function uninstallSkill(id: string) {
    try {
      loading.value = true
      error.value = null
      const response = await skillApi.uninstall(id)
      if (response.data.success) {
        // Remove from local skills
        skills.value = skills.value.filter((s) => s.id !== id)
        // Mark as not installed in remote skills
        const skill = remoteSkills.value.find((s) => s.id === id)
        if (skill) {
          skill.installed = false
        }
      }
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to uninstall skill'
      return { success: false, message: error.value }
    } finally {
      loading.value = false
    }
  }

  async function refreshSources() {
    try {
      refreshing.value = true
      error.value = null
      const response = await skillApi.refresh()
      if (response.data.errors && response.data.errors.length > 0) {
        error.value = response.data.errors.join(', ')
      }
      // Refresh browse results
      await browseSkills()
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to refresh sources'
      return { success: false, skills_count: 0 }
    } finally {
      refreshing.value = false
    }
  }

  function clearError() {
    error.value = null
  }

  return {
    // State
    skills,
    sources,
    remoteSkills,
    loading,
    refreshing,
    error,

    // Computed
    enabledSkills,
    disabledSkills,
    builtinSkills,
    installedSkills,
    skillsByCategory,
    remoteSkillsBySource,
    categories,

    // Actions
    fetchSkills,
    enableSkill,
    disableSkill,
    fetchSources,
    addSource,
    removeSource,
    browseSkills,
    installSkill,
    uninstallSkill,
    refreshSources,
    clearError,
  }
})
