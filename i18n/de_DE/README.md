![](../../docs/assets/bannerX.png)

<p align="center">
  <a href="../../README.md">English</a> |
  <a href="../ca_ES/README.md">Català</a> |
  <a href="../cs_CZ/README.md">Čeština</a> |
  <a href="../da_DK/README.md">Dansk</a> |
  <strong>Deutsch</strong> |
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
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Blue/ci.yml?branch=main&style=for-the-badge" alt="CI-Status"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Blue?include_prereleases&style=for-the-badge" alt="GitHub Release"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT-Lizenz"></a>
</p>

<p align="center">
  <a href="https://discord.gg/b3AgFDxe9v"><img src="../../docs/assets/discord.png" alt="Discord" height="128" /></a>&nbsp;&nbsp;
  <a href="https://www.facebook.com/zimaboard/"><img src="../../docs/assets/facebook.png" alt="Facebook" height="128" /></a>&nbsp;&nbsp;
  <a href="https://x.com/ZimaSpace"><img src="../../docs/assets/x.png" alt="X" height="128" /></a>
</p>

## Einleitung

Inspiriert von Clawdbot sind wir überzeugt, dass die **Zukunft** des Personal Computing durch **vielfältige, lokal-priorisierte KI-Agenten** am Netzwerkrand geprägt sein wird.

**ZimaOS Blue ist unsere Antwort** — eine vollständig **quelloffene, auditierbare und produktionsreife Agenten-Laufzeitumgebung und Werkzeugsammlung**, mit der Sie private, selbst gehostete Agenten ohne jegliche Reibungsverluste bereitstellen können.

Entwickelt für mutige Entwickler, die ihre **eigenen Agenten kreativ oder manuell erstellen** möchten. Blue ist **auf Leistung ausgelegt**: geschrieben in **Go**, mit einem Speicherbedarf von nur 10 MB. Es läuft auf **jedem x86-System, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS** — überall, wo Strom angeschlossen wird.

![](../../docs/assets/features.png)

## Highlights

### Lokal-priorisiertes Design & Automatischer Modellzugriff

Noch weiter gedacht: Native Unterstützung für **20+ IM-Plattformen**, **sprachgesteuerte** Schnittstellen für natürliche, kontextbewusste Dialoge, **konfigurationsfreier Modellwechsel** mit IDE-Erkennung und SOUL-basierte Persönlichkeiten.

<p align="center">
  <img src="../../docs/assets/channels.png" alt="Supported Channels" />
</p>

### Schnell & Leichtgewichtig

Nativ in Go kompiliert — kein Interpreter, keine VM, kein Overhead. Läuft unauffällig auf allem, von Servern bis zu Desktop-Geräten.

| Metrik | ZimaOS Blue (Go) | Reference Agent (Node + dist) |
|--------|-------------------|------------------------|
| `help` kalt / warm | **0.18 s / < 0.01 s** | 3.31 s / ~1.11 s |
| `status` Laufzeit (bester von 3) | **< 0.01 s** | 5.98 s |
| `help` Spitzen-RSS | **~10 MB** | ~394 MB |
| `status` Spitzen-RSS | **~15 MB** | ~1.52 GB |
| Laufzeitabhängigkeiten | **Keine** | Node.js 18+ |

> Gemessen auf macOS arm64 (Servermodus, ohne Desktop-UI), gleicher Host, bester von 3 Durchläufen. Feb 2026.

### Reines Go, Jedes Gerät

100% Go, statisches Binary. **Cross-Kompilierung für 5 Zielplattformen** direkt verfügbar (![linux](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) amd64/arm64, ![macOS](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) amd64/arm64, ![windows](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) amd64). Keine Node-Laufzeitumgebung, kein Python, keine Container erforderlich. Einfach auf ein NAS, einen ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, einen alten x86-Router oder einen Mac legen — es läuft einfach. **Dann eigene UI, Logik und Agenten-Skills hinzufügen** — eine Codebasis, jede Plattform.

### Sicherheit & Governance

Integrierter Sidecar-API-Proxy mit gestaffelter Verteidigung:
- **Sandbox-Ausführung** – Alle Tool-Aufrufe laufen in isolierten Umgebungen.
- **Prompt-Injection-Abwehr** – 7+ integrierte Abfangstrategien.
- **Sitzungs-Auditing** – Vollständige Sitzungsüberwachung, jede Interaktion nachverfolgbar.
- **RBAC & WebAuthn** – Feingranulare Zugriffskontrolle mit passwortloser Authentifizierung.

## Warum Blue

Wir glauben, dass **Personal Computing der nächsten Generation** LLMs einbezieht — aber **kontrollierbare, auditierbare** Agenten bleiben das Fundament für Einzelpersonen und Teams. **Blue bietet**:
- **Umfassender Kern** – Fortschrittliche Modellverwaltung, IM-Integration, erweiterte Persona und natürlichsprachliche Schnittstellen, optimiert für tägliche Interaktionen (Headsets, Sprache, Smart Glasses).
- **Lokal-priorisiert, Ultra-leichtgewichtig, Geräteübergreifend** – Keine High-End-Hardware erforderlich. Läuft auf allem, was rechnen kann.
- **Sicher & Auditierbar** – Sitzungs-Auditing, Sandboxing, Berechtigungskontrollen und ein integrierter API-Proxy, der als Anwendungsschicht-Firewall fungiert — jedes Byte ein/aus ist sichtbar.

![](../../docs/assets/design_principle.png)

Wir minimieren Boilerplate, damit Sie sich **auf das Wesentliche konzentrieren** können. Getreu der <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> **Designphilosophie von ZimaOS** bietet Blue:
- **Von Null auf Eins mit einem Klick** – Sofort bereitstellen, keine komplexe Konfiguration.
- **Schnelles Prototyping** – Kreativ oder manuell szenariospezifische Tools, Interaktionen und App-Pakete erstellen.
- **Global einsatzbereit** – **Die Welt ist groß**, und sie spricht nicht standardmäßig Englisch. **20+ Sprachen, nativ**, keine Barrieren.
- **Offenes Modell-Ökosystem** – Kein Vendor-Lock-in. Bringen Sie Ihre eigenen Modelle mit.

<details>
<summary>
<p align="center">
  <img src="../../docs/assets/providers.png" alt="Supported Providers" />
</p>
</summary>

| Anbieter | Modelle | Typ |
|----------|---------|-----|
| OpenAI | GPT-4o, GPT-4, o1, o3 | Cloud |
| Anthropic | Claude 4.5, Claude 4 | Cloud |
| Google | Gemini 2.5, Gemini 2.0 | Cloud |
| Ollama | Llama, Qwen, Gemma, Phi usw. | Lokal |
| DeepSeek | DeepSeek-V3, DeepSeek-R1 | Cloud |
| Grok | Grok-3, Grok-3-mini | Cloud |
| Qwen | Qwen-Max, Qwen-Plus, Qwen-Turbo | Cloud |
| GLM | GLM-4, GLM-4-Flash | Cloud |
| Moonshot | Moonshot-v1 | Cloud |
| MiniMax | abab6.5, abab5.5 | Cloud |
| Venice | Llama, Mistral (Datenschutz-priorisiert) | Cloud |
| AWS Bedrock | Claude, Llama, Titan | Cloud |
| Azure | OpenAI-Modelle über Azure | Cloud |
| OpenRouter | 100+ aggregierte Modelle | Cloud |
| AIHubMix | Multi-Anbieter-Aggregator | Cloud |
| Codex | OpenAI Codex | Cloud |
| SiliconFlow | DeepSeek, Qwen, Llama via SiliconFlow | Cloud |
| Benutzerdefiniert | Jede OpenAI / Anthropic / Gemini-kompatible API | Cloud / Lokal |

</details>

### Unterstützte IDEs

<p align="center">
  <img src="../../docs/assets/ides.png" alt="Supported IDEs" />
</p>

## Schnellstart

### Option 1: Desktop-App herunterladen

Die native Anwendung herunterladen — keine Abhängigkeiten, keine Kompilierung. Integrierte Testkonfiguration, sofort einsatzbereit — starten Sie über Fernverbindung direkt einen Chat, ohne einen Bot einzurichten. Sofort startklar.

- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)**macOS**: [DMG herunterladen](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)**Windows**: [Installer herunterladen](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### Option 2: Installationsskript

![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### Option 3: Aus Quellcode bauen

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

## Architekturübersicht

<details>
<summary>
<img src="../../docs/assets/architecture.png" alt="Architektur" />
</summary>

### Paketstruktur (`server/internal/`)

| Schicht | Pakete |
|---------|--------|
| Gateway | bootstrap, server, gateway |
| Proxy | proxy, connection, streaming, resilience |
| Anbieter | providerpool, providers, llm |
| Pruner | pruner (detector, segmenter, bm25, pipeline, cache) |
| Agent | context, tools, personality, humanizer |
| Speicher | memory, embedding, kvstore |
| Kanal | channel, autoreply, i18n |
| Sicherheit | security, auth, permission, rbac, mfa, password, oidc, extauth, sandbox, promptguard, audit |
| Sprache | voice, tts, stt, speech |
| Beobachtung | metrics, companion, profiling, leakdetect |
| Plugin | plugin, skill, skillstore |
| Integration | browser, cron, workflow, formfiller, tunnel, crawler |
| Scheduler | scheduler, worker, workerpool, pool |
| Kern | lifecycle, config, logger, database, cache, ratelimit, retry, timeutil, sync |
| System | sysinfo, cgroup, iotask, watcher, resources, backup, update |
| Mehrmandantenfähigkeit | tenant, user, session, preview |

</details>

### Datenfluss

**Chat-Anfrage (Proxy-Hot-Path)**
```
Client [Proxy API Key] → Auth Gate → Prompt Guard → Context Pruner (optional)
  → Provider Pool (route:auto/cloud/local) → CC Cache (L1→L2) check
  → Upstream LLM → Response → Cache Store → Metrics Writer → Client (SSE stream)
```

**Kanal-Nachrichtenfluss**
```
Telegram/Discord/... → Channel Manager → AutoReply check
  → Chat Handler → LLM → Humanizer (MD→text) → Channel → User
```

**Sprach-Pipeline**
```
WebSocket audio → STT (Whisper) → LLM Processing → TTS (eSpeak/Edge) → WebSocket audio
```


## Verwendung

![](../../docs/assets/handcraft.png)

## Meilenstein-Zeitplan

<details>
<summary>
<img src="../../docs/assets/timeline.png" alt="Meilenstein-Zeitplan" />
</summary>

| Version | Schwerpunkt | Kernwert | Status |
|---------|-------------|----------|--------|
| v0.1 | Go-Laufzeitkern | Stabiler Kernel, 24h-Betrieb | Done |
| v0.2 | Kernfähigkeiten | Minimal nutzbar, LLM-Integration | Done |
| v0.3 | NAS-Integration | NAS-nativ, systemd-Unterstützung | Done |
| v0.4 | Plugin-System | Erweiterbar, Sicherheitsgrundlagen | Done |
| v0.5 | Produktbasis | Produktionsreif, Dokumentation | Done |
| v0.6 | Nachrichtenkanäle | Multi-Kanal-Unterstützung | Done |
| v0.7 | Sicherheit | OIDC, MFA, Audit | Done |
| v0.8 | Leistung | Optimierung, Caching, Benchmarks | Done |
| v0.9 | Ökosystem | Mehrmandantenfähigkeit, Browser-Automatisierung, Sprache | Done |
| v0.10.0 | CLI-Bündelung | CC CLI-Bündelung, Erkennung, Auto-Update | Done |
| v0.10.1 | Metrik-Überwachung | API-Statistiken, Token-Tracking, TTFT | Done |
| v0.10.2 | CLI-Zuverlässigkeit | Prozess-Lebenszyklus, Fehlerwiederherstellung | Done |
| v0.10.3 | CLI-Integration | Einrichtungsassistent, Anbieter-Autoerkennung | Done |
| v0.10.4 | Tauri-Paketierung | Desktop-App, System-Tray | Done |
| v0.10.5 | API-Proxy-Sidecar | Routenauswahl, Prompt Guard, Nutzungsstatistiken | Done |
| v0.10.6 | Anbieter-Pool | Multi-Anbieter-Routing, Gesundheitsprüfung, Failover | Done |
| v0.10.7 | Vorschaumodus | Unauthentifizierter Zugriff, Feature-Gating | Done |
| v0.10.8 | Skill Store | Skill-Store-Infrastruktur, Kanalvalidierung | Done |
| v0.10.9–10 | Benutzerverwaltung | Unterbenutzer, seitenbasierte Berechtigungen | Done |
| v0.10.13–14 | Sicherheit & Skills | Sicherheitsseite, Skill-Store-Neugestaltung | Done |
| v0.10.15 | Chat-Verbesserungen | Chat-UX, Nachrichten-Pipeline | Done |
| v0.10.16 | Sprachmodul | Sherpa TTS/ASR, eSpeak, Anbieterwechsel | Done |
| v0.10.17 | Fernzugriff | Ngrok, Cloudflare-Tunnel, ACME-Zertifikate | Done |
| v0.10.18–20 | Leistungs-Sprint | Start-/Chat-Leistung, Kontext-Cache | Done |
| v0.10.21–22 | Prompt & DingTalk | System-Prompt, DingTalk-Kanal | Done |
| v0.10.23 | OTA-Update | OTA-Update-System | Done |
| v0.10.24 | Kanal-Upgrade | 10 Kanäle von Stubs aufgewertet | Done |
| v0.10.25 | CC Cache | Zweistufiger Cache (L1 Speicher + L2 Festplatte) | Done |
| v0.10.26 | Humanizer | Antwort-Humanisierungs-Pipeline | Done |
| v0.10.27 | Context Pruner | 54% Token-Einsparung bei Code (SWE-bench offiziell), 46–47% bei allgemeinen Dokumenten (lokales IR), BM25-Bewertung, Segmentierung | Done |
| v0.10.28 | Memory Service | Progressive Suche, Dual-Write-Backend | Done |

</details>

## Community & Support

- **Issues**: [Bitte melden Sie Fehler und Feature-Anfragen hier](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **Diskussionen**: [Discord](https://discord.gg/b3AgFDxe9v)
- **Folgen Sie uns** auf [GitHub](https://github.com/IceWhaleTech)

## Lizenz

Dieses Projekt ist unter der MIT-Lizenz lizenziert — siehe die [LICENSE](../../LICENSE)-Datei für Details. Wir glauben an Open Source und daran, der Community etwas zurückzugeben.

## Mitwirkende

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
