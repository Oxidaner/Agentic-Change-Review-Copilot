# Agentic Change Review Copilot

Agentic Change Review Copilot is a Go-based MVP for pre-release change risk review. It accepts structured change-review requests and returns a review record with risk signals, risk level, rollout guidance, rollback planning, and human review status.

This repository is intended as a practical starting point for a GitHub-facing change review service, especially for pull request review workflows that need more structure than a plain LLM summary.

## Overview

Modern release risk rarely comes from code diffs alone. It also depends on:

- whether the change targets production
- whether it touches critical services such as auth, gateway, or billing
- whether the change affects sensitive artifacts like migrations or routing rules
- whether rollout and rollback plans are explicit
- whether the review result is traceable and auditable

This project adds a structured review layer before release. It does not execute production deployments.

## Current MVP

The current repository contains a runnable Phase 1 API skeleton with:

- review creation
- review detail query
- timeline query
- human decision submission
- retry for failed reviews
- evaluation feedback persistence
- review export
- aggregate evaluation metrics

The review pipeline is currently a fixed MVP DAG with heuristic rules. It does not yet integrate with real Git providers, incident systems, runbooks, or live metrics.

## GitHub Use Case

One intended direction for this project is to serve as the backend for GitHub pull request risk review:

- receive PR-related metadata from GitHub
- normalize it into a review request
- generate structured risk output
- escalate high-risk changes to human approval

The API already supports `pull_request` as a `source_type`, so a GitHub webhook adapter can be added on top of the current service without changing the core review contract.

## API

Implemented endpoints:

- `POST /api/v1/reviews`
- `GET /api/v1/reviews/{review_id}`
- `GET /api/v1/reviews/{review_id}/timeline`
- `POST /api/v1/reviews/{review_id}/human-decision`
- `POST /api/v1/reviews/{review_id}/retry`
- `POST /api/v1/reviews/{review_id}/evaluation`
- `GET /api/v1/reviews/{review_id}/export`
- `GET /api/v1/evaluations/metrics`

The API contract is defined in [openapi/openapi.yaml](E:/project/AI_project/Agentic-Change-Review-Copilot/openapi/openapi.yaml).

## Quick Start

Run locally with Docker Compose:

```bash
docker compose up --build
```

The API will be available at:

```text
http://localhost:8080
```

Key environment variables:

- `PORT`: HTTP listen port, defaults to `8080`
- `DATABASE_URL`: when set, the service uses PostgreSQL instead of the in-memory store
- `AUTO_MIGRATE=1`: runs `migrations/*.up.sql` on startup

## Example Request

```bash
curl -X POST http://localhost:8080/api/v1/reviews \
  -H "Content-Type: application/json" \
  -d '{
    "source_type": "pull_request",
    "source_id": "PR-123",
    "repo": "gateway-service",
    "service": "api-gateway",
    "environment": "prod",
    "payload": {
      "title": "adjust auth routing",
      "author": "alice",
      "base_commit": "abc123",
      "head_commit": "def456",
      "metadata": {
        "file_list": ["configs/routes.yaml", "gateway/auth.go"]
      }
    }
  }'
```

## Storage

- in-memory storage by default for local development
- PostgreSQL when `DATABASE_URL` is configured
- SQL schema migrations under [migrations](E:/project/AI_project/Agentic-Change-Review-Copilot/migrations)

## Project Layout

```text
cmd/review-api/         API entrypoint
internal/api/           HTTP handlers and tests
internal/review/        review domain logic and storage
migrations/             SQL migrations
openapi/                OpenAPI contract
docs/                   product spec, design, progress, local development
```

## Documentation

- [docs/spec.md](E:/project/AI_project/Agentic-Change-Review-Copilot/docs/spec.md)
- [docs/agentic_change_review_copilot_design.md](E:/project/AI_project/Agentic-Change-Review-Copilot/docs/agentic_change_review_copilot_design.md)
- [docs/progress.md](E:/project/AI_project/Agentic-Change-Review-Copilot/docs/progress.md)
- [docs/local-development.md](E:/project/AI_project/Agentic-Change-Review-Copilot/docs/local-development.md)

## Status

This repository is currently a runnable architecture prototype. It already includes:

- a working Go API
- PostgreSQL-backed persistence
- SQL migrations
- OpenAPI documentation
- handler and service tests
- Docker-based local startup

Still missing:

- real GitHub webhook ingestion
- real evidence retrieval from incidents, runbooks, and metrics
- authentication and authorization
- review console frontend
- PostgreSQL integration tests
- OpenAPI contract validation in CI

## Next Steps

- add a GitHub webhook adapter for PR-triggered review creation
- replace heuristic-only logic with pluggable rule modules
- connect evidence sources such as incidents, runbooks, and metrics
- add PostgreSQL integration coverage
- improve the GitHub landing page with badges, diagrams, and example responses
