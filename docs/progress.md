# Progress

## Current Positioning

The repository is now positioned as:

`Agentic Change Review Copilot`

This should be understood as:

- a workflow-first change-risk audit runtime
- an engineering system for release-review scenarios, not a prompt demo
- a controlled agent workflow with audit persistence, human intervention, and evaluation feedback

## Current State

The repository has been realigned back to the review-centric product direction:

- the active HTTP entrypoint now serves `review` semantics again
- the OpenAPI contract now exposes `reviews` endpoints
- the active domain package is `internal/review`
- the GitHub `pull_request` webhook now maps into review creation instead of test tasks
- the evidence pipeline now supports allowlisted external context attachments for Git, CMDB, metrics, and runbook-style metadata
- the external-context evidence path now strips unexpected nested fields before persistence
- a pluggable hybrid-analysis stage now sits between evidence collection and final rule scoring, with a heuristic fallback analyzer and response-level `analysis` output

Current reusable pieces:

- Go HTTP API skeleton
- controlled workflow / state-machine style service flow
- PostgreSQL persistence
- Docker local startup
- OpenAPI contract
- basic automated tests
- structured review timeline and audit trace persistence
- risk signals, scoring, recommendation, rollback planning, and human-review gating
- human decision persistence for approve / reject / override flows
- post-release evaluation feedback and aggregate review metrics
- GitHub PR webhook ingestion with optional HMAC signature verification

Secondary reusable pieces kept in-tree:

- `internal/testflow` workflow runtime exploration
- live HTTP runner and artifact-oriented execution patterns that may later feed review evidence collection

## Final Product Direction

The final product direction is fixed as one clear main line:

`PR / SQL / K8s change -> context retrieval -> hybrid risk analysis -> recommendation -> human decision -> evaluation feedback`

Near-term scope order:

- Phase 1: PR-driven release review
- Phase 2: SQL and K8s change audit
- Phase 3: stronger external tool integration and feedback-driven rule / prompt iteration

## Important Files

Current runtime and API:

- `cmd/review-api/main.go`
- `internal/api/review_handler.go`
- `internal/api/review_github_webhook.go`
- `internal/api/review_handler_test.go`
- `internal/review/service.go`
- `internal/review/pipeline.go`
- `internal/review/github_webhook.go`
- `internal/review/store.go`
- `internal/review/postgres.go`

Contract and schema:

- `openapi/openapi.yaml`
- `migrations/*.sql`

Reference implementation and exploration:

- `internal/testflow/*`
- `cmd/testflow-cli/main.go`

## What Is Done Against The Acceptance Goal

Implemented:

- review workflow orchestration
- PostgreSQL-backed audit persistence
- review status progression and timeline records
- rule-based risk signal extraction
- recommendation and rollback-plan generation
- human intervention APIs
- GitHub PR webhook ingestion
- evaluation feedback persistence and aggregate metrics
- allowlisted external-context evidence packing
- hybrid `rules + LLM` analysis skeleton with pluggable analyzer injection

Partially implemented:

- hybrid `rules + LLM` analysis architecture
- external tool integration beyond webhook metadata
- checkpoint / resume semantics beyond retry
- measurable feedback loop for rule and prompt iteration

Not implemented yet:

- real LLM adapter and tool-calling analysis stage
- native Git / CMDB / metrics / runbook connectors
- reproducible benchmark proving 10% audit time reduction
- fully explicit checkpoint recovery model

## Immediate Next Gaps

Highest priority:

1. add a real LLM-backed analysis stage on top of the current heuristic pipeline
2. replace metadata-only external context with real adapters for Git, metrics, CMDB, and runbook retrieval
3. add checkpoint / resume support at workflow-stage granularity

Second priority:

1. enrich evaluation ingestion so feedback can tune rules and prompts over time
2. add more source adapters for SQL and K8s native change inputs
3. decide how much of `internal/testflow` should be folded back into the review runtime versus archived

## Resume Checklist

If resuming on another machine:

1. `git pull`
2. `docker compose up --build`
3. `go test ./...`
4. read `README.md`
5. continue from LLM analysis integration or real external context adapters
