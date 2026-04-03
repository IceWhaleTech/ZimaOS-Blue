import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/utils/authStorage', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/utils/authStorage')>()
  return {
    ...actual,
    getStoredAccessToken: () => '',
  }
})

import { useFormFillerWidget } from '@/composables/useFormFillerWidget'

function resetWidgetState() {
  const { state } = useFormFillerWidget()
  state.isVisible = false
  state.isMinimized = false
  state.position = { x: 0, y: 0 }
  state.focusedElement = null
  state.selectedTemplate = null
  state.templates = []
  state.config = null
  state.fillHistory = []
  state.isLoading = false
  state.error = null
  state.isInitialized = false
  state.clipboardData = ''
  state.parsedClipboardFields = {}
  state.parsedClipboardTokens = []
  state.revealedPasswordFields = new Set()
}

async function flushSetup() {
  await Promise.resolve()
  await Promise.resolve()
}

describe('useFormFillerWidget scope gating', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
    window.history.pushState({}, '', '/settings')
    resetWidgetState()
  })

  afterEach(() => {
    const { cleanup } = useFormFillerWidget()
    cleanup()
    document.body.innerHTML = ''
    resetWidgetState()
  })

  it('only opens for fields inside the allowed channel or provider scopes', async () => {
    document.body.innerHTML = `
      <div>
        <div class="outside-form">
          <input id="outside-name" name="name" />
          <input id="outside-secret" name="secret" />
        </div>
      </div>
      <div data-form-filler-scope="channel">
        <div class="channel-form">
          <input id="channel-name" name="name" />
          <input id="channel-secret" name="secret" />
        </div>
      </div>
    `

    const { setup, state } = useFormFillerWidget()
    setup()
    await flushSetup()

    const outsideInput = document.getElementById('outside-name') as HTMLInputElement
    outsideInput.dispatchEvent(new FocusEvent('focusin', { bubbles: true }))
    expect(state.isVisible).toBe(false)

    const channelInput = document.getElementById('channel-name') as HTMLInputElement
    channelInput.dispatchEvent(new FocusEvent('focusin', { bubbles: true }))

    expect(state.isVisible).toBe(true)
    expect(state.focusedElement).toBe(channelInput)
  })

  it('limits fill-all to the current scoped form container instead of the whole document', async () => {
    document.body.innerHTML = `
      <div data-form-filler-scope="provider">
        <section id="provider-form-a">
          <input id="provider-a-name" name="name" />
          <input id="provider-a-secret" name="secret" />
        </section>
        <section id="provider-form-b">
          <input id="provider-b-name" name="name" />
          <input id="provider-b-secret" name="secret" />
        </section>
      </div>
      <div>
        <input id="global-name" name="name" />
        <input id="global-secret" name="secret" />
      </div>
    `

    const { setup, state, setClipboardData, fillAllFields } = useFormFillerWidget()
    setup()
    await flushSetup()

    const providerAName = document.getElementById('provider-a-name') as HTMLInputElement
    const providerASecret = document.getElementById('provider-a-secret') as HTMLInputElement
    const providerBName = document.getElementById('provider-b-name') as HTMLInputElement
    const providerBSecret = document.getElementById('provider-b-secret') as HTMLInputElement
    const globalName = document.getElementById('global-name') as HTMLInputElement
    const globalSecret = document.getElementById('global-secret') as HTMLInputElement

    providerAName.dispatchEvent(new FocusEvent('focusin', { bubbles: true }))
    expect(state.focusedElement).toBe(providerAName)

    setClipboardData('Alpha\nBeta')
    expect(fillAllFields()).toBe(2)

    expect(providerAName.value).toBe('Alpha')
    expect(providerASecret.value).toBe('Beta')
    expect(providerBName.value).toBe('')
    expect(providerBSecret.value).toBe('')
    expect(globalName.value).toBe('')
    expect(globalSecret.value).toBe('')
  })

  it('only allows the keyboard shortcut to open the widget from an allowed scope', async () => {
    document.body.innerHTML = `
      <div>
        <div class="outside-form">
          <input id="outside-name" name="name" />
          <input id="outside-secret" name="secret" />
        </div>
      </div>
      <div data-form-filler-scope="provider">
        <div class="provider-form">
          <input id="provider-name" name="name" />
          <input id="provider-secret" name="secret" />
        </div>
      </div>
    `

    const { setup, state } = useFormFillerWidget()
    setup()
    await flushSetup()

    const outsideInput = document.getElementById('outside-name') as HTMLInputElement
    outsideInput.focus()
    state.isVisible = false
    state.focusedElement = null
    document.dispatchEvent(
      new KeyboardEvent('keydown', {
        key: 'F',
        ctrlKey: true,
        shiftKey: true,
        bubbles: true,
      })
    )
    expect(state.isVisible).toBe(false)

    const providerInput = document.getElementById('provider-name') as HTMLInputElement
    providerInput.focus()
    state.isVisible = false
    state.focusedElement = null
    document.dispatchEvent(
      new KeyboardEvent('keydown', {
        key: 'F',
        ctrlKey: true,
        shiftKey: true,
        bubbles: true,
      })
    )

    expect(state.isVisible).toBe(true)
    expect(state.focusedElement).toBe(providerInput)
  })
})
