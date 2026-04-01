function normalizeModelPreferenceID(modelID: string): string {
  const trimmed = modelID.trim().toLowerCase()
  if (!trimmed) return ''

  const slashIndex = trimmed.lastIndexOf('/')
  if (slashIndex >= 0 && slashIndex < trimmed.length - 1) {
    return trimmed.slice(slashIndex + 1)
  }

  return trimmed
}

export function compareModelPreference(left: string, right: string): number {
  const normalizedLeft = normalizeModelPreferenceID(left)
  const normalizedRight = normalizeModelPreferenceID(right)
  const byNormalized = normalizedRight.localeCompare(normalizedLeft)
  if (byNormalized !== 0) return byNormalized
  return right.trim().toLowerCase().localeCompare(left.trim().toLowerCase())
}

export function shouldPrioritizePreferredModels(_total: number): boolean {
  return true
}

export function sortItemsByModelPreference<T>(
  items: readonly T[],
  getID: (item: T) => string,
  tieBreak?: (left: T, right: T) => number
): T[] {
  return [...items].sort((left, right) => {
    const byModel = compareModelPreference(getID(left), getID(right))
    if (byModel !== 0) return byModel
    return tieBreak ? tieBreak(left, right) : 0
  })
}
