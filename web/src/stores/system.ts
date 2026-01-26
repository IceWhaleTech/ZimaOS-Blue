import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { HealthStatus, WorkerStats } from '@/api'
import { healthApi, workerApi } from '@/api'

export const useSystemStore = defineStore('system', () => {
  const health = ref<HealthStatus | null>(null)
  const workerStats = ref<WorkerStats | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function fetchHealth() {
    try {
      loading.value = true
      error.value = null
      const response = await healthApi.getHealth()
      health.value = response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch health'
    } finally {
      loading.value = false
    }
  }

  async function fetchWorkerStats() {
    try {
      const response = await workerApi.getStats()
      workerStats.value = response.data
    } catch (e) {
      console.error('Failed to fetch worker stats:', e)
    }
  }

  async function fetchAll() {
    await Promise.all([fetchHealth(), fetchWorkerStats()])
  }

  return {
    health,
    workerStats,
    loading,
    error,
    fetchHealth,
    fetchWorkerStats,
    fetchAll,
  }
})
