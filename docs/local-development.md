# Local Development

## Current Reality

The repository now starts on the new testflow API surface.

Default endpoints:
- `POST /api/v1/test-tasks`
- `GET /api/v1/test-tasks/{task_id}`
- `GET /api/v1/test-tasks/{task_id}/timeline`
- `POST /api/v1/test-tasks/{task_id}/retry`
- `GET /api/v1/test-tasks/{task_id}/report`
- `GET /api/v1/test-metrics`

The old `internal/review` package still exists in-tree as legacy code, but it is not the active entrypoint anymore.

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

## Current Product Scope

Current main line:

`PR / 需求变更 -> 测试点抽取 -> 用例生成 -> 工具执行 -> 智能断言 -> 失败归因 -> 测试报告`

Near-term priority order:
- API regression testing first
- Web UI core-path testing second

## Current Development Strategy

Recommended build order from here:
1. integrate a real API test runner
2. add PostgreSQL integration tests for `test_tasks`
3. improve assertion generation and failure analysis
4. remove or archive legacy `internal/review` code when no longer needed

## Example Startup Flow

```bash
docker compose up --build
go test ./...
go build ./...
```

## Important Note

The service binary path is still `cmd/review-api/` for now, but the runtime behavior and API contract have already switched to the new testflow model.
