import api from './client'
import type { AxiosRequestConfig, AxiosResponse } from 'axios'

function request<T>(config: AxiosRequestConfig): Promise<AxiosResponse<T>> {
  return api.request<T>({
    baseURL: '/api',
    validateStatus: () => true,
    ...config,
  })
}

export const channelsApi = {
  list() {
    return request<{ channels?: any[]; message?: string }>({
      url: '/channels',
      method: 'GET',
    })
  },

  getSettings() {
    return request<any>({
      url: '/channels/settings',
      method: 'GET',
    })
  },

  updateSettings(payload: unknown) {
    return request<any>({
      url: '/channels/settings',
      method: 'PUT',
      data: payload,
    })
  },

  updateChannel(channelId: string, payload: unknown) {
    return request<any>({
      url: `/channels/${channelId}`,
      method: 'PUT',
      data: payload,
    })
  },

  toggleChannel(channelId: string, payload: unknown) {
    return request<any>({
      url: `/channels/${channelId}/toggle`,
      method: 'POST',
      data: payload,
    })
  },

  getChannelStatus(channelId: string) {
    return request<any>({
      url: `/channels/${channelId}/status`,
      method: 'GET',
    })
  },

  testConnection(payload: unknown) {
    return request<any>({
      url: '/setup/test-connection',
      method: 'POST',
      data: payload,
    })
  },
}

export default channelsApi
