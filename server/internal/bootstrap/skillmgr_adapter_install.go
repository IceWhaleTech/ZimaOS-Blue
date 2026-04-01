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
)

func (a *skillManagerAdapter) Install(ctx context.Context, id string) (string, error) {
	if strings.TrimSpace(a.dir) == "" {
		return "", fmt.Errorf("skills directory not configured")
	}

	skillDir := filepath.Join(a.dir, id)
	if _, err := os.Stat(filepath.Join(skillDir, "SKILL.md")); err == nil {
		return "", fmt.Errorf("skill already installed")
	}

	content, err := a.downloadSkillMD(ctx, id)
	if err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}
	if err := writeInstalledSkillContent(skillDir, content); err != nil {
		return "", err
	}
	if a.store != nil {
		a.store.SetInstalled(ctx, id, true)
	}
	a.rescanLocalSkills()

	result, _ := json.Marshal(map[string]string{"id": id, "status": "installed"})
	return string(result), nil
}

func (a *skillManagerAdapter) InstallURL(ctx context.Context, url, name string) (string, error) {
	if strings.TrimSpace(a.dir) == "" {
		return "", fmt.Errorf("skills directory not configured")
	}

	body, err := a.downloadURLContent(ctx, url)
	if err != nil {
		return "", err
	}
	skillID, err := deriveInstalledSkillID(url, name)
	if err != nil {
		return "", err
	}

	skillDir := filepath.Join(a.dir, skillID)
	if _, err := os.Stat(skillDir); err == nil {
		return "", fmt.Errorf("skill already installed")
	}
	if err := writeInstalledSkillContent(skillDir, body); err != nil {
		return "", err
	}
	a.rescanLocalSkills()

	result, _ := json.Marshal(map[string]string{"id": skillID, "status": "installed"})
	return string(result), nil
}

func (a *skillManagerAdapter) Uninstall(ctx context.Context, id string) error {
	if strings.TrimSpace(a.dir) == "" {
		return fmt.Errorf("skills directory not configured")
	}

	canonicalID := strings.TrimSpace(id)
	skillDir := filepath.Join(a.dir, id)
	if resolved, ok, err := a.resolveInstalledSkill(id); err != nil {
		return err
	} else if ok {
		canonicalID = resolved.ID
		skillDir = resolved.EntryDir
	} else if _, err := os.Stat(skillDir); os.IsNotExist(err) {
		return fmt.Errorf("skill not installed")
	}

	if err := os.RemoveAll(skillDir); err != nil {
		return fmt.Errorf("remove: %w", err)
	}
	if a.registry != nil {
		a.registry.Unregister(canonicalID)
	}
	if a.store != nil {
		a.store.SetInstalled(ctx, canonicalID, false)
	}
	a.rescanLocalSkills()
	return nil
}

func (a *skillManagerAdapter) Update(ctx context.Context, id string) (string, error) {
	if resolved, ok, err := a.resolveInstalledSkill(id); err != nil {
		return "", err
	} else if ok {
		id = resolved.ID
	}
	_ = a.Uninstall(ctx, id)
	return a.Install(ctx, id)
}

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
				continue
			}
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d from %s", resp.StatusCode, u)
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

	name := id
	if a.store != nil {
		if sk, err := a.store.GetSkill(ctx, id); err == nil && sk != nil {
			name = sk.Name
		}
	}
	_ = lastErr
	return []byte(fmt.Sprintf("# %s\n\nInstalled from skill store.\n", name)), nil
}

func (a *skillManagerAdapter) downloadURLContent(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func deriveInstalledSkillID(url, name string) (string, error) {
	skillID := strings.TrimSpace(name)
	if skillID == "" {
		parts := strings.Split(strings.TrimSuffix(url, "/"), "/")
		for i := len(parts) - 1; i >= 0; i-- {
			part := strings.TrimSpace(parts[i])
			part = strings.TrimSuffix(part, ".md")
			part = strings.TrimSuffix(part, ".MD")
			if part != "" && part != "SKILL" {
				skillID = strings.ToLower(part)
				break
			}
		}
	}
	if skillID == "" {
		return "", fmt.Errorf("could not determine skill ID from URL")
	}
	return strings.ToLower(strings.ReplaceAll(skillID, " ", "-")), nil
}

func writeInstalledSkillContent(skillDir string, content []byte) error {
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), content, 0o644); err != nil {
		os.RemoveAll(skillDir)
		return fmt.Errorf("write: %w", err)
	}
	return nil
}

func (a *skillManagerAdapter) rescanLocalSkills() {
	if a.scanner != nil {
		a.scanner.Scan()
	}
}
