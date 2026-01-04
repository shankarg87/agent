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

func TestInMemoryStore_DeleteSession(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Create a session with associated data
	session := &Session{
		ID:          "test-session-1",
		TenantID:    "tenant-1",
		ProfileName: "test-profile",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	err := store.CreateSession(ctx, session)
	assertNoError(t, err)

	// Create a run for the session
	run := &Run{
		ID:        "test-run-1",
		SessionID: "test-session-1",
		TenantID:  "tenant-1",
		Mode:      "interactive",
		Status:    RunStateRunning,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err = store.CreateRun(ctx, run)
	assertNoError(t, err)

	// Add a message to the session
	message := &Message{
		ID:        "test-message-1",
		SessionID: "test-session-1",
		Role:      "user",
		Content:   "Test message",
		CreatedAt: time.Now(),
	}
	err = store.AddMessage(ctx, "test-session-1", message)
	assertNoError(t, err)

	// Add an event to the run
	event := &Event{
		ID:        "test-event-1",
		RunID:     "test-run-1",
		Type:      EventTypeRunStarted,
		Data:      map[string]any{"test": "data"},
		Timestamp: time.Now(),
	}
	err = store.AddEvent(ctx, "test-run-1", event)
	assertNoError(t, err)

	// Add a tool call to the run
	toolCall := &ToolCall{
		ID:         "test-tool-call-1",
		RunID:      "test-run-1",
		ToolName:   "test-tool",
		ServerName: "test-server",
		Arguments:  map[string]any{"arg": "value"},
		Status:     ToolCallStatusPending,
		CreatedAt:  time.Now(),
	}
	err = store.AddToolCall(ctx, "test-run-1", toolCall)
	assertNoError(t, err)

	// Verify everything exists before deletion
	_, err = store.GetSession(ctx, "test-session-1")
	assertNoError(t, err)
	_, err = store.GetRun(ctx, "test-run-1")
	assertNoError(t, err)
	messages, err := store.GetMessages(ctx, "test-session-1")
	assertNoError(t, err)
	assertEqual(t, 1, len(messages))
	events, err := store.GetEvents(ctx, "test-run-1")
	assertNoError(t, err)
	assertEqual(t, 1, len(events))
	toolCalls, err := store.GetToolCalls(ctx, "test-run-1")
	assertNoError(t, err)
	assertEqual(t, 1, len(toolCalls))

	// Delete the session
	err = store.DeleteSession(ctx, "test-session-1")
	assertNoError(t, err)

	// Verify everything is deleted
	_, err = store.GetSession(ctx, "test-session-1")
	assertError(t, err)
	assertEqual(t, ErrNotFound, err)

	_, err = store.GetRun(ctx, "test-run-1")
	assertError(t, err)
	assertEqual(t, ErrNotFound, err)

	messages, err = store.GetMessages(ctx, "test-session-1")
	assertNoError(t, err)
	assertEqual(t, 0, len(messages))

	events, err = store.GetEvents(ctx, "test-run-1")
	assertNoError(t, err)
	assertEqual(t, 0, len(events))

	toolCalls, err = store.GetToolCalls(ctx, "test-run-1")
	assertNoError(t, err)
	assertEqual(t, 0, len(toolCalls))

	// Verify tenant index is cleaned up
	sessions, err := store.ListSessions(ctx, "tenant-1", 10, 0)
	assertNoError(t, err)
	assertEqual(t, 0, len(sessions))
}

func TestInMemoryStore_DeleteSession_NonExistent(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Try to delete a non-existent session
	err := store.DeleteSession(ctx, "non-existent")
	assertError(t, err)
	assertEqual(t, ErrNotFound, err)
}

func TestInMemoryStore_DeleteRun(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Create a session
	session := &Session{
		ID:          "test-session-1",
		TenantID:    "tenant-1",
		ProfileName: "test-profile",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	err := store.CreateSession(ctx, session)
	assertNoError(t, err)

	// Create two runs for the session
	run1 := &Run{
		ID:        "test-run-1",
		SessionID: "test-session-1",
		TenantID:  "tenant-1",
		Mode:      "interactive",
		Status:    RunStateRunning,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err = store.CreateRun(ctx, run1)
	assertNoError(t, err)

	run2 := &Run{
		ID:        "test-run-2",
		SessionID: "test-session-1",
		TenantID:  "tenant-1",
		Mode:      "interactive",
		Status:    RunStateCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err = store.CreateRun(ctx, run2)
	assertNoError(t, err)

	// Add events to both runs
	event1 := &Event{
		ID:        "test-event-1",
		RunID:     "test-run-1",
		Type:      EventTypeRunStarted,
		Data:      map[string]any{"test": "data1"},
		Timestamp: time.Now(),
	}
	err = store.AddEvent(ctx, "test-run-1", event1)
	assertNoError(t, err)

	event2 := &Event{
		ID:        "test-event-2",
		RunID:     "test-run-2",
		Type:      EventTypeRunStarted,
		Data:      map[string]any{"test": "data2"},
		Timestamp: time.Now(),
	}
	err = store.AddEvent(ctx, "test-run-2", event2)
	assertNoError(t, err)

	// Add tool calls to both runs
	toolCall1 := &ToolCall{
		ID:         "test-tool-call-1",
		RunID:      "test-run-1",
		ToolName:   "test-tool",
		ServerName: "test-server",
		Arguments:  map[string]any{"arg": "value1"},
		Status:     ToolCallStatusPending,
		CreatedAt:  time.Now(),
	}
	err = store.AddToolCall(ctx, "test-run-1", toolCall1)
	assertNoError(t, err)

	toolCall2 := &ToolCall{
		ID:         "test-tool-call-2",
		RunID:      "test-run-2",
		ToolName:   "test-tool",
		ServerName: "test-server",
		Arguments:  map[string]any{"arg": "value2"},
		Status:     ToolCallStatusCompleted,
		CreatedAt:  time.Now(),
	}
	err = store.AddToolCall(ctx, "test-run-2", toolCall2)
	assertNoError(t, err)

	// Verify both runs exist before deletion
	_, err = store.GetRun(ctx, "test-run-1")
	assertNoError(t, err)
	_, err = store.GetRun(ctx, "test-run-2")
	assertNoError(t, err)

	// Delete the first run
	err = store.DeleteRun(ctx, "test-run-1")
	assertNoError(t, err)

	// Verify first run is deleted but second remains
	_, err = store.GetRun(ctx, "test-run-1")
	assertError(t, err)
	assertEqual(t, ErrNotFound, err)

	_, err = store.GetRun(ctx, "test-run-2")
	assertNoError(t, err)

	// Verify events are cleaned up properly
	events1, err := store.GetEvents(ctx, "test-run-1")
	assertNoError(t, err)
	assertEqual(t, 0, len(events1))

	events2, err := store.GetEvents(ctx, "test-run-2")
	assertNoError(t, err)
	assertEqual(t, 1, len(events2))

	// Verify tool calls are cleaned up properly
	toolCalls1, err := store.GetToolCalls(ctx, "test-run-1")
	assertNoError(t, err)
	assertEqual(t, 0, len(toolCalls1))

	toolCalls2, err := store.GetToolCalls(ctx, "test-run-2")
	assertNoError(t, err)
	assertEqual(t, 1, len(toolCalls2))

	// Verify session index is updated
	runs, err := store.ListRuns(ctx, "test-session-1")
	assertNoError(t, err)
	assertEqual(t, 1, len(runs))
	assertEqual(t, "test-run-2", runs[0].ID)

	// Verify session still exists
	_, err = store.GetSession(ctx, "test-session-1")
	assertNoError(t, err)
}

func TestInMemoryStore_DeleteRun_NonExistent(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Try to delete a non-existent run
	err := store.DeleteRun(ctx, "non-existent")
	assertError(t, err)
	assertEqual(t, ErrNotFound, err)
}

func TestInMemoryStore_CleanupOldSessions(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Create sessions with different ages
	oldTime := time.Now().Add(-2 * time.Hour)
	recentTime := time.Now().Add(-30 * time.Minute)

	// Old session (should be cleaned up) - manually set time after creation
	oldSession := &Session{
		ID:          "old-session",
		TenantID:    "tenant-1",
		ProfileName: "test-profile",
	}
	err := store.CreateSession(ctx, oldSession)
	assertNoError(t, err)
	// Manually override the timestamp to simulate old session
	store.sessions["old-session"].CreatedAt = oldTime
	store.sessions["old-session"].UpdatedAt = oldTime

	// Recent session (should remain) - manually set time after creation
	recentSession := &Session{
		ID:          "recent-session",
		TenantID:    "tenant-1",
		ProfileName: "test-profile",
	}
	err = store.CreateSession(ctx, recentSession)
	assertNoError(t, err)
	// Manually override the timestamp to simulate recent session
	store.sessions["recent-session"].CreatedAt = recentTime
	store.sessions["recent-session"].UpdatedAt = recentTime

	// Session from different tenant (should remain)
	otherTenantSession := &Session{
		ID:          "other-tenant-session",
		TenantID:    "tenant-2",
		ProfileName: "test-profile",
	}
	err = store.CreateSession(ctx, otherTenantSession)
	assertNoError(t, err)
	// Manually override the timestamp to simulate old session
	store.sessions["other-tenant-session"].CreatedAt = oldTime
	store.sessions["other-tenant-session"].UpdatedAt = oldTime

	// Add runs and data to the old session
	oldRun := &Run{
		ID:        "old-run",
		SessionID: "old-session",
		TenantID:  "tenant-1",
		Mode:      "interactive",
		Status:    RunStateCompleted,
	}
	err = store.CreateRun(ctx, oldRun)
	assertNoError(t, err)
	store.runs["old-run"].CreatedAt = oldTime
	store.runs["old-run"].UpdatedAt = oldTime

	oldMessage := &Message{
		ID:        "old-message",
		SessionID: "old-session",
		Role:      "user",
		Content:   "Old message",
		CreatedAt: oldTime,
	}
	err = store.AddMessage(ctx, "old-session", oldMessage)
	assertNoError(t, err)

	// Verify initial state
	sessions, err := store.ListSessions(ctx, "tenant-1", 10, 0)
	assertNoError(t, err)
	assertEqual(t, 2, len(sessions))

	sessionsOtherTenant, err := store.ListSessions(ctx, "tenant-2", 10, 0)
	assertNoError(t, err)
	assertEqual(t, 1, len(sessionsOtherTenant))

	// Clean up sessions older than 1 hour for tenant-1
	err = store.CleanupOldSessions(ctx, "tenant-1", 1*time.Hour)
	assertNoError(t, err)

	// Verify cleanup results
	sessions, err = store.ListSessions(ctx, "tenant-1", 10, 0)
	assertNoError(t, err)
	assertEqual(t, 1, len(sessions))
	assertEqual(t, "recent-session", sessions[0].ID)

	// Verify other tenant is unaffected
	sessionsOtherTenant, err = store.ListSessions(ctx, "tenant-2", 10, 0)
	assertNoError(t, err)
	assertEqual(t, 1, len(sessionsOtherTenant))
	assertEqual(t, "other-tenant-session", sessionsOtherTenant[0].ID)

	// Verify old session and associated data are gone
	_, err = store.GetSession(ctx, "old-session")
	assertError(t, err)
	assertEqual(t, ErrNotFound, err)

	_, err = store.GetRun(ctx, "old-run")
	assertError(t, err)
	assertEqual(t, ErrNotFound, err)

	messages, err := store.GetMessages(ctx, "old-session")
	assertNoError(t, err)
	assertEqual(t, 0, len(messages))
}

func TestInMemoryStore_CleanupOldRuns(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Create a session
	session := &Session{
		ID:          "test-session",
		TenantID:    "tenant-1",
		ProfileName: "test-profile",
	}
	err := store.CreateSession(ctx, session)
	assertNoError(t, err)

	// Create runs with different ages and statuses
	oldTime := time.Now().Add(-2 * time.Hour)
	recentTime := time.Now().Add(-30 * time.Minute)

	// Old completed run (should be cleaned up)
	oldCompletedRun := &Run{
		ID:        "old-completed-run",
		SessionID: "test-session",
		TenantID:  "tenant-1",
		Mode:      "interactive",
		Status:    RunStateCompleted,
	}
	err = store.CreateRun(ctx, oldCompletedRun)
	assertNoError(t, err)
	store.runs["old-completed-run"].CreatedAt = oldTime
	store.runs["old-completed-run"].UpdatedAt = oldTime

	// Old failed run (should be cleaned up)
	oldFailedRun := &Run{
		ID:        "old-failed-run",
		SessionID: "test-session",
		TenantID:  "tenant-1",
		Mode:      "interactive",
		Status:    RunStateFailed,
	}
	err = store.CreateRun(ctx, oldFailedRun)
	assertNoError(t, err)
	store.runs["old-failed-run"].CreatedAt = oldTime
	store.runs["old-failed-run"].UpdatedAt = oldTime

	// Old running run (should NOT be cleaned up)
	oldRunningRun := &Run{
		ID:        "old-running-run",
		SessionID: "test-session",
		TenantID:  "tenant-1",
		Mode:      "interactive",
		Status:    RunStateRunning,
	}
	err = store.CreateRun(ctx, oldRunningRun)
	assertNoError(t, err)
	store.runs["old-running-run"].CreatedAt = oldTime
	store.runs["old-running-run"].UpdatedAt = oldTime

	// Recent completed run (should NOT be cleaned up)
	recentCompletedRun := &Run{
		ID:        "recent-completed-run",
		SessionID: "test-session",
		TenantID:  "tenant-1",
		Mode:      "interactive",
		Status:    RunStateCompleted,
	}
	err = store.CreateRun(ctx, recentCompletedRun)
	assertNoError(t, err)
	store.runs["recent-completed-run"].CreatedAt = recentTime
	store.runs["recent-completed-run"].UpdatedAt = recentTime

	// Add events and tool calls to the runs that will be deleted
	event1 := &Event{
		ID:        "event-1",
		RunID:     "old-completed-run",
		Type:      EventTypeRunCompleted,
		Data:      map[string]any{"test": "data"},
		Timestamp: oldTime,
	}
	err = store.AddEvent(ctx, "old-completed-run", event1)
	assertNoError(t, err)

	toolCall1 := &ToolCall{
		ID:         "tool-call-1",
		RunID:      "old-completed-run",
		ToolName:   "test-tool",
		ServerName: "test-server",
		Arguments:  map[string]any{"arg": "value"},
		Status:     ToolCallStatusCompleted,
		CreatedAt:  oldTime,
	}
	err = store.AddToolCall(ctx, "old-completed-run", toolCall1)
	assertNoError(t, err)

	// Verify initial state
	runs, err := store.ListRuns(ctx, "test-session")
	assertNoError(t, err)
	assertEqual(t, 4, len(runs))

	// Clean up runs older than 1 hour
	err = store.CleanupOldRuns(ctx, "test-session", 1*time.Hour)
	assertNoError(t, err)

	// Verify cleanup results - should be 2 remaining (old-running-run and recent-completed-run)
	runs, err = store.ListRuns(ctx, "test-session")
	assertNoError(t, err)

	// We expect only 2 runs to remain: old-running-run and recent-completed-run
	// The old-completed-run and old-failed-run should be deleted
	assertEqual(t, 2, len(runs))

	// Verify the correct runs remain
	runIDs := make(map[string]bool)
	for _, run := range runs {
		runIDs[run.ID] = true
	}

	// These should remain
	if !runIDs["old-running-run"] {
		t.Error("Expected old-running-run to remain (running status)")
	}
	if !runIDs["recent-completed-run"] {
		t.Error("Expected recent-completed-run to remain (recent)")
	}

	// These should be deleted
	if runIDs["old-completed-run"] {
		t.Error("Expected old-completed-run to be deleted")
	}
	if runIDs["old-failed-run"] {
		t.Error("Expected old-failed-run to be deleted")
	}

	// Verify associated data is cleaned up
	events, err := store.GetEvents(ctx, "old-completed-run")
	assertNoError(t, err)
	assertEqual(t, 0, len(events))

	toolCalls, err := store.GetToolCalls(ctx, "old-completed-run")
	assertNoError(t, err)
	assertEqual(t, 0, len(toolCalls))

	// Verify session still exists
	_, err = store.GetSession(ctx, "test-session")
	assertNoError(t, err)
}

func TestInMemoryStore_CleanupOldSessions_EmptyTenant(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Try to clean up sessions for a tenant with no sessions
	err := store.CleanupOldSessions(ctx, "non-existent-tenant", 1*time.Hour)
	assertNoError(t, err)
}

func TestInMemoryStore_CleanupOldRuns_NonExistentSession(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Try to clean up runs for a non-existent session
	err := store.CleanupOldRuns(ctx, "non-existent-session", 1*time.Hour)
	assertNoError(t, err)
}
