import { beforeEach, describe, expect, it, vi } from 'vitest'

import { isTtsAutoPlayEnabled, setTtsAutoPlayEnabled } from '@/utils/ttsPreferences'

describe('ttsPreferences', () => {
  beforeEach(() => {
    const store = new Map<string, string>()
    Object.defineProperty(window, 'localStorage', {
      value: {
        getItem: vi.fn((key: string) => store.get(key) ?? null),
        setItem: vi.fn((key: string, value: string) => {
          store.set(key, value)
        }),
      },
      configurable: true,
    })
  })

  it('defaults auto-play to false when unset', () => {
    expect(isTtsAutoPlayEnabled()).toBe(false)
  })

  it('persists enabled state', () => {
    setTtsAutoPlayEnabled(true)

    expect(isTtsAutoPlayEnabled()).toBe(true)
  })

  it('persists disabled state', () => {
    setTtsAutoPlayEnabled(false)

    expect(isTtsAutoPlayEnabled()).toBe(false)
  })
})
