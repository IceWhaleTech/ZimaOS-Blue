import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import VoiceWakeSettingsSection from '@/components/settings/VoiceWakeSettingsSection.vue'

const settingsState = reactive<Record<string, any>>({
  backendSettings: {},
  fetchBackendSettings: vi.fn(async () => {}),
  updateBackendSettings: vi.fn(async (updates: Record<string, unknown>) => {
    Object.assign(settingsState.backendSettings, updates)
  }),
})

vi.mock('@/stores/settings', () => ({
  useSettingsStore: () => settingsState,
}))

vi.mock('@/api/chat', () => ({
  conversationApi: {
    list: vi.fn(),
  },
}))

vi.mock('@/api/voiceWake', () => ({
  voiceWakeApi: {
    getStatus: vi.fn(),
    restart: vi.fn(),
  },
}))

import { conversationApi } from '@/api/chat'
import { voiceWakeApi } from '@/api/voiceWake'

function mountSection() {
  return mount(VoiceWakeSettingsSection)
}

describe('VoiceWakeSettingsSection', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    settingsState.backendSettings = {}
    settingsState.fetchBackendSettings = vi.fn(async () => {})
    settingsState.updateBackendSettings = vi.fn(async (updates: Record<string, unknown>) => {
      Object.assign(settingsState.backendSettings, updates)
    })
    ;(window as any).__BLUE_DESKTOP__ = true

    vi.mocked(voiceWakeApi.getStatus).mockResolvedValue({
      data: {
        supported: true,
        enabled: false,
        running: false,
        platform: 'darwin',
        speech_authorized: true,
        microphone_ready: true,
        reason: 'disabled',
      },
    } as never)
    vi.mocked(voiceWakeApi.restart).mockResolvedValue({
      data: {
        supported: true,
        enabled: true,
        running: true,
        platform: 'darwin',
        speech_authorized: true,
        microphone_ready: true,
        reason: 'running',
      },
    } as never)
    vi.mocked(conversationApi.list).mockResolvedValue({
      data: [{ id: 'conv-1', title: 'Main conversation', created_at: '', updated_at: '' }],
    } as never)
  })

  afterEach(() => {
    delete (window as any).__BLUE_DESKTOP__
  })

  it('shows on desktop when backend reports support', async () => {
    const wrapper = mountSection()
    await flushPromises()

    expect(wrapper.find('[data-testid="voicewake-section"]').exists()).toBe(true)
    expect(voiceWakeApi.getStatus).toHaveBeenCalledTimes(1)
  })

  it('stays hidden outside the desktop shell', async () => {
    delete (window as any).__BLUE_DESKTOP__
    const wrapper = mountSection()
    await flushPromises()

    expect(wrapper.find('[data-testid="voicewake-section"]').exists()).toBe(false)
    expect(voiceWakeApi.getStatus).not.toHaveBeenCalled()
  })

  it('disables the enable toggle until a target conversation is selected', async () => {
    const wrapper = mountSection()
    await flushPromises()

    const enable = wrapper.get('[data-testid="voicewake-enabled"]')
    expect((enable.element as HTMLInputElement).disabled).toBe(true)
    expect(wrapper.get('[data-testid="voicewake-target-warning"]').text()).toContain(
      'Pick the fixed target conversation'
    )
  })

  it('renders backend error state from status', async () => {
    vi.mocked(voiceWakeApi.getStatus).mockResolvedValueOnce({
      data: {
        supported: true,
        enabled: true,
        running: false,
        platform: 'darwin',
        speech_authorized: true,
        microphone_ready: false,
        reason: 'target_unavailable',
        last_error: 'conversation missing',
      },
    } as never)

    const wrapper = mountSection()
    await flushPromises()

    expect(wrapper.get('[data-testid="voicewake-status-message"]').text()).toContain('unavailable')
    expect(wrapper.get('[data-testid="voicewake-status-error"]').text()).toContain(
      'conversation missing'
    )
  })

  it('saves settings and refreshes runtime status', async () => {
    settingsState.backendSettings = {
      voice_wake_enabled: false,
      voice_wake_triggers: ['Blue'],
      voice_wake_locale: '',
      voice_wake_target_conversation_id: '',
    }
    vi.mocked(voiceWakeApi.getStatus)
      .mockResolvedValueOnce({
        data: {
          supported: true,
          enabled: false,
          running: false,
          platform: 'darwin',
          speech_authorized: true,
          microphone_ready: true,
          reason: 'disabled',
        },
      } as never)
      .mockResolvedValueOnce({
        data: {
          supported: true,
          enabled: true,
          running: true,
          platform: 'darwin',
          speech_authorized: true,
          microphone_ready: true,
          reason: 'running',
          target_conversation_id: 'conv-1',
          triggers: ['Blue'],
        },
      } as never)

    const wrapper = mountSection()
    await flushPromises()

    await wrapper.get('[data-testid="voicewake-target"]').setValue('conv-1')
    await wrapper.get('[data-testid="voicewake-enabled"]').setValue(true)
    await wrapper.get('[data-testid="voicewake-save"]').trigger('click')
    await flushPromises()

    expect(settingsState.updateBackendSettings).toHaveBeenCalledWith({
      voice_wake_enabled: true,
      voice_wake_triggers: ['Blue'],
      voice_wake_locale: '',
      voice_wake_target_conversation_id: 'conv-1',
    })
    expect(voiceWakeApi.getStatus).toHaveBeenCalledTimes(2)
    expect(wrapper.get('[data-testid="voicewake-status-message"]').text()).toContain(
      'Listening for wake words'
    )
  })
})
