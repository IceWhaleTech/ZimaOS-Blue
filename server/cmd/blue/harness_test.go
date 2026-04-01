package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	harnesspkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sockipc"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/user"
	"go.uber.org/zap"
)

func setupHarnessCLIServer(t *testing.T, handler http.Handler) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := &http.Server{Handler: handler}
	go func() {
		_ = server.Serve(listener)
	}()

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	})

	return listener.Addr().String()
}

func setupHarnessCLIIPCSocket(t *testing.T, register func(*sockipc.Server)) string {
	t.Helper()

	sockPath := filepath.Join(os.TempDir(), "blue-harness-"+strconv.FormatInt(time.Now().UnixNano(), 10)+".sock")
	_ = os.Remove(sockPath)
	srv := sockipc.NewServer(sockPath, zap.NewNop())
	if register != nil {
		register(srv)
	}
	if err := srv.Start(); err != nil {
		t.Fatalf("start sockipc server: %v", err)
	}
	t.Cleanup(func() {
		_ = srv.Close()
	})
	return sockPath
}

func resetHarnessCLIState(t *testing.T) {
	t.Helper()

	oldJSONOutput := jsonOutput
	oldVerbose := verbose
	oldNoColor := noColor
	oldDevMode := devMode
	oldCfgFile := cfgFile
	oldProfile := profile

	oldOwnerUserID := harnessOwnerUserID
	oldEvalSpecID := harnessSelectorEvalSpecID
	oldTitle := harnessSelectorTitle
	oldWait := harnessSelectorWait
	oldPollEvery := harnessSelectorPollEvery
	oldTimeout := harnessSelectorTimeout
	oldBaselineID := harnessSelectorBaselineID
	oldBaseEvalID := harnessSelectorBaseEvalID
	oldCandidateID := harnessSelectorCandidateID
	oldMinPassRate := harnessSelectorMinPassRate
	oldMinCriticalPassRate := harnessSelectorMinCriticalPassRate
	oldMinRouteAgreementRate := harnessSelectorMinRouteAgreementRate
	oldMaxClarifyRateDelta := harnessSelectorMaxClarifyRateDelta
	oldMaxCriticalRegressions := harnessSelectorMaxCriticalRegressions
	oldExecutionEvalSpecID := harnessExecutionEvalSpecID
	oldExecutionTitle := harnessExecutionTitle
	oldExecutionWait := harnessExecutionWait
	oldExecutionPollEvery := harnessExecutionPollEvery
	oldExecutionTimeout := harnessExecutionTimeout
	oldExecutionBaselineID := harnessExecutionBaselineID
	oldExecutionBaseEvalID := harnessExecutionBaseEvalID
	oldExecutionCandidateID := harnessExecutionCandidateID
	oldExecutionMaxPassRateDrop := harnessExecutionMaxPassRateDrop
	oldExecutionMaxCriticalRegressions := harnessExecutionMaxCriticalRegressions
	oldExecutionMaxVerificationPassRateDrop := harnessExecutionMaxVerificationPassRateDrop
	oldExecutionMaxEvidenceBackedRateDrop := harnessExecutionMaxEvidenceBackedRateDrop
	oldBudgetBaselineID := harnessBudgetBaselineID
	oldBudgetBaseEvalID := harnessBudgetBaseEvalID
	oldBudgetMinMedianSchemaByteReductionRate := harnessBudgetMinMedianSchemaByteReductionRate
	oldBudgetMaxMedianLatencyIncreaseRate := harnessBudgetMaxMedianLatencyIncreaseRate
	oldBudgetAllowedFinalNativeTools := harnessBudgetAllowedFinalNativeTools
	oldCutoverCandidateID := harnessCutoverCandidateID
	oldCutoverRequiredConsecutiveRuns := harnessCutoverRequiredConsecutiveRuns
	oldCutoverMaxAssessments := harnessCutoverMaxAssessments
	oldCutoverSelectorBaselineID := harnessCutoverSelectorBaselineID
	oldCutoverSelectorBaseEvalID := harnessCutoverSelectorBaseEvalID
	oldCutoverExecutionBaselineID := harnessCutoverExecutionBaselineID
	oldCutoverExecutionBaseEvalID := harnessCutoverExecutionBaseEvalID
	oldCutoverBudgetBaselineID := harnessCutoverBudgetBaselineID
	oldCutoverBudgetBaseEvalID := harnessCutoverBudgetBaseEvalID
	oldCutoverMinMedianSchemaByteReductionRate := harnessCutoverMinMedianSchemaByteReductionRate
	oldCutoverMaxMedianLatencyIncreaseRate := harnessCutoverMaxMedianLatencyIncreaseRate
	oldCutoverAllowedFinalNativeTools := harnessCutoverAllowedFinalNativeTools

	t.Cleanup(func() {
		jsonOutput = oldJSONOutput
		verbose = oldVerbose
		noColor = oldNoColor
		devMode = oldDevMode
		cfgFile = oldCfgFile
		profile = oldProfile

		harnessOwnerUserID = oldOwnerUserID
		harnessSelectorEvalSpecID = oldEvalSpecID
		harnessSelectorTitle = oldTitle
		harnessSelectorWait = oldWait
		harnessSelectorPollEvery = oldPollEvery
		harnessSelectorTimeout = oldTimeout
		harnessSelectorBaselineID = oldBaselineID
		harnessSelectorBaseEvalID = oldBaseEvalID
		harnessSelectorCandidateID = oldCandidateID
		harnessSelectorMinPassRate = oldMinPassRate
		harnessSelectorMinCriticalPassRate = oldMinCriticalPassRate
		harnessSelectorMinRouteAgreementRate = oldMinRouteAgreementRate
		harnessSelectorMaxClarifyRateDelta = oldMaxClarifyRateDelta
		harnessSelectorMaxCriticalRegressions = oldMaxCriticalRegressions
		harnessExecutionEvalSpecID = oldExecutionEvalSpecID
		harnessExecutionTitle = oldExecutionTitle
		harnessExecutionWait = oldExecutionWait
		harnessExecutionPollEvery = oldExecutionPollEvery
		harnessExecutionTimeout = oldExecutionTimeout
		harnessExecutionBaselineID = oldExecutionBaselineID
		harnessExecutionBaseEvalID = oldExecutionBaseEvalID
		harnessExecutionCandidateID = oldExecutionCandidateID
		harnessExecutionMaxPassRateDrop = oldExecutionMaxPassRateDrop
		harnessExecutionMaxCriticalRegressions = oldExecutionMaxCriticalRegressions
		harnessExecutionMaxVerificationPassRateDrop = oldExecutionMaxVerificationPassRateDrop
		harnessExecutionMaxEvidenceBackedRateDrop = oldExecutionMaxEvidenceBackedRateDrop
		harnessBudgetBaselineID = oldBudgetBaselineID
		harnessBudgetBaseEvalID = oldBudgetBaseEvalID
		harnessBudgetMinMedianSchemaByteReductionRate = oldBudgetMinMedianSchemaByteReductionRate
		harnessBudgetMaxMedianLatencyIncreaseRate = oldBudgetMaxMedianLatencyIncreaseRate
		harnessBudgetAllowedFinalNativeTools = oldBudgetAllowedFinalNativeTools
		harnessCutoverCandidateID = oldCutoverCandidateID
		harnessCutoverRequiredConsecutiveRuns = oldCutoverRequiredConsecutiveRuns
		harnessCutoverMaxAssessments = oldCutoverMaxAssessments
		harnessCutoverSelectorBaselineID = oldCutoverSelectorBaselineID
		harnessCutoverSelectorBaseEvalID = oldCutoverSelectorBaseEvalID
		harnessCutoverExecutionBaselineID = oldCutoverExecutionBaselineID
		harnessCutoverExecutionBaseEvalID = oldCutoverExecutionBaseEvalID
		harnessCutoverBudgetBaselineID = oldCutoverBudgetBaselineID
		harnessCutoverBudgetBaseEvalID = oldCutoverBudgetBaseEvalID
		harnessCutoverMinMedianSchemaByteReductionRate = oldCutoverMinMedianSchemaByteReductionRate
		harnessCutoverMaxMedianLatencyIncreaseRate = oldCutoverMaxMedianLatencyIncreaseRate
		harnessCutoverAllowedFinalNativeTools = oldCutoverAllowedFinalNativeTools
	})

	jsonOutput = true
	verbose = false
	noColor = true
	devMode = false
	cfgFile = ""
	profile = ""

	harnessOwnerUserID = ""
	harnessSelectorEvalSpecID = ""
	harnessSelectorTitle = ""
	harnessSelectorWait = true
	harnessSelectorPollEvery = time.Millisecond
	harnessSelectorTimeout = time.Second
	harnessSelectorBaselineID = ""
	harnessSelectorBaseEvalID = ""
	harnessSelectorCandidateID = ""
	harnessSelectorMinPassRate = ""
	harnessSelectorMinCriticalPassRate = ""
	harnessSelectorMinRouteAgreementRate = ""
	harnessSelectorMaxClarifyRateDelta = ""
	harnessSelectorMaxCriticalRegressions = ""
	harnessExecutionEvalSpecID = ""
	harnessExecutionTitle = ""
	harnessExecutionWait = true
	harnessExecutionPollEvery = time.Millisecond
	harnessExecutionTimeout = time.Second
	harnessExecutionBaselineID = ""
	harnessExecutionBaseEvalID = ""
	harnessExecutionCandidateID = ""
	harnessExecutionMaxPassRateDrop = ""
	harnessExecutionMaxCriticalRegressions = ""
	harnessExecutionMaxVerificationPassRateDrop = ""
	harnessExecutionMaxEvidenceBackedRateDrop = ""
	harnessBudgetBaselineID = ""
	harnessBudgetBaseEvalID = ""
	harnessBudgetMinMedianSchemaByteReductionRate = ""
	harnessBudgetMaxMedianLatencyIncreaseRate = ""
	harnessBudgetAllowedFinalNativeTools = ""
	harnessCutoverCandidateID = ""
	harnessCutoverRequiredConsecutiveRuns = 2
	harnessCutoverMaxAssessments = 5
	harnessCutoverSelectorBaselineID = ""
	harnessCutoverSelectorBaseEvalID = ""
	harnessCutoverExecutionBaselineID = ""
	harnessCutoverExecutionBaseEvalID = ""
	harnessCutoverBudgetBaselineID = ""
	harnessCutoverBudgetBaseEvalID = ""
	harnessCutoverMinMedianSchemaByteReductionRate = ""
	harnessCutoverMaxMedianLatencyIncreaseRate = ""
	harnessCutoverAllowedFinalNativeTools = ""
}

func writeHarnessCLITestConfig(t *testing.T, jwtSecret string) string {
	t.Helper()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	content := "security:\n" +
		"  jwt:\n" +
		"    secret: " + jwtSecret + "\n" +
		"    expiration: 24h\n" +
		"    refresh_expiration: 720h\n" +
		"    issuer: zimaos-blue\n"
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return configPath
}

func TestResolveHarnessOwnerUserIDUsesEnvAndFallback(t *testing.T) {
	t.Setenv("BLUE_USER_ID", "env-owner")

	if got := resolveHarnessOwnerUserID(""); got != "env-owner" {
		t.Fatalf("resolveHarnessOwnerUserID(\"\") = %q, want %q", got, "env-owner")
	}
	if got := resolveHarnessOwnerUserID("explicit-owner"); got != "explicit-owner" {
		t.Fatalf("resolveHarnessOwnerUserID(explicit) = %q, want %q", got, "explicit-owner")
	}

	t.Setenv("BLUE_USER_ID", "")
	if got := resolveHarnessOwnerUserID(""); got != "local-cli" {
		t.Fatalf("resolveHarnessOwnerUserID fallback = %q, want %q", got, "local-cli")
	}
}

func TestHarnessBudgetCommandExposesOwnerFlag(t *testing.T) {
	if flag := harnessBudgetCmd.PersistentFlags().Lookup("owner"); flag == nil {
		t.Fatal("harness budget command is missing --owner flag")
	}
}

func TestDoHarnessRequestAttachesLocalBearerToken(t *testing.T) {
	resetHarnessCLIState(t)

	const jwtSecret = "0123456789abcdef0123456789abcdef"
	cfgFile = writeHarnessCLITestConfig(t, jwtSecret)
	t.Setenv("BLUE_USER_ID", "release-user")

	jwtSvc := auth.NewJWTService(&auth.JWTConfig{
		Secret:            jwtSecret,
		Expiration:        24 * time.Hour,
		RefreshExpiration: 720 * time.Hour,
		Issuer:            "zimaos-blue",
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/harness/eval-runs/eval-token", func(w http.ResponseWriter, r *http.Request) {
		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		if !strings.HasPrefix(authHeader, "Bearer ") {
			t.Fatalf("authorization = %q, want bearer token", authHeader)
		}
		claims, err := jwtSvc.ValidateToken(strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer ")))
		if err != nil {
			t.Fatalf("validate token: %v", err)
		}
		if claims.UserID != "release-user" {
			t.Fatalf("claims.UserID = %q, want %q", claims.UserID, "release-user")
		}
		if claims.Role != string(user.RoleAdmin) {
			t.Fatalf("claims.Role = %q, want %q", claims.Role, string(user.RoleAdmin))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"eval-token","status":"completed"}`))
	})

	addr := setupHarnessCLIServer(t, mux)
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("SplitHostPort: %v", err)
	}
	t.Setenv("BLUE_SERVER_PORT", port)

	resp, err := doHarnessRequest(http.MethodGet, "/eval-runs/eval-token", nil)
	if err != nil {
		t.Fatalf("doHarnessRequest() error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestDoHarnessRequestUsesPersistedJWTSecretForProfile(t *testing.T) {
	resetHarnessCLIState(t)

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	profile = "cutover-auth"
	configDir := filepath.Join(homeDir, ".zimaos-blue-"+profile)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("MkdirAll configDir: %v", err)
	}
	cfgFile = filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(cfgFile, []byte("# isolated cutover profile\n"), 0o600); err != nil {
		t.Fatalf("write profile config: %v", err)
	}
	t.Setenv("BLUE_USER_ID", "release-user")

	yamlCfg, err := loadCLIConfig()
	if err != nil {
		t.Fatalf("loadCLIConfig() error = %v", err)
	}
	const persistedSecret = "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	yamlCfg.Security.JWT.Secret = persistedSecret

	dataDir := getDataDir()
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		t.Fatalf("MkdirAll dataDir: %v", err)
	}
	dbConn, err := openPrimaryDatabaseWithStartupRecovery(dataDir, yamlCfg.Performance.Database)
	if err != nil {
		t.Fatalf("openPrimaryDatabaseWithStartupRecovery() error = %v", err)
	}
	sqliteKV, err := kvstore.NewSQLiteStoreWithReadDB(dbConn.Writer, dbConn.Reader)
	if err != nil {
		_ = dbConn.Close()
		t.Fatalf("NewSQLiteStoreWithReadDB() error = %v", err)
	}
	store := config.NewConfigStore(kvstore.NewCachedStore(sqliteKV))
	if _, err := store.LoadOrImport(yamlCfg); err != nil {
		_ = dbConn.Close()
		t.Fatalf("LoadOrImport() error = %v", err)
	}
	if err := dbConn.Close(); err != nil {
		t.Fatalf("db close: %v", err)
	}
	t.Setenv("BLUE_IPC_SOCKET", setupHarnessCLIIPCSocket(t, func(srv *sockipc.Server) {
		registerHarnessCLIIPCHandlers(srv, yamlCfg, store, zap.NewNop())
	}))

	jwtSvc := auth.NewJWTService(&auth.JWTConfig{
		Secret:            persistedSecret,
		Expiration:        24 * time.Hour,
		RefreshExpiration: 720 * time.Hour,
		Issuer:            "zimaos-blue",
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/harness/eval-runs/eval-profile", func(w http.ResponseWriter, r *http.Request) {
		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		if !strings.HasPrefix(authHeader, "Bearer ") {
			t.Fatalf("authorization = %q, want bearer token", authHeader)
		}
		claims, err := jwtSvc.ValidateToken(strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer ")))
		if err != nil {
			t.Fatalf("validate token: %v", err)
		}
		if claims.UserID != "release-user" {
			t.Fatalf("claims.UserID = %q, want %q", claims.UserID, "release-user")
		}
		if claims.Role != string(user.RoleAdmin) {
			t.Fatalf("claims.Role = %q, want %q", claims.Role, string(user.RoleAdmin))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"eval-profile","status":"completed"}`))
	})

	addr := setupHarnessCLIServer(t, mux)
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("SplitHostPort: %v", err)
	}
	t.Setenv("BLUE_SERVER_PORT", port)

	resp, err := doHarnessRequest(http.MethodGet, "/eval-runs/eval-profile", nil)
	if err != nil {
		t.Fatalf("doHarnessRequest() error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestRunHarnessSelectorVerifyEnsuresRunsWaitsAndGates(t *testing.T) {
	resetHarnessCLIState(t)

	var ensureCalls int
	var runCalls int
	var pollCalls int
	var gateCalls int

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/harness/selector-curated/ensure", func(w http.ResponseWriter, r *http.Request) {
		ensureCalls++
		if r.Method != http.MethodPost {
			t.Fatalf("ensure method = %s, want POST", r.Method)
		}
		var req struct {
			OwnerUserID string `json:"owner_user_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode ensure request: %v", err)
		}
		if req.OwnerUserID != "release-user" {
			t.Fatalf("ensure owner = %q, want %q", req.OwnerUserID, "release-user")
		}

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(harnesspkg.SelectorCuratedAssets{
			Dataset:        &harnesspkg.Dataset{ID: "dataset-1", Name: harnesspkg.SelectorCuratedDatasetName},
			DatasetVersion: &harnesspkg.DatasetVersion{ID: "version-1", Version: "v1"},
			EvalSpec:       &harnesspkg.EvalSpec{ID: "eval-spec-1", Name: harnesspkg.SelectorCuratedEvalName},
		})
	})
	mux.HandleFunc("/api/v1/harness/eval-runs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("eval-runs method = %s, want POST", r.Method)
		}
		runCalls++
		var req harnesspkg.EvalRunSpec
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode eval run request: %v", err)
		}
		if req.EvalSpecID != "eval-spec-1" {
			t.Fatalf("eval_spec_id = %q, want %q", req.EvalSpecID, "eval-spec-1")
		}
		if req.OwnerUserID != "release-user" {
			t.Fatalf("owner_user_id = %q, want %q", req.OwnerUserID, "release-user")
		}
		if req.Title != "candidate-verify" {
			t.Fatalf("title = %q, want %q", req.Title, "candidate-verify")
		}
		if req.BaselineEvalRunID != "baseline-run-1" {
			t.Fatalf("baseline_eval_run_id = %q, want %q", req.BaselineEvalRunID, "baseline-run-1")
		}
		if got := req.Metadata["candidate_id"]; got != "rc-selector" {
			t.Fatalf("metadata.candidate_id = %#v, want rc-selector", got)
		}

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(harnesspkg.EvalRun{
			ID:          "eval-run-1",
			EvalSpecID:  "eval-spec-1",
			GroupID:     "group-1",
			OwnerUserID: "release-user",
			Status:      harnesspkg.RunGroupStatusQueued,
		})
	})
	mux.HandleFunc("/api/v1/harness/eval-runs/eval-run-1", func(w http.ResponseWriter, r *http.Request) {
		pollCalls++
		status := harnesspkg.RunGroupStatusQueued
		if pollCalls >= 2 {
			status = harnesspkg.RunGroupStatusCompleted
		}
		_ = json.NewEncoder(w).Encode(harnesspkg.EvalRun{
			ID:          "eval-run-1",
			EvalSpecID:  "eval-spec-1",
			GroupID:     "group-1",
			OwnerUserID: "release-user",
			Title:       "candidate-verify",
			Status:      status,
		})
	})
	mux.HandleFunc("/api/v1/harness/eval-runs/eval-run-1/selector-gate", func(w http.ResponseWriter, r *http.Request) {
		gateCalls++
		if r.Method != http.MethodPost {
			t.Fatalf("selector gate method = %s, want POST", r.Method)
		}
		var req harnesspkg.SelectorGateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode selector gate request: %v", err)
		}
		if req.BaseEvalRunID != "baseline-run-1" {
			t.Fatalf("base_eval_run_id = %q, want %q", req.BaseEvalRunID, "baseline-run-1")
		}
		if req.BaselineID != "baseline-id-1" {
			t.Fatalf("baseline_id = %q, want %q", req.BaselineID, "baseline-id-1")
		}
		if req.Thresholds.MinRouteAgreementRate == nil || *req.Thresholds.MinRouteAgreementRate != 0.95 {
			t.Fatalf("min_route_agreement_rate = %#v, want 0.95", req.Thresholds.MinRouteAgreementRate)
		}
		if req.Thresholds.MaxClarifyRateDelta == nil || *req.Thresholds.MaxClarifyRateDelta != 0.01 {
			t.Fatalf("max_clarify_rate_delta = %#v, want 0.01", req.Thresholds.MaxClarifyRateDelta)
		}

		_ = json.NewEncoder(w).Encode(harnesspkg.SelectorGateReport{
			TargetEvalRunID: "eval-run-1",
			BaseEvalRunID:   "baseline-run-1",
			BaselineID:      "baseline-id-1",
			Passed:          true,
			Metrics: harnesspkg.SelectorGateMetrics{
				CaseCount:           120,
				PassedCount:         119,
				PassRate:            0.9916666667,
				CriticalCaseCount:   10,
				CriticalPassedCount: 10,
				CriticalPassRate:    1.0,
				RouteCaseCount:      120,
				RouteAgreementCount: 118,
				RouteAgreementRate:  0.9833333333,
			},
			Thresholds: map[string]interface{}{
				"min_pass_rate":                 0.98,
				"min_critical_pass_rate":        1.0,
				"min_route_agreement_rate":      0.95,
				"max_clarify_rate_delta":        0.01,
				"max_critical_regression_count": 0,
			},
		})
	})

	addr := setupHarnessCLIServer(t, mux)
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	t.Setenv("BLUE_SERVER_HOST", host)
	t.Setenv("BLUE_SERVER_PORT", portStr)
	t.Setenv("BLUE_USER_ID", "release-user")

	harnessSelectorTitle = "candidate-verify"
	harnessSelectorBaselineID = "baseline-id-1"
	harnessSelectorBaseEvalID = "baseline-run-1"
	harnessSelectorCandidateID = "rc-selector"
	harnessSelectorMinRouteAgreementRate = "0.95"
	harnessSelectorMaxClarifyRateDelta = "0.01"
	harnessSelectorMaxCriticalRegressions = "0"

	out := captureStdout(t, func() {
		if err := runHarnessSelectorVerifyE(nil, nil); err != nil {
			t.Fatalf("runHarnessSelectorVerifyE() error = %v", err)
		}
	})

	var resp selectorVerifyOutput
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("decode verify output: %v\n%s", err, out)
	}
	if resp.EvalRun == nil || resp.EvalRun.ID != "eval-run-1" {
		t.Fatalf("eval_run = %#v, want eval-run-1", resp.EvalRun)
	}
	if resp.SelectorGate == nil || !resp.SelectorGate.Passed {
		t.Fatalf("selector_gate = %#v, want passed report", resp.SelectorGate)
	}
	if ensureCalls != 1 || runCalls != 1 || gateCalls != 1 {
		t.Fatalf("call counts ensure=%d run=%d gate=%d, want 1/1/1", ensureCalls, runCalls, gateCalls)
	}
	if pollCalls < 2 {
		t.Fatalf("expected at least 2 poll calls, got %d", pollCalls)
	}
}

func TestRunHarnessSelectorGateReturnsErrorWhenGateFails(t *testing.T) {
	resetHarnessCLIState(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/harness/eval-runs/eval-run-fail/selector-gate", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(harnesspkg.SelectorGateReport{
			TargetEvalRunID: "eval-run-fail",
			Passed:          false,
			Checks: []harnesspkg.SelectorGateCheck{
				{Name: "pass_rate", Passed: false},
				{Name: "critical_pass_rate", Passed: true},
			},
		})
	})

	addr := setupHarnessCLIServer(t, mux)
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	t.Setenv("BLUE_SERVER_HOST", host)
	t.Setenv("BLUE_SERVER_PORT", portStr)

	out := captureStdout(t, func() {
		err = runHarnessSelectorGateE(nil, []string{"eval-run-fail"})
	})
	if err == nil {
		t.Fatal("expected selector gate failure")
	}
	if !strings.Contains(err.Error(), "selector gate failed: pass_rate") {
		t.Fatalf("error = %v, want selector gate failed: pass_rate", err)
	}

	var report harnesspkg.SelectorGateReport
	if decodeErr := json.Unmarshal([]byte(out), &report); decodeErr != nil {
		t.Fatalf("decode gate output: %v\n%s", decodeErr, out)
	}
	if report.TargetEvalRunID != "eval-run-fail" || report.Passed {
		t.Fatalf("report = %#v, want failed eval-run-fail", report)
	}
}

func TestRunHarnessExecutionVerifyEnsuresRunsWaitsAndGates(t *testing.T) {
	resetHarnessCLIState(t)

	var ensureCalls int
	var runCalls int
	var pollCalls int
	var gateCalls int

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/harness/execution-batch1/ensure", func(w http.ResponseWriter, r *http.Request) {
		ensureCalls++
		if r.Method != http.MethodPost {
			t.Fatalf("ensure method = %s, want POST", r.Method)
		}
		var req struct {
			OwnerUserID string `json:"owner_user_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode ensure request: %v", err)
		}
		if req.OwnerUserID != "release-user" {
			t.Fatalf("ensure owner = %q, want %q", req.OwnerUserID, "release-user")
		}

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(harnesspkg.Batch1ExecutionAssets{
			Dataset:        &harnesspkg.Dataset{ID: "dataset-1", Name: harnesspkg.Batch1ExecutionDatasetName},
			DatasetVersion: &harnesspkg.DatasetVersion{ID: "version-1", Version: "v1"},
			EvalSpec:       &harnesspkg.EvalSpec{ID: "eval-spec-1", Name: harnesspkg.Batch1ExecutionEvalName},
		})
	})
	mux.HandleFunc("/api/v1/harness/eval-runs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("eval-runs method = %s, want POST", r.Method)
		}
		runCalls++
		var req harnesspkg.EvalRunSpec
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode eval run request: %v", err)
		}
		if req.EvalSpecID != "eval-spec-1" {
			t.Fatalf("eval_spec_id = %q, want %q", req.EvalSpecID, "eval-spec-1")
		}
		if req.OwnerUserID != "release-user" {
			t.Fatalf("owner_user_id = %q, want %q", req.OwnerUserID, "release-user")
		}
		if req.Title != "execution-candidate" {
			t.Fatalf("title = %q, want %q", req.Title, "execution-candidate")
		}
		if req.BaselineEvalRunID != "baseline-run-1" {
			t.Fatalf("baseline_eval_run_id = %q, want %q", req.BaselineEvalRunID, "baseline-run-1")
		}
		if got := req.Metadata["candidate_id"]; got != "rc-execution" {
			t.Fatalf("metadata.candidate_id = %#v, want rc-execution", got)
		}

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(harnesspkg.EvalRun{
			ID:          "eval-run-1",
			EvalSpecID:  "eval-spec-1",
			GroupID:     "group-1",
			OwnerUserID: "release-user",
			Status:      harnesspkg.RunGroupStatusQueued,
		})
	})
	mux.HandleFunc("/api/v1/harness/eval-runs/eval-run-1", func(w http.ResponseWriter, r *http.Request) {
		pollCalls++
		status := harnesspkg.RunGroupStatusQueued
		if pollCalls >= 2 {
			status = harnesspkg.RunGroupStatusCompleted
		}
		_ = json.NewEncoder(w).Encode(harnesspkg.EvalRun{
			ID:          "eval-run-1",
			EvalSpecID:  "eval-spec-1",
			GroupID:     "group-1",
			OwnerUserID: "release-user",
			Title:       "execution-candidate",
			Status:      status,
		})
	})
	mux.HandleFunc("/api/v1/harness/eval-runs/eval-run-1/execution-gate", func(w http.ResponseWriter, r *http.Request) {
		gateCalls++
		if r.Method != http.MethodPost {
			t.Fatalf("execution gate method = %s, want POST", r.Method)
		}
		var req harnesspkg.ExecutionEquivalenceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode execution gate request: %v", err)
		}
		if req.BaseEvalRunID != "baseline-run-1" {
			t.Fatalf("base_eval_run_id = %q, want %q", req.BaseEvalRunID, "baseline-run-1")
		}
		if req.BaselineID != "baseline-id-1" {
			t.Fatalf("baseline_id = %q, want %q", req.BaselineID, "baseline-id-1")
		}
		if req.Thresholds.MaxPassRateDrop == nil || *req.Thresholds.MaxPassRateDrop != 0.01 {
			t.Fatalf("max_pass_rate_drop = %#v, want 0.01", req.Thresholds.MaxPassRateDrop)
		}
		if req.Thresholds.MaxCriticalRegressionCount == nil || *req.Thresholds.MaxCriticalRegressionCount != 0 {
			t.Fatalf("max_critical_regression_count = %#v, want 0", req.Thresholds.MaxCriticalRegressionCount)
		}

		_ = json.NewEncoder(w).Encode(harnesspkg.ExecutionEquivalenceReport{
			TargetEvalRunID: "eval-run-1",
			BaseEvalRunID:   "baseline-run-1",
			BaselineID:      "baseline-id-1",
			Passed:          true,
			Metrics: harnesspkg.ExecutionEquivalenceMetrics{
				CaseCount:               81,
				PassedCount:             81,
				PassRate:                1.0,
				CriticalCaseCount:       4,
				CriticalPassedCount:     4,
				CriticalPassRate:        1.0,
				BasePassRate:            1.0,
				TargetPassRate:          1.0,
				PassRateDelta:           0.0,
				CriticalRegressionCount: 0,
			},
			Thresholds: map[string]interface{}{
				"max_pass_rate_drop":            0.01,
				"max_critical_regression_count": 0,
			},
		})
	})

	addr := setupHarnessCLIServer(t, mux)
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	t.Setenv("BLUE_SERVER_HOST", host)
	t.Setenv("BLUE_SERVER_PORT", portStr)
	t.Setenv("BLUE_USER_ID", "release-user")

	harnessExecutionTitle = "execution-candidate"
	harnessExecutionBaselineID = "baseline-id-1"
	harnessExecutionBaseEvalID = "baseline-run-1"
	harnessExecutionCandidateID = "rc-execution"
	harnessExecutionMaxPassRateDrop = "0.01"
	harnessExecutionMaxCriticalRegressions = "0"

	out := captureStdout(t, func() {
		if err := runHarnessExecutionVerifyE(nil, nil); err != nil {
			t.Fatalf("runHarnessExecutionVerifyE() error = %v", err)
		}
	})

	var resp executionVerifyOutput
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("decode verify output: %v\n%s", err, out)
	}
	if resp.EvalRun == nil || resp.EvalRun.ID != "eval-run-1" {
		t.Fatalf("eval_run = %#v, want eval-run-1", resp.EvalRun)
	}
	if resp.ExecutionGate == nil || !resp.ExecutionGate.Passed {
		t.Fatalf("execution_gate = %#v, want passed report", resp.ExecutionGate)
	}
	if ensureCalls != 1 || runCalls != 1 || gateCalls != 1 {
		t.Fatalf("call counts ensure=%d run=%d gate=%d, want 1/1/1", ensureCalls, runCalls, gateCalls)
	}
	if pollCalls < 2 {
		t.Fatalf("expected at least 2 poll calls, got %d", pollCalls)
	}
}

func TestRunHarnessExecutionGateReturnsErrorWhenGateFails(t *testing.T) {
	resetHarnessCLIState(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/harness/eval-runs/eval-run-fail/execution-gate", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(harnesspkg.ExecutionEquivalenceReport{
			TargetEvalRunID: "eval-run-fail",
			Passed:          false,
			Checks: []harnesspkg.ExecutionEquivalenceCheck{
				{Name: "pass_rate_drop", Passed: false},
				{Name: "critical_regression_count", Passed: true},
			},
		})
	})

	addr := setupHarnessCLIServer(t, mux)
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	t.Setenv("BLUE_SERVER_HOST", host)
	t.Setenv("BLUE_SERVER_PORT", portStr)

	out := captureStdout(t, func() {
		err = runHarnessExecutionGateE(nil, []string{"eval-run-fail"})
	})
	if err == nil {
		t.Fatal("expected execution gate failure")
	}
	if !strings.Contains(err.Error(), "execution gate failed: pass_rate_drop") {
		t.Fatalf("error = %v, want execution gate failed: pass_rate_drop", err)
	}

	var report harnesspkg.ExecutionEquivalenceReport
	if decodeErr := json.Unmarshal([]byte(out), &report); decodeErr != nil {
		t.Fatalf("decode gate output: %v\n%s", decodeErr, out)
	}
	if report.TargetEvalRunID != "eval-run-fail" || report.Passed {
		t.Fatalf("report = %#v, want failed eval-run-fail", report)
	}
}

func TestParseHarnessSelectorThresholdsRejectsOutOfRangeRates(t *testing.T) {
	resetHarnessCLIState(t)

	harnessSelectorMinPassRate = "1.1"
	_, err := parseHarnessSelectorThresholds()
	if err == nil {
		t.Fatal("expected threshold parse error")
	}
	if !strings.Contains(err.Error(), "min-pass-rate") {
		t.Fatalf("error = %v, want min-pass-rate", err)
	}
}

func TestParseHarnessExecutionThresholdsRejectsOutOfRangeRates(t *testing.T) {
	resetHarnessCLIState(t)

	harnessExecutionMaxPassRateDrop = "1.1"
	_, err := parseHarnessExecutionThresholds()
	if err == nil {
		t.Fatal("expected threshold parse error")
	}
	if !strings.Contains(err.Error(), "max-pass-rate-drop") {
		t.Fatalf("error = %v, want max-pass-rate-drop", err)
	}
}

func TestParseHarnessBudgetThresholdsRejectsOutOfRangeRates(t *testing.T) {
	resetHarnessCLIState(t)

	harnessBudgetMinMedianSchemaByteReductionRate = "1.1"
	_, err := parseHarnessBudgetThresholds(
		harnessBudgetMinMedianSchemaByteReductionRate,
		harnessBudgetMaxMedianLatencyIncreaseRate,
		harnessBudgetAllowedFinalNativeTools,
	)
	if err == nil {
		t.Fatal("expected threshold parse error")
	}
	if !strings.Contains(err.Error(), "min-median-schema-byte-reduction-rate") {
		t.Fatalf("error = %v, want min-median-schema-byte-reduction-rate", err)
	}
}

func TestGetHarnessBaseURLUsesServiceHostAndPort(t *testing.T) {
	t.Setenv("BLUE_SERVER_HOST", "127.0.0.1")
	t.Setenv("BLUE_SERVER_PORT", strconv.Itoa(19098))

	if got := getHarnessBaseURL(); got != "http://127.0.0.1:19098/api/v1/harness" {
		t.Fatalf("getHarnessBaseURL() = %q, want %q", got, "http://127.0.0.1:19098/api/v1/harness")
	}
}

func TestRunHarnessBudgetGateReturnsStructuredReport(t *testing.T) {
	resetHarnessCLIState(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/harness/eval-runs/eval-budget-1/budget-gate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("budget gate method = %s, want POST", r.Method)
		}
		var req harnesspkg.SkillCutoverBudgetRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode budget gate request: %v", err)
		}
		if req.BaselineID != "budget-baseline-1" {
			t.Fatalf("baseline_id = %q, want %q", req.BaselineID, "budget-baseline-1")
		}
		if req.Thresholds.MinMedianSchemaByteReductionRate == nil || *req.Thresholds.MinMedianSchemaByteReductionRate != 0.8 {
			t.Fatalf("min_median_schema_byte_reduction_rate = %#v, want 0.8", req.Thresholds.MinMedianSchemaByteReductionRate)
		}
		if req.Thresholds.MaxMedianLatencyIncreaseRate == nil || *req.Thresholds.MaxMedianLatencyIncreaseRate != 0.1 {
			t.Fatalf("max_median_latency_increase_rate = %#v, want 0.1", req.Thresholds.MaxMedianLatencyIncreaseRate)
		}
		if got := req.Thresholds.AllowedFinalNativeTools; len(got) != 1 || got[0] != "exec" {
			t.Fatalf("allowed_final_native_tools = %#v, want [exec]", got)
		}

		_ = json.NewEncoder(w).Encode(harnesspkg.SkillCutoverBudgetReport{
			TargetEvalRunID: "eval-budget-1",
			BaseEvalRunID:   "eval-budget-base-1",
			BaselineID:      "budget-baseline-1",
			Metrics: harnesspkg.SkillCutoverBudgetMetrics{
				CaseCount:                     2,
				ComparableCaseCount:           2,
				MedianSchemaByteReductionRate: 0.85,
				MedianLatencyIncreaseRate:     0.04,
			},
			Passed: true,
		})
	})

	addr := setupHarnessCLIServer(t, mux)
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	t.Setenv("BLUE_SERVER_HOST", host)
	t.Setenv("BLUE_SERVER_PORT", portStr)

	harnessBudgetBaselineID = "budget-baseline-1"
	harnessBudgetMinMedianSchemaByteReductionRate = "0.8"
	harnessBudgetMaxMedianLatencyIncreaseRate = "0.1"
	harnessBudgetAllowedFinalNativeTools = "exec"

	out := captureStdout(t, func() {
		err = runHarnessBudgetGateE(nil, []string{"eval-budget-1"})
	})
	if err != nil {
		t.Fatalf("runHarnessBudgetGateE error: %v", err)
	}

	var report harnesspkg.SkillCutoverBudgetReport
	if decodeErr := json.Unmarshal([]byte(out), &report); decodeErr != nil {
		t.Fatalf("decode budget gate output: %v\n%s", decodeErr, out)
	}
	if report.TargetEvalRunID != "eval-budget-1" || !report.Passed {
		t.Fatalf("report = %#v, want passed eval-budget-1", report)
	}
}

func TestRunHarnessCutoverReadinessReturnsStructuredReport(t *testing.T) {
	resetHarnessCLIState(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/harness/cutover-readiness", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("cutover readiness method = %s, want POST", r.Method)
		}
		var req harnesspkg.SkillCutoverReadinessRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode cutover readiness request: %v", err)
		}
		if req.OwnerUserID != "release-user" {
			t.Fatalf("owner_user_id = %q, want %q", req.OwnerUserID, "release-user")
		}
		if req.CandidateID != "rc-cutover" {
			t.Fatalf("candidate_id = %q, want %q", req.CandidateID, "rc-cutover")
		}
		if req.RequiredConsecutiveRuns != 2 {
			t.Fatalf("required_consecutive_runs = %d, want 2", req.RequiredConsecutiveRuns)
		}
		if req.Selector.BaselineID != "selector-baseline-1" {
			t.Fatalf("selector.baseline_id = %q, want %q", req.Selector.BaselineID, "selector-baseline-1")
		}
		if req.Execution.BaselineID != "execution-baseline-1" {
			t.Fatalf("execution.baseline_id = %q, want %q", req.Execution.BaselineID, "execution-baseline-1")
		}
		if req.Budget.BaselineID != "budget-baseline-1" {
			t.Fatalf("budget.baseline_id = %q, want %q", req.Budget.BaselineID, "budget-baseline-1")
		}
		if req.Budget.Thresholds.MinMedianSchemaByteReductionRate == nil || *req.Budget.Thresholds.MinMedianSchemaByteReductionRate != 0.8 {
			t.Fatalf("budget.min_median_schema_byte_reduction_rate = %#v, want 0.8", req.Budget.Thresholds.MinMedianSchemaByteReductionRate)
		}
		if req.Budget.Thresholds.MaxMedianLatencyIncreaseRate == nil || *req.Budget.Thresholds.MaxMedianLatencyIncreaseRate != 0.1 {
			t.Fatalf("budget.max_median_latency_increase_rate = %#v, want 0.1", req.Budget.Thresholds.MaxMedianLatencyIncreaseRate)
		}
		if got := req.Budget.Thresholds.AllowedFinalNativeTools; len(got) != 1 || got[0] != "exec" {
			t.Fatalf("budget.allowed_final_native_tools = %#v, want [exec]", got)
		}

		_ = json.NewEncoder(w).Encode(harnesspkg.SkillCutoverReadinessReport{
			CandidateID:             "rc-cutover",
			RequiredConsecutiveRuns: 2,
			EvaluatedGatesReady:     false,
			Ready:                   false,
			Selector: harnesspkg.SkillCutoverLaneReadiness{
				CandidateID:          "rc-cutover",
				ConsecutivePassCount: 2,
				Ready:                true,
			},
			Execution: harnesspkg.SkillCutoverLaneReadiness{
				CandidateID:          "rc-cutover",
				ConsecutivePassCount: 2,
				Ready:                true,
			},
			Budget: harnesspkg.SkillCutoverLaneReadiness{
				CandidateID:          "rc-cutover",
				ConsecutivePassCount: 0,
				Ready:                false,
			},
			BlockingReasons: []string{`budget has 0 consecutive green runs for candidate "rc-cutover"; need 2`},
		})
	})

	addr := setupHarnessCLIServer(t, mux)
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	t.Setenv("BLUE_SERVER_HOST", host)
	t.Setenv("BLUE_SERVER_PORT", portStr)
	t.Setenv("BLUE_USER_ID", "release-user")

	harnessCutoverCandidateID = "rc-cutover"
	harnessCutoverRequiredConsecutiveRuns = 2
	harnessCutoverSelectorBaselineID = "selector-baseline-1"
	harnessCutoverExecutionBaselineID = "execution-baseline-1"
	harnessCutoverBudgetBaselineID = "budget-baseline-1"
	harnessCutoverMinMedianSchemaByteReductionRate = "0.8"
	harnessCutoverMaxMedianLatencyIncreaseRate = "0.1"
	harnessCutoverAllowedFinalNativeTools = "exec"

	out := captureStdout(t, func() {
		err = runHarnessCutoverReadinessE(nil, nil)
	})
	if err == nil {
		t.Fatal("expected cutover readiness to fail while requirements remain unverified")
	}
	if !strings.Contains(err.Error(), "budget has 0 consecutive green runs") {
		t.Fatalf("error = %v, want budget blocking reason", err)
	}

	var report harnesspkg.SkillCutoverReadinessReport
	if decodeErr := json.Unmarshal([]byte(out), &report); decodeErr != nil {
		t.Fatalf("decode cutover readiness output: %v\n%s", decodeErr, out)
	}
	if report.CandidateID != "rc-cutover" {
		t.Fatalf("candidate_id = %q, want rc-cutover", report.CandidateID)
	}
	if report.EvaluatedGatesReady {
		t.Fatalf("evaluated_gates_ready = %#v, want false", report.EvaluatedGatesReady)
	}
}

func TestPrintSelectorGateSummaryIncludesDiscoverBreakdowns(t *testing.T) {
	resetHarnessCLIState(t)
	jsonOutput = false

	out := captureStdout(t, func() {
		printSelectorGateSummary(&harnesspkg.SelectorGateReport{
			TargetEvalRunID: "eval-selector-1",
			Passed:          true,
			Metrics: harnesspkg.SelectorGateMetrics{
				CaseCount:                       2,
				PassedCount:                     2,
				PassRate:                        1,
				SelectedCanonicalSkillBreakdown: map[string]int{"web_query": 1, "exec": 1},
				NativeSurfaceModeBreakdown:      map[string]int{"skill_exec": 1, "clarify_none": 1},
				NativeSurfaceReasonBreakdown:    map[string]int{"discover_first_cutover": 1, "clarify_required": 1},
				ExecutionProfileBreakdown:       map[string]int{"prefer_fork": 1, "inline": 1},
			},
		})
	})

	if !strings.Contains(out, "Canonical skills: exec=1, web_query=1") {
		t.Fatalf("selector summary missing canonical skills breakdown:\n%s", out)
	}
	if !strings.Contains(out, "Native surface modes: clarify_none=1, skill_exec=1") {
		t.Fatalf("selector summary missing native surface mode breakdown:\n%s", out)
	}
	if !strings.Contains(out, "Native surface reasons: clarify_required=1, discover_first_cutover=1") {
		t.Fatalf("selector summary missing native surface reason breakdown:\n%s", out)
	}
	if !strings.Contains(out, "Execution profiles: inline=1, prefer_fork=1") {
		t.Fatalf("selector summary missing execution profile breakdown:\n%s", out)
	}
}

func TestPrintBudgetGateSummaryIncludesDiscoverBreakdowns(t *testing.T) {
	resetHarnessCLIState(t)
	jsonOutput = false

	out := captureStdout(t, func() {
		printBudgetGateSummary(&harnesspkg.SkillCutoverBudgetReport{
			TargetEvalRunID: "eval-budget-1",
			Passed:          true,
			Metrics: harnesspkg.SkillCutoverBudgetMetrics{
				CaseCount:                       2,
				ComparableCaseCount:             2,
				MedianSchemaByteReductionRate:   0.85,
				MedianLatencyIncreaseRate:       0.04,
				SelectedCanonicalSkillBreakdown: map[string]int{"web_query": 1, "exec": 1},
				NativeSurfaceModeBreakdown:      map[string]int{"skill_exec": 1, "clarify_none": 1},
				NativeSurfaceReasonBreakdown:    map[string]int{"discover_first_cutover": 1, "clarify_required": 1},
				ExecutionProfileBreakdown:       map[string]int{"prefer_fork": 1, "inline": 1},
			},
		})
	})

	if !strings.Contains(out, "Canonical skills: exec=1, web_query=1") {
		t.Fatalf("budget summary missing canonical skills breakdown:\n%s", out)
	}
	if !strings.Contains(out, "Native surface modes: clarify_none=1, skill_exec=1") {
		t.Fatalf("budget summary missing native surface mode breakdown:\n%s", out)
	}
	if !strings.Contains(out, "Native surface reasons: clarify_required=1, discover_first_cutover=1") {
		t.Fatalf("budget summary missing native surface reason breakdown:\n%s", out)
	}
	if !strings.Contains(out, "Execution profiles: inline=1, prefer_fork=1") {
		t.Fatalf("budget summary missing execution profile breakdown:\n%s", out)
	}
}
