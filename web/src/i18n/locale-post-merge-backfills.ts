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
  omitted_results: string
  selected_result: string
  key_facts: string
  research_artifact_path: string
  llm_compacted: string
  materialized: string
  mode: string
  search_card_emitted: string
}

type ResultCardFieldLocalePatch = {
  labels: {
    async: string
    download_url: string
    host_os: string
    image_path: string
    output_path: string
    output_ref: string
    rank: string
    results: string
    route: string
    selected: string
    selected_source_rank: string
    target_format: string
    total_count: string
    warning_count: string
    window_id: string
  }
  messages: {
    host_screenshot_captured: string
    host_windows_listed: string
  }
}

type ResultCardHostActionLabelLocalePatch = {
  fallbacks: string
  input_method: string
  target_hit: string
  verification_method: string
  verification_passed: string
}

type ResultCardHostActionTelemetryLabelLocalePatch = {
  action_ms: string
  cache_hit: string
  candidate_count: string
  end_to_end_ms: string
  intent: string
  node_count: string
  snapshot_revision: string
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

export const workspaceNavigationLocaleBackfills: Record<
  LocaleKey,
  { coreTab: string; generatedTab: string }
> = {
  'ca-ES': {
    coreTab: 'Fitxers de context principal',
    generatedTab: "Fitxers de l'espai de treball",
  },
  'cs-CZ': {
    coreTab: 'Soubory hlavního kontextu',
    generatedTab: 'Soubory pracovního prostoru',
  },
  'da-DK': {
    coreTab: 'Kernekontekstfiler',
    generatedTab: 'Arbejdsområdefiler',
  },
  'de-DE': {
    coreTab: 'Kernkontextdateien',
    generatedTab: 'Arbeitsbereichsdateien',
  },
  'el-GR': {
    coreTab: 'Αρχεία βασικού πλαισίου',
    generatedTab: 'Αρχεία χώρου εργασίας',
  },
  'en-GB': {
    coreTab: 'Core Context Files',
    generatedTab: 'Workspace Files',
  },
  'en-US': {
    coreTab: 'Core Context Files',
    generatedTab: 'Workspace Files',
  },
  'es-ES': {
    coreTab: 'Archivos de contexto principal',
    generatedTab: 'Archivos del espacio de trabajo',
  },
  'fr-FR': {
    coreTab: 'Fichiers de contexte principal',
    generatedTab: "Fichiers de l'espace de travail",
  },
  'ga-IE': {
    coreTab: 'Croíchomhaid chomhthéacs',
    generatedTab: 'Comhaid spáis oibre',
  },
  'hr-HR': {
    coreTab: 'Datoteke glavnog konteksta',
    generatedTab: 'Datoteke radnog prostora',
  },
  'hu-HU': {
    coreTab: 'Alap kontextusfájlok',
    generatedTab: 'Munkaterületfájlok',
  },
  'it-IT': {
    coreTab: 'File del contesto principale',
    generatedTab: "File dell'area di lavoro",
  },
  'ja-JP': {
    coreTab: 'コアコンテキストファイル',
    generatedTab: 'ワークスペースファイル',
  },
  'ko-KR': {
    coreTab: '핵심 컨텍스트 파일',
    generatedTab: '작업공간 파일',
  },
  'ml-IN': {
    coreTab: 'കോർ കോൺടെക്സ്റ്റ് ഫയലുകൾ',
    generatedTab: 'വർക്ക്‌സ്‌പെയ്‌സ് ഫയലുകൾ',
  },
  'nb-NO': {
    coreTab: 'Kjernekontekstfiler',
    generatedTab: 'Arbeidsområdefiler',
  },
  'nl-NL': {
    coreTab: 'Kerncontextbestanden',
    generatedTab: 'Werkruimtebestanden',
  },
  'pl-PL': {
    coreTab: 'Pliki głównego kontekstu',
    generatedTab: 'Pliki obszaru roboczego',
  },
  'pt-BR': {
    coreTab: 'Arquivos de contexto principal',
    generatedTab: 'Arquivos do espaço de trabalho',
  },
  'pt-PT': {
    coreTab: 'Ficheiros de contexto principal',
    generatedTab: 'Ficheiros do espaço de trabalho',
  },
  'ro-RO': {
    coreTab: 'Fișiere de context principal',
    generatedTab: 'Fișierele spațiului de lucru',
  },
  'ru-RU': {
    coreTab: 'Файлы основного контекста',
    generatedTab: 'Файлы рабочей области',
  },
  'sk-SK': {
    coreTab: 'Súbory hlavného kontextu',
    generatedTab: 'Súbory pracovného priestoru',
  },
  'sv-SE': {
    coreTab: 'Kärnkontextfiler',
    generatedTab: 'Arbetsytefiler',
  },
  'zh-CN': {
    coreTab: '核心上下文文件',
    generatedTab: '工作区文件',
  },
  'zh-TW': {
    coreTab: '核心上下文檔案',
    generatedTab: '工作區檔案',
  },
}

type AskResultCardLocalePatch = {
  title: string
  question: string
  options: string
  answer: string
}

const askResultCardBackfills: Record<LocaleKey, AskResultCardLocalePatch> = {
  'ca-ES': { title: 'Pregunta', question: 'Pregunta', options: 'Opcions', answer: 'Resposta' },
  'cs-CZ': { title: 'Otázka', question: 'Otázka', options: 'Možnosti', answer: 'Odpověď' },
  'da-DK': {
    title: 'Spørgsmål',
    question: 'Spørgsmål',
    options: 'Valgmuligheder',
    answer: 'Svar',
  },
  'de-DE': { title: 'Frage', question: 'Frage', options: 'Optionen', answer: 'Antwort' },
  'el-GR': {
    title: 'Ερώτηση',
    question: 'Ερώτηση',
    options: 'Επιλογές',
    answer: 'Απάντηση',
  },
  'en-GB': { title: 'Question', question: 'Question', options: 'Options', answer: 'Answer' },
  'en-US': { title: 'Question', question: 'Question', options: 'Options', answer: 'Answer' },
  'es-ES': { title: 'Pregunta', question: 'Pregunta', options: 'Opciones', answer: 'Respuesta' },
  'fr-FR': { title: 'Question', question: 'Question', options: 'Options', answer: 'Réponse' },
  'ga-IE': { title: 'Ceist', question: 'Ceist', options: 'Roghanna', answer: 'Freagra' },
  'hr-HR': { title: 'Pitanje', question: 'Pitanje', options: 'Opcije', answer: 'Odgovor' },
  'hu-HU': { title: 'Kérdés', question: 'Kérdés', options: 'Lehetőségek', answer: 'Válasz' },
  'it-IT': { title: 'Domanda', question: 'Domanda', options: 'Opzioni', answer: 'Risposta' },
  'ja-JP': { title: '質問', question: '質問', options: '選択肢', answer: '回答' },
  'ko-KR': { title: '질문', question: '질문', options: '옵션', answer: '답변' },
  'ml-IN': { title: 'ചോദ്യം', question: 'ചോദ്യം', options: 'ഓപ്ഷനുകൾ', answer: 'ഉത്തരം' },
  'nb-NO': { title: 'Spørsmål', question: 'Spørsmål', options: 'Alternativer', answer: 'Svar' },
  'nl-NL': { title: 'Vraag', question: 'Vraag', options: 'Opties', answer: 'Antwoord' },
  'pl-PL': { title: 'Pytanie', question: 'Pytanie', options: 'Opcje', answer: 'Odpowiedź' },
  'pt-BR': { title: 'Pergunta', question: 'Pergunta', options: 'Opções', answer: 'Resposta' },
  'pt-PT': { title: 'Pergunta', question: 'Pergunta', options: 'Opções', answer: 'Resposta' },
  'ro-RO': { title: 'Întrebare', question: 'Întrebare', options: 'Opțiuni', answer: 'Răspuns' },
  'ru-RU': { title: 'Вопрос', question: 'Вопрос', options: 'Варианты', answer: 'Ответ' },
  'sk-SK': { title: 'Otázka', question: 'Otázka', options: 'Možnosti', answer: 'Odpoveď' },
  'sv-SE': { title: 'Fråga', question: 'Fråga', options: 'Alternativ', answer: 'Svar' },
  'zh-CN': { title: '提问', question: '问题', options: '选项', answer: '回答' },
  'zh-TW': { title: '提問', question: '問題', options: '選項', answer: '回答' },
}

const resultCardFieldBackfills: Record<LocaleKey, ResultCardFieldLocalePatch> = {
  'ca-ES': {
    labels: {
      async: 'Assíncron',
      download_url: 'URL de descàrrega',
      host_os: "SO de l'amfitrió",
      image_path: 'Camí de la imatge',
      output_path: 'Camí de sortida',
      output_ref: 'Referència de sortida',
      rank: 'Rànquing',
      results: 'Resultats',
      route: 'Ruta',
      selected: 'Seleccionat',
      selected_source_rank: 'Rànquing de la font seleccionada',
      target_format: 'Format de destinació',
      total_count: 'Recompte total',
      warning_count: "Recompte d'avisos",
      window_id: 'ID de la finestra',
    },
    messages: {
      host_screenshot_captured: "S'ha capturat una pantalla de l'amfitrió",
      host_windows_listed: "S'han llistat les finestres de l'amfitrió",
    },
  },
  'cs-CZ': {
    labels: {
      async: 'Asynchronní',
      download_url: 'URL ke stažení',
      host_os: 'Hostitelský OS',
      image_path: 'Cesta k obrázku',
      output_path: 'Výstupní cesta',
      output_ref: 'Odkaz na výstup',
      rank: 'Pořadí',
      results: 'Výsledky',
      route: 'Trasa',
      selected: 'Vybráno',
      selected_source_rank: 'Pořadí vybraného zdroje',
      target_format: 'Cílový formát',
      total_count: 'Celkový počet',
      warning_count: 'Počet varování',
      window_id: 'ID okna',
    },
    messages: {
      host_screenshot_captured: 'Snímek obrazovky hostitele byl pořízen',
      host_windows_listed: 'Okna hostitele byla vypsána',
    },
  },
  'da-DK': {
    labels: {
      async: 'Asynkron',
      download_url: 'Download-URL',
      host_os: 'Værts-OS',
      image_path: 'Billedsti',
      output_path: 'Outputsti',
      output_ref: 'Outputreference',
      rank: 'Rang',
      results: 'Resultater',
      route: 'Rute',
      selected: 'Valgt',
      selected_source_rank: 'Rang for valgt kilde',
      target_format: 'Målformat',
      total_count: 'Samlet antal',
      warning_count: 'Antal advarsler',
      window_id: 'Vindues-ID',
    },
    messages: {
      host_screenshot_captured: 'Værtsskærmbillede optaget',
      host_windows_listed: 'Værtsvinduer listet',
    },
  },
  'de-DE': {
    labels: {
      async: 'Asynchron',
      download_url: 'Download-URL',
      host_os: 'Host-Betriebssystem',
      image_path: 'Bildpfad',
      output_path: 'Ausgabepfad',
      output_ref: 'Ausgabereferenz',
      rank: 'Rang',
      results: 'Ergebnisse',
      route: 'Route',
      selected: 'Ausgewählt',
      selected_source_rank: 'Rang der ausgewählten Quelle',
      target_format: 'Zielformat',
      total_count: 'Gesamtzahl',
      warning_count: 'Anzahl Warnungen',
      window_id: 'Fenster-ID',
    },
    messages: {
      host_screenshot_captured: 'Host-Screenshot erfasst',
      host_windows_listed: 'Host-Fenster aufgelistet',
    },
  },
  'el-GR': {
    labels: {
      async: 'Ασύγχρονο',
      download_url: 'URL λήψης',
      host_os: 'ΛΣ κεντρικού υπολογιστή',
      image_path: 'Διαδρομή εικόνας',
      output_path: 'Διαδρομή εξόδου',
      output_ref: 'Αναφορά εξόδου',
      rank: 'Κατάταξη',
      results: 'Αποτελέσματα',
      route: 'Διαδρομή',
      selected: 'Επιλεγμένο',
      selected_source_rank: 'Κατάταξη επιλεγμένης πηγής',
      target_format: 'Μορφή προορισμού',
      total_count: 'Συνολικός αριθμός',
      warning_count: 'Αριθμός προειδοποιήσεων',
      window_id: 'ID παραθύρου',
    },
    messages: {
      host_screenshot_captured: 'Λήφθηκε στιγμιότυπο οθόνης υπολογιστή',
      host_windows_listed: 'Καταγράφηκαν τα παράθυρα του υπολογιστή',
    },
  },
  'en-GB': {
    labels: {
      async: 'Async',
      download_url: 'Download URL',
      host_os: 'Host OS',
      image_path: 'Image Path',
      output_path: 'Output Path',
      output_ref: 'Output Reference',
      rank: 'Rank',
      results: 'Results',
      route: 'Route',
      selected: 'Selected',
      selected_source_rank: 'Selected Source Rank',
      target_format: 'Target Format',
      total_count: 'Total Count',
      warning_count: 'Warning Count',
      window_id: 'Window ID',
    },
    messages: {
      host_screenshot_captured: 'Host screenshot captured',
      host_windows_listed: 'Host windows listed',
    },
  },
  'en-US': {
    labels: {
      async: 'Async',
      download_url: 'Download URL',
      host_os: 'Host OS',
      image_path: 'Image Path',
      output_path: 'Output Path',
      output_ref: 'Output Reference',
      rank: 'Rank',
      results: 'Results',
      route: 'Route',
      selected: 'Selected',
      selected_source_rank: 'Selected Source Rank',
      target_format: 'Target Format',
      total_count: 'Total Count',
      warning_count: 'Warning Count',
      window_id: 'Window ID',
    },
    messages: {
      host_screenshot_captured: 'Host screenshot captured',
      host_windows_listed: 'Host windows listed',
    },
  },
  'es-ES': {
    labels: {
      async: 'Asíncrono',
      download_url: 'URL de descarga',
      host_os: 'SO del host',
      image_path: 'Ruta de imagen',
      output_path: 'Ruta de salida',
      output_ref: 'Referencia de salida',
      rank: 'Rango',
      results: 'Resultados',
      route: 'Ruta',
      selected: 'Seleccionado',
      selected_source_rank: 'Rango de la fuente seleccionada',
      target_format: 'Formato de destino',
      total_count: 'Conteo total',
      warning_count: 'Conteo de advertencias',
      window_id: 'ID de ventana',
    },
    messages: {
      host_screenshot_captured: 'Captura de pantalla del host tomada',
      host_windows_listed: 'Ventanas del host listadas',
    },
  },
  'fr-FR': {
    labels: {
      async: 'Asynchrone',
      download_url: 'URL de téléchargement',
      host_os: 'OS hôte',
      image_path: "Chemin de l'image",
      output_path: 'Chemin de sortie',
      output_ref: 'Référence de sortie',
      rank: 'Rang',
      results: 'Résultats',
      route: 'Route',
      selected: 'Sélectionné',
      selected_source_rank: 'Rang de la source sélectionnée',
      target_format: 'Format cible',
      total_count: 'Nombre total',
      warning_count: "Nombre d'avertissements",
      window_id: 'ID de fenêtre',
    },
    messages: {
      host_screenshot_captured: "Capture d'écran hôte effectuée",
      host_windows_listed: 'Fenêtres hôtes listées',
    },
  },
  'ga-IE': {
    labels: {
      async: 'Asioncronach',
      download_url: 'URL íoslódála',
      host_os: 'Córas oibriúcháin an ósta',
      image_path: 'Conair íomhá',
      output_path: 'Conair aschuir',
      output_ref: 'Tagairt aschuir',
      rank: 'Rang',
      results: 'Torthaí',
      route: 'Bealach',
      selected: 'Roghnaithe',
      selected_source_rank: 'Rang na foinse roghnaithe',
      target_format: 'Formáid sprice',
      total_count: 'Comhaireamh iomlán',
      warning_count: 'Comhaireamh rabhaidh',
      window_id: 'Aitheantas fuinneoige',
    },
    messages: {
      host_screenshot_captured: 'Gabhadh seat scáileáin den óstach',
      host_windows_listed: 'Liostaíodh fuinneoga an óstaigh',
    },
  },
  'hr-HR': {
    labels: {
      async: 'Asinkrono',
      download_url: 'URL za preuzimanje',
      host_os: 'Operativni sustav hosta',
      image_path: 'Putanja slike',
      output_path: 'Izlazna putanja',
      output_ref: 'Referenca izlaza',
      rank: 'Poredak',
      results: 'Rezultati',
      route: 'Ruta',
      selected: 'Odabrano',
      selected_source_rank: 'Poredak odabranog izvora',
      target_format: 'Ciljani format',
      total_count: 'Ukupan broj',
      warning_count: 'Broj upozorenja',
      window_id: 'ID prozora',
    },
    messages: {
      host_screenshot_captured: 'Snimka zaslona hosta je zabilježena',
      host_windows_listed: 'Prozori hosta su izlistani',
    },
  },
  'hu-HU': {
    labels: {
      async: 'Aszinkron',
      download_url: 'Letöltési URL',
      host_os: 'Gazda operációs rendszer',
      image_path: 'Kép elérési útja',
      output_path: 'Kimeneti útvonal',
      output_ref: 'Kimeneti hivatkozás',
      rank: 'Rang',
      results: 'Eredmények',
      route: 'Útvonal',
      selected: 'Kiválasztva',
      selected_source_rank: 'Kiválasztott forrás rangja',
      target_format: 'Célformátum',
      total_count: 'Teljes darabszám',
      warning_count: 'Figyelmeztetések száma',
      window_id: 'Ablakazonosító',
    },
    messages: {
      host_screenshot_captured: 'Gazdagép képernyőképe elkészült',
      host_windows_listed: 'Gazdagép ablakai listázva',
    },
  },
  'it-IT': {
    labels: {
      async: 'Asincrono',
      download_url: 'URL di download',
      host_os: 'SO host',
      image_path: 'Percorso immagine',
      output_path: 'Percorso di output',
      output_ref: 'Riferimento output',
      rank: 'Posizione',
      results: 'Risultati',
      route: 'Percorso',
      selected: 'Selezionato',
      selected_source_rank: 'Posizione della fonte selezionata',
      target_format: 'Formato di destinazione',
      total_count: 'Conteggio totale',
      warning_count: 'Conteggio avvisi',
      window_id: 'ID finestra',
    },
    messages: {
      host_screenshot_captured: 'Screenshot host acquisito',
      host_windows_listed: 'Finestre host elencate',
    },
  },
  'ja-JP': {
    labels: {
      async: '非同期',
      download_url: 'ダウンロード URL',
      host_os: 'ホスト OS',
      image_path: '画像パス',
      output_path: '出力パス',
      output_ref: '出力参照',
      rank: '順位',
      results: '結果',
      route: 'ルート',
      selected: '選択済み',
      selected_source_rank: '選択したソースの順位',
      target_format: '対象フォーマット',
      total_count: '合計数',
      warning_count: '警告数',
      window_id: 'ウィンドウ ID',
    },
    messages: {
      host_screenshot_captured: 'ホストのスクリーンショットを取得しました',
      host_windows_listed: 'ホストウィンドウを一覧表示しました',
    },
  },
  'ko-KR': {
    labels: {
      async: '비동기',
      download_url: '다운로드 URL',
      host_os: '호스트 OS',
      image_path: '이미지 경로',
      output_path: '출력 경로',
      output_ref: '출력 참조',
      rank: '순위',
      results: '결과',
      route: '라우트',
      selected: '선택됨',
      selected_source_rank: '선택한 소스 순위',
      target_format: '대상 형식',
      total_count: '총 개수',
      warning_count: '경고 수',
      window_id: '창 ID',
    },
    messages: {
      host_screenshot_captured: '호스트 스크린샷을 캡처했습니다',
      host_windows_listed: '호스트 창을 나열했습니다',
    },
  },
  'ml-IN': {
    labels: {
      async: 'അസിങ്ക്',
      download_url: 'ഡൗൺലോഡ് URL',
      host_os: 'ഹോസ്റ്റ് ഓ.എസ്.',
      image_path: 'ചിത്ര പാത',
      output_path: 'ഔട്ട്പുട്ട് പാത',
      output_ref: 'ഔട്ട്പുട്ട് റഫറൻസ്',
      rank: 'റാങ്ക്',
      results: 'ഫലങ്ങൾ',
      route: 'റൂട്ട്',
      selected: 'തിരഞ്ഞെടുത്തത്',
      selected_source_rank: 'തിരഞ്ഞെടുത്ത ഉറവിട റാങ്ക്',
      target_format: 'ലക്ഷ്യ ഫോർമാറ്റ്',
      total_count: 'മൊത്തം എണ്ണം',
      warning_count: 'മുന്നറിയിപ്പ് എണ്ണം',
      window_id: 'വിൻഡോ ഐഡി',
    },
    messages: {
      host_screenshot_captured: 'ഹോസ്റ്റ് സ്ക്രീൻഷോട്ട് എടുത്തു',
      host_windows_listed: 'ഹോസ്റ്റ് വിൻഡോകൾ പട്ടികപ്പെടുത്തി',
    },
  },
  'nb-NO': {
    labels: {
      async: 'Asynkron',
      download_url: 'Nedlastings-URL',
      host_os: 'Verts-OS',
      image_path: 'Bildebane',
      output_path: 'Utdata-bane',
      output_ref: 'Utdatareferanse',
      rank: 'Rang',
      results: 'Resultater',
      route: 'Rute',
      selected: 'Valgt',
      selected_source_rank: 'Rang for valgt kilde',
      target_format: 'Målformat',
      total_count: 'Totalt antall',
      warning_count: 'Antall advarsler',
      window_id: 'Vindus-ID',
    },
    messages: {
      host_screenshot_captured: 'Vertsskjermbilde tatt',
      host_windows_listed: 'Vertsvinduer listet',
    },
  },
  'nl-NL': {
    labels: {
      async: 'Asynchroon',
      download_url: 'Download-URL',
      host_os: 'Host-OS',
      image_path: 'Afbeeldingspad',
      output_path: 'Uitvoerpad',
      output_ref: 'Uitvoerreferentie',
      rank: 'Rang',
      results: 'Resultaten',
      route: 'Route',
      selected: 'Geselecteerd',
      selected_source_rank: 'Rang van geselecteerde bron',
      target_format: 'Doelformaat',
      total_count: 'Totaal aantal',
      warning_count: 'Aantal waarschuwingen',
      window_id: 'Venster-ID',
    },
    messages: {
      host_screenshot_captured: 'Hostscreenshot vastgelegd',
      host_windows_listed: 'Hostvensters weergegeven',
    },
  },
  'pl-PL': {
    labels: {
      async: 'Asynchroniczne',
      download_url: 'URL pobierania',
      host_os: 'System operacyjny hosta',
      image_path: 'Ścieżka obrazu',
      output_path: 'Ścieżka wyjściowa',
      output_ref: 'Odwołanie wyjściowe',
      rank: 'Ranking',
      results: 'Wyniki',
      route: 'Trasa',
      selected: 'Wybrane',
      selected_source_rank: 'Ranking wybranego źródła',
      target_format: 'Format docelowy',
      total_count: 'Łączna liczba',
      warning_count: 'Liczba ostrzeżeń',
      window_id: 'ID okna',
    },
    messages: {
      host_screenshot_captured: 'Zrzut ekranu hosta został wykonany',
      host_windows_listed: 'Okna hosta zostały wylistowane',
    },
  },
  'pt-BR': {
    labels: {
      async: 'Assíncrono',
      download_url: 'URL de download',
      host_os: 'SO do host',
      image_path: 'Caminho da imagem',
      output_path: 'Caminho de saída',
      output_ref: 'Referência de saída',
      rank: 'Classificação',
      results: 'Resultados',
      route: 'Rota',
      selected: 'Selecionado',
      selected_source_rank: 'Classificação da fonte selecionada',
      target_format: 'Formato de destino',
      total_count: 'Contagem total',
      warning_count: 'Contagem de avisos',
      window_id: 'ID da janela',
    },
    messages: {
      host_screenshot_captured: 'Captura de tela do host realizada',
      host_windows_listed: 'Janelas do host listadas',
    },
  },
  'pt-PT': {
    labels: {
      async: 'Assíncrono',
      download_url: 'URL de download',
      host_os: 'SO do anfitrião',
      image_path: 'Caminho da imagem',
      output_path: 'Caminho de saída',
      output_ref: 'Referência de saída',
      rank: 'Classificação',
      results: 'Resultados',
      route: 'Rota',
      selected: 'Selecionado',
      selected_source_rank: 'Classificação da fonte selecionada',
      target_format: 'Formato de destino',
      total_count: 'Contagem total',
      warning_count: 'Contagem de avisos',
      window_id: 'ID da janela',
    },
    messages: {
      host_screenshot_captured: 'Captura de ecrã do anfitrião efetuada',
      host_windows_listed: 'Janelas do anfitrião listadas',
    },
  },
  'ro-RO': {
    labels: {
      async: 'Asincron',
      download_url: 'URL descărcare',
      host_os: 'SO gazdă',
      image_path: 'Cale imagine',
      output_path: 'Cale ieșire',
      output_ref: 'Referință ieșire',
      rank: 'Rang',
      results: 'Rezultate',
      route: 'Rută',
      selected: 'Selectat',
      selected_source_rank: 'Rangul sursei selectate',
      target_format: 'Format țintă',
      total_count: 'Număr total',
      warning_count: 'Număr avertismente',
      window_id: 'ID fereastră',
    },
    messages: {
      host_screenshot_captured: 'Captură de ecran gazdă realizată',
      host_windows_listed: 'Ferestrele gazdei listate',
    },
  },
  'ru-RU': {
    labels: {
      async: 'Асинхронно',
      download_url: 'URL загрузки',
      host_os: 'ОС хоста',
      image_path: 'Путь к изображению',
      output_path: 'Путь вывода',
      output_ref: 'Ссылка на вывод',
      rank: 'Ранг',
      results: 'Результаты',
      route: 'Маршрут',
      selected: 'Выбрано',
      selected_source_rank: 'Ранг выбранного источника',
      target_format: 'Целевой формат',
      total_count: 'Общее количество',
      warning_count: 'Количество предупреждений',
      window_id: 'ID окна',
    },
    messages: {
      host_screenshot_captured: 'Снимок экрана хоста сохранен',
      host_windows_listed: 'Окна хоста перечислены',
    },
  },
  'sk-SK': {
    labels: {
      async: 'Asynchrónne',
      download_url: 'URL na stiahnutie',
      host_os: 'Operačný systém hostiteľa',
      image_path: 'Cesta k obrázku',
      output_path: 'Výstupná cesta',
      output_ref: 'Odkaz na výstup',
      rank: 'Poradie',
      results: 'Výsledky',
      route: 'Trasa',
      selected: 'Vybrané',
      selected_source_rank: 'Poradie vybraného zdroja',
      target_format: 'Cieľový formát',
      total_count: 'Celkový počet',
      warning_count: 'Počet upozornení',
      window_id: 'ID okna',
    },
    messages: {
      host_screenshot_captured: 'Snímka obrazovky hostiteľa bola zachytená',
      host_windows_listed: 'Okná hostiteľa boli vypísané',
    },
  },
  'sv-SE': {
    labels: {
      async: 'Asynkront',
      download_url: 'Nedladdnings-URL',
      host_os: 'Värd-OS',
      image_path: 'Bildväg',
      output_path: 'Utmatningsväg',
      output_ref: 'Utreferens',
      rank: 'Rang',
      results: 'Resultat',
      route: 'Rutt',
      selected: 'Vald',
      selected_source_rank: 'Rang för vald källa',
      target_format: 'Målformat',
      total_count: 'Totalt antal',
      warning_count: 'Antal varningar',
      window_id: 'Fönster-ID',
    },
    messages: {
      host_screenshot_captured: 'Värdskärmbild tagen',
      host_windows_listed: 'Värdfönster listade',
    },
  },
  'zh-CN': {
    labels: {
      async: '异步',
      download_url: '下载 URL',
      host_os: '主机系统',
      image_path: '图像路径',
      output_path: '输出路径',
      output_ref: '输出引用',
      rank: '排名',
      results: '结果',
      route: '路由',
      selected: '已选中',
      selected_source_rank: '选中来源排名',
      target_format: '目标格式',
      total_count: '总数量',
      warning_count: '警告数量',
      window_id: '窗口 ID',
    },
    messages: {
      host_screenshot_captured: '已捕获主机截图',
      host_windows_listed: '已列出主机窗口',
    },
  },
  'zh-TW': {
    labels: {
      async: '異步',
      download_url: '下載 URL',
      host_os: '主機系統',
      image_path: '影像路徑',
      output_path: '輸出路徑',
      output_ref: '輸出參考',
      rank: '排名',
      results: '結果',
      route: '路由',
      selected: '已選取',
      selected_source_rank: '已選取來源排名',
      target_format: '目標格式',
      total_count: '總數量',
      warning_count: '警告數量',
      window_id: '視窗 ID',
    },
    messages: {
      host_screenshot_captured: '已擷取主機畫面',
      host_windows_listed: '已列出主機視窗',
    },
  },
}

const resultCardExecutionModeLabels: Record<LocaleKey, string> = {
  'ca-ES': "Mode d'execucio",
  'cs-CZ': 'Režim spuštění',
  'da-DK': 'Udførelsestilstand',
  'de-DE': 'Ausführungsmodus',
  'el-GR': 'Λειτουργία εκτέλεσης',
  'en-GB': 'Execution Mode',
  'en-US': 'Execution Mode',
  'es-ES': 'Modo de ejecución',
  'fr-FR': "Mode d'execution",
  'ga-IE': 'Modh forghníomhaithe',
  'hr-HR': 'Način izvršavanja',
  'hu-HU': 'Végrehajtási mód',
  'it-IT': 'Modalità di esecuzione',
  'ja-JP': '実行モード',
  'ko-KR': '실행 모드',
  'ml-IN': 'നിർവഹണ മോഡ്',
  'nb-NO': 'Kjøringsmodus',
  'nl-NL': 'Uitvoermodus',
  'pl-PL': 'Tryb wykonania',
  'pt-BR': 'Modo de execução',
  'pt-PT': 'Modo de execução',
  'ro-RO': 'Mod de execuție',
  'ru-RU': 'Режим выполнения',
  'sk-SK': 'Režim vykonania',
  'sv-SE': 'Körläge',
  'zh-CN': '执行模式',
  'zh-TW': '執行模式',
}

const resultCardWindowFocusedMessages: Record<LocaleKey, string> = {
  'ca-ES': 'Finestra enfocada',
  'cs-CZ': 'Okno zaměřeno',
  'da-DK': 'Vindue fokuseret',
  'de-DE': 'Fenster fokussiert',
  'el-GR': 'Το παράθυρο εστιάστηκε',
  'en-GB': 'Window focused',
  'en-US': 'Window focused',
  'es-ES': 'Ventana enfocada',
  'fr-FR': 'Fenetre focalisee',
  'ga-IE': 'Díríodh ar an bhfuinneog',
  'hr-HR': 'Prozor fokusiran',
  'hu-HU': 'Az ablak fókuszálva',
  'it-IT': 'Finestra focalizzata',
  'ja-JP': 'ウィンドウにフォーカスしました',
  'ko-KR': '창에 포커스를 맞췄습니다',
  'ml-IN': 'വിൻഡോ ഫോകസ് ചെയ്തു',
  'nb-NO': 'Vindu fokusert',
  'nl-NL': 'Venster gefocust',
  'pl-PL': 'Okno aktywne',
  'pt-BR': 'Janela focada',
  'pt-PT': 'Janela focada',
  'ro-RO': 'Fereastră focalizată',
  'ru-RU': 'Окно в фокусе',
  'sk-SK': 'Okno zaostrené',
  'sv-SE': 'Fönster fokuserat',
  'zh-CN': '窗口已聚焦',
  'zh-TW': '視窗已聚焦',
}

const resultCardHostActionLabelBackfills: Record<LocaleKey, ResultCardHostActionLabelLocalePatch> =
  {
    'ca-ES': {
      fallbacks: 'Alternatives',
      input_method: "Metode d'entrada",
      target_hit: 'Objectiu encertat',
      verification_method: 'Mètode de verificació',
      verification_passed: 'Verificació superada',
    },
    'cs-CZ': {
      fallbacks: 'Záložní postupy',
      input_method: 'Metoda vstupu',
      target_hit: 'Cíl zasažen',
      verification_method: 'Metoda ověření',
      verification_passed: 'Ověření úspěšné',
    },
    'da-DK': {
      fallbacks: 'Reserveveje',
      input_method: 'Inputmetode',
      target_hit: 'Mål ramt',
      verification_method: 'Verifikationsmetode',
      verification_passed: 'Verifikation bestået',
    },
    'de-DE': {
      fallbacks: 'Ausweichpfade',
      input_method: 'Eingabemethode',
      target_hit: 'Ziel getroffen',
      verification_method: 'Verifikationsmethode',
      verification_passed: 'Verifikation bestanden',
    },
    'el-GR': {
      fallbacks: 'Εναλλακτικές διαδρομές',
      input_method: 'Μέθοδος εισαγωγής',
      target_hit: 'Ο στόχος επιτεύχθηκε',
      verification_method: 'Μέθοδος επαλήθευσης',
      verification_passed: 'Η επαλήθευση πέτυχε',
    },
    'en-GB': {
      fallbacks: 'Fallbacks',
      input_method: 'Input Method',
      target_hit: 'Target Hit',
      verification_method: 'Verification Method',
      verification_passed: 'Verification Passed',
    },
    'en-US': {
      fallbacks: 'Fallbacks',
      input_method: 'Input Method',
      target_hit: 'Target Hit',
      verification_method: 'Verification Method',
      verification_passed: 'Verification Passed',
    },
    'es-ES': {
      fallbacks: 'Alternativas',
      input_method: 'Método de entrada',
      target_hit: 'Objetivo acertado',
      verification_method: 'Método de verificación',
      verification_passed: 'Verificación superada',
    },
    'fr-FR': {
      fallbacks: 'Solutions de repli',
      input_method: "Méthode d'entrée",
      target_hit: 'Cible atteinte',
      verification_method: 'Méthode de vérification',
      verification_passed: 'Vérification réussie',
    },
    'ga-IE': {
      fallbacks: 'Aisfhillte',
      input_method: 'Modh ionchuir',
      target_hit: 'Sprioc buailte',
      verification_method: 'Modh fíoraithe',
      verification_passed: 'Fíorú rite',
    },
    'hr-HR': {
      fallbacks: 'Rezervni koraci',
      input_method: 'Način unosa',
      target_hit: 'Cilj pogođen',
      verification_method: 'Metoda provjere',
      verification_passed: 'Provjera uspješna',
    },
    'hu-HU': {
      fallbacks: 'Tartalék lépések',
      input_method: 'Beviteli mód',
      target_hit: 'Cél eltalálva',
      verification_method: 'Ellenőrzési mód',
      verification_passed: 'Ellenőrzés sikeres',
    },
    'it-IT': {
      fallbacks: 'Fallback',
      input_method: 'Metodo di input',
      target_hit: 'Obiettivo colpito',
      verification_method: 'Metodo di verifica',
      verification_passed: 'Verifica superata',
    },
    'ja-JP': {
      fallbacks: 'フォールバック',
      input_method: '入力方法',
      target_hit: '対象命中',
      verification_method: '検証方法',
      verification_passed: '検証通過',
    },
    'ko-KR': {
      fallbacks: '대체 경로',
      input_method: '입력 방식',
      target_hit: '대상 적중',
      verification_method: '검증 방식',
      verification_passed: '검증 통과',
    },
    'ml-IN': {
      fallbacks: 'ഫോൾബാക്കുകൾ',
      input_method: 'ഇൻപുട്ട് രീതി',
      target_hit: 'ലക്ഷ്യം തൊട്ടു',
      verification_method: 'സ്ഥിരീകരണ രീതി',
      verification_passed: 'സ്ഥിരീകരണം വിജയിച്ചു',
    },
    'nb-NO': {
      fallbacks: 'Reservevalg',
      input_method: 'Inndatametode',
      target_hit: 'Mål truffet',
      verification_method: 'Verifiseringsmetode',
      verification_passed: 'Verifisering bestått',
    },
    'nl-NL': {
      fallbacks: 'Terugvalopties',
      input_method: 'Invoermethode',
      target_hit: 'Doel geraakt',
      verification_method: 'Verificatiemethode',
      verification_passed: 'Verificatie geslaagd',
    },
    'pl-PL': {
      fallbacks: 'Ścieżki awaryjne',
      input_method: 'Metoda wprowadzania',
      target_hit: 'Cel trafiony',
      verification_method: 'Metoda weryfikacji',
      verification_passed: 'Weryfikacja zakończona powodzeniem',
    },
    'pt-BR': {
      fallbacks: 'Alternativas',
      input_method: 'Método de entrada',
      target_hit: 'Alvo atingido',
      verification_method: 'Método de verificação',
      verification_passed: 'Verificação aprovada',
    },
    'pt-PT': {
      fallbacks: 'Alternativas',
      input_method: 'Método de entrada',
      target_hit: 'Alvo atingido',
      verification_method: 'Método de verificação',
      verification_passed: 'Verificação aprovada',
    },
    'ro-RO': {
      fallbacks: 'Rezerve',
      input_method: 'Metodă de introducere',
      target_hit: 'Țintă atinsă',
      verification_method: 'Metodă de verificare',
      verification_passed: 'Verificare reușită',
    },
    'ru-RU': {
      fallbacks: 'Резервные пути',
      input_method: 'Способ ввода',
      target_hit: 'Цель достигнута',
      verification_method: 'Метод проверки',
      verification_passed: 'Проверка пройдена',
    },
    'sk-SK': {
      fallbacks: 'Záložné postupy',
      input_method: 'Spôsob vstupu',
      target_hit: 'Cieľ zasiahnutý',
      verification_method: 'Metóda overenia',
      verification_passed: 'Overenie úspešné',
    },
    'sv-SE': {
      fallbacks: 'Reservvägar',
      input_method: 'Inmatningsmetod',
      target_hit: 'Mål träffat',
      verification_method: 'Verifieringsmetod',
      verification_passed: 'Verifiering godkänd',
    },
    'zh-CN': {
      fallbacks: '回退方式',
      input_method: '输入方式',
      target_hit: '目标命中',
      verification_method: '验证方式',
      verification_passed: '验证通过',
    },
    'zh-TW': {
      fallbacks: '回退方式',
      input_method: '輸入方式',
      target_hit: '目標命中',
      verification_method: '驗證方式',
      verification_passed: '驗證通過',
    },
  }

const resultCardHostActionTelemetryLabelBackfills: Record<
  LocaleKey,
  ResultCardHostActionTelemetryLabelLocalePatch
> = {
  'ca-ES': {
    action_ms: "Temps d'acció (ms)",
    cache_hit: 'Encert de memòria cau',
    candidate_count: 'Nombre de candidats',
    end_to_end_ms: 'Temps de punta a punta (ms)',
    intent: 'Intenció',
    node_count: 'Nombre de nodes',
    snapshot_revision: 'Revisió de la instantània',
  },
  'cs-CZ': {
    action_ms: 'Čas akce (ms)',
    cache_hit: 'Zásah mezipaměti',
    candidate_count: 'Počet kandidátů',
    end_to_end_ms: 'Čas od začátku do konce (ms)',
    intent: 'Záměr',
    node_count: 'Počet uzlů',
    snapshot_revision: 'Revize snímku',
  },
  'da-DK': {
    action_ms: 'Handlingstid (ms)',
    cache_hit: 'Cachetræf',
    candidate_count: 'Antal kandidater',
    end_to_end_ms: 'Samlet tid (ms)',
    intent: 'Hensigt',
    node_count: 'Antal noder',
    snapshot_revision: 'Snapshot-revision',
  },
  'de-DE': {
    action_ms: 'Aktionszeit (ms)',
    cache_hit: 'Cache-Treffer',
    candidate_count: 'Anzahl der Kandidaten',
    end_to_end_ms: 'Ende-zu-Ende-Zeit (ms)',
    intent: 'Absicht',
    node_count: 'Anzahl der Knoten',
    snapshot_revision: 'Snapshot-Revision',
  },
  'el-GR': {
    action_ms: 'Χρόνος ενέργειας (ms)',
    cache_hit: 'Επιτυχία cache',
    candidate_count: 'Πλήθος υποψηφίων',
    end_to_end_ms: 'Χρόνος από άκρο σε άκρο (ms)',
    intent: 'Πρόθεση',
    node_count: 'Πλήθος κόμβων',
    snapshot_revision: 'Αναθεώρηση στιγμιότυπου',
  },
  'en-GB': {
    action_ms: 'Action Time (ms)',
    cache_hit: 'Cache Hit',
    candidate_count: 'Candidate Count',
    end_to_end_ms: 'End-to-End Time (ms)',
    intent: 'Intent',
    node_count: 'Node Count',
    snapshot_revision: 'Snapshot Revision',
  },
  'en-US': {
    action_ms: 'Action Time (ms)',
    cache_hit: 'Cache Hit',
    candidate_count: 'Candidate Count',
    end_to_end_ms: 'End-to-End Time (ms)',
    intent: 'Intent',
    node_count: 'Node Count',
    snapshot_revision: 'Snapshot Revision',
  },
  'es-ES': {
    action_ms: 'Tiempo de acción (ms)',
    cache_hit: 'Acierto de caché',
    candidate_count: 'Número de candidatos',
    end_to_end_ms: 'Tiempo de extremo a extremo (ms)',
    intent: 'Intención',
    node_count: 'Número de nodos',
    snapshot_revision: 'Revisión de instantánea',
  },
  'fr-FR': {
    action_ms: "Temps d'action (ms)",
    cache_hit: 'Succès du cache',
    candidate_count: 'Nombre de candidats',
    end_to_end_ms: 'Temps de bout en bout (ms)',
    intent: 'Intention',
    node_count: 'Nombre de nœuds',
    snapshot_revision: "Révision de l'instantané",
  },
  'ga-IE': {
    action_ms: 'Am gnímh (ms)',
    cache_hit: 'Aimsiú taisce',
    candidate_count: 'Líon iarrthóirí',
    end_to_end_ms: 'Am deireadh go deireadh (ms)',
    intent: 'Rún',
    node_count: 'Líon na nóid',
    snapshot_revision: 'Athbhreithniú seatghraif',
  },
  'hr-HR': {
    action_ms: 'Vrijeme radnje (ms)',
    cache_hit: 'Pogodak predmemorije',
    candidate_count: 'Broj kandidata',
    end_to_end_ms: 'Vrijeme od početka do kraja (ms)',
    intent: 'Namjera',
    node_count: 'Broj čvorova',
    snapshot_revision: 'Revizija snimke',
  },
  'hu-HU': {
    action_ms: 'Műveleti idő (ms)',
    cache_hit: 'Gyorsítótár-találat',
    candidate_count: 'Jelöltek száma',
    end_to_end_ms: 'Teljes átfutási idő (ms)',
    intent: 'Szándék',
    node_count: 'Csomópontok száma',
    snapshot_revision: 'Pillanatkép-verzió',
  },
  'it-IT': {
    action_ms: 'Tempo di azione (ms)',
    cache_hit: 'Hit della cache',
    candidate_count: 'Numero di candidati',
    end_to_end_ms: 'Tempo end-to-end (ms)',
    intent: 'Intenzione',
    node_count: 'Numero di nodi',
    snapshot_revision: 'Revisione snapshot',
  },
  'ja-JP': {
    action_ms: '操作時間 (ms)',
    cache_hit: 'キャッシュヒット',
    candidate_count: '候補数',
    end_to_end_ms: 'エンドツーエンド時間 (ms)',
    intent: '意図',
    node_count: 'ノード数',
    snapshot_revision: 'スナップショット改訂',
  },
  'ko-KR': {
    action_ms: '작업 시간 (ms)',
    cache_hit: '캐시 적중',
    candidate_count: '후보 수',
    end_to_end_ms: '종단간 시간 (ms)',
    intent: '의도',
    node_count: '노드 수',
    snapshot_revision: '스냅샷 리비전',
  },
  'ml-IN': {
    action_ms: 'പ്രവർത്തന സമയം (ms)',
    cache_hit: 'കാഷെ ഹിറ്റ്',
    candidate_count: 'സ്ഥാനാർത്ഥികളുടെ എണ്ണം',
    end_to_end_ms: 'എൻഡ്-ടു-എൻഡ് സമയം (ms)',
    intent: 'ഉദ്ദേശ്യം',
    node_count: 'നോഡുകളുടെ എണ്ണം',
    snapshot_revision: 'സ്നാപ്പ്ഷോട്ട് പരിഷ്കരണം',
  },
  'nb-NO': {
    action_ms: 'Handlingstid (ms)',
    cache_hit: 'Cachetreff',
    candidate_count: 'Antall kandidater',
    end_to_end_ms: 'Ende-til-ende-tid (ms)',
    intent: 'Hensikt',
    node_count: 'Antall noder',
    snapshot_revision: 'Snapshot-revisjon',
  },
  'nl-NL': {
    action_ms: 'Actietijd (ms)',
    cache_hit: 'Cachetreffer',
    candidate_count: 'Aantal kandidaten',
    end_to_end_ms: 'End-to-end-tijd (ms)',
    intent: 'Intentie',
    node_count: 'Aantal knooppunten',
    snapshot_revision: 'Snapshotrevisie',
  },
  'pl-PL': {
    action_ms: 'Czas działania (ms)',
    cache_hit: 'Trafienie pamięci podręcznej',
    candidate_count: 'Liczba kandydatów',
    end_to_end_ms: 'Czas end-to-end (ms)',
    intent: 'Intencja',
    node_count: 'Liczba węzłów',
    snapshot_revision: 'Wersja migawki',
  },
  'pt-BR': {
    action_ms: 'Tempo da ação (ms)',
    cache_hit: 'Acerto de cache',
    candidate_count: 'Número de candidatos',
    end_to_end_ms: 'Tempo de ponta a ponta (ms)',
    intent: 'Intenção',
    node_count: 'Número de nós',
    snapshot_revision: 'Revisão do instantâneo',
  },
  'pt-PT': {
    action_ms: 'Tempo da ação (ms)',
    cache_hit: 'Acerto de cache',
    candidate_count: 'Número de candidatos',
    end_to_end_ms: 'Tempo de ponta a ponta (ms)',
    intent: 'Intenção',
    node_count: 'Número de nós',
    snapshot_revision: 'Revisão do instantâneo',
  },
  'ro-RO': {
    action_ms: 'Timp de acțiune (ms)',
    cache_hit: 'Potrivire cache',
    candidate_count: 'Număr de candidați',
    end_to_end_ms: 'Timp cap-la-cap (ms)',
    intent: 'Intenție',
    node_count: 'Număr de noduri',
    snapshot_revision: 'Revizie instantaneu',
  },
  'ru-RU': {
    action_ms: 'Время действия (мс)',
    cache_hit: 'Попадание в кеш',
    candidate_count: 'Количество кандидатов',
    end_to_end_ms: 'Время от начала до конца (мс)',
    intent: 'Намерение',
    node_count: 'Количество узлов',
    snapshot_revision: 'Ревизия снимка',
  },
  'sk-SK': {
    action_ms: 'Čas akcie (ms)',
    cache_hit: 'Zásah vyrovnávacej pamäte',
    candidate_count: 'Počet kandidátov',
    end_to_end_ms: 'Čas od začiatku do konca (ms)',
    intent: 'Zámer',
    node_count: 'Počet uzlov',
    snapshot_revision: 'Revízia snímky',
  },
  'sv-SE': {
    action_ms: 'Åtgärdstid (ms)',
    cache_hit: 'Cacheträff',
    candidate_count: 'Antal kandidater',
    end_to_end_ms: 'Tid från början till slut (ms)',
    intent: 'Avsikt',
    node_count: 'Antal noder',
    snapshot_revision: 'Snapshot-revision',
  },
  'zh-CN': {
    action_ms: '操作耗时 (ms)',
    cache_hit: '命中缓存',
    candidate_count: '候选数量',
    end_to_end_ms: '端到端耗时 (ms)',
    intent: '意图',
    node_count: '节点数量',
    snapshot_revision: '快照修订号',
  },
  'zh-TW': {
    action_ms: '操作耗時 (ms)',
    cache_hit: '命中快取',
    candidate_count: '候選數量',
    end_to_end_ms: '端對端耗時 (ms)',
    intent: '意圖',
    node_count: '節點數量',
    snapshot_revision: '快照修訂號',
  },
}

const resultCardHostActionCompletedPlainMessages: Record<LocaleKey, string> = {
  'ca-ES': "L'acció de l'amfitrió s'ha completat",
  'cs-CZ': 'Akce hostitele byla dokončena',
  'da-DK': 'Værtshandling fuldført',
  'de-DE': 'Host-Aktion abgeschlossen',
  'el-GR': 'Η ενέργεια του κεντρικού υπολογιστή ολοκληρώθηκε',
  'en-GB': 'Host action completed',
  'en-US': 'Host action completed',
  'es-ES': 'La acción del host se completó',
  'fr-FR': "L'action hôte a été terminée",
  'ga-IE': 'Críochnaíodh gníomh an óstaigh',
  'hr-HR': 'Radnja hosta dovršena',
  'hu-HU': 'A gazdagép művelete befejeződött',
  'it-IT': "L'azione host è stata completata",
  'ja-JP': 'ホスト操作が完了しました',
  'ko-KR': '호스트 작업이 완료되었습니다',
  'ml-IN': 'ഹോസ്റ്റ് പ്രവർത്തനം പൂർത്തിയായി',
  'nb-NO': 'Verts-handlingen er fullført',
  'nl-NL': 'Hostactie voltooid',
  'pl-PL': 'Działanie hosta ukończono',
  'pt-BR': 'A ação do host foi concluída',
  'pt-PT': 'A ação do anfitrião foi concluída',
  'ro-RO': 'Acțiunea gazdei a fost finalizată',
  'ru-RU': 'Действие хоста завершено',
  'sk-SK': 'Akcia hostiteľa bola dokončená',
  'sv-SE': 'Värdåtgärden slutfördes',
  'zh-CN': '主机操作已完成',
  'zh-TW': '主機操作已完成',
}

const resultCardHostActionCompletedMessages: Record<LocaleKey, string> = {
  'ca-ES': "L'acció de l'amfitrió s'ha completat i enviat",
  'cs-CZ': 'Akce hostitele byla dokončena a odeslána',
  'da-DK': 'Værtshandling fuldført og sendt',
  'de-DE': 'Host-Aktion abgeschlossen und gesendet',
  'el-GR': 'Η ενέργεια του κεντρικού υπολογιστή ολοκληρώθηκε και υποβλήθηκε',
  'en-GB': 'Host action completed and submitted',
  'en-US': 'Host action completed and submitted',
  'es-ES': 'La acción del host se completó y se envió',
  'fr-FR': "L'action hôte a été terminée et envoyée",
  'ga-IE': 'Críochnaíodh agus cuireadh gníomh an óstaigh isteach',
  'hr-HR': 'Radnja hosta dovršena i poslana',
  'hu-HU': 'A gazdagép művelete befejeződött és elküldésre került',
  'it-IT': "L'azione host è stata completata e inviata",
  'ja-JP': 'ホスト操作が完了して送信されました',
  'ko-KR': '호스트 작업이 완료되어 제출되었습니다',
  'ml-IN': 'ഹോസ്റ്റ് പ്രവർത്തനം പൂർത്തിയാക്കി സമർപ്പിച്ചു',
  'nb-NO': 'Verts-handlingen er fullført og sendt inn',
  'nl-NL': 'Hostactie voltooid en verzonden',
  'pl-PL': 'Działanie hosta ukończono i wysłano',
  'pt-BR': 'A ação do host foi concluída e enviada',
  'pt-PT': 'A ação do anfitrião foi concluída e enviada',
  'ro-RO': 'Acțiunea gazdei a fost finalizată și trimisă',
  'ru-RU': 'Действие хоста завершено и отправлено',
  'sk-SK': 'Akcia hostiteľa bola dokončená a odoslaná',
  'sv-SE': 'Värdåtgärden slutfördes och skickades',
  'zh-CN': '主机操作已完成并提交',
  'zh-TW': '主機操作已完成並提交',
}

const resultCardKeysSentMessages: Record<LocaleKey, string> = {
  'ca-ES': 'Tecles enviades',
  'cs-CZ': 'Klávesy odeslány',
  'da-DK': 'Taster sendt',
  'de-DE': 'Tasten gesendet',
  'el-GR': 'Τα πλήκτρα στάλθηκαν',
  'en-GB': 'Keys sent',
  'en-US': 'Keys sent',
  'es-ES': 'Teclas enviadas',
  'fr-FR': 'Touches envoyées',
  'ga-IE': 'Seoladh na heochracha',
  'hr-HR': 'Tipke poslane',
  'hu-HU': 'Billentyűk elküldve',
  'it-IT': 'Tasti inviati',
  'ja-JP': 'キーを送信しました',
  'ko-KR': '키를 전송했습니다',
  'ml-IN': 'കീസുകൾ അയച്ചു',
  'nb-NO': 'Taster sendt',
  'nl-NL': 'Toetsen verzonden',
  'pl-PL': 'Klawisze wysłane',
  'pt-BR': 'Teclas enviadas',
  'pt-PT': 'Teclas enviadas',
  'ro-RO': 'Taste trimise',
  'ru-RU': 'Клавиши отправлены',
  'sk-SK': 'Klávesy odoslané',
  'sv-SE': 'Tangenter skickade',
  'zh-CN': '按键已发送',
  'zh-TW': '按鍵已送出',
}

const resultCardClipboardValues: Record<LocaleKey, string> = {
  'ca-ES': 'Porta-retalls',
  'cs-CZ': 'Schránka',
  'da-DK': 'Udklipsholder',
  'de-DE': 'Zwischenablage',
  'el-GR': 'Πρόχειρο',
  'en-GB': 'Clipboard',
  'en-US': 'Clipboard',
  'es-ES': 'Portapapeles',
  'fr-FR': 'Presse-papiers',
  'ga-IE': 'Gearrthaisce',
  'hr-HR': 'Međuspremnik',
  'hu-HU': 'Vágólap',
  'it-IT': 'Appunti',
  'ja-JP': 'クリップボード',
  'ko-KR': '클립보드',
  'ml-IN': 'ക്ലിപ്പ്ബോർഡ്',
  'nb-NO': 'Utklippstavle',
  'nl-NL': 'Klembord',
  'pl-PL': 'Schowek',
  'pt-BR': 'Área de transferência',
  'pt-PT': 'Área de transferência',
  'ro-RO': 'Memorie temporară',
  'ru-RU': 'Буфер обмена',
  'sk-SK': 'Schránka',
  'sv-SE': 'Urklipp',
  'zh-CN': '剪贴板',
  'zh-TW': '剪貼簿',
}

const resultCardFocusedTextValues: Record<LocaleKey, string> = {
  'ca-ES': 'Text enfocat',
  'cs-CZ': 'Zaměřený text',
  'da-DK': 'Fokuseret tekst',
  'de-DE': 'Fokussierter Text',
  'el-GR': 'Εστιασμένο κείμενο',
  'en-GB': 'Focused Text',
  'en-US': 'Focused Text',
  'es-ES': 'Texto enfocado',
  'fr-FR': 'Texte focalisé',
  'ga-IE': 'Téacs faoi fhócas',
  'hr-HR': 'Fokusirani tekst',
  'hu-HU': 'Fókuszált szöveg',
  'it-IT': 'Testo focalizzato',
  'ja-JP': 'フォーカス中のテキスト',
  'ko-KR': '포커스된 텍스트',
  'ml-IN': 'ഫോകസ് ചെയ്ത ടെക്സ്റ്റ്',
  'nb-NO': 'Fokusert tekst',
  'nl-NL': 'Gefocuste tekst',
  'pl-PL': 'Tekst w fokusu',
  'pt-BR': 'Texto em foco',
  'pt-PT': 'Texto em foco',
  'ro-RO': 'Text focalizat',
  'ru-RU': 'Текст в фокусе',
  'sk-SK': 'Text vo fokuse',
  'sv-SE': 'Fokuserad text',
  'zh-CN': '聚焦文本',
  'zh-TW': '聚焦文字',
}

const resultCardExecutionModeSemanticValues: Record<LocaleKey, string> = {
  'ca-ES': 'Semàntic',
  'cs-CZ': 'Sémantický',
  'da-DK': 'Semantisk',
  'de-DE': 'Semantisch',
  'el-GR': 'Σημασιολογικό',
  'en-GB': 'Semantic',
  'en-US': 'Semantic',
  'es-ES': 'Semántico',
  'fr-FR': 'Semantique',
  'ga-IE': 'Séimeantach',
  'hr-HR': 'Semantički',
  'hu-HU': 'Szemantikus',
  'it-IT': 'Semantico',
  'ja-JP': 'セマンティック',
  'ko-KR': '의미 기반',
  'ml-IN': 'സെമാന്റിക്',
  'nb-NO': 'Semantisk',
  'nl-NL': 'Semantisch',
  'pl-PL': 'Semantyczny',
  'pt-BR': 'Semântico',
  'pt-PT': 'Semântico',
  'ro-RO': 'Semantică',
  'ru-RU': 'Семантический',
  'sk-SK': 'Sémantický',
  'sv-SE': 'Semantisk',
  'zh-CN': '语义',
  'zh-TW': '語義',
}

const resultCardExecutionModeInputValues: Record<LocaleKey, string> = {
  'ca-ES': 'Entrada',
  'cs-CZ': 'Vstup',
  'da-DK': 'Indtastning',
  'de-DE': 'Eingabe',
  'el-GR': 'Εισαγωγή',
  'en-GB': 'Input',
  'en-US': 'Input',
  'es-ES': 'Entrada',
  'fr-FR': 'Saisie',
  'ga-IE': 'Ionchur',
  'hr-HR': 'Unos',
  'hu-HU': 'Bevitel',
  'it-IT': 'Inserimento',
  'ja-JP': '入力',
  'ko-KR': '입력',
  'ml-IN': 'ഇൻപുട്ട്',
  'nb-NO': 'Inndata',
  'nl-NL': 'Invoer',
  'pl-PL': 'Wprowadzanie',
  'pt-BR': 'Entrada',
  'pt-PT': 'Entrada',
  'ro-RO': 'Intrare',
  'ru-RU': 'Ввод',
  'sk-SK': 'Vstup',
  'sv-SE': 'Inmatning',
  'zh-CN': '输入',
  'zh-TW': '輸入',
}

const resultCardClickValues: Record<LocaleKey, string> = {
  'ca-ES': 'Clic',
  'cs-CZ': 'Kliknutí',
  'da-DK': 'Klik',
  'de-DE': 'Klick',
  'el-GR': 'Κλικ',
  'en-GB': 'Click',
  'en-US': 'Click',
  'es-ES': 'Clic',
  'fr-FR': 'Clic',
  'ga-IE': 'Cliceáil',
  'hr-HR': 'Klik',
  'hu-HU': 'Kattintás',
  'it-IT': 'Clic',
  'ja-JP': 'クリック',
  'ko-KR': '클릭',
  'ml-IN': 'ക്ലിക്ക്',
  'nb-NO': 'Klikk',
  'nl-NL': 'Klik',
  'pl-PL': 'Kliknięcie',
  'pt-BR': 'Clique',
  'pt-PT': 'Clique',
  'ro-RO': 'Clic',
  'ru-RU': 'Щелчок',
  'sk-SK': 'Kliknutie',
  'sv-SE': 'Klick',
  'zh-CN': '点击',
  'zh-TW': '點擊',
}

const resultCardInputClickValues: Record<LocaleKey, string> = {
  'ca-ES': "Clic d'entrada",
  'cs-CZ': 'Vstupní kliknutí',
  'da-DK': 'Inputklik',
  'de-DE': 'Eingabeklick',
  'el-GR': 'Κλικ εισόδου',
  'en-GB': 'Input Click',
  'en-US': 'Input Click',
  'es-ES': 'Clic de entrada',
  'fr-FR': 'Clic de saisie',
  'ga-IE': 'Cliceáil ionchuir',
  'hr-HR': 'Klik unosom',
  'hu-HU': 'Beviteli kattintás',
  'it-IT': 'Clic di input',
  'ja-JP': '入力クリック',
  'ko-KR': '입력 클릭',
  'ml-IN': 'ഇൻപുട്ട് ക്ലിക്ക്',
  'nb-NO': 'Inndataklikk',
  'nl-NL': 'Invoerklik',
  'pl-PL': 'Klik wejściowy',
  'pt-BR': 'Clique de entrada',
  'pt-PT': 'Clique de entrada',
  'ro-RO': 'Clic de intrare',
  'ru-RU': 'Клик ввода',
  'sk-SK': 'Vstupné kliknutie',
  'sv-SE': 'Inmatningsklick',
  'zh-CN': '输入点击',
  'zh-TW': '輸入點擊',
}

const resultCardInputActionValues: Record<LocaleKey, string> = {
  'ca-ES': "Acció d'entrada",
  'cs-CZ': 'Vstupní akce',
  'da-DK': 'Inputhandling',
  'de-DE': 'Eingabeaktion',
  'el-GR': 'Ενέργεια εισόδου',
  'en-GB': 'Input Action',
  'en-US': 'Input Action',
  'es-ES': 'Acción de entrada',
  'fr-FR': 'Action de saisie',
  'ga-IE': 'Gníomh ionchuir',
  'hr-HR': 'Radnja unosa',
  'hu-HU': 'Beviteli művelet',
  'it-IT': 'Azione di input',
  'ja-JP': '入力アクション',
  'ko-KR': '입력 동작',
  'ml-IN': 'ഇൻപുട്ട് പ്രവർത്തനം',
  'nb-NO': 'Inndatahandling',
  'nl-NL': 'Invoeractie',
  'pl-PL': 'Akcja wejściowa',
  'pt-BR': 'Ação de entrada',
  'pt-PT': 'Ação de entrada',
  'ro-RO': 'Acțiune de intrare',
  'ru-RU': 'Действие ввода',
  'sk-SK': 'Vstupná akcia',
  'sv-SE': 'Inmatningsåtgärd',
  'zh-CN': '输入操作',
  'zh-TW': '輸入操作',
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

const memoryDreamEnglishDefaults = {
  dreamTitle: 'Dream Consolidation',
  dreamDescription: 'Archive older chat context and promote durable memory candidates.',
  dreamPendingCapsules: 'Pending Capsules',
  dreamPromotedCount: 'Promoted Memories',
  dreamArchivedDailyLogs: 'Archived Daily Logs',
  dreamLastRun: 'Last Run',
  dreamRun: 'Run Dream Pass',
  dreamRunning: 'Running Dream Pass...',
  dreamRunSuccess:
    'Dream run complete: {processed} capsules processed, {promoted} memories promoted, {archived} daily logs archived.',
} as const

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
    omitted_results: 'Resultats omesos',
    selected_result: 'Resultat seleccionat',
    key_facts: 'Fets clau',
    research_artifact_path: "Ruta de l'artefacte de recerca",
    llm_compacted: 'Compactat per LLM',
    materialized: 'Materialitzat',
    mode: 'Mode',
    search_card_emitted: 'Targeta de cerca emesa',
  },
  'cs-CZ': {
    has_results: 'Má výsledky',
    omitted_results: 'Vynechané výsledky',
    selected_result: 'Vybraný výsledek',
    key_facts: 'Klíčová fakta',
    research_artifact_path: 'Cesta k artefaktu výzkumu',
    llm_compacted: 'Zkompaktováno LLM',
    materialized: 'Materializováno',
    mode: 'Režim',
    search_card_emitted: 'Karta hledání vygenerována',
  },
  'da-DK': {
    has_results: 'Har resultater',
    omitted_results: 'Udeladte resultater',
    selected_result: 'Valgt resultat',
    key_facts: 'Nøglefakta',
    research_artifact_path: 'Sti til research-artefakt',
    llm_compacted: 'Komprimeret af LLM',
    materialized: 'Materialiseret',
    mode: 'Tilstand',
    search_card_emitted: 'Søgekort udsendt',
  },
  'de-DE': {
    has_results: 'Hat Ergebnisse',
    omitted_results: 'Ausgelassene Ergebnisse',
    selected_result: 'Ausgewähltes Ergebnis',
    key_facts: 'Kernfakten',
    research_artifact_path: 'Pfad zum Recherche-Artefakt',
    llm_compacted: 'Durch LLM komprimiert',
    materialized: 'Materialisiert',
    mode: 'Modus',
    search_card_emitted: 'Suchkarte ausgegeben',
  },
  'el-GR': {
    has_results: 'Έχει αποτελέσματα',
    omitted_results: 'Παραλειφθέντα αποτελέσματα',
    selected_result: 'Επιλεγμένο αποτέλεσμα',
    key_facts: 'Βασικά στοιχεία',
    research_artifact_path: 'Διαδρομή τεχνήματος έρευνας',
    llm_compacted: 'Συμπτυγμένο από LLM',
    materialized: 'Υλοποιημένο',
    mode: 'Λειτουργία',
    search_card_emitted: 'Κάρτα αναζήτησης εκδόθηκε',
  },
  'en-GB': {
    has_results: 'Has Results',
    omitted_results: 'Omitted Results',
    selected_result: 'Selected Result',
    key_facts: 'Key Facts',
    research_artifact_path: 'Research Artifact Path',
    llm_compacted: 'LLM Compacted',
    materialized: 'Materialized',
    mode: 'Mode',
    search_card_emitted: 'Search Card Emitted',
  },
  'en-US': {
    has_results: 'Has Results',
    omitted_results: 'Omitted Results',
    selected_result: 'Selected Result',
    key_facts: 'Key Facts',
    research_artifact_path: 'Research Artifact Path',
    llm_compacted: 'LLM Compacted',
    materialized: 'Materialized',
    mode: 'Mode',
    search_card_emitted: 'Search Card Emitted',
  },
  'es-ES': {
    has_results: 'Tiene resultados',
    omitted_results: 'Resultados omitidos',
    selected_result: 'Resultado seleccionado',
    key_facts: 'Datos clave',
    research_artifact_path: 'Ruta del artefacto de investigacion',
    llm_compacted: 'Compactado por LLM',
    materialized: 'Materializado',
    mode: 'Modo',
    search_card_emitted: 'Tarjeta de busqueda emitida',
  },
  'fr-FR': {
    has_results: 'Contient des resultats',
    omitted_results: 'Resultats omis',
    selected_result: 'Resultat selectionne',
    key_facts: 'Faits cles',
    research_artifact_path: "Chemin de l'artefact de recherche",
    llm_compacted: 'Compacte par le LLM',
    materialized: 'Materialise',
    mode: 'Mode',
    search_card_emitted: 'Carte de recherche emise',
  },
  'ga-IE': {
    has_results: 'Tá torthaí ann',
    omitted_results: 'Torthaí fágtha ar lár',
    selected_result: 'Toradh roghnaithe',
    key_facts: 'Príomhfhíricí',
    research_artifact_path: 'Conair go déantán taighde',
    llm_compacted: 'Comhdhlúite ag LLM',
    materialized: 'Cruthaithe',
    mode: 'Mód',
    search_card_emitted: 'Cárta cuardaigh eisithe',
  },
  'hr-HR': {
    has_results: 'Ima rezultate',
    omitted_results: 'Izostavljeni rezultati',
    selected_result: 'Odabrani rezultat',
    key_facts: 'Ključne činjenice',
    research_artifact_path: 'Putanja artefakta istraživanja',
    llm_compacted: 'Kompaktirano LLM-om',
    materialized: 'Materijalizirano',
    mode: 'Način',
    search_card_emitted: 'Kartica pretrage emitirana',
  },
  'hu-HU': {
    has_results: 'Van találat',
    omitted_results: 'Kihagyott eredmények',
    selected_result: 'Kiválasztott eredmény',
    key_facts: 'Kulcstények',
    research_artifact_path: 'Kutatási artefaktum útvonala',
    llm_compacted: 'LLM által tömörítve',
    materialized: 'Materializálva',
    mode: 'Mód',
    search_card_emitted: 'Keresési kártya kiadva',
  },
  'it-IT': {
    has_results: 'Ha risultati',
    omitted_results: 'Risultati omessi',
    selected_result: 'Risultato selezionato',
    key_facts: 'Fatti chiave',
    research_artifact_path: "Percorso dell'artefatto di ricerca",
    llm_compacted: 'Compattato da LLM',
    materialized: 'Materializzato',
    mode: 'Modalità',
    search_card_emitted: 'Scheda di ricerca emessa',
  },
  'ja-JP': {
    has_results: '結果あり',
    omitted_results: '省略された結果',
    selected_result: '選択結果',
    key_facts: '重要事項',
    research_artifact_path: '調査成果物パス',
    llm_compacted: 'LLM圧縮済み',
    materialized: '実体化済み',
    mode: 'モード',
    search_card_emitted: '検索カード出力済み',
  },
  'ko-KR': {
    has_results: '결과 있음',
    omitted_results: '생략된 결과',
    selected_result: '선택된 결과',
    key_facts: '핵심 사실',
    research_artifact_path: '조사 아티팩트 경로',
    llm_compacted: 'LLM 압축됨',
    materialized: '생성됨',
    mode: '모드',
    search_card_emitted: '검색 카드 출력됨',
  },
  'ml-IN': {
    has_results: 'ഫലങ്ങളുണ്ട്',
    omitted_results: 'ഒഴിവാക്കിയ ഫലങ്ങൾ',
    selected_result: 'തിരഞ്ഞെടുത്ത ഫലം',
    key_facts: 'പ്രധാന വിവരങ്ങള്',
    research_artifact_path: 'ഗവേഷണ ആര്‍ട്ടിഫാക്റ്റ് പാത',
    llm_compacted: 'LLM ചുരുക്കിയത്',
    materialized: 'സൃഷ്ടിച്ചത്',
    mode: 'മോഡ്',
    search_card_emitted: 'തിരച്ചിൽ കാർഡ് പുറപ്പെടുവിച്ചു',
  },
  'nb-NO': {
    has_results: 'Har resultater',
    omitted_results: 'Utelatte resultater',
    selected_result: 'Valgt resultat',
    key_facts: 'Nøkkelfakta',
    research_artifact_path: 'Sti til forskningsartefakt',
    llm_compacted: 'Komprimert av LLM',
    materialized: 'Materialisert',
    mode: 'Modus',
    search_card_emitted: 'Søkekort sendt ut',
  },
  'nl-NL': {
    has_results: 'Heeft resultaten',
    omitted_results: 'Weggelaten resultaten',
    selected_result: 'Geselecteerd resultaat',
    key_facts: 'Kernfeiten',
    research_artifact_path: 'Pad naar onderzoeksartefact',
    llm_compacted: 'Gecompacteerd door LLM',
    materialized: 'Gematerialiseerd',
    mode: 'Modus',
    search_card_emitted: 'Zoekkaart uitgegeven',
  },
  'pl-PL': {
    has_results: 'Ma wyniki',
    omitted_results: 'Pominięte wyniki',
    selected_result: 'Wybrany wynik',
    key_facts: 'Kluczowe fakty',
    research_artifact_path: 'Sciezka artefaktu badawczego',
    llm_compacted: 'Skondensowane przez LLM',
    materialized: 'Zmaterializowane',
    mode: 'Tryb',
    search_card_emitted: 'Karta wyszukiwania wyemitowana',
  },
  'pt-BR': {
    has_results: 'Tem resultados',
    omitted_results: 'Resultados omitidos',
    selected_result: 'Resultado selecionado',
    key_facts: 'Fatos-chave',
    research_artifact_path: 'Caminho do artefato de pesquisa',
    llm_compacted: 'Compactado por LLM',
    materialized: 'Materializado',
    mode: 'Modo',
    search_card_emitted: 'Cartão de busca emitido',
  },
  'pt-PT': {
    has_results: 'Tem resultados',
    omitted_results: 'Resultados omitidos',
    selected_result: 'Resultado selecionado',
    key_facts: 'Factos-chave',
    research_artifact_path: 'Caminho do artefacto de pesquisa',
    llm_compacted: 'Compactado por LLM',
    materialized: 'Materializado',
    mode: 'Modo',
    search_card_emitted: 'Cartão de pesquisa emitido',
  },
  'ro-RO': {
    has_results: 'Are rezultate',
    omitted_results: 'Rezultate omise',
    selected_result: 'Rezultat selectat',
    key_facts: 'Fapte cheie',
    research_artifact_path: 'Calea artefactului de cercetare',
    llm_compacted: 'Compactat de LLM',
    materialized: 'Materializat',
    mode: 'Mod',
    search_card_emitted: 'Card de căutare emis',
  },
  'ru-RU': {
    has_results: 'Есть результаты',
    omitted_results: 'Пропущенные результаты',
    selected_result: 'Выбранный результат',
    key_facts: 'Ключевые факты',
    research_artifact_path: 'Путь к артефакту исследования',
    llm_compacted: 'Сжато LLM',
    materialized: 'Материализовано',
    mode: 'Режим',
    search_card_emitted: 'Карточка поиска отправлена',
  },
  'sk-SK': {
    has_results: 'Má výsledky',
    omitted_results: 'Vynechané výsledky',
    selected_result: 'Vybraný výsledok',
    key_facts: 'Kľúčové fakty',
    research_artifact_path: 'Cesta k artefaktu výskumu',
    llm_compacted: 'Zhutnené pomocou LLM',
    materialized: 'Materializované',
    mode: 'Režim',
    search_card_emitted: 'Karta vyhľadávania odoslaná',
  },
  'sv-SE': {
    has_results: 'Har resultat',
    omitted_results: 'Utelämnade resultat',
    selected_result: 'Valt resultat',
    key_facts: 'Nyckelfakta',
    research_artifact_path: 'Sökväg till forskningsartefakt',
    llm_compacted: 'Kompakterad av LLM',
    materialized: 'Materialiserad',
    mode: 'Läge',
    search_card_emitted: 'Sökkort utsänt',
  },
  'zh-CN': {
    has_results: '有结果',
    omitted_results: '省略结果数',
    selected_result: '已选结果',
    key_facts: '关键信息',
    research_artifact_path: '研究产物路径',
    llm_compacted: 'LLM 压缩',
    materialized: '已生成',
    mode: '模式',
    search_card_emitted: '已生成搜索卡片',
  },
  'zh-TW': {
    has_results: '有結果',
    omitted_results: '省略結果數',
    selected_result: '已選結果',
    key_facts: '關鍵資訊',
    research_artifact_path: '研究產物路徑',
    llm_compacted: 'LLM 壓縮',
    materialized: '已生成',
    mode: '模式',
    search_card_emitted: '已生成搜尋卡片',
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
  const resultCardTitlesPatch: LocaleNode = {}
  const resultCardValuesPatch: LocaleNode = {}
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
  const memoryPatch: LocaleNode = {}
  const usersPatch: LocaleNode = {}
  const usersRolesPatch: LocaleNode = {}
  const workspacePatch: LocaleNode = {}
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
    chatPatch.deepResearchActionDraftSynthesisReady = deepResearchRuntimeCopy.draftSynthesisReady
    chatPatch.deepResearchActionDetectedResearchGap = deepResearchRuntimeCopy.detectedResearchGap
  }
  if (deepResearchSummaryCopy) {
    chatPatch.deepResearchSummaryGapCoverage = deepResearchSummaryCopy.summaryGapCoverage
    chatPatch.deepResearchSummaryOfficialGap = deepResearchSummaryCopy.summaryOfficialGap
    chatPatch.deepResearchSummaryResolvedCoverage = deepResearchSummaryCopy.summaryResolvedCoverage
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
  for (const [key, englishFallback] of Object.entries(memoryDreamEnglishDefaults)) {
    const current = getString(messages, `memory.${key}`)
    if (!current || current === englishFallback) {
      memoryPatch[key] = englishFallback
    }
  }
  const askResultCardLabels = askResultCardBackfills[localeKey]
  if (askResultCardLabels) {
    const maybeFillAskResultCardLabel = (
      key: 'q' | 'o' | 'a',
      localized: string,
      englishFallback: string
    ) => {
      const current = getString(messages, `resultCard.labels.${key}`)
      if (!current || current === englishFallback || current === key) {
        resultCardLabelsPatch[key] = localized
      }
    }

    const currentAskTitle = getString(messages, 'resultCard.titles.ask')
    if (!currentAskTitle || currentAskTitle === 'ask' || currentAskTitle === 'Question') {
      resultCardTitlesPatch.ask = askResultCardLabels.title
    }

    maybeFillAskResultCardLabel('q', askResultCardLabels.question, 'Question')
    maybeFillAskResultCardLabel('o', askResultCardLabels.options, 'Options')
    maybeFillAskResultCardLabel('a', askResultCardLabels.answer, 'Answer')
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
      'omitted_results',
      webQueryResultCardLabels.omitted_results,
      'Omitted Results'
    )
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
    maybeFillResultCardLabel(
      'search_card_emitted',
      webQueryResultCardLabels.search_card_emitted,
      'Search Card Emitted'
    )
  }
  const resultCardFieldCopy = resultCardFieldBackfills[localeKey]
  if (resultCardFieldCopy) {
    const maybeBackfillResultCardLabel = (
      key: string,
      localized: string | null,
      englishFallback: string
    ) => {
      if (!localized) return
      const current = getString(messages, `resultCard.labels.${key}`)
      if (!current || current === key || current === englishFallback) {
        resultCardLabelsPatch[key] = localized
      }
    }
    const maybeBackfillResultCardMessage = (
      key: string,
      localized: string | null,
      englishFallback: string
    ) => {
      if (!localized) return
      const current = getString(messages, `resultCard.messages.${key}`)
      if (!current || current === englishFallback) {
        resultCardPatch.messages = {
          ...(isPlainObject(resultCardPatch.messages) ? resultCardPatch.messages : {}),
          [key]: localized,
        }
      }
    }
    const maybeBackfillResultCardValue = (
      group: string,
      key: string,
      localized: string | null,
      englishFallback: string
    ) => {
      if (!localized) return
      const current = getString(messages, `resultCard.values.${group}.${key}`)
      if (!current || current === key || current === englishFallback) {
        resultCardValuesPatch[group] = {
          ...(isPlainObject(resultCardValuesPatch[group]) ? resultCardValuesPatch[group] : {}),
          [key]: localized,
        }
      }
    }
    const firstTranslatedString = (...paths: string[]) => {
      for (const path of paths) {
        const value = getString(messages, path)
        if (value) return value
      }
      return null
    }

    maybeBackfillResultCardLabel(
      'action',
      firstTranslatedString('tools.params.action', 'askQuestion.browserCheckpoint.action'),
      'Action'
    )
    maybeBackfillResultCardLabel(
      'description',
      getString(messages, 'common.description'),
      'Description'
    )
    maybeBackfillResultCardLabel(
      'outputs',
      getString(messages, 'skills.detail.sections.outputs'),
      'Outputs'
    )
    maybeBackfillResultCardLabel('query', getString(messages, 'tools.params.query'), 'Query')
    maybeBackfillResultCardLabel(
      'source',
      getString(messages, 'skillStore.marketplace.filters.source'),
      'Source'
    )
    maybeBackfillResultCardLabel('title', getString(messages, 'common.title'), 'Title')
    maybeBackfillResultCardLabel(
      'url',
      firstTranslatedString('askQuestion.browserCheckpoint.url', 'skillStore.actions.url'),
      'URL'
    )
    maybeBackfillResultCardLabel(
      'warnings',
      getString(messages, 'security.scan.warnings'),
      'Warnings'
    )
    maybeBackfillResultCardLabel(
      'execution_mode',
      resultCardExecutionModeLabels[localeKey],
      'Execution Mode'
    )

    const englishResultCardFieldLabels = resultCardFieldBackfills['en-US'].labels
    for (const [key, localized] of Object.entries(resultCardFieldCopy.labels)) {
      maybeBackfillResultCardLabel(
        key,
        localized,
        englishResultCardFieldLabels[key as keyof typeof englishResultCardFieldLabels]
      )
    }

    const hostActionLabels = resultCardHostActionLabelBackfills[localeKey]
    const englishHostActionLabels = resultCardHostActionLabelBackfills['en-US']
    for (const [key, localized] of Object.entries(hostActionLabels)) {
      maybeBackfillResultCardLabel(
        key,
        localized,
        englishHostActionLabels[key as keyof typeof englishHostActionLabels]
      )
    }

    const hostActionTelemetryLabels = resultCardHostActionTelemetryLabelBackfills[localeKey]
    const englishHostActionTelemetryLabels = resultCardHostActionTelemetryLabelBackfills['en-US']
    for (const [key, localized] of Object.entries(hostActionTelemetryLabels)) {
      maybeBackfillResultCardLabel(
        key,
        localized,
        englishHostActionTelemetryLabels[key as keyof typeof englishHostActionTelemetryLabels]
      )
    }

    const englishResultCardFieldMessages = resultCardFieldBackfills['en-US'].messages
    for (const [key, localized] of Object.entries(resultCardFieldCopy.messages)) {
      maybeBackfillResultCardMessage(
        key,
        localized,
        englishResultCardFieldMessages[key as keyof typeof englishResultCardFieldMessages]
      )
    }
    maybeBackfillResultCardMessage(
      'host_action_completed',
      resultCardHostActionCompletedPlainMessages[localeKey],
      resultCardHostActionCompletedPlainMessages['en-US']
    )
    maybeBackfillResultCardMessage(
      'host_action_completed_and_submitted',
      resultCardHostActionCompletedMessages[localeKey],
      resultCardHostActionCompletedMessages['en-US']
    )
    maybeBackfillResultCardMessage(
      'keys_sent',
      resultCardKeysSentMessages[localeKey],
      resultCardKeysSentMessages['en-US']
    )
    maybeBackfillResultCardMessage(
      'window_focused',
      resultCardWindowFocusedMessages[localeKey],
      'Window focused'
    )
    maybeBackfillResultCardValue(
      'execution_mode',
      'input',
      resultCardExecutionModeInputValues[localeKey],
      resultCardExecutionModeInputValues['en-US']
    )
    maybeBackfillResultCardValue(
      'execution_mode',
      'semantic',
      resultCardExecutionModeSemanticValues[localeKey],
      'Semantic'
    )
    maybeBackfillResultCardValue(
      'fallbacks',
      'click',
      resultCardClickValues[localeKey],
      resultCardClickValues['en-US']
    )
    maybeBackfillResultCardValue(
      'fallbacks',
      'clipboard',
      resultCardClipboardValues[localeKey],
      resultCardClipboardValues['en-US']
    )
    maybeBackfillResultCardValue(
      'fallbacks',
      'focused_text',
      resultCardFocusedTextValues[localeKey],
      resultCardFocusedTextValues['en-US']
    )
    maybeBackfillResultCardValue(
      'fallbacks',
      'input_click',
      resultCardInputClickValues[localeKey],
      resultCardInputClickValues['en-US']
    )
    maybeBackfillResultCardValue('host_os', 'darwin', 'macOS', 'Darwin')
    maybeBackfillResultCardValue(
      'input_method',
      'clipboard',
      resultCardClipboardValues[localeKey],
      resultCardClipboardValues['en-US']
    )
    maybeBackfillResultCardValue(
      'input_method',
      'input_click',
      resultCardInputClickValues[localeKey],
      resultCardInputClickValues['en-US']
    )
    maybeBackfillResultCardValue(
      'intent',
      'click',
      resultCardClickValues[localeKey],
      resultCardClickValues['en-US']
    )
    maybeBackfillResultCardValue(
      'verification_method',
      'input_action',
      resultCardInputActionValues[localeKey],
      resultCardInputActionValues['en-US']
    )
    maybeBackfillResultCardValue(
      'verification_method',
      'focused_text',
      resultCardFocusedTextValues[localeKey],
      resultCardFocusedTextValues['en-US']
    )
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
  const workspaceNavigationBackfill = workspaceNavigationLocaleBackfills[localeKey]
  if (workspaceNavigationBackfill) {
    navPatch.workspacePanelTitle = workspaceNavigationBackfill.generatedTab
    navPatch.workspaceGeneratedTab = workspaceNavigationBackfill.generatedTab
    navPatch.workspaceCoreTab = workspaceNavigationBackfill.coreTab
    workspacePatch.title = workspaceNavigationBackfill.coreTab
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
  if (hasKeys(resultCardTitlesPatch)) {
    resultCardPatch.titles = resultCardTitlesPatch
  }
  if (hasKeys(resultCardValuesPatch)) {
    resultCardPatch.values = resultCardValuesPatch
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
  if (hasKeys(workspacePatch)) {
    patch.workspace = {
      ...(patch.workspace as LocaleNode | undefined),
      ...workspacePatch,
    }
  }
  if (hasKeys(toolsNamesPatch)) {
    toolsPatch.names = toolsNamesPatch
  }
  if (hasKeys(toolsPatch)) {
    patch.tools = toolsPatch
  }
  if (hasKeys(memoryPatch)) {
    patch.memory = {
      ...(patch.memory as LocaleNode | undefined),
      ...memoryPatch,
    }
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
