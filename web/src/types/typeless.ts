// Typeless Card Types
// These cards are rendered inline within chat messages to provide
// structured information, progress updates, and interactive elements.

export type TypelessCardType =
  | 'info'
  | 'progress'
  | 'action'
  | 'result'
  | 'detection'
  | 'table'
  | 'chart'
  | 'code'
  | 'gallery'
  | 'quote'
  | 'list'
  | 'file'
  | 'link'
  | 'metric'
  | 'comparison'
  | 'steps'
  | 'map'
  | 'weather'
  | 'profile'
  | 'countdown'
  | 'rating'
  | 'accordion'
  | 'alert'
  | 'audio'
  | 'choice'
  | 'collapsible-code'
  | 'diff'
  | 'terminal'
  | 'video'
  | 'mermaid'
  | 'search'
  | 'deep-research'
  | 'deep-research-progress'
  | 'deep-research-event'
  | 'ui-review'
  | 'ui-review-progress'
  | 'media-generate'
  | 'analyze-progress'
  | 'browser-progress'
  | 'web-fetch'
  | 'convert-task'
  | 'exec'

export interface TypelessCardBase {
  type: TypelessCardType
  id?: string
  _streaming?: boolean // Internal flag for streaming/incomplete cards
}

// Info Card - Display informational content with optional icon
export interface TypelessCardInfo extends TypelessCardBase {
  type: 'info'
  title?: string
  content: string
  icon?: string
  variant?: 'default' | 'warning' | 'success' | 'error'
}

// Progress Card - Show progress of an operation
export interface TypelessCardProgress extends TypelessCardBase {
  type: 'progress'
  title: string
  progress: number // 0-100
  status?: 'running' | 'completed' | 'failed' | 'paused'
  message?: string
  steps?: ProgressStep[]
}

export interface ProgressStep {
  name: string
  status: 'pending' | 'running' | 'completed' | 'failed'
  message?: string
}

// Action Card - Interactive card with buttons
export interface TypelessCardAction extends TypelessCardBase {
  type: 'action'
  title: string
  description?: string
  actions: ActionButton[]
}

export interface ActionButton {
  id: string
  label: string
  variant?: 'primary' | 'secondary' | 'danger'
  icon?: string
  disabled?: boolean
  form_data?: Record<string, unknown>
}

// Result Card - Display operation results
export interface TypelessCardResult extends TypelessCardBase {
  type: 'result'
  title: string
  status: 'success' | 'error' | 'warning' | 'info'
  message?: string
  image?: string
  warning?: string
  warning_code?: string
  details?: ResultDetail[]
  actions?: ActionButton[]
}

export interface ResultDetail {
  label: string
  value: string | Record<string, unknown>
  copyable?: boolean
  suffix?: string
  multiline?: boolean
}

// Detection Card - Display image with object detection boxes
export interface TypelessCardDetection extends TypelessCardBase {
  type: 'detection'
  image: string // Base64 or URL of the image
  detections: Detection[]
  title?: string
}

export interface Detection {
  label: string
  confidence?: number // 0-1
  bbox: BoundingBox // Bounding box coordinates
  color?: string // Optional custom color for the box
}

export interface BoundingBox {
  x: number // Top-left x (percentage 0-100 or pixels)
  y: number // Top-left y (percentage 0-100 or pixels)
  width: number // Width (percentage 0-100 or pixels)
  height: number // Height (percentage 0-100 or pixels)
  unit?: 'percent' | 'pixel' // Default: percent
}

// Table Card - Display tabular data
export interface TypelessCardTable extends TypelessCardBase {
  type: 'table'
  title?: string
  headers: string[]
  rows: (string | number | null)[][]
  footer?: string
  striped?: boolean
  compact?: boolean
}

// Chart Card - Display simple charts
export interface TypelessCardChart extends TypelessCardBase {
  type: 'chart'
  title?: string
  chartType: 'bar' | 'line' | 'pie' | 'donut'
  data: ChartDataPoint[]
  showLegend?: boolean
  showValues?: boolean
}

export interface ChartDataPoint {
  label: string
  value: number
  color?: string
}

// Code Card - Display code with syntax highlighting
export interface TypelessCardCode extends TypelessCardBase {
  type: 'code'
  title?: string
  language?: string
  code: string
  filename?: string
  showLineNumbers?: boolean
  highlightLines?: number[]
}

// Gallery Card - Display multiple images
export interface TypelessCardGallery extends TypelessCardBase {
  type: 'gallery'
  title?: string
  images: GalleryImage[]
  columns?: 2 | 3 | 4
  layout?: 'grid' | 'horizontal' // horizontal for swipe/scroll
}

export interface GalleryImage {
  src: string
  thumbnail?: string // Thumbnail URL for local images
  alt?: string
  caption?: string
}

// Quote Card - Display a quote or citation
export interface TypelessCardQuote extends TypelessCardBase {
  type: 'quote'
  content: string
  author?: string
  source?: string
  variant?: 'default' | 'highlight' | 'warning'
}

// List Card - Display a structured list
export interface TypelessCardList extends TypelessCardBase {
  type: 'list'
  title?: string
  items: ListItem[]
  ordered?: boolean
  variant?: 'default' | 'checklist' | 'timeline'
}

export interface ListItem {
  content: string
  icon?: string
  checked?: boolean // For checklist variant
  timestamp?: string // For timeline variant
  subItems?: ListItem[]
}

// File Card - Display file information
export interface TypelessCardFile extends TypelessCardBase {
  type: 'file'
  filename: string
  size?: string
  mimeType?: string
  downloadUrl?: string
  previewUrl?: string
  icon?: string
}

// Link Card - Display a rich link preview
export interface TypelessCardLink extends TypelessCardBase {
  type: 'link'
  url: string
  title: string
  description?: string
  image?: string
  siteName?: string
  favicon?: string
}

// Metric Card - Display key metrics/stats
export interface TypelessCardMetric extends TypelessCardBase {
  type: 'metric'
  title?: string
  metrics: MetricItem[]
  columns?: 2 | 3 | 4
}

export interface MetricItem {
  label: string
  value: string | number
  unit?: string
  change?: number // Percentage change
  changeDirection?: 'up' | 'down' | 'neutral'
  icon?: string
}

// Comparison Card - Compare two or more items side by side
export interface TypelessCardComparison extends TypelessCardBase {
  type: 'comparison'
  title?: string
  items: ComparisonItem[]
  features: ComparisonFeature[]
}

export interface ComparisonItem {
  name: string
  image?: string
  price?: string
  badge?: string // e.g., "Best Value", "Popular"
  highlighted?: boolean
}

export interface ComparisonFeature {
  name: string
  values: (string | boolean | number)[] // One value per item
}

// Steps Card - Display a step-by-step wizard/process
export interface TypelessCardSteps extends TypelessCardBase {
  type: 'steps'
  title?: string
  steps: StepItem[]
  currentStep?: number
  variant?: 'horizontal' | 'vertical'
}

export interface StepItem {
  title: string
  description?: string
  icon?: string
  status?: 'pending' | 'current' | 'completed' | 'error'
}

// Map Card - Display a location on a map
export interface TypelessCardMap extends TypelessCardBase {
  type: 'map'
  title?: string
  latitude: number
  longitude: number
  zoom?: number
  address?: string
  markers?: MapMarker[]
}

export interface MapMarker {
  latitude: number
  longitude: number
  label?: string
  color?: string
}

// Weather Card - Display weather information
export interface TypelessCardWeather extends TypelessCardBase {
  type: 'weather'
  location: string
  temperature: number
  unit?: 'celsius' | 'fahrenheit'
  condition: 'sunny' | 'cloudy' | 'rainy' | 'snowy' | 'stormy' | 'foggy' | 'windy' | 'partly-cloudy'
  humidity?: number
  windSpeed?: number
  windUnit?: string
  forecast?: WeatherForecast[]
}

export interface WeatherForecast {
  day: string
  high: number
  low: number
  condition: string
}

// Profile Card - Display user/person information
export interface TypelessCardProfile extends TypelessCardBase {
  type: 'profile'
  name: string
  avatar?: string
  title?: string
  description?: string
  stats?: ProfileStat[]
  links?: ProfileLink[]
  verified?: boolean
}

export interface ProfileStat {
  label: string
  value: string | number
}

export interface ProfileLink {
  icon: string
  url: string
  label?: string
}

// Countdown Card - Display a countdown timer
export interface TypelessCardCountdown extends TypelessCardBase {
  type: 'countdown'
  title?: string
  targetDate: string // ISO date string
  description?: string
  showDays?: boolean
  showHours?: boolean
  showMinutes?: boolean
  showSeconds?: boolean
  variant?: 'default' | 'compact' | 'large'
}

// Rating Card - Display ratings and reviews
export interface TypelessCardRating extends TypelessCardBase {
  type: 'rating'
  title?: string
  rating: number // 0-5
  maxRating?: number
  reviewCount?: number
  breakdown?: RatingBreakdown[]
  review?: {
    author: string
    avatar?: string
    date?: string
    content: string
  }
}

export interface RatingBreakdown {
  stars: number
  count: number
  percentage?: number
}

// Accordion Card - Collapsible FAQ/content sections
export interface TypelessCardAccordion extends TypelessCardBase {
  type: 'accordion'
  title?: string
  items: AccordionItem[]
  allowMultiple?: boolean
}

export interface AccordionItem {
  title: string
  content: string
  icon?: string
  defaultOpen?: boolean
}

// Alert Card - Display important alerts/banners
export interface TypelessCardAlert extends TypelessCardBase {
  type: 'alert'
  title?: string
  title_key?: string
  message: string
  variant: 'info' | 'success' | 'warning' | 'error'
  icon?: string
  dismissible?: boolean
  actions?: ActionButton[]
}

// Audio Card - Display audio player
export interface TypelessCardAudio extends TypelessCardBase {
  type: 'audio'
  title?: string
  artist?: string
  album?: string
  coverImage?: string
  src: string
  duration?: number // in seconds
}

// Choice Card - Single or multi-select options with optional "Other" input
export interface TypelessCardChoice extends TypelessCardBase {
  type: 'choice'
  title?: string
  description?: string
  options: ChoiceOption[]
  multiple?: boolean // Allow multiple selections
  allowOther?: boolean // Show "Other" option with text input
  otherPlaceholder?: string
  required?: boolean
  selectedIds?: string[] // Pre-selected option IDs
}

export interface ChoiceOption {
  id: string
  label: string
  description?: string
  icon?: string
  disabled?: boolean
}

// Collapsible Code Card - Code block that can be expanded/collapsed
export interface TypelessCardCollapsibleCode extends TypelessCardBase {
  type: 'collapsible-code'
  title?: string
  language?: string
  code: string
  filename?: string
  defaultExpanded?: boolean
  maxCollapsedLines?: number // Number of lines to show when collapsed
  showLineNumbers?: boolean
}

// Diff Card - Display code differences
export interface TypelessCardDiff extends TypelessCardBase {
  type: 'diff'
  title?: string
  filename?: string
  language?: string
  oldCode: string
  newCode: string
  oldLabel?: string // e.g., "Before", "Original"
  newLabel?: string // e.g., "After", "Modified"
  viewMode?: 'split' | 'unified' // Side-by-side or unified view
}

// Terminal Card - Display terminal/CLI output with ANSI color support
export interface TypelessCardTerminal extends TypelessCardBase {
  type: 'terminal'
  title?: string
  content: string // Raw terminal output with ANSI escape codes
  prompt?: string // Optional prompt prefix (e.g., "$ ", "> ")
  showPrompt?: boolean // Whether to show the prompt
  maxHeight?: number // Max height in pixels before scrolling
  theme?: 'dark' | 'light' // Terminal theme
}

// Video Card - Display video player
export interface TypelessCardVideo extends TypelessCardBase {
  type: 'video'
  title?: string
  src: string // Video URL
  poster?: string // Cover/thumbnail image
  duration?: number // Duration in seconds
  autoplay?: boolean
  muted?: boolean
  loop?: boolean
  controls?: boolean // Show native controls (default: true)
  subtitles?: VideoSubtitle[]
}

export interface VideoSubtitle {
  src: string // Subtitle file URL (VTT format)
  label: string // Display label (e.g., "English", "中文")
  srclang: string // Language code (e.g., "en", "zh")
  default?: boolean
}

// Mermaid Card - Display Mermaid diagrams (flowchart, mindmap, sequence, etc.)
export interface TypelessCardMermaid extends TypelessCardBase {
  type: 'mermaid'
  title?: string
  code: string // Mermaid diagram code
  theme?: 'default' | 'dark' | 'forest' | 'neutral' // Mermaid theme
}

// Search Card - Display web search results
export interface SearchResultItem {
  title: string
  url: string
  description?: string
  source?: string
}

export interface TypelessCardSearch extends TypelessCardBase {
  type: 'search'
  query: string
  results: SearchResultItem[]
  totalCount?: number
  provider?: string
}

export interface DeepResearchCitationItem {
  evidence_id?: string
  title?: string
  url: string
}

export interface DeepResearchEntityDisambiguation {
  enabled?: boolean
  threshold?: number
  filtered_count?: number
  ambiguous_count?: number
}

export interface DeepResearchTimelineSection {
  label: string
  highlights?: string[]
  evidence_ids?: string[]
}

export interface DeepResearchTraceItem {
  iteration: number
  focus?: string
  gap?: string
  follow_up_query?: string
  evidence_added?: number
  verification_outcome?: string
}

export interface DeepResearchVerificationItem {
  focus?: string
  gap?: string
  status?: 'resolved' | 'conflicted' | 'insufficient' | string
  summary?: string
  evidence_ids?: string[]
}

export interface DeepResearchVerificationSummary {
  resolved_count?: number
  conflicted_count?: number
  insufficient_count?: number
  items?: DeepResearchVerificationItem[]
}
export interface DeepResearchWorkflowPhase {
  id?: string
  label?: string
  status?: 'pending' | 'current' | 'completed' | string
}

export interface DeepResearchObjectMapItem {
  id?: string
  label?: string
  task_count?: number
  questions?: string[]
  time_windows?: string[]
  status_counts?: Record<string, number>
}

export interface DeepResearchSourceInventoryItem {
  source_id?: string
  title?: string
  url?: string
  source_type?: string
  domain?: string
  fetched_at?: string
  published_at?: string
  relevance_score?: number
  credibility_score?: number
  claim_key?: string
  time_label?: string
}

export interface DeepResearchCoverageSummary {
  task_count?: number
  evidence_count?: number
  distinct_domain_count?: number
  citation_count?: number
  open_question_count?: number
  citation_coverage?: number
  resolved_count?: number
  conflicted_count?: number
  insufficient_count?: number
}


export interface DeepResearchBrief {
  goal?: string
  entity?: string
  time_windows?: string[]
  must_verify_claims?: string[]
}

export interface DeepResearchPlannedTask {
  question: string
  axis?: string
  category?: string
  time_window?: string
}

export interface DeepResearchLiveSource {
  title?: string
  domain?: string
  url?: string
  query?: string
}

export interface DeepResearchEventVerificationSummary {
  resolved_count?: number
  conflicted_count?: number
  insufficient_count?: number
}

export interface TypelessCardDeepResearchEvent extends TypelessCardBase {
  type: 'deep-research-event'
  job_id?: string
  conversation_id?: string
  query?: string
  mode?: 'fast' | 'standard' | 'deep'
  event_kind?: string
  status?: string
  summary?: string
  brief?: DeepResearchBrief
  tasks?: DeepResearchPlannedTask[]
  sources?: DeepResearchLiveSource[]
  verification?: DeepResearchEventVerificationSummary
  gap?: string
  focus?: string
  follow_up_query?: string
  stage?: string
  search_query?: string
  parallelism?: number
  source_title?: string
  stop_reason?: string
  iteration?: number
  task_count?: number
  evidence_count?: number
  citation_coverage?: number
  attempt?: number
  delay_ms?: number
}

export interface TypelessCardDeepResearch extends TypelessCardBase {
  type: 'deep-research'
  job_id?: string
  conversation_id?: string
  query?: string
  mode?: 'fast' | 'standard' | 'deep'
  progress?: number
  iteration?: number
  latest_gap?: string
  latest_action?: string
  answer?: string
  confidence?: number
  evidence_count?: number
  iterations?: number
  stop_reason?: string
  citations?: DeepResearchCitationItem[]
  open_questions?: string[]
  support_count?: number
  conflict_count?: number
  has_conflict?: boolean
  citation_coverage?: number
  entity_disambiguation?: DeepResearchEntityDisambiguation
  stage_errors?: string[]
  timeline_sections?: DeepResearchTimelineSection[]
  strict_entity?: boolean
  time_windows?: string[]
  report_style?: string
  workflow_phases?: DeepResearchWorkflowPhase[]
  object_map?: DeepResearchObjectMapItem[]
  source_inventory?: DeepResearchSourceInventoryItem[]
  coverage_summary?: DeepResearchCoverageSummary
  research_trace?: DeepResearchTraceItem[]
  verification_summary?: DeepResearchVerificationSummary
  status?: string
}

export interface TypelessCardDeepResearchProgress extends TypelessCardBase {
  type: 'deep-research-progress'
  job_id?: string
  conversation_id?: string
  query?: string
  mode?: 'fast' | 'standard' | 'deep'
  stage?: string
  status?: string
  progress?: number
  iteration?: number
  latest_gap?: string
  latest_action?: string
}

// UI Review Card - Display UI quality review results with collapsible steps
export interface UIReviewStep {
  id: string
  name: string
  status: 'success' | 'failed' | 'skipped'
  score?: number
  message?: string
  issues?: number
}

export interface UIReviewScoreDetail {
  score: number
  details?: Record<string, number>
}

export interface UIReviewIssue {
  severity: 'critical' | 'major' | 'minor'
  category: string
  rule?: string
  element?: string
  description: string
  location?: string
}

export interface TypelessCardUIReview extends TypelessCardBase {
  type: 'ui-review'
  url?: string
  overall?: number
  pass?: boolean
  threshold?: number
  visual?: UIReviewScoreDetail
  functional?: UIReviewScoreDetail
  accessibility?: UIReviewScoreDetail
  issues?: UIReviewIssue[]
  suggestions?: string[]
  steps?: UIReviewStep[]
  viewports?: string[]
  screenshot?: string
  media_url?: string
  thumbnail_url?: string
  screenshots?: string[]
  device?: string
  channel?: string
  human?: string
  status?: string
  message?: string
  actions?: ActionButton[]
}

// UI Review Progress card — streaming step-by-step progress during review
export interface UIReviewProgressStep {
  step: string
  name: string
  status: string
  url?: string
  score?: number
}

export interface TypelessCardUIReviewProgress extends TypelessCardBase {
  type: 'ui-review-progress'
  // Single step (raw from backend)
  step?: string
  name?: string
  status?: string
  url?: string
  score?: number
  // Merged steps (after frontend consolidation)
  steps?: UIReviewProgressStep[]
}

// Media generation card — shows placeholder during generation, image/video on completion
export interface TypelessCardMediaGenerate extends TypelessCardBase {
  type: 'media-generate'
  media_type: 'image' | 'video'
  status: 'generating' | 'success' | 'error'
  task_id?: string
  message?: string
  elapsed_ms?: number
  images?: GalleryImage[]
}

// Analyze Progress card — streaming step-by-step progress during analysis
export interface AnalyzeProgressStep {
  step: string
  name: string
  status: string
  detail?: string
  current?: number
  total?: number
  source_kind?: string
  source_label?: string
  char_count?: number
  result_count?: number
}

export interface TypelessCardAnalyzeProgress extends TypelessCardBase {
  type: 'analyze-progress'
  // Single step (raw from backend)
  step?: string
  name?: string
  status?: string
  detail?: string
  current?: number
  total?: number
  source_kind?: string
  source_label?: string
  char_count?: number
  result_count?: number
  // Merged steps (after frontend consolidation)
  steps?: AnalyzeProgressStep[]
}

// Browser Progress card — streaming step-by-step progress during browser actions
export interface BrowserProgressStep {
  step: string
  name: string
  status: string
  url?: string
  recipe_name?: string
}

export interface TypelessCardBrowserProgress extends TypelessCardBase {
  type: 'browser-progress'
  // Single step (raw from backend)
  step?: string
  name?: string
  status?: string
  url?: string
  recipe_name?: string
  // Merged steps (after frontend consolidation)
  steps?: BrowserProgressStep[]
}

export interface TypelessCardWebFetch extends TypelessCardBase {
  type: 'web-fetch'
  title: string
  status: 'success' | 'warning' | 'error'
  url?: string
  content?: string
  content_type?: string
  extract_mode?: 'markdown' | 'text' | string
  extractor?: string
  truncated?: boolean
  warning?: string
  warning_code?: string
  actions?: ActionButton[]
}

export interface ConvertTaskOutput {
  output_id: string
  name: string
  mime_type?: string
  size_bytes?: number
  preview_kind?: 'file' | 'audio' | 'video' | 'image' | 'pdf' | 'text' | string
  download_url?: string
  ref?: string
  preview_text?: string
}

export interface TypelessCardConvertTask extends TypelessCardBase {
  type: 'convert-task'
  task_id: string
  status: 'pending' | 'processing' | 'succeeded' | 'failed' | 'cancelled' | string
  action?: string
  source_summary?: string
  target_format?: string
  progress?: number
  message?: string
  error?: string
  outputs?: ConvertTaskOutput[]
  transcript_preview?: string
}

// Exec Card - Display shell command execution results with terminal styling
export interface TypelessCardExec extends TypelessCardBase {
  type: 'exec'
  command?: string
  command_redacted?: boolean
  hide_command?: boolean
  message?: string
  warning?: string
  warning_code?: string
  status: 'success' | 'error' | 'running'
  exit_code?: number
  stdout?: string
  stdout_redacted?: boolean
  stderr?: string
  stderr_redacted?: boolean
  duration_ms?: number
  truncated?: boolean
  warnings?: string[]
  warning_count?: number
  warnings_redacted?: boolean
  host?: 'local' | 'sandbox' | 'builtin'
  risk_level?: 'low' | 'medium' | 'high' | 'critical'
  session_id?: string
  _streaming?: boolean
  skill_name?: string
}

// Union type for all card types
export type TypelessCard =
  | TypelessCardInfo
  | TypelessCardProgress
  | TypelessCardAction
  | TypelessCardResult
  | TypelessCardDetection
  | TypelessCardTable
  | TypelessCardChart
  | TypelessCardCode
  | TypelessCardGallery
  | TypelessCardQuote
  | TypelessCardList
  | TypelessCardFile
  | TypelessCardLink
  | TypelessCardMetric
  | TypelessCardComparison
  | TypelessCardSteps
  | TypelessCardMap
  | TypelessCardWeather
  | TypelessCardProfile
  | TypelessCardCountdown
  | TypelessCardRating
  | TypelessCardAccordion
  | TypelessCardAlert
  | TypelessCardAudio
  | TypelessCardChoice
  | TypelessCardCollapsibleCode
  | TypelessCardDiff
  | TypelessCardTerminal
  | TypelessCardVideo
  | TypelessCardMermaid
  | TypelessCardSearch
  | TypelessCardDeepResearch
  | TypelessCardDeepResearchProgress
  | TypelessCardDeepResearchEvent
  | TypelessCardUIReview
  | TypelessCardUIReviewProgress
  | TypelessCardMediaGenerate
  | TypelessCardAnalyzeProgress
  | TypelessCardBrowserProgress
  | TypelessCardWebFetch
  | TypelessCardConvertTask
  | TypelessCardExec

// Card parsing result
export interface ParsedContent {
  text: string
  cards: TypelessCard[]
}

// Card marker format in message content
// Cards are embedded as JSON blocks with special markers:
// ```typeless
// { "type": "info", "content": "..." }
// ```
export const TYPELESS_MARKER_START = '```typeless'
export const TYPELESS_MARKER_END = '```'
