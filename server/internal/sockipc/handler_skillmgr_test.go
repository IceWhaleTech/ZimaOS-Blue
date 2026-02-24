package sockipc

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"testing"

	"go.uber.org/zap"
)

// mockSkillManager implements SkillManagerBackend for testing.
type mockSkillManager struct {
	installed map[string]bool
}

func newMockSkillManager() *mockSkillManager {
	return &mockSkillManager{installed: map[string]bool{"weather": true}}
}

func (m *mockSkillManager) Search(_ context.Context, query, category string, page, pageSize int) (string, error) {
	result := map[string]interface{}{
		"skills": []map[string]string{
			{"id": "weather", "name": "Weather", "description": "Get weather info"},
		},
		"total":     1,
		"page":      page,
		"page_size": pageSize,
	}
	data, _ := json.Marshal(result)
	return string(data), nil
}

func (m *mockSkillManager) Install(_ context.Context, id string) (string, error) {
	if m.installed[id] {
		return "", fmt.Errorf("skill already installed")
	}
	m.installed[id] = true
	result, _ := json.Marshal(map[string]string{"id": id, "status": "installed"})
	return string(result), nil
}

func (m *mockSkillManager) InstallURL(_ context.Context, url, name string) (string, error) {
	id := name
	if id == "" {
		id = "from-url"
	}
	m.installed[id] = true
	result, _ := json.Marshal(map[string]string{"id": id, "status": "installed"})
	return string(result), nil
}

func (m *mockSkillManager) Uninstall(_ context.Context, id string) error {
	if !m.installed[id] {
		return fmt.Errorf("skill not installed")
	}
	delete(m.installed, id)
	return nil
}

func (m *mockSkillManager) Update(_ context.Context, id string) (string, error) {
	m.installed[id] = true
	result, _ := json.Marshal(map[string]string{"id": id, "status": "updated"})
	return string(result), nil
}

func (m *mockSkillManager) Enable(_ context.Context, id string) error {
	return nil
}

func (m *mockSkillManager) Disable(_ context.Context, id string) error {
	return nil
}

func (m *mockSkillManager) List(_ context.Context) (string, error) {
	var skills []map[string]string
	for id := range m.installed {
		skills = append(skills, map[string]string{"id": id, "name": id, "enabled": "true"})
	}
	data, _ := json.Marshal(skills)
	return string(data), nil
}

func (m *mockSkillManager) Info(_ context.Context, id string) (string, error) {
	if !m.installed[id] {
		return "", fmt.Errorf("skill not found: %s", id)
	}
	data, _ := json.Marshal(map[string]string{"id": id, "name": id, "installed": "true"})
	return string(data), nil
}

func setupSkillMgrServer(t *testing.T) (net.Conn, func()) {
	t.Helper()
	sock := shortSock(t)
	srv := NewServer(sock, zap.NewNop())
	RegisterSkillManagerHandlers(srv, newMockSkillManager(), zap.NewNop())
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	conn, err := net.Dial("unix", sock)
	if err != nil {
		srv.Close()
		t.Fatal(err)
	}
	return conn, func() { conn.Close(); srv.Close() }
}

func TestHandlerSkillSearch(t *testing.T) {
	conn, cleanup := setupSkillMgrServer(t)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{Cmd: "skill.search", Params: map[string]string{"query": "weather"}})
	if resp.Status != "ok" {
		t.Fatalf("status=%q error=%q", resp.Status, resp.Error)
	}
	if resp.Data["result"] == "" {
		t.Fatal("empty result")
	}
}

func TestHandlerSkillSearchMissingQuery(t *testing.T) {
	conn, cleanup := setupSkillMgrServer(t)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{Cmd: "skill.search"})
	if resp.Status != "error" {
		t.Fatalf("expected error, got status=%q", resp.Status)
	}
}

func TestHandlerSkillInstall(t *testing.T) {
	conn, cleanup := setupSkillMgrServer(t)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{Cmd: "skill.install", Params: map[string]string{"id": "new-skill"}})
	if resp.Status != "ok" {
		t.Fatalf("status=%q error=%q", resp.Status, resp.Error)
	}
}

func TestHandlerSkillInstallAlreadyInstalled(t *testing.T) {
	conn, cleanup := setupSkillMgrServer(t)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{Cmd: "skill.install", Params: map[string]string{"id": "weather"}})
	if resp.Status != "error" {
		t.Fatalf("expected error for already installed, got status=%q", resp.Status)
	}
}

func TestHandlerSkillInstallURL(t *testing.T) {
	conn, cleanup := setupSkillMgrServer(t)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{Cmd: "skill.install_url", Params: map[string]string{"url": "https://example.com/SKILL.md", "name": "my-skill"}})
	if resp.Status != "ok" {
		t.Fatalf("status=%q error=%q", resp.Status, resp.Error)
	}
}

func TestHandlerSkillUninstall(t *testing.T) {
	conn, cleanup := setupSkillMgrServer(t)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{Cmd: "skill.uninstall", Params: map[string]string{"id": "weather"}})
	if resp.Status != "ok" {
		t.Fatalf("status=%q error=%q", resp.Status, resp.Error)
	}
	if resp.Data["uninstalled"] != "weather" {
		t.Errorf("uninstalled=%q", resp.Data["uninstalled"])
	}
}

func TestHandlerSkillUninstallNotInstalled(t *testing.T) {
	conn, cleanup := setupSkillMgrServer(t)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{Cmd: "skill.uninstall", Params: map[string]string{"id": "nonexistent"}})
	if resp.Status != "error" {
		t.Fatalf("expected error, got status=%q", resp.Status)
	}
}

func TestHandlerSkillUpdate(t *testing.T) {
	conn, cleanup := setupSkillMgrServer(t)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{Cmd: "skill.update", Params: map[string]string{"id": "weather"}})
	if resp.Status != "ok" {
		t.Fatalf("status=%q error=%q", resp.Status, resp.Error)
	}
}

func TestHandlerSkillEnable(t *testing.T) {
	conn, cleanup := setupSkillMgrServer(t)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{Cmd: "skill.enable", Params: map[string]string{"id": "weather"}})
	if resp.Status != "ok" {
		t.Fatalf("status=%q error=%q", resp.Status, resp.Error)
	}
	if resp.Data["enabled"] != "weather" {
		t.Errorf("enabled=%q", resp.Data["enabled"])
	}
}

func TestHandlerSkillDisable(t *testing.T) {
	conn, cleanup := setupSkillMgrServer(t)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{Cmd: "skill.disable", Params: map[string]string{"id": "weather"}})
	if resp.Status != "ok" {
		t.Fatalf("status=%q error=%q", resp.Status, resp.Error)
	}
	if resp.Data["disabled"] != "weather" {
		t.Errorf("disabled=%q", resp.Data["disabled"])
	}
}

func TestHandlerSkillList(t *testing.T) {
	conn, cleanup := setupSkillMgrServer(t)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{Cmd: "skill.list"})
	if resp.Status != "ok" {
		t.Fatalf("status=%q error=%q", resp.Status, resp.Error)
	}
	if resp.Data["skills"] == "" {
		t.Fatal("empty skills")
	}
}

func TestHandlerSkillInfo(t *testing.T) {
	conn, cleanup := setupSkillMgrServer(t)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{Cmd: "skill.info", Params: map[string]string{"id": "weather"}})
	if resp.Status != "ok" {
		t.Fatalf("status=%q error=%q", resp.Status, resp.Error)
	}
	if resp.Data["skill"] == "" {
		t.Fatal("empty skill info")
	}
}

func TestHandlerSkillInfoNotFound(t *testing.T) {
	conn, cleanup := setupSkillMgrServer(t)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{Cmd: "skill.info", Params: map[string]string{"id": "nonexistent"}})
	if resp.Status != "error" {
		t.Fatalf("expected error, got status=%q", resp.Status)
	}
}

func TestHandlerSkillMissingID(t *testing.T) {
	conn, cleanup := setupSkillMgrServer(t)
	defer cleanup()

	for _, cmd := range []string{"skill.install", "skill.uninstall", "skill.update", "skill.enable", "skill.disable", "skill.info"} {
		resp := sendRecv(t, conn, &Request{Cmd: cmd})
		if resp.Status != "error" {
			t.Errorf("%s: expected error for missing id, got status=%q", cmd, resp.Status)
		}
	}
}
