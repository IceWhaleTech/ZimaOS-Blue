package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skilladvisor"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmarket"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillstore"
	"github.com/labstack/echo/v4"
)

type multilingualSearchChatFunc func(context.Context, llm.ChatRequest) (*llm.ChatResponse, error)

func (f multilingualSearchChatFunc) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	return f(ctx, req)
}

func TestMarketAdviceSharesMultilingualPlanWithSelectorAndSearcher(t *testing.T) {
	handler := NewSkillHandler(skill.NewRegistry())
	selector := &stubInstalledSkillSelector{}
	handler.SetSkillSelector(selector)
	calls := 0
	handler.SetSearchQueryPlanner(skillmarket.NewSearchQueryPlanner(multilingualSearchChatFunc(func(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
		calls++
		return &llm.ChatResponse{Message: llm.Message{Content: `{"query":"offline PDF OCR without cloud uploads","semantic_query":"离线 PDF OCR，不上传云端"}`}, Usage: llm.Usage{TotalTokens: 73}}, nil
	}), "zh"))
	var queries []string
	handler.SetSkillAdvisor(skilladvisor.NewService(skilladvisor.SearchFunc(func(ctx context.Context, query skillmarket.SearchQuery) (*skillmarket.SearchResponse, error) {
		queries = append(queries, query.Query)
		if len(queries) == 1 {
			plan, ok := skillmarket.PreparedSearchQuery(ctx, query.Query)
			if !ok || plan.SemanticQuery != "离线 PDF OCR，不上传云端" {
				t.Errorf("semantic plan was lost: %+v ok=%v", plan, ok)
			}
		}
		return &skillmarket.SearchResponse{}, nil
	})))
	const raw = "Trouvez une compétence OCR PDF hors ligne sans envoyer les données au cloud"
	body, _ := json.Marshal(map[string]string{"query": raw})
	req := httptest.NewRequest(http.MethodPost, "/skills/advise", strings.NewReader(string(body)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	if err := handler.MarketAdviseSkills(echo.New().NewContext(req, rec)); err != nil {
		t.Fatal(err)
	}
	var advice skilladvisor.Advice
	if err := json.Unmarshal(rec.Body.Bytes(), &advice); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 200 || calls != 1 || selector.query != "offline PDF OCR without cloud uploads" || len(queries) == 0 || queries[0] != selector.query || advice.Query != raw || advice.QueryPlan == nil || advice.QueryPlan.Usage.TotalTokens != 73 {
		t.Fatalf("multilingual advice failed: status=%d calls=%d selector=%q queries=%q advice=%+v", rec.Code, calls, selector.query, queries, advice)
	}
}

func TestLegacySearchUsesMultilingualPlanOnlyForSemanticRequests(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "skills.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := skillstore.NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertSkill(context.Background(), &skillstore.Skill{ID: "rebase-helper", Name: "Rebase", SearchContent: "rebase helper"}); err != nil {
		t.Fatal(err)
	}
	for name, handle := range map[string]func(*SkillHandler, echo.Context) error{"market": (*SkillHandler).MarketSearchSkills, "legacy": (*SkillHandler).SearchSkills} {
		for _, semantic := range []string{"true", "false"} {
			t.Run(name+"/"+semantic, func(t *testing.T) {
				handler := NewSkillHandler(skill.NewRegistry())
				handler.SetStore(store)
				calls := 0
				handler.SetSearchQueryPlanner(skillmarket.NewSearchQueryPlanner(multilingualSearchChatFunc(func(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
					calls++
					return &llm.ChatResponse{Message: llm.Message{Content: `{"query":"rebase","semantic_query":"变基"}`}, Usage: llm.Usage{TotalTokens: 31}}, nil
				}), "zh"))
				q := url.Values{"q": {"Finde eine Fähigkeit zum Umbasieren von Git Branches"}, "semantic": {semantic}}
				req := httptest.NewRequest(http.MethodGet, "/skills/search?"+q.Encode(), nil)
				rec := httptest.NewRecorder()
				if err := handle(handler, echo.New().NewContext(req, rec)); err != nil {
					t.Fatal(err)
				}
				var payload struct {
					Total     int                          `json:"total"`
					QueryPlan *skillmarket.SearchQueryPlan `json:"query_plan"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
					t.Fatal(err)
				}
				if semantic == "true" {
					if calls != 1 || payload.Total != 1 || payload.QueryPlan == nil || payload.QueryPlan.Usage.TotalTokens != 31 {
						t.Fatalf("rewrite missing: calls=%d payload=%+v", calls, payload)
					}
				} else if calls != 0 || payload.Total != 0 || payload.QueryPlan != nil {
					t.Fatalf("literal search changed: calls=%d payload=%+v", calls, payload)
				}
			})
		}
	}
}

func TestLegacySearchWithMarketplaceRetainsRewriteUsage(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "market.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	planner := skillmarket.NewSearchQueryPlanner(multilingualSearchChatFunc(func(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
		return &llm.ChatResponse{Message: llm.Message{Content: `{"query":"rebase","semantic_query":"变基"}`}, Usage: llm.Usage{TotalTokens: 27}}, nil
	}), "zh")
	market, err := skillmarket.NewService(db, skillmarket.Options{Config: testSkillMarketConfig(t.TempDir(), t.TempDir()), QueryPlanner: planner})
	if err != nil {
		t.Fatal(err)
	}
	defer market.Close()
	doc := &skillmarket.SkillDocument{ID: "rebase-helper", Slug: "rebase-helper", Name: "Rebase", Description: "rebase", Published: true}
	if err := market.Store().UpsertSkill(context.Background(), doc, nil, nil); err != nil {
		t.Fatal(err)
	}
	handler := NewSkillHandler(skill.NewRegistry())
	handler.SetMarketplace(market)
	q := url.Values{"q": {"Finde eine Fähigkeit zum Umbasieren von Git Branches"}, "semantic": {"true"}}
	req := httptest.NewRequest(http.MethodGet, "/skills/search?"+q.Encode(), nil)
	rec := httptest.NewRecorder()
	if err := handler.SearchSkills(echo.New().NewContext(req, rec)); err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Total     int                          `json:"total"`
		QueryPlan *skillmarket.SearchQueryPlan `json:"query_plan"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 200 || payload.Total != 1 || payload.QueryPlan == nil || payload.QueryPlan.Usage.TotalTokens != 27 {
		t.Fatalf("primary legacy mapping dropped usage: status=%d payload=%+v", rec.Code, payload)
	}
}
