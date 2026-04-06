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
    deleteProfile: vi.fn(),
  },
}))

const seededProfiles: AgentProfile[] = [
  {
    id: 'codex',
    protocol: 'acp',
    name: 'codex',
    title: 'Codex ACP',
    builtin: true,
    template_only: true,
    command: ['codex-acp'],
    credential_provider_id: 'openai-codex',
    metadata: {
      reference: 'openclaw/acpx',
    },
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
      data: {
        ok: true,
        message_code: 'profile_verified',
        message: 'ACP runtime handshake passed',
      },
    } as never)
    vi.mocked(agentSessionsApi.healthProfile).mockResolvedValue({
      data: {
        healthy: true,
        message_code: 'runtime_healthy',
        message: 'ACP transport ping ok',
      },
    } as never)
  })

  it('renders built-in ACP entries as setup templates and keeps direct runtime actions available', async () => {
    const wrapper = mount(ExternalAgentsSection, {
      global: {
        plugins: [i18n],
      },
    })

    await flushPromises()

    expect(agentSessionsApi.listProfiles).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('Codex ACP')
    expect(wrapper.text()).toContain('可直接验证或使用')
    expect(wrapper.text()).not.toContain('元数据')
    expect(
      wrapper.get('[data-testid="external-agents-verify-current"]').attributes('disabled')
    ).toBeUndefined()
    expect(
      wrapper.get('[data-testid="external-agents-health-current"]').attributes('disabled')
    ).toBeUndefined()
    expect(wrapper.get('[data-testid="external-agents-save"]').attributes()).toHaveProperty(
      'disabled'
    )
    expect(wrapper.find('[data-testid="external-agents-delete-current"]').exists()).toBe(false)

    await wrapper.get('[data-testid="external-agents-verify-current"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="external-agents-notice"]').text()).toContain(
      '配置验证成功。'
    )
    expect(wrapper.get('[data-testid="external-agents-notice"]').text()).not.toContain(
      'ACP runtime handshake passed'
    )

    await wrapper.get('[data-testid="external-agents-health-current"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="external-agents-notice"]').text()).toContain(
      '健康检查成功。'
    )
    expect(wrapper.get('[data-testid="external-agents-notice"]').text()).not.toContain(
      'ACP transport ping ok'
    )

    expect(agentSessionsApi.verifyProfile).toHaveBeenCalledWith({ id: 'codex' })
    expect(agentSessionsApi.healthProfile).toHaveBeenCalledWith('codex')

    wrapper.unmount()
  })

  it('deletes a migrated custom ACP profile from the list', async () => {
    const confirmMock = vi.fn(() => true)
    vi.stubGlobal('confirm', confirmMock)

    const migratedProfile: AgentProfile = {
      id: 'codex-migrated',
      protocol: 'acp',
      name: 'codex-migrated',
      title: 'Codex ACP (Migrated)',
      command: ['npx', '@zed-industries/codex-acp'],
      metadata: {
        migrated_from_builtin_profile_id: 'codex',
      },
    }
    vi.mocked(agentSessionsApi.listProfiles)
      .mockResolvedValueOnce({
        data: { profiles: [migratedProfile, ...seededProfiles] },
      } as never)
      .mockResolvedValueOnce({
        data: { profiles: seededProfiles },
      } as never)
    vi.mocked(agentSessionsApi.deleteProfile).mockResolvedValue({
      data: { deleted: true },
    } as never)

    const wrapper = mount(ExternalAgentsSection, {
      global: {
        plugins: [i18n],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('Codex ACP (Migrated)')
    expect(wrapper.get('[data-testid="external-agents-delete-current"]').exists()).toBe(true)

    await wrapper.get('[data-testid="external-agents-delete-current"]').trigger('click')
    await flushPromises()

    expect(confirmMock).toHaveBeenCalledTimes(1)
    expect(agentSessionsApi.deleteProfile).toHaveBeenCalledWith('codex-migrated')
    expect(wrapper.text()).not.toContain('Codex ACP (Migrated)')

    wrapper.unmount()
    vi.unstubAllGlobals()
  })

  it('keeps built-in A2A verify and health actions available', async () => {
    const healthyProfiles: AgentProfile[] = seededProfiles.map((profile) =>
      profile.id === 'generic-a2a' ? { ...profile, health_status: 'healthy' } : profile
    )
    vi.mocked(agentSessionsApi.verifyProfile).mockResolvedValueOnce({
      data: {
        ok: true,
        message_code: 'profile_verified',
        message: 'A2A card probe completed successfully',
      },
    } as never)
    vi.mocked(agentSessionsApi.healthProfile).mockResolvedValueOnce({
      data: {
        healthy: true,
        message_code: 'runtime_healthy',
        message: 'A2A endpoint probe completed successfully',
      },
    } as never)
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
    expect(wrapper.text()).toContain('可直接验证或使用')

    const profileCards = wrapper.findAll('[data-testid="external-agents-profile-card"]')
    expect(profileCards[0]!.text()).toContain('内置配置')
    expect(profileCards[0]!.text()).not.toContain('就绪')
    expect(profileCards[0]!.text()).not.toContain('复制后填写可执行命令')
    expect(profileCards[1]!.text()).toContain('内置配置')
    expect(profileCards[1]!.text()).not.toContain('已验证')

    await profileCards[1]!.trigger('click')
    await flushPromises()

    await wrapper.get('[data-testid="external-agents-verify-current"]').trigger('click')
    await flushPromises()
    expect(agentSessionsApi.verifyProfile).toHaveBeenCalledWith({ id: 'generic-a2a' })
    expect(wrapper.get('[data-testid="external-agents-notice"]').text()).toContain(
      '配置验证成功。'
    )
    expect(wrapper.get('[data-testid="external-agents-notice"]').text()).not.toContain(
      'A2A card probe completed successfully'
    )

    await wrapper.get('[data-testid="external-agents-health-current"]').trigger('click')
    await flushPromises()
    expect(agentSessionsApi.healthProfile).toHaveBeenCalledWith('generic-a2a')
    expect(wrapper.get('[data-testid="external-agents-notice"]').text()).toContain(
      '健康检查成功。'
    )
    expect(wrapper.get('[data-testid="external-agents-notice"]').text()).not.toContain(
      'A2A endpoint probe completed successfully'
    )
    expect(wrapper.findAll('[data-testid="external-agents-profile-card"]')[1]!.text()).toContain(
      '内置配置'
    )

    wrapper.unmount()
  })

  it('duplicates an ACP template into an editable custom draft and saves a non-Node command', async () => {
    const savedProfile: AgentProfile = {
      id: 'custom-codex',
      protocol: 'acp',
      name: 'codex-copy',
      title: 'Codex ACP Copy',
      command: ['/opt/acp/bin/codex', 'serve'],
      credential_provider_id: 'openai-codex',
    }
    vi.mocked(agentSessionsApi.listProfiles)
      .mockResolvedValueOnce({
        data: { profiles: seededProfiles },
      } as never)
      .mockResolvedValueOnce({
        data: { profiles: [savedProfile, ...seededProfiles] },
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

    await wrapper.get('[data-testid="external-agents-duplicate-selection"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="external-agents-command"]').attributes('disabled')).toBeUndefined()
    expect(wrapper.get('[data-testid="external-agents-command"]').element.value).toBe('codex-acp')
    await wrapper.get('[data-testid="external-agents-name"]').setValue('codex-copy')
    await wrapper
      .get('[data-testid="external-agents-command"]')
      .setValue('/opt/acp/bin/codex\nserve')
    await wrapper.get('[data-testid="external-agents-save"]').trigger('click')
    await flushPromises()

    expect(agentSessionsApi.saveProfile).toHaveBeenCalledWith(
      expect.objectContaining({
        protocol: 'acp',
        name: 'codex-copy',
        command: ['/opt/acp/bin/codex', 'serve'],
        credential_provider_id: 'openai-codex',
      })
    )
    expect(vi.mocked(agentSessionsApi.saveProfile).mock.calls[0]?.[0]).not.toHaveProperty(
      'metadata'
    )
    expect(wrapper.text()).toContain('Codex ACP Copy')

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
    expect(agentSessionsApi.listProfiles).toHaveBeenCalledTimes(2)

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
