# Dynamic Configuration

The agent now supports dynamic configuration reloading, allowing configuration changes to be applied without restarting the server.

## Features

### Configuration Management
- **File Watching**: Automatically detects changes to configuration files
- **Thread-Safe**: Concurrent access to configuration is properly synchronized
- **Per-Run Snapshots**: Each run captures its configuration at creation time
- **Validation**: Configuration changes are validated before applying

### CLI Flags
The agent now uses Viper for advanced configuration management:

```bash
# Basic flags
./agentd --config=path/to/agent.yaml --mcp-config=path/to/mcp.yaml --addr=:8080

# Environment variables (prefixed with AGENT_)
export AGENT_CONFIG=path/to/agent.yaml
export AGENT_ADDR=:9000
./agentd

# Disable config watching
./agentd --watch-config=false
```

### Environment Variables
All CLI flags can be set via environment variables with the `AGENT_` prefix:
- `AGENT_CONFIG` - Agent configuration file path
- `AGENT_MCP_CONFIG` - MCP configuration file path  
- `AGENT_ADDR` - Server address
- `AGENT_WATCH_CONFIG` - Enable/disable config watching (true/false)

## Configuration Reloading Behavior

### When Configuration Changes
1. File system watcher detects changes to config files
2. New configuration is loaded and validated
3. If validation passes, configuration is atomically updated
4. Existing runs continue with their original configuration
5. **New runs use the updated configuration**

### Important Notes
- **Existing runs are not affected** by configuration changes
- Only new runs created after the config change will use updated settings
- Invalid configuration changes are rejected and logged as errors
- The system continues with the previous valid configuration if reload fails

## Monitoring Configuration

### Config Endpoint
The agent exposes a `/config` endpoint showing current configuration state:

```bash
curl http://localhost:8080/config
```

Response includes:
- Current profile name and version
- Last reload timestamp
- Key configuration values (temperature, max_tool_calls, etc.)

### Logging
Configuration changes are logged with details about:
- Which configuration file changed
- Timestamp of reload
- Any validation errors

## Implementation Details

### ConfigManager
- Thread-safe configuration access using RWMutex
- File system watching with debouncing (100ms) for rapid changes
- Graceful cleanup of watchers on shutdown
- Validation before applying configuration changes

### Runtime Integration
- Each RunContext stores a snapshot of configuration at creation time
- Runtime uses ConfigManager instead of static configuration
- No impact on performance - configuration access is optimized

## Example Usage

```bash
# Start agent with config watching
./agentd --config=configs/agents/default.yaml

# In another terminal, modify the config file
vim configs/agents/default.yaml

# Changes are automatically detected and applied to new runs
# Check current config via HTTP endpoint
curl http://localhost:8080/config | jq
```

This enables zero-downtime configuration updates for production deployments.
