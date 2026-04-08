![](../../docs/assets/bannerX.png)

<h2 align="center">ZimaOS Blue: Een local-first agentruntime voor gedurfde bouwers</h2>

<p align="center"><strong>Direct klaar voor gebruik · Open source · Universeel · Leveranciersonafhankelijk</strong></p>

<p align="center">
  <a href="../../README.md">English</a> |
  <a href="../ca_ES/README.md">Català</a> |
  <a href="../cs_CZ/README.md">Čeština</a> |
  <a href="../da_DK/README.md">Dansk</a> |
  <a href="../de_DE/README.md">Deutsch</a> |
  <a href="../el_GR/README.md">Ελληνικά</a> |
  <a href="../en_GB/README.md">English (UK)</a> |
  <a href="../es_ES/README.md">Español</a> |
  <a href="../fr_FR/README.md">Français</a> |
  <a href="../ga_IE/README.md">Gaeilge</a> |
  <a href="../hr_HR/README.md">Hrvatski</a> |
  <a href="../hu_HU/README.md">Magyar</a> |
  <a href="../it_IT/README.md">Italiano</a> |
  <a href="../ja_JP/README.md">日本語</a> |
  <a href="../ko_KR/README.md">한국어</a> |
  <a href="../ml_IN/README.md">മലയാളം</a> |
  <a href="../nb_NO/README.md">Norsk Bokmål</a> |
  <strong>Nederlands</strong> |
  <a href="../pl_PL/README.md">Polski</a> |
  <a href="../pt_BR/README.md">Português (BR)</a> |
  <a href="../pt_PT/README.md">Português (PT)</a> |
  <a href="../ro_RO/README.md">Română</a> |
  <a href="../ru_RU/README.md">Русский</a> |
  <a href="../sk_SK/README.md">Slovenčina</a> |
  <a href="../sv_SE/README.md">Svenska</a> |
  <a href="../zh_CN/README.md">简体中文</a> |
  <a href="../zh_TW/README.md">繁體中文</a>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Blue/ci.yml?branch=main&style=for-the-badge" alt="CI status"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Blue?include_prereleases&style=for-the-badge" alt="GitHub release"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT-licentie"></a>
</p>

<p align="center">
  <a href="https://discord.gg/b3AgFDxe9v"><img src="../../docs/assets/discord.png" alt="Discord" height="128" /></a>&nbsp;&nbsp;
  <a href="https://www.facebook.com/zimaboard/"><img src="../../docs/assets/facebook.png" alt="Facebook" height="128" /></a>&nbsp;&nbsp;
  <a href="https://x.com/ZimaSpace"><img src="../../docs/assets/x.png" alt="X" height="128" /></a>
</p>

## Introductie

Geïnspireerd door OpenClaw geloven wij dat de toekomst van personal computing zal worden gevormd door diverse, lokaal gerichte AI-agenten die aan de rand werken.

ZimaOS Blue is ons antwoord: een volledig open source, controleerbare, leveranciersneutrale en productieklare agentruntime en toolkit waarmee u zonder problemen privé, zelfgehoste agenten kunt verzenden.

Blue is gebouwd voor gedurfde ontwikkelaars die hun eigen agenten willen uitleven of met de hand willen maken. Het is ontworpen voor prestaties: geschreven in Go, met een geheugenoppervlak van slechts 19 MB. Het werkt op elke x86, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS - overal waar u een stopcontact aansluit.

## Demo's

### Gesprek en taakuitvoering

Een snelle demo van de gespreksstroom en taakuitvoering in Blue.

<p align="center">
  <img src="../../docs/assets/demo.gif" alt="Conversation & Task Execution demo" width="800" />
</p>

### LLM-providerintegratie

Een snelle demo van de integratie-ervaring met LLM-providers in Blue.

<p align="center">
  <img src="../../docs/assets/demo Provider.gif" alt="LLM Providers Integration demo" width="800" />
</p>

### Snel overzicht - Overzicht, kanalen en aanvullende configuratie

Een snelle demo van het productoverzicht, de kanalen en de aanvullende configuratie.

<p align="center">
  <img src="../../docs/assets/demo quickv4.gif" alt="Quick Overview demo" width="720" />
</p>

## Waarom Blue

<p align="center">
  <img src="../../docs/assets/design_principle.png" alt="Design Principle" />
</p>

### Pure Go, elk apparaat

100% Go, statisch binair getal. Cross-compileert kant-en-klaar naar 5 doelen (![linux](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) `linux/amd64`, `linux/arm64`, ![macOS](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) `darwin/amd64`, `darwin/arm64`, ![windows](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) `windows/amd64`). Geen Node-runtime, geen Python, geen containers vereist. Zet hem neer op een NAS, een <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS, een ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, een oude x86-router of een ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)Mac – hij werkt gewoon. Voeg vervolgens uw eigen UI, logica en agentvaardigheden toe: één codebase, elk platform.

### Direct uit de doos, klaar om te werken

Iedereen wil tools die eenvoudig en betrouwbaar zijn en schaalbaar zijn wanneer je ze nodig hebt. Tools die gewoon werken, zodat u zich kunt concentreren op wat u daadwerkelijk bouwt.

Dit is geen nieuwe filosofie. Het is dezelfde die <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS heeft gebouwd: eenvoudig, betrouwbaar en gebouwd om uit de weg te blijven. Blue is die filosofie, uitgebreid naar de agentenstack.

### Ontworpen voor jouw leven, gebouwd om lokaal te blijven

Van diepgaand onderzoek dat een volledig HTML-rapport oplevert, tot OCR, PDF, browserautomatisering en documentconversie: Blue verwerkt complexe, realistische workflows zonder uw gegevens naar de cloud te sturen. Voice wake, STT/TTS, Talk Mode en ondersteuning voor lokale gevolgtrekking zorgen ervoor dat dagelijkse interacties direct, privé en altijd beschikbaar zijn.

## Snelle start

### Optie 1: Desktop-app downloaden

Ontvang de native applicatie: geen afhankelijkheden, geen compilatie. Ingebouwde proefconfiguratie met onboarding binnen enkele seconden: begin direct met chatten via een externe verbinding, geen botconfiguratie vereist. Echte out-of-the-box-ervaring.

- <img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="16" /> **ZimaOS**: [Uitvoeren op ZimaOS](https://www.zimaspace.com/zimaos?utm_source=blue)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)**macOS**: [DMG downloaden](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)**Windows**: [Installatieprogramma downloaden](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### Optie 2: Script installeren

<img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="16" /> ZimaOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### Optie 3: Bouwen vanuit de bron

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue
git submodule update --init --recursive
```

<img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="16" /> ZimaOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
sh build.sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
.\build.bat
```

> **Opmerking:** Windows-builds vereisen:
> - [MinGW-w64](https://www.mingw-w64.org/) (gcc) en [CMake](https://cmake.org/) voor native C-afhankelijkheden (espeak-ng,fluister.cpp, opus, kokoro, onnx)
> - [Windows SDK](https://developer.microsoft.com/en-us/windows/downloads/windows-sdk/) voor systeembibliotheken (winmm, enz.)
>
> Zorg ervoor dat `gcc`, `cmake` in uw `PATH` staan.

## Architectuuroverzicht

<p align="center">
  <img src="../../docs/assets/architecture.png" alt="architecture" />
</p>

Ga nog een stap verder: het biedt native ondersteuning voor **20+ IM-platforms**, **stemgestuurde** interfaces voor natuurlijke, contextbewuste dialoog, **zero-config modelwisseling** met IDE-scanning.

<p align="center">
  <img src="../../docs/assets/providers.png" alt="Supported Providers" />
</p>

## Hoe te bouwen

<p align="center">
  <img src="../../docs/assets/handcraft.png" alt="handcraft" />
</p>

> ⚠️ [!IMPORTANT]
>
> Als je van plan bent om tuning of vibe-codering bovenop Blue te blijven doen, behandel dan een paar mooie chats niet als vrijgavebewijs. Elke wijziging die van invloed is op de routing, het uitvoeringsgedrag, het gereedschapsoppervlak, de budgetcontrole, de modelselectie of het uitvoeringsframework moet worden gevalideerd met Blue Harness, niet met ad hoc steekproeven.
>
> Blue zou hier één eenvoudige regel moeten volgen: eerst data, eerst poorten, als laatste oversnijden. In de praktijk betekent dit dat u de relevante Harness dataset/eval-specificatie moet bijwerken voordat u een wijziging beoordeelt, en vervolgens één stabiele `candidate_id` gedurende de hele poging moet behouden, zodat selector-, uitvoerings-, budget- en gereedheidsrapporten allemaal dezelfde kandidaat beschrijven in plaats van vier niet-gerelateerde runs.

### Aanbevolen Harness Werkstroom

1. Voer `blue harness selector verify` uit
2. Voer `blue harness execution verify` uit
3. Hergebruik de selector-evaluatierun voor `blue harness budget gate`
4. Sluit af met `blue harness cutover-readiness`

Voor lokale iteratie, nachtelijke validatie of verzameling van CI-bewijs geeft u de voorkeur aan `python3 scripts/cutover_candidate_pipeline.py`. Het voert de volledige selector -> uitvoering -> budget -> gereedheidsreeks uit onder één gedeelde kandidaat, waardoor het resultaat gemakkelijker te vergelijken, te beoordelen en te verwijderen is.

### Extra leuningen

| Gebied | Wat te bekijken |
|------|---------------|
| Basislijnstabiliteit | Houd de basislijn, de datasetversie en `candidate_id` stabiel, anders zal de vergelijking afwijken en zal het resultaat niet betrouwbaar zijn. |
| Echte bouwoutput | Bouw de betreffende binaire of frontend-bundel opnieuw op voordat u Harness uitvoert, anders valideert u mogelijk verouderd gedrag in plaats van de huidige wijziging. |
| Routeregistratie | Als frontend en backend samen veranderen, bevestig dan dat alle nieuwe backend-routes daadwerkelijk worden geregistreerd voordat u de functie beoordeelt via UI-gedrag, omdat ontbrekende registratie vaak op een logische bug lijkt, maar in werkelijkheid een `404` is. |
| Vrijlating vonnis | Een tuning pass is alleen gereed als Harness geen betekenisvolle regressie vertoont en de gereedheid voor de overstap bevestigt dat de kandidaat daadwerkelijk klaar is om over te schakelen. |

Kortom, afstemmen op Blue gaat niet over "het voelt beter in een paar chats." Het gaat erom de kandidaat in Harness te plaatsen, vergelijkbaar bewijsmateriaal te verzamelen en de poort- en gereedheidsresultaten te laten beslissen of de verandering echt veilig is om te behouden.

## Kenmerken

| Kenmerk | Wat het oplevert |
|---------|------------------|
| Hoge beschikbaarheid via internet en browserruntime | Eén van Blue's **scherpste onderscheidende kenmerken**. Blue verenigt **vier webtoegangspaden** voor zoeken, lezen, uitpakken en crawlen; behoudt **drie fallback-lagen** voor HTTP, proxy-extractie en browsersessies; verwerkt **anti-botpagina's** met uitdagingsdetectie, hergebruik van cookies/sessies, stealth en browseroverdracht; en routes via **drie browser-engines**: `lightpanda`, beheerd Chromium en relay/lokaal Chrome. |
| Drie-in-één onderzoeksruntime | **Eén openbare onderzoeksinzending** kan worden doorgestuurd naar `deep_research`, `analyze` en `ui_review`. Dezelfde ontdekkings- en bewijsstapel levert vervolgens **citatie-eerst onderzoek**, **gebonden rapporten** en **gestructureerde UI/UX/toegankelijkheidsbeoordelingen** op. |
| Harness Runtime-, evaluatie- en evolutieframework | Maakt evaluatie een **runtime primitief** tijdens ontwikkeling, training en productie. Harness omvat **regressie- en rookcontroles**, scores, baselines, rapporten en runtime-validatie, en draagt ​​vervolgens hetzelfde bewijsmateriaal over in **vaardigheidsevolutie**, vervolgevaluatie, promotie of terugdraaiing, en `AGENTS.md` of beoordeling van instructievoorstellen. |
| Multimodale native-capability-first runtime | Houdt **stem, OCR, PDF, browsertaken, documentconversie, gestructureerd invullen van formulieren, mediaverwerking en lokale mediageneratie** op **eigen en lokale paden eerst**, met **modelroutering alleen wanneer dit daadwerkelijk nodig is**. |
| Beveiliging en bestuur | Inclusief **sandbox-uitvoering**, **prompt-injectieverdediging**, **sessie-audit**, machtigingen, **RBAC**, **WebAuthn**, operationele vangrails en **vaardigheidsbeveiligingsscans**. |
| LLM Wiki en kennisruimte | Verandert geheugen-, onderzoeks- en runtime-uitvoer in een **wiki-achtig kennisoppervlak** met **samenvattingspagina's**, indexen, **backlinks**, **nieuwheid** en **archiefworkflows**. |
| Vaardigheidswinkel en marktplaats | Biedt **ingebouwde detectie van vaardigheden**, beheer, synchronisatie en **lokaal scannen**, zodat uitbreidbaarheid **vanaf dag één** beschikbaar is. |
| Leverancierspool van productiekwaliteit | Biedt een echte providerpool met **gezondheidscontroles**, **automatische failover**, **stroomonderbrekers** en **providerracen** voor langlopende werklasten. |
| Ingebouwde lokale runtime voor kleine modellen | Levert een ingebouwde **`Qwen3.5-0.8B` + `llama.cpp`** runtime voor **lokale korte vragen en antwoorden**, beeldherkenning, toolrouting, samenvatting, **contextcompressie** en **documentvoorverwerking**. |
| Langdurige betrouwbaarheid | Behandelt **OTA-updates**, **back-up en herstel**, **hot reload van configuratie** en **herstel na een storing** als **ingebouwde operationele problemen**. |

## Mijlpaal-tijdlijn

<p align="center">
  <img src="../../docs/assets/timeline.png" alt="Milestone Timeline" />
</p>

| Datum | Versie | Trefwoorden / Kenmerken |
|------|---------|--------------------|
| 26 januari 2026 | `v0.1–v0.9` | Go runtime, plug-insysteem, browserautomatisering |
| 27–28 januari 2026 | `v0.9.0–v0.9.2` | Browsertaakweergave, Blue Companion, Smart Form Filler |
| 29–31 januari 2026 | `v0.10.0–v0.10.9` | Claude Code CLI, API Proxy, UI herstructureren |
| 1–3 februari 2026 | `v0.10.1–v0.10.22` | Statistieken, externe toegang, contextcache |
| 5–18 februari 2026 | `v0.10.25–v0.10.29` | i18n, CC Cache, releasepijplijn |
| 20–25 februari 2026 | `v0.10.28–v0.10.29` | Desktoplader, mobiele UX, herontwerp van geheugen |
| 28 februari–2 maart 2026 | `v0.10.30` | Deep Research, herrangschikking van vaardigheden, beveiligingsscan |
| 9–18 maart 2026 | `v0.10.31` | Dashboardrevisie, VoiceChat refactor, goedgekeurde sites |
| 19–22 maart 2026 | `v0.10.32` | Harness uitrol, transcriptieaudit, zoeken op internet |
| 23–25 maart 2026 | `v0.10.33` | Harness groepen, browsergoedkeuringen, vaardighedenmarkt |
| 29–30 maart 2026 | `v0.10.35` | Harness v3, browserrelay, contextcompressie |
| 31 maart – 1 april 2026 | `v0.10.36` | Transcriptaudit, Harness overlays, parseren van tools |
| 1 april 2026 | `v0.10.37` | Runtime-harding, Skill+Exec cutover, herstelpolish |
| 2–5 april 2026 | `v0.10.38` | GitHub ondersteuning, marktverfijning, betrouwbaarheidsverbeteringen |
| 6–7 april 2026 | `v0.10.39` | Onderzoeksunificatie, evolutieoppervlakken, vermindering van het geheugengebruik |

## Gemeenschap en ondersteuning

- **Problemen**: [Dien hier bugs en functieverzoeken in](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **Discussies**: [Discord](https://discord.gg/zwWbKA4S2)
- **Volg ons** op [GitHub](https://github.com/IceWhaleTech)

[![Star History Chart](https://api.star-history.com/svg?repos=IceWhaleTech/ZimaOS-Blue&type=Date)](https://star-history.com/#IceWhaleTech/ZimaOS-Blue&Date)

## Licentie

Dit project is gelicentieerd onder de MIT-licentie - zie het bestand [LICENSE](../../LICENSE) voor details. Wij geloven in open source en iets teruggeven aan de gemeenschap.

## Bijdragers

Dank aan alle Blue bijdragers:

<a href="https://community.vaunt.dev/board/IceWhaleTech/repository/ZimaOS-Blue">
  <img src="https://api.vaunt.dev/v1/github/entities/IceWhaleTech/repositories/ZimaOS-Blue/contributors?format=svg&limit=30" width="450" />
</a>

## Referenties

1. **OpenClaw** — Lokaal-eerste open source-agent. Was een pionier in het verbinden van LLM's met lokale apparaten via kanaaladapters en het aanroepen van tools, wat een directe inspiratie was voor de agent runtime-architectuur van Blue. https://github.com/openclaw/openclaw
2. **MiroMind** — Diepgaande onderzoeksmodus met op bewijzen gebaseerde synthese. Vormgegeven aan de ingebouwde diepgaande onderzoekspijplijn van Blue: planning, parallel ophalen, ontdubbelen van bewijsmateriaal en het genereren van HTML-rapporten. https://www.miromind.ai
3. **Karpathy's LLM Wiki** — LLM als kenniscompiler. Herformuleert LLM's om persistente, evoluerende kennisruimten te bouwen, die verder gaan dan de accumulatievalkuil van RAG.
4. **OpenSpace (HKUDS)** — Zelfontwikkelende vaardigheidsmotor. Een op DAG gebaseerd raamwerk waarin agenten leren van mislukkingen en gespecialiseerde vaardigheden afleiden. https://github.com/HKUDS/OpenSpace
5. **Andrew Ng's Context Hub** — Versie-API-documentatieregister voor codeeragenten. Pakt agentenhallucinaties en vergeten sessiekennis aan. Biedt samengestelde, versiebeheerde documenten met annotaties en feedbackloops, waardoor documentatie wordt omgezet in een zichzelf verbeterende kennislaag. https://github.com/andrewyng/context-hub
6. **Notion** — Eenvoudig, menselijk en opzettelijk stil. Geïnspireerd door het minimalistische ethos van Notion, brengt Blue warmte terug naar het grid. Waar verfijnde schreef en doordacht ontwerp samenkomen, ontstaat een ruimte die aanvoelt als thuis. https://www.notion.com/about
7. **Matrix** — Visuele inspiratie uit de iconische digitale regenesthetiek. De esthetische richting voor de technische diagrammen van Blue.
8. **IceWhale** — Liefde, dood en robots S2E2 "IJs". Een collectief dat wereldwijd samenkomt om de muren van internetgiganten te doorbreken en dataconcentratie tegen te gaan. De ijswalvis symboliseert een gemeenschap die samen aan de rand soevereine instrumenten bouwt.
9. **ZimaOS Blue** — Liefde, dood en robots S1E14 "Zima Blue". Een metafoor: intelligentie die begint in dienstbaarheid en evolueert om de wereld te verkennen. Blue is een vertegenwoordiger van wijsheid, geworteld in eenvoud en reikend naar diepgang.
10. **ZimaOS** — Vereenvoudigde, gerichte, open ontwerpprincipes. Zowel ZimaOS als Blue delen de overtuiging dat technologie de gebruiker moet dienen: binnen 30 seconden implementeren, overal draaien, leverancierneutraal blijven. https://www.zimaspace.com/zimaos
