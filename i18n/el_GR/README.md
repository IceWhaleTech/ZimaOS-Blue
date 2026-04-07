![](../../docs/assets/bannerX.png)

<p align="center">
  <a href="../../README.md">English</a> |
  <a href="../ca_ES/README.md">Català</a> |
  <a href="../cs_CZ/README.md">Čeština</a> |
  <a href="../da_DK/README.md">Dansk</a> |
  <a href="../de_DE/README.md">Deutsch</a> |
  <strong>Ελληνικά</strong> |
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
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Blue/ci.yml?branch=main&style=for-the-badge" alt="Κατάσταση CI"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Blue?include_prereleases&style=for-the-badge" alt="Έκδοση GitHub"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="Άδεια MIT"></a>
</p>

<p align="center">
  <a href="https://discord.gg/b3AgFDxe9v"><img src="../../docs/assets/discord.png" alt="Discord" height="128" /></a>&nbsp;&nbsp;
  <a href="https://www.facebook.com/zimaboard/"><img src="../../docs/assets/facebook.png" alt="Facebook" height="128" /></a>&nbsp;&nbsp;
  <a href="https://x.com/ZimaSpace"><img src="../../docs/assets/x.png" alt="X" height="128" /></a>
</p>

## Εισαγωγή

Εμπνευσμένοι από το Clawdbot, πιστεύουμε ότι το **μέλλον** της προσωπικής πληροφορικής θα **διαμορφωθεί από ποικίλους, τοπικά-προτεραιοποιημένους πράκτορες AI** που εκτελούνται στην άκρη του δικτύου.

**Το ZimaOS Blue είναι η απάντησή μας** — ένα πλήρως **ανοιχτού κώδικα, ελέγξιμο και έτοιμο για παραγωγή περιβάλλον εκτέλεσης πρακτόρων και εργαλειοθήκη** που σας επιτρέπει να αναπτύσσετε ιδιωτικούς, αυτο-φιλοξενούμενους πράκτορες χωρίς καμία τριβή.

Σχεδιασμένο για τολμηρούς προγραμματιστές που θέλουν να **δημιουργήσουν τους δικούς τους πράκτορες με έμπνευση ή χειροτεχνία**, το Blue είναι **σχεδιασμένο για απόδοση**: γραμμένο σε **Go**, με αποτύπωμα μνήμης μόλις 10 MB. Εκτελείται σε **οποιοδήποτε x86, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS** — οπουδήποτε συνδέσετε ρεύμα.

![](../../docs/assets/features.png)

## Κύρια Χαρακτηριστικά

### Τοπικά-Προτεραιοποιημένος Σχεδιασμός & Αυτόματη Πρόσβαση Μοντέλων

Πηγαίνοντας ακόμα πιο μακριά: παρέχει εγγενή υποστήριξη για **20+ πλατφόρμες IM**, **φωνητικά καθοδηγούμενες** διεπαφές για φυσικό, συμφραζόμενο διάλογο, **μηδενικής ρύθμισης εναλλαγή μοντέλων** με σάρωση IDE, και προσωπικότητες επιπέδου SOUL.

<p align="center">
  <img src="../../docs/assets/channels.png" alt="Supported Channels" />
</p>

### Γρήγορο & Ελαφρύ

Μεταγλωττισμένο εγγενώς σε Go — χωρίς διερμηνέα, χωρίς VM, χωρίς επιβάρυνση. Εκτελείται αθόρυβα σε τα πάντα, από διακομιστές μέχρι επιτραπέζιες συσκευές.

| Μέτρηση | ZimaOS Blue (Go) | Reference Agent (Node + dist) |
|---------|-------------------|------------------------|
| `help` κρύα / θερμή | **0.18 s / < 0.01 s** | 3.31 s / ~1.11 s |
| `status` χρόνος εκτέλεσης (καλύτερο από 3) | **< 0.01 s** | 5.98 s |
| `help` μέγιστο RSS | **~10 MB** | ~394 MB |
| `status` μέγιστο RSS | **~15 MB** | ~1.52 GB |
| μνήμη αδράνειας `gateway run` μετά από cold start | **~19 MB** | - |
| Εξαρτήσεις εκτέλεσης | **Καμία** | Node.js 18+ |

> Οι παραπάνω γραμμές CLI είναι το ιστορικό microbenchmark `help` / `status` στον ίδιο host. Η νέα γραμμή `gateway run` δείχνει τη μνήμη αδράνειας μετά από cold start, μετρημένη σε macOS arm64 μέσω `vmmap Physical footprint` αφού σταθεροποιηθεί η εκκίνηση. Φεβ-Απρ 2026.

### Αμιγώς Go, Οποιαδήποτε Συσκευή

100% Go, στατικό δυαδικό αρχείο. **Διασταυρούμενη μεταγλώττιση για 5 στόχους** εκ κατασκευής (![linux](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) amd64/arm64, ![macOS](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) amd64/arm64, ![windows](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) amd64). Χωρίς Node runtime, χωρίς Python, χωρίς containers. Τοποθετήστε το σε NAS, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, παλιό x86 router ή ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)Mac — απλά τρέχει. **Στη συνέχεια προσθέστε τη δική σας διεπαφή, λογική και δεξιότητες πρακτόρων** — μία βάση κώδικα, κάθε πλατφόρμα.

### Ασφάλεια & Διακυβέρνηση

Ενσωματωμένος sidecar API proxy με άμυνα σε βάθος:
- **Εκτέλεση σε Sandbox** – Όλες οι κλήσεις εργαλείων εκτελούνται σε απομονωμένα περιβάλλοντα.
- **Άμυνα κατά Prompt Injection** – 7+ ενσωματωμένες στρατηγικές αναχαίτισης.
- **Έλεγχος Συνεδριών** – Πλήρης παρακολούθηση συνεδριών, κάθε αλληλεπίδραση ανιχνεύσιμη.
- **RBAC & WebAuthn** – Λεπτομερής έλεγχος πρόσβασης με αυθεντικοποίηση χωρίς κωδικό.

## Γιατί Blue

Πιστεύουμε ότι η **προσωπική πληροφορική επόμενης γενιάς** αγκαλιάζει τα LLMs — αλλά οι **ελεγχόμενοι, ελέγξιμοι** πράκτορες παραμένουν ο ακρογωνιαίος λίθος τόσο για άτομα όσο και για ομάδες. **Το Blue προσφέρει**:
- **Ολοκληρωμένος Πυρήνας** – Προηγμένη διαχείριση μοντέλων, ενσωμάτωση IM, βελτιωμένη περσόνα και διεπαφές φυσικής γλώσσας βελτιστοποιημένες για καθημερινές αλληλεπιδράσεις (ακουστικά, φωνή, έξυπνα γυαλιά).
- **Τοπικά-Προτεραιοποιημένο, Εξαιρετικά Ελαφρύ, Πολλαπλών Συσκευών** – Δεν απαιτείται υλικό υψηλών προδιαγραφών. Εκτελείται σε οτιδήποτε μπορεί να υπολογίσει.
- **Ασφαλές & Ελέγξιμο** – Έλεγχος συνεδριών, sandboxing, έλεγχοι δικαιωμάτων και ενσωματωμένος API proxy που λειτουργεί ως τείχος προστασίας επιπέδου εφαρμογής — κάθε byte εισόδου/εξόδου είναι ορατό.

![](../../docs/assets/design_principle.png)

Ελαχιστοποιούμε τον επαναλαμβανόμενο κώδικα ώστε να **εστιάσετε σε αυτό που μετράει**. Πιστοί στη <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> **σχεδιαστική φιλοσοφία του ZimaOS**, το Blue προσφέρει:
- **Από το Μηδέν στο Ένα με Ένα Κλικ** – Άμεση ανάπτυξη, χωρίς πολύπλοκη ρύθμιση.
- **Γρήγορη Πρωτοτυποποίηση** – Δημιουργήστε με έμπνευση ή χειροτεχνία εργαλεία, αλληλεπιδράσεις και πακέτα εφαρμογών για συγκεκριμένα σενάρια.
- **Παγκοσμίως Έτοιμο** – **Ο κόσμος είναι τεράστιος**, και δεν μιλάει εξ ορισμού Αγγλικά. **20+ γλώσσες, εγγενώς**, χωρίς εμπόδια.
- **Ανοιχτό Οικοσύστημα Μοντέλων** – Χωρίς δέσμευση σε προμηθευτή. Φέρτε τα δικά σας μοντέλα.

<details>
<summary>
<p align="center">
  <img src="../../docs/assets/providers.png" alt="Supported Providers" />
</p>
</summary>

| Πάροχος | Μοντέλα | Τύπος |
|---------|---------|-------|
| OpenAI | GPT-4o, GPT-4, o1, o3 | Cloud |
| Anthropic | Claude 4.5, Claude 4 | Cloud |
| Google | Gemini 2.5, Gemini 2.0 | Cloud |
| Ollama | Llama, Qwen, Gemma, Phi κ.ά. | Τοπικό |
| DeepSeek | DeepSeek-V3, DeepSeek-R1 | Cloud |
| Grok | Grok-3, Grok-3-mini | Cloud |
| Qwen | Qwen-Max, Qwen-Plus, Qwen-Turbo | Cloud |
| GLM | GLM-4, GLM-4-Flash | Cloud |
| Moonshot | Moonshot-v1 | Cloud |
| MiniMax | abab6.5, abab5.5 | Cloud |
| Venice | Llama, Mistral (προτεραιότητα απορρήτου) | Cloud |
| AWS Bedrock | Claude, Llama, Titan | Cloud |
| Azure | Μοντέλα OpenAI μέσω Azure | Cloud |
| OpenRouter | 100+ συγκεντρωμένα μοντέλα | Cloud |
| AIHubMix | Συγκεντρωτής πολλαπλών παρόχων | Cloud |
| Codex | OpenAI Codex | Cloud |
| SiliconFlow | DeepSeek, Qwen, Llama via SiliconFlow | Cloud |
| Προσαρμοσμένο | Οποιοδήποτε API συμβατό με OpenAI / Anthropic / Gemini | Cloud / Τοπικό |

</details>

### Υποστηριζόμενα IDE

<p align="center">
  <img src="../../docs/assets/ides.png" alt="Supported IDEs" />
</p>

## Γρήγορη Εκκίνηση

### Επιλογή 1: Λήψη Εφαρμογής Επιφάνειας Εργασίας

Αποκτήστε την εγγενή εφαρμογή — χωρίς εξαρτήσεις, χωρίς μεταγλώττιση.

- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)**macOS**: [Λήψη DMG](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)**Windows**: [Λήψη Εγκαταστάτη](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### Επιλογή 2: Σενάριο Εγκατάστασης

![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### Επιλογή 3: Κατασκευή από Πηγαίο Κώδικα

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

> **Note:** Windows builds require [MinGW-w64](https://www.mingw-w64.org/) (gcc) and [CMake](https://cmake.org/) for native C dependencies (espeak-ng, whisper.cpp, opus). Make sure `gcc` and `cmake` are in your `PATH`.

## Επισκόπηση Αρχιτεκτονικής

![](../../docs/assets/architecture.png)

### Ροή Δεδομένων

**Αίτημα Συνομιλίας (Proxy Hot Path)**
```
Client [Proxy API Key] → Auth Gate → Prompt Guard → Context Pruner (optional)
  → Provider Pool (route:auto/cloud/local) → CC Cache (L1→L2) check
  → Upstream LLM → Response → Cache Store → Metrics Writer → Client (SSE stream)
```

**Ροή Μηνυμάτων Καναλιού**
```
Telegram/Discord/... → Channel Manager → AutoReply check
  → Chat Handler → LLM → Humanizer (MD→text) → Channel → User
```

**Αγωγός Φωνής**
```
WebSocket audio → STT (Whisper) → LLM Processing → TTS (eSpeak/Edge) → WebSocket audio
```

**Ροή αξιολόγησης Harness**
```
Quick Eval / Harness API → Ελεγκτής αξιολόγησης → Dispatcher ομάδων εκτέλεσης
  → Εργασία agent ή eval driver → Εργαλεία + Workspace + Artifacts
  → Scorecards / Reports / Gates προϋπολογισμού+εκτέλεσης+selector
  → Cutover Readiness / Απόφαση υποψηφίου
```


### Χάρτης Πακέτων (`server/internal/`)

| Επίπεδο | Πακέτα |
|---------|--------|
| Gateway | bootstrap, server, gateway |
| Proxy | proxy, connection, streaming, resilience |
| Πάροχος | providerpool, providers, llm |
| Pruner | pruner (detector, segmenter, bm25, pipeline, cache) |
| Πράκτορας | context, tools, personality, humanizer |
| Μνήμη | memory, embedding, kvstore |
| Κανάλι | channel, autoreply, i18n |
| Ασφάλεια | security, auth, permission, rbac, mfa, password, oidc, extauth, sandbox, promptguard, audit |
| Φωνή | voice, tts, stt, speech |
| Παρατήρηση | metrics, companion, profiling, leakdetect |
| Plugin | plugin, skill, skillstore |
| Ενσωμάτωση | browser, cron, workflow, formfiller, tunnel, crawler |
| Χρονοπρογραμματιστής | scheduler, worker, workerpool, pool |
| Πυρήνας | lifecycle, config, logger, database, cache, ratelimit, retry, timeutil, sync |
| Σύστημα | sysinfo, cgroup, iotask, watcher, resources, backup, update |
| Πολυ-ενοικίαση | tenant, user, session, preview |

## Τρόπος Χρήσης

![](../../docs/assets/handcraft.png)

## Χρονοδιάγραμμα Ορόσημων

![](../../docs/assets/timeline.png)

| Έκδοση | Εστίαση | Βασική Αξία | Κατάσταση |
|--------|---------|-------------|-----------|
| v0.1 | Πυρήνας Εκτέλεσης Go | Σταθερός πυρήνας, 24ωρη λειτουργία | Done |
| v0.2 | Βασικές Δυνατότητες | Ελάχιστα χρησιμοποιήσιμο, ενσωμάτωση LLM | Done |
| v0.3 | Ενσωμάτωση NAS | Εγγενές NAS, υποστήριξη systemd | Done |
| v0.4 | Σύστημα Plugin | Επεκτάσιμο, βασικά ασφαλείας | Done |
| v0.5 | Βάση Προϊόντος | Έτοιμο για παραγωγή, τεκμηρίωση | Done |
| v0.6 | Κανάλια Μηνυμάτων | Υποστήριξη πολλαπλών καναλιών | Done |
| v0.7 | Ασφάλεια | OIDC, MFA, έλεγχος | Done |
| v0.8 | Απόδοση | Βελτιστοποίηση, caching, benchmarks | Done |
| v0.9 | Οικοσύστημα | Πολυ-ενοικίαση, αυτοματισμός browser, φωνή | Done |
| v0.10.0 | Ομαδοποίηση CLI | Ομαδοποίηση CC CLI, ανίχνευση, αυτόματη ενημέρωση | Done |
| v0.10.1 | Παρακολούθηση Μετρήσεων | Στατιστικά API, παρακολούθηση token, TTFT | Done |
| v0.10.2 | Αξιοπιστία CLI | Κύκλος ζωής διεργασιών, ανάκτηση σφαλμάτων | Done |
| v0.10.3 | Ενσωμάτωση CLI | Οδηγός εγκατάστασης, αυτόματη ανίχνευση παρόχου | Done |
| v0.10.4 | Πακετοποίηση Tauri | Εφαρμογή επιφάνειας εργασίας, system tray | Done |
| v0.10.5 | API Proxy Sidecar | Επιλογή διαδρομής, prompt guard, στατιστικά χρήσης | Done |
| v0.10.6 | Provider Pool | Δρομολόγηση πολλαπλών παρόχων, έλεγχος υγείας, failover | Done |
| v0.10.7 | Λειτουργία Προεπισκόπησης | Μη αυθεντικοποιημένη πρόσβαση, feature gating | Done |
| v0.10.8 | Skill Store | Υποδομή skill store, επικύρωση καναλιών | Done |
| v0.10.9–10 | Διαχείριση Χρηστών | Υπο-χρήστες, δικαιώματα σε επίπεδο σελίδας | Done |
| v0.10.13–14 | Ασφάλεια & Skills | Σελίδα ασφαλείας, επανασχεδιασμός skill store | Done |
| v0.10.15 | Βελτιώσεις Συνομιλίας | UX συνομιλίας, αγωγός μηνυμάτων | Done |
| v0.10.16 | Μονάδα Ομιλίας | Sherpa TTS/ASR, eSpeak, εναλλαγή παρόχου | Done |
| v0.10.17 | Απομακρυσμένη Πρόσβαση | Ngrok, σήραγγες Cloudflare, πιστοποιητικά ACME | Done |
| v0.10.18–20 | Sprint Απόδοσης | Απόδοση εκκίνησης/συνομιλίας, cache πλαισίου | Done |
| v0.10.21–22 | Prompt & DingTalk | System prompt, κανάλι DingTalk | Done |
| v0.10.23 | Ενημέρωση OTA | Σύστημα ενημέρωσης OTA | Done |
| v0.10.24 | Αναβάθμιση Καναλιών | 10 κανάλια αναβαθμίστηκαν από stubs | Done |
| v0.10.25 | CC Cache | Διπλού επιπέδου cache (L1 μνήμη + L2 δίσκος) | Done |
| v0.10.26 | Humanizer | Αγωγός εξανθρωπισμού απαντήσεων | Done |
| v0.10.27 | Context Pruner | 54% εξοικονόμηση tokens σε κώδικα (SWE-bench επίσημο), 46–47% σε γενικά έγγραφα (τοπικό IR), βαθμολόγηση BM25, τμηματοποίηση | Done |
| v0.10.28 | Memory Service | Προοδευτική αναζήτηση, backend διπλής εγγραφής | Done |

## Κοινότητα & Υποστήριξη

- **Issues**: [Παρακαλούμε αναφέρετε σφάλματα και αιτήματα χαρακτηριστικών εδώ](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **Συζητήσεις**: [Discord](https://discord.gg/b3AgFDxe9v)
- **Ακολουθήστε μας** στο [GitHub](https://github.com/IceWhaleTech)

## Άδεια Χρήσης

Αυτό το έργο αδειοδοτείται υπό την Άδεια MIT — δείτε το αρχείο [LICENSE](../../LICENSE) για λεπτομέρειες. Πιστεύουμε στον ανοιχτό κώδικα και στην ανταπόδοση στην κοινότητα.

## Συνεισφέροντες

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
