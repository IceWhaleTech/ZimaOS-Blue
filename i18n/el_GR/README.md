# ZimaOS Echo

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Echo" width="200">
</p>

<p align="center">
  <strong>Εγγενές Περιβάλλον Εκτέλεσης Πρακτόρων για NAS</strong>
</p>

<p align="center">
  <a href="../../README.md">English</a> | <a href="../zh_CN/README.md">中文</a> | <strong>Ελληνικά</strong>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Echo/ci.yml?branch=main&style=for-the-badge" alt="Κατάσταση CI"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Echo?include_prereleases&style=for-the-badge" alt="Έκδοση GitHub"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="Άδεια MIT"></a>
</p>

**ZimaOS Echo** είναι ένα ελαφρύ, υψηλής απόδοσης περιβάλλον εκτέλεσης πρακτόρων AI σχεδιασμένο ειδικά για συσκευές NAS και edge. Κατασκευασμένο με Go, παρέχει μια πλατφόρμα έτοιμη για παραγωγή για την εκτέλεση βοηθών AI σε υλικό χαμηλής κατανάλωσης.

[Τεκμηρίωση](https://echo.zimaos.com) · [Γρήγορη Εκκίνηση](#γρήγορη-εκκίνηση) · [Χαρακτηριστικά](#χαρακτηριστικά) · [Σύγκριση](#σύγκριση-με-clawdbot)

## Γιατί ZimaOS Echo;

Το ZimaOS Echo είναι εμπνευσμένο από το [clawdbot](https://github.com/clawdbot/clawdbot) αλλά ξαναχτισμένο από την αρχή σε Go για:

- **Χαμηλότερη Χρήση Πόρων**: Τρέχει σε συσκευές με μόλις 256MB RAM
- **Καλύτερη Απόδοση**: Εγγενές Go binary με αποδοτική ταυτόχρονη εκτέλεση βασισμένη σε goroutines
- **Ευκολότερη Ανάπτυξη**: Ένα μόνο binary, δεν απαιτείται Node.js
- **Βελτιστοποίηση για NAS**: Σχεδιασμένο για λειτουργία 24/7 σε συσκευές χαμηλής κατανάλωσης

## Γρήγορη Εκκίνηση

### Linux / macOS

```bash
curl -fsSL https://echo.zimaos.com/install.sh | sudo bash
```

### Windows (PowerShell ως Διαχειριστής)

```powershell
irm https://echo.zimaos.com/install.ps1 | iex
```

### Από τον Πηγαίο Κώδικα

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo/server
go build -o zimaos-echo ./cmd/server
./zimaos-echo server
```

## Χαρακτηριστικά

### Βασικά Χαρακτηριστικά

- 🚀 **Ελαφρύ**: Ένα binary < 15MB, μνήμη < 80MB
- ⚡ **Υψηλή Απόδοση**: Βασισμένο σε Go με ταυτόχρονη εκτέλεση goroutines
- 🔌 **Πολλαπλοί Πάροχοι**: OpenAI, Anthropic, Ollama και άλλοι
- 🛡️ **Έτοιμο για Παραγωγή**: Circuit breaker, κομψή υποβάθμιση, αυτόματη ανάκτηση
- 📊 **Παρατηρήσιμο**: Μετρικές Prometheus, προφίλ pprof, δομημένη καταγραφή
- 🔄 **Hot Reload**: Αλλαγές διαμόρφωσης χωρίς επανεκκίνηση
- 💾 **Αντίγραφα Ασφαλείας/Επαναφορά**: Αυτοματοποιημένα αντίγραφα με επαναφορά σε συγκεκριμένο χρονικό σημείο

### Ενσωμάτωση Έξυπνου Σπιτιού

- 🏠 **Home Assistant**: Εγγενής ενσωμάτωση με το API του Home Assistant
- 💡 **Έλεγχος Συσκευών**: Φώτα, διακόπτες, αισθητήρες, κλιματισμός και άλλα
- 🤖 **Αυτοματισμός AI**: Εντολές φυσικής γλώσσας για έλεγχο έξυπνου σπιτιού
- 📡 **Συμβάντα Πραγματικού Χρόνου**: Εγγραφή σε αλλαγές κατάστασης συσκευών μέσω WebSocket

### Φωνητικές Δυνατότητες

- 🎤 **Αναγνώριση Ομιλίας**: Μετατροπή ομιλίας σε κείμενο βασισμένη στο Whisper
- 🔊 **Κείμενο σε Ομιλία**: Υποστήριξη πολλαπλών μηχανών TTS
- 👂 **Φωνητική Αφύπνιση**: Προσαρμόσιμη ανίχνευση λέξης αφύπνισης
- 🗣️ **Φωνητικές Εντολές**: Αλληλεπίδραση με τον βοηθό AI χωρίς χέρια

### Αρχιτεκτονική Πολλαπλών Ενοικιαστών

- 👥 **Απομόνωση Ενοικιαστών**: Πλήρης απομόνωση δεδομένων και πόρων
- 🔐 **Αυθεντικοποίηση ανά Ενοικιαστή**: Ανεξάρτητη αυθεντικοποίηση ανά ενοικιαστή
- 📊 **Ποσοστώσεις Πόρων**: Όρια CPU, μνήμης και ρυθμού API ανά ενοικιαστή
- 🎛️ **Πίνακας Ελέγχου Ενοικιαστή**: Πύλη αυτοεξυπηρέτησης

### Κανάλια Επικοινωνίας

- 💬 **Πρωτόκολλο Matrix**: Αποκεντρωμένη, κρυπτογραφημένη από άκρο σε άκρο ανταλλαγή μηνυμάτων
- 📱 **Telegram/Discord/Slack**: Υποστήριξη δημοφιλών πλατφορμών μηνυμάτων
- 📞 **Signal/WhatsApp**: Ενσωμάτωση ασφαλών μηνυμάτων
- 🍎 **iMessage**: Εγγενής υποστήριξη iMessage για macOS

### Ασφάλεια και Αυθεντικοποίηση

- 🔑 **WebAuthn/Passkeys**: Αυθεντικοποίηση χωρίς κωδικό με FIDO2
- 🔐 **OIDC/OAuth 2.0**: Επιχειρησιακό SSO (Google, GitHub, Okta, κλπ.)
- 📲 **MFA/TOTP**: Υποστήριξη αυθεντικοποίησης πολλαπλών παραγόντων
- 🛡️ **RBAC**: Λεπτομερής έλεγχος πρόσβασης βασισμένος σε ρόλους
- 📝 **Καταγραφή Ελέγχου**: Ολοκληρωμένο ίχνος ελέγχου ασφαλείας
- 🔒 **Sandbox**: Απομονωμένο περιβάλλον εκτέλεσης για εργαλεία

### Frontend

- 🎨 **Πίνακας Ελέγχου Vue 3**: Σύγχρονη, responsive διεπαφή ιστού
- 💬 **Διεπαφή Συνομιλίας**: Απαντήσεις streaming με υποστήριξη Markdown
- 📈 **Παρακολούθηση Συστήματος**: Γραφήματα χρήσης πόρων σε πραγματικό χρόνο
- ⚙️ **UI Ρυθμίσεων**: Εύκολη διαχείριση διαμόρφωσης

## Σύγκριση με Clawdbot

Το ZimaOS Echo είναι εμπνευσμένο από το clawdbot αλλά βελτιστοποιημένο για ανάπτυξη NAS/edge:

| Χαρακτηριστικό | ZimaOS Echo | Clawdbot |
|----------------|-------------|----------|
| **Γλώσσα** | Go | TypeScript/Node.js |
| **Μέγεθος Binary** | ~15MB | ~200MB+ (με node_modules) |
| **Χρήση Μνήμης** | ~80MB σε αδράνεια | ~200MB+ σε αδράνεια |
| **Χρόνος Εκκίνησης** | < 1s | 3-5s |
| **Runtime** | Εγγενές binary | Απαιτεί Node.js |
| **Πλατφόρμα Στόχος** | Συσκευές NAS/Edge | Desktop/Server |

## Αρχιτεκτονική

```
┌─────────────────────────────────────────────────┐
│                  ZimaOS-Echo                     │
├─────────────────────────────────────────────────┤
│  Vue 3 Frontend  │  REST API  │  WebSocket      │
├─────────────────────────────────────────────────┤
│              Κύριο Runtime (Go)                  │
│  Event Loop │ Worker Pool │ Config │ Logger     │
├─────────────────────────────────────────────────┤
│              Agent Runtime                       │
│  LLM Provider │ Εργαλεία │ Μνήμη │ Πλαίσιο     │
├─────────────────────────────────────────────────┤
│              Επίπεδο Δεδομένων                   │
│  SQLite (Zorm) │ ECache │ Αρχεία                │
└─────────────────────────────────────────────────┘
```

## Οδικός Χάρτης

- [x] **v0.1.0** - Κύριο Runtime (Event loop, Worker pool, Config, Logger)
- [x] **v0.2.0** - Agent Runtime (Πάροχοι LLM συμπ. AWS Bedrock, Εργαλεία, Μνήμη)
- [x] **v0.3.0** - Επίπεδο API (REST, WebSocket, Streaming)
- [x] **v0.4.0** - Σύστημα Plugins (Go modules, υποστήριξη WASM)
- [x] **v0.5.0** - Έτοιμο για Παραγωγή (Μετρικές, Προφίλ, Αντίγραφα)
- [x] **v0.6.0** - Κανάλια Μηνυμάτων (Telegram, Discord, Slack, WhatsApp, Signal, iMessage)
- [x] **v0.7.0** - Ασφάλεια (OIDC, MFA, WebAuthn, Καταγραφή ελέγχου, Sandbox)
- [x] **v0.8.0** - Απόδοση (ECache, Zorm, HTTP/2)
- [x] **v0.9.0** - Μελλοντικές Βελτιώσεις (A2UI, Αυτοματισμός Browser)
- [ ] **v1.0.0** - RAG & Βάση Γνώσεων

## Συνεισφορά

Οι συνεισφορές είναι ευπρόσδεκτες! Παρακαλούμε διαβάστε τον [Οδηγό Συνεισφοράς](CONTRIBUTING.md) για λεπτομέρειες.

```bash
# Κλωνοποίηση αποθετηρίου
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo

# Εγκατάσταση εξαρτήσεων
cd server && go mod download

# Εκτέλεση δοκιμών
go test ./...

# Κατασκευή
go build -o zimaos-echo ./cmd/server
```

## Άδεια

Άδεια MIT - δείτε [LICENSE](LICENSE) για λεπτομέρειες.

## Ευχαριστίες

- [clawdbot](https://github.com/clawdbot/clawdbot) - Έμπνευση για το έργο
- [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) - Ελαφρύ ORM

---

<p align="center">
  Φτιαγμένο με ❤️ από την <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
