import type { PresetQuestion } from '@/api/preview'

export const PRESET_FEED_STORAGE_KEY = 'zima.chat.try_feed.v1'
export const PRESET_FEED_PAGE_SIZE = 12
export const PRESET_FEED_MAX_LAST_SENT = 6

export const PRESET_FEED_INTERESTS = [
  'personal-knowledge',
  'learning-growth',
  'content-creation',
  'market-investing',
  'product-design',
  'user-research',
  'psychological-exploration',
  'philosophical-dialogue',
] as const

export type PresetFeedInterestId = (typeof PRESET_FEED_INTERESTS)[number]

export const PRESET_FEED_INTEREST_I18N_KEYS: Record<PresetFeedInterestId, string> = {
  'personal-knowledge': 'chat.presetQuestions.interests.personalKnowledge',
  'learning-growth': 'chat.presetQuestions.interests.learningGrowth',
  'content-creation': 'chat.presetQuestions.interests.contentCreation',
  'market-investing': 'chat.presetQuestions.interests.marketInvesting',
  'product-design': 'chat.presetQuestions.interests.productDesign',
  'user-research': 'chat.presetQuestions.interests.userResearch',
  'psychological-exploration': 'chat.presetQuestions.interests.psychologicalExploration',
  'philosophical-dialogue': 'chat.presetQuestions.interests.philosophicalDialogue',
}

export interface TryFeedState {
  selectedTags: PresetFeedInterestId[]
  clickCounts: Partial<Record<PresetFeedInterestId, number>>
  lastSentIds: string[]
}

interface RankedPresetQuestion {
  question: PresetQuestion
  baseScore: number
  primaryTag: PresetFeedInterestId | null
  recentIndex: number
  displayTitle: string
}

const PRIMARY_TAG_BONUS = 30
const SECONDARY_TAG_BONUS = 15
const CLICK_COUNT_BONUS = 8
const CLICK_COUNT_CAP = 24
const CONTEXT_MATCH_BONUS = 12
const SAME_PRIMARY_TAG_PENALTY = 20

const TAG_KEYWORDS: Record<PresetFeedInterestId, string[]> = {
  'personal-knowledge': [
    'memory',
    'timeline',
    'archive',
    'calendar',
    'email',
    'mail',
    'inbox',
    'digest',
    'knowledge base',
    'personal data',
    '记忆',
    '时间线',
    '邮件',
    '日程',
    '晨报',
    '知识库',
    '个人数据',
  ],
  'learning-growth': [
    'learn',
    'learning',
    'study',
    'reading',
    'course',
    'skill',
    'review',
    'growth',
    '学习',
    '阅读',
    '课程',
    '技能',
    '复习',
    '成长',
  ],
  'content-creation': [
    'content',
    'creator',
    'social',
    'post',
    'script',
    'youtube',
    'reddit',
    'x ',
    '创作',
    '内容',
    '选题',
    '发帖',
    '脚本',
    '社媒',
  ],
  'market-investing': [
    'market',
    'invest',
    'investing',
    'stock',
    'chip',
    'macro',
    'economy',
    'fed',
    'nvda',
    'amd',
    'asset',
    '市场',
    '投资',
    '行情',
    '芯片',
    '宏观',
    '经济',
    '美联储',
    '股价',
  ],
  'product-design': [
    'design',
    'figma',
    'responsive',
    'breakpoint',
    'inspiration',
    'moodboard',
    'dribbble',
    'behance',
    '设计',
    '适配',
    '响应式',
    '灵感',
    '情绪板',
    '视觉',
  ],
  'user-research': [
    'user',
    'comment',
    'review',
    'feedback',
    'community',
    'sentiment',
    'persona',
    '用户',
    '评论',
    '反馈',
    '社区',
    '舆情',
    '画像',
    '需求',
  ],
  'psychological-exploration': [
    'dream',
    'symbol',
    'emotion',
    'emotionally',
    'mind',
    '心理',
    '梦',
    '梦境',
    '象征',
    '情绪',
  ],
  'philosophical-dialogue': [
    'aristotle',
    'philosophy',
    'philosopher',
    'dialogue',
    'thought',
    'methodology',
    '哲学',
    '亚里士多德',
    '思想',
    '对话',
    '方法论',
  ],
}

function isPresetFeedInterestId(value: unknown): value is PresetFeedInterestId {
  return typeof value === 'string' && PRESET_FEED_INTERESTS.includes(value as PresetFeedInterestId)
}

function safeLocalStorage(): Storage | null {
  try {
    if (typeof window === 'undefined') return null
    return window.localStorage
  } catch {
    return null
  }
}

function normalizeTagList(tags: unknown[]): PresetFeedInterestId[] {
  const seen = new Set<PresetFeedInterestId>()
  const normalized: PresetFeedInterestId[] = []
  for (const tag of tags) {
    if (!isPresetFeedInterestId(tag) || seen.has(tag)) continue
    seen.add(tag)
    normalized.push(tag)
  }
  return normalized
}

function safeQuestionTitle(question: PresetQuestion): string {
  return String(question.title || question.text || question.prompt || question.id || '').trim()
}

function safeEditorialScore(question: PresetQuestion): number {
  return Number.isFinite(question.editorial_score) ? Number(question.editorial_score) : 0
}

export function createDefaultTryFeedState(): TryFeedState {
  return {
    selectedTags: [],
    clickCounts: {},
    lastSentIds: [],
  }
}

export function normalizeTryFeedState(value: unknown): TryFeedState {
  const fallback = createDefaultTryFeedState()
  if (!value || typeof value !== 'object') return fallback

  const raw = value as {
    selectedTags?: unknown[]
    clickCounts?: Record<string, unknown>
    lastSentIds?: unknown[]
  }

  const clickCounts: Partial<Record<PresetFeedInterestId, number>> = {}
  for (const interest of PRESET_FEED_INTERESTS) {
    const rawValue = raw.clickCounts?.[interest]
    if (typeof rawValue !== 'number' || !Number.isFinite(rawValue) || rawValue <= 0) continue
    clickCounts[interest] = Math.max(0, Math.floor(rawValue))
  }

  return {
    selectedTags: normalizeTagList(Array.isArray(raw.selectedTags) ? raw.selectedTags : []),
    clickCounts,
    lastSentIds: Array.isArray(raw.lastSentIds)
      ? raw.lastSentIds.filter((id): id is string => typeof id === 'string' && id.trim().length > 0)
      : [],
  }
}

export function loadTryFeedState(storage = safeLocalStorage()): TryFeedState {
  if (!storage) return createDefaultTryFeedState()
  try {
    const raw = storage.getItem(PRESET_FEED_STORAGE_KEY)
    if (!raw) return createDefaultTryFeedState()
    return normalizeTryFeedState(JSON.parse(raw))
  } catch {
    return createDefaultTryFeedState()
  }
}

export function saveTryFeedState(state: TryFeedState, storage = safeLocalStorage()) {
  if (!storage) return
  try {
    storage.setItem(PRESET_FEED_STORAGE_KEY, JSON.stringify(normalizeTryFeedState(state)))
  } catch {
    // Ignore storage errors
  }
}

export function getPresetQuestionTags(question: PresetQuestion): PresetFeedInterestId[] {
  return normalizeTagList([question.category, ...(question.tags || [])])
}

export function getPresetQuestionPrimaryTag(question: PresetQuestion): PresetFeedInterestId | null {
  return getPresetQuestionTags(question)[0] ?? null
}

function hasContextMatch(contextText: string, tags: PresetFeedInterestId[]): boolean {
  const normalizedContext = contextText.trim().toLowerCase()
  if (!normalizedContext) return false
  return tags.some((tag) =>
    TAG_KEYWORDS[tag].some((keyword) => normalizedContext.includes(keyword.toLowerCase()))
  )
}

function buildBaseScore(
  question: PresetQuestion,
  selectedTags: Set<PresetFeedInterestId>,
  clickCounts: Partial<Record<PresetFeedInterestId, number>>,
  contextText: string
): number {
  const tags = getPresetQuestionTags(question)
  let score = safeEditorialScore(question)

  tags.forEach((tag, index) => {
    if (!selectedTags.has(tag)) return
    score += index === 0 ? PRIMARY_TAG_BONUS : SECONDARY_TAG_BONUS
  })

  const clickBonus = Math.min(
    tags.reduce((sum, tag) => sum + (clickCounts[tag] || 0) * CLICK_COUNT_BONUS, 0),
    CLICK_COUNT_CAP
  )
  score += clickBonus

  if (hasContextMatch(contextText, tags)) {
    score += CONTEXT_MATCH_BONUS
  }

  return score
}

function compareRankedQuestions(a: RankedPresetQuestion, b: RankedPresetQuestion): number {
  if (a.baseScore !== b.baseScore) return b.baseScore - a.baseScore

  const aIsRecent = a.recentIndex >= 0
  const bIsRecent = b.recentIndex >= 0
  if (aIsRecent !== bIsRecent) return aIsRecent ? 1 : -1
  if (a.recentIndex !== b.recentIndex) {
    return b.recentIndex - a.recentIndex
  }

  const aEditorial = safeEditorialScore(a.question)
  const bEditorial = safeEditorialScore(b.question)
  if (aEditorial !== bEditorial) return bEditorial - aEditorial

  return a.displayTitle.localeCompare(b.displayTitle, 'en')
}

export function rankPresetQuestions(
  questions: PresetQuestion[],
  options: {
    contextText?: string
    state?: TryFeedState
  } = {}
): PresetQuestion[] {
  const state = normalizeTryFeedState(options.state ?? createDefaultTryFeedState())
  const selectedTags = new Set(state.selectedTags)
  const ranked: RankedPresetQuestion[] = questions.map((question) => ({
    question,
    baseScore: buildBaseScore(question, selectedTags, state.clickCounts, options.contextText ?? ''),
    primaryTag: getPresetQuestionPrimaryTag(question),
    recentIndex: state.lastSentIds.indexOf(question.id),
    displayTitle: safeQuestionTitle(question),
  }))

  ranked.sort(compareRankedQuestions)

  const ordered: RankedPresetQuestion[] = []
  const remaining = [...ranked]

  while (remaining.length > 0) {
    let bestIndex = 0
    let bestScore = Number.NEGATIVE_INFINITY

    for (let index = 0; index < remaining.length; index += 1) {
      const candidate = remaining[index]
      if (!candidate) continue

      let effectiveScore = candidate.baseScore
      const previous = ordered[ordered.length - 1]
      if (previous && previous.primaryTag && previous.primaryTag === candidate.primaryTag) {
        effectiveScore -= SAME_PRIMARY_TAG_PENALTY
      }

      if (effectiveScore > bestScore) {
        bestScore = effectiveScore
        bestIndex = index
        continue
      }

      const currentBest = remaining[bestIndex]
      if (effectiveScore === bestScore && currentBest && compareRankedQuestions(candidate, currentBest) < 0) {
        bestIndex = index
      }
    }

    const [nextQuestion] = remaining.splice(bestIndex, 1)
    if (!nextQuestion) break
    ordered.push(nextQuestion)
  }

  return ordered.map((entry) => entry.question)
}

export function recordPresetQuestionSend(state: TryFeedState, question: PresetQuestion): TryFeedState {
  const normalized = normalizeTryFeedState(state)
  const nextClickCounts = { ...normalized.clickCounts }

  for (const tag of getPresetQuestionTags(question)) {
    nextClickCounts[tag] = (nextClickCounts[tag] || 0) + 1
  }

  return normalizeTryFeedState({
    selectedTags: normalized.selectedTags,
    clickCounts: nextClickCounts,
    lastSentIds: [question.id, ...normalized.lastSentIds.filter((id) => id !== question.id)].slice(
      0,
      PRESET_FEED_MAX_LAST_SENT
    ),
  })
}
