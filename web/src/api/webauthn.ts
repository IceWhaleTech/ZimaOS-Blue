import api from './client'

export interface WebAuthnCredential {
  id: string
  name: string
  created_at: string
  last_used_at: string
}

export interface WebAuthnStatus {
  enabled: boolean
  credentials: WebAuthnCredential[]
}

export interface RegistrationOptions {
  publicKey: PublicKeyCredentialCreationOptions
}

export interface AuthenticationOptions {
  publicKey: PublicKeyCredentialRequestOptions
}

export const webauthnApi = {
  // Get WebAuthn status and credentials
  getStatus: () => api.get<WebAuthnStatus>('/auth/webauthn/status'),

  // Begin registration ceremony
  beginRegistration: (name: string) =>
    api.post<RegistrationOptions>('/auth/webauthn/register/begin', { name }),

  // Complete registration ceremony
  finishRegistration: (credential: PublicKeyCredential, name: string) => {
    const attestationResponse = credential.response as AuthenticatorAttestationResponse
    return api.post<{ success: boolean }>('/auth/webauthn/register/finish', {
      id: credential.id,
      rawId: arrayBufferToBase64(credential.rawId),
      type: credential.type,
      response: {
        clientDataJSON: arrayBufferToBase64(attestationResponse.clientDataJSON),
        attestationObject: arrayBufferToBase64(attestationResponse.attestationObject),
      },
      name,
    })
  },

  // Delete a credential
  deleteCredential: (credentialId: string) =>
    api.delete(`/auth/webauthn/credentials/${credentialId}`),

  // Begin authentication ceremony
  beginAuthentication: () =>
    api.post<AuthenticationOptions>('/auth/webauthn/authenticate/begin'),

  // Complete authentication ceremony
  finishAuthentication: (credential: PublicKeyCredential) => {
    const assertionResponse = credential.response as AuthenticatorAssertionResponse
    return api.post<{ success: boolean }>('/auth/webauthn/authenticate/finish', {
      id: credential.id,
      rawId: arrayBufferToBase64(credential.rawId),
      type: credential.type,
      response: {
        clientDataJSON: arrayBufferToBase64(assertionResponse.clientDataJSON),
        authenticatorData: arrayBufferToBase64(assertionResponse.authenticatorData),
        signature: arrayBufferToBase64(assertionResponse.signature),
        userHandle: assertionResponse.userHandle
          ? arrayBufferToBase64(assertionResponse.userHandle)
          : null,
      },
    })
  },
}

// Helper functions for base64 encoding/decoding
function arrayBufferToBase64(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer)
  let binary = ''
  for (let i = 0; i < bytes.byteLength; i++) {
    binary += String.fromCharCode(bytes[i])
  }
  return btoa(binary)
}

export function base64ToArrayBuffer(base64: string): ArrayBuffer {
  const binary = atob(base64)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i)
  }
  return bytes.buffer
}

// Convert server options to browser-compatible format
export function prepareRegistrationOptions(options: RegistrationOptions): PublicKeyCredentialCreationOptions {
  const publicKey = options.publicKey
  return {
    ...publicKey,
    challenge: base64ToArrayBuffer(publicKey.challenge as unknown as string),
    user: {
      ...publicKey.user,
      id: base64ToArrayBuffer(publicKey.user.id as unknown as string),
    },
    excludeCredentials: publicKey.excludeCredentials?.map((cred) => ({
      ...cred,
      id: base64ToArrayBuffer(cred.id as unknown as string),
    })),
  }
}

export function prepareAuthenticationOptions(options: AuthenticationOptions): PublicKeyCredentialRequestOptions {
  const publicKey = options.publicKey
  return {
    ...publicKey,
    challenge: base64ToArrayBuffer(publicKey.challenge as unknown as string),
    allowCredentials: publicKey.allowCredentials?.map((cred) => ({
      ...cred,
      id: base64ToArrayBuffer(cred.id as unknown as string),
    })),
  }
}
