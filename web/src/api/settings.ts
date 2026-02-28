import api from './client'

// User settings stored on backend
export interface Settings {
  locale?: string   // User's preferred locale (e.g., "zh-CN", "en-US")
  timezone?: string // User's timezone
  theme_style?: string // Chat theme style (default/bubble/minimal/gradient/ocean)
  smart_tool_selection?: boolean // IR-based tool filtering (default true)
  smart_skill_selection?: boolean // Progressive skill selector (default true)
  skill_selector_mode?: 'hybrid' | 'ir_only' | 'llm_only' // Skill selector strategy
  skill_rerank_enabled?: boolean // Enable stage-2 rerank (default true)
  skill_rerank_model?: string // Reranker model repo
  skill_rerank_onnx_enabled?: boolean // Enable ONNX reranker path (default false)
  skill_rerank_onnx_auto_download?: boolean // Allow ONNX model auto-download (default false)
  skill_selector_confidence_threshold?: number // Confidence threshold for auto skill selection
  agent_mode?: boolean // Autonomous agent mode (default false)
  agent_auto_confirm?: boolean // Skip confirmation in agent mode (default false)
  memory_recall_mode?: 'aggressive' | 'balanced' | 'quality' // Memory recall strategy (default balanced)
}

// Smart tool selection stats
export interface ToolSelectorStats {
  requests: number
  tools_total: number
  tools_sent: number
  tools_skipped: number
  tokens_saved: number
}

export interface SkillRerankerModelStatus {
  ready: boolean
  downloading: boolean
  state?: string
  error?: string
  progress?: {
    file: string
    file_index: number
    total_files: number
    downloaded: number
    total: number
    percentage: number
    speed_human: string
    eta: string
  }
  files?: {
    filename: string
    downloaded: boolean
    size: string
  }[]
}

// Settings API
export const settingsApi = {
  // Get user settings
  get: () => api.get<Settings>('/settings'),

  // Update user settings (full update)
  update: (settings: Settings) => api.put<Settings>('/settings', settings),

  // Patch user settings (partial update)
  patch: (updates: Partial<Settings>) => api.patch<Settings>('/settings', updates),

  // Get smart tool selection stats
  getToolStats: () => api.get<ToolSelectorStats>('/tools/stats'),

  // Skill reranker ONNX model management
  getSkillRerankerModelStatus: () => api.get<SkillRerankerModelStatus>('/settings/skill-reranker/model/status'),
  downloadSkillRerankerModel: () => api.post<{ success: boolean; message?: string }>('/settings/skill-reranker/model/download'),
  cancelSkillRerankerModelDownload: () => api.post<{ success: boolean }>('/settings/skill-reranker/model/cancel'),
}
