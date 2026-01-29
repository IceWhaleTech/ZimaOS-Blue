# ZimaOS Echo

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Echo" width="200">
</p>

<p align="center">
  <strong>Entorn d'Execució d'Agents Natiu per a NAS</strong>
</p>

<p align="center">
  <a href="../../README.md">English</a> | <a href="../zh_CN/README.md">中文</a> | <strong>Català</strong>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Echo/ci.yml?branch=main&style=for-the-badge" alt="Estat CI"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Echo?include_prereleases&style=for-the-badge" alt="Versió GitHub"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="Llicència MIT"></a>
</p>

**ZimaOS Echo** és un entorn d'execució d'agents d'IA lleuger i d'alt rendiment dissenyat específicament per a dispositius NAS i edge. Construït amb Go, proporciona una plataforma preparada per a producció per executar assistents d'IA en maquinari de baix consum.

[Documentació](https://echo.zimaos.com) · [Inici Ràpid](#inici-ràpid) · [Característiques](#característiques) · [Comparació](#comparació-amb-clawdbot)

## Per què ZimaOS Echo?

ZimaOS Echo està inspirat en [clawdbot](https://github.com/clawdbot/clawdbot) però reconstruït des de zero en Go per:

- **Menor Ús de Recursos**: Funciona en dispositius amb només 256MB de RAM
- **Millor Rendiment**: Binari natiu de Go amb concurrència eficient basada en goroutines
- **Desplegament Més Fàcil**: Un sol binari, no requereix Node.js
- **Optimització per NAS**: Dissenyat per a operació 24/7 en dispositius de baix consum

## Inici Ràpid

### Linux / macOS

```bash
curl -fsSL https://echo.zimaos.com/install.sh | sudo bash
```

### Windows (PowerShell com a Administrador)

```powershell
irm https://echo.zimaos.com/install.ps1 | iex
```

### Des del Codi Font

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo/server
go build -o zimaos-echo ./cmd/server
./zimaos-echo server
```

## Característiques

### Característiques Principals

- 🚀 **Lleuger**: Un sol binari < 15MB, memòria < 80MB
- ⚡ **Alt Rendiment**: Basat en Go amb concurrència de goroutines
- 🔌 **Multi-Proveïdor**: OpenAI, Anthropic, Ollama i més
- 🛡️ **Preparat per Producció**: Circuit breaker, degradació elegant, recuperació automàtica
- 📊 **Observable**: Mètriques Prometheus, perfilat pprof, registre estructurat
- 🔄 **Recàrrega en Calent**: Canvis de configuració sense reiniciar
- 💾 **Còpia de Seguretat/Restauració**: Còpia de seguretat automatitzada amb restauració a un punt en el temps

### Integració de Casa Intel·ligent

- 🏠 **Home Assistant**: Integració nativa amb l'API de Home Assistant
- 💡 **Control de Dispositius**: Llums, interruptors, sensors, clima i més
- 🤖 **Automatització IA**: Comandes en llenguatge natural per al control de la casa intel·ligent
- 📡 **Esdeveniments en Temps Real**: Subscripció a canvis d'estat de dispositius via WebSocket

### Capacitats de Veu

- 🎤 **Reconeixement de Veu**: Conversió de veu a text basada en Whisper
- 🔊 **Text a Veu**: Suport per a múltiples motors TTS
- 👂 **Activació per Veu**: Detecció de paraula d'activació personalitzable
- 🗣️ **Comandes de Veu**: Interacció amb l'assistent d'IA sense mans

### Arquitectura Multi-Inquilí

- 👥 **Aïllament d'Inquilins**: Aïllament complet de dades i recursos
- 🔐 **Autenticació per Inquilí**: Autenticació independent per inquilí
- 📊 **Quotes de Recursos**: Límits de CPU, memòria i taxa d'API per inquilí
- 🎛️ **Tauler d'Inquilí**: Portal de gestió d'autoservei

### Canals de Comunicació

- 💬 **Protocol Matrix**: Missatgeria descentralitzada i xifrada d'extrem a extrem
- 📱 **Telegram/Discord/Slack**: Suport per a plataformes de missatgeria populars
- 📞 **Signal/WhatsApp**: Integració de missatgeria segura
- 🍎 **iMessage**: Suport natiu d'iMessage per a macOS

### Seguretat i Autenticació

- 🔑 **WebAuthn/Passkeys**: Autenticació sense contrasenya amb FIDO2
- 🔐 **OIDC/OAuth 2.0**: SSO empresarial (Google, GitHub, Okta, etc.)
- 📲 **MFA/TOTP**: Suport per a autenticació multifactor
- 🛡️ **RBAC**: Control d'accés basat en rols detallat
- 📝 **Registre d'Auditoria**: Traça d'auditoria de seguretat completa
- 🔒 **Sandbox**: Entorn d'execució aïllat per a eines

### Frontend

- 🎨 **Tauler Vue 3**: Interfície web moderna i responsiva
- 💬 **Interfície de Xat**: Respostes en streaming amb suport Markdown
- 📈 **Monitor del Sistema**: Gràfics d'ús de recursos en temps real
- ⚙️ **UI de Configuració**: Gestió de configuració fàcil

## Comparació amb Clawdbot

ZimaOS Echo està inspirat en clawdbot però optimitzat per a desplegament NAS/edge:

| Característica | ZimaOS Echo | Clawdbot |
|----------------|-------------|----------|
| **Llenguatge** | Go | TypeScript/Node.js |
| **Mida del Binari** | ~15MB | ~200MB+ (amb node_modules) |
| **Ús de Memòria** | ~80MB inactiu | ~200MB+ inactiu |
| **Temps d'Inici** | < 1s | 3-5s |
| **Entorn d'Execució** | Binari natiu | Requereix Node.js |
| **Plataforma Objectiu** | Dispositius NAS/Edge | Escriptori/Servidor |

## Arquitectura

```
┌─────────────────────────────────────────────────┐
│                  ZimaOS-Echo                     │
├─────────────────────────────────────────────────┤
│  Frontend Vue 3  │  REST API  │  WebSocket      │
├─────────────────────────────────────────────────┤
│              Entorn d'Execució Principal (Go)    │
│  Bucle d'Esdeveniments │ Pool de Treballadors │ Config │ Logger │
├─────────────────────────────────────────────────┤
│              Entorn d'Execució d'Agents          │
│  Proveïdor LLM │ Eines │ Memòria │ Context      │
├─────────────────────────────────────────────────┤
│              Capa de Dades                       │
│  SQLite (Zorm) │ ECache │ Fitxers               │
└─────────────────────────────────────────────────┘
```

## Full de Ruta

- [x] **v0.1.0** - Entorn d'Execució Principal (Bucle d'esdeveniments, Pool de treballadors, Config, Logger)
- [x] **v0.2.0** - Entorn d'Execució d'Agents (Proveïdors LLM incl. AWS Bedrock, Eines, Memòria)
- [x] **v0.3.0** - Capa API (REST, WebSocket, Streaming)
- [x] **v0.4.0** - Sistema de Plugins (Mòduls Go, suport WASM)
- [x] **v0.5.0** - Preparat per Producció (Mètriques, Perfilat, Còpia de seguretat)
- [x] **v0.6.0** - Canals de Missatgeria (Telegram, Discord, Slack, WhatsApp, Signal, iMessage)
- [x] **v0.7.0** - Seguretat (OIDC, MFA, WebAuthn, Registre d'auditoria, Sandbox)
- [x] **v0.8.0** - Rendiment (ECache, Zorm, HTTP/2)
- [x] **v0.9.0** - Millores Futures (A2UI, Automatització del Navegador)
- [ ] **v1.0.0** - RAG i Base de Coneixement

## Contribuir

Les contribucions són benvingudes! Si us plau, llegiu la nostra [Guia de Contribució](CONTRIBUTING.md) per a més detalls.

```bash
# Clonar el repositori
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo

# Instal·lar dependències
cd server && go mod download

# Executar tests
go test ./...

# Compilar
go build -o zimaos-echo ./cmd/server
```

## Llicència

Llicència MIT - vegeu [LICENSE](LICENSE) per a més detalls.

## Agraïments

- [clawdbot](https://github.com/clawdbot/clawdbot) - Inspiració per al projecte
- [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) - ORM lleuger

---

<p align="center">
  Fet amb ❤️ per <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
