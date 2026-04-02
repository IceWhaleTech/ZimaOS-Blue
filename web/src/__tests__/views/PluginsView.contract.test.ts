import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'
import PluginsView from '@/views/PluginsView.vue'
import { i18n } from '@/i18n'
import { useSkillStore } from '@/stores/skill'

vi.mock('@/components/extensions', () => ({
  SkillTab: {
    name: 'SkillTabStub',
    emits: ['install-skill'],
    template:
      '<button data-testid="open-install-modal" @click="$emit(\'install-skill\')">Open install</button>',
  },
  SkillStoreTab: {
    name: 'SkillStoreTabStub',
    props: ['initialSearchQuery'],
    template: '<div class="skill-store-tab-stub">{{ initialSearchQuery }}</div>',
  },
  ToolTab: {
    name: 'ToolTabStub',
    template: '<div class="tool-tab-stub">Tool tab</div>',
  },
}))

async function mountPluginsView() {
  const pinia = createPinia()
  setActivePinia(pinia)
  i18n.global.locale.value = 'en-US'

  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/plugins', component: { template: '<div>Plugins route</div>' } }],
  })
  await router.push('/plugins')
  await router.isReady()

  const store = useSkillStore(pinia)

  const wrapper = mount(PluginsView, {
    global: {
      plugins: [pinia, router, i18n],
    },
  })

  return { wrapper, store }
}

describe('PluginsView contract install summary', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('shows a generated contract summary after install from URL succeeds', async () => {
    const { wrapper, store } = await mountPluginsView()
    const installFromURL = vi.fn().mockResolvedValue({
      success: true,
      skill: { id: 'plain-thirdparty' },
      entry_file: 'SKILL.md',
      contract_status: 'generated_contract',
      contract_source: 'generated_safe_defaults',
      contract_notes: [
        'generated safe structural contract defaults because strict frontmatter contract was missing',
      ],
      warnings: [],
    })
    store.installFromURL = installFromURL as typeof store.installFromURL

    await wrapper.get('[data-testid="open-install-modal"]').trigger('click')
    await wrapper.get('input[type="url"]').setValue('https://example.com/plain-thirdparty.md')
    await wrapper.get('.btn-confirm').trigger('click')
    await flushPromises()

    expect(installFromURL).toHaveBeenCalledWith({
      url: 'https://example.com/plain-thirdparty.md',
    })

    const banner = wrapper.get('.install-result-banner')
    expect(banner.attributes('data-contract-status')).toBe('generated_contract')
    expect(banner.text()).toContain('Generated contract')
    expect(banner.text()).toContain('Generated safe defaults')
    expect(banner.text()).toContain('plain-thirdparty')
    expect(banner.text()).toContain('strict frontmatter contract was missing')
  })

  it('dismisses the install result banner', async () => {
    const { wrapper, store } = await mountPluginsView()
    store.installFromURL = vi.fn().mockResolvedValue({
      success: true,
      skill: { id: 'legacy-url-skill', name: 'Legacy URL Skill' },
      contract_status: 'legacy_fallback',
      contract_source: 'legacy_frontmatter_fallback',
      contract_notes: ['legacy manifest compatibility fallback applied: missing version'],
      warnings: ['legacy manifest compatibility fallback applied: missing version'],
    }) as typeof store.installFromURL

    await wrapper.get('[data-testid="open-install-modal"]').trigger('click')
    await wrapper.get('input[type="url"]').setValue('https://example.com/legacy-url-skill.md')
    await wrapper.get('.btn-confirm').trigger('click')
    await flushPromises()

    expect(wrapper.find('.install-result-banner').exists()).toBe(true)
    await wrapper.get('.install-result-banner__dismiss').trigger('click')
    expect(wrapper.find('.install-result-banner').exists()).toBe(false)
  })
})
