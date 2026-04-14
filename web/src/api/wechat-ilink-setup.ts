import api from './client'

export interface WeChatILinkSetupSessionResponse {
  session_id: string
  status: string
  qrcode?: string
  mobile_url?: string
  expires_at: string
  error?: string
  message?: string
}

export function createWeChatILinkSetupSession() {
  return api.post<WeChatILinkSetupSessionResponse>('/channels/wechat_ilink/setup/session')
}

export function getWeChatILinkSetupSession(sessionId: string) {
  return api.get<WeChatILinkSetupSessionResponse>(`/channels/wechat_ilink/setup/session/${sessionId}`)
}

export function completeWeChatILinkSetupSession(sessionId: string, pairingPayload: unknown) {
  return api.post<WeChatILinkSetupSessionResponse>(
    `/channels/wechat_ilink/setup/session/${sessionId}/complete`,
    { pairing_payload: pairingPayload }
  )
}
