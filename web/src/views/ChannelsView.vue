<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { getChannelIconOrDefault } from '@/utils/channelIcons'

const { t } = useI18n()

interface ChannelConfig {
  id: string
  name: string
  icon: string
  enabled: boolean
  status: 'connected' | 'disconnected' | 'error' | 'connecting'
  description: string
  fields: ChannelField[]
  hint?: string
}

interface ChannelField {
  key: string
  label: string
  type: 'text' | 'password' | 'tel' | 'url' | 'textarea'
  placeholder: string
  value: string
  required?: boolean
}

const loading = ref(false)
const saving = ref<string | null>(null)
const testingConnection = ref<string | null>(null)
const testResult = ref<{ channelId: string; success: boolean; message: string } | null>(null)

const channels = ref<ChannelConfig[]>([
  {
    id: 'telegram',
    name: 'Telegram',
    icon: getChannelIconOrDefault('telegram'),
    enabled: false,
    status: 'disconnected',
    description: t('channels.telegramDesc'),
    hint: t('channels.telegramHint'),
    fields: [
      { key: 'bot_token', label: t('setup.botToken'), type: 'password', placeholder: '123456789:ABCdefGHIjklMNOpqrsTUVwxyz', value: '', required: true },
    ],
  },
  {
    id: 'discord',
    name: 'Discord',
    icon: getChannelIconOrDefault('discord'),
    enabled: false,
    status: 'disconnected',
    description: t('channels.discordDesc'),
    hint: t('channels.discordHint'),
    fields: [
      { key: 'bot_token', label: t('setup.botToken'), type: 'password', placeholder: 'Enter your Discord bot token', value: '', required: true },
      { key: 'application_id', label: t('channels.applicationId'), type: 'text', placeholder: 'Application ID', value: '' },
    ],
  },
  {
    id: 'slack',
    name: 'Slack',
    icon: getChannelIconOrDefault('slack'),
    enabled: false,
    status: 'disconnected',
    description: t('channels.slackDesc'),
    hint: t('setup.slackHint'),
    fields: [
      { key: 'bot_token', label: t('setup.slackBotToken'), type: 'password', placeholder: 'xoxb-xxxx-xxxx-xxxx', value: '', required: true },
      { key: 'app_token', label: t('setup.slackAppToken'), type: 'password', placeholder: 'xapp-xxxx-xxxx-xxxx', value: '', required: true },
      { key: 'signing_secret', label: t('channels.signingSecret'), type: 'password', placeholder: 'Signing secret', value: '' },
    ],
  },
  {
    id: 'whatsapp',
    name: 'WhatsApp',
    icon: getChannelIconOrDefault('whatsapp'),
    enabled: false,
    status: 'disconnected',
    description: t('channels.whatsappDesc'),
    hint: t('setup.whatsappHint'),
    fields: [
      { key: 'phone_number', label: t('setup.phoneNumber'), type: 'tel', placeholder: '+1234567890', value: '', required: true },
    ],
  },
  {
    id: 'signal',
    name: 'Signal',
    icon: getChannelIconOrDefault('signal'),
    enabled: false,
    status: 'disconnected',
    description: t('channels.signalDesc'),
    hint: t('setup.signalHint'),
    fields: [
      { key: 'phone_number', label: t('setup.phoneNumber'), type: 'tel', placeholder: '+1234567890', value: '', required: true },
    ],
  },
  {
    id: 'teams',
    name: 'Microsoft Teams',
    icon: getChannelIconOrDefault('teams'),
    enabled: false,
    status: 'disconnected',
    description: t('channels.teamsDesc'),
    hint: t('setup.teamsHint'),
    fields: [
      { key: 'app_id', label: t('setup.appId'), type: 'text', placeholder: 'xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx', value: '', required: true },
      { key: 'app_password', label: t('setup.appPassword'), type: 'password', placeholder: 'App password', value: '', required: true },
    ],
  },
  {
    id: 'googlechat',
    name: 'Google Chat',
    icon: getChannelIconOrDefault('google'),
    enabled: false,
    status: 'disconnected',
    description: t('channels.googleChatDesc'),
    hint: t('setup.googleChatHint'),
    fields: [
      { key: 'credentials_json', label: t('setup.serviceAccountJson'), type: 'textarea', placeholder: '{"type": "service_account", ...}', value: '', required: true },
    ],
  },
  {
    id: 'feishu',
    name: t('setup.feishuBot'),
    icon: getChannelIconOrDefault('feishu'),
    enabled: false,
    status: 'disconnected',
    description: t('channels.feishuDesc'),
    fields: [
      { key: 'app_id', label: t('setup.appId'), type: 'text', placeholder: 'cli_xxxxxxxxxx', value: '', required: true },
      { key: 'app_secret', label: t('setup.appSecret'), type: 'password', placeholder: 'App secret', value: '', required: true },
    ],
  },
  {
    id: 'wechat',
    name: t('setup.wechatWorkBot'),
    icon: getChannelIconOrDefault('wechat'),
    enabled: false,
    status: 'disconnected',
    description: t('channels.wechatDesc'),
    fields: [
      { key: 'corp_id', label: t('setup.corpId'), type: 'text', placeholder: 'ww1234567890abcdef', value: '', required: true },
      { key: 'agent_id', label: t('setup.agentId'), type: 'text', placeholder: '1000001', value: '', required: true },
      { key: 'secret', label: t('setup.secret'), type: 'password', placeholder: 'Secret', value: '', required: true },
    ],
  },
  {
    id: 'matrix',
    name: 'Matrix',
    icon: getChannelIconOrDefault('matrix'),
    enabled: false,
    status: 'disconnected',
    description: t('channels.matrixDesc'),
    fields: [
      { key: 'homeserver', label: t('setup.matrixHomeserver'), type: 'url', placeholder: 'https://matrix.org', value: '', required: true },
      { key: 'user_id', label: t('setup.matrixUserId'), type: 'text', placeholder: '@bot:matrix.org', value: '', required: true },
      { key: 'access_token', label: t('setup.accessToken'), type: 'password', placeholder: 'Access token', value: '', required: true },
    ],
  },
  {
    id: 'imessage',
    name: 'iMessage',
    icon: getChannelIconOrDefault('imessage'),
    enabled: false,
    status: 'disconnected',
    description: t('channels.imessageDesc'),
    hint: t('setup.imessageHint'),
    fields: [],
  },
])

const expandedChannel = ref<string | null>(null)

const enabledCount = computed(() => channels.value.filter(c => c.enabled).length)
const connectedCount = computed(() => channels.value.filter(c => c.status === 'connected').length)

function toggleChannel(channelId: string) {
  expandedChannel.value = expandedChannel.value === channelId ? null : channelId
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
        const localChannel = channels.value.find(c => c.id === serverChannel.id)
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

async function saveChannel(channel: ChannelConfig) {
  saving.value = channel.id
  try {
    const config: Record<string, string> = {}
    for (const field of channel.fields) {
      config[field.key] = field.value
    }

    const response = await fetch(`/api/channels/${channel.id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        enabled: channel.enabled,
        config,
      }),
    })

    if (response.ok) {
      testResult.value = { channelId: channel.id, success: true, message: t('channels.savedSuccessfully') }
    } else {
      const data = await response.json()
      testResult.value = { channelId: channel.id, success: false, message: data.message || t('channels.saveFailed') }
    }
  } catch (error) {
    testResult.value = { channelId: channel.id, success: false, message: t('channels.saveFailed') }
  } finally {
    saving.value = null
    setTimeout(() => {
      if (testResult.value?.channelId === channel.id) {
        testResult.value = null
      }
    }, 3000)
  }
}

async function testConnection(channel: ChannelConfig) {
  testingConnection.value = channel.id
  testResult.value = null

  try {
    const config: Record<string, string> = {}
    for (const field of channel.fields) {
      config[field.key] = field.value
    }

    const response = await fetch(`/api/channels/${channel.id}/test`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ config }),
    })

    const data = await response.json()
    testResult.value = { channelId: channel.id, success: data.success, message: data.message }
  } catch (error) {
    testResult.value = { channelId: channel.id, success: false, message: t('channels.testFailed') }
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
                v-model="channel.enabled"
                type="checkbox"
                class="sr-only peer"
              />
              <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-2 peer-focus:ring-blue-300 dark:peer-focus:ring-blue-800 rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-blue-600"></div>
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
            <div v-for="field in channel.fields" :key="field.key">
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                {{ field.label }}
                <span v-if="field.required" class="text-red-500">*</span>
              </label>
              <textarea
                v-if="field.type === 'textarea'"
                v-model="field.value"
                :placeholder="field.placeholder"
                rows="4"
                class="w-full bg-white dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 border border-gray-300 dark:border-gray-600 focus:outline-none focus:ring-2 focus:ring-blue-500 font-mono text-sm"
              />
              <input
                v-else
                v-model="field.value"
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
          <div class="flex gap-3 mt-4">
            <button
              v-if="channel.fields.length > 0"
              :disabled="testingConnection === channel.id"
              class="px-4 py-2 bg-gray-200 dark:bg-gray-600 hover:bg-gray-300 dark:hover:bg-gray-500 text-gray-900 dark:text-white rounded-lg transition-colors disabled:opacity-50 text-sm"
              @click="testConnection(channel)"
            >
              {{ testingConnection === channel.id ? t('setup.testing') : t('setup.testConnection') }}
            </button>
            <button
              :disabled="saving === channel.id"
              class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors disabled:opacity-50 text-sm"
              @click="saveChannel(channel)"
            >
              {{ saving === channel.id ? t('channels.saving') : t('common.save') }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
