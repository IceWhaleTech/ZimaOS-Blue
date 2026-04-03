import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import ChannelDetailPanel from '@/components/channels/ChannelDetailPanel.vue'

vi.mock('@/composables/useTauri', () => ({
  useTauri: () => ({
    openInBrowser: vi.fn(),
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')

  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
      te: () => true,
    }),
  }
})

describe('ChannelDetailPanel', () => {
  it('renders password fields through the dedicated wrapper used for shared input styling', () => {
    const wrapper = mount(ChannelDetailPanel, {
      props: {
        channel: {
          id: 'telegram',
          name: 'Telegram',
          icon: '/icons/channels/telegram.svg',
          enabled: false,
          status: 'disconnected',
          descriptionKey: 'channels.telegramDesc',
          hintKey: 'channels.telegramHint',
          fields: [
            {
              key: 'bot_token',
              labelKey: 'channels.botToken',
              type: 'password',
              placeholderKey: 'channels.placeholderBotToken',
              value: '',
              required: true,
            },
          ],
        },
        toggling: false,
        saving: false,
        testingConnection: false,
        testResult: null,
      },
      global: {
        mocks: {
          $t: (key: string) => key,
        },
      },
    })

    const passwordInput = wrapper.find('input.channel-detail__password-field')

    expect(passwordInput.exists()).toBe(true)
    expect(passwordInput.attributes('type')).toBe('password')
  })

  it('renders toggle fields and emits string booleans when switched', async () => {
    const wrapper = mount(ChannelDetailPanel, {
      props: {
        channel: {
          id: 'feishu',
          name: 'Feishu',
          icon: '/icons/channels/feishu.svg',
          enabled: false,
          status: 'disconnected',
          descriptionKey: 'channels.feishuDesc',
          hintKey: 'channels.feishuHint',
          fields: [
            {
              key: 'session_mode',
              labelKey: 'channels.feishuSessionMode',
              type: 'toggle',
              value: 'false',
            },
          ],
        },
        toggling: false,
        saving: false,
        testingConnection: false,
        testResult: null,
      },
    })

    const checkbox = wrapper.find('.channel-detail__toggle-field input[type="checkbox"].sr-only')
    expect(checkbox.exists()).toBe(true)
    expect(wrapper.text()).toContain('channels.feishuSessionMode')
    expect(wrapper.text()).toContain('common.disabled')

    await checkbox.setValue(true)

    expect(wrapper.emitted('updateField')).toEqual([[0, 'true']])
  })
})
