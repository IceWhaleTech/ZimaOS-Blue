import type { LocaleKey } from './locale-catalog'

type AgentcoreRunnerPartDescriptionSettings = Record<string, unknown>

export const agentcoreRunnerPartDescriptionBase: AgentcoreRunnerPartDescriptionSettings = {
  partDescriptions: {
    constraints: 'Defines the hard limits and guardrails the runner must follow.',
    skill_definition: 'Describes the skill contract, responsibilities, and expected capabilities.',
    prompt_template: 'Shapes the reusable instructions and response structure sent to the model.',
    context_assembly:
      'Controls how evidence, state, and workspace context are gathered before each run.',
    coordinator_policy: 'Decides how top-level tasks are broken down, sequenced, and handed off.',
    orchestrator_policy: 'Governs multi-step flow control, retries, and cross-stage coordination.',
    tool_exposure: 'Chooses which tools are available to the runner and how they are presented.',
    verification_policy: 'Defines how outputs are checked before they are accepted or persisted.',
    runner_code: 'Implements the runtime logic that executes the agent loop and integrations.',
    build_recipe: 'Specifies how the runner is prepared, built, and packaged for execution.',
  },
}

export const agentcoreRunnerPartDescriptionOverrides: Partial<
  Record<LocaleKey, AgentcoreRunnerPartDescriptionSettings>
> = {
  'ca-ES': {
    partDescriptions: {
      constraints: 'Defineix els límits rígids i les salvaguardes que ha de seguir el runner.',
      skill_definition:
        'Descriu el contracte de l’habilitat, les responsabilitats i les capacitats esperades.',
      prompt_template:
        'Dona forma a les instruccions reutilitzables i a l’estructura de resposta enviada al model.',
      context_assembly:
        'Controla com es reuneixen les proves, l’estat i el context de l’espai de treball abans de cada execució.',
      coordinator_policy:
        'Decideix com es descomponen, s’ordenen i es deleguen les tasques de nivell superior.',
      orchestrator_policy:
        'Regula el control del flux multietapa, els reintents i la coordinació entre etapes.',
      tool_exposure: 'Tria quines eines estan disponibles per al runner i com es presenten.',
      verification_policy:
        'Defineix com es comproven les sortides abans d’acceptar-les o persistir-les.',
      runner_code:
        'Implementa la lògica d’execució que governa el bucle de l’agent i les integracions.',
      build_recipe:
        'Especifica com es prepara, es compila i s’empaqueta el runner per executar-lo.',
    },
  },
  'cs-CZ': {
    partDescriptions: {
      constraints: 'Definuje pevné limity a mantinely, které musí runner dodržovat.',
      skill_definition: 'Popisuje kontrakt dovednosti, odpovědnosti a očekávané schopnosti.',
      prompt_template:
        'Určuje opakovaně použitelné instrukce a strukturu odpovědi posílanou modelu.',
      context_assembly:
        'Řídí, jak se před každým během shromažďují důkazy, stav a kontext pracovního prostoru.',
      coordinator_policy: 'Rozhoduje, jak se úkoly nejvyšší úrovně rozdělují, řadí a předávají.',
      orchestrator_policy: 'Řídí vícekrokový tok, opakování pokusů a koordinaci mezi fázemi.',
      tool_exposure: 'Určuje, které nástroje má runner k dispozici a jak jsou prezentovány.',
      verification_policy: 'Definuje, jak se výstupy kontrolují před přijetím nebo uložením.',
      runner_code: 'Implementuje běhovou logiku, která vykonává smyčku agenta a integrace.',
      build_recipe: 'Určuje, jak se runner připravuje, sestavuje a balí pro spuštění.',
    },
  },
  'da-DK': {
    partDescriptions: {
      constraints: 'Definerer de faste grænser og værn, som runneren skal følge.',
      skill_definition: 'Beskriver skill-kontrakten, ansvarsområderne og de forventede evner.',
      prompt_template:
        'Former de genbrugelige instruktioner og svarstrukturen, der sendes til modellen.',
      context_assembly:
        'Styrer, hvordan evidens, tilstand og arbejdsområdekontekst samles før hvert kørsel.',
      coordinator_policy:
        'Afgør, hvordan opgaver på øverste niveau opdeles, sekventeres og overdrages.',
      orchestrator_policy: 'Styrer flertrins-flow, genforsøg og koordinering på tværs af faser.',
      tool_exposure:
        'Vælger hvilke værktøjer der er tilgængelige for runneren, og hvordan de præsenteres.',
      verification_policy:
        'Definerer, hvordan output kontrolleres, før det accepteres eller gemmes.',
      runner_code: 'Implementerer runtime-logikken, der udfører agent-loopet og integrationerne.',
      build_recipe: 'Angiver, hvordan runneren forberedes, bygges og pakkes til eksekvering.',
    },
  },
  'de-DE': {
    partDescriptions: {
      constraints: 'Definiert die festen Grenzen und Leitplanken, denen der Runner folgen muss.',
      skill_definition:
        'Beschreibt den Skill-Vertrag, die Verantwortlichkeiten und die erwarteten Fähigkeiten.',
      prompt_template:
        'Formt die wiederverwendbaren Anweisungen und die Antwortstruktur, die an das Modell gesendet werden.',
      context_assembly:
        'Steuert, wie Belege, Status und Workspace-Kontext vor jedem Lauf gesammelt werden.',
      coordinator_policy:
        'Entscheidet, wie Aufgaben auf oberster Ebene zerlegt, sequenziert und übergeben werden.',
      orchestrator_policy:
        'Steuert mehrstufige Abläufe, Wiederholungen und die Koordination zwischen Phasen.',
      tool_exposure:
        'Legt fest, welche Werkzeuge dem Runner zur Verfügung stehen und wie sie dargestellt werden.',
      verification_policy:
        'Definiert, wie Ausgaben geprüft werden, bevor sie akzeptiert oder gespeichert werden.',
      runner_code:
        'Implementiert die Laufzeitlogik, die die Agentenschleife und Integrationen ausführt.',
      build_recipe:
        'Legt fest, wie der Runner für die Ausführung vorbereitet, gebaut und verpackt wird.',
    },
  },
  'el-GR': {
    partDescriptions: {
      constraints: 'Ορίζει τα αυστηρά όρια και τις δικλείδες που πρέπει να ακολουθεί ο runner.',
      skill_definition:
        'Περιγράφει το συμβόλαιο της δεξιότητας, τις ευθύνες και τις αναμενόμενες δυνατότητες.',
      prompt_template:
        'Διαμορφώνει τις επαναχρησιμοποιήσιμες οδηγίες και τη δομή απάντησης που αποστέλλονται στο μοντέλο.',
      context_assembly:
        'Ελέγχει πώς συγκεντρώνονται τα στοιχεία, η κατάσταση και το context του workspace πριν από κάθε εκτέλεση.',
      coordinator_policy:
        'Αποφασίζει πώς αναλύονται, δρομολογούνται και ανατίθενται οι εργασίες ανώτερου επιπέδου.',
      orchestrator_policy:
        'Ρυθμίζει τον πολυβηματικό έλεγχο ροής, τις επαναλήψεις και τον συντονισμό μεταξύ σταδίων.',
      tool_exposure: 'Επιλέγει ποια εργαλεία είναι διαθέσιμα στον runner και πώς παρουσιάζονται.',
      verification_policy:
        'Ορίζει πώς ελέγχονται τα αποτελέσματα πριν γίνουν αποδεκτά ή αποθηκευτούν.',
      runner_code:
        'Υλοποιεί τη λογική εκτέλεσης που τρέχει τον βρόχο του agent και τις ενσωματώσεις.',
      build_recipe:
        'Καθορίζει πώς προετοιμάζεται, γίνεται build και πακετάρεται ο runner για εκτέλεση.',
    },
  },
  'en-GB': {
    partDescriptions: {
      constraints: 'Defines the hard limits and guardrails the runner must follow.',
      skill_definition:
        'Describes the skill contract, responsibilities, and expected capabilities.',
      prompt_template:
        'Shapes the reusable instructions and response structure sent to the model.',
      context_assembly:
        'Controls how evidence, state, and workspace context are gathered before each run.',
      coordinator_policy: 'Decides how top-level tasks are broken down, sequenced, and handed off.',
      orchestrator_policy: 'Governs multi-step flow control, retries, and cross-stage coordination.',
      tool_exposure: 'Chooses which tools are available to the runner and how they are presented.',
      verification_policy:
        'Defines how outputs are checked before they are accepted or persisted.',
      runner_code: 'Implements the runtime logic that executes the agent loop and integrations.',
      build_recipe: 'Specifies how the runner is prepared, built, and packaged for execution.',
    },
  },
  'es-ES': {
    partDescriptions: {
      constraints: 'Define los límites rígidos y las salvaguardas que debe seguir el runner.',
      skill_definition:
        'Describe el contrato de la skill, las responsabilidades y las capacidades esperadas.',
      prompt_template:
        'Da forma a las instrucciones reutilizables y a la estructura de respuesta que se envía al modelo.',
      context_assembly:
        'Controla cómo se reúnen las evidencias, el estado y el contexto del espacio de trabajo antes de cada ejecución.',
      coordinator_policy:
        'Decide cómo se descomponen, ordenan y delegan las tareas de nivel superior.',
      orchestrator_policy:
        'Gobierna el flujo de varios pasos, los reintentos y la coordinación entre etapas.',
      tool_exposure: 'Elige qué herramientas están disponibles para el runner y cómo se presentan.',
      verification_policy:
        'Define cómo se verifican las salidas antes de aceptarlas o persistirlas.',
      runner_code:
        'Implementa la lógica de ejecución que lleva el bucle del agente y las integraciones.',
      build_recipe: 'Especifica cómo se prepara, compila y empaqueta el runner para ejecutarlo.',
    },
  },
  'fr-FR': {
    partDescriptions: {
      constraints: 'Définit les limites strictes et les garde-fous que le runner doit respecter.',
      skill_definition:
        'Décrit le contrat de la compétence, les responsabilités et les capacités attendues.',
      prompt_template:
        'Structure les instructions réutilisables et le format de réponse envoyés au modèle.',
      context_assembly:
        'Contrôle la façon dont les preuves, l’état et le contexte du workspace sont rassemblés avant chaque exécution.',
      coordinator_policy:
        'Décide comment les tâches de plus haut niveau sont découpées, ordonnées et transmises.',
      orchestrator_policy:
        'Régit le contrôle de flux multi-étapes, les nouvelles tentatives et la coordination entre étapes.',
      tool_exposure:
        'Choisit quels outils sont disponibles pour le runner et comment ils sont présentés.',
      verification_policy:
        'Définit la manière dont les sorties sont vérifiées avant d’être acceptées ou persistées.',
      runner_code:
        'Implémente la logique d’exécution qui fait tourner la boucle d’agent et les intégrations.',
      build_recipe: 'Précise comment le runner est préparé, compilé et empaqueté pour l’exécution.',
    },
  },
  'ga-IE': {
    partDescriptions: {
      constraints:
        'Sainmhíníonn sé na teorainneacha crua agus na ráillí cosanta nach mór don runner a leanúint.',
      skill_definition:
        'Déanann sé cur síos ar chonradh na scile, ar na freagrachtaí agus ar na cumais a bhfuiltear ag súil leo.',
      prompt_template:
        'Múnlaíonn sé na treoracha in-athúsáidte agus struchtúr na freagartha a sheoltar chuig an tsamhail.',
      context_assembly:
        'Rialaíonn sé conas a bhailítear fianaise, staid agus comhthéacs an spáis oibre roimh gach rith.',
      coordinator_policy:
        'Cinneann sé conas a bhristear síos, a sheichimhítear agus a aistrítear tascanna ardleibhéil.',
      orchestrator_policy:
        'Rialaíonn sé rialú sreafa ilchéime, atrialacha agus comhordú traschéime.',
      tool_exposure:
        'Roghnaíonn sé na huirlisí atá ar fáil don runner agus an chaoi a gcuirtear i láthair iad.',
      verification_policy:
        'Sainmhíníonn sé conas a sheiceáltar aschuir sula nglactar leo nó sula sábháiltear iad.',
      runner_code:
        'Cuireann sé i bhfeidhm an loighic ama rite a ritheann lúb an ghníomhaire agus na comhtháthuithe.',
      build_recipe:
        'Sonraíonn sé conas a ullmhaítear, a thógtar agus a phacáistítear an runner lena rith.',
    },
  },
  'hr-HR': {
    partDescriptions: {
      constraints:
        'Definira čvrsta ograničenja i zaštitne okvire kojih se runner mora pridržavati.',
      skill_definition: 'Opisuje ugovor skilla, odgovornosti i očekivane sposobnosti.',
      prompt_template: 'Oblikuje višekratne upute i strukturu odgovora koja se šalje modelu.',
      context_assembly:
        'Upravlja time kako se prije svakog izvođenja prikupljaju dokazi, stanje i kontekst radnog prostora.',
      coordinator_policy: 'Odlučuje kako se zadaci najviše razine razlažu, redaju i predaju dalje.',
      orchestrator_policy:
        'Upravlja višekoračnim tokom, ponovnim pokušajima i koordinacijom između faza.',
      tool_exposure: 'Odabire koji su alati dostupni runneru i kako su predstavljeni.',
      verification_policy:
        'Definira kako se izlazi provjeravaju prije prihvaćanja ili trajne pohrane.',
      runner_code: 'Implementira runtime logiku koja izvršava petlju agenta i integracije.',
      build_recipe: 'Određuje kako se runner priprema, gradi i pakira za izvršavanje.',
    },
  },
  'hu-HU': {
    partDescriptions: {
      constraints:
        'Meghatározza a szigorú korlátokat és védőkereteket, amelyeket a runnernek követnie kell.',
      skill_definition: 'Leírja a skill szerződését, felelősségi köreit és az elvárt képességeket.',
      prompt_template:
        'Formálja az újrahasználható utasításokat és a modellnek küldött válaszstruktúrát.',
      context_assembly:
        'Szabályozza, hogyan gyűlnek össze a bizonyítékok, az állapot és a munkaterület kontextusa minden futás előtt.',
      coordinator_policy:
        'Eldönti, hogyan bontsák fel, ütemezzék és adják tovább a felső szintű feladatokat.',
      orchestrator_policy:
        'Irányítja a több lépéses folyamatvezérlést, az újrapróbálásokat és a szakaszok közötti koordinációt.',
      tool_exposure:
        'Kiválasztja, mely eszközök érhetők el a runner számára és hogyan jelenjenek meg.',
      verification_policy:
        'Meghatározza, hogyan ellenőrizzék a kimeneteket elfogadás vagy tárolás előtt.',
      runner_code:
        'Megvalósítja azt a futásidejű logikát, amely az ágensciklust és az integrációkat végrehajtja.',
      build_recipe:
        'Meghatározza, hogyan készül fel, épül fel és csomagolódik a runner a futtatáshoz.',
    },
  },
  'it-IT': {
    partDescriptions: {
      constraints: 'Definisce i limiti rigidi e i guardrail che il runner deve rispettare.',
      skill_definition:
        'Descrive il contratto della skill, le responsabilità e le capacità attese.',
      prompt_template:
        'Modella le istruzioni riutilizzabili e la struttura della risposta inviata al modello.',
      context_assembly:
        'Controlla come vengono raccolti evidenze, stato e contesto del workspace prima di ogni esecuzione.',
      coordinator_policy:
        'Decide come i task di livello superiore vengono scomposti, ordinati e passati.',
      orchestrator_policy: 'Governa il flusso multi-step, i retry e il coordinamento tra le fasi.',
      tool_exposure:
        'Sceglie quali strumenti sono disponibili per il runner e come vengono presentati.',
      verification_policy:
        'Definisce come gli output vengono verificati prima di essere accettati o persistiti.',
      runner_code: 'Implementa la logica runtime che esegue il loop dell’agente e le integrazioni.',
      build_recipe:
        'Specifica come il runner viene preparato, compilato e impacchettato per l’esecuzione.',
    },
  },
  'ja-JP': {
    partDescriptions: {
      constraints: '実行系が従うべき厳格な制約とガードレールを定義します。',
      skill_definition: 'スキルの契約、責務、期待される能力を説明します。',
      prompt_template: 'モデルへ送る再利用可能な指示と応答構造を形作ります。',
      context_assembly: '各実行の前に、証拠・状態・ワークスペース文脈をどう集めるかを制御します。',
      coordinator_policy: '上位タスクをどう分解し、順序付けし、引き渡すかを決めます。',
      orchestrator_policy: '複数段階のフロー制御、再試行、段階間の調整を司ります。',
      tool_exposure: 'どのツールを実行系に公開し、どう見せるかを決めます。',
      verification_policy: '出力を受け入れたり保存したりする前に、どう検証するかを定義します。',
      runner_code: 'エージェントループと各種統合を実行するランタイムロジックを実装します。',
      build_recipe: '実行用に runner をどう準備し、ビルドし、パッケージ化するかを指定します。',
    },
  },
  'ko-KR': {
    partDescriptions: {
      constraints: '러너가 따라야 하는 엄격한 한계와 가드레일을 정의합니다.',
      skill_definition: '스킬 계약, 책임, 기대되는 역량을 설명합니다.',
      prompt_template: '모델에 보내는 재사용 가능한 지시문과 응답 구조를 설계합니다.',
      context_assembly:
        '각 실행 전에 근거, 상태, 워크스페이스 컨텍스트를 어떻게 수집할지 제어합니다.',
      coordinator_policy: '상위 수준 작업을 어떻게 분해하고, 순서를 정하고, 넘길지 결정합니다.',
      orchestrator_policy: '다단계 흐름 제어, 재시도, 단계 간 조정을 관리합니다.',
      tool_exposure: '러너에서 어떤 도구를 사용할 수 있게 할지와 그 표시 방식을 정합니다.',
      verification_policy: '출력을 수용하거나 저장하기 전에 어떻게 검증할지 정의합니다.',
      runner_code: '에이전트 루프와 통합을 실행하는 런타임 로직을 구현합니다.',
      build_recipe: '실행을 위해 러너를 어떻게 준비하고, 빌드하고, 패키징할지 지정합니다.',
    },
  },
  'ml-IN': {
    partDescriptions: {
      constraints: 'റണ്ണർ പാലിക്കേണ്ട കടുത്ത പരിധികളും സുരക്ഷാ നിയന്ത്രണങ്ങളും നിർവചിക്കുന്നു.',
      skill_definition:
        'സ്കിൽ കരാർ, ഉത്തരവാദിത്വങ്ങൾ, പ്രതീക്ഷിക്കുന്ന കഴിവുകൾ എന്നിവ വിവരിക്കുന്നു.',
      prompt_template:
        'മോഡലിലേക്ക് അയക്കുന്ന പുനരുപയോഗിക്കാവുന്ന നിർദ്ദേശങ്ങളും മറുപടി ഘടനയും രൂപപ്പെടുത്തുന്നു.',
      context_assembly:
        'ഓരോ റണ്ണിനും മുമ്പ് തെളിവുകളും അവസ്ഥയും വർക്ക്‌സ്‌പേസ് സാഹചര്യവും എങ്ങനെ ശേഖരിക്കണമെന്ന് നിയന്ത്രിക്കുന്നു.',
      coordinator_policy:
        'ഉയർന്ന തലത്തിലുള്ള ജോലികൾ എങ്ങനെ വിഭജിക്കണം, ക്രമപ്പെടുത്തണം, കൈമാറണം എന്ന് തീരുമാനിക്കുന്നു.',
      orchestrator_policy:
        'പല ഘട്ടങ്ങളുള്ള പ്രവാഹ നിയന്ത്രണം, റീട്രൈകൾ, ഘട്ടങ്ങൾക്കിടയിലെ ഏകോപനം എന്നിവ നിയന്ത്രിക്കുന്നു.',
      tool_exposure:
        'റണ്ണറിന് ലഭ്യമായ ഉപകരണങ്ങളും അവ എങ്ങനെ അവതരിപ്പിക്കണമെന്നതും തിരഞ്ഞെടുക്കുന്നു.',
      verification_policy:
        'ഫലങ്ങൾ സ്വീകരിക്കുകയോ സ്ഥിരപ്പെടുത്തുകയോ ചെയ്യുന്നതിന് മുമ്പ് എങ്ങനെ പരിശോധിക്കണമെന്ന് നിർവചിക്കുന്നു.',
      runner_code:
        'ഏജന്റ് ലൂപ്പും ഇന്റഗ്രേഷനുകളും പ്രവർത്തിപ്പിക്കുന്ന റൺടൈം ലാജിക്ക് നടപ്പിലാക്കുന്നു.',
      build_recipe:
        'പ്രവർത്തനത്തിനായി റണ്ണർ എങ്ങനെ തയ്യാറാക്കണം, build ചെയ്യണം, package ചെയ്യണം എന്ന് വ്യക്തമാക്കുന്നു.',
    },
  },
  'nb-NO': {
    partDescriptions: {
      constraints: 'Definerer de faste grensene og vernene runneren må følge.',
      skill_definition: 'Beskriver skill-kontrakten, ansvarsområdene og de forventede evnene.',
      prompt_template:
        'Former de gjenbrukbare instruksjonene og svarstrukturen som sendes til modellen.',
      context_assembly:
        'Styrer hvordan bevis, tilstand og arbeidsområdekontekst samles inn før hver kjøring.',
      coordinator_policy:
        'Bestemmer hvordan oppgaver på toppnivå brytes ned, sekvenseres og overleveres.',
      orchestrator_policy:
        'Styrer flertrinns flytkontroll, nye forsøk og koordinering mellom faser.',
      tool_exposure:
        'Velger hvilke verktøy som er tilgjengelige for runneren og hvordan de presenteres.',
      verification_policy: 'Definerer hvordan resultater kontrolleres før de godtas eller lagres.',
      runner_code: 'Implementerer kjøretidslogikken som utfører agentløkken og integrasjonene.',
      build_recipe: 'Angir hvordan runneren forberedes, bygges og pakkes for kjøring.',
    },
  },
  'nl-NL': {
    partDescriptions: {
      constraints: 'Definieert de harde grenzen en waarborgen die de runner moet volgen.',
      skill_definition:
        'Beschrijft het skill-contract, de verantwoordelijkheden en de verwachte mogelijkheden.',
      prompt_template:
        'Vormt de herbruikbare instructies en antwoordstructuur die naar het model worden gestuurd.',
      context_assembly:
        'Stuurt hoe bewijs, status en werkruimtecontext vóór elke run worden verzameld.',
      coordinator_policy:
        'Bepaalt hoe taken op hoog niveau worden opgesplitst, geordend en overgedragen.',
      orchestrator_policy: 'Regelt meerstaps-flow, retries en coördinatie tussen fasen.',
      tool_exposure:
        'Kiest welke tools beschikbaar zijn voor de runner en hoe ze worden gepresenteerd.',
      verification_policy:
        'Definieert hoe outputs worden gecontroleerd voordat ze worden geaccepteerd of opgeslagen.',
      runner_code: 'Implementeert de runtime-logica die de agentlus en integraties uitvoert.',
      build_recipe:
        'Specificeert hoe de runner wordt voorbereid, gebouwd en verpakt voor uitvoering.',
    },
  },
  'pl-PL': {
    partDescriptions: {
      constraints:
        'Definiuje twarde ograniczenia i zabezpieczenia, których runner musi przestrzegać.',
      skill_definition: 'Opisuje kontrakt skilla, odpowiedzialności i oczekiwane możliwości.',
      prompt_template:
        'Kształtuje wielokrotnego użytku instrukcje i strukturę odpowiedzi wysyłaną do modelu.',
      context_assembly:
        'Kontroluje, jak przed każdym uruchomieniem zbierane są dowody, stan i kontekst workspace.',
      coordinator_policy:
        'Decyduje, jak zadania wysokiego poziomu są rozbijane, porządkowane i przekazywane.',
      orchestrator_policy:
        'Steruje wieloetapowym przepływem, ponownymi próbami i koordynacją między etapami.',
      tool_exposure: 'Wybiera, jakie narzędzia są dostępne dla runnera i jak są prezentowane.',
      verification_policy: 'Definiuje, jak wyniki są sprawdzane przed akceptacją lub utrwaleniem.',
      runner_code: 'Implementuje logikę runtime, która wykonuje pętlę agenta i integracje.',
      build_recipe: 'Określa, jak runner jest przygotowywany, budowany i pakowany do uruchomienia.',
    },
  },
  'pt-BR': {
    partDescriptions: {
      constraints: 'Define os limites rígidos e as proteções que o runner deve seguir.',
      skill_definition:
        'Descreve o contrato da skill, as responsabilidades e as capacidades esperadas.',
      prompt_template:
        'Molda as instruções reutilizáveis e a estrutura de resposta enviada ao modelo.',
      context_assembly:
        'Controla como evidências, estado e contexto do workspace são reunidos antes de cada execução.',
      coordinator_policy:
        'Decide como tarefas de nível superior são decompostas, ordenadas e repassadas.',
      orchestrator_policy:
        'Governa o fluxo em várias etapas, as tentativas novamente e a coordenação entre estágios.',
      tool_exposure:
        'Escolhe quais ferramentas ficam disponíveis para o runner e como são apresentadas.',
      verification_policy:
        'Define como as saídas são verificadas antes de serem aceitas ou persistidas.',
      runner_code: 'Implementa a lógica de runtime que executa o loop do agente e as integrações.',
      build_recipe: 'Especifica como o runner é preparado, compilado e empacotado para execução.',
    },
  },
  'pt-PT': {
    partDescriptions: {
      constraints: 'Define os limites rígidos e as salvaguardas que o runner deve seguir.',
      skill_definition:
        'Descreve o contrato da skill, as responsabilidades e as capacidades esperadas.',
      prompt_template:
        'Molda as instruções reutilizáveis e a estrutura de resposta enviada ao modelo.',
      context_assembly:
        'Controla como evidências, estado e contexto do workspace são reunidos antes de cada execução.',
      coordinator_policy:
        'Decide como as tarefas de nível superior são decompostas, ordenadas e passadas adiante.',
      orchestrator_policy:
        'Governa o fluxo em várias etapas, as novas tentativas e a coordenação entre fases.',
      tool_exposure:
        'Escolhe que ferramentas ficam disponíveis para o runner e como são apresentadas.',
      verification_policy:
        'Define como as saídas são verificadas antes de serem aceites ou persistidas.',
      runner_code: 'Implementa a lógica de runtime que executa o ciclo do agente e as integrações.',
      build_recipe: 'Especifica como o runner é preparado, compilado e empacotado para execução.',
    },
  },
  'ro-RO': {
    partDescriptions: {
      constraints:
        'Definește limitele stricte și gardurile de siguranță pe care runnerul trebuie să le respecte.',
      skill_definition:
        'Descrie contractul skill-ului, responsabilitățile și capabilitățile așteptate.',
      prompt_template:
        'Modelează instrucțiunile reutilizabile și structura răspunsului trimisă modelului.',
      context_assembly:
        'Controlează cum sunt adunate dovezile, starea și contextul workspace-ului înainte de fiecare rulare.',
      coordinator_policy:
        'Decide cum sunt descompuse, ordonate și predate sarcinile de nivel înalt.',
      orchestrator_policy:
        'Guvernează controlul fluxului pe mai mulți pași, reîncercările și coordonarea între etape.',
      tool_exposure: 'Alege ce unelte sunt disponibile pentru runner și cum sunt prezentate.',
      verification_policy:
        'Definește cum sunt verificate ieșirile înainte de a fi acceptate sau persistate.',
      runner_code: 'Implementează logica de runtime care execută bucla agentului și integrările.',
      build_recipe:
        'Specifică modul în care runnerul este pregătit, construit și împachetat pentru execuție.',
    },
  },
  'ru-RU': {
    partDescriptions: {
      constraints:
        'Определяет жёсткие ограничения и защитные рамки, которым должен следовать runner.',
      skill_definition: 'Описывает контракт навыка, обязанности и ожидаемые возможности.',
      prompt_template:
        'Формирует переиспользуемые инструкции и структуру ответа, отправляемые модели.',
      context_assembly:
        'Управляет тем, как перед каждым запуском собираются доказательства, состояние и контекст рабочего пространства.',
      coordinator_policy:
        'Решает, как задачи верхнего уровня разбиваются, упорядочиваются и передаются дальше.',
      orchestrator_policy:
        'Управляет многошаговым потоком, повторами и координацией между этапами.',
      tool_exposure: 'Выбирает, какие инструменты доступны runner и как они представлены.',
      verification_policy:
        'Определяет, как проверяются результаты перед принятием или сохранением.',
      runner_code: 'Реализует runtime-логику, которая выполняет цикл агента и интеграции.',
      build_recipe:
        'Определяет, как runner подготавливается, собирается и упаковывается для запуска.',
    },
  },
  'sk-SK': {
    partDescriptions: {
      constraints: 'Definuje pevné limity a ochranné mantinely, ktoré musí runner dodržiavať.',
      skill_definition: 'Opisuje kontrakt zručnosti, zodpovednosti a očakávané schopnosti.',
      prompt_template: 'Formuje znovupoužiteľné inštrukcie a štruktúru odpovede odosielanú modelu.',
      context_assembly:
        'Riadi, ako sa pred každým spustením zhromažďujú dôkazy, stav a kontext pracovného priestoru.',
      coordinator_policy:
        'Rozhoduje, ako sa úlohy vyššej úrovne rozkladajú, zoraďujú a odovzdávajú.',
      orchestrator_policy: 'Riadi viacstupňový tok, opakované pokusy a koordináciu medzi etapami.',
      tool_exposure: 'Vyberá, ktoré nástroje sú runneru dostupné a ako sú prezentované.',
      verification_policy: 'Definuje, ako sa výstupy kontrolujú pred prijatím alebo uložením.',
      runner_code: 'Implementuje runtime logiku, ktorá vykonáva slučku agenta a integrácie.',
      build_recipe: 'Určuje, ako sa runner pripravuje, zostavuje a balí na spustenie.',
    },
  },
  'sv-SE': {
    partDescriptions: {
      constraints: 'Definierar de hårda gränser och skyddsräcken som runnern måste följa.',
      skill_definition: 'Beskriver skill-kontraktet, ansvarsområdena och de förväntade förmågorna.',
      prompt_template:
        'Formar de återanvändbara instruktionerna och svarsstrukturen som skickas till modellen.',
      context_assembly:
        'Styr hur bevis, tillstånd och arbetsytekontext samlas in före varje körning.',
      coordinator_policy: 'Avgör hur uppgifter på hög nivå bryts ned, sekvenseras och lämnas över.',
      orchestrator_policy:
        'Styr flödeskontroll i flera steg, återförsök och samordning mellan steg.',
      tool_exposure: 'Väljer vilka verktyg som är tillgängliga för runnern och hur de presenteras.',
      verification_policy: 'Definierar hur utdata kontrolleras innan de accepteras eller sparas.',
      runner_code: 'Implementerar runtime-logiken som kör agentloopen och integrationerna.',
      build_recipe: 'Anger hur runnern förbereds, byggs och paketeras för körning.',
    },
  },
  'zh-CN': {
    partDescriptions: {
      constraints: '定义执行器必须遵守的硬性限制与安全边界。',
      skill_definition: '描述技能契约、职责范围和预期能力。',
      prompt_template: '决定发给模型的可复用指令模板与响应结构。',
      context_assembly: '控制每次运行前如何汇集证据、状态和工作区上下文。',
      coordinator_policy: '决定顶层任务如何拆解、排序并交接。',
      orchestrator_policy: '负责多阶段流程控制、重试策略和跨阶段协调。',
      tool_exposure: '决定执行器可使用哪些工具，以及如何向它呈现。',
      verification_policy: '定义结果在被接受或持久化之前如何校验。',
      runner_code: '实现驱动代理循环与各类集成的运行时代码。',
      build_recipe: '规定执行器如何被准备、构建并打包后投入运行。',
    },
  },
  'zh-TW': {
    partDescriptions: {
      constraints: '定義執行器必須遵守的硬性限制與安全邊界。',
      skill_definition: '描述技能契約、職責範圍與預期能力。',
      prompt_template: '決定送往模型的可重用指令範本與回應結構。',
      context_assembly: '控制每次執行前如何彙整證據、狀態與工作區上下文。',
      coordinator_policy: '決定頂層任務如何拆解、排序並交接。',
      orchestrator_policy: '負責多階段流程控制、重試策略與跨階段協調。',
      tool_exposure: '決定執行器可使用哪些工具，以及如何向它呈現。',
      verification_policy: '定義結果在被接受或持久化之前如何驗證。',
      runner_code: '實作驅動代理循環與各類整合的執行時程式碼。',
      build_recipe: '規定執行器如何被準備、建置並打包後投入執行。',
    },
  },
}
