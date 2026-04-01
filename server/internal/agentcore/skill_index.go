package agentcore

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmanifest"
)

// SkillDoc is a compact, selector-friendly representation of a skill.
type SkillDoc struct {
	ID               string
	Name             string
	Description      string
	Tags             []string
	Paths            []string
	UserInvocable    bool
	ModelInvocable   bool
	ActivationState  string
	ActivationSource string
	Category         string
	Environment      []string
	Example          string
	Invocation       string
	Examples         []string
	InteractionMode  string
	CardSupport      string
	Setup            string
	ScriptPaths      []string
	InstallSteps     []string
	UsageSteps       []string
	TaskRoutes       []SkillRoute
	ErrorRules       []SkillError
	Body             string
	SourcePath       string
}

// BuildSkillIndex scans workspace and user default skill roots and builds a
// deduplicated index by skill name (workspace root wins on conflicts).
func BuildSkillIndex(workspaceDir string) ([]SkillDoc, error) {
	if err := skillmanifest.ValidateCanonicalConflicts(workspaceDir, skillmanifest.Options{}, true); err != nil {
		return nil, err
	}

	roots := resolveSkillRoots(workspaceDir)
	seen := make(map[string]struct{})
	docs := make([]SkillDoc, 0, 64)
	appendDoc := func(doc SkillDoc, aliases ...string) {
		if strings.TrimSpace(doc.Name) == "" {
			return
		}
		canonicalSeen := false
		canonicalKeys := []string{doc.Name, doc.ID}
		for _, alias := range canonicalKeys {
			key := strings.ToLower(strings.TrimSpace(alias))
			if key == "" {
				continue
			}
			if _, ok := seen[key]; ok {
				canonicalSeen = true
				break
			}
		}
		for _, alias := range append(aliases, canonicalKeys...) {
			key := strings.ToLower(strings.TrimSpace(alias))
			if key == "" {
				continue
			}
			seen[key] = struct{}{}
		}
		if canonicalSeen {
			return
		}
		docs = append(docs, doc)
	}

	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			skillID := entry.Name()
			if _, ok := seen[strings.ToLower(skillID)]; ok {
				continue
			}
			se, data, err := readSkillEntryFromDir(filepath.Join(root, skillID))
			if err != nil {
				continue
			}
			if !se.Enabled || !skillPlatformMatch(se.OS) {
				continue
			}

			doc := buildSkillDoc(se, data)
			appendDoc(doc, skillID)
		}
	}

	embeddedIDs, err := skillmanifest.ListEmbeddedIDs()
	if err == nil {
		for _, skillID := range embeddedIDs {
			if _, ok := seen[strings.ToLower(strings.TrimSpace(skillID))]; ok {
				continue
			}
			doc, _, readErr := skillmanifest.ReadEmbedded(skillID, skillmanifest.Options{})
			if readErr != nil {
				continue
			}
			se := skillEntryFromDocument(doc)
			if !se.Enabled || !skillPlatformMatch(se.OS) {
				continue
			}
			appendDoc(buildSkillDoc(se, nil), skillID)
		}
	}

	sort.Slice(docs, func(i, j int) bool {
		pi, pj := skillSortPriority(docs[i].Name), skillSortPriority(docs[j].Name)
		if pi != pj {
			return pi < pj
		}
		return docs[i].Name < docs[j].Name
	})

	return docs, nil
}

func BuildSkillIndexFromViews(views []skillmanifest.SkillExposureView) []SkillDoc {
	if len(views) == 0 {
		return nil
	}

	docs := make([]SkillDoc, 0, len(views))
	for _, view := range views {
		doc := buildSkillDoc(skillEntryFromDocument(view.Document), view.Raw)
		doc.ActivationState = strings.TrimSpace(view.ActivationState)
		doc.ActivationSource = strings.TrimSpace(view.ActivationSource)
		docs = append(docs, doc)
	}
	return docs
}

func buildSkillDoc(se SkillEntry, raw []byte) SkillDoc {
	content := string(raw)
	body := content
	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) >= 3 {
			body = strings.TrimSpace(parts[2])
		}
	}

	example := strings.TrimSpace(se.Example)
	if example == "" {
		example = extractSkillExample(body, se.Name)
	}
	if se.Description == "" {
		se.Description = extractFirstParagraph(body)
	}

	return SkillDoc{
		ID:              strings.TrimSpace(firstNonBlank(se.ID, se.Name)),
		Name:            strings.TrimSpace(se.Name),
		Description:     strings.TrimSpace(se.Description),
		Tags:            append([]string(nil), se.Tags...),
		Paths:           append([]string(nil), se.Paths...),
		UserInvocable:   se.UserInvocable,
		ModelInvocable:  se.ModelInvocable,
		Category:        strings.TrimSpace(se.Category),
		Environment:     append([]string(nil), se.Environment...),
		Example:         truncateForIndex(example, 180),
		Invocation:      strings.TrimSpace(se.Invocation),
		Examples:        append([]string(nil), se.Examples...),
		InteractionMode: strings.TrimSpace(se.InteractionMode),
		CardSupport:     strings.TrimSpace(se.CardSupport),
		Setup:           se.Setup,
		ScriptPaths:     append([]string(nil), se.ScriptPaths...),
		InstallSteps:    append([]string(nil), se.InstallSteps...),
		UsageSteps:      append([]string(nil), se.UsageSteps...),
		TaskRoutes:      append([]SkillRoute(nil), se.TaskRoutes...),
		ErrorRules:      append([]SkillError(nil), se.ErrorRules...),
		Body:            truncateForIndex(body, 400),
		SourcePath:      se.Location,
	}
}

func extractSkillExample(body, skillName string) string {
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "blue ") || strings.HasPrefix(trimmed, skillName+" ") {
			return truncateForIndex(trimmed, 180)
		}
		if strings.Contains(trimmed, "`blue ") || strings.Contains(trimmed, "`"+skillName+" ") {
			trimmed = strings.Trim(trimmed, "`")
			return truncateForIndex(trimmed, 180)
		}
	}
	return ""
}

func truncateForIndex(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	return strings.TrimSpace(s[:max-3]) + "..."
}
