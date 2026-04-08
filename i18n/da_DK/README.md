![](../../docs/assets/bannerX.png)

<h2 align="center">ZimaOS Blue: En lokal-først agent-runtime til modige byggere</h2>

<p align="center"><strong>Klar fra start · Open source · Universel · Leverandørneutral</strong></p>

<p align="center">
  <a href="../../README.md">English</a> |
  <a href="../ca_ES/README.md">Català</a> |
  <a href="../cs_CZ/README.md">Čeština</a> |
  <strong>Dansk</strong> |
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
  <a href="../nl_NL/README.md">Nederlands</a> |
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
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Blue/ci.yml?branch=main&style=for-the-badge" alt="CI-status"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Blue?include_prereleases&style=for-the-badge" alt="GitHub-udgivelse"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT-licens"></a>
</p>

<p align="center">
  <a href="https://discord.gg/b3AgFDxe9v"><img src="../../docs/assets/discord.png" alt="Discord" height="128" /></a>&nbsp;&nbsp;
  <a href="https://www.facebook.com/zimaboard/"><img src="../../docs/assets/facebook.png" alt="Facebook" height="128" /></a>&nbsp;&nbsp;
  <a href="https://x.com/ZimaSpace"><img src="../../docs/assets/x.png" alt="X" height="128" /></a>
</p>

## Introduktion

Inspireret af Clawdbot tror vi, at fremtiden for personlig computer vil blive formet af forskellige, lokale første AI-agenter, der kører på kanten.

ZimaOS Blue er vores svar - en fuld open source, reviderbar, leverandørneutral og produktionsklar agentruntime og værktøjssæt, der lader dig sende private, selv-hostede agenter uden friktion.

Bygget til dristige udviklere, der ønsker at vibe eller håndlave deres egne agenter, Blue er udviklet til ydeevne: skrevet i Go, med et hukommelsesfodaftryk så lavt som 19 MB. Den kører på enhver x86, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS - hvor som helst du tilslutter strøm.

## Demoer

### Samtale og opgaveudførelse

En hurtig demo af samtaleflow og opgaveudførelse i Blue.

<p align="center">
  <img src="../../docs/assets/demo.gif" alt="Conversation & Task Execution demo" width="800" />
</p>

### Integration af LLM-udbydere

En hurtig demo af Blues integrationsoplevelse for LLM-udbydere.

<p align="center">
  <img src="../../docs/assets/demo Provider.gif" alt="LLM Providers Integration demo" width="800" />
</p>

### Hurtigt overblik - Oversigt, kanaler og ekstra konfiguration

En hurtig demo, der dækker det overordnede produktoverblik, kanaler og ekstra konfiguration.

<p align="center">
  <img src="../../docs/assets/demo quickv4.gif" alt="Quick Overview demo" width="720" />
</p>

## Hvorfor Blue

<p align="center">
  <img src="../../docs/assets/design_principle.png" alt="Design Principle" />
</p>

### Pure Go, enhver enhed

100% Go, statisk binær. Krydskompilerer til 5 mål ud af boksen (![linux](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) `linux/amd64`, `linux/arm64`, ![macOS](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) `darwin/amd64`, `darwin/arm64`, ![windows](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) `windows/amd64`). Ingen node-runtime, ingen Python, ingen containere påkrævet. Slip den på en NAS, en ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, en gammel x86-router eller en ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)Mac - den kører bare. Så lag på din egen brugergrænseflade, logik og agentfærdigheder - én kodebase, hver platform.

### Ud af kassen, klar til at arbejde

Alle vil have værktøjer, der er enkle, pålidelige og skalere, når du har brug for dem. Værktøjer, der bare virker, så du kan fokusere på det, du rent faktisk bygger.

Dette er ikke en ny filosofi. Det er den samme, der byggede <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS: enkel, pålidelig og bygget til at holde sig ude af vejen. Blue er denne filosofi, udvidet til agentstakken.

### Designet til dit liv, bygget til at forblive lokalt

Fra dyb research, der leverer en komplet HTML-rapport, til OCR, PDF, browserautomatisering og dokumentkonvertering, Blue håndterer komplekse arbejdsgange i den virkelige verden uden at sende dine data til skyen. Voice wake, STT/TTS, Talk Mode og understøttelse af lokal inferens gør daglige interaktioner øjeblikkelige, private og altid tilgængelige.

## Hurtig start

### Mulighed 1: Download Desktop App

Hent den oprindelige applikation - ingen afhængigheder, ingen kompilering. Indbygget prøvekonfiguration med onboarding på få sekunder - begynd at chatte med det samme via fjernforbindelse, ingen bot-opsætning påkrævet. Ægte out-of-the-box oplevelse.

- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)**macOS**: [Download DMG](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)**Windows**: [Download installationsprogram](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### Mulighed 2: Installer script

![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### Mulighed 3: Byg fra kilde

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue
git submodule update --init --recursive
```

![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
sh build.sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
.\build.bat
```

> **Bemærk:** Windows-builds kræver:
> - [MinGW-w64](https://www.mingw-w64.org/) (gcc) og [CMake](https://cmake.org/) for native C-afhængigheder (espeak-ng, whisper.cpp, opus, kokoro, onnx)
> - [Windows SDK](https://developer.microsoft.com/en-us/windows/downloads/windows-sdk/) til systembiblioteker (winmm osv.)
>
> Sørg for, at `gcc`, `cmake` er i din `PATH`.

## Arkitekturoversigt

<p align="center">
  <img src="../../docs/assets/architecture.png" alt="architecture" />
</p>

Tag det videre: det leverer indbygget understøttelse af **20+ IM-platforme**, **stemmedrevne** grænseflader til naturlig, kontekstbevidst dialog, **nul-konfigurationsmodelskift** med IDE-scanning.

<p align="center">
  <img src="../../docs/assets/providers.png" alt="Supported Providers" />
</p>

## Sådan bygger du

<p align="center">
  <img src="../../docs/assets/handcraft.png" alt="handcraft" />
</p>

> ⚠️ [!IMPORTANT]
>
> Hvis du planlægger at blive ved med at tune eller vibe-kodning oven på Blue, skal du ikke behandle nogle få flotte chats som udgivelsesbeviser. Enhver ændring, der påvirker routing, udførelsesadfærd, værktøjsoverflade, budgetkontrol, modelvalg eller udførelsesramme, skal valideres med Blue Harness, ikke med ad hoc stikprøvekontrol.
>
> Blue bør følge en simpel regel her: data først, porte først, skær over sidst. I praksis betyder det, at man opdaterer det relevante Harness-datasæt/eval-specifikation, før man bedømmer en ændring, og derefter holder én stabil `candidate_id` på tværs af hele forsøget, så rapporter om valg, udførelse, budget og parathed alle beskriver den samme kandidat i stedet for fire ikke-relaterede kørsler.

### Anbefalet Harness Workflow

1. Kør `blue harness selector verify`
2. Kør `blue harness execution verify`
3. Genbrug vælgerevalkørslen for `blue harness budget gate`
4. Afslut med `blue harness cutover-readiness`

For lokal iteration, natlig validering eller indsamling af CI-beviser foretrækkes `python3 scripts/cutover_candidate_pipeline.py`. Den kører hele vælgeren -> udførelse -> budget -> parathedssekvens under én delt kandidat, hvilket gør resultatet nemmere at sammenligne, gennemgå og skære over fra.

### Ekstra autoværn

| Område | Hvad skal man se |
|------|----------------|
| Baseline stabilitet | Hold basislinjen, datasætversionen og `candidate_id` stabil, ellers vil sammenligningen glide, og resultatet vil ikke være troværdigt. |
| Real build output | Genopbyg den berørte binære eller frontend-pakke, før du kører Harness, ellers kan du ende med at validere forældet adfærd i stedet for den aktuelle ændring. |
| Ruteregistrering | Hvis frontend og backend ændres sammen, skal du bekræfte, at alle nye backend-ruter faktisk er registreret, før du bedømmer funktionen gennem UI-adfærd, fordi manglende registrering ofte ligner en logisk fejl, men er virkelig en `404`. |
| Frigivelsesdom | Et tuningpas er først klar, når Harness ikke viser nogen meningsfuld regression, og cutover-beredskab bekræfter, at kandidaten faktisk er klar til at skære over. |

Kort sagt handler tuning oven på Blue ikke om "det føles bedre i et par chats." Det handler om at sætte kandidaten ind i Harness, indsamle sammenlignelige beviser og lade porten og beredskabsresultaterne afgøre, om ændringen virkelig er sikker at beholde.

## Funktioner

| Funktion | Hvad det leverer |
|--------|------------------------|
| Webhentning og browserkørsel med høj tilgængelighed | En af Blues **skarpeste differentiatorer**. Blue forener **fire webadgangsstier** til søgning, læsning, udtræk og crawl; beholder **tre fallback-lag** på tværs af HTTP, proxy-ekstraktion og browsersessioner; håndterer **anti-bot-sider** med udfordringsdetektion, genbrug af cookies/sessioner, stealth og browseroverdragelse; og ruter på tværs af **tre browsermotorer**: `lightpanda`, administreret Chromium og relæ/lokal Chromium. |
| Three-in-One Research Runtime | **Et offentligt forskningsbidrag** kan sendes til `deep_research`, `analyze` og `ui_review`. Den samme opdagelses- og bevisstak producerer derefter **citationsførst-forskning**, **afgrænsede rapporter** og **strukturerede UI/UX/tilgængelighedsanmeldelser**. |
| Harness Runtime, Evaluation og Evolution Framework | Gør evaluering til en **runtime primitiv** på tværs af udvikling, træning og produktion. Harness dækker **regressions- og røgtjek**, scoring, baselines, rapporter og runtime-validering og fører derefter den samme dokumentation ind i **færdighedsudvikling**, opfølgende evaluering, promovering eller tilbagerulning og `AGENTS.md` eller gennemgang af instruktionsforslag. |
| Multimodal Native-Capability-First Runtime | Holder **stemme, OCR, PDF, browseropgaver, dokumentkonvertering, struktureret formularudfyldning, mediebehandling og lokal mediegenerering** på **native og lokale stier først**, med **modelrouting kun, når det faktisk er nødvendigt**. |
| Sikkerhed og styring | Inkluderer **udførelse af sandkasse**, **prompt-injektionsforsvar**, **sessionsauditering**, tilladelser, **RBAC**, **WebAuthn**, operationelle autoværn og **sikkerhedsscanning af færdigheder**. |
| LLM Wiki og vidensrum | Forvandler hukommelse, research og runtime-output til en **wiki-lignende vidensoverflade** med **oversigtssider**, indekser, **backlinks**, **friskhed** og **arkivarbejdsgange**. |
| Skill Store og markedsplads | Sender **indbygget færdighedsopdagelse**, kurering, synkronisering og **lokal scanning**, så udvidelsesmuligheder er tilgængelig **fra dag ét**. |
| Udbyderpulje i produktionskvalitet | Giver en ægte udbyderpulje med **sundhedstjek**, **automatisk failover**, **kredsløbsafbrydere** og **udbyderracing** til langvarige arbejdsbelastninger. |
| Indbygget lokal lille model runtime | Sender en indbygget **`Qwen3.5-0.8B` + `llama.cpp`** runtime til **lokale korte spørgsmål og svar**, billedgenkendelse, værktøjsrouting, opsummering, **kontekstkomprimering** og **dokumentforbehandling**. |
| Langvarig pålidelighed | Behandler **OTA-opdateringer**, **sikkerhedskopiering og gendannelse**, **hot reload af konfiguration** og **gendannelse efter fejl** som **indbyggede driftsproblemer**. |

## Milepæls tidslinje

<p align="center">
  <img src="../../docs/assets/timeline.png" alt="Milestone Timeline" />
</p>

| Dato | Version | Nøgleord / funktioner |
|------|--------|---------------------|
| 26. januar 2026 | `v0.1–v0.9` | Go runtime, plugin-system, browserautomatisering |
| 27.-28. januar 2026 | `v0.9.0–v0.9.2` | Browseropgavevisning, Blue Companion, Smart Form Filler |
| 29.-31. januar 2026 | `v0.10.0–v0.10.9` | Claude Code CLI, API Proxy, UI-omstrukturering |
| 1-3. februar 2026 | `v0.10.1–v0.10.22` | Metrikker, fjernadgang, kontekstcache |
| 5.-18. februar 2026 | `v0.10.25–v0.10.29` | i18n, CC Cache, release pipeline |
| 20.-25. februar 2026 | `v0.10.28–v0.10.29` | Desktop-loader, mobil UX, redesign af hukommelse |
| 28. februar – 2. marts 2026 | `v0.10.30` | Deep Research, omstilling af færdigheder, sikkerhedsscanning |
| 9-18 marts 2026 | `v0.10.31` | Dashboard eftersyn, VoiceChat refactor, godkendte steder |
| 19-22 marts 2026 | `v0.10.32` | Harness udrulning, transskriptionsrevision, websøgning |
| 23.-25. marts 2026 | `v0.10.33` | Harness grupper, browsergodkendelser, kompetencemarked |
| 29.-30. marts 2026 | `v0.10.35` | Harness v3, browserrelæ, kontekstkomprimering |
| 31. marts – 1. april 2026 | `v0.10.36` | Transskriptionsaudit, Harness overlejringer, værktøjsparsing |
| 1. april 2026 | `v0.10.37` | Runtime hærdning, Skill+Exec cutover, gendannelsespolering |
| 2.-5. april 2026 | `v0.10.38` | GitHub support, forfining af markedspladsen, forbedringer af pålidelighed |
| 6-7 apr 2026 | `v0.10.39` | Forskningssamling, udviklingsflader, reduceret hukommelsesforbrug |

## Fællesskab og support

- **Problemer**: [Send venligst fejl og funktionsanmodninger her](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **Diskussioner**: [Discord](https://discord.gg/zwWbKA4S2)
- **Følg os** på [GitHub](https://github.com/IceWhaleTech)

[![Star History Chart](https://api.star-history.com/svg?repos=IceWhaleTech/ZimaOS-Blue&type=Date)](https://star-history.com/#IceWhaleTech/ZimaOS-Blue&Date)

## Licens

Dette projekt er licenseret under MIT-licensen - se filen [LICENSE](../../LICENSE) for detaljer. Vi tror på open source og at give tilbage til samfundet.

## Bidragydere

Tak til alle Blue bidragydere:

<a href="https://community.vaunt.dev/board/IceWhaleTech/repository/ZimaOS-Blue">
  <img src="https://api.vaunt.dev/v1/github/entities/IceWhaleTech/repositories/ZimaOS-Blue/contributors?format=svg&limit=30" width="450" />
</a>

## Referencer

1. **OpenClaw** — Lokal-første open source-agent. Banebrydende tilslutning af LLM'er til lokale enheder gennem kanaladaptere og værktøjsopkald, hvilket direkte inspirerede Blues agent runtime-arkitektur. https://github.com/openclaw/openclaw
2. **MiroMind** — Dyb research mode med evidensunderbygget syntese. Formede Blues indbyggede dybe forskningspipeline: planlægning, parallel hentning, evidens-deduplikering og HTML rapportgenerering. https://www.miromind.ai
3. **Karpathy's LLM Wiki** — LLM som videnskompiler. Reframes LLM'er for at bygge vedvarende, udviklende videnrum, der bevæger sig ud over RAG's akkumuleringsfælde.
4. **OpenSpace (HKUDS)** — Selvudviklende færdighedsmotor. En DAG-baseret ramme, hvor agenter lærer af fiaskoer og udleder specialiserede færdigheder. https://github.com/HKUDS/OpenSpace
5. **Andrew Ng's Context Hub** — Versioneret API-dokumentationsregister til kodningsagenter. Adresser agent hallucinationer og glemt session viden. Giver kuraterede, versionerede dokumenter med annoterings- og feedbackloops, der gør dokumentation til et selvforbedrende videnslag. https://github.com/andrewyng/context-hub
6. **Notion** — Enkelt, menneskeligt og bevidst stille. Inspireret af Notions minimalistiske etos bringer Blue varmen tilbage til nettet. Hvor raffineret serif møder gennemtænkt design, der skaber et rum, der føles som hjemme. https://www.notion.com/about
7. **Matrix** — Visuel inspiration fra den ikoniske digitale regn-æstetik. Den æstetiske retning for Blues tekniske diagrammer.
8. **IceWhale** — Love, Death & Robots S2E2 "Ice". Et kollektiv, der samles over hele verden for at bryde igennem internetgiganternes mure og modstå datakoncentration. Ishvalen symboliserer et fællesskab, der bygger suveræne redskaber sammen ved kanten.
9. **ZimaOS Blue** — Love, Death & Robots S1E14 "Zima Blue". En metafor: intelligens, der begynder i tjeneste og udvikler sig for at udforske verden. Blue er en agent for visdom, rodfæstet i enkelhed og stræber efter dybde.
10. **ZimaOS** — Forenklede, fokuserede, åbne designprincipper. Både ZimaOS og Blue deler troen på, at teknologien skal tjene brugeren - implementer på 30 sekunder, kør hvor som helst, forbliv leverandørneutral. https://www.zimaspace.com/zimaos
