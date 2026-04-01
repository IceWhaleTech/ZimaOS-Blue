package bootstrap

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentsessions"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	skillpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestNewRuntimeAskSupportBundle_WiresChatSupportAndSeedsTrustedSites(t *testing.T) {
	tmp := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tmp, "ask-support.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{}
	cfg.Browser.TrustedSites = []string{"https://example.com/account"}

	observer := &stubRuntimeObserver{}
	target := &stubChatAskRuntimeTarget{}
	broker := sse.NewBroker()
	defer broker.Close()

	bundle := newRuntimeAskSupportBundle(runtimeAskSupportOptions{
		writeDB:        db,
		readDB:         db,
		appConfig:      cfg,
		toolRegistry:   tools.NewRegistry(),
		skillRegistry:  skillpkg.NewRegistry(),
		broker:         broker,
		harnessRuntime: &HarnessRuntimeBundle{RuntimeObserver: observer},
		chatTarget:     target,
		mediaDir:       " /tmp/media ",
		timeout:        time.Minute,
	})

	if bundle.QuestionManager == nil || bundle.BrowserCheckpointManager == nil || bundle.BrowserSiteStore == nil {
		t.Fatalf("expected ask support bundle to initialize dependencies, got %#v", bundle)
	}
	if target.mediaDir != "/tmp/media" || target.questionMgr != bundle.QuestionManager || target.browserCheckpointMgr != bundle.BrowserCheckpointManager || target.browserSiteStore != bundle.BrowserSiteStore {
		t.Fatalf("expected chat ask support binding, got %#v", target)
	}
	if target.toolObserver != observer || target.toolObserverCalls != 1 {
		t.Fatalf("expected runtime observer binding, got %#v", target)
	}
	if entry := bundle.BrowserSiteStore.Match("https://example.com/settings", ""); entry == nil || entry.Origin != "https://example.com" {
		t.Fatalf("expected trusted site seed to be normalized into allowlist, got %+v", entry)
	}
}

func TestRuntimeSupportBundles_RegisterRoutesExposeGuardAndSessionSurfaces(t *testing.T) {
	tmp := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tmp, "runtime-support.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	memStore, err := memory.NewStoreWithDB(db)
	if err != nil {
		t.Fatalf("memory.NewStoreWithDB: %v", err)
	}

	e := echo.New()
	v1 := e.Group("/api/v1")
	protected := e.Group("/api")
	profileRoutes := e.Group("/api/profile")
	sessionRoutes := e.Group("/api/chat")

	askBroker := sse.NewBroker()
	defer askBroker.Close()
	askSupport := newRuntimeAskSupportBundle(runtimeAskSupportOptions{
		writeDB:       db,
		readDB:        db,
		toolRegistry:  tools.NewRegistry(),
		skillRegistry: skillpkg.NewRegistry(),
		broker:        askBroker,
		timeout:       time.Minute,
	})

	execBroker := sse.NewBroker()
	defer execBroker.Close()
	execClosers := make([]interface{ Close() error }, 0, 1)
	execRegistry := tools.NewRegistry()
	execSupport := newRuntimeExecSupportBundle(runtimeExecSupportOptions{
		writeDB:              db,
		readDB:               db,
		dataDir:              tmp,
		workspaceAllowedPath: []string{tmp},
		memoryStore:          memStore,
		toolRegistry:         execRegistry,
		skillRegistry:        skillpkg.NewRegistry(),
		broker:               execBroker,
		logger:               zap.NewNop(),
		closers:              &execClosers,
		profileRoutes:        profileRoutes,
		sessionRoutes:        sessionRoutes,
		oauthSource: func() agentsessions.OAuthCredentialSource {
			return nil
		},
		lookupAPIKey: func(string) (string, error) {
			return "", nil
		},
	})

	if execSupport.Approvals == nil || execSupport.DirStore == nil || execSupport.AuditStore == nil || execSupport.ConvertHandler == nil {
		t.Fatalf("expected exec support bundle to initialize stores and handlers, got %#v", execSupport)
	}
	if tools.GetExecTool(execRegistry) == nil {
		t.Fatalf("expected exec tool registration, got %v", execRegistry.List())
	}
	if len(execClosers) != 1 {
		t.Fatalf("expected convert service closer to be captured, got %d", len(execClosers))
	}

	registerRuntimeAskSupportRoutes(v1, nil, nil, askSupport)
	registerRuntimeExecSupportRoutes(v1, nil, nil, execSupport)
	if !execSupport.registerConvertRoutes(protected, nil) {
		t.Fatal("expected convert routes to register")
	}

	if !routeExists(e, "GET", "/api/v1/browser/approvals/sites") || !routeExists(e, "GET", "/api/v1/ask-user-question/pending") {
		t.Fatalf("expected ask/browser support routes, got %#v", e.Routes())
	}
	if !routeExists(e, "GET", "/api/v1/exec/approvals/pending") || !routeExists(e, "DELETE", "/api/v1/exec/approvals/directories/:id") {
		t.Fatalf("expected exec approval routes, got %#v", e.Routes())
	}
	if !routeExists(e, "GET", "/api/convert/capabilities") {
		t.Fatalf("expected convert routes, got %#v", e.Routes())
	}
	if !routeExists(e, "GET", "/api/profile/agent-sessions/profiles") || !routeExists(e, "GET", "/api/chat/agent-sessions/sessions") {
		t.Fatalf("expected agent session routes from exec support bundle, got %#v", e.Routes())
	}
}

func TestRuntimeSupportBundles_UseReaderDBForBundleStores(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "runtime-support-reader.db")

	writeDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open sqlite writer: %v", err)
	}

	if _, err := tools.NewBrowserSiteAllowlistStore(writeDB); err != nil {
		_ = writeDB.Close()
		t.Fatalf("bootstrap browser site store: %v", err)
	}
	if _, err := tools.NewDirAllowlistStore(writeDB); err != nil {
		_ = writeDB.Close()
		t.Fatalf("bootstrap dir allowlist store: %v", err)
	}
	if _, err := tools.NewExecAuditStore(writeDB); err != nil {
		_ = writeDB.Close()
		t.Fatalf("bootstrap exec audit store: %v", err)
	}

	readDB, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro")
	if err != nil {
		_ = writeDB.Close()
		t.Fatalf("open sqlite reader: %v", err)
	}
	defer readDB.Close()

	cfg := &config.Config{}
	cfg.Browser.TrustedSites = []string{"https://example.com/account"}

	askBroker := sse.NewBroker()
	defer askBroker.Close()
	askBundle := newRuntimeAskSupportBundle(runtimeAskSupportOptions{
		writeDB:       writeDB,
		readDB:        readDB,
		appConfig:     cfg,
		toolRegistry:  tools.NewRegistry(),
		skillRegistry: skillpkg.NewRegistry(),
		broker:        askBroker,
		timeout:       time.Minute,
	})
	if askBundle.BrowserSiteStore == nil {
		_ = writeDB.Close()
		t.Fatal("expected browser site store")
	}
	if askBundle.BrowserSiteStore.Match("https://example.com/settings", "") == nil {
		_ = writeDB.Close()
		t.Fatal("expected trusted site seed to be present")
	}

	execBroker := sse.NewBroker()
	defer execBroker.Close()
	execClosers := make([]interface{ Close() error }, 0, 1)
	execBundle := newRuntimeExecSupportBundle(runtimeExecSupportOptions{
		writeDB:              writeDB,
		readDB:               readDB,
		dataDir:              tmp,
		workspaceAllowedPath: []string{tmp},
		toolRegistry:         tools.NewRegistry(),
		skillRegistry:        skillpkg.NewRegistry(),
		broker:               execBroker,
		logger:               zap.NewNop(),
		closers:              &execClosers,
		oauthSource: func() agentsessions.OAuthCredentialSource {
			return nil
		},
		lookupAPIKey: func(string) (string, error) {
			return "", nil
		},
	})
	if execBundle.DirStore == nil || execBundle.AuditStore == nil {
		_ = writeDB.Close()
		t.Fatalf("expected exec bundle stores, got %#v", execBundle)
	}
	if err := execBundle.DirStore.Add(tmp, "user-a"); err != nil {
		_ = writeDB.Close()
		t.Fatalf("DirStore.Add: %v", err)
	}
	exitCode := 0
	if err := execBundle.AuditStore.Record(tools.ExecAuditEntry{
		ID:         "bundle-audit",
		Timestamp:  time.Now(),
		UserID:     "user-a",
		Command:    "echo bundle",
		PolicyMode: "full",
		Decision:   "allowed",
		RiskLevel:  tools.RiskLevelLow,
		ExitCode:   &exitCode,
	}); err != nil {
		_ = writeDB.Close()
		t.Fatalf("AuditStore.Record: %v", err)
	}

	if err := writeDB.Close(); err != nil {
		t.Fatalf("close writer db: %v", err)
	}

	if got := askBundle.BrowserSiteStore.Match("https://example.com/settings", ""); got == nil || got.Origin != "https://example.com" {
		t.Fatalf("unexpected trusted site via reader bundle store: %+v", got)
	}
	if got := execBundle.DirStore.Match(filepath.Join(tmp, "subdir")); got == nil || got.Path != tmp {
		t.Fatalf("unexpected dir allowlist via reader bundle store: %+v", got)
	}
	entries, err := execBundle.AuditStore.Recent(10)
	if err != nil {
		t.Fatalf("AuditStore.Recent via reader bundle store: %v", err)
	}
	if len(entries) != 1 || entries[0].ID != "bundle-audit" {
		t.Fatalf("unexpected exec audit entries via reader bundle store: %+v", entries)
	}
}

func TestRuntimeSupportBundlesGo_DelegatesAskAndExecLanes(t *testing.T) {
	askContent, err := os.ReadFile(filepath.Join("runtime_support_bundles_ask.go"))
	if err != nil {
		t.Fatalf("read runtime_support_bundles_ask.go: %v", err)
	}
	askSource := string(askContent)

	execContent, err := os.ReadFile(filepath.Join("runtime_support_bundles_exec.go"))
	if err != nil {
		t.Fatalf("read runtime_support_bundles_exec.go: %v", err)
	}
	execSource := string(execContent)
	execTypesContent, err := os.ReadFile(filepath.Join("runtime_support_bundles_exec_types.go"))
	if err != nil {
		t.Fatalf("read runtime_support_bundles_exec_types.go: %v", err)
	}
	execTypesSource := string(execTypesContent)
	execRoutesContent, err := os.ReadFile(filepath.Join("runtime_support_bundles_exec_routes.go"))
	if err != nil {
		t.Fatalf("read runtime_support_bundles_exec_routes.go: %v", err)
	}
	execRoutesSource := string(execRoutesContent)
	execStoresContent, err := os.ReadFile(filepath.Join("runtime_support_bundles_exec_stores.go"))
	if err != nil {
		t.Fatalf("read runtime_support_bundles_exec_stores.go: %v", err)
	}
	execStoresSource := string(execStoresContent)
	execServicesContent, err := os.ReadFile(filepath.Join("runtime_support_bundles_exec_services.go"))
	if err != nil {
		t.Fatalf("read runtime_support_bundles_exec_services.go: %v", err)
	}
	execServicesSource := string(execServicesContent)
	execSandboxContent, err := os.ReadFile(filepath.Join("runtime_support_bundles_exec_sandbox.go"))
	if err != nil {
		t.Fatalf("read runtime_support_bundles_exec_sandbox.go: %v", err)
	}
	execSandboxSource := string(execSandboxContent)

	requiredAsk := []string{
		"type runtimeAskSupportBundle struct {",
		"type runtimeAskSupportOptions struct {",
		"func newRuntimeAskSupportBundle(",
		"func registerRuntimeAskSupportRoutes(",
		"func newRuntimeBrowserSiteStore(",
	}
	for _, token := range requiredAsk {
		if !strings.Contains(askSource, token) {
			t.Fatalf("expected runtime_support_bundles_ask.go to contain token %q", token)
		}
	}

	if lines := strings.Count(execSource, "\n") + 1; lines > 80 {
		t.Fatalf("expected runtime_support_bundles_exec.go to stay below 80 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(execTypesSource, "\n") + 1; lines > 55 {
		t.Fatalf("expected runtime_support_bundles_exec_types.go to stay below 55 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(execRoutesSource, "\n") + 1; lines > 35 {
		t.Fatalf("expected runtime_support_bundles_exec_routes.go to stay below 35 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(execStoresSource, "\n") + 1; lines > 55 {
		t.Fatalf("expected runtime_support_bundles_exec_stores.go to stay below 55 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(execServicesSource, "\n") + 1; lines > 85 {
		t.Fatalf("expected runtime_support_bundles_exec_services.go to stay below 85 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(execSandboxSource, "\n") + 1; lines > 20 {
		t.Fatalf("expected runtime_support_bundles_exec_sandbox.go to stay below 20 lines after extraction, got %d", lines)
	}

	requiredExecMain := []string{
		"func newRuntimeExecSupportBundle(",
		"newRuntimeExecDirStore(",
		"newRuntimeExecConvertSupport(",
		"newRuntimeExecAuditStore(",
		"newRuntimeExecAgentSessionsService(",
		"newRuntimeExecSandboxExecutor(",
	}
	for _, token := range requiredExecMain {
		if !strings.Contains(execSource, token) {
			t.Fatalf("expected runtime_support_bundles_exec.go to contain token %q", token)
		}
	}

	requiredExecTypes := []string{
		"type runtimeExecSupportBundle struct {",
		"type runtimeExecSupportOptions struct {",
	}
	for _, token := range requiredExecTypes {
		if !strings.Contains(execTypesSource, token) {
			t.Fatalf("expected runtime_support_bundles_exec_types.go to contain token %q", token)
		}
	}

	requiredExecRoutes := []string{
		"func registerRuntimeExecSupportRoutes(",
		"func (bundle runtimeExecSupportBundle) registerConvertRoutes(",
	}
	for _, token := range requiredExecRoutes {
		if !strings.Contains(execRoutesSource, token) {
			t.Fatalf("expected runtime_support_bundles_exec_routes.go to contain token %q", token)
		}
	}

	requiredExecStores := []string{
		"func newRuntimeExecDirStore(",
		"func newRuntimeExecAuditStore(",
	}
	for _, token := range requiredExecStores {
		if !strings.Contains(execStoresSource, token) {
			t.Fatalf("expected runtime_support_bundles_exec_stores.go to contain token %q", token)
		}
	}

	requiredExecServices := []string{
		"func newRuntimeExecConvertSupport(",
		"func newRuntimeExecAgentSessionsService(",
	}
	for _, token := range requiredExecServices {
		if !strings.Contains(execServicesSource, token) {
			t.Fatalf("expected runtime_support_bundles_exec_services.go to contain token %q", token)
		}
	}

	if !strings.Contains(execSandboxSource, "func newRuntimeExecSandboxExecutor(") {
		t.Fatal("expected runtime_support_bundles_exec_sandbox.go to contain newRuntimeExecSandboxExecutor")
	}

	forbiddenAsk := []string{
		"type runtimeExecSupportBundle struct {",
		"func newRuntimeExecSupportBundle(",
		"func newRuntimeExecAgentSessionsService(",
	}
	for _, token := range forbiddenAsk {
		if strings.Contains(askSource, token) {
			t.Fatalf("expected runtime_support_bundles_ask.go to delegate token %q", token)
		}
	}

	forbiddenExec := []string{
		"type runtimeAskSupportBundle struct {",
		"func newRuntimeAskSupportBundle(",
		"func newRuntimeBrowserSiteStore(",
		"type runtimeExecSupportBundle struct {",
		"func registerRuntimeExecSupportRoutes(",
		"func newRuntimeExecDirStore(",
		"func newRuntimeExecConvertSupport(",
	}
	for _, token := range forbiddenExec {
		if strings.Contains(execSource, token) {
			t.Fatalf("expected runtime_support_bundles_exec.go to delegate token %q", token)
		}
	}
}

func TestRuntimeExecSupportBundle_RegisterConvertRoutesRejectsNilHandler(t *testing.T) {
	e := echo.New()
	if (runtimeExecSupportBundle{}).registerConvertRoutes(e.Group("/api"), nil) {
		t.Fatal("expected nil convert handler to skip route registration")
	}
}

func TestNewRuntimeExecSandboxExecutor_ReturnsNilForNilManager(t *testing.T) {
	if newRuntimeExecSandboxExecutor(nil) != nil {
		t.Fatal("expected nil sandbox manager to disable sandbox executor")
	}
}
