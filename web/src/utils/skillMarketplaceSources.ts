export type MarketplaceSourceField = 'label' | 'description'

export type MarketplaceSourceTranslate = (
  path: string,
  fallback: string,
  params?: Record<string, unknown>
) => string

export interface MarketplaceSourceOptionLike {
  value?: string | null
  label?: string | null
}

const sourceLocaleCatalog = {
  skillhub: {
    label: 'SkillHub',
    description: 'Aggregated SkillHub catalogs from Tencent SkillHub and SkillHub Club.',
  },
  tencentSkillHub: {
    label: 'Tencent SkillHub',
    description: 'Official Tencent SkillHub catalog feed.',
  },
  skillhubClub: {
    label: 'SkillHub Club',
    description: 'Community SkillHub catalog and mirror site.',
  },
  github: {
    label: 'GitHub',
    description:
      'GitHub repository search sources for SKILL.md, CLAUDE.md, and AGENT.md files.',
  },
  githubSkillMd: {
    label: 'GitHub SKILL.md',
    description: 'GitHub code search for repositories that publish SKILL.md.',
  },
  githubClaudeMd: {
    label: 'GitHub CLAUDE.md',
    description: 'GitHub code search for repositories that publish CLAUDE.md.',
  },
  githubAgentMd: {
    label: 'GitHub AGENT.md',
    description: 'GitHub code search for repositories that publish AGENT.md.',
  },
  clawhub: {
    label: 'ClawHub',
    description: 'ClawHub marketplace catalog.',
  },
  clawhubMirror: {
    label: 'ClawHub Mirror',
    description: 'Mirror endpoint for the ClawHub catalog.',
  },
  skillstack: {
    label: 'SkillStack',
    description: 'Community catalog focused on reusable skill collections.',
  },
  skillsmp: {
    label: 'SkillsMP',
    description: 'SkillsMP marketplace catalog.',
  },
  llmskills: {
    label: 'LLMSkills',
    description: 'LLMSkills community marketplace catalog.',
  },
  external: {
    label: 'External Sources',
    description: 'Curated skill records imported from external URLs.',
  },
  curatedSkillUrl: {
    label: 'External Skill',
    description: 'Curated marketplace entry imported from a direct skill URL.',
  },
  curatedGithubSeed: {
    label: 'Curated GitHub Sources',
    description: 'Curated GitHub source list imported from GitHub references.',
  },
  seed: {
    label: 'Discovery Pages',
    description: 'Discovery pages that link to additional skills found on the web.',
  },
  seedInstance: {
    label: 'Discovery Page {index}',
    description: 'Discovery page {index} that links to additional skills found on the web.',
  },
} as const

type MarketplaceSourceKey = keyof typeof sourceLocaleCatalog

const sourceLocaleAliases: Record<string, MarketplaceSourceKey> = {
  skillhub: 'skillhub',
  'tencent-skillhub': 'tencentSkillHub',
  'tencent skillhub': 'tencentSkillHub',
  'skillhub-club': 'skillhubClub',
  'skillhub club': 'skillhubClub',
  github: 'github',
  'github-skill-md': 'githubSkillMd',
  'github skill.md': 'githubSkillMd',
  'github-claude-md': 'githubClaudeMd',
  'github claude.md': 'githubClaudeMd',
  'github-agent-md': 'githubAgentMd',
  'github agent.md': 'githubAgentMd',
  clawhub: 'clawhub',
  'clawhub-mirror': 'clawhubMirror',
  'clawhub mirror': 'clawhubMirror',
  skillstack: 'skillstack',
  skillsmp: 'skillsmp',
  llmskills: 'llmskills',
  external: 'external',
  'external skill': 'curatedSkillUrl',
  'curated-skill-url': 'curatedSkillUrl',
  'curated github seed': 'curatedGithubSeed',
  'curated-github-seed': 'curatedGithubSeed',
  seed: 'seed',
}

export function trimMarketplaceSourceToken(value?: string | null): string {
  return value?.trim() || ''
}

export function firstMarketplaceSourceCandidate(
  candidates: Array<string | null | undefined>
): string {
  return candidates.find((candidate) => candidate?.trim())?.trim() || ''
}

function resolveMarketplaceSourceLocale(
  value?: string | null
): { key: MarketplaceSourceKey; params?: Record<string, unknown> } | null {
  const trimmed = trimMarketplaceSourceToken(value)
  if (!trimmed) return null
  const normalized = trimmed.toLowerCase()

  const seedMatch = normalized.match(
    /^(?:seed|discovery(?:[- ]+(?:page|source))?)(?:[- ]+)?(\d+)$/
  )
  if (seedMatch) {
    return {
      key: 'seedInstance',
      params: { index: Number(seedMatch[1]) || seedMatch[1] },
    }
  }

  const mirrorMatch = normalized.match(/^clawhub[- ]mirror(?:[- ]+(\d+))?$/)
  if (mirrorMatch) {
    return {
      key: 'clawhubMirror',
      params: mirrorMatch[1] ? { index: Number(mirrorMatch[1]) || mirrorMatch[1] } : undefined,
    }
  }

  const key = sourceLocaleAliases[normalized]
  return key ? { key } : null
}

export function localizeMarketplaceSource(
  value: string | null | undefined,
  field: MarketplaceSourceField,
  translate: MarketplaceSourceTranslate
): string {
  const resolved = resolveMarketplaceSourceLocale(value)
  if (!resolved) return ''
  const fallback = sourceLocaleCatalog[resolved.key][field]
  return translate(`sources.${resolved.key}.${field}`, fallback, resolved.params)
}

export function localizeMarketplaceSourceFromCandidates(
  candidates: Array<string | null | undefined>,
  field: MarketplaceSourceField,
  translate: MarketplaceSourceTranslate
): string {
  for (const candidate of candidates) {
    const localized = localizeMarketplaceSource(candidate, field, translate)
    if (localized) return localized
  }
  return ''
}

export function localizeMarketplaceSourceOptionLabel(
  option: MarketplaceSourceOptionLike | null | undefined,
  translate: MarketplaceSourceTranslate,
  fallbackLabel: string
): string {
  return (
    localizeMarketplaceSourceFromCandidates([option?.value, option?.label], 'label', translate) ||
    firstMarketplaceSourceCandidate([option?.label, option?.value]) ||
    fallbackLabel
  )
}

export function localizeMarketplaceSourceOptionDescription(
  option: MarketplaceSourceOptionLike | null | undefined,
  translate: MarketplaceSourceTranslate
): string {
  return localizeMarketplaceSourceFromCandidates([option?.value, option?.label], 'description', translate)
}
