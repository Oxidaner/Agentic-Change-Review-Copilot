# AGENTS.md

## Project Overview
- This repository is a Go-based MVP for an agentic change-risk audit system.
- The active product surface is the `review` API.
- Treat release review and change audit as the primary product direction.

## Repository Layout
- `cmd/review-api`: current HTTP server entrypoint.
- `internal/api`: HTTP handlers and transport-layer tests for the review API.
- `internal/review`: active review pipeline, storage, webhook mapping, analysis, and evaluation logic.
- `openapi/openapi.yaml`: API contract source of truth.
- `migrations/`: PostgreSQL schema migrations.
- `docs/`: product, workflow, progress, and local development notes.

## Working Guidelines
- Prefer small, surgical changes that preserve the current architecture.
- Keep transport concerns in `internal/api` and domain logic in `internal/review`.
- Update the OpenAPI spec and docs when changing externally visible API behavior.
- Preserve compatibility with both in-memory storage and PostgreSQL-backed flows when touching persistence logic.
- Keep the review workflow centered on risk identification, evidence collection, recommendation, human decision, and evaluation feedback.

## Development Commands
- Run tests: `go test ./...`
- Build all packages: `go build ./...`
- Run API locally: `docker compose up --build`

## Testing Expectations
- Start with focused package tests for the code you changed, then run broader `go test ./...` when practical.
- If a change affects HTTP contracts, cover both handler behavior and domain/service behavior when applicable.
- Do not enable PostgreSQL-only tests unless the environment is configured for them.

## Implementation Notes
- The default local API base URL is `http://localhost:8080`.
- `AUTO_MIGRATE=1` applies SQL migrations on startup.
- `GITHUB_WEBHOOK_SECRET` enables HMAC verification for the GitHub PR webhook endpoint.
- The primary workflow is `PR / SQL / K8s change -> context collection -> hybrid analysis -> recommendation -> human decision -> evaluation feedback`.

## Documentation To Check First
- `README.md`
- `docs/local-development.md`
- `docs/spec.md`
- `openapi/openapi.yaml`
