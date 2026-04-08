//go:build windows

package main

import "testing"

func TestShouldPauseOnExit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		args        []string
		interactive bool
		parentName  string
		env         map[string]string
		want        bool
	}{
		{name: "explorer double click", args: []string{"blue.exe"}, interactive: true, parentName: "explorer.exe", want: true},
		{name: "cmd launch", args: []string{"blue.exe"}, interactive: true, parentName: "cmd.exe", want: true},
		{name: "subcommand", args: []string{"blue.exe", "status"}, interactive: true, parentName: "explorer.exe", want: false},
		{name: "non interactive", args: []string{"blue.exe"}, interactive: false, parentName: "explorer.exe", want: false},
		{name: "force enable", args: []string{"blue.exe", "status"}, interactive: false, parentName: "cmd.exe", env: map[string]string{"ZIMAOS_BLUE_PAUSE_ON_EXIT": "1"}, want: true},
		{name: "force disable", args: []string{"blue.exe"}, interactive: true, parentName: "explorer.exe", env: map[string]string{"ZIMAOS_BLUE_NO_PAUSE_ON_EXIT": "1"}, want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			getenv := func(key string) string {
				if tt.env == nil {
					return ""
				}
				return tt.env[key]
			}
			if got := shouldPauseOnExit(tt.args, tt.interactive, tt.parentName, getenv); got != tt.want {
				t.Fatalf("shouldPauseOnExit() = %v, want %v", got, tt.want)
			}
		})
	}
}
