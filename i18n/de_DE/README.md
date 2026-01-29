# ZimaOS Echo

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Echo" width="200">
</p>

<p align="center">
  <a href="../README.md">English</a> |
  <a href="../zh_CN/README.md">简体中文</a> |
  <a href="../en_GB/README.md">English (UK)</a> |
  <a href="../es_ES/README.md">Español (ES)</a> |
  <a href="../fr_FR/README.md">Français</a> |
  <strong>Deutsch</strong> |
  <a href="../it_IT/README.md">Italiano</a> |
  <a href="../pt_PT/README.md">Português (PT)</a> |
  <a href="../pt_BR/README.md">Português (BR)</a> |
  <a href="../ru_RU/README.md">Русский</a> |
  <a href="../ja_JP/README.md">日本語</a> |
  <a href="../ko_KR/README.md">한국어</a> |
  <a href="../nl_NL/README.md">Nederlands</a> |
  <a href="../sv_SE/README.md">Svenska</a> |
  <a href="../da_DK/README.md">Dansk</a> |
  <a href="../fi_FI/README.md">Suomi</a> |
  <a href="../no_NO/README.md">Norsk</a> |
  <a href="../pl_PL/README.md">Polski</a> |
  <a href="../tr_TR/README.md">Türkçe</a> |
  <a href="../cs_CZ/README.md">Češtина</a> |
  <a href="../el_GR/README.md">Ελληνικά</a> |
  <a href="../ca_ES/README.md">Català</a> |
  <a href="../ga_IE/README.md">Gaeilge</a> |
  <a href="../ar_SA/README.md">العربية</a> |
  <a href="../hi_IN/README.md">हिन्दी</a> |
  <a href="../th_TH/README.md">ไทย</a> |
  <a href="../id_ID/README.md">Bahasa Indonesia</a> |
  <a href="../vi_VN/README.md">Tiếng Việt</a>
</p>

> Dies ist die deutsche Version der ZimaOS Echo-README.

Die vollständige Dokumentation findest du unter: https://echo.zimaos.com  
Der Rest dieses Dokuments folgt der Struktur der englischen Haupt-README. Siehe `../README.md` für die aktuellsten und detailliertesten Informationen.

# ZimaOS Echo

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Echo" width="200">
</p>

<p align="center">
  <strong>NAS-Native Agent-Laufzeitumgebung</strong>
</p>

<p align="center">
  <a href="../../README.md">English</a> | <a href="../zh_CN/README.md">中文</a> | <strong>Deutsch</strong>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Echo/ci.yml?branch=main&style=for-the-badge" alt="CI Status"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Echo?include_prereleases&style=for-the-badge" alt="GitHub Release"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT Lizenz"></a>
</p>

**ZimaOS Echo** ist eine leichtgewichtige, hochleistungsfähige KI-Agent-Laufzeitumgebung, die speziell für NAS- und Edge-Geräte entwickelt wurde. Mit Go gebaut, bietet sie eine produktionsreife Plattform für den Betrieb von KI-Assistenten auf stromsparender Hardware.

[Dokumentation](https://echo.zimaos.com) · [Schnellstart](#schnellstart) · [Funktionen](#funktionen) · [Vergleich](#vergleich-mit-clawdbot)

## Warum ZimaOS Echo?

ZimaOS Echo ist von [clawdbot](https://github.com/clawdbot/clawdbot) inspiriert, wurde aber von Grund auf in Go neu entwickelt für:

- **Geringerer Ressourcenverbrauch**: Läuft auf Geräten mit nur 256MB RAM
- **Bessere Leistung**: Native Go-Binärdatei mit effizienter Goroutine-basierter Nebenläufigkeit
- **Einfachere Bereitstellung**: Einzelne Binärdatei, keine Node.js-Laufzeit erforderlich
- **NAS-Optimierung**: Entwickelt für 24/7-Betrieb auf stromsparenden Geräten

## Schnellstart

### Linux / macOS

```bash
curl -fsSL https://echo.zimaos.com/install.sh | sudo bash
```

### Windows (PowerShell als Administrator)

```powershell
irm https://echo.zimaos.com/install.ps1 | iex
```

### Aus dem Quellcode

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo/server
go build -o zimaos-echo ./cmd/server
./zimaos-echo server
```

## Funktionen

### Kernfunktionen

- 🚀 **Leichtgewichtig**: Einzelne Binärdatei < 15MB, Speicher < 80MB
- ⚡ **Hohe Leistung**: Go-basiert mit Goroutine-Nebenläufigkeit
- 🔌 **Multi-Provider**: OpenAI, Anthropic, Ollama und mehr
- 🛡️ **Produktionsreif**: Circuit Breaker, elegante Degradation, automatische Wiederherstellung
- 📊 **Beobachtbar**: Prometheus-Metriken, pprof-Profiling, strukturiertes Logging
- 🔄 **Hot Reload**: Konfigurationsänderungen ohne Neustart
- 💾 **Backup/Wiederherstellung**: Automatisiertes Backup mit Point-in-Time-Wiederherstellung

### Smart Home Integration

- 🏠 **Home Assistant**: Native Integration mit Home Assistant API
- 💡 **Gerätesteuerung**: Lichter, Schalter, Sensoren, Klima und mehr
- 🤖 **KI-Automatisierung**: Natürlichsprachliche Befehle für Smart-Home-Steuerung
- 📡 **Echtzeit-Events**: Abonnieren von Gerätezustandsänderungen via WebSocket

### Sprachfähigkeiten

- 🎤 **Spracherkennung**: Whisper-basierte Sprache-zu-Text-Umwandlung
- 🔊 **Text-zu-Sprache**: Unterstützung mehrerer TTS-Engines
- 👂 **Sprachweckwort**: Anpassbare Aktivierungswort-Erkennung
- 🗣️ **Sprachbefehle**: Freihändige KI-Assistenten-Interaktion

### Multi-Tenant-Architektur

- 👥 **Tenant-Isolation**: Vollständige Daten- und Ressourcenisolation
- 🔐 **Pro-Tenant-Auth**: Unabhängige Authentifizierung pro Tenant
- 📊 **Ressourcenkontingente**: CPU-, Speicher- und API-Rate-Limits pro Tenant
- 🎛️ **Tenant-Dashboard**: Self-Service-Verwaltungsportal

### Kommunikationskanäle

- 💬 **Matrix-Protokoll**: Dezentralisierte, Ende-zu-Ende-verschlüsselte Nachrichten
- 📱 **Telegram/Discord/Slack**: Unterstützung beliebter Messaging-Plattformen
- 📞 **Signal/WhatsApp**: Sichere Messaging-Integration
- 🍎 **iMessage**: Native macOS iMessage-Unterstützung

### Sicherheit und Authentifizierung

- 🔑 **WebAuthn/Passkeys**: Passwortlose Authentifizierung mit FIDO2
- 🔐 **OIDC/OAuth 2.0**: Enterprise SSO (Google, GitHub, Okta, etc.)
- 📲 **MFA/TOTP**: Multi-Faktor-Authentifizierungsunterstützung
- 🛡️ **RBAC**: Feingranulare rollenbasierte Zugriffskontrolle
- 📝 **Audit-Logging**: Umfassender Sicherheits-Audit-Trail
- 🔒 **Sandbox**: Isolierte Ausführungsumgebung für Tools

### Frontend

- 🎨 **Vue 3 Dashboard**: Moderne, responsive Weboberfläche
- 💬 **Chat-Interface**: Streaming-Antworten mit Markdown-Unterstützung
- 📈 **Systemmonitor**: Echtzeit-Ressourcennutzungsdiagramme
- ⚙️ **Einstellungs-UI**: Einfache Konfigurationsverwaltung

## Vergleich mit Clawdbot

ZimaOS Echo ist von clawdbot inspiriert, aber für NAS/Edge-Bereitstellung optimiert:

| Funktion | ZimaOS Echo | Clawdbot |
|----------|-------------|----------|
| **Sprache** | Go | TypeScript/Node.js |
| **Binärgröße** | ~15MB | ~200MB+ (mit node_modules) |
| **Speicherverbrauch** | ~80MB im Leerlauf | ~200MB+ im Leerlauf |
| **Startzeit** | < 1s | 3-5s |
| **Laufzeit** | Native Binärdatei | Node.js erforderlich |
| **Zielplattform** | NAS/Edge-Geräte | Desktop/Server |

## Architektur

```
┌─────────────────────────────────────────────────┐
│                  ZimaOS-Echo                     │
├─────────────────────────────────────────────────┤
│  Vue 3 Frontend  │  REST API  │  WebSocket      │
├─────────────────────────────────────────────────┤
│              Kern-Laufzeit (Go)                  │
│  Event Loop │ Worker Pool │ Config │ Logger     │
├─────────────────────────────────────────────────┤
│              Agent-Laufzeit                      │
│  LLM Provider │ Tools │ Speicher │ Kontext      │
├─────────────────────────────────────────────────┤
│              Datenschicht                        │
│  SQLite (Zorm) │ ECache │ Dateien               │
└─────────────────────────────────────────────────┘
```

## Roadmap

- [x] **v0.1.0** - Kern-Laufzeit (Event Loop, Worker Pool, Config, Logger)
- [x] **v0.2.0** - Agent-Laufzeit (LLM-Provider inkl. AWS Bedrock, Tools, Speicher)
- [x] **v0.3.0** - API-Schicht (REST, WebSocket, Streaming)
- [x] **v0.4.0** - Plugin-System (Go-Module, WASM-Unterstützung)
- [x] **v0.5.0** - Produktionsreif (Metriken, Profiling, Backup)
- [x] **v0.6.0** - Nachrichtenkanäle (Telegram, Discord, Slack, WhatsApp, Signal, iMessage)
- [x] **v0.7.0** - Sicherheit (OIDC, MFA, WebAuthn, Audit-Logging, Sandbox)
- [x] **v0.8.0** - Leistung (ECache, Zorm, HTTP/2)
- [x] **v0.9.0** - Zukünftige Verbesserungen (A2UI, Browser-Automatisierung)
- [ ] **v1.0.0** - RAG & Wissensdatenbank

## Mitwirken

Beiträge sind willkommen! Bitte lesen Sie unseren [Beitragsleitfaden](CONTRIBUTING.md) für Details.

```bash
# Repository klonen
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo

# Abhängigkeiten installieren
cd server && go mod download

# Tests ausführen
go test ./...

# Bauen
go build -o zimaos-echo ./cmd/server
```

## Lizenz

MIT-Lizenz - siehe [LICENSE](LICENSE) für Details.

## Danksagungen

- [clawdbot](https://github.com/clawdbot/clawdbot) - Inspiration für das Projekt
- [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) - Leichtgewichtiges ORM

---

<p align="center">
  Mit ❤️ erstellt von <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
