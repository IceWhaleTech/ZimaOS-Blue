import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import CardAudio from '@/components/typeless/CardAudio.vue'

const { authFetchMock } = vi.hoisted(() => ({
  authFetchMock: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  authFetch: authFetchMock,
}))

describe('CardAudio', () => {
  function mockBlobResponse(body: string, type: string, ok = true): Response {
    return {
      ok,
      blob: vi.fn().mockResolvedValue(new Blob([body], { type })),
    } as unknown as Response
  }

  beforeEach(() => {
    authFetchMock.mockReset()
    authFetchMock.mockResolvedValue(mockBlobResponse('audio-data', 'audio/wav'))
    ;(URL as unknown as { createObjectURL: (blob: Blob) => string }).createObjectURL = vi
      .fn()
      .mockReturnValue('blob:audio-card')
    ;(URL as unknown as { revokeObjectURL: (url: string) => void }).revokeObjectURL = vi.fn()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('resolves protected audio URLs through auth fetch', async () => {
    const wrapper = mount(CardAudio, {
      props: {
        card: {
          type: 'audio',
          title: 'Speech',
          src: '/api/v1/convert/tasks/task-1/download/out-1',
        },
      },
    })

    await flushPromises()

    expect(authFetchMock).toHaveBeenCalledWith('/api/v1/convert/tasks/task-1/download/out-1')
    expect(wrapper.get('audio').attributes('src')).toBe('blob:audio-card')
  })
})
