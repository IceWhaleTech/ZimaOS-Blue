import type { LocaleKey } from './locale-catalog'
import skillStoreFollowupLabels from './skill-store-followup-labels'
import skillStoreSourceImportBackfills from './skill-store-source-import-backfills'
import skillStoreTailLabels from './skill-store-tail-labels'

type LocaleLeaf = string | number | boolean | null
type LocaleNode = { [key: string]: LocaleLeaf | LocaleNode }

const skillStoreLabelBackfills: Record<
  LocaleKey,
  {
    changelog: string
    focus: string
    popular: string
    recent: string
    skillsMarketplace: string
    sortBy: string
  }
> = {
  'ca-ES': {
    changelog: 'Registre de canvis',
    focus: 'Focus',
    popular: 'Populars',
    recent: 'Recents',
    skillsMarketplace: "Mercat d'habilitats",
    sortBy: 'Ordena per',
  },
  'cs-CZ': {
    changelog: 'Změny',
    focus: 'Zaměření',
    popular: 'Populární',
    recent: 'Nedávné',
    skillsMarketplace: 'Tržiště dovedností',
    sortBy: 'Seřadit podle',
  },
  'da-DK': {
    changelog: 'Ændringslog',
    focus: 'Fokus',
    popular: 'Populære',
    recent: 'Seneste',
    skillsMarketplace: 'Færdighedsmarked',
    sortBy: 'Sorter efter',
  },
  'de-DE': {
    changelog: 'Änderungsprotokoll',
    focus: 'Fokus',
    popular: 'Beliebt',
    recent: 'Neu',
    skillsMarketplace: 'Skill-Marktplatz',
    sortBy: 'Sortieren nach',
  },
  'el-GR': {
    changelog: 'Αρχείο αλλαγών',
    focus: 'Εστίαση',
    popular: 'Δημοφιλή',
    recent: 'Πρόσφατα',
    skillsMarketplace: 'Αγορά δεξιοτήτων',
    sortBy: 'Ταξινόμηση κατά',
  },
  'en-GB': {
    changelog: 'Changelog',
    focus: 'Focus',
    popular: 'Popular',
    recent: 'Recent',
    skillsMarketplace: 'Skills marketplace',
    sortBy: 'Sort by',
  },
  'en-US': {
    changelog: 'Changelog',
    focus: 'Focus',
    popular: 'Popular',
    recent: 'Recent',
    skillsMarketplace: 'Skills marketplace',
    sortBy: 'Sort by',
  },
  'es-ES': {
    changelog: 'Registro de cambios',
    focus: 'Enfoque',
    popular: 'Populares',
    recent: 'Recientes',
    skillsMarketplace: 'Mercado de habilidades',
    sortBy: 'Ordenar por',
  },
  'fr-FR': {
    changelog: 'Journal des modifications',
    focus: 'Focus',
    popular: 'Populaires',
    recent: 'Récents',
    skillsMarketplace: 'Marché des compétences',
    sortBy: 'Trier par',
  },
  'ga-IE': {
    changelog: 'Loga athruithe',
    focus: 'Fócas',
    popular: 'Coitianta',
    recent: 'Le déanaí',
    skillsMarketplace: 'Margadh scileanna',
    sortBy: 'Sórtáil de réir',
  },
  'hr-HR': {
    changelog: 'Zapis promjena',
    focus: 'Fokus',
    popular: 'Popularno',
    recent: 'Nedavno',
    skillsMarketplace: 'Tržište vještina',
    sortBy: 'Poredaj po',
  },
  'hu-HU': {
    changelog: 'Változásnapló',
    focus: 'Fókusz',
    popular: 'Népszerű',
    recent: 'Legutóbbi',
    skillsMarketplace: 'Készség piactér',
    sortBy: 'Rendezés',
  },
  'it-IT': {
    changelog: 'Registro modifiche',
    focus: 'Focus',
    popular: 'Popolari',
    recent: 'Recenti',
    skillsMarketplace: 'Marketplace delle skill',
    sortBy: 'Ordina per',
  },
  'ja-JP': {
    changelog: '変更履歴',
    focus: '注目',
    popular: '人気',
    recent: '最新',
    skillsMarketplace: 'スキルマーケットプレイス',
    sortBy: '並び替え',
  },
  'ko-KR': {
    changelog: '변경 기록',
    focus: '포커스',
    popular: '인기',
    recent: '최신',
    skillsMarketplace: '스킬 마켓플레이스',
    sortBy: '정렬',
  },
  'ml-IN': {
    changelog: 'മാറ്റപ്പട്ടിക',
    focus: 'ഫോക്കസ്',
    popular: 'ജനപ്രിയം',
    recent: 'സമീപകാലം',
    skillsMarketplace: 'സ്കിൽ മാർക്കറ്റ്പ്ലേസ്',
    sortBy: 'ക്രമപ്പെടുത്തല്‍',
  },
  'nb-NO': {
    changelog: 'Endringslogg',
    focus: 'Fokus',
    popular: 'Populære',
    recent: 'Nylige',
    skillsMarketplace: 'Ferdighetsmarked',
    sortBy: 'Sorter etter',
  },
  'nl-NL': {
    changelog: 'Wijzigingslog',
    focus: 'Focus',
    popular: 'Populair',
    recent: 'Recent',
    skillsMarketplace: 'Vaardighedenmarktplaats',
    sortBy: 'Sorteren op',
  },
  'pl-PL': {
    changelog: 'Dziennik zmian',
    focus: 'Fokus',
    popular: 'Popularne',
    recent: 'Najnowsze',
    skillsMarketplace: 'Rynek umiejętności',
    sortBy: 'Sortuj według',
  },
  'pt-BR': {
    changelog: 'Registro de alterações',
    focus: 'Foco',
    popular: 'Populares',
    recent: 'Recentes',
    skillsMarketplace: 'Marketplace de skills',
    sortBy: 'Ordenar por',
  },
  'pt-PT': {
    changelog: 'Registo de alterações',
    focus: 'Foco',
    popular: 'Populares',
    recent: 'Recentes',
    skillsMarketplace: 'Marketplace de skills',
    sortBy: 'Ordenar por',
  },
  'ro-RO': {
    changelog: 'Jurnal de modificări',
    focus: 'Focus',
    popular: 'Populare',
    recent: 'Recente',
    skillsMarketplace: 'Piața de abilități',
    sortBy: 'Sortează după',
  },
  'ru-RU': {
    changelog: 'Журнал изменений',
    focus: 'Фокус',
    popular: 'Популярные',
    recent: 'Недавние',
    skillsMarketplace: 'Рынок навыков',
    sortBy: 'Сортировать по',
  },
  'sk-SK': {
    changelog: 'Zoznam zmien',
    focus: 'Zameranie',
    popular: 'Obľúbené',
    recent: 'Nedávne',
    skillsMarketplace: 'Trhovisko zručností',
    sortBy: 'Zoradiť podľa',
  },
  'sv-SE': {
    changelog: 'Ändringslogg',
    focus: 'Fokus',
    popular: 'Populära',
    recent: 'Senaste',
    skillsMarketplace: 'Färdighetsmarknad',
    sortBy: 'Sortera efter',
  },
  'zh-CN': {
    changelog: '更新日志',
    focus: '重点',
    popular: '热门',
    recent: '最新',
    skillsMarketplace: '技能市场',
    sortBy: '排序',
  },
  'zh-TW': {
    changelog: '更新日誌',
    focus: '重點',
    popular: '熱門',
    recent: '最新',
    skillsMarketplace: '技能市場',
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
  const followup = skillStoreFollowupLabels[localeKey]
  const sourceImport = skillStoreSourceImportBackfills[localeKey]
  const tail = skillStoreTailLabels[localeKey]
  const statsPatch: LocaleNode = {}
  const tabsPatch: LocaleNode = {}
  const searchPatch: LocaleNode = {}
  const actionsPatch: LocaleNode = {}
  const categoriesPatch: LocaleNode = {}
  const emptyPatch: LocaleNode = {}
  const statusPatch: LocaleNode = {}
  const paginationPatch: LocaleNode = {}
  const detailPatch: LocaleNode = {}
  const detailSectionsPatch: LocaleNode = {}
  const detailMetaPatch: LocaleNode = {}
  const modalPatch: LocaleNode = {}
  const notificationsPatch: LocaleNode = {}
  const marketplacePatch: LocaleNode = {}
  const marketplaceAdvisorPatch: LocaleNode = {}
  const marketplaceDetailPatch: LocaleNode = {}
  const marketplaceDetailMetaPatch: LocaleNode = {}
  const marketplaceEvidenceTypesPatch: LocaleNode = {}
  const marketplaceQuickPatch: LocaleNode = {}
  const marketplaceResultsPatch: LocaleNode = {}
  const marketplaceSortPatch: LocaleNode = {}

  const mirrors: Array<[LocaleNode, string, string | null]> = [
    [statsPatch, 'available', getString(messages, 'extensions.stats.available')],
    [statsPatch, 'downloads', getString(messages, 'skillStore.downloads')],
    [statsPatch, 'enabled', getString(messages, 'extensions.stats.enabled')],
    [statsPatch, 'installed', getString(messages, 'extensions.stats.installed')],
    [statsPatch, 'reviews', getString(messages, 'skillStore.reviews')],
    [statsPatch, 'rating', getString(messages, 'skillStore.rating')],
    [statsPatch, 'stars', getString(messages, 'skillStore.stars')],
    [searchPatch, 'sourceFeatured', followup.featured],
    [searchPatch, 'sourceInstalled', getString(messages, 'extensions.status.installed')],
    [searchPatch, 'sourceLocal', followup.local],
    [searchPatch, 'sourceStore', followup.store],
    [searchPatch, 'clearSearch', getString(messages, 'skillStore.clearSearch')],
    [searchPatch, 'resultsCount', followup.resultsCount],
    [searchPatch, 'sortRelevance', getString(messages, 'chat.deepResearchRelevance')],
    [searchPatch, 'sortDownloads', getString(messages, 'skillStore.downloads')],
    [
      searchPatch,
      'sortStars',
      getString(messages, 'skillStore.sort.stars') ?? getString(messages, 'skillStore.stars'),
    ],
    [searchPatch, 'sortRecent', getString(messages, 'skillStore.sort.updated')],
    [searchPatch, 'sortName', getString(messages, 'common.name')],
    [actionsPatch, 'addSource', getString(messages, 'extensions.actions.addSource')],
    [actionsPatch, 'disable', getString(messages, 'extensions.actions.disable')],
    [
      actionsPatch,
      'details',
      tail?.details ??
        getString(messages, 'extensions.actions.details') ??
        getString(messages, 'common.details'),
    ],
    [actionsPatch, 'enable', getString(messages, 'extensions.actions.enable')],
    [actionsPatch, 'install', getString(messages, 'extensions.actions.install')],
    [actionsPatch, 'installFromURL', getString(messages, 'plugins.installFromUrl')],
    [actionsPatch, 'loadMore', followup.loadMore],
    [actionsPatch, 'refresh', getString(messages, 'extensions.actions.refresh')],
    [actionsPatch, 'scanLocal', followup.scanLocal],
    [actionsPatch, 'uninstall', getString(messages, 'extensions.actions.uninstall')],
    [actionsPatch, 'verify', getString(messages, 'settings.externalAgents.verify')],
    [categoriesPatch, 'other', getString(messages, 'plugins.categories.other')],
    [actionsPatch, 'loading', getString(messages, 'common.loading')],
    [
      emptyPatch,
      'browseCategories',
      tail?.browseCategories ??
        getString(messages, 'skillStore.marketplace.quick.categories') ??
        getString(messages, 'common.category'),
    ],
    [emptyPatch, 'checkFeatured', followup.featured],
    [emptyPatch, 'checkSpelling', tail?.emptyCheckSpelling ?? null],
    [emptyPatch, 'installFromUrl', getString(messages, 'plugins.installFromUrl')],
    [emptyPatch, 'loadingStore', getString(messages, 'extensions.empty.loadingStore')],
    [emptyPatch, 'noMatchingInstalled', getString(messages, 'skillStore.noSkillsFound')],
    [emptyPatch, 'noMatchingStore', getString(messages, 'skillStore.noResults')],
    [emptyPatch, 'noResultsTitle', tail?.emptyNoResultsTitle ?? null],
    [emptyPatch, 'scanLocalSkills', followup.scanLocal],
    [emptyPatch, 'suggestions', tail?.emptySuggestions ?? null],
    [statusPatch, 'builtin', getString(messages, 'extensions.status.builtin')],
    [statusPatch, 'featured', followup.featured],
    [
      statusPatch,
      'initializing',
      getString(messages, 'skillStore.marketplace.progress.phaseInitializing'),
    ],
    [statusPatch, 'initializingDesc', getString(messages, 'extensions.empty.loadingStore')],
    [
      statusPatch,
      'initializingProgress',
      getString(messages, 'skillStore.marketplace.progress.sourcesProgress'),
    ],
    [
      statusPatch,
      'initializingSource',
      getString(messages, 'skillStore.marketplace.progress.processingSource'),
    ],
    [statusPatch, 'installed', getString(messages, 'extensions.status.installed')],
    [statusPatch, 'local', followup.local],
    [statusPatch, 'ready', followup.ready],
    [detailPatch, 'emptyTitle', getString(messages, 'skills.detail.emptyTitle')],
    [detailPatch, 'emptyDescription', getString(messages, 'skills.detail.emptyDescription')],
    [detailPatch, 'noDetails', getString(messages, 'skills.noContent')],
    [detailSectionsPatch, 'description', getString(messages, 'common.description')],
    [
      detailSectionsPatch,
      'details',
      tail?.details ??
        getString(messages, 'extensions.actions.details') ??
        getString(messages, 'common.details'),
    ],
    [detailMetaPatch, 'author', getString(messages, 'skills.detail.labels.author')],
    [
      detailMetaPatch,
      'version',
      getString(messages, 'skills.detail.labels.version') ?? getString(messages, 'system.version'),
    ],
    [detailMetaPatch, 'updated', getString(messages, 'skillStore.sort.updated')],
    [detailMetaPatch, 'downloads', getString(messages, 'skillStore.downloads')],
    [detailMetaPatch, 'stars', getString(messages, 'skillStore.stars')],
    [detailMetaPatch, 'rating', getString(messages, 'skillStore.rating')],
    [
      modalPatch,
      'add',
      getString(messages, 'extensions.modal.add') ?? getString(messages, 'common.add'),
    ],
    [
      modalPatch,
      'addSourceTitle',
      getString(messages, 'extensions.modal.addSourceTitle') ??
        getString(messages, 'extensions.modal.addPluginSourceTitle'),
    ],
    [
      modalPatch,
      'cancel',
      getString(messages, 'extensions.modal.cancel') ?? getString(messages, 'common.cancel'),
    ],
    [
      modalPatch,
      'descriptionOptional',
      getString(messages, 'extensions.modal.descriptionOptional'),
    ],
    [
      modalPatch,
      'descriptionPlaceholder',
      getString(messages, 'extensions.modal.descriptionPlaceholder'),
    ],
    [modalPatch, 'confirmUninstallTitle', getString(messages, 'extensions.actions.uninstall')],
    [modalPatch, 'install', getString(messages, 'extensions.actions.install')],
    [modalPatch, 'installFromURLTitle', getString(messages, 'plugins.installFromUrl')],
    [modalPatch, 'installing', getString(messages, 'skills.deps.installing')],
    [modalPatch, 'name', getString(messages, 'extensions.modal.name')],
    [
      modalPatch,
      'namePlaceholder',
      tail?.namePlaceholder ?? getString(messages, 'extensions.modal.namePlaceholder'),
    ],
    [modalPatch, 'skillNameOptional', getString(messages, 'extensions.modal.name')],
    [
      modalPatch,
      'skillNamePlaceholder',
      tail?.namePlaceholder ?? getString(messages, 'extensions.modal.namePlaceholder'),
    ],
    [modalPatch, 'skillURL', tail?.skillURL ?? getString(messages, 'extensions.modal.url')],
    [modalPatch, 'sourceId', getString(messages, 'extensions.modal.sourceId')],
    [modalPatch, 'type', getString(messages, 'extensions.modal.type')],
    [modalPatch, 'typeCustom', getString(messages, 'extensions.modal.typeCustom')],
    [modalPatch, 'typeGithub', getString(messages, 'extensions.modal.typeGithub')],
    [modalPatch, 'uninstall', getString(messages, 'extensions.actions.uninstall')],
    [notificationsPatch, 'installError', followup.installError],
    [notificationsPatch, 'installSuccess', tail?.notificationInstallSuccess ?? null],
    [notificationsPatch, 'scanSuccess', tail?.notificationScanSuccess ?? null],
    [notificationsPatch, 'uninstallError', followup.uninstallError],
    [notificationsPatch, 'uninstallSuccess', tail?.notificationUninstallSuccess ?? null],
    [marketplaceAdvisorPatch, 'capabilityTags', followup.advisorCapabilityTags],
    [marketplaceAdvisorPatch, 'kicker', followup.advisorKicker],
    [marketplaceAdvisorPatch, 'searchQueries', followup.advisorSearchQueries],
    [marketplaceDetailMetaPatch, 'author', getString(messages, 'skills.detail.labels.author')],
    [marketplaceDetailMetaPatch, 'category', getString(messages, 'skills.detail.labels.category')],
    [
      marketplaceEvidenceTypesPatch,
      'prompt_injection',
      getString(messages, 'security.scan.items.ai_prompt_injection.name'),
    ],
    [marketplaceResultsPatch, 'installable', followup.installable],
    [
      marketplaceResultsPatch,
      'sourcesLabel',
      getString(messages, 'skillStore.marketplace.progress.sourcesLabel'),
    ],
    [marketplaceSortPatch, 'featured', followup.featured],
    [paginationPatch, 'showing', followup.paginationShowing],
    [tabsPatch, 'featured', followup.featured],
    [tabsPatch, 'installedWithCount', getString(messages, 'extensions.tabs.installedWithCount')],
    [tabsPatch, 'local', followup.local],
    [tabsPatch, 'store', followup.store],
    [tabsPatch, 'storeWithCount', getString(messages, 'extensions.tabs.storeWithCount')],
  ]

  for (const [target, key, value] of mirrors) {
    if (value) {
      target[key] = value
    }
  }

  tabsPatch.popular = labels.popular
  tabsPatch.recent = labels.recent
  searchPatch.sortBy = labels.sortBy
  detailSectionsPatch.changelog = labels.changelog
  marketplaceQuickPatch.categories =
    getString(messages, 'skills.detail.labels.categories') ?? getString(messages, 'common.category')
  marketplaceQuickPatch.focus = labels.focus
  if (sourceImport) {
    marketplacePatch.sourceImport = { ...sourceImport }
  }

  const skillStorePatch: LocaleNode = {}
  skillStorePatch.confirmUninstall = followup.confirmUninstall
  skillStorePatch.fetchError = followup.fetchError
  skillStorePatch.installError = followup.installError
  skillStorePatch.uninstallError = followup.uninstallError
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
  if (hasKeys(emptyPatch)) {
    skillStorePatch.empty = emptyPatch
  }
  if (hasKeys(categoriesPatch)) {
    skillStorePatch.categories = categoriesPatch
  }
  if (hasKeys(statusPatch)) {
    skillStorePatch.status = statusPatch
  }
  if (hasKeys(detailSectionsPatch)) {
    detailPatch.sections = detailSectionsPatch
  }
  if (hasKeys(detailMetaPatch)) {
    detailPatch.meta = detailMetaPatch
  }
  if (hasKeys(detailPatch)) {
    skillStorePatch.detail = detailPatch
  }
  if (hasKeys(paginationPatch)) {
    skillStorePatch.pagination = paginationPatch
  }
  if (hasKeys(modalPatch)) {
    skillStorePatch.modal = modalPatch
  }
  if (hasKeys(notificationsPatch)) {
    skillStorePatch.notifications = notificationsPatch
  }
  if (hasKeys(marketplaceAdvisorPatch)) {
    marketplacePatch.advisor = marketplaceAdvisorPatch
  }
  if (hasKeys(marketplaceDetailMetaPatch)) {
    marketplaceDetailPatch.meta = marketplaceDetailMetaPatch
  }
  if (hasKeys(marketplaceDetailPatch)) {
    marketplacePatch.detail = marketplaceDetailPatch
  }
  if (hasKeys(marketplaceEvidenceTypesPatch)) {
    marketplacePatch.evidenceTypes = marketplaceEvidenceTypesPatch
  }
  if (hasKeys(marketplaceResultsPatch)) {
    marketplacePatch.results = marketplaceResultsPatch
  }
  if (hasKeys(marketplaceQuickPatch)) {
    marketplacePatch.quick = marketplaceQuickPatch
  }
  if (hasKeys(marketplaceSortPatch)) {
    marketplacePatch.sort = marketplaceSortPatch
  }
  if (hasKeys(marketplacePatch)) {
    skillStorePatch.marketplace = marketplacePatch
  }

  return hasKeys(skillStorePatch) ? { skillStore: skillStorePatch } : {}
}
