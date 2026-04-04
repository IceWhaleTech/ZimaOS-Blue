package skillmanifest

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ResolvedSkill holds the canonical skill document and raw manual content
// resolved from workspace/home roots or embedded builtins.
type ResolvedSkill struct {
	Document Document
	Raw      []byte
	Source   string
	Root     string
	EntryDir string
	Embedded bool
}

// CandidateIDs expands a help/selector topic into canonical lookup candidates.
// Dotted topics fall back to their base skill and normalized IDs are included.
func CandidateIDs(topic string) []string {
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return nil
	}

	out := make([]string, 0, 4)
	seen := make(map[string]struct{}, 4)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}

	add(topic)
	add(normalizeSkillID(topic))
	for _, alias := range candidateCompatAliases(topic) {
		add(alias)
	}
	if dot := strings.IndexByte(topic, '.'); dot > 0 {
		base := strings.TrimSpace(topic[:dot])
		add(base)
		add(normalizeSkillID(base))
		for _, alias := range candidateCompatAliases(base) {
			add(alias)
		}
	}
	return out
}

func candidateCompatAliases(topic string) []string {
	switch normalizeSkillID(topic) {
	case "web_search":
		return []string{"web_query"}
	case "mgmt":
		return []string{"config"}
	case "config":
		return []string{"mgmt"}
	default:
		return nil
	}
}

// FindByCandidates resolves the first matching skill using candidate priority
// first, then root precedence workspace > home > builtin.
func FindByCandidates(candidates []string, roots []string, opts Options) (ResolvedSkill, bool) {
	return findByCandidates(candidates, roots, opts, true)
}

// FindAnyByCandidates resolves the first matching skill using candidate
// priority first, then root precedence workspace > home > builtin, without
// filtering out disabled or platform-mismatched skills.
func FindAnyByCandidates(candidates []string, roots []string, opts Options) (ResolvedSkill, bool) {
	return findByCandidates(candidates, roots, opts, false)
}

// FindByCandidatesStrict resolves the first matching skill using candidate
// priority first, then root precedence workspace > home > builtin, but returns
// an explicit error when the requested topic is ambiguous within one scope.
func FindByCandidatesStrict(candidates []string, roots []string, workspaceDir string, opts Options) (ResolvedSkill, bool, error) {
	return findByCandidatesStrict(candidates, roots, workspaceDir, opts, true)
}

// FindAnyByCandidatesStrict resolves the first matching skill like
// FindAnyByCandidates but returns an explicit error on ambiguous matches within
// one scope.
func FindAnyByCandidatesStrict(candidates []string, roots []string, workspaceDir string, opts Options) (ResolvedSkill, bool, error) {
	return findByCandidatesStrict(candidates, roots, workspaceDir, opts, false)
}

func findByCandidates(candidates []string, roots []string, opts Options, requireEnabled bool) (ResolvedSkill, bool) {
	queries := normalizeCandidates(candidates)
	if len(queries) == 0 {
		return ResolvedSkill{}, false
	}

	for _, candidate := range queries {
		for _, root := range roots {
			if resolved, ok := lookupInstalledSkill(root, candidate, opts, requireEnabled); ok {
				return resolved, true
			}
		}
	}

	for _, candidate := range queries {
		for _, root := range roots {
			if resolved, ok := scanInstalledSkillRoot(root, candidate, opts, requireEnabled); ok {
				return resolved, true
			}
		}
	}

	for _, candidate := range queries {
		if resolved, ok := lookupEmbeddedSkill(candidate, opts, requireEnabled); ok {
			return resolved, true
		}
	}

	for _, candidate := range queries {
		if resolved, ok := scanEmbeddedSkills(candidate, opts, requireEnabled); ok {
			return resolved, true
		}
	}

	return ResolvedSkill{}, false
}

type lookupScopeGroup struct {
	Scope string
	Roots []string
}

func findByCandidatesStrict(candidates []string, roots []string, workspaceDir string, opts Options, requireEnabled bool) (ResolvedSkill, bool, error) {
	queries := normalizeCandidates(candidates)
	if len(queries) == 0 {
		return ResolvedSkill{}, false, nil
	}

	scopeGroups := buildLookupScopeGroups(roots, workspaceDir)
	for _, candidate := range queries {
		for _, group := range scopeGroups {
			matches := collectInstalledMatches(group.Roots, candidate, opts, requireEnabled)
			if err := validateLookupMatches(group.Scope, candidate, matches); err != nil {
				return ResolvedSkill{}, false, err
			}
			if len(matches) == 1 {
				return matches[0], true, nil
			}
		}

		embeddedMatches := collectEmbeddedMatches(candidate, opts, requireEnabled)
		if err := validateLookupMatches("builtin", candidate, embeddedMatches); err != nil {
			return ResolvedSkill{}, false, err
		}
		if len(embeddedMatches) == 1 {
			return embeddedMatches[0], true, nil
		}
	}

	return ResolvedSkill{}, false, nil
}

func buildLookupScopeGroups(roots []string, workspaceDir string) []lookupScopeGroup {
	groups := make([]lookupScopeGroup, 0, len(roots))
	indexByScope := make(map[string]int, len(roots))
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		scope := canonicalConflictScope(root, workspaceDir)
		if idx, ok := indexByScope[scope]; ok {
			groups[idx].Roots = append(groups[idx].Roots, root)
			continue
		}
		indexByScope[scope] = len(groups)
		groups = append(groups, lookupScopeGroup{
			Scope: scope,
			Roots: []string{root},
		})
	}
	return groups
}

func collectInstalledMatches(roots []string, candidate string, opts Options, requireEnabled bool) []ResolvedSkill {
	out := make([]ResolvedSkill, 0, 4)
	seenSources := make(map[string]struct{}, 4)
	for _, root := range roots {
		root = strings.TrimSpace(root)
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
			if err != nil || (requireEnabled && (!bundle.Document.Enabled || !PlatformMatch(bundle.Document.OS))) {
				continue
			}
			if !candidateMatches(candidate, entry.Name(), bundle.Document.ID, bundle.Document.Name) {
				continue
			}
			source := strings.TrimSpace(bundle.Document.Location)
			if _, ok := seenSources[source]; ok {
				continue
			}
			seenSources[source] = struct{}{}
			out = append(out, ResolvedSkill{
				Document: bundle.Document,
				Raw:      append([]byte(nil), bundle.Raw...),
				Source:   bundle.Document.Location,
				Root:     root,
				EntryDir: bundle.Root,
			})
		}
	}
	return out
}

func collectEmbeddedMatches(candidate string, opts Options, requireEnabled bool) []ResolvedSkill {
	ids, err := ListEmbeddedIDs()
	if err != nil {
		return nil
	}
	out := make([]ResolvedSkill, 0, 2)
	for _, id := range ids {
		doc, raw, err := ReadEmbedded(id, opts)
		if err != nil || (requireEnabled && (!doc.Enabled || !PlatformMatch(doc.OS))) {
			continue
		}
		if !candidateMatches(candidate, id, doc.ID, doc.Name) {
			continue
		}
		out = append(out, ResolvedSkill{
			Document: doc,
			Raw:      append([]byte(nil), raw...),
			Source:   doc.Location,
			Embedded: true,
		})
	}
	return out
}

func validateLookupMatches(scope, candidate string, matches []ResolvedSkill) error {
	if len(matches) < 2 {
		return nil
	}

	conflictsByCanonical := make(map[string]map[string]struct{}, len(matches))
	for _, match := range matches {
		canonicalID := canonicalConflictID(match.Document, filepath.Base(match.EntryDir))
		source := strings.TrimSpace(match.Source)
		if canonicalID == "" || source == "" {
			continue
		}
		if _, ok := conflictsByCanonical[canonicalID]; !ok {
			conflictsByCanonical[canonicalID] = make(map[string]struct{})
		}
		conflictsByCanonical[canonicalID][source] = struct{}{}
	}

	conflicts := make([]CanonicalConflict, 0, len(conflictsByCanonical))
	for canonicalID, sourcesSet := range conflictsByCanonical {
		if len(sourcesSet) < 2 {
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
	if len(conflicts) > 0 {
		sort.Slice(conflicts, func(i, j int) bool {
			return conflicts[i].CanonicalID < conflicts[j].CanonicalID
		})
		return &ConflictError{Conflicts: conflicts}
	}

	summaries := make([]string, 0, len(matches))
	for _, match := range matches {
		label := strings.TrimSpace(firstNonBlank(match.Document.ID, match.Document.Name, filepath.Base(match.EntryDir)))
		if label == "" {
			label = "skill"
		}
		source := strings.TrimSpace(match.Source)
		if source == "" {
			source = label
		}
		summaries = append(summaries, fmt.Sprintf("%s (%s)", label, source))
	}
	sort.Strings(summaries)
	scope = strings.TrimSpace(scope)
	if scope == "" {
		scope = "skill"
	}
	return fmt.Errorf("%s skill lookup for %q is ambiguous: %s", scope, strings.TrimSpace(candidate), strings.Join(summaries, ", "))
}

func normalizeCandidates(candidates []string) []string {
	out := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates)*2)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}

	for _, candidate := range candidates {
		add(candidate)
		add(normalizeSkillID(candidate))
	}
	return out
}

func lookupInstalledSkill(root, candidate string, opts Options, requireEnabled bool) (ResolvedSkill, bool) {
	root = strings.TrimSpace(root)
	candidate = strings.TrimSpace(candidate)
	if root == "" || candidate == "" {
		return ResolvedSkill{}, false
	}
	bundle, err := ValidateInstalledDir(filepath.Join(root, candidate), "", opts)
	if err != nil || (requireEnabled && (!bundle.Document.Enabled || !PlatformMatch(bundle.Document.OS))) {
		return ResolvedSkill{}, false
	}
	return ResolvedSkill{
		Document: bundle.Document,
		Raw:      append([]byte(nil), bundle.Raw...),
		Source:   bundle.Document.Location,
		Root:     root,
		EntryDir: bundle.Root,
	}, true
}

func scanInstalledSkillRoot(root, candidate string, opts Options, requireEnabled bool) (ResolvedSkill, bool) {
	root = strings.TrimSpace(root)
	candidate = strings.TrimSpace(candidate)
	if root == "" || candidate == "" {
		return ResolvedSkill{}, false
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return ResolvedSkill{}, false
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		bundle, err := ValidateInstalledDir(filepath.Join(root, entry.Name()), "", opts)
		if err != nil || (requireEnabled && (!bundle.Document.Enabled || !PlatformMatch(bundle.Document.OS))) {
			continue
		}
		if !candidateMatches(candidate, entry.Name(), bundle.Document.ID, bundle.Document.Name) {
			continue
		}
		return ResolvedSkill{
			Document: bundle.Document,
			Raw:      append([]byte(nil), bundle.Raw...),
			Source:   bundle.Document.Location,
			Root:     root,
			EntryDir: bundle.Root,
		}, true
	}
	return ResolvedSkill{}, false
}

func lookupEmbeddedSkill(candidate string, opts Options, requireEnabled bool) (ResolvedSkill, bool) {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return ResolvedSkill{}, false
	}
	doc, raw, err := ReadEmbedded(candidate, opts)
	if err != nil || (requireEnabled && (!doc.Enabled || !PlatformMatch(doc.OS))) {
		return ResolvedSkill{}, false
	}
	return ResolvedSkill{
		Document: doc,
		Raw:      append([]byte(nil), raw...),
		Source:   doc.Location,
		Embedded: true,
	}, true
}

func scanEmbeddedSkills(candidate string, opts Options, requireEnabled bool) (ResolvedSkill, bool) {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return ResolvedSkill{}, false
	}
	ids, err := ListEmbeddedIDs()
	if err != nil {
		return ResolvedSkill{}, false
	}
	for _, id := range ids {
		doc, raw, err := ReadEmbedded(id, opts)
		if err != nil || (requireEnabled && (!doc.Enabled || !PlatformMatch(doc.OS))) {
			continue
		}
		if !candidateMatches(candidate, id, doc.ID, doc.Name) {
			continue
		}
		return ResolvedSkill{
			Document: doc,
			Raw:      append([]byte(nil), raw...),
			Source:   doc.Location,
			Embedded: true,
		}, true
	}
	return ResolvedSkill{}, false
}

func candidateMatches(candidate string, aliases ...string) bool {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return false
	}
	normalizedCandidate := normalizeSkillID(candidate)
	for _, alias := range aliases {
		alias = strings.TrimSpace(alias)
		if alias == "" {
			continue
		}
		if strings.EqualFold(candidate, alias) {
			return true
		}
		if normalizeSkillID(alias) == normalizedCandidate {
			return true
		}
	}
	return false
}
