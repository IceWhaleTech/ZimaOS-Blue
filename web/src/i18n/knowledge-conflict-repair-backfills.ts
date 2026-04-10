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

const knowledgeConflictRepairBackfills: Partial<Record<LocaleKey, object>> = {
  'en-US': buildKnowledgeConflictRepairBackfill(
    ['Repair conflicts', 'Knowledge conflict repair started.', 'Knowledge conflicts repaired.'],
    {
      repairConflictsProgress: 'Conflict repair in progress',
      repairConflictsCurrent: 'Currently processing',
      repairConflictsQueued: 'Queued for repair',
      repairConflictsRunning: 'Preparing repair',
      repairConflictsScan: 'Scanning conflict groups',
      repairConflictsResolving: 'Repairing current conflict group',
      repairConflictsPersist: 'Writing repaired pages',
      repairConflictsLint: 'Refreshing lint report',
    }
  ),
  'en-GB': buildKnowledgeConflictRepairBackfill(
    ['Repair conflicts', 'Knowledge conflict repair started.', 'Knowledge conflicts repaired.'],
    {
      repairConflictsProgress: 'Conflict repair in progress',
      repairConflictsCurrent: 'Currently processing',
      repairConflictsQueued: 'Queued for repair',
      repairConflictsRunning: 'Preparing repair',
      repairConflictsScan: 'Scanning conflict groups',
      repairConflictsResolving: 'Repairing current conflict group',
      repairConflictsPersist: 'Writing repaired pages',
      repairConflictsLint: 'Refreshing lint report',
    }
  ),
  'zh-CN': buildKnowledgeConflictRepairBackfill(
    ['修复冲突', '知识冲突修复已开始。', '知识冲突已修复。'],
    {
      repairConflictsProgress: '冲突修复进行中',
      repairConflictsCurrent: '当前处理',
      repairConflictsQueued: '已加入修复队列',
      repairConflictsRunning: '正在准备修复',
      repairConflictsScan: '正在扫描冲突分组',
      repairConflictsResolving: '正在修复当前冲突组',
      repairConflictsPersist: '正在写回修复后的页面',
      repairConflictsLint: '正在刷新 lint 报告',
    }
  ),
  'zh-TW': buildKnowledgeConflictRepairBackfill(
    ['修復衝突', '知識衝突修復已開始。', '知識衝突已修復。'],
    {
      repairConflictsProgress: '衝突修復進行中',
      repairConflictsCurrent: '目前處理',
      repairConflictsQueued: '已加入修復佇列',
      repairConflictsRunning: '正在準備修復',
      repairConflictsScan: '正在掃描衝突分組',
      repairConflictsResolving: '正在修復目前衝突組',
      repairConflictsPersist: '正在寫回修復後的頁面',
      repairConflictsLint: '正在刷新 lint 報告',
    }
  ),
  'ja-JP': buildKnowledgeConflictRepairBackfill([
    '競合を修復',
    'ナレッジ競合の修復を開始しました。',
    'ナレッジ競合を修復しました。',
  ]),
  'ko-KR': buildKnowledgeConflictRepairBackfill([
    '충돌 수정',
    '지식 충돌 수정을 시작했습니다.',
    '지식 충돌을 수정했습니다.',
  ]),
  'fr-FR': buildKnowledgeConflictRepairBackfill([
    'Réparer les conflits',
    'La réparation des conflits de connaissances a commencé.',
    'Les conflits de connaissances ont été réparés.',
  ]),
  'de-DE': buildKnowledgeConflictRepairBackfill([
    'Konflikte beheben',
    'Die Behebung der Wissenskonflikte wurde gestartet.',
    'Die Wissenskonflikte wurden behoben.',
  ]),
  'es-ES': buildKnowledgeConflictRepairBackfill([
    'Reparar conflictos',
    'La reparación de conflictos de conocimiento ha comenzado.',
    'Se repararon los conflictos de conocimiento.',
  ]),
  'pt-BR': buildKnowledgeConflictRepairBackfill([
    'Corrigir conflitos',
    'A correção de conflitos de conhecimento foi iniciada.',
    'Os conflitos de conhecimento foram corrigidos.',
  ]),
  'pt-PT': buildKnowledgeConflictRepairBackfill([
    'Corrigir conflitos',
    'A correção de conflitos de conhecimento foi iniciada.',
    'Os conflitos de conhecimento foram corrigidos.',
  ]),
  'it-IT': buildKnowledgeConflictRepairBackfill([
    'Ripara conflitti',
    'La riparazione dei conflitti di conoscenza è iniziata.',
    'I conflitti di conoscenza sono stati riparati.',
  ]),
  'nl-NL': buildKnowledgeConflictRepairBackfill([
    'Conflicten herstellen',
    'Het herstellen van kennisconflicten is gestart.',
    'Kennisconflicten zijn hersteld.',
  ]),
  'pl-PL': buildKnowledgeConflictRepairBackfill([
    'Napraw konflikty',
    'Naprawa konfliktów wiedzy została rozpoczęta.',
    'Konflikty wiedzy zostały naprawione.',
  ]),
  'ru-RU': buildKnowledgeConflictRepairBackfill([
    'Исправить конфликты',
    'Запущено исправление конфликтов знаний.',
    'Конфликты знаний исправлены.',
  ]),
  'sv-SE': buildKnowledgeConflictRepairBackfill([
    'Reparera konflikter',
    'Reparation av kunskapskonflikter har startat.',
    'Kunskapskonflikter har reparerats.',
  ]),
  'nb-NO': buildKnowledgeConflictRepairBackfill([
    'Reparer konflikter',
    'Reparasjon av kunnskapskonflikter har startet.',
    'Kunnskapskonflikter er reparert.',
  ]),
  'da-DK': buildKnowledgeConflictRepairBackfill([
    'Ret konflikter',
    'Reparation af videnskonflikter er startet.',
    'Videnskonflikter er rettet.',
  ]),
  'cs-CZ': buildKnowledgeConflictRepairBackfill([
    'Opravit konflikty',
    'Oprava konfliktů znalostí byla spuštěna.',
    'Konflikty znalostí byly opraveny.',
  ]),
  'sk-SK': buildKnowledgeConflictRepairBackfill([
    'Opraviť konflikty',
    'Oprava konfliktov znalostí sa začala.',
    'Konflikty znalostí boli opravené.',
  ]),
  'ro-RO': buildKnowledgeConflictRepairBackfill([
    'Repară conflictele',
    'Repararea conflictelor de cunoștințe a început.',
    'Conflictele de cunoștințe au fost reparate.',
  ]),
  'hu-HU': buildKnowledgeConflictRepairBackfill([
    'Ütközések javítása',
    'A tudásütközések javítása elindult.',
    'A tudásütközések javítva.',
  ]),
  'hr-HR': buildKnowledgeConflictRepairBackfill([
    'Popravi sukobe',
    'Popravljanje sukoba znanja je započelo.',
    'Sukobi znanja su popravljeni.',
  ]),
  'ga-IE': buildKnowledgeConflictRepairBackfill([
    'Deisigh coinbhleachtaí',
    'Tá deisiú coinbhleachtaí eolais tosaithe.',
    'Tá coinbhleachtaí eolais deisithe.',
  ]),
  'ca-ES': buildKnowledgeConflictRepairBackfill([
    'Resol els conflictes',
    'La reparació de conflictes de coneixement ha començat.',
    "Els conflictes de coneixement s'han reparat.",
  ]),
  'el-GR': buildKnowledgeConflictRepairBackfill([
    'Διόρθωση συγκρούσεων',
    'Η επιδιόρθωση των συγκρούσεων γνώσης ξεκίνησε.',
    'Οι συγκρούσεις γνώσης επιδιορθώθηκαν.',
  ]),
  'ml-IN': buildKnowledgeConflictRepairBackfill([
    'സംഘര്‍ഷങ്ങള്‍ പരിഹരിക്കുക',
    'ജ്ഞാന സംഘര്‍ഷങ്ങളുടെ പരിഹാരം ആരംഭിച്ചു.',
    'ജ്ഞാന സംഘര്‍ഷങ്ങള്‍ പരിഹരിച്ചു.',
  ]),
}

export default knowledgeConflictRepairBackfills
