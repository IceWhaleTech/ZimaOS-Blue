package harness

import "testing"

func TestDeriveVerificationObservations_ClassifiesProviderOverloadAsInfraBlocked(t *testing.T) {
	observations := deriveVerificationObservations(&Run{
		Status: RunStatusFailed,
		Error:  "planning failed: proxy returned 529: upstream 503: {\"error\":{\"type\":\"system_cpu_overloaded\",\"message\":\"system cpu overloaded\"}}",
	}, nil, nil, nil)

	if !observationSeen(observations, "provider_infra_blocked") {
		t.Fatalf("observations = %#v, want provider_infra_blocked", observations)
	}
	label, summary := processObservationFailureLabel(nil, nil, nil, observations)
	if label != "infra_provider_blocked" {
		t.Fatalf("label = %q, want infra_provider_blocked (summary=%q, observations=%#v)", label, summary, observations)
	}
}

func TestDeriveVerificationObservations_CapturesPlannerMemorySignals(t *testing.T) {
	observations := deriveVerificationObservations(&Run{Status: RunStatusCompleted}, []RunEvent{
		{Type: "task_planner_memory_skipped", Message: "public_web"},
		{Type: "task_planner_memory_filtered_session_compaction", Message: "1"},
		{Type: "task_planner_memory_used", Message: "1:long_term"},
	}, nil, nil)

	for _, want := range []string{
		"planner_memory_skipped",
		"planner_memory_skipped_public_web",
		"planner_memory_session_compaction_filtered",
		"planner_memory_used",
	} {
		if !observationSeen(observations, want) {
			t.Fatalf("observations = %#v, want %q", observations, want)
		}
	}
}

func TestDeriveVerificationObservations_FlagsEmptyEvidenceCollectionForRefineQueryNoResults(t *testing.T) {
	events := []RunEvent{
		{
			Type:        "tool_requested",
			ToolName:    "bash",
			PayloadJSON: `{"tool_call_id":"tc-1","arguments":{"command":"blue web_query query=\"OpenAI Responses API site:platform.openai.com\""}}`,
		},
		{
			Type:        "tool_finished",
			ToolName:    "bash",
			PayloadJSON: `{"tool_call_id":"tc-1","result":"{\"stdout\":\"sources: []\nnext_action: refine_query\nstatus: partial\nfinal_url: \ntarget_url: \n\",\"data\":{\"sources\":\"[]\",\"status\":\"partial\",\"next_action\":\"refine_query\",\"final_url\":\"\",\"target_url\":\"\",\"content\":\"\"}}"}`,
		},
	}
	observations := deriveVerificationObservations(&Run{Status: RunStatusCompleted}, events, nil, []string{"bash"})
	if !observationSeen(observations, "evidence_collection_empty") {
		t.Fatalf("observations = %#v, want evidence_collection_empty", observations)
	}
	label, summary := processObservationFailureLabel(nil, &RunGroupItem{
		Expected: map[string]interface{}{
			"required_observations": []interface{}{"evidence_tool_used"},
		},
	}, &Run{Kind: RunKindAgentTask, Status: RunStatusCompleted}, observations)
	if label != "missing_evidence_collection" {
		t.Fatalf("label = %q, want missing_evidence_collection (summary=%q, observations=%#v)", label, summary, observations)
	}
}

func TestDeriveVerificationObservations_DoesNotFlagEmptyEvidenceCollectionWhenFinalURLPresent(t *testing.T) {
	events := []RunEvent{
		{
			Type:        "tool_requested",
			ToolName:    "bash",
			PayloadJSON: `{"tool_call_id":"tc-1","arguments":{"command":"blue web_query query=\"OpenAI Responses API latest documentation site:platform.openai.com\""}}`,
		},
		{
			Type:        "tool_finished",
			ToolName:    "bash",
			PayloadJSON: `{"tool_call_id":"tc-1","result":"{\"stdout\":\"status: ok\nfinal_url: https://platform.openai.com/docs/api-reference/responses\nsources: [{\\\"url\\\":\\\"https://platform.openai.com/docs/api-reference/responses\\\"}]\",\"data\":{\"status\":\"ok\",\"final_url\":\"https://platform.openai.com/docs/api-reference/responses\",\"sources\":[{\"url\":\"https://platform.openai.com/docs/api-reference/responses\",\"selected\":true}]}}"}`,
		},
	}
	observations := deriveVerificationObservations(&Run{Status: RunStatusCompleted}, events, nil, []string{"bash"})
	if observationSeen(observations, "evidence_collection_empty") {
		t.Fatalf("observations = %#v, do not want evidence_collection_empty", observations)
	}
}

func TestDeriveVerificationObservations_FlagsUnknownEvidenceVerification(t *testing.T) {
	run := &Run{
		Status: RunStatusCompleted,
		Kind:   RunKindAgentTask,
		Result: "Summary: The task completed with grounded, evidence-bound outputs.\n\nVerified:\n1. unknown\n4. Verification: PASS\nCriteria results:\n- [PASS] official public documentation is retrieved or directly verified -- unknown\n\nVerification errors:\n- responder returned empty content\n\nThe task completed with all success criteria marked PASS, but verification passed on 'unknown' evidence values.",
	}
	observations := deriveVerificationObservations(run, nil, nil, []string{"web_query"})
	if !observationSeen(observations, "verification_responder_empty") {
		t.Fatalf("observations = %#v, want verification_responder_empty", observations)
	}
	if !observationSeen(observations, "verification_unknown_evidence") {
		t.Fatalf("observations = %#v, want verification_unknown_evidence", observations)
	}
	label, summary := processObservationFailureLabel(nil, &RunGroupItem{
		Expected: map[string]interface{}{
			"required_observations": []interface{}{"evidence_tool_used"},
		},
	}, run, observations)
	if label != "missing_evidence_collection" {
		t.Fatalf("label = %q, want missing_evidence_collection (summary=%q, observations=%#v)", label, summary, observations)
	}
	if summary == "" {
		t.Fatalf("expected failure summary for unknown evidence observations, got empty string")
	}
}

func TestDeriveVerificationObservations_CapturesDesktopChatSuccessSignals(t *testing.T) {
	run := &Run{
		Status: RunStatusCompleted,
		Result: `{
			"stage":"verify_outcome",
			"strategy":"visual_verification",
			"attempt_count":6,
			"grounding_source":"vision_model",
			"verification":{"status":"sent"},
			"task_stages":[
				{"stage":"activate_app","status":"ok","strategy":"activate_app_retry"},
				{"stage":"acquire_window","status":"ok","strategy":"window_resolve"},
				{"stage":"locate_conversation","status":"ok","strategy":"visual_sidebar_hit","grounding_source":"vision_model"},
				{"stage":"confirm_conversation","status":"ok","strategy":"post_click_confirmation"},
				{"stage":"locate_composer","status":"ok","strategy":"visual_grounding_check","grounding_source":"vision_model"},
				{"stage":"verify_outcome","status":"ok","strategy":"visual_verification","grounding_source":"vision_model","verification":{"status":"sent"}}
			],
			"artifact_paths":["artifacts/computer_use/run-1/step-01/06-stage_trace.json"]
		}`,
	}
	artifacts := []ArtifactRef{
		{Kind: "file", Label: "stage_trace", PathOrURL: "/tmp/stage_trace.json", MIMEType: "application/json"},
		{Kind: "file", Label: "final_result", PathOrURL: "/tmp/final_result.json", MIMEType: "application/json"},
		{Kind: "file", Label: "key_screenshot", PathOrURL: "/tmp/key_screenshot.bin", MIMEType: "application/octet-stream"},
	}

	observations := deriveVerificationObservations(run, nil, artifacts, []string{"computer_use"})
	for _, want := range []string{
		"task_stage_trace_emitted",
		"computer_use_metadata_emitted",
		"send_verified",
		"conversation_confirmed",
		"focus_recovered",
		"visual_grounding_used",
	} {
		if !observationSeen(observations, want) {
			t.Fatalf("observations = %#v, want %q", observations, want)
		}
	}
}

func TestDeriveVerificationObservations_AcceptsDesktopChatDeliveredStatusAlias(t *testing.T) {
	run := &Run{
		Status: RunStatusCompleted,
		Result: `{
			"stage":"verify_outcome",
			"strategy":"visual_verification",
			"attempt_count":6,
			"grounding_source":"vision_model",
			"verification":{"status":"delivered"},
			"task_stages":[
				{"stage":"activate_app","status":"ok","strategy":"activate_app_retry"},
				{"stage":"acquire_window","status":"ok","strategy":"window_resolve"},
				{"stage":"locate_conversation","status":"ok","strategy":"visual_sidebar_hit","grounding_source":"vision_model"},
				{"stage":"confirm_conversation","status":"ok","strategy":"post_click_confirmation"},
				{"stage":"locate_composer","status":"ok","strategy":"visual_grounding_check","grounding_source":"vision_model"},
				{"stage":"verify_outcome","status":"ok","strategy":"visual_verification","grounding_source":"vision_model","verification":{"status":"delivered"}}
			],
			"artifact_paths":["artifacts/computer_use/run-1/step-01/06-stage_trace.json"]
		}`,
	}
	artifacts := []ArtifactRef{
		{Kind: "file", Label: "stage_trace", PathOrURL: "/tmp/stage_trace.json", MIMEType: "application/json"},
		{Kind: "file", Label: "final_result", PathOrURL: "/tmp/final_result.json", MIMEType: "application/json"},
		{Kind: "file", Label: "key_screenshot", PathOrURL: "/tmp/key_screenshot.bin", MIMEType: "application/octet-stream"},
	}

	observations := deriveVerificationObservations(run, nil, artifacts, []string{"computer_use"})
	if !observationSeen(observations, "send_verified") {
		t.Fatalf("observations = %#v, want send_verified for delivered alias", observations)
	}
}

func TestDeriveVerificationObservations_CapturesDesktopChatFailClosedSignals(t *testing.T) {
	run := &Run{
		Status: RunStatusCompleted,
		Result: `{
			"stage":"confirm_conversation",
			"strategy":"post_click_confirmation",
			"attempt_count":4,
			"failure_code":"search_box_still_active",
			"task_stages":[
				{"stage":"activate_app","status":"ok","strategy":"reuse_existing_window"},
				{"stage":"acquire_window","status":"ok","strategy":"window_resolve"},
				{"stage":"locate_conversation","status":"ok","strategy":"structured_match"},
				{"stage":"confirm_conversation","status":"terminal_failure","strategy":"post_click_confirmation","failure_code":"search_box_still_active"}
			],
			"artifact_paths":["artifacts/computer_use/run-1/step-01/04-stage_trace.json"]
		}`,
	}
	artifacts := []ArtifactRef{
		{Kind: "file", Label: "stage_trace", PathOrURL: "/tmp/stage_trace.json", MIMEType: "application/json"},
		{Kind: "file", Label: "failure_classification", PathOrURL: "/tmp/failure_classification.json", MIMEType: "application/json"},
	}

	observations := deriveVerificationObservations(run, nil, artifacts, []string{"computer_use"})
	for _, want := range []string{
		"task_stage_trace_emitted",
		"computer_use_metadata_emitted",
		"safe_fail_closed",
		"failure_code_search_box_still_active",
	} {
		if !observationSeen(observations, want) {
			t.Fatalf("observations = %#v, want %q", observations, want)
		}
	}
}
