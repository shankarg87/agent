# E2E Testing Guide

This document describes the End-to-End (E2E) testing strategy and implementation for the agent project.

## Overview

The E2E tests validate the complete system functionality by:
- Starting a full test server with all components
- Making HTTP API calls against the server
- Verifying responses and system behavior
- Testing integration with MCP tools

## Test Structure

### Location
E2E tests are located in `/test/e2e_test.go`

### Test Categories

1. **Core API Tests** (`TestRunsAPI`)
   - Create and retrieve runs
   - Event streaming via Server-Sent Events
   - Run cancellation
   - Error handling

2. **OpenAI-Compatible API Tests** (`TestOpenAICompatibleAPI`)
   - Non-streaming chat completions
   - Streaming chat completions  
   - OpenAI response format validation

3. **MCP Tool Integration Tests** (`TestMCPToolIntegration`)
   - Tool invocation and execution
   - Event verification for tool lifecycle
   - Integration with echo MCP server

4. **Concurrent Operations Tests** (`TestConcurrentOperations`)
   - Multiple simultaneous runs
   - Resource isolation
   - Performance validation

5. **Error Scenarios Tests** (`TestErrorScenarios`)
   - Invalid requests
   - Non-existent resources
   - Method validation

## Running E2E Tests

### Prerequisites
```bash
# Build required binaries
make build-echo
```

### Run All E2E Tests
```bash
make test-e2e
```

### Run Specific Test Categories
```bash
# Core API tests only
go test ./test -v -run TestRunsAPI

# OpenAI API tests only  
go test ./test -v -run TestOpenAICompatibleAPI

# MCP integration tests only
go test ./test -v -run TestMCPToolIntegration
```

### Run Individual Tests
```bash
# Specific test case
go test ./test -v -run TestRunsAPI/CreateAndGetRun
```

## Test Implementation Details

### Test Server Setup
- Each test uses `setupTestServer()` to create an isolated test environment
- Uses `httptest.Server` for HTTP endpoint testing
- Includes mock LLM provider to avoid external dependencies
- Loads real MCP configuration and starts echo server

### Mock LLM Provider
The tests use a `MockLLMProvider` that:
- Returns consistent responses for deterministic testing
- Simulates tool calls when requested
- Handles both streaming and non-streaming scenarios
- Provides realistic token usage data

### Event Stream Testing
- Uses timeouts to prevent hanging on SSE streams
- Validates specific event types (run_started, tool_started, etc.)
- Handles partial reads for long-running streams

### Error Testing
- Validates HTTP status codes
- Tests malformed JSON handling
- Verifies error message formats

## Configuration

### Test Configuration
Tests use the default agent configuration from `configs/agents/default.yaml` and MCP configuration from `configs/mcp/servers.yaml`.

### MCP Server Dependencies
The E2E tests require the echo MCP server to be built:
- Location: `examples/mcp-servers/echo/echo-server`
- Built automatically via `make build-echo`

## Adding New Tests

### Test Structure Template
```go
func TestNewFeature(t *testing.T) {
    ts := setupTestServer(t)
    defer ts.Close()
    
    t.Run("SpecificScenario", func(t *testing.T) {
        // Test implementation
    })
}
```

### Best Practices
1. Always use `setupTestServer()` for test isolation
2. Add meaningful test names that describe the scenario
3. Include both success and error cases
4. Use timeouts for operations that might hang
5. Clean up resources with `defer ts.Close()`

### Common Patterns

#### Creating a Run
```go
createReq := map[string]interface{}{
    "tenant_id": "test-tenant",
    "mode":      "interactive",
    "input":     "Your test input",
}

body, _ := json.Marshal(createReq)
resp, err := http.Post(ts.URL()+"/runs", "application/json", bytes.NewBuffer(body))
// Handle response...
```

#### Polling for Completion
```go
timeout := time.After(10 * time.Second)
ticker := time.NewTicker(200 * time.Millisecond)
defer ticker.Stop()

for {
    select {
    case <-timeout:
        t.Fatal("Operation timed out")
    case <-ticker.C:
        // Check status...
        if completed {
            goto done
        }
    }
}
done:
```

#### Reading SSE Events with Timeout
```go
done := make(chan bool, 1)
go func() {
    scanner := bufio.NewScanner(resp.Body)
    // Read events...
    done <- true
}()

select {
case <-done:
    // Success
case <-time.After(timeout):
    // Handle timeout
}
```

## Troubleshooting

### Common Issues

1. **Test Hangs**: Usually caused by reading SSE streams without timeout
   - Solution: Always use goroutines with timeouts for stream reading

2. **MCP Server Not Found**: Echo server binary not built
   - Solution: Run `make build-echo` before tests

3. **Port Conflicts**: Multiple test runs interfering
   - Solution: Use `httptest.Server` which automatically assigns free ports

4. **Timing Issues**: Race conditions between run creation and status checks
   - Solution: Add appropriate delays or polling with retries

### Debug Tips

1. Add logging to see what's happening:
   ```go
   t.Logf("Status: %s, Output: %s", status, output)
   ```

2. Check the actual HTTP response bodies:
   ```go
   body, _ := io.ReadAll(resp.Body)
   t.Logf("Response: %s", string(body))
   ```

3. Run with verbose output:
   ```bash
   go test ./test -v -run TestName
   ```

## Performance Considerations

- E2E tests take longer than unit tests (2-3 seconds typical)
- Each test creates a new server instance for isolation
- Tests run sequentially to avoid resource conflicts
- Use appropriate timeouts (typically 10-30 seconds for complex operations)

## Integration with CI/CD

The E2E tests are designed to run in CI environments:
- No external dependencies (beyond built binaries)
- Deterministic behavior via mock providers  
- Proper cleanup and resource management
- Clear pass/fail criteria with meaningful error messages
