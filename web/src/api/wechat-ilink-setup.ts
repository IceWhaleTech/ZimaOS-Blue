import api from './client'

export interface WeChatILinkSetupSessionResponse {
  session_id: string
  status: string
  scan_url?: string
  mobile_url?: string
  expires_at: string
  error?: string
  message?: string
}

const channelAPIConfig = {
  baseURL: '/api',
  validateStatus: () => true,
}

export function createWeChatILinkSetupSession() {
  return api.post<WeChatILinkSetupSessionResponse>(
    '/channels/wechat_ilink/setup/session',
    undefined,
    channelAPIConfig
  )
}

export function getWeChatILinkSetupSession(sessionId: string) {
  return api.get<WeChatILinkSetupSessionResponse>(
    `/channels/wechat_ilink/setup/session/${sessionId}`,
    channelAPIConfig
  )
}

export function completeWeChatILinkSetupSession(sessionId: string, pairingPayload: unknown) {
  return api.post<WeChatILinkSetupSessionResponse>(
    `/channels/wechat_ilink/setup/session/${sessionId}/complete`,
    { pairing_payload: pairingPayload },
    channelAPIConfig
  )
}
