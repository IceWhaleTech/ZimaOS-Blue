import { beforeEach, describe, expect, it, vi } from 'vitest'

import {
  getTtsSpeechVolume,
  isTtsAutoPlayEnabled,
  isTtsSpeechMuted,
  setTtsAutoPlayEnabled,
} from '@/utils/ttsPreferences'

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

  it('defaults speech volume to 100 when unset', () => {
    expect(getTtsSpeechVolume()).toBe(100)
    expect(isTtsSpeechMuted()).toBe(false)
  })

  it('treats zero speech volume as muted', () => {
    window.localStorage.setItem('tts-speech-volume', '0')

    expect(getTtsSpeechVolume()).toBe(0)
    expect(isTtsSpeechMuted()).toBe(true)
  })

  it('treats positive speech volume as unmuted', () => {
    window.localStorage.setItem('tts-speech-volume', '15')

    expect(getTtsSpeechVolume()).toBe(15)
    expect(isTtsSpeechMuted()).toBe(false)
  })
})
