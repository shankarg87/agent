package store

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shankarg87/agent/internal/logging"
)

var (
	ErrNotFound = errors.New("not found")
)

// InMemoryStore implements Store using in-memory data structures
type InMemoryStore struct {
	mu sync.RWMutex

	sessions  map[string]*Session
	runs      map[string]*Run
	messages  map[string][]*Message  // sessionID -> messages
	events    map[string][]*Event    // runID -> events
	toolCalls map[string][]*ToolCall // runID -> tool calls

	// Indexes
	sessionsByTenant map[string][]string // tenantID -> sessionIDs
	runsBySession    map[string][]string // sessionID -> runIDs

	logger *logging.SimpleLogger
}

// NewInMemoryStore creates a new in-memory store
func NewInMemoryStore() *InMemoryStore {
	logger := logging.VerboseLogger("store")
	logger.Verbose("Creating new in-memory store")

	return &InMemoryStore{
		sessions:         make(map[string]*Session),
		runs:             make(map[string]*Run),
		messages:         make(map[string][]*Message),
		events:           make(map[string][]*Event),
		toolCalls:        make(map[string][]*ToolCall),
		sessionsByTenant: make(map[string][]string),
		runsBySession:    make(map[string][]string),
		logger:           logger,
	}
}

// Sessions

func (s *InMemoryStore) CreateSession(ctx context.Context, session *Session) error {
	start := time.Now()
	s.logger.Verbose("Creating session", "session_id", session.ID, "tenant_id", session.TenantID)

	s.mu.Lock()
	defer s.mu.Unlock()

	if session.ID == "" {
		session.ID = uuid.New().String()
		s.logger.Verbose("Generated new session ID", "session_id", session.ID)
	}
	session.CreatedAt = time.Now()
	session.UpdatedAt = session.CreatedAt

	s.sessions[session.ID] = session
	s.sessionsByTenant[session.TenantID] = append(s.sessionsByTenant[session.TenantID], session.ID)

	s.logger.LogMemoryOperation("create_session", session.ID, true, time.Since(start))
	return nil
}

func (s *InMemoryStore) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	start := time.Now()
	s.logger.Verbose("Getting session", "session_id", sessionID)

	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		s.logger.LogMemoryOperation("get_session", sessionID, false, time.Since(start))
		return nil, ErrNotFound
	}

	s.logger.LogMemoryOperation("get_session", sessionID, true, time.Since(start))
	return session, nil
}

func (s *InMemoryStore) ListSessions(ctx context.Context, tenantID string, limit, offset int) ([]*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessionIDs := s.sessionsByTenant[tenantID]
	if offset >= len(sessionIDs) {
		return []*Session{}, nil
	}

	end := offset + limit
	if end > len(sessionIDs) {
		end = len(sessionIDs)
	}

	sessions := make([]*Session, 0, end-offset)
	for _, id := range sessionIDs[offset:end] {
		if session, ok := s.sessions[id]; ok {
			sessions = append(sessions, session)
		}
	}

	return sessions, nil
}

// Runs

func (s *InMemoryStore) CreateRun(ctx context.Context, run *Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if run.ID == "" {
		run.ID = uuid.New().String()
	}
	run.CreatedAt = time.Now()
	run.UpdatedAt = run.CreatedAt

	s.runs[run.ID] = run
	s.runsBySession[run.SessionID] = append(s.runsBySession[run.SessionID], run.ID)

	return nil
}

func (s *InMemoryStore) GetRun(ctx context.Context, runID string) (*Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	run, ok := s.runs[runID]
	if !ok {
		return nil, ErrNotFound
	}
	return run, nil
}

func (s *InMemoryStore) UpdateRun(ctx context.Context, run *Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.runs[run.ID]; !ok {
		return ErrNotFound
	}

	run.UpdatedAt = time.Now()
	s.runs[run.ID] = run

	return nil
}

func (s *InMemoryStore) ListRuns(ctx context.Context, sessionID string) ([]*Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	runIDs := s.runsBySession[sessionID]
	runs := make([]*Run, 0, len(runIDs))
	for _, id := range runIDs {
		if run, ok := s.runs[id]; ok {
			runs = append(runs, run)
		}
	}

	return runs, nil
}

// Messages

func (s *InMemoryStore) AddMessage(ctx context.Context, sessionID string, message *Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if message.ID == "" {
		message.ID = uuid.New().String()
	}
	message.CreatedAt = time.Now()
	message.SessionID = sessionID

	s.messages[sessionID] = append(s.messages[sessionID], message)

	return nil
}

func (s *InMemoryStore) GetMessages(ctx context.Context, sessionID string) ([]*Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	messages := s.messages[sessionID]
	if messages == nil {
		return []*Message{}, nil
	}

	return messages, nil
}

// Events

func (s *InMemoryStore) AddEvent(ctx context.Context, runID string, event *Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	event.Timestamp = time.Now()
	event.RunID = runID

	s.events[runID] = append(s.events[runID], event)

	return nil
}

func (s *InMemoryStore) GetEvents(ctx context.Context, runID string) ([]*Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	events := s.events[runID]
	if events == nil {
		return []*Event{}, nil
	}

	return events, nil
}

// Tool calls

func (s *InMemoryStore) AddToolCall(ctx context.Context, runID string, toolCall *ToolCall) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if toolCall.ID == "" {
		toolCall.ID = uuid.New().String()
	}
	toolCall.CreatedAt = time.Now()
	toolCall.RunID = runID

	s.toolCalls[runID] = append(s.toolCalls[runID], toolCall)

	return nil
}

func (s *InMemoryStore) UpdateToolCall(ctx context.Context, toolCall *ToolCall) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	calls := s.toolCalls[toolCall.RunID]
	for i, tc := range calls {
		if tc.ID == toolCall.ID {
			s.toolCalls[toolCall.RunID][i] = toolCall
			return nil
		}
	}

	return ErrNotFound
}

func (s *InMemoryStore) GetToolCalls(ctx context.Context, runID string) ([]*ToolCall, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	toolCalls := s.toolCalls[runID]
	if toolCalls == nil {
		return []*ToolCall{}, nil
	}

	return toolCalls, nil
}

// DeleteSession removes a session and all associated data
func (s *InMemoryStore) DeleteSession(ctx context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, exists := s.sessions[sessionID]
	if !exists {
		return ErrNotFound
	}

	s.deleteSessionLocked(sessionID)

	s.logger.Verbose("Session deleted with all associated data", "session_id", sessionID)
	return nil
}

// deleteSessionLocked is a helper that assumes the caller holds the write lock
func (s *InMemoryStore) deleteSessionLocked(sessionID string) {
	session, exists := s.sessions[sessionID]
	if !exists {
		return
	}

	// Clean up runs associated with this session
	runIDs := s.runsBySession[sessionID]
	for _, runID := range runIDs {
		// Delete run data
		delete(s.runs, runID)
		delete(s.events, runID)
		delete(s.toolCalls, runID)
	}

	// Clean up session data
	delete(s.sessions, sessionID)
	delete(s.messages, sessionID)
	delete(s.runsBySession, sessionID)

	// Remove from tenant index
	if tenantSessions, ok := s.sessionsByTenant[session.TenantID]; ok {
		for i, id := range tenantSessions {
			if id == sessionID {
				s.sessionsByTenant[session.TenantID] = append(tenantSessions[:i], tenantSessions[i+1:]...)
				break
			}
		}
	}
}

// DeleteRun removes a run and its associated events/tool calls
func (s *InMemoryStore) DeleteRun(ctx context.Context, runID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, exists := s.runs[runID]
	if !exists {
		return ErrNotFound
	}

	s.deleteRunLocked(runID)

	s.logger.Verbose("Run deleted with all associated data", "run_id", runID)
	return nil
}

// deleteRunLocked is a helper that assumes the caller holds the write lock
func (s *InMemoryStore) deleteRunLocked(runID string) {
	run, exists := s.runs[runID]
	if !exists {
		return
	}

	// Clean up run data
	delete(s.runs, runID)
	delete(s.events, runID)
	delete(s.toolCalls, runID)

	// Remove from session index
	if sessionRuns, ok := s.runsBySession[run.SessionID]; ok {
		for i, id := range sessionRuns {
			if id == runID {
				s.runsBySession[run.SessionID] = append(sessionRuns[:i], sessionRuns[i+1:]...)
				break
			}
		}
	}
}

// CleanupOldSessions removes sessions older than the specified duration
func (s *InMemoryStore) CleanupOldSessions(ctx context.Context, tenantID string, olderThan time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-olderThan)
	deletedCount := 0

	sessionIDs, exists := s.sessionsByTenant[tenantID]
	if !exists {
		s.logger.Verbose("Cleaned up old sessions",
			"tenant_id", tenantID,
			"deleted_count", deletedCount,
			"older_than", olderThan)
		return nil
	}

	var remainingSessions []string

	for _, sessionID := range sessionIDs {
		if session, exists := s.sessions[sessionID]; exists && session.CreatedAt.Before(cutoff) {
			s.deleteSessionLocked(sessionID)
			deletedCount++
		} else if exists {
			remainingSessions = append(remainingSessions, sessionID)
		}
	}

	// Update the tenant index with remaining sessions
	if len(remainingSessions) == 0 {
		delete(s.sessionsByTenant, tenantID)
	} else {
		s.sessionsByTenant[tenantID] = remainingSessions
	}

	s.logger.Verbose("Cleaned up old sessions",
		"tenant_id", tenantID,
		"deleted_count", deletedCount,
		"older_than", olderThan)
	return nil
}

// CleanupOldRuns removes completed runs older than the specified duration for a session
func (s *InMemoryStore) CleanupOldRuns(ctx context.Context, sessionID string, olderThan time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-olderThan)
	deletedCount := 0

	runIDs, exists := s.runsBySession[sessionID]
	if !exists {
		s.logger.Verbose("Cleaned up old runs",
			"session_id", sessionID,
			"deleted_count", deletedCount,
			"older_than", olderThan)
		return nil
	}

	var remainingRuns []string
	var runsToDelete []string

	// First pass: identify which runs to delete
	for _, runID := range runIDs {
		if run, exists := s.runs[runID]; exists {
			// Only delete completed, failed, or cancelled runs
			if (run.Status == RunStateCompleted || run.Status == RunStateFailed || run.Status == RunStateCancelled) &&
				run.CreatedAt.Before(cutoff) {
				runsToDelete = append(runsToDelete, runID)
			} else {
				remainingRuns = append(remainingRuns, runID)
			}
		} else {
			// Run doesn't exist, don't include it in remaining runs
		}
	}

	// Second pass: delete the identified runs
	for _, runID := range runsToDelete {
		delete(s.runs, runID)
		delete(s.events, runID)
		delete(s.toolCalls, runID)
		deletedCount++
	}

	// Update the session index with remaining runs
	if len(remainingRuns) == 0 {
		delete(s.runsBySession, sessionID)
	} else {
		s.runsBySession[sessionID] = remainingRuns
	}

	s.logger.Verbose("Cleaned up old runs",
		"session_id", sessionID,
		"deleted_count", deletedCount,
		"older_than", olderThan)
	return nil
}
