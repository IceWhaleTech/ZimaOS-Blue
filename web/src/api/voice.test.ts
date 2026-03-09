import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('./client', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
  },
  ensureFreshToken: vi.fn(),
}))

import api from './client'
import { streamingTTSManager } from './voice'

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
