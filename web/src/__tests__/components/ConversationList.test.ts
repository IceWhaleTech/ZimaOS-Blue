import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import ConversationList from '@/components/ConversationList.vue'
import { i18n } from '@/i18n'

describe('ConversationList mobile header', () => {
  it('renders sidebar, create, and more-actions buttons on the mobile header', async () => {
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

    const sidebarButton = wrapper.get('[data-testid="conversation-list-expand-sidebar"]')
    const createButton = wrapper.get('[data-testid="conversation-list-create"]')
    const moreActionsButton = wrapper.get('[data-testid="conversation-list-more-actions"]')

    expect(sidebarButton.exists()).toBe(true)
    expect(createButton.exists()).toBe(true)
    expect(moreActionsButton.exists()).toBe(true)
    expect(wrapper.find('.create-btn-inline').exists()).toBe(false)
    expect(wrapper.get('.search-input-wrap').classes()).toContain('min-w-0')

    await sidebarButton.trigger('click')
    expect(toggleAppSidebar).toHaveBeenCalledTimes(1)

    await createButton.trigger('click')
    expect(wrapper.emitted('create')).toHaveLength(1)

    await moreActionsButton.trigger('click')
    expect(wrapper.emitted('more-actions')).toHaveLength(1)
  })

  it('keeps the inline create button on non-mobile layouts', () => {
    const wrapper = mount(ConversationList, {
      props: {
        conversations: [],
        currentId: null,
        mobile: false,
      },
      global: {
        plugins: [i18n],
      },
    })

    expect(wrapper.find('[data-testid="conversation-list-create"]').exists()).toBe(false)
    expect(wrapper.find('.create-btn-inline').exists()).toBe(true)
  })

  it('shows a per-conversation action button on mobile and opens the bottom sheet', async () => {
    const wrapper = mount(ConversationList, {
      attachTo: document.body,
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
      },
    })

    const overflowButton = wrapper.get('.convo-overflow-btn')
    expect(overflowButton.attributes('title')).toBe(i18n.global.t('chat.moreActions'))

    await overflowButton.trigger('click')

    expect(document.body.textContent).toContain(i18n.global.t('chat.deleteConversation'))
    expect(document.body.textContent).toContain(i18n.global.t('common.cancel'))

    wrapper.unmount()
  })
})
