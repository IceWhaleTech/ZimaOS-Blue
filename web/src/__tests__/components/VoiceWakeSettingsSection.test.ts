import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import { createI18n } from 'vue-i18n'
import VoiceWakeSettingsSection from '@/components/settings/VoiceWakeSettingsSection.vue'

const { openInBrowserMock, isCurrentHostLoopbackMock } = vi.hoisted(() => ({
  openInBrowserMock: vi.fn(async () => true),
  isCurrentHostLoopbackMock: vi.fn(() => false),
}))

const settingsState = reactive<Record<string, any>>({
  backendSettings: {},
  fetchBackendSettings: vi.fn(async () => {}),
  updateBackendSettings: vi.fn(async (updates: Record<string, unknown>) => {
    Object.assign(settingsState.backendSettings, updates)
  }),
})

const chatState = reactive<Record<string, any>>({
  currentConversationId: 'conv-current',
  currentConversation: {
    id: 'conv-current',
    title: 'Focused build',
    created_at: '',
    updated_at: '',
  },
})

vi.mock('@/stores/settings', () => ({
  useSettingsStore: () => settingsState,
}))

vi.mock('@/stores/chat', () => ({
  useChatStore: () => chatState,
}))

vi.mock('@/composables/useTauri', () => ({
  useTauri: () => ({
    openInBrowser: openInBrowserMock,
    platform: { value: 'macos' },
  }),
}))

vi.mock('@/utils/localPath', () => ({
  isCurrentHostLoopback: isCurrentHostLoopbackMock,
}))

vi.mock('@/api/chat', () => ({
  conversationApi: {
    list: vi.fn(),
    get: vi.fn(),
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

function createTestI18n() {
  return createI18n({
    legacy: false,
    locale: 'en-US',
    fallbackLocale: 'en-US',
    messages: {
      'en-US': {
        common: {
          checking: 'Checking...',
          retry: 'Retry',
          retrying: 'Retrying...',
          save: 'Save',
          saving: 'Saving...',
          saved: 'Saved',
          refreshing: 'Refreshing...',
          enable: 'Enable',
          unknown: 'Unknown',
        },
        cardActions: {
          recheck: 'Re-check',
        },
        speech: {
          voiceWake: {
            title: 'Voice Wake',
            description:
              'Keep listening for your wake words in the background, then send the spoken command to one fixed conversation.',
            enableTitle: 'Background listening',
            enableDescription:
              'Turn this on to keep listening for wake words in the background. Commands are only sent after a wake word is detected.',
            statusRunning: 'Running',
            statusIdle: 'Idle',
            speechStatusLabel: 'Speech',
            microphoneStatusLabel: 'Mic',
            statusReady: 'Ready',
            statusMissing: 'Missing',
            statusNotReady: 'Not ready',
            messages: {
              running: 'Listening for wake words in the background.',
              disabled: 'VoiceWake is off.',
              targetMissing: 'Choose a target conversation before enabling VoiceWake.',
              targetUnavailable:
                'The selected conversation is unavailable. Pick another conversation and save again.',
              speechPermissionDenied: 'Speech recognition permission is missing.',
              microphoneUnavailable: 'Microphone input is unavailable.',
              startFailed: 'VoiceWake failed to start.',
              sendFailed: 'VoiceWake captured a command but failed to send it.',
              runtimeError: 'VoiceWake stopped after a runtime error.',
              unsupported:
                'VoiceWake is only available when Blue is running locally on macOS (desktop app or interactive bluecli).',
              unavailable: 'VoiceWake status is unavailable.',
            },
            lastError: 'Last error',
            activeTriggers: 'Active triggers',
            targetConversation: 'Target conversation',
            lastTriggered: 'Last triggered',
            lastSent: 'Last sent',
            wakeWords: 'Wake words',
            wakeWordsHelp:
              'You can enter multiple wake words. Separate them with commas or line breaks.',
            listeningTitle: 'Wake words and recognition',
            listeningDescription:
              'Set the wake words first, then choose the recognition language if needed.',
            localeOverride: 'Recognition language (optional)',
            localeHelp:
              'Optional. Enter a locale such as zh-CN or en-US. Leave this blank to follow the system default.',
            selectConversation: 'Select a conversation',
            currentChatPrefix: 'Current chat',
            savedTargetPrefix: 'Saved target',
            useCurrentChat: 'Use current chat',
            routingTitle: 'Command destination',
            targetFieldLabel: 'Send commands to',
            targetDescription:
              'After Voice Wake is triggered, every command is always sent to this fixed conversation instead of following the currently open chat.',
            openTargetChat: 'Open this conversation',
            fixedTargetWarning: 'Pick the fixed target conversation before turning VoiceWake on.',
            desktopNoteDescription: 'Desktop wake words are managed in Settings > Speech.',
            openSpeechSettings: 'Open Speech Settings',
            openMicrophoneSettings: 'Open Microphone Settings',
            chooseConversation: 'Choose Conversation',
            notSelected: 'Not selected',
            never: 'Never',
          },
        },
        voiceView: {
          wakeWordPlaceholder: 'e.g., Hey Blue',
        },
      },
    },
  })
}

function mountSection() {
  return mount(VoiceWakeSettingsSection, {
    global: {
      plugins: [createTestI18n()],
    },
  })
}

describe('VoiceWakeSettingsSection', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.clearAllMocks()
    settingsState.backendSettings = {}
    settingsState.fetchBackendSettings = vi.fn(async () => {})
    settingsState.updateBackendSettings = vi.fn(async (updates: Record<string, unknown>) => {
      Object.assign(settingsState.backendSettings, updates)
    })
    chatState.currentConversationId = 'conv-current'
    chatState.currentConversation = {
      id: 'conv-current',
      title: 'Focused build',
      created_at: '',
      updated_at: '',
    }
    isCurrentHostLoopbackMock.mockReturnValue(false)
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
    vi.mocked(conversationApi.get).mockImplementation(
      async (id: string) =>
        ({
          data: {
            id,
            title:
              id === 'conv-saved'
                ? 'Archived chat'
                : id === 'conv-status'
                  ? 'Runtime target'
                  : `Conversation ${id}`,
            created_at: '',
            updated_at: '',
          },
        }) as never
    )
  })

  afterEach(() => {
    vi.clearAllTimers()
    vi.useRealTimers()
    delete (window as any).__BLUE_DESKTOP__
  })

  it('shows on desktop when backend reports support', async () => {
    const wrapper = mountSection()
    await flushPromises()

    expect(wrapper.find('[data-testid="voicewake-section"]').exists()).toBe(true)
    expect(voiceWakeApi.getStatus).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })

  it('stays hidden outside the desktop shell', async () => {
    delete (window as any).__BLUE_DESKTOP__
    const wrapper = mountSection()
    await flushPromises()

    expect(wrapper.find('[data-testid="voicewake-section"]').exists()).toBe(true)
    expect(voiceWakeApi.getStatus).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('running locally on macOS')
    expect(wrapper.find('[data-testid="voicewake-save"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('loads VoiceWake status on a local macOS browser session', async () => {
    delete (window as any).__BLUE_DESKTOP__
    isCurrentHostLoopbackMock.mockReturnValue(true)

    const wrapper = mountSection()
    await flushPromises()

    expect(voiceWakeApi.getStatus).toHaveBeenCalledTimes(1)
    expect(wrapper.find('[data-testid="voicewake-save"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('still shows the VoiceWake entry when desktop runtime reports unsupported', async () => {
    vi.mocked(voiceWakeApi.getStatus).mockResolvedValueOnce({
      data: {
        supported: false,
        enabled: false,
        running: false,
        platform: 'darwin',
        speech_authorized: false,
        microphone_ready: false,
        reason: 'unsupported',
      },
    } as never)

    const wrapper = mountSection()
    await flushPromises()

    expect(wrapper.find('[data-testid="voicewake-section"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('running locally on macOS')
    expect(wrapper.find('[data-testid="voicewake-save"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('keeps the toggle in the header and merges locale into the wake-word card', async () => {
    const wrapper = mountSection()
    await flushPromises()

    const wakeWordCard = wrapper.get('[data-testid="voicewake-wakeword-config"]')
    const localeInline = wrapper.get('[data-testid="voicewake-locale-inline"]')

    expect(wrapper.find('[data-testid="voicewake-header-toggle"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="voicewake-inline-config-grid"]').exists()).toBe(true)
    expect(wakeWordCard.element.contains(localeInline.element)).toBe(true)
    expect(wrapper.text()).not.toContain('Background listening')
    expect(wrapper.text()).not.toContain('Command destination')
    wrapper.unmount()
  })

  it('uses the current chat automatically when enabling without a saved target', async () => {
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
          target_conversation_id: 'conv-current',
        },
      } as never)

    const wrapper = mountSection()
    await flushPromises()

    const enable = wrapper.get('[data-testid="voicewake-enabled"]')
    expect((enable.element as HTMLInputElement).disabled).toBe(false)

    await enable.setValue(true)
    await flushPromises()

    expect(settingsState.updateBackendSettings).toHaveBeenCalledWith({
      voice_wake_enabled: true,
      voice_wake_triggers: ['Hey Blue'],
      voice_wake_locale: '',
      voice_wake_target_conversation_id: 'conv-current',
    })
    expect(
      (wrapper.get('[data-testid="voicewake-target"]').element as HTMLSelectElement).value
    ).toBe('conv-current')
    expect(wrapper.find('[data-testid="voicewake-target-warning"]').exists()).toBe(false)
    wrapper.unmount()
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
    wrapper.unmount()
  })

  it('auto-saves wake-word settings and refreshes runtime status', async () => {
    settingsState.backendSettings = {
      voice_wake_enabled: true,
      voice_wake_triggers: ['Hey Blue'],
      voice_wake_locale: '',
      voice_wake_target_conversation_id: 'conv-1',
    }
    vi.mocked(voiceWakeApi.getStatus)
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
          triggers: ['Hey Blue'],
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
          triggers: ['Hey Blue'],
        },
      } as never)

    const wrapper = mountSection()
    await flushPromises()

    await wrapper.get('[data-testid="voicewake-triggers"]').setValue('Hey Blue, Jarvis')
    await wrapper.get('[data-testid="voicewake-locale"]').setValue('en-US')
    vi.advanceTimersByTime(700)
    await flushPromises()

    expect(settingsState.updateBackendSettings).toHaveBeenCalledWith({
      voice_wake_enabled: true,
      voice_wake_triggers: ['Hey Blue', 'Jarvis'],
      voice_wake_locale: 'en-US',
      voice_wake_target_conversation_id: 'conv-1',
    })
    expect(voiceWakeApi.getStatus).toHaveBeenCalledTimes(2)
    expect(wrapper.get('[data-testid="voicewake-status-message"]').text()).toContain(
      'Listening for wake words'
    )
    expect(wrapper.get('[data-testid="voicewake-autosave-status"]').text()).toContain('Saved')
    wrapper.unmount()
  })

  it('polls runtime status while mounted', async () => {
    const wrapper = mountSection()
    await flushPromises()

    vi.advanceTimersByTime(5000)
    await flushPromises()

    expect(voiceWakeApi.getStatus).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })

  it('surfaces recent wake activity when runtime timestamps change', async () => {
    vi.setSystemTime(new Date('2026-03-17T10:00:02.000Z'))
    vi.mocked(voiceWakeApi.getStatus)
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
          triggers: ['Hey Blue'],
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
          triggers: ['Hey Blue'],
          last_triggered_at: '2026-03-17T10:00:00.000Z',
          last_sent_at: '2026-03-17T10:00:01.000Z',
        },
      } as never)

    const wrapper = mountSection()
    await flushPromises()

    expect(wrapper.get('[data-testid="voicewake-activity-title"]').text()).toContain(
      'Listening for wake words'
    )

    vi.advanceTimersByTime(1200)
    await flushPromises()

    expect(wrapper.get('[data-testid="voicewake-activity-title"]').text()).toContain('Last sent')
    expect(wrapper.get('[data-testid="voicewake-activity-meta"]').text()).toContain(
      'Target conversation: Main conversation'
    )
    expect(wrapper.get('[data-testid="voicewake-last-sent-card"]').classes()).toContain(
      'border-green-200'
    )
    wrapper.unmount()
  })

  it('offers a shortcut to speech settings when permission is denied', async () => {
    vi.mocked(voiceWakeApi.getStatus).mockResolvedValueOnce({
      data: {
        supported: true,
        enabled: true,
        running: false,
        platform: 'darwin',
        speech_authorized: false,
        microphone_ready: false,
        reason: 'speech_permission_denied',
      },
    } as never)

    const wrapper = mountSection()
    await flushPromises()

    await wrapper.get('[data-testid="voicewake-open-speech-settings"]').trigger('click')

    expect(openInBrowserMock).toHaveBeenCalledWith(
      'x-apple.systempreferences:com.apple.preference.security?Privacy_SpeechRecognition'
    )
    wrapper.unmount()
  })

  it('keeps the current chat available as a target and can pick it in one click', async () => {
    const wrapper = mountSection()
    await flushPromises()
    await flushPromises()

    const options = wrapper
      .findAll('[data-testid="voicewake-target"] option')
      .map((option) => option.text())

    expect(options).toContain('Current chat: Focused build')

    await wrapper.get('[data-testid="voicewake-use-current"]').trigger('click')
    await flushPromises()

    expect(
      (wrapper.get('[data-testid="voicewake-target"]').element as HTMLSelectElement).value
    ).toBe('conv-current')
    expect(settingsState.updateBackendSettings).toHaveBeenCalledWith({
      voice_wake_enabled: false,
      voice_wake_triggers: ['Hey Blue'],
      voice_wake_locale: '',
      voice_wake_target_conversation_id: 'conv-current',
    })
    wrapper.unmount()
  })

  it('saves the selected target conversation immediately', async () => {
    const wrapper = mountSection()
    await flushPromises()

    await wrapper.get('[data-testid="voicewake-target"]').setValue('conv-1')
    await flushPromises()

    expect(settingsState.updateBackendSettings).toHaveBeenCalledWith({
      voice_wake_enabled: false,
      voice_wake_triggers: ['Hey Blue'],
      voice_wake_locale: '',
      voice_wake_target_conversation_id: 'conv-1',
    })
    wrapper.unmount()
  })

  it('preserves an already saved target even when it is outside the first page of conversations', async () => {
    settingsState.backendSettings = {
      voice_wake_enabled: true,
      voice_wake_triggers: ['Hey Blue'],
      voice_wake_locale: '',
      voice_wake_target_conversation_id: 'conv-saved',
    }

    const wrapper = mountSection()
    await flushPromises()

    const options = wrapper
      .findAll('[data-testid="voicewake-target"] option')
      .map((option) => option.text())

    expect(conversationApi.get).toHaveBeenCalledWith('conv-saved')
    expect(options).toContain('Saved target: Archived chat')
    expect(
      (wrapper.get('[data-testid="voicewake-target"]').element as HTMLSelectElement).value
    ).toBe('conv-saved')
    wrapper.unmount()
  })

  it('shows the runtime target conversation title when it can hydrate the conversation details', async () => {
    vi.mocked(voiceWakeApi.getStatus).mockResolvedValueOnce({
      data: {
        supported: true,
        enabled: true,
        running: true,
        platform: 'darwin',
        speech_authorized: true,
        microphone_ready: true,
        reason: 'running',
        target_conversation_id: 'conv-status',
      },
    } as never)

    const wrapper = mountSection()
    await flushPromises()

    expect(conversationApi.get).toHaveBeenCalledWith('conv-status')
    expect(wrapper.text()).toContain('Target conversation')
    expect(wrapper.text()).toContain('Runtime target')
    wrapper.unmount()
  })

  it('hides the darwin platform label and highlights ready speech and mic states', async () => {
    const wrapper = mountSection()
    await flushPromises()

    expect(wrapper.find('[data-testid="voicewake-platform-status"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="voicewake-speech-status"]').classes()).toContain(
      'bg-green-100'
    )
    expect(wrapper.get('[data-testid="voicewake-mic-status"]').classes()).toContain('bg-green-100')
    wrapper.unmount()
  })
})
