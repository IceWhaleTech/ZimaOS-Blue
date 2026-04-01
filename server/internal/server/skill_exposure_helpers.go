package server

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmanifest"
)

type skillExposureMetadata struct {
	Paths            []string
	UserInvocable    bool
	ModelInvocable   bool
	ActivationState  string
	ActivationSource string
}

func defaultSkillExposureMetadata() skillExposureMetadata {
	return skillExposureMetadata{
		UserInvocable:    true,
		ModelInvocable:   true,
		ActivationState:  string(skillmanifest.SkillActivationStateActive),
		ActivationSource: string(skillmanifest.SkillActivationSourceBaseline),
	}
}

func skillExposureMetadataFromDocument(doc skillmanifest.Document) skillExposureMetadata {
	meta := defaultSkillExposureMetadata()
	meta.Paths = append([]string(nil), doc.Paths...)
	meta.UserInvocable = doc.UserInvocable
	meta.ModelInvocable = doc.ModelInvocable
	return meta
}

func skillExposureMetadataFromManifest(manifest *skill.Manifest) skillExposureMetadata {
	meta := defaultSkillExposureMetadata()
	if manifest == nil {
		return meta
	}
	skill.NormalizeManifestDefaults(manifest)
	meta.Paths = append([]string(nil), manifest.Paths...)
	meta.UserInvocable = manifest.UserInvocable
	meta.ModelInvocable = manifest.ModelInvocable
	return meta
}

func applySkillExposureView(meta skillExposureMetadata, view skillmanifest.SkillExposureView) skillExposureMetadata {
	meta.Paths = append([]string(nil), view.Paths...)
	meta.UserInvocable = view.UserInvocable
	meta.ModelInvocable = view.ModelInvocable
	if state := strings.TrimSpace(view.ActivationState); state != "" {
		meta.ActivationState = state
	}
	if source := strings.TrimSpace(view.ActivationSource); source != "" {
		meta.ActivationSource = source
	}
	return meta
}

func workspaceDirFromSkillsDir(skillsDir string) string {
	skillsDir = filepath.Clean(strings.TrimSpace(skillsDir))
	if skillsDir == "" || filepath.Base(skillsDir) != "skills" {
		return ""
	}
	parent := filepath.Dir(skillsDir)
	switch filepath.Base(parent) {
	case ".agents", ".claude":
		return filepath.Clean(filepath.Dir(parent))
	default:
		return ""
	}
}

func skillExposureLookupForSkillsDir(skillsDir string) map[string]skillmanifest.SkillExposureView {
	workspaceDir := workspaceDirFromSkillsDir(skillsDir)
	if workspaceDir == "" {
		return nil
	}
	snapshot, err := skillmanifest.SharedSkillExposureManager(workspaceDir).Snapshot()
	if err != nil {
		return nil
	}
	return skillExposureLookupFromSnapshot(snapshot)
}

func skillExposureLookupFromSnapshot(snapshot skillmanifest.SkillExposureSnapshot) map[string]skillmanifest.SkillExposureView {
	lookup := make(map[string]skillmanifest.SkillExposureView, len(snapshot.VisibleSkills)*2)
	for _, view := range snapshot.VisibleSkills {
		if key := strings.ToLower(strings.TrimSpace(view.Document.ID)); key != "" {
			lookup[key] = view
		}
		if key := strings.ToLower(strings.TrimSpace(view.Document.Name)); key != "" {
			lookup[key] = view
		}
	}
	return lookup
}

func findSkillExposureView(lookup map[string]skillmanifest.SkillExposureView, values ...string) (skillmanifest.SkillExposureView, bool) {
	for _, value := range values {
		key := strings.ToLower(strings.TrimSpace(value))
		if key == "" {
			continue
		}
		if view, ok := lookup[key]; ok {
			return view, true
		}
	}
	return skillmanifest.SkillExposureView{}, false
}

func parseSkillDocumentFromEntryPath(entryPath string) (skillmanifest.Document, bool) {
	entryPath = strings.TrimSpace(entryPath)
	if entryPath == "" {
		return skillmanifest.Document{}, false
	}
	body, err := os.ReadFile(entryPath)
	if err != nil {
		return skillmanifest.Document{}, false
	}
	doc, err := skillmanifest.ParseEntry(filepath.Base(filepath.Dir(entryPath)), entryPath, body, skillmanifest.Options{})
	if err != nil {
		return skillmanifest.Document{}, false
	}
	return doc, true
}
