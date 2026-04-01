package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmanifest"
)

type runtimeSkillEntry struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Version     string   `json:"version,omitempty"`
	Author      string   `json:"author,omitempty"`
	Category    string   `json:"category,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Enabled     bool     `json:"enabled"`
}

type runtimeSkillDetail struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Version     string   `json:"version,omitempty"`
	Author      string   `json:"author,omitempty"`
	Category    string   `json:"category,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Installed   bool     `json:"installed"`
	Enabled     bool     `json:"enabled"`
	Stars       int      `json:"stars,omitempty"`
	Downloads   int      `json:"downloads,omitempty"`
	Homepage    string   `json:"homepage,omitempty"`
	Content     string   `json:"content,omitempty"`
}

type installedRuntimeSkill struct {
	ID       string
	Document skillmanifest.Document
	Raw      []byte
	EntryDir string
}

func (a *skillManagerAdapter) List(ctx context.Context) (string, error) {
	_ = ctx

	entries := a.collectRuntimeSkillEntries()
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Category != entries[j].Category {
			return entries[i].Category < entries[j].Category
		}
		if entries[i].Name != entries[j].Name {
			return entries[i].Name < entries[j].Name
		}
		return entries[i].ID < entries[j].ID
	})

	data, _ := json.Marshal(entries)
	return string(data), nil
}

func (a *skillManagerAdapter) collectRuntimeSkillEntries() []runtimeSkillEntry {
	var entries []runtimeSkillEntry
	seen := make(map[string]struct{})
	for _, root := range a.skillRoots() {
		dirEntries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, dirEntry := range dirEntries {
			if !dirEntry.IsDir() {
				continue
			}
			bundle, err := skillmanifest.ValidateInstalledDir(filepath.Join(root, dirEntry.Name()), "", skillmanifest.Options{})
			if err != nil {
				continue
			}
			id := strings.TrimSpace(bundle.Document.ID)
			if id == "" {
				continue
			}
			key := strings.ToLower(id)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			entries = append(entries, runtimeSkillEntry{
				ID:          id,
				Name:        firstRuntimeSkillValue(bundle.Document.Name, id),
				Description: strings.TrimSpace(bundle.Document.Description),
				Version:     strings.TrimSpace(bundle.Document.Version),
				Author:      strings.TrimSpace(bundle.Document.Author),
				Category:    strings.TrimSpace(bundle.Document.Category),
				Tags:        append([]string(nil), bundle.Document.Tags...),
				Enabled:     runtimeManagedSkillEnabled(a.registry, bundle.Document),
			})
		}
	}
	if entries == nil {
		return []runtimeSkillEntry{}
	}
	return entries
}

func (a *skillManagerAdapter) Info(ctx context.Context, id string) (string, error) {
	detail := runtimeSkillDetail{ID: strings.TrimSpace(id)}
	resolved, installed, err := a.resolveInstalledSkill(id)
	if err != nil {
		return "", err
	}
	if installed {
		detail.ID = resolved.ID
		detail.Name = firstRuntimeSkillValue(resolved.Document.Name, resolved.ID)
		detail.Description = strings.TrimSpace(resolved.Document.Description)
		detail.Version = strings.TrimSpace(resolved.Document.Version)
		detail.Author = strings.TrimSpace(resolved.Document.Author)
		detail.Category = strings.TrimSpace(resolved.Document.Category)
		detail.Tags = append([]string(nil), resolved.Document.Tags...)
		detail.Installed = true
		if len(resolved.Raw) > 0 {
			detail.Content = string(resolved.Raw)
		}
	}

	if a.store != nil {
		sk, err := a.store.GetSkill(ctx, detail.ID)
		if err == nil && sk != nil {
			detail.Name = sk.Name
			detail.Description = sk.Summary
			detail.Version = sk.Version
			detail.Author = sk.Author
			detail.Category = sk.Category
			if sk.Tags != "" {
				detail.Tags = strings.Split(sk.Tags, ",")
			}
			detail.Stars = sk.Stars
			detail.Downloads = sk.Downloads
			detail.Homepage = sk.Homepage
		}
	}

	if a.dir != "" && !detail.Installed {
		mdPath := filepath.Join(a.dir, detail.ID, "SKILL.md")
		if data, err := os.ReadFile(mdPath); err == nil {
			detail.Installed = true
			detail.Content = string(data)
		}
	}

	if a.scanner != nil {
		if ls := a.scanner.Get(detail.ID); ls != nil {
			if detail.Name == "" {
				detail.Name = ls.Name
			}
			if detail.Description == "" {
				detail.Description = ls.Description
			}
			detail.Installed = true
		}
	}

	if a.registry != nil {
		detail.Enabled = a.registry.IsEnabled(detail.ID)
	}
	if detail.Name == "" && !detail.Installed {
		return "", fmt.Errorf("skill not found: %s", id)
	}

	data, _ := json.Marshal(detail)
	return string(data), nil
}

func (a *skillManagerAdapter) resolveInstalledSkill(rawID string) (installedRuntimeSkill, bool, error) {
	resolved, ok, err := skillmanifest.FindAnyByCandidatesStrict(
		skillmanifest.CandidateIDs(rawID),
		a.skillRoots(),
		"",
		skillmanifest.Options{},
	)
	if err != nil {
		return installedRuntimeSkill{}, false, err
	}
	if !ok {
		return installedRuntimeSkill{}, false, nil
	}
	id := strings.TrimSpace(firstRuntimeSkillValue(resolved.Document.ID, resolved.Document.Name))
	if id == "" {
		return installedRuntimeSkill{}, false, nil
	}
	return installedRuntimeSkill{
		ID:       id,
		Document: resolved.Document,
		Raw:      append([]byte(nil), resolved.Raw...),
		EntryDir: resolved.EntryDir,
	}, true, nil
}
