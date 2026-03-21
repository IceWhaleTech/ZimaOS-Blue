import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

import CardModelDownloadProgress from '@/components/typeless/CardModelDownloadProgress.vue'

const { authFetchMock } = vi.hoisted(() => ({
  authFetchMock: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  authFetch: authFetchMock,
}))

function createTestI18n() {
  return createI18n({
    legacy: false,
    locale: 'en-US',
    fallbackLocale: 'en-US',
    messages: {
      'en-US': {
        media: {
          modelDownload: {
            status: {
              downloading: 'Downloading model',
              ready: 'Model ready',
              error: 'Download failed',
              pending: 'Preparing download',
            },
            auto: {
              fileName: 'Processing files',
            },
            completed: 'Downloaded',
            pending: 'Pending',
          },
        },
      },
    },
  })
}

function makeCard() {
  return {
    type: 'model-download-progress' as const,
    id: 'model-download-u2netp',
    model_id: 'u2netp',
    title: 'Downloading lightweight cutout model',
    message: 'Preparing subject cutout for future renders',
    status: 'not_downloaded' as const,
    downloading: false,
    ready: false,
    state: 'not_downloaded',
    status_url: '/api/v1/media/fallback/models/u2netp/status',
    poll_interval_ms: 1000,
  }
}

describe('CardModelDownloadProgress', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    authFetchMock.mockReset()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('polls the status endpoint and renders download progress', async () => {
    authFetchMock.mockResolvedValue({
      ok: true,
      json: async () => ({
        model_id: 'u2netp',
        status: 'downloading',
        downloading: true,
        ready: false,
        state: 'downloading',
        message: 'Preparing subject cutout for future renders',
        progress: {
          file: 'u2netp.onnx',
          file_index: 1,
          total_files: 2,
          downloaded: 50,
          total: 100,
          percentage: 50,
        },
        files: [{ filename: 'u2netp.onnx', downloaded: false, size: '~4.6 MB' }],
      }),
    })

    const wrapper = mount(CardModelDownloadProgress, {
      props: { card: makeCard() },
      global: {
        plugins: [createTestI18n()],
      },
    })

    await flushPromises()
    await vi.runOnlyPendingTimersAsync()
    await flushPromises()

    expect(authFetchMock).toHaveBeenCalledWith('/api/v1/media/fallback/models/u2netp/status')
    expect(wrapper.text()).toContain('Downloading model')
    expect(wrapper.text()).toContain('u2netp.onnx')
    expect(wrapper.text()).toContain('50%')
  })

  it('stops polling after the model becomes ready', async () => {
    authFetchMock.mockResolvedValue({
      ok: true,
      json: async () => ({
        model_id: 'u2netp',
        status: 'ready',
        downloading: false,
        ready: true,
        state: 'ready',
        files: [{ filename: 'u2netp.onnx', downloaded: true, size: '~4.6 MB' }],
      }),
    })

    mount(CardModelDownloadProgress, {
      props: { card: makeCard() },
      global: {
        plugins: [createTestI18n()],
      },
    })

    await flushPromises()
    expect(authFetchMock).toHaveBeenCalledTimes(1)

    await vi.advanceTimersByTimeAsync(5000)
    await flushPromises()

    expect(authFetchMock).toHaveBeenCalledTimes(1)
  })
})
