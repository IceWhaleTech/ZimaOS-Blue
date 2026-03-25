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
          title: 'Command Execution',
          command: 'Command',
          output: 'Output',
          stdout: 'Output',
          stderr: 'Error Output',
          session: 'Session',
          riskLabel: 'Risk',
          outputUnavailable: 'Output unavailable in this card',
          noOutput: 'No output',
          collapse: 'Show less',
          expand: 'Show more',
          commandHidden: 'Command hidden',
          hideCommand: 'Hide command',
          showCommand: 'Show command',
          builtin: 'Built-in',
          sandbox: 'Sandbox',
          local: 'Local',
          duration: 'Duration',
          exitCode: 'exit',
          lines: 'lines',
          copyCommand: 'Copy command',
          copied: 'Copied!',
          outputTruncated: 'truncated',
          risk: {
            low: 'Low',
            medium: 'Medium',
            high: 'High',
            critical: 'Critical',
          },
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

  it('renders command summary chips and separates stderr from stdout', () => {
    const wrapper = mount(CardExec, {
      props: {
        card: {
          type: 'exec',
          status: 'error',
          host: 'sandbox',
          command: 'npm run build',
          exit_code: 1,
          duration_ms: 1532,
          session_id: 'sess-42',
          risk_level: 'high',
          stdout: 'building app\nfinished step 1',
          stderr: 'Type error in src/main.ts',
          truncated: true,
          warning: 'Build exited with code 1',
        },
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    expect(wrapper.text()).toContain('Command Execution')
    expect(wrapper.text()).toContain('Sandbox')
    expect(wrapper.text()).toContain('Command')
    expect(wrapper.text()).toContain('$ npm run build')
    expect(wrapper.text()).toContain('Duration')
    expect(wrapper.text()).toContain('1.5 s')
    expect(wrapper.text()).toContain('exit')
    expect(wrapper.text()).toContain('1')
    expect(wrapper.text()).toContain('Session')
    expect(wrapper.text()).toContain('sess-42')
    expect(wrapper.text()).toContain('Risk')
    expect(wrapper.text()).toContain('High')
    expect(wrapper.text()).toContain('Error Output')
    expect(wrapper.text()).toContain('Type error in src/main.ts')
    expect(wrapper.text()).toContain('Output')
    expect(wrapper.text()).toContain('building app')
    expect(wrapper.text()).toContain('truncated')
    expect(wrapper.text()).toContain('Build exited with code 1')
  })

  it('toggles the command visibility without losing the command content', async () => {
    const wrapper = mount(CardExec, {
      props: {
        card: {
          type: 'exec',
          status: 'success',
          command: 'ls -la',
          stdout: 'total 0',
        },
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    const hideButton = wrapper
      .findAll('button')
      .find((button) => button.text().includes('Hide command'))
    expect(hideButton).toBeTruthy()

    await hideButton!.trigger('click')
    expect(wrapper.text()).toContain('Command hidden')

    const showButton = wrapper
      .findAll('button')
      .find((button) => button.text().includes('Show command'))
    expect(showButton).toBeTruthy()

    await showButton!.trigger('click')
    expect(wrapper.text()).toContain('$ ls -la')
  })
})
