# Local Development

## Current Reality

The repository is now documented as:

`基于 Workflow Agent 的 AI 自动化测试平台`

However, the current implementation is still a backend skeleton inherited from the previous review-oriented MVP. That is expected during this transition phase.

Use the current runtime as:
- a reusable workflow backend base
- a persistence and API skeleton
- a starting point for migrating into the AI testing platform flow

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

## Final Product Scope

The final project scope is now fixed as:

`PR / 需求变更 -> 测试点抽取 -> 用例生成 -> 工具执行 -> 智能断言 -> 失败归因 -> 测试报告`

Near-term implementation priority:
- API regression testing first
- Web UI core-path testing second

## Current Development Strategy

Do not expand the current codebase in arbitrary directions.

Use this migration order:
1. migrate docs and API contract
2. migrate domain naming and workflow states
3. migrate persistence objects
4. integrate real test execution tools

## Current Example Startup Flow

```bash
docker compose up --build
go test ./...
go build ./...
```

## Important Note

The current HTTP endpoints and request examples still reflect the older review-oriented implementation. They should be treated as transitional scaffolding until the API contract is rewritten around the AI testing workflow.
