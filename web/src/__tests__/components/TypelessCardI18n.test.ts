import { afterEach, describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { createPinia } from 'pinia'
import { nextTick } from 'vue'

import CardAccordion from '@/components/typeless/CardAccordion.vue'
import CardChoice from '@/components/typeless/CardChoice.vue'
import CardCountdown from '@/components/typeless/CardCountdown.vue'
import CardFile from '@/components/typeless/CardFile.vue'
import CardResult from '@/components/typeless/CardResult.vue'
import FullscreenModal from '@/components/typeless/FullscreenModal.vue'
import { fullscreenContent, isFullscreen } from '@/composables/useFullscreen'
import { mergeHarnessLocale as mergeRuntimeHarnessLocale } from '@/i18n/harness-locale-additions'

function createTestI18n(locale = 'zh-CN') {
  return createI18n({
    legacy: false,
    locale,
    fallbackLocale: 'en-US',
    messages: {
      'en-US': {
        common: {
          preview: 'Preview',
          download: 'Download',
          selectMultipleOptions: 'Select multiple options',
          processing: 'Processing...',
          copied: 'Copied!',
          copy: 'Copy',
        },
        askQuestion: {
          other: 'Other',
          otherPlaceholder: 'Type your answer...',
        },
        countdownCard: {
          expired: "Time's up!",
          days: 'Days',
          hours: 'Hours',
          minutes: 'Minutes',
          seconds: 'Seconds',
        },
        accordionCard: {
          thinking: 'Thinking',
        },
        fullscreenModal: {
          exitHint: 'Press {key} or double-click to exit fullscreen',
        },
      },
      'zh-CN': {
        common: {
          preview: '预览',
          download: '下载',
          selectMultipleOptions: '选择多个选项',
          processing: '处理中...',
          copied: '已复制',
          copy: '复制',
        },
        askQuestion: {
          other: '其他',
          otherPlaceholder: '输入你的回答...',
        },
        countdownCard: {
          expired: '时间到了！',
          days: '天',
          hours: '小时',
          minutes: '分钟',
          seconds: '秒',
        },
        accordionCard: {
          thinking: '思考中',
        },
        fullscreenModal: {
          exitHint: '按 {key} 或双击退出全屏',
        },
      },
    },
  })
}

afterEach(() => {
  isFullscreen.value = false
  fullscreenContent.value = null
  document.body.style.overflow = ''
  document.body.innerHTML = ''
})

describe('Typeless card i18n', () => {
  it('localizes file action titles', () => {
    const wrapper = mount(CardFile, {
      props: {
        card: {
          type: 'file',
          filename: 'report.pdf',
          previewUrl: 'https://example.com/preview',
          downloadUrl: 'https://example.com/download',
        },
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    const buttons = wrapper.findAll('button')
    expect(buttons[0]?.attributes('title')).toBe('预览')
    expect(buttons[0]?.attributes('aria-label')).toBe('预览')
    expect(buttons[1]?.attributes('title')).toBe('下载')
    expect(buttons[1]?.attributes('aria-label')).toBe('下载')
  })

  it('localizes countdown expired and unit labels', () => {
    const expiredWrapper = mount(CardCountdown, {
      props: {
        card: {
          type: 'countdown',
          targetDate: new Date(Date.now() - 1000).toISOString(),
        },
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    expect(expiredWrapper.text()).toContain('时间到了！')

    const runningWrapper = mount(CardCountdown, {
      props: {
        card: {
          type: 'countdown',
          targetDate: new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString(),
        },
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    expect(runningWrapper.text()).toContain('天')
    expect(runningWrapper.text()).toContain('小时')
    expect(runningWrapper.text()).toContain('分钟')
    expect(runningWrapper.text()).toContain('秒')
  })

  it('localizes choice helper copy and loading state', async () => {
    const wrapper = mount(CardChoice, {
      props: {
        card: {
          type: 'choice',
          title: 'Choose',
          multiple: true,
          allowOther: true,
          options: [{ id: 'a', label: 'Alpha' }],
        },
        actionLoading: true,
        activeActionId: 'select',
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    expect(wrapper.text()).toContain('选择多个选项')
    expect(wrapper.text()).toContain('其他')
    expect(wrapper.text()).toContain('处理中...')

    await wrapper.findAll('button')[1]?.trigger('click')
    await nextTick()

    const input = wrapper.find('input')
    expect(input.exists()).toBe(false)
  })

  it('localizes ask result detail labels at render time', () => {
    const wrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'ask',
          status: 'success',
          details: [
            { label: 'q', value: '你想怎么做？' },
            { label: 'o', value: '方案 A / 方案 B' },
            { label: 'a', value: '方案 A' },
          ],
        },
      },
      global: {
        plugins: [
          createI18n({
            legacy: false,
            locale: 'zh-CN',
            fallbackLocale: 'en-US',
            messages: {
              'en-US': mergeRuntimeHarnessLocale('en-US', {
                common: {
                  yes: 'Yes',
                  no: 'No',
                },
                resultCard: {
                  labels: {},
                },
              }),
              'zh-CN': mergeRuntimeHarnessLocale('zh-CN', {
                common: {
                  yes: '是',
                  no: '否',
                },
                resultCard: {
                  labels: {},
                },
              }),
            },
          }),
        ],
      },
    })

    expect(wrapper.text()).toContain('问题')
    expect(wrapper.text()).toContain('选项')
    expect(wrapper.text()).toContain('回答')
    expect(wrapper.text()).not.toContain('\nq\n')
    expect(wrapper.text()).not.toContain('\no\n')
  })

  it('localizes host result messages and compact field labels at render time', () => {
    const wrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'computer_use',
          status: 'success',
          message: 'Host screenshot captured',
          details: [
            { label: 'host_os', value: 'windows' },
            { label: 'window_id', value: 'main' },
            { label: 'image_path', value: '/tmp/host-window-main.png' },
            { label: 'selected_source_rank', value: '2' },
            { label: 'output_path', value: '/tmp/output.png' },
            { label: 'target_format', value: 'png' },
            { label: 'warning_count', value: '1' },
          ],
        },
      },
      global: {
        plugins: [
          createI18n({
            legacy: false,
            locale: 'zh-CN',
            fallbackLocale: 'en-US',
            messages: {
              'en-US': mergeRuntimeHarnessLocale('en-US', {
                common: {
                  yes: 'Yes',
                  no: 'No',
                },
                resultCard: {
                  labels: {},
                  messages: {},
                },
              }),
              'zh-CN': mergeRuntimeHarnessLocale('zh-CN', {
                common: {
                  yes: '是',
                  no: '否',
                },
                resultCard: {
                  labels: {},
                  messages: {},
                },
              }),
            },
          }),
        ],
      },
    })

    expect(wrapper.text()).toContain('已捕获主机截图')
    expect(wrapper.text()).toContain('主机系统')
    expect(wrapper.text()).toContain('窗口 ID')
    expect(wrapper.text()).toContain('图像路径')
    expect(wrapper.text()).toContain('选中来源排名')
    expect(wrapper.text()).toContain('输出路径')
    expect(wrapper.text()).toContain('目标格式')
    expect(wrapper.text()).toContain('警告数量')

    expect(wrapper.text()).not.toContain('Host screenshot captured')
    expect(wrapper.text()).not.toContain('host_os')
    expect(wrapper.text()).not.toContain('selected_source_rank')
    expect(wrapper.text()).not.toContain('warning_count')
  })

  it('localizes window focus messages plus execution mode and host OS values at render time', () => {
    const wrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'computer_use',
          status: 'success',
          message: 'Window focused',
          details: [
            { label: 'execution_mode', value: 'semantic' },
            { label: 'host_os', value: 'darwin' },
            { label: 'window_id', value: '14433' },
          ],
        },
      },
      global: {
        plugins: [
          createI18n({
            legacy: false,
            locale: 'zh-CN',
            fallbackLocale: 'en-US',
            messages: {
              'en-US': mergeRuntimeHarnessLocale('en-US', {
                common: {
                  yes: 'Yes',
                  no: 'No',
                },
                resultCard: {
                  labels: {},
                  messages: {},
                  values: {},
                },
              }),
              'zh-CN': mergeRuntimeHarnessLocale('zh-CN', {
                common: {
                  yes: '是',
                  no: '否',
                },
                resultCard: {
                  labels: {},
                  messages: {},
                  values: {},
                },
              }),
            },
          }),
        ],
      },
    })

    expect(wrapper.text()).toContain('窗口已聚焦')
    expect(wrapper.text()).toContain('执行模式')
    expect(wrapper.text()).toContain('语义')
    expect(wrapper.text()).toContain('主机系统')
    expect(wrapper.text()).toContain('macOS')
    expect(wrapper.text()).toContain('窗口 ID')

    expect(wrapper.text()).not.toContain('Window focused')
    expect(wrapper.text()).not.toContain('execution_mode')
    expect(wrapper.text()).not.toContain('semantic')
    expect(wrapper.text()).not.toContain('darwin')
  })

  it('localizes host snapshot readiness and recovered browser-tab focus labels at render time', () => {
    const plugins = [
      createI18n({
        legacy: false,
        locale: 'zh-CN',
        fallbackLocale: 'en-US',
        messages: {
          'en-US': mergeRuntimeHarnessLocale('en-US', {
            common: {
              yes: 'Yes',
              no: 'No',
            },
            resultCard: {
              labels: {},
              messages: {},
            },
          }),
          'zh-CN': mergeRuntimeHarnessLocale('zh-CN', {
            common: {
              yes: '是',
              no: '否',
            },
            resultCard: {
              labels: {},
              messages: {},
            },
          }),
        },
      }),
    ]

    const snapshotWrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'computer_use',
          status: 'success',
          message: 'Host computer-use snapshot ready',
          details: [
            { label: 'recovered_target_id', value: 'node-12' },
            { label: 'surface', value: 'main' },
            { label: 'automation', value: 'enabled' },
            { label: 'browser', value: 'chrome' },
            { label: 'ref_map', value: '{"tab":"active"}' },
            { label: 'windows', value: '2' },
            { label: 'tree_fetch_ms', value: '18' },
          ],
        },
      },
      global: {
        plugins,
      },
    })

    expect(snapshotWrapper.text()).toContain('主机电脑使用快照已就绪')
    expect(snapshotWrapper.text()).toContain('已恢复的目标 ID')
    expect(snapshotWrapper.text()).toContain('界面')
    expect(snapshotWrapper.text()).toContain('自动化')
    expect(snapshotWrapper.text()).toContain('浏览器')
    expect(snapshotWrapper.text()).toContain('引用映射')
    expect(snapshotWrapper.text()).toContain('窗口')
    expect(snapshotWrapper.text()).toContain('树获取耗时 (ms)')

    expect(snapshotWrapper.text()).not.toContain('Host computer-use snapshot ready')
    expect(snapshotWrapper.text()).not.toContain('recovered_target_id')
    expect(snapshotWrapper.text()).not.toContain('tree_fetch_ms')

    const focusRecoveryWrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'computer_use',
          status: 'success',
          message: 'Browser tab focus recovered using active tab',
          details: [{ label: 'browser', value: 'chrome' }],
        },
      },
      global: {
        plugins,
      },
    })

    expect(focusRecoveryWrapper.text()).toContain('已使用活动标签页恢复浏览器标签页焦点')
    expect(focusRecoveryWrapper.text()).not.toContain(
      'Browser tab focus recovered using active tab'
    )
  })

  it('localizes browser bridge, focus, and browser-surface values at render time', () => {
    const plugins = [
      createI18n({
        legacy: false,
        locale: 'zh-CN',
        fallbackLocale: 'en-US',
        messages: {
          'en-US': mergeRuntimeHarnessLocale('en-US', {
            common: {
              yes: 'Yes',
              no: 'No',
            },
            resultCard: {
              labels: {},
              messages: {},
              values: {},
            },
          }),
          'zh-CN': mergeRuntimeHarnessLocale('zh-CN', {
            common: {
              yes: '是',
              no: '否',
            },
            resultCard: {
              labels: {},
              messages: {},
              values: {},
            },
          }),
        },
      }),
    ]

    const bridgeWrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'computer_use',
          status: 'success',
          message: 'Browser computer-use bridge ready',
          details: [
            { label: 'surface', value: 'browser' },
            { label: 'host_os', value: 'browser' },
            { label: 'execution_mode', value: 'automation' },
          ],
        },
      },
      global: {
        plugins,
      },
    })

    expect(bridgeWrapper.text()).toContain('浏览器电脑控制桥接已就绪')
    expect(bridgeWrapper.text()).toContain('界面')
    expect(bridgeWrapper.text()).toContain('浏览器')
    expect(bridgeWrapper.text()).toContain('主机系统')
    expect(bridgeWrapper.text()).toContain('执行模式')
    expect(bridgeWrapper.text()).toContain('自动化')
    expect(bridgeWrapper.text()).not.toContain('Browser computer-use bridge ready')

    const focusWrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'computer_use',
          status: 'success',
          message: 'Browser tab focused',
          details: [{ label: 'window_id', value: 'tab-3' }],
        },
      },
      global: {
        plugins,
      },
    })

    expect(focusWrapper.text()).toContain('浏览器标签页已聚焦')
    expect(focusWrapper.text()).not.toContain('Browser tab focused')
  })

  it('localizes remaining host/browser action status messages at render time', () => {
    const plugins = [
      createI18n({
        legacy: false,
        locale: 'zh-CN',
        fallbackLocale: 'en-US',
        messages: {
          'en-US': mergeRuntimeHarnessLocale('en-US', {
            common: {
              yes: 'Yes',
              no: 'No',
            },
            resultCard: {
              labels: {},
              messages: {},
            },
          }),
          'zh-CN': mergeRuntimeHarnessLocale('zh-CN', {
            common: {
              yes: '是',
              no: '否',
            },
            resultCard: {
              labels: {},
              messages: {},
            },
          }),
        },
      }),
    ]

    const messages = [
      ['Browser keys sent', '浏览器按键已发送'],
      ['Host accessibility snapshot ready', '主机无障碍快照已就绪'],
      ['Application activated', '应用已激活'],
      ['Scroll completed', '滚动已完成'],
      ['Pointer moved', '指针已移动'],
    ] as const

    for (const [message, localized] of messages) {
      const wrapper = mount(CardResult, {
        props: {
          card: {
            type: 'result',
            title: 'computer_use',
            status: 'success',
            message,
            details: [],
          },
        },
        global: {
          plugins,
        },
      })

      expect(wrapper.text()).toContain(localized)
      expect(wrapper.text()).not.toContain(message)
    }
  })

  it('localizes advanced input and verification method values at render time', () => {
    const wrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'computer_use',
          status: 'success',
          message: 'Host action completed',
          details: [
            { label: 'input_method', value: 'set_value' },
            { label: 'verification_method', value: 'ax_value' },
            { label: 'input_method', value: 'semantic_action' },
            { label: 'verification_method', value: 'semantic_action' },
            { label: 'verification_method', value: 'point_click' },
            { label: 'verification_method', value: 'ax_action' },
          ],
        },
      },
      global: {
        plugins: [
          createI18n({
            legacy: false,
            locale: 'zh-CN',
            fallbackLocale: 'en-US',
            messages: {
              'en-US': mergeRuntimeHarnessLocale('en-US', {
                common: {
                  yes: 'Yes',
                  no: 'No',
                },
                resultCard: {
                  labels: {},
                  messages: {},
                  values: {},
                },
              }),
              'zh-CN': mergeRuntimeHarnessLocale('zh-CN', {
                common: {
                  yes: '是',
                  no: '否',
                },
                resultCard: {
                  labels: {},
                  messages: {},
                  values: {},
                },
              }),
            },
          }),
        ],
      },
    })

    expect(wrapper.text()).toContain('设置值')
    expect(wrapper.text()).toContain('AX 值')
    expect(wrapper.text()).toContain('语义操作')
    expect(wrapper.text()).toContain('点按点击')
    expect(wrapper.text()).toContain('AX 操作')

    expect(wrapper.text()).not.toContain('set_value')
    expect(wrapper.text()).not.toContain('ax_value')
    expect(wrapper.text()).not.toContain('semantic_action')
    expect(wrapper.text()).not.toContain('point_click')
    expect(wrapper.text()).not.toContain('ax_action')
  })

  it('localizes intent values and windows host OS at render time', () => {
    const wrapper = mount(CardResult, {
      props: {
        card: {
          type: 'result',
          title: 'computer_use',
          status: 'success',
          message: 'Host action completed',
          details: [
            { label: 'host_os', value: 'windows' },
            { label: 'intent', value: 'message' },
            { label: 'intent', value: 'select' },
            { label: 'intent', value: 'toggle' },
            { label: 'intent', value: 'type' },
          ],
        },
      },
      global: {
        plugins: [
          createI18n({
            legacy: false,
            locale: 'zh-CN',
            fallbackLocale: 'en-US',
            messages: {
              'en-US': mergeRuntimeHarnessLocale('en-US', {
                common: {
                  yes: 'Yes',
                  no: 'No',
                },
                resultCard: {
                  labels: {},
                  messages: {},
                  values: {},
                },
              }),
              'zh-CN': mergeRuntimeHarnessLocale('zh-CN', {
                common: {
                  yes: '是',
                  no: '否',
                },
                resultCard: {
                  labels: {},
                  messages: {},
                  values: {},
                },
              }),
            },
          }),
        ],
      },
    })

    expect(wrapper.text()).toContain('主机系统')
    expect(wrapper.text()).toContain('Windows')
    expect(wrapper.text()).toContain('意图')
    expect(wrapper.text()).toContain('消息')
    expect(wrapper.text()).toContain('选择')
    expect(wrapper.text()).toContain('切换')
    expect(wrapper.text()).toContain('输入')

    expect(wrapper.text()).not.toContain('windows')
    expect(wrapper.text()).not.toContain('message')
    expect(wrapper.text()).not.toContain('select')
    expect(wrapper.text()).not.toContain('toggle')
    expect(wrapper.text()).not.toContain('type')
  })

  it('localizes thinking accordion header', () => {
    const wrapper = mount(CardAccordion, {
      props: {
        card: {
          type: 'accordion',
          id: 'thinking-1',
          items: [{ content: 'step' }],
        },
      },
      global: {
        plugins: [createPinia(), createTestI18n()],
      },
    })

    expect(wrapper.text()).toContain('思考中')
    expect(wrapper.text()).not.toContain('Thinking')
  })

  it('localizes fullscreen exit hint with key slot', async () => {
    isFullscreen.value = true
    fullscreenContent.value = {
      type: 'code',
      content: 'const x = 1',
    }

    const wrapper = mount(FullscreenModal, {
      attachTo: document.body,
      global: {
        plugins: [createTestI18n()],
      },
    })

    await nextTick()

    expect(document.body.textContent || '').toContain('按')
    expect(document.body.textContent || '').toContain('Esc')
    expect(document.body.textContent || '').toContain('退出全屏')

    wrapper.unmount()
  })
})
