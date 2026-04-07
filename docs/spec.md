# Agentic Change Review Copilot Spec

## 1. Goal And Boundary

### 1.1 Goal
- Turn engineering changes into a controlled AI-driven change-risk audit workflow.
- Accept PR, SQL, and K8s configuration changes as review inputs.
- Combine rule prefiltering and LLM analysis to identify risk, collect evidence, and produce audit decisions.
- Support human intervention, checkpoint / resume, and audit trace persistence.
- Use external tools to enrich review context and improve release decisions.

### 1.2 Non-Goal
- Do not build a generic autonomous agent.
- Do not build a generic testing platform in the MVP.
- Do not optimize for free-form chat as the primary interface.
- Do not try to cover every external system integration in the first phase.
- Do not optimize for chat interaction as the primary interface.

### 1.3 Core Principle
- Workflow first.
- Evidence first.
- Hybrid `rules + LLM` analysis first.
- Human-in-the-loop first.
- Structured output first.
- Feedback loop first.

## 2. Main Line

The project boundary is now fixed to one clear path:

`PR / SQL / K8s change -> change normalization -> context collection -> risk signal extraction -> scoring / hybrid analysis -> recommendation -> human decision -> evaluation feedback`

This is the only main line the MVP should optimize for.

## 3. Three-Layer Architecture

### 3.1 Runtime / Orchestration Layer
Responsibilities:
- receive review requests
- manage workflow states and checkpoints
- coordinate rule engines, model analysis, and external tools
- handle retry, resume, and manual intervention
- persist audit traces and operator decisions

Target abstractions:
- `Workflow`
- `Step`
- `State`
- `Tool`
- `ToolResult`
- `ModelAdapter`
- `Review`
- `TraceEvent`

### 3.2 AI Capability Layer
Responsibilities:
- normalize engineering changes
- extract risk signals
- organize supporting evidence
- run hybrid analysis
- generate release recommendation and rollback guidance
- summarize audit rationale

Suggested modules:
- `change_parser`
- `risk_signal_extractor`
- `context_collector`
- `hybrid_analyzer`
- `recommendation_engine`
- `rollback_planner`
- `evaluation_recorder`

### 3.3 Scenario Layer
Responsibilities:
- bind the runtime and AI capability layers to concrete change-audit scenarios

Phase 1 priority:
- PR-driven release review
- SQL change audit
- K8s configuration audit

## 4. Core Workflow

1. Accept PR, SQL, or K8s change input.
2. Normalize the change into a shared review bundle.
3. Collect context from Git, CMDB, Metrics, Runbook, and attached metadata.
4. Run rule-based risk signal extraction.
5. Run hybrid analysis with heuristic and LLM-capable stages.
6. Score the change and generate recommendation output.
7. Produce rollback guidance and observability checks.
8. Persist audit trace and expose human review / override actions.
9. Record release outcome feedback and aggregate evaluation metrics.

## 5. Key Modules

### 5.1 Change Input Layer
Normalizes PR, SQL, and K8s inputs into a shared review object.

### 5.2 Context Collection Layer
Fetches or accepts Git, CMDB, metrics, runbook, and other review evidence.

### 5.3 Risk Signal Extractor
Produces structured risk signals from change content and surrounding context.

### 5.4 Hybrid Analysis Layer
Combines rule prefiltering and model analysis into a coherent audit result.

### 5.5 Recommendation And Rollback Layer
Produces review recommendation, rollback guidance, and release window notes.

### 5.6 Human Decision Layer
Captures approve / reject / override decisions and keeps them in the audit trail.

### 5.7 Evaluation Layer
Captures release outcomes and aggregate metrics for rule / prompt iteration.

## 6. Core Data Models

- `CreateReviewRequest`
- `Review`
- `ChangeBundle`
- `RiskSignal`
- `HybridAnalysis`
- `Recommendation`
- `RollbackPlan`
- `EvidenceItem`
- `TimelineEvent`
- `EvaluationMetrics`
- `TraceEvent`

## 7. Current Repository Status

The repository currently contains an active review-oriented MVP.

What can be reused:
- HTTP API skeleton
- service orchestration pattern
- PostgreSQL persistence pattern
- evaluation / metrics persistence idea
- OpenAPI-first contract workflow
- Docker startup and basic tests

What still needs hardening:
- real LLM adapter wiring
- richer external tool integrations
- explicit checkpoint / resume semantics
- measurable feedback loop proving review efficiency gains

## 8. Immediate Next Steps

1. add a real LLM-backed analysis stage on top of the current heuristic pipeline
2. replace metadata-only external context with real adapters for Git, Metrics, CMDB, and Runbook retrieval
3. add checkpoint / resume support at workflow-stage granularity
4. establish outcome-based evaluation proving the workflow reduces audit time in core scenarios

## 9. MVP Success Criteria

The MVP is successful if it can show:
- controlled workflow execution
- rule prefiltering plus LLM analysis for risk identification and evidence organization
- multi-stage workflow scheduling with audit trace persistence
- checkpoint recovery and human intervention support
- integration with Git, CMDB, Metrics, and Runbook-style context sources
- recommendation and rollback output for release-review decisions
- outcome feedback that supports rule / prompt iteration
- measurable audit-time reduction in core scenarios
