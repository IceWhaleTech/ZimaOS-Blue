import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import PreviewOnboardingModal from '@/components/onboarding/PreviewOnboardingModal.vue'
import { i18n } from '@/i18n'

const { getOnboardingStatus, setOnboardingSeen } = vi.hoisted(() => ({
  getOnboardingStatus: vi.fn(),
  setOnboardingSeen: vi.fn(),
}))

vi.mock('@/api/preview', () => ({
  previewApi: {
    getOnboardingStatus,
    setOnboardingSeen,
  },
}))

function setViewport(width: number, height: number) {
  Object.defineProperty(window, 'innerWidth', {
    configurable: true,
    value: width,
  })
  Object.defineProperty(window, 'innerHeight', {
    configurable: true,
    value: height,
  })
}

function appendCreateAccountAnchor() {
  const anchor = document.createElement('button')
  anchor.setAttribute('data-onboarding-anchor', 'preview-create-account')
  anchor.getBoundingClientRect = () =>
    ({
      x: 96,
      y: 740,
      top: 740,
      right: 276,
      bottom: 784,
      left: 96,
      width: 180,
      height: 44,
      toJSON: () => ({}),
    }) as DOMRect
  document.body.appendChild(anchor)
  return anchor
}

describe('PreviewOnboardingModal', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
    setViewport(1280, 900)
    getOnboardingStatus.mockResolvedValue({ data: { seen: false } })
    setOnboardingSeen.mockResolvedValue({ data: { success: true } })
  })

  afterEach(() => {
    document.body.innerHTML = ''
    vi.clearAllMocks()
  })

  it('positions the tooltip above the visible create-account button and centers the arrow', async () => {
    appendCreateAccountAnchor()

    const wrapper = mount(PreviewOnboardingModal, {
      global: {
        plugins: [i18n],
      },
    })

    await flushPromises()
    await wrapper.vm.$nextTick()

    const tooltip = document.body.querySelector(
      '[data-testid="preview-onboarding-tooltip"]'
    ) as HTMLElement | null
    const arrow = document.body.querySelector(
      '[data-testid="preview-onboarding-tooltip-arrow"]'
    ) as HTMLElement | null

    expect(tooltip).not.toBeNull()
    expect(tooltip?.style.left).toBe('26px')
    expect(tooltip?.style.top).toBe('476px')
    expect(arrow).not.toBeNull()
    expect(arrow?.style.left).toBe('152px')
    expect(arrow?.className).toContain('-bottom-2')

    wrapper.unmount()
  })
})
