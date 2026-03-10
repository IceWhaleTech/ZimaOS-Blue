package claudecode

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// OutputParser parses CLI output based on the configured format.
type OutputParser struct {
	config *CliBackendConfig
}

// NewOutputParser creates a new OutputParser with the given configuration.
func NewOutputParser(config *CliBackendConfig) *OutputParser {
	return &OutputParser{config: config}
}

// Parse parses the CLI output based on the format.
func (p *OutputParser) Parse(output string, format OutputFormat) (*CliOutput, error) {
	switch format {
	case OutputFormatJSON:
		return p.ParseJSON(output)
	case OutputFormatJSONL:
		return p.ParseJSONL(output)
	case OutputFormatText:
		return p.ParseText(output)
	default:
		return p.ParseJSON(output)
	}
}

// ParseJSON parses a single JSON object output.
func (p *OutputParser) ParseJSON(output string) (*CliOutput, error) {
	output = strings.TrimSpace(output)
	if output == "" {
		return &CliOutput{}, nil
	}

	// Try to parse as a generic JSON object
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(output), &raw); err != nil {
		return nil, ErrParseOutput{Format: "json", Content: output, Cause: err}
	}

	result := &CliOutput{Raw: output}

	// Extract text from various possible fields
	result.Text = p.extractText(raw)

	// Extract session ID from configured fields
	result.SessionId = p.extractSessionId(raw)

	// Extract usage statistics
	result.Usage = p.extractUsage(raw)

	return result, nil
}

// ParseJSONL parses newline-delimited JSON output (streaming format).
func (p *OutputParser) ParseJSONL(output string) (*CliOutput, error) {
	result := &CliOutput{Raw: output}
	var textParts []string

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			// Skip non-JSON lines
			continue
		}

		// Extract and accumulate text
		if text := p.extractText(raw); text != "" {
			textParts = append(textParts, text)
		}

		// Extract session ID (take the first non-empty one)
		if result.SessionId == "" {
			result.SessionId = p.extractSessionId(raw)
		}

		// Extract usage (take the last one, as it's typically in the final message)
		if usage := p.extractUsage(raw); usage.TotalTokens > 0 {
			result.Usage = usage
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, ErrParseOutput{Format: "jsonl", Content: output, Cause: err}
	}

	result.Text = strings.Join(textParts, "")
	return result, nil
}

// ParseText parses plain text output.
func (p *OutputParser) ParseText(output string) (*CliOutput, error) {
	return &CliOutput{
		Text: strings.TrimSpace(output),
		Raw:  output,
	}, nil
}

// ParseStream parses streaming output from a reader.
func (p *OutputParser) ParseStream(reader io.Reader, format OutputFormat) <-chan CliStreamChunk {
	ch := make(chan CliStreamChunk, 10)

	go func() {
		defer close(ch)

		switch format {
		case OutputFormatJSONL:
			p.parseStreamJSONL(reader, ch)
		case OutputFormatJSON:
			p.parseStreamJSON(reader, ch)
		default:
			p.parseStreamText(reader, ch)
		}
	}()

	return ch
}

// parseStreamJSONL parses streaming JSONL output.
// Uses buffered reading with immediate line processing for real-time streaming.
// For CC CLI stream-json format, sends text chunks as they are read.
func (p *OutputParser) parseStreamJSONL(reader io.Reader, ch chan<- CliStreamChunk) {
	// Use a buffered reader for efficient byte-level reading
	bufReader := bufio.NewReaderSize(reader, 64*1024)
	var lineBuffer strings.Builder
	var sessionId string

	// Track if we're inside a text field for partial sending
	inTextField := false
	textFieldBuffer := strings.Builder{}

	for {
		// Read one byte at a time to detect newlines immediately
		b, err := bufReader.ReadByte()
		if err != nil {
			if err == io.EOF {
				// Process any remaining data in buffer
				if lineBuffer.Len() > 0 {
					p.processJSONLLine(lineBuffer.String(), ch)
				}
				return
			}
			ch <- CliStreamChunk{Error: err}
			return
		}

		if b == '\n' {
			// Process the complete line immediately
			line := lineBuffer.String()
			lineBuffer.Reset()
			inTextField = false
			textFieldBuffer.Reset()

			if line != "" {
				p.processJSONLLine(line, ch)
			}
		} else {
			lineBuffer.WriteByte(b)

			// Try to detect and send text content incrementally
			// Look for patterns like "text":" or "content":"
			lineStr := lineBuffer.String()
			if !inTextField {
				// Check if we just entered a text field
				if strings.HasSuffix(lineStr, `"text":"`) || strings.HasSuffix(lineStr, `"content":"`) {
					inTextField = true
					textFieldBuffer.Reset()
				}
			} else {
				// We're in a text field, accumulate and send chunks
				if b == '"' && !strings.HasSuffix(lineStr, `\"`) {
					// End of text field
					if textFieldBuffer.Len() > 0 {
						// Send remaining text
						text := textFieldBuffer.String()
						// Unescape JSON string
						text = strings.ReplaceAll(text, `\"`, `"`)
						text = strings.ReplaceAll(text, `\\`, `\`)
						text = strings.ReplaceAll(text, `\n`, "\n")
						text = strings.ReplaceAll(text, `\t`, "\t")
						if text != "" {
							ch <- CliStreamChunk{
								Text:      text,
								SessionId: sessionId,
							}
						}
					}
					inTextField = false
					textFieldBuffer.Reset()
				} else {
					textFieldBuffer.WriteByte(b)
					// Send chunks immediately when we have enough characters (3-8 chars)
					// No artificial delay - let the natural network/process latency provide pacing
					chunkSize := 3 + int(timeutil.NowNano()%6)
					if textFieldBuffer.Len() >= chunkSize {
						text := textFieldBuffer.String()
						textFieldBuffer.Reset()
						// Unescape JSON string
						text = strings.ReplaceAll(text, `\"`, `"`)
						text = strings.ReplaceAll(text, `\\`, `\`)
						text = strings.ReplaceAll(text, `\n`, "\n")
						text = strings.ReplaceAll(text, `\t`, "\t")
						if text != "" {
							ch <- CliStreamChunk{
								Text:      text,
								SessionId: sessionId,
							}
						}
					}
				}
			}

			// Extract session ID if present
			if strings.Contains(lineStr, `"session_id":"`) || strings.Contains(lineStr, `"sessionId":"`) {
				// Try to extract session ID
				for _, pattern := range []string{`"session_id":"`, `"sessionId":"`} {
					if idx := strings.Index(lineStr, pattern); idx >= 0 {
						start := idx + len(pattern)
						end := strings.Index(lineStr[start:], `"`)
						if end > 0 {
							sessionId = lineStr[start : start+end]
						}
					}
				}
			}
		}
	}
}

// processJSONLLine processes a single JSONL line and sends final metadata.
// Text content is already sent incrementally during byte reading.
func (p *OutputParser) processJSONLLine(line string, ch chan<- CliStreamChunk) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}

	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		// Skip non-JSON lines
		return
	}

	sessionId := p.extractSessionId(raw)

	// Check for usage (indicates final message)
	var usage *CliUsage
	if u := p.extractUsage(raw); u.TotalTokens > 0 {
		usage = &u
	}

	// Check for done/stop indicators
	done := false
	if d, ok := raw["done"].(bool); ok && d {
		done = true
	}
	if stopReason, ok := raw["stop_reason"].(string); ok && stopReason != "" && !isNonTerminalStopReason(stopReason) {
		done = true
	}

	// Only send chunk if there's metadata to report (usage/done)
	// Text was already sent incrementally
	if done || usage != nil {
		ch <- CliStreamChunk{
			SessionId: sessionId,
			Usage:     usage,
			Done:      done,
		}
	}
}

func isNonTerminalStopReason(stopReason string) bool {
	switch strings.ToLower(strings.TrimSpace(stopReason)) {
	case "tool_use", "tooluse", "tool_call", "toolcall", "function_call", "functioncall", "pause_turn", "pause":
		return true
	default:
		return false
	}
}

// parseStreamJSON parses streaming JSON output (single object at end).
func (p *OutputParser) parseStreamJSON(reader io.Reader, ch chan<- CliStreamChunk) {
	data, err := io.ReadAll(reader)
	if err != nil {
		ch <- CliStreamChunk{Error: err}
		return
	}

	output, err := p.ParseJSON(string(data))
	if err != nil {
		ch <- CliStreamChunk{Error: err}
		return
	}

	ch <- CliStreamChunk{
		Text:      output.Text,
		SessionId: output.SessionId,
		Usage:     &output.Usage,
		Done:      true,
	}
}

// parseStreamText parses streaming text output.
// Uses rune-level reading for real-time character-by-character streaming.
// This correctly handles multi-byte UTF-8 characters (e.g., Chinese, emoji).
func (p *OutputParser) parseStreamText(reader io.Reader, ch chan<- CliStreamChunk) {
	bufReader := bufio.NewReaderSize(reader, 4096)

	for {
		// Read one rune at a time for character-level streaming
		// ReadRune correctly handles multi-byte UTF-8 characters
		r, _, err := bufReader.ReadRune()
		if err != nil {
			if err == io.EOF {
				ch <- CliStreamChunk{Done: true}
				return
			}
			ch <- CliStreamChunk{Error: err}
			return
		}

		// Send each character immediately for real-time streaming effect
		ch <- CliStreamChunk{
			Text: string(r),
		}
	}
}

// extractText extracts text content from a JSON object.
// It tries multiple common field names used by different CLI tools.
func (p *OutputParser) extractText(raw map[string]interface{}) string {
	// Try direct text fields
	textFields := []string{"text", "content", "result", "message", "response", "output"}
	for _, field := range textFields {
		if text, ok := raw[field].(string); ok && text != "" {
			return text
		}
	}

	// Try nested message.content (Claude API format)
	if message, ok := raw["message"].(map[string]interface{}); ok {
		if content, ok := message["content"].(string); ok && content != "" {
			return content
		}
		// Try content array (Claude API format)
		if contentArr, ok := message["content"].([]interface{}); ok {
			return p.extractTextFromContentArray(contentArr)
		}
	}

	// Try content array at top level
	if contentArr, ok := raw["content"].([]interface{}); ok {
		return p.extractTextFromContentArray(contentArr)
	}

	// Try delta.text (streaming format)
	if delta, ok := raw["delta"].(map[string]interface{}); ok {
		if text, ok := delta["text"].(string); ok {
			return text
		}
	}

	return ""
}

// extractTextFromContentArray extracts text from a content array.
func (p *OutputParser) extractTextFromContentArray(contentArr []interface{}) string {
	var texts []string
	for _, item := range contentArr {
		if block, ok := item.(map[string]interface{}); ok {
			if blockType, ok := block["type"].(string); ok && blockType == "text" {
				if text, ok := block["text"].(string); ok {
					texts = append(texts, text)
				}
			}
		}
	}
	return strings.Join(texts, "")
}

// extractSessionId extracts session ID from a JSON object.
func (p *OutputParser) extractSessionId(raw map[string]interface{}) string {
	// Try configured fields first
	for _, field := range p.config.SessionIdFields {
		if id, ok := raw[field].(string); ok && id != "" {
			return id
		}
	}

	// Try common field names
	commonFields := []string{"session_id", "sessionId", "session", "conversation_id", "conversationId"}
	for _, field := range commonFields {
		if id, ok := raw[field].(string); ok && id != "" {
			return id
		}
	}

	return ""
}

// extractUsage extracts usage statistics from a JSON object.
func (p *OutputParser) extractUsage(raw map[string]interface{}) CliUsage {
	usage := CliUsage{}

	// Try direct usage object
	if usageObj, ok := raw["usage"].(map[string]interface{}); ok {
		usage = p.parseUsageObject(usageObj)
	}

	// Try top-level fields
	if usage.TotalTokens == 0 {
		if inputTokens, ok := raw["input_tokens"].(float64); ok {
			usage.InputTokens = int(inputTokens)
		}
		if outputTokens, ok := raw["output_tokens"].(float64); ok {
			usage.OutputTokens = int(outputTokens)
		}
		if totalTokens, ok := raw["total_tokens"].(float64); ok {
			usage.TotalTokens = int(totalTokens)
		}
	}

	// Calculate total if not provided
	if usage.TotalTokens == 0 && (usage.InputTokens > 0 || usage.OutputTokens > 0) {
		usage.TotalTokens = usage.InputTokens + usage.OutputTokens
	}

	return usage
}

// parseUsageObject parses a usage object.
func (p *OutputParser) parseUsageObject(usageObj map[string]interface{}) CliUsage {
	usage := CliUsage{}

	// Input tokens
	if v, ok := usageObj["input_tokens"].(float64); ok {
		usage.InputTokens = int(v)
	} else if v, ok := usageObj["prompt_tokens"].(float64); ok {
		usage.InputTokens = int(v)
	}

	// Output tokens
	if v, ok := usageObj["output_tokens"].(float64); ok {
		usage.OutputTokens = int(v)
	} else if v, ok := usageObj["completion_tokens"].(float64); ok {
		usage.OutputTokens = int(v)
	}

	// Cache tokens
	if v, ok := usageObj["cache_read_input_tokens"].(float64); ok {
		usage.CacheReadTokens = int(v)
	} else if v, ok := usageObj["cache_read_tokens"].(float64); ok {
		usage.CacheReadTokens = int(v)
	}

	if v, ok := usageObj["cache_creation_input_tokens"].(float64); ok {
		usage.CacheWriteTokens = int(v)
	} else if v, ok := usageObj["cache_write_tokens"].(float64); ok {
		usage.CacheWriteTokens = int(v)
	}

	// Total tokens
	if v, ok := usageObj["total_tokens"].(float64); ok {
		usage.TotalTokens = int(v)
	} else {
		usage.TotalTokens = usage.InputTokens + usage.OutputTokens
	}

	return usage
}
