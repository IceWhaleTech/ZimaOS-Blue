import { deepResearchRuntimeBackfills } from './deep-research-runtime-backfills'
import { deepResearchSummaryBackfills } from './deep-research-summary-backfills'
import { deepResearchStructuredBackfills } from './deep-research-structured-backfills'
import { deepResearchTokenBackfills } from './deep-research-token-backfills'
import type { LocaleKey } from './locale-catalog'

type LocaleLeaf = string | number | boolean | null
type LocaleNode = { [key: string]: LocaleLeaf | LocaleNode }
type HarnessTermGlossary = {
  preset: string
  dataset: string
  version: string
  spec: string
  run: string
  baseline: string
  group: string
  scorecard: string
  agentTask: string
  prepare: string
}

type ResultCardWebQueryLabelBackfill = {
  has_results: string
  selected_result: string
  key_facts: string
  research_artifact_path: string
  llm_compacted: string
  materialized: string
  mode: string
}

const protectedHarnessTermReplacementPaths = new Set([
  'harness.group.remediationInfraProviderAuth',
  'harness.group.remediationInfraProviderQuota',
  'harness.group.remediationInfraProviderBlocked',
])

const execCardNoCommandLabels: Partial<Record<LocaleKey, string>> = {
  'ca-ES': 'Sense ordre',
  'cs-CZ': 'Bez příkazu',
  'da-DK': 'Ingen kommando',
  'el-GR': 'Χωρίς εντολή',
  'ga-IE': 'Gan ordú',
  'hr-HR': 'Bez naredbe',
  'hu-HU': 'Nincs parancs',
  'ml-IN': 'കമാൻഡ് ഇല്ല',
  'nb-NO': 'Ingen kommando',
  'pt-PT': 'Sem comando',
  'ro-RO': 'Fără comandă',
  'sk-SK': 'Bez príkazu',
}

const commonFilterLabels: Record<LocaleKey, string> = {
  'ca-ES': 'Filtre',
  'cs-CZ': 'Filtr',
  'da-DK': 'Filter',
  'de-DE': 'Filter',
  'el-GR': 'Φίλτρο',
  'en-GB': 'Filter',
  'en-US': 'Filter',
  'es-ES': 'Filtro',
  'fr-FR': 'Filtre',
  'ga-IE': 'Scagaire',
  'hr-HR': 'Filtar',
  'hu-HU': 'Szűrő',
  'it-IT': 'Filtro',
  'ja-JP': 'フィルター',
  'ko-KR': '필터',
  'ml-IN': 'ഫിൽട്ടർ',
  'nb-NO': 'Filter',
  'nl-NL': 'Filter',
  'pl-PL': 'Filtr',
  'pt-BR': 'Filtro',
  'pt-PT': 'Filtro',
  'ro-RO': 'Filtru',
  'ru-RU': 'Фильтр',
  'sk-SK': 'Filter',
  'sv-SE': 'Filter',
  'zh-CN': '筛选',
  'zh-TW': '篩選',
}

type ProviderRecoveryLocalePatch = {
  recoverableEyebrow: string
  recoverableTitle: string
  recoverableDescription: string
  retry: string
  reviewSingle: string
  providerNeedsAttention: string
  builtinProfile: string
}

const providerRecoveryEnglishDefaults: ProviderRecoveryLocalePatch = {
  recoverableEyebrow: 'Temporary provider issue',
  recoverableTitle: 'Provider needs attention',
  recoverableDescription:
    'Your configured provider hit a temporary error. Review the latest reason below, reset the provider, and then try again.',
  retry: 'Retry Provider',
  reviewSingle: 'Review Provider',
  providerNeedsAttention: 'Provider needs attention. Click to check settings.',
  builtinProfile: 'Built-in profile',
}

const providerRecoveryLocaleBackfills: Partial<Record<LocaleKey, ProviderRecoveryLocalePatch>> = {
  'ca-ES': {
    recoverableEyebrow: 'Incidència temporal del proveïdor',
    recoverableTitle: 'El proveïdor necessita atenció',
    recoverableDescription:
      'El proveïdor configurat ha trobat un error temporal. Reviseu el motiu més recent a continuació, reinicieu el proveïdor i torneu-ho a provar.',
    retry: 'Torna a provar el proveïdor',
    reviewSingle: 'Revisa el proveïdor',
    providerNeedsAttention: 'El proveïdor necessita atenció. Feu clic per revisar la configuració.',
    builtinProfile: 'Perfil integrat',
  },
  'cs-CZ': {
    recoverableEyebrow: 'Dočasný problém poskytovatele',
    recoverableTitle: 'Poskytovatel vyžaduje pozornost',
    recoverableDescription:
      'Nakonfigurovaný poskytovatel narazil na dočasnou chybu. Níže zkontrolujte poslední důvod, resetujte poskytovatele a zkuste to znovu.',
    retry: 'Zkusit poskytovatele znovu',
    reviewSingle: 'Zkontrolovat poskytovatele',
    providerNeedsAttention: 'Poskytovatel vyžaduje pozornost. Kliknutím zkontrolujte nastavení.',
    builtinProfile: 'Vestavěný profil',
  },
  'da-DK': {
    recoverableEyebrow: 'Midlertidigt problem med udbyderen',
    recoverableTitle: 'Udbyderen kræver opmærksomhed',
    recoverableDescription:
      'Din konfigurerede udbyder ramte en midlertidig fejl. Gennemgå den seneste årsag nedenfor, nulstil udbyderen, og prøv igen.',
    retry: 'Prøv udbyderen igen',
    reviewSingle: 'Gennemgå udbyderen',
    providerNeedsAttention:
      'Udbyderen kræver opmærksomhed. Klik for at kontrollere indstillingerne.',
    builtinProfile: 'Indbygget profil',
  },
  'de-DE': {
    recoverableEyebrow: 'Vorübergehendes Anbieterproblem',
    recoverableTitle: 'Anbieter benötigt Aufmerksamkeit',
    recoverableDescription:
      'Ihr konfigurierter Anbieter ist auf einen vorübergehenden Fehler gestoßen. Prüfen Sie unten den neuesten Grund, setzen Sie den Anbieter zurück und versuchen Sie es dann erneut.',
    retry: 'Anbieter erneut versuchen',
    reviewSingle: 'Anbieter prüfen',
    providerNeedsAttention:
      'Anbieter benötigt Aufmerksamkeit. Klicken Sie, um die Einstellungen zu prüfen.',
    builtinProfile: 'Integriertes Profil',
  },
  'el-GR': {
    recoverableEyebrow: 'Προσωρινό πρόβλημα παρόχου',
    recoverableTitle: 'Ο πάροχος χρειάζεται προσοχή',
    recoverableDescription:
      'Ο ρυθμισμένος πάροχός σας αντιμετώπισε ένα προσωρινό σφάλμα. Δείτε παρακάτω την πιο πρόσφατη αιτία, επαναφέρετε τον πάροχο και δοκιμάστε ξανά.',
    retry: 'Δοκιμάστε τον πάροχο ξανά',
    reviewSingle: 'Ελέγξτε τον πάροχο',
    providerNeedsAttention:
      'Ο πάροχος χρειάζεται προσοχή. Κάντε κλικ για να ελέγξετε τις ρυθμίσεις.',
    builtinProfile: 'Ενσωματωμένο προφίλ',
  },
  'es-ES': {
    recoverableEyebrow: 'Incidencia temporal del proveedor',
    recoverableTitle: 'El proveedor necesita atención',
    recoverableDescription:
      'El proveedor configurado encontró un error temporal. Revise abajo la razón más reciente, restablezca el proveedor y vuelva a intentarlo.',
    retry: 'Reintentar el proveedor',
    reviewSingle: 'Revisar proveedor',
    providerNeedsAttention:
      'El proveedor necesita atención. Haga clic para revisar la configuración.',
    builtinProfile: 'Perfil integrado',
  },
  'fr-FR': {
    recoverableEyebrow: 'Incident temporaire du fournisseur',
    recoverableTitle: 'Le fournisseur nécessite une attention',
    recoverableDescription:
      'Le fournisseur configuré a rencontré une erreur temporaire. Vérifiez la raison la plus récente ci-dessous, réinitialisez le fournisseur, puis réessayez.',
    retry: 'Réessayer le fournisseur',
    reviewSingle: 'Vérifier le fournisseur',
    providerNeedsAttention:
      'Le fournisseur nécessite une attention. Cliquez pour vérifier les paramètres.',
    builtinProfile: 'Profil intégré',
  },
  'ga-IE': {
    recoverableEyebrow: 'Fadhb shealadach leis an soláthraí',
    recoverableTitle: 'Teastaíonn aird ón soláthraí',
    recoverableDescription:
      'Bhain earráid shealadach leis an soláthraí atá cumraithe agat. Féach ar an gcúis is déanaí thíos, athshocraigh an soláthraí agus bain triail eile as.',
    retry: 'Bain triail eile as an soláthraí',
    reviewSingle: 'Déan athbhreithniú ar an soláthraí',
    providerNeedsAttention: 'Teastaíonn aird ón soláthraí. Cliceáil chun na socruithe a sheiceáil.',
    builtinProfile: 'Próifíl ionsuite',
  },
  'hr-HR': {
    recoverableEyebrow: 'Privremeni problem s pružateljem',
    recoverableTitle: 'Pružatelj zahtijeva pažnju',
    recoverableDescription:
      'Vaš konfigurirani pružatelj naišao je na privremenu pogrešku. Pregledajte najnoviji razlog u nastavku, resetirajte pružatelja i zatim pokušajte ponovno.',
    retry: 'Pokušaj ponovno s pružateljem',
    reviewSingle: 'Pregledaj pružatelja',
    providerNeedsAttention: 'Pružatelj zahtijeva pažnju. Kliknite za provjeru postavki.',
    builtinProfile: 'Ugrađeni profil',
  },
  'hu-HU': {
    recoverableEyebrow: 'Ideiglenes szolgáltatói probléma',
    recoverableTitle: 'A szolgáltató figyelmet igényel',
    recoverableDescription:
      'A konfigurált szolgáltató ideiglenes hibába ütközött. Nézze meg alább a legutóbbi okot, állítsa vissza a szolgáltatót, majd próbálja újra.',
    retry: 'Szolgáltató újrapróbálása',
    reviewSingle: 'Szolgáltató ellenőrzése',
    providerNeedsAttention:
      'A szolgáltató figyelmet igényel. Kattintson a beállítások ellenőrzéséhez.',
    builtinProfile: 'Beépített profil',
  },
  'it-IT': {
    recoverableEyebrow: 'Problema temporaneo del provider',
    recoverableTitle: 'Il provider richiede attenzione',
    recoverableDescription:
      'Il provider configurato ha riscontrato un errore temporaneo. Controlla qui sotto il motivo più recente, reimposta il provider e poi riprova.',
    retry: 'Riprova il provider',
    reviewSingle: 'Controlla il provider',
    providerNeedsAttention:
      'Il provider richiede attenzione. Fai clic per controllare le impostazioni.',
    builtinProfile: 'Profilo integrato',
  },
  'ja-JP': {
    recoverableEyebrow: '一時的なプロバイダーの問題',
    recoverableTitle: 'プロバイダーに対応が必要です',
    recoverableDescription:
      '設定済みのプロバイダーで一時的なエラーが発生しました。下にある最新の理由を確認し、プロバイダーをリセットしてからもう一度お試しください。',
    retry: 'プロバイダーを再試行',
    reviewSingle: 'プロバイダーを確認',
    providerNeedsAttention: 'プロバイダーに対応が必要です。クリックして設定を確認してください。',
    builtinProfile: '組み込みプロファイル',
  },
  'ko-KR': {
    recoverableEyebrow: '일시적인 공급자 문제',
    recoverableTitle: '공급자 확인이 필요합니다',
    recoverableDescription:
      '구성된 공급자에서 일시적인 오류가 발생했습니다. 아래의 최신 원인을 확인하고 공급자를 재설정한 다음 다시 시도하세요.',
    retry: '공급자 다시 시도',
    reviewSingle: '공급자 검토',
    providerNeedsAttention: '공급자 확인이 필요합니다. 클릭하여 설정을 확인하세요.',
    builtinProfile: '내장 프로필',
  },
  'ml-IN': {
    recoverableEyebrow: 'താൽക്കാലിക പ്രൊവൈഡർ പ്രശ്നം',
    recoverableTitle: 'പ്രൊവൈഡറിന് ശ്രദ്ധ ആവശ്യമാണ്',
    recoverableDescription:
      'നിങ്ങൾ ക്രമീകരിച്ച പ്രൊവൈഡറിൽ താൽക്കാലിക പിശക് സംഭവിച്ചു. താഴെ കാണുന്ന പുതിയ കാരണം പരിശോധിച്ച് പ്രൊവൈഡർ റീസെറ്റ് ചെയ്ത് വീണ്ടും ശ്രമിക്കുക.',
    retry: 'പ്രൊവൈഡർ വീണ്ടും ശ്രമിക്കുക',
    reviewSingle: 'പ്രൊവൈഡർ പരിശോധിക്കുക',
    providerNeedsAttention:
      'പ്രൊവൈഡറിന് ശ്രദ്ധ ആവശ്യമാണ്. ക്രമീകരണങ്ങൾ പരിശോധിക്കാൻ ക്ലിക്ക് ചെയ്യുക.',
    builtinProfile: 'ബിൽറ്റ്-ഇൻ പ്രൊഫൈൽ',
  },
  'nb-NO': {
    recoverableEyebrow: 'Midlertidig leverandørproblem',
    recoverableTitle: 'Leverandøren trenger oppmerksomhet',
    recoverableDescription:
      'Den konfigurerte leverandøren traff en midlertidig feil. Se den nyeste årsaken nedenfor, tilbakestill leverandøren og prøv igjen.',
    retry: 'Prøv leverandøren igjen',
    reviewSingle: 'Se gjennom leverandøren',
    providerNeedsAttention:
      'Leverandøren trenger oppmerksomhet. Klikk for å kontrollere innstillingene.',
    builtinProfile: 'Innebygd profil',
  },
  'nl-NL': {
    recoverableEyebrow: 'Tijdelijk probleem met de provider',
    recoverableTitle: 'Provider heeft aandacht nodig',
    recoverableDescription:
      'Je geconfigureerde provider kreeg een tijdelijke fout. Bekijk hieronder de nieuwste reden, reset de provider en probeer het daarna opnieuw.',
    retry: 'Provider opnieuw proberen',
    reviewSingle: 'Provider controleren',
    providerNeedsAttention:
      'Provider heeft aandacht nodig. Klik om de instellingen te controleren.',
    builtinProfile: 'Ingebouwd profiel',
  },
  'pl-PL': {
    recoverableEyebrow: 'Tymczasowy problem z dostawcą',
    recoverableTitle: 'Dostawca wymaga uwagi',
    recoverableDescription:
      'Skonfigurowany dostawca napotkał tymczasowy błąd. Sprawdź poniżej najnowszy powód, zresetuj dostawcę i spróbuj ponownie.',
    retry: 'Spróbuj ponownie z dostawcą',
    reviewSingle: 'Sprawdź dostawcę',
    providerNeedsAttention: 'Dostawca wymaga uwagi. Kliknij, aby sprawdzić ustawienia.',
    builtinProfile: 'Wbudowany profil',
  },
  'pt-BR': {
    recoverableEyebrow: 'Problema temporário no provedor',
    recoverableTitle: 'O provedor precisa de atenção',
    recoverableDescription:
      'O provedor configurado encontrou um erro temporário. Revise abaixo o motivo mais recente, redefina o provedor e tente novamente.',
    retry: 'Tentar provedor novamente',
    reviewSingle: 'Revisar provedor',
    providerNeedsAttention:
      'O provedor precisa de atenção. Clique para verificar as configurações.',
    builtinProfile: 'Perfil integrado',
  },
  'pt-PT': {
    recoverableEyebrow: 'Problema temporário no fornecedor',
    recoverableTitle: 'O fornecedor precisa de atenção',
    recoverableDescription:
      'O fornecedor configurado encontrou um erro temporário. Reveja abaixo o motivo mais recente, reponha o fornecedor e tente novamente.',
    retry: 'Tentar fornecedor novamente',
    reviewSingle: 'Rever fornecedor',
    providerNeedsAttention: 'O fornecedor precisa de atenção. Clique para verificar as definições.',
    builtinProfile: 'Perfil integrado',
  },
  'ro-RO': {
    recoverableEyebrow: 'Problemă temporară a furnizorului',
    recoverableTitle: 'Furnizorul necesită atenție',
    recoverableDescription:
      'Furnizorul configurat a întâmpinat o eroare temporară. Verificați mai jos cel mai recent motiv, resetați furnizorul și încercați din nou.',
    retry: 'Reîncearcă furnizorul',
    reviewSingle: 'Verifică furnizorul',
    providerNeedsAttention: 'Furnizorul necesită atenție. Faceți clic pentru a verifica setările.',
    builtinProfile: 'Profil integrat',
  },
  'ru-RU': {
    recoverableEyebrow: 'Временная проблема с провайдером',
    recoverableTitle: 'Провайдеру требуется внимание',
    recoverableDescription:
      'У настроенного провайдера возникла временная ошибка. Проверьте ниже последнюю причину, сбросьте провайдера и попробуйте снова.',
    retry: 'Повторить попытку с провайдером',
    reviewSingle: 'Проверить провайдера',
    providerNeedsAttention: 'Провайдеру требуется внимание. Нажмите, чтобы проверить настройки.',
    builtinProfile: 'Встроенный профиль',
  },
  'sk-SK': {
    recoverableEyebrow: 'Dočasný problém poskytovateľa',
    recoverableTitle: 'Poskytovateľ vyžaduje pozornosť',
    recoverableDescription:
      'Nakonfigurovaný poskytovateľ narazil na dočasnú chybu. Nižšie skontrolujte posledný dôvod, resetujte poskytovateľa a potom to skúste znova.',
    retry: 'Skúsiť poskytovateľa znova',
    reviewSingle: 'Skontrolovať poskytovateľa',
    providerNeedsAttention: 'Poskytovateľ vyžaduje pozornosť. Kliknutím skontrolujte nastavenia.',
    builtinProfile: 'Vstavaný profil',
  },
  'sv-SE': {
    recoverableEyebrow: 'Tillfälligt leverantörsproblem',
    recoverableTitle: 'Leverantören behöver uppmärksamhet',
    recoverableDescription:
      'Din konfigurerade leverantör råkade ut för ett tillfälligt fel. Granska den senaste orsaken nedan, återställ leverantören och försök igen.',
    retry: 'Försök med leverantören igen',
    reviewSingle: 'Granska leverantören',
    providerNeedsAttention:
      'Leverantören behöver uppmärksamhet. Klicka för att kontrollera inställningarna.',
    builtinProfile: 'Inbyggd profil',
  },
}

type RoutingModePinnedLocalePatch = {
  providerPinnedTitle: string
  providerPinnedDesc: string
  providerPinnedReset: string
}

const routingModePinnedBackfills: Partial<Record<LocaleKey, RoutingModePinnedLocalePatch>> = {
  'ca-ES': {
    providerPinnedTitle: 'Proveïdor fixat',
    providerPinnedDesc:
      'Aquesta conversa es manté a {provider}, però el model continua en mode automàtic.',
    providerPinnedReset: 'Fes servir tots els proveïdors',
  },
  'cs-CZ': {
    providerPinnedTitle: 'Připnutý poskytovatel',
    providerPinnedDesc: 'Tato konverzace zůstává u {provider}, ale model je stále automatický.',
    providerPinnedReset: 'Použít všechny poskytovatele',
  },
  'da-DK': {
    providerPinnedTitle: 'Fastgjort udbyder',
    providerPinnedDesc: 'Denne samtale bliver hos {provider}, men modellen er stadig automatisk.',
    providerPinnedReset: 'Brug alle udbydere',
  },
  'de-DE': {
    providerPinnedTitle: 'Angehefteter Anbieter',
    providerPinnedDesc:
      'Diese Unterhaltung bleibt bei {provider}, aber das Modell bleibt weiterhin automatisch.',
    providerPinnedReset: 'Alle Anbieter verwenden',
  },
  'el-GR': {
    providerPinnedTitle: 'Καρφιτσωμένος πάροχος',
    providerPinnedDesc:
      'Αυτή η συνομιλία παραμένει στο {provider}, αλλά το μοντέλο εξακολουθεί να είναι αυτόματο.',
    providerPinnedReset: 'Χρήση όλων των παρόχων',
  },
  'en-GB': {
    providerPinnedTitle: 'Pinned Provider',
    providerPinnedDesc: 'This conversation stays on {provider}, but the model is still automatic.',
    providerPinnedReset: 'Use all providers',
  },
  'es-ES': {
    providerPinnedTitle: 'Proveedor fijado',
    providerPinnedDesc:
      'Esta conversación se mantiene en {provider}, pero el modelo sigue siendo automático.',
    providerPinnedReset: 'Usar todos los proveedores',
  },
  'fr-FR': {
    providerPinnedTitle: 'Fournisseur épinglé',
    providerPinnedDesc:
      'Cette conversation reste sur {provider}, mais le modèle reste automatique.',
    providerPinnedReset: 'Utiliser tous les fournisseurs',
  },
  'ga-IE': {
    providerPinnedTitle: 'Soláthraí greamaithe',
    providerPinnedDesc: 'Fanann an comhrá seo ar {provider}, ach tá an tsamhail fós uathoibríoch.',
    providerPinnedReset: 'Úsáid gach soláthraí',
  },
  'hr-HR': {
    providerPinnedTitle: 'Prikvačeni pružatelj',
    providerPinnedDesc: 'Ovaj razgovor ostaje na {provider}, ali model je i dalje automatski.',
    providerPinnedReset: 'Koristi sve pružatelje',
  },
  'hu-HU': {
    providerPinnedTitle: 'Rögzített szolgáltató',
    providerPinnedDesc:
      'Ez a beszélgetés a(z) {provider} szolgáltatón marad, de a modell továbbra is automatikus.',
    providerPinnedReset: 'Összes szolgáltató használata',
  },
  'it-IT': {
    providerPinnedTitle: 'Provider bloccato',
    providerPinnedDesc:
      'Questa conversazione resta su {provider}, ma il modello è ancora automatico.',
    providerPinnedReset: 'Usa tutti i provider',
  },
  'ja-JP': {
    providerPinnedTitle: '固定されたプロバイダー',
    providerPinnedDesc: 'この会話は {provider} に固定されますが、モデルは引き続き自動です。',
    providerPinnedReset: 'すべてのプロバイダーを使う',
  },
  'ko-KR': {
    providerPinnedTitle: '고정된 공급자',
    providerPinnedDesc: '이 대화는 {provider}에 고정되지만 모델은 계속 자동입니다.',
    providerPinnedReset: '모든 공급자 사용',
  },
  'ml-IN': {
    providerPinnedTitle: 'പിൻ ചെയ്ത പ്രൊവൈഡർ',
    providerPinnedDesc:
      'ഈ സംഭാഷണം {provider} ല്‍ തന്നെയിരിക്കും, പക്ഷേ മോഡൽ ഇപ്പോഴും ഓട്ടോമാറ്റിക്കാണ്.',
    providerPinnedReset: 'എല്ലാ പ്രൊവൈഡറുകളും ഉപയോഗിക്കുക',
  },
  'nb-NO': {
    providerPinnedTitle: 'Festet leverandør',
    providerPinnedDesc:
      'Denne samtalen holder seg på {provider}, men modellen er fortsatt automatisk.',
    providerPinnedReset: 'Bruk alle leverandører',
  },
  'nl-NL': {
    providerPinnedTitle: 'Vastgezette provider',
    providerPinnedDesc: 'Dit gesprek blijft op {provider}, maar het model blijft automatisch.',
    providerPinnedReset: 'Gebruik alle providers',
  },
  'pl-PL': {
    providerPinnedTitle: 'Przypięty dostawca',
    providerPinnedDesc:
      'Ta rozmowa pozostaje przy {provider}, ale model nadal działa automatycznie.',
    providerPinnedReset: 'Użyj wszystkich dostawców',
  },
  'pt-BR': {
    providerPinnedTitle: 'Provedor fixado',
    providerPinnedDesc: 'Esta conversa fica em {provider}, mas o modelo continua automático.',
    providerPinnedReset: 'Usar todos os provedores',
  },
  'pt-PT': {
    providerPinnedTitle: 'Fornecedor fixado',
    providerPinnedDesc: 'Esta conversa fica em {provider}, mas o modelo continua automático.',
    providerPinnedReset: 'Usar todos os fornecedores',
  },
  'ro-RO': {
    providerPinnedTitle: 'Furnizor fixat',
    providerPinnedDesc:
      'Această conversație rămâne pe {provider}, dar modelul este în continuare automat.',
    providerPinnedReset: 'Folosește toți furnizorii',
  },
  'ru-RU': {
    providerPinnedTitle: 'Закреплённый провайдер',
    providerPinnedDesc:
      'Этот чат остаётся на {provider}, но модель по-прежнему выбирается автоматически.',
    providerPinnedReset: 'Использовать всех провайдеров',
  },
  'sk-SK': {
    providerPinnedTitle: 'Pripnutý poskytovateľ',
    providerPinnedDesc: 'Tento rozhovor zostáva na {provider}, ale model je stále automatický.',
    providerPinnedReset: 'Použiť všetkých poskytovateľov',
  },
  'sv-SE': {
    providerPinnedTitle: 'Fäst leverantör',
    providerPinnedDesc:
      'Den här konversationen stannar på {provider}, men modellen är fortfarande automatisk.',
    providerPinnedReset: 'Använd alla leverantörer',
  },
}

const failoverHealthyLabels: Partial<Record<LocaleKey, string>> = {
  'cs-CZ': 'V pořádku',
  'da-DK': 'Sund',
  'de-DE': 'Gesund',
  'el-GR': 'Υγιές',
  'es-ES': 'Saludable',
  'fr-FR': 'Sain',
  'hu-HU': 'Egészséges',
  'it-IT': 'Sano',
  'nb-NO': 'Frisk',
  'nl-NL': 'Gezond',
  'pl-PL': 'Zdrowy',
  'pt-BR': 'Saudável',
  'pt-PT': 'Saudável',
  'ro-RO': 'Sănătos',
  'ru-RU': 'Работает',
  'sk-SK': 'V poriadku',
  'sv-SE': 'Frisk',
}

const localizedWebTermLabels: Partial<Record<LocaleKey, string>> = {
  'ca-ES': 'Lloc web',
  'cs-CZ': 'Webová stránka',
  'da-DK': 'Webside',
  'de-DE': 'Webseite',
  'es-ES': 'Sitio web',
  'fr-FR': 'Site web',
  'hr-HR': 'Web-stranica',
  'hu-HU': 'Weboldal',
  'it-IT': 'Sito web',
  'nb-NO': 'Nettsted',
  'nl-NL': 'Website',
  'pl-PL': 'Strona WWW',
  'pt-BR': 'Site',
  'pt-PT': 'Site',
  'ro-RO': 'Site web',
  'sk-SK': 'Webová stránka',
}

const localizedCacheTermLabels: Partial<Record<LocaleKey, string>> = {
  'cs-CZ': 'Mezipaměť',
  'da-DK': 'Mellemlager',
  'de-DE': 'Zwischenspeicher',
  'fr-FR': 'Mémoire cache',
  'it-IT': 'Memoria cache',
  'nb-NO': 'Mellomlager',
  'nl-NL': 'Cachegeheugen',
  'pt-BR': 'Memória cache',
  'pt-PT': 'Memória cache',
  'ro-RO': 'Memorie cache',
  'sk-SK': 'Vyrovnávacia pamäť',
  'sv-SE': 'Cacheminne',
}

const localizedDirectoryWhitelistAliasLabels: Partial<Record<LocaleKey, string>> = {
  'da-DK': 'Aliasnavn',
  'de-DE': 'Aliasname',
  'es-ES': 'Nombre alternativo',
  'fr-FR': 'Nom alternatif',
  'hr-HR': 'Pseudonim',
  'it-IT': 'Nome alternativo',
  'nb-NO': 'Aliasnavn',
  'nl-NL': 'Aliasnaam',
  'pl-PL': 'Pseudonim',
  'pt-BR': 'Apelido',
  'pt-PT': 'Apelido',
  'ro-RO': 'Pseudonim',
  'sk-SK': 'Prezývka',
  'sv-SE': 'Aliasnamn',
}

const localizedOnlineLabels: Partial<Record<LocaleKey, string>> = {
  'cs-CZ': 'Připojeno',
  'da-DK': 'Forbundet',
  'de-DE': 'Verbunden',
  'hr-HR': 'Povezano',
  'hu-HU': 'Kapcsolódva',
  'nb-NO': 'Tilkoblet',
  'nl-NL': 'Verbonden',
  'ro-RO': 'Conectat',
  'sk-SK': 'Pripojené',
  'sv-SE': 'Ansluten',
}

const localizedBackendLabels: Partial<Record<LocaleKey, string>> = {
  'ca-ES': 'Back-end',
  'cs-CZ': 'Back-end',
  'da-DK': 'Bagende',
  'de-DE': 'Back-end',
  'el-GR': 'Παρασκήνιο',
  'it-IT': 'Back-end',
  'nb-NO': 'Bakende',
  'ro-RO': 'Back-end',
  'sk-SK': 'Back-end',
  'sv-SE': 'Bakände',
}

const webQueryResultCardLabelBackfills: Record<LocaleKey, ResultCardWebQueryLabelBackfill> = {
  'ca-ES': {
    has_results: 'Té resultats',
    selected_result: 'Resultat seleccionat',
    key_facts: 'Fets clau',
    research_artifact_path: "Ruta de l'artefacte de recerca",
    llm_compacted: 'Compactat per LLM',
    materialized: 'Materialitzat',
    mode: 'Mode',
  },
  'cs-CZ': {
    has_results: 'Má výsledky',
    selected_result: 'Vybraný výsledek',
    key_facts: 'Klíčová fakta',
    research_artifact_path: 'Cesta k artefaktu výzkumu',
    llm_compacted: 'Zkompaktováno LLM',
    materialized: 'Materializováno',
    mode: 'Režim',
  },
  'da-DK': {
    has_results: 'Har resultater',
    selected_result: 'Valgt resultat',
    key_facts: 'Nøglefakta',
    research_artifact_path: 'Sti til research-artefakt',
    llm_compacted: 'Komprimeret af LLM',
    materialized: 'Materialiseret',
    mode: 'Tilstand',
  },
  'de-DE': {
    has_results: 'Hat Ergebnisse',
    selected_result: 'Ausgewähltes Ergebnis',
    key_facts: 'Kernfakten',
    research_artifact_path: 'Pfad zum Recherche-Artefakt',
    llm_compacted: 'Durch LLM komprimiert',
    materialized: 'Materialisiert',
    mode: 'Modus',
  },
  'el-GR': {
    has_results: 'Έχει αποτελέσματα',
    selected_result: 'Επιλεγμένο αποτέλεσμα',
    key_facts: 'Βασικά στοιχεία',
    research_artifact_path: 'Διαδρομή τεχνήματος έρευνας',
    llm_compacted: 'Συμπτυγμένο από LLM',
    materialized: 'Υλοποιημένο',
    mode: 'Λειτουργία',
  },
  'en-GB': {
    has_results: 'Has Results',
    selected_result: 'Selected Result',
    key_facts: 'Key Facts',
    research_artifact_path: 'Research Artifact Path',
    llm_compacted: 'LLM Compacted',
    materialized: 'Materialized',
    mode: 'Mode',
  },
  'en-US': {
    has_results: 'Has Results',
    selected_result: 'Selected Result',
    key_facts: 'Key Facts',
    research_artifact_path: 'Research Artifact Path',
    llm_compacted: 'LLM Compacted',
    materialized: 'Materialized',
    mode: 'Mode',
  },
  'es-ES': {
    has_results: 'Tiene resultados',
    selected_result: 'Resultado seleccionado',
    key_facts: 'Datos clave',
    research_artifact_path: 'Ruta del artefacto de investigacion',
    llm_compacted: 'Compactado por LLM',
    materialized: 'Materializado',
    mode: 'Modo',
  },
  'fr-FR': {
    has_results: 'Contient des resultats',
    selected_result: 'Resultat selectionne',
    key_facts: 'Faits cles',
    research_artifact_path: "Chemin de l'artefact de recherche",
    llm_compacted: 'Compacte par le LLM',
    materialized: 'Materialise',
    mode: 'Mode',
  },
  'ga-IE': {
    has_results: 'Tá torthaí ann',
    selected_result: 'Toradh roghnaithe',
    key_facts: 'Príomhfhíricí',
    research_artifact_path: 'Conair go déantán taighde',
    llm_compacted: 'Comhdhlúite ag LLM',
    materialized: 'Cruthaithe',
    mode: 'Mód',
  },
  'hr-HR': {
    has_results: 'Ima rezultate',
    selected_result: 'Odabrani rezultat',
    key_facts: 'Ključne činjenice',
    research_artifact_path: 'Putanja artefakta istraživanja',
    llm_compacted: 'Kompaktirano LLM-om',
    materialized: 'Materijalizirano',
    mode: 'Način',
  },
  'hu-HU': {
    has_results: 'Van találat',
    selected_result: 'Kiválasztott eredmény',
    key_facts: 'Kulcstények',
    research_artifact_path: 'Kutatási artefaktum útvonala',
    llm_compacted: 'LLM által tömörítve',
    materialized: 'Materializálva',
    mode: 'Mód',
  },
  'it-IT': {
    has_results: 'Ha risultati',
    selected_result: 'Risultato selezionato',
    key_facts: 'Fatti chiave',
    research_artifact_path: "Percorso dell'artefatto di ricerca",
    llm_compacted: 'Compattato da LLM',
    materialized: 'Materializzato',
    mode: 'Modalità',
  },
  'ja-JP': {
    has_results: '結果あり',
    selected_result: '選択結果',
    key_facts: '重要事項',
    research_artifact_path: '調査成果物パス',
    llm_compacted: 'LLM圧縮済み',
    materialized: '実体化済み',
    mode: 'モード',
  },
  'ko-KR': {
    has_results: '결과 있음',
    selected_result: '선택된 결과',
    key_facts: '핵심 사실',
    research_artifact_path: '조사 아티팩트 경로',
    llm_compacted: 'LLM 압축됨',
    materialized: '생성됨',
    mode: '모드',
  },
  'ml-IN': {
    has_results: 'ഫലങ്ങളുണ്ട്',
    selected_result: 'തിരഞ്ഞെടുത്ത ഫലം',
    key_facts: 'പ്രധാന വിവരങ്ങള്',
    research_artifact_path: 'ഗവേഷണ ആര്‍ട്ടിഫാക്റ്റ് പാത',
    llm_compacted: 'LLM ചുരുക്കിയത്',
    materialized: 'സൃഷ്ടിച്ചത്',
    mode: 'മോഡ്',
  },
  'nb-NO': {
    has_results: 'Har resultater',
    selected_result: 'Valgt resultat',
    key_facts: 'Nøkkelfakta',
    research_artifact_path: 'Sti til forskningsartefakt',
    llm_compacted: 'Komprimert av LLM',
    materialized: 'Materialisert',
    mode: 'Modus',
  },
  'nl-NL': {
    has_results: 'Heeft resultaten',
    selected_result: 'Geselecteerd resultaat',
    key_facts: 'Kernfeiten',
    research_artifact_path: 'Pad naar onderzoeksartefact',
    llm_compacted: 'Gecompacteerd door LLM',
    materialized: 'Gematerialiseerd',
    mode: 'Modus',
  },
  'pl-PL': {
    has_results: 'Ma wyniki',
    selected_result: 'Wybrany wynik',
    key_facts: 'Kluczowe fakty',
    research_artifact_path: 'Sciezka artefaktu badawczego',
    llm_compacted: 'Skondensowane przez LLM',
    materialized: 'Zmaterializowane',
    mode: 'Tryb',
  },
  'pt-BR': {
    has_results: 'Tem resultados',
    selected_result: 'Resultado selecionado',
    key_facts: 'Fatos-chave',
    research_artifact_path: 'Caminho do artefato de pesquisa',
    llm_compacted: 'Compactado por LLM',
    materialized: 'Materializado',
    mode: 'Modo',
  },
  'pt-PT': {
    has_results: 'Tem resultados',
    selected_result: 'Resultado selecionado',
    key_facts: 'Factos-chave',
    research_artifact_path: 'Caminho do artefacto de pesquisa',
    llm_compacted: 'Compactado por LLM',
    materialized: 'Materializado',
    mode: 'Modo',
  },
  'ro-RO': {
    has_results: 'Are rezultate',
    selected_result: 'Rezultat selectat',
    key_facts: 'Fapte cheie',
    research_artifact_path: 'Calea artefactului de cercetare',
    llm_compacted: 'Compactat de LLM',
    materialized: 'Materializat',
    mode: 'Mod',
  },
  'ru-RU': {
    has_results: 'Есть результаты',
    selected_result: 'Выбранный результат',
    key_facts: 'Ключевые факты',
    research_artifact_path: 'Путь к артефакту исследования',
    llm_compacted: 'Сжато LLM',
    materialized: 'Материализовано',
    mode: 'Режим',
  },
  'sk-SK': {
    has_results: 'Má výsledky',
    selected_result: 'Vybraný výsledok',
    key_facts: 'Kľúčové fakty',
    research_artifact_path: 'Cesta k artefaktu výskumu',
    llm_compacted: 'Zhutnené pomocou LLM',
    materialized: 'Materializované',
    mode: 'Režim',
  },
  'sv-SE': {
    has_results: 'Har resultat',
    selected_result: 'Valt resultat',
    key_facts: 'Nyckelfakta',
    research_artifact_path: 'Sökväg till forskningsartefakt',
    llm_compacted: 'Kompakterad av LLM',
    materialized: 'Materialiserad',
    mode: 'Läge',
  },
  'zh-CN': {
    has_results: '有结果',
    selected_result: '已选结果',
    key_facts: '关键信息',
    research_artifact_path: '研究产物路径',
    llm_compacted: 'LLM 压缩',
    materialized: '已生成',
    mode: '模式',
  },
  'zh-TW': {
    has_results: '有結果',
    selected_result: '已選結果',
    key_facts: '關鍵資訊',
    research_artifact_path: '研究產物路徑',
    llm_compacted: 'LLM 壓縮',
    materialized: '已生成',
    mode: '模式',
  },
}

const localizedMetricsMinLabels: Partial<Record<LocaleKey, string>> = {
  'ca-ES': 'Mín.',
  'cs-CZ': 'Min.',
  'da-DK': 'Min.',
  'de-DE': 'Min.',
  'ga-IE': 'Íos.',
  'hr-HR': 'Min.',
  'hu-HU': 'Min.',
  'nb-NO': 'Min.',
  'nl-NL': 'Min.',
  'pl-PL': 'Min.',
  'ro-RO': 'Min.',
  'sk-SK': 'Min.',
  'sv-SE': 'Min.',
}

const localizedTaskKindAgentLabels: Partial<Record<LocaleKey, string>> = {
  'ca-ES': 'Agent IA',
  'cs-CZ': 'AI agent',
  'da-DK': 'AI-agent',
  'de-DE': 'KI-Agent',
  'fr-FR': 'Agent IA',
  'hr-HR': 'AI agent',
  'nb-NO': 'KI-agent',
  'nl-NL': 'AI-agent',
  'pl-PL': 'Agent AI',
  'ro-RO': 'Agent AI',
  'sv-SE': 'AI-agent',
}

const localizedCliEtaLabels: Partial<Record<LocaleKey, string>> = {
  'ca-ES': 'Temps estimat: {time}',
  'da-DK': 'Anslået tid: {time}',
  'de-DE': 'Geschätzte Zeit: {time}',
  'hr-HR': 'Procijenjeno vrijeme: {time}',
  'it-IT': 'Tempo stimato: {time}',
  'ml-IN': 'കണക്കാക്കിയ സമയം: {time}',
  'nb-NO': 'Estimert tid: {time}',
  'pl-PL': 'Szacowany czas: {time}',
  'ro-RO': 'Timp estimat: {time}',
  'sk-SK': 'Odhadovaný čas: {time}',
  'sv-SE': 'Beräknad tid: {time}',
}

const localizedWorkspaceTreeDirCountLabels: Partial<Record<LocaleKey, string>> = {
  'ca-ES': 'directoris',
  'cs-CZ': 'adresáře',
  'da-DK': 'mapper',
  'hr-HR': 'mape',
  'hu-HU': 'könyvtárak',
  'nb-NO': 'mapper',
  'nl-NL': 'mappen',
  'ro-RO': 'directoare',
  'sk-SK': 'adresáre',
  'sv-SE': 'mappar',
}

const localizedMountPointLabels: Partial<Record<LocaleKey, string>> = {
  'ca-ES': 'Punt de muntatge',
  'cs-CZ': 'Přípojný bod',
  'da-DK': 'Monteringspunkt',
  'de-DE': 'Einhängepunkt',
  'el-GR': 'Σημείο προσάρτησης',
  'hu-HU': 'Csatolási pont',
  'nb-NO': 'Monteringspunkt',
  'ro-RO': 'Punct de montare',
  'sk-SK': 'Prípojný bod',
  'sv-SE': 'Monteringspunkt',
}

const localizedAdminLabels: Partial<Record<LocaleKey, string>> = {
  'ca-ES': 'Administrador',
  'cs-CZ': 'Správce',
  'da-DK': 'Administrator',
  'de-DE': 'Administrator',
  'hu-HU': 'Rendszergazda',
  'nb-NO': 'Administrator',
  'ro-RO': 'Administrator',
  'sk-SK': 'Správca',
  'sv-SE': 'Administratör',
}

const localizedVideoPreviewLabels: Partial<Record<LocaleKey, string>> = {
  'cs-CZ': 'Videozáznam',
  'da-DK': 'Videofil',
  'de-DE': 'Videodatei',
  'hr-HR': 'Videozapis',
  'it-IT': 'File video',
  'nb-NO': 'Videofil',
  'nl-NL': 'Videobestand',
  'ro-RO': 'Fișier video',
  'sk-SK': 'Videozáznam',
  'sv-SE': 'Videofil',
}

const localizedGoroutineLabels: Partial<Record<LocaleKey, string>> = {
  'ca-ES': 'Rutines',
  'cs-CZ': 'Goroutiny',
  'fr-FR': 'Routines',
  'ga-IE': 'Gnáthaimh',
  'hr-HR': 'Rutine',
  'nl-NL': 'Routines',
  'pt-BR': 'Rotinas',
  'pt-PT': 'Rotinas',
  'sk-SK': 'Gorutiny',
}

const localizedGoroutineChartLabels: Partial<Record<LocaleKey, string>> = {
  'ca-ES': 'Rutines',
  'ga-IE': 'Gnáthaimh',
  'hr-HR': 'Rutine',
  'nl-NL': 'Routines',
}

const localizedAuthTokenLabels: Partial<Record<LocaleKey, string>> = {
  'cs-CZ': 'Autentizační token',
  'da-DK': 'Godkendelsestoken',
  'hr-HR': 'Token za autentikaciju',
  'hu-HU': 'Hitelesítési token',
  'nb-NO': 'Autentiseringstoken',
  'sk-SK': 'Autentifikačný token',
  'sv-SE': 'Autentiseringstoken',
}

const localizedViberAuthTokenPlaceholders: Partial<Record<LocaleKey, string>> = {
  'cs-CZ': 'Autentizační token pro Viber',
  'da-DK': 'Viber-godkendelsestoken',
  'el-GR': 'Διακριτικό ελέγχου ταυτότητας Viber',
  'hr-HR': 'Viber token za autentikaciju',
  'hu-HU': 'Viber hitelesítési token',
  'nb-NO': 'Viber-autentiseringstoken',
  'ro-RO': 'Jeton de autentificare Viber',
  'sk-SK': 'Autentifikačný token pre Viber',
  'sv-SE': 'Viber-autentiseringstoken',
}

const localizedSystemInfoLabels: Partial<Record<LocaleKey, string>> = {
  'cs-CZ': 'Informace',
  'da-DK': 'Information',
  'hr-HR': 'Informacije',
  'hu-HU': 'Információ',
  'nb-NO': 'Informasjon',
  'ro-RO': 'Informații',
  'sk-SK': 'Informácie',
  'sv-SE': 'Information',
}

const localizedKernelLabels: Partial<Record<LocaleKey, string>> = {
  'cs-CZ': 'Jádro',
  'da-DK': 'Kerne',
  'de-DE': 'Kern',
  'hr-HR': 'Jezgra',
  'hu-HU': 'Rendszermag',
  'nl-NL': 'Kern',
  'ro-RO': 'Nucleu',
  'sk-SK': 'Jadro',
}

const localizedHeapAllocLabels: Partial<Record<LocaleKey, string>> = {
  'da-DK': 'Heap-allokering',
  'nb-NO': 'Heap-allokering',
}

function isPlainObject(value: unknown): value is LocaleNode {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

function getString(root: unknown, path: string): string | null {
  const value = getValue(root, path)

  return typeof value === 'string' && value.length > 0 ? value : null
}

function getValue(root: unknown, path: string): unknown {
  return path.split('.').reduce<unknown>((current, segment) => {
    if (isPlainObject(current)) {
      return current[segment]
    }
    return undefined
  }, root)
}

function hasKeys(node: LocaleNode): boolean {
  return Object.keys(node).length > 0
}

function buildHarnessTermGlossary(messages: Record<string, unknown>): HarnessTermGlossary | null {
  const preset = getString(messages, 'harness.terms.preset')
  const dataset = getString(messages, 'harness.terms.dataset')
  const version = getString(messages, 'harness.terms.version')
  const spec = getString(messages, 'harness.terms.spec')
  const run = getString(messages, 'harness.terms.run')
  const baseline = getString(messages, 'harness.terms.baseline')
  const group = getString(messages, 'harness.terms.group')
  const scorecard = getString(messages, 'harness.terms.scorecard')
  const agentTask = getString(messages, 'harness.terms.agentTask')
  const prepare = getString(messages, 'harness.terms.prepare')

  if (
    !preset ||
    !dataset ||
    !version ||
    !spec ||
    !run ||
    !baseline ||
    !group ||
    !scorecard ||
    !agentTask ||
    !prepare
  ) {
    return null
  }

  return {
    preset,
    dataset,
    version,
    spec,
    run,
    baseline,
    group,
    scorecard,
    agentTask,
    prepare,
  }
}

function replaceHarnessTermsInText(text: string, glossary: HarnessTermGlossary): string {
  const replacements: Array<[RegExp, string]> = [
    [/\bAgent Task\b|\bagent_task\b/g, glossary.agentTask],
    [/\bPrepare\b/g, glossary.prepare],
    [/\bpresets?\b/gi, glossary.preset],
    [/\bdatasets?\b/gi, glossary.dataset],
    [/\bversions?\b/gi, glossary.version],
    [/\bspecs?\b/gi, glossary.spec],
    [/\bruns?\b/gi, glossary.run],
    [/\bbaselines?\b/gi, glossary.baseline],
    [/\bgroups?\b/gi, glossary.group],
    [/\bscorecards?\b/gi, glossary.scorecard],
  ]

  return replacements.reduce(
    (current, [pattern, replacement]) => current.replace(pattern, replacement),
    text
  )
}

function buildStringReplacementPatch(
  value: unknown,
  replace: (text: string) => string,
  options?: {
    excludedPaths?: ReadonlySet<string>
    pathPrefix?: string
  },
  currentPath = options?.pathPrefix ?? ''
): LocaleNode | LocaleLeaf | null {
  if (typeof value === 'string') {
    if (options?.excludedPaths?.has(currentPath)) {
      return null
    }
    const replaced = replace(value)
    return replaced === value ? null : replaced
  }
  if (!isPlainObject(value)) return null

  const patch: LocaleNode = {}
  for (const [key, child] of Object.entries(value)) {
    const childPath = currentPath ? `${currentPath}.${key}` : key
    const childPatch = buildStringReplacementPatch(child, replace, options, childPath)
    if (childPatch !== null) {
      patch[key] = childPatch
    }
  }

  return hasKeys(patch) ? patch : null
}

export function buildLocalePostMergeBackfill(
  localeKey: LocaleKey,
  messages: Record<string, unknown>
): LocaleNode {
  const cliPatch: LocaleNode = {}
  const deepResearchRuntimeCopy = deepResearchRuntimeBackfills[localeKey]
  const deepResearchSummaryCopy = deepResearchSummaryBackfills[localeKey]
  const deepResearchStructuredCopy = deepResearchStructuredBackfills[localeKey]
  const deepResearchTokenCopy = deepResearchTokenBackfills[localeKey]
  const commonPatch: LocaleNode = {}
  const channelsPatch: LocaleNode = {}
  const dashboardPatch: LocaleNode = {}
  const dashboardCardsPatch: LocaleNode = {}
  const execCardPatch: LocaleNode = {}
  const metricsPatch: LocaleNode = {}
  const settingsPatch: LocaleNode = {}
  const failoverPatch: LocaleNode = {}
  const failoverChipsPatch: LocaleNode = {}
  const failoverErrorTypesPatch: LocaleNode = {}
  const apiProxyPatch: LocaleNode = {}
  const chatPatch: LocaleNode = {}
  const noProviderPatch: LocaleNode = {}
  const navPatch: LocaleNode = {}
  const profilePatch: LocaleNode = {}
  const resultCardPatch: LocaleNode = {}
  const resultCardLabelsPatch: LocaleNode = {}
  const routingModePatch: LocaleNode = {}
  const companionPatch: LocaleNode = {}
  const companionPlatformsPatch: LocaleNode = {}
  const externalAgentsPatch: LocaleNode = {}
  const securityPatch: LocaleNode = {}
  const skillStoreSignalsPatch: LocaleNode = {}
  const speechPatch: LocaleNode = {}
  const speechConvertTaskPatch: LocaleNode = {}
  const speechConvertTaskPreviewKindPatch: LocaleNode = {}
  const systemPatch: LocaleNode = {}
  const tokenEconomyPatch: LocaleNode = {}
  const toolsPatch: LocaleNode = {}
  const toolsNamesPatch: LocaleNode = {}
  const userdataPatch: LocaleNode = {}
  const userdataMemoryPatch: LocaleNode = {}
  const usersPatch: LocaleNode = {}
  const usersRolesPatch: LocaleNode = {}
  const harnessTermGlossary = buildHarnessTermGlossary(messages)

  const mirrors: Array<[LocaleNode, string, string | null]> = [
    [
      failoverChipsPatch,
      'healthy',
      (() => {
        const currentHealthy = getString(messages, 'settings.failover.chips.healthy')
        const dashboardHealthy = getString(messages, 'dashboard.healthy')
        const localizedHealthy = failoverHealthyLabels[localeKey]
        if (
          localizedHealthy &&
          (!currentHealthy || currentHealthy === 'Healthy' || currentHealthy === 'OK')
        ) {
          return localizedHealthy
        }
        if (currentHealthy && currentHealthy !== 'Healthy' && currentHealthy !== 'OK') {
          return currentHealthy
        }
        return dashboardHealthy ?? currentHealthy
      })(),
    ],
    [failoverChipsPatch, 'halfOpen', getString(messages, 'settings.failover.chips.halfOpen')],
    [failoverChipsPatch, 'open', getString(messages, 'settings.failover.chips.open')],
    [
      failoverErrorTypesPatch,
      'timeout',
      getString(messages, 'settings.failover.errorTypes.timeout'),
    ],
    [
      failoverErrorTypesPatch,
      'unknown',
      getString(messages, 'settings.failover.errorTypes.unknown'),
    ],
    [execCardPatch, 'noCommand', execCardNoCommandLabels[localeKey] ?? null],
  ]

  for (const [target, key, value] of mirrors) {
    if (value) {
      target[key] = value
    }
  }

  if (hasKeys(failoverChipsPatch)) {
    failoverPatch.chips = failoverChipsPatch
  }
  if (hasKeys(failoverErrorTypesPatch)) {
    failoverPatch.errorTypes = failoverErrorTypesPatch
  }
  if (hasKeys(failoverPatch)) {
    settingsPatch.failover = failoverPatch
  }
  if (harnessTermGlossary) {
    const agentcoreRunnerPatch = buildStringReplacementPatch(
      getValue(messages, 'settings.agentcoreRunner'),
      (text) => replaceHarnessTermsInText(text, harnessTermGlossary)
    )
    if (isPlainObject(agentcoreRunnerPatch) && hasKeys(agentcoreRunnerPatch)) {
      settingsPatch.agentcoreRunner = agentcoreRunnerPatch
    }
  }

  const patch: LocaleNode = {}
  if (harnessTermGlossary) {
    const harnessPatch = buildStringReplacementPatch(
      getValue(messages, 'harness'),
      (text) => replaceHarnessTermsInText(text, harnessTermGlossary),
      {
        excludedPaths: protectedHarnessTermReplacementPaths,
        pathPrefix: 'harness',
      }
    )
    if (isPlainObject(harnessPatch) && hasKeys(harnessPatch)) {
      patch.harness = harnessPatch
    }
  }
  if (hasKeys(settingsPatch)) {
    patch.settings = settingsPatch
  }
  if (deepResearchStructuredCopy) {
    chatPatch.deepResearchSourceTypeLaw = deepResearchStructuredCopy.sourceTypeLaw
    chatPatch.deepResearchSourceTypeFiling = deepResearchStructuredCopy.sourceTypeFiling
    chatPatch.deepResearchSourceTypePaper = deepResearchStructuredCopy.sourceTypePaper
    chatPatch.deepResearchSourceTypeWeb = deepResearchStructuredCopy.sourceTypeWeb
    chatPatch.deepResearchMetaOfficial = deepResearchStructuredCopy.metaOfficial
    chatPatch.deepResearchMetaFinancial = deepResearchStructuredCopy.metaFinancial
    chatPatch.deepResearchWorkflowScope = deepResearchStructuredCopy.workflowScope
    chatPatch.deepResearchWorkflowSources = deepResearchStructuredCopy.workflowSources
    chatPatch.deepResearchWorkflowExtraction = deepResearchStructuredCopy.workflowExtraction
  }
  if (deepResearchRuntimeCopy) {
    chatPatch.deepResearchRetainedSearches = deepResearchRuntimeCopy.retainedSearches
    chatPatch.deepResearchReportStyleKnowledgeBase =
      deepResearchRuntimeCopy.reportStyleKnowledgeBase
    chatPatch.deepResearchActionResearchBriefPrepared =
      deepResearchRuntimeCopy.researchBriefPrepared
    chatPatch.deepResearchActionDraftSynthesisReady =
      deepResearchRuntimeCopy.draftSynthesisReady
    chatPatch.deepResearchActionDetectedResearchGap =
      deepResearchRuntimeCopy.detectedResearchGap
  }
  if (deepResearchSummaryCopy) {
    chatPatch.deepResearchSummaryGapCoverage = deepResearchSummaryCopy.summaryGapCoverage
    chatPatch.deepResearchSummaryOfficialGap = deepResearchSummaryCopy.summaryOfficialGap
    chatPatch.deepResearchSummaryResolvedCoverage =
      deepResearchSummaryCopy.summaryResolvedCoverage
    chatPatch.deepResearchSummaryDomainCoverage = deepResearchSummaryCopy.summaryDomainCoverage
    chatPatch.deepResearchSummaryFreshnessCoverage =
      deepResearchSummaryCopy.summaryFreshnessCoverage
    chatPatch.deepResearchSummaryClaimSupportCoverage =
      deepResearchSummaryCopy.summaryClaimSupportCoverage
    chatPatch.deepResearchOpenQuestionPrimarySourceVerification =
      deepResearchSummaryCopy.openQuestionPrimarySourceVerification
  }
  if (deepResearchTokenCopy) {
    chatPatch.deepResearchAxisIdentityValidation = deepResearchTokenCopy.axisIdentityValidation
    chatPatch.deepResearchAxisInternetFootprint = deepResearchTokenCopy.axisInternetFootprint
    chatPatch.deepResearchAxisLatest = deepResearchTokenCopy.axisLatest
    chatPatch.deepResearchAxisOfficial = deepResearchTokenCopy.axisOfficial
    chatPatch.deepResearchAxisComparison = deepResearchTokenCopy.axisComparison
    chatPatch.deepResearchAxisBestPractices = deepResearchTokenCopy.axisBestPractices
    chatPatch.deepResearchAxisRetry = deepResearchTokenCopy.axisRetry
    chatPatch.deepResearchAxisResearch = deepResearchTokenCopy.axisResearch
    chatPatch.deepResearchFocusSourceDiversity = deepResearchTokenCopy.focusSourceDiversity
    chatPatch.deepResearchFocusFreshness = deepResearchTokenCopy.focusFreshness
    chatPatch.deepResearchFocusClaimValidation = deepResearchTokenCopy.focusClaimValidation
    chatPatch.deepResearchTimeWindowEarlier = deepResearchTokenCopy.timeWindowEarlier
    chatPatch.deepResearchTimeWindowMiddle = deepResearchTokenCopy.timeWindowMiddle
    chatPatch.deepResearchTimeWindowRecent = deepResearchTokenCopy.timeWindowRecent
  }
  const localizedWebTerm = localizedWebTermLabels[localeKey]
  if (localizedWebTerm) {
    const maybeFillEnglishWebFallback = (target: LocaleNode, key: string, path: string) => {
      const current = getString(messages, path)
      if (!current || current === 'Web') {
        target[key] = localizedWebTerm
      }
    }

    maybeFillEnglishWebFallback(
      chatPatch,
      'deepResearchSourceTypeWeb',
      'chat.deepResearchSourceTypeWeb'
    )
    maybeFillEnglishWebFallback(companionPlatformsPatch, 'web', 'companion.platforms.web')
    maybeFillEnglishWebFallback(toolsNamesPatch, 'web', 'tools.names.web')
  }
  const localizedCacheTerm = localizedCacheTermLabels[localeKey]
  if (localizedCacheTerm) {
    const maybeFillEnglishCacheFallback = (target: LocaleNode, key: string, path: string) => {
      const current = getString(messages, path)
      if (!current || current === 'Cache') {
        target[key] = localizedCacheTerm
      }
    }

    maybeFillEnglishCacheFallback(navPatch, 'cache', 'nav.cache')
    maybeFillEnglishCacheFallback(tokenEconomyPatch, 'cache', 'tokenEconomy.cache')
    maybeFillEnglishCacheFallback(systemPatch, 'cpuCache', 'system.cpuCache')
  }
  const localizedDirectoryWhitelistAlias = localizedDirectoryWhitelistAliasLabels[localeKey]
  if (localizedDirectoryWhitelistAlias) {
    const currentDirectoryWhitelistAlias = getString(messages, 'security.directoryWhitelistAlias')
    if (!currentDirectoryWhitelistAlias || currentDirectoryWhitelistAlias === 'Alias') {
      securityPatch.directoryWhitelistAlias = localizedDirectoryWhitelistAlias
    }
  }
  const localizedOnline = localizedOnlineLabels[localeKey]
  if (localizedOnline) {
    const maybeFillEnglishOnlineFallback = (target: LocaleNode, key: string, path: string) => {
      const current = getString(messages, path)
      if (!current || current === 'Online') {
        target[key] = localizedOnline
      }
    }

    maybeFillEnglishOnlineFallback(commonPatch, 'online', 'common.online')
    maybeFillEnglishOnlineFallback(speechPatch, 'online', 'speech.online')
  }
  const localizedBackend = localizedBackendLabels[localeKey]
  if (localizedBackend) {
    const maybeFillEnglishBackendFallback = (target: LocaleNode, key: string, path: string) => {
      const current = getString(messages, path)
      if (!current || current === 'Backend') {
        target[key] = localizedBackend
      }
    }

    maybeFillEnglishBackendFallback(apiProxyPatch, 'prunerBackend', 'apiProxy.prunerBackend')
    maybeFillEnglishBackendFallback(resultCardLabelsPatch, 'backend', 'resultCard.labels.backend')
    maybeFillEnglishBackendFallback(userdataMemoryPatch, 'backend', 'userdata.memory.backend')
  }
  const webQueryResultCardLabels = webQueryResultCardLabelBackfills[localeKey]
  if (webQueryResultCardLabels) {
    const maybeFillResultCardLabel = (
      key: string,
      localized: string | null,
      englishFallback: string
    ) => {
      if (!localized) return
      const current = getString(messages, `resultCard.labels.${key}`)
      if (!current || current === englishFallback) {
        resultCardLabelsPatch[key] = localized
      }
    }

    maybeFillResultCardLabel('input', getString(messages, 'companion.llmDetails.input'), 'Input')
    maybeFillResultCardLabel('provider', getString(messages, 'common.provider'), 'Provider')
    maybeFillResultCardLabel('mode', webQueryResultCardLabels.mode, 'Mode')
    maybeFillResultCardLabel('has_results', webQueryResultCardLabels.has_results, 'Has Results')
    maybeFillResultCardLabel(
      'selected_result',
      webQueryResultCardLabels.selected_result,
      'Selected Result'
    )
    maybeFillResultCardLabel('key_facts', webQueryResultCardLabels.key_facts, 'Key Facts')
    maybeFillResultCardLabel(
      'key_factsanalysis',
      webQueryResultCardLabels.key_facts,
      'Key Facts Analysis'
    )
    maybeFillResultCardLabel(
      'research_artifact_path',
      webQueryResultCardLabels.research_artifact_path,
      'Research Artifact Path'
    )
    maybeFillResultCardLabel(
      'llm_compacted',
      webQueryResultCardLabels.llm_compacted,
      'LLM Compacted'
    )
    maybeFillResultCardLabel('materialized', webQueryResultCardLabels.materialized, 'Materialized')
  }
  const localizedMetricsMin = localizedMetricsMinLabels[localeKey]
  if (localizedMetricsMin) {
    const currentMetricsMin = getString(messages, 'metrics.min')
    if (!currentMetricsMin || currentMetricsMin === 'Min') {
      metricsPatch.min = localizedMetricsMin
    }
  }
  const localizedTaskKindAgent = localizedTaskKindAgentLabels[localeKey]
  if (localizedTaskKindAgent) {
    const currentTaskKindAgent = getString(messages, 'chat.taskKindAgent')
    if (!currentTaskKindAgent || currentTaskKindAgent === 'Agent') {
      chatPatch.taskKindAgent = localizedTaskKindAgent
    }
  }
  const localizedRoutingModeAuto = getString(messages, 'chat.processTrace.fields.auto')
  if (localizedRoutingModeAuto && localizedRoutingModeAuto !== 'Auto') {
    const currentRoutingModeAuto = getString(messages, 'chat.routingMode.auto')
    if (!currentRoutingModeAuto || currentRoutingModeAuto === 'Auto') {
      routingModePatch.auto = localizedRoutingModeAuto
    }
  }
  const localizedCliEta = localizedCliEtaLabels[localeKey]
  if (localizedCliEta) {
    const currentCliEta = getString(messages, 'cli.eta')
    if (!currentCliEta || currentCliEta === 'ETA: {time}') {
      cliPatch.eta = localizedCliEta
    }
  }
  const localizedWorkspaceTreeDirCount = localizedWorkspaceTreeDirCountLabels[localeKey]
  if (localizedWorkspaceTreeDirCount) {
    const currentWorkspaceTreeDirCount = getString(messages, 'nav.workspaceTreeDirCount')
    if (!currentWorkspaceTreeDirCount || currentWorkspaceTreeDirCount === 'Dirs') {
      navPatch.workspaceTreeDirCount = localizedWorkspaceTreeDirCount
    }
  }
  const localizedMountPoint = localizedMountPointLabels[localeKey]
  if (localizedMountPoint) {
    const currentMountPoint = getString(messages, 'system.mountPoint')
    if (!currentMountPoint || currentMountPoint === 'Mount Point') {
      systemPatch.mountPoint = localizedMountPoint
    }
  }
  const currentHeapAllocation = getString(messages, 'system.heapAllocation')
  const localizedHeapAllocation =
    currentHeapAllocation && currentHeapAllocation !== 'Heap Allocation'
      ? currentHeapAllocation
      : (localizedHeapAllocLabels[localeKey] ?? null)
  const currentHeapAlloc = getString(messages, 'system.heapAlloc')
  if (
    (!currentHeapAlloc || currentHeapAlloc === 'Heap Alloc') &&
    localizedHeapAllocation &&
    localizedHeapAllocation !== 'Heap Allocation'
  ) {
    systemPatch.heapAlloc = localizedHeapAllocation
  }
  const localizedSystemInfo = localizedSystemInfoLabels[localeKey]
  if (localizedSystemInfo) {
    const currentSystemInfo = getString(messages, 'system.info')
    if (!currentSystemInfo || currentSystemInfo === 'Info') {
      systemPatch.info = localizedSystemInfo
    }
  }
  const localizedKernel = localizedKernelLabels[localeKey]
  if (localizedKernel) {
    const currentSystemKernel = getString(messages, 'system.kernel')
    if (!currentSystemKernel || currentSystemKernel === 'Kernel') {
      systemPatch.kernel = localizedKernel
    }
  }
  const localizedAdmin = localizedAdminLabels[localeKey]
  if (localizedAdmin) {
    const currentProfileScopeAdmin = getString(messages, 'profile.scopeAdmin')
    if (!currentProfileScopeAdmin || currentProfileScopeAdmin === 'Admin') {
      profilePatch.scopeAdmin = localizedAdmin
    }
    const currentUserRoleAdmin = getString(messages, 'users.roles.admin')
    if (!currentUserRoleAdmin || currentUserRoleAdmin === 'Admin') {
      usersRolesPatch.admin = localizedAdmin
    }
  }
  const localizedUsersRoleAdmin = getString(messages, 'users.roles.admin')
  const localizedUserRoleAdmin =
    localizedUsersRoleAdmin && localizedUsersRoleAdmin !== 'Admin'
      ? localizedUsersRoleAdmin
      : localizedAdmin
  if (localizedUserRoleAdmin && localizedUserRoleAdmin !== 'Admin') {
    const currentUserRoleAdminLabel = getString(messages, 'users.roleAdmin')
    if (!currentUserRoleAdminLabel || currentUserRoleAdminLabel === 'Admin') {
      usersPatch.roleAdmin = localizedUserRoleAdmin
    }
  }
  const localizedVideoPreview = localizedVideoPreviewLabels[localeKey]
  if (localizedVideoPreview) {
    const currentVideoPreviewKind = getString(messages, 'speech.convertTask.previewKind.video')
    if (!currentVideoPreviewKind || currentVideoPreviewKind === 'Video') {
      speechConvertTaskPreviewKindPatch.video = localizedVideoPreview
    }
  }
  const localizedGoroutine = localizedGoroutineLabels[localeKey]
  if (localizedGoroutine) {
    const maybeFillEnglishGoroutineFallback = (target: LocaleNode, key: string, path: string) => {
      const current = getString(messages, path)
      if (!current || current === 'Goroutines') {
        target[key] = localizedGoroutine
      }
    }

    maybeFillEnglishGoroutineFallback(dashboardPatch, 'goroutines', 'dashboard.goroutines')
    maybeFillEnglishGoroutineFallback(
      dashboardCardsPatch,
      'goroutines',
      'dashboard.cards.goroutines'
    )
    maybeFillEnglishGoroutineFallback(systemPatch, 'goroutines', 'system.goroutines')
    maybeFillEnglishGoroutineFallback(systemPatch, 'numGoroutines', 'system.numGoroutines')
  }
  const localizedGoroutineChart = localizedGoroutineChartLabels[localeKey]
  if (localizedGoroutineChart) {
    const currentGoroutineChart = getString(messages, 'dashboard.cards.goroutinesChart')
    if (!currentGoroutineChart || currentGoroutineChart === 'Goroutines') {
      dashboardCardsPatch.goroutinesChart = localizedGoroutineChart
    }
  }
  const localizedAuthToken = localizedAuthTokenLabels[localeKey]
  if (localizedAuthToken) {
    const currentAuthToken = getString(messages, 'channels.authToken')
    if (!currentAuthToken || currentAuthToken === 'Auth Token') {
      channelsPatch.authToken = localizedAuthToken
    }
  }
  const localizedViberAuthTokenPlaceholder = localizedViberAuthTokenPlaceholders[localeKey]
  if (localizedViberAuthTokenPlaceholder) {
    const currentViberAuthTokenPlaceholder = getString(
      messages,
      'channels.placeholderViberAuthToken'
    )
    if (
      !currentViberAuthTokenPlaceholder ||
      currentViberAuthTokenPlaceholder === 'Viber Auth Token'
    ) {
      channelsPatch.placeholderViberAuthToken = localizedViberAuthTokenPlaceholder
    }
  }
  const providerRecoveryBackfill = providerRecoveryLocaleBackfills[localeKey]
  if (providerRecoveryBackfill) {
    const maybeFillEnglishFallback = (
      target: LocaleNode,
      key: keyof ProviderRecoveryLocalePatch,
      path: string
    ) => {
      const current = getString(messages, path)
      const english = providerRecoveryEnglishDefaults[key]
      if (!current || current === english) {
        target[key] = providerRecoveryBackfill[key]
      }
    }

    maybeFillEnglishFallback(
      noProviderPatch,
      'recoverableEyebrow',
      'chat.noProvider.recoverableEyebrow'
    )
    maybeFillEnglishFallback(
      noProviderPatch,
      'recoverableTitle',
      'chat.noProvider.recoverableTitle'
    )
    maybeFillEnglishFallback(
      noProviderPatch,
      'recoverableDescription',
      'chat.noProvider.recoverableDescription'
    )
    maybeFillEnglishFallback(noProviderPatch, 'retry', 'chat.noProvider.retry')
    maybeFillEnglishFallback(noProviderPatch, 'reviewSingle', 'chat.noProvider.reviewSingle')
    maybeFillEnglishFallback(chatPatch, 'providerNeedsAttention', 'chat.providerNeedsAttention')
    maybeFillEnglishFallback(
      externalAgentsPatch,
      'builtinProfile',
      'settings.externalAgents.builtinProfile'
    )
  }
  const routingModePinnedBackfill = routingModePinnedBackfills[localeKey]
  if (routingModePinnedBackfill) {
    if (!getString(messages, 'chat.routingMode.providerPinnedTitle')) {
      routingModePatch.providerPinnedTitle = routingModePinnedBackfill.providerPinnedTitle
    }
    if (!getString(messages, 'chat.routingMode.providerPinnedDesc')) {
      routingModePatch.providerPinnedDesc = routingModePinnedBackfill.providerPinnedDesc
    }
    if (!getString(messages, 'chat.routingMode.providerPinnedReset')) {
      routingModePatch.providerPinnedReset = routingModePinnedBackfill.providerPinnedReset
    }
  }
  const currentSignalVulnerabilities = getString(
    messages,
    'skillStore.marketplace.signals.vulnerabilities'
  )
  const localizedFilterVulnerabilities = getString(
    messages,
    'skillStore.marketplace.filters.vulnerabilities'
  )
  if (
    currentSignalVulnerabilities === 'Vuln' &&
    localizedFilterVulnerabilities &&
    localizedFilterVulnerabilities !== 'Vulnerabilities'
  ) {
    skillStoreSignalsPatch.vulnerabilities = localizedFilterVulnerabilities
  }
  if (hasKeys(noProviderPatch)) {
    chatPatch.noProvider = noProviderPatch
  }
  if (hasKeys(routingModePatch)) {
    chatPatch.routingMode = routingModePatch
  }
  if (hasKeys(chatPatch)) {
    patch.chat = chatPatch
  }
  if (hasKeys(apiProxyPatch)) {
    patch.apiProxy = apiProxyPatch
  }
  if (hasKeys(cliPatch)) {
    patch.cli = cliPatch
  }
  if (hasKeys(navPatch)) {
    patch.nav = navPatch
  }
  if (hasKeys(profilePatch)) {
    patch.profile = profilePatch
  }
  if (hasKeys(companionPlatformsPatch)) {
    companionPatch.platforms = companionPlatformsPatch
  }
  if (hasKeys(companionPatch)) {
    patch.companion = companionPatch
  }
  if (hasKeys(externalAgentsPatch)) {
    patch.settings = {
      ...(patch.settings as LocaleNode | undefined),
      externalAgents: externalAgentsPatch,
    }
  }
  if (hasKeys(skillStoreSignalsPatch)) {
    patch.skillStore = {
      marketplace: {
        signals: skillStoreSignalsPatch,
      },
    }
  }
  if (hasKeys(securityPatch)) {
    patch.security = securityPatch
  }
  if (hasKeys(resultCardLabelsPatch)) {
    resultCardPatch.labels = resultCardLabelsPatch
  }
  if (hasKeys(resultCardPatch)) {
    patch.resultCard = resultCardPatch
  }
  if (hasKeys(channelsPatch)) {
    patch.channels = channelsPatch
  }
  if (hasKeys(dashboardCardsPatch)) {
    dashboardPatch.cards = dashboardCardsPatch
  }
  if (hasKeys(dashboardPatch)) {
    patch.dashboard = dashboardPatch
  }
  if (hasKeys(metricsPatch)) {
    patch.metrics = metricsPatch
  }
  if (hasKeys(systemPatch)) {
    patch.system = {
      ...(patch.system as LocaleNode | undefined),
      ...systemPatch,
    }
  }
  if (hasKeys(speechPatch)) {
    patch.speech = speechPatch
  }
  if (hasKeys(speechConvertTaskPreviewKindPatch)) {
    speechConvertTaskPatch.previewKind = speechConvertTaskPreviewKindPatch
  }
  if (hasKeys(speechConvertTaskPatch)) {
    patch.speech = {
      ...(patch.speech as LocaleNode | undefined),
      convertTask: speechConvertTaskPatch,
    }
  }
  if (hasKeys(tokenEconomyPatch)) {
    patch.tokenEconomy = tokenEconomyPatch
  }
  if (hasKeys(toolsNamesPatch)) {
    toolsPatch.names = toolsNamesPatch
  }
  if (hasKeys(toolsPatch)) {
    patch.tools = toolsPatch
  }
  if (hasKeys(userdataMemoryPatch)) {
    userdataPatch.memory = userdataMemoryPatch
  }
  if (hasKeys(userdataPatch)) {
    patch.userdata = userdataPatch
  }
  if (hasKeys(usersRolesPatch)) {
    usersPatch.roles = usersRolesPatch
  }
  if (hasKeys(usersPatch)) {
    patch.users = usersPatch
  }
  commonPatch.filter = commonFilterLabels[localeKey]
  if (hasKeys(commonPatch)) {
    patch.common = commonPatch
  }
  if (hasKeys(execCardPatch)) {
    patch.execCard = execCardPatch
  }

  return patch
}
