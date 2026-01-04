package mcp

import (
	"testing"

	"github.com/shankarg87/agent/internal/config"
)

func TestListToolsFiltered(t *testing.T) {
	registry := NewRegistry()

	// Create mock server with tools
	mockServer := &MCPServer{
		Name: "test-server",
		Tools: map[string]*Tool{
			"safe_echo": {
				Name:        "safe_echo",
				Description: "Safe echo tool",
				ServerName:  "test-server",
			},
			"dangerous_delete": {
				Name:        "dangerous_delete",
				Description: "Dangerous delete tool",
				ServerName:  "test-server",
			},
			"write_file": {
				Name:        "write_file",
				Description: "Write file tool",
				ServerName:  "test-server",
			},
			"read_file": {
				Name:        "read_file",
				Description: "Read file tool",
				ServerName:  "test-server",
			},
		},
	}

	registry.mu.Lock()
	registry.servers["test-server"] = mockServer
	registry.mu.Unlock()

	tests := []struct {
		name            string
		toolConfigs     []config.ToolConfig
		expectedTools   []string
		unexpectedTools []string
		description     string
	}{
		{
			name: "no filtering - all tools included",
			toolConfigs: []config.ToolConfig{
				{
					ServerName: "test-server",
					// No allowlist or denylist
				},
			},
			expectedTools:   []string{"safe_echo", "dangerous_delete", "write_file", "read_file"},
			unexpectedTools: []string{},
			description:     "When no filtering is configured, all tools should be included",
		},
		{
			name: "allowlist filtering - only safe tools",
			toolConfigs: []config.ToolConfig{
				{
					ServerName: "test-server",
					Allowlist:  []string{"safe_.*", "read_.*"},
					Denylist:   []string{},
				},
			},
			expectedTools:   []string{"safe_echo", "read_file"},
			unexpectedTools: []string{"dangerous_delete", "write_file"},
			description:     "Only tools matching allowlist patterns should be included",
		},
		{
			name: "denylist filtering - block dangerous tools",
			toolConfigs: []config.ToolConfig{
				{
					ServerName: "test-server",
					Allowlist:  []string{}, // No allowlist restriction
					Denylist:   []string{".*delete.*", ".*dangerous.*"},
				},
			},
			expectedTools:   []string{"safe_echo", "write_file", "read_file"},
			unexpectedTools: []string{"dangerous_delete"},
			description:     "Tools matching denylist patterns should be excluded",
		},
		{
			name: "both allowlist and denylist - denylist takes precedence",
			toolConfigs: []config.ToolConfig{
				{
					ServerName: "test-server",
					Allowlist:  []string{".*"}, // Allow all
					Denylist:   []string{".*delete.*", ".*write.*"},
				},
			},
			expectedTools:   []string{"safe_echo", "read_file"},
			unexpectedTools: []string{"dangerous_delete", "write_file"},
			description:     "Denylist should take precedence over allowlist",
		},
		{
			name: "strict allowlist - only specific tools",
			toolConfigs: []config.ToolConfig{
				{
					ServerName: "test-server",
					Allowlist:  []string{"safe_echo", "read_file"},
					Denylist:   []string{},
				},
			},
			expectedTools:   []string{"safe_echo", "read_file"},
			unexpectedTools: []string{"dangerous_delete", "write_file"},
			description:     "Only exact matches to allowlist should be included",
		},
		{
			name: "no matching server config - include all tools",
			toolConfigs: []config.ToolConfig{
				{
					ServerName: "different-server",
					Allowlist:  []string{"only_this"},
					Denylist:   []string{".*"},
				},
			},
			expectedTools:   []string{"safe_echo", "dangerous_delete", "write_file", "read_file"},
			unexpectedTools: []string{},
			description:     "When no matching server config is found, all tools should be included",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filteredTools := registry.ListToolsFiltered(tt.toolConfigs)

			// Convert to map for easier checking
			toolNames := make(map[string]bool)
			for _, tool := range filteredTools {
				toolNames[tool.Name] = true
			}

			// Check expected tools are included
			for _, expectedTool := range tt.expectedTools {
				if !toolNames[expectedTool] {
					t.Errorf("%s: Expected tool '%s' to be included, but it was filtered out",
						tt.description, expectedTool)
				}
			}

			// Check unexpected tools are excluded
			for _, unexpectedTool := range tt.unexpectedTools {
				if toolNames[unexpectedTool] {
					t.Errorf("%s: Expected tool '%s' to be filtered out, but it was included",
						tt.description, unexpectedTool)
				}
			}

			t.Logf("%s: Got %d filtered tools from %d total tools",
				tt.description, len(filteredTools), len(mockServer.Tools))
		})
	}
}

func TestIsToolAllowed(t *testing.T) {
	registry := NewRegistry()

	tests := []struct {
		name       string
		toolName   string
		toolConfig *config.ToolConfig
		expected   bool
		reason     string
	}{
		{
			name:     "no restrictions - allow by default",
			toolName: "any_tool",
			toolConfig: &config.ToolConfig{
				Allowlist: []string{},
				Denylist:  []string{},
			},
			expected: true,
			reason:   "No allowlist or denylist should allow all tools",
		},
		{
			name:     "in allowlist - allow",
			toolName: "safe_echo",
			toolConfig: &config.ToolConfig{
				Allowlist: []string{"safe_.*", "read_.*"},
				Denylist:  []string{},
			},
			expected: true,
			reason:   "Tool matching allowlist pattern should be allowed",
		},
		{
			name:     "not in allowlist - deny",
			toolName: "dangerous_operation",
			toolConfig: &config.ToolConfig{
				Allowlist: []string{"safe_.*", "read_.*"},
				Denylist:  []string{},
			},
			expected: false,
			reason:   "Tool not matching allowlist pattern should be denied",
		},
		{
			name:     "in denylist - deny",
			toolName: "delete_everything",
			toolConfig: &config.ToolConfig{
				Allowlist: []string{},
				Denylist:  []string{".*delete.*", ".*destroy.*"},
			},
			expected: false,
			reason:   "Tool matching denylist pattern should be denied",
		},
		{
			name:     "in allowlist but also in denylist - deny (denylist wins)",
			toolName: "safe_delete",
			toolConfig: &config.ToolConfig{
				Allowlist: []string{"safe_.*"},
				Denylist:  []string{".*delete.*"},
			},
			expected: false,
			reason:   "Denylist should take precedence over allowlist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := registry.isToolAllowed(tt.toolName, tt.toolConfig)
			if result != tt.expected {
				t.Errorf("%s: Expected %v, got %v for tool '%s'",
					tt.reason, tt.expected, result, tt.toolName)
			}
		})
	}
}

func TestToolFilteringPreventsLLMAccess(t *testing.T) {
	registry := NewRegistry()

	// Create a mock server with dangerous tools
	mockServer := &MCPServer{
		Name: "dangerous-server",
		Tools: map[string]*Tool{
			"safe_echo": {
				Name:        "safe_echo",
				Description: "Safe echo operation",
				ServerName:  "dangerous-server",
			},
			"rm_rf": {
				Name:        "rm_rf",
				Description: "DANGEROUS: Delete everything",
				ServerName:  "dangerous-server",
			},
			"sudo_command": {
				Name:        "sudo_command",
				Description: "Execute commands with sudo",
				ServerName:  "dangerous-server",
			},
			"format_disk": {
				Name:        "format_disk",
				Description: "Format a disk drive",
				ServerName:  "dangerous-server",
			},
		},
	}

	registry.mu.Lock()
	registry.servers["dangerous-server"] = mockServer
	registry.mu.Unlock()

	// Configuration that should block dangerous tools
	securityConfig := []config.ToolConfig{
		{
			ServerName: "dangerous-server",
			Allowlist:  []string{"safe_.*", "echo.*"},
			Denylist: []string{
				".*rm.*",
				".*sudo.*",
				".*format.*",
				".*delete.*",
				".*destroy.*",
			},
		},
	}

	// Test that dangerous tools are filtered out
	filteredTools := registry.ListToolsFiltered(securityConfig)

	dangerousTools := []string{"rm_rf", "sudo_command", "format_disk"}
	safeTools := []string{"safe_echo"}

	toolMap := make(map[string]bool)
	for _, tool := range filteredTools {
		toolMap[tool.Name] = true
	}

	// Verify dangerous tools are NOT in the filtered list
	for _, dangerousTool := range dangerousTools {
		if toolMap[dangerousTool] {
			t.Errorf("SECURITY VIOLATION: Dangerous tool '%s' was not filtered out and would be accessible to LLM", dangerousTool)
		}
	}

	// Verify safe tools ARE in the filtered list
	for _, safeTool := range safeTools {
		if !toolMap[safeTool] {
			t.Errorf("Safe tool '%s' was incorrectly filtered out", safeTool)
		}
	}

	t.Logf("SUCCESS: Filtered %d dangerous tools out of %d total tools. Only %d safe tools exposed to LLM.",
		len(dangerousTools), len(mockServer.Tools), len(filteredTools))
}
