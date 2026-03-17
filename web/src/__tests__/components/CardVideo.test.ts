import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import CardVideo from '@/components/typeless/CardVideo.vue'

const { authFetchMock } = vi.hoisted(() => ({
  authFetchMock: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  authFetch: authFetchMock,
}))

describe('CardVideo', () => {
  function mockBlobResponse(body: string, type: string, ok = true): Response {
    return {
      ok,
      blob: vi.fn().mockResolvedValue(new Blob([body], { type })),
    } as unknown as Response
  }

  beforeEach(() => {
    authFetchMock.mockReset()
    authFetchMock
      .mockResolvedValueOnce(mockBlobResponse('video-data', 'video/mp4'))
      .mockResolvedValueOnce(mockBlobResponse('subtitle-data', 'text/vtt'))
    ;(URL as unknown as { createObjectURL: (blob: Blob) => string }).createObjectURL = vi
      .fn()
      .mockReturnValueOnce('blob:video-card')
      .mockReturnValueOnce('blob:subtitle-card')
    ;(URL as unknown as { revokeObjectURL: (url: string) => void }).revokeObjectURL = vi.fn()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('resolves protected video and subtitle URLs through auth fetch', async () => {
    const wrapper = mount(CardVideo, {
      props: {
        card: {
          type: 'video',
          src: '/api/v1/media/protected/video.mp4',
          subtitles: [
            {
              src: '/api/v1/media/protected/captions.vtt',
              label: 'English',
              srclang: 'en',
              default: true,
            },
          ],
        },
      },
    })

    await flushPromises()
    await flushPromises()

    expect(authFetchMock).toHaveBeenNthCalledWith(1, '/api/v1/media/protected/video.mp4')
    expect(authFetchMock).toHaveBeenNthCalledWith(2, '/api/v1/media/protected/captions.vtt')
    expect(wrapper.get('video').attributes('src')).toBe('blob:video-card')
    expect(wrapper.get('track').attributes('src')).toBe('blob:subtitle-card')
  })
})
