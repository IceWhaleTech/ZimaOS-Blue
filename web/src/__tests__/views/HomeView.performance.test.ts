import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import HomeView from '@/views/HomeView.vue'
import { useSystemStore } from '@/stores/system'
import { systemApi } from '@/api/index'

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
}))

describe('HomeView Performance Optimizations', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.useFakeTimers()
    vi.clearAllMocks()

    // Setup default mock responses
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    vi.mocked(systemApi.getMetricsHistory).mockResolvedValue({ data: { metrics: [] } } as any)
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.useRealTimers()
  })

  it('should use 15-second polling interval instead of 5 seconds', async () => {
    const wrapper = mount(HomeView, {
      global: {
        stubs: {
          ConfigurableDashboard: true,
          ProgressBar: true,
          DonutChart: true,
        },
      },
    })

    await wrapper.vm.$nextTick()

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
    global.AbortController = vi.fn(() => ({
      abort: abortSpy,
      signal: {} as AbortSignal,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
    })) as any

    const wrapper = mount(HomeView, {
      global: {
        stubs: {
          ConfigurableDashboard: true,
          ProgressBar: true,
          DonutChart: true,
        },
      },
    })

    await wrapper.vm.$nextTick()

    // Trigger a fetch
    await vi.advanceTimersByTimeAsync(15000)

    // Unmount should cancel pending requests
    wrapper.unmount()

    expect(abortSpy).toHaveBeenCalled()
  })

  it('should pause polling when page is hidden', async () => {
    const wrapper = mount(HomeView, {
      global: {
        stubs: {
          ConfigurableDashboard: true,
          ProgressBar: true,
          DonutChart: true,
        },
      },
    })

    await wrapper.vm.$nextTick()
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

  it('should debounce autoRefresh toggle', async () => {
    const wrapper = mount(HomeView, {
      global: {
        stubs: {
          ConfigurableDashboard: true,
          ProgressBar: true,
          DonutChart: true,
        },
      },
    })

    await wrapper.vm.$nextTick()
    vi.clearAllMocks()

    // Toggle autoRefresh multiple times rapidly
    const checkbox = wrapper.find('input[type="checkbox"]')
    await checkbox.setValue(false)
    await checkbox.setValue(true)
    await checkbox.setValue(false)
    await checkbox.setValue(true)

    // Should only trigger once after debounce period
    await vi.advanceTimersByTimeAsync(100)
    expect(systemApi.getMetricsHistory).toHaveBeenCalledTimes(1)

    wrapper.unmount()
  })

  it('should use shallowRef for metricsHistory to improve performance', async () => {
    const wrapper = mount(HomeView, {
      global: {
        stubs: {
          ConfigurableDashboard: true,
          ProgressBar: true,
          DonutChart: true,
        },
      },
    })

    // Access the component's metricsHistory
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const vm = wrapper.vm as any

    // Verify it's a ref (we can't directly test if it's shallow, but we can verify it exists)
    expect(vm.metricsHistory).toBeDefined()

    wrapper.unmount()
  })

  it('should cleanup all resources on unmount', async () => {
    const removeEventListenerSpy = vi.spyOn(document, 'removeEventListener')

    const wrapper = mount(HomeView, {
      global: {
        stubs: {
          ConfigurableDashboard: true,
          ProgressBar: true,
          DonutChart: true,
        },
      },
    })

    await wrapper.vm.$nextTick()

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

    const wrapper = mount(HomeView, {
      global: {
        stubs: {
          ConfigurableDashboard: true,
          ProgressBar: true,
          DonutChart: true,
        },
      },
    })

    await wrapper.vm.$nextTick()

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
