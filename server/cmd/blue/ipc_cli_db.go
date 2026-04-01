package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/contextpack"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sockipc"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/user"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workspace"
	"go.uber.org/zap"
)

var contextCLIIPCMu sync.Mutex

type contextCLIExit struct {
	code int
}

func registerCLIIPCHandlers(
	srv *sockipc.Server,
	workspaceMgr *workspace.Manager,
	registry *contextpack.Registry,
	store *contextpack.AnnotationStore,
	yamlCfg *config.Config,
	cfgStore *config.ConfigStore,
	log *zap.Logger,
) {
	if srv == nil {
		return
	}
	registerContextCLIIPCHandlers(srv, workspaceMgr, registry, store, log)
	registerHarnessCLIIPCHandlers(srv, yamlCfg, cfgStore, log)
}

func registerContextCLIIPCHandlers(
	srv *sockipc.Server,
	workspaceMgr *workspace.Manager,
	registry *contextpack.Registry,
	store *contextpack.AnnotationStore,
	log *zap.Logger,
) {
	if srv == nil || workspaceMgr == nil || registry == nil {
		return
	}

	rt := &localContextRuntime{
		workspace: workspaceMgr,
		registry:  registry,
		store:     store,
	}

	handlers := map[string]string{
		"context.search":   "search",
		"context.get":      "get",
		"context.annotate": "annotate",
		"context.import":   "import",
		"context.validate": "validate",
	}
	for cmd, action := range handlers {
		action := action
		srv.Handle(cmd, func(_ context.Context, req *sockipc.Request) *sockipc.Response {
			if requiresContextAnnotationStore(action) && rt.store == nil {
				return sockipc.ErrResponse("context annotation store is unavailable in the running Blue service")
			}
			stdout, exitCode, err := executeContextCLIIPCAction(rt, action, req.Params)
			if err != nil {
				if log != nil {
					log.Warn("sockipc context command failed", zap.String("cmd", cmd), zap.Error(err))
				}
				return sockipc.ErrResponse(err.Error())
			}
			return sockipc.OkResponse(map[string]string{
				"__stdout":    stdout,
				"__exit_code": strconv.Itoa(exitCode),
			})
		})
	}
}

func registerHarnessCLIIPCHandlers(
	srv *sockipc.Server,
	yamlCfg *config.Config,
	cfgStore *config.ConfigStore,
	log *zap.Logger,
) {
	if srv == nil {
		return
	}
	srv.Handle("cli.harness_auth_header", func(_ context.Context, req *sockipc.Request) *sockipc.Response {
		authHeader, err := buildHarnessAuthorizationHeaderFromRuntimeConfig(
			strings.TrimSpace(req.Params["owner_user_id"]),
			yamlCfg,
			cfgStore,
		)
		if err != nil {
			if log != nil {
				log.Warn("sockipc harness auth header failed", zap.Error(err))
			}
			return sockipc.ErrResponse(err.Error())
		}
		return sockipc.OkResponse(map[string]string{"authorization": authHeader})
	})
}

func executeContextCLIIPCAction(rt *localContextRuntime, action string, params map[string]string) (string, int, error) {
	if rt == nil {
		return "", 0, fmt.Errorf("context runtime is unavailable")
	}
	if rt.registry != nil && action != "annotate" {
		if err := rt.registry.Refresh(context.Background()); err != nil {
			return "", 0, err
		}
	}

	args, err := contextIPCArgs(action, params)
	if err != nil {
		return "", 0, err
	}

	contextCLIIPCMu.Lock()
	defer contextCLIIPCMu.Unlock()

	oldJSONOutput := jsonOutput
	oldContextLang := contextLang
	oldContextVersion := contextVersion
	oldContextFile := contextFile
	oldContextTenant := contextTenant
	oldContextUser := contextUser
	oldContextClear := contextClear
	oldContextList := contextList
	oldContextFull := contextFull
	oldRuntimeOpener := contextRuntimeOpener
	oldRuntimeCloser := contextRuntimeCloser
	oldContextExit := contextExit

	jsonOutput = parseIPCBoolParam(params["__blue_json"])
	contextLang = strings.TrimSpace(params["lang"])
	contextVersion = strings.TrimSpace(params["version"])
	contextFile = strings.TrimSpace(params["file"])
	contextTenant = strings.TrimSpace(params["tenant"])
	contextUser = strings.TrimSpace(params["user"])
	contextClear = parseIPCBoolParam(params["clear"])
	contextList = parseIPCBoolParam(params["list"])
	contextFull = parseIPCBoolParam(params["full"])
	contextRuntimeOpener = func() (*localContextRuntime, error) { return rt, nil }
	contextRuntimeCloser = func(*localContextRuntime) {}
	contextExit = func(code int) { panic(contextCLIExit{code: code}) }

	defer func() {
		jsonOutput = oldJSONOutput
		contextLang = oldContextLang
		contextVersion = oldContextVersion
		contextFile = oldContextFile
		contextTenant = oldContextTenant
		contextUser = oldContextUser
		contextClear = oldContextClear
		contextList = oldContextList
		contextFull = oldContextFull
		contextRuntimeOpener = oldRuntimeOpener
		contextRuntimeCloser = oldRuntimeCloser
		contextExit = oldContextExit
	}()

	return captureCLIStdoutAndExit(func() {
		switch action {
		case "search":
			runContextSearch(nil, args)
		case "get":
			runContextGet(nil, args)
		case "annotate":
			runContextAnnotate(nil, args)
		case "import":
			runContextImport(nil, args)
		case "validate":
			runContextValidate(nil, args)
		default:
			fmt.Printf("unknown context action %q\n", action)
			contextExit(1)
		}
	})
}

func contextIPCArgs(action string, params map[string]string) ([]string, error) {
	switch action {
	case "search":
		query := strings.TrimSpace(params["query"])
		if query == "" {
			return nil, fmt.Errorf("context search query is required")
		}
		return []string{query}, nil
	case "get":
		id := strings.TrimSpace(params["id"])
		if id == "" {
			return nil, fmt.Errorf("context id is required")
		}
		return []string{id}, nil
	case "annotate":
		id := strings.TrimSpace(params["id"])
		if id == "" {
			return nil, fmt.Errorf("context id is required")
		}
		if parseIPCBoolParam(params["list"]) || parseIPCBoolParam(params["clear"]) {
			return []string{id}, nil
		}
		note := strings.TrimSpace(params["note"])
		if note == "" {
			return []string{id}, nil
		}
		return []string{id, note}, nil
	case "import":
		path := strings.TrimSpace(params["path"])
		if path == "" {
			return nil, fmt.Errorf("import path is required")
		}
		return []string{path}, nil
	case "validate":
		path := strings.TrimSpace(params["path"])
		if path == "" {
			return nil, nil
		}
		return []string{path}, nil
	default:
		return nil, fmt.Errorf("unknown context action: %s", action)
	}
}

func requiresContextAnnotationStore(action string) bool {
	switch action {
	case "get", "annotate":
		return true
	default:
		return false
	}
}

func parseIPCBoolParam(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "t", "true", "y", "yes", "on":
		return true
	default:
		return false
	}
}

func captureCLIStdoutAndExit(fn func()) (string, int, error) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return "", 0, err
	}

	os.Stdout = w
	exitCode := 0
	var runErr error

	func() {
		defer func() {
			if rec := recover(); rec != nil {
				switch v := rec.(type) {
				case contextCLIExit:
					exitCode = v.code
				case error:
					runErr = v
				default:
					runErr = fmt.Errorf("panic: %v", rec)
				}
			}
		}()
		fn()
	}()

	os.Stdout = oldStdout
	_ = w.Close()

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()

	return buf.String(), exitCode, runErr
}

func buildHarnessAuthorizationHeaderFromRuntimeConfig(
	ownerUserID string,
	yamlCfg *config.Config,
	cfgStore *config.ConfigStore,
) (string, error) {
	cfg, err := effectiveHarnessRuntimeConfig(yamlCfg, cfgStore)
	if err != nil {
		return "", err
	}
	return buildHarnessAuthorizationHeaderFromConfig(cfg, ownerUserID)
}

func effectiveHarnessRuntimeConfig(yamlCfg *config.Config, cfgStore *config.ConfigStore) (*config.Config, error) {
	if yamlCfg == nil {
		return nil, nil
	}
	if !usesDefaultHarnessJWTSecret(yamlCfg) {
		return yamlCfg, nil
	}
	if cfgStore == nil {
		return yamlCfg, nil
	}
	persisted, err := cfgStore.LoadOrImport(yamlCfg)
	if err != nil || persisted == nil {
		return yamlCfg, err
	}
	return persisted, nil
}

func buildHarnessAuthorizationHeaderFromConfig(cfg *config.Config, ownerUserID string) (string, error) {
	if cfg == nil {
		return "", nil
	}
	jwtCfg := cfg.Security.JWT
	if strings.TrimSpace(jwtCfg.Secret) == "" {
		return "", nil
	}
	svc := auth.NewJWTService(&auth.JWTConfig{
		Secret:            jwtCfg.Secret,
		Expiration:        jwtCfg.Expiration,
		RefreshExpiration: jwtCfg.RefreshExpiration,
		Issuer:            jwtCfg.Issuer,
	})
	token, err := svc.GenerateAccessToken(&auth.UserClaims{
		UserID:   resolveHarnessOwnerUserID(ownerUserID),
		Username: "local-cli",
		Role:     string(user.RoleAdmin),
	})
	if err != nil {
		return "", err
	}
	return "Bearer " + token, nil
}
