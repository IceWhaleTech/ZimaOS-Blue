package bootstrap

import (
	"fmt"
	"strings"

	skillpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmanifest"
)

func ensureRuntimeManagedSkillRegistered(registry *skillpkg.Registry, workspaceDir, rawID string) (string, error) {
	return ensureRuntimeManagedSkillRegisteredWithRoots(registry, skillmanifest.ResolveRoots(workspaceDir), rawID)
}

func ensureRuntimeManagedSkillRegisteredWithRoots(registry *skillpkg.Registry, roots []string, rawID string) (string, error) {
	id, doc, err := resolveRuntimeManagedSkillIDWithRoots(registry, roots, rawID)
	if err != nil {
		return "", err
	}
	if registry == nil {
		return id, nil
	}
	if registry.Get(id) != nil {
		return id, nil
	}
	if doc == nil || doc.Manifest == nil {
		return "", fmt.Errorf("skill %s is not registered", id)
	}
	if err := registry.Register(skillpkg.NewManifestSkill(doc.Manifest), false); err != nil {
		return "", err
	}
	if !doc.Enabled {
		if err := registry.Disable(id); err != nil {
			return "", err
		}
	}
	return id, nil
}

func resolveRuntimeManagedSkillIDWithRoots(registry *skillpkg.Registry, roots []string, rawID string) (string, *skillmanifest.Document, error) {
	rawID = strings.TrimSpace(rawID)
	if rawID == "" {
		return "", nil, fmt.Errorf("skill id is required")
	}

	resolved, ok, err := skillmanifest.FindAnyByCandidatesStrict(skillmanifest.CandidateIDs(rawID), roots, "", skillmanifest.Options{})
	if err != nil {
		return "", nil, err
	}
	if ok {
		doc := resolved.Document
		id := strings.TrimSpace(firstRuntimeSkillValue(doc.ID, doc.Name))
		if id != "" {
			return id, &doc, nil
		}
	}

	if registry != nil {
		for _, candidate := range skillmanifest.CandidateIDs(rawID) {
			if info := registry.GetInfo(candidate); info != nil && info.Manifest != nil {
				return strings.TrimSpace(info.Manifest.ID), nil, nil
			}
		}
	}

	return "", nil, fmt.Errorf("skill %q not found", rawID)
}
