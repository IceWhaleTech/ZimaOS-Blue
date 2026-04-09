package bootstrap

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workspace"
)

func TestNewRuntimeCapabilityContract_CreatesSharedRuntimeServices(t *testing.T) {
	tmp := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tmp, "runtime-capability-contract.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{}
	cfg.Harness.Enabled = true
	cfg.Harness.StorePath = filepath.Join(tmp, "blue.db")
	cfg.Harness.ArtifactRoot = filepath.Join(tmp, "artifacts")

	contract := newRuntimeCapabilityContract(
		db,
		db,
		cfg,
		zap.NewNop(),
		&stubRuntimeWorkspaceManagerSource{mgr: workspace.NewManager(tmp)},
		buildWebSearchConfig(cfg),
	)

	if contract.ResearchService() == nil || contract.ReflectService() == nil || contract.HarnessRuntime() == nil {
		t.Fatalf("expected runtime capability contract to initialize research/reflect/harness, got %#v", contract)
	}

	target := &stubChatResearchRuntimeTarget{}
	registry := tools.NewRegistry()
	contract.bindChatResearch(target, sse.NewBroker(), registry, tmp)

	if target.service != contract.ResearchService() || target.serviceCalls != 1 {
		t.Fatalf("expected contract to wire shared deep research service, got %#v", target)
	}
	if target.hook == nil || target.hookCalls != 1 {
		t.Fatalf("expected contract to wire harness-backed turn hook, got %#v", target)
	}
	if registry.Get("research") == nil {
		t.Fatalf("expected research tool to be registered, got %#v", registry.List())
	}
}

func TestRuntimeCapabilitiesGo_DelegatesHarnessAndResearchLanes(t *testing.T) {
	capabilityContent, err := os.ReadFile(filepath.Join("runtime_capabilities.go"))
	if err != nil {
		t.Fatalf("read runtime_capabilities.go: %v", err)
	}
	capabilitySource := string(capabilityContent)

	factoryContent, err := os.ReadFile(filepath.Join("runtime_capability_factory.go"))
	if err != nil {
		t.Fatalf("read runtime_capability_factory.go: %v", err)
	}
	factorySource := string(factoryContent)

	implContent, err := os.ReadFile(filepath.Join("runtime_capability_impl.go"))
	if err != nil {
		t.Fatalf("read runtime_capability_impl.go: %v", err)
	}
	implSource := string(implContent)

	constructorContent, err := os.ReadFile(filepath.Join("runtime_capability_constructor.go"))
	if err != nil {
		t.Fatalf("read runtime_capability_constructor.go: %v", err)
	}
	constructorSource := string(constructorContent)

	typeContent, err := os.ReadFile(filepath.Join("runtime_capability_types.go"))
	if err != nil {
		t.Fatalf("read runtime_capability_types.go: %v", err)
	}
	typeSource := string(typeContent)

	contractContent, err := os.ReadFile(filepath.Join("runtime_capability_contract.go"))
	if err != nil {
		t.Fatalf("read runtime_capability_contract.go: %v", err)
	}
	contractSource := string(contractContent)

	surfaceRuntimeContent, err := os.ReadFile(filepath.Join("runtime_capability_surface_runtime.go"))
	if err != nil {
		t.Fatalf("read runtime_capability_surface_runtime.go: %v", err)
	}
	surfaceRuntimeSource := string(surfaceRuntimeContent)

	surfaceRuntimeChatContent, err := os.ReadFile(filepath.Join("runtime_capability_surface_runtime_chat.go"))
	if err != nil {
		t.Fatalf("read runtime_capability_surface_runtime_chat.go: %v", err)
	}
	surfaceRuntimeChatSource := string(surfaceRuntimeChatContent)

	surfaceRuntimeResearchContent, err := os.ReadFile(filepath.Join("runtime_capability_surface_runtime_research.go"))
	if err != nil {
		t.Fatalf("read runtime_capability_surface_runtime_research.go: %v", err)
	}
	surfaceRuntimeResearchSource := string(surfaceRuntimeResearchContent)

	adapterContent, err := os.ReadFile(filepath.Join("runtime_capability_adapter.go"))
	if err != nil {
		t.Fatalf("read runtime_capability_adapter.go: %v", err)
	}
	adapterSource := string(adapterContent)

	adapterSurfaceContent, err := os.ReadFile(filepath.Join("runtime_capability_adapter_surface.go"))
	if err != nil {
		t.Fatalf("read runtime_capability_adapter_surface.go: %v", err)
	}
	adapterSurfaceSource := string(adapterSurfaceContent)

	adapterRouteContent, err := os.ReadFile(filepath.Join("runtime_capability_adapter_routes.go"))
	if err != nil {
		t.Fatalf("read runtime_capability_adapter_routes.go: %v", err)
	}
	adapterRouteSource := string(adapterRouteContent)

	adapterChatContent, err := os.ReadFile(filepath.Join("runtime_capability_adapter_chat.go"))
	if err != nil {
		t.Fatalf("read runtime_capability_adapter_chat.go: %v", err)
	}
	adapterChatSource := string(adapterChatContent)

	harnessContent, err := os.ReadFile(filepath.Join("runtime_capability_harness.go"))
	if err != nil {
		t.Fatalf("read runtime_capability_harness.go: %v", err)
	}
	harnessSource := string(harnessContent)

	researchContent, err := os.ReadFile(filepath.Join("runtime_capability_research.go"))
	if err != nil {
		t.Fatalf("read runtime_capability_research.go: %v", err)
	}
	researchSource := string(researchContent)

	researchTypeContent, err := os.ReadFile(filepath.Join("runtime_capability_research_types.go"))
	if err != nil {
		t.Fatalf("read runtime_capability_research_types.go: %v", err)
	}
	researchTypeSource := string(researchTypeContent)

	researchBindingContent, err := os.ReadFile(filepath.Join("runtime_capability_research_binding.go"))
	if err != nil {
		t.Fatalf("read runtime_capability_research_binding.go: %v", err)
	}
	researchBindingSource := string(researchBindingContent)

	researchChatContent, err := os.ReadFile(filepath.Join("runtime_capability_research_chat.go"))
	if err != nil {
		t.Fatalf("read runtime_capability_research_chat.go: %v", err)
	}
	researchChatSource := string(researchChatContent)

	if lines := strings.Count(capabilitySource, "\n") + 1; lines > 35 {
		t.Fatalf("expected runtime_capabilities.go to stay below 35 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(factorySource, "\n") + 1; lines > 30 {
		t.Fatalf("expected runtime_capability_factory.go to stay below 30 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(implSource, "\n") + 1; lines > 30 {
		t.Fatalf("expected runtime_capability_impl.go to stay below 30 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(constructorSource, "\n") + 1; lines > 70 {
		t.Fatalf("expected runtime_capability_constructor.go to stay below 70 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(typeSource, "\n") + 1; lines > 40 {
		t.Fatalf("expected runtime_capability_types.go to stay below 40 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(contractSource, "\n") + 1; lines > 35 {
		t.Fatalf("expected runtime_capability_contract.go to stay below 25 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(surfaceRuntimeSource, "\n") + 1; lines > 25 {
		t.Fatalf("expected runtime_capability_surface_runtime.go to stay below 25 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(surfaceRuntimeChatSource, "\n") + 1; lines > 30 {
		t.Fatalf("expected runtime_capability_surface_runtime_chat.go to stay below 30 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(surfaceRuntimeResearchSource, "\n") + 1; lines > 25 {
		t.Fatalf("expected runtime_capability_surface_runtime_research.go to stay below 25 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(adapterSource, "\n") + 1; lines > 15 {
		t.Fatalf("expected runtime_capability_adapter.go to stay below 15 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(adapterSurfaceSource, "\n") + 1; lines > 20 {
		t.Fatalf("expected runtime_capability_adapter_surface.go to stay below 20 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(adapterRouteSource, "\n") + 1; lines > 15 {
		t.Fatalf("expected runtime_capability_adapter_routes.go to stay below 15 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(adapterChatSource, "\n") + 1; lines > 20 {
		t.Fatalf("expected runtime_capability_adapter_chat.go to stay below 20 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(harnessSource, "\n") + 1; lines > 80 {
		t.Fatalf("expected runtime_capability_harness.go to stay below 80 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(researchSource, "\n") + 1; lines > 35 {
		t.Fatalf("expected runtime_capability_research.go to stay below 35 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(researchTypeSource, "\n") + 1; lines > 35 {
		t.Fatalf("expected runtime_capability_research_types.go to stay below 35 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(researchBindingSource, "\n") + 1; lines > 55 {
		t.Fatalf("expected runtime_capability_research_binding.go to stay below 55 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(researchChatSource, "\n") + 1; lines > 55 {
		t.Fatalf("expected runtime_capability_research_chat.go to stay below 55 lines after extraction, got %d", lines)
	}

	requiredCapability := []string{
		"func newRuntimeCapabilityContract(",
		"return newRuntimeCapabilityAdapter(newRuntimeCapabilityImplementation(",
	}
	for _, token := range requiredCapability {
		if !strings.Contains(capabilitySource, token) {
			t.Fatalf("expected runtime_capabilities.go to contain token %q", token)
		}
	}

	requiredFactory := []string{
		"func newRuntimeCapabilitySurface(",
		"return newRuntimeCapabilityContract(",
	}
	for _, token := range requiredFactory {
		if !strings.Contains(factorySource, token) {
			t.Fatalf("expected runtime_capability_factory.go to contain token %q", token)
		}
	}

	requiredImpl := []string{
		"func newRuntimeCapabilityImplementation(",
		"newRuntimeCapabilityServices(",
		"normalizeRuntimeWorkspaceManagerSource(",
		"bindRuntimeCapabilityProposalStore(",
		"bindRuntimeCapabilityWorkspaceManager(",
		"bindRuntimeCapabilityHarness(",
	}
	for _, token := range requiredImpl {
		if !strings.Contains(implSource, token) {
			t.Fatalf("expected runtime_capability_impl.go to contain token %q", token)
		}
	}

	requiredConstructor := []string{
		"func newRuntimeCapabilityServices(",
		"deepresearch.NewService(",
		"selfreflect.NewService(",
		"func bindRuntimeCapabilityProposalStore(",
		"selfreflect.NewSQLiteProposalStoreWithReadDB(",
		"func bindRuntimeCapabilityWorkspaceManager(",
		"func bindRuntimeCapabilityHarness(",
		"newHarnessRuntimeBundleWithReadDB(",
		"newHarnessRuntimeBundle(",
	}
	for _, token := range requiredConstructor {
		if !strings.Contains(constructorSource, token) {
			t.Fatalf("expected runtime_capability_constructor.go to contain token %q", token)
		}
	}

	requiredTypes := []string{
		"type HarnessRuntimeBundle struct {",
		"type runtimeCapabilityContract struct {",
		"type runtimeWorkspaceManagerSource interface {",
	}
	for _, token := range requiredTypes {
		if !strings.Contains(typeSource, token) {
			t.Fatalf("expected runtime_capability_types.go to contain token %q", token)
		}
	}

	requiredContract := []string{
		"type runtimeCapabilitySurface interface {",
		"type runtimeCapabilityResearchSurface interface {",
		"type runtimeCapabilityTaskSurface interface {",
		"runtimeCapabilityBoundary()",
		"HarnessRuntime() *HarnessRuntimeBundle",
		"ResearchService() *deepresearch.Service",
		"ReflectService() *selfreflect.Service",
		"registerTaskSurface(options runtimeTaskSurfaceOptions) runtimeTaskSurfaceRegistration",
		"var _ runtimeCapabilitySurface = runtimeCapabilityAdapter" + "{}",
		"var _ runtimeCapabilityResearchSurface = runtimeCapabilityAdapter" + "{}",
		"var _ runtimeCapabilityTaskSurface = runtimeCapabilityAdapter" + "{}",
	}
	for _, token := range requiredContract {
		if !strings.Contains(contractSource, token) {
			t.Fatalf("expected runtime_capability_contract.go to contain token %q", token)
		}
	}

	requiredSurfaceRuntime := []string{
		"func registerRuntimeTaskSurface(",
		"newRuntimeTaskResearchSurfaceOptions(contract, options)",
	}
	for _, token := range requiredSurfaceRuntime {
		if !strings.Contains(surfaceRuntimeSource, token) {
			t.Fatalf("expected runtime_capability_surface_runtime.go to contain token %q", token)
		}
	}
	for _, token := range []string{
		"func bindRuntimeCapabilityChatResearch(",
		"applyChatResearchRuntimeBinding(binding, chatResearchRuntimeBindingTargets{",
	} {
		if !strings.Contains(surfaceRuntimeChatSource, token) {
			t.Fatalf("expected runtime_capability_surface_runtime_chat.go to contain token %q", token)
		}
	}
	for _, token := range []string{
		"func registerRuntimeTaskResearchSurface(",
		"registerHarnessRuntimeResearchTaskRoutes(",
	} {
		if !strings.Contains(surfaceRuntimeResearchSource, token) {
			t.Fatalf("expected runtime_capability_surface_runtime_research.go to contain token %q", token)
		}
	}

	requiredHarness := []string{
		"func normalizeRuntimeWorkspaceManagerSource(",
		"func harnessRuntimeController(",
		"func harnessRuntimeObserver(",
		"func newHarnessRuntimeBundle(",
		"func newHarnessRuntimeBundleWithReadDB(",
	}
	for _, token := range requiredHarness {
		if !strings.Contains(harnessSource, token) {
			t.Fatalf("expected runtime_capability_harness.go to contain token %q", token)
		}
	}

	requiredAdapter := []string{
		"type runtimeCapabilityAdapter struct {",
		"func newRuntimeCapabilityAdapter(",
	}
	for _, token := range requiredAdapter {
		if !strings.Contains(adapterSource, token) {
			t.Fatalf("expected runtime_capability_adapter.go to contain token %q", token)
		}
	}

	for _, token := range []string{
		"func (adapter runtimeCapabilityAdapter) HarnessRuntime(",
		"func (adapter runtimeCapabilityAdapter) ResearchService(",
		"func (adapter runtimeCapabilityAdapter) ReflectService(",
	} {
		if !strings.Contains(adapterSurfaceSource, token) {
			t.Fatalf("expected runtime_capability_adapter_surface.go to contain token %q", token)
		}
	}

	for _, token := range []string{
		"func (adapter runtimeCapabilityAdapter) registerTaskSurface(",
		"func (adapter runtimeCapabilityAdapter) registerResearchTaskSurface(",
	} {
		if !strings.Contains(adapterRouteSource, token) {
			t.Fatalf("expected runtime_capability_adapter_routes.go to contain token %q", token)
		}
	}

	for _, token := range []string{
		"func (adapter runtimeCapabilityAdapter) bindChatResearch(",
		"bindRuntimeCapabilityChatResearch(adapter, target, broker, registry, workspaceDir)",
	} {
		if !strings.Contains(adapterChatSource, token) {
			t.Fatalf("expected runtime_capability_adapter_chat.go to contain token %q", token)
		}
	}

	requiredResearch := []string{
		"func bindHarnessRuntimeChatResearch(",
		"func bindHarnessRuntimeToResearchService(",
		"bindChatResearchRuntime(handler, service, binding)",
		"registerResearchRuntime(harnessRuntimeController(bundle), binding)",
	}
	for _, token := range requiredResearch {
		if !strings.Contains(researchSource, token) {
			t.Fatalf("expected runtime_capability_research.go to contain token %q", token)
		}
	}

	for _, token := range []string{
		"type researchRuntimeBinding struct {",
		"type chatResearchRuntimeBinding struct {",
		"type researchEventPublisherTarget interface {",
		"type chatResearchRuntimeTarget interface {",
	} {
		if !strings.Contains(researchTypeSource, token) {
			t.Fatalf("expected runtime_capability_research_types.go to contain token %q", token)
		}
	}

	for _, token := range []string{
		"func newResearchRuntimeBinding(",
		"func bindResearchRuntimeService(",
		"func registerResearchRuntime(",
		"harnessdrivers.NewResearchDriver(service, controller)",
	} {
		if !strings.Contains(researchBindingSource, token) {
			t.Fatalf("expected runtime_capability_research_binding.go to contain token %q", token)
		}
	}

	for _, token := range []string{
		"func newChatResearchRuntimeBinding(",
		"func bindChatResearchRuntime(",
		"func registerChatResearchRuntime(",
		"tools.RegisterResearchTools(registry, binding.toolAdapter)",
	} {
		if !strings.Contains(researchChatSource, token) {
			t.Fatalf("expected runtime_capability_research_chat.go to contain token %q", token)
		}
	}

	forbiddenCapability := []string{
		"type HarnessRuntimeBundle struct {",
		"type researchRuntimeBinding struct {",
		"func newResearchRuntimeBinding(",
		"func (contract runtimeCapabilityContract) bindChatResearch(",
		"func newRuntimeCapabilitySurface(",
		"newRuntimeCapabilityServices(",
		"normalizeRuntimeWorkspaceManagerSource(",
		"bindRuntimeCapabilityProposalStore(",
		"bindRuntimeCapabilityWorkspaceManager(",
		"bindRuntimeCapabilityHarness(",
		"selfreflect.NewSQLiteProposalStoreWithReadDB(",
		"newHarnessRuntimeBundleWithReadDB(",
		"newHarnessRuntimeBundle(",
	}
	for _, token := range forbiddenCapability {
		if strings.Contains(capabilitySource, token) {
			t.Fatalf("expected runtime_capabilities.go to delegate token %q", token)
		}
	}

	forbiddenFactory := []string{
		"newRuntimeCapabilityServices(",
		"bindRuntimeCapabilityProposalStore(",
		"bindRuntimeCapabilityWorkspaceManager(",
		"bindRuntimeCapabilityHarness(",
	}
	for _, token := range forbiddenFactory {
		if strings.Contains(factorySource, token) {
			t.Fatalf("expected runtime_capability_factory.go to delegate token %q", token)
		}
	}

	forbiddenImpl := []string{
		"func newRuntimeCapabilityContract(",
		"func newRuntimeCapabilitySurface(",
		"type runtimeCapabilitySurface interface {",
		"func newResearchRuntimeBinding(",
	}
	for _, token := range forbiddenImpl {
		if strings.Contains(implSource, token) {
			t.Fatalf("expected runtime_capability_impl.go to delegate token %q", token)
		}
	}

	forbiddenConstructor := []string{
		"type runtimeCapabilityContract struct {",
		"func normalizeRuntimeWorkspaceManagerSource(",
		"func newResearchRuntimeBinding(",
	}
	for _, token := range forbiddenConstructor {
		if strings.Contains(constructorSource, token) {
			t.Fatalf("expected runtime_capability_constructor.go to delegate token %q", token)
		}
	}

	forbiddenTypes := []string{
		"type runtimeCapabilitySurface interface {",
		"func normalizeRuntimeWorkspaceManagerSource(",
		"func newResearchRuntimeBinding(",
		"func newRuntimeCapabilityContract(",
	}
	for _, token := range forbiddenTypes {
		if strings.Contains(typeSource, token) {
			t.Fatalf("expected runtime_capability_types.go to delegate token %q", token)
		}
	}

	forbiddenContract := []string{
		"type runtimeCapabilityContract struct {",
		"func newRuntimeCapabilityContract(",
		"func newResearchRuntimeBinding(",
		"runtimeCapabilityContract" + "{}",
	}
	for _, token := range forbiddenContract {
		if strings.Contains(contractSource, token) {
			t.Fatalf("expected runtime_capability_contract.go to delegate token %q", token)
		}
	}

	forbiddenSurfaceRuntime := []string{
		"func newRuntimeCapabilityContract(",
		"type runtimeCapabilityContract struct {",
		"func bindRuntimeCapabilityChatResearch(",
		"func registerRuntimeTaskResearchSurface(",
	}
	for _, token := range forbiddenSurfaceRuntime {
		if strings.Contains(surfaceRuntimeSource, token) {
			t.Fatalf("expected runtime_capability_surface_runtime.go to delegate token %q", token)
		}
	}

	forbiddenHarness := []string{
		"func newResearchRuntimeBinding(",
		"func bindHarnessRuntimeChatResearch(",
		"func newRuntimeCapabilityContract(",
	}
	for _, token := range forbiddenHarness {
		if strings.Contains(harnessSource, token) {
			t.Fatalf("expected runtime_capability_harness.go to delegate token %q", token)
		}
	}

	forbiddenResearch := []string{
		"func normalizeRuntimeWorkspaceManagerSource(",
		"func newHarnessRuntimeBundle(",
		"func newRuntimeCapabilityContract(",
		"contract.Research,",
		"contract.Research)",
		"contract.Harness,",
		"contract.Harness)",
		"contract.Reflect,",
		"contract.Reflect)",
		"type researchRuntimeBinding struct {",
		"type chatResearchRuntimeBinding struct {",
		"func newResearchRuntimeBinding(",
		"func newChatResearchRuntimeBinding(",
	}
	for _, token := range forbiddenResearch {
		if strings.Contains(researchSource, token) {
			t.Fatalf("expected runtime_capability_research.go to delegate token %q", token)
		}
	}

	forbiddenAdapter := []string{
		"func newRuntimeCapabilityContract(",
		"func newRuntimeCapabilitySurface(",
		"func (adapter runtimeCapabilityAdapter) HarnessRuntime(",
		"func (adapter runtimeCapabilityAdapter) registerTaskSurface(",
		"func (adapter runtimeCapabilityAdapter) bindChatResearch(",
	}
	for _, token := range forbiddenAdapter {
		if strings.Contains(adapterSource, token) {
			t.Fatalf("expected runtime_capability_adapter.go to delegate token %q", token)
		}
	}
}

func TestRuntimeCapabilityTests_UseSharedHelperInsteadOfInlineStructLiterals(t *testing.T) {
	token := "runtimeCapabilityContract" + "{"
	files, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatalf("glob *_test.go: %v", err)
	}
	for _, name := range files {
		if name == "runtime_capability_test_helpers_test.go" {
			continue
		}
		content, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if strings.Contains(string(content), token) {
			t.Fatalf("expected %s to use shared test helper instead of inline runtimeCapabilityContract literal", name)
		}
	}
}
