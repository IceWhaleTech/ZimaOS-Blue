# ZimaOS Echo

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Echo" width="200">
</p>

<p align="center">
  <strong>Sicher, beobachtbar, lokal-zuerst: AI-Agent-Runtime</strong>
</p>

<p align="center">
  <a href="../../README.md">English</a> |
  <a href="../zh_CN/README.md">简体中文</a> |
  <a href="../zh_TW/README.md">繁體中文</a> |
  <a href="../ja_JP/README.md">日本語</a> |
  <a href="../ko_KR/README.md">한국어</a> |
  <strong>Deutsch</strong> |
  <a href="../fr_FR/README.md">Français</a> |
  <a href="../es_ES/README.md">Español</a> |
  <a href="../it_IT/README.md">Italiano</a> |
  <a href="../pt_BR/README.md">Português</a> |
  <a href="../ru_RU/README.md">Русский</a> |
  <a href="../ar_SA/README.md">العربية</a> |
  <a href="../hi_IN/README.md">हिन्दी</a> |
  <a href="../th_TH/README.md">ไทย</a> |
  <a href="../vi_VN/README.md">Tiếng Việt</a> |
  <a href="../id_ID/README.md">Bahasa Indonesia</a> |
  <a href="../tr_TR/README.md">Türkçe</a> |
  <a href="../pl_PL/README.md">Polski</a> |
  <a href="../nl_NL/README.md">Nederlands</a> |
  <a href="../sv_SE/README.md">Svenska</a>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Echo/ci.yml?branch=main&style=for-the-badge" alt="CI status"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Echo?include_prereleases&style=for-the-badge" alt="GitHub release"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT License"></a>
</p>

**ZimaOS Echo** ist eine gehärtete AI-Agent-Runtime für NAS und Edge-Geräte. Deine Daten bleiben auf deiner Hardware, jede Aktion ist nachvollziehbar, und AI läuft in isolierten Sandboxes.

[Dokumentation](https://echo.zimaos.com) · [Schnellstart](#schnellstart) · [Funktionen](#kernprinzipien) · [Vergleich](#vergleich-mit-clawdbot)

## Warum ZimaOS Echo?

ZimaOS Echo ist von [clawdbot](https://github.com/clawdbot/clawdbot) inspiriert und in Go neu aufgebaut:

- **Geringerer Ressourcenverbrauch**: Läuft auf Geräten mit nur 256MB RAM
- **Bessere Leistung**: Natives Go-Binary, goroutine-basierte Nebenläufigkeit
- **Einfachere Bereitstellung**: Single Binary, kein Node.js nötig
- **NAS-Optimierung**: Für 24/7-Betrieb auf stromsparenden Geräten

## Kernprinzipien

### Lokal zuerst

- **Datenhoheit**: Alle Daten lokal auf deinem NAS – keine Cloud-Abhängigkeit
- **Ollama-Integration**: LLMs vollständig On-Device, null externe API-Aufrufe
- **Offline-fähig**: Kernfunktionen ohne Internet
- **Single Binary**: ~15 MB natives Go-Binary, keine Laufzeit-Abhängigkeiten

### Beobachtbar & auditierbar

- **Audit-Logging**: Jede AI-Aktion mit Kontext und Zeitstempel protokolliert
- **Prometheus-Metriken**: Echtzeit-Monitoring aller Systemoperationen
- **pprof-Profiling**: Einblick in CPU, Speicher, Goroutinen
- **Strukturierte Logs**: JSON-Logs für Parsing und Alerting

### Sicherheitshärtung

- **Sandbox-Ausführung**: Alle Tool-Aufrufe in isolierten Umgebungen
- **RBAC**: Feingranulare rollenbasierte Zugriffskontrolle
- **WebAuthn/Passkeys**: Passwortlose FIDO2-Authentifizierung
- **MFA/TOTP**: Multi-Faktor-Authentifizierung
- **OIDC/OAuth 2.0**: Enterprise-SSO-Anbindung
- **Circuit Breaker**: Automatische Fehlerisolation, Kaskadenausfälle verhindert

## Schnellstart

```bash
# Linux / macOS
curl -fsSL https://echo.zimaos.com/install.sh | sudo bash

# Aus Quellcode
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo/server
go build -o zimaos-echo ./cmd/server
./zimaos-echo server
```

## Sicherheitshärtung

### Authentifizierungs-Stack

| Schicht | Technologie | Zweck |
|--------|-------------|--------|
| Primär | WebAuthn/Passkeys | Phishing-resistente passwortlose Auth |
| Sekundär | TOTP/MFA | zeitbasierte Einmalpasswörter |
| Enterprise | OIDC/OAuth 2.0 | SSO mit Google, GitHub, Okta |
| Autorisierung | RBAC | Ressourcenbezogene Berechtigungen |

### Laufzeitschutz

- **Sandbox-Isolation**: Tool-Ausführung in eingeschränkter Umgebung
- **Rate Limiting**: API-Drosselung pro Tenant
- **Tenant-Isolation**: Vollständige Daten- und Ressourcentrennung
- **Audit Trail**: Unveränderliche Logs privilegierter Operationen

### Resilienz

- **Circuit Breaker**: Automatische Service-Isolation bei Ausfällen
- **Graceful Degradation**: Fallback-Strategien bei Provider-Ausfällen
- **LLM-Fallback-Kette**: Automatischer Provider-Wechsel
- **Hot Reload**: Konfigurationsänderungen ohne Neustart

## Beobachtbarkeit

```yaml
# Vollständigen Observability-Stack aktivieren
metrics:
  enabled: true
  endpoint: "/metrics"

profiling:
  enabled: true
  endpoint_prefix: "/debug/pprof"

audit:
  enabled: true
  retention_days: 90
```

### Exponierte Metriken

- Anfrage-Latenz (p50, p95, p99)
- LLM-Token-Nutzung pro Provider
- Tool-Erfolgs-/Fehlerraten
- Speicher- und Goroutine-Zählung
- Circuit-Breaker-Zustandsübergänge

## Architektur

```
┌─────────────────────────────────────────────────────┐
│                    ZimaOS Echo                       │
├─────────────────────────────────────────────────────┤
│  Audit Log  │  Metrics  │  RBAC  │  Rate Limiter   │
├─────────────────────────────────────────────────────┤
│              Sandbox Execution Layer                 │
│         Tool Isolation │ Resource Limits            │
├─────────────────────────────────────────────────────┤
│              Agent Runtime (Go)                      │
│  LLM Provider │ Tools │ Memory │ Circuit Breaker   │
├─────────────────────────────────────────────────────┤
│              Local Data Layer                        │
│  SQLite │ ECache │ Encrypted Storage                │
└─────────────────────────────────────────────────────┘
```

## Lokales LLM-Setup (Ollama)

AI vollständig offline, ohne externe API-Aufrufe:

```bash
# Ollama installieren
curl -fsSL https://ollama.com/install.sh | sh

# Modell laden
ollama pull llama3.2

# Echo für lokales LLM konfigurieren
cat >> config.yaml << EOF
llm:
  provider: "ollama"
  model: "llama3.2"
  base_url: "http://localhost:11434"
EOF
```

## Entwicklungsumgebung

### Voraussetzungen

| Tool | Version | Installieren |
|------|---------|--------------|
| Go | 1.21+ | [golang.org](https://golang.org/dl/) |
| Node.js | 18+ | [nodejs.org](https://nodejs.org/) |
| Make | - | Unter macOS/Linux i. d. R. vorhanden |

### Ein-Befehl-Start

```bash
# Klonen und starten
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo
make build && ./dist/zimaos-echo
```

Dashboard unter `http://localhost:3000`

### Entwicklungsmodus (Hot Reload)

```bash
# Linux / macOS
./dev.sh

# Windows
dev.bat
```

- Frontend: `http://localhost:5173` (API-Proxy zum Backend)
- Backend: `http://localhost:8080`

### Build-Befehle

```bash
make build              # Einzelbinary (Frontend eingebettet)
make build-embedded     # Build mit eingebettetem Claude Code CLI
make build-all          # Cross-Compile für alle Plattformen
make clean              # Build-Artefakte löschen
```

### Projektstruktur

```
ZimaOS-Echo/
├── server/             # Go-Backend
│   ├── cmd/echo/       # Einstiegspunkt
│   └── internal/       # Kernmodule
├── web/                # Vue-3-Frontend
│   └── src/
└── dist/               # Build-Output
```

## Vergleich mit Clawdbot

ZimaOS Echo ist von clawdbot inspiriert, aber für NAS/Edge-Bereitstellung optimiert:

| Merkmal | ZimaOS Echo | Clawdbot |
|---------|-------------|----------|
| **Sprache** | Go | TypeScript/Node.js |
| **Binärgröße** | ~15MB | ~200MB+ (mit node_modules) |
| **Speicherverbrauch** | ~80MB Leerlauf | ~200MB+ Leerlauf |
| **Startzeit** | < 1s | 3–5s |
| **Laufzeit** | Native Binärdatei | Node.js erforderlich |
| **Zielplattform** | NAS/Edge-Geräte | Desktop/Server |

## Danksagungen

- [clawdbot](https://github.com/clawdbot/clawdbot) - Inspiration für das Projekt
- [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) - Leichtgewichtiges ORM

---

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
