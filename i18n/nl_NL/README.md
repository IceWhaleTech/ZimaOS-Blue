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
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT License"></a>
</p>

<p align="center">
  <a href="https://discord.gg/SrCYvumF"><img src="../../docs/assets/discord.png" alt="Discord" height="128" /></a>&nbsp;&nbsp;
  <a href="https://www.facebook.com/zimaboard/"><img src="../../docs/assets/facebook.png" alt="Facebook" height="128" /></a>&nbsp;&nbsp;
  <a href="https://x.com/ZimaSpace"><img src="../../docs/assets/x.png" alt="X" height="128" /></a>
</p>

## Introductie

Geïnspireerd door Clawdbot geloven wij dat de **toekomst** van persoonlijk computergebruik zal worden **gevormd door diverse, local-first AI-agents** die aan de edge draaien.

**ZimaOS Blue is ons antwoord** — een volledig **open-source, auditeerbare en productieklare agent-runtime en toolkit** waarmee je privé, zelf-gehoste agents kunt uitrollen zonder enige frictie.

Gebouwd voor gedurfde ontwikkelaars die hun **eigen agents willen viben of handmatig bouwen**, is Blue **ontworpen voor prestaties**: geschreven in **Go**, met een geheugengebruik van slechts 10 MB. Het draait op **elke x86, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS** — overal waar je stroom hebt.

![](../../docs/assets/features.png)

## Hoogtepunten

### Local-First Ontwerp & Automatische Modeltoegang

Ga verder: het biedt native ondersteuning voor **20+ IM-platformen**, **spraakgestuurde** interfaces voor natuurlijke, contextbewuste dialoog, **zero-config modelwisseling** met IDE-scanning, en SOUL-gelaagde persoonlijkheden.

<p align="center">
  <img src="../../docs/assets/providers.png" alt="Supported Providers" />
</p>

### Snel, Licht

Native gecompileerd in Go — geen interpreter, geen VM, geen overhead. Draait geruisloos op alles, van servers tot je desktopapparaten.

| Metriek | ZimaOS Blue (Go) | OpenClaw (Node + dist) |
|---------|-------------------|------------------------|
| `--help` koud / warm | **0.18 s / < 0.01 s** | 3.31 s / ~1.11 s |
| `status` runtime (beste van 3) | **< 0.01 s** | 5.98 s |
| `--help` piek RSS | **~10 MB** | ~394 MB |
| `status` piek RSS | **~15 MB** | ~1.52 GB |
| Runtime-afhankelijkheden | **Geen** | Node.js 18+ |

> Benchmark op macOS arm64 (servermodus, geen desktop-UI), dezelfde host, beste van 3 runs. Feb 2026.

### Pure Go, Elk Apparaat

100% Go, statisch binair bestand. **Cross-compileert naar 5 doelplatformen** out of the box (![linux](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) amd64/arm64, ![macOS](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) amd64/arm64, ![windows](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) amd64). Geen Node-runtime, geen Python, geen containers nodig. Zet het op een NAS, een ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, een oude x86-router of een ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)Mac — het werkt gewoon. **Bouw er vervolgens je eigen UI, logica en agent-vaardigheden bovenop** — één codebase, elk platform.

### Beveiliging & Governance

Ingebouwde sidecar API-proxy met diepgaande verdediging:
- **Sandbox-uitvoering** – Alle tool-aanroepen draaien in geïsoleerde omgevingen.
- **Prompt Injection-verdediging** – 7+ ingebouwde onderscheppingsstrategieën.
- **Sessie-auditing** – Volledige sessiemonitoring, elke interactie traceerbaar.
- **RBAC & WebAuthn** – Fijnmazige toegangscontrole met wachtwoordloze authenticatie.

## Waarom Blue

Wij geloven dat **next-gen persoonlijk computergebruik** LLM's omarmt — maar **controleerbare, auditeerbare** agents blijven het fundament voor zowel individuen als teams. **Blue levert**:
- **Uitgebreide Kern** – Geavanceerd modelbeheer, IM-integratie, verbeterde persona en natuurlijke taalinterfaces afgestemd op dagelijkse interacties (headsets, spraak, slimme brillen).
- **Local-First, Ultralicht, Cross-Device** – Geen high-end hardware vereist. Draait op alles wat kan rekenen.
- **Veilig & Auditeerbaar** – Sessie-auditing, sandboxing, permissiecontroles en een ingebouwde API-proxy die fungeert als applicatielaag-firewall — elke byte in/uit is zichtbaar.

We minimaliseren boilerplate zodat jij je **kunt focussen op wat ertoe doet**. Trouw aan de <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> **ontwerpfilosofie van ZimaOS** levert Blue:
- **Van Nul naar Eén in Één Klik** – Direct uitrollen, geen complexe configuratie.
- **Snel Prototypen** – Vibe of bouw handmatig scenariospecifieke tools, interacties en app-pakketten.
- **Wereldwijd Klaar** – **De wereld is groot**, en Engels is niet de standaard. **20+ talen, native**, geen barrières.
- **Open Model-ecosysteem** – Geen vendor lock-in. Breng je eigen modellen mee.

![](../../docs/assets/design_principle.png)

## Snel Starten

### Optie 1: Download Desktop-app

Download de native applicatie — geen afhankelijkheden, geen compilatie. Ingebouwde proefconfiguratie, direct aan de slag — begin met chatten via een externe verbinding, zonder bot-configuratie. Echt plug-and-play.

- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)**macOS**: [Download DMG](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)**Windows**: [Download Installer](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### Optie 2: Installatiescript

![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### Optie 3: Bouwen vanuit Broncode

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

> **Opmerking:** Windows-builds vereisen [MinGW-w64](https://www.mingw-w64.org/) (gcc) en [CMake](https://cmake.org/) voor native C-afhankelijkheden (espeak-ng, whisper.cpp, opus). Zorg ervoor dat `gcc` en `cmake` in uw `PATH` staan.

## Architectuuroverzicht

<details>
<summary>
<img src="../../docs/assets/architecture.png" alt="Architecture" />
</summary>

### Pakketoverzicht (`server/internal/`)

| Laag | Pakketten |
|------|-----------|
| Gateway | bootstrap, server, gateway |
| Proxy | proxy, connection, streaming, resilience |
| Provider | providerpool, providers, llm |
| Pruner | pruner (detector, segmenter, bm25, pipeline, cache) |
| Agent | context, tools, personality, humanizer |
| Memory | memory, embedding, kvstore |
| Channel | channel, autoreply, i18n |
| Security | security, auth, permission, rbac, mfa, password, oidc, extauth, sandbox, promptguard, audit |
| Voice | voice, tts, stt, speech |
| Observe | metrics, heartbeat, companion, profiling, leakdetect |
| Plugin | plugin, skill, skillstore |
| Integrate | browser, homeassistant, cron, workflow, formfiller, tunnel, crawler |
| Scheduler | scheduler, worker, workerpool, pool |
| Core | lifecycle, config, logger, database, cache, ratelimit, retry, timeutil, sync |
| System | sysinfo, cgroup, iotask, watcher, resources, backup, update |
| Multi-tenant | tenant, user, session, preview |

</details>

### Gegevensstroom

**Chatverzoek (Proxy Hot Path)**
```
Client [Proxy API Key] → Auth Gate → Prompt Guard → Context Pruner (optional)
  → Provider Pool (route:auto/cloud/local) → CC Cache (L1→L2) check
  → Upstream LLM → Response → Cache Store → Metrics Writer → Client (SSE stream)
```

**Kanaalberichtstroom**
```
Telegram/Discord/... → Channel Manager → AutoReply check
  → Chat Handler → LLM → Humanizer (MD→text) → Channel → User
```

**Spraakpijplijn**
```
WebSocket audio → STT (Whisper) → LLM Processing → TTS (eSpeak/Edge) → WebSocket audio
```

## Gebruik

![](../../docs/assets/handcraft.png)

## Mijlpalen Tijdlijn

<details>
<summary>
<img src="../../docs/assets/timeline.png" alt="Milestone Timeline" />
</summary>

| Versie | Focus | Kernwaarde | Status |
|--------|-------|------------|--------|
| v0.1 | Go Runtime-kern | Stabiele kernel, 24u draaiend | Voltooid |
| v0.2 | Kernfunctionaliteit | Minimaal bruikbaar, LLM-integratie | Voltooid |
| v0.3 | NAS-integratie | NAS-native, systemd-ondersteuning | Voltooid |
| v0.4 | Pluginsysteem | Uitbreidbaar, beveiligingsbasis | Voltooid |
| v0.5 | Productbasislijn | Productieklaar, documentatie | Voltooid |
| v0.6 | Berichtkanalen | Multi-kanaal ondersteuning | Voltooid |
| v0.7 | Beveiliging | OIDC, MFA, audit | Voltooid |
| v0.8 | Prestaties | Optimalisatie, caching, benchmarks | Voltooid |
| v0.9 | Ecosysteem | Multi-tenant, browserautomatisering, spraak | Voltooid |
| v0.10.0 | CLI-bundeling | CC CLI-bundeling, detectie, auto-update | Voltooid |
| v0.10.1 | Metriekenmonitoring | API-statistieken, tokentracking, TTFT | Voltooid |
| v0.10.2 | CLI-betrouwbaarheid | Proceslevenscyclus, foutherstel | Voltooid |
| v0.10.3 | CLI-integratie | Installatiewizard, provider-autodetectie | Voltooid |
| v0.10.4 | Tauri-verpakking | Desktop-app, systeemvak | Voltooid |
| v0.10.5 | API Proxy Sidecar | Routeselectie, prompt guard, gebruiksstatistieken | Voltooid |
| v0.10.6 | Provider Pool | Multi-provider routing, health check, failover | Voltooid |
| v0.10.7 | Voorbeeldmodus | Ongeauthenticeerde toegang, feature gating | Voltooid |
| v0.10.8 | Skill Store | Skill store-infrastructuur, kanaalvalidatie | Voltooid |
| v0.10.9–10 | Gebruikersbeheer | Subgebruikers, paginaniveau-permissies | Voltooid |
| v0.10.13–14 | Beveiliging & Vaardigheden | Beveiligingspagina, skill store-herontwerp | Voltooid |
| v0.10.15 | Chatverbeteringen | Chat-UX, berichtpijplijn | Voltooid |
| v0.10.16 | Spraakmodule | Sherpa TTS/ASR, eSpeak, providerwisseling | Voltooid |
| v0.10.17 | Externe Toegang | Ngrok, Cloudflare-tunnels, ACME-certificaten | Voltooid |
| v0.10.18–20 | Prestatiesprint | Opstart-/chatprestaties, contextcache | Voltooid |
| v0.10.21–22 | Prompt & DingTalk | Systeemprompt, DingTalk-kanaal | Voltooid |
| v0.10.23 | OTA-update | OTA-updatesysteem | Voltooid |
| v0.10.24 | Kanaalupgrade | 10 kanalen geüpgraded van stubs | Voltooid |
| v0.10.25 | CC Cache | Tweelaagsecache (L1 geheugen + L2 schijf) | Voltooid |
| v0.10.26 | Humanizer | Responshumaniseringspijplijn | Voltooid |
| v0.10.27 | Context Pruner | 54% tokenbesparing op code (SWE-bench officieel), 46–47% op algemene documenten (lokale IR), BM25-scoring, segmentatie | Voltooid |
| v0.10.28 | Geheugenservice | Progressief zoeken, dual-write backend | Voltooid |

</details>

## Community & Ondersteuning

- **Issues**: [Meld bugs en functieverzoeken hier](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **Discussies**: [Discord](https://discord.gg/SrCYvumF)
- **Volg ons** op [GitHub](https://github.com/IceWhaleTech)

## Licentie

Dit project is gelicentieerd onder de MIT-licentie - zie het [LICENSE](../../LICENSE)-bestand voor details. Wij geloven in open source en het teruggeven aan de community.

## Bijdragers

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
