package memory

import (
	"fmt"
	"strings"
)

func metadataFromTags(tags []string) map[string]string {
	if len(tags) == 0 {
		return map[string]string{}
	}
	metadata := make(map[string]string, len(tags))
	for i, tag := range tags {
		if trimmed := strings.TrimSpace(tag); trimmed != "" {
			metadata[fmt.Sprintf("tag_%d", i)] = trimmed
		}
	}
	return metadata
}

func sourceIDFromMetadata(metadata map[string]string) string {
	if len(metadata) == 0 {
		return ""
	}
	return strings.TrimSpace(metadata["source_id"])
}

func applySourceIDToChunk(chunk *MemoryChunk) {
	if chunk == nil {
		return
	}
	if sourceID := sourceIDFromMetadata(chunk.Metadata); sourceID != "" {
		chunk.ID = sourceID
	}
}
