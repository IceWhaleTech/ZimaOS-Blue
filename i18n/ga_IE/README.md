# ZimaOS Echo

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Echo" width="200">
</p>

<p align="center">
  <strong>Timpeallacht Rite Gníomhaire Dúchasach NAS</strong>
</p>

<p align="center">
  <a href="../../README.md">English</a> | <a href="../zh_CN/README.md">中文</a> | <strong>Gaeilge</strong>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Echo/ci.yml?branch=main&style=for-the-badge" alt="Stádas CI"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Echo?include_prereleases&style=for-the-badge" alt="Eisiúint GitHub"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="Ceadúnas MIT"></a>
</p>

**ZimaOS Echo** is timpeallacht rite gníomhaire AI éadrom, ardfheidhmíochta atá deartha go sonrach do ghléasanna NAS agus edge. Tógtha le Go, soláthraíonn sé ardán réidh le haghaidh táirgthe chun cúntóirí AI a rith ar chrua-earraí ísealchumhachta.

[Doiciméadúchán](https://echo.zimaos.com) · [Tús Tapa](#tús-tapa) · [Gnéithe](#gnéithe) · [Comparáid](#comparáid-le-clawdbot)

## Cén Fáth ZimaOS Echo?

Tá ZimaOS Echo spreagtha ag [clawdbot](https://github.com/clawdbot/clawdbot) ach atógtha ón mbun i Go le haghaidh:

- **Úsáid Acmhainní Níos Ísle**: Ritheann ar ghléasanna le chomh beag le 256MB RAM
- **Feidhmíocht Níos Fearr**: Dénártha Go dúchasach le comhthráthacht éifeachtach bunaithe ar goroutines
- **Imscaradh Níos Éasca**: Dénártha amháin, níl Node.js ag teastáil
- **Optamú NAS**: Deartha le haghaidh oibríochta 24/7 ar ghléasanna ísealchumhachta

## Tús Tapa

### Linux / macOS

```bash
curl -fsSL https://echo.zimaos.com/install.sh | sudo bash
```

### Windows (PowerShell mar Riarthóir)

```powershell
irm https://echo.zimaos.com/install.ps1 | iex
```

### Ón gCód Foinse

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo/server
go build -o zimaos-echo ./cmd/server
./zimaos-echo server
```

## Gnéithe

### Príomhghnéithe

- 🚀 **Éadrom**: Dénártha amháin < 15MB, cuimhne < 80MB
- ⚡ **Ardfheidhmíocht**: Bunaithe ar Go le comhthráthacht goroutine
- 🔌 **Ilsholáthraí**: OpenAI, Anthropic, Ollama, agus níos mó
- 🛡️ **Réidh le haghaidh Táirgthe**: Circuit breaker, meath galánta, téarnamh uathoibríoch
- 📊 **Inbhreathnaithe**: Méadracht Prometheus, próifíliú pprof, logáil struchtúrtha
- 🔄 **Athlódáil Te**: Athruithe cumraíochta gan atosú
- 💾 **Cúltaca/Athchóirigh**: Cúltaca uathoibrithe le hathchóiriú pointe-in-am

### Comhtháthú Teach Cliste

- 🏠 **Home Assistant**: Comhtháthú dúchasach le API Home Assistant
- 💡 **Rialú Gléasanna**: Soilse, lasc, braiteoirí, aeráid, agus níos mó
- 🤖 **Uathoibriú AI**: Orduithe teanga nádúrtha le haghaidh rialú teach cliste
- 📡 **Imeachtaí Fíor-Ama**: Liostáil le hathruithe stáit gléasanna trí WebSocket

### Cumais Gutha

- 🎤 **Aithint Cainte**: Tiontú cainte go téacs bunaithe ar Whisper
- 🔊 **Téacs go Caint**: Tacaíocht d'innill TTS iolracha
- 👂 **Múscailt Gutha**: Brath focal múscailte inoiriúnaithe
- 🗣️ **Orduithe Gutha**: Idirghníomhaíocht cúntóir AI gan lámha

### Ailtireacht Ilthionónta

- 👥 **Leithlisiú Tionónta**: Leithlisiú iomlán sonraí agus acmhainní
- 🔐 **Fíordheimhniú in aghaidh an Tionónta**: Fíordheimhniú neamhspleách in aghaidh an tionónta
- 📊 **Cuótaí Acmhainní**: Teorainneacha CPU, cuimhne, agus ráta API in aghaidh an tionónta
- 🎛️ **Painéal Tionónta**: Tairseach bainistíochta féinseirbhíse

### Bealaí Cumarsáide

- 💬 **Prótacal Matrix**: Teachtaireachtaí díláraithe, criptithe ó cheann go ceann
- 📱 **Telegram/Discord/Slack**: Tacaíocht d'ardáin teachtaireachtaí coitianta
- 📞 **Signal/WhatsApp**: Comhtháthú teachtaireachtaí slána
- 🍎 **iMessage**: Tacaíocht dhúchasach iMessage do macOS

### Slándáil & Fíordheimhniú

- 🔑 **WebAuthn/Passkeys**: Fíordheimhniú gan pasfhocal le FIDO2
- 🔐 **OIDC/OAuth 2.0**: SSO fiontraíochta (Google, GitHub, Okta, srl.)
- 📲 **MFA/TOTP**: Tacaíocht d'fhíordheimhniú ilfhachtóra
- 🛡️ **RBAC**: Rialú rochtana bunaithe ar róil mionsonraithe
- 📝 **Logáil Iniúchta**: Rian iniúchta slándála cuimsitheach
- 🔒 **Sandbox**: Timpeallacht fhorghníomhaithe leithlisithe d'uirlisí

### Tosaigh

- 🎨 **Painéal Vue 3**: Comhéadan gréasáin nua-aimseartha, freagrúil
- 💬 **Comhéadan Comhrá**: Freagraí sruthaithe le tacaíocht Markdown
- 📈 **Monatóir Córais**: Cairteacha úsáide acmhainní fíor-ama
- ⚙️ **UI Socruithe**: Bainistíocht cumraíochta éasca

## Comparáid le Clawdbot

Tá ZimaOS Echo spreagtha ag clawdbot ach optamaithe le haghaidh imscaradh NAS/edge:

| Gné | ZimaOS Echo | Clawdbot |
|-----|-------------|----------|
| **Teanga** | Go | TypeScript/Node.js |
| **Méid Dénártha** | ~15MB | ~200MB+ (le node_modules) |
| **Úsáid Cuimhne** | ~80MB díomhaoin | ~200MB+ díomhaoin |
| **Am Tosaithe** | < 1s | 3-5s |
| **Runtime** | Dénártha dúchasach | Node.js ag teastáil |
| **Ardán Sprice** | Gléasanna NAS/Edge | Deasc/Freastalaí |

## Ailtireacht

```
┌─────────────────────────────────────────────────┐
│                  ZimaOS-Echo                     │
├─────────────────────────────────────────────────┤
│  Tosaigh Vue 3  │  REST API  │  WebSocket       │
├─────────────────────────────────────────────────┤
│              Príomh-Runtime (Go)                 │
│  Lúb Imeachtaí │ Linn Oibrithe │ Cumraíocht │ Logálaí │
├─────────────────────────────────────────────────┤
│              Runtime Gníomhaire                  │
│  Soláthraí LLM │ Uirlisí │ Cuimhne │ Comhthéacs │
├─────────────────────────────────────────────────┤
│              Ciseal Sonraí                       │
│  SQLite (Zorm) │ ECache │ Comhaid               │
└─────────────────────────────────────────────────┘
```

## Treochlár

- [x] **v0.1.0** - Príomh-Runtime (Lúb imeachtaí, Linn oibrithe, Cumraíocht, Logálaí)
- [x] **v0.2.0** - Runtime Gníomhaire (Soláthraithe LLM lena n-áirítear AWS Bedrock, Uirlisí, Cuimhne)
- [x] **v0.3.0** - Ciseal API (REST, WebSocket, Sruthú)
- [x] **v0.4.0** - Córas Breiseán (Modúil Go, tacaíocht WASM)
- [x] **v0.5.0** - Réidh le haghaidh Táirgthe (Méadracht, Próifíliú, Cúltaca)
- [x] **v0.6.0** - Bealaí Teachtaireachtaí (Telegram, Discord, Slack, WhatsApp, Signal, iMessage)
- [x] **v0.7.0** - Slándáil (OIDC, MFA, WebAuthn, Logáil iniúchta, Sandbox)
- [x] **v0.8.0** - Feidhmíocht (ECache, Zorm, HTTP/2)
- [x] **v0.9.0** - Feabhsuithe Amach Anseo (A2UI, Uathoibriú Brabhsálaí)
- [ ] **v1.0.0** - RAG & Bunachar Eolais

## Rannchuidiú

Tá fáilte roimh rannchuidithe! Léigh ár [Treoir Rannchuidithe](CONTRIBUTING.md) le haghaidh sonraí.

```bash
# Clónáil an stór
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo

# Suiteáil spleáchais
cd server && go mod download

# Rith tástálacha
go test ./...

# Tóg
go build -o zimaos-echo ./cmd/server
```

## Ceadúnas

Ceadúnas MIT - féach [LICENSE](LICENSE) le haghaidh sonraí.

## Buíochas

- [clawdbot](https://github.com/clawdbot/clawdbot) - Inspioráid don tionscadal
- [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) - ORM éadrom

---

<p align="center">
  Déanta le ❤️ ag <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
