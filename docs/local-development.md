# Local Development

## Start with Docker Compose

```bash
docker compose up --build
```

The API will be available at `http://localhost:8080`.

## Key environment variables

- `PORT`: HTTP listen port, defaults to `8080`
- `DATABASE_URL`: when set, the API uses PostgreSQL instead of the in-memory store
- `AUTO_MIGRATE=1`: runs the SQL files under `migrations/` on startup

## Example create review request

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

## Example decision request

```bash
curl -X POST http://localhost:8080/api/v1/reviews/<review_id>/human-decision \
  -H "Content-Type: application/json" \
  -d '{
    "reviewer": "bob",
    "decision": "override",
    "reason": "approved during low-traffic window"
  }'
```
