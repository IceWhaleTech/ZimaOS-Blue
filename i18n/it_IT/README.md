![](../../docs/assets/bannerX.png)

<h2 align="center">ZimaOS Blue: Un runtime per agenti local-first per costruttori audaci</h2>

<p align="center"><strong>Pronto all'uso · Open source · Universale · Neutrale rispetto ai fornitori</strong></p>

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
  <strong>Italiano</strong> |
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
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Blue/ci.yml?branch=main&style=for-the-badge" alt="Stato CI"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Blue?include_prereleases&style=for-the-badge" alt="Release GitHub"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="Licenza MIT"></a>
</p>

<p align="center">
  <a href="https://discord.gg/b3AgFDxe9v"><img src="../../docs/assets/discord.png" alt="Discord" height="128" /></a>&nbsp;&nbsp;
  <a href="https://www.facebook.com/zimaboard/"><img src="../../docs/assets/facebook.png" alt="Facebook" height="128" /></a>&nbsp;&nbsp;
  <a href="https://x.com/ZimaSpace"><img src="../../docs/assets/x.png" alt="X" height="128" /></a>
</p>

## Introduzione

Ispirandoci a OpenClaw, crediamo che il futuro del personal computing sarà plasmato da diversi agenti IA locali che operano all’edge.

ZimaOS Blue è la nostra risposta: un runtime e un toolkit di agenti completamente open source, verificabili, indipendenti dal fornitore e pronti per la produzione che ti consentono di fornire agenti privati ​​e self-hosted senza alcun attrito.

Realizzato per sviluppatori audaci che desiderano stimolare o creare manualmente i propri agenti, Blue è progettato per le prestazioni: scritto in Go, con un ingombro di memoria di soli 19 MB. Funziona su qualsiasi x86, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS, ovunque sia collegato all'alimentazione.

## Demo

### Conversazione ed esecuzione delle attività

Una rapida demo del flusso di conversazione e dell'esecuzione delle attività in Blue.

<p align="center">
  <img src="../../docs/assets/demo.gif" alt="Conversation & Task Execution demo" width="800" />
</p>

### Integrazione dei provider LLM

Una rapida demo dell'esperienza di integrazione dei provider LLM in Blue.

<p align="center">
  <img src="../../docs/assets/demo Provider.gif" alt="LLM Providers Integration demo" width="800" />
</p>

### Panoramica rapida - Panoramica, canali e configurazione aggiuntiva

Una rapida demo che copre la panoramica del prodotto, i canali e la configurazione aggiuntiva.

<p align="center">
  <img src="../../docs/assets/demo quickv4.gif" alt="Quick Overview demo" width="720" />
</p>

## Perché Blue

<p align="center">
  <img src="../../docs/assets/design_principle.png" alt="Design Principle" />
</p>

### Pure Go, qualsiasi dispositivo

100% Go, binario statico. Effettua la compilazione incrociata su 5 destinazioni predefinite (![linux](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) `linux/amd64`, `linux/arm64`, ![macOS](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) `darwin/amd64`, `darwin/arm64`, ![windows](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) `windows/amd64`). Nessun runtime del nodo, nessun Python, nessun contenitore richiesto. Trascinalo su un NAS, <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS, un ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, un vecchio router x86 o un ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)Mac: funziona e basta. Quindi aggiungi le tue competenze in termini di interfaccia utente, logica e agente: una base di codice, ogni piattaforma.

### Fuori dagli schemi, pronto a lavorare

Tutti desiderano strumenti semplici, affidabili e scalabili quando ne hai bisogno. Strumenti che funzionano, così puoi concentrarti su ciò che stai effettivamente costruendo.

Questa non è una nuova filosofia. È lo stesso che ha costruito <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS: semplice, affidabile e costruito per non intralciarti. Blue è questa filosofia, estesa allo stack degli agenti.

### Progettato per la tua vita, costruito per rimanere locale

Dalla ricerca approfondita che fornisce un report HTML completo, a OCR, PDF, all'automazione del browser e alla conversione dei documenti, Blue gestisce flussi di lavoro complessi e reali senza inviare i dati al cloud. L'attivazione vocale, STT/TTS, Talk Mode e il supporto per l'inferenza locale rendono le interazioni quotidiane istantanee, private e sempre disponibili.

## Avvio rapido

### Opzione 1: scarica l'app desktop

Ottieni l'applicazione nativa: nessuna dipendenza, nessuna compilazione. Configurazione di prova integrata con onboarding in pochi secondi: inizia a chattare istantaneamente tramite connessione remota, non è richiesta la configurazione del bot. Vera esperienza fuori dagli schemi.

- <img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="16" /> **ZimaOS**: [Esegui su ZimaOS](https://www.zimaspace.com/zimaos?utm_source=blue)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)**macOS**: [Scarica DMG](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)**Windows**: [Scarica programma di installazione](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### Opzione 2: installa script

<img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="16" /> ZimaOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### Opzione 3: creazione dal codice sorgente

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

> **Nota:** le build di Windows richiedono:
> - [MinGW-w64](https://www.mingw-w64.org/) (gcc) e [CMake](https://cmake.org/) per le dipendenze C native (espeak-ng, Whisper.cpp, opus, kokoro, onnx)
> - [Windows SDK](https://developer.microsoft.com/en-us/windows/downloads/windows-sdk/) per le librerie di sistema (winmm, ecc.)
>
> Assicurati che `gcc`, `cmake` siano nel tuo `PATH`.

## Panoramica dell'architettura

<p align="center">
  <img src="../../docs/assets/architecture.png" alt="architecture" />
</p>

Vai oltre: offre supporto nativo per **oltre 20 piattaforme di messaggistica istantanea**, interfacce **guidate dalla voce** per dialoghi naturali e sensibili al contesto, **cambio di modello a configurazione zero** con scansione IDE.

<p align="center">
  <img src="../../docs/assets/providers.png" alt="Supported Providers" />
</p>

## Come costruire

<p align="center">
  <img src="../../docs/assets/handcraft.png" alt="handcraft" />
</p>

> ⚠️ [!IMPORTANT]
>
> Se hai intenzione di continuare ad accordare o codificare le vibrazioni su Blue, non considerare alcune belle chat come prove di rilascio. Qualsiasi modifica che influenzi il routing, il comportamento di esecuzione, la superficie dello strumento, il controllo del budget, la selezione del modello o il framework di esecuzione deve essere convalidata con Blue Harness, non con controlli a campione ad hoc.
>
> Blue dovrebbe seguire una semplice regola qui: prima i dati, prima i gate, per ultimi tagliati. In pratica, ciò significa aggiornare il set di dati / specifica di valutazione Harness pertinente prima di giudicare una modifica, quindi mantenerne uno stabile `candidate_id` durante l'intero tentativo in modo che i report di selezione, esecuzione, budget e disponibilità descrivano tutti lo stesso candidato invece di quattro esecuzioni non correlate.

### Flusso di lavoro consigliato Harness

1. Esegui `blue harness selector verify`
2. Esegui `blue harness execution verify`
3. Riutilizzare la corsa di valutazione del selettore per `blue harness budget gate`
4. Termina con `blue harness cutover-readiness`

Per l'iterazione locale, la convalida notturna o la raccolta di prove CI, preferire `python3 scripts/cutover_candidate_pipeline.py`. Esegue il selettore completo -> esecuzione -> budget -> sequenza di preparazione sotto un candidato condiviso, il che rende più facile il confronto, la revisione e il taglio del risultato.

### Guardrail extra

| Zona | Cosa guardare |
|------|----------------|
| Stabilità di base | Mantenere stabili la linea di base, la versione del set di dati e `candidate_id`, altrimenti il ​​confronto andrà alla deriva e il risultato non sarà affidabile. |
| Output di build reale | Ricostruisci il pacchetto binario o frontend interessato prima di eseguire Harness, altrimenti potresti finire per convalidare il comportamento obsoleto invece della modifica corrente. |
| Registrazione del percorso | Se frontend e backend cambiano insieme, conferma che tutti i nuovi percorsi di backend sono effettivamente registrati prima di giudicare la funzionalità attraverso il comportamento dell'interfaccia utente, perché la registrazione mancante spesso sembra un bug logico ma in realtà è un `404`. |
| Sentenza di rilascio | Un passaggio di ottimizzazione è pronto solo quando Harness non mostra alcuna regressione significativa e la disponibilità al cutover conferma che il candidato è effettivamente pronto per il cutover. |

In breve, mettere a punto Blue non significa "ci si sente meglio in poche chiacchierate". Si tratta di inserire il candidato in Harness, raccogliere prove comparabili e lasciare che siano i risultati del gate e della preparazione a decidere se il cambiamento è veramente sicuro da mantenere.

## Caratteristiche

| Caratteristica | Cosa offre |
|---------|-----|
| Recupero Web e runtime del browser ad alta disponibilità | Uno dei **più netti elementi di differenziazione** di Blue. Blue unifica **quattro percorsi di accesso web** per ricerca, lettura, estrazione e scansione; mantiene **tre livelli di fallback** tra HTTP, estrazione proxy e sessioni del browser; gestisce le **pagine anti-bot** con rilevamento delle sfide, riutilizzo di cookie/sessioni, azione invisibile e trasferimento del browser; e percorsi attraverso **tre motori browser**: `lightpanda`, Chromium gestito e Chromium relè/locale. |
| Runtime di ricerca tre in uno | **Una voce di ricerca pubblica** può essere indirizzata a `deep_research`, `analyze` e `ui_review`. Lo stesso insieme di scoperte e prove produce quindi una **ricerca citation-first**, **rapporti delimitati** e **revisioni strutturate di UI/UX/accessibilità**. |
| Harness Framework di runtime, valutazione ed evoluzione | Rende la valutazione una **primitiva di runtime** nelle fasi di sviluppo, formazione e produzione. Harness copre **regressione e controlli di fumo**, punteggio, linee di base, report e convalida in fase di esecuzione, quindi trasporta le stesse prove in **evoluzione delle competenze**, valutazione di follow-up, promozione o rollback e `AGENTS.md` o revisione della proposta di istruzione. |
| Runtime multimodale con capacità nativa | Mantiene **voce, OCR, PDF, attività del browser, conversione di documenti, compilazione di moduli strutturati, elaborazione e generazione locale di contenuti multimediali** su **prima percorsi nativi e locali**, con **instradamento del modello solo quando effettivamente necessario**. |
| Sicurezza e governance | Include **esecuzione sandbox**, **difesa con inserimento rapido**, **controllo della sessione**, autorizzazioni, **RBAC**, **WebAuthn**, guardrail operativi e **scansione di sicurezza delle competenze**. |
| Wiki LLM e spazio della conoscenza | Trasforma gli output di memoria, ricerca e runtime in una **superficie di conoscenza simile a una wiki** con **pagine di riepilogo**, indici, **backlink**, **freschezza** e **flussi di lavoro di archivio**. |
| Negozio e mercato delle competenze | Fornisce **scoperta di competenze integrate**, curation, sincronizzazione e **scansione locale** in modo che l'estensibilità sia disponibile **dal primo giorno**. |
| Pool di fornitori di livello produttivo | Fornisce un vero e proprio pool di provider con **controlli di integrità**, **failover automatico**, **interruttori di circuito** e **gara tra provider** per carichi di lavoro a lunga esecuzione. |
| Runtime modello piccolo locale integrato | Fornisce un runtime integrato **`Qwen3.5-0.8B` + `llama.cpp`** per **brevi domande e risposte locali**, riconoscimento delle immagini, instradamento degli strumenti, riepilogo, **compressione del contesto** e **preelaborazione dei documenti**. |
| Affidabilità a lungo termine | Tratta **OTA aggiornamenti**, **backup e ripristino**, **ricaricamento a caldo della configurazione** e **ripristino post-errore** come **problemi operativi integrati**. |

## Cronologia delle tappe fondamentali

<p align="center">
  <img src="../../docs/assets/timeline.png" alt="Milestone Timeline" />
</p>

| Data | Versione | Parole chiave/caratteristiche |
|------|---------|---------------------|
| 26 gennaio 2026 | `v0.1–v0.9` | Go runtime, sistema di plug-in, automazione del browser |
| 27–28 gennaio 2026 | `v0.9.0–v0.9.2` | Visualizzazione attività del browser, Blue Companion, Smart Form Filler |
| 29–31 gennaio 2026 | `v0.10.0–v0.10.9` | Claude Code CLI, API Proxy, ristrutturazione dell'interfaccia utente |
| 1–3 febbraio 2026 | `v0.10.1–v0.10.22` | Metriche, accesso remoto, cache di contesto |
| 5–18 febbraio 2026 | `v0.10.25–v0.10.29` | i18n, CC Cache, pipeline di rilascio |
| 20–25 febbraio 2026 | `v0.10.28–v0.10.29` | Caricatore desktop, UX mobile, riprogettazione della memoria |
| 28 febbraio – 2 marzo 2026 | `v0.10.30` | Deep Research, riclassificazione delle abilità, scansione di sicurezza |
| 9–18 marzo 2026 | `v0.10.31` | Revisione dashboard, refactoring VoiceChat, siti approvati |
| 19–22 marzo 2026 | `v0.10.32` | Harness implementazione, verifica delle trascrizioni, ricerca sul web |
| 23–25 marzo 2026 | `v0.10.33` | Harness gruppi, approvazioni dei browser, mercato delle competenze |
| 29–30 marzo 2026 | `v0.10.35` | Harness v3, relè browser, compressione del contesto |
| 31 marzo – 1 aprile 2026 | `v0.10.36` | Controllo della trascrizione, sovrapposizioni Harness, analisi dello strumento |
| 1 aprile 2026 | `v0.10.37` | Rafforzamento del runtime, cutover Skill+Exec, perfezionamento del ripristino |
| 2–5 aprile 2026 | `v0.10.38` | GitHub supporto, perfezionamento del mercato, miglioramenti dell'affidabilità |
| 6–7 aprile 2026 | `v0.10.39` | Unificazione della ricerca, superfici evolutive, riduzione dell'impronta di memoria |

## Comunità e supporto

- **Problemi**: [Segnalare qui bug e richieste di funzionalità](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **Discussioni**: [Discord](https://discord.gg/zwWbKA4S2)
- **Seguici** su [GitHub](https://github.com/IceWhaleTech)

[![Star History Chart](https://api.star-history.com/svg?repos=IceWhaleTech/ZimaOS-Blue&type=Date)](https://star-history.com/#IceWhaleTech/ZimaOS-Blue&Date)

## Licenza

Questo progetto è concesso in licenza in base alla licenza MIT: vedere il file [LICENSE](../../LICENSE) per i dettagli. Crediamo nell'open source e nel restituire qualcosa alla comunità.

## Collaboratori

Grazie a tutti i contributori di Blue:

<a href="https://community.vaunt.dev/board/IceWhaleTech/repository/ZimaOS-Blue">
  <img src="https://api.vaunt.dev/v1/github/entities/IceWhaleTech/repositories/ZimaOS-Blue/contributors?format=svg&limit=30" width="450" />
</a>

## Riferimenti

1. **OpenClaw**: agente open source con priorità locale. È stato pioniere nel connettere LLM ai dispositivi locali tramite adattatori di canale e chiamate di strumenti, ispirando direttamente l'architettura runtime dell'agente di Blue. https://github.com/openclaw/openclaw
2. **MiroMind** — Modalità di ricerca approfondita con sintesi supportata da prove. Ha modellato la pipeline di ricerca approfondita integrata di Blue: pianificazione, recupero parallelo, deduplicazione delle prove e generazione di report HTML. https://www.miromind.ai
3. **Karpathy's LLM Wiki** — LLM come compilatore di conoscenze. Ristruttura i LLM per costruire spazi di conoscenza persistenti e in evoluzione, andando oltre la trappola dell'accumulo di RAG.
4. **OpenSpace (HKUDS)** — Motore di abilità in evoluzione automatica. Un framework basato su DAG in cui gli agenti imparano dai fallimenti e ricavano competenze specializzate. https://github.com/HKUDS/OpenSpace
5. **Andrew Ng's Context Hub**: registro della documentazione API con versione per agenti di codifica. Risolve le allucinazioni degli agenti e la conoscenza dimenticata della sessione. Fornisce documenti curati e con versione con cicli di annotazione e feedback, trasformando la documentazione in un livello di conoscenza in grado di automigliorarsi. https://github.com/andrewyng/context-hub
6. **Notion** — Semplice, umano e intenzionalmente silenzioso. Ispirato dall'etica minimalista di Notion, Blue riporta il calore alla griglia. Dove il serif raffinato incontra un design attento, creando uno spazio che ti fa sentire come a casa. https://www.notion.com/about
7. **Matrix** — Ispirazione visiva dall'iconica estetica della pioggia digitale. La direzione estetica degli schemi tecnici di Blue.
8. **IceWhale** — Amore, morte e robot S2E2 "Ghiaccio". Un collettivo che si riunisce in tutto il mondo per sfondare i muri dei giganti di Internet e resistere alla concentrazione dei dati. La balena di ghiaccio simboleggia una comunità che costruisce insieme strumenti sovrani ai margini.
9. **ZimaOS Blue** — Amore, morte e robot S1E14 "Zima Blue". Una metafora: l'intelligenza che nasce dal servizio e si evolve per esplorare il mondo. Blue è un agente di saggezza, radicato nella semplicità e che raggiunge la profondità.
10. **ZimaOS** — Principi di progettazione semplificati, mirati e aperti. Sia ZimaOS che Blue condividono la convinzione che la tecnologia debba essere al servizio dell'utente: distribuzione in 30 secondi, esecuzione ovunque, neutralità nei confronti del fornitore. https://www.zimaspace.com/zimaos
