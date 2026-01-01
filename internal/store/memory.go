package store

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
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
}

// NewInMemoryStore creates a new in-memory store
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		sessions:         make(map[string]*Session),
		runs:             make(map[string]*Run),
		messages:         make(map[string][]*Message),
		events:           make(map[string][]*Event),
		toolCalls:        make(map[string][]*ToolCall),
		sessionsByTenant: make(map[string][]string),
		runsBySession:    make(map[string][]string),
	}
}

// Sessions

func (s *InMemoryStore) CreateSession(ctx context.Context, session *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if session.ID == "" {
		session.ID = uuid.New().String()
	}
	session.CreatedAt = time.Now()
	session.UpdatedAt = session.CreatedAt

	s.sessions[session.ID] = session
	s.sessionsByTenant[session.TenantID] = append(s.sessionsByTenant[session.TenantID], session.ID)

	return nil
}

func (s *InMemoryStore) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return nil, ErrNotFound
	}
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
