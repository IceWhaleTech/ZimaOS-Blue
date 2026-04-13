package slack

import (
	"sync"
	"testing"
)

func TestSlackMentionRegex_InitializesOnDemand(t *testing.T) {
	originalPattern := slackUserMentionPattern
	originalOnce := slackUserMentionPatternOnce

	slackUserMentionPattern = nil
	slackUserMentionPatternOnce = sync.Once{}
	t.Cleanup(func() {
		slackUserMentionPattern = originalPattern
		slackUserMentionPatternOnce = originalOnce
	})

	if slackUserMentionPattern != nil {
		t.Fatal("expected slack mention regex to start nil")
	}

	mentions, ids := slackExtractMentions("hello <@U999> and <@U888>")
	if len(mentions) != 2 || len(ids) != 2 || ids[0] != "U999" || ids[1] != "U888" {
		t.Fatalf("slackExtractMentions() = (%v, %v), want two extracted mention IDs", mentions, ids)
	}
	if slackUserMentionPattern == nil {
		t.Fatal("expected slack mention regex to initialize on first mention extraction")
	}
}
