package store

import (
	"context"
	"testing"
	"time"
)

func TestInMemoryStore_Sessions(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Test CreateSession
	session := &Session{
		ID:          "test-session-1",
		TenantID:    "tenant-1",
		ProfileName: "test-profile",
		Metadata:    map[string]any{"key": "value"},
	}

	err := store.CreateSession(ctx, session)
	assertNoError(t, err)
	assertNotNil(t, session.CreatedAt)
	assertNotNil(t, session.UpdatedAt)

	// Test GetSession
	retrieved, err := store.GetSession(ctx, "test-session-1")
	assertNoError(t, err)
	assertEqual(t, "test-session-1", retrieved.ID)
	assertEqual(t, "tenant-1", retrieved.TenantID)
	assertEqual(t, "test-profile", retrieved.ProfileName)
	assertEqual(t, "value", retrieved.Metadata["key"])

	// Test GetSession with non-existent ID
	_, err = store.GetSession(ctx, "non-existent")
	assertError(t, err)
	assertEqual(t, ErrNotFound, err)

	// Test CreateSession with auto-generated ID
	session2 := &Session{
		TenantID:    "tenant-1",
		ProfileName: "test-profile-2",
	}
	err = store.CreateSession(ctx, session2)
	assertNoError(t, err)
	assertNotEqual(t, "", session2.ID)

	// Test ListSessions
	session3 := &Session{
		ID:          "test-session-3",
		TenantID:    "tenant-1",
		ProfileName: "test-profile-3",
	}
	err = store.CreateSession(ctx, session3)
	assertNoError(t, err)

	sessions, err := store.ListSessions(ctx, "tenant-1", 10, 0)
	assertNoError(t, err)
	assertEqual(t, 3, len(sessions))

	// Test ListSessions with different tenant
	sessions, err = store.ListSessions(ctx, "tenant-2", 10, 0)
	assertNoError(t, err)
	assertEqual(t, 0, len(sessions))

	// Test ListSessions with limit and offset
	sessions, err = store.ListSessions(ctx, "tenant-1", 1, 1)
	assertNoError(t, err)
	assertEqual(t, 1, len(sessions))
}

func TestInMemoryStore_Runs(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Create a session first
	session := &Session{
		ID:          "test-session",
		TenantID:    "tenant-1",
		ProfileName: "test-profile",
	}
	err := store.CreateSession(ctx, session)
	assertNoError(t, err)

	// Test CreateRun
	run := &Run{
		ID:        "test-run-1",
		SessionID: "test-session",
		TenantID:  "tenant-1",
		Mode:      "interactive",
		Status:    RunStateQueued,
		Metadata:  map[string]any{"test": true},
	}

	err = store.CreateRun(ctx, run)
	assertNoError(t, err)
	assertNotNil(t, run.CreatedAt)
	assertNotNil(t, run.UpdatedAt)

	// Test GetRun
	retrieved, err := store.GetRun(ctx, "test-run-1")
	assertNoError(t, err)
	assertEqual(t, "test-run-1", retrieved.ID)
	assertEqual(t, "test-session", retrieved.SessionID)
	assertEqual(t, "tenant-1", retrieved.TenantID)
	assertEqual(t, "interactive", retrieved.Mode)
	assertEqual(t, RunStateQueued, retrieved.Status)

	// Test UpdateRun
	retrieved.Status = RunStateRunning
	retrieved.StartedAt = &time.Time{}
	*retrieved.StartedAt = time.Now()

	err = store.UpdateRun(ctx, retrieved)
	assertNoError(t, err)

	updated, err := store.GetRun(ctx, "test-run-1")
	assertNoError(t, err)
	assertEqual(t, RunStateRunning, updated.Status)
	assertNotNil(t, updated.StartedAt)

	// Test GetRun with non-existent ID
	_, err = store.GetRun(ctx, "non-existent")
	assertError(t, err)
	assertEqual(t, ErrNotFound, err)

	// Test ListRuns
	run2 := &Run{
		ID:        "test-run-2",
		SessionID: "test-session",
		TenantID:  "tenant-1",
		Mode:      "autonomous",
		Status:    RunStateCompleted,
	}
	err = store.CreateRun(ctx, run2)
	assertNoError(t, err)

	runs, err := store.ListRuns(ctx, "test-session")
	assertNoError(t, err)
	assertEqual(t, 2, len(runs))
}

func TestInMemoryStore_Messages(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Test AddMessage
	message1 := &Message{
		ID:        "msg-1",
		SessionID: "session-1",
		Role:      "user",
		Content:   "Hello",
		Metadata:  map[string]any{"timestamp": time.Now().Unix()},
	}

	err := store.AddMessage(ctx, "session-1", message1)
	assertNoError(t, err)
	assertNotNil(t, message1.CreatedAt)

	message2 := &Message{
		ID:        "msg-2",
		SessionID: "session-1",
		Role:      "assistant",
		Content:   "Hi there!",
	}

	err = store.AddMessage(ctx, "session-1", message2)
	assertNoError(t, err)

	// Test GetMessages
	messages, err := store.GetMessages(ctx, "session-1")
	assertNoError(t, err)
	assertEqual(t, 2, len(messages))
	assertEqual(t, "msg-1", messages[0].ID)
	assertEqual(t, "user", messages[0].Role)
	assertEqual(t, "Hello", messages[0].Content)
	assertEqual(t, "msg-2", messages[1].ID)
	assertEqual(t, "assistant", messages[1].Role)
	assertEqual(t, "Hi there!", messages[1].Content)

	// Test GetMessages for non-existent session
	messages, err = store.GetMessages(ctx, "non-existent")
	assertNoError(t, err)
	assertEqual(t, 0, len(messages))
}

func TestInMemoryStore_Events(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Test AddEvent
	event1 := &Event{
		ID:    "event-1",
		RunID: "run-1",
		Type:  EventTypeRunStarted,
		Data:  map[string]any{"run_id": "run-1"},
	}

	err := store.AddEvent(ctx, "run-1", event1)
	assertNoError(t, err)
	assertNotNil(t, event1.Timestamp)

	event2 := &Event{
		ID:    "event-2",
		RunID: "run-1",
		Type:  EventTypeTextDelta,
		Data:  map[string]any{"text": "Hello"},
	}

	err = store.AddEvent(ctx, "run-1", event2)
	assertNoError(t, err)

	// Test GetEvents
	events, err := store.GetEvents(ctx, "run-1")
	assertNoError(t, err)
	assertEqual(t, 2, len(events))
	assertEqual(t, "event-1", events[0].ID)
	assertEqual(t, EventTypeRunStarted, events[0].Type)
	assertEqual(t, "run-1", events[0].Data["run_id"])
	assertEqual(t, "event-2", events[1].ID)
	assertEqual(t, EventTypeTextDelta, events[1].Type)
	assertEqual(t, "Hello", events[1].Data["text"])

	// Test GetEvents for non-existent run
	events, err = store.GetEvents(ctx, "non-existent")
	assertNoError(t, err)
	assertEqual(t, 0, len(events))
}

func TestInMemoryStore_ToolCalls(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Test AddToolCall
	toolCall := &ToolCall{
		ID:         "tool-1",
		RunID:      "run-1",
		ToolName:   "echo",
		ServerName: "echo-server",
		Arguments:  map[string]any{"message": "hello"},
		Status:     ToolCallStatusPending,
	}

	err := store.AddToolCall(ctx, "run-1", toolCall)
	assertNoError(t, err)
	assertNotNil(t, toolCall.CreatedAt)

	// Test UpdateToolCall
	toolCall.Status = ToolCallStatusCompleted
	toolCall.Output = "hello"
	now := time.Now()
	toolCall.CompletedAt = &now

	err = store.UpdateToolCall(ctx, toolCall)
	assertNoError(t, err)

	// Test GetToolCalls
	toolCalls, err := store.GetToolCalls(ctx, "run-1")
	assertNoError(t, err)
	assertEqual(t, 1, len(toolCalls))

	retrieved := toolCalls[0]
	assertEqual(t, "tool-1", retrieved.ID)
	assertEqual(t, "run-1", retrieved.RunID)
	assertEqual(t, "echo", retrieved.ToolName)
	assertEqual(t, "echo-server", retrieved.ServerName)
	assertEqual(t, "hello", retrieved.Arguments["message"])
	assertEqual(t, ToolCallStatusCompleted, retrieved.Status)
	assertEqual(t, "hello", retrieved.Output)
	assertNotNil(t, retrieved.CompletedAt)

	// Add another tool call
	toolCall2 := &ToolCall{
		ID:         "tool-2",
		RunID:      "run-1",
		ToolName:   "uppercase",
		ServerName: "echo-server",
		Arguments:  map[string]any{"text": "world"},
		Status:     ToolCallStatusFailed,
		Error:      "some error",
	}

	err = store.AddToolCall(ctx, "run-1", toolCall2)
	assertNoError(t, err)

	toolCalls, err = store.GetToolCalls(ctx, "run-1")
	assertNoError(t, err)
	assertEqual(t, 2, len(toolCalls))

	// Test GetToolCalls for non-existent run
	toolCalls, err = store.GetToolCalls(ctx, "non-existent")
	assertNoError(t, err)
	assertEqual(t, 0, len(toolCalls))
}

func TestInMemoryStore_AutoGeneratedIDs(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Test auto-generated session ID
	session := &Session{
		TenantID:    "tenant-1",
		ProfileName: "test",
	}
	err := store.CreateSession(ctx, session)
	assertNoError(t, err)
	assertNotEqual(t, "", session.ID)

	// Test auto-generated run ID
	run := &Run{
		SessionID: session.ID,
		TenantID:  "tenant-1",
		Mode:      "interactive",
		Status:    RunStateQueued,
	}
	err = store.CreateRun(ctx, run)
	assertNoError(t, err)
	assertNotEqual(t, "", run.ID)

	// Test auto-generated message ID
	message := &Message{
		SessionID: session.ID,
		Role:      "user",
		Content:   "test message",
	}
	err = store.AddMessage(ctx, session.ID, message)
	assertNoError(t, err)
	assertNotEqual(t, "", message.ID)

	// Test auto-generated event ID
	event := &Event{
		RunID: run.ID,
		Type:  EventTypeRunStarted,
		Data:  map[string]any{},
	}
	err = store.AddEvent(ctx, run.ID, event)
	assertNoError(t, err)
	assertNotEqual(t, "", event.ID)

	// Test auto-generated tool call ID
	toolCall := &ToolCall{
		RunID:      run.ID,
		ToolName:   "test-tool",
		ServerName: "test-server",
		Arguments:  map[string]any{},
		Status:     ToolCallStatusPending,
	}
	err = store.AddToolCall(ctx, run.ID, toolCall)
	assertNoError(t, err)
	assertNotEqual(t, "", toolCall.ID)
}

// Test helpers
func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

func assertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("Expected an error, got nil")
	}
}

func assertEqual[T comparable](t *testing.T, expected, actual T) {
	t.Helper()
	if expected != actual {
		t.Fatalf("Expected %v, got %v", expected, actual)
	}
}

func assertNotEqual[T comparable](t *testing.T, notExpected, actual T) {
	t.Helper()
	if notExpected == actual {
		t.Fatalf("Expected not %v, got %v", notExpected, actual)
	}
}

func assertNotNil(t *testing.T, value any) {
	t.Helper()
	if value == nil {
		t.Fatal("Expected non-nil value, got nil")
	}
}
