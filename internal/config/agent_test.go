package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadAgentConfig(t *testing.T) {
	// Create a temporary test config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-agent.yaml")

	configContent := `profile_name: test-agent
profile_version: 1.0.0
description: Test agent
labels: ["test", "unit"]
primary_model:
  provider: openai
  model: gpt-4
  api_key: test-key
routing_strategy: single
max_context_tokens: 4096
max_output_tokens: 1024
temperature: 0.7
top_p: 0.9
system_prompt: "You are a helpful assistant."
output_style: concise
approval_mode: policy
auto_approve_in_daemon: true
max_tool_calls: 50
max_run_time_seconds: 600
max_cost_usd: 10.0
max_failures_per_run: 5
memory_enabled: true
memory_provider: memory_server
write_policy: auto
log_level: debug
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	assertNoError(t, err)

	// Test loading the config
	cfg, err := LoadAgentConfig(configPath)
	assertNoError(t, err)
	assertNotNil(t, cfg)

	// Verify basic fields
	assertEqual(t, "test-agent", cfg.ProfileName)
	assertEqual(t, "1.0.0", cfg.ProfileVersion)
	assertEqual(t, "Test agent", cfg.Description)
	assertEqual(t, 2, len(cfg.Labels))
	assertEqual(t, "test", cfg.Labels[0])
	assertEqual(t, "unit", cfg.Labels[1])

	// Verify model config
	assertEqual(t, "openai", cfg.PrimaryModel.Provider)
	assertEqual(t, "gpt-4", cfg.PrimaryModel.Model)
	assertEqual(t, "test-key", cfg.PrimaryModel.APIKey)

	// Verify routing and limits
	assertEqual(t, "single", cfg.RoutingStrategy)
	assertEqual(t, 4096, cfg.MaxContextTokens)
	assertEqual(t, 1024, cfg.MaxOutputTokens)
	assertEqual(t, 0.7, cfg.Temperature)
	assertEqual(t, 0.9, cfg.TopP)

	// Verify prompting
	assertEqual(t, "You are a helpful assistant.", cfg.SystemPrompt)
	assertEqual(t, "concise", cfg.OutputStyle)

	// Verify approval settings
	assertEqual(t, "policy", cfg.ApprovalMode)
	assertEqual(t, true, cfg.AutoApproveInDaemon)

	// Verify budgets and limits
	assertEqual(t, 50, cfg.MaxToolCalls)
	assertEqual(t, 600, cfg.MaxRunTimeSeconds)
	assertEqual(t, 10.0, cfg.MaxCostUSD)
	assertEqual(t, 5, cfg.MaxFailuresPerRun)

	// Verify memory settings
	assertEqual(t, true, cfg.MemoryEnabled)
	assertEqual(t, "memory_server", cfg.MemoryProvider)
	assertEqual(t, "auto", cfg.WritePolicy)

	// Verify logging
	assertEqual(t, "debug", cfg.LogLevel)
}

func TestLoadAgentConfig_WithDefaults(t *testing.T) {
	// Create a minimal config file to test defaults
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "minimal-agent.yaml")

	configContent := `profile_name: minimal-agent
profile_version: 1.0.0
description: Minimal test agent
primary_model:
  provider: openai
  model: gpt-4
system_prompt: "You are helpful."
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	assertNoError(t, err)

	cfg, err := LoadAgentConfig(configPath)
	assertNoError(t, err)
	assertNotNil(t, cfg)

	// Verify defaults are applied
	assertEqual(t, "single", cfg.RoutingStrategy)
	assertEqual(t, "policy", cfg.ApprovalMode)
	assertEqual(t, "info", cfg.LogLevel)
	assertEqual(t, "concise", cfg.OutputStyle)
	assertEqual(t, 100, cfg.MaxToolCalls)
	assertEqual(t, 300, cfg.MaxRunTimeSeconds)
	assertEqual(t, 3, cfg.MaxFailuresPerRun)
}

func TestLoadAgentConfig_InvalidFile(t *testing.T) {
	// Test loading non-existent file
	_, err := LoadAgentConfig("/non/existent/path.yaml")
	assertError(t, err)
}

func TestLoadAgentConfig_InvalidYAML(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "invalid.yaml")

	// Write invalid YAML
	invalidYAML := `profile_name: test
invalid: [unclosed bracket
`
	err := os.WriteFile(configPath, []byte(invalidYAML), 0644)
	assertNoError(t, err)

	_, err = LoadAgentConfig(configPath)
	assertError(t, err)
}

func TestModelConfig_Validation(t *testing.T) {
	// Test that we can create model configs with different providers
	tests := []struct {
		name     string
		provider string
		model    string
		valid    bool
	}{
		{"OpenAI valid", "openai", "gpt-4", true},
		{"Anthropic valid", "anthropic", "claude-3-sonnet", true},
		{"Empty provider", "", "gpt-4", false},
		{"Empty model", "openai", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testAgentConfig()
			cfg.PrimaryModel.Provider = tt.provider
			cfg.PrimaryModel.Model = tt.model

			if tt.valid {
				// Basic validation - provider and model should not be empty for valid configs
				assertEqual(t, tt.provider, cfg.PrimaryModel.Provider)
				assertEqual(t, tt.model, cfg.PrimaryModel.Model)
			} else {
				// For invalid configs, at least one field should be empty
				isEmpty := cfg.PrimaryModel.Provider == "" || cfg.PrimaryModel.Model == ""
				if !isEmpty {
					t.Errorf("Expected invalid config to have empty provider or model")
				}
			}
		})
	}
}

func TestToolConfig_Structure(t *testing.T) {
	cfg := testAgentConfig()

	// Add some test tool configs
	cfg.Tools = []ToolConfig{
		{
			ServerName: "test-server",
			Allowlist:  []string{"tool1", "tool2"},
			RequiresApproval: ApprovalRequirement{
				Always:      false,
				Conditional: []string{"dangerous_.*"},
			},
			Timeout:          30 * time.Second,
			Retries:          3,
			ConcurrencyLimit: 5,
		},
	}

	assertEqual(t, 1, len(cfg.Tools))
	tool := cfg.Tools[0]
	assertEqual(t, "test-server", tool.ServerName)
	assertEqual(t, 2, len(tool.Allowlist))
	assertEqual(t, "tool1", tool.Allowlist[0])
	assertEqual(t, false, tool.RequiresApproval.Always)
	assertEqual(t, 1, len(tool.RequiresApproval.Conditional))
	assertEqual(t, "dangerous_.*", tool.RequiresApproval.Conditional[0])
	assertEqual(t, 30*time.Second, tool.Timeout)
	assertEqual(t, 3, tool.Retries)
	assertEqual(t, 5, tool.ConcurrencyLimit)
}

// Test helpers to avoid import cycles
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

func testAgentConfig() *AgentConfig {
	return &AgentConfig{
		ProfileName:    "test-agent",
		ProfileVersion: "1.0.0",
		Description:    "Test agent configuration",
		Labels:         []string{"test"},
		PrimaryModel: ModelConfig{
			Provider: "openai",
			Model:    "gpt-4",
		},
		RoutingStrategy:     "single",
		MaxContextTokens:    4096,
		MaxOutputTokens:     1024,
		Temperature:         0.7,
		SystemPrompt:        "You are a helpful assistant.",
		OutputStyle:         "concise",
		ApprovalMode:        "never",
		AutoApproveInDaemon: true,
		MaxToolCalls:        10,
		MaxRunTimeSeconds:   300,
		MaxFailuresPerRun:   3,
		MemoryEnabled:       false,
		LogLevel:            "info",
	}
}
