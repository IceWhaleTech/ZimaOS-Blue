import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import TaskActionDialog from '@/components/TaskActionDialog.vue'

function createTestI18n() {
  return createI18n({
    legacy: false,
    locale: 'en-US',
    fallbackLocale: 'en-US',
    messages: {
      'en-US': {
        common: {
          cancel: 'Cancel',
          confirm: 'Confirm',
        },
        chat: {
          taskActionDialog: {
            eyebrow: 'Task input',
            title: 'Task action',
            description: 'Provide an optional decision and JSON payload for this action.',
            decision: 'Decision',
            decisionPlaceholder: 'approve',
            payload: 'Payload',
            invalidJson: 'Payload must be valid JSON.',
            invalidObject: 'Payload must be a JSON object.',
            submitting: 'Submitting...',
          },
        },
      },
    },
  })
}

describe('TaskActionDialog', () => {
  it('emits a structured payload after parsing decision and JSON object input', async () => {
    const wrapper = mount(TaskActionDialog, {
      props: {
        open: true,
        action: { label: 'Resume workflow' },
      },
      global: {
        plugins: [createTestI18n()],
        stubs: {
          Teleport: true,
          Transition: true,
        },
      },
    })

    expect(wrapper.text()).toContain('Resume workflow')

    await wrapper.get('[data-testid="task-action-decision-input"]').setValue('approve')
    await wrapper.get('[data-testid="task-action-payload-input"]').setValue('{"ticket":"A-9"}')
    await wrapper.get('[data-testid="task-action-confirm"]').trigger('click')

    expect(wrapper.emitted('confirm')?.[0]).toEqual([
      {
        decision: 'approve',
        payload: { ticket: 'A-9' },
      },
    ])
  })

  it('shows a validation error instead of emitting when payload JSON is invalid', async () => {
    const wrapper = mount(TaskActionDialog, {
      props: {
        open: true,
        action: { label: 'Resume workflow' },
      },
      global: {
        plugins: [createTestI18n()],
        stubs: {
          Teleport: true,
          Transition: true,
        },
      },
    })

    await wrapper.get('[data-testid="task-action-payload-input"]').setValue('{')
    await wrapper.get('[data-testid="task-action-confirm"]').trigger('click')

    expect(wrapper.emitted('confirm')).toBeUndefined()
    expect(wrapper.text()).toContain('Payload must be valid JSON.')
  })

  it('emits close on Escape and confirm on Ctrl+Enter', async () => {
    const focusSpy = vi
      .spyOn(HTMLInputElement.prototype, 'focus')
      .mockImplementation(() => undefined)
    const selectSpy = vi
      .spyOn(HTMLInputElement.prototype, 'select')
      .mockImplementation(() => undefined)

    const wrapper = mount(TaskActionDialog, {
      props: {
        open: true,
        action: { label: 'Resume workflow' },
      },
      attachTo: document.body,
      global: {
        plugins: [createTestI18n()],
        stubs: {
          Teleport: true,
          Transition: true,
        },
      },
    })

    await wrapper.vm.$nextTick()
    await wrapper.vm.$nextTick()

    const decisionInput = wrapper.get('[data-testid="task-action-decision-input"]')
    expect(focusSpy).toHaveBeenCalled()
    expect(selectSpy).toHaveBeenCalled()

    await decisionInput.trigger('keydown', { key: 'Escape' })
    expect(wrapper.emitted('close')?.length).toBe(1)

    await wrapper.get('[data-testid="task-action-payload-input"]').setValue('{"ticket":"A-9"}')
    await wrapper.get('[data-testid="task-action-payload-input"]').trigger('keydown', {
      key: 'Enter',
      ctrlKey: true,
    })

    expect(wrapper.emitted('confirm')?.[0]).toEqual([
      {
        payload: { ticket: 'A-9' },
      },
    ])

    wrapper.unmount()
    focusSpy.mockRestore()
    selectSpy.mockRestore()
  })

  it('applies suggested decisions from action input hints and uses custom placeholders', async () => {
    const wrapper = mount(TaskActionDialog, {
      props: {
        open: true,
        action: {
          label: 'Resume workflow',
          input: {
            fields: [
              {
                key: 'decision',
                label: 'Decision',
                kind: 'choice',
                target: 'decision',
                required: true,
                placeholder: 'approve',
                options: ['approve', 'reject'],
              },
              {
                key: 'comment',
                label: 'Comment',
                kind: 'textarea',
                target: 'payload',
                payload_key: 'comment',
                placeholder: 'Optional comment',
              },
            ],
            title: 'Review approval',
            description: 'Review the workflow checkpoint and choose whether to continue.',
            submit_label: 'Submit decision',
          },
        },
      },
      global: {
        plugins: [createTestI18n()],
        stubs: {
          Teleport: true,
          Transition: true,
        },
      },
    })

    expect(wrapper.text()).toContain('Review approval')
    expect(wrapper.text()).toContain(
      'Review the workflow checkpoint and choose whether to continue.'
    )
    expect(wrapper.text()).toContain('Decision')
    expect(wrapper.text()).toContain('Comment')
    expect(
      wrapper.get('[data-testid="task-action-decision-input"]').attributes('placeholder')
    ).toBe('approve')
    expect(wrapper.get('[data-testid="task-action-payload-input"]').attributes('placeholder')).toBe(
      'Optional comment'
    )

    const approveButton = wrapper.findAll('button').find((button) => button.text() === 'approve')
    expect(approveButton?.exists()).toBe(true)

    await approveButton!.trigger('click')
    expect(
      (wrapper.get('[data-testid="task-action-decision-input"]').element as HTMLInputElement).value
    ).toBe('approve')

    await wrapper.get('[data-testid="task-action-payload-input"]').setValue('Approved after review')
    await wrapper.get('[data-testid="task-action-confirm"]').trigger('click')
    expect(wrapper.text()).toContain('Submit decision')

    expect(wrapper.emitted('confirm')?.[0]).toEqual([
      {
        decision: 'approve',
        payload: { comment: 'Approved after review' },
      },
    ])
  })

  it('supports payload-only text input contracts without rendering a decision field', async () => {
    const wrapper = mount(TaskActionDialog, {
      props: {
        open: true,
        action: {
          label: 'Provide clarification',
          input: {
            fields: [
              {
                key: 'response',
                label: 'Response',
                kind: 'textarea',
                target: 'payload',
                payload_key: 'response',
                required: true,
                placeholder: 'Provide the missing detail',
              },
            ],
            title: 'Provide clarification',
            submit_label: 'Send response',
          },
        },
      },
      global: {
        plugins: [createTestI18n()],
        stubs: {
          Teleport: true,
          Transition: true,
        },
      },
    })

    expect(wrapper.find('[data-testid="task-action-decision-input"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('Provide clarification')
    expect(wrapper.text()).toContain('Response')
    expect(wrapper.text()).toContain('Send response')
    expect(wrapper.text()).toContain('Provide the required response for this action.')

    await wrapper.get('[data-testid="task-action-confirm"]').trigger('click')
    expect(wrapper.text()).toContain('Response is required.')

    await wrapper.get('[data-testid="task-action-payload-input"]').setValue('Need admin access')
    await wrapper.get('[data-testid="task-action-confirm"]').trigger('click')

    expect(wrapper.emitted('confirm')?.[0]).toEqual([
      {
        payload: { response: 'Need admin access' },
      },
    ])
  })

  it('supports root-body schema fields for send_update style actions', async () => {
    const wrapper = mount(TaskActionDialog, {
      props: {
        open: true,
        action: {
          label: 'Send update',
          input: {
            fields: [
              {
                key: 'message',
                label: 'Message',
                kind: 'textarea',
                target: 'root',
                required: true,
                placeholder: 'Send an update to this task',
              },
            ],
            title: 'Send update',
            description: 'Send a follow-up instruction or clarification to the running task.',
            submit_label: 'Send update',
          },
        },
      },
      global: {
        plugins: [createTestI18n()],
        stubs: {
          Teleport: true,
          Transition: true,
        },
      },
    })

    await wrapper.get('[data-testid="task-action-confirm"]').trigger('click')
    expect(wrapper.text()).toContain('Message is required.')

    await wrapper
      .get('[data-testid="task-action-field-message"]')
      .setValue('Keep the final answer concise')
    await wrapper.get('[data-testid="task-action-confirm"]').trigger('click')

    expect(wrapper.emitted('confirm')?.[0]).toEqual([{ message: 'Keep the final answer concise' }])
  })

  it('falls back to legacy input hints when schema fields are absent', async () => {
    const wrapper = mount(TaskActionDialog, {
      props: {
        open: true,
        action: {
          label: 'Resume workflow',
          input: {
            title: 'Provide clarification',
            submit_label: 'Send response',
            mode: 'payload',
            payload_label: 'Response',
            payload_required: true,
            payload_format: 'text',
            payload_text_key: 'response',
            payload_placeholder: 'Provide the missing detail',
          },
        },
      },
      global: {
        plugins: [createTestI18n()],
        stubs: {
          Teleport: true,
          Transition: true,
        },
      },
    })

    expect(wrapper.find('[data-testid="task-action-decision-input"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="task-action-payload-input"]').attributes('placeholder')).toBe(
      'Provide the missing detail'
    )

    await wrapper.get('[data-testid="task-action-confirm"]').trigger('click')
    expect(wrapper.text()).toContain('Response is required.')

    await wrapper.get('[data-testid="task-action-payload-input"]').setValue('Need admin access')
    await wrapper.get('[data-testid="task-action-confirm"]').trigger('click')

    expect(wrapper.emitted('confirm')?.[0]).toEqual([
      {
        payload: { response: 'Need admin access' },
      },
    ])
  })
})
