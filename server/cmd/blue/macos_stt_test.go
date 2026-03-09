//go:build darwin

package main

import "testing"

func TestShouldSkipMacOSSTTAuthorization(t *testing.T) {
	tests := []struct {
		name        string
		env         string
		interactive bool
		wantSkip    bool
		wantReason  string
	}{
		{
			name:        "env override skips auth",
			env:         "1",
			interactive: true,
			wantSkip:    true,
			wantReason:  "env:ZIMA_SKIP_STT_AUTH",
		},
		{
			name:        "non interactive skips auth",
			env:         "",
			interactive: false,
			wantSkip:    true,
			wantReason:  "non-interactive",
		},
		{
			name:        "interactive keeps auth",
			env:         "",
			interactive: true,
			wantSkip:    false,
			wantReason:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skip, reason := shouldSkipMacOSSTTAuthorization(func(key string) string {
				if key == "ZIMA_SKIP_STT_AUTH" {
					return tt.env
				}
				return ""
			}, tt.interactive)
			if skip != tt.wantSkip {
				t.Fatalf("skip = %v, want %v", skip, tt.wantSkip)
			}
			if reason != tt.wantReason {
				t.Fatalf("reason = %q, want %q", reason, tt.wantReason)
			}
		})
	}
}
