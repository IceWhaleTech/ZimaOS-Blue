import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useSystemStore } from '@/stores/system'
import api from '@/api/client'

vi.mock('@/api/client', () => ({
  default: {
    get: vi.fn(),
  },
}))

describe('useSystemStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('should initialize with null values', () => {
    const store = useSystemStore()
    expect(store.health).toBeNull()
    expect(store.workerStats).toBeNull()
    expect(store.loading).toBe(false)
    expect(store.error).toBeNull()
  })

  it('should fetch health successfully', async () => {
    const mockHealth = {
      status: 'ok',
      timestamp: '2024-01-01T00:00:00Z',
      uptime: '1h0m0s',
      version: '0.1.0',
      go_version: 'go1.21',
      num_cpu: 4,
      goroutines: 10,
      mem_alloc_bytes: 1024000,
    }

    vi.mocked(api.get).mockResolvedValueOnce({ data: mockHealth })

    const store = useSystemStore()
    await store.fetchHealth()

    expect(store.health).toEqual(mockHealth)
    expect(store.loading).toBe(false)
    expect(store.error).toBeNull()
  })

  it('should handle fetch health error', async () => {
    vi.mocked(api.get).mockRejectedValueOnce(new Error('Network error'))

    const store = useSystemStore()
    await store.fetchHealth()

    expect(store.health).toBeNull()
    expect(store.loading).toBe(false)
    expect(store.error).toBe('Network error')
  })

  it('should fetch worker stats successfully', async () => {
    const mockStats = {
      pool_size: 10,
      running: 2,
      total: 100,
    }

    vi.mocked(api.get).mockResolvedValueOnce({ data: mockStats })

    const store = useSystemStore()
    await store.fetchWorkerStats()

    expect(store.workerStats).toEqual(mockStats)
  })

  it('should fetch all data', async () => {
    const mockHealth = {
      status: 'ok',
      timestamp: '2024-01-01T00:00:00Z',
      uptime: '1h0m0s',
      version: '0.1.0',
      go_version: 'go1.21',
      num_cpu: 4,
      goroutines: 10,
      mem_alloc_bytes: 1024000,
    }
    const mockStats = {
      pool_size: 10,
      running: 2,
      total: 100,
    }

    vi.mocked(api.get)
      .mockResolvedValueOnce({ data: mockHealth })
      .mockResolvedValueOnce({ data: mockStats })

    const store = useSystemStore()
    await store.fetchAll()

    expect(store.health).toEqual(mockHealth)
    expect(store.workerStats).toEqual(mockStats)
  })
})
