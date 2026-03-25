package main

import "testing"

func TestParseIPCArgs_AggregatesRepeatedOptionParams(t *testing.T) {
	params, positional := parseIPCArgs([]string{
		`question=pick`,
		`option=A`,
		`option=B`,
		`option=C`,
	})

	if len(positional) != 0 {
		t.Fatalf("unexpected positional args: %#v", positional)
	}
	if got := params["option"]; got != `["A","B","C"]` {
		t.Fatalf("option param = %q, want %q", got, `["A","B","C"]`)
	}
}

func TestParseIPCArgs_RepeatedNonListKeyKeepsLastValue(t *testing.T) {
	params, _ := parseIPCArgs([]string{
		`question=first`,
		`question=second`,
	})

	if got := params["question"]; got != "second" {
		t.Fatalf("question param = %q, want %q", got, "second")
	}
}

func TestParseIPCArgs_OptionWithCommaIsPreservedInJSONArray(t *testing.T) {
	params, _ := parseIPCArgs([]string{
		`option=A, with comma`,
		`option=B`,
	})
	if got := params["option"]; got != `["A, with comma","B"]` {
		t.Fatalf("option param = %q, want %q", got, `["A, with comma","B"]`)
	}
}

func TestInjectIPCContextParams_AddsBlueUserID(t *testing.T) {
	params := map[string]string{
		"query": "hello",
	}
	getenv := func(key string) string {
		if key == "BLUE_USER_ID" {
			return "user-123"
		}
		return ""
	}

	injectIPCContextParams(params, getenv)

	if got := params["__blue_user_id"]; got != "user-123" {
		t.Fatalf("__blue_user_id = %q, want %q", got, "user-123")
	}
	if got := params["query"]; got != "hello" {
		t.Fatalf("query = %q, want %q", got, "hello")
	}
}

func TestInjectIPCContextParams_DoesNotOverrideExplicitValue(t *testing.T) {
	params := map[string]string{
		"__blue_user_id": "explicit",
	}
	getenv := func(key string) string {
		if key == "BLUE_USER_ID" {
			return "from-env"
		}
		return ""
	}

	injectIPCContextParams(params, getenv)

	if got := params["__blue_user_id"]; got != "explicit" {
		t.Fatalf("__blue_user_id = %q, want %q", got, "explicit")
	}
}

func TestInjectIPCContextParams_AddsSessionID(t *testing.T) {
	params := map[string]string{
		"query": "hello",
	}
	getenv := func(key string) string {
		switch key {
		case "BLUE_USER_ID":
			return "user-123"
		case "BLUE_SESSION_ID":
			return "conv-456"
		default:
			return ""
		}
	}

	injectIPCContextParams(params, getenv)

	if got := params["session_id"]; got != "conv-456" {
		t.Fatalf("session_id = %q, want %q", got, "conv-456")
	}
	if got := params["__blue_user_id"]; got != "user-123" {
		t.Fatalf("__blue_user_id = %q, want %q", got, "user-123")
	}
}

func TestInjectIPCContextParams_DoesNotOverrideExplicitSessionID(t *testing.T) {
	params := map[string]string{
		"session_id": "explicit-conv",
	}
	getenv := func(key string) string {
		if key == "BLUE_SESSION_ID" {
			return "from-env"
		}
		return ""
	}

	injectIPCContextParams(params, getenv)

	if got := params["session_id"]; got != "explicit-conv" {
		t.Fatalf("session_id = %q, want %q", got, "explicit-conv")
	}
}

func TestInjectIPCWorkdirParam_AddsCurrentWorkingDirectory(t *testing.T) {
	params := map[string]string{
		"query": "hello",
	}

	injectIPCWorkdirParam(params, func() (string, error) {
		return "/tmp/pinchbench-workspace", nil
	})

	if got := params["__blue_workdir"]; got != "/tmp/pinchbench-workspace" {
		t.Fatalf("__blue_workdir = %q, want %q", got, "/tmp/pinchbench-workspace")
	}
}

func TestInjectIPCWorkdirParam_DoesNotOverrideExplicitValue(t *testing.T) {
	params := map[string]string{
		"__blue_workdir": "/explicit",
	}

	injectIPCWorkdirParam(params, func() (string, error) {
		return "/tmp/pinchbench-workspace", nil
	})

	if got := params["__blue_workdir"]; got != "/explicit" {
		t.Fatalf("__blue_workdir = %q, want %q", got, "/explicit")
	}
}

func TestNormalizeIPCCommand_MapsSlashInstallToSkillInstall(t *testing.T) {
	cmd, params, positional := normalizeIPCCommand("/install", []string{"humanizer"})

	if cmd != "skill.install" {
		t.Fatalf("cmd = %q, want %q", cmd, "skill.install")
	}
	if got := params["id"]; got != "humanizer" {
		t.Fatalf("id = %q, want %q", got, "humanizer")
	}
	if len(positional) != 0 {
		t.Fatalf("unexpected positional args: %#v", positional)
	}
}

func TestNormalizeIPCCommand_PreservesExplicitInstallID(t *testing.T) {
	cmd, params, positional := normalizeIPCCommand("/install", []string{"id=weather", "humanizer"})

	if cmd != "skill.install" {
		t.Fatalf("cmd = %q, want %q", cmd, "skill.install")
	}
	if got := params["id"]; got != "weather" {
		t.Fatalf("id = %q, want %q", got, "weather")
	}
	if len(positional) != 1 || positional[0] != "humanizer" {
		t.Fatalf("positional = %#v, want %#v", positional, []string{"humanizer"})
	}
}

func TestNormalizeIPCCommand_MapsSlashSearchToSkillSearchQuery(t *testing.T) {
	cmd, params, positional := normalizeIPCCommand("/search", []string{"humanizer", "rewrite", "category=content_creation"})

	if cmd != "skill.search" {
		t.Fatalf("cmd = %q, want %q", cmd, "skill.search")
	}
	if got := params["query"]; got != "humanizer rewrite" {
		t.Fatalf("query = %q, want %q", got, "humanizer rewrite")
	}
	if got := params["category"]; got != "content_creation" {
		t.Fatalf("category = %q, want %q", got, "content_creation")
	}
	if len(positional) != 0 {
		t.Fatalf("unexpected positional args: %#v", positional)
	}
}

func TestNormalizeIPCCommand_MapsSlashInstallURLToSkillInstallURL(t *testing.T) {
	cmd, params, positional := normalizeIPCCommand("/install-url", []string{"https://example.com/SKILL.md", "humanizer"})

	if cmd != "skill.install_url" {
		t.Fatalf("cmd = %q, want %q", cmd, "skill.install_url")
	}
	if got := params["url"]; got != "https://example.com/SKILL.md" {
		t.Fatalf("url = %q, want %q", got, "https://example.com/SKILL.md")
	}
	if got := params["name"]; got != "humanizer" {
		t.Fatalf("name = %q, want %q", got, "humanizer")
	}
	if len(positional) != 0 {
		t.Fatalf("unexpected positional args: %#v", positional)
	}
}
