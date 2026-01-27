package plugin

import (
	"testing"
)

func TestDependencyResolver_Resolve_NoDependencies(t *testing.T) {
	plugins := map[string]*PluginInfo{
		"plugin-a": {Manifest: &Manifest{ID: "plugin-a", Version: "1.0.0"}},
		"plugin-b": {Manifest: &Manifest{ID: "plugin-b", Version: "1.0.0"}},
		"plugin-c": {Manifest: &Manifest{ID: "plugin-c", Version: "1.0.0"}},
	}

	resolver := NewDependencyResolver(plugins)
	result := resolver.Resolve()

	if len(result.Errors) > 0 {
		t.Errorf("expected no errors, got %v", result.Errors)
	}
	if len(result.Order) != 3 {
		t.Errorf("expected 3 plugins in order, got %d", len(result.Order))
	}
}

func TestDependencyResolver_Resolve_SimpleDependency(t *testing.T) {
	plugins := map[string]*PluginInfo{
		"plugin-a": {Manifest: &Manifest{
			ID:      "plugin-a",
			Version: "1.0.0",
			Dependencies: []Dependency{
				{ID: "plugin-b"},
			},
		}},
		"plugin-b": {Manifest: &Manifest{ID: "plugin-b", Version: "1.0.0"}},
	}

	resolver := NewDependencyResolver(plugins)
	result := resolver.Resolve()

	if len(result.Errors) > 0 {
		t.Errorf("expected no errors, got %v", result.Errors)
	}

	// plugin-b should come before plugin-a
	bIndex := -1
	aIndex := -1
	for i, id := range result.Order {
		if id == "plugin-b" {
			bIndex = i
		}
		if id == "plugin-a" {
			aIndex = i
		}
	}

	if bIndex >= aIndex {
		t.Errorf("plugin-b should come before plugin-a, got b=%d, a=%d", bIndex, aIndex)
	}
}

func TestDependencyResolver_Resolve_ChainDependency(t *testing.T) {
	plugins := map[string]*PluginInfo{
		"plugin-a": {Manifest: &Manifest{
			ID:      "plugin-a",
			Version: "1.0.0",
			Dependencies: []Dependency{
				{ID: "plugin-b"},
			},
		}},
		"plugin-b": {Manifest: &Manifest{
			ID:      "plugin-b",
			Version: "1.0.0",
			Dependencies: []Dependency{
				{ID: "plugin-c"},
			},
		}},
		"plugin-c": {Manifest: &Manifest{ID: "plugin-c", Version: "1.0.0"}},
	}

	resolver := NewDependencyResolver(plugins)
	result := resolver.Resolve()

	if len(result.Errors) > 0 {
		t.Errorf("expected no errors, got %v", result.Errors)
	}

	// Order should be: c, b, a
	cIndex := -1
	bIndex := -1
	aIndex := -1
	for i, id := range result.Order {
		switch id {
		case "plugin-c":
			cIndex = i
		case "plugin-b":
			bIndex = i
		case "plugin-a":
			aIndex = i
		}
	}

	if cIndex >= bIndex || bIndex >= aIndex {
		t.Errorf("expected order c < b < a, got c=%d, b=%d, a=%d", cIndex, bIndex, aIndex)
	}
}

func TestDependencyResolver_Resolve_MissingDependency(t *testing.T) {
	plugins := map[string]*PluginInfo{
		"plugin-a": {Manifest: &Manifest{
			ID:      "plugin-a",
			Version: "1.0.0",
			Dependencies: []Dependency{
				{ID: "plugin-missing"},
			},
		}},
	}

	resolver := NewDependencyResolver(plugins)
	result := resolver.Resolve()

	if len(result.Errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(result.Errors))
	}
	if result.Errors[0].DependencyID != "plugin-missing" {
		t.Errorf("expected dependency 'plugin-missing', got %s", result.Errors[0].DependencyID)
	}
}

func TestDependencyResolver_Resolve_OptionalMissingDependency(t *testing.T) {
	plugins := map[string]*PluginInfo{
		"plugin-a": {Manifest: &Manifest{
			ID:      "plugin-a",
			Version: "1.0.0",
			Dependencies: []Dependency{
				{ID: "plugin-missing", Optional: true},
			},
		}},
	}

	resolver := NewDependencyResolver(plugins)
	result := resolver.Resolve()

	if len(result.Errors) > 0 {
		t.Errorf("expected no errors for optional dependency, got %v", result.Errors)
	}
	if len(result.Warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(result.Warnings))
	}
}

func TestDependencyResolver_Resolve_CircularDependency(t *testing.T) {
	plugins := map[string]*PluginInfo{
		"plugin-a": {Manifest: &Manifest{
			ID:      "plugin-a",
			Version: "1.0.0",
			Dependencies: []Dependency{
				{ID: "plugin-b"},
			},
		}},
		"plugin-b": {Manifest: &Manifest{
			ID:      "plugin-b",
			Version: "1.0.0",
			Dependencies: []Dependency{
				{ID: "plugin-a"},
			},
		}},
	}

	resolver := NewDependencyResolver(plugins)
	result := resolver.Resolve()

	if len(result.Errors) == 0 {
		t.Error("expected circular dependency error")
	}

	found := false
	for _, err := range result.Errors {
		if err.Reason == "circular dependency detected" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'circular dependency detected' error")
	}
}

func TestDependencyResolver_Resolve_VersionConstraint(t *testing.T) {
	tests := []struct {
		name       string
		version    string
		constraint string
		shouldPass bool
	}{
		{"exact match", "1.0.0", "1.0.0", true},
		{"exact match with v prefix", "v1.0.0", "1.0.0", true},
		{"exact mismatch", "1.0.0", "2.0.0", false},
		{"caret major", "1.5.0", "^1.0.0", true},
		{"caret major fail", "2.0.0", "^1.0.0", false},
		{"tilde minor", "1.2.5", "~1.2.0", true},
		{"tilde minor fail", "1.3.0", "~1.2.0", false},
		{"greater than", "2.0.0", ">1.0.0", true},
		{"greater than fail", "1.0.0", ">1.0.0", false},
		{"greater or equal", "1.0.0", ">=1.0.0", true},
		{"less than", "0.9.0", "<1.0.0", true},
		{"less than fail", "1.0.0", "<1.0.0", false},
		{"less or equal", "1.0.0", "<=1.0.0", true},
		{"range", "1.5.0", ">=1.0.0 <2.0.0", true},
		{"range fail", "2.0.0", ">=1.0.0 <2.0.0", false},
		{"wildcard", "5.0.0", "*", true},
		{"empty constraint", "1.0.0", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugins := map[string]*PluginInfo{
				"plugin-a": {Manifest: &Manifest{
					ID:      "plugin-a",
					Version: "1.0.0",
					Dependencies: []Dependency{
						{ID: "plugin-b", Version: tt.constraint},
					},
				}},
				"plugin-b": {Manifest: &Manifest{ID: "plugin-b", Version: tt.version}},
			}

			resolver := NewDependencyResolver(plugins)
			result := resolver.Resolve()

			if tt.shouldPass && len(result.Errors) > 0 {
				t.Errorf("expected no errors, got %v", result.Errors)
			}
			if !tt.shouldPass && len(result.Errors) == 0 {
				t.Error("expected version constraint error")
			}
		})
	}
}

func TestDependencyResolver_GetDependencies(t *testing.T) {
	plugins := map[string]*PluginInfo{
		"plugin-a": {Manifest: &Manifest{
			ID:      "plugin-a",
			Version: "1.0.0",
			Dependencies: []Dependency{
				{ID: "plugin-b"},
				{ID: "plugin-c"},
			},
		}},
		"plugin-b": {Manifest: &Manifest{ID: "plugin-b", Version: "1.0.0"}},
		"plugin-c": {Manifest: &Manifest{ID: "plugin-c", Version: "1.0.0"}},
	}

	resolver := NewDependencyResolver(plugins)
	deps := resolver.GetDependencies("plugin-a")

	if len(deps) != 2 {
		t.Errorf("expected 2 dependencies, got %d", len(deps))
	}
}

func TestDependencyResolver_GetDependents(t *testing.T) {
	plugins := map[string]*PluginInfo{
		"plugin-a": {Manifest: &Manifest{
			ID:      "plugin-a",
			Version: "1.0.0",
			Dependencies: []Dependency{
				{ID: "plugin-c"},
			},
		}},
		"plugin-b": {Manifest: &Manifest{
			ID:      "plugin-b",
			Version: "1.0.0",
			Dependencies: []Dependency{
				{ID: "plugin-c"},
			},
		}},
		"plugin-c": {Manifest: &Manifest{ID: "plugin-c", Version: "1.0.0"}},
	}

	resolver := NewDependencyResolver(plugins)
	dependents := resolver.GetDependents("plugin-c")

	if len(dependents) != 2 {
		t.Errorf("expected 2 dependents, got %d", len(dependents))
	}
}

func TestDependencyResolver_CheckDependenciesSatisfied(t *testing.T) {
	plugins := map[string]*PluginInfo{
		"plugin-a": {Manifest: &Manifest{
			ID:      "plugin-a",
			Version: "1.0.0",
			Dependencies: []Dependency{
				{ID: "plugin-b"},
			},
		}},
		"plugin-b": {Manifest: &Manifest{ID: "plugin-b", Version: "1.0.0"}, Status: StatusLoaded},
	}

	resolver := NewDependencyResolver(plugins)
	satisfied, errors := resolver.CheckDependenciesSatisfied("plugin-a")

	if !satisfied {
		t.Errorf("expected dependencies to be satisfied, got errors: %v", errors)
	}
}

func TestDependencyResolver_CheckDependenciesSatisfied_ErrorState(t *testing.T) {
	plugins := map[string]*PluginInfo{
		"plugin-a": {Manifest: &Manifest{
			ID:      "plugin-a",
			Version: "1.0.0",
			Dependencies: []Dependency{
				{ID: "plugin-b"},
			},
		}},
		"plugin-b": {Manifest: &Manifest{ID: "plugin-b", Version: "1.0.0"}, Status: StatusError},
	}

	resolver := NewDependencyResolver(plugins)
	satisfied, errors := resolver.CheckDependenciesSatisfied("plugin-a")

	if satisfied {
		t.Error("expected dependencies to not be satisfied when dependency is in error state")
	}
	if len(errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(errors))
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input      string
		major      int
		minor      int
		patch      int
		prerelease string
		shouldFail bool
	}{
		{"1.0.0", 1, 0, 0, "", false},
		{"v1.0.0", 1, 0, 0, "", false},
		{"1.2.3", 1, 2, 3, "", false},
		{"1.2", 1, 2, 0, "", false},
		{"1", 1, 0, 0, "", false},
		{"1.0.0-alpha", 1, 0, 0, "alpha", false},
		{"1.0.0-beta.1", 1, 0, 0, "beta.1", false},
		{"1.0.0+build", 1, 0, 0, "", false},
		{"1.0.0-alpha+build", 1, 0, 0, "alpha", false},
		{"invalid", 0, 0, 0, "", true},
		{"", 0, 0, 0, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			v, err := parseVersion(tt.input)

			if tt.shouldFail {
				if err == nil {
					t.Error("expected error")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if v.major != tt.major {
				t.Errorf("expected major %d, got %d", tt.major, v.major)
			}
			if v.minor != tt.minor {
				t.Errorf("expected minor %d, got %d", tt.minor, v.minor)
			}
			if v.patch != tt.patch {
				t.Errorf("expected patch %d, got %d", tt.patch, v.patch)
			}
			if v.prerelease != tt.prerelease {
				t.Errorf("expected prerelease %s, got %s", tt.prerelease, v.prerelease)
			}
		})
	}
}

func TestSemverCompare(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected int // -1, 0, 1
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.0", "2.0.0", -1},
		{"2.0.0", "1.0.0", 1},
		{"1.1.0", "1.0.0", 1},
		{"1.0.1", "1.0.0", 1},
		{"1.0.0-alpha", "1.0.0", -1},
		{"1.0.0", "1.0.0-alpha", 1},
		{"1.0.0-alpha", "1.0.0-beta", -1},
	}

	for _, tt := range tests {
		t.Run(tt.v1+" vs "+tt.v2, func(t *testing.T) {
			v1, _ := parseVersion(tt.v1)
			v2, _ := parseVersion(tt.v2)

			result := v1.compare(v2)

			// Normalize to -1, 0, 1
			if result < 0 {
				result = -1
			} else if result > 0 {
				result = 1
			}

			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestDependencyError_Error(t *testing.T) {
	err := &DependencyError{
		PluginID:    "plugin-a",
		DependencyID: "plugin-b",
		Reason:      "not found",
	}

	expected := "plugin plugin-a: dependency plugin-b: not found"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}
