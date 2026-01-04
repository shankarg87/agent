package runtime

import (
	"fmt"
	"testing"

	"github.com/shankarg87/agent/internal/config"
	"github.com/shankarg87/agent/internal/mcp"
	"github.com/shankarg87/agent/internal/provider"
)

func TestApprovalRequirementChecks(t *testing.T) {
	mcpRegistry := mcp.NewRegistry()

	tests := []struct {
		name           string
		toolName       string
		arguments      map[string]any
		toolConfig     *config.ToolConfig
		expectApproval bool
		description    string
	}{
		{
			name:      "always_approval_required",
			toolName:  "any_tool",
			arguments: map[string]any{},
			toolConfig: &config.ToolConfig{
				RequiresApproval: config.ApprovalRequirement{
					Always: true,
				},
			},
			expectApproval: true,
			description:    "Tool with always approval should require consent",
		},
		{
			name:      "conditional_approval_matches",
			toolName:  "write_database",
			arguments: map[string]any{},
			toolConfig: &config.ToolConfig{
				RequiresApproval: config.ApprovalRequirement{
					Conditional: []string{".*write.*"},
				},
			},
			expectApproval: true,
			description:    "Tool matching conditional pattern should require consent",
		},
		{
			name:     "dangerous_argument_triggers_approval",
			toolName: "shell_command",
			arguments: map[string]any{
				"command": "sudo rm -rf /important",
			},
			toolConfig:     &config.ToolConfig{},
			expectApproval: true,
			description:    "Tool with dangerous arguments should require consent",
		},
		{
			name:     "safe_tool_no_approval",
			toolName: "read_config",
			arguments: map[string]any{
				"path": "/etc/config.json",
			},
			toolConfig:     &config.ToolConfig{},
			expectApproval: false,
			description:    "Safe tool should not require consent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requiresConsent, reason := mcpRegistry.RequiresUserConsent(
				tt.toolName, tt.arguments, tt.toolConfig)

			if requiresConsent != tt.expectApproval {
				t.Errorf("%s: Expected approval requirement: %v, got: %v (reason: %s)",
					tt.description, tt.expectApproval, requiresConsent, reason)
			}

			if tt.expectApproval && reason == "" {
				t.Errorf("%s: Expected reason for approval requirement, got empty string",
					tt.description)
			}
		})
	}
}

func TestToolApprovalFlowWithMockRunContext(t *testing.T) {
	// This test validates the approval logic without needing a full runtime
	mcpRegistry := mcp.NewRegistry()

	// Test configuration requiring approval
	toolConfig := &config.ToolConfig{
		ServerName: "test-server",
		RequiresApproval: config.ApprovalRequirement{
			Always: true, // Require approval for all tools
		},
	}

	// Create tool call that should require approval
	toolCall := provider.ToolCall{
		ID:   "tool-call-123",
		Type: "function",
		Function: provider.FunctionCall{
			Name:      "dangerous_operation",
			Arguments: `{"action": "delete_files"}`,
		},
	}

	toolName := toolCall.Function.Name
	args := map[string]any{"action": "delete_files"}

	t.Run("approval_always_required", func(t *testing.T) {
		requiresConsent, reason := mcpRegistry.RequiresUserConsent(toolName, args, toolConfig)

		if !requiresConsent {
			t.Errorf("Expected tool to require consent due to 'always' setting")
		}

		expectedReason := "always requires user consent"
		if reason != fmt.Sprintf("Tool '%s' %s", toolName, expectedReason) {
			t.Errorf("Expected reason to contain '%s', got: %s", expectedReason, reason)
		}
	})

	t.Run("conditional_approval_pattern_matching", func(t *testing.T) {
		conditionalConfig := &config.ToolConfig{
			ServerName: "test-server",
			RequiresApproval: config.ApprovalRequirement{
				Always:      false,
				Conditional: []string{".*dangerous.*", ".*delete.*"},
			},
		}

		requiresConsent, reason := mcpRegistry.RequiresUserConsent(toolName, args, conditionalConfig)

		if !requiresConsent {
			t.Errorf("Expected tool to require consent due to pattern matching 'dangerous'")
		}

		expectedPattern := ".*dangerous.*"
		if reason != fmt.Sprintf("Tool '%s' requires consent due to pattern '%s'", toolName, expectedPattern) {
			t.Errorf("Expected reason to mention pattern '%s', got: %s", expectedPattern, reason)
		}
	})

	t.Run("dangerous_argument_detection", func(t *testing.T) {
		safeConfig := &config.ToolConfig{
			ServerName: "test-server",
			RequiresApproval: config.ApprovalRequirement{
				Always: false,
			},
		}

		dangerousArgs := map[string]any{
			"command": "sudo rm -rf /critical/data",
		}

		requiresConsent, reason := mcpRegistry.RequiresUserConsent("shell_exec", dangerousArgs, safeConfig)

		if !requiresConsent {
			t.Errorf("Expected tool to require consent due to dangerous arguments")
		}

		expectedText := "dangerous operations"
		if reason != fmt.Sprintf("Tool '%s' contains potentially %s", "shell_exec", expectedText) {
			t.Errorf("Expected reason to mention '%s', got: %s", expectedText, reason)
		}
	})

	t.Run("auto_approve_daemon_mode_simulation", func(t *testing.T) {
		// Simulate daemon mode with auto-approve
		daemonConfig := &config.ToolConfig{
			ServerName: "test-server",
			RequiresApproval: config.ApprovalRequirement{
				Always: true, // Would normally require approval
			},
		}

		// In daemon mode with auto-approve, RequiresUserConsent would still return true,
		// but the runtime logic would auto-approve
		requiresConsent, _ := mcpRegistry.RequiresUserConsent(toolName, args, daemonConfig)

		if !requiresConsent {
			t.Errorf("Expected RequiresUserConsent to return true even in daemon mode (approval logic is in runtime)")
		}

		// The runtime would check autoApproveInDaemon and runMode and bypass the pause
		t.Logf("✓ RequiresUserConsent correctly identifies approval requirement regardless of runtime mode")
	})

	t.Run("write_operation_pattern_detection", func(t *testing.T) {
		writeConfig := &config.ToolConfig{
			ServerName: "test-server",
		}

		// Test various write operation patterns
		writeTools := []string{
			"write_file", "create_document", "update_database",
			"modify_config", "save_data", "file_operation",
		}

		for _, tool := range writeTools {
			requiresConsent, reason := mcpRegistry.RequiresUserConsent(tool, map[string]any{}, writeConfig)

			if !requiresConsent {
				t.Errorf("Expected '%s' to require consent due to write operation pattern", tool)
			}

			if reason == "" {
				t.Errorf("Expected non-empty reason for '%s'", tool)
			}

			t.Logf("✓ Tool '%s' correctly identified as requiring approval: %s", tool, reason)
		}
	})
}
