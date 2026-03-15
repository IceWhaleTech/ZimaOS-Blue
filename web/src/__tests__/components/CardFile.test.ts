import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

import CardFile from '@/components/typeless/CardFile.vue'

const { openInBrowserMock, resolveLocalFileMock } = vi.hoisted(() => ({
  openInBrowserMock: vi.fn(),
  resolveLocalFileMock: vi.fn(),
}))

vi.mock('@/composables/useTauri', () => ({
  useTauri: () => ({
    openInBrowser: openInBrowserMock,
  }),
}))

vi.mock('@/api/system', () => ({
  systemApi: {
    resolveLocalFile: resolveLocalFileMock,
  },
}))

function createTestI18n() {
  return createI18n({
    legacy: false,
    locale: 'en-US',
    fallbackLocale: 'en-US',
    messages: {
      'en-US': {
        common: {
          preview: 'Preview',
          download: 'Download',
          openLocation: 'Open location',
        },
      },
    },
  })
}

describe('CardFile', () => {
  beforeEach(() => {
    openInBrowserMock.mockReset()
    openInBrowserMock.mockResolvedValue(true)
    resolveLocalFileMock.mockReset()
    resolveLocalFileMock.mockResolvedValue({
      data: {
        path: '/Users/orca/Documents/report.pdf',
        name: 'report.pdf',
        size_bytes: 1024,
        mime_type: 'application/pdf',
        download_url:
          '/api/v1/system/local-file/content?path=%2FUsers%2Forca%2FDocuments%2Freport.pdf',
        thumbnail_url:
          '/api/v1/system/local-file/thumbnail?path=%2FUsers%2Forca%2FDocuments%2Freport.pdf',
      },
    })
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('loads thumbnail metadata for local absolute paths', async () => {
    const wrapper = mount(CardFile, {
      props: {
        card: {
          type: 'file',
          filename: 'report.pdf',
          downloadUrl: '/Users/orca/Documents/report.pdf',
        },
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    await flushPromises()

    expect(resolveLocalFileMock).toHaveBeenCalledWith('/Users/orca/Documents/report.pdf')
    const image = wrapper.find('img')
    expect(image.exists()).toBe(true)
    expect(image.attributes('src')).toContain('/api/v1/system/local-file/thumbnail')
    expect(image.attributes('src')).toContain('size=128')
  })

  it('thumbnail click opens resolved download URL', async () => {
    const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null)
    const wrapper = mount(CardFile, {
      props: {
        card: {
          type: 'file',
          filename: 'report.pdf',
          downloadUrl: '/Users/orca/Documents/report.pdf',
        },
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    await flushPromises()

    const thumbnailButton = wrapper
      .findAll('button')
      .find((button) => button.attributes('aria-label') === 'Download')
    expect(thumbnailButton).toBeTruthy()
    await thumbnailButton!.trigger('click')

    expect(openSpy).toHaveBeenCalledWith(
      '/api/v1/system/local-file/content?path=%2FUsers%2Forca%2FDocuments%2Freport.pdf',
      '_blank'
    )
  })

  it('uses openInBrowser for local absolute paths', async () => {
    const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null)
    const wrapper = mount(CardFile, {
      props: {
        card: {
          type: 'file',
          filename: 'report.pdf',
          downloadUrl: '/Users/orca/Documents/report.pdf',
        },
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    await flushPromises()
    const actionButton = wrapper
      .findAll('button')
      .find((button) => button.attributes('aria-label') === 'Open location')
    expect(actionButton).toBeTruthy()
    await actionButton!.trigger('click')

    expect(openInBrowserMock).toHaveBeenCalledWith('/Users/orca/Documents/report.pdf')
    expect(openSpy).not.toHaveBeenCalled()
  })

  it('falls back to resolved download URL when openInBrowser fails', async () => {
    openInBrowserMock.mockResolvedValue(false)
    const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null)
    const wrapper = mount(CardFile, {
      props: {
        card: {
          type: 'file',
          filename: 'report.pdf',
          downloadUrl: '/Users/orca/Documents/report.pdf',
        },
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    await flushPromises()
    const actionButton = wrapper
      .findAll('button')
      .find((button) => button.attributes('aria-label') === 'Open location')
    expect(actionButton).toBeTruthy()
    await actionButton!.trigger('click')

    expect(openSpy).toHaveBeenCalledWith(
      '/api/v1/system/local-file/content?path=%2FUsers%2Forca%2FDocuments%2Freport.pdf',
      '_blank'
    )
  })

  it('does not open raw local path when openInBrowser fails and no download fallback is available', async () => {
    openInBrowserMock.mockResolvedValue(false)
    resolveLocalFileMock.mockRejectedValueOnce(new Error('resolve failed'))
    const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null)
    const wrapper = mount(CardFile, {
      props: {
        card: {
          type: 'file',
          filename: 'project',
          downloadUrl: '/Users/orca/Documents/project',
        },
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    await flushPromises()
    const actionButton = wrapper
      .findAll('button')
      .find((button) => button.attributes('aria-label') === 'Open location')
    expect(actionButton).toBeTruthy()
    await actionButton!.trigger('click')

    expect(openInBrowserMock).toHaveBeenCalledWith('/Users/orca/Documents/project')
    expect(openSpy).not.toHaveBeenCalled()
  })

  it('keeps URL download behavior for non-local links', async () => {
    const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null)
    const wrapper = mount(CardFile, {
      props: {
        card: {
          type: 'file',
          filename: 'report.pdf',
          downloadUrl: 'https://example.com/report.pdf',
        },
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    await wrapper.get('button').trigger('click')

    expect(resolveLocalFileMock).not.toHaveBeenCalled()
    expect(openInBrowserMock).toHaveBeenCalledWith('https://example.com/report.pdf')
    expect(openSpy).not.toHaveBeenCalled()
  })
})
