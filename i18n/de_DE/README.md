![](../../docs/assets/bannerX.png)

<h2 align="center">ZimaOS Blue: Eine Local-First-Agent-Laufzeit für mutige Builder</h2>

<p align="center"><strong>Sofort einsatzbereit · Open Source · Universell · Anbieterneutral</strong></p>

<p align="center">
  <a href="../../README.md">English</a> |
  <a href="../ca_ES/README.md">Català</a> |
  <a href="../cs_CZ/README.md">Čeština</a> |
  <a href="../da_DK/README.md">Dansk</a> |
  <strong>Deutsch</strong> |
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
  <a href="../pl_PL/README.md">Polski</a> |
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
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Blue/ci.yml?branch=main&style=for-the-badge" alt="CI-Status"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Blue?include_prereleases&style=for-the-badge" alt="GitHub Release"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT-Lizenz"></a>
</p>

<p align="center">
  <a href="https://discord.gg/b3AgFDxe9v"><img src="../../docs/assets/discord.png" alt="Discord" height="128" /></a>&nbsp;&nbsp;
  <a href="https://www.facebook.com/zimaboard/"><img src="../../docs/assets/facebook.png" alt="Facebook" height="128" /></a>&nbsp;&nbsp;
  <a href="https://x.com/ZimaSpace"><img src="../../docs/assets/x.png" alt="X" height="128" /></a>
</p>

## Einführung

Inspiriert von OpenClaw glauben wir, dass die Zukunft des Personal Computing von vielfältigen, lokal agierenden KI-Agenten geprägt wird, die an der Edge arbeiten.

ZimaOS Blue ist unsere Antwort – eine vollständig quelloffene, überprüfbare, herstellerneutrale und produktionsbereite Agentenlaufzeit und ein Toolkit, mit dem Sie private, selbst gehostete Agenten reibungslos bereitstellen können.

Blue wurde für mutige Entwickler entwickelt, die ihre eigenen Agenten vibieren oder manuell erstellen möchten, und ist auf Leistung ausgelegt: geschrieben in Go, mit einem Speicherbedarf von nur 19 MB. Es läuft auf jedem x86, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS – überall dort, wo Sie Strom anschließen.

## Demos

### Konversation und Aufgabenausführung

Eine kurze Demo des Gesprächsablaufs und der Aufgabenausführung in Blue.

<p align="center">
  <img src="../../docs/assets/demo.gif" alt="Conversation & Task Execution demo" width="800" />
</p>

### Integration von LLM-Anbietern

Eine kurze Demo der LLM-Anbieterintegration in Blue.

<p align="center">
  <img src="../../docs/assets/demo Provider.gif" alt="LLM Providers Integration demo" width="800" />
</p>

### Schneller Überblick - Übersicht, Kanäle und zusätzliche Konfiguration

Eine kurze Demo, die die Produktübersicht, Kanäle und zusätzliche Konfiguration abdeckt.

<p align="center">
  <img src="../../docs/assets/demo quickv4.gif" alt="Quick Overview demo" width="720" />
</p>

## Warum Blue

<p align="center">
  <img src="../../docs/assets/design_principle.png" alt="Design Principle" />
</p>

### Pure Go, jedes Gerät

100 % Go, statische Binärdatei. Kompiliert sofort auf 5 Ziele (![linux](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) `linux/amd64`, `linux/arm64`, ![macOS](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) `darwin/amd64`, `darwin/arm64`, ![windows](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) `windows/amd64`). Keine Knotenlaufzeit, kein Python, keine Container erforderlich. Legen Sie es auf einem NAS, <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS, einem ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, einem alten x86-Router oder einem ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)Mac ab – es läuft einfach. Dann ergänzen Sie Ihre eigenen UI-, Logik- und Agentenfähigkeiten – eine Codebasis, jede Plattform.

### Sofort einsatzbereit, sofort einsatzbereit

Jeder möchte einfache, zuverlässige und skalierbare Tools, wenn Sie sie benötigen. Werkzeuge, die einfach funktionieren, sodass Sie sich auf das konzentrieren können, was Sie tatsächlich erstellen.

Das ist keine neue Philosophie. Es ist dasselbe, das <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS gebaut hat: einfach, zuverlässig und so gebaut, dass es Ihnen nicht im Weg steht. Blue ist diese Philosophie, erweitert auf den Agentenstapel.

### Entworfen für Ihr Leben, gebaut, um lokal zu bleiben

Von gründlicher Recherche, die einen vollständigen HTML-Bericht liefert, bis hin zu OCR, PDF, Browser-Automatisierung und Dokumentenkonvertierung – Blue bewältigt komplexe, reale Arbeitsabläufe, ohne Ihre Daten in die Cloud zu senden. Sprachaktivierung, STT/TTS, Talk Mode und Unterstützung für lokale Inferenz machen alltägliche Interaktionen sofort, privat und immer verfügbar.

## Schnellstart

### Option 1: Desktop-App herunterladen

Holen Sie sich die native Anwendung – keine Abhängigkeiten, keine Kompilierung. Integrierte Testkonfiguration mit sekundenschnellem Onboarding – beginnen Sie sofort mit dem Chatten über eine Remote-Verbindung, keine Bot-Einrichtung erforderlich. Echtes Out-of-the-Box-Erlebnis.

- <img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="16" /> **ZimaOS**: [Auf ZimaOS ausführen](https://www.zimaspace.com/zimaos?utm_source=blue)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)**macOS**: [DMG herunterladen](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)**Windows**: [Installationsprogramm herunterladen](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### Option 2: Skript installieren

<img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="16" /> ZimaOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### Option 3: Aus der Quelle erstellen

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

> **Hinweis:** Windows-Builds erfordern:
> - [MinGW-w64](https://www.mingw-w64.org/) (gcc) und [CMake](https://cmake.org/) für native C-Abhängigkeiten (espeak-ng, whisper.cpp, opus, kokoro, onnx)
> - [Windows SDK](https://developer.microsoft.com/en-us/windows/downloads/windows-sdk/) für Systembibliotheken (winmm usw.)
>
> Stellen Sie sicher, dass sich `gcc`, `cmake` in Ihrem `PATH` befinden.

## Architekturübersicht

<p align="center">
  <img src="../../docs/assets/architecture.png" alt="architecture" />
</p>

Gehen Sie noch einen Schritt weiter: Es bietet native Unterstützung für **über 20 IM-Plattformen**, **sprachgesteuerte** Schnittstellen für natürliche, kontextbezogene Dialoge, **Zero-Config Model Switching** mit IDE-Scanning.

<p align="center">
  <img src="../../docs/assets/providers.png" alt="Supported Providers" />
</p>

## Wie man baut

<p align="center">
  <img src="../../docs/assets/handcraft.png" alt="handcraft" />
</p>

> ⚠️ [!IMPORTANT]
>
> Wenn Sie vorhaben, zusätzlich zu Blue weiter zu optimieren oder Vibe-Coding durchzuführen, betrachten Sie ein paar gut aussehende Chats nicht als Veröffentlichungsbeweis. Jede Änderung, die sich auf Routing, Ausführungsverhalten, Werkzeugoberfläche, Budgetkontrolle, Modellauswahl oder den Ausführungsrahmen auswirkt, sollte mit Blue Harness und nicht mit Ad-hoc-Stichproben validiert werden.
>
> Blue sollte hier einer einfachen Regel folgen: Daten zuerst, Gates zuerst, Cut-Over zuletzt. In der Praxis bedeutet das, den relevanten Harness Datensatz/die Bewertungsspezifikation zu aktualisieren, bevor eine Änderung beurteilt wird, und dann einen stabilen `candidate_id` über den gesamten Versuch hinweg beizubehalten, sodass Selektor-, Ausführungs-, Budget- und Bereitschaftsberichte alle denselben Kandidaten beschreiben und nicht vier unabhängige Durchläufe.

### Empfohlener Harness Workflow

1. Führen Sie `blue harness selector verify` aus
2. Führen Sie `blue harness execution verify` aus
3. Wiederverwendung des Selektor-Evaluierungslaufs für `blue harness budget gate`
4. Beenden Sie mit `blue harness cutover-readiness`

Für lokale Iteration, nächtliche Validierung oder CI-Beweissammlung bevorzugen Sie `python3 scripts/cutover_candidate_pipeline.py`. Es führt die vollständige Selektor->Ausführung->Budget->Bereitschaftssequenz unter einem gemeinsamen Kandidaten aus, was den Vergleich, die Überprüfung und das Überschneiden des Ergebnisses erleichtert.

### Zusätzliche Leitplanken

| Bereich | Was Sie sehen sollten |
|------|----------------|
| Grundlinienstabilität | Halten Sie die Basislinie, die Datensatzversion und `candidate_id` stabil, sonst weicht der Vergleich ab und das Ergebnis ist nicht vertrauenswürdig. |
| Echte Build-Ausgabe | Erstellen Sie das betroffene Binär- oder Frontend-Bundle neu, bevor Sie Harness ausführen. Andernfalls validieren Sie möglicherweise veraltetes Verhalten anstelle der aktuellen Änderung. |
| Streckenregistrierung | Wenn sich Frontend und Backend gleichzeitig ändern, stellen Sie sicher, dass alle neuen Backend-Routen tatsächlich registriert sind, bevor Sie die Funktion anhand des UI-Verhaltens beurteilen, da eine fehlende Registrierung oft wie ein logischer Fehler aussieht, in Wirklichkeit aber ein `404` ist. |
| Freilassungsurteil | Ein Tuning-Durchlauf ist nur dann bereit, wenn Harness keine sinnvolle Regression zeigt und die Umstellungsbereitschaft bestätigt, dass der Kandidat tatsächlich zur Umstellung bereit ist. |

Kurz gesagt geht es beim Tuning zusätzlich zu Blue nicht darum, dass es sich in ein paar Chats besser anfühlt. Es geht darum, den Kandidaten in Harness einzuordnen, vergleichbare Beweise zu sammeln und die Gate- und Bereitschaftsergebnisse entscheiden zu lassen, ob die Änderung wirklich sicher beizubehalten ist.

## Funktionen

| Funktion | Was es bietet |
|---------|-----|
| Hochverfügbarer Webabruf und Browser-Laufzeit | Eines der **schärfsten Unterscheidungsmerkmale** von Blue. Blue vereint **vier Webzugriffspfade** für Suchen, Lesen, Extrahieren und Crawlen; behält **drei Fallback-Ebenen** über HTTP, Proxy-Extraktion und Browsersitzungen bei; verwaltet **Anti-Bot-Seiten** mit Challenge-Erkennung, Wiederverwendung von Cookies/Sitzungen, Stealth und Browser-Übergabe; und leitet über **drei Browser-Engines**: `lightpanda`, verwaltetes Chromium und Relay/lokales Chromium. |
| Drei-in-Eins-Forschungslaufzeit | **Ein öffentlicher Forschungseintrag** kann in `deep_research`, `analyze` und `ui_review` weitergeleitet werden. Derselbe Entdeckungs- und Beweisstapel erstellt dann **Zitat-zuerst-Recherche**, **begrenzte Berichte** und **strukturierte UI/UX/Barrierefreiheitsüberprüfungen**. |
| Harness Laufzeit-, Evaluierungs- und Evolutions-Framework | Macht die Evaluierung zu einem **Laufzeitprimitiv** für Entwicklung, Schulung und Produktion. Harness deckt **Regressions- und Smoke-Checks**, Scoring, Baselines, Berichte und Laufzeitvalidierung ab und trägt dann die gleichen Beweise in **Fähigkeitsentwicklung**, Folgebewertung, Beförderung oder Rollback und `AGENTS.md` oder Überprüfung von Anweisungsvorschlägen ein. |
| Multimodale Native-Capability-First Runtime | Hält **Sprache, OCR, PDF, Browseraufgaben, Dokumentkonvertierung, strukturiertes Ausfüllen von Formularen, Medienverarbeitung und lokale Mediengenerierung** auf **nativen und lokalen Pfaden zuerst**, mit **Modellweiterleitung nur dann, wenn es tatsächlich benötigt wird**. |
| Sicherheit und Governance | Beinhaltet **Sandbox-Ausführung**, **Prompt-Injection-Verteidigung**, **Sitzungsüberwachung**, Berechtigungen, **RBAC**, **WebAuthn**, betriebliche Leitplanken und **Skill-Sicherheitsscans**. |
| LLM-Wiki und Wissensbereich | Verwandelt Speicher-, Recherche- und Laufzeitausgaben in eine **Wiki-ähnliche Wissensoberfläche** mit **Zusammenfassungsseiten**, Indizes, **Backlinks**, **Aktualität** und **Archivierungsworkflows**. |
| Skill Store und Marktplatz | Verfügt über **integrierte Skill-Erkennung**, Kuratierung, Synchronisierung und **lokales Scannen**, sodass Erweiterbarkeit **vom ersten Tag an** verfügbar ist. |
| Anbieterpool für die Produktion | Bietet einen echten Anbieterpool mit **Zustandsprüfungen**, **automatischem Failover**, **Leistungsschaltern** und **Anbieter-Rennen** für lang andauernde Workloads. |
| Integrierte lokale Runtime für kleine Modelle | Verfügt über eine integrierte **`Qwen3.5-0.8B` + `llama.cpp`**-Laufzeit für **lokale kurze Fragen und Antworten**, Bilderkennung, Tool-Routing, Zusammenfassung, **Kontextkomprimierung** und **Dokumentvorverarbeitung**. |
| Langfristige Zuverlässigkeit | Behandelt **OTA Aktualisierungen**, **Sichern und Wiederherstellen**, **Konfigurations-Hot-Reload** und **Wiederherstellung nach einem Fehler** als **integrierte Betriebsprobleme**. |

## Meilenstein-Zeitleiste

<p align="center">
  <img src="../../docs/assets/timeline.png" alt="Milestone Timeline" />
</p>

| Datum | Version | Schlüsselwörter / Funktionen |
|------|---------|-------|
| 26. Januar 2026 | `v0.1–v0.9` | Go-Laufzeit, Plugin-System, Browser-Automatisierung |
| 27.–28. Januar 2026 | `v0.9.0–v0.9.2` | Browser-Aufgabenansicht, Blue Companion, Smart Form Filler |
| 29.–31. Januar 2026 | `v0.10.0–v0.10.9` | Claude Code CLI, API Proxy, UI-Umstrukturierung |
| 1.–3. Februar 2026 | `v0.10.1–v0.10.22` | Metriken, Fernzugriff, Kontext-Cache |
| 5.–18. Februar 2026 | `v0.10.25–v0.10.29` | i18n, CC-Cache, Release-Pipeline |
| 20.–25. Februar 2026 | `v0.10.28–v0.10.29` | Desktop-Loader, mobile UX, Speicher-Redesign |
| 28. Februar–2. März 2026 | `v0.10.30` | Deep Research, Skill-Reranker, Sicherheitsscan |
| 9.–18. März 2026 | `v0.10.31` | Überarbeitung des Dashboards, VoiceChat Refaktorierung, genehmigte Websites |
| 19.–22. März 2026 | `v0.10.32` | Harness Rollout, Transkriptprüfung, Websuche |
| 23.–25. März 2026 | `v0.10.33` | Harness Gruppen, Browserfreigaben, Kompetenzmarkt |
| 29.–30. März 2026 | `v0.10.35` | Harness v3, Browser-Relay, Kontextkomprimierung |
| 31. März–1. April 2026 | `v0.10.36` | Transkriptprüfung, Harness Overlays, Tool-Parsing |
| 1. April 2026 | `v0.10.37` | Laufzeithärtung, Skill+Exec-Umstellung, Wiederherstellungspolitur |
| 2.–5. April 2026 | `v0.10.38` | GitHub Unterstützung, Marktplatzverfeinerung, Zuverlässigkeitsverbesserungen |
| 6.–7. April 2026 | `v0.10.39` | Vereinheitlichung der Forschung, Evolutionsoberflächen, Reduzierung des Speicherbedarfs |

## Community und Support

- **Probleme**: [Bitte melden Sie hier Fehler und Funktionsanfragen](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **Diskussionen**: [Discord](https://discord.gg/zwWbKA4S2)
- **Folgen Sie uns** auf [GitHub](https://github.com/IceWhaleTech)

[![Star History Chart](https://api.star-history.com/svg?repos=IceWhaleTech/ZimaOS-Blue&type=Date)](https://star-history.com/#IceWhaleTech/ZimaOS-Blue&Date)

## Lizenz

Dieses Projekt ist unter der MIT-Lizenz lizenziert – Einzelheiten finden Sie in der Datei [LICENSE](../../LICENSE). Wir glauben an Open Source und daran, der Community etwas zurückzugeben.

## Mitwirkende

Vielen Dank an alle Blue Mitwirkenden:

<a href="https://community.vaunt.dev/board/IceWhaleTech/repository/ZimaOS-Blue">
  <img src="https://api.vaunt.dev/v1/github/entities/IceWhaleTech/repositories/ZimaOS-Blue/contributors?format=svg&limit=30" width="450" />
</a>

## Referenzen

1. **OpenClaw** – Local-First-Open-Source-Agent. Pionierarbeit bei der Verbindung von LLMs mit lokalen Geräten über Kanaladapter und Tool-Aufrufe, was die Agent-Laufzeitarchitektur von Blue direkt inspirierte. https://github.com/openclaw/openclaw
2. **MiroMind** – Tiefenforschungsmodus mit evidenzgestützter Synthese. Gestaltete Blues integrierte, umfassende Forschungspipeline: Planung, paralleles Abrufen, Beweisdeduplizierung und HTML Berichtserstellung. https://www.miromind.ai
3. **Karpathy's LLM Wiki** – LLM als Wissenscompiler. Gestaltet LLMs neu, um dauerhafte, sich weiterentwickelnde Wissensräume aufzubauen und die RAG-Akkumulationsfalle zu überwinden.
4. **OpenSpace (HKUDS)** – Sich selbst entwickelnde Skill-Engine. Ein DAG-basiertes Framework, in dem Agenten aus Fehlern lernen und spezielle Fähigkeiten ableiten. https://github.com/HKUDS/OpenSpace
5. **Andrew Ng's Context Hub** – Versionierte API-Dokumentationsregistrierung für Codierungsagenten. Behebt Halluzinationen von Agenten und vergessenes Sitzungswissen. Bietet kuratierte, versionierte Dokumente mit Anmerkungen und Feedbackschleifen und verwandelt die Dokumentation in eine sich selbst verbessernde Wissensebene. https://github.com/andrewyng/context-hub
6. **Notion** – Einfach, menschlich und absichtlich ruhig. Inspiriert vom minimalistischen Ethos von Notion bringt Blue wieder Wärme ins Netz. Wo raffinierte Serifenschrift auf durchdachtes Design trifft und so einen Raum schafft, in dem man sich wie zu Hause fühlt. https://www.notion.com/about
7. **Matrix** – Visuelle Inspiration von der ikonischen digitalen Regenästhetik. Die ästhetische Ausrichtung für die technischen Diagramme von Blue.
8. **IceWhale** – Liebe, Tod und Roboter S2E2 „Ice“. Ein Kollektiv, das sich weltweit versammelt, um die Mauern der Internetgiganten zu durchbrechen und der Datenkonzentration Widerstand zu leisten. Der Eiswal symbolisiert eine Gemeinschaft, die gemeinsam am Rand souveräne Werkzeuge aufbaut.
9. **ZimaOS Blue** – Liebe, Tod und Roboter S1E14 „Zima Blue“. Eine Metapher: Intelligenz, die im Dienst beginnt und sich weiterentwickelt, um die Welt zu erkunden. Blue ist ein Mittel der Weisheit, das in der Einfachheit verwurzelt ist und nach Tiefe strebt.
10. **ZimaOS** – Vereinfachte, fokussierte, offene Designprinzipien. Sowohl ZimaOS als auch Blue teilen die Überzeugung, dass Technologie dem Benutzer dienen sollte – in 30 Sekunden einsetzbar, überall lauffähig, anbieterneutral bleiben. https://www.zimaspace.com/zimaos
