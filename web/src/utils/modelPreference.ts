const modelVersionNumberRe = /\d+/g
const preferredModelPriorityThreshold = 100

function normalizeModelPreferenceID(modelID: string): string {
  const trimmed = modelID.trim().toLowerCase()
  if (!trimmed) return ''

  const slashIndex = trimmed.lastIndexOf('/')
  if (slashIndex >= 0 && slashIndex < trimmed.length - 1) {
    return trimmed.slice(slashIndex + 1)
  }

  return trimmed
}

function preferredModelPrefixRank(modelID: string): number {
  const normalized = normalizeModelPreferenceID(modelID)
  if (normalized.startsWith('claude') || normalized.startsWith('gpt')) {
    return 0
  }
  return 1
}

function modelVersionScore(modelID: string): number {
  const matches = normalizeModelPreferenceID(modelID).match(modelVersionNumberRe)
  if (!matches) return 0

  const weights = [100, 10, 1]
  let total = 0
  let weightIndex = 0

  for (const match of matches) {
    if (weightIndex >= weights.length) break

    const value = Number.parseInt(match, 10)
    if (Number.isNaN(value) || value >= 1000) continue

    const weight = weights[weightIndex]
    if (weight === undefined) break
    total += value * weight
    weightIndex += 1
  }

  return total
}

function modelIntelligenceScore(modelID: string): number {
  const normalized = normalizeModelPreferenceID(modelID)
  if (!normalized) return 0

  let score = modelVersionScore(normalized)

  const boosts: Array<[string, number]> = [
    ['gpt-5', 520],
    ['o3', 480],
    ['o1', 430],
    ['opus', 420],
    ['reasoner', 380],
    ['thinking', 360],
    ['sonnet', 320],
    ['pro', 220],
    ['max', 210],
    ['ultra', 180],
    ['plus', 120],
    ['codex', 95],
    ['coder', 60],
    ['codestral', 60],
    ['turbo', 50],
    ['haiku', 40],
  ]
  for (const [key, value] of boosts) {
    if (normalized.includes(key)) {
      score += value
    }
  }

  const penalties: Array<[string, number]> = [
    ['mini', -180],
    ['nano', -200],
    ['lite', -150],
    ['flash', -130],
    ['spark', -120],
    ['small', -100],
    ['tiny', -100],
  ]
  for (const [key, value] of penalties) {
    if (normalized.includes(key)) {
      score += value
    }
  }

  return score
}

export function compareModelPreference(left: string, right: string): number {
  const leftPrefixRank = preferredModelPrefixRank(left)
  const rightPrefixRank = preferredModelPrefixRank(right)
  if (leftPrefixRank !== rightPrefixRank) {
    return leftPrefixRank - rightPrefixRank
  }

  const leftScore = modelIntelligenceScore(left)
  const rightScore = modelIntelligenceScore(right)
  if (leftScore !== rightScore) {
    return rightScore - leftScore
  }

  return normalizeModelPreferenceID(left).localeCompare(normalizeModelPreferenceID(right))
}

export function shouldPrioritizePreferredModels(total: number): boolean {
  return total >= preferredModelPriorityThreshold
}

export function sortItemsByModelPreference<T>(
  items: readonly T[],
  getID: (item: T) => string,
  tieBreak?: (left: T, right: T) => number
): T[] {
  const out = [...items]
  if (!shouldPrioritizePreferredModels(out.length)) {
    return out
  }

  return out.sort((left, right) => {
    const byModel = compareModelPreference(getID(left), getID(right))
    if (byModel !== 0) return byModel
    return tieBreak ? tieBreak(left, right) : 0
  })
}
