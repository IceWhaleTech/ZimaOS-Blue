package harness

import "testing"

func TestWorkflowResumeInputDescriptorApprovalCheckpoint(t *testing.T) {
	run := &Run{
		Kind: RunKindWorkflow,
		Metadata: map[string]interface{}{
			"workflow_checkpoint_kind":      "pause_for_approval",
			"workflow_status_reason":        "approval_needed",
			"workflow_checkpoint_node_name": "Manager Review",
		},
	}

	input := workflowResumeInputDescriptor(run)
	if input == nil {
		t.Fatal("workflowResumeInputDescriptor() = nil, want descriptor")
	}
	if input.Title != "Review approval for Manager Review" {
		t.Fatalf("title = %q, want approval title", input.Title)
	}
	if input.Description == "" || input.SubmitLabel != "Submit decision" {
		t.Fatalf("input labels = %#v, want approval dialog copy", input)
	}
	if len(input.Fields) != 2 || input.Fields[0].Target != "decision" || input.Fields[1].PayloadKey != "comment" {
		t.Fatalf("fields = %#v, want decision/comment schema", input.Fields)
	}
	if input.Fields[0].Kind != "choice" || len(input.Fields[0].Options) != 2 || input.Fields[0].Options[0] != "approve" || input.Fields[0].Options[1] != "reject" {
		t.Fatalf("fields = %#v, want approve/reject decision options", input.Fields)
	}
}

func TestWorkflowResumeInputDescriptorClarificationCheckpoint(t *testing.T) {
	run := &Run{
		Kind: RunKindWorkflow,
		Metadata: map[string]interface{}{
			"workflow_checkpoint_kind": "pause_for_question",
			"workflow_status_reason":   "clarify_missing_input",
		},
	}

	input := workflowResumeInputDescriptor(run)
	if input == nil {
		t.Fatal("workflowResumeInputDescriptor() = nil, want descriptor")
	}
	if input.Title != "Provide clarification" || input.SubmitLabel != "Send response" {
		t.Fatalf("input labels = %#v, want clarification dialog copy", input)
	}
	if len(input.Fields) != 1 || input.Fields[0].Kind != "textarea" || input.Fields[0].PayloadKey != "response" || !input.Fields[0].Required {
		t.Fatalf("fields = %#v, want clarification response schema", input.Fields)
	}
	if input.Fields[0].Placeholder != "Provide the missing detail" {
		t.Fatalf("placeholder = %q, want clarification hint", input.Fields[0].Placeholder)
	}
}

func TestWorkflowResumeInputDescriptorJSONCheckpoint(t *testing.T) {
	run := &Run{
		Kind: RunKindWorkflow,
		Metadata: map[string]interface{}{
			"workflow_checkpoint_kind": "json_task",
		},
	}

	input := workflowResumeInputDescriptor(run)
	if input == nil {
		t.Fatal("workflowResumeInputDescriptor() = nil, want descriptor")
	}
	if input.Title != "Provide JSON payload" || input.SubmitLabel != "Submit payload" {
		t.Fatalf("input labels = %#v, want json payload dialog copy", input)
	}
	if len(input.Fields) != 1 || input.Fields[0].Kind != "json" || input.Fields[0].Target != "payload_root" || !input.Fields[0].Required {
		t.Fatalf("fields = %#v, want json payload schema", input.Fields)
	}
	if input.Fields[0].Placeholder != `{"result":"Provide the requested JSON payload"}` {
		t.Fatalf("placeholder = %q, want json payload hint", input.Fields[0].Placeholder)
	}
}

func TestAgentTaskSendUpdateInputDescriptor(t *testing.T) {
	input := agentTaskSendUpdateInputDescriptor()
	if input == nil {
		t.Fatal("agentTaskSendUpdateInputDescriptor() = nil, want descriptor")
	}
	if input.Title != "Send update" || input.SubmitLabel != "Send update" {
		t.Fatalf("input labels = %#v, want send update copy", input)
	}
	if len(input.Fields) != 1 {
		t.Fatalf("fields = %#v, want one message field", input.Fields)
	}
	if input.Fields[0].Target != "root" || input.Fields[0].Key != "message" || !input.Fields[0].Required {
		t.Fatalf("fields = %#v, want required root message field", input.Fields)
	}
}
