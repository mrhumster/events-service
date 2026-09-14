# events-service

GoCast user activity feed service. Records events about streams and the
account (`stream.created`, `stream.published`, `user.login`, ...) and exposes
a personal REST feed per user with unread tracking.

Two binaries in one image:

- **`events-server`** — REST reader (`GET /events`, unread count, mark read).
- **`events-worker`** — asynq worker consuming the `events` queue (Redis **DB 3**)
  and persisting into Postgres.

Built on the same skeleton as the other GoCast workers:

- `github.com/mrhumster/go-shared` (`worker`, `config`, `metrics`) via `replace => ../shared`
- asynq queue `events` (Redis DB 3, KEDA `asynq:{events}:pending/active`, listLength 2)
- Prometheus `/metrics` on 9090 (liveness probe)

Producers (other services) enqueue the task `event:activity` after each
relevant change:

| Producer | Events |
| --- | --- |
| identity-service | `user.registered`, `user.login`, `user.email.verified` |
| stream-service | `stream.created`, `stream.upload.started`, `stream.upload.completed`, `stream.transcode.started`, `stream.transcode.finish`, `stream.transcode.failed`, `stream.ready`, `stream.published`, `stream.unpublished`, `stream.deleted`, `stream.reprocessed` |

## Wire contract

Task `event:activity`, queue `events`, JSON payload:

```json
{
  "event_id": "uuid",       // client-generated, idempotency key
  "user_id": "uuid",
  "event_type": "stream.ready",
  "stream_id": "uuid",      // optional
  "payload": {},            // optional free-form map
  "occurred_at": "RFC3339"
}
```

Idempotency: unique index on `activity_events.event_id`, INSERT
`ON CONFLICT DO NOTHING` — re-delivered tasks are dropped safely.

## REST API

All endpoints require `Authorization: Bearer <JWT>` (public RSA key fetched
from `JWT_ACCESS_PUBLIC_KEY_URL`). Cursor is an RFC3339 `created_at`; the feed
is newest-first.

| Method | Path | Description |
| --- | --- | --- |
| GET | `/events?limit&cursor` | Personal feed (default 50, max 100). Returns `next_cursor` when the page is full. |
| GET | `/events/unread-count` | `{"count": N}` unread events |
| POST | `/events/:id/read` | Mark one event read (owned only) |
| POST | `/events/read-all` | Mark all read |
| GET | `/health` | Probe (DB check) |
| GET | `/metrics` | Prometheus metrics |

Feed item shape:

```json
{
  "id": "uuid",
  "event_id": "uuid",
  "user_id": "uuid",
  "event_type": "stream.ready",
  "stream_id": "uuid",
  "payload": {},
  "read_at": "RFC3339",
  "created_at": "RFC3339"
}
```

## Configuration

| Env | Default | Used by |
| --- | --- | --- |
| `SERVER_ADDR` | `:8080` | server |
| `MODE` | `debug` | gin mode |
| `CORS_ALLOW_ORIGINS` | `"http://localhost:5173,https://example.com,https://events.example.com"` | server CORS |
| `JWT_ACCESS_PUBLIC_KEY_URL` | — | server JWT verification |
| `DB_HOST` / `DB_PORT` / `DB_USER` / `DB_PASS` / `DB_NAME` | `localhost`/`5432`/`postgres`/`""`/`postgres` | both |
| `REDIS_ADDR` / `REDIS_PASS` | `localhost`/`""` | both |
| `REDIS_QUEUE_DB` | `3` | worker queue DB |
| `WORKER_CONCURRENCY` | `2` | worker |
| `WORKER_SHUTDOWN_TIMEOUT` | `50m` | worker |
| `METRICS_ADDR` | `""` (off) | worker metrics server |

## Deployment

Schema via `db-migrate`:

```bash
go run ./services/db-migrate -target=events
# or in cluster: job db-migrate-events (`make apply-db-migrate`)
```

Manifests in `deploy/k8s/`:

- `reader/` — Deployment (`args: ["server"]`, ClusterIP 8080) + probe `/health`
- `worker/` — Deployment (`args: ["worker"]`, metrics 9090) + KEDA-scaled
- `keda/` — ScaledObject `events-keda` (`asynq:{events}:pending/active`, DB 3)

Build context is `services/` (the Dockerfile copies `shared` beside the
module, resolving `replace => ../shared`):

```bash
make build && make push && make deploy
```