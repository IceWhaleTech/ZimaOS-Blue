import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import PluginsView from '@/views/PluginsView.vue'
import { i18n } from '@/i18n'
import { skillApi } from '@/api/skill'

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: {} }),
}))

vi.mock('@/components/extensions', () => ({
  SkillTab: {
    name: 'SkillTab',
    emits: ['install-skill'],
    template:
      '<button class="skill-tab-install" type="button" @click="$emit(\'install-skill\')">Install</button>',
  },
  SkillStoreTab: {
    name: 'SkillStoreTab',
    template: '<div class="skill-store-tab-stub"></div>',
  },
  ToolTab: {
    name: 'ToolTab',
    template: '<div class="tool-tab-stub"></div>',
  },
}))

vi.mock('@/api/skill', () => ({
  skillApi: {
    list: vi.fn(),
    installFromURL: vi.fn(),
    upload: vi.fn(),
  },
}))

async function mountPluginsView() {
  const pinia = createPinia()
  setActivePinia(pinia)

  const wrapper = mount(PluginsView, {
    global: {
      plugins: [pinia, i18n],
    },
  })

  await flushPromises()
  return wrapper
}

describe('PluginsView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    i18n.global.locale.value = 'en-US'

    vi.mocked(skillApi.list).mockResolvedValue({
      data: [],
    } as never)

    vi.mocked(skillApi.installFromURL).mockResolvedValue({
      data: {
        success: true,
        entry_file: 'SKILL.md',
        skill: {
          id: 'plain-thirdparty',
          name: 'Plain Third Party Skill',
        },
        contract_status: 'generated_contract',
        contract_source: 'generated_safe_defaults',
        contract_notes: [
          'generated safe structural contract defaults because strict frontmatter contract was missing: missing required frontmatter field: name',
        ],
      },
    } as never)

    vi.mocked(skillApi.upload).mockResolvedValue({
      data: {
        success: true,
        entry_file: 'AGENT.md',
        skill: {
          id: 'legacy-agent-skill',
          name: 'Legacy Agent Skill',
          version: '1.0.0',
        },
        contract_status: 'legacy_fallback',
        contract_source: 'legacy_frontmatter_fallback',
        contract_notes: [
          'legacy manifest compatibility fallback applied: missing required frontmatter field: version',
        ],
        warnings: [
          'legacy manifest compatibility fallback applied: missing required frontmatter field: version',
        ],
      },
    } as never)
  })

  it('shows generated contract metadata after install-from-url succeeds', async () => {
    const wrapper = await mountPluginsView()

    await wrapper.get('.skill-tab-install').trigger('click')
    await wrapper.get('.url-input').setValue('https://example.com/plain-thirdparty.md')
    await wrapper.get('.btn-confirm').trigger('click')
    await flushPromises()

    expect(skillApi.installFromURL).toHaveBeenCalledWith({
      url: 'https://example.com/plain-thirdparty.md',
    })

    const banner = wrapper.get('.install-result-banner')
    expect(banner.attributes('data-contract-status')).toBe('generated_contract')
    expect(banner.text()).toContain('Plain Third Party Skill')
    expect(banner.text()).toContain('Generated contract')
    expect(banner.text()).toContain('Generated safe defaults')
    expect(banner.text()).toContain('generated safe structural contract defaults')
    expect(wrapper.find('.modal-overlay').exists()).toBe(false)

    await banner.get('.install-result-banner__dismiss').trigger('click')
    expect(wrapper.find('.install-result-banner').exists()).toBe(false)
  })

  it('shows legacy fallback metadata after file upload succeeds', async () => {
    const wrapper = await mountPluginsView()

    await wrapper.get('.skill-tab-install').trigger('click')
    const methodTabs = wrapper.findAll('.method-tab')
    await methodTabs[1]!.trigger('click')

    const file = new File(['legacy archive'], 'legacy-agent-skill.zip', {
      type: 'application/zip',
    })
    const input = wrapper.get('.file-input')
    Object.defineProperty(input.element, 'files', {
      value: [file],
      configurable: true,
    })
    await input.trigger('change')
    await wrapper.get('.btn-confirm').trigger('click')
    await flushPromises()

    expect(skillApi.upload).toHaveBeenCalledTimes(1)

    const banner = wrapper.get('.install-result-banner')
    expect(banner.attributes('data-contract-status')).toBe('legacy_fallback')
    expect(banner.text()).toContain('Legacy Agent Skill')
    expect(banner.text()).toContain('Legacy fallback')
    expect(banner.text()).toContain('Legacy frontmatter fallback')
    expect(banner.text()).toContain('legacy manifest compatibility fallback applied')
  })
})
