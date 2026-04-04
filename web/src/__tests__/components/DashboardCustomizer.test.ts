import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import { nextTick } from 'vue'
import DashboardCustomizer from '@/components/dashboard/DashboardCustomizer.vue'

vi.stubGlobal(
  'confirm',
  vi.fn(() => true)
)

const localStorageMock = (() => {
  let store: Record<string, string> = {}
  return {
    getItem: (key: string) => (key in store ? store[key] : null),
    setItem: (key: string, value: string) => {
      store[key] = String(value)
    },
    removeItem: (key: string) => {
      delete store[key]
    },
    clear: () => {
      store = {}
    },
  }
})()

vi.stubGlobal('localStorage', localStorageMock)

const messages = {
  'zh-CN': {
    common: {
      done: '完成',
    },
    dashboard: {
      customize: '配置',
      customizeTitle: '仪表盘配置',
      resetToDefaults: '恢复默认',
      confirmReset: '确认恢复默认？',
      categories: {
        all: '全部',
        overview: '概览',
        system: '系统',
        metrics: '指标',
      },
      cards: {
        metricsOverview: '指标总览',
        tokenUsageChart: 'Token 使用情况',
        latencyChart: '请求延迟',
        modelStats: '模型统计',
        failoverStatus: '故障切换状态',
        mediaGeneration: '媒体生成',
      },
    },
  },
}

describe('DashboardCustomizer', () => {
  let pinia: ReturnType<typeof createPinia>

  beforeEach(() => {
    localStorageMock.clear()
    document.body.innerHTML = ''
    pinia = createPinia()
    setActivePinia(pinia)
  })

  it('keeps the footer outside the scroll frame when toggling token usage in metrics filter', async () => {
    const i18n = createI18n({
      legacy: false,
      locale: 'zh-CN',
      messages,
    })

    const wrapper = mount(DashboardCustomizer, {
      attachTo: document.body,
      global: {
        plugins: [pinia, i18n],
        stubs: {
          teleport: true,
          transition: false,
        },
      },
    })

    await wrapper.get('.dashboard-customize-trigger').trigger('click')
    await nextTick()

    const tabs = wrapper.findAll('.dashboard-customize-tab')
    await tabs[3]?.trigger('click')
    await nextTick()

    expect(wrapper.find('.dashboard-customize-frame').exists()).toBe(true)
    expect(wrapper.findAll('.dashboard-customize-foot')).toHaveLength(1)

    const tokenUsageToggle = wrapper
      .findAll('input[type="checkbox"]')
      .find((input) => input.attributes('aria-label') === 'Token 使用情况')

    expect(tokenUsageToggle).toBeTruthy()

    await tokenUsageToggle?.trigger('change')
    await nextTick()

    expect(wrapper.find('.dashboard-customize-frame').exists()).toBe(true)
    expect(wrapper.findAll('.dashboard-customize-foot')).toHaveLength(1)
    expect(wrapper.find('.dashboard-customize-frame .dashboard-customize-foot').exists()).toBe(
      false
    )

    wrapper.unmount()
  })
})
