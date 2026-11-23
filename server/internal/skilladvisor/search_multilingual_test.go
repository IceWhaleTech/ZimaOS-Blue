package skilladvisor

import (
	"context"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmarket"
)

type multilingualQueryCaller func(context.Context, llm.ChatRequest) (*llm.ChatResponse, error)

func (f multilingualQueryCaller) Chat(ctx context.Context, r llm.ChatRequest) (*llm.ChatResponse, error) {
	return f(ctx, r)
}

func TestAdviceMultilingualPlannerPreservesExplicitInstalledDecision(t *testing.T) {
	calls, searches := 0, 0
	service := NewService(SearchFunc(func(ctx context.Context, q skillmarket.SearchQuery) (*skillmarket.SearchResponse, error) {
		searches++
		if searches == 1 && q.Query != "offline PDF OCR" {
			t.Errorf("raw query reached search: %q", q.Query)
		}
		return &skillmarket.SearchResponse{}, nil
	}))
	service.SetSearchQueryPlanner(skillmarket.NewSearchQueryPlanner(multilingualQueryCaller(func(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
		calls++
		return &llm.ChatResponse{Message: llm.Message{Content: `{"query":"offline PDF OCR","semantic_query":"离线 PDF OCR"}`}, Usage: llm.Usage{TotalTokens: 61}}, nil
	}), "zh"))
	const raw = "Trouvez une compétence OCR PDF hors ligne"
	decision := &agentcore.Decision{SelectedSkill: "pdf", Confidence: .99}
	advice, err := service.Advise(context.Background(), raw, decision)
	if err != nil || calls != 0 || searches != 0 || advice.InstalledDecision != decision || advice.Query != raw {
		t.Fatalf("explicit decision caused extra model work: calls=%d searches=%d advice=%+v err=%v", calls, searches, advice, err)
	}
	advice, err = service.Advise(context.Background(), raw, nil)
	if err != nil || calls != 1 || searches == 0 || advice.QueryPlan == nil || advice.QueryPlan.Usage.TotalTokens != 61 {
		t.Fatalf("direct advisor not normalized: calls=%d searches=%d advice=%+v err=%v", calls, searches, advice, err)
	}
}
