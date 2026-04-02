import type { LocaleKey } from './locale-catalog'

type LocaleLeaf = string | number | boolean | null
type LocaleNode = { [key: string]: LocaleLeaf | LocaleNode }

const skillStoreLabelBackfills: Record<
  LocaleKey,
  {
    popular: string
    recent: string
    sortBy: string
  }
> = {
  'ca-ES': {
    popular: 'Populars',
    recent: 'Recents',
    sortBy: 'Ordena per',
  },
  'cs-CZ': {
    popular: 'Populární',
    recent: 'Nedávné',
    sortBy: 'Seřadit podle',
  },
  'da-DK': {
    popular: 'Populære',
    recent: 'Seneste',
    sortBy: 'Sorter efter',
  },
  'de-DE': {
    popular: 'Beliebt',
    recent: 'Neu',
    sortBy: 'Sortieren nach',
  },
  'el-GR': {
    popular: 'Δημοφιλή',
    recent: 'Πρόσφατα',
    sortBy: 'Ταξινόμηση κατά',
  },
  'en-GB': {
    popular: 'Popular',
    recent: 'Recent',
    sortBy: 'Sort by',
  },
  'en-US': {
    popular: 'Popular',
    recent: 'Recent',
    sortBy: 'Sort by',
  },
  'es-ES': {
    popular: 'Populares',
    recent: 'Recientes',
    sortBy: 'Ordenar por',
  },
  'fr-FR': {
    popular: 'Populaires',
    recent: 'Récents',
    sortBy: 'Trier par',
  },
  'ga-IE': {
    popular: 'Coitianta',
    recent: 'Le déanaí',
    sortBy: 'Sórtáil de réir',
  },
  'hr-HR': {
    popular: 'Popularno',
    recent: 'Nedavno',
    sortBy: 'Poredaj po',
  },
  'hu-HU': {
    popular: 'Népszerű',
    recent: 'Legutóbbi',
    sortBy: 'Rendezés',
  },
  'it-IT': {
    popular: 'Popolari',
    recent: 'Recenti',
    sortBy: 'Ordina per',
  },
  'ja-JP': {
    popular: '人気',
    recent: '最新',
    sortBy: '並び替え',
  },
  'ko-KR': {
    popular: '인기',
    recent: '최신',
    sortBy: '정렬',
  },
  'ml-IN': {
    popular: 'ജനപ്രിയം',
    recent: 'സമീപകാലം',
    sortBy: 'ക്രമപ്പെടുത്തല്‍',
  },
  'nb-NO': {
    popular: 'Populære',
    recent: 'Nylige',
    sortBy: 'Sorter etter',
  },
  'nl-NL': {
    popular: 'Populair',
    recent: 'Recent',
    sortBy: 'Sorteren op',
  },
  'pl-PL': {
    popular: 'Popularne',
    recent: 'Najnowsze',
    sortBy: 'Sortuj według',
  },
  'pt-BR': {
    popular: 'Populares',
    recent: 'Recentes',
    sortBy: 'Ordenar por',
  },
  'pt-PT': {
    popular: 'Populares',
    recent: 'Recentes',
    sortBy: 'Ordenar por',
  },
  'ro-RO': {
    popular: 'Populare',
    recent: 'Recente',
    sortBy: 'Sortează după',
  },
  'ru-RU': {
    popular: 'Популярные',
    recent: 'Недавние',
    sortBy: 'Сортировать по',
  },
  'sk-SK': {
    popular: 'Obľúbené',
    recent: 'Nedávne',
    sortBy: 'Zoradiť podľa',
  },
  'sv-SE': {
    popular: 'Populära',
    recent: 'Senaste',
    sortBy: 'Sortera efter',
  },
  'zh-CN': {
    popular: '热门',
    recent: '最新',
    sortBy: '排序',
  },
  'zh-TW': {
    popular: '熱門',
    recent: '最新',
    sortBy: '排序',
  },
}

function isPlainObject(value: unknown): value is LocaleNode {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

function getString(root: unknown, path: string): string | null {
  const value = path.split('.').reduce<unknown>((current, segment) => {
    if (isPlainObject(current)) {
      return current[segment]
    }
    return undefined
  }, root)

  return typeof value === 'string' && value.length > 0 ? value : null
}

function hasKeys(node: LocaleNode): boolean {
  return Object.keys(node).length > 0
}

export function buildSkillStoreDerivedBackfill(
  localeKey: LocaleKey,
  messages: Record<string, unknown>
): LocaleNode {
  const labels = skillStoreLabelBackfills[localeKey]
  const statsPatch: LocaleNode = {}
  const tabsPatch: LocaleNode = {}
  const searchPatch: LocaleNode = {}
  const actionsPatch: LocaleNode = {}
  const detailMetaPatch: LocaleNode = {}

  const mirrors: Array<[LocaleNode, string, string | null]> = [
    [statsPatch, 'downloads', getString(messages, 'skillStore.downloads')],
    [statsPatch, 'reviews', getString(messages, 'skillStore.reviews')],
    [statsPatch, 'rating', getString(messages, 'skillStore.rating')],
    [statsPatch, 'stars', getString(messages, 'skillStore.stars')],
    [searchPatch, 'clearSearch', getString(messages, 'skillStore.clearSearch')],
    [searchPatch, 'sortRelevance', getString(messages, 'chat.deepResearchRelevance')],
    [searchPatch, 'sortDownloads', getString(messages, 'skillStore.downloads')],
    [searchPatch, 'sortStars', getString(messages, 'skillStore.sort.stars') ?? getString(messages, 'skillStore.stars')],
    [searchPatch, 'sortRecent', getString(messages, 'skillStore.sort.updated')],
    [searchPatch, 'sortName', getString(messages, 'common.name')],
    [actionsPatch, 'loading', getString(messages, 'common.loading')],
    [detailMetaPatch, 'author', getString(messages, 'skills.detail.labels.author')],
    [detailMetaPatch, 'version', getString(messages, 'skills.detail.labels.version') ?? getString(messages, 'system.version')],
    [detailMetaPatch, 'updated', getString(messages, 'skillStore.sort.updated')],
    [detailMetaPatch, 'downloads', getString(messages, 'skillStore.downloads')],
    [detailMetaPatch, 'stars', getString(messages, 'skillStore.stars')],
    [detailMetaPatch, 'rating', getString(messages, 'skillStore.rating')],
  ]

  for (const [target, key, value] of mirrors) {
    if (value) {
      target[key] = value
    }
  }

  tabsPatch.popular = labels.popular
  tabsPatch.recent = labels.recent
  searchPatch.sortBy = labels.sortBy

  const skillStorePatch: LocaleNode = {}
  if (hasKeys(statsPatch)) {
    skillStorePatch.stats = statsPatch
  }
  if (hasKeys(tabsPatch)) {
    skillStorePatch.tabs = tabsPatch
  }
  if (hasKeys(searchPatch)) {
    skillStorePatch.search = searchPatch
  }
  if (hasKeys(actionsPatch)) {
    skillStorePatch.actions = actionsPatch
  }
  if (hasKeys(detailMetaPatch)) {
    skillStorePatch.detail = { meta: detailMetaPatch }
  }

  return hasKeys(skillStorePatch) ? { skillStore: skillStorePatch } : {}
}
