import type { LocaleKey } from './locale-catalog'

type LocaleLeaf = string | number | boolean | null
type LocaleNode = { [key: string]: LocaleLeaf | LocaleNode }

const visibleBuiltinToolLocaleTerms = {
  'ca-ES': {
    findName: 'Cerca',
    lsName: 'Llista',
    toolSearchName: "Cerca d'eines",
    findDescription: 'Cerca fitxers i directoris per patró glob',
    lsDescription: 'Llista fitxers i directoris',
    toolSearchDescription: 'Cerca eines, habilitats i agents per capacitat',
  },
  'cs-CZ': {
    findName: 'Najít',
    lsName: 'Seznam',
    toolSearchName: 'Hledání nástrojů',
    findDescription: 'Najde soubory a adresáře podle vzoru glob',
    lsDescription: 'Vypíše soubory a adresáře',
    toolSearchDescription: 'Vyhledává nástroje, dovednosti a agenty podle schopností',
  },
  'da-DK': {
    findName: 'Søg',
    lsName: 'Liste',
    toolSearchName: 'Værktøjssøgning',
    findDescription: 'Søg efter filer og mapper med glob-mønstre',
    lsDescription: 'Vis filer og mapper',
    toolSearchDescription: 'Søg efter værktøjer, færdigheder og agenter efter kapacitet',
  },
  'de-DE': {
    findName: 'Finden',
    lsName: 'Auflisten',
    toolSearchName: 'Werkzeugsuche',
    findDescription: 'Dateien und Verzeichnisse per Glob-Muster finden',
    lsDescription: 'Dateien und Verzeichnisse auflisten',
    toolSearchDescription: 'Werkzeuge, Skills und Agenten nach Fähigkeiten suchen',
  },
  'el-GR': {
    findName: 'Εύρεση',
    lsName: 'Λίστα',
    toolSearchName: 'Αναζήτηση εργαλείων',
    findDescription: 'Εντοπισμός αρχείων και καταλόγων με μοτίβο glob',
    lsDescription: 'Λίστα αρχείων και καταλόγων',
    toolSearchDescription: 'Αναζήτηση εργαλείων, δεξιοτήτων και πρακτόρων ανά δυνατότητα',
  },
  'en-GB': {
    findName: 'Find',
    lsName: 'List',
    toolSearchName: 'Tool Search',
    findDescription: 'Find files and directories by glob pattern',
    lsDescription: 'List files and directories',
    toolSearchDescription: 'Search tools, skills, and agents by capability',
  },
  'en-US': {
    findName: 'Find',
    lsName: 'List',
    toolSearchName: 'Tool Search',
    findDescription: 'Find files and directories by glob pattern',
    lsDescription: 'List files and directories',
    toolSearchDescription: 'Search tools, skills, and agents by capability',
  },
  'es-ES': {
    findName: 'Buscar',
    lsName: 'Listar',
    toolSearchName: 'Buscar herramientas',
    findDescription: 'Buscar archivos y directorios por patrón glob',
    lsDescription: 'Listar archivos y directorios',
    toolSearchDescription: 'Buscar herramientas, habilidades y agentes por capacidad',
  },
  'fr-FR': {
    findName: 'Rechercher',
    lsName: 'Lister',
    toolSearchName: "Recherche d'outils",
    findDescription: 'Rechercher des fichiers et dossiers par motif glob',
    lsDescription: 'Lister les fichiers et dossiers',
    toolSearchDescription: 'Rechercher des outils, compétences et agents par capacité',
  },
  'ga-IE': {
    findName: 'Aimsigh',
    lsName: 'Liosta',
    toolSearchName: 'Cuardach uirlisí',
    findDescription: 'Aimsigh comhaid agus fillteáin de réir patrún glob',
    lsDescription: 'Liostaigh comhaid agus fillteáin',
    toolSearchDescription: 'Cuardaigh uirlisí, scileanna agus gníomhairí de réir cumais',
  },
  'hr-HR': {
    findName: 'Pronađi',
    lsName: 'Popis',
    toolSearchName: 'Pretraga alata',
    findDescription: 'Pronađi datoteke i direktorije prema glob uzorku',
    lsDescription: 'Prikaži datoteke i direktorije',
    toolSearchDescription: 'Pretraži alate, vještine i agente prema mogućnostima',
  },
  'hu-HU': {
    findName: 'Keresés',
    lsName: 'Listázás',
    toolSearchName: 'Eszközkeresés',
    findDescription: 'Fájlok és könyvtárak keresése glob minta alapján',
    lsDescription: 'Fájlok és könyvtárak listázása',
    toolSearchDescription: 'Eszközök, készségek és ügynökök keresése képesség alapján',
  },
  'it-IT': {
    findName: 'Trova',
    lsName: 'Elenca',
    toolSearchName: 'Ricerca strumenti',
    findDescription: 'Trova file e directory per pattern glob',
    lsDescription: 'Elenca file e directory',
    toolSearchDescription: 'Cerca strumenti, competenze e agenti per capacità',
  },
  'ja-JP': {
    findName: '検索',
    lsName: '一覧',
    toolSearchName: 'ツール検索',
    findDescription: 'glob パターンでファイルとディレクトリを検索します',
    lsDescription: 'ファイルとディレクトリを一覧表示します',
    toolSearchDescription: '機能別にツール、スキル、エージェントを検索します',
  },
  'ko-KR': {
    findName: '찾기',
    lsName: '목록',
    toolSearchName: '도구 검색',
    findDescription: 'glob 패턴으로 파일과 디렉터리를 찾습니다',
    lsDescription: '파일과 디렉터리를 나열합니다',
    toolSearchDescription: '기능별로 도구, 스킬, 에이전트를 검색합니다',
  },
  'ml-IN': {
    findName: 'കണ്ടെത്തുക',
    lsName: 'പട്ടിക',
    toolSearchName: 'ഉപകരണ തിരയൽ',
    findDescription: 'glob മാതൃക ഉപയോഗിച്ച് ഫയലുകളും ഡയറക്ടറികളും കണ്ടെത്തുക',
    lsDescription: 'ഫയലുകളും ഡയറക്ടറികളും പട്ടികപ്പെടുത്തുക',
    toolSearchDescription: 'ശേഷി അനുസരിച്ച് ഉപകരണങ്ങളും സ്കില്ലുകളും ഏജന്റുകളും തിരയുക',
  },
  'nb-NO': {
    findName: 'Finn',
    lsName: 'List opp',
    toolSearchName: 'Verktøysøk',
    findDescription: 'Finn filer og kataloger etter glob-mønster',
    lsDescription: 'List opp filer og kataloger',
    toolSearchDescription: 'Søk etter verktøy, ferdigheter og agenter etter kapasitet',
  },
  'nl-NL': {
    findName: 'Zoeken',
    lsName: 'Lijst',
    toolSearchName: 'Zoeken naar tools',
    findDescription: 'Zoek bestanden en mappen op glob-patroon',
    lsDescription: 'Lijst bestanden en mappen op',
    toolSearchDescription: 'Zoek tools, vaardigheden en agents op basis van mogelijkheden',
  },
  'pl-PL': {
    findName: 'Znajdź',
    lsName: 'Lista',
    toolSearchName: 'Wyszukiwanie narzędzi',
    findDescription: 'Znajdź pliki i katalogi według wzorca glob',
    lsDescription: 'Wyświetl pliki i katalogi',
    toolSearchDescription: 'Wyszukuj narzędzia, umiejętności i agentów według możliwości',
  },
  'pt-BR': {
    findName: 'Buscar',
    lsName: 'Listar',
    toolSearchName: 'Busca de ferramentas',
    findDescription: 'Buscar arquivos e diretórios por padrão glob',
    lsDescription: 'Listar arquivos e diretórios',
    toolSearchDescription: 'Buscar ferramentas, habilidades e agentes por capacidade',
  },
  'pt-PT': {
    findName: 'Pesquisar',
    lsName: 'Listar',
    toolSearchName: 'Pesquisa de ferramentas',
    findDescription: 'Procurar ficheiros e diretórios por padrão glob',
    lsDescription: 'Listar ficheiros e diretórios',
    toolSearchDescription: 'Pesquisar ferramentas, competências e agentes por capacidade',
  },
  'ro-RO': {
    findName: 'Căutare',
    lsName: 'Listare',
    toolSearchName: 'Căutare unelte',
    findDescription: 'Găsește fișiere și directoare după model glob',
    lsDescription: 'Listează fișiere și directoare',
    toolSearchDescription: 'Caută unelte, abilități și agenți după capacitate',
  },
  'ru-RU': {
    findName: 'Поиск',
    lsName: 'Список',
    toolSearchName: 'Поиск инструментов',
    findDescription: 'Ищет файлы и каталоги по шаблону glob',
    lsDescription: 'Показывает список файлов и каталогов',
    toolSearchDescription: 'Ищет инструменты, навыки и агентов по возможностям',
  },
  'sk-SK': {
    findName: 'Nájsť',
    lsName: 'Zoznam',
    toolSearchName: 'Hľadanie nástrojov',
    findDescription: 'Nájde súbory a priečinky podľa glob vzoru',
    lsDescription: 'Vypíše súbory a priečinky',
    toolSearchDescription: 'Vyhľadáva nástroje, zručnosti a agentov podľa schopností',
  },
  'sv-SE': {
    findName: 'Hitta',
    lsName: 'Lista',
    toolSearchName: 'Verktygssökning',
    findDescription: 'Hitta filer och kataloger med glob-mönster',
    lsDescription: 'Lista filer och kataloger',
    toolSearchDescription: 'Sök efter verktyg, färdigheter och agenter efter kapacitet',
  },
  'zh-CN': {
    findName: '查找',
    lsName: '列表',
    toolSearchName: '工具搜索',
    findDescription: '按 glob 模式查找文件和目录',
    lsDescription: '列出文件和目录',
    toolSearchDescription: '按能力搜索工具、技能和代理',
  },
  'zh-TW': {
    findName: '查找',
    lsName: '列表',
    toolSearchName: '工具搜尋',
    findDescription: '依 glob 模式查找檔案與目錄',
    lsDescription: '列出檔案與目錄',
    toolSearchDescription: '依能力搜尋工具、技能與代理',
  },
} as const satisfies Record<
  LocaleKey,
  {
    findName: string
    lsName: string
    toolSearchName: string
    findDescription: string
    lsDescription: string
    toolSearchDescription: string
  }
>

const builtinToolBackfills = Object.fromEntries(
  Object.entries(visibleBuiltinToolLocaleTerms).map(([locale, terms]) => [
    locale,
    {
      tools: {
        names: {
          find: terms.findName,
          ls: terms.lsName,
          tool_search: terms.toolSearchName,
        },
        descriptions: {
          find: terms.findDescription,
          ls: terms.lsDescription,
          tool_search: terms.toolSearchDescription,
        },
      },
    },
  ])
) as Partial<Record<LocaleKey, LocaleNode>>

export default builtinToolBackfills
