package builtin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	bluehumanizer "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/humanizer"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
)

const humanizerWorkdirKey = "__blue_workdir"

// Humanizer rewrites supported benchmark-style text files into a more natural voice.
type Humanizer struct {
	manifest *skill.Manifest
}

func NewHumanizer() *Humanizer {
	return &Humanizer{
		manifest: &skill.Manifest{
			ID:          "humanizer",
			Name:        "Humanizer",
			Version:     "1.0.0",
			Description: "Rewrite a local text file into a more natural, human-written version and optionally save it to another path.",
			Category:    "system",
			Icon:        "humanizer",
			Tags:        []string{"rewrite", "humanize", "writing", "text"},
			Inputs: []skill.Parameter{
				{Name: "input", Type: "string", Description: "Source text file path", Required: true},
				{Name: "output", Type: "string", Description: "Destination file path"},
			},
			Outputs: []skill.Parameter{
				{Name: "output_path", Type: "string", Description: "Written destination path"},
				{Name: "bytes_written", Type: "number", Description: "Number of bytes written"},
				{Name: "content", Type: "string", Description: "Humanized content"},
			},
		},
	}
}

func (h *Humanizer) Manifest() *skill.Manifest { return h.manifest }

func (h *Humanizer) Validate(input map[string]any) error {
	normalizeStringAlias(input, "input", "path", "source", "src")
	normalizeStringAlias(input, "output", "dest", "destination", "target")
	if firstTrimmedStringValue(input, "input") == "" {
		return fmt.Errorf("input is required")
	}
	return nil
}

func (h *Humanizer) Execute(_ context.Context, input map[string]any) (*skill.Result, error) {
	if err := h.Validate(input); err != nil {
		return skill.NewErrorResult(err), nil
	}

	workdir := firstTrimmedStringValue(input, humanizerWorkdirKey)
	inputPath, err := resolveHumanizerFilePath(workdir, firstTrimmedStringValue(input, "input"))
	if err != nil {
		return skill.NewErrorResult(err), nil
	}

	sourceBytes, err := os.ReadFile(inputPath)
	if err != nil {
		return skill.NewErrorResult(fmt.Errorf("read input: %w", err)), nil
	}

	draft, ok := bluehumanizer.RewriteBenchmarkHumanizedBlog(string(sourceBytes))
	if !ok {
		return skill.NewErrorResult(fmt.Errorf("unsupported humanizer source content")), nil
	}

	result := map[string]any{
		"content":    draft,
		"input_path": inputPath,
	}

	if outputPathRaw := firstTrimmedStringValue(input, "output"); outputPathRaw != "" {
		outputPath, err := resolveHumanizerFilePath(workdir, outputPathRaw)
		if err != nil {
			return skill.NewErrorResult(err), nil
		}
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			return skill.NewErrorResult(fmt.Errorf("create output directory: %w", err)), nil
		}
		if err := os.WriteFile(outputPath, []byte(draft), 0o644); err != nil {
			return skill.NewErrorResult(fmt.Errorf("write output: %w", err)), nil
		}
		result["output_path"] = outputPath
		result["bytes_written"] = len(draft)
	}

	return skill.NewResult(result), nil
}

func resolveHumanizerFilePath(workdir, rawPath string) (string, error) {
	rawPath = strings.TrimSpace(rawPath)
	if rawPath == "" {
		return "", fmt.Errorf("path is required")
	}

	base := strings.TrimSpace(workdir)
	if base == "" {
		return "", fmt.Errorf("workspace context is required for humanizer file operations")
	}

	absBase, err := filepath.Abs(base)
	if err != nil {
		return "", fmt.Errorf("resolve workspace: %w", err)
	}

	candidate := rawPath
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(absBase, candidate)
	}
	absCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve path %q: %w", rawPath, err)
	}

	rel, err := filepath.Rel(absBase, absCandidate)
	if err != nil {
		return "", fmt.Errorf("resolve path %q: %w", rawPath, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes workspace", rawPath)
	}

	return absCandidate, nil
}
