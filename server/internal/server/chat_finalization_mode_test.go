package server

import "testing"

func TestResolveFinalStreamContentMode_MergeWhenProcessCardsMissing(t *testing.T) {
	previous := "```typeless\n" +
		`{"type":"browser-progress","id":"browser-progress-chain","steps":[{"step":"navigate","name":"Navigating","status":"completed"}]}` +
		"\n```\n\n" +
		"```typeless\n" +
		`{"type":"web-fetch","id":"web-fetch-chain","title":"web_fetch","status":"success"}` +
		"\n```"
	final := "```typeless\n" +
		`{"type":"web-fetch","id":"web-fetch-chain","title":"web_fetch","status":"success"}` +
		"\n```"

	if got := resolveFinalStreamContentMode(previous, final); got != finalizationModeMergeProcessCards {
		t.Fatalf("resolveFinalStreamContentMode() = %q, want %q", got, finalizationModeMergeProcessCards)
	}
}

func TestResolveFinalStreamContentMode_ReplaceWhenNothingMissing(t *testing.T) {
	previous := "```typeless\n" +
		`{"type":"deep-research-progress","id":"deep-research-progress-job-1","job_id":"job-1"}` +
		"\n```\n\n" +
		"```typeless\n" +
		`{"type":"deep-research","id":"deep-research-result-job-1","job_id":"job-1"}` +
		"\n```"
	final := previous

	if got := resolveFinalStreamContentMode(previous, final); got != finalizationModeReplace {
		t.Fatalf("resolveFinalStreamContentMode() = %q, want %q", got, finalizationModeReplace)
	}
}

func TestResolveFinalStreamContentMode_MergeWhenFinalKeepsOnlySubsetOfProcessCards(t *testing.T) {
	previous := "```typeless\n" +
		`{"type":"deep-research-progress","id":"deep-research-progress-job-1","job_id":"job-1","stage":"retrieve"}` +
		"\n```\n\n" +
		"```typeless\n" +
		`{"type":"deep-research-event","id":"deep-research-event-job-1-01","job_id":"job-1","event_kind":"planning"}` +
		"\n```\n\n" +
		"```typeless\n" +
		`{"type":"deep-research","id":"deep-research-result-job-1","job_id":"job-1"}` +
		"\n```"
	final := "```typeless\n" +
		`{"type":"deep-research-progress","id":"deep-research-progress-job-1","job_id":"job-1","stage":"fullcontext"}` +
		"\n```\n\n" +
		"```typeless\n" +
		`{"type":"deep-research","id":"deep-research-result-job-1","job_id":"job-1"}` +
		"\n```"

	if got := resolveFinalStreamContentMode(previous, final); got != finalizationModeMergeProcessCards {
		t.Fatalf("resolveFinalStreamContentMode() = %q, want %q", got, finalizationModeMergeProcessCards)
	}
}
