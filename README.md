# Agent - MCP-First Swiss Army Knife Runtime

A general-purpose, MCP-first agent runtime that gains capabilities through tools exposed via Model Context Protocol (MCP) servers rather than hard-coded business logic.

## Features

- **MCP-First Architecture**: Pluggable tools via MCP servers
- **Dual API Surface**:
  - Native `/runs` API for full agent capabilities
  - OpenAI-compatible `/v1/chat/completions` for standard clients
- **Multiple Modes**:
  - Interactive (human-in-the-loop)
  - Autonomous (daemon mode)
- **Multi-Provider Support**: Anthropic, OpenAI, Gemini, Ollama (Anthropic & OpenAI implemented)
- **Event Streaming**: Real-time SSE streaming of run events
- **Configurable Profiles**: Agent behavior defined through YAML configuration

## Quick Start

### Prerequisites

- Go 1.21+
- Anthropic API key or OpenAI API key

### Installation

```bash
# Clone the repository
cd agent

# Install dependencies
go mod download

# Build the agent daemon
go build -o bin/agentd ./cmd/agentd

# Build the example echo MCP server
go build -o examples/mcp-servers/echo/echo-server ./examples/mcp-servers/echo
```

### Configuration

Set your API key:

```bash
export ANTHROPIC_API_KEY=your_api_key_here
# or
export OPENAI_API_KEY=your_api_key_here
```

### Running the Agent

```bash
# Start the agent server (default port 8080)
./bin/agentd

# With custom configuration files and port
./bin/agentd --config configs/agents/default.yaml --mcp-config configs/mcp/servers.yaml --addr :8080

# With environment variables
export AGENT_CONFIG=configs/agents/custom.yaml
export AGENT_ADDR=:9000
./bin/agentd

# Disable configuration file watching (useful for production)
./bin/agentd --watch-config=false
```

#### Dynamic Configuration
The agent now supports dynamic configuration reloading:
- Configuration files are automatically watched for changes
- New runs use updated configuration immediately
- Existing runs continue with their original configuration
- See [docs/DYNAMIC_CONFIG.md](docs/DYNAMIC_CONFIG.md) for details

#### CLI Flags
- `--config`: Path to agent configuration file (default: `configs/agents/default.yaml`)
- `--mcp-config`: Path to MCP servers configuration file (default: `configs/mcp/servers.yaml`)
- `--addr`: HTTP server address (default: `:8080`)
- `--watch-config`: Enable configuration file watching (default: `true`)

All flags can be set via environment variables with `AGENT_` prefix (e.g., `AGENT_CONFIG`, `AGENT_ADDR`)

## API Usage

### Native `/runs` API

#### Create a Run

```bash
curl -X POST http://localhost:8080/runs \
  -H "Content-Type: application/json" \
  -d '{
    "mode": "interactive",
    "input": "Echo this message: Hello World!"
  }'
```

Response:
```json
{
  "id": "run_abc123",
  "session_id": "session_xyz",
  "status": "queued",
  "mode": "interactive",
  ...
}
```

#### Get Run Status

```bash
curl http://localhost:8080/runs/run_abc123
```

#### Stream Run Events (SSE)

```bash
curl -N http://localhost:8080/runs/run_abc123/events
```

Output:
```
event: run_started
data: {"id":"evt_1","run_id":"run_abc123","type":"run_started",...}

event: text_delta
data: {"id":"evt_2","run_id":"run_abc123","type":"text_delta","data":{"text":"I'll help..."}}

event: tool_started
data: {"id":"evt_3","run_id":"run_abc123","type":"tool_started","data":{"tool_name":"echo",...}}

event: tool_completed
data: {"id":"evt_4","run_id":"run_abc123","type":"tool_completed",...}

event: run_completed
data: {"id":"evt_5","run_id":"run_abc123","type":"run_completed",...}
```

#### Cancel a Run

```bash
curl -X POST http://localhost:8080/runs/run_abc123/cancel
```

### OpenAI-Compatible `/v1/chat/completions` API

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "user", "content": "Echo: Hello World"}
    ],
    "stream": false
  }'
```

Streaming:
```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "user", "content": "Echo: Hello World"}
    ],
    "stream": true
  }'
```

## Configuration

### Agent Profile (`configs/agents/default.yaml`)

```yaml
profile_name: "default"
profile_version: "1.0.0"
description: "General-purpose Swiss Army Knife agent"

# Model Configuration
primary_model:
  provider: "anthropic"  # or "openai"
  model: "claude-sonnet-4-5-20250929"

# System Prompt
system_prompt: |
  You are a helpful AI assistant with access to various tools...

# Limits
max_tool_calls: 50
max_run_time_seconds: 300
max_failures_per_run: 3

# Approval & Checkpoints
approval_mode: "policy"
auto_approve_in_daemon: true

# See configs/agents/default.yaml for full configuration
```

### MCP Servers (`configs/mcp/servers.yaml`)

```yaml
servers:
  - name: "echo"
    transport: "stdio"
    endpoint: "./examples/mcp-servers/echo/echo-server"
    args: []
    timeout: 30s
```

## Architecture

```
┌─────────────────────────────────────────┐
│           HTTP Server                    │
├─────────────────┬───────────────────────┤
│  /runs API      │  /v1/chat/completions │
│  (Native)       │  (OpenAI-compatible)  │
└────────┬────────┴────────┬──────────────┘
         │                 │
         └────────┬────────┘
                  │
         ┌────────▼────────┐
         │    Runtime      │
         │  (State Machine)│
         └────────┬────────┘
                  │
      ┌───────────┼───────────┐
      │           │           │
┌─────▼─────┐ ┌──▼───┐ ┌─────▼──────┐
│ LLM       │ │ MCP  │ │ Event Bus  │
│ Provider  │ │ Reg  │ │ (Streaming)│
└───────────┘ └──┬───┘ └────────────┘
                 │
         ┌───────┼───────┐
    ┌────▼────┐ ┌▼──────┐
    │ MCP     │ │ MCP   │
    │ Server1 │ │Server2│
    └─────────┘ └───────┘
```

## Project Structure

```
agent/
├── cmd/
│   └── agentd/          # Main HTTP server
├── internal/
│   ├── config/          # Configuration loading
│   ├── events/          # Event bus for streaming
│   ├── mcp/             # MCP client registry
│   ├── provider/        # LLM provider abstraction
│   ├── runtime/         # Core agent runtime & APIs
│   └── store/           # Persistence layer
├── configs/
│   ├── agents/          # Agent profile configs
│   └── mcp/             # MCP server configs
└── examples/
    └── mcp-servers/
        └── echo/        # Example MCP server
```

## Creating Custom MCP Servers

```go
package main

import (
    "context"
    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"
)

func main() {
    s := server.NewMCPServer("my-server", "1.0.0")

    tool := mcp.NewTool("my_tool",
        mcp.WithDescription("Description of my tool"),
        mcp.WithString("param1", mcp.Required()),
    )

    s.AddTool(tool, handleMyTool)
    server.ServeStdio(s)
}

func handleMyTool(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    param, _ := req.RequireString("param1")
    return mcp.NewToolResultText("Result: " + param), nil
}
```

## Integrations

### Use with Open WebUI

```bash
# In Open WebUI, add a new OpenAI connection:
# API Base URL: http://localhost:8080/v1
# API Key: (not required)
# Model: (any name, will use agent's configured model)
```

### Use with Continue (VSCode Extension)

```json
{
  "models": [
    {
      "title": "Agent",
      "provider": "openai",
      "model": "agent",
      "apiBase": "http://localhost:8080/v1"
    }
  ]
}
```

## Testing

The project includes comprehensive E2E tests that validate the complete system functionality:

```bash
# Run all tests
make test

# Run only E2E tests
make test-e2e

# Run only unit tests
make test-unit
```

### E2E Test Coverage
- **Core APIs**: Run creation, retrieval, cancellation, and event streaming
- **OpenAI Compatibility**: Both streaming and non-streaming chat completions
- **MCP Integration**: Tool invocation and lifecycle events
- **Concurrent Operations**: Multiple simultaneous runs
- **Error Scenarios**: Invalid requests and edge cases

See [E2E Testing Guide](docs/E2E_TESTING.md) for detailed documentation.

## Development Roadmap

### V1 (Current)
- ✅ Synchronous tool execution
- ✅ In-memory storage
- ✅ Interactive & autonomous modes
- ✅ Event streaming (SSE)
- ✅ Anthropic & OpenAI providers

### V2 (Future)
- [ ] Async/event-driven tool execution
- [ ] PostgreSQL persistence
- [ ] Gemini & Ollama providers
- [ ] Long-term memory via MCP
- [ ] WebSocket event streaming
- [ ] Multi-tenant auth & isolation
- [ ] Checkpoint approval workflows
- [ ] Cost tracking & budgets

## Contributing

This is an early-stage project. Contributions welcome!

### Development Setup

1. **Clone and build**:
   ```bash
   git clone <repo>
   cd agent
   make deps
   make build
   ```

2. **Set up pre-commit hooks** (recommended):
   ```bash
   make install-hooks
   ```
   This installs Git pre-commit hooks that automatically run code formatting, linting, tests, and security checks before each commit. See [Pre-commit Setup Guide](docs/PRE_COMMIT_SETUP.md) for details.

3. **Run tests**:
   ```bash
   make test-unit    # Unit tests only
   make test-e2e     # E2E tests only
   make test         # All tests
   ```

4. **Run full CI checks**:
   ```bash
   make ci-check     # What CI will run
   ```

## License

MIT License - see LICENSE file for details
