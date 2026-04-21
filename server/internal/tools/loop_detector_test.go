package tools

import "testing"

func TestToolLoopDetector_IdenticalRepeat(t *testing.T) {
	var detector ToolLoopDetector
	summaries := []string{"status:queued", "status:retrying", "status:waiting", "status:waiting"}
	for i := 0; i < 3; i++ {
		detection := detector.Observe(`exec:{"command":"go test ./..."}`, "retry tests", []string{summaries[i]})
		if detection.Abort {
			t.Fatalf("unexpected abort at iteration %d: %#v", i, detection)
		}
	}

	detection := detector.Observe(`exec:{"command":"go test ./..."}`, "retry tests", []string{summaries[3]})
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

func TestToolLoopDetector_PingPongRequiresStableOutcomes(t *testing.T) {
	var detector ToolLoopDetector
	detections := []ToolLoopDetection{
		detector.Observe(`browser:{"action":"open","url":"https://a.example"}`, "continue", []string{`{"status":"ok","url":"https://a.example"}`}),
		detector.Observe(`browser:{"action":"open","url":"https://b.example"}`, "continue", []string{`{"status":"ok","url":"https://b.example"}`}),
		detector.Observe(`browser:{"action":"open","url":"https://a.example"}`, "continue", []string{`{"status":"ok","url":"https://a.example/updated"}`}),
		detector.Observe(`browser:{"action":"open","url":"https://b.example"}`, "continue", []string{`{"status":"ok","url":"https://b.example"}`}),
	}
	for i, detection := range detections {
		if detection.Abort {
			t.Fatalf("unexpected ping-pong abort at iteration %d: %#v", i, detection)
		}
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

func TestToolLoopDetector_ProgressFingerprintPreventsFalseNoProgressAbort(t *testing.T) {
	var detector ToolLoopDetector
	rounds := []string{
		`read:path=README.md start_line=1 end_line=20`,
		`read:path=README.md start_line=21 end_line=40`,
		`read:path=README.md start_line=41 end_line=60`,
	}
	for i, sig := range rounds {
		detection := detector.Observe(sig, "continue reading", []string{`{"status":"completed","path":"README.md"}`})
		if detection.Abort {
			t.Fatalf("unexpected abort for progressive read fingerprint at iteration %d: %#v", i, detection)
		}
	}
}

func TestToolLoopDetector_ProgressMarkersResetLoopState(t *testing.T) {
	var detector ToolLoopDetector
	for i := 0; i < 2; i++ {
		detection := detector.Observe(`web_fetch:{"url":"https://example.com/a"}`, "keep going", []string{`{"status":"completed","url":"https://example.com/a"}`})
		if detection.Abort {
			t.Fatalf("unexpected abort before progress marker at iteration %d: %#v", i, detection)
		}
	}

	if detection := detector.Observe(
		`write:{"path":"market_research.md"}`,
		"save report",
		[]string{`{"success":true,"path":"market_research.md"}`},
		"market_research.md",
	); detection.Abort {
		t.Fatalf("expected progress marker to reset detector, got %#v", detection)
	}

	for i := 0; i < 2; i++ {
		detection := detector.Observe(`web_fetch:{"url":"https://example.com/a"}`, "keep going", []string{`{"status":"completed","url":"https://example.com/a"}`})
		if detection.Abort {
			t.Fatalf("unexpected abort after reset at iteration %d: %#v", i, detection)
		}
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
	if got := NormalizeToolProgressSummary(`{"status":"completed","url":"https://example.com/path","results":[1,2,3]}`); got != "status:completed|url:https://example.com/path|results_len:#" {
		t.Fatalf("unexpected structured summary: %q", got)
	}
}
