package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeMediaContractGo_DelegatesTypesStoragePushPersistenceAndIPCLanes(t *testing.T) {
	contractContent, err := os.ReadFile(filepath.Join("runtime_media_contract.go"))
	if err != nil {
		t.Fatalf("read runtime_media_contract.go: %v", err)
	}
	contractSource := string(contractContent)

	typeContent, err := os.ReadFile(filepath.Join("runtime_media_contract_types.go"))
	if err != nil {
		t.Fatalf("read runtime_media_contract_types.go: %v", err)
	}
	typeSource := string(typeContent)

	storageContent, err := os.ReadFile(filepath.Join("runtime_media_contract_storage.go"))
	if err != nil {
		t.Fatalf("read runtime_media_contract_storage.go: %v", err)
	}
	storageSource := string(storageContent)

	pushContent, err := os.ReadFile(filepath.Join("runtime_media_contract_push_persistence.go"))
	if err != nil {
		t.Fatalf("read runtime_media_contract_push_persistence.go: %v", err)
	}
	pushSource := string(pushContent)

	ipcContent, err := os.ReadFile(filepath.Join("runtime_media_contract_ipc.go"))
	if err != nil {
		t.Fatalf("read runtime_media_contract_ipc.go: %v", err)
	}
	ipcSource := string(ipcContent)

	authHelperContent, err := os.ReadFile(filepath.Join("runtime_route_auth_helpers.go"))
	if err != nil {
		t.Fatalf("read runtime_route_auth_helpers.go: %v", err)
	}
	authHelperSource := string(authHelperContent)

	if lines := strings.Count(contractSource, "\n") + 1; lines > 120 {
		t.Fatalf("expected runtime_media_contract.go to stay below 120 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(typeSource, "\n") + 1; lines > 25 {
		t.Fatalf("expected runtime_media_contract_types.go to stay below 25 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(storageSource, "\n") + 1; lines > 135 {
		t.Fatalf("expected runtime_media_contract_storage.go to stay below 135 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(pushSource, "\n") + 1; lines > 170 {
		t.Fatalf("expected runtime_media_contract_push_persistence.go to stay below 170 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(ipcSource, "\n") + 1; lines > 115 {
		t.Fatalf("expected runtime_media_contract_ipc.go to stay below 115 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(authHelperSource, "\n") + 1; lines > 25 {
		t.Fatalf("expected runtime_route_auth_helpers.go to stay below 25 lines after extraction, got %d", lines)
	}

	requiredContract := []string{
		"func (binding *runtimeContractBinding) BindMediaRuntime(",
		"func bindRouteRuntimeMedia(",
		"newRouteRuntimeMediaConfigStore(",
		"bindRouteRuntimeMediaFallback(",
		"newRouteRuntimeMediaTaskStore(",
		"bindRouteRuntimeMediaWebPush(",
		"configureRouteRuntimeMediaPersistence(",
		"registerRouteRuntimeMediaIPC(",
	}
	for _, token := range requiredContract {
		if !strings.Contains(contractSource, token) {
			t.Fatalf("expected runtime_media_contract.go to contain token %q", token)
		}
	}

	requiredTypes := []string{
		"type routeRuntimeContractMediaOptions struct {",
		"type routeRuntimeContractMediaResult struct {",
	}
	for _, token := range requiredTypes {
		if !strings.Contains(typeSource, token) {
			t.Fatalf("expected runtime_media_contract_types.go to contain token %q", token)
		}
	}

	requiredStorage := []string{
		"func newRouteRuntimeMediaConfigStore(",
		"func bindRouteRuntimeMediaFallback(",
		"func newRouteRuntimeMediaTaskStore(",
	}
	for _, token := range requiredStorage {
		if !strings.Contains(storageSource, token) {
			t.Fatalf("expected runtime_media_contract_storage.go to contain token %q", token)
		}
	}

	requiredPush := []string{
		"func bindRouteRuntimeMediaWebPush(",
		"func configureRouteRuntimeMediaPersistence(",
	}
	for _, token := range requiredPush {
		if !strings.Contains(pushSource, token) {
			t.Fatalf("expected runtime_media_contract_push_persistence.go to contain token %q", token)
		}
	}

	if !strings.Contains(ipcSource, "func registerRouteRuntimeMediaIPC(") {
		t.Fatalf("expected runtime_media_contract_ipc.go to contain registerRouteRuntimeMediaIPC")
	}
	if !strings.Contains(authHelperSource, "func routeRuntimeAuthMiddleware(") {
		t.Fatalf("expected runtime_route_auth_helpers.go to contain routeRuntimeAuthMiddleware")
	}
	if !strings.Contains(authHelperSource, "func routeRuntimePageMiddleware(") {
		t.Fatalf("expected runtime_route_auth_helpers.go to contain routeRuntimePageMiddleware")
	}

	forbiddenContract := []string{
		"type routeRuntimeContractMediaOptions struct {",
		"func newRouteRuntimeMediaConfigStore(",
		"func bindRouteRuntimeMediaWebPush(",
		"func registerRouteRuntimeMediaIPC(",
	}
	for _, token := range forbiddenContract {
		if strings.Contains(contractSource, token) {
			t.Fatalf("expected runtime_media_contract.go to delegate token %q", token)
		}
	}
}
