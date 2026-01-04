package mcp

import (
	"strings"
	"testing"

	"github.com/shankarg87/agent/internal/config"
)

func TestToolAuthorization(t *testing.T) {
	registry := NewRegistry()

	tests := []struct {
		name         string
		toolName     string
		arguments    map[string]any
		toolConfig   *config.ToolConfig
		expectError  bool
		errorPattern string
	}{
		{
			name:     "allowed tool passes",
			toolName: "echo",
			arguments: map[string]any{
				"message": "hello world",
			},
			toolConfig: &config.ToolConfig{
				Allowlist: []string{"echo", "uppercase"},
				Denylist:  []string{},
			},
			expectError: false,
		},
		{
			name:     "denied tool blocked",
			toolName: "delete_file",
			arguments: map[string]any{
				"path": "/tmp/test.txt",
			},
			toolConfig: &config.ToolConfig{
				Allowlist: []string{"echo"},
				Denylist:  []string{".*delete.*", ".*remove.*"},
			},
			expectError:  true,
			errorPattern: "denied by pattern",
		},
		{
			name:      "tool not in allowlist blocked",
			toolName:  "dangerous_operation",
			arguments: map[string]any{},
			toolConfig: &config.ToolConfig{
				Allowlist: []string{"echo", "uppercase"},
				Denylist:  []string{},
			},
			expectError:  true,
			errorPattern: "not in allowlist",
		},
		{
			name:     "tool requires approval",
			toolName: "create_file",
			arguments: map[string]any{
				"path": "/tmp/test.txt",
			},
			toolConfig: &config.ToolConfig{
				RequiresApproval: config.ApprovalRequirement{
					Always: true,
				},
			},
			expectError:  true,
			errorPattern: "requires explicit approval",
		},
		{
			name:     "conditional approval pattern matches",
			toolName: "write_file",
			arguments: map[string]any{
				"data": "test data",
			},
			toolConfig: &config.ToolConfig{
				RequiresApproval: config.ApprovalRequirement{
					Conditional: []string{".*write.*", ".*create.*"},
				},
			},
			expectError:  true,
			errorPattern: "requires approval due to pattern",
		},
		{
			name:     "dangerous operation in argument",
			toolName: "shell_exec",
			arguments: map[string]any{
				"command": "rm -rf /important/data",
			},
			toolConfig:   &config.ToolConfig{},
			expectError:  true,
			errorPattern: "dangerous operations",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := registry.validateToolAuthorization(tt.toolName, tt.arguments, tt.toolConfig)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorPattern != "" && !containsPattern(err.Error(), tt.errorPattern) {
					t.Errorf("Error message '%s' doesn't contain pattern '%s'", err.Error(), tt.errorPattern)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestRequiresUserConsent(t *testing.T) {
	registry := NewRegistry()

	tests := []struct {
		name           string
		toolName       string
		arguments      map[string]any
		toolConfig     *config.ToolConfig
		expectConsent  bool
		expectedReason string
	}{
		{
			name:     "safe tool doesn't require consent",
			toolName: "echo",
			arguments: map[string]any{
				"message": "hello",
			},
			toolConfig:    &config.ToolConfig{},
			expectConsent: false,
		},
		{
			name:      "always approval tool requires consent",
			toolName:  "critical_operation",
			arguments: map[string]any{},
			toolConfig: &config.ToolConfig{
				RequiresApproval: config.ApprovalRequirement{
					Always: true,
				},
			},
			expectConsent:  true,
			expectedReason: "always requires user consent",
		},
		{
			name:     "write operation requires consent",
			toolName: "write_database",
			arguments: map[string]any{
				"query": "UPDATE users SET active = false",
			},
			toolConfig:     &config.ToolConfig{},
			expectConsent:  true,
			expectedReason: "write operations",
		},
		{
			name:     "dangerous argument requires consent",
			toolName: "system_command",
			arguments: map[string]any{
				"cmd": "sudo rm -rf /",
			},
			toolConfig:     &config.ToolConfig{},
			expectConsent:  true,
			expectedReason: "dangerous operations",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requiresConsent, reason := registry.RequiresUserConsent(tt.toolName, tt.arguments, tt.toolConfig)

			if requiresConsent != tt.expectConsent {
				t.Errorf("Expected consent requirement: %v, got: %v", tt.expectConsent, requiresConsent)
			}

			if tt.expectConsent && tt.expectedReason != "" && !containsPattern(reason, tt.expectedReason) {
				t.Errorf("Expected reason to contain '%s', got: '%s'", tt.expectedReason, reason)
			}
		})
	}
}

func TestContainsDangerousOperations(t *testing.T) {
	registry := NewRegistry()

	tests := []struct {
		name        string
		toolName    string
		arguments   map[string]any
		expectTrue  bool
		description string
	}{
		{
			name:        "safe operation",
			toolName:    "echo",
			arguments:   map[string]any{"message": "hello world"},
			expectTrue:  false,
			description: "Simple echo should be safe",
		},
		{
			name:        "dangerous tool name",
			toolName:    "delete_all",
			arguments:   map[string]any{},
			expectTrue:  true,
			description: "Tool name contains 'delete'",
		},
		{
			name:     "dangerous argument",
			toolName: "shell",
			arguments: map[string]any{
				"command": "rm -rf /important/data",
			},
			expectTrue:  true,
			description: "Command contains 'rm -rf'",
		},
		{
			name:     "sudo in argument",
			toolName: "execute",
			arguments: map[string]any{
				"cmd": "sudo systemctl restart network",
			},
			expectTrue:  true,
			description: "Command contains 'sudo'",
		},
		{
			name:     "format command",
			toolName: "disk_utils",
			arguments: map[string]any{
				"operation": "format /dev/sdb1",
			},
			expectTrue:  true,
			description: "Format operation is dangerous",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := registry.containsDangerousOperations(tt.toolName, tt.arguments)
			if result != tt.expectTrue {
				t.Errorf("%s: expected %v, got %v", tt.description, tt.expectTrue, result)
			}
		})
	}
}

// containsPattern checks if text contains the expected pattern (case insensitive)
func containsPattern(text, pattern string) bool {
	return len(text) > 0 && len(pattern) > 0 &&
		(text == pattern ||
			findSubstring(strings.ToLower(text), strings.ToLower(pattern)))
}

func findSubstring(text, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(text) < len(substr) {
		return false
	}

	for i := 0; i <= len(text)-len(substr); i++ {
		if text[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
