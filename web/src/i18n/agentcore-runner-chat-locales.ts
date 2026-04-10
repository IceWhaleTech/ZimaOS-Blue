import type { LocaleKey } from './locale-catalog'

export const agentcoreRunnerChatBase = {
  refLabel: 'Runner version',
  defaultBranchLabel: 'Baseline',
  mobileHint: 'Saved for this chat and switched automatically when you change sessions.',
}

export const agentcoreRunnerChatOverrides: Partial<Record<LocaleKey, typeof agentcoreRunnerChatBase>> =
  {
    'ca-ES': {
      refLabel: 'Versió del runner',
      defaultBranchLabel: 'Línia base',
      mobileHint: 'Es desa per a aquest xat i canvia automàticament quan canvies de sessió.',
    },
    'cs-CZ': {
      refLabel: 'Verze runneru',
      defaultBranchLabel: 'Základní linie',
      mobileHint: 'Uloží se pro tento chat a při přepnutí konverzace se automaticky změní.',
    },
    'da-DK': {
      refLabel: 'Runner-version',
      defaultBranchLabel: 'Basislinje',
      mobileHint: 'Gemmes for denne chat og skifter automatisk, når du skifter samtale.',
    },
    'de-DE': {
      refLabel: 'Runner-Version',
      defaultBranchLabel: 'Basislinie',
      mobileHint:
        'Wird für diesen Chat gespeichert und beim Wechseln der Unterhaltung automatisch umgeschaltet.',
    },
    'el-GR': {
      refLabel: 'Έκδοση runner',
      defaultBranchLabel: 'Γραμμή βάσης',
      mobileHint:
        'Αποθηκεύεται για αυτήν τη συνομιλία και αλλάζει αυτόματα όταν αλλάζετε συνεδρία.',
    },
    'en-GB': {
      refLabel: 'Runner version',
      defaultBranchLabel: 'Baseline',
      mobileHint: 'Saved for this chat and switched automatically when you change sessions.',
    },
    'es-ES': {
      refLabel: 'Versión del runner',
      defaultBranchLabel: 'Línea base',
      mobileHint:
        'Se guarda para este chat y cambia automáticamente cuando cambias de conversación.',
    },
    'fr-FR': {
      refLabel: 'Version du runner',
      defaultBranchLabel: 'Référence',
      mobileHint:
        'Enregistré pour cette discussion et basculé automatiquement lorsque vous changez de session.',
    },
    'ga-IE': {
      refLabel: 'Leagan an runner',
      defaultBranchLabel: 'Bunlíne',
      mobileHint:
        'Sábháiltear é don chomhrá seo agus athraítear go huathoibríoch é nuair a athraíonn tú seisiúin.',
    },
    'hr-HR': {
      refLabel: 'Verzija runnera',
      defaultBranchLabel: 'Osnovna linija',
      mobileHint: 'Sprema se za ovaj chat i automatski se prebacuje kada promijenite sesiju.',
    },
    'hu-HU': {
      refLabel: 'Runner verzió',
      defaultBranchLabel: 'Alapvonal',
      mobileHint: 'Ehhez a csevegéshez mentődik, és beszélgetésváltáskor automatikusan átvált.',
    },
    'it-IT': {
      refLabel: 'Versione del runner',
      defaultBranchLabel: 'Linea base',
      mobileHint:
        'Viene salvato per questa chat e cambia automaticamente quando passi a un’altra sessione.',
    },
    'ja-JP': {
      refLabel: 'Runner のバージョン',
      defaultBranchLabel: 'ベースライン',
      mobileHint: 'このチャット用に保存され、セッションを切り替えると自動で切り替わります。',
    },
    'ko-KR': {
      refLabel: '러너 버전',
      defaultBranchLabel: '기준선',
      mobileHint: '이 채팅에 저장되며 세션을 바꾸면 자동으로 전환됩니다.',
    },
    'ml-IN': {
      refLabel: 'റണ്ണർ പതിപ്പ്',
      defaultBranchLabel: 'അടിസ്ഥാനരേഖ',
      mobileHint: 'ഈ ചാറ്റിനായി ഇത് സംരക്ഷിക്കപ്പെടുകയും സെഷൻ മാറ്റുമ്പോൾ സ്വയം മാറുകയും ചെയ്യും.',
    },
    'nb-NO': {
      refLabel: 'Runner-versjon',
      defaultBranchLabel: 'Grunnlinje',
      mobileHint: 'Lagres for denne chatten og byttes automatisk når du bytter samtale.',
    },
    'nl-NL': {
      refLabel: 'Runner-versie',
      defaultBranchLabel: 'Basislijn',
      mobileHint: 'Wordt voor deze chat opgeslagen en automatisch gewisseld wanneer je van sessie wisselt.',
    },
    'pl-PL': {
      refLabel: 'Wersja runnera',
      defaultBranchLabel: 'Linia bazowa',
      mobileHint: 'Jest zapisywana dla tego czatu i przełącza się automatycznie przy zmianie sesji.',
    },
    'pt-BR': {
      refLabel: 'Versão do runner',
      defaultBranchLabel: 'Linha de base',
      mobileHint: 'Fica salva para este chat e muda automaticamente quando você troca de sessão.',
    },
    'pt-PT': {
      refLabel: 'Versão do runner',
      defaultBranchLabel: 'Linha de base',
      mobileHint: 'Fica guardada para esta conversa e muda automaticamente quando muda de sessão.',
    },
    'ro-RO': {
      refLabel: 'Versiunea runnerului',
      defaultBranchLabel: 'Linie de bază',
      mobileHint: 'Este salvat pentru acest chat și se schimbă automat când schimbați sesiunea.',
    },
    'ru-RU': {
      refLabel: 'Версия runner',
      defaultBranchLabel: 'Базовая линия',
      mobileHint: 'Сохраняется для этого чата и автоматически переключается при смене сессии.',
    },
    'sk-SK': {
      refLabel: 'Verzia runnera',
      defaultBranchLabel: 'Základná línia',
      mobileHint: 'Uloží sa pre tento chat a pri prepnutí konverzácie sa automaticky zmení.',
    },
    'sv-SE': {
      refLabel: 'Runner-version',
      defaultBranchLabel: 'Baslinje',
      mobileHint: 'Sparas för den här chatten och byts automatiskt när du byter session.',
    },
    'zh-CN': {
      refLabel: 'Runner 版本',
      defaultBranchLabel: '基线',
      mobileHint: '会为当前聊天保存，并在切换会话时自动切换。',
    },
    'zh-TW': {
      refLabel: 'Runner 版本',
      defaultBranchLabel: '基線',
      mobileHint: '會為目前聊天儲存，並在切換工作階段時自動切換。',
    },
  }
