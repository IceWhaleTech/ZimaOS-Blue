import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

import WorkspaceSettings from '@/components/settings/WorkspaceSettings.vue'
import { workspaceApi } from '@/api/workspace'

vi.mock('@/api/workspace', () => ({
  workspaceApi: {
    listFiles: vi.fn(),
    getStats: vi.fn(),
    putFile: vi.fn(),
  },
}))

function createTestI18n() {
  return createI18n({
    legacy: false,
    locale: 'zh-CN',
    fallbackLocale: 'en-US',
    messages: {
      'en-US': {
        common: {
          loading: 'Loading...',
          save: 'Save',
          cancel: 'Cancel',
          saving: 'Saving...',
        },
        workspace: {
          title: 'Workspace Files',
          description: 'These files define Blue.',
          noFiles: 'No workspace files found.',
          coreTokens: '~{count} core-file tokens',
          coreTokensHint: 'Counts only the core workspace files shown here.',
          chars: '{count} chars',
        },
      },
      'zh-CN': {
        common: {
          loading: '加载中...',
          save: '保存',
          cancel: '取消',
          saving: '保存中...',
        },
        workspace: {
          title: '工作区文件',
          description: '这些文件定义了 Blue 的性格、记忆和行为。',
          noFiles: '未找到工作区文件。',
          coreTokens: '核心文件约 {count} 个 token',
          coreTokensHint: '仅统计当前显示的核心工作区文件。',
          chars: '{count} 字符',
        },
      },
    },
  })
}

describe('WorkspaceSettings i18n', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(workspaceApi.listFiles).mockResolvedValue({ data: { files: [] } } as never)
    vi.mocked(workspaceApi.getStats).mockResolvedValue({
      data: {
        files: [],
        total_tokens: 321,
        total_bytes: 128,
      },
    } as never)
  })

  it('renders the localized core token count with interpolation', async () => {
    const wrapper = mount(WorkspaceSettings, {
      global: {
        plugins: [createTestI18n()],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('核心文件约 321 个 token')
    expect(wrapper.text()).not.toContain('核心文件约 个 token')
  })
})
