import api from './client'

export interface EncryptionStatus {
  enabled: boolean
  encrypted_count: number
  plaintext_count: number
  total_count: number
  migrating: boolean
  migration_progress: number
  migration_total: number
}

export const encryptionApi = {
  getStatus() {
    return api.get<EncryptionStatus>('/v2/encryption/status')
  },

  enable(passphrase: string) {
    return api.post('/v2/encryption/enable', { passphrase })
  },

  disable() {
    return api.post('/v2/encryption/disable')
  },

  rotateKey(oldPassphrase: string, newPassphrase: string) {
    return api.post('/v2/encryption/rotate-key', {
      old_passphrase: oldPassphrase,
      new_passphrase: newPassphrase,
    })
  },
}
