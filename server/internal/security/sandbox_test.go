package security

import (
	"strings"
	"testing"
)

func TestEnvSanitizer_SanitizeEnv(t *testing.T) {
	tests := []struct {
		name        string
		sanitizer   *EnvSanitizer
		env         map[string]string
		wantKeys    []string
		wantErr     bool
		errContains string
	}{
		{
			name:      "basic env vars",
			sanitizer: DefaultEnvSanitizer(),
			env: map[string]string{
				"HOME":   "/home/user",
				"PATH":   "/usr/bin",
				"EDITOR": "vim",
			},
			wantKeys: []string{"HOME", "PATH", "EDITOR"},
			wantErr:  false,
		},
		{
			name:      "filters LD_PRELOAD",
			sanitizer: DefaultEnvSanitizer(),
			env: map[string]string{
				"HOME":       "/home/user",
				"LD_PRELOAD": "/evil/lib.so",
			},
			wantKeys: []string{"HOME"},
			wantErr:  false,
		},
		{
			name:      "filters LD_LIBRARY_PATH",
			sanitizer: DefaultEnvSanitizer(),
			env: map[string]string{
				"HOME":            "/home/user",
				"LD_LIBRARY_PATH": "/evil/lib",
			},
			wantKeys: []string{"HOME"},
			wantErr:  false,
		},
		{
			name:      "filters DYLD_INSERT_LIBRARIES",
			sanitizer: DefaultEnvSanitizer(),
			env: map[string]string{
				"HOME":                   "/home/user",
				"DYLD_INSERT_LIBRARIES":  "/evil/lib.dylib",
			},
			wantKeys: []string{"HOME"},
			wantErr:  false,
		},
		{
			name: "invalid env key",
			sanitizer: DefaultEnvSanitizer(),
			env: map[string]string{
				"VALID_KEY":   "value",
				"123INVALID":  "value", // Starts with digit
			},
			wantErr:     true,
			errContains: "invalid environment variable name",
		},
		{
			name: "value too long",
			sanitizer: &EnvSanitizer{
				MaxValueLength: 10,
			},
			env: map[string]string{
				"KEY": "this value is way too long",
			},
			wantErr:     true,
			errContains: "value too long",
		},
		{
			name: "allowlist filtering",
			sanitizer: &EnvSanitizer{
				AllowedEnvVars: []string{"HOME", "PATH"},
			},
			env: map[string]string{
				"HOME":   "/home/user",
				"PATH":   "/usr/bin",
				"EDITOR": "vim", // Not in allowlist
			},
			wantKeys: []string{"HOME", "PATH"},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.sanitizer.SanitizeEnv(tt.env)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Check that expected keys are present
			for _, key := range tt.wantKeys {
				if _, ok := result[key]; !ok {
					t.Errorf("expected key %s not found in result", key)
				}
			}

			// Check that no extra keys are present
			if len(result) != len(tt.wantKeys) {
				t.Errorf("result has %d keys, want %d", len(result), len(tt.wantKeys))
			}
		})
	}
}

func TestBuildDockerExecArgs(t *testing.T) {
	tests := []struct {
		name           string
		params         DockerExecParams
		wantContains   []string
		wantNotContain []string
	}{
		{
			name: "basic command",
			params: DockerExecParams{
				ContainerName: "test-container",
				Command:       "echo hello",
				Env:           map[string]string{},
			},
			wantContains:   []string{"exec", "test-container", "sh", "-lc", "echo hello"},
			wantNotContain: []string{"ZIMAOS_PREPEND_PATH"},
		},
		{
			name: "with PATH - uses env var reference",
			params: DockerExecParams{
				ContainerName: "test-container",
				Command:       "echo hello",
				Env: map[string]string{
					"PATH": "/custom/bin:/usr/bin",
				},
			},
			wantContains: []string{
				"ZIMAOS_PREPEND_PATH=/custom/bin:/usr/bin",
				"${ZIMAOS_PREPEND_PATH}",
				"unset ZIMAOS_PREPEND_PATH",
			},
			wantNotContain: []string{},
		},
		{
			name: "PATH injection attempt - value not interpolated",
			params: DockerExecParams{
				ContainerName: "test-container",
				Command:       "echo hello",
				Env: map[string]string{
					"PATH": "$(touch /tmp/pwned)",
				},
			},
			wantContains: []string{
				"ZIMAOS_PREPEND_PATH=$(touch /tmp/pwned)", // Passed as env var, not interpolated
				"${ZIMAOS_PREPEND_PATH}",                  // Reference, not value
			},
			wantNotContain: []string{},
		},
		{
			name: "with TTY",
			params: DockerExecParams{
				ContainerName: "test-container",
				Command:       "bash",
				TTY:           true,
			},
			wantContains: []string{"-t"},
		},
		{
			name: "with interactive",
			params: DockerExecParams{
				ContainerName: "test-container",
				Command:       "bash",
				Interactive:   true,
			},
			wantContains: []string{"-i"},
		},
		{
			name: "with other env vars",
			params: DockerExecParams{
				ContainerName: "test-container",
				Command:       "echo $HOME",
				Env: map[string]string{
					"HOME": "/home/user",
				},
			},
			wantContains: []string{"HOME=/home/user"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := BuildDockerExecArgs(tt.params)
			argsStr := strings.Join(args, " ")

			for _, want := range tt.wantContains {
				if !strings.Contains(argsStr, want) {
					t.Errorf("args %q does not contain %q", argsStr, want)
				}
			}

			for _, notWant := range tt.wantNotContain {
				if strings.Contains(argsStr, notWant) {
					t.Errorf("args %q should not contain %q", argsStr, notWant)
				}
			}
		})
	}
}

func TestBuildDockerExecArgs_PathInjectionPrevention(t *testing.T) {
	// This test specifically verifies that PATH values are not directly
	// interpolated into the shell command, preventing injection attacks.

	injectionPayload := "$(touch /tmp/clawdbot-path-injection)"

	params := DockerExecParams{
		ContainerName: "test-container",
		Command:       "echo hello",
		Env: map[string]string{
			"PATH": injectionPayload,
			"HOME": "/home/user",
		},
	}

	args := BuildDockerExecArgs(params)
	commandArg := args[len(args)-1] // Last arg is the shell command

	// The injection payload should NOT appear directly in the command
	// It should only appear in the env var assignment
	if strings.Contains(commandArg, injectionPayload) {
		t.Errorf("command arg contains injection payload directly: %s", commandArg)
	}

	// The command should use the env var reference
	if !strings.Contains(commandArg, "ZIMAOS_PREPEND_PATH") {
		t.Errorf("command arg should reference ZIMAOS_PREPEND_PATH: %s", commandArg)
	}

	// The env var assignment should be in the args (before the command)
	argsStr := strings.Join(args, " ")
	if !strings.Contains(argsStr, "ZIMAOS_PREPEND_PATH="+injectionPayload) {
		t.Errorf("env var assignment not found in args: %s", argsStr)
	}
}

func TestSanitizeCommand(t *testing.T) {
	tests := []struct {
		name    string
		cmd     string
		wantErr bool
	}{
		{
			name:    "simple command",
			cmd:     "ls",
			wantErr: false,
		},
		{
			name:    "command with args",
			cmd:     "ls -la /tmp",
			wantErr: false,
		},
		{
			name:    "empty command",
			cmd:     "",
			wantErr: true,
		},
		{
			name:    "command substitution $(",
			cmd:     "echo $(whoami)",
			wantErr: true,
		},
		{
			name:    "backtick substitution",
			cmd:     "echo `whoami`",
			wantErr: true,
		},
		{
			name:    "command chaining &&",
			cmd:     "ls && rm -rf /",
			wantErr: true,
		},
		{
			name:    "command chaining ||",
			cmd:     "ls || rm -rf /",
			wantErr: true,
		},
		{
			name:    "semicolon separator",
			cmd:     "ls; rm -rf /",
			wantErr: true,
		},
		{
			name:    "pipe",
			cmd:     "ls | grep foo",
			wantErr: true,
		},
		{
			name:    "redirect output",
			cmd:     "echo foo > /etc/passwd",
			wantErr: true,
		},
		{
			name:    "redirect input",
			cmd:     "cat < /etc/passwd",
			wantErr: true,
		},
		{
			name:    "newline injection",
			cmd:     "ls\nrm -rf /",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SanitizeCommand(tt.cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("SanitizeCommand(%q) error = %v, wantErr %v", tt.cmd, err, tt.wantErr)
			}
		})
	}
}

func TestSanitizeCommandArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "simple args",
			args:    []string{"-l", "-a", "/tmp"},
			wantErr: false,
		},
		{
			name:    "empty args",
			args:    []string{},
			wantErr: false,
		},
		{
			name:    "null byte",
			args:    []string{"foo\x00bar"},
			wantErr: true,
		},
		{
			name:    "newline",
			args:    []string{"foo\nbar"},
			wantErr: true,
		},
		{
			name:    "carriage return",
			args:    []string{"foo\rbar"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SanitizeCommandArgs(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("SanitizeCommandArgs(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
			}
		})
	}
}

func TestIsValidEnvKey(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"HOME", true},
		{"PATH", true},
		{"MY_VAR", true},
		{"_PRIVATE", true},
		{"var123", true},
		{"", false},
		{"123VAR", false},    // Starts with digit
		{"MY-VAR", false},    // Contains hyphen
		{"MY.VAR", false},    // Contains dot
		{"MY VAR", false},    // Contains space
		{"MY=VAR", false},    // Contains equals
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if got := isValidEnvKey(tt.key); got != tt.want {
				t.Errorf("isValidEnvKey(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}
