package skillbundle

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"
)

func TestFindEntryDocumentPrefersMatchingIDThenPriorityAndDepth(t *testing.T) {
	root := t.TempDir()

	matchingDir := filepath.Join(root, "pkg", "writer")
	if err := os.MkdirAll(matchingDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(matchingDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(matchingDir, "CLAUDE.md"), []byte("# writer\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(matching CLAUDE.md) error = %v", err)
	}

	shallowSkillDir := filepath.Join(root, "pkg", "other")
	if err := os.MkdirAll(shallowSkillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(shallowSkillDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(shallowSkillDir, "SKILL.md"), []byte("# other\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(shallow SKILL.md) error = %v", err)
	}

	deepMatchDir := filepath.Join(root, "nested", "deeper", "writer")
	if err := os.MkdirAll(deepMatchDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(deepMatchDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(deepMatchDir, "SKILL.md"), []byte("# deep writer\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(deep matching SKILL.md) error = %v", err)
	}

	entry, err := FindEntryDocument(root, "writer")
	if err != nil {
		t.Fatalf("FindEntryDocument() error = %v", err)
	}
	if entry.Path != filepath.Join(deepMatchDir, "SKILL.md") {
		t.Fatalf("entry.Path = %q, want %q", entry.Path, filepath.Join(deepMatchDir, "SKILL.md"))
	}
	if !entry.MatchesID {
		t.Fatalf("entry.MatchesID = false, want true")
	}
	if entry.Depth != 3 {
		t.Fatalf("entry.Depth = %d, want 3", entry.Depth)
	}
}

func TestEntrySourceDoesNotUseFilepathWalk(t *testing.T) {
	filePath := filepath.Join(".", "entry.go")
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, filePath, nil, 0)
	if err != nil {
		t.Fatalf("ParseFile(%q) error = %v", filePath, err)
	}

	var foundWalk bool
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := selector.X.(*ast.Ident)
		if !ok {
			return true
		}
		if ident.Name == "filepath" && selector.Sel.Name == "Walk" {
			foundWalk = true
			return false
		}
		return true
	})

	if foundWalk {
		t.Fatalf("entry.go should not call filepath.Walk")
	}
}
