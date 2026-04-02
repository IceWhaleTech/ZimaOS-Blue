![](../../docs/assets/bannerX.png)

<p align="center">
  <a href="../../README.md">English</a> |
  <a href="../ca_ES/README.md">Català</a> |
  <a href="../cs_CZ/README.md">Čeština</a> |
  <a href="../da_DK/README.md">Dansk</a> |
  <a href="../de_DE/README.md">Deutsch</a> |
  <a href="../el_GR/README.md">Ελληνικά</a> |
  <a href="../en_GB/README.md">English (UK)</a> |
  <a href="../es_ES/README.md">Español</a> |
  <a href="../fr_FR/README.md">Français</a> |
  <a href="../ga_IE/README.md">Gaeilge</a> |
  <a href="../hr_HR/README.md">Hrvatski</a> |
  <a href="../hu_HU/README.md">Magyar</a> |
  <a href="../it_IT/README.md">Italiano</a> |
  <a href="../ja_JP/README.md">日本語</a> |
  <a href="../ko_KR/README.md">한국어</a> |
  <a href="../ml_IN/README.md">മലയാളം</a> |
  <a href="../nb_NO/README.md">Norsk Bokmål</a> |
  <a href="../nl_NL/README.md">Nederlands</a> |
  <strong>Polski</strong> |
  <a href="../pt_BR/README.md">Português (BR)</a> |
  <a href="../pt_PT/README.md">Português (PT)</a> |
  <a href="../ro_RO/README.md">Română</a> |
  <a href="../ru_RU/README.md">Русский</a> |
  <a href="../sk_SK/README.md">Slovenčina</a> |
  <a href="../sv_SE/README.md">Svenska</a> |
  <a href="../zh_CN/README.md">简体中文</a> |
  <a href="../zh_TW/README.md">繁體中文</a>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Blue/ci.yml?branch=main&style=for-the-badge" alt="Status CI"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Blue?include_prereleases&style=for-the-badge" alt="Wydanie GitHub"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="Licencja MIT"></a>
</p>

<p align="center">
  <a href="https://discord.gg/b3AgFDxe9v"><img src="../../docs/assets/discord.png" alt="Discord" height="128" /></a>&nbsp;&nbsp;
  <a href="https://www.facebook.com/zimaboard/"><img src="../../docs/assets/facebook.png" alt="Facebook" height="128" /></a>&nbsp;&nbsp;
  <a href="https://x.com/ZimaSpace"><img src="../../docs/assets/x.png" alt="X" height="128" /></a>
</p>

## Wprowadzenie

Zainspirowani przez Clawdbot, wierzymy, że **przyszłość** komputerów osobistych będzie **kształtowana przez zróżnicowanych, lokalnych agentów AI** działających na brzegu sieci.

**ZimaOS Blue to nasza odpowiedź** — w pełni **otwartoźródłowe, audytowalne i gotowe produkcyjnie środowisko uruchomieniowe agentów oraz zestaw narzędzi**, który pozwala wdrażać prywatnych, samodzielnie hostowanych agentów bez żadnych przeszkód.

Stworzony dla odważnych programistów, którzy chcą **tworzyć własnych agentów w dowolny sposób**, Blue jest **zoptymalizowany pod kątem wydajności**: napisany w **Go**, ze zużyciem pamięci już od 10 MB. Działa na **dowolnym x86, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS** — wszędzie, gdzie podłączysz zasilanie.

![](../../docs/assets/features.png)

## Najważniejsze cechy

### Lokalne podejście i automatyczny dostęp do modeli

Idąc dalej: oferuje natywne wsparcie dla **ponad 20 platform komunikacyjnych**, interfejsy **sterowane głosem** do naturalnego, kontekstowego dialogu, **automatyczne przełączanie modeli** ze skanowaniem IDE oraz wielowarstwowe osobowości SOUL.

<p align="center">
  <img src="../../docs/assets/channels.png" alt="Supported Channels" />
</p>

### Szybki, lekki

Kompilowany natywnie w Go — bez interpretera, bez maszyny wirtualnej, bez narzutu. Działa cicho na wszystkim, od serwerów po urządzenia biurkowe.

| Metryka | ZimaOS Blue (Go) | Reference Agent (Node + dist) |
|---------|-------------------|------------------------|
| `help` zimny / ciepły start | **0.18 s / < 0.01 s** | 3.31 s / ~1.11 s |
| `status` czas wykonania (najlepszy z 3) | **< 0.01 s** | 5.98 s |
| `help` szczytowe RSS | **~10 MB** | ~394 MB |
| `status` szczytowe RSS | **~15 MB** | ~1.52 GB |
| Zależności uruchomieniowe | **Brak** | Node.js 18+ |

> Testy przeprowadzone na macOS arm64 (tryb serwerowy, bez desktopowego UI), ten sam host, najlepszy z 3 przebiegów. Luty 2026.

### Czysty Go, dowolne urządzenie

100% Go, statyczny plik binarny. **Kompilacja krzyżowa na 5 platform** od razu (![linux](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) amd64/arm64, ![macOS](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) amd64/arm64, ![windows](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) amd64). Bez Node, bez Pythona, bez kontenerów. Wrzuć go na NAS, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, stary router x86 lub Maca — po prostu działa. **Następnie dodaj własny interfejs, logikę i umiejętności agenta** — jeden kod źródłowy, każda platforma.

### Bezpieczeństwo i zarządzanie

Wbudowany sidecar proxy API z wielowarstwową obroną:
- **Izolowane wykonanie** – Wszystkie wywołania narzędzi działają w odizolowanych środowiskach.
- **Obrona przed wstrzykiwaniem promptów** – Ponad 7 wbudowanych strategii przechwytywania.
- **Audyt sesji** – Pełne monitorowanie sesji, każda interakcja jest śledzona.
- **RBAC i WebAuthn** – Szczegółowa kontrola dostępu z uwierzytelnianiem bezhasłowym.

## Dlaczego Blue

Wierzymy, że **komputery osobiste nowej generacji** obejmują LLM — ale **kontrolowalni, audytowalni** agenci pozostają fundamentem zarówno dla osób indywidualnych, jak i zespołów. **Blue zapewnia**:
- **Kompleksowy rdzeń** – Zaawansowane zarządzanie modelami, integracja z komunikatorami, rozbudowana persona i interfejsy języka naturalnego dostosowane do codziennych interakcji (słuchawki, głos, inteligentne okulary).
- **Lokalne podejście, ultralekki, wieloplatformowy** – Nie wymaga wydajnego sprzętu. Działa na wszystkim, co potrafi obliczać.
- **Bezpieczny i audytowalny** – Audyt sesji, sandboxing, kontrola uprawnień i wbudowany proxy API działający jako zapora warstwy aplikacji — każdy bajt wejścia/wyjścia jest widoczny.

![](../../docs/assets/design_principle.png)

Minimalizujemy szablonowy kod, abyś **skupił się na tym, co ważne**. Wierni <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> **filozofii projektowej ZimaOS**, Blue zapewnia:
- **Od zera do jedynki jednym kliknięciem** – Natychmiastowe wdrożenie, bez skomplikowanej konfiguracji.
- **Szybkie prototypowanie** – Twórz narzędzia, interakcje i pakiety aplikacji dopasowane do scenariuszy.
- **Gotowy na cały świat** – **Świat jest ogromny** i nie domyślnie angielski. **Ponad 20 języków, natywnie**, bez barier.
- **Otwarty ekosystem modeli** – Bez uzależnienia od dostawcy. Przynieś własne modele.

<details>
<summary>
<p align="center">
  <img src="../../docs/assets/providers.png" alt="Supported Providers" />
</p>
</summary>

| Dostawca | Modele | Typ |
|----------|--------|-----|
| OpenAI | GPT-4o, GPT-4, o1, o3 | Chmura |
| Anthropic | Claude 4.5, Claude 4 | Chmura |
| Google | Gemini 2.5, Gemini 2.0 | Chmura |
| Ollama | Llama, Qwen, Gemma, Phi itp. | Lokalny |
| DeepSeek | DeepSeek-V3, DeepSeek-R1 | Chmura |
| Grok | Grok-3, Grok-3-mini | Chmura |
| Qwen | Qwen-Max, Qwen-Plus, Qwen-Turbo | Chmura |
| GLM | GLM-4, GLM-4-Flash | Chmura |
| Moonshot | Moonshot-v1 | Chmura |
| MiniMax | abab6.5, abab5.5 | Chmura |
| Venice | Llama, Mistral (prywatność) | Chmura |
| AWS Bedrock | Claude, Llama, Titan | Chmura |
| Azure | Modele OpenAI przez Azure | Chmura |
| OpenRouter | 100+ zagregowanych modeli | Chmura |
| AIHubMix | Agregator wielu dostawców | Chmura |
| Codex | OpenAI Codex | Chmura |
| SiliconFlow | DeepSeek, Qwen, Llama via SiliconFlow | Chmura |
| Własny | Dowolne API kompatybilne z OpenAI / Anthropic / Gemini | Chmura / Lokalny |

</details>

### Obsługiwane IDE

<p align="center">
  <img src="../../docs/assets/ides.png" alt="Supported IDEs" />
</p>

## Szybki start

### Opcja 1: Pobierz aplikację desktopową (macOS i Windows)

Pobierz natywną aplikację — bez zależności, bez kompilacji. Wbudowana konfiguracja próbna, gotowa w sekundy — połącz się zdalnie i zacznij rozmawiać natychmiast, bez konfigurowania bota. Prawdziwe uruchomienie od razu.

- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)**macOS**: [Pobierz DMG](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)**Windows**: [Pobierz instalator](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### Opcja 2: Skrypt instalacyjny

![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### Opcja 3: Kompilacja ze źródeł

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue
```

![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
sh build.sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
.\build.bat
```

## Przegląd architektury

<details>
<summary>
<img src="../../docs/assets/architecture.png" alt="Architecture" />
</summary>

### Mapa pakietów (`server/internal/`)

| Warstwa | Pakiety |
|---------|---------|
| Gateway | bootstrap, server, gateway |
| Proxy | proxy, connection, streaming, resilience |
| Dostawca | providerpool, providers, llm |
| Przycinanie | pruner (detector, segmenter, bm25, pipeline, cache) |
| Agent | context, tools, personality, humanizer |
| Pamięć | memory, embedding, kvstore |
| Kanał | channel, autoreply, i18n |
| Bezpieczeństwo | security, auth, permission, rbac, mfa, password, oidc, extauth, sandbox, promptguard, audit |
| Głos | voice, tts, stt, speech |
| Obserwacja | metrics, companion, profiling, leakdetect |
| Wtyczka | plugin, skill, skillstore |
| Integracja | browser, cron, workflow, formfiller, tunnel, crawler |
| Harmonogram | scheduler, worker, workerpool, pool |
| Rdzeń | lifecycle, config, logger, database, cache, ratelimit, retry, timeutil, sync |
| System | sysinfo, cgroup, iotask, watcher, resources, backup, update |
| Wielodostęp | tenant, user, session, preview |

</details>

### Przepływ danych

**Żądanie czatu (gorąca ścieżka proxy)**
```
Client [Proxy API Key] → Auth Gate → Prompt Guard → Context Pruner (optional)
  → Provider Pool (route:auto/cloud/local) → CC Cache (L1→L2) check
  → Upstream LLM → Response → Cache Store → Metrics Writer → Client (SSE stream)
```

**Przepływ wiadomości kanałowych**
```
Telegram/Discord/... → Channel Manager → AutoReply check
  → Chat Handler → LLM → Humanizer (MD→text) → Channel → User
```

**Potok głosowy**
```
WebSocket audio → STT (Whisper) → LLM Processing → TTS (eSpeak/Edge) → WebSocket audio
```


## Jak używać

![](../../docs/assets/handcraft.png)

## Harmonogram kamieni milowych

<details>
<summary>
<img src="../../docs/assets/timeline.png" alt="Milestone Timeline" />
</summary>

| Wersja | Obszar | Kluczowa wartość | Status |
|--------|--------|------------------|--------|
| v0.1 | Rdzeń środowiska Go | Stabilne jądro, praca 24h | Done |
| v0.2 | Podstawowe możliwości | Minimalnie użyteczny, integracja LLM | Done |
| v0.3 | Integracja z NAS | Natywne wsparcie NAS, obsługa systemd | Done |
| v0.4 | System wtyczek | Rozszerzalność, podstawy bezpieczeństwa | Done |
| v0.5 | Baza produktowa | Gotowy produkcyjnie, dokumentacja | Done |
| v0.6 | Kanały wiadomości | Obsługa wielu kanałów | Done |
| v0.7 | Bezpieczeństwo | OIDC, MFA, audyt | Done |
| v0.8 | Wydajność | Optymalizacja, cache, benchmarki | Done |
| v0.9 | Ekosystem | Wielodostęp, automatyzacja przeglądarki, głos | Done |
| v0.10.0 | Pakietowanie CLI | Pakietowanie CC CLI, wykrywanie, auto-aktualizacja | Done |
| v0.10.1 | Monitorowanie metryk | Statystyki API, śledzenie tokenów, TTFT | Done |
| v0.10.2 | Niezawodność CLI | Cykl życia procesów, odzyskiwanie po błędach | Done |
| v0.10.3 | Integracja CLI | Kreator konfiguracji, auto-wykrywanie dostawców | Done |
| v0.10.4 | Pakietowanie Tauri | Aplikacja desktopowa, zasobnik systemowy | Done |
| v0.10.5 | Sidecar proxy API | Wybór tras, ochrona promptów, statystyki użycia | Done |
| v0.10.6 | Pula dostawców | Routing wielu dostawców, health check, failover | Done |
| v0.10.7 | Tryb podglądu | Dostęp bez uwierzytelnienia, bramkowanie funkcji | Done |
| v0.10.8 | Sklep umiejętności | Infrastruktura sklepu umiejętności, walidacja kanałów | Done |
| v0.10.9–10 | Zarządzanie użytkownikami | Podużytkownicy, uprawnienia na poziomie stron | Done |
| v0.10.13–14 | Bezpieczeństwo i umiejętności | Strona bezpieczeństwa, przeprojektowanie sklepu umiejętności | Done |
| v0.10.15 | Ulepszenia czatu | UX czatu, potok wiadomości | Done |
| v0.10.16 | Moduł mowy | Sherpa TTS/ASR, eSpeak, przełączanie dostawców | Done |
| v0.10.17 | Zdalny dostęp | Tunele Ngrok, Cloudflare, certyfikaty ACME | Done |
| v0.10.18–20 | Sprint wydajności | Wydajność startu/czatu, cache kontekstu | Done |
| v0.10.21–22 | Prompt i DingTalk | Prompt systemowy, kanał DingTalk | Done |
| v0.10.23 | Aktualizacja OTA | System aktualizacji OTA | Done |
| v0.10.24 | Rozbudowa kanałów | 10 kanałów zaktualizowanych ze stubów | Done |
| v0.10.25 | CC Cache | Dwupoziomowy cache (L1 pamięć + L2 dysk) | Done |
| v0.10.26 | Humanizer | Potok humanizacji odpowiedzi | Done |
| v0.10.27 | Przycinanie kontekstu | 54% oszczędność tokenów na kodzie (SWE-bench oficjalnie), 46–47% na dokumentach ogólnych (lokalny IR), scoring BM25, segmentacja | Done |
| v0.10.28 | Usługa pamięci | Wyszukiwanie progresywne, backend dual-write | Done |

</details>

## Społeczność i wsparcie

- **Zgłoszenia**: [Zgłaszaj błędy i propozycje funkcji tutaj](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **Dyskusje**: [Discord](https://discord.gg/b3AgFDxe9v)
- **Śledź nas** na [GitHub](https://github.com/IceWhaleTech)

## Licencja

Ten projekt jest udostępniony na licencji MIT — szczegóły w pliku [LICENSE](../../LICENSE). Wierzymy w open source i dzielenie się ze społecznością.

## Współtwórcy

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
