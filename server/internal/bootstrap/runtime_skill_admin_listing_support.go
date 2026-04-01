package bootstrap

import (
	"os"
	"path/filepath"
	"strings"

	skillpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmanifest"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func overlayRuntimeAdminSkillExposure(items []tools.AdminSkillInfo, workspaceDir string) ([]tools.AdminSkillInfo, error) {
	snapshot, err := skillmanifest.SharedSkillExposureManager(workspaceDir).Snapshot()
	if err != nil {
		return nil, err
	}
	lookup := runtimeAdminSkillExposureLookup(snapshot)
	for idx := range items {
		applyRuntimeAdminSkillExposure(&items[idx], lookup)
	}
	return items, nil
}

func runtimeAdminSkillExposureLookup(snapshot skillmanifest.SkillExposureSnapshot) map[string]skillmanifest.SkillExposureView {
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

func applyRuntimeAdminSkillExposure(info *tools.AdminSkillInfo, lookup map[string]skillmanifest.SkillExposureView) {
	if info == nil {
		return
	}
	for _, key := range []string{
		strings.ToLower(strings.TrimSpace(info.ID)),
		strings.ToLower(strings.TrimSpace(info.Name)),
	} {
		view, ok := lookup[key]
		if !ok {
			continue
		}
		info.Paths = append([]string(nil), view.Paths...)
		info.UserInvocable = view.UserInvocable
		info.ModelInvocable = view.ModelInvocable
		info.ActivationState = strings.TrimSpace(view.ActivationState)
		info.ActivationSource = strings.TrimSpace(view.ActivationSource)
		return
	}
	if info.ActivationState == "" {
		info.ActivationState = string(skillmanifest.SkillActivationStateActive)
	}
	if info.ActivationSource == "" {
		info.ActivationSource = string(skillmanifest.SkillActivationSourceBaseline)
	}
}

func appendRegistryAdminSkills(items map[string]tools.AdminSkillInfo, registry *skillpkg.Registry, roots []string) error {
	if registry == nil {
		return nil
	}
	for _, info := range registry.List() {
		if info == nil || info.Manifest == nil {
			continue
		}
		item := tools.AdminSkillInfo{
			ID:               strings.TrimSpace(info.Manifest.ID),
			Name:             strings.TrimSpace(info.Manifest.Name),
			Category:         strings.TrimSpace(info.Manifest.Category),
			Enabled:          info.Enabled,
			Builtin:          info.Builtin,
			Paths:            append([]string(nil), info.Manifest.Paths...),
			UserInvocable:    info.Manifest.UserInvocable,
			ModelInvocable:   info.Manifest.ModelInvocable,
			ActivationState:  string(skillmanifest.SkillActivationStateActive),
			ActivationSource: string(skillmanifest.SkillActivationSourceBaseline),
		}
		resolved, ok, err := skillmanifest.FindAnyByCandidatesStrict(skillmanifest.CandidateIDs(item.ID), roots, "", skillmanifest.Options{})
		if err != nil {
			return err
		}
		if ok {
			item = adminSkillInfoFromDocument(resolved.Document, item.Enabled, item.Builtin && resolved.Embedded)
			if regInfo := registry.GetInfo(item.ID); regInfo != nil {
				item.Enabled = regInfo.Enabled
			}
		}
		setRuntimeAdminSkillInfo(items, item)
	}
	return nil
}

func appendInstalledRootAdminSkills(items map[string]tools.AdminSkillInfo, registry *skillpkg.Registry, roots []string) error {
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			bundle, err := skillmanifest.ValidateInstalledDir(filepath.Join(root, entry.Name()), "", skillmanifest.Options{})
			if err != nil {
				continue
			}
			item := adminSkillInfoFromDocument(bundle.Document, bundle.Document.Enabled, false)
			if registry != nil {
				if regInfo := registry.GetInfo(item.ID); regInfo != nil {
					item.Enabled = regInfo.Enabled
					item.Builtin = regInfo.Builtin
				}
			}
			if _, exists := items[strings.ToLower(item.ID)]; exists {
				continue
			}
			setRuntimeAdminSkillInfo(items, item)
		}
	}
	return nil
}

func setRuntimeAdminSkillInfo(items map[string]tools.AdminSkillInfo, info tools.AdminSkillInfo) {
	id := strings.TrimSpace(info.ID)
	if id == "" {
		return
	}
	if strings.TrimSpace(info.Name) == "" {
		info.Name = id
	}
	items[strings.ToLower(id)] = info
}
