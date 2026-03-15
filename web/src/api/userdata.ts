import api from './client'

// User settings for export
export interface UserSettings {
  selected_provider_model?: string
  temperature: number
  max_tokens: number
  theme?: string
  locale?: string
  timezone?: string
}

// Export request
export interface ExportRequest {
  password: string
  format: 'json' | 'encrypted'
  settings?: UserSettings
}

// Import request
export interface ImportRequest {
  password: string
  data: string // Base64 encoded data
}

// Export response for JSON format
export interface UserDataExport {
  version: string
  exported_at: string
  data_type: string
  settings?: UserSettings
  chat_history?: {
    conversations: Array<{
      id: string
      title: string
      created_at: string
      updated_at: string
      messages: Array<{
        id: string
        role: string
        content: string
        provider?: string
        model?: string
        created_at: string
      }>
    }>
    total_messages: number
  }
  checksum?: string
}

// Export response for encrypted format
export interface EncryptedExport {
  version: string
  format: string
  salt: string
  iv: string
  data: string
  checksum: string
}

// Import preview response
export interface ImportPreview {
  version: string
  exported_at: string
  data_type: string
  has_settings?: boolean
  settings_preview?: {
    theme?: string
    locale?: string
  }
  has_chat_history?: boolean
  chat_preview?: {
    conversations: number
    messages: number
  }
}

// Import result
export interface ImportResult {
  success: boolean
  imported: {
    conversations: number
    messages: number
    settings: boolean
  }
  settings?: UserSettings
}

export const userDataApi = {
  // Export user data
  export: (request: ExportRequest) =>
    api.post<UserDataExport | EncryptedExport>('/userdata/export', request),

  // Import user data
  import: (request: ImportRequest) => api.post<ImportResult>('/userdata/import', request),

  // Preview import data
  importPreview: (request: ImportRequest) =>
    api.post<ImportPreview>('/userdata/import/preview', request),
}
