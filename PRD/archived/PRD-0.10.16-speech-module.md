# PRD-0.10.16: Speech Module

## Overview

This PRD describes the implementation of a Speech module that provides Automatic Speech Recognition (ASR) and Text-to-Speech (TTS) capabilities using locally-running ONNX models. The module supports model downloading with resume capability, model switching, and seamless integration with the chat interface.

## Goals

1. Provide offline ASR using CTranslate2-ONNX compatible models (faster-whisper distil variants)
2. Provide offline TTS using ONNX-based models
3. Support model downloading with resume capability (resumable downloads)
4. Guide users to download required models when attempting to use speech features
5. Allow text editing before sending transcribed messages
6. Support walkie-talkie mode without text display

---

## 1. Model Architecture

### 1.1 ASR Models (Speech-to-Text)

**Technology Stack**:
- Runtime: sherpa-onnx (Go bindings)
- Model Format: ONNX
- Existing Implementation: `server/internal/stt/sherpa.go`, `server/internal/stt/sherpa_download.go`

**Supported Models** (via sherpa-onnx):
| Model ID | Name | Size | Languages | Streaming |
|----------|------|------|-----------|-----------|
| `whisper-tiny` | Whisper Tiny | ~75MB | Multilingual | No |
| `whisper-base` | Whisper Base | ~150MB | Multilingual | No |
| `zipformer-en` | Zipformer English | ~50MB | English | Yes |
| `paraformer-zh` | Paraformer Chinese | ~220MB | Chinese | No |
| `sensevoice-small` | SenseVoice Small | ~100MB | ZH/EN/JA/KO/YUE | No |

**Model Selection Criteria**:
- Real-time/Streaming: `zipformer-en` (English only, ~50MB)
- Fast multilingual: `whisper-tiny` (~75MB)
- Best multilingual: `sensevoice-small` (~100MB, includes emotion detection)
- Chinese focused: `paraformer-zh` (~220MB, highest accuracy)

### 1.2 TTS Models (Text-to-Speech)

**Technology Stack**:
- Runtime: sherpa-onnx (Go bindings)
- Model Format: ONNX
- Existing Implementation: `server/internal/tts/sherpa.go`, `server/internal/tts/sherpa_download.go`

**Supported Models** (via sherpa-onnx):
| Model ID | Name | Size | Languages | Voices | Quality |
|----------|------|------|-----------|--------|---------|
| `kokoro-en` | Kokoro English v0.19 | ~82MB | English | 9 voices | ★★★★★ |
| `kokoro-multi` | Kokoro Multilingual v1.0 | ~100MB | EN/ZH/JA/KO | Multiple | ★★★★★ |
| `piper-en` | Piper Lessac (US) | ~60MB | English | 1 voice | ★★★★ |
| `vits-zh` | VITS AISHELL3 | ~80MB | Chinese | Multiple | ★★★★ |

**Kokoro Voice Options** (kokoro-en):
- `af` - American Female (default)
- `af_bella` - Bella (American Female)
- `af_sarah` - Sarah (American Female)
- `am_adam` - Adam (American Male)
- `am_michael` - Michael (American Male)
- `bf_emma` - Emma (British Female)
- `bf_isabella` - Isabella (British Female)
- `bm_george` - George (British Male)
- `bm_lewis` - Lewis (British Male)

**Model Selection Criteria**:
- Best quality (small size): Kokoro - state-of-the-art quality at only ~82MB
- Multilingual: `kokoro-multi` supports EN/ZH/JA/KO in single model
- Low latency: Piper models for fastest inference
- Chinese: `vits-zh` or `kokoro-multi`

---

## 2. Model Management

### 2.1 Model Storage Structure

```
data/
└── models/
    └── speech/
        ├── asr/
        │   ├── whisper-distil-small-en/
        │   │   ├── model.onnx
        │   │   ├── config.json
        │   │   └── tokenizer.json
        │   └── whisper-distil-large-v3/
        │       └── ...
        └── tts/
            ├── piper-en-us-amy/
            │   ├── model.onnx
            │   └── config.json
            └── piper-zh-cn-huayan/
                └── ...
```

### 2.2 Model Download Sources

**Multi-Source Download Strategy**:
The system attempts to download models from multiple sources for reliability and speed:

1. **Primary Source**: HuggingFace (huggingface.co)
   - Global CDN with good international coverage
   - Direct model file downloads

2. **Mirror Source**: HF-Mirror (hf-mirror.com)
   - HuggingFace mirror with better connectivity for China users
   - Same URL structure as HuggingFace

3. **Fallback Source**: ModelScope (modelscope.cn)
   - Chinese model hosting platform
   - Mirror of popular models

**Download Logic**:
```
Start Download
    │
    ▼
Try HuggingFace URL
    │
    ├── Success → Continue download
    │
    └── Fail (timeout/error)
            │
            ▼
        Try HF-Mirror URL
            │
            ├── Success → Continue download
            │
            └── Fail (timeout/error)
                    │
                    ▼
                Try ModelScope URL
                    │
                    ├── Success → Continue download
                    │
                    └── Fail → Report error to user
```

**URL Mapping**:
| Model | HuggingFace | HF-Mirror | ModelScope |
|-------|-------------|-----------|------------|
| kokoro-en | `huggingface.co/k2-fsa/...` | `hf-mirror.com/k2-fsa/...` | `modelscope.cn/models/...` |
| whisper-tiny | `huggingface.co/k2-fsa/...` | `hf-mirror.com/k2-fsa/...` | `modelscope.cn/models/...` |

### 2.3 Model Registry

**Database Schema**:
```sql
CREATE TABLE speech_models (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,              -- 'asr' or 'tts'
    name TEXT NOT NULL,
    description TEXT,
    size_bytes INTEGER NOT NULL,
    languages TEXT,                  -- JSON array
    download_url TEXT NOT NULL,
    checksum TEXT NOT NULL,          -- SHA256
    version TEXT NOT NULL,
    is_downloaded INTEGER DEFAULT 0,
    is_active INTEGER DEFAULT 0,
    download_progress INTEGER DEFAULT 0,
    downloaded_bytes INTEGER DEFAULT 0,
    local_path TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_speech_models_type ON speech_models(type);
CREATE INDEX idx_speech_models_active ON speech_models(is_active);
```

### 2.3 Resumable Download Implementation

**Download State Tracking**:
```go
type DownloadState struct {
    ModelID        string    `json:"model_id"`
    URL            string    `json:"url"`
    TotalBytes     int64     `json:"total_bytes"`
    DownloadedBytes int64    `json:"downloaded_bytes"`
    TempFilePath   string    `json:"temp_file_path"`
    Checksum       string    `json:"checksum"`
    StartedAt      time.Time `json:"started_at"`
    LastUpdatedAt  time.Time `json:"last_updated_at"`
    Status         string    `json:"status"` // pending, downloading, paused, completed, failed
}
```

**Resume Logic**:
```
Start Download
    │
    ▼
Check for existing temp file
    │
    ├── Exists with valid state
    │       │
    │       ▼
    │   Send Range header: bytes={downloaded}-
    │       │
    │       ▼
    │   Append to existing file
    │
    └── No existing file
            │
            ▼
        Start fresh download
            │
            ▼
        Create temp file with .part extension

    │
    ▼
On completion:
    - Verify checksum
    - Rename .part to final name
    - Update database
    - Clean up state file
```

**HTTP Headers for Resume**:
```
Request:
  Range: bytes=1048576-

Response (206 Partial Content):
  Content-Range: bytes 1048576-5242880/5242880
  Content-Length: 4194304
```

---

## 3. User Experience Flow

### 3.1 No Model Installed - Guided Download

**Scenario**: User clicks microphone button but no ASR model is installed.

**Flow**:
```
User clicks microphone
    │
    ▼
Check for active ASR model
    │
    ├── Model exists and loaded
    │       │
    │       ▼
    │   Start recording
    │
    └── No model available
            │
            ▼
        Show model download dialog
            │
            ▼
        User selects model
            │
            ▼
        Start download with progress
            │
            ▼
        On complete: Auto-activate and retry
```

**Download Dialog UI**:
```
┌─────────────────────────────────────────────────┐
│  Speech Recognition Model Required              │
│                                                 │
│  To use voice input, please download a speech  │
│  recognition model.                             │
│                                                 │
│  Recommended Models:                            │
│                                                 │
│  ○ Distil Whisper Small (English)    [250 MB]  │
│    Fast, English only                           │
│                                                 │
│  ● Distil Whisper Large v3           [1.5 GB]  │
│    Best quality, multilingual         ★ Recommended │
│                                                 │
│  ○ Whisper Medium                    [1.5 GB]  │
│    Good quality, multilingual                   │
│                                                 │
│  [Cancel]                    [Download & Install] │
└─────────────────────────────────────────────────┘
```

### 3.2 TTS Model Download Flow

**Scenario**: User clicks play button on a message but no TTS model is installed.

**Flow**:
```
User clicks play/speak button
    │
    ▼
Check for active TTS model
    │
    ├── Model exists and loaded
    │       │
    │       ▼
    │   Generate and play audio
    │
    └── No model available
            │
            ▼
        Show TTS model download dialog
            │
            ▼
        User selects voice/model
            │
            ▼
        Start download with progress
            │
            ▼
        On complete: Auto-activate and play
```

### 3.3 Transcription with Edit Before Send

**Flow**:
```
User holds/clicks microphone
    │
    ▼
Start recording (show waveform)
    │
    ▼
User releases/clicks stop
    │
    ▼
Transcribe audio
    │
    ▼
Show transcription in editable text area
    │
    ├── User edits text (optional)
    │
    ▼
User clicks Send or presses Enter
    │
    ▼
Send message
```

**UI During Transcription**:
```
┌─────────────────────────────────────────────────┐
│ ┌─────────────────────────────────────────────┐ │
│ │ Hello, can you help me with my code?       │ │
│ │ I'm having trouble with the API.           │ │
│ │                                      [Edit]│ │
│ └─────────────────────────────────────────────┘ │
│                                                 │
│ [Cancel]                              [Send ➤] │
└─────────────────────────────────────────────────┘
```

### 3.4 Walkie-Talkie Mode

**Description**: Push-to-talk mode that sends voice directly without showing transcription.

**Behavior**:
- Press and hold to record
- Release to send
- No text preview shown
- Audio is transcribed server-side and sent as text message
- Optional: Play TTS response automatically

**UI**:
```
┌─────────────────────────────────────────────────┐
│                                                 │
│              🎤 Recording...                    │
│              ████████░░░░ 0:03                  │
│                                                 │
│         Release to send, swipe to cancel        │
│                                                 │
└─────────────────────────────────────────────────┘
```

---

## 4. API Endpoints

### 4.1 Model Management

```
GET    /api/speech/models                    - List all available models
GET    /api/speech/models/:type              - List models by type (asr/tts)
GET    /api/speech/models/:id                - Get model details
POST   /api/speech/models/:id/download       - Start model download
DELETE /api/speech/models/:id/download       - Cancel download
GET    /api/speech/models/:id/download       - Get download progress
POST   /api/speech/models/:id/activate       - Set as active model
DELETE /api/speech/models/:id                - Delete downloaded model
```

### 4.2 Speech Processing

```
POST   /api/speech/transcribe                - Transcribe audio to text
POST   /api/speech/synthesize                - Synthesize text to audio
GET    /api/speech/status                    - Get speech service status
```

### 4.3 Request/Response Examples

**Start Download**:
```
POST /api/speech/models/whisper-distil-large-v3/download

Response:
{
  "model_id": "whisper-distil-large-v3",
  "status": "downloading",
  "total_bytes": 1610612736,
  "downloaded_bytes": 0,
  "progress": 0
}
```

**Download Progress** (SSE):
```
GET /api/speech/models/whisper-distil-large-v3/download

event: progress
data: {"downloaded_bytes": 104857600, "total_bytes": 1610612736, "progress": 6.5, "speed_bps": 5242880}

event: progress
data: {"downloaded_bytes": 209715200, "total_bytes": 1610612736, "progress": 13.0, "speed_bps": 5500000}

event: complete
data: {"status": "completed", "model_id": "whisper-distil-large-v3"}
```

**Transcribe Audio**:
```
POST /api/speech/transcribe
Content-Type: multipart/form-data

audio: <binary audio data>
language: auto  (optional, default: auto-detect)
format: wav     (optional, default: auto-detect)

Response:
{
  "text": "Hello, can you help me with my code?",
  "language": "en",
  "confidence": 0.95,
  "duration_ms": 2340,
  "segments": [
    {"start": 0.0, "end": 1.2, "text": "Hello,"},
    {"start": 1.2, "end": 2.34, "text": "can you help me with my code?"}
  ]
}
```

**Synthesize Speech**:
```
POST /api/speech/synthesize
Content-Type: application/json

{
  "text": "Hello, how can I help you today?",
  "voice": "piper-en-us-amy",
  "speed": 1.0,
  "format": "wav"
}

Response:
Content-Type: audio/wav
<binary audio data>
```

---

## 5. Settings UI

### 5.1 Speech Settings Page

```
┌─────────────────────────────────────────────────────────────┐
│ Speech Settings                                             │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│ Speech Recognition (ASR)                                    │
│ ─────────────────────────────────────────────────────────── │
│                                                             │
│ Active Model: [Distil Whisper Large v3        ▼]           │
│                                                             │
│ Downloaded Models:                                          │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │ ● Distil Whisper Large v3          1.5 GB    [Delete]  │ │
│ │ ○ Distil Whisper Small (EN)        250 MB    [Delete]  │ │
│ └─────────────────────────────────────────────────────────┘ │
│                                                             │
│ [+ Download More Models]                                    │
│                                                             │
│ Text-to-Speech (TTS)                                        │
│ ─────────────────────────────────────────────────────────── │
│                                                             │
│ Active Voice: [Piper Amy (US English)         ▼]           │
│                                                             │
│ Downloaded Voices:                                          │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │ ● Kokoro 82M                       82 MB     [Delete]  │ │
│ │ ○ Piper Amy (US)                   60 MB     [Delete]  │ │
│ │ ○ MeloTTS Chinese                  100 MB    [Delete]  │ │
│ └─────────────────────────────────────────────────────────┘ │
│                                                             │
│ [+ Download More Voices]                                    │
│                                                             │
│ Speech Speed: [━━━━━━━●━━━] 1.0x                           │
│                                                             │
│ Walkie-Talkie Mode                                          │
│ ─────────────────────────────────────────────────────────── │
│                                                             │
│ [✓] Enable walkie-talkie mode                              │
│ [✓] Auto-play TTS responses                                │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### 5.2 Download Progress UI

```
┌─────────────────────────────────────────────────────────────┐
│ Downloading: Distil Whisper Large v3                        │
│                                                             │
│ ████████████████████░░░░░░░░░░░░░░░░░░░░  45%              │
│                                                             │
│ 675 MB / 1.5 GB                          12.5 MB/s         │
│ Estimated time remaining: 1 min 10 sec                      │
│                                                             │
│ [Pause]                                          [Cancel]   │
└─────────────────────────────────────────────────────────────┘
```

---

## 6. Data Models

### 6.1 Go Structs

```go
// SpeechModel represents a downloadable speech model
type SpeechModel struct {
    ID              string    `json:"id"`
    Type            string    `json:"type"` // "asr" or "tts"
    Name            string    `json:"name"`
    Description     string    `json:"description"`
    SizeBytes       int64     `json:"size_bytes"`
    Languages       []string  `json:"languages"`
    DownloadURL     string    `json:"download_url"`
    Checksum        string    `json:"checksum"`
    Version         string    `json:"version"`
    IsDownloaded    bool      `json:"is_downloaded"`
    IsActive        bool      `json:"is_active"`
    DownloadProgress int      `json:"download_progress"`
    DownloadedBytes int64     `json:"downloaded_bytes"`
    LocalPath       string    `json:"local_path,omitempty"`
    CreatedAt       time.Time `json:"created_at"`
    UpdatedAt       time.Time `json:"updated_at"`
}

// TranscriptionResult represents ASR output
type TranscriptionResult struct {
    Text       string              `json:"text"`
    Language   string              `json:"language"`
    Confidence float64             `json:"confidence"`
    DurationMs int64               `json:"duration_ms"`
    Segments   []TranscriptSegment `json:"segments,omitempty"`
}

// TranscriptSegment represents a timed segment
type TranscriptSegment struct {
    Start float64 `json:"start"`
    End   float64 `json:"end"`
    Text  string  `json:"text"`
}

// SynthesisRequest represents TTS input
type SynthesisRequest struct {
    Text   string  `json:"text"`
    Voice  string  `json:"voice"`
    Speed  float64 `json:"speed"`
    Format string  `json:"format"`
}
```

---

## 7. Implementation Tasks

### Phase 1: Model Infrastructure
- [ ] Create `speech_models` database table
- [ ] Implement model registry with available models
- [ ] Create model storage directory structure
- [ ] Implement resumable download manager

### Phase 2: ASR Integration
- [ ] Integrate CTranslate2-ONNX Go bindings
- [ ] Implement Whisper model loading
- [ ] Create transcription API endpoint
- [ ] Add audio format conversion (WebM/MP3 to WAV)

### Phase 3: TTS Integration
- [ ] Integrate multi-framework TTS support (Kokoro, Piper, MeloTTS)
- [ ] Implement TTS model loading with ONNX runtime
- [ ] Create synthesis API endpoint
- [ ] Add audio streaming support

### Phase 4: Frontend - Model Management
- [ ] Create Speech Settings page
- [ ] Implement model download UI with progress
- [ ] Add model switching functionality
- [ ] Implement download pause/resume UI

### Phase 5: Frontend - Chat Integration
- [ ] Add microphone button to chat input
- [ ] Implement recording UI with waveform
- [ ] Create transcription preview with edit
- [ ] Add TTS playback button to messages

### Phase 6: Walkie-Talkie Mode
- [ ] Implement push-to-talk recording
- [ ] Add direct send without preview
- [ ] Implement auto-play TTS responses
- [ ] Add visual feedback during recording

---

## 8. Technical Notes

### 8.1 Files to Create

**Backend**:
- `server/internal/speech/manager.go` - Model management
- `server/internal/speech/download.go` - Resumable download logic
- `server/internal/speech/asr.go` - ASR inference wrapper
- `server/internal/speech/tts.go` - TTS inference wrapper
- `server/internal/server/speech_handler.go` - API handlers

**Frontend**:
- `web/src/views/SpeechSettingsView.vue` - Settings page
- `web/src/components/speech/ModelDownloadDialog.vue` - Download dialog
- `web/src/components/speech/VoiceRecorder.vue` - Recording component
- `web/src/components/speech/TranscriptionPreview.vue` - Edit before send
- `web/src/components/speech/WalkieTalkieButton.vue` - PTT button
- `web/src/api/speech.ts` - API client

### 8.2 Dependencies

**Go**:
- `github.com/nicksherron/go-ctranslate2` or similar ONNX runtime
- `github.com/onnx/onnx-go` for ONNX model loading

**Frontend**:
- Web Audio API for recording
- MediaRecorder API for audio capture

### 8.3 Model Sources

**ASR Models**:
- Hugging Face: `distil-whisper/distil-large-v3`
- Convert to ONNX using `optimum` library

**TTS Models**:
- Kokoro: https://github.com/hexgrad/kokoro (~82MB, state-of-the-art quality)
- Piper: https://github.com/rhasspy/piper (~60MB per voice)
- MeloTTS: https://github.com/myshell-ai/MeloTTS (~100MB)
- Pre-converted ONNX models available on Hugging Face

---

## 9. Success Metrics

1. **Download Reliability**: 99% successful downloads with resume
2. **Transcription Accuracy**: >95% WER on clear audio
3. **Latency**: <2s for 10s audio transcription
4. **TTS Quality**: Natural-sounding output at 1x speed

---

## 10. Open Questions

1. Should we support real-time streaming transcription?
2. What is the maximum audio duration to support?
3. Should we add voice activity detection (VAD)?
4. Do we need speaker diarization for multi-speaker audio?

---

## References

- [Faster Whisper](https://github.com/SYSTRAN/faster-whisper)
- [Distil Whisper](https://github.com/huggingface/distil-whisper)
- [Kokoro TTS](https://github.com/hexgrad/kokoro) - 82M parameter high-quality TTS
- [Piper TTS](https://github.com/rhasspy/piper)
- [MeloTTS](https://github.com/myshell-ai/MeloTTS)
- [ONNX Runtime](https://onnxruntime.ai/)
- [CTranslate2](https://github.com/OpenNMT/CTranslate2)
