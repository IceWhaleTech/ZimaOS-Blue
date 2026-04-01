import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import ConfigurableDashboard from '@/components/dashboard/ConfigurableDashboard.vue'
import { useDashboardStore } from '@/stores/dashboard'
import { i18n } from '@/i18n'

const systemStore = reactive({
  loading: false,
  health: {
    status: 'ok',
    version: '0.10.36',
    num_cpu: 8,
    timestamp: '2026-03-14T00:00:25.000Z',
    uptime: '4m36s',
    go_version: 'go1.24.2',
    goroutines: 57,
    mem_alloc_bytes: 9.91 * 1024 * 1024,
  },
})

vi.mock('@/stores/system', () => ({
  useSystemStore: () => systemStore,
}))

vi.mock('@/components/dashboard/DashboardCustomizer.vue', () => ({
  default: { name: 'DashboardCustomizer', template: '<div />' },
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

describe('ConfigurableDashboard hero sparklines', () => {
  beforeEach(() => {
    localStorageMock.clear()
    setActivePinia(createPinia())
    const dashboardStore = useDashboardStore()
    dashboardStore.cardStates = dashboardStore.cardStates.map((card) => ({
      ...card,
      enabled: ['system-status', 'uptime', 'memory-usage', 'goroutines'].includes(card.id),
    }))
  })

  it('renders live sparkline paths for hero cards with metrics history', async () => {
    const wrapper = mount(ConfigurableDashboard, {
      props: {
        showHeader: false,
        metricsHistory: [
          {
            timestamp: '2026-03-14T00:00:05.000Z',
            cpu_percent: 18,
            memory_used_bytes: 128 * 1024 * 1024,
            goroutines: 43,
            heap_alloc_bytes: 8.6 * 1024 * 1024,
          },
          {
            timestamp: '2026-03-14T00:00:10.000Z',
            cpu_percent: 24,
            memory_used_bytes: 132 * 1024 * 1024,
            goroutines: 49,
            heap_alloc_bytes: 8.9 * 1024 * 1024,
          },
          {
            timestamp: '2026-03-14T00:00:15.000Z',
            cpu_percent: 22,
            memory_used_bytes: 130 * 1024 * 1024,
            goroutines: 46,
            heap_alloc_bytes: 9.1 * 1024 * 1024,
          },
          {
            timestamp: '2026-03-14T00:00:20.000Z',
            cpu_percent: 27,
            memory_used_bytes: 134 * 1024 * 1024,
            goroutines: 54,
            heap_alloc_bytes: 9.5 * 1024 * 1024,
          },
          {
            timestamp: '2026-03-14T00:00:25.000Z',
            cpu_percent: 21,
            memory_used_bytes: 131 * 1024 * 1024,
            goroutines: 57,
            heap_alloc_bytes: 9.9 * 1024 * 1024,
          },
        ],
      },
      global: {
        plugins: [i18n],
      },
    })

    await flushPromises()

    const sparklineLines = wrapper.findAll('.dashboard-mini-sparkline-line')

    expect(sparklineLines).toHaveLength(3)
    expect(sparklineLines.every((line) => (line.attributes('d') || '').includes('L'))).toBe(true)
  })
})
