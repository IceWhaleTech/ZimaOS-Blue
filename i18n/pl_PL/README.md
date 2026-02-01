# ZimaOS Echo

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Echo" width="200">
</p>

<p align="center">
  <strong>Bezpieczne, obserwowalne środowisko uruchomieniowe agenta AI</strong>
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
  <strong>Polski</strong> |
  <a href="../nl_NL/README.md">Nederlands</a> |
  <a href="../sv_SE/README.md">Svenska</a>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Echo/ci.yml?branch=main&style=for-the-badge" alt="CI status"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Echo?include_prereleases&style=for-the-badge" alt="GitHub release"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT License"></a>
</p>

**ZimaOS Echo** to lekkie, wydajne środowisko uruchomieniowe agenta AI zaprojektowane dla NAS i urządzeń brzegowych. Zbudowane w Go, oferuje platformę gotową do produkcji z wdrożeniem bez konfiguracji, monitorowaniem sesji i analityką użycia.

[Szybki start](#szybki-start) · [Funkcje](#funkcje-główne)

## Najważniejsze

| Specyfikacja | Wartość |
|------|------|
| **Rozmiar binarki** | ~40 MB (pojedynczy plik wykonywalny) |
| **Pamięć (bezczynność)** | ~4 MB |
| **Czas uruchomienia** | &lt; 1 s |
| **Zależności** | Brak (wdrożenie bez konfiguracji) |

## Funkcje główne

### Wdrożenie bez konfiguracji

- **Pojedyncza binarka**: pobierz i uruchom, bez zależności środowiska uruchomieniowego
- **Konfiguracja na żądanie**: działa od razu, dostosuj w razie potrzeby
- **Wieloplatformowość**: Windows, macOS, Linux — ta sama binarka, to samo doświadczenie
- **Wsparcie demona**: uruchamianie jako usługa w tle

### Monitorowanie sesji

- **Śledzenie sesji w czasie rzeczywistym**: monitoruj wszystkie aktywne sesje AI i ich stan
- **Historia konwersacji**: pełna ścieżka audytu wszystkich interakcji
- **Odtwarzanie sesji**: przeglądaj i analizuj przeszłe konwersacje
- **Izolacja wielodostępowa**: pełne rozdzielenie sesji między użytkownikami

### Optymalizacja łańcucha wywołań

- **Śledzenie żądań**: widoczność end-to-end każdego wywołania API
- **Analiza opóźnień**: identyfikacja wąskich gardeł w potoku żądań
- **Routing dostawców**: inteligentne kierowanie do optymalnych dostawców LLM
- **Wyłącznik**: automatyczne przełączanie awaryjne przy błędach dostawcy

### Analityka użycia

- **Zużycie tokenów**: śledzenie użycia na użytkownika, sesję i dostawcę
- **Atrybucja kosztów**: szczegółowy podział kosztów według operacji
- **Ograniczanie szybkości**: zarządzanie limitami na tenant
- **Eksport raportów**: generowanie raportów użycia w wielu formatach

### Wzmacnianie bezpieczeństwa

- **Wykonanie w piaskownicy**: wszystkie wywołania narzędzi w izolowanych środowiskach
- **RBAC**: szczegółowa kontrola dostępu oparta na rolach
- **WebAuthn/Passkeys**: uwierzytelnianie FIDO2 bez hasła
- **MFA/TOTP**: uwierzytelnianie wieloskładnikowe
- **Ścieżka audytu**: niezmienne dzienniki wszystkich operacji uprzywilejowanych

## Szybki start

```bash
# Ze źródeł
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo
make build && ./dist/zimaos-echo server
```

Panel dostępny pod adresem `http://localhost:3000`.

## Konfiguracja dostawców LLM

ZimaOS Echo obsługuje wielu dostawców LLM, w tym lokalne usługi LLM:

```yaml
llm:
  # Dostawcy chmurowi
  provider: "openai"  # lub "anthropic", "azure" itd.
  api_key: "your-api-key"

  # Lokalny LLM (opcjonalnie)
  # provider: "ollama"
  # base_url: "http://localhost:11434"
```

## Architektura

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

## Obserwowalność

```yaml
# Włącz pełny stos obserwowalności
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

### Udostępniane metryki

- Opóźnienie żądań (p50, p95, p99)
- Użycie tokenów LLM na dostawcę
- Wskaźniki sukcesu/porażki wykonania narzędzi
- Liczniki pamięci i goroutine
- Przejścia stanu wyłącznika

## Środowisko deweloperskie

### Wymagania

| Narzędzie | Wersja | Instalacja |
|------|---------|---------|
| Go | 1.21+ | [golang.org](https://golang.org/dl/) |
| Node.js | 18+ | [nodejs.org](https://nodejs.org/) |
| Make | - | Preinstalowane w macOS/Linux |

### Tryb deweloperski (hot reload)

```bash
# Linux / macOS
./dev.sh

# Windows
dev.bat
```

- Frontend: `http://localhost:3000`
- Backend: `http://localhost:23456`

### Polecenia budowania

```bash
make build              # Pojedyncza binarka (frontend wbudowany)
make build-embedded     # Budowanie z wbudowanym Claude Code CLI
make build-all          # Kompilacja krzyżowa dla wszystkich platform
make clean              # Czyszczenie artefaktów budowania
```

### Struktura projektu

```
ZimaOS-Echo/
├── server/             # Backend Go
│   ├── cmd/echo/       # Punkt wejścia
│   └── internal/       # Moduły rdzenia
├── web/                # Frontend Vue 3
│   └── src/
└── dist/               # Wynik budowania
```

## Podziękowania

- [clawdbot](https://github.com/clawdbot/clawdbot) – Inspiracja projektu
- [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) – Lekki ORM

---

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
