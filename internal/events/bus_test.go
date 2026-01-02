package events

import (
	"testing"
	"time"

	"github.com/shankarg87/agent/internal/store"
)

func TestEventBus_Subscribe(t *testing.T) {
	bus := NewEventBus()

	// Test subscription
	ch := bus.Subscribe("run-1")
	assertNotNil(t, ch)

	// Check that subscriber is registered
	bus.mu.RLock()
	assertEqual(t, 1, len(bus.subscribers["run-1"]))
	bus.mu.RUnlock()

	// Test multiple subscriptions to same run
	ch2 := bus.Subscribe("run-1")
	assertNotNil(t, ch2)

	bus.mu.RLock()
	assertEqual(t, 2, len(bus.subscribers["run-1"]))
	bus.mu.RUnlock()

	// Test subscription to different run
	ch3 := bus.Subscribe("run-2")
	assertNotNil(t, ch3)

	bus.mu.RLock()
	assertEqual(t, 2, len(bus.subscribers["run-1"]))
	assertEqual(t, 1, len(bus.subscribers["run-2"]))
	bus.mu.RUnlock()
}

func TestEventBus_Publish(t *testing.T) {
	bus := NewEventBus()

	// Subscribe to run-1
	ch1 := bus.Subscribe("run-1")
	ch2 := bus.Subscribe("run-1")

	// Subscribe to run-2
	ch3 := bus.Subscribe("run-2")

	// Create test events
	event1 := &store.Event{
		ID:        "event-1",
		RunID:     "run-1",
		Type:      store.EventTypeRunStarted,
		Data:      map[string]any{"test": true},
		Timestamp: time.Now(),
	}

	event2 := &store.Event{
		ID:        "event-2",
		RunID:     "run-2",
		Type:      store.EventTypeTextDelta,
		Data:      map[string]any{"text": "hello"},
		Timestamp: time.Now(),
	}

	// Publish to run-1
	bus.Publish("run-1", event1)

	// Check that run-1 subscribers received the event
	select {
	case received := <-ch1:
		assertEqual(t, "event-1", received.ID)
		assertEqual(t, "run-1", received.RunID)
		assertEqual(t, store.EventTypeRunStarted, received.Type)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected event on ch1")
	}

	select {
	case received := <-ch2:
		assertEqual(t, "event-1", received.ID)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected event on ch2")
	}

	// Check that run-2 subscriber did not receive run-1 event
	select {
	case <-ch3:
		t.Fatal("ch3 should not have received run-1 event")
	case <-time.After(50 * time.Millisecond):
		// Expected - no event received
	}

	// Publish to run-2
	bus.Publish("run-2", event2)

	// Check that run-2 subscriber received the event
	select {
	case received := <-ch3:
		assertEqual(t, "event-2", received.ID)
		assertEqual(t, "run-2", received.RunID)
		assertEqual(t, store.EventTypeTextDelta, received.Type)
		assertEqual(t, "hello", received.Data["text"])
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected event on ch3")
	}
}

func TestEventBus_Unsubscribe(t *testing.T) {
	bus := NewEventBus()

	// Subscribe
	ch1 := bus.Subscribe("run-1")
	ch2 := bus.Subscribe("run-1")

	// Verify both subscriptions exist
	bus.mu.RLock()
	assertEqual(t, 2, len(bus.subscribers["run-1"]))
	bus.mu.RUnlock()

	// Unsubscribe one channel
	bus.Unsubscribe("run-1", ch1)

	// Verify one subscription remains
	bus.mu.RLock()
	assertEqual(t, 1, len(bus.subscribers["run-1"]))
	bus.mu.RUnlock()

	// Test that the remaining channel still works
	event := &store.Event{
		ID:        "event-1",
		RunID:     "run-1",
		Type:      store.EventTypeRunStarted,
		Data:      map[string]any{},
		Timestamp: time.Now(),
	}

	bus.Publish("run-1", event)

	select {
	case received := <-ch2:
		assertEqual(t, "event-1", received.ID)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected event on ch2")
	}

	// Unsubscribe the last channel
	bus.Unsubscribe("run-1", ch2)

	// Verify run-1 is removed from subscribers map
	bus.mu.RLock()
	_, exists := bus.subscribers["run-1"]
	assertEqual(t, false, exists)
	bus.mu.RUnlock()
}

func TestEventBus_CloseAll(t *testing.T) {
	bus := NewEventBus()

	// Subscribe multiple channels
	ch1 := bus.Subscribe("run-1")
	ch2 := bus.Subscribe("run-1")
	ch3 := bus.Subscribe("run-2")

	// Verify subscriptions
	bus.mu.RLock()
	assertEqual(t, 2, len(bus.subscribers["run-1"]))
	assertEqual(t, 1, len(bus.subscribers["run-2"]))
	bus.mu.RUnlock()

	// Close all subscriptions for run-1
	bus.CloseAll("run-1")

	// Verify run-1 is removed but run-2 remains
	bus.mu.RLock()
	_, exists := bus.subscribers["run-1"]
	assertEqual(t, false, exists)
	assertEqual(t, 1, len(bus.subscribers["run-2"]))
	bus.mu.RUnlock()

	// Verify channels are closed
	_, ok1 := <-ch1
	assertEqual(t, false, ok1)

	_, ok2 := <-ch2
	assertEqual(t, false, ok2)

	// Verify run-2 channel is still open
	event := &store.Event{
		ID:        "event-1",
		RunID:     "run-2",
		Type:      store.EventTypeRunStarted,
		Data:      map[string]any{},
		Timestamp: time.Now(),
	}

	bus.Publish("run-2", event)

	select {
	case received := <-ch3:
		assertEqual(t, "event-1", received.ID)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected event on ch3")
	}
}

func TestEventBus_ChannelBuffer(t *testing.T) {
	bus := NewEventBus()
	ch := bus.Subscribe("run-1")

	// Send multiple events without reading from channel
	for i := 0; i < 10; i++ {
		event := &store.Event{
			ID:        string(rune('0' + i)),
			RunID:     "run-1",
			Type:      store.EventTypeTextDelta,
			Data:      map[string]any{"i": i},
			Timestamp: time.Now(),
		}
		bus.Publish("run-1", event)
	}

	// Read all events
	eventsReceived := 0
	for i := 0; i < 10; i++ {
		select {
		case event := <-ch:
			assertNotNil(t, event)
			eventsReceived++
		case <-time.After(100 * time.Millisecond):
			break
		}
	}

	// Should receive all 10 events due to buffered channel
	assertEqual(t, 10, eventsReceived)
}

func TestEventBus_PublishToNonExistentRun(t *testing.T) {
	bus := NewEventBus()

	// Publishing to non-existent run should not panic
	event := &store.Event{
		ID:        "event-1",
		RunID:     "non-existent",
		Type:      store.EventTypeRunStarted,
		Data:      map[string]any{},
		Timestamp: time.Now(),
	}

	// This should not panic
	bus.Publish("non-existent", event)

	// Verify no subscribers were created
	bus.mu.RLock()
	assertEqual(t, 0, len(bus.subscribers))
	bus.mu.RUnlock()
}

func TestEventBus_UnsubscribeNonExistentChannel(t *testing.T) {
	bus := NewEventBus()

	// Create a channel but don't subscribe it
	ch := make(chan *store.Event)

	// Unsubscribing non-existent channel should not panic
	bus.Unsubscribe("run-1", ch)

	// Verify no subscribers
	bus.mu.RLock()
	assertEqual(t, 0, len(bus.subscribers))
	bus.mu.RUnlock()
}

// Test helpers
func assertNotNil(t *testing.T, value any) {
	t.Helper()
	if value == nil {
		t.Fatal("Expected non-nil value, got nil")
	}
}

func assertEqual[T comparable](t *testing.T, expected, actual T) {
	t.Helper()
	if expected != actual {
		t.Fatalf("Expected %v, got %v", expected, actual)
	}
}
