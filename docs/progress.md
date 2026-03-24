# Progress

## Current Positioning

The repository is now positioned as:

`Workflow Agent AI Test Platform`

This should be understood as:
- a workflow-first AI test automation runtime
- an engineering platform skeleton, not a single prompt demo
- a controlled agent workflow embedded into real test processes

## Current State

The repository has already crossed the key migration line:
- the active HTTP entrypoint now serves `testflow` semantics
- the OpenAPI contract now exposes `test-tasks` endpoints
- the new domain package is `internal/testflow`
- the old `internal/review` package is still kept temporarily as legacy code, but it is no longer the default API path
- the live HTTP probe path now supports sequential request chaining, query templating, response extraction from JSON bodies, headers, and cookies, cookie-backed session reuse, payload assertions, schema-aware checks, and per-request timeout/latency/retry controls

Current reusable pieces:
- Go HTTP API skeleton
- controlled workflow / state-machine style service flow
- PostgreSQL persistence
- Docker local startup
- GitHub Actions CI for unit and PostgreSQL integration coverage
- OpenAPI contract
- basic automated tests
- metadata-driven live HTTP runner for API regression probes
- richer request/response/assertion/extraction artifacts for live probes, including cookie/session and retry/timeout evidence
- schema-aware positive and negative assertions for headers, body content, JSON presence, and JSON types
- derived workflow view for runtime stages and tools
- derived execution-plan view for task detail and report export
- structured trace-event view for workflow state, planning, execution, and reporting
- artifact-grounded failure analysis with explicit result/artifact evidence refs
- local CLI adapter for create/get/report flows against the HTTP API
- GitHub pull_request webhook adapter that maps supported PR events into test tasks with optional HMAC signature verification

## Final Product Direction

The final product direction is fixed as one clear main line:

`PR / requirement change -> test point extraction -> test case generation -> tool execution -> smart assertion -> failure analysis -> test report`

Near-term scope order:
- Phase 1: PR-driven API regression testing
- Phase 2: Web UI core-path testing
- Phase 3: mobile automation expansion

## Important Files

Design and handoff:
- `docs/workflow_agent_ai_test_platform_design.md`
- `docs/spec.md`
- `docs/local-development.md`

Current runtime and API:
- `cmd/review-api/main.go`
- `cmd/testflow-cli/main.go`
- `.github/workflows/ci.yml`
- `internal/api/handler.go`
- `internal/api/github_webhook.go`
- `internal/api/handler_test.go`
- `internal/testflow/service.go`
- `internal/testflow/workflow_view.go`
- `internal/testflow/execution_plan.go`
- `internal/testflow/trace_events.go`
- `internal/testflow/live_http_runner.go`
- `internal/testflow/github_webhook.go`
- `internal/testflow/types.go`
- `internal/testflow/postgres.go`
- `internal/testflow/store.go`

Contract and schema:
- `openapi/openapi.yaml`
- `migrations/*.sql`

## Key Recent Commits

- `05fbe73` `feat: add testflow core domain`
- `a665021` `feat: switch api to testflow workflow`

These two commits are the actual business pivot point from the old review MVP into the new test automation workflow.

## What Still Needs Work

Highest priority:
1. Continue broadening the live HTTP runner beyond the current sequential HTTP path.
2. Continue improving assertion generation and artifact-grounded failure attribution.
3. Improve artifact collection and richer execution evidence.

Second priority:
1. Continue expanding PR-source ingestion beyond the current GitHub pull_request webhook path.
2. Decide whether to delete or archive `internal/review` after the migration stabilizes.
3. Make runtime abstractions more explicit: `Workflow`, `Step`, `Tool`, `ModelAdapter`, `TraceEvent`.

## Resume Checklist

If resuming on another machine:
1. `git pull`
2. `docker compose up --build`
3. `go test ./...`
4. Read `docs/workflow_agent_ai_test_platform_design.md`
5. Continue from API runner expansion or richer artifact-based analysis

## Notes

- `.idea/` is still untracked and intentionally untouched.
- The codebase is no longer in the old "docs switched but API did not" state.
- Remaining migration work is now mostly about deleting legacy code and deepening the real tool execution path.