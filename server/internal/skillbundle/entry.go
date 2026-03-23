package skillbundle

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var (
	ErrEntryDocumentNotFound = errors.New("skill bundle does not contain an entry document")
	entryDocPriority         = []string{"SKILL.md", "CLAUDE.md", "AGENT.md"}
)

type EntryDocument struct {
	Dir       string
	Path      string
	Name      string
	Depth     int
	MatchesID bool
}

func EntryDocumentNames() []string {
	names := make([]string, len(entryDocPriority))
	copy(names, entryDocPriority)
	return names
}

func IsEntryDocumentName(name string) bool {
	return entryDocumentRank(name) >= 0
}

func EntryDocumentPriority(name string) int {
	rank := entryDocumentRank(name)
	if rank >= 0 {
		return rank
	}
	return len(entryDocPriority)
}

func FindEntryDocumentInDir(dir string) (*EntryDocument, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var best *EntryDocument
	for _, entry := range entries {
		if entry.IsDir() || !IsEntryDocumentName(entry.Name()) {
			continue
		}
		candidate := &EntryDocument{
			Dir:  dir,
			Path: filepath.Join(dir, entry.Name()),
			Name: entry.Name(),
		}
		if best == nil || compareEntryDocument(candidate, best) < 0 {
			best = candidate
		}
	}
	if best == nil {
		return nil, ErrEntryDocumentNotFound
	}
	return best, nil
}

func FindEntryDocument(root string, skillID string) (*EntryDocument, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if !IsEntryDocumentName(info.Name()) {
			return nil, ErrEntryDocumentNotFound
		}
		dir := filepath.Dir(root)
		rel := "."
		if relPath, err := filepath.Rel(dir, dir); err == nil {
			rel = relPath
		}
		_ = rel
		return &EntryDocument{
			Dir:       dir,
			Path:      root,
			Name:      info.Name(),
			Depth:     0,
			MatchesID: normalizeID(filepath.Base(dir)) == normalizeID(skillID),
		}, nil
	}

	candidates := make([]*EntryDocument, 0, 4)
	if err := filepath.Walk(root, func(path string, walkInfo os.FileInfo, walkErr error) error {
		if walkErr != nil || walkInfo == nil || walkInfo.IsDir() || !IsEntryDocumentName(walkInfo.Name()) {
			return nil
		}
		dir := filepath.Dir(path)
		rel, err := filepath.Rel(root, dir)
		if err != nil {
			return nil
		}
		depth := 0
		if rel != "." {
			depth = len(strings.Split(filepath.ToSlash(rel), "/"))
		}
		candidates = append(candidates, &EntryDocument{
			Dir:       dir,
			Path:      path,
			Name:      walkInfo.Name(),
			Depth:     depth,
			MatchesID: normalizeID(filepath.Base(dir)) == normalizeID(skillID),
		})
		return nil
	}); err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, ErrEntryDocumentNotFound
	}
	sort.Slice(candidates, func(i, j int) bool {
		return compareEntryDocument(candidates[i], candidates[j]) < 0
	})
	return candidates[0], nil
}

func EnsureCompatibilitySkillDoc(dir, entryPath string) (string, error) {
	skillPath := filepath.Join(dir, "SKILL.md")
	if filepath.Clean(entryPath) == filepath.Clean(skillPath) {
		return skillPath, nil
	}
	data, err := os.ReadFile(entryPath)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(skillPath, data, 0o644); err != nil {
		return "", err
	}
	return skillPath, nil
}

func compareEntryDocument(left, right *EntryDocument) int {
	if left == nil && right == nil {
		return 0
	}
	if left == nil {
		return 1
	}
	if right == nil {
		return -1
	}
	if left.MatchesID != right.MatchesID {
		if left.MatchesID {
			return -1
		}
		return 1
	}
	if leftRank, rightRank := EntryDocumentPriority(left.Name), EntryDocumentPriority(right.Name); leftRank != rightRank {
		if leftRank < rightRank {
			return -1
		}
		return 1
	}
	if left.Depth != right.Depth {
		if left.Depth < right.Depth {
			return -1
		}
		return 1
	}
	return strings.Compare(left.Path, right.Path)
}

func entryDocumentRank(name string) int {
	for i, candidate := range entryDocPriority {
		if strings.EqualFold(strings.TrimSpace(name), candidate) {
			return i
		}
	}
	return -1
}

func normalizeID(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, "-", "_")
	return value
}
