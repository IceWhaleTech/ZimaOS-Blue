package companion

import (
	"context"
	"sync"
	"sync/atomic"
)

// EventStreamer handles real-time event streaming to subscribers.
type EventStreamer struct {
	config *Config
	mu     sync.RWMutex

	// Subscribers
	subscribers     map[uint64]*subscriber
	subscriberCount uint64

	// Event buffer for batch processing
	eventBuffer chan *SessionEvent

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

type subscriber struct {
	id        uint64
	sessionID string // empty means subscribe to all
	ch        chan *SessionEvent
	closed    atomic.Bool
}

// NewEventStreamer creates a new event streamer.
func NewEventStreamer(config *Config) *EventStreamer {
	return &EventStreamer{
		config:      config,
		subscribers: make(map[uint64]*subscriber),
		eventBuffer: make(chan *SessionEvent, config.Performance.EventBufferSize),
	}
}

// Start starts the event streamer.
func (s *EventStreamer) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)

	// Start event dispatcher
	s.wg.Add(1)
	go s.dispatchLoop()

	return nil
}

// Stop stops the event streamer.
func (s *EventStreamer) Stop() error {
	if s.cancel != nil {
		s.cancel()
	}

	// Close all subscriber channels
	s.mu.Lock()
	for _, sub := range s.subscribers {
		s.closeSubscriber(sub)
	}
	s.subscribers = make(map[uint64]*subscriber)
	s.mu.Unlock()

	// Wait for dispatcher to finish
	s.wg.Wait()

	return nil
}

// Emit emits an event to all subscribers.
func (s *EventStreamer) Emit(event *SessionEvent) {
	select {
	case s.eventBuffer <- event:
	default:
		// Buffer full, drop oldest event
		select {
		case <-s.eventBuffer:
		default:
		}
		s.eventBuffer <- event
	}
}

// Subscribe subscribes to events.
// If sessionID is empty, subscribes to all events.
// Returns a channel for receiving events and a cleanup function.
func (s *EventStreamer) Subscribe(sessionID string) (<-chan *SessionEvent, func()) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := atomic.AddUint64(&s.subscriberCount, 1)
	sub := &subscriber{
		id:        id,
		sessionID: sessionID,
		ch:        make(chan *SessionEvent, 100),
	}

	s.subscribers[id] = sub

	cleanup := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if sub, ok := s.subscribers[id]; ok {
			s.closeSubscriber(sub)
			delete(s.subscribers, id)
		}
	}

	return sub.ch, cleanup
}

// SubscriberCount returns the number of active subscribers.
func (s *EventStreamer) SubscriberCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.subscribers)
}

// dispatchLoop dispatches events to subscribers.
func (s *EventStreamer) dispatchLoop() {
	defer s.wg.Done()

	for {
		select {
		case <-s.ctx.Done():
			return
		case event := <-s.eventBuffer:
			s.dispatch(event)
		}
	}
}

// dispatch sends an event to all matching subscribers.
func (s *EventStreamer) dispatch(event *SessionEvent) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, sub := range s.subscribers {
		if sub.closed.Load() {
			continue
		}

		// Check if subscriber is interested in this event
		if sub.sessionID != "" && sub.sessionID != event.SessionID {
			continue
		}

		// Non-blocking send
		select {
		case sub.ch <- event:
		default:
			// Subscriber channel full, skip
		}
	}
}

// closeSubscriber closes a subscriber's channel.
func (s *EventStreamer) closeSubscriber(sub *subscriber) {
	if sub.closed.CompareAndSwap(false, true) {
		close(sub.ch)
	}
}
