# Progress

## Current Positioning

The repository is now positioned as:

`Evidence-Grounded Change Review Copilot`

This means the project should be understood as:
- a workflow-first runtime for engineering change review
- an AI capability layer embedded into real engineering processes
- a controlled, evidence-grounded review system rather than a free-form agent

The current codebase is already aligned with that direction at the MVP skeleton level.

## Current Implementation State

The repository contains a runnable Go backend skeleton for the Phase 1 review flow.

Implemented API capabilities:
- `POST /api/v1/reviews`
- `GET /api/v1/reviews/{review_id}`
- `GET /api/v1/reviews/{review_id}/timeline`
- `POST /api/v1/reviews/{review_id}/human-decision`
- `POST /api/v1/reviews/{review_id}/retry`
- `POST /api/v1/reviews/{review_id}/evaluation`
- `GET /api/v1/reviews/{review_id}/export`
- `GET /api/v1/evaluations/metrics`

Storage behavior:
- Without `DATABASE_URL`, the service uses the in-memory store.
- With `DATABASE_URL`, the service uses PostgreSQL through `pgx`.
- With `AUTO_MIGRATE=1`, startup runs all `migrations/*.up.sql`.

Current workflow behavior:
- fixed DAG / state-machine style flow
- heuristic risk signals
- structured recommendation output
- human decision persistence
- evaluation feedback persistence
- metrics aggregation from persisted review records

## How To Read The Current Architecture

The current repository should be mapped to the target three-layer architecture like this.

Runtime / Orchestration layer:
- `cmd/review-api/main.go`
- `internal/api/handler.go`
- `internal/review/service.go`

AI Capability layer:
- heuristic rule extraction in `internal/review/service.go`
- evidence, signal, recommendation, rollback, evaluation models in `internal/review/types.go`

Scenario layer:
- engineering change review as the only implemented scenario
- current input contract is general, but the intended Phase 1 narrative is now:
  `PR diff + K8s/YAML review first`

## Important Files

Design and handoff:
- `docs/agentic_change_review_copilot_design.md`
- `docs/spec.md`
- `docs/local-development.md`

Entrypoint and HTTP layer:
- `cmd/review-api/main.go`
- `internal/api/handler.go`
- `internal/api/handler_test.go`

Domain and workflow logic:
- `internal/review/types.go`
- `internal/review/service.go`

Storage:
- `internal/review/store.go`
- `internal/review/postgres.go`

Contracts and schema:
- `openapi/openapi.yaml`
- `migrations/*.sql`

## Major Completed Commits

- `ca387dd` `feat: add review api skeleton and postgres store`
- `93e043a` `feat: persist human decisions`
- `53061f4` `chore: add local development setup`
- `d77c182` `feat: persist evaluation records`
- `f4740a5` `test: add api integration coverage`
- `d1d17b1` `feat: add evaluation feedback endpoint`
- `06db8e9` `docs: align openapi with evaluation endpoint`
- `85d0a0a` `docs: add progress handoff notes`
- `849d64d` `docs: refocus architecture narrative`

## Verified Commands

Verified successfully in this repository:
- `go build ./...`
- `go test ./...`

Local startup path added:
- `docker compose up --build`

Note:
- `docker compose up --build` is the intended local path, but full Docker end-to-end verification was not completed in-session.

## Known Gaps

Still missing or intentionally simplified:
- No PostgreSQL integration tests yet.
- No contract validation tooling against `openapi/openapi.yaml`.
- No real Git / CI / incident / runbook / metrics integrations yet.
- No explicit `Tool`, `Workflow`, or `ModelAdapter` abstractions in code yet.
- No CLI demo yet.
- No frontend console yet.
- No authentication / authorization layer yet.
- Current AI layer is still heuristic-heavy and not connected to real external evidence providers.

## Recommended Next Steps

Highest priority:
1. Add PostgreSQL integration tests for migrations, create review, human decision persistence, evaluation persistence, and metrics queries.
2. Make the runtime abstractions explicit in code: `Workflow`, `Step`, `Tool`, `ModelAdapter`, `TraceEvent`.
3. Narrow implementation work to the Phase 1 scenario narrative: `PR diff + K8s/YAML`.

Second priority:
1. Add contract validation against `openapi/openapi.yaml`.
2. Add a CLI demo path that produces JSON and Markdown report outputs.
3. Start separating rule engine, retriever, and recommendation generator modules from the current monolithic service implementation.

## Resume Checklist

If resuming work on another machine:
1. `git pull`
2. `docker compose up --build`
3. `go test ./...`
4. Read `docs/agentic_change_review_copilot_design.md`
5. Continue with PostgreSQL integration tests as the next engineering step

When changing behavior:
- keep `openapi/openapi.yaml` aligned with implementation
- keep docs aligned with the Evidence-Grounded positioning
- keep one logical change per commit
