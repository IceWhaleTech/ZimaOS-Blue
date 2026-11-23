package skillmarket

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

func TestSearchQueryPlanInputBoundary(t *testing.T) {
	planner := NewSearchQueryPlanner(queryPlanChatFunc(func(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
		t.Error("local or invalid input reached model")
		return nil, nil
	}), "zh")
	for name, raw := range map[string]string{
		"empty": "", "whitespace": " \t\n", "oversize": strings.Repeat("é", 2049),
		"invalid-utf8": "Trouvez une compétence \xff", "english": "Find a skill for offline OCR",
		"chinese-ascii":    "帮我找一个技能，离线 PDF OCR，不要上传云端",
		"unicode-filename": "résumé.pdf", "unicode-windows-path": `C:\资料\报告.pdf`,
		"unicode-absolute-path": "/tmp/résumé.pdf", "unicode-relative-path": "./résumé.pdf",
		"unicode-parent-path": "../資料/報告.pdf", "combining-filename": "re\u0301sume\u0301.pdf",
	} {
		t.Run(name, func(t *testing.T) {
			plan := planner.Plan(context.Background(), raw)
			if plan.Query != RewriteSearchQuery(raw) || plan.SemanticQuery != plan.Query || plan.Usage != nil || plan.ModelCalled || plan.Degraded {
				t.Fatalf("local input changed: %+v", plan)
			}
		})
	}
	var missing *SearchQueryPlanner
	if got := missing.Plan(context.Background(), "Trouvez une compétence PDF"); got.ModelCalled || got.Degraded || got.Query != "Trouvez une compétence PDF" {
		t.Fatalf("nil planner must retain local fallback: %+v", got)
	}
}

func TestSearchQueryPlanRejectsMalformedModelResponse(t *testing.T) {
	const raw = "Trouvez une compétence pour lire les documents"
	valid := `{"query":"read documents","semantic_query":"读取文档"}`
	for name, response := range map[string]*llm.ChatResponse{
		"nil":              nil,
		"empty":            {},
		"array":            {Message: llm.Message{Content: `[]`}},
		"null":             {Message: llm.Message{Content: `null`}},
		"numeric-field":    {Message: llm.Message{Content: `{"query":42,"semantic_query":"读取文档"}`}},
		"null-field":       {Message: llm.Message{Content: `{"query":null,"semantic_query":"读取文档"}`}},
		"blank-semantic":   {Message: llm.Message{Content: `{"query":"read documents","semantic_query":" \n\t"}`}},
		"missing-query":    {Message: llm.Message{Content: `{"semantic_query":"读取文档"}`}},
		"missing-semantic": {Message: llm.Message{Content: `{"query":"read documents"}`}},
		"unknown-field":    {Message: llm.Message{Content: `{"query":"read documents","semantic_query":"读取文档","tools":[]}`}},
		"trailing-text":    {Message: llm.Message{Content: valid + " explanation"}},
		"trailing-json":    {Message: llm.Message{Content: valid + " null"}},
		"markdown":         {Message: llm.Message{Content: "```json\n" + valid + "\n```"}},
		"oversize":         {Message: llm.Message{Content: valid + strings.Repeat(" ", 8193)}},
		"invalid-utf8":     {Message: llm.Message{Content: "{\"query\":\"read \xff documents\",\"semantic_query\":\"读取文档\"}"}},
		"tool-call":        {Message: llm.Message{Content: valid, ToolCalls: []llm.ToolCall{{ID: "forbidden"}}}},
	} {
		t.Run(name, func(t *testing.T) {
			planner := NewSearchQueryPlanner(queryPlanChatFunc(func(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
				return response, nil
			}), "zh")
			got := planner.Plan(context.Background(), raw)
			if !got.Degraded || !got.ModelCalled || got.Query != raw || got.SemanticQuery != raw || got.Source != "rules" {
				t.Fatalf("untrusted response accepted: %+v", got)
			}
		})
	}
}

func TestSearchQueryPlanEmbeddingTargetAndPartialUsage(t *testing.T) {
	for _, target := range []struct{ language, prompt, semantic string }{
		{"zh", "Chinese", "读取文档"}, {"en", "English", "read documents"}, {"", "the original user language", "lire les documents"},
	} {
		t.Run(target.language, func(t *testing.T) {
			for name, usage := range map[string]llm.Usage{
				"prompt": {PromptTokens: 11}, "completion": {CompletionTokens: 7}, "total": {TotalTokens: 18},
				"cache-read": {CacheReadInputTokens: 9}, "cache-create": {CacheCreationInputTokens: 6},
			} {
				t.Run(name, func(t *testing.T) {
					planner := NewSearchQueryPlanner(queryPlanChatFunc(func(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
						if !strings.Contains(req.Messages[0].Content, target.prompt) {
							t.Errorf("wrong embedding target: %s", req.Messages[0].Content)
						}
						content, _ := json.Marshal(map[string]string{"query": "read documents", "semantic_query": target.semantic})
						return &llm.ChatResponse{Message: llm.Message{Content: " \n" + string(content) + "\t"}, Usage: usage}, nil
					}), target.language)
					got := planner.Plan(context.Background(), "Trouvez une compétence pour lire les documents")
					if got.Degraded || got.SemanticQuery != target.semantic || got.Usage == nil || *got.Usage != usage {
						t.Fatalf("lost target or partial usage: %+v", got)
					}
				})
			}
		})
	}
}

func TestSearchQueryPlanPreservesUnicodeLiteralsAndSignedNumbers(t *testing.T) {
	for name, literal := range map[string]string{
		"unicode-file": "résumé.pdf", "combining-file": "re\u0301sume\u0301.pdf", "unicode-path": "/資料/報告.pdf",
		"backtick": "`::std::find`", "curly-quote": "“rapport.pdf”", "corner-quote": "「報告書.pdf」",
		"negative": "-10", "positive-sign": "+10", "version": "1.2.3", "date": "2026-11-23",
	} {
		t.Run(name, func(t *testing.T) {
			raw := "Trouvez une compétence pour traiter " + literal
			for _, keep := range []bool{true, false} {
				planner := NewSearchQueryPlanner(queryPlanChatFunc(func(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
					value := literal
					if !keep {
						value = "10"
					}
					content, _ := json.Marshal(map[string]string{"query": "process " + value, "semantic_query": "处理 " + value})
					return &llm.ChatResponse{Message: llm.Message{Content: string(content)}}, nil
				}), "zh")
				got := planner.Plan(context.Background(), raw)
				if got.Degraded == keep || (!keep && (got.Query != raw || got.SemanticQuery != raw)) {
					t.Errorf("literal retention=%v accepted wrong result: %+v", keep, got)
				}
			}
		})
	}
}

func TestSearchQueryPlanProviderErrorRetainsReportedUsage(t *testing.T) {
	planner := NewSearchQueryPlanner(queryPlanChatFunc(func(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
		return &llm.ChatResponse{Usage: llm.Usage{PromptTokens: 80}}, errors.New("provider failed after billing")
	}), "zh")
	const raw = "Trouvez une compétence pour lire les documents"
	first, second := planner.Plan(context.Background(), raw), planner.Plan(context.Background(), raw)
	if !first.Degraded || !first.ModelCalled || first.Usage == nil || first.Usage.PromptTokens != 80 || !second.CacheHit || second.Usage != nil || second.ModelCalled {
		t.Fatalf("error usage lost or counted twice: first=%+v second=%+v", first, second)
	}
}

func FuzzSearchQueryPlanModelOutputBoundary(f *testing.F) {
	for _, seed := range []string{
		`{"query":"read documents","semantic_query":"读取文档"}`, `null`, `{"query":"read","semantic_query":null}`,
		"{\"query\":\"\xff\",\"semantic_query\":\"read\"}", `{"query":"","semantic_query":"read"}`, "{} {}",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, content string) {
		if len(content) > 16384 {
			t.Skip()
		}
		planner := NewSearchQueryPlanner(queryPlanChatFunc(func(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
			return &llm.ChatResponse{Message: llm.Message{Content: content}, Usage: llm.Usage{TotalTokens: 17}}, nil
		}), "zh")
		const raw = "Trouvez une compétence pour lire les documents"
		got := planner.Plan(context.Background(), raw)
		if got.OriginalQuery != raw || got.Usage == nil || got.Usage.TotalTokens != 17 || !got.ModelCalled {
			t.Fatalf("response damaged source or billing: %+v", got)
		}
		if got.Degraded {
			if got.Query != raw || got.SemanticQuery != raw {
				t.Fatalf("failed rewrite damaged fallback: %+v", got)
			}
		} else if !utf8.ValidString(content) || len(content) > 8192 || got.Query == "" || got.SemanticQuery == "" {
			t.Fatalf("invalid model output accepted: %+v", got)
		}
	})
}
