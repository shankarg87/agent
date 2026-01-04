# MCP Tool Security Framework

This document describes the comprehensive security framework implemented for MCP tool execution in the agent runtime.

## Overview

The agent implements multiple layers of security to protect against dangerous tool operations:

1. **Tool Authorization** - Allowlist/denylist patterns
2. **User Consent Requirements** - Explicit approval for risky operations
3. **Dangerous Operation Detection** - Pattern-based risk assessment
4. **Argument Redaction** - Sensitive data protection
5. **Output Filtering** - Result sanitization

## Security Components

### 1. Tool Authorization (`validateToolAuthorization`)

**Allowlist Control:**
- If an allowlist is specified, tools must match at least one pattern
- Uses regex patterns for flexible matching
- Empty allowlist means all tools are permitted (subject to other checks)

**Denylist Control:**
- Tools matching denylist patterns are explicitly blocked
- Takes precedence over allowlist
- Supports regex patterns for broad coverage

**Example Configuration:**
```yaml
tools:
  - server_name: "file-server"
    allowlist:
      - "read_.*"      # Allow all read operations
      - "list_.*"      # Allow listing operations
    denylist:
      - ".*delete.*"   # Block all delete operations
      - ".*format.*"   # Block disk formatting
```

### 2. User Consent Requirements (`RequiresUserConsent`)

**Always Require Approval:**
```yaml
requires_approval:
  always: true  # Tool always needs explicit user consent
```

**Conditional Approval:**
```yaml
requires_approval:
  conditional:
    - ".*write.*"    # Require approval for write operations
    - ".*network.*"  # Require approval for network operations
    - ".*exec.*"     # Require approval for execution operations
```

**Automatic Risk Detection:**
- Write operations: `create`, `write`, `modify`, `update`, `save`, etc.
- Execution operations: `exec`, `run`, `shell`, `command`
- Network operations: `http`, `curl`, `wget`, `download`
- File operations: Based on tool name patterns

### 3. Dangerous Operation Detection (`containsDangerousOperations`)

**Built-in Dangerous Patterns:**
- `rm -rf` - Recursive file deletion
- `sudo` - Privilege escalation
- `chmod 777` - Permissive file permissions
- `delete`, `drop table`, `truncate` - Data destruction
- `format`, `mkfs` - Disk formatting
- `dd if=` - Low-level disk operations
- `>/dev/` - Device file writes
- `curl|sh`, `wget|sh` - Remote code execution

**Pattern Matching:**
- Checks both tool names and argument values
- Case-insensitive matching
- Regex-based for flexible detection
- Logs warnings when dangerous patterns are detected

### 4. Argument Redaction

**Sensitive Argument Names:**
```yaml
redaction:
  arguments:
    - "password"
    - "secret"
    - "key"
    - "token"
    - "auth"
    - "credential"
    - "private"
    - "sensitive"
```

**Output Redaction:**
```yaml
redaction:
  outputs: true  # Redact all tool outputs
```

### 5. Security Integration

**Runtime Integration:**
- Security checks occur before tool execution
- Failed authorization prevents tool execution
- Consent requirements trigger checkpoint events
- All security events are logged with context

**Configuration Inheritance:**
- Tool-level security settings override global defaults
- Server-level settings can be refined per tool
- Security policies are enforced consistently

## Security Best Practices

### 1. Principle of Least Privilege

**Start Restrictive:**
```yaml
tools:
  - server_name: "file-server"
    allowlist: ["read_file", "list_directory"]  # Only allow safe operations
    denylist: [".*delete.*", ".*write.*"]       # Explicitly block dangerous ops
```

### 2. Defense in Depth

**Multiple Security Layers:**
```yaml
tools:
  - server_name: "system-tools"
    allowlist: ["safe_.*"]                      # Layer 1: Allowlist
    denylist: [".*dangerous.*"]                 # Layer 2: Denylist
    requires_approval:                          # Layer 3: User consent
      conditional: [".*modify.*"]
    redaction:                                  # Layer 4: Data protection
      arguments: ["password", "key"]
      outputs: false
```

### 3. Risk-Based Approval

**High-Risk Operations:**
```yaml
requires_approval:
  always: true  # For critical systems

# Or conditional based on operation type:
requires_approval:
  conditional:
    - ".*delete.*"     # Data destruction
    - ".*exec.*"       # Code execution
    - ".*network.*"    # External communication
    - ".*write.*"      # Data modification
```

### 4. Comprehensive Logging

**Security Event Tracking:**
- All authorization failures are logged
- Dangerous pattern detections are recorded
- User consent requests are tracked
- Tool execution results are audited

**Verbose Logging:**
```yaml
log_level: "info"
verbose_logging: true  # Enable detailed security logs
trace_enabled: true    # Track execution flow
```

## Example Configurations

### Minimal Security (Development)
```yaml
tools:
  - server_name: "dev-tools"
    timeout: 30s
    requires_approval:
      always: false
```

### Balanced Security (Production)
```yaml
tools:
  - server_name: "prod-tools"
    allowlist: ["read_.*", "list_.*", "get_.*"]
    denylist: [".*delete.*", ".*format.*", ".*sudo.*"]
    requires_approval:
      conditional: [".*write.*", ".*create.*"]
    redaction:
      arguments: ["password", "secret", "token"]
```

### Maximum Security (Critical Systems)
```yaml
tools:
  - server_name: "critical-tools"
    allowlist: ["read_file", "get_status"]      # Very limited operations
    denylist: [".*"]                            # Block everything else
    requires_approval:
      always: true                              # Always require consent
    redaction:
      arguments: ["password", "secret", "key", "token", "auth", "credential"]
      outputs: true                             # Redact all outputs
```

## Error Handling

**Authorization Failures:**
- Tool execution is blocked
- Detailed error message is returned
- Security event is logged
- Run continues with error response

**User Consent Required:**
- Checkpoint event is emitted
- Execution pauses (in interactive mode)
- Auto-approval available in daemon mode
- Consent decision is logged

## Extending Security

### Adding New Dangerous Patterns

Edit `registry.go` in the `containsDangerousOperations` method:

```go
dangerousPatterns := []string{
    // Existing patterns...
    `your_pattern_here`,
    `another_dangerous_.*`,
}
```

### Custom Authorization Logic

Implement additional checks in `validateToolAuthorization`:

```go
// Custom business logic
if toolName == "critical_operation" && !userHasPermission(ctx) {
    return fmt.Errorf("user lacks required permission for %s", toolName)
}
```

### Integration Points

- **Event Bus**: Security events for monitoring
- **Metrics**: Security violation counters
- **Logging**: Detailed audit trail
- **Checkpoints**: User interaction for consent

## Security Monitoring

**Key Metrics:**
- `tool_authorization_failures_total` - Authorization denials
- `tool_consent_required_total` - User consent requests
- `dangerous_operations_detected_total` - Risk pattern matches
- `tools_executed_total{outcome="blocked"}` - Blocked executions

**Log Analysis:**
- Search for "authorization failed" events
- Monitor "dangerous pattern detected" warnings
- Track "requires user consent" checkpoints
- Audit tool execution outcomes
