![](../../docs/assets/bannerX.png)

<h2 align="center">ZimaOS Blue: Runtime agentów local-first dla odważnych twórców</h2>

<p align="center"><strong>Gotowe od razu do użycia · Open source · Uniwersalne · Niezależne od dostawcy</strong></p>

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

Zainspirowani OpenClawem wierzymy, że przyszłość komputerów osobistych będzie kształtowana przez różnorodnych, lokalnych agentów AI działających na krawędzi.

ZimaOS Blue to nasza odpowiedź — środowisko wykonawcze agentów i zestaw narzędzi w pełni open source, podlegające audytowi, neutralne dla dostawców i gotowe do produkcji, umożliwiające dostarczanie prywatnych, hostowanych agentów bez żadnych problemów.

Stworzony z myślą o odważnych programistach, którzy chcą nadać swoim agentom charakter lub tworzyć je ręcznie, Blue został zaprojektowany z myślą o wydajności: napisany w Go i zajmujący zaledwie 19 MB pamięci. Działa na każdym x86, <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS — gdziekolwiek podłączysz zasilanie.

## Demonstracje

### Rozmowa i wykonywanie zadań

Krótka demonstracja przepływu rozmowy i wykonywania zadań w Blue.

<p align="center">
  <img src="../../docs/assets/demo.gif" alt="Conversation & Task Execution demo" width="800" />
</p>

### Integracja dostawców LLM

Krótka demonstracja integracji dostawców LLM w Blue.

<p align="center">
  <img src="../../docs/assets/demo Provider.gif" alt="LLM Providers Integration demo" width="800" />
</p>

### Szybki przegląd - Przegląd, kanały i dodatkowa konfiguracja

Krótka demonstracja obejmująca ogólny przegląd produktu, kanały i dodatkową konfigurację.

<p align="center">
  <img src="../../docs/assets/demo quickv4.gif" alt="Quick Overview demo" width="720" />
</p>

## Dlaczego Blue

<p align="center">
  <img src="../../docs/assets/design_principle.png" alt="Design Principle" />
</p>

### Pure Go, dowolne urządzenie

100% Go, statyczny plik binarny. Kompiluje krzyżowo do 5 gotowych obiektów docelowych (![linux](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) `linux/amd64`, `linux/arm64`, ![macOS](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) `darwin/amd64`, `darwin/arm64`, ![windows](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) `windows/amd64`). Żadnego środowiska wykonawczego Node, żadnego Pythona, żadnych kontenerów. Upuść go na serwerze NAS, <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, starym routerze x86 lub ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)Mac — po prostu działa. Następnie zastosuj własny interfejs użytkownika, logikę i umiejętności agenta — jedna baza kodu, każda platforma.

### Po wyjęciu z pudełka, gotowy do pracy

Każdy chce narzędzi, które są proste, niezawodne i skalowalne, gdy ich potrzebujesz. Narzędzia, które po prostu działają, dzięki czemu możesz skupić się na tym, co faktycznie budujesz.

To nie jest nowa filozofia. To ten sam, który zbudował <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS: prosty, niezawodny i zbudowany tak, aby nie przeszkadzać. Blue to filozofia rozszerzona na stos agentów.

### Zaprojektowane z myślą o Twoim życiu, zbudowane tak, aby pozostać lokalnymi

Od dogłębnych badań, które dostarczają pełny raport HTML, po OCR, PDF, automatyzację przeglądarki i konwersję dokumentów, Blue obsługuje złożone, rzeczywiste przepływy pracy bez wysyłania danych do chmury. Wybudzanie głosowe, STT/TTS, Talk Mode i obsługa wnioskowania lokalnego sprawiają, że codzienne interakcje są natychmiastowe, prywatne i zawsze dostępne.

## Szybki start

### Opcja 1: Pobierz aplikację komputerową

Uzyskaj natywną aplikację — bez zależności i bez kompilacji. Wbudowana konfiguracja próbna z możliwością wdrożenia w ciągu kilku sekund — natychmiast rozpocznij rozmowę za pośrednictwem połączenia zdalnego, bez konieczności konfigurowania bota. Prawdziwe, nieszablonowe doświadczenie.

- <img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="16" /> **ZimaOS**: [Uruchom na ZimaOS](https://www.zimaspace.com/zimaos?utm_source=blue)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)**macOS**: [Pobierz DMG](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)**Windows**: [Pobierz instalator](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### Opcja 2: Zainstaluj skrypt

<img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="16" /> ZimaOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### Opcja 3: Kompiluj ze źródła

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue
git submodule update --init --recursive
```

<img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="16" /> ZimaOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
sh build.sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
.\build.bat
```

> **Uwaga:** Kompilacje systemu Windows wymagają:
> - [MinGW-w64](https://www.mingw-w64.org/) (gcc) i [CMake](https://cmake.org/) dla natywnych zależności C (espeak-ng, whistle.cpp, opus, kokoro, onnx)
> - [Windows SDK](https://developer.microsoft.com/en-us/windows/downloads/windows-sdk/) dla bibliotek systemowych (winmm itp.)
>
> Upewnij się, że `gcc`, `cmake` znajdują się w Twoim `PATH`.

## Przegląd architektury

<p align="center">
  <img src="../../docs/assets/architecture.png" alt="architecture" />
</p>

Pójdź dalej: zapewnia natywną obsługę **ponad 20 platform komunikatorów**, **interfejsy sterowane głosem** umożliwiające naturalny dialog kontekstowy, **przełączanie modeli bez konfiguracji** ze skanowaniem IDE.

<p align="center">
  <img src="../../docs/assets/providers.png" alt="Supported Providers" />
</p>

## Jak budować

<p align="center">
  <img src="../../docs/assets/handcraft.png" alt="handcraft" />
</p>

> ⚠️ [!IMPORTANT]
>
> Jeśli planujesz dalej dostrajać lub kodować wibracje na Blue, nie traktuj kilku dobrze wyglądających czatów jako dowodu wydania. Wszelkie zmiany wpływające na routing, zachowanie wykonania, powierzchnię narzędzia, kontrolę budżetu, wybór modelu lub strukturę wykonania należy zweryfikować za pomocą Blue Harness, a nie za pomocą doraźnych kontroli wyrywkowych.
>
> Blue powinien tu kierować się jedną prostą zasadą: najpierw dane, najpierw bramki, na końcu przecięcie. W praktyce oznacza to aktualizację odpowiedniego zestawu danych / specyfikacji eval Harness przed oceną zmiany, a następnie utrzymanie jednego stabilnego `candidate_id` przez całą próbę, tak aby raporty dotyczące selekcji, wykonania, budżetu i gotowości opisywały tego samego kandydata zamiast czterech niepowiązanych serii.

### Zalecany przepływ pracy Harness

1. Uruchom `blue harness selector verify`
2. Uruchom `blue harness execution verify`
3. Użyj ponownie przebiegu ewaluacyjnego selektora dla `blue harness budget gate`
4. Zakończ `blue harness cutover-readiness`

W przypadku iteracji lokalnej, conocnej walidacji lub gromadzenia dowodów CI wybierz `python3 scripts/cutover_candidate_pipeline.py`. Uruchamia sekwencję pełnego selektora -> wykonanie -> budżet -> gotowość w ramach jednego wspólnego kandydata, co ułatwia porównywanie, przeglądanie i wycinanie wyników.

### Dodatkowe poręcze

| Powierzchnia | Co obejrzeć |
|------|----------------|
| Stabilność podstawowa | Utrzymuj linię bazową, wersję zbioru danych i `candidate_id` na stabilnym poziomie, w przeciwnym razie porównanie będzie się wahać, a wynik nie będzie godny zaufania. |
| Prawdziwe wyniki kompilacji | Odbuduj pakiet binarny lub interfejs frontendowy, którego dotyczy problem, przed uruchomieniem Harness, w przeciwnym razie zamiast bieżącej zmiany możesz sprawdzić nieaktualne zachowanie. |
| Rejestracja trasy | Jeśli frontend i backend ulegną jednoczesnej zmianie, przed oceną funkcji na podstawie zachowania interfejsu użytkownika potwierdź, że wszystkie nowe trasy backendu są rzeczywiście zarejestrowane, ponieważ brak rejestracji często wygląda na błąd logiczny, ale w rzeczywistości jest `404`. |
| Zwolnienie wyroku | Przepustka dostrajająca jest gotowa tylko wtedy, gdy Harness nie wykazuje znaczącej regresji, a gotowość do przełączenia potwierdza, że ​​kandydat jest rzeczywiście gotowy do przełączenia. |

Krótko mówiąc, dostrojenie Blue nie polega na tym, że „po kilku rozmowach poczujesz się lepiej”. Chodzi o umieszczenie kandydata w Harness, zebranie porównywalnych dowodów i pozwolenie, aby wyniki bramki i gotowości zadecydowały, czy zmiana jest naprawdę bezpieczna do utrzymania.

## Funkcje

| Funkcja | Co to zapewnia |
|--------|----------------------|
| Wysoka dostępność pobierania z Internetu i środowisko wykonawcze przeglądarki | Jeden z **najostrzejszych wyróżników** Blue. Blue ujednolica **cztery ścieżki dostępu do sieci** w celu wyszukiwania, czytania, wyodrębniania i przeszukiwania; utrzymuje **trzy warstwy rezerwowe** w ramach protokołu HTTP, ekstrakcji proxy i sesji przeglądarki; obsługuje **strony antybotowe** z wykrywaniem wyzwań, ponownym wykorzystaniem plików cookie/sesji, ukrywaniem się i przekazywaniem przeglądarki; i trasy w **trzech silnikach przeglądarek**: `lightpanda`, zarządzanym Chromium i przekaźnikowym/lokalnym Chrome. |
| Czas realizacji badań „trzy w jednym” | **Jeden publiczny wpis badawczy** może zostać przekierowany do `deep_research`, `analyze` i `ui_review`. Ten sam stos odkryć i dowodów tworzy następnie **badania oparte na cytowaniu**, **raporty ograniczone** i **ustrukturyzowane przeglądy interfejsu użytkownika/UX/dostępności**. |
| Harness Ramy środowiska wykonawczego, oceny i ewolucji | Sprawia, że ​​ocena jest **prymitywnym elementem środowiska wykonawczego** w obszarze programowania, szkolenia i produkcji. Harness obejmuje **kontrolę regresji i dymu**, punktację, linie bazowe, raporty i walidację w czasie wykonywania, a następnie przenosi te same dowody do **ewolucji umiejętności**, oceny uzupełniającej, awansu lub wycofania oraz `AGENTS.md` lub przeglądu propozycji instrukcji. |
| Multimodalne środowisko wykonawcze z możliwością natywną | Utrzymuje **głos, OCR, PDF, zadania przeglądarki, konwersję dokumentów, wypełnianie formularzy strukturalnych, przetwarzanie multimediów i lokalne generowanie multimediów** na **najpierw ścieżkach natywnych i lokalnych**, z **routowaniem modelu tylko wtedy, gdy jest to rzeczywiście potrzebne**. |
| Bezpieczeństwo i zarządzanie | Obejmuje **wykonanie piaskownicy**, **obronę przed wstrzyknięciem podpowiedzi**, **audyt sesji**, uprawnienia, **RBAC**, **WebAuthn**, bariery operacyjne i **skanowanie bezpieczeństwa umiejętności**. |
| LLM Wiki i przestrzeń wiedzy | Zamienia dane wyjściowe z pamięci, badań i środowiska wykonawczego w **powierzchnię wiedzy przypominającą wiki** z **stronami podsumowań**, indeksami, **linkami zwrotnymi**, **świeżością** i **przepływami pracy w archiwach**. |
| Sklep i rynek umiejętności | Zapewnia **wbudowane odkrywanie umiejętności**, selekcję, synchronizację i **skanowanie lokalne**, dzięki czemu rozszerzalność jest dostępna **od pierwszego dnia**. |
| Pula dostawców klasy produkcyjnej | Zapewnia prawdziwą pulę dostawców z **kontrolami stanu**, **automatycznym przełączaniem awaryjnym**, **wyłącznikami** i **wyścigami dostawców** w przypadku długotrwałych obciążeń. |
| Wbudowane lokalne środowisko wykonawcze małego modelu | Zawiera wbudowane środowisko wykonawcze **`Qwen3.5-0.8B` + `llama.cpp`** do **lokalnych krótkich pytań i odpowiedzi**, rozpoznawania obrazów, wyznaczania tras narzędzi, podsumowań, **kompresji kontekstu** i **wstępnego przetwarzania dokumentów**. |
| Długotrwała niezawodność | Traktuje **OTA aktualizacje**, **tworzenie kopii zapasowych i przywracanie**, **przeładowywanie konfiguracji na gorąco** i **odzyskiwanie po awarii** jako **wbudowane problemy operacyjne**. |

## Oś czasu kamieni milowych

<p align="center">
  <img src="../../docs/assets/timeline.png" alt="Milestone Timeline" />
</p>

| Data | Wersja | Słowa kluczowe / funkcje |
|------|---------|--------------------------------------|
| 26 stycznia 2026 | `v0.1–v0.9` | Go runtime, system wtyczek, automatyzacja przeglądarki |
| 27–28 stycznia 2026 r. | `v0.9.0–v0.9.2` | Widok zadań przeglądarki, Blue Companion, Smart Form Filler |
| 29–31 stycznia 2026 r. | `v0.10.0–v0.10.9` | Claude Code CLI, API Proxy, restrukturyzacja interfejsu użytkownika |
| 1–3 lutego 2026 r. | `v0.10.1–v0.10.22` | Metryki, dostęp zdalny, pamięć podręczna kontekstu |
| 5–18 lutego 2026 r. | `v0.10.25–v0.10.29` | i18n, pamięć podręczna CC, potok wydań |
| 20–25 lutego 2026 r. | `v0.10.28–v0.10.29` | Program ładujący komputer stacjonarny, mobilny UX, przeprojektowanie pamięci |
| 28 lutego – 2 marca 2026 | `v0.10.30` | Deep Research, zmiana rankingu umiejętności, skanowanie bezpieczeństwa |
| 9–18 marca 2026 r. | `v0.10.31` | Remont panelu, refaktor VoiceChat, zatwierdzone strony |
| 19–22 marca 2026 r. | `v0.10.32` | Harness wdrożenie, audyt transkrypcji, wyszukiwanie w Internecie |
| 23–25 marca 2026 r. | `v0.10.33` | Harness grupy, zatwierdzenia przeglądarek, rynek umiejętności |
| 29–30 marca 2026 r. | `v0.10.35` | Harness v3, przekaźnik przeglądarki, kompresja kontekstu |
| 31 marca – 1 kwietnia 2026 | `v0.10.36` | Audyt transkrypcji, nakładki Harness, parsowanie narzędzi |
| 1 kwietnia 2026 | `v0.10.37` | Utwardzanie środowiska wykonawczego, przełączanie Skill+Exec, polerowanie odzyskiwania |
| 2–5 kwietnia 2026 r. | `v0.10.38` | GitHub wsparcie, udoskonalenie rynku, poprawa niezawodności |
| 6–7 kwietnia 2026 r. | `v0.10.39` | Ujednolicenie badań, powierzchnie ewolucji, zmniejszenie zużycia pamięci |

## Społeczność i wsparcie

- **Problemy**: [Tutaj prosimy o zgłaszanie błędów i propozycji nowych funkcji](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **Dyskusje**: [Discord](https://discord.gg/zwWbKA4S2)
- **Śledź nas** na [GitHub](https://github.com/IceWhaleTech)

[![Star History Chart](https://api.star-history.com/svg?repos=IceWhaleTech/ZimaOS-Blue&type=Date)](https://star-history.com/#IceWhaleTech/ZimaOS-Blue&Date)

## Licencja

Ten projekt jest objęty licencją MIT — szczegółowe informacje można znaleźć w pliku [LICENSE](../../LICENSE). Wierzymy w otwarte oprogramowanie i dawanie czegoś społeczności.

## Współtwórcy

Dziękujemy wszystkim współpracownikom Blue:

<a href="https://community.vaunt.dev/board/IceWhaleTech/repository/ZimaOS-Blue">
  <img src="https://api.vaunt.dev/v1/github/entities/IceWhaleTech/repositories/ZimaOS-Blue/contributors?format=svg&limit=30" width="450" />
</a>

## Referencje

1. **OpenClaw** — Lokalny agent open source. Pionier łączenia LLM z urządzeniami lokalnymi poprzez adaptery kanałów i wywoływanie narzędzi, bezpośrednio inspirując architekturę środowiska wykonawczego agentów Blue. https://github.com/openclaw/openclaw
2. **MiroMind** — Tryb głębokich badań z syntezą popartą dowodami. Ukształtowano wbudowany w Blue potok głębokich badań: planowanie, równoległe wyszukiwanie, deduplikacja dowodów i generowanie raportów HTML. https://www.miromind.ai
3. **Karpathy's LLM Wiki** — LLM jako kompilator wiedzy. Przekształca LLM w celu budowania trwałych, rozwijających się przestrzeni wiedzy, wychodząc poza pułapkę akumulacji RAG.
4. **OpenSpace (HKUDS)** — Samorozwijający się silnik umiejętności. Struktura oparta na DAG, w której agenci uczą się na błędach i czerpią specjalistyczne umiejętności. https://github.com/HKUDS/OpenSpace
5. **Andrew Ng's Context Hub** — Wersjonowany rejestr dokumentacji API dla agentów kodujących. Rozwiązanie problemu halucynacji agentów i zapomnianej wiedzy sesyjnej. Zapewnia wyselekcjonowane, wersjonowane dokumenty z adnotacjami i pętlami informacji zwrotnych, przekształcając dokumentację w samodoskonalącą się warstwę wiedzy. https://github.com/andrewyng/context-hub
6. **Notion** — Prosty, ludzki i celowo cichy. Zainspirowany minimalistycznym etosem Notion, Blue przywraca ciepło do sieci. Tam, gdzie wyrafinowany szeryf spotyka się z przemyślanym designem, tworząc przestrzeń, w której czujesz się jak w domu. https://www.notion.com/about
7. **Matrix** — Wizualna inspiracja kultową estetyką cyfrowego deszczu. Kierunek estetyczny schematów technicznych Blue.
8. **IceWhale** — Miłość, śmierć i roboty S2E2 „Lód”. Kolektyw, który gromadzi się na całym świecie, aby przebić się przez mury internetowych gigantów i przeciwstawić się koncentracji danych. Lodowy wieloryb symbolizuje społeczność budującą suwerenne narzędzia razem na krawędzi.
9. **ZimaOS Blue** — Miłość, śmierć i roboty S1E14 „Zima Blue”. Metafora: inteligencja, która zaczyna się w służbie i ewoluuje, aby poznawać świat. Blue to agent mądrości zakorzenionej w prostocie i sięgającej głębi.
10. **ZimaOS** — Uproszczone, ukierunkowane i otwarte zasady projektowania. Zarówno ZimaOS, jak i Blue podzielają przekonanie, że technologia powinna służyć użytkownikowi — wdrożyć ją w 30 sekund, uruchomić w dowolnym miejscu i zachować neutralność wobec dostawców. https://www.zimaspace.com/zimaos
