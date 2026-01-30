# ZimaOS Echo

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Echo" width="200">
</p>

<p align="center">
  <strong>Säker, observerbar AI-agentkörningsmiljö</strong>
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
  <a href="../nl_NL/README.md">Nederlands</a> |
  <strong>Svenska</strong>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Echo/ci.yml?branch=main&style=for-the-badge" alt="CI status"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Echo?include_prereleases&style=for-the-badge" alt="GitHub release"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT License"></a>
</p>

**ZimaOS Echo** är en lätt, högpresterande AI-agentkörningsmiljö för NAS- och kantenheter. Byggd i Go, erbjuder den en produktionsklar plattform med nollkonfigurationsdistribution, sessionsövervakning och användningsanalys.

[Snabbstart](#snabbstart) · [Funktioner](#kärnfunktioner)

## Höjdpunkter

| Specifikation | Värde |
|------|------|
| **Binärstorlek** | ~40 MB (en körbar fil) |
| **Minne (vilande)** | ~4 MB |
| **Uppstartstid** | &lt; 1 s |
| **Beroenden** | Inga (nollkonfigurationsdistribution) |

## Kärnfunktioner

### Nollkonfigurationsdistribution

- **En binär**: ladda ner och kör, inga körningsberoenden
- **Konfiguration vid behov**: fungerar direkt, anpassa vid behov
- **Plattformsoberoende**: Windows, macOS, Linux — samma binär, samma upplevelse
- **Daemon-stöd**: kör som bakgrundstjänst

### Sessionsövervakning

- **Realtidssessionsspårning**: övervaka alla aktiva AI-sessioner och deras status
- **Konversationshistorik**: fullständig revisionsspår av alla interaktioner
- **Sessionsuppspelning**: granska och analysera tidigare konversationer
- **Multi-tenant-isolering**: fullständig sessionsseparation mellan användare

### Anropskedjeoptimering

- **Förfrågningsspårning**: slut-till-slut-synlighet för varje API-anrop
- **Latensanalys**: identifiera flaskhalsar i förfrågningspipelinen
- **Providerroutning**: intelligent routning till optimala LLM-providers
- **Kretsbrytare**: automatisk redundans vid providerfel

### Användningsanalys

- **Tokenförbrukning**: spåra användning per användare, session och provider
- **Kostnadstilldelning**: detaljerad kostnadsfördelning per operation
- **Hastighetsbegränsning**: kvothantering per tenant
- **Exportera rapporter**: generera användningsrapporter i flera format

### Säkerhetsförstärkning

- **Sandboxkörning**: alla verktygsanrop körs i isolerade miljöer
- **RBAC**: finmaskig rollbaserad åtkomstkontroll
- **WebAuthn/Passkeys**: lösenordsfri FIDO2-autentisering
- **MFA/TOTP**: multifaktorautentisering
- **Revisionsspår**: oföränderliga loggar av alla privilegierade operationer

## Snabbstart

```bash
# Från källkod
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo
make build && ./dist/zimaos-echo server
```

Åtkomst till instrumentpanelen på `http://localhost:3000`.

## LLM-providerkonfiguration

ZimaOS Echo stöder flera LLM-providers inklusive lokala LLM-tjänster:

```yaml
llm:
  # Molnproviders
  provider: "openai"  # eller "anthropic", "azure" m.m.
  api_key: "your-api-key"

  # Lokal LLM (valfritt)
  # provider: "ollama"
  # base_url: "http://localhost:11434"
```

## Arkitektur

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

## Observerbarhet

```yaml
# Aktivera full observerbarhetsstack
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

### Exponerade mätvärden

- Förfrågningslatens (p50, p95, p99)
- LLM-tokenanvändning per provider
- Lyckade/misslyckade verktygskörningar
- Minne- och goroutineantal
- Kretsbrytartillståndsövergångar

## Utvecklingsmiljö

### Förutsättningar

| Verktyg | Version | Installation |
|------|---------|---------|
| Go | 1.21+ | [golang.org](https://golang.org/dl/) |
| Node.js | 18+ | [nodejs.org](https://nodejs.org/) |
| Make | - | Förinstallerat på macOS/Linux |

### Utvecklingsläge (hot reload)

```bash
# Linux / macOS
./dev.sh

# Windows
dev.bat
```

- Frontend: `http://localhost:3000`
- Backend: `http://localhost:8080`

### Byggkommandon

```bash
make build              # En binär (frontend inbäddad)
make build-embedded     # Bygg med inbäddad Claude Code CLI
make build-all          # Korskompilera för alla plattformar
make clean              # Rensa byggartefakter
```

### Projektstruktur

```
ZimaOS-Echo/
├── server/             # Go-backend
│   ├── cmd/echo/       # Ingångspunkt
│   └── internal/       # Kärnmoduler
├── web/                # Vue 3-frontend
│   └── src/
└── dist/               # Byggutdata
```

## Tack

- [clawdbot](https://github.com/clawdbot/clawdbot) – Inspiration till projektet
- [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) – Lättvikts-ORM

---

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
