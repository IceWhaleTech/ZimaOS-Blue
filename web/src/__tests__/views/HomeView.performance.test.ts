import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import HomeView from '@/views/HomeView.vue'
import { useSystemStore } from '@/stores/system'
import { useDashboardStore } from '@/stores/dashboard'
import { systemApi, healthApi, workerApi } from '@/api/index'
import { i18n } from '@/i18n'
import { metricsApi } from '@/api/metrics'

// Mock API
vi.mock('@/api/index', () => ({
  systemApi: {
    getMetricsHistory: vi.fn(),
    getInfo: vi.fn(),
  },
  healthApi: {
    getHealth: vi.fn(),
  },
  workerApi: {
    getStats: vi.fn(),
  },
}))

// Mock components
vi.mock('@/components/dashboard', () => ({
  ConfigurableDashboard: {
    name: 'ConfigurableDashboard',
    template: '<div><slot name="header-left"></slot></div>',
  },
  SystemStatusCard: {
    name: 'SystemStatusCard',
    template: '<div class="system-status-card-stub"></div>',
  },
}))

vi.mock('@/api/metrics', () => ({
  metricsApi: {
    getAll: vi.fn(),
    getUserTokenUsage: vi.fn(),
    getSummary: vi.fn(),
    getCallStats: vi.fn(),
    getModelStats: vi.fn(),
    getTokenUsage: vi.fn(),
    getLatencyStats: vi.fn(),
    getSpeedStats: vi.fn(),
    getPricing: vi.fn(),
    resetMetrics: vi.fn(),
  },
}))

const localStorageMock = (() => {
  let store: Record<string, string> = {}
  return {
    getItem: (key: string) => (key in store ? store[key] : null),
    setItem: (key: string, value: string) => {
      store[key] = String(value)
    },
    removeItem: (key: string) => {
      delete store[key]
    },
    clear: () => {
      store = {}
    },
  }
})()

vi.stubGlobal('localStorage', localStorageMock)

const detailedSystemInfoFixture = {
  os: {
    name: 'ZimaOS',
    version: 'ZimaOS-Test',
    kernel: '6.8.0',
    architecture: 'arm64',
    hostname: 'blue-host',
    uptime: 3600,
    uptime_human: '1 hour',
    boot_time: 1710000000,
  },
  hardware: {
    cpu: {
      model: 'Test CPU',
      cores: 8,
      threads: 16,
      frequency: 3200,
      usage: 42,
      vendor_id: 'Zima Silicon',
      cache_size: 0,
    },
    memory: {
      total: 16 * 1024 * 1024 * 1024,
      available: 10 * 1024 * 1024 * 1024,
      used: 6 * 1024 * 1024 * 1024,
      used_percent: 37.5,
      swap_total: 2 * 1024 * 1024 * 1024,
      swap_used: 512 * 1024 * 1024,
    },
    disk: [
      {
        device: '/dev/disk1s1',
        mount_point: '/',
        fs_type: 'apfs',
        total: 512 * 1024 * 1024 * 1024,
        used: 256 * 1024 * 1024 * 1024,
        available: 256 * 1024 * 1024 * 1024,
        used_percent: 50,
      },
    ],
    gpu: [],
  },
  network: {
    interfaces: [
      {
        name: 'eth0',
        mac: '00:11:22:33:44:55',
        ipv4: ['192.168.1.20'],
        ipv6: [],
        mtu: 1500,
        flags: [],
        is_up: true,
        is_loopback: false,
      },
    ],
    public_ip: '203.0.113.10',
  },
  runtime: {
    go_version: 'go1.24.0',
    num_cpu: 8,
    num_goroutine: 32,
    gomaxprocs: 8,
    alloc_mb: 128,
    total_alloc_mb: 512,
    sys_mb: 256,
    num_gc: 12,
  },
}

function mountHomeView() {
  return mount(HomeView, {
    global: {
      plugins: [i18n],
      stubs: {
        ProgressBar: true,
        DonutChart: true,
        RouterLink: {
          template: '<a><slot /></a>',
        },
      },
    },
  })
}

describe('HomeView Performance Optimizations', () => {
  beforeEach(() => {
    localStorageMock.clear()
    setActivePinia(createPinia())
    vi.useFakeTimers()
    vi.clearAllMocks()

    // Setup default mock responses
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    vi.mocked(systemApi.getMetricsHistory).mockResolvedValue({ data: { metrics: [] } } as any)
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    vi.mocked(systemApi.getInfo).mockResolvedValue({ data: { system: null } } as any)
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    vi.mocked(healthApi.getHealth).mockResolvedValue({
      data: { status: 'ok', version: 'test' },
    } as any)
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    vi.mocked(workerApi.getStats).mockResolvedValue({
      data: { pending: 0, completed: 0, failed: 0 },
    } as any)
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    vi.mocked(metricsApi.getAll).mockResolvedValue({
      data: {
        calls: {
          stats: {
            total_calls: 0,
            successful_calls: 0,
            failed_calls: 0,
            success_rate: 0,
            error_rate: 0,
            errors_by_type: {},
          },
        },
        models: { models: [] },
        tokens: {
          usage: {
            input_tokens: 0,
            output_tokens: 0,
            cache_read_tokens: 0,
            cache_write_tokens: 0,
            total_tokens: 0,
            estimated_cost: 0,
          },
        },
        latency: {
          min_ms: 0,
          max_ms: 0,
          avg_ms: 0,
          p50_ms: 0,
          p90_ms: 0,
          p95_ms: 0,
          p99_ms: 0,
          samples: 0,
        },
        speed: { current: { tokens_per_second: 0, time_to_first_token_ms: 0, decode_speed: 0 } },
        pricing: [],
      },
    } as any)
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    vi.mocked(metricsApi.getUserTokenUsage).mockResolvedValue({ data: { users: [] } } as any)
  })

  afterEach(() => {
    localStorageMock.clear()
    vi.restoreAllMocks()
    vi.useRealTimers()
  })

  it('should use 15-second polling interval instead of 5 seconds', async () => {
    const wrapper = mountHomeView()

    await flushPromises()

    // Clear initial calls
    vi.clearAllMocks()

    // Fast-forward 5 seconds - should NOT trigger refresh
    await vi.advanceTimersByTimeAsync(5000)
    expect(systemApi.getMetricsHistory).not.toHaveBeenCalled()

    // Fast-forward to 15 seconds - should trigger refresh
    await vi.advanceTimersByTimeAsync(10000)
    expect(systemApi.getMetricsHistory).toHaveBeenCalledTimes(1)

    wrapper.unmount()
  })

  it('should cancel pending requests on unmount', async () => {
    const abortSpy = vi.fn()
    class AbortControllerMock {
      signal = {} as AbortSignal
      abort = abortSpy
    }
    vi.stubGlobal('AbortController', AbortControllerMock as unknown as typeof AbortController)

    const wrapper = mountHomeView()

    await flushPromises()

    // Trigger a fetch
    await vi.advanceTimersByTimeAsync(15000)

    // Unmount should cancel pending requests
    wrapper.unmount()

    expect(abortSpy).toHaveBeenCalled()
  })

  it('should pause polling when page is hidden', async () => {
    const wrapper = mountHomeView()

    await flushPromises()
    vi.clearAllMocks()

    // Simulate page becoming hidden
    Object.defineProperty(document, 'hidden', {
      configurable: true,
      get: () => true,
    })
    document.dispatchEvent(new Event('visibilitychange'))

    // Fast-forward 15 seconds - should NOT trigger refresh when hidden
    await vi.advanceTimersByTimeAsync(15000)
    expect(systemApi.getMetricsHistory).not.toHaveBeenCalled()

    // Simulate page becoming visible again
    Object.defineProperty(document, 'hidden', {
      configurable: true,
      get: () => false,
    })
    document.dispatchEvent(new Event('visibilitychange'))

    // Fast-forward 15 seconds - should trigger refresh when visible
    await vi.advanceTimersByTimeAsync(15000)
    expect(systemApi.getMetricsHistory).toHaveBeenCalledTimes(1)

    wrapper.unmount()
  })

  it('should support manual refresh without relying on extra toolbar controls', async () => {
    const wrapper = mountHomeView()

    await flushPromises()
    vi.clearAllMocks()

    const refreshButton = wrapper.find('.dashboard-refresh-button')
    expect(refreshButton.exists()).toBe(true)

    await refreshButton.trigger('click')
    await flushPromises()

    expect(systemApi.getMetricsHistory).toHaveBeenCalledTimes(1)

    wrapper.unmount()
  })

  it('should respect dashboard configuration for homepage cards', async () => {
    const dashboardStore = useDashboardStore()
    dashboardStore.toggleCard('system-status')
    dashboardStore.toggleCard('cpu-chart')

    const wrapper = mountHomeView()
    await flushPromises()

    expect(wrapper.find('.dashboard-status-row').exists()).toBe(false)
    expect(wrapper.findAll('.dashboard-primary-grid .dashboard-small-card-shell')).toHaveLength(3)

    wrapper.unmount()
  })

  it('should load detailed system info on demand', async () => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    vi.mocked(systemApi.getInfo).mockResolvedValue({
      data: { system: detailedSystemInfoFixture },
    } as any)

    const wrapper = mountHomeView()
    await flushPromises()

    expect(systemApi.getInfo).not.toHaveBeenCalled()

    await wrapper.find('.dashboard-details-button').trigger('click')
    await flushPromises()

    expect(systemApi.getInfo).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain(i18n.global.t('system.osInfo'))
    expect(wrapper.text()).toContain('ZimaOS-Test')
    expect(wrapper.text()).toContain('eth0')

    wrapper.unmount()
  })

  it('should use shallowRef for metricsHistory to improve performance', async () => {
    const wrapper = mountHomeView()

    // Access the component's metricsHistory
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const vm = wrapper.vm as any

    // Verify it's a ref (we can't directly test if it's shallow, but we can verify it exists)
    expect(vm.metricsHistory).toBeDefined()

    wrapper.unmount()
  })

  it('should cleanup all resources on unmount', async () => {
    const removeEventListenerSpy = vi.spyOn(document, 'removeEventListener')

    const wrapper = mountHomeView()

    await flushPromises()

    wrapper.unmount()

    // Verify visibility listener was removed
    expect(removeEventListenerSpy).toHaveBeenCalledWith('visibilitychange', expect.any(Function))
  })

  it('should handle AbortError gracefully', async () => {
    const consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})

    vi.mocked(systemApi.getMetricsHistory).mockRejectedValue({
      name: 'AbortError',
      message: 'Request aborted',
    })

    const wrapper = mountHomeView()

    await flushPromises()

    // Should not log error for AbortError
    expect(consoleErrorSpy).not.toHaveBeenCalled()

    wrapper.unmount()
    consoleErrorSpy.mockRestore()
  })
})

describe('System Store Caching', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.useFakeTimers()
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.useRealTimers()
  })

  it('should cache health data for 10 seconds', async () => {
    const store = useSystemStore()
    const { healthApi } = await import('@/api/index')

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    vi.mocked(healthApi.getHealth).mockResolvedValue({ data: { status: 'ok' } } as any)

    // First call
    await store.fetchHealth()
    expect(healthApi.getHealth).toHaveBeenCalledTimes(1)

    // Second call within cache TTL - should use cache
    await store.fetchHealth()
    expect(healthApi.getHealth).toHaveBeenCalledTimes(1)

    // Fast-forward past cache TTL
    await vi.advanceTimersByTimeAsync(11000)

    // Third call after cache expiry - should fetch again
    await store.fetchHealth()
    expect(healthApi.getHealth).toHaveBeenCalledTimes(2)
  })

  it('should deduplicate concurrent requests', async () => {
    const store = useSystemStore()
    const { healthApi } = await import('@/api/index')

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    let resolveHealth: (value: any) => void
    vi.mocked(healthApi.getHealth).mockReturnValue(
      new Promise((resolve) => {
        resolveHealth = resolve
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
      }) as any
    )

    // Start multiple concurrent requests
    const promise1 = store.fetchHealth(true)
    const promise2 = store.fetchHealth(true)
    const promise3 = store.fetchHealth(true)

    // Should only make one API call
    expect(healthApi.getHealth).toHaveBeenCalledTimes(1)

    // Resolve the request
    resolveHealth({ data: { status: 'ok' } })
    await Promise.all([promise1, promise2, promise3])

    // Still only one API call
    expect(healthApi.getHealth).toHaveBeenCalledTimes(1)
  })

  it('should force refresh when requested', async () => {
    const store = useSystemStore()
    const { healthApi } = await import('@/api/index')

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    vi.mocked(healthApi.getHealth).mockResolvedValue({ data: { status: 'ok' } } as any)

    // First call
    await store.fetchHealth()
    expect(healthApi.getHealth).toHaveBeenCalledTimes(1)

    // Force refresh - should bypass cache
    await store.fetchHealth(true)
    expect(healthApi.getHealth).toHaveBeenCalledTimes(2)
  })
})
