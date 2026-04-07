import type { LocaleKey } from './locale-catalog'

type LocaleLeaf = string | number | boolean | null
type LocaleNode = { [key: string]: LocaleLeaf | LocaleNode }

function isPlainObject(value: unknown): value is LocaleNode {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

function mergeLocaleNodes(base: LocaleNode, override: LocaleNode): LocaleNode {
  const merged: LocaleNode = { ...base }

  for (const [key, value] of Object.entries(override)) {
    const current = merged[key]
    if (isPlainObject(current) && isPlainObject(value)) {
      merged[key] = mergeLocaleNodes(current, value)
      continue
    }
    merged[key] = value
  }

  return merged
}

type EvolutionRuntimeLocaleCopy = {
  timeline: string
  lineage: string
  noDiff: string
  caseStatusOpen: string
  timelineHint: string
  followupEvalRun: string
  openFollowupEval: string
  linksSubtitle: string
  followupHint: string
  followupTitle: string
  followupSubtitle: string
  followupEmpty: string
  comparisonHint: string
  metricsHint: string
}

function buildEvolutionRuntimeBackfill(copy: EvolutionRuntimeLocaleCopy): LocaleNode {
  return {
    evolution: {
      noDiff: copy.noDiff,
      skills: {
        timeline: copy.timeline,
        lineage: copy.lineage,
        caseStatus: {
          open: copy.caseStatusOpen,
        },
        timelineHint: copy.timelineHint,
        comparisonHint: copy.comparisonHint,
        metricsHint: copy.metricsHint,
      },
      runner: {
        links: {
          followupEval: copy.followupEvalRun,
          openFollowupEval: copy.openFollowupEval,
          subtitle: copy.linksSubtitle,
        },
        summary: {
          followupHint: copy.followupHint,
        },
        metrics: {
          followupTitle: copy.followupTitle,
          followupSubtitle: copy.followupSubtitle,
          empty: copy.followupEmpty,
        },
      },
    },
  }
}

function buildEvolutionScorecardEvidenceHintBackfill(hint: string): LocaleNode {
  return {
    evolution: {
      skills: {
        scorecardEvidenceHint: hint,
      },
    },
  }
}

type EvolutionScorecardActionCopy = {
  openComparison: string
  openMetrics: string
  openEvidence: string
}

function buildEvolutionScorecardActionBackfill(copy: EvolutionScorecardActionCopy): LocaleNode {
  return {
    evolution: {
      skills: {
        scorecardOpenComparison: copy.openComparison,
        scorecardOpenMetrics: copy.openMetrics,
        scorecardOpenEvidence: copy.openEvidence,
      },
    },
  }
}

const baseEvolutionUILocaleBackfills: Partial<Record<LocaleKey, object>> = {
  'ca-ES': {
    automation: {
      tabs: {
        knowledge: 'Coneixement',
        evolution: 'Evolució',
      },
    },
    evolution: {
      tabs: {
        skills: 'Habilitats',
        runner: 'Executor',
      },
    },
    settings: {
      agentcoreRunner: {
        source: 'Repositori GitHub i referència',
      },
    },
    harness: {
      dataset: {
        githubBundle: 'Paquet de GitHub',
      },
    },
  },
  'cs-CZ': {
    automation: {
      tabs: {
        knowledge: 'Znalosti',
        evolution: 'Evoluce',
      },
    },
    evolution: {
      tabs: {
        skills: 'Dovednosti',
        runner: 'Spouštěč',
      },
    },
    settings: {
      agentcoreRunner: {
        source: 'Repozitář GitHub a reference',
      },
    },
    harness: {
      dataset: {
        githubBundle: 'Balíček z GitHubu',
      },
    },
  },
  'da-DK': {
    automation: {
      tabs: {
        knowledge: 'Viden',
        evolution: 'Udvikling',
      },
    },
    evolution: {
      tabs: {
        skills: 'Færdigheder',
        runner: 'Kører',
      },
    },
    settings: {
      agentcoreRunner: {
        source: 'GitHub-repo og reference',
      },
    },
    harness: {
      dataset: {
        githubBundle: 'GitHub-pakke',
      },
    },
  },
  'de-DE': {
    automation: {
      tabs: {
        knowledge: 'Wissen',
        evolution: 'Weiterentwicklung',
      },
    },
    evolution: {
      tabs: {
        skills: 'Fähigkeiten',
        runner: 'Ausführer',
      },
    },
    settings: {
      agentcoreRunner: {
        source: 'GitHub-Repository und Referenz',
      },
    },
    harness: {
      dataset: {
        githubBundle: 'GitHub-Paket',
      },
    },
  },
  'el-GR': {
    automation: {
      tabs: {
        knowledge: 'Γνώση',
        evolution: 'Εξέλιξη',
      },
    },
    evolution: {
      tabs: {
        skills: 'Δεξιότητες',
        runner: 'Εκτελεστής',
      },
    },
    settings: {
      agentcoreRunner: {
        source: 'Αποθετήριο GitHub και αναφορά',
      },
    },
    harness: {
      dataset: {
        githubBundle: 'Πακέτο GitHub',
      },
    },
  },
  'en-GB': {
    automation: {
      tabs: {
        knowledge: 'Knowledge',
        evolution: 'Evolution',
      },
    },
    evolution: {
      tabs: {
        skills: 'Skills',
        runner: 'Runner',
      },
    },
    settings: {
      agentcoreRunner: {
        source: 'GitHub Repository & Ref',
      },
    },
    harness: {
      dataset: {
        githubBundle: 'GitHub bundle',
      },
    },
  },
  'en-US': {
    automation: {
      tabs: {
        knowledge: 'Knowledge',
        evolution: 'Evolution',
      },
    },
    evolution: {
      tabs: {
        skills: 'Skills',
        runner: 'Runner',
      },
    },
    settings: {
      agentcoreRunner: {
        source: 'GitHub Repo & Ref',
      },
    },
    harness: {
      dataset: {
        githubBundle: 'GitHub bundle',
      },
    },
  },
  'es-ES': {
    automation: {
      tabs: {
        knowledge: 'Conocimiento',
        evolution: 'Evolución',
      },
    },
    evolution: {
      tabs: {
        skills: 'Habilidades',
        runner: 'Ejecutor',
      },
    },
    settings: {
      agentcoreRunner: {
        source: 'Repositorio de GitHub y referencia',
      },
    },
    harness: {
      dataset: {
        githubBundle: 'Paquete de GitHub',
      },
    },
  },
  'fr-FR': {
    automation: {
      tabs: {
        knowledge: 'Connaissance',
        evolution: 'Évolution',
      },
    },
    evolution: {
      tabs: {
        skills: 'Compétences',
        runner: 'Exécuteur',
      },
    },
    settings: {
      agentcoreRunner: {
        source: 'Dépôt GitHub et référence',
      },
    },
    harness: {
      dataset: {
        githubBundle: 'Paquet GitHub',
      },
    },
  },
  'ga-IE': {
    automation: {
      tabs: {
        knowledge: 'Eolas',
        evolution: 'Éabhlóid',
      },
    },
    evolution: {
      tabs: {
        skills: 'Scileanna',
        runner: 'Seoltóir',
      },
    },
    settings: {
      agentcoreRunner: {
        source: 'Stór GitHub agus tagairt',
      },
    },
    harness: {
      dataset: {
        githubBundle: 'Beartán GitHub',
      },
    },
  },
  'hr-HR': {
    automation: {
      tabs: {
        knowledge: 'Znanje',
        evolution: 'Evolucija',
      },
    },
    evolution: {
      tabs: {
        skills: 'Vještine',
        runner: 'Izvršivač',
      },
    },
    settings: {
      agentcoreRunner: {
        source: 'GitHub repozitorij i referenca',
      },
    },
    harness: {
      dataset: {
        githubBundle: 'GitHub paket',
      },
    },
  },
  'hu-HU': {
    automation: {
      tabs: {
        knowledge: 'Tudás',
        evolution: 'Fejlődés',
      },
    },
    evolution: {
      tabs: {
        skills: 'Készségek',
        runner: 'Futtató',
      },
    },
    settings: {
      agentcoreRunner: {
        source: 'GitHub tárhely és hivatkozás',
      },
    },
    harness: {
      dataset: {
        githubBundle: 'GitHub-csomag',
      },
    },
  },
  'it-IT': {
    automation: {
      tabs: {
        knowledge: 'Conoscenza',
        evolution: 'Evoluzione',
      },
    },
    evolution: {
      tabs: {
        skills: 'Competenze',
        runner: 'Esecutore',
      },
    },
    settings: {
      agentcoreRunner: {
        source: 'Repository GitHub e riferimento',
      },
    },
    harness: {
      dataset: {
        githubBundle: 'Pacchetto GitHub',
      },
    },
  },
  'ja-JP': {
    automation: {
      tabs: {
        knowledge: 'ナレッジ',
        evolution: '進化',
      },
    },
    evolution: {
      tabs: {
        skills: 'スキル',
        runner: 'ランナー',
      },
    },
    settings: {
      agentcoreRunner: {
        source: 'GitHub リポジトリと参照',
      },
    },
    harness: {
      dataset: {
        githubBundle: 'GitHub バンドル',
      },
    },
  },
  'ko-KR': {
    automation: {
      tabs: {
        knowledge: '지식',
        evolution: '진화',
      },
    },
    evolution: {
      tabs: {
        skills: '스킬',
        runner: '러너',
      },
    },
    settings: {
      agentcoreRunner: {
        source: 'GitHub 저장소 및 참조',
      },
    },
    harness: {
      dataset: {
        githubBundle: 'GitHub 번들',
      },
    },
  },
  'ml-IN': {
    automation: {
      tabs: {
        knowledge: 'വിജ്ഞാനം',
        evolution: 'പരിണാമം',
      },
    },
    evolution: {
      tabs: {
        skills: 'കഴിവുകൾ',
        runner: 'റണ്ണർ',
      },
    },
    settings: {
      agentcoreRunner: {
        source: 'GitHub ശേഖരവും റഫറൻസും',
      },
    },
    harness: {
      dataset: {
        githubBundle: 'GitHub ബണ്ടിൽ',
      },
    },
  },
  'nb-NO': {
    automation: {
      tabs: {
        knowledge: 'Kunnskap',
        evolution: 'Utvikling',
      },
    },
    evolution: {
      tabs: {
        skills: 'Ferdigheter',
        runner: 'Kjører',
      },
    },
    settings: {
      agentcoreRunner: {
        source: 'GitHub-repositorium og referanse',
      },
    },
    harness: {
      dataset: {
        githubBundle: 'GitHub-pakke',
      },
    },
  },
  'nl-NL': {
    automation: {
      tabs: {
        knowledge: 'Kennis',
        evolution: 'Evolutie',
      },
    },
    evolution: {
      tabs: {
        skills: 'Vaardigheden',
        runner: 'Uitvoerder',
      },
    },
    settings: {
      agentcoreRunner: {
        source: 'GitHub-repository en referentie',
      },
    },
    harness: {
      dataset: {
        githubBundle: 'GitHub-bundel',
      },
    },
  },
  'pl-PL': {
    automation: {
      tabs: {
        knowledge: 'Wiedza',
        evolution: 'Ewolucja',
      },
    },
    evolution: {
      tabs: {
        skills: 'Umiejętności',
        runner: 'Uruchamiacz',
      },
    },
    settings: {
      agentcoreRunner: {
        source: 'Repozytorium GitHub i odniesienie',
      },
    },
    harness: {
      dataset: {
        githubBundle: 'Pakiet GitHub',
      },
    },
  },
  'pt-BR': {
    automation: {
      tabs: {
        knowledge: 'Conhecimento',
        evolution: 'Evolução',
      },
    },
    evolution: {
      tabs: {
        skills: 'Habilidades',
        runner: 'Executor',
      },
    },
    settings: {
      agentcoreRunner: {
        source: 'Repositório GitHub e referência',
      },
    },
    harness: {
      dataset: {
        githubBundle: 'Pacote do GitHub',
      },
    },
  },
  'pt-PT': {
    automation: {
      tabs: {
        knowledge: 'Conhecimento',
        evolution: 'Evolução',
      },
    },
    evolution: {
      tabs: {
        skills: 'Competências',
        runner: 'Executor',
      },
    },
    settings: {
      agentcoreRunner: {
        source: 'Repositório GitHub e referência',
      },
    },
    harness: {
      dataset: {
        githubBundle: 'Pacote do GitHub',
      },
    },
  },
  'ro-RO': {
    automation: {
      tabs: {
        knowledge: 'Cunoștințe',
        evolution: 'Evoluție',
      },
    },
    evolution: {
      tabs: {
        skills: 'Abilități',
        runner: 'Executor',
      },
    },
    settings: {
      agentcoreRunner: {
        source: 'Depozit GitHub și referință',
      },
    },
    harness: {
      dataset: {
        githubBundle: 'Pachet GitHub',
      },
    },
  },
  'ru-RU': {
    automation: {
      tabs: {
        knowledge: 'Знания',
        evolution: 'Эволюция',
      },
    },
    evolution: {
      tabs: {
        skills: 'Навыки',
        runner: 'Исполнитель',
      },
    },
    settings: {
      agentcoreRunner: {
        source: 'Репозиторий GitHub и ссылка',
      },
    },
    harness: {
      dataset: {
        githubBundle: 'Пакет GitHub',
      },
    },
  },
  'sk-SK': {
    automation: {
      tabs: {
        knowledge: 'Znalosti',
        evolution: 'Evolúcia',
      },
    },
    evolution: {
      tabs: {
        skills: 'Zručnosti',
        runner: 'Spúšťač',
      },
    },
    settings: {
      agentcoreRunner: {
        source: 'Repozitár GitHub a referencia',
      },
    },
    harness: {
      dataset: {
        githubBundle: 'Balík GitHub',
      },
    },
  },
  'sv-SE': {
    automation: {
      tabs: {
        knowledge: 'Kunskap',
        evolution: 'Utveckling',
      },
    },
    evolution: {
      tabs: {
        skills: 'Färdigheter',
        runner: 'Körare',
      },
    },
    settings: {
      agentcoreRunner: {
        source: 'GitHub-repo och referens',
      },
    },
    harness: {
      dataset: {
        githubBundle: 'GitHub-paket',
      },
    },
  },
  'zh-CN': {
    automation: {
      tabs: {
        knowledge: '知识',
        evolution: '进化',
      },
    },
    evolution: {
      tabs: {
        skills: '技能',
        runner: '运行器',
      },
    },
    settings: {
      agentcoreRunner: {
        source: 'GitHub 仓库与引用',
      },
    },
    harness: {
      dataset: {
        githubBundle: 'GitHub 包',
      },
    },
  },
  'zh-TW': {
    automation: {
      tabs: {
        knowledge: '知識',
        evolution: '演進',
      },
    },
    evolution: {
      tabs: {
        skills: '技能',
        runner: '執行器',
      },
    },
    settings: {
      agentcoreRunner: {
        source: 'GitHub 儲存庫與引用',
      },
    },
    harness: {
      dataset: {
        githubBundle: 'GitHub 套件',
      },
    },
  },
}

const evolutionRuntimeLocaleBackfills: Partial<Record<LocaleKey, object>> = {
  'ca-ES': buildEvolutionRuntimeBackfill({
    timeline: 'Cicle de vida del cas',
    lineage: 'Resum del llinatge de revisions',
    noDiff: 'Encara no hi ha cap pedaç candidat disponible.',
    caseStatusOpen: 'Entrada oberta, pendent de candidat',
    timelineHint:
      "Segueix el cas seleccionat des de l'entrada fins a la promoció final en un sol lloc.",
    followupEvalRun: "Execució de l'avaluació de seguiment",
    openFollowupEval: "Obre l'avaluació de seguiment",
    linksSubtitle:
      "Fes servir aquests enllaços per obrir l'avaluació de seguiment, la revisió vinculada de l'skill o l'avaluació d'origen.",
    followupHint:
      "L'avaluació de seguiment vinculada decideix si el candidat s'accepta, es rebutja o continua en execució.",
    followupTitle: "Instantània de l'avaluació de seguiment",
    followupSubtitle:
      "Aquestes mètriques provenen de l'avaluació de seguiment vinculada i haurien de guiar l'acceptació o el rebuig abans de qualsevol promoció manual.",
    followupEmpty: "Encara no hi ha mètriques estructurades adjuntes de l'avaluació de seguiment.",
    comparisonHint:
      'Quan una avaluació de seguiment inclou una línia base, aquestes diferències expliquen per què la revisió seleccionada és millor o pitjor que la versió anterior.',
    metricsHint:
      "Les mètriques de qualitat, temps d'execució i tokens provenen de l'informe de l'avaluació de seguiment vinculada quan està disponible.",
  }),
  'cs-CZ': buildEvolutionRuntimeBackfill({
    timeline: 'Životní cyklus případu',
    lineage: 'Souhrn návaznosti revizí',
    noDiff: 'Oprava kandidáta zatím není k dispozici.',
    caseStatusOpen: 'Příjem otevřen, čeká se na kandidáta',
    timelineHint: 'Sledujte vybraný případ od příjmu až po finální povýšení na jednom místě.',
    followupEvalRun: 'Běh následného vyhodnocení',
    openFollowupEval: 'Otevřít následné vyhodnocení',
    linksSubtitle:
      'Pomocí těchto odkazů otevřete následné vyhodnocení, propojenou revizi dovednosti nebo zdrojové vyhodnocení.',
    followupHint:
      'Propojené následné vyhodnocení rozhoduje, zda je kandidát přijat, zamítnut nebo stále běží.',
    followupTitle: 'Snímek následného vyhodnocení',
    followupSubtitle:
      'Tyto metriky pocházejí z propojeného následného vyhodnocení a měly by řídit rozhodnutí o přijetí nebo zamítnutí před jakýmkoli ručním povýšením.',
    followupEmpty: 'Zatím nejsou připojeny žádné strukturované metriky následného vyhodnocení.',
    comparisonHint:
      'Když následné vyhodnocení obsahuje základní linii, tyto rozdíly vysvětlují, proč je vybraná revize lepší nebo horší než předchozí verze.',
    metricsHint:
      'Metriky kvality, běhu a tokenů pocházejí z propojené zprávy následného vyhodnocení, pokud je k dispozici.',
  }),
  'da-DK': buildEvolutionRuntimeBackfill({
    timeline: 'Sagens livscyklus',
    lineage: 'Oversigt over revisionslinje',
    noDiff: 'Ingen kandidatpatch er tilgængelig endnu.',
    caseStatusOpen: 'Modtagelse åben, venter på kandidat',
    timelineHint: 'Følg den valgte sag fra modtagelse til endelig promovering ét sted.',
    followupEvalRun: 'Kørsel for opfølgende evaluering',
    openFollowupEval: 'Åbn opfølgende evaluering',
    linksSubtitle:
      'Brug disse links til at åbne den opfølgende evaluering, den tilknyttede skill-revision eller kildeevalueringen.',
    followupHint:
      'Den tilknyttede opfølgende evaluering afgør, om kandidaten accepteres, afvises eller stadig kører.',
    followupTitle: 'Øjebliksbillede af opfølgende evaluering',
    followupSubtitle:
      'Disse målinger kommer fra den tilknyttede opfølgende evaluering og bør styre beslutningen om at acceptere eller afvise før enhver manuel promovering.',
    followupEmpty: 'Ingen strukturerede målinger fra den opfølgende evaluering er vedhæftet endnu.',
    comparisonHint:
      'Når en opfølgende evaluering indeholder en baseline, forklarer disse deltaer, hvorfor den valgte revision er bedre eller dårligere end den forrige version.',
    metricsHint:
      'Kvalitets-, runtime- og tokenmålinger kommer fra den tilknyttede rapport for opfølgende evaluering, når den er tilgængelig.',
  }),
  'de-DE': buildEvolutionRuntimeBackfill({
    timeline: 'Lebenszyklus des Falls',
    lineage: 'Zusammenfassung der Revisionslinie',
    noDiff: 'Es ist noch kein Kandidaten-Patch verfügbar.',
    caseStatusOpen: 'Aufnahme offen, Kandidat ausstehend',
    timelineHint:
      'Verfolge den ausgewählten Fall von der Aufnahme bis zur endgültigen Übernahme an einem Ort.',
    followupEvalRun: 'Lauf der Folgeauswertung',
    openFollowupEval: 'Folgeauswertung öffnen',
    linksSubtitle:
      'Nutze diese Links, um die Folgeauswertung, die verknüpfte Skill-Revision oder die Quellauswertung zu öffnen.',
    followupHint:
      'Die verknüpfte Folgeauswertung entscheidet, ob der Kandidat akzeptiert, abgelehnt oder noch ausgeführt wird.',
    followupTitle: 'Momentaufnahme der Folgeauswertung',
    followupSubtitle:
      'Diese Metriken stammen aus der verknüpften Folgeauswertung und sollten die Annahme- oder Ablehnungsentscheidung vor jeder manuellen Übernahme steuern.',
    followupEmpty: 'Es sind noch keine strukturierten Metriken der Folgeauswertung angehängt.',
    comparisonHint:
      'Wenn eine Folgeauswertung eine Basis enthält, erklären diese Deltas, warum die ausgewählte Revision besser oder schlechter als die vorherige Version ist.',
    metricsHint:
      'Qualitäts-, Laufzeit- und Token-Metriken stammen aus dem verknüpften Bericht der Folgeauswertung, wenn er verfügbar ist.',
  }),
  'el-GR': buildEvolutionRuntimeBackfill({
    timeline: 'Κύκλος ζωής υπόθεσης',
    lineage: 'Σύνοψη γενεαλογίας αναθεωρήσεων',
    noDiff: 'Δεν υπάρχει ακόμη διαθέσιμο υποψήφιο patch.',
    caseStatusOpen: 'Ανοιχτή εισαγωγή, αναμονή υποψηφίου',
    timelineHint:
      'Παρακολουθήστε την επιλεγμένη υπόθεση από την εισαγωγή έως την τελική προώθηση σε ένα σημείο.',
    followupEvalRun: 'Εκτέλεση αξιολόγησης συνέχειας',
    openFollowupEval: 'Άνοιγμα αξιολόγησης συνέχειας',
    linksSubtitle:
      'Χρησιμοποιήστε αυτούς τους συνδέσμους για να ανοίξετε την αξιολόγηση συνέχειας, τη συνδεδεμένη αναθεώρηση skill ή την αρχική αξιολόγηση.',
    followupHint:
      'Η συνδεδεμένη αξιολόγηση συνέχειας αποφασίζει αν ο υποψήφιος γίνεται αποδεκτός, απορρίπτεται ή συνεχίζει να εκτελείται.',
    followupTitle: 'Στιγμιότυπο αξιολόγησης συνέχειας',
    followupSubtitle:
      'Αυτές οι μετρικές προέρχονται από τη συνδεδεμένη αξιολόγηση συνέχειας και πρέπει να καθοδηγούν την απόφαση αποδοχής ή απόρριψης πριν από οποιαδήποτε χειροκίνητη προώθηση.',
    followupEmpty: 'Δεν έχουν επισυναφθεί ακόμη δομημένες μετρικές αξιολόγησης συνέχειας.',
    comparisonHint:
      'Όταν μια αξιολόγηση συνέχειας περιλαμβάνει baseline, αυτές οι διαφορές εξηγούν γιατί η επιλεγμένη αναθεώρηση είναι καλύτερη ή χειρότερη από την προηγούμενη έκδοση.',
    metricsHint:
      'Οι μετρικές ποιότητας, χρόνου εκτέλεσης και token προέρχονται από τη συνδεδεμένη αναφορά αξιολόγησης συνέχειας όταν είναι διαθέσιμη.',
  }),
  'en-GB': buildEvolutionRuntimeBackfill({
    timeline: 'Case Lifecycle',
    lineage: 'Revision Lineage Summary',
    noDiff: 'No candidate patch available yet.',
    caseStatusOpen: 'Open intake, waiting candidate',
    timelineHint: 'Follow the selected case from intake to final promotion in one place.',
    followupEvalRun: 'Follow-up eval run',
    openFollowupEval: 'Open follow-up eval',
    linksSubtitle:
      'Use these links to open the follow-up eval, linked skill revision, or source eval.',
    followupHint:
      'The linked follow-up eval decides whether the candidate is accepted, rejected, or still running.',
    followupTitle: 'Follow-up Eval Snapshot',
    followupSubtitle:
      'These metrics come from the linked follow-up eval and should guide accept or reject before any manual promotion.',
    followupEmpty: 'No structured follow-up eval metrics are attached yet.',
    comparisonHint:
      'When a follow-up eval includes a baseline, these deltas explain why the selected revision is better or worse than the previous version.',
    metricsHint:
      'Quality, runtime, and token metrics come from the linked follow-up eval report when available.',
  }),
  'en-US': buildEvolutionRuntimeBackfill({
    timeline: 'Case Lifecycle',
    lineage: 'Revision Lineage Summary',
    noDiff: 'No candidate patch available yet.',
    caseStatusOpen: 'Open intake, waiting candidate',
    timelineHint:
      'Follow the selected case from intake through candidate creation, gate decision, and final promotion without reconstructing the history from scattered timestamps.',
    followupEvalRun: 'Follow-up eval run',
    openFollowupEval: 'Open follow-up eval',
    linksSubtitle:
      'Use these links to jump from the runner evidence into the follow-up eval, the linked skill revision, or the original source eval that produced the optimization trigger.',
    followupHint:
      'The linked follow-up eval decides whether the candidate is accepted, rejected, or still running.',
    followupTitle: 'Follow-up Eval Snapshot',
    followupSubtitle:
      'These metrics come from the linked follow-up eval and should drive the accept or reject decision before any human promote step.',
    followupEmpty: 'No structured follow-up eval metrics are attached yet.',
    comparisonHint:
      'When a follow-up eval includes a baseline, these deltas make it clear why the selected revision looks better or worse than the previous version.',
    metricsHint:
      'Quality, runtime, and token metrics come from the linked follow-up eval report when available.',
  }),
  'es-ES': buildEvolutionRuntimeBackfill({
    timeline: 'Ciclo de vida del caso',
    lineage: 'Resumen del linaje de revisiones',
    noDiff: 'Todavía no hay ningún parche candidato disponible.',
    caseStatusOpen: 'Entrada abierta, esperando candidato',
    timelineHint:
      'Sigue el caso seleccionado desde la entrada hasta la promoción final en un solo lugar.',
    followupEvalRun: 'Ejecución de evaluación de seguimiento',
    openFollowupEval: 'Abrir evaluación de seguimiento',
    linksSubtitle:
      'Usa estos enlaces para abrir la evaluación de seguimiento, la revisión de skill vinculada o la evaluación de origen.',
    followupHint:
      'La evaluación de seguimiento vinculada decide si el candidato se acepta, se rechaza o sigue en ejecución.',
    followupTitle: 'Instantánea de la evaluación de seguimiento',
    followupSubtitle:
      'Estas métricas proceden de la evaluación de seguimiento vinculada y deben guiar la decisión de aceptar o rechazar antes de cualquier promoción manual.',
    followupEmpty:
      'Todavía no hay métricas estructuradas adjuntas de la evaluación de seguimiento.',
    comparisonHint:
      'Cuando una evaluación de seguimiento incluye una línea base, estas diferencias explican por qué la revisión seleccionada es mejor o peor que la versión anterior.',
    metricsHint:
      'Las métricas de calidad, tiempo de ejecución y tokens proceden del informe de la evaluación de seguimiento vinculada cuando está disponible.',
  }),
  'fr-FR': buildEvolutionRuntimeBackfill({
    timeline: 'Cycle de vie du cas',
    lineage: 'Résumé de la lignée des révisions',
    noDiff: "Aucun correctif candidat n'est encore disponible.",
    caseStatusOpen: "Entrée ouverte, en attente d'un candidat",
    timelineHint: "Suivez le cas sélectionné de l'entrée à la promotion finale au même endroit.",
    followupEvalRun: "Exécution de l'évaluation de suivi",
    openFollowupEval: "Ouvrir l'évaluation de suivi",
    linksSubtitle:
      "Utilisez ces liens pour ouvrir l'évaluation de suivi, la révision de compétence liée ou l'évaluation source.",
    followupHint:
      "L'évaluation de suivi liée décide si le candidat est accepté, rejeté ou toujours en cours.",
    followupTitle: "Instantané de l'évaluation de suivi",
    followupSubtitle:
      "Ces métriques proviennent de l'évaluation de suivi liée et doivent guider la décision d'accepter ou de rejeter avant toute promotion manuelle.",
    followupEmpty: "Aucune métrique structurée de l'évaluation de suivi n'est encore jointe.",
    comparisonHint:
      "Lorsqu'une évaluation de suivi inclut une référence, ces écarts expliquent pourquoi la révision sélectionnée est meilleure ou pire que la version précédente.",
    metricsHint:
      "Les métriques de qualité, d'exécution et de tokens proviennent du rapport d'évaluation de suivi lié lorsqu'il est disponible.",
  }),
  'ga-IE': buildEvolutionRuntimeBackfill({
    timeline: 'Saolré an cháis',
    lineage: 'Achoimre ar shinsearacht athbhreithnithe',
    noDiff: 'Níl aon phaiste iarrthóra ar fáil fós.',
    caseStatusOpen: 'Ionghabháil oscailte, iarrthóir fós le teacht',
    timelineHint:
      'Lean an cás roghnaithe ón ionghabháil go dtí an cur chun cinn deiridh in aon áit amháin.',
    followupEvalRun: 'Rith meastóireachta leantaí',
    openFollowupEval: 'Oscail meastóireacht leantach',
    linksSubtitle:
      'Úsáid na naisc seo chun an mheastóireacht leantach, an t-athbhreithniú scile nasctha nó an mheastóireacht fhoinse a oscailt.',
    followupHint:
      'Cinneann an mheastóireacht leantach nasctha an nglactar leis an iarrthóir, an ndiúltaítear dó, nó an bhfuil sé fós ag rith.',
    followupTitle: 'Léargas meastóireachta leantaí',
    followupSubtitle:
      'Tagann na méadrachtaí seo ón mheastóireacht leantach nasctha agus ba chóir dóibh an cinneadh glactha nó diúltaithe a threorú sula ndéantar aon chur chun cinn de láimh.',
    followupEmpty: 'Níl aon mhéadrachtaí struchtúrtha meastóireachta leantaí ceangailte fós.',
    comparisonHint:
      'Nuair a chuimsíonn meastóireacht leantach bunlíne, míníonn na difríochtaí seo cén fáth a bhfuil an t-athbhreithniú roghnaithe níos fearr nó níos measa ná an leagan roimhe.',
    metricsHint:
      'Tagann méadrachtaí cáilíochta, ama reatha agus token ón tuarascáil mheastóireachta leantaí nasctha nuair atá sí ar fáil.',
  }),
  'hr-HR': buildEvolutionRuntimeBackfill({
    timeline: 'Životni ciklus slučaja',
    lineage: 'Sažetak loze revizija',
    noDiff: 'Zakrpa kandidata još nije dostupna.',
    caseStatusOpen: 'Prijem otvoren, čeka se kandidat',
    timelineHint: 'Pratite odabrani slučaj od prijema do konačne promocije na jednom mjestu.',
    followupEvalRun: 'Pokretanje naknadne procjene',
    openFollowupEval: 'Otvori naknadnu procjenu',
    linksSubtitle:
      'Koristite ove poveznice za otvaranje naknadne procjene, povezane revizije vještine ili izvorne procjene.',
    followupHint:
      'Povezana naknadna procjena odlučuje prihvaća li se kandidat, odbija ili je još u tijeku.',
    followupTitle: 'Snimka naknadne procjene',
    followupSubtitle:
      'Ove metrike dolaze iz povezane naknadne procjene i trebale bi voditi odluku o prihvaćanju ili odbijanju prije bilo kakve ručne promocije.',
    followupEmpty: 'Još nisu priložene strukturirane metrike naknadne procjene.',
    comparisonHint:
      'Kada naknadna procjena uključuje osnovnu liniju, ove razlike objašnjavaju zašto je odabrana revizija bolja ili lošija od prethodne verzije.',
    metricsHint:
      'Metrike kvalitete, vremena izvođenja i tokena dolaze iz povezanog izvješća naknadne procjene kada je dostupno.',
  }),
  'hu-HU': buildEvolutionRuntimeBackfill({
    timeline: 'Eset életciklusa',
    lineage: 'Revíziós leszármazási összefoglaló',
    noDiff: 'Még nincs elérhető jelöltfolt.',
    caseStatusOpen: 'Felvétel nyitva, jelöltre vár',
    timelineHint: 'Kövesse a kiválasztott esetet a felvételtől a végső előléptetésig egy helyen.',
    followupEvalRun: 'Utókövetési értékelés futása',
    openFollowupEval: 'Utókövetési értékelés megnyitása',
    linksSubtitle:
      'Ezekkel a hivatkozásokkal megnyithatja az utókövetési értékelést, a kapcsolt skill-revíziót vagy a forrásértékelést.',
    followupHint:
      'A kapcsolt utókövetési értékelés dönti el, hogy a jelölt elfogadott, elutasított vagy még fut.',
    followupTitle: 'Utókövetési értékelési pillanatkép',
    followupSubtitle:
      'Ezek a metrikák a kapcsolt utókövetési értékelésből származnak, és minden kézi előléptetés előtt ezeknek kell irányítaniuk az elfogadási vagy elutasítási döntést.',
    followupEmpty: 'Még nincsenek csatolva strukturált utókövetési értékelési metrikák.',
    comparisonHint:
      'Ha egy utókövetési értékelés tartalmaz alapvonalat, ezek az eltérések megmutatják, miért jobb vagy rosszabb a kiválasztott revízió az előző verziónál.',
    metricsHint:
      'A minőségi, futásidejű és token metrikák a kapcsolt utókövetési értékelési jelentésből származnak, amikor elérhető.',
  }),
  'it-IT': buildEvolutionRuntimeBackfill({
    timeline: 'Ciclo di vita del caso',
    lineage: 'Riepilogo della genealogia delle revisioni',
    noDiff: 'Nessuna patch candidata è ancora disponibile.',
    caseStatusOpen: 'Acquisizione aperta, in attesa di un candidato',
    timelineHint:
      "Segui il caso selezionato dall'acquisizione alla promozione finale in un unico punto.",
    followupEvalRun: 'Esecuzione della valutazione successiva',
    openFollowupEval: 'Apri la valutazione successiva',
    linksSubtitle:
      'Usa questi collegamenti per aprire la valutazione successiva, la revisione della skill collegata o la valutazione di origine.',
    followupHint:
      'La valutazione successiva collegata decide se il candidato viene accettato, rifiutato o è ancora in esecuzione.',
    followupTitle: 'Istantanea della valutazione successiva',
    followupSubtitle:
      'Queste metriche provengono dalla valutazione successiva collegata e dovrebbero guidare la decisione di accettare o rifiutare prima di qualsiasi promozione manuale.',
    followupEmpty: 'Non sono ancora allegate metriche strutturate della valutazione successiva.',
    comparisonHint:
      'Quando una valutazione successiva include una baseline, queste differenze spiegano perché la revisione selezionata è migliore o peggiore della versione precedente.',
    metricsHint:
      'Le metriche di qualità, runtime e token provengono dal report della valutazione successiva collegata quando disponibile.',
  }),
  'ja-JP': buildEvolutionRuntimeBackfill({
    timeline: '案件ライフサイクル',
    lineage: '改訂系統サマリー',
    noDiff: '候補パッチはまだありません。',
    caseStatusOpen: '受付中、候補を待機中',
    timelineHint: '選択した案件を受付から最終昇格まで一か所で追跡できます。',
    followupEvalRun: '後続評価の実行',
    openFollowupEval: '後続評価を開く',
    linksSubtitle: 'これらのリンクから、後続評価、関連スキル改訂、または元の評価を開けます。',
    followupHint:
      '関連する後続評価が、その候補を受け入れるか、却下するか、まだ実行中かを決定します。',
    followupTitle: '後続評価スナップショット',
    followupSubtitle:
      'これらの指標は関連する後続評価から取得され、手動昇格の前に受け入れか却下かの判断を導くべきです。',
    followupEmpty: '構造化された後続評価メトリクスはまだ添付されていません。',
    comparisonHint:
      '後続評価にベースラインが含まれる場合、これらの差分によって選択した改訂が前の版より優れているか劣っているかが分かります。',
    metricsHint:
      '品質、実行時間、トークンの指標は、利用可能な場合は関連する後続評価レポートから取得されます。',
  }),
  'ko-KR': buildEvolutionRuntimeBackfill({
    timeline: '사례 수명 주기',
    lineage: '리비전 계보 요약',
    noDiff: '후보 패치가 아직 없습니다.',
    caseStatusOpen: '접수됨, 후보 대기 중',
    timelineHint: '선택한 사례를 접수부터 최종 승격까지 한곳에서 추적합니다.',
    followupEvalRun: '후속 평가 실행',
    openFollowupEval: '후속 평가 열기',
    linksSubtitle: '이 링크로 후속 평가, 연결된 스킬 리비전 또는 원본 평가를 열 수 있습니다.',
    followupHint: '연결된 후속 평가는 후보를 수락할지, 거절할지, 아직 실행 중인지 결정합니다.',
    followupTitle: '후속 평가 스냅샷',
    followupSubtitle:
      '이 지표는 연결된 후속 평가에서 오며, 수동 승격 전에 수락 또는 거절 판단을 이끌어야 합니다.',
    followupEmpty: '구조화된 후속 평가 지표가 아직 첨부되지 않았습니다.',
    comparisonHint:
      '후속 평가에 기준선이 포함되면, 이 차이가 선택한 리비전이 이전 버전보다 왜 더 좋은지 또는 나쁜지 보여 줍니다.',
    metricsHint: '품질, 실행 시간, 토큰 지표는 가능할 때 연결된 후속 평가 보고서에서 가져옵니다.',
  }),
  'ml-IN': buildEvolutionRuntimeBackfill({
    timeline: 'കേസ് ജീവിതചക്രം',
    lineage: 'റിവിഷൻ വംശാവലി സംഗ്രഹം',
    noDiff: 'കാൻഡിഡേറ്റ് പാച്ച് ഇതുവരെ ലഭ്യമല്ല.',
    caseStatusOpen: 'സ്വീകരണം തുറന്നിരിക്കുന്നു, കാൻഡിഡേറ്റിനായി കാത്തിരിക്കുന്നു',
    timelineHint:
      'തിരഞ്ഞെടുത്ത കേസ് സ്വീകരണത്തിൽ നിന്ന് അന്തിമ പ്രമോഷൻ വരെ ഒരൊറ്റ ഇടത്തിൽ പിന്തുടരുക.',
    followupEvalRun: 'തുടർ മൂല്യനിർണയ പ്രവർത്തനം',
    openFollowupEval: 'തുടർ മൂല്യനിർണയം തുറക്കുക',
    linksSubtitle:
      'ഈ ലിങ്കുകൾ ഉപയോഗിച്ച് തുടർ മൂല്യനിർണയം, ബന്ധപ്പെട്ട സ്കിൽ റിവിഷൻ, അല്ലെങ്കിൽ ഉറവിട മൂല്യനിർണയം തുറക്കാം.',
    followupHint:
      'ബന്ധപ്പെട്ട തുടർ മൂല്യനിർണയം കാൻഡിഡേറ്റ് സ്വീകരിക്കണോ നിരസിക്കണോ ഇപ്പോഴും പ്രവർത്തിക്കുകയാണോ എന്ന് തീരുമാനിക്കുന്നു.',
    followupTitle: 'തുടർ മൂല്യനിർണയ സ്നാപ്ഷോട്ട്',
    followupSubtitle:
      'ഈ മെട്രിക്കുകൾ ബന്ധപ്പെട്ട തുടർ മൂല്യനിർണയത്തിൽ നിന്ന് വരുന്നു, അതിനാൽ കൈയോടെ പ്രമോഷൻ ചെയ്യുന്നതിന് മുമ്പ് സ്വീകരിക്കണമോ നിരസിക്കണമോ എന്നത് ഇതു നയിക്കണം.',
    followupEmpty: 'ക്രമബദ്ധമായ തുടർ മൂല്യനിർണയ മെട്രിക്കുകൾ ഇതുവരെ ചേർത്തിട്ടില്ല.',
    comparisonHint:
      'ഒരു തുടർ മൂല്യനിർണയം ബേസ്ലൈൻ ഉൾക്കൊള്ളുമ്പോൾ, തിരഞ്ഞെടുത്ത റിവിഷൻ മുൻ പതിപ്പിനെക്കാൾ എന്തുകൊണ്ട് നല്ലതോ മോശമോ ആണെന്ന് ഈ വ്യത്യാസങ്ങൾ കാണിക്കുന്നു.',
    metricsHint:
      'ഗുണമേന്മ, പ്രവർത്തനസമയം, ടോക്കൺ മെട്രിക്കുകൾ ലഭ്യമായപ്പോൾ ബന്ധപ്പെട്ട തുടർ മൂല്യനിർണയ റിപ്പോർട്ടിൽ നിന്ന് വരുന്നു.',
  }),
  'nb-NO': buildEvolutionRuntimeBackfill({
    timeline: 'Sakens livssyklus',
    lineage: 'Sammendrag av revisjonslinje',
    noDiff: 'Ingen kandidatpatch er tilgjengelig ennå.',
    caseStatusOpen: 'Mottak åpent, venter på kandidat',
    timelineHint: 'Følg den valgte saken fra mottak til endelig promotering på ett sted.',
    followupEvalRun: 'Oppfølgingsvurderingskjøring',
    openFollowupEval: 'Åpne oppfølgingsvurdering',
    linksSubtitle:
      'Bruk disse lenkene til å åpne oppfølgingsvurderingen, den tilknyttede skill-revisjonen eller kildevurderingen.',
    followupHint:
      'Den tilknyttede oppfølgingsvurderingen avgjør om kandidaten godtas, avvises eller fortsatt kjører.',
    followupTitle: 'Øyeblikksbilde av oppfølgingsvurdering',
    followupSubtitle:
      'Disse målingene kommer fra den tilknyttede oppfølgingsvurderingen og bør styre beslutningen om å godta eller avvise før manuell promotering.',
    followupEmpty: 'Ingen strukturerte målinger fra oppfølgingsvurderingen er vedlagt ennå.',
    comparisonHint:
      'Når en oppfølgingsvurdering inkluderer en basislinje, forklarer disse forskjellene hvorfor den valgte revisjonen er bedre eller dårligere enn forrige versjon.',
    metricsHint:
      'Kvalitets-, kjøretids- og tokenmålinger kommer fra den tilknyttede rapporten for oppfølgingsvurdering når den er tilgjengelig.',
  }),
  'nl-NL': buildEvolutionRuntimeBackfill({
    timeline: 'Levenscyclus van de zaak',
    lineage: 'Samenvatting van revisie-afstamming',
    noDiff: 'Er is nog geen kandidaatpatch beschikbaar.',
    caseStatusOpen: 'Ontvangst open, wacht op kandidaat',
    timelineHint: 'Volg de geselecteerde zaak van ontvangst tot definitieve promotie op één plek.',
    followupEvalRun: 'Vervolgevaluatie-uitvoering',
    openFollowupEval: 'Vervolgevaluatie openen',
    linksSubtitle:
      'Gebruik deze links om de vervolgevaluatie, de gekoppelde skill-revisie of de bronevaluatie te openen.',
    followupHint:
      'De gekoppelde vervolgevaluatie bepaalt of de kandidaat wordt geaccepteerd, afgewezen of nog loopt.',
    followupTitle: 'Momentopname van vervolgevaluatie',
    followupSubtitle:
      'Deze metrieken komen uit de gekoppelde vervolgevaluatie en moeten de beslissing om te accepteren of af te wijzen sturen vóór een handmatige promotie.',
    followupEmpty: 'Er zijn nog geen gestructureerde metrieken van de vervolgevaluatie gekoppeld.',
    comparisonHint:
      'Wanneer een vervolgevaluatie een basislijn bevat, verklaren deze verschillen waarom de gekozen revisie beter of slechter is dan de vorige versie.',
    metricsHint:
      'Kwaliteits-, runtime- en tokenmetrieken komen uit het gekoppelde rapport van de vervolgevaluatie wanneer dat beschikbaar is.',
  }),
  'pl-PL': buildEvolutionRuntimeBackfill({
    timeline: 'Cykl życia sprawy',
    lineage: 'Podsumowanie linii rewizji',
    noDiff: 'Łatka kandydata nie jest jeszcze dostępna.',
    caseStatusOpen: 'Przyjęcie otwarte, oczekiwanie na kandydata',
    timelineHint: 'Śledź wybraną sprawę od przyjęcia do końcowej promocji w jednym miejscu.',
    followupEvalRun: 'Uruchomienie oceny uzupełniającej',
    openFollowupEval: 'Otwórz ocenę uzupełniającą',
    linksSubtitle:
      'Użyj tych linków, aby otworzyć ocenę uzupełniającą, powiązaną rewizję skilla lub ocenę źródłową.',
    followupHint:
      'Powiązana ocena uzupełniająca decyduje, czy kandydat zostanie przyjęty, odrzucony czy nadal jest uruchomiony.',
    followupTitle: 'Migawka oceny uzupełniającej',
    followupSubtitle:
      'Te metryki pochodzą z powiązanej oceny uzupełniającej i powinny kierować decyzją o przyjęciu lub odrzuceniu przed jakąkolwiek ręczną promocją.',
    followupEmpty: 'Nie dołączono jeszcze uporządkowanych metryk oceny uzupełniającej.',
    comparisonHint:
      'Gdy ocena uzupełniająca zawiera linię bazową, te różnice wyjaśniają, dlaczego wybrana rewizja jest lepsza lub gorsza od poprzedniej wersji.',
    metricsHint:
      'Metryki jakości, czasu działania i tokenów pochodzą z powiązanego raportu oceny uzupełniającej, gdy jest dostępny.',
  }),
  'pt-BR': buildEvolutionRuntimeBackfill({
    timeline: 'Ciclo de vida do caso',
    lineage: 'Resumo da linhagem das revisões',
    noDiff: 'Ainda não há patch candidato disponível.',
    caseStatusOpen: 'Entrada aberta, aguardando candidato',
    timelineHint: 'Acompanhe o caso selecionado da entrada até a promoção final em um só lugar.',
    followupEvalRun: 'Execução da avaliação de acompanhamento',
    openFollowupEval: 'Abrir avaliação de acompanhamento',
    linksSubtitle:
      'Use estes links para abrir a avaliação de acompanhamento, a revisão de skill vinculada ou a avaliação de origem.',
    followupHint:
      'A avaliação de acompanhamento vinculada decide se o candidato é aceito, rejeitado ou ainda está em execução.',
    followupTitle: 'Instantâneo da avaliação de acompanhamento',
    followupSubtitle:
      'Estas métricas vêm da avaliação de acompanhamento vinculada e devem orientar a decisão de aceitar ou rejeitar antes de qualquer promoção manual.',
    followupEmpty: 'Ainda não há métricas estruturadas anexadas da avaliação de acompanhamento.',
    comparisonHint:
      'Quando uma avaliação de acompanhamento inclui uma linha de base, essas diferenças explicam por que a revisão selecionada é melhor ou pior que a versão anterior.',
    metricsHint:
      'As métricas de qualidade, tempo de execução e tokens vêm do relatório da avaliação de acompanhamento vinculada quando disponível.',
  }),
  'pt-PT': buildEvolutionRuntimeBackfill({
    timeline: 'Ciclo de vida do caso',
    lineage: 'Resumo da linhagem das revisões',
    noDiff: 'Ainda não há patch candidato disponível.',
    caseStatusOpen: 'Entrada aberta, a aguardar candidato',
    timelineHint: 'Acompanhe o caso selecionado da entrada até à promoção final num só lugar.',
    followupEvalRun: 'Execução da avaliação de acompanhamento',
    openFollowupEval: 'Abrir avaliação de acompanhamento',
    linksSubtitle:
      'Use estas ligações para abrir a avaliação de acompanhamento, a revisão de skill ligada ou a avaliação de origem.',
    followupHint:
      'A avaliação de acompanhamento ligada decide se o candidato é aceite, rejeitado ou ainda está em execução.',
    followupTitle: 'Instantâneo da avaliação de acompanhamento',
    followupSubtitle:
      'Estas métricas vêm da avaliação de acompanhamento ligada e devem orientar a decisão de aceitar ou rejeitar antes de qualquer promoção manual.',
    followupEmpty: 'Ainda não há métricas estruturadas anexadas da avaliação de acompanhamento.',
    comparisonHint:
      'Quando uma avaliação de acompanhamento inclui uma linha de base, estas diferenças explicam porque a revisão selecionada é melhor ou pior do que a versão anterior.',
    metricsHint:
      'As métricas de qualidade, tempo de execução e tokens vêm do relatório da avaliação de acompanhamento ligada quando disponível.',
  }),
  'ro-RO': buildEvolutionRuntimeBackfill({
    timeline: 'Ciclul de viață al cazului',
    lineage: 'Rezumatul filiației reviziilor',
    noDiff: 'Nu există încă niciun patch candidat disponibil.',
    caseStatusOpen: 'Preluare deschisă, în așteptarea unui candidat',
    timelineHint:
      'Urmăriți cazul selectat de la preluare până la promovarea finală într-un singur loc.',
    followupEvalRun: 'Rulare evaluare de urmărire',
    openFollowupEval: 'Deschide evaluarea de urmărire',
    linksSubtitle:
      'Folosiți aceste legături pentru a deschide evaluarea de urmărire, revizia de skill legată sau evaluarea sursă.',
    followupHint:
      'Evaluarea de urmărire legată decide dacă candidatul este acceptat, respins sau încă rulează.',
    followupTitle: 'Instantaneu al evaluării de urmărire',
    followupSubtitle:
      'Aceste metrici provin din evaluarea de urmărire legată și ar trebui să ghideze decizia de acceptare sau respingere înaintea oricărei promovări manuale.',
    followupEmpty: 'Nu sunt atașate încă metrici structurate pentru evaluarea de urmărire.',
    comparisonHint:
      'Când o evaluare de urmărire include un reper, aceste diferențe explică de ce revizia selectată este mai bună sau mai slabă decât versiunea anterioară.',
    metricsHint:
      'Metricile de calitate, timp de execuție și tokeni provin din raportul evaluării de urmărire legate atunci când este disponibil.',
  }),
  'ru-RU': buildEvolutionRuntimeBackfill({
    timeline: 'Жизненный цикл кейса',
    lineage: 'Сводка родословной ревизий',
    noDiff: 'Патч-кандидат пока недоступен.',
    caseStatusOpen: 'Приём открыт, ожидание кандидата',
    timelineHint: 'Отслеживайте выбранный кейс от приёма до финального продвижения в одном месте.',
    followupEvalRun: 'Запуск последующей оценки',
    openFollowupEval: 'Открыть последующую оценку',
    linksSubtitle:
      'Используйте эти ссылки, чтобы открыть последующую оценку, связанную ревизию навыка или исходную оценку.',
    followupHint:
      'Связанная последующая оценка решает, принят кандидат, отклонён или всё ещё выполняется.',
    followupTitle: 'Снимок последующей оценки',
    followupSubtitle:
      'Эти метрики поступают из связанной последующей оценки и должны направлять решение о принятии или отклонении до любого ручного продвижения.',
    followupEmpty: 'Структурированные метрики последующей оценки пока не прикреплены.',
    comparisonHint:
      'Когда последующая оценка включает базовую линию, эти различия объясняют, почему выбранная ревизия лучше или хуже предыдущей версии.',
    metricsHint:
      'Метрики качества, времени выполнения и токенов поступают из связанного отчёта последующей оценки, когда он доступен.',
  }),
  'sk-SK': buildEvolutionRuntimeBackfill({
    timeline: 'Životný cyklus prípadu',
    lineage: 'Súhrn línie revízií',
    noDiff: 'Kandidátska záplata zatiaľ nie je k dispozícii.',
    caseStatusOpen: 'Príjem otvorený, čaká sa na kandidáta',
    timelineHint: 'Sledujte vybraný prípad od príjmu až po finálne povýšenie na jednom mieste.',
    followupEvalRun: 'Spustenie následného hodnotenia',
    openFollowupEval: 'Otvoriť následné hodnotenie',
    linksSubtitle:
      'Použite tieto odkazy na otvorenie následného hodnotenia, prepojenej revízie skillu alebo zdrojového hodnotenia.',
    followupHint:
      'Prepojené následné hodnotenie rozhoduje, či je kandidát prijatý, zamietnutý alebo ešte beží.',
    followupTitle: 'Snímka následného hodnotenia',
    followupSubtitle:
      'Tieto metriky pochádzajú z prepojeného následného hodnotenia a mali by riadiť rozhodnutie o prijatí alebo zamietnutí pred akýmkoľvek ručným povýšením.',
    followupEmpty: 'Zatiaľ nie sú pripojené žiadne štruktúrované metriky následného hodnotenia.',
    comparisonHint:
      'Keď následné hodnotenie obsahuje základnú líniu, tieto rozdiely vysvetľujú, prečo je vybraná revízia lepšia alebo horšia než predchádzajúca verzia.',
    metricsHint:
      'Metriky kvality, behu a tokenov pochádzajú z prepojenej správy následného hodnotenia, keď je dostupná.',
  }),
  'sv-SE': buildEvolutionRuntimeBackfill({
    timeline: 'Ärendets livscykel',
    lineage: 'Sammanfattning av revisionslinje',
    noDiff: 'Ingen kandidatpatch är tillgänglig ännu.',
    caseStatusOpen: 'Mottagning öppen, väntar på kandidat',
    timelineHint: 'Följ det valda ärendet från mottagning till slutlig befordran på ett ställe.',
    followupEvalRun: 'Körning för uppföljningsutvärdering',
    openFollowupEval: 'Öppna uppföljningsutvärdering',
    linksSubtitle:
      'Använd dessa länkar för att öppna uppföljningsutvärderingen, den länkade skill-revisionen eller källutvärderingen.',
    followupHint:
      'Den länkade uppföljningsutvärderingen avgör om kandidaten accepteras, avvisas eller fortfarande körs.',
    followupTitle: 'Ögonblicksbild av uppföljningsutvärdering',
    followupSubtitle:
      'Dessa mätvärden kommer från den länkade uppföljningsutvärderingen och bör styra beslutet att acceptera eller avvisa före någon manuell befordran.',
    followupEmpty: 'Inga strukturerade mätvärden från uppföljningsutvärderingen är bifogade ännu.',
    comparisonHint:
      'När en uppföljningsutvärdering innehåller en baslinje förklarar dessa skillnader varför den valda revisionen är bättre eller sämre än den tidigare versionen.',
    metricsHint:
      'Kvalitets-, körtids- och tokenmätvärden kommer från den länkade rapporten för uppföljningsutvärdering när den är tillgänglig.',
  }),
  'zh-CN': buildEvolutionRuntimeBackfill({
    timeline: '案例生命周期',
    lineage: '修订谱系摘要',
    noDiff: '尚无候选补丁。',
    caseStatusOpen: '已接收入队，等待候选',
    timelineHint: '在一个地方沿着所选案例从受理一路追踪到最终提升。',
    followupEvalRun: '后续评估运行',
    openFollowupEval: '打开后续评估',
    linksSubtitle: '通过这些链接，你可以打开后续评估、关联技能修订或源评估。',
    followupHint: '关联的后续评估会决定该候选是被接受、拒绝，还是仍在运行中。',
    followupTitle: '后续评估快照',
    followupSubtitle: '这些指标来自关联的后续评估，应在任何人工提升前驱动接受或拒绝决策。',
    followupEmpty: '当前还没有附带结构化的后续评估指标。',
    comparisonHint: '当后续评估带有基线时，这些差异会解释所选修订为何比上一版本更好或更差。',
    metricsHint: '质量、运行时和令牌指标会在可用时来自关联的后续评估报告。',
  }),
  'zh-TW': buildEvolutionRuntimeBackfill({
    timeline: '案例生命週期',
    lineage: '修訂譜系摘要',
    noDiff: '尚無候選補丁。',
    caseStatusOpen: '已接收入列，等待候選',
    timelineHint: '在同一處沿著所選案例從受理一路追蹤到最終提升。',
    followupEvalRun: '後續評估運行',
    openFollowupEval: '打開後續評估',
    linksSubtitle: '透過這些連結，你可以打開後續評估、關聯技能修訂或來源評估。',
    followupHint: '關聯的後續評估會決定該候選是被接受、拒絕，還是仍在執行中。',
    followupTitle: '後續評估快照',
    followupSubtitle: '這些指標來自關聯的後續評估，應在任何人工提升前驅動接受或拒絕決策。',
    followupEmpty: '目前還沒有附帶結構化的後續評估指標。',
    comparisonHint: '當後續評估帶有基線時，這些差異會說明所選修訂為何比上一版本更好或更差。',
    metricsHint: '品質、執行時與令牌指標會在可用時來自關聯的後續評估報告。',
  }),
}

const evolutionScorecardEvidenceHintBackfills: Record<LocaleKey, LocaleNode> = {
  'ca-ES': buildEvolutionScorecardEvidenceHintBackfill(
    "Encara no s'ha adjuntat cap resum estructurat d'evidències a aquesta revisió."
  ),
  'cs-CZ': buildEvolutionScorecardEvidenceHintBackfill(
    'K této revizi zatím není připojeno žádné strukturované shrnutí důkazů.'
  ),
  'da-DK': buildEvolutionScorecardEvidenceHintBackfill(
    'Der er endnu ikke knyttet nogen struktureret evidensopsummering til denne revision.'
  ),
  'de-DE': buildEvolutionScorecardEvidenceHintBackfill(
    'Dieser Revision ist noch keine strukturierte Evidenzzusammenfassung beigefügt.'
  ),
  'el-GR': buildEvolutionScorecardEvidenceHintBackfill(
    'Δεν έχει επισυναφθεί ακόμη δομημένη σύνοψη τεκμηρίων σε αυτή την αναθεώρηση.'
  ),
  'en-GB': buildEvolutionScorecardEvidenceHintBackfill(
    'No structured evidence summary is attached to this revision yet.'
  ),
  'en-US': buildEvolutionScorecardEvidenceHintBackfill(
    'No structured evidence summary is attached to this revision yet.'
  ),
  'es-ES': buildEvolutionScorecardEvidenceHintBackfill(
    'Todavía no hay un resumen estructurado de evidencias adjunto a esta revisión.'
  ),
  'fr-FR': buildEvolutionScorecardEvidenceHintBackfill(
    "Aucun résumé structuré des preuves n'est encore joint à cette révision."
  ),
  'ga-IE': buildEvolutionScorecardEvidenceHintBackfill(
    'Níl aon achoimre struchtúrtha fianaise ceangailte leis an athbhreithniú seo fós.'
  ),
  'hr-HR': buildEvolutionScorecardEvidenceHintBackfill(
    'Uz ovu reviziju još nije priložen strukturirani sažetak dokaza.'
  ),
  'hu-HU': buildEvolutionScorecardEvidenceHintBackfill(
    'Ehhez a revízióhoz még nincs csatolva strukturált bizonyítékösszefoglaló.'
  ),
  'it-IT': buildEvolutionScorecardEvidenceHintBackfill(
    'A questa revisione non è ancora allegato un riepilogo strutturato delle evidenze.'
  ),
  'ja-JP': buildEvolutionScorecardEvidenceHintBackfill(
    'このリビジョンには、構造化されたエビデンス要約がまだ添付されていません。'
  ),
  'ko-KR': buildEvolutionScorecardEvidenceHintBackfill(
    '이 리비전에 구조화된 근거 요약이 아직 첨부되지 않았습니다.'
  ),
  'ml-IN': buildEvolutionScorecardEvidenceHintBackfill(
    'ഈ റിവിഷനോട് ഇതുവരെ ഘടനാപരമായ തെളിവ് സംഗ്രഹം ചേർത്തിട്ടില്ല.'
  ),
  'nb-NO': buildEvolutionScorecardEvidenceHintBackfill(
    'Det er ennå ikke knyttet noe strukturert evidenssammendrag til denne revisjonen.'
  ),
  'nl-NL': buildEvolutionScorecardEvidenceHintBackfill(
    'Er is nog geen gestructureerde samenvatting van bewijsmateriaal aan deze revisie gekoppeld.'
  ),
  'pl-PL': buildEvolutionScorecardEvidenceHintBackfill(
    'Do tej rewizji nie dołączono jeszcze uporządkowanego podsumowania dowodów.'
  ),
  'pt-BR': buildEvolutionScorecardEvidenceHintBackfill(
    'Ainda não há um resumo estruturado de evidências anexado a esta revisão.'
  ),
  'pt-PT': buildEvolutionScorecardEvidenceHintBackfill(
    'Ainda não existe um resumo estruturado de evidências anexado a esta revisão.'
  ),
  'ro-RO': buildEvolutionScorecardEvidenceHintBackfill(
    'Încă nu este atașat niciun rezumat structurat al dovezilor acestei revizii.'
  ),
  'ru-RU': buildEvolutionScorecardEvidenceHintBackfill(
    'К этой ревизии пока не прикреплено структурированное резюме доказательств.'
  ),
  'sk-SK': buildEvolutionScorecardEvidenceHintBackfill(
    'K tejto revízii zatiaľ nie je pripojené žiadne štruktúrované zhrnutie dôkazov.'
  ),
  'sv-SE': buildEvolutionScorecardEvidenceHintBackfill(
    'Ingen strukturerad evidenssammanfattning är ännu bifogad till den här revisionen.'
  ),
  'zh-CN': buildEvolutionScorecardEvidenceHintBackfill('当前这次修订还没有附带结构化证据摘要。'),
  'zh-TW': buildEvolutionScorecardEvidenceHintBackfill('目前這次修訂還沒有附帶結構化證據摘要。'),
}

const evolutionScorecardActionBackfills: Record<LocaleKey, LocaleNode> = {
  'ca-ES': buildEvolutionScorecardActionBackfill({
    openComparison: 'Obre la comparació',
    openMetrics: 'Obre les mètriques',
    openEvidence: 'Obre les evidències',
  }),
  'cs-CZ': buildEvolutionScorecardActionBackfill({
    openComparison: 'Otevřít porovnání',
    openMetrics: 'Otevřít metriky',
    openEvidence: 'Otevřít důkazy',
  }),
  'da-DK': buildEvolutionScorecardActionBackfill({
    openComparison: 'Åbn sammenligning',
    openMetrics: 'Åbn målinger',
    openEvidence: 'Åbn evidens',
  }),
  'de-DE': buildEvolutionScorecardActionBackfill({
    openComparison: 'Vergleich öffnen',
    openMetrics: 'Metriken öffnen',
    openEvidence: 'Evidenz öffnen',
  }),
  'el-GR': buildEvolutionScorecardActionBackfill({
    openComparison: 'Άνοιγμα σύγκρισης',
    openMetrics: 'Άνοιγμα μετρήσεων',
    openEvidence: 'Άνοιγμα τεκμηρίων',
  }),
  'en-GB': buildEvolutionScorecardActionBackfill({
    openComparison: 'Open comparison',
    openMetrics: 'Open metrics',
    openEvidence: 'Open evidence',
  }),
  'en-US': buildEvolutionScorecardActionBackfill({
    openComparison: 'Open comparison',
    openMetrics: 'Open metrics',
    openEvidence: 'Open evidence',
  }),
  'es-ES': buildEvolutionScorecardActionBackfill({
    openComparison: 'Abrir comparación',
    openMetrics: 'Abrir métricas',
    openEvidence: 'Abrir evidencias',
  }),
  'fr-FR': buildEvolutionScorecardActionBackfill({
    openComparison: 'Ouvrir la comparaison',
    openMetrics: 'Ouvrir les métriques',
    openEvidence: 'Ouvrir les preuves',
  }),
  'ga-IE': buildEvolutionScorecardActionBackfill({
    openComparison: 'Oscail comparáid',
    openMetrics: 'Oscail méadrachtaí',
    openEvidence: 'Oscail fianaise',
  }),
  'hr-HR': buildEvolutionScorecardActionBackfill({
    openComparison: 'Otvori usporedbu',
    openMetrics: 'Otvori metrike',
    openEvidence: 'Otvori dokaze',
  }),
  'hu-HU': buildEvolutionScorecardActionBackfill({
    openComparison: 'Összehasonlítás megnyitása',
    openMetrics: 'Metrikák megnyitása',
    openEvidence: 'Bizonyítékok megnyitása',
  }),
  'it-IT': buildEvolutionScorecardActionBackfill({
    openComparison: 'Apri confronto',
    openMetrics: 'Apri metriche',
    openEvidence: 'Apri evidenze',
  }),
  'ja-JP': buildEvolutionScorecardActionBackfill({
    openComparison: '比較を開く',
    openMetrics: 'メトリクスを開く',
    openEvidence: 'エビデンスを開く',
  }),
  'ko-KR': buildEvolutionScorecardActionBackfill({
    openComparison: '비교 열기',
    openMetrics: '지표 열기',
    openEvidence: '근거 열기',
  }),
  'ml-IN': buildEvolutionScorecardActionBackfill({
    openComparison: 'താരതമ്യം തുറക്കുക',
    openMetrics: 'മെട്രിക്കുകൾ തുറക്കുക',
    openEvidence: 'തെളിവ് തുറക്കുക',
  }),
  'nb-NO': buildEvolutionScorecardActionBackfill({
    openComparison: 'Åpne sammenligning',
    openMetrics: 'Åpne målinger',
    openEvidence: 'Åpne evidens',
  }),
  'nl-NL': buildEvolutionScorecardActionBackfill({
    openComparison: 'Vergelijking openen',
    openMetrics: 'Metrieken openen',
    openEvidence: 'Bewijs openen',
  }),
  'pl-PL': buildEvolutionScorecardActionBackfill({
    openComparison: 'Otwórz porównanie',
    openMetrics: 'Otwórz metryki',
    openEvidence: 'Otwórz dowody',
  }),
  'pt-BR': buildEvolutionScorecardActionBackfill({
    openComparison: 'Abrir comparação',
    openMetrics: 'Abrir métricas',
    openEvidence: 'Abrir evidências',
  }),
  'pt-PT': buildEvolutionScorecardActionBackfill({
    openComparison: 'Abrir comparação',
    openMetrics: 'Abrir métricas',
    openEvidence: 'Abrir evidências',
  }),
  'ro-RO': buildEvolutionScorecardActionBackfill({
    openComparison: 'Deschide comparația',
    openMetrics: 'Deschide metricile',
    openEvidence: 'Deschide dovezile',
  }),
  'ru-RU': buildEvolutionScorecardActionBackfill({
    openComparison: 'Открыть сравнение',
    openMetrics: 'Открыть метрики',
    openEvidence: 'Открыть доказательства',
  }),
  'sk-SK': buildEvolutionScorecardActionBackfill({
    openComparison: 'Otvoriť porovnanie',
    openMetrics: 'Otvoriť metriky',
    openEvidence: 'Otvoriť dôkazy',
  }),
  'sv-SE': buildEvolutionScorecardActionBackfill({
    openComparison: 'Öppna jämförelse',
    openMetrics: 'Öppna mätvärden',
    openEvidence: 'Öppna bevis',
  }),
  'zh-CN': buildEvolutionScorecardActionBackfill({
    openComparison: '打开对比',
    openMetrics: '打开指标',
    openEvidence: '打开证据',
  }),
  'zh-TW': buildEvolutionScorecardActionBackfill({
    openComparison: '打開對比',
    openMetrics: '打開指標',
    openEvidence: '打開證據',
  }),
}

const evolutionUILocaleBackfills = Object.fromEntries(
  Array.from(
    new Set([
      ...Object.keys(baseEvolutionUILocaleBackfills),
      ...Object.keys(evolutionRuntimeLocaleBackfills),
      ...Object.keys(evolutionScorecardEvidenceHintBackfills),
      ...Object.keys(evolutionScorecardActionBackfills),
    ])
  ).map((locale) => [
    locale,
    mergeLocaleNodes(
      mergeLocaleNodes(
        (baseEvolutionUILocaleBackfills[locale as LocaleKey] ?? {}) as LocaleNode,
        (evolutionRuntimeLocaleBackfills[locale as LocaleKey] ?? {}) as LocaleNode
      ),
      mergeLocaleNodes(
        evolutionScorecardEvidenceHintBackfills[locale as LocaleKey] ?? {},
        evolutionScorecardActionBackfills[locale as LocaleKey] ?? {}
      )
    ),
  ])
) as Partial<Record<LocaleKey, object>>

export default evolutionUILocaleBackfills
