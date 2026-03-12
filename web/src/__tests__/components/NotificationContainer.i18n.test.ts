import { beforeEach, describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { createI18n } from 'vue-i18n'
import { createPinia, setActivePinia } from 'pinia'

import NotificationContainer from '@/components/NotificationContainer.vue'
import { useNotificationStore } from '@/stores/notification'

function createTestI18n() {
  return createI18n({
    legacy: false,
    locale: 'zh-CN',
    fallbackLocale: 'en-US',
    messages: {
      'en-US': {
        companion: {
          toasts: {
            manageMemory: 'Manage memory',
            memorySavedTitle: 'Remembered new content',
            memorySavedMessage: 'Extracted memory from the conversation',
          },
        },
        providerPool: {
          probeComplete: 'Probe complete: {available}/{total} models available',
        },
      },
      'zh-CN': {
        companion: {
          toasts: {
            manageMemory: '管理记忆',
            memorySavedTitle: '已记住新内容',
            memorySavedMessage: '已从对话中提取记忆',
          },
        },
        providerPool: {
          probeComplete: '探测完成：{available}/{total} 个模型可用',
        },
      },
    },
  })
}

describe('NotificationContainer i18n', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })


  it('renders translated params for keyed titles', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const store = useNotificationStore()
    store.add({
      type: 'success',
      title: 'Probe complete: 3/8 models available',
      titleKey: 'providerPool.probeComplete',
      titleParams: { available: 3, total: 8 },
      duration: 0,
    })

    const wrapper = mount(NotificationContainer, {
      global: {
        plugins: [pinia, createTestI18n()],
        stubs: {
          Teleport: true,
          TransitionGroup: false,
        },
      },
    })

    await nextTick()

    expect(wrapper.text()).toContain('探测完成：3/8 个模型可用')
    expect(wrapper.text()).not.toContain('Probe complete: 3/8 models available')
  })

  it('renders notification action label from labelKey', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const store = useNotificationStore()
    store.add({
      type: 'info',
      title: 'Remembered new content',
      titleKey: 'companion.toasts.memorySavedTitle',
      message: 'Extracted memory from the conversation',
      messageKey: 'companion.toasts.memorySavedMessage',
      action: {
        label: 'Manage memory',
        labelKey: 'companion.toasts.manageMemory',
        handler: () => {},
      },
      duration: 0,
    })

    const wrapper = mount(NotificationContainer, {
      global: {
        plugins: [pinia, createTestI18n()],
        stubs: {
          Teleport: true,
          TransitionGroup: false,
        },
      },
    })

    await nextTick()

    expect(wrapper.text()).toContain('已记住新内容')
    expect(wrapper.text()).toContain('已从对话中提取记忆')
    expect(wrapper.text()).toContain('管理记忆')
    expect(wrapper.text()).not.toContain('Remembered new content')
    expect(wrapper.text()).not.toContain('Extracted memory from the conversation')
    expect(wrapper.text()).not.toContain('Manage memory')
  })
})
