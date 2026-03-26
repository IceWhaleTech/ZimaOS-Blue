package agentsessions

import "sync"

type runtimeStreamImpl struct {
	events chan RuntimeEvent
	done   chan error
	once   sync.Once
}

func newRuntimeStream(buffer int) *runtimeStreamImpl {
	if buffer <= 0 {
		buffer = 32
	}
	return &runtimeStreamImpl{
		events: make(chan RuntimeEvent, buffer),
		done:   make(chan error, 1),
	}
}

func (s *runtimeStreamImpl) Events() <-chan RuntimeEvent {
	return s.events
}

func (s *runtimeStreamImpl) Wait() error {
	err, ok := <-s.done
	if !ok {
		return nil
	}
	return err
}

func (s *runtimeStreamImpl) push(event RuntimeEvent) {
	s.events <- event
}

func (s *runtimeStreamImpl) finish(err error) {
	s.once.Do(func() {
		close(s.events)
		s.done <- err
		close(s.done)
	})
}

