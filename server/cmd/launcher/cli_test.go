package main

import "testing"

func TestTryFastCmd_HelpHandledInLauncher(t *testing.T) {
	if handled := tryFastCmd([]string{"help"}); !handled {
		t.Fatal("expected launcher help to be handled directly")
	}
}

func TestTryFastCmd_HelpWithTopicHandledInLauncher(t *testing.T) {
	if handled := tryFastCmd([]string{"help", "reminder"}); !handled {
		t.Fatal("expected launcher help topic to be handled directly")
	}
}

func TestServicePort_DefaultsTo80(t *testing.T) {
	if got := servicePort(cliFlags{}); got != 80 {
		t.Fatalf("servicePort()=%d, want 80", got)
	}
}

func TestServicePort_UsesShiftedDevPort(t *testing.T) {
	if got := servicePort(cliFlags{devMode: true}); got != 8081 {
		t.Fatalf("servicePort(dev)=%d, want 8081", got)
	}
}

func TestParseCLIFlags_CapturesProfileAndConfig(t *testing.T) {
	positional, flags := parseCLIFlags([]string{"--profile", "cutover", "--config", "/tmp/blue.yaml", "web_query", "docs"})
	if len(positional) != 2 || positional[0] != "web_query" || positional[1] != "docs" {
		t.Fatalf("parseCLIFlags positional = %v", positional)
	}
	if flags.profile != "cutover" {
		t.Fatalf("profile = %q, want cutover", flags.profile)
	}
	if flags.config != "/tmp/blue.yaml" {
		t.Fatalf("config = %q, want /tmp/blue.yaml", flags.config)
	}
}

func TestGetDataDir_UsesProfile(t *testing.T) {
	t.Setenv("HOME", "/tmp/launcher-home")
	got := getDataDir(cliFlags{profile: "qa"})
	if got != "/tmp/launcher-home/.zimaos-blue-qa/data" {
		t.Fatalf("getDataDir(profile)= %q", got)
	}
}

func TestCandidateLauncherIPCSocketPaths_PrefersScopedSocket(t *testing.T) {
	t.Setenv("BLUE_IPC_SOCKET", "")
	t.Setenv("HOME", "/tmp/launcher-home")
	got := candidateLauncherIPCSocketPaths(cliFlags{profile: "qa"})
	want := []string{"/tmp/launcher-home/.zimaos-blue-qa/data/blue.sock", "/tmp/blue.sock"}
	if len(got) != len(want) {
		t.Fatalf("candidateLauncherIPCSocketPaths() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("candidateLauncherIPCSocketPaths() = %v, want %v", got, want)
		}
	}
}

func TestCandidateLauncherIPCSocketPaths_EnvOverride(t *testing.T) {
	t.Setenv("BLUE_IPC_SOCKET", "/tmp/blue-explicit.sock")
	got := candidateLauncherIPCSocketPaths(cliFlags{profile: "qa"})
	if len(got) != 1 || got[0] != "/tmp/blue-explicit.sock" {
		t.Fatalf("candidateLauncherIPCSocketPaths() = %v", got)
	}
}

func TestShouldAttemptIPCShortcut_SkipsBuiltinCommandGroups(t *testing.T) {
	for _, cmd := range []string{"context", "sessions", "harness", "media"} {
		if shouldAttemptIPCShortcut(cmd) {
			t.Fatalf("expected %q to skip launcher IPC shortcut", cmd)
		}
	}
}

func TestShouldAttemptIPCShortcut_AllowsSkillLikeCommands(t *testing.T) {
	for _, cmd := range []string{"web_query", "browser", "reminder"} {
		if !shouldAttemptIPCShortcut(cmd) {
			t.Fatalf("expected %q to use launcher IPC shortcut", cmd)
		}
	}
}
