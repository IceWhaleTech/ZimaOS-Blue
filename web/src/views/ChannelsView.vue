<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, shallowRef, triggerRef } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSettingsStore } from '@/stores/settings'
import { getChannelIconOrDefault, getTunnelProviderIcon } from '@/utils/channelIcons'
import ChannelCard from '@/components/channels/ChannelCard.vue'
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
import TunnelStatus from '@/components/remote-access/TunnelStatus.vue'

const { t, te, locale: i18nLocale } = useI18n()
const settingsStore = useSettingsStore()

interface ChannelFieldDef {
  key: string
  labelKey: string
  type: 'text' | 'password' | 'tel' | 'url' | 'textarea'
  placeholder?: string
  placeholderKey?: string
  value: string
  required?: boolean
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

const loading = ref(false)
const saving = ref<string | null>(null)
const toggling = ref<string | null>(null)
const testingConnection = ref<string | null>(null)
const testResult = ref<{ channelId: string; success: boolean; message: string } | null>(null)

// Resolve a channel error message: prefer i18n key, fallback to raw string
function resolveChannelError(channel: ChannelDef): string {
  if (channel.lastErrorKey) {
    const i18nKey = `channels.errors.${channel.lastErrorKey}`
    if (te(i18nKey)) return t(i18nKey)
  }
  return channel.lastError || ''
}

// Start collapsed; the primary section still keeps locale favorites,
// self-hosted channels, and any enabled channels visible by default.
const getInitialShowMoreState = (): boolean => {
  return false
}

const showMoreChannels = ref(getInitialShowMoreState())

// Localized channel ordering based on user's language/region
const getLocalizedChannelOrder = (): string[] => {
  // Priority 1: Use backend settings locale, fallback to current i18n locale
  const locale = settingsStore.backendSettings.locale || i18nLocale.value || 'en-US'

  // Chinese regions (Mainland China)
  if (locale === 'zh-CN') {
    return ['wechat', 'dingtalk', 'feishu', 'qq', 'telegram', 'imessage']
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

// Remote Access state
type RemoteAccessState = 'loading' | 'ready' | 'connecting' | 'connected' | 'error'
const remoteAccessState = ref<RemoteAccessState>('loading')
const tunnelStatus = ref<TunnelStatusType | null>(null)
const remoteAccessError = ref<string | null>(null)
const remoteAccessExpanded = ref(false)
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

const heroPreviewChannels = computed(() => primaryChannels.value.slice(0, defaultVisibleChannelCount))

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
    const missingFields = channelDef.fields.filter((f) => f.required && !f.value)
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
    const response = await fetch(`/api/channels/${channelId}/toggle`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enabled }),
    })

    const data = await response.json()
    if (response.ok) {
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
  try {
    const response = await fetch('/api/channels')
    if (response.ok) {
      const data = await response.json()
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
            if (serverChannel.config && serverChannel.config[field.key]) {
              field.value = serverChannel.config[field.key]
            }
          }
        }
      }
      triggerRef(channelDefs)
    }
  } catch (err) {
    console.error('Failed to load channel configs:', err)
  } finally {
    loading.value = false
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

    const response = await fetch(`/api/channels/${channelId}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        enabled: channelDef.enabled,
        config,
      }),
    })

    const data = await response.json()
    if (response.ok) {
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
      const response = await fetch(`/api/channels/${channelId}/status`)
      if (!response.ok) {
        // API error, stop polling
        clearInterval(interval)
        channelPollIntervals.delete(channelId)
        return
      }

      const data = await response.json()
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

    const response = await fetch('/api/setup/test-connection', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ type: channelId, config }),
    })

    const data = await response.json()
    // Use message_key for i18n translation if available, fallback to message
    let message = data.message
    if (data.message_key) {
      const i18nKey = `channels.validation.${data.message_key}`
      // Check if translation exists, otherwise use original message
      message = te(i18nKey) ? t(i18nKey) : data.message
    }
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
  const isDesktop = typeof window !== 'undefined' && !!(window as any).__BLUE_DESKTOP__

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
  remoteAccessExpanded.value = !remoteAccessExpanded.value
}

function updateChannelField(channelId: string, fieldIndex: number, value: string) {
  const channel = channelMap.value.get(channelId)
  if (channel && channel.fields[fieldIndex]) {
    channel.fields[fieldIndex].value = value
    triggerRef(channelDefs)
  }
}

onMounted(() => {
  // Load backend settings first to get locale
  settingsStore.fetchBackendSettings()
  loadChannelConfigs()
  loadRemoteAccessStatus()
})

onUnmounted(() => {
  stopRemoteAccessPolling()
  // Clean up all channel polling intervals
  for (const interval of channelPollIntervals.values()) {
    clearInterval(interval)
  }
  channelPollIntervals.clear()
})

watch(
  () => tunnelStatus.value?.active,
  (active) => {
    if (active && remoteAccessState.value === 'connecting') {
      remoteAccessState.value = 'connected'
    }
  }
)
</script>

<template>
  <div class="channels-page config-page-frame">
    <section class="channels-stage config-page-stage">
      <section class="channels-header config-page-hero-surface">
        <div class="channels-header__copy config-page-hero__copy">
          <p class="config-page-hero__eyebrow">{{ t('nav.configuration') }}</p>
          <h1 class="channels-page__title config-page-hero__title">{{ t('channels.title') }}</h1>
          <p class="channels-page__description config-page-hero__description">
            {{ t('channels.subtitle') }}
          </p>
        </div>
      </section>

      <section class="channels-summary-grid">
        <article class="dashboard-card-surface channels-summary-card">
          <span class="channels-summary-label">{{ t('channels.enabledChannels') }}</span>
          <strong class="channels-summary-value">{{ enabledCount }}</strong>
          <p class="channels-summary-footnote">
            {{ connectedCount }} {{ t('channels.connectedChannels') }}
          </p>
        </article>

        <article class="dashboard-card-surface channels-summary-card channels-summary-card--connected">
          <span class="channels-summary-label">{{ t('channels.connectedChannels') }}</span>
          <strong class="channels-summary-value">{{ connectedCount }}</strong>
          <p class="channels-summary-footnote">
            {{ enabledCount }} {{ t('channels.enabledChannels') }}
          </p>
        </article>

        <article class="dashboard-card-surface channels-summary-card channels-summary-card--preview">
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
                class="w-5 h-5 object-contain"
              />
            </div>
            <div
              v-if="secondaryChannels.length > 0"
              class="channels-summary-icon-pill channels-summary-icon-pill--count"
            >
              +{{ secondaryChannels.length }}
            </div>
          </div>
          <p class="channels-summary-footnote">
            {{
              secondaryChannels.length > 0
                ? `${t('common.loadMore')} (${secondaryChannels.length})`
                : t('channels.subtitle')
            }}
          </p>
        </article>
      </section>

      <div v-if="loading" class="text-center py-8">
        <div
          class="animate-spin w-8 h-8 border-2 border-gray-900 dark:border-gray-400 border-t-transparent rounded-full mx-auto mb-2"
        ></div>
        <p class="text-gray-500 dark:text-gray-400">{{ t('common.loading') }}</p>
      </div>

      <div v-else class="channels-board">
        <div class="bg-white dark:bg-gray-700/30 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
          <div
            class="flex items-center gap-4 p-4 cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors"
            @click="toggleRemoteAccessExpanded"
          >
            <div
              class="w-10 h-10 bg-gray-100 dark:bg-gray-700 rounded-lg flex items-center justify-center overflow-hidden"
            >
              <img src="/icons/tunnel/remote-access.svg" alt="Remote Access" class="w-8 h-8" />
            </div>
            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-2 flex-wrap">
                <h3 class="font-medium text-gray-900 dark:text-white">{{ t('remoteAccess.title') }}</h3>
                <span
                  class="px-2 py-0.5 text-xs font-medium bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-full"
                >
                  {{ t('remoteAccess.recommended') }}
                </span>
                <span
                  class="w-2 h-2 rounded-full"
                  :class="{
                    'bg-green-500': remoteAccessState === 'connected',
                    'bg-yellow-500 animate-pulse': remoteAccessState === 'connecting',
                    'bg-red-500': remoteAccessState === 'error',
                    'bg-gray-400': ['loading', 'ready'].includes(remoteAccessState),
                  }"
                ></span>
              </div>
              <p class="text-sm text-gray-500 dark:text-gray-400 truncate">
                {{ t('remoteAccess.channelDescription') }}
              </p>
            </div>
            <div class="flex items-center gap-3">
              <label
                v-if="remoteAccessState === 'connected' || remoteAccessState === 'ready'"
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
                  class="sr-only peer"
                />
                <div
                  class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-2 peer-focus:ring-gray-900 dark:peer-focus:ring-gray-400 rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-green-600 dark:peer-checked:bg-green-500 peer-disabled:opacity-50"
                ></div>
              </label>
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="h-5 w-5 text-gray-400 transition-transform"
                :class="{ 'rotate-180': remoteAccessExpanded }"
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
            </div>
          </div>

          <div
            v-if="remoteAccessExpanded"
            class="border-t border-gray-200 dark:border-gray-700 p-4 bg-gray-50 dark:bg-gray-700/50"
          >
            <div v-if="remoteAccessState === 'loading'" class="flex items-center justify-center py-8">
              <svg
                class="animate-spin h-8 w-8 text-gray-900 dark:text-white"
                fill="none"
                viewBox="0 0 24 24"
              >
                <circle
                  class="opacity-25"
                  cx="12"
                  cy="12"
                  r="10"
                  stroke="currentColor"
                  stroke-width="4"
                />
                <path
                  class="opacity-75"
                  fill="currentColor"
                  d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                />
              </svg>
            </div>

            <div v-else-if="remoteAccessState === 'ready'" class="space-y-4">
              <div class="space-y-3">
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('remoteAccess.selectProvider') }}
                </label>
                <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
                  <button
                    v-for="provider in tunnelProviders"
                    :key="provider.id"
                    class="p-3 rounded-lg border text-left transition-colors flex items-center gap-3"
                    :class="
                      selectedProvider === provider.id
                        ? 'border-gray-600 dark:border-gray-600 bg-gray-100 dark:bg-gray-700/30'
                        : 'border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600'
                    "
                    @click="selectedProvider = provider.id"
                  >
                    <img
                      v-if="getTunnelProviderIcon(provider.id)"
                      :src="getTunnelProviderIcon(provider.id)"
                      :alt="provider.name"
                      class="w-6 h-6 shrink-0 rounded object-contain"
                    />
                    <div
                      v-else
                      class="w-6 h-6 shrink-0 bg-gray-700 dark:bg-gray-500 rounded flex items-center justify-center"
                    >
                      <svg
                        class="w-4 h-4 text-gray-400"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9"
                        />
                      </svg>
                    </div>
                    <div class="flex-1 min-w-0">
                      <div class="font-medium text-gray-900 dark:text-white text-sm">
                        {{ provider.name }}
                      </div>
                      <div class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
                        {{
                          provider.requires_key
                            ? t('remoteAccess.requiresKey')
                            : t('remoteAccess.noKeyRequired')
                        }}
                      </div>
                    </div>
                  </button>
                </div>
              </div>

              <div v-if="selectedProviderInfo?.requires_key" class="space-y-2">
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ selectedProviderInfo.key_label || 'Auth Token' }}
                </label>
                <input
                  v-model="currentProviderToken"
                  type="password"
                  class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400 focus:border-transparent"
                  :placeholder="selectedProviderInfo.key_hint || ''"
                />
                <a
                  v-if="selectedProviderInfo.doc_url"
                  :href="selectedProviderInfo.doc_url"
                  target="_blank"
                  class="inline-flex items-center gap-1 text-xs text-gray-900 dark:text-white hover:underline"
                >
                  <svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"
                    />
                  </svg>
                  {{ t('remoteAccess.getToken') }}
                </a>
              </div>

              <div v-if="selectedProvider === 'ngrok'" class="space-y-2">
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('remoteAccess.ngrokDomain') }}
                  <span class="text-gray-400 text-xs ml-1">({{ t('common.optional') }})</span>
                </label>
                <input
                  v-model="ngrokDomain"
                  type="text"
                  class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400 focus:border-transparent"
                  :placeholder="t('remoteAccess.ngrokDomainPlaceholder')"
                />
                <p class="text-xs text-gray-500 dark:text-gray-400">
                  {{ t('remoteAccess.ngrokDomainHint') }}
                  <a
                    href="https://dashboard.ngrok.com/domains"
                    target="_blank"
                    class="text-gray-900 dark:text-white hover:underline"
                  >
                    {{ t('remoteAccess.ngrokClaimDomain') }}
                  </a>
                </p>
              </div>

              <button
                class="w-full px-4 py-3 bg-gray-800 dark:bg-gray-500 hover:bg-gray-900 dark:hover:bg-gray-400 text-white rounded-lg font-medium transition-colors flex items-center justify-center gap-2"
                :disabled="selectedProviderInfo?.requires_key && !currentProviderToken"
                :class="{
                  'opacity-50 cursor-not-allowed':
                    selectedProviderInfo?.requires_key && !currentProviderToken,
                }"
                @click="handleRemoteAccessStart"
              >
                <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M13 10V3L4 14h7v7l9-11h-7z"
                  />
                </svg>
                {{ t('remoteAccess.enable') }}
              </button>
              <div class="text-xs text-gray-500 dark:text-gray-400 text-center">
                {{ t('remoteAccess.securityWarning') }}
              </div>
            </div>

            <div v-else-if="remoteAccessState === 'connecting'" class="space-y-4">
              <div class="flex items-center justify-center py-4">
                <div class="text-center">
                  <svg
                    class="animate-spin h-8 w-8 text-gray-900 dark:text-white mx-auto mb-4"
                    fill="none"
                    viewBox="0 0 24 24"
                  >
                    <circle
                      class="opacity-25"
                      cx="12"
                      cy="12"
                      r="10"
                      stroke="currentColor"
                      stroke-width="4"
                    />
                    <path
                      class="opacity-75"
                      fill="currentColor"
                      d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                    />
                  </svg>
                  <p class="text-gray-600 dark:text-gray-400">{{ t('remoteAccess.connecting') }}</p>
                </div>
              </div>
              <TunnelStatus
                :status="tunnelStatus || { active: false, connecting: true }"
                @disconnect="handleRemoteAccessStop"
              />
            </div>

            <div v-else-if="remoteAccessState === 'connected' && tunnelStatus" class="space-y-4">
              <TunnelStatus :status="tunnelStatus" @disconnect="handleRemoteAccessStop" />
            </div>

            <div v-else-if="remoteAccessState === 'error'" class="space-y-4">
              <div class="bg-red-50 dark:bg-red-900/20 rounded-lg p-4">
                <div class="flex items-start gap-3">
                  <svg
                    class="h-5 w-5 text-red-600 dark:text-red-400 mt-0.5"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                    />
                  </svg>
                  <div>
                    <p class="text-sm text-red-800 dark:text-red-200">{{ remoteAccessError }}</p>
                  </div>
                </div>
              </div>
              <button
                class="w-full px-4 py-3 bg-gray-600 hover:bg-gray-700 text-white rounded-lg font-medium transition-colors"
                @click="loadRemoteAccessStatus"
              >
                {{ t('common.retry') }}
              </button>
              <TunnelStatus
                :status="tunnelStatus || { active: false }"
                @disconnect="handleRemoteAccessStop"
              />
            </div>
          </div>
        </div>

        <div class="channels-board__stack">
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
              saving === channel.id,
              testingConnection === channel.id,
              testResult,
              ...channel.fields.map((f) => f.value),
            ]"
            :channel="channel"
            :expanded="expandedChannel === channel.id"
            :toggling="toggling === channel.id"
            :saving="saving === channel.id"
            :testing-connection="testingConnection === channel.id"
            :test-result="testResult"
            @toggle="toggleChannel(channel.id)"
            @toggle-enabled="toggleChannelEnabled(channel.id, $event)"
            @save="saveChannel(channel.id)"
            @test-connection="testConnection(channel.id)"
            @update-field="
              (fieldIndex: number, value: string) => updateChannelField(channel.id, fieldIndex, value)
            "
          />
        </div>

        <button
          v-if="!showMoreChannels && secondaryChannels.length > 0"
          class="channels-load-more"
          @click="showMoreChannels = true"
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M19 9l-7 7-7-7"
            />
          </svg>
          {{ t('common.loadMore') }} ({{ secondaryChannels.length }})
        </button>

        <div v-if="showMoreChannels" class="channels-board__stack channels-board__stack--secondary">
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
              saving === channel.id,
              testingConnection === channel.id,
              testResult,
              ...channel.fields.map((f) => f.value),
            ]"
            :channel="channel"
            :expanded="expandedChannel === channel.id"
            :toggling="toggling === channel.id"
            :saving="saving === channel.id"
            :testing-connection="testingConnection === channel.id"
            :test-result="testResult"
            @toggle="toggleChannel(channel.id)"
            @toggle-enabled="toggleChannelEnabled(channel.id, $event)"
            @save="saveChannel(channel.id)"
            @test-connection="testConnection(channel.id)"
            @update-field="
              (fieldIndex: number, value: string) => updateChannelField(channel.id, fieldIndex, value)
            "
          />
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.channels-page {
  --config-page-accent: 8, 145, 178;
  max-width: 1480px;
  margin: 0 auto;
  padding: 0 0.75rem 1.8rem;
}

.channels-stage {
  position: relative;
  padding: 1.15rem 0 0.25rem;
}

.channels-stage::before,
.channels-stage::after {
  content: none;
  position: absolute;
  width: 18rem;
  height: 18rem;
  pointer-events: none;
  opacity: 0.78;
  background-image: radial-gradient(circle, rgba(37, 99, 235, 0.22) 1px, transparent 1px);
  background-size: 14px 14px;
  z-index: 0;
}

.channels-stage::before {
  right: 10%;
  top: 7.5rem;
}

.channels-stage::after {
  left: 12%;
  bottom: -1.25rem;
}

.channels-header {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 0.9rem;
  padding: 1.35rem 1.45rem;
  margin-bottom: 0.88rem;
}

.channels-header__copy {
  max-width: 46rem;
}

.channels-summary-grid {
  position: relative;
  z-index: 1;
  display: grid;
  gap: 0.88rem;
  grid-template-columns: repeat(1, minmax(0, 1fr));
  margin-bottom: 0.88rem;
}

.channels-summary-card {
  position: relative;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  gap: 0.72rem;
  min-height: 8.9rem;
  padding: 1rem 1.05rem;
}

.channels-summary-card::after {
  content: none;
}

.channels-page .dashboard-card-surface {
  border-color: rgba(226, 232, 240, 0.92);
  background: rgba(255, 255, 255, 0.98);
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
  font-size: 0.7rem;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.channels-summary-value {
  position: relative;
  z-index: 1;
  display: block;
  color: #111827;
  font-size: clamp(1.65rem, 1vw + 1rem, 2.2rem);
  line-height: 1.04;
  letter-spacing: -0.04em;
  font-weight: 700;
}

.channels-summary-footnote {
  position: relative;
  z-index: 1;
  margin: auto 0 0;
  color: #6b7280;
  font-size: 0.78rem;
  line-height: 1.45;
}

.channels-summary-icon-row {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: center;
  flex-wrap: nowrap;
  gap: clamp(0.25rem, 0.12rem + 0.3vw, 0.5rem);
}

.channels-summary-icon-pill {
  width: clamp(1.9rem, 1.55rem + 0.75vw, 2.35rem);
  height: clamp(1.9rem, 1.55rem + 0.75vw, 2.35rem);
  flex: 0 0 auto;
  display: grid;
  place-items: center;
  border-radius: clamp(0.78rem, 0.62rem + 0.35vw, 0.95rem);
  background: rgba(255, 255, 255, 0.9);
  border: 1px solid rgba(203, 213, 225, 0.9);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.9);
}

.channels-summary-icon-pill img {
  width: clamp(1rem, 0.88rem + 0.24vw, 1.15rem);
  height: clamp(1rem, 0.88rem + 0.24vw, 1.15rem);
}

.channels-summary-icon-pill--count {
  width: auto;
  min-width: clamp(2.25rem, 1.95rem + 0.48vw, 2.55rem);
  padding: 0 0.5rem;
  font-size: 0.82rem;
  font-weight: 700;
  color: #475569;
}

.channels-board {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 1rem;
  padding: 0;
  border-radius: 0;
  background: transparent;
  border: 0;
}

.channels-board::before {
  content: none;
}

.channels-board__stack {
  position: relative;
  z-index: 1;
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.channels-board__stack--secondary {
  padding-top: 0.3rem;
}

.channels-load-more {
  width: 100%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.6rem;
  padding: 0.9rem 1rem;
  border-radius: 1rem;
  border: 1px solid rgba(203, 213, 225, 0.96);
  background: rgba(255, 255, 255, 0.96);
  color: #475569;
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
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 768px) {
  .channels-stage {
    padding-top: 0.7rem;
  }
}

:global(.dark) .channels-summary-label,
:global(.dark) .channels-summary-footnote {
  color: #cbd5e1;
}

:global(.dark) .channels-stage::before,
:global(.dark) .channels-stage::after {
  opacity: 0.46;
  background-image: radial-gradient(circle, rgba(96, 165, 250, 0.28) 1px, transparent 1px);
}

:global(.dark) .channels-page .dashboard-card-surface {
  background: rgba(30, 41, 59, 0.96);
  border-color: rgba(100, 116, 139, 0.56);
}

:global(.dark) .channels-summary-value,
:global(.dark) .channels-summary-icon-pill--count {
  color: #f8fafc;
}

:global(.dark) .channels-summary-icon-pill {
  background: rgba(30, 41, 59, 0.92);
  border-color: rgba(100, 116, 139, 0.62);
}

:global(.dark) .channels-board {
  background: transparent;
  border-color: transparent;
}

:global(.dark) .channels-board::before {
  content: none;
}

:global(.dark) .channels-load-more {
  background: rgba(30, 41, 59, 0.96);
  border-color: rgba(100, 116, 139, 0.58);
  color: #cbd5e1;
}
</style>
