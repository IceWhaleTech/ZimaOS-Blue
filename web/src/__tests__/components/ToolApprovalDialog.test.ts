import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { nextTick, reactive } from 'vue'

let mockChatStore: any
const taskProjectionStoreMock = {
  refreshNow: vi.fn(),
}

vi.mock('@/stores/chat', () => ({
  useChatStore: () => mockChatStore,
}))

vi.mock('@/stores/taskProjections', () => ({
  useTaskProjectionsStore: () => taskProjectionStoreMock,
}))

import ToolApprovalDialog from '@/components/ToolApprovalDialog.vue'

function createTestI18n() {
  return createI18n({
    legacy: false,
    locale: 'en-US',
    fallbackLocale: 'en-US',
    messages: {
      'en-US': {
        approval: {
          title: 'Tool Call Approval',
          subtitle: 'A tool is requesting permission to execute',
          tool: 'Tool',
          arguments: 'Arguments',
          deny: 'Deny',
          allow: 'Allow',
          alwaysAllow: 'Always Allow',
        },
        common: {
          processing: 'Processing...',
        },
        tools: {
          names: {
            file_write: 'File Write',
          },
        },
      },
    },
  })
}

function createPendingApproval() {
  return {
    request_id: 'approval-1',
    tool_name: 'file_write',
    session_id: 'conv-1',
    binding_hash: 'binding-tool-1',
    purpose: 'Allow tool to write a local file',
    risk_summary: 'High risk: this tool call may modify data or access sensitive resources.',
    scope_summary: 'Targets: /tmp/approved.txt',
    expected_effects: 'May create new files or modify existing file content.',
    affected_targets: ['/tmp/approved.txt'],
    arguments: {
      path: 'approved.txt',
      content: 'hello after approval',
    },
  }
}

function flushPromises() {
  return new Promise((resolve) => setTimeout(resolve, 0))
}

function mountDialog() {
  return mount(ToolApprovalDialog, {
    global: {
      plugins: [createTestI18n()],
      stubs: {
        Teleport: true,
        Transition: false,
      },
    },
  })
}

describe('ToolApprovalDialog', () => {
  beforeEach(() => {
    taskProjectionStoreMock.refreshNow.mockReset().mockResolvedValue(undefined)
    mockChatStore = reactive({
      pendingApproval: null,
      resolveApproval: vi.fn().mockResolvedValue(true),
    })
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('renders the pending tool approval with translated tool name and arguments', async () => {
    const wrapper = mountDialog()
    mockChatStore.pendingApproval = createPendingApproval()
    await nextTick()

    expect(wrapper.text()).toContain('Tool Call Approval')
    expect(wrapper.text()).toContain('File Write')
    expect(wrapper.text()).toContain('approved.txt')
    expect(wrapper.text()).toContain('hello after approval')
    expect(wrapper.text()).toContain('Allow tool to write a local file')
    expect(wrapper.text()).toContain('High risk')
    expect(wrapper.text()).toContain('Targets: /tmp/approved.txt')

    mockChatStore.pendingApproval = null
    await nextTick()
    wrapper.unmount()
  })

  it('submits approve and always-allow decisions through the chat store', async () => {
    const wrapper = mountDialog()
    mockChatStore.pendingApproval = createPendingApproval()
    await nextTick()

    const buttons = wrapper.findAll('button')
    expect(buttons).toHaveLength(3)

    await buttons[1]!.trigger('click')
    await flushPromises()
    await nextTick()

    expect(mockChatStore.resolveApproval).toHaveBeenNthCalledWith(1, 'approve')
    expect(taskProjectionStoreMock.refreshNow).toHaveBeenCalledTimes(1)

    await buttons[2]!.trigger('click')
    await flushPromises()
    await nextTick()

    expect(mockChatStore.resolveApproval).toHaveBeenNthCalledWith(2, 'approve', true)
    expect(taskProjectionStoreMock.refreshNow).toHaveBeenCalledTimes(2)

    mockChatStore.pendingApproval = null
    await nextTick()
    wrapper.unmount()
  })
})
