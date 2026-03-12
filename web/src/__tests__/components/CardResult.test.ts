import { describe, expect, it, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

import CardResult from '@/components/typeless/CardResult.vue'

const { openInBrowserMock } = vi.hoisted(() => ({
  openInBrowserMock: vi.fn(),
}))

vi.mock('@/composables/useTauri', () => ({
  useTauri: () => ({
    openInBrowser: openInBrowserMock,
  }),
}))

function createTestI18n(locale = 'en-US') {
  return createI18n({
    legacy: false,
    locale,
    fallbackLocale: 'en-US',
    messages: {
      'en-US': {
        common: {
          processing: 'Processing...',
          openLocation: 'Open location',
          yes: 'Yes',
          no: 'No',
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
            target_id: 'Target ID',
            strategy: 'Strategy',
            include_hidden: 'Include Hidden',
          },
          values: {
            strategy: {
              strict: 'Strict',
            },
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
          openLocation: '打开所在位置',
          yes: '是',
          no: '否',
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
            target_id: '目标 ID',
            strategy: '策略',
            include_hidden: '包含隐藏项',
          },
          values: {
            strategy: {
              strict: '严格',
            },
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
  beforeEach(() => {
    openInBrowserMock.mockReset()
    openInBrowserMock.mockResolvedValue(true)
  })

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

  it('translates snake_case detail labels and strategy values', () => {
    const wrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'Browser page',
          status: 'success',
          details: [
            { label: 'target_id', value: 'tab-42' },
            { label: 'strategy', value: 'strict' },
            { label: 'include_hidden', value: 'true' },
          ],
        },
      },
      global: {
        plugins: [createTestI18n('zh-CN')],
      },
    })

    expect(wrapper.text()).toContain('目标 ID')
    expect(wrapper.text()).toContain('策略')
    expect(wrapper.text()).toContain('严格')
    expect(wrapper.text()).toContain('是')
    expect(wrapper.text()).not.toContain('target_id')
  })

  it('renders image previews for result cards and normalizes raw base64 strings', () => {
    const base64PNG = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO7+5VQAAAAASUVORK5CYII='
    const wrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'Browser page',
          status: 'success',
          image: base64PNG,
        },
      },
      global: {
        plugins: [createTestI18n('en-US')],
      },
    })

    const image = wrapper.find('img')
    expect(image.exists()).toBe(true)
    expect(image.attributes('src')).toBe(`data:image/png;base64,${base64PNG}`)
  })

  it('renders local absolute detail paths as open-location action', async () => {
    const wrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'File Write',
          status: 'success',
          details: [
            { label: 'Path', value: '/Users/orca/Documents/report.pdf' },
          ],
        },
      },
      global: {
        plugins: [createTestI18n('en-US')],
      },
    })

    const button = wrapper.get('button')
    expect(button.text()).toContain('Open location')
    await button.trigger('click')
    expect(openInBrowserMock).toHaveBeenCalledWith('/Users/orca/Documents/report.pdf')
    expect(wrapper.find('a').exists()).toBe(false)
  })

  it('keeps /api detail paths as links instead of local file paths', () => {
    const wrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'analyze',
          status: 'success',
          details: [
            { label: 'report_url', value: '/api/v1/media/analyze/r1.html' },
          ],
        },
      },
      global: {
        plugins: [createTestI18n('en-US')],
      },
    })

    const link = wrapper.get('a')
    expect(link.attributes('href')).toBe('/api/v1/media/analyze/r1.html')
    expect(wrapper.text()).not.toContain('Open location')
  })
})
