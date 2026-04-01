<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  agentSessionsApi,
  type AgentProfile,
  type ProfileHealthResult,
  type ProfileVerifyResult,
  type ProtocolKind,
} from '@/api/agentSessions'

type NoticeTone = 'info' | 'success' | 'error'

interface ProfileDraft {
  id: string
  protocol: ProtocolKind
  name: string
  title: string
  description: string
  commandText: string
  envText: string
  cwd: string
  cardUrl: string
  endpointUrl: string
  headersText: string
  credentialProviderId: string
  authMethodId: string
  metadataText: string
}

const { t } = useI18n()

const loading = ref(false)
const saving = ref(false)
const verifying = ref(false)
const checkingHealth = ref(false)
const profiles = ref<AgentProfile[]>([])
const selectedProfileID = ref('')
const noticeMessage = ref('')
const noticeTone = ref<NoticeTone>('info')
const protocolOrder: ProtocolKind[] = ['acp', 'a2a']
const activeProtocol = ref<ProtocolKind>('acp')

const draft = ref<ProfileDraft>(createDraft('acp'))

const selectedProfile = computed(
  () => profiles.value.find((profile) => profile.id === selectedProfileID.value) ?? null
)
const isBuiltinSelection = computed(() => selectedProfile.value?.builtin === true)
const activeProfiles = computed(() =>
  profiles.value.filter((profile) => profile.protocol === activeProtocol.value)
)
const activeProtocolCount = computed(() => activeProfiles.value.length)
const activeProtocolTitle = computed(() => protocolTitle(activeProtocol.value))
const currentSelectionLabel = computed(() => {
  if (selectedProfile.value) {
    return selectedProfile.value.title || selectedProfile.value.name
  }
  return (
    draft.value.title || draft.value.name || t('settings.externalAgents.newProfile')
  )
})
const currentSelectionSummary = computed(() => {
  if (!selectedProfile.value) {
    return t('settings.externalAgents.draftHint')
  }
  return `${profileModeLabel(selectedProfile.value)} · ${draftRouteValue()}`
})
const protocolCards = computed(() =>
  protocolOrder.map((protocol) => ({
    protocol,
    title: protocolTitle(protocol),
    count: profiles.value.filter((profile) => profile.protocol === protocol).length,
  }))
)
const checklistItems = computed(() => [
  {
    key: 'name',
    done: Boolean(draft.value.name.trim()),
    label: t('settings.externalAgents.requiredName'),
  },
  {
    key: 'route',
    done: isRouteReady(),
    label:
      draft.value.protocol === 'acp'
        ? t('settings.externalAgents.requiredAcpRoute')
        : t('settings.externalAgents.requiredA2aRoute'),
  },
  {
    key: 'checks',
    done: Boolean(selectedProfile.value?.last_verified_at || selectedProfile.value?.last_health_at),
    label: selectedProfile.value
      ? t('settings.externalAgents.requiredChecks')
      : t('settings.externalAgents.saveThenCheck'),
  },
])

function createDraft(protocol: ProtocolKind): ProfileDraft {
  return {
    id: '',
    protocol,
    name: '',
    title: '',
    description: '',
    commandText: '',
    envText: '',
    cwd: '',
    cardUrl: '',
    endpointUrl: '',
    headersText: '',
    credentialProviderId: '',
    authMethodId: '',
    metadataText: '',
  }
}

function stringifyObject(value?: Record<string, unknown>): string {
  if (!value || Object.keys(value).length === 0) {
    return ''
  }
  return JSON.stringify(value, null, 2)
}

function profileToDraft(profile: AgentProfile): ProfileDraft {
  return {
    id: profile.id || '',
    protocol: profile.protocol,
    name: profile.name || '',
    title: profile.title || '',
    description: profile.description || '',
    commandText: (profile.command || []).join('\n'),
    envText: stringifyObject(profile.env),
    cwd: profile.cwd || '',
    cardUrl: profile.card_url || '',
    endpointUrl: profile.endpoint_url || '',
    headersText: stringifyObject(profile.headers),
    credentialProviderId: profile.credential_provider_id || '',
    authMethodId: profile.auth_method_id || '',
    metadataText: stringifyObject(profile.metadata),
  }
}

function protocolTitle(protocol: ProtocolKind): string {
  return protocol === 'acp'
    ? t('settings.externalAgents.acpPlainTitle')
    : t('settings.externalAgents.a2aPlainTitle')
}

function protocolDescription(protocol: ProtocolKind): string {
  return protocol === 'acp'
    ? t('settings.externalAgents.acpOverview')
    : t('settings.externalAgents.a2aOverview')
}

function protocolListDescription(protocol: ProtocolKind): string {
  return protocol === 'acp'
    ? t('settings.externalAgents.acpListHelp')
    : t('settings.externalAgents.a2aListHelp')
}

function protocolSetupDescription(protocol: ProtocolKind): string {
  return protocol === 'acp'
    ? t('settings.externalAgents.acpSetupHelp')
    : t('settings.externalAgents.a2aSetupHelp')
}

function selectProfile(profileID: string) {
  const profile = profiles.value.find((item) => item.id === profileID)
  if (!profile) {
    return
  }
  selectedProfileID.value = profile.id
  activeProtocol.value = profile.protocol
  draft.value = profileToDraft(profile)
  clearNotice()
}

function startNewProfile(protocol: ProtocolKind) {
  activeProtocol.value = protocol
  selectedProfileID.value = ''
  draft.value = createDraft(protocol)
  clearNotice()
}

function setActiveProtocol(protocol: ProtocolKind) {
  activeProtocol.value = protocol
  if (!selectedProfile.value) {
    draft.value.protocol = protocol
    return
  }
  if (selectedProfile.value.protocol === protocol) {
    return
  }
  const match = profiles.value.find((profile) => profile.protocol === protocol)
  if (match) {
    selectProfile(match.id)
    return
  }
  startNewProfile(protocol)
}

function setDraftProtocol(protocol: ProtocolKind) {
  if (isBuiltinSelection.value) {
    return
  }
  draft.value.protocol = protocol
  activeProtocol.value = protocol
}

function duplicateSelection() {
  if (!selectedProfile.value) {
    return
  }
  const next = profileToDraft(selectedProfile.value)
  selectedProfileID.value = ''
  activeProtocol.value = next.protocol
  draft.value = {
    ...next,
    id: '',
    name: next.name ? `${next.name}-copy` : '',
    title: next.title ? `${next.title} Copy` : '',
  }
  showNotice(
    t('settings.externalAgents.duplicateReady'),
    'info'
  )
}

function parseCommandText(value: string): string[] {
  return value
    .split('\n')
    .map((part) => part.trim())
    .filter(Boolean)
}

function parseJSONObject(value: string, label: string): Record<string, unknown> {
  const trimmed = value.trim()
  if (!trimmed) {
    return {}
  }
  let parsed: unknown
  try {
    parsed = JSON.parse(trimmed)
  } catch {
    throw new Error(t('settings.externalAgents.errors.labelMustBeValidJson', { label }))
  }
  if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') {
    throw new Error(t('settings.externalAgents.errors.labelMustBeJsonObject', { label }))
  }
  return parsed as Record<string, unknown>
}

function parseRecordText(value: string, label: string): Record<string, string> | undefined {
  const trimmed = value.trim()
  if (!trimmed) {
    return undefined
  }
  const parsed = parseJSONObject(trimmed, label)
  return Object.fromEntries(
    Object.entries(parsed).map(([key, item]) => [key, String(item)])
  )
}

function parseMetadataText(value: string): Record<string, unknown> | undefined {
  const trimmed = value.trim()
  if (!trimmed) {
    return undefined
  }
  return parseJSONObject(trimmed, t('settings.externalAgents.metadata'))
}

function buildProfilePayload(): AgentProfile {
  const name = draft.value.name.trim()
  if (!name) {
    throw new Error(t('settings.externalAgents.errors.nameRequired'))
  }

  const payload: AgentProfile = {
    id: draft.value.id.trim(),
    protocol: draft.value.protocol,
    name,
  }

  if (draft.value.title.trim()) {
    payload.title = draft.value.title.trim()
  }
  if (draft.value.description.trim()) {
    payload.description = draft.value.description.trim()
  }
  if (draft.value.credentialProviderId.trim()) {
    payload.credential_provider_id = draft.value.credentialProviderId.trim()
  }
  if (draft.value.metadataText.trim()) {
    payload.metadata = parseMetadataText(draft.value.metadataText)
  }

  if (draft.value.protocol === 'acp') {
    const command = parseCommandText(draft.value.commandText)
    if (command.length === 0) {
      throw new Error(t('settings.externalAgents.errors.acpCommandRequired'))
    }
    payload.command = command
    if (draft.value.cwd.trim()) {
      payload.cwd = draft.value.cwd.trim()
    }
    if (draft.value.envText.trim()) {
      payload.env = parseRecordText(draft.value.envText, 'Environment overrides')
    }
    if (draft.value.authMethodId.trim()) {
      payload.auth_method_id = draft.value.authMethodId.trim()
    }
    return payload
  }

  if (!draft.value.endpointUrl.trim() && !draft.value.cardUrl.trim()) {
    throw new Error(t('settings.externalAgents.errors.a2aRouteRequired'))
  }
  if (draft.value.endpointUrl.trim()) {
    payload.endpoint_url = draft.value.endpointUrl.trim()
  }
  if (draft.value.cardUrl.trim()) {
    payload.card_url = draft.value.cardUrl.trim()
  }
  if (draft.value.headersText.trim()) {
    payload.headers = parseRecordText(draft.value.headersText, 'Request headers')
  }
  return payload
}

function showNotice(message: string, tone: NoticeTone) {
  noticeMessage.value = message
  noticeTone.value = tone
}

function extractErrorMessage(error: unknown, fallback: string): string {
  const apiError = (error as { response?: { data?: { error?: string } } })?.response?.data?.error
  if (typeof apiError === 'string' && apiError.trim()) {
    return apiError.trim()
  }
  const message = (error as Error | undefined)?.message
  if (typeof message === 'string' && message.trim()) {
    return message.trim()
  }
  return fallback
}

function clearNotice() {
  noticeMessage.value = ''
  noticeTone.value = 'info'
}

async function loadProfiles(preferredID = '') {
  loading.value = true
  try {
    const response = await agentSessionsApi.listProfiles()
    profiles.value = response.data.profiles || []
    const nextSelection =
      preferredID ||
      (selectedProfileID.value &&
      profiles.value.some((profile) => profile.id === selectedProfileID.value)
        ? selectedProfileID.value
        : profiles.value[0]?.id || '')

    if (nextSelection) {
      selectProfile(nextSelection)
    } else {
      startNewProfile(activeProtocol.value)
    }
  } catch (error) {
    showNotice(
      extractErrorMessage(
        error,
        t('settings.externalAgents.loadFailed')
      ),
      'error'
    )
  } finally {
    loading.value = false
  }
}

function summarizeProfile(profile: AgentProfile): string {
  if (profile.protocol === 'acp') {
    return (
      (profile.command || []).join(' ') ||
      t('settings.externalAgents.notAvailable')
    )
  }
  return (
    profile.endpoint_url ||
    profile.card_url ||
    t('settings.externalAgents.remoteAgent')
  )
}

function profileModeLabel(profile: AgentProfile): string {
  return profile.builtin
    ? t('settings.externalAgents.builtinTemplate')
    : t('settings.externalAgents.customProfile')
}

function statusLabel(profile: AgentProfile): string {
  if (profile.health_status) {
    return profile.health_status
  }
  return profile.builtin
    ? t('settings.externalAgents.ready')
    : t('settings.externalAgents.custom')
}

function profileStatusTone(profile: AgentProfile): 'neutral' | 'healthy' | 'warning' | 'error' {
  const value = profile.health_status?.toLowerCase() || ''
  if (
    value.includes('healthy') ||
    value.includes('ready') ||
    value.includes('ok') ||
    value.includes('verified') ||
    value.includes('pass')
  ) {
    return 'healthy'
  }
  if (
    value.includes('warn') ||
    value.includes('degraded') ||
    value.includes('pending') ||
    value.includes('checking')
  ) {
    return 'warning'
  }
  if (
    value.includes('fail') ||
    value.includes('error') ||
    value.includes('down') ||
    value.includes('unhealthy')
  ) {
    return 'error'
  }
  return 'neutral'
}

function formatCompactTimestamp(value?: string): string {
  if (!value) {
    return t('settings.externalAgents.notAvailable')
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  return date.toLocaleDateString()
}

function profileLastActivity(profile: AgentProfile): string {
  if (profile.last_health_at) {
    return `${t('settings.externalAgents.health')} · ${formatCompactTimestamp(profile.last_health_at)}`
  }
  if (profile.last_verified_at) {
    return `${t('settings.externalAgents.verify')} · ${formatCompactTimestamp(profile.last_verified_at)}`
  }
  return t('settings.externalAgents.notAvailable')
}

function draftRouteValue(): string {
  if (draft.value.protocol === 'acp') {
    const command = parseCommandText(draft.value.commandText)
    return command[0] || t('settings.externalAgents.pendingSetup')
  }
  const hasEndpoint = Boolean(draft.value.endpointUrl.trim())
  const hasCard = Boolean(draft.value.cardUrl.trim())
  if (hasEndpoint && hasCard) {
    return t('settings.externalAgents.endpointAndCard')
  }
  if (hasEndpoint) {
    return draft.value.endpointUrl.trim()
  }
  if (hasCard) {
    return draft.value.cardUrl.trim()
  }
  return t('settings.externalAgents.pendingSetup')
}

function draftRouteHint(): string {
  if (draft.value.protocol === 'acp') {
    const details = []
    if (parseCommandText(draft.value.commandText).length > 0) {
      details.push(t('settings.externalAgents.commandConfigured'))
    }
    if (draft.value.cwd.trim()) {
      details.push(t('settings.externalAgents.cwd'))
    }
    if (draft.value.envText.trim()) {
      details.push(t('settings.externalAgents.environment'))
    }
    return (
      details.join(' · ') ||
      t('settings.externalAgents.acpRouteHint')
    )
  }

  const details = []
  if (draft.value.endpointUrl.trim()) {
    details.push(t('settings.externalAgents.endpoint'))
  }
  if (draft.value.cardUrl.trim()) {
    details.push(t('settings.externalAgents.cardUrl'))
  }
  if (draft.value.headersText.trim()) {
    details.push(t('settings.externalAgents.headers'))
  }
  return (
    details.join(' · ') ||
    t('settings.externalAgents.a2aRouteHint')
  )
}

function isRouteReady(): boolean {
  if (draft.value.protocol === 'acp') {
    return parseCommandText(draft.value.commandText).length > 0
  }
  return Boolean(draft.value.endpointUrl.trim() || draft.value.cardUrl.trim())
}

async function saveDraft() {
  if (isBuiltinSelection.value) {
    showNotice(
      t('settings.externalAgents.builtinLocked'),
      'info'
    )
    return
  }
  saving.value = true
  try {
    const payload = buildProfilePayload()
    const response = await agentSessionsApi.saveProfile(payload)
    await loadProfiles(response.data.id)
    showNotice(t('settings.externalAgents.saved'), 'success')
  } catch (error) {
    showNotice(
      extractErrorMessage(
        error,
        t('settings.externalAgents.saveFailed')
      ),
      'error'
    )
  } finally {
    saving.value = false
  }
}

function verifyMessage(result: ProfileVerifyResult): string {
  if (result.message) {
    return result.message
  }
  if (result.ok) {
    return t('settings.externalAgents.verifyOk')
  }
  return t('settings.externalAgents.verifyFailed')
}

function healthMessage(result: ProfileHealthResult): string {
  if (result.message) {
    return result.message
  }
  if (result.healthy) {
    return t('settings.externalAgents.healthOk')
  }
  return t('settings.externalAgents.healthFailed')
}

async function verifyCurrent() {
  verifying.value = true
  try {
    let result
    if (selectedProfile.value) {
      result = await agentSessionsApi.verifyProfile({ id: selectedProfile.value.id })
      await loadProfiles(selectedProfile.value.id)
    } else {
      result = await agentSessionsApi.verifyProfile({ profile: buildProfilePayload() })
    }
    showNotice(verifyMessage(result.data), result.data.ok ? 'success' : 'info')
  } catch (error) {
    showNotice(
      extractErrorMessage(
        error,
        t('settings.externalAgents.verifyRequestFailed')
      ),
      'error'
    )
  } finally {
    verifying.value = false
  }
}

async function checkCurrentHealth() {
  if (!selectedProfile.value) {
    showNotice(
      t('settings.externalAgents.saveBeforeHealth'),
      'info'
    )
    return
  }
  checkingHealth.value = true
  try {
    const result = await agentSessionsApi.healthProfile(selectedProfile.value.id)
    await loadProfiles(selectedProfile.value.id)
    showNotice(healthMessage(result.data), result.data.healthy ? 'success' : 'info')
  } catch (error) {
    showNotice(
      extractErrorMessage(
        error,
        t('settings.externalAgents.healthRequestFailed')
      ),
      'error'
    )
  } finally {
    checkingHealth.value = false
  }
}

onMounted(() => {
  void loadProfiles()
})
</script>

<template>
  <div class="external-agents" data-testid="external-agents-section">
    <header class="external-agents__hero">
      <div class="external-agents__hero-copy">
        <p class="external-agents__eyebrow">
          {{ t('settings.externalAgents.eyebrow') }}
        </p>
        <h3 class="external-agents__hero-title">
          {{ t('settings.externalAgents.title') }}
        </h3>
        <p class="external-agents__copy">
          {{
            t('settings.externalAgents.description')
          }}
        </p>
      </div>

      <div class="external-agents__hero-actions">
        <button
          type="button"
          class="external-agents__button external-agents__button--quiet"
          data-testid="external-agents-new-acp"
          @click="startNewProfile('acp')"
        >
          {{ t('settings.externalAgents.newAcp') }}
        </button>
        <button
          type="button"
          class="external-agents__button external-agents__button--quiet"
          data-testid="external-agents-new-a2a"
          @click="startNewProfile('a2a')"
        >
          {{ t('settings.externalAgents.newA2a') }}
        </button>
        <button
          type="button"
          class="external-agents__button external-agents__button--quiet"
          :disabled="loading"
          @click="loadProfiles(selectedProfileID)"
        >
          {{ t('common.refresh', 'Refresh') }}
        </button>
      </div>
    </header>

    <div
      v-if="noticeMessage"
      class="external-agents__notice"
      :class="`external-agents__notice--${noticeTone}`"
      data-testid="external-agents-notice"
    >
      {{ noticeMessage }}
    </div>

    <section class="external-agents__protocol-strip">
      <div class="external-agents__protocol-tabs">
        <button
          v-for="item in protocolCards"
          :key="item.protocol"
          type="button"
          class="external-agents__protocol-tab"
          :class="{ 'external-agents__protocol-tab--active': activeProtocol === item.protocol }"
          :data-testid="`external-agents-protocol-card-${item.protocol}`"
          @click="setActiveProtocol(item.protocol)"
        >
          <span
            class="external-agents__badge external-agents__badge--protocol"
            :class="`external-agents__badge--${item.protocol}`"
          >
            {{ item.protocol.toUpperCase() }}
          </span>
          <strong class="external-agents__protocol-tab-title">{{ item.title }}</strong>
          <span class="external-agents__count">{{ item.count }}</span>
        </button>
      </div>

      <div class="external-agents__protocol-summary">
        <div class="external-agents__protocol-summary-copy">
          <strong class="external-agents__protocol-summary-title">{{ activeProtocolTitle }}</strong>
          <p class="external-agents__protocol-summary-text">
            {{ protocolDescription(activeProtocol) }}
          </p>
        </div>
        <span class="external-agents__count">{{ activeProtocolCount }}</span>
      </div>
    </section>

    <div class="external-agents__workspace">
      <aside class="external-agents__panel external-agents__panel--sidebar">
        <div class="external-agents__panel-header">
          <div>
            <p class="external-agents__eyebrow">
              {{ t('settings.externalAgents.availableProfiles') }}
            </p>
            <h4 class="external-agents__panel-title">{{ activeProtocolTitle }}</h4>
            <p class="external-agents__panel-copy">
              {{ protocolListDescription(activeProtocol) }}
            </p>
          </div>
          <span class="external-agents__count">{{ activeProfiles.length }}</span>
        </div>

        <div v-if="loading" class="external-agents__empty">
          {{ t('common.loading', 'Loading') }}
        </div>
        <div v-else-if="activeProfiles.length === 0" class="external-agents__empty">
          <strong>{{ activeProtocolTitle }}</strong>
          <p>
            {{ protocolListDescription(activeProtocol) }}
          </p>
          <button
            type="button"
            class="external-agents__button external-agents__button--quiet"
            @click="startNewProfile(activeProtocol)"
          >
            {{
              activeProtocol === 'acp'
                ? t('settings.externalAgents.newAcp')
                : t('settings.externalAgents.newA2a')
            }}
          </button>
        </div>
        <div v-else class="external-agents__profile-list">
          <button
            v-for="profile in activeProfiles"
            :key="profile.id"
            type="button"
            class="external-agents__profile-card"
            :class="{ 'external-agents__profile-card--active': selectedProfileID === profile.id }"
            data-testid="external-agents-profile-card"
            @click="selectProfile(profile.id)"
          >
            <div class="external-agents__profile-top">
              <div class="external-agents__profile-title-group">
                <strong class="external-agents__profile-name">
                  {{ profile.title || profile.name }}
                </strong>
                <span class="external-agents__profile-type">{{ profileModeLabel(profile) }}</span>
              </div>
              <span
                class="external-agents__badge external-agents__badge--status"
                :class="`external-agents__badge--${profileStatusTone(profile)}`"
              >
                {{ statusLabel(profile) }}
              </span>
            </div>

            <p class="external-agents__profile-summary">{{ summarizeProfile(profile) }}</p>

            <div class="external-agents__profile-meta">
              <span class="external-agents__meta-chip">{{ profile.id }}</span>
              <span class="external-agents__meta-chip">{{ profileLastActivity(profile) }}</span>
            </div>
          </button>
        </div>
      </aside>

      <section class="external-agents__panel external-agents__panel--editor">
        <div class="external-agents__panel-header">
          <div>
            <p class="external-agents__eyebrow">
              {{
                selectedProfile
                  ? t('settings.externalAgents.editor')
                  : t('settings.externalAgents.newProfile')
              }}
            </p>
            <h4 class="external-agents__panel-title">{{ currentSelectionLabel }}</h4>
            <p class="external-agents__panel-copy">{{ currentSelectionSummary }}</p>
          </div>

          <div class="external-agents__panel-actions">
            <button
              v-if="selectedProfile && !isBuiltinSelection"
              type="button"
              class="external-agents__button external-agents__button--quiet"
              @click="duplicateSelection"
            >
              {{ t('settings.externalAgents.duplicate') }}
            </button>
            <button
              type="button"
              class="external-agents__button external-agents__button--quiet"
              data-testid="external-agents-verify-current"
              :disabled="verifying"
              @click="verifyCurrent"
            >
              {{ t('settings.externalAgents.verify') }}
            </button>
            <button
              type="button"
              class="external-agents__button external-agents__button--quiet"
              data-testid="external-agents-health-current"
              :disabled="checkingHealth || !selectedProfile"
              @click="checkCurrentHealth"
            >
              {{ t('settings.externalAgents.health') }}
            </button>
            <button
              type="button"
              class="external-agents__button external-agents__button--primary"
              data-testid="external-agents-save"
              :disabled="saving || isBuiltinSelection"
              @click="saveDraft"
            >
              {{ t('common.save', 'Save') }}
            </button>
          </div>
        </div>

        <div v-if="isBuiltinSelection" class="external-agents__locked">
          <div class="external-agents__locked-copy">
            <strong>{{ t('settings.externalAgents.builtinTemplate') }}</strong>
            <p>
              {{
                t('settings.externalAgents.builtinHelp')
              }}
            </p>
          </div>
          <button
            type="button"
            class="external-agents__button external-agents__button--quiet"
            @click="duplicateSelection"
          >
            {{ t('settings.externalAgents.duplicate') }}
          </button>
        </div>

        <section class="external-agents__section">
          <div class="external-agents__section-head">
            <div>
              <h5 class="external-agents__section-title">
                {{ t('settings.externalAgents.connectionType') }}
              </h5>
              <p class="external-agents__section-copy">
                {{ protocolSetupDescription(draft.protocol) }}
              </p>
            </div>

            <article class="external-agents__route-preview">
              <span class="external-agents__route-label">
                {{ t('settings.externalAgents.route') }}
              </span>
              <strong class="external-agents__route-value">{{ draftRouteValue() }}</strong>
              <p class="external-agents__route-copy">{{ draftRouteHint() }}</p>
            </article>
          </div>

          <div class="external-agents__protocol-switch">
            <button
              type="button"
              class="external-agents__protocol-switch-card"
              :class="{ 'external-agents__protocol-switch-card--active': draft.protocol === 'acp' }"
              :disabled="isBuiltinSelection"
              @click="setDraftProtocol('acp')"
            >
              <span
                class="external-agents__badge external-agents__badge--protocol external-agents__badge--acp"
              >
                ACP
              </span>
              <strong>{{ protocolTitle('acp') }}</strong>
              <p>{{ protocolDescription('acp') }}</p>
            </button>

            <button
              type="button"
              class="external-agents__protocol-switch-card"
              :class="{ 'external-agents__protocol-switch-card--active': draft.protocol === 'a2a' }"
              :disabled="isBuiltinSelection"
              @click="setDraftProtocol('a2a')"
            >
              <span
                class="external-agents__badge external-agents__badge--protocol external-agents__badge--a2a"
              >
                A2A
              </span>
              <strong>{{ protocolTitle('a2a') }}</strong>
              <p>{{ protocolDescription('a2a') }}</p>
            </button>
          </div>

          <div class="external-agents__checklist">
            <div
              v-for="item in checklistItems"
              :key="item.key"
              class="external-agents__checklist-item"
              :class="{ 'external-agents__checklist-item--done': item.done }"
            >
              <span class="external-agents__check-dot"></span>
              <span>{{ item.label }}</span>
            </div>
          </div>
        </section>

        <section class="external-agents__section">
          <div class="external-agents__section-head">
            <div>
              <h5 class="external-agents__section-title">
                {{ t('settings.externalAgents.basicInfo') }}
              </h5>
              <p class="external-agents__section-copy">
                {{
                  t('settings.externalAgents.basicInfoHelp')
                }}
              </p>
            </div>
          </div>

          <div class="external-agents__form-grid">
            <label class="external-agents__field">
              <span>{{ t('common.name', 'Name') }}</span>
              <input
                v-model="draft.name"
                class="external-agents__input"
                data-testid="external-agents-name"
                :disabled="isBuiltinSelection"
              />
            </label>

            <label class="external-agents__field">
              <span>{{ t('common.title', 'Title') }}</span>
              <input
                v-model="draft.title"
                class="external-agents__input"
                :disabled="isBuiltinSelection"
              />
            </label>

            <label class="external-agents__field external-agents__field--full">
              <span>{{ t('common.description', 'Description') }}</span>
              <input
                v-model="draft.description"
                class="external-agents__input"
                :disabled="isBuiltinSelection"
              />
            </label>
          </div>
        </section>

        <section class="external-agents__section">
          <div class="external-agents__section-head">
            <div>
              <h5 class="external-agents__section-title">
                {{ t('settings.externalAgents.connectionDetails') }}
              </h5>
              <p class="external-agents__section-copy">
                {{
                  draft.protocol === 'acp'
                    ? t('settings.externalAgents.acpConnectionHelp')
                    : t('settings.externalAgents.a2aConnectionHelp')
                }}
              </p>
            </div>
          </div>

          <div class="external-agents__form-grid">
            <template v-if="draft.protocol === 'acp'">
              <label class="external-agents__field external-agents__field--full">
                <span>{{ t('settings.externalAgents.command') }}</span>
                <textarea
                  v-model="draft.commandText"
                  class="external-agents__textarea"
                  data-testid="external-agents-command"
                  :disabled="isBuiltinSelection"
                  placeholder="npx&#10;-y&#10;@zed-industries/codex-acp"
                />
                <p class="external-agents__field-help">
                  {{
                    t('settings.externalAgents.commandHint')
                  }}
                </p>
              </label>

              <label class="external-agents__field">
                <span>{{ t('settings.externalAgents.cwd') }}</span>
                <input
                  v-model="draft.cwd"
                  class="external-agents__input"
                  :disabled="isBuiltinSelection"
                  placeholder="/workspace/project"
                />
              </label>

              <label class="external-agents__field external-agents__field--full">
                <span>{{ t('settings.externalAgents.environment') }}</span>
                <textarea
                  v-model="draft.envText"
                  class="external-agents__textarea"
                  :disabled="isBuiltinSelection"
                  placeholder='{&#10;  "NODE_ENV": "development"&#10;}'
                />
              </label>
            </template>

            <template v-else>
              <label class="external-agents__field">
                <span>{{ t('settings.externalAgents.endpoint') }}</span>
                <input
                  v-model="draft.endpointUrl"
                  class="external-agents__input"
                  data-testid="external-agents-endpoint"
                  :disabled="isBuiltinSelection"
                  placeholder="https://agent.example.com/rpc"
                />
              </label>

              <label class="external-agents__field">
                <span>{{ t('settings.externalAgents.cardUrl') }}</span>
                <input
                  v-model="draft.cardUrl"
                  class="external-agents__input"
                  data-testid="external-agents-card-url"
                  :disabled="isBuiltinSelection"
                  placeholder="https://agent.example.com/.well-known/agent-card.json"
                />
              </label>

              <label class="external-agents__field external-agents__field--full">
                <span>{{ t('settings.externalAgents.headers') }}</span>
                <textarea
                  v-model="draft.headersText"
                  class="external-agents__textarea"
                  :disabled="isBuiltinSelection"
                  placeholder='{&#10;  "Authorization": "Bearer ..."&#10;}'
                />
              </label>
            </template>
          </div>
        </section>

        <section class="external-agents__section">
          <div class="external-agents__section-head">
            <div>
              <h5 class="external-agents__section-title">
                {{ t('settings.externalAgents.accessControl') }}
              </h5>
              <p class="external-agents__section-copy">
                {{
                  t('settings.externalAgents.accessControlHelp')
                }}
              </p>
            </div>
          </div>

          <div class="external-agents__form-grid">
            <label class="external-agents__field">
              <span>{{ t('settings.externalAgents.credentialSource') }}</span>
              <input
                v-model="draft.credentialProviderId"
                class="external-agents__input"
                :disabled="isBuiltinSelection"
                placeholder="openai-codex"
              />
            </label>

            <label v-if="draft.protocol === 'acp'" class="external-agents__field">
              <span>{{ t('settings.externalAgents.authMethod') }}</span>
              <input
                v-model="draft.authMethodId"
                class="external-agents__input"
                :disabled="isBuiltinSelection"
                placeholder="oauth"
              />
            </label>
          </div>
        </section>

        <details class="external-agents__advanced">
          <summary class="external-agents__advanced-summary">
            {{ t('settings.externalAgents.advancedOptions') }}
          </summary>
          <p class="external-agents__section-copy">
            {{
              t('settings.externalAgents.advancedOptionsHelp')
            }}
          </p>

          <div class="external-agents__form-grid">
            <label class="external-agents__field">
              <span>{{ t('common.id', 'ID') }}</span>
              <input
                v-model="draft.id"
                class="external-agents__input"
                :disabled="isBuiltinSelection"
              />
            </label>

            <label class="external-agents__field external-agents__field--full">
              <span>{{ t('settings.externalAgents.metadata') }}</span>
              <textarea
                v-model="draft.metadataText"
                class="external-agents__textarea"
                :disabled="isBuiltinSelection"
                placeholder='{&#10;  "team": "blue"&#10;}'
              />
            </label>
          </div>
        </details>
      </section>
    </div>
  </div>
</template>

<style scoped>
.external-agents {
  display: grid;
  gap: 0.44rem;
  width: 100%;
}

.external-agents__hero,
.external-agents__panel-header,
.external-agents__section-head,
.external-agents__locked {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 0.4rem;
}

.external-agents__hero {
  padding-bottom: 0.1rem;
  border-bottom: 1px solid rgba(226, 232, 240, 0.88);
}

.external-agents__hero-copy,
.external-agents__locked-copy {
  display: grid;
  gap: 0.14rem;
  min-width: 0;
}

.external-agents__hero-title,
.external-agents__panel-title {
  margin: 0;
  color: rgba(15, 23, 42, 0.94);
  font-size: 0.73rem;
  font-weight: 700;
  line-height: 1.18;
}

.external-agents__copy,
.external-agents__panel-copy,
.external-agents__section-copy,
.external-agents__field-help,
.external-agents__notice,
.external-agents__empty,
.external-agents__locked p,
.external-agents__route-copy {
  margin: 0;
  color: rgba(71, 85, 105, 0.92);
  font-size: 0.6rem;
  line-height: 1.26;
}

.external-agents__eyebrow {
  margin: 0;
  color: rgba(var(--settings-accent, 37, 99, 235), 0.92);
  font-size: 0.49rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.external-agents__hero-actions,
.external-agents__panel-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.22rem;
}

.external-agents__button {
  border: 1px solid rgba(203, 213, 225, 0.96);
  border-radius: 0.58rem;
  background: #ffffff;
  color: rgba(15, 23, 42, 0.96);
  font-size: 0.58rem;
  font-weight: 600;
  line-height: 1.2;
  padding: 0.22rem 0.38rem;
  cursor: pointer;
  transition:
    border-color 160ms ease,
    background-color 160ms ease,
    color 160ms ease;
}

.external-agents__button:hover {
  border-color: rgba(148, 163, 184, 0.6);
  background: rgba(248, 250, 252, 0.96);
}

.external-agents__button:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.external-agents__button--quiet {
  background: rgba(248, 250, 252, 0.94);
  color: rgba(51, 65, 85, 0.94);
}

.external-agents__button--primary {
  border-color: rgba(var(--settings-accent, 37, 99, 235), 0.18);
  background: rgb(var(--settings-accent, 37, 99, 235));
  color: #ffffff;
}

.external-agents__button--primary:hover {
  border-color: rgba(var(--settings-accent, 37, 99, 235), 0.24);
  background: rgba(var(--settings-accent, 37, 99, 235), 0.92);
}

.external-agents__notice,
.external-agents__empty,
.external-agents__locked,
.external-agents__section,
.external-agents__advanced {
  padding: 0.42rem 0.46rem;
  border-radius: 0.72rem;
  border: 1px solid rgba(226, 232, 240, 0.96);
  background: rgba(248, 250, 252, 0.84);
}

.external-agents__empty {
  display: grid;
  gap: 0.18rem;
}

.external-agents__notice--success {
  border-color: rgba(22, 163, 74, 0.22);
  background: rgba(240, 253, 244, 0.92);
  color: rgba(21, 128, 61, 0.94);
}

.external-agents__notice--error {
  border-color: rgba(220, 38, 38, 0.2);
  background: rgba(254, 242, 242, 0.94);
  color: rgba(185, 28, 28, 0.94);
}

.external-agents__protocol-tab,
.external-agents__protocol-switch-card,
.external-agents__profile-card {
  border: 1px solid rgba(226, 232, 240, 0.96);
  border-radius: 0.68rem;
  background: #ffffff;
  cursor: pointer;
  text-align: left;
  transition:
    border-color 160ms ease,
    background-color 160ms ease,
    box-shadow 160ms ease;
}

.external-agents__protocol-strip {
  display: grid;
  gap: 0.18rem;
}

.external-agents__protocol-tabs {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.18rem;
}

.external-agents__protocol-tab {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.24rem;
  min-width: 0;
  padding: 0.24rem 0.32rem;
}

.external-agents__protocol-tab:hover,
.external-agents__protocol-tab--active,
.external-agents__protocol-switch-card:hover,
.external-agents__protocol-switch-card--active,
.external-agents__profile-card:hover,
.external-agents__profile-card--active {
  border-color: rgba(148, 163, 184, 0.62);
  background: rgba(248, 250, 252, 0.96);
  box-shadow: inset 0 0 0 1px rgba(148, 163, 184, 0.12);
}

.external-agents__protocol-tab-title {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: rgba(15, 23, 42, 0.95);
  font-size: 0.6rem;
  font-weight: 700;
  line-height: 1.16;
}

.external-agents__protocol-summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.28rem;
  padding: 0.24rem 0.32rem;
  border-radius: 0.64rem;
  border: 1px solid rgba(226, 232, 240, 0.96);
  background: rgba(248, 250, 252, 0.74);
}

.external-agents__protocol-summary-copy {
  display: grid;
  gap: 0.04rem;
  min-width: 0;
}

.external-agents__protocol-summary-title {
  color: rgba(15, 23, 42, 0.95);
  font-size: 0.6rem;
  font-weight: 700;
  line-height: 1.16;
}

.external-agents__protocol-summary-text {
  margin: 0;
  color: rgba(100, 116, 139, 0.92);
  font-size: 0.56rem;
  line-height: 1.18;
}

.external-agents__profile-top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.24rem;
}

.external-agents__profile-name,
.external-agents__section-title,
.external-agents__route-value,
.external-agents__advanced-summary {
  color: rgba(15, 23, 42, 0.95);
  font-size: 0.62rem;
  font-weight: 700;
  line-height: 1.22;
}

.external-agents__profile-type {
  margin: 0;
  color: rgba(100, 116, 139, 0.92);
  font-size: 0.55rem;
  line-height: 1.2;
}

.external-agents__workspace {
  display: grid;
  grid-template-columns: minmax(15rem, 0.76fr) minmax(0, 1.28fr);
  gap: 0.42rem;
}

.external-agents__panel {
  display: grid;
  align-content: start;
  gap: 0.38rem;
  padding: 0.48rem;
  border-radius: 0.74rem;
  border: 1px solid rgba(226, 232, 240, 0.96);
  background: #ffffff;
  min-width: 0;
}

.external-agents__count,
.external-agents__badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0.08rem 0.28rem;
  border-radius: 999px;
  border: 1px solid rgba(203, 213, 225, 0.96);
  background: rgba(248, 250, 252, 0.94);
  color: rgba(71, 85, 105, 0.94);
  font-size: 0.49rem;
  font-weight: 700;
  letter-spacing: 0.04em;
}

.external-agents__badge--protocol {
  border: 0;
}

.external-agents__badge--acp {
  background: rgba(var(--settings-accent, 37, 99, 235), 0.1);
  color: rgba(var(--settings-accent, 37, 99, 235), 0.94);
}

.external-agents__badge--a2a {
  background: rgba(71, 85, 105, 0.12);
  color: rgba(51, 65, 85, 0.92);
}

.external-agents__badge--status {
  border-color: rgba(203, 213, 225, 0.96);
  background: rgba(248, 250, 252, 0.96);
}

.external-agents__badge--healthy {
  border-color: rgba(22, 163, 74, 0.22);
  background: rgba(240, 253, 244, 0.92);
  color: rgba(21, 128, 61, 0.94);
}

.external-agents__badge--warning {
  border-color: rgba(245, 158, 11, 0.24);
  background: rgba(255, 251, 235, 0.96);
  color: rgba(180, 83, 9, 0.94);
}

.external-agents__badge--error {
  border-color: rgba(220, 38, 38, 0.2);
  background: rgba(254, 242, 242, 0.96);
  color: rgba(185, 28, 28, 0.94);
}

.external-agents__profile-list {
  display: grid;
  gap: 0.24rem;
}

.external-agents__profile-card {
  display: grid;
  gap: 0.14rem;
  padding: 0.34rem 0.38rem;
}

.external-agents__profile-title-group {
  display: grid;
  gap: 0.08rem;
  min-width: 0;
}

.external-agents__profile-summary {
  margin: 0;
  color: rgba(30, 41, 59, 0.9);
  font-size: 0.57rem;
  line-height: 1.22;
  word-break: break-word;
}

.external-agents__profile-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 0.14rem;
}

.external-agents__meta-chip {
  display: inline-flex;
  align-items: center;
  gap: 0.2rem;
  padding: 0.08rem 0.24rem;
  border-radius: 999px;
  background: rgba(241, 245, 249, 0.96);
  color: rgba(71, 85, 105, 0.92);
  font-size: 0.49rem;
  line-height: 1.2;
}

.external-agents__locked {
  align-items: center;
}

.external-agents__section {
  display: grid;
  gap: 0.32rem;
}

.external-agents__section-head {
  gap: 0.28rem;
}

.external-agents__route-preview {
  display: grid;
  gap: 0.1rem;
  min-width: min(100%, 11rem);
  padding: 0.3rem 0.34rem;
  border-radius: 0.62rem;
  border: 1px solid rgba(226, 232, 240, 0.96);
  background: #ffffff;
}

.external-agents__route-label {
  color: rgba(100, 116, 139, 0.9);
  font-size: 0.47rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.external-agents__protocol-switch {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.2rem;
}

.external-agents__protocol-switch-card {
  display: grid;
  gap: 0.08rem;
  min-width: 0;
  padding: 0.28rem 0.32rem;
}

.external-agents__protocol-switch-card:disabled {
  cursor: not-allowed;
  opacity: 0.66;
}

.external-agents__protocol-switch-card strong {
  min-width: 0;
  color: rgba(15, 23, 42, 0.95);
  font-size: 0.58rem;
  font-weight: 700;
  line-height: 1.18;
}

.external-agents__protocol-switch-card p {
  margin: 0;
  color: rgba(100, 116, 139, 0.92);
  font-size: 0.53rem;
  line-height: 1.18;
  word-break: break-word;
}

.external-agents__checklist {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.18rem;
}

.external-agents__checklist-item {
  display: flex;
  align-items: center;
  gap: 0.24rem;
  min-width: 0;
  padding: 0.22rem 0.28rem;
  border-radius: 0.54rem;
  background: rgba(255, 255, 255, 0.82);
  color: rgba(71, 85, 105, 0.94);
  font-size: 0.54rem;
  line-height: 1.16;
}

.external-agents__checklist-item--done {
  background: rgba(240, 253, 244, 0.9);
  color: rgba(21, 128, 61, 0.94);
}

.external-agents__check-dot {
  flex: 0 0 auto;
  width: 0.28rem;
  height: 0.28rem;
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.8);
}

.external-agents__checklist-item--done .external-agents__check-dot {
  background: rgba(22, 163, 74, 0.92);
}

.external-agents__form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.32rem;
}

.external-agents__field {
  display: grid;
  gap: 0.14rem;
  min-width: 0;
}

.external-agents__field > span {
  color: rgba(30, 41, 59, 0.92);
  font-size: 0.55rem;
  font-weight: 600;
}

.external-agents__field--full {
  grid-column: 1 / -1;
}

.external-agents__input,
.external-agents__textarea {
  width: 100%;
  border-radius: 0.58rem;
  border: 1px solid rgba(148, 163, 184, 0.34);
  background: #ffffff;
  color: rgba(15, 23, 42, 0.96);
  padding: 0.26rem 0.34rem;
  font: inherit;
  font-size: 0.58rem;
  transition:
    border-color 160ms ease,
    box-shadow 160ms ease,
    background-color 160ms ease;
}

.external-agents__input:focus,
.external-agents__textarea:focus {
  outline: none;
  border-color: rgba(var(--settings-accent, 37, 99, 235), 0.46);
  box-shadow: 0 0 0 3px rgba(var(--settings-accent, 37, 99, 235), 0.12);
}

.external-agents__input:disabled,
.external-agents__textarea:disabled {
  background: rgba(248, 250, 252, 0.96);
  color: rgba(100, 116, 139, 0.92);
}

.external-agents__textarea {
  min-height: 3.15rem;
  resize: vertical;
}

.external-agents__advanced {
  display: grid;
  gap: 0.3rem;
}

.external-agents__advanced-summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.3rem;
  cursor: pointer;
  list-style: none;
}

.external-agents__advanced-summary::after {
  content: '+';
  color: rgba(100, 116, 139, 0.9);
  font-size: 0.7rem;
  line-height: 1;
}

.external-agents__advanced[open] > .external-agents__advanced-summary::after {
  content: '-';
}

.external-agents__advanced-summary::-webkit-details-marker {
  display: none;
}

@media (max-width: 1100px) {
  .external-agents__workspace {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 760px) {
  .external-agents__hero,
  .external-agents__panel-header,
  .external-agents__section-head,
  .external-agents__locked {
    flex-direction: column;
    align-items: stretch;
  }

  .external-agents__protocol-tabs,
  .external-agents__protocol-switch,
  .external-agents__form-grid,
  .external-agents__checklist {
    grid-template-columns: 1fr;
  }

  .external-agents__protocol-summary {
    justify-content: flex-start;
  }

  .external-agents__field--full {
    grid-column: auto;
  }
}

:global(.dark) .external-agents__panel,
:global(.dark) .external-agents__protocol-tab,
:global(.dark) .external-agents__protocol-summary,
:global(.dark) .external-agents__protocol-switch-card,
:global(.dark) .external-agents__profile-card,
:global(.dark) .external-agents__notice,
:global(.dark) .external-agents__empty,
:global(.dark) .external-agents__locked,
:global(.dark) .external-agents__section,
:global(.dark) .external-agents__advanced,
:global(.dark) .external-agents__route-preview,
:global(.dark) .external-agents__input,
:global(.dark) .external-agents__textarea {
  background: rgba(15, 23, 42, 0.76);
  border-color: rgba(71, 85, 105, 0.86);
  color: rgba(226, 232, 240, 0.94);
}

:global(.dark) .external-agents__hero {
  border-bottom-color: rgba(51, 65, 85, 0.86);
}

:global(.dark) .external-agents__hero-title,
:global(.dark) .external-agents__panel-title,
:global(.dark) .external-agents__section-title,
:global(.dark) .external-agents__route-value,
:global(.dark) .external-agents__protocol-tab-title,
:global(.dark) .external-agents__protocol-summary-title,
:global(.dark) .external-agents__advanced-summary,
:global(.dark) .external-agents__field > span,
:global(.dark) .external-agents__protocol-switch-card strong,
:global(.dark) .external-agents__profile-name {
  color: rgba(241, 245, 249, 0.95);
}

:global(.dark) .external-agents__copy,
:global(.dark) .external-agents__panel-copy,
:global(.dark) .external-agents__section-copy,
:global(.dark) .external-agents__field-help,
:global(.dark) .external-agents__profile-summary,
:global(.dark) .external-agents__profile-type,
:global(.dark) .external-agents__route-copy,
:global(.dark) .external-agents__protocol-summary-text,
:global(.dark) .external-agents__protocol-switch-card p,
:global(.dark) .external-agents__locked p,
:global(.dark) .external-agents__empty {
  color: rgba(203, 213, 225, 0.88);
}

:global(.dark) .external-agents__button,
:global(.dark) .external-agents__count,
:global(.dark) .external-agents__badge,
:global(.dark) .external-agents__meta-chip {
  background: rgba(30, 41, 59, 0.94);
  border-color: rgba(71, 85, 105, 0.92);
  color: rgba(226, 232, 240, 0.92);
}

:global(.dark) .external-agents__button:hover,
:global(.dark) .external-agents__protocol-tab:hover,
:global(.dark) .external-agents__protocol-tab--active,
:global(.dark) .external-agents__protocol-switch-card:hover,
:global(.dark) .external-agents__protocol-switch-card--active,
:global(.dark) .external-agents__profile-card:hover,
:global(.dark) .external-agents__profile-card--active {
  background: rgba(30, 41, 59, 0.98);
  border-color: rgba(100, 116, 139, 0.92);
}

:global(.dark) .external-agents__button--primary,
:global(.dark) .external-agents__button--primary:hover {
  background: rgba(var(--settings-accent, 37, 99, 235), 0.9);
  border-color: rgba(var(--settings-accent, 37, 99, 235), 0.3);
  color: #ffffff;
}

:global(.dark) .external-agents__advanced-summary::after {
  color: rgba(203, 213, 225, 0.88);
}

:global(.dark) .external-agents__notice--success {
  border-color: rgba(34, 197, 94, 0.22);
  background: rgba(20, 83, 45, 0.45);
  color: rgba(187, 247, 208, 0.94);
}

:global(.dark) .external-agents__notice--error {
  border-color: rgba(248, 113, 113, 0.24);
  background: rgba(127, 29, 29, 0.45);
  color: rgba(254, 202, 202, 0.94);
}

:global(.dark) .external-agents__checklist-item {
  background: rgba(15, 23, 42, 0.78);
  color: rgba(226, 232, 240, 0.9);
}

:global(.dark) .external-agents__checklist-item--done {
  background: rgba(20, 83, 45, 0.45);
  color: rgba(187, 247, 208, 0.94);
}

:global(.dark) .external-agents__check-dot {
  background: rgba(100, 116, 139, 0.88);
}

:global(.dark) .external-agents__checklist-item--done .external-agents__check-dot {
  background: rgba(34, 197, 94, 0.92);
}

:global(.dark) .external-agents__badge--acp {
  background: rgba(var(--settings-accent, 37, 99, 235), 0.2);
  color: rgba(191, 219, 254, 0.95);
}

:global(.dark) .external-agents__badge--a2a {
  background: rgba(51, 65, 85, 0.94);
  color: rgba(226, 232, 240, 0.92);
}

:global(.dark) .external-agents__badge--healthy {
  background: rgba(20, 83, 45, 0.45);
  border-color: rgba(34, 197, 94, 0.22);
  color: rgba(187, 247, 208, 0.94);
}

:global(.dark) .external-agents__badge--warning {
  background: rgba(120, 53, 15, 0.45);
  border-color: rgba(245, 158, 11, 0.24);
  color: rgba(253, 224, 71, 0.94);
}

:global(.dark) .external-agents__badge--error {
  background: rgba(127, 29, 29, 0.45);
  border-color: rgba(248, 113, 113, 0.24);
  color: rgba(254, 202, 202, 0.94);
}
</style>
