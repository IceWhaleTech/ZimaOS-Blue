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

import ExecApprovalDialog from '@/components/ExecApprovalDialog.vue'

function createTestI18n() {
  return createI18n({
    legacy: false,
    locale: 'en-US',
    fallbackLocale: 'en-US',
    messages: {
      'en-US': {
        execApproval: {
          title: 'Directory Access Request',
          subtitle: 'A command wants to access a directory outside the allowed list',
          commandTitle: 'Command Approval Request',
          commandSubtitle: 'This command needs approval before it can run',
          directory: 'Directory',
          workdir: 'Working Directory',
          command: 'Command',
          deny: 'Deny',
          allowOnce: 'Allow Once',
          allowAlways: 'Always Allow',
          allowAlwaysCommand: 'Allow Exact Command',
          commandHint:
            'This does not add the directory to the allowlist. It only skips future prompts for the same command while Blue is running.',
        },
        common: {
          processing: 'Processing...',
        },
      },
    },
  })
}

function mountDialog() {
  return mount(ExecApprovalDialog, {
    global: {
      plugins: [createTestI18n()],
      stubs: {
        Teleport: true,
        Transition: false,
      },
    },
  })
}

describe('ExecApprovalDialog', () => {
  beforeEach(() => {
    taskProjectionStoreMock.refreshNow.mockReset().mockResolvedValue(undefined)
    mockChatStore = reactive({
      pendingExecApproval: null,
      resolveExecApproval: vi.fn().mockResolvedValue(true),
      dismissExecApproval: vi.fn(),
    })
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('renders command approvals with command-specific copy instead of directory copy', async () => {
    const wrapper = mountDialog()

    mockChatStore.pendingExecApproval = {
      id: 'exec-1',
      type: 'command',
      command: 'rm -rf /tmp/demo',
      workdir: '/Users/orca/.zimaos-blue/data/workspace',
      purpose: 'Allow running the requested command',
      risk_summary: 'High risk: this execution may write files, access the network, or launch subprocesses.',
      scope_summary: 'Working directory: /Users/orca/.zimaos-blue/data/workspace',
      expected_effects: 'May create, modify, or overwrite files.',
      affected_targets: [
        '/Users/orca/.zimaos-blue/data/workspace',
        '/Users/orca/.zimaos-blue/data/workspace/config.yaml',
      ],
      expires_at: Date.now() + 60_000,
      session_id: 'conv-1',
    }
    await nextTick()

    expect(wrapper.text()).toContain('Command Approval Request')
    expect(wrapper.text()).toContain('This command needs approval before it can run')
    expect(wrapper.text()).toContain('Working Directory')
    expect(wrapper.text()).toContain('/Users/orca/.zimaos-blue/data/workspace')
    expect(wrapper.text()).toContain('Allow Exact Command')
    expect(wrapper.text()).toContain('does not add the directory to the allowlist')
    expect(wrapper.text()).toContain('Allow running the requested command')
    expect(wrapper.text()).toContain('High risk')
    expect(wrapper.text()).toContain('May create, modify, or overwrite files')
    expect(wrapper.text()).not.toContain('Directory Access Request')

    wrapper.unmount()
  })
})
