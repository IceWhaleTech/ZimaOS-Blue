package claudecode

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
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
func (p *OutputParser) parseStreamJSONL(reader io.Reader, ch chan<- CliStreamChunk) {
	scanner := bufio.NewScanner(reader)
	// Increase buffer size for large JSON lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

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

		chunk := CliStreamChunk{
			Text:      p.extractText(raw),
			SessionId: p.extractSessionId(raw),
		}

		// Check for usage (indicates final message)
		if usage := p.extractUsage(raw); usage.TotalTokens > 0 {
			chunk.Usage = &usage
		}

		// Check for done/stop indicators
		if done, ok := raw["done"].(bool); ok && done {
			chunk.Done = true
		}
		if stopReason, ok := raw["stop_reason"].(string); ok && stopReason != "" {
			chunk.Done = true
		}

		ch <- chunk
	}

	if err := scanner.Err(); err != nil {
		ch <- CliStreamChunk{Error: err}
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
func (p *OutputParser) parseStreamText(reader io.Reader, ch chan<- CliStreamChunk) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		ch <- CliStreamChunk{
			Text: scanner.Text() + "\n",
		}
	}

	if err := scanner.Err(); err != nil {
		ch <- CliStreamChunk{Error: err}
		return
	}

	ch <- CliStreamChunk{Done: true}
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
