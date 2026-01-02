package mcp

import (
	"context"
	"testing"
	"time"

	"github.com/shankarg87/agent/internal/config"
)

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()
	assertNotNil(t, registry)
	assertNotNil(t, registry.servers)
	assertNotNil(t, registry.logger)
	assertEqual(t, 0, len(registry.servers))
}

func TestRegistry_LoadServers_EmptyConfig(t *testing.T) {
	registry := NewRegistry()
	ctx := context.Background()

	cfg := &config.MCPConfig{
		Servers: []config.MCPServerConfig{},
	}

	err := registry.LoadServers(ctx, cfg)
	assertNoError(t, err)
	assertEqual(t, 0, len(registry.servers))
}

func TestMCPServer_Structure(t *testing.T) {
	server := &MCPServer{
		Name: "test-server",
		Config: config.MCPServerConfig{
			Name:       "test-server",
			Transport:  "stdio",
			Endpoint:   "/usr/bin/test",
			Args:       []string{"arg1", "arg2"},
			Env:        map[string]string{"VAR1": "value1"},
			Timeout:    30 * time.Second,
			RetryMax:   3,
			RetryDelay: 1 * time.Second,
		},
		Client: nil, // Would be set by actual connection
		Tools: map[string]*Tool{
			"test-tool": {
				Name:        "test-tool",
				Description: "A test tool",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"input": map[string]any{"type": "string"},
					},
				},
				ServerName: "test-server",
			},
		},
	}

	assertEqual(t, "test-server", server.Name)
	assertEqual(t, "stdio", server.Config.Transport)
	assertEqual(t, "/usr/bin/test", server.Config.Endpoint)
	assertEqual(t, 2, len(server.Config.Args))
	assertEqual(t, "arg1", server.Config.Args[0])
	assertEqual(t, "arg2", server.Config.Args[1])
	assertEqual(t, 1, len(server.Config.Env))
	assertEqual(t, "value1", server.Config.Env["VAR1"])
	assertEqual(t, 30*time.Second, server.Config.Timeout)
	assertEqual(t, 3, server.Config.RetryMax)
	assertEqual(t, 1*time.Second, server.Config.RetryDelay)

	assertEqual(t, 1, len(server.Tools))
	tool := server.Tools["test-tool"]
	assertNotNil(t, tool)
	assertEqual(t, "test-tool", tool.Name)
	assertEqual(t, "A test tool", tool.Description)
	assertEqual(t, "test-server", tool.ServerName)
	assertNotNil(t, tool.InputSchema)
	assertEqual(t, "object", tool.InputSchema["type"])
}

func TestTool_Structure(t *testing.T) {
	tool := &Tool{
		Name:        "echo_tool",
		Description: "Echoes input back",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{
					"type":        "string",
					"description": "Message to echo",
				},
			},
			"required": []string{"message"},
		},
		ServerName: "echo-server",
	}

	assertEqual(t, "echo_tool", tool.Name)
	assertEqual(t, "Echoes input back", tool.Description)
	assertEqual(t, "echo-server", tool.ServerName)

	assertEqual(t, "object", tool.InputSchema["type"])
	properties := tool.InputSchema["properties"].(map[string]any)
	assertNotNil(t, properties)

	message := properties["message"].(map[string]any)
	assertEqual(t, "string", message["type"])
	assertEqual(t, "Message to echo", message["description"])

	required := tool.InputSchema["required"].([]string)
	assertEqual(t, 1, len(required))
	assertEqual(t, "message", required[0])
}

func TestToolResult_Structure(t *testing.T) {
	// Test successful tool result
	successResult := &ToolResult{
		Content: []ContentBlock{
			{
				Type: "text",
				Text: "Tool executed successfully",
			},
			{
				Type: "resource",
				Data: map[string]any{"result": "success"},
			},
		},
		IsError: false,
	}

	assertEqual(t, false, successResult.IsError)
	assertEqual(t, 2, len(successResult.Content))
	assertEqual(t, "text", successResult.Content[0].Type)
	assertEqual(t, "Tool executed successfully", successResult.Content[0].Text)
	assertEqual(t, "resource", successResult.Content[1].Type)
	assertNotNil(t, successResult.Content[1].Data)

	// Test error tool result
	errorResult := &ToolResult{
		Content: []ContentBlock{
			{
				Type: "text",
				Text: "Tool execution failed",
			},
		},
		IsError: true,
	}

	assertEqual(t, true, errorResult.IsError)
	assertEqual(t, 1, len(errorResult.Content))
	assertEqual(t, "text", errorResult.Content[0].Type)
	assertEqual(t, "Tool execution failed", errorResult.Content[0].Text)
}

func TestContentBlock_Types(t *testing.T) {
	// Text content block
	textBlock := ContentBlock{
		Type: "text",
		Text: "This is text content",
	}
	assertEqual(t, "text", textBlock.Type)
	assertEqual(t, "This is text content", textBlock.Text)
	assertNil(t, textBlock.Data)

	// Data content block
	dataBlock := ContentBlock{
		Type: "resource",
		Data: map[string]any{
			"uri":  "file:///path/to/file.txt",
			"mime": "text/plain",
		},
	}
	assertEqual(t, "resource", dataBlock.Type)
	assertEqual(t, "", dataBlock.Text)
	assertNotNil(t, dataBlock.Data)

	data := dataBlock.Data.(map[string]any)
	assertEqual(t, "file:///path/to/file.txt", data["uri"])
	assertEqual(t, "text/plain", data["mime"])
}

func TestRegistry_GetServers(t *testing.T) {
	registry := NewRegistry()

	// Test with empty registry
	registry.mu.RLock()
	servers := make(map[string]*MCPServer)
	for k, v := range registry.servers {
		servers[k] = v
	}
	registry.mu.RUnlock()
	assertEqual(t, 0, len(servers))

	// Add a mock server directly (for testing)
	mockServer := &MCPServer{
		Name:   "mock-server",
		Config: config.MCPServerConfig{Name: "mock-server"},
		Client: nil,
		Tools:  make(map[string]*Tool),
	}

	registry.mu.Lock()
	registry.servers["mock-server"] = mockServer
	registry.mu.Unlock()

	// Test with server present
	registry.mu.RLock()
	servers = make(map[string]*MCPServer)
	for k, v := range registry.servers {
		servers[k] = v
	}
	registry.mu.RUnlock()
	assertEqual(t, 1, len(servers))
	assertNotNil(t, servers["mock-server"])
	assertEqual(t, "mock-server", servers["mock-server"].Name)
}

func TestRegistry_UnsupportedTransport(t *testing.T) {
	registry := NewRegistry()
	ctx := context.Background()

	cfg := config.MCPServerConfig{
		Name:      "test-server",
		Transport: "http", // Unsupported transport
		Endpoint:  "http://localhost:8080",
	}

	err := registry.LoadServer(ctx, cfg)
	assertError(t, err)
	if err != nil && !contains(err.Error(), "only stdio transport is currently supported") {
		t.Errorf("Expected transport error, got: %v", err)
	}
}

// Test helper functions
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

func assertNotNil(t *testing.T, value any) {
	t.Helper()
	if value == nil {
		t.Fatal("Expected non-nil value, got nil")
	}
}

func assertNil(t *testing.T, value any) {
	t.Helper()
	if value != nil {
		t.Fatalf("Expected nil value, got %v", value)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			(len(s) > len(substr) &&
				(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
					func() bool {
						for i := 0; i <= len(s)-len(substr); i++ {
							if s[i:i+len(substr)] == substr {
								return true
							}
						}
						return false
					}())))
}
