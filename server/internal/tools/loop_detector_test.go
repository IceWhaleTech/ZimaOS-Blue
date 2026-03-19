package tools

import "testing"

func TestToolLoopDetector_IdenticalRepeat(t *testing.T) {
	var detector ToolLoopDetector
	summaries := []string{"status:queued", "status:retrying", "status:waiting"}
	for i := 0; i < 2; i++ {
		detection := detector.Observe(`exec:{"command":"go test ./..."}`, "retry tests", []string{summaries[i]})
		if detection.Abort {
			t.Fatalf("unexpected abort at iteration %d: %#v", i, detection)
		}
	}

	detection := detector.Observe(`exec:{"command":"go test ./..."}`, "retry tests", []string{summaries[2]})
	if !detection.Abort || detection.Reason != ToolLoopReasonIdenticalRepeat {
		t.Fatalf("expected identical repeat abort, got %#v", detection)
	}
}

func TestToolLoopDetector_ErrorRepeat(t *testing.T) {
	var detector ToolLoopDetector
	for i := 0; i < 2; i++ {
		detection := detector.Observe(`web_search:{"query":"blue"}`, "search again", []string{`{"error":"rate limited"}`})
		if detection.Abort {
			t.Fatalf("unexpected abort at iteration %d: %#v", i, detection)
		}
	}

	detection := detector.Observe(`web_search:{"query":"blue"}`, "search again", []string{`{"error":"rate limited"}`})
	if !detection.Abort || detection.Reason != ToolLoopReasonErrorRepeat {
		t.Fatalf("expected error repeat abort, got %#v", detection)
	}
}

func TestToolLoopDetector_PingPong(t *testing.T) {
	var detector ToolLoopDetector
	rounds := []string{
		`browser:{"action":"open","url":"https://a.example"}`,
		`browser:{"action":"open","url":"https://b.example"}`,
		`browser:{"action":"open","url":"https://a.example"}`,
		`browser:{"action":"open","url":"https://b.example"}`,
	}
	for i := 0; i < len(rounds)-1; i++ {
		detection := detector.Observe(rounds[i], "continue", []string{"status:ok"})
		if detection.Abort {
			t.Fatalf("unexpected abort at iteration %d: %#v", i, detection)
		}
	}

	detection := detector.Observe(rounds[len(rounds)-1], "continue", []string{"status:ok"})
	if !detection.Abort || detection.Reason != ToolLoopReasonPingPong {
		t.Fatalf("expected ping-pong abort, got %#v", detection)
	}
}

func TestToolLoopDetector_PollingNoProgress(t *testing.T) {
	var detector ToolLoopDetector
	for i := 0; i < 2; i++ {
		detection := detector.Observe(`web_search:{"query":"blue latest"}`, "keep polling", []string{`{"results":[]}`})
		if detection.Abort {
			t.Fatalf("unexpected abort at iteration %d: %#v", i, detection)
		}
	}

	detection := detector.Observe(`web_search:{"query":"blue latest"}`, "keep polling", []string{`{"results":[]}`})
	if !detection.Abort || detection.Reason != ToolLoopReasonPollingNoProgress {
		t.Fatalf("expected no-progress abort, got %#v", detection)
	}
}

func TestNormalizeToolProgressSummary(t *testing.T) {
	if got := NormalizeToolProgressSummary(`{"error":"command exited with code 1"}`); got != "error:command exited with code #" {
		t.Fatalf("unexpected error summary: %q", got)
	}
	if got := NormalizeToolProgressSummary(`{"status":"completed"}`); got != "status:completed" {
		t.Fatalf("unexpected status summary: %q", got)
	}
	if got := NormalizeToolProgressSummary(`{"summary":"Found 12 results"}`); got != "summary:found # results" {
		t.Fatalf("unexpected summary summary: %q", got)
	}
}
