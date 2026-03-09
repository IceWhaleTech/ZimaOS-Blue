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
