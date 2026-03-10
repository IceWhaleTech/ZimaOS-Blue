import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

import CardExec from '@/components/typeless/CardExec.vue'

function createTestI18n(locale = 'en-US') {
  return createI18n({
    legacy: false,
    locale,
    fallbackLocale: 'en-US',
    messages: {
      'en-US': {
        execCard: {
          outputUnavailable: 'Output unavailable in this card',
          noOutput: 'No output',
          collapse: 'Show less',
          expand: 'Show more',
          commandHidden: 'Command hidden',
          hideCommand: 'Hide command',
          showCommand: 'Show command',
          builtin: 'Built-in',
        },
        tools: {
          sandboxProtected: 'Sandbox protected',
        },
        toolWarnings: {
          warning: 'Warning',
        },
      },
    },
  })
}

describe('CardExec', () => {
  it('shows neutral fallback when output is omitted from the card', () => {
    const wrapper = mount(CardExec, {
      props: {
        card: {
          type: 'exec',
          status: 'error',
          stdout_redacted: true,
        },
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    expect(wrapper.text()).toContain('Output unavailable in this card')
  })

  it('shows normal no-output fallback without redaction markers', () => {
    const wrapper = mount(CardExec, {
      props: {
        card: {
          type: 'exec',
          status: 'success',
        },
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    expect(wrapper.text()).toContain('No output')
  })
})
