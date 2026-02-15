package pruner

import (
	"strings"
	"testing"
)

func TestIsCodeContent(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		minLines int
		expected bool
	}{
		{
			name:     "short content below min lines",
			content:  "x := 1\ny := 2",
			minLines: 10,
			expected: false,
		},
		{
			name:     "code fences trigger detection",
			content:  "```go\n" + strings.Repeat("fmt.Println(\"hello\")\n", 15) + "```\n",
			minLines: 10,
			expected: true,
		},
		{
			name: "go source code",
			content: strings.Repeat("package main\nimport \"fmt\"\nfunc main() {\n"+
				"\tfmt.Println(\"hello\")\n}\n", 5),
			minLines: 10,
			expected: true,
		},
		{
			name:     "natural language text",
			content:  strings.Repeat("This is a paragraph of natural language text that describes something.\n", 20),
			minLines: 10,
			expected: false,
		},
		{
			name: "python source code",
			content: strings.Repeat("import os\ndef main():\n    print('hello')\n"+
				"    return 0\nclass Foo:\n    pass\n", 5),
			minLines: 10,
			expected: true,
		},
		{
			name:     "empty content",
			content:  "",
			minLines: 1,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsCodeContent(tt.content, tt.minLines)
			if got != tt.expected {
				t.Errorf("IsCodeContent() = %v, want %v", got, tt.expected)
			}
		})
	}
}
