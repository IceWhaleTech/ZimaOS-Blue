# ZimaOS Echo

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Echo" width="200">
</p>

<p align="center">
  <strong>Veilige, observeerbare AI-agent runtime</strong>
</p>

<p align="center">
  <a href="../../README.md">English</a> |
  <a href="../zh_CN/README.md">简体中文</a> |
  <a href="../zh_TW/README.md">繁體中文</a> |
  <a href="../ja_JP/README.md">日本語</a> |
  <a href="../ko_KR/README.md">한국어</a> |
  <a href="../de_DE/README.md">Deutsch</a> |
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
  <strong>Nederlands</strong> |
  <a href="../sv_SE/README.md">Svenska</a>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Echo/ci.yml?branch=main&style=for-the-badge" alt="CI status"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Echo?include_prereleases&style=for-the-badge" alt="GitHub release"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT License"></a>
</p>

**ZimaOS Echo** is een lichte, hoogpresterende AI-agent runtime voor NAS- en edge-apparaten. Gebouwd in Go, biedt het een productieklare omgeving met zero-config deployment, sessiemonitoring en gebruikersanalytics.

[Snel starten](#snel-starten) · [Functies](#kernfuncties)

## Highlights

| Specificatie | Waarde |
|------|------|
| **Binairgrootte** | ~40 MB (één uitvoerbaar bestand) |
| **Geheugen (inactief)** | ~4 MB |
| **Opstarttijd** | &lt; 1 s |
| **Afhankelijkheden** | Geen (zero-config deployment) |

## Kernfuncties

### Zero-config deployment

- **Eén binair**: download en voer uit, geen runtime-afhankelijkheden
- **Configuratie op verzoek**: werkt out-of-the-box, aanpasbaar wanneer nodig
- **Cross-platform**: Windows, macOS, Linux —zelfde binair,zelfde ervaring
- **Daemon-ondersteuning**: draaien als achtergrondservice

### Sessiemonitoring

- **Realtime sessietracking**: monitor alle actieve AI-sessies en hun status
- **Conversatiegeschiedenis**: volledig audittrail van alle interacties
- **Sessieherhaling**: bekijk en analyseer eerdere gesprekken
- **Multi-tenantisolatie**: volledige scheiding van sessies tussen gebruikers

### Aanroepketenoptimalisatie

- **Requesttracing**: end-to-end zicht op elke API-aanroep
- **Latentie-analyse**: knelpunten in de requestpipeline vinden
- **Providerrouting**: intelligente routing naar optimale LLM-providers
- **Circuit breaker**: automatische failover bij providerstoringen

### Gebruiksanalytics

- **Tokenverbruik**: gebruik per gebruiker, sessie en provider volgen
- **Kostentoewijzing**: gedetailleerde kostensplitsing per operatie
- **Rate limiting**: quotabeheer per tenant
- **Rapporten exporteren**: gebruiksrapporten in meerdere formaten genereren

### Beveiligingsverharding

- **Sandbox-uitvoering**: alle toolaanroepen in geïsoleerde omgevingen
- **RBAC**: fijnmazige rolgebaseerde toegangscontrole
- **WebAuthn/Passkeys**: wachtwoordloze FIDO2-authenticatie
- **MFA/TOTP**: multi-factorauthenticatie
- **Audittrail**: onveranderbare logs van alle bevoegde operaties

## Snel starten

```bash
# Uit broncode
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo
make build && ./dist/zimaos-echo server
```

Dashboard bereikbaar op `http://localhost:3000`.

## LLM-providerconfiguratie

ZimaOS Echo ondersteunt meerdere LLM-providers, inclusief lokale LLM-diensten:

```yaml
llm:
  # Cloudproviders
  provider: "openai"  # of "anthropic", "azure", enz.
  api_key: "your-api-key"

  # Lokaal LLM (optioneel)
  # provider: "ollama"
  # base_url: "http://localhost:11434"
```

## Architectuur

```
┌─────────────────────────────────────────────────────┐
│                    ZimaOS Echo                       │
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

## Observeerbaarheid

```yaml
# Volledige observeerbaarheidsstack inschakelen
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

### Geëxposeerde metrieken

- Requestlatentie (p50, p95, p99)
- LLM-tokengebruik per provider
- Slagings-/foutpercentages tooluitvoering
- Geheugen- en goroutineaantallen
- Circuit-breaker-toestandsovergangen

## Ontwikkelomgeving

### Vereisten

| Tool | Versie | Installatie |
|------|---------|---------|
| Go | 1.21+ | [golang.org](https://golang.org/dl/) |
| Node.js | 18+ | [nodejs.org](https://nodejs.org/) |
| Make | - | Voorgeïnstalleerd op macOS/Linux |

### Ontwikkelmodus (hot reload)

```bash
# Linux / macOS
./dev.sh

# Windows
dev.bat
```

- Frontend: `http://localhost:3000`
- Backend: `http://localhost:8080`

### Buildcommando's

```bash
make build              # Enkele binair (frontend ingesloten)
make build-embedded     # Build met ingesloten Claude Code CLI
make build-all          # Cross-compileren voor alle platformen
make clean              # Buildartefacten opschonen
```

### Projectstructuur

```
ZimaOS-Echo/
├── server/             # Go-backend
│   ├── cmd/echo/       # Entrypoint
│   └── internal/       # Kernmodules
├── web/                # Vue 3-frontend
│   └── src/
└── dist/               # Buildoutput
```

## Dankbetuigingen

- [clawdbot](https://github.com/clawdbot/clawdbot) – Inspiratie voor het project
- [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) – Lichtgewicht ORM

---

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
