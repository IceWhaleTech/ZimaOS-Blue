package skillmarket

import (
	"os"
	"path/filepath"
	"strings"
)

var dependencyManifestNames = []string{
	"package.json",
	"package-lock.json",
	"requirements.txt",
	"requirements-dev.txt",
	"poetry.lock",
	"pyproject.toml",
	"go.mod",
	"go.sum",
	"Cargo.toml",
	"Cargo.lock",
}

func detectInstallSurface(root string) InstallSurface {
	surface := InstallSurface{
		InstallType:  InstallTypeRawSkill,
		ArtifactKind: ArtifactKindOpenSource,
		Installable:  true,
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return normalizeInstallSurface(surface)
	}
	for _, entry := range entries {
		name := entry.Name()
		fullPath := filepath.Join(root, name)
		if entry.IsDir() {
			switch name {
			case "scripts":
				surface.HasScripts = true
				surface.InstallType = InstallTypeScriptPackage
				if hasBinaryInTree(fullPath) {
					surface.HasBinary = true
				}
			default:
				if hasBinaryInTree(fullPath) {
					surface.HasBinary = true
				}
			}
			continue
		}
		if isDependencyManifest(name) {
			surface.DependencyManifests = append(surface.DependencyManifests, name)
		}
		if isBinaryFile(fullPath, name) {
			surface.HasBinary = true
		}
	}
	switch {
	case surface.HasBinary && surface.HasScripts:
		surface.ArtifactKind = ArtifactKindMixed
		surface.InstallType = InstallTypeBinaryPackage
	case surface.HasBinary:
		surface.ArtifactKind = ArtifactKindClosedBinary
		surface.InstallType = InstallTypeBinaryPackage
	case surface.HasScripts:
		surface.ArtifactKind = ArtifactKindOpenSource
		surface.InstallType = InstallTypeScriptPackage
	default:
		surface.ArtifactKind = ArtifactKindOpenSource
	}
	return normalizeInstallSurface(surface)
}

func inferredSurfaceForSource(sourceType, downloadURL string, installable bool) InstallSurface {
	surface := InstallSurface{
		InstallType:  InstallTypeManualExternal,
		ArtifactKind: ArtifactKindUnknown,
		Installable:  installable,
	}
	switch {
	case strings.EqualFold(sourceType, "github_repo"):
		surface.InstallType = InstallTypeGitRepo
		surface.ArtifactKind = ArtifactKindOpenSource
		surface.Installable = true
	case looksLikeArchiveURL(downloadURL):
		surface.InstallType = InstallTypeSourceArchive
		surface.ArtifactKind = ArtifactKindOpenSource
	case looksLikeSkillURL(downloadURL):
		surface.InstallType = InstallTypeRawSkill
		surface.ArtifactKind = ArtifactKindOpenSource
	case installable:
		surface.InstallType = InstallTypeBuiltinCommands
		surface.ArtifactKind = ArtifactKindOpenSource
	}
	return normalizeInstallSurface(surface)
}

func hasBinaryInTree(root string) bool {
	found := false
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if isBinaryFile(path, info.Name()) {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

func isBinaryFile(path, name string) bool {
	lower := strings.ToLower(name)
	for _, suffix := range []string{".exe", ".bin", ".so", ".dylib", ".dll", ".o", ".a"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	if strings.HasPrefix(lower, "install") || strings.HasPrefix(lower, "setup") {
		data, err := os.ReadFile(path)
		if err == nil && len(data) > 0 && strings.IndexByte(string(data[:min(len(data), 256)]), 0) >= 0 {
			return true
		}
	}
	return false
}

func isDependencyManifest(name string) bool {
	lower := strings.ToLower(name)
	for _, candidate := range dependencyManifestNames {
		if lower == candidate {
			return true
		}
	}
	return false
}

func looksLikeArchiveURL(raw string) bool {
	lower := strings.ToLower(raw)
	return strings.HasSuffix(lower, ".zip") || strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz")
}
