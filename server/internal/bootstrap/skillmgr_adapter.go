package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/network"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sockipc"
)

// skillManagerAdapter implements sockipc.SkillManagerBackend by wrapping
// the existing skillstore.Store, skill.Registry, and local scanner.
type skillManagerAdapter struct {
	store    *skillstore.Store
	registry *skill.Registry
	scanner  *skillstore.LocalSkillScanner
	dir      string // {dataDir}/workspace/.claude/skills/
	client   *http.Client
}

// newSkillManagerAdapter creates a new adapter.
func newSkillManagerAdapter(
	store *skillstore.Store,
	registry *skill.Registry,
	scanner *skillstore.LocalSkillScanner,
	skillsDir string,
) sockipc.SkillManagerBackend {
	return &skillManagerAdapter{
		store:    store,
		registry: registry,
		scanner:  scanner,
		dir:      skillsDir,
		client:   network.NewPooledHTTPClient(5 * time.Minute),
	}
}

func (a *skillManagerAdapter) Search(ctx context.Context, query, category string, page, pageSize int) (string, error) {
	if a.store == nil {
		return "[]", nil
	}
	opts := skillstore.SearchOptions{
		Query:    query,
		Page:     page,
		PageSize: pageSize,
	}
	if category != "" {
		opts.Categories = []string{category}
	}
	result, err := a.store.Search(ctx, opts)
	if err != nil {
		return "", err
	}
	data, _ := json.Marshal(result)
	return string(data), nil
}

func (a *skillManagerAdapter) Install(ctx context.Context, id string) (string, error) {
	if a.dir == "" {
		return "", fmt.Errorf("skills directory not configured")
	}

	skillDir := filepath.Join(a.dir, id)
	if _, err := os.Stat(filepath.Join(skillDir, "SKILL.md")); err == nil {
		return "", fmt.Errorf("skill already installed")
	}

	// Download SKILL.md from ClawHub
	content, err := a.downloadSkillMD(ctx, id)
	if err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}

	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), content, 0o644); err != nil {
		os.RemoveAll(skillDir)
		return "", fmt.Errorf("write: %w", err)
	}

	// Mark as installed in store
	if a.store != nil {
		a.store.SetInstalled(ctx, id, true)
	}

	// Rescan local skills
	if a.scanner != nil {
		a.scanner.Scan()
	}

	result, _ := json.Marshal(map[string]string{"id": id, "status": "installed"})
	return string(result), nil
}

func (a *skillManagerAdapter) InstallURL(ctx context.Context, url, name string) (string, error) {
	if a.dir == "" {
		return "", fmt.Errorf("skills directory not configured")
	}

	// Download content
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Derive skill ID
	skillID := name
	if skillID == "" {
		// Extract from URL path
		parts := strings.Split(strings.TrimSuffix(url, "/"), "/")
		for i := len(parts) - 1; i >= 0; i-- {
			p := strings.TrimSuffix(parts[i], ".md")
			p = strings.TrimSuffix(p, ".MD")
			if p != "" && p != "SKILL" {
				skillID = strings.ToLower(p)
				break
			}
		}
	}
	if skillID == "" {
		return "", fmt.Errorf("could not determine skill ID from URL")
	}
	skillID = strings.ToLower(strings.ReplaceAll(skillID, " ", "-"))

	skillDir := filepath.Join(a.dir, skillID)
	if _, err := os.Stat(skillDir); err == nil {
		return "", fmt.Errorf("skill already installed")
	}

	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), body, 0o644); err != nil {
		os.RemoveAll(skillDir)
		return "", fmt.Errorf("write: %w", err)
	}

	if a.scanner != nil {
		a.scanner.Scan()
	}

	result, _ := json.Marshal(map[string]string{"id": skillID, "status": "installed"})
	return string(result), nil
}

func (a *skillManagerAdapter) Uninstall(ctx context.Context, id string) error {
	if a.dir == "" {
		return fmt.Errorf("skills directory not configured")
	}

	skillDir := filepath.Join(a.dir, id)
	if _, err := os.Stat(skillDir); os.IsNotExist(err) {
		return fmt.Errorf("skill not installed")
	}

	if err := os.RemoveAll(skillDir); err != nil {
		return fmt.Errorf("remove: %w", err)
	}

	a.registry.Unregister(id)

	if a.store != nil {
		a.store.SetInstalled(ctx, id, false)
	}

	if a.scanner != nil {
		a.scanner.Scan()
	}

	return nil
}

func (a *skillManagerAdapter) Update(ctx context.Context, id string) (string, error) {
	// Uninstall (ignore error if not installed)
	_ = a.Uninstall(ctx, id)

	// Reinstall
	return a.Install(ctx, id)
}

func (a *skillManagerAdapter) Enable(ctx context.Context, id string) error {
	if err := a.registry.Enable(id); err != nil {
		return err
	}
	if a.store != nil {
		a.store.SetEnabled(ctx, id, true)
	}
	return nil
}

func (a *skillManagerAdapter) Disable(ctx context.Context, id string) error {
	if err := a.registry.Disable(id); err != nil {
		return err
	}
	if a.store != nil {
		a.store.SetEnabled(ctx, id, false)
	}
	return nil
}

func (a *skillManagerAdapter) List(ctx context.Context) (string, error) {
	type skillEntry struct {
		ID          string   `json:"id"`
		Name        string   `json:"name"`
		Description string   `json:"description,omitempty"`
		Version     string   `json:"version,omitempty"`
		Author      string   `json:"author,omitempty"`
		Category    string   `json:"category,omitempty"`
		Tags        []string `json:"tags,omitempty"`
		Enabled     bool     `json:"enabled"`
	}

	var entries []skillEntry

	if a.scanner != nil {
		for _, ls := range a.scanner.GetAll() {
			entries = append(entries, skillEntry{
				ID:          ls.ID,
				Name:        ls.Name,
				Description: ls.Description,
				Version:     ls.Version,
				Author:      ls.Author,
				Category:    ls.Category,
				Tags:        ls.Tags,
				Enabled:     a.registry.IsEnabled(ls.ID),
			})
		}
	}

	if entries == nil {
		entries = []skillEntry{}
	}
	data, _ := json.Marshal(entries)
	return string(data), nil
}

func (a *skillManagerAdapter) Info(ctx context.Context, id string) (string, error) {
	type skillDetail struct {
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

	detail := skillDetail{ID: id}

	// Check store for metadata
	if a.store != nil {
		sk, err := a.store.GetSkill(ctx, id)
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

	// Check if installed locally
	if a.dir != "" {
		mdPath := filepath.Join(a.dir, id, "SKILL.md")
		if data, err := os.ReadFile(mdPath); err == nil {
			detail.Installed = true
			detail.Content = string(data)
		}
	}

	// Check local scanner
	if a.scanner != nil {
		if ls := a.scanner.Get(id); ls != nil {
			if detail.Name == "" {
				detail.Name = ls.Name
			}
			if detail.Description == "" {
				detail.Description = ls.Description
			}
			detail.Installed = true
		}
	}

	detail.Enabled = a.registry.IsEnabled(id)

	if detail.Name == "" && !detail.Installed {
		return "", fmt.Errorf("skill not found: %s", id)
	}

	data, _ := json.Marshal(detail)
	return string(data), nil
}

// downloadSkillMD downloads SKILL.md for a skill from ClawHub with retry.
func (a *skillManagerAdapter) downloadSkillMD(ctx context.Context, id string) ([]byte, error) {
	urls := []string{
		fmt.Sprintf("https://www.clawhub.ai/api/v1/skills/%s/skill-md", id),
		fmt.Sprintf("https://www.clawhub.ai/skills/%s", id),
	}

	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		for _, u := range urls {
			req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
			if err != nil {
				continue
			}
			resp, err := a.client.Do(req)
			if err != nil {
				lastErr = err
				continue
			}
			if resp.StatusCode == http.StatusOK {
				data, err := io.ReadAll(resp.Body)
				resp.Body.Close()
				if err == nil && len(data) > 0 {
					return data, nil
				}
				lastErr = err
			} else {
				resp.Body.Close()
				lastErr = fmt.Errorf("HTTP %d from %s", resp.StatusCode, u)
			}
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if attempt < 3 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(1<<(attempt-1)) * time.Second):
			}
		}
	}

	// Fallback: generate minimal SKILL.md
	name := id
	if a.store != nil {
		if sk, err := a.store.GetSkill(ctx, id); err == nil && sk != nil {
			name = sk.Name
		}
	}
	_ = lastErr
	return []byte(fmt.Sprintf("# %s\n\nInstalled from skill store.\n", name)), nil
}
