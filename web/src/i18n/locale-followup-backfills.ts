import type { LocaleKey } from './locale-catalog'

const githubAwesomeSkillsLabel = 'GitHub Awesome Skills'

function buildGitHubAwesomeSkillsBackfill(description: string, upstream: string) {
  return {
    marketplace: {
      sources: {
        githubAwesomeSkills: {
          label: githubAwesomeSkillsLabel,
          description,
        },
      },
      detail: {
        meta: {
          upstream,
        },
      },
    },
  }
}

function buildQuickEvalAutoCreatedBackfill(autoCreatedBy: string) {
  return {
    harness: {
      evalRun: {
        autoCreatedBy,
      },
    },
  }
}

function buildHarnessTermsBackfill(terms: {
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
}) {
  return {
    harness: {
      terms,
    },
  }
}

const localeFollowupBackfills: Partial<Record<LocaleKey, object>> = {
  'ca-ES': {
    common: { backToTop: 'Torna a dalt' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          "Les habilitats integrades es poden desactivar, però no es poden desinstal·lar.",
        builtinToolUninstallBlocked:
          "Les eines integrades es poden desactivar, però no es poden desinstal·lar.",
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Agrega fonts seleccionades de GitHub awesome skills.',
      'Origen original'
    ),
  },
  'cs-CZ': {
    common: { backToTop: 'Zpět nahoru' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'Vestavěné dovednosti lze zakázat, ale nelze je odinstalovat.',
        builtinToolUninstallBlocked:
          'Vestavěné nástroje lze zakázat, ale nelze je odinstalovat.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Agregovaný výběr kurátorovaných zdrojů GitHub awesome skills.',
      'Původní zdroj'
    ),
  },
  'da-DK': {
    common: { backToTop: 'Tilbage til toppen' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'Indbyggede færdigheder kan deaktiveres, men de kan ikke afinstalleres.',
        builtinToolUninstallBlocked:
          'Indbyggede værktøjer kan deaktiveres, men de kan ikke afinstalleres.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Samlet udvalg af kuraterede GitHub awesome skills-kilder.',
      'Oprindelig kilde'
    ),
  },
  'de-DE': {
    common: { backToTop: 'Zurück nach oben' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'Integrierte Skills können deaktiviert, aber nicht deinstalliert werden.',
        builtinToolUninstallBlocked:
          'Integrierte Tools können deaktiviert, aber nicht deinstalliert werden.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Zusammengefasste kuratierte GitHub-Awesome-Skills-Quellen.',
      'Ursprungsquelle'
    ),
  },
  'el-GR': {
    common: { backToTop: 'Επιστροφή στην κορυφή' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'Οι ενσωματωμένες δεξιότητες μπορούν να απενεργοποιηθούν, αλλά δεν μπορούν να απεγκατασταθούν.',
        builtinToolUninstallBlocked:
          'Τα ενσωματωμένα εργαλεία μπορούν να απενεργοποιηθούν, αλλά δεν μπορούν να απεγκατασταθούν.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Συγκεντρωμένες επιλεγμένες πηγές GitHub awesome skills.',
      'Αρχική πηγή'
    ),
  },
  'en-GB': {
    common: { backToTop: 'Back to top' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'Built-in skills can be disabled, but they cannot be uninstalled.',
        builtinToolUninstallBlocked:
          'Built-in tools can be disabled, but they cannot be uninstalled.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Aggregated curated GitHub awesome skills upstreams.',
      'Upstream'
    ),
  },
  'es-ES': {
    common: { backToTop: 'Volver arriba' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'Las habilidades integradas se pueden desactivar, pero no se pueden desinstalar.',
        builtinToolUninstallBlocked:
          'Las herramientas integradas se pueden desactivar, pero no se pueden desinstalar.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Agregado de fuentes seleccionadas de GitHub awesome skills.',
      'Origen'
    ),
  },
  'fr-FR': {
    common: { backToTop: 'Retour en haut' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'Les compétences intégrées peuvent être désactivées, mais elles ne peuvent pas être désinstallées.',
        builtinToolUninstallBlocked:
          'Les outils intégrés peuvent être désactivés, mais ils ne peuvent pas être désinstallés.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Agrégat de sources GitHub awesome skills sélectionnées.',
      "Source d'origine"
    ),
  },
  'ga-IE': {
    common: { backToTop: 'Ar ais go barr' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'Is féidir scileanna ionsuite a dhíchumasú, ach ní féidir iad a dhíshuiteáil.',
        builtinToolUninstallBlocked:
          'Is féidir uirlisí ionsuite a dhíchumasú, ach ní féidir iad a dhíshuiteáil.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Comhiomlán de fhoinsí roghnaithe GitHub awesome skills.',
      'Foinse bhunaidh'
    ),
  },
  'hr-HR': {
    common: { backToTop: 'Natrag na vrh' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'Ugrađene vještine mogu se onemogućiti, ali se ne mogu deinstalirati.',
        builtinToolUninstallBlocked:
          'Ugrađeni alati mogu se onemogućiti, ali se ne mogu deinstalirati.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Objedinjeni odabrani izvori GitHub awesome skills.',
      'Izvorni izvor'
    ),
  },
  'hu-HU': {
    common: { backToTop: 'Vissza a tetejére' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'A beépített készségek letilthatók, de nem távolíthatók el.',
        builtinToolUninstallBlocked:
          'A beépített eszközök letilthatók, de nem távolíthatók el.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Összesített, válogatott GitHub awesome skills források.',
      'Eredeti forrás'
    ),
  },
  'it-IT': {
    common: { backToTop: 'Torna in alto' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'Le skill integrate possono essere disabilitate, ma non possono essere disinstallate.',
        builtinToolUninstallBlocked:
          'Gli strumenti integrati possono essere disabilitati, ma non possono essere disinstallati.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Raccolta di fonti GitHub awesome skills selezionate.',
      'Fonte originale'
    ),
  },
  'ja-JP': {
    common: { backToTop: 'トップに戻る' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          '組み込みスキルは無効にできますが、アンインストールはできません。',
        builtinToolUninstallBlocked:
          '組み込みツールは無効にできますが、アンインストールはできません。',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      '厳選された GitHub awesome skills ソースを集約した一覧です。',
      '元のソース'
    ),
  },
  'ko-KR': {
    common: { backToTop: '맨 위로' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          '내장 스킬은 비활성화할 수 있지만 제거할 수는 없습니다.',
        builtinToolUninstallBlocked:
          '내장 도구는 비활성화할 수 있지만 제거할 수는 없습니다.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      '엄선된 GitHub awesome skills 소스를 모아 둔 집계입니다.',
      '원본 출처'
    ),
  },
  'ml-IN': {
    common: { backToTop: 'മുകളിൽേക്ക് മടങ്ങുക' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'ഉൾനിർമ്മിത സ്കില്ലുകൾ പ്രവർത്തനരഹിതമാക്കാം, എന്നാൽ അൺഇൻസ്റ്റാൾ ചെയ്യാനാവില്ല.',
        builtinToolUninstallBlocked:
          'ഉൾനിർമ്മിത ടൂളുകൾ പ്രവർത്തനരഹിതമാക്കാം, എന്നാൽ അൺഇൻസ്റ്റാൾ ചെയ്യാനാവില്ല.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'തിരഞ്ഞെടുത്ത GitHub awesome skills ഉറവിടങ്ങളെ ഏകോപിപ്പിച്ച സമാഹാരം.',
      'മൂല ഉറവിടം'
    ),
  },
  'nb-NO': {
    common: { backToTop: 'Tilbake til toppen' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'Innebygde ferdigheter kan deaktiveres, men de kan ikke avinstalleres.',
        builtinToolUninstallBlocked:
          'Innebygde verktøy kan deaktiveres, men de kan ikke avinstalleres.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Samlet oversikt over kuraterte GitHub awesome skills-kilder.',
      'Opprinnelig kilde'
    ),
  },
  'nl-NL': {
    common: { backToTop: 'Terug naar boven' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'Ingebouwde vaardigheden kunnen worden uitgeschakeld, maar niet worden verwijderd.',
        builtinToolUninstallBlocked:
          'Ingebouwde tools kunnen worden uitgeschakeld, maar niet worden verwijderd.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Verzamelde selectie van GitHub awesome skills-bronnen.',
      'Oorspronkelijke bron'
    ),
  },
  'pl-PL': {
    common: { backToTop: 'Wróć na górę' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'Wbudowane umiejętności można wyłączyć, ale nie można ich odinstalować.',
        builtinToolUninstallBlocked:
          'Wbudowane narzędzia można wyłączyć, ale nie można ich odinstalować.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Zbiorczy zestaw wyselekcjonowanych źródeł GitHub awesome skills.',
      'Źródło pierwotne'
    ),
  },
  'pt-BR': {
    common: { backToTop: 'Voltar ao topo' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'As habilidades integradas podem ser desativadas, mas não podem ser desinstaladas.',
        builtinToolUninstallBlocked:
          'As ferramentas integradas podem ser desativadas, mas não podem ser desinstaladas.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Agregado de fontes selecionadas de GitHub awesome skills.',
      'Fonte de origem'
    ),
  },
  'pt-PT': {
    common: { backToTop: 'Voltar ao topo' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'As habilidades integradas podem ser desativadas, mas não podem ser desinstaladas.',
        builtinToolUninstallBlocked:
          'As ferramentas integradas podem ser desativadas, mas não podem ser desinstaladas.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Agregado de fontes selecionadas de GitHub awesome skills.',
      'Fonte de origem'
    ),
  },
  'ro-RO': {
    common: { backToTop: 'Înapoi sus' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'Abilitățile integrate pot fi dezactivate, dar nu pot fi dezinstalate.',
        builtinToolUninstallBlocked:
          'Instrumentele integrate pot fi dezactivate, dar nu pot fi dezinstalate.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Agregare de surse GitHub awesome skills selectate.',
      'Sursa de origine'
    ),
  },
  'ru-RU': {
    common: { backToTop: 'Наверх' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'Встроенные навыки можно отключить, но нельзя удалить.',
        builtinToolUninstallBlocked:
          'Встроенные инструменты можно отключить, но нельзя удалить.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Собранные отобранные источники GitHub awesome skills.',
      'Исходный источник'
    ),
  },
  'sk-SK': {
    common: { backToTop: 'Späť hore' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'Vstavané zručnosti možno zakázať, ale nemožno ich odinštalovať.',
        builtinToolUninstallBlocked:
          'Vstavané nástroje možno zakázať, ale nemožno ich odinštalovať.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Agregovaný výber kurátorovaných zdrojov GitHub awesome skills.',
      'Pôvodný zdroj'
    ),
  },
  'sv-SE': {
    common: { backToTop: 'Till toppen' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'Inbyggda färdigheter kan inaktiveras, men de kan inte avinstalleras.',
        builtinToolUninstallBlocked:
          'Inbyggda verktyg kan inaktiveras, men de kan inte avinstalleras.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Samlad översikt över utvalda GitHub awesome skills-källor.',
      'Ursprungskälla'
    ),
  },
  'zh-TW': {
    common: { backToTop: '回到頂部' },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      '聚合多個人工篩選的 GitHub awesome skills 上游來源。',
      '上游來源'
    ),
  },
}

const quickEvalEvalRunBackfills: Partial<Record<LocaleKey, object>> = {
  'ca-ES': buildQuickEvalAutoCreatedBackfill('Creat automàticament per {name}'),
  'cs-CZ': buildQuickEvalAutoCreatedBackfill('Automaticky vytvořeno pomocí {name}'),
  'da-DK': buildQuickEvalAutoCreatedBackfill('Automatisk oprettet af {name}'),
  'de-DE': buildQuickEvalAutoCreatedBackfill('Automatisch von {name} erstellt'),
  'el-GR': buildQuickEvalAutoCreatedBackfill('Δημιουργήθηκε αυτόματα από το {name}'),
  'en-GB': buildQuickEvalAutoCreatedBackfill('Auto-created by {name}'),
  'en-US': buildQuickEvalAutoCreatedBackfill('Auto-created by {name}'),
  'es-ES': buildQuickEvalAutoCreatedBackfill('Creado automáticamente por {name}'),
  'fr-FR': buildQuickEvalAutoCreatedBackfill('Créé automatiquement par {name}'),
  'ga-IE': buildQuickEvalAutoCreatedBackfill('Cruthaíodh go huathoibríoch le {name}'),
  'hr-HR': buildQuickEvalAutoCreatedBackfill('Automatski stvorio {name}'),
  'hu-HU': buildQuickEvalAutoCreatedBackfill('Automatikusan létrehozta: {name}'),
  'it-IT': buildQuickEvalAutoCreatedBackfill('Creato automaticamente da {name}'),
  'ja-JP': buildQuickEvalAutoCreatedBackfill('{name} により自動作成'),
  'ko-KR': buildQuickEvalAutoCreatedBackfill('{name}이 자동 생성'),
  'ml-IN': buildQuickEvalAutoCreatedBackfill('{name} സ്വയമേവ സൃഷ്ടിച്ചത്'),
  'nb-NO': buildQuickEvalAutoCreatedBackfill('Automatisk opprettet av {name}'),
  'nl-NL': buildQuickEvalAutoCreatedBackfill('Automatisch aangemaakt door {name}'),
  'pl-PL': buildQuickEvalAutoCreatedBackfill('Utworzono automatycznie przez {name}'),
  'pt-BR': buildQuickEvalAutoCreatedBackfill('Criado automaticamente pelo {name}'),
  'pt-PT': buildQuickEvalAutoCreatedBackfill('Criado automaticamente pelo {name}'),
  'ro-RO': buildQuickEvalAutoCreatedBackfill('Creat automat de {name}'),
  'ru-RU': buildQuickEvalAutoCreatedBackfill('Автоматически создано через {name}'),
  'sk-SK': buildQuickEvalAutoCreatedBackfill('Automaticky vytvorené pomocou {name}'),
  'sv-SE': buildQuickEvalAutoCreatedBackfill('Skapad automatiskt av {name}'),
  'zh-CN': buildQuickEvalAutoCreatedBackfill('由{name}自动创建'),
  'zh-TW': buildQuickEvalAutoCreatedBackfill('由{name}自動建立'),
}

const harnessTermBackfills: Partial<Record<LocaleKey, object>> = {
  'ca-ES': buildHarnessTermsBackfill({
    preset: 'preajust',
    dataset: 'conjunt de dades',
    version: 'versió',
    spec: 'especificació',
    run: 'execució',
    baseline: 'línia base',
    group: 'grup',
    scorecard: 'targeta de puntuació',
    agentTask: "tasca d'agent",
    prepare: 'prepara',
  }),
  'cs-CZ': buildHarnessTermsBackfill({
    preset: 'předvolba',
    dataset: 'datová sada',
    version: 'verze',
    spec: 'specifikace',
    run: 'běh',
    baseline: 'základní linie',
    group: 'skupina',
    scorecard: 'hodnoticí karta',
    agentTask: 'úloha agenta',
    prepare: 'připravit',
  }),
  'da-DK': buildHarnessTermsBackfill({
    preset: 'forvalg',
    dataset: 'datasæt',
    version: 'version',
    spec: 'specifikation',
    run: 'kørsel',
    baseline: 'basislinje',
    group: 'gruppe',
    scorecard: 'scorekort',
    agentTask: 'agentopgave',
    prepare: 'forbered',
  }),
  'de-DE': buildHarnessTermsBackfill({
    preset: 'Vorgabe',
    dataset: 'Datensatz',
    version: 'Version',
    spec: 'Spezifikation',
    run: 'Lauf',
    baseline: 'Basislinie',
    group: 'Gruppe',
    scorecard: 'Bewertungskarte',
    agentTask: 'Agentenaufgabe',
    prepare: 'vorbereiten',
  }),
  'el-GR': buildHarnessTermsBackfill({
    preset: 'προρύθμιση',
    dataset: 'σύνολο δεδομένων',
    version: 'έκδοση',
    spec: 'προδιαγραφή',
    run: 'εκτέλεση',
    baseline: 'γραμμή βάσης',
    group: 'ομάδα',
    scorecard: 'κάρτα βαθμολόγησης',
    agentTask: 'εργασία πράκτορα',
    prepare: 'προετοιμασία',
  }),
  'en-GB': buildHarnessTermsBackfill({
    preset: 'preset',
    dataset: 'dataset',
    version: 'version',
    spec: 'spec',
    run: 'run',
    baseline: 'baseline',
    group: 'group',
    scorecard: 'scorecard',
    agentTask: 'Agent Task',
    prepare: 'prepare',
  }),
  'en-US': buildHarnessTermsBackfill({
    preset: 'preset',
    dataset: 'dataset',
    version: 'version',
    spec: 'spec',
    run: 'run',
    baseline: 'baseline',
    group: 'group',
    scorecard: 'scorecard',
    agentTask: 'Agent Task',
    prepare: 'prepare',
  }),
  'es-ES': buildHarnessTermsBackfill({
    preset: 'preajuste',
    dataset: 'conjunto de datos',
    version: 'versión',
    spec: 'especificación',
    run: 'ejecución',
    baseline: 'línea base',
    group: 'grupo',
    scorecard: 'tarjeta de puntuación',
    agentTask: 'tarea del agente',
    prepare: 'preparar',
  }),
  'fr-FR': buildHarnessTermsBackfill({
    preset: 'préréglage',
    dataset: 'jeu de données',
    version: 'version',
    spec: 'spécification',
    run: 'exécution',
    baseline: 'référence',
    group: 'groupe',
    scorecard: "fiche d'évaluation",
    agentTask: 'tâche agent',
    prepare: 'préparer',
  }),
  'ga-IE': buildHarnessTermsBackfill({
    preset: 'réamhshocrú',
    dataset: 'tacar sonraí',
    version: 'leagan',
    spec: 'sonraíocht',
    run: 'rith',
    baseline: 'bunlíne',
    group: 'grúpa',
    scorecard: 'cárta scórála',
    agentTask: 'tasc gníomhaire',
    prepare: 'ullmhaigh',
  }),
  'hr-HR': buildHarnessTermsBackfill({
    preset: 'predložak',
    dataset: 'skup podataka',
    version: 'verzija',
    spec: 'specifikacija',
    run: 'izvršavanje',
    baseline: 'osnovna linija',
    group: 'grupa',
    scorecard: 'kartica bodovanja',
    agentTask: 'zadatak agenta',
    prepare: 'pripremi',
  }),
  'hu-HU': buildHarnessTermsBackfill({
    preset: 'előbeállítás',
    dataset: 'adatkészlet',
    version: 'verzió',
    spec: 'specifikáció',
    run: 'futás',
    baseline: 'alapvonal',
    group: 'csoport',
    scorecard: 'pontozólap',
    agentTask: 'ügynökfeladat',
    prepare: 'előkészítés',
  }),
  'it-IT': buildHarnessTermsBackfill({
    preset: 'preimpostazione',
    dataset: 'insieme di dati',
    version: 'versione',
    spec: 'specifica',
    run: 'esecuzione',
    baseline: 'linea base',
    group: 'gruppo',
    scorecard: 'scheda punteggi',
    agentTask: 'attività agente',
    prepare: 'prepara',
  }),
  'ja-JP': buildHarnessTermsBackfill({
    preset: 'プリセット',
    dataset: 'データセット',
    version: 'バージョン',
    spec: '仕様',
    run: '実行',
    baseline: 'ベースライン',
    group: 'グループ',
    scorecard: 'スコアカード',
    agentTask: 'エージェントタスク',
    prepare: '準備',
  }),
  'ko-KR': buildHarnessTermsBackfill({
    preset: '프리셋',
    dataset: '데이터셋',
    version: '버전',
    spec: '사양',
    run: '실행',
    baseline: '기준선',
    group: '그룹',
    scorecard: '점수표',
    agentTask: '에이전트 작업',
    prepare: '준비',
  }),
  'ml-IN': buildHarnessTermsBackfill({
    preset: 'പ്രീസെറ്റ്',
    dataset: 'ഡാറ്റാസെറ്റ്',
    version: 'പതിപ്പ്',
    spec: 'സ്പെസിഫിക്കേഷൻ',
    run: 'പ്രവർത്തനം',
    baseline: 'അടിസ്ഥാനരേഖ',
    group: 'ഗ്രൂപ്പ്',
    scorecard: 'സ്കോർകാർഡ്',
    agentTask: 'ഏജന്റ് ടാസ്‌ക്',
    prepare: 'തയ്യാറാക്കുക',
  }),
  'nb-NO': buildHarnessTermsBackfill({
    preset: 'forvalg',
    dataset: 'datasett',
    version: 'versjon',
    spec: 'spesifikasjon',
    run: 'kjøring',
    baseline: 'grunnlinje',
    group: 'gruppe',
    scorecard: 'poengkort',
    agentTask: 'agentoppgave',
    prepare: 'forbered',
  }),
  'nl-NL': buildHarnessTermsBackfill({
    preset: 'voorinstelling',
    dataset: 'gegevensset',
    version: 'versie',
    spec: 'specificatie',
    run: 'uitvoering',
    baseline: 'basislijn',
    group: 'groep',
    scorecard: 'scorekaart',
    agentTask: 'agenttaak',
    prepare: 'voorbereiden',
  }),
  'pl-PL': buildHarnessTermsBackfill({
    preset: 'ustawienie wstępne',
    dataset: 'zbiór danych',
    version: 'wersja',
    spec: 'specyfikacja',
    run: 'uruchomienie',
    baseline: 'linia bazowa',
    group: 'grupa',
    scorecard: 'karta wyników',
    agentTask: 'zadanie agenta',
    prepare: 'przygotować',
  }),
  'pt-BR': buildHarnessTermsBackfill({
    preset: 'predefinição',
    dataset: 'conjunto de dados',
    version: 'versão',
    spec: 'especificação',
    run: 'execução',
    baseline: 'linha de base',
    group: 'grupo',
    scorecard: 'cartão de pontuação',
    agentTask: 'tarefa do agente',
    prepare: 'preparar',
  }),
  'pt-PT': buildHarnessTermsBackfill({
    preset: 'predefinição',
    dataset: 'conjunto de dados',
    version: 'versão',
    spec: 'especificação',
    run: 'execução',
    baseline: 'linha de base',
    group: 'grupo',
    scorecard: 'cartão de pontuação',
    agentTask: 'tarefa do agente',
    prepare: 'preparar',
  }),
  'ro-RO': buildHarnessTermsBackfill({
    preset: 'presetare',
    dataset: 'set de date',
    version: 'versiune',
    spec: 'specificație',
    run: 'execuție',
    baseline: 'linie de bază',
    group: 'grup',
    scorecard: 'fișă de scor',
    agentTask: 'sarcină agent',
    prepare: 'pregătește',
  }),
  'ru-RU': buildHarnessTermsBackfill({
    preset: 'предустановка',
    dataset: 'набор данных',
    version: 'версия',
    spec: 'спецификация',
    run: 'запуск',
    baseline: 'базовая линия',
    group: 'группа',
    scorecard: 'карта оценок',
    agentTask: 'задача агента',
    prepare: 'подготовить',
  }),
  'sk-SK': buildHarnessTermsBackfill({
    preset: 'predvoľba',
    dataset: 'dátová sada',
    version: 'verzia',
    spec: 'špecifikácia',
    run: 'beh',
    baseline: 'základná línia',
    group: 'skupina',
    scorecard: 'hodnotiaca karta',
    agentTask: 'úloha agenta',
    prepare: 'pripraviť',
  }),
  'sv-SE': buildHarnessTermsBackfill({
    preset: 'förinställning',
    dataset: 'datauppsättning',
    version: 'version',
    spec: 'specifikation',
    run: 'körning',
    baseline: 'baslinje',
    group: 'grupp',
    scorecard: 'poängkort',
    agentTask: 'agentuppgift',
    prepare: 'förbered',
  }),
  'zh-CN': buildHarnessTermsBackfill({
    preset: '预设',
    dataset: '数据集',
    version: '版本',
    spec: '规格',
    run: '运行',
    baseline: '基线',
    group: '分组',
    scorecard: '评分卡',
    agentTask: '代理任务',
    prepare: '准备',
  }),
  'zh-TW': buildHarnessTermsBackfill({
    preset: '預設',
    dataset: '資料集',
    version: '版本',
    spec: '規格',
    run: '執行',
    baseline: '基線',
    group: '群組',
    scorecard: '評分卡',
    agentTask: '代理任務',
    prepare: '準備',
  }),
}

const mergedLocaleFollowupBackfills = Object.fromEntries(
  Array.from(
    new Set([
      ...Object.keys(localeFollowupBackfills),
      ...Object.keys(quickEvalEvalRunBackfills),
      ...Object.keys(harnessTermBackfills),
    ])
  ).map((locale) => [
    locale,
    {
      ...((localeFollowupBackfills[locale as LocaleKey] as Record<string, object>) || {}),
      ...((quickEvalEvalRunBackfills[locale as LocaleKey] as Record<string, object>) || {}),
      ...((harnessTermBackfills[locale as LocaleKey] as Record<string, object>) || {}),
    },
  ])
) as Partial<Record<LocaleKey, object>>

export default mergedLocaleFollowupBackfills
