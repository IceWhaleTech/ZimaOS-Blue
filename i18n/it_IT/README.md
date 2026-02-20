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
  <strong>Italiano</strong> |
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
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Blue/ci.yml?branch=main&style=for-the-badge" alt="Stato CI"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Blue?include_prereleases&style=for-the-badge" alt="Release GitHub"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="Licenza MIT"></a>
</p>

<p align="center">
  <a href="https://discord.gg/SrCYvumF"><img src="../../docs/assets/discord.png" alt="Discord" height="128" /></a>&nbsp;&nbsp;
  <a href="https://www.facebook.com/zimaboard/"><img src="../../docs/assets/facebook.png" alt="Facebook" height="128" /></a>&nbsp;&nbsp;
  <a href="https://x.com/ZimaSpace"><img src="../../docs/assets/x.png" alt="X" height="128" /></a>
</p>

## Introduzione

Ispirati da Clawdbot, crediamo che il **futuro** dell'informatica personale sarà **plasmato da agenti AI diversificati e local-first** in esecuzione all'edge.

**ZimaOS Blue è la nostra risposta** — un **runtime e toolkit per agenti completamente open-source, verificabile e pronto per la produzione** che ti permette di distribuire agenti privati e self-hosted senza alcuna complessità.

Pensato per sviluppatori audaci che vogliono **creare i propri agenti in modo libero o artigianale**, Blue è **progettato per le prestazioni**: scritto in **Go**, con un consumo di memoria a partire da soli 10 MB. Funziona su **qualsiasi x86, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS** — ovunque ci sia una presa di corrente.

![](../../docs/assets/features.png)

## Punti di Forza

### Design Local-First e Accesso Automatico ai Modelli

Vai oltre: offre supporto nativo per **oltre 20 piattaforme di messaggistica**, interfacce **a controllo vocale** per dialoghi naturali e contestuali, **cambio modello senza configurazione** con scansione IDE, e personalità a livelli SOUL.

<p align="center">
  <img src="../../docs/assets/providers.png" alt="Supported Providers" />
</p>

### Veloce e Leggero

Compilato nativamente in Go — nessun interprete, nessuna VM, nessun overhead. Funziona silenziosamente su tutto, dai server ai tuoi dispositivi desktop.

| Metrica | ZimaOS Blue (Go) | OpenClaw (Node + dist) |
|---------|-------------------|------------------------|
| `--help` cold / warm | **0.18 s / < 0.01 s** | 3.31 s / ~1.11 s |
| `status` runtime (migliore su 3) | **< 0.01 s** | 5.98 s |
| `--help` picco RSS | **~10 MB** | ~394 MB |
| `status` picco RSS | **~15 MB** | ~1.52 GB |
| Dipendenze runtime | **Nessuna** | Node.js 18+ |

> Benchmark su macOS arm64 (modalità server, senza interfaccia desktop), stesso host, migliore su 3 esecuzioni. Feb 2026.

### Go Puro, Qualsiasi Dispositivo

100% Go, binario statico. **Cross-compila per 5 target** pronti all'uso (![linux](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) amd64/arm64, ![macOS](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) amd64/arm64, ![windows](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) amd64). Nessun runtime Node, nessun Python, nessun container richiesto. Mettilo su un NAS, un ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, un vecchio router x86 o un ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)Mac — funziona e basta. **Poi aggiungi la tua UI, la tua logica e le tue skill per agenti** — un unico codebase, ogni piattaforma.

### Sicurezza e Governance

Proxy API sidecar integrato con difesa in profondità:
- **Esecuzione in Sandbox** – Tutte le chiamate agli strumenti vengono eseguite in ambienti isolati.
- **Difesa contro Prompt Injection** – 7+ strategie di intercettazione integrate.
- **Audit delle Sessioni** – Monitoraggio completo delle sessioni, ogni interazione è tracciabile.
- **RBAC e WebAuthn** – Controllo degli accessi granulare con autenticazione senza password.

## Perché Blue

Crediamo che la **prossima generazione dell'informatica personale** abbracci gli LLM — ma gli agenti **controllabili e verificabili** restano il fondamento sia per individui che per team. **Blue offre**:
- **Nucleo Completo** – Gestione avanzata dei modelli, integrazione con messaggistica, personalità avanzata e interfacce in linguaggio naturale ottimizzate per l'uso quotidiano (cuffie, voce, smart glasses).
- **Local-First, Ultra-Leggero, Multi-Dispositivo** – Nessun hardware di fascia alta richiesto. Funziona su qualsiasi cosa in grado di calcolare.
- **Sicuro e Verificabile** – Audit delle sessioni, sandboxing, controllo dei permessi e un proxy API integrato che funge da firewall a livello applicativo — ogni byte in entrata/uscita è visibile.

Riduciamo al minimo il codice ripetitivo così puoi **concentrarti su ciò che conta**. Fedeli alla <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> **filosofia di design di ZimaOS**, Blue offre:
- **Da Zero a Uno con Un Click** – Deploy istantaneo, nessuna configurazione complessa.
- **Prototipazione Rapida** – Crea liberamente o artigianalmente strumenti, interazioni e pacchetti app specifici per ogni scenario.
- **Pronto per il Mondo** – **Il mondo è grande**, e non parla solo inglese. **Oltre 20 lingue, native**, senza barriere.
- **Ecosistema di Modelli Aperto** – Nessun vendor lock-in. Porta i tuoi modelli.

![](../../docs/assets/design_principle.png)

## Avvio Rapido

### Opzione 1: Scarica l'App Desktop (macOS e Windows)

Ottieni l'applicazione nativa — nessuna dipendenza, nessuna compilazione. Configurazione di prova integrata, pronta in pochi secondi — inizia a chattare subito tramite connessione remota, senza configurare alcun bot. Pronta all'uso.

- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)**macOS**: [Scarica DMG](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)**Windows**: [Scarica Installer](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### Opzione 2: Script di Installazione

![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### Opzione 3: Compilazione dal Sorgente

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

> **Note:** Windows builds require:
> - [MinGW-w64](https://www.mingw-w64.org/) (gcc) and [CMake](https://cmake.org/) for native C dependencies (espeak-ng, whisper.cpp, opus, kokoro, onnx)
> - [Windows SDK](https://developer.microsoft.com/en-us/windows/downloads/windows-sdk/) for system libraries (winmm, etc.)
>
> Make sure `gcc`, `cmake` are in your `PATH`.

## Panoramica dell'Architettura

<details>
<summary>
<img src="../../docs/assets/architecture.png" alt="Architettura" />
</summary>

### Mappa dei Pacchetti (`server/internal/`)

| Livello | Pacchetti |
|---------|-----------|
| Gateway | bootstrap, server, gateway |
| Proxy | proxy, connection, streaming, resilience |
| Provider | providerpool, providers, llm |
| Pruner | pruner (detector, segmenter, bm25, pipeline, cache) |
| Agente | context, tools, personality, humanizer |
| Memoria | memory, embedding, kvstore |
| Canale | channel, autoreply, i18n |
| Sicurezza | security, auth, permission, rbac, mfa, password, oidc, extauth, sandbox, promptguard, audit |
| Voce | voice, tts, stt, speech |
| Osservabilità | metrics, heartbeat, companion, profiling, leakdetect |
| Plugin | plugin, skill, skillstore |
| Integrazione | browser, homeassistant, cron, workflow, formfiller, tunnel, crawler |
| Scheduler | scheduler, worker, workerpool, pool |
| Core | lifecycle, config, logger, database, cache, ratelimit, retry, timeutil, sync |
| Sistema | sysinfo, cgroup, iotask, watcher, resources, backup, update |
| Multi-tenant | tenant, user, session, preview |

</details>

### Flusso dei Dati

**Richiesta Chat (Percorso Caldo del Proxy)**
```
Client [Proxy API Key] → Auth Gate → Prompt Guard → Context Pruner (optional)
  → Provider Pool (route:auto/cloud/local) → CC Cache (L1→L2) check
  → Upstream LLM → Response → Cache Store → Metrics Writer → Client (SSE stream)
```

**Flusso dei Messaggi nei Canali**
```
Telegram/Discord/... → Channel Manager → AutoReply check
  → Chat Handler → LLM → Humanizer (MD→text) → Channel → User
```

**Pipeline Vocale**
```
WebSocket audio → STT (Whisper) → LLM Processing → TTS (eSpeak/Edge) → WebSocket audio
```

## Come Usare

![](../../docs/assets/handcraft.png)

## Cronologia delle Milestone

<details>
<summary>
<img src="../../docs/assets/timeline.png" alt="Cronologia delle Milestone" />
</summary>

| Versione | Focus | Valore Chiave | Stato |
|----------|-------|---------------|-------|
| v0.1 | Runtime Core in Go | Kernel stabile, esecuzione 24h | Done |
| v0.2 | Funzionalità Core | Minimamente utilizzabile, integrazione LLM | Done |
| v0.3 | Integrazione NAS | NAS nativo, supporto systemd | Done |
| v0.4 | Sistema Plugin | Estensibile, basi di sicurezza | Done |
| v0.5 | Baseline di Prodotto | Pronto per la produzione, documentazione | Done |
| v0.6 | Canali di Messaggistica | Supporto multi-canale | Done |
| v0.7 | Sicurezza | OIDC, MFA, audit | Done |
| v0.8 | Prestazioni | Ottimizzazione, caching, benchmark | Done |
| v0.9 | Ecosistema | Multi-tenant, automazione browser, voce | Done |
| v0.10.0 | Bundling CLI | Bundling CC CLI, rilevamento, auto-update | Done |
| v0.10.1 | Monitoraggio Metriche | Statistiche API, tracciamento token, TTFT | Done |
| v0.10.2 | Affidabilità CLI | Ciclo di vita dei processi, recupero errori | Done |
| v0.10.3 | Integrazione CLI | Wizard di configurazione, auto-rilevamento provider | Done |
| v0.10.4 | Packaging Tauri | App desktop, system tray | Done |
| v0.10.5 | Proxy API Sidecar | Selezione route, prompt guard, statistiche utilizzo | Done |
| v0.10.6 | Provider Pool | Routing multi-provider, health check, failover | Done |
| v0.10.7 | Modalità Anteprima | Accesso non autenticato, gating delle funzionalità | Done |
| v0.10.8 | Skill Store | Infrastruttura skill store, validazione canali | Done |
| v0.10.9–10 | Gestione Utenti | Sotto-utenti, permessi a livello di pagina | Done |
| v0.10.13–14 | Sicurezza e Skill | Pagina sicurezza, redesign skill store | Done |
| v0.10.15 | Miglioramenti Chat | UX chat, pipeline messaggi | Done |
| v0.10.16 | Modulo Vocale | Sherpa TTS/ASR, eSpeak, cambio provider | Done |
| v0.10.17 | Accesso Remoto | Tunnel Ngrok, Cloudflare, certificati ACME | Done |
| v0.10.18–20 | Sprint Prestazioni | Perf avvio/chat, cache contesto | Done |
| v0.10.21–22 | Prompt e DingTalk | Prompt di sistema, canale DingTalk | Done |
| v0.10.23 | Aggiornamento OTA | Sistema di aggiornamento OTA | Done |
| v0.10.24 | Upgrade Canali | 10 canali aggiornati da stub | Done |
| v0.10.25 | CC Cache | Cache a due livelli (L1 mem + L2 disco) | Done |
| v0.10.26 | Humanizer | Pipeline di umanizzazione delle risposte | Done |
| v0.10.27 | Context Pruner | 54% risparmio token su codice (SWE-bench ufficiale), 46–47% su documenti generali (IR locale), scoring BM25, segmentazione | Done |
| v0.10.28 | Servizio Memoria | Ricerca progressiva, backend dual-write | Done |

</details>

## Comunità e Supporto

- **Segnalazioni**: [Segnala bug e richieste di funzionalità qui](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **Discussioni**: [Discord](https://discord.gg/SrCYvumF)
- **Seguici** su [GitHub](https://github.com/IceWhaleTech)

## Licenza

Questo progetto è rilasciato sotto la Licenza MIT - consulta il file [LICENSE](../../LICENSE) per i dettagli. Crediamo nell'open source e nel restituire alla comunità.

## Contributori

<p align="center">
  Fatto con ❤️ da <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
