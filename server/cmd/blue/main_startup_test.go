package main

import "testing"

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
