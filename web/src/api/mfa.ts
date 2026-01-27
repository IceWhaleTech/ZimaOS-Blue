import api from './client'

export interface MFAStatus {
  enabled: boolean
  recovery_codes_remaining: number
  setup_required?: boolean
}

export interface MFASetupResponse {
  secret: string
  uri: string
  qr_code?: string
}

export interface MFAVerifyResponse {
  enabled: boolean
  recovery_codes?: string[]
}

export interface RecoveryCodesResponse {
  codes: string[]
  remaining: number
}

export const mfaApi = {
  // Get MFA status
  getStatus: () => api.get<MFAStatus>('/auth/mfa/status'),

  // Start MFA setup
  setup: (includeQRCode = true) =>
    api.post<MFASetupResponse>('/auth/mfa/setup', { include_qr_code: includeQRCode }),

  // Verify MFA setup with code
  verify: (code: string, secret: string) =>
    api.post<MFAVerifyResponse>('/auth/mfa/verify', { code, secret }),

  // Disable MFA
  disable: (password: string, code?: string) =>
    api.post<{ enabled: boolean }>('/auth/mfa/disable', { password, code }),

  // Get recovery codes
  getRecoveryCodes: () => api.get<RecoveryCodesResponse>('/auth/mfa/recovery'),

  // Regenerate recovery codes
  regenerateRecoveryCodes: () =>
    api.post<RecoveryCodesResponse>('/auth/mfa/recovery/regenerate'),
}
