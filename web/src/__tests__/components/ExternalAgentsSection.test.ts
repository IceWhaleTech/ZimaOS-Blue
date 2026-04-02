import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { i18n } from '@/i18n'
import ExternalAgentsSection from '@/components/settings/ExternalAgentsSection.vue'
import { agentSessionsApi, type AgentProfile } from '@/api/agentSessions'
import zhCN from '@/i18n/locales/zh-CN'

vi.mock('@/api/agentSessions', () => ({
  agentSessionsApi: {
    listProfiles: vi.fn(),
    saveProfile: vi.fn(),
    verifyProfile: vi.fn(),
    healthProfile: vi.fn(),
  },
}))

const seededProfiles: AgentProfile[] = [
  {
    id: 'codex',
    protocol: 'acp',
    name: 'codex',
    title: 'Codex ACP',
    builtin: true,
    command: ['npx', '@zed-industries/codex-acp'],
    health_status: 'ready',
  },
  {
    id: 'generic-a2a',
    protocol: 'a2a',
    name: 'generic-a2a',
    title: 'Generic Remote A2A Agent',
    builtin: true,
    endpoint_url: 'https://agent.example.com/rpc',
    health_status: 'verified',
  },
]

describe('ExternalAgentsSection', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    ;(i18n.global as { setLocaleMessage: (locale: string, message: unknown) => void }).setLocaleMessage(
      'zh-CN',
      zhCN
    )
    i18n.global.locale.value = 'zh-CN'

    vi.mocked(agentSessionsApi.listProfiles).mockResolvedValue({
      data: { profiles: seededProfiles },
    } as never)
    vi.mocked(agentSessionsApi.verifyProfile).mockResolvedValue({
      data: { ok: true, message: 'profile verified' },
    } as never)
    vi.mocked(agentSessionsApi.healthProfile).mockResolvedValue({
      data: { healthy: true, message: 'profile healthy' },
    } as never)
  })

  it('loads mixed profiles, simplifies builtin cards, and runs verify and health actions', async () => {
    const healthyProfiles: AgentProfile[] = seededProfiles.map((profile) =>
      profile.id === 'generic-a2a' ? { ...profile, health_status: 'healthy' } : profile
    )
    vi.mocked(agentSessionsApi.listProfiles)
      .mockResolvedValueOnce({
        data: { profiles: seededProfiles },
      } as never)
      .mockResolvedValueOnce({
        data: { profiles: seededProfiles },
      } as never)
      .mockResolvedValueOnce({
        data: { profiles: healthyProfiles },
      } as never)

    const wrapper = mount(ExternalAgentsSection, {
      global: {
        plugins: [i18n],
      },
    })

    await flushPromises()

    expect(agentSessionsApi.listProfiles).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('Codex ACP')
    expect(wrapper.text()).toContain('Generic Remote A2A Agent')

    const profileCards = wrapper.findAll('[data-testid="external-agents-profile-card"]')
    expect(profileCards[0]!.text()).toContain('内置模板')
    expect(profileCards[0]!.text()).not.toContain('就绪')
    expect(profileCards[1]!.text()).toContain('内置模板')
    expect(profileCards[1]!.text()).not.toContain('已验证')

    await profileCards[1]!.trigger('click')
    await flushPromises()

    await wrapper.get('[data-testid="external-agents-verify-current"]').trigger('click')
    await flushPromises()
    expect(agentSessionsApi.verifyProfile).toHaveBeenCalledWith({ id: 'generic-a2a' })

    await wrapper.get('[data-testid="external-agents-health-current"]').trigger('click')
    await flushPromises()
    expect(agentSessionsApi.healthProfile).toHaveBeenCalledWith('generic-a2a')
    expect(wrapper.get('[data-testid="external-agents-notice"]').text()).toContain(
      'profile healthy'
    )
    expect(wrapper.findAll('[data-testid="external-agents-profile-card"]')[1]!.text()).toContain(
      '内置模板'
    )

    wrapper.unmount()
  })

  it('creates a new custom A2A profile from the editor form', async () => {
    const savedProfile: AgentProfile = {
      id: 'custom-a2a',
      protocol: 'a2a',
      name: 'remote-agent',
      title: 'Remote Agent',
      endpoint_url: 'https://remote.example.com/rpc',
    }
    vi.mocked(agentSessionsApi.listProfiles)
      .mockResolvedValueOnce({
        data: { profiles: seededProfiles },
      } as never)
      .mockResolvedValueOnce({
        data: { profiles: [...seededProfiles, savedProfile] },
      } as never)
    vi.mocked(agentSessionsApi.saveProfile).mockResolvedValue({
      data: savedProfile,
    } as never)

    const wrapper = mount(ExternalAgentsSection, {
      global: {
        plugins: [i18n],
      },
    })

    await flushPromises()

    await wrapper.get('[data-testid="external-agents-new-profile"]').trigger('click')
    await wrapper.get('[data-testid="external-agents-draft-protocol-a2a"]').trigger('click')
    await wrapper.get('[data-testid="external-agents-name"]').setValue('remote-agent')
    await wrapper
      .get('[data-testid="external-agents-endpoint"]')
      .setValue('https://remote.example.com/rpc')
    await wrapper.get('[data-testid="external-agents-save"]').trigger('click')
    await flushPromises()

    expect(agentSessionsApi.saveProfile).toHaveBeenCalledWith(
      expect.objectContaining({
        protocol: 'a2a',
        name: 'remote-agent',
        endpoint_url: 'https://remote.example.com/rpc',
      })
    )
    expect(wrapper.text()).toContain('Remote Agent')

    wrapper.unmount()
  })

  it('prefers API error details over generic axios 400 messages when saving fails', async () => {
    vi.mocked(agentSessionsApi.saveProfile).mockRejectedValue({
      message: 'Request failed with status code 400',
      response: {
        data: {
          error: 'profile name is required',
        },
      },
    } as never)

    const wrapper = mount(ExternalAgentsSection, {
      global: {
        plugins: [i18n],
      },
    })

    await flushPromises()

    await wrapper.get('[data-testid="external-agents-new-profile"]').trigger('click')
    await wrapper.get('[data-testid="external-agents-draft-protocol-a2a"]').trigger('click')
    await wrapper.get('[data-testid="external-agents-name"]').setValue('remote-agent')
    await wrapper
      .get('[data-testid="external-agents-endpoint"]')
      .setValue('https://remote.example.com/rpc')
    await wrapper.get('[data-testid="external-agents-save"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="external-agents-notice"]').text()).toContain(
      'profile name is required'
    )
    expect(wrapper.text()).not.toContain('Request failed with status code 400')

    wrapper.unmount()
  })
})
