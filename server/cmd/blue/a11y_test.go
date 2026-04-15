package main

import "testing"

func TestNormalizeA11yCLIArgs_UsesPositionalAction(t *testing.T) {
	params, positional := normalizeA11yCLIArgs(nil, []string{"focus", "Finder"})

	if got := params["action"]; got != "focus" {
		t.Fatalf("action = %q, want %q", got, "focus")
	}
	if len(positional) != 1 || positional[0] != "Finder" {
		t.Fatalf("positional = %#v, want %#v", positional, []string{"Finder"})
	}
}

func TestNormalizeA11yCLIArgs_CanonicalizesHyphenatedKeys(t *testing.T) {
	params, positional := normalizeA11yCLIArgs(map[string]string{
		"app-name":     "Feishu",
		"window-title": "Lark",
		"act-type":     "click",
	}, nil)

	if len(positional) != 0 {
		t.Fatalf("unexpected positional args: %#v", positional)
	}
	if got := params["app_name"]; got != "Feishu" {
		t.Fatalf("app_name = %q, want %q", got, "Feishu")
	}
	if got := params["window_title"]; got != "Lark" {
		t.Fatalf("window_title = %q, want %q", got, "Lark")
	}
	if got := params["act_type"]; got != "click" {
		t.Fatalf("act_type = %q, want %q", got, "click")
	}
}

func TestRootCmdFind_A11yCommandRegistered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"a11y"})
	if err != nil {
		t.Fatalf("rootCmd.Find(a11y): %v", err)
	}
	if cmd == nil || cmd.Name() != "a11y" {
		t.Fatalf("command = %#v, want a11y", cmd)
	}
}
