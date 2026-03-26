package main

import "testing"

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
