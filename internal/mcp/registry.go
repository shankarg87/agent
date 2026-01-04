package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/shankarg87/agent/internal/config"
	"github.com/shankarg87/agent/internal/logging"
)

// Registry manages MCP server connections and tool invocations
type Registry struct {
	mu      sync.RWMutex
	servers map[string]*MCPServer
	logger  *logging.SimpleLogger
}

// MCPServer wraps an MCP client with metadata
type MCPServer struct {
	Name   string
	Config config.MCPServerConfig
	Client *client.Client
	Tools  map[string]*Tool
}

// Tool represents an MCP tool definition
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
	ServerName  string         `json:"server_name"`
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"is_error,omitempty"`
}

// ContentBlock represents a content block in a tool result
type ContentBlock struct {
	Type string `json:"type"` // text, resource, etc.
	Text string `json:"text,omitempty"`
	Data any    `json:"data,omitempty"`
}

// NewRegistry creates a new MCP server registry
func NewRegistry() *Registry {
	logger := logging.VerboseLogger("mcp")
	logger.Verbose("Creating new MCP registry")

	return &Registry{
		servers: make(map[string]*MCPServer),
		logger:  logger,
	}
}

// LoadServers loads and connects to all configured MCP servers
func (r *Registry) LoadServers(ctx context.Context, cfg *config.MCPConfig) error {
	r.logger.Info("Loading MCP servers", "server_count", len(cfg.Servers))

	for _, serverCfg := range cfg.Servers {
		r.logger.Verbose("Loading MCP server",
			"name", serverCfg.Name,
			"transport", serverCfg.Transport,
			"endpoint", serverCfg.Endpoint,
		)

		if err := r.LoadServer(ctx, serverCfg); err != nil {
			r.logger.Error("Failed to load MCP server",
				"name", serverCfg.Name,
				"error", err,
			)
			return fmt.Errorf("failed to load server %s: %w", serverCfg.Name, err)
		}

		r.logger.Info("MCP server loaded successfully", "name", serverCfg.Name)
	}

	r.logger.Info("All MCP servers loaded successfully", "total_servers", len(cfg.Servers))
	return nil
}

// LoadServer loads a single MCP server
func (r *Registry) LoadServer(ctx context.Context, cfg config.MCPServerConfig) error {
	start := time.Now()
	r.logger.Verbose("Starting MCP server initialization",
		"name", cfg.Name,
		"transport", cfg.Transport,
		"endpoint", cfg.Endpoint,
		"args", cfg.Args,
	)

	r.mu.Lock()
	defer r.mu.Unlock()

	if cfg.Transport != "stdio" {
		r.logger.Error("Unsupported transport type",
			"name", cfg.Name,
			"transport", cfg.Transport,
		)
		return fmt.Errorf("only stdio transport is currently supported")
	}

	// Convert env map to slice
	var envSlice []string
	for k, v := range cfg.Env {
		envSlice = append(envSlice, fmt.Sprintf("%s=%s", k, v))
	}

	r.logger.Verbose("Environment variables prepared",
		"name", cfg.Name,
		"env_count", len(envSlice),
	)

	// Create stdio client (automatically starts)
	r.logger.Verbose("Creating MCP client", "name", cfg.Name)
	mcpClient, err := client.NewStdioMCPClient(cfg.Endpoint, envSlice, cfg.Args...)
	if err != nil {
		r.logger.Error("Failed to create MCP client",
			"name", cfg.Name,
			"error", err,
		)
		return fmt.Errorf("failed to create MCP client: %w", err)
	}

	// Initialize the client
	r.logger.Verbose("Initializing MCP client", "name", cfg.Name)
	initReq := mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: "2024-11-05",
			Capabilities:    mcp.ClientCapabilities{},
			ClientInfo: mcp.Implementation{
				Name:    "agent",
				Version: "1.0.0",
			},
		},
	}

	_, err = mcpClient.Initialize(ctx, initReq)
	if err != nil {
		r.logger.Error("Failed to initialize MCP client",
			"name", cfg.Name,
			"error", err,
		)
		mcpClient.Close()
		return fmt.Errorf("failed to initialize MCP client: %w", err)
	}

	r.logger.Verbose("MCP client initialized successfully", "name", cfg.Name)

	// List available tools
	r.logger.Verbose("Listing available tools", "name", cfg.Name)
	toolsResp, err := mcpClient.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		r.logger.Error("Failed to list tools",
			"name", cfg.Name,
			"error", err,
		)
		mcpClient.Close()
		return fmt.Errorf("failed to list tools: %w", err)
	}

	r.logger.Verbose("Tools listed successfully",
		"name", cfg.Name,
		"tool_count", len(toolsResp.Tools),
	)

	// Build tool map
	tools := make(map[string]*Tool)
	for _, t := range toolsResp.Tools {
		// Convert InputSchema to map[string]any
		schemaBytes, _ := json.Marshal(t.InputSchema)
		var schemaMap map[string]any
		json.Unmarshal(schemaBytes, &schemaMap)

		tools[t.Name] = &Tool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: schemaMap,
			ServerName:  cfg.Name,
		}

		r.logger.Verbose("Tool registered",
			"server_name", cfg.Name,
			"tool_name", t.Name,
			"description", t.Description,
		)
	}

	r.servers[cfg.Name] = &MCPServer{
		Name:   cfg.Name,
		Config: cfg,
		Client: mcpClient,
		Tools:  tools,
	}

	r.logger.LogMCPConnection(cfg.Name, cfg.Transport, cfg.Endpoint, true)
	r.logger.LogPerformance("load_mcp_server", time.Since(start), map[string]interface{}{
		"server_name": cfg.Name,
		"tool_count":  len(tools),
	})

	return nil
}

// GetServer returns an MCP server by name
func (r *Registry) GetServer(name string) (*MCPServer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	server, ok := r.servers[name]
	if !ok {
		return nil, fmt.Errorf("server not found: %s", name)
	}

	return server, nil
}

// GetTool returns a tool by name, searching across all servers
func (r *Registry) GetTool(toolName string) (*Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, server := range r.servers {
		if tool, ok := server.Tools[toolName]; ok {
			return tool, nil
		}
	}

	return nil, fmt.Errorf("tool not found: %s", toolName)
}

// ListTools returns all available tools across all servers
func (r *Registry) ListTools() []*Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var tools []*Tool
	for _, server := range r.servers {
		for _, tool := range server.Tools {
			tools = append(tools, tool)
		}
	}

	return tools
}

// ListToolsFiltered returns tools filtered by agent configuration (allowlist/denylist)
func (r *Registry) ListToolsFiltered(toolConfigs []config.ToolConfig) []*Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var filteredTools []*Tool

	for _, server := range r.servers {
		// Find the tool config for this server
		var toolConfig *config.ToolConfig
		for i := range toolConfigs {
			if toolConfigs[i].ServerName == server.Name {
				toolConfig = &toolConfigs[i]
				break
			}
		}

		// If no config found, include all tools from this server
		if toolConfig == nil {
			for _, tool := range server.Tools {
				filteredTools = append(filteredTools, tool)
			}
			continue
		}

		// Filter tools based on allowlist/denylist
		for _, tool := range server.Tools {
			if r.isToolAllowed(tool.Name, toolConfig) {
				filteredTools = append(filteredTools, tool)
				r.logger.Verbose("Tool included in filtered list",
					"tool", tool.Name,
					"server", server.Name,
				)
			} else {
				r.logger.Info("Tool filtered out from LLM schema",
					"tool", tool.Name,
					"server", server.Name,
					"reason", "denied by allowlist/denylist",
				)
			}
		}
	}

	return filteredTools
}

// isToolAllowed checks if a tool is allowed based on allowlist/denylist configuration
func (r *Registry) isToolAllowed(toolName string, toolConfig *config.ToolConfig) bool {
	// Check denylist first (takes precedence)
	for _, denied := range toolConfig.Denylist {
		if matched, _ := regexp.MatchString(denied, toolName); matched {
			r.logger.Verbose("Tool denied by denylist pattern",
				"tool", toolName,
				"pattern", denied,
			)
			return false
		}
	}

	// If allowlist is specified, tool must match at least one pattern
	if len(toolConfig.Allowlist) > 0 {
		for _, pattern := range toolConfig.Allowlist {
			if matched, _ := regexp.MatchString(pattern, toolName); matched {
				r.logger.Verbose("Tool allowed by allowlist pattern",
					"tool", toolName,
					"pattern", pattern,
				)
				return true
			}
		}
		// Tool doesn't match any allowlist pattern
		r.logger.Verbose("Tool not in allowlist",
			"tool", toolName,
		)
		return false
	}

	// No allowlist specified and not in denylist - allow by default
	return true
}

// CallTool executes a tool by name with safety checks
func (r *Registry) CallTool(ctx context.Context, toolName string, arguments map[string]any, toolConfig *config.ToolConfig) (*ToolResult, error) {
	tool, err := r.GetTool(toolName)
	if err != nil {
		return nil, err
	}

	server, err := r.GetServer(tool.ServerName)
	if err != nil {
		return nil, err
	}

	// Apply tool authorization and safety checks
	if toolConfig != nil {
		if err := r.validateToolAuthorization(toolName, arguments, toolConfig); err != nil {
			r.logger.Warn("Tool authorization failed",
				"tool", toolName,
				"error", err,
			)
			return nil, fmt.Errorf("tool authorization failed: %w", err)
		}
	}

	r.logger.Verbose("Executing tool",
		"tool", toolName,
		"server", tool.ServerName,
		"args_count", len(arguments),
	)

	// Execute the tool
	result, err := server.Client.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      toolName,
			Arguments: arguments,
		},
	})
	if err != nil {
		r.logger.Error("Tool execution failed",
			"tool", toolName,
			"error", err,
		)
		return nil, fmt.Errorf("tool execution failed: %w", err)
	}

	// Convert result to our format
	toolResult := &ToolResult{
		Content: make([]ContentBlock, len(result.Content)),
		IsError: result.IsError,
	}

	for i, content := range result.Content {
		// Extract text from content using helper function
		text := mcp.GetTextFromContent(content)

		toolResult.Content[i] = ContentBlock{
			Type: "text",
			Text: text,
		}
	}

	r.logger.Verbose("Tool executed successfully",
		"tool", toolName,
		"is_error", toolResult.IsError,
		"content_blocks", len(toolResult.Content),
	)

	return toolResult, nil
}

// Close closes all MCP server connections
func (r *Registry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var errs []error
	for name, server := range r.servers {
		if err := server.Client.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close server %s: %w", name, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing servers: %v", errs)
	}

	return nil
}

// SetServer sets or replaces an MCP server in the registry. This is a small
// helper used by tests to inject mock servers without going through the full
// LoadServer flow.
func (r *Registry) SetServer(name string, server *MCPServer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.servers == nil {
		r.servers = make(map[string]*MCPServer)
	}
	r.servers[name] = server
}

// validateToolAuthorization checks if a tool call is authorized based on configuration
func (r *Registry) validateToolAuthorization(toolName string, arguments map[string]any, toolConfig *config.ToolConfig) error {
	// Check if tool is in denylist
	for _, denied := range toolConfig.Denylist {
		if matched, _ := regexp.MatchString(denied, toolName); matched {
			return fmt.Errorf("tool %s is denied by pattern %s", toolName, denied)
		}
	}

	// Check if tool is in allowlist (if allowlist is specified, tool must match)
	if len(toolConfig.Allowlist) > 0 {
		allowed := false
		for _, pattern := range toolConfig.Allowlist {
			if matched, _ := regexp.MatchString(pattern, toolName); matched {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("tool %s is not in allowlist", toolName)
		}
	}

	// Check if tool requires approval
	if toolConfig.RequiresApproval.Always {
		return fmt.Errorf("tool %s requires explicit approval", toolName)
	}

	// Check conditional approval patterns
	for _, pattern := range toolConfig.RequiresApproval.Conditional {
		if matched, _ := regexp.MatchString(pattern, toolName); matched {
			return fmt.Errorf("tool %s requires approval due to pattern %s", toolName, pattern)
		}
	}

	// Check for dangerous operations in arguments
	if r.containsDangerousOperations(toolName, arguments) {
		return fmt.Errorf("tool %s contains potentially dangerous operations", toolName)
	}

	return nil
}

// containsDangerousOperations checks for patterns that might indicate dangerous operations
func (r *Registry) containsDangerousOperations(toolName string, arguments map[string]any) bool {
	// Common dangerous patterns
	dangerousPatterns := []string{
		`rm\s+-rf`,
		`sudo\s+`,
		`chmod\s+777`,
		`delete`,
		`drop\s+table`,
		`truncate`,
		`format`,
		`mkfs`,
		`dd\s+if=`,
		`>/dev/`,
		`curl.*\|.*sh`,
		`wget.*\|.*sh`,
	}

	// Check tool name for dangerous patterns
	toolLower := strings.ToLower(toolName)
	for _, pattern := range dangerousPatterns {
		if matched, _ := regexp.MatchString(pattern, toolLower); matched {
			r.logger.Warn("Dangerous pattern detected in tool name",
				"tool", toolName,
				"pattern", pattern,
			)
			return true
		}
	}

	// Check arguments for dangerous patterns
	for key, value := range arguments {
		if str, ok := value.(string); ok {
			strLower := strings.ToLower(str)
			for _, pattern := range dangerousPatterns {
				if matched, _ := regexp.MatchString(pattern, strLower); matched {
					r.logger.Warn("Dangerous pattern detected in argument",
						"tool", toolName,
						"arg", key,
						"pattern", pattern,
					)
					return true
				}
			}
		}
	}

	return false
}

// RequiresUserConsent checks if a tool requires explicit user consent before execution
func (r *Registry) RequiresUserConsent(toolName string, arguments map[string]any, toolConfig *config.ToolConfig) (bool, string) {
	if toolConfig == nil {
		return false, ""
	}

	// Always requires consent
	if toolConfig.RequiresApproval.Always {
		return true, fmt.Sprintf("Tool '%s' always requires user consent", toolName)
	}

	// Check conditional patterns
	for _, pattern := range toolConfig.RequiresApproval.Conditional {
		if matched, _ := regexp.MatchString(pattern, toolName); matched {
			return true, fmt.Sprintf("Tool '%s' requires consent due to pattern '%s'", toolName, pattern)
		}
	}

	// Check for dangerous operations
	if r.containsDangerousOperations(toolName, arguments) {
		return true, fmt.Sprintf("Tool '%s' contains potentially dangerous operations", toolName)
	}

	// Check for write operations patterns
	writePatterns := []string{
		`.*write.*`,
		`.*create.*`,
		`.*delete.*`,
		`.*update.*`,
		`.*modify.*`,
		`.*save.*`,
		`.*file.*`,
		`.*exec.*`,
		`.*run.*`,
		`.*shell.*`,
		`.*command.*`,
	}

	for _, pattern := range writePatterns {
		if matched, _ := regexp.MatchString(pattern, strings.ToLower(toolName)); matched {
			return true, fmt.Sprintf("Tool '%s' appears to perform write operations", toolName)
		}
	}

	return false, ""
}
