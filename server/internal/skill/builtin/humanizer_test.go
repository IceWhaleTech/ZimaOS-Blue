package builtin

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHumanizerExecute_RewritesBenchmarkBlogToOutputFile(t *testing.T) {
	workdir := t.TempDir()
	inputPath := filepath.Join(workdir, "ai_blog.txt")
	outputPath := filepath.Join(workdir, "humanized_blog.txt")

	source := strings.Join([]string{
		"# 7 Powerful Strategies to Boost Your Productivity and Achieve Your Goals",
		"## 1. Prioritize Your Tasks Effectively",
		"## 2. Eliminate Distractions from Your Environment",
		"## 3. Leverage the Power of Time Blocking",
		"## 4. Take Regular Breaks to Recharge",
		"## 5. Utilize Technology to Your Advantage",
		"## 6. Establish Clear Goals and Objectives",
		"## 7. Maintain a Healthy Work-Life Balance",
	}, "\n")
	if err := os.WriteFile(inputPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	skill := NewHumanizer()
	result, err := skill.Execute(context.Background(), map[string]any{
		humanizerWorkdirKey: workdir,
		"input":             "ai_blog.txt",
		"output":            "humanized_blog.txt",
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got error=%q", result.Error)
	}

	written, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	text := string(written)
	for _, needle := range []string{
		"# 7 Productivity Habits That Actually Help You Get More Done",
		"## 1. Start With the Work That Matters Most",
		"## 7. Protect Your Work-Life Balance",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("expected output to contain %q, got=%q", needle, text)
		}
	}
}

func TestHumanizerExecute_BlocksWorkspaceEscape(t *testing.T) {
	skill := NewHumanizer()
	result, err := skill.Execute(context.Background(), map[string]any{
		humanizerWorkdirKey: t.TempDir(),
		"input":             "../escape.txt",
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Success {
		t.Fatal("expected workspace escape to fail")
	}
	if !strings.Contains(result.Error, "escapes workspace") {
		t.Fatalf("error = %q, want escape message", result.Error)
	}
}
