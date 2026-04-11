import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

import CardProgress from '@/components/typeless/CardProgress.vue'

function createTestI18n() {
  return createI18n({
    legacy: false,
    locale: 'zh-CN',
    fallbackLocale: 'en-US',
    messages: {
      'en-US': {
        chat: {
          taskStageCompleted: 'Completed',
        },
        system: {
          statusOk: 'OK',
        },
      },
      'zh-CN': {
        chat: {
          taskStageCompleted: '已完成',
        },
        system: {
          statusOk: '正常',
        },
      },
    },
  })
}

describe('CardProgress i18n', () => {
  it('localizes the headline status token', () => {
    const wrapper = mount(CardProgress, {
      props: {
        card: {
          id: 'progress-1',
          type: 'progress',
          title: '任务',
          status: 'completed',
          progress: 100,
        },
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    expect(wrapper.text()).toContain('已完成')
    expect(wrapper.text()).not.toContain('completed')
  })
})
