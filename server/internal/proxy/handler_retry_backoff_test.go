package proxy

import (
	"testing"
	"time"
)

func TestTransientUpstreamRetryDelay(t *testing.T) {
	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{attempt: 0, want: 500 * time.Millisecond},
		{attempt: 1, want: 1500 * time.Millisecond},
		{attempt: 2, want: 3 * time.Second},
	}

	for _, tc := range tests {
		if got := transientUpstreamRetryDelay(tc.attempt); got != tc.want {
			t.Fatalf("transientUpstreamRetryDelay(%d) = %s, want %s", tc.attempt, got, tc.want)
		}
	}
}
