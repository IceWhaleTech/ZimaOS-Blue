package deepresearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type AutoresearchBackendConfig struct {
	Command     string
	Args        []string
	WorkingDir  string
	Timeout     time.Duration
	ArtifactDir string
	Env         map[string]string
}

type AutoresearchBackend struct {
	config AutoresearchBackendConfig
}

func NewAutoresearchBackend(cfg AutoresearchBackendConfig) (*AutoresearchBackend, error) {
	cfg.Command = strings.TrimSpace(cfg.Command)
	if cfg.Command == "" {
		return nil, fmt.Errorf("autoresearch command is required")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 20 * time.Minute
	}
	if strings.TrimSpace(cfg.ArtifactDir) == "" {
		cfg.ArtifactDir = filepath.Join(os.TempDir(), "blue-deepresearch", "experiments")
	}
	return &AutoresearchBackend{config: cfg}, nil
}

func (b *AutoresearchBackend) Run(ctx context.Context, req ExperimentRequest) (*ExperimentResult, error) {
	if b == nil {
		return nil, fmt.Errorf("autoresearch backend is not configured")
	}
	artifactDir, err := b.prepareArtifactDir(req.JobID)
	if err != nil {
		return nil, err
	}

	runCtx := ctx
	var cancel context.CancelFunc
	if b.config.Timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, b.config.Timeout)
		defer cancel()
	}

	payload, err := json.Marshal(buildAutoresearchPayload(req, artifactDir))
	if err != nil {
		return nil, fmt.Errorf("marshal autoresearch payload: %w", err)
	}

	args := expandPlaceholdersSlice(b.config.Args, req, artifactDir)
	cmd := exec.CommandContext(runCtx, b.config.Command, args...)
	if dir := strings.TrimSpace(expandPlaceholders(b.config.WorkingDir, req, artifactDir)); dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(), b.commandEnv(req, artifactDir)...)
	cmd.Stdin = bytes.NewReader(payload)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("autoresearch command failed: %s", msg)
	}

	result, err := parseExperimentResult(stdout.Bytes())
	if err != nil {
		return nil, err
	}
	if result.Metadata == nil {
		result.Metadata = make(map[string]interface{})
	}
	result.Metadata["backend"] = "autoresearch"
	result.Metadata["artifact_dir"] = artifactDir
	if cmd.Dir != "" {
		result.Metadata["working_dir"] = cmd.Dir
	}
	return result, nil
}

func (b *AutoresearchBackend) prepareArtifactDir(jobID string) (string, error) {
	base := strings.TrimSpace(b.config.ArtifactDir)
	if base == "" {
		base = filepath.Join(os.TempDir(), "blue-deepresearch", "experiments")
	}
	artifactDir := filepath.Join(base, sanitizePathFragment(jobID))
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return "", fmt.Errorf("create artifact dir: %w", err)
	}
	return artifactDir, nil
}

func (b *AutoresearchBackend) commandEnv(req ExperimentRequest, artifactDir string) []string {
	env := []string{
		"BLUE_RESEARCH_JOB_ID=" + req.JobID,
		"BLUE_RESEARCH_QUERY=" + req.Query,
		"BLUE_RESEARCH_MODE=" + string(req.Mode),
		"BLUE_RESEARCH_ROUTE_MODE=" + string(req.RouteMode),
		"BLUE_RESEARCH_LANG=" + req.Lang,
		"BLUE_RESEARCH_REPORT_STYLE=" + req.ReportStyle,
		"BLUE_RESEARCH_ARTIFACT_DIR=" + artifactDir,
	}
	for key, value := range b.config.Env {
		env = append(env, key+"="+expandPlaceholders(value, req, artifactDir))
	}
	return env
}

func buildAutoresearchPayload(req ExperimentRequest, artifactDir string) map[string]interface{} {
	payload := map[string]interface{}{
		"job_id":       req.JobID,
		"query":        req.Query,
		"lang":         req.Lang,
		"mode":         req.Mode,
		"route_mode":   req.RouteMode,
		"budget":       req.Budget,
		"report_style": req.ReportStyle,
		"artifact_dir": artifactDir,
	}
	if req.StrictEntity {
		payload["strict_entity"] = true
	}
	if len(req.TimeWindows) > 0 {
		payload["time_windows"] = append([]string(nil), req.TimeWindows...)
	}
	if req.WebReport != nil {
		payload["web_report"] = req.WebReport
	}
	if len(req.WebEvidence) > 0 {
		payload["web_evidence"] = req.WebEvidence
	}
	return payload
}

func parseExperimentResult(raw []byte) (*ExperimentResult, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return &ExperimentResult{Summary: "Experiment completed."}, nil
	}
	var result ExperimentResult
	if err := json.Unmarshal([]byte(trimmed), &result); err == nil {
		return &result, nil
	}
	return &ExperimentResult{Summary: trimmed}, nil
}

func expandPlaceholdersSlice(values []string, req ExperimentRequest, artifactDir string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = expandPlaceholders(value, req, artifactDir)
	}
	return out
}

func expandPlaceholders(value string, req ExperimentRequest, artifactDir string) string {
	replacer := strings.NewReplacer(
		"{job_id}", req.JobID,
		"{query}", req.Query,
		"{lang}", req.Lang,
		"{mode}", string(req.Mode),
		"{route_mode}", string(req.RouteMode),
		"{report_style}", req.ReportStyle,
		"{artifact_dir}", artifactDir,
	)
	return replacer.Replace(value)
}

func sanitizePathFragment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "job"
	}
	mapped := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-', r == '_':
			return r
		default:
			return '_'
		}
	}, value)
	mapped = strings.Trim(mapped, "_")
	if mapped == "" {
		return "job"
	}
	return mapped
}
