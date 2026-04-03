import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

import HarnessRunDetailDrawer from '@/components/harness/HarnessRunDetailDrawer.vue'

function createTestI18n() {
  return createI18n({
    legacy: false,
    locale: 'en-US',
    fallbackLocale: 'en-US',
    messages: {
      'en-US': {
        common: {
          cancel: 'Close',
          loading: 'Loading',
          notAvailable: 'Not available',
          updatedAt: 'Updated',
        },
        harness: {
          group: {
            runDetail: 'Run detail',
            selectRunPrompt: 'Select a run to inspect its timeline and artifacts.',
            runSummary: 'Run summary',
            noGoalSummary: 'No goal summary was recorded for this run.',
            model: 'Model',
            runtimeState: 'Runtime state',
            progress: 'Progress',
            overview: 'Overview',
            runtimeTrace: 'Runtime trace',
            noRuntimeTrace: 'No runtime trace snapshot is available for this run yet.',
            timeline: 'Timeline',
            noTimelineEvents: 'No timeline events recorded for this run.',
            artifacts: 'Artifacts',
            noArtifacts: 'No artifacts attached to the linked runs yet.',
            pending: 'Pending',
            noPendingItems: 'No pending approvals or questions for this run.',
            owner: 'Agent',
            kind: 'Kind',
            status: 'Status',
            depth: 'Depth',
            sandboxMode: 'Sandbox mode',
            approvalMode: 'Approval mode',
            workspaceRoot: 'Workspace root',
            contextPacks: 'Context Packs',
            contextPackSelectedCount: 'Selected',
            contextPackTotalTokens: 'Tokens',
            contextPackSelectedSkill: 'Skill',
            contextPackSelectionDigest: 'Selection digest',
            contextPackAnnotated: 'Annotated',
            contextPackTruncated: 'Truncated',
            contextPackLanguage: 'Language',
            contextPackVersion: 'Version',
            contextPackEntryId: 'Entry',
            contextPackSourceTrust: 'Trust',
            contextPackFile: 'File',
          },
        },
      },
    },
  })
}

describe('HarnessRunDetailDrawer', () => {
  it('renders context pack snapshot metadata when present', () => {
    const wrapper = mount(HarnessRunDetailDrawer, {
      props: {
        open: true,
        detail: {
          run: {
            id: 'run-1',
            root_run_id: 'run-1',
            kind: 'agent_task',
            status: 'completed',
            goal: 'Verify Responses API docs fix',
            progress: 100,
            metadata: {
              contextpack_snapshot: {
                selected_count: 1,
                total_tokens: 420,
                selected_skill: 'web_query',
                selection_digest: 'digest-123',
                files: [
                  {
                    entry_id: 'openai/docs/responses-api',
                    type: 'doc',
                    source_trust: 'official',
                    language: 'en-US',
                    version: 'latest',
                    file: 'references/tools.md',
                    sha256: 'abc123',
                    tokens: 420,
                    annotated: true,
                    truncated: false,
                  },
                ],
              },
            },
            created_at: '2026-04-03T10:00:00.000Z',
            updated_at: '2026-04-03T10:05:00.000Z',
          },
          events: [],
          artifacts: [],
          pending_approvals: [],
          pending_questions: [],
          run_trace: null,
        },
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    expect(wrapper.text()).toContain('Context Packs')
    expect(wrapper.text()).toContain('Selected: 1')
    expect(wrapper.text()).toContain('Tokens: 420')
    expect(wrapper.text()).toContain('Skill: web_query')
    expect(wrapper.text()).toContain('Selection digest: digest-123')
    expect(wrapper.text()).toContain('openai/docs/responses-api')
    expect(wrapper.text()).toContain('Trust: official')
    expect(wrapper.text()).toContain('Language: en-US')
    expect(wrapper.text()).toContain('Version: latest')
    expect(wrapper.text()).toContain('references/tools.md')
    expect(wrapper.text()).toContain('Annotated')
  })
})
