package tools

import "time"

func init() {
	// Keep accessibility tests deterministic without paying production-scale waits.
	a11yMessageConversationSettleDelay = 0
	a11yMessageConversationConfirmationTimeout = 100 * time.Millisecond
	a11yMessageConversationConfirmationPollInterval = time.Millisecond
	a11ySubmitConfirmationTimeout = 100 * time.Millisecond
	a11ySubmitConfirmationPollInterval = time.Millisecond
	a11yChatComposerConfirmationTimeout = 100 * time.Millisecond
	a11yChatComposerConfirmationPollInterval = time.Millisecond
	a11yChatVerifyOutcomeTimeout = 100 * time.Millisecond
	a11yChatVerifyOutcomePollInterval = time.Millisecond
}
