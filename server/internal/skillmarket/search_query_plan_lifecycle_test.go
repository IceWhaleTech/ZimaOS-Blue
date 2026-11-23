package skillmarket

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

// Observes entry into the pending-flight select, without a timing-dependent sleep.
type observedQueryWaitContext struct {
	context.Context
	once    sync.Once
	waiting chan struct{}
}

func (c *observedQueryWaitContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.waiting) })
	return c.Context.Done()
}

func TestSearchQueryPlanCancellationDuringFlight(t *testing.T) {
	for _, cancelOwner := range []bool{false, true} {
		name := "waiter"
		if cancelOwner {
			name = "owner"
		}
		t.Run(name, func(t *testing.T) {
			const raw = "Trouvez une compétence pour lire les documents"
			started, release := make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			defer releaseOnce.Do(func() { close(release) })
			var calls atomic.Int32
			planner := NewSearchQueryPlanner(queryPlanChatFunc(func(ctx context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
				if calls.Add(1) == 1 {
					close(started)
				}
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-release:
					return &llm.ChatResponse{Message: llm.Message{Content: `{"query":"read documents","semantic_query":"读取文档"}`}, Usage: llm.Usage{TotalTokens: 21}}, nil
				}
			}), "zh")
			ownerCtx, ownerCancel := context.WithCancel(context.Background())
			defer ownerCancel()
			ownerDone, waiterDone := make(chan SearchQueryPlan, 1), make(chan SearchQueryPlan, 1)
			var group sync.WaitGroup
			defer func() {
				ownerCancel()
				releaseOnce.Do(func() { close(release) })
				group.Wait()
			}()
			group.Add(1)
			go func() { defer group.Done(); ownerDone <- planner.Plan(ownerCtx, raw) }()
			<-started
			waiterCtx, waiterCancel := context.WithCancel(context.Background())
			defer waiterCancel()
			observed := &observedQueryWaitContext{Context: waiterCtx, waiting: make(chan struct{})}
			group.Add(1)
			go func() { defer group.Done(); waiterDone <- planner.Plan(observed, raw) }()
			select {
			case <-observed.waiting:
			case <-time.After(2 * time.Second):
				t.Fatal("waiter never joined the in-flight request")
			}
			if cancelOwner {
				ownerCancel()
			} else {
				waiterCancel()
			}
			var waiter SearchQueryPlan
			select {
			case waiter = <-waiterDone:
			case <-time.After(2 * time.Second):
				t.Fatal("cancellation did not release waiter")
			}
			if !waiter.Degraded || waiter.FallbackReason != "cancelled" || waiter.ModelCalled || waiter.Usage != nil {
				t.Errorf("cancelled waiter billed or changed query: %+v", waiter)
			}
			releaseOnce.Do(func() { close(release) })
			owner := <-ownerDone
			if owner.Degraded != cancelOwner || !owner.ModelCalled || (!cancelOwner && (owner.Usage == nil || owner.Usage.TotalTokens != 21)) {
				t.Errorf("wrong owner outcome: %+v", owner)
			}
			recovered := planner.Plan(context.Background(), raw)
			wantCalls := int32(1)
			if cancelOwner {
				wantCalls = 2
			}
			if recovered.Degraded || recovered.Query != "read documents" || calls.Load() != wantCalls || recovered.CacheHit == cancelOwner {
				t.Fatalf("cancel poisoned cache or owner work: %+v calls=%d", recovered, calls.Load())
			}
		})
	}
}

func TestSearchQueryPlanAdditionalLanguages(t *testing.T) {
	for language, raw := range map[string]string{
		"italian":    "Trova una competenza per leggere documenti senza rete",
		"portuguese": "Encontre uma habilidade para ler documentos sem rede",
		"dutch":      "Vind een vaardigheid om documenten offline te lezen",
		"polish":     "Znajdź umiejętność odczytywania dokumentów bez internetu",
		"romanian":   "Caută o abilitate pentru a citi documente fără internet",
		"russian":    "Найди навык для чтения документов без интернета",
		"turkish":    "Belgeleri çevrimdışı okumak için bir beceri bul",
		"arabic":     "ابحث عن مهارة لقراءة المستندات بدون إنترنت",
		"hindi":      "बिना इंटरनेट के दस्तावेज़ पढ़ने का कौशल खोजें",
		"thai":       "ค้นหาทักษะสำหรับอ่านเอกสารโดยไม่ใช้อินเทอร์เน็ต",
	} {
		t.Run(language, func(t *testing.T) {
			calls := 0
			planner := NewSearchQueryPlanner(queryPlanChatFunc(func(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
				calls++
				if req.Messages[1].Content != raw {
					t.Error("source language text changed before rewriting")
				}
				return &llm.ChatResponse{Message: llm.Message{Content: `{"query":"read documents offline","semantic_query":"离线读取文档"}`}}, nil
			}), "zh")
			got := planner.Plan(context.Background(), raw)
			if calls != 1 || got.Degraded || got.Query != "read documents offline" || got.OriginalQuery != raw {
				t.Fatalf("language did not reach configured rewrite backend: %+v calls=%d", got, calls)
			}
		})
	}
}
