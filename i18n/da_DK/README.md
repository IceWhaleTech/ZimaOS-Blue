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
  <a href="../de_DE/README.md">Deutsch</a> |
  <a href="../it_IT/README.md">Italiano</a> |
  <a href="../pt_PT/README.md">Português (PT)</a> |
  <a href="../pt_BR/README.md">Português (BR)</a> |
  <a href="../ru_RU/README.md">Русский</a> |
  <a href="../ja_JP/README.md">日本語</a> |
  <a href="../ko_KR/README.md">한국어</a> |
  <a href="../nl_NL/README.md">Nederlands</a> |
  <a href="../sv_SE/README.md">Svenska</a> |
  <strong>Dansk</strong> |
  <a href="../fi_FI/README.md">Suomi</a> |
  <a href="../no_NO/README.md">Norsk</a> |
  <a href="../pl_PL/README.md">Polski</a> |
  <a href="../tr_TR/README.md">Türkçe</a> |
  <a href="../cs_CZ/README.md">Čeština</a> |
  <a href="../el_GR/README.md">Ελληνικά</a> |
  <a href="../ca_ES/README.md">Català</a> |
  <a href="../ga_IE/README.md">Gaeilge</a> |
  <a href="../ar_SA/README.md">العربية</a> |
  <a href="../hi_IN/README.md">हिन्दी</a> |
  <a href="../th_TH/README.md">ไทย</a> |
  <a href="../id_ID/README.md">Bahasa Indonesia</a> |
  <a href="../vi_VN/README.md">Tiếng Việt</a>
</p>

> Dette er den danske version af ZimaOS Echo README.

For fuld dokumentation, besøg: https://echo.zimaos.com  
Resten af dette dokument følger strukturen i den engelske hoved-README. Se `../README.md` for den nyeste og mest detaljerede information.

# ZimaOS Echo

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Echo" width="200">
</p>

<p align="center">
  <strong>NAS-Native Agent Runtime</strong>
</p>

<p align="center">
  <a href="../../README.md">English</a> | <a href="../zh_CN/README.md">中文</a> | <strong>Dansk</strong>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Echo/ci.yml?branch=main&style=for-the-badge" alt="CI status"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Echo?include_prereleases&style=for-the-badge" alt="GitHub udgivelse"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT Licens"></a>
</p>

**ZimaOS Echo** er en letvægts, højtydende AI-agent runtime designet specifikt til NAS og edge-enheder. Bygget med Go, leverer den en produktionsklar platform til at køre AI-assistenter på lavenergi-hardware.

[Dokumentation](https://echo.zimaos.com) · [Hurtig Start](#hurtig-start) · [Funktioner](#funktioner) · [Sammenligning](#sammenligning-med-clawdbot)

## Hvorfor ZimaOS Echo?

ZimaOS Echo er inspireret af [clawdbot](https://github.com/clawdbot/clawdbot) men genopbygget fra bunden i Go for:

- **Lavere Ressourceforbrug**: Kører på enheder med kun 256MB RAM
- **Bedre Ydeevne**: Native Go-binær med effektiv goroutine-baseret samtidighed
- **Nemmere Udrulning**: Enkelt binær, ingen Node.js runtime påkrævet
- **NAS-Optimering**: Designet til 24/7 drift på lavenergi-enheder

## Hurtig Start

### Linux / macOS

```bash
curl -fsSL https://echo.zimaos.com/install.sh | sudo bash
```

### Windows (PowerShell som Administrator)

```powershell
irm https://echo.zimaos.com/install.ps1 | iex
```

### Fra Kildekode

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo/server
go build -o zimaos-echo ./cmd/server
./zimaos-echo server
```

## Funktioner

### Kernefunktioner

- 🚀 **Letvægts**: Enkelt binær < 15MB, hukommelse < 80MB
- ⚡ **Høj Ydeevne**: Go-baseret med goroutine-samtidighed
- 🔌 **Multi-Provider**: OpenAI, Anthropic, Ollama og mere
- 🛡️ **Produktionsklar**: Circuit breaker, elegant nedgradering, automatisk gendannelse
- 📊 **Observerbar**: Prometheus-metrikker, pprof-profilering, struktureret logning
- 🔄 **Hot Reload**: Konfigurationsændringer uden genstart
- 💾 **Backup/Gendannelse**: Automatiseret backup med punkt-i-tid gendannelse

### Smart Home Integration

- 🏠 **Home Assistant**: Native integration med Home Assistant API
- 💡 **Enhedskontrol**: Lys, kontakter, sensorer, klima og mere
- 🤖 **AI-Automatisering**: Naturlige sprogkommandoer til smart home-kontrol
- 📡 **Realtidshændelser**: Abonner på enhedstilstandsændringer via WebSocket

### Stemmefunktioner

- 🎤 **Talegenkendelse**: Whisper-baseret tale-til-tekst
- 🔊 **Tekst-til-Tale**: Understøttelse af flere TTS-motorer
- 👂 **Stemmeaktivering**: Tilpasselig aktiveringsord-detektion
- 🗣️ **Stemmekommandoer**: Håndfri AI-assistent interaktion

### Multi-Tenant Arkitektur

- 👥 **Tenant-Isolation**: Komplet data- og ressourceisolation
- 🔐 **Per-Tenant Auth**: Uafhængig autentificering per tenant
- 📊 **Ressourcekvoter**: CPU, hukommelse og API-hastighedsgrænser per tenant
- 🎛️ **Tenant Dashboard**: Selvbetjeningsportal

### Kommunikationskanaler

- 💬 **Matrix-Protokol**: Decentraliseret, ende-til-ende krypteret beskedudveksling
- 📱 **Telegram/Discord/Slack**: Understøttelse af populære beskedplatforme
- 📞 **Signal/WhatsApp**: Sikker beskedintegration
- 🍎 **iMessage**: Native macOS iMessage-understøttelse

### Sikkerhed og Autentificering

- 🔑 **WebAuthn/Passkeys**: Adgangskodefri autentificering med FIDO2
- 🔐 **OIDC/OAuth 2.0**: Enterprise SSO (Google, GitHub, Okta, osv.)
- 📲 **MFA/TOTP**: Multi-faktor autentificeringsunderstøttelse
- 🛡️ **RBAC**: Finkornet rollebaseret adgangskontrol
- 📝 **Audit Logning**: Omfattende sikkerhedsauditspor
- 🔒 **Sandbox**: Isoleret eksekveringsmiljø for værktøjer

### Frontend

- 🎨 **Vue 3 Dashboard**: Moderne, responsivt webinterface
- 💬 **Chat Interface**: Streaming-svar med Markdown-understøttelse
- 📈 **Systemmonitor**: Realtids ressourceforbrugsgrafer
- ⚙️ **Indstillinger UI**: Nem konfigurationsstyring

## Sammenligning med Clawdbot

ZimaOS Echo er inspireret af clawdbot men optimeret til NAS/edge-udrulning:

| Funktion | ZimaOS Echo | Clawdbot |
|----------|-------------|----------|
| **Sprog** | Go | TypeScript/Node.js |
| **Binær Størrelse** | ~15MB | ~200MB+ (med node_modules) |
| **Hukommelsesforbrug** | ~80MB inaktiv | ~200MB+ inaktiv |
| **Opstartstid** | < 1s | 3-5s |
| **Runtime** | Native binær | Kræver Node.js |
| **Målplatform** | NAS/Edge-enheder | Desktop/Server |

## Arkitektur

```
┌─────────────────────────────────────────────────┐
│                  ZimaOS-Echo                     │
├─────────────────────────────────────────────────┤
│  Vue 3 Frontend  │  REST API  │  WebSocket      │
├─────────────────────────────────────────────────┤
│              Kerne Runtime (Go)                  │
│  Event Loop │ Worker Pool │ Config │ Logger     │
├─────────────────────────────────────────────────┤
│              Agent Runtime                       │
│  LLM Provider │ Værktøjer │ Hukommelse │ Kontekst│
├─────────────────────────────────────────────────┤
│              Datalag                             │
│  SQLite (Zorm) │ ECache │ Filer                 │
└─────────────────────────────────────────────────┘
```

## Køreplan

- [x] **v0.1.0** - Kerne Runtime (Event loop, Worker pool, Config, Logger)
- [x] **v0.2.0** - Agent Runtime (LLM-providere inkl. AWS Bedrock, Værktøjer, Hukommelse)
- [x] **v0.3.0** - API-Lag (REST, WebSocket, Streaming)
- [x] **v0.4.0** - Plugin-System (Go-moduler, WASM-understøttelse)
- [x] **v0.5.0** - Produktionsklar (Metrikker, Profilering, Backup)
- [x] **v0.6.0** - Beskedkanaler (Telegram, Discord, Slack, WhatsApp, Signal, iMessage)
- [x] **v0.7.0** - Sikkerhed (OIDC, MFA, WebAuthn, Audit-logning, Sandbox)
- [x] **v0.8.0** - Ydeevne (ECache, Zorm, HTTP/2)
- [x] **v0.9.0** - Fremtidige Forbedringer (A2UI, Browser-Automatisering)
- [ ] **v1.0.0** - RAG & Vidensbase

## Bidrag

Bidrag er velkomne! Læs venligst vores [Bidragsguide](CONTRIBUTING.md) for detaljer.

```bash
# Klon repository
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo

# Installer afhængigheder
cd server && go mod download

# Kør tests
go test ./...

# Byg
go build -o zimaos-echo ./cmd/server
```

## Licens

MIT Licens - se [LICENSE](LICENSE) for detaljer.

## Anerkendelser

- [clawdbot](https://github.com/clawdbot/clawdbot) - Inspiration til projektet
- [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) - Letvægts ORM

---

<p align="center">
  Lavet med ❤️ af <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
