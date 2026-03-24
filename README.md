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
- query task detail with generated execution plan
- query workflow timeline and structured trace events
- retry tasks that need manual triage
- export Markdown or JSON reports
- query workflow metrics
- store data in memory or PostgreSQL

The workflow is still mostly heuristic, but it now supports metadata-driven live HTTP probe execution, sequential request chaining, extracted workflow variables, cookie-backed session reuse, per-request timeout, latency, and retry controls, and payload, schema, and cookie assertions as an optional path.

## API

Implemented endpoints:

- `POST /api/v1/test-tasks`
- `GET /api/v1/test-tasks/{task_id}`
- `GET /api/v1/test-tasks/{task_id}/timeline`
- `POST /api/v1/test-tasks/{task_id}/retry`
- `GET /api/v1/test-tasks/{task_id}/report`
- `GET /api/v1/test-metrics`
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

## CLI Adapter

A small local CLI now lives under `cmd/testflow-cli/` and talks to the active HTTP API.

Examples:

```bash
go run ./cmd/testflow-cli create -file request.json
go run ./cmd/testflow-cli create -file request.json -wait
go run ./cmd/testflow-cli get -task-id tsk_123
go run ./cmd/testflow-cli report -task-id tsk_123 -format markdown
```

By default the CLI targets `http://localhost:8080`. You can override that with `-base-url` or `TESTFLOW_API_BASE_URL`.

## GitHub Webhook Adapter

A narrow GitHub PR webhook endpoint now exists for `pull_request` events:

```bash
curl -X POST http://localhost:8080/api/v1/webhooks/github/pull-request \
  -H "Content-Type: application/json" \
  -H "X-GitHub-Event: pull_request" \
  -d '{
    "action": "synchronize",
    "number": 42,
    "repository": {
      "name": "gateway-service",
      "full_name": "octo/gateway-service"
    },
    "pull_request": {
      "number": 42,
      "title": "adjust auth routing",
      "body": "route auth traffic through gateway",
      "head": {
        "ref": "feature/auth-routing",
        "sha": "def456"
      },
      "base": {
        "ref": "main",
        "sha": "abc123"
      },
      "user": {
        "login": "alice"
      }
    },
    "sender": {
      "login": "alice"
    }
  }'
```

The adapter currently accepts `opened`, `reopened`, `synchronize`, and `ready_for_review`, maps them into `CreateTestTaskRequest`, and intentionally ignores unsupported PR actions with an `accepted=false` response.

When `GITHUB_WEBHOOK_SECRET` is configured on the server, the same endpoint also requires GitHub's `X-Hub-Signature-256` header and rejects missing or invalid signatures.

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

## Optional Live HTTP Probe Mode

When `payload.metadata` includes `api_base_url` and a `requests` list, the workflow runs real HTTP requests in sequence instead of the heuristic executor.

Supported metadata keys:

- `api_base_url`: target service base URL
- `variables`: initial workflow variables available to later request templates
- `default_headers`: shared headers merged into every request
- `default_cookies`: shared cookies merged into every request
- `requests[].headers`: per-request headers
- `requests[].cookies`: per-request cookies
- `requests[].query`: per-request query parameters
- `requests[].body`: string or JSON body payload
- `requests[].timeout_ms`: hard per-request timeout budget in milliseconds
- `requests[].max_duration_ms`: soft latency assertion budget in milliseconds
- `requests[].max_attempts`: retry budget for the request
- `requests[].expect_status`: expected HTTP status code
- `requests[].expect_headers`: exact response header assertions
- `requests[].expect_headers_absent`: response headers that must be absent
- `requests[].expect_cookies`: expected response cookie values
- `requests[].expect_cookies_absent`: response cookies that must be absent
- `requests[].expect_body_contains`: required response-body substrings
- `requests[].expect_body_not_contains`: response-body substrings that must be absent
- `requests[].expect_json_present`: required dotted JSON paths
- `requests[].expect_json_absent`: dotted JSON paths that must be absent
- `requests[].expect_json`: expected JSON values keyed by dotted response paths
- `requests[].expect_json_types`: expected JSON types keyed by dotted response paths
- `requests[].extract`: response JSON paths captured into workflow variables for later requests
- `requests[].extract_headers`: response headers captured into workflow variables for later requests
- `requests[].extract_cookies`: response cookies captured into workflow variables for later requests

```json
{
  "payload": {
    "metadata": {
      "api_base_url": "http://localhost:8080",
      "variables": {
        "username": "alice"
      },
      "default_cookies": {
        "locale": "en-US"
      },
      "requests": [
        {
          "name": "login",
          "method": "POST",
          "path": "/login",
          "headers": {
            "Content-Type": "application/json"
          },
          "cookies": {
            "flow": "login"
          },
          "body": {
            "user": "{{username}}"
          },
          "timeout_ms": 2000,
          "max_duration_ms": 250,
          "max_attempts": 2,
          "expect_status": 200,
          "expect_headers": {
            "Content-Type": "application/json"
          },
          "expect_json_present": ["token"],
          "expect_json": {
            "token": "abc123"
          },
          "expect_json_types": {
            "token": "string"
          },
          "extract": {
            "auth_token": "token"
          },
          "extract_headers": {
            "request_id": "X-Request-Id"
          },
          "expect_cookies": {
            "session_id": "abc123"
          },
          "extract_cookies": {
            "session_cookie": "session_id"
          }
        },
        {
          "name": "profile",
          "method": "GET",
          "path": "/profile",
          "timeout_ms": 2000,
          "max_duration_ms": 250,
          "headers": {
            "Authorization": "Bearer {{auth_token}}",
            "X-Session-Mirror": "{{session_cookie}}"
          },
          "query": {
            "trace_id": "{{request_id}}"
          },
          "expect_status": 200,
          "expect_headers_absent": ["X-Debug-Trace"],
          "expect_body_contains": ["admin"],
          "expect_body_not_contains": ["stacktrace"],
          "expect_json_present": ["user.id"],
          "expect_json_absent": ["debug.trace"],
          "expect_json": {
            "status": "ok",
            "user.role": "admin"
          }
        }
      ]
    }
  }
}
```

Each configured request is converted into a generated test case with `tool_name=api_test_runner_http`, and later requests can reuse values extracted from earlier JSON payloads, response headers, or response cookies in subsequent headers, cookies, paths, query parameters, or bodies. Successful responses also seed a simple in-memory session cookie store for later requests in the same workflow. Requests can now also declare per-step timeout budgets, latency budgets, and retry counts when probing unstable services. Task detail responses now also include a derived `workflow` view for runtime stages and tools, an `execution_plan` view that summarizes planned tools, targets, assertions, variable references, and expected artifacts, plus `trace_events` that capture state transitions, planned steps, tool results, assertion outcomes, failure analysis, and final report milestones. Execution artifacts now include request summaries, response snippets, assertion evidence, cookie/session traces, retry/timeout details, and extracted-variable traces with basic redaction for sensitive keys such as authorization headers, tokens, and cookies. Failure analysis now consumes those artifacts directly and emits explicit `result:` / `artifact:` evidence references in task detail and reports.

## Storage

- in-memory storage by default for local development
- PostgreSQL when `DATABASE_URL` is configured
- SQL schema migrations under [migrations](migrations)

## Project Layout

```text
cmd/review-api/         API entrypoint
cmd/testflow-cli/       Local CLI adapter for create/get/report flows
.github/workflows/      CI workflows
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

- broader API runner coverage beyond the current sequential HTTP path
- smarter assertion generation and schema validation
- failure analysis grounded in logs, traces, and artifacts
- webhook adapters and richer PR-source ingestion adapters

The repository now includes a GitHub Actions workflow that runs unit tests, PostgreSQL-backed integration tests, and `go build` on pushes to `main` and on pull requests.
