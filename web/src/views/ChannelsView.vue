<script setup lang="ts">
import {
  ref,
  computed,
  onMounted,
  onUnmounted,
  watch,
  shallowRef,
  triggerRef,
  onErrorCaptured,
} from 'vue'
import { useI18n } from 'vue-i18n'
import { useSettingsStore } from '@/stores/settings'
import { channelsApi } from '@/api/channels'
import { getChannelIconOrDefault, getChannelIconStyleVars } from '@/utils/channelIcons'
import { getErrorMessage } from '@/utils/error'
import { publicAsset } from '@/utils/publicAsset'
import ChannelCard from '@/components/channels/ChannelCard.vue'
import ChannelCardShell from '@/components/channels/ChannelCardShell.vue'
import ChannelDetailPanel from '@/components/channels/ChannelDetailPanel.vue'
import RemoteAccessDetailPanel from '@/components/remote-access/RemoteAccessDetailPanel.vue'
import {
  createWeChatILinkSetupSession,
  getWeChatILinkSetupSession,
  type WeChatILinkSetupSessionResponse,
} from '@/api/wechat-ilink-setup'
import {
  getRemoteAccessStatus,
  startRemoteAccess,
  stopRemoteAccess,
  getTunnelProviders,
  getRemoteAccessConfig,
  updateRemoteAccessConfig,
  type TunnelStatus as TunnelStatusType,
  type TunnelProvider,
} from '@/api/remote-access'

const { t, locale: i18nLocale } = useI18n()
const settingsStore = useSettingsStore()

interface ChannelFieldDef {
  key: string
  labelKey: string
  type: 'text' | 'password' | 'tel' | 'url' | 'textarea' | 'toggle' | 'select'
  placeholder?: string
  placeholderKey?: string
  value: string
  required?: boolean
  options?: Array<{
    value: string
    labelKey?: string
    label?: string
  }>
  visibleWhen?: {
    fieldKey: string
    value: string
  }
}

interface ChannelDef {
  id: string
  nameKey?: string
  name?: string
  icon: string
  enabled: boolean
  status: 'connected' | 'disconnected' | 'error' | 'connecting'
  lastError?: string
  lastErrorKey?: string
  descriptionKey: string
  hintKey?: string
  docUrl?: string
  fields: ChannelFieldDef[]
  // Message statistics
  messagesReceived?: number
  messagesSent?: number
  lastMessageAt?: string
  lastReplyAt?: string
}

type GroupAccessPolicy = 'open' | 'allowlist' | 'disabled'
type GroupMentionPolicy = 'mentioned' | 'always'
type RemoteAccessState = 'loading' | 'ready' | 'connecting' | 'connected' | 'error'

const REMOTE_ACCESS_PANEL_ID = '__remote_access__'

interface ChannelSettingsResponse {
  group_access?: {
    policy?: GroupAccessPolicy
    mention_policy?: GroupMentionPolicy
    allowed_chat_ids?: Record<string, string[]>
  }
}

const loading = ref(false)
const saving = ref<string | null>(null)
const toggling = ref<string | null>(null)
const testingConnection = ref<string | null>(null)
const testResult = ref<{ channelId: string; success: boolean; message: string } | null>(null)
const groupAccessPolicy = ref<GroupAccessPolicy>('open')
const groupAccessMentionPolicy = ref<GroupMentionPolicy>('mentioned')
const groupAccessAllowedChatIDsText = ref('')
const showGroupAccessModal = ref(false)
const groupAccessDraftPolicy = ref<GroupAccessPolicy>('open')
const groupAccessDraftMentionPolicy = ref<GroupMentionPolicy>('mentioned')
const groupAccessDraftAllowedChatIDsText = ref('')
const savingGroupAccess = ref(false)
const groupAccessResult = ref<{ success: boolean; message: string } | null>(null)
const channelLoadError = ref<string | null>(null)
const pageRuntimeError = ref<string | null>(null)
const wechatILinkSetupOpen = ref(false)
const wechatILinkSetupPending = ref(false)
const wechatILinkSetupError = ref('')
const wechatILinkSetupSession = ref<WeChatILinkSetupSessionResponse | null>(null)
let wechatILinkSetupPoller: ReturnType<typeof setInterval> | null = null
const wechatILinkSetupFailureStatuses = new Set(['error', 'expired'])

function getWeChatILinkSetupStatusLabel(status?: string): string {
  switch ((status || '').trim()) {
    case 'pending':
      return t('channels.wechatILinkSetupStatePending')
    case 'authorizing':
      return t('channels.wechatILinkSetupStateAuthorizing')
    case 'configuring':
      return t('channels.wechatILinkSetupStateConfiguring')
    case 'connected':
      return t('channels.wechatILinkSetupStateConnected')
    case 'error':
      return t('channels.wechatILinkSetupStateError')
    case 'expired':
      return t('channels.wechatILinkSetupStateExpired')
    default:
      return status?.trim() || t('channels.statusConnecting')
  }
}

const canRetryWeChatILinkSetup = computed(() => {
  if (wechatILinkSetupPending.value) return false
  if (wechatILinkSetupError.value) return true
  const status = wechatILinkSetupSession.value?.status || ''
  return wechatILinkSetupFailureStatuses.has(status)
})

const hasWeChatILinkSetupStatus = computed(() =>
  Boolean((wechatILinkSetupSession.value?.status || '').trim())
)

function isChannelFieldVisible(channel: ChannelDef, field: ChannelFieldDef): boolean {
  if (!field.visibleWhen) return true
  const driver = channel.fields.find((candidate) => candidate.key === field.visibleWhen?.fieldKey)
  return (driver?.value || '') === field.visibleWhen.value
}

function getVisibleChannelFields(channel: ChannelDef): ChannelFieldDef[] {
  return channel.fields.filter((field) => isChannelFieldVisible(channel, field))
}

// Channel/runtime errors already include display-ready server text.
function resolveChannelError(channel: ChannelDef): string {
  return channel.lastError || ''
}

function normalizeGroupAccessPolicy(value: unknown): GroupAccessPolicy {
  switch (value) {
    case 'allowlist':
    case 'disabled':
      return value
    default:
      return 'open'
  }
}

function normalizeGroupMentionPolicy(value: unknown): GroupMentionPolicy {
  switch (value) {
    case 'always':
      return value
    default:
      return 'mentioned'
  }
}

function formatAllowedChatIDs(allowed?: Record<string, string[]>): string {
  if (!allowed) return ''

  const lines: string[] = []
  for (const channelName of Object.keys(allowed).sort()) {
    const chatIDs = [...(allowed[channelName] || [])].sort()
    for (const chatID of chatIDs) {
      if (!chatID) continue
      lines.push(`${channelName}:${chatID}`)
    }
  }
  return lines.join('\n')
}

function parseAllowedChatIDs(raw: string): {
  allowed_chat_ids: Record<string, string[]>
  error?: string
} {
  const allowed: Record<string, string[]> = {}

  for (const line of raw.split('\n')) {
    const trimmed = line.trim()
    if (!trimmed) continue

    const separatorIndex = trimmed.indexOf(':')
    if (separatorIndex <= 0 || separatorIndex >= trimmed.length - 1) {
      return { allowed_chat_ids: {}, error: t('channels.groupAccessFormatError') }
    }

    const channelName = trimmed.slice(0, separatorIndex).trim()
    const chatID = trimmed.slice(separatorIndex + 1).trim()
    if (!channelName || !chatID) {
      return { allowed_chat_ids: {}, error: t('channels.groupAccessFormatError') }
    }

    const existing = allowed[channelName] || []
    if (!existing.includes(chatID)) {
      allowed[channelName] = [...existing, chatID]
    }
  }

  return { allowed_chat_ids: allowed }
}

function countAllowedChatEntries(raw: string): number {
  return raw
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean).length
}

function syncGroupAccessDraftFromCurrent() {
  groupAccessDraftPolicy.value = groupAccessPolicy.value
  groupAccessDraftMentionPolicy.value = groupAccessMentionPolicy.value
  groupAccessDraftAllowedChatIDsText.value = groupAccessAllowedChatIDsText.value
}

function showGroupAccessResult(success: boolean, message: string) {
  groupAccessResult.value = { success, message }
  window.setTimeout(() => {
    if (groupAccessResult.value?.message === message) {
      groupAccessResult.value = null
    }
  }, 3000)
}

function isSuccessfulStatus(status: number): boolean {
  return status >= 200 && status < 300
}

function formatFetchFailureMessage(
  response: { status?: number },
  fallback: string,
  message?: string
): string {
  const status = Number(response.status) || 0
  if (status === 401) {
    return 'Channels request was rejected by the server (401).'
  }
  if (message && message.trim()) {
    return message
  }
  return status > 0 ? `${fallback} (${status})` : fallback
}

const pageErrorMessage = computed(() => pageRuntimeError.value || channelLoadError.value)

// Start collapsed; the primary section still keeps locale favorites,
// self-hosted channels, and any enabled channels visible by default.
const getInitialShowMoreState = (): boolean => {
  return false
}

const showMoreChannels = ref(getInitialShowMoreState())

// Localized channel ordering based on user's language/region
const getLocalizedChannelOrder = (): string[] => {
  // Priority 1: Use backend settings locale, fallback to current i18n locale
  const locale = settingsStore.backendSettings?.locale || i18nLocale.value || 'en-US'

  // Chinese regions (Mainland China)
  if (locale === 'zh-CN') {
    return ['wechat', 'wechat_ilink', 'dingtalk', 'feishu', 'qq', 'telegram', 'imessage']
  }

  // Taiwan region
  if (locale === 'zh-TW') {
    return ['line', 'telegram', 'imessage', 'instagram', 'messenger', 'discord']
  }

  // Japanese region
  if (locale === 'ja-JP') {
    return ['line', 'telegram', 'imessage', 'twitter', 'instagram', 'discord']
  }

  // Korean region
  if (locale === 'ko-KR') {
    return ['telegram', 'instagram', 'imessage', 'twitter', 'line', 'discord']
  }

  // Default (Western/International)
  return ['whatsapp', 'telegram', 'imessage', 'messenger', 'instagram', 'discord']
}

const alwaysVisibleChannelIds = ['mattermost', 'nextcloudtalk'] as const
const defaultVisibleChannelCount = 5

// Primary channels shown by default - locale favorites, self-hosted channels,
// plus any channels the user already enabled. The list itself is trimmed to 5 items.
const primaryChannelIds = computed(() => {
  const seen = new Set<string>()
  const ordered: string[] = []

  for (const id of getLocalizedChannelOrder()) {
    if (!seen.has(id)) {
      seen.add(id)
      ordered.push(id)
    }
  }

  for (const id of alwaysVisibleChannelIds) {
    if (!seen.has(id)) {
      seen.add(id)
      ordered.push(id)
    }
  }

  for (const channel of channelDefs.value) {
    if (channel.enabled && !seen.has(channel.id)) {
      seen.add(channel.id)
      ordered.push(channel.id)
    }
  }

  return ordered
})

const remoteAccessState = ref<RemoteAccessState>('loading')
const tunnelStatus = ref<TunnelStatusType | null>(null)
const remoteAccessError = ref<string | null>(null)
const tunnelProviders = ref<TunnelProvider[]>([])
const selectedProvider = ref<string>('localhost_run')
const ngrokAuthtoken = ref<string>('')
const ngrokDomain = ref<string>('')
const cloudflareToken = ref<string>('')
let statusInterval: ReturnType<typeof setInterval> | null = null

// When tunnelStatus gets URL (e.g. from polling), switch to connected
watch(
  () => tunnelStatus.value?.url,
  (url) => {
    if (url && remoteAccessState.value === 'connecting') {
      remoteAccessState.value = 'connected'
    }
  }
)

const channelDefs = shallowRef<ChannelDef[]>([
  {
    id: 'telegram',
    nameKey: 'channels.telegram',
    icon: getChannelIconOrDefault('telegram'),
    enabled: false,
    status: 'disconnected',
    descriptionKey: 'channels.telegramDesc',
    hintKey: 'channels.telegramHint',
    docUrl: 'https://core.telegram.org/bots#how-do-i-create-a-bot',
    fields: [
      {
        key: 'bot_token',
        labelKey: 'channels.botToken',
        type: 'password',
        placeholderKey: 'channels.placeholderBotToken',
        value: '',
        required: true,
      },
      {
        key: 'bot_username',
        labelKey: 'channels.botUsername',
        type: 'text',
        placeholderKey: 'channels.placeholderBotUsername',
        value: '',
      },
    ],
  },
  {
    id: 'discord',
    nameKey: 'channels.discord',
    icon: getChannelIconOrDefault('discord'),
    enabled: false,
    status: 'disconnected',
    descriptionKey: 'channels.discordDesc',
    hintKey: 'channels.discordHint',
    docUrl: 'https://discord.com/developers/docs/getting-started',
    fields: [
      {
        key: 'bot_token',
        labelKey: 'channels.botToken',
        type: 'password',
        placeholderKey: 'channels.placeholderDiscordBotToken',
        value: '',
        required: true,
      },
      {
        key: 'application_id',
        labelKey: 'channels.applicationId',
        type: 'text',
        placeholderKey: 'channels.placeholderApplicationId',
        value: '',
      },
    ],
  },
  {
    id: 'slack',
    nameKey: 'channels.slack',
    icon: getChannelIconOrDefault('slack'),
    enabled: false,
    status: 'disconnected',
    descriptionKey: 'channels.slackDesc',
    hintKey: 'channels.slackHint',
    docUrl: 'https://api.slack.com/start/quickstart',
    fields: [
      {
        key: 'bot_token',
        labelKey: 'channels.slackBotToken',
        type: 'password',
        placeholderKey: 'channels.placeholderSlackBotToken',
        value: '',
        required: true,
      },
      {
        key: 'app_token',
        labelKey: 'channels.slackAppToken',
        type: 'password',
        placeholderKey: 'channels.placeholderSlackAppToken',
        value: '',
        required: true,
      },
      {
        key: 'signing_secret',
        labelKey: 'channels.signingSecret',
        type: 'password',
        placeholderKey: 'channels.placeholderSigningSecret',
        value: '',
      },
    ],
  },
  {
    id: 'whatsapp',
    nameKey: 'channels.whatsapp',
    icon: getChannelIconOrDefault('whatsapp'),
    enabled: false,
    status: 'disconnected',
    descriptionKey: 'channels.whatsappDesc',
    hintKey: 'channels.whatsappHint',
    docUrl: 'https://developers.facebook.com/docs/whatsapp/cloud-api/get-started',
    fields: [
      {
        key: 'phone_number',
        labelKey: 'channels.phoneNumber',
        type: 'tel',
        placeholderKey: 'channels.placeholderPhoneNumber',
        value: '',
        required: true,
      },
    ],
  },
  {
    id: 'signal',
    nameKey: 'channels.signal',
    icon: getChannelIconOrDefault('signal'),
    enabled: false,
    status: 'disconnected',
    descriptionKey: 'channels.signalDesc',
    hintKey: 'channels.signalHint',
    docUrl: 'https://github.com/AsamK/signal-cli',
    fields: [
      {
        key: 'phone_number',
        labelKey: 'channels.phoneNumber',
        type: 'tel',
        placeholderKey: 'channels.placeholderPhoneNumber',
        value: '',
        required: true,
      },
    ],
  },
  {
    id: 'teams',
    nameKey: 'channels.teams',
    icon: getChannelIconOrDefault('teams'),
    enabled: false,
    status: 'disconnected',
    descriptionKey: 'channels.teamsDesc',
    hintKey: 'channels.teamsHint',
    docUrl:
      'https://learn.microsoft.com/en-us/microsoftteams/platform/bots/how-to/create-a-bot-for-teams',
    fields: [
      {
        key: 'app_id',
        labelKey: 'channels.appId',
        type: 'text',
        placeholderKey: 'channels.placeholderAppId',
        value: '',
        required: true,
      },
      {
        key: 'app_password',
        labelKey: 'channels.appPassword',
        type: 'password',
        placeholderKey: 'channels.placeholderAppPassword',
        value: '',
        required: true,
      },
      {
        key: 'tenant_id',
        labelKey: 'channels.tenantId',
        type: 'text',
        placeholderKey: 'channels.placeholderTenantId',
        value: '',
      },
    ],
  },
  {
    id: 'mattermost',
    nameKey: 'channels.mattermost',
    icon: getChannelIconOrDefault('mattermost'),
    enabled: false,
    status: 'disconnected',
    descriptionKey: 'channels.mattermostDesc',
    hintKey: 'channels.mattermostHint',
    docUrl: 'https://developers.mattermost.com/integrate/reference/bot-accounts/',
    fields: [
      {
        key: 'server_url',
        labelKey: 'channels.serverUrl',
        type: 'url',
        placeholderKey: 'channels.placeholderServerUrl',
        value: '',
        required: true,
      },
      {
        key: 'bot_token',
        labelKey: 'channels.botToken',
        type: 'password',
        placeholderKey: 'channels.placeholderBotAccessToken',
        value: '',
        required: true,
      },
    ],
  },
  {
    id: 'nextcloudtalk',
    nameKey: 'channels.nextcloudTalk',
    icon: getChannelIconOrDefault('nextcloudtalk'),
    enabled: false,
    status: 'disconnected',
    descriptionKey: 'channels.nextcloudTalkDesc',
    hintKey: 'channels.nextcloudTalkHint',
    docUrl: 'https://nextcloud.com/talk/',
    fields: [
      {
        key: 'server_url',
        labelKey: 'channels.serverUrl',
        type: 'url',
        placeholderKey: 'channels.placeholderNextcloudServerUrl',
        value: '',
        required: true,
      },
      {
        key: 'username',
        labelKey: 'channels.username',
        type: 'text',
        placeholderKey: 'channels.placeholderUsername',
        value: '',
        required: true,
      },
      {
        key: 'password',
        labelKey: 'channels.password',
        type: 'password',
        placeholderKey: 'channels.placeholderPassword',
        value: '',
        required: true,
      },
      {
        key: 'room_token',
        labelKey: 'channels.roomToken',
        type: 'text',
        placeholderKey: 'channels.placeholderNextcloudRoomToken',
        value: '',
        required: true,
      },
    ],
  },
  {
    id: 'googlechat',
    nameKey: 'channels.googleChat',
    icon: getChannelIconOrDefault('googlechat'),
    enabled: false,
    status: 'disconnected',
    descriptionKey: 'channels.googleChatDesc',
    hintKey: 'channels.googleChatHint',
    docUrl: 'https://developers.google.com/workspace/chat/quickstart/gcf-app',
    fields: [
      {
        key: 'credentials_json',
        labelKey: 'channels.serviceAccountJson',
        type: 'textarea',
        placeholderKey: 'channels.placeholderServiceAccountJson',
        value: '',
        required: true,
      },
    ],
  },
  {
    id: 'feishu',
    nameKey: 'channels.feishuBot',
    icon: getChannelIconOrDefault('feishu'),
    enabled: false,
    status: 'disconnected',
    descriptionKey: 'channels.feishuDesc',
    hintKey: 'channels.feishuHint',
    docUrl: 'https://open.feishu.cn/document/develop-an-echo-bot/introduction',
    fields: [
      {
        key: 'app_id',
        labelKey: 'channels.appId',
        type: 'text',
        placeholderKey: 'channels.placeholderFeishuAppId',
        value: '',
        required: true,
      },
      {
        key: 'app_secret',
        labelKey: 'channels.appSecret',
        type: 'password',
        placeholderKey: 'channels.placeholderAppSecret',
        value: '',
        required: true,
      },
      {
        key: 'verification_token',
        labelKey: 'channels.verificationToken',
        type: 'password',
        placeholderKey: 'channels.placeholderVerificationToken',
        value: '',
      },
      {
        key: 'encrypt_key',
        labelKey: 'channels.encryptKey',
        type: 'password',
        placeholder: '',
        value: '',
      },
      {
        key: 'session_mode',
        labelKey: 'channels.feishuSessionMode',
        type: 'toggle',
        value: 'false',
      },
    ],
  },
  {
    id: 'dingtalk',
    nameKey: 'channels.dingtalkBot',
    icon: getChannelIconOrDefault('dingtalk'),
    enabled: false,
    status: 'disconnected',
    descriptionKey: 'channels.dingtalkDesc',
    hintKey: 'channels.dingtalkHint',
    docUrl: 'https://open.dingtalk.com/document/orgapp/create-an-enterprise-chatbot',
    fields: [
      {
        key: 'app_key',
        labelKey: 'channels.appKey',
        type: 'text',
        placeholderKey: 'channels.placeholderDingtalkAppKey',
        value: '',
        required: true,
      },
      {
        key: 'app_secret',
        labelKey: 'channels.appSecret',
        type: 'password',
        placeholderKey: 'channels.placeholderAppSecret',
        value: '',
        required: true,
      },
      {
        key: 'robot_code',
        labelKey: 'channels.robotCode',
        type: 'text',
        placeholderKey: 'channels.placeholderRobotCode',
        value: '',
        required: true,
      },
    ],
  },
  {
    id: 'qq',
    nameKey: 'channels.qqBot',
    icon: getChannelIconOrDefault('qq'),
    enabled: false,
    status: 'disconnected',
    descriptionKey: 'channels.qqDesc',
    hintKey: 'channels.qqHint',
    docUrl: 'https://bot.q.qq.com/wiki',
    fields: [
      {
        key: 'app_id',
        labelKey: 'channels.appId',
        type: 'text',
        placeholderKey: 'channels.placeholderQQAppId',
        value: '',
        required: true,
      },
      {
        key: 'app_secret',
        labelKey: 'channels.appSecret',
        type: 'password',
        placeholderKey: 'channels.placeholderAppSecret',
        value: '',
        required: true,
      },
      {
        key: 'token',
        labelKey: 'channels.botToken',
        type: 'password',
        placeholderKey: 'channels.placeholderBotTokenGeneric',
        value: '',
        required: true,
      },
    ],
  },
  {
    id: 'wechat',
    nameKey: 'channels.wechatWorkBot',
    icon: getChannelIconOrDefault('wechat'),
    enabled: false,
    status: 'disconnected',
    descriptionKey: 'channels.wechatDesc',
    hintKey: 'channels.wechatHint',
    docUrl: 'https://developer.work.weixin.qq.com/document/path/90664',
    fields: [
      {
        key: 'corp_id',
        labelKey: 'channels.corpId',
        type: 'text',
        placeholderKey: 'channels.placeholderWechatCorpId',
        value: '',
        required: true,
      },
      {
        key: 'agent_id',
        labelKey: 'channels.agentId',
        type: 'text',
        placeholderKey: 'channels.placeholderAgentId',
        value: '',
        required: true,
      },
      {
        key: 'secret',
        labelKey: 'channels.secret',
        type: 'password',
        placeholderKey: 'channels.placeholderSecret',
        value: '',
        required: true,
      },
    ],
  },
  {
    id: 'wechat_ilink',
    nameKey: 'channels.wechatILink',
    icon: getChannelIconOrDefault('wechat'),
    enabled: false,
    status: 'disconnected',
    descriptionKey: 'channels.wechatILinkDesc',
    hintKey: 'channels.wechatILinkHint',
    fields: [
      {
        key: 'api_base_url',
        labelKey: 'channels.apiBaseURL',
        type: 'url',
        placeholderKey: 'channels.placeholderILinkAPIBaseURL',
        value: '',
        required: true,
      },
      {
        key: 'bot_token',
        labelKey: 'channels.botToken',
        type: 'password',
        placeholderKey: 'channels.placeholderBotTokenGeneric',
        value: '',
        required: true,
      },
    ],
  },
  {
    id: 'matrix',
    nameKey: 'channels.matrix',
    icon: getChannelIconOrDefault('matrix'),
    enabled: false,
    status: 'disconnected',
    descriptionKey: 'channels.matrixDesc',
    hintKey: 'channels.matrixHint',
    docUrl: 'https://spec.matrix.org/latest/client-server-api/',
    fields: [
      {
        key: 'homeserver',
        labelKey: 'channels.matrixHomeserver',
        type: 'url',
        placeholderKey: 'channels.placeholderMatrixHomeserver',
        value: '',
        required: true,
      },
      {
        key: 'user_id',
        labelKey: 'channels.matrixUserId',
        type: 'text',
        placeholderKey: 'channels.placeholderMatrixUserId',
        value: '',
        required: true,
      },
      {
        key: 'access_token',
        labelKey: 'channels.accessToken',
        type: 'password',
        placeholderKey: 'channels.placeholderAccessToken',
        value: '',
        required: true,
      },
    ],
  },
  {
    id: 'imessage',
    nameKey: 'channels.imessage',
    icon: getChannelIconOrDefault('imessage'),
    enabled: false,
    status: 'disconnected',
    descriptionKey: 'channels.imessageDesc',
    hintKey: 'channels.imessageHint',
    docUrl: 'https://github.com/mautrix/imessage',
    fields: [],
  },
  {
    id: 'bluebubbles',
    nameKey: 'channels.blueBubbles',
    icon: getChannelIconOrDefault('bluebubbles'),
    enabled: false,
    status: 'disconnected',
    descriptionKey: 'channels.blueBubblesDesc',
    hintKey: 'channels.blueBubblesHint',
    docUrl: 'https://bluebubbles.app/docs/',
    fields: [
      {
        key: 'server_url',
        labelKey: 'channels.serverUrl',
        type: 'url',
        placeholderKey: 'channels.placeholderBlueBubblesServerUrl',
        value: '',
        required: true,
      },
      {
        key: 'password',
        labelKey: 'channels.password',
        type: 'password',
        placeholderKey: 'channels.placeholderServerPassword',
        value: '',
        required: true,
      },
    ],
  },
  {
    id: 'zalo',
    nameKey: 'channels.zaloOA',
    icon: getChannelIconOrDefault('zalo'),
    enabled: false,
    status: 'disconnected',
    descriptionKey: 'channels.zaloDesc',
    hintKey: 'channels.zaloHint',
    docUrl: 'https://developers.zalo.me/docs/api/official-account-api-147',
    fields: [
      {
        key: 'oa_id',
        labelKey: 'channels.oaId',
        type: 'text',
        placeholderKey: 'channels.placeholderOaId',
        value: '',
        required: true,
      },
      {
        key: 'access_token',
        labelKey: 'channels.accessToken',
        type: 'password',
        placeholderKey: 'channels.placeholderOaAccessToken',
        value: '',
        required: true,
      },
      {
        key: 'refresh_token',
        labelKey: 'channels.refreshToken',
        type: 'password',
        placeholderKey: 'channels.placeholderOaRefreshToken',
        value: '',
      },
      {
        key: 'app_id',
        labelKey: 'channels.appId',
        type: 'text',
        placeholderKey: 'channels.placeholderZaloAppId',
        value: '',
      },
      {
        key: 'secret_key',
        labelKey: 'channels.secretKey',
        type: 'password',
        placeholderKey: 'channels.placeholderZaloSecretKey',
        value: '',
      },
    ],
  },
  {
    id: 'line',
    nameKey: 'channels.line',
    icon: getChannelIconOrDefault('line'),
    enabled: false,
    status: 'disconnected',
    descriptionKey: 'channels.lineDesc',
    hintKey: 'channels.lineHint',
    docUrl: 'https://developers.line.biz/en/docs/messaging-api/',
    fields: [
      {
        key: 'channel_access_token',
        labelKey: 'channels.channelAccessToken',
        type: 'password',
        placeholderKey: 'channels.placeholderLineChannelToken',
        value: '',
        required: true,
      },
      {
        key: 'channel_secret',
        labelKey: 'channels.channelSecret',
        type: 'password',
        placeholderKey: 'channels.placeholderLineChannelSecret',
        value: '',
        required: true,
      },
    ],
  },
  {
    id: 'messenger',
    nameKey: 'channels.messenger',
    icon: getChannelIconOrDefault('messenger'),
    enabled: false,
    status: 'disconnected',
    descriptionKey: 'channels.messengerDesc',
    hintKey: 'channels.messengerHint',
    docUrl: 'https://developers.facebook.com/docs/messenger-platform/',
    fields: [
      {
        key: 'page_access_token',
        labelKey: 'channels.pageAccessToken',
        type: 'password',
        placeholderKey: 'channels.placeholderMessengerPageToken',
        value: '',
        required: true,
      },
      {
        key: 'verify_token',
        labelKey: 'channels.verifyToken',
        type: 'password',
        placeholderKey: 'channels.placeholderMessengerVerifyToken',
        value: '',
        required: true,
      },
      {
        key: 'app_secret',
        labelKey: 'channels.appSecret',
        type: 'password',
        placeholderKey: 'channels.placeholderAppSecret',
        value: '',
      },
    ],
  },
  {
    id: 'viber',
    nameKey: 'channels.viber',
    icon: getChannelIconOrDefault('viber'),
    enabled: false,
    status: 'disconnected',
    descriptionKey: 'channels.viberDesc',
    hintKey: 'channels.viberHint',
    docUrl: 'https://developers.viber.com/docs/api/rest-bot-api/',
    fields: [
      {
        key: 'auth_token',
        labelKey: 'channels.authToken',
        type: 'password',
        placeholderKey: 'channels.placeholderViberAuthToken',
        value: '',
        required: true,
      },
      {
        key: 'bot_name',
        labelKey: 'channels.botName',
        type: 'text',
        placeholderKey: 'channels.placeholderViberBotName',
        value: '',
      },
    ],
  },
  {
    id: 'twitter',
    nameKey: 'channels.twitterDM',
    icon: getChannelIconOrDefault('twitter'),
    enabled: false,
    status: 'disconnected',
    descriptionKey: 'channels.twitterDMDesc',
    hintKey: 'channels.twitterDMHint',
    docUrl: 'https://developer.twitter.com/en/docs/twitter-api/direct-messages',
    fields: [
      {
        key: 'api_key',
        labelKey: 'channels.apiKey',
        type: 'password',
        placeholderKey: 'channels.placeholderTwitterApiKey',
        value: '',
        required: true,
      },
      {
        key: 'api_secret',
        labelKey: 'channels.apiSecret',
        type: 'password',
        placeholderKey: 'channels.placeholderTwitterApiSecret',
        value: '',
        required: true,
      },
      {
        key: 'access_token',
        labelKey: 'channels.accessToken',
        type: 'password',
        placeholderKey: 'channels.placeholderTwitterAccessToken',
        value: '',
        required: true,
      },
      {
        key: 'access_token_secret',
        labelKey: 'channels.accessTokenSecret',
        type: 'password',
        placeholderKey: 'channels.placeholderTwitterAccessTokenSecret',
        value: '',
        required: true,
      },
    ],
  },
  {
    id: 'instagram',
    nameKey: 'channels.instagramDM',
    icon: getChannelIconOrDefault('instagram'),
    enabled: false,
    status: 'disconnected',
    descriptionKey: 'channels.instagramDMDesc',
    hintKey: 'channels.instagramDMHint',
    docUrl: 'https://developers.facebook.com/docs/messenger-platform/instagram/',
    fields: [
      {
        key: 'page_access_token',
        labelKey: 'channels.pageAccessToken',
        type: 'password',
        placeholderKey: 'channels.placeholderInstagramPageToken',
        value: '',
        required: true,
      },
      {
        key: 'instagram_account_id',
        labelKey: 'channels.instagramAccountId',
        type: 'text',
        placeholderKey: 'channels.placeholderInstagramAccountId',
        value: '',
        required: true,
      },
    ],
  },
  {
    id: 'twitch',
    nameKey: 'channels.twitchBot',
    icon: getChannelIconOrDefault('twitch'),
    enabled: false,
    status: 'disconnected',
    descriptionKey: 'channels.twitchDesc',
    hintKey: 'channels.twitchHint',
    docUrl: 'https://dev.twitch.tv/docs/irc/',
    fields: [
      {
        key: 'client_id',
        labelKey: 'channels.clientId',
        type: 'text',
        placeholderKey: 'channels.placeholderTwitchClientId',
        value: '',
        required: true,
      },
      {
        key: 'client_secret',
        labelKey: 'channels.clientSecret',
        type: 'password',
        placeholderKey: 'channels.placeholderTwitchClientSecret',
        value: '',
        required: true,
      },
      {
        key: 'bot_username',
        labelKey: 'channels.botUsername',
        type: 'text',
        placeholderKey: 'channels.placeholderTwitchBotUsername',
        value: '',
        required: true,
      },
      {
        key: 'channel_name',
        labelKey: 'channels.channelName',
        type: 'text',
        placeholderKey: 'channels.placeholderTwitchChannelName',
        value: '',
      },
    ],
  },
])

// Sorted channels - enabled channels first
const sortedChannels = computed(() => {
  return [...channelDefs.value].sort((a, b) => {
    // Enabled channels first
    if (a.enabled && !b.enabled) return -1
    if (!a.enabled && b.enabled) return 1
    // Then by connection status (connected > connecting > error > disconnected)
    const statusOrder = { connected: 0, connecting: 1, error: 2, disconnected: 3 }
    return (statusOrder[a.status] ?? 3) - (statusOrder[b.status] ?? 3)
  })
})

const orderedChannels = computed(() => {
  const ids = primaryChannelIds.value
  const prioritized = sortedChannels.value.filter((c) => ids.includes(c.id))

  // Sort by the order defined in primaryChannelIds
  prioritized.sort((a, b) => {
    const indexA = ids.indexOf(a.id)
    const indexB = ids.indexOf(b.id)
    return indexA - indexB
  })

  const prioritizedIds = new Set(prioritized.map((channel) => channel.id))
  const remaining = sortedChannels.value.filter((channel) => !prioritizedIds.has(channel.id))

  return [...prioritized, ...remaining]
})

// Primary channels (top channels based on locale)
const primaryChannels = computed(() => {
  return orderedChannels.value.slice(0, defaultVisibleChannelCount)
})

// Secondary channels (shown after clicking "Load More")
const secondaryChannels = computed(() => {
  return orderedChannels.value.slice(defaultVisibleChannelCount)
})

// Channel map for O(1) lookup
const channelMap = computed(() => {
  const map = new Map<string, ChannelDef>()
  for (const ch of channelDefs.value) {
    map.set(ch.id, ch)
  }
  return map
})

const expandedChannel = ref<string | null>(null)
const isRemoteAccessSelected = computed(() => expandedChannel.value === REMOTE_ACCESS_PANEL_ID)
const selectedChannel = computed(() => {
  return expandedChannel.value ? channelMap.value.get(expandedChannel.value) || null : null
})
const selectedChannelId = computed(() => selectedChannel.value?.id || '')
const isWeChatILinkSelected = computed(() => selectedChannelId.value === 'wechat_ilink')
const selectedChannelRenderKey = computed(() => {
  const channel = selectedChannel.value
  if (!channel) return 'no-channel'

  const testState =
    testResult.value?.channelId === channel.id
      ? `${testResult.value.success}:${testResult.value.message}`
      : 'no-test-result'

  return [
    channel.id,
    channel.enabled ? 'enabled' : 'disabled',
    channel.status,
    channel.lastError || '',
    toggling.value === channel.id ? 'toggling' : 'idle',
    saving.value === channel.id ? 'saving' : 'idle',
    testingConnection.value === channel.id ? 'testing' : 'idle',
    testState,
  ].join('|')
})
const remoteAccessRenderKey = computed(() => {
  return [
    remoteAccessState.value,
    remoteAccessError.value || '',
    selectedProvider.value,
    currentProviderToken.value,
    ngrokDomain.value,
    tunnelProviders.value.map((provider) => provider.id).join(','),
    tunnelStatus.value?.active ? 'active' : 'inactive',
    tunnelStatus.value?.connecting ? 'connecting' : 'idle',
    tunnelStatus.value?.provider || '',
    tunnelStatus.value?.url || '',
  ].join('|')
})
const remoteAccessStatusTone = computed(() => {
  switch (remoteAccessState.value) {
    case 'connected':
      return 'connected'
    case 'connecting':
      return 'connecting'
    case 'error':
      return 'error'
    default:
      return 'disconnected'
  }
})

const remoteAccessStatusText = computed(() => {
  switch (remoteAccessState.value) {
    case 'connected':
      return t('channels.statusConnected')
    case 'connecting':
      return t('channels.statusConnecting')
    case 'error':
      return t('channels.statusError')
    case 'loading':
      return t('common.loading')
    default:
      return t('channels.statusDisconnected')
  }
})

const remoteAccessStatusTitle = computed(() => {
  return remoteAccessStatusTone.value === 'error' && remoteAccessError.value
    ? `${remoteAccessStatusText.value}: ${remoteAccessError.value}`
    : remoteAccessStatusText.value
})

const remoteAccessDescriptionError = computed(
  () => remoteAccessStatusTone.value === 'error' && !!remoteAccessError.value
)

const remoteAccessDescription = computed(() => {
  return remoteAccessDescriptionError.value
    ? remoteAccessError.value || ''
    : t('remoteAccess.channelDescription')
})

const remoteAccessIconAlt = computed(() => t('remoteAccess.title'))

const remoteAccessRecommendedLabel = computed(() => t('remoteAccess.recommended'))

const remoteAccessTitle = computed(() => t('remoteAccess.title'))

const remoteAccessShouldShowToggle = computed(
  () => remoteAccessState.value === 'connected' || remoteAccessState.value === 'ready'
)

watch(
  orderedChannels,
  (channels) => {
    if (channels.length === 0) {
      expandedChannel.value = null
      return
    }

    if (
      expandedChannel.value &&
      expandedChannel.value !== REMOTE_ACCESS_PANEL_ID &&
      !channels.some((channel) => channel.id === expandedChannel.value)
    ) {
      expandedChannel.value = null
    }
  },
  { immediate: true }
)

const enabledCount = computed(() => {
  const channelCount = channelDefs.value.filter((c) => c.enabled).length
  const remoteCount = remoteAccessState.value === 'connected' ? 1 : 0
  return channelCount + remoteCount
})

const connectedCount = computed(() => {
  const channelCount = channelDefs.value.filter((c) => c.status === 'connected').length
  const remoteCount = remoteAccessState.value === 'connected' ? 1 : 0
  return channelCount + remoteCount
})

const heroPreviewChannels = computed(() =>
  primaryChannels.value.slice(0, defaultVisibleChannelCount)
)

const groupAccessPolicyLabel = computed(() => {
  switch (groupAccessPolicy.value) {
    case 'allowlist':
      return t('channels.groupAccessPolicyAllowlist')
    case 'disabled':
      return t('channels.groupAccessPolicyDisabled')
    default:
      return t('channels.groupAccessPolicyOpen')
  }
})

const groupAccessMentionPolicyLabel = computed(() => {
  switch (groupAccessMentionPolicy.value) {
    case 'always':
      return t('channels.groupAccessMentionPolicyAlways')
    default:
      return t('channels.groupAccessMentionPolicyMentioned')
  }
})

const groupAccessAllowedChatCount = computed(() =>
  countAllowedChatEntries(groupAccessAllowedChatIDsText.value)
)

const groupAccessSummaryDetail = computed(() => {
  if (groupAccessPolicy.value === 'allowlist') {
    return `${t('channels.groupAccessAllowedChats')}: ${groupAccessAllowedChatCount.value}`
  }

  if (groupAccessPolicy.value === 'disabled') {
    return t('channels.groupAccessDesc')
  }

  return groupAccessMentionPolicyLabel.value
})

const selectedProviderInfo = computed(() => {
  return tunnelProviders.value.find((p) => p.id === selectedProvider.value)
})

const currentProviderToken = computed({
  get: () => {
    if (selectedProvider.value === 'ngrok') return ngrokAuthtoken.value
    if (selectedProvider.value === 'cloudflare') return cloudflareToken.value
    return ''
  },
  set: (value: string) => {
    if (selectedProvider.value === 'ngrok') ngrokAuthtoken.value = value
    else if (selectedProvider.value === 'cloudflare') cloudflareToken.value = value
  },
})

function toggleChannel(channelId: string) {
  expandedChannel.value = expandedChannel.value === channelId ? null : channelId
}

async function toggleChannelEnabled(channelId: string, enabled: boolean) {
  toggling.value = channelId
  const channelDef = channelMap.value.get(channelId)
  if (!channelDef) return

  // Check if required fields are filled when enabling
  if (enabled) {
    const missingFields = getVisibleChannelFields(channelDef).filter((f) => f.required && !f.value)
    if (missingFields.length > 0) {
      // Expand the channel to show config
      expandedChannel.value = channelId
      testResult.value = { channelId, success: false, message: t('channels.fillRequiredFields') }
      toggling.value = null
      return
    }
  }

  // Optimistically update the UI
  channelDef.enabled = enabled
  channelDef.lastError = undefined
  // Set status to connecting when enabling, disconnected when disabling
  if (enabled) {
    channelDef.status = 'connecting'
  } else {
    channelDef.status = 'disconnected'
  }
  triggerRef(channelDefs)

  try {
    const response = await channelsApi.toggleChannel(channelId, { enabled })
    const data = response.data
    if (isSuccessfulStatus(response.status)) {
      // Update status from server response
      channelDef.status = data.status || (enabled ? 'connected' : 'disconnected')
      channelDef.lastError = data.channel?.last_error
      channelDef.lastErrorKey = data.channel?.last_error_key
      triggerRef(channelDefs)

      // Start polling for status if enabling and still connecting
      if (enabled && channelDef.status === 'connecting') {
        pollChannelStatus(channelId)
      }

      // Show error message if status is error
      if (channelDef.status === 'error' && channelDef.lastError) {
        testResult.value = { channelId, success: false, message: resolveChannelError(channelDef) }
      }
    } else {
      // Revert on error
      channelDef.enabled = !enabled
      channelDef.status = 'error'
      channelDef.lastError = data.message
      triggerRef(channelDefs)
      testResult.value = {
        channelId,
        success: false,
        message: data.message || t('channels.toggleFailed'),
      }
    }
  } catch {
    // Revert on error
    channelDef.enabled = !enabled
    channelDef.status = 'error'
    triggerRef(channelDefs)
    testResult.value = { channelId, success: false, message: t('channels.toggleFailed') }
  } finally {
    toggling.value = null
  }
}

async function loadChannelConfigs() {
  loading.value = true
  channelLoadError.value = null
  try {
    const response = await channelsApi.list()
    if (!isSuccessfulStatus(response.status)) {
      const data = response.data || {}
      channelLoadError.value = formatFetchFailureMessage(
        response,
        'Failed to load channels.',
        data.message
      )
      return
    }

    const data = response.data
    // Merge server data with local channel definitions
    for (const serverChannel of data.channels || []) {
      const localChannel = channelMap.value.get(serverChannel.id)
      if (localChannel) {
        localChannel.enabled = serverChannel.enabled
        localChannel.status = serverChannel.status
        localChannel.lastError = serverChannel.last_error
        localChannel.lastErrorKey = serverChannel.last_error_key
        // Update message statistics
        localChannel.messagesReceived = serverChannel.messages_received
        localChannel.messagesSent = serverChannel.messages_sent
        localChannel.lastMessageAt = serverChannel.last_message_at
        localChannel.lastReplyAt = serverChannel.last_reply_at
        // Update field values
        for (const field of localChannel.fields) {
          if (
            serverChannel.config &&
            Object.prototype.hasOwnProperty.call(serverChannel.config, field.key)
          ) {
            const fieldValue = serverChannel.config[field.key]
            if (typeof fieldValue === 'string') {
              field.value = fieldValue
            }
          }
        }
      }
    }
    triggerRef(channelDefs)
  } catch (err) {
    console.error('Failed to load channel configs:', err)
    channelLoadError.value =
      err instanceof Error && err.message ? err.message : 'Failed to load channels.'
  } finally {
    loading.value = false
  }
}

async function loadChannelSettings() {
  try {
    const response = await channelsApi.getSettings()
    if (!isSuccessfulStatus(response.status)) return

    const data = response.data as ChannelSettingsResponse
    const groupAccess = data.group_access || {}
    groupAccessPolicy.value = normalizeGroupAccessPolicy(groupAccess.policy)
    groupAccessMentionPolicy.value = normalizeGroupMentionPolicy(groupAccess.mention_policy)
    groupAccessAllowedChatIDsText.value = formatAllowedChatIDs(groupAccess.allowed_chat_ids)
    if (!showGroupAccessModal.value) {
      syncGroupAccessDraftFromCurrent()
    }
  } catch (err) {
    console.error('Failed to load channel settings:', err)
  }
}

function openGroupAccessModal() {
  groupAccessResult.value = null
  syncGroupAccessDraftFromCurrent()
  showGroupAccessModal.value = true
}

function closeGroupAccessModal() {
  if (savingGroupAccess.value) return
  showGroupAccessModal.value = false
  syncGroupAccessDraftFromCurrent()
}

function stopWeChatILinkSetupPolling() {
  if (wechatILinkSetupPoller) {
    clearInterval(wechatILinkSetupPoller)
    wechatILinkSetupPoller = null
  }
}

function closeWeChatILinkSetupModal() {
  stopWeChatILinkSetupPolling()
  wechatILinkSetupOpen.value = false
  wechatILinkSetupError.value = ''
  wechatILinkSetupSession.value = null
}

async function refreshWeChatILinkSetupSession(sessionId: string) {
  try {
    const response = await getWeChatILinkSetupSession(sessionId)
    if (!isSuccessfulStatus(response.status)) {
      wechatILinkSetupError.value =
        response.data?.error || t('channels.wechatILinkSetupLoadFailed')
      stopWeChatILinkSetupPolling()
      return
    }

    const previousSession = wechatILinkSetupSession.value
    wechatILinkSetupSession.value = {
      ...previousSession,
      ...response.data,
      qrcode: response.data.qrcode || previousSession?.qrcode,
      mobile_url: response.data.mobile_url || previousSession?.mobile_url,
    }
    wechatILinkSetupError.value = response.data.error || ''

    if (
      response.data.status === 'connected' ||
      response.data.status === 'error' ||
      response.data.status === 'expired'
    ) {
      stopWeChatILinkSetupPolling()
      if (response.data.status === 'connected') {
        await loadChannelConfigs()
      }
    }
  } catch (error) {
    wechatILinkSetupError.value = getErrorMessage(error) || t('channels.wechatILinkSetupLoadFailed')
    stopWeChatILinkSetupPolling()
  }
}

async function startWeChatILinkSetup() {
  wechatILinkSetupPending.value = true
  wechatILinkSetupError.value = ''
  wechatILinkSetupSession.value = null
  wechatILinkSetupOpen.value = true

  try {
    const response = await createWeChatILinkSetupSession()
    if (!isSuccessfulStatus(response.status)) {
      wechatILinkSetupError.value =
        response.data?.error || t('channels.wechatILinkSetupCreateFailed')
      return
    }

    wechatILinkSetupSession.value = response.data
    stopWeChatILinkSetupPolling()
    wechatILinkSetupPoller = setInterval(() => {
      if (!wechatILinkSetupSession.value?.session_id) return
      void refreshWeChatILinkSetupSession(wechatILinkSetupSession.value.session_id)
    }, 1500)
  } catch (error) {
    wechatILinkSetupError.value =
      getErrorMessage(error) || t('channels.wechatILinkSetupCreateFailed')
  } finally {
    wechatILinkSetupPending.value = false
  }
}

async function saveGroupAccessSettings() {
  savingGroupAccess.value = true

  try {
    const parsed =
      groupAccessDraftPolicy.value === 'allowlist'
        ? parseAllowedChatIDs(groupAccessDraftAllowedChatIDsText.value)
        : { allowed_chat_ids: {} }
    if (parsed.error) {
      showGroupAccessResult(false, parsed.error)
      return
    }

    const response = await channelsApi.updateSettings({
      group_access: {
        policy: groupAccessDraftPolicy.value,
        mention_policy: groupAccessDraftMentionPolicy.value,
        allowed_chat_ids: parsed.allowed_chat_ids,
      },
    })

    const data = response.data || {}
    if (!isSuccessfulStatus(response.status)) {
      showGroupAccessResult(false, data.message || t('channels.groupAccessSaveFailed'))
      return
    }

    const settings = (data.settings || data) as ChannelSettingsResponse
    const groupAccess = settings.group_access || {
      policy: groupAccessDraftPolicy.value,
      mention_policy: groupAccessDraftMentionPolicy.value,
      allowed_chat_ids: parsed.allowed_chat_ids,
    }
    groupAccessPolicy.value = normalizeGroupAccessPolicy(groupAccess.policy)
    groupAccessMentionPolicy.value = normalizeGroupMentionPolicy(groupAccess.mention_policy)
    groupAccessAllowedChatIDsText.value = formatAllowedChatIDs(groupAccess.allowed_chat_ids)
    syncGroupAccessDraftFromCurrent()
    showGroupAccessModal.value = false
    showGroupAccessResult(true, t('channels.savedSuccessfully'))
  } catch (err) {
    console.error('Failed to save channel settings:', err)
    showGroupAccessResult(false, t('channels.groupAccessSaveFailed'))
  } finally {
    savingGroupAccess.value = false
  }
}

async function saveChannel(channelId: string) {
  saving.value = channelId
  const channelDef = channelMap.value.get(channelId)
  if (!channelDef) return

  try {
    const config: Record<string, string> = {}
    for (const field of channelDef.fields) {
      config[field.key] = field.value
    }

    const response = await channelsApi.updateChannel(channelId, {
      enabled: channelDef.enabled,
      config,
    })

    const data = response.data
    if (isSuccessfulStatus(response.status)) {
      // Update status from server response
      if (data.channel) {
        channelDef.status = data.channel.status || channelDef.status
        channelDef.lastError = data.channel.last_error
        triggerRef(channelDefs)
      }
      if (channelDef.status === 'error' && channelDef.lastError) {
        testResult.value = { channelId, success: false, message: channelDef.lastError }
      } else {
        testResult.value = { channelId, success: true, message: t('channels.savedSuccessfully') }
      }

      // Start polling for status if channel is enabled and still connecting
      if (channelDef.enabled && channelDef.status === 'connecting') {
        pollChannelStatus(channelId)
      }
    } else {
      testResult.value = {
        channelId,
        success: false,
        message: data.message || t('channels.saveFailed'),
      }
    }
  } catch {
    testResult.value = { channelId, success: false, message: t('channels.saveFailed') }
  } finally {
    saving.value = null
    setTimeout(() => {
      if (testResult.value?.channelId === channelId) {
        testResult.value = null
      }
    }, 3000)
  }
}

// Poll channel status until connected or error (for async channel startup)
const channelPollIntervals: Map<string, ReturnType<typeof setInterval>> = new Map()

async function pollChannelStatus(channelId: string) {
  // Clear any existing polling for this channel
  const existingInterval = channelPollIntervals.get(channelId)
  if (existingInterval) {
    clearInterval(existingInterval)
  }

  const maxAttempts = 30 // 30 seconds max (30 * 1s)
  let attempts = 0

  const interval = setInterval(async () => {
    attempts++

    try {
      const response = await channelsApi.getChannelStatus(channelId)
      if (!isSuccessfulStatus(response.status)) {
        // API error, stop polling
        clearInterval(interval)
        channelPollIntervals.delete(channelId)
        return
      }

      const data = response.data
      const channelDef = channelMap.value.get(channelId)
      if (!channelDef) {
        clearInterval(interval)
        channelPollIntervals.delete(channelId)
        return
      }

      // Update status
      channelDef.status = data.status
      if (data.last_error) {
        channelDef.lastError = data.last_error
      }
      triggerRef(channelDefs)

      // Stop polling if connected, error, or max attempts reached
      if (data.status === 'connected' || data.status === 'error' || attempts >= maxAttempts) {
        clearInterval(interval)
        channelPollIntervals.delete(channelId)
      }
    } catch (e) {
      console.error('Polling error:', e)
      if (attempts >= maxAttempts) {
        clearInterval(interval)
        channelPollIntervals.delete(channelId)
      }
    }
  }, 1000) // Poll every 1 second

  channelPollIntervals.set(channelId, interval)
}

async function testConnection(channelId: string) {
  testingConnection.value = channelId
  testResult.value = null
  const channelDef = channelMap.value.get(channelId)
  if (!channelDef) return

  try {
    const config: Record<string, string> = {}
    for (const field of channelDef.fields) {
      config[field.key] = field.value
    }

    const response = await channelsApi.testConnection({ type: channelId, config })
    const data = response.data
    const message = data.message
    testResult.value = { channelId, success: data.success, message }

    // Update status based on test result
    if (data.success) {
      // If enabled, set to connected; otherwise keep disconnected but mark as valid config
      if (channelDef.enabled) {
        channelDef.status = 'connected'
        triggerRef(channelDefs)
      }
    } else {
      // Test failed - if enabled, show error
      if (channelDef.enabled) {
        channelDef.status = 'error'
        triggerRef(channelDefs)
      }
    }
  } catch {
    testResult.value = { channelId, success: false, message: t('channels.testFailed') }
    if (channelDef.enabled) {
      channelDef.status = 'error'
      triggerRef(channelDefs)
    }
  } finally {
    testingConnection.value = null
  }
}

// Remote Access functions
async function loadRemoteAccessStatus() {
  try {
    const providersRes = await getTunnelProviders()
    if (providersRes.data.providers) {
      tunnelProviders.value = providersRes.data.providers
    }

    try {
      const configRes = await getRemoteAccessConfig()
      if (configRes.data.config) {
        if (configRes.data.config.ngrok_authtoken) {
          ngrokAuthtoken.value = configRes.data.config.ngrok_authtoken
        }
        if (configRes.data.config.ngrok_domain) {
          ngrokDomain.value = configRes.data.config.ngrok_domain
        }
        if (configRes.data.config.cloudflare_token) {
          cloudflareToken.value = configRes.data.config.cloudflare_token
        }
        if (configRes.data.config.default_provider) {
          selectedProvider.value = configRes.data.config.default_provider
        }
      }
    } catch (e) {
      console.warn('Failed to load saved config:', e)
    }

    const statusRes = await getRemoteAccessStatus()
    tunnelStatus.value = statusRes.data.tunnel

    if (statusRes.data.tunnel.provider) {
      selectedProvider.value = statusRes.data.tunnel.provider
    }

    if (statusRes.data.tunnel.active || statusRes.data.tunnel.url) {
      remoteAccessState.value = 'connected'
      startRemoteAccessPolling()
    } else if (statusRes.data.tunnel.connecting) {
      remoteAccessState.value = 'connecting'
      startRemoteAccessPolling()
    } else {
      remoteAccessState.value = 'ready'
    }
  } catch (e) {
    console.error('Failed to load remote access status:', e)
    remoteAccessError.value = t('remoteAccess.loadError')
    remoteAccessState.value = 'error'
  }
}

async function handleRemoteAccessStart() {
  remoteAccessState.value = 'connecting'
  remoteAccessError.value = null

  try {
    if (currentProviderToken.value || (selectedProvider.value === 'ngrok' && ngrokDomain.value)) {
      const configUpdate: Record<string, string> = { default_provider: selectedProvider.value }
      if (selectedProvider.value === 'ngrok') {
        configUpdate.ngrok_authtoken = ngrokAuthtoken.value
        if (ngrokDomain.value) {
          configUpdate.ngrok_domain = ngrokDomain.value
        }
      } else if (selectedProvider.value === 'cloudflare') {
        configUpdate.cloudflare_token = cloudflareToken.value
      }
      await updateRemoteAccessConfig(configUpdate)
    }

    const response = await startRemoteAccess(
      selectedProvider.value,
      undefined,
      selectedProvider.value === 'ngrok' ? ngrokAuthtoken.value : undefined,
      selectedProvider.value === 'cloudflare' ? cloudflareToken.value : undefined,
      selectedProvider.value === 'ngrok' ? ngrokDomain.value : undefined
    )
    if (response.data.success) {
      if (response.data.tunnel) {
        tunnelStatus.value = response.data.tunnel
        if (response.data.tunnel.active || response.data.tunnel.url) {
          remoteAccessState.value = 'connected'
        }
      }
      try {
        const statusRes = await getRemoteAccessStatus()
        tunnelStatus.value = statusRes.data.tunnel
        if (statusRes.data.tunnel.active || statusRes.data.tunnel.url) {
          remoteAccessState.value = 'connected'
        }
      } catch {
        // Polling will retry.
      }
      startRemoteAccessPolling()
    } else {
      throw new Error(response.data.message || 'Failed to start tunnel')
    }
  } catch (e: unknown) {
    console.error('Failed to start tunnel:', e)
    const err = e as { response?: { data?: { error?: string } }; message?: string }
    remoteAccessState.value = 'error'
    remoteAccessError.value =
      err.response?.data?.error || err.message || t('remoteAccess.startError')
  }
}

async function handleRemoteAccessStop() {
  const isDesktop = typeof window !== 'undefined' && !!window.__BLUE_DESKTOP__

  if (!isDesktop) {
    const tunnelUrl = tunnelStatus.value?.url
    let isAccessingViaTunnel = false
    if (tunnelUrl) {
      try {
        isAccessingViaTunnel = window.location.host === new URL(tunnelUrl).host
      } catch {
        // Ignore invalid URL parsing here.
      }
    }
    const confirmMessage = isAccessingViaTunnel
      ? t('remoteAccess.disconnectConfirmMessageSameHost')
      : t('remoteAccess.disconnectConfirmMessage')
    if (!window.confirm(`${t('remoteAccess.disconnectConfirmTitle')}\n\n${confirmMessage}`)) {
      return
    }
  }

  try {
    await stopRemoteAccess()
    stopRemoteAccessPolling()
    tunnelStatus.value = { active: false }
    remoteAccessState.value = 'ready'
  } catch (e) {
    console.error('Failed to stop tunnel:', e)
    remoteAccessError.value = t('remoteAccess.stopError')
  }
}

function startRemoteAccessPolling() {
  statusInterval = setInterval(async () => {
    try {
      const response = await getRemoteAccessStatus()
      tunnelStatus.value = response.data.tunnel

      if (response.data.tunnel.active || response.data.tunnel.url) {
        remoteAccessState.value = 'connected'
      } else if (remoteAccessState.value === 'connecting') {
        // Still waiting for tunnel to start.
      } else if (!response.data.tunnel.active && !response.data.tunnel.url) {
        remoteAccessState.value = 'ready'
      }
    } catch (e) {
      console.error('Failed to poll status:', e)
    }
  }, 2000)
}

function stopRemoteAccessPolling() {
  if (statusInterval) {
    clearInterval(statusInterval)
    statusInterval = null
  }
}

function toggleRemoteAccessExpanded() {
  expandedChannel.value =
    expandedChannel.value === REMOTE_ACCESS_PANEL_ID ? null : REMOTE_ACCESS_PANEL_ID
}

function handleGroupAccessDialogKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape' && showGroupAccessModal.value) {
    closeGroupAccessModal()
  }
}

function updateChannelField(channelId: string, fieldIndex: number, value: string) {
  const channelIndex = channelDefs.value.findIndex((channel) => channel.id === channelId)
  if (channelIndex < 0) return

  const channel = channelDefs.value[channelIndex]
  if (!channel?.fields[fieldIndex]) return

  const nextFields = channel.fields.map((field, index) =>
    index === fieldIndex ? { ...field, value } : field
  )

  const nextChannels = [...channelDefs.value]
  nextChannels[channelIndex] = {
    ...channel,
    fields: nextFields,
  }

  channelDefs.value = nextChannels
}

onMounted(() => {
  // Load backend settings first to get locale
  settingsStore.fetchBackendSettings()
  loadChannelConfigs()
  loadChannelSettings()
  loadRemoteAccessStatus()
  window.addEventListener('keydown', handleGroupAccessDialogKeydown)
})

onUnmounted(() => {
  stopRemoteAccessPolling()
  stopWeChatILinkSetupPolling()
  // Clean up all channel polling intervals
  for (const interval of channelPollIntervals.values()) {
    clearInterval(interval)
  }
  channelPollIntervals.clear()
  window.removeEventListener('keydown', handleGroupAccessDialogKeydown)
})

watch(
  () => tunnelStatus.value?.active,
  (active) => {
    if (active && remoteAccessState.value === 'connecting') {
      remoteAccessState.value = 'connected'
    }
  }
)

onErrorCaptured((error, _instance, info) => {
  const message = error instanceof Error ? error.message : String(error)
  pageRuntimeError.value = message || 'Channels content failed to render.'
  console.error('Channels page runtime error:', error, info)
  return false
})
</script>

<template>
  <div class="channels-page dashboard-page-frame">
    <section class="channels-stage dashboard-page-stage configuration-page-stage">
      <div class="channels-shell">
        <header class="channels-header dashboard-page-hero configuration-page-hero">
          <div class="channels-header__copy dashboard-page-copy configuration-page-copy">
            <span class="channels-header__eyebrow dashboard-page-eyebrow">
              {{ t('nav.configuration') }}
            </span>
            <h1 class="channels-page__title dashboard-page-title configuration-page-title">
              {{ t('channels.title') }}
            </h1>
            <p
              class="channels-page__description dashboard-page-description configuration-page-description"
            >
              {{ t('channels.subtitle') }}
            </p>
          </div>
        </header>

        <section class="channels-summary-grid">
          <article class="dashboard-card-surface channels-summary-card">
            <span class="channels-summary-label">{{ t('channels.enabledChannels') }}</span>
            <strong class="channels-summary-value">{{ enabledCount }}</strong>
          </article>

          <article
            class="dashboard-card-surface channels-summary-card channels-summary-card--connected"
          >
            <span class="channels-summary-label">{{ t('channels.connectedChannels') }}</span>
            <strong class="channels-summary-value">{{ connectedCount }}</strong>
          </article>

          <article
            class="dashboard-card-surface channels-summary-card channels-summary-card--preview"
          >
            <span class="channels-summary-label">{{ t('channels.title') }}</span>
            <div class="channels-summary-icon-row">
              <div
                v-for="channel in heroPreviewChannels"
                :key="channel.id"
                class="channels-summary-icon-pill"
                :title="channel.nameKey ? t(channel.nameKey) : channel.name || channel.id"
              >
                <img
                  :src="channel.icon"
                  :alt="channel.nameKey ? t(channel.nameKey) : channel.name || channel.id"
                  class="channels-summary-icon"
                  :style="getChannelIconStyleVars(channel.id)"
                >
              </div>
              <div
                v-if="secondaryChannels.length > 0"
                class="channels-summary-icon-pill channels-summary-icon-pill--count"
              >
                +{{ secondaryChannels.length }}
              </div>
            </div>
          </article>

          <article
            class="dashboard-card-surface channels-summary-card channels-summary-card--group-access"
          >
            <div class="channels-summary-head">
              <span class="channels-summary-label">{{ t('channels.groupAccessTitle') }}</span>
              <div class="channels-summary-head-actions">
                <span
                  class="channels-summary-pill"
                  :class="{
                    'channels-summary-pill--open': groupAccessPolicy === 'open',
                    'channels-summary-pill--allowlist': groupAccessPolicy === 'allowlist',
                    'channels-summary-pill--disabled': groupAccessPolicy === 'disabled',
                  }"
                >
                  {{ groupAccessPolicyLabel }}
                </span>
                <button
                  type="button"
                  class="channels-summary-button"
                  @click="openGroupAccessModal"
                >
                  {{ t('common.configure') }}
                </button>
              </div>
            </div>
            <p class="channels-summary-note">
              {{ groupAccessSummaryDetail }}
            </p>
            <div
              v-if="groupAccessResult && !showGroupAccessModal"
              class="channels-summary-footer"
            >
              <p
                class="channels-summary-result"
                :class="
                  groupAccessResult.success
                    ? 'text-green-700 dark:text-green-300'
                    : 'text-red-700 dark:text-red-300'
                "
              >
                {{ groupAccessResult.message }}
              </p>
            </div>
          </article>
        </section>

        <div
          v-if="loading"
          class="text-center py-8"
        >
          <div
            class="animate-spin w-8 h-8 border-2 border-gray-900 dark:border-gray-400 border-t-transparent rounded-full mx-auto mb-2"
          />
          <p class="text-gray-500 dark:text-slate-300">
            {{ t('common.loading') }}
          </p>
        </div>

        <div
          v-else
          class="channels-board"
        >
          <div
            v-if="pageErrorMessage"
            class="channels-error-banner"
          >
            <div
              class="channels-error-banner__icon-shell"
              aria-hidden="true"
            >
              <svg
                class="channels-error-banner__icon"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="1.85"
                  d="M12 9v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                />
              </svg>
            </div>
            <div class="channels-error-banner__copy">
              <h3 class="channels-error-banner__title">
                Channels did not fully load
              </h3>
              <p class="channels-error-banner__description">
                {{ pageErrorMessage }}
              </p>
            </div>
          </div>

          <div
            v-if="pageRuntimeError"
            class="channels-safe-list"
          >
            <article
              v-for="channel in orderedChannels"
              :key="channel.id"
              class="dashboard-card-surface channels-safe-item"
            >
              <div class="channels-safe-item__title-row">
                <span class="channels-safe-item__title">
                  {{ channel.nameKey ? t(channel.nameKey) : channel.name || channel.id }}
                </span>
                <span class="channels-safe-item__status">
                  {{
                    channel.enabled
                      ? t('channels.statusConnected')
                      : t('channels.statusDisconnected')
                  }}
                </span>
              </div>
              <p class="channels-safe-item__description">
                {{ t(channel.descriptionKey) }}
              </p>
            </article>
          </div>

          <div
            v-else
            class="channels-board__content"
          >
            <div class="channels-board__main">
              <div class="channels-board__stack">
                <ChannelCardShell
                  class-prefix="channels-remote-card"
                  :title="remoteAccessTitle"
                  :description="remoteAccessDescription"
                  :description-error="remoteAccessDescriptionError"
                  :status-tone="remoteAccessStatusTone"
                  :status-text="remoteAccessStatusText"
                  :status-title="remoteAccessStatusTitle"
                  :expanded="isRemoteAccessSelected"
                  :icon-src="publicAsset('icons/tunnel/remote-access.svg')"
                  :icon-alt="remoteAccessIconAlt"
                  :meta-label="remoteAccessRecommendedLabel"
                  @header-click="toggleRemoteAccessExpanded"
                >
                  <template #actions>
                    <label
                      v-if="remoteAccessShouldShowToggle"
                      class="relative inline-flex items-center cursor-pointer"
                      @click.stop.prevent="
                        remoteAccessState === 'connected'
                          ? handleRemoteAccessStop()
                          : handleRemoteAccessStart()
                      "
                    >
                      <input
                        :checked="remoteAccessState === 'connected'"
                        type="checkbox"
                        class="sr-only peer channels-remote-card__toggle-input"
                      >
                      <div
                        class="channels-remote-card__toggle bg-gray-200 peer-focus:outline-none peer-focus:ring-2 peer-focus:ring-gray-900 dark:peer-focus:ring-gray-400 rounded-full peer dark:bg-slate-700 after:content-[''] after:absolute after:bg-white after:border-gray-300 after:border after:rounded-full after:transition-all dark:border-slate-500 peer-checked:bg-green-600 dark:peer-checked:bg-green-500 peer-disabled:opacity-50"
                      />
                    </label>
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      class="channels-remote-card__chevron"
                      :class="{ 'channels-remote-card__chevron--active': isRemoteAccessSelected }"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                      aria-hidden="true"
                    >
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M9 5l7 7-7 7"
                      />
                    </svg>
                  </template>
                </ChannelCardShell>

                <ChannelCard
                  v-for="channel in primaryChannels"
                  :key="channel.id"
                  v-memo="[
                    channel.id,
                    channel.enabled,
                    channel.status,
                    channel.lastError,
                    expandedChannel === channel.id,
                    toggling === channel.id,
                  ]"
                  :channel="channel"
                  :expanded="expandedChannel === channel.id"
                  :toggling="toggling === channel.id"
                  @toggle="toggleChannel(channel.id)"
                  @toggle-enabled="toggleChannelEnabled(channel.id, $event)"
                />
              </div>

              <button
                v-if="!showMoreChannels && secondaryChannels.length > 0"
                class="channels-load-more"
                @click="showMoreChannels = true"
              >
                <svg
                  class="w-5 h-5"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M19 9l-7 7-7-7"
                  />
                </svg>
                {{ t('common.loadMore') }} ({{ secondaryChannels.length }})
              </button>

              <div
                v-if="showMoreChannels"
                class="channels-board__stack channels-board__stack--secondary"
              >
                <ChannelCard
                  v-for="channel in secondaryChannels"
                  :key="channel.id"
                  v-memo="[
                    channel.id,
                    channel.enabled,
                    channel.status,
                    channel.lastError,
                    expandedChannel === channel.id,
                    toggling === channel.id,
                  ]"
                  :channel="channel"
                  :expanded="expandedChannel === channel.id"
                  :toggling="toggling === channel.id"
                  @toggle="toggleChannel(channel.id)"
                  @toggle-enabled="toggleChannelEnabled(channel.id, $event)"
                />
              </div>
            </div>

            <aside class="channels-board__detail">
              <RemoteAccessDetailPanel
                v-if="isRemoteAccessSelected"
                :key="remoteAccessRenderKey"
                :state="remoteAccessState"
                :error-message="remoteAccessError"
                :tunnel-status="tunnelStatus"
                :tunnel-providers="tunnelProviders"
                :selected-provider="selectedProvider"
                :selected-provider-info="selectedProviderInfo || null"
                :current-provider-token="currentProviderToken"
                :ngrok-domain="ngrokDomain"
                @select-provider="selectedProvider = $event"
                @update-provider-token="currentProviderToken = $event"
                @update-ngrok-domain="ngrokDomain = $event"
                @start="handleRemoteAccessStart"
                @stop="handleRemoteAccessStop"
                @retry="loadRemoteAccessStatus"
              />
              <div
                v-else-if="selectedChannel"
                class="channels-board__detail-stack"
              >
                <section
                  v-if="isWeChatILinkSelected"
                  class="channels-ilink-setup-card dashboard-card-surface"
                >
                  <div class="channels-ilink-setup-card__copy">
                    <p class="channels-ilink-setup-card__eyebrow">
                      {{ t('channels.wechatILinkPrimaryAction') }}
                    </p>
                    <h3 class="channels-ilink-setup-card__title">
                      {{ t('channels.wechatILinkScanAction') }}
                    </h3>
                    <p class="channels-ilink-setup-card__description">
                      {{ t('channels.wechatILinkScanHint') }}
                    </p>
                  </div>
                  <div class="channels-ilink-setup-card__actions">
                    <button
                      type="button"
                      class="channels-ilink-setup-card__primary"
                      :disabled="wechatILinkSetupPending"
                      @click="startWeChatILinkSetup"
                    >
                      {{
                        wechatILinkSetupPending
                          ? t('channels.wechatILinkSetupCreating')
                          : t('channels.wechatILinkScanAction')
                      }}
                    </button>
                  </div>
                </section>

                <ChannelDetailPanel
                  :key="selectedChannelRenderKey"
                  :channel="selectedChannel"
                  :toggling="toggling === selectedChannelId"
                  :saving="saving === selectedChannelId"
                  :testing-connection="testingConnection === selectedChannelId"
                  :test-result="testResult"
                  @toggle-enabled="toggleChannelEnabled(selectedChannelId, $event)"
                  @save="saveChannel(selectedChannelId)"
                  @test-connection="testConnection(selectedChannelId)"
                  @update-field="
                    (fieldIndex: number, value: string) =>
                      updateChannelField(selectedChannelId, fieldIndex, value)
                  "
                />
              </div>
              <article
                v-else
                class="channels-board__detail-empty dashboard-card-surface"
                aria-live="polite"
              >
                <div
                  class="channels-board__detail-empty-icon"
                  aria-hidden="true"
                >
                  <svg
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="1.8"
                      d="M9 12h6m-3-3v6m9 0a2 2 0 01-2 2H5a2 2 0 01-2-2V9a2 2 0 012-2h4l2-2h2l2 2h4a2 2 0 012 2v6z"
                    />
                  </svg>
                </div>
                <h3 class="channels-board__detail-empty-title">
                  {{ t('common.select') }} {{ t('channels.title') }}
                </h3>
                <p class="channels-board__detail-empty-copy">
                  {{ t('channels.subtitle') }}
                </p>
              </article>
            </aside>
          </div>
        </div>
      </div>
    </section>

    <Teleport to="body">
      <div
        v-if="wechatILinkSetupOpen"
        class="channels-ilink-modal-backdrop"
        @click.self="closeWeChatILinkSetupModal"
      >
        <section
          class="channels-ilink-modal"
          role="dialog"
          aria-modal="true"
        >
          <header class="channels-ilink-modal__header">
            <div>
              <p class="channels-ilink-modal__eyebrow">
                {{ t('channels.wechatILinkPrimaryAction') }}
              </p>
              <h2 class="channels-ilink-modal__title">
                {{ t('channels.wechatILinkScanAction') }}
              </h2>
            </div>
            <button
              type="button"
              class="channels-ilink-modal__close"
              :aria-label="t('common.close')"
              @click="closeWeChatILinkSetupModal"
            >
              ×
            </button>
          </header>

          <div class="channels-ilink-modal__body">
            <div
              v-if="wechatILinkSetupSession?.qrcode"
              class="channels-ilink-modal__qr"
            >
              <img
                :src="wechatILinkSetupSession.qrcode"
                :alt="t('channels.wechatILinkScanAction')"
                class="channels-ilink-modal__qr-image"
              >
            </div>

            <p class="channels-ilink-modal__description">
              {{ t('channels.wechatILinkSetupDescription') }}
            </p>

            <div
              v-if="hasWeChatILinkSetupStatus"
              class="channels-ilink-modal__status"
            >
              {{ t('channels.wechatILinkSetupStatus') }}:
              {{ getWeChatILinkSetupStatusLabel(wechatILinkSetupSession?.status) }}
            </div>

            <a
              v-if="wechatILinkSetupSession?.mobile_url"
              :href="wechatILinkSetupSession.mobile_url"
              target="_blank"
              rel="noreferrer"
              class="channels-ilink-modal__link"
            >
              {{ t('channels.wechatILinkOpenOnPhone') }}
            </a>

            <p
              v-if="wechatILinkSetupError"
              class="channels-ilink-modal__error"
            >
              {{ wechatILinkSetupError }}
            </p>
          </div>

          <footer class="channels-ilink-modal__footer">
            <button
              type="button"
              class="channels-ilink-modal__secondary"
              @click="closeWeChatILinkSetupModal"
            >
              {{ t('common.close') }}
            </button>
            <button
              v-if="canRetryWeChatILinkSetup"
              type="button"
              class="channels-ilink-modal__primary"
              @click="startWeChatILinkSetup"
            >
              {{ t('common.retry') }}
            </button>
          </footer>
        </section>
      </div>
    </Teleport>

    <Teleport to="body">
      <div
        v-if="showGroupAccessModal"
        class="channels-group-modal-backdrop"
        @click.self="closeGroupAccessModal"
      >
        <div
          class="channels-group-modal"
          role="dialog"
          aria-modal="true"
          aria-labelledby="channels-group-access-title"
        >
          <div class="channels-group-modal__header">
            <div class="channels-group-modal__copy">
              <span class="channels-group-modal__eyebrow">{{
                t('channels.groupAccessTitle')
              }}</span>
              <h2
                id="channels-group-access-title"
                class="channels-group-modal__title"
              >
                {{ t('channels.groupAccessTitle') }}
              </h2>
              <p class="channels-group-modal__description">
                {{ t('channels.groupAccessDesc') }}
              </p>
            </div>
            <button
              type="button"
              class="channels-group-modal__close"
              :disabled="savingGroupAccess"
              @click="closeGroupAccessModal"
            >
              <span class="sr-only">{{ t('common.close') }}</span>
              <svg
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                aria-hidden="true"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="1.8"
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>
            </button>
          </div>

          <div class="channels-group-modal__body">
            <div class="space-y-2">
              <label
                class="channels-policy-card__label block text-sm font-medium text-gray-700 dark:text-slate-200"
              >
                {{ t('channels.groupAccessPolicy') }}
              </label>
              <select
                v-model="groupAccessDraftPolicy"
                class="channels-policy-card__select w-full border border-gray-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-900/70 text-gray-900 dark:text-white focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400 focus:border-transparent"
              >
                <option value="open">
                  {{ t('channels.groupAccessPolicyOpen') }}
                </option>
                <option value="allowlist">
                  {{ t('channels.groupAccessPolicyAllowlist') }}
                </option>
                <option value="disabled">
                  {{ t('channels.groupAccessPolicyDisabled') }}
                </option>
              </select>
              <p class="channels-policy-card__note text-xs text-gray-500 dark:text-slate-300">
                {{ t('channels.groupAccessHint') }}
              </p>
            </div>

            <div class="space-y-2">
              <label
                class="channels-policy-card__label block text-sm font-medium text-gray-700 dark:text-slate-200"
              >
                {{ t('channels.groupAccessMentionPolicy') }}
              </label>
              <select
                v-model="groupAccessDraftMentionPolicy"
                class="channels-policy-card__select w-full border border-gray-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-900/70 text-gray-900 dark:text-white focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400 focus:border-transparent"
              >
                <option value="mentioned">
                  {{ t('channels.groupAccessMentionPolicyMentioned') }}
                </option>
                <option value="always">
                  {{ t('channels.groupAccessMentionPolicyAlways') }}
                </option>
              </select>
              <p class="channels-policy-card__note text-xs text-gray-500 dark:text-slate-300">
                {{ t('channels.groupAccessMentionHint') }}
              </p>
            </div>

            <div
              v-if="groupAccessDraftPolicy === 'allowlist'"
              class="space-y-2"
            >
              <label
                class="channels-policy-card__label block text-sm font-medium text-gray-700 dark:text-slate-200"
              >
                {{ t('channels.groupAccessAllowedChats') }}
              </label>
              <textarea
                v-model="groupAccessDraftAllowedChatIDsText"
                class="channels-policy-card__textarea w-full border border-gray-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-900/70 text-gray-900 dark:text-white focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400 focus:border-transparent"
                :placeholder="t('channels.groupAccessAllowedChatsPlaceholder')"
              />
              <p class="channels-policy-card__note text-xs text-gray-500 dark:text-slate-300">
                {{ t('channels.groupAccessAllowedChatsHint') }}
              </p>
            </div>
          </div>

          <div class="channels-group-modal__footer">
            <p
              v-if="groupAccessResult"
              class="channels-group-modal__result"
              :class="
                groupAccessResult.success
                  ? 'text-green-700 dark:text-green-300'
                  : 'text-red-700 dark:text-red-300'
              "
            >
              {{ groupAccessResult.message }}
            </p>

            <div class="channels-group-modal__actions">
              <button
                type="button"
                class="channels-group-modal__secondary-action"
                :disabled="savingGroupAccess"
                @click="closeGroupAccessModal"
              >
                {{ t('common.cancel') }}
              </button>
              <button
                type="button"
                class="channels-group-modal__primary-action"
                :disabled="savingGroupAccess"
                :class="{ 'opacity-60 cursor-not-allowed': savingGroupAccess }"
                @click="saveGroupAccessSettings"
              >
                {{ savingGroupAccess ? t('channels.saving') : t('common.save') }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
.channels-page {
  --channels-accent: 37, 99, 235;
  --dashboard-page-accent: var(--channels-accent);
  width: 100%;
  max-width: 1480px;
  margin: 0 auto;
  padding: 0 0.75rem 1.8rem;
  color: #0f172a;
}

.channels-stage {
  position: relative;
  padding: 1.15rem 0 0.35rem;
}

.channels-shell {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 1rem;
  max-width: 82rem;
  margin: 0 auto;
}

.channels-header {
  position: relative;
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  padding: 0 0 0.2rem;
  margin-bottom: 0;
  border: 0;
  background: transparent;
  box-shadow: none;
}

.channels-header__copy {
  position: relative;
  z-index: 1;
  flex: 1 1 0%;
  min-width: 0;
  max-width: 42rem;
  padding-top: 0.1rem;
}

.channels-page__title {
  font-size: clamp(1.34rem, 0.7vw + 0.95rem, 1.9rem);
  line-height: 1.06;
  letter-spacing: -0.04em;
  font-weight: 700;
  color: #111827;
}

.channels-page__description {
  margin: 0.42rem 0 0;
  max-width: 34rem;
  font-size: 0.92rem;
  line-height: 1.55;
  color: #9ca3af;
}

.channels-summary-grid {
  position: relative;
  z-index: 1;
  display: grid;
  gap: 0.62rem;
  grid-template-columns: repeat(1, minmax(0, 1fr));
  margin-bottom: 0.72rem;
}

.channels-summary-card {
  position: relative;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 0.36rem;
  min-height: 4.9rem;
  padding: 0.78rem 0.88rem;
}

.channels-summary-card::after {
  content: none;
}

.channels-page .dashboard-card-surface {
  border-color: rgba(226, 232, 240, 0.92);
  background: #ffffff;
  box-shadow: none;
}

.channels-summary-card--connected {
  color: #15803d;
}

.channels-summary-card--preview {
  color: #475569;
}

.channels-summary-label {
  position: relative;
  z-index: 1;
  display: block;
  color: #9ca3af;
  font-size: 0.56rem;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.channels-summary-value {
  position: relative;
  z-index: 1;
  display: block;
  color: #111827;
  font-size: clamp(1.18rem, 0.7vw + 0.92rem, 1.7rem);
  line-height: 1.04;
  letter-spacing: -0.04em;
  font-weight: 700;
}

.channels-summary-icon-row {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: center;
  flex-wrap: nowrap;
  gap: clamp(0.16rem, 0.08rem + 0.16vw, 0.32rem);
}

.channels-summary-icon-pill {
  width: clamp(1.34rem, 1.22rem + 0.28vw, 1.6rem);
  height: clamp(1.34rem, 1.22rem + 0.28vw, 1.6rem);
  flex: 0 0 auto;
  display: grid;
  place-items: center;
  border-radius: clamp(0.46rem, 0.4rem + 0.12vw, 0.56rem);
  background: rgba(255, 255, 255, 0.9);
  border: 1px solid rgba(203, 213, 225, 0.9);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.9);
}

.channels-summary-icon-pill img {
  width: clamp(0.72rem, 0.64rem + 0.12vw, 0.84rem);
  height: clamp(0.72rem, 0.64rem + 0.12vw, 0.84rem);
}

.channels-summary-icon {
  width: clamp(0.72rem, 0.64rem + 0.12vw, 0.84rem);
  height: clamp(0.72rem, 0.64rem + 0.12vw, 0.84rem);
  object-fit: contain;
  transform: scale(var(--channel-icon-scale, 1));
  transform-origin: center;
}

.channels-summary-icon-pill--count {
  width: auto;
  min-width: clamp(1.54rem, 1.38rem + 0.24vw, 1.76rem);
  padding: 0 0.3rem;
  font-size: 0.58rem;
  font-weight: 700;
  color: #475569;
}

.channels-summary-card--group-access {
  justify-content: flex-start;
  gap: 0.3rem;
  min-height: 0;
  padding: 0.68rem 0.84rem;
}

.channels-summary-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.42rem;
}

.channels-summary-head-actions {
  display: inline-flex;
  align-items: center;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 0.34rem;
}

.channels-summary-pill {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0.18rem 0.42rem;
  border-radius: 999px;
  font-size: 0.56rem;
  font-weight: 700;
  white-space: nowrap;
}

.channels-summary-pill--open {
  background: rgba(220, 252, 231, 0.95);
  color: #166534;
}

.channels-summary-pill--allowlist {
  background: rgba(254, 249, 195, 0.95);
  color: #854d0e;
}

.channels-summary-pill--disabled {
  background: rgba(254, 226, 226, 0.95);
  color: #b91c1c;
}

.channels-summary-note {
  font-size: 0.66rem;
  line-height: 1.35;
  color: #64748b;
  min-height: 0;
}

.channels-summary-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  margin-top: 0.04rem;
}

.channels-summary-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 1.72rem;
  padding: 0.28rem 0.62rem;
  border-radius: 0.62rem;
  border: 0;
  background: #111827;
  color: #ffffff;
  font-size: 0.62rem;
  font-weight: 700;
  transition:
    transform 160ms ease,
    background-color 160ms ease;
}

.channels-summary-button:hover {
  transform: translateY(-1px);
  background: #030712;
}

.channels-summary-result {
  font-size: 0.64rem;
  line-height: 1.35;
  text-align: end;
}

.channels-board {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 0.72rem;
  padding: 0;
  border-radius: 0;
  background: transparent;
  border: 0;
}

.channels-board::before {
  content: none;
}

.channels-board__content {
  display: grid;
  gap: 0.88rem;
}

.channels-board__main,
.channels-board__detail {
  min-width: 0;
}

.channels-board__main {
  display: flex;
  flex-direction: column;
  gap: 0.72rem;
}

.channels-board__detail-empty {
  display: flex;
  min-height: 100%;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.78rem;
  padding: 2rem 1.4rem;
  text-align: center;
}

.channels-board__detail-empty-icon {
  display: grid;
  width: 3.2rem;
  height: 3.2rem;
  place-items: center;
  border-radius: 999px;
  border: 1px solid rgba(203, 213, 225, 0.96);
  background: rgba(248, 250, 252, 0.96);
  color: #64748b;
}

.channels-board__detail-empty-icon svg {
  width: 1.4rem;
  height: 1.4rem;
}

.channels-board__detail-empty-title {
  font-size: 0.92rem;
  font-weight: 700;
  color: #0f172a;
}

.channels-board__detail-empty-copy {
  max-width: 24rem;
  font-size: 0.76rem;
  line-height: 1.5;
  color: #64748b;
}

.channels-board__detail-stack {
  display: grid;
  gap: 1rem;
}

.channels-ilink-setup-card {
  display: grid;
  gap: 1rem;
  padding: 1.4rem;
  border-radius: 1.5rem;
  border: 1px solid rgba(34, 197, 94, 0.16);
  background:
    radial-gradient(circle at top right, rgba(34, 197, 94, 0.16), transparent 38%),
    linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(248, 250, 252, 0.98));
}

.channels-ilink-setup-card__copy {
  display: grid;
  gap: 0.44rem;
}

.channels-ilink-setup-card__eyebrow {
  margin: 0;
  font-size: 0.7rem;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: #166534;
}

.channels-ilink-setup-card__title {
  margin: 0;
  font-size: 1.14rem;
  line-height: 1.15;
  color: #0f172a;
}

.channels-ilink-setup-card__description {
  margin: 0;
  color: #475569;
  line-height: 1.55;
}

.channels-ilink-setup-card__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.72rem;
}

.channels-ilink-setup-card__primary,
.channels-ilink-setup-card__secondary {
  border-radius: 999px;
  padding: 0.68rem 1rem;
  font: inherit;
  font-weight: 700;
}

.channels-ilink-setup-card__primary {
  border: none;
  background: linear-gradient(135deg, #15803d 0%, #22c55e 100%);
  color: #fff;
}

.channels-ilink-setup-card__secondary {
  border: 1px solid rgba(21, 128, 61, 0.18);
  background: rgba(255, 255, 255, 0.84);
  color: #166534;
}

.channels-ilink-modal-backdrop {
  position: fixed;
  inset: 0;
  z-index: 80;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 1.5rem;
  background: rgba(15, 23, 42, 0.5);
  backdrop-filter: blur(14px);
}

.channels-ilink-modal {
  width: min(100%, 27.5rem);
  border-radius: 1.75rem;
  background: rgba(255, 255, 255, 0.96);
  border: 1px solid rgba(148, 163, 184, 0.2);
  box-shadow: 0 28px 80px rgba(15, 23, 42, 0.24);
  overflow: hidden;
}

.channels-ilink-modal__header,
.channels-ilink-modal__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 1.1rem 1.35rem;
}

.channels-ilink-modal__header {
  border-bottom: 1px solid rgba(226, 232, 240, 0.85);
}

.channels-ilink-modal__footer {
  border-top: 1px solid rgba(226, 232, 240, 0.85);
}

.channels-ilink-modal__eyebrow {
  margin: 0 0 0.32rem;
  font-size: 0.7rem;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: #166534;
}

.channels-ilink-modal__title {
  margin: 0;
  font-size: 1.16rem;
  line-height: 1.15;
  color: #0f172a;
}

.channels-ilink-modal__close {
  border: none;
  background: transparent;
  color: #64748b;
  font-size: 1.8rem;
  line-height: 1;
}

.channels-ilink-modal__body {
  display: grid;
  gap: 1rem;
  padding: 1.35rem;
}

.channels-ilink-modal__qr {
  display: flex;
  justify-content: center;
}

.channels-ilink-modal__qr-image {
  width: min(100%, 15rem);
  border-radius: 1.2rem;
  border: 1px solid rgba(148, 163, 184, 0.2);
  background: #fff;
  padding: 0.62rem;
}

.channels-ilink-modal__description {
  margin: 0;
  color: #475569;
  line-height: 1.6;
}

.channels-ilink-modal__status {
  border-radius: 1rem;
  background: rgba(15, 23, 42, 0.04);
  padding: 0.78rem 0.92rem;
  color: #0f172a;
}

.channels-ilink-modal__link {
  color: #166534;
  font-weight: 700;
  text-decoration: none;
}

.channels-ilink-modal__error {
  margin: 0;
  color: #b91c1c;
}

.channels-ilink-modal__primary,
.channels-ilink-modal__secondary {
  border-radius: 999px;
  padding: 0.68rem 1rem;
  font: inherit;
  font-weight: 700;
}

.channels-ilink-modal__primary {
  border: none;
  background: linear-gradient(135deg, #15803d 0%, #22c55e 100%);
  color: #fff;
}

.channels-ilink-modal__secondary {
  border: 1px solid rgba(148, 163, 184, 0.3);
  background: rgba(255, 255, 255, 0.88);
  color: #0f172a;
}

.channels-error-banner {
  display: flex;
  align-items: flex-start;
  gap: 0.78rem;
  padding: 0.88rem 0.96rem;
  border-radius: 1rem;
  border: 1px solid rgba(252, 165, 165, 0.7);
  background: rgba(254, 242, 242, 0.96);
}

.channels-error-banner__icon-shell {
  width: 2rem;
  height: 2rem;
  flex-shrink: 0;
  display: grid;
  place-items: center;
  border-radius: 0.7rem;
  background: rgba(254, 226, 226, 0.9);
  color: #b91c1c;
}

.channels-error-banner__icon {
  width: 1rem;
  height: 1rem;
}

.channels-error-banner__copy {
  min-width: 0;
  flex: 1;
}

.channels-error-banner__title {
  font-size: 0.82rem;
  font-weight: 700;
  color: #991b1b;
}

.channels-error-banner__description {
  margin-top: 0.16rem;
  font-size: 0.72rem;
  line-height: 1.45;
  color: #b91c1c;
}

.channels-board__stack {
  position: relative;
  z-index: 1;
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  align-items: start;
  gap: 0.72rem;
}

.channels-board__stack--secondary {
  padding-top: 0;
}

.channels-safe-list {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 0.72rem;
}

.channels-safe-item {
  display: flex;
  flex-direction: column;
  gap: 0.32rem;
  padding: 0.92rem 0.96rem;
  border-radius: 1rem;
}

.channels-safe-item__title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.72rem;
}

.channels-safe-item__title {
  font-size: 0.8rem;
  font-weight: 700;
  color: #111827;
}

.channels-safe-item__status {
  font-size: 0.62rem;
  font-weight: 700;
  letter-spacing: 0.02em;
  color: #475569;
}

.channels-safe-item__description {
  font-size: 0.7rem;
  line-height: 1.45;
  color: #64748b;
}

.channels-group-modal-backdrop {
  position: fixed;
  inset: 0;
  z-index: 60;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 1rem;
  background: rgba(15, 23, 42, 0.56);
  backdrop-filter: blur(10px);
}

.channels-group-modal {
  width: min(100%, 42rem);
  max-height: min(100vh - 2rem, 44rem);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border-radius: 1.4rem;
  border: 1px solid rgba(226, 232, 240, 0.96);
  background: #ffffff;
  box-shadow: 0 24px 80px rgba(15, 23, 42, 0.28);
}

.channels-group-modal__header,
.channels-group-modal__footer {
  display: flex;
  align-items: flex-start;
  gap: 1rem;
  padding: 1rem 1.08rem;
  border-bottom: 1px solid rgba(226, 232, 240, 0.96);
}

.channels-group-modal__copy {
  min-width: 0;
  flex: 1;
}

.channels-group-modal__eyebrow {
  display: inline-flex;
  align-items: center;
  padding: 0.2rem 0.44rem;
  border-radius: 999px;
  background: #f8fafc;
  color: #475569;
  font-size: 0.58rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.channels-group-modal__title {
  margin-top: 0.58rem;
  font-size: 1.02rem;
  line-height: 1.15;
  font-weight: 700;
  color: #111827;
}

.channels-group-modal__description {
  margin-top: 0.32rem;
  font-size: 0.78rem;
  line-height: 1.5;
  color: #64748b;
}

.channels-group-modal__close {
  width: 2.2rem;
  height: 2.2rem;
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 0.8rem;
  border: 1px solid rgba(203, 213, 225, 0.96);
  background: #f8fafc;
  color: #475569;
  transition:
    background-color 160ms ease,
    border-color 160ms ease;
}

.channels-group-modal__close:hover {
  background: #f1f5f9;
  border-color: rgba(148, 163, 184, 0.96);
}

.channels-group-modal__close svg {
  width: 1rem;
  height: 1rem;
}

.channels-group-modal__body {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  overflow-y: auto;
  padding: 1rem 1.08rem;
  background: #f8fafc;
}

.channels-group-modal__footer {
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  border-top: 1px solid rgba(226, 232, 240, 0.96);
  border-bottom: 0;
  background: #ffffff;
}

.channels-group-modal__result {
  flex: 1 1 14rem;
  font-size: 0.72rem;
  line-height: 1.45;
}

.channels-group-modal__actions {
  margin-inline-start: auto;
  display: flex;
  align-items: center;
  gap: 0.62rem;
}

.channels-group-modal__primary-action,
.channels-group-modal__secondary-action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 2.14rem;
  padding: 0.42rem 0.9rem;
  border-radius: 0.78rem;
  font-size: 0.72rem;
  font-weight: 700;
  transition: background-color 160ms ease;
}

.channels-group-modal__primary-action {
  border: 0;
  background: #111827;
  color: #ffffff;
}

.channels-group-modal__primary-action:hover {
  background: #030712;
}

.channels-group-modal__secondary-action {
  border: 1px solid rgba(203, 213, 225, 0.96);
  background: #ffffff;
  color: #475569;
}

.channels-group-modal__secondary-action:hover {
  background: #f8fafc;
}

.channels-policy-card {
  overflow: hidden;
  border-radius: 1.25rem;
  border: 1px solid rgba(226, 232, 240, 0.96);
  background: #ffffff;
  box-shadow: none;
}

.channels-policy-card__header {
  display: flex;
  align-items: center;
  gap: 0.84rem;
  min-height: 4.6rem;
  padding: 0.86rem 0.96rem;
}

.channels-policy-card__identity {
  min-width: 0;
  flex: 1;
  display: flex;
  align-items: center;
  gap: 0.74rem;
}

.channels-policy-card__icon-shell {
  width: 2.32rem;
  height: 2.32rem;
  flex-shrink: 0;
  display: grid;
  place-items: center;
  overflow: hidden;
  border-radius: 0.74rem;
  background: #f8fafc;
  border: 1px solid rgba(226, 232, 240, 0.96);
  box-shadow: none;
}

.channels-policy-card__icon {
  width: 1.18rem;
  height: 1.18rem;
  color: #334155;
}

.channels-policy-card__copy {
  min-width: 0;
  flex: 1;
}

.channels-policy-card__title {
  font-size: 0.86rem;
  font-weight: 700;
}

.channels-policy-card__description {
  margin-top: 0.22rem;
  font-size: 0.74rem;
  line-height: 1.38;
}

.channels-policy-card__body {
  display: flex;
  flex-direction: column;
  gap: 0.9rem;
  border-top: 1px solid rgba(226, 232, 240, 0.96);
  padding: 0.92rem;
  background: #f8fafc;
}

.channels-policy-card__label {
  font-size: 0.68rem;
}

.channels-policy-card__select,
.channels-policy-card__textarea {
  width: 100%;
  min-height: 2.08rem;
  padding: 0.4rem 0.54rem;
  border-radius: 0.66rem;
  font-size: 0.7rem;
}

.channels-policy-card__textarea {
  min-height: 7.5rem;
  resize: vertical;
  font-family:
    ui-monospace, SFMono-Regular, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono',
    'Courier New', monospace;
}

.channels-policy-card__note {
  font-size: 0.58rem;
  line-height: 1.45;
}

.channels-policy-card__footer {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.72rem;
}

.channels-policy-card__primary-action {
  min-height: 2.14rem;
  padding: 0.42rem 0.9rem;
  border-radius: 0.72rem;
  font-size: 0.7rem;
}

.channels-policy-card__result {
  font-size: 0.72rem;
  line-height: 1.4;
}

.channels-remote-card__toggle {
  width: 2.16rem;
  height: 1.16rem;
  position: relative;
}

.channels-remote-card__toggle::after {
  top: 2px;
  inset-inline-start: 2px;
  width: 0.82rem;
  height: 0.82rem;
}

.channels-remote-card__toggle-input:checked + .channels-remote-card__toggle::after {
  inset-inline-start: calc(100% - 0.82rem - 2px);
  border-color: #ffffff;
}

.channels-remote-card__chevron {
  width: 0.92rem;
  height: 0.92rem;
  color: #94a3b8;
  transition:
    transform 160ms ease,
    color 160ms ease;
}

.channels-remote-card__chevron--active {
  color: #0f172a;
  transform: translateX(2px);
}

.channels-load-more {
  width: 100%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.6rem;
  padding: 0.68rem 0.82rem;
  border-radius: 0.72rem;
  border: 1px solid rgba(203, 213, 225, 0.96);
  background: rgba(255, 255, 255, 0.96);
  color: #475569;
  font-size: 0.72rem;
  font-weight: 700;
  transition:
    transform 160ms ease,
    border-color 160ms ease;
}

.channels-load-more:hover {
  transform: translateY(-1px);
  border-color: rgba(148, 163, 184, 0.95);
}

@media (min-width: 760px) {
  .channels-summary-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (min-width: 960px) {
  .channels-summary-grid {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }

  .channels-board__content {
    grid-template-columns: minmax(0, 0.94fr) minmax(22rem, 0.96fr);
    align-items: start;
  }

  .channels-board__detail {
    position: sticky;
    top: 1rem;
  }

  .channels-safe-list {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 768px) {
  .channels-stage {
    padding-top: 0.82rem;
  }
}

@media (max-width: 639px) {
  .channels-page {
    padding-inline: 0.58rem;
  }

  .channels-group-modal {
    max-height: min(100vh - 1.5rem, 100%);
  }

  .channels-group-modal__header,
  .channels-group-modal__body,
  .channels-group-modal__footer {
    padding-inline: 0.92rem;
  }

  .channels-group-modal__footer,
  .channels-group-modal__actions {
    width: 100%;
  }

  .channels-group-modal__actions {
    margin-inline-start: 0;
    flex-direction: column-reverse;
  }

  .channels-group-modal__primary-action,
  .channels-group-modal__secondary-action {
    width: 100%;
    justify-content: center;
  }
}

:root.dark .channels-summary-label,
[data-theme='dark'] .channels-summary-label,
html.dark .channels-summary-label {
  color: #cbd5e1;
}

:root.dark .channels-page,
[data-theme='dark'] .channels-page,
html.dark .channels-page {
  color: #e2e8f0;
}

:root.dark .channels-board__detail-empty,
[data-theme='dark'] .channels-board__detail-empty,
html.dark .channels-board__detail-empty {
  background: #111827;
}

:root.dark .channels-board__detail-empty-icon,
[data-theme='dark'] .channels-board__detail-empty-icon,
html.dark .channels-board__detail-empty-icon {
  border-color: rgba(71, 85, 105, 0.46);
  background: rgba(15, 23, 42, 0.72);
  color: #94a3b8;
}

:root.dark .channels-board__detail-empty-title,
[data-theme='dark'] .channels-board__detail-empty-title,
html.dark .channels-board__detail-empty-title {
  color: #f8fafc;
}

:root.dark .channels-board__detail-empty-copy,
[data-theme='dark'] .channels-board__detail-empty-copy,
html.dark .channels-board__detail-empty-copy {
  color: #cbd5e1;
}

:root.dark .channels-page :deep(.dashboard-card-surface),
[data-theme='dark'] .channels-page :deep(.dashboard-card-surface),
html.dark .channels-page :deep(.dashboard-card-surface) {
  background: #111827;
  border-color: rgba(71, 85, 105, 0.46);
  box-shadow: none;
}

:root.dark .channels-remote-card__chevron,
[data-theme='dark'] .channels-remote-card__chevron,
html.dark .channels-remote-card__chevron {
  color: #64748b;
}

:root.dark .channels-remote-card__chevron--active,
[data-theme='dark'] .channels-remote-card__chevron--active,
html.dark .channels-remote-card__chevron--active {
  color: #f8fafc;
}

:root.dark .channels-summary-value,
[data-theme='dark'] .channels-summary-value,
html.dark .channels-summary-value,
:root.dark .channels-summary-icon-pill--count,
[data-theme='dark'] .channels-summary-icon-pill--count,
html.dark .channels-summary-icon-pill--count {
  color: #f8fafc;
}

:root.dark .channels-summary-note,
[data-theme='dark'] .channels-summary-note,
html.dark .channels-summary-note,
:root.dark .channels-group-modal__description,
[data-theme='dark'] .channels-group-modal__description,
html.dark .channels-group-modal__description {
  color: #cbd5e1;
}

:root.dark .channels-summary-button,
[data-theme='dark'] .channels-summary-button,
html.dark .channels-summary-button,
:root.dark .channels-group-modal__primary-action,
[data-theme='dark'] .channels-group-modal__primary-action,
html.dark .channels-group-modal__primary-action {
  background: #94a3b8;
  color: #0f172a;
}

:root.dark .channels-summary-button:hover,
[data-theme='dark'] .channels-summary-button:hover,
html.dark .channels-summary-button:hover,
:root.dark .channels-group-modal__primary-action:hover,
[data-theme='dark'] .channels-group-modal__primary-action:hover,
html.dark .channels-group-modal__primary-action:hover {
  background: #cbd5e1;
}

:root.dark .channels-summary-pill--open,
[data-theme='dark'] .channels-summary-pill--open,
html.dark .channels-summary-pill--open {
  background: rgba(34, 197, 94, 0.2);
  color: #86efac;
}

:root.dark .channels-summary-pill--allowlist,
[data-theme='dark'] .channels-summary-pill--allowlist,
html.dark .channels-summary-pill--allowlist {
  background: rgba(250, 204, 21, 0.2);
  color: #fde68a;
}

:root.dark .channels-summary-pill--disabled,
[data-theme='dark'] .channels-summary-pill--disabled,
html.dark .channels-summary-pill--disabled {
  background: rgba(239, 68, 68, 0.2);
  color: #fca5a5;
}

:root.dark .channels-group-modal,
[data-theme='dark'] .channels-group-modal,
html.dark .channels-group-modal {
  background: #111827;
  border-color: rgba(71, 85, 105, 0.46);
}

:root.dark .channels-group-modal__header,
[data-theme='dark'] .channels-group-modal__header,
html.dark .channels-group-modal__header,
:root.dark .channels-group-modal__footer,
[data-theme='dark'] .channels-group-modal__footer,
html.dark .channels-group-modal__footer {
  border-color: rgba(71, 85, 105, 0.46);
}

:root.dark .channels-group-modal__body,
[data-theme='dark'] .channels-group-modal__body,
html.dark .channels-group-modal__body {
  background: #1e293b;
}

:root.dark .channels-group-modal__title,
[data-theme='dark'] .channels-group-modal__title,
html.dark .channels-group-modal__title {
  color: #f8fafc;
}

:root.dark .channels-group-modal__eyebrow,
[data-theme='dark'] .channels-group-modal__eyebrow,
html.dark .channels-group-modal__eyebrow,
:root.dark .channels-group-modal__close,
[data-theme='dark'] .channels-group-modal__close,
html.dark .channels-group-modal__close,
:root.dark .channels-group-modal__secondary-action,
[data-theme='dark'] .channels-group-modal__secondary-action,
html.dark .channels-group-modal__secondary-action {
  background: rgba(15, 23, 42, 0.72);
  border-color: rgba(71, 85, 105, 0.5);
  color: #cbd5e1;
}

:root.dark .channels-group-modal__close:hover,
[data-theme='dark'] .channels-group-modal__close:hover,
html.dark .channels-group-modal__close:hover,
:root.dark .channels-group-modal__secondary-action:hover,
[data-theme='dark'] .channels-group-modal__secondary-action:hover,
html.dark .channels-group-modal__secondary-action:hover {
  background: rgba(30, 41, 59, 0.9);
}

:root.dark .channels-summary-icon-pill,
[data-theme='dark'] .channels-summary-icon-pill,
html.dark .channels-summary-icon-pill {
  background: rgba(15, 23, 42, 0.72);
  border-color: rgba(71, 85, 105, 0.5);
}

:root.dark .channels-board,
[data-theme='dark'] .channels-board,
html.dark .channels-board {
  background: transparent;
  border-color: transparent;
}

:root.dark .channels-board::before,
[data-theme='dark'] .channels-board::before,
html.dark .channels-board::before {
  content: none;
}

:root.dark .channels-load-more,
[data-theme='dark'] .channels-load-more,
html.dark .channels-load-more {
  background: #111827;
  border-color: rgba(71, 85, 105, 0.46);
  color: #cbd5e1;
}

:root.dark .channels-error-banner,
[data-theme='dark'] .channels-error-banner,
html.dark .channels-error-banner {
  border-color: rgba(248, 113, 113, 0.34);
  background: rgba(69, 10, 10, 0.5);
}

:root.dark .channels-error-banner__icon-shell,
[data-theme='dark'] .channels-error-banner__icon-shell,
html.dark .channels-error-banner__icon-shell {
  background: rgba(127, 29, 29, 0.65);
  color: #fca5a5;
}

:root.dark .channels-error-banner__title,
[data-theme='dark'] .channels-error-banner__title,
html.dark .channels-error-banner__title {
  color: #fecaca;
}

:root.dark .channels-error-banner__description,
[data-theme='dark'] .channels-error-banner__description,
html.dark .channels-error-banner__description {
  color: #fca5a5;
}

:root.dark .channels-safe-item__title,
[data-theme='dark'] .channels-safe-item__title,
html.dark .channels-safe-item__title {
  color: #f8fafc;
}

:root.dark .channels-safe-item__status,
[data-theme='dark'] .channels-safe-item__status,
html.dark .channels-safe-item__status {
  color: #cbd5e1;
}

:root.dark .channels-safe-item__description,
[data-theme='dark'] .channels-safe-item__description,
html.dark .channels-safe-item__description {
  color: #94a3b8;
}
</style>
