# Review Agent LLM Tool Calling Design

## Summary

This design adds a real LLM-backed agent analysis stage to the review workflow
 without changing the overall `review` product boundary.

The new capability is scoped to the hybrid analysis stage only:

`normalized change + collected evidence -> review agent -> structured HybridAnalysis`

The runtime keeps the existing controlled workflow and persistence model. The
agent is bounded, read-only, limited to at most three rounds of tool calling,
and always falls back to the existing heuristic analyzer when configuration or
runtime failures occur.

## Goals

- Replace the current heuristic-only analyzer path with a real
  OpenAI-compatible chat completions integration.
- Support controlled multi-round tool calling during the hybrid analysis stage.
- Preserve the existing `ChangeAnalyzer` seam so `review.Service` remains
  decoupled from vendor-specific model details.
- Keep the final output mapped onto the existing `HybridAnalysis` and
  `SuggestedRiskSignal` structures.
- Allow local private model credentials without using environment variables.

## Non-Goals

- Do not agentize the full review workflow in this change.
- Do not add write-capable tools or external side effects.
- Do not yet implement real Git, CMDB, Metrics, or Runbook connectors.
- Do not remove heuristic fallback behavior.

## Architecture

### Existing Boundary

Today the pipeline reaches `runHybridAnalysis(...)`, which delegates to
`ChangeAnalyzer`. The default implementation is `HeuristicAnalyzer`.

### New Boundary

The pipeline will continue to depend on `ChangeAnalyzer`, but the default
analyzer selection will become:

1. Construct `AgentAnalyzer` if config is present and valid.
2. Otherwise use `HeuristicAnalyzer`.
3. If `AgentAnalyzer` fails at runtime, return a structured fallback result.

### New Components

- `agent_config.go`
  Loads and merges public and local private config.
- `llm_client.go`
  Implements the OpenAI-compatible chat completions client.
- `agent_tools.go`
  Registers and dispatches the read-only analysis tools.
- `agent_runtime.go`
  Owns the multi-round loop, transcript, tool execution, and final parsing.
- `agent_analyzer.go`
  Adapts the runtime back into the existing `ChangeAnalyzer` interface.

## Configuration

### Public Config

File:
- `config/review-agent.json`

Fields:
- `enabled`
- `base_url`
- `model`
- `max_rounds`
- `timeout_ms`
- `tool_calling_enabled`
- `max_tool_calls_per_round`

### Local Private Config

File:
- `config/review-agent.local.json`

Fields:
- `api_key`

### Merge Rules

- Load public config first.
- Load local config second and override overlapping fields.
- Missing local config is allowed.
- Missing or invalid effective config disables the agent path and keeps
  heuristic fallback.

## Tool Model

The first version exposes only read-only tools over already-available review
context.

### Tool Set

- `get_change_bundle`
- `get_change_understanding`
- `list_evidence`
- `get_evidence_by_id`
- `list_rule_signals`
- `get_scoring_policy`

### Tool Principles

- No external writes.
- No network access inside tools.
- No hidden mutable state.
- Tool outputs must be deterministic and serializable.

## Runtime Loop

The runtime will execute at most three model rounds.

### Round Flow

1. Build initial system and user messages.
2. Send request with tool definitions.
3. If the model returns final structured analysis, stop.
4. If the model returns tool calls, execute them.
5. Append tool outputs as tool messages.
6. Repeat until final output or round limit is reached.

### Hard Limits

- Maximum rounds: `3`
- Maximum tool calls per round: config-driven
- Timeout: config-driven

Exceeding any limit triggers fallback.

## Prompt Contract

The system prompt defines the model as a change risk review agent. It must:

- analyze change risk for release review scenarios
- use only the provided tools
- produce a final structured result
- avoid speculation when evidence is missing
- raise `requires_human_review` when confidence is low or risk is high

The final response schema maps to:

- `summary`
- `confidence`
- `requires_human_review`
- `rationale`
- `suggested_signals[]`
  - `signal_name`
  - `severity`
  - `explanation`

## Error Handling

The agent path must never make the review API unavailable.

Fallback to `HeuristicAnalyzer` when:

- config files are missing or invalid
- API key is missing
- model request fails
- tool call payload is invalid
- response cannot be parsed
- round limit is exceeded
- final analysis payload is incomplete

Fallback results must remain explicit through:

- `analysis.mode`
- `analysis.analyzer`
- `analysis.summary`

## Testing

### Unit Tests

- config merge behavior
- tool registration and dispatch
- tool output serialization
- round limit enforcement
- malformed tool call handling
- malformed final response handling
- fallback selection behavior

### Integration Tests

Use a fake OpenAI-compatible HTTP server to simulate:

- direct final response
- one-round tool calling
- two-round tool calling
- runtime failure and heuristic fallback

### Service-Level Tests

Verify review responses surface:

- real agent `analysis.mode`
- real agent `analysis.analyzer`
- merged `suggested_signals`
- fallback markers when agent execution fails

## Rollout Strategy

This change should be introduced behind configuration rather than by replacing
the current heuristic path unconditionally.

Recommended rollout:

1. land config loading and runtime loop
2. land fake-server tests
3. enable locally with private config
4. verify review output shape remains backward compatible

## Open Questions Resolved

- Provider protocol: OpenAI-compatible chat completions
- Credentials source: local private config file, not environment variables
- Agent style: full multi-round analysis runtime
- Tool calling limit: maximum three rounds
- Fallback: always preserve heuristic analyzer path
