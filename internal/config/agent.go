package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// AgentConfig represents a complete agent profile configuration
type AgentConfig struct {
	// Identity
	ProfileName    string   `yaml:"profile_name"`
	ProfileVersion string   `yaml:"profile_version"`
	Description    string   `yaml:"description"`
	Labels         []string `yaml:"labels"`

	// Model routing
	PrimaryModel    ModelConfig   `yaml:"primary_model"`
	FallbackModels  []ModelConfig `yaml:"fallback_models,omitempty"`
	RoutingStrategy string        `yaml:"routing_strategy"` // single, fallback, cost_aware, latency_aware
	MaxContextTokens int          `yaml:"max_context_tokens,omitempty"`
	MaxOutputTokens  int          `yaml:"max_output_tokens,omitempty"`
	Temperature      float64      `yaml:"temperature,omitempty"`
	TopP             float64      `yaml:"top_p,omitempty"`

	// Prompting
	SystemPrompt     string                    `yaml:"system_prompt"`
	PromptTemplates  PromptTemplates           `yaml:"prompt_templates,omitempty"`
	FewShotExamples  []string                  `yaml:"few_shot_examples,omitempty"`
	OutputStyle      string                    `yaml:"output_style"` // concise, verbose, json

	// Tools/MCP (references to MCP server configs)
	Tools []ToolConfig `yaml:"tools,omitempty"`

	// Approval & checkpoints
	ApprovalMode     string           `yaml:"approval_mode"` // never, always, policy
	ApprovalPolicies ApprovalPolicies `yaml:"approval_policies,omitempty"`
	AutoApproveInDaemon bool          `yaml:"auto_approve_in_daemon"`

	// Budgets & limits
	MaxToolCalls       int     `yaml:"max_tool_calls,omitempty"`
	MaxRunTimeSeconds  int     `yaml:"max_run_time_seconds,omitempty"`
	MaxCostUSD         float64 `yaml:"max_cost_usd,omitempty"`
	MaxFailuresPerRun  int     `yaml:"max_failures_per_run,omitempty"`

	// Memory
	MemoryEnabled  bool   `yaml:"memory_enabled"`
	MemoryProvider string `yaml:"memory_provider,omitempty"`
	WritePolicy    string `yaml:"write_policy,omitempty"` // never, explicit, auto
	ReadPolicy     string `yaml:"read_policy,omitempty"`  // always, on_demand, by_classifier
	RetentionDays  int    `yaml:"retention_days,omitempty"`

	// Logging & observability
	LogLevel          string `yaml:"log_level"` // debug, info, warn, error
	TraceEnabled      bool   `yaml:"trace_enabled"`
	MetricsEnabled    bool   `yaml:"metrics_enabled"`
	LogPayloadPolicy  string `yaml:"log_payload_policy"` // full, redacted, hashes_only

	// Reliability
	IdempotencyKeys bool `yaml:"idempotency_keys"`
	ResumeOnRestart bool `yaml:"resume_on_restart"`
}

type ModelConfig struct {
	Provider string            `yaml:"provider"` // anthropic, openai, gemini, ollama
	Model    string            `yaml:"model"`
	Endpoint string            `yaml:"endpoint,omitempty"` // for custom endpoints
	APIKey   string            `yaml:"api_key,omitempty"`  // can also use env vars
	Params   map[string]any    `yaml:"params,omitempty"`   // provider-specific params
}

type PromptTemplates struct {
	InteractivePreamble string `yaml:"interactive_preamble,omitempty"`
	AutonomousPreamble  string `yaml:"autonomous_preamble,omitempty"`
	ToolUsePreamble     string `yaml:"tool_use_preamble,omitempty"`
	CheckpointPreamble  string `yaml:"checkpoint_preamble,omitempty"`
}

type ToolConfig struct {
	ServerName        string                 `yaml:"server_name"`        // reference to MCP server
	Capabilities      []string               `yaml:"capabilities,omitempty"`
	Timeout           time.Duration          `yaml:"timeout,omitempty"`
	Retries           int                    `yaml:"retries,omitempty"`
	ConcurrencyLimit  int                    `yaml:"concurrency_limit,omitempty"`
	Allowlist         []string               `yaml:"allowlist,omitempty"`
	Denylist          []string               `yaml:"denylist,omitempty"`
	RequiresApproval  ApprovalRequirement    `yaml:"requires_approval,omitempty"`
	Redaction         RedactionConfig        `yaml:"redaction,omitempty"`
}

type ApprovalRequirement struct {
	Always      bool     `yaml:"always"`
	Conditional []string `yaml:"conditional,omitempty"` // regex patterns
}

type RedactionConfig struct {
	Arguments []string `yaml:"arguments,omitempty"` // argument names to redact
	Outputs   bool     `yaml:"outputs,omitempty"`   // redact all outputs
}

type ApprovalPolicies struct {
	WriteOps        bool `yaml:"write_ops"`
	DangerousOps    bool `yaml:"dangerous_ops"`
	BudgetExceeded  bool `yaml:"budget_exceeded"`
}

// LoadAgentConfig loads an agent configuration from a YAML file
func LoadAgentConfig(path string) (*AgentConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg AgentConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Set defaults
	if cfg.RoutingStrategy == "" {
		cfg.RoutingStrategy = "single"
	}
	if cfg.ApprovalMode == "" {
		cfg.ApprovalMode = "policy"
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}
	if cfg.OutputStyle == "" {
		cfg.OutputStyle = "concise"
	}
	if cfg.MaxToolCalls == 0 {
		cfg.MaxToolCalls = 100
	}
	if cfg.MaxRunTimeSeconds == 0 {
		cfg.MaxRunTimeSeconds = 300 // 5 minutes
	}
	if cfg.MaxFailuresPerRun == 0 {
		cfg.MaxFailuresPerRun = 3
	}

	return &cfg, nil
}
