# crawlerv2

Distributed web crawler. One master server coordinates any number of stateless workers via HTTP polling. Workers authenticate with bearer tokens and are interchangeable — start as many as you need, anywhere on the internet.

Built with Go, DDD, GORM, SQLite (no CGO), chi, urfave/cli v3.

## Quick start

```bash
go build -o crawlerv2 ./cmd/app
```

### 1. Start the server

```bash
./crawlerv2 server --db crawl.db --addr :8080
```

On first run with no tokens, seed one via bootstrap:

```bash
./crawlerv2 server --db crawl.db --addr :8080 --bootstrap-token mysecret
```

Or create a token manually (server can be running):

```bash
./crawlerv2 token create --db crawl.db
# token id=1
# 3f9a2c... (save this — shown once)
```

### 2. Add a domain

```bash
curl -X POST http://localhost:8080/api/domains \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "host": "example.com",
    "scope": {
      "include_subdomains": true,
      "max_depth": 3,
      "max_pages_per_run": 1000
    },
    "politeness": {
      "respect_robots_txt": true,
      "crawl_delay_ms": 500,
      "max_concurrency_per_worker": 4,
      "max_concurrency_global": 20
    },
    "cron": "0 2 * * *"
  }'
```

`cron` is optional — omit for manual-only crawls.

### 3. Trigger a crawl job

```bash
curl -X POST http://localhost:8080/api/domains/1/jobs \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "extract": {
      "extract_title": true,
      "extract_meta": true,
      "extract_body": false
    }
  }'
# returns job with id
```

### 4. Start a worker

```bash
./crawlerv2 worker \
  --master http://your-server:8080 \
  --token <token> \
  --job-id 1 \
  --batch 10 \
  --concurrency 4 \
  --poll-interval 3s
```

Start as many workers as you want — on any machine with network access to the master.

---

## CLI reference

### `server`

```
./crawlerv2 server [flags]

Flags:
  --addr             Listen address          (env: HTTP_ADDR,           default: :8080)
  --db               SQLite database path    (env: DATABASE_PATH,       default: crawlerv2.db)
  --bootstrap-token  Seed token on first run (env: ADMIN_BOOTSTRAP_TOKEN)
  --stale-timeout    Reclaim stale tasks     (env: STALE_TASK_TIMEOUT,  default: 5m)
```

### `worker`

```
./crawlerv2 worker [flags]

Flags:
  --master        Master server URL   (env: MASTER_URL)        required
  --token         Bearer token        (env: WORKER_TOKEN)      required
  --job-id        Job ID to work on   (env: JOB_ID)            required
  --batch         URLs per poll       (env: WORKER_BATCH,      default: 10)
  --concurrency   Parallel fetches    (env: WORKER_CONCURRENCY, default: 4)
  --poll-interval Idle poll interval  (env: WORKER_POLL_INTERVAL, default: 5s)
```

### `token create`

```
./crawlerv2 token create --db crawl.db
```

Prints the plaintext token once. Store it — it cannot be retrieved again.

### `token revoke`

```
./crawlerv2 token revoke --db crawl.db --id <id>
```

---

## REST API

All endpoints require `Authorization: Bearer <token>`.

### Domains

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/domains` | Add a domain |
| `GET` | `/api/domains` | List all domains |
| `PUT` | `/api/domains/:id` | Update domain config |
| `DELETE` | `/api/domains/:id` | Remove a domain |

### Jobs

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/domains/:id/jobs` | Trigger a crawl job |
| `GET` | `/api/jobs` | List jobs (`?domain_id=1&status=running`) |
| `GET` | `/api/jobs/:id` | Job detail + pages crawled |

### Tokens

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/tokens` | Create token (returns plaintext once) |
| `GET` | `/api/tokens` | List tokens (no plaintext) |
| `DELETE` | `/api/tokens/:id` | Revoke token |

### Worker

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/worker/tasks/next` | Poll for URL batch (`?job_id=1&batch=10`) |
| `POST` | `/worker/jobs/:jobID/results` | Submit crawl results |

`GET /health` — unauthenticated health check.

---

## Domain config

### Scope

```json
{
  "include_subdomains": true,
  "path_filter": "^/blog/",
  "max_depth": 5,
  "max_pages_per_run": 500
}
```

| Field | Description |
|-------|-------------|
| `include_subdomains` | Crawl `*.example.com` in addition to `example.com` |
| `path_filter` | Regex — only enqueue URLs whose path matches |
| `max_depth` | Max hops from seed URL (0 = unlimited) |
| `max_pages_per_run` | Stop enqueuing after N pages per job (0 = unlimited) |

### Politeness

```json
{
  "respect_robots_txt": true,
  "crawl_delay_ms": 500,
  "max_concurrency_per_worker": 4,
  "max_concurrency_global": 20
}
```

| Field | Description |
|-------|-------------|
| `respect_robots_txt` | Parse and honor `robots.txt` |
| `crawl_delay_ms` | Minimum ms between requests per worker |
| `max_concurrency_per_worker` | Max parallel requests from a single worker |
| `max_concurrency_global` | Max in-flight requests across **all workers** for `*.domain.tld` |

### Scheduling

`"cron": "0 2 * * *"` — standard 5-field cron (runs at 02:00 daily). Empty string = manual only. Only one active job per domain runs at a time.

---

## Architecture

```
┌─────────────────────────────────────┐
│            Master server            │
│  ┌──────────┐  ┌──────────────────┐ │
│  │ HTTP API │  │    Scheduler     │ │
│  │  (chi)   │  │  (robfig/cron)   │ │
│  └────┬─────┘  └────────┬─────────┘ │
│       │                 │           │
│  ┌────▼─────────────────▼─────────┐ │
│  │        Application layer       │ │
│  │  AddDomain / TriggerJob /      │ │
│  │  PollTasks / SubmitResults     │ │
│  └────────────────┬───────────────┘ │
│                   │                 │
│  ┌────────────────▼───────────────┐ │
│  │    SQLite (glebarez, no CGO)   │ │
│  └──────────────────────────���─────┘ │
└──────────────────┬───────────────���──┘
                   │  HTTP polling (bearer auth)
        ┌──────────┴──────────┐
   ┌────▼────┐           ┌────▼────┐
   │Worker 1 │           │Worker N │
   │ (local) │           │(remote) │
   └─────────┘           └─────────┘
```

- **Workers are stateless.** No registration. Any machine with a valid token can participate.
- **Global concurrency cap** is enforced server-side at poll time by counting in-progress tasks for the root domain (`a.example.com` and `b.example.com` share the `example.com` cap).
- **Stale task reclaim:** tasks in-progress beyond `--stale-timeout` revert to pending automatically.
- **URL deduplication:** same URL won't be enqueued twice for the same domain (upsert with `ON CONFLICT DO NOTHING`).

---

## Environment variables

All CLI flags have env var equivalents:

| Env var | Flag | Description |
|---------|------|-------------|
| `HTTP_ADDR` | `--addr` | Server listen address |
| `DATABASE_PATH` | `--db` | SQLite file path |
| `ADMIN_BOOTSTRAP_TOKEN` | `--bootstrap-token` | Seed token value |
| `STALE_TASK_TIMEOUT` | `--stale-timeout` | Stale task reclaim duration |
| `MASTER_URL` | `--master` | Worker: master server URL |
| `WORKER_TOKEN` | `--token` | Worker: bearer token |
| `JOB_ID` | `--job-id` | Worker: job to process |
| `WORKER_BATCH` | `--batch` | Worker: URLs per poll |
| `WORKER_CONCURRENCY` | `--concurrency` | Worker: parallel fetches |
| `WORKER_POLL_INTERVAL` | `--poll-interval` | Worker: idle poll interval |
