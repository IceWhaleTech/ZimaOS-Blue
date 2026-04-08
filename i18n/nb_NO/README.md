![](../../docs/assets/bannerX.png)

<h2 align="center">ZimaOS Blue: En lokal-først agent-runtime for modige byggere</h2>

<p align="center"><strong>Klar rett ut av boksen · Åpen kildekode · Universell · Leverandørnøytral</strong></p>

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
  <strong>Norsk Bokmål</strong> |
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
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Blue?include_prereleases&style=for-the-badge" alt="GitHub-utgivelse"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT-lisens"></a>
</p>

<p align="center">
  <a href="https://discord.gg/b3AgFDxe9v"><img src="../../docs/assets/discord.png" alt="Discord" height="128" /></a>&nbsp;&nbsp;
  <a href="https://www.facebook.com/zimaboard/"><img src="../../docs/assets/facebook.png" alt="Facebook" height="128" /></a>&nbsp;&nbsp;
  <a href="https://x.com/ZimaSpace"><img src="../../docs/assets/x.png" alt="X" height="128" /></a>
</p>

## Introduksjon

Inspirert av OpenClaw tror vi fremtiden for personlig databehandling vil bli formet av ulike, lokale første AI-agenter som kjører på kanten.

ZimaOS Blue er svaret vårt – en fullstendig åpen kildekode, reviderbar, leverandørnøytral og produksjonsklar agentkjøring og verktøysett som lar deg sende private, selvvertsbaserte agenter uten friksjon.

Blue er bygget for dristige utviklere som vil vibe eller håndlage sine egne agenter, og er utviklet for ytelse: skrevet i Go, med et minneavtrykk helt ned mot 19 MB. Den kjører på alle x86-systemer, <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows og ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS — hvor enn du har strøm.

## Demoer

### Samtale og oppgaveutførelse

En rask demo av samtaleflyten og oppgaveutførelsen i Blue.

<p align="center">
  <img src="../../docs/assets/demo.gif" alt="Conversation & Task Execution demo" width="800" />
</p>

### Integrasjon av LLM-leverandører

En rask demo av Blues integrasjon av LLM-leverandører.

<p align="center">
  <img src="../../docs/assets/demo Provider.gif" alt="LLM Providers Integration demo" width="800" />
</p>

### Rask oversikt - Oversikt, kanaler og ekstra konfigurasjon

En rask demo som dekker produktoverblikket, kanaler og ekstra konfigurasjon.

<p align="center">
  <img src="../../docs/assets/demo quickv4.gif" alt="Quick Overview demo" width="720" />
</p>

## Hvorfor Blue

<p align="center">
  <img src="../../docs/assets/design_principle.png" alt="Design Principle" />
</p>

### Ren Go, enhver enhet

100 % Go, statisk binær. Krysskompilerer til 5 mål rett ut av esken (![linux](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) `linux/amd64`, `linux/arm64`, ![macOS](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) `darwin/amd64`, `darwin/arm64`, ![windows](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) `windows/amd64`). Ingen Node-runtime, ingen Python, og ingen containere kreves. Slipp den på en NAS, en <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS, en ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, en gammel x86-ruter eller en ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)Mac – den bare kjører. Deretter legger du på ditt eget brukergrensesnitt, din egen logikk og agentferdigheter – én kodebase, alle plattformer.

### Ut av esken, klar til å jobbe

Alle vil ha verktøy som er enkle, pålitelige og kan skalere når du trenger dem. Verktøy som bare fungerer, slik at du kan fokusere på det du faktisk bygger.

Dette er ikke en ny filosofi. Det er den samme som bygde <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS: enkel, pålitelig og bygget for å holde deg unna. Blue er den filosofien, utvidet til agentstabelen.

### Designet for livet ditt, bygget for å forbli lokalt

Fra dyp forskning som leverer en fullstendig HTML-rapport, til OCR, PDF, nettleserautomatisering og dokumentkonvertering, håndterer Blue komplekse arbeidsflyter i den virkelige verden uten å sende dataene dine til skyen. Stemmevekking, STT/TTS, Talk Mode og støtte for lokal inferens gjør daglige interaksjoner øyeblikkelige, private og alltid tilgjengelige.

## Hurtigstart

### Alternativ 1: Last ned skrivebordsapp

Få den opprinnelige applikasjonen - ingen avhengigheter, ingen kompilering. Innebygd prøvekonfigurasjon med onboarding på sekunder – begynn å chatte umiddelbart via ekstern tilkobling, ingen bot-oppsett kreves. Ekte ut-av-boksen opplevelse.

- <img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="16" /> **ZimaOS**: [Kjør på ZimaOS](https://www.zimaspace.com/zimaos?utm_source=blue)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)**macOS**: [Last ned DMG](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)**Windows**: [Last ned installasjonsprogram](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### Alternativ 2: Installer skript

<img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="16" /> ZimaOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### Alternativ 3: Bygg fra kilden

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

> **Merk:** Windows-bygg krever:
> - [MinGW-w64](https://www.mingw-w64.org/) (gcc) og [CMake](https://cmake.org/) for native C-avhengigheter (espeak-ng, whisper.cpp, opus, kokoro, onnx)
> - [Windows SDK](https://developer.microsoft.com/en-us/windows/downloads/windows-sdk/) for systembiblioteker (winmm, etc.)
>
> Sørg for at `gcc`, `cmake` er i `PATH`.

## Arkitekturoversikt

<p align="center">
  <img src="../../docs/assets/architecture.png" alt="architecture" />
</p>

Ta det videre: den leverer innebygd støtte for **20+ IM-plattformer**, **stemmedrevne** grensesnitt for naturlig, kontekstbevisst dialog, **nullkonfigurasjonsmodellbytte** med IDE-skanning.

<p align="center">
  <img src="../../docs/assets/providers.png" alt="Supported Providers" />
</p>

## Hvordan bygge

<p align="center">
  <img src="../../docs/assets/handcraft.png" alt="handcraft" />
</p>

> ⚠️ [!IMPORTANT]
>
> Hvis du planlegger å fortsette tuning eller vibe-koding på toppen av Blue, ikke behandle noen få flotte chatter som utgivelsesbevis. Enhver endring som påvirker ruting, utførelsesatferd, verktøyoverflate, budsjettkontroll, modellvalg eller utførelsesrammeverket bør valideres med Blue Harness, ikke med ad hoc-stikkprøver.
>
> Blue bør følge en enkel regel her: data først, porter først, cut over sist. I praksis betyr det å oppdatere det relevante Harness datasettet/evalspesifikasjonen før du bedømmer en endring, og deretter holde én stabil `candidate_id` over hele forsøket, slik at velger-, utførelses-, budsjett- og beredskapsrapporter alle beskriver den samme kandidaten i stedet for fire urelaterte kjøringer.

### Anbefalt Harness Arbeidsflyt

1. Kjør `blue harness selector verify`
2. Kjør `blue harness execution verify`
3. Gjenbruk velgerevalkjøringen for `blue harness budget gate`
4. Avslutt med `blue harness cutover-readiness`

For lokal iterasjon, nattlig validering eller CI-bevisinnsamling, foretrekk `python3 scripts/cutover_candidate_pipeline.py`. Den kjører hele velgeren -> utførelse -> budsjett -> beredskapssekvensen under én delt kandidat, noe som gjør resultatet lettere å sammenligne, gjennomgå og klippe over fra.

### Ekstra rekkverk

| Område | Hva du bør se |
|------|----------------|
| Baseline stabilitet | Hold grunnlinjen, datasettversjonen og `candidate_id` stabil, ellers vil sammenligningen avvike og resultatet vil ikke være pålitelig. |
| Ekte byggeutgang | Gjenoppbygg den berørte binære eller frontend-pakken før du kjører Harness, ellers kan du ende opp med å validere gammel atferd i stedet for den gjeldende endringen. |
| Ruteregistrering | Hvis frontend og backend endres sammen, bekreft at alle nye backend-ruter faktisk er registrert før du bedømmer funksjonen gjennom UI-atferd, fordi manglende registrering ofte ser ut som en logisk feil, men er egentlig en `404`. |
| Frigjør dommen | Et tuningpass er først klart når Harness ikke viser noen meningsfull regresjon og cutover-beredskap bekrefter at kandidaten faktisk er klar til å kutte over. |

Kort sagt, tuning på toppen av Blue handler ikke om "det føles bedre i noen få chatter." Det handler om å sette kandidaten inn i Harness, samle sammenlignbare bevis, og la porten og beredskapsresultatene avgjøre om endringen virkelig er trygg å beholde.

## Funksjoner

| Funksjon | Hva det leverer |
|--------|------------------------|
| Høy tilgjengelig netthenting og nettleserkjøring | En av Blues **skarpeste differensiatorer**. Blue forener **fire nettilgangsbaner** for søk, lesing, ekstraksjon og gjennomgang; beholder **tre reservelag** på tvers av HTTP, proxy-utvinning og nettleserøkter; håndterer **anti-bot-sider** med utfordringsdeteksjon, gjenbruk av informasjonskapsler/økter, stealth og nettleseroverlevering; og ruter på tvers av **tre nettlesermotorer**: `lightpanda`, administrert Chromium og relé/lokal Chrome. |
| Tre-i-ett forskningsløpetid | **Ett offentlig forskningsbidrag** kan rutes til `deep_research`, `analyze` og `ui_review`. Den samme oppdagelsen og bevisstabelen produserer deretter **sitering-først forskning**, **avgrensede rapporter** og **strukturerte brukergrensesnitt/UX/tilgjengelighetsanmeldelser**. |
| Harness-kjøretid, evaluering og evolusjonsrammeverk | Gjør evaluering til en **runtime-primitiv** på tvers av utvikling, opplæring og produksjon. Harness dekker **regresjons- og røyksjekker**, scoring, grunnlinjer, rapporter og kjøretidsvalidering, og fører deretter samme bevis inn i **ferdighetsutvikling**, oppfølgingsevaluering, promotering eller tilbakeføring, og `AGENTS.md` eller gjennomgang av instruksjonsforslag. |
| Multimodal kjøretid med native funksjoner først | Holder **stemme, OCR, PDF, nettleseroppgaver, dokumentkonvertering, strukturert skjemautfylling, mediebehandling og lokal mediegenerering** på **native og lokale stier først**, med **modellruting kun når det faktisk er nødvendig**. |
| Sikkerhet og styring | Inkluderer **utførelse av sandkasse**, **forsvar mot prompt-injeksjon**, **sesjonsrevisjon**, tillatelser, **RBAC**, **WebAuthn**, operative rekkverk og **sikkerhetsskanning for ferdigheter**. |
| LLM Wiki og kunnskapsrom | Gjør minne, forskning og kjøretidsutdata til en **wiki-lignende kunnskapsoverflate** med **oppsummeringssider**, indekser, **tilbakekoblinger**, **friskhet** og **arkivarbeidsflyter**. |
| Ferdighetsbutikk og markedsplass | Sender **innebygd ferdighetsoppdagelse**, kurering, synkronisering og **lokal skanning** slik at utvidbarhet er tilgjengelig **fra dag én**. |
| Produksjonsklar leverandørpool | Gir en ekte leverandørpool med **helsesjekker**, **automatisk failover**, **kretsbrytere** og **leverandørracing** for langvarige arbeidsbelastninger. |
| Innebygd lokal småmodell-kjøretid | Sender en innebygd **`Qwen3.5-0.8B` + `llama.cpp`** kjøretid for **lokale korte spørsmål og svar**, bildegjenkjenning, verktøyruting, oppsummering, **kontekstkomprimering** og **dokumentforbehandling**. |
| Langvarig pålitelighet | Behandler **OTA-oppdateringer**, **sikkerhetskopiering og gjenoppretting**, **varm omlasting av konfigurasjon** og **gjenoppretting etter feil** som **innebygde driftsbehov**. |

## Milepæl Tidslinje

<p align="center">
  <img src="../../docs/assets/timeline.png" alt="Milestone Timeline" />
</p>

| Dato | Versjon | Nøkkelord / funksjoner |
|------|--------|------------------------|
| 26. januar 2026 | `v0.1–v0.9` | Go runtime, plugin-system, nettleserautomatisering |
| 27.–28. januar 2026 | `v0.9.0–v0.9.2` | Nettleseroppgavevisning, Blue Companion, Smart Form Filler |
| 29.–31. januar 2026 | `v0.10.0–v0.10.9` | Claude Code CLI, API Proxy, UI restrukturering |
| 1.–3. februar 2026 | `v0.10.1–v0.10.22` | Beregninger, ekstern tilgang, kontekstbuffer |
| 5.–18. februar 2026 | `v0.10.25–v0.10.29` | i18n, CC Cache, utgivelsesrørledning |
| 20.–25. februar 2026 | `v0.10.28–v0.10.29` | Desktop loader, mobil UX, redesign av minne |
| 28. februar–2. mars 2026 | `v0.10.30` | Deep Research, omstilling av ferdigheter, sikkerhetsskanning |
| 9.–18. mars 2026 | `v0.10.31` | Dashboardoverhaling, VoiceChat refactor, godkjente steder |
| 19–22 mars 2026 | `v0.10.32` | Harness utrulling, transkripsjonsrevisjon, nettsøk |
| 23.–25. mars 2026 | `v0.10.33` | Harness grupper, nettlesergodkjenninger, kompetansemarked |
| 29.–30. mars 2026 | `v0.10.35` | Harness v3, nettleserrelé, kontekstkomprimering |
| 31. mars–1. april 2026 | `v0.10.36` | Transkripsjonsrevisjon, Harness overlegg, verktøyparsing |
| 1. april 2026 | `v0.10.37` | Runtime herding, Skill+Exec cutover, gjenopprettingspolering |
| 2.–5. april 2026 | `v0.10.38` | GitHub støtte, markedsplassforbedring, pålitelighetsforbedringer |
| 6.–7. april 2026 | `v0.10.39` | Forskningssamling, evolusjonsflater, redusert minneforbruk |

## Fellesskap og støtte

- **Problemer**: [Vennligst arkiver feil og funksjonsforespørsler her](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **Diskusjoner**: [Discord](https://discord.gg/zwWbKA4S2)
- **Følg oss** på [GitHub](https://github.com/IceWhaleTech)

[![Star History Chart](https://api.star-history.com/svg?repos=IceWhaleTech/ZimaOS-Blue&type=Date)](https://star-history.com/#IceWhaleTech/ZimaOS-Blue&Date)

## Lisens

Dette prosjektet er lisensiert under MIT-lisensen - se [LISENS](../../LISENS)-filen for detaljer. Vi tror på åpen kildekode og å gi tilbake til samfunnet.

## Bidragsytere

Takk til alle Blue bidragsytere:

<a href="https://community.vaunt.dev/board/IceWhaleTech/repository/ZimaOS-Blue">
  <img src="https://api.vaunt.dev/v1/github/entities/IceWhaleTech/repositories/ZimaOS-Blue/contributors?format=svg&limit=30" width="450" />
</a>

## Referanser

1. **OpenClaw** — Lokal første åpen kildekode-agent. Pioner for å koble LLM-er til lokale enheter gjennom kanaladaptere og verktøyoppringing, direkte inspirerende Blues agent kjøretidsarkitektur. https://github.com/openclaw/openclaw
2. **MiroMind** — Dyp forskningsmodus med evidensstøttet syntese. Formet Blues innebygde dype forskningspipeline: planlegging, parallell henting, bevisdeduplisering og HTML rapportgenerering. https://www.miromind.ai
3. **Karpathy's LLM Wiki** — LLM som kunnskapskompilator. Reframes LLMs for å bygge vedvarende, utviklende kunnskapsrom som beveger seg forbi RAGs akkumuleringsfelle.
4. **OpenSpace (HKUDS)** — Selvutviklende ferdighetsmotor. Et DAG-basert rammeverk der agenter lærer av feil og utleder spesialiserte ferdigheter. https://github.com/HKUDS/OpenSpace
5. **Andrew Ng's Context Hub** — Versjonert API-dokumentasjonsregister for kodingsagenter. Tar opp agenthallusinasjoner og glemt øktkunnskap. Gir kuraterte, versjonerte dokumenter med kommentarer og tilbakemeldingsløkker, og gjør dokumentasjon til et selvforbedrende kunnskapslag. https://github.com/andrewyng/context-hub
6. **Notion** — Enkelt, menneskelig og med vilje stille. Inspirert av den minimalistiske etosen til Notion, bringer Blue varmen tilbake til rutenettet. Der raffinert serif møter gjennomtenkt design, og skaper et rom som føles som hjemme. https://www.notion.com/about
7. **Matrix** — Visuell inspirasjon fra den ikoniske digitale regn-estetikken. Den estetiske retningen for Blues tekniske diagrammer.
8. **IceWhale** — Love, Death & Robots S2E2 "Ice". Et kollektiv som samles over hele verden for å bryte gjennom nettgigantenes vegger og motstå datakonsentrasjon. Ishvalen symboliserer et fellesskap som bygger suverene verktøy sammen ved kanten.
9. **ZimaOS Blue** — Love, Death & Robots S1E14 "Zima Blue". En metafor: intelligens som begynner i tjeneste og utvikler seg for å utforske verden. Blue er en agent for visdom, forankret i enkelhet og strekker seg etter dybde.
10. **ZimaOS** — Forenklede, fokuserte, åpne designprinsipper. Både ZimaOS og Blue deler troen på at teknologien skal tjene brukeren - distribuer på 30 sekunder, kjør hvor som helst, hold leverandørnøytral. https://www.zimaspace.com/zimaos
