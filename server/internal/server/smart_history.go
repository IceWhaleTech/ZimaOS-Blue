package server

import (
	"context"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/memory"
)

// ConversationState tracks the state of a conversation for incremental fetching.
type ConversationState struct {
	lastMessageCount int
	messages         []memory.Message
	mu               sync.RWMutex
}

// SmartHistoryFetcher fetches only new messages since last request.
type SmartHistoryFetcher struct {
	states map[string]*ConversationState
	mu     sync.RWMutex
	store  *memory.Store
}

// NewSmartHistoryFetcher creates a new smart history fetcher.
func NewSmartHistoryFetcher(store *memory.Store) *SmartHistoryFetcher {
	return &SmartHistoryFetcher{
		states: make(map[string]*ConversationState),
		store:  store,
	}
}

// FetchMessages fetches messages intelligently - only new ones if possible.
func (f *SmartHistoryFetcher) FetchMessages(ctx context.Context, conversationID string, limit int) ([]memory.Message, error) {
	f.mu.RLock()
	state, exists := f.states[conversationID]
	f.mu.RUnlock()

	if !exists {
		// First fetch - get all messages
		messages, err := f.store.GetMessages(ctx, conversationID, limit, 0)
		if err != nil {
			return nil, err
		}

		// Create state
		f.mu.Lock()
		f.states[conversationID] = &ConversationState{
			lastMessageCount: len(messages),
			messages:         messages,
		}
		f.mu.Unlock()

		return messages, nil
	}

	// Subsequent fetch - check if there are new messages
	state.mu.RLock()
	cachedCount := state.lastMessageCount
	cachedMessages := state.messages
	state.mu.RUnlock()

	// Get current message count (lightweight query)
	currentMessages, err := f.store.GetMessages(ctx, conversationID, limit, 0)
	if err != nil {
		return nil, err
	}

	currentCount := len(currentMessages)

	// If count hasn't changed, return cached messages
	if currentCount == cachedCount {
		return cachedMessages, nil
	}

	// Count changed - update state
	state.mu.Lock()
	state.lastMessageCount = currentCount
	state.messages = currentMessages
	state.mu.Unlock()

	return currentMessages, nil
}

// Invalidate removes conversation state (call after new message).
func (f *SmartHistoryFetcher) Invalidate(conversationID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.states, conversationID)
}

// Clear removes all conversation states.
func (f *SmartHistoryFetcher) Clear() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.states = make(map[string]*ConversationState)
}
