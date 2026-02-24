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
  <strong>Slovenčina</strong> |
  <a href="../sv_SE/README.md">Svenska</a> |
  <a href="../zh_CN/README.md">简体中文</a> |
  <a href="../zh_TW/README.md">繁體中文</a>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Blue/ci.yml?branch=main&style=for-the-badge" alt="Stav CI"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Blue?include_prereleases&style=for-the-badge" alt="GitHub vydanie"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT licencia"></a>
</p>

<p align="center">
  <a href="https://discord.gg/b3AgFDxe9v"><img src="../../docs/assets/discord.png" alt="Discord" height="128" /></a>&nbsp;&nbsp;
  <a href="https://www.facebook.com/zimaboard/"><img src="../../docs/assets/facebook.png" alt="Facebook" height="128" /></a>&nbsp;&nbsp;
  <a href="https://x.com/ZimaSpace"><img src="../../docs/assets/x.png" alt="X" height="128" /></a>
</p>

## Úvod

Inšpirovaní projektom Clawdbot veríme, že **budúcnosť** osobného počítačového sveta budú formovať **rozmanití, lokálne orientovaní AI agenti** bežiaci na okraji siete.

**ZimaOS Blue je naša odpoveď** — plne **open-source, auditovateľné a produkčne pripravené agentové prostredie a sada nástrojov**, ktoré vám umožnia nasadiť súkromných, vlastne hostovaných agentov bez akéhokoľvek trenia.

Vytvorený pre odvážnych vývojárov, ktorí chcú **tvoriť vlastných agentov kreatívne alebo ručne**. Blue je **navrhnutý pre výkon**: napísaný v **Go**, s pamäťovou náročnosťou len 10 MB. Beží na **akomkoľvek x86, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS** — kdekoľvek, kde zapojíte napájanie.

![](../../docs/assets/features.png)

## Hlavné vlastnosti

### Lokálne orientovaný dizajn a automatický prístup k modelom

Ideme ešte ďalej: natívna podpora pre **20+ IM platforiem**, **hlasovo ovládané** rozhrania pre prirodzený, kontextovo uvedomelý dialóg, **bezúdržbové prepínanie modelov** s IDE skenovaním a SOUL-vrstvené osobnosti.

<p align="center">
  <img src="../../docs/assets/channels.png" alt="Supported Channels" />
</p>

### Rýchly a ľahký

Natívne kompilovaný v Go — žiadny interpreter, žiadny VM, žiadna réžia. Beží ticho na všetkom od serverov po vaše stolné zariadenia.

| Metrika | ZimaOS Blue (Go) | OpenClaw (Node + dist) |
|---------|-------------------|------------------------|
| `--help` studený / teplý | **0.18 s / < 0.01 s** | 3.31 s / ~1.11 s |
| `status` beh (najlepší z 3) | **< 0.01 s** | 5.98 s |
| `--help` špičkové RSS | **~10 MB** | ~394 MB |
| `status` špičkové RSS | **~15 MB** | ~1.52 GB |
| Závislosti za behu | **Žiadne** | Node.js 18+ |

> Merané na macOS arm64 (serverový režim, bez desktopového UI), rovnaký hostiteľ, najlepší z 3 behov. Feb 2026.

### Čisté Go, akékoľvek zariadenie

100% Go, statický binárny súbor. **Krížová kompilácia pre 5 cieľov** priamo z krabice (![linux](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) amd64/arm64, ![macOS](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) amd64/arm64, ![windows](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) amd64). Žiadny Node runtime, žiadny Python, žiadne kontajnery. Položte ho na NAS, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, starý x86 router alebo ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)Mac — jednoducho beží. **Potom pridajte vlastné UI, logiku a agentové zručnosti** — jeden kód, každá platforma.

### Bezpečnosť a správa

Vstavaný sidecar API proxy s hĺbkovou obranou:
- **Sandbox spúšťanie** – Všetky volania nástrojov bežia v izolovaných prostrediach.
- **Obrana proti prompt injection** – 7+ vstavaných stratégií zachytávania.
- **Audit relácií** – Úplné monitorovanie relácií, každá interakcia je sledovateľná.
- **RBAC & WebAuthn** – Jemnozrnné riadenie prístupu s bezheslovým overovaním.

## Prečo Blue

Veríme, že **osobné počítačové systémy novej generácie** prijímajú LLM — ale **kontrolovateľní, auditovateľní** agenti zostávajú základom pre jednotlivcov aj tímy. **Blue prináša**:
- **Komplexné jadro** – Pokročilá správa modelov, IM integrácia, rozšírená persona a rozhrania v prirodzenom jazyku optimalizované pre každodenné interakcie (headsety, hlas, inteligentné okuliare).
- **Lokálne orientovaný, ultra ľahký, multiplatformový** – Nevyžaduje výkonný hardvér. Beží na čomkoľvek, čo dokáže počítať.
- **Bezpečný a auditovateľný** – Audit relácií, sandboxing, riadenie oprávnení a vstavaný API proxy fungujúci ako aplikačný firewall — každý bajt dnu/von je viditeľný.

![](../../docs/assets/design_principle.png)

Minimalizujeme šablónový kód, aby ste sa **sústredili na to, čo je dôležité**. Verní <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> **dizajnovej filozofii ZimaOS**, Blue prináša:
- **Od nuly k jednej jedným kliknutím** – Okamžité nasadenie, žiadna zložitá konfigurácia.
- **Rýchle prototypovanie** – Tvorte kreatívne alebo ručne scenárovo špecifické nástroje, interakcie a balíčky aplikácií.
- **Globálne pripravený** – **Svet je veľký** a nehovorí predvolene anglicky. **20+ jazykov, natívne**, žiadne bariéry.
- **Otvorený ekosystém modelov** – Žiadne uzamknutie dodávateľom. Prineste si vlastné modely.

<details>
<summary>
<p align="center">
  <img src="../../docs/assets/providers.png" alt="Supported Providers" />
</p>
</summary>

| Poskytovateľ | Modely | Typ |
|--------------|--------|-----|
| OpenAI | GPT-4o, GPT-4, o1, o3 | Cloud |
| Anthropic | Claude 4.5, Claude 4 | Cloud |
| Google | Gemini 2.5, Gemini 2.0 | Cloud |
| Ollama | Llama, Qwen, Gemma, Phi atď. | Lokálny |
| DeepSeek | DeepSeek-V3, DeepSeek-R1 | Cloud |
| Grok | Grok-3, Grok-3-mini | Cloud |
| Qwen | Qwen-Max, Qwen-Plus, Qwen-Turbo | Cloud |
| GLM | GLM-4, GLM-4-Flash | Cloud |
| Moonshot | Moonshot-v1 | Cloud |
| MiniMax | abab6.5, abab5.5 | Cloud |
| Venice | Llama, Mistral (zameranie na súkromie) | Cloud |
| AWS Bedrock | Claude, Llama, Titan | Cloud |
| Azure | Modely OpenAI cez Azure | Cloud |
| OpenRouter | 100+ agregovaných modelov | Cloud |
| AIHubMix | Multi-provider agregátor | Cloud |
| Codex | OpenAI Codex | Cloud |
| SiliconFlow | DeepSeek, Qwen, Llama via SiliconFlow | Cloud |
| Vlastný | Akékoľvek API kompatibilné s OpenAI / Anthropic / Gemini | Cloud / Lokálny |

</details>

### Podporované IDE

<p align="center">
  <img src="../../docs/assets/ides.png" alt="Supported IDEs" />
</p>

## Rýchly štart

### Možnosť 1: Stiahnuť desktopovú aplikáciu

Získajte natívnu aplikáciu — žiadne závislosti, žiadna kompilácia. Vstavaná skúšobná konfigurácia, pripravená za sekundy — pripojte sa vzdialene a začnite chatovať okamžite, bez nastavovania bota. Skutočný štart na prvý pokus.

- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)**macOS**: [Stiahnuť DMG](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)**Windows**: [Stiahnuť inštalátor](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### Možnosť 2: Inštalačný skript

![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### Možnosť 3: Zostaviť zo zdrojového kódu

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

## Prehľad architektúry

<details>
<summary>
<img src="../../docs/assets/architecture.png" alt="Architecture" />
</summary>

### Mapa balíčkov (`server/internal/`)

| Vrstva | Balíčky |
|--------|---------|
| Gateway | bootstrap, server, gateway |
| Proxy | proxy, connection, streaming, resilience |
| Poskytovateľ | providerpool, providers, llm |
| Pruner | pruner (detector, segmenter, bm25, pipeline, cache) |
| Agent | context, tools, personality, humanizer |
| Pamäť | memory, embedding, kvstore |
| Kanál | channel, autoreply, i18n |
| Bezpečnosť | security, auth, permission, rbac, mfa, password, oidc, extauth, sandbox, promptguard, audit |
| Hlas | voice, tts, stt, speech |
| Pozorovanie | metrics, heartbeat, companion, profiling, leakdetect |
| Plugin | plugin, skill, skillstore |
| Integrácia | browser, homeassistant, cron, workflow, formfiller, tunnel, crawler |
| Plánovač | scheduler, worker, workerpool, pool |
| Jadro | lifecycle, config, logger, database, cache, ratelimit, retry, timeutil, sync |
| Systém | sysinfo, cgroup, iotask, watcher, resources, backup, update |
| Viacnájomníctvo | tenant, user, session, preview |

</details>

### Tok dát

**Chatová požiadavka (Proxy Hot Path)**
```
Client [Proxy API Key] → Auth Gate → Prompt Guard → Context Pruner (optional)
  → Provider Pool (route:auto/cloud/local) → CC Cache (L1→L2) check
  → Upstream LLM → Response → Cache Store → Metrics Writer → Client (SSE stream)
```

**Tok správ kanálov**
```
Telegram/Discord/... → Channel Manager → AutoReply check
  → Chat Handler → LLM → Humanizer (MD→text) → Channel → User
```

**Hlasový pipeline**
```
WebSocket audio → STT (Whisper) → LLM Processing → TTS (eSpeak/Edge) → WebSocket audio
```

**Monitorovanie Heartbeat**
```
Časovač (30 min) → Čítanie HEARTBEAT.md → Vyhodnotenie LLM → Odstránenie tokenu HEARTBEAT_OK
  → Deduplikácia (hash FNV, TTL 24h) → Upozornenie kanála (Telegram/Slack/...)
  → Prúd udalostí → UI indikátor
```

## Ako používať

![](../../docs/assets/handcraft.png)

## Časový plán míľnikov

<details>
<summary>
<img src="../../docs/assets/timeline.png" alt="Milestone Timeline" />
</summary>

| Verzia | Zameranie | Kľúčová hodnota | Stav |
|--------|-----------|------------------|------|
| v0.1 | Jadro Go runtime | Stabilný kernel, 24h beh | Done |
| v0.2 | Základné schopnosti | Minimálne použiteľný, LLM integrácia | Done |
| v0.3 | NAS integrácia | NAS natívny, podpora systemd | Done |
| v0.4 | Systém pluginov | Rozšíriteľný, základy bezpečnosti | Done |
| v0.5 | Produktový základ | Produkčne pripravený, dokumentácia | Done |
| v0.6 | Kanály správ | Podpora viacerých kanálov | Done |
| v0.7 | Bezpečnosť | OIDC, MFA, audit | Done |
| v0.8 | Výkon | Optimalizácia, cachovanie, benchmarky | Done |
| v0.9 | Ekosystém | Viacnájomníctvo, automatizácia prehliadača, hlas | Done |
| v0.10.0 | Zväzovanie CLI | CC CLI zväzovanie, detekcia, auto-update | Done |
| v0.10.1 | Monitorovanie metrík | API štatistiky, sledovanie tokenov, TTFT | Done |
| v0.10.2 | Spoľahlivosť CLI | Životný cyklus procesov, obnova po chybách | Done |
| v0.10.3 | Integrácia CLI | Sprievodca nastavením, auto-detekcia poskytovateľov | Done |
| v0.10.4 | Balenie Tauri | Desktopová aplikácia, systémová lišta | Done |
| v0.10.5 | API Proxy Sidecar | Výber trás, prompt guard, štatistiky využitia | Done |
| v0.10.6 | Pool poskytovateľov | Multi-provider routing, kontrola zdravia, failover | Done |
| v0.10.7 | Režim náhľadu | Neautentifikovaný prístup, feature gating | Done |
| v0.10.8 | Skill Store | Infraštruktúra skill store, validácia kanálov | Done |
| v0.10.9–10 | Správa používateľov | Podpoužívatelia, oprávnenia na úrovni stránok | Done |
| v0.10.13–14 | Bezpečnosť & Skills | Stránka bezpečnosti, redizajn skill store | Done |
| v0.10.15 | Vylepšenia chatu | Chat UX, pipeline správ | Done |
| v0.10.16 | Hlasový modul | Sherpa TTS/ASR, eSpeak, prepínanie poskytovateľov | Done |
| v0.10.17 | Vzdialený prístup | Ngrok, Cloudflare tunely, ACME certifikáty | Done |
| v0.10.18–20 | Výkonnostný šprint | Výkon štartu/chatu, kontextový cache | Done |
| v0.10.21–22 | Prompt & DingTalk | Systémový prompt, DingTalk kanál | Done |
| v0.10.23 | OTA aktualizácia | Systém OTA aktualizácií | Done |
| v0.10.24 | Upgrade kanálov | 10 kanálov povýšených zo stubov | Done |
| v0.10.25 | CC Cache | Dvojúrovňový cache (L1 pamäť + L2 disk) | Done |
| v0.10.26 | Humanizer | Pipeline humanizácie odpovedí | Done |
| v0.10.27 | Context Pruner | 54% úspora tokenov na kóde (SWE-bench oficiálne), 46–47% na všeobecných dokumentoch (lokálny IR), BM25 skórovanie, segmentácia | Done |
| v0.10.28 | Memory Service | Progresívne vyhľadávanie, dual-write backend | Done |

</details>

## Komunita a podpora

- **Issues**: [Prosím nahlasujte chyby a požiadavky na funkcie tu](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **Diskusie**: [Discord](https://discord.gg/b3AgFDxe9v)
- **Sledujte nás** na [GitHub](https://github.com/IceWhaleTech)

## Licencia

Tento projekt je licencovaný pod licenciou MIT — podrobnosti nájdete v súbore [LICENSE](../../LICENSE). Veríme v open source a v prínos pre komunitu.

## Prispievatelia

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
