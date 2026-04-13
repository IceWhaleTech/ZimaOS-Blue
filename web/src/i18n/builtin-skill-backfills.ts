import type { LocaleKey } from './locale-catalog'

type LocaleLeaf = string | number | boolean | null
type LocaleNode = { [key: string]: LocaleLeaf | LocaleNode }
type NamedLocaleTerms = { name: string; description: string }

type BuiltinSkillLocaleTerms = {
  configDescription: string
  himalayaName: string
  himalayaDescription: string
  humanizerName: string
  humanizerDescription: string
  planAppendName: string
  planAppendDescription: string
  planCreateName: string
  planCreateDescription: string
  planUpdateName: string
  planUpdateDescription: string
  selfReflectName: string
  selfReflectDescription: string
  summarizeName: string
  summarizeDescription: string
  webQueryName: string
  webQueryDescription: string
}

const manualSkillLocaleTerms: Partial<Record<LocaleKey, BuiltinSkillLocaleTerms>> = {
  'ca-ES': {
    configDescription:
      "Gestiona la configuracio d'execucio, els proveidors, els usuaris i els controls d'administracio.",
    himalayaName: 'CLI de correu Himalaya',
    himalayaDescription:
      'Fes servir la CLI externa de correu Himalaya per a fluxos reals de correu.',
    humanizerName: 'Humanitzador',
    humanizerDescription: 'Reescriu un fitxer de text local amb un llenguatge mes natural.',
    planAppendName: 'Afegir al pla',
    planAppendDescription: 'Afegeix una tasca a un pla existent.',
    planCreateName: 'Crear pla',
    planCreateDescription: "Crea un pla de comprovacio per a l'ambit actual.",
    planUpdateName: 'Actualitzar pla',
    planUpdateDescription: 'Actualitza una tasca existent del pla.',
    selfReflectName: 'Autoreflexio',
    selfReflectDescription: 'Recull les llisons apreses d una tasca completada.',
    summarizeName: 'Resumir',
    summarizeDescription: 'Fes servir summarize.sh per resumir URL, fitxers i transcripcions.',
    webQueryName: 'Consulta web',
    webQueryDescription: 'Cerca o llegeix pagines web publiques a partir d una consulta o URL.',
  },
  'da-DK': {
    configDescription:
      'Administrer runtime-indstillinger, udbydere, brugere og administrationskontroller.',
    himalayaName: 'Himalaya e-mail-CLI',
    himalayaDescription: 'Brug den eksterne Himalaya e-mail-CLI til rigtige mail-workflows.',
    humanizerName: 'Humanizer',
    humanizerDescription: 'Omskriv en lokal tekstfil til mere naturligt sprog.',
    planAppendName: 'Tilfoj til plan',
    planAppendDescription: 'Tilfoj en opgave til en eksisterende plan.',
    planCreateName: 'Opret plan',
    planCreateDescription: 'Opret en tjeklisteplan for det aktuelle omrade.',
    planUpdateName: 'Opdater plan',
    planUpdateDescription: 'Opdater en eksisterende planopgave.',
    selfReflectName: 'Selvrefleksion',
    selfReflectDescription: 'Indfang laering fra en afsluttet opgave.',
    summarizeName: 'Opsummer',
    summarizeDescription: 'Brug summarize.sh til at opsummere URLer, filer og transskriptioner.',
    webQueryName: 'Webforesporgsel',
    webQueryDescription: 'Sog eller laes offentlige websider ud fra en foresporgsel eller URL.',
  },
  'de-DE': {
    configDescription: 'Verwalte Laufzeiteinstellungen, Anbieter, Benutzer und Admin-Steuerungen.',
    himalayaName: 'Himalaya E-Mail-CLI',
    himalayaDescription: 'Nutze die externe Himalaya-E-Mail-CLI fur echte Mail-Workflows.',
    humanizerName: 'Text-Humanizer',
    humanizerDescription: 'Schreibe eine lokale Textdatei in naturlicherer Sprache um.',
    planAppendName: 'Plan erweitern',
    planAppendDescription: 'Fuge einem bestehenden Plan eine Aufgabe hinzu.',
    planCreateName: 'Plan erstellen',
    planCreateDescription: 'Erstelle einen Checklistenplan fur den aktuellen Bereich.',
    planUpdateName: 'Plan aktualisieren',
    planUpdateDescription: 'Aktualisiere eine bestehende Planaufgabe.',
    selfReflectName: 'Selbstreflexion',
    selfReflectDescription: 'Halte Erkenntnisse aus einer abgeschlossenen Aufgabe fest.',
    summarizeName: 'Zusammenfassen',
    summarizeDescription: 'Nutze summarize.sh, um URLs, Dateien und Transkripte zusammenzufassen.',
    webQueryName: 'Web-Abfrage',
    webQueryDescription: 'Suche oder lies offentliche Webseiten anhand einer Anfrage oder URL.',
  },
  'en-GB': {
    configDescription: 'Manage runtime settings, providers, users, and admin controls.',
    himalayaName: 'Himalaya Email CLI',
    himalayaDescription: 'Use the external Himalaya email CLI for real mail workflows.',
    humanizerName: 'Humanizer',
    humanizerDescription: 'Rewrite a local text file into more natural language.',
    planAppendName: 'Plan Append',
    planAppendDescription: 'Append a task to an existing plan.',
    planCreateName: 'Plan Create',
    planCreateDescription: 'Create a checklist plan for the current scope.',
    planUpdateName: 'Plan Update',
    planUpdateDescription: 'Update an existing plan task.',
    selfReflectName: 'Self Reflect',
    selfReflectDescription: 'Capture lessons learned from a completed task.',
    summarizeName: 'Summarise',
    summarizeDescription: 'Use summarize.sh to summarise URLs, files, and transcripts.',
    webQueryName: 'Web Query',
    webQueryDescription: 'Search or read public web pages from a query or URL.',
  },
  'en-US': {
    configDescription: 'Manage runtime settings, providers, users, and admin controls.',
    himalayaName: 'Himalaya Email CLI',
    himalayaDescription: 'Use the external Himalaya email CLI for real mail workflows.',
    humanizerName: 'Humanizer',
    humanizerDescription: 'Rewrite a local text file into more natural language.',
    planAppendName: 'Plan Append',
    planAppendDescription: 'Append a task to an existing plan.',
    planCreateName: 'Plan Create',
    planCreateDescription: 'Create a checklist plan for the current scope.',
    planUpdateName: 'Plan Update',
    planUpdateDescription: 'Update an existing plan task.',
    selfReflectName: 'Self Reflect',
    selfReflectDescription: 'Capture lessons learned from a completed task.',
    summarizeName: 'Summarize',
    summarizeDescription: 'Use summarize.sh to summarize URLs, files, and transcripts.',
    webQueryName: 'Web Query',
    webQueryDescription: 'Search or read public web pages from a query or URL.',
  },
  'es-ES': {
    configDescription:
      'Gestiona ajustes de ejecucion, proveedores, usuarios y controles de administracion.',
    himalayaName: 'CLI de correo Himalaya',
    himalayaDescription: 'Usa la CLI de correo externa Himalaya para flujos de correo reales.',
    humanizerName: 'Humanizador',
    humanizerDescription: 'Reescribe un archivo de texto local con un lenguaje mas natural.',
    planAppendName: 'Anadir al plan',
    planAppendDescription: 'Anade una tarea a un plan existente.',
    planCreateName: 'Crear plan',
    planCreateDescription: 'Crea un plan de lista de verificacion para el ambito actual.',
    planUpdateName: 'Actualizar plan',
    planUpdateDescription: 'Actualiza una tarea existente del plan.',
    selfReflectName: 'Autorreflexion',
    selfReflectDescription: 'Captura las lecciones aprendidas de una tarea completada.',
    summarizeName: 'Resumir',
    summarizeDescription: 'Usa summarize.sh para resumir URL, archivos y transcripciones.',
    webQueryName: 'Consulta web',
    webQueryDescription: 'Busca o lee paginas web publicas desde una consulta o URL.',
  },
  'fr-FR': {
    configDescription:
      "Gerez les parametres d'execution, les fournisseurs, les utilisateurs et les controles d'administration.",
    himalayaName: 'CLI email Himalaya',
    himalayaDescription:
      'Utilisez la CLI email externe Himalaya pour de vrais workflows de messagerie.',
    humanizerName: 'Humaniseur',
    humanizerDescription: 'Reecrivez un fichier texte local dans un style plus naturel.',
    planAppendName: 'Ajouter au plan',
    planAppendDescription: 'Ajoutez une tache a un plan existant.',
    planCreateName: 'Creer un plan',
    planCreateDescription: 'Creez un plan de checklist pour la portee actuelle.',
    planUpdateName: 'Mettre a jour le plan',
    planUpdateDescription: 'Mettez a jour une tache de plan existante.',
    selfReflectName: 'Auto-reflexion',
    selfReflectDescription: 'Capturez les lecons tirees d une tache terminee.',
    summarizeName: 'Resume',
    summarizeDescription: 'Utilisez summarize.sh pour resumer des URL, fichiers et transcriptions.',
    webQueryName: 'Requete web',
    webQueryDescription:
      "Recherchez ou lisez des pages Web publiques a partir d'une requete ou d'une URL.",
  },
  'it-IT': {
    configDescription:
      'Gestisci impostazioni di runtime, provider, utenti e controlli amministrativi.',
    himalayaName: 'CLI email Himalaya',
    himalayaDescription: 'Usa la CLI email esterna Himalaya per flussi email reali.',
    humanizerName: 'Umanizzatore',
    humanizerDescription: 'Riscrivi un file di testo locale con un linguaggio piu naturale.',
    planAppendName: 'Aggiungi al piano',
    planAppendDescription: 'Aggiungi un attivita a un piano esistente.',
    planCreateName: 'Crea piano',
    planCreateDescription: "Crea un piano checklist per l'ambito corrente.",
    planUpdateName: 'Aggiorna piano',
    planUpdateDescription: 'Aggiorna un attivita esistente del piano.',
    selfReflectName: 'Autoriflessione',
    selfReflectDescription: 'Raccogli le lezioni apprese da un attivita completata.',
    summarizeName: 'Riassumi',
    summarizeDescription: 'Usa summarize.sh per riassumere URL, file e trascrizioni.',
    webQueryName: 'Query web',
    webQueryDescription: 'Cerca o leggi pagine web pubbliche da una query o URL.',
  },
  'nl-NL': {
    configDescription: 'Beheer runtime-instellingen, providers, gebruikers en beheerdersbediening.',
    himalayaName: 'Himalaya e-mail-CLI',
    himalayaDescription: 'Gebruik de externe Himalaya e-mail-CLI voor echte mailworkflows.',
    humanizerName: 'Humanizer',
    humanizerDescription: 'Herschrijf een lokaal tekstbestand in natuurlijkere taal.',
    planAppendName: 'Aan plan toevoegen',
    planAppendDescription: 'Voeg een taak toe aan een bestaand plan.',
    planCreateName: 'Plan maken',
    planCreateDescription: 'Maak een checklistplan voor de huidige scope.',
    planUpdateName: 'Plan bijwerken',
    planUpdateDescription: 'Werk een bestaande plandtaak bij.',
    selfReflectName: 'Zelfreflectie',
    selfReflectDescription: 'Leg geleerde lessen van een voltooide taak vast.',
    summarizeName: 'Samenvatten',
    summarizeDescription:
      "Gebruik summarize.sh om URL's, bestanden en transcripties samen te vatten.",
    webQueryName: 'Webquery',
    webQueryDescription: "Zoek of lees openbare webpagina's via een zoekopdracht of URL.",
  },
  'pt-BR': {
    configDescription:
      'Gerencie configuracoes de runtime, provedores, usuarios e controles administrativos.',
    himalayaName: 'CLI de email Himalaya',
    himalayaDescription: 'Use a CLI de email Himalaya para fluxos reais de correio.',
    humanizerName: 'Humanizador',
    humanizerDescription: 'Reescreva um arquivo de texto local com linguagem mais natural.',
    planAppendName: 'Adicionar ao plano',
    planAppendDescription: 'Adicione uma tarefa a um plano existente.',
    planCreateName: 'Criar plano',
    planCreateDescription: 'Crie um plano de checklist para o escopo atual.',
    planUpdateName: 'Atualizar plano',
    planUpdateDescription: 'Atualize uma tarefa existente do plano.',
    selfReflectName: 'Autorreflexao',
    selfReflectDescription: 'Registre licoes aprendidas de uma tarefa concluida.',
    summarizeName: 'Resumir',
    summarizeDescription: 'Use o summarize.sh para resumir URLs, arquivos e transcricoes.',
    webQueryName: 'Consulta web',
    webQueryDescription:
      'Pesquise ou leia paginas publicas da web a partir de uma consulta ou URL.',
  },
  'pt-PT': {
    configDescription:
      'Gira definicoes de runtime, fornecedores, utilizadores e controlos de administracao.',
    himalayaName: 'CLI de email Himalaya',
    himalayaDescription: 'Usa a CLI de email Himalaya para fluxos reais de correio.',
    humanizerName: 'Humanizador',
    humanizerDescription: 'Reescreve um ficheiro de texto local com linguagem mais natural.',
    planAppendName: 'Adicionar ao plano',
    planAppendDescription: 'Adiciona uma tarefa a um plano existente.',
    planCreateName: 'Criar plano',
    planCreateDescription: 'Cria um plano de checklist para o ambito atual.',
    planUpdateName: 'Atualizar plano',
    planUpdateDescription: 'Atualiza uma tarefa existente do plano.',
    selfReflectName: 'Autorreflexao',
    selfReflectDescription: 'Regista licoes aprendidas de uma tarefa concluida.',
    summarizeName: 'Resumir',
    summarizeDescription: 'Usa o summarize.sh para resumir URLs, ficheiros e transcricoes.',
    webQueryName: 'Consulta web',
    webQueryDescription: 'Pesquisa ou le paginas web publicas a partir de uma consulta ou URL.',
  },
  'sv-SE': {
    configDescription:
      'Hantera runtime-installningar, leverantorer, anvandare och administrationskontroller.',
    himalayaName: 'Himalaya e-post-CLI',
    himalayaDescription: 'Anvand den externa Himalaya e-post-CLI:n for riktiga e-postfloden.',
    humanizerName: 'Humanizer',
    humanizerDescription: 'Skriv om en lokal textfil till mer naturligt sprak.',
    planAppendName: 'Lagg till i plan',
    planAppendDescription: 'Lagg till en uppgift i en befintlig plan.',
    planCreateName: 'Skapa plan',
    planCreateDescription: 'Skapa en checklisteplan for aktuell omfattning.',
    planUpdateName: 'Uppdatera plan',
    planUpdateDescription: 'Uppdatera en befintlig planuppgift.',
    selfReflectName: 'Sjalvreflektion',
    selfReflectDescription: 'Samla lardomar fran en slutford uppgift.',
    summarizeName: 'Sammanfatta',
    summarizeDescription:
      'Anvand summarize.sh for att sammanfatta URLer, filer och transkriptioner.',
    webQueryName: 'Webbfraga',
    webQueryDescription: 'Sok efter eller las offentliga webbsidor fran en fraga eller URL.',
  },
  'zh-CN': {
    configDescription: '管理运行时设置、提供商、用户和管理控制项。',
    himalayaName: 'Himalaya 邮件 CLI',
    himalayaDescription: '使用外部 Himalaya 邮件 CLI 处理真实邮箱工作流。',
    humanizerName: '文本润色',
    humanizerDescription: '将本地文本文件改写成更自然的表达。',
    planAppendName: '计划追加',
    planAppendDescription: '向现有计划追加一个任务。',
    planCreateName: '创建计划',
    planCreateDescription: '为当前范围创建清单计划。',
    planUpdateName: '更新计划',
    planUpdateDescription: '更新现有计划中的任务。',
    selfReflectName: '任务复盘',
    selfReflectDescription: '记录已完成任务的经验教训。',
    summarizeName: '内容摘要',
    summarizeDescription: '使用 summarize.sh 摘要 URL、文件和转录内容。',
    webQueryName: '网页查询',
    webQueryDescription: '根据查询词或 URL 搜索或读取公开网页。',
  },
  'zh-TW': {
    configDescription: '管理執行階段設定、供應商、使用者與管理控制項。',
    himalayaName: 'Himalaya 郵件 CLI',
    himalayaDescription: '使用外部 Himalaya 郵件 CLI 處理真實郵件工作流程。',
    humanizerName: '文字潤飾',
    humanizerDescription: '將本機文字檔改寫成更自然的表達。',
    planAppendName: '追加計畫',
    planAppendDescription: '將任務追加到現有計畫。',
    planCreateName: '建立計畫',
    planCreateDescription: '為目前範圍建立檢查清單計畫。',
    planUpdateName: '更新計畫',
    planUpdateDescription: '更新現有計畫中的任務。',
    selfReflectName: '任務復盤',
    selfReflectDescription: '記錄已完成任務的經驗教訓。',
    summarizeName: '內容摘要',
    summarizeDescription: '使用 summarize.sh 摘要 URL、檔案與轉錄內容。',
    webQueryName: '網頁查詢',
    webQueryDescription: '依查詢或 URL 搜尋或讀取公開網頁。',
  },
  'cs-CZ': {
    configDescription:
      'Spravujte nastaveni behu, poskytovatele, uzivatele a administratorske ovladani.',
    himalayaName: 'E-mailove CLI Himalaya',
    himalayaDescription: 'Pouzijte externi e-mailove CLI Himalaya pro skutecne mailove workflow.',
    humanizerName: 'Humanizer',
    humanizerDescription: 'Prepiste mistni textovy soubor prirozenejsim jazykem.',
    planAppendName: 'Pridat do planu',
    planAppendDescription: 'Pridejte ukol do existujiciho planu.',
    planCreateName: 'Vytvorit plan',
    planCreateDescription: 'Vytvorte checklistovy plan pro aktualni rozsah.',
    planUpdateName: 'Aktualizovat plan',
    planUpdateDescription: 'Aktualizujte existujici ukol v planu.',
    selfReflectName: 'Sebereflexe',
    selfReflectDescription: 'Zaznamenejte ponauceni z dokonceneho ukolu.',
    summarizeName: 'Shrnout',
    summarizeDescription: 'Pouzijte summarize.sh ke shrnuti URL, souboru a prepisu.',
    webQueryName: 'Webovy dotaz',
    webQueryDescription: 'Vyhledavejte nebo ctete verejne webove stranky z dotazu nebo URL.',
  },
  'el-GR': {
    configDescription:
      'Diaxeiriste rythmiseis runtime, paroxous, xristes kai diacheiristika stoixeia elegxou.',
    himalayaName: 'CLI email Himalaya',
    himalayaDescription:
      'Xrisimopoiiste to exoteriko CLI email Himalaya gia pragmatikes roes allilografias.',
    humanizerName: 'Humanizer',
    humanizerDescription: 'Xanaγράψτε ena topiko arxeio keimenou se pio fysiki glossa.',
    planAppendName: 'Prosthiki sto schedio',
    planAppendDescription: 'Prosthesste mia ergasia se ena yparchon schedio.',
    planCreateName: 'Dimiourgia schediou',
    planCreateDescription: 'Dimiourgiste ena schedio listas elegxou gia to trechon pedio.',
    planUpdateName: 'Enimerosi schediou',
    planUpdateDescription: 'Enimeroste mia yparchousa ergasia schediou.',
    selfReflectName: 'Autoanastochasmos',
    selfReflectDescription: 'Katagrapste mathimata apo mia olokliromeni ergasia.',
    summarizeName: 'Synopsi',
    summarizeDescription:
      'Xrisimopoiiste to summarize.sh gia synopsi URL, archeion kai apomagnitofoniseon.',
    webQueryName: 'Erotima istou',
    webQueryDescription: 'Anazitiste i diavaste dimosies istoselides apo erotima i URL.',
  },
  'ga-IE': {
    configDescription:
      'Bainistigh socruithe ama rite, solathraithe, usaidioiri agus rialuithe riarachain.',
    himalayaName: 'CLI r-phoist Himalaya',
    himalayaDescription:
      'Usaid an CLI r-phoist seachtrach Himalaya le haghaidh sreafai poist fior.',
    humanizerName: 'Daonaitheoir',
    humanizerDescription: 'Athscriobh comhad teacs aitiuil i dteanga nios nadurtha.',
    planAppendName: 'Cuir leis an bplean',
    planAppendDescription: 'Cuir tasc le plean ata ann cheana.',
    planCreateName: 'Cruthaigh plean',
    planCreateDescription: 'Cruthaigh plean seicliosta don raon reatha.',
    planUpdateName: 'Nuashonraigh plean',
    planUpdateDescription: 'Nuashonraigh tasc plean ata ann cheana.',
    selfReflectName: 'Feinmhachnamh',
    selfReflectDescription: 'Taifead ceachtanna o thasc criochnaithe.',
    summarizeName: 'Achoimrigh',
    summarizeDescription:
      'Usaid summarize.sh chun URLanna, comhaid agus tras-scribhinní a achoimriu.',
    webQueryName: 'Ceist Ghréasáin',
    webQueryDescription: 'Cuardaigh no leigh leathanaigh ghréasáin phoibli o cheist no URL.',
  },
  'hr-HR': {
    configDescription:
      'Upravljajte postavkama izvodenja, pruzateljima, korisnicima i administratorskim kontrolama.',
    himalayaName: 'Himalaya e-mail CLI',
    himalayaDescription: 'Koristite vanjski Himalaya e-mail CLI za stvarne mail tokove rada.',
    humanizerName: 'Humanizator',
    humanizerDescription: 'Prepisite lokalnu tekstnu datoteku prirodnijim jezikom.',
    planAppendName: 'Dodaj u plan',
    planAppendDescription: 'Dodajte zadatak u postojeci plan.',
    planCreateName: 'Izradi plan',
    planCreateDescription: 'Izradite kontrolni plan za trenutni opseg.',
    planUpdateName: 'Azuriraj plan',
    planUpdateDescription: 'Azurirajte postojeci zadatak u planu.',
    selfReflectName: 'Samorefleksija',
    selfReflectDescription: 'Zabiljezite lekcije iz dovrsenog zadatka.',
    summarizeName: 'Sazmi',
    summarizeDescription: 'Koristite summarize.sh za sažimanje URL-ova, datoteka i transkripata.',
    webQueryName: 'Web upit',
    webQueryDescription: 'Pretrazujte ili citajte javne web stranice iz upita ili URL-a.',
  },
  'hu-HU': {
    configDescription:
      'Kezeld a futtatasi beallitasokat, szolgaltatokat, felhasznalokat es adminisztracios vezerlokat.',
    himalayaName: 'Himalaya email CLI',
    himalayaDescription:
      'Hasznald a kulso Himalaya email CLI-t valos levelezesi munkafolyamatokhoz.',
    humanizerName: 'Humanizalo',
    humanizerDescription: 'Irj at egy helyi szovegfajlt termeszetesebb nyelvezetre.',
    planAppendName: 'Hozzaadas a tervhez',
    planAppendDescription: 'Adj hozza egy feladatot egy meglevo tervhez.',
    planCreateName: 'Terv letrehozasa',
    planCreateDescription: 'Hozz letre egy ellenorzolistat az aktualis hatokorhoz.',
    planUpdateName: 'Terv frissitese',
    planUpdateDescription: 'Frissits egy meglevo tervfeladatot.',
    selfReflectName: 'Onreflexio',
    selfReflectDescription: 'Rogzitsd a befejezett feladat tanulsagait.',
    summarizeName: 'Osszefoglalas',
    summarizeDescription:
      'A summarize.sh segitsegevel foglalj ossze URL-eket, fajlokat es atiratokat.',
    webQueryName: 'Webes lekerdezes',
    webQueryDescription:
      'Nyilvanos weboldalakat kereshetsz vagy olvashatsz lekerdezesbol vagy URL-bol.',
  },
  'ja-JP': {
    configDescription: 'ランタイム設定、プロバイダー、ユーザー、管理コントロールを管理します。',
    himalayaName: 'Himalaya メール CLI',
    himalayaDescription: '実際のメール運用に外部の Himalaya メール CLI を使用します。',
    humanizerName: '自然化',
    humanizerDescription: 'ローカルのテキストファイルをより自然な文章に書き換えます。',
    planAppendName: 'プラン追加',
    planAppendDescription: '既存のプランにタスクを追加します。',
    planCreateName: 'プラン作成',
    planCreateDescription: '現在のスコープ向けにチェックリストプランを作成します。',
    planUpdateName: 'プラン更新',
    planUpdateDescription: '既存のプランタスクを更新します。',
    selfReflectName: '自己振り返り',
    selfReflectDescription: '完了したタスクの学びを記録します。',
    summarizeName: '要約',
    summarizeDescription: 'summarize.sh を使って URL、ファイル、文字起こしを要約します。',
    webQueryName: 'Web クエリ',
    webQueryDescription: 'クエリまたは URL から公開 Web ページを検索または閲覧します。',
  },
  'ko-KR': {
    configDescription: '런타임 설정, 제공자, 사용자 및 관리자 제어를 관리합니다.',
    himalayaName: 'Himalaya 메일 CLI',
    himalayaDescription: '실제 메일 워크플로를 위해 외부 Himalaya 메일 CLI를 사용합니다.',
    humanizerName: '문장 다듬기',
    humanizerDescription: '로컬 텍스트 파일을 더 자연스러운 언어로 다시 씁니다.',
    planAppendName: '계획에 추가',
    planAppendDescription: '기존 계획에 작업을 추가합니다.',
    planCreateName: '계획 만들기',
    planCreateDescription: '현재 범위에 대한 체크리스트 계획을 만듭니다.',
    planUpdateName: '계획 업데이트',
    planUpdateDescription: '기존 계획 작업을 업데이트합니다.',
    selfReflectName: '회고',
    selfReflectDescription: '완료된 작업에서 얻은 교훈을 기록합니다.',
    summarizeName: '요약',
    summarizeDescription: 'summarize.sh로 URL, 파일, 전사본을 요약합니다.',
    webQueryName: '웹 쿼리',
    webQueryDescription: '질문 또는 URL로 공개 웹페이지를 검색하거나 읽습니다.',
  },
  'ml-IN': {
    configDescription:
      'റൺടൈം സജ്ജീകരണങ്ങൾ, പ്രൊവൈഡർമാർ, ഉപയോക്താക്കൾ, അഡ്മിൻ നിയന്ത്രണങ്ങൾ എന്നിവ നിയന്ത്രിക്കുക.',
    himalayaName: 'Himalaya ഇമെയിൽ CLI',
    himalayaDescription: 'യഥാർത്ഥ മെയിൽ പ്രവാഹങ്ങൾക്കായി പുറം Himalaya ഇമെയിൽ CLI ഉപയോഗിക്കുക.',
    humanizerName: 'ഹ്യൂമനൈസർ',
    humanizerDescription:
      'പ്രാദേശിക ടെക്സ്റ്റ് ഫയൽ കൂടുതൽ സ്വാഭാവികമായ ഭാഷയാക്കി പുനരാഖ്യാനം ചെയ്യുക.',
    planAppendName: 'പദ്ധതിയിലേക്ക് ചേർക്കുക',
    planAppendDescription: 'നിലവിലുള്ള പദ്ധതിയിലേക്ക് ഒരു പ്രവർത്തി ചേർക്കുക.',
    planCreateName: 'പദ്ധതി സൃഷ്ടിക്കുക',
    planCreateDescription: 'നിലവിലെ പരിധിക്കായി ഒരു ചെക്ക്ലിസ്റ്റ് പദ്ധതി സൃഷ്ടിക്കുക.',
    planUpdateName: 'പദ്ധതി പുതുക്കുക',
    planUpdateDescription: 'നിലവിലുള്ള പദ്ധതിയിലെ ഒരു പ്രവർത്തി പുതുക്കുക.',
    selfReflectName: 'സ്വയംപരിശോധന',
    selfReflectDescription: 'പൂർത്തിയായ ഒരു പ്രവർത്തിയിൽ നിന്നുള്ള പാഠങ്ങൾ രേഖപ്പെടുത്തുക.',
    summarizeName: 'സംഗ്രഹിക്കുക',
    summarizeDescription:
      'URL-കൾ, ഫയലുകൾ, ട്രാൻസ്ക്രിപ്റ്റുകൾ എന്നിവ സംഗ്രഹിക്കാൻ summarize.sh ഉപയോഗിക്കുക.',
    webQueryName: 'വെബ് ക്വറി',
    webQueryDescription:
      'ചോദ്യമോ URL-മോ ഉപയോഗിച്ച് പൊതുവായ വെബ് പേജുകൾ തിരയുകയോ വായിക്കുകയോ ചെയ്യുക.',
  },
  'nb-NO': {
    configDescription:
      'Administrer kjoretidsinnstillinger, leverandorer, brukere og admin-kontroller.',
    himalayaName: 'Himalaya e-post-CLI',
    himalayaDescription: 'Bruk den eksterne Himalaya e-post-CLI-en for ekte e-postflyter.',
    humanizerName: 'Humanizer',
    humanizerDescription: 'Omskriv en lokal tekstfil til mer naturlig sprak.',
    planAppendName: 'Legg til i plan',
    planAppendDescription: 'Legg til en oppgave i en eksisterende plan.',
    planCreateName: 'Opprett plan',
    planCreateDescription: 'Opprett en sjekklisteplan for gjeldende omrade.',
    planUpdateName: 'Oppdater plan',
    planUpdateDescription: 'Oppdater en eksisterende planoppgave.',
    selfReflectName: 'Selvrefleksjon',
    selfReflectDescription: 'Fang opp laerdom fra en fullfort oppgave.',
    summarizeName: 'Oppsummer',
    summarizeDescription: 'Bruk summarize.sh til a oppsummere URL-er, filer og transkripsjoner.',
    webQueryName: 'Nettsporring',
    webQueryDescription: 'Sok i eller les offentlige nettsider fra en foresporsel eller URL.',
  },
  'pl-PL': {
    configDescription:
      'Zarzadzaj ustawieniami srodowiska uruchomieniowego, dostawcami, uzytkownikami i kontrolkami administracyjnymi.',
    himalayaName: 'CLI poczty Himalaya',
    himalayaDescription:
      'Uzywaj zewnetrznego CLI poczty Himalaya do prawdziwych przeplywow pocztowych.',
    humanizerName: 'Humanizator',
    humanizerDescription: 'Przepisz lokalny plik tekstowy na bardziej naturalny jezyk.',
    planAppendName: 'Dodaj do planu',
    planAppendDescription: 'Dodaj zadanie do istniejacego planu.',
    planCreateName: 'Utworz plan',
    planCreateDescription: 'Utworz plan checklisty dla biezacego zakresu.',
    planUpdateName: 'Zaktualizuj plan',
    planUpdateDescription: 'Zaktualizuj istniejace zadanie w planie.',
    selfReflectName: 'Autorefleksja',
    selfReflectDescription: 'Zapisz wnioski z ukonczonego zadania.',
    summarizeName: 'Podsumuj',
    summarizeDescription: 'Uzyj summarize.sh do podsumowania adresow URL, plikow i transkrypcji.',
    webQueryName: 'Zapytanie webowe',
    webQueryDescription: 'Wyszukuj lub czytaj publiczne strony WWW na podstawie zapytania lub URL.',
  },
  'ro-RO': {
    configDescription:
      'Gestioneaza setarile runtime, furnizorii, utilizatorii si controalele administrative.',
    himalayaName: 'CLI email Himalaya',
    himalayaDescription: 'Foloseste CLI-ul extern de email Himalaya pentru fluxuri reale de posta.',
    humanizerName: 'Humanizator',
    humanizerDescription: 'Rescrie un fisier text local intr-un limbaj mai natural.',
    planAppendName: 'Adauga la plan',
    planAppendDescription: 'Adauga o sarcina la un plan existent.',
    planCreateName: 'Creeaza plan',
    planCreateDescription: 'Creeaza un plan checklist pentru domeniul curent.',
    planUpdateName: 'Actualizeaza plan',
    planUpdateDescription: 'Actualizeaza o sarcina existenta din plan.',
    selfReflectName: 'Auto-reflectie',
    selfReflectDescription: 'Captureaza lectiile invatate dintr-o sarcina finalizata.',
    summarizeName: 'Rezuma',
    summarizeDescription: 'Foloseste summarize.sh pentru a rezuma URL-uri, fisiere si transcrieri.',
    webQueryName: 'Interogare web',
    webQueryDescription: 'Cauta sau citeste pagini web publice dintr-o interogare sau URL.',
  },
  'ru-RU': {
    configDescription:
      'Upravlyayte nastroykami rantayma, provayderami, polzovatelyami i administrativnymi parametrami.',
    himalayaName: 'Pochtovyy CLI Himalaya',
    himalayaDescription:
      'Ispolzuyte vneshniy pochtovyy CLI Himalaya dlya realnykh pochtovykh stsenariev.',
    humanizerName: 'Humanizer',
    humanizerDescription: 'Perepishite lokalnyy tekstovyy fayl bolee estestvennym yazykom.',
    planAppendName: 'Dobavit v plan',
    planAppendDescription: 'Dobavte zadachu v sushchestvuyushchiy plan.',
    planCreateName: 'Sozdat plan',
    planCreateDescription: 'Sozdayte cheklist-plan dlya tekushchey oblasti.',
    planUpdateName: 'Obnovit plan',
    planUpdateDescription: 'Obnovite sushchestvuyushchuyu zadachu plana.',
    selfReflectName: 'Samorefleksiya',
    selfReflectDescription: 'Zafiksiruyte vyvody iz zavershennoy zadachi.',
    summarizeName: 'Summarizatsiya',
    summarizeDescription:
      'Ispolzuyte summarize.sh dlya kratkogo izlozheniya URL, faylov i rasshifrovok.',
    webQueryName: 'Veb-zapros',
    webQueryDescription: 'Ishchite ili chitajte publichnye veb-stranitsy po zaprosu ili URL.',
  },
  'sk-SK': {
    configDescription:
      'Spravujte nastavenia behu, poskytovatelov, pouzivatelov a administratorske ovladanie.',
    himalayaName: 'E-mailove CLI Himalaya',
    himalayaDescription: 'Pouzite externe e-mailove CLI Himalaya pre skutocne mailove workflow.',
    humanizerName: 'Humanizer',
    humanizerDescription: 'Prepiste lokalny textovy subor prirodzenejsim jazykom.',
    planAppendName: 'Pridat do planu',
    planAppendDescription: 'Pridajte ulohu do existujuceho planu.',
    planCreateName: 'Vytvorit plan',
    planCreateDescription: 'Vytvorte checklist plan pre aktualny rozsah.',
    planUpdateName: 'Aktualizovat plan',
    planUpdateDescription: 'Aktualizujte existujucu ulohu v plane.',
    selfReflectName: 'Sebareflexia',
    selfReflectDescription: 'Zachytte ponaucenia z dokoncenej ulohy.',
    summarizeName: 'Zhrnut',
    summarizeDescription: 'Pouzite summarize.sh na zhrnutie URL, suborov a prepisov.',
    webQueryName: 'Webovy dotaz',
    webQueryDescription:
      'Vyhladavajte alebo citajte verejne webove stranky podla dotazu alebo URL.',
  },
}

const officeDocsSkillLocaleTerms: Record<LocaleKey, NamedLocaleTerms> = {
  'ca-ES': {
    name: "Documents d'ofimàtica",
    description:
      "Fes-la servir quan la tasca impliqui crear, llegir, editar, validar o reformatar fitxers .docx, .xlsx, .pptx o .pdf de l'espai de treball, especialment si tambe cal interactuar amb l'aplicacio o les finestres de l'host mitjancant a11y.",
  },
  'cs-CZ': {
    name: 'Kancelarske dokumenty',
    description:
      'Pouzijte ji, kdyz ukol zahrnuje vytvareni, cteni, upravy, validaci nebo preformatovani souboru .docx, .xlsx, .pptx nebo .pdf v pracovnim prostoru, zvlast kdyz je potreba interakce s hostitelskou aplikaci nebo okny pres a11y.',
  },
  'da-DK': {
    name: 'Office-dokumenter',
    description:
      'Brug den, nar opgaven omfatter oprettelse, laesning, redigering, validering eller omformatering af .docx-, .xlsx-, .pptx- eller .pdf-filer i arbejdsomradet, isaer hvis der ogsa er brug for interaktion med vaertsapp eller vinduer via a11y.',
  },
  'de-DE': {
    name: 'Office-Dokumente',
    description:
      'Nutze dies, wenn die Aufgabe das Erstellen, Lesen, Bearbeiten, Validieren oder Neuformatieren von .docx-, .xlsx-, .pptx- oder .pdf-Arbeitsbereichsdateien umfasst, besonders wenn zusaetzlich Host-App- oder Fensterinteraktionen ueber a11y noetig sind.',
  },
  'el-GR': {
    name: 'Εγγραφα Office',
    description:
      'Χρησιμοποιηστε το οταν η εργασια περιλαμβανει δημιουργια, αναγνωση, επεξεργασια, επικυρωση η αναδιαμορφωση αρχειων .docx, .xlsx, .pptx η .pdf του χωρου εργασιας, ειδικα οταν απαιτειται και αλληλεπιδραση με την εφαρμογη η τα παραθυρα του host μεσω a11y.',
  },
  'en-GB': {
    name: 'Office Docs',
    description:
      'Use when the task involves creating, reading, editing, validating, or reformatting .docx, .xlsx, .pptx, or .pdf workspace files, especially when host app or window interaction should also go through a11y.',
  },
  'en-US': {
    name: 'Office Docs',
    description:
      'Use when the task involves creating, reading, editing, validating, or reformatting .docx, .xlsx, .pptx, or .pdf workspace files, especially when host app or window interaction should also go through a11y.',
  },
  'es-ES': {
    name: 'Documentos de oficina',
    description:
      'Usalo cuando la tarea implique crear, leer, editar, validar o reformatear archivos .docx, .xlsx, .pptx o .pdf del espacio de trabajo, especialmente si tambien hace falta interactuar con la aplicacion o las ventanas del host mediante a11y.',
  },
  'fr-FR': {
    name: 'Documents bureautiques',
    description:
      "Utilisez-le lorsque la tache consiste a creer, lire, modifier, valider ou reformater des fichiers .docx, .xlsx, .pptx ou .pdf de l'espace de travail, surtout si l'interaction avec l'application ou les fenetres de l'hote doit aussi passer par a11y.",
  },
  'ga-IE': {
    name: 'Doiciméid oifige',
    description:
      'Bain feidhm as nuair a bhaineann an tasc le comhaid .docx, .xlsx, .pptx no .pdf sa spás oibre a chruthu, a leamh, a chur in eagar, a bhailiu no a athfhormáidiu, go háirithe nuair ba cheart idirghniomhú leis an aip no leis na fuinneoga ar an host a dhéanamh tri a11y freisin.',
  },
  'hr-HR': {
    name: 'Uredski dokumenti',
    description:
      'Koristi ovo kada zadatak ukljucuje stvaranje, citanje, uredjivanje, provjeru ili ponovno formatiranje .docx, .xlsx, .pptx ili .pdf datoteka radnog prostora, posebno kada interakcija s host aplikacijom ili prozorima takodjer treba ici kroz a11y.',
  },
  'hu-HU': {
    name: 'Office-dokumentumok',
    description:
      'Akkor hasznald, ha a feladat .docx, .xlsx, .pptx vagy .pdf munkateruletfajlok letrehozasat, olvasasat, szerkeszteset, ellenorzeset vagy ujraformazasat erinti, kulonosen akkor, ha a host alkalmazassal vagy ablakokkal valo interakcionak is a11y-n keresztul kell tortennie.',
  },
  'it-IT': {
    name: 'Documenti Office',
    description:
      "Usalo quando l'attivita richiede di creare, leggere, modificare, validare o riformattare file .docx, .xlsx, .pptx o .pdf del workspace, soprattutto se anche l'interazione con l'app host o con le finestre deve passare tramite a11y.",
  },
  'ja-JP': {
    name: 'Office ドキュメント',
    description:
      '.docx、.xlsx、.pptx、.pdf のワークスペースファイルを作成、読み取り、編集、検証、再整形する作業で使います。特に、ホストアプリやウィンドウとの操作も a11y 経由で行う必要がある場合に向いています。',
  },
  'ko-KR': {
    name: 'Office 문서',
    description:
      '.docx, .xlsx, .pptx, .pdf 작업공간 파일을 생성, 읽기, 편집, 검증, 재포맷하는 작업에 사용하며, 특히 호스트 앱이나 창 상호작용도 a11y를 통해 처리해야 할 때 적합합니다.',
  },
  'ml-IN': {
    name: 'ഓഫീസ് ഡോക്യുമെന്റുകൾ',
    description:
      '.docx, .xlsx, .pptx, .pdf വർക്ക്‌സ്‌പേസ് ഫയലുകൾ സൃഷ്ടിക്കുക, വായിക്കുക, തിരുത്തുക, സാധൂകരിക്കുക, വീണ്ടും ഫോർമാറ്റ് ചെയ്യുക തുടങ്ങിയ ജോലികൾക്കായി ഇത് ഉപയോഗിക്കുക. പ്രത്യേകിച്ച് host ആപ്പ് അല്ലെങ്കിൽ ജാലകവുമായി ഉള്ള ഇടപെടലും a11y വഴിയാകണം എങ്കിൽ ഇത് ഉപകാരപ്പെടും.',
  },
  'nb-NO': {
    name: 'Office-dokumenter',
    description:
      'Bruk dette nar oppgaven innebærer a opprette, lese, redigere, validere eller omformatere .docx-, .xlsx-, .pptx- eller .pdf-filer i arbeidsomradet, spesielt nar samhandling med vertsapp eller vinduer ogsa bor ga gjennom a11y.',
  },
  'nl-NL': {
    name: 'Office-documenten',
    description:
      'Gebruik dit wanneer de taak het maken, lezen, bewerken, valideren of herformatteren van .docx-, .xlsx-, .pptx- of .pdf-werkruimtebestanden omvat, vooral als interactie met de host-app of vensters ook via a11y moet verlopen.',
  },
  'pl-PL': {
    name: 'Dokumenty biurowe',
    description:
      'Uzyj tego, gdy zadanie obejmuje tworzenie, odczyt, edycje, walidacje lub ponowne formatowanie plikow .docx, .xlsx, .pptx lub .pdf w obszarze roboczym, zwlaszcza gdy interakcja z aplikacja hosta lub oknami rowniez powinna przechodzic przez a11y.',
  },
  'pt-BR': {
    name: 'Documentos de escritorio',
    description:
      'Use quando a tarefa envolver criar, ler, editar, validar ou reformatar arquivos .docx, .xlsx, .pptx ou .pdf do workspace, especialmente quando a interacao com o app host ou com as janelas tambem precisar passar pelo a11y.',
  },
  'pt-PT': {
    name: 'Documentos de escritorio',
    description:
      'Use quando a tarefa envolver criar, ler, editar, validar ou reformatar ficheiros .docx, .xlsx, .pptx ou .pdf do workspace, especialmente quando a interacao com a app host ou com as janelas tambem tiver de passar pelo a11y.',
  },
  'ro-RO': {
    name: 'Documente Office',
    description:
      'Foloseste asta cand sarcina implica sa creezi, sa citesti, sa editezi, sa validezi sau sa reformatezi fisiere .docx, .xlsx, .pptx ori .pdf din workspace, mai ales cand interactiunea cu aplicatia host sau cu ferestrele trebuie sa treaca si prin a11y.',
  },
  'ru-RU': {
    name: 'Office-документы',
    description:
      'Используйте это, когда задача включает создание, чтение, редактирование, проверку или переформатирование файлов .docx, .xlsx, .pptx или .pdf в рабочем пространстве, особенно если взаимодействие с хост-приложением или окнами тоже должно идти через a11y.',
  },
  'sk-SK': {
    name: 'Kancelarske dokumenty',
    description:
      'Pouzite to vtedy, ked uloha zahrna vytvaranie, citanie, upravu, validaciu alebo preformatovanie suborov .docx, .xlsx, .pptx alebo .pdf v pracovnom priestore, najma ked ma interakcia s host aplikaciou alebo oknami tiez prebiehat cez a11y.',
  },
  'sv-SE': {
    name: 'Office-dokument',
    description:
      'Anvand detta nar uppgiften innebar att skapa, lasa, redigera, validera eller formatera om .docx-, .xlsx-, .pptx- eller .pdf-filer i arbetsytan, sarskilt nar interaktion med vardappen eller fonster ocksa bor ga via a11y.',
  },
  'zh-CN': {
    name: '办公文档',
    description:
      '在任务涉及创建、读取、编辑、校验或重新格式化 .docx、.xlsx、.pptx 或 .pdf 工作区文件时使用，尤其适用于还需要通过 a11y 与宿主应用或窗口交互的情况。',
  },
  'zh-TW': {
    name: '辦公文件',
    description:
      '當任務涉及建立、讀取、編輯、驗證或重新格式化 .docx、.xlsx、.pptx 或 .pdf 工作區檔案時使用，特別適合還需要透過 a11y 與宿主應用程式或視窗互動的情況。',
  },
}

function isPlainObject(value: unknown): value is LocaleNode {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

function getString(root: unknown, path: string): string | null {
  const value = path.split('.').reduce<unknown>((current, segment) => {
    if (isPlainObject(current)) {
      return current[segment]
    }
    return undefined
  }, root)

  return typeof value === 'string' && value.trim().length > 0 ? value : null
}

function setCatalogEntry(
  catalog: LocaleNode,
  id: string,
  name: string | null,
  description: string | null
): void {
  if (!name && !description) return
  const entry: LocaleNode = {}
  if (name) entry.name = name
  if (description) entry.description = description
  catalog[id] = entry
}

function firstString(root: unknown, paths: string[]): string | null {
  for (const path of paths) {
    const value = getString(root, path)
    if (value) return value
  }
  return null
}

export function buildBuiltinSkillBackfill(
  localeKey: LocaleKey,
  messages: Record<string, unknown>
): LocaleNode {
  const terms = manualSkillLocaleTerms[localeKey] ?? manualSkillLocaleTerms['en-US']
  const officeDocsTerms = officeDocsSkillLocaleTerms[localeKey]
  if (!terms) return {}

  const catalog: LocaleNode = {}

  setCatalogEntry(
    catalog,
    'ask',
    firstString(messages, ['tools.names.ask']),
    firstString(messages, ['tools.descriptions.ask'])
  )
  setCatalogEntry(
    catalog,
    'advisor',
    firstString(messages, ['tools.names.advisor']),
    firstString(messages, ['tools.descriptions.advisor'])
  )
  setCatalogEntry(
    catalog,
    'calendar',
    firstString(messages, ['skills.builtin.calendar.name', 'tools.names.calendar']),
    firstString(messages, ['skills.builtin.calendar.description'])
  )
  setCatalogEntry(
    catalog,
    'contacts',
    firstString(messages, ['skills.builtin.contacts.name', 'tools.names.contacts']),
    firstString(messages, ['skills.builtin.contacts.description'])
  )
  setCatalogEntry(
    catalog,
    'config',
    firstString(messages, [
      'skills.catalog.config.name',
      'skills.catalog.configuration.name',
      'tools.names.configuration',
    ]),
    terms.configDescription
  )
  setCatalogEntry(
    catalog,
    'deep_research',
    firstString(messages, ['tools.names.deep_research', 'tools.names.research_run']),
    firstString(messages, ['tools.descriptions.deep_research', 'tools.descriptions.research_run'])
  )
  setCatalogEntry(
    catalog,
    'email',
    firstString(messages, ['skills.builtin.email.name', 'tools.names.email']),
    firstString(messages, ['skills.builtin.email.description'])
  )
  setCatalogEntry(catalog, 'himalaya', terms.himalayaName, terms.himalayaDescription)
  setCatalogEntry(catalog, 'humanizer', terms.humanizerName, terms.humanizerDescription)
  setCatalogEntry(catalog, 'office_docs', officeDocsTerms.name, officeDocsTerms.description)
  setCatalogEntry(catalog, 'plan_append', terms.planAppendName, terms.planAppendDescription)
  setCatalogEntry(catalog, 'plan_create', terms.planCreateName, terms.planCreateDescription)
  setCatalogEntry(catalog, 'plan_update', terms.planUpdateName, terms.planUpdateDescription)
  setCatalogEntry(
    catalog,
    'scheduler',
    firstString(messages, ['skills.builtin.scheduler.name', 'tools.names.scheduler']),
    firstString(messages, ['skills.builtin.scheduler.description'])
  )
  setCatalogEntry(catalog, 'self_reflect', terms.selfReflectName, terms.selfReflectDescription)
  setCatalogEntry(catalog, 'summarize', terms.summarizeName, terms.summarizeDescription)
  setCatalogEntry(
    catalog,
    'tasks',
    firstString(messages, ['skills.builtin.tasks.name', 'tools.names.tasks']),
    firstString(messages, ['skills.builtin.tasks.description'])
  )
  setCatalogEntry(catalog, 'web_query', terms.webQueryName, terms.webQueryDescription)

  return {
    skills: {
      catalog,
    },
  }
}

export default buildBuiltinSkillBackfill
