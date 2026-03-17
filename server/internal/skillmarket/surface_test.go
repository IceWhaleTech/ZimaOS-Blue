package skillmarket

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectInstallSurfaceClassifiesScriptsBinariesAndDeps(t *testing.T) {
	root := t.TempDir()
	scriptsDir := filepath.Join(root, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatalf("mkdir scripts: %v", err)
	}
	if err := os.WriteFile(filepath.Join(scriptsDir, "run.sh"), []byte("#!/bin/sh\necho ok\n"), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"name":"fixture"}`), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "installer.bin"), []byte{0x00, 0x01, 0x02}, 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}

	surface := detectInstallSurface(root)

	if !surface.HasScripts {
		t.Fatal("expected scripts to be detected")
	}
	if !surface.HasBinary {
		t.Fatal("expected binary to be detected")
	}
	if surface.InstallType != InstallTypeBinaryPackage {
		t.Fatalf("install type = %q, want %q", surface.InstallType, InstallTypeBinaryPackage)
	}
	if surface.ArtifactKind != ArtifactKindMixed {
		t.Fatalf("artifact kind = %q, want %q", surface.ArtifactKind, ArtifactKindMixed)
	}
	if len(surface.DependencyManifests) != 1 || surface.DependencyManifests[0] != "package.json" {
		t.Fatalf("dependency manifests = %v, want [package.json]", surface.DependencyManifests)
	}
}

func TestInferredSurfaceForSourceRespectsInstallability(t *testing.T) {
	repoSurface := inferredSurfaceForSource("github_repo", "", true)
	if repoSurface.InstallType != InstallTypeGitRepo {
		t.Fatalf("repo install type = %q, want %q", repoSurface.InstallType, InstallTypeGitRepo)
	}
	if repoSurface.ArtifactKind != ArtifactKindOpenSource {
		t.Fatalf("repo artifact kind = %q, want %q", repoSurface.ArtifactKind, ArtifactKindOpenSource)
	}

	manualSurface := inferredSurfaceForSource("html_catalog", "", false)
	if manualSurface.InstallType != InstallTypeManualExternal {
		t.Fatalf("manual install type = %q, want %q", manualSurface.InstallType, InstallTypeManualExternal)
	}
	if manualSurface.Installable {
		t.Fatal("expected manual surface to be non-installable")
	}
}
