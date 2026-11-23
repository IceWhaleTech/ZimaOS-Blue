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

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skilladvisor"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmarket"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillstore"
	"github.com/labstack/echo/v4"
)

func TestMarketAdviseSkillsSelectsRequestedCapability(t *testing.T) {
	handler := NewSkillHandler(skill.NewRegistry())
	selector := &stubInstalledSkillSelector{}
	handler.SetSkillSelector(selector)
	var queries []string
	handler.SetSkillAdvisor(skilladvisor.NewService(skilladvisor.SearchFunc(func(_ context.Context, query skillmarket.SearchQuery) (*skillmarket.SearchResponse, error) {
		queries = append(queries, query.Query)
		return &skillmarket.SearchResponse{}, nil
	})))
	const raw = "帮我找一个技能，PDF OCR"
	req := httptest.NewRequest(http.MethodPost, "/skills/advise", strings.NewReader(`{"query":"`+raw+`"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	if err := handler.MarketAdviseSkills(echo.New().NewContext(req, rec)); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if selector.query != "PDF OCR" {
		t.Errorf("selector received %q, want requested capability only", selector.query)
	}
	if len(queries) == 0 || queries[0] != "PDF OCR" {
		t.Errorf("store queries = %q, want requested capability first", queries)
	}
	var advice skilladvisor.Advice
	if err := json.Unmarshal(rec.Body.Bytes(), &advice); err != nil {
		t.Fatal(err)
	}
	if advice.Query != raw {
		t.Errorf("advisor response lost original query: %q", advice.Query)
	}
}

func TestSkillSearchFallbackRewritesSemanticDiscoveryQuery(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "skills.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := skillstore.NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertSkill(context.Background(), &skillstore.Skill{
		ID: "rebase-helper", Name: "Rebase Helper", Version: "1.0.0", SearchContent: "rebase helper",
	}); err != nil {
		t.Fatal(err)
	}
	for name, handle := range map[string]func(*SkillHandler, echo.Context) error{
		"marketplace": (*SkillHandler).MarketSearchSkills,
		"legacy":      (*SkillHandler).SearchSkills,
	} {
		t.Run(name, func(t *testing.T) {
			for _, tc := range []struct {
				name, flag, query string
				total             int
			}{
				{"semantic_true", "true", "帮我找一个技能，rebase", 1},
				{"semantic_one", "1", "帮我找一个技能，rebase", 1},
				{"literal_false", "false", "帮我找一个技能，rebase", 0},
				{"literal_zero", "0", "帮我找一个技能，rebase", 0},
				{"literal_omitted", "", "帮我找一个技能，rebase", 0},
				{"literal_capability", "false", "rebase", 1},
			} {
				t.Run(tc.name, func(t *testing.T) {
					handler := NewSkillHandler(skill.NewRegistry())
					handler.SetStore(store)
					params := url.Values{"q": {tc.query}}
					if tc.flag != "" {
						params.Set("semantic", tc.flag)
					}
					req := httptest.NewRequest(http.MethodGet, "/skills/search?"+params.Encode(), nil)
					rec := httptest.NewRecorder()
					if err := handle(handler, echo.New().NewContext(req, rec)); err != nil {
						t.Fatal(err)
					}
					var payload struct {
						Total int `json:"total"`
					}
					if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
						t.Fatal(err)
					}
					if rec.Code != http.StatusOK || payload.Total != tc.total || strings.Contains(rec.Body.String(), "rebase-helper") != (tc.total == 1) {
						t.Fatalf("fallback total want %d: status=%d body=%s", tc.total, rec.Code, rec.Body.String())
					}
				})
			}
		})
	}
}
