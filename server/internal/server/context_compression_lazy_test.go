package server

import (
	"sync"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

func TestContextCompressionRegexes_InitializeOnDemand(t *testing.T) {
	reLatestIntentResume = nil
	reLatestIntentOverride = nil
	reLatestIntentQuestion = nil
	reLatestIntentInvestigate = nil
	reCarryOverItem = nil
	reIdentifierHeavy = nil
	reProcessOnly = nil
	reGoalLike = nil
	reInstructionLike = nil
	reDiscoveryLike = nil
	reAccomplishedLike = nil
	reFormattingOnlyInstruction = nil
	contextCompressionRegexesOnce = sync.Once{}
	t.Cleanup(func() {
		contextCompressionRegexesOnce = sync.Once{}
		ensureContextCompressionRegexes()
	})

	if reLatestIntentResume != nil || reLatestIntentOverride != nil || reLatestIntentQuestion != nil || reLatestIntentInvestigate != nil ||
		reCarryOverItem != nil || reIdentifierHeavy != nil || reProcessOnly != nil || reGoalLike != nil ||
		reInstructionLike != nil || reDiscoveryLike != nil || reAccomplishedLike != nil || reFormattingOnlyInstruction != nil {
		t.Fatal("expected context compression regexes to start nil")
	}

	if !looksLikeFormattingOnlyInstruction("summary please") {
		t.Fatal("expected formatting-only instruction to be recognized after lazy init")
	}
	if !looksLikeProcessOnlyItem("I will inspect the issue") {
		t.Fatal("expected process-only item to be recognized after lazy init")
	}

	decision := latestIntentVsCarryover([]llm.Message{
		{Role: llm.RoleUser, Content: "Continue updating docs/old_plan.md"},
		{Role: llm.RoleAssistant, Content: "I am still editing docs/old_plan.md"},
		{Role: llm.RoleUser, Content: "先解释原因？不要继续旧任务"},
	}, "", []llm.ToolCall{{
		ID:        "call-write-1",
		Name:      "write",
		Arguments: `{"path":"docs/old_plan.md","content":"stale carry-over edit"}`,
	}})
	if !decision.ExplicitOverride || !decision.QuestionLike {
		t.Fatalf("latestIntentVsCarryover() = %+v, want override/question detection", decision)
	}

	marked := markCarryOverItems("Accomplished\n- pending docs/old_plan.md follow-up")
	if marked == "" {
		t.Fatal("expected carry-over summary output")
	}

	section, score, keep := classifyCompressionUnit(llm.RoleUser, "Need to update docs/plan.md before 2026-04-06", 0, 3, extractIntentKeywords("update docs plan"))
	if !keep || score <= 0 || section == "" {
		t.Fatalf("classifyCompressionUnit() = (%q, %d, %v), want kept scored section", section, score, keep)
	}

	if reLatestIntentResume == nil || reLatestIntentOverride == nil || reLatestIntentQuestion == nil || reLatestIntentInvestigate == nil ||
		reCarryOverItem == nil || reIdentifierHeavy == nil || reProcessOnly == nil || reGoalLike == nil ||
		reInstructionLike == nil || reDiscoveryLike == nil || reAccomplishedLike == nil || reFormattingOnlyInstruction == nil {
		t.Fatal("expected context compression regexes to initialize on demand")
	}
}
