package skillmarket

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

type queryPlanChatFunc func(context.Context, llm.ChatRequest) (*llm.ChatResponse, error)

func (f queryPlanChatFunc) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	return f(ctx, req)
}

func TestSearchQueryPlanMultilingualAndUsage(t *testing.T) {
	for _, raw := range []string{
		"Aidez-moi à trouver une compétence pour lire des fichiers PDF sans connexion",
		"Finde eine Fähigkeit zum Lesen von PDF Dateien ohne Internet",
		"PDFをオフラインで読めるスキルを探してください",
		"인터넷 없이 PDF 문서를 읽는 스킬을 찾아주세요",
		"Encuentra una habilidad para leer archivos PDF sin conexión",
	} {
		t.Run(raw, func(t *testing.T) {
			calls := 0
			planner := NewSearchQueryPlanner(queryPlanChatFunc(func(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
				calls++
				if _, ok := ctx.Deadline(); !ok || len(req.Tools) != 0 || req.MaxTokens > 512 || req.Model != "auto" {
					t.Errorf("rewrite must be bounded and use configured routing: %+v", req)
				}
				if len(req.Messages) != 2 || req.Messages[1].Content != raw || !strings.Contains(req.Messages[0].Content, "Chinese") {
					t.Errorf("wrong source/embedding language: %+v", req.Messages)
				}
				return &llm.ChatResponse{Model: "untrusted-upstream-name", Message: llm.Message{Content: `{"query":"read PDF files offline without internet","semantic_query":"离线读取 PDF 文件，不联网"}`}, Usage: llm.Usage{PromptTokens: 120, CompletionTokens: 20, TotalTokens: 140}}, nil
			}), "zh")
			first := planner.Plan(context.Background(), raw)
			if first.OriginalQuery != raw || first.Query != "read PDF files offline without internet" || first.SemanticQuery != "离线读取 PDF 文件，不联网" || first.Source != "model" || first.Degraded || first.CacheHit || first.Usage == nil || first.Usage.TotalTokens != 140 {
				t.Fatalf("incorrect multilingual plan: %+v", first)
			}
			second := planner.Plan(context.Background(), raw)
			if calls != 1 || !second.CacheHit || second.Query != first.Query || second.Usage != nil {
				t.Fatalf("cache billed a second model call: calls=%d plan=%+v", calls, second)
			}
		})
	}
}

func TestSearchQueryPlanKeepsLocalRulesAndLiterals(t *testing.T) {
	planner := NewSearchQueryPlanner(queryPlanChatFunc(func(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
		t.Error("literal/local query must not reach model")
		return nil, nil
	}), "zh")
	for _, raw := range []string{"帮我找一个技能，::std::find", "请帮我搜索一个技能：本地搜索 PDF，不要上传云端", "帮我找一个技能，离线 OCR PDF，不要上传云端", "Help me find a skill for :root", "find-skills", "search.py", `"Aidez-moi à trouver un skill"`, `C:\work\file.pdf`, "PDF OCR", ""} {
		plan := planner.Plan(context.Background(), raw)
		if plan.Query != RewriteSearchQuery(raw) || plan.SemanticQuery != plan.Query || plan.Degraded || plan.Usage != nil {
			t.Errorf("local content changed: %q: %+v", raw, plan)
		}
	}
}

func TestSearchQueryPlanFailureRetainsOriginalAndUsage(t *testing.T) {
	const raw = "Trouvez une compétence pour lire le fichier \"C:\\work\\report.pdf\" sans réseau"
	for _, content := range []string{"", "not JSON", `{"query":"","semantic_query":"读取 PDF"}`, `{"query":"read files","semantic_query":"读取文件"}`, `{"query":"read files","semantic_query":"读取文件","extra":true}`} {
		t.Run(content, func(t *testing.T) {
			calls := 0
			planner := NewSearchQueryPlanner(queryPlanChatFunc(func(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
				calls++
				return &llm.ChatResponse{Message: llm.Message{Content: content}, Usage: llm.Usage{TotalTokens: 50}}, nil
			}), "zh")
			plan := planner.Plan(context.Background(), raw)
			if plan.Query != raw || plan.SemanticQuery != raw || !plan.Degraded || plan.FallbackReason != "invalid_output" || plan.Usage == nil || plan.Usage.TotalTokens != 50 {
				t.Fatalf("invalid rewrite not rejected: %+v", plan)
			}
			cached := planner.Plan(context.Background(), raw)
			if calls != 1 || !cached.CacheHit || cached.Usage != nil || !cached.Degraded {
				t.Fatalf("failure cooldown lost: calls=%d plan=%+v", calls, cached)
			}
		})
	}
}

func TestSearchQueryPlanTimeoutCancellationAndRecovery(t *testing.T) {
	const raw = "Finde eine Fähigkeit zur Verarbeitung von Dokumenten"
	var calls atomic.Int32
	planner := NewSearchQueryPlanner(queryPlanChatFunc(func(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
		if calls.Add(1) == 1 {
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return &llm.ChatResponse{Message: llm.Message{Content: `{"query":"process documents","semantic_query":"处理文档"}`}}, nil
	}), "zh")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if plan := planner.Plan(ctx, raw); !plan.Degraded || calls.Load() != 0 {
		t.Fatalf("cancelled request reached provider: %+v calls=%d", plan, calls.Load())
	}
	planner.timeout = 10 * time.Millisecond
	if plan := planner.Plan(context.Background(), raw); !plan.Degraded || plan.FallbackReason != "model_timeout" {
		t.Fatalf("timeout did not fall back: %+v", plan)
	}
	planner.failures.Del(raw)
	if plan := planner.Plan(context.Background(), raw); plan.Degraded || plan.Query != "process documents" || calls.Load() != 2 {
		t.Fatalf("model did not recover: %+v calls=%d", plan, calls.Load())
	}
	missing := NewSearchQueryPlanner(nil, "zh").Plan(context.Background(), raw)
	if !missing.Degraded || missing.Query != raw || missing.FallbackReason != "model_unavailable" {
		t.Fatalf("missing model lost original: %+v", missing)
	}
}

func TestSearchQueryPlanCoalescesCallsAndWaiterCancellation(t *testing.T) {
	const raw = "Finde eine Fähigkeit zur Verarbeitung von Dokumenten"
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	planner := NewSearchQueryPlanner(queryPlanChatFunc(func(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
		calls.Add(1)
		close(started)
		<-release
		return &llm.ChatResponse{Message: llm.Message{Content: `{"query":"process documents","semantic_query":"处理文档"}`}, Usage: llm.Usage{TotalTokens: 30}}, nil
	}), "zh")
	var group sync.WaitGroup
	group.Add(1)
	go func() { defer group.Done(); planner.Plan(context.Background(), raw) }()
	<-started
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := planner.Plan(ctx, raw); !got.Degraded || got.FallbackReason != "cancelled" {
		t.Errorf("waiter cancellation blocked: %+v", got)
	}
	for range 12 {
		group.Add(1)
		go func() {
			defer group.Done()
			got := planner.Plan(context.Background(), raw)
			if got.Query != "process documents" || got.Usage != nil || !got.CacheHit {
				t.Errorf("duplicate call/usage: %+v", got)
			}
		}()
	}
	close(release)
	group.Wait()
	if calls.Load() != 1 {
		t.Fatalf("provider calls=%d", calls.Load())
	}
}

func TestSearchQueryPlanProviderError(t *testing.T) {
	planner := NewSearchQueryPlanner(queryPlanChatFunc(func(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
		return nil, errors.New("not configured")
	}), "en")
	plan := planner.Plan(context.Background(), "Finde eine Fähigkeit zur Verarbeitung von Dokumenten")
	if !plan.Degraded || plan.FallbackReason != "model_unavailable" {
		t.Fatalf("lost fallback: %+v", plan)
	}
}

func TestSearchQueryPlanConcurrentBudgetRecoversWithoutCachingBusy(t *testing.T) {
	started, release := make(chan struct{}, 4), make(chan struct{})
	var calls atomic.Int32
	planner := NewSearchQueryPlanner(queryPlanChatFunc(func(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
		calls.Add(1)
		started <- struct{}{}
		select {
		case <-release:
			return &llm.ChatResponse{Message: llm.Message{Content: `{"query":"process documents","semantic_query":"处理文档"}`}}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}), "zh")
	var group sync.WaitGroup
	for _, noun := range []string{"Dokumenten", "Fotos", "Videos", "Musik"} {
		group.Add(1)
		go func() {
			defer group.Done()
			planner.Plan(context.Background(), "Finde eine Fähigkeit zur Verarbeitung von "+noun)
		}()
	}
	for range 4 {
		<-started
	}
	const extra = "Finde eine Fähigkeit zum Lesen von Dokumenten"
	busy := planner.Plan(context.Background(), extra)
	close(release)
	group.Wait()
	if busy.FallbackReason != "model_busy" || !busy.Degraded || busy.ModelCalled || busy.Usage != nil || calls.Load() != 4 {
		t.Fatalf("concurrent budget failed: %+v calls=%d", busy, calls.Load())
	}
	if recovered := planner.Plan(context.Background(), extra); recovered.Degraded || calls.Load() != 5 {
		t.Fatalf("busy fallback prevented recovery: %+v calls=%d", recovered, calls.Load())
	}
}

func TestSearchQueryPlanRejectsLostNumbersAndExcessOutput(t *testing.T) {
	for _, tc := range []struct{ raw, content string }{
		{"Trouvez une compétence pour compresser des fichiers à moins de 10 Mo", `{"query":"compress files below 100 MB","semantic_query":"压缩文件到 100 MB 以下"}`},
		{"Trouvez une compétence pour utiliser ::std::find dans du code C++", `{"query":"use std::find in C++","semantic_query":"在 C++ 中使用 std::find"}`},
		{"Trouvez une compétence pour lire des fichiers", `{"query":"read files","semantic_query":"读取文件"} {"query":"other"}`},
	} {
		planner := NewSearchQueryPlanner(queryPlanChatFunc(func(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
			return &llm.ChatResponse{Message: llm.Message{Content: tc.content}}, nil
		}), "zh")
		if got := planner.Plan(context.Background(), tc.raw); !got.Degraded || got.Query != tc.raw {
			t.Errorf("unsafe rewrite accepted: %+v", got)
		}
	}
}

func TestSearchQueryPlanUsageUnknownIsNotZeroAndCacheIsNotBilled(t *testing.T) {
	planner := NewSearchQueryPlanner(queryPlanChatFunc(func(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
		return &llm.ChatResponse{Message: llm.Message{Content: `{"query":"read files","semantic_query":"读取文件"}`}}, nil
	}), "zh")
	const raw = "Trouvez une compétence pour lire des fichiers"
	first := planner.Plan(context.Background(), raw)
	if !first.ModelCalled || first.Usage != nil {
		t.Fatalf("unknown usage was reported as zero: %+v", first)
	}
	second := planner.Plan(context.Background(), raw)
	if second.ModelCalled || second.Usage != nil || !second.CacheHit {
		t.Fatalf("cache hit counted as provider usage: %+v", second)
	}
}
