import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useDashboardStore } from '@/stores/dashboard'

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

describe('dashboard store registry sync', () => {
  beforeEach(() => {
    localStorageMock.clear()
    setActivePinia(createPinia())
  })

  it('recovers default-enabled cards that are missing from persisted state', () => {
    const dashboardStore = useDashboardStore()

    dashboardStore.cardStates = [
      {
        id: 'system-status',
        enabled: true,
        order: 1,
        collapsed: false,
      },
      {
        id: 'uptime',
        enabled: true,
        order: 2,
        collapsed: false,
      },
    ]

    const enabledIds = dashboardStore.enabledCards.map((card) => card.id)

    expect(enabledIds).toContain('cpu-chart')
  })

  it('creates missing card state before toggling a card', () => {
    const dashboardStore = useDashboardStore()

    dashboardStore.cardStates = [
      {
        id: 'system-status',
        enabled: true,
        order: 1,
        collapsed: false,
      },
    ]

    dashboardStore.toggleCard('failover-status')

    const failoverStatus = dashboardStore.cardStates.find((card) => card.id === 'failover-status')

    expect(failoverStatus).toBeTruthy()
    expect(failoverStatus?.enabled).toBe(true)
    expect(dashboardStore.enabledCards.map((card) => card.id)).toContain('failover-status')
  })
})
