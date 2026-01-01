package events

import (
	"sync"

	"github.com/shankgan/agent/internal/store"
)

// EventBus manages event subscriptions and publishing
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[string][]chan *store.Event // runID -> channels
}

// NewEventBus creates a new event bus
func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[string][]chan *store.Event),
	}
}

// Subscribe creates a new subscription for events from a specific run
func (b *EventBus) Subscribe(runID string) <-chan *store.Event {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan *store.Event, 100)
	b.subscribers[runID] = append(b.subscribers[runID], ch)

	return ch
}

// Unsubscribe removes a subscription
func (b *EventBus) Unsubscribe(runID string, ch <-chan *store.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	subs := b.subscribers[runID]
	for i, sub := range subs {
		if sub == ch {
			// Close and remove the channel
			close(sub)
			b.subscribers[runID] = append(subs[:i], subs[i+1:]...)
			break
		}
	}

	// Clean up empty subscriber lists
	if len(b.subscribers[runID]) == 0 {
		delete(b.subscribers, runID)
	}
}

// Publish sends an event to all subscribers of a run
func (b *EventBus) Publish(runID string, event *store.Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, ch := range b.subscribers[runID] {
		select {
		case ch <- event:
		default:
			// Channel full, skip to prevent blocking
		}
	}
}

// CloseAll closes all subscriptions for a run
func (b *EventBus) CloseAll(runID string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, ch := range b.subscribers[runID] {
		close(ch)
	}

	delete(b.subscribers, runID)
}
