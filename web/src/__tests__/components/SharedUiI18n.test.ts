import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

import AutoReplyRuleCard from '@/components/AutoReplyRuleCard.vue'
import PluginConfigForm from '@/components/PluginConfigForm.vue'
import ResourceChart from '@/components/ResourceChart.vue'

function createTestI18n(locale = 'zh-CN') {
  return createI18n({
    legacy: false,
    locale,
    fallbackLocale: 'en-US',
    messages: {
      'en-US': {
        common: {
          selectOption: 'Select an option',
          addItem: 'Add item',
          saving: 'Saving...',
          cancel: 'Cancel',
          noResponses: 'No responses',
          edit: 'Edit',
          test: 'Test',
          delete: 'Delete',
        },
        plugins: {
          configForm: {
            save: 'Save Configuration',
            empty: 'This plugin has no configurable options.',
          },
        },
        metrics: {
          noData: 'No data available',
          min: 'Min',
          max: 'Max',
        },
        resourceChart: {
          avg: 'Avg',
        },
        autoReply: {
          keyword: 'Keyword',
          regex: 'Regex',
          contains: 'Contains',
          prefix: 'Prefix',
          suffix: 'Suffix',
          responses: 'Responses',
          card: {
            trigger: 'Trigger',
            channels: 'Channels',
            priority: 'Priority: {priority}',
            more: '+{count} more',
            matchedTimes: 'Matched {count} times',
            updatedAt: 'Updated {date}',
          },
        },
      },
      'zh-CN': {
        common: {
          selectOption: '选择一个选项',
          addItem: '添加项目',
          saving: '保存中...',
          cancel: '取消',
          noResponses: '无响应',
          edit: '编辑',
          test: '测试',
          delete: '删除',
        },
        plugins: {
          configForm: {
            save: '保存配置',
            empty: '此插件没有可配置选项。',
          },
        },
        metrics: {
          noData: '暂无数据',
          min: '最小',
          max: '最大',
        },
        resourceChart: {
          avg: '平均',
        },
        autoReply: {
          keyword: '关键词',
          regex: '正则',
          contains: '包含',
          prefix: '前缀',
          suffix: '后缀',
          responses: '回复',
          card: {
            trigger: '触发器',
            channels: '渠道',
            priority: '优先级：{priority}',
            more: '+{count} 更多',
            matchedTimes: '匹配 {count} 次',
            updatedAt: '更新于 {date}',
          },
        },
      },
    },
  })
}

describe('Shared UI i18n', () => {
  it('localizes plugin config form labels and empty state', async () => {
    const wrapper = mount(PluginConfigForm, {
      props: {
        plugin: {
          id: 'p1',
          name: 'Plugin',
          version: '1.0.0',
          description: 'Plugin desc',
          type: 'native',
          status: 'loaded',
          enabled: true,
          config_schema: {
            type: 'object',
            properties: {
              mode: { type: 'string', enum: ['fast', 'safe'], title: 'Mode' },
              tags: { type: 'array', title: 'Tags' },
            },
            required: [],
          },
        },
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    expect(wrapper.text()).toContain('选择一个选项')
    expect(wrapper.text()).toContain('添加项目')

    const emptyWrapper = mount(PluginConfigForm, {
      props: {
        plugin: {
          id: 'p2',
          name: 'Empty',
          version: '1.0.0',
          description: 'Plugin desc',
          type: 'native',
          status: 'loaded',
          enabled: true,
        },
        loading: true,
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    expect(emptyWrapper.text()).toContain('此插件没有可配置选项。')
    expect(emptyWrapper.text()).toContain('保存中...')
    expect(emptyWrapper.text()).toContain('取消')
  })

  it('localizes resource chart empty and stat labels', () => {
    const emptyWrapper = mount(ResourceChart, {
      props: {
        title: 'CPU',
        data: [],
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    expect(emptyWrapper.text()).toContain('暂无数据')

    const wrapper = mount(ResourceChart, {
      props: {
        title: 'CPU',
        data: [
          { timestamp: '2026-03-09T00:00:00Z', value: 2 },
          { timestamp: '2026-03-09T00:01:00Z', value: 4 },
        ],
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    expect(wrapper.text()).toContain('最小')
    expect(wrapper.text()).toContain('平均')
    expect(wrapper.text()).toContain('最大')
  })

  it('localizes auto-reply rule card labels and actions', () => {
    const wrapper = mount(AutoReplyRuleCard, {
      props: {
        rule: {
          id: 'r1',
          name: 'Greeting',
          trigger_type: 'keyword',
          trigger_value: 'hello',
          responses: ['hi', 'hey'],
          priority: 5,
          enabled: true,
          channels: ['slack'],
          created_at: '2026-03-08T00:00:00Z',
          updated_at: '2026-03-09T00:00:00Z',
          match_count: 12,
        },
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    expect(wrapper.text()).toContain('关键词')
    expect(wrapper.text()).toContain('优先级：5')
    expect(wrapper.text()).toContain('触发器')
    expect(wrapper.text()).toContain('回复')
    expect(wrapper.text()).toContain('+1 更多')
    expect(wrapper.text()).toContain('渠道')
    expect(wrapper.text()).toContain('匹配 12 次')
    expect(wrapper.text()).toContain('更新于')
    expect(wrapper.text()).toContain('编辑')
    expect(wrapper.text()).toContain('测试')
  })
})
