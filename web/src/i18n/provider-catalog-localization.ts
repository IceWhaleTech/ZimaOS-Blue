import { localeKeys, type LocaleKey } from './locale-catalog'

const PROVIDER_CATALOG_DESCRIPTION_IDS = [
  'openai',
  'anthropic',
  'openrouter',
  'minimax',
  'qwen',
  'google',
  'deepseek',
  'moonshot',
  'ollama',
  'grok',
  'glm',
  'siliconflow',
  'nvidia',
  'venice',
  'bedrock',
  'aihubmix',
  'openrouter-free',
  'google-antigravity',
  'google-gemini-cli',
  'github-copilot',
  'openai-codex',
] as const

export const providerCatalogDescriptionIds = PROVIDER_CATALOG_DESCRIPTION_IDS

type ProviderCatalogDescriptionId = (typeof PROVIDER_CATALOG_DESCRIPTION_IDS)[number]
type ExtraProviderCatalogApiFormat = 'ollama' | 'cloudcode' | 'copilot'

type ProviderCatalogTemplates = {
  apiOf: string
  apiWithItems: string
  apiWithItemsAndMore: string
  accessMultipleModelsOneApi: string
  apiSeriesModels: string
  apiSeriesModelsWithMultilingualSupport: string
  runOpenSourceLocallyVia: string
  modelPlatform: string
  managedModelPlatform: string
  privacyFocusedAiApi: string
  aggregatorApi: string
  freeTierProfile: string
  oauthBasedVia: string
  oauthBasedProvider: string
  oauthBasedSubscriptionProvider: string
}

const localeKeySet = new Set(localeKeys)
const providerCatalogDescriptionIdSet = new Set<string>(PROVIDER_CATALOG_DESCRIPTION_IDS)
const extraProviderCatalogApiFormats = {
  ollama: 'Ollama',
  cloudcode: 'Cloud Code Assist',
  copilot: 'GitHub Copilot',
} as const satisfies Record<ExtraProviderCatalogApiFormat, string>

const localeFallbacks: Record<string, LocaleKey> = {
  ca: 'ca-ES',
  cs: 'cs-CZ',
  da: 'da-DK',
  de: 'de-DE',
  el: 'el-GR',
  en: 'en-US',
  es: 'es-ES',
  fr: 'fr-FR',
  ga: 'ga-IE',
  hr: 'hr-HR',
  hu: 'hu-HU',
  it: 'it-IT',
  ja: 'ja-JP',
  ko: 'ko-KR',
  ml: 'ml-IN',
  nb: 'nb-NO',
  nl: 'nl-NL',
  no: 'nb-NO',
  pl: 'pl-PL',
  pt: 'pt-BR',
  ro: 'ro-RO',
  ru: 'ru-RU',
  sk: 'sk-SK',
  sv: 'sv-SE',
  zh: 'zh-CN',
}

function applyTemplate(template: string, values: Record<string, string>): string {
  return template.replace(/\{(\w+)\}/g, (_, key: string) => values[key] ?? '')
}

function resolveProviderCatalogLocale(locale: string | null | undefined): LocaleKey {
  const normalized = locale?.trim()
  if (normalized && localeKeySet.has(normalized as LocaleKey)) {
    return normalized as LocaleKey
  }

  const lower = normalized?.toLowerCase() ?? ''
  if (lower === 'zh-hk' || lower === 'zh-tw') return 'zh-TW'
  if (lower === 'en-gb') return 'en-GB'
  if (lower === 'en-us') return 'en-US'
  if (lower === 'pt-pt') return 'pt-PT'
  if (lower === 'pt-br') return 'pt-BR'
  if (lower === 'zh-cn') return 'zh-CN'

  const language = lower.split(/[-_]/)[0] || ''
  return localeFallbacks[language] ?? 'en-US'
}

function isProviderCatalogDescriptionId(value: string): value is ProviderCatalogDescriptionId {
  return providerCatalogDescriptionIdSet.has(value)
}

const providerCatalogTemplatesByLocale: Record<LocaleKey, ProviderCatalogTemplates> = {
  'ca-ES': {
    apiOf: "API de {label}",
    apiWithItems: "API de {label} - {items}",
    apiWithItemsAndMore: "API de {label} - {items} i més",
    accessMultipleModelsOneApi: "{label} - Accés a múltiples models a través d'una sola API",
    apiSeriesModels: "API de {label} - models de la sèrie {series}",
    apiSeriesModelsWithMultilingualSupport:
      "API de {label} - models de la sèrie {series} amb suport multilingüe",
    runOpenSourceLocallyVia: "Executa models de codi obert localment amb {label}",
    modelPlatform: "plataforma de models de {label}",
    managedModelPlatform: "plataforma gestionada de models de {label}",
    privacyFocusedAiApi: "API d'IA centrada en la privadesa de {label}",
    aggregatorApi: "API agregadora de {label}",
    freeTierProfile: "perfil de nivell gratuït de {label}",
    oauthBasedVia: "{label} basat en OAuth mitjançant {variant}",
    oauthBasedProvider: "proveïdor de {label} basat en OAuth",
    oauthBasedSubscriptionProvider: "proveïdor per subscripció de {label} basat en OAuth",
  },
  'cs-CZ': {
    apiOf: 'API {label}',
    apiWithItems: 'API {label} - {items}',
    apiWithItemsAndMore: 'API {label} - {items} a další',
    accessMultipleModelsOneApi: '{label} - přístup k více modelům přes jedno API',
    apiSeriesModels: 'API {label} - modely řady {series}',
    apiSeriesModelsWithMultilingualSupport:
      'API {label} - modely řady {series} s vícejazyčnou podporou',
    runOpenSourceLocallyVia: 'Spouštějte open-source modely lokálně přes {label}',
    modelPlatform: 'platforma modelů {label}',
    managedModelPlatform: 'spravovaná platforma modelů {label}',
    privacyFocusedAiApi: 'API AI zaměřené na soukromí od {label}',
    aggregatorApi: 'agregační API {label}',
    freeTierProfile: 'profil bezplatné úrovně {label}',
    oauthBasedVia: '{label} založené na OAuth přes {variant}',
    oauthBasedProvider: 'poskytovatel {label} založený na OAuth',
    oauthBasedSubscriptionProvider: 'předplacený poskytovatel {label} založený na OAuth',
  },
  'da-DK': {
    apiOf: '{label} API',
    apiWithItems: '{label} API - {items}',
    apiWithItemsAndMore: '{label} API - {items} og mere',
    accessMultipleModelsOneApi: '{label} - Få adgang til flere modeller gennem ét API',
    apiSeriesModels: '{label} API - modeller i {series}-serien',
    apiSeriesModelsWithMultilingualSupport:
      '{label} API - modeller i {series}-serien med flersproget understøttelse',
    runOpenSourceLocallyVia: 'Kør open source-modeller lokalt via {label}',
    modelPlatform: '{label}-modelplatform',
    managedModelPlatform: '{label}-administreret modelplatform',
    privacyFocusedAiApi: '{label} privatlivsfokuseret AI-API',
    aggregatorApi: '{label} aggregator-API',
    freeTierProfile: '{label} gratisprofil',
    oauthBasedVia: 'OAuth-baseret {label} via {variant}',
    oauthBasedProvider: 'OAuth-baseret {label}-udbyder',
    oauthBasedSubscriptionProvider: 'OAuth-baseret abonnementsudbyder for {label}',
  },
  'de-DE': {
    apiOf: '{label} API',
    apiWithItems: '{label} API - {items}',
    apiWithItemsAndMore: '{label} API - {items} und mehr',
    accessMultipleModelsOneApi: '{label} - Zugriff auf mehrere Modelle über eine API',
    apiSeriesModels: '{label} API - Modelle der {series}-Reihe',
    apiSeriesModelsWithMultilingualSupport:
      '{label} API - Modelle der {series}-Reihe mit mehrsprachiger Unterstützung',
    runOpenSourceLocallyVia: 'Open-Source-Modelle lokal über {label} ausführen',
    modelPlatform: '{label}-Modellplattform',
    managedModelPlatform: 'verwaltete Modellplattform von {label}',
    privacyFocusedAiApi: 'datenschutzorientierte KI-API von {label}',
    aggregatorApi: 'Aggregator-API von {label}',
    freeTierProfile: 'Free-Tier-Profil von {label}',
    oauthBasedVia: 'OAuth-basiertes {label} über {variant}',
    oauthBasedProvider: 'OAuth-basierter {label}-Anbieter',
    oauthBasedSubscriptionProvider: 'OAuth-basierter Abonnement-Anbieter für {label}',
  },
  'el-GR': {
    apiOf: 'API του {label}',
    apiWithItems: 'API του {label} - {items}',
    apiWithItemsAndMore: 'API του {label} - {items} και άλλα',
    accessMultipleModelsOneApi: '{label} - Πρόσβαση σε πολλαπλά μοντέλα μέσω ενός API',
    apiSeriesModels: 'API του {label} - μοντέλα σειράς {series}',
    apiSeriesModelsWithMultilingualSupport:
      'API του {label} - μοντέλα σειράς {series} με πολύγλωσση υποστήριξη',
    runOpenSourceLocallyVia: 'Τρέξτε open-source μοντέλα τοπικά μέσω του {label}',
    modelPlatform: 'πλατφόρμα μοντέλων {label}',
    managedModelPlatform: 'διαχειριζόμενη πλατφόρμα μοντέλων {label}',
    privacyFocusedAiApi: 'AI API με έμφαση στην ιδιωτικότητα από το {label}',
    aggregatorApi: 'συγκεντρωτικό API του {label}',
    freeTierProfile: 'προφίλ δωρεάν βαθμίδας του {label}',
    oauthBasedVia: '{label} βασισμένο σε OAuth μέσω {variant}',
    oauthBasedProvider: 'πάροχος {label} βασισμένος σε OAuth',
    oauthBasedSubscriptionProvider: 'συνδρομητικός πάροχος {label} βασισμένος σε OAuth',
  },
  'en-GB': {
    apiOf: '{label} API',
    apiWithItems: '{label} API - {items}',
    apiWithItemsAndMore: '{label} API - {items}, and more',
    accessMultipleModelsOneApi: '{label} - Access multiple models through one API',
    apiSeriesModels: '{label} API - {series} series models',
    apiSeriesModelsWithMultilingualSupport:
      '{label} API - {series} series models with multilingual support',
    runOpenSourceLocallyVia: 'Run open-source models locally via {label}',
    modelPlatform: '{label} model platform',
    managedModelPlatform: '{label} managed model platform',
    privacyFocusedAiApi: '{label} privacy-focused AI API',
    aggregatorApi: '{label} aggregator API',
    freeTierProfile: '{label} free-tier profile',
    oauthBasedVia: 'OAuth-based {label} via {variant}',
    oauthBasedProvider: 'OAuth-based {label} provider',
    oauthBasedSubscriptionProvider: 'OAuth-based {label} subscription provider',
  },
  'en-US': {
    apiOf: '{label} API',
    apiWithItems: '{label} API - {items}',
    apiWithItemsAndMore: '{label} API - {items}, and more',
    accessMultipleModelsOneApi: '{label} - Access multiple models through one API',
    apiSeriesModels: '{label} API - {series} series models',
    apiSeriesModelsWithMultilingualSupport:
      '{label} API - {series} series models with multilingual support',
    runOpenSourceLocallyVia: 'Run open-source models locally via {label}',
    modelPlatform: '{label} model platform',
    managedModelPlatform: '{label} managed model platform',
    privacyFocusedAiApi: '{label} privacy-focused AI API',
    aggregatorApi: '{label} aggregator API',
    freeTierProfile: '{label} free-tier profile',
    oauthBasedVia: 'OAuth-based {label} via {variant}',
    oauthBasedProvider: 'OAuth-based {label} provider',
    oauthBasedSubscriptionProvider: 'OAuth-based {label} subscription provider',
  },
  'es-ES': {
    apiOf: 'API de {label}',
    apiWithItems: 'API de {label} - {items}',
    apiWithItemsAndMore: 'API de {label} - {items} y más',
    accessMultipleModelsOneApi: '{label} - Acceso a múltiples modelos mediante una sola API',
    apiSeriesModels: 'API de {label} - modelos de la serie {series}',
    apiSeriesModelsWithMultilingualSupport:
      'API de {label} - modelos de la serie {series} con soporte multilingüe',
    runOpenSourceLocallyVia: 'Ejecuta modelos de código abierto localmente con {label}',
    modelPlatform: 'plataforma de modelos de {label}',
    managedModelPlatform: 'plataforma administrada de modelos de {label}',
    privacyFocusedAiApi: 'API de IA centrada en la privacidad de {label}',
    aggregatorApi: 'API agregadora de {label}',
    freeTierProfile: 'perfil de nivel gratuito de {label}',
    oauthBasedVia: '{label} basado en OAuth a través de {variant}',
    oauthBasedProvider: 'proveedor de {label} basado en OAuth',
    oauthBasedSubscriptionProvider: 'proveedor por suscripción de {label} basado en OAuth',
  },
  'fr-FR': {
    apiOf: 'API {label}',
    apiWithItems: 'API {label} - {items}',
    apiWithItemsAndMore: 'API {label} - {items} et plus encore',
    accessMultipleModelsOneApi: '{label} - Accédez à plusieurs modèles via une seule API',
    apiSeriesModels: 'API {label} - modèles de la série {series}',
    apiSeriesModelsWithMultilingualSupport:
      'API {label} - modèles de la série {series} avec prise en charge multilingue',
    runOpenSourceLocallyVia: 'Exécutez des modèles open source en local via {label}',
    modelPlatform: 'plateforme de modèles {label}',
    managedModelPlatform: 'plateforme de modèles gérée {label}',
    privacyFocusedAiApi: 'API IA axée sur la confidentialité de {label}',
    aggregatorApi: 'API agrégatrice {label}',
    freeTierProfile: 'profil niveau gratuit {label}',
    oauthBasedVia: '{label} basé sur OAuth via {variant}',
    oauthBasedProvider: 'fournisseur {label} basé sur OAuth',
    oauthBasedSubscriptionProvider: 'fournisseur par abonnement {label} basé sur OAuth',
  },
  'ga-IE': {
    apiOf: 'API {label}',
    apiWithItems: 'API {label} - {items}',
    apiWithItemsAndMore: 'API {label} - {items} agus níos mó',
    accessMultipleModelsOneApi: '{label} - Rochtain ar ilmhúnlaí trí API amháin',
    apiSeriesModels: 'API {label} - samhlacha sraithe {series}',
    apiSeriesModelsWithMultilingualSupport:
      'API {label} - samhlacha sraithe {series} le tacaíocht ilteangach',
    runOpenSourceLocallyVia: 'Rith samhlacha foinse oscailte go háitiúil trí {label}',
    modelPlatform: 'ardán samhlacha {label}',
    managedModelPlatform: 'ardán bainistithe samhlacha {label}',
    privacyFocusedAiApi: 'API AI atá dírithe ar phríobháideachas ó {label}',
    aggregatorApi: 'API comhiomlánaithe {label}',
    freeTierProfile: 'próifíl saor-in-aisce {label}',
    oauthBasedVia: '{label} bunaithe ar OAuth trí {variant}',
    oauthBasedProvider: 'soláthraí {label} bunaithe ar OAuth',
    oauthBasedSubscriptionProvider: 'soláthraí síntiúis {label} bunaithe ar OAuth',
  },
  'hr-HR': {
    apiOf: '{label} API',
    apiWithItems: '{label} API - {items}',
    apiWithItemsAndMore: '{label} API - {items} i više',
    accessMultipleModelsOneApi: '{label} - Pristup više modela kroz jedan API',
    apiSeriesModels: '{label} API - modeli serije {series}',
    apiSeriesModelsWithMultilingualSupport:
      '{label} API - modeli serije {series} s višejezičnom podrškom',
    runOpenSourceLocallyVia: 'Pokrećite open-source modele lokalno putem {label}',
    modelPlatform: 'platforma modela {label}',
    managedModelPlatform: 'upravljana platforma modela {label}',
    privacyFocusedAiApi: '{label} AI API usmjeren na privatnost',
    aggregatorApi: '{label} agregacijski API',
    freeTierProfile: '{label} profil besplatne razine',
    oauthBasedVia: '{label} temeljen na OAuth-u putem {variant}',
    oauthBasedProvider: '{label} pružatelj temeljen na OAuth-u',
    oauthBasedSubscriptionProvider: 'pretplatnički pružatelj {label} temeljen na OAuth-u',
  },
  'hu-HU': {
    apiOf: '{label} API',
    apiWithItems: '{label} API - {items}',
    apiWithItemsAndMore: '{label} API - {items} és még több',
    accessMultipleModelsOneApi: '{label} - Több modell elérése egyetlen API-n keresztül',
    apiSeriesModels: '{label} API - {series} sorozatú modellek',
    apiSeriesModelsWithMultilingualSupport:
      '{label} API - {series} sorozatú modellek többnyelvű támogatással',
    runOpenSourceLocallyVia: 'Nyílt forráskódú modellek helyi futtatása {label} segítségével',
    modelPlatform: '{label} modellplatform',
    managedModelPlatform: '{label} felügyelt modellplatform',
    privacyFocusedAiApi: '{label} adatvédelem-központú AI API',
    aggregatorApi: '{label} aggregátor API',
    freeTierProfile: '{label} ingyenes csomagprofil',
    oauthBasedVia: 'OAuth-alapú {label} {variant} használatával',
    oauthBasedProvider: 'OAuth-alapú {label} szolgáltató',
    oauthBasedSubscriptionProvider: 'OAuth-alapú {label} előfizetéses szolgáltató',
  },
  'it-IT': {
    apiOf: 'API {label}',
    apiWithItems: 'API {label} - {items}',
    apiWithItemsAndMore: 'API {label} - {items} e altro',
    accessMultipleModelsOneApi: '{label} - Accedi a più modelli tramite una sola API',
    apiSeriesModels: 'API {label} - modelli della serie {series}',
    apiSeriesModelsWithMultilingualSupport:
      'API {label} - modelli della serie {series} con supporto multilingue',
    runOpenSourceLocallyVia: 'Esegui modelli open source in locale tramite {label}',
    modelPlatform: 'piattaforma di modelli {label}',
    managedModelPlatform: 'piattaforma gestita di modelli {label}',
    privacyFocusedAiApi: 'API IA orientata alla privacy di {label}',
    aggregatorApi: 'API aggregatrice di {label}',
    freeTierProfile: 'profilo free tier di {label}',
    oauthBasedVia: '{label} basato su OAuth tramite {variant}',
    oauthBasedProvider: 'provider {label} basato su OAuth',
    oauthBasedSubscriptionProvider: 'provider in abbonamento {label} basato su OAuth',
  },
  'ja-JP': {
    apiOf: '{label} API',
    apiWithItems: '{label} API - {items}',
    apiWithItemsAndMore: '{label} API - {items} など',
    accessMultipleModelsOneApi: '{label} - 1 つの API で複数のモデルにアクセス',
    apiSeriesModels: '{label} API - {series} シリーズモデル',
    apiSeriesModelsWithMultilingualSupport:
      '{label} API - 多言語対応の {series} シリーズモデル',
    runOpenSourceLocallyVia: '{label} でオープンソースモデルをローカル実行',
    modelPlatform: '{label} モデルプラットフォーム',
    managedModelPlatform: '{label} マネージドモデルプラットフォーム',
    privacyFocusedAiApi: '{label} のプライバシー重視 AI API',
    aggregatorApi: '{label} アグリゲーター API',
    freeTierProfile: '{label} 無料ティアプロファイル',
    oauthBasedVia: '{variant} 経由の OAuth ベース {label}',
    oauthBasedProvider: 'OAuth ベースの {label} プロバイダー',
    oauthBasedSubscriptionProvider: 'OAuth ベースの {label} サブスクリプションプロバイダー',
  },
  'ko-KR': {
    apiOf: '{label} API',
    apiWithItems: '{label} API - {items}',
    apiWithItemsAndMore: '{label} API - {items} 등',
    accessMultipleModelsOneApi: '{label} - 하나의 API로 여러 모델에 액세스',
    apiSeriesModels: '{label} API - {series} 시리즈 모델',
    apiSeriesModelsWithMultilingualSupport:
      '{label} API - 다국어 지원을 갖춘 {series} 시리즈 모델',
    runOpenSourceLocallyVia: '{label}로 오픈소스 모델을 로컬에서 실행',
    modelPlatform: '{label} 모델 플랫폼',
    managedModelPlatform: '{label} 관리형 모델 플랫폼',
    privacyFocusedAiApi: '{label} 프라이버시 중심 AI API',
    aggregatorApi: '{label} 집계 API',
    freeTierProfile: '{label} 무료 티어 프로필',
    oauthBasedVia: '{variant}를 통한 OAuth 기반 {label}',
    oauthBasedProvider: 'OAuth 기반 {label} 제공자',
    oauthBasedSubscriptionProvider: 'OAuth 기반 {label} 구독 제공자',
  },
  'ml-IN': {
    apiOf: '{label} API',
    apiWithItems: '{label} API - {items}',
    apiWithItemsAndMore: '{label} API - {items}യും മറ്റു പലതും',
    accessMultipleModelsOneApi: '{label} - ഒരു API വഴിയായി നിരവധി മോഡലുകൾ ലഭ്യമാക്കുക',
    apiSeriesModels: '{label} API - {series} സീരീസ് മോഡലുകൾ',
    apiSeriesModelsWithMultilingualSupport:
      '{label} API - ബഹുഭാഷാ പിന്തുണയുള്ള {series} സീരീസ് മോഡലുകൾ',
    runOpenSourceLocallyVia: '{label} വഴി ഓപ്പൺ സോഴ്‌സ് മോഡലുകൾ ലോക്കലായി പ്രവർത്തിപ്പിക്കുക',
    modelPlatform: '{label} മോഡൽ പ്ലാറ്റ്ഫോം',
    managedModelPlatform: '{label} മാനേജുചെയ്യുന്ന മോഡൽ പ്ലാറ്റ്ഫോം',
    privacyFocusedAiApi: '{label} സ്വകാര്യത കേന്ദ്രീകരിച്ച AI API',
    aggregatorApi: '{label} അഗ്രിഗേറ്റർ API',
    freeTierProfile: '{label} ഫ്രീ-ടയർ പ്രൊഫൈൽ',
    oauthBasedVia: '{variant} വഴി OAuth അടിസ്ഥാനമാക്കിയ {label}',
    oauthBasedProvider: 'OAuth അടിസ്ഥാനമാക്കിയ {label} ദാതാവ്',
    oauthBasedSubscriptionProvider: 'OAuth അടിസ്ഥാനമാക്കിയ {label} സബ്സ്ക്രിപ്ഷൻ ദാതാവ്',
  },
  'nb-NO': {
    apiOf: '{label} API',
    apiWithItems: '{label} API - {items}',
    apiWithItemsAndMore: '{label} API - {items} og mer',
    accessMultipleModelsOneApi: '{label} - Tilgang til flere modeller gjennom ett API',
    apiSeriesModels: '{label} API - modeller i {series}-serien',
    apiSeriesModelsWithMultilingualSupport:
      '{label} API - modeller i {series}-serien med flerspråklig støtte',
    runOpenSourceLocallyVia: 'Kjør open source-modeller lokalt via {label}',
    modelPlatform: '{label} modellplattform',
    managedModelPlatform: '{label} administrert modellplattform',
    privacyFocusedAiApi: '{label} personvernfokusert AI-API',
    aggregatorApi: '{label} aggregator-API',
    freeTierProfile: '{label} gratisnivåprofil',
    oauthBasedVia: 'OAuth-basert {label} via {variant}',
    oauthBasedProvider: 'OAuth-basert {label}-leverandør',
    oauthBasedSubscriptionProvider: 'OAuth-basert abonnementsleverandør for {label}',
  },
  'nl-NL': {
    apiOf: '{label} API',
    apiWithItems: '{label} API - {items}',
    apiWithItemsAndMore: '{label} API - {items} en meer',
    accessMultipleModelsOneApi: '{label} - Toegang tot meerdere modellen via één API',
    apiSeriesModels: '{label} API - modellen uit de {series}-serie',
    apiSeriesModelsWithMultilingualSupport:
      '{label} API - modellen uit de {series}-serie met meertalige ondersteuning',
    runOpenSourceLocallyVia: 'Draai open-sourcemodellen lokaal via {label}',
    modelPlatform: '{label} modellenplatform',
    managedModelPlatform: 'beheerd modellenplatform van {label}',
    privacyFocusedAiApi: 'privacygerichte AI-API van {label}',
    aggregatorApi: 'aggregator-API van {label}',
    freeTierProfile: 'gratis-tierprofiel van {label}',
    oauthBasedVia: 'OAuth-gebaseerde {label} via {variant}',
    oauthBasedProvider: 'OAuth-gebaseerde {label}-provider',
    oauthBasedSubscriptionProvider: 'OAuth-gebaseerde abonnementsprovider voor {label}',
  },
  'pl-PL': {
    apiOf: 'API {label}',
    apiWithItems: 'API {label} - {items}',
    apiWithItemsAndMore: 'API {label} - {items} i więcej',
    accessMultipleModelsOneApi: '{label} - Dostęp do wielu modeli przez jedno API',
    apiSeriesModels: 'API {label} - modele z serii {series}',
    apiSeriesModelsWithMultilingualSupport:
      'API {label} - modele z serii {series} z obsługą wielu języków',
    runOpenSourceLocallyVia: 'Uruchamiaj modele open source lokalnie przez {label}',
    modelPlatform: 'platforma modeli {label}',
    managedModelPlatform: 'zarządzana platforma modeli {label}',
    privacyFocusedAiApi: 'API AI {label} skoncentrowane na prywatności',
    aggregatorApi: 'agregujące API {label}',
    freeTierProfile: 'profil warstwy bezpłatnej {label}',
    oauthBasedVia: '{label} oparte na OAuth przez {variant}',
    oauthBasedProvider: 'dostawca {label} oparty na OAuth',
    oauthBasedSubscriptionProvider: 'subskrypcyjny dostawca {label} oparty na OAuth',
  },
  'pt-BR': {
    apiOf: 'API do {label}',
    apiWithItems: 'API do {label} - {items}',
    apiWithItemsAndMore: 'API do {label} - {items} e mais',
    accessMultipleModelsOneApi: '{label} - Acesse vários modelos por uma única API',
    apiSeriesModels: 'API do {label} - modelos da série {series}',
    apiSeriesModelsWithMultilingualSupport:
      'API do {label} - modelos da série {series} com suporte multilíngue',
    runOpenSourceLocallyVia: 'Execute modelos de código aberto localmente via {label}',
    modelPlatform: 'plataforma de modelos da {label}',
    managedModelPlatform: 'plataforma gerenciada de modelos da {label}',
    privacyFocusedAiApi: 'API de IA focada em privacidade da {label}',
    aggregatorApi: 'API agregadora da {label}',
    freeTierProfile: 'perfil de nível gratuito da {label}',
    oauthBasedVia: '{label} baseado em OAuth via {variant}',
    oauthBasedProvider: 'provedor {label} baseado em OAuth',
    oauthBasedSubscriptionProvider: 'provedor por assinatura {label} baseado em OAuth',
  },
  'pt-PT': {
    apiOf: 'API {label}',
    apiWithItems: 'API {label} - {items}',
    apiWithItemsAndMore: 'API {label} - {items} e mais',
    accessMultipleModelsOneApi: '{label} - Aceda a vários modelos através de uma única API',
    apiSeriesModels: 'API {label} - modelos da série {series}',
    apiSeriesModelsWithMultilingualSupport:
      'API {label} - modelos da série {series} com suporte multilingue',
    runOpenSourceLocallyVia: 'Execute modelos open source localmente através de {label}',
    modelPlatform: 'plataforma de modelos {label}',
    managedModelPlatform: 'plataforma gerida de modelos {label}',
    privacyFocusedAiApi: 'API de IA focada na privacidade de {label}',
    aggregatorApi: 'API agregadora de {label}',
    freeTierProfile: 'perfil de nível gratuito de {label}',
    oauthBasedVia: '{label} baseado em OAuth via {variant}',
    oauthBasedProvider: 'fornecedor {label} baseado em OAuth',
    oauthBasedSubscriptionProvider: 'fornecedor por subscrição {label} baseado em OAuth',
  },
  'ro-RO': {
    apiOf: 'API {label}',
    apiWithItems: 'API {label} - {items}',
    apiWithItemsAndMore: 'API {label} - {items} și multe altele',
    accessMultipleModelsOneApi: '{label} - Acces la mai multe modele printr-un singur API',
    apiSeriesModels: 'API {label} - modele din seria {series}',
    apiSeriesModelsWithMultilingualSupport:
      'API {label} - modele din seria {series} cu suport multilingv',
    runOpenSourceLocallyVia: 'Rulează local modele open-source prin {label}',
    modelPlatform: 'platformă de modele {label}',
    managedModelPlatform: 'platformă administrată de modele {label}',
    privacyFocusedAiApi: 'API AI axat pe confidențialitate de la {label}',
    aggregatorApi: 'API agregator {label}',
    freeTierProfile: 'profil gratuit {label}',
    oauthBasedVia: '{label} bazat pe OAuth prin {variant}',
    oauthBasedProvider: 'furnizor {label} bazat pe OAuth',
    oauthBasedSubscriptionProvider: 'furnizor pe bază de abonament {label} bazat pe OAuth',
  },
  'ru-RU': {
    apiOf: 'API {label}',
    apiWithItems: 'API {label} - {items}',
    apiWithItemsAndMore: 'API {label} - {items} и многое другое',
    accessMultipleModelsOneApi: '{label} - доступ к нескольким моделям через один API',
    apiSeriesModels: 'API {label} - модели серии {series}',
    apiSeriesModelsWithMultilingualSupport:
      'API {label} - модели серии {series} с многоязычной поддержкой',
    runOpenSourceLocallyVia: 'Запускайте open-source модели локально через {label}',
    modelPlatform: 'платформа моделей {label}',
    managedModelPlatform: 'управляемая платформа моделей {label}',
    privacyFocusedAiApi: 'ориентированный на конфиденциальность AI API {label}',
    aggregatorApi: 'агрегирующий API {label}',
    freeTierProfile: 'профиль бесплатного уровня {label}',
    oauthBasedVia: '{label} на основе OAuth через {variant}',
    oauthBasedProvider: 'провайдер {label} на основе OAuth',
    oauthBasedSubscriptionProvider: 'подписочный провайдер {label} на основе OAuth',
  },
  'sk-SK': {
    apiOf: 'API {label}',
    apiWithItems: 'API {label} - {items}',
    apiWithItemsAndMore: 'API {label} - {items} a ďalšie',
    accessMultipleModelsOneApi: '{label} - Prístup k viacerým modelom cez jedno API',
    apiSeriesModels: 'API {label} - modely série {series}',
    apiSeriesModelsWithMultilingualSupport:
      'API {label} - modely série {series} s viacjazyčnou podporou',
    runOpenSourceLocallyVia: 'Spúšťajte open-source modely lokálne cez {label}',
    modelPlatform: 'platforma modelov {label}',
    managedModelPlatform: 'spravovaná platforma modelov {label}',
    privacyFocusedAiApi: 'AI API zamerané na súkromie od {label}',
    aggregatorApi: 'agregačné API {label}',
    freeTierProfile: 'profil bezplatnej úrovne {label}',
    oauthBasedVia: '{label} založené na OAuth cez {variant}',
    oauthBasedProvider: 'poskytovateľ {label} založený na OAuth',
    oauthBasedSubscriptionProvider: 'predplatný poskytovateľ {label} založený na OAuth',
  },
  'sv-SE': {
    apiOf: '{label} API',
    apiWithItems: '{label} API - {items}',
    apiWithItemsAndMore: '{label} API - {items} och mer',
    accessMultipleModelsOneApi: '{label} - Få tillgång till flera modeller via ett API',
    apiSeriesModels: '{label} API - modeller i {series}-serien',
    apiSeriesModelsWithMultilingualSupport:
      '{label} API - modeller i {series}-serien med flerspråkigt stöd',
    runOpenSourceLocallyVia: 'Kör öppen källkodsmodeller lokalt via {label}',
    modelPlatform: '{label} modellplattform',
    managedModelPlatform: '{label} hanterad modellplattform',
    privacyFocusedAiApi: '{label} integritetsfokuserade AI-API',
    aggregatorApi: '{label} aggregator-API',
    freeTierProfile: '{label} gratisnivåprofil',
    oauthBasedVia: 'OAuth-baserad {label} via {variant}',
    oauthBasedProvider: 'OAuth-baserad {label}-leverantör',
    oauthBasedSubscriptionProvider: 'OAuth-baserad prenumerationsleverantör för {label}',
  },
  'zh-CN': {
    apiOf: '{label} API',
    apiWithItems: '{label} API - {items}',
    apiWithItemsAndMore: '{label} API - {items} 等',
    accessMultipleModelsOneApi: '{label} - 通过一个 API 访问多个模型',
    apiSeriesModels: '{label} API - {series} 系列模型',
    apiSeriesModelsWithMultilingualSupport: '{label} API - 支持多语言的 {series} 系列模型',
    runOpenSourceLocallyVia: '通过 {label} 在本地运行开源模型',
    modelPlatform: '{label} 模型平台',
    managedModelPlatform: '{label} 托管模型平台',
    privacyFocusedAiApi: '{label} 注重隐私的 AI API',
    aggregatorApi: '{label} 聚合 API',
    freeTierProfile: '{label} 免费层配置',
    oauthBasedVia: '基于 OAuth 的 {label}（通过 {variant}）',
    oauthBasedProvider: '基于 OAuth 的 {label} 提供商',
    oauthBasedSubscriptionProvider: '基于 OAuth 的 {label} 订阅提供商',
  },
  'zh-TW': {
    apiOf: '{label} API',
    apiWithItems: '{label} API - {items}',
    apiWithItemsAndMore: '{label} API - {items} 等',
    accessMultipleModelsOneApi: '{label} - 透過一個 API 存取多個模型',
    apiSeriesModels: '{label} API - {series} 系列模型',
    apiSeriesModelsWithMultilingualSupport: '{label} API - 支援多語言的 {series} 系列模型',
    runOpenSourceLocallyVia: '透過 {label} 在本機執行開源模型',
    modelPlatform: '{label} 模型平台',
    managedModelPlatform: '{label} 託管模型平台',
    privacyFocusedAiApi: '{label} 注重隱私的 AI API',
    aggregatorApi: '{label} 聚合 API',
    freeTierProfile: '{label} 免費層設定檔',
    oauthBasedVia: '基於 OAuth 的 {label}（透過 {variant}）',
    oauthBasedProvider: '基於 OAuth 的 {label} 提供商',
    oauthBasedSubscriptionProvider: '基於 OAuth 的 {label} 訂閱提供商',
  },
}

function buildProviderCatalogDescriptions(
  templates: ProviderCatalogTemplates
): Record<ProviderCatalogDescriptionId, string> {
  return {
    openai: applyTemplate(templates.apiWithItemsAndMore, {
      label: 'OpenAI',
      items: 'GPT-4, GPT-4o, o1',
    }),
    anthropic: applyTemplate(templates.apiWithItems, {
      label: 'Anthropic',
      items: 'Claude Opus, Sonnet, Haiku',
    }),
    openrouter: applyTemplate(templates.accessMultipleModelsOneApi, {
      label: 'OpenRouter',
    }),
    minimax: applyTemplate(templates.apiSeriesModels, {
      label: 'MiniMax',
      series: 'M2.7',
    }),
    qwen: applyTemplate(templates.apiSeriesModelsWithMultilingualSupport, {
      label: 'Alibaba Cloud Qwen',
      series: 'Qwen',
    }),
    google: applyTemplate(templates.apiOf, {
      label: 'Google Gemini',
    }),
    deepseek: applyTemplate(templates.apiOf, {
      label: 'DeepSeek',
    }),
    moonshot: applyTemplate(templates.apiOf, {
      label: 'Moonshot/Kimi',
    }),
    ollama: applyTemplate(templates.runOpenSourceLocallyVia, {
      label: 'Ollama',
    }),
    grok: applyTemplate(templates.apiOf, {
      label: 'xAI Grok',
    }),
    glm: applyTemplate(templates.apiOf, {
      label: 'Zhipu GLM',
    }),
    siliconflow: applyTemplate(templates.modelPlatform, {
      label: 'SiliconFlow',
    }),
    nvidia: applyTemplate(templates.apiOf, {
      label: 'NVIDIA NIM',
    }),
    venice: applyTemplate(templates.privacyFocusedAiApi, {
      label: 'Venice',
    }),
    bedrock: applyTemplate(templates.managedModelPlatform, {
      label: 'Amazon Bedrock',
    }),
    aihubmix: applyTemplate(templates.aggregatorApi, {
      label: 'AiHubMix',
    }),
    'openrouter-free': applyTemplate(templates.freeTierProfile, {
      label: 'OpenRouter',
    }),
    'google-antigravity': applyTemplate(templates.oauthBasedVia, {
      label: 'Cloud Code Assist',
      variant: 'Antigravity',
    }),
    'google-gemini-cli': applyTemplate(templates.oauthBasedVia, {
      label: 'Cloud Code Assist',
      variant: 'Gemini CLI',
    }),
    'github-copilot': applyTemplate(templates.oauthBasedProvider, {
      label: 'GitHub Copilot',
    }),
    'openai-codex': applyTemplate(templates.oauthBasedSubscriptionProvider, {
      label: 'OpenAI Codex',
    }),
  }
}

const providerCatalogDescriptionsByLocale = Object.fromEntries(
  localeKeys.map((locale) => [
    locale,
    buildProviderCatalogDescriptions(providerCatalogTemplatesByLocale[locale]),
  ])
) as Record<LocaleKey, Record<ProviderCatalogDescriptionId, string>>

export function getLocalizedProviderCatalogDescription(
  locale: string | null | undefined,
  providerId: string,
  fallbackDescription = ''
): string {
  if (!isProviderCatalogDescriptionId(providerId)) {
    return fallbackDescription
  }

  const resolvedLocale = resolveProviderCatalogLocale(locale)
  return providerCatalogDescriptionsByLocale[resolvedLocale][providerId] || fallbackDescription
}

export function getLocalizedExtraProviderApiFormatLabel(
  locale: string | null | undefined,
  apiFormat: string | null | undefined
): string {
  if (!apiFormat) return ''

  resolveProviderCatalogLocale(locale)

  return extraProviderCatalogApiFormats[apiFormat as ExtraProviderCatalogApiFormat] || apiFormat
}
