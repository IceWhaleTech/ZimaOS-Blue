export type MarketplaceSourceField = 'label' | 'description'
import { publicAsset } from '@/utils/publicAsset'

export type MarketplaceSourceTranslate = (
  path: string,
  fallback: string,
  params?: Record<string, unknown>
) => string

export interface MarketplaceSourceOptionLike {
  value?: string | null
  label?: string | null
}

export interface MarketplaceSourceBrand {
  iconUrl?: string
  logoUrl?: string
  logoDarkUrl?: string
}

type MarketplaceSourceCatalogEntry = {
  label: string
  description: string
  iconUrl?: string
  logoUrl?: string
  logoDarkUrl?: string
}

const sourceLocaleCatalog = {
  skillhub: {
    label: 'SkillHub',
    description: 'Aggregated SkillHub catalogs from Tencent SkillHub and SkillHub Club.',
    iconUrl: 'https://www.skillhub.club/favicon-48x48.png',
  },
  tencentSkillHub: {
    label: 'Tencent SkillHub',
    description: 'Official Tencent SkillHub catalog feed.',
    // Lightmake does not expose a stable public favicon during probing, so reuse the shared SkillHub mark.
    iconUrl: 'https://www.skillhub.club/favicon-48x48.png',
  },
  skillhubClub: {
    label: 'SkillHub Club',
    description: 'Community SkillHub catalog and mirror site.',
    iconUrl: 'https://www.skillhub.club/favicon-48x48.png',
  },
  github: {
    label: 'GitHub',
    description: 'GitHub repository search sources for SKILL.md files.',
    iconUrl: 'https://github.githubassets.com/favicons/favicon.svg',
  },
  githubAwesomeSkills: {
    label: 'GitHub Awesome Skills',
    description: 'Aggregated curated GitHub awesome skills upstreams.',
    iconUrl: 'https://github.githubassets.com/favicons/favicon.svg',
  },
  vercel: {
    label: 'Vercel',
    description: 'Official Vercel skills collection from vercel-labs/skills.',
    iconUrl: publicAsset('icons/providers/vercel.svg'),
    logoUrl: publicAsset('icons/providers/vercel-wordmark.svg'),
    logoDarkUrl: publicAsset('icons/providers/vercel-wordmark-dark.svg'),
  },
  githubSkillMd: {
    label: 'GitHub SKILL.md',
    description: 'GitHub code search for repositories that publish SKILL.md.',
    iconUrl: 'https://github.githubassets.com/favicons/favicon.svg',
  },
  clawhub: {
    label: 'ClawHub',
    description: 'ClawHub marketplace catalog.',
    iconUrl: 'https://clawhub.ai/favicon.ico',
  },
  clawhubMirror: {
    label: 'ClawHub Mirror',
    description: 'Mirror endpoint for the ClawHub catalog.',
    iconUrl: 'https://clawhub.ai/favicon.ico',
  },
  llmskills: {
    label: 'LLMSkills',
    description: 'LLMSkills community marketplace catalog.',
    iconUrl: 'https://llmskills.org/logo.png',
    logoUrl: 'https://llmskills.org/logo.png',
  },
  minimax: {
    label: 'MiniMax',
    description: 'MiniMax GitHub skill collection.',
    iconUrl: publicAsset('icons/providers/minimax.svg'),
    logoUrl: publicAsset('icons/providers/minimax.svg'),
  },
  agentskills: {
    label: 'AgentSkills',
    description: 'Public agent skill marketplace with HTML-embedded catalog data.',
    iconUrl: 'https://agentskills.to/favicon.svg',
  },
  skillmd: {
    label: 'SkillMD',
    description: 'SKILL.md registry with public skill snapshots and rendered detail pages.',
    iconUrl: 'https://skillmd.io/favicon.png',
    logoUrl: 'https://skillmd.io/logo.png',
    logoDarkUrl: 'https://skillmd.io/logo-dark.png',
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
} satisfies Record<string, MarketplaceSourceCatalogEntry>

type MarketplaceSourceKey = keyof typeof sourceLocaleCatalog

const sourceLocaleAliases: Record<string, MarketplaceSourceKey> = {
  skillhub: 'skillhub',
  'tencent-skillhub': 'tencentSkillHub',
  'tencent skillhub': 'tencentSkillHub',
  'skillhub-club': 'skillhubClub',
  'skillhub club': 'skillhubClub',
  github: 'github',
  'github-awesome-skills': 'githubAwesomeSkills',
  'github awesome skills': 'githubAwesomeSkills',
  vercel: 'vercel',
  'vercel-labs': 'vercel',
  'vercel labs': 'vercel',
  'vercel-labs/skills': 'vercel',
  'github-skill-md': 'githubSkillMd',
  'github skill.md': 'githubSkillMd',
  clawhub: 'clawhub',
  'clawhub-mirror': 'clawhubMirror',
  'clawhub mirror': 'clawhubMirror',
  llmskills: 'llmskills',
  minimax: 'minimax',
  'minimax-ai': 'minimax',
  'minimax ai': 'minimax',
  agentskills: 'agentskills',
  'agentskills.to': 'agentskills',
  skillmd: 'skillmd',
  'skillmd.io': 'skillmd',
  external: 'external',
  'external skill': 'curatedSkillUrl',
  'curated-skill-url': 'curatedSkillUrl',
  'curated github seed': 'curatedGithubSeed',
  'curated-github-seed': 'curatedGithubSeed',
  seed: 'seed',
} satisfies Record<string, MarketplaceSourceKey>

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

  const seedMatch = normalized.match(/^(?:seed|discovery(?:[- ]+(?:page|source))?)(?:[- ]+)?(\d+)$/)
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

  if (normalized.includes('github.com/minimax-ai')) {
    return { key: 'minimax' }
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
  return localizeMarketplaceSourceFromCandidates(
    [option?.value, option?.label],
    'description',
    translate
  )
}

export function resolveMarketplaceSourceBrand(
  value?: string | null
): MarketplaceSourceBrand | null {
  const resolved = resolveMarketplaceSourceLocale(value)
  if (!resolved) return null
  const entry = sourceLocaleCatalog[resolved.key] as MarketplaceSourceCatalogEntry
  if (!entry.iconUrl && !entry.logoUrl && !entry.logoDarkUrl) return null
  return {
    iconUrl: entry.iconUrl,
    logoUrl: entry.logoUrl,
    logoDarkUrl: entry.logoDarkUrl,
  }
}

export function resolveMarketplaceSourceBrandFromCandidates(
  candidates: Array<string | null | undefined>
): MarketplaceSourceBrand | null {
  for (const candidate of candidates) {
    const brand = resolveMarketplaceSourceBrand(candidate)
    if (brand) return brand
  }
  return null
}
