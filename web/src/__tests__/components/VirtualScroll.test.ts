import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import VirtualScroll from '@/components/VirtualScroll.vue'

vi.stubGlobal(
  'ResizeObserver',
  class {
    observe() {}
    unobserve() {}
    disconnect() {}
  }
)

async function flushRafChain() {
  await new Promise((resolve) => window.setTimeout(resolve, 0))
  await new Promise((resolve) => window.setTimeout(resolve, 0))
}

describe('VirtualScroll', () => {
  let rafSpy: ReturnType<typeof vi.spyOn>
  let cafSpy: ReturnType<typeof vi.spyOn>

  function mockRect(params: { top: number; height?: number }): DOMRect {
    const height = params.height ?? 200
    return {
      top: params.top,
      left: 0,
      right: 320,
      bottom: params.top + height,
      width: 320,
      height,
      x: 0,
      y: params.top,
      toJSON: () => ({}),
    } as DOMRect
  }

  function createExternalScrollSetup() {
    const scrollContainer = document.createElement('div')
    Object.defineProperty(scrollContainer, 'clientHeight', {
      configurable: true,
      value: 200,
    })
    Object.defineProperty(scrollContainer, 'scrollHeight', {
      configurable: true,
      value: 4000,
    })
    scrollContainer.getBoundingClientRect = () => mockRect({ top: 0 })

    const wrapper = mount(VirtualScroll, {
      props: {
        itemCount: 100,
        estimatedItemHeight: 100,
        overscan: 0,
        scrollContainer,
      },
      slots: {
        default: '<div style="height: 100px;">row</div>',
      },
    })

    ;(wrapper.element as HTMLElement).getBoundingClientRect = () =>
      mockRect({ top: -scrollContainer.scrollTop })

    return { scrollContainer, wrapper }
  }

  beforeEach(() => {
    rafSpy = vi
      .spyOn(window, 'requestAnimationFrame')
      .mockImplementation((cb: FrameRequestCallback) => window.setTimeout(() => cb(0), 0))
    cafSpy = vi.spyOn(window, 'cancelAnimationFrame').mockImplementation((id: number) => {
      window.clearTimeout(id)
    })
  })

  afterEach(() => {
    rafSpy.mockRestore()
    cafSpy.mockRestore()
  })

  it('reuses the provided outer scroll container instead of creating a nested scroll root', async () => {
    const scrollContainer = document.createElement('div')
    Object.defineProperty(scrollContainer, 'clientHeight', {
      configurable: true,
      value: 200,
    })
    Object.defineProperty(scrollContainer, 'scrollHeight', {
      configurable: true,
      value: 4000,
    })
    scrollContainer.getBoundingClientRect = () =>
      ({
        top: 100,
        left: 0,
        right: 320,
        bottom: 300,
        width: 320,
        height: 200,
        x: 0,
        y: 100,
        toJSON: () => ({}),
      }) as DOMRect

    const wrapper = mount(VirtualScroll, {
      props: {
        itemCount: 100,
        estimatedItemHeight: 20,
        overscan: 0,
        scrollContainer,
      },
      slots: {
        default: '<div style="height: 20px;">row</div>',
      },
    })

    ;(wrapper.element as HTMLElement).getBoundingClientRect = () =>
      ({
        top: 150 - scrollContainer.scrollTop,
        left: 0,
        right: 320,
        bottom: 350 - scrollContainer.scrollTop,
        width: 320,
        height: 200,
        x: 0,
        y: 150 - scrollContainer.scrollTop,
        toJSON: () => ({}),
      }) as DOMRect

    await wrapper.vm.$nextTick()
    await flushRafChain()

    expect((wrapper.vm as { getContainer: () => HTMLElement | null }).getContainer()).toBe(
      scrollContainer
    )

    scrollContainer.scrollTop = 250
    scrollContainer.dispatchEvent(new Event('scroll'))
    await wrapper.vm.$nextTick()
    await flushRafChain()

    const emittedRanges = wrapper.emitted('visibleRangeChange') ?? []
    const lastRange = emittedRanges.at(-1)

    expect(lastRange).toBeTruthy()
    expect(lastRange?.[0]).toBe(10)
    expect(lastRange?.[1]).toBe(21)
  })

  it('preserves scroll position when a remeasured item is fully above the viewport', async () => {
    const { scrollContainer, wrapper } = createExternalScrollSetup()

    await wrapper.vm.$nextTick()
    await flushRafChain()

    scrollContainer.scrollTop = 150
    scrollContainer.dispatchEvent(new Event('scroll'))
    await wrapper.vm.$nextTick()
    await flushRafChain()
    ;(
      wrapper.vm as unknown as { updateItemHeight: (index: number, height: number) => void }
    ).updateItemHeight(0, 200)
    await wrapper.vm.$nextTick()
    await flushRafChain()

    expect(scrollContainer.scrollTop).toBe(250)
  })

  it('does not shift scroll position when the resized item is still in view', async () => {
    const { scrollContainer, wrapper } = createExternalScrollSetup()

    await wrapper.vm.$nextTick()
    await flushRafChain()

    scrollContainer.scrollTop = 150
    scrollContainer.dispatchEvent(new Event('scroll'))
    await wrapper.vm.$nextTick()
    await flushRafChain()
    ;(
      wrapper.vm as unknown as { updateItemHeight: (index: number, height: number) => void }
    ).updateItemHeight(1, 200)
    await wrapper.vm.$nextTick()
    await flushRafChain()

    expect(scrollContainer.scrollTop).toBe(150)
  })

  it('keeps the current viewport anchored when older keyed items are prepended', async () => {
    const scrollContainer = document.createElement('div')
    Object.defineProperty(scrollContainer, 'clientHeight', {
      configurable: true,
      value: 200,
    })
    Object.defineProperty(scrollContainer, 'scrollHeight', {
      configurable: true,
      value: 4000,
    })
    scrollContainer.getBoundingClientRect = () => mockRect({ top: 0 })

    const initialItems = Array.from({ length: 100 }, (_, index) => ({ id: `row-${index}` }))
    const wrapper = mount(VirtualScroll, {
      props: {
        itemCount: initialItems.length,
        items: initialItems,
        itemKey: 'id',
        estimatedItemHeight: 100,
        overscan: 0,
        scrollContainer,
      },
      slots: {
        default: '<div style="height: 100px;">row</div>',
      },
    })

    ;(wrapper.element as HTMLElement).getBoundingClientRect = () =>
      mockRect({ top: -scrollContainer.scrollTop })

    await wrapper.vm.$nextTick()
    await flushRafChain()

    scrollContainer.scrollTop = 150
    scrollContainer.dispatchEvent(new Event('scroll'))
    await wrapper.vm.$nextTick()
    await flushRafChain()

    const nextItems = [
      { id: 'older-0' },
      { id: 'older-1' },
      ...initialItems,
    ]
    await wrapper.setProps({
      itemCount: nextItems.length,
      items: nextItems,
    })
    await wrapper.vm.$nextTick()
    await flushRafChain()

    expect(scrollContainer.scrollTop).toBe(350)
  })
})
