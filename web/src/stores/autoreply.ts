import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import {
  autoReplyApi,
  type AutoReplyRule,
  type CreateRuleRequest,
  type UpdateRuleRequest,
  type TestRuleResponse,
} from '@/api/autoreply'

export const useAutoReplyStore = defineStore('autoreply', () => {
  // State
  const rules = ref<AutoReplyRule[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  // Getters
  const enabledRules = computed(() => rules.value.filter((r) => r.enabled))
  const disabledRules = computed(() => rules.value.filter((r) => !r.enabled))
  const sortedRules = computed(() =>
    [...rules.value].sort((a, b) => b.priority - a.priority)
  )

  const rulesByTriggerType = computed(() => ({
    keyword: rules.value.filter((r) => r.trigger_type === 'keyword'),
    regex: rules.value.filter((r) => r.trigger_type === 'regex'),
    contains: rules.value.filter((r) => r.trigger_type === 'contains'),
    prefix: rules.value.filter((r) => r.trigger_type === 'prefix'),
    suffix: rules.value.filter((r) => r.trigger_type === 'suffix'),
  }))

  // Actions
  async function fetchRules(): Promise<void> {
    loading.value = true
    error.value = null
    try {
      const response = await autoReplyApi.list()
      rules.value = response.data
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to fetch rules'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function getRule(id: string): Promise<AutoReplyRule | null> {
    try {
      const response = await autoReplyApi.get(id)
      return response.data
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to get rule'
      return null
    }
  }

  async function createRule(data: CreateRuleRequest): Promise<AutoReplyRule | null> {
    loading.value = true
    error.value = null
    try {
      const response = await autoReplyApi.create(data)
      rules.value.push(response.data)
      return response.data
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to create rule'
      return null
    } finally {
      loading.value = false
    }
  }

  async function updateRule(id: string, data: UpdateRuleRequest): Promise<boolean> {
    loading.value = true
    error.value = null
    try {
      const response = await autoReplyApi.update(id, data)
      const index = rules.value.findIndex((r) => r.id === id)
      if (index !== -1) {
        rules.value[index] = response.data
      }
      return true
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to update rule'
      return false
    } finally {
      loading.value = false
    }
  }

  async function deleteRule(id: string): Promise<boolean> {
    loading.value = true
    error.value = null
    try {
      await autoReplyApi.delete(id)
      rules.value = rules.value.filter((r) => r.id !== id)
      return true
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to delete rule'
      return false
    } finally {
      loading.value = false
    }
  }

  async function enableRule(id: string): Promise<boolean> {
    try {
      await autoReplyApi.enable(id)
      const rule = rules.value.find((r) => r.id === id)
      if (rule) {
        rule.enabled = true
      }
      return true
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to enable rule'
      return false
    }
  }

  async function disableRule(id: string): Promise<boolean> {
    try {
      await autoReplyApi.disable(id)
      const rule = rules.value.find((r) => r.id === id)
      if (rule) {
        rule.enabled = false
      }
      return true
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to disable rule'
      return false
    }
  }

  async function setRuleChannels(id: string, channels: string[]): Promise<boolean> {
    try {
      await autoReplyApi.setChannels(id, channels)
      const rule = rules.value.find((r) => r.id === id)
      if (rule) {
        rule.channels = channels
      }
      return true
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to set channels'
      return false
    }
  }

  async function testMessage(
    message: string,
    channel?: string
  ): Promise<TestRuleResponse | null> {
    try {
      const response = await autoReplyApi.test({ message, channel })
      return response.data
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to test message'
      return null
    }
  }

  function clearError(): void {
    error.value = null
  }

  return {
    // State
    rules,
    loading,
    error,

    // Getters
    enabledRules,
    disabledRules,
    sortedRules,
    rulesByTriggerType,

    // Actions
    fetchRules,
    getRule,
    createRule,
    updateRule,
    deleteRule,
    enableRule,
    disableRule,
    setRuleChannels,
    testMessage,
    clearError,
  }
})
