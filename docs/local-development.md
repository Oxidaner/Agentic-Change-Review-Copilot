# Local Development

## What You Are Running

This repository currently runs the MVP backend skeleton for:

`Evidence-Grounded Change Review Copilot`

In the current implementation, that means:
- a controlled review workflow
- review task persistence
- human review feedback
- evaluation feedback
- export and metrics APIs

## Start With Docker Compose

```bash
docker compose up --build
```

The API will be available at `http://localhost:8080`.

## Key Environment Variables

- `PORT`: HTTP listen port, defaults to `8080`
- `DATABASE_URL`: when set, the API uses PostgreSQL instead of the in-memory store
- `AUTO_MIGRATE=1`: runs the SQL files under `migrations/` on startup

## Recommended Local Checks

```bash
go test ./...
go build ./...
```

## Current MVP Scope

The current product narrative is now:
- primary scenario: engineering change review
- Phase 1 focus: `PR diff + K8s/YAML`
- runtime style: controlled workflow, not free-form agent

The code skeleton is broader than that narrative in a few places, but future implementation should follow this narrower scope first.

## Example Create Review Request

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

## Example Human Decision Request

```bash
curl -X POST http://localhost:8080/api/v1/reviews/<review_id>/human-decision \
  -H "Content-Type: application/json" \
  -d '{
    "reviewer": "bob",
    "decision": "override",
    "reason": "approved during low-traffic window"
  }'
```

## Example Evaluation Feedback Request

```bash
curl -X POST http://localhost:8080/api/v1/reviews/<review_id>/evaluation \
  -H "Content-Type: application/json" \
  -d '{
    "release_outcome": "success",
    "incident_flag": false,
    "outcome_metadata": {
      "release_id": "rel-001"
    }
  }'
```

## Local Development Notes

- If you want the fastest path, keep using the backend API and test flow first.
- Do not expand into too many scenarios before `PR diff + K8s/YAML` is truly solid.
- Keep OpenAPI, docs, and implementation in sync whenever endpoints change.
