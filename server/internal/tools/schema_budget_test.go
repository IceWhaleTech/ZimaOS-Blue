package tools

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"
)

type toolBudgetStat struct {
	Name            string
	RawBytes        int
	CompressedBytes int
}

func TestSelectedToolDefinitionsStayCompact(t *testing.T) {
	selected := selectedToolDefsForBudgetTest(t)
	router := DefaultToolRouter()
	router.DynamicExposure = false

	compressed := router.Route("compact tool definitions", "auto", selected)
	if len(compressed) != len(selected) {
		t.Fatalf("compressed defs count = %d, want %d", len(compressed), len(selected))
	}

	compressedByName := make(map[string]ToolDefinition, len(compressed))
	for _, def := range compressed {
		compressedByName[def.Name] = def
	}

	stats := make([]toolBudgetStat, 0, len(selected))
	rawTotal := 0
	compressedTotal := 0
	for _, def := range selected {
		rawBytes := len(mustMarshalToolDefBudget(t, def))
		compressedDef, ok := compressedByName[def.Name]
		if !ok {
			t.Fatalf("missing compressed definition for %q", def.Name)
		}
		compressedBytes := len(mustMarshalToolDefBudget(t, compressedDef))
		stats = append(stats, toolBudgetStat{
			Name:            def.Name,
			RawBytes:        rawBytes,
			CompressedBytes: compressedBytes,
		})
		rawTotal += rawBytes
		compressedTotal += compressedBytes
	}

	sort.Slice(stats, func(i, j int) bool {
		if stats[i].CompressedBytes == stats[j].CompressedBytes {
			return stats[i].Name < stats[j].Name
		}
		return stats[i].CompressedBytes > stats[j].CompressedBytes
	})

	var report strings.Builder
	for _, stat := range stats {
		fmt.Fprintf(&report, "\n- %s: %d -> %d bytes", stat.Name, stat.RawBytes, stat.CompressedBytes)
	}
	t.Logf("selected tool definition bytes: raw=%d compressed=%d%s", rawTotal, compressedTotal, report.String())

	if compressedTotal >= rawTotal {
		t.Fatalf("compressed total should shrink: raw=%d compressed=%d", rawTotal, compressedTotal)
	}
	if compressedTotal > 11000 {
		t.Fatalf("compressed selected-tool budget too large: %d bytes", compressedTotal)
	}
}

func selectedToolDefsForBudgetTest(t *testing.T) []ToolDefinition {
	t.Helper()

	registry := NewRegistry()
	registry.Register(NewAskTool(nil))
	registry.Register(NewBrowserTool())
	registry.Register(NewCalendarTool(nil))
	registry.Register(NewConvertTool(nil, nil, nil, nil))
	registry.Register(NewDeepResearchTool(nil))
	registry.Register(NewEditTool(nil, 0))
	registry.Register(NewEmailTool(nil))
	registry.Register(NewFindTool(nil))
	registry.Register(NewGrepTool(nil, 0))
	registry.Register(NewImageTool(nil, nil, nil))
	registry.Register(NewLsTool(nil))
	registry.Register(NewPDFTool(nil))
	registry.Register(NewPublicBashTool(registry))
	registry.Register(NewPublicReadTool(registry))
	registry.Register(NewPublicWriteTool(registry))

	searchTool := NewWebSearchTool(WebSearchConfig{})
	fetchTool := NewWebFetchTool(WebFetchConfig{})
	readTool := NewWebReadTool(WebFetchConfig{})
	extractTool := NewWebExtractTool(WebFetchConfig{})
	crawlTool := NewWebCrawlTool(WebFetchConfig{})
	registry.Register(NewWebQueryTool(searchTool, fetchTool, readTool, extractTool, crawlTool))

	wantNames := []string{
		"ask",
		"bash",
		"browser",
		"calendar",
		"convert",
		"deep_research",
		"edit",
		"email",
		"find",
		"grep",
		"image",
		"ls",
		"pdf",
		"read",
		"web_query",
		"write",
	}
	wantSet := make(map[string]struct{}, len(wantNames))
	for _, name := range wantNames {
		wantSet[name] = struct{}{}
	}

	selected := make([]ToolDefinition, 0, len(wantNames))
	for _, def := range registry.Definitions() {
		if _, ok := wantSet[def.Name]; ok {
			selected = append(selected, def)
		}
	}
	if len(selected) != len(wantNames) {
		have := make([]string, 0, len(selected))
		for _, def := range selected {
			have = append(have, def.Name)
		}
		sort.Strings(have)
		t.Fatalf("selected defs count = %d, want %d (have=%v)", len(selected), len(wantNames), have)
	}
	return selected
}

func mustMarshalToolDefBudget(t *testing.T, def ToolDefinition) []byte {
	t.Helper()
	data, err := json.Marshal(def)
	if err != nil {
		t.Fatalf("marshal %q: %v", def.Name, err)
	}
	return data
}
