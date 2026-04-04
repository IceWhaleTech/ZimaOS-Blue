package optimization

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type EvolvablePart string

const (
	EvolvablePartConstraints        EvolvablePart = "constraints"
	EvolvablePartSkillDefinition    EvolvablePart = "skill_definition"
	EvolvablePartPromptTemplate     EvolvablePart = "prompt_template"
	EvolvablePartContextAssembly    EvolvablePart = "context_assembly"
	EvolvablePartCoordinatorPolicy  EvolvablePart = "coordinator_policy"
	EvolvablePartOrchestratorPolicy EvolvablePart = "orchestrator_policy"
	EvolvablePartToolExposure       EvolvablePart = "tool_exposure"
	EvolvablePartVerificationPolicy EvolvablePart = "verification_policy"
	EvolvablePartRunnerCode         EvolvablePart = "runner_code"
	EvolvablePartBuildRecipe        EvolvablePart = "build_recipe"
)

const RunnerArtifactManifestSchemaVersion = "v1"

type RunnerArtifactManifest struct {
	SchemaVersion         string    `json:"schema_version"`
	BinarySHA256          string    `json:"binary_sha256,omitempty"`
	RepoURL               string    `json:"repo_url,omitempty"`
	Ref                   string    `json:"ref,omitempty"`
	Commit                string    `json:"commit,omitempty"`
	SupportedParts        []string  `json:"supported_parts"`
	OptimizedParts        []string  `json:"optimized_parts"`
	PrimaryPart           string    `json:"primary_part,omitempty"`
	SourceOptimizationRun string    `json:"source_optimization_run_id,omitempty"`
	SourceEvalRun         string    `json:"source_eval_run_id,omitempty"`
	BuiltAt               time.Time `json:"built_at,omitempty"`
}

var defaultEvolvableParts = []EvolvablePart{
	EvolvablePartConstraints,
	EvolvablePartSkillDefinition,
	EvolvablePartPromptTemplate,
	EvolvablePartContextAssembly,
	EvolvablePartCoordinatorPolicy,
	EvolvablePartOrchestratorPolicy,
	EvolvablePartToolExposure,
	EvolvablePartVerificationPolicy,
	EvolvablePartRunnerCode,
	EvolvablePartBuildRecipe,
}

func DefaultSupportedEvolvableParts() []string {
	out := make([]string, 0, len(defaultEvolvableParts))
	for _, part := range defaultEvolvableParts {
		out = append(out, string(part))
	}
	return out
}

func NormalizeRequestedEvolvableParts(raw []string) ([]string, error) {
	return normalizeRequestedEvolvableParts(raw, true)
}

func RunnerArtifactManifestPath(binaryPath string) string {
	binaryPath = strings.TrimSpace(binaryPath)
	if binaryPath == "" {
		return ""
	}
	return binaryPath + ".manifest.json"
}

func NormalizeStatusEvolvableParts(status *Status) {
	if status == nil {
		return
	}
	supported := DefaultSupportedEvolvableParts()
	if normalizedSupported, _ := normalizeRequestedEvolvableParts(status.SupportedParts, false); len(normalizedSupported) > 0 {
		supported = normalizedSupported
	}
	optimized, _ := normalizeRequestedEvolvableParts(status.OptimizedParts, false)
	if len(optimized) == 0 {
		if primary := strings.TrimSpace(status.PrimaryPart); primary != "" {
			optimized = []string{primary}
		}
	}
	if manifestPath := RunnerArtifactManifestPath(status.BinaryPath); manifestPath != "" {
		status.ManifestPath = manifestPath
		if manifest, err := readRunnerArtifactManifest(manifestPath); err == nil && manifest != nil {
			if normalizedSupported, _ := normalizeRequestedEvolvableParts(manifest.SupportedParts, false); len(normalizedSupported) > 0 {
				supported = normalizedSupported
			}
			if len(optimized) == 0 {
				if normalizedOptimized, _ := normalizeRequestedEvolvableParts(manifest.OptimizedParts, false); len(normalizedOptimized) > 0 {
					optimized = normalizedOptimized
				}
			}
			if status.PrimaryPart == "" {
				status.PrimaryPart = firstNonEmptyString(
					strings.TrimSpace(manifest.PrimaryPart),
					firstStringFromSlice(optimized),
				)
			}
			if status.SourceOptimizationRunID == "" {
				status.SourceOptimizationRunID = strings.TrimSpace(manifest.SourceOptimizationRun)
			}
			if status.SourceEvalRunID == "" {
				status.SourceEvalRunID = strings.TrimSpace(manifest.SourceEvalRun)
			}
		}
	}
	if status.PrimaryPart == "" {
		status.PrimaryPart = firstStringFromSlice(optimized)
	}
	if len(optimized) == 0 {
		optimized = []string{}
	}
	status.SupportedParts = append([]string{}, supported...)
	status.OptimizedParts = append([]string{}, optimized...)
}

func NormalizeOptimizationRunRecordEvolvableParts(record OptimizationRunRecord) OptimizationRunRecord {
	if len(record) == 0 {
		return record
	}
	cloned := cloneOptimizationRunRecord(record)
	manifestPath := optimizationRecordString(cloned["manifest_path"])
	if manifestPath == "" {
		manifestPath = RunnerArtifactManifestPath(optimizationRecordString(cloned["runner_artifact_path"]))
	}
	enrichOptimizationRunRecordEvolvableParts(cloned, manifestPath)
	return cloned
}

func normalizeRequestedEvolvableParts(raw []string, strict bool) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	seen := make(map[string]struct{}, len(raw))
	unknown := make([]string, 0)
	for _, item := range raw {
		part := strings.TrimSpace(item)
		if part == "" {
			continue
		}
		if !isKnownEvolvablePart(part) {
			if strict {
				return nil, fmt.Errorf("unsupported requested part %q", part)
			}
			if _, ok := seen[part]; !ok {
				seen[part] = struct{}{}
				unknown = append(unknown, part)
			}
			continue
		}
		seen[part] = struct{}{}
	}
	if len(seen) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(seen))
	for _, part := range defaultEvolvableParts {
		name := string(part)
		if _, ok := seen[name]; ok {
			out = append(out, name)
			delete(seen, name)
		}
	}
	if !strict && len(unknown) > 0 {
		out = append(out, unknown...)
	}
	return out, nil
}

func isKnownEvolvablePart(part string) bool {
	for _, candidate := range defaultEvolvableParts {
		if string(candidate) == part {
			return true
		}
	}
	return false
}

func firstStringFromSlice(values []string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func cloneOptimizationRunRecord(record OptimizationRunRecord) OptimizationRunRecord {
	if len(record) == 0 {
		return record
	}
	cloned := make(OptimizationRunRecord, len(record))
	for key, value := range record {
		cloned[key] = value
	}
	return cloned
}

func enrichOptimizationRunRecordEvolvableParts(record OptimizationRunRecord, manifestPath string) {
	if len(record) == 0 {
		return
	}
	supported := DefaultSupportedEvolvableParts()
	optimized, _ := normalizeRequestedEvolvableParts(optimizationRecordStrings(record["optimized_parts"]), false)

	if len(optimized) == 0 {
		if primary := optimizationRecordString(record["primary_part"]); primary != "" {
			optimized = []string{primary}
		}
	}
	if len(optimized) == 0 {
		if legacy := optimizationRecordString(record["optimization_surface"]); legacy != "" {
			optimized = []string{legacy}
		}
	}
	if manifestPath != "" {
		record["manifest_path"] = manifestPath
		if manifest, err := readRunnerArtifactManifest(manifestPath); err == nil && manifest != nil {
			if normalizedSupported, _ := normalizeRequestedEvolvableParts(manifest.SupportedParts, false); len(normalizedSupported) > 0 {
				supported = normalizedSupported
			}
			if len(optimized) == 0 {
				if normalizedOptimized, _ := normalizeRequestedEvolvableParts(manifest.OptimizedParts, false); len(normalizedOptimized) > 0 {
					optimized = normalizedOptimized
				}
			}
			if optimizationRecordString(record["primary_part"]) == "" {
				record["primary_part"] = firstNonEmptyString(
					strings.TrimSpace(manifest.PrimaryPart),
					firstStringFromSlice(optimized),
				)
			}
			if optimizationRecordString(record["source_optimization_run_id"]) == "" && strings.TrimSpace(manifest.SourceOptimizationRun) != "" {
				record["source_optimization_run_id"] = strings.TrimSpace(manifest.SourceOptimizationRun)
			}
			if optimizationRecordString(record["source_eval_run_id"]) == "" && strings.TrimSpace(manifest.SourceEvalRun) != "" {
				record["source_eval_run_id"] = strings.TrimSpace(manifest.SourceEvalRun)
			}
		}
	}
	if len(optimized) == 0 {
		optimized = []string{}
	}
	record["optimized_parts"] = optimized
	if optimizationRecordString(record["primary_part"]) == "" {
		record["primary_part"] = firstStringFromSlice(optimized)
	}
	if optimizationRecordString(record["optimization_surface"]) == "" {
		record["optimization_surface"] = firstStringFromSlice(optimized)
	}
	if optimizationRecordString(record["source_optimization_run_id"]) == "" {
		record["source_optimization_run_id"] = firstNonEmptyString(
			optimizationRecordString(record["id"]),
			optimizationRecordString(record["last_optimization_run_id"]),
		)
	}
	if optimizationRecordString(record["source_eval_run_id"]) == "" {
		record["source_eval_run_id"] = optimizationRecordString(record["eval_run_id"])
	}
	record["supported_parts"] = supported
}

func optimizationRecordStrings(value interface{}) []string {
	switch typed := value.(type) {
	case []string:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if item = strings.TrimSpace(item); item != "" {
				out = append(out, item)
			}
		}
		return out
	case []interface{}:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := strings.TrimSpace(fmt.Sprint(item)); text != "" && text != "<nil>" {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

func readRunnerArtifactManifest(path string) (*RunnerArtifactManifest, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, os.ErrNotExist
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var manifest RunnerArtifactManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	if manifest.SchemaVersion == "" {
		manifest.SchemaVersion = RunnerArtifactManifestSchemaVersion
	}
	return &manifest, nil
}

func writeRunnerArtifactManifest(binaryPath string, manifest RunnerArtifactManifest) (string, error) {
	path := RunnerArtifactManifestPath(binaryPath)
	if path == "" {
		return "", fmt.Errorf("binary path is required")
	}
	manifest.SchemaVersion = firstNonEmptyString(manifest.SchemaVersion, RunnerArtifactManifestSchemaVersion)
	if normalizedSupported, _ := normalizeRequestedEvolvableParts(manifest.SupportedParts, false); len(normalizedSupported) > 0 {
		manifest.SupportedParts = normalizedSupported
	} else {
		manifest.SupportedParts = DefaultSupportedEvolvableParts()
	}
	normalizedOptimized, err := normalizeRequestedEvolvableParts(manifest.OptimizedParts, false)
	if err != nil {
		return "", err
	}
	if len(normalizedOptimized) == 0 {
		normalizedOptimized = []string{}
	}
	manifest.OptimizedParts = normalizedOptimized
	if manifest.PrimaryPart == "" {
		manifest.PrimaryPart = firstStringFromSlice(manifest.OptimizedParts)
	}
	if manifest.BuiltAt.IsZero() {
		manifest.BuiltAt = time.Now().UTC()
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}
