package server

import (
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

func TestLimitToolCallsForRound(t *testing.T) {
	tests := []struct {
		name      string
		calls     []llm.ToolCall
		wantCount int
		wantFirst llm.ToolCall
		wantCut   bool
	}{
		{
			name:      "empty",
			calls:     nil,
			wantCount: 0,
			wantCut:   false,
		},
		{
			name:      "single",
			calls:     []llm.ToolCall{{ID: "call_1", Name: "web_search", Arguments: `{"query":"openclaw"}`}},
			wantCount: 1,
			wantFirst: llm.ToolCall{ID: "call_1", Name: "web_search", Arguments: `{"query":"openclaw"}`},
			wantCut:   false,
		},
		{
			name: "multiple keeps first only",
			calls: []llm.ToolCall{
				{ID: "call_1", Name: "web_search", Arguments: `{"query":"openclaw latest news"}`},
				{ID: "call_2", Name: "web_search", Arguments: `{"query":"openclaw github"}`},
			},
			wantCount: 1,
			wantFirst: llm.ToolCall{ID: "call_1", Name: "web_search", Arguments: `{"query":"openclaw latest news"}`},
			wantCut:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, cut := limitToolCallsForRound(tc.calls)
			if cut != tc.wantCut {
				t.Fatalf("limitToolCallsForRound() cut = %v, want %v", cut, tc.wantCut)
			}
			if len(got) != tc.wantCount {
				t.Fatalf("limitToolCallsForRound() count = %d, want %d", len(got), tc.wantCount)
			}
			if tc.wantCount == 0 {
				return
			}
			if got[0] != tc.wantFirst {
				t.Fatalf("limitToolCallsForRound() first = %#v, want %#v", got[0], tc.wantFirst)
			}
		})
	}
}

func TestLimitToolCallsForRound_BatchesArtifactReads(t *testing.T) {
	calls := []llm.ToolCall{
		{ID: "call_1", Name: "pdf", Arguments: `{"path":"report.pdf","pages":[1,2]}`},
		{ID: "call_2", Name: "pdf", Arguments: `{"path":"report.pdf","pages":[3,4]}`},
		{ID: "call_3", Name: "convert", Arguments: `{"input_path":"report.pdf","output_path":"report.txt","target_format":"txt"}`},
	}

	got, cut := limitToolCallsForRound(calls)
	if cut {
		t.Fatalf("limitToolCallsForRound() cut = %v, want false", cut)
	}
	if len(got) != len(calls) {
		t.Fatalf("len(got) = %d, want %d", len(got), len(calls))
	}
}

func TestLimitToolCallsForRound_BatchesArtifactReadsWithTrailingWrite(t *testing.T) {
	calls := []llm.ToolCall{
		{ID: "call_1", Name: "pdf", Arguments: `{"path":"report.pdf","pages":[1,2]}`},
		{ID: "call_2", Name: "pdf", Arguments: `{"path":"report.pdf","pages":[3,4]}`},
		{ID: "call_3", Name: "file_write", Arguments: `{"path":"answer.txt","content":"done"}`},
	}

	got, cut := limitToolCallsForRound(calls)
	if cut {
		t.Fatalf("limitToolCallsForRound() cut = %v, want false", cut)
	}
	if len(got) != len(calls) {
		t.Fatalf("len(got) = %d, want %d", len(got), len(calls))
	}
	if got[len(got)-1].Name != "file_write" {
		t.Fatalf("last tool = %#v, want trailing file_write", got[len(got)-1])
	}
}
