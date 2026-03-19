# Progress

## Current Positioning

The repository is now positioned as:

`基于 Workflow Agent 的 AI 自动化测试平台`

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

Current reusable pieces:
- Go HTTP API skeleton
- controlled workflow / state-machine style service flow
- PostgreSQL persistence
- Docker local startup
- OpenAPI contract
- basic automated tests

## Final Product Direction

The final product direction is fixed as one clear main line:

`PR / 需求变更 -> 测试点抽取 -> 用例生成 -> 工具执行 -> 智能断言 -> 失败归因 -> 测试报告`

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
- `internal/api/handler.go`
- `internal/api/handler_test.go`
- `internal/testflow/service.go`
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
1. Integrate a real API test runner instead of heuristic execution.
2. Add PostgreSQL integration tests for the new `test_tasks` lifecycle.
3. Add smarter assertion generation and failure analysis grounded in real artifacts.

Second priority:
1. Add a CLI or webhook adapter for PR-triggered task creation.
2. Decide whether to delete or archive `internal/review` after the migration stabilizes.
3. Make runtime abstractions more explicit: `Workflow`, `Step`, `Tool`, `ModelAdapter`, `TraceEvent`.

## Resume Checklist

If resuming on another machine:
1. `git pull`
2. `docker compose up --build`
3. `go test ./...`
4. Read `docs/workflow_agent_ai_test_platform_design.md`
5. Continue from real tool integration or PostgreSQL integration tests

## Notes

- `.idea/` is still untracked and intentionally untouched.
- The codebase is no longer in the old "docs switched but API did not" state.
- Remaining migration work is now mostly about deleting legacy code and replacing simulated execution with real tools.
