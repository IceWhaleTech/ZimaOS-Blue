import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import AutomationTabs from '@/components/automation/AutomationTabs.vue'

const routeMock = {
  path: '/operations/evolution',
}

vi.mock('vue-router', async () => {
  const actual = await vi.importActual<typeof import('vue-router')>('vue-router')
  return {
    ...actual,
    useRoute: () => routeMock,
  }
})

function createTestI18n() {
  return createI18n({
    legacy: false,
    locale: 'en-US',
    fallbackLocale: 'en-US',
    missingWarn: false,
    fallbackWarn: false,
    messages: {
      'en-US': {
        nav: {
          automation: 'Operations',
        },
        automation: {
          tabs: {
            cron: 'Scheduled Tasks',
            cronDesc: 'Manage cron jobs and scheduled executions',
            harness: 'Harness',
            harnessDesc: 'Review run records, eval groups, and scoring results.',
            evolution: 'Evolution',
            evolutionDesc: 'Review skill, runner, and instruction evolution evidence.',
          },
        },
      },
    },
  })
}

describe('AutomationTabs', () => {
  beforeEach(() => {
    routeMock.path = '/operations/evolution'
  })

  it('shows only the three top-level operations tabs and marks evolution active on evolution routes', () => {
    const wrapper = mount(AutomationTabs, {
      global: {
        plugins: [createTestI18n()],
        stubs: {
          RouterLink: {
            props: ['to'],
            template: '<a :data-to="JSON.stringify(to)"><slot /></a>',
          },
        },
      },
    })

    expect(wrapper.get('[data-testid="automation-tab-evolution"]').text()).toContain('Evolution')
    expect(wrapper.get('[data-testid="automation-tab-evolution"]').attributes('data-to')).toBe(
      JSON.stringify('/operations/evolution')
    )
    expect(wrapper.get('[data-testid="automation-tab-evolution"]').attributes('aria-current')).toBe(
      'page'
    )
    expect(wrapper.get('[data-testid="automation-tab-evolution"]').text()).toContain('Beta')
    expect(wrapper.find('[data-testid="automation-tab-knowledge"]').exists()).toBe(false)
    expect(wrapper.findAll('[data-testid^="automation-tab-"]')).toHaveLength(3)
  })
})
