package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeRouteRegistrationCorePhaseGo_DelegatesInfrastructureExperienceToolingAndSupport(t *testing.T) {
	mainContent, err := os.ReadFile(filepath.Join("runtime_route_registration_core_phase.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_core_phase.go: %v", err)
	}
	mainSource := string(mainContent)

	infraContent, err := os.ReadFile(filepath.Join("runtime_route_registration_core_infrastructure.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_core_infrastructure.go: %v", err)
	}
	infraSource := string(infraContent)

	infraOptionContent, err := os.ReadFile(filepath.Join("runtime_route_registration_core_infrastructure_options.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_core_infrastructure_options.go: %v", err)
	}
	infraOptionSource := string(infraOptionContent)

	infraTLSOptionContent, err := os.ReadFile(filepath.Join("runtime_route_registration_core_infrastructure_tls_options.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_core_infrastructure_tls_options.go: %v", err)
	}
	infraTLSOptionSource := string(infraTLSOptionContent)

	infraMediaOptionContent, err := os.ReadFile(filepath.Join("runtime_route_registration_core_infrastructure_media_options.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_core_infrastructure_media_options.go: %v", err)
	}
	infraMediaOptionSource := string(infraMediaOptionContent)

	infraGatewayOptionContent, err := os.ReadFile(filepath.Join("runtime_route_registration_core_infrastructure_gateway_options.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_core_infrastructure_gateway_options.go: %v", err)
	}
	infraGatewayOptionSource := string(infraGatewayOptionContent)

	infraChatSurfaceOptionContent, err := os.ReadFile(filepath.Join("runtime_route_registration_core_infrastructure_chat_surface_options.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_core_infrastructure_chat_surface_options.go: %v", err)
	}
	infraChatSurfaceOptionSource := string(infraChatSurfaceOptionContent)

	experienceContent, err := os.ReadFile(filepath.Join("runtime_route_registration_core_experience.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_core_experience.go: %v", err)
	}
	experienceSource := string(experienceContent)

	experienceOptionContent, err := os.ReadFile(filepath.Join("runtime_route_registration_core_experience_options.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_core_experience_options.go: %v", err)
	}
	experienceOptionSource := string(experienceOptionContent)

	experienceChatOptionContent, err := os.ReadFile(filepath.Join("runtime_route_registration_core_experience_chat_options.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_core_experience_chat_options.go: %v", err)
	}
	experienceChatOptionSource := string(experienceChatOptionContent)

	experienceChatBindingOptionContent, err := os.ReadFile(filepath.Join("runtime_route_registration_core_experience_chat_binding_options.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_core_experience_chat_binding_options.go: %v", err)
	}
	experienceChatBindingOptionSource := string(experienceChatBindingOptionContent)

	experienceSurfaceOptionContent, err := os.ReadFile(filepath.Join("runtime_route_registration_core_experience_surface_options.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_core_experience_surface_options.go: %v", err)
	}
	experienceSurfaceOptionSource := string(experienceSurfaceOptionContent)

	experiencePlatformOptionContent, err := os.ReadFile(filepath.Join("runtime_route_registration_core_experience_platform_options.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_core_experience_platform_options.go: %v", err)
	}
	experiencePlatformOptionSource := string(experiencePlatformOptionContent)

	experienceProductivityOptionContent, err := os.ReadFile(filepath.Join("runtime_route_registration_core_experience_productivity_options.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_core_experience_productivity_options.go: %v", err)
	}
	experienceProductivityOptionSource := string(experienceProductivityOptionContent)

	experienceSkillOptionContent, err := os.ReadFile(filepath.Join("runtime_route_registration_core_experience_skill_options.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_core_experience_skill_options.go: %v", err)
	}
	experienceSkillOptionSource := string(experienceSkillOptionContent)

	toolingContent, err := os.ReadFile(filepath.Join("runtime_route_registration_core_tooling.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_core_tooling.go: %v", err)
	}
	toolingSource := string(toolingContent)

	toolingOptionContent, err := os.ReadFile(filepath.Join("runtime_route_registration_core_tooling_options.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_core_tooling_options.go: %v", err)
	}
	toolingOptionSource := string(toolingOptionContent)

	toolingSchedulerOptionContent, err := os.ReadFile(filepath.Join("runtime_route_registration_core_tooling_scheduler_options.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_core_tooling_scheduler_options.go: %v", err)
	}
	toolingSchedulerOptionSource := string(toolingSchedulerOptionContent)

	toolingSurfaceOptionContent, err := os.ReadFile(filepath.Join("runtime_route_registration_core_tooling_surface_options.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_core_tooling_surface_options.go: %v", err)
	}
	toolingSurfaceOptionSource := string(toolingSurfaceOptionContent)

	toolingAnalyzeOptionContent, err := os.ReadFile(filepath.Join("runtime_route_registration_core_tooling_analyze_options.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_core_tooling_analyze_options.go: %v", err)
	}
	toolingAnalyzeOptionSource := string(toolingAnalyzeOptionContent)

	toolingProviderOptionContent, err := os.ReadFile(filepath.Join("runtime_route_registration_core_tooling_provider_options.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_core_tooling_provider_options.go: %v", err)
	}
	toolingProviderOptionSource := string(toolingProviderOptionContent)

	supportContent, err := os.ReadFile(filepath.Join("runtime_route_registration_core_support.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_core_support.go: %v", err)
	}
	supportSource := string(supportContent)

	supportOptionContent, err := os.ReadFile(filepath.Join("runtime_route_registration_core_support_options.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_core_support_options.go: %v", err)
	}
	supportOptionSource := string(supportOptionContent)

	supportAskOptionContent, err := os.ReadFile(filepath.Join("runtime_route_registration_core_support_ask_options.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_core_support_ask_options.go: %v", err)
	}
	supportAskOptionSource := string(supportAskOptionContent)

	supportExecOptionContent, err := os.ReadFile(filepath.Join("runtime_route_registration_core_support_exec_options.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_core_support_exec_options.go: %v", err)
	}
	supportExecOptionSource := string(supportExecOptionContent)

	supportCapabilityOptionContent, err := os.ReadFile(filepath.Join("runtime_route_registration_core_support_capability_options.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_core_support_capability_options.go: %v", err)
	}
	supportCapabilityOptionSource := string(supportCapabilityOptionContent)

	if lines := strings.Count(mainSource, "\n") + 1; lines > 20 {
		t.Fatalf("expected runtime_route_registration_core_phase.go to stay below 20 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(infraSource, "\n") + 1; lines > 45 {
		t.Fatalf("expected runtime_route_registration_core_infrastructure.go to stay below 45 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(infraOptionSource, "\n") + 1; lines > 55 {
		t.Fatalf("expected runtime_route_registration_core_infrastructure_options.go to stay below 55 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(infraTLSOptionSource, "\n") + 1; lines > 12 {
		t.Fatalf("expected runtime_route_registration_core_infrastructure_tls_options.go to stay below 12 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(infraMediaOptionSource, "\n") + 1; lines > 14 {
		t.Fatalf("expected runtime_route_registration_core_infrastructure_media_options.go to stay below 14 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(infraGatewayOptionSource, "\n") + 1; lines > 18 {
		t.Fatalf("expected runtime_route_registration_core_infrastructure_gateway_options.go to stay below 18 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(infraChatSurfaceOptionSource, "\n") + 1; lines > 12 {
		t.Fatalf("expected runtime_route_registration_core_infrastructure_chat_surface_options.go to stay below 12 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(experienceSource, "\n") + 1; lines > 65 {
		t.Fatalf("expected runtime_route_registration_core_experience.go to stay below 65 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(experienceOptionSource, "\n") + 1; lines > 75 {
		t.Fatalf("expected runtime_route_registration_core_experience_options.go to stay below 75 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(experienceChatOptionSource, "\n") + 1; lines > 18 {
		t.Fatalf("expected runtime_route_registration_core_experience_chat_options.go to stay below 18 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(experienceChatBindingOptionSource, "\n") + 1; lines > 10 {
		t.Fatalf("expected runtime_route_registration_core_experience_chat_binding_options.go to stay below 10 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(experienceSurfaceOptionSource, "\n") + 1; lines > 20 {
		t.Fatalf("expected runtime_route_registration_core_experience_surface_options.go to stay below 20 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(experiencePlatformOptionSource, "\n") + 1; lines > 25 {
		t.Fatalf("expected runtime_route_registration_core_experience_platform_options.go to stay below 25 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(experienceProductivityOptionSource, "\n") + 1; lines > 15 {
		t.Fatalf("expected runtime_route_registration_core_experience_productivity_options.go to stay below 15 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(experienceSkillOptionSource, "\n") + 1; lines > 25 {
		t.Fatalf("expected runtime_route_registration_core_experience_skill_options.go to stay below 25 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(toolingSource, "\n") + 1; lines > 20 {
		t.Fatalf("expected runtime_route_registration_core_tooling.go to stay below 20 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(toolingOptionSource, "\n") + 1; lines > 80 {
		t.Fatalf("expected runtime_route_registration_core_tooling_options.go to stay below 80 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(toolingSchedulerOptionSource, "\n") + 1; lines > 28 {
		t.Fatalf("expected runtime_route_registration_core_tooling_scheduler_options.go to stay below 28 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(toolingSurfaceOptionSource, "\n") + 1; lines > 24 {
		t.Fatalf("expected runtime_route_registration_core_tooling_surface_options.go to stay below 24 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(toolingAnalyzeOptionSource, "\n") + 1; lines > 14 {
		t.Fatalf("expected runtime_route_registration_core_tooling_analyze_options.go to stay below 14 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(toolingProviderOptionSource, "\n") + 1; lines > 16 {
		t.Fatalf("expected runtime_route_registration_core_tooling_provider_options.go to stay below 16 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(supportSource, "\n") + 1; lines > 20 {
		t.Fatalf("expected runtime_route_registration_core_support.go to stay below 20 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(supportOptionSource, "\n") + 1; lines > 70 {
		t.Fatalf("expected runtime_route_registration_core_support_options.go to stay below 70 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(supportAskOptionSource, "\n") + 1; lines > 18 {
		t.Fatalf("expected runtime_route_registration_core_support_ask_options.go to stay below 18 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(supportExecOptionSource, "\n") + 1; lines > 35 {
		t.Fatalf("expected runtime_route_registration_core_support_exec_options.go to stay below 35 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(supportCapabilityOptionSource, "\n") + 1; lines > 20 {
		t.Fatalf("expected runtime_route_registration_core_support_capability_options.go to stay below 20 lines after extraction, got %d", lines)
	}

	requiredMain := []string{
		"func bindRouteRuntimeCorePhase(",
		"bindRouteRuntimeCoreInfrastructure(state)",
		"bindRouteRuntimeCoreExperience(state)",
		"bindRouteRuntimeCoreTooling(state)",
		"bindRouteRuntimeCoreSupport(state)",
	}
	for _, token := range requiredMain {
		if !strings.Contains(mainSource, token) {
			t.Fatalf("expected runtime_route_registration_core_phase.go to contain token %q", token)
		}
	}

	requiredInfra := []string{
		"func bindRouteRuntimeCoreInfrastructure(",
		"state.setInfrastructureRuntime(state.runtimeContract.BindInfrastructureRuntime(",
		"newRouteRuntimeCoreInfrastructureOptions(state)",
	}
	for _, token := range requiredInfra {
		if !strings.Contains(infraSource, token) {
			t.Fatalf("expected runtime_route_registration_core_infrastructure.go to contain token %q", token)
		}
	}
	if !strings.Contains(infraOptionSource, "func newRouteRuntimeCoreInfrastructureOptions(") {
		t.Fatal("expected runtime_route_registration_core_infrastructure_options.go to keep infrastructure option assembly")
	}
	for _, token := range []string{
		"newRouteRuntimeInfrastructureTLSOptions(state)",
		"newRouteRuntimeInfrastructureMediaOptions(state)",
		"newRouteRuntimeInfrastructureGatewayOptions(state)",
		"newRouteRuntimeInfrastructureChatSurfaceOptions(state)",
	} {
		if !strings.Contains(infraOptionSource, token) {
			t.Fatalf("expected runtime_route_registration_core_infrastructure_options.go to contain token %q", token)
		}
	}
	for _, token := range []string{
		"func newRouteRuntimeInfrastructureTLSOptions(",
		"routeRuntimeContractTLSOptions{",
		"configKV:",
	} {
		if !strings.Contains(infraTLSOptionSource, token) {
			t.Fatalf("expected runtime_route_registration_core_infrastructure_tls_options.go to contain token %q", token)
		}
	}
	for _, token := range []string{
		"func newRouteRuntimeInfrastructureMediaOptions(",
		"routeRuntimeContractMediaOptions{",
		"requirePagePermission:",
	} {
		if !strings.Contains(infraMediaOptionSource, token) {
			t.Fatalf("expected runtime_route_registration_core_infrastructure_media_options.go to contain token %q", token)
		}
	}
	for _, token := range []string{
		"func newRouteRuntimeInfrastructureGatewayOptions(",
		"routeRuntimeContractGatewayOptions{",
		"closers:",
	} {
		if !strings.Contains(infraGatewayOptionSource, token) {
			t.Fatalf("expected runtime_route_registration_core_infrastructure_gateway_options.go to contain token %q", token)
		}
	}
	for _, token := range []string{
		"func newRouteRuntimeInfrastructureChatSurfaceOptions(",
		"routeRuntimeContractChatSurfaceOptions{",
		"sseBroker:",
	} {
		if !strings.Contains(infraChatSurfaceOptionSource, token) {
			t.Fatalf("expected runtime_route_registration_core_infrastructure_chat_surface_options.go to contain token %q", token)
		}
	}
	for _, token := range []string{
		"routeRuntimeContractTLSOptions{",
		"routeRuntimeContractMediaOptions{",
		"routeRuntimeContractGatewayOptions{",
		"routeRuntimeContractChatSurfaceOptions{",
	} {
		if strings.Contains(infraOptionSource, token) {
			t.Fatalf("expected runtime_route_registration_core_infrastructure_options.go to delegate infrastructure token %q", token)
		}
	}

	requiredExperience := []string{
		"func bindRouteRuntimeCoreExperience(",
		"state.setExperienceRuntime(state.runtimeContract.BindExperienceRuntime(",
		"newRouteRuntimeCoreExperienceOptions(state)",
	}
	for _, token := range requiredExperience {
		if !strings.Contains(experienceSource, token) {
			t.Fatalf("expected runtime_route_registration_core_experience.go to contain token %q", token)
		}
	}
	if !strings.Contains(experienceOptionSource, "func newRouteRuntimeCoreExperienceOptions(") {
		t.Fatal("expected runtime_route_registration_core_experience_options.go to keep experience option assembly")
	}
	for _, token := range []string{
		"newRouteRuntimeExperienceChatOptions(state)",
		"newRouteRuntimeExperienceChatBindingOptions(state)",
		"newRouteRuntimeExperienceSurfaceOptions(state)",
		"newRouteRuntimeExperiencePlatformOptions(state)",
		"newRouteRuntimeExperienceProductivityOptions(state)",
		"newRouteRuntimeExperienceSkillOptions(state)",
	} {
		if !strings.Contains(experienceOptionSource, token) {
			t.Fatalf("expected runtime_route_registration_core_experience_options.go to contain token %q", token)
		}
	}
	for _, token := range []string{
		"func newRouteRuntimeExperienceChatOptions(",
		"routeRuntimeContractChatOptions{",
		"security:",
		"flagEvaluator:",
		"workspaceDir:",
	} {
		if !strings.Contains(experienceChatOptionSource, token) {
			t.Fatalf("expected runtime_route_registration_core_experience_chat_options.go to contain token %q", token)
		}
	}
	for _, token := range []string{
		"func newRouteRuntimeExperienceChatBindingOptions(",
		"routeRuntimeContractChatBindingOptions{",
		"target:",
		"handler:",
	} {
		if !strings.Contains(experienceChatBindingOptionSource, token) {
			t.Fatalf("expected runtime_route_registration_core_experience_chat_binding_options.go to contain token %q", token)
		}
	}
	for _, token := range []string{
		"func newRouteRuntimeExperienceSurfaceOptions(",
		"routeRuntimeContractExperienceSurfaceOptions{",
		"routeRuntimeContractResearchOptions{",
	} {
		if !strings.Contains(experienceSurfaceOptionSource, token) {
			t.Fatalf("expected runtime_route_registration_core_experience_surface_options.go to contain token %q", token)
		}
	}
	for _, token := range []string{
		"func newRouteRuntimeExperiencePlatformOptions(",
		"routeRuntimeContractPlatformSurfaceOptions{",
		"billingPool:",
		"metricsCollector:",
		"connectionManager:",
	} {
		if !strings.Contains(experiencePlatformOptionSource, token) {
			t.Fatalf("expected runtime_route_registration_core_experience_platform_options.go to contain token %q", token)
		}
	}
	for _, token := range []string{
		"func newRouteRuntimeExperienceProductivityOptions(",
		"routeRuntimeContractProductivityOptions{",
		"writeDB:",
		"skillRegistry:",
	} {
		if !strings.Contains(experienceProductivityOptionSource, token) {
			t.Fatalf("expected runtime_route_registration_core_experience_productivity_options.go to contain token %q", token)
		}
	}
	for _, token := range []string{
		"func newRouteRuntimeExperienceSkillOptions(",
		"routeRuntimeContractSkillOptions{",
		"settings:",
		"authPageV1Group:",
		"eventBroker:",
		"skillEmbedFS:",
	} {
		if !strings.Contains(experienceSkillOptionSource, token) {
			t.Fatalf("expected runtime_route_registration_core_experience_skill_options.go to contain token %q", token)
		}
	}
	for _, token := range []string{
		"routeRuntimeContractChatOptions{",
		"flagEvaluator:",
		"routeRuntimeContractChatBindingOptions{",
		"routeRuntimeContractPlatformSurfaceOptions{",
		"routeRuntimeContractProductivityOptions{",
		"routeRuntimeContractSkillOptions{",
		"billingPool:",
		"metricsCollector:",
		"connectionManager:",
		"skillRegistry:",
		"eventBroker:",
		"skillEmbedFS:",
	} {
		if strings.Contains(experienceOptionSource, token) {
			t.Fatalf("expected runtime_route_registration_core_experience_options.go to delegate platform assembly token %q", token)
		}
	}

	requiredTooling := []string{
		"func bindRouteRuntimeCoreTooling(",
		"state.setCoreToolingRuntime(state.runtimeContract.BindCoreToolingRuntime(",
		"newRouteRuntimeCoreToolingOptions(state)",
	}
	for _, token := range requiredTooling {
		if !strings.Contains(toolingSource, token) {
			t.Fatalf("expected runtime_route_registration_core_tooling.go to contain token %q", token)
		}
	}
	if !strings.Contains(toolingOptionSource, "func newRouteRuntimeCoreToolingOptions(") {
		t.Fatal("expected runtime_route_registration_core_tooling_options.go to keep tooling option assembly")
	}
	for _, token := range []string{
		"newRouteRuntimeCoreSchedulerOptions(state)",
		"newRouteRuntimeCoreToolingSurfaceOptions(state)",
		"newRouteRuntimeCoreAnalyzeOptions(state)",
		"newRouteRuntimeCoreProviderOptions(state)",
	} {
		if !strings.Contains(toolingOptionSource, token) {
			t.Fatalf("expected runtime_route_registration_core_tooling_options.go to contain token %q", token)
		}
	}
	for _, token := range []string{
		"func newRouteRuntimeCoreSchedulerOptions(",
		"routeRuntimeContractSchedulerOptions{",
		"workflowResolver:",
		"calendar:",
		"memoryStore:",
	} {
		if !strings.Contains(toolingSchedulerOptionSource, token) {
			t.Fatalf("expected runtime_route_registration_core_tooling_scheduler_options.go to contain token %q", token)
		}
	}
	for _, token := range []string{
		"func newRouteRuntimeCoreToolingSurfaceOptions(",
		"routeRuntimeContractToolingOptions{",
		"workspaceAllowedPaths:",
		"llmRegistry:",
		"voiceSource:",
	} {
		if !strings.Contains(toolingSurfaceOptionSource, token) {
			t.Fatalf("expected runtime_route_registration_core_tooling_surface_options.go to contain token %q", token)
		}
	}
	for _, token := range []string{
		"func newRouteRuntimeCoreAnalyzeOptions(",
		"routeRuntimeContractAnalyzeOptions{",
		"browserBackend:",
		"webSearchConfig:",
	} {
		if !strings.Contains(toolingAnalyzeOptionSource, token) {
			t.Fatalf("expected runtime_route_registration_core_tooling_analyze_options.go to contain token %q", token)
		}
	}
	for _, token := range []string{
		"func newRouteRuntimeCoreProviderOptions(",
		"routeRuntimeContractProviderPoolOptions{",
		"providerPool:",
		"requirePagePermission:",
	} {
		if !strings.Contains(toolingProviderOptionSource, token) {
			t.Fatalf("expected runtime_route_registration_core_tooling_provider_options.go to contain token %q", token)
		}
	}
	for _, token := range []string{
		"routeRuntimeContractSchedulerOptions{",
		"workflowResolver:",
		"routeRuntimeContractToolingOptions{",
		"workspaceAllowedPaths:",
		"routeRuntimeContractAnalyzeOptions{",
		"webSearchConfig:",
		"routeRuntimeContractProviderPoolOptions{",
		"requirePagePermission:",
	} {
		if strings.Contains(toolingOptionSource, token) {
			t.Fatalf("expected runtime_route_registration_core_tooling_options.go to delegate tooling assembly token %q", token)
		}
	}

	requiredSupport := []string{
		"func bindRouteRuntimeCoreSupport(",
		"state.setCoreSupportRuntime(state.runtimeContract.BindCoreSupportRuntime(",
		"newRouteRuntimeCoreSupportOptions(state)",
	}
	for _, token := range requiredSupport {
		if !strings.Contains(supportSource, token) {
			t.Fatalf("expected runtime_route_registration_core_support.go to contain token %q", token)
		}
	}
	if !strings.Contains(supportOptionSource, "func newRouteRuntimeCoreSupportOptions(") {
		t.Fatal("expected runtime_route_registration_core_support_options.go to keep support option assembly")
	}
	for _, token := range []string{
		"newRouteRuntimeCoreAskSupportOptions(state)",
		"newRouteRuntimeCoreExecSupportOptions(state)",
		"newRouteRuntimeCoreCapabilitySupportOptions(state)",
	} {
		if !strings.Contains(supportOptionSource, token) {
			t.Fatalf("expected runtime_route_registration_core_support_options.go to contain token %q", token)
		}
	}
	for _, token := range []string{
		"func newRouteRuntimeCoreAskSupportOptions(",
		"routeRuntimeContractAskSupportOptions{",
		"mediaDir:",
		"timeout:",
	} {
		if !strings.Contains(supportAskOptionSource, token) {
			t.Fatalf("expected runtime_route_registration_core_support_ask_options.go to contain token %q", token)
		}
	}
	for _, token := range []string{
		"func newRouteRuntimeCoreExecSupportOptions(",
		"routeRuntimeContractExecSupportOptions{",
		"workspaceAllowedPath:",
		"profileRoutes:",
		"lookupAPIKey:",
	} {
		if !strings.Contains(supportExecOptionSource, token) {
			t.Fatalf("expected runtime_route_registration_core_support_exec_options.go to contain token %q", token)
		}
	}
	for _, token := range []string{
		"func newRouteRuntimeCoreCapabilitySupportOptions(",
		"routeRuntimeContractCapabilitySupportOptions{",
		"authRouteMiddleware:",
		"flagEvaluator:",
		"trace:",
	} {
		if !strings.Contains(supportCapabilityOptionSource, token) {
			t.Fatalf("expected runtime_route_registration_core_support_capability_options.go to contain token %q", token)
		}
	}

	forbiddenMain := []string{
		".BindTLSRuntime(",
		".ConfigureChatRuntime(",
		".BindSchedulerServices(",
		".NewAskSupportBundle(",
	}
	for _, token := range forbiddenMain {
		if strings.Contains(mainSource, token) {
			t.Fatalf("expected runtime_route_registration_core_phase.go to delegate token %q", token)
		}
	}

	forbiddenSupport := []string{
		".NewAskSupportBundle(",
		".NewExecSupportBundle(",
		".BindCapabilitySupportRuntime(",
	}
	for _, token := range forbiddenSupport {
		if strings.Contains(supportSource, token) {
			t.Fatalf("expected runtime_route_registration_core_support.go to delegate token %q", token)
		}
	}
	for _, token := range []string{
		"routeRuntimeContractAskSupportOptions{",
		"routeRuntimeContractExecSupportOptions{",
		"routeRuntimeContractCapabilitySupportOptions{",
		"profileRoutes:",
		"flagEvaluator:",
	} {
		if strings.Contains(supportOptionSource, token) {
			t.Fatalf("expected runtime_route_registration_core_support_options.go to delegate support assembly token %q", token)
		}
	}

	forbiddenTooling := []string{
		".BindSchedulerServices(",
		".BindTooling(",
		".BindAnalyzeTool(",
		".BindProviderPoolRuntime(",
	}
	for _, token := range forbiddenTooling {
		if strings.Contains(toolingSource, token) {
			t.Fatalf("expected runtime_route_registration_core_tooling.go to delegate token %q", token)
		}
	}
}
