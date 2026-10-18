// Package events provides a small in-process event bus for lifecycle and status
// notifications. Subscribers are bounded; slow consumers may miss events rather
// than block engine lifecycle operations.
package events

import (
	"errors"
	"sync"
	"time"
)

type Event struct {
	Type      string    `json:"type"`
	EngineID  string    `json:"engine_id,omitempty"`
	Profile   string    `json:"profile,omitempty"`
	Status    string    `json:"status,omitempty"`
	ErrorCode string    `json:"error_code,omitempty"`
	At        time.Time `json:"at"`
}

type Bus struct {
	mu     sync.RWMutex
	nextID uint64
	subs   map[uint64]chan Event
	closed bool
}

func NewBus() *Bus {
	return &Bus{subs: make(map[uint64]chan Event)}
}

// Subscribe returns a receive-only channel and an idempotent unsubscribe function.
func (b *Bus) Subscribe(buffer int) (<-chan Event, func(), error) {
	if b == nil {
		return nil, nil, errors.New("event bus is nil")
	}
	if buffer < 1 {
		return nil, nil, errors.New("subscriber buffer must be positive")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, nil, errors.New("event bus is closed")
	}
	b.nextID++
	id := b.nextID
	ch := make(chan Event, buffer)
	b.subs[id] = ch
	var once sync.Once
	return ch, func() {
		once.Do(func() { b.remove(id) })
	}, nil
}

func (b *Bus) remove(id uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if ch, ok := b.subs[id]; ok {
		delete(b.subs, id)
		close(ch)
	}
}

// Publish never blocks on a slow subscriber. It returns the number of subscribers
// whose buffers were full and therefore did not receive this event.
func (b *Bus) Publish(event Event) int {
	if b == nil {
		return 0
	}
	if event.At.IsZero() {
		event.At = time.Now().UTC()
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return 0
	}
	dropped := 0
	for _, ch := range b.subs {
		select {
		case ch <- event:
		default:
			dropped++
		}
	}
	return dropped
}

func (b *Bus) Close() {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for id, ch := range b.subs {
		delete(b.subs, id)
		close(ch)
	}
}
