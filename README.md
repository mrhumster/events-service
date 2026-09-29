# events-service

GoCast user activity feed service. Records events about streams, the account
and interactions (`stream.created`, `stream.published`, `user.login`,
`comment.created`, `reaction.liked`, ...) and exposes a personal REST feed per
user with unread tracking.

Two binaries in one image:

- **`events-server`** — REST reader (`GET /events`, unread count, mark read).
- **`events-worker`** — asynq worker consuming the `events` queue (Redis **DB 3**)
  and persisting into Postgres.

Built on the same skeleton as the other GoCast workers:

- `github.com/mrhumster/go-shared` (`worker`, `config`, `metrics`) via `replace => ../shared`
- asynq queue `events` (Redis DB 3, KEDA `asynq:{events}:pending/active`, listLength 2)
- Prometheus `/metrics` on 9090 (liveness probe)

There is **no** `AutoMigrate` in the service — the schema is owned by
`db-migrate` (target `events`, table `schema_migrations_events`).

Producers (other services) enqueue the task `event:activity` after each
relevant change:

| Producer | Events |
| --- | --- |
| identity-service | `user.registered`, `user.login`, `user.email.verified` |
| stream-service | `stream.created`, `stream.upload.started`, `stream.upload.completed`, `stream.transcode.started`, `stream.transcode.finish`, `stream.transcode.failed`, `stream.ready`, `stream.published`, `stream.unpublished`, `stream.deleted`, `stream.reprocessed` |
| comments-service | `comment.created`, `comment.replied` |
| stats-service | `reaction.liked`, `reaction.disliked` |

### Event types

The worker accepts only these 18 whitelisted types (`ValidEventType`); anything
else is rejected as an invalid event and **not** retried:

- Stream lifecycle (11): `stream.created`, `stream.upload.started`,
  `stream.upload.completed`, `stream.transcode.started`,
  `stream.transcode.finish`, `stream.transcode.failed`, `stream.ready`,
  `stream.published`, `stream.unpublished`, `stream.deleted`,
  `stream.reprocessed`
- Account (3): `user.registered`, `user.login`, `user.email.verified`
- Comments (2): `comment.created`, `comment.replied`
- Reactions (2): `reaction.liked`, `reaction.disliked`

The feed is a **personal** timeline: `user_id` is the *owner of the timeline*
the event belongs to, not necessarily the actor. Producers decide whose feed an
event lands in — e.g. stats-service attributes a reaction to the stream owner and
carries the actor in `payload.actor_email`.

## Wire contract

Task `event:activity`, queue `events`, priority `6`, JSON payload:

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

Processing rules in the worker:

- `event_id` and `user_id` must parse as UUIDs, `event_type` must be
  whitelisted. A missing/zero `event_id` is **not** an error — the worker
  generates one (`uuid.New()`), despite the code comment suggesting otherwise.
- `payload` empty/absent is stored as `{}`; `stream_id` may be null.
- `created_at` defaults to the current UTC time when `occurred_at` is absent.

Idempotency: unique index on `activity_events.event_id`, INSERT
`ON CONFLICT DO NOTHING` — re-delivered tasks are dropped safely.

Retry behaviour: malformed JSON payloads and invalid events are wrapped in
`asynq.SkipRetry` (one attempt, no retry) and counted with
`events_recorded_total{status="error"}`. A database failure is a normal error
and is retried by asynq.

## REST API

All `/events` endpoints require `Authorization: Bearer <JWT>` (public RSA key
fetched from `JWT_ACCESS_PUBLIC_KEY_URL`). Cursor is an RFC3339 `created_at`; the
feed is newest-first.

| Method | Path | Description |
| --- | --- | --- |
| GET | `/events?limit&cursor` | Personal feed (default 50, max 100). Returns `next_cursor` when the page is full. |
| GET | `/events/unread-count` | `{"count": N}` unread events |
| POST | `/events/:id/read` | Mark one event read (owned only) |
| POST | `/events/read-all` | Mark all read |
| GET | `/health` | Probe (DB check) |
| GET | `/metrics` | Prometheus metrics |

Write endpoints take the JWT only via the header (no `?token=` fallback). The
RSA public key is fetched **once at startup** (10s timeout) from
`JWT_ACCESS_PUBLIC_KEY_URL` — a failure there is fatal, and the key is not
refreshed at runtime.

### Pagination

Keyset (not offset) pagination on `created_at`, newest-first:

- `limit` defaults to `50`; values `> 100` are silently clamped to `100` in the
  repository, values `< 1` or non-numeric are rejected with `400 invalid limit`.
- `cursor` is the RFC3339 `created_at` of the **last item of the previous
  page**; the next page is `WHERE created_at < cursor`. Malformed cursors give
  `400 invalid cursor`.
- `next_cursor` is present only when the page is full (`len(events) >= limit`),
  so an absent cursor means the end of the feed.
- Note: the cursor is timestamp-only. If several events share an identical
  `created_at` (or a `limit` splits a tie), events on the boundary can be
  skipped or repeated across pages.

Feed item shape (`read_at` and `next_cursor` are omitted when empty):

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

### Read semantics

- `GET /events/unread-count` counts rows with `read_at IS NULL` for the caller.
- `POST /events/:id/read` is a **scoped `UPDATE`** (`id` + caller's `user_id`).
  It returns `200 {"ok":true}` even when the event does not exist or belongs to
  another user, so it is not an existence oracle; it also overwrites an already
  set `read_at`. Only a malformed `:id` gives `400 invalid event id`.
- `POST /events/read-all` stamps `read_at` on every unread event of the caller.

### Errors

| Status | Body | Cause |
| --- | --- | --- |
| 400 | `{"error":"invalid limit"}` | `limit` non-numeric or `< 1` |
| 400 | `{"error":"invalid cursor"}` | malformed RFC3339 `cursor` |
| 400 | `{"error":"invalid event id"}` | `:id` is not a UUID |
| 401 | `{"error":"auth token required"}` | missing `Authorization` header |
| 401 | `{"error":"invalid token"}` | signature/claims failure, or JWT `user_id` not a UUID |
| 429 | `{"error":"too many requests"}` | read rate limit exceeded (see below) |
| 500 | `{"error":"internal server error"}` | reader/storage failure |

`/health` returns `503 {"status":"down","error":...}` when the Postgres ping
fails, otherwise `200 {"status":"up"}`.

### Rate limiting

The read endpoints (`/events`, `/events/unread-count`, and the two read POSTs)
sit behind an in-memory fixed-window limiter:

- `EVENTS_READ_RATE_LIMIT` requests per minute, **default `120`**.
- Keyed by the authenticated **user ID** (from the JWT), so it cannot be
  spoofed and is not shared across users.
- Overflow returns `429 {"error":"too many requests"}`.

## Configuration

| Env | Default | Used by |
| --- | --- | --- |
| `SERVER_ADDR` | `:8080` | server |
| `MODE` | `debug` | gin mode |
| `CORS_ALLOW_ORIGINS` | `"http://localhost:5173,https://example.com,https://events.example.com"` | server CORS |
| `JWT_ACCESS_PUBLIC_KEY_URL` | — | server JWT verification |
| `EVENTS_READ_RATE_LIMIT` | `120` | server read rate limit (per user, per minute) |
| `DB_HOST` / `DB_PORT` / `DB_USER` / `DB_PASS` / `DB_NAME` | `localhost`/`5432`/`postgres`/`""`/`postgres` | both |
| `REDIS_ADDR` / `REDIS_PASS` | `localhost`/`""` | both |
| `REDIS_QUEUE_DB` | `3` | worker queue DB |
| `WORKER_CONCURRENCY` | `2` | worker |
| `WORKER_SHUTDOWN_TIMEOUT` | `50m` | worker |
| `METRICS_ADDR` | `""` (off) | worker metrics server |

The Postgres DSN hardcodes `sslmode=disable` and `TimeZone=UTC` (in-cluster
connection over the private network).

CORS on the reader allows `GET`, `POST`, `OPTIONS`, headers `Content-Type` and
`Authorization`, with credentials, for the configured origins. The REST server
uses read/write `15s` and idle `60s` timeouts, and a `30s` graceful-shutdown
context.

## Metrics

- **Reader** — Prometheus `/metrics` on the REST port `8080` (scrape
  annotations `prometheus.io/scrape|port=8080|path=/metrics`), exposing RED
  metrics `http_requests_total{method,endpoint,status}` and
  `http_request_duration_seconds`.
- **Worker** — Prometheus `/metrics` on `METRICS_ADDR` (port `9090` in-cluster,
  also the liveness probe), exposing `events_recorded_total{status}`,
  `events_record_duration_seconds`, and the shared `asynq_task_*` metrics.

## Deployment

Schema via `db-migrate`:

```bash
go run ./services/db-migrate -target=events
# or in cluster: job db-migrate-events (`make apply-db-migrate`)
```

Manifests in `deploy/k8s/`:

- `reader/` — Deployment (`args: ["server"]`, ClusterIP 8080) + probes `/health`
- `worker/` — Deployment (`args: ["worker"]`, metrics 9090) + KEDA-scaled
- `keda/` — ScaledObject `events-keda` (`asynq:{events}:pending/active`, DB 3,
  `minReplicaCount 0`, `maxReplicaCount 1`, `cooldownPeriod 120`, `listLength 2`,
  `keda-redis-auth`)

Runtime wiring:

- Reader takes `events-service-config` + `DB_USER`/`DB_PASS` from
  `go-app-secret`.
- Worker additionally takes `REDIS_PASS` from `casbin-redis` (key
  `redis-password`).

The Dockerfile is a multi-stage build (`golang:1.25.14-alpine` builder,
`alpine:3.18` runtime, non-root `appuser` UID `1000`) with an entrypoint
dispatching `server` / `worker`; `CMD ["server"]` is the default.

Build context is `services/` (the Dockerfile copies `shared` beside the
module, resolving `replace => ../shared`):

```bash
make build && make push && make deploy
```

`make test` runs `go test ./...`. `make deploy` sets the image on both
deployments and waits for the `events-reader` rollout only.

Key operational commands:

```bash
kubectl get so events-keda -n go-app                              # KEDA scaling status
kubectl logs -n go-app -l app=events-worker --tail=50              # worker processing/errors
kubectl get pods -n go-app -l app=events-reader                    # reader pods
```
