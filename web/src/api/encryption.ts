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
    return api.get<EncryptionStatus>('/encryption/status')
  },

  enable(passphrase: string) {
    return api.post('/encryption/enable', { passphrase })
  },

  disable() {
    return api.post('/encryption/disable')
  },

  rotateKey(oldPassphrase: string, newPassphrase: string) {
    return api.post('/encryption/rotate-key', {
      old_passphrase: oldPassphrase,
      new_passphrase: newPassphrase,
    })
  },
}
