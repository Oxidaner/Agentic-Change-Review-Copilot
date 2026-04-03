# Agentic Change Review Copilot

This repository is a Go-based MVP for an agentic change-risk audit system.
Its primary workflow is:

`PR / SQL / K8s change -> change normalization -> context collection -> risk signal extraction -> scoring -> recommendation -> human decision -> evaluation feedback`

The system is scoped around release-review and change-audit scenarios first:

- Phase 1: PR-driven release review
- Phase 2: SQL and K8s change audit
- Phase 3: broader external tool integration and feedback-driven policy iteration

## Current MVP

The current repository exposes a runnable backend for the review-first direction:

- create change reviews
- query review detail with risk signals, recommendation, rollback plan, and evidence
- query review detail with hybrid-analysis output alongside rule-based signals
- query workflow timeline and audit trace history
- submit human approval / rejection / override decisions
- retry failed reviews
- export Markdown or JSON reports
- submit post-release evaluation feedback
- query evaluation metrics
- store data in memory or PostgreSQL
- ingest GitHub `pull_request` webhooks

The current review pipeline is still mostly heuristic, but it already supports:

- normalized change bundles for PR-style inputs
- rule-based risk signal extraction across PR, SQL, K8s, gateway, and production-change patterns
- a pluggable hybrid-analysis stage with a default heuristic fallback analyzer and future LLM integration seam
- structured evidence packing
- human-review gating for high-risk or low-confidence decisions
- recommendation and rollback-plan generation
- audit timeline persistence
- basic feedback metrics such as override rate and false-positive rate

The repository still contains the newer `testflow` exploration under `internal/testflow`, but the active HTTP entrypoint is now the review domain again.

## API

Implemented endpoints:

- `POST /api/v1/reviews`
- `GET /api/v1/reviews/{review_id}`
- `GET /api/v1/reviews/{review_id}/timeline`
- `POST /api/v1/reviews/{review_id}/decision`
- `POST /api/v1/reviews/{review_id}/retry`
- `GET /api/v1/reviews/{review_id}/report`
- `POST /api/v1/reviews/{review_id}/evaluation`
- `GET /api/v1/review-metrics`
- `POST /api/v1/webhooks/github/pull-request`

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
- `GITHUB_WEBHOOK_SECRET`: when set, the GitHub webhook endpoint requires a valid `X-Hub-Signature-256` HMAC signature

## Example Review Request

```bash
curl -X POST http://localhost:8080/api/v1/reviews \
  -H "Content-Type: application/json" \
  -d '{
    "source_type": "pull_request",
    "source_id": "octo/gateway-service#42",
    "repo": "octo/gateway-service",
    "service": "gateway-service",
    "environment": "prod",
    "payload": {
      "title": "adjust auth routing",
      "author": "alice",
      "base_commit": "abc123",
      "head_commit": "def456",
      "diff_url": "https://github.com/octo/gateway-service/pull/42.diff",
      "metadata": {
        "file_list": ["configs/routes.yaml", "deploy/ingress.yaml"],
        "cmdb": {
          "service_name": "gateway-service",
          "service_tier": "tier-1",
          "owner": "traffic-platform"
        },
        "metrics": {
          "summary": "latency_p95 elevated over last 15m",
          "window": "15m",
          "latency_p95": "420ms",
          "error_rate": "0.3%"
        },
        "runbook": {
          "title": "Gateway rollback playbook",
          "url": "https://runbooks.example.com/gateway/rollback"
        }
      }
    }
  }'
```

## GitHub Webhook Adapter

A narrow GitHub PR webhook adapter is available at `POST /api/v1/webhooks/github/pull-request`.

Supported actions:

- `opened`
- `reopened`
- `synchronize`
- `ready_for_review`

The adapter maps supported PR events into `CreateReviewRequest` payloads, infers a coarse target environment from the PR base branch, and intentionally ignores unsupported actions with an `accepted=false` response.

When `GITHUB_WEBHOOK_SECRET` is configured on the server, the endpoint also requires GitHub's `X-Hub-Signature-256` header.

## Evaluation Feedback

The review pipeline already supports post-release outcome feedback:

- `release_outcome`
- `incident_flag`
- `outcome_metadata`

This feedback is persisted and contributes to evaluation metrics such as:

- `review_count`
- `high_risk_recall`
- `false_positive_rate`
- `override_rate`
- `p95_latency_ms`

## Local Checks

```bash
go test ./...
go build ./...
```

## Notes

- `internal/review` is the active product surface.
- `internal/testflow` remains in-tree as a secondary exploration and reusable workflow runtime reference.
- External context metadata attached to evidence is allowlisted before persistence to avoid leaking secrets or internal-only payloads.
- The `analysis` field in review responses is currently backed by the new hybrid-analysis skeleton and is ready for a future real LLM adapter.
