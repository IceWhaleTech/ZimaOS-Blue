package skillmanifest

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type CanonicalConflict struct {
	Scope       string
	CanonicalID string
	Sources     []string
}

type ConflictError struct {
	Conflicts []CanonicalConflict
}

func (e *ConflictError) Error() string {
	if e == nil || len(e.Conflicts) == 0 {
		return "canonical skill conflicts detected"
	}
	parts := make([]string, 0, len(e.Conflicts))
	for _, conflict := range e.Conflicts {
		scope := strings.TrimSpace(conflict.Scope)
		if scope == "" {
			scope = "skill"
		}
		parts = append(parts, fmt.Sprintf("%s canonical skill %q declared by %s", scope, conflict.CanonicalID, strings.Join(conflict.Sources, ", ")))
	}
	return "canonical skill conflicts detected: " + strings.Join(parts, "; ")
}

type canonicalConflictTarget struct {
	Scope    string
	Root     string
	Embedded bool
}

func DetectCanonicalConflicts(workspaceDir string, opts Options, requireEnabled bool) ([]CanonicalConflict, error) {
	return detectCanonicalConflicts(buildDefaultConflictTargets(workspaceDir), opts, requireEnabled)
}

func ValidateCanonicalConflicts(workspaceDir string, opts Options, requireEnabled bool) error {
	conflicts, err := DetectCanonicalConflicts(workspaceDir, opts, requireEnabled)
	if err != nil {
		return err
	}
	if len(conflicts) == 0 {
		return nil
	}
	return &ConflictError{Conflicts: conflicts}
}

func detectCanonicalConflicts(targets []canonicalConflictTarget, opts Options, requireEnabled bool) ([]CanonicalConflict, error) {
	seen := make(map[string]map[string]struct{})
	for _, target := range targets {
		if target.Embedded {
			ids, err := ListEmbeddedIDs()
			if err != nil {
				return nil, err
			}
			for _, id := range ids {
				doc, _, err := ReadEmbedded(id, opts)
				if err != nil {
					continue
				}
				if requireEnabled && (!doc.Enabled || !PlatformMatch(doc.OS)) {
					continue
				}
				recordCanonicalConflictSource(seen, target.Scope, canonicalConflictID(doc, id), doc.Location)
			}
			continue
		}

		root := strings.TrimSpace(target.Root)
		if root == "" {
			continue
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			bundle, err := ValidateInstalledDir(filepath.Join(root, entry.Name()), "", opts)
			if err != nil {
				continue
			}
			if requireEnabled && (!bundle.Document.Enabled || !PlatformMatch(bundle.Document.OS)) {
				continue
			}
			recordCanonicalConflictSource(seen, target.Scope, canonicalConflictID(bundle.Document, entry.Name()), bundle.Document.Location)
		}
	}

	conflicts := make([]CanonicalConflict, 0, len(seen))
	for key, sourcesSet := range seen {
		if len(sourcesSet) < 2 {
			continue
		}
		scope, canonicalID, ok := strings.Cut(key, "\x00")
		if !ok {
			continue
		}
		sources := make([]string, 0, len(sourcesSet))
		for source := range sourcesSet {
			sources = append(sources, source)
		}
		sort.Strings(sources)
		conflicts = append(conflicts, CanonicalConflict{
			Scope:       scope,
			CanonicalID: canonicalID,
			Sources:     sources,
		})
	}
	sort.Slice(conflicts, func(i, j int) bool {
		if conflicts[i].Scope != conflicts[j].Scope {
			return conflicts[i].Scope < conflicts[j].Scope
		}
		return conflicts[i].CanonicalID < conflicts[j].CanonicalID
	})
	return conflicts, nil
}

func buildDefaultConflictTargets(workspaceDir string) []canonicalConflictTarget {
	targets := make([]canonicalConflictTarget, 0, 5)
	for _, root := range ResolveRoots(workspaceDir) {
		targets = append(targets, canonicalConflictTarget{
			Scope: canonicalConflictScope(root, workspaceDir),
			Root:  root,
		})
	}
	targets = append(targets, canonicalConflictTarget{Scope: "builtin", Embedded: true})
	return targets
}

func canonicalConflictScope(root, workspaceDir string) string {
	root = filepath.Clean(strings.TrimSpace(root))
	if root == "" {
		return "installed"
	}
	if workspaceDir = strings.TrimSpace(workspaceDir); workspaceDir != "" {
		if root == filepath.Clean(filepath.Join(workspaceDir, ".agents", "skills")) {
			return "workspace/.agents"
		}
		if root == filepath.Clean(filepath.Join(workspaceDir, ".claude", "skills")) {
			return "workspace/.claude"
		}
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		if root == filepath.Clean(filepath.Join(home, ".agents", "skills")) {
			return "home/.agents"
		}
		if root == filepath.Clean(filepath.Join(home, ".claude", "skills")) {
			return "home/.claude"
		}
	}
	return root
}

func canonicalConflictID(doc Document, fallback string) string {
	return normalizeSkillID(firstNonBlank(doc.ID, doc.Name, fallback))
}

func recordCanonicalConflictSource(target map[string]map[string]struct{}, scope, canonicalID, source string) {
	scope = strings.TrimSpace(scope)
	canonicalID = normalizeSkillID(canonicalID)
	source = strings.TrimSpace(source)
	if scope == "" || canonicalID == "" || source == "" {
		return
	}
	key := scope + "\x00" + canonicalID
	if _, ok := target[key]; !ok {
		target[key] = make(map[string]struct{})
	}
	target[key][source] = struct{}{}
}
