![](../../docs/assets/bannerX.png)

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

Inspireret af Clawdbot tror vi, at **fremtiden** for personlig computing vil blive **formet af mangfoldige, lokalt-først AI-agenter**, der kører i kanten af netværket.

**ZimaOS Blue er vores svar** — en fuldt **open source, reviderbar og produktionsklar agent-runtime og værktøjskasse**, der lader dig levere private, selvhostede agenter uden friktion.

Bygget til modige udviklere, der vil **vibe eller håndbygge deres egne agenter**, er Blue **konstrueret til ydeevne**: skrevet i **Go** med et hukommelsesforbrug helt ned til 10 MB. Det kører på **enhver x86, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS** — overalt hvor du sætter strøm til.

![](../../docs/assets/features.png)

## Højdepunkter

### Lokalt-først-design og automatisk modeladgang

Tag det videre: det leverer indbygget understøttelse af **20+ IM-platforme**, **stemmestyrede** grænseflader til naturlig, kontekstbevidst dialog, **nulkonfigurations-modelskift** med IDE-scanning og SOUL-lagdelte personligheder.

<p align="center">
  <img src="../../docs/assets/channels.png" alt="Supported Channels" />
</p>

### Hurtigt, let

Kompileret direkte i Go — ingen fortolker, ingen VM, ingen overhead. Kører lydløst på alt fra servere til dine stationære enheder.

| Målepunkt | ZimaOS Blue (Go) | Reference Agent (Node + dist) |
|-----------|-------------------|------------------------|
| `help` kold / varm | **0,18 s / < 0,01 s** | 3,31 s / ~1,11 s |
| `status` køretid (bedste af 3) | **< 0,01 s** | 5,98 s |
| `help` top-RSS | **~10 MB** | ~394 MB |
| `status` top-RSS | **~15 MB** | ~1,52 GB |
| Køretidsafhængigheder | **Ingen** | Node.js 18+ |

> Benchmarket på macOS arm64 (servertilstand, uden desktop-UI), samme vært, bedste af 3 kørsler. Feb 2026.

### Ren Go, enhver enhed

100% Go, statisk binær. **Krydskompilerer til 5 mål** direkte fra kassen (![linux](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) amd64/arm64, ![macOS](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) amd64/arm64, ![windows](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) amd64). Ingen Node-runtime, ingen Python, ingen containere påkrævet. Læg den på en NAS, en ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, en gammel x86-router eller en ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)Mac — den kører bare. **Tilføj derefter din egen brugerflade, logik og agentfærdigheder** — én kodebase, alle platforme.

### Sikkerhed og styring

Indbygget sidecar API-proxy med forsvar i dybden:
- **Sandkasseeksekvering** – Alle værktøjskald kører i isolerede miljøer.
- **Prompt-injektionsforsvar** – 7+ indbyggede afskæringsstrategier.
- **Sessionsrevision** – Fuld sessionsovervågning, enhver interaktion sporbar.
- **RBAC & WebAuthn** – Finkornet adgangskontrol med adgangskodefri autentificering.

## Hvorfor Blue

Vi tror, at **næste generations personlig computing** omfavner LLM'er — men **kontrollerbare, reviderbare** agenter forbliver fundamentet for både enkeltpersoner og teams. **Blue leverer**:
- **Omfattende kerne** – Avanceret modelstyring, IM-integration, forbedret persona og naturlige sproggrænseflader tilpasset daglig interaktion (headsets, stemme, smartbriller).
- **Lokalt-først, ultralet, på tværs af enheder** – Ingen avanceret hardware påkrævet. Kører på alt, der kan beregne.
- **Sikkert og reviderbart** – Sessionsrevision, sandkassekørsel, rettighedskontroller og en indbygget API-proxy, der fungerer som en applikationslags-firewall — hver byte ind/ud er synlig.

![](../../docs/assets/design_principle.png)

Vi minimerer standardkode, så du **fokuserer på det, der betyder noget**. Tro mod <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> **ZimaOS' designfilosofi** leverer Blue:
- **Fra nul til ét med ét klik** – Udrulning med det samme, ingen kompleks konfiguration.
- **Hurtig prototyping** – Vibe eller håndbyg scenariespecifikke værktøjer, interaktioner og apppakker.
- **Globalt klar** – **Verden er stor**, og den har ikke engelsk som standard. **20+ sprog, indbygget**, ingen barrierer.
- **Åbent modeløkosystem** – Ingen leverandørlåsning. Medbring dine egne modeller.

<details>
<summary>
<p align="center">
  <img src="../../docs/assets/providers.png" alt="Supported Providers" />
</p>
</summary>

| Udbyder | Modeller | Type |
|---------|----------|------|
| OpenAI | GPT-4o, GPT-4, o1, o3 | Cloud |
| Anthropic | Claude 4.5, Claude 4 | Cloud |
| Google | Gemini 2.5, Gemini 2.0 | Cloud |
| Ollama | Llama, Qwen, Gemma, Phi m.fl. | Lokal |
| DeepSeek | DeepSeek-V3, DeepSeek-R1 | Cloud |
| Grok | Grok-3, Grok-3-mini | Cloud |
| Qwen | Qwen-Max, Qwen-Plus, Qwen-Turbo | Cloud |
| GLM | GLM-4, GLM-4-Flash | Cloud |
| Moonshot | Moonshot-v1 | Cloud |
| MiniMax | abab6.5, abab5.5 | Cloud |
| Venice | Llama, Mistral (privatlivsfokus) | Cloud |
| AWS Bedrock | Claude, Llama, Titan | Cloud |
| Azure | OpenAI-modeller via Azure | Cloud |
| OpenRouter | 100+ aggregerede modeller | Cloud |
| AIHubMix | Multiudbyder-aggregator | Cloud |
| Codex | OpenAI Codex | Cloud |
| SiliconFlow | DeepSeek, Qwen, Llama via SiliconFlow | Cloud |
| Brugerdefineret | Alle OpenAI / Anthropic / Gemini-kompatible API'er | Cloud / Lokal |

</details>

### Understøttede IDE'er

<p align="center">
  <img src="../../docs/assets/ides.png" alt="Supported IDEs" />
</p>

## Hurtig start

### Mulighed 1: Download skrivebordsappen

Hent den native applikation — ingen afhængigheder, ingen kompilering. Indbygget prøvekonfiguration, klar på sekunder — opret fjernforbindelse og begynd at chatte med det samme, uden at konfigurere en bot. Ægte kørsel fra start.

- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)**macOS**: [Download DMG](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)**Windows**: [Download installationsprogram](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### Mulighed 2: Installationsscript

![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### Mulighed 3: Byg fra kildekode

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue
```

![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
sh build.sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
.\build.bat
```

> **Note:** Windows builds require [MinGW-w64](https://www.mingw-w64.org/) (gcc) and [CMake](https://cmake.org/) for native C dependencies (espeak-ng, whisper.cpp, opus). Make sure `gcc` and `cmake` are in your `PATH`.

## Arkitekturoversigt

<details>
<summary>
<img src="../../docs/assets/architecture.png" alt="Architecture" />
</summary>

### Pakkeoversigt (`server/internal/`)

| Lag | Pakker |
|-----|--------|
| Gateway | bootstrap, server, gateway |
| Proxy | proxy, connection, streaming, resilience |
| Udbyder | providerpool, providers, llm |
| Beskærer | pruner (detector, segmenter, bm25, pipeline, cache) |
| Agent | context, tools, personality, humanizer |
| Hukommelse | memory, embedding, kvstore |
| Kanal | channel, autoreply, i18n |
| Sikkerhed | security, auth, permission, rbac, mfa, password, oidc, extauth, sandbox, promptguard, audit |
| Stemme | voice, tts, stt, speech |
| Overvågning | metrics, heartbeat, companion, profiling, leakdetect |
| Plugin | plugin, skill, skillstore |
| Integration | browser, cron, workflow, formfiller, tunnel, crawler |
| Planlægger | scheduler, worker, workerpool, pool |
| Kerne | lifecycle, config, logger, database, cache, ratelimit, retry, timeutil, sync |
| System | sysinfo, cgroup, iotask, watcher, resources, backup, update |
| Multilejemål | tenant, user, session, preview |

</details>

### Dataflow

**Chatanmodning (Proxy Hot Path)**
```
Client [Proxy API Key] → Auth Gate → Prompt Guard → Context Pruner (optional)
  → Provider Pool (route:auto/cloud/local) → CC Cache (L1→L2) check
  → Upstream LLM → Response → Cache Store → Metrics Writer → Client (SSE stream)
```

**Kanalbeskedflow**
```
Telegram/Discord/... → Channel Manager → AutoReply check
  → Chat Handler → LLM → Humanizer (MD→text) → Channel → User
```

**Stemmepipeline**
```
WebSocket audio → STT (Whisper) → LLM Processing → TTS (eSpeak/Edge) → WebSocket audio
```

**Heartbeat-overvågning**
```
Timer (30 min) → Læs HEARTBEAT.md → LLM-evaluering → Fjern HEARTBEAT_OK-token
  → Deduplikering (FNV-hash, 24t TTL) → Kanalbesked (Telegram/Slack/...)
  → Hændelsesstrøm → UI-indikator
```

## Sådan bruges det

![](../../docs/assets/handcraft.png)

## Milepælstidslinje

<details>
<summary>
<img src="../../docs/assets/timeline.png" alt="Milestone Timeline" />
</summary>

| Version | Fokus | Nøgleværdi | Status |
|---------|-------|------------|--------|
| v0.1 | Go Runtime-kerne | Stabil kerne, 24t drift | Done |
| v0.2 | Kernefunktioner | Minimalt brugbar, LLM-integration | Done |
| v0.3 | NAS-integration | NAS-nativ, systemd-understøttelse | Done |
| v0.4 | Pluginsystem | Udvidelsesbar, grundlæggende sikkerhed | Done |
| v0.5 | Produktbasislinje | Produktionsklar, dokumentation | Done |
| v0.6 | Beskedkanaler | Flerkanalunderstøttelse | Done |
| v0.7 | Sikkerhed | OIDC, MFA, revision | Done |
| v0.8 | Ydeevne | Optimering, caching, benchmarks | Done |
| v0.9 | Økosystem | Multilejemål, browserautomatisering, stemme | Done |
| v0.10.0 | CLI-bundling | CC CLI-bundling, detektion, automatisk opdatering | Done |
| v0.10.1 | Metrikovervågning | API-statistik, tokensporing, TTFT | Done |
| v0.10.2 | CLI-pålidelighed | Proceslivscyklus, fejlgenopretning | Done |
| v0.10.3 | CLI-integration | Opsætningsguide, udbyder-autodetektion | Done |
| v0.10.4 | Tauri-pakning | Skrivebordsapp, systembakke | Done |
| v0.10.5 | API Proxy Sidecar | Rutevalg, promptbeskyttelse, brugsstatistik | Done |
| v0.10.6 | Udbydergruppe | Multiudbyder-routing, sundhedstjek, failover | Done |
| v0.10.7 | Forhåndsvisningstilstand | Uautentificeret adgang, funktionsstyring | Done |
| v0.10.8 | Færdighedsbutik | Færdighedsbutiksinfrastruktur, kanalvalidering | Done |
| v0.10.9–10 | Brugerstyring | Underbrugere, sideniveaurettigheder | Done |
| v0.10.13–14 | Sikkerhed & færdigheder | Sikkerhedsside, redesign af færdighedsbutik | Done |
| v0.10.15 | Chatforbedringer | Chat-UX, beskedpipeline | Done |
| v0.10.16 | Talemodul | Sherpa TTS/ASR, eSpeak, udbydersskift | Done |
| v0.10.17 | Fjernadgang | Ngrok, Cloudflare-tunneler, ACME-certifikater | Done |
| v0.10.18–20 | Ydeevnesprint | Opstart-/chatydeevne, kontekstcache | Done |
| v0.10.21–22 | Prompt & DingTalk | Systemprompt, DingTalk-kanal | Done |
| v0.10.23 | OTA-opdatering | OTA-opdateringssystem | Done |
| v0.10.24 | Kanalopgradering | 10 kanaler opgraderet fra stubs | Done |
| v0.10.25 | CC Cache | Toniveaucache (L1 hukommelse + L2 disk) | Done |
| v0.10.26 | Humanisering | Responshumaniseringspipeline | Done |
| v0.10.27 | Kontekstbeskærer | 54% tokenbesparelse på kode (SWE-bench officielt), 46–47% på generelle dokumenter (lokal IR), BM25-scoring, segmentering | Done |
| v0.10.28 | Hukommelsestjeneste | Progressiv søgning, dobbeltskrivningsbackend | Done |

</details>

## Fællesskab og support

- **Fejlrapporter**: [Indsend venligst fejl og funktionsønsker her](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **Diskussioner**: [Discord](https://discord.gg/b3AgFDxe9v)
- **Følg os** på [GitHub](https://github.com/IceWhaleTech)

## Licens

Dette projekt er licenseret under MIT-licensen - se filen [LICENSE](../../LICENSE) for detaljer. Vi tror på open source og på at give tilbage til fællesskabet.

## Bidragydere

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
