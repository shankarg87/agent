# Tool Approval System

This document describes the tool approval system implemented in the agent runtime, which provides user consent mechanisms for potentially dangerous tool operations.

## Overview

The tool approval system provides multiple layers of protection:

1. **Pre-execution filtering**: Dangerous tools are removed from the LLM's available tool schema
2. **Runtime authorization checks**: Tool calls are validated against allowlist/denylist patterns
3. **User consent flow**: Risky operations pause execution and require explicit user approval
4. **Automatic approval**: Daemon mode can auto-approve tools based on configuration

## Approval Flow

### 1. Tool Schema Filtering (Pre-LLM)

Before the LLM sees available tools, they are filtered based on configuration:

```yaml
tools:
  - server_name: "file-server"
    allowlist: ["read_.*", "list_.*"]  # LLM only sees read/list operations
    denylist: [".*delete.*"]           # Delete operations completely hidden
```

**Result**: The LLM cannot even attempt to call filtered tools.

### 2. Runtime Authorization (Pre-execution)

If a tool call passes the schema filter, it's checked against runtime policies:

```yaml
tools:
  - server_name: "system-tools"
    requires_approval:
      always: false
      conditional:
        - ".*write.*"    # Require approval for write operations
        - ".*exec.*"     # Require approval for execution
```

### 3. User Consent Flow (Interactive Mode)

When a tool requires approval:

1. **Execution pauses** - Run status becomes `paused_checkpoint`
2. **Event emitted** - `checkpoint_required` event with approval details
3. **User decision** - Client must call `/runs/{id}/approve` with approval decision
4. **Resume or cancel** - Execution continues or terminates based on decision

## API Usage

### Starting a Run with Approval-Required Tools

```bash
# Create a run that might use dangerous tools
curl -X POST http://localhost:8080/runs \
  -H "Content-Type: application/json" \
  -d '{
    "input": "Delete all temporary files in /tmp",
    "mode": "interactive",
    "session_id": "user-session-123"
  }'

# Response
{
  "run_id": "run-abc123",
  "status": "running"
}
```

### Monitoring for Approval Events

```bash
# Stream events to watch for approval requests
curl -N http://localhost:8080/runs/run-abc123/events

# You'll see events like:
data: {"type": "checkpoint_required", "data": {
  "tool_call_id": "call-456",
  "tool_name": "delete_files",
  "reason": "Tool 'delete_files' appears to perform write operations",
  "prompt": "Do you approve executing 'delete_files'? Tool 'delete_files' appears to perform write operations",
  "tool_arguments": "{\"path\": \"/tmp/*\"}",
  "approval_required": true,
  "approval_schema": {
    "type": "object",
    "properties": {
      "approved": {"type": "boolean", "description": "Whether to approve the tool execution"},
      "reason": {"type": "string", "description": "Optional reason for the decision"}
    },
    "required": ["approved"]
  }
}}

data: {"type": "run_paused", "data": {
  "reason": "tool_approval_required",
  "tool_name": "delete_files"
}}
```

### Approving or Denying Tool Execution

```bash
# Approve the tool execution
curl -X POST http://localhost:8080/runs/run-abc123/approve \
  -H "Content-Type: application/json" \
  -d '{
    "approved": true,
    "reason": "User confirmed deletion is safe"
  }'

# OR deny the tool execution
curl -X POST http://localhost:8080/runs/run-abc123/approve \
  -H "Content-Type: application/json" \
  -d '{
    "approved": false,
    "reason": "Too risky - might delete important files"
  }'
```

### Response to Approval

**If approved:**
```json
{
  "status": "approved",
  "message": "Tool execution approved and resumed"
}
```

**If denied:**
```json
{
  "status": "denied",
  "message": "Tool execution denied and run cancelled"
}
```

## Configuration Examples

### Maximum Security (Critical Systems)
```yaml
tools:
  - server_name: "critical-tools"
    allowlist: ["read_status", "get_info"]     # Very limited operations
    denylist: [".*"]                           # Block everything else
    requires_approval:
      always: true                             # Every tool needs approval
    redaction:
      arguments: ["password", "secret", "key"]
      outputs: true                            # Hide all outputs

approval_mode: "policy"
approval_policies:
  write_ops: true
  dangerous_ops: true
auto_approve_in_daemon: false                  # Never auto-approve
```

### Balanced Security (Production)
```yaml
tools:
  - server_name: "prod-tools"
    allowlist: ["read_.*", "list_.*", "get_.*"]
    denylist: [".*delete.*", ".*format.*", ".*sudo.*"]
    requires_approval:
      always: false
      conditional:
        - ".*write.*"
        - ".*create.*"
        - ".*modify.*"
    redaction:
      arguments: ["password", "secret", "token"]

auto_approve_in_daemon: false
```

### Development Environment
```yaml
tools:
  - server_name: "dev-tools"
    denylist: [".*format.*", ".*sudo.*"]       # Block only extremely dangerous ops
    requires_approval:
      conditional:
        - ".*delete.*"                         # Only deletions need approval

auto_approve_in_daemon: true                   # Auto-approve in daemon mode
```

### Daemon Mode with Auto-Approval
```yaml
tools:
  - server_name: "automation-tools"
    allowlist: [".*"]                          # Allow all tools
    denylist: ["format_disk", "sudo_.*"]      # Except destructive ones
    requires_approval:
      conditional:
        - ".*delete.*"                         # Would normally require approval

auto_approve_in_daemon: true                   # But auto-approve in daemon mode
```

## Event Types

### Checkpoint Required
```json
{
  "type": "checkpoint_required",
  "data": {
    "tool_call_id": "call-123",
    "tool_name": "dangerous_operation",
    "reason": "Tool contains potentially dangerous operations",
    "prompt": "Do you approve executing 'dangerous_operation'?",
    "tool_arguments": "{\"action\": \"delete_all\"}",
    "approval_required": true,
    "approval_schema": {...}
  }
}
```

### Run State Changes
```json
{"type": "run_paused", "data": {"reason": "tool_approval_required"}}
{"type": "run_resumed", "data": {"reason": "tool_approved"}}
{"type": "run_cancelled", "data": {"reason": "tool_execution_denied"}}
```

## Error Handling

### Invalid Approval Request
```bash
# Missing required fields
curl -X POST http://localhost:8080/runs/run-abc123/approve \
  -d '{"reason": "test"}'  # Missing "approved" field

# Response: 400 Bad Request
{
  "error": "Missing required field: approved"
}
```

### Run Not in Approval State
```bash
# Trying to approve when run isn't paused for approval
curl -X POST http://localhost:8080/runs/run-abc123/approve \
  -d '{"approved": true}'

# Response: 400 Bad Request
{
  "error": "run is not paused for approval, current status: running"
}
```

### Run Not Found
```bash
curl -X POST http://localhost:8080/runs/invalid-run/approve \
  -d '{"approved": true}'

# Response: 404 Not Found
{
  "error": "run not found or not active"
}
```

## Security Best Practices

### 1. Defense in Depth
- Use both allowlist and denylist filtering
- Configure approval requirements at multiple levels
- Enable verbose logging for audit trails

### 2. Principle of Least Privilege
- Start with restrictive allowlists
- Gradually add permissions as needed
- Regularly review and update tool permissions

### 3. Environment-Specific Policies
- **Development**: More permissive, faster iteration
- **Production**: Balanced security with operational needs
- **Critical Systems**: Maximum security, manual approval required

### 4. Audit and Monitoring
```yaml
log_level: "info"
verbose_logging: true
trace_enabled: true
metrics_enabled: true
```

Monitor these metrics:
- `tool_authorization_failures_total`
- `tool_consent_required_total`
- `dangerous_operations_detected_total`

### 5. User Training
- Educate users on approval decisions
- Provide clear context in approval prompts
- Document expected approval scenarios

## Integration Examples

### Web UI Integration
```javascript
// Listen for approval events via SSE
const eventSource = new EventSource(`/runs/${runId}/events`);
eventSource.onmessage = (event) => {
  const data = JSON.parse(event.data);

  if (data.type === 'checkpoint_required') {
    showApprovalDialog({
      tool: data.data.tool_name,
      reason: data.data.reason,
      prompt: data.data.prompt,
      arguments: data.data.tool_arguments,
      onApprove: (reason) => approveToolCall(runId, true, reason),
      onDeny: (reason) => approveToolCall(runId, false, reason)
    });
  }
};

function approveToolCall(runId, approved, reason) {
  fetch(`/runs/${runId}/approve`, {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({approved, reason})
  });
}
```

### CLI Integration
```bash
#!/bin/bash
# approval-handler.sh - Simple approval handler

run_id=$1
event_stream=$(curl -s -N "http://localhost:8080/runs/$run_id/events")

echo "$event_stream" | while IFS= read -r line; do
  if [[ "$line" == *"checkpoint_required"* ]]; then
    tool_name=$(echo "$line" | jq -r '.data.tool_name')
    reason=$(echo "$line" | jq -r '.data.reason')

    echo "Tool '$tool_name' requires approval: $reason"
    echo -n "Approve? (y/n): "
    read -r approval

    if [[ "$approval" == "y" ]]; then
      curl -X POST "http://localhost:8080/runs/$run_id/approve" \
        -H "Content-Type: application/json" \
        -d '{"approved": true, "reason": "CLI user approved"}'
      echo "✓ Tool approved"
    else
      curl -X POST "http://localhost:8080/runs/$run_id/approve" \
        -H "Content-Type: application/json" \
        -d '{"approved": false, "reason": "CLI user denied"}'
      echo "✗ Tool denied"
    fi
  fi
done
```

The approval system provides comprehensive protection while maintaining usability for different deployment scenarios.
