package feishu

import (
	"sync"
	"testing"
)

func TestFeishuMentionRegexes_InitializeOnDemand(t *testing.T) {
	originalAtUserIDTagRe := feishuAtUserIDTagRe
	originalAtIDTagRe := feishuAtIDTagRe
	originalOnce := feishuMentionRegexesOnce

	feishuAtUserIDTagRe = nil
	feishuAtIDTagRe = nil
	feishuMentionRegexesOnce = sync.Once{}
	t.Cleanup(func() {
		feishuAtUserIDTagRe = originalAtUserIDTagRe
		feishuAtIDTagRe = originalAtIDTagRe
		feishuMentionRegexesOnce = originalOnce
	})

	if feishuAtUserIDTagRe != nil || feishuAtIDTagRe != nil {
		t.Fatal("expected feishu mention regexes to start nil")
	}

	mentions, mentionIDs := feishuMentionsMetadata(`<at user_id="ou_bot">Blue</at> hello`, nil)
	if len(mentions) != 1 || len(mentionIDs) != 1 || mentionIDs[0] != "ou_bot" {
		t.Fatalf("feishuMentionsMetadata() = (%v, %v), want one mention for ou_bot", mentions, mentionIDs)
	}
	if feishuAtUserIDTagRe == nil || feishuAtIDTagRe == nil {
		t.Fatal("expected feishu mention regexes to initialize on first at-tag fallback")
	}
}
