<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { getChannelIconOrDefault } from '@/utils/channelIcons'
import PasswordInput from '@/components/ui/PasswordInput.vue'

const { t } = useI18n()

interface ChannelFieldDef {
  key: string
  labelKey: string
  type: 'text' | 'password' | 'tel' | 'url' | 'textarea'
  placeholder: string
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
  descriptionKey: string
  hintKey?: string
  docUrl?: string
  fields: ChannelFieldDef[]
}

const loading = ref(false)
const saving = ref<string | null>(null)
const toggling = ref<string | null>(null)
const testingConnection = ref<string | null>(null)
const testResult = ref<{ channelId: string; success: boolean; message: string } | null>(null)

const channelDefs = ref<ChannelDef[]>([
  {
    id: 'telegram',
    name: 'Telegram',
    icon: getChannelIconOrDefault('telegram'),
    enabled: false,
    status: 'disconnected',
    descriptionKey: 'channels.telegramDesc',
    hintKey: 'channels.telegramHint',
    docUrl: 'https://core.telegram.org/bots#how-do-i-create-a-bot',
    fields: [
      { key: 'bot_token', labelKey: 'setup.botToken', type: 'password', placeholder: '123456789:ABCdefGHIjklMNOpqrsTUVwxyz', value: '', required: true },
      { key: 'bot_username', labelKey: 'channels.botUsername', type: 'text', placeholder: 'my_bot', value: '' },
    ],
  },
  {
    id: 'discord',
    name: 'Discord',
    icon: getChannelIconOrDefault('discord'),
    enabled: false,
    status: 'disconnected',
    descriptionKey: 'channels.discordDesc',
    hintKey: 'channels.discordHint',
    docUrl: 'https://discord.com/developers/docs/getting-started',
    fields: [
      { key: 'bot_token', labelKey: 'setup.botToken', type: 'password', placeholder: 'Enter your Discord bot token', value: '', required: true },
      { key: 'application_id', labelKey: 'channels.applicationId', type: 'text', placeholder: 'Application ID', value: '' },
    ],
  },
  {
    id: 'slack',
    name: 'Slack',
    icon: getChannelIconOrDefault('slack'),
    enabled: false,
    status: 'disconnected',
    descriptionKey: 'channels.slackDesc',
    hintKey: 'setup.slackHint',
    docUrl: 'https://api.slack.com/start/quickstart',
    fields: [
      { key: 'bot_token', labelKey: 'setup.slackBotToken', type: 'password', placeholder: 'xoxb-xxxx-xxxx-xxxx', value: '', required: true },
      { key: 'app_token', labelKey: 'setup.slackAppToken', type: 'password', placeholder: 'xapp-xxxx-xxxx-xxxx', value: '', required: true },
      { key: 'signing_secret', labelKey: 'channels.signingSecret', type: 'password', placeholder: 'Signing secret', value: '' },
    ],
  },
  {
    id: 'whatsapp',
    name: 'WhatsApp',
    icon: getChannelIconOrDefault('whatsapp'),
    enabled: false,
    status: 'disconnected',
    descriptionKey: 'channels.whatsappDesc',
    hintKey: 'setup.whatsappHint',
    docUrl: 'https://developers.facebook.com/docs/whatsapp/cloud-api/get-started',
    fields: [
      { key: 'phone_number', labelKey: 'setup.phoneNumber', type: 'tel', placeholder: '+1234567890', value: '', required: true },
    ],
  },
  {
    id: 'signal',
    name: 'Signal',
    icon: getChannelIconOrDefault('signal'),
    enabled: false,
    status: 'disconnected',
    descriptionKey: 'channels.signalDesc',
    hintKey: 'setup.signalHint',
    docUrl: 'https://github.com/AsamK/signal-cli',
    fields: [
      { key: 'phone_number', labelKey: 'setup.phoneNumber', type: 'tel', placeholder: '+1234567890', value: '', required: true },
    ],
  },
  {
    id: 'teams',
    name: 'Microsoft Teams',
    icon: getChannelIconOrDefault('teams'),
    enabled: false,
    status: 'disconnected',
    descriptionKey: 'channels.teamsDesc',
    hintKey: 'setup.teamsHint',
    docUrl: 'https://learn.microsoft.com/en-us/microsoftteams/platform/bots/how-to/create-a-bot-for-teams',
    fields: [
      { key: 'app_id', labelKey: 'setup.appId', type: 'text', placeholder: 'xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx', value: '', required: true },
      { key: 'app_password', labelKey: 'setup.appPassword', type: 'password', placeholder: 'App password', value: '', required: true },
    ],
  },
  {
    id: 'googlechat',
    name: 'Google Chat',
    icon: getChannelIconOrDefault('google'),
    enabled: false,
    status: 'disconnected',
    descriptionKey: 'channels.googleChatDesc',
    hintKey: 'setup.googleChatHint',
    docUrl: 'https://developers.google.com/workspace/chat/quickstart/gcf-app',
    fields: [
      { key: 'credentials_json', labelKey: 'setup.serviceAccountJson', type: 'textarea', placeholder: '{"type": "service_account", ...}', value: '', required: true },
    ],
  },
  {
    id: 'feishu',
    nameKey: 'setup.feishuBot',
    icon: getChannelIconOrDefault('feishu'),
    enabled: false,
    status: 'disconnected',
    descriptionKey: 'channels.feishuDesc',
    hintKey: 'channels.feishuHint',
    docUrl: 'https://open.feishu.cn/document/develop-an-echo-bot/introduction',
    fields: [
      { key: 'app_id', labelKey: 'setup.appId', type: 'text', placeholder: 'cli_xxxxxxxxxx', value: '', required: true },
      { key: 'app_secret', labelKey: 'setup.appSecret', type: 'password', placeholder: 'App secret', value: '', required: true },
      { key: 'verification_token', labelKey: 'channels.verificationToken', type: 'password', placeholder: 'Verification token', value: '', required: true },
      { key: 'encrypt_key', labelKey: 'channels.encryptKey', type: 'password', placeholder: '', value: '' },
      { key: 'webhook_url', labelKey: 'channels.webhookUrl', type: 'url', placeholder: 'https://your-server.com/api/v1/channels/feishu/callback', value: '' },
    ],
  },
  {
    id: 'dingtalk',
    nameKey: 'setup.dingtalkBot',
    icon: getChannelIconOrDefault('dingtalk'),
    enabled: false,
    status: 'disconnected',
    descriptionKey: 'channels.dingtalkDesc',
    hintKey: 'channels.dingtalkHint',
    docUrl: 'https://open.dingtalk.com/document/orgapp/create-an-enterprise-chatbot',
    fields: [
      { key: 'app_key', labelKey: 'channels.appKey', type: 'text', placeholder: 'dingxxxxxxxxxx', value: '', required: true },
      { key: 'app_secret', labelKey: 'setup.appSecret', type: 'password', placeholder: 'App secret', value: '', required: true },
      { key: 'robot_code', labelKey: 'channels.robotCode', type: 'text', placeholder: 'dingxxxxxxxxxx', value: '', required: true },
    ],
  },
  {
    id: 'qq',
    name: 'QQ Bot',
    icon: getChannelIconOrDefault('qq'),
    enabled: false,
    status: 'disconnected',
    descriptionKey: 'channels.qqDesc',
    hintKey: 'channels.qqHint',
    docUrl: 'https://q.qq.com/wiki/develop/api-231017/dev-prepare/interface-framework/api-use.html',
    fields: [
      { key: 'app_id', labelKey: 'setup.appId', type: 'text', placeholder: '102xxxxxx', value: '', required: true },
      { key: 'app_secret', labelKey: 'setup.appSecret', type: 'password', placeholder: 'App secret', value: '', required: true },
      { key: 'token', labelKey: 'setup.botToken', type: 'password', placeholder: 'Bot token', value: '', required: true },
    ],
  },
  {
    id: 'wechat',
    nameKey: 'setup.wechatWorkBot',
    icon: getChannelIconOrDefault('wechat'),
    enabled: false,
    status: 'disconnected',
    descriptionKey: 'channels.wechatDesc',
    hintKey: 'channels.wechatHint',
    docUrl: 'https://developer.work.weixin.qq.com/document/path/90664',
    fields: [
      { key: 'corp_id', labelKey: 'setup.corpId', type: 'text', placeholder: 'ww1234567890abcdef', value: '', required: true },
      { key: 'agent_id', labelKey: 'setup.agentId', type: 'text', placeholder: '1000001', value: '', required: true },
      { key: 'secret', labelKey: 'setup.secret', type: 'password', placeholder: 'Secret', value: '', required: true },
    ],
  },
  {
    id: 'matrix',
    name: 'Matrix',
    icon: getChannelIconOrDefault('matrix'),
    enabled: false,
    status: 'disconnected',
    descriptionKey: 'channels.matrixDesc',
    hintKey: 'channels.matrixHint',
    docUrl: 'https://spec.matrix.org/latest/client-server-api/',
    fields: [
      { key: 'homeserver', labelKey: 'setup.matrixHomeserver', type: 'url', placeholder: 'https://matrix.org', value: '', required: true },
      { key: 'user_id', labelKey: 'setup.matrixUserId', type: 'text', placeholder: '@bot:matrix.org', value: '', required: true },
      { key: 'access_token', labelKey: 'setup.accessToken', type: 'password', placeholder: 'Access token', value: '', required: true },
    ],
  },
  {
    id: 'imessage',
    name: 'iMessage',
    icon: getChannelIconOrDefault('imessage'),
    enabled: false,
    status: 'disconnected',
    descriptionKey: 'channels.imessageDesc',
    hintKey: 'setup.imessageHint',
    docUrl: 'https://github.com/mautrix/imessage',
    fields: [],
  },
])

// Computed channels with translated strings
const channels = computed(() => channelDefs.value.map(ch => ({
  ...ch,
  name: ch.nameKey ? t(ch.nameKey) : ch.name!,
  description: t(ch.descriptionKey),
  hint: ch.hintKey ? t(ch.hintKey) : undefined,
  fields: ch.fields.map(f => ({
    ...f,
    label: t(f.labelKey),
    placeholder: f.key === 'encrypt_key' ? t('channels.encryptKeyPlaceholder') : f.placeholder,
  })),
})))

const expandedChannel = ref<string | null>(null)

const enabledCount = computed(() => channels.value.filter(c => c.enabled).length)
const connectedCount = computed(() => channels.value.filter(c => c.status === 'connected').length)

function toggleChannel(channelId: string) {
  expandedChannel.value = expandedChannel.value === channelId ? null : channelId
}

async function toggleChannelEnabled(channelId: string, enabled: boolean) {
  toggling.value = channelId
  const channelDef = channelDefs.value.find(c => c.id === channelId)
  if (!channelDef) return

  // Check if required fields are filled when enabling
  if (enabled) {
    const missingFields = channelDef.fields.filter(f => f.required && !f.value)
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
  // Set status to connecting when enabling, disconnected when disabling
  if (enabled) {
    channelDef.status = 'connecting'
  } else {
    channelDef.status = 'disconnected'
  }

  try {
    const response = await fetch(`/api/channels/${channelId}/toggle`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enabled }),
    })

    if (response.ok) {
      const data = await response.json()
      // Update status from server response
      channelDef.status = data.status || (enabled ? 'connected' : 'disconnected')
    } else {
      // Revert on error
      channelDef.enabled = !enabled
      channelDef.status = 'error'
      const data = await response.json()
      testResult.value = { channelId, success: false, message: data.message || t('channels.toggleFailed') }
    }
  } catch (error) {
    // Revert on error
    channelDef.enabled = !enabled
    channelDef.status = 'error'
    testResult.value = { channelId, success: false, message: t('channels.toggleFailed') }
  } finally {
    toggling.value = null
  }
}

function getStatusColor(status: string): string {
  switch (status) {
    case 'connected': return 'bg-green-500'
    case 'connecting': return 'bg-yellow-500 animate-pulse'
    case 'error': return 'bg-red-500'
    default: return 'bg-gray-400'
  }
}

function getStatusText(status: string): string {
  switch (status) {
    case 'connected': return t('channels.statusConnected')
    case 'connecting': return t('channels.statusConnecting')
    case 'error': return t('channels.statusError')
    default: return t('channels.statusDisconnected')
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
        const localChannel = channelDefs.value.find(c => c.id === serverChannel.id)
        if (localChannel) {
          localChannel.enabled = serverChannel.enabled
          localChannel.status = serverChannel.status
          // Update field values
          for (const field of localChannel.fields) {
            if (serverChannel.config && serverChannel.config[field.key]) {
              field.value = serverChannel.config[field.key]
            }
          }
        }
      }
    }
  } catch (error) {
    console.error('Failed to load channel configs:', error)
  } finally {
    loading.value = false
  }
}

async function saveChannel(channelId: string) {
  saving.value = channelId
  const channelDef = channelDefs.value.find(c => c.id === channelId)
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

    if (response.ok) {
      testResult.value = { channelId, success: true, message: t('channels.savedSuccessfully') }
    } else {
      const data = await response.json()
      testResult.value = { channelId, success: false, message: data.message || t('channels.saveFailed') }
    }
  } catch (error) {
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

async function testConnection(channelId: string) {
  testingConnection.value = channelId
  testResult.value = null
  const channelDef = channelDefs.value.find(c => c.id === channelId)
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
    testResult.value = { channelId, success: data.success, message: data.message }

    // Update status based on test result
    if (data.success) {
      // If enabled, set to connected; otherwise keep disconnected but mark as valid config
      if (channelDef.enabled) {
        channelDef.status = 'connected'
      }
    } else {
      // Test failed - if enabled, show error
      if (channelDef.enabled) {
        channelDef.status = 'error'
      }
    }
  } catch (error) {
    testResult.value = { channelId, success: false, message: t('channels.testFailed') }
    if (channelDef.enabled) {
      channelDef.status = 'error'
    }
  } finally {
    testingConnection.value = null
  }
}

onMounted(() => {
  loadChannelConfigs()
})
</script>

<template>
  <div class="channels-view p-4 sm:p-6 max-w-4xl mx-auto">
    <div class="mb-6">
      <h1 class="text-xl sm:text-2xl font-bold text-gray-900 dark:text-white">{{ t('channels.title') }}</h1>
      <p class="text-gray-500 dark:text-gray-400 text-sm mt-1">{{ t('channels.subtitle') }}</p>
    </div>

    <!-- Stats -->
    <div class="grid grid-cols-2 gap-4 mb-6">
      <div class="bg-white dark:bg-gray-800 rounded-lg p-4 border border-gray-200 dark:border-gray-700">
        <div class="text-2xl font-bold text-gray-900 dark:text-white">{{ enabledCount }}</div>
        <div class="text-sm text-gray-500 dark:text-gray-400">{{ t('channels.enabledChannels') }}</div>
      </div>
      <div class="bg-white dark:bg-gray-800 rounded-lg p-4 border border-gray-200 dark:border-gray-700">
        <div class="text-2xl font-bold text-green-600 dark:text-green-400">{{ connectedCount }}</div>
        <div class="text-sm text-gray-500 dark:text-gray-400">{{ t('channels.connectedChannels') }}</div>
      </div>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="text-center py-8">
      <div class="animate-spin w-8 h-8 border-2 border-blue-500 border-t-transparent rounded-full mx-auto mb-2"></div>
      <p class="text-gray-500 dark:text-gray-400">{{ t('common.loading') }}</p>
    </div>

    <!-- Channel List -->
    <div v-else class="space-y-3">
      <div
        v-for="channel in channels"
        :key="channel.id"
        class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden"
      >
        <!-- Channel Header -->
        <div
          class="flex items-center gap-4 p-4 cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors"
          @click="toggleChannel(channel.id)"
        >
          <img :src="channel.icon" :alt="channel.name" class="w-10 h-10 object-contain" />
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-2">
              <h3 class="font-medium text-gray-900 dark:text-white">{{ channel.name }}</h3>
              <span
                class="w-2 h-2 rounded-full"
                :class="getStatusColor(channel.status)"
                :title="getStatusText(channel.status)"
              ></span>
            </div>
            <p class="text-sm text-gray-500 dark:text-gray-400 truncate">{{ channel.description }}</p>
          </div>
          <div class="flex items-center gap-3">
            <label class="relative inline-flex items-center cursor-pointer" @click.stop>
              <input
                :checked="channelDefs.find(c => c.id === channel.id)!.enabled"
                type="checkbox"
                class="sr-only peer"
                :disabled="toggling === channel.id"
                @change="toggleChannelEnabled(channel.id, ($event.target as HTMLInputElement).checked)"
              />
              <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-2 peer-focus:ring-blue-300 dark:peer-focus:ring-blue-800 rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-blue-600 peer-disabled:opacity-50"></div>
            </label>
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-5 w-5 text-gray-400 transition-transform"
              :class="{ 'rotate-180': expandedChannel === channel.id }"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
            </svg>
          </div>
        </div>

        <!-- Channel Config (Expanded) -->
        <div
          v-if="expandedChannel === channel.id"
          class="border-t border-gray-200 dark:border-gray-700 p-4 bg-gray-50 dark:bg-gray-800/50"
        >
          <!-- Hint -->
          <p v-if="channel.hint" class="text-xs text-gray-500 dark:text-gray-400 mb-4 flex items-start gap-2">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 flex-shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            {{ channel.hint }}
          </p>

          <!-- Fields -->
          <div v-if="channel.fields.length > 0" class="space-y-4">
            <div v-for="(field, fieldIndex) in channel.fields" :key="field.key">
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                {{ field.label }}
                <span v-if="field.required" class="text-red-500">*</span>
              </label>
              <textarea
                v-if="field.type === 'textarea'"
                v-model="channelDefs.find(c => c.id === channel.id)!.fields[fieldIndex].value"
                :name="field.key"
                :placeholder="field.placeholder"
                rows="4"
                class="w-full bg-white dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 border border-gray-300 dark:border-gray-600 focus:outline-none focus:ring-2 focus:ring-blue-500 font-mono text-sm"
              />
              <PasswordInput
                v-else-if="field.type === 'password'"
                v-model="channelDefs.find(c => c.id === channel.id)!.fields[fieldIndex].value"
                :name="field.key"
                :placeholder="field.placeholder"
                class="w-full bg-white dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 border border-gray-300 dark:border-gray-600 focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
              <input
                v-else
                v-model="channelDefs.find(c => c.id === channel.id)!.fields[fieldIndex].value"
                :name="field.key"
                :type="field.type"
                :placeholder="field.placeholder"
                class="w-full bg-white dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 border border-gray-300 dark:border-gray-600 focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
          </div>

          <p v-else class="text-sm text-gray-500 dark:text-gray-400">
            {{ t('channels.noConfigRequired') }}
          </p>

          <!-- Test Result -->
          <div
            v-if="testResult && testResult.channelId === channel.id"
            class="mt-4 p-3 rounded-lg"
            :class="testResult.success ? 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400' : 'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400'"
          >
            {{ testResult.message }}
          </div>

          <!-- Actions -->
          <div class="flex flex-wrap items-center gap-3 mt-4">
            <button
              v-if="channel.fields.length > 0"
              :disabled="testingConnection === channel.id"
              class="px-4 py-2 bg-gray-200 dark:bg-gray-600 hover:bg-gray-300 dark:hover:bg-gray-500 text-gray-900 dark:text-white rounded-lg transition-colors disabled:opacity-50 text-sm"
              @click="testConnection(channel.id)"
            >
              {{ testingConnection === channel.id ? t('setup.testing') : t('setup.testConnection') }}
            </button>
            <button
              :disabled="saving === channel.id"
              class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors disabled:opacity-50 text-sm"
              @click="saveChannel(channel.id)"
            >
              {{ saving === channel.id ? t('channels.saving') : t('common.save') }}
            </button>
            <a
              v-if="channel.docUrl"
              :href="channel.docUrl"
              target="_blank"
              rel="noopener noreferrer"
              class="inline-flex items-center gap-1.5 px-3 py-2 text-sm text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 hover:underline"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253" />
              </svg>
              {{ t('channels.viewDocs') }}
            </a>
            <!-- Feishu Open Bot Chat Link -->
            <a
              v-if="channel.id === 'feishu' && channelDefs.find(c => c.id === 'feishu')?.fields.find(f => f.key === 'app_id')?.value"
              :href="`https://applink.feishu.cn/client/bot/open?appId=${channelDefs.find(c => c.id === 'feishu')?.fields.find(f => f.key === 'app_id')?.value}`"
              target="_blank"
              rel="noopener noreferrer"
              class="inline-flex items-center gap-1.5 px-3 py-2 text-sm text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 hover:underline"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
              </svg>
              {{ t('channels.feishuOpenChat') }}
            </a>
            <!-- Telegram Open Bot Chat Link -->
            <a
              v-if="channel.id === 'telegram' && channelDefs.find(c => c.id === 'telegram')?.fields.find(f => f.key === 'bot_username')?.value"
              :href="`https://t.me/${channelDefs.find(c => c.id === 'telegram')?.fields.find(f => f.key === 'bot_username')?.value}`"
              target="_blank"
              rel="noopener noreferrer"
              class="inline-flex items-center gap-1.5 px-3 py-2 text-sm text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 hover:underline"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
              </svg>
              {{ t('channels.telegramOpenChat') }}
            </a>
            <!-- DingTalk Open Bot Chat Link -->
            <a
              v-if="channel.id === 'dingtalk' && channelDefs.find(c => c.id === 'dingtalk')?.fields.find(f => f.key === 'robot_code')?.value"
              :href="`dingtalk://dingtalkclient/action/sendRobot?robotCode=${channelDefs.find(c => c.id === 'dingtalk')?.fields.find(f => f.key === 'robot_code')?.value}`"
              target="_blank"
              rel="noopener noreferrer"
              class="inline-flex items-center gap-1.5 px-3 py-2 text-sm text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 hover:underline"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
              </svg>
              {{ t('channels.dingtalkOpenChat') }}
            </a>
            <!-- WhatsApp Open Chat Link -->
            <a
              v-if="channel.id === 'whatsapp' && channelDefs.find(c => c.id === 'whatsapp')?.fields.find(f => f.key === 'phone_number')?.value"
              :href="`https://wa.me/${channelDefs.find(c => c.id === 'whatsapp')?.fields.find(f => f.key === 'phone_number')?.value?.replace(/[^0-9]/g, '')}`"
              target="_blank"
              rel="noopener noreferrer"
              class="inline-flex items-center gap-1.5 px-3 py-2 text-sm text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 hover:underline"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
              </svg>
              {{ t('channels.whatsappOpenChat') }}
            </a>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
