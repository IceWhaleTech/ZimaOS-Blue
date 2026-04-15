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
            browser: 'Browser',
            file_write: 'File Write',
            ls: 'ls',
            find: 'find',
            grep: 'grep',
            rg: 'rg',
            analyze: 'analyze',
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
            pattern: 'Pattern',
            max_results: 'Max Results',
            case_sensitive: 'Case Sensitive',
            backend: 'Backend',
            backend_source: 'Backend Source',
            fallback_reason: 'Fallback Reason',
          },
          values: {
            strategy: {
              strict: 'Strict',
            },
          },
          actions: {
            extract_with_web_fetch: 'Extract from Web',
          },
          messages: {
            browser_tab_ready: 'Browser tab ready',
            browser_not_running: 'Browser is not running',
            browser_service_not_available: 'Browser service not available',
            browser_service_not_available_cannot_review_url:
              'Browser service not available: cannot review URL',
            browser_page_structure_section: 'Page structure',
            browser_interactive_elements_section: 'Interactive elements',
            browser_scroll_direction_up: 'up',
            browser_scroll_direction_down: 'down',
            browser_scroll_direction_left: 'left',
            browser_scroll_direction_right: 'right',
            no_result_data: 'No result data',
            file_written_successfully: 'File written successfully',
            navigation_failed_url_not_allowed: 'Navigation failed: URL not allowed',
            proxy_bridge_not_available: 'Proxy bridge not available',
            proxy_bridge_not_available_cannot_call_vlm:
              'Proxy bridge not available: cannot call VLM',
            search_completed: 'Search completed',
            auto_answered_silent_mode: 'Auto-answered (silent mode)',
            screenshot_captured: 'Screenshot captured',
            screenshot_captured_interactive_elements_unavailable:
              'Screenshot captured (interactive elements unavailable)',
            screenshot_captured_for_active_tab: 'Screenshot captured for active tab',
          },
          messageTemplates: {
            found_results: 'Found {count} results',
            reminder_count: '{count} reminders',
            cleared_reminders: 'Cleared {count} reminders',
            entries_in_path: '{count} entries in {path}',
            single_entry_in_path: '1 entry in {path}',
            no_entries_in_path: 'No entries in {path}',
            showing_first_entries_in_path: 'Showing first {count} entries in {path} (more omitted)',
            matches_for_pattern_in_path: '{count} matches for {pattern} in {path}',
            no_matches_for_pattern_in_path: 'No matches for {pattern} in {path}',
            browser_start_failed: 'Browser start failed: {detail}',
            navigation_failed: 'Navigation failed: {detail}',
            browser_action_performed_on_ref: "Performed {action} on {'@'}{ref}",
            browser_scrolled_page: 'Scrolled page {direction}',
            browser_open_tabs: '{count} open tabs',
            browser_tab_closed: 'Tab {target} closed',
            browser_recipes_available: '{count} recipes available',
            browser_large_dom_note:
              "Page has {count} interactive elements and a large DOM. Using interactive elements list. Use 'screenshot' for visual layout.",
            browser_page: 'Page: {title} ({url})\n\n{body}',
            browser_page_with_interactive_count:
              'Page: {title} ({url}) — {count} interactive elements\n\n{body}',
            browser_page_with_screenshot_interactive_count:
              'Page: {title} ({url}) — screenshot + {count} interactive elements\n\n{body}',
            browser_page_main_content: 'Page: {title} ({url})\n\nMain content:\n{content}',
            browser_page_main_content_with_section:
              'Page: {title} ({url})\n\nMain content:\n{content}\n\n{section}:\n{body}',
            screenshot_captured_for: 'Screenshot captured for {target}',
            screenshot_captured_for_tab: 'Screenshot captured for tab {target}',
          },
          warnings: {
            listing_truncated: 'Listing was truncated; narrow the path or increase max_entries.',
          },
          copy: 'Copy',
          copied: 'Copied!',
          openLink: 'Open',
        },
        toolWarnings: {
          warning: 'Warning',
        },
        tools: {
          names: {
            web: 'Web',
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
            browser: '浏览器',
            file_write: '写入文件',
            ls: 'ls',
            find: 'find',
            grep: 'grep',
            rg: 'rg',
            analyze: '分析',
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
            pattern: '模式',
            max_results: '最大结果数',
            case_sensitive: '区分大小写',
            backend: '后端',
            backend_source: '后端来源',
            fallback_reason: '回退原因',
          },
          values: {
            strategy: {
              strict: '严格',
            },
          },
          actions: {
            extract_with_web_fetch: '从网页提取',
          },
          messages: {
            browser_tab_ready: '浏览器标签页已就绪',
            browser_not_running: '浏览器未运行',
            browser_service_not_available: '浏览器服务不可用',
            browser_service_not_available_cannot_review_url: '浏览器服务不可用：无法评审 URL',
            browser_page_structure_section: '页面结构',
            browser_interactive_elements_section: '交互元素',
            browser_scroll_direction_up: '上',
            browser_scroll_direction_down: '下',
            browser_scroll_direction_left: '左',
            browser_scroll_direction_right: '右',
            no_result_data: '没有结果数据',
            file_written_successfully: '文件写入成功',
            navigation_failed_url_not_allowed: '导航失败：URL 不被允许',
            proxy_bridge_not_available: '代理桥不可用',
            proxy_bridge_not_available_cannot_call_vlm: '代理桥不可用：无法调用 VLM',
            search_completed: '搜索完成',
            auto_answered_silent_mode: '已自动回答（静默模式）',
            screenshot_captured: '已捕获截图',
            screenshot_captured_interactive_elements_unavailable: '已捕获截图（交互元素不可用）',
            screenshot_captured_for_active_tab: '已为当前标签页捕获截图',
          },
          messageTemplates: {
            found_results: '找到 {count} 条结果',
            reminder_count: '{count} 个提醒',
            cleared_reminders: '已清除 {count} 个提醒',
            entries_in_path: '{path} 中有 {count} 个条目',
            single_entry_in_path: '{path} 中有 1 个条目',
            no_entries_in_path: '{path} 中没有条目',
            showing_first_entries_in_path: '显示 {path} 中前 {count} 个条目（更多已省略）',
            matches_for_pattern_in_path: '{path} 中找到 {count} 处匹配 {pattern}',
            no_matches_for_pattern_in_path: '{path} 中未找到 {pattern} 的匹配',
            browser_start_failed: '浏览器启动失败：{detail}',
            navigation_failed: '导航失败：{detail}',
            browser_action_performed_on_ref: "已在 {'@'}{ref} 上执行 {action}",
            browser_scrolled_page: '页面已向{direction}滚动',
            browser_open_tabs: '{count} 个打开的标签页',
            browser_tab_closed: '标签页 {target} 已关闭',
            browser_recipes_available: '{count} 个可用配方',
            browser_large_dom_note:
              '页面有 {count} 个交互元素且 DOM 很大。正在使用交互元素列表。若要查看视觉布局，请使用“screenshot”。',
            browser_page: '页面：{title} ({url})\n\n{body}',
            browser_page_with_interactive_count:
              '页面：{title} ({url}) — {count} 个交互元素\n\n{body}',
            browser_page_with_screenshot_interactive_count:
              '页面：{title} ({url}) — 截图 + {count} 个交互元素\n\n{body}',
            browser_page_main_content: '页面：{title} ({url})\n\n主要内容：\n{content}',
            browser_page_main_content_with_section:
              '页面：{title} ({url})\n\n主要内容：\n{content}\n\n{section}：\n{body}',
            screenshot_captured_for: '已为 {target} 捕获截图',
            screenshot_captured_for_tab: '已为标签页 {target} 捕获截图',
          },
          warnings: {
            listing_truncated: '列表已截断；请缩小路径范围或增大 max_entries。',
          },
          copy: '复制',
          copied: '已复制！',
          openLink: '打开',
        },
        toolWarnings: {
          warning: '警告',
        },
        tools: {
          names: {
            web: '网页',
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

    expect(wrapper.text()).toContain('网页')
    expect(wrapper.text()).toContain('浏览器标签页已就绪')
    expect(wrapper.text()).toContain('最终 URL')
    expect(wrapper.text()).toContain('标题')
    expect(wrapper.text()).toContain('检查点')
    expect(wrapper.text()).toContain('待处理')
    expect(wrapper.text()).toContain('已完成')
    expect(wrapper.text()).toContain('从网页提取')
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
    expect(wrapper.text()).not.toContain('从网页提取')
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

  it('keeps inline detail labels visually separated from their values', () => {
    const wrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'Browser page',
          status: 'success',
          details: [
            {
              label: 'fallback_reason',
              value:
                'ripgrep execution failed with exit code 2 because the configured binary was unavailable',
            },
          ],
        },
      },
      global: {
        plugins: [createTestI18n('en-US')],
      },
    })

    const row = wrapper.find('.result-detail-row--inline')
    expect(row.exists()).toBe(true)
    expect(row.classes()).toContain('gap-3')

    const label = wrapper.find('.result-detail-label')
    expect(label.exists()).toBe(true)
    expect(label.text()).toBe('Fallback Reason')

    const value = wrapper.find('.result-detail-content')
    expect(value.exists()).toBe(true)
    expect(value.classes()).toContain('min-w-0')
  })

  it('translates exact backend result messages', () => {
    const wrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'File Write',
          status: 'success',
          message: 'File written successfully',
        },
      },
      global: {
        plugins: [createTestI18n('zh-CN')],
      },
    })

    expect(wrapper.text()).toContain('文件写入成功')
    expect(wrapper.text()).not.toContain('File written successfully')
  })

  it('translates exact browser navigation errors without leaking fallback keys', () => {
    const wrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'Browser',
          status: 'error',
          message: 'navigation failed: URL not allowed',
        },
      },
      global: {
        plugins: [createTestI18n('zh-CN')],
      },
    })

    expect(wrapper.text()).toContain('导航失败：URL 不被允许')
    expect(wrapper.text()).not.toContain('navigation failed: URL not allowed')
    expect(wrapper.text()).not.toContain('uiReview.errors.navigationFailed')
  })

  it('translates exact browser infrastructure errors', () => {
    const wrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'Browser',
          status: 'error',
          message: 'browser service not available — cannot review URL',
        },
      },
      global: {
        plugins: [createTestI18n('zh-CN')],
      },
    })

    expect(wrapper.text()).toContain('浏览器服务不可用：无法评审 URL')
    expect(wrapper.text()).not.toContain('browser service not available')
  })

  it('translates templated browser startup and navigation failures', () => {
    const wrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'Browser',
          status: 'error',
          message: 'browser start failed: launch timeout',
        },
      },
      global: {
        plugins: [createTestI18n('zh-CN')],
      },
    })

    expect(wrapper.text()).toContain('浏览器启动失败：launch timeout')
    expect(wrapper.text()).not.toContain('browser start failed: launch timeout')
  })

  it('translates proxy bridge errors for browser review flows', () => {
    const wrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'Browser',
          status: 'error',
          message: 'proxy bridge not available — cannot call VLM',
        },
      },
      global: {
        plugins: [createTestI18n('zh-CN')],
      },
    })

    expect(wrapper.text()).toContain('代理桥不可用：无法调用 VLM')
    expect(wrapper.text()).not.toContain('proxy bridge not available')
  })

  it('translates legacy browser scroll summaries with localized directions', () => {
    const wrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'Browser',
          status: 'success',
          message: 'Scrolled page down',
        },
      },
      global: {
        plugins: [createTestI18n('zh-CN')],
      },
    })

    expect(wrapper.text()).toContain('页面已向下滚动')
    expect(wrapper.text()).not.toContain('Scrolled page down')
  })

  it('translates legacy browser page summaries with localized section labels', () => {
    const wrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'Browser',
          status: 'success',
          message:
            'Page: Example (https://example.com)\n\nMain content:\nAlpha\n\nInteractive elements:\n[@1] button "OK"',
        },
      },
      global: {
        plugins: [createTestI18n('zh-CN')],
      },
    })

    expect(wrapper.text()).toContain('页面：Example (https://example.com)')
    expect(wrapper.text()).toContain('主要内容：')
    expect(wrapper.text()).toContain('交互元素：')
    expect(wrapper.text()).not.toContain('Main content:')
    expect(wrapper.text()).not.toContain('Interactive elements:')
  })

  it('translates templated backend summaries and warnings', () => {
    const wrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'ls',
          status: 'warning',
          message: 'Showing first 12 entries in /tmp/demo (more omitted)',
          warning: 'Listing was truncated; narrow the path or increase max_entries.',
        },
      },
      global: {
        plugins: [createTestI18n('zh-CN')],
      },
    })

    expect(wrapper.text()).toContain('显示 /tmp/demo 中前 12 个条目（更多已省略）')
    expect(wrapper.text()).toContain('列表已截断；请缩小路径范围或增大 max_entries。')
    expect(wrapper.text()).not.toContain(
      'Listing was truncated; narrow the path or increase max_entries.'
    )
  })

  it('renders raw ls JSON payload messages as a structured directory listing', () => {
    const wrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'ls',
          status: 'info',
          message: JSON.stringify({
            base_path: '.',
            count: 2,
            entries: [
              {
                path: '.blue/',
                type: 'dir',
                mode: 'drwxr-xr-x',
                size: 96,
                modified_at: '2026-03-23T16:15:35Z',
              },
              {
                path: 'README.md',
                type: 'file',
                mode: '-rw-r--r--',
                size: 1024,
                modified_at: '2026-03-23T16:15:35Z',
              },
            ],
            max_depth: 1,
            max_entries: 200,
            include_hidden: false,
          }),
        },
      },
      global: {
        plugins: [createTestI18n('zh-CN')],
      },
    })

    expect(wrapper.text()).toContain('. 中有 2 个条目')
    expect(wrapper.text()).toContain('.blue/')
    expect(wrapper.text()).toContain('README.md')
    expect(wrapper.text()).not.toContain('"base_path"')
    expect(wrapper.text()).not.toContain('"entries"')
  })

  it('renders ls preview details as directory rows instead of raw bracketed lines', () => {
    const wrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'ls',
          status: 'success',
          message: '3 entries in .',
          details: [
            { label: 'path', value: '.' },
            { label: 'count', value: '3' },
            {
              label: 'entries',
              value: '[dir] sub\n[file] README.md (123 B)\n[dir] server',
              multiline: true,
            },
          ],
        },
      },
      global: {
        plugins: [createTestI18n('en-US')],
      },
    })

    expect(wrapper.text()).toContain('sub')
    expect(wrapper.text()).toContain('README.md')
    expect(wrapper.text()).toContain('server')
    expect(wrapper.text()).not.toContain('[dir] sub')
    expect(wrapper.text()).not.toContain('[file] README.md (123 B)')
  })

  it('renders find results as a structured search listing instead of raw JSON arrays', () => {
    const wrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'find',
          status: 'success',
          details: [
            { label: 'base_path', value: 'web/src' },
            { label: 'pattern', value: '*.vue' },
            { label: 'type', value: 'file' },
            {
              label: 'entries',
              value: JSON.stringify([
                { path: 'components/ChatMessage.vue', type: 'file', size: 2048 },
                { path: 'views/ChatView.vue', type: 'file', size: 4096 },
              ]),
              multiline: true,
            },
          ],
        },
      },
      global: {
        plugins: [createTestI18n('en-US')],
      },
    })

    expect(wrapper.text()).toContain('2 matches for *.vue in web/src')
    expect(wrapper.text()).toContain('Pattern')
    expect(wrapper.text()).toContain('*.vue')
    expect(wrapper.text()).toContain('components/ChatMessage.vue')
    expect(wrapper.text()).toContain('views/ChatView.vue')
    expect(wrapper.text()).not.toContain('[{"path":"components/ChatMessage.vue"')
  })

  it('renders grep details as a specialized text-search card instead of raw matches JSON', () => {
    const wrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'grep',
          status: 'success',
          details: [
            { label: 'path', value: 'web/src' },
            { label: 'pattern', value: 'CardResult' },
            { label: 'max_results', value: '20' },
            { label: 'case_sensitive', value: 'false' },
            { label: 'include_hidden', value: 'true' },
            { label: 'backend', value: 'builtin' },
            { label: 'fallback_reason', value: 'ripgrep execution failed with exit code 2' },
            {
              label: 'matches',
              value: JSON.stringify([
                {
                  path: 'components/typeless/CardResult.vue',
                  line: 42,
                  column: 7,
                  preview: 'const cardResult = true',
                },
                {
                  path: 'views/ChatView.vue',
                  line: 103,
                  column: 15,
                  preview: 'import CardResult from \"@/components/typeless/CardResult.vue\"',
                },
              ]),
              multiline: true,
            },
          ],
        },
      },
      global: {
        plugins: [createTestI18n('en-US')],
      },
    })

    expect(wrapper.text()).toContain('2 matches for CardResult in web/src')
    expect(wrapper.text()).toContain('Max Results')
    expect(wrapper.text()).toContain('Case Sensitive')
    expect(wrapper.text()).toContain('Include Hidden')
    expect(wrapper.text()).toContain('Backend')
    expect(wrapper.text()).toContain('builtin')
    expect(wrapper.text()).toContain('components/typeless/CardResult.vue')
    expect(wrapper.text()).toContain('views/ChatView.vue')
    expect(wrapper.text()).toContain('L42:C7')
    expect(wrapper.text()).toContain('L103:C15')
    expect(wrapper.text()).toContain('const cardResult = true')
    expect(wrapper.text()).toContain('Fallback Reason')
    expect(wrapper.text()).toContain('ripgrep execution failed with exit code 2')
    expect(wrapper.text()).not.toContain(
      '[{"path":"components/typeless/CardResult.vue","line":42,"column":7'
    )
  })

  it('renders rg message payloads as the same specialized search card and localizes the summary', () => {
    const wrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'rg',
          status: 'info',
          message: JSON.stringify({
            path: 'web/src',
            pattern: 'createTestI18n',
            count: 1,
            max_results: 50,
            case_sensitive: false,
            backend: 'ripgrep',
            matches: [
              {
                path: 'tests/components/CardResult.test.ts',
                line: 16,
                column: 10,
                preview: 'function createTestI18n(locale = \"en-US\") {',
              },
            ],
          }),
        },
      },
      global: {
        plugins: [createTestI18n('zh-CN')],
      },
    })

    expect(wrapper.text()).toContain('web/src 中找到 1 处匹配 createTestI18n')
    expect(wrapper.text()).toContain('后端')
    expect(wrapper.text()).toContain('ripgrep')
    expect(wrapper.text()).toContain('tests/components/CardResult.test.ts')
    expect(wrapper.text()).toContain('L16:C10')
    expect(wrapper.text()).not.toContain('1 matches for createTestI18n in web/src')
  })

  it('synthesizes no-match grep summaries from structured payloads without showing raw JSON', () => {
    const wrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'grep',
          status: 'info',
          message: JSON.stringify({
            path: 'server/internal',
            pattern: 'DefinitelyMissingToken',
            count: 0,
            max_results: 25,
            case_sensitive: true,
            matches: [],
          }),
        },
      },
      global: {
        plugins: [createTestI18n('en-US')],
      },
    })

    expect(wrapper.text()).toContain('No matches for DefinitelyMissingToken in server/internal')
    expect(wrapper.text()).toContain('Pattern')
    expect(wrapper.text()).toContain('DefinitelyMissingToken')
    expect(wrapper.text()).not.toContain('"matches":[]')
  })

  it('renders image previews for result cards and normalizes raw base64 strings', () => {
    const base64PNG =
      'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO7+5VQAAAAASUVORK5CYII='
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

  it('extracts screenshot details into an image preview and hides raw base64 detail text', () => {
    const base64PNG =
      'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO7+5VQAAAAASUVORK5CYII='
    const wrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'Browser',
          status: 'success',
          details: [{ label: 'screenshot', value: base64PNG }],
        },
      },
      global: {
        plugins: [createTestI18n('en-US')],
      },
    })

    const image = wrapper.find('img')
    expect(image.exists()).toBe(true)
    expect(image.attributes('src')).toBe(`data:image/png;base64,${base64PNG}`)
    expect(wrapper.text()).not.toContain(base64PNG)
    expect(wrapper.text()).not.toContain('screenshot')
  })

  it('extracts screenshot previews from JSON-encoded message payloads', () => {
    const base64PNG =
      'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO7+5VQAAAAASUVORK5CYII='
    const wrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'browser',
          status: 'info',
          message: JSON.stringify({
            message: 'Screenshot captured for https://example.com',
            screenshot: base64PNG,
          }),
        },
      },
      global: {
        plugins: [createTestI18n('en-US')],
      },
    })

    const image = wrapper.find('img')
    expect(image.exists()).toBe(true)
    expect(image.attributes('src')).toBe(`data:image/png;base64,${base64PNG}`)
    expect(wrapper.text()).toContain('Screenshot captured for https://example.com')
    expect(wrapper.text()).not.toContain(base64PNG)
    expect(wrapper.text()).not.toContain('{"message"')
  })

  it('translates screenshot-captured summary templates from JSON-encoded message payloads', () => {
    const base64PNG =
      'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO7+5VQAAAAASUVORK5CYII='
    const wrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'browser',
          status: 'info',
          message: JSON.stringify({
            message: 'Screenshot captured for https://example.com',
            screenshot: base64PNG,
          }),
        },
      },
      global: {
        plugins: [createTestI18n('zh-CN')],
      },
    })

    expect(wrapper.text()).toContain('已为 https://example.com 捕获截图')
    expect(wrapper.text()).not.toContain('Screenshot captured for https://example.com')
  })

  it('translates screenshot-captured tab summary templates from JSON-encoded message payloads', () => {
    const base64PNG =
      'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO7+5VQAAAAASUVORK5CYII='
    const wrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'browser',
          status: 'info',
          message: JSON.stringify({
            message: 'Screenshot captured for tab tab-7',
            screenshot: base64PNG,
          }),
        },
      },
      global: {
        plugins: [createTestI18n('zh-CN')],
      },
    })

    expect(wrapper.text()).toContain('已为标签页 tab-7 捕获截图')
    expect(wrapper.text()).not.toContain('Screenshot captured for tab tab-7')
  })

  it('translates screenshot-captured active-tab summaries from JSON-encoded message payloads', () => {
    const base64PNG =
      'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO7+5VQAAAAASUVORK5CYII='
    const wrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'browser',
          status: 'info',
          message: JSON.stringify({
            message: 'Screenshot captured for active tab',
            screenshot: base64PNG,
          }),
        },
      },
      global: {
        plugins: [createTestI18n('zh-CN')],
      },
    })

    expect(wrapper.text()).toContain('已为当前标签页捕获截图')
    expect(wrapper.text()).not.toContain('Screenshot captured for active tab')
  })

  it('bridges local screenshot file paths from JSON-encoded message payloads', () => {
    const screenshotPath = '/Users/orca/.zimaos-blue/data/browser-checkpoints/example-shot.png'
    const wrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'browser',
          status: 'info',
          message: JSON.stringify({
            message: 'Screenshot captured for https://example.com',
            screenshot: screenshotPath,
          }),
        },
      },
      global: {
        plugins: [createTestI18n('en-US')],
      },
    })

    const image = wrapper.find('img')
    expect(image.exists()).toBe(true)
    expect(image.attributes('src')).toBe(
      '/api/v1/system/local-file/content?path=%2FUsers%2Forca%2F.zimaos-blue%2Fdata%2Fbrowser-checkpoints%2Fexample-shot.png&inline=1'
    )
    expect(wrapper.text()).toContain('Screenshot captured for https://example.com')
    expect(wrapper.text()).not.toContain(screenshotPath)
  })

  it('extracts screenshots arrays from detail payloads into a gallery', () => {
    const pngA =
      'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO7+5VQAAAAASUVORK5CYII='
    const pngB =
      'iVBORw0KGgoAAAANSUhEUgAAAAIAAAACCAQAAADZc7J/AAAADUlEQVR42mNk+M/wHwAFAgJ/l8V6NwAAAABJRU5ErkJggg=='
    const wrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'Browser',
          status: 'success',
          details: [{ label: 'screenshots', value: JSON.stringify([pngA, pngB]) }],
        },
      },
      global: {
        plugins: [createTestI18n('en-US')],
      },
    })

    const images = wrapper.findAll('img')
    expect(images).toHaveLength(2)
    expect(images[0]?.attributes('src')).toBe(`data:image/png;base64,${pngA}`)
    expect(images[1]?.attributes('src')).toBe(`data:image/png;base64,${pngB}`)
  })

  it('renders top-level result images as a gallery before falling back to details parsing', () => {
    const pngA =
      'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO7+5VQAAAAASUVORK5CYII='
    const pngB =
      'iVBORw0KGgoAAAANSUhEUgAAAAIAAAACCAQAAADZc7J/AAAADUlEQVR42mNk+M/wHwAFAgJ/l8V6NwAAAABJRU5ErkJggg=='
    const wrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'Browser',
          status: 'success',
          images: [
            { src: `data:image/png;base64,${pngA}`, alt: 'shot-a' },
            { src: `data:image/png;base64,${pngB}`, alt: 'shot-b' },
          ],
          details: [{ label: 'count', value: '2' }],
        },
      },
      global: {
        plugins: [createTestI18n('en-US')],
      },
    })

    const images = wrapper.findAll('img')
    expect(images).toHaveLength(2)
    expect(images[0]?.attributes('src')).toBe(`data:image/png;base64,${pngA}`)
    expect(images[1]?.attributes('src')).toBe(`data:image/png;base64,${pngB}`)
    expect(wrapper.text()).toContain('2')
  })

  it('renders local absolute detail paths as open-location action', async () => {
    const wrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'File Write',
          status: 'success',
          details: [{ label: 'Path', value: '/Users/orca/Documents/report.pdf' }],
        },
      },
      global: {
        plugins: [createTestI18n('en-US')],
      },
    })

    const button = wrapper.get('button')
    expect(button.text()).toContain('Open location')
    expect(wrapper.text()).toContain('/Users/orca/Documents/report.pdf')
    await button.trigger('click')
    expect(openInBrowserMock).toHaveBeenCalledWith('/Users/orca/Documents/report.pdf')
    expect(wrapper.find('a').exists()).toBe(false)
  })

  it('shows the original path while revealing via a separate absolute path', async () => {
    const wrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'File Write',
          status: 'success',
          details: [
            {
              label: 'Path',
              value: '@docs/reports/summary.md',
              reveal_path: '/Users/orca/Documents/project/reports/summary.md',
            },
          ],
        },
      },
      global: {
        plugins: [createTestI18n('zh-CN')],
      },
    })

    expect(wrapper.text()).toContain('@docs/reports/summary.md')
    expect(wrapper.text()).toContain('打开所在位置')
    await wrapper.get('button').trigger('click')
    expect(openInBrowserMock).toHaveBeenCalledWith(
      '/Users/orca/Documents/project/reports/summary.md'
    )
  })

  it('keeps /api detail paths as links instead of local file paths', () => {
    const wrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'analyze',
          status: 'success',
          details: [{ label: 'report_url', value: '/api/v1/media/analyze/r1.html' }],
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

  it('renders a localized empty-state message instead of a blank body', () => {
    const wrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'web_query',
          status: 'info',
        },
      },
      global: {
        plugins: [createTestI18n('zh-CN')],
      },
    })

    expect(wrapper.text()).toContain('网页')
    expect(wrapper.text()).toContain('没有结果数据')
  })
})
