# Pause/Resume Functionality

This document describes the newly added pause/resume functionality for agent runs.

## Overview

The pause/resume functionality allows users to manually pause an agent run during execution and later resume it. This is distinct from the existing checkpoint-based pausing that occurs automatically when tools require approval.

## New Features

### Run States

- **`RunStatePaused`**: New run state for user-initiated pauses
- **`RunStatePausedCheckpoint`**: Existing state for automatic checkpoint pauses (unchanged)

### Event Types

- **`EventTypeRunPaused`**: Emitted when a run is paused by user request
- **`EventTypeRunResumed`**: Emitted when a run is resumed by user request

### API Endpoints

#### Pause a Run
```
POST /runs/{id}/pause
```

**Response:**
```json
{
  "status": "paused",
  "run_id": "run-123"
}
```

**Status Codes:**
- `200 OK`: Run successfully paused
- `400 Bad Request`: Run is not in a pausable state
- `404 Not Found`: Run not found
- `500 Internal Server Error`: Server error

#### Resume a Run
```
POST /runs/{id}/resume
```

**Response:**
```json
{
  "status": "resumed", 
  "run_id": "run-123"
}
```

**Status Codes:**
- `200 OK`: Run successfully resumed
- `400 Bad Request`: Run is not paused
- `404 Not Found`: Run not found
- `500 Internal Server Error`: Server error

### Behavior

1. **Graceful Pausing**: When a pause is requested, the agent loop will complete its current iteration before pausing. This ensures the run is in a consistent state.

2. **State Persistence**: The paused state is persisted in the store, so paused runs survive server restarts.

3. **Event Streaming**: 
   - Pause/resume events are sent through the event stream
   - Users monitoring `/runs/{id}/events` will receive real-time notifications
   - OpenAI-compatible streaming API includes pause/resume notifications in the stream

4. **Status Handling**:
   - GET `/runs/{id}` returns the current state including "paused"
   - V1 API returns appropriate HTTP status for paused runs
   - Paused runs don't timeout while paused

## Usage Examples

### Using cURL

```bash
# Pause a running run
curl -X POST http://localhost:8080/runs/run-123/pause

# Resume a paused run  
curl -X POST http://localhost:8080/runs/run-123/resume

# Check run status
curl http://localhost:8080/runs/run-123
```

### Using the Go API

```go
// Pause a run
err := runtime.PauseRun(ctx, "run-123")
if err != nil {
    log.Printf("Failed to pause run: %v", err)
}

// Resume a run
err = runtime.ResumeRun(ctx, "run-123") 
if err != nil {
    log.Printf("Failed to resume run: %v", err)
}
```

## Implementation Details

### RunContext Extensions

The `RunContext` struct now includes:
- `isPaused bool`: Tracks pause state
- `pauseSignal chan struct{}`: Channel for pause signals
- `resumeSignal chan struct{}`: Channel for resume signals
- `mu sync.RWMutex`: Mutex for thread-safe access

### Agent Loop Integration

The main agent loop (`runAgentLoop`) checks for pause signals at the beginning of each iteration:

1. **Pause Detection**: Checks the `pauseSignal` channel
2. **Graceful Pause**: Publishes pause notification and waits for resume
3. **Resume Detection**: Waits on `resumeSignal` channel
4. **Continuation**: Continues execution after resume

### Thread Safety

- All pause/resume operations are thread-safe using mutexes
- Multiple pause/resume requests are handled gracefully
- State consistency is maintained across concurrent operations

## Error Handling

- **Invalid State Transitions**: Attempting to pause a non-running run or resume a non-paused run returns an error
- **Run Not Found**: Operations on non-existent runs return appropriate 404 errors  
- **Concurrent Operations**: Multiple pause/resume calls are handled safely

## Compatibility

- **Backward Compatible**: Existing APIs and functionality are unchanged
- **OpenAI API**: Pause/resume events are properly handled in streaming responses
- **Event System**: Integrates seamlessly with existing event bus

## Limitations

- Pausing only occurs at safe points (between agent loop iterations)
- Tool execution cannot be paused mid-execution
- Very fast runs might complete before pause takes effect

## Future Enhancements

- **Scheduled Pausing**: Automatic pausing after specified durations
- **Conditional Pausing**: Pause based on cost thresholds or other conditions  
- **Batch Operations**: Pause/resume multiple runs at once
- **Pause Reasons**: Track and display why a run was paused
