import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

const { info, ota, releaseNotes } = vi.hoisted(() => ({
  info: vi.fn(),
  ota: vi.fn(),
  releaseNotes: vi.fn(),
}))

vi.mock('@/api/update', () => ({
  updateApi: {
    info,
    ota,
    releaseNotes,
    check: vi.fn(),
    download: vi.fn(),
    downloadOTA: vi.fn(),
    apply: vi.fn(),
    rollback: vi.fn(),
    history: vi.fn(),
    health: vi.fn(),
  },
}))

vi.mock('@/utils/markdown', () => ({
  renderMarkdown: (value: string) => value,
}))

import UpdateSettings from '@/components/settings/UpdateSettings.vue'

function createTestI18n() {
  return createI18n({
    legacy: false,
    locale: 'en-US',
    fallbackLocale: 'en-US',
    messages: {
      'en-US': {
        common: {
          cancel: 'Cancel',
          update: 'Update',
        },
        settings: {
          update: {
            title: 'About',
            currentVersion: 'Current Version',
            autoCheck: 'Auto check for updates on startup',
            checkNow: 'Check Now',
            newVersionAvailable: 'New Version Available',
            downloading: 'Downloading...',
            downloadComplete: 'Download complete',
            download: 'Download',
            confirmRestart: 'Apply & Restart',
            applying: 'Applying update...',
            restarting: 'Restarting...',
            waitingForServer: 'Waiting for server to come back...',
          },
        },
      },
    },
  })
}

describe('UpdateSettings', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.clearAllMocks()
    const localStorageMock = {
      getItem: vi.fn(() => null),
      setItem: vi.fn(),
      removeItem: vi.fn(),
      clear: vi.fn(),
    }
    Object.defineProperty(window, 'localStorage', {
      value: localStorageMock,
      configurable: true,
    })
    Object.defineProperty(globalThis, 'localStorage', {
      value: localStorageMock,
      configurable: true,
    })

    info.mockResolvedValue({
      data: {
        current_version: '1.0.0',
        status: {
          state: 'idle',
          progress: 0,
          last_checked: '',
        },
        latest: null,
        uptime: '1h',
      },
    })
    ota.mockResolvedValue({
      data: {
        current_version: '1.0.0',
        update_available: true,
        latest_version: '1.1.0',
        release_note_url: 'https://example.com/release-notes',
        delay: 86400,
      },
    })
    releaseNotes.mockResolvedValue({
      data: '# Release Notes',
    })
  })

  afterEach(() => {
    vi.clearAllTimers()
    vi.useRealTimers()
  })

  it('renders an enabled update button without showing raw delay seconds', async () => {
    const wrapper = mount(UpdateSettings, {
      global: {
        plugins: [createTestI18n()],
        stubs: {
          Teleport: true,
        },
      },
    })

    await flushPromises()

    const updateButton = wrapper.findAll('button').find((node) => node.text().trim() === 'Update')

    expect(updateButton).toBeTruthy()
    expect(updateButton!.attributes('disabled')).toBeUndefined()
    expect(wrapper.text()).not.toContain('86400s')

    wrapper.unmount()
  })
})
