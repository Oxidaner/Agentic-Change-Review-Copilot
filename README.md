# Workflow Agent AI Test Platform

This repository is a Go-based MVP for a workflow-first AI test automation platform.
Its current main line is:

`PR / requirement change -> test point extraction -> test case generation -> tool execution -> smart assertion -> failure analysis -> test report`

The project is intentionally scoped around one primary scenario first:

- Phase 1: PR-driven API regression testing
- Phase 2: Web UI core-path testing
- Phase 3: mobile automation expansion

## Current MVP

The current repository already exposes a runnable backend for the new testflow direction:

- create test tasks
- query task detail
- query workflow timeline
- retry tasks that need manual triage
- export Markdown or JSON reports
- query workflow metrics
- store data in memory or PostgreSQL

The workflow is still heuristic and simulated. Real test tool integrations are the next layer.

## API

Implemented endpoints:

- `POST /api/v1/test-tasks`
- `GET /api/v1/test-tasks/{task_id}`
- `GET /api/v1/test-tasks/{task_id}/timeline`
- `POST /api/v1/test-tasks/{task_id}/retry`
- `GET /api/v1/test-tasks/{task_id}/report`
- `GET /api/v1/test-metrics`

The API contract is defined in [openapi/openapi.yaml](openapi/openapi.yaml).

## Quick Start

Run locally with Docker Compose:

```bash
docker compose up --build
```

The API will be available at `http://localhost:8080`.

Key environment variables:

- `PORT`: HTTP listen port, defaults to `8080`
- `DATABASE_URL`: when set, the service uses PostgreSQL instead of the in-memory store
- `AUTO_MIGRATE=1`: runs `migrations/*.up.sql` on startup

## Example Request

```bash
curl -X POST http://localhost:8080/api/v1/test-tasks \
  -H "Content-Type: application/json" \
  -d '{
    "input_type": "pull_request",
    "source_id": "PR-123",
    "repo": "gateway-service",
    "service": "api-gateway",
    "payload": {
      "title": "adjust auth routing",
      "author": "alice",
      "head_commit": "def456",
      "description": "route auth traffic through gateway"
    }
  }'
```

## Storage

- in-memory storage by default for local development
- PostgreSQL when `DATABASE_URL` is configured
- SQL schema migrations under [migrations](migrations)

## Project Layout

```text
cmd/review-api/         API entrypoint
internal/api/           HTTP handlers and tests
internal/testflow/      test workflow domain logic and storage
internal/review/        legacy review-oriented prototype retained temporarily
migrations/             SQL migrations
openapi/                OpenAPI contract
docs/                   spec, design, progress, local development
```

## Documentation

- [docs/spec.md](docs/spec.md)
- [docs/workflow_agent_ai_test_platform_design.md](docs/workflow_agent_ai_test_platform_design.md)
- [docs/progress.md](docs/progress.md)
- [docs/local-development.md](docs/local-development.md)

## Status

This repository now runs on the new `test-tasks` API surface, but some legacy `internal/review` code is still kept in-tree during the rewrite. The primary missing pieces are:

- real API test runner integration
- smarter assertion generation and schema validation
- failure analysis grounded in logs, traces, and artifacts
- PostgreSQL integration tests for the new task lifecycle
- optional CLI or webhook adapters
