import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('./client', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
  },
  ensureFreshToken: vi.fn(),
}))

import api from './client'
import { streamingTTSManager, ttsAudioManager } from './voice'

describe('streamingTTSManager.stop', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    ;(api as any).post.mockResolvedValue({})
    const manager = streamingTTSManager as any
    manager.isStopped = false
    manager.isPlaying = false
    manager.playbackStarted = false
    manager.queue = []
    manager.playIndex = 0
    manager.fetchIndex = 0
    manager.activeFetches = 0
    manager.currentEventSource = null
  })

  it('does not call stop endpoint when no synthesis request is active', () => {
    streamingTTSManager.stop()

    expect((api as any).post).not.toHaveBeenCalled()
  })

  it('calls stop endpoint when synthesis fetching is active', () => {
    const manager = streamingTTSManager as any
    manager.activeFetches = 1

    streamingTTSManager.stop()

    expect((api as any).post).toHaveBeenCalledWith('/voice/synthesize/stop')
  })
})

describe('streamingTTSManager streaming chunks', () => {
  type Listener = (event: MessageEvent<string>) => void
  class MockEventSource {
    public close = vi.fn()
    private listeners = new Map<string, Listener[]>()

    constructor(public readonly url: string) {
      instances.push(this)
    }

    addEventListener(type: string, listener: Listener) {
      const current = this.listeners.get(type) || []
      current.push(listener)
      this.listeners.set(type, current)
    }

    emit(type: string, data: unknown) {
      const event = { data: JSON.stringify(data) } as MessageEvent<string>
      for (const listener of this.listeners.get(type) || []) {
        listener(event)
      }
    }
  }

  const instances: MockEventSource[] = []

  beforeEach(() => {
    vi.clearAllMocks()
    instances.length = 0
    vi.stubGlobal('EventSource', MockEventSource as unknown as typeof EventSource)
    vi.stubGlobal('localStorage', {
      getItem: vi.fn(() => null),
      setItem: vi.fn(),
      removeItem: vi.fn(),
      clear: vi.fn(),
      key: vi.fn(),
      length: 0,
    })

    const manager = streamingTTSManager as any
    manager.generation = 0
    manager.isStopped = true
    manager.isPlaying = false
    manager.playbackStarted = false
    manager.queue = []
    manager.playIndex = 0
    manager.fetchIndex = 0
    manager.activeFetches = 0
    manager.currentEventSource = null
    manager.onComplete = null
    manager.onSentenceStart = null
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    streamingTTSManager.stop()
  })

  it('waits for all SSE audio events and plays every chunk', async () => {
    const playSpy = vi.spyOn(ttsAudioManager, 'play').mockResolvedValue()

    await streamingTTSManager.streamAndPlay('你好，世界。')

    expect(instances).toHaveLength(1)
    const source = instances[0]!

    source.emit('audio', { audio: 'chunk-1', content_type: 'audio/wav' })
    expect(source.close).not.toHaveBeenCalled()
    expect(playSpy).not.toHaveBeenCalled()

    source.emit('audio', { audio: 'chunk-2', content_type: 'audio/wav' })
    source.emit('done', {})

    await Promise.resolve()
    await Promise.resolve()

    expect(playSpy).toHaveBeenCalledTimes(2)
    expect(playSpy).toHaveBeenNthCalledWith(1, 'chunk-1', 'audio/wav')
    expect(playSpy).toHaveBeenNthCalledWith(2, 'chunk-2', 'audio/wav')
    expect(source.close).toHaveBeenCalledTimes(1)
  })
})
