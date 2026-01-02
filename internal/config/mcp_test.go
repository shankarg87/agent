package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadMCPConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-mcp.yaml")

	configContent := `servers:
  - name: echo
    transport: stdio
    endpoint: ./examples/mcp-servers/echo/main.go
    args: ["run"]
    env:
      DEBUG: "true"
    timeout: 30s
    retry_max: 5
    retry_delay: 2s
  - name: http-server
    transport: http
    endpoint: http://localhost:8080/mcp
    timeout: 60s
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	assertNoError(t, err)

	cfg, err := LoadMCPConfig(configPath)
	assertNoError(t, err)
	assertNotNil(t, cfg)

	assertEqual(t, 2, len(cfg.Servers))

	// Test first server (echo)
	echo := cfg.Servers[0]
	assertEqual(t, "echo", echo.Name)
	assertEqual(t, "stdio", echo.Transport)
	assertEqual(t, "./examples/mcp-servers/echo/main.go", echo.Endpoint)
	assertEqual(t, 1, len(echo.Args))
	assertEqual(t, "run", echo.Args[0])
	assertEqual(t, 1, len(echo.Env))
	assertEqual(t, "true", echo.Env["DEBUG"])
	assertEqual(t, 30*time.Second, echo.Timeout)
	assertEqual(t, 5, echo.RetryMax)
	assertEqual(t, 2*time.Second, echo.RetryDelay)

	// Test second server (http-server) with defaults
	http := cfg.Servers[1]
	assertEqual(t, "http-server", http.Name)
	assertEqual(t, "http", http.Transport)
	assertEqual(t, "http://localhost:8080/mcp", http.Endpoint)
	assertEqual(t, 60*time.Second, http.Timeout)
	assertEqual(t, 3, http.RetryMax)               // default
	assertEqual(t, 1*time.Second, http.RetryDelay) // default
}

func TestLoadMCPConfig_WithDefaults(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "minimal-mcp.yaml")

	configContent := `servers:
  - name: simple
    transport: stdio
    endpoint: /path/to/server
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	assertNoError(t, err)

	cfg, err := LoadMCPConfig(configPath)
	assertNoError(t, err)
	assertNotNil(t, cfg)

	assertEqual(t, 1, len(cfg.Servers))
	server := cfg.Servers[0]

	// Verify defaults are applied
	assertEqual(t, 30*time.Second, server.Timeout)
	assertEqual(t, 3, server.RetryMax)
	assertEqual(t, 1*time.Second, server.RetryDelay)
}

func TestLoadMCPConfig_InvalidFile(t *testing.T) {
	_, err := LoadMCPConfig("/non/existent/path.yaml")
	assertError(t, err)
}

func TestLoadMCPConfig_InvalidYAML(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "invalid.yaml")

	invalidYAML := `servers:
  - name: test
    invalid: [unclosed
`
	err := os.WriteFile(configPath, []byte(invalidYAML), 0644)
	assertNoError(t, err)

	_, err = LoadMCPConfig(configPath)
	assertError(t, err)
}

func TestMCPServerConfig_Structure(t *testing.T) {
	// Test server configuration with all fields
	server := MCPServerConfig{
		Name:       "test-server",
		Transport:  "stdio",
		Endpoint:   "/usr/bin/test",
		Args:       []string{"arg1", "arg2"},
		Env:        map[string]string{"VAR1": "value1", "VAR2": "value2"},
		Timeout:    45 * time.Second,
		RetryMax:   10,
		RetryDelay: 5 * time.Second,
	}

	assertEqual(t, "test-server", server.Name)
	assertEqual(t, "stdio", server.Transport)
	assertEqual(t, "/usr/bin/test", server.Endpoint)
	assertEqual(t, 2, len(server.Args))
	assertEqual(t, "arg1", server.Args[0])
	assertEqual(t, "arg2", server.Args[1])
	assertEqual(t, 2, len(server.Env))
	assertEqual(t, "value1", server.Env["VAR1"])
	assertEqual(t, "value2", server.Env["VAR2"])
	assertEqual(t, 45*time.Second, server.Timeout)
	assertEqual(t, 10, server.RetryMax)
	assertEqual(t, 5*time.Second, server.RetryDelay)
}

func TestMCPConfig_EmptyServers(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "empty-mcp.yaml")

	configContent := `servers: []`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	assertNoError(t, err)

	cfg, err := LoadMCPConfig(configPath)
	assertNoError(t, err)
	assertNotNil(t, cfg)
	assertEqual(t, 0, len(cfg.Servers))
}
