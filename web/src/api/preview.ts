import api from './client'

// Types
export interface SystemMode {
  mode: 'preview' | 'normal'
  features: Record<string, boolean>
}

export interface PreviewStatus {
  active: boolean
  message: string
}

export interface UpgradeRequest {
  username: string
  password: string
}

export interface UpgradeResponse {
  success: boolean
  user: {
    id: string
    username: string
    role: string
  }
  data_migrated: boolean
  migrated_counts?: Record<string, number>
}

export interface PresetQuestionAttachment {
  type: 'image' | 'file'
  name: string
  mime_type: string
  placeholder?: string
}

export interface PresetQuestion {
  id: string
  title?: string
  description?: string
  prompt?: string
  text: string
  category: string
  tags?: string[]
  icon?: string
  editorial_score?: number
  attachments?: PresetQuestionAttachment[]
}

export interface PresetQuestionsResponse {
  questions: PresetQuestion[]
  total: number
  next_offset: number
  has_more: boolean
}

export interface PreviewTokenResponse {
  token: string
  expires_in: number
}

export interface OnboardingStatusResponse {
  seen: boolean
}

// API functions
export const previewApi = {
  /**
   * Get current system mode (preview or normal)
   */
  getSystemMode: () => api.get<SystemMode>('/system/mode'),

  /**
   * Get preview mode status
   */
  getStatus: () => api.get<PreviewStatus>('/preview/status'),

  /**
   * Upgrade from preview mode by creating admin account
   */
  upgrade: (data: UpgradeRequest) => api.post<UpgradeResponse>('/preview/upgrade', data),

  /**
   * Get preset questions for empty chat area
   * @param count Number of questions to return
   * @param lang Language code (e.g., 'en', 'zh')
   * @param offset Starting offset for incremental loading
   */
  getPresetQuestions: (count = 4, lang?: string, offset = 0) => {
    const params = new URLSearchParams({ count: String(count) })
    params.append('offset', String(offset))
    if (lang) {
      params.append('lang', lang)
    }
    return api.get<PresetQuestionsResponse>(`/preset-questions?${params.toString()}`)
  },

  /**
   * Get a temporary token for preview mode
   */
  getPreviewToken: () => api.post<PreviewTokenResponse>('/preview/token', {}),

  /**
   * Get onboarding status (whether user has seen the onboarding modal)
   */
  getOnboardingStatus: () => api.get<OnboardingStatusResponse>('/preview/onboarding'),

  /**
   * Mark onboarding as seen
   */
  setOnboardingSeen: () => api.post<{ success: boolean }>('/preview/onboarding', {}),
}
