package bootstrap

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skilladvisor"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillstore"
	"github.com/labstack/echo/v4"
)

type multilingualInstalledSelector struct{}

func (multilingualInstalledSelector) Select(_ context.Context, query string, _ agentcore.SelectOptions) (agentcore.Decision, error) {
	return agentcore.Decision{Query: query, SelectedSkill: "pdf", Confidence: .99}, nil
}

func TestSkillMarketplaceWiresConfiguredRuntimeForMultilingualSearch(t *testing.T) {
	cfg := &config.Config{}
	handler := serverpkg.NewSkillHandler(skill.NewRegistry())
	provider := &recordingProvider{name: "configured", models: []string{"configured-model"}}
	runtime := newRuntimeLLMProviderRef()
	runtime.SetProvider(provider)
	dataDir := t.TempDir()
	bindRouteRuntimeSkillMarketplace(handler, skillstore.NewLocalSkillScanner(dataDir), routeRuntimeContractSkillOptions{
		dataDir: dataDir, appConfig: cfg, services: &Services{SkillRegistry: skill.NewRegistry()}, runtimeLLM: runtime,
	}, context.Background())
	handler.SetSkillSelector(multilingualInstalledSelector{})
	// A literal request must not load or call the configured model.
	for _, raw := range []string{"find-skills", "Finde eine Fähigkeit zur Verarbeitung von Dokumenten"} {
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
		if raw == "find-skills" && provider.calls != 0 {
			t.Fatal("literal query called LLM")
		}
		if raw != "find-skills" && (provider.calls != 1 || advice.QueryPlan == nil || advice.QueryPlan.FallbackReason != "invalid_output" || advice.Query != raw) {
			t.Fatalf("runtime not wired: calls=%d advice=%+v", provider.calls, advice)
		}
	}
	_ = handler.Close()
}

func TestSkillMarketplaceMultilingualEmbeddingTarget(t *testing.T) {
	for _, tc := range []struct{ model, target string }{
		{"", "Chinese"}, {"BAAI/bge-small-zh-v1.5", "Chinese"},
		{"org/bge-small-EN-v1.5", "English"}, {"org/multilingual-embed", "the original user language"},
	} {
		t.Run(tc.model, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Embedding.Model = tc.model
			handler := serverpkg.NewSkillHandler(skill.NewRegistry())
			defer handler.Close()
			provider := &recordingProvider{name: "configured", models: []string{"configured-model"}}
			runtime := newRuntimeLLMProviderRef()
			runtime.SetProvider(provider)
			dataDir := t.TempDir()
			bindRouteRuntimeSkillMarketplace(handler, skillstore.NewLocalSkillScanner(dataDir), routeRuntimeContractSkillOptions{
				dataDir: dataDir, appConfig: cfg, services: &Services{SkillRegistry: skill.NewRegistry()}, runtimeLLM: runtime,
			}, context.Background())
			handler.SetSkillSelector(multilingualInstalledSelector{})
			body := `{"query":"Trouvez une compétence pour lire les documents"}`
			req := httptest.NewRequest(http.MethodPost, "/skills/advise", strings.NewReader(body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			if err := handler.MarketAdviseSkills(echo.New().NewContext(req, rec)); err != nil {
				t.Fatal(err)
			}
			messages, _ := json.Marshal(provider.lastReq.Messages)
			if rec.Code != 200 || provider.calls != 1 || !strings.Contains(string(messages), tc.target) || len(provider.lastReq.Tools) != 0 || provider.lastReq.MaxTokens != 512 {
				t.Fatalf("configured embedding target was not wired: calls=%d req=%+v", provider.calls, provider.lastReq)
			}
		})
	}
}
