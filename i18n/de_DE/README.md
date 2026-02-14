# ZimaOS Blue

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Blue" width="200">
</p>

<p align="center">
  <strong>Sicherer, beobachtbarer AI-Agent-Runtime</strong>
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
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Blue/ci.yml?branch=main&style=for-the-badge" alt="CI status"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Blue?include_prereleases&style=for-the-badge" alt="GitHub release"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT License"></a>
</p>

**ZimaOS Blue** ist eine schlanke, leistungsstarke AI-Agent-Runtime für NAS- und Edge-Geräte. In Go entwickelt, bietet sie eine produktionsreife Plattform mit Zero-Config-Deployment, Sitzungsüberwachung und Nutzungsanalysen.

[Schnellstart](#schnellstart) · [Funktionen](#kernfunktionen)

## Highlights

| Spezifikation | Wert |
|------|------|
| **Binärgröße** | ~40MB (einzelne ausführbare Datei) |
| **Speicher (Leerlauf)** | ~4MB |
| **Startzeit** | < 1s |
| **Abhängigkeiten** | Keine (Zero-Config-Deployment) |

## Kernfunktionen

### Zero-Config-Deployment

- **Einzelne Binärdatei**: Herunterladen und ausführen – keine Laufzeit-Abhängigkeiten
- **Konfiguration bei Bedarf**: Sofort einsatzbereit, bei Bedarf anpassbar
- **Plattformübergreifend**: Windows, macOS, Linux – dieselbe Binärdatei, dieselbe Erfahrung
- **Daemon-Unterstützung**: Als dauerhafter Hintergrunddienst ausführbar

### Sitzungsüberwachung

- **Echtzeit-Sitzungsverfolgung**: Alle aktiven AI-Sitzungen mit Live-Status überwachen
- **Konversationsverlauf**: Vollständige Prüfprotokolle aller Interaktionen
- **Sitzungswiedergabe**: Vergangene Konversationen prüfen und analysieren
- **Multi-Tenant-Isolation**: Vollständige Sitzungstrennung zwischen Nutzern

### Aufrufketten-Optimierung

- **Anfrage-Tracing**: End-to-End-Sichtbarkeit jedes API-Aufrufs
- **Latenzanalyse**: Engpässe in der Anfrage-Pipeline identifizieren
- **Provider-Routing**: Intelligentes Routing zu optimalen LLM-Providern
- **Circuit Breaker**: Automatisches Failover bei Provider-Ausfällen

### Nutzungsanalysen

- **Token-Verbrauch**: Nutzung pro Nutzer, Sitzung und Provider verfolgen
- **Kostenzuordnung**: Detaillierte Kostenaufschlüsselung pro Vorgang
- **Ratenbegrenzung**: Mandantenbezogenes Quoten-Management
- **Berichte exportieren**: Nutzungsberichte in mehreren Formaten erstellen

### Sicherheitshärtung

- **Sandbox-Ausführung**: Alle Tool-Aufrufe laufen in isolierten Umgebungen
- **RBAC**: Fein abgestufte rollenbasierte Zugriffskontrolle
- **WebAuthn/Passkeys**: Passwortlose FIDO2-Authentifizierung
- **MFA/TOTP**: Multi-Faktor-Authentifizierung
- **Prüfprotokoll**: Unveränderliche Protokolle aller privilegierten Vorgänge

## Schnellstart

```bash
# Aus Quellcode
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue
make build && ./dist/zimaos-blue server
```

Dashboard unter `http://localhost:3000` aufrufen.

## LLM-Provider-Konfiguration

ZimaOS Blue unterstützt mehrere LLM-Provider inkl. lokaler LLM-Dienste:

```yaml
llm:
  # Cloud-Provider
  provider: "openai"  # oder "anthropic", "azure" usw.
  api_key: "your-api-key"

  # Lokales LLM (optional)
  # provider: "ollama"
  # base_url: "http://localhost:11434"
```

## Architektur

```
┌─────────────────────────────────────────────────────┐
│                    ZimaOS Blue                       │
├─────────────────────────────────────────────────────┤
│  Session Monitor │ Usage Analytics │ Call Tracing  │
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

## Beobachtbarkeit

```yaml
# Vollständigen Beobachtbarkeits-Stack aktivieren
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

### Bereitgestellte Metriken

- Anfrage-Latenz (p50, p95, p99)
- LLM-Token-Nutzung pro Provider
- Erfolgs-/Fehlerrate der Tool-Ausführung
- Speicher- und Goroutine-Anzahl
- Circuit-Breaker-Zustandsübergänge

## Entwicklungsumgebung

### Voraussetzungen

| Tool | Version | Installation |
|------|---------|---------|
| Go | 1.21+ | [golang.org](https://golang.org/dl/) |
| Node.js | 18+ | [nodejs.org](https://nodejs.org/) |
| Make | - | Unter macOS/Linux vorinstalliert |

### Entwicklungsmodus (Hot Reload)

```bash
# Linux / macOS
./dev.sh

# Windows
dev.bat
```

- Frontend: `http://localhost:3000`
- Backend: `http://localhost:23456`

### Build-Befehle

```bash
make build              # Einzelne Binärdatei (Frontend eingebettet)
make build-embedded     # Mit eingebettetem Claude Code CLI bauen
make build-all          # Für alle Plattformen cross-kompilieren
make clean              # Build-Artefakte bereinigen
```

### Projektstruktur

```
ZimaOS-Blue/
├── server/             # Go-Backend
│   ├── cmd/blue/       # Einstiegspunkt
│   └── internal/       # Kernmodule
├── web/                # Vue-3-Frontend
│   └── src/
└── dist/               # Build-Ausgabe
```

## Danksagungen

- [clawdbot](https://github.com/clawdbot/clawdbot) – Inspiration für das Projekt
- [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) – Leichtgewichtiges ORM

---

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
