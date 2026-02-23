![](../../docs/assets/bannerX.png)

<p align="center">
  <a href="../../README.md">English</a> |
  <strong>Català</strong> |
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
  <a href="../sv_SE/README.md">Svenska</a> |
  <a href="../zh_CN/README.md">简体中文</a> |
  <a href="../zh_TW/README.md">繁體中文</a>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Blue/ci.yml?branch=main&style=for-the-badge" alt="Estat CI"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Blue?include_prereleases&style=for-the-badge" alt="Versió GitHub"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="Llicència MIT"></a>
</p>

<p align="center">
  <a href="https://discord.gg/SrCYvumF"><img src="../../docs/assets/discord.png" alt="Discord" height="128" /></a>&nbsp;&nbsp;
  <a href="https://www.facebook.com/zimaboard/"><img src="../../docs/assets/facebook.png" alt="Facebook" height="128" /></a>&nbsp;&nbsp;
  <a href="https://x.com/ZimaSpace"><img src="../../docs/assets/x.png" alt="X" height="128" /></a>
</p>

## Introducció

Inspirats per Clawdbot, creiem que el **futur** de la informàtica personal serà **modelat per agents d'IA locals i diversos** que s'executen a la vora de la xarxa.

**ZimaOS Blue és la nostra resposta** — un **entorn d'execució i conjunt d'eines per a agents totalment obert, auditable i preparat per a producció** que us permet desplegar agents privats i autoallotjats sense cap fricció.

Dissenyat per a desenvolupadors audaços que volen **crear els seus propis agents amb inspiració o a mà**, Blue està **optimitzat per al rendiment**: escrit en **Go**, amb un consum de memòria tan baix com 10 MB. Funciona en **qualsevol x86, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS** — allà on hi hagi corrent.

![](../../docs/assets/features.png)

## Característiques destacades

### Disseny local-first i accés automàtic a models

Aneu més enllà: ofereix suport natiu per a **més de 20 plataformes de missatgeria instantània**, interfícies **controlades per veu** per a diàlegs naturals i contextuals, **canvi de model sense configuració** amb escaneig d'IDE, i personalitats amb capes SOUL.

<p align="center">
  <img src="../../docs/assets/channels.png" alt="Supported Channels" />
</p>

### Ràpid i lleuger

Compilat nativament en Go — sense intèrpret, sense VM, sense sobrecàrrega. Funciona silenciosament en tot, des de servidors fins als vostres dispositius d'escriptori.

| Mètrica | ZimaOS Blue (Go) | OpenClaw (Node + dist) |
|---------|-------------------|------------------------|
| `--help` fred / calent | **0.18 s / < 0.01 s** | 3.31 s / ~1.11 s |
| `status` temps d'execució (millor de 3) | **< 0.01 s** | 5.98 s |
| `--help` RSS màxim | **~10 MB** | ~394 MB |
| `status` RSS màxim | **~15 MB** | ~1.52 GB |
| Dependències d'execució | **Cap** | Node.js 18+ |

> Mesurat en macOS arm64, mateix host, millor de 3 execucions. Feb 2026.

### Go pur, qualsevol dispositiu

100% Go, binari estàtic. **Compilació creuada per a 5 objectius** de sèrie (![linux](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) amd64/arm64, ![macOS](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) amd64/arm64, ![windows](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) amd64). Sense runtime de Node, sense Python, sense contenidors. Poseu-lo en un NAS, una ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, un router x86 antic o un ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)Mac — simplement funciona. **Després afegiu la vostra pròpia interfície, lògica i habilitats d'agent** — un sol codi, totes les plataformes.

### Seguretat i governança

Proxy API sidecar integrat amb defensa en profunditat:
- **Execució en sandbox** – Totes les crides a eines s'executen en entorns aïllats.
- **Defensa contra injecció de prompts** – Més de 7 estratègies d'intercepció integrades.
- **Auditoria de sessions** – Monitoratge complet de sessions, cada interacció és traçable.
- **RBAC i WebAuthn** – Control d'accés granular amb autenticació sense contrasenya.

## Per què Blue

Creiem que la **informàtica personal de nova generació** abraça els LLM — però els agents **controlables i auditables** segueixen sent la base tant per a individus com per a equips. **Blue ofereix**:
- **Nucli complet** – Gestió avançada de models, integració de missatgeria instantània, persona millorada i interfícies de llenguatge natural adaptades per a interaccions diàries (auriculars, veu, ulleres intel·ligents).
- **Local-first, ultralleuger, multidispositiu** – No cal maquinari d'alta gamma. Funciona en qualsevol cosa que pugui computar.
- **Segur i auditable** – Auditoria de sessions, sandboxing, controls de permisos i un proxy API integrat que actua com a tallafocs de capa d'aplicació — cada byte d'entrada/sortida és visible.

![](../../docs/assets/design_principle.png)

Minimitzem el codi repetitiu perquè us **concentreu en el que importa**. Fidels a la <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> **filosofia de disseny de ZimaOS**, Blue ofereix:
- **De zero a u amb un sol clic** – Desplegament instantani, sense configuració complexa.
- **Prototipatge ràpid** – Creeu eines, interaccions i paquets d'aplicacions específics per a cada escenari.
- **Preparat per al món** – **El món és gran**, i no parla anglès per defecte. **Més de 20 idiomes, natius**, sense barreres.
- **Ecosistema de models obert** – Sense dependència de proveïdor. Porteu els vostres propis models.

<details>
<summary>
<p align="center">
  <img src="../../docs/assets/providers.png" alt="Supported Providers" />
</p>
</summary>

| Proveïdor | Models | Tipus |
|-----------|--------|-------|
| OpenAI | GPT-4o, GPT-4, o1, o3 | Cloud |
| Anthropic | Claude 4.5, Claude 4 | Cloud |
| Google | Gemini 2.5, Gemini 2.0 | Cloud |
| Ollama | Llama, Qwen, Gemma, Phi, etc. | Local |
| DeepSeek | DeepSeek-V3, DeepSeek-R1 | Cloud |
| Grok | Grok-3, Grok-3-mini | Cloud |
| Qwen | Qwen-Max, Qwen-Plus, Qwen-Turbo | Cloud |
| GLM | GLM-4, GLM-4-Flash | Cloud |
| Moonshot | Moonshot-v1 | Cloud |
| MiniMax | abab6.5, abab5.5 | Cloud |
| Venice | Llama, Mistral (privadesa primer) | Cloud |
| AWS Bedrock | Claude, Llama, Titan | Cloud |
| Azure | Models OpenAI via Azure | Cloud |
| OpenRouter | 100+ models agregats | Cloud |
| AIHubMix | Agregador multi-proveïdor | Cloud |
| Codex | OpenAI Codex | Cloud |
| SiliconFlow | DeepSeek, Qwen, Llama via SiliconFlow | Cloud |
| Personalitzat | Qualsevol API compatible amb OpenAI / Anthropic / Gemini | Cloud / Local |

</details>

### IDE compatibles

<p align="center">
  <img src="../../docs/assets/ides.png" alt="Supported IDEs" />
</p>

## Inici ràpid

### Opció 1: Descarregar l'aplicació d'escriptori

Obteniu l'aplicació nativa — sense dependències, sense compilació.

- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)**macOS**: [Descarregar DMG](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)**Windows**: [Descarregar instal·lador](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### Opció 2: Script d'instal·lació

![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### Opció 3: Compilar des del codi font

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

## Visió general de l'arquitectura

![](../../docs/assets/architecture.png)

### Flux de dades

**Sol·licitud de xat (camí calent del proxy)**
```
Client [Clau API Proxy] → Porta d'autenticació → Guarda de prompts → Poda de context (opcional)
  → Pool de proveïdors (ruta:auto/núvol/local) → CC Cache (L1→L2) comprovació
  → LLM upstream → Resposta → Emmagatzematge en cache → Escriptor de mètriques → Client (flux SSE)
```

**Flux de missatges de canal**
```
Telegram/Discord/... → Gestor de canals → Comprovació d'autoresposta
  → (sense coincidència) → Gestor de xat → LLM → Humanitzador (MD→text) → Canal → Usuari
```

**Pipeline de veu**
```
WebSocket àudio → STT (Whisper) → Processament LLM → TTS (eSpeak/Edge) → WebSocket àudio
```

**Monitoratge Heartbeat**
```
Temporitzador (30 min) → Lectura HEARTBEAT.md → Avaluació LLM → Eliminació token HEARTBEAT_OK
  → Deduplicació (hash FNV, TTL 24h) → Alerta de canal (Telegram/Slack/...)
  → Flux d'esdeveniments → Indicador UI
```

### Mapa de paquets (`server/internal/`)

| Capa | Paquets |
|------|---------|
| Passarel·la | bootstrap, server, gateway |
| Proxy | proxy, connection, streaming, resilience |
| Proveïdor | providerpool, providers, llm |
| Poda | pruner (detector, segmenter, bm25, pipeline, cache) |
| Agent | context, tools, personality, humanizer |
| Memòria | memory, embedding, kvstore |
| Canal | channel, autoreply, i18n |
| Seguretat | security, auth, permission, rbac, mfa, password, oidc, extauth, sandbox, promptguard, audit |
| Veu | voice, tts, stt, speech |
| Observació | metrics, heartbeat, companion, profiling, leakdetect |
| Connector | plugin, skill, skillstore |
| Integració | browser, homeassistant, cron, workflow, formfiller, tunnel, crawler |
| Planificador | scheduler, worker, workerpool, pool |
| Nucli | lifecycle, config, logger, database, cache, ratelimit, retry, timeutil, sync |
| Sistema | sysinfo, cgroup, iotask, watcher, resources, backup, update |
| Multi-inquilí | tenant, user, session, preview |

## Com utilitzar-lo

![](../../docs/assets/handcraft.png)

## Cronologia de fites

![](../../docs/assets/timeline.png)

| Versió | Focus | Valor clau | Estat |
|--------|-------|------------|-------|
| v0.1 | Nucli d'execució Go | Kernel estable, funcionament 24h | Done |
| v0.2 | Capacitats bàsiques | Mínim usable, integració LLM | Done |
| v0.3 | Integració NAS | NAS natiu, suport systemd | Done |
| v0.4 | Sistema de connectors | Extensible, bases de seguretat | Done |
| v0.5 | Línia base de producte | Preparat per a producció, documentació | Done |
| v0.6 | Canals de missatgeria | Suport multicanal | Done |
| v0.7 | Seguretat | OIDC, MFA, auditoria | Done |
| v0.8 | Rendiment | Optimització, cache, benchmarks | Done |
| v0.9 | Ecosistema | Multi-inquilí, automatització del navegador, veu | Done |
| v0.10.0 | Empaquetament CLI | Empaquetament CC CLI, detecció, actualització automàtica | Done |
| v0.10.1 | Monitoratge de mètriques | Estadístiques API, seguiment de tokens, TTFT | Done |
| v0.10.2 | Fiabilitat CLI | Cicle de vida de processos, recuperació d'errors | Done |
| v0.10.3 | Integració CLI | Assistent de configuració, autodetecció de proveïdors | Done |
| v0.10.4 | Empaquetament Tauri | Aplicació d'escriptori, safata del sistema | Done |
| v0.10.5 | Proxy API Sidecar | Selecció de rutes, guarda de prompts, estadístiques d'ús | Done |
| v0.10.6 | Pool de proveïdors | Enrutament multiproveïdor, comprovació de salut, failover | Done |
| v0.10.7 | Mode de previsualització | Accés sense autenticació, control de funcionalitats | Done |
| v0.10.8 | Botiga d'habilitats | Infraestructura de botiga d'habilitats, validació de canals | Done |
| v0.10.9–10 | Gestió d'usuaris | Subusuaris, permisos a nivell de pàgina | Done |
| v0.10.13–14 | Seguretat i habilitats | Pàgina de seguretat, redisseny de la botiga d'habilitats | Done |
| v0.10.15 | Millores de xat | UX de xat, pipeline de missatges | Done |
| v0.10.16 | Mòdul de parla | Sherpa TTS/ASR, eSpeak, canvi de proveïdor | Done |
| v0.10.17 | Accés remot | Túnels Ngrok, Cloudflare, certificats ACME | Done |
| v0.10.18–20 | Sprint de rendiment | Rendiment d'inici/xat, cache de context | Done |
| v0.10.21–22 | Prompt i DingTalk | Prompt del sistema, canal DingTalk | Done |
| v0.10.23 | Actualització OTA | Sistema d'actualització OTA | Done |
| v0.10.24 | Actualització de canals | 10 canals actualitzats des d'stubs | Done |
| v0.10.25 | CC Cache | Cache de dos nivells (L1 memòria + L2 disc) | Done |
| v0.10.26 | Humanitzador | Pipeline d'humanització de respostes | Done |
| v0.10.27 | Poda de context | 54% estalvi de tokens en codi (SWE-bench oficial), 46–47% en documents generals (IR local), puntuació BM25, segmentació | Done |
| v0.10.28 | Servei de memòria | Cerca progressiva, backend d'escriptura dual | Done |

## Comunitat i suport

- **Incidències**: [Si us plau, reporteu errors i sol·licituds de funcionalitats aquí](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **Discussions**: [Discord](https://discord.gg/SrCYvumF)
- **Seguiu-nos** a [GitHub](https://github.com/IceWhaleTech)

## Llicència

Aquest projecte està llicenciat sota la Llicència MIT - consulteu el fitxer [LICENSE](../../LICENSE) per a més detalls. Creiem en el codi obert i en retornar a la comunitat.

## Col·laboradors

<p align="center">
  Fet amb ❤️ per <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
