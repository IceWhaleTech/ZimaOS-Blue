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

Inspirerade av Clawdbot tror vi att **framtiden** för personlig databehandling kommer att **formas av mångfaldiga, lokalt-först AI-agenter** som körs vid nätverkskanten.

**ZimaOS Blue är vårt svar** — en helt **öppen källkod, granskningsbar och produktionsklar agentkörningsmiljö och verktygslåda** som låter dig leverera privata, självhostade agenter utan friktion.

Byggt för modiga utvecklare som vill **vibba eller handgjort skapa sina egna agenter**, Blue är **konstruerat för prestanda**: skrivet i **Go**, med ett minnesavtryck så lågt som 10 MB. Det körs på **vilken x86, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS som helst** — överallt där du kopplar in ström.

![](../../docs/assets/features.png)

## Höjdpunkter

### Lokalt-först-design och automatisk modelltillgång

Ta det vidare: det levererar inbyggt stöd för **20+ IM-plattformar**, **röststyrda** gränssnitt för naturlig, kontextmedveten dialog, **nollkonfigurations-modellväxling** med IDE-skanning, och SOUL-skiktade personligheter.

<p align="center">
  <img src="../../docs/assets/channels.png" alt="Supported Channels" />
</p>

### Snabbt, lätt

Nativt kompilerat i Go — ingen tolk, ingen VM, ingen overhead. Körs tyst på allt från servrar till dina stationära enheter.

| Mätvärde | ZimaOS Blue (Go) | OpenClaw (Node + dist) |
|----------|-------------------|------------------------|
| `help` kall / varm | **0,18 s / < 0,01 s** | 3,31 s / ~1,11 s |
| `status` körtid (bästa av 3) | **< 0,01 s** | 5,98 s |
| `help` topp-RSS | **~10 MB** | ~394 MB |
| `status` topp-RSS | **~15 MB** | ~1,52 GB |
| Körtidsberoenden | **Inga** | Node.js 18+ |

> Benchmarkat på macOS arm64 (serverläge, utan skrivbords-UI), samma värd, bästa av 3 körningar. Feb 2026.

### Ren Go, vilken enhet som helst

100% Go, statisk binär. **Korskompilerar till 5 mål** direkt ur lådan (![linux](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) amd64/arm64, ![macOS](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) amd64/arm64, ![windows](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) amd64). Ingen Node-körningsmiljö, ingen Python, inga containrar krävs. Lägg den på en NAS, en ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, en gammal x86-router eller en ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)Mac — den bara körs. **Lägg sedan till ditt eget gränssnitt, logik och agentfärdigheter** — en kodbas, varje plattform.

### Säkerhet och styrning

Inbyggd sidovagns-API-proxy med försvar på djupet:
- **Sandlådeexekvering** – Alla verktygsanrop körs i isolerade miljöer.
- **Prompt-injektionsförsvar** – 7+ inbyggda avlyssningsstrategier.
- **Sessionsgranskning** – Fullständig sessionsövervakning, varje interaktion spårbar.
- **RBAC & WebAuthn** – Finkornig åtkomstkontroll med lösenordsfri autentisering.

## Varför Blue

Vi tror att **nästa generations personlig databehandling** omfamnar LLM:er — men **kontrollerbara, granskningsbara** agenter förblir grunden för både individer och team. **Blue levererar**:
- **Omfattande kärna** – Avancerad modellhantering, IM-integration, förbättrad persona och naturliga språkgränssnitt anpassade för daglig interaktion (headset, röst, smarta glasögon).
- **Lokalt-först, ultralätt, plattformsoberoende** – Ingen avancerad hårdvara krävs. Körs på allt som kan beräkna.
- **Säkert och granskningsbart** – Sessionsgranskning, sandlådning, behörighetskontroller och en inbyggd API-proxy som fungerar som en applikationslager-brandvägg — varje byte in/ut är synlig.

![](../../docs/assets/design_principle.png)

Vi minimerar standardkod så att du **fokuserar på det som spelar roll**. Trogen <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> **ZimaOS designfilosofi** levererar Blue:
- **Noll till ett med ett klick** – Distribuera direkt, ingen komplex konfiguration.
- **Snabb prototypning** – Vibba eller handgjort skapa scenariospecifika verktyg, interaktioner och apppaket.
- **Globalt redo** – **Världen är stor**, och den har inte engelska som standard. **20+ språk, inbyggt**, inga barriärer.
- **Öppet modellekosystem** – Ingen leverantörsinlåsning. Ta med dina egna modeller.

<details>
<summary>
<p align="center">
  <img src="../../docs/assets/providers.png" alt="Supported Providers" />
</p>
</summary>

| Leverantör | Modeller | Typ |
|------------|----------|-----|
| OpenAI | GPT-4o, GPT-4, o1, o3 | Moln |
| Anthropic | Claude 4.5, Claude 4 | Moln |
| Google | Gemini 2.5, Gemini 2.0 | Moln |
| Ollama | Llama, Qwen, Gemma, Phi m.fl. | Lokal |
| DeepSeek | DeepSeek-V3, DeepSeek-R1 | Moln |
| Grok | Grok-3, Grok-3-mini | Moln |
| Qwen | Qwen-Max, Qwen-Plus, Qwen-Turbo | Moln |
| GLM | GLM-4, GLM-4-Flash | Moln |
| Moonshot | Moonshot-v1 | Moln |
| MiniMax | abab6.5, abab5.5 | Moln |
| Venice | Llama, Mistral (integritetsfokus) | Moln |
| AWS Bedrock | Claude, Llama, Titan | Moln |
| Azure | OpenAI-modeller via Azure | Moln |
| OpenRouter | 100+ aggregerade modeller | Moln |
| AIHubMix | Flerleverantörsaggregator | Moln |
| Codex | OpenAI Codex | Moln |
| SiliconFlow | DeepSeek, Qwen, Llama via SiliconFlow | Moln |
| Anpassad | Alla OpenAI / Anthropic / Gemini-kompatibla API:er | Moln / Lokal |

</details>

### IDE-stöd

<p align="center">
  <img src="../../docs/assets/ides.png" alt="Supported IDEs" />
</p>

## Snabbstart

### Alternativ 1: Ladda ner skrivbordsappen

Hämta den inbyggda applikationen — inga beroenden, ingen kompilering. Inbyggd provkonfiguration, redo på sekunder — anslut på distans och börja chatta direkt, utan att konfigurera en bot. Verklig igångkörning direkt.

- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)**macOS**: [Ladda ner DMG](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)**Windows**: [Ladda ner installationsprogram](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### Alternativ 2: Installationsskript

![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### Alternativ 3: Bygg från källkod

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

## Arkitekturöversikt

<details>
<summary>
<img src="../../docs/assets/architecture.png" alt="Architecture" />
</summary>

### Paketkarta (`server/internal/`)

| Lager | Paket |
|-------|-------|
| Gateway | bootstrap, server, gateway |
| Proxy | proxy, connection, streaming, resilience |
| Leverantör | providerpool, providers, llm |
| Beskärare | pruner (detector, segmenter, bm25, pipeline, cache) |
| Agent | context, tools, personality, humanizer |
| Minne | memory, embedding, kvstore |
| Kanal | channel, autoreply, i18n |
| Säkerhet | security, auth, permission, rbac, mfa, password, oidc, extauth, sandbox, promptguard, audit |
| Röst | voice, tts, stt, speech |
| Observera | metrics, heartbeat, companion, profiling, leakdetect |
| Plugin | plugin, skill, skillstore |
| Integrera | browser, homeassistant, cron, workflow, formfiller, tunnel, crawler |
| Schemaläggare | scheduler, worker, workerpool, pool |
| Kärna | lifecycle, config, logger, database, cache, ratelimit, retry, timeutil, sync |
| System | sysinfo, cgroup, iotask, watcher, resources, backup, update |
| Flerhyresgäst | tenant, user, session, preview |

</details>

### Dataflöde

**Chattförfrågan (Proxy Hot Path)**
```
Client [Proxy API Key] → Auth Gate → Prompt Guard → Context Pruner (optional)
  → Provider Pool (route:auto/cloud/local) → CC Cache (L1→L2) check
  → Upstream LLM → Response → Cache Store → Metrics Writer → Client (SSE stream)
```

**Kanalmeddelande-flöde**
```
Telegram/Discord/... → Channel Manager → AutoReply check
  → Chat Handler → LLM → Humanizer (MD→text) → Channel → User
```

**Röstpipeline**
```
WebSocket audio → STT (Whisper) → LLM Processing → TTS (eSpeak/Edge) → WebSocket audio
```

**Heartbeat-övervakning**
```
Timer (30 min) → Läs HEARTBEAT.md → LLM-utvärdering → Ta bort HEARTBEAT_OK-token
  → Deduplicering (FNV-hash, 24h TTL) → Kanalvarning (Telegram/Slack/...)
  → Händelseström → UI-indikator
```

## Hur man använder

![](../../docs/assets/handcraft.png)

## Milstolpstidslinje

<details>
<summary>
<img src="../../docs/assets/timeline.png" alt="Milestone Timeline" />
</summary>

| Version | Fokus | Nyckelvärde | Status |
|---------|-------|-------------|--------|
| v0.1 | Go-körningsmiljökärna | Stabil kärna, 24h drift | Done |
| v0.2 | Kärnfunktioner | Minimalt användbar, LLM-integration | Done |
| v0.3 | NAS-integration | NAS-nativ, systemd-stöd | Done |
| v0.4 | Pluginsystem | Utbyggbar, grundläggande säkerhet | Done |
| v0.5 | Produktbaslinje | Produktionsklar, dokumentation | Done |
| v0.6 | Meddelandekanaler | Flerkanalsstöd | Done |
| v0.7 | Säkerhet | OIDC, MFA, granskning | Done |
| v0.8 | Prestanda | Optimering, cachning, benchmarks | Done |
| v0.9 | Ekosystem | Flerhyresgäst, webbläsarautomation, röst | Done |
| v0.10.0 | CLI-paketering | CC CLI-paketering, detektering, automatisk uppdatering | Done |
| v0.10.1 | Mätvärdesövervakning | API-statistik, tokenspårning, TTFT | Done |
| v0.10.2 | CLI-tillförlitlighet | Processlivscykel, felåterställning | Done |
| v0.10.3 | CLI-integration | Installationsguide, leverantörsautodetektering | Done |
| v0.10.4 | Tauri-paketering | Skrivbordsapp, systemfält | Done |
| v0.10.5 | API Proxy Sidecar | Ruttval, promptskydd, användningsstatistik | Done |
| v0.10.6 | Leverantörspool | Flerleverantörsrouting, hälsokontroll, failover | Done |
| v0.10.7 | Förhandsgranskningsläge | Oautentiserad åtkomst, funktionsstyrning | Done |
| v0.10.8 | Färdighetsbutik | Färdighetsbutiksinfrastruktur, kanalvalidering | Done |
| v0.10.9–10 | Användarhantering | Underanvändare, sidnivåbehörigheter | Done |
| v0.10.13–14 | Säkerhet & färdigheter | Säkerhetssida, omdesign av färdighetsbutik | Done |
| v0.10.15 | Chattförbättringar | Chatt-UX, meddelandepipeline | Done |
| v0.10.16 | Talmodul | Sherpa TTS/ASR, eSpeak, leverantörsväxling | Done |
| v0.10.17 | Fjärråtkomst | Ngrok, Cloudflare-tunnlar, ACME-certifikat | Done |
| v0.10.18–20 | Prestandasprint | Start-/chattprestanda, kontextcache | Done |
| v0.10.21–22 | Prompt & DingTalk | Systemprompt, DingTalk-kanal | Done |
| v0.10.23 | OTA-uppdatering | OTA-uppdateringssystem | Done |
| v0.10.24 | Kanaluppgradering | 10 kanaler uppgraderade från stubbar | Done |
| v0.10.25 | CC Cache | Tvånivåcache (L1 minne + L2 disk) | Done |
| v0.10.26 | Humaniserare | Responshumaniseringspipeline | Done |
| v0.10.27 | Kontextbeskärare | 54% tokenbesparing på kod (SWE-bench officiellt), 46–47% på allmänna dokument (lokal IR), BM25-poängsättning, segmentering | Done |
| v0.10.28 | Minnestjänst | Progressiv sökning, dubbelskrivningsbackend | Done |

</details>

## Community och support

- **Ärenden**: [Vänligen rapportera buggar och funktionsförfrågningar här](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **Diskussioner**: [Discord](https://discord.gg/b3AgFDxe9v)
- **Följ oss** på [GitHub](https://github.com/IceWhaleTech)

## Licens

Detta projekt är licensierat under MIT-licensen - se filen [LICENSE](../../LICENSE) för detaljer. Vi tror på öppen källkod och att ge tillbaka till communityn.

## Bidragsgivare

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
