import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import ConversationList from '@/components/ConversationList.vue'
import { i18n } from '@/i18n'

describe('ConversationList mobile header', () => {
  it('renders sidebar and more-actions buttons on mobile', async () => {
    const toggleAppSidebar = vi.fn()
    const wrapper = mount(ConversationList, {
      props: {
        conversations: [
          {
            id: 'conv-1',
            title: 'Plan spring release',
            updated_at: '2026-03-16T09:00:00.000Z',
          },
        ],
        currentId: null,
        mobile: true,
      },
      global: {
        plugins: [i18n],
        provide: {
          toggleAppSidebar,
        },
      },
    })

    const buttons = wrapper.findAll('button')
    const sidebarButton = buttons.find(
      (button) => button.attributes('title') === i18n.global.t('nav.expandSidebar')
    )
    const moreActionsButton = buttons.find(
      (button) => button.attributes('title') === i18n.global.t('chat.moreActions')
    )

    expect(sidebarButton?.exists()).toBe(true)
    expect(moreActionsButton?.exists()).toBe(true)
    expect(wrapper.get('.search-input-wrap').classes()).toContain('min-w-0')

    await sidebarButton!.trigger('click')
    expect(toggleAppSidebar).toHaveBeenCalledTimes(1)

    await moreActionsButton!.trigger('click')
    expect(wrapper.emitted('more-actions')).toHaveLength(1)
  })
})
