import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

import CardResult from '@/components/typeless/CardResult.vue'

function createTestI18n(locale = 'en-US') {
  return createI18n({
    legacy: false,
    locale,
    fallbackLocale: 'en-US',
    messages: {
      'en-US': {
        common: {
          processing: 'Processing...',
        },
        resultCard: {
          titles: {
            browser_page: 'Browser page',
          },
          labels: {
            final_url: 'Final URL',
            title: 'Title',
            checkpoint: 'Checkpoint',
            pending: 'Pending',
            completed: 'Completed',
          },
          actions: {
            extract_with_web_fetch: 'Extract with Web Fetch',
          },
          messages: {
            browser_tab_ready: 'Browser tab ready',
            no_result_data: 'No result data',
          },
          copy: 'Copy',
          copied: 'Copied!',
          openLink: 'Open',
        },
        tools: {
          names: {
            web_crawl: 'Web Crawl',
          },
        },
      },
      'zh-CN': {
        common: {
          processing: '处理中...',
        },
        resultCard: {
          titles: {
            browser_page: 'Browser page',
          },
          labels: {
            final_url: '最终 URL',
            title: '标题',
            checkpoint: '检查点',
            pending: '待处理',
            completed: '已完成',
          },
          actions: {
            extract_with_web_fetch: '使用网页抓取提取',
          },
          messages: {
            browser_tab_ready: '浏览器标签页已就绪',
            no_result_data: '没有结果数据',
          },
          copy: '复制',
          copied: '已复制！',
          openLink: '打开',
        },
        tools: {
          names: {
            web_crawl: '网页爬取',
          },
        },
      },
    },
  })
}

describe('CardResult', () => {
  it('localizes generic result-card titles, messages, actions, and nested labels', () => {
    const wrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'web_crawl',
          status: 'info',
          message: 'Browser tab ready',
          details: [
            { label: 'final_url', value: 'https://example.com/final' },
            { label: 'title', value: 'Example page' },
            { label: 'checkpoint', value: '{"pending":2,"completed":1}' },
          ],
          actions: [
            { id: 'extract_with_web_fetch', label: 'Extract with web_fetch', variant: 'primary' },
          ],
        },
      },
      global: {
        plugins: [createTestI18n('zh-CN')],
      },
    })

    expect(wrapper.text()).toContain('网页爬取')
    expect(wrapper.text()).toContain('浏览器标签页已就绪')
    expect(wrapper.text()).toContain('最终 URL')
    expect(wrapper.text()).toContain('标题')
    expect(wrapper.text()).toContain('检查点')
    expect(wrapper.text()).toContain('待处理')
    expect(wrapper.text()).toContain('已完成')
    expect(wrapper.text()).toContain('使用网页抓取提取')
    expect(wrapper.text()).not.toContain('Extract with web_fetch')
  })

  it('preserves richer custom action labels instead of overriding them', () => {
    const wrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'Browser page',
          status: 'info',
          actions: [
            { id: 'extract_with_web_fetch', label: 'Extract readable content', variant: 'primary' },
          ],
        },
      },
      global: {
        plugins: [createTestI18n('zh-CN')],
      },
    })

    expect(wrapper.text()).toContain('Extract readable content')
    expect(wrapper.text()).not.toContain('使用网页抓取提取')
  })
})
