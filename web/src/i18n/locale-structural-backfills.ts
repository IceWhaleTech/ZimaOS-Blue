import type { LocaleKey } from './locale-catalog'

const localeStructuralBackfills = {
  'ca-ES': {
    common: {
      update: 'Actualitzar',
    },
    plugins: {
      noToolsTitle: "No s'han trobat eines",
    },
    speech: {
      convertTask: {
        sources: 'Fonts',
      },
    },
    media: {
      modelDownload: {
        auto: {
          fileName: "S'estan processant els fitxers",
        },
        completed: 'Descarregat',
        pending: 'Pendent',
        status: {
          downloading: "S'estan baixant les metadades",
          error: 'Ha fallat la descàrrega',
          pending: "S'està preparant la descàrrega",
          ready: 'Model llest',
        },
      },
    },
  },
  'cs-CZ': {
    common: {
      update: 'Aktualizovat',
    },
    plugins: {
      noToolsTitle: 'Nebyly nalezeny žádné nástroje',
    },
    speech: {
      convertTask: {
        sources: 'Zdroje',
      },
    },
    media: {
      modelDownload: {
        auto: {
          fileName: 'Zpracovávají se soubory',
        },
        completed: 'Staženo',
        pending: 'Čekající',
        status: {
          downloading: 'Stahují se metadata',
          error: 'Stahování se nezdařilo',
          pending: 'Připravuje se stahování',
          ready: 'Model je připraven',
        },
      },
    },
  },
  'da-DK': {
    common: {
      update: 'Opdater',
    },
    plugins: {
      noToolsTitle: 'Ingen værktøjer fundet',
    },
    speech: {
      convertTask: {
        sources: 'Kilder',
      },
    },
    media: {
      modelDownload: {
        auto: {
          fileName: 'Behandler filer',
        },
        completed: 'Downloadet',
        pending: 'Afventer',
        status: {
          downloading: 'Downloader metadata',
          error: 'Download mislykkedes',
          pending: 'Forbereder download',
          ready: 'Modellen er klar',
        },
      },
    },
  },
  'de-DE': {
    common: {
      update: 'Aktualisieren',
    },
    plugins: {
      noToolsTitle: 'Keine Tools gefunden',
    },
    speech: {
      convertTask: {
        sources: 'Quellen',
      },
    },
    media: {
      modelDownload: {
        auto: {
          fileName: 'Dateien werden verarbeitet',
        },
        completed: 'Heruntergeladen',
        pending: 'Ausstehend',
        status: {
          downloading: 'Metadaten werden heruntergeladen',
          error: 'Download fehlgeschlagen',
          pending: 'Download wird vorbereitet',
          ready: 'Modell bereit',
        },
      },
    },
  },
  'el-GR': {
    common: {
      update: 'Ενημέρωση',
    },
    plugins: {
      noToolsTitle: 'Δεν βρέθηκαν εργαλεία',
    },
    speech: {
      convertTask: {
        sources: 'Πηγές',
      },
    },
    media: {
      modelDownload: {
        auto: {
          fileName: 'Γίνεται επεξεργασία αρχείων',
        },
        completed: 'Λήφθηκε',
        pending: 'Σε αναμονή',
        status: {
          downloading: 'Λήψη μεταδεδομένων',
          error: 'Η λήψη απέτυχε',
          pending: 'Προετοιμασία λήψης',
          ready: 'Το μοντέλο είναι έτοιμο',
        },
      },
    },
  },
  'en-GB': {
    common: {
      update: 'Update',
    },
    plugins: {
      noToolsTitle: 'No tools found',
    },
    speech: {
      convertTask: {
        sources: 'Sources',
      },
    },
    media: {
      modelDownload: {
        auto: {
          fileName: 'Processing files',
        },
        completed: 'Downloaded',
        pending: 'Pending',
        status: {
          downloading: 'Downloading metadata',
          error: 'Download failed',
          pending: 'Preparing download',
          ready: 'Model ready',
        },
      },
    },
  },
  'en-US': {
    common: {
      update: 'Update',
    },
    plugins: {
      noToolsTitle: 'No tools found',
    },
    speech: {
      convertTask: {
        sources: 'Sources',
      },
    },
    media: {
      modelDownload: {
        auto: {
          fileName: 'Processing files',
        },
        completed: 'Downloaded',
        pending: 'Pending',
        status: {
          downloading: 'Downloading metadata',
          error: 'Download failed',
          pending: 'Preparing download',
          ready: 'Model ready',
        },
      },
    },
  },
  'es-ES': {
    common: {
      update: 'Actualizar',
    },
    plugins: {
      noToolsTitle: 'No se encontraron herramientas',
    },
    speech: {
      convertTask: {
        sources: 'Fuentes',
      },
    },
    media: {
      modelDownload: {
        auto: {
          fileName: 'Procesando archivos',
        },
        completed: 'Descargado',
        pending: 'Pendiente',
        status: {
          downloading: 'Descargando metadatos',
          error: 'Error al descargar',
          pending: 'Preparando descarga',
          ready: 'Modelo listo',
        },
      },
    },
  },
  'fr-FR': {
    common: {
      update: 'Mettre à jour',
    },
    plugins: {
      noToolsTitle: 'Aucun outil trouvé',
    },
    speech: {
      convertTask: {
        sources: 'Sources',
      },
    },
    media: {
      modelDownload: {
        auto: {
          fileName: 'Traitement des fichiers',
        },
        completed: 'Téléchargé',
        pending: 'En attente',
        status: {
          downloading: 'Téléchargement des métadonnées',
          error: 'Échec du téléchargement',
          pending: 'Préparation du téléchargement',
          ready: 'Modèle prêt',
        },
      },
    },
  },
  'ga-IE': {
    common: {
      update: 'Nuashonrú',
    },
    plugins: {
      noToolsTitle: 'Níor aimsíodh uirlisí',
    },
    speech: {
      convertTask: {
        sources: 'Foinsí',
      },
    },
    media: {
      modelDownload: {
        auto: {
          fileName: 'Comhaid á bpróiseáil',
        },
        completed: 'Íoslódáilte',
        pending: 'Ar feitheamh',
        status: {
          downloading: 'Meiteashonraí á n-íoslódáil',
          error: 'Theip ar an íoslódáil',
          pending: 'Íoslódáil á hullmhú',
          ready: 'Tá an tsamhail réidh',
        },
      },
    },
  },
  'hr-HR': {
    common: {
      update: 'Ažuriraj',
    },
    plugins: {
      noToolsTitle: 'Nisu pronađeni alati',
    },
    speech: {
      convertTask: {
        sources: 'Izvori',
      },
    },
    media: {
      modelDownload: {
        auto: {
          fileName: 'Obrada datoteka',
        },
        completed: 'Preuzeto',
        pending: 'Na čekanju',
        status: {
          downloading: 'Preuzimanje metapodataka',
          error: 'Preuzimanje nije uspjelo',
          pending: 'Priprema preuzimanja',
          ready: 'Model je spreman',
        },
      },
    },
  },
  'hu-HU': {
    common: {
      update: 'Frissítés',
    },
    plugins: {
      noToolsTitle: 'Nem találhatók eszközök',
    },
    speech: {
      convertTask: {
        sources: 'Források',
      },
    },
    media: {
      modelDownload: {
        auto: {
          fileName: 'Fájlok feldolgozása',
        },
        completed: 'Letöltve',
        pending: 'Függőben',
        status: {
          downloading: 'Metaadatok letöltése',
          error: 'A letöltés sikertelen',
          pending: 'Letöltés előkészítése',
          ready: 'A modell készen áll',
        },
      },
    },
  },
  'it-IT': {
    common: {
      update: 'Aggiorna',
    },
    plugins: {
      noToolsTitle: 'Nessuno strumento trovato',
    },
    speech: {
      convertTask: {
        sources: 'Fonti',
      },
    },
    media: {
      modelDownload: {
        auto: {
          fileName: 'Elaborazione dei file',
        },
        completed: 'Scaricato',
        pending: 'In attesa',
        status: {
          downloading: 'Download dei metadati',
          error: 'Download fallito',
          pending: 'Preparazione del download',
          ready: 'Modello pronto',
        },
      },
    },
  },
  'ja-JP': {
    common: {
      update: '更新',
    },
    plugins: {
      noToolsTitle: 'ツールが見つかりません',
    },
    speech: {
      convertTask: {
        sources: 'ソース',
      },
    },
    media: {
      modelDownload: {
        auto: {
          fileName: 'ファイルを処理中',
        },
        completed: 'ダウンロード済み',
        pending: '待機中',
        status: {
          downloading: 'メタデータをダウンロード中',
          error: 'ダウンロードに失敗しました',
          pending: 'ダウンロードを準備中',
          ready: 'モデル準備完了',
        },
      },
    },
  },
  'ko-KR': {
    common: {
      update: '업데이트',
    },
    plugins: {
      noToolsTitle: '도구를 찾을 수 없습니다',
    },
    speech: {
      convertTask: {
        sources: '출처',
      },
    },
    media: {
      modelDownload: {
        auto: {
          fileName: '파일 처리 중',
        },
        completed: '다운로드됨',
        pending: '대기 중',
        status: {
          downloading: '메타데이터 다운로드 중',
          error: '다운로드에 실패했습니다',
          pending: '다운로드 준비 중',
          ready: '모델 준비 완료',
        },
      },
    },
  },
  'ml-IN': {
    common: {
      update: 'അപ്ഡേറ്റ്',
    },
    plugins: {
      noToolsTitle: 'ഉപകരണങ്ങളൊന്നും കണ്ടെത്തിയില്ല',
    },
    speech: {
      convertTask: {
        sources: 'ഉറവിടങ്ങൾ',
      },
    },
    media: {
      modelDownload: {
        auto: {
          fileName: 'ഫയലുകൾ പ്രോസസ്സ് ചെയ്യുന്നു',
        },
        completed: 'ഡൗൺലോഡ് ചെയ്തു',
        pending: 'കാത്തിരിക്കുന്നു',
        status: {
          downloading: 'മെറ്റാഡേറ്റ ഡൗൺലോഡ് ചെയ്യുന്നു',
          error: 'ഡൗൺലോഡ് പരാജയപ്പെട്ടു',
          pending: 'ഡൗൺലോഡ് തയ്യാറാക്കുന്നു',
          ready: 'മോഡൽ തയ്യാറാണ്',
        },
      },
    },
  },
  'nb-NO': {
    common: {
      update: 'Oppdater',
    },
    plugins: {
      noToolsTitle: 'Ingen verktøy funnet',
    },
    speech: {
      convertTask: {
        sources: 'Kilder',
      },
    },
    media: {
      modelDownload: {
        auto: {
          fileName: 'Behandler filer',
        },
        completed: 'Lastet ned',
        pending: 'Venter',
        status: {
          downloading: 'Laster ned metadata',
          error: 'Nedlasting mislyktes',
          pending: 'Forbereder nedlasting',
          ready: 'Modellen er klar',
        },
      },
    },
  },
  'nl-NL': {
    common: {
      update: 'Bijwerken',
    },
    plugins: {
      noToolsTitle: 'Geen tools gevonden',
    },
    speech: {
      convertTask: {
        sources: 'Bronnen',
      },
    },
    media: {
      modelDownload: {
        auto: {
          fileName: 'Bestanden verwerken',
        },
        completed: 'Gedownload',
        pending: 'In behandeling',
        status: {
          downloading: 'Metadata downloaden',
          error: 'Download mislukt',
          pending: 'Download voorbereiden',
          ready: 'Model gereed',
        },
      },
    },
  },
  'pl-PL': {
    common: {
      update: 'Aktualizuj',
    },
    plugins: {
      noToolsTitle: 'Nie znaleziono narzędzi',
    },
    speech: {
      convertTask: {
        sources: 'Źródła',
      },
    },
    media: {
      modelDownload: {
        auto: {
          fileName: 'Przetwarzanie plików',
        },
        completed: 'Pobrano',
        pending: 'Oczekujące',
        status: {
          downloading: 'Pobieranie metadanych',
          error: 'Pobieranie nie powiodło się',
          pending: 'Przygotowywanie pobierania',
          ready: 'Model gotowy',
        },
      },
    },
  },
  'pt-BR': {
    common: {
      update: 'Atualizar',
    },
    plugins: {
      noToolsTitle: 'Nenhuma ferramenta encontrada',
    },
    speech: {
      convertTask: {
        sources: 'Fontes',
      },
    },
    media: {
      modelDownload: {
        auto: {
          fileName: 'Processando arquivos',
        },
        completed: 'Baixado',
        pending: 'Pendente',
        status: {
          downloading: 'Baixando metadados',
          error: 'Falha no download',
          pending: 'Preparando download',
          ready: 'Modelo pronto',
        },
      },
    },
  },
  'pt-PT': {
    common: {
      update: 'Atualizar',
    },
    plugins: {
      noToolsTitle: 'Nenhuma ferramenta encontrada',
    },
    speech: {
      convertTask: {
        sources: 'Fontes',
      },
    },
    media: {
      modelDownload: {
        auto: {
          fileName: 'A processar ficheiros',
        },
        completed: 'Descarregado',
        pending: 'Pendente',
        status: {
          downloading: 'A transferir metadados',
          error: 'Falha no download',
          pending: 'A preparar download',
          ready: 'Modelo pronto',
        },
      },
    },
  },
  'ro-RO': {
    common: {
      update: 'Actualizare',
    },
    plugins: {
      noToolsTitle: 'Nu au fost găsite instrumente',
    },
    speech: {
      convertTask: {
        sources: 'Surse',
      },
    },
    media: {
      modelDownload: {
        auto: {
          fileName: 'Se procesează fișierele',
        },
        completed: 'Descărcat',
        pending: 'În așteptare',
        status: {
          downloading: 'Se descarcă metadatele',
          error: 'Descărcarea a eșuat',
          pending: 'Se pregătește descărcarea',
          ready: 'Modelul este pregătit',
        },
      },
    },
  },
  'ru-RU': {
    common: {
      update: 'Обновить',
    },
    plugins: {
      noToolsTitle: 'Инструменты не найдены',
    },
    speech: {
      convertTask: {
        sources: 'Источники',
      },
    },
    media: {
      modelDownload: {
        auto: {
          fileName: 'Обработка файлов',
        },
        completed: 'Скачано',
        pending: 'Ожидание',
        status: {
          downloading: 'Загрузка метаданных',
          error: 'Ошибка загрузки',
          pending: 'Подготовка загрузки',
          ready: 'Модель готова',
        },
      },
    },
  },
  'sk-SK': {
    common: {
      update: 'Aktualizovať',
    },
    plugins: {
      noToolsTitle: 'Nenašli sa žiadne nástroje',
    },
    speech: {
      convertTask: {
        sources: 'Zdroje',
      },
    },
    media: {
      modelDownload: {
        auto: {
          fileName: 'Spracúvajú sa súbory',
        },
        completed: 'Stiahnuté',
        pending: 'Čakajúce',
        status: {
          downloading: 'Sťahujú sa metadáta',
          error: 'Sťahovanie zlyhalo',
          pending: 'Pripravuje sa sťahovanie',
          ready: 'Model je pripravený',
        },
      },
    },
  },
  'sv-SE': {
    common: {
      update: 'Uppdatera',
    },
    plugins: {
      noToolsTitle: 'Inga verktyg hittades',
    },
    speech: {
      convertTask: {
        sources: 'Källor',
      },
    },
    media: {
      modelDownload: {
        auto: {
          fileName: 'Bearbetar filer',
        },
        completed: 'Nedladdad',
        pending: 'Väntande',
        status: {
          downloading: 'Laddar ned metadata',
          error: 'Nedladdning misslyckades',
          pending: 'Förbereder nedladdning',
          ready: 'Modellen är klar',
        },
      },
    },
  },
  'zh-CN': {
    common: {
      update: '更新',
    },
    plugins: {
      noToolsTitle: '未找到工具',
    },
    speech: {
      convertTask: {
        sources: '来源',
      },
    },
    media: {
      modelDownload: {
        auto: {
          fileName: '处理中',
        },
        completed: '已下载',
        pending: '等待中',
        status: {
          downloading: '正在下载元数据',
          error: '下载失败',
          pending: '准备下载',
          ready: '模型已就绪',
        },
      },
    },
  },
  'zh-TW': {
    common: {
      update: '更新',
    },
    plugins: {
      noToolsTitle: '找不到工具',
    },
    speech: {
      convertTask: {
        sources: '來源',
      },
    },
    media: {
      modelDownload: {
        auto: {
          fileName: '處理檔案中',
        },
        completed: '已下載',
        pending: '等待中',
        status: {
          downloading: '正在下載中繼資料',
          error: '下載失敗',
          pending: '準備下載',
          ready: '模型已就緒',
        },
      },
    },
  },
} satisfies Partial<Record<LocaleKey, Record<string, unknown>>>

const cacheTermBackfills: Partial<
  Record<
    LocaleKey,
    { nav: { cache: string }; tokenEconomy: { cache: string }; system: { cpuCache: string } }
  >
> = {
  'cs-CZ': {
    nav: { cache: 'Mezipaměť' },
    tokenEconomy: { cache: 'Mezipaměť' },
    system: { cpuCache: 'Mezipaměť' },
  },
  'da-DK': {
    nav: { cache: 'Cachelager' },
    tokenEconomy: { cache: 'Cachelager' },
    system: { cpuCache: 'Cachelager' },
  },
  'de-DE': {
    nav: { cache: 'Zwischenspeicher' },
    tokenEconomy: { cache: 'Zwischenspeicher' },
    system: { cpuCache: 'Zwischenspeicher' },
  },
  'fr-FR': {
    nav: { cache: 'Mémoire cache' },
    tokenEconomy: { cache: 'Mémoire cache' },
    system: { cpuCache: 'Mémoire cache' },
  },
  'it-IT': {
    nav: { cache: 'Memoria cache' },
    tokenEconomy: { cache: 'Memoria cache' },
    system: { cpuCache: 'Memoria cache' },
  },
  'nb-NO': {
    nav: { cache: 'Hurtigbuffer' },
    tokenEconomy: { cache: 'Hurtigbuffer' },
    system: { cpuCache: 'Hurtigbuffer' },
  },
  'nl-NL': {
    nav: { cache: 'Cachegeheugen' },
    tokenEconomy: { cache: 'Cachegeheugen' },
    system: { cpuCache: 'Cachegeheugen' },
  },
  'pt-BR': {
    nav: { cache: 'Memória cache' },
    tokenEconomy: { cache: 'Memória cache' },
    system: { cpuCache: 'Memória cache' },
  },
  'pt-PT': {
    nav: { cache: 'Memória cache' },
    tokenEconomy: { cache: 'Memória cache' },
    system: { cpuCache: 'Memória cache' },
  },
  'ro-RO': {
    nav: { cache: 'Memorie cache' },
    tokenEconomy: { cache: 'Memorie cache' },
    system: { cpuCache: 'Memorie cache' },
  },
  'sk-SK': {
    nav: { cache: 'Vyrovnávacia pamäť' },
    tokenEconomy: { cache: 'Vyrovnávacia pamäť' },
    system: { cpuCache: 'Vyrovnávacia pamäť' },
  },
  'sv-SE': {
    nav: { cache: 'Cacheminne' },
    tokenEconomy: { cache: 'Cacheminne' },
    system: { cpuCache: 'Cacheminne' },
  },
}

const backendTermBackfills: Partial<Record<LocaleKey, { backend: string }>> = {
  'ca-ES': { backend: 'Servidor' },
  'cs-CZ': { backend: 'Serverová část' },
  'da-DK': { backend: 'Serverside' },
  'de-DE': { backend: 'Serverteil' },
  'el-GR': { backend: 'Εξυπηρετητής' },
  'it-IT': { backend: 'Back-end' },
  'nb-NO': { backend: 'Baksystem' },
  'ro-RO': { backend: 'Server' },
  'sk-SK': { backend: 'Serverová časť' },
  'sv-SE': { backend: 'Serversida' },
}

const webTermBackfills: Partial<Record<LocaleKey, { web: string }>> = {
  'ca-ES': { web: 'Lloc web' },
  'cs-CZ': { web: 'Webový' },
  'da-DK': { web: 'Net' },
  'de-DE': { web: 'Netz' },
  'es-ES': { web: 'Sitio web' },
  'fr-FR': { web: 'Site web' },
  'hr-HR': { web: 'Mreža' },
  'hu-HU': { web: 'Webes' },
  'nl-NL': { web: 'Website' },
  'ro-RO': { web: 'Webul' },
  'sk-SK': { web: 'Webový' },
}

const mutableLocaleStructuralBackfills =
  localeStructuralBackfills as Partial<Record<LocaleKey, Record<string, unknown>>>

for (const [localeKey, patch] of Object.entries(cacheTermBackfills) as Array<
  [
    LocaleKey,
    { nav: { cache: string }; tokenEconomy: { cache: string }; system: { cpuCache: string } },
  ]
>) {
  const current = (mutableLocaleStructuralBackfills[localeKey] ?? {}) as Record<string, unknown>
  const currentNav = (current.nav ?? {}) as Record<string, unknown>
  const currentTokenEconomy = (current.tokenEconomy ?? {}) as Record<string, unknown>
  const currentSystem = (current.system ?? {}) as Record<string, unknown>

  mutableLocaleStructuralBackfills[localeKey] = {
    ...current,
    nav: {
      ...currentNav,
      ...patch.nav,
    },
    tokenEconomy: {
      ...currentTokenEconomy,
      ...patch.tokenEconomy,
    },
    system: {
      ...currentSystem,
      ...patch.system,
    },
  }
}

for (const [localeKey, patch] of Object.entries(backendTermBackfills) as Array<
  [LocaleKey, { backend: string }]
>) {
  const current = (mutableLocaleStructuralBackfills[localeKey] ?? {}) as Record<string, unknown>
  const currentApiProxy = (current.apiProxy ?? {}) as Record<string, unknown>
  const currentResultCard = (current.resultCard ?? {}) as Record<string, unknown>
  const currentResultCardLabels = (currentResultCard.labels ?? {}) as Record<string, unknown>
  const currentUserdata = (current.userdata ?? {}) as Record<string, unknown>
  const currentUserdataMemory = (currentUserdata.memory ?? {}) as Record<string, unknown>

  mutableLocaleStructuralBackfills[localeKey] = {
    ...current,
    apiProxy: {
      ...currentApiProxy,
      prunerBackend: patch.backend,
    },
    resultCard: {
      ...currentResultCard,
      labels: {
        ...currentResultCardLabels,
        backend: patch.backend,
      },
    },
    userdata: {
      ...currentUserdata,
      memory: {
        ...currentUserdataMemory,
        backend: patch.backend,
      },
    },
  }
}

for (const [localeKey, patch] of Object.entries(webTermBackfills) as Array<
  [LocaleKey, { web: string }]
>) {
  const current = (mutableLocaleStructuralBackfills[localeKey] ?? {}) as Record<string, unknown>
  const currentCompanion = (current.companion ?? {}) as Record<string, unknown>
  const currentCompanionPlatforms = (currentCompanion.platforms ?? {}) as Record<string, unknown>
  const currentTools = (current.tools ?? {}) as Record<string, unknown>
  const currentToolNames = (currentTools.names ?? {}) as Record<string, unknown>

  mutableLocaleStructuralBackfills[localeKey] = {
    ...current,
    companion: {
      ...currentCompanion,
      platforms: {
        ...currentCompanionPlatforms,
        web: patch.web,
      },
    },
    tools: {
      ...currentTools,
      names: {
        ...currentToolNames,
        web: patch.web,
      },
    },
  }
}

export default localeStructuralBackfills
