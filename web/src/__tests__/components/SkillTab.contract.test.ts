import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import SkillTab from '@/components/extensions/SkillTab.vue'
import { i18n } from '@/i18n'
import { skillApi } from '@/api/skill'

vi.mock('@/api/skill', () => ({
  skillApi: {
    list: vi.fn(),
    getContent: vi.fn(),
    enable: vi.fn(),
    disable: vi.fn(),
  },
}))

describe('SkillTab contract detail', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setActivePinia(createPinia())
    i18n.global.locale.value = 'en-US'

    vi.mocked(skillApi.list).mockResolvedValue({
      data: [
        {
          id: 'legacy_skill',
          name: 'Legacy Skill',
          version: '1.0.0',
          description: 'Legacy fixture skill',
          enabled: true,
          builtin: false,
          category: 'productivity',
          tags: ['legacy'],
          contract_status: 'legacy_fallback',
          contract_source: 'legacy_frontmatter_fallback',
        },
      ],
    } as never)
    vi.mocked(skillApi.getContent).mockResolvedValue({
      data: {
        id: 'legacy_skill',
        name: 'Legacy Skill',
        content: '# Legacy Skill\n\nFixture docs.',
        source: 'directory',
        entry_file: 'SKILL.md',
        contract_status: 'legacy_fallback',
        contract_source: 'legacy_frontmatter_fallback',
        contract_notes: [
          'legacy manifest compatibility fallback applied: missing required frontmatter field: version',
        ],
      },
    } as never)
  })

  it('shows contract notice in the skill detail modal from content metadata', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const wrapper = mount(SkillTab, {
      global: {
        plugins: [pinia, i18n],
        stubs: {
          teleport: true,
        },
      },
    })

    await flushPromises()
    await flushPromises()

    expect(wrapper.text()).toContain('Legacy')

    await wrapper.get('.skill-showcase-card').trigger('click')
    await flushPromises()

    const contractPanel = wrapper.get('.detail-contract-panel')
    expect(contractPanel.text()).toContain('Legacy fallback')
    expect(contractPanel.text()).toContain('Legacy frontmatter fallback')
    expect(contractPanel.text()).toContain('missing required frontmatter field: version')
  })
})
