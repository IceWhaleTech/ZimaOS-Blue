# ZimaOS Blue

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Blue" width="200">
</p>

<p align="center">
  <strong>Runtime per agenti IA sicuro e osservabile</strong>
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
  <strong>Italiano</strong> |
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

**ZimaOS Blue** è un runtime per agenti IA leggero e ad alte prestazioni pensato per NAS e dispositivi edge. Sviluppato in Go, offre una piattaforma pronta per la produzione con deploy a zero configurazione, monitoraggio delle sessioni e analisi dell’utilizzo.

[Avvio rapido](#avvio-rapido) · [Funzionalità](#funzionalità-principali)

## Punti salienti

| Specifica | Valore |
|------|------|
| **Dimensione binario** | ~40 MB (eseguibile unico) |
| **Memoria (idle)** | ~4 MB |
| **Tempo di avvio** | < 1 s |
| **Dipendenze** | Nessuna (deploy a zero configurazione) |

## Funzionalità principali

### Deploy a zero configurazione

- **Binario unico**: scarica ed esegui, senza dipendenze di runtime
- **Configurazione su richiesta**: funziona subito, personalizzabile quando serve
- **Cross-platform**: Windows, macOS, Linux – stesso binario, stessa esperienza
- **Supporto demone**: esecuzione come servizio in background

### Monitoraggio sessioni

- **Tracciamento in tempo reale**: monitora tutte le sessioni IA attive e il loro stato
- **Cronologia conversazioni**: tracciabilità completa di tutte le interazioni
- **Replay sessioni**: rivedere e analizzare conversazioni passate
- **Isolamento multi-tenant**: separazione completa delle sessioni tra utenti

### Ottimizzazione catena di chiamate

- **Tracciamento richieste**: visibilità end-to-end di ogni chiamata API
- **Analisi latenza**: individuare colli di bottiglia nella pipeline delle richieste
- **Routing provider**: instradamento intelligente verso i provider LLM ottimali
- **Circuit breaker**: failover automatico in caso di guasto del provider

### Analisi utilizzo

- **Consumo token**: tracciamento per utente, sessione e provider
- **Attribuzione costi**: scomposizione dettagliata dei costi per operazione
- **Rate limiting**: gestione quote per tenant
- **Export report**: generazione report di utilizzo in più formati

### Hardening sicurezza

- **Esecuzione in sandbox**: tutte le chiamate agli strumenti avvengono in ambienti isolati
- **RBAC**: controllo degli accessi granulare basato sui ruoli
- **WebAuthn/Passkeys**: autenticazione FIDO2 senza password
- **MFA/TOTP**: autenticazione multifattore
- **Registro audit**: log immutabili di tutte le operazioni privilegiate

## Avvio rapido

```bash
# Da sorgenti
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue
make build && ./dist/zimaos-blue server
```

Accedi alla dashboard su `http://localhost:3000`.

## Configurazione provider LLM

ZimaOS Blue supporta più provider LLM, inclusi servizi LLM locali:

```yaml
llm:
  # Provider cloud
  provider: "openai"  # oppure "anthropic", "azure", ecc.
  api_key: "your-api-key"

  # LLM locale (opzionale)
  # provider: "ollama"
  # base_url: "http://localhost:11434"
```

## Architettura

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

## Osservabilità

```yaml
# Abilita lo stack di osservabilità completo
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

### Metriche esposte

- Latenza richieste (p50, p95, p99)
- Uso token LLM per provider
- Tassi di successo/fallimento esecuzione strumenti
- Conteggi memoria e goroutine
- Transizioni di stato del circuit breaker

## Ambiente di sviluppo

### Prerequisiti

| Strumento | Versione | Installazione |
|------|---------|---------|
| Go | 1.21+ | [golang.org](https://golang.org/dl/) |
| Node.js | 18+ | [nodejs.org](https://nodejs.org/) |
| Make | - | Preinstallato su macOS/Linux |

### Modalità sviluppo (hot reload)

```bash
# Linux / macOS
./dev.sh

# Windows
dev.bat
```

- Frontend: `http://localhost:3000`
- Backend: `http://localhost:23456`

### Comandi di build

```bash
make build              # Binario unico (frontend incorporato)
make build-embedded     # Build con Claude Code CLI incorporato
make build-all          # Cross-compilazione per tutte le piattaforme
make clean              # Pulizia artefatti di build
```

### Struttura del progetto

```
ZimaOS-Blue/
├── server/             # Backend Go
│   ├── cmd/blue/       # Punto di ingresso
│   └── internal/       # Moduli core
├── web/                # Frontend Vue 3
│   └── src/
└── dist/               # Output di build
```

## Ringraziamenti

- [clawdbot](https://github.com/clawdbot/clawdbot) – Ispirazione del progetto
- [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) – ORM leggero

---

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
