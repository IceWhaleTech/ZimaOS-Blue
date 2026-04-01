package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeRouteRegistrationManagementPhaseGo_PersistsManagementSupportSnapshot(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("runtime_route_registration_management_phase.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_management_phase.go: %v", err)
	}
	source := string(content)

	optionContent, err := os.ReadFile(filepath.Join("runtime_route_registration_management_phase_options.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_management_phase_options.go: %v", err)
	}
	optionSource := string(optionContent)

	mgmtContent, err := os.ReadFile(filepath.Join("runtime_route_registration_management_phase_mgmt_options.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_management_phase_mgmt_options.go: %v", err)
	}
	mgmtSource := string(mgmtContent)

	heartbeatContent, err := os.ReadFile(filepath.Join("runtime_route_registration_management_phase_heartbeat_options.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_management_phase_heartbeat_options.go: %v", err)
	}
	heartbeatSource := string(heartbeatContent)

	supportContent, err := os.ReadFile(filepath.Join("runtime_route_registration_management_phase_support_options.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_management_phase_support_options.go: %v", err)
	}
	supportSource := string(supportContent)

	userContent, err := os.ReadFile(filepath.Join("runtime_route_registration_management_phase_user_options.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_management_phase_user_options.go: %v", err)
	}
	userSource := string(userContent)

	channelContent, err := os.ReadFile(filepath.Join("runtime_route_registration_management_phase_channel_options.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_management_phase_channel_options.go: %v", err)
	}
	channelSource := string(channelContent)

	if lines := strings.Count(source, "\n") + 1; lines > 20 {
		t.Fatalf("expected runtime_route_registration_management_phase.go to stay below 20 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(optionSource, "\n") + 1; lines > 95 {
		t.Fatalf("expected runtime_route_registration_management_phase_options.go to stay below 95 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(mgmtSource, "\n") + 1; lines > 14 {
		t.Fatalf("expected runtime_route_registration_management_phase_mgmt_options.go to stay below 14 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(heartbeatSource, "\n") + 1; lines > 28 {
		t.Fatalf("expected runtime_route_registration_management_phase_heartbeat_options.go to stay below 28 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(supportSource, "\n") + 1; lines > 22 {
		t.Fatalf("expected runtime_route_registration_management_phase_support_options.go to stay below 22 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(userSource, "\n") + 1; lines > 24 {
		t.Fatalf("expected runtime_route_registration_management_phase_user_options.go to stay below 24 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(channelSource, "\n") + 1; lines > 18 {
		t.Fatalf("expected runtime_route_registration_management_phase_channel_options.go to stay below 18 lines after extraction, got %d", lines)
	}

	required := []string{
		"func bindRouteRuntimeManagementPhase(",
		"state.setManagementRuntime(state.runtimeContract.BindManagementRuntime(",
		"newRouteRuntimeManagementOptions(state)",
	}
	for _, token := range required {
		if !strings.Contains(source, token) {
			t.Fatalf("expected runtime_route_registration_management_phase.go to contain token %q", token)
		}
	}
	requiredOptions := []string{
		"func newRouteRuntimeManagementOptions(",
		"routeRuntimeContractManagementRuntimeOptions{",
		"newRouteRuntimeManagementMgmtOptions(state)",
		"newRouteRuntimeManagementHeartbeatOptions(state)",
		"newRouteRuntimeManagementSupportOptions(state)",
		"newRouteRuntimeManagementUserOptions(state)",
		"newRouteRuntimeManagementChannelOptions(state)",
	}
	for _, token := range requiredOptions {
		if !strings.Contains(optionSource, token) {
			t.Fatalf("expected runtime_route_registration_management_phase_options.go to contain token %q", token)
		}
	}
	for _, token := range []string{
		"func newRouteRuntimeManagementMgmtOptions(",
		"routeRuntimeContractMgmtOptions{",
		"apiKeyService:",
	} {
		if !strings.Contains(mgmtSource, token) {
			t.Fatalf("expected runtime_route_registration_management_phase_mgmt_options.go to contain token %q", token)
		}
	}
	for _, token := range []string{
		"func newRouteRuntimeManagementHeartbeatOptions(",
		"routeRuntimeContractHeartbeatOptions{",
		"Visibility: heartbeat.VisibilityConfig{",
		"runtimeLLM:",
	} {
		if !strings.Contains(heartbeatSource, token) {
			t.Fatalf("expected runtime_route_registration_management_phase_heartbeat_options.go to contain token %q", token)
		}
	}
	for _, token := range []string{
		"func newRouteRuntimeManagementSupportOptions(",
		"routeRuntimeContractManagementSupportOptions{",
		"providerRegistry:",
		"configKV:",
	} {
		if !strings.Contains(supportSource, token) {
			t.Fatalf("expected runtime_route_registration_management_phase_support_options.go to contain token %q", token)
		}
	}
	for _, token := range []string{
		"func newRouteRuntimeManagementUserOptions(",
		"routeRuntimeContractUserSurfaceOptions{",
		"authPageAPIGroup:",
		"skillsDir:",
	} {
		if !strings.Contains(userSource, token) {
			t.Fatalf("expected runtime_route_registration_management_phase_user_options.go to contain token %q", token)
		}
	}
	for _, token := range []string{
		"func newRouteRuntimeManagementChannelOptions(",
		"routeRuntimeContractChannelOptions{",
		"channelTaskWatcher:",
		"autoreplyService:",
	} {
		if !strings.Contains(channelSource, token) {
			t.Fatalf("expected runtime_route_registration_management_phase_channel_options.go to contain token %q", token)
		}
	}
	for _, token := range []string{
		"routeRuntimeContractMgmtOptions{",
		"routeRuntimeContractHeartbeatOptions{",
		"routeRuntimeContractManagementSupportOptions{",
		"routeRuntimeContractUserSurfaceOptions{",
		"routeRuntimeContractChannelOptions{",
		"Visibility: heartbeat.VisibilityConfig{",
		"providerRegistry:",
		"channelTaskWatcher:",
	} {
		if strings.Contains(optionSource, token) {
			t.Fatalf("expected runtime_route_registration_management_phase_options.go to delegate management assembly token %q", token)
		}
	}

	forbidden := []string{
		".RegisterMgmtTool(",
		".BindHeartbeatRuntime(",
		".BindManagementSupport(",
		".BindUserSurfaceRuntime(",
		".BindMgmtUpgrade(",
		".BindChannelRuntime(",
	}
	for _, token := range forbidden {
		if strings.Contains(source, token) {
			t.Fatalf("expected runtime_route_registration_management_phase.go to delegate token %q", token)
		}
	}
}
