.PHONY: all build clean run test help

# Default target
all: build

# Build all binaries
build: build-agent build-echo

# Build the agent daemon
build-agent:
	@echo "Building agent daemon..."
	@mkdir -p bin
	@go build -o bin/agentd ./cmd/agentd
	@echo "✓ Built bin/agentd"

# Build the echo MCP server
build-echo:
	@echo "Building echo MCP server..."
	@go build -o examples/mcp-servers/echo/echo-server ./examples/mcp-servers/echo
	@echo "✓ Built examples/mcp-servers/echo/echo-server"

# Run the agent daemon
run: build-agent
	@echo "Starting agent daemon on :8080..."
	@./bin/agentd

# Run with custom config
run-custom: build-agent
	@./bin/agentd --config $(CONFIG) --addr $(ADDR)

# Clean built binaries
clean:
	@echo "Cleaning binaries..."
	@rm -rf bin/
	@rm -f examples/mcp-servers/echo/echo-server
	@echo "✓ Cleaned"

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Run only e2e tests
test-e2e: build-echo
	@echo "Running E2E tests..."
	@go test -v ./test -timeout 30s

# Run only unit tests (excluding e2e)
test-unit:
	@echo "Running unit tests..."
	@go test -v ./... -short

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy
	@echo "✓ Dependencies ready"

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@echo "✓ Code formatted"

# Run linter
lint:
	@echo "Running linter..."
	@go vet ./...
	@echo "✓ Lint passed"

# Show help
help:
	@echo "Available targets:"
	@echo "  make build        - Build all binaries"
	@echo "  make build-agent  - Build agent daemon only"
	@echo "  make build-echo   - Build echo MCP server only"
	@echo "  make run          - Build and run agent daemon"
	@echo "  make clean        - Remove built binaries"
	@echo "  make test         - Run all tests"
	@echo "  make test-e2e     - Run E2E tests only"  
	@echo "  make test-unit    - Run unit tests only"
	@echo "  make deps         - Download dependencies"
	@echo "  make fmt          - Format code"
	@echo "  make lint         - Run linter"
	@echo "  make help         - Show this help"
