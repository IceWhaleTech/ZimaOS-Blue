import api from './client'
import type { AxiosRequestConfig, AxiosResponse } from 'axios'

export type ChannelConnectionStatus = 'connected' | 'disconnected' | 'error' | 'connecting'

export interface ChannelConfigRecord {
  id: string
  enabled: boolean
  status: ChannelConnectionStatus
  last_error?: string
  last_error_key?: string
  config?: Record<string, string>
  messages_received?: number
  messages_sent?: number
  last_message_at?: string
  last_reply_at?: string
}

export interface ChannelGroupAccessSettings {
  policy?: string
  mention_policy?: string
  allowed_chat_ids?: Record<string, string[]>
}

export interface ChannelSettingsRecord {
  group_access?: ChannelGroupAccessSettings
  [key: string]: unknown
}

export interface ChannelListResponse {
  channels?: ChannelConfigRecord[]
  message?: string
}

export interface ChannelMutationResponse {
  status?: ChannelConnectionStatus
  message?: string
  channel?: Partial<ChannelConfigRecord>
}

export interface ChannelStatusResponse {
  status: ChannelConnectionStatus
  last_error?: string
  last_error_key?: string
}

export interface ChannelSettingsResponse {
  settings?: ChannelSettingsRecord
  group_access?: ChannelGroupAccessSettings
  message?: string
}

export interface ChannelConnectionTestResponse {
  success: boolean
  message: string
}

function request<T>(config: AxiosRequestConfig): Promise<AxiosResponse<T>> {
  return api.request<T>({
    baseURL: '/api',
    validateStatus: () => true,
    ...config,
  })
}

export const channelsApi = {
  list() {
    return request<ChannelListResponse>({
      url: '/channels',
      method: 'GET',
    })
  },

  getSettings() {
    return request<ChannelSettingsResponse>({
      url: '/channels/settings',
      method: 'GET',
    })
  },

  updateSettings(payload: unknown) {
    return request<ChannelSettingsResponse>({
      url: '/channels/settings',
      method: 'PUT',
      data: payload,
    })
  },

  updateChannel(channelId: string, payload: unknown) {
    return request<ChannelMutationResponse>({
      url: `/channels/${channelId}`,
      method: 'PUT',
      data: payload,
    })
  },

  toggleChannel(channelId: string, payload: unknown) {
    return request<ChannelMutationResponse>({
      url: `/channels/${channelId}/toggle`,
      method: 'POST',
      data: payload,
    })
  },

  getChannelStatus(channelId: string) {
    return request<ChannelStatusResponse>({
      url: `/channels/${channelId}/status`,
      method: 'GET',
    })
  },

  testConnection(payload: unknown) {
    return request<ChannelConnectionTestResponse>({
      url: '/setup/test-connection',
      method: 'POST',
      data: payload,
    })
  },
}

export default channelsApi
