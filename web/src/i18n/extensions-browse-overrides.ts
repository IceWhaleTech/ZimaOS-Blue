type BrowseOverridePack = {
  analysisReport: string
  uiReviewer: string
  configuration: string
  skillGalleryHint: string
  toolGalleryHint: string
  closeSkillDetails: string
  sourceLabel: string
  skillCollectionFilters: string
}

function buildLocaleOverrides(pack: BrowseOverridePack) {
  return {
    extensions: {
      browse: {
        skillGalleryHint: pack.skillGalleryHint,
        toolGalleryHint: pack.toolGalleryHint,
        closeSkillDetails: pack.closeSkillDetails,
        sourceLabel: pack.sourceLabel,
        skillCollectionFilters: pack.skillCollectionFilters,
      },
    },
    skills: {
      catalog: {
        analyze: {
          name: pack.analysisReport,
        },
        ui_reviewer: {
          name: pack.uiReviewer,
        },
        mgmt: {
          name: pack.configuration,
        },
        configuration: {
          name: pack.configuration,
        },
      },
      names: {
        'Analyze Report': pack.analysisReport,
        'Analysis Report': pack.analysisReport,
        'UI Reviewer': pack.uiReviewer,
        Configuration: pack.configuration,
      },
    },
    tools: {
      names: {
        analyze: pack.analysisReport,
        ui_reviewer: pack.uiReviewer,
        configuration: pack.configuration,
        'Analyze Report': pack.analysisReport,
        'Analysis Report': pack.analysisReport,
        'UI Reviewer': pack.uiReviewer,
        Configuration: pack.configuration,
      },
    },
  }
}

export default {
  'ca-ES': buildLocaleOverrides({
    analysisReport: "Informe d'anàlisi",
    uiReviewer: "Revisor d'interfície",
    configuration: 'Configuració',
    skillGalleryHint:
      "Explora les habilitats en targetes. Obre qualsevol habilitat per revisar la documentació i gestionar-ne l'estat.",
    toolGalleryHint:
      'Explora les eines en targetes, revisa què fan i activa-les o desactiva-les ràpidament.',
    closeSkillDetails: "Tanca els detalls de l'habilitat",
    sourceLabel: 'Origen',
    skillCollectionFilters: "Filtres de la col·lecció d'habilitats",
  }),
  'cs-CZ': buildLocaleOverrides({
    analysisReport: 'Zpráva o analýze',
    uiReviewer: 'Kontrola UI',
    configuration: 'Konfigurace',
    skillGalleryHint:
      'Procházejte dovednosti jako karty. Otevřete libovolnou dovednost a zobrazte dokumentaci nebo spravujte její stav.',
    toolGalleryHint:
      'Procházejte nástroje jako karty, podívejte se, co dělají, a rychle je zapínejte nebo vypínejte.',
    closeSkillDetails: 'Zavřít podrobnosti dovednosti',
    sourceLabel: 'Zdroj',
    skillCollectionFilters: 'Filtry kolekce dovedností',
  }),
  'da-DK': buildLocaleOverrides({
    analysisReport: 'Analyserapport',
    uiReviewer: 'UI-gennemgang',
    configuration: 'Konfiguration',
    skillGalleryHint:
      'Gennemse færdigheder som kort. Åbn en vilkårlig færdighed for at læse dokumentationen og administrere dens status.',
    toolGalleryHint:
      'Gennemse værktøjer som kort, se hvad de gør, og slå dem hurtigt til eller fra.',
    closeSkillDetails: 'Luk færdighedsdetaljer',
    sourceLabel: 'Kilde',
    skillCollectionFilters: 'Filtre for færdighedssamling',
  }),
  'de-DE': buildLocaleOverrides({
    analysisReport: 'Analysebericht',
    uiReviewer: 'UI-Prüfung',
    configuration: 'Konfiguration',
    skillGalleryHint:
      'Fähigkeiten als Karten durchsuchen. Öffnen Sie eine Fähigkeit, um die Dokumentation zu prüfen und ihren Status zu verwalten.',
    toolGalleryHint:
      'Werkzeuge als Karten durchsuchen, ihre Funktion prüfen und sie schnell aktivieren oder deaktivieren.',
    closeSkillDetails: 'Skill-Details schließen',
    sourceLabel: 'Quelle',
    skillCollectionFilters: 'Filter der Skill-Sammlung',
  }),
  'el-GR': buildLocaleOverrides({
    analysisReport: 'Αναφορά ανάλυσης',
    uiReviewer: 'Έλεγχος UI',
    configuration: 'Ρύθμιση',
    skillGalleryHint:
      'Περιηγηθείτε στις δεξιότητες ως κάρτες. Ανοίξτε οποιαδήποτε δεξιότητα για να δείτε την τεκμηρίωση και να διαχειριστείτε την κατάστασή της.',
    toolGalleryHint:
      'Περιηγηθείτε στα εργαλεία ως κάρτες, δείτε τι κάνουν και ενεργοποιήστε ή απενεργοποιήστε τα γρήγορα.',
    closeSkillDetails: 'Κλείσιμο λεπτομερειών δεξιότητας',
    sourceLabel: 'Πηγή',
    skillCollectionFilters: 'Φίλτρα συλλογής δεξιοτήτων',
  }),
  'en-GB': buildLocaleOverrides({
    analysisReport: 'Analysis Report',
    uiReviewer: 'UI Reviewer',
    configuration: 'Configuration',
    skillGalleryHint: 'Browse skills as cards. Open any skill to review docs and manage its status.',
    toolGalleryHint:
      'Browse tools as cards, review what they do, and toggle them on or off quickly.',
    closeSkillDetails: 'Close skill details',
    sourceLabel: 'Source',
    skillCollectionFilters: 'Skill collection filters',
  }),
  'en-US': buildLocaleOverrides({
    analysisReport: 'Analysis Report',
    uiReviewer: 'UI Reviewer',
    configuration: 'Configuration',
    skillGalleryHint: 'Browse skills as cards. Open any skill to review docs and manage its status.',
    toolGalleryHint:
      'Browse tools as cards, review what they do, and toggle them on or off quickly.',
    closeSkillDetails: 'Close skill details',
    sourceLabel: 'Source',
    skillCollectionFilters: 'Skill collection filters',
  }),
  'es-ES': buildLocaleOverrides({
    analysisReport: 'Informe de análisis',
    uiReviewer: 'Revisión UI',
    configuration: 'Configuración',
    skillGalleryHint:
      'Explora las habilidades como tarjetas. Abre cualquier habilidad para revisar la documentación y gestionar su estado.',
    toolGalleryHint:
      'Explora las herramientas como tarjetas, revisa qué hacen y actívalas o desactívalas rápidamente.',
    closeSkillDetails: 'Cerrar detalles de la habilidad',
    sourceLabel: 'Origen',
    skillCollectionFilters: 'Filtros de la colección de habilidades',
  }),
  'fr-FR': buildLocaleOverrides({
    analysisReport: 'Rapport d’analyse',
    uiReviewer: 'Revue UI',
    configuration: 'Configuration',
    skillGalleryHint:
      'Parcourez les compétences sous forme de cartes. Ouvrez n’importe quelle compétence pour consulter la documentation et gérer son statut.',
    toolGalleryHint:
      'Parcourez les outils sous forme de cartes, voyez à quoi ils servent et activez-les ou désactivez-les rapidement.',
    closeSkillDetails: 'Fermer les détails de la compétence',
    sourceLabel: 'Source',
    skillCollectionFilters: 'Filtres de la collection de compétences',
  }),
  'ga-IE': buildLocaleOverrides({
    analysisReport: 'Tuarascáil anailíse',
    uiReviewer: 'Athbhreithniú UI',
    configuration: 'Cumraíocht',
    skillGalleryHint:
      'Brabhsáil scileanna mar chártaí. Oscail aon scil chun an cháipéisíocht a léamh agus a stádas a bhainistiú.',
    toolGalleryHint:
      'Brabhsáil uirlisí mar chártaí, féach cad a dhéanann siad, agus cuir ar siúl nó as iad go tapa.',
    closeSkillDetails: 'Dún sonraí na scile',
    sourceLabel: 'Foinse',
    skillCollectionFilters: 'Scagairí bailiúcháin scileanna',
  }),
  'hr-HR': buildLocaleOverrides({
    analysisReport: 'Izvješće o analizi',
    uiReviewer: 'Pregled UI-ja',
    configuration: 'Konfiguracija',
    skillGalleryHint:
      'Pregledavajte vještine kao kartice. Otvorite bilo koju vještinu da pregledate dokumentaciju i upravljate njezinim statusom.',
    toolGalleryHint:
      'Pregledavajte alate kao kartice, provjerite što rade i brzo ih uključite ili isključite.',
    closeSkillDetails: 'Zatvori detalje vještine',
    sourceLabel: 'Izvor',
    skillCollectionFilters: 'Filtri zbirke vještina',
  }),
  'hu-HU': buildLocaleOverrides({
    analysisReport: 'Elemzési jelentés',
    uiReviewer: 'UI-ellenőrzés',
    configuration: 'Konfiguráció',
    skillGalleryHint:
      'Böngéssze a készségeket kártyákként. Nyisson meg bármely készséget a dokumentáció megtekintéséhez és az állapot kezeléséhez.',
    toolGalleryHint:
      'Böngéssze az eszközöket kártyákként, nézze meg, mire valók, és gyorsan kapcsolja be vagy ki őket.',
    closeSkillDetails: 'Készség részleteinek bezárása',
    sourceLabel: 'Forrás',
    skillCollectionFilters: 'Készséggyűjtemény szűrői',
  }),
  'it-IT': buildLocaleOverrides({
    analysisReport: 'Rapporto di analisi',
    uiReviewer: 'Revisione UI',
    configuration: 'Configurazione',
    skillGalleryHint:
      'Sfoglia le abilità come schede. Apri qualsiasi abilità per consultare la documentazione e gestirne lo stato.',
    toolGalleryHint:
      'Sfoglia gli strumenti come schede, scopri cosa fanno e attivali o disattivali rapidamente.',
    closeSkillDetails: 'Chiudi dettagli abilità',
    sourceLabel: 'Fonte',
    skillCollectionFilters: 'Filtri della raccolta abilità',
  }),
  'ja-JP': buildLocaleOverrides({
    analysisReport: '分析レポート',
    uiReviewer: 'UIレビュー',
    configuration: '設定',
    skillGalleryHint:
      'スキルをカード形式で閲覧できます。任意のスキルを開いてドキュメントを確認し、状態を管理できます。',
    toolGalleryHint:
      'ツールをカード形式で閲覧し、役割を確認して、すばやくオン・オフを切り替えられます。',
    closeSkillDetails: 'スキル詳細を閉じる',
    sourceLabel: 'ソース',
    skillCollectionFilters: 'スキルコレクションのフィルター',
  }),
  'ko-KR': buildLocaleOverrides({
    analysisReport: '분석 보고서',
    uiReviewer: 'UI 리뷰',
    configuration: '구성',
    skillGalleryHint:
      '스킬을 카드 형태로 둘러볼 수 있습니다. 원하는 스킬을 열어 문서를 확인하고 상태를 관리하세요.',
    toolGalleryHint:
      '도구를 카드 형태로 둘러보고, 어떤 역할을 하는지 확인한 뒤 빠르게 켜거나 끌 수 있습니다.',
    closeSkillDetails: '스킬 세부 정보 닫기',
    sourceLabel: '출처',
    skillCollectionFilters: '스킬 모음 필터',
  }),
  'ml-IN': buildLocaleOverrides({
    analysisReport: 'വിശകലന റിപ്പോർട്ട്',
    uiReviewer: 'UI അവലോകനം',
    configuration: 'ക്രമീകരണം',
    skillGalleryHint:
      'കൗശലങ്ങൾ കാർഡുകളായി കാണുക. ഏതെങ്കിലും കൗശലം തുറന്ന് ഡോക്യുമെന്റേഷൻ പരിശോധിക്കാനും അതിന്റെ സ്ഥിതി നിയന്ത്രിക്കാനും കഴിയും.',
    toolGalleryHint:
      'ഉപകരണങ്ങളെ കാർഡുകളായി കാണുക, അവ എന്ത് ചെയ്യുന്നുവെന്ന് പരിശോധിക്കുക, പിന്നെ വേഗത്തിൽ ഓണാക്കുകയോ ഓഫ് ചെയ്യുകയോ ചെയ്യുക.',
    closeSkillDetails: 'കൗശലത്തിന്റെ വിശദാംശങ്ങൾ അടയ്ക്കുക',
    sourceLabel: 'ഉറവിടം',
    skillCollectionFilters: 'കൗശൽ ശേഖര ഫിൽട്ടറുകൾ',
  }),
  'nb-NO': buildLocaleOverrides({
    analysisReport: 'Analyserapport',
    uiReviewer: 'UI-gjennomgang',
    configuration: 'Konfigurasjon',
    skillGalleryHint:
      'Bla gjennom ferdigheter som kort. Åpne en ferdighet for å lese dokumentasjonen og administrere statusen.',
    toolGalleryHint:
      'Bla gjennom verktøy som kort, se hva de gjør, og slå dem raskt av eller på.',
    closeSkillDetails: 'Lukk ferdighetsdetaljer',
    sourceLabel: 'Kilde',
    skillCollectionFilters: 'Filtre for ferdighetssamling',
  }),
  'nl-NL': buildLocaleOverrides({
    analysisReport: 'Analyserapport',
    uiReviewer: 'UI-beoordeling',
    configuration: 'Configuratie',
    skillGalleryHint:
      'Blader door vaardigheden als kaarten. Open een vaardigheid om de documentatie te bekijken en de status te beheren.',
    toolGalleryHint:
      'Blader door tools als kaarten, bekijk wat ze doen en schakel ze snel in of uit.',
    closeSkillDetails: 'Vaardigheidsdetails sluiten',
    sourceLabel: 'Bron',
    skillCollectionFilters: 'Filters voor vaardighedencollectie',
  }),
  'pl-PL': buildLocaleOverrides({
    analysisReport: 'Raport z analizy',
    uiReviewer: 'Przegląd UI',
    configuration: 'Konfiguracja',
    skillGalleryHint:
      'Przeglądaj umiejętności jako karty. Otwórz dowolną umiejętność, aby sprawdzić dokumentację i zarządzać jej stanem.',
    toolGalleryHint:
      'Przeglądaj narzędzia jako karty, zobacz do czego służą i szybko je włączaj lub wyłączaj.',
    closeSkillDetails: 'Zamknij szczegóły umiejętności',
    sourceLabel: 'Źródło',
    skillCollectionFilters: 'Filtry kolekcji umiejętności',
  }),
  'pt-BR': buildLocaleOverrides({
    analysisReport: 'Relatório de análise',
    uiReviewer: 'Revisão de UI',
    configuration: 'Configuração',
    skillGalleryHint:
      'Navegue pelas habilidades como cartões. Abra qualquer habilidade para revisar a documentação e gerenciar seu status.',
    toolGalleryHint:
      'Navegue pelas ferramentas como cartões, veja o que fazem e ative ou desative rapidamente.',
    closeSkillDetails: 'Fechar detalhes da habilidade',
    sourceLabel: 'Fonte',
    skillCollectionFilters: 'Filtros da coleção de habilidades',
  }),
  'pt-PT': buildLocaleOverrides({
    analysisReport: 'Relatório de análise',
    uiReviewer: 'Revisão de UI',
    configuration: 'Configuração',
    skillGalleryHint:
      'Navegue pelas habilidades como cartões. Abra qualquer habilidade para rever a documentação e gerir o respetivo estado.',
    toolGalleryHint:
      'Navegue pelas ferramentas como cartões, veja o que fazem e ative-as ou desative-as rapidamente.',
    closeSkillDetails: 'Fechar detalhes da habilidade',
    sourceLabel: 'Fonte',
    skillCollectionFilters: 'Filtros da coleção de habilidades',
  }),
  'ro-RO': buildLocaleOverrides({
    analysisReport: 'Raport de analiză',
    uiReviewer: 'Revizuire UI',
    configuration: 'Configurare',
    skillGalleryHint:
      'Răsfoiți abilitățile ca pe niște carduri. Deschideți orice abilitate pentru a consulta documentația și a-i gestiona starea.',
    toolGalleryHint:
      'Răsfoiți uneltele ca pe niște carduri, vedeți ce fac și activați-le sau dezactivați-le rapid.',
    closeSkillDetails: 'Închide detaliile abilității',
    sourceLabel: 'Sursă',
    skillCollectionFilters: 'Filtrele colecției de abilități',
  }),
  'ru-RU': buildLocaleOverrides({
    analysisReport: 'Отчет об анализе',
    uiReviewer: 'Проверка UI',
    configuration: 'Конфигурация',
    skillGalleryHint:
      'Просматривайте навыки в виде карточек. Откройте любой навык, чтобы изучить документацию и управлять его статусом.',
    toolGalleryHint:
      'Просматривайте инструменты в виде карточек, смотрите, что они делают, и быстро включайте или отключайте их.',
    closeSkillDetails: 'Закрыть сведения о навыке',
    sourceLabel: 'Источник',
    skillCollectionFilters: 'Фильтры коллекции навыков',
  }),
  'sk-SK': buildLocaleOverrides({
    analysisReport: 'Správa o analýze',
    uiReviewer: 'Kontrola UI',
    configuration: 'Konfigurácia',
    skillGalleryHint:
      'Prehliadajte zručnosti ako karty. Otvorte ľubovoľnú zručnosť, zobrazte si dokumentáciu a spravujte jej stav.',
    toolGalleryHint:
      'Prehliadajte nástroje ako karty, pozrite si, čo robia, a rýchlo ich zapínajte alebo vypínajte.',
    closeSkillDetails: 'Zavrieť podrobnosti zručnosti',
    sourceLabel: 'Zdroj',
    skillCollectionFilters: 'Filtre zbierky zručností',
  }),
  'sv-SE': buildLocaleOverrides({
    analysisReport: 'Analysrapport',
    uiReviewer: 'UI-granskning',
    configuration: 'Konfiguration',
    skillGalleryHint:
      'Bläddra bland färdigheter som kort. Öppna valfri färdighet för att läsa dokumentationen och hantera dess status.',
    toolGalleryHint:
      'Bläddra bland verktyg som kort, se vad de gör och slå snabbt på eller av dem.',
    closeSkillDetails: 'Stäng färdighetsdetaljer',
    sourceLabel: 'Källa',
    skillCollectionFilters: 'Filter för färdighetssamling',
  }),
  'zh-CN': buildLocaleOverrides({
    analysisReport: '分析报告',
    uiReviewer: 'UI 评审',
    configuration: '配置',
    skillGalleryHint: '以卡片方式浏览技能，点击任意技能查看说明文档并管理启用状态。',
    toolGalleryHint: '以卡片方式浏览工具，快速查看用途，并直接启用或停用。',
    closeSkillDetails: '关闭技能详情',
    sourceLabel: '来源',
    skillCollectionFilters: '技能榜单筛选',
  }),
  'zh-TW': buildLocaleOverrides({
    analysisReport: '分析報告',
    uiReviewer: 'UI 審查',
    configuration: '設定',
    skillGalleryHint: '以卡片方式瀏覽技能，點開任一技能即可查看說明文件並管理啟用狀態。',
    toolGalleryHint: '以卡片方式瀏覽工具，快速查看用途，並直接啟用或停用。',
    closeSkillDetails: '關閉技能詳情',
    sourceLabel: '來源',
    skillCollectionFilters: '技能榜單篩選',
  }),
}
