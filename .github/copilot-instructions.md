# Copilot Instructions — Swiss Army Knife Agent (MCP-first)

You are a coding agent working on a **general-purpose** “Swiss Army Knife” agent runtime. The agent should be **domain-agnostic** and gain capability primarily through **tools exposed via MCP servers** rather than hard-coded business logic.

## 0) Primary outcome

Deliver a minimal, production-lean **agent runtime + API surface** that supports:

* **Interactive** (human-in-the-loop) sessions
* **Autonomous** (single directive "daemon mode") runs
* **Tooling via MCP** (pluggable, many servers)
* **Long-running workflows** with checkpoints
* **OpenAI-compatible facade** for maximum client compatibility (Open WebUI etc)
* **Multi-tenant capable** Multiple tenants can share the same agent instance with data isolation.

V1 is **synchronous**. Design internal abstractions so V2 can become event-driven/async.

---

## 1) Guiding principles

* **Keep the core tiny**: orchestration + state + transports + tool wiring.
* **No domain assumptions**: do not bake in workflows for any specific field.
* **Everything is a tool**: capabilities come from MCP servers.
* **Deterministic, inspectable execution**: store run logs, tool calls, tool outputs, and checkpoints.
* **Compatibility wins**: prioritize working with standard clients over novel protocols.

---

## 2) Non-goals

* Do **not** implement a custom DSL for tasks.
* Do **not** build bespoke “business logic” modules.
* Do **not** build a heavy policy/guardrail system in V1 (leave extension points).
* Do **not** require a custom UI client for basic usage.

---

## 3) Functional requirements

### 3.1 Modes

**A) Interactive mode**

* A user provides messages; the agent responds and can invoke tools.
* The agent can pause at checkpoints and request input/approval.

**B) Autonomous mode**

* A caller submits a single directive (plus optional context payload).
* The agent runs a workflow with minimal/no further input.

### 3.2 Sessions and runs

* Support **multiple concurrent sessions**.
* A session may have **multiple runs**.
* **Interactive mode conversation handling**:
  * Each `/runs` call with a `session_id` continues the conversation
  * Server maintains conversation history per session
  * Similar to OpenAI's new Conversations API approach
* **OpenAI compatibility for `/v1/chat/completions`**:
  * Each call creates a new run internally
  * Accept full message array (stateless, traditional OpenAI style)
  * For now, does NOT map agent profiles to OpenAI model parameter
* Persist:

  * conversation history
  * run state machine status
  * tool calls + outputs
  * artifacts/attachments (optional)
  * pointers to memory (if used)

### 3.3 MCP-first tool system

* The runtime connects to **multiple MCP servers**.
* Required tool categories (as examples):

  * **shell/command execution** (generic “power tool”)
  * **code repo integration** (e.g., GitHub)
  * **memory** (episodic and/or semantic)
  * **custom servers** (first-class support)

Tool invocation must be:

* logged
* time-bounded
* cancelable at the run level

### 3.4 Long-running workflows and checkpoints

* Runs may span many tool calls.
* The agent can emit **checkpoints**:

  * `checkpoint_required` (needs human input/approval)
  * `checkpoint_info` (progress marker)
* V1: synchronous tool calls; for long tasks use polling patterns.
* V2: reserve extension points for server push / async resumption.

# Agent Configuration — What to include beyond the basics

This doc expands the agent configuration knobs beyond the initial list:

1. system prompt, 2) MCP tools + permission gating, 3) long-term memory flag, 4) daemon/interactive/both, 5) logging mode.

The goal: a **config-driven agent profile** that is flexible, safe-by-default, and compatible with standard clients.

### 3.5 Agent Configuration

#### 3.5.1 Identity + versioning

* `profile_name`: stable identifier used by clients (maps to OpenAI `model` string).
* `profile_version`: semantic version; enables safe rollouts and reproducibility.
* `description`: human-readable purpose.
* `labels/tags`: e.g., `"general"`, `"coding"`, `"research"` for UI filtering.

Why: traceability and reproducible behavior.

#### 3.5.2 Model routing + fallbacks

* `primary_model`: e.g., provider + model id.
* `fallback_models`: ordered list used on 429/5xx/timeout.
* `routing_strategy`: `"single" | "fallback" | "cost_aware" | "latency_aware"`.
* `max_context_tokens`, `max_output_tokens`.
* `temperature/top_p` (or equivalent), default values.

Why: stability in production and graceful degradation.

#### 3.5.3 Prompting + persona structure

* `system_prompt`: base role/instructions.
* `prompt_templates`:

  * `interactive_preamble`
  * `autonomous_preamble`
  * `tool_use_preamble`
  * `checkpoint_preamble` (how to ask for approval/input)
* `few_shot_examples`: optional library.
* `output_style`: e.g., `"concise" | "verbose" | "json"`.

Why: keep core runtime generic; encode behavior in profile.

#### 3.5.4 Tools/MCP configuration (beyond “list of servers”)

For each MCP server/tool:

* `name`, `transport` (`stdio` / `http`), `endpoint`.
* `capabilities`: declared tool groups.
* `timeouts`: connect + per-call.
* `retries`: max + backoff.
* `concurrency_limit`: per tool/server.
* `allowlist/denylist`: tool names and/or argument patterns.
* `requires_approval` rules:

  * `always` tools
  * `conditional` rules (regex/policy) e.g., "writes", "deletes", "network", "shell".
* `redaction`: which arguments/outputs must be masked in logs.

Why: predictable ops + safer defaults.

#### 3.5.5 Approval + human-in-the-loop policy

* `approval_mode`: `"never" | "always" | "policy"`.
* `approval_policies`:

  * `write_ops`: require approval
  * `dangerous_ops`: require approval
  * `budget_exceeded`: require approval
* `checkpoint_schema`: how input is requested (freeform vs structured).
* `auto_approve_in_daemon`: bool (usually false).

Why: you’ll want consistent behavior across interactive vs daemon.

---

#### 3.5.6 Budgets + stop conditions

* `max_tool_calls`
* `max_run_time_seconds`
* `max_cost_usd` (if you track)
* `max_failures_per_run`
* `rate_limits`: per user / per org / per token bucket.

Why: prevent runaway loops and surprise bills.

#### 3.5.8 Memory configuration (beyond a bool)

* `memory_enabled`: bool
* `memory_provider`: MCP server reference
* `write_policy`: `"never" | "explicit" | "auto"`
* `read_policy`: when memory is retrieved (always / on demand / by classifier)
* `retention_days`: how long to keep memories
* `pii_policy`: what must not be stored; redaction rules

Why: long-term memory will need lifecycle + governance. I expect this to be changed a lot.


#### 3.5.9 Logging, tracing, and observability

* `log_level`: debug/info/warn/error
* `log_destinations`: stdout, file, OTEL, etc.
* `trace_enabled`: bool
* `metrics_enabled`: bool
* `sampling_rate`
* `log_payload_policy`: full, redacted, hashes only

Why: without this, debugging multi-step tool runs is pain.

#### 3.5.10 Reliability knobs

* `idempotency_keys`: support on `/runs` creates
* `resume_on_restart`: bool (even in V1 you can restore state)
* `retry_failed_steps`: limited + policy-driven
* `circuit_breakers`: per tool/server/provider

Why: makes the runtime feel “real” fast.

## 4) External API requirements (clients)

### 4.1 Native API (recommended)

Implement a rich `/runs` API for first-class operation and internal correctness.

**Endpoints** (suggested):

* `POST /runs` → start a run; returns `run_id`
* `GET /runs/{run_id}` → run status + final output
* `GET /runs/{run_id}/events` → SSE stream of structured events
* `POST /runs/{run_id}/cancel` → cancel
* (optional) `POST /runs/{run_id}/input` → supply checkpoint input

**Events** (suggested types):

* `run_started`, `run_completed`, `run_failed`, `run_cancelled`
* `text_delta`, `final_text`
* `tool_started`, `tool_stdout`, `tool_stderr`, `tool_completed`, `tool_failed`
* `checkpoint_required` (include prompt + schema)
* `artifact_created` (reference)

## 5) Architecture constraints

### 5.1 Language/runtime

* Implementation language: **Go** (monorepo with multiple packages).
* Prioritize simplicity and debuggability.
* Use production-ready dependencies (stdlib-first when possible).

### 5.2 State/storage

* V1 uses:

  * **In-memory store** (default for quick start)
  * Storage interface is **swappable** for Postgres/SQLite in future
  * Note: memory may eventually be provided by an agent/MCP in a more generic way
* Must support:

  * idempotent writes
  * recovery after crash (V2 feature)

### 5.3 MCP Integration

* Use **github.com/mark3labs/mcp-go** library (NOT custom protocol implementation)
* Test with mock MCP server (echo or shell executor included in examples/)
* Stdio transport only for V1

### 5.4 LLM Providers

* **Primary providers**: Anthropic + OpenAI (fully implemented)
* **Future providers**: Gemini + Ollama (stubbed for extensibility)
* Use custom wrapper around native Go SDKs (anthropic-sdk-go, go-openai, etc.)
* Provider selection via agent config `primary_model.provider`

### 5.5 Concurrency model

* Multiple runs may execute concurrently.
* Agent **SHOULD be able to handle parallel tool call execution** (V1 supports this)
* Tool timeouts/retries dictated by configuration
* After configured retries, behavior is configurable: fail run OR let LLM decide
* Cancellation should propagate to tool calls when possible.

### 5.6 Checkpoint Behavior

* When run hits `checkpoint_required`: **pause and emit event**
* In daemon mode: **auto-skip checkpoints** (per config `auto_approve_in_daemon: true`)
* Follow standard behavior for checkpoints in interactive mode

### 5.7 Event Streaming

* **SSE only** for V1 (WebSocket reserved for V2)
* Events are **persisted** for replay
* Available via `GET /runs/{id}/events`

### 5.8 Run State Machine

* States: `queued → running → (paused_checkpoint) → completed | failed | cancelled`
* Runs do **not** need to resume after server restart in V1

### 5.9 Extensibility

* MCP server registry should be pluggable via config.
* Agent "profiles" should be config-driven (YAML in `configs/agents/`)
* MCP servers defined in `configs/mcp/servers.yaml`
* Event schema should be stable and versioned.

### 5.10 Multi-Tenancy & Auth

* Tenant isolation: **API keys** (tenant management NOT in scope for V1)
* Common database for all tenants
* Authentication: **NOT in scope for V1** (will be refactored later)

---

## 6) Implementation plan (deliverables)

### 6.1 Repo skeleton (minimum)

* `cmd/agentd` — HTTP server exposing `/runs` + `/v1/*` facade
* `internal/runtime` — run loop + state machine
* `internal/mcp` — MCP client + server registry + tool invocation
* `internal/store` — persistence layer
* `internal/events` — SSE + event encoding
* `configs/` — agent profiles + MCP server config

### 6.2 Reference flows (must implement)

* **Interactive**: user message → run → tool call → response
* **Autonomous**: directive → run → tool calls → final output
* **Checkpoint**: run pauses → caller posts input → run resumes
* **Streaming**: both `/runs/{id}/events` and `/v1/chat/completions` streaming work

### 6.3 Tests

* Unit tests for:

  * run state transitions
  * event encoding/decoding
  * MCP tool invocation wrapper
* Integration test:

  * start run → tool executes → streamed output arrives

---

## 7) Quality bar

* Clear logs and traceability: every tool call, result, and decision point should be discoverable.
* Minimal dependencies.
* Works with:

  * Open WebUI (via OpenAI-compatible base URL)
  * VSCode extension (Continue) using `apiBase: .../v1`

---

## 8) Notes

* Keep V1 synchronous, but do not paint yourself into a corner: represent the run loop as a state machine so async triggers can be added later.
* The native `/runs` API is the ground truth - `/v1/chat/completions` calls INTO the runs interface
* IMPORTANT: `/v1/chat/completions` is a facade that internally creates runs

---

## 9) Implementation Decisions (V1 Complete)

### What Was Built

✅ **Project Structure**: Go monorepo with `cmd/agentd`, `internal/*`, `configs/`, `examples/`
✅ **Storage Layer**: Interface-based with in-memory implementation (swappable for Postgres)
✅ **LLM Providers**: Anthropic (full), OpenAI (full), Gemini/Ollama (stubbed)
✅ **MCP Integration**: Using `mark3labs/mcp-go` library
✅ **Run State Machine**: Full lifecycle management with event streaming
✅ **Native API**: `/runs` endpoints (create, get, events, cancel)
✅ **OpenAI Compatibility**: `/v1/chat/completions` with streaming support
✅ **Event Bus**: SSE streaming with event persistence
✅ **Agent Profiles**: YAML-based configuration in `configs/agents/`
✅ **Example MCP Server**: Echo server with 3 tools (echo, uppercase, add_numbers)
✅ **Default Configuration**: Production-ready default profile

### Key Design Choices

1. **Session Management**: Server-side conversation tracking per session_id (similar to OpenAI Conversations API)
2. **OpenAI Facade**: Each `/v1/chat/completions` call creates a new run internally
3. **Tool Execution**: Synchronous in V1, but architecture allows async in V2
4. **Checkpoints**: Auto-skip in daemon mode, manual approval in interactive
5. **Error Handling**: Configurable retries, then either fail or let LLM recover
6. **Event Persistence**: All events stored for replay and debugging
7. **Multi-Provider**: Custom abstraction layer (not using heavy frameworks like langchain)

### Known Limitations (V1)

- No resume after server restart
- No WebSocket streaming (SSE only)
- No Gemini/Ollama implementations (coming in V2)
- No authentication/authorization
- No tenant management CRUD
- No long-term memory integration (stubbed)
- No checkpoint approval workflow UI

### Next Steps for V2

- Implement Gemini & Ollama providers
- Add PostgreSQL storage backend
- Implement WebSocket streaming
- Add authentication layer
- Build checkpoint approval workflows
- Integrate memory via MCP
- Add cost tracking & budgets
- Support async tool execution
