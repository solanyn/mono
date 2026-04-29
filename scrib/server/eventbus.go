package server

import (
	"sync"
	"time"
)

// Event is a single progress marker in a meeting's processing lifecycle.
// Stage names: started, processing, matching, done, error.
type Event struct {
	Stage  string    `json:"stage"`
	At     time.Time `json:"at"`
	Detail string    `json:"detail,omitempty"`
}

// eventBus is a single meeting's pub-sub. One publisher (the processor
// goroutine); zero-or-more subscribers (SSE clients that reconnect).
type eventBus struct {
	mu          sync.Mutex
	last        *Event
	history     []Event // small ring — SSE clients replay on (re)connect
	subscribers []chan Event
	closed      bool
}

const eventHistoryCap = 32

func newEventBus() *eventBus {
	return &eventBus{history: make([]Event, 0, eventHistoryCap)}
}

func (b *eventBus) publish(e Event) {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.last = &e
	if len(b.history) >= eventHistoryCap {
		b.history = append(b.history[:0], b.history[1:]...)
	}
	b.history = append(b.history, e)
	subs := append([]chan Event(nil), b.subscribers...)
	b.mu.Unlock()

	// Fire-and-forget; slow subscribers get their stale events dropped.
	for _, ch := range subs {
		select {
		case ch <- e:
		default:
		}
	}
}

// subscribe returns a buffered channel pre-primed with history, plus an
// unsubscribe function the caller must defer.
func (b *eventBus) subscribe() (<-chan Event, func()) {
	ch := make(chan Event, eventHistoryCap)
	b.mu.Lock()
	for _, e := range b.history {
		ch <- e
	}
	b.subscribers = append(b.subscribers, ch)
	closed := b.closed
	b.mu.Unlock()
	if closed {
		close(ch)
	}
	return ch, func() {
		b.mu.Lock()
		for i, s := range b.subscribers {
			if s == ch {
				b.subscribers = append(b.subscribers[:i], b.subscribers[i+1:]...)
				break
			}
		}
		b.mu.Unlock()
	}
}

func (b *eventBus) close() {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	subs := b.subscribers
	b.subscribers = nil
	b.mu.Unlock()
	for _, ch := range subs {
		close(ch)
	}
}

// bus returns (or lazily creates) the eventBus for meetingUUID.
func (s *Server) bus(meetingUUID string) *eventBus {
	s.busMu.Lock()
	defer s.busMu.Unlock()
	if s.buses == nil {
		s.buses = map[string]*eventBus{}
	}
	if b, ok := s.buses[meetingUUID]; ok {
		return b
	}
	b := newEventBus()
	s.buses[meetingUUID] = b
	return b
}

// lookupBus returns the bus if one exists — used by SSE handlers that
// want to report 404 for unknown meetings instead of creating empty buses.
func (s *Server) lookupBus(meetingUUID string) (*eventBus, bool) {
	s.busMu.Lock()
	defer s.busMu.Unlock()
	b, ok := s.buses[meetingUUID]
	return b, ok
}

// closeBus removes the bus after a short retention window so late SSE
// clients can still catch the final "done" event.
func (s *Server) closeBus(meetingUUID string) {
	s.busMu.Lock()
	b, ok := s.buses[meetingUUID]
	s.busMu.Unlock()
	if !ok {
		return
	}
	time.AfterFunc(5*time.Minute, func() {
		s.busMu.Lock()
		delete(s.buses, meetingUUID)
		s.busMu.Unlock()
		b.close()
	})
}
