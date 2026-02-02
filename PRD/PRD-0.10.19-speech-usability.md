# PRD-0.10.19: Speech Usability Enhancement

## Overview

This PRD describes the enhancement of speech capabilities in ZimaOS-Echo v0.10.19, introducing multiple TTS providers (Edge-TTS, eSpeak-NG) alongside the existing Sherpa-ONNX implementation, with improved user experience, privacy controls, and voice customization options.

## Goals

1. **Expand TTS Provider Options**: Add Microsoft Edge-TTS (free, online) and eSpeak-NG (offline, lightweight)
2. **Privacy-First Design**: Implement user-level privacy consent for online services
3. **Lightweight Offline Support**: Integrate eSpeak-NG (~5MB) with 27 language support
4. **Voice Customization**: Support voice selection, pitch, rate, and volume controls
5. **Seamless Provider Switching**: Allow users to switch between providers without friction
6. **Voice Package Management**: Provide downloadable voice packs for eSpeak-NG

---

## 1. TTS Provider Architecture

### 1.1 Provider Comparison

| Feature | Sherpa-ONNX (Kokoro) | Edge-TTS | eSpeak-NG |
|---------|----------------------|----------|-----------|
| **Type** | Offline, Local | Online, Cloud | Offline, Local |
| **Quality** | ★★★★★ Excellent | ★★★★ Very Good | ★★★ Good |
| **Size** | ~82MB | None (online) | ~5MB base |
| **Languages** | 4+ | 100+ | 27+ |
| **Voices** | 9+ per language | 200+ | 1 per language |
| **Speed** | Fast | Medium | Very Fast |
| **Privacy** | Full local | Requires consent | Full local |
| **Cost** | Free | Free | Free |
| **Setup** | Download model | None | Optional voice packs |
| **Voice Control** | Limited | Good | Good |

### 1.2 Provider Implementation Strategy

**Sherpa-ONNX (Fallback)**:
- **Default**: NOT downloaded, NOT loaded on startup
- **Role**: Ultimate fallback option for users who want premium quality
- **Activation**: User must explicitly download model from settings
- **Support**: Multiple models (Kokoro, Piper) when activated

**Edge-TTS (Primary Online)**:
- Online service, no local setup required
- Requires user privacy consent (per-user, persistent)
- Support 100+ voices and languages
- Default option for users who accept online service

**eSpeak-NG (Primary Offline)**:
- Pure Go integration using purego (no CGO)
- ~5MB base binary, optional voice packs
- 27 language support
- Default lightweight offline option

**Provider Selection Flow**:
```
User opens app
    ↓
Check user TTS preference
    ├── Edge-TTS (default) → Check consent → Use if consented
    ├── eSpeak-NG → Use directly (no setup)
    └── Sherpa-ONNX → Check if downloaded → Use if available
```

---

## 2. Privacy & Consent Management

### 2.1 Privacy Consent Flow

**First-Time Edge-TTS Usage**:
```
User selects Edge-TTS provider
    ↓
Check user privacy consent status
    ├── Consent exists → Use service
    └── No consent → Show privacy dialog
            ↓
        User reviews privacy notice
            ├── Accept → Save consent, use service
            └── Decline → Switch to offline provider
```

### 2.2 Privacy Consent Dialog

```
┌─────────────────────────────────────────────────────┐
│ Privacy Notice: Edge-TTS Service                    │
├─────────────────────────────────────────────────────┤
│                                                     │
│ You are about to use Microsoft Edge-TTS, an        │
│ online text-to-speech service.                     │
│                                                     │
│ ⚠️  Privacy Information:                            │
│ • Your text will be sent to Microsoft servers      │
│ • Audio is generated in the cloud                  │
│ • Microsoft may log requests for service           │
│   improvement (see their privacy policy)           │
│ • No personal data is required                     │
│                                                     │
│ Alternatives:                                       │
│ • Sherpa-ONNX: Fully offline, high quality         │
│ • eSpeak-NG: Fully offline, lightweight            │
│                                                     │
│ [Learn More]  [Decline]  [Accept & Continue]      │
│                                                     │
│ ☐ Don't show this again for this account           │
│                                                     │
└─────────────────────────────────────────────────────┘
```

### 2.3 Consent Storage

**Database Schema**:
```sql
CREATE TABLE user_privacy_consent (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    service TEXT NOT NULL,        -- 'edge-tts', 'sherpa', 'espeak'
    consent_given BOOLEAN DEFAULT FALSE,
    consent_date TIMESTAMP,
    consent_version TEXT,         -- Track policy version
    ip_address TEXT,              -- For audit
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, service)
);
```

---

## 3. Edge-TTS Integration

### 3.1 Implementation Details

**Provider**: Microsoft Edge-TTS (free, no registration)
- Uses Edge browser's TTS endpoint
- 100+ voices across multiple languages
- Natural prosody and emotion support
- Rate, pitch, volume control

### 3.2 Supported Voices

**English**:
- en-US: 10+ voices (male/female)
- en-GB: 5+ voices
- en-AU: 3+ voices
- en-IN: 2+ voices

**Other Languages**: 90+ additional voices

### 3.3 API Implementation

```go
type EdgeTTSProvider struct {
    client           *http.Client
    consentManager   *ConsentManager
    voiceCache       map[string][]Voice
    cacheMu          sync.RWMutex
}

type EdgeTTSRequest struct {
    Text     string  `json:"text"`
    Voice    string  `json:"voice"`      // e.g., "en-US-AriaNeural"
    Rate     float32 `json:"rate"`       // 0.5 - 2.0
    Pitch    float32 `json:"pitch"`      // -50 - 50
    Volume   float32 `json:"volume"`     // 0 - 100
}
```

---

## 4. eSpeak-NG Integration

### 4.1 Implementation Strategy

**Pure Go Integration**:
- Use purego for FFI (no CGO required)
- Embed espeak-ng-data for base functionality
- Optional downloadable voice packs

### 4.2 Size Verification

**Base Package**:
- espeak-ng binary: ~2MB
- espeak-ng-data: ~3MB
- **Total**: ~5MB ✓

**Voice Packs** (optional):
- Each language pack: 100-500KB
- Users download only needed languages

### 4.3 Supported Languages (27+)

English, Spanish, French, German, Italian, Portuguese, Russian, Polish, Dutch, Swedish, Norwegian, Danish, Finnish, Czech, Slovak, Hungarian, Romanian, Greek, Turkish, Arabic, Hebrew, Persian, Chinese, Japanese, Korean, Vietnamese, Thai

### 4.4 Voice Customization

```go
type EspeakNGProvider struct {
    libPath      string
    dataPath     string
    voicePacks   map[string]bool  // Downloaded packs
    mu           sync.RWMutex
}

type EspeakNGRequest struct {
    Text     string  `json:"text"`
    Language string  `json:"language"`  // e.g., "en", "zh", "ja"
    Rate     int     `json:"rate"`      // 80-500 (words per minute)
    Pitch    int     `json:"pitch"`     // 0-99
    Volume   int     `json:"volume"`    // 0-100
}
```

---

## 5. Voice Package Management

### 5.1 Voice Pack Download UI

```
┌─────────────────────────────────────────────────────┐
│ eSpeak-NG Voice Packs                               │
├─────────────────────────────────────────────────────┤
│                                                     │
│ Base Package (Required)                             │
│ ✓ English (en)                    2.5 MB            │
│                                                     │
│ Additional Languages                                │
│ ○ Spanish (es)                    250 KB  [Download]│
│ ○ French (fr)                     280 KB  [Download]│
│ ○ German (de)                     300 KB  [Download]│
│ ○ Chinese (zh)                    350 KB  [Download]│
│ ○ Japanese (ja)                   400 KB  [Download]│
│ ○ Russian (ru)                    320 KB  [Download]│
│ ... (21 more languages)                             │
│                                                     │
│ [Select All]  [Clear All]                           │
│                                                     │
│ Total Size: 2.5 MB + selected packs                 │
│ [Download Selected]                                 │
│                                                     │
└─────────────────────────────────────────────────────┘
```

### 5.2 Voice Pack Storage

```
~/.local/share/zimaos-echo/espeak-ng/
├── espeak-ng-data/          (base, ~3MB)
├── voices/
│   ├── en/                  (included)
│   ├── es/                  (optional)
│   ├── fr/                  (optional)
│   ├── zh/                  (optional)
│   └── ...
└── .downloaded              (tracking file)
```

---

## 6. Voice Customization Features

### 6.1 Voice Selection UI

```
┌─────────────────────────────────────────────────────┐
│ Voice Settings                                      │
├─────────────────────────────────────────────────────┤
│                                                     │
│ TTS Provider: [Sherpa-ONNX ▼]                       │
│                                                     │
│ Voice: [Kokoro - Female (af) ▼]                     │
│                                                     │
│ Voice Preview:                                      │
│ "Hello, this is a voice preview."  [▶ Play]        │
│                                                     │
│ Speech Rate:    [━━━━●━━━━] 1.0x                   │
│ Pitch:          [━━━━●━━━━] Normal                 │
│ Volume:         [━━━━●━━━━] 100%                   │
│                                                     │
│ [Apply]  [Reset to Default]                         │
│                                                     │
└─────────────────────────────────────────────────────┘
```

### 6.2 Voice Control Parameters

**Sherpa-ONNX**:
- Voice selection (9+ voices per model)
- Speed: 0.5x - 2.0x
- Limited pitch/volume control

**Edge-TTS**:
- Voice selection (100+ voices)
- Rate: 0.5x - 2.0x
- Pitch: -50 to +50
- Volume: 0-100

**eSpeak-NG**:
- Language selection (27+)
- Rate: 80-500 WPM
- Pitch: 0-99
- Volume: 0-100

---

## 7. API Endpoints

### 7.1 Provider Management

```
GET    /api/tts/providers              - List all providers
GET    /api/tts/providers/:id/voices   - List voices for provider
POST   /api/tts/providers/:id/consent  - Set privacy consent
GET    /api/tts/providers/:id/consent  - Get consent status
```

### 7.2 Voice Packs (eSpeak-NG)

```
GET    /api/tts/espeak/packs           - List available packs
POST   /api/tts/espeak/packs/:lang     - Download language pack
DELETE /api/tts/espeak/packs/:lang     - Delete language pack
GET    /api/tts/espeak/packs/status    - Get download status
```

### 7.3 Synthesis

```
POST   /api/tts/synthesize
{
  "text": "Hello world",
  "provider": "edge-tts",             -- or "sherpa", "espeak"
  "voice": "en-US-AriaNeural",
  "rate": 1.0,
  "pitch": 0,
  "volume": 100
}
```

---

## 8. Implementation Phases

### Phase 1: Privacy & Consent Framework
- [ ] Create user_privacy_consent table
- [ ] Implement ConsentManager
- [ ] Add privacy consent dialog UI
- [ ] Store/retrieve consent per user

### Phase 2: Edge-TTS Integration
- [ ] Implement EdgeTTSProvider
- [ ] Add voice listing endpoint
- [ ] Integrate with consent check
- [ ] Add voice preview functionality

### Phase 3: eSpeak-NG Integration
- [ ] Implement EspeakNGProvider (purego)
- [ ] Create voice pack download system
- [ ] Add language pack management UI
- [ ] Implement voice customization

### Phase 4: Provider Switching
- [ ] Update TTS settings UI
- [ ] Add provider selection dropdown
- [ ] Implement fallback logic
- [ ] Add provider health checks

### Phase 5: Voice Customization
- [ ] Add rate/pitch/volume controls
- [ ] Implement voice preview
- [ ] Save user preferences
- [ ] Add preset voices

---

## 9. Technical Specifications

### 9.1 eSpeak-NG Purego Integration

**No CGO Required**:
```go
// Load espeak-ng library dynamically
libPath := filepath.Join(dataDir, "espeak-ng.dll")  // Windows
libHandle, _ := purego.Dlopen(libPath, purego.RTLD_LAZY)

// Register functions
var espeak_Initialize func() int
purego.RegisterLibFunc(&espeak_Initialize, libHandle, "espeak_Initialize")
```

**Size Confirmation**: ~5MB total (verified)

### 9.2 Edge-TTS Implementation

**No Local Dependencies**:
- HTTP client only
- Uses public Edge-TTS endpoint
- No authentication required
- Rate limiting: ~100 requests/minute

### 9.3 Fallback Strategy

```
User requests TTS
    ↓
Try preferred provider
    ├── Success → Return audio
    └── Fail (offline/error)
            ↓
        Try next available provider
            ├── Success → Return audio
            └── Fail
                    ↓
                Try next provider
                    ├── Success → Return audio
                    └── Fail → Return error
```

**Fallback Order** (automatic when preferred fails):
1. User's preferred provider
2. eSpeak-NG (always available, no setup)
3. Edge-TTS (if online & consented)
4. Sherpa-ONNX (if model downloaded)

---

## 10. Success Metrics

- [ ] Edge-TTS: 100+ voices available
- [ ] eSpeak-NG: 27+ languages supported, <5MB base size
- [ ] Privacy: 100% user consent before Edge-TTS usage
- [ ] Voice customization: Rate, pitch, volume controls working
- [ ] Provider switching: <100ms latency
- [ ] Voice packs: Download/delete working smoothly

---

## 11. Open Questions & Confirmations Needed

1. **eSpeak-NG Size**: Confirm ~5MB base + optional packs ✓
2. **Purego Integration**: Feasible without CGO? ✓
3. **Voice Selection**: Support pitch/rate/volume for all providers? ✓
4. **Privacy Consent**: Per-user, persistent storage? ✓
5. **Fallback Logic**: Automatic or user-controlled? → User-controlled preferred

---

## References

- [Microsoft Edge-TTS](https://github.com/rany2/edge-tts)
- [eSpeak-NG](https://github.com/espeak-ng/espeak-ng)
- [eSpeak-NG Go Bindings](https://github.com/go-echarts/go-echarts)
- [Purego FFI](https://github.com/ebitengine/purego)
- [Sherpa-ONNX](https://github.com/k2-fsa/sherpa-onnx)
