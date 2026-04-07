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

const localizedVerifySuccessMessages = new Set([
  'profile verified',
  'acp profile verified',
  'a2a profile verified',
])

const localizedHealthSuccessMessages = new Set([
  'runtime is healthy',
  'acp runtime is healthy',
  'a2a runtime is healthy',
])

function localizeVerifyMessageCode(messageCode: string | undefined): string | null {
  switch ((messageCode || '').trim().toLowerCase()) {
    case 'profile_verified':
      return t('settings.externalAgents.verifyOk')
    default:
      return null
  }
}

function localizeHealthMessageCode(messageCode: string | undefined): string | null {
  switch ((messageCode || '').trim().toLowerCase()) {
    case 'runtime_healthy':
      return t('settings.externalAgents.healthOk')
    default:
      return null
  }
}

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
const deleting = ref(false)
const verifying = ref(false)
const checkingHealth = ref(false)
const profiles = ref<AgentProfile[]>([])
const selectedProfileID = ref('')
const noticeMessage = ref('')
const noticeTone = ref<NoticeTone>('info')
const draft = ref<ProfileDraft>(createDraft('acp'))

const selectedProfile = computed(
  () => profiles.value.find((profile) => profile.id === selectedProfileID.value) ?? null
)
const isBuiltinSelection = computed(() => selectedProfile.value?.builtin === true)
const isTemplateOnlySelection = computed(() => isTemplateOnlyACPProfile(selectedProfile.value))
const templateRuntimeBlocked = computed(
  () => isTemplateOnlySelection.value && parseCommandText(draft.value.commandText).length === 0
)
const showMetadataField = computed(
  () => !isBuiltinSelection.value || draft.value.metadataText.trim().length > 0
)
const currentSelectionLabel = computed(() => {
  if (selectedProfile.value) {
    return profileDisplayTitle(selectedProfile.value) || t('settings.externalAgents.newProfile')
  }
  return draft.value.title || draft.value.name || t('settings.externalAgents.newProfile')
})
const draftTitleValue = computed(() => {
  if (selectedProfile.value && isBuiltinSelection.value) {
    return profileDisplayTitle(selectedProfile.value)
  }
  return draft.value.title
})
const currentSelectionMeta = computed(() => {
  if (selectedProfile.value) {
    return profileModeLabel(selectedProfile.value)
  }
  return protocolTitle(draft.value.protocol)
})
const lockedHeadingText = computed(() =>
  selectedProfile.value ? profileModeLabel(selectedProfile.value) : t('settings.externalAgents.builtinProfile')
)
const lockedHelpText = computed(() =>
  isTemplateOnlySelection.value
    ? t('settings.externalAgents.acpTemplateHelp')
    : t('settings.externalAgents.builtinHelp')
)

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

function isTemplateOnlyACPProfile(profile?: AgentProfile | null): boolean {
  return profile?.protocol === 'acp' && profile.template_only === true
}

function sanitizeDraftMetadata(profile: AgentProfile): Record<string, unknown> | undefined {
  if (!profile.metadata || Object.keys(profile.metadata).length === 0) {
    return undefined
  }
  const next = { ...profile.metadata }
  if (profile.builtin) {
    delete next.reference
  }
  return Object.keys(next).length > 0 ? next : undefined
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
    metadataText: stringifyObject(sanitizeDraftMetadata(profile)),
  }
}

function localizedBuiltinProfileTitle(profile?: AgentProfile | null): string | null {
  if (!profile?.builtin) {
    return null
  }
  switch (profile.id) {
    case 'generic-a2a':
      return t('settings.externalAgents.genericA2ATitle')
    default:
      return null
  }
}

function profileDisplayTitle(profile?: AgentProfile | null): string {
  return localizedBuiltinProfileTitle(profile) || profile?.title || profile?.name || ''
}

function protocolTitle(protocol: ProtocolKind): string {
  return protocol === 'acp'
    ? t('settings.externalAgents.acpPlainTitle')
    : t('settings.externalAgents.a2aPlainTitle')
}

function selectProfile(profileID: string) {
  const profile = profiles.value.find((item) => item.id === profileID)
  if (!profile) {
    return
  }
  selectedProfileID.value = profile.id
  draft.value = profileToDraft(profile)
  clearNotice()
}

function startNewProfile(
  protocol: ProtocolKind = selectedProfile.value?.protocol ?? draft.value.protocol ?? 'acp'
) {
  selectedProfileID.value = ''
  draft.value = createDraft(protocol)
  clearNotice()
}

function setDraftProtocol(protocol: ProtocolKind) {
  if (isBuiltinSelection.value) {
    return
  }
  draft.value.protocol = protocol
}

function updateDraftTitle(event: Event) {
  draft.value.title = (event.target as HTMLInputElement).value
}

function duplicateSelection() {
  if (!selectedProfile.value) {
    return
  }
  const next = profileToDraft(selectedProfile.value)
  selectedProfileID.value = ''
  draft.value = {
    ...next,
    id: '',
    name: next.name ? `${next.name}-copy` : '',
    title: next.title ? `${next.title} Copy` : '',
  }
  clearNotice()
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

function normalizeBackendMessage(message?: string): string {
  return message?.trim().replace(/\s+/g, ' ').toLowerCase() || ''
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
      startNewProfile()
    }
  } catch (error) {
    showNotice(extractErrorMessage(error, t('settings.externalAgents.loadFailed')), 'error')
  } finally {
    loading.value = false
  }
}

function summarizeProfile(profile: AgentProfile): string {
  if (isTemplateOnlyACPProfile(profile)) {
    return t('settings.externalAgents.acpTemplateSummary')
  }
  if (profile.protocol === 'acp') {
    return (profile.command || []).join(' ') || t('settings.externalAgents.notAvailable')
  }
  return profile.endpoint_url || profile.card_url || t('settings.externalAgents.remoteAgent')
}

function builtinModeLabel(profile: AgentProfile): string {
  return isTemplateOnlyACPProfile(profile) && (profile.command || []).length === 0
    ? t('settings.externalAgents.builtinTemplate')
    : t('settings.externalAgents.builtinProfile')
}

function profileModeLabel(profile: AgentProfile): string {
  return profile.builtin ? builtinModeLabel(profile) : t('settings.externalAgents.customProfile')
}

function profileSidebarMeta(profile: AgentProfile): string {
  return profile.builtin ? builtinModeLabel(profile) : statusLabel(profile)
}

function profileSidebarSummary(profile: AgentProfile): string {
  if (profile.builtin) {
    return ''
  }
  return summarizeProfile(profile)
}

function translateProfileStatus(status: string): string | null {
  const normalized = status.trim().toLowerCase()

  switch (normalized) {
    case 'healthy':
    case 'ok':
      return t('dashboard.healthy')
    case 'verified':
      return t('settings.externalAgents.status.verified')
    case 'ready':
      return t('skillStore.status.ready')
    case 'error':
    case 'unhealthy':
      return t('common.error')
    default:
      return null
  }
}

function statusLabel(profile: AgentProfile): string {
  const status = profile.health_status?.trim()
  if (!status) {
    return profileModeLabel(profile)
  }
  return translateProfileStatus(status) || status
}

function profileStatusTone(profile: AgentProfile): 'neutral' | 'healthy' | 'warning' | 'error' {
  const value = profile.health_status?.toLowerCase() || ''
  if (
    value.includes('fail') ||
    value.includes('error') ||
    value.includes('down') ||
    value.includes('unhealthy')
  ) {
    return 'error'
  }
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
  return 'neutral'
}

async function saveDraft() {
  if (isTemplateOnlySelection.value) {
    showNotice(t('settings.externalAgents.acpTemplateActionDisabled'), 'info')
    return
  }
  if (isBuiltinSelection.value) {
    showNotice(t('settings.externalAgents.builtinLocked'), 'info')
    return
  }
  saving.value = true
  try {
    const payload = buildProfilePayload()
    const response = await agentSessionsApi.saveProfile(payload)
    await loadProfiles(response.data.id)
    showNotice(t('settings.externalAgents.saved'), 'success')
  } catch (error) {
    showNotice(extractErrorMessage(error, t('settings.externalAgents.saveFailed')), 'error')
  } finally {
    saving.value = false
  }
}

async function deleteCurrent() {
  if (!selectedProfile.value) {
    return
  }
  if (isBuiltinSelection.value) {
    showNotice(t('settings.externalAgents.builtinLocked'), 'info')
    return
  }
  const profileLabel =
    selectedProfile.value.title?.trim() || selectedProfile.value.name?.trim() || selectedProfile.value.id
  if (!window.confirm(t('settings.externalAgents.deleteConfirm', { name: profileLabel }))) {
    return
  }
  deleting.value = true
  try {
    await agentSessionsApi.deleteProfile(selectedProfile.value.id)
    await loadProfiles()
    showNotice(t('settings.externalAgents.deleted'), 'success')
  } catch (error) {
    showNotice(extractErrorMessage(error, t('settings.externalAgents.deleteFailed')), 'error')
  } finally {
    deleting.value = false
  }
}

function verifyMessage(result: ProfileVerifyResult): string {
  const localizedByCode = localizeVerifyMessageCode(result.message_code)
  if (localizedByCode) {
    return localizedByCode
  }
  if (localizedVerifySuccessMessages.has(normalizeBackendMessage(result.message))) {
    return t('settings.externalAgents.verifyOk')
  }
  if (result.message) {
    return result.message
  }
  return result.ok
    ? t('settings.externalAgents.verifyOk')
    : t('settings.externalAgents.verifyFailed')
}

function healthMessage(result: ProfileHealthResult): string {
  const localizedByCode = localizeHealthMessageCode(result.message_code)
  if (localizedByCode) {
    return localizedByCode
  }
  if (localizedHealthSuccessMessages.has(normalizeBackendMessage(result.message))) {
    return t('settings.externalAgents.healthOk')
  }
  if (result.message) {
    return result.message
  }
  return result.healthy
    ? t('settings.externalAgents.healthOk')
    : t('settings.externalAgents.healthFailed')
}

async function verifyCurrent() {
  if (templateRuntimeBlocked.value) {
    showNotice(t('settings.externalAgents.acpTemplateActionDisabled'), 'info')
    return
  }
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
      extractErrorMessage(error, t('settings.externalAgents.verifyRequestFailed')),
      'error'
    )
  } finally {
    verifying.value = false
  }
}

async function checkCurrentHealth() {
  if (templateRuntimeBlocked.value) {
    showNotice(t('settings.externalAgents.acpTemplateActionDisabled'), 'info')
    return
  }
  if (!selectedProfile.value) {
    showNotice(t('settings.externalAgents.saveBeforeHealth'), 'info')
    return
  }
  checkingHealth.value = true
  try {
    const result = await agentSessionsApi.healthProfile(selectedProfile.value.id)
    await loadProfiles(selectedProfile.value.id)
    showNotice(healthMessage(result.data), result.data.healthy ? 'success' : 'info')
  } catch (error) {
    showNotice(
      extractErrorMessage(error, t('settings.externalAgents.healthRequestFailed')),
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
    <div class="external-agents__toolbar">
      <div class="external-agents__actions">
        <button
          type="button"
          class="external-agents__button external-agents__button--quiet"
          data-testid="external-agents-new-profile"
          @click="startNewProfile()"
        >
          {{ t('settings.externalAgents.newExternalAgent', t('settings.externalAgents.newProfile')) }}
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
    </div>

    <div
      v-if="noticeMessage"
      class="external-agents__notice"
      :class="`external-agents__notice--${noticeTone}`"
      data-testid="external-agents-notice"
    >
      {{ noticeMessage }}
    </div>

    <div class="external-agents__workspace">
      <aside class="external-agents__panel external-agents__panel--sidebar">
        <div v-if="loading" class="external-agents__empty">
          {{ t('common.loading', 'Loading') }}
        </div>
        <div v-else-if="profiles.length === 0" class="external-agents__empty">
          <p>{{ t('settings.externalAgents.title') }}</p>
          <button
            type="button"
            class="external-agents__button external-agents__button--quiet"
            @click="startNewProfile()"
          >
            {{
              t('settings.externalAgents.newExternalAgent', t('settings.externalAgents.newProfile'))
            }}
          </button>
        </div>
        <div v-else class="external-agents__profile-list">
          <button
            v-for="profile in profiles"
            :key="profile.id"
            type="button"
            class="external-agents__profile-card"
            :class="[
              'external-agents__profile-card--' + profileStatusTone(profile),
              { 'external-agents__profile-card--active': selectedProfileID === profile.id }
            ]"
            data-testid="external-agents-profile-card"
            @click="selectProfile(profile.id)"
          >
            <div class="external-agents__profile-top">
                <div class="external-agents__profile-main">
                <div class="external-agents__profile-heading">
                  <h3 class="external-agents__profile-name">{{ profileDisplayTitle(profile) }}</h3>
                  <span
                    class="external-agents__badge external-agents__badge--protocol"
                    :class="`external-agents__badge--${profile.protocol}`"
                  >
                    {{ profile.protocol.toUpperCase() }}
                  </span>
                </div>
                <p
                  class="external-agents__profile-meta"
                  :class="`external-agents__profile-meta--${profileStatusTone(profile)}`"
                >
                  {{ profileSidebarMeta(profile) }}
                </p>
                <p v-if="profileSidebarSummary(profile)" class="external-agents__profile-summary">
                  {{ profileSidebarSummary(profile) }}
                </p>
              </div>
            </div>
          </button>
        </div>
      </aside>

      <section class="external-agents__panel external-agents__panel--editor">
        <div class="external-agents__editor-head">
          <div class="external-agents__editor-copy">
            <h4 class="external-agents__editor-title">{{ currentSelectionLabel }}</h4>
            <p class="external-agents__editor-meta">{{ currentSelectionMeta }}</p>
          </div>

          <div class="external-agents__actions">
            <button
              v-if="selectedProfile && !isBuiltinSelection"
              type="button"
              class="external-agents__button external-agents__button--quiet"
              data-testid="external-agents-duplicate-selection"
              @click="duplicateSelection"
            >
              {{ t('settings.externalAgents.duplicate') }}
            </button>
            <button
              v-if="selectedProfile && !isBuiltinSelection"
              type="button"
              class="external-agents__button external-agents__button--quiet"
              data-testid="external-agents-delete-current"
              :disabled="deleting"
              @click="deleteCurrent"
            >
              {{ t('common.delete', 'Delete') }}
            </button>
            <button
              type="button"
              class="external-agents__button external-agents__button--quiet"
              data-testid="external-agents-verify-current"
              :disabled="verifying || templateRuntimeBlocked"
              @click="verifyCurrent"
            >
              {{ t('settings.externalAgents.verify') }}
            </button>
            <button
              type="button"
              class="external-agents__button external-agents__button--quiet"
              data-testid="external-agents-health-current"
              :disabled="checkingHealth || !selectedProfile || templateRuntimeBlocked"
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
          <div>
            <strong>{{ lockedHeadingText }}</strong>
            <p>{{ lockedHelpText }}</p>
          </div>
          <button
            type="button"
            class="external-agents__button"
            :class="
              isTemplateOnlySelection
                ? 'external-agents__button--primary'
                : 'external-agents__button--quiet'
            "
            data-testid="external-agents-duplicate-selection"
            @click="duplicateSelection"
          >
            {{ t('settings.externalAgents.duplicate') }}
          </button>
        </div>

        <div class="external-agents__protocol-switch">
          <button
            type="button"
            class="external-agents__protocol-switch-button"
            :class="{ 'external-agents__protocol-switch-button--active': draft.protocol === 'acp' }"
            :disabled="isBuiltinSelection"
            data-testid="external-agents-draft-protocol-acp"
            @click="setDraftProtocol('acp')"
          >
            ACP
          </button>
          <button
            type="button"
            class="external-agents__protocol-switch-button"
            :class="{ 'external-agents__protocol-switch-button--active': draft.protocol === 'a2a' }"
            :disabled="isBuiltinSelection"
            data-testid="external-agents-draft-protocol-a2a"
            @click="setDraftProtocol('a2a')"
          >
            A2A
          </button>
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

          <template v-if="draft.protocol === 'acp'">
            <label class="external-agents__field external-agents__field--full">
              <span>{{ t('settings.externalAgents.command') }}</span>
              <textarea
                v-model="draft.commandText"
                class="external-agents__textarea"
                data-testid="external-agents-command"
                :disabled="isBuiltinSelection"
                placeholder="/usr/local/bin/acp-runtime&#10;--stdio"
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
          </template>
        </div>

        <details class="external-agents__advanced">
          <summary class="external-agents__advanced-summary">
            {{ t('settings.externalAgents.advancedOptions') }}
          </summary>

          <div class="external-agents__form-grid">
            <label class="external-agents__field">
              <span>{{ t('common.title', 'Title') }}</span>
              <input
                :value="draftTitleValue"
                class="external-agents__input"
                :disabled="isBuiltinSelection"
                @input="updateDraftTitle"
              />
            </label>

            <label class="external-agents__field">
              <span>{{ t('settings.externalAgents.credentialSource') }}</span>
              <input
                v-model="draft.credentialProviderId"
                class="external-agents__input"
                :disabled="isBuiltinSelection"
                placeholder="openai-codex"
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

            <template v-if="draft.protocol === 'acp'">
              <label class="external-agents__field">
                <span>{{ t('settings.externalAgents.cwd') }}</span>
                <input
                  v-model="draft.cwd"
                  class="external-agents__input"
                  :disabled="isBuiltinSelection"
                  placeholder="/workspace/project"
                />
              </label>

              <label class="external-agents__field">
                <span>{{ t('settings.externalAgents.authMethod') }}</span>
                <input
                  v-model="draft.authMethodId"
                  class="external-agents__input"
                  :disabled="isBuiltinSelection"
                  placeholder="oauth"
                />
              </label>

              <label class="external-agents__field external-agents__field--full">
                <span>{{ t('settings.externalAgents.environment') }}</span>
                <textarea
                  v-model="draft.envText"
                  class="external-agents__textarea"
                  :disabled="isBuiltinSelection"
                  placeholder='{&#10;  "LOG_LEVEL": "debug"&#10;}'
                />
              </label>
            </template>

            <template v-else>
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

            <label class="external-agents__field">
              <span>{{ t('common.id', 'ID') }}</span>
              <input
                v-model="draft.id"
                class="external-agents__input"
                :disabled="isBuiltinSelection"
              />
            </label>

            <label
              v-if="showMetadataField"
              class="external-agents__field external-agents__field--full"
            >
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
  gap: 0.56rem;
  width: 100%;
}

.external-agents__toolbar,
.external-agents__editor-head,
.external-agents__locked {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 0.4rem;
}

.external-agents__toolbar {
  justify-content: flex-end;
  padding-bottom: 0.08rem;
}

.external-agents__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.22rem;
}

.external-agents__protocol-switch-button,
.external-agents__button {
  border: 1px solid rgba(203, 213, 225, 0.96);
  border-radius: 0.62rem;
  background: #ffffff;
  color: rgba(15, 23, 42, 0.96);
  transition:
    border-color 160ms ease,
    background-color 160ms ease,
    box-shadow 160ms ease,
    color 160ms ease;
}

.external-agents__button {
  cursor: pointer;
}

.external-agents__protocol-switch-button:hover,
.external-agents__protocol-switch-button--active,
.external-agents__button:hover {
  border-color: rgba(148, 163, 184, 0.62);
  background: rgba(248, 250, 252, 0.96);
  box-shadow: inset 0 0 0 1px rgba(148, 163, 184, 0.12);
}

.external-agents__editor-title,
.external-agents__profile-name,
.external-agents__advanced-summary {
  color: rgba(15, 23, 42, 0.95);
  font-size: 0.82rem;
  font-weight: 700;
  line-height: 1.35;
}

.external-agents__editor-title {
  margin: 0;
}

.external-agents__editor-copy,
.external-agents__profile-main {
  display: grid;
  gap: 0.08rem;
  min-width: 0;
}

.external-agents__editor-meta,
.external-agents__profile-meta,
.external-agents__profile-summary,
.external-agents__locked p,
.external-agents__empty,
.external-agents__notice {
  margin: 0;
  color: rgba(71, 85, 105, 0.92);
  font-size: 0.8rem;
  line-height: 1.5;
}

.external-agents__button {
  font-size: 0.82rem;
  font-weight: 600;
  line-height: 1.35;
  padding: 0.22rem 0.38rem;
}

.external-agents__button:disabled,
.external-agents__protocol-switch-button:disabled {
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
.external-agents__locked {
  padding: 0.42rem 0.46rem;
  border-radius: 0.72rem;
  background: rgba(248, 250, 252, 0.84);
}

.external-agents__notice {
  border: 1px solid rgba(226, 232, 240, 0.96);
}

.external-agents__empty,
.external-agents__locked {
  border: 0;
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

.external-agents__workspace {
  display: grid;
  grid-template-columns: minmax(14rem, 0.72fr) minmax(0, 1.28fr);
  gap: 0.88rem;
  align-items: start;
}

.external-agents__panel {
  display: grid;
  align-content: start;
  gap: 0.56rem;
  padding: 0;
  min-width: 0;
}

.external-agents__panel--sidebar {
  padding-right: 0.88rem;
  border-right: 1px solid rgba(226, 232, 240, 0.96);
}

.external-agents__panel--editor {
  padding-left: 0.12rem;
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
  font-size: 0.68rem;
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
  white-space: nowrap;
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
  gap: 0.12rem;
  max-height: 30rem;
  overflow-y: auto;
  padding-right: 0.12rem;
}

.external-agents__profile-card {
  display: grid;
  gap: 0.18rem;
  padding: 0.68rem 0.72rem;
  text-align: left;
  cursor: pointer;
  border: 1px solid rgba(226, 232, 240, 0.96);
  border-radius: 0.78rem;
  background: rgba(248, 250, 252, 0.92);
  transition:
    background-color 160ms ease,
    border-color 160ms ease,
    box-shadow 160ms ease,
    transform 160ms ease;
}

.external-agents__profile-card:hover {
  background: rgba(248, 250, 252, 0.98);
  border-color: rgba(203, 213, 225, 0.98);
}

.external-agents__profile-card--healthy {
  border-color: rgba(187, 247, 208, 0.92);
}

.external-agents__profile-card--warning {
  border-color: rgba(253, 230, 138, 0.92);
}

.external-agents__profile-card--error {
  border-color: rgba(254, 202, 202, 0.94);
}

.external-agents__profile-card--active {
  background: rgba(241, 245, 249, 0.88);
  border-color: rgba(203, 213, 225, 0.98);
}

.external-agents__profile-heading {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.28rem;
}

.external-agents__profile-top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.24rem;
}

.external-agents__profile-meta {
  font-size: 0.72rem;
  line-height: 1.35;
}

.external-agents__profile-meta--healthy {
  color: rgba(21, 128, 61, 0.94);
}

.external-agents__profile-meta--warning {
  color: rgba(180, 83, 9, 0.94);
}

.external-agents__profile-meta--error {
  color: rgba(185, 28, 28, 0.94);
}

.external-agents__profile-summary {
  display: -webkit-box;
  overflow: hidden;
  text-overflow: ellipsis;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.external-agents__profile-placeholder {
  margin: 0;
  flex-shrink: 0;
  color: rgba(100, 116, 139, 0.9);
  font-size: 0.74rem;
  line-height: 1.35;
  white-space: nowrap;
}

.external-agents__protocol-switch {
  display: inline-grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.22rem;
}

.external-agents__protocol-switch-button {
  min-width: 5rem;
  padding: 0.22rem 0.42rem;
  font-size: 0.82rem;
  font-weight: 700;
  line-height: 1.35;
  cursor: pointer;
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
  font-size: 0.82rem;
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
  font-size: 0.82rem;
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
  padding-top: 0.5rem;
  border-top: 1px solid rgba(226, 232, 240, 0.96);
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

  .external-agents__panel--sidebar {
    padding-right: 0;
    padding-bottom: 0.72rem;
    border-right: 0;
    border-bottom: 1px solid rgba(226, 232, 240, 0.96);
  }

  .external-agents__panel--editor {
    padding-left: 0;
  }
}

@media (max-width: 760px) {
  .external-agents__toolbar,
  .external-agents__editor-head,
  .external-agents__locked {
    flex-direction: column;
    align-items: stretch;
  }

  .external-agents__actions,
  .external-agents__form-grid,
  .external-agents__protocol-switch {
    grid-template-columns: 1fr;
  }

  .external-agents__actions {
    display: grid;
  }

  .external-agents__profile-top,
  .external-agents__profile-heading {
    flex-direction: column;
    align-items: flex-start;
  }

  .external-agents__profile-placeholder,
  .external-agents__badge--status {
    white-space: normal;
  }

  .external-agents__field--full {
    grid-column: auto;
  }
}

:global(.dark) .external-agents__notice,
:global(.dark) .external-agents__empty,
:global(.dark) .external-agents__locked,
:global(.dark) .external-agents__protocol-switch-button,
:global(.dark) .external-agents__input,
:global(.dark) .external-agents__textarea {
  background: rgba(15, 23, 42, 0.76);
  color: rgba(226, 232, 240, 0.94);
}

:global(.dark) .external-agents__notice,
:global(.dark) .external-agents__protocol-switch-button,
:global(.dark) .external-agents__input,
:global(.dark) .external-agents__textarea {
  border-color: rgba(71, 85, 105, 0.86);
}

:global(.dark) .external-agents__empty,
:global(.dark) .external-agents__locked {
  background: rgba(15, 23, 42, 0.48);
}

:global(.dark) .external-agents__panel--sidebar,
:global(.dark) .external-agents__advanced {
  border-color: rgba(51, 65, 85, 0.86);
}

:global(.dark) .external-agents__editor-title,
:global(.dark) .external-agents__profile-name,
:global(.dark) .external-agents__advanced-summary,
:global(.dark) .external-agents__field > span {
  color: rgba(241, 245, 249, 0.95);
}

:global(.dark) .external-agents__editor-meta,
:global(.dark) .external-agents__profile-meta,
:global(.dark) .external-agents__profile-summary,
:global(.dark) .external-agents__profile-placeholder,
:global(.dark) .external-agents__locked p,
:global(.dark) .external-agents__empty {
  color: rgba(203, 213, 225, 0.88);
}

:global(.dark) .external-agents__button,
:global(.dark) .external-agents__badge {
  background: rgba(30, 41, 59, 0.94);
  border-color: rgba(71, 85, 105, 0.92);
  color: rgba(226, 232, 240, 0.92);
}

:global(.dark) .external-agents__button:hover,
:global(.dark) .external-agents__protocol-switch-button:hover,
:global(.dark) .external-agents__protocol-switch-button--active {
  background: rgba(30, 41, 59, 0.9);
}

:global(.dark) .external-agents__profile-card {
  background: rgba(15, 23, 42, 0.5);
  border-color: rgba(51, 65, 85, 0.82);
}

:global(.dark) .external-agents__profile-card:hover {
  background: rgba(15, 23, 42, 0.68);
  border-color: rgba(71, 85, 105, 0.88);
}

:global(.dark) .external-agents__profile-card--active {
  background: rgba(55, 65, 81, 0.2);
  border-color: rgba(75, 85, 99, 0.88);
}

:global(.dark) .external-agents__profile-card--healthy {
  border-color: rgba(34, 197, 94, 0.4);
}

:global(.dark) .external-agents__profile-card--warning {
  border-color: rgba(245, 158, 11, 0.4);
}

:global(.dark) .external-agents__profile-card--error {
  border-color: rgba(248, 113, 113, 0.42);
}

:global(.dark) .external-agents__button--primary,
:global(.dark) .external-agents__button--primary:hover {
  background: rgba(var(--settings-accent, 37, 99, 235), 0.9);
  border-color: rgba(var(--settings-accent, 37, 99, 235), 0.3);
  color: #ffffff;
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

:global(.dark) .external-agents__advanced-summary::after {
  color: rgba(203, 213, 225, 0.88);
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

:global(.dark) .external-agents__profile-meta--healthy {
  color: rgba(187, 247, 208, 0.94);
}

:global(.dark) .external-agents__profile-meta--warning {
  color: rgba(253, 224, 71, 0.94);
}

:global(.dark) .external-agents__profile-meta--error {
  color: rgba(254, 202, 202, 0.94);
}
</style>
