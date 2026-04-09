package tools

import (
	"bufio"
	"fmt"
	"strings"
)

const (
	maxFileWriteChunkBytes = 32 << 10 // 32 KiB per write call; stricter than the requested 50 KB ceiling.
	maxFileWriteChunkLines = 200
)

func validateWriteContentChunk(content, limitLabel, guidance string) error {
	if len(content) > maxFileWriteChunkBytes {
		return fmt.Errorf(
			"content chunk too large: %d bytes (max: %d bytes per %s); %s",
			len(content),
			maxFileWriteChunkBytes,
			limitLabel,
			guidance,
		)
	}

	if lineCount := countWriteContentLines(content); lineCount > maxFileWriteChunkLines {
		return fmt.Errorf(
			"content chunk too large: %d lines (max: %d lines per %s); %s",
			lineCount,
			maxFileWriteChunkLines,
			limitLabel,
			guidance,
		)
	}
	if hasSuspiciousTerminalTruncationMarker(content) {
		return fmt.Errorf(
			"content appears truncated: terminal [truncated] marker is not allowed in %s payloads; resend the full content without truncation markers",
			limitLabel,
		)
	}

	return nil
}

func countWriteContentLines(content string) int {
	if content == "" {
		return 0
	}

	normalized := strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(content)
	scanner := bufio.NewScanner(strings.NewReader(normalized))
	lines := 0
	for scanner.Scan() {
		lines++
	}
	return lines
}

func hasSuspiciousTerminalTruncationMarker(content string) bool {
	trimmed := strings.TrimSpace(strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(content))
	return trimmed == "[truncated]" || strings.HasSuffix(trimmed, "\n[truncated]")
}
