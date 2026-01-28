package claudecode

import (
	"strings"
	"testing"
)

func TestNewOutputParser(t *testing.T) {
	config := DefaultClaudeCodeBackend()
	parser := NewOutputParser(&config)

	if parser == nil {
		t.Fatal("expected non-nil parser")
	}
}

func TestParseJSON(t *testing.T) {
	config := DefaultClaudeCodeBackend()
	parser := NewOutputParser(&config)

	tests := []struct {
		name      string
		input     string
		wantText  string
		wantSess  string
		wantErr   bool
	}{
		{
			name:     "empty input",
			input:    "",
			wantText: "",
			wantErr:  false,
		},
		{
			name:     "simple text field",
			input:    `{"text": "Hello, world!"}`,
			wantText: "Hello, world!",
			wantErr:  false,
		},
		{
			name:     "content field",
			input:    `{"content": "Hello from content"}`,
			wantText: "Hello from content",
			wantErr:  false,
		},
		{
			name:     "result field",
			input:    `{"result": "Hello from result"}`,
			wantText: "Hello from result",
			wantErr:  false,
		},
		{
			name:     "with session_id",
			input:    `{"text": "Hello", "session_id": "sess-123"}`,
			wantText: "Hello",
			wantSess: "sess-123",
			wantErr:  false,
		},
		{
			name:     "with sessionId",
			input:    `{"text": "Hello", "sessionId": "sess-456"}`,
			wantText: "Hello",
			wantSess: "sess-456",
			wantErr:  false,
		},
		{
			name:     "nested message.content",
			input:    `{"message": {"content": "Nested content"}}`,
			wantText: "Nested content",
			wantErr:  false,
		},
		{
			name:     "content array",
			input:    `{"content": [{"type": "text", "text": "Array text"}]}`,
			wantText: "Array text",
			wantErr:  false,
		},
		{
			name:     "delta.text",
			input:    `{"delta": {"text": "Delta text"}}`,
			wantText: "Delta text",
			wantErr:  false,
		},
		{
			name:    "invalid json",
			input:   `{invalid json}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := parser.ParseJSON(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			if output.Text != tt.wantText {
				t.Errorf("ParseJSON() text = '%s', want '%s'", output.Text, tt.wantText)
			}

			if output.SessionId != tt.wantSess {
				t.Errorf("ParseJSON() sessionId = '%s', want '%s'", output.SessionId, tt.wantSess)
			}
		})
	}
}

func TestParseJSONL(t *testing.T) {
	config := DefaultClaudeCodeBackend()
	parser := NewOutputParser(&config)

	tests := []struct {
		name      string
		input     string
		wantText  string
		wantSess  string
		wantErr   bool
	}{
		{
			name:     "empty input",
			input:    "",
			wantText: "",
			wantErr:  false,
		},
		{
			name:     "single line",
			input:    `{"text": "Hello"}`,
			wantText: "Hello",
			wantErr:  false,
		},
		{
			name: "multiple lines",
			input: `{"text": "Hello"}
{"text": " world"}
{"text": "!"}`,
			wantText: "Hello world!",
			wantErr:  false,
		},
		{
			name: "with session in first line",
			input: `{"text": "Hello", "session_id": "sess-123"}
{"text": " world"}`,
			wantText: "Hello world",
			wantSess: "sess-123",
			wantErr:  false,
		},
		{
			name: "skip invalid lines",
			input: `{"text": "Hello"}
invalid line
{"text": " world"}`,
			wantText: "Hello world",
			wantErr:  false,
		},
		{
			name: "with usage in last line",
			input: `{"text": "Hello"}
{"text": " world", "usage": {"input_tokens": 10, "output_tokens": 5, "total_tokens": 15}}`,
			wantText: "Hello world",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := parser.ParseJSONL(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseJSONL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			if output.Text != tt.wantText {
				t.Errorf("ParseJSONL() text = '%s', want '%s'", output.Text, tt.wantText)
			}

			if output.SessionId != tt.wantSess {
				t.Errorf("ParseJSONL() sessionId = '%s', want '%s'", output.SessionId, tt.wantSess)
			}
		})
	}
}

func TestParseText(t *testing.T) {
	config := DefaultClaudeCodeBackend()
	parser := NewOutputParser(&config)

	tests := []struct {
		name     string
		input    string
		wantText string
	}{
		{
			name:     "empty input",
			input:    "",
			wantText: "",
		},
		{
			name:     "simple text",
			input:    "Hello, world!",
			wantText: "Hello, world!",
		},
		{
			name:     "text with whitespace",
			input:    "  Hello, world!  \n",
			wantText: "Hello, world!",
		},
		{
			name:     "multiline text",
			input:    "Line 1\nLine 2\nLine 3",
			wantText: "Line 1\nLine 2\nLine 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := parser.ParseText(tt.input)

			if err != nil {
				t.Errorf("ParseText() error = %v", err)
				return
			}

			if output.Text != tt.wantText {
				t.Errorf("ParseText() text = '%s', want '%s'", output.Text, tt.wantText)
			}
		})
	}
}

func TestParse(t *testing.T) {
	config := DefaultClaudeCodeBackend()
	parser := NewOutputParser(&config)

	// Test JSON format
	output, err := parser.Parse(`{"text": "Hello"}`, OutputFormatJSON)
	if err != nil {
		t.Errorf("Parse(JSON) error = %v", err)
	}
	if output.Text != "Hello" {
		t.Errorf("Parse(JSON) text = '%s', want 'Hello'", output.Text)
	}

	// Test JSONL format
	output, err = parser.Parse(`{"text": "Hello"}
{"text": " world"}`, OutputFormatJSONL)
	if err != nil {
		t.Errorf("Parse(JSONL) error = %v", err)
	}
	if output.Text != "Hello world" {
		t.Errorf("Parse(JSONL) text = '%s', want 'Hello world'", output.Text)
	}

	// Test text format
	output, err = parser.Parse("Hello, world!", OutputFormatText)
	if err != nil {
		t.Errorf("Parse(Text) error = %v", err)
	}
	if output.Text != "Hello, world!" {
		t.Errorf("Parse(Text) text = '%s', want 'Hello, world!'", output.Text)
	}
}

func TestExtractUsage(t *testing.T) {
	config := DefaultClaudeCodeBackend()
	parser := NewOutputParser(&config)

	tests := []struct {
		name        string
		input       string
		wantInput   int
		wantOutput  int
		wantTotal   int
	}{
		{
			name:       "no usage",
			input:      `{"text": "Hello"}`,
			wantInput:  0,
			wantOutput: 0,
			wantTotal:  0,
		},
		{
			name:       "with usage object",
			input:      `{"text": "Hello", "usage": {"input_tokens": 100, "output_tokens": 50, "total_tokens": 150}}`,
			wantInput:  100,
			wantOutput: 50,
			wantTotal:  150,
		},
		{
			name:       "with prompt_tokens and completion_tokens",
			input:      `{"text": "Hello", "usage": {"prompt_tokens": 100, "completion_tokens": 50}}`,
			wantInput:  100,
			wantOutput: 50,
			wantTotal:  150,
		},
		{
			name:       "top-level tokens",
			input:      `{"text": "Hello", "input_tokens": 100, "output_tokens": 50}`,
			wantInput:  100,
			wantOutput: 50,
			wantTotal:  150,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := parser.ParseJSON(tt.input)
			if err != nil {
				t.Errorf("ParseJSON() error = %v", err)
				return
			}

			if output.Usage.InputTokens != tt.wantInput {
				t.Errorf("Usage.InputTokens = %d, want %d", output.Usage.InputTokens, tt.wantInput)
			}
			if output.Usage.OutputTokens != tt.wantOutput {
				t.Errorf("Usage.OutputTokens = %d, want %d", output.Usage.OutputTokens, tt.wantOutput)
			}
			if output.Usage.TotalTokens != tt.wantTotal {
				t.Errorf("Usage.TotalTokens = %d, want %d", output.Usage.TotalTokens, tt.wantTotal)
			}
		})
	}
}

func TestParseStream(t *testing.T) {
	// Test with text format (default for CC CLI)
	config := DefaultClaudeCodeBackend()
	parser := NewOutputParser(&config)

	input := "Hello world"

	reader := strings.NewReader(input)
	ch := parser.ParseStream(reader, OutputFormatText)

	var chunks []CliStreamChunk
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}

	// Collect all text from chunks
	var fullText strings.Builder
	var hasDone bool
	for _, chunk := range chunks {
		if chunk.Text != "" {
			fullText.WriteString(chunk.Text)
		}
		if chunk.Done {
			hasDone = true
		}
	}

	// Verify full text matches input
	if fullText.String() != input {
		t.Errorf("expected text '%s', got '%s'", input, fullText.String())
	}

	// Should have a done chunk
	if !hasDone {
		t.Error("expected at least one done chunk")
	}
}

func TestParseStreamText(t *testing.T) {
	config := DefaultClaudeCodeBackend()
	config.Output = "text" // Set to text mode
	parser := NewOutputParser(&config)

	input := "Hello, world!"

	reader := strings.NewReader(input)
	ch := parser.ParseStream(reader, OutputFormatText)

	var chunks []CliStreamChunk
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}

	// Should have one chunk per rune plus a done chunk
	// "Hello, world!" = 13 runes + 1 done chunk = 14 chunks
	expectedChars := len([]rune(input))

	// Count non-done chunks
	textChunks := 0
	var fullText strings.Builder
	for _, chunk := range chunks {
		if chunk.Text != "" {
			textChunks++
			fullText.WriteString(chunk.Text)
		}
	}

	if textChunks != expectedChars {
		t.Errorf("expected %d text chunks (one per rune), got %d", expectedChars, textChunks)
	}

	// Verify full text is reconstructed correctly
	if fullText.String() != input {
		t.Errorf("reconstructed text = '%s', want '%s'", fullText.String(), input)
	}

	// Last chunk should be done
	lastChunk := chunks[len(chunks)-1]
	if !lastChunk.Done {
		t.Error("expected last chunk to be done")
	}
}

func TestParseStreamTextUTF8(t *testing.T) {
	config := DefaultClaudeCodeBackend()
	config.Output = "text"
	parser := NewOutputParser(&config)

	// Test with Chinese characters (multi-byte UTF-8)
	input := "你好世界"

	reader := strings.NewReader(input)
	ch := parser.ParseStream(reader, OutputFormatText)

	var chunks []CliStreamChunk
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}

	// Should have one chunk per rune (4 Chinese characters) plus a done chunk
	expectedChars := len([]rune(input)) // 4 runes, not 12 bytes

	// Count non-done chunks
	textChunks := 0
	var fullText strings.Builder
	for _, chunk := range chunks {
		if chunk.Text != "" {
			textChunks++
			fullText.WriteString(chunk.Text)
		}
	}

	if textChunks != expectedChars {
		t.Errorf("expected %d text chunks (one per rune), got %d", expectedChars, textChunks)
	}

	// Verify full text is reconstructed correctly
	if fullText.String() != input {
		t.Errorf("reconstructed text = '%s', want '%s'", fullText.String(), input)
	}

	// Verify each chunk is a complete Chinese character
	for i, chunk := range chunks {
		if chunk.Done {
			continue
		}
		runeCount := len([]rune(chunk.Text))
		if runeCount != 1 {
			t.Errorf("chunk %d has %d runes, expected 1", i, runeCount)
		}
	}
}
