package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skilladvisor"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmarket"
)

func TestMarketAdviseRejectsInvalidQueriesBeforeSelection(t *testing.T) {
	for _, body := range []string{`{}`, `{"query":"  \n\t"}`, `{"query":"\u3000\u00a0"}`, `{"query":42}`, `{"query":[]}`, `{"query":`} {
		t.Run(body, func(t *testing.T) {
			handler := NewSkillHandler(skill.NewRegistry())
			selector := &stubInstalledSkillSelector{}
			handler.SetSkillSelector(selector)
			req := httptest.NewRequest(http.MethodPost, "/skills/advise", strings.NewReader(body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			if err := handler.MarketAdviseSkills(echo.New().NewContext(req, rec)); err != nil {
				t.Fatal(err)
			}
			if rec.Code != http.StatusBadRequest || selector.called != 0 {
				t.Fatalf("invalid query reached selector: status=%d calls=%d body=%s", rec.Code, selector.called, rec.Body.String())
			}
		})
	}
}

func TestMarketAdvisePreservesExplicitDecisionWithDiscoveryWrapper(t *testing.T) {
	handler := NewSkillHandler(skill.NewRegistry())
	selector := &stubInstalledSkillSelector{}
	handler.SetSkillSelector(selector)
	searchCalls := 0
	handler.SetSkillAdvisor(skilladvisor.NewService(skilladvisor.SearchFunc(func(_ context.Context, _ skillmarket.SearchQuery) (*skillmarket.SearchResponse, error) {
		searchCalls++
		return nil, nil
	})))
	const raw = "帮我找一个技能，解析 PDF"
	decision := agentcore.Decision{Query: "解析 PDF", SelectedSkill: "pdf", Confidence: .95}
	body, err := json.Marshal(marketAdviceRequest{Query: raw, InstalledDecision: &decision})
	if err != nil {
		t.Fatal(err)
	}
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
	if rec.Code != http.StatusOK || selector.called != 0 || searchCalls != 0 || advice.Query != raw || !reflect.DeepEqual(advice.InstalledDecision, &decision) {
		t.Fatalf("explicit decision was changed: status=%d selector=%d search=%d advice=%+v", rec.Code, selector.called, searchCalls, advice)
	}
}
