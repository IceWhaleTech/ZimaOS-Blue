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

export interface TypelessCardBase {
  type: TypelessCardType
  id?: string
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
}

// Result Card - Display operation results
export interface TypelessCardResult extends TypelessCardBase {
  type: 'result'
  title: string
  status: 'success' | 'error' | 'warning' | 'info'
  message?: string
  details?: ResultDetail[]
  actions?: ActionButton[]
}

export interface ResultDetail {
  label: string
  value: string
  copyable?: boolean
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
  rows: (string | number)[][]
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
