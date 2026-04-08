![](../../docs/assets/bannerX.png)

<h2 align="center">ZimaOS Blue: En lokal-först agent-runtime för djärva byggare</h2>

<p align="center"><strong>Redo direkt från start · Öppen källkod · Universell · Leverantörsneutral</strong></p>

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
  <a href="../nl_NL/README.md">Nederlands</a> |
  <a href="../pl_PL/README.md">Polski</a> |
  <a href="../pt_BR/README.md">Português (BR)</a> |
  <a href="../pt_PT/README.md">Português (PT)</a> |
  <a href="../ro_RO/README.md">Română</a> |
  <a href="../ru_RU/README.md">Русский</a> |
  <a href="../sk_SK/README.md">Slovenčina</a> |
  <strong>Svenska</strong> |
  <a href="../zh_CN/README.md">简体中文</a> |
  <a href="../zh_TW/README.md">繁體中文</a>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Blue/ci.yml?branch=main&style=for-the-badge" alt="CI-status"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Blue?include_prereleases&style=for-the-badge" alt="GitHub-release"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT-licens"></a>
</p>

<p align="center">
  <a href="https://discord.gg/b3AgFDxe9v"><img src="../../docs/assets/discord.png" alt="Discord" height="128" /></a>&nbsp;&nbsp;
  <a href="https://www.facebook.com/zimaboard/"><img src="../../docs/assets/facebook.png" alt="Facebook" height="128" /></a>&nbsp;&nbsp;
  <a href="https://x.com/ZimaSpace"><img src="../../docs/assets/x.png" alt="X" height="128" /></a>
</p>

## Introduktion

Inspirerade av OpenClaw tror vi att framtiden för persondatorer kommer att formas av olika, lokala först AI-agenter som kör på kanten.

ZimaOS Blue är vårt svar — en fullständigt öppen källkod, granskningsbar, leverantörsneutral och produktionsklar agentkörning och verktygslåda som låter dig skicka privata agenter med egen värd utan friktion.

Blue är byggt för djärva utvecklare som vill vibba eller hantverka sina egna agenter. Blue är konstruerad för prestanda: skriven i Go, med ett minnesutrymme så lite som 19 MB. Den körs på valfri x86, <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS — var som helst du kopplar in ström.

## Demor

### Samtal och uppgiftskörning

En snabb demo av samtalsflödet och uppgiftskörningen i Blue.

<p align="center">
  <img src="../../docs/assets/demo.gif" alt="Conversation & Task Execution demo" width="800" />
</p>

### Integration av LLM-leverantörer

En snabb demo av Blues integration av LLM-leverantörer.

<p align="center">
  <img src="../../docs/assets/demo Provider.gif" alt="LLM Providers Integration demo" width="800" />
</p>

### Snabb översikt - Översikt, kanaler och ytterligare konfiguration

En snabb demo som täcker produktöversikten, kanalerna och ytterligare konfiguration.

<p align="center">
  <img src="../../docs/assets/demo quickv4.gif" alt="Quick Overview demo" width="720" />
</p>

## Varför Blue

<p align="center">
  <img src="../../docs/assets/design_principle.png" alt="Design Principle" />
</p>

### Pure Go, vilken enhet som helst

100 % Go, statisk binär. Korskompilerar till 5 mål ur lådan (![linux](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) `linux/amd64`, `linux/arm64`, ![macOS](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) `darwin/amd64`, `darwin/arm64`, ![windows](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) `windows/amd64`). Ingen nodkörning, ingen Python, inga behållare krävs. Släpp den på en NAS, en <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS, en ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, en gammal x86-router eller en ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)Mac — den bara körs. Lägg sedan på ditt eget användargränssnitt, logik och agentfärdigheter – en kodbas, varje plattform.

### ur kartongen, redo att arbeta

Alla vill ha verktyg som är enkla, pålitliga och skalbara när du behöver dem. Verktyg som bara fungerar, så att du kan fokusera på det du faktiskt bygger.

Det här är ingen ny filosofi. Det är samma som byggde <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS: enkel, pålitlig och byggd för att hålla sig ur vägen. Blue är den filosofin, utvidgad till agentstacken.

### Designad för ditt liv, byggd för att förbli lokal

Från djup forskning som levererar en fullständig HTML-rapport, till OCR, PDF, webbläsarautomation och dokumentkonvertering, Blue hanterar komplexa, verkliga arbetsflöden utan att skicka dina data till molnet. Voice wake, STT/TTS, Talk Mode, och stöd för lokal slutledning gör vardagliga interaktioner omedelbara, privata och alltid tillgängliga.

## Snabbstart

### Alternativ 1: Ladda ner skrivbordsappen

Skaffa den inbyggda applikationen - inga beroenden, ingen kompilering. Inbyggd provkonfiguration med onboarding på några sekunder — börja chatta direkt via fjärranslutning, ingen botinstallation krävs. Verklig upplevelse direkt.

- <img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="16" /> **ZimaOS**: [Kör på ZimaOS](https://www.zimaspace.com/zimaos?utm_source=blue)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)**macOS**: [Ladda ner DMG](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)**Windows**: [Ladda ned installationsprogram](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### Alternativ 2: Installera skript

<img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="16" /> ZimaOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### Alternativ 3: Bygg från källan

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

> **Obs!** Windows-versioner kräver:
> - [MinGW-w64](https://www.mingw-w64.org/) (gcc) och [CMake](https://cmake.org/) för infödda C-beroenden (espeak-ng, whisper.cpp, opus, kokoro, onnx)
> - [Windows SDK](https://developer.microsoft.com/en-us/windows/downloads/windows-sdk/) för systembibliotek (winmm, etc.)
>
> Se till att `gcc`, `cmake` finns i din `PATH`.

## Arkitekturöversikt

<p align="center">
  <img src="../../docs/assets/architecture.png" alt="architecture" />
</p>

Ta det vidare: det ger inbyggt stöd för **20+ IM-plattformar**, **röstdrivna** gränssnitt för naturlig, sammanhangsmedveten dialog, **noll-konfigurerad modellväxling** med IDE-skanning.

<p align="center">
  <img src="../../docs/assets/providers.png" alt="Supported Providers" />
</p>

## Hur man bygger

<p align="center">
  <img src="../../docs/assets/handcraft.png" alt="handcraft" />
</p>

> ⚠️ [!IMPORTANT]
>
> Om du planerar att fortsätta trimma eller vibba kodning ovanpå Blue, behandla inte några snygga chattar som releasebevis. Alla ändringar som påverkar routing, exekveringsbeteende, verktygsyta, budgetkontroll, modellval eller exekveringsramverket bör valideras med Blue Harness, inte med ad hoc-stickkontroller.
>
> Blue bör följa en enkel regel här: data först, grindar först, skär över sist. I praktiken innebär det att man uppdaterar den relevanta Harness-datauppsättningen/eval-specifikationen innan man bedömer en förändring, och sedan håller man en stabil `candidate_id` under hela försöket så att väljar-, utförande-, budget- och beredskapsrapporter alla beskriver samma kandidat istället för fyra orelaterade körningar.

### Rekommenderas Harness Arbetsflöde

1. Kör `blue harness selector verify`
2. Kör `blue harness execution verify`
3. Återanvänd väljarevalkörningen för `blue harness budget gate`
4. Avsluta med `blue harness cutover-readiness`

För lokal iteration, nattlig validering eller insamling av CI-bevis, föredra `python3 scripts/cutover_candidate_pipeline.py`. Den kör hela väljaren -> utförande -> budget -> beredskapssekvensen under en delad kandidat, vilket gör resultatet lättare att jämföra, granska och skära över från.

### Extra skyddsräcken

| Område | Vad man ska titta på |
|------|----------------|
| Baslinjestabilitet | Håll baslinjen, datauppsättningsversionen och `candidate_id` stabil, annars kommer jämförelsen att glida och resultatet blir inte tillförlitligt. |
| Real build output | Bygg om det berörda binära eller frontend-paketet innan du kör Harness, annars kan du sluta med att validera inaktuellt beteende istället för den aktuella ändringen. |
| Ruttregistrering | Om frontend och backend ändras tillsammans, bekräfta att alla nya backend-rutter faktiskt är registrerade innan du bedömer funktionen genom UI-beteende, eftersom saknad registrering ofta ser ut som en logisk bugg men är verkligen en `404`. |
| Frigivningsdom | Ett trimpass är bara klart när Harness inte visar någon meningsfull regression och cutover-beredskap bekräftar att kandidaten faktiskt är redo att skära över. |

Kort sagt, att trimma ovanpå Blue handlar inte om "det känns bättre i några chattar." Det handlar om att sätta in kandidaten i Harness, samla in jämförbara bevis och låta porten och beredskapsresultaten avgöra om förändringen verkligen är säker att behålla.

## Funktioner

| Funktion | Vad det levererar |
|--------|------------------------|
| Webbhämtning och webbläsarkörning med hög tillgänglighet | En av Blues **skäraste differentiatorer**. Blue förenar **fyra webbåtkomstvägar** för sökning, läsning, extrahering och genomsökning; behåller **tre reservlager** över HTTP, proxyextraktion och webbläsarsessioner; hanterar **anti-bot-sidor** med utmaningsdetektering, återanvändning av cookies/sessioner, stealth och webbläsarhandoff; och rutter över **tre webbläsarmotorer**: `lightpanda`, hanterad Chromium och relä/lokal Chrome. |
| Tre-i-ett forskning Runtime | **Ett offentligt forskningsbidrag** kan dirigeras till `deep_research`, `analyze` och `ui_review`. Samma upptäckt och bevisstack producerar sedan **citatförst forskning**, **avgränsade rapporter** och **strukturerade UI/UX/tillgänglighetsgranskningar**. |
| Harness Framework för körning, utvärdering och utveckling | Gör utvärdering till en **runtime primitiv** över utveckling, utbildning och produktion. Harness täcker **regression och rökkontroller**, poängsättning, baslinjer, rapporter och körtidsvalidering, och bär sedan samma bevis i **kompetensutveckling**, uppföljningsutvärdering, marknadsföring eller återställning och `AGENTS.md` eller granskning av instruktionsförslag. |
| Multimodal Native-Capability-First Runtime | Håller **röst, OCR, PDF, webbläsaruppgifter, dokumentkonvertering, ifyllning av strukturerade formulär, mediabearbetning och lokal mediagenerering** på **infödda och lokala vägar först**, med **modellrouting endast när det verkligen behövs**. |
| Säkerhet och styrning | Inkluderar **utförande av sandlåde**, **försvar av prompt-injektion**, **sessionsrevision**, behörigheter, **RBAC**, **WebAuthn**, operativa skyddsräcken och **säkerhetsskanning av skicklighet**. |
| LLM Wiki och kunskapsutrymme | Förvandlar minne, forskning och körtidsutdata till en **wikiliknande kunskapsyta** med **sammanfattningssidor**, index, **bakåtlänkar**, **nyhet** och **arkivarbetsflöden**. |
| Skicklighetsbutik och marknadsplats | Skickar **inbyggd kunskapsupptäckt**, kurering, synkronisering och **lokal skanning** så att utökningsbarhet är tillgänglig **från dag ett**. |
| Produktionsklass leverantörspool | Ger en riktig leverantörspool med **hälsokontroller**, **automatisk failover**, **kretsbrytare** och **leverantörsracing** för långvariga arbetsbelastningar. |
| Inbyggd lokal liten modell körtid | Skickar en inbyggd **`Qwen3.5-0.8B` + `llama.cpp`** körtid för **lokala korta frågor och svar**, bildigenkänning, verktygsdirigering, sammanfattning, **kontextkomprimering** och **dokumentförbehandling**. |
| Långvarig tillförlitlighet | Behandlar **OTA-uppdateringar**, **säkerhetskopiering och återställning**, **varm omladdning av konfiguration** och **återställning efter fel** som **inbyggda driftsproblem**. |

## Milstolpe Tidslinje

<p align="center">
  <img src="../../docs/assets/timeline.png" alt="Milestone Timeline" />
</p>

| Datum | Version | Nyckelord / Funktioner |
|------|--------|------------------------|
| 26 januari 2026 | `v0.1–v0.9` | Go runtime, plugin-system, webbläsarautomatisering |
| 27–28 januari 2026 | `v0.9.0–v0.9.2` | Webbläsaruppgiftsvy, Blue Companion, Smart Form Filler |
| 29–31 januari 2026 | `v0.10.0–v0.10.9` | Claude Code CLI, API Proxy, UI-omstrukturering |
| 1–3 februari 2026 | `v0.10.1–v0.10.22` | Mätvärden, fjärråtkomst, kontextcache |
| 5–18 februari 2026 | `v0.10.25–v0.10.29` | i18n, CC-cache, releasepipeline |
| 20–25 februari 2026 | `v0.10.28–v0.10.29` | Desktop loader, mobil UX, minne redesign |
| 28 februari–2 mars 2026 | `v0.10.30` | Deep Research, omplacering av färdigheter, säkerhetsskanning |
| 9–18 mars 2026 | `v0.10.31` | Översyn av instrumentbrädan, VoiceChat refactor, godkända platser |
| 19–22 mars 2026 | `v0.10.32` | Harness utrullning, transkriptionsgranskning, webbsökning |
| 23–25 mars 2026 | `v0.10.33` | Harness grupper, webbläsargodkännanden, kompetensmarknad |
| 29–30 mars 2026 | `v0.10.35` | Harness v3, webbläsarrelä, kontextkomprimering |
| 31 mars–1 april 2026 | `v0.10.36` | Transkriptionsgranskning, Harness överlagringar, verktygsanalys |
| 1 april 2026 | `v0.10.37` | Runtime-härdning, Skill+Exec cutover, återvinningspolish |
| 2–5 apr 2026 | `v0.10.38` | GitHub support, marknadsplatsförfining, tillförlitlighetsförbättringar |
| 6–7 apr 2026 | `v0.10.39` | Forskningssamordning, evolutionsytor, minskat minnesfotavtryck |

## Community och support

- **Problem**: [Var vänlig arkivera buggar och funktionsförfrågningar här](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **Diskusioner**: [Discord](https://discord.gg/zwWbKA4S2)
- **Följ oss** på [GitHub](https://github.com/IceWhaleTech)

[![Star History Chart](https://api.star-history.com/svg?repos=IceWhaleTech/ZimaOS-Blue&type=Date)](https://star-history.com/#IceWhaleTech/ZimaOS-Blue&Date)

## Licens

Det här projektet är licensierat under MIT-licensen - se filen [LICENSE](../../LICENSE) för detaljer. Vi tror på öppen källkod och att ge tillbaka till samhället.

## Bidragsgivare

Tack till alla Blue bidragsgivare:

<a href="https://community.vaunt.dev/board/IceWhaleTech/repository/ZimaOS-Blue">
  <img src="https://api.vaunt.dev/v1/github/entities/IceWhaleTech/repositories/ZimaOS-Blue/contributors?format=svg&limit=30" width="450" />
</a>

## Referenser

1. **OpenClaw** — Lokalt första öppen källkodsagent. Blev banbrytande för att ansluta LLM:er till lokala enheter genom kanaladaptrar och verktygsanrop, vilket direkt inspirerade Blues agentkörningsarkitektur. https://github.com/openclaw/openclaw
2. **MiroMind** — Djupt forskningsläge med evidensstödd syntes. Formade Blues inbyggda djupa forskningspipeline: planering, parallell hämtning, bevisdeduplicering och HTML rapportgenerering. https://www.miromind.ai
3. **Karpathy's LLM Wiki** — LLM som kunskapskompilator. Reframes LLMs för att bygga beständiga, utvecklande kunskapsutrymmen, som går bortom RAG:s ackumuleringsfälla.
4. **OpenSpace (HKUDS)** — Självutvecklande skicklighetsmotor. Ett DAG-baserat ramverk där agenter lär sig av misslyckanden och får specialiserade färdigheter. https://github.com/HKUDS/OpenSpace
5. **Andrew Ng's Context Hub** — Versionerat API-dokumentationsregister för kodningsagenter. Tar upp agenthallucinationer och glömd sessionskunskap. Tillhandahåller utvalda, versionerade dokument med antecknings- och feedbackloopar, vilket gör dokumentation till ett självförbättrande kunskapsskikt. https://github.com/andrewyng/context-hub
6. **Notion** — Enkelt, mänskligt och avsiktligt tyst. Inspirerad av Notions minimalistiska etos tar Blue tillbaka värmen till nätet. Där raffinerad serif möter genomtänkt design, skapa ett utrymme som känns som hemma. https://www.notion.com/about
7. **Matrix** — Visuell inspiration från den ikoniska digitala regnestetiken. Den estetiska riktningen för Blues tekniska diagram.
8. **IceWhale** — Love, Death & Robots S2E2 "Ice". Ett kollektiv som samlas över hela världen för att bryta igenom internetjättarnas väggar och motstå datakoncentration. Isvalen symboliserar ett samhälle som bygger suveräna verktyg tillsammans vid kanten.
9. **ZimaOS Blue** — Love, Death & Robots S1E14 "Zima Blue". En metafor: intelligens som börjar i tjänst och utvecklas för att utforska världen. Blue är en agent för visdom, rotad i enkelhet och strävar efter djup.
10. **ZimaOS** — Förenklade, fokuserade, öppna designprinciper. Både ZimaOS och Blue delar övertygelsen om att tekniken ska tjäna användaren - implementera på 30 sekunder, kör var som helst, förbli leverantörsneutral. https://www.zimaspace.com/zimaos
