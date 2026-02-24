package tools

import (
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ProcessStatus represents the state of a process session.
type ProcessStatus string

const (
	ProcessRunning   ProcessStatus = "running"
	ProcessCompleted ProcessStatus = "completed"
	ProcessFailed    ProcessStatus = "failed"
	ProcessKilled    ProcessStatus = "killed"
)

const (
	defaultSessionTTL     = 30 * time.Minute
	defaultMaxOutputChars = 200_000
	sweeperInterval       = 5 * time.Minute
)

// OutputBuffer is a capped ring buffer for process output.
type OutputBuffer struct {
	mu        sync.Mutex
	chunks    []string
	totalLen  int
	cap       int
	truncated bool
}

// NewOutputBuffer creates a new output buffer with the given capacity.
func NewOutputBuffer(cap int) *OutputBuffer {
	if cap <= 0 {
		cap = defaultMaxOutputChars
	}
	return &OutputBuffer{cap: cap}
}

// Append adds a chunk to the buffer, evicting old data if capacity is exceeded.
func (b *OutputBuffer) Append(chunk string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.chunks = append(b.chunks, chunk)
	b.totalLen += len(chunk)

	for b.totalLen > b.cap && len(b.chunks) > 1 {
		b.totalLen -= len(b.chunks[0])
		b.chunks = b.chunks[1:]
		b.truncated = true
	}

	// Single chunk exceeds cap — trim from front.
	if b.totalLen > b.cap && len(b.chunks) == 1 {
		excess := b.totalLen - b.cap
		b.chunks[0] = b.chunks[0][excess:]
		b.totalLen = b.cap
		b.truncated = true
	}
}

// Drain returns all buffered content and resets the buffer.
func (b *OutputBuffer) Drain() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	s := strings.Join(b.chunks, "")
	b.chunks = b.chunks[:0]
	b.totalLen = 0
	return s
}

// String returns all buffered content without draining.
func (b *OutputBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.Join(b.chunks, "")
}

// Tail returns the last n characters of the buffer.
func (b *OutputBuffer) Tail(n int) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := strings.Join(b.chunks, "")
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// Len returns the current buffer length.
func (b *OutputBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.totalLen
}

// Truncated returns whether the buffer has been truncated.
func (b *OutputBuffer) Truncated() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.truncated
}

// ProcessSession represents a running or recently finished process.
type ProcessSession struct {
	ID           string
	Command      string
	Workdir      string
	PID          int
	StartedAt    time.Time
	Stdout       *OutputBuffer
	Stderr       *OutputBuffer
	ExitCode     *int
	ExitSignal   string
	Status       ProcessStatus
	Backgrounded bool
}

// FinishedSession is a snapshot of a completed session.
type FinishedSession struct {
	ID         string
	Command    string
	Workdir    string
	StartedAt  time.Time
	EndedAt    time.Time
	Status     ProcessStatus
	ExitCode   *int
	ExitSignal string
	Output     string // aggregated stdout+stderr
	Truncated  bool
}

// SessionRegistry manages running and finished process sessions.
type SessionRegistry struct {
	mu       sync.RWMutex
	running  map[string]*ProcessSession
	finished map[string]*FinishedSession
	ttl      time.Duration
	stopCh   chan struct{}
}

// NewSessionRegistry creates a new session registry with a background sweeper.
func NewSessionRegistry() *SessionRegistry {
	r := &SessionRegistry{
		running:  make(map[string]*ProcessSession),
		finished: make(map[string]*FinishedSession),
		ttl:      defaultSessionTTL,
		stopCh:   make(chan struct{}),
	}
	go r.sweeper()
	return r
}

// NewSessionID generates a unique session ID.
func NewSessionID() string {
	return uuid.New().String()[:8]
}

// Add registers a new running session.
func (r *SessionRegistry) Add(s *ProcessSession) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.running[s.ID] = s
}

// Get retrieves a running session by ID.
func (r *SessionRegistry) Get(id string) *ProcessSession {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.running[id]
}

// GetFinished retrieves a finished session by ID.
func (r *SessionRegistry) GetFinished(id string) *FinishedSession {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.finished[id]
}

// Delete removes a session from both running and finished maps.
func (r *SessionRegistry) Delete(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.running, id)
	delete(r.finished, id)
}

// MarkExited transitions a running session to finished.
func (r *SessionRegistry) MarkExited(id string, exitCode *int, exitSignal string, status ProcessStatus) {
	r.mu.Lock()
	defer r.mu.Unlock()

	s, ok := r.running[id]
	if !ok {
		return
	}

	s.ExitCode = exitCode
	s.ExitSignal = exitSignal
	s.Status = status

	// Move to finished.
	delete(r.running, id)
	r.finished[id] = &FinishedSession{
		ID:         s.ID,
		Command:    s.Command,
		Workdir:    s.Workdir,
		StartedAt:  s.StartedAt,
		EndedAt:    time.Now(),
		Status:     status,
		ExitCode:   exitCode,
		ExitSignal: exitSignal,
		Output:     s.Stdout.String() + s.Stderr.String(),
		Truncated:  s.Stdout.Truncated() || s.Stderr.Truncated(),
	}
}

// MarkBackgrounded marks a session as backgrounded.
func (r *SessionRegistry) MarkBackgrounded(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.running[id]; ok {
		s.Backgrounded = true
	}
}

// ListRunning returns all running sessions.
func (r *SessionRegistry) ListRunning() []*ProcessSession {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*ProcessSession, 0, len(r.running))
	for _, s := range r.running {
		result = append(result, s)
	}
	return result
}

// ListFinished returns all finished sessions.
func (r *SessionRegistry) ListFinished() []*FinishedSession {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*FinishedSession, 0, len(r.finished))
	for _, s := range r.finished {
		result = append(result, s)
	}
	return result
}

// Cleanup stops the sweeper and clears all sessions.
func (r *SessionRegistry) Cleanup() {
	close(r.stopCh)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.running = make(map[string]*ProcessSession)
	r.finished = make(map[string]*FinishedSession)
}

func (r *SessionRegistry) sweeper() {
	ticker := time.NewTicker(sweeperInterval)
	defer ticker.Stop()
	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.pruneFinished()
		}
	}
}

func (r *SessionRegistry) pruneFinished() {
	cutoff := time.Now().Add(-r.ttl)
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, s := range r.finished {
		if s.EndedAt.Before(cutoff) {
			delete(r.finished, id)
		}
	}
}
