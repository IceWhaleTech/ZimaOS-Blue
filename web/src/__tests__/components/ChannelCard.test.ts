import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import ChannelCard from '@/components/channels/ChannelCard.vue'

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

describe('ChannelCard', () => {
  it('renders password fields through the dedicated wrapper used for shared input styling', () => {
    const wrapper = mount(ChannelCard, {
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
        expanded: true,
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

    const passwordInput = wrapper.find('input.channel-card__password-field')

    expect(passwordInput.exists()).toBe(true)
    expect(passwordInput.attributes('type')).toBe('password')
  })
})
