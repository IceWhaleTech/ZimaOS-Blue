// Package sse provides a Server-Sent Events broker for pushing real-time
// events to connected clients, scoped by user ID.
package sse

import (
	"encoding/json"
	"sync"
)

// Event represents a server-sent event.
type Event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// MarshalData returns the JSON-encoded data field.
func (e Event) MarshalData() []byte {
	b, _ := json.Marshal(e.Data)
	return b
}

// Broker manages per-user SSE client channels.
type Broker struct {
	mu      sync.RWMutex
	clients map[string]map[chan Event]struct{} // userID → set of channels
	done    chan struct{}
}

// NewBroker creates a new SSE broker.
func NewBroker() *Broker {
	return &Broker{
		clients: make(map[string]map[chan Event]struct{}),
		done:    make(chan struct{}),
	}
}

// Subscribe creates a new event channel for the given user.
// The caller must call Unsubscribe when done.
func (b *Broker) Subscribe(userID string) chan Event {
	ch := make(chan Event, 32) // buffered to avoid blocking publisher

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.clients[userID] == nil {
		b.clients[userID] = make(map[chan Event]struct{})
	}
	b.clients[userID][ch] = struct{}{}
	return ch
}

// Unsubscribe removes a client channel. Only closes the channel if it
// hasn't already been closed by Close().
func (b *Broker) Unsubscribe(userID string, ch chan Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	subs, ok := b.clients[userID]
	if !ok {
		return // already removed (e.g., by Close)
	}
	if _, exists := subs[ch]; !exists {
		return // already removed
	}
	delete(subs, ch)
	if len(subs) == 0 {
		delete(b.clients, userID)
	}
	close(ch)
}

// Publish sends an event to all subscribers of the given user.
// Non-blocking: if a client's buffer is full, the event is dropped.
func (b *Broker) Publish(userID string, eventType string, data any) {
	evt := Event{Type: eventType, Data: data}

	b.mu.RLock()
	defer b.mu.RUnlock()

	subs, ok := b.clients[userID]
	if !ok {
		return
	}
	for ch := range subs {
		select {
		case ch <- evt:
		default:
			// Drop event if client is slow
		}
	}
}

// ClientCount returns the number of connected clients for a user.
func (b *Broker) ClientCount(userID string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients[userID])
}

// Done returns a channel that is closed when the broker is shut down.
func (b *Broker) Done() <-chan struct{} {
	return b.done
}

// Close signals all SSE handlers to exit and closes all client channels.
func (b *Broker) Close() {
	close(b.done)

	b.mu.Lock()
	defer b.mu.Unlock()

	for userID, subs := range b.clients {
		for ch := range subs {
			close(ch)
		}
		delete(b.clients, userID)
	}
}
