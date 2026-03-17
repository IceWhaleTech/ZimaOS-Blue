import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { createMemoryHistory, createRouter } from 'vue-router'
import GlobalVoiceWakeBanner from '@/components/voicewake/GlobalVoiceWakeBanner.vue'

const { isCurrentHostLoopbackMock } = vi.hoisted(() => ({
  isCurrentHostLoopbackMock: vi.fn(() => false),
}))

vi.mock('@/composables/useTauri', () => ({
  useTauri: () => ({
    platform: { value: 'macos' },
  }),
}))

vi.mock('@/utils/localPath', () => ({
  isCurrentHostLoopback: isCurrentHostLoopbackMock,
}))

vi.mock('@/api/chat', () => ({
  conversationApi: {
    get: vi.fn(),
  },
}))

vi.mock('@/api/voiceWake', () => ({
  voiceWakeApi: {
    getStatus: vi.fn(),
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
        speech: {
          voiceWake: {
            title: 'Voice Wake',
            activeTriggers: 'Active triggers',
            targetConversation: 'Target conversation',
            lastTriggered: 'Last triggered',
            lastSent: 'Last sent',
            openTargetChat: 'Open target chat',
            notSelected: 'Not selected',
            never: 'Never',
            messages: {
              running: 'Listening for wake words in the background.',
            },
          },
        },
      },
    },
  })
}

async function mountBanner(path = '/home') {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/home', name: 'Home', component: { template: '<div>Home</div>' } },
      { path: '/chat', name: 'Chat', component: { template: '<div>Chat</div>' } },
    ],
  })
  router.push(path)
  await router.isReady()

  const wrapper = mount(GlobalVoiceWakeBanner, {
    global: {
      plugins: [router, createTestI18n()],
      stubs: {
        teleport: true,
      },
    },
  })

  return { wrapper, router }
}

describe('GlobalVoiceWakeBanner', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.clearAllMocks()
    ;(window as any).__BLUE_DESKTOP__ = true
    vi.mocked(conversationApi.get).mockResolvedValue({
      data: {
        id: 'conv-1',
        title: 'Main conversation',
        created_at: '',
        updated_at: '',
      },
    } as never)
  })

  afterEach(() => {
    vi.clearAllTimers()
    vi.useRealTimers()
    delete (window as any).__BLUE_DESKTOP__
  })

  it('shows a global listening banner while VoiceWake is running', async () => {
    vi.mocked(voiceWakeApi.getStatus).mockResolvedValue({
      data: {
        supported: true,
        enabled: true,
        running: true,
        platform: 'darwin',
        reason: 'running',
        speech_authorized: true,
        microphone_ready: true,
        target_conversation_id: 'conv-1',
        triggers: ['Hey Blue'],
      },
    } as never)

    const { wrapper } = await mountBanner('/home')
    await flushPromises()

    expect(wrapper.find('[data-testid="global-voicewake-banner"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="global-voicewake-title"]').text()).toContain(
      'Listening for wake words'
    )
    expect(wrapper.get('[data-testid="global-voicewake-meta"]').text()).toContain(
      'Active triggers: Hey Blue'
    )
    expect(wrapper.find('[data-testid="global-voicewake-open-target"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('promotes recent wake activity into a global sent-state banner', async () => {
    vi.setSystemTime(new Date('2026-03-17T10:00:02.000Z'))
    vi.mocked(voiceWakeApi.getStatus)
      .mockResolvedValueOnce({
        data: {
          supported: true,
          enabled: true,
          running: true,
          platform: 'darwin',
          reason: 'running',
          speech_authorized: true,
          microphone_ready: true,
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
          reason: 'running',
          speech_authorized: true,
          microphone_ready: true,
          target_conversation_id: 'conv-1',
          triggers: ['Hey Blue'],
          last_triggered_at: '2026-03-17T10:00:00.000Z',
          last_sent_at: '2026-03-17T10:00:01.000Z',
        },
      } as never)

    const { wrapper } = await mountBanner('/home')
    await flushPromises()

    vi.advanceTimersByTime(1200)
    await flushPromises()

    expect(wrapper.get('[data-testid="global-voicewake-title"]').text()).toContain('Last sent')
    expect(wrapper.get('[data-testid="global-voicewake-meta"]').text()).toContain(
      'Target conversation: Main conversation'
    )
    expect(wrapper.get('[data-testid="global-voicewake-banner"]').find('.is-sent').exists()).toBe(
      true
    )
    expect(wrapper.find('[data-testid="global-voicewake-open-target"]').exists()).toBe(true)
    wrapper.unmount()
  })

  it('opens the target conversation from the global banner action', async () => {
    vi.setSystemTime(new Date('2026-03-17T10:00:02.000Z'))
    vi.mocked(voiceWakeApi.getStatus).mockResolvedValue({
      data: {
        supported: true,
        enabled: true,
        running: true,
        platform: 'darwin',
        reason: 'running',
        speech_authorized: true,
        microphone_ready: true,
        target_conversation_id: 'conv-1',
        triggers: ['Hey Blue'],
        last_sent_at: '2026-03-17T10:00:01.000Z',
      },
    } as never)

    const { wrapper, router } = await mountBanner('/home')
    await flushPromises()

    await wrapper.get('[data-testid="global-voicewake-open-target"]').trigger('click')
    await flushPromises()

    expect(router.currentRoute.value.name).toBe('Chat')
    expect(router.currentRoute.value.query.conversationId).toBe('conv-1')
    wrapper.unmount()
  })

  it('hides the action when already viewing the target conversation', async () => {
    vi.mocked(voiceWakeApi.getStatus).mockResolvedValue({
      data: {
        supported: true,
        enabled: true,
        running: true,
        platform: 'darwin',
        reason: 'running',
        speech_authorized: true,
        microphone_ready: true,
        target_conversation_id: 'conv-1',
        triggers: ['Hey Blue'],
        last_sent_at: '2026-03-17T10:00:01.000Z',
      },
    } as never)

    const { wrapper } = await mountBanner('/chat?conversationId=conv-1')
    await flushPromises()

    expect(wrapper.find('[data-testid="global-voicewake-open-target"]').exists()).toBe(false)
    wrapper.unmount()
  })
})
