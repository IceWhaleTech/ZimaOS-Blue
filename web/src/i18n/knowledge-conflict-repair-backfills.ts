import type { LocaleKey } from './locale-catalog'

type KnowledgeConflictRepairStrings = readonly [
  repairConflicts: string,
  repairConflictsStarted: string,
  repairConflictsComplete: string,
]

type KnowledgeConflictRepairExtras = Partial<{
  repairConflictsProgress: string
  repairConflictsCurrent: string
  repairConflictsQueued: string
  repairConflictsRunning: string
  repairConflictsScan: string
  repairConflictsResolving: string
  repairConflictsPersist: string
  repairConflictsLint: string
}>

function buildKnowledgeConflictRepairBackfill(
  copy: KnowledgeConflictRepairStrings,
  extras: KnowledgeConflictRepairExtras = {}
) {
  const [repairConflicts, repairConflictsStarted, repairConflictsComplete] = copy

  return {
    knowledge: {
      repairConflicts,
      repairConflictsStarted,
      repairConflictsComplete,
      ...extras,
    },
  }
}

const knowledgeConflictRepairStageBackfills: Record<LocaleKey, KnowledgeConflictRepairExtras> = {
  'ca-ES': {
    repairConflictsProgress: 'Reparació de conflictes en curs',
    repairConflictsCurrent: "S'està processant",
    repairConflictsQueued: 'En cua per reparar',
    repairConflictsRunning: "S'està preparant la reparació",
    repairConflictsScan: "S'estan escanejant els grups de conflicte",
    repairConflictsResolving: "S'està reparant el grup de conflicte actual",
    repairConflictsPersist: "S'estan escrivint les pàgines reparades",
    repairConflictsLint: "S'està actualitzant l'informe de lint",
  },
  'cs-CZ': {
    repairConflictsProgress: 'Oprava konfliktů probíhá',
    repairConflictsCurrent: 'Právě se zpracovává',
    repairConflictsQueued: 'Ve frontě na opravu',
    repairConflictsRunning: 'Připravuje se oprava',
    repairConflictsScan: 'Skenují se skupiny konfliktů',
    repairConflictsResolving: 'Opravuje se aktuální skupina konfliktů',
    repairConflictsPersist: 'Zapisují se opravené stránky',
    repairConflictsLint: 'Obnovuje se lint report',
  },
  'da-DK': {
    repairConflictsProgress: 'Konfliktreparation i gang',
    repairConflictsCurrent: 'Behandler nu',
    repairConflictsQueued: 'Sat i kø til reparation',
    repairConflictsRunning: 'Forbereder reparation',
    repairConflictsScan: 'Scanner konfliktgrupper',
    repairConflictsResolving: 'Reparerer den aktuelle konfliktgruppe',
    repairConflictsPersist: 'Skriver reparerede sider',
    repairConflictsLint: 'Opdaterer lint-rapport',
  },
  'de-DE': {
    repairConflictsProgress: 'Konfliktbehebung läuft',
    repairConflictsCurrent: 'Wird gerade verarbeitet',
    repairConflictsQueued: 'Zur Behebung in Warteschlange',
    repairConflictsRunning: 'Behebung wird vorbereitet',
    repairConflictsScan: 'Konfliktgruppen werden gescannt',
    repairConflictsResolving: 'Aktuelle Konfliktgruppe wird behoben',
    repairConflictsPersist: 'Behobene Seiten werden geschrieben',
    repairConflictsLint: 'Lint-Bericht wird aktualisiert',
  },
  'el-GR': {
    repairConflictsProgress: 'Η επιδιόρθωση συγκρούσεων βρίσκεται σε εξέλιξη',
    repairConflictsCurrent: 'Γίνεται επεξεργασία',
    repairConflictsQueued: 'Στην ουρά για επιδιόρθωση',
    repairConflictsRunning: 'Προετοιμάζεται η επιδιόρθωση',
    repairConflictsScan: 'Σάρωση ομάδων συγκρούσεων',
    repairConflictsResolving: 'Επιδιορθώνεται η τρέχουσα ομάδα συγκρούσεων',
    repairConflictsPersist: 'Γράφονται οι επιδιορθωμένες σελίδες',
    repairConflictsLint: 'Ανανεώνεται η αναφορά lint',
  },
  'en-GB': {
    repairConflictsProgress: 'Conflict repair in progress',
    repairConflictsCurrent: 'Currently processing',
    repairConflictsQueued: 'Queued for repair',
    repairConflictsRunning: 'Preparing repair',
    repairConflictsScan: 'Scanning conflict groups',
    repairConflictsResolving: 'Repairing current conflict group',
    repairConflictsPersist: 'Writing repaired pages',
    repairConflictsLint: 'Refreshing lint report',
  },
  'en-US': {
    repairConflictsProgress: 'Conflict repair in progress',
    repairConflictsCurrent: 'Currently processing',
    repairConflictsQueued: 'Queued for repair',
    repairConflictsRunning: 'Preparing repair',
    repairConflictsScan: 'Scanning conflict groups',
    repairConflictsResolving: 'Repairing current conflict group',
    repairConflictsPersist: 'Writing repaired pages',
    repairConflictsLint: 'Refreshing lint report',
  },
  'es-ES': {
    repairConflictsProgress: 'Reparación de conflictos en curso',
    repairConflictsCurrent: 'Procesando ahora',
    repairConflictsQueued: 'En cola para reparar',
    repairConflictsRunning: 'Preparando reparación',
    repairConflictsScan: 'Escaneando grupos de conflicto',
    repairConflictsResolving: 'Reparando el grupo de conflicto actual',
    repairConflictsPersist: 'Escribiendo páginas reparadas',
    repairConflictsLint: 'Actualizando el informe de lint',
  },
  'fr-FR': {
    repairConflictsProgress: 'Réparation des conflits en cours',
    repairConflictsCurrent: 'Traitement en cours',
    repairConflictsQueued: 'En file pour réparation',
    repairConflictsRunning: 'Préparation de la réparation',
    repairConflictsScan: 'Analyse des groupes de conflits',
    repairConflictsResolving: 'Réparation du groupe de conflits en cours',
    repairConflictsPersist: 'Écriture des pages réparées',
    repairConflictsLint: 'Actualisation du rapport lint',
  },
  'ga-IE': {
    repairConflictsProgress: 'Deisiú coinbhleachtaí ar siúl',
    repairConflictsCurrent: 'Á phróiseáil faoi láthair',
    repairConflictsQueued: 'Sa scuaine le deisigh',
    repairConflictsRunning: 'Ag ullmhú an deisithe',
    repairConflictsScan: 'Ag scanadh grúpaí coinbhleachta',
    repairConflictsResolving: 'Ag deisiú an ghrúpa coinbhleachta reatha',
    repairConflictsPersist: 'Ag scríobh na leathanach deisithe',
    repairConflictsLint: 'Ag athnuachan na tuarascála lint',
  },
  'hr-HR': {
    repairConflictsProgress: 'Popravak sukoba je u tijeku',
    repairConflictsCurrent: 'Trenutačno se obrađuje',
    repairConflictsQueued: 'U redu za popravak',
    repairConflictsRunning: 'Priprema se popravak',
    repairConflictsScan: 'Skeniraju se skupine sukoba',
    repairConflictsResolving: 'Popravlja se trenutna skupina sukoba',
    repairConflictsPersist: 'Upisuju se popravljene stranice',
    repairConflictsLint: 'Osvježava se lint izvještaj',
  },
  'hu-HU': {
    repairConflictsProgress: 'Az ütközések javítása folyamatban',
    repairConflictsCurrent: 'Jelenleg feldolgozás alatt',
    repairConflictsQueued: 'Javításra vár a sorban',
    repairConflictsRunning: 'Javítás előkészítése',
    repairConflictsScan: 'Ütközéscsoportok vizsgálata',
    repairConflictsResolving: 'Az aktuális ütközéscsoport javítása',
    repairConflictsPersist: 'A javított oldalak írása',
    repairConflictsLint: 'A lint jelentés frissítése',
  },
  'it-IT': {
    repairConflictsProgress: 'Riparazione dei conflitti in corso',
    repairConflictsCurrent: 'Elaborazione in corso',
    repairConflictsQueued: 'In coda per la riparazione',
    repairConflictsRunning: 'Preparazione della riparazione',
    repairConflictsScan: 'Scansione dei gruppi di conflitto',
    repairConflictsResolving: 'Riparazione del gruppo di conflitto corrente',
    repairConflictsPersist: 'Scrittura delle pagine riparate',
    repairConflictsLint: 'Aggiornamento del report lint',
  },
  'ja-JP': {
    repairConflictsProgress: '競合の修復を進行中',
    repairConflictsCurrent: '現在処理中',
    repairConflictsQueued: '修復待ちキューに追加済み',
    repairConflictsRunning: '修復を準備中',
    repairConflictsScan: '競合グループをスキャン中',
    repairConflictsResolving: '現在の競合グループを修復中',
    repairConflictsPersist: '修復したページを書き込み中',
    repairConflictsLint: 'lint レポートを更新中',
  },
  'ko-KR': {
    repairConflictsProgress: '충돌 수정 진행 중',
    repairConflictsCurrent: '현재 처리 중',
    repairConflictsQueued: '수정 대기열에 추가됨',
    repairConflictsRunning: '수정 준비 중',
    repairConflictsScan: '충돌 그룹을 스캔하는 중',
    repairConflictsResolving: '현재 충돌 그룹을 수정하는 중',
    repairConflictsPersist: '수정된 페이지를 쓰는 중',
    repairConflictsLint: 'lint 보고서를 새로 고치는 중',
  },
  'ml-IN': {
    repairConflictsProgress: 'സംഘര്‍ഷ പരിഹാരം പുരോഗമിക്കുന്നു',
    repairConflictsCurrent: 'ഇപ്പോൾ പ്രോസസ്സ് ചെയ്യുന്നു',
    repairConflictsQueued: 'പരിഹാരത്തിനായി നിരയിൽ ചേർത്തു',
    repairConflictsRunning: 'പരിഹാരം തയ്യാറാക്കുന്നു',
    repairConflictsScan: 'സംഘര്‍ഷ ഗ്രൂപ്പുകൾ സ്കാൻ ചെയ്യുന്നു',
    repairConflictsResolving: 'നിലവിലെ സംഘര്‍ഷ ഗ്രൂപ്പ് പരിഹരിക്കുന്നു',
    repairConflictsPersist: 'പരിഹരിച്ച പേജുകൾ എഴുതുന്നു',
    repairConflictsLint: 'lint റിപ്പോർട്ട് പുതുക്കുന്നു',
  },
  'nb-NO': {
    repairConflictsProgress: 'Konfliktreparasjon pågår',
    repairConflictsCurrent: 'Behandler nå',
    repairConflictsQueued: 'Satt i kø for reparasjon',
    repairConflictsRunning: 'Forbereder reparasjon',
    repairConflictsScan: 'Skanner konfliktgrupper',
    repairConflictsResolving: 'Reparerer gjeldende konfliktgruppe',
    repairConflictsPersist: 'Skriver reparerte sider',
    repairConflictsLint: 'Oppdaterer lint-rapport',
  },
  'nl-NL': {
    repairConflictsProgress: 'Conflictherstel wordt uitgevoerd',
    repairConflictsCurrent: 'Wordt nu verwerkt',
    repairConflictsQueued: 'In wachtrij voor herstel',
    repairConflictsRunning: 'Herstel voorbereiden',
    repairConflictsScan: 'Conflictgroepen worden gescand',
    repairConflictsResolving: 'Huidige conflictgroep wordt hersteld',
    repairConflictsPersist: 'Herstelde pagina’s worden weggeschreven',
    repairConflictsLint: 'Lint-rapport wordt vernieuwd',
  },
  'pl-PL': {
    repairConflictsProgress: 'Naprawa konfliktów w toku',
    repairConflictsCurrent: 'Obecnie przetwarzane',
    repairConflictsQueued: 'W kolejce do naprawy',
    repairConflictsRunning: 'Przygotowywanie naprawy',
    repairConflictsScan: 'Skanowanie grup konfliktów',
    repairConflictsResolving: 'Naprawianie bieżącej grupy konfliktów',
    repairConflictsPersist: 'Zapisywanie naprawionych stron',
    repairConflictsLint: 'Odświeżanie raportu lint',
  },
  'pt-BR': {
    repairConflictsProgress: 'Correção de conflitos em andamento',
    repairConflictsCurrent: 'Processando agora',
    repairConflictsQueued: 'Na fila para correção',
    repairConflictsRunning: 'Preparando a correção',
    repairConflictsScan: 'Escaneando grupos de conflito',
    repairConflictsResolving: 'Corrigindo o grupo de conflito atual',
    repairConflictsPersist: 'Gravando páginas corrigidas',
    repairConflictsLint: 'Atualizando o relatório de lint',
  },
  'pt-PT': {
    repairConflictsProgress: 'Correção de conflitos em curso',
    repairConflictsCurrent: 'A processar agora',
    repairConflictsQueued: 'Na fila para correção',
    repairConflictsRunning: 'A preparar a correção',
    repairConflictsScan: 'A analisar grupos de conflito',
    repairConflictsResolving: 'A corrigir o grupo de conflito atual',
    repairConflictsPersist: 'A escrever páginas corrigidas',
    repairConflictsLint: 'A atualizar o relatório de lint',
  },
  'ro-RO': {
    repairConflictsProgress: 'Repararea conflictelor este în curs',
    repairConflictsCurrent: 'Se procesează acum',
    repairConflictsQueued: 'În coadă pentru reparare',
    repairConflictsRunning: 'Se pregătește repararea',
    repairConflictsScan: 'Se scanează grupurile de conflicte',
    repairConflictsResolving: 'Se repară grupul curent de conflicte',
    repairConflictsPersist: 'Se scriu paginile reparate',
    repairConflictsLint: 'Se reîmprospătează raportul lint',
  },
  'ru-RU': {
    repairConflictsProgress: 'Исправление конфликтов выполняется',
    repairConflictsCurrent: 'Сейчас обрабатывается',
    repairConflictsQueued: 'В очереди на исправление',
    repairConflictsRunning: 'Подготовка исправления',
    repairConflictsScan: 'Сканирование групп конфликтов',
    repairConflictsResolving: 'Исправляется текущая группа конфликтов',
    repairConflictsPersist: 'Записываются исправленные страницы',
    repairConflictsLint: 'Обновляется отчёт lint',
  },
  'sk-SK': {
    repairConflictsProgress: 'Oprava konfliktov prebieha',
    repairConflictsCurrent: 'Práve sa spracúva',
    repairConflictsQueued: 'V rade na opravu',
    repairConflictsRunning: 'Pripravuje sa oprava',
    repairConflictsScan: 'Skenujú sa skupiny konfliktov',
    repairConflictsResolving: 'Opravuje sa aktuálna skupina konfliktov',
    repairConflictsPersist: 'Zapisujú sa opravené stránky',
    repairConflictsLint: 'Obnovuje sa lint report',
  },
  'sv-SE': {
    repairConflictsProgress: 'Konfliktreparation pågår',
    repairConflictsCurrent: 'Bearbetar just nu',
    repairConflictsQueued: 'I kö för reparation',
    repairConflictsRunning: 'Förbereder reparation',
    repairConflictsScan: 'Skannar konfliktgrupper',
    repairConflictsResolving: 'Reparerar aktuell konfliktgrupp',
    repairConflictsPersist: 'Skriver reparerade sidor',
    repairConflictsLint: 'Uppdaterar lint-rapport',
  },
  'zh-CN': {
    repairConflictsProgress: '冲突修复进行中',
    repairConflictsCurrent: '当前处理',
    repairConflictsQueued: '已加入修复队列',
    repairConflictsRunning: '正在准备修复',
    repairConflictsScan: '正在扫描冲突分组',
    repairConflictsResolving: '正在修复当前冲突组',
    repairConflictsPersist: '正在写回修复后的页面',
    repairConflictsLint: '正在刷新 lint 报告',
  },
  'zh-TW': {
    repairConflictsProgress: '衝突修復進行中',
    repairConflictsCurrent: '目前處理',
    repairConflictsQueued: '已加入修復佇列',
    repairConflictsRunning: '正在準備修復',
    repairConflictsScan: '正在掃描衝突分組',
    repairConflictsResolving: '正在修復目前衝突組',
    repairConflictsPersist: '正在寫回修復後的頁面',
    repairConflictsLint: '正在刷新 lint 報告',
  },
}

function buildLocalizedKnowledgeConflictRepairBackfill(
  locale: LocaleKey,
  copy: KnowledgeConflictRepairStrings
) {
  return buildKnowledgeConflictRepairBackfill(copy, knowledgeConflictRepairStageBackfills[locale])
}

const knowledgeConflictRepairBackfills: Partial<Record<LocaleKey, object>> = {
  'en-US': buildLocalizedKnowledgeConflictRepairBackfill('en-US', [
    'Repair conflicts',
    'Knowledge conflict repair started.',
    'Knowledge conflicts repaired.',
  ]),
  'en-GB': buildLocalizedKnowledgeConflictRepairBackfill('en-GB', [
    'Repair conflicts',
    'Knowledge conflict repair started.',
    'Knowledge conflicts repaired.',
  ]),
  'zh-CN': buildLocalizedKnowledgeConflictRepairBackfill('zh-CN', [
    '修复冲突',
    '知识冲突修复已开始。',
    '知识冲突已修复。',
  ]),
  'zh-TW': buildLocalizedKnowledgeConflictRepairBackfill('zh-TW', [
    '修復衝突',
    '知識衝突修復已開始。',
    '知識衝突已修復。',
  ]),
  'ja-JP': buildLocalizedKnowledgeConflictRepairBackfill('ja-JP', [
    '競合を修復',
    'ナレッジ競合の修復を開始しました。',
    'ナレッジ競合を修復しました。',
  ]),
  'ko-KR': buildLocalizedKnowledgeConflictRepairBackfill('ko-KR', [
    '충돌 수정',
    '지식 충돌 수정을 시작했습니다.',
    '지식 충돌을 수정했습니다.',
  ]),
  'fr-FR': buildLocalizedKnowledgeConflictRepairBackfill('fr-FR', [
    'Réparer les conflits',
    'La réparation des conflits de connaissances a commencé.',
    'Les conflits de connaissances ont été réparés.',
  ]),
  'de-DE': buildLocalizedKnowledgeConflictRepairBackfill('de-DE', [
    'Konflikte beheben',
    'Die Behebung der Wissenskonflikte wurde gestartet.',
    'Die Wissenskonflikte wurden behoben.',
  ]),
  'es-ES': buildLocalizedKnowledgeConflictRepairBackfill('es-ES', [
    'Reparar conflictos',
    'La reparación de conflictos de conocimiento ha comenzado.',
    'Se repararon los conflictos de conocimiento.',
  ]),
  'pt-BR': buildLocalizedKnowledgeConflictRepairBackfill('pt-BR', [
    'Corrigir conflitos',
    'A correção de conflitos de conhecimento foi iniciada.',
    'Os conflitos de conhecimento foram corrigidos.',
  ]),
  'pt-PT': buildLocalizedKnowledgeConflictRepairBackfill('pt-PT', [
    'Corrigir conflitos',
    'A correção de conflitos de conhecimento foi iniciada.',
    'Os conflitos de conhecimento foram corrigidos.',
  ]),
  'it-IT': buildLocalizedKnowledgeConflictRepairBackfill('it-IT', [
    'Ripara conflitti',
    'La riparazione dei conflitti di conoscenza è iniziata.',
    'I conflitti di conoscenza sono stati riparati.',
  ]),
  'nl-NL': buildLocalizedKnowledgeConflictRepairBackfill('nl-NL', [
    'Conflicten herstellen',
    'Het herstellen van kennisconflicten is gestart.',
    'Kennisconflicten zijn hersteld.',
  ]),
  'pl-PL': buildLocalizedKnowledgeConflictRepairBackfill('pl-PL', [
    'Napraw konflikty',
    'Naprawa konfliktów wiedzy została rozpoczęta.',
    'Konflikty wiedzy zostały naprawione.',
  ]),
  'ru-RU': buildLocalizedKnowledgeConflictRepairBackfill('ru-RU', [
    'Исправить конфликты',
    'Запущено исправление конфликтов знаний.',
    'Конфликты знаний исправлены.',
  ]),
  'sv-SE': buildLocalizedKnowledgeConflictRepairBackfill('sv-SE', [
    'Reparera konflikter',
    'Reparation av kunskapskonflikter har startat.',
    'Kunskapskonflikter har reparerats.',
  ]),
  'nb-NO': buildLocalizedKnowledgeConflictRepairBackfill('nb-NO', [
    'Reparer konflikter',
    'Reparasjon av kunnskapskonflikter har startet.',
    'Kunnskapskonflikter er reparert.',
  ]),
  'da-DK': buildLocalizedKnowledgeConflictRepairBackfill('da-DK', [
    'Ret konflikter',
    'Reparation af videnskonflikter er startet.',
    'Videnskonflikter er rettet.',
  ]),
  'cs-CZ': buildLocalizedKnowledgeConflictRepairBackfill('cs-CZ', [
    'Opravit konflikty',
    'Oprava konfliktů znalostí byla spuštěna.',
    'Konflikty znalostí byly opraveny.',
  ]),
  'sk-SK': buildLocalizedKnowledgeConflictRepairBackfill('sk-SK', [
    'Opraviť konflikty',
    'Oprava konfliktov znalostí sa začala.',
    'Konflikty znalostí boli opravené.',
  ]),
  'ro-RO': buildLocalizedKnowledgeConflictRepairBackfill('ro-RO', [
    'Repară conflictele',
    'Repararea conflictelor de cunoștințe a început.',
    'Conflictele de cunoștințe au fost reparate.',
  ]),
  'hu-HU': buildLocalizedKnowledgeConflictRepairBackfill('hu-HU', [
    'Ütközések javítása',
    'A tudásütközések javítása elindult.',
    'A tudásütközések javítva.',
  ]),
  'hr-HR': buildLocalizedKnowledgeConflictRepairBackfill('hr-HR', [
    'Popravi sukobe',
    'Popravljanje sukoba znanja je započelo.',
    'Sukobi znanja su popravljeni.',
  ]),
  'ga-IE': buildLocalizedKnowledgeConflictRepairBackfill('ga-IE', [
    'Deisigh coinbhleachtaí',
    'Tá deisiú coinbhleachtaí eolais tosaithe.',
    'Tá coinbhleachtaí eolais deisithe.',
  ]),
  'ca-ES': buildLocalizedKnowledgeConflictRepairBackfill('ca-ES', [
    'Resol els conflictes',
    'La reparació de conflictes de coneixement ha començat.',
    "Els conflictes de coneixement s'han reparat.",
  ]),
  'el-GR': buildLocalizedKnowledgeConflictRepairBackfill('el-GR', [
    'Διόρθωση συγκρούσεων',
    'Η επιδιόρθωση των συγκρούσεων γνώσης ξεκίνησε.',
    'Οι συγκρούσεις γνώσης επιδιορθώθηκαν.',
  ]),
  'ml-IN': buildLocalizedKnowledgeConflictRepairBackfill('ml-IN', [
    'സംഘര്‍ഷങ്ങള്‍ പരിഹരിക്കുക',
    'ജ്ഞാന സംഘര്‍ഷങ്ങളുടെ പരിഹാരം ആരംഭിച്ചു.',
    'ജ്ഞാന സംഘര്‍ഷങ്ങള്‍ പരിഹരിച്ചു.',
  ]),
}

export default knowledgeConflictRepairBackfills
