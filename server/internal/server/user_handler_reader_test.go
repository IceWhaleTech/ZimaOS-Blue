package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	z "github.com/IceWhaleTech/zorm"
	"github.com/labstack/echo/v4"
	_ "github.com/mattn/go-sqlite3"
)

func TestUserProviderHandler_UsesReaderDBForReads(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "user-provider.db")

	writeDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open(write): %v", err)
	}

	registry := llm.NewProviderRegistry()
	registry.Register(&requestCaptureProvider{})

	bootstrapHandler, err := NewUserProviderHandler(writeDB, registry)
	if err != nil {
		_ = writeDB.Close()
		t.Fatalf("NewUserProviderHandler(bootstrap): %v", err)
	}

	_, err = bootstrapHandler.writeTable(context.Background()).Insert(map[string]interface{}{
		"id":            "provider-reader",
		"user_id":       "user-1",
		"provider_name": "capture",
		"api_key":       "sk-reader-secret",
		"base_url":      "https://reader.example.com",
		"enabled":       1,
		"created_at":    time.Now(),
		"updated_at":    time.Now(),
	})
	if err != nil {
		_ = writeDB.Close()
		t.Fatalf("seed provider config: %v", err)
	}

	readDB, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro")
	if err != nil {
		_ = writeDB.Close()
		t.Fatalf("sql.Open(read): %v", err)
	}
	defer readDB.Close()

	handler, err := NewUserProviderHandlerWithReadDB(writeDB, readDB, registry)
	if err != nil {
		_ = writeDB.Close()
		t.Fatalf("NewUserProviderHandlerWithReadDB: %v", err)
	}
	if handler.readDB == nil || handler.readDB == handler.db {
		_ = writeDB.Close()
		t.Fatal("expected separate reader db")
	}

	if err := writeDB.Close(); err != nil {
		t.Fatalf("close writer db: %v", err)
	}

	listCtx, listRec := newAuthenticatedHandlerContext(http.MethodGet, "/api/v1/user/providers", "", "user-1")
	if got := getUserIDStrict(listCtx); got != "user-1" {
		t.Fatalf("getUserIDStrict() = %q, want user-1", got)
	}
	var providerCount int
	if err := readDB.QueryRow("SELECT COUNT(*) FROM user_provider_configs WHERE user_id = ? AND provider_name = ?", "user-1", "capture").Scan(&providerCount); err != nil {
		t.Fatalf("direct reader query failed: %v", err)
	}
	if providerCount != 1 {
		t.Fatalf("direct reader query count = %d, want 1", providerCount)
	}
	var providerRows []userProviderConfigRow
	_, err = handler.readTable(listCtx.Request().Context()).Select(
		&providerRows,
		z.Fields("id", "provider_name", "api_key", "base_url", "enabled"),
		z.Where(z.Eq("user_id", "user-1")),
	)
	if err != nil {
		t.Fatalf("direct zorm reader select failed: %v", err)
	}
	if len(providerRows) != 1 {
		t.Fatalf("direct zorm reader select len = %d, want 1", len(providerRows))
	}
	if err := handler.List(listCtx); err != nil {
		t.Fatalf("List via reader db: %v", err)
	}
	if listRec.Code != http.StatusOK {
		t.Fatalf("List status=%d body=%s", listRec.Code, listRec.Body.String())
	}

	var listResp []UserProviderConfigResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("decode list response: %v body=%s", err, listRec.Body.String())
	}
	if len(listResp) != 1 {
		t.Fatalf("List response len=%d, want 1 body=%s", len(listResp), listRec.Body.String())
	}
	if listResp[0].ProviderName != "capture" || !listResp[0].Configured || !listResp[0].HasAPIKey || !listResp[0].Enabled {
		t.Fatalf("unexpected List response via reader db: %+v", listResp[0])
	}
	if listResp[0].APIKey == "" || listResp[0].APIKey == "sk-reader-secret" {
		t.Fatalf("expected masked api key in List response, got %q", listResp[0].APIKey)
	}

	getCtx, getRec := newAuthenticatedHandlerContext(http.MethodGet, "/api/v1/user/providers/capture", "", "user-1")
	getCtx.SetParamNames("name")
	getCtx.SetParamValues("capture")
	if err := handler.Get(getCtx); err != nil {
		t.Fatalf("Get via reader db: %v", err)
	}
	if getRec.Code != http.StatusOK {
		t.Fatalf("Get status=%d body=%s", getRec.Code, getRec.Body.String())
	}

	var getResp UserProviderConfigResponse
	if err := json.Unmarshal(getRec.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("decode get response: %v body=%s", err, getRec.Body.String())
	}
	if getResp.ProviderName != "capture" || getResp.BaseURL != "https://reader.example.com" || !getResp.Configured {
		t.Fatalf("unexpected Get response via reader db: %+v", getResp)
	}
	if getResp.APIKey == "" || getResp.APIKey == "sk-reader-secret" {
		t.Fatalf("expected masked api key in Get response, got %q", getResp.APIKey)
	}

	keyCtx, _ := newAuthenticatedHandlerContext(http.MethodGet, "/api/v1/user/providers/capture", "", "user-1")
	apiKey, baseURL, found := handler.GetUserProviderKey(keyCtx, "user-1", "capture")
	if !found || apiKey != "sk-reader-secret" || baseURL != "https://reader.example.com" {
		t.Fatalf("GetUserProviderKey via reader db = (%q, %q, %v), want seeded values", apiKey, baseURL, found)
	}
}

func TestUserSkillHandler_UsesReaderDBForReads(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "user-skill.db")

	writeDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open(write): %v", err)
	}

	skillsDir := filepath.Join(t.TempDir(), "skills")
	skillDir := filepath.Join(skillsDir, "reader-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		_ = writeDB.Close()
		t.Fatalf("MkdirAll(skill): %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(strings.Join([]string{
		"---",
		"name: Reader Skill",
		"---",
		"# Reader Skill",
		"",
	}, "\n")), 0o644); err != nil {
		_ = writeDB.Close()
		t.Fatalf("WriteFile(SKILL.md): %v", err)
	}

	bootstrapHandler, err := NewUserSkillHandler(writeDB, skillsDir)
	if err != nil {
		_ = writeDB.Close()
		t.Fatalf("NewUserSkillHandler(bootstrap): %v", err)
	}

	_, err = bootstrapHandler.writeTable(context.Background()).Insert(map[string]interface{}{
		"id":         "skill-reader",
		"user_id":    "user-1",
		"skill_id":   "reader-skill",
		"enabled":    0,
		"created_at": time.Now(),
		"updated_at": time.Now(),
	})
	if err != nil {
		_ = writeDB.Close()
		t.Fatalf("seed skill config: %v", err)
	}

	readDB, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro")
	if err != nil {
		_ = writeDB.Close()
		t.Fatalf("sql.Open(read): %v", err)
	}
	defer readDB.Close()

	handler, err := NewUserSkillHandlerWithReadDB(writeDB, readDB, skillsDir)
	if err != nil {
		_ = writeDB.Close()
		t.Fatalf("NewUserSkillHandlerWithReadDB: %v", err)
	}
	if handler.readDB == nil || handler.readDB == handler.db {
		_ = writeDB.Close()
		t.Fatal("expected separate reader db")
	}

	if err := writeDB.Close(); err != nil {
		t.Fatalf("close writer db: %v", err)
	}

	listCtx, listRec := newAuthenticatedHandlerContext(http.MethodGet, "/api/v1/user/skills", "", "user-1")
	if got := getUserIDFromContext(listCtx); got != "user-1" {
		t.Fatalf("getUserIDFromContext() = %q, want user-1", got)
	}
	var skillCount int
	if err := readDB.QueryRow("SELECT COUNT(*) FROM user_skill_configs WHERE user_id = ? AND skill_id = ?", "user-1", "reader-skill").Scan(&skillCount); err != nil {
		t.Fatalf("direct reader query failed: %v", err)
	}
	if skillCount != 1 {
		t.Fatalf("direct reader query count = %d, want 1", skillCount)
	}
	var skillRows []userSkillConfigRow
	_, err = handler.readTable(listCtx.Request().Context()).Select(
		&skillRows,
		z.Fields("skill_id", "enabled"),
		z.Where(z.Eq("user_id", "user-1")),
	)
	if err != nil {
		t.Fatalf("direct zorm reader select failed: %v", err)
	}
	if len(skillRows) != 1 {
		t.Fatalf("direct zorm reader select len = %d, want 1", len(skillRows))
	}
	if err := handler.List(listCtx); err != nil {
		t.Fatalf("List via reader db: %v", err)
	}
	if listRec.Code != http.StatusOK {
		t.Fatalf("List status=%d body=%s", listRec.Code, listRec.Body.String())
	}

	var listResp []UserSkillResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("decode list response: %v body=%s", err, listRec.Body.String())
	}
	if len(listResp) != 1 {
		t.Fatalf("List response len=%d, want 1 body=%s", len(listResp), listRec.Body.String())
	}
	if listResp[0].ID != "reader_skill" || listResp[0].Name != "Reader Skill" {
		t.Fatalf("unexpected skill identity via reader db: %+v", listResp[0])
	}
	if listResp[0].Enabled || !listResp[0].Installed || !listResp[0].UserToggled {
		t.Fatalf("unexpected skill state via reader db: %+v", listResp[0])
	}
}

func TestUserSkillHandler_ListCanonicalizesAliasDirAndLegacyConfig(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "user-skill-alias.db")

	writeDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open(write): %v", err)
	}
	defer writeDB.Close()

	skillsDir := filepath.Join(t.TempDir(), "skills")
	skillDir := filepath.Join(skillsDir, "team-browser")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(skill): %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(strings.Join([]string{
		"---",
		"name: browser",
		"description: Browser skill",
		"version: 1.0.0",
		"invocation: blue browser",
		"examples:",
		"  - blue browser",
		"capability_tags:",
		"  - browser",
		"interaction_mode: stateless",
		"card_support: none",
		"---",
		"# Browser",
		"",
	}, "\n")), 0o644); err != nil {
		t.Fatalf("WriteFile(SKILL.md): %v", err)
	}

	handler, err := NewUserSkillHandler(writeDB, skillsDir)
	if err != nil {
		t.Fatalf("NewUserSkillHandler: %v", err)
	}

	_, err = handler.writeTable(context.Background()).Insert(map[string]interface{}{
		"id":         "skill-alias",
		"user_id":    "user-1",
		"skill_id":   "team-browser",
		"enabled":    0,
		"created_at": time.Now(),
		"updated_at": time.Now(),
	})
	if err != nil {
		t.Fatalf("seed skill config: %v", err)
	}

	listCtx, listRec := newAuthenticatedHandlerContext(http.MethodGet, "/api/v1/user/skills", "", "user-1")
	if err := handler.List(listCtx); err != nil {
		t.Fatalf("List: %v", err)
	}
	if listRec.Code != http.StatusOK {
		t.Fatalf("List status=%d body=%s", listRec.Code, listRec.Body.String())
	}

	var listResp []UserSkillResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("decode list response: %v body=%s", err, listRec.Body.String())
	}
	if len(listResp) != 1 {
		t.Fatalf("List response len=%d, want 1 body=%s", len(listResp), listRec.Body.String())
	}
	if listResp[0].ID != "browser" || listResp[0].Name != "browser" {
		t.Fatalf("unexpected canonicalized skill identity: %+v", listResp[0])
	}
	if listResp[0].Enabled || !listResp[0].Installed || !listResp[0].UserToggled {
		t.Fatalf("unexpected skill state via alias config: %+v", listResp[0])
	}
}

func TestUserSkillHandler_ListKeepsMgmtSkillAsDistinctSkill(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "user-skill-mgmt.db")

	writeDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open(write): %v", err)
	}
	defer writeDB.Close()

	skillsDir := filepath.Join(t.TempDir(), "skills")
	skillDir := filepath.Join(skillsDir, "mgmt")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(skill): %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(strings.Join([]string{
		"---",
		"name: mgmt",
		"description: Legacy management skill",
		"version: 1.0.0",
		"invocation: blue mgmt.providers.list",
		"examples:",
		"  - blue mgmt.providers.list",
		"capability_tags:",
		"  - admin",
		"interaction_mode: stateless",
		"card_support: none",
		"---",
		"# Mgmt Skill",
		"",
	}, "\n")), 0o644); err != nil {
		t.Fatalf("WriteFile(SKILL.md): %v", err)
	}

	handler, err := NewUserSkillHandler(writeDB, skillsDir)
	if err != nil {
		t.Fatalf("NewUserSkillHandler: %v", err)
	}

	_, err = handler.writeTable(context.Background()).Insert(map[string]interface{}{
		"id":         "skill-mgmt",
		"user_id":    "user-1",
		"skill_id":   "mgmt",
		"enabled":    0,
		"created_at": time.Now(),
		"updated_at": time.Now(),
	})
	if err != nil {
		t.Fatalf("seed skill config: %v", err)
	}

	listCtx, listRec := newAuthenticatedHandlerContext(http.MethodGet, "/api/v1/user/skills", "", "user-1")
	if err := handler.List(listCtx); err != nil {
		t.Fatalf("List: %v", err)
	}
	if listRec.Code != http.StatusOK {
		t.Fatalf("List status=%d body=%s", listRec.Code, listRec.Body.String())
	}

	var listResp []UserSkillResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("decode list response: %v body=%s", err, listRec.Body.String())
	}
	if len(listResp) != 1 {
		t.Fatalf("List response len=%d, want 1 body=%s", len(listResp), listRec.Body.String())
	}
	if listResp[0].ID != "mgmt" || listResp[0].Name != "mgmt" {
		t.Fatalf("unexpected mgmt skill identity after alias removal: %+v", listResp[0])
	}
	if listResp[0].Enabled || !listResp[0].Installed || !listResp[0].UserToggled {
		t.Fatalf("unexpected mgmt skill state after alias removal: %+v", listResp[0])
	}
}

func TestUserSkillHandler_ToggleCanonicalizesInstalledAliasIDs(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "user-skill-toggle.db")

	writeDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open(write): %v", err)
	}
	defer writeDB.Close()

	skillsDir := filepath.Join(t.TempDir(), "skills")

	browserDir := filepath.Join(skillsDir, "team-browser")
	if err := os.MkdirAll(browserDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(browser skill): %v", err)
	}
	if err := os.WriteFile(filepath.Join(browserDir, "SKILL.md"), []byte(strings.Join([]string{
		"---",
		"name: browser",
		"description: Browser skill",
		"version: 1.0.0",
		"invocation: blue browser",
		"examples:",
		"  - blue browser",
		"capability_tags:",
		"  - browser",
		"interaction_mode: stateless",
		"card_support: none",
		"---",
		"# Browser",
		"",
	}, "\n")), 0o644); err != nil {
		t.Fatalf("WriteFile(browser SKILL.md): %v", err)
	}

	wordDir := filepath.Join(skillsDir, "word_docx")
	if err := os.MkdirAll(wordDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(word skill): %v", err)
	}
	if err := os.WriteFile(filepath.Join(wordDir, "SKILL.md"), []byte(strings.Join([]string{
		"---",
		"id: word_docx",
		"name: Word DOCX",
		"description: Word skill",
		"version: 1.0.0",
		"invocation: blue word_docx",
		"examples:",
		"  - blue word_docx",
		"capability_tags:",
		"  - word",
		"interaction_mode: stateless",
		"card_support: none",
		"---",
		"# Word DOCX",
		"",
	}, "\n")), 0o644); err != nil {
		t.Fatalf("WriteFile(word SKILL.md): %v", err)
	}

	handler, err := NewUserSkillHandler(writeDB, skillsDir)
	if err != nil {
		t.Fatalf("NewUserSkillHandler: %v", err)
	}

	toggleCtx, toggleRec := newAuthenticatedHandlerContext(http.MethodPut, "/api/v1/user/skills/team-browser", `{"enabled":false}`, "user-1")
	toggleCtx.Request().Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	toggleCtx.SetParamNames("id")
	toggleCtx.SetParamValues("team-browser")
	if err := handler.Toggle(toggleCtx); err != nil {
		t.Fatalf("Toggle(browser): %v", err)
	}
	if toggleRec.Code != http.StatusOK {
		t.Fatalf("Toggle(browser) status=%d body=%s", toggleRec.Code, toggleRec.Body.String())
	}

	var browserResp map[string]interface{}
	if err := json.Unmarshal(toggleRec.Body.Bytes(), &browserResp); err != nil {
		t.Fatalf("decode browser toggle response: %v body=%s", err, toggleRec.Body.String())
	}
	if browserResp["skill_id"] != "browser" {
		t.Fatalf("browser toggle skill_id=%#v, want browser", browserResp["skill_id"])
	}

	hyphenCtx, hyphenRec := newAuthenticatedHandlerContext(http.MethodPut, "/api/v1/user/skills/word-docx", `{"enabled":true}`, "user-1")
	hyphenCtx.Request().Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	hyphenCtx.SetParamNames("id")
	hyphenCtx.SetParamValues("word-docx")
	if err := handler.Toggle(hyphenCtx); err != nil {
		t.Fatalf("Toggle(word-docx): %v", err)
	}
	if hyphenRec.Code != http.StatusOK {
		t.Fatalf("Toggle(word-docx) status=%d body=%s", hyphenRec.Code, hyphenRec.Body.String())
	}

	var hyphenResp map[string]interface{}
	if err := json.Unmarshal(hyphenRec.Body.Bytes(), &hyphenResp); err != nil {
		t.Fatalf("decode hyphen toggle response: %v body=%s", err, hyphenRec.Body.String())
	}
	if hyphenResp["skill_id"] != "word_docx" {
		t.Fatalf("word toggle skill_id=%#v, want word_docx", hyphenResp["skill_id"])
	}

	rows, err := writeDB.Query(`SELECT skill_id, enabled FROM user_skill_configs WHERE user_id = ? ORDER BY skill_id`, "user-1")
	if err != nil {
		t.Fatalf("query user skill configs: %v", err)
	}
	defer rows.Close()

	got := make(map[string]bool)
	for rows.Next() {
		var skillID string
		var enabled bool
		if err := rows.Scan(&skillID, &enabled); err != nil {
			t.Fatalf("scan user skill config: %v", err)
		}
		got[skillID] = enabled
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows err: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("config rows=%#v, want browser + word_docx", got)
	}
	if enabled, ok := got["browser"]; !ok || enabled {
		t.Fatalf("browser config=%#v, want false", got["browser"])
	}
	if enabled, ok := got["word_docx"]; !ok || !enabled {
		t.Fatalf("word_docx config=%#v, want true", got["word_docx"])
	}
	if _, ok := got["team-browser"]; ok {
		t.Fatalf("unexpected legacy alias row persisted: %#v", got)
	}
	if _, ok := got["word-docx"]; ok {
		t.Fatalf("unexpected hyphen alias row persisted: %#v", got)
	}
}

func TestUserSkillHandler_ToggleReturnsConflictForAmbiguousInstalledAlias(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "user-skill-conflict.db")

	writeDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open(write): %v", err)
	}
	defer writeDB.Close()

	skillsDir := filepath.Join(t.TempDir(), "skills")

	firstDir := filepath.Join(skillsDir, "team-browser")
	if err := os.MkdirAll(firstDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(first skill): %v", err)
	}
	if err := os.WriteFile(filepath.Join(firstDir, "SKILL.md"), []byte(strings.Join([]string{
		"---",
		"name: browser",
		"description: Browser alias",
		"version: 1.0.0",
		"invocation: blue browser",
		"examples:",
		"  - blue browser",
		"capability_tags:",
		"  - browser",
		"interaction_mode: stateless",
		"card_support: none",
		"---",
		"# Browser",
		"",
	}, "\n")), 0o644); err != nil {
		t.Fatalf("WriteFile(first SKILL.md): %v", err)
	}

	secondDir := filepath.Join(skillsDir, "browser")
	if err := os.MkdirAll(secondDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(second skill): %v", err)
	}
	if err := os.WriteFile(filepath.Join(secondDir, "SKILL.md"), []byte(strings.Join([]string{
		"---",
		"name: browser",
		"description: Browser duplicate",
		"version: 1.0.0",
		"invocation: blue browser",
		"examples:",
		"  - blue browser",
		"capability_tags:",
		"  - browser",
		"interaction_mode: stateless",
		"card_support: none",
		"---",
		"# Browser",
		"",
	}, "\n")), 0o644); err != nil {
		t.Fatalf("WriteFile(second SKILL.md): %v", err)
	}

	handler, err := NewUserSkillHandler(writeDB, skillsDir)
	if err != nil {
		t.Fatalf("NewUserSkillHandler: %v", err)
	}

	toggleCtx, toggleRec := newAuthenticatedHandlerContext(http.MethodPut, "/api/v1/user/skills/browser", `{"enabled":false}`, "user-1")
	toggleCtx.Request().Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	toggleCtx.SetParamNames("id")
	toggleCtx.SetParamValues("browser")

	err = handler.Toggle(toggleCtx)
	if err == nil {
		t.Fatalf("expected Toggle(browser) to fail on canonical conflict")
	}
	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected echo.HTTPError, got %T (%v)", err, err)
	}
	if httpErr.Code != http.StatusConflict {
		t.Fatalf("HTTP status=%d, want %d", httpErr.Code, http.StatusConflict)
	}
	if !strings.Contains(httpErr.Error(), "canonical skill conflicts detected") {
		t.Fatalf("expected canonical conflict error, got %v", err)
	}
	if toggleRec.Code != http.StatusOK {
		t.Fatalf("unexpected recorder status=%d body=%s", toggleRec.Code, toggleRec.Body.String())
	}

	var configCount int
	if err := writeDB.QueryRow(`SELECT COUNT(*) FROM user_skill_configs WHERE user_id = ?`, "user-1").Scan(&configCount); err != nil {
		t.Fatalf("count user skill configs: %v", err)
	}
	if configCount != 0 {
		t.Fatalf("configCount=%d, want 0", configCount)
	}
}

func newAuthenticatedHandlerContext(method, target, body, userID string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{
		UserID:   userID,
		Username: userID,
		Role:     "user",
	}))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user", &auth.Claims{
		UserClaims: auth.UserClaims{
			UserID:   userID,
			Username: userID,
			Role:     "user",
		},
	})
	return c, rec
}
