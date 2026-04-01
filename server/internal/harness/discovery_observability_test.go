package harness

import "testing"

func TestDecodeDiscoverFirstObservation_PrefersDiscoverFirstFields(t *testing.T) {
	structured := map[string]interface{}{
		"selected_canonical_skill":       "web_query",
		"selected_native_surface_mode":   "skill_exec",
		"selected_native_surface_reason": "discover_first_cutover",
		"execution_profile":              "prefer_fork",
		"discovery_runtime": map[string]interface{}{
			"canonical_target":     "browser",
			"selected_native_mode": "legacy",
			"surface_reason":       "legacy_native_surface",
			"execution_profile":    "inline",
		},
	}

	observation := decodeDiscoverFirstObservation(structured)
	if observation.SelectedCanonicalSkill != "web_query" {
		t.Fatalf("SelectedCanonicalSkill = %q, want web_query", observation.SelectedCanonicalSkill)
	}
	if observation.NativeSurfaceMode != "skill_exec" {
		t.Fatalf("NativeSurfaceMode = %q, want skill_exec", observation.NativeSurfaceMode)
	}
	if observation.NativeSurfaceReason != "discover_first_cutover" {
		t.Fatalf("NativeSurfaceReason = %q, want discover_first_cutover", observation.NativeSurfaceReason)
	}
	if observation.ExecutionProfile != "prefer_fork" {
		t.Fatalf("ExecutionProfile = %q, want prefer_fork", observation.ExecutionProfile)
	}
}

func TestDecodeDiscoverFirstObservation_FallsBackToLegacyFields(t *testing.T) {
	structured := map[string]interface{}{
		"canonical_skill_id": "web_search",
		"skill_decision": map[string]interface{}{
			"selected_skill": "web_search",
		},
		"discovery_runtime": map[string]interface{}{
			"selected_native_mode": "legacy_native",
			"surface_reason":       "legacy_exec_collapse_compat",
		},
		"discovery_decision": map[string]interface{}{
			"execution_profile": "prefer_fork",
		},
	}

	observation := decodeDiscoverFirstObservation(structured)
	if observation.SelectedCanonicalSkill != "web_query" {
		t.Fatalf("SelectedCanonicalSkill = %q, want web_query", observation.SelectedCanonicalSkill)
	}
	if observation.NativeSurfaceMode != "legacy" {
		t.Fatalf("NativeSurfaceMode = %q, want legacy", observation.NativeSurfaceMode)
	}
	if observation.NativeSurfaceReason != "legacy_exec_collapse_compat" {
		t.Fatalf("NativeSurfaceReason = %q, want legacy_exec_collapse_compat", observation.NativeSurfaceReason)
	}
	if observation.ExecutionProfile != "prefer_fork" {
		t.Fatalf("ExecutionProfile = %q, want prefer_fork", observation.ExecutionProfile)
	}
}
