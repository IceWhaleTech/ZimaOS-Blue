![](../../docs/assets/bannerX.png)

<p align="center">
  <a href="../../README.md">English</a> |
  <a href="../ca_ES/README.md">Català</a> |
  <strong>Čeština</strong> |
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
  <a href="../sv_SE/README.md">Svenska</a> |
  <a href="../zh_CN/README.md">简体中文</a> |
  <a href="../zh_TW/README.md">繁體中文</a>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Blue/ci.yml?branch=main&style=for-the-badge" alt="Stav CI"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Blue?include_prereleases&style=for-the-badge" alt="Vydání na GitHubu"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="Licence MIT"></a>
</p>

<p align="center">
  <a href="https://discord.gg/b3AgFDxe9v"><img src="../../docs/assets/discord.png" alt="Discord" height="128" /></a>&nbsp;&nbsp;
  <a href="https://www.facebook.com/zimaboard/"><img src="../../docs/assets/facebook.png" alt="Facebook" height="128" /></a>&nbsp;&nbsp;
  <a href="https://x.com/ZimaSpace"><img src="../../docs/assets/x.png" alt="X" height="128" /></a>
</p>

## Úvod

Inspirováni projektem Clawdbot věříme, že **budoucnost** osobních počítačů bude **utvářena rozmanitými, lokálně orientovanými AI agenty** běžícími na okraji sítě.

**ZimaOS Blue je naše odpověď** — plně **open-source, auditovatelný a produkčně připravený runtime a sada nástrojů pro agenty**, který vám umožní nasadit soukromé, self-hosted agenty bez jakýchkoli překážek.

Vytvořený pro odvážné vývojáře, kteří chtějí **tvořit vlastní agenty intuitivně i ručně**, Blue je **navržený pro výkon**: napsaný v **Go**, s paměťovou náročností pouhých 10 MB. Běží na **jakémkoli x86, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS** — kdekoli, kde je zásuvka.

![](../../docs/assets/features.png)

## Hlavní přednosti

### Lokálně orientovaný design a automatický přístup k modelům

Jdeme ještě dál: nativní podpora **20+ IM platforem**, **hlasové** rozhraní pro přirozený, kontextově uvědomělý dialog, **přepínání modelů bez konfigurace** se skenováním IDE a SOUL-vrstvené osobnosti.

<p align="center">
  <img src="../../docs/assets/channels.png" alt="Supported Channels" />
</p>

### Rychlý, lehký

Nativně kompilovaný v Go — žádný interpret, žádný VM, žádná režie. Tiše běží na všem od serverů po vaše stolní zařízení.

| Metrika | ZimaOS Blue (Go) | OpenClaw (Node + dist) |
|---------|-------------------|------------------------|
| `--help` studený / teplý start | **0,18 s / < 0,01 s** | 3,31 s / ~1,11 s |
| `status` runtime (nejlepší ze 3) | **< 0,01 s** | 5,98 s |
| `--help` špičková RSS | **~10 MB** | ~394 MB |
| `status` špičková RSS | **~15 MB** | ~1,52 GB |
| Runtime závislosti | **Žádné** | Node.js 18+ |

> Měřeno na macOS arm64 (serverový režim, bez desktopového UI), stejný stroj, nejlepší ze 3 běhů. Únor 2026.

### Čisté Go, jakékoli zařízení

100% Go, statický binární soubor. **Křížová kompilace na 5 cílů** ihned po vybalení (![linux](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) amd64/arm64, ![macOS](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) amd64/arm64, ![windows](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) amd64). Žádný Node runtime, žádný Python, žádné kontejnery. Umístěte ho na NAS, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, starý x86 router nebo ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)Mac — prostě funguje. **Pak přidejte vlastní UI, logiku a dovednosti agenta** — jeden kód, každá platforma.

### Bezpečnost a správa

Vestavěný sidecar API proxy s vícevrstvou obranou:
- **Sandboxové spouštění** – Všechna volání nástrojů běží v izolovaných prostředích.
- **Obrana proti prompt injection** – 7+ vestavěných strategií zachycení.
- **Audit relací** – Kompletní monitoring relací, každá interakce je sledovatelná.
- **RBAC a WebAuthn** – Jemně granulované řízení přístupu s bezheslovým ověřováním.

## Proč Blue

Věříme, že **osobní počítače nové generace** přijmou LLM — ale **kontrolovatelní, auditovatelní** agenti zůstávají základem pro jednotlivce i týmy. **Blue přináší**:
- **Komplexní jádro** – Pokročilá správa modelů, integrace IM, vylepšená persona a rozhraní v přirozeném jazyce laděná pro každodenní interakce (sluchátka, hlas, chytré brýle).
- **Lokálně orientovaný, ultralehký, multiplatformní** – Nevyžaduje výkonný hardware. Běží na čemkoli, co umí počítat.
- **Bezpečný a auditovatelný** – Audit relací, sandboxing, řízení oprávnění a vestavěný API proxy fungující jako aplikační firewall — každý bajt dovnitř i ven je viditelný.

![](../../docs/assets/design_principle.png)

Minimalizujeme šablonový kód, abyste se **soustředili na to, co je důležité**. Věrni <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> **designové filozofii ZimaOS**, Blue přináší:
- **Od nuly k jedničce jedním kliknutím** – Okamžité nasazení, žádná složitá konfigurace.
- **Rychlé prototypování** – Tvořte intuitivně nebo ručně nástroje, interakce a balíčky aplikací pro konkrétní scénáře.
- **Připravený pro celý svět** – **Svět je obrovský** a nepoužívá výchozí angličtinu. **20+ jazyků, nativně**, bez bariér.
- **Otevřený ekosystém modelů** – Žádné vendor lock-in. Přineste si vlastní modely.

<details>
<summary>
<p align="center">
  <img src="../../docs/assets/providers.png" alt="Supported Providers" />
</p>
</summary>

| Poskytovatel | Modely | Typ |
|--------------|--------|-----|
| OpenAI | GPT-4o, GPT-4, o1, o3 | Cloud |
| Anthropic | Claude 4.5, Claude 4 | Cloud |
| Google | Gemini 2.5, Gemini 2.0 | Cloud |
| Ollama | Llama, Qwen, Gemma, Phi aj. | Lokální |
| DeepSeek | DeepSeek-V3, DeepSeek-R1 | Cloud |
| Grok | Grok-3, Grok-3-mini | Cloud |
| Qwen | Qwen-Max, Qwen-Plus, Qwen-Turbo | Cloud |
| GLM | GLM-4, GLM-4-Flash | Cloud |
| Moonshot | Moonshot-v1 | Cloud |
| MiniMax | abab6.5, abab5.5 | Cloud |
| Venice | Llama, Mistral (zaměření na soukromí) | Cloud |
| AWS Bedrock | Claude, Llama, Titan | Cloud |
| Azure | Modely OpenAI přes Azure | Cloud |
| OpenRouter | 100+ agregovaných modelů | Cloud |
| AIHubMix | Multi-provider agregátor | Cloud |
| Codex | OpenAI Codex | Cloud |
| SiliconFlow | DeepSeek, Qwen, Llama via SiliconFlow | Cloud |
| Vlastní | Jakékoli API kompatibilní s OpenAI / Anthropic / Gemini | Cloud / Lokální |

</details>

### Podporovaná IDE

<p align="center">
  <img src="../../docs/assets/ides.png" alt="Supported IDEs" />
</p>

## Rychlý start

### Možnost 1: Stáhnout desktopovou aplikaci (macOS a Windows)

Získejte nativní aplikaci — žádné závislosti, žádná kompilace. Vestavěná zkušební konfigurace, připravená za sekundy — připojte se vzdáleně a začněte chatovat okamžitě, bez nastavování bota. Skutečný start na první pokus.

- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)**macOS**: [Stáhnout DMG](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)**Windows**: [Stáhnout instalátor](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### Možnost 2: Instalační skript

![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### Možnost 3: Sestavení ze zdrojového kódu

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

## Přehled architektury

<details>
<summary>
<img src="../../docs/assets/architecture.png" alt="Architecture" />
</summary>

### Mapa balíčků (`server/internal/`)

| Vrstva | Balíčky |
|--------|---------|
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

### Tok dat

**Chatovací požadavek (horká cesta proxy)**
```
Client [Proxy API Key] → Auth Gate → Prompt Guard → Context Pruner (optional)
  → Provider Pool (route:auto/cloud/local) → CC Cache (L1→L2) check
  → Upstream LLM → Response → Cache Store → Metrics Writer → Client (SSE stream)
```

**Tok zpráv kanálem**
```
Telegram/Discord/... → Channel Manager → AutoReply check
  → Chat Handler → LLM → Humanizer (MD→text) → Channel → User
```

**Hlasový pipeline**
```
WebSocket audio → STT (Whisper) → LLM Processing → TTS (eSpeak/Edge) → WebSocket audio
```

**Monitorování Heartbeat**
```
Časovač (30 min) → Čtení HEARTBEAT.md → Vyhodnocení LLM → Odstranění tokenu HEARTBEAT_OK
  → Deduplikace (hash FNV, TTL 24h) → Upozornění kanálu (Telegram/Slack/...)
  → Proud událostí → UI indikátor
```

## Jak používat

![](../../docs/assets/handcraft.png)

## Časová osa milníků

<details>
<summary>
<img src="../../docs/assets/timeline.png" alt="Milestone Timeline" />
</summary>

| Version | Zaměření | Klíčová hodnota | Status |
|---------|----------|-----------------|--------|
| v0.1 | Jádro Go runtime | Stabilní jádro, 24h běh | Done |
| v0.2 | Základní schopnosti | Minimálně použitelný, integrace LLM | Done |
| v0.3 | Integrace NAS | Nativní NAS, podpora systemd | Done |
| v0.4 | Systém pluginů | Rozšiřitelný, základy bezpečnosti | Done |
| v0.5 | Produktový základ | Připravený pro produkci, dokumentace | Done |
| v0.6 | Kanály zpráv | Podpora více kanálů | Done |
| v0.7 | Bezpečnost | OIDC, MFA, audit | Done |
| v0.8 | Výkon | Optimalizace, cachování, benchmarky | Done |
| v0.9 | Ekosystém | Multi-tenant, automatizace prohlížeče, hlas | Done |
| v0.10.0 | Balíčkování CLI | Balíčkování CC CLI, detekce, auto-update | Done |
| v0.10.1 | Monitoring metrik | Statistiky API, sledování tokenů, TTFT | Done |
| v0.10.2 | Spolehlivost CLI | Životní cyklus procesů, obnova po chybách | Done |
| v0.10.3 | Integrace CLI | Průvodce nastavením, auto-detekce poskytovatele | Done |
| v0.10.4 | Balíčkování Tauri | Desktopová aplikace, systémová lišta | Done |
| v0.10.5 | API Proxy Sidecar | Výběr tras, prompt guard, statistiky využití | Done |
| v0.10.6 | Provider Pool | Multi-provider routing, health check, failover | Done |
| v0.10.7 | Režim náhledu | Neautentizovaný přístup, gating funkcí | Done |
| v0.10.8 | Skill Store | Infrastruktura skill store, validace kanálů | Done |
| v0.10.9–10 | Správa uživatelů | Poduživatelé, oprávnění na úrovni stránek | Done |
| v0.10.13–14 | Bezpečnost a dovednosti | Stránka bezpečnosti, redesign skill store | Done |
| v0.10.15 | Vylepšení chatu | UX chatu, pipeline zpráv | Done |
| v0.10.16 | Hlasový modul | Sherpa TTS/ASR, eSpeak, přepínání poskytovatelů | Done |
| v0.10.17 | Vzdálený přístup | Ngrok, Cloudflare tunely, ACME certifikáty | Done |
| v0.10.18–20 | Sprint výkonu | Výkon startu/chatu, kontextová cache | Done |
| v0.10.21–22 | Prompt a DingTalk | Systémový prompt, kanál DingTalk | Done |
| v0.10.23 | OTA aktualizace | Systém OTA aktualizací | Done |
| v0.10.24 | Upgrade kanálů | 10 kanálů upgradováno ze stubů | Done |
| v0.10.25 | CC Cache | Dvouúrovňová cache (L1 paměť + L2 disk) | Done |
| v0.10.26 | Humanizer | Pipeline humanizace odpovědí | Done |
| v0.10.27 | Context Pruner | 54% úspora tokenů na kódu (SWE-bench oficiálně), 46–47% na obecných dokumentech (lokální IR), BM25 skórování, segmentace | Done |
| v0.10.28 | Memory Service | Progresivní vyhledávání, dual-write backend | Done |

</details>

## Komunita a podpora

- **Issues**: [Nahlaste chyby a požadavky na funkce zde](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **Diskuze**: [Discord](https://discord.gg/b3AgFDxe9v)
- **Sledujte nás** na [GitHubu](https://github.com/IceWhaleTech)

## Licence

Tento projekt je licencován pod licencí MIT – podrobnosti naleznete v souboru [LICENSE](../../LICENSE). Věříme v open source a v přínos komunitě.

## Přispěvatelé

<p align="center">
  Vytvořeno s ❤️ týmem <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
