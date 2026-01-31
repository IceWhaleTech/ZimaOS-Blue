import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { HealthStatus, WorkerStats } from '@/api'
import { healthApi, workerApi } from '@/api'

// Cache configuration
const CACHE_TTL = 10000 // 10 seconds cache TTL

export const useSystemStore = defineStore('system', () => {
  const health = ref<HealthStatus | null>(null)
  const workerStats = ref<WorkerStats | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)

  // Cache timestamps
  let healthCacheTime = 0
  let workerStatsCacheTime = 0

  // Pending request tracking to prevent duplicate requests
  let pendingHealthRequest: Promise<void> | null = null
  let pendingWorkerStatsRequest: Promise<void> | null = null

  async function fetchHealth(forceRefresh = false) {
    // Return cached data if still valid
    const now = Date.now()
    if (!forceRefresh && health.value && now - healthCacheTime < CACHE_TTL) {
      return
    }

    // Return existing pending request if one is in flight
    if (pendingHealthRequest) {
      return pendingHealthRequest
    }

    try {
      loading.value = true
      error.value = null
      pendingHealthRequest = (async () => {
        const response = await healthApi.getHealth()
        health.value = response.data
        healthCacheTime = Date.now()
      })()
      await pendingHealthRequest
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch health'
    } finally {
      loading.value = false
      pendingHealthRequest = null
    }
  }

  async function fetchWorkerStats(forceRefresh = false) {
    // Return cached data if still valid
    const now = Date.now()
    if (!forceRefresh && workerStats.value && now - workerStatsCacheTime < CACHE_TTL) {
      return
    }

    // Return existing pending request if one is in flight
    if (pendingWorkerStatsRequest) {
      return pendingWorkerStatsRequest
    }

    try {
      pendingWorkerStatsRequest = (async () => {
        const response = await workerApi.getStats()
        workerStats.value = response.data
        workerStatsCacheTime = Date.now()
      })()
      await pendingWorkerStatsRequest
    } catch (e) {
      console.error('Failed to fetch worker stats:', e)
    } finally {
      pendingWorkerStatsRequest = null
    }
  }

  async function fetchAll(forceRefresh = false) {
    await Promise.all([fetchHealth(forceRefresh), fetchWorkerStats(forceRefresh)])
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
