import api from './client'

export type SmallModelRuntime = 'llama.cpp'
export type SmallModelID = 'qwen3.5-0.8b-gguf-q4km'
export type NoLLMDegradeMode = 'deepresearch'
export type SmallModelUnavailablePolicy = 'ir_first'
export type ContextCompressionMode = 'offline' | 'small_model' | 'auto'

export interface DirectoryWhitelistEntry {
  path: string
  alias?: string
}

// User settings stored on backend
export interface Settings {
  locale?: string // User's preferred locale (e.g., "zh-CN", "en-US")
  timezone?: string // User's timezone
  skill_selector_mode?: 'hybrid' | 'ir_only' | 'llm_only' // Skill selector strategy
  skill_rerank_enabled?: boolean // Enable stage-2 rerank (default false)
  skill_rerank_model?: string // Reranker model repo
  skill_rerank_onnx_enabled?: boolean // Enable ONNX reranker path (default false)
  skill_rerank_onnx_auto_download?: boolean // Allow ONNX model auto-download (default false)
  skill_selector_confidence_threshold?: number // Confidence threshold for auto skill selection
  prompt_policy_version?: string // Prompt policy version marker
  prompt_policy_profile?: 'default' // Prompt policy profile
  agent_mode?: boolean // Autonomous agent mode (default false)
  agent_auto_reflect?: boolean // Run post-task reflection in agent mode (default true)
  agent_auto_confirm?: boolean // Skip confirmation in agent mode (default false)
  agent_loop_policy_max_tool_rounds?: number // Agent loop max tool rounds
  agent_loop_policy_max_auto_continue?: number // Agent loop max auto-continue retries
  agent_loop_policy_pseudo_tool_call_budget?: number // Agent loop pseudo tool-call budget
  agent_loop_policy_action_pledge_budget?: number // Agent loop action-pledge budget
  agent_loop_policy_missing_todo_budget?: number // Agent loop missing-todo budget
  agent_loop_policy_pending_todo_budget?: number // Agent loop pending-todo budget
  memory_recall_mode?: 'aggressive' | 'balanced' | 'quality' // Memory recall strategy (default balanced)
  small_model_enabled?: boolean // Enable small-model routing features (default false)
  small_model_runtime?: SmallModelRuntime // Fixed: llama.cpp
  small_model_id?: SmallModelID // Fixed: qwen3.5-0.8b-gguf-q4km
  small_model_auto_download?: boolean // Auto download small model (default true)
  small_model_summary_enabled?: boolean // Phase1 default false
  small_model_context_compress_enabled?: boolean // Whether to prefer the lightweight-model compression path when available
  small_model_doc_extract_enabled?: boolean // Phase1 default false
  small_model_rerank_enabled?: boolean // Phase1 default false
  context_compression_mode?: ContextCompressionMode // Compression path preference; pressure-based triggering stays automatic
  small_model_context_prune_enabled?: boolean // Phase1 default false
  small_model_media_intent_enabled?: boolean // Phase1 default false
  offline_ir_fallback_enabled?: boolean // Offline IR fallback (default false)
  feature_intent_ir_enabled?: boolean // Channel feature-intent IR hints (default false)
  deep_research_v2_enabled?: boolean // default false
  small_model_route_image_qa_enabled?: boolean // default inherits short QA
  small_model_route_short_qa_enabled?: boolean // default false
  no_llm_degrade_mode?: NoLLMDegradeMode // Fixed deepresearch
  small_model_unavailable_policy?: SmallModelUnavailablePolicy // Fixed ir_first
  directory_whitelist_enabled?: boolean // default true with /tmp on non-Windows when unset
  directory_whitelist?: DirectoryWhitelistEntry[] // extra tool roots outside workspace; defaults to [/tmp] on non-Windows when unset
  voice_wake_enabled?: boolean // default false
  voice_wake_triggers?: string[] // default ["Hey Blue"]
  voice_wake_locale?: string // optional locale override
  voice_wake_target_conversation_id?: string // fixed background target conversation
  experimental_agentcore_runner_enabled?: boolean
  experimental_agentcore_runner_repo_url?: string
  experimental_agentcore_runner_ref?: string
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

export interface SmallModelStats {
  short_qa_route_attempts: number
  short_qa_route_success: number
  image_qa_route_attempts?: number
  image_qa_route_success?: number
  summary_attempts: number
  summary_success: number
  context_compress_attempts?: number
  context_compress_success?: number
  doc_extract_attempts: number
  doc_extract_success: number
  small_model_fallback_total?: number
  small_model_timeout_total?: number
  small_model_latency_ms?: number
  small_model_latency_samples?: number
  short_qa_latency_ms?: number
  short_qa_latency_samples?: number
  image_qa_latency_ms?: number
  image_qa_latency_samples?: number
  summary_latency_ms?: number
  summary_latency_samples?: number
  context_compress_latency_ms?: number
  context_compress_latency_samples?: number
  doc_extract_latency_ms?: number
  doc_extract_latency_samples?: number
  auto_rollback_total?: number
  no_provider_deepresearch_total: number
  ir_takeover_total: number
  fallback_reasons: Record<string, number>
}

export interface AgentcoreRunnerStatus {
  enabled: boolean
  repo_url?: string
  resolved_ref?: string
  resolved_commit?: string
  required_go_version?: string
  installed_go_version?: string
  toolchain_ready: boolean
  binary_ready: boolean
  binary_path?: string
  binary_sha256?: string
  last_prepare_at?: string
  last_prepare_state?: string
  last_error?: string
  last_optimization_run_id?: string
  last_optimization_at?: string
  last_optimization_state?: string
  last_optimization_summary?: string
  manifest_path?: string
  supported_parts?: string[]
  optimized_parts?: string[]
  primary_part?: string
  source_optimization_run_id?: string
  source_eval_run_id?: string
}

export interface AgentcoreRunnerLastRunTranscriptEntry {
  direction?: string
  method?: string
  id?: string
  text?: string
}

export interface AgentcoreRunnerLastRun {
  id?: string
  created_at?: string
  reason?: string
  candidate_id?: string
  eval_run_id?: string
  base_eval_run_id?: string
  optimization_surface?: string
  manifest_path?: string
  supported_parts?: string[]
  optimized_parts?: string[]
  primary_part?: string
  source_optimization_run_id?: string
  source_eval_run_id?: string
  repo_url?: string
  ref?: string
  runner_artifact_path?: string
  runner_artifact_sha256?: string
  runner_protocol?: string
  runner_session_id?: string
  runner_stop_reason?: string
  runner_response_text?: string
  runner_stderr?: string
  runner_error?: string
  runner_started_at?: string
  runner_finished_at?: string
  runner_duration_ms?: number
  metadata?: Record<string, unknown>
  runner_transcript?: AgentcoreRunnerLastRunTranscriptEntry[]
  [key: string]: unknown
}

export interface AgentcoreRunnerTagList {
  repo_url: string
  default_ref: string
  tags: string[]
}

export interface AgentcoreRunnerPrepareRequest {
  requested_parts?: string[]
}

export interface PromptPolicyStatus {
  prompt_policy_version: string
  prompt_policy_profile: string
  prompt_policy_hash: string
  agent_loop_policy: {
    max_tool_rounds: number
    max_auto_continue: number
    pseudo_tool_call_budget: number
    action_pledge_budget: number
    missing_todo_budget: number
    pending_todo_budget: number
  }
}

export interface SelectorDryRunResponse {
  query: string
  model: string
  selected_tools: string[]
  skill_decision?: unknown
  skill_prompt_hint?: string
  skill_selector_error?: string
}

// Settings API
export const settingsApi = {
  // Get user settings
  get: () => api.get<Settings>('/settings'),

  // Update user settings (full update)
  update: (settings: Settings) => api.put<Settings>('/settings', settings),

  // Patch user settings (partial update)
  patch: (updates: Partial<Settings>) => api.patch<Settings>('/settings', updates),

  // Skill reranker ONNX model management
  getSkillRerankerModelStatus: () =>
    api.get<SkillRerankerModelStatus>('/settings/skill-reranker/model/status'),
  downloadSkillRerankerModel: () =>
    api.post<{ success: boolean; message?: string }>('/settings/skill-reranker/model/download'),
  cancelSkillRerankerModelDownload: () =>
    api.post<{ success: boolean }>('/settings/skill-reranker/model/cancel'),

  // Fixed small-model management
  getSmallModelStatus: () => api.get<SmallModelStatus>('/settings/small-model/status'),
  downloadSmallModel: () =>
    api.post<{ success: boolean; message?: string }>('/settings/small-model/download'),
  cancelSmallModelDownload: () => api.post<{ success: boolean }>('/settings/small-model/cancel'),
  getAgentcoreRunnerStatus: () => api.get<AgentcoreRunnerStatus>('/settings/agentcore-runner/status'),
  getAgentcoreRunnerLastRun: () => api.get<AgentcoreRunnerLastRun | null>('/settings/agentcore-runner/last-run'),
  getAgentcoreRunnerTags: (repoURL?: string) =>
    api.get<AgentcoreRunnerTagList>('/settings/agentcore-runner/tags', {
      params: repoURL ? { repo_url: repoURL } : undefined,
    }),
  prepareAgentcoreRunner: (request?: AgentcoreRunnerPrepareRequest) =>
    api.post<AgentcoreRunnerStatus>('/settings/agentcore-runner/prepare', request ?? {}),
  // Small-model observability counters
  getSmallModelStats: () => api.get<SmallModelStats>('/small-model/stats'),
  resetSmallModelStats: () => api.post<{ success: boolean }>('/small-model/stats/reset'),

  // Prompt/selector policy observability
  getPromptPolicyStatus: () => api.get<PromptPolicyStatus>('/settings/prompt-policy'),
  selectorDryRun: (query: string, model?: string) =>
    api.post<SelectorDryRunResponse>('/settings/selector/dry-run', { query, model }),
}
