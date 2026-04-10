package main

import (
	"testing"

	"go.uber.org/zap"
)

func TestRunServerRestartsInProcessUntilStable(t *testing.T) {
	original := runServerIteration
	t.Cleanup(func() {
		runServerIteration = original
	})

	calls := 0
	runServerIteration = func() serverRunOutcome {
		calls++
		return serverRunOutcome{RestartRequested: calls == 1}
	}

	runServer()

	if calls != 2 {
		t.Fatalf("runServer() calls = %d, want 2", calls)
	}
}

func TestServerRestartControllerSuppressesPendingRestartOnExternalStop(t *testing.T) {
	controller := newServerRestartController()
	if !controller.Request() {
		t.Fatalf("expected first restart request to succeed")
	}

	controller.Suppress()

	if controller.Requested() {
		t.Fatalf("expected suppressed controller to report no pending restart")
	}
	if controller.Request() {
		t.Fatalf("expected suppressed controller to reject new restart request")
	}
}

func TestShouldSkipStartupSTTAuthorization(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{
			name: "gateway run with config does not skip stt",
			args: []string{"--config", "/tmp/test.yaml", "gateway", "run", "--bind", "127.0.0.1"},
			want: false,
		},
		{
			name: "gateway run with no-intercept does not skip stt",
			args: []string{"--no-intercept", "gateway", "run"},
			want: false,
		},
		{
			name: "gateway run without config does not skip stt",
			args: []string{"gateway", "run"},
			want: false,
		},
		{
			name: "gateway status skips stt",
			args: []string{"gateway", "status"},
			want: true,
		},
		{
			name: "gateway root skips stt",
			args: []string{"gateway"},
			want: true,
		},
		{
			name: "status command does not skip here",
			args: []string{"status"},
			want: false,
		},
		{
			name: "no subcommand does not skip",
			args: nil,
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldSkipStartupSTTAuthorization(tc.args); got != tc.want {
				t.Fatalf("shouldSkipStartupSTTAuthorization(%v) = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}

func TestAuxServiceLoggerConfig_DisablesSampling(t *testing.T) {
	cfg := auxServiceLoggerConfig()
	if cfg.Sampling != nil {
		t.Fatalf("expected aux service logger sampling to be disabled, got %+v", cfg.Sampling)
	}

	production := zap.NewProductionConfig()
	if cfg.Level.Level() != production.Level.Level() {
		t.Fatalf("expected production log level %d, got %d", production.Level.Level(), cfg.Level.Level())
	}
	if cfg.Encoding != production.Encoding {
		t.Fatalf("expected production encoding %q, got %q", production.Encoding, cfg.Encoding)
	}
}

func TestRootCommand_DisablesCobraDefaultCompletionCommand(t *testing.T) {
	if !rootCmd.CompletionOptions.DisableDefaultCmd {
		t.Fatalf("expected cobra default completion command to be disabled")
	}
}

func TestRootCommand_RegistersCustomCompletionCommand(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"completion"})
	if err != nil {
		t.Fatalf("rootCmd.Find(completion): %v", err)
	}
	if cmd == nil || cmd.Name() != "completion" {
		t.Fatalf("expected custom completion command to be registered, got %#v", cmd)
	}
}

func TestRootCommand_RegistersSkillsAlias(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"skill"})
	if err != nil {
		t.Fatalf("rootCmd.Find(skill): %v", err)
	}
	if cmd == nil || cmd.Name() != "skills" {
		t.Fatalf("expected skill alias to resolve to skills command, got %#v", cmd)
	}
}
