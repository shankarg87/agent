# Contributing to Agent Runtime

Thank you for your interest in contributing to the Agent Runtime project!

## Development Setup

### Prerequisites

- Go 1.21 or higher
- Git
- An LLM API key (Anthropic or OpenAI)

### Getting Started

```bash
# Clone the repository
git clone <repository-url>
cd agent

# Install dependencies
make deps

# Build the project
make build

# Run tests
make test

# Run the agent
export ANTHROPIC_API_KEY=your_key_here
make run
```

## Project Structure

See [PROJECT_STATUS.md](PROJECT_STATUS.md) for a detailed overview of the codebase structure.

Key directories:
- `cmd/agentd` - Main HTTP server
- `internal/runtime` - Core agent runtime & APIs
- `internal/provider` - LLM provider implementations
- `internal/mcp` - MCP client integration
- `internal/store` - Storage layer
- `configs/` - Configuration files
- `examples/` - Example MCP servers

## Making Changes

### Code Style

- Follow standard Go conventions
- Run `make fmt` before committing
- Run `make lint` to check for issues
- Keep functions focused and well-documented
- Use meaningful variable names

### Testing

Currently, the project focuses on integration testing via manual verification. Unit tests are welcome contributions!

```bash
# Run existing tests
make test

# Add tests in *_test.go files
# Example: internal/store/memory_test.go
```

### Adding a New Feature

1. Check [PROJECT_STATUS.md](PROJECT_STATUS.md) for V2 roadmap items
2. Create an issue describing your proposed feature
3. Wait for feedback before starting implementation
4. Fork the repository
5. Create a feature branch (`git checkout -b feature/amazing-feature`)
6. Implement your feature
7. Add tests if applicable
8. Update documentation
9. Submit a pull request

### Adding a New LLM Provider

To add support for a new LLM provider (e.g., Gemini, Ollama):

1. Implement the `provider.Provider` interface in `internal/provider/`
2. Add constructor in `provider.NewProvider()`
3. Update default config in `configs/agents/default.yaml`
4. Update documentation

Example structure (see `anthropic.go` or `openai.go`):

```go
type MyProvider struct {
    apiKey string
    model  string
}

func (p *MyProvider) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
    // Implementation
}

func (p *MyProvider) Stream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error) {
    // Implementation
}
```

### Adding a New Storage Backend

To add a new storage backend (e.g., PostgreSQL):

1. Implement the `store.Store` interface in `internal/store/`
2. Update `main.go` to support new backend via flag
3. Add migration scripts if needed
4. Update configuration documentation

### Creating MCP Servers

Example MCP servers should be added to `examples/mcp-servers/`:

```bash
mkdir examples/mcp-servers/myserver
# Create main.go with server implementation
# Add to configs/mcp/servers.yaml
```

See `examples/mcp-servers/echo/` for a reference implementation.

## Pull Request Guidelines

### Before Submitting

- [ ] Code follows Go conventions
- [ ] `make fmt` has been run
- [ ] `make lint` passes
- [ ] `make build` succeeds
- [ ] Documentation is updated
- [ ] CHANGELOG.md is updated (if applicable)

### PR Description

Include:
- **What**: Brief description of changes
- **Why**: Motivation and context
- **How**: Technical approach
- **Testing**: How to test the changes
- **Screenshots**: If applicable (UI changes)

### Review Process

1. Automated checks will run (once CI is set up)
2. Maintainers will review your code
3. Address any feedback
4. Once approved, your PR will be merged

## Coding Guidelines

### Error Handling

- Always handle errors explicitly
- Use `fmt.Errorf` with `%w` for error wrapping
- Log errors with context

```go
if err != nil {
    return fmt.Errorf("failed to process request: %w", err)
}
```

### Logging

- Use structured logging (when logger is added)
- Include relevant context (run ID, session ID, etc.)
- Use appropriate log levels

### Configuration

- Add new config options to `internal/config/agent.go`
- Set sensible defaults
- Document all options in `configs/agents/default.yaml`

### Concurrency

- Use mutexes to protect shared state
- Prefer channels for communication
- Document goroutine lifecycle

## Areas Needing Contribution

### High Priority

- [ ] Unit tests for core components
- [ ] PostgreSQL storage backend
- [ ] Gemini provider implementation
- [ ] Ollama provider implementation
- [ ] Integration tests

### Medium Priority

- [ ] Authentication middleware
- [ ] Rate limiting
- [ ] Cost tracking
- [ ] WebSocket streaming
- [ ] Checkpoint approval UI

### Low Priority

- [ ] Admin dashboard
- [ ] Metrics/monitoring
- [ ] Docker compose setup
- [ ] Kubernetes manifests
- [ ] Performance benchmarks

## Communication

- **Issues**: For bug reports and feature requests
- **Discussions**: For questions and ideas
- **Pull Requests**: For code contributions

## License

By contributing, you agree that your contributions will be licensed under the same license as the project (MIT License).

## Questions?

Feel free to open an issue for any questions about contributing!
