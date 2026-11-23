package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skilladvisor"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmarket"
	"github.com/labstack/echo/v4"
)

func TestMarketAdviceMultilingualOutcomeBoundaries(t *testing.T) {
	for _, mode := range []string{"explicit-installed", "selector-sufficient", "selector-error", "search-error"} {
		t.Run(mode, func(t *testing.T) {
			calls, searches := 0, 0
			handler := NewSkillHandler(skill.NewRegistry())
			selector := &stubInstalledSkillSelector{}
			if mode == "selector-sufficient" {
				selector.decision = agentcore.Decision{SelectedSkill: "pdf", Confidence: .99}
			}
			if mode == "selector-error" {
				selector.err = errors.New("selector unavailable")
			}
			handler.SetSkillSelector(selector)
			planner := skillmarket.NewSearchQueryPlanner(multilingualSearchChatFunc(func(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
				calls++
				return &llm.ChatResponse{Message: llm.Message{Content: `{"query":"offline PDF OCR","semantic_query":"离线 PDF OCR"}`}, Usage: llm.Usage{TotalTokens: 29}}, nil
			}), "zh")
			handler.SetSearchQueryPlanner(planner)
			advisor := skilladvisor.NewService(skilladvisor.SearchFunc(func(_ context.Context, q skillmarket.SearchQuery) (*skillmarket.SearchResponse, error) {
				searches++
				if searches == 1 && q.Query != "offline PDF OCR" {
					t.Errorf("raw query reached retrieval: %q", q.Query)
				}
				if mode == "search-error" {
					return nil, errors.New("market unavailable")
				}
				return &skillmarket.SearchResponse{}, nil
			}))
			advisor.SetSearchQueryPlanner(planner)
			handler.SetSkillAdvisor(advisor)
			const raw = "Trouvez une compétence OCR PDF hors ligne"
			body := marketAdviceRequest{Query: raw}
			if mode == "explicit-installed" {
				body.InstalledDecision = &agentcore.Decision{SelectedSkill: "pdf", Confidence: .99}
			}
			encoded, _ := json.Marshal(body)
			req := httptest.NewRequest(http.MethodPost, "/skills/advise", strings.NewReader(string(encoded)))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			if err := handler.MarketAdviseSkills(echo.New().NewContext(req, rec)); err != nil {
				t.Fatal(err)
			}
			var got marketAdviceResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if rec.Code != 200 || got.Advice == nil || got.Query != raw {
				t.Fatalf("lost raw request: %s", rec.Body.String())
			}
			if mode == "explicit-installed" {
				if calls != 0 || searches != 0 || selector.called != 0 || got.QueryPlan != nil || got.InstalledDecision.SelectedSkill != "pdf" {
					t.Fatalf("explicit decision caused extra work: calls=%d searches=%d response=%s", calls, searches, rec.Body.String())
				}
				return
			}
			if calls != 1 || selector.called != 1 || selector.query != "offline PDF OCR" || got.QueryPlan == nil || got.QueryPlan.Usage == nil || got.QueryPlan.Usage.TotalTokens != 29 {
				t.Fatalf("lost shared plan or usage: calls=%d response=%s", calls, rec.Body.String())
			}
			if mode == "selector-sufficient" && (searches != 0 || got.NeedStoreSearch) {
				t.Error("sufficient installed skill caused market search")
			}
			if mode == "selector-error" && (searches == 0 || got.SkillSelectorError != "selector unavailable") {
				t.Error("selector failure blocked fallback or diagnostic")
			}
			if mode == "search-error" && (searches == 0 || got.SearchError != "market unavailable") {
				t.Error("search failure lost diagnostic")
			}
		})
	}
}

func TestMarketAdviceMultilingualUnknownUsageAndCacheJSON(t *testing.T) {
	handler := NewSkillHandler(skill.NewRegistry())
	handler.SetSkillSelector(&stubInstalledSkillSelector{decision: agentcore.Decision{SelectedSkill: "pdf", Confidence: .99}})
	calls := 0
	handler.SetSearchQueryPlanner(skillmarket.NewSearchQueryPlanner(multilingualSearchChatFunc(func(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
		calls++
		return &llm.ChatResponse{Message: llm.Message{Content: `{"query":"read PDF files","semantic_query":"读取 PDF 文件"}`}}, nil
	}), "zh"))
	handler.SetSkillAdvisor(skilladvisor.NewService(nil))
	for attempt := range 2 {
		req := httptest.NewRequest(http.MethodPost, "/skills/advise", strings.NewReader(`{"query":"Trouvez une compétence pour lire les fichiers PDF"}`))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		if err := handler.MarketAdviseSkills(echo.New().NewContext(req, rec)); err != nil {
			t.Fatal(err)
		}
		var got struct {
			Plan map[string]any `json:"query_plan"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if rec.Code != 200 || got.Plan == nil || calls != 1 {
			t.Fatalf("invalid HTTP plan: %s", rec.Body.String())
		}
		if _, present := got.Plan["usage"]; present {
			t.Fatalf("unknown usage exposed as billed zero: %s", rec.Body.String())
		}
		if attempt == 0 && got.Plan["model_called"] != true {
			t.Fatal("first model attempt missing from JSON")
		}
		if attempt == 1 && (got.Plan["cache_hit"] != true || got.Plan["model_called"] == true) {
			t.Fatal("cache response billed as fresh call")
		}
	}
}
