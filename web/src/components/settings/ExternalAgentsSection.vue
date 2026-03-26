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

const draft = ref<ProfileDraft>(createDraft('acp'))

const selectedProfile = computed(
  () => profiles.value.find((profile) => profile.id === selectedProfileID.value) ?? null
)
const isBuiltinSelection = computed(() => selectedProfile.value?.builtin === true)
const currentSelectionLabel = computed(() => {
  if (selectedProfile.value) {
    return selectedProfile.value.title || selectedProfile.value.name
  }
  return (
    draft.value.title || draft.value.name || t('settings.externalAgents.newProfile', 'New profile')
  )
})

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

function selectProfile(profileID: string) {
  const profile = profiles.value.find((item) => item.id === profileID)
  if (!profile) {
    return
  }
  selectedProfileID.value = profile.id
  draft.value = profileToDraft(profile)
}

function startNewProfile(protocol: ProtocolKind) {
  selectedProfileID.value = ''
  draft.value = createDraft(protocol)
  clearNotice()
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
  showNotice(
    t('settings.externalAgents.duplicateReady', 'Profile copied into a new editable draft.'),
    'info'
  )
}

function parseCommandText(value: string): string[] {
  return value
    .split('\n')
    .map((part) => part.trim())
    .filter(Boolean)
}

function parseRecordText(value: string, label: string): Record<string, string> | undefined {
  const trimmed = value.trim()
  if (!trimmed) {
    return undefined
  }
  let parsed: unknown
  try {
    parsed = JSON.parse(trimmed)
  } catch {
    throw new Error(`${label} must be valid JSON`)
  }
  if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') {
    throw new Error(`${label} must be a JSON object`)
  }
  return Object.fromEntries(
    Object.entries(parsed as Record<string, unknown>).map(([key, item]) => [key, String(item)])
  )
}

function parseMetadataText(value: string): Record<string, unknown> | undefined {
  const trimmed = value.trim()
  if (!trimmed) {
    return undefined
  }
  let parsed: unknown
  try {
    parsed = JSON.parse(trimmed)
  } catch {
    throw new Error('Metadata must be valid JSON')
  }
  if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') {
    throw new Error('Metadata must be a JSON object')
  }
  return parsed as Record<string, unknown>
}

function buildProfilePayload(): AgentProfile {
  const name = draft.value.name.trim()
  if (!name) {
    throw new Error('Profile name is required')
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
      throw new Error('ACP profiles need a command and arguments')
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
    throw new Error('A2A profiles need an endpoint URL or agent card URL')
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
    } else if (profiles.value.length === 0) {
      startNewProfile('acp')
    }
  } catch (error) {
    showNotice(
      (error as { response?: { data?: { error?: string } } })?.response?.data?.error ||
        t('settings.externalAgents.loadFailed', 'Failed to load external agent profiles.'),
      'error'
    )
  } finally {
    loading.value = false
  }
}

function summarizeProfile(profile: AgentProfile): string {
  if (profile.protocol === 'acp') {
    return (profile.command || []).join(' ')
  }
  return (
    profile.card_url ||
    profile.endpoint_url ||
    t('settings.externalAgents.remoteAgent', 'Remote agent')
  )
}

function formatTimestamp(value?: string): string {
  if (!value) {
    return t('settings.externalAgents.notAvailable', 'Not available')
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  return date.toLocaleString()
}

function statusLabel(profile: AgentProfile): string {
  if (profile.health_status) {
    return profile.health_status
  }
  return profile.builtin
    ? t('settings.externalAgents.ready', 'Ready')
    : t('settings.externalAgents.custom', 'Custom')
}

async function saveDraft() {
  if (isBuiltinSelection.value) {
    showNotice(
      t(
        'settings.externalAgents.builtinLocked',
        'Built-in profiles are read-only. Duplicate one to customize it.'
      ),
      'info'
    )
    return
  }
  saving.value = true
  try {
    const payload = buildProfilePayload()
    const response = await agentSessionsApi.saveProfile(payload)
    await loadProfiles(response.data.id)
    showNotice(t('settings.externalAgents.saved', 'External agent profile saved.'), 'success')
  } catch (error) {
    showNotice(
      (error as Error)?.message ||
        (error as { response?: { data?: { error?: string } } })?.response?.data?.error ||
        t('settings.externalAgents.saveFailed', 'Failed to save external agent profile.'),
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
    return t('settings.externalAgents.verifyOk', 'Profile verification succeeded.')
  }
  return t('settings.externalAgents.verifyFailed', 'Profile verification failed.')
}

function healthMessage(result: ProfileHealthResult): string {
  if (result.message) {
    return result.message
  }
  if (result.healthy) {
    return t('settings.externalAgents.healthOk', 'Health check succeeded.')
  }
  return t('settings.externalAgents.healthFailed', 'Health check failed.')
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
      (error as Error)?.message ||
        (error as { response?: { data?: { error?: string } } })?.response?.data?.error ||
        t('settings.externalAgents.verifyFailed', 'Failed to verify external agent profile.'),
      'error'
    )
  } finally {
    verifying.value = false
  }
}

async function selectAndVerify(profileID: string) {
  selectProfile(profileID)
  await verifyCurrent()
}

async function checkCurrentHealth() {
  if (!selectedProfile.value) {
    showNotice(
      t(
        'settings.externalAgents.saveBeforeHealth',
        'Save the profile first before running a health check.'
      ),
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
      (error as Error)?.message ||
        (error as { response?: { data?: { error?: string } } })?.response?.data?.error ||
        t('settings.externalAgents.healthFailed', 'Failed to check external agent health.'),
      'error'
    )
  } finally {
    checkingHealth.value = false
  }
}

async function selectAndCheckHealth(profileID: string) {
  selectProfile(profileID)
  await checkCurrentHealth()
}

onMounted(() => {
  void loadProfiles()
})
</script>

<template>
  <div class="external-agents" data-testid="external-agents-section">
    <div class="external-agents__header">
      <div>
        <h3 class="external-agents__title">
          {{ t('settings.externalAgents.title', 'External Agents') }}
        </h3>
        <p class="external-agents__copy">
          {{
            t(
              'settings.externalAgents.description',
              'Manage ACP and A2A runtime profiles for local coding CLIs and remote agent cards.'
            )
          }}
        </p>
      </div>

      <div class="external-agents__toolbar">
        <button
          type="button"
          class="external-agents__button external-agents__button--quiet"
          data-testid="external-agents-new-acp"
          @click="startNewProfile('acp')"
        >
          {{ t('settings.externalAgents.newAcp', 'New ACP') }}
        </button>
        <button
          type="button"
          class="external-agents__button external-agents__button--quiet"
          data-testid="external-agents-new-a2a"
          @click="startNewProfile('a2a')"
        >
          {{ t('settings.externalAgents.newA2a', 'New A2A') }}
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

    <div class="external-agents__layout">
      <section class="external-agents__panel external-agents__panel--profiles">
        <div class="external-agents__panel-header">
          <div>
            <p class="external-agents__eyebrow">
              {{ t('settings.externalAgents.availableProfiles', 'Available Profiles') }}
            </p>
            <h4 class="external-agents__panel-title">
              {{ t('settings.externalAgents.profiles', 'Profiles') }}
            </h4>
          </div>
          <span class="external-agents__count">{{ profiles.length }}</span>
        </div>

        <div v-if="loading" class="external-agents__empty">
          {{ t('common.loading', 'Loading') }}
        </div>
        <div v-else-if="profiles.length === 0" class="external-agents__empty">
          {{ t('settings.externalAgents.empty', 'No external agent profiles yet.') }}
        </div>
        <div v-else class="external-agents__profile-list">
          <button
            v-for="profile in profiles"
            :key="profile.id"
            type="button"
            class="external-agents__profile-card"
            :class="{ 'external-agents__profile-card--active': selectedProfileID === profile.id }"
            data-testid="external-agents-profile-card"
            @click="selectProfile(profile.id)"
          >
            <div class="external-agents__profile-row">
              <div class="external-agents__profile-main">
                <div class="external-agents__profile-name-row">
                  <strong>{{ profile.title || profile.name }}</strong>
                  <span class="external-agents__badge">{{ profile.protocol.toUpperCase() }}</span>
                  <span
                    v-if="profile.builtin"
                    class="external-agents__badge external-agents__badge--builtin"
                  >
                    {{ t('settings.externalAgents.builtin', 'Built-in') }}
                  </span>
                </div>
                <p class="external-agents__profile-summary">{{ summarizeProfile(profile) }}</p>
                <p class="external-agents__profile-meta">
                  {{ t('settings.externalAgents.status', 'Status') }}: {{ statusLabel(profile) }}
                </p>
              </div>

              <div class="external-agents__profile-actions">
                <button
                  type="button"
                  class="external-agents__mini-button"
                  :disabled="verifying"
                  @click.stop="selectAndVerify(profile.id)"
                >
                  {{ t('settings.externalAgents.verify', 'Verify') }}
                </button>
                <button
                  type="button"
                  class="external-agents__mini-button"
                  :disabled="checkingHealth"
                  @click.stop="selectAndCheckHealth(profile.id)"
                >
                  {{ t('settings.externalAgents.health', 'Health') }}
                </button>
              </div>
            </div>
          </button>
        </div>
      </section>

      <section class="external-agents__panel external-agents__panel--editor">
        <div class="external-agents__panel-header">
          <div>
            <p class="external-agents__eyebrow">
              {{ t('settings.externalAgents.editor', 'Profile Editor') }}
            </p>
            <h4 class="external-agents__panel-title">{{ currentSelectionLabel }}</h4>
          </div>
          <div class="external-agents__panel-actions">
            <button
              v-if="selectedProfile"
              type="button"
              class="external-agents__button external-agents__button--quiet"
              @click="duplicateSelection"
            >
              {{ t('settings.externalAgents.duplicate', 'Duplicate') }}
            </button>
            <button
              type="button"
              class="external-agents__button"
              data-testid="external-agents-save"
              :disabled="saving || isBuiltinSelection"
              @click="saveDraft"
            >
              {{ t('common.save', 'Save') }}
            </button>
          </div>
        </div>

        <div v-if="isBuiltinSelection" class="external-agents__locked">
          {{
            t(
              'settings.externalAgents.builtinHelp',
              'Built-in profiles stay read-only so the seeded Claude, Codex, Gemini, and generic A2A entries remain stable.'
            )
          }}
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

          <label class="external-agents__field">
            <span>{{ t('common.id', 'ID') }}</span>
            <input
              v-model="draft.id"
              class="external-agents__input"
              :disabled="isBuiltinSelection"
            />
          </label>

          <label class="external-agents__field">
            <span>{{ t('settings.externalAgents.protocol', 'Protocol') }}</span>
            <select
              v-model="draft.protocol"
              class="external-agents__input"
              data-testid="external-agents-protocol"
              :disabled="isBuiltinSelection"
            >
              <option value="acp">ACP</option>
              <option value="a2a">A2A</option>
            </select>
          </label>

          <label class="external-agents__field">
            <span>{{ t('settings.externalAgents.credentialSource', 'Credential Source') }}</span>
            <input
              v-model="draft.credentialProviderId"
              class="external-agents__input"
              :disabled="isBuiltinSelection"
              placeholder="openai-codex"
            />
          </label>

          <label v-if="draft.protocol === 'acp'" class="external-agents__field">
            <span>{{ t('settings.externalAgents.authMethod', 'Auth Method') }}</span>
            <input
              v-model="draft.authMethodId"
              class="external-agents__input"
              :disabled="isBuiltinSelection"
              placeholder="oauth"
            />
          </label>

          <label
            v-if="draft.protocol === 'acp'"
            class="external-agents__field external-agents__field--full"
          >
            <span>{{ t('settings.externalAgents.command', 'Command and Args') }}</span>
            <textarea
              v-model="draft.commandText"
              class="external-agents__textarea"
              data-testid="external-agents-command"
              :disabled="isBuiltinSelection"
              placeholder="npx&#10;-y&#10;@zed-industries/codex-acp"
            />
          </label>

          <label v-if="draft.protocol === 'acp'" class="external-agents__field">
            <span>{{ t('settings.externalAgents.cwd', 'Working Directory') }}</span>
            <input
              v-model="draft.cwd"
              class="external-agents__input"
              :disabled="isBuiltinSelection"
              placeholder="/workspace/project"
            />
          </label>

          <label
            v-if="draft.protocol === 'acp'"
            class="external-agents__field external-agents__field--full"
          >
            <span>{{ t('settings.externalAgents.environment', 'Environment Overrides') }}</span>
            <textarea
              v-model="draft.envText"
              class="external-agents__textarea"
              :disabled="isBuiltinSelection"
              placeholder='{&#10;  "NODE_ENV": "development"&#10;}'
            />
          </label>

          <label v-if="draft.protocol === 'a2a'" class="external-agents__field">
            <span>{{ t('settings.externalAgents.endpoint', 'Endpoint URL') }}</span>
            <input
              v-model="draft.endpointUrl"
              class="external-agents__input"
              data-testid="external-agents-endpoint"
              :disabled="isBuiltinSelection"
              placeholder="https://agent.example.com/rpc"
            />
          </label>

          <label v-if="draft.protocol === 'a2a'" class="external-agents__field">
            <span>{{ t('settings.externalAgents.cardUrl', 'Agent Card URL') }}</span>
            <input
              v-model="draft.cardUrl"
              class="external-agents__input"
              data-testid="external-agents-card-url"
              :disabled="isBuiltinSelection"
              placeholder="https://agent.example.com/.well-known/agent-card.json"
            />
          </label>

          <label
            v-if="draft.protocol === 'a2a'"
            class="external-agents__field external-agents__field--full"
          >
            <span>{{ t('settings.externalAgents.headers', 'Request Headers') }}</span>
            <textarea
              v-model="draft.headersText"
              class="external-agents__textarea"
              :disabled="isBuiltinSelection"
              placeholder='{&#10;  "Authorization": "Bearer ..."&#10;}'
            />
          </label>

          <label class="external-agents__field external-agents__field--full">
            <span>{{ t('settings.externalAgents.metadata', 'Metadata') }}</span>
            <textarea
              v-model="draft.metadataText"
              class="external-agents__textarea"
              :disabled="isBuiltinSelection"
              placeholder='{&#10;  "team": "blue"&#10;}'
            />
          </label>
        </div>

        <div class="external-agents__footer">
          <div class="external-agents__timestamps">
            <span>
              {{ t('settings.externalAgents.lastVerified', 'Last verified') }}:
              {{ formatTimestamp(selectedProfile?.last_verified_at) }}
            </span>
            <span>
              {{ t('settings.externalAgents.lastHealth', 'Last health check') }}:
              {{ formatTimestamp(selectedProfile?.last_health_at) }}
            </span>
          </div>

          <div class="external-agents__footer-actions">
            <button
              type="button"
              class="external-agents__button external-agents__button--quiet"
              data-testid="external-agents-verify-current"
              :disabled="verifying"
              @click="verifyCurrent"
            >
              {{ t('settings.externalAgents.verify', 'Verify') }}
            </button>
            <button
              type="button"
              class="external-agents__button external-agents__button--quiet"
              data-testid="external-agents-health-current"
              :disabled="checkingHealth"
              @click="checkCurrentHealth"
            >
              {{ t('settings.externalAgents.health', 'Health') }}
            </button>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.external-agents {
  display: grid;
  gap: 1rem;
  width: 100%;
  padding: 1.25rem;
}

.external-agents__header,
.external-agents__panel-header,
.external-agents__profile-row,
.external-agents__footer {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
}

.external-agents__title,
.external-agents__panel-title {
  margin: 0;
  font-size: 1rem;
  font-weight: 700;
  color: rgba(15, 23, 42, 0.92);
}

.external-agents__copy,
.external-agents__profile-summary,
.external-agents__profile-meta,
.external-agents__locked,
.external-agents__notice,
.external-agents__timestamps {
  margin: 0.35rem 0 0;
  color: rgba(71, 85, 105, 0.9);
  font-size: 0.92rem;
  line-height: 1.45;
}

.external-agents__eyebrow {
  margin: 0 0 0.25rem;
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: rgba(14, 116, 144, 0.85);
}

.external-agents__toolbar,
.external-agents__panel-actions,
.external-agents__footer-actions,
.external-agents__profile-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.external-agents__layout {
  display: grid;
  grid-template-columns: minmax(0, 0.95fr) minmax(0, 1.35fr);
  gap: 1rem;
}

.external-agents__panel {
  display: grid;
  gap: 1rem;
  padding: 1rem;
  border-radius: 1rem;
  border: 1px solid rgba(148, 163, 184, 0.22);
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.92), rgba(248, 250, 252, 0.88)),
    rgba(255, 255, 255, 0.76);
}

.external-agents__count,
.external-agents__badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0.18rem 0.5rem;
  border-radius: 999px;
  background: rgba(14, 116, 144, 0.12);
  color: rgba(14, 116, 144, 0.92);
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.04em;
}

.external-agents__badge--builtin {
  background: rgba(15, 23, 42, 0.08);
  color: rgba(15, 23, 42, 0.78);
}

.external-agents__profile-list {
  display: grid;
  gap: 0.75rem;
}

.external-agents__profile-card {
  display: block;
  width: 100%;
  padding: 0.9rem 1rem;
  text-align: left;
  border: 1px solid rgba(148, 163, 184, 0.22);
  border-radius: 0.9rem;
  background: rgba(255, 255, 255, 0.88);
  transition:
    border-color 160ms ease,
    transform 160ms ease,
    box-shadow 160ms ease;
}

.external-agents__profile-card:hover,
.external-agents__profile-card--active {
  border-color: rgba(14, 116, 144, 0.4);
  transform: translateY(-1px);
  box-shadow: 0 10px 24px rgba(14, 116, 144, 0.08);
}

.external-agents__profile-main {
  min-width: 0;
}

.external-agents__profile-name-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
  align-items: center;
}

.external-agents__button,
.external-agents__mini-button {
  border: 0;
  border-radius: 999px;
  background: linear-gradient(135deg, rgba(8, 145, 178, 0.92), rgba(14, 116, 144, 0.94));
  color: #fff;
  font-size: 0.86rem;
  font-weight: 600;
  padding: 0.58rem 0.95rem;
  cursor: pointer;
}

.external-agents__mini-button {
  padding: 0.42rem 0.72rem;
  font-size: 0.8rem;
}

.external-agents__button--quiet {
  background: rgba(14, 116, 144, 0.08);
  color: rgba(14, 116, 144, 0.92);
}

.external-agents__button:disabled,
.external-agents__mini-button:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.external-agents__notice,
.external-agents__locked,
.external-agents__empty {
  padding: 0.8rem 0.95rem;
  border-radius: 0.85rem;
  border: 1px solid rgba(148, 163, 184, 0.22);
  background: rgba(248, 250, 252, 0.88);
}

.external-agents__notice--success {
  border-color: rgba(22, 163, 74, 0.22);
  background: rgba(240, 253, 244, 0.9);
  color: rgba(21, 128, 61, 0.95);
}

.external-agents__notice--error {
  border-color: rgba(220, 38, 38, 0.2);
  background: rgba(254, 242, 242, 0.92);
  color: rgba(185, 28, 28, 0.94);
}

.external-agents__form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.9rem;
}

.external-agents__field {
  display: grid;
  gap: 0.45rem;
  min-width: 0;
}

.external-agents__field > span {
  font-size: 0.8rem;
  font-weight: 600;
  color: rgba(30, 41, 59, 0.88);
}

.external-agents__field--full {
  grid-column: 1 / -1;
}

.external-agents__input,
.external-agents__textarea {
  width: 100%;
  border-radius: 0.8rem;
  border: 1px solid rgba(148, 163, 184, 0.28);
  background: rgba(255, 255, 255, 0.96);
  color: rgba(15, 23, 42, 0.96);
  padding: 0.72rem 0.82rem;
  font: inherit;
}

.external-agents__textarea {
  min-height: 7rem;
  resize: vertical;
}

.external-agents__timestamps {
  display: grid;
  gap: 0.2rem;
  margin-top: 0;
}

@media (max-width: 980px) {
  .external-agents__layout {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 720px) {
  .external-agents {
    padding: 1rem;
  }

  .external-agents__form-grid {
    grid-template-columns: 1fr;
  }

  .external-agents__field--full {
    grid-column: auto;
  }

  .external-agents__header,
  .external-agents__panel-header,
  .external-agents__profile-row,
  .external-agents__footer {
    flex-direction: column;
  }
}

:global(.dark) .external-agents__panel,
:global(.dark) .external-agents__profile-card,
:global(.dark) .external-agents__notice,
:global(.dark) .external-agents__locked,
:global(.dark) .external-agents__empty,
:global(.dark) .external-agents__input,
:global(.dark) .external-agents__textarea {
  background: rgba(15, 23, 42, 0.72);
  border-color: rgba(148, 163, 184, 0.18);
  color: rgba(226, 232, 240, 0.94);
}

:global(.dark) .external-agents__title,
:global(.dark) .external-agents__panel-title,
:global(.dark) .external-agents__field > span {
  color: rgba(241, 245, 249, 0.94);
}

:global(.dark) .external-agents__copy,
:global(.dark) .external-agents__profile-summary,
:global(.dark) .external-agents__profile-meta,
:global(.dark) .external-agents__locked,
:global(.dark) .external-agents__timestamps {
  color: rgba(203, 213, 225, 0.86);
}

:global(.dark) .external-agents__button--quiet {
  background: rgba(103, 232, 249, 0.12);
  color: rgba(165, 243, 252, 0.92);
}

:global(.dark) .external-agents__badge--builtin {
  background: rgba(226, 232, 240, 0.12);
  color: rgba(226, 232, 240, 0.9);
}
</style>
