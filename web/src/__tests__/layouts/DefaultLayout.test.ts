import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import DefaultLayout from '@/layouts/DefaultLayout.vue'

const { setupFormFillerWidget, cleanupFormFillerWidget, previewStoreState, settingsStoreState } =
  vi.hoisted(() => ({
    setupFormFillerWidget: vi.fn(),
    cleanupFormFillerWidget: vi.fn(),
    previewStoreState: {
      isPreviewMode: false,
    },
    settingsStoreState: {
      closeBehavior: 'quit',
      claudeCodeEnabled: false,
      claudeCodeEnabledLoaded: true,
    },
  }))

vi.mock('@/composables/useFormFillerWidget', () => ({
  useFormFillerWidget: () => ({
    setup: setupFormFillerWidget,
    cleanup: cleanupFormFillerWidget,
  }),
}))

vi.mock('@/composables/useTauri', () => ({
  useTauri: () => ({
    isTauri: { value: false },
    setCloseBehavior: vi.fn(),
  }),
}))

vi.mock('@/stores/preview', () => ({
  usePreviewStore: () => previewStoreState,
}))

vi.mock('@/stores/settings', () => ({
  useSettingsStore: () => settingsStoreState,
}))

vi.mock('@/components/AppSidebar.vue', () => ({
  default: {
    name: 'AppSidebar',
    template: '<aside class="app-sidebar-stub" />',
  },
}))

vi.mock('@/components/formfiller/FormFillerWidget.vue', () => ({
  default: {
    name: 'FormFillerWidget',
    template: '<div class="form-filler-widget-stub" />',
  },
}))

vi.mock('@/components/onboarding/PreviewOnboardingModal.vue', () => ({
  default: {
    name: 'PreviewOnboardingModal',
    template: '<div class="preview-onboarding-modal-stub" />',
  },
}))

vi.mock('@/components/typeless/FullscreenModal.vue', () => ({
  default: {
    name: 'FullscreenModal',
    template: '<div class="fullscreen-modal-stub" />',
  },
}))

vi.mock('@/components/voicewake/GlobalVoiceWakeBanner.vue', () => ({
  default: {
    name: 'GlobalVoiceWakeBanner',
    template: '<div class="global-voicewake-banner-stub" />',
  },
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

  return mount(DefaultLayout, {
    global: {
      plugins: [router],
    },
  })
}

describe('DefaultLayout', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    previewStoreState.isPreviewMode = false
    settingsStoreState.claudeCodeEnabled = false
    settingsStoreState.claudeCodeEnabledLoaded = true
    setViewportWidth(1440)
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

  it('does not render the global nav button on the chat route because chat owns its own entry points', async () => {
    setUserAgent(
      'Mozilla/5.0 (iPhone; CPU iPhone OS 18_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.3 Mobile/15E148 Safari/604.1',
      5
    )
    setViewportWidth(390)

    const wrapper = await mountLayout('/chat')

    expect(wrapper.find('.layout-mobile-nav-button').exists()).toBe(false)

    wrapper.unmount()
  })

  it('renders preview onboarding on the chat route when preview mode is active and enhanced mode is disabled', async () => {
    setUserAgent(
      'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36'
    )
    previewStoreState.isPreviewMode = true
    settingsStoreState.claudeCodeEnabled = false
    settingsStoreState.claudeCodeEnabledLoaded = true

    const wrapper = await mountLayout('/chat')

    expect(wrapper.find('.preview-onboarding-modal-stub').exists()).toBe(true)

    wrapper.unmount()
  })
})
