# ZimaOS Echo

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Echo" width="200">
</p>

<p align="center">
  <strong>Nativní Runtime pro AI Agenty na NAS</strong>
</p>

<p align="center">
  <a href="../../README.md">English</a> | <a href="../zh_CN/README.md">中文</a> | <strong>Čeština</strong>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Echo/ci.yml?branch=main&style=for-the-badge" alt="Stav CI"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Echo?include_prereleases&style=for-the-badge" alt="GitHub verze"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT Licence"></a>
</p>

**ZimaOS Echo** je lehký, vysoce výkonný runtime pro AI agenty navržený speciálně pro NAS a edge zařízení. Postaven v Go, poskytuje produkčně připravenou platformu pro provoz AI asistentů na nízkoenergetickém hardwaru.

[Dokumentace](https://echo.zimaos.com) · [Rychlý Start](#rychlý-start) · [Funkce](#funkce) · [Srovnání](#srovnání-s-clawdbot)

## Proč ZimaOS Echo?

ZimaOS Echo je inspirován projektem [clawdbot](https://github.com/clawdbot/clawdbot), ale přestavěn od základu v Go pro:

- **Nižší Spotřebu Zdrojů**: Běží na zařízeních s pouhými 256MB RAM
- **Lepší Výkon**: Nativní Go binárka s efektivní konkurencí založenou na goroutinách
- **Jednodušší Nasazení**: Jediná binárka, není potřeba Node.js
- **Optimalizace pro NAS**: Navrženo pro nepřetržitý provoz na nízkoenergetických zařízeních

## Rychlý Start

### Linux / macOS

```bash
curl -fsSL https://echo.zimaos.com/install.sh | sudo bash
```

### Windows (PowerShell jako Administrátor)

```powershell
irm https://echo.zimaos.com/install.ps1 | iex
```

### Ze Zdrojového Kódu

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo/server
go build -o zimaos-echo ./cmd/server
./zimaos-echo server
```

## Funkce

### Základní Funkce

- 🚀 **Lehký**: Jediná binárka < 15MB, paměť < 80MB
- ⚡ **Vysoký Výkon**: Založeno na Go s konkurencí goroutin
- 🔌 **Multi-Provider**: OpenAI, Anthropic, Ollama a další
- 🛡️ **Připraveno pro Produkci**: Circuit breaker, elegantní degradace, automatické obnovení
- 📊 **Pozorovatelný**: Prometheus metriky, pprof profilování, strukturované logování
- 🔄 **Hot Reload**: Změny konfigurace bez restartu
- 💾 **Záloha/Obnova**: Automatizované zálohování s obnovou k určitému bodu v čase

### Integrace Chytré Domácnosti

- 🏠 **Home Assistant**: Nativní integrace s Home Assistant API
- 💡 **Ovládání Zařízení**: Světla, vypínače, senzory, klimatizace a další
- 🤖 **AI Automatizace**: Příkazy v přirozeném jazyce pro ovládání chytré domácnosti
- 📡 **Události v Reálném Čase**: Odběr změn stavu zařízení přes WebSocket

### Hlasové Schopnosti

- 🎤 **Rozpoznávání Řeči**: Převod řeči na text založený na Whisper
- 🔊 **Text na Řeč**: Podpora více TTS enginů
- 👂 **Hlasové Probuzení**: Přizpůsobitelná detekce aktivačního slova
- 🗣️ **Hlasové Příkazy**: Interakce s AI asistentem bez použití rukou

### Multi-Tenant Architektura

- 👥 **Izolace Tenantů**: Kompletní izolace dat a zdrojů
- 🔐 **Autentizace per Tenant**: Nezávislá autentizace pro každého tenanta
- 📊 **Kvóty Zdrojů**: Limity CPU, paměti a API rate pro každého tenanta
- 🎛️ **Tenant Dashboard**: Samoobslužný portál pro správu

### Komunikační Kanály

- 💬 **Matrix Protokol**: Decentralizované, end-to-end šifrované zprávy
- 📱 **Telegram/Discord/Slack**: Podpora populárních komunikačních platforem
- 📞 **Signal/WhatsApp**: Integrace zabezpečených zpráv
- 🍎 **iMessage**: Nativní podpora iMessage pro macOS

### Bezpečnost a Autentizace

- 🔑 **WebAuthn/Passkeys**: Autentizace bez hesla s FIDO2
- 🔐 **OIDC/OAuth 2.0**: Podnikové SSO (Google, GitHub, Okta, atd.)
- 📲 **MFA/TOTP**: Podpora vícefaktorové autentizace
- 🛡️ **RBAC**: Detailní řízení přístupu založené na rolích
- 📝 **Audit Log**: Kompletní bezpečnostní audit trail
- 🔒 **Sandbox**: Izolované prostředí pro spouštění nástrojů

### Frontend

- 🎨 **Vue 3 Dashboard**: Moderní, responzivní webové rozhraní
- 💬 **Chat Rozhraní**: Streamované odpovědi s podporou Markdown
- 📈 **Systémový Monitor**: Grafy využití zdrojů v reálném čase
- ⚙️ **Nastavení UI**: Snadná správa konfigurace

## Srovnání s Clawdbot

ZimaOS Echo je inspirován clawdbotem, ale optimalizován pro nasazení na NAS/edge:

| Vlastnost | ZimaOS Echo | Clawdbot |
|-----------|-------------|----------|
| **Jazyk** | Go | TypeScript/Node.js |
| **Velikost Binárky** | ~15MB | ~200MB+ (s node_modules) |
| **Využití Paměti** | ~80MB v klidu | ~200MB+ v klidu |
| **Čas Startu** | < 1s | 3-5s |
| **Runtime** | Nativní binárka | Vyžaduje Node.js |
| **Cílová Platforma** | NAS/Edge zařízení | Desktop/Server |

## Architektura

```
┌─────────────────────────────────────────────────┐
│                  ZimaOS-Echo                     │
├─────────────────────────────────────────────────┤
│  Vue 3 Frontend  │  REST API  │  WebSocket      │
├─────────────────────────────────────────────────┤
│              Hlavní Runtime (Go)                 │
│  Event Loop │ Worker Pool │ Config │ Logger     │
├─────────────────────────────────────────────────┤
│              Agent Runtime                       │
│  LLM Provider │ Nástroje │ Paměť │ Kontext      │
├─────────────────────────────────────────────────┤
│              Datová Vrstva                       │
│  SQLite (Zorm) │ ECache │ Soubory               │
└─────────────────────────────────────────────────┘
```

## Plán Vývoje

- [x] **v0.1.0** - Hlavní Runtime (Event loop, Worker pool, Config, Logger)
- [x] **v0.2.0** - Agent Runtime (LLM provideři vč. AWS Bedrock, Nástroje, Paměť)
- [x] **v0.3.0** - API Vrstva (REST, WebSocket, Streaming)
- [x] **v0.4.0** - Plugin Systém (Go moduly, WASM podpora)
- [x] **v0.5.0** - Připraveno pro Produkci (Metriky, Profilování, Záloha)
- [x] **v0.6.0** - Komunikační Kanály (Telegram, Discord, Slack, WhatsApp, Signal, iMessage)
- [x] **v0.7.0** - Bezpečnost (OIDC, MFA, WebAuthn, Audit log, Sandbox)
- [x] **v0.8.0** - Výkon (ECache, Zorm, HTTP/2)
- [x] **v0.9.0** - Budoucí Vylepšení (A2UI, Automatizace Prohlížeče)
- [ ] **v1.0.0** - RAG a Znalostní Báze

## Přispívání

Příspěvky jsou vítány! Přečtěte si prosím naši [Příručku pro Přispěvatele](CONTRIBUTING.md) pro více informací.

```bash
# Klonovat repozitář
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo

# Nainstalovat závislosti
cd server && go mod download

# Spustit testy
go test ./...

# Sestavit
go build -o zimaos-echo ./cmd/server
```

## Licence

MIT Licence - viz [LICENSE](LICENSE) pro detaily.

## Poděkování

- [clawdbot](https://github.com/clawdbot/clawdbot) - Inspirace pro projekt
- [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) - Lehký ORM

---

<p align="center">
  Vytvořeno s ❤️ týmem <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
