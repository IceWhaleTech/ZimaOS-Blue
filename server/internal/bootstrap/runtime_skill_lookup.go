package bootstrap

import (
	"fmt"
	"strings"

	skillpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmanifest"
)

type runtimeResolvedSkill struct {
	ID    string
	Skill skillpkg.Skill
}

func runtimeSkillExecutionWorkspace(defaultWorkspace string, input map[string]any) string {
	if input != nil {
		if workdir, ok := input["__blue_workdir"].(string); ok && strings.TrimSpace(workdir) != "" {
			return strings.TrimSpace(workdir)
		}
	}
	return strings.TrimSpace(defaultWorkspace)
}

func resolveRuntimeSkillForExecution(
	skills runtimeSkillRegistrySource,
	workspaceDir string,
	requestedID string,
) (runtimeResolvedSkill, error) {
	requestedID = strings.TrimSpace(requestedID)
	if requestedID == "" {
		return runtimeResolvedSkill{}, fmt.Errorf("unknown skill: %s", requestedID)
	}

	candidates := skillmanifest.CandidateIDs(requestedID)
	if resolved, ok, err := resolveManifestBackedRuntimeSkill(skills, workspaceDir, candidates); ok {
		return resolved, err
	}
	if resolved, ok, err := lookupRuntimeSkillInRegistry(skills, candidates); ok {
		return resolved, err
	}
	return runtimeResolvedSkill{}, fmt.Errorf("unknown skill: %s", requestedID)
}

func resolveManifestBackedRuntimeSkill(
	skills runtimeSkillRegistrySource,
	workspaceDir string,
	candidates []string,
) (runtimeResolvedSkill, bool, error) {
	resolvedManifest, ok, err := skillmanifest.FindAnyByCandidatesStrict(candidates, skillmanifest.ResolveRoots(workspaceDir), workspaceDir, skillmanifest.Options{})
	if err != nil {
		return runtimeResolvedSkill{}, true, err
	}
	if !ok {
		return runtimeResolvedSkill{}, false, nil
	}

	canonicalID := strings.TrimSpace(resolvedManifest.Document.ID)
	if canonicalID == "" {
		canonicalID = strings.TrimSpace(resolvedManifest.Document.Name)
	}
	if canonicalID == "" {
		return runtimeResolvedSkill{}, false, nil
	}

	if resolved, ok, err := lookupRuntimeSkillInRegistry(skills, skillmanifest.CandidateIDs(canonicalID)); ok {
		return resolved, true, err
	}
	if runtimeManifestSkillRequiresToolFallback(canonicalID) {
		return runtimeResolvedSkill{}, false, nil
	}

	if !resolvedManifest.Document.Enabled || !skillmanifest.PlatformMatch(resolvedManifest.Document.OS) {
		return runtimeResolvedSkill{ID: canonicalID}, true, fmt.Errorf("skill %s is disabled", canonicalID)
	}
	if resolvedManifest.Document.Manifest == nil {
		return runtimeResolvedSkill{}, false, nil
	}

	return runtimeResolvedSkill{
		ID:    canonicalID,
		Skill: skillpkg.NewManifestSkill(resolvedManifest.Document.Manifest),
	}, true, nil
}

func lookupRuntimeSkillInRegistry(
	skills runtimeSkillRegistrySource,
	candidates []string,
) (runtimeResolvedSkill, bool, error) {
	if skills == nil {
		return runtimeResolvedSkill{}, false, nil
	}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		sk := skills.Get(candidate)
		if sk == nil {
			continue
		}
		if !skills.IsEnabled(candidate) {
			return runtimeResolvedSkill{ID: candidate}, true, fmt.Errorf("skill %s is disabled", candidate)
		}
		return runtimeResolvedSkill{
			ID:    candidate,
			Skill: sk,
		}, true, nil
	}
	return runtimeResolvedSkill{}, false, nil
}

func runtimeManifestSkillRequiresToolFallback(id string) bool {
	switch strings.ToLower(strings.TrimSpace(id)) {
	case "web_query", "computer_use":
		return true
	default:
		return false
	}
}
