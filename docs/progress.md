# Progress

## Current State

The repository now contains a runnable Go API skeleton for the Phase 1 review flow.

Implemented capabilities:
- `POST /api/v1/reviews`
- `GET /api/v1/reviews/{review_id}`
- `GET /api/v1/reviews/{review_id}/timeline`
- `POST /api/v1/reviews/{review_id}/human-decision`
- `POST /api/v1/reviews/{review_id}/retry`
- `POST /api/v1/reviews/{review_id}/evaluation`
- `GET /api/v1/reviews/{review_id}/export`
- `GET /api/v1/evaluations/metrics`

Storage behavior:
- If `DATABASE_URL` is unset, the service uses the in-memory store.
- If `DATABASE_URL` is set, the service uses PostgreSQL through `pgx`.
- If `AUTO_MIGRATE=1`, startup runs all `migrations/*.up.sql` files.

The review pipeline is still a fixed MVP DAG with heuristic rules. It is not connected to real Git, incident, runbook, or metrics systems yet.

## Important Files

Entry point:
- `cmd/review-api/main.go`

HTTP layer:
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

Local startup:
- `Dockerfile`
- `docker-compose.yml`
- `docs/local-development.md`

## What Has Been Finished

Recent milestone commits:
- `ca387dd` `feat: add review api skeleton and postgres store`
- `93e043a` `feat: persist human decisions`
- `53061f4` `chore: add local development setup`
- `d77c182` `feat: persist evaluation records`
- `f4740a5` `test: add api integration coverage`
- `d1d17b1` `feat: add evaluation feedback endpoint`
- `06db8e9` `docs: align openapi with evaluation endpoint`

The current branch was pushed to `origin/main` after `06db8e9`.

## Verified Commands

Commands already verified successfully in this repository:
- `go build ./...`
- `go test ./...`
- `docker compose up --build`

Note:
- `docker compose up --build` was added as the local startup path, but end-to-end Docker verification was not executed in this session.
- `go build ./...` and `go test ./...` passed locally in this workspace.

## Known Gaps

Still missing or intentionally simplified:
- No PostgreSQL integration tests yet.
- No API contract validation tooling yet.
- No real ingestion from webhook, Git, CI, incidents, runbooks, or metrics providers.
- No authentication or authorization layer.
- No release approval console or frontend.
- `evaluation_records` currently support feedback persistence, but there is no separate endpoint to query raw evaluation detail.
- Metrics are computed from persisted records in-process; there is no dedicated reporting pipeline yet.

## Recommended Next Steps

Highest priority:
1. Add PostgreSQL integration tests covering migrations, create review, human decision persistence, evaluation feedback persistence, and metrics queries.
2. Add request/response contract validation against `openapi/openapi.yaml`.
3. Add a small README or expand `docs/local-development.md` so a fresh machine can bootstrap immediately.

Second priority:
1. Replace heuristic-only workflow with pluggable ingestion and rule modules.
2. Add explicit repository interfaces for PostgreSQL-backed reads/writes if storage logic starts growing further.
3. Add a query endpoint for raw evaluation detail if downstream reporting needs it.

## Handoff Notes

If resuming work on another machine, start with:
1. `git pull`
2. `docker compose up --build`
3. Run `go test ./...`
4. Continue with PostgreSQL integration tests as the next concrete engineering step.

When modifying behavior, keep `openapi/openapi.yaml` aligned with implementation and continue the current practice of making one logical change per commit.
