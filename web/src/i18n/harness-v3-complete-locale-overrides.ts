import type { LocaleMessages } from './merge'

const harnessV3CompleteLocaleOverrides: Record<string, LocaleMessages> = {
  'ca-ES': {
    harness: {
      builder: {
        eyebrow: 'Controls avançats',
        hideAdvanced: 'Amaga avançat',
        showAdvanced: 'Obre avançat',
        title: "Model d'objectes V3 avançat",
        description:
          "Fes servir el flux complet de dataset, version, spec i run quan necessitis control exacte sobre cada objecte d'avaluació.",
        collapsed:
          "El mode avançat està replegat. Obre'l quan necessitis definir manualment datasets, versions immutables, eval specs o metadades de run.",
      },
      datasets: {
        eyebrow: 'Fonts reutilitzables',
        title: 'Datasets i versions',
        description:
          'Cada dataset pot tenir diverses versions congelades perquè els spec puguin relançar-se més endavant sobre el mateix manifest.',
        empty: 'Encara no hi ha datasets. Crea’n un per començar el flux V3.',
        total: 'Datasets',
      },
      dataset: {
        activeVersion: 'Versió activa',
        caseCount: 'Casos',
        create: 'Crea dataset',
        createHint: 'Inicia una col·lecció reutilitzable de casos amb configuració de run per defecte.',
        created: 'Dataset creat',
        items: 'elements',
        manifest: 'Manifest JSON',
        profile: 'Perfil',
        publishHint:
          'Congela un manifest immutable perquè els spec sempre apuntin a una instantània concreta.',
        publishVersion: 'Publica versió',
        runKind: 'Tipus de run',
        selectDataset: 'Selecciona un dataset',
        selectVersion: 'Selecciona una versió',
        snapshot: 'Instantània del dataset',
        sourceRef: "Referència d'origen",
        sourceType: "Tipus d'origen",
        subject: 'Tema',
        targetDataset: 'Dataset',
        versionCreated: 'Versió del dataset publicada',
        versionLabel: 'Versió',
      },
      evalSpec: {
        create: 'Crea eval spec',
        createHint:
          'Vincula una instantània de dataset a una plantilla reutilitzable de scoring i runtime.',
        created: 'Eval spec creat',
        judgeModel: 'Model judge',
        passThreshold: 'Llindar de pas',
        ruleProfile: 'Perfil de regles',
        scoringMode: 'Mode de scoring',
        title: 'Eval spec',
      },
      evalSpecs: {
        eyebrow: 'Plantilles reutilitzables',
        title: 'Eval specs',
        description:
          'Els spec capturen una instantània de dataset amb el run kind, el profile i el scoring necessaris per materialitzar runs repetibles.',
        empty: 'Encara no hi ha eval specs. Publica una versió i vincula-la a un spec.',
        total: 'Eval specs',
      },
      evalRun: {
        baseline: 'Run baseline',
        cancelled: 'Eval run cancel·lat',
        created: 'Eval run iniciat',
        inspect: 'Inspecciona informe',
        launch: 'Llança eval run',
        launchHint:
          'Materialitza el spec en un run group traçat i mantén aquí l’enllaç amb l’informe.',
        report: 'Informe enllaçat',
        reportDescription:
          "Aquesta vista uneix el registre V3 de l'eval run amb el group report subjacent perquè puguis inspeccionar resultats sense sortir de la consola.",
        selectSpec: 'Selecciona un eval spec',
        spec: 'Eval spec',
        triggerKind: 'Tipus de trigger',
        triggerRef: 'Referència del trigger',
      },
      evalRuns: {
        active: 'Eval runs actius',
        empty: 'Encara no hi ha eval runs. Llança’n un des d’un spec per omplir el registre de runs.',
        eyebrow: 'Execucions traçades',
        title: 'Eval runs',
        description:
          'Les files de run mantenen visible l’entrada V3 i alhora enllacen amb el group report i la traça de runtime.',
        total: 'Eval runs',
      },
      baseline: {
        created: 'Baseline fixat',
        default: 'Per defecte',
        defaultCount: 'Baselines per defecte',
        description:
          'Fixa l’eval run seleccionat com a baseline reutilitzable i compara-hi futurs candidats des de la mateixa consola.',
        empty: 'Encara no hi ha baselines fixats per a aquest eval spec.',
        makeDefault: 'Fes-ne el baseline per defecte',
        name: 'Nom del baseline',
        pin: 'Fixa com a baseline',
        title: 'Registre de baseline',
        total: 'Baselines',
      },
      compare: {
        baseline: 'Baseline',
        baseRun: 'Run alternatiu',
        created: 'Informe de comparació generat',
        description:
          'Genera un informe de comparació persistent contra un baseline amb nom o un altre eval run del mateix spec.',
        generate: 'Genera comparació',
        hint:
          "Si hi ha un baseline seleccionat, té prioritat. Si no, Harness compara amb l'eval run triat o amb el baseline per defecte del run objectiu.",
        improvements: 'Millores',
        kind: 'Mode de comparació',
        noImprovements: 'No s’han registrat millores en aquesta comparació.',
        noRegressions: 'No s’han registrat regressions en aquesta comparació.',
        passRateDelta: 'Delta de taxa de pas',
        regressions: 'Regressions',
        scoreDelta: 'Delta de puntuació',
        title: 'Compara runs',
      },
      group: {
        retryable: 'Reintentable',
        outcomeScore: 'Puntuació de resultat',
        executionScore: "Puntuació d'execució",
        remediation: 'Remediació',
        scorecardInsights: 'Resum de calibratge i propostes',
        scorecardInsightsHint:
          'Harness mostra aquí els diagnòstics de scoring; la revisió de propostes continua fent-se a Memory / Self-evolution.',
        reviewProposals: 'Revisa a Memory',
        noScorecardInsights:
          "No s'han adjuntat diagnòstics de calibratge ni de propostes als scorecards actuals.",
        remediationMissingArtifact:
          "Adjunta al run la ruta o etiqueta d'artifact esperada, o alinea expected_artifacts amb el contracte de sortida real.",
        remediationRequiredCheckMissing:
          'Emet un marcador estable de finalització a events o results, o alinea required_checks amb el senyal observable d’èxit.',
        remediationToolSelectionError:
          "Assegura que l'eina necessària estigui disponible en aquest profile i orienta la planificació perquè la seleccioni explícitament.",
        remediationForbiddenToolUsed:
          "Endureix la política d'eines, les allowlists o les restriccions del prompt perquè l'eina prohibida no es pugui seleccionar en aquest cas.",
        remediationRunMissing:
          'Comprova la persistència del scheduler i l’enllaç del run perquè el run produït es guardi abans de començar el scoring.',
        remediationRunFailed:
          "Inspecciona l'error i els logs del linked run, corregeix la fallada d'execució i torna a provar el cas.",
        remediationRunCancelled:
          "Comprova què ha cancel·lat el run i evita que approvals o esperes d'input de l'usuari acabin l'intent massa aviat.",
        remediationRunAborted:
          "Inspecciona els motius d'abort del driver o runtime i restaura el camí d'execució abans de tornar-ho a provar.",
        remediationRunNotCompleted:
          'Redueix l’abast de la tasca o augmenta els pressupostos de runtime perquè el run arribi a completed i publiqui evidència.',
        remediationVerificationFailed:
          'Afegeix evidència o artifacts més observables per a l’èxit, o relaxa el contracte si és més estricte que el resultat previst.',
        remediationTimeout:
          'Escurça la tasca o augmenta els pressupostos relacionats amb timeout perquè els passos necessaris acabin abans del scoring.',
        remediationGeneric:
          'Inspecciona linked runs, checks i artifacts per alinear la sortida del runtime amb el contracte declarat.',
        verificationChecks: 'Checks de verificació',
        expectedArtifacts: 'Artifacts esperats',
        traceSummary: 'Resum de traça',
        expectedValue: 'Esperat',
        actualValue: 'Real',
        eventCount: "Nombre d'esdeveniments",
        artifactCount: "Nombre d'artifacts",
        observedTools: 'Eines observades',
        calibrationRef: 'Referència de calibratge',
        takeawayCandidateCount: 'Candidats de takeaway',
        proposalCount: 'Nombre de propostes',
        proposalIds: 'IDs de proposta',
        noProposalIds: 'No hi ha propostes de revisió adjuntes',
      },
      quickEval: {
        activeVersion: 'Versió activa',
        autoChip: 'Auto: versió + spec + run',
        caseManifest: 'Casos JSON',
        created: 'Quick eval iniciat',
        datasetCreateFailed: 'Quick eval no ha pogut crear un dataset',
        datasetVersionRequired: 'Publica una versió del dataset abans d’iniciar un quick eval.',
        description:
          'Mantén l’entrada al mínim: tria casos i preset, i Harness crearà automàticament la versió del dataset, l’eval spec i el run.',
        eyebrow: 'Camí per defecte',
        launch: 'Inicia quick eval',
        minimalDescription:
          'La majoria de runs només necessiten la font dels casos i un preset. La resta hauria de ser un detall d’implementació i no un formulari obligatori.',
        minimalInput: 'Només dues decisions',
        pasteCases: 'Enganxa casos',
        preset: 'Preset',
        presetChip: '2. Preset',
        regressionHint: 'Fes servir un llindar de pas més estricte per a comprovacions de regressió repetibles.',
        researchHint: 'Orienta el run kind i el profile cap a casos d’avaluació de tipus research.',
        reuseDataset: 'Reutilitza dataset',
        selectDataset: 'Selecciona un dataset',
        smokeHint: 'Preset ràpid per a comprovacions smoke i una primera validació.',
        source: 'Font dels casos',
        sourceChip: '1. Casos',
        specCreateFailed: 'Quick eval no ha pogut preparar un eval spec',
        systemWillDo: 'Harness farà',
        systemWillDoHint:
          'Crear o reutilitzar la instantània del dataset, triar un spec compatible, connectar-hi el baseline per defecte i iniciar el run.',
        title: 'Quick Eval',
        versionCreateFailed: 'Quick eval no ha pogut publicar una versió del dataset',
      },
      groups: {
        controlPlaneDescription:
          'El registre cru de groups continua sent important per a retries, scorecards, linked runs i debugging del runtime, per això es manté visible al costat dels objectes V3 de més nivell.',
      },
    },
  },
  'cs-CZ': {
    harness: {
      builder: {
        eyebrow: 'Pokročilé ovládání',
        hideAdvanced: 'Skrýt pokročilé',
        showAdvanced: 'Otevřít pokročilé',
        title: 'Pokročilý model objektů V3',
        description:
          'Použijte plný workflow dataset, version, spec a run, když potřebujete přesnou kontrolu nad každým eval objektem.',
        collapsed:
          'Pokročilý režim je sbalený. Otevřete jej, když potřebujete ručně definovat datasety, neměnné verze, eval specy nebo metadata runu.',
      },
      datasets: {
        eyebrow: 'Znovupoužitelné zdroje',
        title: 'Datasety a verze',
        description:
          'Každý dataset může mít více zmrazených verzí, takže lze spec později spustit nad stejným manifestem.',
        empty: 'Zatím žádné datasety. Vytvořte jeden a začněte tok V3.',
        total: 'Datasety',
      },
      dataset: {
        activeVersion: 'Aktivní verze',
        caseCount: 'Případy',
        create: 'Vytvořit dataset',
        createHint: 'Začněte znovupoužitelnou kolekci případů s výchozím nastavením runu.',
        created: 'Dataset vytvořen',
        items: 'položek',
        manifest: 'Manifest JSON',
        profile: 'Profil',
        publishHint:
          'Zmrazte neměnný manifest, aby spec vždy odkazoval na konkrétní snapshot.',
        publishVersion: 'Publikovat verzi',
        runKind: 'Typ runu',
        selectDataset: 'Vyberte dataset',
        selectVersion: 'Vyberte verzi',
        snapshot: 'Snapshot datasetu',
        sourceRef: 'Odkaz na zdroj',
        sourceType: 'Typ zdroje',
        subject: 'Předmět',
        targetDataset: 'Dataset',
        versionCreated: 'Verze datasetu publikována',
        versionLabel: 'Verze',
      },
      evalSpec: {
        create: 'Vytvořit eval spec',
        createHint:
          'Přivažte jeden snapshot datasetu k znovupoužitelné šabloně pro scoring a runtime.',
        created: 'Eval spec vytvořen',
        judgeModel: 'Judge model',
        passThreshold: 'Práh úspěchu',
        ruleProfile: 'Profil pravidel',
        scoringMode: 'Režim scoringu',
        title: 'Eval spec',
      },
      evalSpecs: {
        eyebrow: 'Znovupoužitelné šablony',
        title: 'Eval specy',
        description:
          'Specy zachycují snapshot datasetu spolu s run kind, profilem a scoringem potřebným pro opakovatelné runy.',
        empty: 'Zatím žádné eval specy. Publikujte verzi a přivažte ji ke specu.',
        total: 'Eval specy',
      },
      evalRun: {
        baseline: 'Baseline run',
        cancelled: 'Eval run zrušen',
        created: 'Eval run spuštěn',
        inspect: 'Prozkoumat report',
        launch: 'Spustit eval run',
        launchHint:
          'Zmaterializujte spec do sledovaného run group a ponechte zde odkaz na report.',
        report: 'Propojený report',
        reportDescription:
          'Tento náhled spojuje záznam eval runu V3 s podkladovým group reportem, takže můžete prohlížet výsledky bez opuštění konzole.',
        selectSpec: 'Vyberte eval spec',
        spec: 'Eval spec',
        triggerKind: 'Typ triggeru',
        triggerRef: 'Reference triggeru',
      },
      evalRuns: {
        active: 'Aktivní eval runy',
        empty: 'Zatím žádné eval runy. Spusťte jeden ze specu a naplňte sledovaný registr runů.',
        eyebrow: 'Sledované běhy',
        title: 'Eval runy',
        description:
          'Řádky runů udržují vstup V3 na očích a zároveň odkazují na podkladový group report a runtime stopu.',
        total: 'Eval runy',
      },
      baseline: {
        created: 'Baseline připnut',
        default: 'Výchozí',
        defaultCount: 'Výchozí baselines',
        description:
          'Připněte vybraný eval run jako znovupoužitelný baseline a porovnávejte vůči němu budoucí kandidáty ze stejné konzole.',
        empty: 'Pro tento eval spec zatím nejsou připnuté žádné baseline.',
        makeDefault: 'Nastavit jako výchozí baseline',
        name: 'Název baseline',
        pin: 'Připnout jako baseline',
        title: 'Registr baseline',
        total: 'Baselines',
      },
      compare: {
        baseline: 'Baseline',
        baseRun: 'Záložní run',
        created: 'Porovnávací report vygenerován',
        description:
          'Vygenerujte trvalý porovnávací report proti pojmenovanému baseline nebo jinému eval runu ze stejného specu.',
        generate: 'Vygenerovat porovnání',
        hint:
          'Pokud je vybraný baseline, má přednost. Jinak Harness porovná zvolený eval run nebo výchozí baseline cílového runu.',
        improvements: 'Zlepšení',
        kind: 'Režim porovnání',
        noImprovements: 'V tomto porovnání nebyla zaznamenána žádná zlepšení.',
        noRegressions: 'V tomto porovnání nebyly zaznamenány žádné regrese.',
        passRateDelta: 'Delta míry úspěchu',
        regressions: 'Regrese',
        scoreDelta: 'Delta skóre',
        title: 'Porovnat runy',
      },
      group: {
        retryable: 'Lze opakovat',
        outcomeScore: 'Skóre výsledku',
        executionScore: 'Skóre provedení',
        remediation: 'Náprava',
        scorecardInsights: 'Souhrn kalibrace a návrhů',
        scorecardInsightsHint:
          'Harness zde zobrazuje diagnostiku scoringu; revize návrhů dál probíhá v Memory / Self-evolution.',
        reviewProposals: 'Zkontrolovat v Memory',
        noScorecardInsights:
          'K aktuálním scorecardům nebyla připojena žádná diagnostika kalibrace ani návrhů.',
        remediationMissingArtifact:
          'Připojte k runu očekávanou cestu nebo štítek artifactu, nebo slaďte expected_artifacts se skutečným výstupním kontraktem.',
        remediationRequiredCheckMissing:
          'Vypisujte stabilní značku dokončení do events nebo results, nebo slaďte required_checks s pozorovatelným signálem úspěchu.',
        remediationToolSelectionError:
          'Zajistěte, aby byl požadovaný nástroj v tomto profilu dostupný, a nasměrujte plánování tak, aby jej explicitně vybralo.',
        remediationForbiddenToolUsed:
          'Zpřísněte politiku nástrojů, allowlisty nebo omezení promptu, aby v tomto případě nešlo zvolit zakázaný nástroj.',
        remediationRunMissing:
          'Zkontrolujte persistenci scheduleru a vazbu runu, aby byl vytvořený run uložen ještě před zahájením scoringu.',
        remediationRunFailed:
          'Projděte chybu a logy linked runu, opravte selhání runtime a případ znovu spusťte.',
        remediationRunCancelled:
          'Zjistěte, co run zrušilo, a zabraňte tomu, aby approvals nebo čekání na vstup uživatele ukončily pokus příliš brzy.',
        remediationRunAborted:
          'Zkontrolujte důvody abortu z driveru nebo runtime a obnovte cestu provedení před dalším pokusem.',
        remediationRunNotCompleted:
          'Zmenšete rozsah úlohy nebo navýšte rozpočty runtime, aby run dosáhl stavu completed a publikoval důkazy.',
        remediationVerificationFailed:
          'Přidejte lépe pozorovatelné důkazy nebo artifacty o úspěchu, nebo uvolněte kontrakt, pokud je přísnější než zamýšlený výsledek.',
        remediationTimeout:
          'Zkraťte úlohu nebo zvyšte timeout rozpočty, aby potřebné kroky doběhly před scoringem.',
        remediationGeneric:
          'Zkontrolujte linked runy, checks a artifacty a slaďte výstup runtime s deklarovaným kontraktem.',
        verificationChecks: 'Ověřovací kontroly',
        expectedArtifacts: 'Očekávané artifacty',
        traceSummary: 'Shrnutí stopy',
        expectedValue: 'Očekávané',
        actualValue: 'Skutečné',
        eventCount: 'Počet událostí',
        artifactCount: 'Počet artifactů',
        observedTools: 'Pozorované nástroje',
        calibrationRef: 'Reference kalibrace',
        takeawayCandidateCount: 'Kandidáti závěrů',
        proposalCount: 'Počet návrhů',
        proposalIds: 'ID návrhů',
        noProposalIds: 'Nejsou připojeny žádné návrhy k revizi',
      },
      quickEval: {
        activeVersion: 'Aktivní verze',
        autoChip: 'Auto: verze + spec + run',
        caseManifest: 'Případy JSON',
        created: 'Quick eval spuštěn',
        datasetCreateFailed: 'Quick eval nemohl vytvořit dataset',
        datasetVersionRequired: 'Před spuštěním quick evalu publikujte verzi datasetu.',
        description:
          'Udržte vstup minimální: vyberte případy, preset a Harness za vás vytvoří verzi datasetu, eval spec i run.',
        eyebrow: 'Výchozí cesta',
        launch: 'Spustit quick eval',
        minimalDescription:
          'Většina runů potřebuje jen zdroj případů a preset. Všechno ostatní by měl být detail implementace, ne povinný formulář.',
        minimalInput: 'Jen dvě rozhodnutí',
        pasteCases: 'Vložit případy',
        preset: 'Preset',
        presetChip: '2. Preset',
        regressionHint: 'Použijte přísnější práh úspěchu pro opakovatelné regresní kontroly.',
        researchHint: 'Posuňte run kind a profile směrem k případům research evaluace.',
        reuseDataset: 'Použít existující dataset',
        selectDataset: 'Vyberte dataset',
        smokeHint: 'Rychlý výchozí preset pro smoke kontroly a první validaci.',
        source: 'Zdroj případů',
        sourceChip: '1. Případy',
        specCreateFailed: 'Quick eval nemohl připravit eval spec',
        systemWillDo: 'Harness udělá',
        systemWillDoHint:
          'Vytvoří nebo znovu použije snapshot datasetu, vybere odpovídající spec, připojí výchozí baseline a spustí run.',
        title: 'Quick Eval',
        versionCreateFailed: 'Quick eval nemohl publikovat verzi datasetu',
      },
      groups: {
        controlPlaneDescription:
          'Surový registr groups je stále důležitý pro retries, scorecards, linked runy a ladění runtime, proto zůstává viditelný vedle vyšších objektů V3.',
      },
    },
  },
  'da-DK': {
    harness: {
      builder: {
        eyebrow: 'Avancerede kontroller',
        hideAdvanced: 'Skjul avanceret',
        showAdvanced: 'Åbn avanceret',
        title: 'Avanceret V3-objektmodel',
        description:
          'Brug det fulde workflow for dataset, version, spec og run, når du har brug for præcis kontrol over hvert eval-objekt.',
        collapsed:
          'Avanceret tilstand er skjult. Åbn den, når du skal forfatte datasets, uforanderlige versioner, eval specs eller run-metadata manuelt.',
      },
      datasets: {
        eyebrow: 'Genbrugelige kilder',
        title: 'Datasets og versioner',
        description:
          'Hvert dataset kan have flere frosne versioner, så specs senere kan køres igen mod det samme manifest.',
        empty: 'Ingen datasets endnu. Opret et for at starte V3-flowet.',
        total: 'Datasets',
      },
      dataset: {
        activeVersion: 'Aktiv version',
        caseCount: 'Cases',
        create: 'Opret dataset',
        createHint: 'Start en genbrugelig samling cases med standardindstillinger for run.',
        created: 'Dataset oprettet',
        items: 'elementer',
        manifest: 'Manifest JSON',
        profile: 'Profil',
        publishHint:
          'Frys et uforanderligt manifest, så specs altid peger på et konkret snapshot.',
        publishVersion: 'Udgiv version',
        runKind: 'Run-type',
        selectDataset: 'Vælg et dataset',
        selectVersion: 'Vælg en version',
        snapshot: 'Dataset-snapshot',
        sourceRef: 'Kildereference',
        sourceType: 'Kildetype',
        subject: 'Emne',
        targetDataset: 'Dataset',
        versionCreated: 'Dataset-version udgivet',
        versionLabel: 'Version',
      },
      evalSpec: {
        create: 'Opret eval spec',
        createHint:
          'Bind ét dataset-snapshot til en genbrugelig skabelon for scoring og runtime.',
        created: 'Eval spec oprettet',
        judgeModel: 'Judge-model',
        passThreshold: 'Beståelsesgrænse',
        ruleProfile: 'Regelprofil',
        scoringMode: 'Scoringtilstand',
        title: 'Eval spec',
      },
      evalSpecs: {
        eyebrow: 'Genbrugelige skabeloner',
        title: 'Eval specs',
        description:
          'Specs samler et dataset-snapshot med run-kind, profile og scoringopsætning til gentagelige runs.',
        empty: 'Ingen eval specs endnu. Udgiv en version og bind den til et spec.',
        total: 'Eval specs',
      },
      evalRun: {
        baseline: 'Baseline-run',
        cancelled: 'Eval run annulleret',
        created: 'Eval run startet',
        inspect: 'Inspicér rapport',
        launch: 'Start eval run',
        launchHint:
          'Materialisér spec’et til en sporet run group og behold rapportkoblingen her.',
        report: 'Tilknyttet rapport',
        reportDescription:
          'Denne forhåndsvisning samler V3-eval run-posten med den underliggende group-rapport, så du kan inspicere resultater uden at forlade konsollen.',
        selectSpec: 'Vælg et eval spec',
        spec: 'Eval spec',
        triggerKind: 'Triggertype',
        triggerRef: 'Triggerreference',
      },
      evalRuns: {
        active: 'Aktive eval runs',
        empty: 'Ingen eval runs endnu. Start et fra et spec for at udfylde den sporede run-oversigt.',
        eyebrow: 'Sporede eksekveringer',
        title: 'Eval runs',
        description:
          'Run-rækker holder V3-indgangen synlig og linker stadig tilbage til den underliggende group-rapport og runtime-spor.',
        total: 'Eval runs',
      },
      baseline: {
        created: 'Baseline fastgjort',
        default: 'Standard',
        defaultCount: 'Standard-baselines',
        description:
          'Fastgør det valgte eval run som en genbrugelig baseline, og sammenlign fremtidige kandidater mod den fra samme konsol.',
        empty: 'Ingen baselines er fastgjort for dette eval spec endnu.',
        makeDefault: 'Gør dette til standard-baseline',
        name: 'Baseline-navn',
        pin: 'Fastgør som baseline',
        title: 'Baseline-register',
        total: 'Baselines',
      },
      compare: {
        baseline: 'Baseline',
        baseRun: 'Fallback-run',
        created: 'Sammenligningsrapport genereret',
        description:
          'Generér en vedvarende sammenligningsrapport mod en navngiven baseline eller et andet eval run fra samme spec.',
        generate: 'Generér sammenligning',
        hint:
          'Hvis en baseline er valgt, bruges den først. Ellers sammenligner Harness mod det valgte eval run eller mål-run’ets standard-baseline.',
        improvements: 'Forbedringer',
        kind: 'Sammenligningstilstand',
        noImprovements: 'Ingen forbedringer registreret i denne sammenligning.',
        noRegressions: 'Ingen regressioner registreret i denne sammenligning.',
        passRateDelta: 'Ændring i beståelsesrate',
        regressions: 'Regressioner',
        scoreDelta: 'Ændring i score',
        title: 'Sammenlign runs',
      },
      group: {
        retryable: 'Kan prøves igen',
        outcomeScore: 'Resultatscore',
        executionScore: 'Eksekveringsscore',
        remediation: 'Afhjælpning',
        scorecardInsights: 'Kalibrerings- og forslagsoversigt',
        scorecardInsightsHint:
          'Harness viser scoringsdiagnostik her; gennemgang af forslag sker stadig i Memory / Self-evolution.',
        reviewProposals: 'Gennemgå i Memory',
        noScorecardInsights:
          'Der er ikke knyttet kalibrerings- eller forslagsdiagnostik til de aktuelle scorecards.',
        remediationMissingArtifact:
          'Vedhæft den forventede artifact-sti eller etiket til run’et, eller justér expected_artifacts, så det matcher den faktiske outputkontrakt.',
        remediationRequiredCheckMissing:
          'Udsend en stabil completion-markør i events eller results, eller justér required_checks til det observerbare succes-signal.',
        remediationToolSelectionError:
          'Sørg for, at det nødvendige værktøj er tilgængeligt i denne profil, og styr planlægningen, så det vælges eksplicit.',
        remediationForbiddenToolUsed:
          'Stram værktøjspolitik, allowlists eller promptbegrænsninger, så det forbudte værktøj ikke kan vælges til denne case.',
        remediationRunMissing:
          'Kontrollér scheduler-persistens og run-linking, så det producerede run gemmes, før scoring starter.',
        remediationRunFailed:
          'Inspicér fejl og logs for det linked run, ret runtime-fejlen, og prøv derefter casen igen.',
        remediationRunCancelled:
          'Undersøg hvad der annullerede run’et, og undgå at approvals eller brugerinput-ventetid afslutter forsøget for tidligt.',
        remediationRunAborted:
          'Inspicér abortårsager fra driver eller runtime, og gendan eksekveringsstien før nyt forsøg.',
        remediationRunNotCompleted:
          'Reducer opgavens omfang eller øg runtime-budgetter, så run’et kan nå completed og publicere evidens.',
        remediationVerificationFailed:
          'Tilføj stærkere observerbar evidens eller artifacts for succes, eller lemp kontrakten, hvis den er strengere end det tilsigtede resultat.',
        remediationTimeout:
          'Forkort opgaven eller øg timeout-budgetter, så nødvendige trin kan afsluttes før scoring.',
        remediationGeneric:
          'Inspicér linked runs, checks og artifacts for at afstemme runtime-output med den erklærede kontrakt.',
        verificationChecks: 'Verifikationskontroller',
        expectedArtifacts: 'Forventede artifacts',
        traceSummary: 'Sporingsresumé',
        expectedValue: 'Forventet',
        actualValue: 'Faktisk',
        eventCount: 'Antal events',
        artifactCount: 'Antal artifacts',
        observedTools: 'Observerede værktøjer',
        calibrationRef: 'Kalibreringsreference',
        takeawayCandidateCount: 'Takeaway-kandidater',
        proposalCount: 'Antal forslag',
        proposalIds: 'Forslags-id’er',
        noProposalIds: 'Ingen review-forslag vedhæftet',
      },
      quickEval: {
        activeVersion: 'Aktiv version',
        autoChip: 'Auto: version + spec + run',
        caseManifest: 'Cases JSON',
        created: 'Quick eval startet',
        datasetCreateFailed: 'Quick eval kunne ikke oprette et dataset',
        datasetVersionRequired: 'Udgiv en dataset-version før du starter en quick eval.',
        description:
          'Hold input minimalt: vælg cases og preset, så opretter Harness automatisk dataset-versionen, eval spec’et og run’et.',
        eyebrow: 'Standardsti',
        launch: 'Start quick eval',
        minimalDescription:
          'De fleste runs kræver kun case-kilden og et preset. Alt andet bør være en implementeringsdetalje og ikke en obligatorisk formular.',
        minimalInput: 'Kun to valg',
        pasteCases: 'Indsæt cases',
        preset: 'Preset',
        presetChip: '2. Preset',
        regressionHint: 'Brug en strengere beståelsesgrænse til gentagelige regressionskontroller.',
        researchHint: 'Skub run-kind og profile i retning af research-lignende eval cases.',
        reuseDataset: 'Genbrug dataset',
        selectDataset: 'Vælg et dataset',
        smokeHint: 'Hurtigt standardpreset til smoke-kontroller og første validering.',
        source: 'Case-kilde',
        sourceChip: '1. Cases',
        specCreateFailed: 'Quick eval kunne ikke forberede et eval spec',
        systemWillDo: 'Harness vil gøre',
        systemWillDoHint:
          'Oprette eller genbruge dataset-snapshot, vælge et matchende spec, tilknytte standard-baseline og starte run’et.',
        title: 'Quick Eval',
        versionCreateFailed: 'Quick eval kunne ikke udgive en dataset-version',
      },
      groups: {
        controlPlaneDescription:
          'Det rå group-register er stadig vigtigt for retries, scorecards, linked runs og runtime-fejlfinding, så det forbliver synligt ved siden af V3-objekterne på højere niveau.',
      },
    },
  },
  'de-DE': {
    harness: {
      builder: {
        eyebrow: 'Erweiterte Steuerung',
        hideAdvanced: 'Erweitert ausblenden',
        showAdvanced: 'Erweitert öffnen',
        title: 'Erweitertes V3-Objektmodell',
        description:
          'Nutzen Sie den vollständigen Workflow aus Dataset, Version, Spec und Run, wenn Sie jedes Eval-Objekt exakt steuern müssen.',
        collapsed:
          'Der erweiterte Modus ist eingeklappt. Öffnen Sie ihn, wenn Sie Datasets, unveränderliche Versionen, Eval Specs oder Run-Metadaten manuell definieren müssen.',
      },
      datasets: {
        eyebrow: 'Wiederverwendbare Quellen',
        title: 'Datasets und Versionen',
        description:
          'Jedes Dataset kann mehrere eingefrorene Versionen tragen, sodass Specs später erneut gegen dasselbe Manifest laufen können.',
        empty: 'Noch keine Datasets. Erstellen Sie eines, um den V3-Eval-Flow zu starten.',
        total: 'Datasets',
      },
      dataset: {
        activeVersion: 'Aktive Version',
        caseCount: 'Fälle',
        create: 'Dataset erstellen',
        createHint: 'Starten Sie eine wiederverwendbare Fallsammlung mit Standard-Run-Einstellungen.',
        created: 'Dataset erstellt',
        items: 'Elemente',
        manifest: 'Manifest JSON',
        profile: 'Profil',
        publishHint:
          'Frieren Sie ein unveränderliches Manifest ein, damit Specs immer an einen konkreten Snapshot gebunden sind.',
        publishVersion: 'Version veröffentlichen',
        runKind: 'Run-Typ',
        selectDataset: 'Dataset auswählen',
        selectVersion: 'Version auswählen',
        snapshot: 'Dataset-Snapshot',
        sourceRef: 'Quellreferenz',
        sourceType: 'Quelltyp',
        subject: 'Betreff',
        targetDataset: 'Dataset',
        versionCreated: 'Dataset-Version veröffentlicht',
        versionLabel: 'Version',
      },
      evalSpec: {
        create: 'Eval Spec erstellen',
        createHint:
          'Binden Sie einen Dataset-Snapshot an eine wiederverwendbare Vorlage für Scoring und Runtime.',
        created: 'Eval Spec erstellt',
        judgeModel: 'Judge-Modell',
        passThreshold: 'Bestehensschwelle',
        ruleProfile: 'Regelprofil',
        scoringMode: 'Scoring-Modus',
        title: 'Eval Spec',
      },
      evalSpecs: {
        eyebrow: 'Wiederverwendbare Vorlagen',
        title: 'Eval Specs',
        description:
          'Specs erfassen einen Dataset-Snapshot zusammen mit Run-Typ, Profil und Scoring-Konfiguration für wiederholbare Runs.',
        empty: 'Noch keine Eval Specs. Veröffentlichen Sie eine Version und binden Sie sie an ein Spec.',
        total: 'Eval Specs',
      },
      evalRun: {
        baseline: 'Baseline-Run',
        cancelled: 'Eval Run abgebrochen',
        created: 'Eval Run gestartet',
        inspect: 'Report prüfen',
        launch: 'Eval Run starten',
        launchHint:
          'Materialisieren Sie das Spec in eine nachverfolgbare Run Group und behalten Sie den Report hier verlinkt.',
        report: 'Verknüpfter Report',
        reportDescription:
          'Diese Vorschau verbindet den V3-Eval-Run-Eintrag mit dem zugrunde liegenden Group-Report, sodass Sie Ergebnisse prüfen können, ohne die Konsole zu verlassen.',
        selectSpec: 'Eval Spec auswählen',
        spec: 'Eval Spec',
        triggerKind: 'Trigger-Typ',
        triggerRef: 'Trigger-Referenz',
      },
      evalRuns: {
        active: 'Aktive Eval Runs',
        empty:
          'Noch keine Eval Runs. Starten Sie einen aus einem Spec, um das nachverfolgte Run-Register zu füllen.',
        eyebrow: 'Nachverfolgte Ausführungen',
        title: 'Eval Runs',
        description:
          'Die Run-Zeilen halten den V3-Einstieg sichtbar und verweisen weiterhin auf den zugrunde liegenden Group-Report und die Runtime-Spur.',
        total: 'Eval Runs',
      },
      baseline: {
        created: 'Baseline fixiert',
        default: 'Standard',
        defaultCount: 'Standard-Baselines',
        description:
          'Fixieren Sie den ausgewählten Eval Run als wiederverwendbare Baseline und vergleichen Sie zukünftige Kandidaten in derselben Konsole damit.',
        empty: 'Für dieses Eval Spec wurden noch keine Baselines fixiert.',
        makeDefault: 'Als Standard-Baseline festlegen',
        name: 'Baseline-Name',
        pin: 'Als Baseline fixieren',
        title: 'Baseline-Register',
        total: 'Baselines',
      },
      compare: {
        baseline: 'Baseline',
        baseRun: 'Fallback-Run',
        created: 'Vergleichsreport generiert',
        description:
          'Generieren Sie einen persistierten Vergleichsreport gegen eine benannte Baseline oder einen anderen Eval Run aus demselben Spec.',
        generate: 'Vergleich generieren',
        hint:
          'Wenn eine Baseline ausgewählt ist, hat sie Vorrang. Andernfalls vergleicht Harness mit dem gewählten Eval Run oder der Standard-Baseline des Ziel-Runs.',
        improvements: 'Verbesserungen',
        kind: 'Vergleichsmodus',
        noImprovements: 'Für diesen Vergleich wurden keine Verbesserungen erfasst.',
        noRegressions: 'Für diesen Vergleich wurden keine Regressionen erfasst.',
        passRateDelta: 'Delta der Erfolgsquote',
        regressions: 'Regressionen',
        scoreDelta: 'Score-Delta',
        title: 'Runs vergleichen',
      },
      group: {
        retryable: 'Erneut versuchbar',
        outcomeScore: 'Ergebnis-Score',
        executionScore: 'Ausführungs-Score',
        remediation: 'Abhilfe',
        scorecardInsights: 'Zusammenfassung von Kalibrierung und Vorschlägen',
        scorecardInsightsHint:
          'Harness zeigt hier Scoring-Diagnosen an; die Vorschlagsprüfung erfolgt weiterhin in Memory / Self-evolution.',
        reviewProposals: 'In Memory prüfen',
        noScorecardInsights:
          'An die aktuellen Scorecards wurden keine Kalibrierungs- oder Vorschlagsdiagnosen angehängt.',
        remediationMissingArtifact:
          'Hängen Sie dem Run den erwarteten Artifact-Pfad oder das erwartete Label an oder gleichen Sie expected_artifacts an den tatsächlichen Output-Vertrag an.',
        remediationRequiredCheckMissing:
          'Geben Sie ein stabiles Abschlussmerkmal in Events oder Results aus oder richten Sie required_checks am beobachtbaren Erfolgssignal aus.',
        remediationToolSelectionError:
          'Stellen Sie sicher, dass das benötigte Tool in diesem Profil verfügbar ist, und steuern Sie die Planung so, dass es explizit ausgewählt wird.',
        remediationForbiddenToolUsed:
          'Verschärfen Sie Tool-Policy, Allowlists oder Prompt-Einschränkungen, damit das verbotene Tool in diesem Fall nicht gewählt werden kann.',
        remediationRunMissing:
          'Prüfen Sie Scheduler-Persistenz und Run-Verknüpfung, damit der erzeugte Run gespeichert wird, bevor das Scoring beginnt.',
        remediationRunFailed:
          'Prüfen Sie Fehler und Logs des verknüpften Runs, beheben Sie den Laufzeitfehler und versuchen Sie den Fall erneut.',
        remediationRunCancelled:
          'Prüfen Sie, was den Run abgebrochen hat, und verhindern Sie, dass Freigaben oder Benutzereingaben den Versuch zu früh beenden.',
        remediationRunAborted:
          'Prüfen Sie Abbruchgründe aus Driver oder Runtime und stellen Sie den Ausführungspfad vor dem nächsten Versuch wieder her.',
        remediationRunNotCompleted:
          'Reduzieren Sie den Aufgabenumfang oder erhöhen Sie Runtime-Budgets, damit der Run completed erreicht und Belege veröffentlichen kann.',
        remediationVerificationFailed:
          'Fügen Sie besser beobachtbare Belege oder Artifacts für den Erfolg hinzu oder lockern Sie den Vertrag, wenn er strenger als das gewünschte Ergebnis ist.',
        remediationTimeout:
          'Verkürzen Sie die Aufgabe oder erhöhen Sie Timeout-Budgets, damit notwendige Schritte vor dem Scoring abgeschlossen werden.',
        remediationGeneric:
          'Prüfen Sie verknüpfte Runs, Checks und Artifacts, um die Runtime-Ausgabe mit dem deklarierten Vertrag in Einklang zu bringen.',
        verificationChecks: 'Verifikationsprüfungen',
        expectedArtifacts: 'Erwartete Artifacts',
        traceSummary: 'Spur-Zusammenfassung',
        expectedValue: 'Erwartet',
        actualValue: 'Tatsächlich',
        eventCount: 'Anzahl Events',
        artifactCount: 'Anzahl Artifacts',
        observedTools: 'Beobachtete Tools',
        calibrationRef: 'Kalibrierungsreferenz',
        takeawayCandidateCount: 'Anzahl Takeaway-Kandidaten',
        proposalCount: 'Anzahl Vorschläge',
        proposalIds: 'Vorschlags-IDs',
        noProposalIds: 'Keine Review-Vorschläge angehängt',
      },
      quickEval: {
        activeVersion: 'Aktive Version',
        autoChip: 'Auto: Version + Spec + Run',
        caseManifest: 'Fälle JSON',
        created: 'Quick Eval gestartet',
        datasetCreateFailed: 'Quick Eval konnte kein Dataset erstellen',
        datasetVersionRequired: 'Veröffentlichen Sie vor dem Start eines Quick Eval eine Dataset-Version.',
        description:
          'Halten Sie die Eingabe minimal: Wählen Sie Fälle und ein Preset, und Harness erstellt automatisch Dataset-Version, Eval Spec und Run.',
        eyebrow: 'Standardpfad',
        launch: 'Quick Eval starten',
        minimalDescription:
          'Die meisten Runs brauchen nur die Fallquelle und ein Preset. Alles andere sollte Implementierungsdetail statt Pflichtformular sein.',
        minimalInput: 'Nur zwei Entscheidungen',
        pasteCases: 'Fälle einfügen',
        preset: 'Preset',
        presetChip: '2. Preset',
        regressionHint:
          'Verwenden Sie eine strengere Bestehensschwelle für wiederholbare Regressionsprüfungen.',
        researchHint:
          'Verschieben Sie Run-Typ und Profil in Richtung forschungsorientierter Eval-Fälle.',
        reuseDataset: 'Dataset wiederverwenden',
        selectDataset: 'Dataset auswählen',
        smokeHint: 'Schnelles Standard-Preset für Smoke-Checks und eine erste Validierung.',
        source: 'Fallquelle',
        sourceChip: '1. Fälle',
        specCreateFailed: 'Quick Eval konnte kein Eval Spec vorbereiten',
        systemWillDo: 'Harness erledigt',
        systemWillDoHint:
          'Erstellt oder wiederverwendet den Dataset-Snapshot, wählt ein passendes Spec, hängt die Standard-Baseline an und startet den Run.',
        title: 'Quick Eval',
        versionCreateFailed: 'Quick Eval konnte keine Dataset-Version veröffentlichen',
      },
      groups: {
        controlPlaneDescription:
          'Das rohe Group-Register bleibt wichtig für Retries, Scorecards, verknüpfte Runs und Runtime-Debugging, daher bleibt es neben den höherstufigen V3-Objekten sichtbar.',
      },
    },
  },
  'el-GR': {
    harness: {
      builder: {
        eyebrow: 'Προχωρημένοι έλεγχοι',
        hideAdvanced: 'Απόκρυψη προχωρημένων',
        showAdvanced: 'Άνοιγμα προχωρημένων',
        title: 'Προχωρημένο μοντέλο αντικειμένων V3',
        description:
          'Χρησιμοποίησε το πλήρες workflow dataset, version, spec και run όταν χρειάζεσαι ακριβή έλεγχο για κάθε eval αντικείμενο.',
        collapsed:
          'Η προχωρημένη λειτουργία είναι κλειστή. Άνοιξέ την όταν χρειάζεται να ορίσεις χειροκίνητα datasets, αμετάβλητες versions, eval specs ή metadata run.',
      },
      datasets: {
        eyebrow: 'Επαναχρησιμοποιήσιμες πηγές',
        title: 'Datasets και versions',
        description:
          'Κάθε dataset μπορεί να έχει πολλές παγωμένες versions, ώστε τα specs να εκτελούνται ξανά αργότερα πάνω στο ίδιο manifest.',
        empty: 'Δεν υπάρχουν ακόμα datasets. Δημιούργησε ένα για να ξεκινήσεις τη ροή V3.',
        total: 'Datasets',
      },
      dataset: {
        activeVersion: 'Ενεργή version',
        caseCount: 'Περιπτώσεις',
        create: 'Δημιουργία dataset',
        createHint:
          'Ξεκίνα μια επαναχρησιμοποιήσιμη συλλογή περιπτώσεων με προεπιλεγμένες ρυθμίσεις run.',
        created: 'Το dataset δημιουργήθηκε',
        items: 'στοιχεία',
        manifest: 'Manifest JSON',
        profile: 'Προφίλ',
        publishHint:
          'Πάγωσε ένα αμετάβλητο manifest ώστε τα specs να δένουν πάντα σε συγκεκριμένο snapshot.',
        publishVersion: 'Δημοσίευση version',
        runKind: 'Τύπος run',
        selectDataset: 'Επίλεξε dataset',
        selectVersion: 'Επίλεξε version',
        snapshot: 'Snapshot dataset',
        sourceRef: 'Αναφορά πηγής',
        sourceType: 'Τύπος πηγής',
        subject: 'Θέμα',
        targetDataset: 'Dataset',
        versionCreated: 'Η version του dataset δημοσιεύτηκε',
        versionLabel: 'Version',
      },
      evalSpec: {
        create: 'Δημιουργία eval spec',
        createHint:
          'Σύνδεσε ένα snapshot dataset με ένα επαναχρησιμοποιήσιμο πρότυπο scoring και runtime.',
        created: 'Το eval spec δημιουργήθηκε',
        judgeModel: 'Judge model',
        passThreshold: 'Όριο επιτυχίας',
        ruleProfile: 'Προφίλ κανόνων',
        scoringMode: 'Λειτουργία scoring',
        title: 'Eval spec',
      },
      evalSpecs: {
        eyebrow: 'Επαναχρησιμοποιήσιμα πρότυπα',
        title: 'Eval specs',
        description:
          'Τα specs καταγράφουν ένα snapshot dataset μαζί με run kind, profile και scoring setup για επαναλήψιμα runs.',
        empty: 'Δεν υπάρχουν ακόμα eval specs. Δημοσίευσε μια version και σύνδεσέ την με ένα spec.',
        total: 'Eval specs',
      },
      evalRun: {
        baseline: 'Baseline run',
        cancelled: 'Το eval run ακυρώθηκε',
        created: 'Το eval run ξεκίνησε',
        inspect: 'Επιθεώρηση αναφοράς',
        launch: 'Εκκίνηση eval run',
        launchHint:
          'Υλοποίησε το spec ως tracked run group και κράτησε την αναφορά συνδεδεμένη εδώ.',
        report: 'Συνδεδεμένη αναφορά',
        reportDescription:
          'Αυτή η προεπισκόπηση ενώνει την εγγραφή eval run V3 με το υποκείμενο group report ώστε να ελέγχεις τα αποτελέσματα χωρίς να φύγεις από την κονσόλα.',
        selectSpec: 'Επίλεξε eval spec',
        spec: 'Eval spec',
        triggerKind: 'Τύπος trigger',
        triggerRef: 'Αναφορά trigger',
      },
      evalRuns: {
        active: 'Ενεργά eval runs',
        empty: 'Δεν υπάρχουν ακόμα eval runs. Εκκίνησε ένα από ένα spec για να γεμίσει το tracked ledger.',
        eyebrow: 'Tracked εκτελέσεις',
        title: 'Eval runs',
        description:
          'Οι γραμμές run κρατούν ορατό το σημείο εισόδου V3 ενώ παραμένουν συνδεδεμένες με το υποκείμενο group report και το runtime trail.',
        total: 'Eval runs',
      },
      baseline: {
        created: 'Το baseline καρφιτσώθηκε',
        default: 'Προεπιλογή',
        defaultCount: 'Προεπιλεγμένα baselines',
        description:
          'Καρφίτσωσε το επιλεγμένο eval run ως επαναχρησιμοποιήσιμο baseline και σύγκρινε μελλοντικούς υποψηφίους από την ίδια κονσόλα.',
        empty: 'Δεν υπάρχουν ακόμα baselines για αυτό το eval spec.',
        makeDefault: 'Ορισμός ως προεπιλεγμένου baseline',
        name: 'Όνομα baseline',
        pin: 'Καρφίτσωμα ως baseline',
        title: 'Μητρώο baseline',
        total: 'Baselines',
      },
      compare: {
        baseline: 'Baseline',
        baseRun: 'Εναλλακτικό run',
        created: 'Η αναφορά σύγκρισης δημιουργήθηκε',
        description:
          'Δημιούργησε μια μόνιμη αναφορά σύγκρισης απέναντι σε ονομασμένο baseline ή σε άλλο eval run του ίδιου spec.',
        generate: 'Δημιουργία σύγκρισης',
        hint:
          'Αν έχει επιλεγεί baseline, προηγείται. Αλλιώς το Harness συγκρίνει με το επιλεγμένο eval run ή με το προεπιλεγμένο baseline του target run.',
        improvements: 'Βελτιώσεις',
        kind: 'Λειτουργία σύγκρισης',
        noImprovements: 'Δεν καταγράφηκαν βελτιώσεις σε αυτή τη σύγκριση.',
        noRegressions: 'Δεν καταγράφηκαν regressions σε αυτή τη σύγκριση.',
        passRateDelta: 'Μεταβολή ποσοστού επιτυχίας',
        regressions: 'Regressions',
        scoreDelta: 'Μεταβολή score',
        title: 'Σύγκριση runs',
      },
      group: {
        retryable: 'Επαναλήψιμο',
        outcomeScore: 'Score αποτελέσματος',
        executionScore: 'Score εκτέλεσης',
        remediation: 'Αποκατάσταση',
        scorecardInsights: 'Σύνοψη βαθμονόμησης και προτάσεων',
        scorecardInsightsHint:
          'Το Harness εμφανίζει εδώ διαγνωστικά scoring· η αξιολόγηση προτάσεων συνεχίζει στο Memory / Self-evolution.',
        reviewProposals: 'Έλεγχος στο Memory',
        noScorecardInsights:
          'Δεν συνδέθηκαν διαγνωστικά βαθμονόμησης ή προτάσεων με τα τρέχοντα scorecards.',
        remediationMissingArtifact:
          'Σύνδεσε στο run την αναμενόμενη διαδρομή ή ετικέτα artifact ή ευθυγράμμισε το expected_artifacts με το πραγματικό output contract.',
        remediationRequiredCheckMissing:
          'Εξέπεμψε ένα σταθερό completion marker σε events ή results ή ευθυγράμμισε το required_checks με το παρατηρήσιμο σήμα επιτυχίας.',
        remediationToolSelectionError:
          'Βεβαιώσου ότι το απαιτούμενο tool είναι διαθέσιμο σε αυτό το profile και οδήγησε τον σχεδιασμό ώστε να το επιλέγει ρητά.',
        remediationForbiddenToolUsed:
          'Σκλήρυνε την πολιτική tools, τα allowlists ή τους περιορισμούς prompt ώστε να μην μπορεί να επιλεγεί το απαγορευμένο tool για αυτή την περίπτωση.',
        remediationRunMissing:
          'Έλεγξε την persistence του scheduler και τη σύνδεση run ώστε το παραγόμενο run να αποθηκεύεται πριν αρχίσει το scoring.',
        remediationRunFailed:
          'Επιθεώρησε το σφάλμα και τα logs του linked run, διόρθωσε την αποτυχία runtime και δοκίμασε ξανά την περίπτωση.',
        remediationRunCancelled:
          'Έλεγξε τι ακύρωσε το run και απόφυγε approvals ή αναμονές εισόδου χρήστη που τερματίζουν την προσπάθεια πολύ νωρίς.',
        remediationRunAborted:
          'Επιθεώρησε τους λόγους abort από driver ή runtime και επανέφερε τη διαδρομή εκτέλεσης πριν από νέα προσπάθεια.',
        remediationRunNotCompleted:
          'Μείωσε το εύρος της εργασίας ή αύξησε τα budgets runtime ώστε το run να φτάσει σε completed και να δημοσιεύσει τεκμήρια.',
        remediationVerificationFailed:
          'Πρόσθεσε ισχυρότερα παρατηρήσιμα τεκμήρια ή artifacts επιτυχίας ή χαλάρωσε το contract αν είναι αυστηρότερο από το επιδιωκόμενο αποτέλεσμα.',
        remediationTimeout:
          'Συντόμευσε την εργασία ή αύξησε τα budgets timeout ώστε τα απαραίτητα βήματα να ολοκληρώνονται πριν το scoring.',
        remediationGeneric:
          'Επιθεώρησε linked runs, checks και artifacts ώστε το output του runtime να ευθυγραμμιστεί με το δηλωμένο contract.',
        verificationChecks: 'Έλεγχοι επαλήθευσης',
        expectedArtifacts: 'Αναμενόμενα artifacts',
        traceSummary: 'Σύνοψη ίχνους',
        expectedValue: 'Αναμενόμενο',
        actualValue: 'Πραγματικό',
        eventCount: 'Πλήθος events',
        artifactCount: 'Πλήθος artifacts',
        observedTools: 'Παρατηρημένα tools',
        calibrationRef: 'Αναφορά βαθμονόμησης',
        takeawayCandidateCount: 'Υποψήφια takeaways',
        proposalCount: 'Πλήθος προτάσεων',
        proposalIds: 'IDs προτάσεων',
        noProposalIds: 'Δεν έχουν συνδεθεί προτάσεις review',
      },
      quickEval: {
        activeVersion: 'Ενεργή version',
        autoChip: 'Auto: version + spec + run',
        caseManifest: 'Περιπτώσεις JSON',
        created: 'Το quick eval ξεκίνησε',
        datasetCreateFailed: 'Το quick eval δεν μπόρεσε να δημιουργήσει dataset',
        datasetVersionRequired: 'Δημοσίευσε version dataset πριν ξεκινήσεις quick eval.',
        description:
          'Κράτησε το input ελάχιστο: επίλεξε περιπτώσεις και preset, και το Harness θα δημιουργήσει αυτόματα version dataset, eval spec και run.',
        eyebrow: 'Προεπιλεγμένη διαδρομή',
        launch: 'Εκκίνηση quick eval',
        minimalDescription:
          'Τα περισσότερα runs χρειάζονται μόνο την πηγή περιπτώσεων και ένα preset. Όλα τα άλλα πρέπει να είναι λεπτομέρεια υλοποίησης, όχι υποχρεωτική φόρμα.',
        minimalInput: 'Μόνο δύο αποφάσεις',
        pasteCases: 'Επικόλληση περιπτώσεων',
        preset: 'Preset',
        presetChip: '2. Preset',
        regressionHint:
          'Χρησιμοποίησε αυστηρότερο όριο επιτυχίας για επαναλήψιμους ελέγχους regression.',
        researchHint:
          'Μετέφερε το run kind και το profile προς περιπτώσεις eval τύπου research.',
        reuseDataset: 'Επαναχρησιμοποίηση dataset',
        selectDataset: 'Επίλεξε dataset',
        smokeHint: 'Γρήγορο προεπιλεγμένο preset για smoke checks και πρώτη επικύρωση.',
        source: 'Πηγή περιπτώσεων',
        sourceChip: '1. Περιπτώσεις',
        specCreateFailed: 'Το quick eval δεν μπόρεσε να ετοιμάσει eval spec',
        systemWillDo: 'Το Harness θα κάνει',
        systemWillDoHint:
          'Θα δημιουργήσει ή θα επαναχρησιμοποιήσει snapshot dataset, θα επιλέξει συμβατό spec, θα συνδέσει το προεπιλεγμένο baseline και θα ξεκινήσει το run.',
        title: 'Quick Eval',
        versionCreateFailed: 'Το quick eval δεν μπόρεσε να δημοσιεύσει version dataset',
      },
      groups: {
        controlPlaneDescription:
          'Το ακατέργαστο ledger των groups παραμένει σημαντικό για retries, scorecards, linked runs και runtime debugging, γι’ αυτό παραμένει ορατό δίπλα στα αντικείμενα V3 υψηλότερου επιπέδου.',
      },
    },
  },
  'es-ES': {
    harness: {
      builder: {
        eyebrow: 'Controles avanzados',
        hideAdvanced: 'Ocultar avanzado',
        showAdvanced: 'Abrir avanzado',
        title: 'Modelo de objetos V3 avanzado',
        description:
          'Usa el flujo completo de dataset, version, spec y run cuando necesites control preciso sobre cada objeto de evaluación.',
        collapsed:
          'El modo avanzado está contraído. Ábrelo cuando necesites definir manualmente datasets, versiones inmutables, eval specs o metadatos de run.',
      },
      datasets: {
        eyebrow: 'Fuentes reutilizables',
        title: 'Datasets y versiones',
        description:
          'Cada dataset puede tener varias versiones congeladas para que los specs puedan volver a ejecutarse más adelante sobre el mismo manifest.',
        empty: 'Todavía no hay datasets. Crea uno para iniciar el flujo V3.',
        total: 'Datasets',
      },
      dataset: {
        activeVersion: 'Versión activa',
        caseCount: 'Casos',
        create: 'Crear dataset',
        createHint: 'Inicia una colección reutilizable de casos con ajustes predeterminados de run.',
        created: 'Dataset creado',
        items: 'elementos',
        manifest: 'Manifest JSON',
        profile: 'Perfil',
        publishHint:
          'Congela un manifest inmutable para que los specs siempre apunten a una instantánea concreta.',
        publishVersion: 'Publicar versión',
        runKind: 'Tipo de run',
        selectDataset: 'Selecciona un dataset',
        selectVersion: 'Selecciona una versión',
        snapshot: 'Instantánea del dataset',
        sourceRef: 'Referencia de origen',
        sourceType: 'Tipo de origen',
        subject: 'Asunto',
        targetDataset: 'Dataset',
        versionCreated: 'Versión del dataset publicada',
        versionLabel: 'Versión',
      },
      evalSpec: {
        create: 'Crear eval spec',
        createHint:
          'Vincula una instantánea de dataset a una plantilla reutilizable de scoring y runtime.',
        created: 'Eval spec creado',
        judgeModel: 'Modelo judge',
        passThreshold: 'Umbral de aprobación',
        ruleProfile: 'Perfil de reglas',
        scoringMode: 'Modo de scoring',
        title: 'Eval spec',
      },
      evalSpecs: {
        eyebrow: 'Plantillas reutilizables',
        title: 'Eval specs',
        description:
          'Los specs capturan una instantánea de dataset junto con run kind, profile y configuración de scoring para materializar runs repetibles.',
        empty: 'Todavía no hay eval specs. Publica una versión y vincúlala a un spec.',
        total: 'Eval specs',
      },
      evalRun: {
        baseline: 'Run baseline',
        cancelled: 'Eval run cancelado',
        created: 'Eval run lanzado',
        inspect: 'Inspeccionar informe',
        launch: 'Lanzar eval run',
        launchHint:
          'Materializa el spec en un run group trazable y mantén aquí enlazado el informe.',
        report: 'Informe vinculado',
        reportDescription:
          'Esta vista previa une el registro V3 del eval run con el group report subyacente para que puedas inspeccionar resultados sin salir de la consola.',
        selectSpec: 'Selecciona un eval spec',
        spec: 'Eval spec',
        triggerKind: 'Tipo de disparador',
        triggerRef: 'Referencia del disparador',
      },
      evalRuns: {
        active: 'Eval runs activos',
        empty:
          'Todavía no hay eval runs. Lanza uno desde un spec para poblar el registro de ejecuciones trazadas.',
        eyebrow: 'Ejecuciones trazadas',
        title: 'Eval runs',
        description:
          'Las filas de runs mantienen visible el punto de entrada V3 y siguen enlazando al group report y al rastro de runtime.',
        total: 'Eval runs',
      },
      baseline: {
        created: 'Baseline fijado',
        default: 'Predeterminado',
        defaultCount: 'Baselines predeterminados',
        description:
          'Fija el eval run seleccionado como baseline reutilizable y compara futuros candidatos contra él desde la misma consola.',
        empty: 'Todavía no hay baselines fijados para este eval spec.',
        makeDefault: 'Convertir en baseline predeterminado',
        name: 'Nombre del baseline',
        pin: 'Fijar como baseline',
        title: 'Registro de baselines',
        total: 'Baselines',
      },
      compare: {
        baseline: 'Baseline',
        baseRun: 'Run alternativo',
        created: 'Informe de comparación generado',
        description:
          'Genera un informe de comparación persistente frente a un baseline con nombre o a otro eval run del mismo spec.',
        generate: 'Generar comparación',
        hint:
          'Si se selecciona un baseline, tiene prioridad. Si no, Harness compara con el eval run elegido o con el baseline predeterminado del run objetivo.',
        improvements: 'Mejoras',
        kind: 'Modo de comparación',
        noImprovements: 'No se registraron mejoras en esta comparación.',
        noRegressions: 'No se registraron regresiones en esta comparación.',
        passRateDelta: 'Delta de tasa de aprobación',
        regressions: 'Regresiones',
        scoreDelta: 'Delta de puntuación',
        title: 'Comparar runs',
      },
      group: {
        retryable: 'Reintentable',
        outcomeScore: 'Puntuación de resultado',
        executionScore: 'Puntuación de ejecución',
        remediation: 'Remediación',
        scorecardInsights: 'Resumen de calibración y propuestas',
        scorecardInsightsHint:
          'Harness muestra aquí diagnósticos de scoring; la revisión de propuestas sigue ocurriendo en Memory / Self-evolution.',
        reviewProposals: 'Revisar en Memory',
        noScorecardInsights:
          'No se adjuntaron diagnósticos de calibración ni propuestas a las scorecards actuales.',
        remediationMissingArtifact:
          'Adjunta al run la ruta o etiqueta de artifact esperada, o alinea expected_artifacts con el contrato de salida real.',
        remediationRequiredCheckMissing:
          'Emite un marcador estable de finalización en events o results, o alinea required_checks con la señal observable de éxito.',
        remediationToolSelectionError:
          'Asegura que la herramienta necesaria esté disponible en este profile y orienta la planificación para que la seleccione explícitamente.',
        remediationForbiddenToolUsed:
          'Endurece la política de herramientas, las allowlists o las restricciones del prompt para que la herramienta prohibida no pueda seleccionarse en este caso.',
        remediationRunMissing:
          'Revisa la persistencia del scheduler y el enlace del run para que el run producido se guarde antes de iniciar el scoring.',
        remediationRunFailed:
          'Inspecciona el error y los logs del linked run, corrige el fallo de runtime y vuelve a intentar el caso.',
        remediationRunCancelled:
          'Comprueba qué canceló el run y evita que approvals o esperas de entrada del usuario terminen el intento demasiado pronto.',
        remediationRunAborted:
          'Inspecciona los motivos de abort del driver o runtime y restaura la ruta de ejecución antes de reintentar.',
        remediationRunNotCompleted:
          'Reduce el alcance de la tarea o aumenta los presupuestos de runtime para que el run alcance completed y publique evidencia.',
        remediationVerificationFailed:
          'Añade evidencia o artifacts de éxito más observables, o relaja el contrato si es más estricto que el resultado deseado.',
        remediationTimeout:
          'Acorta la tarea o aumenta los presupuestos relacionados con timeout para que los pasos necesarios finalicen antes del scoring.',
        remediationGeneric:
          'Inspecciona linked runs, checks y artifacts para alinear la salida del runtime con el contrato declarado.',
        verificationChecks: 'Comprobaciones de verificación',
        expectedArtifacts: 'Artifacts esperados',
        traceSummary: 'Resumen de traza',
        expectedValue: 'Esperado',
        actualValue: 'Real',
        eventCount: 'Recuento de events',
        artifactCount: 'Recuento de artifacts',
        observedTools: 'Herramientas observadas',
        calibrationRef: 'Referencia de calibración',
        takeawayCandidateCount: 'Candidatos de takeaway',
        proposalCount: 'Cantidad de propuestas',
        proposalIds: 'IDs de propuestas',
        noProposalIds: 'No hay propuestas de revisión adjuntas',
      },
      quickEval: {
        activeVersion: 'Versión activa',
        autoChip: 'Auto: versión + spec + run',
        caseManifest: 'Casos JSON',
        created: 'Quick eval lanzado',
        datasetCreateFailed: 'Quick eval no pudo crear un dataset',
        datasetVersionRequired: 'Publica una versión del dataset antes de lanzar un quick eval.',
        description:
          'Mantén la entrada al mínimo: elige casos y preset, y Harness creará automáticamente la versión del dataset, el eval spec y el run.',
        eyebrow: 'Ruta predeterminada',
        launch: 'Lanzar quick eval',
        minimalDescription:
          'La mayoría de los runs solo necesitan la fuente de casos y un preset. Todo lo demás debería ser un detalle de implementación y no un formulario obligatorio.',
        minimalInput: 'Solo dos decisiones',
        pasteCases: 'Pegar casos',
        preset: 'Preset',
        presetChip: '2. Preset',
        regressionHint:
          'Usa un umbral de aprobación más estricto para comprobaciones de regresión repetibles.',
        researchHint:
          'Orienta el run kind y el profile hacia casos de evaluación de tipo research.',
        reuseDataset: 'Reutilizar dataset',
        selectDataset: 'Selecciona un dataset',
        smokeHint: 'Preset rápido para comprobaciones smoke y una primera validación.',
        source: 'Fuente de casos',
        sourceChip: '1. Casos',
        specCreateFailed: 'Quick eval no pudo preparar un eval spec',
        systemWillDo: 'Harness hará',
        systemWillDoHint:
          'Creará o reutilizará la instantánea del dataset, elegirá un spec compatible, conectará el baseline predeterminado y lanzará el run.',
        title: 'Quick Eval',
        versionCreateFailed: 'Quick eval no pudo publicar una versión del dataset',
      },
      groups: {
        controlPlaneDescription:
          'El registro bruto de groups sigue siendo importante para retries, scorecards, linked runs y depuración de runtime, por eso permanece visible junto a los objetos V3 de nivel superior.',
      },
    },
  },
}

export default harnessV3CompleteLocaleOverrides
