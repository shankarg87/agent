package runtime

import (
	"testing"

	"github.com/shankarg87/agent/internal/config"
	"github.com/shankarg87/agent/internal/logging"
	"github.com/shankarg87/agent/internal/mcp"
	"github.com/shankarg87/agent/internal/store"
)

// Test that ListToolsFiltered respects allowlist/denylist rules.
func TestMCPRegistryListToolsFiltered(t *testing.T) {
	registry := mcp.NewRegistry()

	// Inject a mock server with tools
	mockServer := &mcp.MCPServer{
		Name: "test-server",
		Tools: map[string]*mcp.Tool{
			"safe_read":        {Name: "safe_read", ServerName: "test-server", Description: "Safe read operation"},
			"dangerous_delete": {Name: "dangerous_delete", ServerName: "test-server", Description: "DANGEROUS: Delete files"},
			"sudo_exec":        {Name: "sudo_exec", ServerName: "test-server", Description: "Execute with sudo privileges"},
		},
	}

	registry.SetServer("test-server", mockServer)

	tests := []struct {
		name        string
		cfg         []config.ToolConfig
		expectNames []string
	}{
		{
			name:        "no_filtering",
			cfg:         []config.ToolConfig{},
			expectNames: []string{"safe_read", "dangerous_delete", "sudo_exec"},
		},
		{
			name: "allowlist_only",
			cfg: []config.ToolConfig{
				{ServerName: "test-server", Allowlist: []string{"safe_.*"}},
			},
			expectNames: []string{"safe_read"},
		},
		{
			name: "denylist_blocks",
			cfg: []config.ToolConfig{
				{ServerName: "test-server", Denylist: []string{".*delete.*", ".*sudo.*"}},
			},
			expectNames: []string{"safe_read"},
		},
		{
			name: "allow_all_but_deny",
			cfg: []config.ToolConfig{
				{ServerName: "test-server", Allowlist: []string{".*"}, Denylist: []string{".*delete.*"}},
			},
			expectNames: []string{"safe_read", "sudo_exec"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := registry.ListToolsFiltered(tt.cfg)
			got := make(map[string]bool)
			for _, f := range filtered {
				got[f.Name] = true
			}

			if len(got) != len(tt.expectNames) {
				t.Fatalf("expected %d tools, got %d", len(tt.expectNames), len(got))
			}

			for _, name := range tt.expectNames {
				if !got[name] {
					t.Fatalf("expected tool %s in filtered list", name)
				}
			}
		})
	}
}

// Test that the runtime's buildProviderTools uses the registry filtering
func TestRuntimeBuildProviderToolsFiltering(t *testing.T) {
	// Create registry and inject mock server
	mcpRegistry := mcp.NewRegistry()
	mockServer := &mcp.MCPServer{
		Name: "test-server",
		Tools: map[string]*mcp.Tool{
			"safe_read":        {Name: "safe_read", ServerName: "test-server", Description: "Safe read operation", InputSchema: map[string]any{"type": "object"}},
			"dangerous_delete": {Name: "dangerous_delete", ServerName: "test-server", Description: "DANGEROUS: Delete files", InputSchema: map[string]any{"type": "object"}},
			"sudo_exec":        {Name: "sudo_exec", ServerName: "test-server", Description: "Execute with sudo privileges", InputSchema: map[string]any{"type": "object"}},
		},
	}
	mcpRegistry.SetServer("test-server", mockServer)

	// Minimal runtime that needs mcpRegistry and logger
	logger := logging.VerboseLogger("runtime")
	rt := &Runtime{mcpRegistry: mcpRegistry, logger: logger}

	agentConfig := &config.AgentConfig{
		Tools: []config.ToolConfig{
			{ServerName: "test-server", Allowlist: []string{"safe_.*", "read_.*"}, Denylist: []string{".*delete.*", ".*sudo.*"}},
		},
	}

	runCtx := &RunContext{
		Run:    &store.Run{ID: "r1"},
		Config: agentConfig,
	}

	tools := rt.buildProviderTools(runCtx)

	// Expect only safe_read to be present
	found := map[string]bool{}
	for _, t := range tools {
		found[t.Function.Name] = true
	}

	if !found["safe_read"] {
		t.Fatalf("expected safe_read to be included")
	}
	if found["dangerous_delete"] || found["sudo_exec"] {
		t.Fatalf("dangerous tools should not be exposed to LLM")
	}
}
