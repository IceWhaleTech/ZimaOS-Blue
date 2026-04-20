import type { LocaleKey } from './locale-catalog'
import { a11yLocaleTerms } from './builtin-tool-backfills'
import { browserLegacyResultCardBackfills } from './browser-result-card-backfills'

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

const resultCardSearchDetailBackfills: Partial<
  Record<
    LocaleKey,
    {
      resultCard: {
        messages: {
          backend_source: string
          fallback_reason: string
        }
      }
    }
  >
> = {
  'ca-ES': {
    resultCard: {
      messages: {
        backend_source: 'Origen del backend',
        fallback_reason: 'Motiu de fallback',
      },
    },
  },
  'cs-CZ': {
    resultCard: {
      messages: {
        backend_source: 'Zdroj backendu',
        fallback_reason: 'Důvod fallbacku',
      },
    },
  },
  'da-DK': {
    resultCard: {
      messages: {
        backend_source: 'Backend-kilde',
        fallback_reason: 'Fallback-årsag',
      },
    },
  },
  'de-DE': {
    resultCard: {
      messages: {
        backend_source: 'Backend-Quelle',
        fallback_reason: 'Fallback-Grund',
      },
    },
  },
  'el-GR': {
    resultCard: {
      messages: {
        backend_source: 'Πηγή backend',
        fallback_reason: 'Λόγος εναλλακτικής λύσης',
      },
    },
  },
  'en-GB': {
    resultCard: {
      messages: {
        backend_source: 'Backend source',
        fallback_reason: 'Fallback reason',
      },
    },
  },
  'en-US': {
    resultCard: {
      messages: {
        backend_source: 'Backend Source',
        fallback_reason: 'Fallback Reason',
      },
    },
  },
  'es-ES': {
    resultCard: {
      messages: {
        backend_source: 'Origen del backend',
        fallback_reason: 'Motivo de respaldo',
      },
    },
  },
  'fr-FR': {
    resultCard: {
      messages: {
        backend_source: 'Source du back-end',
        fallback_reason: 'Raison du repli',
      },
    },
  },
  'ga-IE': {
    resultCard: {
      messages: {
        backend_source: 'Foinse an innill',
        fallback_reason: 'Cúis chúlaithe',
      },
    },
  },
  'hr-HR': {
    resultCard: {
      messages: {
        backend_source: 'Izvor pozadinskog sustava',
        fallback_reason: 'Razlog povrata',
      },
    },
  },
  'hu-HU': {
    resultCard: {
      messages: {
        backend_source: 'Háttérrendszer forrása',
        fallback_reason: 'Visszalépési ok',
      },
    },
  },
  'it-IT': {
    resultCard: {
      messages: {
        backend_source: 'Origine del backend',
        fallback_reason: 'Motivo del fallback',
      },
    },
  },
  'ja-JP': {
    resultCard: {
      messages: {
        backend_source: 'バックエンドのソース元',
        fallback_reason: 'フォールバック理由',
      },
    },
  },
  'ko-KR': {
    resultCard: {
      messages: {
        backend_source: '백엔드 출처',
        fallback_reason: '폴백 사유',
      },
    },
  },
  'ml-IN': {
    resultCard: {
      messages: {
        backend_source: 'ബാക്ക്എൻഡ് ഉറവിടം',
        fallback_reason: 'Fallback കാരണം',
      },
    },
  },
  'nb-NO': {
    resultCard: {
      messages: {
        backend_source: 'Backend-kilde',
        fallback_reason: 'Fallback-årsak',
      },
    },
  },
  'nl-NL': {
    resultCard: {
      messages: {
        backend_source: 'Bron van de backend',
        fallback_reason: 'Fallback-reden',
      },
    },
  },
  'pl-PL': {
    resultCard: {
      messages: {
        backend_source: 'Źródło zaplecza',
        fallback_reason: 'Przyczyna fallbacku',
      },
    },
  },
  'pt-BR': {
    resultCard: {
      messages: {
        backend_source: 'Origem do back-end',
        fallback_reason: 'Motivo do fallback',
      },
    },
  },
  'pt-PT': {
    resultCard: {
      messages: {
        backend_source: 'Origem do back-end',
        fallback_reason: 'Motivo do fallback',
      },
    },
  },
  'ro-RO': {
    resultCard: {
      messages: {
        backend_source: 'Sursa backendului',
        fallback_reason: 'Motiv de fallback',
      },
    },
  },
  'ru-RU': {
    resultCard: {
      messages: {
        backend_source: 'Источник бэкенда',
        fallback_reason: 'Причина fallback',
      },
    },
  },
  'sk-SK': {
    resultCard: {
      messages: {
        backend_source: 'Zdroj backendu',
        fallback_reason: 'Dôvod fallbacku',
      },
    },
  },
  'sv-SE': {
    resultCard: {
      messages: {
        backend_source: 'Backend-källa',
        fallback_reason: 'Fallback-skäl',
      },
    },
  },
  'zh-CN': {
    resultCard: {
      messages: {
        backend_source: '后端来源',
        fallback_reason: '回退原因',
      },
    },
  },
  'zh-TW': {
    resultCard: {
      messages: {
        backend_source: '後端來源',
        fallback_reason: '回退原因',
      },
    },
  },
}

const browserResultCardMessageBackfills: Partial<
  Record<
    LocaleKey,
    {
      resultCard: {
        messages: {
          browser_tab_ready: string
          browser_not_running: string
          navigation_failed_url_not_allowed: string
          browser_service_not_available: string
          browser_service_not_available_cannot_review_url: string
          proxy_bridge_not_available: string
          proxy_bridge_not_available_cannot_call_vlm: string
          browser_start_failed: string
          navigation_failed: string
        }
      }
    }
  >
> = {
  'ca-ES': {
    resultCard: {
      messages: {
        browser_tab_ready: 'La pestanya del navegador està a punt',
        browser_not_running: "El navegador no s'està executant",
        navigation_failed_url_not_allowed: "La navegació ha fallat: l'URL no està permès",
        browser_service_not_available: 'El servei del navegador no està disponible',
        browser_service_not_available_cannot_review_url:
          'El servei del navegador no està disponible: no es pot revisar la URL',
        proxy_bridge_not_available: 'El pont proxy no està disponible',
        proxy_bridge_not_available_cannot_call_vlm:
          'El pont proxy no està disponible: no es pot cridar VLM',
        browser_start_failed: "El navegador no s'ha pogut iniciar: {msg}",
        navigation_failed: 'La navegació ha fallat: {msg}',
      },
    },
  },
  'cs-CZ': {
    resultCard: {
      messages: {
        browser_tab_ready: 'Karta prohlížeče je připravena',
        browser_not_running: 'Prohlížeč není spuštěn',
        navigation_failed_url_not_allowed: 'Navigace se nezdařila: URL není povolena',
        browser_service_not_available: 'Služba prohlížeče není dostupná',
        browser_service_not_available_cannot_review_url:
          'Služba prohlížeče není dostupná: nelze zkontrolovat URL',
        proxy_bridge_not_available: 'Proxy most není dostupný',
        proxy_bridge_not_available_cannot_call_vlm: 'Proxy most není dostupný: nelze volat VLM',
        browser_start_failed: 'Spuštění prohlížeče se nezdařilo: {msg}',
        navigation_failed: 'Navigace se nezdařila: {msg}',
      },
    },
  },
  'da-DK': {
    resultCard: {
      messages: {
        browser_tab_ready: 'Browserfanen er klar',
        browser_not_running: 'Browseren kører ikke',
        navigation_failed_url_not_allowed: 'Navigation mislykkedes: URL er ikke tilladt',
        browser_service_not_available: 'Browsertjenesten er ikke tilgængelig',
        browser_service_not_available_cannot_review_url:
          'Browsertjenesten er ikke tilgængelig: kan ikke gennemgå URL',
        proxy_bridge_not_available: 'Proxy-bro er ikke tilgængelig',
        proxy_bridge_not_available_cannot_call_vlm:
          'Proxy-bro er ikke tilgængelig: kan ikke kalde VLM',
        browser_start_failed: 'Browserstart mislykkedes: {msg}',
        navigation_failed: 'Navigation mislykkedes: {msg}',
      },
    },
  },
  'de-DE': {
    resultCard: {
      messages: {
        browser_tab_ready: 'Browser-Tab ist bereit',
        browser_not_running: 'Browser läuft nicht',
        navigation_failed_url_not_allowed: 'Navigation fehlgeschlagen: URL ist nicht erlaubt',
        browser_service_not_available: 'Browserdienst ist nicht verfügbar',
        browser_service_not_available_cannot_review_url:
          'Browserdienst ist nicht verfügbar: URL kann nicht geprüft werden',
        proxy_bridge_not_available: 'Proxy-Bridge ist nicht verfügbar',
        proxy_bridge_not_available_cannot_call_vlm:
          'Proxy-Bridge ist nicht verfügbar: VLM kann nicht aufgerufen werden',
        browser_start_failed: 'Browserstart fehlgeschlagen: {msg}',
        navigation_failed: 'Navigation fehlgeschlagen: {msg}',
      },
    },
  },
  'el-GR': {
    resultCard: {
      messages: {
        browser_tab_ready: 'Η καρτέλα του προγράμματος περιήγησης είναι έτοιμη',
        browser_not_running: 'Το πρόγραμμα περιήγησης δεν εκτελείται',
        navigation_failed_url_not_allowed: 'Η πλοήγηση απέτυχε: η διεύθυνση URL δεν επιτρέπεται',
        browser_service_not_available: 'Η υπηρεσία προγράμματος περιήγησης δεν είναι διαθέσιμη',
        browser_service_not_available_cannot_review_url:
          'Η υπηρεσία προγράμματος περιήγησης δεν είναι διαθέσιμη: δεν είναι δυνατή η επισκόπηση URL',
        proxy_bridge_not_available: 'Η γέφυρα proxy δεν είναι διαθέσιμη',
        proxy_bridge_not_available_cannot_call_vlm:
          'Η γέφυρα proxy δεν είναι διαθέσιμη: δεν είναι δυνατή η κλήση VLM',
        browser_start_failed: 'Η εκκίνηση του προγράμματος περιήγησης απέτυχε: {msg}',
        navigation_failed: 'Η πλοήγηση απέτυχε: {msg}',
      },
    },
  },
  'en-GB': {
    resultCard: {
      messages: {
        browser_tab_ready: 'Browser tab ready',
        browser_not_running: 'Browser not running',
        navigation_failed_url_not_allowed: 'Navigation failed: URL not allowed',
        browser_service_not_available: 'Browser service is not available',
        browser_service_not_available_cannot_review_url:
          'Browser service is not available — cannot review URL',
        proxy_bridge_not_available: 'Proxy bridge is not available',
        proxy_bridge_not_available_cannot_call_vlm:
          'Proxy bridge is not available — cannot call VLM',
        browser_start_failed: 'Browser start failed: {msg}',
        navigation_failed: 'Navigation failed: {msg}',
      },
    },
  },
  'en-US': {
    resultCard: {
      messages: {
        browser_tab_ready: 'Browser tab ready',
        browser_not_running: 'Browser not running',
        navigation_failed_url_not_allowed: 'Navigation failed: URL not allowed',
        browser_service_not_available: 'Browser service is not available',
        browser_service_not_available_cannot_review_url:
          'Browser service is not available — cannot review URL',
        proxy_bridge_not_available: 'Proxy bridge is not available',
        proxy_bridge_not_available_cannot_call_vlm:
          'Proxy bridge is not available — cannot call VLM',
        browser_start_failed: 'Browser start failed: {msg}',
        navigation_failed: 'Navigation failed: {msg}',
      },
    },
  },
  'es-ES': {
    resultCard: {
      messages: {
        browser_tab_ready: 'La pestaña del navegador está lista',
        browser_not_running: 'El navegador no está en ejecución',
        navigation_failed_url_not_allowed: 'La navegación falló: URL no permitida',
        browser_service_not_available: 'El servicio del navegador no está disponible',
        browser_service_not_available_cannot_review_url:
          'El servicio del navegador no está disponible: no se puede revisar la URL',
        proxy_bridge_not_available: 'El puente proxy no está disponible',
        proxy_bridge_not_available_cannot_call_vlm:
          'El puente proxy no está disponible: no se puede llamar a VLM',
        browser_start_failed: 'Error al iniciar el navegador: {msg}',
        navigation_failed: 'Error de navegación: {msg}',
      },
    },
  },
  'fr-FR': {
    resultCard: {
      messages: {
        browser_tab_ready: "L'onglet du navigateur est prêt",
        browser_not_running: "Le navigateur n'est pas en cours d'exécution",
        navigation_failed_url_not_allowed: 'Échec de la navigation : URL non autorisée',
        browser_service_not_available: "Le service du navigateur n'est pas disponible",
        browser_service_not_available_cannot_review_url:
          "Le service du navigateur n'est pas disponible — impossible de réviser l'URL",
        proxy_bridge_not_available: "Le pont proxy n'est pas disponible",
        proxy_bridge_not_available_cannot_call_vlm:
          "Le pont proxy n'est pas disponible — impossible d'appeler VLM",
        browser_start_failed: 'Échec du démarrage du navigateur : {msg}',
        navigation_failed: 'Échec de la navigation : {msg}',
      },
    },
  },
  'ga-IE': {
    resultCard: {
      messages: {
        browser_tab_ready: 'Tá cluaisín an bhrabhsálaí réidh',
        browser_not_running: 'Níl an brabhsálaí ag rith',
        navigation_failed_url_not_allowed: 'Theip ar an nascleanúint: níl an URL ceadaithe',
        browser_service_not_available: 'Níl an tseirbhís bhrabhsálaí ar fáil',
        browser_service_not_available_cannot_review_url:
          'Níl an tseirbhís bhrabhsálaí ar fáil — ní féidir an URL a athbhreithniú',
        proxy_bridge_not_available: 'Níl an droichead seachfhreastalaí ar fáil',
        proxy_bridge_not_available_cannot_call_vlm:
          'Níl an droichead seachfhreastalaí ar fáil — ní féidir VLM a ghlaoch',
        browser_start_failed: 'Theip ar thosú an bhrabhsálaí: {msg}',
        navigation_failed: 'Theip ar an nascleanúint: {msg}',
      },
    },
  },
  'hr-HR': {
    resultCard: {
      messages: {
        browser_tab_ready: 'Kartica preglednika je spremna',
        browser_not_running: 'Preglednik nije pokrenut',
        navigation_failed_url_not_allowed: 'Navigacija nije uspjela: URL nije dopušten',
        browser_service_not_available: 'Usluga preglednika nije dostupna',
        browser_service_not_available_cannot_review_url:
          'Usluga preglednika nije dostupna: nije moguće pregledati URL',
        proxy_bridge_not_available: 'Proxy most nije dostupan',
        proxy_bridge_not_available_cannot_call_vlm:
          'Proxy most nije dostupan: nije moguće pozvati VLM',
        browser_start_failed: 'Pokretanje preglednika nije uspjelo: {msg}',
        navigation_failed: 'Navigacija nije uspjela: {msg}',
      },
    },
  },
  'hu-HU': {
    resultCard: {
      messages: {
        browser_tab_ready: 'A böngészőlap készen áll',
        browser_not_running: 'A böngésző nem fut',
        navigation_failed_url_not_allowed: 'A navigáció sikertelen: az URL nem engedélyezett',
        browser_service_not_available: 'A böngészőszolgáltatás nem érhető el',
        browser_service_not_available_cannot_review_url:
          'A böngészőszolgáltatás nem érhető el — nem lehet felülvizsgálni az URL-t',
        proxy_bridge_not_available: 'A proxy híd nem érhető el',
        proxy_bridge_not_available_cannot_call_vlm:
          'A proxy híd nem érhető el — nem lehet meghívni a VLM-et',
        browser_start_failed: 'A böngésző indítása sikertelen: {msg}',
        navigation_failed: 'A navigáció sikertelen: {msg}',
      },
    },
  },
  'it-IT': {
    resultCard: {
      messages: {
        browser_tab_ready: 'La scheda del browser è pronta',
        browser_not_running: 'Il browser non è in esecuzione',
        navigation_failed_url_not_allowed: 'Navigazione non riuscita: URL non consentito',
        browser_service_not_available: 'Il servizio del browser non è disponibile',
        browser_service_not_available_cannot_review_url:
          "Il servizio del browser non è disponibile: impossibile rivedere l'URL",
        proxy_bridge_not_available: 'Il ponte proxy non è disponibile',
        proxy_bridge_not_available_cannot_call_vlm:
          'Il ponte proxy non è disponibile: impossibile chiamare VLM',
        browser_start_failed: 'Avvio del browser non riuscito: {msg}',
        navigation_failed: 'Navigazione non riuscita: {msg}',
      },
    },
  },
  'ja-JP': {
    resultCard: {
      messages: {
        browser_tab_ready: 'ブラウザータブの準備ができました',
        browser_not_running: 'ブラウザーが起動していません',
        navigation_failed_url_not_allowed: 'ナビゲーションに失敗しました: URL は許可されていません',
        browser_service_not_available: 'ブラウザサービスは利用できません',
        browser_service_not_available_cannot_review_url:
          'ブラウザサービスは利用できません — URL をレビューできません',
        proxy_bridge_not_available: 'プロキシブリッジは利用できません',
        proxy_bridge_not_available_cannot_call_vlm:
          'プロキシブリッジは利用できません — VLM を呼び出せません',
        browser_start_failed: 'ブラウザの起動に失敗しました: {msg}',
        navigation_failed: 'ナビゲーションに失敗しました: {msg}',
      },
    },
  },
  'ko-KR': {
    resultCard: {
      messages: {
        browser_tab_ready: '브라우저 탭이 준비되었습니다',
        browser_not_running: '브라우저가 실행 중이 아닙니다',
        navigation_failed_url_not_allowed: '탐색 실패: URL이 허용되지 않습니다',
        browser_service_not_available: '브라우저 서비스를 사용할 수 없습니다',
        browser_service_not_available_cannot_review_url:
          '브라우저 서비스를 사용할 수 없습니다 — URL을 검토할 수 없습니다',
        proxy_bridge_not_available: '프록시 브리지를 사용할 수 없습니다',
        proxy_bridge_not_available_cannot_call_vlm:
          '프록시 브리지를 사용할 수 없습니다 — VLM을 호출할 수 없습니다',
        browser_start_failed: '브라우저 시작 실패: {msg}',
        navigation_failed: '탐색 실패: {msg}',
      },
    },
  },
  'ml-IN': {
    resultCard: {
      messages: {
        browser_tab_ready: 'ബ്രൗസർ ടാബ് തയ്യാറാണ്',
        browser_not_running: 'ബ്രൗസർ പ്രവർത്തിക്കുന്നില്ല',
        navigation_failed_url_not_allowed: 'നാവിഗേഷൻ പരാജയപ്പെട്ടു: URL അനുവദനീയമല്ല',
        browser_service_not_available: 'ബ്രൗസർ സേവനം ലഭ്യമല്ല',
        browser_service_not_available_cannot_review_url:
          'ബ്രൗസർ സേവനം ലഭ്യമല്ല — URL അവലോകനം ചെയ്യാൻ കഴിയില്ല',
        proxy_bridge_not_available: 'പ്രോക്സി ബ്രിഡ്ജ് ലഭ്യമല്ല',
        proxy_bridge_not_available_cannot_call_vlm:
          'പ്രോക്സി ബ്രിഡ്ജ് ലഭ്യമല്ല — VLM വിളിക്കാൻ കഴിയില്ല',
        browser_start_failed: 'ബ്രൗസർ ആരംഭം പരാജയപ്പെട്ടു: {msg}',
        navigation_failed: 'നാവിഗേഷൻ പരാജയപ്പെട്ടു: {msg}',
      },
    },
  },
  'nb-NO': {
    resultCard: {
      messages: {
        browser_tab_ready: 'Nettleserfanen er klar',
        browser_not_running: 'Nettleseren kjører ikke',
        navigation_failed_url_not_allowed: 'Navigasjonen mislyktes: URL er ikke tillatt',
        browser_service_not_available: 'Nettlesertjenesten er ikke tilgjengelig',
        browser_service_not_available_cannot_review_url:
          'Nettlesertjenesten er ikke tilgjengelig — kan ikke gjennomgå URL',
        proxy_bridge_not_available: 'Proxy-bro er ikke tilgjengelig',
        proxy_bridge_not_available_cannot_call_vlm:
          'Proxy-bro er ikke tilgjengelig — kan ikke kalle VLM',
        browser_start_failed: 'Nettleserstart mislyktes: {msg}',
        navigation_failed: 'Navigering mislyktes: {msg}',
      },
    },
  },
  'nl-NL': {
    resultCard: {
      messages: {
        browser_tab_ready: 'Browsertabblad is gereed',
        browser_not_running: 'Browser draait niet',
        navigation_failed_url_not_allowed: 'Navigatie mislukt: URL is niet toegestaan',
        browser_service_not_available: 'Browserservice is niet beschikbaar',
        browser_service_not_available_cannot_review_url:
          'Browserservice is niet beschikbaar — kan URL niet beoordelen',
        proxy_bridge_not_available: 'Proxy-bridge is niet beschikbaar',
        proxy_bridge_not_available_cannot_call_vlm:
          'Proxy-bridge is niet beschikbaar — kan VLM niet aanroepen',
        browser_start_failed: 'Browser starten mislukt: {msg}',
        navigation_failed: 'Navigatie mislukt: {msg}',
      },
    },
  },
  'pl-PL': {
    resultCard: {
      messages: {
        browser_tab_ready: 'Karta przeglądarki jest gotowa',
        browser_not_running: 'Przeglądarka nie jest uruchomiona',
        navigation_failed_url_not_allowed:
          'Nawigacja nie powiodła się: adres URL jest niedozwolony',
        browser_service_not_available: 'Usługa przeglądarki jest niedostępna',
        browser_service_not_available_cannot_review_url:
          'Usługa przeglądarki jest niedostępna — nie można przejrzeć adresu URL',
        proxy_bridge_not_available: 'Mostek proxy jest niedostępny',
        proxy_bridge_not_available_cannot_call_vlm:
          'Mostek proxy jest niedostępny — nie można wywołać VLM',
        browser_start_failed: 'Nie udało się uruchomić przeglądarki: {msg}',
        navigation_failed: 'Nawigacja nie powiodła się: {msg}',
      },
    },
  },
  'pt-BR': {
    resultCard: {
      messages: {
        browser_tab_ready: 'A guia do navegador está pronta',
        browser_not_running: 'O navegador não está em execução',
        navigation_failed_url_not_allowed: 'Falha na navegação: URL não permitida',
        browser_service_not_available: 'O serviço do navegador não está disponível',
        browser_service_not_available_cannot_review_url:
          'O serviço do navegador não está disponível — não é possível revisar a URL',
        proxy_bridge_not_available: 'A ponte proxy não está disponível',
        proxy_bridge_not_available_cannot_call_vlm:
          'A ponte proxy não está disponível — não é possível chamar VLM',
        browser_start_failed: 'Falha ao iniciar o navegador: {msg}',
        navigation_failed: 'Falha na navegação: {msg}',
      },
    },
  },
  'pt-PT': {
    resultCard: {
      messages: {
        browser_tab_ready: 'O separador do navegador está pronto',
        browser_not_running: 'O navegador não está em execução',
        navigation_failed_url_not_allowed: 'Falha na navegação: URL não permitida',
        browser_service_not_available: 'O serviço do navegador não está disponível',
        browser_service_not_available_cannot_review_url:
          'O serviço do navegador não está disponível — não é possível rever o URL',
        proxy_bridge_not_available: 'A ponte proxy não está disponível',
        proxy_bridge_not_available_cannot_call_vlm:
          'A ponte proxy não está disponível — não é possível chamar VLM',
        browser_start_failed: 'Falha ao iniciar o navegador: {msg}',
        navigation_failed: 'Falha na navegação: {msg}',
      },
    },
  },
  'ro-RO': {
    resultCard: {
      messages: {
        browser_tab_ready: 'Fila browserului este pregătită',
        browser_not_running: 'Browserul nu rulează',
        navigation_failed_url_not_allowed: 'Navigarea a eșuat: URL-ul nu este permis',
        browser_service_not_available: 'Serviciul de browser nu este disponibil',
        browser_service_not_available_cannot_review_url:
          'Serviciul de browser nu este disponibil — nu se poate revizui URL-ul',
        proxy_bridge_not_available: 'Puntea proxy nu este disponibilă',
        proxy_bridge_not_available_cannot_call_vlm:
          'Puntea proxy nu este disponibilă — nu se poate apela VLM',
        browser_start_failed: 'Pornirea browserului a eșuat: {msg}',
        navigation_failed: 'Navigarea a eșuat: {msg}',
      },
    },
  },
  'ru-RU': {
    resultCard: {
      messages: {
        browser_tab_ready: 'Вкладка браузера готова',
        browser_not_running: 'Браузер не запущен',
        navigation_failed_url_not_allowed: 'Навигация не удалась: URL не разрешён',
        browser_service_not_available: 'Служба браузера недоступна',
        browser_service_not_available_cannot_review_url:
          'Служба браузера недоступна — невозможно проверить URL',
        proxy_bridge_not_available: 'Прокси-мост недоступен',
        proxy_bridge_not_available_cannot_call_vlm:
          'Прокси-мост недоступен — невозможно вызвать VLM',
        browser_start_failed: 'Не удалось запустить браузер: {msg}',
        navigation_failed: 'Ошибка навигации: {msg}',
      },
    },
  },
  'sk-SK': {
    resultCard: {
      messages: {
        browser_tab_ready: 'Karta prehliadača je pripravená',
        browser_not_running: 'Prehliadač nie je spustený',
        navigation_failed_url_not_allowed: 'Navigácia zlyhala: URL nie je povolená',
        browser_service_not_available: 'Služba prehliadača nie je dostupná',
        browser_service_not_available_cannot_review_url:
          'Služba prehliadača nie je dostupná — nie je možné skontrolovať URL',
        proxy_bridge_not_available: 'Proxy most nie je dostupný',
        proxy_bridge_not_available_cannot_call_vlm:
          'Proxy most nie je dostupný — nie je možné zavolať VLM',
        browser_start_failed: 'Spustenie prehliadača zlyhalo: {msg}',
        navigation_failed: 'Navigácia zlyhala: {msg}',
      },
    },
  },
  'sv-SE': {
    resultCard: {
      messages: {
        browser_tab_ready: 'Webbläsarfliken är klar',
        browser_not_running: 'Webbläsaren körs inte',
        navigation_failed_url_not_allowed: 'Navigeringen misslyckades: URL:en är inte tillåten',
        browser_service_not_available: 'Webbläsartjänsten är inte tillgänglig',
        browser_service_not_available_cannot_review_url:
          'Webbläsartjänsten är inte tillgänglig — kan inte granska URL',
        proxy_bridge_not_available: 'Proxy-bron är inte tillgänglig',
        proxy_bridge_not_available_cannot_call_vlm:
          'Proxy-bron är inte tillgänglig — kan inte anropa VLM',
        browser_start_failed: 'Webbläsarstart misslyckades: {msg}',
        navigation_failed: 'Navigering misslyckades: {msg}',
      },
    },
  },
  'zh-CN': {
    resultCard: {
      messages: {
        browser_tab_ready: '浏览器标签页已就绪',
        browser_not_running: '浏览器未运行',
        navigation_failed_url_not_allowed: '导航失败：URL 不被允许',
        browser_service_not_available: '浏览器服务不可用',
        browser_service_not_available_cannot_review_url: '浏览器服务不可用 — 无法评审 URL',
        proxy_bridge_not_available: '代理桥不可用',
        proxy_bridge_not_available_cannot_call_vlm: '代理桥不可用 — 无法调用 VLM',
        browser_start_failed: '浏览器启动失败：{msg}',
        navigation_failed: '导航失败：{msg}',
      },
    },
  },
  'zh-TW': {
    resultCard: {
      messages: {
        browser_tab_ready: '瀏覽器分頁已就緒',
        browser_not_running: '瀏覽器未執行',
        navigation_failed_url_not_allowed: '導覽失敗：URL 不被允許',
        browser_service_not_available: '瀏覽器服務不可用',
        browser_service_not_available_cannot_review_url: '瀏覽器服務不可用 — 無法評審 URL',
        proxy_bridge_not_available: '代理橋不可用',
        proxy_bridge_not_available_cannot_call_vlm: '代理橋不可用 — 無法呼叫 VLM',
        browser_start_failed: '瀏覽器啟動失敗：{msg}',
        navigation_failed: '導覽失敗：{msg}',
      },
    },
  },
}

const browserScreenshotResultCardBackfills: Partial<
  Record<
    LocaleKey,
    {
      resultCard: {
        messages: {
          screenshot_captured: string
          screenshot_captured_interactive_elements_unavailable: string
        }
        messageTemplates: {
          screenshot_captured_for: string
        }
      }
    }
  >
> = {
  'ca-ES': {
    resultCard: {
      messages: {
        screenshot_captured: 'Captura de pantalla capturada',
        screenshot_captured_interactive_elements_unavailable:
          'Captura de pantalla capturada (els elements interactius no estan disponibles)',
      },
      messageTemplates: {
        screenshot_captured_for: 'Captura de pantalla capturada per a {target}',
      },
    },
  },
  'cs-CZ': {
    resultCard: {
      messages: {
        screenshot_captured: 'Snímek obrazovky pořízen',
        screenshot_captured_interactive_elements_unavailable:
          'Snímek obrazovky pořízen (interaktivní prvky nejsou dostupné)',
      },
      messageTemplates: {
        screenshot_captured_for: 'Snímek obrazovky pořízen pro {target}',
      },
    },
  },
  'da-DK': {
    resultCard: {
      messages: {
        screenshot_captured: 'Skærmbillede taget',
        screenshot_captured_interactive_elements_unavailable:
          'Skærmbillede taget (interaktive elementer er ikke tilgængelige)',
      },
      messageTemplates: {
        screenshot_captured_for: 'Skærmbillede taget for {target}',
      },
    },
  },
  'de-DE': {
    resultCard: {
      messages: {
        screenshot_captured: 'Screenshot aufgenommen',
        screenshot_captured_interactive_elements_unavailable:
          'Screenshot aufgenommen (interaktive Elemente nicht verfügbar)',
      },
      messageTemplates: {
        screenshot_captured_for: 'Screenshot für {target} aufgenommen',
      },
    },
  },
  'el-GR': {
    resultCard: {
      messages: {
        screenshot_captured: 'Το στιγμιότυπο οθόνης καταγράφηκε',
        screenshot_captured_interactive_elements_unavailable:
          'Το στιγμιότυπο οθόνης καταγράφηκε (τα διαδραστικά στοιχεία δεν είναι διαθέσιμα)',
      },
      messageTemplates: {
        screenshot_captured_for: 'Το στιγμιότυπο οθόνης καταγράφηκε για το {target}',
      },
    },
  },
  'en-GB': {
    resultCard: {
      messages: {
        screenshot_captured: 'Screenshot captured',
        screenshot_captured_interactive_elements_unavailable:
          'Screenshot captured (interactive elements unavailable)',
      },
      messageTemplates: {
        screenshot_captured_for: 'Screenshot captured for {target}',
      },
    },
  },
  'en-US': {
    resultCard: {
      messages: {
        screenshot_captured: 'Screenshot captured',
        screenshot_captured_interactive_elements_unavailable:
          'Screenshot captured (interactive elements unavailable)',
      },
      messageTemplates: {
        screenshot_captured_for: 'Screenshot captured for {target}',
      },
    },
  },
  'es-ES': {
    resultCard: {
      messages: {
        screenshot_captured: 'Captura de pantalla realizada',
        screenshot_captured_interactive_elements_unavailable:
          'Captura de pantalla realizada (elementos interactivos no disponibles)',
      },
      messageTemplates: {
        screenshot_captured_for: 'Captura de pantalla realizada para {target}',
      },
    },
  },
  'fr-FR': {
    resultCard: {
      messages: {
        screenshot_captured: 'Capture d’écran effectuée',
        screenshot_captured_interactive_elements_unavailable:
          'Capture d’écran effectuée (éléments interactifs indisponibles)',
      },
      messageTemplates: {
        screenshot_captured_for: 'Capture d’écran effectuée pour {target}',
      },
    },
  },
  'ga-IE': {
    resultCard: {
      messages: {
        screenshot_captured: 'Gabháil scáileáin déanta',
        screenshot_captured_interactive_elements_unavailable:
          'Gabháil scáileáin déanta (níl eilimintí idirghníomhacha ar fáil)',
      },
      messageTemplates: {
        screenshot_captured_for: 'Gabháil scáileáin déanta do {target}',
      },
    },
  },
  'hr-HR': {
    resultCard: {
      messages: {
        screenshot_captured: 'Snimka zaslona je snimljena',
        screenshot_captured_interactive_elements_unavailable:
          'Snimka zaslona je snimljena (interaktivni elementi nisu dostupni)',
      },
      messageTemplates: {
        screenshot_captured_for: 'Snimka zaslona je snimljena za {target}',
      },
    },
  },
  'hu-HU': {
    resultCard: {
      messages: {
        screenshot_captured: 'Képernyőkép rögzítve',
        screenshot_captured_interactive_elements_unavailable:
          'Képernyőkép rögzítve (az interaktív elemek nem érhetők el)',
      },
      messageTemplates: {
        screenshot_captured_for: 'Képernyőkép rögzítve ehhez: {target}',
      },
    },
  },
  'it-IT': {
    resultCard: {
      messages: {
        screenshot_captured: 'Screenshot acquisito',
        screenshot_captured_interactive_elements_unavailable:
          'Screenshot acquisito (elementi interattivi non disponibili)',
      },
      messageTemplates: {
        screenshot_captured_for: 'Screenshot acquisito per {target}',
      },
    },
  },
  'ja-JP': {
    resultCard: {
      messages: {
        screenshot_captured: 'スクリーンショットを取得しました',
        screenshot_captured_interactive_elements_unavailable:
          'スクリーンショットを取得しました（インタラクティブ要素は利用できません）',
      },
      messageTemplates: {
        screenshot_captured_for: '{target} のスクリーンショットを取得しました',
      },
    },
  },
  'ko-KR': {
    resultCard: {
      messages: {
        screenshot_captured: '스크린샷을 캡처했습니다',
        screenshot_captured_interactive_elements_unavailable:
          '스크린샷을 캡처했습니다(상호작용 요소를 사용할 수 없음)',
      },
      messageTemplates: {
        screenshot_captured_for: '{target}의 스크린샷을 캡처했습니다',
      },
    },
  },
  'ml-IN': {
    resultCard: {
      messages: {
        screenshot_captured: 'സ്ക്രീൻഷോട്ട് പകർത്തി',
        screenshot_captured_interactive_elements_unavailable:
          'സ്ക്രീൻഷോട്ട് പകർത്തി (ഇന്ററാക്ടീവ് ഘടകങ്ങൾ ലഭ്യമല്ല)',
      },
      messageTemplates: {
        screenshot_captured_for: '{target} ന് വേണ്ടി സ്ക്രീൻഷോട്ട് പകർത്തി',
      },
    },
  },
  'nb-NO': {
    resultCard: {
      messages: {
        screenshot_captured: 'Skjermbilde tatt',
        screenshot_captured_interactive_elements_unavailable:
          'Skjermbilde tatt (interaktive elementer er ikke tilgjengelige)',
      },
      messageTemplates: {
        screenshot_captured_for: 'Skjermbilde tatt for {target}',
      },
    },
  },
  'nl-NL': {
    resultCard: {
      messages: {
        screenshot_captured: 'Screenshot gemaakt',
        screenshot_captured_interactive_elements_unavailable:
          'Screenshot gemaakt (interactieve elementen niet beschikbaar)',
      },
      messageTemplates: {
        screenshot_captured_for: 'Screenshot gemaakt voor {target}',
      },
    },
  },
  'pl-PL': {
    resultCard: {
      messages: {
        screenshot_captured: 'Zrzut ekranu został wykonany',
        screenshot_captured_interactive_elements_unavailable:
          'Zrzut ekranu został wykonany (elementy interaktywne są niedostępne)',
      },
      messageTemplates: {
        screenshot_captured_for: 'Wykonano zrzut ekranu dla {target}',
      },
    },
  },
  'pt-BR': {
    resultCard: {
      messages: {
        screenshot_captured: 'Captura de tela realizada',
        screenshot_captured_interactive_elements_unavailable:
          'Captura de tela realizada (elementos interativos indisponíveis)',
      },
      messageTemplates: {
        screenshot_captured_for: 'Captura de tela realizada para {target}',
      },
    },
  },
  'pt-PT': {
    resultCard: {
      messages: {
        screenshot_captured: 'Captura de ecrã efetuada',
        screenshot_captured_interactive_elements_unavailable:
          'Captura de ecrã efetuada (elementos interativos indisponíveis)',
      },
      messageTemplates: {
        screenshot_captured_for: 'Captura de ecrã efetuada para {target}',
      },
    },
  },
  'ro-RO': {
    resultCard: {
      messages: {
        screenshot_captured: 'Captura de ecran a fost realizată',
        screenshot_captured_interactive_elements_unavailable:
          'Captura de ecran a fost realizată (elementele interactive nu sunt disponibile)',
      },
      messageTemplates: {
        screenshot_captured_for: 'Captura de ecran a fost realizată pentru {target}',
      },
    },
  },
  'ru-RU': {
    resultCard: {
      messages: {
        screenshot_captured: 'Скриншот сделан',
        screenshot_captured_interactive_elements_unavailable:
          'Скриншот сделан (интерактивные элементы недоступны)',
      },
      messageTemplates: {
        screenshot_captured_for: 'Скриншот сделан для {target}',
      },
    },
  },
  'sk-SK': {
    resultCard: {
      messages: {
        screenshot_captured: 'Snímka obrazovky bola vytvorená',
        screenshot_captured_interactive_elements_unavailable:
          'Snímka obrazovky bola vytvorená (interaktívne prvky nie sú dostupné)',
      },
      messageTemplates: {
        screenshot_captured_for: 'Snímka obrazovky bola vytvorená pre {target}',
      },
    },
  },
  'sv-SE': {
    resultCard: {
      messages: {
        screenshot_captured: 'Skärmbild tagen',
        screenshot_captured_interactive_elements_unavailable:
          'Skärmbild tagen (interaktiva element är inte tillgängliga)',
      },
      messageTemplates: {
        screenshot_captured_for: 'Skärmbild tagen för {target}',
      },
    },
  },
  'zh-CN': {
    resultCard: {
      messages: {
        screenshot_captured: '已捕获截图',
        screenshot_captured_interactive_elements_unavailable: '已捕获截图（交互元素不可用）',
      },
      messageTemplates: {
        screenshot_captured_for: '已为 {target} 捕获截图',
      },
    },
  },
  'zh-TW': {
    resultCard: {
      messages: {
        screenshot_captured: '已擷取螢幕截圖',
        screenshot_captured_interactive_elements_unavailable: '已擷取螢幕截圖（互動元素不可用）',
      },
      messageTemplates: {
        screenshot_captured_for: '已為 {target} 擷取螢幕截圖',
      },
    },
  },
}

const browserScreenshotLegacyResultCardBackfills: Partial<
  Record<
    LocaleKey,
    {
      resultCard: {
        messages: {
          screenshot_captured_for_active_tab: string
        }
        messageTemplates: {
          screenshot_captured_for_tab: string
        }
      }
    }
  >
> = {
  'ca-ES': {
    resultCard: {
      messages: {
        screenshot_captured_for_active_tab:
          'Captura de pantalla capturada per a la pestanya activa',
      },
      messageTemplates: {
        screenshot_captured_for_tab: 'Captura de pantalla capturada per a la pestanya {target}',
      },
    },
  },
  'cs-CZ': {
    resultCard: {
      messages: {
        screenshot_captured_for_active_tab: 'Snímek obrazovky pořízen pro aktivní kartu',
      },
      messageTemplates: {
        screenshot_captured_for_tab: 'Snímek obrazovky pořízen pro kartu {target}',
      },
    },
  },
  'da-DK': {
    resultCard: {
      messages: { screenshot_captured_for_active_tab: 'Skærmbillede taget for den aktive fane' },
      messageTemplates: { screenshot_captured_for_tab: 'Skærmbillede taget for fanen {target}' },
    },
  },
  'de-DE': {
    resultCard: {
      messages: { screenshot_captured_for_active_tab: 'Screenshot für aktiven Tab aufgenommen' },
      messageTemplates: { screenshot_captured_for_tab: 'Screenshot für Tab {target} aufgenommen' },
    },
  },
  'el-GR': {
    resultCard: {
      messages: {
        screenshot_captured_for_active_tab:
          'Το στιγμιότυπο οθόνης καταγράφηκε για την ενεργή καρτέλα',
      },
      messageTemplates: {
        screenshot_captured_for_tab: 'Το στιγμιότυπο οθόνης καταγράφηκε για την καρτέλα {target}',
      },
    },
  },
  'en-GB': {
    resultCard: {
      messages: { screenshot_captured_for_active_tab: 'Screenshot captured for active tab' },
      messageTemplates: { screenshot_captured_for_tab: 'Screenshot captured for tab {target}' },
    },
  },
  'en-US': {
    resultCard: {
      messages: { screenshot_captured_for_active_tab: 'Screenshot captured for active tab' },
      messageTemplates: { screenshot_captured_for_tab: 'Screenshot captured for tab {target}' },
    },
  },
  'es-ES': {
    resultCard: {
      messages: {
        screenshot_captured_for_active_tab: 'Captura de pantalla realizada para la pestaña activa',
      },
      messageTemplates: {
        screenshot_captured_for_tab: 'Captura de pantalla realizada para la pestaña {target}',
      },
    },
  },
  'fr-FR': {
    resultCard: {
      messages: {
        screenshot_captured_for_active_tab: 'Capture d’écran effectuée pour l’onglet actif',
      },
      messageTemplates: {
        screenshot_captured_for_tab: 'Capture d’écran effectuée pour l’onglet {target}',
      },
    },
  },
  'ga-IE': {
    resultCard: {
      messages: {
        screenshot_captured_for_active_tab: 'Gabháil scáileáin déanta don chluaisín gníomhach',
      },
      messageTemplates: {
        screenshot_captured_for_tab: 'Gabháil scáileáin déanta don chluaisín {target}',
      },
    },
  },
  'hr-HR': {
    resultCard: {
      messages: {
        screenshot_captured_for_active_tab: 'Snimka zaslona je snimljena za aktivnu karticu',
      },
      messageTemplates: {
        screenshot_captured_for_tab: 'Snimka zaslona je snimljena za karticu {target}',
      },
    },
  },
  'hu-HU': {
    resultCard: {
      messages: { screenshot_captured_for_active_tab: 'Képernyőkép rögzítve az aktív lapról' },
      messageTemplates: {
        screenshot_captured_for_tab: 'Képernyőkép rögzítve ehhez a laphoz: {target}',
      },
    },
  },
  'it-IT': {
    resultCard: {
      messages: { screenshot_captured_for_active_tab: 'Screenshot acquisito per la scheda attiva' },
      messageTemplates: {
        screenshot_captured_for_tab: 'Screenshot acquisito per la scheda {target}',
      },
    },
  },
  'ja-JP': {
    resultCard: {
      messages: {
        screenshot_captured_for_active_tab: '現在のタブのスクリーンショットを取得しました',
      },
      messageTemplates: {
        screenshot_captured_for_tab: 'タブ {target} のスクリーンショットを取得しました',
      },
    },
  },
  'ko-KR': {
    resultCard: {
      messages: { screenshot_captured_for_active_tab: '현재 탭의 스크린샷을 캡처했습니다' },
      messageTemplates: { screenshot_captured_for_tab: '탭 {target}의 스크린샷을 캡처했습니다' },
    },
  },
  'ml-IN': {
    resultCard: {
      messages: { screenshot_captured_for_active_tab: 'നിലവിലെ ടാബിനായി സ്ക്രീൻഷോട്ട് പകർത്തി' },
      messageTemplates: { screenshot_captured_for_tab: '{target} ടാബിനായി സ്ക്രീൻഷോട്ട് പകർത്തി' },
    },
  },
  'nb-NO': {
    resultCard: {
      messages: { screenshot_captured_for_active_tab: 'Skjermbilde tatt for den aktive fanen' },
      messageTemplates: { screenshot_captured_for_tab: 'Skjermbilde tatt for fanen {target}' },
    },
  },
  'nl-NL': {
    resultCard: {
      messages: { screenshot_captured_for_active_tab: 'Screenshot gemaakt voor actief tabblad' },
      messageTemplates: { screenshot_captured_for_tab: 'Screenshot gemaakt voor tabblad {target}' },
    },
  },
  'pl-PL': {
    resultCard: {
      messages: { screenshot_captured_for_active_tab: 'Wykonano zrzut ekranu dla aktywnej karty' },
      messageTemplates: { screenshot_captured_for_tab: 'Wykonano zrzut ekranu dla karty {target}' },
    },
  },
  'pt-BR': {
    resultCard: {
      messages: {
        screenshot_captured_for_active_tab: 'Captura de tela realizada para a guia ativa',
      },
      messageTemplates: {
        screenshot_captured_for_tab: 'Captura de tela realizada para a guia {target}',
      },
    },
  },
  'pt-PT': {
    resultCard: {
      messages: {
        screenshot_captured_for_active_tab: 'Captura de ecrã efetuada para o separador ativo',
      },
      messageTemplates: {
        screenshot_captured_for_tab: 'Captura de ecrã efetuada para o separador {target}',
      },
    },
  },
  'ro-RO': {
    resultCard: {
      messages: {
        screenshot_captured_for_active_tab: 'Captura de ecran a fost realizată pentru fila activă',
      },
      messageTemplates: {
        screenshot_captured_for_tab: 'Captura de ecran a fost realizată pentru fila {target}',
      },
    },
  },
  'ru-RU': {
    resultCard: {
      messages: { screenshot_captured_for_active_tab: 'Скриншот сделан для активной вкладки' },
      messageTemplates: { screenshot_captured_for_tab: 'Скриншот сделан для вкладки {target}' },
    },
  },
  'sk-SK': {
    resultCard: {
      messages: {
        screenshot_captured_for_active_tab: 'Snímka obrazovky bola vytvorená pre aktívnu kartu',
      },
      messageTemplates: {
        screenshot_captured_for_tab: 'Snímka obrazovky bola vytvorená pre kartu {target}',
      },
    },
  },
  'sv-SE': {
    resultCard: {
      messages: { screenshot_captured_for_active_tab: 'Skärmbild tagen för den aktiva fliken' },
      messageTemplates: { screenshot_captured_for_tab: 'Skärmbild tagen för fliken {target}' },
    },
  },
  'zh-CN': {
    resultCard: {
      messages: { screenshot_captured_for_active_tab: '已为当前标签页捕获截图' },
      messageTemplates: { screenshot_captured_for_tab: '已为标签页 {target} 捕获截图' },
    },
  },
  'zh-TW': {
    resultCard: {
      messages: { screenshot_captured_for_active_tab: '已為目前分頁擷取螢幕截圖' },
      messageTemplates: { screenshot_captured_for_tab: '已為分頁 {target} 擷取螢幕截圖' },
    },
  },
}

const mutableLocaleStructuralBackfills = localeStructuralBackfills as Partial<
  Record<LocaleKey, Record<string, unknown>>
>

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

for (const [localeKey, patch] of Object.entries(resultCardSearchDetailBackfills) as Array<
  [
    LocaleKey,
    {
      resultCard: {
        messages: {
          backend_source: string
          fallback_reason: string
        }
      }
    },
  ]
>) {
  const current = (mutableLocaleStructuralBackfills[localeKey] ?? {}) as Record<string, unknown>
  const currentResultCard = (current.resultCard ?? {}) as Record<string, unknown>
  const currentResultCardMessages = (currentResultCard.messages ?? {}) as Record<string, unknown>

  mutableLocaleStructuralBackfills[localeKey] = {
    ...current,
    resultCard: {
      ...currentResultCard,
      messages: {
        ...currentResultCardMessages,
        ...patch.resultCard.messages,
      },
    },
  }
}

for (const [localeKey, patch] of Object.entries(browserResultCardMessageBackfills) as Array<
  [
    LocaleKey,
    {
      resultCard: {
        messages: {
          browser_tab_ready: string
          browser_not_running: string
          navigation_failed_url_not_allowed: string
          browser_service_not_available: string
          browser_service_not_available_cannot_review_url: string
          proxy_bridge_not_available: string
          proxy_bridge_not_available_cannot_call_vlm: string
          browser_start_failed: string
          navigation_failed: string
        }
      }
    },
  ]
>) {
  const current = (mutableLocaleStructuralBackfills[localeKey] ?? {}) as Record<string, unknown>
  const currentResultCard = (current.resultCard ?? {}) as Record<string, unknown>
  const currentResultCardMessages = (currentResultCard.messages ?? {}) as Record<string, unknown>
  const currentResultCardTemplates = (currentResultCard.messageTemplates ?? {}) as Record<
    string,
    unknown
  >

  mutableLocaleStructuralBackfills[localeKey] = {
    ...current,
    resultCard: {
      ...currentResultCard,
      messages: {
        ...currentResultCardMessages,
        ...patch.resultCard.messages,
      },
      messageTemplates: {
        ...currentResultCardTemplates,
        browser_start_failed: patch.resultCard.messages.browser_start_failed,
        navigation_failed: patch.resultCard.messages.navigation_failed,
      },
    },
  }
}

for (const [localeKey, patch] of Object.entries(browserScreenshotResultCardBackfills) as Array<
  [
    LocaleKey,
    {
      resultCard: {
        messages: {
          screenshot_captured: string
          screenshot_captured_interactive_elements_unavailable: string
        }
        messageTemplates: {
          screenshot_captured_for: string
        }
      }
    },
  ]
>) {
  const current = (mutableLocaleStructuralBackfills[localeKey] ?? {}) as Record<string, unknown>
  const currentResultCard = (current.resultCard ?? {}) as Record<string, unknown>
  const currentResultCardMessages = (currentResultCard.messages ?? {}) as Record<string, unknown>
  const currentResultCardTemplates = (currentResultCard.messageTemplates ?? {}) as Record<
    string,
    unknown
  >

  mutableLocaleStructuralBackfills[localeKey] = {
    ...current,
    resultCard: {
      ...currentResultCard,
      messages: {
        ...currentResultCardMessages,
        ...patch.resultCard.messages,
      },
      messageTemplates: {
        ...currentResultCardTemplates,
        ...patch.resultCard.messageTemplates,
      },
    },
  }
}

for (const [localeKey, patch] of Object.entries(
  browserScreenshotLegacyResultCardBackfills
) as Array<
  [
    LocaleKey,
    {
      resultCard: {
        messages: {
          screenshot_captured_for_active_tab: string
        }
        messageTemplates: {
          screenshot_captured_for_tab: string
        }
      }
    },
  ]
>) {
  const current = (mutableLocaleStructuralBackfills[localeKey] ?? {}) as Record<string, unknown>
  const currentResultCard = (current.resultCard ?? {}) as Record<string, unknown>
  const currentResultCardMessages = (currentResultCard.messages ?? {}) as Record<string, unknown>
  const currentResultCardTemplates = (currentResultCard.messageTemplates ?? {}) as Record<
    string,
    unknown
  >

  mutableLocaleStructuralBackfills[localeKey] = {
    ...current,
    resultCard: {
      ...currentResultCard,
      messages: {
        ...currentResultCardMessages,
        ...patch.resultCard.messages,
      },
      messageTemplates: {
        ...currentResultCardTemplates,
        ...patch.resultCard.messageTemplates,
      },
    },
  }
}

for (const [localeKey, patch] of Object.entries(browserLegacyResultCardBackfills) as Array<
  [
    LocaleKey,
    {
      resultCard: {
        messages: {
          browser_interactive_elements_section: string
          browser_page_structure_section: string
          browser_scroll_direction_down: string
          browser_scroll_direction_left: string
          browser_scroll_direction_right: string
          browser_scroll_direction_up: string
        }
        messageTemplates: {
          browser_action_performed_on_ref: string
          browser_large_dom_note: string
          browser_open_tabs: string
          browser_page: string
          browser_page_main_content: string
          browser_page_main_content_with_section: string
          browser_page_with_interactive_count: string
          browser_page_with_screenshot_interactive_count: string
          browser_recipes_available: string
          browser_scrolled_page: string
          browser_tab_closed: string
        }
      }
    },
  ]
>) {
  const current = (mutableLocaleStructuralBackfills[localeKey] ?? {}) as Record<string, unknown>
  const currentResultCard = (current.resultCard ?? {}) as Record<string, unknown>
  const currentResultCardMessages = (currentResultCard.messages ?? {}) as Record<string, unknown>
  const currentResultCardTemplates = (currentResultCard.messageTemplates ?? {}) as Record<
    string,
    unknown
  >

  mutableLocaleStructuralBackfills[localeKey] = {
    ...current,
    resultCard: {
      ...currentResultCard,
      messages: {
        ...currentResultCardMessages,
        ...patch.resultCard.messages,
      },
      messageTemplates: {
        ...currentResultCardTemplates,
        ...patch.resultCard.messageTemplates,
      },
    },
  }
}

for (const [localeKey, patch] of Object.entries(a11yLocaleTerms) as Array<
  [LocaleKey, { name: string; description: string }]
>) {
  const current = (mutableLocaleStructuralBackfills[localeKey] ?? {}) as Record<string, unknown>
  const currentTools = (current.tools ?? {}) as Record<string, unknown>
  const currentToolNames = (currentTools.names ?? {}) as Record<string, unknown>
  const currentToolDescriptions = (currentTools.descriptions ?? {}) as Record<string, unknown>

  mutableLocaleStructuralBackfills[localeKey] = {
    ...current,
    tools: {
      ...currentTools,
      names: {
        ...currentToolNames,
        computer_use: patch.name,
      },
      descriptions: {
        ...currentToolDescriptions,
        computer_use: patch.description,
      },
    },
  }
}

export default localeStructuralBackfills
