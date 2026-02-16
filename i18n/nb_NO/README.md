![](../../docs/assets/bannerX.png)

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
  <a href="https://discord.gg/SrCYvumF"><img src="../../docs/assets/discord.png" alt="Discord" height="128" /></a>&nbsp;&nbsp;
  <a href="https://www.facebook.com/zimaboard/"><img src="../../docs/assets/facebook.png" alt="Facebook" height="128" /></a>&nbsp;&nbsp;
  <a href="https://x.com/ZimaSpace"><img src="../../docs/assets/x.png" alt="X" height="128" /></a>&nbsp;&nbsp;
  <img src="../../docs/assets/wechat.png" height="128"/>
</p>

## Introduksjon

Inspirert av Clawdbot tror vi at **fremtiden** for personlig databehandling vil bli **formet av mangfoldige, lokalt-først AI-agenter** som kjører på kanten av nettverket.

**ZimaOS Blue er vårt svar** — en fullstendig **åpen kildekode, reviderbar og produksjonsklar agent-kjøretid og verktøysett** som lar deg levere private, selvhostede agenter uten friksjon.

Bygget for modige utviklere som ønsker å **vibe eller håndlage sine egne agenter**. Blue er **konstruert for ytelse**: skrevet i **Go**, med et minneavtrykk helt ned til 10 MB. Den kjører på **alle x86-systemer, Raspberry Pi, Windows, macOS** — overalt hvor du kobler til strøm.

![](../../docs/assets/features.png)

## Høydepunkter

### Lokalt-først-design og automatisk modelltilgang

Ta det videre: innebygd støtte for **20+ IM-plattformer**, **stemmedrevne** grensesnitt for naturlig, kontekstbevisst dialog, **nullkonfigurasjon modellbytte** med IDE-skanning, og SOUL-lagdelte personligheter.

### Rask og lett

Kompilert direkte i Go — ingen tolk, ingen VM, ingen overhead. Kjører stille på alt fra servere til stasjonære enheter.

| Metrikk | ZimaOS Blue (Go) | OpenClaw (Node + dist) |
|---------|-------------------|------------------------|
| `--help` kald / varm | **0.18 s / < 0.01 s** | 3.31 s / ~1.11 s |
| `status` kjøretid (beste av 3) | **< 0.01 s** | 5.98 s |
| `--help` topp-RSS | **~10 MB** | ~394 MB |
| `status` topp-RSS | **~15 MB** | ~1.52 GB |
| Kjøretidsavhengigheter | **Ingen** | Node.js 18+ |

> Benchmarket på macOS arm64, samme vert, beste av 3 kjøringer. Feb 2026.

### Ren Go, alle enheter

100 % Go, statisk binærfil. **Krysskompilerer til 5 mål** rett ut av boksen (![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) amd64/arm64, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) amd64/arm64, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) amd64). Ingen Node-kjøretid, ingen Python, ingen containere nødvendig. Slipp den på en NAS, en Raspberry Pi, en gammel x86-ruter eller en Mac — den bare kjører. **Legg deretter på din egen UI, logikk og agent-ferdigheter** — én kodebase, alle plattformer.

### Sikkerhet og styring

Innebygd sidecar API-proxy med forsvar i dybden:
- **Sandkassekjøring** – Alle verktøykall kjører i isolerte miljøer.
- **Prompt-injeksjonsforsvar** – 7+ innebygde avskjæringsstrategier.
- **Sesjonsrevisjon** – Full sesjonsovervåking, hver interaksjon sporbar.
- **RBAC og WebAuthn** – Finkornet tilgangskontroll med passordløs autentisering.

## Hvorfor Blue

Vi tror at **neste generasjons personlig databehandling** omfavner LLM-er — men **kontrollerbare, reviderbare** agenter forblir grunnfjellet for både enkeltpersoner og team. **Blue leverer**:
- **Omfattende kjerne** – Avansert modellhåndtering, IM-integrasjon, forbedret persona og naturligspråklige grensesnitt tilpasset daglige interaksjoner (headset, stemme, smartbriller).
- **Lokalt-først, ultralettevekt, på tvers av enheter** – Ingen kraftig maskinvare nødvendig. Kjører på alt som kan beregne.
- **Sikkert og reviderbart** – Sesjonsrevisjon, sandkassing, tillatelseskontroller og en innebygd API-proxy som fungerer som en applikasjonslagsbrannmur — hver byte inn/ut er synlig.

Vi minimerer standardkode slik at du kan **fokusere på det som betyr noe**. Tro mot **ZimaOS sin designfilosofi** leverer Blue:
- **Fra null til én med ett klikk** – Distribuer umiddelbart, ingen kompleks konfigurasjon.
- **Rask prototyping** – Vibe eller håndlag scenariospesifikke verktøy, interaksjoner og app-pakker.
- **Globalt klar** – **Verden er stor**, og den snakker ikke engelsk som standard. **20+ språk, innebygd**, ingen barrierer.
- **Åpent modell-økosystem** – Ingen leverandørlåsing. Ta med dine egne modeller.

![](../../docs/assets/design_principle.png)

## Hurtigstart

### Alternativ 1: Last ned skrivebordsappen (macOS og Windows)

Hent den native applikasjonen — ingen avhengigheter, ingen kompilering.

- **macOS**: [Last ned DMG](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- **Windows**: [Last ned installasjonsprogram](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### Alternativ 2: Installasjonsskript

**macOS / Linux**
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### Alternativ 3: Bygg fra kildekode

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue
./dev.sh
```

## Arkitekturoversikt

![](../../docs/assets/architecture.png)

### Dataflyt

**Chat-forespørsel (Proxy Hot Path)**
```
Client [Proxy API Key] → Auth Gate → Prompt Guard → Context Pruner (optional)
  → Provider Pool (route:auto/cloud/local) → CC Cache (L1→L2) check
  → Upstream LLM → Response → Cache Store → Metrics Writer → Client (SSE stream)
```

**Kanalmeldingsflyt**
```
Telegram/Discord/... → Channel Manager → AutoReply check
  → (no match) → Chat Handler → LLM → Humanizer (MD→text) → Channel → User
```

**Talepipeline**
```
WebSocket audio → STT (Whisper) → LLM Processing → TTS (eSpeak/Edge) → WebSocket audio
```

### Pakkeoversikt (`server/internal/`)

| Lag | Pakker |
|-----|--------|
| Gateway | bootstrap, server, gateway |
| Proxy | proxy, connection, streaming, resilience |
| Leverandør | providerpool, providers, llm |
| Pruner | pruner (detector, segmenter, bm25, pipeline, cache) |
| Agent | context, tools, personality, humanizer |
| Minne | memory, embedding, kvstore |
| Kanal | channel, autoreply, i18n |
| Sikkerhet | security, auth, permission, rbac, mfa, password, oidc, extauth, sandbox, promptguard, audit |
| Tale | voice, tts, stt, speech |
| Observasjon | metrics, heartbeat, companion, profiling, leakdetect |
| Plugin | plugin, skill, skillstore |
| Integrasjon | browser, homeassistant, cron, workflow, formfiller, tunnel, crawler |
| Planlegger | scheduler, worker, workerpool, pool |
| Kjerne | lifecycle, config, logger, database, cache, ratelimit, retry, timeutil, sync |
| System | sysinfo, cgroup, iotask, watcher, resources, backup, update |
| Flerleietaker | tenant, user, session, preview |

## Slik bruker du det

![](../../docs/assets/handcraft.png)

## Milepælstidslinje

![](../../docs/assets/timeline.png)

| Versjon | Fokus | Nøkkelverdi | Status |
|---------|-------|-------------|--------|
| v0.1 | Go-kjøretidskjerne | Stabil kjerne, 24t drift | Done |
| v0.2 | Kjernefunksjoner | Minimalt brukbar, LLM-integrasjon | Done |
| v0.3 | NAS-integrasjon | NAS-nativ, systemd-støtte | Done |
| v0.4 | Plugin-system | Utvidbart, sikkerhetsgrunnlag | Done |
| v0.5 | Produktgrunnlinje | Produksjonsklar, dokumentasjon | Done |
| v0.6 | Meldingskanaler | Flerkanalstøtte | Done |
| v0.7 | Sikkerhet | OIDC, MFA, revisjon | Done |
| v0.8 | Ytelse | Optimalisering, hurtigbuffer, benchmarks | Done |
| v0.9 | Økosystem | Flerleietaker, nettleserautomatisering, tale | Done |
| v0.10.0 | CLI-bunting | CC CLI-bunting, deteksjon, auto-oppdatering | Done |
| v0.10.1 | Metrikkoverv. | API-statistikk, token-sporing, TTFT | Done |
| v0.10.2 | CLI-pålitelighet | Prosesslivssyklus, feilgjenoppretting | Done |
| v0.10.3 | CLI-integrasjon | Oppsettveiviser, leverandør-autodeteksjon | Done |
| v0.10.4 | Tauri-pakking | Skrivebordsapp, systemstatusfeltet | Done |
| v0.10.5 | API-proxy-sidecar | Rutevalg, prompt guard, bruksstatistikk | Done |
| v0.10.6 | Leverandørpool | Fler-leverandør-ruting, helsesjekk, failover | Done |
| v0.10.7 | Forhåndsvisningsmodus | Uautentisert tilgang, funksjonsporting | Done |
| v0.10.8 | Skill Store | Skill store-infrastruktur, kanalvalidering | Done |
| v0.10.9–10 | Brukeradministrasjon | Underbrukere, sidenivåtillatelser | Done |
| v0.10.13–14 | Sikkerhet og ferdigheter | Sikkerhetsside, skill store-redesign | Done |
| v0.10.15 | Chat-forbedringer | Chat-UX, meldingspipeline | Done |
| v0.10.16 | Talemodul | Sherpa TTS/ASR, eSpeak, leverandørbytte | Done |
| v0.10.17 | Fjerntilgang | Ngrok, Cloudflare-tunneler, ACME-sertifikater | Done |
| v0.10.18–20 | Ytelsessprint | Oppstart-/chat-ytelse, kontekstbuffer | Done |
| v0.10.21–22 | Prompt og DingTalk | Systemprompt, DingTalk-kanal | Done |
| v0.10.23 | OTA-oppdatering | OTA-oppdateringssystem | Done |
| v0.10.24 | Kanaloppgradering | 10 kanaler oppgradert fra stubber | Done |
| v0.10.25 | CC Cache | Tonivåbuffer (L1 minne + L2 disk) | Done |
| v0.10.26 | Humanizer | Responshumaniseringspipeline | Done |
| v0.10.27 | Context Pruner | BM25-scoring, segmentering, benchmarks | Done |
| v0.10.28 | Memory Service | Progressivt søk, dual-write-backend | Done |

## Fellesskap og støtte

- **Issues**: [Vennligst rapporter feil og funksjonsønsker her](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **Diskusjoner**: [Discord](https://discord.gg/SrCYvumF)
- **Følg oss** på [GitHub](https://github.com/IceWhaleTech)

## Lisens

Dette prosjektet er lisensiert under MIT-lisensen — se [LICENSE](../../LICENSE)-filen for detaljer. Vi tror på åpen kildekode og på å gi tilbake til fellesskapet.

## Bidragsytere

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
