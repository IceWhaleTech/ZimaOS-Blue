//go:build darwin

package sandbox

import (
	"strings"
	"testing"
)

func skipIfSandboxExecUnavailable(t *testing.T, result *ExecutionResult, err error) {
	t.Helper()
	var msg strings.Builder
	if err != nil {
		msg.WriteString(err.Error())
	}
	if result != nil {
		if result.Error != "" {
			if msg.Len() > 0 {
				msg.WriteByte('\n')
			}
			msg.WriteString(result.Error)
		}
		if result.Stderr != "" {
			if msg.Len() > 0 {
				msg.WriteByte('\n')
			}
			msg.WriteString(result.Stderr)
		}
	}
	combined := msg.String()
	if strings.Contains(combined, ErrSandboxNotSupported.Error()) ||
		strings.Contains(combined, "sandbox-exec: sandbox_apply: Operation not permitted") ||
		strings.Contains(combined, "executable file not found in $PATH") {
		t.Skip("sandbox-exec unavailable in this environment")
	}
}
