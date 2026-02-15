# PRD: Humanizer — Natural Language Response Filter

**Version**: 0.10.26
**Author**: ZimaOS-Blue Team
**Status**: Draft
**Created**: 2026-02-15

---

## 1. Overview

### 1.1 Background

ZimaOS Blue's AI assistant generates responses using LLMs that naturally produce Markdown-formatted text — bold markers (`**`), code fences (`` ``` ``), headers (`##`), bullet points, inline code backticks, and other formatting artifacts. While this formatting renders well in the web chat UI, it creates two significant problems:

1. **IM Channels** (Telegram, Discord, Slack, WeChat, etc.): Many channels have limited or inconsistent Markdown support. Raw Markdown artifacts appear as literal characters, making messages look robotic and cluttered. Even channels that support Markdown often render it differently, leading to broken formatting.

2. **Voice / TTS Output**: Text-to-speech engines read Markdown syntax literally — "asterisk asterisk hello asterisk asterisk" instead of "hello". Code blocks, URLs, and formatting symbols produce unintelligible speech output.

Currently, the only text cleaning is `stripEmojis()` in the voice service (`server/internal/voice/service.go:22`). There is no shared text normalization layer for IM channel replies.

### 1.2 Goals

1. Create a shared `humanizer` package that transforms LLM Markdown output into natural, human-readable text
2. Support two processing modes: **IM mode** (light cleanup, preserve structure) and **Voice mode** (aggressive cleanup, optimized for speech)
3. Integrate seamlessly into the existing channel `Send()` pipeline and voice `Synthesize()` pipeline
4. Make the feature configurable and enabled by default
5. Subsume the existing `stripEmojis()` function into the humanizer

### 1.3 Non-Goals

1. Translating or rewriting the content (only formatting cleanup)
2. Content moderation or safety filtering (handled separately by prompt guard)
3. Modifying the web chat UI rendering pipeline (Markdown renders correctly there)
4. Per-channel Markdown dialect conversion (e.g., Telegram MarkdownV2 escaping)

---

## 2. User Stories

### Story 1: Clean IM Replies
**As a** user chatting with Blue via Telegram
**I want** replies to read like natural chat messages
**So that** I don't see raw `**bold**`, `` `code` ``, or `### headers` in my conversation

**Acceptance Criteria:**
- [ ] Markdown bold/italic markers are stripped, leaving plain text
- [ ] Code fences are converted to plain text or simplified notation
- [ ] Headers are converted to plain text (content preserved, `#` symbols removed)
- [ ] Bullet points and numbered lists are preserved but markers normalized
- [ ] URLs remain intact and clickable
- [ ] Horizontal rules (`---`) are removed
- [ ] Excessive blank lines are collapsed

### Story 2: Natural Voice Output
**As a** user using voice conversation mode
**I want** Blue's spoken responses to sound natural
**So that** TTS doesn't read formatting symbols aloud

**Acceptance Criteria:**
- [ ] All Markdown syntax is completely removed
- [ ] Code blocks are either summarized ("here's some code") or omitted
- [ ] URLs are simplified (e.g., "link to example dot com") or omitted
- [ ] Emojis are stripped (existing behavior preserved)
- [ ] Numbered lists read naturally ("First, ... Second, ...")
- [ ] No awkward pauses from excessive whitespace or blank lines
- [ ] Special characters that TTS mispronounces are cleaned

### Story 3: Configurable Behavior
**As an** administrator
**I want** to enable/disable the humanizer per output mode
**So that** I can control formatting behavior for different use cases

**Acceptance Criteria:**
- [ ] Humanizer can be toggled on/off globally
- [ ] IM mode and Voice mode can be independently enabled/disabled
- [ ] Configuration is available via config file

---

## 3. Functional Requirements

### 3.1 Text Transformation Rules

| ID | Rule | IM Mode | Voice Mode | Priority |
|----|------|---------|------------|----------|
| HR-001 | Strip bold markers (`**text**` → `text`) | Yes | Yes | P0 |
| HR-002 | Strip italic markers (`*text*`, `_text_` → `text`) | Yes | Yes | P0 |
| HR-003 | Strip inline code backticks (`` `code` `` → `code`) | Yes | Yes | P0 |
| HR-004 | Convert code fences to plain text (remove `` ``` `` lines) | Yes | Yes | P0 |
| HR-005 | Strip header markers (`## Title` → `Title`) | Yes | Yes | P0 |
| HR-006 | Normalize bullet points (`- item` → `• item` for IM, `item` for voice) | Light | Strip | P0 |
| HR-007 | Remove horizontal rules (`---`, `***`, `___`) | Yes | Yes | P1 |
| HR-008 | Collapse excessive blank lines (max 1 consecutive) | Yes | Yes | P1 |
| HR-009 | Strip strikethrough markers (`~~text~~` → `text`) | Yes | Yes | P1 |
| HR-010 | Convert links `[text](url)` → `text` (IM) / `text` (voice) | Keep URL | Strip URL | P1 |
| HR-011 | Strip emojis | No | Yes | P1 |
| HR-012 | Remove blockquote markers (`> text` → `text`) | Yes | Yes | P1 |
| HR-013 | Strip image references `![alt](url)` → `(image: alt)` / omit | Simplify | Omit | P2 |
| HR-014 | Clean HTML tags if present (`<br>`, `<b>`, etc.) | Yes | Yes | P2 |
| HR-015 | Normalize whitespace (tabs → spaces, trim trailing) | Yes | Yes | P1 |
| HR-016 | Convert numbered lists for speech (`1.` → `First,`) | No | Yes | P2 |

### 3.2 Processing Pipeline

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-001 | Provide `Humanize(text string, mode Mode) string` as the primary API | P0 |
| FR-002 | Support `ModeIM` and `ModeVoice` processing modes | P0 |
| FR-003 | Rules are applied in a deterministic, ordered pipeline | P0 |
| FR-004 | Processing must be stateless and safe for concurrent use | P0 |
| FR-005 | Processing must complete in < 1ms for typical message lengths (< 4KB) | P1 |
| FR-006 | Subsume existing `stripEmojis()` from voice service | P1 |

### 3.3 Integration Points

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-007 | Integrate into channel `Send()` — humanize `OutgoingMessage.Content` when `Format` is not explicitly set to "markdown" | P0 |
| FR-008 | Integrate into voice `Synthesize()` — replace `stripEmojis()` with full humanizer in Voice mode | P0 |
| FR-009 | Integrate into channel `SendStreaming()` — humanize accumulated text before final message update | P1 |
| FR-010 | Do NOT apply to web chat API responses (only IM channels and voice) | P0 |

### 3.4 Configuration

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-011 | Global enable/disable toggle | P0 |
| FR-012 | Per-mode enable/disable (IM mode, Voice mode) | P1 |
| FR-013 | Configurable via `config.yaml` under `humanizer:` section | P0 |

---

## 4. Technical Design

### 4.1 Architecture

```
LLM Response (Markdown text)
        |
        v
   ┌─────────────┐
   │  Humanizer   │
   │              │
   │  Mode: IM    │──→ Channel Send() ──→ Telegram / Discord / Slack / ...
   │  Mode: Voice │──→ TTS Synthesize() ──→ Audio output
   └─────────────┘
```

The Humanizer sits as a pure text transformation layer between the LLM output and the delivery mechanism. It does not modify the stored message — only the outbound copy.

### 4.2 Package Structure

```
server/internal/humanizer/
├── humanizer.go       // Core Humanize() function, Mode type, rule pipeline
├── rules.go           // Individual transformation rules (strip bold, strip code, etc.)
├── humanizer_test.go  // Unit tests with table-driven test cases
└── rules_test.go      // Tests for individual rules
```

### 4.3 Core API

```go
package humanizer

// Mode determines the level of text transformation.
type Mode int

const (
    ModeIM    Mode = iota // Light cleanup for IM channels
    ModeVoice             // Aggressive cleanup for TTS
)

// Config holds humanizer configuration.
type Config struct {
    Enabled      bool `mapstructure:"enabled"`
    IMEnabled    bool `mapstructure:"im_enabled"`
    VoiceEnabled bool `mapstructure:"voice_enabled"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
    return Config{
        Enabled:      true,
        IMEnabled:    true,
        VoiceEnabled: true,
    }
}

// Humanize transforms Markdown-formatted LLM output into natural text.
// It is safe for concurrent use.
func Humanize(text string, mode Mode) string {
    // Apply rules in order based on mode
}
```

### 4.4 Rule Implementation (examples)

```go
// stripBold removes ** and __ bold markers.
// "**hello**" → "hello", "__world__" → "world"
func stripBold(text string) string {
    text = regexp.MustCompile(`\*\*(.+?)\*\*`).ReplaceAllString(text, "$1")
    text = regexp.MustCompile(`__(.+?)__`).ReplaceAllString(text, "$1")
    return text
}

// stripCodeFences removes ``` code blocks, keeping content.
// For voice mode, replaces with a spoken indicator.
func stripCodeFences(text string, mode Mode) string {
    if mode == ModeVoice {
        // Replace code blocks with spoken description
        return regexp.MustCompile("(?s)```\\w*\\n(.+?)```").
            ReplaceAllString(text, "(code omitted)")
    }
    // IM mode: keep content, remove fences
    text = regexp.MustCompile("(?m)^```\\w*$").ReplaceAllString(text, "")
    return text
}

// stripHeaders removes # header markers.
// "## Title" → "Title"
func stripHeaders(text string) string {
    return regexp.MustCompile(`(?m)^#{1,6}\s+`).ReplaceAllString(text, "")
}
```

### 4.5 Integration Points

#### Channel Send (IM Mode)

In each channel's `Send()` method, or in a shared middleware before dispatch:

```go
// Before sending to channel
if humanizerConfig.Enabled && humanizerConfig.IMEnabled {
    msg.Content = humanizer.Humanize(msg.Content, humanizer.ModeIM)
}
```

**Recommended approach**: Add humanization in the channel manager's dispatch layer rather than in each individual channel implementation, to avoid duplicating logic across 20+ channel implementations.

#### Voice Synthesize (Voice Mode)

In `server/internal/voice/service.go`, replace:

```go
// Before (line 177):
cleanText := stripEmojis(req.Text)

// After:
cleanText := humanizer.Humanize(req.Text, humanizer.ModeVoice)
```

### 4.6 Configuration

```yaml
humanizer:
  enabled: true          # Global toggle
  im_enabled: true       # Apply to IM channel replies
  voice_enabled: true    # Apply to voice/TTS output
```

---

## 5. Implementation Phases

| Phase | Scope | Priority |
|-------|-------|----------|
| Phase 1 | Core `humanizer` package with all P0 rules + unit tests | P0 |
| Phase 2 | Integrate into voice `Synthesize()`, replacing `stripEmojis()` | P0 |
| Phase 3 | Integrate into channel `Send()` pipeline | P0 |
| Phase 4 | Add P1 rules (horizontal rules, blockquotes, links, whitespace) | P1 |
| Phase 5 | Configuration support via `config.yaml` | P1 |
| Phase 6 | Add P2 rules (image refs, HTML tags, numbered list speech) | P2 |

---

## 6. Test Strategy

### 6.1 Unit Tests

Table-driven tests for each rule and each mode:

```go
func TestHumanize(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        mode     Mode
        expected string
    }{
        {"bold IM", "**hello** world", ModeIM, "hello world"},
        {"bold voice", "**hello** world", ModeVoice, "hello world"},
        {"code fence IM", "```go\nfmt.Println()\n```", ModeIM, "fmt.Println()"},
        {"code fence voice", "```go\nfmt.Println()\n```", ModeVoice, "(code omitted)"},
        {"header", "## Section Title", ModeIM, "Section Title"},
        {"mixed", "# Title\n\n**Bold** and `code`\n\n---\n\nText", ModeIM,
            "Title\n\nBold and code\n\nText"},
    }
    // ...
}
```

### 6.2 Integration Tests

- Send a Markdown-heavy message through a mock channel and verify the output is clean
- Synthesize a Markdown-heavy text through voice service and verify TTS input is clean

---

## 7. Success Metrics

| Metric | Target | Measurement |
|--------|--------|-------------|
| Markdown artifacts in IM replies | 0% of replies contain raw `**`, `` ``` ``, `##` | Spot-check channel logs |
| TTS pronunciation issues from formatting | Eliminated | Manual voice testing |
| Processing latency overhead | < 1ms per message | Benchmark tests |
| Test coverage | > 90% for humanizer package | `go test -cover` |

---

## 8. Open Questions

- [ ] Should code blocks in IM mode be wrapped in a simplified format (e.g., indented text) or stripped entirely?
- [ ] Should the humanizer handle table Markdown (`| col | col |`)? Tables are rare in chat responses but possible.
- [ ] For voice mode, should long code blocks trigger a spoken summary ("I've written some code for you") or be silently omitted?
