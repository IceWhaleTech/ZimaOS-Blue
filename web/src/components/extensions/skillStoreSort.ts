export type SkillStoreSortMode = 'trending' | 'newest' | 'most_used' | 'featured'

const skillStoreSortTranslationKeys: Record<SkillStoreSortMode, string> = {
  featured: 'featured',
  trending: 'trending',
  newest: 'newest',
  most_used: 'mostUsed',
}

export function skillStoreSortTranslationPath(mode: SkillStoreSortMode): string {
  return `sort.${skillStoreSortTranslationKeys[mode]}`
}
