import api from './client'

export type SmallModelRuntime = 'llama_cpp_native'
export type SmallModelID = 'lfm2.5-1.2b-instruct-q4km'
export type NoLLMDegradeMode = 'deepresearch'
export type SmallModelUnavailablePolicy = 'ir_first'
export type SoulProposalStatus = 'pending' | 'approved' | 'rejected'

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
  small_model_enabled?: boolean // Enable small-model routing features (default false)
  small_model_runtime?: SmallModelRuntime // Fixed: llama_cpp_native
  small_model_id?: SmallModelID // Fixed: lfm2.5-1.2b-instruct-q4km
  small_model_auto_download?: boolean // Auto download small model (default true)
  small_model_shadow_ratio?: number // Shadow sample ratio in range (0, 1], default 0.1
  small_model_summary_enabled?: boolean // Phase1 default true
  small_model_doc_extract_enabled?: boolean // Phase1 default true
  small_model_rerank_enabled?: boolean // Phase1 default true
  small_model_context_prune_enabled?: boolean // Phase1 default true
  small_model_route_short_qa_enabled?: boolean // Phase1 default false (shadow only)
  small_model_route_tool_dispatch_enabled?: boolean // Phase1 default false (shadow only)
  no_llm_degrade_mode?: NoLLMDegradeMode // Fixed deepresearch
  small_model_unavailable_policy?: SmallModelUnavailablePolicy // Fixed ir_first
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

export interface SmallModelDownloadProgress {
  file: string
  file_index: number
  total_files: number
  downloaded: number
  total: number
  percentage: number
  speed_human: string
  eta: string
}

export interface SmallModelFileStatus {
  filename: string
  downloaded: boolean
  size: string
}

export interface SmallModelStatus {
  ready: boolean
  downloading: boolean
  state?: string
  error?: string
  model_id: SmallModelID
  runtime: SmallModelRuntime
  model_path: string
  progress?: SmallModelDownloadProgress
  files?: SmallModelFileStatus[]
}

export interface SoulProposal {
  id: string
  title: string
  content: string
  source?: string
  status: SoulProposalStatus
  created_at: string
  reviewed_at?: string
}

export interface SoulProposalListResponse {
  proposals: SoulProposal[]
}

export interface SmallModelStats {
  short_qa_route_attempts: number
  short_qa_route_success: number
  tool_dispatch_route_attempts: number
  tool_dispatch_route_success: number
  summary_attempts: number
  summary_success: number
  doc_extract_attempts: number
  doc_extract_success: number
  short_qa_shadow_total: number
  tool_dispatch_shadow_total: number
  shadow_failures: number
  small_model_fallback_total?: number
  small_model_timeout_total?: number
  small_model_latency_ms?: number
  small_model_latency_samples?: number
  short_qa_latency_ms?: number
  short_qa_latency_samples?: number
  tool_dispatch_latency_ms?: number
  tool_dispatch_latency_samples?: number
  summary_latency_ms?: number
  summary_latency_samples?: number
  doc_extract_latency_ms?: number
  doc_extract_latency_samples?: number
  shadow_quality_delta?: number
  shadow_quality_samples?: number
  auto_rollback_total?: number
  no_provider_deepresearch_total: number
  ir_takeover_total: number
  fallback_reasons: Record<string, number>
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

  // Fixed small-model management
  getSmallModelStatus: () => api.get<SmallModelStatus>('/settings/small-model/status'),
  downloadSmallModel: () => api.post<{ success: boolean; message?: string }>('/settings/small-model/download'),
  cancelSmallModelDownload: () => api.post<{ success: boolean }>('/settings/small-model/cancel'),

  // SOUL proposal review workflow
  listSoulProposals: () => api.get<SoulProposalListResponse>('/settings/soul/proposals'),
  approveSoulProposal: (id: string) => api.post<SoulProposal>(`/settings/soul/proposals/${id}/approve`),
  rejectSoulProposal: (id: string) => api.post<SoulProposal>(`/settings/soul/proposals/${id}/reject`),

  // Small-model observability counters
  getSmallModelStats: () => api.get<SmallModelStats>('/small-model/stats'),
  resetSmallModelStats: () => api.post<{ success: boolean }>('/small-model/stats/reset'),
}
