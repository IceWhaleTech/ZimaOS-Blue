package server

import (
	"sync"
	"testing"
)

func resetChatContextRegexesForTest(t *testing.T) {
	t.Helper()

	reChineseRef = nil
	reEnglishRef = nil
	reContinuation = nil
	reChineseEllipsisRef = nil
	reMemoryCue = nil
	reTopicSwitchCue = nil
	chatContextReferenceOnce = sync.Once{}
	chatContextMemoryCueOnce = sync.Once{}
	chatContextTopicSwitchOnce = sync.Once{}

	t.Cleanup(func() {
		chatContextReferenceOnce = sync.Once{}
		chatContextMemoryCueOnce = sync.Once{}
		chatContextTopicSwitchOnce = sync.Once{}
		ensureChatContextReferenceRegexes()
		ensureChatContextMemoryCueRegex()
		ensureChatContextTopicSwitchRegex()
	})
}

func TestChatContextRegexes_InitializeOnDemand(t *testing.T) {
	resetChatContextRegexesForTest(t)

	if reChineseRef != nil || reEnglishRef != nil || reContinuation != nil || reChineseEllipsisRef != nil || reMemoryCue != nil || reTopicSwitchCue != nil {
		t.Fatal("expected chat context regexes to start nil")
	}

	if !hasReference("继续说这个方案") {
		t.Fatal("expected reference detection to work after lazy init")
	}
	if reChineseRef == nil || reEnglishRef == nil || reContinuation == nil || reChineseEllipsisRef == nil {
		t.Fatal("expected reference regexes to initialize on first access")
	}
	if reMemoryCue != nil || reTopicSwitchCue != nil {
		t.Fatal("expected unrelated chat context regexes to remain cold")
	}

	if !hasMemoryCue("Remember my preference for concise release notes.") {
		t.Fatal("expected memory cue detection to work after lazy init")
	}
	if reMemoryCue == nil {
		t.Fatal("expected memory cue regex to initialize on first access")
	}
	if reTopicSwitchCue != nil {
		t.Fatal("expected topic-switch regex to remain cold until accessed")
	}

	if !hasTopicSwitchCue("换个话题，我们聊点别的。") {
		t.Fatal("expected topic-switch detection to work after lazy init")
	}
	if reTopicSwitchCue == nil {
		t.Fatal("expected topic-switch regex to initialize on first access")
	}
}
