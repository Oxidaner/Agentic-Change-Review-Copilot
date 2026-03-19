# Progress

## Current Positioning

The repository is now positioned as:

`基于 Workflow Agent 的 AI 自动化测试平台`

This should be understood as:
- a workflow-first AI test automation runtime
- an engineering platform skeleton, not a single prompt demo
- a controlled agent workflow embedded into real test processes

## Current State

The current codebase still contains review-oriented domain naming, but it already has reusable infrastructure that can be migrated into the test platform direction.

Reusable existing pieces:
- Go HTTP API skeleton
- controlled workflow / state-machine style service flow
- PostgreSQL persistence
- evaluation feedback persistence
- metrics API pattern
- Docker local startup
- OpenAPI contract workflow
- basic automated tests

What is not migrated yet:
- review domain model -> test task domain model
- review API semantics -> test workflow API semantics
- risk signal model -> test point / assertion / failure analysis model
- actual test tool integrations

## Final Product Direction

The final product direction is now fixed as one clear main line:

`PR / 需求变更 -> 测试点抽取 -> 用例生成 -> 工具执行 -> 智能断言 -> 失败归因 -> 测试报告`

This means future implementation work should optimize for:
- API regression testing first
- Web UI core-path testing second
- controlled workflow rather than free-form agent planning

## Important Files

Design and handoff:
- `docs/workflow_agent_ai_test_platform_design.md`
- `docs/spec.md`
- `docs/local-development.md`

Current runtime and API skeleton:
- `cmd/review-api/main.go`
- `internal/api/handler.go`
- `internal/api/handler_test.go`
- `internal/review/service.go`
- `internal/review/types.go`
- `internal/review/postgres.go`
- `internal/review/store.go`

Contract and schema:
- `openapi/openapi.yaml`
- `migrations/*.sql`

## Major Completed Commits Before The Narrative Switch

- `ca387dd` `feat: add review api skeleton and postgres store`
- `93e043a` `feat: persist human decisions`
- `53061f4` `chore: add local development setup`
- `d77c182` `feat: persist evaluation records`
- `f4740a5` `test: add api integration coverage`
- `d1d17b1` `feat: add evaluation feedback endpoint`
- `06db8e9` `docs: align openapi with evaluation endpoint`
- `85d0a0a` `docs: add progress handoff notes`
- `849d64d` `docs: refocus architecture narrative`
- `1286fc2` `docs: sync documentation narrative`

These commits built the reusable backend skeleton, but they still use review-oriented naming.

## What Should Happen Next

Highest priority:
1. Rename and migrate the domain model from review semantics to AI test workflow semantics.
2. Rewrite `openapi/openapi.yaml` around test workflow endpoints.
3. Keep the implementation scope narrow: API regression test workflow first.

Second priority:
1. Make runtime abstractions explicit: `Workflow`, `Step`, `Tool`, `ModelAdapter`, `TraceEvent`.
2. Add PostgreSQL integration tests for the new test-task lifecycle.
3. Add CLI output for JSON and Markdown test reports.

## Resume Checklist

If resuming on another machine:
1. `git pull`
2. `docker compose up --build`
3. `go test ./...`
4. Read `docs/workflow_agent_ai_test_platform_design.md`
5. Continue with domain model migration and OpenAPI rewrite

## Notes

- The code and docs are temporarily in a transition period.
- The docs now describe the final target direction.
- The code still reflects the previous review-oriented MVP skeleton and should be migrated incrementally, not thrown away.
