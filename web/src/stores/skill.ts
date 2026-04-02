import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { skillApi } from '@/api/skill'
import { i18n } from '@/i18n'
import type {
  Skill,
  SkillSource,
  RemoteSkill,
  BrowseParams,
  InstallFromURLRequest,
} from '@/api/skill'

export const useSkillStore = defineStore('skill', () => {
  const t = i18n.global.t.bind(i18n.global)
  const te = i18n.global.te.bind(i18n.global)

  // State
  const skills = ref<Skill[]>([])
  const sources = ref<SkillSource[]>([])
  const remoteSkills = ref<RemoteSkill[]>([])
  const loading = ref(false)
  const refreshing = ref(false)
  const error = ref<string | null>(null)

  // Pagination state
  const currentPage = ref(1)
  const totalPages = ref(1)
  const totalSkills = ref(0)
  const pageSize = ref(24)
  const loadingMore = ref(false)

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
      const sourceId = s.source_id || 'unknown'
      if (!grouped[sourceId]) {
        grouped[sourceId] = []
      }
      grouped[sourceId].push(s)
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
      const data = response.data
      // Handle both paginated response (object with skills array) and legacy array response
      if (data && typeof data === 'object' && 'skills' in data) {
        remoteSkills.value = data.skills || []
        // Update pagination state
        currentPage.value = data.page || 1
        totalPages.value = data.total_pages || 1
        totalSkills.value = data.total || 0
        pageSize.value = data.page_size || 24
      } else if (Array.isArray(data)) {
        const arr = data as RemoteSkill[]
        remoteSkills.value = arr
        totalSkills.value = arr.length
        totalPages.value = 1
        currentPage.value = 1
      } else {
        remoteSkills.value = []
        totalSkills.value = 0
        totalPages.value = 1
        currentPage.value = 1
      }
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to browse skills'
    } finally {
      loading.value = false
    }
  }

  async function loadMoreSkills(params?: BrowseParams) {
    if (loadingMore.value || currentPage.value >= totalPages.value) return false

    try {
      loadingMore.value = true
      error.value = null
      const nextPage = currentPage.value + 1
      const response = await skillApi.browse({
        ...params,
        page: nextPage,
        page_size: pageSize.value,
      })

      if (response.data && 'skills' in response.data) {
        // Append new skills to existing list
        const newSkills = response.data.skills || []
        remoteSkills.value = [...remoteSkills.value, ...newSkills]
        currentPage.value = response.data.page || nextPage
        totalPages.value = response.data.total_pages || totalPages.value
        totalSkills.value = response.data.total || totalSkills.value
        return newSkills.length > 0
      }
      return false
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to load more skills'
      return false
    } finally {
      loadingMore.value = false
    }
  }

  function hasMoreSkills() {
    return currentPage.value < totalPages.value
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
      const skill = skills.value.find((s) => s.id === id)
      if (skill?.builtin) {
        const message = te('extensions.browse.builtinSkillUninstallBlocked')
          ? String(t('extensions.browse.builtinSkillUninstallBlocked'))
          : 'Built-in skills can be disabled, but they cannot be uninstalled'
        error.value = message
        return { success: false, message }
      }
      const response = await skillApi.uninstall(id)
      if (response.data.success) {
        // Remove from local skills
        skills.value = skills.value.filter((s) => s.id !== id)
        // Mark as not installed in remote skills
        const remoteSkill = remoteSkills.value.find((s) => s.id === id)
        if (remoteSkill) {
          remoteSkill.installed = false
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

  async function installFromURL(req: InstallFromURLRequest) {
    try {
      loading.value = true
      error.value = null
      const response = await skillApi.installFromURL(req)
      if (response.data.success) {
        // Refresh local skills to show the newly installed skill
        await fetchSkills()
      }
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to install skill from URL'
      return { success: false }
    } finally {
      loading.value = false
    }
  }

  async function fetchFeaturedSkills() {
    try {
      loading.value = true
      error.value = null
      const response = await skillApi.featured()
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch featured skills'
      return []
    } finally {
      loading.value = false
    }
  }

  async function fetchLocalSkills() {
    try {
      loading.value = true
      error.value = null
      const response = await skillApi.listLocal()
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch local skills'
      return { skills: [], count: 0 }
    } finally {
      loading.value = false
    }
  }

  async function scanLocalSkills() {
    try {
      loading.value = true
      error.value = null
      const response = await skillApi.scanLocal()
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to scan local skills'
      return { success: false, skills_found: 0 }
    } finally {
      loading.value = false
    }
  }

  function clearError() {
    error.value = null
  }

  async function uploadSkill(file: File) {
    try {
      loading.value = true
      error.value = null
      const response = await skillApi.upload(file)
      if (response.data.success) {
        // Refresh local skills to show the newly uploaded skill
        await fetchSkills()
      } else if (response.data.message) {
        throw new Error(response.data.message)
      }
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to upload skill'
      throw e
    } finally {
      loading.value = false
    }
  }

  return {
    // State
    skills,
    sources,
    remoteSkills,
    loading,
    refreshing,
    error,

    // Pagination state
    currentPage,
    totalPages,
    totalSkills,
    pageSize,
    loadingMore,

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
    loadMoreSkills,
    hasMoreSkills,
    installSkill,
    installFromURL,
    uninstallSkill,
    refreshSources,
    fetchFeaturedSkills,
    fetchLocalSkills,
    scanLocalSkills,
    uploadSkill,
    clearError,
  }
})
