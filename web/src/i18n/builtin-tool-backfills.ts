import type { LocaleKey } from './locale-catalog'
import nativeDocumentToolBackfills from './native-document-tool-backfills'

type LocaleLeaf = string | number | boolean | null
type LocaleNode = { [key: string]: LocaleLeaf | LocaleNode }

const visibleBuiltinToolLocaleTerms = {
  'ca-ES': {
    findName: 'Cerca',
    lsName: 'Llista',
    toolSearchName: "Cerca d'eines",
    agentsListName: 'Agents',
    bashName: 'Bash',
    canvasName: 'Canvas',
    editName: 'Editar',
    gatewayName: 'Passarel·la',
    grepName: 'Cerca de text',
    imageName: 'Imatge',
    nodesName: 'Nodes',
    officeName: 'Ofimàtica',
    pdfName: 'PDF',
    subagentsName: 'Subagents',
    ttsName: 'Text a veu',
    findDescription: 'Cerca fitxers i directoris per patró glob',
    lsDescription: 'Llista fitxers i directoris',
    toolSearchDescription: 'Cerca eines, habilitats i agents per capacitat',
    agentsListDescription: 'Llista els agents configurats i la configuració predeterminada.',
    bashDescription: 'Executa una ordre de shell i retorna stdout, stderr i el codi de sortida.',
    canvasDescription: "Crea i gestiona canvases lleugers d'Agent-to-UI.",
    editDescription: 'Fa una substitució precisa de text dins d’un fitxer.',
    gatewayDescription:
      'Inspecciona l’estat de la passarel·la i les connexions actives, i pot tancar connexions bloquejades.',
    grepDescription: 'Cerca un patró de text entre fitxers.',
    imageDescription:
      'Genera imatges, revisa entrades d’imatge o crea recursos de diapositives PPT.',
    nodesDescription: 'Gestiona fluxos de treball i automatitzacions basades en nodes.',
    officeDescription: 'Crea fulls de càlcul .xlsx i informes .docx amb tema.',
    pdfDescription: 'Llegeix metadades PDF o extreu text de PDFs locals o remots.',
    subagentsDescription: 'Inspecciona la política de subagents o crea un agent fill.',
    ttsDescription: 'Genera àudio de veu, mostra veus i estat, o atura la reproducció.',
    mcpDescription: 'Crida eines mitjançant el despatxador MCP unificat.',
    memoryDescription: 'Cerca i gestiona la memòria personal desada.',
  },
  'cs-CZ': {
    findName: 'Najít',
    lsName: 'Seznam',
    toolSearchName: 'Hledání nástrojů',
    agentsListName: 'Agenti',
    bashName: 'Bash',
    canvasName: 'Plátno',
    editName: 'Upravit',
    gatewayName: 'Brána',
    grepName: 'Hledání textu',
    imageName: 'Obrázek',
    nodesName: 'Uzly',
    officeName: 'Office',
    pdfName: 'PDF',
    subagentsName: 'Subagenti',
    ttsName: 'Převod textu na řeč',
    findDescription: 'Najde soubory a adresáře podle vzoru glob',
    lsDescription: 'Vypíše soubory a adresáře',
    toolSearchDescription: 'Vyhledává nástroje, dovednosti a agenty podle schopností',
    agentsListDescription: 'Vypíše nakonfigurované agenty a výchozí nastavení.',
    bashDescription: 'Spustí shellový příkaz a vrátí stdout, stderr a návratový kód.',
    canvasDescription: 'Vytváří a spravuje lehká plátna Agent-to-UI.',
    editDescription: 'Provede přesnou náhradu textu přímo v souboru.',
    gatewayDescription: 'Zobrazí stav brány a aktivní spojení a může zavřít zaseknutá spojení.',
    grepDescription: 'Vyhledá textový vzor napříč soubory.',
    imageDescription:
      'Generuje obrázky, kontroluje obrazové vstupy nebo vytváří podklady pro snímky PPT.',
    nodesDescription: 'Spravuje grafy workflow a automatizace založené na uzlech.',
    officeDescription: 'Vytváří tematické tabulky .xlsx a zprávy .docx.',
    pdfDescription: 'Čte metadata PDF nebo extrahuje text z místních či vzdálených PDF.',
    subagentsDescription: 'Zobrazí zásady subagentů nebo spustí podřízeného agenta.',
    ttsDescription: 'Generuje řečový zvuk, zobrazuje hlasy a stav nebo zastaví přehrávání.',
    mcpDescription: 'Volá nástroje přes sjednocený MCP dispatcher.',
    memoryDescription: 'Vyhledává a spravuje uloženou osobní paměť.',
  },
  'da-DK': {
    findName: 'Søg',
    lsName: 'Liste',
    toolSearchName: 'Værktøjssøgning',
    agentsListName: 'Agenter',
    bashName: 'Bash',
    canvasName: 'Canvas',
    editName: 'Rediger',
    gatewayName: 'Gateway',
    grepName: 'Tekstsøgning',
    imageName: 'Billede',
    nodesName: 'Noder',
    officeName: 'Office',
    pdfName: 'PDF',
    subagentsName: 'Underagenter',
    ttsName: 'Tekst til tale',
    findDescription: 'Søg efter filer og mapper med glob-mønstre',
    lsDescription: 'Vis filer og mapper',
    toolSearchDescription: 'Søg efter værktøjer, færdigheder og agenter efter kapacitet',
    agentsListDescription: 'Viser konfigurerede agenter og standardindstillinger.',
    bashDescription: 'Kører en shell-kommando og returnerer stdout, stderr og exit-kode.',
    canvasDescription: 'Opretter og administrerer lette Agent-to-UI-canvasser.',
    editDescription: 'Foretager en præcis tekstudskiftning direkte i en fil.',
    gatewayDescription:
      'Inspicerer gateway-status og aktive forbindelser og kan lukke fastlåste forbindelser.',
    grepDescription: 'Søger efter et tekstmønster på tværs af filer.',
    imageDescription: 'Genererer billeder, gennemgår billedinput eller bygger PPT-slideaktiver.',
    nodesDescription: 'Administrerer workflow-grafer og nodebaserede automatiseringer.',
    officeDescription: 'Opretter tematiserede .xlsx-regneark og .docx-rapporter.',
    pdfDescription: 'Læser PDF-metadata eller udtrækker tekst fra lokale eller eksterne PDF-filer.',
    subagentsDescription: 'Inspicerer underagentpolitik eller starter en underagent.',
    ttsDescription: 'Genererer tale, viser stemmer og status eller stopper afspilning.',
    mcpDescription: 'Kalder værktøjer gennem den samlede MCP-dispatcher.',
    memoryDescription: 'Søger og administrerer gemt personlig hukommelse.',
  },
  'de-DE': {
    findName: 'Finden',
    lsName: 'Auflisten',
    toolSearchName: 'Werkzeugsuche',
    agentsListName: 'Agenten',
    bashName: 'Bash',
    canvasName: 'Canvas',
    editName: 'Bearbeiten',
    gatewayName: 'Gateway',
    grepName: 'Textsuche',
    imageName: 'Bild',
    nodesName: 'Knoten',
    officeName: 'Office',
    pdfName: 'PDF',
    subagentsName: 'Subagenten',
    ttsName: 'Text-zu-Sprache',
    findDescription: 'Dateien und Verzeichnisse per Glob-Muster finden',
    lsDescription: 'Dateien und Verzeichnisse auflisten',
    toolSearchDescription: 'Werkzeuge, Skills und Agenten nach Fähigkeiten suchen',
    agentsListDescription: 'Listet konfigurierte Agenten und Standardeinstellungen auf.',
    bashDescription:
      'Führt einen Shell-Befehl aus und gibt stdout, stderr und den Exit-Code zurück.',
    canvasDescription: 'Erstellt und verwaltet leichtgewichtige Agent-to-UI-Canvases.',
    editDescription: 'Ersetzt Text präzise direkt in einer Datei.',
    gatewayDescription:
      'Prüft Gateway-Status und aktive Verbindungen und kann blockierte Verbindungen schließen.',
    grepDescription: 'Sucht dateiübergreifend nach einem Textmuster.',
    imageDescription: 'Erzeugt Bilder, prüft Bildeingaben oder erstellt Assets für PPT-Folien.',
    nodesDescription: 'Verwaltet Workflow-Graphen und nodebasierte Automatisierungen.',
    officeDescription: 'Erstellt thematisierte .xlsx-Tabellen und .docx-Berichte.',
    pdfDescription:
      'Liest PDF-Metadaten oder extrahiert Text aus lokalen oder entfernten PDF-Dateien.',
    subagentsDescription: 'Prüft Subagenten-Richtlinien oder startet einen Child-Agenten.',
    ttsDescription: 'Erzeugt Sprachaudio, zeigt Stimmen und Status an oder stoppt die Wiedergabe.',
    mcpDescription: 'Ruft Werkzeuge über den einheitlichen MCP-Dispatcher auf.',
    memoryDescription: 'Durchsucht und verwaltet gespeicherte persönliche Erinnerungen.',
  },
  'el-GR': {
    findName: 'Εύρεση',
    lsName: 'Λίστα',
    toolSearchName: 'Αναζήτηση εργαλείων',
    agentsListName: 'Πράκτορες',
    bashName: 'Bash',
    canvasName: 'Καμβάς',
    editName: 'Επεξεργασία',
    gatewayName: 'Πύλη',
    grepName: 'Αναζήτηση κειμένου',
    imageName: 'Εικόνα',
    nodesName: 'Κόμβοι',
    officeName: 'Office',
    pdfName: 'PDF',
    subagentsName: 'Υποπράκτορες',
    ttsName: 'Κείμενο σε ομιλία',
    findDescription: 'Εντοπισμός αρχείων και καταλόγων με μοτίβο glob',
    lsDescription: 'Λίστα αρχείων και καταλόγων',
    toolSearchDescription: 'Αναζήτηση εργαλείων, δεξιοτήτων και πρακτόρων ανά δυνατότητα',
    agentsListDescription: 'Παραθέτει τους ρυθμισμένους πράκτορες και τις προεπιλογές.',
    bashDescription: 'Εκτελεί μια εντολή shell και επιστρέφει stdout, stderr και κωδικό εξόδου.',
    canvasDescription: 'Δημιουργεί και διαχειρίζεται ελαφριούς καμβάδες Agent-to-UI.',
    editDescription: 'Κάνει ακριβή αντικατάσταση κειμένου μέσα σε αρχείο.',
    gatewayDescription:
      'Ελέγχει την κατάσταση της πύλης και τις ενεργές συνδέσεις και μπορεί να κλείσει κολλημένες συνδέσεις.',
    grepDescription: 'Αναζητά μοτίβο κειμένου σε αρχεία.',
    imageDescription:
      'Δημιουργεί εικόνες, εξετάζει εισόδους εικόνας ή δημιουργεί υλικά διαφανειών PPT.',
    nodesDescription:
      'Διαχειρίζεται γράφους ροών εργασίας και αυτοματισμούς βασισμένους σε κόμβους.',
    officeDescription: 'Δημιουργεί θεματικά υπολογιστικά φύλλα .xlsx και αναφορές .docx.',
    pdfDescription: 'Διαβάζει μεταδεδομένα PDF ή εξάγει κείμενο από τοπικά ή απομακρυσμένα PDF.',
    subagentsDescription: 'Ελέγχει την πολιτική υποπρακτόρων ή εκκινεί έναν θυγατρικό πράκτορα.',
    ttsDescription: 'Παράγει ήχο ομιλίας, εμφανίζει φωνές και κατάσταση ή σταματά την αναπαραγωγή.',
    mcpDescription: 'Καλεί εργαλεία μέσω του ενοποιημένου MCP dispatcher.',
    memoryDescription: 'Αναζητά και διαχειρίζεται αποθηκευμένη προσωπική μνήμη.',
  },
  'en-GB': {
    findName: 'Find',
    lsName: 'List',
    toolSearchName: 'Tool Search',
    agentsListName: 'Agents',
    bashName: 'Bash',
    canvasName: 'Canvas',
    editName: 'Edit',
    gatewayName: 'Gateway',
    grepName: 'Text Search',
    imageName: 'Image',
    nodesName: 'Nodes',
    officeName: 'Office',
    pdfName: 'PDF',
    subagentsName: 'Subagents',
    ttsName: 'Text-to-Speech',
    findDescription: 'Find files and directories by glob pattern',
    lsDescription: 'List files and directories',
    toolSearchDescription: 'Search tools, skills, and agents by capability',
    agentsListDescription: 'List configured agents and default settings.',
    bashDescription: 'Run a shell command and return stdout, stderr, and exit code.',
    canvasDescription: 'Create and manage lightweight Agent-to-UI canvases.',
    editDescription: 'Make a precise in-place text replacement in a file.',
    gatewayDescription:
      'Inspect gateway status and active connections, and close stuck connections.',
    grepDescription: 'Search for a text pattern across files.',
    imageDescription: 'Generate images, review image inputs, or build PPT slide assets.',
    nodesDescription: 'Manage workflow graphs and node-based automations.',
    officeDescription: 'Create themed .xlsx spreadsheets and .docx reports.',
    pdfDescription: 'Read PDF metadata or extract text from local or remote PDFs.',
    subagentsDescription: 'Inspect subagent policy or spawn a child agent.',
    ttsDescription: 'Generate speech audio, inspect voices and status, or stop playback.',
    mcpDescription: 'Call tools through the unified MCP dispatcher.',
    memoryDescription: 'Search and manage saved personal memory.',
  },
  'en-US': {
    findName: 'Find',
    lsName: 'List',
    toolSearchName: 'Tool Search',
    agentsListName: 'Agents',
    bashName: 'Bash',
    canvasName: 'Canvas',
    editName: 'Edit',
    gatewayName: 'Gateway',
    grepName: 'Text Search',
    imageName: 'Image',
    nodesName: 'Nodes',
    officeName: 'Office',
    pdfName: 'PDF',
    subagentsName: 'Subagents',
    ttsName: 'Text-to-Speech',
    findDescription: 'Find files and directories by glob pattern',
    lsDescription: 'List files and directories',
    toolSearchDescription: 'Search tools, skills, and agents by capability',
    agentsListDescription: 'List configured agents and default settings.',
    bashDescription: 'Run a shell command and return stdout, stderr, and exit code.',
    canvasDescription: 'Create and manage lightweight Agent-to-UI canvases.',
    editDescription: 'Make a precise in-place text replacement in a file.',
    gatewayDescription:
      'Inspect gateway status and active connections, and close stuck connections.',
    grepDescription: 'Search for a text pattern across files.',
    imageDescription: 'Generate images, review image inputs, or build PPT slide assets.',
    nodesDescription: 'Manage workflow graphs and node-based automations.',
    officeDescription: 'Create themed .xlsx spreadsheets and .docx reports.',
    pdfDescription: 'Read PDF metadata or extract text from local or remote PDFs.',
    subagentsDescription: 'Inspect subagent policy or spawn a child agent.',
    ttsDescription: 'Generate speech audio, inspect voices and status, or stop playback.',
    mcpDescription: 'Call tools through the unified MCP dispatcher.',
    memoryDescription: 'Search and manage saved personal memory.',
  },
  'es-ES': {
    findName: 'Buscar',
    lsName: 'Listar',
    toolSearchName: 'Buscar herramientas',
    agentsListName: 'Agentes',
    bashName: 'Bash',
    canvasName: 'Lienzo',
    editName: 'Editar',
    gatewayName: 'Puerta de enlace',
    grepName: 'Búsqueda de texto',
    imageName: 'Imagen',
    nodesName: 'Nodos',
    officeName: 'Ofimática',
    pdfName: 'PDF',
    subagentsName: 'Subagentes',
    ttsName: 'Texto a voz',
    findDescription: 'Buscar archivos y directorios por patrón glob',
    lsDescription: 'Listar archivos y directorios',
    toolSearchDescription: 'Buscar herramientas, habilidades y agentes por capacidad',
    agentsListDescription: 'Enumera los agentes configurados y los ajustes predeterminados.',
    bashDescription: 'Ejecuta un comando de shell y devuelve stdout, stderr y el código de salida.',
    canvasDescription: 'Crea y gestiona lienzos ligeros de Agent-to-UI.',
    editDescription: 'Hace un reemplazo preciso de texto dentro de un archivo.',
    gatewayDescription:
      'Inspecciona el estado de la puerta de enlace y las conexiones activas, y puede cerrar conexiones bloqueadas.',
    grepDescription: 'Busca un patrón de texto en archivos.',
    imageDescription:
      'Genera imágenes, revisa entradas de imagen o crea recursos para diapositivas PPT.',
    nodesDescription: 'Gestiona grafos de flujos de trabajo y automatizaciones basadas en nodos.',
    officeDescription: 'Crea hojas de cálculo .xlsx e informes .docx con tema.',
    pdfDescription: 'Lee metadatos PDF o extrae texto de PDF locales o remotos.',
    subagentsDescription: 'Inspecciona la política de subagentes o crea un agente hijo.',
    ttsDescription: 'Genera audio de voz, muestra voces y estado, o detiene la reproducción.',
    mcpDescription: 'Llama herramientas mediante el despachador MCP unificado.',
    memoryDescription: 'Busca y gestiona la memoria personal guardada.',
  },
  'fr-FR': {
    findName: 'Rechercher',
    lsName: 'Lister',
    toolSearchName: "Recherche d'outils",
    agentsListName: 'Agents',
    bashName: 'Bash',
    canvasName: 'Canevas',
    editName: 'Modifier',
    gatewayName: 'Passerelle',
    grepName: 'Recherche de texte',
    imageName: 'Image',
    nodesName: 'Nœuds',
    officeName: 'Bureautique',
    pdfName: 'PDF',
    subagentsName: 'Sous-agents',
    ttsName: 'Synthèse vocale',
    findDescription: 'Rechercher des fichiers et dossiers par motif glob',
    lsDescription: 'Lister les fichiers et dossiers',
    toolSearchDescription: 'Rechercher des outils, compétences et agents par capacité',
    agentsListDescription: 'Liste les agents configurés et les paramètres par défaut.',
    bashDescription: 'Exécute une commande shell et renvoie stdout, stderr et le code de sortie.',
    canvasDescription: 'Crée et gère des canevas Agent-to-UI légers.',
    editDescription: 'Effectue un remplacement précis de texte dans un fichier.',
    gatewayDescription:
      'Inspecte le statut de la passerelle et les connexions actives, et peut fermer les connexions bloquées.',
    grepDescription: 'Recherche un motif de texte dans les fichiers.',
    imageDescription:
      'Génère des images, examine des entrées d’image ou crée des ressources pour des diapositives PPT.',
    nodesDescription: 'Gère des graphes de workflow et des automatisations basées sur des nœuds.',
    officeDescription: 'Crée des feuilles .xlsx et des rapports .docx avec thème.',
    pdfDescription: 'Lit les métadonnées PDF ou extrait le texte de PDF locaux ou distants.',
    subagentsDescription: 'Inspecte la politique des sous-agents ou lance un agent enfant.',
    ttsDescription: 'Génère de la voix, affiche les voix et le statut, ou arrête la lecture.',
    mcpDescription: 'Appelle des outils via le répartiteur MCP unifié.',
    memoryDescription: 'Recherche et gère la mémoire personnelle enregistrée.',
  },
  'ga-IE': {
    findName: 'Aimsigh',
    lsName: 'Liosta',
    toolSearchName: 'Cuardach uirlisí',
    agentsListName: 'Gníomhairí',
    bashName: 'Bash',
    canvasName: 'Canbhás',
    editName: 'Cuir in eagar',
    gatewayName: 'Geata',
    grepName: 'Cuardach téacs',
    imageName: 'Íomhá',
    nodesName: 'Nóid',
    officeName: 'Oifig',
    pdfName: 'PDF',
    subagentsName: 'Fo-ghníomhairí',
    ttsName: 'Téacs go caint',
    findDescription: 'Aimsigh comhaid agus fillteáin de réir patrún glob',
    lsDescription: 'Liostaigh comhaid agus fillteáin',
    toolSearchDescription: 'Cuardaigh uirlisí, scileanna agus gníomhairí de réir cumais',
    agentsListDescription: 'Liostaíonn sé gníomhairí cumraithe agus socruithe réamhshocraithe.',
    bashDescription: 'Ritheann sé ordú sliogáin agus filleann sé stdout, stderr agus cód scoir.',
    canvasDescription: 'Cruthaíonn agus bainistíonn sé canbháis éadroma Agent-to-UI.',
    editDescription: 'Déanann sé athsholáthar cruinn téacs i gcomhad.',
    gatewayDescription:
      'Scrúdaíonn sé stádas an gheata agus na naisc ghníomhacha, agus féadann sé naisc bhfostaithe a dhúnadh.',
    grepDescription: 'Cuardaíonn sé patrún téacs ar fud comhad.',
    imageDescription:
      'Gineann sé íomhánna, déanann sé athbhreithniú ar ionchuir íomhá nó cruthaíonn sé sócmhainní sleamhnán PPT.',
    nodesDescription: 'Bainistíonn sé graif sreafa oibre agus uathoibrithe bunaithe ar nóid.',
    officeDescription: 'Cruthaíonn sé scarbhileoga .xlsx agus tuarascálacha .docx le téama.',
    pdfDescription: 'Léann sé meiteashonraí PDF nó baintear téacs as PDFanna áitiúla nó cianda.',
    subagentsDescription: 'Scrúdaíonn sé polasaí fo-ghníomhairí nó gineann sé gníomhaire linbh.',
    ttsDescription:
      'Gineann sé fuaim chainte, taispeánann sé guthanna agus stádas, nó stopann sé seinm.',
    mcpDescription: 'Glaonn sé ar uirlisí tríd an seoltóir MCP aontaithe.',
    memoryDescription: 'Cuardaíonn agus bainistíonn sé cuimhne phearsanta shábháilte.',
  },
  'hr-HR': {
    findName: 'Pronađi',
    lsName: 'Popis',
    toolSearchName: 'Pretraga alata',
    agentsListName: 'Agenti',
    bashName: 'Bash',
    canvasName: 'Platno',
    editName: 'Uredi',
    gatewayName: 'Pristupnik',
    grepName: 'Pretraživanje teksta',
    imageName: 'Slika',
    nodesName: 'Čvorovi',
    officeName: 'Office',
    pdfName: 'PDF',
    subagentsName: 'Podagenti',
    ttsName: 'Tekst u govor',
    findDescription: 'Pronađi datoteke i direktorije prema glob uzorku',
    lsDescription: 'Prikaži datoteke i direktorije',
    toolSearchDescription: 'Pretraži alate, vještine i agente prema mogućnostima',
    agentsListDescription: 'Prikazuje konfigurirane agente i zadane postavke.',
    bashDescription: 'Pokreće shell naredbu i vraća stdout, stderr i izlazni kod.',
    canvasDescription: 'Stvara i upravlja laganim Agent-to-UI platnima.',
    editDescription: 'Radi preciznu zamjenu teksta unutar datoteke.',
    gatewayDescription:
      'Provjerava stanje pristupnika i aktivne veze te može zatvoriti zaglavljene veze.',
    grepDescription: 'Pretražuje uzorak teksta kroz datoteke.',
    imageDescription:
      'Generira slike, pregledava slikovne ulaze ili izrađuje resurse za PPT slajdove.',
    nodesDescription: 'Upravlja grafovima tijeka rada i automatizacijama temeljenima na čvorovima.',
    officeDescription: 'Stvara tematske .xlsx proračunske tablice i .docx izvještaje.',
    pdfDescription: 'Čita PDF metapodatke ili izvlači tekst iz lokalnih ili udaljenih PDF-ova.',
    subagentsDescription: 'Provjerava pravila podagenata ili pokreće podređenog agenta.',
    ttsDescription: 'Generira govor, prikazuje glasove i status ili zaustavlja reprodukciju.',
    mcpDescription: 'Poziva alate kroz objedinjeni MCP posrednik.',
    memoryDescription: 'Pretražuje i upravlja spremljenom osobnom memorijom.',
  },
  'hu-HU': {
    findName: 'Keresés',
    lsName: 'Listázás',
    toolSearchName: 'Eszközkeresés',
    agentsListName: 'Ügynökök',
    bashName: 'Bash',
    canvasName: 'Vászon',
    editName: 'Szerkesztés',
    gatewayName: 'Átjáró',
    grepName: 'Szövegkeresés',
    imageName: 'Kép',
    nodesName: 'Csomópontok',
    officeName: 'Office',
    pdfName: 'PDF',
    subagentsName: 'Alügynökök',
    ttsName: 'Szövegfelolvasás',
    findDescription: 'Fájlok és könyvtárak keresése glob minta alapján',
    lsDescription: 'Fájlok és könyvtárak listázása',
    toolSearchDescription: 'Eszközök, készségek és ügynökök keresése képesség alapján',
    agentsListDescription:
      'Felsorolja a beállított ügynököket és az alapértelmezett beállításokat.',
    bashDescription:
      'Shell parancsot futtat, majd visszaadja a stdout, stderr és a kilépési kód értékét.',
    canvasDescription: 'Könnyű Agent-to-UI vásznakat hoz létre és kezel.',
    editDescription: 'Pontos szövegcserét végez közvetlenül egy fájlban.',
    gatewayDescription:
      'Megvizsgálja az átjáró állapotát és az aktív kapcsolatokat, és lezárhatja a beragadt kapcsolatokat.',
    grepDescription: 'Szövegmintát keres a fájlok között.',
    imageDescription:
      'Képeket generál, képbemeneteket vizsgál át, vagy PPT-diákhoz készít erőforrásokat.',
    nodesDescription: 'Munkafolyamat-gráfokat és csomópont alapú automatizálásokat kezel.',
    officeDescription: 'Tematikus .xlsx táblázatokat és .docx jelentéseket készít.',
    pdfDescription: 'PDF metaadatokat olvas vagy szöveget emel ki helyi vagy távoli PDF-ekből.',
    subagentsDescription: 'Áttekinti az alügynök-szabályokat vagy elindít egy gyermek ügynököt.',
    ttsDescription:
      'Beszédhangot generál, megjeleníti a hangokat és az állapotot, vagy leállítja a lejátszást.',
    mcpDescription: 'Eszközöket hív az egységes MCP-közvetítőn keresztül.',
    memoryDescription: 'Keresi és kezeli a mentett személyes memóriát.',
  },
  'it-IT': {
    findName: 'Trova',
    lsName: 'Elenca',
    toolSearchName: 'Ricerca strumenti',
    agentsListName: 'Agenti',
    bashName: 'Bash',
    canvasName: 'Canvas',
    editName: 'Modifica',
    gatewayName: 'Gateway',
    grepName: 'Ricerca testo',
    imageName: 'Immagine',
    nodesName: 'Nodi',
    officeName: 'Office',
    pdfName: 'PDF',
    subagentsName: 'Sottoagenti',
    ttsName: 'Sintesi vocale',
    findDescription: 'Trova file e directory per pattern glob',
    lsDescription: 'Elenca file e directory',
    toolSearchDescription: 'Cerca strumenti, competenze e agenti per capacità',
    agentsListDescription: 'Elenca gli agenti configurati e le impostazioni predefinite.',
    bashDescription: 'Esegue un comando shell e restituisce stdout, stderr e codice di uscita.',
    canvasDescription: 'Crea e gestisce canvas leggeri Agent-to-UI.',
    editDescription: 'Esegue una sostituzione precisa del testo all’interno di un file.',
    gatewayDescription:
      'Controlla lo stato del gateway e le connessioni attive e può chiudere le connessioni bloccate.',
    grepDescription: 'Cerca un pattern di testo nei file.',
    imageDescription: 'Genera immagini, esamina input immagine o crea risorse per slide PPT.',
    nodesDescription: 'Gestisce grafi di workflow e automazioni basate su nodi.',
    officeDescription: 'Crea fogli .xlsx e report .docx con tema.',
    pdfDescription: 'Legge i metadati PDF o estrae testo da PDF locali o remoti.',
    subagentsDescription: 'Controlla la politica dei sottoagenti o avvia un agente figlio.',
    ttsDescription: 'Genera audio vocale, mostra voci e stato oppure ferma la riproduzione.',
    mcpDescription: 'Chiama strumenti tramite il dispatcher MCP unificato.',
    memoryDescription: 'Cerca e gestisce la memoria personale salvata.',
  },
  'ja-JP': {
    findName: '検索',
    lsName: '一覧',
    toolSearchName: 'ツール検索',
    agentsListName: 'エージェント',
    bashName: 'Bash',
    canvasName: 'キャンバス',
    editName: '編集',
    gatewayName: 'ゲートウェイ',
    grepName: 'テキスト検索',
    imageName: '画像',
    nodesName: 'ノード',
    officeName: 'Office',
    pdfName: 'PDF',
    subagentsName: 'サブエージェント',
    ttsName: '音声合成',
    findDescription: 'glob パターンでファイルとディレクトリを検索します',
    lsDescription: 'ファイルとディレクトリを一覧表示します',
    toolSearchDescription: '機能別にツール、スキル、エージェントを検索します',
    agentsListDescription: '設定済みエージェントと既定設定を一覧表示します。',
    bashDescription: 'シェルコマンドを実行し、stdout、stderr、終了コードを返します。',
    canvasDescription: '軽量な Agent-to-UI キャンバスを作成および管理します。',
    editDescription: 'ファイル内のテキストを正確に置換します。',
    gatewayDescription:
      'ゲートウェイの状態とアクティブ接続を確認し、固まった接続を閉じることができます。',
    grepDescription: '複数ファイルを横断してテキストパターンを検索します。',
    imageDescription: '画像を生成し、画像入力をレビューし、または PPT スライド素材を作成します。',
    nodesDescription: 'ワークフローのグラフとノードベースの自動化を管理します。',
    officeDescription: 'テーマ付きの .xlsx 表計算ファイルと .docx レポートを作成します。',
    pdfDescription:
      'PDF メタデータを読み取るか、ローカルまたはリモートの PDF からテキストを抽出します。',
    subagentsDescription: 'サブエージェント方針を確認するか、子エージェントを起動します。',
    ttsDescription: '音声を生成し、音声一覧と状態を表示し、または再生を停止します。',
    mcpDescription: '統合 MCP ディスパッチャー経由でツールを呼び出します。',
    memoryDescription: '保存された個人メモリを検索および管理します。',
  },
  'ko-KR': {
    findName: '찾기',
    lsName: '목록',
    toolSearchName: '도구 검색',
    agentsListName: '에이전트',
    bashName: 'Bash',
    canvasName: '캔버스',
    editName: '편집',
    gatewayName: '게이트웨이',
    grepName: '텍스트 검색',
    imageName: '이미지',
    nodesName: '노드',
    officeName: 'Office',
    pdfName: 'PDF',
    subagentsName: '서브에이전트',
    ttsName: '음성 합성',
    findDescription: 'glob 패턴으로 파일과 디렉터리를 찾습니다',
    lsDescription: '파일과 디렉터리를 나열합니다',
    toolSearchDescription: '기능별로 도구, 스킬, 에이전트를 검색합니다',
    agentsListDescription: '구성된 에이전트와 기본 설정을 나열합니다.',
    bashDescription: '셸 명령을 실행하고 stdout, stderr, 종료 코드를 반환합니다.',
    canvasDescription: '가벼운 Agent-to-UI 캔버스를 만들고 관리합니다.',
    editDescription: '파일 안에서 텍스트를 정확하게 치환합니다.',
    gatewayDescription: '게이트웨이 상태와 활성 연결을 확인하고, 멈춘 연결을 닫을 수 있습니다.',
    grepDescription: '여러 파일에서 텍스트 패턴을 검색합니다.',
    imageDescription: '이미지를 생성하고, 이미지 입력을 검토하거나, PPT 슬라이드 자산을 만듭니다.',
    nodesDescription: '워크플로 그래프와 노드 기반 자동화를 관리합니다.',
    officeDescription: '테마가 적용된 .xlsx 스프레드시트와 .docx 보고서를 만듭니다.',
    pdfDescription: 'PDF 메타데이터를 읽거나 로컬 또는 원격 PDF에서 텍스트를 추출합니다.',
    subagentsDescription: '서브에이전트 정책을 확인하거나 하위 에이전트를 생성합니다.',
    ttsDescription: '음성 오디오를 생성하고, 음성과 상태를 확인하거나, 재생을 중지합니다.',
    mcpDescription: '통합 MCP 디스패처를 통해 도구를 호출합니다.',
    memoryDescription: '저장된 개인 기억을 검색하고 관리합니다.',
  },
  'ml-IN': {
    findName: 'കണ്ടെത്തുക',
    lsName: 'പട്ടിക',
    toolSearchName: 'ഉപകരണ തിരയൽ',
    agentsListName: 'ഏജന്റുകൾ',
    bashName: 'Bash',
    canvasName: 'കാൻവാസ്',
    editName: 'എഡിറ്റ്',
    gatewayName: 'ഗേറ്റ്വേ',
    grepName: 'ടെക്സ്റ്റ് തിരയൽ',
    imageName: 'ചിത്രം',
    nodesName: 'നോഡുകൾ',
    officeName: 'ഓഫീസ്',
    pdfName: 'PDF',
    subagentsName: 'ഉപഏജന്റുകൾ',
    ttsName: 'ടെക്സ്റ്റ്-ടു-സ്പീച്ച്',
    findDescription: 'glob മാതൃക ഉപയോഗിച്ച് ഫയലുകളും ഡയറക്ടറികളും കണ്ടെത്തുക',
    lsDescription: 'ഫയലുകളും ഡയറക്ടറികളും പട്ടികപ്പെടുത്തുക',
    toolSearchDescription: 'ശേഷി അനുസരിച്ച് ഉപകരണങ്ങളും സ്കില്ലുകളും ഏജന്റുകളും തിരയുക',
    agentsListDescription: 'ക്രമീകരിച്ച ഏജന്റുകളും ഡീഫോൾട്ട് സജ്ജീകരണങ്ങളും പട്ടികപ്പെടുത്തുന്നു.',
    bashDescription:
      'ഒരു shell കമാൻഡ് പ്രവർത്തിപ്പിച്ച് stdout, stderr, exit code എന്നിവ തിരികെ നൽകുന്നു.',
    canvasDescription:
      'ലഘുഭാരമുള്ള Agent-to-UI canvasകൾ സൃഷ്ടിക്കുകയും നിയന്ത്രിക്കുകയും ചെയ്യുന്നു.',
    editDescription: 'ഒരു ഫയലിനുള്ളിൽ കൃത്യമായ ടെക്സ്റ്റ് പകരംവയ്ക്കൽ നടത്തുന്നു.',
    gatewayDescription:
      'gateway നിലയും സജീവ കണക്ഷനുകളും പരിശോധിക്കുകയും കുടുങ്ങിയ കണക്ഷനുകൾ അടയ്ക്കുകയും ചെയ്യാം.',
    grepDescription: 'ഫയലുകളിലാകെ ഒരു ടെക്സ്റ്റ് മാതൃക തിരയുന്നു.',
    imageDescription:
      'ചിത്രങ്ങൾ സൃഷ്ടിക്കുന്നു, ചിത്ര ഇൻപുട്ടുകൾ അവലോകനം ചെയ്യുന്നു, അല്ലെങ്കിൽ PPT slide ആസ്തികൾ നിർമ്മിക്കുന്നു.',
    nodesDescription: 'workflow ഗ്രാഫുകളും node അടിസ്ഥാനമാക്കിയ automationകളും നിയന്ത്രിക്കുന്നു.',
    officeDescription: 'തീം ചെയ്ത .xlsx സ്പ്രെഡ്ഷീറ്റുകളും .docx റിപ്പോർട്ടുകളും സൃഷ്ടിക്കുന്നു.',
    pdfDescription:
      'PDF metadata വായിക്കുകയോ local/remote PDFകളിൽ നിന്ന് ടെക്സ്റ്റ് എടുക്കുകയോ ചെയ്യുന്നു.',
    subagentsDescription: 'ഉപഏജന്റ് നയം പരിശോധിക്കുകയോ ഒരു child agent സൃഷ്ടിക്കുകയോ ചെയ്യുന്നു.',
    ttsDescription:
      'ശബ്ദ ഓഡിയോ സൃഷ്ടിക്കുന്നു, voiceകളും നിലയും കാണിക്കുന്നു, അല്ലെങ്കിൽ പ്ലേബാക്ക് നിർത്തുന്നു.',
    mcpDescription: 'ഏകീകൃത MCP dispatcher വഴി ഉപകരണങ്ങളെ വിളിക്കുന്നു.',
    memoryDescription: 'സംരക്ഷിച്ച വ്യക്തിഗത മെമ്മറി തിരയുകയും നിയന്ത്രിക്കുകയും ചെയ്യുന്നു.',
  },
  'nb-NO': {
    findName: 'Finn',
    lsName: 'List opp',
    toolSearchName: 'Verktøysøk',
    agentsListName: 'Agenter',
    bashName: 'Bash',
    canvasName: 'Canvas',
    editName: 'Rediger',
    gatewayName: 'Gateway',
    grepName: 'Tekstsøk',
    imageName: 'Bilde',
    nodesName: 'Noder',
    officeName: 'Office',
    pdfName: 'PDF',
    subagentsName: 'Underagenter',
    ttsName: 'Tekst-til-tale',
    findDescription: 'Finn filer og kataloger etter glob-mønster',
    lsDescription: 'List opp filer og kataloger',
    toolSearchDescription: 'Søk etter verktøy, ferdigheter og agenter etter kapasitet',
    agentsListDescription: 'Lister konfigurerte agenter og standardinnstillinger.',
    bashDescription: 'Kjører en shell-kommando og returnerer stdout, stderr og avslutningskode.',
    canvasDescription: 'Oppretter og administrerer lette Agent-to-UI-canvaser.',
    editDescription: 'Gjør en presis tekstutskifting direkte i en fil.',
    gatewayDescription:
      'Inspiserer gateway-status og aktive tilkoblinger og kan lukke fastlåste tilkoblinger.',
    grepDescription: 'Søker etter et tekstmønster på tvers av filer.',
    imageDescription: 'Genererer bilder, vurderer bildeinnganger eller bygger PPT-lysbildeaktiva.',
    nodesDescription: 'Administrerer arbeidsflytgraf og nodebaserte automatiseringer.',
    officeDescription: 'Oppretter tematiserte .xlsx-regneark og .docx-rapporter.',
    pdfDescription: 'Leser PDF-metadata eller henter ut tekst fra lokale eller eksterne PDF-er.',
    subagentsDescription: 'Inspiserer underagentpolicy eller starter en barneagent.',
    ttsDescription: 'Genererer tale, viser stemmer og status eller stopper avspilling.',
    mcpDescription: 'Kaller verktøy gjennom den samlede MCP-dispatcheren.',
    memoryDescription: 'Søker og administrerer lagret personlig minne.',
  },
  'nl-NL': {
    findName: 'Zoeken',
    lsName: 'Lijst',
    toolSearchName: 'Zoeken naar tools',
    agentsListName: 'Agenten',
    bashName: 'Bash',
    canvasName: 'Canvas',
    editName: 'Bewerken',
    gatewayName: 'Gateway',
    grepName: 'Tekst zoeken',
    imageName: 'Afbeelding',
    nodesName: 'Knooppunten',
    officeName: 'Office',
    pdfName: 'PDF',
    subagentsName: 'Subagenten',
    ttsName: 'Tekst-naar-spraak',
    findDescription: 'Zoek bestanden en mappen op glob-patroon',
    lsDescription: 'Lijst bestanden en mappen op',
    toolSearchDescription: 'Zoek tools, vaardigheden en agents op basis van mogelijkheden',
    agentsListDescription: 'Geeft geconfigureerde agenten en standaardinstellingen weer.',
    bashDescription: 'Voert een shell-opdracht uit en geeft stdout, stderr en exitcode terug.',
    canvasDescription: 'Maakt en beheert lichte Agent-to-UI-canvassen.',
    editDescription: 'Voert een nauwkeurige tekstvervanging uit in een bestand.',
    gatewayDescription:
      'Controleert de gatewaystatus en actieve verbindingen en kan vastgelopen verbindingen sluiten.',
    grepDescription: 'Zoekt een tekstpatroon in bestanden.',
    imageDescription:
      'Genereert afbeeldingen, beoordeelt beeldinvoer of maakt middelen voor PPT-dia’s.',
    nodesDescription: 'Beheert workflowgrafen en op knooppunten gebaseerde automatiseringen.',
    officeDescription: 'Maakt thematische .xlsx-spreadsheets en .docx-rapporten.',
    pdfDescription: 'Leest PDF-metadata of haalt tekst uit lokale of externe PDF-bestanden.',
    subagentsDescription: 'Controleert subagentbeleid of start een kindagent.',
    ttsDescription: 'Genereert spraakaudio, toont stemmen en status, of stopt afspelen.',
    mcpDescription: 'Roept tools aan via de uniforme MCP-dispatcher.',
    memoryDescription: 'Zoekt en beheert opgeslagen persoonlijk geheugen.',
  },
  'pl-PL': {
    findName: 'Znajdź',
    lsName: 'Lista',
    toolSearchName: 'Wyszukiwanie narzędzi',
    agentsListName: 'Agenci',
    bashName: 'Bash',
    canvasName: 'Kanwa',
    editName: 'Edytuj',
    gatewayName: 'Brama',
    grepName: 'Wyszukiwanie tekstu',
    imageName: 'Obraz',
    nodesName: 'Węzły',
    officeName: 'Office',
    pdfName: 'PDF',
    subagentsName: 'Podagenci',
    ttsName: 'Tekst na mowę',
    findDescription: 'Znajdź pliki i katalogi według wzorca glob',
    lsDescription: 'Wyświetl pliki i katalogi',
    toolSearchDescription: 'Wyszukuj narzędzia, umiejętności i agentów według możliwości',
    agentsListDescription: 'Wyświetla skonfigurowanych agentów i ustawienia domyślne.',
    bashDescription: 'Uruchamia polecenie shell i zwraca stdout, stderr oraz kod zakończenia.',
    canvasDescription: 'Tworzy i zarządza lekkimi kanwami Agent-to-UI.',
    editDescription: 'Wykonuje precyzyjną zamianę tekstu w pliku.',
    gatewayDescription:
      'Sprawdza stan bramy i aktywne połączenia oraz może zamknąć zablokowane połączenia.',
    grepDescription: 'Wyszukuje wzorzec tekstu w plikach.',
    imageDescription:
      'Generuje obrazy, przegląda wejścia obrazów lub tworzy zasoby do slajdów PPT.',
    nodesDescription: 'Zarządza grafami workflow i automatyzacjami opartymi na węzłach.',
    officeDescription: 'Tworzy tematyczne arkusze .xlsx i raporty .docx.',
    pdfDescription: 'Czyta metadane PDF lub wyciąga tekst z lokalnych albo zdalnych plików PDF.',
    subagentsDescription: 'Sprawdza politykę podagentów lub uruchamia agenta potomnego.',
    ttsDescription: 'Generuje mowę, pokazuje głosy i stan lub zatrzymuje odtwarzanie.',
    mcpDescription: 'Wywołuje narzędzia przez ujednolicony dispatcher MCP.',
    memoryDescription: 'Wyszukuje i zarządza zapisaną pamięcią osobistą.',
  },
  'pt-BR': {
    findName: 'Buscar',
    lsName: 'Listar',
    toolSearchName: 'Busca de ferramentas',
    agentsListName: 'Agentes',
    bashName: 'Bash',
    canvasName: 'Canvas',
    editName: 'Editar',
    gatewayName: 'Gateway',
    grepName: 'Busca de texto',
    imageName: 'Imagem',
    nodesName: 'Nós',
    officeName: 'Office',
    pdfName: 'PDF',
    subagentsName: 'Subagentes',
    ttsName: 'Texto para fala',
    findDescription: 'Buscar arquivos e diretórios por padrão glob',
    lsDescription: 'Listar arquivos e diretórios',
    toolSearchDescription: 'Buscar ferramentas, habilidades e agentes por capacidade',
    agentsListDescription: 'Lista os agentes configurados e as configurações padrão.',
    bashDescription: 'Executa um comando de shell e retorna stdout, stderr e código de saída.',
    canvasDescription: 'Cria e gerencia canvases leves de Agent-to-UI.',
    editDescription: 'Faz uma substituição precisa de texto dentro de um arquivo.',
    gatewayDescription:
      'Inspeciona o status do gateway e as conexões ativas, e pode fechar conexões travadas.',
    grepDescription: 'Procura um padrão de texto em arquivos.',
    imageDescription: 'Gera imagens, revisa entradas de imagem ou cria recursos para slides PPT.',
    nodesDescription: 'Gerencia grafos de workflow e automações baseadas em nós.',
    officeDescription: 'Cria planilhas .xlsx e relatórios .docx com tema.',
    pdfDescription: 'Lê metadados de PDF ou extrai texto de PDFs locais ou remotos.',
    subagentsDescription: 'Inspeciona a política de subagentes ou inicia um agente filho.',
    ttsDescription: 'Gera áudio de fala, mostra vozes e status, ou interrompe a reprodução.',
    mcpDescription: 'Chama ferramentas por meio do despachante MCP unificado.',
    memoryDescription: 'Pesquisa e gerencia a memória pessoal salva.',
  },
  'pt-PT': {
    findName: 'Pesquisar',
    lsName: 'Listar',
    toolSearchName: 'Pesquisa de ferramentas',
    agentsListName: 'Agentes',
    bashName: 'Bash',
    canvasName: 'Canvas',
    editName: 'Editar',
    gatewayName: 'Gateway',
    grepName: 'Pesquisa de texto',
    imageName: 'Imagem',
    nodesName: 'Nós',
    officeName: 'Office',
    pdfName: 'PDF',
    subagentsName: 'Subagentes',
    ttsName: 'Texto para fala',
    findDescription: 'Procurar ficheiros e diretórios por padrão glob',
    lsDescription: 'Listar ficheiros e diretórios',
    toolSearchDescription: 'Pesquisar ferramentas, competências e agentes por capacidade',
    agentsListDescription: 'Lista os agentes configurados e as definições predefinidas.',
    bashDescription: 'Executa um comando de shell e devolve stdout, stderr e código de saída.',
    canvasDescription: 'Cria e gere canvases leves de Agent-to-UI.',
    editDescription: 'Faz uma substituição precisa de texto dentro de um ficheiro.',
    gatewayDescription:
      'Inspeciona o estado do gateway e as ligações ativas, e pode fechar ligações bloqueadas.',
    grepDescription: 'Procura um padrão de texto em ficheiros.',
    imageDescription:
      'Gera imagens, revê entradas de imagem ou cria recursos para diapositivos PPT.',
    nodesDescription: 'Gere grafos de fluxo de trabalho e automações baseadas em nós.',
    officeDescription: 'Cria folhas de cálculo .xlsx e relatórios .docx com tema.',
    pdfDescription: 'Lê metadados PDF ou extrai texto de PDFs locais ou remotos.',
    subagentsDescription: 'Inspeciona a política de subagentes ou lança um agente filho.',
    ttsDescription: 'Gera áudio de fala, mostra vozes e estado, ou pára a reprodução.',
    mcpDescription: 'Chama ferramentas através do despachante MCP unificado.',
    memoryDescription: 'Pesquisa e gere a memória pessoal guardada.',
  },
  'ro-RO': {
    findName: 'Căutare',
    lsName: 'Listare',
    toolSearchName: 'Căutare unelte',
    agentsListName: 'Agenți',
    bashName: 'Bash',
    canvasName: 'Pânză',
    editName: 'Editare',
    gatewayName: 'Gateway',
    grepName: 'Căutare text',
    imageName: 'Imagine',
    nodesName: 'Noduri',
    officeName: 'Office',
    pdfName: 'PDF',
    subagentsName: 'Subagenți',
    ttsName: 'Text în vorbire',
    findDescription: 'Găsește fișiere și directoare după model glob',
    lsDescription: 'Listează fișiere și directoare',
    toolSearchDescription: 'Caută unelte, abilități și agenți după capacitate',
    agentsListDescription: 'Listează agenții configurați și setările implicite.',
    bashDescription: 'Rulează o comandă shell și returnează stdout, stderr și codul de ieșire.',
    canvasDescription: 'Creează și gestionează pânze ușoare Agent-to-UI.',
    editDescription: 'Face o înlocuire precisă de text într-un fișier.',
    gatewayDescription:
      'Inspectează starea gateway-ului și conexiunile active și poate închide conexiunile blocate.',
    grepDescription: 'Caută un model de text în fișiere.',
    imageDescription:
      'Generează imagini, revizuiește intrări de imagine sau construiește resurse pentru slide-uri PPT.',
    nodesDescription: 'Gestionează grafuri de workflow și automatizări bazate pe noduri.',
    officeDescription: 'Creează foi .xlsx și rapoarte .docx cu temă.',
    pdfDescription: 'Citește metadate PDF sau extrage text din PDF-uri locale ori la distanță.',
    subagentsDescription: 'Inspectează politica de subagenți sau pornește un agent copil.',
    ttsDescription: 'Generează audio vocal, afișează vocile și starea sau oprește redarea.',
    mcpDescription: 'Apelează unelte prin dispatcherul MCP unificat.',
    memoryDescription: 'Caută și gestionează memoria personală salvată.',
  },
  'ru-RU': {
    findName: 'Поиск',
    lsName: 'Список',
    toolSearchName: 'Поиск инструментов',
    agentsListName: 'Агенты',
    bashName: 'Bash',
    canvasName: 'Холст',
    editName: 'Правка',
    gatewayName: 'Шлюз',
    grepName: 'Поиск текста',
    imageName: 'Изображение',
    nodesName: 'Узлы',
    officeName: 'Office',
    pdfName: 'PDF',
    subagentsName: 'Субагенты',
    ttsName: 'Синтез речи',
    findDescription: 'Ищет файлы и каталоги по шаблону glob',
    lsDescription: 'Показывает список файлов и каталогов',
    toolSearchDescription: 'Ищет инструменты, навыки и агентов по возможностям',
    agentsListDescription: 'Показывает настроенных агентов и параметры по умолчанию.',
    bashDescription: 'Выполняет shell-команду и возвращает stdout, stderr и код завершения.',
    canvasDescription: 'Создает и управляет легкими холстами Agent-to-UI.',
    editDescription: 'Выполняет точную замену текста прямо в файле.',
    gatewayDescription:
      'Проверяет состояние шлюза и активные подключения и может закрыть зависшие подключения.',
    grepDescription: 'Ищет текстовый шаблон по файлам.',
    imageDescription:
      'Создает изображения, проверяет входные изображения или готовит ресурсы для слайдов PPT.',
    nodesDescription: 'Управляет графами workflow и автоматизациями на основе узлов.',
    officeDescription: 'Создает тематические таблицы .xlsx и отчеты .docx.',
    pdfDescription: 'Читает метаданные PDF или извлекает текст из локальных и удаленных PDF.',
    subagentsDescription: 'Проверяет политику субагентов или запускает дочернего агента.',
    ttsDescription: 'Создает речь, показывает голоса и статус или останавливает воспроизведение.',
    mcpDescription: 'Вызывает инструменты через единый диспетчер MCP.',
    memoryDescription: 'Ищет и управляет сохраненной личной памятью.',
  },
  'sk-SK': {
    findName: 'Nájsť',
    lsName: 'Zoznam',
    toolSearchName: 'Hľadanie nástrojov',
    agentsListName: 'Agenti',
    bashName: 'Bash',
    canvasName: 'Plátno',
    editName: 'Upraviť',
    gatewayName: 'Brána',
    grepName: 'Hľadanie textu',
    imageName: 'Obrázok',
    nodesName: 'Uzly',
    officeName: 'Office',
    pdfName: 'PDF',
    subagentsName: 'Subagenti',
    ttsName: 'Prevod textu na reč',
    findDescription: 'Nájde súbory a priečinky podľa glob vzoru',
    lsDescription: 'Vypíše súbory a priečinky',
    toolSearchDescription: 'Vyhľadáva nástroje, zručnosti a agentov podľa schopností',
    agentsListDescription: 'Vypíše nakonfigurovaných agentov a predvolené nastavenia.',
    bashDescription: 'Spustí shellový príkaz a vráti stdout, stderr a návratový kód.',
    canvasDescription: 'Vytvára a spravuje ľahké Agent-to-UI plátna.',
    editDescription: 'Vykoná presnú náhradu textu priamo v súbore.',
    gatewayDescription:
      'Skontroluje stav brány a aktívne pripojenia a môže zatvoriť zaseknuté pripojenia.',
    grepDescription: 'Vyhľadá textový vzor naprieč súbormi.',
    imageDescription:
      'Generuje obrázky, kontroluje obrazové vstupy alebo vytvára podklady pre snímky PPT.',
    nodesDescription: 'Spravuje workflow grafy a automatizácie založené na uzloch.',
    officeDescription: 'Vytvára tematické tabuľky .xlsx a správy .docx.',
    pdfDescription: 'Číta metadata PDF alebo extrahuje text z lokálnych či vzdialených PDF.',
    subagentsDescription: 'Skontroluje politiku subagentov alebo spustí podradeného agenta.',
    ttsDescription: 'Generuje hlasový zvuk, zobrazuje hlasy a stav alebo zastaví prehrávanie.',
    mcpDescription: 'Volá nástroje cez zjednotený MCP dispatcher.',
    memoryDescription: 'Vyhľadáva a spravuje uloženú osobnú pamäť.',
  },
  'sv-SE': {
    findName: 'Hitta',
    lsName: 'Lista',
    toolSearchName: 'Verktygssökning',
    agentsListName: 'Agenter',
    bashName: 'Bash',
    canvasName: 'Canvas',
    editName: 'Redigera',
    gatewayName: 'Gateway',
    grepName: 'Textsökning',
    imageName: 'Bild',
    nodesName: 'Noder',
    officeName: 'Office',
    pdfName: 'PDF',
    subagentsName: 'Underagenter',
    ttsName: 'Text till tal',
    findDescription: 'Hitta filer och kataloger med glob-mönster',
    lsDescription: 'Lista filer och kataloger',
    toolSearchDescription: 'Sök efter verktyg, färdigheter och agenter efter kapacitet',
    agentsListDescription: 'Visar konfigurerade agenter och standardinställningar.',
    bashDescription: 'Kör ett shellkommando och returnerar stdout, stderr och avslutningskod.',
    canvasDescription: 'Skapar och hanterar lätta Agent-to-UI-canvaser.',
    editDescription: 'Gör en exakt textersättning direkt i en fil.',
    gatewayDescription:
      'Inspekterar gateway-status och aktiva anslutningar och kan stänga fastnade anslutningar.',
    grepDescription: 'Söker efter ett textmönster i filer.',
    imageDescription:
      'Genererar bilder, granskar bildinmatning eller bygger tillgångar för PPT-bilder.',
    nodesDescription: 'Hanterar arbetsflödesgrafer och nodbaserade automatiseringar.',
    officeDescription: 'Skapar tematiska .xlsx-kalkylblad och .docx-rapporter.',
    pdfDescription:
      'Läser PDF-metadata eller extraherar text från lokala eller fjärranslutna PDF-filer.',
    subagentsDescription: 'Inspekterar policy för underagenter eller startar en barnagent.',
    ttsDescription: 'Genererar tal, visar röster och status eller stoppar uppspelning.',
    mcpDescription: 'Anropar verktyg via den enhetliga MCP-dispatchern.',
    memoryDescription: 'Söker och hanterar sparat personligt minne.',
  },
  'zh-CN': {
    findName: '查找',
    lsName: '列表',
    toolSearchName: '工具搜索',
    agentsListName: '代理',
    bashName: 'Bash',
    canvasName: '画布',
    editName: '编辑',
    gatewayName: '网关',
    grepName: '文本搜索',
    imageName: '图片',
    nodesName: '节点',
    officeName: '办公文档',
    pdfName: 'PDF',
    subagentsName: '子代理',
    ttsName: '语音合成',
    findDescription: '按 glob 模式查找文件和目录',
    lsDescription: '列出文件和目录',
    toolSearchDescription: '按能力搜索工具、技能和代理',
    agentsListDescription: '列出已配置的代理及默认设置。',
    bashDescription: '运行 shell 命令，并返回 stdout、stderr 和退出码。',
    canvasDescription: '创建并管理轻量级的 Agent-to-UI 画布。',
    editDescription: '在文件中精确替换指定文本。',
    gatewayDescription: '检查网关状态和活动连接，并可关闭卡住的连接。',
    grepDescription: '在多个文件中搜索文本模式。',
    imageDescription: '生成图片、审查图像输入，或构建 PPT 幻灯片素材。',
    nodesDescription: '管理工作流图和基于节点的自动化。',
    officeDescription: '创建带主题的 .xlsx 电子表格和 .docx 报告。',
    pdfDescription: '读取 PDF 元数据，或从本地或远程 PDF 中提取文本。',
    subagentsDescription: '检查子代理策略，或创建一个子代理。',
    ttsDescription: '生成语音音频、查看音色与状态，或停止播放。',
    mcpDescription: '通过统一的 MCP 分发器调用工具。',
    memoryDescription: '搜索并管理已保存的个人记忆。',
  },
  'zh-TW': {
    findName: '查找',
    lsName: '列表',
    toolSearchName: '工具搜尋',
    agentsListName: '代理',
    bashName: 'Bash',
    canvasName: '畫布',
    editName: '編輯',
    gatewayName: '閘道',
    grepName: '文字搜尋',
    imageName: '圖片',
    nodesName: '節點',
    officeName: '辦公文件',
    pdfName: 'PDF',
    subagentsName: '子代理',
    ttsName: '語音合成',
    findDescription: '依 glob 模式查找檔案與目錄',
    lsDescription: '列出檔案與目錄',
    toolSearchDescription: '依能力搜尋工具、技能與代理',
    agentsListDescription: '列出已設定的代理與預設設定。',
    bashDescription: '執行 shell 指令，並回傳 stdout、stderr 與結束碼。',
    canvasDescription: '建立並管理輕量級的 Agent-to-UI 畫布。',
    editDescription: '在檔案中精確取代指定文字。',
    gatewayDescription: '檢查閘道狀態與作用中的連線，並可關閉卡住的連線。',
    grepDescription: '在多個檔案中搜尋文字模式。',
    imageDescription: '產生圖片、檢視圖像輸入，或建立 PPT 投影片素材。',
    nodesDescription: '管理工作流程圖與節點式自動化。',
    officeDescription: '建立帶主題的 .xlsx 試算表與 .docx 報告。',
    pdfDescription: '讀取 PDF 中繼資料，或從本機或遠端 PDF 擷取文字。',
    subagentsDescription: '檢查子代理政策，或建立一個子代理。',
    ttsDescription: '產生語音音訊、查看聲音與狀態，或停止播放。',
    mcpDescription: '透過統一的 MCP 分派器呼叫工具。',
    memoryDescription: '搜尋並管理已儲存的個人記憶。',
  },
} as const satisfies Record<
  LocaleKey,
  {
    findName: string
    lsName: string
    toolSearchName: string
    agentsListName: string
    bashName: string
    canvasName: string
    editName: string
    gatewayName: string
    grepName: string
    imageName: string
    nodesName: string
    officeName: string
    pdfName: string
    subagentsName: string
    ttsName: string
    findDescription: string
    lsDescription: string
    toolSearchDescription: string
    agentsListDescription: string
    bashDescription: string
    canvasDescription: string
    editDescription: string
    gatewayDescription: string
    grepDescription: string
    imageDescription: string
    nodesDescription: string
    officeDescription: string
    pdfDescription: string
    subagentsDescription: string
    ttsDescription: string
    mcpDescription: string
    memoryDescription: string
  }
>

const fileDeleteLocaleTerms: Record<
  LocaleKey,
  {
    name: string
    description: string
  }
> = {
  'ca-ES': {
    name: 'Eliminar fitxer',
    description:
      'Suprimeix un fitxer o directori local. Per als directoris, estableix recursive=true per eliminar contingut no buit.',
  },
  'cs-CZ': {
    name: 'Smazat soubor',
    description:
      'Smaže místní soubor nebo adresář. U adresářů nastavte recursive=true, chcete-li odstranit i neprázdný obsah.',
  },
  'da-DK': {
    name: 'Slet fil',
    description:
      'Sletter en lokal fil eller mappe. For mapper skal du sætte recursive=true for at fjerne ikke-tomt indhold.',
  },
  'de-DE': {
    name: 'Datei löschen',
    description:
      'Löscht eine lokale Datei oder ein lokales Verzeichnis. Setze bei Verzeichnissen recursive=true, um auch nicht-leere Inhalte zu entfernen.',
  },
  'el-GR': {
    name: 'Διαγραφή αρχείου',
    description:
      'Διαγράφει ένα τοπικό αρχείο ή κατάλογο. Για καταλόγους, ορίστε recursive=true για να αφαιρέσετε και μη κενό περιεχόμενο.',
  },
  'en-GB': {
    name: 'File Delete',
    description:
      'Deletes a local file or directory. For directories, set recursive=true to remove non-empty contents.',
  },
  'en-US': {
    name: 'File Delete',
    description:
      'Deletes a local file or directory. For directories, set recursive=true to remove non-empty contents.',
  },
  'es-ES': {
    name: 'Eliminar archivo',
    description:
      'Elimina un archivo o directorio local. Para los directorios, establece recursive=true para quitar contenido no vacío.',
  },
  'fr-FR': {
    name: 'Supprimer le fichier',
    description:
      'Supprime un fichier ou répertoire local. Pour les répertoires, définissez recursive=true pour supprimer aussi le contenu non vide.',
  },
  'ga-IE': {
    name: 'Scrios comhad',
    description:
      'Scriosann sé comhad nó comhadlann áitiúil. Maidir le comhadlanna, socraigh recursive=true chun ábhar neamhfholamh a bhaint.',
  },
  'hr-HR': {
    name: 'Obriši datoteku',
    description:
      'Briše lokalnu datoteku ili direktorij. Za direktorije postavite recursive=true kako biste uklonili i neprazan sadržaj.',
  },
  'hu-HU': {
    name: 'Fájl törlése',
    description:
      'Töröl egy helyi fájlt vagy könyvtárat. Könyvtáraknál állítsa a recursive=true értéket a nem üres tartalom eltávolításához.',
  },
  'it-IT': {
    name: 'Elimina file',
    description:
      'Elimina un file o una directory locale. Per le directory, imposta recursive=true per rimuovere anche il contenuto non vuoto.',
  },
  'ja-JP': {
    name: 'ファイル削除',
    description:
      'ローカルのファイルまたはディレクトリを削除します。ディレクトリでは、空でない内容も削除するには recursive=true を設定します。',
  },
  'ko-KR': {
    name: '파일 삭제',
    description:
      '로컬 파일이나 디렉터리를 삭제합니다. 디렉터리의 경우 비어 있지 않은 내용까지 제거하려면 recursive=true로 설정하세요.',
  },
  'ml-IN': {
    name: 'ഫയൽ ഇല്ലാതാക്കുക',
    description:
      'ഒരു ലോക്കൽ ഫയൽ അല്ലെങ്കിൽ ഡയറക്ടറി ഇല്ലാതാക്കുന്നു. ഡയറക്ടറികൾക്കായി ശൂന്യമല്ലാത്ത ഉള്ളടക്കവും നീക്കാൻ recursive=true ആയി സജ്ജീകരിക്കുക.',
  },
  'nb-NO': {
    name: 'Slett fil',
    description:
      'Sletter en lokal fil eller katalog. For kataloger må du sette recursive=true for å fjerne innhold som ikke er tomt.',
  },
  'nl-NL': {
    name: 'Bestand verwijderen',
    description:
      'Verwijdert een lokaal bestand of map. Stel voor mappen recursive=true in om ook niet-lege inhoud te verwijderen.',
  },
  'pl-PL': {
    name: 'Usuń plik',
    description:
      'Usuwa lokalny plik lub katalog. W przypadku katalogów ustaw recursive=true, aby usunąć także niepustą zawartość.',
  },
  'pt-BR': {
    name: 'Excluir arquivo',
    description:
      'Exclui um arquivo ou diretório local. Para diretórios, defina recursive=true para remover também conteúdo não vazio.',
  },
  'pt-PT': {
    name: 'Eliminar ficheiro',
    description:
      'Elimina um ficheiro ou diretório local. Para diretórios, defina recursive=true para remover também conteúdo não vazio.',
  },
  'ro-RO': {
    name: 'Șterge fișier',
    description:
      'Șterge un fișier sau director local. Pentru directoare, setați recursive=true pentru a elimina și conținutul nevid.',
  },
  'ru-RU': {
    name: 'Удалить файл',
    description:
      'Удаляет локальный файл или каталог. Для каталогов установите recursive=true, чтобы удалить и непустое содержимое.',
  },
  'sk-SK': {
    name: 'Odstrániť súbor',
    description:
      'Odstráni lokálny súbor alebo adresár. Pri adresároch nastavte recursive=true, ak chcete odstrániť aj neprázdny obsah.',
  },
  'sv-SE': {
    name: 'Ta bort fil',
    description:
      'Tar bort en lokal fil eller katalog. För kataloger anger du recursive=true för att även ta bort innehåll som inte är tomt.',
  },
  'zh-CN': {
    name: '删除文件',
    description: '删除本地文件或目录。对于目录，请设置 recursive=true 以删除其中的非空内容。',
  },
  'zh-TW': {
    name: '刪除檔案',
    description: '刪除本機檔案或目錄。對於目錄，請設定 recursive=true 以移除其中的非空內容。',
  },
}

const configLocaleTerms: Record<
  LocaleKey,
  {
    name: string
    description: string
  }
> = {
  'ca-ES': {
    name: 'Configuració',
    description:
      'Eina de configuració i administració del runtime. Utilitza el format {domain}.{action}. Dominis: providers, settings, channels, skills, tools, system, proxy, users, apikeys, upgrade. Crida primer amb action="providers.list" per explorar les operacions disponibles. Comuns: providers.list, settings.get, system.health, tools.list, users.list, upgrade.status.',
  },
  'cs-CZ': {
    name: 'Konfigurace',
    description:
      'Nástroj pro konfiguraci runtime a správu. Používejte formát {domain}.{action}. Domény: providers, settings, channels, skills, tools, system, proxy, users, apikeys, upgrade. Nejprve zavolejte action="providers.list" pro prozkoumání dostupných operací. Běžné: providers.list, settings.get, system.health, tools.list, users.list, upgrade.status.',
  },
  'da-DK': {
    name: 'Konfiguration',
    description:
      'Runtime-konfigurations- og administrationsværktøj. Brug formatet {domain}.{action}. Domæner: providers, settings, channels, skills, tools, system, proxy, users, apikeys, upgrade. Kald først med action="providers.list" for at udforske tilgængelige handlinger. Almindelige: providers.list, settings.get, system.health, tools.list, users.list, upgrade.status.',
  },
  'de-DE': {
    name: 'Konfiguration',
    description:
      'Laufzeit-Konfigurations- und Verwaltungstool. Verwende das Format {domain}.{action}. Domänen: providers, settings, channels, skills, tools, system, proxy, users, apikeys, upgrade. Rufe zuerst action="providers.list" auf, um verfügbare Operationen zu erkunden. Häufig: providers.list, settings.get, system.health, tools.list, users.list, upgrade.status.',
  },
  'el-GR': {
    name: 'Ρυθμίσεις',
    description:
      'Εργαλείο ρυθμίσεων και διαχείρισης runtime. Χρησιμοποιήστε τη μορφή {domain}.{action}. Τομείς: providers, settings, channels, skills, tools, system, proxy, users, apikeys, upgrade. Καλέστε πρώτα με action="providers.list" για να εξερευνήσετε τις διαθέσιμες λειτουργίες. Συνήθη: providers.list, settings.get, system.health, tools.list, users.list, upgrade.status.',
  },
  'en-GB': {
    name: 'Config',
    description:
      'Runtime configuration and admin tool. Use {domain}.{action} format. Domains: providers, settings, channels, skills, tools, system, proxy, users, apikeys, upgrade. Call with action="providers.list" first to explore available operations. Common: providers.list, settings.get, system.health, tools.list, users.list, upgrade.status.',
  },
  'en-US': {
    name: 'Config',
    description:
      'Runtime configuration and admin tool. Use {domain}.{action} format. Domains: providers, settings, channels, skills, tools, system, proxy, users, apikeys, upgrade. Call with action="providers.list" first to explore available operations. Common: providers.list, settings.get, system.health, tools.list, users.list, upgrade.status.',
  },
  'es-ES': {
    name: 'Configuración',
    description:
      'Herramienta de configuración y administración del runtime. Usa el formato {domain}.{action}. Dominios: providers, settings, channels, skills, tools, system, proxy, users, apikeys, upgrade. Llama primero con action="providers.list" para explorar las operaciones disponibles. Comunes: providers.list, settings.get, system.health, tools.list, users.list, upgrade.status.',
  },
  'fr-FR': {
    name: 'Configuration',
    description:
      'Outil de configuration et d’administration du runtime. Utilisez le format {domain}.{action}. Domaines : providers, settings, channels, skills, tools, system, proxy, users, apikeys, upgrade. Appelez d’abord avec action="providers.list" pour explorer les opérations disponibles. Courants : providers.list, settings.get, system.health, tools.list, users.list, upgrade.status.',
  },
  'ga-IE': {
    name: 'Cumraíocht',
    description:
      'Uirlis chumraíochta agus riaracháin runtime. Úsáid an fhormáid {domain}.{action}. Fearainn: providers, settings, channels, skills, tools, system, proxy, users, apikeys, upgrade. Glaoigh ar action="providers.list" ar dtús chun na hoibríochtaí atá ar fáil a iniúchadh. Coitianta: providers.list, settings.get, system.health, tools.list, users.list, upgrade.status.',
  },
  'hr-HR': {
    name: 'Konfiguracija',
    description:
      'Alat za konfiguraciju i administraciju runtimea. Koristite format {domain}.{action}. Domene: providers, settings, channels, skills, tools, system, proxy, users, apikeys, upgrade. Najprije pozovite action="providers.list" kako biste istražili dostupne operacije. Uobičajeno: providers.list, settings.get, system.health, tools.list, users.list, upgrade.status.',
  },
  'hu-HU': {
    name: 'Konfiguráció',
    description:
      'Futásidejű konfigurációs és adminisztrációs eszköz. Használja a(z) {domain}.{action} formátumot. Domainek: providers, settings, channels, skills, tools, system, proxy, users, apikeys, upgrade. Először a action="providers.list" hívással fedezze fel az elérhető műveleteket. Gyakoriak: providers.list, settings.get, system.health, tools.list, users.list, upgrade.status.',
  },
  'it-IT': {
    name: 'Configurazione',
    description:
      'Strumento di configurazione e amministrazione del runtime. Usa il formato {domain}.{action}. Domini: providers, settings, channels, skills, tools, system, proxy, users, apikeys, upgrade. Chiama prima con action="providers.list" per esplorare le operazioni disponibili. Comuni: providers.list, settings.get, system.health, tools.list, users.list, upgrade.status.',
  },
  'ja-JP': {
    name: '設定',
    description:
      'ランタイム設定と管理のためのツールです。{domain}.{action} 形式を使用します。ドメイン: providers, settings, channels, skills, tools, system, proxy, users, apikeys, upgrade。利用可能な操作を確認するには、まず action="providers.list" を呼び出してください。一般的な操作: providers.list, settings.get, system.health, tools.list, users.list, upgrade.status。',
  },
  'ko-KR': {
    name: '설정',
    description:
      '런타임 구성 및 관리자 도구입니다. {domain}.{action} 형식을 사용하세요. 도메인: providers, settings, channels, skills, tools, system, proxy, users, apikeys, upgrade. 사용 가능한 작업을 먼저 확인하려면 action="providers.list"로 호출하세요. 일반 예: providers.list, settings.get, system.health, tools.list, users.list, upgrade.status.',
  },
  'ml-IN': {
    name: 'ക്രമീകരണം',
    description:
      'റൺടൈം കോൺഫിഗറേഷനും അഡ്മിൻ പ്രവർത്തനങ്ങൾക്കും ഉള്ള ടൂളാണ് ഇത്. {domain}.{action} ഫോർമാറ്റ് ഉപയോഗിക്കുക. ഡൊമെയ്‌നുകൾ: providers, settings, channels, skills, tools, system, proxy, users, apikeys, upgrade. ലഭ്യമായ പ്രവർത്തനങ്ങൾ പരിശോധിക്കാൻ ആദ്യം action="providers.list" ഉപയോഗിച്ച് വിളിക്കുക. സാധാരണ: providers.list, settings.get, system.health, tools.list, users.list, upgrade.status.',
  },
  'nb-NO': {
    name: 'Konfigurasjon',
    description:
      'Konfigurasjons- og administrasjonsverktøy for runtime. Bruk formatet {domain}.{action}. Domener: providers, settings, channels, skills, tools, system, proxy, users, apikeys, upgrade. Kall først med action="providers.list" for å utforske tilgjengelige operasjoner. Vanlige: providers.list, settings.get, system.health, tools.list, users.list, upgrade.status.',
  },
  'nl-NL': {
    name: 'Configuratie',
    description:
      'Runtime-configuratie- en beheertool. Gebruik de indeling {domain}.{action}. Domeinen: providers, settings, channels, skills, tools, system, proxy, users, apikeys, upgrade. Roep eerst action="providers.list" aan om beschikbare bewerkingen te verkennen. Gebruikelijk: providers.list, settings.get, system.health, tools.list, users.list, upgrade.status.',
  },
  'pl-PL': {
    name: 'Konfiguracja',
    description:
      'Narzędzie do konfiguracji środowiska uruchomieniowego i administracji. Używaj formatu {domain}.{action}. Domeny: providers, settings, channels, skills, tools, system, proxy, users, apikeys, upgrade. Najpierw wywołaj action="providers.list", aby poznać dostępne operacje. Typowe: providers.list, settings.get, system.health, tools.list, users.list, upgrade.status.',
  },
  'pt-BR': {
    name: 'Configuração',
    description:
      'Ferramenta de configuração e administração do runtime. Use o formato {domain}.{action}. Domínios: providers, settings, channels, skills, tools, system, proxy, users, apikeys, upgrade. Chame primeiro com action="providers.list" para explorar as operações disponíveis. Comuns: providers.list, settings.get, system.health, tools.list, users.list, upgrade.status.',
  },
  'pt-PT': {
    name: 'Configuração',
    description:
      'Ferramenta de configuração e administração do runtime. Use o formato {domain}.{action}. Domínios: providers, settings, channels, skills, tools, system, proxy, users, apikeys, upgrade. Chame primeiro com action="providers.list" para explorar as operações disponíveis. Comuns: providers.list, settings.get, system.health, tools.list, users.list, upgrade.status.',
  },
  'ro-RO': {
    name: 'Configurare',
    description:
      'Instrument de configurare și administrare pentru runtime. Folosește formatul {domain}.{action}. Domenii: providers, settings, channels, skills, tools, system, proxy, users, apikeys, upgrade. Apelează mai întâi cu action="providers.list" pentru a explora operațiile disponibile. Comune: providers.list, settings.get, system.health, tools.list, users.list, upgrade.status.',
  },
  'ru-RU': {
    name: 'Конфиг',
    description:
      'Инструмент настройки и администрирования рантайма. Используйте формат {domain}.{action}. Домены: providers, settings, channels, skills, tools, system, proxy, users, apikeys, upgrade. Сначала вызовите action="providers.list", чтобы изучить доступные операции. Частые: providers.list, settings.get, system.health, tools.list, users.list, upgrade.status.',
  },
  'sk-SK': {
    name: 'Konfigurácia',
    description:
      'Nástroj na konfiguráciu runtime a správu. Používajte formát {domain}.{action}. Domény: providers, settings, channels, skills, tools, system, proxy, users, apikeys, upgrade. Najprv zavolajte action="providers.list", aby ste preskúmali dostupné operácie. Bežné: providers.list, settings.get, system.health, tools.list, users.list, upgrade.status.',
  },
  'sv-SE': {
    name: 'Konfiguration',
    description:
      'Verktyg för runtime-konfiguration och administration. Använd formatet {domain}.{action}. Domäner: providers, settings, channels, skills, tools, system, proxy, users, apikeys, upgrade. Anropa först med action="providers.list" för att utforska tillgängliga operationer. Vanliga: providers.list, settings.get, system.health, tools.list, users.list, upgrade.status.',
  },
  'zh-CN': {
    name: '配置',
    description:
      '运行时配置与管理工具。使用 {domain}.{action} 格式。域包括：providers、settings、channels、skills、tools、system、proxy、users、apikeys、upgrade。请先调用 action="providers.list" 以探索可用操作。常见操作：providers.list、settings.get、system.health、tools.list、users.list、upgrade.status。',
  },
  'zh-TW': {
    name: '設定',
    description:
      '執行階段設定與管理工具。使用 {domain}.{action} 格式。領域包括：providers、settings、channels、skills、tools、system、proxy、users、apikeys、upgrade。請先以 action="providers.list" 呼叫來探索可用操作。常見操作：providers.list、settings.get、system.health、tools.list、users.list、upgrade.status。',
  },
}

const convertLocaleTerms: Record<
  LocaleKey,
  {
    name: string
    description: string
  }
> = {
  'ca-ES': {
    name: 'Converteix',
    description:
      'Converteix fitxers locals, adjunts i sortides prèvies. La forma preferida és input_path + output_path amb camins relatius. Les tasques normals d’un sol fitxer retornen de manera síncrona; només les més pesades retornen async=true amb un task_id per consultar-les.',
  },
  'cs-CZ': {
    name: 'Převést',
    description:
      'Převádí místní soubory, přílohy a dřívější výstupy. Preferovaná forma je input_path + output_path s relativními cestami. Běžné úlohy s jedním souborem se vracejí synchronně; pouze náročnější vracejí async=true s task_id pro dotazování.',
  },
  'da-DK': {
    name: 'Konverter',
    description:
      'Konverterer lokale filer, vedhæftninger og tidligere output. Den foretrukne form er input_path + output_path med relative stier. Normale enkeltfiljobs returnerer synkront; kun tungere job returnerer async=true med et task_id til polling.',
  },
  'de-DE': {
    name: 'Konvertieren',
    description:
      'Konvertiert lokale Dateien, Anhänge und frühere Ausgaben. Die bevorzugte Form ist input_path + output_path mit relativen Pfaden. Normale Einzeldatei-Jobs kehren synchron zurück; nur schwerere Jobs liefern async=true mit einer task_id zum Abfragen.',
  },
  'el-GR': {
    name: 'Μετατροπή',
    description:
      'Μετατρέπει τοπικά αρχεία, συνημμένα και προηγούμενα αποτελέσματα. Η προτιμώμενη μορφή είναι input_path + output_path με σχετικές διαδρομές. Οι κανονικές εργασίες ενός αρχείου επιστρέφουν συγχρονισμένα· μόνο οι βαρύτερες επιστρέφουν async=true με task_id για έλεγχο κατάστασης.',
  },
  'en-GB': {
    name: 'Convert',
    description:
      'Convert local files, attachments, and prior outputs. Preferred form: input_path + output_path using relative paths. Normal single-file jobs return synchronously; only heavier jobs return async=true with a task_id for polling.',
  },
  'en-US': {
    name: 'Convert',
    description:
      'Convert local files, attachments, and prior outputs. Preferred form: input_path + output_path using relative paths. Normal single-file jobs return synchronously; only heavier jobs return async=true with a task_id for polling.',
  },
  'es-ES': {
    name: 'Convertir',
    description:
      'Convierte archivos locales, adjuntos y salidas previas. La forma preferida es input_path + output_path usando rutas relativas. Los trabajos normales de un solo archivo devuelven resultados de forma síncrona; solo los más pesados devuelven async=true con un task_id para consultarlos.',
  },
  'fr-FR': {
    name: 'Convertir',
    description:
      'Convertit les fichiers locaux, les pièces jointes et les sorties précédentes. La forme recommandée est input_path + output_path avec des chemins relatifs. Les tâches normales sur un seul fichier reviennent de façon synchrone ; seules les plus lourdes renvoient async=true avec un task_id pour le suivi.',
  },
  'ga-IE': {
    name: 'Tiontaigh',
    description:
      'Tiontaíonn sé comhaid áitiúla, ceangaltáin agus aschuir roimhe seo. Is é input_path + output_path le cosáin choibhneasta an fhoirm is fearr. Filleann gnáthphoist aon chomhaid go sioncronach; ní fhilleann ach poist níos troime async=true le task_id le haghaidh pobalbhreithe.',
  },
  'hr-HR': {
    name: 'Pretvori',
    description:
      'Pretvara lokalne datoteke, privitke i prethodne izlaze. Poželjni oblik je input_path + output_path s relativnim putanjama. Uobičajeni poslovi s jednom datotekom vraćaju se sinkrono; samo teži vraćaju async=true s task_id za provjeru.',
  },
  'hu-HU': {
    name: 'Konvertálás',
    description:
      'Helyi fájlokat, mellékleteket és korábbi kimeneteket alakít át. Az ajánlott forma: input_path + output_path relatív útvonalakkal. A normál egyfájlos feladatok szinkron módon térnek vissza; csak a nehezebbek adnak vissza async=true értéket lekérdezhető task_id-val.',
  },
  'it-IT': {
    name: 'Converti',
    description:
      'Converte file locali, allegati e output precedenti. La forma preferita è input_path + output_path usando percorsi relativi. I normali job con un solo file ritornano in modo sincrono; solo quelli più pesanti restituiscono async=true con un task_id da interrogare.',
  },
  'ja-JP': {
    name: '変換',
    description:
      'ローカルファイル、添付ファイル、以前の出力を変換します。推奨形式は相対パスを使う input_path + output_path です。通常の単一ファイルジョブは同期で返り、より重いジョブだけが状態確認用の task_id とともに async=true を返します。',
  },
  'ko-KR': {
    name: '변환',
    description:
      '로컬 파일, 첨부파일, 이전 출력물을 변환합니다. 권장 형식은 상대 경로를 사용하는 input_path + output_path 입니다. 일반적인 단일 파일 작업은 동기적으로 반환되며, 더 무거운 작업만 상태 조회용 task_id와 함께 async=true를 반환합니다.',
  },
  'ml-IN': {
    name: 'പരിവർത്തനം',
    description:
      'ലോക്കൽ ഫയലുകളും അറ്റാച്ച്മെന്റുകളും മുൻ ഔട്ട്പുട്ടുകളും പരിവർത്തനം ചെയ്യുന്നു. മുൻഗണന നൽകുന്ന രൂപം റിലേറ്റീവ് പാത്തുകൾ ഉപയോഗിക്കുന്ന input_path + output_path ആണ്. സാധാരണ സിംഗിൾ-ഫൈൽ ജോബുകൾ സിങ്ക്രോണസായി മടങ്ങും; കൂടുതൽ ഭാരമുള്ള ജോബുകൾ മാത്രം പോളിംഗിനായി task_id സഹിതം async=true ആയി മടങ്ങും.',
  },
  'nb-NO': {
    name: 'Konverter',
    description:
      'Konverterer lokale filer, vedlegg og tidligere utdata. Foretrukket form er input_path + output_path med relative stier. Vanlige enkeltfiljobber returnerer synkront; bare tyngre jobber returnerer async=true med en task_id for polling.',
  },
  'nl-NL': {
    name: 'Converteren',
    description:
      'Converteert lokale bestanden, bijlagen en eerdere uitvoer. De voorkeursvorm is input_path + output_path met relatieve paden. Normale taken met één bestand keren synchroon terug; alleen zwaardere taken geven async=true terug met een task_id om te pollen.',
  },
  'pl-PL': {
    name: 'Konwertuj',
    description:
      'Konwertuje lokalne pliki, załączniki i wcześniejsze wyniki. Preferowana forma to input_path + output_path z użyciem ścieżek względnych. Zwykłe zadania na jednym pliku zwracają wynik synchronicznie; tylko cięższe zwracają async=true z task_id do odpytywania.',
  },
  'pt-BR': {
    name: 'Converter',
    description:
      'Converte arquivos locais, anexos e saídas anteriores. A forma preferida é input_path + output_path usando caminhos relativos. Trabalhos normais de arquivo único retornam de forma síncrona; apenas os mais pesados retornam async=true com um task_id para consulta.',
  },
  'pt-PT': {
    name: 'Converter',
    description:
      'Converte ficheiros locais, anexos e resultados anteriores. A forma preferida é input_path + output_path usando caminhos relativos. Os trabalhos normais de um único ficheiro devolvem resultados de forma síncrona; apenas os mais pesados devolvem async=true com um task_id para consulta.',
  },
  'ro-RO': {
    name: 'Convertește',
    description:
      'Convertește fișiere locale, atașamente și rezultate anterioare. Forma preferată este input_path + output_path folosind căi relative. Joburile normale pe un singur fișier revin sincron; doar cele mai grele returnează async=true cu un task_id pentru interogare.',
  },
  'ru-RU': {
    name: 'Конвертировать',
    description:
      'Преобразует локальные файлы, вложения и предыдущие результаты. Предпочтительная форма: input_path + output_path с относительными путями. Обычные задачи с одним файлом возвращаются синхронно; только более тяжёлые возвращают async=true с task_id для опроса.',
  },
  'sk-SK': {
    name: 'Konvertovať',
    description:
      'Konvertuje lokálne súbory, prílohy a predchádzajúce výstupy. Uprednostňovaná forma je input_path + output_path s relatívnymi cestami. Bežné úlohy s jedným súborom sa vracajú synchrónne; iba náročnejšie vracajú async=true s task_id na zisťovanie stavu.',
  },
  'sv-SE': {
    name: 'Konvertera',
    description:
      'Konverterar lokala filer, bilagor och tidigare utdata. Föredragen form är input_path + output_path med relativa sökvägar. Normala jobb med en fil returnerar synkront; endast tyngre jobb returnerar async=true med ett task_id för polling.',
  },
  'zh-CN': {
    name: '转换',
    description:
      '转换本地文件、附件和先前输出。推荐形式是使用相对路径的 input_path + output_path。普通单文件任务会同步返回；只有更重的任务才会返回 async=true，并附带用于轮询的 task_id。',
  },
  'zh-TW': {
    name: '轉換',
    description:
      '轉換本機檔案、附件與先前輸出。建議形式是使用相對路徑的 input_path + output_path。一般單檔工作會同步回傳；只有較重的工作才會回傳 async=true，並附帶可輪詢的 task_id。',
  },
}

const advisorLocaleTerms: Record<
  LocaleKey,
  {
    name: string
    description: string
  }
> = {
  'ca-ES': {
    name: 'Assessor',
    description:
      'Assessor de decisions per a preguntes de selecció, substitució, migració i bones pràctiques.',
  },
  'cs-CZ': {
    name: 'Poradce',
    description:
      'Poradce pro rozhodování u otázek výběru, náhrady, migrace a osvědčených postupů.',
  },
  'da-DK': {
    name: 'Rådgiver',
    description:
      'Beslutningsrådgiver til spørgsmål om valg, udskiftning, migrering og bedste praksis.',
  },
  'de-DE': {
    name: 'Berater',
    description:
      'Entscheidungsberater für Fragen zu Auswahl, Ersatz, Migration und Best Practices.',
  },
  'el-GR': {
    name: 'Σύμβουλος',
    description:
      'Σύμβουλος αποφάσεων για ερωτήσεις επιλογής, αντικατάστασης, μετεγκατάστασης και βέλτιστων πρακτικών.',
  },
  'en-GB': {
    name: 'Advisor',
    description:
      'Decision advisor for selection, replacement, migration, and best-practice questions.',
  },
  'en-US': {
    name: 'Advisor',
    description:
      'Decision advisor for selection, replacement, migration, and best-practice questions.',
  },
  'es-ES': {
    name: 'Asesor',
    description:
      'Asesor de decisiones para preguntas de selección, reemplazo, migración y buenas prácticas.',
  },
  'fr-FR': {
    name: 'Conseiller',
    description:
      'Conseiller décisionnel pour les questions de sélection, de remplacement, de migration et de bonnes pratiques.',
  },
  'ga-IE': {
    name: 'Comhairleoir',
    description:
      'Comhairleoir cinntí do cheisteanna roghnúcháin, athsholáthair, imirce agus dea-chleachtais.',
  },
  'hr-HR': {
    name: 'Savjetnik',
    description:
      'Savjetnik za odluke za pitanja odabira, zamjene, migracije i dobre prakse.',
  },
  'hu-HU': {
    name: 'Tanácsadó',
    description:
      'Döntési tanácsadó kiválasztási, lecserélési, migrációs és bevált gyakorlatokkal kapcsolatos kérdésekhez.',
  },
  'it-IT': {
    name: 'Consulente',
    description:
      'Consulente decisionale per domande su selezione, sostituzione, migrazione e buone pratiche.',
  },
  'ja-JP': {
    name: 'アドバイザー',
    description:
      '選定、置き換え、移行、ベストプラクティスに関する判断を支援するアドバイザー。',
  },
  'ko-KR': {
    name: '어드바이저',
    description:
      '선정, 대체, 마이그레이션, 모범 사례 관련 질문을 위한 의사결정 어드바이저.',
  },
  'ml-IN': {
    name: 'ഉപദേശകൻ',
    description:
      'തിരഞ്ഞെടുപ്പ്, പകരംവയ്‌പ്പ്, മൈഗ്രേഷൻ, മികച്ച പ്രാക്ടീസ് ചോദ്യങ്ങൾക്കായുള്ള തീരുമാന ഉപദേശകൻ.',
  },
  'nb-NO': {
    name: 'Rådgiver',
    description:
      'Beslutningsrådgiver for spørsmål om valg, utskifting, migrering og beste praksis.',
  },
  'nl-NL': {
    name: 'Adviseur',
    description:
      'Beslissingsadviseur voor vragen over selectie, vervanging, migratie en best practices.',
  },
  'pl-PL': {
    name: 'Doradca',
    description:
      'Doradca decyzyjny do pytań o wybór, zastąpienie, migrację i dobre praktyki.',
  },
  'pt-BR': {
    name: 'Consultor',
    description:
      'Consultor de decisão para perguntas sobre seleção, substituição, migração e boas práticas.',
  },
  'pt-PT': {
    name: 'Consultor',
    description:
      'Consultor de decisão para questões de seleção, substituição, migração e boas práticas.',
  },
  'ro-RO': {
    name: 'Consilier',
    description:
      'Consilier pentru decizii privind întrebări de selecție, înlocuire, migrare și bune practici.',
  },
  'ru-RU': {
    name: 'Советник',
    description:
      'Помощник по принятию решений для вопросов выбора, замены, миграции и лучших практик.',
  },
  'sk-SK': {
    name: 'Poradca',
    description:
      'Rozhodovací poradca pre otázky výberu, náhrady, migrácie a osvedčených postupov.',
  },
  'sv-SE': {
    name: 'Rådgivare',
    description:
      'Beslutsrådgivare för frågor om val, ersättning, migrering och bästa praxis.',
  },
  'zh-CN': {
    name: '顾问',
    description: '用于选型、替换、迁移和最佳实践问题的决策顾问。',
  },
  'zh-TW': {
    name: '顧問',
    description: '用於選型、替換、遷移與最佳實務問題的決策顧問。',
  },
}

const ocrLocaleTerms: Record<LocaleKey, string> = {
  'ca-ES': "Extreu text d'imatges, captures de pantalla o fitxers escanejats mitjançant OCR.",
  'cs-CZ': 'Extrahuje text z obrázků, snímků obrazovky nebo skenovaných souborů pomocí OCR.',
  'da-DK': 'Udtrækker tekst fra billeder, skærmbilleder eller scannede filer ved hjælp af OCR.',
  'de-DE': 'Extrahiert Text aus Bildern, Screenshots oder gescannten Dateien mit OCR.',
  'el-GR': 'Εξάγει κείμενο από εικόνες, στιγμιότυπα οθόνης ή σαρωμένα αρχεία με χρήση OCR.',
  'en-GB': 'Extract text from images, screenshots, or scanned files using OCR.',
  'en-US': 'Extract text from images, screenshots, or scanned files using OCR.',
  'es-ES': 'Extrae texto de imágenes, capturas de pantalla o archivos escaneados mediante OCR.',
  'fr-FR': "Extrait le texte des images, captures d'écran ou fichiers numérisés à l'aide de l'OCR.",
  'ga-IE': 'Baineann sé téacs as íomhánna, seatanna scáileáin nó comhaid scanta trí OCR a úsáid.',
  'hr-HR': 'Izdvaja tekst iz slika, snimki zaslona ili skeniranih datoteka pomoću OCR-a.',
  'hu-HU': 'OCR segítségével szöveget nyer ki képekből, képernyőképekből vagy szkennelt fájlokból.',
  'it-IT': "Estrae testo da immagini, schermate o file scansionati usando l'OCR.",
  'ja-JP': 'OCR を使って画像、スクリーンショット、またはスキャンしたファイルからテキストを抽出します。',
  'ko-KR': 'OCR을 사용해 이미지, 스크린샷 또는 스캔된 파일에서 텍스트를 추출합니다.',
  'ml-IN':
    'OCR ഉപയോഗിച്ച് ചിത്രങ്ങൾ, സ്ക്രീൻഷോട്ടുകൾ, അല്ലെങ്കിൽ സ്കാൻ ചെയ്ത ഫയലുകളിൽ നിന്ന് ടെക്സ്റ്റ് പുറത്തെടുക്കുന്നു.',
  'nb-NO': 'Henter ut tekst fra bilder, skjermbilder eller skannede filer ved hjelp av OCR.',
  'nl-NL': 'Haalt tekst uit afbeeldingen, schermafbeeldingen of gescande bestanden met OCR.',
  'pl-PL': 'Wyodrębnia tekst z obrazów, zrzutów ekranu lub zeskanowanych plików za pomocą OCR.',
  'pt-BR': 'Extrai texto de imagens, capturas de tela ou arquivos digitalizados usando OCR.',
  'pt-PT': 'Extrai texto de imagens, capturas de ecrã ou ficheiros digitalizados usando OCR.',
  'ro-RO': 'Extrage text din imagini, capturi de ecran sau fișiere scanate folosind OCR.',
  'ru-RU': 'Извлекает текст из изображений, снимков экрана или отсканированных файлов с помощью OCR.',
  'sk-SK': 'Extrahuje text z obrázkov, snímok obrazovky alebo naskenovaných súborov pomocou OCR.',
  'sv-SE': 'Extraherar text från bilder, skärmdumpar eller skannade filer med OCR.',
  'zh-CN': '使用 OCR 从图像、屏幕截图或扫描文件中提取文本。',
  'zh-TW': '使用 OCR 從圖像、螢幕截圖或掃描檔案中擷取文字。',
}

const builtinToolBackfills = Object.fromEntries(
  Object.entries(visibleBuiltinToolLocaleTerms).map(([locale, terms]) => {
    const fileDeleteTerms = fileDeleteLocaleTerms[locale as LocaleKey]
    const configTerms = configLocaleTerms[locale as LocaleKey]
    const convertTerms = convertLocaleTerms[locale as LocaleKey]
    const advisorTerms = advisorLocaleTerms[locale as LocaleKey]
    const ocrDescription = ocrLocaleTerms[locale as LocaleKey]
    const nativeDocumentTerms = nativeDocumentToolBackfills[locale as LocaleKey]

    return [
      locale,
      {
        tools: {
          names: {
            find: terms.findName,
            ls: terms.lsName,
            tool_search: terms.toolSearchName,
            agents_list: terms.agentsListName,
            advisor: advisorTerms.name,
            bash: terms.bashName,
            canvas: terms.canvasName,
            config: configTerms.name,
            convert: convertTerms.name,
            docx: 'DOCX',
            edit: terms.editName,
            file_delete: fileDeleteTerms.name,
            gateway: terms.gatewayName,
            grep: terms.grepName,
            image: terms.imageName,
            nodes: terms.nodesName,
            office: terms.officeName,
            ocr: 'OCR',
            pdf: terms.pdfName,
            pptx: 'PPTX',
            subagents: terms.subagentsName,
            tts: terms.ttsName,
            xlsx: 'XLSX',
          },
          descriptions: {
            find: terms.findDescription,
            ls: terms.lsDescription,
            tool_search: terms.toolSearchDescription,
            agents_list: terms.agentsListDescription,
            advisor: advisorTerms.description,
            bash: terms.bashDescription,
            canvas: terms.canvasDescription,
            config: configTerms.description,
            convert: convertTerms.description,
            docx: nativeDocumentTerms.docxDescription,
            edit: terms.editDescription,
            file_delete: fileDeleteTerms.description,
            gateway: terms.gatewayDescription,
            grep: terms.grepDescription,
            image: terms.imageDescription,
            nodes: terms.nodesDescription,
            office: terms.officeDescription,
            ocr: ocrDescription,
            pdf: nativeDocumentTerms.pdfDescription,
            pptx: nativeDocumentTerms.pptxDescription,
            subagents: terms.subagentsDescription,
            tts: terms.ttsDescription,
            mcp: terms.mcpDescription,
            memory: terms.memoryDescription,
            xlsx: nativeDocumentTerms.xlsxDescription,
          },
        },
      },
    ]
  })
) as Partial<Record<LocaleKey, LocaleNode>>

export default builtinToolBackfills
