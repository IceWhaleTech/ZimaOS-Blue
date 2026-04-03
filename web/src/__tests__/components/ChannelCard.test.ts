import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import ChannelCard from '@/components/channels/ChannelCard.vue'

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
  it('emits toggle when the summary card is clicked', async () => {
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
          fields: [],
        },
        expanded: false,
        toggling: false,
      },
    })

    await wrapper.find('.channel-card__header').trigger('click')

    expect(wrapper.emitted('toggle')).toEqual([[]])
  })

  it('emits toggleEnabled when the list switch changes', async () => {
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
          fields: [],
        },
        expanded: true,
        toggling: false,
      },
    })

    const checkbox = wrapper.find('.channel-card__toggle-input')
    expect(checkbox.exists()).toBe(true)

    await checkbox.setValue(true)

    expect(wrapper.emitted('toggleEnabled')).toEqual([[true]])
  })
})
