import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import DefaultLayout from '@/layouts/DefaultLayout.vue'

const {
  setupFormFillerWidget,
  cleanupFormFillerWidget,
  settingsStoreState,
  tauriState,
} = vi.hoisted(() => ({
  setupFormFillerWidget: vi.fn(),
  cleanupFormFillerWidget: vi.fn(),
  settingsStoreState: {
    closeBehavior: 'minimize',
  },
  tauriState: {
    isTauri: false,
    platform: 'unknown',
    setCloseBehavior: vi.fn(),
    startWindowDragging: vi.fn(),
    refreshTauriDetection: vi.fn(),
  },
}))

const helpers = vi.hoisted(() => ({
  asAsyncSFCModule(component: Record<string, unknown>) {
    return {
      __esModule: true,
      __isTeleport: false,
      __isKeepAlive: false,
      default: component,
      ...component,
    }
  },
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (_key: string, fallback?: string) => fallback || '',
    te: () => false,
  }),
}))

vi.mock('@/composables/useFormFillerWidget', () => ({
  useFormFillerWidget: () => ({
    setup: setupFormFillerWidget,
    cleanup: cleanupFormFillerWidget,
  }),
}))

vi.mock('@/composables/useTauri', () => ({
  useTauri: () => ({
    isTauri: { value: tauriState.isTauri },
    platform: { value: tauriState.platform },
    setCloseBehavior: tauriState.setCloseBehavior,
    startWindowDragging: tauriState.startWindowDragging,
  }),
  refreshTauriDetection: tauriState.refreshTauriDetection,
}))

vi.mock('@/stores/settings', () => ({
  useSettingsStore: () => settingsStoreState,
}))

vi.mock('@/components/AppSidebar.vue', () => ({
  ...helpers.asAsyncSFCModule({
    name: 'AppSidebar',
    template: '<aside class="app-sidebar-stub" />',
  }),
}))

vi.mock('@/components/formfiller/FormFillerWidget.vue', () => ({
  ...helpers.asAsyncSFCModule({
    name: 'FormFillerWidget',
    template: '<div class="form-filler-widget-stub" />',
  }),
}))

vi.mock('@/components/BrowserMonitorWidget.vue', () => ({
  ...helpers.asAsyncSFCModule({
    name: 'BrowserMonitorWidget',
    template: '<div class="browser-monitor-widget-stub" />',
  }),
}))

vi.mock('@/components/typeless/FullscreenModal.vue', () => ({
  ...helpers.asAsyncSFCModule({
    name: 'FullscreenModal',
    template: '<div class="fullscreen-modal-stub" />',
  }),
}))

vi.mock('@/components/voicewake/GlobalVoiceWakeBanner.vue', () => ({
  ...helpers.asAsyncSFCModule({
    name: 'GlobalVoiceWakeBanner',
    template: '<div class="global-voicewake-banner-stub" />',
  }),
}))

function createTestRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/home', name: 'Home', component: { template: '<div>Home</div>' } },
      { path: '/profile', name: 'Profile', component: { template: '<div>Profile</div>' } },
      { path: '/chat', name: 'Chat', component: { template: '<div>Chat</div>' } },
    ],
  })
}

function setUserAgent(userAgent: string, maxTouchPoints = 0) {
  Object.defineProperty(window.navigator, 'userAgent', {
    configurable: true,
    value: userAgent,
  })
  Object.defineProperty(window.navigator, 'maxTouchPoints', {
    configurable: true,
    value: maxTouchPoints,
  })
}

function setViewportWidth(width: number) {
  Object.defineProperty(window, 'innerWidth', {
    configurable: true,
    writable: true,
    value: width,
  })
}

async function mountLayout(path: string) {
  const router = createTestRouter()
  router.push(path)
  await router.isReady()

  const wrapper = mount(DefaultLayout, {
    global: {
      plugins: [router],
    },
  })

  await flushPromises()
  await vi.dynamicImportSettled()
  await flushPromises()

  return wrapper
}

describe('DefaultLayout', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => {
      callback(0)
      return 1
    })
    vi.stubGlobal('cancelAnimationFrame', vi.fn())
    settingsStoreState.closeBehavior = 'minimize'
    tauriState.isTauri = false
    tauriState.platform = 'unknown'
    setViewportWidth(1440)
    document.documentElement.removeAttribute('data-blue-window-resizing')
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('does not render the nav button when the sidebar stays visible on wide desktop screens', async () => {
    setUserAgent(
      'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36'
    )

    const wrapper = await mountLayout('/profile')

    expect(wrapper.find('.layout-mobile-nav-button').exists()).toBe(false)

    wrapper.unmount()
  })

  it('renders the nav button for desktop browsers when the window is too narrow to show the sidebar', async () => {
    setUserAgent(
      'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36'
    )
    setViewportWidth(900)

    const wrapper = await mountLayout('/profile')

    expect(wrapper.find('.layout-mobile-nav-button').exists()).toBe(true)

    wrapper.unmount()
  })

  it('enables a temporary resize performance mode while the window is resizing', async () => {
    vi.useFakeTimers()

    try {
      setUserAgent(
        'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36'
      )

      const wrapper = await mountLayout('/profile')

      window.dispatchEvent(new Event('resize'))

      expect(document.documentElement.getAttribute('data-blue-window-resizing')).toBe('true')

      vi.advanceTimersByTime(179)
      expect(document.documentElement.getAttribute('data-blue-window-resizing')).toBe('true')

      vi.advanceTimersByTime(1)
      expect(document.documentElement.hasAttribute('data-blue-window-resizing')).toBe(false)

      wrapper.unmount()
    } finally {
      vi.useRealTimers()
    }
  })

  it('renders the mobile nav button for phone browsers on profile routes', async () => {
    setUserAgent(
      'Mozilla/5.0 (iPhone; CPU iPhone OS 18_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.3 Mobile/15E148 Safari/604.1',
      5
    )
    setViewportWidth(390)

    const wrapper = await mountLayout('/profile')

    expect(wrapper.find('.layout-mobile-nav-button').exists()).toBe(true)

    wrapper.unmount()
  })

  it('renders the mobile nav button for phone browsers on the dashboard route', async () => {
    setUserAgent(
      'Mozilla/5.0 (iPhone; CPU iPhone OS 18_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.3 Mobile/15E148 Safari/604.1',
      5
    )
    setViewportWidth(390)

    const wrapper = await mountLayout('/home')

    expect(wrapper.find('.layout-mobile-nav-button').exists()).toBe(true)

    wrapper.unmount()
  })

  it('keeps the floating back-to-top button hidden before a phone-sized layout has scrolled one viewport', async () => {
    setUserAgent(
      'Mozilla/5.0 (iPhone; CPU iPhone OS 18_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.3 Mobile/15E148 Safari/604.1',
      5
    )
    setViewportWidth(390)

    const wrapper = await mountLayout('/profile')
    const layoutMain = wrapper.get('.layout-main').element as HTMLElement

    Object.defineProperty(layoutMain, 'clientHeight', {
      configurable: true,
      value: 640,
    })

    Object.defineProperty(layoutMain, 'scrollTop', {
      configurable: true,
      writable: true,
      value: 639,
    })

    layoutMain.dispatchEvent(new Event('scroll'))
    await flushPromises()

    expect(wrapper.find('.layout-back-to-top-button').exists()).toBe(false)

    wrapper.unmount()
  })

  it('scrolls the shared layout container back to the top when the mobile button is pressed', async () => {
    setUserAgent(
      'Mozilla/5.0 (iPhone; CPU iPhone OS 18_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.3 Mobile/15E148 Safari/604.1',
      5
    )
    setViewportWidth(390)

    const wrapper = await mountLayout('/profile')
    const layoutMain = wrapper.get('.layout-main').element as HTMLElement
    const scrollToMock = vi.fn()

    Object.defineProperty(layoutMain, 'clientHeight', {
      configurable: true,
      value: 640,
    })
    Object.defineProperty(layoutMain, 'scrollTop', {
      configurable: true,
      writable: true,
      value: 640,
    })
    Object.defineProperty(layoutMain, 'scrollTo', {
      configurable: true,
      value: scrollToMock,
    })

    layoutMain.dispatchEvent(new Event('scroll'))
    await flushPromises()

    expect(wrapper.find('.layout-back-to-top-button').exists()).toBe(true)
    await wrapper.get('.layout-back-to-top-button').trigger('click')

    expect(scrollToMock).toHaveBeenCalledWith({
      top: 0,
      behavior: 'smooth',
    })

    wrapper.unmount()
  })

  it('does not render the floating back-to-top button on wider non-phone layouts', async () => {
    setUserAgent(
      'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36'
    )
    setViewportWidth(900)

    const wrapper = await mountLayout('/profile')
    const layoutMain = wrapper.get('.layout-main').element as HTMLElement

    Object.defineProperty(layoutMain, 'clientHeight', {
      configurable: true,
      value: 640,
    })
    Object.defineProperty(layoutMain, 'scrollTop', {
      configurable: true,
      writable: true,
      value: 900,
    })

    layoutMain.dispatchEvent(new Event('scroll'))
    await flushPromises()

    expect(wrapper.find('.layout-back-to-top-button').exists()).toBe(false)

    wrapper.unmount()
  })

  it('does not render the global nav button on the chat route because chat owns its own entry points', async () => {
    setUserAgent(
      'Mozilla/5.0 (iPhone; CPU iPhone OS 18_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.3 Mobile/15E148 Safari/604.1',
      5
    )
    setViewportWidth(390)

    const wrapper = await mountLayout('/chat')

    expect(wrapper.find('.layout-mobile-nav-button').exists()).toBe(false)
    expect(wrapper.find('.layout-back-to-top-button').exists()).toBe(false)

    wrapper.unmount()
  })

  it('renders the custom macOS window drag area inside Tauri desktop mode', async () => {
    setUserAgent(
      'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36'
    )
    tauriState.isTauri = true
    tauriState.platform = 'macos'

    const wrapper = await mountLayout('/home')

    expect(wrapper.find('.layout-window-chrome').exists()).toBe(true)
    expect(wrapper.find('.layout-window-chrome-pill').exists()).toBe(false)
    expect(wrapper.find('.layout-window-chrome').attributes('data-tauri-drag-region')).toBe('')
    expect(wrapper.find('.layout-window-chrome-bar').attributes('data-tauri-drag-region')).toBe('')
    expect(wrapper.find('.layout-window-chrome-traffic-slot').exists()).toBe(true)

    wrapper.unmount()
  })

  it('keeps the macOS window chrome titleless on the chat route', async () => {
    setUserAgent(
      'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36'
    )
    tauriState.isTauri = true
    tauriState.platform = 'macos'

    const wrapper = await mountLayout('/chat')

    expect(wrapper.find('.layout-window-chrome').exists()).toBe(true)
    expect(wrapper.find('.layout-window-chrome-pill').exists()).toBe(false)
    expect(wrapper.find('.layout-window-chrome-bar').attributes('data-tauri-drag-region')).toBe('')

    wrapper.unmount()
  })

  it('syncs the saved desktop close behavior to Tauri on mount', async () => {
    tauriState.isTauri = true
    settingsStoreState.closeBehavior = 'minimize'

    const wrapper = await mountLayout('/home')

    expect(tauriState.setCloseBehavior).toHaveBeenCalledWith('minimize')

    wrapper.unmount()
  })

  it('starts dragging when the macOS chrome strip is pressed', async () => {
    tauriState.isTauri = true
    tauriState.platform = 'macos'

    const wrapper = await mountLayout('/home')

    await wrapper.get('.layout-window-chrome').trigger('mousedown', { button: 0 })

    expect(tauriState.startWindowDragging).toHaveBeenCalledTimes(1)

    wrapper.unmount()
  })
})
